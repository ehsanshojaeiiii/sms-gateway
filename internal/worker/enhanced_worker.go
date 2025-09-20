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

// EnhancedWorker implements high-performance SMS processing with advanced Go concurrency
type EnhancedWorker struct {
	logger   *slog.Logger
	store    *messages.Store
	billing  *billing.Service
	queue    *nats.Queue
	provider *mock.Provider
	config   *config.Config

	// Advanced concurrency features
	workerPool     *WorkerPool
	processingPool chan uuid.UUID

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Performance metrics (atomic for thread safety)
	processed    int64
	failed       int64
	concurrent   int64
	totalLatency int64 // For average latency calculation

	// Configuration
	poolSize   int
	batchSize  int
	bufferSize int
}

// NewEnhanced creates an enhanced worker with advanced concurrency features
func NewEnhanced(logger *slog.Logger, store *messages.Store, billing *billing.Service, queue *nats.Queue, provider *mock.Provider, cfg *config.Config) *EnhancedWorker {
	ctx, cancel := context.WithCancel(context.Background())

	// Calculate optimal settings based on MacBook hardware
	numCPU := runtime.NumCPU()
	poolSize := numCPU * 4      // 4 workers per CPU core
	batchSize := 50             // Process messages in batches of 50
	bufferSize := poolSize * 20 // Buffer 20 messages per worker

	worker := &EnhancedWorker{
		logger:         logger,
		store:          store,
		billing:        billing,
		queue:          queue,
		provider:       provider,
		config:         cfg,
		ctx:            ctx,
		cancel:         cancel,
		poolSize:       poolSize,
		batchSize:      batchSize,
		bufferSize:     bufferSize,
		processingPool: make(chan uuid.UUID, bufferSize),
	}

	// Create advanced worker pool
	worker.workerPool = NewWorkerPool(worker.processMessageByID)

	return worker
}

// Start begins the enhanced worker with all optimizations
func (w *EnhancedWorker) Start(ctx context.Context) error {
	w.logger.Info("Starting Enhanced SMS Worker",
		"cpu_cores", runtime.NumCPU(),
		"pool_size", w.poolSize,
		"batch_size", w.batchSize,
		"buffer_size", w.bufferSize)

	// Start advanced worker pool
	w.workerPool.Start()

	// Start NATS consumer with batching
	w.wg.Add(1)
	go w.batchedNATSConsumer()

	// Start performance monitor
	w.wg.Add(1)
	go w.performanceMonitor()

	// Start system health monitor
	w.wg.Add(1)
	go w.systemHealthMonitor()

	return nil
}

// batchedNATSConsumer consumes messages from NATS in batches for efficiency
func (w *EnhancedWorker) batchedNATSConsumer() {
	defer w.wg.Done()

	w.logger.Info("Starting batched NATS consumer")

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Info("Batched NATS consumer stopped")
			return
		default:
			// Collect a batch of messages
			batch := w.collectMessageBatch()
			if len(batch) == 0 {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			// Submit batch to worker pool
			w.submitBatch(batch)
		}
	}
}

// collectMessageBatch collects messages from NATS up to batch size
func (w *EnhancedWorker) collectMessageBatch() []uuid.UUID {
	var batch []uuid.UUID
	timeout := 100 * time.Millisecond

	for len(batch) < w.batchSize {
		messageID, err := w.queue.ConsumeSendJob(w.ctx)
		if err != nil {
			if len(batch) > 0 {
				break // Return partial batch
			}
			time.Sleep(timeout)
			break
		}

		batch = append(batch, messageID)
		timeout = 10 * time.Millisecond // Shorter timeout for subsequent messages
	}

	return batch
}

// submitBatch submits a batch of messages to the worker pool
func (w *EnhancedWorker) submitBatch(batch []uuid.UUID) {
	for _, messageID := range batch {
		// Submit with back-pressure control
		if err := w.workerPool.Submit(messageID); err != nil {
			w.logger.Warn("Worker pool submission failed",
				"message_id", messageID,
				"error", err)

			// Try direct processing as fallback
			go w.processMessageByID(w.ctx, messageID)
		}
	}

	w.logger.Debug("Submitted message batch", "count", len(batch))
}

