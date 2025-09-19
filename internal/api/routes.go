package api

import (
	"fmt"
	"sms-gateway/internal/auth"
	"sms-gateway/internal/observability"
	"sms-gateway/internal/rate"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func SetupRoutes(
	app *fiber.App,
	logger *zap.Logger,
	metrics *observability.Metrics,
	handlers *Handlers,
	authService *auth.AuthService,
	rateLimiter *rate.Limiter,
) {
	// Set up middleware
	SetupMiddleware(app, logger, metrics, authService, rateLimiter)

	// Health endpoints (no auth required)
	app.Get("/healthz", handlers.HealthCheck)
	app.Get("/readyz", handlers.ReadyCheck)

	// API documentation endpoint
	app.Get("/docs", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"title":   "ArvanCloud SMS Gateway API",
			"version": "1.0",
			"endpoints": fiber.Map{
				"health":      "GET /healthz - Health check",
				"ready":       "GET /readyz - Readiness check",
				"client_info": "GET /v1/me - Get client info (requires X-API-Key: secret)",
				"send_sms":    "POST /v1/messages - Send SMS (requires X-API-Key: secret)",
				"get_message": "GET /v1/messages/{id} - Get message status (requires X-API-Key: secret)",
				"metrics":     "GET /metrics - Prometheus metrics",
			},
			"auth": "Add header: X-API-Key: secret",
			"example_send": fiber.Map{
				"method":  "POST",
				"url":     "/v1/messages",
				"headers": fiber.Map{"X-API-Key": "secret", "Content-Type": "application/json"},
				"body":    fiber.Map{"to": "+1234567890", "from": "TEST", "text": "Hello SMS Gateway!"},
			},
		})
	})

	// Swagger UI endpoint - Clean approach
	app.Get("/swagger", func(c *fiber.Ctx) error {
		// Redirect to Swagger Editor with our API spec
		return c.Redirect("https://editor.swagger.io/?url=" +
			c.Protocol() + "://" + c.Hostname() + ":" + c.Port() + "/api-spec")
	})

	// OpenAPI spec endpoint
	app.Get("/api-spec", func(c *fiber.Ctx) error {
		spec := map[string]interface{}{
			"openapi": "3.0.0",
			"info": map[string]interface{}{
				"title":       "ArvanCloud SMS Gateway API",
				"description": "Production-grade SMS Gateway with high throughput and reliability",
				"version":     "1.0.0",
				"contact": map[string]interface{}{
					"name":  "ArvanCloud SMS Gateway",
					"email": "support@arvancloud.ir",
				},
			},
			"servers": []map[string]interface{}{
				{"url": "http://localhost:8080", "description": "Development server"},
			},
			"components": map[string]interface{}{
				"securitySchemes": map[string]interface{}{
					"ApiKeyAuth": map[string]interface{}{
						"type": "apiKey",
						"in":   "header",
						"name": "X-API-Key",
					},
				},
			},
			"paths": map[string]interface{}{
				"/healthz": map[string]interface{}{
					"get": map[string]interface{}{
						"summary":     "Health Check",
						"description": "Basic health check endpoint",
						"tags":        []string{"Health"},
						"responses": map[string]interface{}{
							"200": map[string]interface{}{
								"description": "OK",
								"content": map[string]interface{}{
									"application/json": map[string]interface{}{
										"example": map[string]interface{}{
											"status":    "healthy",
											"timestamp": 1234567890,
										},
									},
								},
							},
						},
					},
				},
				"/v1/me": map[string]interface{}{
					"get": map[string]interface{}{
						"summary":     "Get Client Info",
						"description": "Get current client information and credit balance",
						"tags":        []string{"Client"},
						"security":    []map[string]interface{}{{"ApiKeyAuth": []string{}}},
						"responses": map[string]interface{}{
							"200": map[string]interface{}{
								"description": "OK",
								"content": map[string]interface{}{
									"application/json": map[string]interface{}{
										"example": map[string]interface{}{
											"id":           "550e8400-e29b-41d4-a716-446655440000",
											"name":         "Demo Client",
											"credit_cents": 95000,
										},
									},
								},
							},
						},
					},
				},
				"/v1/messages": map[string]interface{}{
					"post": map[string]interface{}{
						"summary":     "Send SMS",
						"description": "Send SMS message with optional idempotency key",
						"tags":        []string{"Messages"},
						"security":    []map[string]interface{}{{"ApiKeyAuth": []string{}}},
						"requestBody": map[string]interface{}{
							"required": true,
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":     "object",
										"required": []string{"to", "from", "text"},
										"properties": map[string]interface{}{
											"to":               map[string]interface{}{"type": "string", "example": "+1234567890"},
											"from":             map[string]interface{}{"type": "string", "example": "SENDER"},
											"text":             map[string]interface{}{"type": "string", "example": "Hello from SMS Gateway!"},
											"client_reference": map[string]interface{}{"type": "string", "example": "order-123"},
										},
									},
								},
							},
						},
						"responses": map[string]interface{}{
							"202": map[string]interface{}{
								"description": "Accepted",
								"content": map[string]interface{}{
									"application/json": map[string]interface{}{
										"example": map[string]interface{}{
											"message_id": "uuid-here",
											"status":     "QUEUED",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		return c.JSON(spec)
	})

	// Metrics endpoint (no auth required, but could be restricted in production)
	app.Get("/metrics", func(c *fiber.Ctx) error {
		// Convert Prometheus metrics to text format manually
		registry := prometheus.DefaultGatherer
		metricFamilies, err := registry.Gather()
		if err != nil {
			return c.Status(500).SendString("Error gathering metrics")
		}

		c.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		// Simple metrics output - in production you'd use proper exposition format
		for _, mf := range metricFamilies {
			name := mf.GetName()
			for _, m := range mf.GetMetric() {
				if m.GetCounter() != nil {
					c.WriteString(fmt.Sprintf("# TYPE %s counter\n%s %g\n", name, name, m.GetCounter().GetValue()))
				} else if m.GetGauge() != nil {
					c.WriteString(fmt.Sprintf("# TYPE %s gauge\n%s %g\n", name, name, m.GetGauge().GetValue()))
				} else if m.GetHistogram() != nil {
					h := m.GetHistogram()
					c.WriteString(fmt.Sprintf("# TYPE %s histogram\n%s_count %d\n%s_sum %g\n",
						name, name, h.GetSampleCount(), name, h.GetSampleSum()))
				}
			}
		}
		return nil
	})

	// API v1 routes
	v1 := app.Group("/v1")

	// Client info (requires auth)
	v1.Get("/me", authService.RequireAPIKey(), handlers.GetClientInfo)

	// Messages endpoints (requires auth)
	messages := v1.Group("/messages", authService.RequireAPIKey())
	messages.Post("/", handlers.SendMessage)
	messages.Get("/:id", handlers.GetMessage)

	// Provider webhooks (no auth for simplicity, but should have HMAC verification in production)
	providers := v1.Group("/providers")
	providers.Post("/mock/dlr", handlers.HandleDLR)
}
