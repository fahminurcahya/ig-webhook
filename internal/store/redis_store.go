package store

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	rdb *redis.Client
}

func NewRedisStore(rdb *redis.Client) *RedisStore {
	return &RedisStore{rdb: rdb}
}

// SETNX dengan TTL
func (s *RedisStore) AcquireOnce(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return s.rdb.SetNX(ctx, key, "1", ttl).Result()
}

func (s *RedisStore) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	v := s.rdb.Incr(ctx, key)
	if v.Err() != nil {
		return 0, v.Err()
	}
	// set expire jika belum ada
	_ = s.rdb.ExpireNX(ctx, key, ttl).Err()
	return v.Val(), nil
}

func (s *RedisStore) Get(ctx context.Context, key string) (string, error) {
	return s.rdb.Get(ctx, key).Result()
}

func (s *RedisStore) Set(ctx context.Context, key string, val string, ttl time.Duration) error {
	return s.rdb.Set(ctx, key, val, ttl).Err()
}
