package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

const requestIDKey ctxKey = "request_id"

// responseRecorder wraps http.ResponseWriter to capture the status code and
// the number of bytes written, which are not otherwise observable.
type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (rec *responseRecorder) WriteHeader(status int) {
	if rec.status == 0 {
		rec.status = status
	}
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *responseRecorder) Write(b []byte) (int, error) {
	if rec.status == 0 {
		rec.status = http.StatusOK // handler wrote a body without calling WriteHeader
	}
	n, err := rec.ResponseWriter.Write(b)
	rec.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer, so
// flushing / hijacking (SSE, websockets) keeps working through the wrapper.
func (rec *responseRecorder) Unwrap() http.ResponseWriter { return rec.ResponseWriter }

// RequestLogger logs one line per HTTP request once the handler has returned:
// method, path, status, duration, client ip and a per-request id.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			reqID := r.Header.Get("X-Request-Id")
			if reqID == "" {
				reqID = newRequestID()
			}
			w.Header().Set("X-Request-Id", reqID)

			ctx := context.WithValue(r.Context(), requestIDKey, reqID)
			r = r.WithContext(ctx)

			rec := &responseRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)

			if rec.status == 0 {
				rec.status = http.StatusOK // handler wrote nothing at all
			}

			attrs := []slog.Attr{
				slog.String("request_id", reqID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Duration("duration", time.Since(start)),
				slog.Int("bytes", rec.bytes),
				slog.String("ip", clientIP(r)),
			}
			if q := r.URL.RawQuery; q != "" {
				attrs = append(attrs, slog.String("query", q))
			}

			level := slog.LevelInfo
			switch {
			case rec.status >= 500:
				level = slog.LevelError
			case rec.status >= 400:
				level = slog.LevelWarn
			}

			logger.LogAttrs(r.Context(), level, "http request", attrs...)
		})
	}
}

// RequestIDFromContext returns the id assigned by RequestLogger.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey).(string)
	return id, ok
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		// left-most entry is the original client
		first, _, _ := strings.Cut(fwd, ",")
		return strings.TrimSpace(first)
	}
	if ip := r.Header.Get("X-Real-Ip"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
