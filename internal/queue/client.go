package queue

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mistryviral05/dockyard/internal/config"
)

type Client struct {
	client *asynq.Client
	logger *slog.Logger
}

func NewClient(cfg *config.Redis, logger *slog.Logger) *Client {

	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &Client{
		client: client,
		logger: logger,
	}
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) EnqueueDebugPing(ctx context.Context, message string) error {
	payload, err := json.Marshal(DebugPingPayload{Message: message})
	if err != nil {
		return err
	}

	info, err := c.client.EnqueueContext(ctx,
		asynq.NewTask(TypeDebugPing, payload),
		asynq.Queue(QueueDefault),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
		asynq.Retention(1*time.Hour),
	)
	if err != nil {
		c.logger.Error("failed to enqueue task",
			slog.String("type", TypeDebugPing),
			slog.String("error", err.Error()),
		)
		return err
	}

	c.logger.Info("task enqueued",
		slog.String("id", info.ID),
		slog.String("type", info.Type),
		slog.String("queue", info.Queue),
	)
	return nil
}
