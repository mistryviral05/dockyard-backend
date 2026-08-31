package queue

import (
	"bufio"
	"context"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mistryviral05/dockyard/internal/config"
	"github.com/mistryviral05/dockyard/internal/http/modules/deployment"
	"github.com/mistryviral05/dockyard/internal/http/utils/centrifugo"
	"github.com/mistryviral05/dockyard/internal/http/utils/gh"
)

var (
	// fakeBuildDuration  = 60 * time.Second
	fakeDeployDuration = 15 * time.Second
)

type Service struct {
	repo       deployment.Repository
	ghClient   *gh.Client
	registry   *config.ContainerRegistry
	centrifugo *centrifugo.Client
	logger     *slog.Logger
}

func NewService(repo deployment.Repository, ghClient *gh.Client, registry *config.ContainerRegistry, realtime *centrifugo.Client, logger *slog.Logger) *Service {
	return &Service{
		repo:       repo,
		ghClient:   ghClient,
		registry:   registry,
		centrifugo: realtime,
		logger:     logger,
	}
}

func (s *Service) DeployBuild(ctx context.Context, deploymentId string) error {

	s.publishSystemLine(deploymentId, "deployment queued, waiting for a worker to pick it up")
	sleep(ctx, fakeDeployDuration)
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
	s.publishStatus(deploymentId, deployment.StatusBuilding, "")
	s.publishSystemLine(deploymentId, "build started, fetching your repository")

	buildSource, err := s.prepareBuildSource(ctx, deploymentId)
	if buildSource != nil {
		defer os.RemoveAll(buildSource.Dir)
	}
	if err != nil {
		s.logger.Error("The build source could not be prepared : " + err.Error())
		return s.markFailed(deploymentId, "the build source could not be prepared")
	}

	dockerfilePath, buildArgs, err := s.resolveDockerfile(ctx, buildSource.Dir, deploymentId)
	if err != nil {
		s.logger.Error("failed to resolve the dockerfile",
			slog.String("deployment_id", deploymentId),
			slog.String("error", err.Error()),
		)

		if errors.Is(err, errNoDockerfile) {
			return s.markFailed(deploymentId, "could not detect how to build this project, add a Dockerfile to your repository")
		}

		return s.markFailed(deploymentId, "the build could not be prepared")
	}
	buildSource.Dockerfile = dockerfilePath
	buildSource.BuildArgs = buildArgs

	s.logger.Info("Dockerfile found", slog.String("deployment_id", deploymentId), slog.String("dockerfile_path", buildSource.Dockerfile))

	imageTag, err := s.buildImage(ctx, deploymentId, buildSource)
	if err != nil {
		if errors.Is(err, errRegistryLogin) {
			return s.markFailed(deploymentId, "the image registry is unreachable right now, please try again")
		}

		s.logger.Error("the image could not be built",
			slog.String("deployment_id", deploymentId),
			slog.String("error", err.Error()),
		)
		return s.markFailed(deploymentId, "the image could not be built, check the build logs")
	}
	if _, err := s.repo.Transition(ctx, deploymentId, deployment.StatusDeploying, map[string]interface{}{
		"image_tag": imageTag,
	}); err != nil {
		return s.markFailed(deploymentId, "the deployment could not be started")
	}

	s.logger.Info("deploying",
		slog.String("deployment_id", deploymentId),
		slog.String("image_tag", imageTag),
	)
	s.publishStatus(deploymentId, deployment.StatusDeploying, "")
	s.publishSystemLine(deploymentId, "image pushed, starting your app")

	if err := sleep(ctx, fakeDeployDuration); err != nil {
		return s.markFailed(deploymentId, "the deployment was cancelled")
	}

	if _, err := s.repo.Transition(ctx, deploymentId, deployment.StatusRunning, map[string]interface{}{
		"container_id": "fake_container_" + deploymentId,
	}); err != nil {
		return err
	}

	s.logger.Info("deployment is running", slog.String("deployment_id", deploymentId))
	s.publishStatus(deploymentId, deployment.StatusRunning, "")
	s.publishSystemLine(deploymentId, "deployment is running")
	return nil
}

