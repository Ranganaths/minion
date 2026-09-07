package swarm

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type swarmRegistry struct {
	mu     sync.RWMutex
	swarms map[SwarmID]Swarm
	config *RegistryConfig
	logger Logger
}

type RegistryConfig struct {
	MaxSwarms       int
	DefaultTags     []Tag
	EnableDiscovery bool
}

func DefaultRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		MaxSwarms:       100,
		EnableDiscovery: true,
	}
}

func NewSwarmRegistry(config *RegistryConfig) SwarmRegistry {
	if config == nil {
		config = DefaultRegistryConfig()
	}

	return &swarmRegistry{
		swarms: make(map[SwarmID]Swarm),
		config:  config,
		logger:  defaultLogger,
	}
}

func (r *swarmRegistry) CreateSwarm(ctx context.Context, config *SwarmConfig, options ...SwarmOption) (Swarm, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.swarms) >= r.config.MaxSwarms {
		return nil, fmt.Errorf("registry full: max %d swarms allowed", r.config.MaxSwarms)
	}

	if config == nil {
		config = DefaultSwarmConfig()
	}

	if config.ID == "" {
		config.ID = SwarmID(uuid.New().String())
	}

	if r.config.EnableDiscovery && len(r.config.DefaultTags) > 0 && len(config.Tags) == 0 {
		config.Tags = r.config.DefaultTags
	}

	swarm, err := NewSwarm(ctx, config, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create swarm: %w", err)
	}

	r.swarms[config.ID] = swarm

	r.logger.Info("Swarm created",
		String("swarm_id", string(config.ID)),
		String("swarm_name", config.Name),
	)

	return swarm, nil
}

func (r *swarmRegistry) GetSwarm(swarmID SwarmID) (Swarm, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	swarm, exists := r.swarms[swarmID]
	return swarm, exists
}

func (r *swarmRegistry) DeleteSwarm(ctx context.Context, swarmID SwarmID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	swarm, exists := r.swarms[swarmID]
	if !exists {
		return fmt.Errorf("swarm not found: %s", swarmID)
	}

	if err := swarm.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown swarm: %w", err)
	}

	delete(r.swarms, swarmID)

	r.logger.Info("Swarm deleted",
		String("swarm_id", string(swarmID)),
	)

	return nil
}

func (r *swarmRegistry) ListSwarms() []Swarm {
	r.mu.RLock()
	defer r.mu.RUnlock()

	swarms := make([]Swarm, 0, len(r.swarms))
	for _, swarm := range r.swarms {
		swarms = append(swarms, swarm)
	}
	return swarms
}

func (r *swarmRegistry) FindSwarmsByTag(tag Tag) []Swarm {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []Swarm
	for _, swarm := range r.swarms {
		for _, t := range swarm.Tags() {
			if t == tag {
				results = append(results, swarm)
				break
			}
		}
	}
	return results
}

func (r *swarmRegistry) FindSwarmsByTags(tags ...Tag) []Swarm {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []Swarm
	for _, swarm := range r.swarms {
		if hasAllTags(swarm.Tags(), tags) {
			results = append(results, swarm)
		}
	}
	return results
}

func (r *swarmRegistry) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastErr error
	for swarmID, swarm := range r.swarms {
		if err := swarm.Shutdown(ctx); err != nil {
			lastErr = fmt.Errorf("failed to shutdown swarm %s: %w", swarmID, err)
		}
	}

	r.swarms = make(map[SwarmID]Swarm)

	return lastErr
}

func hasAllTags(swarmTags, required []Tag) bool {
	if len(required) == 0 {
		return true
	}

	tagSet := make(map[Tag]bool)
	for _, t := range swarmTags {
		tagSet[t] = true
	}

	for _, t := range required {
		if !tagSet[t] {
			return false
		}
	}
	return true
}

type SwarmManager interface {
	CreateSwarm(ctx context.Context, name string, tags []Tag) (Swarm, error)
	GetOrCreateSwarm(ctx context.Context, name string, tags []Tag) (Swarm, error)
	JoinSwarm(ctx context.Context, swarmID SwarmID, agent Agent) error
	LeaveSwarm(ctx context.Context, swarmID SwarmID, agentID AgentID) error
	FindSwarms(ctx context.Context, tags ...Tag) []Swarm
	ExecuteAcrossSwarms(ctx context.Context, task *Task, tags ...Tag) ([]*AggregatedResult, error)
}

type swarmManager struct {
	registry  SwarmRegistry
	swarmByName map[string]SwarmID
	mu         sync.RWMutex
}

func NewSwarmManager(registry SwarmRegistry) SwarmManager {
	return &swarmManager{
		registry:    registry,
		swarmByName: make(map[string]SwarmID),
	}
}

func (m *swarmManager) CreateSwarm(ctx context.Context, name string, tags []Tag) (Swarm, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if swarmID, exists := m.swarmByName[name]; exists {
		if swarm, ok := m.registry.GetSwarm(swarmID); ok {
			return swarm, nil
		}
	}

	config := DefaultSwarmConfig()
	config.Name = name
	config.Tags = tags

	swarm, err := m.registry.CreateSwarm(ctx, config)
	if err != nil {
		return nil, err
	}

	m.swarmByName[name] = swarm.ID()

	return swarm, nil
}

func (m *swarmManager) GetOrCreateSwarm(ctx context.Context, name string, tags []Tag) (Swarm, error) {
	m.mu.RLock()
	if swarmID, exists := m.swarmByName[name]; exists {
		if swarm, ok := m.registry.GetSwarm(swarmID); ok {
			m.mu.RUnlock()
			return swarm, nil
		}
	}
	m.mu.RUnlock()

	return m.CreateSwarm(ctx, name, tags)
}

func (m *swarmManager) JoinSwarm(ctx context.Context, swarmID SwarmID, agent Agent) error {
	swarm, ok := m.registry.GetSwarm(swarmID)
	if !ok {
		return fmt.Errorf("swarm not found: %s", swarmID)
	}
	return swarm.RegisterAgent(ctx, agent)
}

func (m *swarmManager) LeaveSwarm(ctx context.Context, swarmID SwarmID, agentID AgentID) error {
	swarm, ok := m.registry.GetSwarm(swarmID)
	if !ok {
		return fmt.Errorf("swarm not found: %s", swarmID)
	}
	return swarm.UnregisterAgent(ctx, agentID)
}

func (m *swarmManager) FindSwarms(ctx context.Context, tags ...Tag) []Swarm {
	return m.registry.FindSwarmsByTags(tags...)
}

func (m *swarmManager) ExecuteAcrossSwarms(ctx context.Context, task *Task, tags ...Tag) ([]*AggregatedResult, error) {
	swarms := m.FindSwarms(ctx, tags...)
	if len(swarms) == 0 {
		return nil, ErrNoEligibleAgents
	}

	var results []*AggregatedResult
	for _, s := range swarms {
		result, err := s.Execute(ctx, task)
		if err != nil {
			continue
		}
		results = append(results, result)
	}

	return results, nil
}
