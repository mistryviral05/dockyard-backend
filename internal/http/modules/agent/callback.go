package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mistryviral05/dockyard/internal/dispatch"
	"github.com/mistryviral05/dockyard/internal/http/middleware"
	"github.com/mistryviral05/dockyard/internal/http/utils/centrifugo"
	"github.com/mistryviral05/dockyard/internal/http/utils/response"
)

const (
	maxCallbackBody  = 4 << 10
	publishTimeout   = 5 * time.Second
	statusDeploying  = "deploying"
	statusRunningDB  = "running"
	statusFailedDB   = "failed"
	eventStatusLabel = "status"
)

var (
	ErrDeploymentNotOwned = errors.New("this deployment does not belong to the calling agent")
	ErrCallbackStatus     = errors.New("status must be running or failed")
)

type DeploymentUpdater interface {
	FindAgentForDeployment(ctx context.Context, deploymentID, agentID string) (bool, error)
	Transition(ctx context.Context, id, to string, extra map[string]interface{}) (bool, error)
	StopOtherRunning(ctx context.Context, deploymentID string) (int64, error)
}

type statusMessage struct {
	Event  string `json:"event"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	URL    string `json:"url,omitempty"`
	At     string `json:"at"`
}

func (h *Handler) DeploymentCallback(w http.ResponseWriter, r *http.Request) {

	agentID, ok := middleware.AgentIDFromContext(r.Context())
	if !ok || agentID == "" {
		response.Unauthorized(w, "unauthorized")
		return
	}

	deploymentID := r.PathValue("id")
	if deploymentID == "" {
		response.BadRequest(w, "deployment id is required")
		return
	}

	var req dispatch.CallbackRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxCallbackBody)).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	status := strings.TrimSpace(req.Status)
	if status != dispatch.StatusRunning && status != dispatch.StatusFailed {
		response.BadRequest(w, ErrCallbackStatus.Error())
		return
	}

	if err := h.service.RecordDeployResult(r.Context(), agentID, deploymentID, &req); err != nil {
		switch {
		case errors.Is(err, ErrDeploymentNotOwned):
			h.logger.Warn("callback rejected",
				slog.String("agent_id", agentID),
				slog.String("deployment_id", deploymentID),
			)
			response.NotFound(w, "deployment not found")
		default:
			h.logger.Error("failed to record the deploy result",
				slog.String("deployment_id", deploymentID),
				slog.String("error", err.Error()),
			)
			response.InternalError(w, "failed to record the deploy result")
		}
		return
	}

	h.logger.Info("deploy result recorded",
		slog.String("agent_id", agentID),
		slog.String("deployment_id", deploymentID),
		slog.String("status", status),
		slog.String("url", req.URL),
	)

	h.publishDeployStatus(deploymentID, &req)

	response.WriteJson(w, http.StatusOK, response.GeneralMessage("deploy result recorded"))
}

func (h *Handler) publishDeployStatus(deploymentID string, req *dispatch.CallbackRequest) {
	if h.realtime == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
	defer cancel()

	msg := statusMessage{
		Event:  eventStatusLabel,
		Status: req.Status,
		Error:  req.Error,
		URL:    req.URL,
		At:     time.Now().UTC().Format(time.RFC3339Nano),
	}

	if err := h.realtime.Publish(ctx, centrifugo.DeploymentChannel(deploymentID), msg); err != nil {
		h.logger.Warn("could not publish the deploy status",
			slog.String("deployment_id", deploymentID),
			slog.String("error", err.Error()),
		)
	}
}
