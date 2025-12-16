package multiagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/agentql/agentql/pkg/minion/observability"
	"github.com/google/uuid"
)

// CoordinatorConfig configures the multi-agent coordinator
type CoordinatorConfig struct {
	// Protocol configuration
	ProtocolSecurity *SecurityPolicy

	// Orchestrator configuration
	OrchestratorConfig *OrchestratorConfig

	// Agent limits
	MaxWorkers int

	// Group management
	DefaultGroupID string
}

// DefaultCoordinatorConfig returns default configuration
func DefaultCoordinatorConfig() *CoordinatorConfig {
	return &CoordinatorConfig{
		ProtocolSecurity: &SecurityPolicy{
			RequireAuthentication: false,
			RequireEncryption:     false,
			MaxMessageSize:        1024 * 1024,
			RateLimitPerSecond:    1000,
		},
		OrchestratorConfig: DefaultOrchestratorConfig(),
		MaxWorkers:         10,
		DefaultGroupID:     "default",
	}
}

// Coordinator manages the entire multi-agent system
type Coordinator struct {
	mu            sync.RWMutex
	config        *CoordinatorConfig
	protocol      Protocol
	orchestrator  *Orchestrator
	workers       map[string]*WorkerAgent
	llmProvider   LLMProvider
	groupID       string
	metrics       *observability.MetricsCollector
	tracer        *observability.Tracer
}

// NewCoordinator creates a new multi-agent coordinator
func NewCoordinator(llmProvider LLMProvider, config *CoordinatorConfig) *Coordinator {
	if config == nil {
		config = DefaultCoordinatorConfig()
	}

	// Create protocol
	protocol := NewInMemoryProtocol(config.ProtocolSecurity)

	// Create orchestrator
	orchestrator := NewOrchestrator(protocol, llmProvider, config.OrchestratorConfig)

	// Initialize metrics collector
	metrics := observability.GetMetrics()

	// Initialize tracer
	tracer := observability.GetTracer()

	return &Coordinator{
		config:       config,
		protocol:     protocol,
		orchestrator: orchestrator,
		workers:      make(map[string]*WorkerAgent),
		llmProvider:  llmProvider,
		groupID:      config.DefaultGroupID,
		metrics:      metrics,
		tracer:       tracer,
	}
}

// Initialize initializes the coordinator with default workers
func (c *Coordinator) Initialize(ctx context.Context) error {
	// Create default workers
	workers := []struct {
		handler TaskHandler
		role    AgentRole
	}{
		{NewCoderWorker(c.llmProvider), RoleSpecialist},
		{NewAnalystWorker(c.llmProvider), RoleSpecialist},
		{NewResearcherWorker(c.llmProvider), RoleSpecialist},
		{NewWriterWorker(c.llmProvider), RoleSpecialist},
		{NewReviewerWorker(c.llmProvider), RoleSpecialist},
	}

	for _, w := range workers {
		metadata := &AgentMetadata{
			AgentID:      uuid.New().String(),
			Role:         w.role,
			Capabilities: w.handler.GetCapabilities(),
			GroupID:      c.groupID,
			Priority:     5,
			Status:       StatusIdle,
		}

		worker := NewWorkerAgent(metadata, c.protocol, w.handler)

		if err := c.RegisterWorker(ctx, worker); err != nil {
			return fmt.Errorf("failed to register %s worker: %w", w.handler.GetName(), err)
		}
	}

	return nil
}

// RegisterWorker registers a worker agent
func (c *Coordinator) RegisterWorker(ctx context.Context, worker *WorkerAgent) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.workers) >= c.config.MaxWorkers {
		return fmt.Errorf("maximum number of workers reached (%d)", c.config.MaxWorkers)
	}

	metadata := worker.GetMetadata()

	// Register with orchestrator
	if err := c.orchestrator.RegisterWorker(metadata); err != nil {
		return fmt.Errorf("failed to register with orchestrator: %w", err)
	}

	// Start worker
	if err := worker.Start(ctx); err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}

	c.workers[metadata.AgentID] = worker

	// Record metrics
	c.metrics.SetMultiagentActiveWorkers(len(c.workers))
	c.metrics.RecordMultiagentWorkerIdle()

	return nil
}

