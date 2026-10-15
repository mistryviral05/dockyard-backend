package agent

import "net/http"

func RegisterRoutes(mux *http.ServeMux, h *Handler, auth func(http.Handler) http.Handler, base string) {
	const prefix = "/agents"
	prefixV1 := base + "/v1" + prefix

	mux.Handle("POST "+prefixV1, auth(http.HandlerFunc(h.Create)))
	mux.Handle("GET "+prefixV1, auth(http.HandlerFunc(h.Get)))
	mux.Handle("GET "+prefixV1+"/{id}", auth(http.HandlerFunc(h.GetByID)))

	mux.Handle("POST "+prefixV1+"/heartbeat", h.agentAuth(http.HandlerFunc(h.Heartbeat)))
}
