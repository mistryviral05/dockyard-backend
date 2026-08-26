package deployment

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/mistryviral05/dockyard/internal/http/middleware"
	"github.com/mistryviral05/dockyard/internal/http/utils/response"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (h *Handler) Deploy(w http.ResponseWriter, r *http.Request) {

	clerkID, ok := middleware.ClerkIDFromContext(r.Context())
	if !ok || clerkID == "" {
		response.Unauthorized(w, "unauthorized")
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" {
		response.BadRequest(w, "project id is required")
		return
	}

	// body is optional: no branch means the project's default branch
	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		response.BadRequest(w, "invalid request body")
		return
	}

	deploymentID, err := h.service.CreateManual(r.Context(), clerkID, projectID, req.Branch)
	if err != nil {
		switch {
		case errors.Is(err, ErrProjectNotFound):
			response.NotFound(w, "project not found")
		case errors.Is(err, ErrAgentNotSet):
			response.BadRequest(w, "this project has no server connected, connect an agent before deploying")
		case errors.Is(err, ErrDeployInProgress):
			response.NewError(w, http.StatusConflict, "a deployment is already running for this project")
		default:
			h.logger.Error("failed to create deployment",
				slog.String("clerk_id", clerkID),
				slog.String("project_id", projectID),
				slog.String("error", err.Error()),
			)
			response.InternalError(w, "failed to start the deployment, please try again")
		}
		return
	}

	h.logger.Info("deployment created",
		slog.String("clerk_id", clerkID),
		slog.String("project_id", projectID),
		slog.String("deployment_id", deploymentID),
	)

	// 202: the work has started, it is not finished
	response.WriteJson(w, http.StatusAccepted, response.GeneralData(&DeployResponse{
		DeploymentId: deploymentID,
		Status:       StatusQueued,
	}))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {

	clerkID, ok := middleware.ClerkIDFromContext(r.Context())
	if !ok || clerkID == "" {
		response.Unauthorized(w, "unauthorized")
		return
	}

	deploymentID := r.PathValue("id")
	if deploymentID == "" {
		response.BadRequest(w, "deployment id is required")
		return
	}

	d, err := h.service.Get(r.Context(), clerkID, deploymentID)
	if err != nil {
		if errors.Is(err, ErrDeploymentNotFound) {
			response.NotFound(w, "deployment not found")
			return
		}
		h.logger.Error("failed to get deployment",
			slog.String("clerk_id", clerkID),
			slog.String("deployment_id", deploymentID),
			slog.String("error", err.Error()),
		)
		response.InternalError(w, "failed to get the deployment, please try again")
		return
	}

	response.WriteJson(w, http.StatusOK, response.GeneralData(toResponse(d)))
}

func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {

	clerkID, ok := middleware.ClerkIDFromContext(r.Context())
	if !ok || clerkID == "" {
		response.Unauthorized(w, "unauthorized")
		return
	}

	deploymentID := r.PathValue("id")
	if deploymentID == "" {
		response.BadRequest(w, "deployment id is required")
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	logs, err := h.service.Logs(r.Context(), clerkID, deploymentID, cursor, limit)
	if err != nil {
		h.logger.Error("failed to get deployment logs",
			slog.String("clerk_id", clerkID),
			slog.String("deployment_id", deploymentID),
			slog.String("error", err.Error()),
		)
		response.InternalError(w, "failed to get the logs, please try again")
		return
	}

	page := &LogPage{Items: make([]*LogResponse, 0, len(logs))}
	for _, l := range logs {
		page.Items = append(page.Items, toLogResponse(l))
	}
	if len(page.Items) > 0 {
		page.NextCursor = &page.Items[len(page.Items)-1].Id
	}

	response.WriteJson(w, http.StatusOK, response.GeneralData(page))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {

	clerkID, ok := middleware.ClerkIDFromContext(r.Context())
	if !ok || clerkID == "" {
		response.Unauthorized(w, "unauthorized")
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" {
		response.BadRequest(w, "project id is required")
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	deployments, err := h.service.List(r.Context(), clerkID, projectID, cursor, limit)
	if err != nil {
		h.logger.Error("failed to list deployments",
			slog.String("clerk_id", clerkID),
			slog.String("project_id", projectID),
			slog.String("error", err.Error()),
		)
		response.InternalError(w, "failed to get deployments, please try again")
		return
	}

	items := make([]*DeploymentResponse, 0, len(deployments))
	for _, d := range deployments {
		items = append(items, toResponse(d))
	}

	response.WriteJson(w, http.StatusOK, response.GeneralData(items))
}
