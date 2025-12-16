package tools

import (
	"context"

	"github.com/yourusername/minion/models"
)

// Tool defines the interface for agent tools
type Tool interface {
	// Name returns the tool name
	Name() string

	// Description returns what this tool does
	Description() string

	// Execute runs the tool
	Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error)

	// CanExecute checks if this tool can execute for the given agent
	CanExecute(agent *models.Agent) bool
}

// Registry manages tool registration and execution
type Registry interface {
	// Register adds a tool to the registry
	Register(tool Tool) error

	// Get retrieves a tool by name
	Get(name string) (Tool, error)

	// GetToolsForAgent returns tools available for an agent
	GetToolsForAgent(agent *models.Agent) []Tool

	// Execute runs a tool by name
	Execute(ctx context.Context, toolName string, input *models.ToolInput) (*models.ToolOutput, error)

	// List returns all registered tool names
	List() []string

	// Count returns the number of registered tools
	Count() int
}
