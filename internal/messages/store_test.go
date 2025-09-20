package messages

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCalculateParts(t *testing.T) {
	tests := []struct {
		text     string
		expected int
	}{
		{"Hello", 1},
		{"This is a longer message that should be split into multiple parts because it exceeds the normal SMS limit of 160 characters for GSM7 encoding and this makes it even longer to definitely exceed the limit", 2},
		{"🚀", 1}, // Unicode
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := CalculateParts(tt.text)
			if result != tt.expected {
				t.Errorf("CalculateParts(%q) = %d, want %d", tt.text, result, tt.expected)
			}
		})
	}
}

func TestMessage(t *testing.T) {
	msg := &Message{
		ID:        uuid.New(),
		ClientID:  uuid.New(),
		To:        "+1234567890",
		From:      "TEST",
		Text:      "Hello World",
		Parts:     1,
		Status:    StatusQueued,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if msg.Status != StatusQueued {
		t.Errorf("Expected status %s, got %s", StatusQueued, msg.Status)
	}

	if msg.Parts != 1 {
		t.Errorf("Expected 1 part, got %d", msg.Parts)
	}
}
