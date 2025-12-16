package storage

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/agentql/agentql/pkg/minion/models"
)

var (
	ErrAgentNotFound    = errors.New("agent not found")
	ErrMetricsNotFound  = errors.New("metrics not found")
	ErrActivityNotFound = errors.New("activity not found")
)

// InMemoryStore is a thread-safe in-memory storage implementation
type InMemoryStore struct {
	agents     map[string]*models.Agent
	metrics    map[string]*models.Metrics
	activities map[string][]*models.Activity
	mu         sync.RWMutex
}

// NewInMemory creates a new in-memory store
func NewInMemory() *InMemoryStore {
	return &InMemoryStore{
		agents:     make(map[string]*models.Agent),
		metrics:    make(map[string]*models.Metrics),
		activities: make(map[string][]*models.Activity),
	}
}

// Agent CRUD operations

func (s *InMemoryStore) Create(ctx context.Context, agent *models.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.agents[agent.ID]; exists {
		return errors.New("agent already exists")
	}

	s.agents[agent.ID] = agent
	return nil
}

func (s *InMemoryStore) Get(ctx context.Context, id string) (*models.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent, exists := s.agents[id]
	if !exists {
		return nil, ErrAgentNotFound
	}

	return agent, nil
}

func (s *InMemoryStore) Update(ctx context.Context, agent *models.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.agents[agent.ID]; !exists {
		return ErrAgentNotFound
	}

	s.agents[agent.ID] = agent
	return nil
}

func (s *InMemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.agents[id]; !exists {
		return ErrAgentNotFound
	}

	delete(s.agents, id)
	delete(s.metrics, id)
	delete(s.activities, id)

	return nil
}

func (s *InMemoryStore) List(ctx context.Context, filter *models.ListAgentsRequest) ([]*models.Agent, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Filter agents
	var filtered []*models.Agent
	for _, agent := range s.agents {
		if s.matchesFilter(agent, filter) {
			filtered = append(filtered, agent)
		}
	}

	total := len(filtered)

	// Apply pagination
	page := filter.Page
	if page == 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize == 0 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= total {
		return []*models.Agent{}, total, nil
	}

	if end > total {
		end = total
	}

	return filtered[start:end], total, nil
}

func (s *InMemoryStore) FindByBehaviorType(ctx context.Context, behaviorType string) ([]*models.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*models.Agent
	for _, agent := range s.agents {
		if agent.BehaviorType == behaviorType {
			result = append(result, agent)
		}
	}

	return result, nil
}

func (s *InMemoryStore) FindByStatus(ctx context.Context, status models.AgentStatus) ([]*models.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*models.Agent
	for _, agent := range s.agents {
		if agent.Status == status {
			result = append(result, agent)
		}
	}

	return result, nil
}

// Metrics operations

func (s *InMemoryStore) GetMetrics(ctx context.Context, agentID string) (*models.Metrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics, exists := s.metrics[agentID]
	if !exists {
		return nil, ErrMetricsNotFound
	}

	return metrics, nil
}

func (s *InMemoryStore) UpdateMetrics(ctx context.Context, metrics *models.Metrics) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metrics[metrics.AgentID] = metrics
	return nil
}

func (s *InMemoryStore) CreateMetrics(ctx context.Context, metrics *models.Metrics) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.metrics[metrics.AgentID]; exists {
		return errors.New("metrics already exist")
	}

	s.metrics[metrics.AgentID] = metrics
	return nil
}

// Activity operations

func (s *InMemoryStore) RecordActivity(ctx context.Context, activity *models.Activity) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.activities[activity.AgentID] = append(s.activities[activity.AgentID], activity)
	return nil
}

