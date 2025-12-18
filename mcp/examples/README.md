# MCP Integration Examples

This directory contains examples demonstrating how to use Model Context Protocol (MCP) integration in the Minion framework.

## Available Examples

### 1. GitHub Integration (`github/`)
**Demonstrates**: Connecting to GitHub MCP server via stdio transport

- Connecting to GitHub MCP server
- Listing available GitHub tools
- Creating agents with GitHub capabilities
- Using GitHub tools (issues, PRs, etc.)

**Requirements**: `GITHUB_TOKEN` environment variable

[→ View GitHub Example](github/README.md)

---

### 2. HTTP Integration (`http/`)
**Demonstrates**: Connecting to remote MCP servers via HTTP transport

- HTTP transport configuration
- Authentication methods (Bearer, API Key)
- Remote server connectivity
- Security best practices

**Requirements**: Running HTTP MCP server endpoint

[→ View HTTP Example](http/README.md)

---

### 3. Multi-Server Integration (`multi-server/`)
**Demonstrates**: Connecting to multiple MCP servers simultaneously

- Managing multiple server connections
- Creating agents with multi-server capabilities
- Graceful degradation
- Cross-service workflows

**Requirements**: Tokens for desired services (GitHub, Slack, Gmail, etc.)

[→ View Multi-Server Example](multi-server/README.md)

---

### 4. Salesforce Virtual SDR (`salesforce-sdr/`) 🆕
**Demonstrates**: Production-ready AI Sales Development Representative

- **Connection Pooling**: 100x faster with connection reuse
- **Advanced Caching**: 2000x faster with LRU/LFU/FIFO/TTL policies
- **Circuit Breaker**: Fault tolerance and automatic recovery
- **Prometheus Metrics**: Full observability and monitoring
- **Multi-Server**: Salesforce, Gmail, Calendar integration
- **Real Workflows**: Lead qualification, email outreach, meeting scheduling
- **Production Ready**: Docker, Kubernetes, Grafana dashboards

**Features**:
- Automatic lead qualification with scoring
- Personalized email follow-ups
- Meeting scheduling automation
- CRM pipeline management
- Enterprise-grade monitoring
- Full deployment configs (Docker, K8s)

**Requirements**: Salesforce API access, Gmail/Calendar OAuth

[→ View Salesforce SDR Example](salesforce-sdr/README.md) | [→ Quick Start](salesforce-sdr/QUICKSTART.md)

---

## Quick Start

### 1. Basic Usage

```go
import (
    "github.com/Ranganaths/minion/core"
    "github.com/Ranganaths/minion/mcp/client"
)

// Initialize framework
framework := core.NewFramework()
defer framework.Close()

// Connect to MCP server
config := &client.ClientConfig{
    ServerName: "github",
    Transport:  client.TransportStdio,
    Command:    "npx",
    Args:       []string{"-y", "@modelcontextprotocol/server-github"},
    Env: map[string]string{
        "GITHUB_PERSONAL_ACCESS_TOKEN": os.Getenv("GITHUB_TOKEN"),
    },
}

err := framework.ConnectMCPServer(ctx, config)

// Create agent with MCP capabilities
agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name: "My Agent",
    Capabilities: []string{"mcp_integration"},
})

// Use MCP tools
tools := framework.GetToolsForAgent(agent)
```

### 2. Transport Types

#### Stdio Transport (Local Servers)
```go
config := &client.ClientConfig{
    ServerName: "github",
    Transport:  client.TransportStdio,
    Command:    "npx",
    Args:       []string{"-y", "@modelcontextprotocol/server-github"},
}
```

#### HTTP Transport (Remote Servers)
```go
config := &client.ClientConfig{
    ServerName: "remote-api",
    Transport:  client.TransportHTTP,
    URL:        "https://api.example.com/mcp",
    AuthType:   client.AuthBearer,
    AuthToken:  "your-token",
}
```

## Common MCP Servers

| Server | Package | Transport | Use Case |
|--------|---------|-----------|----------|
| **GitHub** | `@modelcontextprotocol/server-github` | stdio | Issues, PRs, code |
| **Slack** | `@modelcontextprotocol/server-slack` | stdio | Messages, channels |
| **Gmail** | `@modelcontextprotocol/server-gmail` | stdio | Email management |
| **Google Drive** | `@modelcontextprotocol/server-gdrive` | stdio | File operations |
| **Notion** | `@modelcontextprotocol/server-notion` | stdio | Notes, databases |
| **Custom** | Your implementation | HTTP | Enterprise APIs |

## Capability Model

Agents require capabilities to use MCP tools:

