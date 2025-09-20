package test

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// PerformanceTestConfig defines performance test parameters
type PerformanceTestConfig struct {
	BaseURL             string
	ClientID            string
	ConcurrentClients   int
	MessagesPerClient   int
	TestDuration        time.Duration
	RampUpDuration      time.Duration
	MessageDistribution MessageDistribution
}

// MessageDistribution defines the distribution of message types
type MessageDistribution struct {
	RegularPercent int
	ExpressPercent int
	OTPPercent     int
}

// PerformanceResult holds comprehensive performance test results
type PerformanceResult struct {
	TotalMessages       int64         `json:"total_messages"`
	SuccessfulMessages  int64         `json:"successful_messages"`
	FailedMessages      int64         `json:"failed_messages"`
	AverageLatency      time.Duration `json:"average_latency"`
	P95Latency          time.Duration `json:"p95_latency"`
	P99Latency          time.Duration `json:"p99_latency"`
	ThroughputTPS       float64       `json:"throughput_tps"`
	TestDuration        time.Duration `json:"test_duration"`
	ConcurrentClients   int           `json:"concurrent_clients"`
	SystemResourceUsage ResourceUsage `json:"system_resource_usage"`
}

// ResourceUsage tracks system resource consumption during tests
type ResourceUsage struct {
	InitialMemoryMB   float64 `json:"initial_memory_mb"`
	FinalMemoryMB     float64 `json:"final_memory_mb"`
	MemoryDeltaMB     float64 `json:"memory_delta_mb"`
	InitialGoroutines int     `json:"initial_goroutines"`
	FinalGoroutines   int     `json:"final_goroutines"`
	GoroutineDelta    int     `json:"goroutine_delta"`
	CPUCores          int     `json:"cpu_cores"`
}

// TestEnhancedWorkerPerformance tests the enhanced worker under various load patterns
func TestEnhancedWorkerPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Test configurations for different scenarios
	testCases := []struct {
		name   string
		config PerformanceTestConfig
	}{
		{
			name: "Moderate Load Test",
			config: PerformanceTestConfig{
				BaseURL:           "http://localhost:8080",
				ClientID:          "550e8400-e29b-41d4-a716-446655440000",
				ConcurrentClients: 20,
				MessagesPerClient: 50,
				TestDuration:      2 * time.Minute,
				RampUpDuration:    30 * time.Second,
				MessageDistribution: MessageDistribution{
					RegularPercent: 70,
					ExpressPercent: 20,
					OTPPercent:     10,
				},
			},
		},
		{
			name: "High Load Test",
			config: PerformanceTestConfig{
				BaseURL:           "http://localhost:8080",
				ClientID:          "550e8400-e29b-41d4-a716-446655440000",
				ConcurrentClients: 50,
				MessagesPerClient: 100,
				TestDuration:      3 * time.Minute,
				RampUpDuration:    45 * time.Second,
				MessageDistribution: MessageDistribution{
					RegularPercent: 60,
					ExpressPercent: 30,
					OTPPercent:     10,
				},
			},
		},
		{
			name: "Burst Load Test",
			config: PerformanceTestConfig{
				BaseURL:           "http://localhost:8080",
				ClientID:          "550e8400-e29b-41d4-a716-446655440000",
				ConcurrentClients: 100,
				MessagesPerClient: 20,
				TestDuration:      1 * time.Minute,
				RampUpDuration:    10 * time.Second,
				MessageDistribution: MessageDistribution{
					RegularPercent: 50,
					ExpressPercent: 40,
					OTPPercent:     10,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := runPerformanceTest(t, tc.config)
			validatePerformanceResult(t, result, tc.config)
			logPerformanceResult(t, tc.name, result)
		})
	}
}

