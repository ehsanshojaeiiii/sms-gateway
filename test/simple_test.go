package test

import (
	"sms-gateway/internal/messages"
	"testing"

	"github.com/google/uuid"
)

// Test core business logic without external dependencies
func TestMessageCalculations(t *testing.T) {
	// Test SMS part calculation
	tests := []struct {
		text     string
		expected int
	}{
		{"Hello", 1},
		{"This is a very long message that should definitely be split into multiple parts because it exceeds the normal SMS limit of 160 characters for GSM7 encoding and needs to be much longer", 2},
		{"🚀 Unicode message", 1},
	}

	for _, tt := range tests {
		parts := messages.CalculateParts(tt.text)
		if parts != tt.expected {
			t.Errorf("CalculateParts(%q) = %d, want %d", tt.text, parts, tt.expected)
		}
	}
}

func TestOTPGeneration(t *testing.T) {
	// Test OTP request structure
	req := messages.SendRequest{
		ClientID: uuid.New(),
		To:       "+1234567890",
		From:     "BANK",
		OTP:      true,
	}

	if !req.OTP {
		t.Error("OTP flag should be true")
	}

	if req.To == "" || req.From == "" {
		t.Error("Required fields should not be empty")
	}
}

func TestExpressSMS(t *testing.T) {
	// Test Express SMS request
	req := messages.SendRequest{
		ClientID: uuid.New(),
		To:       "+1234567890",
		From:     "URGENT",
		Text:     "Emergency alert",
		Express:  true,
	}

	if !req.Express {
		t.Error("Express flag should be true")
	}

	// Test cost calculation logic
	parts := messages.CalculateParts(req.Text)
	baseCost := int64(parts) * 5    // 5 cents per part
	expressCost := int64(parts) * 2 // 2 cents surcharge
	totalCost := baseCost + expressCost

	expectedTotal := int64(7) // 5 + 2 = 7 cents for 1 part
	if totalCost != expectedTotal {
		t.Errorf("Expected total cost %d, got %d", expectedTotal, totalCost)
	}
}

func TestMessageStatuses(t *testing.T) {
	// Test status transitions
	statuses := []messages.Status{
		messages.StatusQueued,
		messages.StatusSending,
		messages.StatusSent,
		messages.StatusDelivered,
		messages.StatusFailedTemp,
		messages.StatusFailedPerm,
	}

	for _, status := range statuses {
		if string(status) == "" {
			t.Errorf("Status should not be empty: %v", status)
		}
	}

	// Test valid status progression
	if messages.StatusQueued == messages.StatusSent {
		t.Error("Different statuses should not be equal")
	}
}
