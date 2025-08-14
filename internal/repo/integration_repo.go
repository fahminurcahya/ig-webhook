package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"ig-webhook/internal/store"
	"time"
)

type IGTokenLookup struct {
	kv   *store.RedisStore
	pool *pgxpool.Pool
}

func NewIGTokenLookup(kv *store.RedisStore, pool *pgxpool.Pool) *IGTokenLookup {
	return &IGTokenLookup{kv: kv, pool: pool}
}

type tokenCache struct {
	Token     string     `json:"token"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// 1) try to lookup in redis
// 2) hit DB (Integration) to provider INSTAGRAM & aktif
// 3) cache hasil dgn TTL aman (<= expiry - 2m) atau 30m kalau tidak ada expiry
func (l *IGTokenLookup) Lookup(ctx context.Context, brandID string) (string, error) {
	if brandID == "" {
		return "", fmt.Errorf("brandID empty")
	}
	cacheKey := fmt.Sprintf("ig:token:%s", brandID)

	// 1) Cache
	if raw, err := l.kv.Get(ctx, cacheKey); err == nil && raw != "" {
		var c tokenCache
		if json.Unmarshal([]byte(raw), &c) == nil && c.Token != "" {
			// cek hampir kadaluarsa?
			if c.ExpiresAt == nil || time.Until(*c.ExpiresAt) > 3*time.Minute {
				return c.Token, nil
			}
		}
	}

	// 2) DB
	const q = `
		SELECT access_token, expires_at
		FROM zosmed."integration"
		WHERE account_id = $1
		  AND type = 'INSTAGRAM'
		ORDER BY updated_at DESC
		LIMIT 1;
	`
	var (
		token     string
		expiresAt *time.Time
	)
	if err := l.pool.QueryRow(ctx, q, brandID).Scan(&token, &expiresAt); err != nil {
		return "", fmt.Errorf("lookup ig token db: %w", err)
	}
	if token == "" {
		return "", fmt.Errorf("empty token for brand=%s", brandID)
	}

	// 3) Cache hasil
	ttl := 30 * time.Minute
	if expiresAt != nil {
		// simpan sedikit di bawah expiry supaya otomatis refresh
		d := time.Until(*expiresAt) - 2*time.Minute
		if d > 1*time.Minute {
			ttl = d
		}
	}
	b, _ := json.Marshal(tokenCache{Token: token, ExpiresAt: expiresAt})
	_ = l.kv.Set(ctx, cacheKey, string(b), ttl)

	return token, nil
}

type IntegrationRepo struct {
	Pool *pgxpool.Pool
}

type IntegrationRow struct {
	ID          string
	UserID      string
	AccessToken string
	ExpiresAt   *time.Time
}

func NewIntegrationRepo(p *pgxpool.Pool) *IntegrationRepo { return &IntegrationRepo{Pool: p} }

func (r *IntegrationRepo) GetByIDForUser(ctx context.Context, id, userID string) (*IntegrationRow, error) {
	const q = `
		SELECT id, user_id, access_token, expires_at
		FROM zosmed."integration"
		WHERE id = $1 AND user_id = $2
		LIMIT 1;`
	var row IntegrationRow
	if err := r.Pool.QueryRow(ctx, q, id, userID).
		Scan(&row.ID, &row.UserID, &row.AccessToken, &row.ExpiresAt); err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *IntegrationRepo) UpdateToken(ctx context.Context, id string, newToken string, expiresAt *time.Time) error {
	const u = `
		UPDATE zosmed."integration"
		SET access_token = $2,
			expires_at   = $3,
			last_sync_at = NOW(),
			updated_at   = NOW()
		WHERE id = $1;`
	_, err := r.Pool.Exec(ctx, u, id, newToken, expiresAt)
	return err
}
