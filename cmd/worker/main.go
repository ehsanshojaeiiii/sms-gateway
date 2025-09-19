package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sms-gateway/internal/billing"
	"sms-gateway/internal/config"
	"sms-gateway/internal/messages"
	"sms-gateway/internal/observability"
	"sms-gateway/internal/persistence"
	"sms-gateway/internal/provider/mock"
	"sms-gateway/internal/queue/nats"
	"syscall"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Setup logger
	logger := observability.GetLoggerFromEnv()
	defer logger.Sync()

	logger.Info("starting SMS Gateway Worker",
		zap.String("log_level", cfg.LogLevel))

	// Setup metrics
	var metrics *observability.Metrics
	if cfg.MetricsEnabled {
        metrics = observability.NewMetrics()
	}

	// Setup database connections
	ctx := context.Background()

	postgres, err := persistence.NewPostgres(ctx, cfg.PostgresURL)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer postgres.Close()

	// Setup queue
	queue, err := nats.NewQueue(cfg.NATSURL, logger)
	if err != nil {
		logger.Fatal("failed to connect to NATS", zap.Error(err))
	}
	defer queue.Close()

	// Setup provider
	provider := mock.NewProvider(
		logger,
		cfg.MockSuccessRate,
		cfg.MockTempFailRate,
		cfg.MockPermFailRate,
		cfg.MockLatencyMs,
	)

	// Initialize services
	messageStore := messages.NewStore(postgres, logger)
	billingService := billing.NewBillingService(postgres, logger)

	// Initialize worker service
	workerService := messages.NewWorkerService(
		logger,
		metrics,
		messageStore,
		billingService,
		queue,
		provider,
		cfg,
	)

	// Create worker pool for concurrent message processing
	const numWorkers = 5
	jobChan := make(chan *nats.SendJob, 100) // Buffered channel for jobs

	// Start worker pool
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			logger.Info("worker started", zap.Int("worker_id", workerID))
			for job := range jobChan {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

				logger.Debug("worker processing job",
					zap.Int("worker_id", workerID),
					zap.String("message_id", job.MessageID.String()))

				if err := workerService.ProcessMessage(ctx, job); err != nil {
					logger.Error("worker failed to process message job",
						zap.Int("worker_id", workerID),
						zap.String("message_id", job.MessageID.String()),
						zap.Int("attempt", job.Attempt),
						zap.Error(err))
				}
				cancel()
			}
		}(i)
	}

	// Subscribe to send jobs and feed them to worker pool
	subscription, err := queue.SubscribeSendJobs(func(job *nats.SendJob) error {
		// Non-blocking send to worker pool
		select {
		case jobChan <- job:
			return nil
		default:
			logger.Warn("worker pool full, dropping job", zap.String("message_id", job.MessageID.String()))
			return fmt.Errorf("worker pool saturated")
		}
	})

	if err != nil {
		logger.Fatal("failed to subscribe to send jobs", zap.Error(err))
	}
	defer subscription.Unsubscribe()

	// Subscribe to DLQ for monitoring (optional)
	dlqSubscription, err := queue.SubscribeDLQJobs(func(messageID uuid.UUID, reason string, timestamp time.Time) {
		logger.Warn("message sent to DLQ",
			zap.String("message_id", messageID.String()),
			zap.String("reason", reason),
			zap.Time("timestamp", timestamp))
	})

	if err != nil {
		logger.Error("failed to subscribe to DLQ", zap.Error(err))
	} else {
		defer dlqSubscription.Unsubscribe()
	}

	logger.Info("worker started, waiting for messages...")

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	logger.Info("shutting down worker...")

	// Close job channel to stop workers
	close(jobChan)

	// Give ongoing jobs time to complete
	time.Sleep(5 * time.Second)
	logger.Info("worker shutdown complete")
}
