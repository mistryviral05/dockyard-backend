package user

import "net/http"

func RegisterRoutes(mux *http.ServeMux, h *Handler, base string) {
	const prefix = "/users"
	prefixV1 := base + "/v1" + prefix
	mux.HandleFunc("POST "+prefixV1+"/webhook/clerk", h.handleWebhook)
}
