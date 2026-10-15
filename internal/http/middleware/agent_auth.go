package middleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/mistryviral05/dockyard/internal/http/utils/response"
)

const agentIDKey ctxKey = "agent_id"

type AgentAuthenticator interface {
	AuthenticateAgent(ctx context.Context, plainToken string) (agentID string, ok bool, err error)
}

func RequireAgentAuth(logger *slog.Logger, authenticator AgentAuthenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				response.Unauthorized(w, "missing bearer token")
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
			if token == "" {
				response.Unauthorized(w, "missing bearer token")
				return
			}

			agentID, ok, err := authenticator.AuthenticateAgent(r.Context(), token)
			if err != nil {
				logger.Error("agent token lookup failed", slog.String("error", err.Error()))
				response.InternalError(w, "could not verify agent token")
				return
			}
			if !ok {
				logger.Warn("agent token rejected",
					slog.String("remote_addr", ClientIP(r)),
					slog.String("path", r.URL.Path),
				)
				response.Unauthorized(w, "invalid agent token")
				return
			}

			ctx := context.WithValue(r.Context(), agentIDKey, agentID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AgentIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(agentIDKey).(string)
	return id, ok
}

func ClientIP(r *http.Request) string {
	if ip := firstForwardedFor(r.Header.Get("X-Forwarded-For")); ip != "" {
		return ip
	}
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func firstForwardedFor(header string) string {
	if header == "" {
		return ""
	}

	parts := strings.Split(header, ",")
	return strings.TrimSpace(parts[0])
}
