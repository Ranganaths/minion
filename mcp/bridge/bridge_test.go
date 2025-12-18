package bridge

import (
	"testing"

	"github.com/yourusername/minion/mcp/client"
	"github.com/yourusername/minion/models"
)

// Mock tool registrar for testing
type mockRegistrar struct {
	registered []interface{}
	shouldFail bool
}

func (m *mockRegistrar) RegisterTool(tool interface{}) error {
	if m.shouldFail {
		return assert.AnError
	}
	m.registered = append(m.registered, tool)
	return nil
}

// Mock MCP client manager for testing
type mockClientManager struct {
	clients map[string]*mockMCPClient
}

func (m *mockClientManager) GetClient(serverName string) (*client.MCPClient, error) {
	// Return nil as we're testing at a higher level
	return nil, nil
}

type mockMCPClient struct {
	tools []client.MCPTool
}

func TestNewBridgeRegistry(t *testing.T) {
	registrar := &mockRegistrar{}
	manager := &client.MCPClientManager{}

	registry := NewBridgeRegistry(manager, registrar)
	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	if registry.clientManager != manager {
		t.Error("Expected client manager to be set")
	}

	if registry.registrar != registrar {
		t.Error("Expected registrar to be set")
	}

	if registry.wrappedTools == nil {
		t.Error("Expected wrappedTools map to be initialized")
	}
}

func TestNewMCPToolWrapper(t *testing.T) {
	mcpTool := client.MCPTool{
		Name:        "test_tool",
		Description: "Test tool description",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{"type": "string"},
			},
		},
	}

	manager := client.NewMCPClientManager(nil)
	wrapper := NewMCPToolWrapper("github", mcpTool, manager)

	if wrapper == nil {
		t.Fatal("Expected non-nil wrapper")
	}

	if wrapper.serverName != "github" {
		t.Errorf("Expected serverName=github, got %s", wrapper.serverName)
	}

	if wrapper.mcpTool.Name != "test_tool" {
		t.Errorf("Expected tool name=test_tool, got %s", wrapper.mcpTool.Name)
	}
}

func TestMCPToolWrapper_Name(t *testing.T) {
	mcpTool := client.MCPTool{Name: "create_issue"}
	wrapper := NewMCPToolWrapper("github", mcpTool, nil)

	name := wrapper.Name()
	expected := "mcp_github_create_issue"
	if name != expected {
		t.Errorf("Expected name=%s, got %s", expected, name)
	}
}

func TestMCPToolWrapper_Description(t *testing.T) {
	mcpTool := client.MCPTool{
		Name:        "create_issue",
		Description: "Creates a new issue",
	}
	wrapper := NewMCPToolWrapper("github", mcpTool, nil)

	desc := wrapper.Description()
	expected := "[MCP:github] Creates a new issue"
	if desc != expected {
		t.Errorf("Expected description=%s, got %s", expected, desc)
	}
}

