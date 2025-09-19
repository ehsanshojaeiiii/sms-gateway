package messages

import (
	"context"
	"fmt"
	"math"
	"sms-gateway/internal/billing"
	"sms-gateway/internal/config"
	"sms-gateway/internal/observability"
	"sms-gateway/internal/provider/mock"
	"sms-gateway/internal/queue/nats"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type WorkerService struct {
	logger         *zap.Logger
	metrics        *observability.Metrics
	messageStore   *Store
	billingService *billing.BillingService
	queue          *nats.Queue
	provider       *mock.Provider
	config         *config.Config
}

func NewWorkerService(
	logger *zap.Logger,
	metrics *observability.Metrics,
	messageStore *Store,
	billingService *billing.BillingService,
	queue *nats.Queue,
	provider *mock.Provider,
	config *config.Config,
) *WorkerService {
	return &WorkerService{
		logger:         logger,
		metrics:        metrics,
		messageStore:   messageStore,
		billingService: billingService,
		queue:          queue,
		provider:       provider,
		config:         config,
	}
}

func (w *WorkerService) ProcessMessage(ctx context.Context, job *nats.SendJob) error {
	w.logger.Info("processing message job",
		zap.String("message_id", job.MessageID.String()),
		zap.Int("attempt", job.Attempt))

	// Get message from database
	msg, err := w.messageStore.GetMessage(ctx, job.MessageID)
	if err != nil {
		w.logger.Error("failed to get message", zap.Error(err))
		return fmt.Errorf("failed to get message: %w", err)
	}

	// Skip if message is not in a processable state
	if msg.Status != StatusQueued && msg.Status != StatusFailedTemp {
		w.logger.Debug("skipping message with status",
			zap.String("status", string(msg.Status)))
		return nil
	}

	// Update message status to SENDING
	err = w.messageStore.UpdateMessageStatus(ctx, msg.ID, StatusSending, nil, nil)
	if err != nil {
		w.logger.Error("failed to update message status to SENDING", zap.Error(err))
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Update provider info
	err = w.messageStore.UpdateProvider(ctx, msg.ID, w.provider.GetName())
	if err != nil {
		w.logger.Error("failed to update provider", zap.Error(err))
	}

	// Increment attempt counter
	err = w.messageStore.IncrementAttempts(ctx, msg.ID)
	if err != nil {
		w.logger.Error("failed to increment attempts", zap.Error(err))
	}

	// Convert to provider format
	providerMsg := &mock.Message{
		ID:         msg.ID,
		ToMSISDN:   msg.ToMSISDN,
		FromSender: msg.FromSender,
		Text:       msg.Text,
	}

	// Send via provider
	result := w.provider.SendSMS(ctx, providerMsg)
	
	if result.Error != nil {
		return w.handleSendFailure(ctx, msg, job.Attempt, result)
	}

	// Success - update message with provider ID and status
	status := Status(result.Status)
	err = w.messageStore.UpdateMessageStatus(ctx, msg.ID, status, &result.ProviderMessageID, nil)
	if err != nil {
		w.logger.Error("failed to update message status after send", zap.Error(err))
		return fmt.Errorf("failed to update status: %w", err)
	}

	w.logger.Info("message sent successfully",
		zap.String("message_id", msg.ID.String()),
		zap.String("provider_message_id", result.ProviderMessageID),
		zap.String("status", string(result.Status)))

	if w.metrics != nil {
		w.metrics.MessagesProcessedTotal.WithLabelValues("sent", w.provider.GetName()).Inc()
	}

	// For the mock provider, simulate DLR after a delay
	go w.simulateDelayedDLR(context.Background(), result.ProviderMessageID, msg)

	return nil
}

func (w *WorkerService) handleSendFailure(ctx context.Context, msg *Message, attempt int, result *mock.SendResult) error {
	var finalStatus Status
	var shouldRetry bool

	switch result.Status {
	case mock.StatusFailedTemp:
		if attempt < w.config.MaxAttempts {
			finalStatus = StatusFailedTemp
			shouldRetry = true
		} else {
			finalStatus = StatusFailedPerm
			shouldRetry = false
		}
	case mock.StatusFailedPerm:
		finalStatus = StatusFailedPerm
		shouldRetry = false
	default:
		finalStatus = StatusFailedPerm
		shouldRetry = false
	}

	errorMsg := result.Error.Error()
	err := w.messageStore.UpdateMessageStatus(ctx, msg.ID, finalStatus, nil, &errorMsg)
	if err != nil {
		w.logger.Error("failed to update message status after failure", zap.Error(err))
		return fmt.Errorf("failed to update status: %w", err)
	}

	if shouldRetry {
		// Calculate exponential backoff delay
		delay := w.calculateRetryDelay(attempt)

		w.logger.Info("scheduling retry",
			zap.String("message_id", msg.ID.String()),
			zap.Int("attempt", attempt),
			zap.Duration("delay", delay))

		// Schedule retry
		err = w.queue.PublishSendJobWithDelay(ctx, msg.ID, attempt+1, delay)
		if err != nil {
			w.logger.Error("failed to schedule retry", zap.Error(err))
			return fmt.Errorf("failed to schedule retry: %w", err)
		}

		if w.metrics != nil {
			w.metrics.RetryAttemptsTotal.WithLabelValues("temp_failure").Inc()
		}
	} else {
		// Permanent failure - release held credits
		if err := w.billingService.ReleaseCredits(ctx, msg.ID); err != nil {
			w.logger.Error("failed to release credits for failed message", zap.Error(err))
		}

		// Send to DLQ
		reason := fmt.Sprintf("max attempts reached (%d) or permanent failure: %s", attempt, result.Error.Error())
		if err := w.queue.PublishDLQJob(ctx, msg.ID, reason); err != nil {
			w.logger.Error("failed to send to DLQ", zap.Error(err))
		}

		w.logger.Warn("message permanently failed",
			zap.String("message_id", msg.ID.String()),
			zap.Int("attempts", attempt),
			zap.Error(result.Error))

		if w.metrics != nil {
			w.metrics.MessagesProcessedTotal.WithLabelValues("failed_permanent", w.provider.GetName()).Inc()
		}
	}

	return nil
}

func (w *WorkerService) calculateRetryDelay(attempt int) time.Duration {
	// Exponential backoff with jitter: min_delay * (factor ^ (attempt-1)) + jitter
	exponentialDelay := float64(w.config.RetryMinDelay) * math.Pow(w.config.RetryFactor, float64(attempt-1))

	// Apply max delay cap
	if exponentialDelay > float64(w.config.RetryMaxDelay) {
		exponentialDelay = float64(w.config.RetryMaxDelay)
	}

	// Add jitter (±25% of delay)
	jitterFactor := float64(time.Now().UnixNano()%1000) / 1000.0 // 0.0-1.0
	jitter := exponentialDelay * 0.25 * (2*jitterFactor - 1) // ±25%
	finalDelay := time.Duration(exponentialDelay + jitter)

	// Ensure minimum delay
	if finalDelay < w.config.RetryMinDelay {
		finalDelay = w.config.RetryMinDelay
	}

	return finalDelay
}

func (w *WorkerService) simulateDelayedDLR(ctx context.Context, providerMessageID string, msg *Message) {
	// Simulate DLR after 2-5 seconds
	delay := time.Duration(2+time.Now().UnixNano()%3) * time.Second
	time.Sleep(delay)

	// Determine final status based on message ID (for deterministic testing)
	// This simulates the real provider sending us a DLR
	finalStatus := w.determineFinalStatus(msg.ID)

	w.logger.Debug("simulating DLR",
		zap.String("provider_message_id", providerMessageID),
		zap.String("final_status", string(finalStatus)))

	// In a real system, this would be an HTTP callback to our DLR endpoint
	// For simulation, we can use the provider's SimulateDLR method
	providerStatus := mock.Status(finalStatus)
	w.provider.SimulateDLR(ctx, providerMessageID, providerStatus)
}

func (w *WorkerService) determineFinalStatus(messageID uuid.UUID) Status {
	// Simple deterministic logic based on message ID
	// In real system, this would be determined by the actual provider
	hash := messageID.String()
	lastChar := hash[len(hash)-1]

	// 80% delivery, 20% permanent failure
	if lastChar >= '0' && lastChar <= '7' {
		return StatusDelivered
	}
	return StatusFailedPerm
}
