package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestDoubleSpendingPrevention tests the system's ability to prevent double spending
// under extreme concurrent load conditions
func TestDoubleSpendingPrevention(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping double spending test in short mode")
	}

	baseURL := "http://localhost:8080"
	clientID := "550e8400-e29b-41d4-a716-446655440000"

	// Test cases designed to detect double spending vulnerabilities
	testCases := []struct {
		name           string
		concurrentReqs int
		description    string
	}{
		{
			name:           "Moderate Concurrency",
			concurrentReqs: 20,
			description:    "20 concurrent requests to test basic race condition handling",
		},
		{
			name:           "High Concurrency", 
			concurrentReqs: 50,
			description:    "50 concurrent requests to stress test credit system",
		},
		{
			name:           "Extreme Concurrency",
			concurrentReqs: 100,
			description:    "100 concurrent requests to test maximum load scenario",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Get initial client state
			initialCredits := getClientCredits(t, baseURL, clientID)
			maxPossibleMessages := initialCredits / 5 // 5 cents per message

			t.Logf("=== %s ===", tc.name)
			t.Logf("Description: %s", tc.description)
			t.Logf("Initial credits: %d cents", initialCredits)
			t.Logf("Max possible messages: %d", maxPossibleMessages)
			t.Logf("Concurrent requests: %d", tc.concurrentReqs)

			// Launch concurrent requests
			var successCount, failCount int64
			var wg sync.WaitGroup
			start := time.Now()

			for i := 0; i < tc.concurrentReqs; i++ {
				wg.Add(1)
				go func(reqNum int) {
					defer wg.Done()

					payload := fmt.Sprintf(`{
						"client_id": "%s",
						"to": "+1%09d",
						"from": "DOUBLE_SPEND_TEST",
						"text": "Double spending test #%d"
					}`, clientID, reqNum, reqNum)

					resp, err := http.Post(baseURL+"/v1/messages", "application/json", strings.NewReader(payload))
					if err != nil {
						atomic.AddInt64(&failCount, 1)
						return
					}
					defer resp.Body.Close()

					switch resp.StatusCode {
					case 202:
						atomic.AddInt64(&successCount, 1)
					case 402:
						atomic.AddInt64(&failCount, 1) // Expected for insufficient credits
					default:
						t.Errorf("Unexpected status: %d", resp.StatusCode)
						atomic.AddInt64(&failCount, 1)
					}
				}(i)
			}

			wg.Wait()
			duration := time.Since(start)

			// Get final credits
			finalCredits := getClientCredits(t, baseURL, clientID)
			creditsUsed := initialCredits - finalCredits
			expectedCreditsUsed := successCount * 5

			// Analysis
			t.Logf("Results:")
			t.Logf("  Duration: %v", duration)
			t.Logf("  Successful messages: %d", successCount)
			t.Logf("  Failed/rejected: %d", failCount)
			t.Logf("  Credits used: %d", creditsUsed)
			t.Logf("  Expected credits used: %d", expectedCreditsUsed)

			// Critical validations
			if creditsUsed != expectedCreditsUsed {
				t.Errorf("DOUBLE SPENDING DETECTED! Used %d credits but expected %d", creditsUsed, expectedCreditsUsed)
			}

			if successCount > maxPossibleMessages {
				t.Errorf("OVERSPENDING DETECTED! %d messages succeeded but only %d possible", successCount, maxPossibleMessages)
			}

			if creditsUsed > initialCredits {
				t.Errorf("CREDIT OVERDRAFT! Used %d credits but only had %d", creditsUsed, initialCredits)
			}

			// Performance validation
			requestsPerSecond := float64(tc.concurrentReqs) / duration.Seconds()
			t.Logf("  Performance: %.1f requests/second", requestsPerSecond)

			if requestsPerSecond < 50 {
				t.Errorf("Performance too slow under concurrent load: %.1f req/s", requestsPerSecond)
			}

			t.Logf("✅ Double spending prevention validated!")
		})
	}
}

// getClientCredits retrieves current client credits
func getClientCredits(t *testing.T, baseURL, clientID string) int64 {
	resp, err := http.Get(fmt.Sprintf("%s/v1/me?client_id=%s", baseURL, clientID))
	if err != nil {
		t.Fatalf("Failed to get client credits: %v", err)
	}
	defer resp.Body.Close()

	var data struct {
		Credits int64 `json:"credits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("Failed to decode credits response: %v", err)
	}

	return data.Credits
}
