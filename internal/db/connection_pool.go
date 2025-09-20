package db

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"time"

	_ "github.com/lib/pq"
)

// ConnectionPoolConfig defines database connection pool configuration
type ConnectionPoolConfig struct {
	// Connection limits
	MaxOpenConns    int           // Maximum open connections
	MaxIdleConns    int           // Maximum idle connections
	ConnMaxLifetime time.Duration // Maximum connection lifetime
	ConnMaxIdleTime time.Duration // Maximum connection idle time

	// Performance tuning
	MaxBatchSize int           // Maximum batch size for bulk operations
	QueryTimeout time.Duration // Query timeout

	// Health checks
	PingInterval time.Duration // Health check interval
}

// OptimizedPostgresDB wraps PostgresDB with advanced connection pooling
type OptimizedPostgresDB struct {
	*PostgresDB
	config ConnectionPoolConfig
}

// NewOptimizedPostgres creates a PostgreSQL connection with optimized pooling
func NewOptimizedPostgres(ctx context.Context, databaseURL string) (*OptimizedPostgresDB, error) {
	// Open database connection
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Calculate optimal connection pool settings based on system resources
	config := calculateOptimalPoolConfig()

	// Configure connection pool for high performance
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	optimizedDB := &OptimizedPostgresDB{
		PostgresDB: &PostgresDB{DB: db},
		config:     config,
	}

	// Start connection health monitoring
	go optimizedDB.healthMonitor(ctx)

	return optimizedDB, nil
}

// calculateOptimalPoolConfig determines optimal connection pool settings
func calculateOptimalPoolConfig() ConnectionPoolConfig {
	numCPU := runtime.NumCPU()

	return ConnectionPoolConfig{
		// Connection pool sizing for high concurrency
		MaxOpenConns:    numCPU * 8,       // 8 connections per CPU core
		MaxIdleConns:    numCPU * 4,       // 4 idle connections per CPU core
		ConnMaxLifetime: 1 * time.Hour,    // Rotate connections every hour
		ConnMaxIdleTime: 15 * time.Minute, // Close idle connections after 15 minutes

		// Performance settings
		MaxBatchSize: 1000,             // Batch up to 1000 operations
		QueryTimeout: 30 * time.Second, // 30 second query timeout

		// Health monitoring
		PingInterval: 30 * time.Second, // Health check every 30 seconds
	}
}

// healthMonitor continuously monitors database connection health
func (db *OptimizedPostgresDB) healthMonitor(ctx context.Context) {
	ticker := time.NewTicker(db.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := db.PingContext(ctx); err != nil {
				// Log connection health issue (would use logger in production)
				fmt.Printf("Database health check failed: %v\n", err)
			}
		}
	}
}

// BulkInsert performs optimized bulk insert operations
func (db *OptimizedPostgresDB) BulkInsert(ctx context.Context, query string, values [][]interface{}) error {
	if len(values) == 0 {
		return nil
	}

	// Process in batches to avoid memory issues and connection timeouts
	batchSize := db.config.MaxBatchSize
	for i := 0; i < len(values); i += batchSize {
		end := i + batchSize
		if end > len(values) {
			end = len(values)
		}

		if err := db.executeBatch(ctx, query, values[i:end]); err != nil {
			return fmt.Errorf("bulk insert batch failed: %w", err)
		}
	}

	return nil
}

// executeBatch executes a single batch of operations
func (db *OptimizedPostgresDB) executeBatch(ctx context.Context, query string, batch [][]interface{}) error {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, db.config.QueryTimeout)
	defer cancel()

	// Begin transaction for batch
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statement for reuse
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Execute batch operations
	for _, values := range batch {
		if _, err := stmt.ExecContext(ctx, values...); err != nil {
			return fmt.Errorf("failed to execute batch item: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit batch transaction: %w", err)
	}

	return nil
}

// GetConnectionStats returns current connection pool statistics
func (db *OptimizedPostgresDB) GetConnectionStats() ConnectionStats {
	stats := db.DB.Stats()

	return ConnectionStats{
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration,
		MaxIdleClosed:      stats.MaxIdleClosed,
		MaxLifetimeClosed:  stats.MaxLifetimeClosed,

		// Calculated metrics
		UtilizationPercent: float64(stats.InUse) / float64(stats.MaxOpenConnections) * 100,
		IdlePercent:        float64(stats.Idle) / float64(stats.MaxOpenConnections) * 100,
	}
}

// ConnectionStats represents database connection pool statistics
type ConnectionStats struct {
	MaxOpenConnections int           `json:"max_open_connections"`
	OpenConnections    int           `json:"open_connections"`
	InUse              int           `json:"in_use"`
	Idle               int           `json:"idle"`
	WaitCount          int64         `json:"wait_count"`
	WaitDuration       time.Duration `json:"wait_duration"`
	MaxIdleClosed      int64         `json:"max_idle_closed"`
	MaxLifetimeClosed  int64         `json:"max_lifetime_closed"`

	// Calculated metrics
	UtilizationPercent float64 `json:"utilization_percent"`
	IdlePercent        float64 `json:"idle_percent"`
}

// IsHealthy checks if the connection pool is in a healthy state
func (stats ConnectionStats) IsHealthy() bool {
	// Consider healthy if:
	// - Utilization is under 80%
	// - Not too many waits
	// - Reasonable wait duration
	return stats.UtilizationPercent < 80.0 &&
		stats.WaitDuration < 100*time.Millisecond
}