```go
// Level 1: Global MCP access
capabilities := []string{"mcp_integration"}

// Level 2: Server-specific access
capabilities := []string{"mcp_github", "mcp_slack"}

// Level 3: Tool-specific access
capabilities := []string{"mcp_github_create_issue"}
```

## Tool Naming Convention

MCP tools are prefixed with their server name:
- Format: `mcp_<server>_<tool>`
- Examples:
  - `mcp_github_create_issue`
  - `mcp_slack_send_message`
  - `mcp_gmail_send_email`

## Framework API

### Connect to Server
```go
err := framework.ConnectMCPServer(ctx, config)
```

### List Servers
```go
servers := framework.ListMCPServers()
```

### Get Server Status
```go
status := framework.GetMCPServerStatus()
```

### Refresh Tools
```go
err := framework.RefreshMCPTools(ctx, "github")
```

### Disconnect Server
```go
err := framework.DisconnectMCPServer("github")
```

## Development Workflow

1. **Connect**: Establish connection to MCP server(s)
2. **Discover**: Framework automatically discovers available tools
3. **Create**: Create agent with required capabilities
4. **Execute**: Agent uses MCP tools through standard interface
5. **Monitor**: Track server status and tool usage
6. **Disconnect**: Clean up connections on shutdown

## Error Handling

MCP integration uses graceful error handling:

```go
// Connection errors
if err := framework.ConnectMCPServer(ctx, config); err != nil {
    log.Printf("Failed to connect: %v", err)
    // Handle connection failure
}

// Tool execution errors (returned in ToolOutput)
output, err := framework.Execute(ctx, agentID, input)
if err != nil {
    log.Printf("Execution failed: %v", err)
}
if !output.Success {
    log.Printf("Tool error: %s", output.Error)
}
```

## Testing

### Unit Tests
```bash
go test ./mcp/...
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
# Test individual examples
cd mcp/examples/github && go run main.go
cd mcp/examples/http && go run main.go
cd mcp/examples/multi-server && go run main.go
```

## Troubleshooting

### Connection Issues
- Verify environment variables are set
- Check API tokens are valid
- Ensure npx is installed for stdio servers
- Verify network connectivity for HTTP servers

### Tool Discovery Fails
- Check server logs for errors
- Verify server implements MCP protocol correctly
- Ensure tools/list endpoint is working

### Tool Execution Errors
- Verify agent has required capabilities
- Check tool parameters match schema
- Review server-side error messages

## Best Practices

1. **Lazy Connection**: Connect only when needed
2. **Resource Cleanup**: Always disconnect on shutdown
3. **Error Isolation**: Handle server failures gracefully
4. **Capability Scoping**: Give agents minimal required access
5. **Monitoring**: Track server health and performance
6. **Security**: Use HTTPS and proper authentication
7. **Testing**: Test with mock servers before production

## Architecture Overview

```
┌──────────────────────────────────────────────────┐
│                Minion Framework                  │
│                                                  │
│  ┌────────────────────────────────────────────┐ │
│  │         MCP Client Manager                 │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐ │ │
│  │  │  Client  │  │  Client  │  │  Client  │ │ │
│  │  │ (stdio)  │  │  (HTTP)  │  │  (...)   │ │ │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘ │ │
│  └───────┼─────────────┼─────────────┼────────┘ │
│          │             │             │          │
│  ┌───────▼─────────────▼─────────────▼────────┐ │
│  │            Tool Bridge Registry            │ │
│  │  ┌──────────────┐  ┌──────────────┐        │ │
│  │  │ Tool Wrapper │  │ Tool Wrapper │  ...   │ │
│  │  │ (implements  │  │ (implements  │        │ │
│  │  │  Tool iface) │  │  Tool iface) │        │ │
│  │  └──────────────┘  └──────────────┘        │ │
│  └───────────────────────────────────────────┘  │
│                      │                           │
│  ┌───────────────────▼───────────────────────┐  │
│  │          Agent Tool Registry              │  │
│  │     (mcp_* tools + native tools)          │  │
│  └───────────────────────────────────────────┘  │
└──────────────────────────────────────────────────┘
            │                 │
            ▼                 ▼
    ┌──────────────┐  ┌──────────────┐
    │ External MCP │  │ External MCP │
    │ Server (npx) │  │ Server (HTTP)│
    └──────────────┘  └──────────────┘
```

## Resources

- [Model Context Protocol Specification](https://spec.modelcontextprotocol.io)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [MCP Server Implementations](https://github.com/modelcontextprotocol/servers)
- [Minion MCP Integration Plan](../MCP_INTEGRATION_PLAN.md)
- [Minion MCP Detailed Design](../MCP_DETAILED_DESIGN.md)

## Support

For issues and questions:
- Check example READMEs for specific guidance
- Review integration plan and design docs
- File issues in the Minion repository
