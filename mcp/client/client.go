package client

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// NewMCPClientManager creates a new MCP client manager
func NewMCPClientManager(config *ManagerConfig) *MCPClientManager {
	if config == nil {
		config = DefaultManagerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &MCPClientManager{
		clients: make(map[string]*MCPClient),
		config:  config,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// ConnectServer connects to an external MCP server
func (m *MCPClientManager) ConnectServer(ctx context.Context, config *ClientConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already connected
	if _, exists := m.clients[config.ServerName]; exists {
		return fmt.Errorf("already connected to server: %s", config.ServerName)
	}

	// Check max servers limit
	if m.config.MaxServers > 0 && len(m.clients) >= m.config.MaxServers {
		return fmt.Errorf("max servers limit reached: %d", m.config.MaxServers)
	}

	// Create new client
	client, err := newMCPClient(config)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Connect to server
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	// Store client
	m.clients[config.ServerName] = client

	return nil
}

// DisconnectServer disconnects from an MCP server
func (m *MCPClientManager) DisconnectServer(serverName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[serverName]
	if !exists {
		return fmt.Errorf("not connected to server: %s", serverName)
	}

	// Disconnect client
	if err := client.Disconnect(); err != nil {
		return fmt.Errorf("failed to disconnect: %w", err)
	}

	// Remove from map
	delete(m.clients, serverName)

	return nil
}

// GetClient returns a client by server name
func (m *MCPClientManager) GetClient(serverName string) (*MCPClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[serverName]
	if !exists {
		return nil, fmt.Errorf("not connected to server: %s", serverName)
	}

	return client, nil
}

// ListServers returns names of all connected servers
func (m *MCPClientManager) ListServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	servers := make([]string, 0, len(m.clients))
	for name := range m.clients {
		servers = append(servers, name)
	}

	return servers
}

// GetStatus returns status of all connected servers
func (m *MCPClientManager) GetStatus() map[string]*MCPClientStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]*MCPClientStatus)
	for name, client := range m.clients {
		status[name] = client.GetStatus()
	}

	return status
}

// Close closes all connections
func (m *MCPClientManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, client := range m.clients {
		if err := client.Disconnect(); err != nil {
			lastErr = fmt.Errorf("failed to disconnect from %s: %w", name, err)
		}
	}

	m.clients = make(map[string]*MCPClient)
	m.cancel()

	return lastErr
}

// newMCPClient creates a new MCP client
func newMCPClient(config *ClientConfig) (*MCPClient, error) {
	if config.ServerName == "" {
		return nil, fmt.Errorf("server name is required")
	}

	return &MCPClient{
		serverName: config.ServerName,
		config:     config,
		tools:      make([]MCPTool, 0),
		metrics: &clientMetrics{
			mu: sync.RWMutex{},
		},
	}, nil
}

// Connect establishes connection to the MCP server with retry logic
func (c *MCPClient) Connect(ctx context.Context) error {
	c.stateMu.Lock()
	if c.connected {
		c.stateMu.Unlock()
		return fmt.Errorf("already connected")
	}
	c.stateMu.Unlock()

	// Use retry logic for connection
	retryConfig := &RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
		Jitter:         true,
	}

	return WithRetry(ctx, retryConfig, func(ctx context.Context) error {
		return c.connectOnce(ctx)
	})
}

// connectOnce attempts a single connection without retry
func (c *MCPClient) connectOnce(ctx context.Context) error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	if c.connected {
		return nil // Already connected during retry
	}

	// Create transport based on config
	transport, err := c.createTransport()
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}
	c.transport = transport

	// Connect transport
	if err := c.transport.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect transport: %w", err)
	}

	// Discover available tools
	if err := c.discoverTools(ctx); err != nil {
		c.transport.Close()
		return fmt.Errorf("failed to discover tools: %w", err)
	}

	c.connected = true
	c.lastConnectTime = time.Now()
	c.reconnectAttempts = 0

	return nil
}

// Disconnect closes connection to server
func (c *MCPClient) Disconnect() error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	if !c.connected {
		return nil
	}

	if c.transport != nil {
		if err := c.transport.Close(); err != nil {
			return err
		}
		c.transport = nil
	}

	c.connected = false
	c.tools = make([]MCPTool, 0)

	return nil
}

