package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/mistryviral05/dockyard/internal/config"
	"github.com/mistryviral05/dockyard/internal/http/utils/response"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
	cfg     *config.Config
}

func newHandler(service *Service, logger *slog.Logger, cfg *config.Config) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
		cfg:     cfg,
	}
}

func (h *Handler) handleWebhook(w http.ResponseWriter, r *http.Request) {

	var event ClerkEvent

	err := json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		if errors.Is(err, io.EOF) {
			response.BadRequest(w, "request body cannot be empty")
			return
		}
		response.BadRequest(w, "invalid request body")
		return
	}

	switch event.Type {
	case "user.created":
		var userData ClerkUserData
		if err := json.Unmarshal(event.Data, &userData); err != nil {
			response.BadRequest(w, "invalid user data")
			return
		}
		dto := userData.ToCreateUserDto()
		if err := h.service.createUser(r.Context(), dto); err != nil {
			h.logger.Error("failed to create user", slog.String("error", err.Error()))
			response.InternalError(w, "failed to create user!")
			return
		}
		h.logger.Info("User created successfully!." + dto.ClerkId)
	case "user.deleted":
		h.logger.Info("User deleted")
	case "user.updated":
		h.logger.Info("User updated")
	default:
		h.logger.Error("unsupported event type", slog.String("event_type", event.Type))
		response.BadRequest(w, fmt.Sprintf("unsupported event type: %s", event.Type))
		return
	}

	response.WriteJson(w, http.StatusNoContent, nil)

}
