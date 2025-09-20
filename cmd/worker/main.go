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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("Starting SMS Gateway Worker", "version", "1.0.0")

	// Database
	ctx := context.Background()
	database, err := db.NewPostgres(ctx, cfg.PostgresURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Redis
	redis, err := db.NewRedis(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()

	// NATS
	queue, err := nats.NewQueue(cfg.NATSURL, logger)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer queue.Close()

	// Services
	store := messages.NewStore(database, logger)
	billingService := billing.NewService(database, logger)

	// SMS Provider
	provider := mock.NewProvider()

	// Worker (simplified for interview demo)
	w := worker.NewSimple(logger, store, billingService, queue, provider, cfg)

	// Start worker
	if err := w.Start(ctx); err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}

	logger.Info("SMS Gateway Worker started")

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down SMS Gateway Worker...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	w.Stop(ctx)

	logger.Info("SMS Gateway Worker stopped")
}
