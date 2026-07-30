package middleware

import (
	"context"
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
func RequireAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// clerkhttp verifies the token and puts SessionClaims in the context;
		// on an invalid/missing token it responds 401 itself.
		return clerkhttp.WithHeaderAuthorization()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
