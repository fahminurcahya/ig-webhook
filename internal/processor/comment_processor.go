package processor

import (
	"context"
	"encoding/json"
	"ig-webhook/internal/queue"
	"ig-webhook/internal/store"
	"ig-webhook/internal/types"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/hibiken/asynq"
)

type CommentEvent struct {
	EventID       string // unique id dari IG webhook (atau gabungan: comment_id + timestamp)
	BrandID       string // tenant/brand internal ID
	IGBusinessID  string // IG business account id
	CommentID     string
	PostID        string
	Text          string
	FromIGUserID  string
	FromUsername  string
	IGAccessToken string // token spesifik brand
}

type WorkflowRepo interface {
	types.WorkflowRepo
}

type CommentProcessor struct {
	kv *store.RedisStore
	q  *asynq.Client
	db WorkflowRepo
}

func NewCommentProcessor(kv *store.RedisStore, q *asynq.Client, db WorkflowRepo) *CommentProcessor {
	return &CommentProcessor{kv: kv, q: q, db: db}
}

func (p *CommentProcessor) Process(ctx context.Context, ev CommentEvent) error {
	// Idempotensi event level
	ok, err := p.kv.AcquireOnce(ctx, "idem:event:"+ev.EventID, 7*24*time.Hour)
	if err != nil {
		return err
	}
	if !ok {
		log.Printf("[IDEMP] duplicate event %s", ev.EventID)
		return nil
	}

	// Ambil workflows
	wfs, err := p.db.ListActiveWorkflowsForIGAccount(ev.IGBusinessID)
	if err != nil {
		return err
	}

	for _, wf := range wfs {
		// Cari trigger node IG_COMMENT_RECEIVED
		var trig types.Node
		for _, n := range wf.Nodes {
			// n.Data["type"] di JSON menyimpan trigger type
			if t, _ := n.Data["type"].(string); t == string(types.TriggerIGCommentReceived) {
				trig = n
				break
			}
		}
		if trig.ID == "" {
			continue
		}

		// Parse igUserCommentData
		b, _ := json.Marshal(trig.Data["igUserCommentData"])
		var cfg types.IGUserCommentData
		_ = json.Unmarshal(b, &cfg)

		// Post filter
		if !contains(cfg.SelectedPostID, ev.PostID) {
			continue
		}

		// Keyword filter
		if !matchIncludeExclude(ev.Text, cfg.IncludeKeywords, cfg.ExcludeKeywords) {
			continue
		}

		// Eksekusi next node: cari node target dari edge
		next := nextNodeID(wf, trig.ID)
		if next == "" {
			continue
		}

		var actionNode types.Node
		for _, n := range wf.Nodes {
			if n.ID == next {
				actionNode = n
				break
			}
		}
		if actionNode.ID == "" {
			continue
		}

		// Idempotensi node execution
		execKey := "idem:exec:" + wf.ID + ":" + actionNode.ID + ":" + ev.CommentID
		ok, err := p.kv.AcquireOnce(ctx, execKey, 7*24*time.Hour)
		if err != nil {
			return err
		}
		if !ok {
			log.Printf("[IDEMP] node execution exists wf=%s node=%s", wf.ID, actionNode.ID)
			continue
		}

		// Parse IG_SEND_MSG
		if t, _ := actionNode.Data["type"].(string); t == string(types.ActionIGSendMsg) {
			// igReplyData
			b2, _ := json.Marshal(actionNode.Data["igReplyData"])
			var rd types.IGReplyData
			_ = json.Unmarshal(b2, &rd)

			// Safety
			limits := rd.Safety.CombinedLimits
			delayBetween := queue.RandDelaySec(limits.DelayBetweenActions[0], limits.DelayBetweenActions[1])
			commentToDm := queue.RandDelaySec(limits.CommentToDmDelay[0], limits.CommentToDmDelay[1])

			// Pick public reply (random/round-robin; di sini ambil index by hash)
			msg := pickOne(rd.PublicReplies, ev.FromIGUserID)

			// Enqueue public reply
			pubPayload := queue.TaskSendPublicReplyPayload{
				BrandID:    ev.BrandID,
				CommentID:  ev.CommentID,
				Message:    sanitizePublicMessage(msg, rd.Safety.ContentRules),
				IGToken:    ev.IGAccessToken,
				WorkflowID: wf.ID,
				NodeID:     actionNode.ID,
			}
			taskA, optsA := queue.NewPublicReplyTask(pubPayload, delayBetween)
			if _, err := p.q.EnqueueContext(ctx, taskA, optsA...); err != nil {
				return err
			}

			// Compose DM message + tombol (render sederhana jadi teks)
			dmText := rd.DMMessage
			for _, btn := range rd.Buttons {
				if btn.Enabled && btn.URL != "" {
					dmText += "\n" + btn.Title + ": " + btn.URL
				}
			}

			// Enqueue DM (depends on commentToDmDelay)
			dmPayload := queue.TaskSendDMPayload{
				BrandID:           ev.BrandID,
				RecipientIGUserID: ev.FromIGUserID,
				Message:           dmText,
				IGToken:           ev.IGAccessToken,
				WorkflowID:        wf.ID,
				NodeID:            actionNode.ID,
			}
			taskB, optsB := queue.NewDMTask(dmPayload, commentToDm+delayBetween)
			if _, err := p.q.EnqueueContext(ctx, taskB, optsB...); err != nil {
				return err
			}
		}
	}
	return nil
}

func contains(a []string, x string) bool {
	for _, v := range a {
		if v == x {
			return true
		}
	}
	return false
}

func matchIncludeExclude(text string, includes, excludes []string) bool {
	// Normalisasi sederhana
	n := strings.ToLower(strings.TrimSpace(text))
	n = stripEmoji(n)

	// Exclude first
	for _, ex := range excludes {
		if ex == "" {
			continue
		}
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(strings.ToLower(ex)) + `\b`)
		if re.FindStringIndex(n) != nil {
			return false
		}
	}

	// Include (match salah satu)
	if len(includes) == 0 {
		return true
	}
	for _, in := range includes {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(strings.ToLower(in)) + `\b`)
		if re.FindStringIndex(n) != nil {
			return true
		}
	}
	// tambahan sinonim ID sederhana
	if strings.Contains(n, "harga") && contains(includes, "price") {
		return true
	}
	if strings.Contains(n, "informasi") && contains(includes, "info") {
		return true
	}

	return false
}

func pickOne(arr []string, salt string) string {
	if len(arr) == 0 {
		return ""
	}
	h := 0
	for i := 0; i < len(salt); i++ {
		h = (h*31 + int(salt[i])) & 0x7fffffff
	}
	idx := h % len(arr)
	return arr[idx]
}

func sanitizePublicMessage(msg string, rules types.SafetyContentRules) string {
	// Placeholder: implement trimming mention/hashtags > limit.
	return msg
}

func stripEmoji(s string) string {
	// Minimal: hapus karakter non-ASCII
	b := make([]rune, 0, len(s))
	for _, r := range s {
		if r <= 127 {
			b = append(b, r)
		}
	}
	return string(b)
}

func nextNodeID(wf *types.WorkflowDefinition, from string) string {
	for _, e := range wf.Edges {
		if e.Source == from {
			return e.Target
		}
	}
	return ""
}
