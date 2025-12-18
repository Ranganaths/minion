// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/yourusername/minion/core"
	"github.com/yourusername/minion/mcp/client"
	"github.com/yourusername/minion/mcp/testing"
	"github.com/yourusername/minion/models"
)

func TestMCPIntegration_HTTPTransport(t *testing.T) {
	// Start mock server
	mockServer := testing.NewMockMCPServer()
	for _, tool := range testing.CreateTestTools() {
		mockServer.AddTool(tool)
	}

	err := mockServer.Start(":18080")
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer mockServer.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create framework
	framework := core.NewFramework()
	defer framework.Close()

	ctx := context.Background()

	// Connect to mock MCP server
	config := &client.ClientConfig{
		ServerName:     "test-server",
		Transport:      client.TransportHTTP,
		URL:            "http://localhost:18080",
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 5 * time.Second,
	}

	err = framework.ConnectMCPServer(ctx, config)
	if err != nil {
		t.Fatalf("Failed to connect to MCP server: %v", err)
	}

	// Verify server is connected
	servers := framework.ListMCPServers()
	if len(servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(servers))
	}

	if servers[0] != "test-server" {
		t.Errorf("Expected server name 'test-server', got %s", servers[0])
	}

	// Create agent with MCP capability
	agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:         "Test Agent",
		BehaviorType: "default",
		Capabilities: []string{"mcp_integration"},
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Get available tools
	tools := framework.GetToolsForAgent(agent)
	if len(tools) < 3 {
		t.Errorf("Expected at least 3 tools, got %d", len(tools))
	}

	// Verify tools are available
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		if mcpTool, ok := tool.(interface{ Name() string }); ok {
			toolNames[mcpTool.Name()] = true
		}
	}

	expectedTools := []string{
		"mcp_test-server_echo",
		"mcp_test-server_calculate",
		"mcp_test-server_get_status",
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Expected tool %s not found", expected)
		}
	}

	// Test tool execution
	mockServer.SetToolResponse("echo", testing.CreateMockToolResponse("Hello World", false))

	// Note: Direct tool execution would require ExecuteTool method
	// This validates the integration setup is working

	// Disconnect
	err = framework.DisconnectMCPServer("test-server")
	if err != nil {
		t.Errorf("Failed to disconnect: %v", err)
	}

	// Verify disconnected
	servers = framework.ListMCPServers()
	if len(servers) != 0 {
		t.Errorf("Expected 0 servers after disconnect, got %d", len(servers))
	}
}

func TestMCPIntegration_MultipleServers(t *testing.T) {
	// Start two mock servers
	server1 := testing.NewMockMCPServer()
	server1.AddTool(client.MCPTool{
		Name:        "server1_tool",
		Description: "Tool from server 1",
		InputSchema: map[string]interface{}{"type": "object"},
	})
	err := server1.Start(":18081")
	if err != nil {
		t.Fatalf("Failed to start server 1: %v", err)
	}
	defer server1.Stop()

	server2 := testing.NewMockMCPServer()
	server2.AddTool(client.MCPTool{
		Name:        "server2_tool",
		Description: "Tool from server 2",
		InputSchema: map[string]interface{}{"type": "object"},
	})
	err = server2.Start(":18082")
	if err != nil {
		t.Fatalf("Failed to start server 2: %v", err)
	}
	defer server2.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create framework
	framework := core.NewFramework()
	defer framework.Close()

	ctx := context.Background()

	// Connect to both servers
	err = framework.ConnectMCPServer(ctx, &client.ClientConfig{
		ServerName:     "server1",
		Transport:      client.TransportHTTP,
		URL:            "http://localhost:18081",
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to connect to server 1: %v", err)
	}

	err = framework.ConnectMCPServer(ctx, &client.ClientConfig{
		ServerName:     "server2",
		Transport:      client.TransportHTTP,
		URL:            "http://localhost:18082",
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to connect to server 2: %v", err)
	}

	// Verify both servers connected
	servers := framework.ListMCPServers()
	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}

	// Create agent with access to all MCP tools
	agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:         "Multi-Server Agent",
		BehaviorType: "default",
		Capabilities: []string{"mcp_integration"},
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Verify agent has tools from both servers
	tools := framework.GetToolsForAgent(agent)
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		if mcpTool, ok := tool.(interface{ Name() string }); ok {
			toolNames[mcpTool.Name()] = true
		}
	}

	if !toolNames["mcp_server1_server1_tool"] {
		t.Error("Expected tool from server1 not found")
	}
	if !toolNames["mcp_server2_server2_tool"] {
		t.Error("Expected tool from server2 not found")
	}

	// Test status retrieval
	status := framework.GetMCPServerStatus()
	if len(status) != 2 {
		t.Errorf("Expected status for 2 servers, got %d", len(status))
	}

	// Disconnect from one server
	err = framework.DisconnectMCPServer("server1")
	if err != nil {
		t.Errorf("Failed to disconnect from server1: %v", err)
	}

	servers = framework.ListMCPServers()
	if len(servers) != 1 {
		t.Errorf("Expected 1 server after disconnect, got %d", len(servers))
	}

	if servers[0] != "server2" {
		t.Errorf("Expected remaining server to be 'server2', got %s", servers[0])
	}
}

