package queue

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mistryviral05/dockyard/internal/dispatch"
	"github.com/mistryviral05/dockyard/internal/http/modules/deployment"
	"github.com/mistryviral05/dockyard/internal/http/utils/signer"
)

const (
	dispatchPath     = "/v1/deploy"
	dispatchTimeout  = 30 * time.Second
	dispatchAttempts = 5
	dispatchBackoff  = 6 * time.Second
	maxSlugLen       = 40
)

var (
	errDispatchRejected = errors.New("the agent rejected the deploy command")
	errAgentUnreachable = errors.New("the agent could not be reached")
)

func (s *Service) dispatch(ctx context.Context, deploymentId, imageTag string) (string, error) {
	target, err := s.repo.GetDispatchTarget(ctx, deploymentId)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(target.ImageTag) == "" {
		target.ImageTag = imageTag
	}

	domain := s.appDomain(target)

	cmd := dispatch.Command{
		DeploymentID:  target.DeploymentId,
		ProjectID:     target.ProjectId,
		Image:         target.ImageTag,
		Domain:        domain,
		Port:          target.AppPort,
		RegistryHost:  s.registry.RegistryHost,
		RegistryUser:  s.registry.RegistryUsername,
		RegistryToken: s.registry.RegistryToken,
	}

	token, err := signer.New(target.SigningSecret).Sign(cmd,
		time.Now().Add(dispatch.CommandTTLSeconds*time.Second))
	if err != nil {
		return "", fmt.Errorf("sign deploy command: %w", err)
	}

	url := "https://" + s.agentDomain(target) + dispatchPath

	s.logger.Info("dispatching deploy",
		slog.String("deployment_id", deploymentId),
		slog.String("agent_id", target.AgentId),
		slog.String("url", url),
		slog.String("app_domain", domain),
	)

	if err := s.postCommand(ctx, url, token); err != nil {
		return "", err
	}

	return domain, nil
}

func (s *Service) postCommand(ctx context.Context, url, token string) error {
	client := &http.Client{Timeout: dispatchTimeout}

	var lastErr error

	for attempt := 1; attempt <= dispatchAttempts; attempt++ {
		if attempt > 1 {
			if err := sleep(ctx, dispatchBackoff); err != nil {
				return err
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(nil))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		res, err := client.Do(req)
		if err != nil {
			lastErr = err
			s.logger.Warn("agent not reachable yet",
				slog.String("url", url),
				slog.Int("attempt", attempt),
				slog.String("error", err.Error()),
			)
			continue
		}

		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<10))
		res.Body.Close()

		if res.StatusCode == http.StatusAccepted || res.StatusCode == http.StatusOK {
			return nil
		}

		if res.StatusCode >= 500 {
			lastErr = fmt.Errorf("agent status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
			continue
		}

		return fmt.Errorf("%w: status %d: %s", errDispatchRejected,
			res.StatusCode, strings.TrimSpace(string(body)))
	}

	return fmt.Errorf("%w after %d attempts: %v", errAgentUnreachable, dispatchAttempts, lastErr)
}

func (s *Service) agentDomain(t *deployment.DispatchTarget) string {
	return t.AgentSubdomain + "." + s.baseDomain
}

func (s *Service) appDomain(t *deployment.DispatchTarget) string {
	slug := slugify(t.ProjectName)
	if slug == "" {
		slug = slugify(t.ProjectId)
	}

	return slug + "." + s.agentDomain(t)
}

func slugify(name string) string {
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
	if len(slug) > maxSlugLen {
		slug = strings.Trim(slug[:maxSlugLen], "-")
	}

	return slug
}
