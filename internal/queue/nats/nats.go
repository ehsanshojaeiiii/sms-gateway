package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const (
	SubjectSMSSend = "sms.send"
	SubjectSMSDLQ  = "sms.dlq"
)

type SendJob struct {
	MessageID uuid.UUID     `json:"message_id"`
	Attempt   int           `json:"attempt"`
	MaxDelay  time.Duration `json:"max_delay,omitempty"`
}

type Queue struct {
	conn   *nats.Conn
	logger *zap.Logger
}

func NewQueue(natsURL string, logger *zap.Logger) (*Queue, error) {
	opts := []nats.Option{
		nats.Name("SMS Gateway"),
		nats.Timeout(10 * time.Second),
		nats.ReconnectWait(5 * time.Second),
		nats.MaxReconnects(-1), // Infinite reconnects
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			logger.Error("NATS disconnected", zap.Error(err))
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", zap.String("url", nc.ConnectedUrl()))
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			logger.Info("NATS connection closed")
		}),
	}

	conn, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	logger.Info("connected to NATS", zap.String("url", conn.ConnectedUrl()))

	return &Queue{
		conn:   conn,
		logger: logger,
	}, nil
}

func (q *Queue) Close() error {
	q.conn.Close()
	return nil
}

func (q *Queue) HealthCheck(ctx context.Context) error {
	if q.conn.Status() != nats.CONNECTED {
		return fmt.Errorf("NATS not connected, status: %v", q.conn.Status())
	}
	return nil
}

// PublishSendJob publishes a message to be sent by workers
func (q *Queue) PublishSendJob(ctx context.Context, messageID uuid.UUID, attempt int) error {
	job := SendJob{
		MessageID: messageID,
		Attempt:   attempt,
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal send job: %w", err)
	}

	err = q.conn.Publish(SubjectSMSSend, data)
	if err != nil {
		return fmt.Errorf("failed to publish send job: %w", err)
	}

	q.logger.Debug("published send job",
		zap.String("message_id", messageID.String()),
		zap.Int("attempt", attempt))

	return nil
}

// PublishSendJobWithDelay publishes a message with a delay (for retries)
func (q *Queue) PublishSendJobWithDelay(ctx context.Context, messageID uuid.UUID, attempt int, delay time.Duration) error {
	job := SendJob{
		MessageID: messageID,
		Attempt:   attempt,
		MaxDelay:  delay,
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal delayed send job: %w", err)
	}

	// For delayed messages, we'll use a simple approach with goroutine
	// In production, consider using a more robust delayed queue solution
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			if err := q.conn.Publish(SubjectSMSSend, data); err != nil {
				q.logger.Error("failed to publish delayed send job",
					zap.String("message_id", messageID.String()),
					zap.Error(err))
			}
		case <-ctx.Done():
			q.logger.Debug("delayed send job cancelled",
				zap.String("message_id", messageID.String()))
		}
	}()

	q.logger.Debug("scheduled delayed send job",
		zap.String("message_id", messageID.String()),
		zap.Int("attempt", attempt),
		zap.Duration("delay", delay))

	return nil
}

// PublishDLQJob publishes a message to the dead letter queue
func (q *Queue) PublishDLQJob(ctx context.Context, messageID uuid.UUID, reason string) error {
	job := map[string]interface{}{
		"message_id": messageID,
		"reason":     reason,
		"timestamp":  time.Now(),
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ job: %w", err)
	}

	err = q.conn.Publish(SubjectSMSDLQ, data)
	if err != nil {
		return fmt.Errorf("failed to publish DLQ job: %w", err)
	}

	q.logger.Warn("published DLQ job",
		zap.String("message_id", messageID.String()),
		zap.String("reason", reason))

	return nil
}

// SubscribeSendJobs subscribes to send jobs for processing
func (q *Queue) SubscribeSendJobs(handler func(job *SendJob) error) (*nats.Subscription, error) {
	return q.conn.Subscribe(SubjectSMSSend, func(msg *nats.Msg) {
		var job SendJob
		if err := json.Unmarshal(msg.Data, &job); err != nil {
			q.logger.Error("failed to unmarshal send job", zap.Error(err))
			return
		}

		q.logger.Debug("received send job",
			zap.String("message_id", job.MessageID.String()),
			zap.Int("attempt", job.Attempt))

		if err := handler(&job); err != nil {
			q.logger.Error("failed to handle send job",
				zap.String("message_id", job.MessageID.String()),
				zap.Error(err))
		}
	})
}

// SubscribeDLQJobs subscribes to dead letter queue jobs for monitoring
func (q *Queue) SubscribeDLQJobs(handler func(messageID uuid.UUID, reason string, timestamp time.Time)) (*nats.Subscription, error) {
	return q.conn.Subscribe(SubjectSMSDLQ, func(msg *nats.Msg) {
		var job map[string]interface{}
		if err := json.Unmarshal(msg.Data, &job); err != nil {
			q.logger.Error("failed to unmarshal DLQ job", zap.Error(err))
			return
		}

		messageIDStr, ok := job["message_id"].(string)
		if !ok {
			q.logger.Error("invalid message_id in DLQ job")
			return
		}

		messageID, err := uuid.Parse(messageIDStr)
		if err != nil {
			q.logger.Error("failed to parse message_id", zap.Error(err))
			return
		}

		reason, _ := job["reason"].(string)
		timestampStr, _ := job["timestamp"].(string)
		timestamp, _ := time.Parse(time.RFC3339, timestampStr)

		handler(messageID, reason, timestamp)
	})
}