// runPerformanceTest executes a performance test with the given configuration
func runPerformanceTest(t *testing.T, config PerformanceTestConfig) PerformanceResult {
	// Record initial system resources
	initialMemory := getCurrentMemoryUsage()
	initialGoroutines := runtime.NumGoroutine()

	// Test metrics
	var totalMessages, successfulMessages, failedMessages int64
	var totalLatency int64
	latencies := make([]time.Duration, 0, config.ConcurrentClients*config.MessagesPerClient)
	var latencyMutex sync.Mutex

	// Worker synchronization
	var wg sync.WaitGroup
	startTime := time.Now()

	// Create client workers
	for i := 0; i < config.ConcurrentClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			// Ramp-up delay
			rampDelay := time.Duration(clientID) * config.RampUpDuration / time.Duration(config.ConcurrentClients)
			time.Sleep(rampDelay)

			// Send messages
			for j := 0; j < config.MessagesPerClient; j++ {
				messageType := selectMessageType(config.MessageDistribution, j)
				latency, success := sendTestMessage(config.BaseURL, config.ClientID, clientID, j, messageType)

				atomic.AddInt64(&totalMessages, 1)
				atomic.AddInt64(&totalLatency, latency.Milliseconds())

				if success {
					atomic.AddInt64(&successfulMessages, 1)
				} else {
					atomic.AddInt64(&failedMessages, 1)
				}

				// Record latency for percentile calculation
				latencyMutex.Lock()
				latencies = append(latencies, latency)
				latencyMutex.Unlock()

				// Brief pause to simulate realistic usage
				time.Sleep(time.Duration(50+clientID*10) * time.Millisecond)
			}
		}(i)
	}

	// Wait for all clients to complete
	wg.Wait()
	testDuration := time.Since(startTime)

	// Record final system resources
	finalMemory := getCurrentMemoryUsage()
	finalGoroutines := runtime.NumGoroutine()

	// Calculate performance metrics
	avgLatency := time.Duration(totalLatency/totalMessages) * time.Millisecond
	p95Latency := calculatePercentile(latencies, 95)
	p99Latency := calculatePercentile(latencies, 99)
	throughputTPS := float64(totalMessages) / testDuration.Seconds()

	return PerformanceResult{
		TotalMessages:      totalMessages,
		SuccessfulMessages: successfulMessages,
		FailedMessages:     failedMessages,
		AverageLatency:     avgLatency,
		P95Latency:         p95Latency,
		P99Latency:         p99Latency,
		ThroughputTPS:      throughputTPS,
		TestDuration:       testDuration,
		ConcurrentClients:  config.ConcurrentClients,
		SystemResourceUsage: ResourceUsage{
			InitialMemoryMB:   initialMemory,
			FinalMemoryMB:     finalMemory,
			MemoryDeltaMB:     finalMemory - initialMemory,
			InitialGoroutines: initialGoroutines,
			FinalGoroutines:   finalGoroutines,
			GoroutineDelta:    finalGoroutines - initialGoroutines,
			CPUCores:          runtime.NumCPU(),
		},
	}
}

// selectMessageType determines message type based on distribution
func selectMessageType(dist MessageDistribution, messageIndex int) string {
	remainder := messageIndex % 100

	if remainder < dist.RegularPercent {
		return "regular"
	} else if remainder < dist.RegularPercent+dist.ExpressPercent {
		return "express"
	}
	return "otp"
}

// sendTestMessage sends a single test message and measures latency
func sendTestMessage(baseURL, clientID string, clientIndex, messageIndex int, messageType string) (time.Duration, bool) {
	start := time.Now()

	var payload string
	switch messageType {
	case "regular":
		payload = fmt.Sprintf(`{
			"client_id": "%s",
			"to": "+1%03d%07d",
			"from": "PERF_TEST",
			"text": "Performance test message #%d from client %d"
		}`, clientID, clientIndex, messageIndex, messageIndex, clientIndex)
	case "express":
		payload = fmt.Sprintf(`{
			"client_id": "%s",
			"to": "+1%03d%07d",
			"from": "PERF_EXP",
			"text": "EXPRESS: Performance test message #%d from client %d",
			"express": true
		}`, clientID, clientIndex, messageIndex, messageIndex, clientIndex)
	case "otp":
		payload = fmt.Sprintf(`{
			"client_id": "%s",
			"to": "+1%03d%07d",
			"from": "PERF_OTP",
			"otp": true
		}`, clientID, clientIndex, messageIndex)
	}

	resp, err := http.Post(baseURL+"/v1/messages", "application/json", strings.NewReader(payload))
	if err != nil {
		return time.Since(start), false
	}
	defer resp.Body.Close()

	// Check response
	success := resp.StatusCode == 200 || resp.StatusCode == 202
	return time.Since(start), success
}

