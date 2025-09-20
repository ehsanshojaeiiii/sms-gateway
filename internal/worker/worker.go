package worker

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sms-gateway/internal/billing"
	"sms-gateway/internal/config"
	"sms-gateway/internal/messages"
	"sms-gateway/internal/messaging/nats"
	"sms-gateway/internal/providers/mock"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Worker processes SMS messages with controlled concurrency
type Worker struct {
	logger   *slog.Logger
	store    *messages.Store
	billing  *billing.Service
	queue    *nats.Queue
	provider *mock.Provider
	config   *config.Config

	// Worker pool for controlled concurrency
	jobChan    chan uuid.UUID
	workerPool int
	wg         sync.WaitGroup
	stop       chan bool

	// Metrics
	processed  int64
	failed     int64
	concurrent int64
}

func New(logger *slog.Logger, store *messages.Store, billing *billing.Service, queue *nats.Queue, provider *mock.Provider, cfg *config.Config) *Worker {
	// Safe worker pool size: CPU cores * 2 (reasonable for I/O bound work)
	workerPoolSize := runtime.NumCPU() * 2
	if workerPoolSize > 10 {
		workerPoolSize = 10 // Cap at 10 workers for safety
	}

	return &Worker{
		logger:     logger,
		store:      store,
		billing:    billing,
		queue:      queue,
		provider:   provider,
		config:     cfg,
		jobChan:    make(chan uuid.UUID, 100), // Buffered channel
		workerPool: workerPoolSize,
		stop:       make(chan bool),
	}
}

func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("Starting SMS Worker", 
		"worker_pool_size", w.workerPool,
		"max_concurrent_jobs", 100)

	// Start fixed number of worker goroutines
	for i := 0; i < w.workerPool; i++ {
		w.wg.Add(1)
		go w.worker(ctx, i)
	}

	// Start message consumer (single goroutine)
	w.wg.Add(1)
	go w.consumeMessages(ctx)

	// Start metrics reporter
	w.wg.Add(1)
	go w.metricsLogger(ctx)

	return nil
}

func (w *Worker) Stop(ctx context.Context) error {
	w.logger.Info("Stopping SMS Worker...")
	close(w.stop)
	
	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Info("All workers stopped gracefully")
	case <-time.After(30 * time.Second):
		w.logger.Warn("Worker shutdown timeout")
	}

	return nil
}

// worker is a fixed worker goroutine that processes jobs from the channel
func (w *Worker) worker(ctx context.Context, workerID int) {
	defer w.wg.Done()
	w.logger.Info("Worker started", "worker_id", workerID)

	for {
		select {
		case <-w.stop:
			w.logger.Info("Worker stopping", "worker_id", workerID)
			return
		case <-ctx.Done():
			w.logger.Info("Worker context cancelled", "worker_id", workerID)
			return
		case messageID := <-w.jobChan:
			w.processMessage(ctx, messageID, workerID)
		}
	}
}

// consumeMessages gets messages from NATS and sends to worker pool
func (w *Worker) consumeMessages(ctx context.Context) {
	defer w.wg.Done()
	w.logger.Info("Starting NATS message consumer")

	for {
		select {
		case <-w.stop:
			w.logger.Info("Message consumer stopped")
			return
		case <-ctx.Done():
			w.logger.Info("Message consumer context cancelled")
			return
		default:
			// Get message from NATS
			messageID, err := w.queue.ConsumeSendJob(ctx)
			if err != nil {
				time.Sleep(1 * time.Second) // Brief pause on error
				continue
			}

			// Send to worker pool (non-blocking)
			select {
			case w.jobChan <- messageID:
				// Message queued for processing
			default:
				// Worker pool full - this prevents overwhelming the system
				w.logger.Warn("Worker pool full, dropping message", "message_id", messageID)
				atomic.AddInt64(&w.failed, 1)
			}
		}
	}
}

