package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/mistryviral05/dockyard/internal/http/modules/deployment"
)

var (
	fakeBuildDuration  = 5 * time.Second
	fakeDeployDuration = 2 * time.Second
)

type Service struct {
	repo   deployment.Repository
	logger *slog.Logger
}

func NewService(repo deployment.Repository, logger *slog.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

func (s *Service) DeployBuild(ctx context.Context, deploymentId string) error {
	s.logger.Info("Deployment build started!")
	if err := s.repo.Transition(ctx, deploymentId, deployment.StatusRunning, nil); err != nil {
		return err
	}

	if err := sleep(ctx, fakeBuildDuration); err != nil {
		return s.markFailed(deploymentId, "build failed: "+err.Error())
	}

	imageTag := "local.fake/dockyard/" + deploymentId + ":latest"
	if err := s.repo.Transition(ctx, deploymentId, deployment.StatusDeploying, map[string]interface{}{
		"image_tag": imageTag,
	}); err != nil {
		return s.markFailed(deploymentId, "deploy failed: "+err.Error())
	}

	s.logger.Info("deploying",
		slog.String("deployment_id", deploymentId),
		slog.String("image_tag", imageTag),
	)
	if err := sleep(ctx, fakeDeployDuration); err != nil {
		return s.markFailed(deploymentId, "Deploying faild : "+err.Error())
	}
	if err := s.repo.Transition(ctx, deploymentId, deployment.StatusRunning, map[string]interface{}{
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

	if err := s.repo.Transition(ctx, deploymentID, deployment.StatusFailed, map[string]interface{}{
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
