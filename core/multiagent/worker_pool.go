package multiagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Ranganaths/minion/observability"
	"github.com/google/uuid"
)

// WorkerPoolConfig configures worker pool behavior
type WorkerPoolConfig struct {
	// Initial workers
	InitialWorkers map[string]int // capability -> count

	// Defaults
	DefaultCapability string
	DefaultPriority   int

	// Health monitoring
	HealthCheckInterval time.Duration
	HeartbeatTimeout    time.Duration

	// Graceful shutdown
	ShutdownTimeout time.Duration
}

// DefaultWorkerPoolConfig returns default worker pool configuration
func DefaultWorkerPoolConfig() *WorkerPoolConfig {
	return &WorkerPoolConfig{
		InitialWorkers: map[string]int{
			"code_generation": 2,
			"data_analysis":   2,
		},
		DefaultCapability:   "general",
		DefaultPriority:     5,
		HealthCheckInterval: 30 * time.Second,
		HeartbeatTimeout:    2 * time.Minute,
		ShutdownTimeout:     30 * time.Second,
	}
}

// WorkerPool manages a dynamic pool of workers
type WorkerPool struct {
	config   *WorkerPoolConfig
	protocol Protocol
	ledger   LedgerBackend

	// Worker management
	mu              sync.RWMutex
	workers         map[string]*WorkerAgent          // workerID -> worker
	workersByType   map[string][]string              // capability -> workerIDs
	workerStatus    map[string]AgentStatus           // workerID -> status
	workerHeartbeat map[string]time.Time             // workerID -> last heartbeat

	// Metrics
	metrics *observability.MetricsCollector
	logger  *observability.Logger

	// Control
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(config *WorkerPoolConfig, protocol Protocol, ledger LedgerBackend) *WorkerPool {
	if config == nil {
		config = DefaultWorkerPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		config:          config,
		protocol:        protocol,
		ledger:          ledger,
		workers:         make(map[string]*WorkerAgent),
		workersByType:   make(map[string][]string),
		workerStatus:    make(map[string]AgentStatus),
		workerHeartbeat: make(map[string]time.Time),
		metrics:         observability.GetMetrics(),
		logger:          observability.GetLogger(),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start(ctx context.Context) error {
	wp.mu.Lock()
	if wp.running {
		wp.mu.Unlock()
		return fmt.Errorf("worker pool already running")
	}
	wp.running = true
	wp.mu.Unlock()

	wp.logger.Info("Starting worker pool")

	// Start initial workers
	for capability, count := range wp.config.InitialWorkers {
		for i := 0; i < count; i++ {
			if err := wp.AddWorker(ctx, capability); err != nil {
				wp.logger.Error("Failed to start initial worker",
					observability.String("capability", capability),
					observability.String("error", err.Error()),
				)
			}
		}
	}

	// Start health monitoring
	go wp.healthMonitor()

	return nil
}

// Stop stops the worker pool gracefully
func (wp *WorkerPool) Stop(ctx context.Context) error {
	wp.mu.Lock()
	if !wp.running {
		wp.mu.Unlock()
		return fmt.Errorf("worker pool not running")
	}
	wp.running = false
	wp.mu.Unlock()

	wp.logger.Info("Stopping worker pool")

	// Stop all workers
	wp.mu.RLock()
	workerIDs := make([]string, 0, len(wp.workers))
	for id := range wp.workers {
		workerIDs = append(workerIDs, id)
	}
	wp.mu.RUnlock()

	for _, id := range workerIDs {
		wp.RemoveWorker(ctx, id)
	}

	wp.cancel()

	return nil
}

// AddWorker adds a new worker to the pool
func (wp *WorkerPool) AddWorker(ctx context.Context, capability string) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	// Create worker metadata
	workerID := fmt.Sprintf("worker-%s-%s", capability, uuid.New().String()[:8])
	metadata := &AgentMetadata{
		AgentID:      workerID,
		Role:         RoleWorker,
		Capabilities: []string{capability},
		Priority:     wp.config.DefaultPriority,
		Status:       StatusIdle,
	}

	// Create worker agent
	worker := &WorkerAgent{
		metadata: metadata,
		protocol: wp.protocol,
	}

	// Register worker
	wp.workers[workerID] = worker
	wp.workerStatus[workerID] = StatusIdle
	wp.workerHeartbeat[workerID] = time.Now()

	// Track by capability
	wp.workersByType[capability] = append(wp.workersByType[capability], workerID)

	wp.logger.Info("Added worker to pool",
		observability.String("worker_id", workerID),
		observability.String("capability", capability),
	)

	// Record metrics
	if wp.metrics != nil {
		wp.metrics.RecordMultiagentWorkerIdle()
	}

	return nil
}

// RemoveWorker removes a worker from the pool
func (wp *WorkerPool) RemoveWorker(ctx context.Context, workerID string) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	worker, exists := wp.workers[workerID]
	if !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	// Remove from capability tracking
	for capability, workers := range wp.workersByType {
		for i, id := range workers {
			if id == workerID {
				wp.workersByType[capability] = append(workers[:i], workers[i+1:]...)
				break
			}
		}
	}

	// Remove from maps
	delete(wp.workers, workerID)
	delete(wp.workerStatus, workerID)
	delete(wp.workerHeartbeat, workerID)

	wp.logger.Info("Removed worker from pool",
		observability.String("worker_id", workerID),
		observability.String("capability", worker.metadata.Capabilities[0]),
	)

	return nil
}

// GetWorker retrieves a worker by ID
func (wp *WorkerPool) GetWorker(workerID string) (*WorkerAgent, error) {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	worker, exists := wp.workers[workerID]
	if !exists {
		return nil, fmt.Errorf("worker %s not found", workerID)
	}

	return worker, nil
}

// GetWorkersByCapability returns workers with a specific capability
func (wp *WorkerPool) GetWorkersByCapability(capability string) []*WorkerAgent {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	workerIDs := wp.workersByType[capability]
	workers := make([]*WorkerAgent, 0, len(workerIDs))

	for _, id := range workerIDs {
		if worker, ok := wp.workers[id]; ok {
			workers = append(workers, worker)
		}
	}

	return workers
}

// GetIdleWorkers returns idle workers (optionally filtered by capability)
func (wp *WorkerPool) GetIdleWorkers(capability string, limit int) []*WorkerAgent {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	var workers []*WorkerAgent

	for id, status := range wp.workerStatus {
		if status != StatusIdle {
			continue
		}

		worker, ok := wp.workers[id]
		if !ok {
			continue
		}

		// Filter by capability if specified
		if capability != "" {
			hasCapability := false
			for _, cap := range worker.metadata.Capabilities {
				if cap == capability {
					hasCapability = true
					break
				}
			}
			if !hasCapability {
				continue
			}
		}

		workers = append(workers, worker)

		if limit > 0 && len(workers) >= limit {
			break
		}
	}

	return workers
}

// UpdateWorkerStatus updates worker status
func (wp *WorkerPool) UpdateWorkerStatus(workerID string, status AgentStatus) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if _, exists := wp.workers[workerID]; !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	oldStatus := wp.workerStatus[workerID]
	wp.workerStatus[workerID] = status
	wp.workerHeartbeat[workerID] = time.Now()

	// Record metrics
	if wp.metrics != nil {
		if oldStatus == StatusBusy && status == StatusIdle {
			wp.metrics.RecordMultiagentWorkerIdle()
		} else if oldStatus == StatusIdle && status == StatusBusy {
			wp.metrics.RecordMultiagentWorkerBusy()
		}
	}

	return nil
}

