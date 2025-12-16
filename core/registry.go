package core

import (
	"context"

	"github.com/agentql/agentql/pkg/minion/models"
	"github.com/agentql/agentql/pkg/minion/storage"
)

// AgentRegistryImpl implements the AgentRegistry interface
type AgentRegistryImpl struct {
	store storage.Store
}

// NewAgentRegistry creates a new agent registry
func NewAgentRegistry(store storage.Store) *AgentRegistryImpl {
	return &AgentRegistryImpl{
		store: store,
	}
}

// Create creates a new agent
func (r *AgentRegistryImpl) Create(ctx context.Context, req *models.CreateAgentRequest) (*models.Agent, error) {
	// The registry delegates to the framework's CreateAgent method
	// This is a simplified registry that can be used without the full framework
	framework := NewFramework(WithStorage(r.store))
	return framework.CreateAgent(ctx, req)
}

// Get retrieves an agent by ID
func (r *AgentRegistryImpl) Get(ctx context.Context, id string) (*models.Agent, error) {
	return r.store.Get(ctx, id)
}

// Update updates an existing agent
func (r *AgentRegistryImpl) Update(ctx context.Context, id string, req *models.UpdateAgentRequest) (*models.Agent, error) {
	framework := NewFramework(WithStorage(r.store))
	return framework.UpdateAgent(ctx, id, req)
}

// Delete deletes an agent
func (r *AgentRegistryImpl) Delete(ctx context.Context, id string) error {
	return r.store.Delete(ctx, id)
}

// List returns a paginated list of agents
func (r *AgentRegistryImpl) List(ctx context.Context, req *models.ListAgentsRequest) (*models.ListAgentsResponse, error) {
	framework := NewFramework(WithStorage(r.store))
	return framework.ListAgents(ctx, req)
}

// GetMetrics retrieves metrics for an agent
func (r *AgentRegistryImpl) GetMetrics(ctx context.Context, agentID string) (*models.Metrics, error) {
	return r.store.GetMetrics(ctx, agentID)
}

// RecordActivity records an activity for an agent
func (r *AgentRegistryImpl) RecordActivity(ctx context.Context, activity *models.Activity) error {
	return r.store.RecordActivity(ctx, activity)
}

// GetActivities retrieves recent activities for an agent
func (r *AgentRegistryImpl) GetActivities(ctx context.Context, agentID string, limit int) ([]*models.Activity, error) {
	return r.store.GetActivities(ctx, agentID, limit)
}
