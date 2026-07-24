package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/mistryviral05/dockyard/internal/http/utils/response"
)

func Recover(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered",
						slog.Any("error", err),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.String("stack", string(debug.Stack())),
					)
					response.InternalError(w, "Internal server error!")
				}

			}()
			next.ServeHTTP(w, r)
		})
	}
}
