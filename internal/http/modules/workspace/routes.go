package workspace

import "net/http"

// auth is middleware.RequireAuth(...) built at the composition root and injected here.
func RegisterRoutes(mux *http.ServeMux, h *Handler, auth func(http.Handler) http.Handler, base string) {
	const prefix = "/workspaces"
	prefixV1 := base + "/v1" + prefix

	// POST /api/v1/workspaces  (protected)
	mux.Handle("POST "+prefixV1, auth(http.HandlerFunc(h.Create)))
	mux.Handle("GET "+prefixV1, auth(http.HandlerFunc(h.Get)))
}
