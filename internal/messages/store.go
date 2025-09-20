package messages

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sms-gateway/internal/db"
	"time"

	"github.com/google/uuid"
)

type Store struct {
	db     *db.PostgresDB
	logger *slog.Logger
}

func NewStore(db *db.PostgresDB, logger *slog.Logger) *Store {
	return &Store{db: db, logger: logger}
}

func (s *Store) Create(ctx context.Context, msg *Message) error {
	query := `INSERT INTO messages (id, client_id, to_msisdn, from_sender, text, parts, status, client_reference, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := s.db.ExecContext(ctx, query, msg.ID, msg.ClientID, msg.To, msg.From, msg.Text, msg.Parts, msg.Status, msg.Reference, msg.CreatedAt, msg.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	s.logger.Info("message created", "id", msg.ID, "to", msg.To)
	return nil
}

func (s *Store) GetByID(ctx context.Context, messageID uuid.UUID) (*Message, error) {
	query := `SELECT id, client_id, to_msisdn, from_sender, text, parts, status, client_reference, provider, provider_message_id, attempts, last_error, created_at, updated_at
		FROM messages WHERE id = $1`

	var msg Message
	err := s.db.QueryRowContext(ctx, query, messageID).Scan(
		&msg.ID, &msg.ClientID, &msg.To, &msg.From, &msg.Text, &msg.Parts, &msg.Status, &msg.Reference,
		&msg.Provider, &msg.ProviderMessageID, &msg.Attempts, &msg.LastError, &msg.CreatedAt, &msg.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("message not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return &msg, nil
}

func (s *Store) ListByClient(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*Message, error) {
	query := `SELECT id, client_id, to_msisdn, from_sender, text, parts, status, client_reference, provider, provider_message_id, attempts, last_error, created_at, updated_at
		FROM messages WHERE client_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, clientID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		var msg Message
		err := rows.Scan(&msg.ID, &msg.ClientID, &msg.To, &msg.From, &msg.Text, &msg.Parts, &msg.Status, &msg.Reference,
			&msg.Provider, &msg.ProviderMessageID, &msg.Attempts, &msg.LastError, &msg.CreatedAt, &msg.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

func (s *Store) Delete(ctx context.Context, messageID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM messages WHERE id = $1", messageID)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	s.logger.Info("message deleted", "id", messageID)
	return nil
}

func (s *Store) Health(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) UpdateStatus(ctx context.Context, messageID uuid.UUID, status Status, providerID *string, lastError *string) error {
	query := `UPDATE messages SET status = $2, provider_message_id = COALESCE($3, provider_message_id), last_error = $4, updated_at = $5 WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, messageID, status, providerID, lastError, time.Now())
	return err
}

func (s *Store) IncrementAttempts(ctx context.Context, messageID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "UPDATE messages SET attempts = attempts + 1, updated_at = $2 WHERE id = $1", messageID, time.Now())
	return err
}

func (s *Store) UpdateProvider(ctx context.Context, messageID uuid.UUID, provider string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE messages SET provider = $2, updated_at = $3 WHERE id = $1", messageID, provider, time.Now())
	return err
}

func (s *Store) GetByProviderID(ctx context.Context, providerMessageID string) (*Message, error) {
	query := `SELECT id, client_id, to_msisdn, from_sender, text, parts, status, client_reference, provider, provider_message_id, attempts, last_error, created_at, updated_at
		FROM messages WHERE provider_message_id = $1`

	var msg Message
	err := s.db.QueryRowContext(ctx, query, providerMessageID).Scan(
		&msg.ID, &msg.ClientID, &msg.To, &msg.From, &msg.Text, &msg.Parts, &msg.Status, &msg.Reference,
		&msg.Provider, &msg.ProviderMessageID, &msg.Attempts, &msg.LastError, &msg.CreatedAt, &msg.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("message not found")
	}
	return &msg, err
}
