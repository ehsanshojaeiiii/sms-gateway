package api

import (
	"context"
	"fmt"
	"log/slog"
	"sms-gateway/internal/billing"
	"sms-gateway/internal/delivery"
	"sms-gateway/internal/messages"
	"sms-gateway/internal/messaging/nats"
	"sms-gateway/internal/otp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handlers struct {
	logger       *slog.Logger
	store        *messages.Store
	billing      *billing.Service
	queue        *nats.Queue
	delivery     *delivery.Service
	otpService   *otp.OTPService
	pricePerPart int64
	expressCost  int64
}

func NewHandlers(logger *slog.Logger, store *messages.Store, billing *billing.Service, queue *nats.Queue, delivery *delivery.Service, otpService *otp.OTPService, pricePerPart, expressCost int64) *Handlers {
	return &Handlers{
		logger:       logger,
		store:        store,
		billing:      billing,
		queue:        queue,
		delivery:     delivery,
		otpService:   otpService,
		pricePerPart: pricePerPart,
		expressCost:  expressCost,
	}
}

// SendMessage handles POST /v1/messages
//
//	@Summary		Send SMS
//	@Description	Send SMS message (regular, OTP, or Express)
//	@Tags			Messages
//	@Accept			json
//	@Produce		json
//	@Param			request	body		messages.SendRequest	true	"SMS request"
//	@Success		200		{object}	messages.SendResponse	"OTP delivered immediately"
//	@Success		202		{object}	messages.SendResponse	"Message queued"
//	@Failure		400		{object}	map[string]string		"Bad request"
//	@Failure		402		{object}	map[string]interface{}	"Insufficient credits"
//	@Failure		503		{object}	map[string]string		"OTP delivery failed"
//	@Router			/v1/messages [post]
func (h *Handlers) SendMessage(c *fiber.Ctx) error {
	var req messages.SendRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	// Validate required fields
	if req.ClientID.String() == "00000000-0000-0000-0000-000000000000" {
		return c.Status(400).JSON(fiber.Map{"error": "client_id is required"})
	}
	if req.To == "" || req.From == "" || (!req.OTP && req.Text == "") {
		return c.Status(400).JSON(fiber.Map{"error": "missing required fields"})
	}

	// Handle OTP with delivery guarantee (as per PDF requirement)
	if req.OTP {
		return h.handleOTPMessage(c, &req)
	}

	// Calculate cost
	parts := messages.CalculateParts(req.Text)
	cost := int64(parts) * h.pricePerPart
	if req.Express {
		cost += int64(parts) * h.expressCost
	}

	// Create message
	msg := &messages.Message{
		ID:        uuid.New(),
		ClientID:  req.ClientID,
		To:        req.To,
		From:      req.From,
		Text:      req.Text,
		Parts:     parts,
		Status:    messages.StatusQueued,
		Reference: req.Reference,
		Express:   req.Express,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.store.Create(c.Context(), msg); err != nil {
		h.logger.Error("failed to create message", "error", err)
		// Check for foreign key constraint violation (invalid client_id)
		if strings.Contains(err.Error(), "foreign key constraint") || strings.Contains(err.Error(), "client_id_fkey") {
			return c.Status(400).JSON(fiber.Map{"error": "invalid client_id"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "internal error"})
	}

	// Actually hold credits for the real message
	if _, err := h.billing.HoldCredits(c.Context(), req.ClientID, msg.ID, cost); err != nil {
		h.store.Delete(c.Context(), msg.ID)
		return c.Status(402).JSON(fiber.Map{"error": "insufficient credits", "required": cost})
	}

	// Publish message to NATS for worker processing
	if err := h.queue.PublishSendJob(c.Context(), msg.ID, 1); err != nil {
		h.logger.Error("failed to publish message to NATS", "error", err, "message_id", msg.ID)
		// If NATS fails, release credits and mark message as failed
		h.billing.ReleaseCredits(c.Context(), msg.ID)
		h.store.UpdateStatus(c.Context(), msg.ID, messages.StatusFailedPerm, nil, &[]string{"NATS publish failed"}[0])
		return c.Status(500).JSON(fiber.Map{"error": "failed to queue message"})
	}

	h.logger.Info("message published to NATS for processing", "id", msg.ID, "express", msg.Express)

	h.logger.Info("message created", "id", msg.ID, "client", req.ClientID, "cost", cost)

	return c.Status(202).JSON(&messages.SendResponse{
		MessageID: msg.ID,
		Status:    msg.Status,
	})
}

// handleOTPMessage handles OTP messages with delivery guarantee (PDF requirement)
func (h *Handlers) handleOTPMessage(c *fiber.Ctx, req *messages.SendRequest) error {
	// Generate 6-digit OTP code
	otpCode := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	if req.Text == "" {
		req.Text = fmt.Sprintf("Your verification code is %s", otpCode)
	}

	// Calculate cost
	parts := messages.CalculateParts(req.Text)
	cost := int64(parts) * h.pricePerPart

	// Create message first
	msg := &messages.Message{
		ID:        uuid.New(),
		ClientID:  req.ClientID,
		To:        req.To,
		From:      req.From,
		Text:      req.Text,
		Parts:     parts,
		Status:    messages.StatusQueued,
		Reference: req.Reference,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.store.Create(c.Context(), msg); err != nil {
		h.logger.Error("failed to create OTP message", "error", err)
		return c.Status(500).JSON(fiber.Map{"error": "internal error"})
	}

	// Hold credits
	if _, err := h.billing.HoldCredits(c.Context(), req.ClientID, msg.ID, cost); err != nil {
		h.store.Delete(c.Context(), msg.ID)
		return c.Status(402).JSON(fiber.Map{"error": "insufficient credits", "required": cost})
	}

	// Try immediate OTP delivery (PDF requirement: guaranteed delivery or error)
	result, err := h.otpService.SendOTPImmediate(c.Context(), req.To, req.From, req.Text)
	if err != nil {
		// Release held credits on failure
		h.billing.ReleaseCredits(c.Context(), msg.ID)
		h.store.Delete(c.Context(), msg.ID)

		h.logger.Warn("OTP delivery failed immediately", "error", err, "to", req.To)

		// Return immediate error as required by PDF
		return c.Status(503).JSON(fiber.Map{
			"error":  "OTP delivery failed - operator cannot deliver immediately",
			"reason": err.Error(),
		})
	}

	// Success - update message with provider info
	h.store.UpdateStatus(c.Context(), msg.ID, messages.StatusSent, &result.ProviderMessageID, nil)

	// Capture credits on successful delivery
	h.billing.CaptureCredits(c.Context(), msg.ID)

	h.logger.Info("OTP delivered immediately", "id", msg.ID, "to", req.To, "provider_id", result.ProviderMessageID)

	// Return success with OTP code (200 OK for immediate delivery)
	return c.Status(200).JSON(&messages.SendResponse{
		MessageID: msg.ID,
		Status:    messages.StatusSent,
		OTPCode:   &otpCode,
	})
}

// GetMessage handles GET /v1/messages/:id
func (h *Handlers) GetMessage(c *fiber.Ctx) error {
	msgID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid message ID"})
	}

	msg, err := h.store.GetByID(c.Context(), msgID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "message not found"})
	}

	cost := int64(msg.Parts) * h.pricePerPart
	if msg.Express {
		cost += int64(msg.Parts) * h.expressCost
	}

	return c.JSON(&messages.GetResponse{Message: msg, Cost: cost})
}