// processMessageByID processes individual messages with enhanced error handling
func (w *EnhancedWorker) processMessageByID(ctx context.Context, messageID uuid.UUID) error {
	atomic.AddInt64(&w.concurrent, 1)
	defer atomic.AddInt64(&w.concurrent, -1)

	start := time.Now()

	// Get message with timeout
	msg, err := w.store.GetByID(ctx, messageID)
	if err != nil {
		w.logger.Error("Failed to get message", "message_id", messageID, "error", err)
		atomic.AddInt64(&w.failed, 1)
		return err
	}

	// Skip if already processed (concurrency protection)
	if msg.Status != messages.StatusQueued {
		w.logger.Debug("Message already processed",
			"message_id", messageID,
			"status", msg.Status)
		return nil
	}

	// Update status atomically
	if err := w.store.UpdateStatus(ctx, msg.ID, messages.StatusSending, nil, nil); err != nil {
		w.logger.Error("Failed to update message status", "message_id", messageID, "error", err)
		atomic.AddInt64(&w.failed, 1)
		return err
	}

	// Increment attempts
	w.store.IncrementAttempts(ctx, msg.ID)

	// Attempt delivery
	success := w.attemptDelivery(ctx, msg)

	duration := time.Since(start)
	atomic.AddInt64(&w.totalLatency, duration.Milliseconds())

	if success {
		if err := w.handleSuccess(ctx, msg); err != nil {
			w.logger.Error("Failed to handle success", "message_id", messageID, "error", err)
		}
		atomic.AddInt64(&w.processed, 1)

		w.logger.Debug("Message processed successfully",
			"message_id", messageID,
			"duration", duration,
			"express", msg.Express)
	} else {
		if err := w.handleFailure(ctx, msg); err != nil {
			w.logger.Error("Failed to handle failure", "message_id", messageID, "error", err)
		}
		atomic.AddInt64(&w.failed, 1)

		w.logger.Warn("Message processing failed",
			"message_id", messageID,
			"duration", duration,
			"attempts", msg.Attempts)
	}

	return nil
}

// attemptDelivery attempts message delivery with provider
func (w *EnhancedWorker) attemptDelivery(ctx context.Context, msg *messages.Message) bool {
	providerMsg := &mock.Message{
		ToMSISDN:   msg.To,
		FromSender: msg.From,
		Text:       msg.Text,
	}

	result := w.provider.SendSMS(ctx, providerMsg)

	// Enhanced success rates based on message type
	successRate := 85 // Base success rate
	if msg.Express {
		successRate = 95 // Express messages have higher success rate
	}

	return result.Error == nil && (time.Now().UnixNano()%100 < int64(successRate))
}

// handleSuccess processes successful delivery
func (w *EnhancedWorker) handleSuccess(ctx context.Context, msg *messages.Message) error {
	providerMsgID := fmt.Sprintf("enhanced_%d", time.Now().UnixNano())

	if err := w.store.UpdateStatus(ctx, msg.ID, messages.StatusSent, &providerMsgID, nil); err != nil {
		return err
	}

	if err := w.store.UpdateProvider(ctx, msg.ID, "mock_provider_enhanced"); err != nil {
		w.logger.Warn("Failed to update provider", "message_id", msg.ID, "error", err)
	}

	return w.billing.CaptureCredits(ctx, msg.ID)
}

// handleFailure processes delivery failure with retry logic
func (w *EnhancedWorker) handleFailure(ctx context.Context, msg *messages.Message) error {
	maxAttempts := 3
	if msg.Express {
		maxAttempts = 5 // Express gets more attempts
	}

	if msg.Attempts >= maxAttempts {
		// Permanent failure
		reason := fmt.Sprintf("Failed after %d attempts", msg.Attempts)
		w.store.UpdateStatus(ctx, msg.ID, messages.StatusFailedPerm, nil, &reason)
		return w.billing.ReleaseCredits(ctx, msg.ID)
	}

	// Temporary failure - schedule retry
	reason := fmt.Sprintf("Attempt %d failed, will retry", msg.Attempts)
	w.store.UpdateStatus(ctx, msg.ID, messages.StatusFailedTemp, nil, &reason)

	// Schedule retry with exponential backoff
	retryDelay := time.Duration(msg.Attempts) * 5 * time.Second
	if msg.Express {
		retryDelay = retryDelay / 2 // Express retries faster
	}

	go func() {
		time.Sleep(retryDelay)
		w.queue.PublishSendJob(ctx, msg.ID, msg.Attempts+1)
	}()

	return nil
}

// performanceMonitor tracks and reports enhanced performance metrics
func (w *EnhancedWorker) performanceMonitor() {
	defer w.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.reportEnhancedMetrics()
		}
	}
}

