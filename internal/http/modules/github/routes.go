package github

import "net/http"

func RegisterRoutes(mux *http.ServeMux, h *Handler, auth func(http.Handler) http.Handler, base string) {
	const prefix = "/github"
	prefixV1 := base + "/v1" + prefix

	mux.Handle("POST "+prefixV1+"/webhook", http.HandlerFunc(h.HandleWebhook))
	mux.Handle("POST "+prefixV1+"/installations", auth(http.HandlerFunc(h.CreateInstallation)))
	mux.Handle("GET "+prefixV1+"/installations", auth(http.HandlerFunc(h.GetInstallations)))
	mux.Handle("GET "+prefixV1+"/install-url", auth(http.HandlerFunc(h.GetInstallationLink)))
}