// ListMessages handles GET /v1/messages
func (h *Handlers) ListMessages(c *fiber.Ctx) error {
	clientID, err := uuid.Parse(c.Query("client_id", ""))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "client_id required"})
	}

	msgs, err := h.store.ListByClient(c.Context(), clientID, 50, 0)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "internal error"})
	}
	return c.JSON(msgs)
}

// GetClientInfo handles GET /v1/me
//
//	@Summary		Get client info
//	@Description	Get client credit balance and information
//	@Tags			Client
//	@Produce		json
//	@Param			client_id	query		string					true	"Client ID"
//	@Success		200			{object}	map[string]interface{}	"Client info"
//	@Failure		400			{object}	map[string]string		"Bad request"
//	@Failure		500			{object}	map[string]string		"Internal error"
//	@Router			/v1/me [get]
func (h *Handlers) GetClientInfo(c *fiber.Ctx) error {
	clientIDStr := c.Query("client_id")
	if clientIDStr == "" {
		return c.Status(400).JSON(fiber.Map{"error": "client_id query parameter required"})
	}
	clientID, err := uuid.Parse(clientIDStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid client_id format"})
	}

	credits, err := h.billing.GetCredits(c.Context(), clientID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "internal error"})
	}

	return c.JSON(fiber.Map{"id": clientID, "credits": credits})
}

// Health endpoints
//
//	@Summary		Health check
//	@Description	Basic health check endpoint
//	@Tags			System
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"Health status"
//	@Router			/health [get]
func (h *Handlers) Health(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok", "time": time.Now().Unix()})
}

func (h *Handlers) Ready(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	if err := h.store.Health(ctx); err != nil {
		return c.Status(503).JSON(fiber.Map{"status": "not ready"})
	}
	return c.JSON(fiber.Map{"status": "ready"})
}

// HandleDLR handles delivery receipts
func (h *Handlers) HandleDLR(c *fiber.Ctx) error {
	var req delivery.Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.ProviderMessageID == "" || req.Status == "" {
		return c.Status(400).JSON(fiber.Map{"error": "missing required fields"})
	}

	if err := h.delivery.Process(c.Context(), &req); err != nil {
		h.logger.Error("failed to process DLR", "error", err)
		return c.Status(500).JSON(fiber.Map{"error": "failed to process DLR"})
	}

	return c.SendStatus(204)
}
