package github

import (
	"log/slog"
	"net/http"

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

func (h *Handler) CreateInstallation(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.ClerkIDFromContext(r.Context())
	link, err := h.service.CreateInstallation(userID)
	if err != nil {
		h.logger.Error("failed to create installation link", slog.String("error", err.Error()))
		response.InternalError(w, "failed to login with github")
	}
	rp := map[string]string{
		"link": link,
	}
	response.WriteJson(w, http.StatusOK, response.GeneralData(rp))

}