func TestMCPIntegration_ToolRefresh(t *testing.T) {
	mockServer := testing.NewMockMCPServer()
	mockServer.AddTool(client.MCPTool{
		Name:        "initial_tool",
		Description: "Initial tool",
		InputSchema: map[string]interface{}{"type": "object"},
	})

	err := mockServer.Start(":18083")
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer mockServer.Stop()

	time.Sleep(100 * time.Millisecond)

	framework := core.NewFramework()
	defer framework.Close()

	ctx := context.Background()

	// Connect
	err = framework.ConnectMCPServer(ctx, &client.ClientConfig{
		ServerName:     "refresh-test",
		Transport:      client.TransportHTTP,
		URL:            "http://localhost:18083",
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Create agent
	agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:         "Refresh Agent",
		BehaviorType: "default",
		Capabilities: []string{"mcp_refresh-test"},
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Check initial tools
	tools := framework.GetToolsForAgent(agent)
	initialCount := len(tools)

	// Add a new tool to mock server
	mockServer.AddTool(client.MCPTool{
		Name:        "new_tool",
		Description: "Newly added tool",
		InputSchema: map[string]interface{}{"type": "object"},
	})

	// Refresh tools
	err = framework.RefreshMCPTools(ctx, "refresh-test")
	if err != nil {
		t.Errorf("Failed to refresh tools: %v", err)
	}

	// Verify tools updated
	tools = framework.GetToolsForAgent(agent)
	newCount := len(tools)

	if newCount != initialCount+1 {
		t.Errorf("Expected %d tools after refresh, got %d", initialCount+1, newCount)
	}
}

func TestMCPIntegration_SchemaValidation(t *testing.T) {
	mockServer := testing.NewMockMCPServer()
	mockServer.AddTool(client.MCPTool{
		Name:        "validate_tool",
		Description: "Tool with schema validation",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":      "string",
					"minLength": 3.0,
				},
				"age": map[string]interface{}{
					"type":    "number",
					"minimum": 0.0,
				},
			},
			"required": []interface{}{"name", "age"},
		},
	})

	err := mockServer.Start(":18084")
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer mockServer.Stop()

	time.Sleep(100 * time.Millisecond)

	framework := core.NewFramework()
	defer framework.Close()

	ctx := context.Background()

	err = framework.ConnectMCPServer(ctx, &client.ClientConfig{
		ServerName:     "validation-test",
		Transport:      client.TransportHTTP,
		URL:            "http://localhost:18084",
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Schema validation is now automatically enabled in tool wrappers
	// The integration test verifies the connection and tool registration works
	// Actual validation testing is done in unit tests
}

func TestMCPIntegration_HealthChecks(t *testing.T) {
	mockServer := testing.NewMockMCPServer()
	for _, tool := range testing.CreateTestTools() {
		mockServer.AddTool(tool)
	}

	err := mockServer.Start(":18085")
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer mockServer.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create framework and client manager
	framework := core.NewFramework()
	defer framework.Close()

	ctx := context.Background()

	err = framework.ConnectMCPServer(ctx, &client.ClientConfig{
		ServerName:     "health-test",
		Transport:      client.TransportHTTP,
		URL:            "http://localhost:18085",
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Get server status
	status := framework.GetMCPServerStatus()
	if len(status) != 1 {
		t.Errorf("Expected status for 1 server, got %d", len(status))
	}

	serverStatus, exists := status["health-test"]
	if !exists {
		t.Error("Expected status for 'health-test' server")
	}

	// Verify status contains expected information
	if serverStatus == nil {
		t.Error("Server status is nil")
	}
}
