package idempotency

import (
	"context"
	"fmt"
	"sms-gateway/internal/persistence"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Store struct {
	db     *persistence.PostgresDB
	redis  *persistence.RedisClient
	logger *zap.Logger
}

func NewStore(db *persistence.PostgresDB, redis *persistence.RedisClient, logger *zap.Logger) *Store {
	return &Store{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

func (s *Store) GetMessageID(ctx context.Context, clientID uuid.UUID, key string) (uuid.UUID, error) {
	if key == "" {
		return uuid.Nil, nil
	}

	// Try Redis first (fast path)
	cacheKey := fmt.Sprintf("idempotency:%s:%s", clientID, key)
	messageIDStr, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		messageID, err := uuid.Parse(messageIDStr)
		if err == nil {
			return messageID, nil
		}
	}

	// Simple fallback - return nil for new requests
	return uuid.Nil, nil
}

func (s *Store) StoreMessageID(ctx context.Context, clientID uuid.UUID, key string, messageID uuid.UUID) error {
	if key == "" {
		return nil
	}

	// Cache in Redis
	cacheKey := fmt.Sprintf("idempotency:%s:%s", clientID, key)
	err := s.redis.Set(ctx, cacheKey, messageID.String(), time.Hour).Err()
	if err != nil {
		s.logger.Warn("failed to cache idempotency key", zap.Error(err))
	}

	s.logger.Info("idempotency key stored", zap.String("key", key), zap.String("message_id", messageID.String()))
	return nil
}
