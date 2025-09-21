package api

import (
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"sms-gateway/internal/config"
)

// ConcurrencyLimiter manages concurrent requests using atomic operations
type ConcurrencyLimiter struct {
	maxConcurrent int32
	active        int32
}

// NewConcurrencyLimiter creates a simple concurrency limiter
func NewConcurrencyLimiter(maxConcurrent int32) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{
		maxConcurrent: maxConcurrent,
	}
}

// Handler returns middleware that limits concurrent requests
func (cl *ConcurrencyLimiter) Handler(logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Try to acquire slot
		current := atomic.AddInt32(&cl.active, 1)
		if current > cl.maxConcurrent {
			atomic.AddInt32(&cl.active, -1)
			logger.Warn("Concurrency limit reached", "active", current-1, "max", cl.maxConcurrent)
			return c.Status(503).JSON(fiber.Map{
				"error":  "Server temporarily overloaded, please retry",
				"active": current - 1,
			})
		}

		// Release slot when done
		defer atomic.AddInt32(&cl.active, -1)
		return c.Next()
	}
}

// SetupMiddleware configures middleware in the right order
func SetupMiddleware(app *fiber.App, logger *slog.Logger, cfg *config.Config) {
	logger.Info("Setting up middleware", "rate_limit_enabled", cfg.RateLimitEnabled)

	// 1. Recovery and request ID (always first)
	app.Use(recover.New())
	app.Use(requestid.New())

	// 2. CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// 3. Rate limiting (if enabled)
	if cfg.RateLimitEnabled {
		logger.Info("Rate limiting enabled", "rpm", cfg.RateLimitRPM, "concurrent", cfg.RateLimitConcurrent)

		// Requests per minute limit
		app.Use(limiter.New(limiter.Config{
			Max:        cfg.RateLimitRPM,
			Expiration: 1 * time.Minute,
			KeyGenerator: func(c *fiber.Ctx) string {
				return c.IP()
			},
			LimitReached: func(c *fiber.Ctx) error {
				return c.Status(429).JSON(fiber.Map{
					"error":       "Too many requests, please slow down",
					"retry_after": "60 seconds",
				})
			},
		}))

		// Concurrent request limit
		if cfg.RateLimitConcurrent > 0 {
			concurrencyLimiter := NewConcurrencyLimiter(int32(cfg.RateLimitConcurrent))
			app.Use(concurrencyLimiter.Handler(logger))
		}
	}

	// 4. Request logging (always last)
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		status := c.Response().StatusCode()
		duration := time.Since(start)

		// Log level based on status and duration
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 || duration > time.Second {
			level = slog.LevelWarn
		}

		logger.Log(c.Context(), level, "request",
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"duration", duration,
			"ip", c.IP(),
		)
		return err
	})

	logger.Info("Middleware setup completed")
}
