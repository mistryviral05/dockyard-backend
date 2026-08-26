package queue

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/hibiken/asynq"
)

type Handlers struct {
	logger  *slog.Logger
	service *Service
	// aage: db *sql.DB, client *Client, github, registry...
}

func NewHandlers(logger *slog.Logger, service *Service) *Handlers {
	return &Handlers{logger: logger, service: service}
}

func (h *Handlers) HandleEqueueBuild(ctx context.Context, t *asynq.Task) error {

	var p DeployBuildPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return asynq.SkipRetry
	}

	h.logger.Info("deploy build", slog.String("deployment_id", p.DeploymentId))

	if err := h.service.DeployBuild(ctx, p.DeploymentId); err != nil {
		return err
	}

	return nil
}
