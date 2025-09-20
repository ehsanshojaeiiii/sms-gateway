package worker

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// WorkerPool implements a high-performance worker pool with work-stealing queues
type WorkerPool struct {
	// Configuration
	poolSize    int
	queueSize   int
	
	// Work distribution
	workers     []*WorkerInstance
	globalQueue chan uuid.UUID
	
	// Lifecycle management
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	
	// Metrics (atomic for thread safety)
	submitted   int64
	processed   int64
	active      int64
	
	// Back-pressure control
	maxPending  int64
	currentPending int64
}

// WorkerInstance represents a single worker with its own local queue
type WorkerInstance struct {
	id          int
	localQueue  chan uuid.UUID
	processor   func(context.Context, uuid.UUID) error
	active      int64
	processed   int64
}

// NewWorkerPool creates an optimized worker pool
func NewWorkerPool(processor func(context.Context, uuid.UUID) error) *WorkerPool {
	// Optimal pool size: CPU cores × 4 for I/O bound work
	poolSize := runtime.NumCPU() * 4
	queueSize := poolSize * 100 // 100 messages per worker buffer
	
	ctx, cancel := context.WithCancel(context.Background())
	
	pool := &WorkerPool{
		poolSize:    poolSize,
		queueSize:   queueSize,
		globalQueue: make(chan uuid.UUID, queueSize),
		ctx:         ctx,
		cancel:      cancel,
		maxPending:  int64(poolSize * 200), // Back-pressure limit
	}
	
	// Create worker instances
	pool.workers = make([]*WorkerInstance, poolSize)
	for i := 0; i < poolSize; i++ {
		pool.workers[i] = &WorkerInstance{
			id:         i,
			localQueue: make(chan uuid.UUID, 50), // Local queue per worker
			processor:  processor,
		}
	}
	
	return pool
}

// Start begins all workers with work-stealing capability
func (p *WorkerPool) Start() {
	for i, worker := range p.workers {
		p.wg.Add(1)
		go p.runWorker(i, worker)
	}
	
	// Start work distributor
	p.wg.Add(1)
	go p.workDistributor()
}

// Submit adds a message to the worker pool with back-pressure control
func (p *WorkerPool) Submit(messageID uuid.UUID) error {
	// Check back-pressure
	if atomic.LoadInt64(&p.currentPending) >= p.maxPending {
		return ErrPoolOverloaded
	}
	
	select {
	case p.globalQueue <- messageID:
		atomic.AddInt64(&p.submitted, 1)
		atomic.AddInt64(&p.currentPending, 1)
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	default:
		return ErrPoolOverloaded
	}
}

// workDistributor distributes work to workers using work-stealing algorithm
func (p *WorkerPool) workDistributor() {
	defer p.wg.Done()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case messageID := <-p.globalQueue:
			// Find least busy worker
			selectedWorker := p.selectOptimalWorker()
			
			select {
			case selectedWorker.localQueue <- messageID:
				// Successfully distributed
			default:
				// Worker queue full, try work-stealing
				if !p.tryWorkStealing(messageID) {
					// All workers busy, put back in global queue
					select {
					case p.globalQueue <- messageID:
					default:
						// System overloaded, message will be retried later
					}
				}
			}
		}
	}
}

// selectOptimalWorker finds the worker with least load
func (p *WorkerPool) selectOptimalWorker() *WorkerInstance {
	minLoad := atomic.LoadInt64(&p.workers[0].active)
	selectedWorker := p.workers[0]
	
	for _, worker := range p.workers[1:] {
		if load := atomic.LoadInt64(&worker.active); load < minLoad {
			minLoad = load
			selectedWorker = worker
		}
	}
	
	return selectedWorker
}

// tryWorkStealing attempts to steal work from other workers
func (p *WorkerPool) tryWorkStealing(messageID uuid.UUID) bool {
	// Try each worker's local queue
	for _, worker := range p.workers {
		select {
		case worker.localQueue <- messageID:
			return true
		default:
			continue
		}
	}
	return false
}

// runWorker executes the worker loop with work-stealing capability
func (p *WorkerPool) runWorker(workerID int, worker *WorkerInstance) {
	defer p.wg.Done()
	
	for {
		select {
		case <-p.ctx.Done():
			return
			
		case messageID := <-worker.localQueue:
			// Process from local queue
			p.processMessage(worker, messageID)
			
		default:
			// Try to steal work from global queue
			select {
			case messageID := <-p.globalQueue:
				p.processMessage(worker, messageID)
			case <-p.ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
				// Brief pause to prevent busy waiting
				continue
			}
		}
	}
}

// processMessage handles individual message processing
func (p *WorkerPool) processMessage(worker *WorkerInstance, messageID uuid.UUID) {
	atomic.AddInt64(&worker.active, 1)
	atomic.AddInt64(&p.active, 1)
	defer func() {
		atomic.AddInt64(&worker.active, -1)
		atomic.AddInt64(&p.active, -1)
		atomic.AddInt64(&p.currentPending, -1)
		atomic.AddInt64(&worker.processed, 1)
		atomic.AddInt64(&p.processed, 1)
	}()
	
	// Process the message
	if err := worker.processor(p.ctx, messageID); err != nil {
		// Error handling - could implement retry logic here
	}
}

// Stop gracefully shuts down the worker pool
func (p *WorkerPool) Stop(timeout time.Duration) error {
	p.cancel()
	
	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return ErrShutdownTimeout
	}
}

// Stats returns current pool statistics
func (p *WorkerPool) Stats() PoolStats {
	return PoolStats{
		PoolSize:       p.poolSize,
		Submitted:      atomic.LoadInt64(&p.submitted),
		Processed:      atomic.LoadInt64(&p.processed),
		Active:         atomic.LoadInt64(&p.active),
		Pending:        atomic.LoadInt64(&p.currentPending),
		QueueUtilization: float64(len(p.globalQueue)) / float64(cap(p.globalQueue)),
	}
}

// PoolStats represents worker pool statistics
type PoolStats struct {
	PoolSize         int     `json:"pool_size"`
	Submitted        int64   `json:"submitted"`
	Processed        int64   `json:"processed"`
	Active           int64   `json:"active"`
	Pending          int64   `json:"pending"`
	QueueUtilization float64 `json:"queue_utilization"`
}

// Errors
var (
	ErrPoolOverloaded   = fmt.Errorf("worker pool overloaded")
	ErrShutdownTimeout  = fmt.Errorf("shutdown timeout exceeded")
)