// processMessage handles a single message (called by worker goroutines)
func (w *Worker) processMessage(ctx context.Context, messageID uuid.UUID, workerID int) {
	atomic.AddInt64(&w.concurrent, 1)
	defer atomic.AddInt64(&w.concurrent, -1)

	start := time.Now()

	// Get message
	msg, err := w.store.GetByID(ctx, messageID)
	if err != nil {
		w.logger.Error("Failed to get message", "message_id", messageID, "worker_id", workerID, "error", err)
		atomic.AddInt64(&w.failed, 1)
		return
	}

	// Check if already processed (idempotency)
	if msg.Status != messages.StatusQueued {
		w.logger.Debug("Message already processed", "message_id", messageID, "status", msg.Status)
		return
	}

	w.logger.Info("Processing message", "message_id", messageID, "worker_id", workerID, "to", msg.To)

	// Update status to SENDING
	if err := w.store.UpdateStatus(ctx, messageID, messages.StatusSending, nil, nil); err != nil {
		w.logger.Error("Failed to update status", "message_id", messageID, "error", err)
		atomic.AddInt64(&w.failed, 1)
		return
	}

	// Increment attempts
	w.store.IncrementAttempts(ctx, messageID)

	// Attempt delivery
	success := w.attemptDelivery(ctx, msg)
	duration := time.Since(start)

	if success {
		w.handleSuccess(ctx, msg)
		atomic.AddInt64(&w.processed, 1)
		w.logger.Info("Message delivered", "message_id", messageID, "duration", duration)
	} else {
		w.handleFailure(ctx, msg)
		atomic.AddInt64(&w.failed, 1)
		w.logger.Warn("Message failed", "message_id", messageID, "duration", duration)
	}
}

// attemptDelivery tries to deliver via provider
func (w *Worker) attemptDelivery(ctx context.Context, msg *messages.Message) bool {
	providerMsg := &mock.Message{
		ToMSISDN:   msg.To,
		FromSender: msg.From,
		Text:       msg.Text,
	}

	result := w.provider.SendSMS(ctx, providerMsg)

	// Realistic success rates
	if msg.Express {
		return result.Error == nil && (time.Now().UnixNano()%100 < 98) // 98% success
	}
	return result.Error == nil && (time.Now().UnixNano()%100 < 95) // 95% success
}

// handleSuccess processes successful delivery
func (w *Worker) handleSuccess(ctx context.Context, msg *messages.Message) {
	providerMsgID := fmt.Sprintf("prov_%d", time.Now().UnixNano())
	
	// Update to SENT status
	w.store.UpdateStatus(ctx, msg.ID, messages.StatusSent, &providerMsgID, nil)
	w.store.UpdateProvider(ctx, msg.ID, "mock_provider")
	
	// Capture credits
	if err := w.billing.CaptureCredits(ctx, msg.ID); err != nil {
		w.logger.Error("Failed to capture credits", "message_id", msg.ID, "error", err)
	}
}

// handleFailure processes delivery failure with retry logic
func (w *Worker) handleFailure(ctx context.Context, msg *messages.Message) {
	maxAttempts := 3
	if msg.Express {
		maxAttempts = 5
	}

	if msg.Attempts >= maxAttempts {
		// Permanent failure
		reason := fmt.Sprintf("Failed after %d attempts", msg.Attempts)
		w.store.UpdateStatus(ctx, msg.ID, messages.StatusFailedPerm, nil, &reason)
		
		// Release credits
		if err := w.billing.ReleaseCredits(ctx, msg.ID); err != nil {
			w.logger.Error("Failed to release credits", "message_id", msg.ID, "error", err)
		}
		
		w.logger.Error("Message permanently failed", "message_id", msg.ID, "attempts", msg.Attempts)
		return
	}

	// Schedule retry with exponential backoff
	reason := fmt.Sprintf("Attempt %d failed, will retry", msg.Attempts)
	w.store.UpdateStatus(ctx, msg.ID, messages.StatusFailedTemp, nil, &reason)

	retryDelay := time.Duration(msg.Attempts) * 30 * time.Second
	if msg.Express {
		retryDelay = retryDelay / 2
	}

	// Schedule retry
	go func() {
		timer := time.NewTimer(retryDelay)
		defer timer.Stop()

		select {
		case <-timer.C:
			if err := w.queue.PublishSendJob(context.Background(), msg.ID, msg.Attempts+1); err != nil {
				w.logger.Error("Failed to schedule retry", "message_id", msg.ID, "error", err)
			}
		case <-ctx.Done():
			w.logger.Debug("Retry cancelled", "message_id", msg.ID)
		}
	}()
}

// metricsLogger reports worker performance
func (w *Worker) metricsLogger(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			processed := atomic.LoadInt64(&w.processed)
			failed := atomic.LoadInt64(&w.failed)
			concurrent := atomic.LoadInt64(&w.concurrent)

			total := processed + failed
			successRate := float64(0)
			if total > 0 {
				successRate = float64(processed) / float64(total) * 100
			}

			w.logger.Info("Worker metrics",
				"processed", processed,
				"failed", failed,
				"success_rate", fmt.Sprintf("%.1f%%", successRate),
				"concurrent", concurrent,
				"worker_pool_size", w.workerPool)
		}
	}
}