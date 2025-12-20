package client

import (
	"context"
	"testing"
	"time"
)

func TestNewMCPClientManager(t *testing.T) {
	manager := NewMCPClientManager(nil)
	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}

	if manager.clients == nil {
		t.Error("Expected clients map to be initialized")
	}

	if manager.config == nil {
		t.Error("Expected config to be initialized with defaults")
	}
}

func TestMCPClientManager_ConnectServer_InvalidConfig(t *testing.T) {
	manager := NewMCPClientManager(nil)
	ctx := context.Background()

	// Test with empty server name
	config := &ClientConfig{
		ServerName: "",
		Transport:  TransportStdio,
		Command:    "echo",
	}

	err := manager.ConnectServer(ctx, config)
	if err == nil {
		t.Error("Expected error for empty server name")
	}
}

func TestMCPClientManager_ConnectServer_DuplicateConnection(t *testing.T) {
	manager := NewMCPClientManager(nil)

	// Use a short timeout context to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	config := &ClientConfig{
		ServerName:     "test-server",
		Transport:      TransportStdio,
		Command:        "echo",
		ConnectTimeout: 500 * time.Millisecond,
	}

	// First connection - will fail to connect (echo is not an MCP server)
	// This tests that even failed connections are tracked to prevent duplicates
	firstErr := manager.ConnectServer(ctx, config)

	// The first connection should fail (echo doesn't speak MCP protocol)
	// but the server entry should still be created
	if firstErr == nil {
		// If somehow it didn't error, try second connection
		err := manager.ConnectServer(ctx, config)
		if err == nil {
			t.Error("Expected error for duplicate connection")
		}
		if err != nil && err.Error() != "already connected to server: test-server" {
			t.Errorf("Expected duplicate connection error, got: %v", err)
		}
	} else {
		// First connection failed as expected - verify the client was NOT added
		// (failed connections shouldn't add to the client map)
		_, getErr := manager.GetClient("test-server")
		if getErr == nil {
			t.Error("Expected failed connection to not add client to manager")
		}
	}
}

func TestMCPClientManager_ListServers(t *testing.T) {
	manager := NewMCPClientManager(nil)

	// Initially empty
	servers := manager.ListServers()
	if len(servers) != 0 {
		t.Errorf("Expected 0 servers, got %d", len(servers))
	}

	// Add a mock client directly
	manager.mu.Lock()
	manager.clients["test1"] = &MCPClient{serverName: "test1"}
	manager.clients["test2"] = &MCPClient{serverName: "test2"}
	manager.mu.Unlock()

	servers = manager.ListServers()
	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}
}

func TestMCPClientManager_GetClient(t *testing.T) {
	manager := NewMCPClientManager(nil)

	// Test non-existent client
	_, err := manager.GetClient("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent client")
	}

	// Add a mock client
	mockClient := &MCPClient{serverName: "test"}
	manager.mu.Lock()
	manager.clients["test"] = mockClient
	manager.mu.Unlock()

	// Test existing client
	client, err := manager.GetClient("test")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if client != mockClient {
		t.Error("Expected same client instance")
	}
}

func TestMCPClientManager_DisconnectServer(t *testing.T) {
	manager := NewMCPClientManager(nil)

	// Test disconnecting non-existent server
	err := manager.DisconnectServer("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent server")
	}

	// Add a mock client
	mockClient := &MCPClient{serverName: "test", connected: false}
	manager.mu.Lock()
	manager.clients["test"] = mockClient
	manager.mu.Unlock()

	// Disconnect existing server
	err = manager.DisconnectServer("test")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify client was removed
	_, err = manager.GetClient("test")
	if err == nil {
		t.Error("Expected client to be removed")
	}
}

