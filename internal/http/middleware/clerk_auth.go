package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	"github.com/mistryviral05/dockyard/internal/http/utils/response"
)

type ctxKey string

const clerkIDKey ctxKey = "clerk_id"

// RequireAuth verifies the Clerk session token (Authorization: Bearer <token>)
// and stores the Clerk user id (JWT subject) in the request context.
// No database access.
func RequireAuth(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		authMiddleware := clerkhttp.WithHeaderAuthorization(
			clerkhttp.AuthorizationFailureHandler(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					logger.Error("clerk token rejected",
						slog.String("authorization", r.Header.Get("Authorization")),
					)
					response.Unauthorized(w, "invalid or expired token")
				}),
			),
		)

		return authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := clerk.SessionClaimsFromContext(r.Context())
			if !ok || claims.Subject == "" {
				response.Unauthorized(w, "You are not authorized to access this resource")
				return
			}

			ctx := context.WithValue(r.Context(), clerkIDKey, claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		}))
	}
}

// ClerkIDFromContext returns the authenticated Clerk user id set by RequireAuth.
func ClerkIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(clerkIDKey).(string)
	return id, ok
}
