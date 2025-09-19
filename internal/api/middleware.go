package api

import (
	"fmt"
	"sms-gateway/internal/auth"
	"sms-gateway/internal/observability"
	"sms-gateway/internal/rate"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"go.uber.org/zap"
)

func SetupMiddleware(app *fiber.App, logger *zap.Logger, metrics *observability.Metrics, authSvc *auth.AuthService, rateLimiter *rate.Limiter) {
	// Recovery middleware
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))

	// Request ID middleware
	app.Use(requestid.New())

	// CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization,X-API-Key,Idempotency-Key",
	}))

	// Logging middleware
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		duration := time.Since(start)
		status := c.Response().StatusCode()

		// Log the request
		logger.Info("http_request",
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.Int("status", status),
			zap.Duration("duration", duration),
			zap.String("request_id", c.Get("X-Request-ID")),
			zap.String("user_agent", c.Get("User-Agent")),
		)

		// Record metrics
		if metrics != nil {
			clientID := ""
			if client, err := auth.GetClientFromContext(c); err == nil {
				clientID = client.ID.String()
			}

			metrics.HTTPRequestsTotal.WithLabelValues(
				c.Method(),
				c.Path(),
				fmt.Sprintf("%d", status),
				clientID,
			).Inc()

			metrics.HTTPRequestDuration.WithLabelValues(
				c.Method(),
				c.Path(),
				fmt.Sprintf("%d", status),
			).Observe(duration.Seconds())
		}

		return err
	})

	// Rate limiting middleware (for authenticated routes)
	app.Use("/v1/messages", func(c *fiber.Ctx) error {
		client, err := auth.GetClientFromContext(c)
		if err != nil {
			return c.Next() // Skip rate limiting if not authenticated yet
		}

		allowed, retryAfter, err := rateLimiter.Allow(c.Context(), client.ID)
		if err != nil {
			logger.Error("rate limiting error", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "rate limiting error",
			})
		}

		if !allowed {
			c.Set("Retry-After", fmt.Sprintf("%.0f", retryAfter.Seconds()))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":               "rate limit exceeded",
				"retry_after_seconds": int(retryAfter.Seconds()),
			})
		}

		return c.Next()
	})
}
