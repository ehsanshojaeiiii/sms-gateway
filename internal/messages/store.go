package messages

import (
	"context"
	"database/sql"
	"fmt"
	"sms-gateway/internal/persistence"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Store struct {
	db     *persistence.PostgresDB
	logger *zap.Logger
}

func NewStore(db *persistence.PostgresDB, logger *zap.Logger) *Store {
	return &Store{
		db:     db,
		logger: logger,
	}
}

func (s *Store) CreateMessage(ctx context.Context, msg *Message) error {
	query := `
		INSERT INTO messages (id, client_id, to_msisdn, from_sender, text, parts, status, client_reference, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := s.db.ExecContext(ctx, query,
		msg.ID, msg.ClientID, msg.ToMSISDN, msg.FromSender, msg.Text,
		msg.Parts, msg.Status, msg.ClientReference, msg.CreatedAt, msg.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	s.logger.Info("message created",
		zap.String("id", msg.ID.String()),
		zap.String("to", msg.ToMSISDN),
		zap.String("from", msg.FromSender))

	return nil
}

func (s *Store) DeleteMessage(ctx context.Context, messageID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM messages WHERE id = $1`, messageID)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	return nil
}

func (s *Store) GetMessage(ctx context.Context, messageID uuid.UUID) (*Message, error) {
	query := `
		SELECT id, client_id, to_msisdn, from_sender, text, parts, status,
		       client_reference, provider, provider_message_id, attempts, last_error,
		       created_at, updated_at
		FROM messages
		WHERE id = $1`

	var msg Message
	err := s.db.QueryRowContext(ctx, query, messageID).Scan(
		&msg.ID, &msg.ClientID, &msg.ToMSISDN, &msg.FromSender, &msg.Text,
		&msg.Parts, &msg.Status, &msg.ClientReference, &msg.Provider,
		&msg.ProviderMessageID, &msg.Attempts, &msg.LastError,
		&msg.CreatedAt, &msg.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("message not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return &msg, nil
}

func (s *Store) GetMessagesByClient(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*Message, error) {
	query := `
		SELECT id, client_id, to_msisdn, from_sender, text, parts, status,
		       client_reference, provider, provider_message_id, attempts, last_error,
		       created_at, updated_at
		FROM messages
		WHERE client_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, clientID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		var msg Message
		err := rows.Scan(
			&msg.ID, &msg.ClientID, &msg.ToMSISDN, &msg.FromSender, &msg.Text,
			&msg.Parts, &msg.Status, &msg.ClientReference, &msg.Provider,
			&msg.ProviderMessageID, &msg.Attempts, &msg.LastError,
			&msg.CreatedAt, &msg.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

func (s *Store) ListMessages(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*Message, error) {
	return s.GetMessagesByClient(ctx, clientID, limit, offset)
}

func (s *Store) HealthCheck(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) UpdateMessageStatus(ctx context.Context, messageID uuid.UUID, status Status, providerMessageID *string, lastError *string) error {
	query := `
		UPDATE messages 
		SET status = $2, provider_message_id = COALESCE($3, provider_message_id), 
		    last_error = $4, updated_at = $5
		WHERE id = $1`

	_, err := s.db.ExecContext(ctx, query, messageID, status, providerMessageID, lastError, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update message status: %w", err)
	}

	return nil
}

func (s *Store) IncrementAttempts(ctx context.Context, messageID uuid.UUID) error {
	query := `UPDATE messages SET attempts = attempts + 1, updated_at = $2 WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, messageID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to increment attempts: %w", err)
	}
	return nil
}

func (s *Store) UpdateProvider(ctx context.Context, messageID uuid.UUID, provider string) error {
	query := `UPDATE messages SET provider = $2, updated_at = $3 WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, messageID, provider, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update provider: %w", err)
	}
	return nil
}

func (s *Store) GetMessageByProviderID(ctx context.Context, providerMessageID string) (*Message, error) {
	query := `
		SELECT id, client_id, to_msisdn, from_sender, text, parts, status,
		       client_reference, provider, provider_message_id, attempts, last_error,
		       created_at, updated_at
		FROM messages
		WHERE provider_message_id = $1`

	var msg Message
	err := s.db.QueryRowContext(ctx, query, providerMessageID).Scan(
		&msg.ID, &msg.ClientID, &msg.ToMSISDN, &msg.FromSender, &msg.Text,
		&msg.Parts, &msg.Status, &msg.ClientReference, &msg.Provider,
		&msg.ProviderMessageID, &msg.Attempts, &msg.LastError,
		&msg.CreatedAt, &msg.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("message not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return &msg, nil
}
