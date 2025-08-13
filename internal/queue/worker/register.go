package worker

import (
	"github.com/hibiken/asynq"
	"ig-webhook/internal/store"
)

// RegisterHandlers mengikat semua handler task ke mux asynq.
func RegisterHandlers(mux *asynq.ServeMux, kv *store.RedisStore) {
	// inject dependency ke masing-masing file handler
	registerPublicReplyHandler(mux, kv)
	registerDMHandler(mux, kv)
}
