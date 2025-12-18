# MCP Integration - Detailed Design Document

**Version**: 2.0
**Date**: 2025-12-17
**Status**: Design Review (Updated - Client Only)
**Purpose**: Deep technical design for MCP client integration into Minion framework

---

## Table of Contents

1. [Design Overview](#design-overview)
2. [Component Architecture](#component-architecture)
3. [Data Structures](#data-structures)
4. [Sequence Diagrams](#sequence-diagrams)
5. [Design Decisions](#design-decisions)
6. [Interface Contracts](#interface-contracts)
7. [State Management](#state-management)
8. [Error Handling](#error-handling)
9. [Concurrency Model](#concurrency-model)
10. [Integration Points](#integration-points)
11. [Alternative Approaches](#alternative-approaches)
12. [Open Questions](#open-questions)

---

## 1. Design Overview

### 1.1 Core Concept

The MCP integration adds a **unidirectional bridge** allowing Minion agents to consume external MCP tools:

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                  │
│                    MINION FRAMEWORK                              │
│                                                                  │
│  ┌────────────────────────────────────────────────────────┐    │
│  │           EXISTING TOOL SYSTEM (84 tools)              │    │
│  │                                                          │    │
│  │  tools.Registry ──→ []tools.Tool                       │    │
│  │      ├─ Sales Tools (8)                                │    │
│  │      ├─ Marketing Tools (9)                            │    │
│  │      ├─ Analytics Tools (10)                           │    │
│  │      └─ ... (57 more tools)                            │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                  │
│                     MCP Client Manager                          │
│                     ┌────────────────┐                          │
│                     │   Inbound      │                          │
│                     │   Direction    │                          │
│                     │                │                          │
│                     │   Consume      │                          │
│                     │   External     │                          │
│                     │   MCP Tools    │                          │
│                     │   as Minion    │                          │
│                     └────────┬───────┘                          │
│                              │                                   │
└──────────────────────────────┼───────────────────────────────────┘
                               │
                               │
                   ┌───────────▼────────────┐
                   │                        │
                   │  External MCP Servers  │
                   │                        │
                   │  - GitHub Server       │
                   │  - Slack Server        │
                   │  - Gmail Server        │
                   │  - Filesystem Server   │
                   │  - 1000+ more          │
                   └────────────────────────┘
```

### 1.2 Design Principles

1. **Non-Invasive**: MCP integration is optional, doesn't modify existing code
2. **Transparent**: External MCP tools appear as native Minion tools
3. **Standard-Compliant**: Strictly follow MCP specification 2025-11-25
4. **Production-Ready**: Handle errors, retries, connection failures gracefully
5. **Performance-Conscious**: Minimize overhead, support concurrent operations
6. **Secure**: Proper authentication, authorization, input validation

### 1.3 Key Design Challenges

| Challenge | Solution |
|-----------|----------|
| **Tool Interface Mismatch** | Adapter pattern to convert between Minion Tool and MCP Tool formats |
| **State Management** | Client maintains connection state and session pool |
| **Concurrent Access** | Use sync.RWMutex for thread-safe access to tool registries and connections |
| **Error Propagation** | Wrap MCP errors in Minion error format, maintain error context |
| **Transport Abstraction** | Support both stdio and HTTP with common interface |
| **Lifecycle Management** | Proper initialization, graceful shutdown, connection cleanup |

---

## 2. Component Architecture

### 2.1 MCP Client Manager

**Purpose**: Connect to external MCP servers and consume their tools

#### 2.1.1 Component Structure

```go
// File: mcp/client/client.go

package client

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPClientManager manages connections to external MCP servers
type MCPClientManager struct {
    // Active connections
    clients map[string]*MCPClient // serverName → client
    mu      sync.RWMutex

    // Configuration
    config *ManagerConfig

    // Lifecycle
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

// MCPClient represents a connection to a single MCP server
type MCPClient struct {
    // Identity
    serverName string

    // Configuration
    config *ClientConfig

    // MCP SDK components
    mcpClient *mcp.Client
    session   *mcp.ClientSession
    transport mcp.Transport

    // Cached tools from this server
    tools   []mcp.Tool
    toolsMu sync.RWMutex

    // Connection state
    connected bool
    stateMu   sync.RWMutex

    // Reconnection
    reconnectAttempts int
    lastConnectTime   time.Time

    // Metrics
    metrics *clientMetrics
}

// ManagerConfig configures the client manager
type ManagerConfig struct {
    // Global settings
    MaxServers      int           // Max concurrent server connections
    DefaultTimeout  time.Duration // Default operation timeout

    // Reconnection policy
    EnableAutoReconnect bool
    MaxReconnectRetries int
    ReconnectBackoff    time.Duration

    // Logging
    Logger Logger
}

// ClientConfig configures a single MCP server connection
type ClientConfig struct {
    // Server identity
    ServerName  string
    Description string

    // Transport configuration
    Transport   TransportType // stdio or http

    // For stdio transport
    Command     string            // e.g., "npx"
    Args        []string          // e.g., ["-y", "@modelcontextprotocol/server-github"]
    Env         map[string]string // Environment variables (API keys, etc.)
    WorkingDir  string            // Working directory for command

    // For HTTP transport
    URL         string // e.g., "http://localhost:8080/mcp"

    // Authentication
    AuthType    AuthType
    AuthConfig  interface{} // Type depends on AuthType

    // Capabilities
    Capabilities []string // Capabilities to request from server

    // Timeouts
    ConnectTimeout time.Duration
    RequestTimeout time.Duration
}

type AuthType string

const (
    AuthNone   AuthType = "none"
    AuthBearer AuthType = "bearer"
    AuthOAuth  AuthType = "oauth"
    AuthAPIKey AuthType = "apikey"
)

type TransportType string

const (
    TransportStdio TransportType = "stdio"
    TransportHTTP  TransportType = "http"
)

// clientMetrics tracks client performance
type clientMetrics struct {
    toolCallsTotal     int64
    toolCallsSucceeded int64
    toolCallsFailed    int64
    avgResponseTime    float64
    lastError          string
    lastErrorTime      time.Time
    mu                 sync.RWMutex
}
```

#### 2.1.2 Connection Management

```go
// File: mcp/client/connection.go

package client

// Connect establishes connection to the MCP server
func (c *MCPClient) Connect(ctx context.Context) error {
    c.stateMu.Lock()
    defer c.stateMu.Unlock()

    if c.connected {
        return fmt.Errorf("already connected")
    }

    // Create transport based on config
    transport, err := c.createTransport()
    if err != nil {
        return fmt.Errorf("failed to create transport: %w", err)
    }
    c.transport = transport

    // Create MCP client
    c.mcpClient = mcp.NewClient(transport)

    // Establish session with timeout
    connectCtx, cancel := context.WithTimeout(ctx, c.config.ConnectTimeout)
    defer cancel()

    session, err := c.mcpClient.Connect(connectCtx)
    if err != nil {
        return fmt.Errorf("failed to connect: %w", err)
    }
    c.session = session

    // Discover available tools
    if err := c.discoverTools(ctx); err != nil {
        return fmt.Errorf("failed to discover tools: %w", err)
    }

    c.connected = true
    c.lastConnectTime = time.Now()

    return nil
}

// createTransport creates appropriate transport based on config
func (c *MCPClient) createTransport() (mcp.Transport, error) {
    switch c.config.Transport {
    case TransportStdio:
        return c.createStdioTransport()
    case TransportHTTP:
        return c.createHTTPTransport()
    default:
        return nil, fmt.Errorf("unsupported transport: %s", c.config.Transport)
    }
}

// createStdioTransport creates stdio transport
func (c *MCPClient) createStdioTransport() (mcp.Transport, error) {
    // Build command with args
    cmd := exec.Command(c.config.Command, c.config.Args...)

    // Set environment variables
    if len(c.config.Env) > 0 {
        env := os.Environ()
        for k, v := range c.config.Env {
            env = append(env, fmt.Sprintf("%s=%s", k, v))
        }
        cmd.Env = env
    }

    // Set working directory
    if c.config.WorkingDir != "" {
        cmd.Dir = c.config.WorkingDir
    }

    // Create command transport
    transport := mcp.NewCommandTransport(cmd)

    return transport, nil
}

// createHTTPTransport creates HTTP transport
func (c *MCPClient) createHTTPTransport() (mcp.Transport, error) {
    // Create HTTP client with authentication
    httpClient := &http.Client{
        Timeout: c.config.RequestTimeout,
    }

    // Add authentication if configured
    if c.config.AuthType != AuthNone {
        httpClient.Transport = c.createAuthTransport()
    }

    // Create HTTP transport
    transport := mcp.NewHTTPTransport(c.config.URL, httpClient)

    return transport, nil
}

// Disconnect closes connection to server
func (c *MCPClient) Disconnect() error {
    c.stateMu.Lock()
    defer c.stateMu.Unlock()

    if !c.connected {
        return nil
    }

    // Close session
    if c.session != nil {
        c.session.Close()
        c.session = nil
    }

    // Close transport
    if c.transport != nil {
        c.transport.Close()
        c.transport = nil
    }

    c.connected = false

    return nil
}

// IsConnected returns connection status
func (c *MCPClient) IsConnected() bool {
    c.stateMu.RLock()
    defer c.stateMu.RUnlock()
    return c.connected
}
```

#### 2.1.3 Tool Discovery

```go
// File: mcp/client/discovery.go

package client

// discoverTools fetches available tools from the server
func (c *MCPClient) discoverTools(ctx context.Context) error {
    if !c.connected || c.session == nil {
        return fmt.Errorf("not connected")
    }

    // List tools from server
    listResult, err := c.session.ListTools(ctx)
    if err != nil {
        return fmt.Errorf("failed to list tools: %w", err)
    }

    // Cache discovered tools
    c.toolsMu.Lock()
    c.tools = listResult.Tools
    c.toolsMu.Unlock()

    return nil
}

// GetTools returns cached tools
func (c *MCPClient) GetTools() []mcp.Tool {
    c.toolsMu.RLock()
    defer c.toolsMu.RUnlock()

    // Return copy to prevent modification
    tools := make([]mcp.Tool, len(c.tools))
    copy(tools, c.tools)

    return tools
}

// GetTool returns a specific tool by name
func (c *MCPClient) GetTool(name string) (*mcp.Tool, error) {
    c.toolsMu.RLock()
    defer c.toolsMu.RUnlock()

    for _, tool := range c.tools {
        if tool.Name == name {
            return &tool, nil
        }
    }

    return nil, fmt.Errorf("tool not found: %s", name)
}

// RefreshTools re-discovers tools from server
func (c *MCPClient) RefreshTools(ctx context.Context) error {
    if !c.connected {
        return fmt.Errorf("not connected")
    }

    return c.discoverTools(ctx)
}
```

#### 2.1.4 Tool Execution

```go
// File: mcp/client/execution.go

package client

// CallTool executes a tool on the remote server
func (c *MCPClient) CallTool(
    ctx context.Context,
    toolName string,
    args map[string]interface{},
) (*mcp.CallToolResult, error) {
    // Check connection
    if !c.IsConnected() {
        return nil, fmt.Errorf("not connected to server")
    }

    // Verify tool exists
    tool, err := c.GetTool(toolName)
    if err != nil {
        return nil, fmt.Errorf("tool not found: %w", err)
    }

    // Record metrics
    startTime := time.Now()
    defer func() {
        c.recordMetrics(time.Since(startTime), err == nil)
    }()

    // Execute tool with timeout
    execCtx := ctx
    if c.config.RequestTimeout > 0 {
        var cancel context.CancelFunc
        execCtx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
        defer cancel()
    }

    // Call tool through session
    result, err := c.session.CallTool(execCtx, &mcp.CallToolRequest{
        Params: &mcp.CallToolParams{
            Name:      toolName,
            Arguments: args,
        },
    })
    if err != nil {
        return nil, fmt.Errorf("tool call failed: %w", err)
    }

    return result, nil
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
    alpha := 0.1 // smoothing factor
    c.metrics.avgResponseTime = (1-alpha)*c.metrics.avgResponseTime +
                                 alpha*duration.Seconds()
}
```

---

### 2.2 Tool Bridge

**Purpose**: Wrap external MCP tools as native Minion tools

#### 2.2.1 MCPToolWrapper

```go
// File: mcp/bridge/tool_wrapper.go

package bridge

import (
    "context"
    "fmt"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/yourusername/minion/mcp/client"
    "github.com/yourusername/minion/models"
    "github.com/yourusername/minion/tools"
)

// MCPToolWrapper wraps an external MCP tool as a Minion Tool
type MCPToolWrapper struct {
    // Server information
    serverName string

    // MCP tool metadata
    mcpTool mcp.Tool

    // Client connection
    clientManager *client.MCPClientManager

    // Capability mapping
    requiredCapabilities []string
}

// Ensure MCPToolWrapper implements tools.Tool interface
var _ tools.Tool = (*MCPToolWrapper)(nil)

// Name returns the qualified tool name
func (w *MCPToolWrapper) Name() string {
    // Format: mcp_<server>_<tool>
    // Example: mcp_github_create_issue
    return fmt.Sprintf("mcp_%s_%s", w.serverName, w.mcpTool.Name)
}

// Description returns tool description with MCP prefix
func (w *MCPToolWrapper) Description() string {
    return fmt.Sprintf("[MCP:%s] %s", w.serverName, w.mcpTool.Description)
}

// Execute executes the MCP tool via the client
func (w *MCPToolWrapper) Execute(
    ctx context.Context,
    input *models.ToolInput,
) (*models.ToolOutput, error) {
    // Prepare output structure
    output := &models.ToolOutput{
        ToolName: w.Name(),
    }

    // Get MCP client for this server
    client, err := w.clientManager.GetClient(w.serverName)
    if err != nil {
        output.Success = false
        output.Error = fmt.Sprintf("Server not connected: %v", err)
        return output, nil
    }

    // Execute tool on remote server
    result, err := client.CallTool(ctx, w.mcpTool.Name, input.Params)
    if err != nil {
        output.Success = false
        output.Error = fmt.Sprintf("Tool execution failed: %v", err)
        return output, nil
    }

    // Check if MCP tool reported an error
    if result.IsError {
        output.Success = false
        output.Error = w.extractErrorMessage(result)
        return output, nil
    }

    // Extract result from MCP response
    output.Success = true
    output.Result = w.extractResult(result)

    return output, nil
}

// CanExecute checks if agent has required capabilities
func (w *MCPToolWrapper) CanExecute(agent *models.Agent) bool {
    // Check for MCP integration capability
    hasMCPCapability := false
    hasServerCapability := false

    for _, cap := range agent.Capabilities {
        if cap == "mcp_integration" {
            hasMCPCapability = true
        }
        if cap == fmt.Sprintf("mcp_%s", w.serverName) {
            hasServerCapability = true
        }
    }

    // Agent needs either global MCP capability or server-specific capability
    if !hasMCPCapability && !hasServerCapability {
        return false
    }

    // Check additional required capabilities
    if len(w.requiredCapabilities) > 0 {
        capMap := make(map[string]bool)
        for _, cap := range agent.Capabilities {
            capMap[cap] = true
        }

        for _, required := range w.requiredCapabilities {
            if !capMap[required] {
                return false
            }
        }
    }

    return true
}

// extractResult extracts result data from MCP response
func (w *MCPToolWrapper) extractResult(result *mcp.CallToolResult) interface{} {
    if len(result.Content) == 0 {
        return nil
    }

    // Handle different content types
    content := result.Content[0]

    switch v := content.(type) {
    case map[string]interface{}:
        // Extract text content
        if text, ok := v["text"].(string); ok {
            // Try to parse as JSON for structured data
            var structured interface{}
            if err := json.Unmarshal([]byte(text), &structured); err == nil {
                return structured
            }
            return text
        }
        return v
    case string:
        return v
    default:
        return fmt.Sprintf("%v", v)
    }
}

// extractErrorMessage extracts error message from MCP result
func (w *MCPToolWrapper) extractErrorMessage(result *mcp.CallToolResult) string {
    if len(result.Content) == 0 {
        return "Unknown error"
    }

    content := result.Content[0]

    switch v := content.(type) {
    case map[string]interface{}:
        if text, ok := v["text"].(string); ok {
            return text
        }
        return fmt.Sprintf("%v", v)
    case string:
        return v
    default:
        return fmt.Sprintf("%v", v)
    }
}
```

#### 2.2.2 Bridge Registry

```go
// File: mcp/bridge/registry.go

package bridge

import (
    "context"
    "fmt"

    "github.com/yourusername/minion/core"
    "github.com/yourusername/minion/mcp/client"
)

// BridgeRegistry manages MCP tool wrappers
type BridgeRegistry struct {
    clientManager *client.MCPClientManager
    framework     core.Framework

    // Track wrapped tools
    wrappedTools map[string]*MCPToolWrapper // tool name → wrapper
    mu           sync.RWMutex
}

// NewBridgeRegistry creates a new bridge registry
func NewBridgeRegistry(
    clientManager *client.MCPClientManager,
    framework core.Framework,
) *BridgeRegistry {
    return &BridgeRegistry{
        clientManager: clientManager,
        framework:     framework,
        wrappedTools:  make(map[string]*MCPToolWrapper),
    }
}

// RegisterServerTools wraps and registers all tools from an MCP server
func (r *BridgeRegistry) RegisterServerTools(
    ctx context.Context,
    serverName string,
) error {
    // Get MCP client for server
    client, err := r.clientManager.GetClient(serverName)
    if err != nil {
        return fmt.Errorf("server not found: %w", err)
    }

    // Get tools from server
    tools := client.GetTools()
    if len(tools) == 0 {
        return fmt.Errorf("no tools found on server: %s", serverName)
    }

    // Wrap and register each tool
    for _, mcpTool := range tools {
        wrapper := &MCPToolWrapper{
            serverName:    serverName,
            mcpTool:       mcpTool,
            clientManager: r.clientManager,
            requiredCapabilities: []string{}, // Could be inferred from schema
        }

        // Register with Minion framework
        if err := r.framework.RegisterTool(wrapper); err != nil {
            return fmt.Errorf("failed to register tool %s: %w", wrapper.Name(), err)
        }

        // Track wrapped tool
        r.mu.Lock()
        r.wrappedTools[wrapper.Name()] = wrapper
        r.mu.Unlock()
    }

    return nil
}

// UnregisterServerTools removes all tools from a server
func (r *BridgeRegistry) UnregisterServerTools(serverName string) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Find and remove all tools for this server
    prefix := fmt.Sprintf("mcp_%s_", serverName)
    for toolName := range r.wrappedTools {
        if strings.HasPrefix(toolName, prefix) {
            delete(r.wrappedTools, toolName)
            // Note: Can't unregister from framework as it doesn't have Unregister method
            // This is a design decision - tools remain registered but unusable
        }
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
```

---

## 3. Data Structures

### 3.1 Core Data Models

```go
// Enhanced ToolInput to support MCP metadata
type ToolInput struct {
    Params   map[string]interface{}

    // MCP-specific fields (optional)
    MCP *MCPMetadata `json:"mcp,omitempty"`
}

type MCPMetadata struct {
    ServerName string `json:"server_name,omitempty"` // Source MCP server
    ToolName   string `json:"tool_name,omitempty"`   // Original tool name
    RequestID  string `json:"request_id,omitempty"`  // For tracking
}

// Enhanced ToolOutput to include MCP information
type ToolOutput struct {
    ToolName string
    Success  bool
    Result   interface{}
    Error    string

    // MCP-specific fields (optional)
    MCP *MCPResultMetadata `json:"mcp,omitempty"`
}

type MCPResultMetadata struct {
    ServerName   string        `json:"server_name,omitempty"`
    ResponseTime time.Duration `json:"response_time,omitempty"`
    ContentType  string        `json:"content_type,omitempty"` // text, json, binary
}
```

### 3.2 Connection State

```go
// ConnectionState tracks MCP client connection lifecycle
type ConnectionState int

const (
    StateDisconnected ConnectionState = iota
    StateConnecting
    StateConnected
    StateReconnecting
    StateError
)

func (s ConnectionState) String() string {
    switch s {
    case StateDisconnected:
        return "disconnected"
    case StateConnecting:
        return "connecting"
    case StateConnected:
        return "connected"
    case StateReconnecting:
        return "reconnecting"
    case StateError:
        return "error"
    default:
        return "unknown"
    }
}
```

---

## 4. Sequence Diagrams

### 4.1 MCP Client: Connecting to External Server

```
Minion Framework    MCP Client Manager    MCP Client    External MCP Server
      │                     │                   │                │
      ├─ 1. ConnectMCPServer(config) ────────>│                │
      │                     │                   │                │
      │                     ├─ 2. CreateClient ────────>        │
      │                     │                   │                │
      │                     │                   ├─ 3. Connect() ──────────>│
      │                     │                   │   (stdio/http) │
      │                     │                   │                │
      │                     │                   │<─ 4. Session ───────────┤
      │                     │                   │                │
      │                     │                   ├─ 5. ListTools() ────────>│
      │                     │                   │                │
      │                     │                   │<─ 6. Tools[] ────────────┤
      │                     │                   │                │
      │                     │<─ 7. MCPClient ────┤                │
      │                     │   (with cached tools)               │
      │                     │                   │                │
      │                     ├─ 8. RegisterServerTools() ──>      │
      │                     │   (wrap tools)    │                │
      │                     │                   │                │
      │<─ 9. Success ───────┤                   │                │
```

### 4.2 MCP Client: Using External Tools

```
Minion Agent    Framework    Tool Registry    MCPToolWrapper    MCP Client    External Server
    │               │               │                │                │                │
    ├─ 1. ExecuteTool("mcp_github_create_issue") ────────────────────>│                │
    │               │               │                │                │                │
    │               ├─ 2. GetTool() ──────────────>  │                │                │
    │               │               │                │                │                │
    │               │<─ 3. MCPToolWrapper ───────────┤                │                │
    │               │               │                │                │                │
    │               ├─ 4. Execute() ────────────────────────────────> │                │
    │               │               │    (wrapper)   │                │                │
    │               │               │                │                │                │
    │               │               │                ├─ 5. GetClient("github") ───>   │
    │               │               │                │                │                │
    │               │               │                │<─ 6. MCPClient ─────────────────┤
    │               │               │                │                │                │
    │               │               │                ├─ 7. CallTool() ────────────────>│
    │               │               │                │    ("create_issue", args)       │
    │               │               │                │                │                │
    │               │               │                │                ├─ 8. tools/call ───────>│
    │               │               │                │                │                │
    │               │               │                │                │<─ 9. response ─────────┤
    │               │               │                │                │                │
    │               │               │                │<─ 10. MCP Result ───────────────┤
    │               │               │                │                │                │
    │               │<─ 11. ToolOutput ──────────────────────────────┤                │
    │               │               (converted)      │                │                │
    │               │               │                │                │                │
    │<─ 12. Output ─────────────────┤                │                │                │
```

### 4.3 MCP Client: Auto-Reconnect on Failure

```
MCPToolWrapper    MCP Client    External Server
      │                │                │
      ├─ 1. CallTool() ────────>        │
      │                │                │
      │                ├─ 2. tools/call ──────────>│
      │                │                │
      │                │<─ 3. Error ──────────────┤
      │                │   (Connection Lost)      │
      │                │                │
      │                ├─ 4. Reconnect() ─────────>│
      │                │   (with backoff) │
      │                │                │
      │                │<─ 5. Session ─────────────┤
      │                │                │
      │                ├─ 6. Retry tools/call ─────>│
      │                │                │
      │                │<─ 7. Success ─────────────┤
      │                │                │
      │<─ 8. Result ───┤                │
```

---

## 5. Design Decisions

### 5.1 Tool Naming Convention

**Decision**: Prefix MCP tools with `mcp_<server>_<tool>`

**Rationale**:
- Avoids naming conflicts with native tools
- Makes tool origin clear to users
- Enables capability-based filtering (e.g., `mcp_github` capability)
- Allows easy identification and debugging

**Alternatives Considered**:
1. Use original tool names → Rejected due to conflict risk
2. Use `<server>::<tool>` → Rejected as it's not valid in some contexts
3. Use namespace prefix → Rejected as it's less readable

**Example**:
```
GitHub MCP Server Tool: "create_issue"
Minion Wrapped Tool:    "mcp_github_create_issue"
```

### 5.2 Capability Model

**Decision**: Use layered capability model

**Layers**:
1. **Global MCP**: `mcp_integration` - Allows any MCP tool
2. **Server-Specific**: `mcp_github` - Allows all GitHub tools
3. **Tool-Specific**: `mcp_github_create_issue` - Allows specific tool

**Rationale**:
- Flexibility: Admin can grant broad or narrow access
- Security: Fine-grained control over what agents can do
- Backward Compatible: Existing capability system unchanged

**Example**:
```go
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name: "GitHub Bot",
    Capabilities: []string{
        "mcp_github",  // Can use ALL GitHub MCP tools
    },
})

agent2, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name: "Limited Bot",
    Capabilities: []string{
        "mcp_github_list_issues",  // Can ONLY list issues
    },
})
```

### 5.3 Connection Lifecycle

**Decision**: Lazy connection with auto-reconnect

**Behavior**:
- Connections not established until first tool call
- Auto-reconnect on transient failures (with exponential backoff)
- Explicit disconnect required for cleanup

**Rationale**:
- Performance: Don't connect to unused servers
- Reliability: Resilient to temporary network issues
- Resource Management: Clean shutdown when needed

**Implementation**:
```go
func (c *MCPClient) CallTool(...) {
    // Check connection, reconnect if needed
    if !c.IsConnected() {
        if err := c.Connect(ctx); err != nil {
            return err
        }
    }

    // Execute tool...
}
```

### 5.4 Error Handling Strategy

**Decision**: Graceful degradation with detailed errors

**Principles**:
1. MCP errors don't crash Minion
2. Return ToolOutput with Success=false instead of hard errors
3. Include error context (server name, tool name, error message)
4. Log errors for debugging

**Rationale**:
- Agents can handle tool failures programmatically
- Better user experience (partial functionality vs total failure)
- Easier debugging with context

**Example**:
```go
output, err := framework.ExecuteTool(ctx, "mcp_github_create_issue", input)
// err is always nil - check output.Success instead
if !output.Success {
    log.Printf("Tool failed: %s", output.Error)
    // Continue with fallback logic...
}
```

### 5.5 Transport Selection

**Decision**: Support both stdio and HTTP transports

**Use Cases**:
- **Stdio**: Local MCP servers (npx-based), subprocess management
- **HTTP**: Remote servers, production deployments, cloud services

**Rationale**:
- Stdio: MCP standard for local servers, easy debugging
- HTTP: Production-ready, scalable, firewall-friendly

**Configuration**:
```go
// Stdio for local GitHub server
framework.ConnectMCPServer(ctx, &client.ClientConfig{
    ServerName: "github",
    Transport:  "stdio",
    Command:    "npx",
    Args:       []string{"-y", "@modelcontextprotocol/server-github"},
})

// HTTP for remote server
framework.ConnectMCPServer(ctx, &client.ClientConfig{
    ServerName: "custom",
    Transport:  "http",
    HTTPUrl:    "http://mcp.example.com/api",
})
```

### 5.6 Thread Safety Model

**Decision**: Coarse-grained locking with RWMutex

**Protected Resources**:
- Tool cache: RWMutex (many reads, few writes)
- Client connections: RWMutex (connection state changes rare)
- Metrics: Mutex (frequent updates)

**Rationale**:
- Simplicity: Easier to reason about than fine-grained locking
- Performance: RWMutex allows concurrent reads
- Correctness: Prevents race conditions

**Example**:
```go
type MCPClient struct {
    tools   []mcp.Tool
    toolsMu sync.RWMutex  // Protects tools

    connected bool
    stateMu   sync.RWMutex  // Protects connected
}

// Many concurrent readers OK
func (c *MCPClient) GetTools() []mcp.Tool {
    c.toolsMu.RLock()
    defer c.toolsMu.RUnlock()
    return c.tools
}

// Exclusive write access
func (c *MCPClient) discoverTools(...) {
    c.toolsMu.Lock()
    defer c.toolsMu.Unlock()
    c.tools = newTools
}
```

---

## 6. Interface Contracts

### 6.1 Framework Interface Extension

```go
// Extended Framework interface with MCP client support
type Framework interface {
    // ... existing methods ...

    // MCP Client Methods

    // ConnectMCPServer connects to an external MCP server
    // Tools are automatically discovered and registered
    // Returns error if already connected or connection fails
    ConnectMCPServer(ctx context.Context, config *client.ClientConfig) error

    // DisconnectMCPServer disconnects from an external MCP server
    // Wrapped tools remain in registry but become unavailable
    DisconnectMCPServer(serverName string) error

    // GetMCPClientStatus returns status of all connected MCP servers
    GetMCPClientStatus() map[string]*MCPClientStatus

    // ListMCPServers returns names of all connected MCP servers
    ListMCPServers() []string

    // RefreshMCPTools re-discovers tools from a connected server
    // Useful when server adds new tools dynamically
    RefreshMCPTools(ctx context.Context, serverName string) error

    // Tool Execution (Enhanced)

    // ExecuteTool executes a tool by name
    // Works for both native Minion tools and MCP-wrapped tools
    // Replaces direct tool.Execute() calls for better abstraction
    ExecuteTool(ctx context.Context, toolName string, input *models.ToolInput) (*models.ToolOutput, error)
}

// MCPClientStatus represents MCP client connection status
type MCPClientStatus struct {
    ServerName    string
    Connected     bool
    Transport     string
    ToolsDiscovered int
    TotalCalls    int64
    SuccessCalls  int64
    FailedCalls   int64
    LastError     string
    LastErrorTime time.Time
}
```

### 6.2 Tool Interface (No Changes Required)

```go
// Existing Tool interface - NO CHANGES NEEDED
// MCPToolWrapper implements this interface
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error)
    CanExecute(agent *models.Agent) bool
}
```

### 6.3 Registry Interface (Enhanced)

```go
// Enhanced Registry interface
type Registry interface {
    // Existing methods
    Register(tool Tool) error
    Get(name string) (Tool, error)
    GetToolsForAgent(agent *models.Agent) []Tool
    Execute(ctx context.Context, toolName string, input *models.ToolInput) (*models.ToolOutput, error)
    List() []string
    Count() int

    // New MCP-specific methods

    // IsMCPTool returns true if tool is MCP-wrapped
    IsMCPTool(toolName string) bool

    // GetMCPToolInfo returns MCP metadata for a wrapped tool
    GetMCPToolInfo(toolName string) (*MCPToolInfo, error)

    // ListByServer returns tools from a specific MCP server
    ListByServer(serverName string) []string
}

// MCPToolInfo provides metadata about MCP-wrapped tools
type MCPToolInfo struct {
    ServerName  string
    OriginalName string
    Schema      map[string]interface{}
    Connected   bool
}
```

---

## 7. State Management

### 7.1 Client Connection State

```
[DISCONNECTED] ──Connect()──> [CONNECTING] ──success──> [CONNECTED]
      ▲                            │                          │
      │                            │                          │
      │                            └──fail──> [ERROR]         │
      │                                           │           │
      │                                           │           │
      └──────────────────────<Reconnect>──────────┘           │
                                                               │
                              Disconnect() <───────────────────┘
                                   │
                                   v
                             [DISCONNECTED]
```

**State Transitions**:
- `DISCONNECTED` → `CONNECTING`: First `Connect()` or tool call
- `CONNECTING` → `CONNECTED`: Successful session establishment
- `CONNECTING` → `ERROR`: Connection failure
- `CONNECTED` → `DISCONNECTED`: Explicit `Disconnect()`
- `ERROR` → `CONNECTING`: Auto-reconnect attempt

**State Storage**:
```go
type connectionState struct {
    status            ConnectionStatus
    lastConnectTime   time.Time
    reconnectAttempts int
    lastError         error
    mu                sync.RWMutex
}
```

### 7.2 Tool Cache State

**Lifecycle**:
1. **Empty**: No tools cached
2. **Populating**: Discovery in progress
3. **Ready**: Tools cached and available
4. **Stale**: Needs refresh

**Cache Invalidation**:
- Manual: `RefreshMCPTools()` called
- Automatic: On reconnection
- Time-based: Optional TTL (not implemented in MVP)

---

## 8. Error Handling

### 8.1 Error Categories

| Category | Examples | Handling Strategy |
|----------|----------|-------------------|
| **Configuration Errors** | Invalid transport, missing required fields | Fail fast, return error immediately |
| **Connection Errors** | Network timeout, server unreachable | Auto-retry with backoff, eventual failure |
| **Authentication Errors** | Invalid API key, OAuth failure | Fail fast, log warning, don't retry |
| **Tool Execution Errors** | Invalid parameters, tool logic error | Return ToolOutput with Success=false |
| **Protocol Errors** | Invalid MCP message, schema mismatch | Log error, return generic failure |
| **Resource Errors** | Out of memory, too many connections | Log error, reject new requests |

### 8.2 Error Response Format

```go
// For Framework-level errors
type MCPError struct {
    Code      string    // Error code (e.g., "CONNECTION_FAILED")
    Message   string    // Human-readable message
    ServerName string   // Relevant MCP server (if applicable)
    ToolName  string    // Relevant tool (if applicable)
    Cause     error     // Underlying error
    Timestamp time.Time
}

func (e *MCPError) Error() string {
    return fmt.Sprintf("[MCP:%s] %s: %v", e.ServerName, e.Message, e.Cause)
}

// For Tool execution errors
// Use ToolOutput.Success = false and ToolOutput.Error
```

### 8.3 Error Propagation

```
External MCP Server Error
         │
         v
  MCP Client catches
         │
         v
  MCPToolWrapper converts to ToolOutput.Error
         │
         v
  Framework.ExecuteTool returns ToolOutput
         │
         v
  Agent checks output.Success
```

### 8.4 Retry Policy

```go
type RetryPolicy struct {
    MaxRetries      int
    InitialBackoff  time.Duration
    MaxBackoff      time.Duration
    BackoffMultiplier float64
}

// Default policy
var DefaultRetryPolicy = RetryPolicy{
    MaxRetries:        3,
    InitialBackoff:    1 * time.Second,
    MaxBackoff:        30 * time.Second,
    BackoffMultiplier: 2.0,
}

// Retryable errors
func isRetryable(err error) bool {
    // Network timeouts, temporary failures
    // NOT auth errors, invalid parameters
}
```

---

## 9. Concurrency Model

### 9.1 Client Concurrency

**Model**: Single connection, multiple concurrent calls OK

```go
// Multiple goroutines can call tools concurrently
// MCP session is thread-safe
go func() {
    result1, _ := client.CallTool(ctx, "tool1", args1)
}()

go func() {
    result2, _ := client.CallTool(ctx, "tool2", args2)
}()
```

### 9.2 Tool Registry Concurrency

**Model**: RWMutex for read-heavy workload

```go
type RegistryImpl struct {
    tools map[string]Tool
    mu    sync.RWMutex
}

// Many concurrent readers
func (r *RegistryImpl) Get(name string) (Tool, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    // Read tool...
}

// Exclusive writer (rare)
func (r *RegistryImpl) Register(tool Tool) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    // Write tool...
}
```

---

## 10. Integration Points

### 10.1 Integration with Existing Framework

**Minimal Changes Required**:

1. **core/framework.go**: Add MCP components
```go
type FrameworkImpl struct {
    // Existing fields
    store            storage.Store
    llmProvider      llm.Provider
    behaviorRegistry BehaviorRegistry
    toolRegistry     tools.Registry

    // NEW: MCP components
    mcpClientManager *client.MCPClientManager
    mcpBridge        *bridge.BridgeRegistry
}
```

2. **core/interfaces.go**: Extend Framework interface
```go
// Add new methods to Framework interface
type Framework interface {
    // ... existing methods ...

    // NEW: MCP methods
    ConnectMCPServer(...)
    DisconnectMCPServer(...)
    ListMCPServers()
    RefreshMCPTools(...)
    ExecuteTool(...)  // NEW helper method
}
```

3. **No changes required to**:
   - `models/` - All models remain unchanged
   - `storage/` - No storage changes needed
   - `llm/` - No LLM changes needed
   - `behaviors/` - No behavior changes needed
   - Existing tools - All tools work as-is

### 10.2 Integration with Tool System

**Strategy**: MCP tools are first-class citizens

```go
// Tool registry doesn't care if tool is native or MCP-wrapped
registry.Register(nativeTool)        // Native Minion tool
registry.Register(mcpWrappedTool)    // MCP-wrapped tool

// Both retrieved the same way
tool1, _ := registry.Get("revenue_analyzer")        // Native
tool2, _ := registry.Get("mcp_github_create_issue") // MCP

// Both executed the same way
output1, _ := tool1.Execute(ctx, input)
output2, _ := tool2.Execute(ctx, input)
```

**No special handling needed** in:
- Agent execution logic
- Behavior processors
- Activity tracking
- Metrics collection

### 10.3 Backward Compatibility

**Guarantee**: Existing code works unchanged

**Test Case**:
```go
// Existing code before MCP - STILL WORKS
func TestBackwardCompatibility(t *testing.T) {
    framework := core.NewFramework(
        core.WithStorage(storage.NewInMemory()),
    )

    // Register native tools
    domains.RegisterAllDomainTools(framework)

    // Create agent
    agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
        Name: "Test Agent",
        Capabilities: []string{"revenue_analysis"},
    })

    // Get tools
    tools := framework.GetToolsForAgent(agent)

    // Execute through registry
    tool, _ := framework.toolRegistry.Get("revenue_analyzer")
    output, _ := tool.Execute(ctx, input)

    // ALL ABOVE CODE WORKS EXACTLY AS BEFORE
}
```

---

## 11. Alternative Approaches

### 11.1 Alternative: Direct Protocol Implementation

**Considered**: Implement MCP protocol from scratch without SDK

**Decision**: ❌ Rejected - Use official Go SDK
**Reason**: Official SDK is maintained by Google, reduces risk, faster to market

### 11.2 Alternative: Separate MCP Service

**Considered**: Build standalone MCP service, separate from Minion

**Decision**: ❌ Rejected for MVP - Integrated approach simpler
**Future**: Could extract as microservice later if needed

### 11.3 Alternative: Tool Inheritance vs Wrapping

**Considered**: Make MCP tools inherit from base Tool struct

**Decision**: ✅ Use Composition (Wrapper pattern)
**Reason**: Better encapsulation, clearer separation, more flexible

### 11.4 Alternative: Synchronous vs Asynchronous Tool Calls

**Considered**: Make all tool calls async with callbacks

**Decision**: ✅ Synchronous for MVP, Async optional later
**Reason**: Simpler API, meets current needs, can add async later

---

## 12. Open Questions

### 12.1 Authentication Management

**Question**: How should we manage API keys and OAuth tokens for external MCP servers?

**Options**:
1. **Environment variables** - Simple, but limited
2. **Configuration files** - More structured, but security risk if committed
3. **Secrets manager integration** - Secure, but adds complexity
4. **Per-agent credentials** - Flexible, but complicated to manage

**Proposed**: Start with environment variables, add secrets manager support in Phase 3

**Decision Needed**: ⏳ Week 1

---

### 12.2 Tool Versioning

**Question**: How do we handle MCP server tool updates that break compatibility?

**Scenarios**:
- Tool renamed on server
- Parameter schema changed
- Tool removed

**Options**:
1. **Ignore - let it break** - Simplest, but poor UX
2. **Version in tool name** - e.g., `mcp_github_v2_create_issue`
3. **Version negotiation** - Request specific tool version
4. **Graceful degradation** - Fall back to older tool version

**Proposed**: Log warnings on tool changes, manual refresh required

**Decision Needed**: ⏳ Week 2

---

### 12.3 Resource and Prompt Support

**Question**: Should MVP support MCP Resources and Prompts, or just Tools?

**Context**: MCP spec includes three primitives:
- **Tools**: Functions AI can call
- **Resources**: Data/context for AI (files, docs, etc.)
- **Prompts**: Templated conversation starters

**Options**:
1. **Tools only** - Faster MVP
2. **Tools + Resources** - More complete
3. **All three** - Full MCP support

**Proposed**: Tools only for MVP, Resources in Phase 4

**Decision Needed**: ⏳ Before implementation start

---

### 12.4 Performance Optimization

**Question**: Should we implement tool result caching?

**Scenario**: Same tool called with same parameters multiple times

**Options**:
1. **No caching** - Simplest, always fresh data
2. **Memory cache** - Fast, but may return stale data
3. **Configurable cache** - Flexible, but complex

**Proposed**: No caching in MVP, add as optional feature later

**Decision Needed**: ⏳ Based on performance testing

---

## 13. Summary

This detailed design document provides:

✅ **Component Architecture** - Clear structure for Client, Bridge
✅ **Data Structures** - Exact types and fields
✅ **Sequence Diagrams** - Step-by-step interaction flows
✅ **Design Decisions** - Rationale for key choices
✅ **Interface Contracts** - Precise API specifications
✅ **State Management** - State machines and transitions
✅ **Error Handling** - Comprehensive error strategy
✅ **Concurrency Model** - Thread-safety approach
✅ **Integration Points** - How MCP fits into existing Minion
✅ **Alternatives** - Trade-offs and decisions made
✅ **Open Questions** - Issues needing resolution

### Next Steps After Design Review

1. **Review this document** - Gather feedback, answer questions
2. **Resolve open questions** - Make final decisions on pending items
3. **Create detailed tickets** - Break down into implementation tasks
4. **Set up development environment** - Install MCP Go SDK
5. **Begin Phase 1** - Implement MCP Client Manager

**Estimated Review Time**: 1-2 hours
**Estimated Q&A**: 1 day

---

**Document Status**: ✅ Ready for Review (Updated - Client Only)
**Last Updated**: 2025-12-17
**Reviewers**: [To be assigned]
**Approval Required Before**: Implementation start
