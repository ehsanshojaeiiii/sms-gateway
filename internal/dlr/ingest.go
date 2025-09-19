package dlr

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sms-gateway/internal/auth"
	"sms-gateway/internal/billing"
	"sms-gateway/internal/messages"
	"sms-gateway/internal/observability"
	"time"

	"go.uber.org/zap"
)

type IngestRequest struct {
	ProviderMessageID string          `json:"provider_message_id"`
	Status            messages.Status `json:"status"`
	Reason            string          `json:"reason,omitempty"`
	Timestamp         time.Time       `json:"timestamp"`
}

type Service struct {
	logger         *zap.Logger
	metrics        *observability.Metrics
	messageStore   *messages.Store
	authService    *auth.AuthService
	billingService *billing.BillingService
}

func NewService(
	logger *zap.Logger,
	metrics *observability.Metrics,
	messageStore *messages.Store,
	authService *auth.AuthService,
	billingService *billing.BillingService,
) *Service {
	return &Service{
		logger:         logger,
		metrics:        metrics,
		messageStore:   messageStore,
		authService:    authService,
		billingService: billingService,
	}
}

func (s *Service) ProcessDLR(ctx context.Context, req *IngestRequest) error {
	// Find message by provider ID
	msg, err := s.messageStore.GetMessageByProviderID(ctx, req.ProviderMessageID)
	if err != nil {
		s.logger.Warn("message not found for DLR",
			zap.String("provider_message_id", req.ProviderMessageID),
			zap.Error(err))
		return fmt.Errorf("message not found")
	}

	s.logger.Info("processing DLR",
		zap.String("message_id", msg.ID.String()),
		zap.String("provider_message_id", req.ProviderMessageID),
		zap.String("old_status", string(msg.Status)),
		zap.String("new_status", string(req.Status)))

	// Update message status
	var lastError *string
	if req.Reason != "" {
		lastError = &req.Reason
	}

	err = s.messageStore.UpdateMessageStatus(ctx, msg.ID, req.Status, nil, lastError)
	if err != nil {
		s.logger.Error("failed to update message status", zap.Error(err))
		return fmt.Errorf("failed to update message status: %w", err)
	}

	// Handle billing based on status
	switch req.Status {
	case messages.StatusDelivered:
		// Capture held credits
		if err := s.billingService.CaptureCredits(ctx, msg.ID); err != nil {
			s.logger.Error("failed to capture credits", zap.Error(err))
		}

	case messages.StatusFailedPerm:
		// Release held credits for permanent failures
		if err := s.billingService.ReleaseCredits(ctx, msg.ID); err != nil {
			s.logger.Error("failed to release credits", zap.Error(err))
		}
	}

	// Update metrics
	if s.metrics != nil {
		s.metrics.MessagesProcessedTotal.WithLabelValues(string(req.Status), "mock").Inc()
	}

	// Send callback to client if configured
	go s.sendClientCallback(context.Background(), msg, req.Status, req.Reason)

	return nil
}

func (s *Service) sendClientCallback(ctx context.Context, msg *messages.Message, status messages.Status, reason string) {
	// Get client info
	client, err := s.authService.GetClientByID(ctx, msg.ClientID)
	if err != nil {
		s.logger.Error("failed to get client for callback", zap.Error(err))
		return
	}

	// Only send callback if client has configured a callback URL
	if client.DLRCallbackURL == nil || *client.DLRCallbackURL == "" {
		return
	}

	callbackPayload := map[string]interface{}{
		"message_id": msg.ID,
		"status":     status,
		"reason":     reason,
		"timestamp":  time.Now().Unix(),
	}

	if msg.ClientReference != nil {
		callbackPayload["client_reference"] = *msg.ClientReference
	}

	// Generate HMAC signature if secret is configured
	var signature string
	if client.CallbackHMACSecret != nil && *client.CallbackHMACSecret != "" {
		signature = s.generateHMACSignature(callbackPayload, *client.CallbackHMACSecret)
	}

	s.logger.Info("sending DLR callback to client",
		zap.String("client_id", client.ID.String()),
		zap.String("message_id", msg.ID.String()),
		zap.String("callback_url", *client.DLRCallbackURL),
		zap.String("status", string(status)))

	// TODO: Implement actual HTTP callback with retries
	// This would typically use an HTTP client with exponential backoff
	// For now, just log the callback details
	s.logger.Debug("callback payload prepared",
		zap.Any("payload", callbackPayload),
		zap.String("signature", signature))
}

func (s *Service) generateHMACSignature(payload map[string]interface{}, secret string) string {
	// In a real implementation, you'd serialize the payload properly
	// This is a simplified version for demonstration
	data := fmt.Sprintf("%v", payload)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Service) ValidateHMACSignature(payload []byte, signature, secret string) bool {
	expectedMAC := hmac.New(sha256.New, []byte(secret))
	expectedMAC.Write(payload)
	expectedSignature := hex.EncodeToString(expectedMAC.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}
