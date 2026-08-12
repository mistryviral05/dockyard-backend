package config

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)


type Redis struct {
	Addr     string `yaml:"addr" env-required:"true"`
	Password string `yaml:"password" env-required:"true"`
	DB       int    `yaml:"db" env-required:"true"`
}

type HTTPServer struct {
	Addr string `yaml:"address" env-required:"true" env-default:":8080"`
	ReadTimeout  time.Duration `yaml:"read_timeout" env:"HTTP_READ_TIMEOUT" env-default:"10s"`
	WriteTimeout time.Duration `yaml:"write_timeout" env:"HTTP_WRITE_TIMEOUT" env-default:"30s"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" env:"HTTP_IDLE_TIMEOUT" env-default:"60s"`
}

type CORS struct {
	AllowedOrigins []string `yaml:"allowed_origins" env-required:"true"`
}

type Clerk struct {
	SecretKey     string `yaml:"secret_key" env-required:"true"`
	WebhookSecret string `yaml:"webhook_secret" env-required:"true"`
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
	AppId          string `yaml:"app_id" env-required:"true"`
	AppSlug        string `yaml:"app_slug" env-required:"true"`
	ClientId       string `yaml:"client_id" env-required:"true"`
	ClientSecret   string `yaml:"client_secret" env-required:"true"`
	PrivateKeyPath string `yaml:"private_key_path" env-required:"true"`
	WebhookSecret  string `yaml:"webhook_secret" env-required:"true"`
	StateSecret    string `yaml:"state_secret" env-required:"true"`
}

type Config struct {
	Env        string `yaml:"env" env-required:"true" env-default:"production"`
	HTTPServer `yaml:"http_server"`
	CORS       `yaml:"cors"`
	Clerk      `yaml:"clerk"`
	Database   `yaml:"database"`
	Github     `yaml:"github"`
	Redis      `yaml:"redis"`
}

func MustLoad() *Config {

	var configPath string

	configPath = os.Getenv("CONFIG_PATH")

	if configPath == "" {
		flags := flag.String("config", "", "Path to config file")
		flag.Parse()
		configPath = *flags

		if configPath == "" {
			log.Fatal("Config Path is not provided!")
		}
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("Config file does not exist : %s", err.Error())
	}
	var cfg Config
	err := cleanenv.ReadConfig(configPath, &cfg)

	if err != nil {
		log.Fatalf("Can't read your config file : %s", err.Error())
	}

	return &cfg
}
