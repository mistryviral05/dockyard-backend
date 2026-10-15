package agent

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
)

const (
	tokenPrefix      = "agt_"
	tokenByteSize    = 32
	defaultAgentPort = "8999"

	serverURL = "http://localhost:8080"
)

func GenerateToken() (plain string, hashed string, err error) {
	buf := make([]byte, tokenByteSize)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	plain = tokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	return plain, HashToken(plain), nil
}

func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func CompareToken(plain, storedHash string) bool {
	return subtle.ConstantTimeCompare([]byte(HashToken(plain)), []byte(storedHash)) == 1
}

func BuildRunCommand(token, port string) string {
	return fmt.Sprintf(
		"curl -fsSL %s/api/v1/agents/install.sh | bash -s -- --token %s --server %s --port %s",
		serverURL, token, serverURL, port,
	)
}

func parseMetricsQuery(values url.Values) (MetricsQuery, error) {
	var q MetricsQuery

	if raw := strings.TrimSpace(values.Get("window")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return q, fmt.Errorf("window must be a duration like 15m or 6h")
		}
		q.Window = d
	}

	if raw := strings.TrimSpace(values.Get("since")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return q, fmt.Errorf("since must be an RFC3339 timestamp")
		}
		q.Since = t.UTC()
	}

	return q, nil
}

func clampWindow(w time.Duration) time.Duration {
	switch {
	case w <= 0:
		return defaultWindow
	case w < minWindow:
		return minWindow
	case w > maxWindow:
		return maxWindow
	default:
		return w
	}
}

func bucketFor(window time.Duration) int {
	bucket := int(window.Seconds()) / targetPoints
	if bucket < 1 {
		return 1
	}
	return bucket
}

func seriesStart(window time.Duration, since time.Time) time.Time {
	oldest := time.Now().UTC().Add(-window)
	if since.IsZero() || since.Before(oldest) {
		return oldest
	}
	return since
}

func loadPerCore(load *float64, cores *int) *float64 {
	if load == nil || cores == nil || *cores <= 0 {
		return nil
	}

	v := math.Round((*load / float64(*cores) * 100)) / 100
	return &v
}

func normalizePort(p string) string {
	if p == "" {
		return defaultAgentPort
	}
	return p
}
