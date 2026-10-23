package agentd

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mistryviral05/dockyard/internal/dispatch"
	"github.com/mistryviral05/dockyard/internal/http/utils/signer"
)

const maxDeployBody = 16 << 10

type deployResponse struct {
	Status       string `json:"status"`
	DeploymentID string `json:"deployment_id"`
	Container    string `json:"container"`
}

func (a *Agent) handleDeploy(w http.ResponseWriter, r *http.Request) {
	if !a.state.IsRegistered() {
		writeAgentError(w, http.StatusServiceUnavailable, "agent is not registered")
		return
	}

	token := bearerToken(r)
	if token == "" {
		writeAgentError(w, http.StatusUnauthorized, "missing signed command")
		return
	}

	var cmd dispatch.Command
	if err := signer.New(a.state.SigningSecret).Verify(token, &cmd); err != nil {
		a.log.Warn("rejected a deploy command",
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("error", err.Error()),
		)
		writeAgentError(w, http.StatusUnauthorized, "the command signature is not valid")
		return
	}

	if err := validateCommand(&cmd); err != nil {
		writeAgentError(w, http.StatusBadRequest, err.Error())
		return
	}

	a.log.Info("deploy accepted",
		slog.String("deployment_id", cmd.DeploymentID),
		slog.String("project_id", cmd.ProjectID),
		slog.String("image", cmd.Image),
		slog.String("domain", cmd.Domain),
	)

	a.background(func() { a.runDeploy(cmd) })

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(deployResponse{
		Status:       "accepted",
		DeploymentID: cmd.DeploymentID,
		Container:    ContainerName(cmd.ProjectID),
	})
}

func (a *Agent) runDeploy(cmd dispatch.Command) {
	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.DeployTimeout)
	defer cancel()

	log := a.log.With(slog.String("deployment_id", cmd.DeploymentID))

	containerID, err := a.docker.Deploy(ctx, &cmd)
	if err != nil {
		log.Error("deploy failed", slog.String("error", err.Error()))
		a.reportDeploy(ctx, &cmd, dispatch.CallbackRequest{
			Status: dispatch.StatusFailed,
			Error:  err.Error(),
		})
		return
	}

	log.Info("deploy finished",
		slog.String("container_id", containerID),
		slog.String("url", "https://"+cmd.Domain),
	)

	a.reportDeploy(ctx, &cmd, dispatch.CallbackRequest{
		Status:      dispatch.StatusRunning,
		ContainerID: containerID,
		URL:         "https://" + cmd.Domain,
	})
}

func (a *Agent) reportDeploy(ctx context.Context, cmd *dispatch.Command, result dispatch.CallbackRequest) {
	reportCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := a.client.SendDeployResult(reportCtx, cmd.DeploymentID, result); err != nil {
		a.log.Error("could not report the deploy result",
			slog.String("deployment_id", cmd.DeploymentID),
			slog.String("error", err.Error()),
		)
	}
}

func validateCommand(cmd *dispatch.Command) error {
	switch {
	case strings.TrimSpace(cmd.DeploymentID) == "":
		return errors.New("deployment_id is required")
	case strings.TrimSpace(cmd.ProjectID) == "":
		return errors.New("project_id is required")
	case strings.TrimSpace(cmd.Image) == "":
		return errors.New("image is required")
	case strings.TrimSpace(cmd.Domain) == "":
		return errors.New("domain is required")
	case cmd.Port <= 0 || cmd.Port > 65535:
		return errors.New("port must be between 1 and 65535")
	}

	return nil
}

func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
}

func writeAgentError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "error",
		"error":  message,
	})
}
