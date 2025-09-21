package worker

import (
	"context"
	"log/slog"
	"runtime"
	"sms-gateway/internal/billing"
	"sms-gateway/internal/config"
	"sms-gateway/internal/messages"
	"sms-gateway/internal/providers/mock"
	"sms-gateway/internal/queue"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Worker processes SMS messages using database polling and Go channels
type Worker struct {
	logger   *slog.Logger
	billing  *billing.Service
	queue    *queue.Queue
	provider *mock.Provider

	// Go channels - proper way to share memory by communicating
	jobs    chan *messages.Message
	results chan result
	stop    chan struct{}
	wg      sync.WaitGroup

	// Atomic counters
	processed int64
	failed    int64
}

type result struct {
	id      uuid.UUID
	success bool
	err     error
}

// New creates a worker with optimal configuration
func New(logger *slog.Logger, store *messages.Store, billing *billing.Service,
	provider *mock.Provider, cfg *config.Config) *Worker {

	return &Worker{
		logger:   logger,
		billing:  billing,
		queue:    queue.New(store, logger),
		provider: provider,
		jobs:     make(chan *messages.Message, 200),
		results:  make(chan result, 200),
		stop:     make(chan struct{}),
	}
}

// Start the worker pool
func (w *Worker) Start(ctx context.Context) error {
	workers := runtime.NumCPU() * 10 // I/O bound work
	w.logger.Info("Starting SMS Worker", "workers", workers)

	// Start workers
	for i := 0; i < workers; i++ {
		w.wg.Add(1)
		go w.worker(ctx)
	}

	// Start poller
	w.wg.Add(1)
	go w.poll(ctx)

	// Start result processor
	w.wg.Add(1)
	go w.processResults(ctx)

	// Start retry processor
	w.wg.Add(1)
	go w.retryLoop(ctx)

	// Start metrics
	w.wg.Add(1)
	go w.metrics(ctx)

	return nil
}

// Stop gracefully shuts down
func (w *Worker) Stop() error {
	close(w.stop)
	w.wg.Wait()
	return nil
}

// poll continuously fetches messages from database
func (w *Worker) poll(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(50 * time.Millisecond) // Fast polling
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			msgs, err := w.queue.Poll(ctx, 20) // Batch size
			if err != nil {
				w.logger.Error("Poll failed", "error", err)
				continue
			}

			// Send to workers via channels
			for _, msg := range msgs {
				select {
				case w.jobs <- msg:
				case <-w.stop:
					return
				}
			}
		}
	}
}

// worker processes individual messages
func (w *Worker) worker(ctx context.Context) {
	defer w.wg.Done()

	for {
		select {
		case <-w.stop:
			return
		case msg := <-w.jobs:
			// Send SMS
			mockMsg := &mock.Message{
				ID:         msg.ID,
				ToMSISDN:   msg.To,
				FromSender: msg.From,
				Text:       msg.Text,
			}
			providerResult := w.provider.SendSMS(ctx, mockMsg)

			// Send result via channel
			success := providerResult.Status == mock.StatusSent
			var err error
			if providerResult.Error != nil {
				err = providerResult.Error
			}

			select {
			case w.results <- result{id: msg.ID, success: success, err: err}:
			case <-w.stop:
				return
			}
		}
	}
}

// processResults handles job results
func (w *Worker) processResults(ctx context.Context) {
	defer w.wg.Done()

	for {
		select {
		case <-w.stop:
			return
		case res := <-w.results:
			if res.success {
				w.queue.Complete(ctx, res.id)
				w.billing.CaptureCredits(ctx, res.id)
				atomic.AddInt64(&w.processed, 1)
			} else {
				w.queue.Fail(ctx, res.id, res.err.Error())
				atomic.AddInt64(&w.failed, 1)
			}
		}
	}
}

// retryLoop handles message retries
func (w *Worker) retryLoop(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			count, _ := w.queue.Retry(ctx)
			if count > 0 {
				w.logger.Info("Retried messages", "count", count)
			}
		}
	}
}

// metrics reports performance
func (w *Worker) metrics(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			processed := atomic.LoadInt64(&w.processed)
			failed := atomic.LoadInt64(&w.failed)
			total := processed + failed

			if total > 0 {
				successRate := float64(processed) / float64(total) * 100
				w.logger.Info("Worker Stats",
					"processed", processed,
					"failed", failed,
					"success_rate", successRate)
			}
		}
	}
}
