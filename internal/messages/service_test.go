package messages_test

import (
	"sms-gateway/internal/messages"
	"testing"
)

func TestCalculateParts(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "short SMS (GSM 7-bit)",
			text:     "Hello world",
			expected: 1,
		},
		{
			name:     "exactly 160 chars (GSM 7-bit)",
			text:     string(make([]rune, 160)),
			expected: 1,
		},
		{
			name:     "long SMS (GSM 7-bit) - 2 parts",
			text:     string(make([]rune, 200)),
			expected: 2,
		},
		{
			name:     "short SMS with unicode",
			text:     "Hello 世界",
			expected: 1,
		},
		{
			name:     "exactly 70 chars (UCS-2)",
			text:     "Hello 世界" + string(make([]rune, 62)),
			expected: 1,
		},
		{
			name:     "long SMS with unicode - 2 parts",
			text:     "Hello 世界" + string(make([]rune, 100)),
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := messages.CalculateParts(tt.text)
			if parts != tt.expected {
				t.Errorf("CalculateParts(%s) = %d, expected %d", tt.name, parts, tt.expected)
			}
		})
	}
}

func TestStatusConstants(t *testing.T) {
	// Test that all status constants are defined
	statuses := []messages.Status{
		messages.StatusQueued,
		messages.StatusSending,
		messages.StatusSent,
		messages.StatusDelivered,
		messages.StatusFailedTemp,
		messages.StatusFailedPerm,
		messages.StatusCancelled,
	}

	expected := []string{
		"QUEUED",
		"SENDING",
		"SENT",
		"DELIVERED",
		"FAILED_TEMP",
		"FAILED_PERM",
		"CANCELLED",
	}

	for i, status := range statuses {
		if string(status) != expected[i] {
			t.Errorf("Status %d: got %s, expected %s", i, status, expected[i])
		}
	}
}
