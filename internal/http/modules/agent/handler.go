package agent

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/mistryviral05/dockyard/internal/http/middleware"
	"github.com/mistryviral05/dockyard/internal/http/utils/response"
)

const maxHeartbeatBody = 8 << 10

type Handler struct {
	service   *Service
	logger    *slog.Logger
	install   InstallConfig
	agentAuth func(http.Handler) http.Handler
}

func NewHandler(service *Service, logger *slog.Logger, install InstallConfig) *Handler {
	return &Handler{
		service:   service,
		logger:    logger,
		install:   install,
		agentAuth: middleware.RequireAgentAuth(logger, service),
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {

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

func (h *Handler) InstallScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(RenderInstallScript(h.install)))
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {

	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		response.Unauthorized(w, "missing registration token")
		return
	}

	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	if token == "" {
		response.Unauthorized(w, "missing registration token")
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxHeartbeatBody)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
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

	res, err := h.service.Register(r.Context(), token, &req, middleware.ClientIP(r))
	if err != nil {
		switch {
		case errors.Is(err, ErrRegistrationTokenInvalid):
			h.logger.Warn("agent registration rejected",
				slog.String("remote_addr", middleware.ClientIP(r)),
			)
			response.Unauthorized(w, "registration token is invalid, used or expired")
		case errors.Is(err, ErrPublicIPInvalid):
			response.BadRequest(w, "public ip is missing or not a valid address")
		case errors.Is(err, ErrSubdomainConflict):
			response.NewError(w, http.StatusConflict, "could not pick a free subdomain for this agent")
		default:
			h.logger.Error("failed to register agent", slog.String("error", err.Error()))
			response.InternalError(w, "failed to register the agent, please try again")
		}
		return
	}

	h.logger.Info("agent registered",
		slog.String("agent_id", res.AgentID),
		slog.String("domain", res.Domain),
	)

	response.WriteJson(w, http.StatusCreated, response.GeneralData(res))
}

func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {

	agentID, ok := middleware.AgentIDFromContext(r.Context())
	if !ok || agentID == "" {
		response.Unauthorized(w, "unauthorized")
		return
	}

	var req HeartbeatRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxHeartbeatBody)).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			response.BadRequest(w, "request body cannot be empty")
			return
		}
		h.logger.Warn("failed to decode heartbeat body",
			slog.String("agent_id", agentID),
			slog.String("error", err.Error()),
		)
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

	dto := req.ToDto(agentID, middleware.ClientIP(r))

	res, err := h.service.RecordHeartbeat(r.Context(), dto)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			response.NotFound(w, "agent not found")
			return
		}
		h.logger.Error("failed to record heartbeat",
			slog.String("agent_id", agentID),
			slog.String("error", err.Error()),
		)
		response.InternalError(w, "failed to record heartbeat, please try again")
		return
	}

	response.WriteJson(w, http.StatusOK, response.GeneralData(res))
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {

	clerkID, ok := middleware.ClerkIDFromContext(r.Context())
	if !ok || clerkID == "" {
		response.Unauthorized(w, "unauthorized")
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		response.BadRequest(w, "agent id is required")
		return
	}

	agent, err := h.service.GetByID(r.Context(), clerkID, agentID)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			response.NotFound(w, "agent not found")
			return
		}
		h.logger.Error("failed to get agent",
			slog.String("clerk_id", clerkID),
			slog.String("agent_id", agentID),
			slog.String("error", err.Error()),
		)
		response.InternalError(w, "failed to get the agent, please try again")
		return
	}

	response.WriteJson(w, http.StatusOK, response.GeneralData(agent))
}

func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {

	clerkID, ok := middleware.ClerkIDFromContext(r.Context())
	if !ok || clerkID == "" {
		response.Unauthorized(w, "unauthorized")
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		response.BadRequest(w, "agent id is required")
		return
	}

	q, err := parseMetricsQuery(r.URL.Query())
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	metrics, err := h.service.GetMetrics(r.Context(), clerkID, agentID, q)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			response.NotFound(w, "agent not found")
			return
		}
		h.logger.Error("failed to get agent metrics",
			slog.String("clerk_id", clerkID),
			slog.String("agent_id", agentID),
			slog.String("error", err.Error()),
		)
		response.InternalError(w, "failed to get agent metrics, please try again")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	response.WriteJson(w, http.StatusOK, response.GeneralData(metrics))
}

func (h *Handler) GetMetricsBench(w http.ResponseWriter, r *http.Request) {

	agentID := r.PathValue("id")
	if agentID == "" {
		response.BadRequest(w, "agent id is required")
		return
	}

	q, err := parseMetricsQuery(r.URL.Query())
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	metrics, err := h.service.GetPublicMetrics(r.Context(), agentID, q)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			response.NotFound(w, "agent not found")
			return
		}
		response.InternalError(w, "failed to get agent metrics, please try again")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	response.WriteJson(w, http.StatusOK, response.GeneralData(metrics))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {

	clerkID, ok := middleware.ClerkIDFromContext(r.Context())
	if !ok || clerkID == "" {
		response.Unauthorized(w, "unauthorized")
		return
	}

	agents, err := h.service.Get(r.Context(), clerkID)
	if err != nil {
		h.logger.Error("failed to get agents",
			slog.String("clerk_id", clerkID),
			slog.String("error", err.Error()),
		)
		response.InternalError(w, "failed to get agents, please try again")
		return
	}

	response.WriteJson(w, http.StatusOK, response.GeneralData(agents))
}
