package auth

import (
	"context"
	"database/sql"
	"fmt"
	"sms-gateway/internal/persistence"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Client struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	APIKeyHash         string    `json:"-"`
	DLRCallbackURL     *string   `json:"dlr_callback_url,omitempty"`
	CallbackHMACSecret *string   `json:"-"`
	CreditCents        int64     `json:"credit_cents"`
}

type AuthService struct {
	db     *persistence.PostgresDB
	logger *zap.Logger
}

func NewAuthService(db *persistence.PostgresDB, logger *zap.Logger) *AuthService {
	return &AuthService{
		db:     db,
		logger: logger,
	}
}

func (a *AuthService) CreateClient(ctx context.Context, name, apiKey string, dlrCallbackURL *string, callbackSecret *string) (*Client, error) {
	hashedKey, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash API key: %w", err)
	}

	client := &Client{
		ID:                 uuid.New(),
		Name:               name,
		APIKeyHash:         string(hashedKey),
		DLRCallbackURL:     dlrCallbackURL,
		CallbackHMACSecret: callbackSecret,
		CreditCents:        0,
	}

	query := `
		INSERT INTO clients (id, name, api_key_hash, dlr_callback_url, callback_hmac_secret, credit_cents)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err = a.db.ExecContext(ctx, query,
		client.ID, client.Name, client.APIKeyHash,
		client.DLRCallbackURL, client.CallbackHMACSecret, client.CreditCents)
	if err != nil {
		return nil, fmt.Errorf("failed to insert client: %w", err)
	}

	return client, nil
}

func (a *AuthService) AuthenticateAPIKey(ctx context.Context, apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("invalid API key")
	}

	// Fetch clients and validate key (supports bcrypt or plaintext for demo seeds)
	rows, err := a.db.QueryContext(ctx, `
        SELECT id, name, api_key_hash, dlr_callback_url, callback_hmac_secret, credit_cents
        FROM clients`)
	if err != nil {
		return nil, fmt.Errorf("failed to query clients: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var c Client
		if err := rows.Scan(&c.ID, &c.Name, &c.APIKeyHash, &c.DLRCallbackURL, &c.CallbackHMACSecret, &c.CreditCents); err != nil {
			return nil, fmt.Errorf("failed to scan client: %w", err)
		}
		// Accept either bcrypt match or plaintext equality (for simple seeds)
		if bcrypt.CompareHashAndPassword([]byte(c.APIKeyHash), []byte(apiKey)) == nil || c.APIKeyHash == apiKey {
			return &c, nil
		}
	}

	return nil, fmt.Errorf("invalid API key")
}

func (a *AuthService) GetClientByID(ctx context.Context, clientID uuid.UUID) (*Client, error) {
	query := `
		SELECT id, name, api_key_hash, dlr_callback_url, callback_hmac_secret, credit_cents
		FROM clients
		WHERE id = $1`

	var client Client
	err := a.db.QueryRowContext(ctx, query, clientID).Scan(
		&client.ID, &client.Name, &client.APIKeyHash,
		&client.DLRCallbackURL, &client.CallbackHMACSecret, &client.CreditCents)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("client not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	return &client, nil
}

// Middleware for Fiber
func (a *AuthService) RequireAPIKey() fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiKey := c.Get("X-API-Key")
		client, err := a.AuthenticateAPIKey(c.Context(), apiKey)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid API key"})
		}
		c.Locals("client", client)
		return c.Next()
	}
}

// Helper to get client from Fiber context
func GetClientFromContext(c *fiber.Ctx) (*Client, error) {
	client, ok := c.Locals("client").(*Client)
	if !ok {
		return nil, fmt.Errorf("client not found in context")
	}
	return client, nil
}
