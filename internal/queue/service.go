package queue

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mistryviral05/dockyard/internal/http/modules/deployment"
	"github.com/mistryviral05/dockyard/internal/http/utils/gh"
)

var (
	fakeBuildDuration  = 10 * time.Second
	fakeDeployDuration = 5 * time.Second
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

	buildDir, err := s.prepareBuildSource(ctx, deploymentId)
	if buildDir != "" {
		defer os.RemoveAll(buildDir)
	}
	if err != nil {
		s.logger.Error("The build source could not be prepared : " + err.Error())
		return s.markFailed(deploymentId, "the build source could not be prepared")
	}

	dockerfilePath, err := s.resolveDockerfile(ctx, buildDir, deploymentId)
	if err != nil {
		s.logger.Error("The Dockerfile could not be found : " + err.Error())
		return s.markFailed(deploymentId, "the Dockerfile could not be found")
	}

	s.logger.Info("Dockerfile found", slog.String("deployment_id", deploymentId), slog.String("dockerfile_path", dockerfilePath))

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

func (s *Service) prepareBuildSource(ctx context.Context, deploymentId string) (string, error) {
	buildCtx, err := s.repo.GetBuildContext(ctx, deploymentId)

	token, err := s.ghClient.InstallationToken(ctx, buildCtx.InstallationId)
	if err != nil {
		s.logger.Error("failed to mint the installation token",
			slog.String("deployment_id", deploymentId),
			slog.String("error", err.Error()),
		)
		return "", err
	}

	commit, err := s.ghClient.LatestCommit(ctx, buildCtx.InstallationId, buildCtx.RepoFullName, buildCtx.Branch)

	if err != nil {
		s.logger.Error("failed to read the branch",
			slog.String("deployment_id", deploymentId),
			slog.String("branch", buildCtx.Branch),
			slog.String("error", err.Error()),
		)
		return "", err
	}

	if err := s.repo.SetCommit(ctx, deploymentId, commit.Sha, commit.Message); err != nil {
		return "", err
	}

	s.logger.Info("commit resolved",
		slog.String("deployment_id", deploymentId),
		slog.String("sha", commit.Sha),
	)

	buildDir, err := s.clone(ctx, buildCtx, token)
	if err != nil {
		return buildDir, err
	}
	return buildDir, nil
}

func (s *Service) clone(ctx context.Context, b *deployment.BuildContext, token string) (string, error) {
	s.logger.Info("Starting Clonning the repository...")
	buildDir := filepath.Join(os.TempDir(), "dockyard", "build_"+b.DeploymentId)

	if err := os.MkdirAll(filepath.Dir(buildDir), 0o755); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", b.Branch, gh.CloneURL(token, b.RepoFullName), buildDir)
	out, err := cmd.CombinedOutput()

	if err != nil {
		s.logger.Error("git clone failed",
			slog.String("deployment_id", b.DeploymentId),
			slog.String("repo", b.RepoFullName),
			slog.String("output", gh.Redact(string(out), token)),
			slog.String("error", err.Error()),
		)
		return buildDir, err
	}

	entries, _ := os.ReadDir(buildDir)
	s.logger.Info("repository cloned",
		slog.String("deployment_id", b.DeploymentId),
		slog.String("repo", b.RepoFullName),
		slog.String("dir", buildDir),
		slog.Int("entries", len(entries)),
	)
	return buildDir, nil
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

func (s *Service) resolveDockerfile(ctx context.Context, buildDir string, deploymentId string) (string, error) {
	dockerfilePath := filepath.Join(buildDir, "Dockerfile")

	info, err := os.Stat(dockerfilePath)
	if err == nil && !info.IsDir() {
		s.logger.Info("using the repository Dockerfile",
			slog.String("deployment_id", deploymentId),
			slog.String("path", dockerfilePath),
		)
		return dockerfilePath, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return s.generateDockerfile(ctx, deploymentId, buildDir)
}

func (s *Service) generateDockerfile(ctx context.Context, deploymentId string, buildDir string) (string, error) {
	s.logger.Info("no Dockerfile in the repository, generating one",
		slog.String("deployment_id", deploymentId),
	)

	args := []string{
		"run", "--rm",
		"-v", buildDir + ":/app",
		"-w", "/app",
	}

	args = append(args, nixpacksImage, "build", "/app", "--out", "/app")

	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()

	if err != nil {
		s.logger.Error("nixpacks failed",
			slog.String("deployment_id", deploymentId),
			slog.String("output", string(out)),
			slog.String("error", err.Error()),
		)
		return "", errNoDockerfile
	}

	generated := filepath.Join(buildDir, ".nixpacks", "Dockerfile")
	if _, err := os.Stat(generated); err != nil {
		s.logger.Error("nixpacks produced no Dockerfile",
			slog.String("deployment_id", deploymentId),
			slog.String("output", string(out)),
		)
		return "", errNoDockerfile
	}

	s.logger.Info("dockerfile generated",
		slog.String("deployment_id", deploymentId),
		slog.String("path", generated),
	)

	return generated, nil

}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
