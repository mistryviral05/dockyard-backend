package agentd

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/mistryviral05/dockyard/internal/dispatch"
)

const containerPrefix = "dockyard-app-"

type Docker struct {
	network      string
	certResolver string
	log          *slog.Logger
}

func NewDocker(cfg *Config, log *slog.Logger) *Docker {
	return &Docker{
		network:      cfg.TraefikNetwork,
		certResolver: cfg.CertResolver,
		log:          log,
	}
}

func ContainerName(projectID string) string {
	return containerPrefix + projectID
}

func (d *Docker) Deploy(ctx context.Context, cmd *dispatch.Command) (string, error) {
	if err := d.login(ctx, cmd); err != nil {
		return "", err
	}
	defer d.logout(cmd.RegistryHost)

	if err := d.pull(ctx, cmd.Image); err != nil {
		return "", err
	}

	name := ContainerName(cmd.ProjectID)
	d.remove(ctx, name)

	id, err := d.run(ctx, name, cmd)
	if err != nil {
		return "", err
	}

	return id, nil
}

func (d *Docker) login(ctx context.Context, cmd *dispatch.Command) error {
	if cmd.RegistryToken == "" {
		d.log.Debug("no registry token in the command, skipping login")
		return nil
	}

	c := exec.CommandContext(ctx, "docker", "login", cmd.RegistryHost,
		"-u", cmd.RegistryUser, "--password-stdin")
	c.Stdin = strings.NewReader(cmd.RegistryToken)

	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("registry login failed: %s", firstLine(out))
	}

	return nil
}

func (d *Docker) logout(host string) {
	if host == "" {
		return
	}
	_ = exec.Command("docker", "logout", host).Run()
}

func (d *Docker) pull(ctx context.Context, image string) error {
	out, err := exec.CommandContext(ctx, "docker", "pull", image).CombinedOutput()
	if err != nil {
		return fmt.Errorf("image pull failed: %s", firstLine(out))
	}
	return nil
}

func (d *Docker) remove(ctx context.Context, name string) {
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", name).Run()
}

func (d *Docker) run(ctx context.Context, name string, cmd *dispatch.Command) (string, error) {
	port := strconv.Itoa(cmd.Port)
	router := strings.ReplaceAll(name, ".", "-")

	args := []string{
		"run", "-d",
		"--name", name,
		"--restart", "unless-stopped",
		"--network", d.network,
		"--label", "traefik.enable=true",
		"--label", fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", router, cmd.Domain),
		"--label", fmt.Sprintf("traefik.http.routers.%s.entrypoints=websecure", router),
		"--label", fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=%s", router, d.certResolver),
		"--label", fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%s", router, port),
		"--label", "dockyard.project=" + cmd.ProjectID,
		"--label", "dockyard.deployment=" + cmd.DeploymentID,
		"-e", "PORT=" + port,
	}

	for _, key := range sortedKeys(cmd.Env) {
		args = append(args, "-e", key+"="+cmd.Env[key])
	}

	args = append(args, cmd.Image)

	var stdout, stderr bytes.Buffer
	c := exec.CommandContext(ctx, "docker", args...)
	c.Stdout = &stdout
	c.Stderr = &stderr

	if err := c.Run(); err != nil {
		return "", fmt.Errorf("container start failed: %s", firstLine(stderr.Bytes()))
	}

	id := strings.TrimSpace(stdout.String())
	if len(id) > 12 {
		id = id[:12]
	}

	return id, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func firstLine(out []byte) string {
	text := strings.TrimSpace(string(out))
	if text == "" {
		return "no output"
	}

	lines := strings.Split(text, "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" {
		return strings.TrimSpace(lines[0])
	}

	return last
}