// GetStats returns worker pool statistics
func (wp *WorkerPool) GetStats() *WorkerStats {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	stats := &WorkerStats{
		TotalWorkers: len(wp.workers),
		Capabilities: make(map[string]int),
	}

	for _, status := range wp.workerStatus {
		if status == StatusBusy {
			stats.BusyWorkers++
		} else if status == StatusIdle {
			stats.IdleWorkers++
		}
	}

	// Calculate utilization
	if stats.TotalWorkers > 0 {
		stats.Utilization = float64(stats.BusyWorkers) / float64(stats.TotalWorkers)
	}

	// Count by capability
	for capability, workers := range wp.workersByType {
		stats.Capabilities[capability] = len(workers)
	}

	// Get pending tasks count (would need integration with task ledger)
	// For now, estimate from metrics
	stats.PendingTasks = 0 // Placeholder

	return stats
}

// ScaleUp adds workers to the pool
func (wp *WorkerPool) ScaleUp(ctx context.Context, count int, capability string) error {
	if capability == "" {
		capability = wp.config.DefaultCapability
	}

	wp.logger.Info("Scaling up workers",
		observability.Int("count", count),
		observability.String("capability", capability),
	)

	for i := 0; i < count; i++ {
		if err := wp.AddWorker(ctx, capability); err != nil {
			return fmt.Errorf("failed to add worker %d: %w", i+1, err)
		}
	}

	return nil
}

// ScaleDown removes workers from the pool
func (wp *WorkerPool) ScaleDown(ctx context.Context, count int, capability string) error {
	wp.logger.Info("Scaling down workers",
		observability.Int("count", count),
		observability.String("capability", capability),
	)

	// Get idle workers to remove
	idleWorkers := wp.GetIdleWorkers(capability, count)

	removed := 0
	for _, worker := range idleWorkers {
		if err := wp.RemoveWorker(ctx, worker.metadata.AgentID); err != nil {
			wp.logger.Error("Failed to remove worker",
				observability.String("worker_id", worker.metadata.AgentID),
				observability.String("error", err.Error()),
			)
		} else {
			removed++
		}

		if removed >= count {
			break
		}
	}

	if removed < count {
		return fmt.Errorf("only removed %d of %d workers", removed, count)
	}

	return nil
}

// healthMonitor monitors worker health
func (wp *WorkerPool) healthMonitor() {
	ticker := time.NewTicker(wp.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wp.checkWorkerHealth()
		case <-wp.ctx.Done():
			return
		}
	}
}

// checkWorkerHealth checks health of all workers
func (wp *WorkerPool) checkWorkerHealth() {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	now := time.Now()
	timeout := wp.config.HeartbeatTimeout

	for workerID, lastHeartbeat := range wp.workerHeartbeat {
		if now.Sub(lastHeartbeat) > timeout {
			wp.logger.Warn("Worker heartbeat timeout",
				observability.String("worker_id", workerID),
				observability.Duration("timeout", now.Sub(lastHeartbeat)),
			)

			// Mark as failed
			wp.workerStatus[workerID] = StatusFailed

			// Record metric
			if wp.metrics != nil {
				wp.metrics.RecordMultiagentError("worker_pool", "heartbeat_timeout")
			}
		}
	}
}

// GetAllWorkers returns all workers
func (wp *WorkerPool) GetAllWorkers() []*WorkerAgent {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	workers := make([]*WorkerAgent, 0, len(wp.workers))
	for _, worker := range wp.workers {
		workers = append(workers, worker)
	}

	return workers
}

// GetWorkerCount returns count of workers by capability
func (wp *WorkerPool) GetWorkerCount(capability string) int {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	if capability == "" {
		return len(wp.workers)
	}

	return len(wp.workersByType[capability])
}
