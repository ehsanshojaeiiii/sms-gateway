package rate

import (
	"context"
	"fmt"
	"sms-gateway/internal/persistence"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Limiter struct {
	redis  *persistence.RedisClient
	logger *zap.Logger
	rps    int
	burst  int
}

func NewLimiter(redis *persistence.RedisClient, logger *zap.Logger, rps, burst int) *Limiter {
	return &Limiter{
		redis:  redis,
		logger: logger,
		rps:    rps,
		burst:  burst,
	}
}

// Allow checks if a client is within their rate limit using a token bucket algorithm
func (l *Limiter) Allow(ctx context.Context, clientID uuid.UUID) (bool, time.Duration, error) {
	key := fmt.Sprintf("rate_limit:%s", clientID)
	now := time.Now()
	windowStart := now.Truncate(time.Second)

	// Use Redis pipeline for atomic operations
	pipe := l.redis.Pipeline()

	// Get current token count
	getCmd := pipe.Get(ctx, key)
	pipe.Exec(ctx)

	currentTokensStr, err := getCmd.Result()
	currentTokens := 0
	lastRefill := windowStart

	if err != redis.Nil {
		// Parse existing data: "tokens:timestamp"
		var lastRefillUnix int64
		fmt.Sscanf(currentTokensStr, "%d:%d", &currentTokens, &lastRefillUnix)
		lastRefill = time.Unix(lastRefillUnix, 0)
	}

	// Calculate tokens to add based on elapsed time
	elapsed := windowStart.Sub(lastRefill)
	tokensToAdd := int(elapsed.Seconds()) * l.rps

	// Update token count (capped at burst limit)
	currentTokens = min(currentTokens+tokensToAdd, l.burst)

	if currentTokens <= 0 {
		// Rate limited - calculate retry after
		retryAfter := time.Second - time.Duration(now.Nanosecond())
		return false, retryAfter, nil
	}

	// Consume one token
	currentTokens--

	// Update Redis with new token count and timestamp
	newValue := fmt.Sprintf("%d:%d", currentTokens, windowStart.Unix())
	l.redis.Set(ctx, key, newValue, time.Minute) // TTL for cleanup

	return true, 0, nil
}

// Reset clears the rate limit for a client (for testing or administrative purposes)
func (l *Limiter) Reset(ctx context.Context, clientID uuid.UUID) error {
	key := fmt.Sprintf("rate_limit:%s", clientID)
	return l.redis.Del(ctx, key).Err()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
