package realtime

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/mistryviral05/dockyard/internal/http/middleware"
	"github.com/mistryviral05/dockyard/internal/http/utils/response"
)

// subscribeSecretHeader wahi header hai jo centrifugo.json ke
// "proxy_static_http_headers" me set hai.
const subscribeSecretHeader = "X-Centrifugo-Secret"

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// Token dashboard ko Centrifugo se connect hone ka token deta hai.
func (h *Handler) Token(w http.ResponseWriter, r *http.Request) {

	clerkID, ok := middleware.ClerkIDFromContext(r.Context())
	if !ok || clerkID == "" {
		response.Unauthorized(w, "unauthorized")
		return
	}

	res, err := h.service.ConnectionToken(clerkID)
	if err != nil {
		h.logger.Error("failed to sign the centrifugo token",
			slog.String("clerk_id", clerkID),
			slog.String("error", err.Error()),
		)
		response.InternalError(w, "live updates are unavailable right now, please try again")
		return
	}

	response.WriteJson(w, http.StatusOK, response.GeneralData(res))
}

// Subscribe Centrifugo khud call karta hai, browser nahi. Isliye ismein clerk
// auth nahi hai - iski jagah ek shared secret header hai.
//
// Jawab hamesha 200 jaata hai: Centrifugo allow aur deny dono body se padhta
// hai, aur non-200 ko apni internal error samajh ke connection tod deta hai.
func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {

	if !h.verifySecret(r) {
		h.logger.Warn("centrifugo subscribe called without a valid secret",
			slog.String("remote_addr", r.RemoteAddr),
		)
		writeProxy(w, false)
		return
	}

	var req subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("failed to decode the subscribe proxy body",
			slog.String("error", err.Error()),
		)
		writeProxy(w, false)
		return
	}

	allowed := h.service.CanSubscribe(r.Context(), req.User, req.Channel)

	h.logger.Info("subscribe checked",
		slog.String("clerk_id", req.User),
		slog.String("channel", req.Channel),
		slog.Bool("allowed", allowed),
	)

	writeProxy(w, allowed)
}

// verifySecret constant time compare karta hai. Secret khali ho to check off
// hai - local dev me Centrifugo ke bina bhi chal jaye.
func (h *Handler) verifySecret(r *http.Request) bool {
	want := h.service.cfg.SubscribeSecret
	if want == "" {
		return true
	}

	got := r.Header.Get(subscribeSecretHeader)

	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func writeProxy(w http.ResponseWriter, allowed bool) {
	body := subscribeResponse{}
	if allowed {
		body.Result = &subscribeResult{}
	} else {
		body.Error = &proxyError{Code: 403, Message: "permission denied"}
	}

	response.WriteJson(w, http.StatusOK, body)
}
