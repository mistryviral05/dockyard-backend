package workspace

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
	// set by middleware.RequireAuth — the internal users.id
	userID, _ := middleware.ClerkIDFromContext(r.Context())

	var req CreateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			response.BadRequest(w, "request body cannot be empty")
			return
		}
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

	dto := &CreateWorkspaceDto{
		UserId: userID,
		Name:   req.Name,
		Slug:   req.Slug,
	}

	id, err := h.service.Create(r.Context(), dto)
	if err != nil {
		h.logger.Error("failed to create workspace", slog.String("error", err.Error()))
		response.InternalError(w, "failed to create workspace")
		return
	}

	response.WriteJson(w, http.StatusCreated, response.GeneralMessage("workspace created: "+id))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	// set by middleware.RequireAuth — the internal users.id
	userID, _ := middleware.ClerkIDFromContext(r.Context())
	workspaces, err := h.service.Get(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get workspaces", slog.String("error", err.Error()))
		response.InternalError(w, "failed to get workspaces")
		return
	}

	response.WriteJson(w, http.StatusOK, workspaces)
}