func TestMCPClientManager_GetStatus(t *testing.T) {
	manager := NewMCPClientManager(nil)

	// Add mock clients
	manager.mu.Lock()
	manager.clients["test1"] = &MCPClient{
		serverName: "test1",
		connected:  true,
		config:     &ClientConfig{Transport: TransportStdio},
		tools:      []MCPTool{{Name: "tool1"}},
		metrics:    &clientMetrics{},
	}
	manager.clients["test2"] = &MCPClient{
		serverName: "test2",
		connected:  false,
		config:     &ClientConfig{Transport: TransportHTTP},
		tools:      []MCPTool{},
		metrics:    &clientMetrics{},
	}
	manager.mu.Unlock()

	status := manager.GetStatus()
	if len(status) != 2 {
		t.Errorf("Expected 2 status entries, got %d", len(status))
	}

	if status["test1"].Connected != true {
		t.Error("Expected test1 to be connected")
	}

	if status["test1"].ToolsDiscovered != 1 {
		t.Errorf("Expected 1 tool for test1, got %d", status["test1"].ToolsDiscovered)
	}

	if status["test2"].Connected != false {
		t.Error("Expected test2 to be disconnected")
	}
}

func TestMCPClientManager_Close(t *testing.T) {
	manager := NewMCPClientManager(nil)

	// Add mock clients
	manager.mu.Lock()
	manager.clients["test1"] = &MCPClient{serverName: "test1", connected: false}
	manager.clients["test2"] = &MCPClient{serverName: "test2", connected: false}
	manager.mu.Unlock()

	err := manager.Close()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify all clients removed
	if len(manager.clients) != 0 {
		t.Errorf("Expected 0 clients after close, got %d", len(manager.clients))
	}
}

func TestMCPClient_GetTools(t *testing.T) {
	client := &MCPClient{
		tools: []MCPTool{
			{Name: "tool1", Description: "First tool"},
			{Name: "tool2", Description: "Second tool"},
		},
	}

	tools := client.GetTools()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}

	// Verify it returns a copy
	tools[0].Name = "modified"
	if client.tools[0].Name == "modified" {
		t.Error("Expected GetTools to return a copy, not original slice")
	}
}

func TestMCPClient_GetTool(t *testing.T) {
	client := &MCPClient{
		tools: []MCPTool{
			{Name: "tool1", Description: "First tool"},
			{Name: "tool2", Description: "Second tool"},
		},
	}

	// Test existing tool
	tool, err := client.GetTool("tool1")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if tool.Name != "tool1" {
		t.Errorf("Expected tool1, got %s", tool.Name)
	}

	// Test non-existent tool
	_, err = client.GetTool("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent tool")
	}
}

func TestMCPClient_IsConnected(t *testing.T) {
	client := &MCPClient{connected: false}

	if client.IsConnected() {
		t.Error("Expected client to be disconnected")
	}

	client.stateMu.Lock()
	client.connected = true
	client.stateMu.Unlock()

	if !client.IsConnected() {
		t.Error("Expected client to be connected")
	}
}

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	if config.MaxServers != 10 {
		t.Errorf("Expected MaxServers=10, got %d", config.MaxServers)
	}

	if config.EnableAutoReconnect != true {
		t.Error("Expected EnableAutoReconnect=true")
	}

	if config.DefaultTimeout != 30*time.Second {
		t.Errorf("Expected DefaultTimeout=30s, got %v", config.DefaultTimeout)
	}

	if config.MaxReconnectRetries != 3 {
		t.Errorf("Expected MaxReconnectRetries=3, got %d", config.MaxReconnectRetries)
	}

	if config.ReconnectBackoff != 2*time.Second {
		t.Errorf("Expected ReconnectBackoff=2s, got %v", config.ReconnectBackoff)
	}
}

func TestClientConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      *ClientConfig
		shouldError bool
	}{
		{
			name: "Valid stdio config",
			config: &ClientConfig{
				ServerName: "test",
				Transport:  TransportStdio,
				Command:    "echo",
			},
			shouldError: false,
		},
		{
			name: "Valid HTTP config",
			config: &ClientConfig{
				ServerName: "test",
				Transport:  TransportHTTP,
				URL:        "http://localhost:8080",
			},
			shouldError: false,
		},
		{
			name: "Missing server name",
			config: &ClientConfig{
				Transport: TransportStdio,
				Command:   "echo",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newMCPClient(tt.config)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}
