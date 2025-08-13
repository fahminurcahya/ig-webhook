package worker

import (
	"context"
	"encoding/json"
	"github.com/hibiken/asynq"
	"ig-webhook/internal/ig"
	"ig-webhook/internal/queue"
	"ig-webhook/internal/rate"
	"ig-webhook/internal/store"
	"log"
)

func registerPublicReplyHandler(mux *asynq.ServeMux, kv *store.RedisStore) {
	lim := rate.NewLimiter(kv)

	mux.HandleFunc(queue.TypeSendPublicReply, func(ctx context.Context, t *asynq.Task) error {
		var p queue.TaskSendPublicReplyPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}

		// Rate limit per brand per action (contoh angka default 25/jam, 200/hari)
		allow, hc, dc, err := lim.CheckAndIncr(ctx, p.BrandID, "public_reply", 25, 200)
		if err != nil {
			return err
		}
		if !allow {
			log.Printf("[RL] public_reply throttled brand=%s hour=%d day=%d", p.BrandID, hc, dc)
			// reschedule sebentar, biar antrian lanjut
			return asynq.SkipRetry // skip saja, job ini selesai
		}

		client := ig.NewClient(p.IGToken) // gunakan graph.instagram.com untuk GET; reply perlu FB Graph
		if err := client.ReplyComment(ctx, p.CommentID, p.Message); err != nil {
			// biarkan Asynq retry dengan backoff
			return err
		}

		log.Printf("[OK] public reply sent comment=%s", p.CommentID)
		return nil
	})
}

var errRateLimited = asynq.SkipRetry // ganti sesuai kebutuhan; contoh: bisa juga retry dengan delay manual
