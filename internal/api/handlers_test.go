package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"os"
	"sms-gateway/internal/messages"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func TestHealthEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handlers := &Handlers{logger: logger}

	app := fiber.New()
	app.Get("/health", handlers.Health)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestSendMessageValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handlers := &Handlers{logger: logger}

	app := fiber.New()
	app.Post("/messages", handlers.SendMessage)

	// Test missing fields
	reqBody := messages.SendRequest{
		ClientID: uuid.New(),
		// Missing To, From, Text
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("Expected status 400 for missing fields, got %d", resp.StatusCode)
	}
}
