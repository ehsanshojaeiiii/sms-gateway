package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// AdvancedConsumer implements high-performance NATS consumption with connection multiplexing
type AdvancedConsumer struct {
	conn         *nats.Conn
	logger       *slog.Logger
	
	// Connection multiplexing
	subscriptions map[string]*nats.Subscription
	subMutex      sync.RWMutex
	
	// Performance optimization
	batchSize     int
	prefetchCount int
	workerCount   int
	
	// Message handling
	messageHandler func(context.Context, uuid.UUID) error
	
	// Lifecycle management
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	
	// Metrics
	consumed      int64
	processed     int64
	failed        int64
	
	// Back-pressure control
	processingChan chan uuid.UUID
}

// AdvancedConsumerConfig defines configuration for the advanced consumer
type AdvancedConsumerConfig struct {
	BatchSize     int    // Number of messages to fetch in batch
	PrefetchCount int    // Number of messages to prefetch
	WorkerCount   int    // Number of parallel workers
	QueueGroup    string // NATS queue group for load balancing
}

// NewAdvancedConsumer creates a high-performance NATS consumer
func NewAdvancedConsumer(conn *nats.Conn, logger *slog.Logger, handler func(context.Context, uuid.UUID) error) *AdvancedConsumer {
	ctx, cancel := context.WithCancel(context.Background())
	
	config := AdvancedConsumerConfig{
		BatchSize:     50,                    // Fetch 50 messages at a time
		PrefetchCount: 100,                   // Prefetch 100 messages
		WorkerCount:   20,                    // 20 parallel workers
		QueueGroup:    "sms-workers-v2",      // Load balancing group
	}
	
	return &AdvancedConsumer{
		conn:           conn,
		logger:         logger,
		subscriptions:  make(map[string]*nats.Subscription),
		batchSize:      config.BatchSize,
		prefetchCount:  config.PrefetchCount,
		workerCount:    config.WorkerCount,
		messageHandler: handler,
		ctx:            ctx,
		cancel:         cancel,
		processingChan: make(chan uuid.UUID, config.PrefetchCount),
	}
}

// Start begins consuming messages with advanced patterns
func (c *AdvancedConsumer) Start() error {
	c.logger.Info("Starting advanced NATS consumer",
		"batch_size", c.batchSize,
		"prefetch_count", c.prefetchCount,
		"worker_count", c.workerCount)

	// Start multiple workers for parallel processing
	for i := 0; i < c.workerCount; i++ {
		c.wg.Add(1)
		go c.messageWorker(i)
	}

	// Start batch consumer
	c.wg.Add(1)
	go c.batchConsumer()

	// Start metrics reporter
	c.wg.Add(1)
	go c.metricsReporter()

	return nil
}

// batchConsumer fetches messages in batches for efficiency
func (c *AdvancedConsumer) batchConsumer() {
	defer c.wg.Done()

	// Subscribe with queue group for load balancing
	sub, err := c.conn.QueueSubscribeSync("sms.send", "sms-workers-advanced")
	if err != nil {
		c.logger.Error("Failed to create batch subscription", "error", err)
		return
	}
	defer sub.Unsubscribe()

	// Set subscription limits for performance
	sub.SetPendingLimits(c.prefetchCount*2, 16*1024*1024) // 2x prefetch, 16MB

	c.logger.Info("Batch consumer started", "subject", "sms.send")

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Info("Batch consumer stopping")
			return
		default:
			// Fetch batch of messages
			messages := c.fetchBatch(sub)
			if len(messages) == 0 {
				time.Sleep(10 * time.Millisecond) // Brief pause when no messages
				continue
			}

			// Process batch
			c.processBatch(messages)
		}
	}
}

// fetchBatch fetches a batch of messages efficiently
func (c *AdvancedConsumer) fetchBatch(sub *nats.Subscription) []*nats.Msg {
	var messages []*nats.Msg
	timeout := 100 * time.Millisecond

	// Try to fetch up to batchSize messages
	for len(messages) < c.batchSize {
		msg, err := sub.NextMsg(timeout)
		if err != nil {
			if err == nats.ErrTimeout {
				break // No more messages available
			}
			c.logger.Error("Error fetching message", "error", err)
			break
		}
		messages = append(messages, msg)
		
		// Use shorter timeout for subsequent messages in batch
		timeout = 10 * time.Millisecond
	}

	if len(messages) > 0 {
		atomic.AddInt64(&c.consumed, int64(len(messages)))
		c.logger.Debug("Fetched message batch", "count", len(messages))
	}

	return messages
}

