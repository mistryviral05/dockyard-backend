package github

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"
	gogithub "github.com/google/go-github/v66/github"
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

func (h *Handler) GetRepositories(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.ClerkIDFromContext(r.Context())
	instRowId := r.PathValue("id")

	if instRowId == "" {
		response.BadRequest(w, "missing installation id")
		return
	}
	repositories, err := h.service.GetRepositories(r.Context(), instRowId, userID)
	if err != nil {
		if errors.Is(err, ErrInstallationNotFound) {
			h.logger.Error("failed to get repositories", slog.String("error", err.Error()))
			response.NotFound(w, "installation not found")
			return
		}

		h.logger.Error("failed to get repositories", slog.String("error", err.Error()))
		response.InternalError(w, "failed to get repositories")
		return
	}

	response.WriteJson(w, http.StatusOK, response.GeneralData(repositories))
}

func (h *Handler) GetInstallations(w http.ResponseWriter, r *http.Request) {

	userID, _ := middleware.ClerkIDFromContext(r.Context())

	installations, err := h.service.GetInstallations(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get installations", slog.String("error", err.Error()))
		response.InternalError(w, "failed to get installations")
		return
	}

	response.WriteJson(w, http.StatusOK, response.GeneralData(installations))

}

// HandleWebhook is the only unauthenticated route in this module. GitHub's
// HMAC signature is what stands in for auth, so nothing is read from the body
// until that check passes.
//
// Every answer except a bad signature or a broken database is 202. GitHub
// flags a webhook as failing on non-2xx and will keep redelivering it, and a
// push we deliberately ignored is not something a retry can improve.
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := h.service.VerifyWebhook(r)
	if err != nil {
		h.logger.Warn("rejected github webhook", slog.String("error", err.Error()))
		response.Unauthorized(w, "invalid webhook signature")
		return
	}

	eventType := gogithub.WebHookType(r)
	deliveryId := gogithub.DeliveryID(r)

	event, err := gogithub.ParseWebHook(eventType, payload)
	if err != nil {
		h.logger.Error("failed to parse github webhook",
			slog.String("event", eventType),
			slog.String("delivery_id", deliveryId),
			slog.String("error", err.Error()),
		)
		response.BadRequest(w, "invalid webhook payload")
		return
	}

	push, ok := event.(*gogithub.PushEvent)
	if !ok {
		// The app is subscribed to more than push; the rest is not acted on yet.
		h.logger.Debug("ignored github webhook", slog.String("event", eventType))
		response.WriteJson(w, http.StatusAccepted, response.GeneralMessage("event ignored"))
		return
	}

	outcome, err := h.service.HandlePush(r.Context(), push, deliveryId, payload)
	if err != nil {
		h.logger.Error("failed to handle push webhook",
			slog.String("delivery_id", deliveryId),
			slog.String("repo", push.GetRepo().GetFullName()),
			slog.String("error", err.Error()),
		)
		response.InternalError(w, "failed to handle webhook")
		return
	}

	if outcome.Ignored {
		h.logger.Info("push ignored",
			slog.String("delivery_id", deliveryId),
			slog.String("repo", push.GetRepo().GetFullName()),
			slog.String("ref", push.GetRef()),
			slog.String("reason", outcome.Reason),
		)
		response.WriteJson(w, http.StatusAccepted, response.GeneralMessage("push ignored: "+outcome.Reason))
		return
	}

	h.logger.Info("deployment queued from push",
		slog.String("delivery_id", deliveryId),
		slog.String("repo", push.GetRepo().GetFullName()),
		slog.String("ref", push.GetRef()),
		slog.String("deployment_id", outcome.DeploymentId),
	)
	response.WriteJson(w, http.StatusAccepted, response.GeneralData(map[string]string{
		"deployment_id": outcome.DeploymentId,
	}))
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
