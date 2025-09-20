package messages

import (
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Status string

const (
	StatusQueued     Status = "QUEUED"
	StatusSending    Status = "SENDING"
	StatusSent       Status = "SENT"
	StatusDelivered  Status = "DELIVERED"
	StatusFailedTemp Status = "FAILED_TEMP"
	StatusFailedPerm Status = "FAILED_PERM"
	StatusCancelled  Status = "CANCELLED"
)

type Message struct {
	ID                uuid.UUID `json:"id" db:"id"`
	ClientID          uuid.UUID `json:"client_id" db:"client_id"`
	ToMSISDN          string    `json:"to" db:"to_msisdn"`
	FromSender        string    `json:"from" db:"from_sender"`
	Text              string    `json:"text" db:"text"`
	Parts             int       `json:"parts" db:"parts"`
	Status            Status    `json:"status" db:"status"`
	ClientReference   *string   `json:"client_reference,omitempty" db:"client_reference"`
	Provider          *string   `json:"provider,omitempty" db:"provider"`
	ProviderMessageID *string   `json:"provider_message_id,omitempty" db:"provider_message_id"`
	Attempts          int       `json:"attempts" db:"attempts"`
	LastError         *string   `json:"last_error,omitempty" db:"last_error"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

type CreateMessageRequest struct {
	To              string  `json:"to" validate:"required,e164"`
	From            string  `json:"from" validate:"required"`
	Text            string  `json:"text" validate:"required,max=1600"`
	DLRCallbackURL  *string `json:"dlr_callback_url,omitempty" validate:"omitempty,url"`
	ClientReference *string `json:"client_reference,omitempty" validate:"omitempty,max=64"`
	OTP             bool    `json:"otp,omitempty"`
	Express         bool    `json:"express,omitempty"`
}

type CreateMessageResponse struct {
	MessageID uuid.UUID `json:"message_id"`
	Status    Status    `json:"status"`
	OTPCode   *string   `json:"otp_code,omitempty"`
}

type GetMessageResponse struct {
	*Message
	CostCents int64 `json:"cost_cents"`
}

// CalculateParts calculates the number of SMS parts based on text content
func CalculateParts(text string) int {
	length := utf8.RuneCountInString(text)

	// Basic GSM-7 vs UCS-2 detection (simplified)
	isGSM7 := isGSM7Compatible(text)

	if isGSM7 {
		if length <= 160 {
			return 1
		}
		// For concatenated SMS, each part has 153 characters (7 bytes for headers)
		return (length-1)/153 + 1
	} else {
		// UCS-2 encoding
		if length <= 70 {
			return 1
		}
		// For concatenated SMS, each part has 67 characters (6 bytes for headers)
		return (length-1)/67 + 1
	}
}

// isGSM7Compatible checks if text can be encoded in GSM 7-bit alphabet
func isGSM7Compatible(text string) bool {
	// Simplified GSM 7-bit alphabet check
	// In production, this would be more comprehensive
	for _, r := range text {
		if r > 127 && !isGSMExtendedChar(r) {
			return false
		}
	}
	return true
}

func isGSMExtendedChar(r rune) bool {
	// GSM extended characters that require 2 bytes
	extendedChars := []rune{'^', '{', '}', '\\', '[', '~', ']', '|', '€'}
	for _, char := range extendedChars {
		if r == char {
			return true
		}
	}
	return false
}