func (s *Service) prepareBuildSource(ctx context.Context, deploymentId string) (*BuildSource, error) {
	buildCtx, err := s.repo.GetBuildContext(ctx, deploymentId)

	if err != nil {
		s.logger.Error("failed to load the build context",
			slog.String("deployment_id", deploymentId),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	token, err := s.ghClient.InstallationToken(ctx, buildCtx.InstallationId)
	if err != nil {
		s.logger.Error("failed to mint the installation token",
			slog.String("deployment_id", deploymentId),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	commit, err := s.ghClient.LatestCommit(ctx, buildCtx.InstallationId, buildCtx.RepoFullName, buildCtx.Branch)

	if err != nil {
		s.logger.Error("failed to read the branch",
			slog.String("deployment_id", deploymentId),
			slog.String("branch", buildCtx.Branch),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if err := s.repo.SetCommit(ctx, deploymentId, commit.Sha, commit.Message); err != nil {
		return nil, err
	}

	s.logger.Info("commit resolved",
		slog.String("deployment_id", deploymentId),
		slog.String("sha", commit.Sha),
	)

	buildDir, err := s.clone(ctx, buildCtx, token)
	if err != nil {
		// adhoora clone bhi disk pe pada reh sakta hai, caller ko cleanup ka mauka do
		return &BuildSource{Dir: buildDir}, err
	}

	s.writeDockerignore(deploymentId, buildDir)

	return &BuildSource{
		Dir:    buildDir,
		Commit: commit,
		Build:  buildCtx,
	}, nil
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

// nixpacks ka Dockerfile "COPY . /app" teen baar karta hai, isliye context ka
// har extra byte teen layers me multiply hota hai. Repo ka apna .dockerignore
// ho to usko haath nahi lagate. Fail hone pe build rokna nahi hai.
func (s *Service) writeDockerignore(deploymentId, buildDir string) {
	path := filepath.Join(buildDir, ".dockerignore")

	if _, err := os.Stat(path); err == nil {
		return
	}

	if err := os.WriteFile(path, []byte(defaultDockerignore), 0o644); err != nil {
		s.logger.Warn("could not write the .dockerignore",
			slog.String("deployment_id", deploymentId),
			slog.String("error", err.Error()),
		)
	}
}

// markFailed ke waqt job ka ctx mar chuka hota hai, isliye naya ctx.
func (s *Service) markFailed(deploymentID, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// wahi message jo db me jaata hai, wahi live view me bhi dikhe
	s.publishSystemLine(deploymentID, message)

	if _, err := s.repo.Transition(ctx, deploymentID, deployment.StatusFailed, map[string]interface{}{
		"error_message": message,
	}); err != nil {
		s.logger.Error("failed to mark the deployment as failed",
			slog.String("deployment_id", deploymentID),
			slog.String("error", err.Error()),
		)
	}

	s.publishStatus(deploymentID, deployment.StatusFailed, message)

	return nil
}

// dusra return value sirf nixpacks wale raaste me bharta hai; repo ka apna
// Dockerfile ho to build args ki zaroorat hi nahi.
func (s *Service) resolveDockerfile(ctx context.Context, buildDir string, deploymentId string) (string, map[string]string, error) {
	dockerfilePath := filepath.Join(buildDir, "Dockerfile")

	info, err := os.Stat(dockerfilePath)
	if err == nil && !info.IsDir() {
		s.logger.Info("using the repository Dockerfile",
			slog.String("deployment_id", deploymentId),
			slog.String("path", dockerfilePath),
		)
		return dockerfilePath, nil, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", nil, err
	}
	return s.generateDockerfile(ctx, deploymentId, buildDir)
}

func (s *Service) generateDockerfile(ctx context.Context, deploymentId string, buildDir string) (string, map[string]string, error) {
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
		reason := nixpacksReason(string(out))

		s.logger.Error("nixpacks failed",
			slog.String("deployment_id", deploymentId),
			slog.String("output", string(out)),
			slog.String("reason", reason),
			slog.String("error", err.Error()),
		)

		return "", nil, fmt.Errorf("%w: %s", errNoDockerfile, reason)
	}

	generated := filepath.Join(buildDir, ".nixpacks", "Dockerfile")
	if _, err := os.Stat(generated); err != nil {
		s.logger.Error("nixpacks produced no Dockerfile",
			slog.String("deployment_id", deploymentId),
			slog.String("output", string(out)),
		)
		return "", nil, errNoDockerfile
	}

	vars, err := s.nixpacksVariables(ctx, deploymentId, buildDir)
	if err != nil {
		return "", nil, err
	}

	s.logger.Info("dockerfile generated",
		slog.String("deployment_id", deploymentId),
		slog.String("path", generated),
		slog.Int("build_args", len(vars)),
	)

	return generated, vars, nil

}


func (s *Service) nixpacksVariables(ctx context.Context, deploymentId string, buildDir string) (map[string]string, error) {
	args := []string{
		"run", "--rm",
		"-v", buildDir + ":/app",
		"-w", "/app",
		nixpacksImage, "plan", "/app",
	}

	// plan JSON stdout pe aata hai aur baaki shor stderr pe, isliye
	// CombinedOutput nahi - wo JSON ko ganda kar deta.
	out, err := exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		var stderr string
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr = string(exitErr.Stderr)
		}

		s.logger.Error("nixpacks plan failed",
			slog.String("deployment_id", deploymentId),
			slog.String("output", stderr),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("nixpacks plan: %w", err)
	}

	var plan struct {
		Variables map[string]string `json:"variables"`
	}
	if err := json.Unmarshal(out, &plan); err != nil {
		s.logger.Error("nixpacks plan could not be parsed",
			slog.String("deployment_id", deploymentId),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("nixpacks plan: %w", err)
	}

	return plan.Variables, nil
}

func (s *Service) imageTag(src *BuildSource) string {
	sha := src.Commit.Sha
	if len(sha) > 12 {
		sha = sha[:12]
	}

	return fmt.Sprintf("%s/%s/%s:%s",
		s.registry.RegistryHost,
		strings.ToLower(s.registry.RegistryNamespace),
		imageName(src.Build.ProjectId),
		sha,
	)
}

// registry repo name sirf lowercase leta hai aur separator ke strict rules
// rakhta hai. nanoid me uppercase, '-' aur '_' kahin bhi aa sakte hain,
// isliye use base32-hex me encode karke safe naam banate hain.
func imageName(projectId string) string {
	enc := base32.HexEncoding.WithPadding(base32.NoPadding)
	return "proj-" + strings.ToLower(enc.EncodeToString([]byte(projectId)))
}

func (s *Service) stream(_ context.Context, deploymentId string, cmd *exec.Cmd) error {
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	// stream ke bahar nikalne se pehle bacha hua batch nikal jaana chahiye,
	// warna failure ki aakhri lines user ko kabhi nahi dikhengi
	logs := s.newLogStream(deploymentId, LogTypeBuild)
	defer logs.Close()

	sc := bufio.NewScanner(pipe)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		if err := sc.Err(); err != nil {
			s.logger.Error("Failed to build output :" + err.Error())
			return err
		}

		line := sc.Text()
		logs.Add(line)
		s.logger.Info("build",
			slog.String("deployment_id", deploymentId),
			slog.String("line", line),
		)
	}

	return cmd.Wait()
}

// dockerLogin har build ke liye alag docker config folder banata hai. Global
// ~/.docker/config.json share karne se parallel builds ek doosre ka credential
// overwrite kar dete hain, aur ek kharab entry poori machine ke pulls tod deti hai.
func (s *Service) dockerLogin(ctx context.Context, deploymentId string) (string, error) {
	cfgDir, err := os.MkdirTemp("", "dockyard-docker-")
	if err != nil {
		return "", err
	}

	// local/anonymous registry: username khali hai to login ki zaroorat nahi
	if s.registry.RegistryUsername == "" {
		return cfgDir, nil
	}

	cmd := exec.CommandContext(ctx, "docker", "--config", cfgDir,
		"login", s.registry.RegistryHost,
		"-u", s.registry.RegistryUsername,
		"--password-stdin",
	)
	cmd.Stdin = strings.NewReader(s.registry.RegistryToken)

	out, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(cfgDir)
		s.logger.Error("registry login failed",
			slog.String("deployment_id", deploymentId),
			slog.String("registry", s.registry.RegistryHost),
			slog.String("output", gh.Redact(string(out), s.registry.RegistryToken)),
			slog.String("error", err.Error()),
		)
		return "", errRegistryLogin
	}

	return cfgDir, nil
}

func (s *Service) buildImage(ctx context.Context, deploymentId string, src *BuildSource) (string, error) {
	tag := s.imageTag(src)

	cfgDir, err := s.dockerLogin(ctx, deploymentId)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(cfgDir)

	s.logger.Info("building the image",
		slog.String("deployment_id", deploymentId),
		slog.String("image_tag", tag),
	)

	args := []string{
		"--config", cfgDir,
		"buildx", "build",
		"--progress", "plain",
		"--provenance=false",
	}

	// nixpacks ke Dockerfile ko plan variables chahiye. Repo ka apna Dockerfile
	// ho to ye map nil hota hai aur loop chalta hi nahi.
	for name, value := range src.BuildArgs {
		args = append(args, "--build-arg", name+"="+value)
	}

	args = append(args,
		"-f", src.Dockerfile,
		"-t", tag,
		"--push",
		src.Dir,
	)

	cmd := exec.CommandContext(ctx, "docker", args...)

	if err := s.stream(ctx, deploymentId, cmd); err != nil {
		return "", err
	}

	return tag, nil
}

func nixpacksReason(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "Error: "); ok {
			return after
		}
	}

	return "could not detect how to build this project"
}
func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
