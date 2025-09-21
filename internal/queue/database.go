package queue

import (
	"context"
	"database/sql"
	"log/slog"
	"sms-gateway/internal/messages"

	"github.com/google/uuid"
)

// Queue implements reliable SMS queue using PostgreSQL only
type Queue struct {
	db     *sql.DB
	logger *slog.Logger
}

// Result represents processing result
type Result struct {
	MessageID uuid.UUID
	Success   bool
	Error     error
}

// New creates a database queue
func New(store *messages.Store, logger *slog.Logger) *Queue {
	return &Queue{
		db:     store.DB(),
		logger: logger,
	}
}

// Poll atomically claims messages for processing
func (q *Queue) Poll(ctx context.Context, limit int) ([]*messages.Message, error) {
	// Simple, fast atomic update - claims messages in one query
	query := `
		UPDATE messages 
		SET status = 'SENDING', updated_at = NOW()
		WHERE id IN (
			SELECT id FROM messages 
			WHERE status = 'QUEUED'
			ORDER BY express DESC, created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, client_id, to_msisdn, from_sender, text, parts, 
				  client_reference, express, attempts`

	rows, err := q.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*messages.Message
	for rows.Next() {
		msg := &messages.Message{Status: messages.StatusSending}
		rows.Scan(&msg.ID, &msg.ClientID, &msg.To, &msg.From, &msg.Text, &msg.Parts,
			&msg.Reference, &msg.Express, &msg.Attempts)
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

// Complete marks message as sent
func (q *Queue) Complete(ctx context.Context, messageID uuid.UUID) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE messages SET status = 'SENT', updated_at = NOW() 
		 WHERE id = $1 AND status = 'SENDING'`, messageID)
	return err
}

// Fail marks message as failed with retry logic
func (q *Queue) Fail(ctx context.Context, messageID uuid.UUID, errorMsg string) error {
	_, err := q.db.ExecContext(ctx, `
		UPDATE messages 
		SET status = CASE WHEN attempts >= 2 THEN 'FAILED_PERM' ELSE 'FAILED_TEMP' END,
			attempts = attempts + 1,
			last_error = $2,
			retry_after = CASE WHEN attempts >= 2 THEN NULL ELSE NOW() + INTERVAL '30 seconds' END,
			updated_at = NOW()
		WHERE id = $1 AND status = 'SENDING'`, messageID, errorMsg)
	return err
}

// Retry moves failed messages back to queue
func (q *Queue) Retry(ctx context.Context) (int64, error) {
	result, err := q.db.ExecContext(ctx, `
		UPDATE messages 
		SET status = 'QUEUED', updated_at = NOW()
		WHERE status = 'FAILED_TEMP' AND retry_after <= NOW()`)
	if err != nil {
		return 0, err
	}
	count, _ := result.RowsAffected()
	return count, nil
}
