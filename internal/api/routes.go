package api

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App, logger *slog.Logger, handlers *Handlers) {
	SetupMiddleware(app, logger)

	// Health
	app.Get("/health", handlers.Health)
	app.Get("/ready", handlers.Ready)

	// Docs
	app.Get("/docs", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"title": "SMS Gateway API",
			"endpoints": fiber.Map{
				"health": "GET /health",
				"send":   "POST /v1/messages",
				"get":    "GET /v1/messages/:id",
				"list":   "GET /v1/messages?client_id=uuid",
				"client": "GET /v1/me?client_id=uuid",
			},
		})
	})

	// API v1
	v1 := app.Group("/v1")
	v1.Get("/me", handlers.GetClientInfo)

	msgs := v1.Group("/messages")
	msgs.Post("/", handlers.SendMessage)
	msgs.Get("/", handlers.ListMessages)
	msgs.Get("/:id", handlers.GetMessage)

	// Provider webhooks
	v1.Post("/providers/mock/dlr", handlers.HandleDLR)
}
