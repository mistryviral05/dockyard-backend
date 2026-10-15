package agentd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

const (
	heartbeatPath  = "/api/v1/agents/heartbeat"
	maxAttempts    = 3
	initialBackoff = time.Second
	maxBackoff     = 8 * time.Second
	errBodyLimit   = 1 << 10
)

var ErrUnauthorized = errors.New("control plane rejected the agent token")

type HeartbeatSender interface {
	SendHeartbeat(ctx context.Context, s *Snapshot) error
}

type permanentError struct{ err error }

func (p permanentError) Error() string { return p.err.Error() }
func (p permanentError) Unwrap() error { return p.err }

func permanent(err error) error { return permanentError{err: err} }

func IsPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

type ControlPlaneClient struct {
	baseURL  string
	token    string
	http     *http.Client
	attempts int
}

func NewControlPlaneClient(cfg *Config, token string) *ControlPlaneClient {
	return &ControlPlaneClient{
		baseURL:  strings.TrimRight(cfg.ControlPlaneURL, "/"),
		token:    token,
		http:     &http.Client{Timeout: cfg.ControlPlaneTimeout},
		attempts: maxAttempts,
	}
}

func (c *ControlPlaneClient) SendHeartbeat(ctx context.Context, s *Snapshot) error {
	if s == nil {
		return permanent(errors.New("send heartbeat: nil snapshot"))
	}

	body, err := json.Marshal(s)
	if err != nil {
		return permanent(fmt.Errorf("encode heartbeat: %w", err))
	}

	return c.post(ctx, c.baseURL+heartbeatPath, body)
}

func (c *ControlPlaneClient) post(ctx context.Context, url string, body []byte) error {
	var lastErr error

	for attempt := 1; attempt <= c.attempts; attempt++ {
		if attempt > 1 {
			if err := sleepContext(ctx, backoffFor(attempt)); err != nil {
				return err
			}
		}

		err := c.do(ctx, url, body)
		if err == nil {
			return nil
		}
		if IsPermanent(err) || ctx.Err() != nil {
			return err
		}

		lastErr = err
	}

	return fmt.Errorf("control plane unreachable after %d attempts: %w", c.attempts, lastErr)
}

func (c *ControlPlaneClient) do(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return permanent(fmt.Errorf("build request: %w", err))
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", "dockyard-agent/"+Version)

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("post %s: %w", url, err)
	}
	defer res.Body.Close()

	snippet := readSnippet(res.Body)

	switch {
	case res.StatusCode >= 200 && res.StatusCode < 300:
		return nil

	case res.StatusCode == http.StatusUnauthorized, res.StatusCode == http.StatusForbidden:
		return permanent(fmt.Errorf("%w: status %d: %s", ErrUnauthorized, res.StatusCode, snippet))

	case res.StatusCode == http.StatusTooManyRequests, res.StatusCode >= 500:
		return fmt.Errorf("control plane status %d: %s", res.StatusCode, snippet)

	default:
		return permanent(fmt.Errorf("control plane status %d: %s", res.StatusCode, snippet))
	}
}

func readSnippet(r io.Reader) string {
	data, err := io.ReadAll(io.LimitReader(r, errBodyLimit))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func backoffFor(attempt int) time.Duration {
	d := initialBackoff << (attempt - 2)
	if d > maxBackoff {
		d = maxBackoff
	}

	jitter := time.Duration(rand.Int63n(int64(d/5) + 1))
	return d + jitter
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
