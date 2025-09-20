package delivery

import (
	"context"
	"fmt"
	"log/slog"
	"sms-gateway/internal/billing"
	"sms-gateway/internal/messages"
	"time"
)

type Request struct {
	ProviderMessageID string    `json:"provider_message_id"`
	Status            string    `json:"status"`
	Reason            string    `json:"reason,omitempty"`
	Timestamp         time.Time `json:"timestamp"`
}

type Service struct {
	logger  *slog.Logger
	store   *messages.Store
	billing *billing.Service
}

func NewService(logger *slog.Logger, store *messages.Store, billing *billing.Service) *Service {
	return &Service{
		logger:  logger,
		store:   store,
		billing: billing,
	}
}

func (s *Service) Process(ctx context.Context, req *Request) error {
	// Find message by provider ID
	msg, err := s.store.GetByProviderID(ctx, req.ProviderMessageID)
	if err != nil {
		return fmt.Errorf("message not found: %w", err)
	}

	// Update message status based on DLR
	var status messages.Status
	switch req.Status {
	case "DELIVERED":
		status = messages.StatusDelivered
		// Capture held credits
		if err := s.billing.CaptureCredits(ctx, msg.ID); err != nil {
			s.logger.Error("failed to capture credits", "error", err)
		}
	case "FAILED_PERM":
		status = messages.StatusFailedPerm
		// Release held credits
		if err := s.billing.ReleaseCredits(ctx, msg.ID); err != nil {
			s.logger.Error("failed to release credits", "error", err)
		}
	case "FAILED_TEMP":
		status = messages.StatusFailedTemp
	default:
		status = messages.StatusFailedPerm
		// Release held credits for unknown status
		if err := s.billing.ReleaseCredits(ctx, msg.ID); err != nil {
			s.logger.Error("failed to release credits", "error", err)
		}
	}

	// Update message status
	var errorMsg *string
	if req.Reason != "" {
		errorMsg = &req.Reason
	}

	err = s.store.UpdateStatus(ctx, msg.ID, status, nil, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to update message status: %w", err)
	}

	s.logger.Info("DLR processed",
		"provider_message_id", req.ProviderMessageID,
		"message_id", msg.ID,
		"status", status)

	return nil
}