// getCurrentMemoryUsage returns current memory usage in MB
func getCurrentMemoryUsage() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024 / 1024
}

// calculatePercentile calculates the specified percentile of latencies
func calculatePercentile(latencies []time.Duration, percentile int) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Simple percentile calculation (for production, use a proper sorting algorithm)
	index := len(latencies) * percentile / 100
	if index >= len(latencies) {
		index = len(latencies) - 1
	}

	// Find approximate percentile value
	var sum time.Duration
	count := 0
	for _, latency := range latencies {
		sum += latency
		count++
		if count >= index {
			break
		}
	}

	if count > 0 {
		return sum / time.Duration(count)
	}
	return 0
}

// validatePerformanceResult validates that performance meets expectations
func validatePerformanceResult(t *testing.T, result PerformanceResult, config PerformanceTestConfig) {
	// Success rate should be above 80%
	successRate := float64(result.SuccessfulMessages) / float64(result.TotalMessages) * 100
	if successRate < 80.0 {
		t.Errorf("Success rate too low: %.1f%% (expected >= 80%%)", successRate)
	}

	// Average latency should be reasonable
	if result.AverageLatency > 2*time.Second {
		t.Errorf("Average latency too high: %v (expected <= 2s)", result.AverageLatency)
	}

	// P95 latency should be acceptable
	if result.P95Latency > 5*time.Second {
		t.Errorf("P95 latency too high: %v (expected <= 5s)", result.P95Latency)
	}

	// Throughput should meet minimum requirements
	minTPS := 100.0 // Minimum 100 TPS expected
	if result.ThroughputTPS < minTPS {
		t.Errorf("Throughput too low: %.1f TPS (expected >= %.1f TPS)", result.ThroughputTPS, minTPS)
	}

	// Memory usage should be reasonable
	if result.SystemResourceUsage.MemoryDeltaMB > 200 {
		t.Errorf("Memory usage too high: %.1f MB increase", result.SystemResourceUsage.MemoryDeltaMB)
	}
}

// logPerformanceResult logs comprehensive performance test results
func logPerformanceResult(t *testing.T, testName string, result PerformanceResult) {
	successRate := float64(result.SuccessfulMessages) / float64(result.TotalMessages) * 100

	t.Logf("\n"+
		"=== %s Results ===\n"+
		"Total Messages: %d\n"+
		"Success Rate: %.1f%% (%d successful, %d failed)\n"+
		"Throughput: %.1f TPS\n"+
		"Latency - Avg: %v, P95: %v, P99: %v\n"+
		"Test Duration: %v\n"+
		"Concurrent Clients: %d\n"+
		"System Resources:\n"+
		"  - CPU Cores: %d\n"+
		"  - Memory Delta: %.1f MB (%.1f -> %.1f MB)\n"+
		"  - Goroutine Delta: %d (%d -> %d)\n",
		testName,
		result.TotalMessages,
		successRate, result.SuccessfulMessages, result.FailedMessages,
		result.ThroughputTPS,
		result.AverageLatency, result.P95Latency, result.P99Latency,
		result.TestDuration,
		result.ConcurrentClients,
		result.SystemResourceUsage.CPUCores,
		result.SystemResourceUsage.MemoryDeltaMB,
		result.SystemResourceUsage.InitialMemoryMB,
		result.SystemResourceUsage.FinalMemoryMB,
		result.SystemResourceUsage.GoroutineDelta,
		result.SystemResourceUsage.InitialGoroutines,
		result.SystemResourceUsage.FinalGoroutines)
}