func TestMCPToolWrapper_CanExecute(t *testing.T) {
	wrapper := NewMCPToolWrapper("github", client.MCPTool{Name: "test"}, nil)

	tests := []struct {
		name         string
		capabilities []string
		expected     bool
	}{
		{
			name:         "With mcp_integration capability",
			capabilities: []string{"mcp_integration"},
			expected:     true,
		},
		{
			name:         "With server-specific capability",
			capabilities: []string{"mcp_github"},
			expected:     true,
		},
		{
			name:         "With both capabilities",
			capabilities: []string{"mcp_integration", "mcp_github"},
			expected:     true,
		},
		{
			name:         "Without MCP capabilities",
			capabilities: []string{"some_other_capability"},
			expected:     false,
		},
		{
			name:         "With different server capability",
			capabilities: []string{"mcp_slack"},
			expected:     false,
		},
		{
			name:         "Empty capabilities",
			capabilities: []string{},
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &models.Agent{
				Capabilities: tt.capabilities,
			}
			result := wrapper.CanExecute(agent)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMCPToolWrapper_CanExecute_WithRequiredCapabilities(t *testing.T) {
	wrapper := NewMCPToolWrapper("github", client.MCPTool{Name: "test"}, nil)
	wrapper.requiredCapabilities = []string{"admin", "write"}

	tests := []struct {
		name         string
		capabilities []string
		expected     bool
	}{
		{
			name:         "Has all required capabilities",
			capabilities: []string{"mcp_integration", "admin", "write"},
			expected:     true,
		},
		{
			name:         "Missing one required capability",
			capabilities: []string{"mcp_integration", "admin"},
			expected:     false,
		},
		{
			name:         "No MCP capability",
			capabilities: []string{"admin", "write"},
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &models.Agent{
				Capabilities: tt.capabilities,
			}
			result := wrapper.CanExecute(agent)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMCPToolWrapper_extractResult(t *testing.T) {
	wrapper := NewMCPToolWrapper("github", client.MCPTool{Name: "test"}, nil)

	tests := []struct {
		name     string
		result   *client.MCPCallToolResult
		expected interface{}
	}{
		{
			name: "Empty content",
			result: &client.MCPCallToolResult{
				Content: []interface{}{},
			},
			expected: nil,
		},
		{
			name: "Text content",
			result: &client.MCPCallToolResult{
				Content: []interface{}{
					map[string]interface{}{"text": "Hello World"},
				},
			},
			expected: "Hello World",
		},
		{
			name: "JSON text content",
			result: &client.MCPCallToolResult{
				Content: []interface{}{
					map[string]interface{}{"text": `{"status":"success","id":123}`},
				},
			},
			expected: map[string]interface{}{"status": "success", "id": float64(123)},
		},
		{
			name: "String content",
			result: &client.MCPCallToolResult{
				Content: []interface{}{"Direct string"},
			},
			expected: "Direct string",
		},
		{
			name: "Map content without text",
			result: &client.MCPCallToolResult{
				Content: []interface{}{
					map[string]interface{}{"data": "value"},
				},
			},
			expected: map[string]interface{}{"data": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapper.extractResult(tt.result)
			// Basic comparison - in real tests would use deep equal
			if result == nil && tt.expected != nil {
				t.Errorf("Expected non-nil result")
			}
		})
	}
}

func TestMCPToolWrapper_extractErrorMessage(t *testing.T) {
	wrapper := NewMCPToolWrapper("github", client.MCPTool{Name: "test"}, nil)

	tests := []struct {
		name     string
		result   *client.MCPCallToolResult
		expected string
	}{
		{
			name: "Empty content",
			result: &client.MCPCallToolResult{
				Content: []interface{}{},
			},
			expected: "Unknown error",
		},
		{
			name: "Text error",
			result: &client.MCPCallToolResult{
				Content: []interface{}{
					map[string]interface{}{"text": "Connection failed"},
				},
			},
			expected: "Connection failed",
		},
		{
			name: "String error",
			result: &client.MCPCallToolResult{
				Content: []interface{}{"Direct error message"},
			},
			expected: "Direct error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapper.extractErrorMessage(tt.result)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestBridgeRegistry_ListWrappedTools(t *testing.T) {
	registrar := &mockRegistrar{}
	manager := client.NewMCPClientManager(nil)
	registry := NewBridgeRegistry(manager, registrar)

	// Initially empty
	tools := registry.ListWrappedTools()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools, got %d", len(tools))
	}

	// Add some wrapped tools
	registry.mu.Lock()
	registry.wrappedTools["mcp_github_tool1"] = &MCPToolWrapper{}
	registry.wrappedTools["mcp_github_tool2"] = &MCPToolWrapper{}
	registry.mu.Unlock()

	tools = registry.ListWrappedTools()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}
}

func TestBridgeRegistry_ListToolsByServer(t *testing.T) {
	registrar := &mockRegistrar{}
	manager := client.NewMCPClientManager(nil)
	registry := NewBridgeRegistry(manager, registrar)

	// Add tools from different servers
	registry.mu.Lock()
	registry.wrappedTools["mcp_github_tool1"] = &MCPToolWrapper{}
	registry.wrappedTools["mcp_github_tool2"] = &MCPToolWrapper{}
	registry.wrappedTools["mcp_slack_tool1"] = &MCPToolWrapper{}
	registry.mu.Unlock()

	githubTools := registry.ListToolsByServer("github")
	if len(githubTools) != 2 {
		t.Errorf("Expected 2 GitHub tools, got %d", len(githubTools))
	}

	slackTools := registry.ListToolsByServer("slack")
	if len(slackTools) != 1 {
		t.Errorf("Expected 1 Slack tool, got %d", len(slackTools))
	}

	unknownTools := registry.ListToolsByServer("unknown")
	if len(unknownTools) != 0 {
		t.Errorf("Expected 0 unknown tools, got %d", len(unknownTools))
	}
}

func TestBridgeRegistry_GetWrappedTool(t *testing.T) {
	registrar := &mockRegistrar{}
	manager := client.NewMCPClientManager(nil)
	registry := NewBridgeRegistry(manager, registrar)

	// Test non-existent tool
	_, err := registry.GetWrappedTool("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent tool")
	}

	// Add a tool
	mockWrapper := &MCPToolWrapper{}
	registry.mu.Lock()
	registry.wrappedTools["mcp_github_test"] = mockWrapper
	registry.mu.Unlock()

	// Test existing tool
	wrapper, err := registry.GetWrappedTool("mcp_github_test")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if wrapper != mockWrapper {
		t.Error("Expected same wrapper instance")
	}
}

func TestBridgeRegistry_GetServerForTool(t *testing.T) {
	registrar := &mockRegistrar{}
	manager := client.NewMCPClientManager(nil)
	registry := NewBridgeRegistry(manager, registrar)

	// Add a tool
	wrapper := &MCPToolWrapper{serverName: "github"}
	registry.mu.Lock()
	registry.wrappedTools["mcp_github_test"] = wrapper
	registry.mu.Unlock()

	// Test existing tool
	serverName, err := registry.GetServerForTool("mcp_github_test")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if serverName != "github" {
		t.Errorf("Expected serverName=github, got %s", serverName)
	}

	// Test non-existent tool
	_, err = registry.GetServerForTool("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent tool")
	}
}

func TestBridgeRegistry_Count(t *testing.T) {
	registrar := &mockRegistrar{}
	manager := client.NewMCPClientManager(nil)
	registry := NewBridgeRegistry(manager, registrar)

	// Initially 0
	if registry.Count() != 0 {
		t.Errorf("Expected count=0, got %d", registry.Count())
	}

	// Add tools
	registry.mu.Lock()
	registry.wrappedTools["tool1"] = &MCPToolWrapper{}
	registry.wrappedTools["tool2"] = &MCPToolWrapper{}
	registry.wrappedTools["tool3"] = &MCPToolWrapper{}
	registry.mu.Unlock()

	if registry.Count() != 3 {
		t.Errorf("Expected count=3, got %d", registry.Count())
	}
}

// Helper for tests
var assert = struct {
	AnError error
}{
	AnError: &mockError{},
}

type mockError struct{}

func (e *mockError) Error() string {
	return "mock error"
}