// UnregisterWorker removes a worker agent
func (c *Coordinator) UnregisterWorker(ctx context.Context, agentID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	worker, exists := c.workers[agentID]
	if !exists {
		return fmt.Errorf("worker %s not found", agentID)
	}

	// Stop worker
	if err := worker.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop worker: %w", err)
	}

	// Unregister from orchestrator
	if err := c.orchestrator.UnregisterWorker(agentID); err != nil {
		return fmt.Errorf("failed to unregister from orchestrator: %w", err)
	}

	delete(c.workers, agentID)

	// Record metrics
	c.metrics.SetMultiagentActiveWorkers(len(c.workers))

	return nil
}

// ExecuteTask executes a complex task using the multi-agent system
func (c *Coordinator) ExecuteTask(ctx context.Context, req *TaskRequest) (*TaskResult, error) {
	// Start tracing span
	taskID := uuid.New().String()
	ctx, span := c.tracer.StartMultiAgentTaskSpan(ctx, taskID, req.Name, req.Type, int(req.Priority))
	defer c.tracer.EndSpan(span, nil) // Will be updated with error if any

	// Record task start
	c.metrics.RecordMultiagentTaskStarted()
	start := time.Now()

	// Execute task
	result, err := c.orchestrator.ExecuteTask(ctx, req)
	duration := time.Since(start)

	// Record task completion or failure
	if err != nil {
		c.metrics.RecordMultiagentTaskFailed(req.Type, duration)
		c.metrics.RecordMultiagentError("coordinator", "task_execution_failed")
		span.SetStatus(1, err.Error()) // Set error status on span
		span.RecordError(err)
	} else {
		c.metrics.RecordMultiagentTaskCompleted(req.Type, duration)
	}

	return result, err
}

// GetWorkers returns all registered workers
func (c *Coordinator) GetWorkers() []*AgentMetadata {
	return c.orchestrator.GetWorkers()
}

// GetTaskLedger returns the task ledger
func (c *Coordinator) GetTaskLedger() *TaskLedger {
	return c.orchestrator.taskLedger
}

// GetProgressLedger returns the progress ledger
func (c *Coordinator) GetProgressLedger() *ProgressLedger {
	return c.orchestrator.progressLedger
}

// GetProtocol returns the communication protocol
func (c *Coordinator) GetProtocol() Protocol {
	return c.protocol
}

// GetMetrics returns protocol metrics
func (c *Coordinator) GetMetrics() *ProtocolMetrics {
	if impl, ok := c.protocol.(*InMemoryProtocol); ok {
		return impl.GetMetrics()
	}
	return nil
}

// Shutdown gracefully shuts down the coordinator
func (c *Coordinator) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Stop all workers
	for agentID, worker := range c.workers {
		if err := worker.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop worker %s: %w", agentID, err)
		}
	}

	return nil
}

// RegisterGroup registers a group of agents
func (c *Coordinator) RegisterGroup(groupID string, agentIDs []string) error {
	if impl, ok := c.protocol.(*InMemoryProtocol); ok {
		return impl.RegisterGroup(groupID, agentIDs)
	}
	return fmt.Errorf("protocol does not support groups")
}

// BroadcastToGroup sends a message to all agents in a group
func (c *Coordinator) BroadcastToGroup(ctx context.Context, msg *Message, groupID string) error {
	return c.protocol.Broadcast(ctx, msg, groupID)
}

// CreateCustomWorker creates a custom worker with a custom task handler
func (c *Coordinator) CreateCustomWorker(
	ctx context.Context,
	name string,
	role AgentRole,
	handler TaskHandler,
) (*WorkerAgent, error) {
	metadata := &AgentMetadata{
		AgentID:      uuid.New().String(),
		Role:         role,
		Capabilities: handler.GetCapabilities(),
		GroupID:      c.groupID,
		Priority:     5,
		Status:       StatusIdle,
		CustomData: map[string]interface{}{
			"name": name,
		},
	}

	worker := NewWorkerAgent(metadata, c.protocol, handler)

	if err := c.RegisterWorker(ctx, worker); err != nil {
		return nil, err
	}

	return worker, nil
}

