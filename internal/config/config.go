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
	PricePerPartCents int64 `envconfig:"PRICE_PER_PART_CENTS" default:"5"`

	// Rate Limiting
	RateLimitRPS   int `envconfig:"RATE_LIMIT_RPS" default:"100"`
	RateLimitBurst int `envconfig:"RATE_LIMIT_BURST" default:"200"`

	// Provider Mock
	MockSuccessRate  float64 `envconfig:"MOCK_SUCCESS_RATE" default:"0.8"`
	MockTempFailRate float64 `envconfig:"MOCK_TEMP_FAIL_RATE" default:"0.15"`
	MockPermFailRate float64 `envconfig:"MOCK_PERM_FAIL_RATE" default:"0.05"`
	MockLatencyMs    int     `envconfig:"MOCK_LATENCY_MS" default:"100"`

	// Retry Policy
	RetryMinDelay time.Duration `envconfig:"RETRY_MIN_DELAY" default:"15s"`
	RetryMaxDelay time.Duration `envconfig:"RETRY_MAX_DELAY" default:"30m"`
	RetryFactor   float64       `envconfig:"RETRY_FACTOR" default:"2.0"`
	MaxAttempts   int           `envconfig:"MAX_ATTEMPTS" default:"10"`

	// Observability
	MetricsEnabled bool   `envconfig:"METRICS_ENABLED" default:"true"`
	TracingEnabled bool   `envconfig:"TRACING_ENABLED" default:"true"`
	LogLevel       string `envconfig:"LOG_LEVEL" default:"info"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
