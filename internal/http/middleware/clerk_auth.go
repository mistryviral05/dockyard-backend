package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/mistryviral05/dockyard/internal/http/utils/response"
)

type ctxKey string

const clerkIDKey ctxKey = "clerk_id"

func RequireAuth(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				response.Unauthorized(w, "missing bearer token")
				return
			}

			token := strings.TrimPrefix(auth, "Bearer ")

			// Decode token to get KID
			decoded, err := jwt.Decode(r.Context(), &jwt.DecodeParams{
				Token: token,
			})
			if err != nil {
				logger.Error("decode failed", slog.Any("err", err))
				response.Unauthorized(w, "invalid token")
				return
			}

			// Fetch JWK
			jwk, err := jwt.GetJSONWebKey(r.Context(), &jwt.GetJSONWebKeyParams{
				KeyID: decoded.KeyID,
			})
			if err != nil {
				logger.Error("jwk fetch failed", slog.Any("err", err))
				response.Unauthorized(w, "invalid token")
				return
			}

			// Verify
			claims, err := jwt.Verify(r.Context(), &jwt.VerifyParams{
				Token: token,
				JWK:   jwk,
			})
			if err != nil {
				logger.Error("verify failed", slog.Any("err", err))
				response.Unauthorized(w, "invalid token")
				return
			}

			logger.Info("verified",
				slog.String("sub", claims.Subject),
			)

			ctx := context.WithValue(r.Context(), clerkIDKey, claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ClerkIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(clerkIDKey).(string)
	return id, ok
}
