package realtime

import "net/http"

func RegisterRoutes(mux *http.ServeMux, h *Handler, auth func(http.Handler) http.Handler, base string) {
	const prefix = "/centrifugo"
	prefixV1 := base + "/v1" + prefix

	// browser maangta hai, clerk auth ke peeche
	mux.Handle("POST "+prefixV1+"/token", auth(http.HandlerFunc(h.Token)))

	// centrifugo khud call karta hai, shared secret se protected.
	// Path centrifugo.json ke "proxy_subscribe_endpoint" se match karna chahiye.
	mux.Handle("POST "+prefixV1+"/subscribe", http.HandlerFunc(h.Subscribe))
}
