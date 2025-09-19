package api

import (
	"context"
	"sms-gateway/internal/auth"
	"sms-gateway/internal/billing"
	"sms-gateway/internal/dlr"
	"sms-gateway/internal/idempotency"
	"sms-gateway/internal/messages"
	"sms-gateway/internal/observability"
	"sms-gateway/internal/queue/nats"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Handlers struct {
	logger            *zap.Logger
	metrics           *observability.Metrics
	messageStore      *messages.Store
	idempotencyStore  *idempotency.Store
	billingService    *billing.BillingService
	queue             *nats.Queue
	dlrService        *dlr.Service
	pricePerPartCents int64
}

func NewHandlers(
	logger *zap.Logger,
	metrics *observability.Metrics,
	messageStore *messages.Store,
	idempotencyStore *idempotency.Store,
	billingService *billing.BillingService,
	queue *nats.Queue,
	dlrService *dlr.Service,
	pricePerPartCents int64,
) *Handlers {
	return &Handlers{
		logger:            logger,
		metrics:           metrics,
		messageStore:      messageStore,
		idempotencyStore:  idempotencyStore,
		billingService:    billingService,
		queue:             queue,
		dlrService:        dlrService,
		pricePerPartCents: pricePerPartCents,
	}
}

// SendMessage handles POST /v1/messages
func (h *Handlers) SendMessage(c *fiber.Ctx) error {
	client, err := auth.GetClientFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var req messages.CreateMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Validate request
	if req.To == "" || req.From == "" || req.Text == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing required fields: to, from, text",
		})
	}

	idempotencyKey := c.Get("Idempotency-Key")

	// Check idempotency
	if idempotencyKey != "" {
		existingMessageID, err := h.idempotencyStore.GetMessageID(c.Context(), client.ID, idempotencyKey)
		if err != nil {
			h.logger.Error("failed to check idempotency", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
		}

		if existingMessageID != uuid.Nil {
			// Return existing message
			msg, err := h.messageStore.GetMessage(c.Context(), existingMessageID)
			if err != nil {
				h.logger.Error("failed to get existing message", zap.Error(err))
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
			}

			return c.Status(fiber.StatusOK).JSON(&messages.CreateMessageResponse{
				MessageID: msg.ID,
				Status:    msg.Status,
			})
		}
	}

	// Calculate parts and cost
	parts := messages.CalculateParts(req.Text)
	costCents := int64(parts) * h.pricePerPartCents

	// Create message
	msg := &messages.Message{
		ID:              uuid.New(),
		ClientID:        client.ID,
		ToMSISDN:        req.To,
		FromSender:      req.From,
		Text:            req.Text,
		Parts:           parts,
		Status:          messages.StatusQueued,
		ClientReference: req.ClientReference,
		Attempts:        0,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Hold credits
	_, err = h.billingService.HoldCredits(c.Context(), client.ID, msg.ID, costCents)
	if err != nil {
		h.logger.Warn("failed to hold credits",
			zap.String("client_id", client.ID.String()),
			zap.Int64("cost_cents", costCents),
			zap.Error(err))
		return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
			"error":          "insufficient credits",
			"required_cents": costCents,
		})
	}

	// Store message
	if err := h.messageStore.CreateMessage(c.Context(), msg); err != nil {
		// Release held credits on failure
		h.billingService.ReleaseCredits(c.Context(), msg.ID)
		h.logger.Error("failed to create message", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
	}

	// Store idempotency key
	if idempotencyKey != "" {
		if err := h.idempotencyStore.StoreMessageID(c.Context(), client.ID, idempotencyKey, msg.ID); err != nil {
			h.logger.Error("failed to store idempotency key", zap.Error(err))
			// Continue anyway, message is already created
		}
	}

	// Enqueue for processing
	if err := h.queue.PublishSendJob(c.Context(), msg.ID, 1); err != nil {
		h.logger.Error("failed to enqueue send job", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
	}

	h.logger.Info("message created",
		zap.String("message_id", msg.ID.String()),
		zap.String("client_id", client.ID.String()),
		zap.String("to", req.To),
		zap.Int("parts", parts),
		zap.Int64("cost_cents", costCents))

	if h.metrics != nil {
		h.metrics.MessagesProcessedTotal.WithLabelValues("queued", "").Inc()
	}

	return c.Status(fiber.StatusAccepted).JSON(&messages.CreateMessageResponse{
		MessageID: msg.ID,
		Status:    msg.Status,
	})
}

// GetMessage handles GET /v1/messages/:id
func (h *Handlers) GetMessage(c *fiber.Ctx) error {
	client, err := auth.GetClientFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	messageIDStr := c.Params("id")
	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid message ID"})
	}

	msg, err := h.messageStore.GetMessage(c.Context(), messageID)
	if err != nil {
		h.logger.Debug("message not found", zap.String("message_id", messageIDStr))
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "message not found"})
	}

	// Check if client owns this message
	if msg.ClientID != client.ID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "message not found"})
	}

	costCents := int64(msg.Parts) * h.pricePerPartCents

	response := &messages.GetMessageResponse{
		Message:   msg,
		CostCents: costCents,
	}

	return c.JSON(response)
}

// GetClientInfo handles GET /v1/me
func (h *Handlers) GetClientInfo(c *fiber.Ctx) error {
	client, err := auth.GetClientFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	credits, err := h.billingService.GetClientCredits(c.Context(), client.ID)
	if err != nil {
		h.logger.Error("failed to get client credits", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
	}

	return c.JSON(fiber.Map{
		"id":           client.ID,
		"name":         client.Name,
		"credit_cents": credits,
	})
}

// HealthCheck handles GET /healthz
func (h *Handlers) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// ReadyCheck handles GET /readyz
func (h *Handlers) ReadyCheck(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	// Check database connectivity
	if err := h.messageStore.HealthCheck(ctx); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not ready",
			"error":  "database connectivity issue",
		})
	}

	return c.JSON(fiber.Map{
		"status":    "ready",
		"timestamp": time.Now().Unix(),
	})
}

// HandleDLR handles POST /v1/providers/mock/dlr
func (h *Handlers) HandleDLR(c *fiber.Ctx) error {
	var req dlr.IngestRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Validate required fields
	if req.ProviderMessageID == "" || req.Status == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing required fields: provider_message_id, status",
		})
	}

	if err := h.dlrService.ProcessDLR(c.Context(), &req); err != nil {
		h.logger.Error("failed to process DLR", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to process DLR",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
