package nats

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type Queue struct {
	conn   *nats.Conn
	logger *slog.Logger
}

type SendJob struct {
	MessageID uuid.UUID `json:"message_id"`
	Attempt   int       `json:"attempt"`
}

func NewQueue(url string, logger *slog.Logger) (*Queue, error) {
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}

	return &Queue{conn: conn, logger: logger}, nil
}

func (q *Queue) PublishSendJob(ctx context.Context, messageID uuid.UUID, attempt int) error {
	job := SendJob{MessageID: messageID, Attempt: attempt}
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	return q.conn.Publish("sms.send", data)
}

func (q *Queue) PublishSendJobWithDelay(ctx context.Context, messageID uuid.UUID, attempt int, delay time.Duration) error {
	// For simplicity, just publish immediately (in production, use NATS JetStream for delays)
	return q.PublishSendJob(ctx, messageID, attempt)
}

func (q *Queue) PublishDLQJob(ctx context.Context, messageID uuid.UUID, reason string) error {
	data := map[string]interface{}{
		"message_id": messageID,
		"reason":     reason,
		"timestamp":  time.Now(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return q.conn.Publish("sms.dlq", jsonData)
}

func (q *Queue) Close() {
	if q.conn != nil {
		q.conn.Close()
	}
}
