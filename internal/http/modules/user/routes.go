package user

import (
	"net/http"

	"github.com/mistryviral05/dockyard/internal/http/middleware"
)

func RegisterRoutes(mux *http.ServeMux, h *Handler, base string) {
	const prefix = "/users"
	prefixV1 := base + "/v1" + prefix

	webHookHandler := middleware.VerifyClerkWebhook(h.cfg, h.logger)(http.HandlerFunc(h.handleWebhook))
	mux.Handle("POST "+prefixV1+"/webhook/clerk", webHookHandler)
}