// MonitoringStats provides monitoring statistics
type MonitoringStats struct {
	TotalWorkers     int                `json:"total_workers"`
	IdleWorkers      int                `json:"idle_workers"`
	BusyWorkers      int                `json:"busy_workers"`
	OfflineWorkers   int                `json:"offline_workers"`
	TotalTasks       int                `json:"total_tasks"`
	CompletedTasks   int                `json:"completed_tasks"`
	FailedTasks      int                `json:"failed_tasks"`
	PendingTasks     int                `json:"pending_tasks"`
	ProtocolMetrics  *ProtocolMetrics   `json:"protocol_metrics"`
	WorkersByRole    map[AgentRole]int  `json:"workers_by_role"`
	Timestamp        time.Time          `json:"timestamp"`
}

// GetMonitoringStats returns monitoring statistics
func (c *Coordinator) GetMonitoringStats(ctx context.Context) (*MonitoringStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := &MonitoringStats{
		TotalWorkers:   len(c.workers),
		WorkersByRole:  make(map[AgentRole]int),
		Timestamp:      time.Now(),
		ProtocolMetrics: c.GetMetrics(),
	}

	// Count workers by status
	for _, worker := range c.workers {
		metadata := worker.GetMetadata()

		switch metadata.Status {
		case StatusIdle:
			stats.IdleWorkers++
		case StatusBusy:
			stats.BusyWorkers++
		case StatusOffline:
			stats.OfflineWorkers++
		}

		stats.WorkersByRole[metadata.Role]++
	}

	// Get task statistics
	taskLedger := c.GetTaskLedger()
	taskLedger.mu.RLock()
	stats.TotalTasks = len(taskLedger.tasks)
	for _, task := range taskLedger.tasks {
		switch task.Status {
		case TaskStatusCompleted:
			stats.CompletedTasks++
		case TaskStatusFailed:
			stats.FailedTasks++
		case TaskStatusPending:
			stats.PendingTasks++
		}
	}
	taskLedger.mu.RUnlock()

	// Update metrics gauges
	c.metrics.SetMultiagentActiveWorkers(stats.TotalWorkers)
	c.metrics.SetMultiagentTaskLedgerSize(stats.TotalTasks)

	// Get progress ledger size
	progressLedger := c.GetProgressLedger()
	progressLedger.mu.RLock()
	totalEntries := 0
	for _, entries := range progressLedger.entries {
		totalEntries += len(entries)
	}
	progressLedger.mu.RUnlock()
	c.metrics.SetMultiagentProgressLedgerSize(totalEntries)

	return stats, nil
}

// HealthCheck performs a health check on the system
type HealthStatus struct {
	Status      string            `json:"status"`
	Components  map[string]string `json:"components"`
	Errors      []string          `json:"errors,omitempty"`
	CheckedAt   time.Time         `json:"checked_at"`
}

// HealthCheck performs a health check
func (c *Coordinator) HealthCheck(ctx context.Context) *HealthStatus {
	status := &HealthStatus{
		Status:     "healthy",
		Components: make(map[string]string),
		Errors:     []string{},
		CheckedAt:  time.Now(),
	}

	// Check workers
	c.mu.RLock()
	workerCount := len(c.workers)
	c.mu.RUnlock()

	if workerCount == 0 {
		status.Components["workers"] = "degraded"
		status.Errors = append(status.Errors, "no workers registered")
		status.Status = "degraded"
	} else {
		status.Components["workers"] = "healthy"
	}

	// Check protocol
	if c.protocol != nil {
		status.Components["protocol"] = "healthy"
	} else {
		status.Components["protocol"] = "unhealthy"
		status.Errors = append(status.Errors, "protocol not initialized")
		status.Status = "unhealthy"
	}

	// Check orchestrator
	if c.orchestrator != nil {
		status.Components["orchestrator"] = "healthy"
	} else {
		status.Components["orchestrator"] = "unhealthy"
		status.Errors = append(status.Errors, "orchestrator not initialized")
		status.Status = "unhealthy"
	}

	// Check LLM provider
	if c.llmProvider != nil {
		status.Components["llm_provider"] = "healthy"
	} else {
		status.Components["llm_provider"] = "unhealthy"
		status.Errors = append(status.Errors, "LLM provider not initialized")
		status.Status = "unhealthy"
	}

	return status
}
