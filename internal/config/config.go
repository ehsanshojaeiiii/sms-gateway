package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	// Server
	Port         string        `envconfig:"PORT" default:"8080"`
	ReadTimeout  time.Duration `envconfig:"READ_TIMEOUT" default:"30s"`
	WriteTimeout time.Duration `envconfig:"WRITE_TIMEOUT" default:"30s"`
	IdleTimeout  time.Duration `envconfig:"IDLE_TIMEOUT" default:"120s"`

	// Database
	PostgresURL string `envconfig:"POSTGRES_URL" required:"true"`

	// Redis
	RedisURL string `envconfig:"REDIS_URL" required:"true"`

	// NATS
	NATSURL string `envconfig:"NATS_URL" required:"true"`

	// Billing
	PricePerPartCents     int64 `envconfig:"PRICE_PER_PART_CENTS" default:"5"`
	ExpressSurchargeCents int64 `envconfig:"EXPRESS_SURCHARGE_CENTS" default:"2"`

	// Observability
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
