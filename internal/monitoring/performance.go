package monitoring

import (
	"context"
	"log/slog"
	"runtime"
	"sync/atomic"
	"time"
)

// PerformanceMonitor tracks system performance metrics for production scaling
type PerformanceMonitor struct {
	logger *slog.Logger

	// Metrics (atomic for thread safety)
	totalRequests  int64
	successfulReqs int64
	failedReqs     int64
	totalLatency   int64 // milliseconds
	currentRPS     int64

	// System metrics
	lastGCTime    time.Time
	initialMemory uint64

	// Monitoring control
	stop     chan bool
	interval time.Duration
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(logger *slog.Logger) *PerformanceMonitor {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &PerformanceMonitor{
		logger:        logger,
		stop:          make(chan bool),
		interval:      30 * time.Second, // Report every 30 seconds
		initialMemory: m.Alloc,
		lastGCTime:    time.Now(),
	}
}

// Start begins performance monitoring
func (pm *PerformanceMonitor) Start(ctx context.Context) {
	go pm.monitorLoop(ctx)
	pm.logger.Info("Performance monitoring started", "interval", pm.interval)
}

// Stop stops performance monitoring
func (pm *PerformanceMonitor) Stop() {
	close(pm.stop)
	pm.logger.Info("Performance monitoring stopped")
}

// RecordRequest records a request with its latency and success status
func (pm *PerformanceMonitor) RecordRequest(latency time.Duration, success bool) {
	atomic.AddInt64(&pm.totalRequests, 1)
	atomic.AddInt64(&pm.totalLatency, latency.Milliseconds())

	if success {
		atomic.AddInt64(&pm.successfulReqs, 1)
	} else {
		atomic.AddInt64(&pm.failedReqs, 1)
	}
}

// GetCurrentRPS returns the current requests per second
func (pm *PerformanceMonitor) GetCurrentRPS() int64 {
	return atomic.LoadInt64(&pm.currentRPS)
}

// monitorLoop runs the monitoring loop
func (pm *PerformanceMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(pm.interval)
	defer ticker.Stop()

	var lastTotalRequests int64
	var lastTime = time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stop:
			return
		case <-ticker.C:
			pm.reportMetrics(&lastTotalRequests, &lastTime)
		}
	}
}

// reportMetrics reports comprehensive performance metrics
func (pm *PerformanceMonitor) reportMetrics(lastTotal *int64, lastTime *time.Time) {
	now := time.Now()
	currentTotal := atomic.LoadInt64(&pm.totalRequests)
	successful := atomic.LoadInt64(&pm.successfulReqs)
	failed := atomic.LoadInt64(&pm.failedReqs)
	totalLatency := atomic.LoadInt64(&pm.totalLatency)

	// Calculate RPS
	timeDiff := now.Sub(*lastTime).Seconds()
	requestDiff := currentTotal - *lastTotal
	currentRPS := float64(requestDiff) / timeDiff
	atomic.StoreInt64(&pm.currentRPS, int64(currentRPS))

	// Calculate success rate and average latency
	var successRate, avgLatency float64
	if currentTotal > 0 {
		successRate = float64(successful) / float64(currentTotal) * 100
		avgLatency = float64(totalLatency) / float64(currentTotal)
	}

	// Get system metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memoryUsageMB := float64(m.Alloc) / 1024 / 1024
	memoryDeltaMB := float64(m.Alloc-pm.initialMemory) / 1024 / 1024
	gcCount := m.NumGC

	// Check for performance issues
	performanceIssues := pm.detectPerformanceIssues(currentRPS, successRate, memoryUsageMB)

	// Log comprehensive metrics
	pm.logger.Info("Performance Metrics",
		// Request metrics
		"total_requests", currentTotal,
		"successful_requests", successful,
		"failed_requests", failed,
		"success_rate_pct", successRate,
		"current_rps", currentRPS,
		"avg_latency_ms", avgLatency,

		// System metrics
		"memory_usage_mb", memoryUsageMB,
		"memory_delta_mb", memoryDeltaMB,
		"gc_cycles", gcCount,
		"goroutines", runtime.NumGoroutine(),
		"cpu_cores", runtime.NumCPU(),

		// Performance assessment
		"performance_issues", performanceIssues,
		"scale_recommendation", pm.getScaleRecommendation(currentRPS, successRate))

	// Update tracking variables
	*lastTotal = currentTotal
	*lastTime = now
}

// detectPerformanceIssues identifies potential performance problems
func (pm *PerformanceMonitor) detectPerformanceIssues(rps, successRate, memoryMB float64) []string {
	var issues []string

	// Low success rate
	if successRate < 95.0 && pm.totalRequests > 100 {
		issues = append(issues, "low_success_rate")
	}

	// Low throughput
	if rps < 50.0 && pm.totalRequests > 100 {
		issues = append(issues, "low_throughput")
	}

	// High memory usage
	if memoryMB > 500 {
		issues = append(issues, "high_memory_usage")
	}

	// Too many goroutines
	if runtime.NumGoroutine() > 1000 {
		issues = append(issues, "goroutine_leak")
	}

	if len(issues) == 0 {
		issues = []string{"none"}
	}

	return issues
}

// getScaleRecommendation provides scaling recommendations based on current metrics
func (pm *PerformanceMonitor) getScaleRecommendation(rps, successRate float64) string {
	if rps > 800 && successRate > 95 {
		return "performing_well"
	} else if rps > 500 && successRate > 90 {
		return "good_performance"
	} else if rps > 200 && successRate > 85 {
		return "acceptable_performance"
	} else if rps < 100 {
		return "scale_up_needed"
	} else if successRate < 80 {
		return "reliability_issues"
	}
	return "monitor_closely"
}

// GetMetricsSummary returns a summary of current performance metrics
func (pm *PerformanceMonitor) GetMetricsSummary() PerformanceSummary {
	total := atomic.LoadInt64(&pm.totalRequests)
	successful := atomic.LoadInt64(&pm.successfulReqs)
	failed := atomic.LoadInt64(&pm.failedReqs)
	latency := atomic.LoadInt64(&pm.totalLatency)
	rps := atomic.LoadInt64(&pm.currentRPS)

	var successRate, avgLatency float64
	if total > 0 {
		successRate = float64(successful) / float64(total) * 100
		avgLatency = float64(latency) / float64(total)
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return PerformanceSummary{
		TotalRequests:  total,
		SuccessfulReqs: successful,
		FailedReqs:     failed,
		SuccessRate:    successRate,
		CurrentRPS:     float64(rps),
		AvgLatencyMs:   avgLatency,
		MemoryUsageMB:  float64(m.Alloc) / 1024 / 1024,
		GoroutineCount: runtime.NumGoroutine(),
		CPUCores:       runtime.NumCPU(),
	}
}

// PerformanceSummary represents a snapshot of performance metrics
type PerformanceSummary struct {
	TotalRequests  int64   `json:"total_requests"`
	SuccessfulReqs int64   `json:"successful_requests"`
	FailedReqs     int64   `json:"failed_requests"`
	SuccessRate    float64 `json:"success_rate_pct"`
	CurrentRPS     float64 `json:"current_rps"`
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
	MemoryUsageMB  float64 `json:"memory_usage_mb"`
	GoroutineCount int     `json:"goroutine_count"`
	CPUCores       int     `json:"cpu_cores"`
}
