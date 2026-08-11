package agent

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
	// set by middleware.RequireAuth
	clerkID, ok := middleware.ClerkIDFromContext(r.Context())
	if !ok || clerkID == "" {
		response.Unauthorized(w, "unauthorized")
		return
	}

	var req CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			response.BadRequest(w, "request body cannot be empty")
			return
		}
		h.logger.Error("failed to decode create agent body", slog.String("error", err.Error()))
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

	res, err := h.service.Create(r.Context(), clerkID, req.Name, req.Port)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			response.NotFound(w, "user not found")
		case errors.Is(err, ErrAgentNameConflict):
			response.NewError(w, http.StatusConflict, "an agent with this name already exists")
		default:
			h.logger.Error("failed to create agent",
				slog.String("clerk_id", clerkID),
				slog.String("agent_name", req.Name),
				slog.String("error", err.Error()),
			)
			response.InternalError(w, "failed to create agent, please try again")
		}
		return
	}

	h.logger.Info("agent created", slog.String("agent_id", res.Id), slog.String("clerk_id", clerkID))
	response.WriteJson(w, http.StatusCreated, response.GeneralData(res))
}
