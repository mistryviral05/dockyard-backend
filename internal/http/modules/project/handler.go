package project

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"
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

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {

	clerkID, ok := middleware.ClerkIDFromContext(r.Context())
	if !ok || clerkID == "" {
		response.Unauthorized(w, "unauthorized")
		return
	}

	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			response.BadRequest(w, "request body cannot be empty")
			return
		}
		h.logger.Error("failed to decode create project body", slog.String("error", err.Error()))
		response.BadRequest(w, "invalid request body")
		return
	}

	// validates the embedded RepositoryRequest fields too
	if err := validator.New().Struct(req); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			response.WriteJson(w, http.StatusBadRequest, response.ValidationError(ve))
			return
		}
		response.BadRequest(w, "invalid request")
		return
	}

	if err := h.service.Create(r.Context(), clerkID, &req, &req.RepositoryRequest); err != nil {
		switch {
		case errors.Is(err, ErrWorkspaceNotFound):
			response.NotFound(w, "workspace not found")
		case errors.Is(err, ErrInstallationNotFound):
			response.NotFound(w, "github installation not found for this repository owner")
		case errors.Is(err, ErrRepoAlreadyLinked):
			response.NewError(w, http.StatusConflict, "this repository is already linked to a project")
		default:
			h.logger.Error("failed to create project",
				slog.String("clerk_id", clerkID),
				slog.String("project_name", req.Name),
				slog.String("repo_full_name", req.RepoFullName),
				slog.String("error", err.Error()),
			)
			response.InternalError(w, "failed to create project, please try again")
		}
		return
	}

	h.logger.Info("project created",
		slog.String("clerk_id", clerkID),
		slog.String("project_name", req.Name),
	)
	response.WriteJson(w, http.StatusCreated, response.GeneralMessage("Project is created successfully"))
}

func (h *Handler) SetAgent(w http.ResponseWriter, r *http.Request) {

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

	var req SetAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			response.BadRequest(w, "request body cannot be empty")
			return
		}
		h.logger.Error("failed to decode set agent body", slog.String("error", err.Error()))
		response.BadRequest(w, "invalid request body")
		return
	}

	if err := validator.New().Struct(req); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			response.WriteJson(w, http.StatusBadRequest, response.ValidationError(ve))
			return
		}
		response.BadRequest(w, "invalid request")
		return
	}

	project, err := h.service.SetAgent(r.Context(), clerkID, projectID, req.AgentId)
	if err != nil {
		switch {
		case errors.Is(err, ErrProjectNotFound):
			response.NotFound(w, "project not found")
		case errors.Is(err, ErrAgentNotFound):
			response.NotFound(w, "agent not found")
		default:
			h.logger.Error("failed to set project agent",
				slog.String("clerk_id", clerkID),
				slog.String("project_id", projectID),
				slog.String("agent_id", req.AgentId),
				slog.String("error", err.Error()),
			)
			response.InternalError(w, "failed to set the agent, please try again")
		}
		return
	}

	h.logger.Info("project agent set",
		slog.String("clerk_id", clerkID),
		slog.String("project_id", projectID),
		slog.String("agent_id", req.AgentId),
	)
	response.WriteJson(w, http.StatusOK, response.GeneralData(project))
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {

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

	project, err := h.service.GetByID(r.Context(), clerkID, projectID)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			response.NotFound(w, "project not found")
			return
		}
		h.logger.Error("failed to get project",
			slog.String("clerk_id", clerkID),
			slog.String("project_id", projectID),
			slog.String("error", err.Error()),
		)
		response.InternalError(w, "failed to get project, please try again")
		return
	}

	response.WriteJson(w, http.StatusOK, response.GeneralData(project))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {

	clerkID, ok := middleware.ClerkIDFromContext(r.Context())
	if !ok || clerkID == "" {
		response.Unauthorized(w, "unauthorized")
		return
	}

	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		response.BadRequest(w, "workspace_id is required")
		return
	}

	projects, err := h.service.Get(r.Context(), clerkID, workspaceID)
	if err != nil {
		h.logger.Error("failed to get projects",
			slog.String("clerk_id", clerkID),
			slog.String("workspace_id", workspaceID),
			slog.String("error", err.Error()),
		)
		response.InternalError(w, "failed to get projects, please try again")
		return
	}

	response.WriteJson(w, http.StatusOK, response.GeneralData(projects))
}
