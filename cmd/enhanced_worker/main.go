package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sms-gateway/internal/billing"
	"sms-gateway/internal/config"
	"sms-gateway/internal/db"
	"sms-gateway/internal/messages"
	"sms-gateway/internal/messaging/nats"
	"sms-gateway/internal/providers/mock"
	"sms-gateway/internal/worker"
	"syscall"
	"time"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Load worker-specific configuration
	workerCfg := config.GetWorkerConfig()

	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("Starting SMS Gateway Worker", 
		"version", "2.0.0",
		"worker_mode", workerCfg.Mode,
		"pool_size", workerCfg.PoolSize,
		"batch_size", workerCfg.BatchSize)

	// Database connection
	ctx := context.Background()
	database, err := db.NewPostgres(ctx, cfg.PostgresURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Redis connection
	redis, err := db.NewRedis(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()

	// NATS connection
	queue, err := nats.NewQueue(cfg.NATSURL, logger)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer queue.Close()

	// Services
	store := messages.NewStore(database, logger)
	billingService := billing.NewService(database, logger)
	provider := mock.NewProvider()

	// Create worker based on configuration
	var workerInstance WorkerInterface
	switch workerCfg.Mode {
	case config.WorkerModeEnhanced:
		logger.Info("Creating Enhanced Worker with advanced concurrency")
		workerInstance = worker.NewEnhanced(logger, store, billingService, queue, provider, cfg)
	default:
		logger.Info("Creating Simple Worker (backward compatibility)")
		workerInstance = worker.NewSimple(logger, store, billingService, queue, provider, cfg)
	}

	// Start worker
	if err := workerInstance.Start(ctx); err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}

	logger.Info("SMS Gateway Worker started successfully")

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down SMS Gateway Worker...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := workerInstance.Stop(shutdownCtx); err != nil {
		logger.Error("Failed to shutdown worker gracefully", "error", err)
	}

	logger.Info("SMS Gateway Worker stopped")
}

// WorkerInterface defines the common interface for all worker implementations
type WorkerInterface interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