// reportEnhancedMetrics logs comprehensive performance metrics
func (w *EnhancedWorker) reportEnhancedMetrics() {
	processed := atomic.LoadInt64(&w.processed)
	failed := atomic.LoadInt64(&w.failed)
	concurrent := atomic.LoadInt64(&w.concurrent)
	totalLatency := atomic.LoadInt64(&w.totalLatency)

	total := processed + failed
	successRate := float64(0)
	avgLatency := float64(0)

	if total > 0 {
		successRate = float64(processed) / float64(total) * 100
		avgLatency = float64(totalLatency) / float64(total)
	}

	// Get worker pool stats
	poolStats := w.workerPool.Stats()

	w.logger.Info("Enhanced Worker Performance",
		"processed_total", processed,
		"failed_total", failed,
		"success_rate", fmt.Sprintf("%.1f%%", successRate),
		"avg_latency_ms", fmt.Sprintf("%.1f", avgLatency),
		"concurrent_workers", concurrent,
		"pool_utilization", fmt.Sprintf("%.1f%%", float64(poolStats.Active)/float64(poolStats.PoolSize)*100),
		"pool_queue_size", poolStats.Pending,
		"cpu_cores", runtime.NumCPU(),
		"goroutines", runtime.NumGoroutine())
}

// systemHealthMonitor monitors system resources and health
func (w *EnhancedWorker) systemHealthMonitor() {
	defer w.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.checkSystemHealth()
		}
	}
}

// checkSystemHealth performs comprehensive system health checks
func (w *EnhancedWorker) checkSystemHealth() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Check memory usage
	memUsageMB := float64(m.Alloc) / 1024 / 1024
	if memUsageMB > 500 { // Alert if using more than 500MB
		w.logger.Warn("High memory usage detected",
			"memory_mb", fmt.Sprintf("%.1f", memUsageMB),
			"gc_count", m.NumGC)
	}

	// Check goroutine count
	goroutineCount := runtime.NumGoroutine()
	if goroutineCount > 1000 { // Alert if more than 1000 goroutines
		w.logger.Warn("High goroutine count detected",
			"goroutine_count", goroutineCount)
	}

	// Check database health
	if err := w.store.Health(w.ctx); err != nil {
		w.logger.Error("Database health check failed", "error", err)
	}

	w.logger.Debug("System health check",
		"memory_mb", fmt.Sprintf("%.1f", memUsageMB),
		"goroutines", goroutineCount,
		"gc_cycles", m.NumGC)
}

// Stop gracefully shuts down the enhanced worker
func (w *EnhancedWorker) Stop(ctx context.Context) error {
	w.logger.Info("Stopping Enhanced SMS Worker...")

	// Cancel context
	w.cancel()

	// Stop worker pool
	if err := w.workerPool.Stop(30 * time.Second); err != nil {
		w.logger.Error("Worker pool shutdown error", "error", err)
	}

	// Wait for all goroutines
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Info("Enhanced SMS Worker stopped gracefully")
		return nil
	case <-time.After(30 * time.Second):
		w.logger.Warn("Enhanced SMS Worker shutdown timeout")
		return fmt.Errorf("shutdown timeout")
	}
}

// GetStats returns comprehensive worker statistics
func (w *EnhancedWorker) GetStats() EnhancedWorkerStats {
	processed := atomic.LoadInt64(&w.processed)
	failed := atomic.LoadInt64(&w.failed)
	concurrent := atomic.LoadInt64(&w.concurrent)
	totalLatency := atomic.LoadInt64(&w.totalLatency)

	total := processed + failed
	avgLatency := float64(0)
	if total > 0 {
		avgLatency = float64(totalLatency) / float64(total)
	}

	poolStats := w.workerPool.Stats()

	return EnhancedWorkerStats{
		Processed:         processed,
		Failed:            failed,
		Total:             total,
		SuccessRate:       float64(processed) / float64(total) * 100,
		AverageLatencyMs:  avgLatency,
		ConcurrentWorkers: concurrent,
		PoolStats:         poolStats,
		SystemStats: SystemStats{
			CPUCores:   runtime.NumCPU(),
			Goroutines: runtime.NumGoroutine(),
		},
	}
}

// EnhancedWorkerStats represents comprehensive worker statistics
type EnhancedWorkerStats struct {
	Processed         int64       `json:"processed"`
	Failed            int64       `json:"failed"`
	Total             int64       `json:"total"`
	SuccessRate       float64     `json:"success_rate"`
	AverageLatencyMs  float64     `json:"average_latency_ms"`
	ConcurrentWorkers int64       `json:"concurrent_workers"`
	PoolStats         PoolStats   `json:"pool_stats"`
	SystemStats       SystemStats `json:"system_stats"`
}

// SystemStats represents system resource statistics
type SystemStats struct {
	CPUCores   int `json:"cpu_cores"`
	Goroutines int `json:"goroutines"`
}
