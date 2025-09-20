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
	ID                uuid.UUID `json:"id"`
	ClientID          uuid.UUID `json:"client_id"`
	To                string    `json:"to"`
	From              string    `json:"from"`
	Text              string    `json:"text"`
	Parts             int       `json:"parts"`
	Status            Status    `json:"status"`
	Reference         *string   `json:"reference,omitempty"`
	Provider          *string   `json:"provider,omitempty"`
	ProviderMessageID *string   `json:"provider_message_id,omitempty"`
	Attempts          int       `json:"attempts"`
	LastError         *string   `json:"last_error,omitempty"`
	Express           bool      `json:"express"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type SendRequest struct {
	ClientID  uuid.UUID `json:"client_id" validate:"required"`
	To        string    `json:"to" validate:"required"`
	From      string    `json:"from" validate:"required"`
	Text      string    `json:"text,omitempty"`
	Reference *string   `json:"reference,omitempty"`
	OTP       bool      `json:"otp,omitempty"`
	Express   bool      `json:"express,omitempty"`
}

type SendResponse struct {
	MessageID uuid.UUID `json:"message_id"`
	Status    Status    `json:"status"`
	OTPCode   *string   `json:"otp_code,omitempty"`
}

type GetResponse struct {
	*Message
	Cost int64 `json:"cost"`
}

func CalculateParts(text string) int {
	length := utf8.RuneCountInString(text)

	if isGSM7(text) {
		if length <= 160 {
			return 1
		}
		return (length-1)/153 + 1
	}

	if length <= 70 {
		return 1
	}
	return (length-1)/67 + 1
}

func isGSM7(text string) bool {
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
