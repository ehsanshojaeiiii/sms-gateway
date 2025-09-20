package db

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisDB struct {
	*redis.Client
}

func NewRedis(ctx context.Context, url string) (*RedisDB, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisDB{Client: client}, nil
}
