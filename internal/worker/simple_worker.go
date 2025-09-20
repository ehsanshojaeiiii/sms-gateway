package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sms-gateway/internal/billing"
	"sms-gateway/internal/config"
	"sms-gateway/internal/messages"
	"sms-gateway/internal/messaging/nats"
	"sms-gateway/internal/providers/mock"
	"time"
)

// SimpleWorker processes messages in a polling manner for interview demo
type SimpleWorker struct {
	logger   *slog.Logger
	store    *messages.Store
	billing  *billing.Service
	queue    *nats.Queue
	provider *mock.Provider
	config   *config.Config
	stop     chan bool
}

func NewSimple(logger *slog.Logger, store *messages.Store, billing *billing.Service, queue *nats.Queue, provider *mock.Provider, cfg *config.Config) *SimpleWorker {
	return &SimpleWorker{
		logger:   logger,
		store:    store,
		billing:  billing,
		queue:    queue,
		provider: provider,
		config:   cfg,
		stop:     make(chan bool),
	}
}

func (w *SimpleWorker) Start(ctx context.Context) error {
	w.logger.Info("Starting simple worker (polling mode)")

	go w.processLoop(ctx)

	return nil
}

func (w *SimpleWorker) Stop(ctx context.Context) {
	w.logger.Info("Stopping simple worker")
	w.stop <- true
}

// processLoop simulates processing queued messages
// In a real system, this would subscribe to NATS and process actual messages
func (w *SimpleWorker) processLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			w.logger.Info("Worker stopped")
			return
		case <-ticker.C:
			w.logger.Debug("Worker heartbeat - ready to process messages")
			// In a real implementation, this would:
			// 1. Subscribe to NATS "sms.send" subject
			// 2. Process incoming messages
			// 3. Send via provider
			// 4. Update message status
			// 5. Handle retries and failures
		case <-ctx.Done():
			w.logger.Info("Worker context cancelled")
			return
		}
	}
}

// ProcessMessage demonstrates how a message would be processed
func (w *SimpleWorker) ProcessMessage(ctx context.Context, messageID string) error {
	w.logger.Info("Processing message", "message_id", messageID)

	// This is a demo method to show the worker logic
	// In the real system, messages would come from NATS queue

	return nil
}

// ProcessExpressMessage handles Express messages with priority and enhanced retry logic
func (w *SimpleWorker) ProcessExpressMessage(ctx context.Context, msg *messages.Message) error {
	w.logger.Info("Processing EXPRESS message", "message_id", msg.ID, "to", msg.To)

	// Express messages get:
	// 1. Higher priority processing
	// 2. More aggressive retry attempts
	// 3. Faster retry intervals
	// 4. Premium routing (if available)
	
	maxAttempts := 5  // Express: 5 attempts vs Regular: 3
	retryDelay := 30 * time.Second  // Express: 30s vs Regular: 60s

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		w.logger.Info("Express delivery attempt", "message_id", msg.ID, "attempt", attempt)
		
		// Simulate provider call
		success := w.attemptDelivery(ctx, msg, true) // true = express mode
		
		if success {
			w.logger.Info("Express message delivered successfully", "message_id", msg.ID, "attempts", attempt)
			return w.finalizeSuccessfulDelivery(ctx, msg)
		}
		
		if attempt < maxAttempts {
			w.logger.Warn("Express delivery failed, retrying", "message_id", msg.ID, "attempt", attempt, "retry_in", retryDelay)
			time.Sleep(retryDelay)
			retryDelay = retryDelay / 2  // Exponential backoff for Express (faster)
		}
	}
	
	// All attempts failed
	w.logger.Error("Express message failed after all attempts", "message_id", msg.ID, "max_attempts", maxAttempts)
	return w.handlePermanentFailure(ctx, msg, "Express delivery failed after maximum attempts")
}

// ProcessRegularMessage handles regular messages with standard retry logic
func (w *SimpleWorker) ProcessRegularMessage(ctx context.Context, msg *messages.Message) error {
	w.logger.Info("Processing regular message", "message_id", msg.ID, "to", msg.To)

	// Regular messages get:
	// 1. Standard priority processing
	// 2. Standard retry attempts
	// 3. Standard retry intervals
	
	maxAttempts := 3  // Regular: 3 attempts
	retryDelay := 60 * time.Second  // Regular: 60s

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		w.logger.Info("Regular delivery attempt", "message_id", msg.ID, "attempt", attempt)
		
		// Simulate provider call
		success := w.attemptDelivery(ctx, msg, false) // false = regular mode
		
		if success {
			w.logger.Info("Regular message delivered successfully", "message_id", msg.ID, "attempts", attempt)
			return w.finalizeSuccessfulDelivery(ctx, msg)
		}
		
		if attempt < maxAttempts {
			w.logger.Warn("Regular delivery failed, retrying", "message_id", msg.ID, "attempt", attempt, "retry_in", retryDelay)
			time.Sleep(retryDelay)
		}
	}
	
	// All attempts failed
	w.logger.Error("Regular message failed after all attempts", "message_id", msg.ID, "max_attempts", maxAttempts)
	return w.handlePermanentFailure(ctx, msg, "Regular delivery failed after maximum attempts")
}

// attemptDelivery simulates sending via SMS provider
func (w *SimpleWorker) attemptDelivery(ctx context.Context, msg *messages.Message, isExpress bool) bool {
	// In Express mode, we might:
	// - Use premium routes
	// - Have higher success rates
	// - Get priority with providers
	
	if isExpress {
		// Express mode: 95% success rate (premium routing)
		return time.Now().UnixNano()%100 < 95
	} else {
		// Regular mode: 85% success rate (standard routing)
		return time.Now().UnixNano()%100 < 85
	}
}

// finalizeSuccessfulDelivery handles successful message delivery
func (w *SimpleWorker) finalizeSuccessfulDelivery(ctx context.Context, msg *messages.Message) error {
	// Update message status to SENT
	providerMsgID := fmt.Sprintf("prov_%d", time.Now().UnixNano())
	if err := w.store.UpdateStatus(ctx, msg.ID, messages.StatusSent, &providerMsgID, nil); err != nil {
		w.logger.Error("Failed to update message status", "message_id", msg.ID, "error", err)
		return err
	}
	
	// Capture held credits (finalize billing)
	if err := w.billing.CaptureCredits(ctx, msg.ID); err != nil {
		w.logger.Error("Failed to capture credits", "message_id", msg.ID, "error", err)
		// Continue - message was sent, this is a billing issue
	}
	
	return nil
}

// handlePermanentFailure handles messages that failed all retry attempts
func (w *SimpleWorker) handlePermanentFailure(ctx context.Context, msg *messages.Message, reason string) error {
	// Update message status to permanent failure
	if err := w.store.UpdateStatus(ctx, msg.ID, messages.StatusFailedPerm, nil, &reason); err != nil {
		w.logger.Error("Failed to update failed message status", "message_id", msg.ID, "error", err)
	}
	
	// Release held credits back to client
	if err := w.billing.ReleaseCredits(ctx, msg.ID); err != nil {
		w.logger.Error("Failed to release credits for failed message", "message_id", msg.ID, "error", err)
	}
	
	return fmt.Errorf("message delivery failed: %s", reason)
}
