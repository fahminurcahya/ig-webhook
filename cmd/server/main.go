package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"ig-webhook/internal/config"
	httpserver "ig-webhook/internal/http"
	"ig-webhook/internal/processor"
	"ig-webhook/internal/queue"
	"ig-webhook/internal/queue/worker"
	"ig-webhook/internal/repo"
	"ig-webhook/internal/store"
	"log"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func mustPGPool(dsn string) *pgxpool.Pool {
	log.Println(dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("pgx connect: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("pgx ping: %v", err)
	}
	return pool
}

func main() {
	_ = godotenv.Load()
	// Load Env
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	kv := store.NewRedisStore(rdb)

	// PG
	pg := mustPGPool(cfg.DatabaseURL)
	defer pg.Close()

	igTokenLookup := repo.NewIGTokenLookup(kv, pg)
	httpserver.SetIGTokenLookup(igTokenLookup)

	// Asynq
	asynqDB := cfg.AsynqRedisDB
	asynqOpt := asynq.RedisClientOpt{
		Addr:     cfg.AsynqRedisAddr,
		Password: cfg.AsynqRedisPassword,
		DB:       asynqDB,
	}
	asynqClient := asynq.NewClient(asynqOpt)
	defer asynqClient.Close()

	// Worker (consumer)
	srv := asynq.NewServer(asynqOpt, asynq.Config{
		Concurrency: 10,
		Queues: map[string]int{
			queue.QueueDefault:  5,
			queue.QueuePriority: 5,
		},
	})
	mux := asynq.NewServeMux()
	worker.RegisterHandlers(mux, kv)

	// Run worker asynchronously
	go func() {
		if err := srv.Run(mux); err != nil {
			log.Fatalf("asynq server error: %v", err)
		}
	}()

	// HTTP server
	e := echo.New()
	e.GET("/healthz", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

	// Webhook
	workflowRepo := repo.NewPGWorkflowRepo(pg)
	commentProc := processor.NewCommentProcessor(kv, asynqClient, workflowRepo)
	webhook := httpserver.NewWebhookHandler(kv, asynqClient, cfg.IGAppSecret, commentProc)
	e.POST("/webhook/instagram", webhook.HandleInstagram)

	s := &http.Server{
		Addr:              cfg.HTTPAddr,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("HTTP listening on %s (env=%s)", cfg.HTTPAddr, cfg.AppEnv)
	if err := e.StartServer(s); err != nil {
		log.Fatal(err)
	}
}
