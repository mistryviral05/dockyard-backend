package agentd

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	ControlPlaneURL string `yaml:"control_plane_url" env:"DOCKYARD_CONTROL_PLANE_URL" env-required:"true"`

	RegistrationToken string `yaml:"registration_token" env:"DOCKYARD_REGISTRATION_TOKEN"`

	Addr string `yaml:"addr" env:"DOCKYARD_AGENT_ADDR" env-default:"0.0.0.0:8999"`

	StateFile string `yaml:"state_file" env:"DOCKYARD_STATE_FILE" env-default:"/var/lib/dockyard/agent.json"`

	DockerHost string `yaml:"docker_host" env:"DOCKER_HOST" env-default:"unix:///var/run/docker.sock"`

	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" env:"DOCKYARD_HEARTBEAT_INTERVAL" env-default:"30s"`

	ControlPlaneTimeout time.Duration `yaml:"control_plane_timeout" env:"DOCKYARD_CONTROL_PLANE_TIMEOUT" env-default:"10s"`

	ImagePullTimeout time.Duration `yaml:"image_pull_timeout" env:"DOCKYARD_IMAGE_PULL_TIMEOUT" env-default:"10m"`

	ClockSkew time.Duration `yaml:"clock_skew" env:"DOCKYARD_CLOCK_SKEW" env-default:"5m"`

	ReadTimeout  time.Duration `yaml:"read_timeout" env:"DOCKYARD_READ_TIMEOUT" env-default:"10s"`
	WriteTimeout time.Duration `yaml:"write_timeout" env:"DOCKYARD_WRITE_TIMEOUT" env-default:"30s"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" env:"DOCKYARD_IDLE_TIMEOUT" env-default:"60s"`

	LogLevel string `yaml:"log_level" env:"DOCKYARD_LOG_LEVEL" env-default:"info"`
}

func MustLoad() *Config {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("agent config: %s", err.Error())
	}
	return cfg
}

func LoadConfig() (*Config, error) {
	var cfg Config

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("read agent config from environment: %w", err)
	}

	cfg.ControlPlaneURL = strings.TrimRight(strings.TrimSpace(cfg.ControlPlaneURL), "/")

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if !strings.HasPrefix(c.ControlPlaneURL, "http://") && !strings.HasPrefix(c.ControlPlaneURL, "https://") {
		return fmt.Errorf("DOCKYARD_CONTROL_PLANE_URL must start with http:// or https://, got %q", c.ControlPlaneURL)
	}

	if c.HeartbeatInterval <= 0 {
		return fmt.Errorf("DOCKYARD_HEARTBEAT_INTERVAL must be positive, got %s", c.HeartbeatInterval)
	}

	if c.ClockSkew <= 0 || c.ClockSkew > time.Hour {
		return fmt.Errorf("DOCKYARD_CLOCK_SKEW must be between 0 and 1h, got %s", c.ClockSkew)
	}

	return nil
}
