package billing

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sms-gateway/internal/db"

	"github.com/google/uuid"
)

type Service struct {
	db     *db.PostgresDB
	logger *slog.Logger
}

func NewService(db *db.PostgresDB, logger *slog.Logger) *Service {
	return &Service{db: db, logger: logger}
}

func (s *Service) HoldCredits(ctx context.Context, clientID, messageID uuid.UUID, amount int64) (*CreditLock, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Deduct credits
	result, err := tx.ExecContext(ctx, "UPDATE clients SET credit_cents = credit_cents - $1 WHERE id = $2 AND credit_cents >= $1", amount, clientID)
	if err != nil {
		return nil, err
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("insufficient credits")
	}

	// Create lock
	lock := &CreditLock{
		ID:        uuid.New(),
		ClientID:  clientID,
		MessageID: messageID,
		Amount:    amount,
		State:     "HELD",
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO credit_locks (id, client_id, message_id, amount_cents, state) VALUES ($1, $2, $3, $4, $5)",
		lock.ID, lock.ClientID, lock.MessageID, lock.Amount, lock.State)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	s.logger.Info("credits held", "client", clientID, "amount", amount)
	return lock, nil
}

func (s *Service) CaptureCredits(ctx context.Context, messageID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "UPDATE credit_locks SET state = 'CAPTURED' WHERE message_id = $1 AND state = 'HELD'", messageID)
	if err != nil {
		return err
	}
	s.logger.Info("credits captured", "message", messageID)
	return nil
}

func (s *Service) ReleaseCredits(ctx context.Context, messageID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var lock CreditLock
	err = tx.QueryRowContext(ctx, "SELECT id, client_id, amount_cents FROM credit_locks WHERE message_id = $1 AND state = 'HELD'", messageID).
		Scan(&lock.ID, &lock.ClientID, &lock.Amount)
	if err != nil {
		return err
	}

	// Return credits
	_, err = tx.ExecContext(ctx, "UPDATE clients SET credit_cents = credit_cents + $1 WHERE id = $2", lock.Amount, lock.ClientID)
	if err != nil {
		return err
	}

	// Mark released
	_, err = tx.ExecContext(ctx, "UPDATE credit_locks SET state = 'RELEASED' WHERE id = $1", lock.ID)
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	s.logger.Info("credits released", "client", lock.ClientID, "amount", lock.Amount)
	return nil
}

func (s *Service) GetCredits(ctx context.Context, clientID uuid.UUID) (int64, error) {
	var credits int64
	err := s.db.QueryRowContext(ctx, "SELECT credit_cents FROM clients WHERE id = $1", clientID).Scan(&credits)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("client not found")
	}
	return credits, err
}

func (s *Service) AddCredits(ctx context.Context, clientID uuid.UUID, amount int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE clients SET credit_cents = credit_cents + $1 WHERE id = $2", amount, clientID)
	if err != nil {
		return err
	}
	s.logger.Info("credits added", "client", clientID, "amount", amount)
	return nil
}

type CreditLock struct {
	ID        uuid.UUID `json:"id"`
	ClientID  uuid.UUID `json:"client_id"`
	MessageID uuid.UUID `json:"message_id"`
	Amount    int64     `json:"amount"`
	State     string    `json:"state"`
}
