package bridge

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/yourusername/minion/mcp/client"
)

// ToolRegistrar is the minimal interface needed for tool registration
// This avoids circular dependency with core package
type ToolRegistrar interface {
	RegisterTool(tool interface{}) error
}

// BridgeRegistry manages MCP tool wrappers
type BridgeRegistry struct {
	clientManager *client.MCPClientManager
	registrar     ToolRegistrar

	// Track wrapped tools
	wrappedTools map[string]*MCPToolWrapper // tool name → wrapper
	mu           sync.RWMutex
}

// NewBridgeRegistry creates a new bridge registry
func NewBridgeRegistry(
	clientManager *client.MCPClientManager,
	registrar ToolRegistrar,
) *BridgeRegistry {
	return &BridgeRegistry{
		clientManager: clientManager,
		registrar:     registrar,
		wrappedTools:  make(map[string]*MCPToolWrapper),
	}
}

// RegisterServerTools wraps and registers all tools from an MCP server
func (r *BridgeRegistry) RegisterServerTools(
	ctx context.Context,
	serverName string,
) error {
	// Get MCP client for server
	mcpClient, err := r.clientManager.GetClient(serverName)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Get tools from server
	tools := mcpClient.GetTools()
	if len(tools) == 0 {
		return fmt.Errorf("no tools found on server: %s", serverName)
	}

	// Wrap and register each tool
	registered := 0
	for _, mcpTool := range tools {
		wrapper := NewMCPToolWrapper(serverName, mcpTool, r.clientManager)

		// Register with tool registrar
		if err := r.registrar.RegisterTool(wrapper); err != nil {
			// Log error but continue with other tools
			continue
		}

		// Track wrapped tool
		r.mu.Lock()
		r.wrappedTools[wrapper.Name()] = wrapper
		r.mu.Unlock()

		registered++
	}

	if registered == 0 {
		return fmt.Errorf("failed to register any tools from server: %s", serverName)
	}

	return nil
}

// UnregisterServerTools removes all tools from a server
func (r *BridgeRegistry) UnregisterServerTools(serverName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find and remove all tools for this server
	prefix := fmt.Sprintf("mcp_%s_", serverName)
	removed := 0

	for toolName := range r.wrappedTools {
		if strings.HasPrefix(toolName, prefix) {
			delete(r.wrappedTools, toolName)
			removed++
			// Note: Can't unregister from framework as it doesn't have Unregister method
			// Tools remain registered but become unavailable when disconnected
		}
	}

	if removed == 0 {
		return fmt.Errorf("no tools found for server: %s", serverName)
	}

	return nil
}

// GetWrappedTool returns a wrapped tool by name
func (r *BridgeRegistry) GetWrappedTool(toolName string) (*MCPToolWrapper, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	wrapper, ok := r.wrappedTools[toolName]
	if !ok {
		return nil, fmt.Errorf("wrapped tool not found: %s", toolName)
	}

	return wrapper, nil
}

// ListWrappedTools returns all wrapped tool names
func (r *BridgeRegistry) ListWrappedTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]string, 0, len(r.wrappedTools))
	for name := range r.wrappedTools {
		tools = append(tools, name)
	}

	return tools
}

// ListToolsByServer returns tools from a specific server
func (r *BridgeRegistry) ListToolsByServer(serverName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	prefix := fmt.Sprintf("mcp_%s_", serverName)
	tools := make([]string, 0)

	for name := range r.wrappedTools {
		if strings.HasPrefix(name, prefix) {
			tools = append(tools, name)
		}
	}

	return tools
}

// GetServerForTool returns the server name for a given tool
func (r *BridgeRegistry) GetServerForTool(toolName string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	wrapper, ok := r.wrappedTools[toolName]
	if !ok {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	return wrapper.serverName, nil
}

// Count returns the number of wrapped tools
func (r *BridgeRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.wrappedTools)
}
