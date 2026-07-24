package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"

	"github.com/mistryviral05/dockyard/internal/config"
	"github.com/mistryviral05/dockyard/internal/http/utils/response"
	svix "github.com/svix/svix-webhooks/go"
)

func VerifyClerkWebhook(cfg *config.Config, logger *slog.Logger) func(next http.Handler) http.Handler {
	wh, err := svix.NewWebhook(cfg.Clerk.WebhookSecret)
	if err != nil {
		panic("failed to create Svix webhook verifier: " + err.Error())
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				response.BadRequest(w, "unable to read request body")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(payload))

			if err := wh.Verify(payload, r.Header); err != nil {
				logger.Error("clerk webhook signature verification failed", slog.String("error", err.Error()))
				response.Unauthorized(w, "invalid webhook signature")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
