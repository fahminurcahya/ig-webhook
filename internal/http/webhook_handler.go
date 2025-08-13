package httpserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"ig-webhook/internal/processor"
	"ig-webhook/internal/store"
	"ig-webhook/internal/types"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

type WebhookHandler struct {
	kv          *store.RedisStore
	asynqClient *asynq.Client
	appSecret   string

	commentProc *processor.CommentProcessor
}

func NewWebhookHandler(
	kv *store.RedisStore,
	asynqClient *asynq.Client,
	appSecret string,
	commentProc *processor.CommentProcessor, // inject processor SEKARANG
) *WebhookHandler {
	return &WebhookHandler{
		kv:          kv,
		asynqClient: asynqClient,
		appSecret:   appSecret,
		commentProc: commentProc,
	}
}

func (h *WebhookHandler) HandleInstagram(c echo.Context) error {
	// 1) Baca body
	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	// 2) Verifikasi signature (X-Hub-Signature-256) kalau appSecret diset
	if h.appSecret != "" {
		if sig := c.Request().Header.Get("X-Hub-Signature-256"); !verifySignature(h.appSecret, bodyBytes, sig) {
			return c.NoContent(http.StatusForbidden)
		}
	}

	// 3) Quick ACK — proses async agar IG dapat response 200
	go h.process(c, bodyBytes)

	return c.NoContent(http.StatusOK)
}

func (h *WebhookHandler) process(ctx echo.Context, body []byte) {

	stdCtx := ctx.Request().Context()

	var bodyRq types.IGWebhookEnvelope
	if err := json.Unmarshal(body, &bodyRq); err != nil {
		log.Printf("[ERR] parse webhook: %v", err)
		return
	}

	for _, entry := range bodyRq.Entry {
		brandID := mapPageToBrand(entry.ID)
		for _, ch := range entry.Changes {
			if ch.Field != "comments" && ch.Field != "ig_comments" {
				continue
			}

			ev := processor.CommentEvent{
				EventID:       ch.Value.CommentID, // boleh gabung timestamp kalau perlu
				BrandID:       brandID,
				IGBusinessID:  entry.ID,
				CommentID:     ch.Value.CommentID,
				PostID:        ch.Value.PostID,
				Text:          ch.Value.Text,
				FromIGUserID:  ch.Value.From.ID,
				FromUsername:  ch.Value.From.Username,
				IGAccessToken: lookupIGToken(brandID),
			}

			if h.commentProc == nil {
				log.Printf("[WARN] commentProc not configured")
				continue
			}
			if err := h.commentProc.Process(stdCtx, ev); err != nil {
				log.Printf("[ERR] process event: %v", err)
			}
		}
	}
}

// ======= Helpers & types =======

func verifySignature(appSecret string, body []byte, sigHeader string) bool {
	// sigHeader format: "sha256=hexdigest"
	if len(sigHeader) < 7 || sigHeader[:7] != "sha256=" {
		return false
	}
	sigProvided := sigHeader[7:]

	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sigProvided), []byte(expected))
}

func mapPageToBrand(pageID string) string {
	// TODO: lookup DB mapping pageID -> brand/tenant
	return "brand-" + pageID
}

func lookupIGToken(brandID string) string {
	// TODO: ambil dari DB/KMS. Untuk sementara dari ENV
	return os.Getenv("IG_PAGE_ACCESS_TOKEN")
}
