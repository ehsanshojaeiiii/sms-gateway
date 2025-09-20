package config

import (
	"os"
	"strconv"
)

// WorkerMode defines the worker implementation to use
type WorkerMode string

const (
	WorkerModeSimple   WorkerMode = "simple"   // Original simple worker
	WorkerModeEnhanced WorkerMode = "enhanced" // Advanced high-performance worker
)

// WorkerConfig defines worker-specific configuration
type WorkerConfig struct {
	Mode          WorkerMode `json:"mode"`
	PoolSize      int        `json:"pool_size"`
	BatchSize     int        `json:"batch_size"`
	BufferSize    int        `json:"buffer_size"`
	EnableMetrics bool       `json:"enable_metrics"`
}

// GetWorkerConfig returns worker configuration from environment variables
func GetWorkerConfig() WorkerConfig {
	// Default to simple worker for backward compatibility
	mode := WorkerModeSimple
	if envMode := os.Getenv("WORKER_MODE"); envMode != "" {
		if envMode == "enhanced" {
			mode = WorkerModeEnhanced
		}
	}

	// Pool size (defaults to CPU cores * 4 for enhanced, 1 for simple)
	poolSize := 1
	if mode == WorkerModeEnhanced {
		poolSize = 0 // 0 means auto-detect in enhanced worker
	}
	if envPoolSize := os.Getenv("WORKER_POOL_SIZE"); envPoolSize != "" {
		if size, err := strconv.Atoi(envPoolSize); err == nil && size > 0 {
			poolSize = size
		}
	}

	// Batch size for message processing
	batchSize := 1
	if mode == WorkerModeEnhanced {
		batchSize = 50 // Default batch size for enhanced mode
	}
	if envBatchSize := os.Getenv("WORKER_BATCH_SIZE"); envBatchSize != "" {
		if size, err := strconv.Atoi(envBatchSize); err == nil && size > 0 {
			batchSize = size
		}
	}

	// Buffer size for message queue
	bufferSize := 100
	if mode == WorkerModeEnhanced {
		bufferSize = 1000 // Larger buffer for enhanced mode
	}
	if envBufferSize := os.Getenv("WORKER_BUFFER_SIZE"); envBufferSize != "" {
		if size, err := strconv.Atoi(envBufferSize); err == nil && size > 0 {
			bufferSize = size
		}
	}

	// Enable advanced metrics
	enableMetrics := mode == WorkerModeEnhanced
	if envMetrics := os.Getenv("WORKER_ENABLE_METRICS"); envMetrics != "" {
		enableMetrics = envMetrics == "true" || envMetrics == "1"
	}

	return WorkerConfig{
		Mode:          mode,
		PoolSize:      poolSize,
		BatchSize:     batchSize,
		BufferSize:    bufferSize,
		EnableMetrics: enableMetrics,
	}
}
