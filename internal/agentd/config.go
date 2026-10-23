package agentd

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
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

	AgentDomain string `yaml:"agent_domain" env:"DOCKYARD_AGENT_DOMAIN"`

	TraefikNetwork string `yaml:"traefik_network" env:"DOCKYARD_TRAEFIK_NETWORK" env-default:"web"`

	CertResolver string `yaml:"cert_resolver" env:"DOCKYARD_CERT_RESOLVER" env-default:"le"`

	DeployTimeout time.Duration `yaml:"deploy_timeout" env:"DOCKYARD_DEPLOY_TIMEOUT" env-default:"15m"`

	MetricsEnabled bool `yaml:"metrics_enabled" env:"DOCKYARD_METRICS_ENABLED" env-default:"true"`

	DiskPath string `yaml:"disk_path" env:"DOCKYARD_DISK_PATH" env-default:"/"`

	ReadTimeout  time.Duration `yaml:"read_timeout" env:"DOCKYARD_READ_TIMEOUT" env-default:"10s"`
	WriteTimeout time.Duration `yaml:"write_timeout" env:"DOCKYARD_WRITE_TIMEOUT" env-default:"30s"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" env:"DOCKYARD_IDLE_TIMEOUT" env-default:"60s"`

	// Env log format chunta hai: development me rangeen text, production me
	// JSON. Default production hai - user ke server pe wahi chalta hai, aur
	// galti se colour codes journald me chale jayein usse behtar hai ki dev me
	// ek line likhni pade.
	Env string `yaml:"env" env:"DOCKYARD_ENV" env-default:"production"`

	// LogFormat: auto | text | json. auto Env dekh kar faisla karta hai.
	LogFormat string `yaml:"log_format" env:"DOCKYARD_LOG_FORMAT" env-default:"auto"`

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

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		flags := flag.String("config", "", "Path to agent config file")
		flag.Parse()
		configPath = *flags
	}

	if configPath == "" {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return nil, fmt.Errorf("read agent config from environment: %w", err)
		}
	} else {
		if _, err := os.Stat(configPath); err != nil {
			return nil, fmt.Errorf("agent config file not readable: %w", err)
		}
		if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
			return nil, fmt.Errorf("read agent config from %s: %w", configPath, err)
		}
	}

	cfg.ControlPlaneURL = strings.TrimRight(strings.TrimSpace(cfg.ControlPlaneURL), "/")
	cfg.DiskPath = resolveDiskPath(cfg.DiskPath)

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func resolveDiskPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		p = "/"
	}

	if runtime.GOOS != "windows" || p != "/" {
		return p
	}

	if drive := os.Getenv("SystemDrive"); drive != "" {
		return drive + `\`
	}

	return `C:\`
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

	if c.MetricsEnabled && c.DiskPath == "" {
		return errors.New("DOCKYARD_DISK_PATH must not be empty when metrics are enabled")
	}

	return nil
}
