package multiagent

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/agentql/agentql/pkg/minion/observability"
)

// LoadBalancerStrategy defines the load balancing strategy
type LoadBalancerStrategy string

const (
	StrategyRoundRobin     LoadBalancerStrategy = "round_robin"
	StrategyLeastLoaded    LoadBalancerStrategy = "least_loaded"
	StrategyRandom         LoadBalancerStrategy = "random"
	StrategyCapabilityBest LoadBalancerStrategy = "capability_best"
	StrategyLatencyBased   LoadBalancerStrategy = "latency_based"
	StrategyWeightedRound  LoadBalancerStrategy = "weighted_round"
)

// LoadBalancer interface for worker selection
type LoadBalancer interface {
	// SelectWorker selects the best worker for a task
	SelectWorker(ctx context.Context, task *Task, workers []*WorkerAgent) (*WorkerAgent, error)

	// RecordResult records task execution result for learning
	RecordResult(workerID string, task *Task, duration time.Duration, err error)

	// GetStats returns load balancer statistics
	GetStats() map[string]interface{}
}

// LoadBalancerConfig configures load balancer behavior
type LoadBalancerConfig struct {
	Strategy            LoadBalancerStrategy
	EnablePerformanceTracking bool
	TrackingWindowSize  int
	WeightDecayFactor   float64
}

// DefaultLoadBalancerConfig returns default configuration
func DefaultLoadBalancerConfig() *LoadBalancerConfig {
	return &LoadBalancerConfig{
		Strategy:                  StrategyCapabilityBest,
		EnablePerformanceTracking: true,
		TrackingWindowSize:        100,
		WeightDecayFactor:         0.9,
	}
}

// LoadBalancerFactory creates load balancer instances
type LoadBalancerFactory struct {
	config *LoadBalancerConfig
}

// NewLoadBalancerFactory creates a new factory
func NewLoadBalancerFactory(config *LoadBalancerConfig) *LoadBalancerFactory {
	if config == nil {
		config = DefaultLoadBalancerConfig()
	}
	return &LoadBalancerFactory{config: config}
}

// CreateLoadBalancer creates a load balancer based on strategy
func (f *LoadBalancerFactory) CreateLoadBalancer() LoadBalancer {
	switch f.config.Strategy {
	case StrategyRoundRobin:
		return NewRoundRobinBalancer()
	case StrategyLeastLoaded:
		return NewLeastLoadedBalancer()
	case StrategyRandom:
		return NewRandomBalancer()
	case StrategyCapabilityBest:
		return NewCapabilityBalancer(f.config)
	case StrategyLatencyBased:
		return NewLatencyBasedBalancer(f.config)
	case StrategyWeightedRound:
		return NewWeightedRoundRobinBalancer()
	default:
		return NewCapabilityBalancer(f.config)
	}
}

// Common errors
var (
	ErrNoWorkerAvailable = fmt.Errorf("no worker available")
	ErrNoCapableWorker   = fmt.Errorf("no worker with required capability")
)

// ===== Round Robin Balancer =====

type RoundRobinBalancer struct {
	mu      sync.Mutex
	counter int
	logger  *observability.Logger
}

func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{
		logger: observability.GetLogger(),
	}
}

