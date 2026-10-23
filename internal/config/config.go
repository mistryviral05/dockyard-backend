package config

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type ContainerRegistry struct {
	RegistryHost      string `yaml:"registry_host" env:"REGISTRY_HOST" env-required:"true"`
	RegistryNamespace string `yaml:"registry_namespace" env:"REGISTRY_NAMESPACE" env-required:"true"`
	RegistryUsername  string `yaml:"registry_username" env:"REGISTRY_USERNAME" env-required:"true"`
	RegistryToken     string `yaml:"registry_token" env:"REGISTRY_TOKEN" env-required:"true"`
}
type Centrifugo struct {
	APIUrl string `yaml:"api_url" env:"CENTRIFUGO_API_URL" env-required:"true"`
	WSUrl  string `yaml:"ws_url" env:"CENTRIFUGO_WS_URL" env-required:"true"`

	APIKey string `yaml:"api_key" env:"CENTRIFUGO_API_KEY" env-required:"true"`

	TokenSecret string `yaml:"token_secret" env:"CENTRIFUGO_TOKEN_SECRET" env-required:"true"`

	SubscribeSecret string `yaml:"subscribe_secret" env:"CENTRIFUGO_SUBSCRIBE_SECRET"`

	TokenTTL time.Duration `yaml:"token_ttl" env:"CENTRIFUGO_TOKEN_TTL" env-default:"1h"`
}

type Cloudflare struct {
	APIToken   string        `yaml:"api_token" env:"CLOUDFLARE_API_TOKEN" env-required:"true"`
	ZoneID     string        `yaml:"zone_id" env:"CLOUDFLARE_ZONE_ID" env-required:"true"`
	BaseDomain string        `yaml:"base_domain" env:"CLOUDFLARE_BASE_DOMAIN" env-required:"true"`
	RecordTTL  int           `yaml:"record_ttl" env:"CLOUDFLARE_RECORD_TTL" env-default:"60"`
	Timeout    time.Duration `yaml:"timeout" env:"CLOUDFLARE_TIMEOUT" env-default:"15s"`
}

type Redis struct {
	Addr     string `yaml:"addr" env:"REDIS_ADDR" env-required:"true"`
	Password string `yaml:"password" env:"REDIS_PASSWORD" env-required:"true"`
	DB       int    `yaml:"db" env:"REDIS_DB" env-default:"0"`
}

type HTTPServer struct {
	Addr         string        `yaml:"address" env:"HTTP_ADDRESS" env-required:"true"`
	ReadTimeout  time.Duration `yaml:"read_timeout" env:"HTTP_READ_TIMEOUT" env-default:"10s"`
	WriteTimeout time.Duration `yaml:"write_timeout" env:"HTTP_WRITE_TIMEOUT" env-default:"30s"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" env:"HTTP_IDLE_TIMEOUT" env-default:"60s"`
}

type CORS struct {
	AllowedOrigins []string `yaml:"allowed_origins" env:"CORS_ALLOWED_ORIGINS" env-required:"true"`
}

type Clerk struct {
	SecretKey     string `yaml:"secret_key" env:"CLERK_SECRET_KEY" env-required:"true"`
	WebhookSecret string `yaml:"webhook_secret" env:"CLERK_WEBHOOK_SECRET" env-required:"true"`
}

type Database struct {
	Host            string        `yaml:"host" env:"DB_HOST" env-default:"localhost"`
	Port            int           `yaml:"port" env:"DB_PORT" env-default:"5432"`
	User            string        `yaml:"user" env:"DB_USER" env-default:"postgres"`
	Password        string        `yaml:"password" env:"DB_PASSWORD" env-default:"postgres"`
	Name            string        `yaml:"name" env:"DB_NAME" env-default:"dockyard"`
	SSLMode         string        `yaml:"sslmode" env:"DB_SSLMODE" env-default:"disable"`
	MaxOpenConns    int           `yaml:"max_open_conns" env:"DB_MAX_OPEN_CONNS" env-default:"25"`
	MaxIdleConns    int           `yaml:"max_idle_conns" env:"DB_MAX_IDLE_CONNS" env-default:"25"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME" env-default:"5m"`
}

type Github struct {
	AppId        string `yaml:"app_id" env:"GITHUB_APP_ID" env-required:"true"`
	AppSlug      string `yaml:"app_slug" env:"GITHUB_APP_SLUG" env-required:"true"`
	ClientId     string `yaml:"client_id" env:"GITHUB_CLIENT_ID" env-required:"true"`
	ClientSecret string `yaml:"client_secret" env:"GITHUB_CLIENT_SECRET" env-required:"true"`

	PrivateKeyPath string `yaml:"private_key_path" env:"GITHUB_PRIVATE_KEY_PATH" env-required:"true"`

	WebhookSecret string `yaml:"webhook_secret" env:"GITHUB_WEBHOOK_SECRET" env-required:"true"`
	StateSecret   string `yaml:"state_secret" env:"GITHUB_STATE_SECRET" env-required:"true"`
}

// Logging dono khali chhod sakte ho - logger ENV ke hisaab se chun leta hai.
type Logging struct {
	Level  string `yaml:"level" env:"LOG_LEVEL"`
	Format string `yaml:"format" env:"LOG_FORMAT"`
}

type Config struct {
	Env               string `yaml:"env" env:"ENV" env-required:"true"`
	Logging           `yaml:"logging"`
	HTTPServer        `yaml:"http_server"`
	CORS              `yaml:"cors"`
	Clerk             `yaml:"clerk"`
	Database          `yaml:"database"`
	Github            `yaml:"github"`
	Redis             `yaml:"redis"`
	ContainerRegistry `yaml:"container_registry"`
	Centrifugo        `yaml:"centrifugo"`
	Cloudflare        `yaml:"cloudflare"`
}

func MustLoad() *Config {

	configPath := os.Getenv("CONFIG_PATH")

	if configPath == "" {
		flags := flag.String("config", "", "Path to config file")
		flag.Parse()
		configPath = *flags
	}

	var cfg Config

	if configPath == "" {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			log.Fatalf("Can't read the config from the environment : %s", err.Error())
		}
		return &cfg
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("Config file does not exist : %s", err.Error())
	}

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("Can't read your config file : %s", err.Error())
	}

	return &cfg
}