func (s *InMemoryStore) GetActivities(ctx context.Context, agentID string, limit int) ([]*models.Activity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	activities, exists := s.activities[agentID]
	if !exists {
		return []*models.Activity{}, nil
	}

	// Return most recent activities
	start := len(activities) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*models.Activity, len(activities)-start)
	copy(result, activities[start:])

	// Reverse to get most recent first
	for i := 0; i < len(result)/2; i++ {
		j := len(result) - i - 1
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}

func (s *InMemoryStore) GetActivityByID(ctx context.Context, id string) (*models.Activity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, activities := range s.activities {
		for _, activity := range activities {
			if activity.ID == id {
				return activity, nil
			}
		}
	}

	return nil, ErrActivityNotFound
}

// Transaction operations (no-op for in-memory)

func (s *InMemoryStore) Begin(ctx context.Context) (Transaction, error) {
	return &inMemoryTransaction{store: s}, nil
}

func (s *InMemoryStore) Close() error {
	return nil
}

// Helper methods

func (s *InMemoryStore) matchesFilter(agent *models.Agent, filter *models.ListAgentsRequest) bool {
	if filter.BehaviorType != nil && agent.BehaviorType != *filter.BehaviorType {
		return false
	}

	if filter.Status != nil && agent.Status != *filter.Status {
		return false
	}

	if filter.Search != "" {
		search := strings.ToLower(filter.Search)
		name := strings.ToLower(agent.Name)
		desc := strings.ToLower(agent.Description)

		if !strings.Contains(name, search) && !strings.Contains(desc, search) {
			return false
		}
	}

	return true
}

// inMemoryTransaction implements Transaction for in-memory store
type inMemoryTransaction struct {
	store *InMemoryStore
}

func (t *inMemoryTransaction) Create(ctx context.Context, agent *models.Agent) error {
	return t.store.Create(ctx, agent)
}

func (t *inMemoryTransaction) Get(ctx context.Context, id string) (*models.Agent, error) {
	return t.store.Get(ctx, id)
}

func (t *inMemoryTransaction) Update(ctx context.Context, agent *models.Agent) error {
	return t.store.Update(ctx, agent)
}

func (t *inMemoryTransaction) Delete(ctx context.Context, id string) error {
	return t.store.Delete(ctx, id)
}

func (t *inMemoryTransaction) List(ctx context.Context, filter *models.ListAgentsRequest) ([]*models.Agent, int, error) {
	return t.store.List(ctx, filter)
}

func (t *inMemoryTransaction) FindByBehaviorType(ctx context.Context, behaviorType string) ([]*models.Agent, error) {
	return t.store.FindByBehaviorType(ctx, behaviorType)
}

func (t *inMemoryTransaction) FindByStatus(ctx context.Context, status models.AgentStatus) ([]*models.Agent, error) {
	return t.store.FindByStatus(ctx, status)
}

func (t *inMemoryTransaction) GetMetrics(ctx context.Context, agentID string) (*models.Metrics, error) {
	return t.store.GetMetrics(ctx, agentID)
}

func (t *inMemoryTransaction) UpdateMetrics(ctx context.Context, metrics *models.Metrics) error {
	return t.store.UpdateMetrics(ctx, metrics)
}

func (t *inMemoryTransaction) CreateMetrics(ctx context.Context, metrics *models.Metrics) error {
	return t.store.CreateMetrics(ctx, metrics)
}

func (t *inMemoryTransaction) RecordActivity(ctx context.Context, activity *models.Activity) error {
	return t.store.RecordActivity(ctx, activity)
}

func (t *inMemoryTransaction) GetActivities(ctx context.Context, agentID string, limit int) ([]*models.Activity, error) {
	return t.store.GetActivities(ctx, agentID, limit)
}

func (t *inMemoryTransaction) GetActivityByID(ctx context.Context, id string) (*models.Activity, error) {
	return t.store.GetActivityByID(ctx, id)
}

func (t *inMemoryTransaction) Commit() error {
	return nil // No-op for in-memory
}

func (t *inMemoryTransaction) Rollback() error {
	return nil // No-op for in-memory
}
