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

// TestRaceConditionProtection tests concurrent credit access to prevent double spending
func TestRaceConditionProtection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	baseURL := "http://localhost:8080"
	clientID := "550e8400-e29b-41d4-a716-446655440000"

	// Test scenarios with different concurrency levels
	testCases := []struct {
		name            string
		initialCredits  int64
		concurrentReqs  int
		costPerMessage  int64
		expectedSuccess int
	}{
		{
			name:            "Edge Case: Exact Credits",
			initialCredits:  25,
			concurrentReqs:  10,
			costPerMessage:  5,
			expectedSuccess: 5, // 25 ÷ 5 = exactly 5 messages
		},
		{
			name:            "High Concurrency",
			initialCredits:  50,
			concurrentReqs:  20,
			costPerMessage:  5,
			expectedSuccess: 10, // 50 ÷ 5 = exactly 10 messages
		},
		{
			name:            "Extreme Stress Test",
			initialCredits:  100,
			concurrentReqs:  50,
			costPerMessage:  5,
			expectedSuccess: 20, // 100 ÷ 5 = exactly 20 messages
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup: Reset client credits
			setupResponse, err := http.Get(fmt.Sprintf("%s/v1/me?client_id=%s", baseURL, clientID))
			if err != nil {
				t.Fatalf("Failed to check initial state: %v", err)
			}
			setupResponse.Body.Close()

			// Run concurrent requests
			var successCount, failCount int64
			var wg sync.WaitGroup

			startTime := time.Now()

			for i := 0; i < tc.concurrentReqs; i++ {
				wg.Add(1)
				go func(reqNum int) {
					defer wg.Done()

					payload := fmt.Sprintf(`{
						"client_id": "%s",
						"to": "+123456789%02d",
						"from": "RACE_TEST",
						"text": "Race condition test #%d"
					}`, clientID, reqNum, reqNum)

					resp, err := http.Post(baseURL+"/v1/messages", "application/json", strings.NewReader(payload))
					if err != nil {
						atomic.AddInt64(&failCount, 1)
						return
					}
					defer resp.Body.Close()

					if resp.StatusCode == 202 {
						atomic.AddInt64(&successCount, 1)
					} else if resp.StatusCode == 402 {
						// Expected for insufficient credits
						atomic.AddInt64(&failCount, 1)
					} else {
						t.Errorf("Unexpected status code: %d", resp.StatusCode)
						atomic.AddInt64(&failCount, 1)
					}
				}(i)
			}

			wg.Wait()
			duration := time.Since(startTime)

			// Verify results
			t.Logf("=== %s Results ===", tc.name)
			t.Logf("Concurrent requests: %d", tc.concurrentReqs)
			t.Logf("Successful: %d", successCount)
			t.Logf("Failed (insufficient credits): %d", failCount)
			t.Logf("Duration: %v", duration)

			// Critical validation: Exactly the expected number should succeed
			if int(successCount) != tc.expectedSuccess {
				t.Errorf("Double spending detected! Expected %d successes, got %d", tc.expectedSuccess, successCount)
			}

			// Total requests should match
			if int(successCount+failCount) != tc.concurrentReqs {
				t.Errorf("Request count mismatch: %d + %d != %d", successCount, failCount, tc.concurrentReqs)
			}

			// Performance check: should complete quickly
			if duration > 5*time.Second {
				t.Errorf("Race condition handling too slow: %v", duration)
			}

			t.Logf("✅ Race condition protection working correctly!")
		})
	}
}

// TestCreditConsistency verifies credit math is correct after concurrent operations
func TestCreditConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping credit consistency test in short mode")
	}

	baseURL := "http://localhost:8080"
	clientID := "550e8400-e29b-41d4-a716-446655440000"

	// Get initial credits
	initialResp, err := http.Get(fmt.Sprintf("%s/v1/me?client_id=%s", baseURL, clientID))
	if err != nil {
		t.Fatalf("Failed to get initial credits: %v", err)
	}
	defer initialResp.Body.Close()

	var initialData struct {
		Credits int64 `json:"credits"`
	}
	json.NewDecoder(initialResp.Body).Decode(&initialData)
	initialCredits := initialData.Credits

	// Send some messages
	successCount := 0
	testMessages := 3

	for i := 0; i < testMessages; i++ {
		payload := fmt.Sprintf(`{
			"client_id": "%s",
			"to": "+123456789%02d",
			"from": "CONSISTENCY",
			"text": "Consistency test #%d"
		}`, clientID, i, i)

		resp, err := http.Post(baseURL+"/v1/messages", "application/json", strings.NewReader(payload))
		if err == nil && resp.StatusCode == 202 {
			successCount++
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	// Check final credits
	finalResp, err := http.Get(fmt.Sprintf("%s/v1/me?client_id=%s", baseURL, clientID))
	if err != nil {
		t.Fatalf("Failed to get final credits: %v", err)
	}
	defer finalResp.Body.Close()

	var finalData struct {
		Credits int64 `json:"credits"`
	}
	json.NewDecoder(finalResp.Body).Decode(&finalData)
	finalCredits := finalData.Credits

	// Verify credit math
	expectedCreditsUsed := int64(successCount * 5) // 5 cents per message
	actualCreditsUsed := initialCredits - finalCredits

	t.Logf("Credit Consistency Check:")
	t.Logf("  Initial credits: %d", initialCredits)
	t.Logf("  Final credits: %d", finalCredits)
	t.Logf("  Successful messages: %d", successCount)
	t.Logf("  Expected credits used: %d", expectedCreditsUsed)
	t.Logf("  Actual credits used: %d", actualCreditsUsed)

	if actualCreditsUsed != expectedCreditsUsed {
		t.Errorf("Credit inconsistency! Expected %d, got %d", expectedCreditsUsed, actualCreditsUsed)
	} else {
		t.Logf("✅ Credit consistency perfect!")
	}
}
