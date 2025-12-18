package core

import (
	"context"

	"github.com/yourusername/minion/models"
)

// AgentExecutor defines the interface for agent execution
type AgentExecutor interface {
	Execute(ctx context.Context, input *models.Input) (*models.Output, error)
}

// Behavior defines how an agent processes input and output
type Behavior interface {
	// GetSystemPrompt generates the system prompt for the agent
	GetSystemPrompt(agent *models.Agent) string

	// ProcessInput prepares input before execution
	ProcessInput(ctx context.Context, agent *models.Agent, input *models.Input) (*models.ProcessedInput, error)

	// ProcessOutput enhances output after execution
	ProcessOutput(ctx context.Context, agent *models.Agent, output *models.Output) (*models.ProcessedOutput, error)
}

// BehaviorRegistry manages different behavior implementations
type BehaviorRegistry interface {
	Register(behaviorType string, behavior Behavior) error
	Get(behaviorType string) (Behavior, error)
	List() []string
}

// AgentRegistry manages agent lifecycle
type AgentRegistry interface {
	// CRUD operations
	Create(ctx context.Context, req *models.CreateAgentRequest) (*models.Agent, error)
	Get(ctx context.Context, id string) (*models.Agent, error)
	Update(ctx context.Context, id string, req *models.UpdateAgentRequest) (*models.Agent, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, req *models.ListAgentsRequest) (*models.ListAgentsResponse, error)

	// Metrics and activity
	GetMetrics(ctx context.Context, agentID string) (*models.Metrics, error)
	RecordActivity(ctx context.Context, activity *models.Activity) error
	GetActivities(ctx context.Context, agentID string, limit int) ([]*models.Activity, error)
}

// Framework is the main entry point for the agent framework
type Framework interface {
	// Agent operations
	CreateAgent(ctx context.Context, req *models.CreateAgentRequest) (*models.Agent, error)
	GetAgent(ctx context.Context, id string) (*models.Agent, error)
	UpdateAgent(ctx context.Context, id string, req *models.UpdateAgentRequest) (*models.Agent, error)
	DeleteAgent(ctx context.Context, id string) error
	ListAgents(ctx context.Context, req *models.ListAgentsRequest) (*models.ListAgentsResponse, error)

	// Behavior operations
	RegisterBehavior(behaviorType string, behavior Behavior) error
	GetBehavior(behaviorType string) (Behavior, error)

	// Tool operations (defined in tools package)
	RegisterTool(tool interface{}) error
	GetToolsForAgent(agent *models.Agent) []interface{}

	// MCP operations (Model Context Protocol)
	ConnectMCPServer(ctx context.Context, config interface{}) error
	DisconnectMCPServer(serverName string) error
	ListMCPServers() []string
	GetMCPServerStatus() map[string]interface{}
	RefreshMCPTools(ctx context.Context, serverName string) error

	// Execution
	Execute(ctx context.Context, agentID string, input *models.Input) (*models.Output, error)

	// Metrics
	GetMetrics(ctx context.Context, agentID string) (*models.Metrics, error)
	GetActivities(ctx context.Context, agentID string, limit int) ([]*models.Activity, error)
}
