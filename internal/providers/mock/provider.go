package mock

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusSent       Status = "SENT"
	StatusFailedTemp Status = "FAILED_TEMP"
	StatusFailedPerm Status = "FAILED_PERM"
	StatusDelivered  Status = "DELIVERED"
)

type Message struct {
	ID         uuid.UUID
	ToMSISDN   string
	FromSender string
	Text       string
}

type SendResult struct {
	ProviderMessageID string
	Status            Status
	Error             error
}

type Provider struct {
	name         string
	successRate  float64
	tempFailRate float64
	permFailRate float64
	latencyMs    int
}

func NewProvider() *Provider {
	return &Provider{
		name:         "mock",
		successRate:  0.95,
		tempFailRate: 0.03,
		permFailRate: 0.02,
		latencyMs:    100,
	}
}

func (p *Provider) GetName() string {
	return p.name
}

func (p *Provider) SendSMS(ctx context.Context, msg *Message) *SendResult {
	// Simulate latency
	time.Sleep(time.Duration(p.latencyMs) * time.Millisecond)

	providerID := fmt.Sprintf("mock_%d", time.Now().UnixNano())

	// Simulate different outcomes
	r := rand.Float64()

	if r < p.successRate {
		return &SendResult{
			ProviderMessageID: providerID,
			Status:            StatusSent,
		}
	} else if r < p.successRate+p.tempFailRate {
		return &SendResult{
			ProviderMessageID: providerID,
			Status:            StatusFailedTemp,
			Error:             fmt.Errorf("temporary network error"),
		}
	} else {
		return &SendResult{
			ProviderMessageID: providerID,
			Status:            StatusFailedPerm,
			Error:             fmt.Errorf("invalid phone number"),
		}
	}
}

func (p *Provider) SimulateDLR(ctx context.Context, providerMessageID string, status Status) {
	// This would normally be called by the provider via webhook
	// For testing, we can simulate DLR callbacks
}
