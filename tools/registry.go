package tools

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Ranganaths/minion/models"
)

var (
	ErrToolNotFound      = errors.New("tool not found")
	ErrToolAlreadyExists = errors.New("tool already exists")
)

// InMemoryRegistry is a thread-safe in-memory tool registry
type InMemoryRegistry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewRegistry creates a new tool registry
func NewRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *InMemoryRegistry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("%w: %s", ErrToolAlreadyExists, name)
	}

	r.tools[name] = tool
	return nil
}

// Get retrieves a tool by name
func (r *InMemoryRegistry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}

	return tool, nil
}

// GetToolsForAgent returns tools available for an agent
func (r *InMemoryRegistry) GetToolsForAgent(agent *models.Agent) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var availableTools []Tool
	for _, tool := range r.tools {
		if tool.CanExecute(agent) {
			availableTools = append(availableTools, tool)
		}
	}

	return availableTools
}

// Execute runs a tool by name
func (r *InMemoryRegistry) Execute(ctx context.Context, toolName string, input *models.ToolInput) (*models.ToolOutput, error) {
	tool, err := r.Get(toolName)
	if err != nil {
		return nil, err
	}

	return tool.Execute(ctx, input)
}

// List returns all registered tool names
func (r *InMemoryRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}

	return names
}

// Count returns the number of registered tools
func (r *InMemoryRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tools)
}
