package otp

import (
	"context"
	"fmt"
	"log/slog"
	"sms-gateway/internal/providers/mock"
	"time"
)

// OTPService handles OTP messages with delivery guarantee
type OTPService struct {
	logger   *slog.Logger
	provider *mock.Provider
	timeout  time.Duration
}

func NewOTPService(logger *slog.Logger, provider *mock.Provider) *OTPService {
	return &OTPService{
		logger:   logger,
		provider: provider,
		timeout:  5 * time.Second, // 5 second timeout for OTP delivery
	}
}

// SendOTPImmediate tries to send OTP immediately, returns error if can't deliver
func (s *OTPService) SendOTPImmediate(ctx context.Context, to, from, text string) (*OTPResult, error) {
	// Create timeout context for immediate delivery
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	msg := &mock.Message{
		ToMSISDN:   to,
		FromSender: from,
		Text:       text,
	}

	// Try to send immediately with timeout
	result := s.provider.SendSMS(ctx, msg)

	// Check if context timed out
	if ctx.Err() == context.DeadlineExceeded {
		s.logger.Warn("OTP delivery timeout", "to", to, "timeout", s.timeout)
		return nil, fmt.Errorf("OTP delivery timeout - operator not responding within %v", s.timeout)
	}

	// Check for immediate delivery failure
	if result.Error != nil {
		s.logger.Warn("OTP delivery failed", "to", to, "error", result.Error)
		return nil, fmt.Errorf("OTP delivery failed: %w", result.Error)
	}

	// Success - OTP was accepted by provider immediately
	s.logger.Info("OTP delivered immediately", "to", to, "provider_id", result.ProviderMessageID)

	return &OTPResult{
		ProviderMessageID: result.ProviderMessageID,
		Status:            "SENT_IMMEDIATELY",
	}, nil
}

type OTPResult struct {
	ProviderMessageID string `json:"provider_message_id"`
	Status            string `json:"status"`
}
