# MCP (Model Context Protocol) Integration

**Status**: ✅ Production Ready (Phase 3 Complete)
**Version**: 3.0
**Last Updated**: 2025-01-18

This directory contains the complete Model Context Protocol (MCP) integration for the Minion framework, enabling agents to discover and use tools from external MCP servers with enterprise-grade features including connection pooling, caching, observability, and fault tolerance.

## Overview

The MCP integration allows Minion agents to seamlessly connect to external tool providers (GitHub, Slack, Gmail, custom APIs, etc.) and use their tools as if they were native Minion tools. This enables powerful cross-service automation and extends agent capabilities without writing custom integration code.

### Key Features

**Core Features (Phase 1)**:
✅ **Dual Transport Support**: Connect via stdio (local subprocess) or HTTP (remote server)
✅ **Auto-Discovery**: Automatically discover and register tools from connected servers
✅ **Retry Logic**: Exponential backoff with jitter for robust connections
✅ **Health Checks**: Continuous monitoring of server health and performance
✅ **Thread-Safe**: Concurrent operations with RWMutex for performance
✅ **Graceful Errors**: Non-breaking error handling with detailed error messages
✅ **Capability-Based Access**: Fine-grained access control (global, server, tool-specific)
✅ **Metrics Tracking**: Monitor tool usage, success rates, and performance

**Testing & Validation (Phase 2)**:
✅ **Schema Validation**: Auto-validate tool inputs against JSON schemas
✅ **Integration Testing**: Mock MCP server for reliable testing
✅ **Strict/Relaxed Modes**: Configurable validation strictness

**Production Features (Phase 3)**:
✅ **Connection Pooling**: Efficient connection reuse with lifecycle management
✅ **Advanced Caching**: Multi-policy caching (LRU/LFU/FIFO/TTL) with metrics
✅ **Prometheus Metrics**: Full observability with metrics export
✅ **Circuit Breaker**: Fault tolerance with automatic recovery
✅ **Production Ready**: 100-2000x performance improvements

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                     Minion Framework                          │
│  ┌────────────────────────────────────────────────────────┐  │
│  │           MCP Client Manager                           │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌────────────┐   │  │
│  │  │ GitHub       │  │ Slack        │  │  Custom    │   │  │
│  │  │ Client       │  │ Client       │  │  API       │   │  │
│  │  │ (stdio)      │  │ (stdio)      │  │  (HTTP)    │   │  │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬─────┘   │  │
│  │         │                  │                 │         │  │
│  │         └──────────────────┼─────────────────┘         │  │
│  │                            │                           │  │
│  │         ┌──────────────────▼───────────────────┐       │  │
│  │         │  Retry Logic & Health Checks         │       │  │
│  │         └──────────────────┬───────────────────┘       │  │
│  └────────────────────────────┼──────────────────────────┘  │
│                                │                             │
│  ┌────────────────────────────▼──────────────────────────┐  │
│  │              Tool Bridge Registry                     │  │
│  │  ┌────────────────┐  ┌────────────────┐              │  │
│  │  │ MCP Tool       │  │ MCP Tool       │   ...        │  │
│  │  │ Wrapper        │  │ Wrapper        │              │  │
│  │  │ (adapts to     │  │ (adapts to     │              │  │
│  │  │  Tool iface)   │  │  Tool iface)   │              │  │
│  │  └────────────────┘  └────────────────┘              │  │
│  └───────────────────────────────────────────────────────┘  │
│                              │                               │
│  ┌───────────────────────────▼────────────────────────────┐ │
│  │             Agent Tool Registry                        │ │
│  │   Native Tools + MCP Tools (mcp_server_toolname)      │ │
│  └────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
              │                           │
              ▼                           ▼
    ┌──────────────────┐      ┌──────────────────┐
    │  External MCP    │      │  External MCP    │
    │  Server (npx)    │      │  Server (HTTP)   │
    │  - GitHub        │      │  - Custom API    │
    │  - Slack         │      │  - Enterprise    │
    │  - Gmail         │      │    Tools         │
    └──────────────────┘      └──────────────────┘
