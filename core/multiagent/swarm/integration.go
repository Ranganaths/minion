package swarm

import (
	"context"
	"sync"
	"time"

	"github.com/Ranganaths/minion/observability"
)

type metricsCollector struct {
	metrics *observability.MetricsCollector
	mu      sync.RWMutex
	counters map[string]int64
	gauges   map[string]int64
}

func NewMetricsCollector() *metricsCollector {
	return &metricsCollector{
		counters: make(map[string]int64),
		gauges:   make(map[string]int64),
	}
}

func (m *metricsCollector) RecordSwarmTask(ctx context.Context, swarmID, taskType, status string, duration time.Duration) {
	key := "swarm.task." + status
	m.increment(key)

	statusKey := "swarm.task." + taskType + "." + status
	m.increment(statusKey)

	durationKey := "swarm.task.duration." + status
	m.observeDuration(durationKey, duration)
}

func (m *metricsCollector) RecordSwarmAgentEvent(ctx context.Context, swarmID, agentID, event string) {
	key := "swarm.agent." + event
	m.increment(key)

	swarmKey := "swarm." + swarmID + ".agent." + event
	m.increment(swarmKey)
}

func (m *metricsCollector) increment(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[key]++
}

func (m *metricsCollector) observeDuration(key string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
}

func (m *metricsCollector) GetCounter(name string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.counters[name]
}

func (m *metricsCollector) GetGauge(name string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.gauges[name]
}

type CoordinatorIntegration struct {
	manager       SwarmManager
	swarmRegistry SwarmRegistry
}

func NewCoordinatorIntegration(swarmManager SwarmManager) *CoordinatorIntegration {
	return &CoordinatorIntegration{
		manager:       swarmManager,
		swarmRegistry: nil,
	}
}

func (c *CoordinatorIntegration) CreateSwarmForTask(ctx context.Context, name string, tags []Tag) (Swarm, error) {
	return c.manager.CreateSwarm(ctx, name, tags)
}

func (c *CoordinatorIntegration) RegisterWorkerAsSwarmAgent(ctx context.Context, swarmID SwarmID, workerID string, handler TaskHandler) error {
	agent := AgentFromHandler(handler)
	return c.manager.JoinSwarm(ctx, swarmID, agent)
}

func (c *CoordinatorIntegration) ExecuteInSwarm(ctx context.Context, swarmID SwarmID, task *Task, strategy AggregationStrategy) (*AggregatedResult, error) {
	swarm, ok := c.swarmRegistry.GetSwarm(swarmID)
	if !ok {
		return nil, ErrAgentNotFound
	}

	if strategy != "" {
		return swarm.ExecuteWithStrategy(ctx, task, strategy)
	}

	return swarm.Execute(ctx, task)
}

func (c *CoordinatorIntegration) FindSwarmsByTags(ctx context.Context, tags ...Tag) []Swarm {
	return c.manager.FindSwarms(ctx, tags...)
}

type SwarmPoolConfig struct {
	MaxConcurrentSwarms int
	MinAgentsPerSwarm   int
	MaxAgentsPerSwarm   int
	TaskTimeout         time.Duration
	AutoCreate         bool
	TagPrefix          string
}

func DefaultSwarmPoolConfig() *SwarmPoolConfig {
	return &SwarmPoolConfig{
		MaxConcurrentSwarms: 10,
		MinAgentsPerSwarm:   1,
		MaxAgentsPerSwarm:   10,
		TaskTimeout:         30 * time.Second,
		AutoCreate:         true,
		TagPrefix:          "pool:",
	}
}

type SwarmPool struct {
	config     *SwarmPoolConfig
	manager    SwarmManager
	registry   SwarmRegistry
	mu         sync.RWMutex
	swarms     map[string]*SwarmPoolItem
	handlers   map[string]TaskHandler
}

type SwarmPoolItem struct {
	Swarm     Swarm
	Handler   TaskHandler
	CreatedAt time.Time
	Tasks     int64
}

func NewSwarmPool(config *SwarmPoolConfig, registry SwarmRegistry) *SwarmPool {
	if config == nil {
		config = DefaultSwarmPoolConfig()
	}

	return &SwarmPool{
		config:   config,
		manager:  NewSwarmManager(registry),
		registry: registry,
		swarms:   make(map[string]*SwarmPoolItem),
		handlers: make(map[string]TaskHandler),
	}
}

func (p *SwarmPool) RegisterHandler(ctx context.Context, name string, handler TaskHandler, tags []Tag) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.handlers[name] = handler

	if p.config.AutoCreate {
		swarm, err := p.manager.CreateSwarm(ctx, "pool-"+name, tags)
		if err != nil {
			return err
		}

		agent := AgentFromHandler(handler)
		if err := swarm.RegisterAgent(ctx, agent); err != nil {
			return err
		}

		p.swarms[name] = &SwarmPoolItem{
			Swarm:     swarm,
			Handler:   handler,
			CreatedAt: time.Now(),
		}
	}

	return nil
}

func (p *SwarmPool) GetSwarm(name string) (Swarm, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	item, exists := p.swarms[name]
	if exists {
		return item.Swarm, true
	}
	return nil, false
}

func (p *SwarmPool) Execute(ctx context.Context, handlerName string, task *Task, strategy AggregationStrategy) (*AggregatedResult, error) {
	p.mu.RLock()
	item, exists := p.swarms[handlerName]
	p.mu.RUnlock()

	if !exists {
		return nil, ErrAgentNotFound
	}

	item.Tasks++

	if strategy != "" {
		return item.Swarm.ExecuteWithStrategy(ctx, task, strategy)
	}

	return item.Swarm.Execute(ctx, task)
}

func (p *SwarmPool) ScaleUp(ctx context.Context, handlerName string, count int) error {
	p.mu.RLock()
	item, exists := p.swarms[handlerName]
	p.mu.RUnlock()

	if !exists {
		return ErrAgentNotFound
	}

	handler, ok := p.handlers[handlerName]
	if !ok {
		return ErrAgentNotFound
	}

	for i := 0; i < count; i++ {
		agent := AgentFromHandler(handler)
		if err := item.Swarm.RegisterAgent(ctx, agent); err != nil {
			return err
		}
	}

	return nil
}

func (p *SwarmPool) ScaleDown(ctx context.Context, handlerName string, count int) error {
	p.mu.RLock()
	item, exists := p.swarms[handlerName]
	p.mu.RUnlock()

	if !exists {
		return ErrAgentNotFound
	}

	agents := item.Swarm.ListAgents()
	removed := 0

	for _, agent := range agents {
		if removed >= count {
			break
		}
		if err := item.Swarm.UnregisterAgent(ctx, agent.ID()); err == nil {
			removed++
		}
	}

	return nil
}

func (p *SwarmPool) GetStats(handlerName string) *SwarmPoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	item, exists := p.swarms[handlerName]
	if !exists {
		return nil
	}

	stats := item.Swarm.Stats()
	return &SwarmPoolStats{
		SwarmID:        item.Swarm.ID(),
		SwarmName:      item.Swarm.Name(),
		TotalAgents:    stats.TotalAgents,
		ActiveAgents:   stats.ActiveAgents,
		TotalTasks:     stats.TotalTasks,
		TasksProcessed: item.Tasks,
		SuccessRate:    stats.SuccessRate,
		CreatedAt:      item.CreatedAt,
	}
}

type SwarmPoolStats struct {
	SwarmID        SwarmID
	SwarmName      string
	TotalAgents    int
	ActiveAgents   int
	TotalTasks     int64
	TasksProcessed int64
	SuccessRate    float64
	CreatedAt      time.Time
}
