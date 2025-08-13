package worker

import (
	"context"
	"encoding/json"
	"ig-webhook/internal/ig"
	"ig-webhook/internal/queue"
	"ig-webhook/internal/rate"
	"ig-webhook/internal/store"
	"log"
	"time"

	"github.com/hibiken/asynq"
)

func registerDMHandler(mux *asynq.ServeMux, kv *store.RedisStore) {
	lim := rate.NewLimiter(kv)

	mux.HandleFunc(queue.TypeSendDM, func(ctx context.Context, t *asynq.Task) error {
		var p queue.TaskSendDMPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}

		// Cooldown per user (contoh 24 jam)
		cooling, _ := lim.IsCoolingDown(ctx, p.BrandID, p.RecipientIGUserID)
		if cooling {
			log.Printf("[SKIP] DM cooldown brand=%s user=%s", p.BrandID, p.RecipientIGUserID)
			return nil
		}

		// Rate limit (contoh angka default 25/jam, 200/hari)
		allow, hc, dc, err := lim.CheckAndIncr(ctx, p.BrandID, "dm", 25, 200)
		if err != nil {
			return err
		}
		if !allow {
			log.Printf("[RL] dm throttled brand=%s hour=%d day=%d", p.BrandID, hc, dc)
			return asynq.SkipRetry // skip saja, job ini selesai
		}

		client := ig.NewClient(p.IGToken)
		if err := client.SendDM(ctx, p.RecipientIGUserID, p.Message); err != nil {
			// TODO: mapping error: kalau permission denied → bisa fallback ke public reply alternatif
			return err
		}

		// Set cooldown setelah sukses
		_ = lim.SetCooldown(ctx, p.BrandID, p.RecipientIGUserID, 24*time.Hour)

		log.Printf("[OK] DM sent to user=%s", p.RecipientIGUserID)
		return nil
	})
}