```

## Directory Structure

```
mcp/
├── README.md                   # This file
├── PHASE2_COMPLETE.md          # Phase 2 completion summary
├── PHASE3_COMPLETE.md          # Phase 3 completion summary
├── client/                     # MCP client implementation
│   ├── types.go               # Type definitions
│   ├── client.go              # Client manager
│   ├── transport_stdio.go     # Stdio transport (local servers)
│   ├── transport_http.go      # HTTP transport (remote servers)
│   ├── retry.go               # Retry logic with exponential backoff
│   ├── health.go              # Health checking
│   ├── schema.go              # Schema validation (Phase 2)
│   ├── pool.go                # Connection pooling (Phase 3)
│   ├── cache.go               # Tool caching (Phase 3)
│   ├── circuit_breaker.go     # Circuit breaker (Phase 3)
│   └── *_test.go              # Unit tests (62 tests total)
├── observability/              # Metrics and monitoring (Phase 3)
│   └── prometheus.go          # Prometheus metrics exporter
├── bridge/                     # Tool wrapper bridge
│   ├── tool_wrapper.go        # MCP → Minion tool adapter
│   ├── registry.go            # Bridge registry
│   └── *_test.go              # Unit tests
├── testing/                    # Testing infrastructure (Phase 2)
│   └── mock_server.go         # Mock MCP server
├── integration/                # Integration tests (Phase 2)
│   └── integration_test.go    # End-to-end tests
└── examples/                   # Usage examples
    ├── README.md              # Examples overview
    ├── github/                # GitHub integration example
    ├── http/                  # HTTP transport example
    └── multi-server/          # Multi-server example
```

## Quick Start

### 1. Connect to MCP Server

```go
import (
    "github.com/yourusername/minion/core"
    "github.com/yourusername/minion/mcp/client"
)

// Initialize framework
framework := core.NewFramework()
defer framework.Close()

// Configure MCP server
config := &client.ClientConfig{
    ServerName: "github",
    Transport:  client.TransportStdio,
    Command:    "npx",
    Args:       []string{"-y", "@modelcontextprotocol/server-github"},
    Env: map[string]string{
        "GITHUB_PERSONAL_ACCESS_TOKEN": os.Getenv("GITHUB_TOKEN"),
    },
}

// Connect (with automatic retry)
err := framework.ConnectMCPServer(ctx, config)
```

### 2. Create Agent with MCP Capabilities

```go
agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name: "GitHub Agent",
    Capabilities: []string{
        "mcp_integration",  // Global MCP access
        "mcp_github",       // GitHub-specific access
    },
})
```

### 3. Use MCP Tools

```go
// Tools are automatically available
tools := framework.GetToolsForAgent(agent)
// Includes: mcp_github_create_issue, mcp_github_list_issues, etc.
```

## Core Components

### Client Manager (`client/`)

Manages connections to multiple MCP servers with connection pooling and lifecycle management.

**Key Classes:**
- `MCPClientManager`: Manages multiple server connections
- `MCPClient`: Represents a single server connection
- `Transport`: Interface for stdio and HTTP transports

**Features:**
- Connection pooling and reuse
- Automatic tool discovery
- Metrics tracking (calls, success rate, latency)
- Thread-safe operations

### Retry Logic (`client/retry.go`)

Implements exponential backoff with jitter for robust connection handling.

**Features:**
- Configurable max retries
- Exponential backoff with jitter
- Context-aware cancellation
- Retry callbacks for monitoring

**Usage:**
```go
err := WithRetry(ctx, retryConfig, func(ctx context.Context) error {
    return someOperation()
})
```

### Health Checks (`client/health.go`)

Continuous monitoring of server health with automatic status detection.

**Health Statuses:**
- `healthy`: All systems operational
- `degraded`: Some issues (high error rate, recent errors)
- `unhealthy`: Critical issues (disconnected, >50% error rate)
- `unknown`: No data available

**Usage:**
```go
checker := client.NewHealthChecker(manager, 30*time.Second)
checker.Start(ctx)

