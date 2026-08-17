package queue

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/hibiken/asynq"
)

type Handlers struct {
	logger *slog.Logger
	// aage: db *sql.DB, client *Client, github, registry...
}

func NewHandlers(logger *slog.Logger) *Handlers {
	return &Handlers{logger: logger}
}

func (h *Handlers) HandleDebugPing(ctx context.Context, t *asynq.Task) error {
	var p DebugPingPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		// payload hi kharab hai - retry se theek nahi hoga
		return asynq.SkipRetry
	}

	h.logger.Info("pong", slog.String("message", p.Message))
	return nil
}

func (h *Handlers) HandleEqueueBuild(ctx context.Context, t *asynq.Task) error {

	var p DeployBuildPayload
	if err := json.Unmarshal(t.Payload(),&p); err != nil {
		return asynq.SkipRetry
	}

	h.logger.Info("deploy build", slog.String("deployment_id", p.DeploymentId))

	return nil
}
