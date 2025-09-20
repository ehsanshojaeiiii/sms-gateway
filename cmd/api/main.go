package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sms-gateway/internal/api"
	"sms-gateway/internal/billing"
	"sms-gateway/internal/config"
	"sms-gateway/internal/db"
	"sms-gateway/internal/delivery"
	"sms-gateway/internal/messages"
	"sms-gateway/internal/messaging/nats"
	"sms-gateway/internal/otp"
	"sms-gateway/internal/providers/mock"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("Starting SMS Gateway API", "version", "1.0.0")

	// Database
	ctx := context.Background()
	database, err := db.NewPostgres(ctx, cfg.PostgresURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := database.RunMigrations("migrations"); err != nil {
		logger.Warn("Failed to run migrations", "error", err)
	}

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
	deliveryService := delivery.NewService(logger, store, billingService)

	// SMS Provider and OTP service for delivery guarantee
	provider := mock.NewProvider()
	otpService := otp.NewOTPService(logger, provider)

	// Handlers
	handlers := api.NewHandlers(logger, store, billingService, queue, deliveryService, otpService, cfg.PricePerPartCents, cfg.ExpressSurchargeCents)

	// App
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger.Error("Fiber error", "error", err)
			return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
		},
	})

	api.SetupRoutes(app, logger, handlers)

	// Start server
	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	logger.Info("SMS Gateway API started", "port", cfg.Port)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		logger.Error("Failed to shutdown gracefully", "error", err)
	}

	logger.Info("SMS Gateway stopped")
}
