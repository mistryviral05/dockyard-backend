package github

import (
	"encoding/json"
	"errors"
	"fmt"
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

func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// currently general data logs
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		if errors.Is(err, io.EOF) {
			response.BadRequest(w, "request body cannot be empty")
			return
		}
		h.logger.Error("failed to decode request body", slog.String("error", err.Error()))
		response.BadRequest(w, "invalid request body")
		return
	}

	pretty, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println(string(pretty))
	h.logger.Info("webhook received", slog.String("event", r.Header.Get("x-github-event")))
	h.logger.Info("webhook received", slog.String("signature", r.Header.Get("x-hub-signature-256")))
	response.WriteJson(w, http.StatusAccepted, "Accepted")
}

func (h *Handler) CreateInstallation(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.ClerkIDFromContext(r.Context())

	//decoding json
	var req ConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			response.BadRequest(w, "request body cannot be empty")
			return
		}
		h.logger.Error("failed to decode request body", slog.String("error", err.Error()))
		response.BadRequest(w, "invalid request body")
		return
	}

	req.ClerkId = userID
	// validation
	if err := validator.New().Struct(req); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			response.WriteJson(w, http.StatusBadRequest, response.ValidationError(ve))
			return
		}
		response.BadRequest(w, "invalid request")
		return
	}

	// creating installation
	if err := h.service.CreateInstallation(r.Context(), req); err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			response.NotFound(w, "user not found")
		case errors.Is(err, ErrStateMismatch):
			response.Unauthorized(w, "invalid state")
		default:
			h.logger.Error("github connect failed", slog.String("error", err.Error()))
			response.InternalError(w, "failed to connect github")
		}
		return
	}

	response.WriteJson(w, http.StatusOK, response.GeneralMessage("github connected!"))
}

func (h *Handler) GetInstallationLink(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.ClerkIDFromContext(r.Context())
	link, err := h.service.CreateInstallationLink(userID)
	if err != nil {
		h.logger.Error("failed to create installation link", slog.String("error", err.Error()))
		response.InternalError(w, "failed to login with github")
	}
	rp := map[string]string{
		"link": link,
	}
	response.WriteJson(w, http.StatusOK, response.GeneralData(rp))

}