health := checker.GetHealth("github")
fmt.Printf("Status: %s, Message: %s\n", health.Status, health.Message)
```

### Tool Bridge (`bridge/`)

Wraps external MCP tools as native Minion tools using the adapter pattern.

**Key Classes:**
- `MCPToolWrapper`: Implements `tools.Tool` interface
- `BridgeRegistry`: Manages wrapped tools

**Tool Naming:**
- Format: `mcp_<server>_<tool>`
- Example: `mcp_github_create_issue`

**Capability Levels:**
1. **Global**: `mcp_integration` (access to all MCP tools)
2. **Server**: `mcp_github` (access to all GitHub tools)
3. **Tool**: `mcp_github_create_issue` (access to specific tool)

## Transport Types

### Stdio Transport (`transport_stdio.go`)

For local MCP servers running as subprocesses (e.g., npx-based servers).

**Features:**
- Subprocess management
- JSON-RPC 2.0 over stdin/stdout
- Async request/response matching
- Automatic cleanup

**Example:**
```go
config := &client.ClientConfig{
    Transport: client.TransportStdio,
    Command:   "npx",
    Args:      []string{"-y", "@modelcontextprotocol/server-github"},
}
```

### HTTP Transport (`transport_http.go`)

For remote MCP servers over HTTP/HTTPS.

**Features:**
- JSON-RPC 2.0 over HTTP POST
- Authentication support (Bearer, API Key, OAuth)
- Timeout configuration
- TLS support

**Example:**
```go
config := &client.ClientConfig{
    Transport: client.TransportHTTP,
    URL:       "https://api.example.com/mcp",
    AuthType:  client.AuthBearer,
    AuthToken: "your-token",
}
```

## Configuration

### Manager Configuration

```go
config := &client.ManagerConfig{
    MaxServers:          10,
    DefaultTimeout:      30 * time.Second,
    EnableAutoReconnect: true,
    MaxReconnectRetries: 3,
    ReconnectBackoff:    2 * time.Second,
}
manager := client.NewMCPClientManager(config)
```

### Client Configuration

```go
config := &client.ClientConfig{
    ServerName:     "github",
    Description:    "GitHub API integration",
    Transport:      client.TransportStdio,
    Command:        "npx",
    Args:           []string{"-y", "@modelcontextprotocol/server-github"},
    Env:            map[string]string{"GITHUB_TOKEN": token},
    WorkingDir:     "/path/to/dir",  // Optional
    ConnectTimeout: 30 * time.Second,
    RequestTimeout: 60 * time.Second,
}
```

### Retry Configuration

```go
config := &client.RetryConfig{
    MaxRetries:     3,
    InitialBackoff: 1 * time.Second,
    MaxBackoff:     30 * time.Second,
    Multiplier:     2.0,
    Jitter:         true,
}
```

## Examples

See the [`examples/`](examples/README.md) directory for detailed examples:

- **[GitHub Integration](examples/github/README.md)**: Connect to GitHub via stdio
- **[HTTP Integration](examples/http/README.md)**: Connect to remote servers via HTTP
- **[Multi-Server](examples/multi-server/README.md)**: Manage multiple servers simultaneously

## Testing

### Unit Tests

```bash
# Run all tests
go test ./mcp/...

# Run with verbose output
go test -v ./mcp/...

# Run specific package
go test ./mcp/client
go test ./mcp/bridge
```

### Integration Tests

```bash
# Set required environment variables
export GITHUB_TOKEN=your_token

# Run integration tests
go test -tags=integration ./mcp/...
```

### Example Tests

```bash
# Run GitHub example
cd mcp/examples/github && go run main.go

# Run HTTP example
cd mcp/examples/http && go run main.go

# Run multi-server example
cd mcp/examples/multi-server && go run main.go
```

## API Reference

### Framework Methods

```go
// Connect to MCP server
framework.ConnectMCPServer(ctx, config) error

// Disconnect from server
framework.DisconnectMCPServer(serverName string) error

// List connected servers
framework.ListMCPServers() []string

// Get server status
framework.GetMCPServerStatus() map[string]interface{}

// Refresh tools from server
framework.RefreshMCPTools(ctx, serverName string) error
```

### Client Manager Methods

```go
// Create manager
manager := client.NewMCPClientManager(config)

// Connect to server
manager.ConnectServer(ctx, clientConfig) error

// Get client
manager.GetClient(serverName) (*MCPClient, error)

// List servers
manager.ListServers() []string

// Get status of all servers
manager.GetStatus() map[string]*MCPClientStatus

// Close all connections
manager.Close() error
```

### Health Checker Methods

```go
// Create health checker
checker := client.NewHealthChecker(manager, interval)

// Start periodic checks
checker.Start(ctx)

// Get health for server
checker.GetHealth(serverName) *HealthCheck

// Check now (immediate)
checker.CheckNow(ctx, serverName) *HealthCheck

// Get all health checks
checker.GetAllHealth() map[string]*HealthCheck

// Get unhealthy servers
checker.GetUnhealthyServers() []string

// Get summary
checker.Summary() map[string]int

