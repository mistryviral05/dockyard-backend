package deployment

import "net/http"

func RegisterRoutes(mux *http.ServeMux, h *Handler, auth func(http.Handler) http.Handler, base string) {
	prefixV1 := base + "/v1"

	mux.Handle("POST "+prefixV1+"/projects/{id}/deploy", auth(http.HandlerFunc(h.Deploy)))
	mux.Handle("GET "+prefixV1+"/projects/{id}/deployments", auth(http.HandlerFunc(h.List)))
	mux.Handle("GET "+prefixV1+"/deployments/{id}", auth(http.HandlerFunc(h.Get)))
	mux.Handle("GET "+prefixV1+"/deployments/{id}/logs", auth(http.HandlerFunc(h.Logs)))
}
