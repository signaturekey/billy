package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
}

type AppConfig struct {
	Env     string        `env:"APP_ENV" env-default:"development"`
	Port    string        `env:"APP_PORT" env-default:"8080"`
	BaseURL string        `env:"APP_BASE_URL" env-default:"http://localhost:8080"`
	HoldTTL time.Duration `env:"HOLD_TTL" env-default:"15m"`
}

type DatabaseConfig struct {
	Host              string        `env:"DB_HOST" env-default:"localhost"`
	Port              string        `env:"DB_PORT" env-default:"5432"`
	User              string        `env:"DB_USER" env-default:"postgres"`
	Password          string        `env:"DB_PASSWORD" env-default:"postgres"`
	Name              string        `env:"DB_NAME" env-default:"billy_db"`
	SSLMode           string        `env:"DB_SSL_MODE" env-default:"disable"`
	MaxConns          int32         `env:"DB_MAX_CONNS" env-default:"25"`
	MinConns          int32         `env:"DB_MIN_CONNS" env-default:"10"`
	MaxConnLifetime   time.Duration `env:"DB_MAX_CONN_LIFETIME" env-default:"5m"`
	MaxConnIdleTime   time.Duration `env:"DB_MAX_CONN_IDLE_TIME" env-default:"30m"`
	HealthCheckPeriod time.Duration `env:"DB_HEALTH_CHECK_PERIOD" env-default:"1m"`
}

func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

func Load() (*Config, error) {
	var cfg Config

	if err := cleanenv.ReadConfig(".env", &cfg); err != nil {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
	}

	return &cfg, nil
}

func MustLoad() *Config {
	cfg, err := Load()

	if err != nil {
		panic(err)
	}

	return cfg
}
