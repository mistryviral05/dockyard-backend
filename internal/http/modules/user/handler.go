package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/mistryviral05/dockyard/internal/http/utils/response"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func newHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (h *Handler) handleWebhook(w http.ResponseWriter, r *http.Request) {

	var event ClerkEvent

	err := json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		if errors.Is(err, io.EOF) {
			response.WriteJson(w, http.StatusBadRequest, response.GeneralError(fmt.Errorf("request body cannot be empty")))
			return
		}
		response.WriteJson(w, http.StatusBadRequest, response.GeneralError(fmt.Errorf("invalid request body")))
		return
	}

	switch event.Type {
	case "user.created":
		var userData ClerkUserData
		if err := json.Unmarshal(event.Data, &userData); err != nil {
			response.WriteJson(w, http.StatusBadRequest, response.GeneralError(fmt.Errorf("invalid user data")))
			return
		}
		dto := userData.ToCreateUserDto()
		h.logger.Info("User created", slog.Any("user", dto))
	case "user.deleted":
		h.logger.Info("User deleted")
	case "user.updated":
		h.logger.Info("User updated")
	default:
		h.logger.Error("unsupported event type", slog.String("event_type", event.Type))
		response.WriteJson(w, http.StatusBadRequest, response.GeneralError(fmt.Errorf("unsupported event type: %s", event.Type)))
	}

	response.WriteJson(w, http.StatusNoContent, nil)

}