func (rb *RoundRobinBalancer) SelectWorker(ctx context.Context, task *Task, workers []*WorkerAgent) (*WorkerAgent, error) {
	if len(workers) == 0 {
		return nil, ErrNoWorkerAvailable
	}

	// Filter capable workers
	capable := filterCapableWorkers(task, workers)
	if len(capable) == 0 {
		return nil, ErrNoCapableWorker
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	selected := capable[rb.counter%len(capable)]
	rb.counter++

	return selected, nil
}

func (rb *RoundRobinBalancer) RecordResult(workerID string, task *Task, duration time.Duration, err error) {
	// No-op for round robin
}

func (rb *RoundRobinBalancer) GetStats() map[string]interface{} {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	return map[string]interface{}{
		"strategy":       "round_robin",
		"total_requests": rb.counter,
	}
}

// ===== Least Loaded Balancer =====

type LeastLoadedBalancer struct {
	mu         sync.RWMutex
	taskCounts map[string]int // workerID -> active task count
	logger     *observability.Logger
}

func NewLeastLoadedBalancer() *LeastLoadedBalancer {
	return &LeastLoadedBalancer{
		taskCounts: make(map[string]int),
		logger:     observability.GetLogger(),
	}
}

func (lb *LeastLoadedBalancer) SelectWorker(ctx context.Context, task *Task, workers []*WorkerAgent) (*WorkerAgent, error) {
	if len(workers) == 0 {
		return nil, ErrNoWorkerAvailable
	}

	capable := filterCapableWorkers(task, workers)
	if len(capable) == 0 {
		return nil, ErrNoCapableWorker
	}

	lb.mu.RLock()
	defer lb.mu.RUnlock()

	var selected *WorkerAgent
	minLoad := int(^uint(0) >> 1) // Max int

	for _, worker := range capable {
		load := lb.taskCounts[worker.metadata.AgentID]
		if load < minLoad {
			minLoad = load
			selected = worker
		}
	}

	if selected == nil {
		selected = capable[0]
	}

	return selected, nil
}

func (lb *LeastLoadedBalancer) RecordResult(workerID string, task *Task, duration time.Duration, err error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if err == nil {
		lb.taskCounts[workerID]++
	}
}

func (lb *LeastLoadedBalancer) GetStats() map[string]interface{} {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	// Create copy for thread safety
	counts := make(map[string]int)
	for k, v := range lb.taskCounts {
		counts[k] = v
	}

	return map[string]interface{}{
		"strategy":    "least_loaded",
		"task_counts": counts,
	}
}

// ===== Random Balancer =====

type RandomBalancer struct {
	logger *observability.Logger
	rng    *rand.Rand
	mu     sync.Mutex
}

func NewRandomBalancer() *RandomBalancer {
	return &RandomBalancer{
		logger: observability.GetLogger(),
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (rb *RandomBalancer) SelectWorker(ctx context.Context, task *Task, workers []*WorkerAgent) (*WorkerAgent, error) {
	if len(workers) == 0 {
		return nil, ErrNoWorkerAvailable
	}

	capable := filterCapableWorkers(task, workers)
	if len(capable) == 0 {
		return nil, ErrNoCapableWorker
	}

	rb.mu.Lock()
	idx := rb.rng.Intn(len(capable))
	rb.mu.Unlock()

	return capable[idx], nil
}

func (rb *RandomBalancer) RecordResult(workerID string, task *Task, duration time.Duration, err error) {
	// No-op for random
}

func (rb *RandomBalancer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"strategy": "random",
	}
}

// ===== Capability-Based Balancer =====

type CapabilityBalancer struct {
	mu      sync.RWMutex
	weights map[string]map[string]float64 // taskType -> workerID -> weight
	config  *LoadBalancerConfig
	logger  *observability.Logger
}

func NewCapabilityBalancer(config *LoadBalancerConfig) *CapabilityBalancer {
	if config == nil {
		config = DefaultLoadBalancerConfig()
	}

	return &CapabilityBalancer{
		weights: make(map[string]map[string]float64),
		config:  config,
		logger:  observability.GetLogger(),
	}
}

func (cb *CapabilityBalancer) SelectWorker(ctx context.Context, task *Task, workers []*WorkerAgent) (*WorkerAgent, error) {
	if len(workers) == 0 {
		return nil, ErrNoWorkerAvailable
	}

	capable := filterCapableWorkers(task, workers)
	if len(capable) == 0 {
		return nil, ErrNoCapableWorker
	}

	cb.mu.RLock()
	defer cb.mu.RUnlock()

	var selected *WorkerAgent
	maxWeight := 0.0

	for _, worker := range capable {
		weight := cb.getWeight(task.Type, worker)
		if weight > maxWeight {
			maxWeight = weight
			selected = worker
		}
	}

	if selected == nil {
		selected = capable[0]
	}

	return selected, nil
}

func (cb *CapabilityBalancer) getWeight(taskType string, worker *WorkerAgent) float64 {
	baseWeight := 1.0

	// Exact capability match
	for _, cap := range worker.metadata.Capabilities {
		if cap == taskType {
			baseWeight *= 2.0
			break
		}
	}

	// Historical performance
	if weights, ok := cb.weights[taskType]; ok {
		if w, ok := weights[worker.metadata.AgentID]; ok {
			baseWeight *= w
		}
	}

	// Current status penalty
	if worker.metadata.Status == StatusBusy {
		baseWeight *= 0.5
	}

	return baseWeight
}

func (cb *CapabilityBalancer) RecordResult(workerID string, task *Task, duration time.Duration, err error) {
	if !cb.config.EnablePerformanceTracking {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.weights[task.Type] == nil {
		cb.weights[task.Type] = make(map[string]float64)
	}

	// Calculate weight based on performance
	// Lower duration = higher weight
	// Errors reduce weight
	weight := 1.0
	if err == nil {
		// Success increases weight
		weight = 1.2
		// Fast execution gets bonus
		if duration < 5*time.Second {
			weight = 1.5
		}
	} else {
		// Errors decrease weight
		weight = 0.5
	}

	// Apply decay to existing weight and add new
	existingWeight := cb.weights[task.Type][workerID]
	cb.weights[task.Type][workerID] = existingWeight*cb.config.WeightDecayFactor + weight*(1-cb.config.WeightDecayFactor)
}

func (cb *CapabilityBalancer) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	// Create copy
	weights := make(map[string]map[string]float64)
	for taskType, workerWeights := range cb.weights {
		weights[taskType] = make(map[string]float64)
		for workerID, weight := range workerWeights {
			weights[taskType][workerID] = weight
		}
	}

	return map[string]interface{}{
		"strategy": "capability_based",
		"weights":  weights,
	}
}

// ===== Latency-Based Balancer =====

type LatencyBasedBalancer struct {
	mu              sync.RWMutex
	latencyHistory  map[string][]time.Duration // workerID -> latencies
	config          *LoadBalancerConfig
	logger          *observability.Logger
}

func NewLatencyBasedBalancer(config *LoadBalancerConfig) *LatencyBasedBalancer {
	if config == nil {
		config = DefaultLoadBalancerConfig()
	}

	return &LatencyBasedBalancer{
		latencyHistory: make(map[string][]time.Duration),
		config:         config,
		logger:         observability.GetLogger(),
	}
}

func (lb *LatencyBasedBalancer) SelectWorker(ctx context.Context, task *Task, workers []*WorkerAgent) (*WorkerAgent, error) {
	if len(workers) == 0 {
		return nil, ErrNoWorkerAvailable
	}

	capable := filterCapableWorkers(task, workers)
	if len(capable) == 0 {
		return nil, ErrNoCapableWorker
	}

	lb.mu.RLock()
	defer lb.mu.RUnlock()

	var selected *WorkerAgent
	minAvgLatency := time.Duration(^uint64(0) >> 1) // Max duration

	for _, worker := range capable {
		avgLatency := lb.getAverageLatency(worker.metadata.AgentID)
		if avgLatency < minAvgLatency {
			minAvgLatency = avgLatency
			selected = worker
		}
	}

	if selected == nil {
		selected = capable[0]
	}

	return selected, nil
}

func (lb *LatencyBasedBalancer) getAverageLatency(workerID string) time.Duration {
	latencies, ok := lb.latencyHistory[workerID]
	if !ok || len(latencies) == 0 {
		return 5 * time.Second // Default for new workers
	}

	total := time.Duration(0)
	for _, lat := range latencies {
		total += lat
	}

	return total / time.Duration(len(latencies))
}

func (lb *LatencyBasedBalancer) RecordResult(workerID string, task *Task, duration time.Duration, err error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if lb.latencyHistory[workerID] == nil {
		lb.latencyHistory[workerID] = make([]time.Duration, 0, lb.config.TrackingWindowSize)
	}

	// Add new latency
	lb.latencyHistory[workerID] = append(lb.latencyHistory[workerID], duration)

	// Keep only recent history
	if len(lb.latencyHistory[workerID]) > lb.config.TrackingWindowSize {
		lb.latencyHistory[workerID] = lb.latencyHistory[workerID][1:]
	}
}

func (lb *LatencyBasedBalancer) GetStats() map[string]interface{} {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	avgLatencies := make(map[string]time.Duration)
	for workerID := range lb.latencyHistory {
		avgLatencies[workerID] = lb.getAverageLatency(workerID)
	}

	return map[string]interface{}{
		"strategy":       "latency_based",
		"avg_latencies":  avgLatencies,
		"window_size":    lb.config.TrackingWindowSize,
	}
}

// ===== Weighted Round Robin Balancer =====

type WeightedRoundRobinBalancer struct {
	mu           sync.Mutex
	weights      map[string]int // workerID -> weight
	currentIndex int
	currentWeight int
	logger       *observability.Logger
}

func NewWeightedRoundRobinBalancer() *WeightedRoundRobinBalancer {
	return &WeightedRoundRobinBalancer{
		weights: make(map[string]int),
		logger:  observability.GetLogger(),
	}
}

func (wrr *WeightedRoundRobinBalancer) SelectWorker(ctx context.Context, task *Task, workers []*WorkerAgent) (*WorkerAgent, error) {
	if len(workers) == 0 {
		return nil, ErrNoWorkerAvailable
	}

	capable := filterCapableWorkers(task, workers)
	if len(capable) == 0 {
		return nil, ErrNoCapableWorker
	}

	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	// Initialize weights if needed
	for _, worker := range capable {
		if _, ok := wrr.weights[worker.metadata.AgentID]; !ok {
			wrr.weights[worker.metadata.AgentID] = 1
		}
	}

	// Weighted round robin selection
	// Simple implementation - could be optimized
	return capable[wrr.currentIndex%len(capable)], nil
}

func (wrr *WeightedRoundRobinBalancer) RecordResult(workerID string, task *Task, duration time.Duration, err error) {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	// Adjust weight based on performance
	if err == nil && duration < 5*time.Second {
		wrr.weights[workerID] += 1
	} else if err != nil {
		wrr.weights[workerID] = max(1, wrr.weights[workerID]-1)
	}
}

func (wrr *WeightedRoundRobinBalancer) GetStats() map[string]interface{} {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	weights := make(map[string]int)
	for k, v := range wrr.weights {
		weights[k] = v
	}

	return map[string]interface{}{
		"strategy": "weighted_round_robin",
		"weights":  weights,
	}
}

// ===== Helper Functions =====

// filterCapableWorkers filters workers that can handle the task
func filterCapableWorkers(task *Task, workers []*WorkerAgent) []*WorkerAgent {
	var capable []*WorkerAgent

	for _, worker := range workers {
		// Skip if worker is not idle
		if worker.metadata.Status != StatusIdle {
			continue
		}

		// Check if worker has required capability
		hasCapability := false
		for _, cap := range worker.metadata.Capabilities {
			if cap == task.Type || cap == "general" {
				hasCapability = true
				break
			}
		}

		if hasCapability {
			capable = append(capable, worker)
		}
	}

	return capable
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
