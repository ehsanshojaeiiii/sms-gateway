package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	*redis.Client
}

func NewRedis(ctx context.Context, redisURL string) (*RedisClient, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	opts.PoolSize = 10
	opts.MinIdleConns = 5
	opts.ConnMaxLifetime = 1 * time.Hour

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return &RedisClient{Client: client}, nil
}

func (r *RedisClient) Close() error {
	return r.Client.Close()
}

func (r *RedisClient) HealthCheck(ctx context.Context) error {
	return r.Ping(ctx).Err()
}
