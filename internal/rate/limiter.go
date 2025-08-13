package rate

import (
	"context"
	"fmt"
	"ig-webhook/internal/store"
	"time"
)

type Limiter struct {
	kv *store.RedisStore
}

func NewLimiter(kv *store.RedisStore) *Limiter {
	return &Limiter{kv: kv}
}

// returns allow bool, current count hour, current count day
func (l *Limiter) CheckAndIncr(ctx context.Context, brandID, action string, maxHour, maxDay int) (bool, int64, int64, error) {
	hourKey := fmt.Sprintf("rl:%s:%s:hour:%s", brandID, action, time.Now().UTC().Format("2006010215"))
	dayKey := fmt.Sprintf("rl:%s:%s:day:%s", brandID, action, time.Now().UTC().Format("20060102"))

	hc, err := l.kv.IncrWithTTL(ctx, hourKey, time.Hour+5*time.Minute)
	if err != nil {
		return false, 0, 0, err
	}
	dc, err := l.kv.IncrWithTTL(ctx, dayKey, 24*time.Hour+30*time.Minute)
	if err != nil {
		return false, hc, 0, err
	}

	allow := (maxHour <= 0 || int(hc) <= maxHour) && (maxDay <= 0 || int(dc) <= maxDay)
	return allow, hc, dc, nil
}

func (l *Limiter) SetCooldown(ctx context.Context, brandID, igUserID string, dur time.Duration) error {
	key := fmt.Sprintf("cooldown:dm:%s:%s", brandID, igUserID)
	return l.kv.Set(ctx, key, "1", dur)
}

func (l *Limiter) IsCoolingDown(ctx context.Context, brandID, igUserID string) (bool, error) {
	key := fmt.Sprintf("cooldown:dm:%s:%s", brandID, igUserID)
	_, err := l.kv.Get(ctx, key)
	if err != nil {
		// key not found
		return false, nil
	}
	return true, nil
}
