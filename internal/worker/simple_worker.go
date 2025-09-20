package worker

import (
	"context"
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
