package queue

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/mistryviral05/dockyard/internal/config"
)

type Server struct {
	server *asynq.Server
	mux    *asynq.ServeMux
	logger *slog.Logger
}

func NewServer(cfg *config.Redis, logger *slog.Logger, h *Handlers) *Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				QueueCritical: 6,
				QueueDefault:  3,
				QueueLow:      1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(
				func(ctx context.Context, t *asynq.Task, err error) {
					retried, _ := asynq.GetRetryCount(ctx)
					maxRetry, _ := asynq.GetMaxRetry(ctx)
					logger.Error("task failed",
						slog.String("type", t.Type()),
						slog.Int("retry", retried),
						slog.Int("max_retry", maxRetry),
						slog.String("error", err.Error()),
					)
				},
			),
			Logger: newAsynqLogger(logger),
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeDebugPing, h.HandleDebugPing)

	return &Server{server: srv, mux: mux, logger: logger}
}

func (s *Server) Run() error { return s.server.Run(s.mux) }
func (s *Server) Shutdown()  { s.server.Shutdown() }
