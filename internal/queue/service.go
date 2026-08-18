package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/mistryviral05/dockyard/internal/http/modules/deployment"
	"github.com/mistryviral05/dockyard/internal/http/utils/gh"
)

var (
	fakeBuildDuration  = 5 * time.Second
	fakeDeployDuration = 2 * time.Second
)

type Service struct {
	repo     deployment.Repository
	ghClient *gh.Client
	logger   *slog.Logger
}

func NewService(repo deployment.Repository, ghClient *gh.Client, logger *slog.Logger) *Service {
	return &Service{repo: repo, ghClient: ghClient, logger: logger}
}

func (s *Service) DeployBuild(ctx context.Context, deploymentId string) error {

	claimed, err := s.repo.Transition(ctx, deploymentId, deployment.StatusBuilding, nil)
	if err != nil {
		return err
	}
	if !claimed {
		// row queued me nahi hai: kisi aur ne le li, ya cancel ho chuki hai
		s.logger.Warn("build skipped, deployment is not queued",
			slog.String("deployment_id", deploymentId),
		)
		return nil
	}

	s.logger.Info("build started", slog.String("deployment_id", deploymentId))

	if err := sleep(ctx, fakeBuildDuration); err != nil {
		return s.markFailed(deploymentId, "the build was cancelled")
	}

	imageTag := "local.fake/dockyard/" + deploymentId + ":latest"
	if _, err := s.repo.Transition(ctx, deploymentId, deployment.StatusDeploying, map[string]interface{}{
		"image_tag": imageTag,
	}); err != nil {
		return s.markFailed(deploymentId, "the deployment could not be started")
	}

	s.logger.Info("deploying",
		slog.String("deployment_id", deploymentId),
		slog.String("image_tag", imageTag),
	)

	if err := sleep(ctx, fakeDeployDuration); err != nil {
		return s.markFailed(deploymentId, "the deployment was cancelled")
	}

	if _, err := s.repo.Transition(ctx, deploymentId, deployment.StatusRunning, map[string]interface{}{
		"container_id": "fake_container_" + deploymentId,
	}); err != nil {
		return err
	}

	s.logger.Info("deployment is running", slog.String("deployment_id", deploymentId))
	return nil
}

// markFailed ke waqt job ka ctx mar chuka hota hai, isliye naya ctx.
func (s *Service) markFailed(deploymentID, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := s.repo.Transition(ctx, deploymentID, deployment.StatusFailed, map[string]interface{}{
		"error_message": message,
	}); err != nil {
		s.logger.Error("failed to mark the deployment as failed",
			slog.String("deployment_id", deploymentID),
			slog.String("error", err.Error()),
		)
	}

	return nil
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
