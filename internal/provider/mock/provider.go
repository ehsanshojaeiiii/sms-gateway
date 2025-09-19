package mock

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Provider struct {
	logger       *zap.Logger
	successRate  float64
	tempFailRate float64
	permFailRate float64
	latencyMs    int
	rng          *rand.Rand
}

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

func NewProvider(logger *zap.Logger, successRate, tempFailRate, permFailRate float64, latencyMs int) *Provider {
	// For demo: High success rate to show system working
	return &Provider{
		logger:       logger,
		successRate:  0.95, // 95% success for demo
		tempFailRate: 0.03, // 3% temp failures
		permFailRate: 0.02, // 2% permanent failures
		latencyMs:    latencyMs,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (p *Provider) SendSMS(ctx context.Context, msg *Message) *SendResult {
	// Simulate API latency
	time.Sleep(time.Duration(p.latencyMs) * time.Millisecond)

	// Generate deterministic provider message ID based on our message ID
	providerID := p.generateProviderID(msg.ID)

	// Determine result based on message ID hash for deterministic behavior
	outcome := p.determineOutcome(msg.ID)

	result := &SendResult{
		ProviderMessageID: providerID,
	}

	switch outcome {
	case "success":
		result.Status = StatusSent
		p.logger.Debug("mock provider: message sent successfully",
			zap.String("message_id", msg.ID.String()),
			zap.String("provider_id", providerID))

	case "temp_fail":
		result.Status = StatusFailedTemp
		result.Error = fmt.Errorf("temporary failure: network timeout")
		p.logger.Debug("mock provider: temporary failure",
			zap.String("message_id", msg.ID.String()),
			zap.Error(result.Error))

	case "perm_fail":
		result.Status = StatusFailedPerm
		result.Error = fmt.Errorf("permanent failure: invalid number")
		p.logger.Debug("mock provider: permanent failure",
			zap.String("message_id", msg.ID.String()),
			zap.Error(result.Error))
	}

	return result
}

func (p *Provider) GetName() string {
	return "mock"
}

// generateProviderID creates a deterministic provider ID based on message ID
func (p *Provider) generateProviderID(messageID uuid.UUID) string {
	hash := md5.Sum(messageID[:])
	return "mock_" + hex.EncodeToString(hash[:])[:12]
}

// determineOutcome returns outcome based on message ID hash for deterministic results
func (p *Provider) determineOutcome(messageID uuid.UUID) string {
	// Use message ID bytes to create deterministic outcome
	hash := md5.Sum(messageID[:])
	value := float64(hash[0]) / 255.0 // Convert first byte to 0.0-1.0 range

	if value < p.successRate {
		return "success"
	} else if value < p.successRate+p.tempFailRate {
		return "temp_fail"
	} else {
		return "perm_fail"
	}
}

// SimulateDLR simulates delivery report callback (would normally come from provider webhook)
func (p *Provider) SimulateDLR(ctx context.Context, providerMessageID string, status Status) {
	// This would typically be called by provider webhooks
	// For testing, we can manually trigger DLRs
	p.logger.Debug("simulating DLR",
		zap.String("provider_message_id", providerMessageID),
		zap.String("status", string(status)))
}
