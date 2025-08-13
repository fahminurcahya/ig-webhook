package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// App
	AppEnv   string // development | production | staging
	HTTPAddr string // e.g. :8080
	LogLevel string // debug | info | warn | error

	// Postgres
	DatabaseURL string

	// Redis (KV/idempotency/rate-limit)
	RedisAddr     string
	RedisPassword string

	// Asynq (job queue)
	AsynqRedisAddr     string
	AsynqRedisPassword string
	AsynqRedisDB       int

	// Instagram / Meta
	IGAppSecret       string // untuk verifikasi X-Hub-Signature-256
	IGPageAccessToken string // dev only (prod: ambil per-tenant dari DB/KMS)
}

func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		HTTPAddr:    getEnv("HTTP_ADDR", ":8080"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		DatabaseURL: getEnv("DATABASE_URL", ""),

		RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		AsynqRedisAddr:     getEnvFallback("ASYNQ_REDIS_ADDR", "REDIS_ADDR", "127.0.0.1:6379"),
		AsynqRedisPassword: getEnvFallback("ASYNQ_REDIS_PASSWORD", "REDIS_PASSWORD", ""),
		AsynqRedisDB:       getEnvInt("ASYNQ_REDIS_DB", 1),

		IGAppSecret:       getEnv("IG_APP_SECRET", ""),
		IGPageAccessToken: getEnv("IG_PAGE_ACCESS_TOKEN", ""),
	}

	// Normalisasi
	cfg.AppEnv = strings.ToLower(strings.TrimSpace(cfg.AppEnv))
	cfg.LogLevel = strings.ToLower(strings.TrimSpace(cfg.LogLevel))

	// Validasi
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	var missing []string

	// Wajib di semua env
	if c.HTTPAddr == "" {
		missing = append(missing, "HTTP_ADDR")
	}
	if c.RedisAddr == "" {
		missing = append(missing, "REDIS_ADDR")
	}
	if c.AsynqRedisAddr == "" {
		missing = append(missing, "ASYNQ_REDIS_ADDR")
	}

	if c.IsProd() {
		if c.DatabaseURL == "" {
			missing = append(missing, "DATABASE_URL")
		}
		if c.IGAppSecret == "" {
			missing = append(missing, "IG_APP_SECRET")
		}
		// IG_PAGE_ACCESS_TOKEN boleh kosong di prod (ambil per-tenant dari DB/KMS)
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (c *Config) IsProd() bool {
	return c.AppEnv == "production"
}

// --- helpers ---

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvFallback(primary, fallback, def string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	if v := os.Getenv(fallback); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