// IsConnected returns connection status
func (c *MCPClient) IsConnected() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.connected
}

// GetTools returns cached tools
func (c *MCPClient) GetTools() []MCPTool {
	c.toolsMu.RLock()
	defer c.toolsMu.RUnlock()

	// Return copy
	tools := make([]MCPTool, len(c.tools))
	copy(tools, c.tools)

	return tools
}

// GetTool returns a specific tool by name
func (c *MCPClient) GetTool(name string) (*MCPTool, error) {
	c.toolsMu.RLock()
	defer c.toolsMu.RUnlock()

	for i := range c.tools {
		if c.tools[i].Name == name {
			return &c.tools[i], nil
		}
	}

	return nil, fmt.Errorf("tool not found: %s", name)
}

// CallTool executes a tool on the remote server
func (c *MCPClient) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (*MCPCallToolResult, error) {
	// Check connection
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to server")
	}

	// Verify tool exists
	if _, err := c.GetTool(toolName); err != nil {
		return nil, err
	}

	// Record metrics
	startTime := time.Now()
	var callErr error
	defer func() {
		c.recordMetrics(time.Since(startTime), callErr == nil)
	}()

	// Create request
	request := &MCPCallToolRequest{
		Name:      toolName,
		Arguments: args,
	}

	// Send request
	result, err := c.transport.SendRequest(ctx, "tools/call", request)
	if err != nil {
		callErr = err
		return nil, fmt.Errorf("tool call failed: %w", err)
	}

	// Parse result
	toolResult, ok := result.(*MCPCallToolResult)
	if !ok {
		callErr = fmt.Errorf("invalid result type")
		return nil, callErr
	}

	return toolResult, nil
}

// GetStatus returns client status
func (c *MCPClient) GetStatus() *MCPClientStatus {
	c.stateMu.RLock()
	connected := c.connected
	c.stateMu.RUnlock()

	c.toolsMu.RLock()
	toolsCount := len(c.tools)
	c.toolsMu.RUnlock()

	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	return &MCPClientStatus{
		ServerName:      c.serverName,
		Connected:       connected,
		Transport:       string(c.config.Transport),
		ToolsDiscovered: toolsCount,
		TotalCalls:      c.metrics.toolCallsTotal,
		SuccessCalls:    c.metrics.toolCallsSucceeded,
		FailedCalls:     c.metrics.toolCallsFailed,
		LastError:       c.metrics.lastError,
		LastErrorTime:   c.metrics.lastErrorTime,
	}
}

// recordMetrics updates client metrics
func (c *MCPClient) recordMetrics(duration time.Duration, success bool) {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	c.metrics.toolCallsTotal++
	if success {
		c.metrics.toolCallsSucceeded++
	} else {
		c.metrics.toolCallsFailed++
	}

	// Update average response time (exponential moving average)
	alpha := 0.1
	c.metrics.avgResponseTime = (1-alpha)*c.metrics.avgResponseTime +
		alpha*duration.Seconds()
}

// createTransport creates the appropriate transport based on config
func (c *MCPClient) createTransport() (Transport, error) {
	switch c.config.Transport {
	case TransportStdio:
		return newStdioTransport(c.config)
	case TransportHTTP:
		return newHTTPTransport(c.config)
	default:
		return nil, fmt.Errorf("unsupported transport: %s", c.config.Transport)
	}
}

// discoverTools fetches available tools from the server
func (c *MCPClient) discoverTools(ctx context.Context) error {
	result, err := c.transport.SendRequest(ctx, "tools/list", nil)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	// Parse tools
	toolsData, ok := result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid tools response")
	}

	toolsArray, ok := toolsData["tools"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid tools array")
	}

	tools := make([]MCPTool, 0, len(toolsArray))
	for _, toolData := range toolsArray {
		toolMap, ok := toolData.(map[string]interface{})
		if !ok {
			continue
		}

		tool := MCPTool{
			Name:        getStringField(toolMap, "name"),
			Description: getStringField(toolMap, "description"),
			InputSchema: getMapField(toolMap, "inputSchema"),
		}

		tools = append(tools, tool)
	}

	c.toolsMu.Lock()
	c.tools = tools
	c.toolsMu.Unlock()

	return nil
}

// Helper functions

func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getMapField(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key].(map[string]interface{}); ok {
		return val
	}
	return make(map[string]interface{})
}
