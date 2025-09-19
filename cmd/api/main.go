package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sms-gateway/internal/api"
	"sms-gateway/internal/auth"
	"sms-gateway/internal/billing"
	"sms-gateway/internal/config"
	"sms-gateway/internal/dlr"
	"sms-gateway/internal/idempotency"
	"sms-gateway/internal/messages"
	"sms-gateway/internal/observability"
	"sms-gateway/internal/persistence"
	"sms-gateway/internal/queue/nats"
	"sms-gateway/internal/rate"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger, err := observability.NewLogger(cfg.LogLevel)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting SMS Gateway API", zap.String("version", "1.0.0"))

	// Initialize database
	ctx := context.Background()
	db, err := persistence.NewPostgres(ctx, cfg.PostgresURL)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	// Run migrations
	if err := db.RunMigrations("migrations"); err != nil {
		logger.Warn("Failed to run migrations (may already be applied)", zap.Error(err))
	}

	// Initialize Redis
	redis, err := persistence.NewRedis(ctx, cfg.RedisURL)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redis.Close()

	// Initialize NATS
	queue, err := nats.NewQueue(cfg.NATSURL, logger)
	if err != nil {
		logger.Fatal("Failed to connect to NATS", zap.Error(err))
	}
	defer queue.Close()

	// Initialize services
	metrics := observability.NewMetrics()
	messageStore := messages.NewStore(db, logger)
	authService := auth.NewAuthService(db, logger)
	billingService := billing.NewBillingService(db, logger)
	rateLimiter := rate.NewLimiter(redis, logger, cfg.RateLimitRPS, cfg.RateLimitBurst)
	idempotencyStore := idempotency.NewStore(db, redis, logger)

	// Initialize DLR service
	dlrService := dlr.NewService(logger, metrics, messageStore, authService, billingService)

	// Initialize API handlers
	handlers := api.NewHandlers(logger, metrics, messageStore, idempotencyStore, billingService, queue, dlrService, cfg.PricePerPartCents)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger.Error("Fiber error", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Internal server error",
			})
		},
	})

	// Setup routes
	api.SetupRoutes(app, logger, metrics, handlers, authService, rateLimiter)

	// Start server in a goroutine
	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	logger.Info("SMS Gateway API started", zap.String("port", cfg.Port))

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down SMS Gateway API...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		logger.Error("Failed to shutdown server gracefully", zap.Error(err))
	}

	logger.Info("SMS Gateway API stopped")
}
