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

	signingSecretSize = 32
	maxSubdomainLen   = 40
	fallbackSubdomain = "agent"
)

var serverURL = "http://localhost:8040"

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

func SetServerURL(url string) {
	if url = strings.TrimRight(strings.TrimSpace(url), "/"); url != "" {
		serverURL = url
	}
}

func BuildRunCommand(token, port string) string {
	return fmt.Sprintf(
		"curl -fsSL %s/api/v1/agents/install.sh | sudo bash -s -- --token %s --server %s --port %s",
		serverURL, token, serverURL, port,
	)
}

func GenerateSigningSecret() (string, error) {
	buf := make([]byte, signingSecretSize)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate signing secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func Slugify(name string) string {
	var b strings.Builder

	previousDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			previousDash = false
		case !previousDash && b.Len() > 0:
			b.WriteByte('-')
			previousDash = true
		}
	}

	slug := strings.Trim(b.String(), "-")
	if len(slug) > maxSubdomainLen {
		slug = strings.Trim(slug[:maxSubdomainLen], "-")
	}

	return slug
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
