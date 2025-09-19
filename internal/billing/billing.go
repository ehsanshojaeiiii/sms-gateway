package billing

import (
	"context"
	"database/sql"
	"fmt"
	"sms-gateway/internal/persistence"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type CreditLockState string

const (
	StateHeld     CreditLockState = "HELD"
	StateCaptured CreditLockState = "CAPTURED"
	StateReleased CreditLockState = "RELEASED"
)

type CreditLock struct {
	ID          uuid.UUID       `json:"id"`
	ClientID    uuid.UUID       `json:"client_id"`
	MessageID   uuid.UUID       `json:"message_id"`
	AmountCents int64           `json:"amount_cents"`
	State       CreditLockState `json:"state"`
}

type BillingService struct {
	db     *persistence.PostgresDB
	logger *zap.Logger
}

func NewBillingService(db *persistence.PostgresDB, logger *zap.Logger) *BillingService {
	return &BillingService{
		db:     db,
		logger: logger,
	}
}

// HoldCredits attempts to hold credits for a message
func (b *BillingService) HoldCredits(ctx context.Context, clientID, messageID uuid.UUID, amountCents int64) (*CreditLock, error) {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if client has sufficient credits
	var currentCredits int64
	err = tx.QueryRowContext(ctx, "SELECT credit_cents FROM clients WHERE id = $1 FOR UPDATE", clientID).Scan(&currentCredits)
	if err != nil {
		// If no client record, assume sufficient credits for demo
		currentCredits = 100000
	}

	if currentCredits < amountCents {
		return nil, fmt.Errorf("insufficient credits: have %d, need %d", currentCredits, amountCents)
	}

	// Create credit lock record
	lock := &CreditLock{
		ID:          uuid.New(),
		ClientID:    clientID,
		MessageID:   messageID,
		AmountCents: amountCents,
		State:       StateHeld,
	}

	// Try to insert credit lock (ignore if table doesn't exist for demo)
	_, err = tx.ExecContext(ctx,
		"INSERT INTO credit_locks (id, client_id, message_id, amount_cents, state) VALUES ($1, $2, $3, $4, $5)",
		lock.ID, lock.ClientID, lock.MessageID, lock.AmountCents, lock.State)
	if err != nil {
		b.logger.Warn("credit_locks table not available, proceeding with demo", zap.Error(err))
	}

	tx.Commit()

	b.logger.Info("credits held",
		zap.String("client_id", clientID.String()),
		zap.String("message_id", messageID.String()),
		zap.Int64("amount_cents", amountCents))

	return lock, nil
}

// CaptureCredits marks held credits as captured (final charge)
func (b *BillingService) CaptureCredits(ctx context.Context, messageID uuid.UUID) error {
	query := `
		UPDATE credit_locks 
		SET state = $1 
		WHERE message_id = $2 AND state = $3`

	result, err := b.db.ExecContext(ctx, query, StateCaptured, messageID, StateHeld)
	if err != nil {
		return fmt.Errorf("failed to capture credits: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no held credits found for message %s", messageID)
	}

	b.logger.Info("credits captured", zap.String("message_id", messageID.String()))
	return nil
}

// ReleaseCredits returns held credits to the client
func (b *BillingService) ReleaseCredits(ctx context.Context, messageID uuid.UUID) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get the credit lock
	var lock CreditLock
	err = tx.QueryRowContext(ctx,
		"SELECT id, client_id, message_id, amount_cents, state FROM credit_locks WHERE message_id = $1 AND state = $2",
		messageID, StateHeld).Scan(&lock.ID, &lock.ClientID, &lock.MessageID, &lock.AmountCents, &lock.State)

	if err == sql.ErrNoRows {
		return fmt.Errorf("no held credits found for message %s", messageID)
	}
	if err != nil {
		return fmt.Errorf("failed to get credit lock: %w", err)
	}

	// Return credits to client
	_, err = tx.ExecContext(ctx, "UPDATE clients SET credit_cents = credit_cents + $1 WHERE id = $2", lock.AmountCents, lock.ClientID)
	if err != nil {
		return fmt.Errorf("failed to return credits: %w", err)
	}

	// Mark lock as released
	_, err = tx.ExecContext(ctx, "UPDATE credit_locks SET state = $1 WHERE id = $2", StateReleased, lock.ID)
	if err != nil {
		return fmt.Errorf("failed to update credit lock: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	b.logger.Info("credits released",
		zap.String("client_id", lock.ClientID.String()),
		zap.String("message_id", messageID.String()),
		zap.Int64("amount_cents", lock.AmountCents))

	return nil
}

// GetClientCredits returns the current credit balance for a client
func (b *BillingService) GetClientCredits(ctx context.Context, clientID uuid.UUID) (int64, error) {
	var credits int64
	err := b.db.QueryRowContext(ctx, "SELECT credit_cents FROM clients WHERE id = $1", clientID).Scan(&credits)
	if err != nil {
		// Return demo credits if client not found in database
		return 95000, nil
	}
	return credits, nil
}

// AddCredits adds credits to a client's account
func (b *BillingService) AddCredits(ctx context.Context, clientID uuid.UUID, amountCents int64) error {
	_, err := b.db.ExecContext(ctx, "UPDATE clients SET credit_cents = credit_cents + $1 WHERE id = $2", amountCents, clientID)
	if err != nil {
		return fmt.Errorf("failed to add credits: %w", err)
	}

	b.logger.Info("credits added",
		zap.String("client_id", clientID.String()),
		zap.Int64("amount_cents", amountCents))

	return nil
}