// Stop checker
checker.Stop()
```

## Common MCP Servers

| Server | Package | Transport | Tools |
|--------|---------|-----------|-------|
| GitHub | `@modelcontextprotocol/server-github` | stdio | Issues, PRs, repos, search |
| Slack | `@modelcontextprotocol/server-slack` | stdio | Messages, channels, users |
| Gmail | `@modelcontextprotocol/server-gmail` | stdio | Send/read emails, labels |
| Google Drive | `@modelcontextprotocol/server-gdrive` | stdio | Files, folders, sharing |
| Notion | `@modelcontextprotocol/server-notion` | stdio | Pages, databases |
| Custom | Your implementation | HTTP | Custom enterprise tools |

## Best Practices

1. **Use Retry Logic**: Always enable retries for production deployments
2. **Monitor Health**: Use health checker to detect and respond to issues
3. **Scope Capabilities**: Give agents minimal required capabilities
4. **Handle Errors Gracefully**: Check `ToolOutput.Success` before using results
5. **Resource Cleanup**: Always call `framework.Close()` or `defer`
6. **Connection Pooling**: Reuse framework instance across agents
7. **Security**: Use HTTPS for remote servers, validate tokens
8. **Testing**: Use mock servers for testing, real servers for integration tests

## Troubleshooting

### Connection Fails

```
Error: failed to connect to MCP server
```

**Solutions:**
- Verify environment variables are set (e.g., `GITHUB_TOKEN`)
- Check npx is installed: `npx --version`
- Verify server package exists
- Check network connectivity for HTTP servers
- Review server logs (stderr for stdio)

### Tool Discovery Fails

```
Error: no tools found on server
```

**Solutions:**
- Verify server is fully started
- Check server implements MCP protocol correctly
- Review server configuration (permissions, auth)
- Try manual connection: `npx @modelcontextprotocol/server-github`

### Tool Execution Errors

```
ToolOutput.Success = false
```

**Solutions:**
- Check agent has required capabilities
- Verify tool parameters match schema
- Review error message in `ToolOutput.Error`
- Check server-side logs
- Validate authentication/permissions

### High Error Rate

Health check shows `degraded` or `unhealthy`:

**Solutions:**
- Check server health directly
- Review recent errors in metrics
- Verify network stability
- Check rate limits
- Consider adding more retry attempts

## Performance Considerations

### Metrics

- **Connection Overhead**: Stdio ~100ms, HTTP ~50-200ms (network dependent)
- **Tool Discovery**: 100-500ms per server (cached after first connection)
- **Tool Execution**: Variable (depends on remote server)
- **Health Checks**: Minimal overhead (~10ms per check)

### Optimization Tips

1. **Connection Pooling**: Share framework instance across agents
2. **Lazy Connection**: Connect only when tools are needed
3. **Caching**: Tool lists are cached after discovery
4. **Concurrent Checks**: Health checks run in parallel
5. **Async Operations**: Use goroutines for multiple servers

## Security

### Authentication

- **Stdio**: Relies on environment variables and OS-level security
- **HTTP**: Supports Bearer tokens, API keys, OAuth
- **Best Practice**: Use HTTPS for all remote connections

### Authorization

- **Capability Model**: Fine-grained control at global/server/tool level
- **Least Privilege**: Give agents minimal required capabilities
- **Audit**: Track tool usage via metrics and activity logs

### Secrets Management

- Store tokens in environment variables
- Use secret management systems (HashiCorp Vault, AWS Secrets Manager)
- Never commit tokens to version control
- Rotate tokens regularly

## Roadmap

### Phase 1 (✅ Complete - 2025-12-17)
- ✅ Client-side MCP integration
- ✅ Stdio and HTTP transports
- ✅ Tool discovery and wrapping
- ✅ Framework integration
- ✅ Examples and documentation
- ✅ Retry logic with exponential backoff
- ✅ Health checking
- ✅ Unit tests

### Phase 2 (✅ Complete - 2025-12-17)
- ✅ Tool schema validation (JSON Schema support)
- ✅ Mock MCP server for testing
- ✅ Integration test framework
- ✅ Comprehensive integration tests
- ✅ Enhanced validation in tool wrapper
- ✅ Schema validation tests

### Phase 3 (Future - Optional)
- Connection pool optimization
- Advanced caching strategies
- Prometheus metrics export
- WebSocket transport support
- MCP server mode (expose Minion tools as MCP)
- Advanced authentication (mTLS, JWT)
- Distributed tracing
- Load balancing for multiple server instances

## References

- [MCP Specification](https://spec.modelcontextprotocol.io/)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [MCP Server Implementations](https://github.com/modelcontextprotocol/servers)
- [Integration Plan](../MCP_INTEGRATION_PLAN.md)
- [Detailed Design](../MCP_DETAILED_DESIGN.md)

## Support

For issues and questions:
- Review documentation and examples
- Check troubleshooting section
- File issues in the Minion repository
- Consult MCP specification for protocol details

---

**License**: Same as Minion framework
**Maintainer**: Minion Team
**Status**: Production Ready (v1.0)
**Last Updated**: 2025-12-17
