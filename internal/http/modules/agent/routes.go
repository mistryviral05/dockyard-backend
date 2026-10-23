package agent

import "net/http"

func RegisterRoutes(mux *http.ServeMux, h *Handler, auth func(http.Handler) http.Handler, base string) {
	const prefix = "/agents"
	prefixV1 := base + "/v1" + prefix

	mux.Handle("POST "+prefixV1, auth(http.HandlerFunc(h.Create)))
	mux.Handle("GET "+prefixV1, auth(http.HandlerFunc(h.Get)))
	mux.Handle("GET "+prefixV1+"/{id}", auth(http.HandlerFunc(h.GetByID)))
	mux.Handle("GET "+prefixV1+"/{id}/metrics", auth(http.HandlerFunc(h.GetMetrics)))

	mux.Handle("GET "+prefixV1+"/install.sh", http.HandlerFunc(h.InstallScript))
	mux.Handle("POST "+prefixV1+"/register", http.HandlerFunc(h.Register))
	mux.Handle("POST "+prefixV1+"/heartbeat", h.agentAuth(http.HandlerFunc(h.Heartbeat)))
	mux.Handle("POST "+prefixV1+"/deployments/{id}/callback", h.agentAuth(http.HandlerFunc(h.DeploymentCallback)))

	mux.Handle("GET "+prefixV1+"/{id}/metrics/bench", http.HandlerFunc(h.GetMetricsBench))
}
