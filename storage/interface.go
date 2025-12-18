package storage

import (
	"context"

	"github.com/Ranganaths/minion/models"
)

// AgentStore defines the interface for agent persistence
type AgentStore interface {
	// Agent CRUD
	Create(ctx context.Context, agent *models.Agent) error
	Get(ctx context.Context, id string) (*models.Agent, error)
	Update(ctx context.Context, agent *models.Agent) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter *models.ListAgentsRequest) ([]*models.Agent, int, error)

	// Find by criteria
	FindByBehaviorType(ctx context.Context, behaviorType string) ([]*models.Agent, error)
	FindByStatus(ctx context.Context, status models.AgentStatus) ([]*models.Agent, error)
}

// MetricsStore defines the interface for metrics persistence
type MetricsStore interface {
	// Metrics operations
	GetMetrics(ctx context.Context, agentID string) (*models.Metrics, error)
	UpdateMetrics(ctx context.Context, metrics *models.Metrics) error
	CreateMetrics(ctx context.Context, metrics *models.Metrics) error
}

// ActivityStore defines the interface for activity persistence
type ActivityStore interface {
	// Activity operations
	RecordActivity(ctx context.Context, activity *models.Activity) error
	GetActivities(ctx context.Context, agentID string, limit int) ([]*models.Activity, error)
	GetActivityByID(ctx context.Context, id string) (*models.Activity, error)
}

// Store combines all storage interfaces
type Store interface {
	AgentStore
	MetricsStore
	ActivityStore

	// Transaction support
	Begin(ctx context.Context) (Transaction, error)
	Close() error
}

// Transaction represents a storage transaction
type Transaction interface {
	AgentStore
	MetricsStore
	ActivityStore

	Commit() error
	Rollback() error
}