// processBatch processes a batch of messages
func (c *AdvancedConsumer) processBatch(messages []*nats.Msg) {
	for _, msg := range messages {
		// Parse message
		var job SendJob
		if err := json.Unmarshal(msg.Data, &job); err != nil {
			c.logger.Error("Failed to unmarshal message", "error", err)
			atomic.AddInt64(&c.failed, 1)
			continue
		}

		// Send to worker pool with back-pressure control
		select {
		case c.processingChan <- job.MessageID:
			// Successfully queued for processing
		case <-c.ctx.Done():
			return
		default:
			// Processing channel full, apply back-pressure
			c.logger.Warn("Processing channel full, applying back-pressure")
			time.Sleep(50 * time.Millisecond)
			
			// Try again
			select {
			case c.processingChan <- job.MessageID:
			case <-c.ctx.Done():
				return
			default:
				c.logger.Error("Failed to queue message for processing")
				atomic.AddInt64(&c.failed, 1)
			}
		}
	}
}

// messageWorker processes messages from the processing channel
func (c *AdvancedConsumer) messageWorker(workerID int) {
	defer c.wg.Done()

	c.logger.Debug("Message worker started", "worker_id", workerID)

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("Message worker stopping", "worker_id", workerID)
			return
		case messageID := <-c.processingChan:
			c.processMessage(workerID, messageID)
		}
	}
}

// processMessage handles individual message processing
func (c *AdvancedConsumer) processMessage(workerID int, messageID uuid.UUID) {
	start := time.Now()

	// Process message with timeout context
	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	err := c.messageHandler(ctx, messageID)
	
	duration := time.Since(start)

	if err != nil {
		atomic.AddInt64(&c.failed, 1)
		c.logger.Error("Message processing failed",
			"worker_id", workerID,
			"message_id", messageID,
			"duration", duration,
			"error", err)
	} else {
		atomic.AddInt64(&c.processed, 1)
		c.logger.Debug("Message processed successfully",
			"worker_id", workerID,
			"message_id", messageID,
			"duration", duration)
	}
}

// metricsReporter reports consumer performance metrics
func (c *AdvancedConsumer) metricsReporter() {
	defer c.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.reportMetrics()
		}
	}
}

// reportMetrics logs current consumer performance
func (c *AdvancedConsumer) reportMetrics() {
	consumed := atomic.LoadInt64(&c.consumed)
	processed := atomic.LoadInt64(&c.processed)
	failed := atomic.LoadInt64(&c.failed)
	queueSize := len(c.processingChan)
	
	successRate := float64(0)
	if consumed > 0 {
		successRate = float64(processed) / float64(consumed) * 100
	}

	c.logger.Info("Advanced consumer metrics",
		"consumed_total", consumed,
		"processed_total", processed,
		"failed_total", failed,
		"success_rate", fmt.Sprintf("%.1f%%", successRate),
		"queue_size", queueSize,
		"queue_capacity", cap(c.processingChan),
		"worker_count", c.workerCount)
}

// Stop gracefully shuts down the consumer
func (c *AdvancedConsumer) Stop(timeout time.Duration) error {
	c.logger.Info("Stopping advanced NATS consumer")

	c.cancel()

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("Advanced NATS consumer stopped gracefully")
		return nil
	case <-time.After(timeout):
		c.logger.Warn("Advanced NATS consumer shutdown timeout")
		return fmt.Errorf("shutdown timeout exceeded")
	}
}

// GetStats returns current consumer statistics
func (c *AdvancedConsumer) GetStats() ConsumerStats {
	return ConsumerStats{
		Consumed:         atomic.LoadInt64(&c.consumed),
		Processed:        atomic.LoadInt64(&c.processed),
		Failed:           atomic.LoadInt64(&c.failed),
		QueueSize:        len(c.processingChan),
		QueueCapacity:    cap(c.processingChan),
		WorkerCount:      c.workerCount,
		BatchSize:        c.batchSize,
		QueueUtilization: float64(len(c.processingChan)) / float64(cap(c.processingChan)),
	}
}

// ConsumerStats represents consumer performance statistics
type ConsumerStats struct {
	Consumed         int64   `json:"consumed"`
	Processed        int64   `json:"processed"`
	Failed           int64   `json:"failed"`
	QueueSize        int     `json:"queue_size"`
	QueueCapacity    int     `json:"queue_capacity"`
	WorkerCount      int     `json:"worker_count"`
	BatchSize        int     `json:"batch_size"`
	QueueUtilization float64 `json:"queue_utilization"`
}
