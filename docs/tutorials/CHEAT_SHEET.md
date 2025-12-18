# Minion Framework - Cheat Sheet

**One-page reference for Minion + MCP**

## 🚀 Quick Start

```go
// 1. Initialize
fw := core.NewFramework()
defer fw.Close()

// 2. Create Agent
agent, _ := fw.CreateAgent(ctx, &models.CreateAgentRequest{
    Name: "MyBot",
    Capabilities: []string{"mcp_github"},
})

// 3. Connect MCP
fw.ConnectMCPServer(ctx, &client.ClientConfig{
    ServerName: "github",
    Command: "npx",
    Args: []string{"-y", "@modelcontextprotocol/server-github"},
    Env: map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": token},
})

// 4. Execute Tool
output, _ := fw.ExecuteTool(ctx, agent.ID, &models.ToolInput{
    ToolName: "mcp_github_create_issue",
    Params: map[string]interface{}{"owner": "user", "repo": "repo", "title": "Bug"},
})
```

## 📋 Core API

| Action | Code |
|--------|------|
| **Framework** | `fw := core.NewFramework()` |
| **Create Agent** | `agent, _ := fw.CreateAgent(ctx, req)` |
| **List Tools** | `tools := fw.GetToolsForAgent(agent)` |
| **Execute Tool** | `output, _ := fw.ExecuteTool(ctx, agent.ID, input)` |
| **Connect MCP** | `fw.ConnectMCPServer(ctx, config)` |
| **List Servers** | `servers := fw.ListMCPServers()` |
| **Disconnect** | `fw.DisconnectMCPServer("name")` |

## 🔧 Tool Execution Pattern

```go
input := &models.ToolInput{
    ToolName: "tool_name",
    Params: map[string]interface{}{
        "param1": "value",
        "param2": 123,
    },
}

output, err := fw.ExecuteTool(ctx, agent.ID, input)

// Check errors
if err != nil {
    // Framework error
}
if !output.Success {
    // Tool error: output.Error
} else {
    // Success: output.Result
}
```

## 🎯 Capabilities

```go
// All MCP servers
[]string{"mcp_integration"}

// Specific servers
[]string{"mcp_github", "mcp_slack"}

// Specific tools
[]string{"mcp_github_create_issue"}

// Mix types
[]string{"file_operations", "mcp_github"}

// All access
[]string{"*"}
```

## 🔌 MCP Transports

### Stdio (Local)
```go
config := &client.ClientConfig{
    ServerName: "github",
    Transport: client.TransportStdio,
    Command: "npx",
    Args: []string{"-y", "@modelcontextprotocol/server-github"},
    Env: map[string]string{"TOKEN": token},
}
```

### HTTP (Remote)
```go
config := &client.ClientConfig{
    ServerName: "api",
    Transport: client.TransportHTTP,
    URL: "https://api.example.com/mcp",
    AuthType: client.AuthBearer,
    AuthToken: token,
}
```

## 📊 Popular MCP Servers

| Server | Package | Token Env |
|--------|---------|-----------|
| **GitHub** | `@modelcontextprotocol/server-github` | `GITHUB_PERSONAL_ACCESS_TOKEN` |
| **Slack** | `@modelcontextprotocol/server-slack` | `SLACK_BOT_TOKEN` |
| **Gmail** | `@modelcontextprotocol/server-gmail` | `GMAIL_CREDENTIALS` (file path) |
| **Salesforce** | `@modelcontextprotocol/server-salesforce` | `SALESFORCE_*` (multiple) |
| **Notion** | `@modelcontextprotocol/server-notion` | `NOTION_API_KEY` |

## 🔄 Common Patterns

### Retry Logic
```go
for attempt := 1; attempt <= 3; attempt++ {
    output, err := fw.ExecuteTool(ctx, agentID, input)
    if err == nil && output.Success { return nil }
    time.Sleep(time.Duration(attempt) * time.Second)
}
```

### Rate Limiting
```go
limiter := rate.NewLimiter(rate.Every(time.Second), 10)
limiter.Wait(ctx)
fw.ExecuteTool(ctx, agentID, input)
```

### Parallel Execution
```go
var wg sync.WaitGroup
for _, input := range inputs {
    wg.Add(1)
    go func(in *models.ToolInput) {
        defer wg.Done()
        fw.ExecuteTool(ctx, agentID, in)
    }(input)
}
wg.Wait()
```

### Error Handling
```go
output, err := fw.ExecuteTool(ctx, agentID, input)
switch {
case err != nil:
    log.Printf("Framework error: %v", err)
case !output.Success:
    log.Printf("Tool error: %s", output.Error)
default:
    log.Printf("Success: %v", output.Result)
}
```

## 🚀 Phase 3 Features

### Connection Pool
```go
pool := client.NewConnectionPool(client.DefaultPoolConfig())
pooled, _ := pool.Acquire(ctx, "server", config)
defer pool.Release(pooled, "server")
```

### Tool Cache
```go
cache := client.NewToolCache(client.DefaultCacheConfig())
tools, found := cache.Get("server")
if !found {
    tools = discover()
    cache.Set("server", tools)
}
```

### Circuit Breaker
```go
cb := client.NewCircuitBreaker(client.DefaultCircuitBreakerConfig())
cb.Execute(ctx, func(ctx context.Context) error {
    return operation()
})
if cb.IsOpen() { /* service down */ }
```

## 🎓 Tool Naming

**Format**: `mcp_<server>_<tool>`

**Examples**:
- `mcp_github_create_issue`
- `mcp_slack_send_message`
- `mcp_gmail_send_email`
- `mcp_salesforce_create_lead`

## 🐛 Quick Fixes

| Problem | Solution |
|---------|----------|
| "Agent not found" | Use `agent.ID` not `agent.Name` |
| "Tool not available" | Check `Capabilities` includes tool prefix |
| "npx not found" | Install Node.js 18+ |
| "Auth failed" | Verify environment variable set |
| "Connection timeout" | Add `context.WithTimeout(ctx, 30*time.Second)` |
| "Tool timeout" | Same as above |

## 📦 Imports

```go
import (
    "context"
    "github.com/yourusername/minion/core"
    "github.com/yourusername/minion/models"
    "github.com/yourusername/minion/mcp/client"
)
```

## 🔍 Debugging

```go
// List connected servers
servers := fw.ListMCPServers()
fmt.Println("Servers:", servers)

// Get server status
status := fw.GetMCPServerStatus()
for name, s := range status {
    fmt.Printf("%s: Connected=%v Tools=%d\n",
        name, s.Connected, s.ToolsDiscovered)
}

// List agent tools
tools := fw.GetToolsForAgent(agent)
for _, tool := range tools {
    fmt.Printf("- %s: %s\n", tool.Name(), tool.Description())
}

// Check tool parameters
tool := fw.GetTool("tool_name")
fmt.Println("Params:", tool.Parameters())
```

## 📚 Resources

- **Tutorials**: Start with [Tutorial 1: Framework Basics](01-framework-basics.md)
- **Quick Reference**: [QUICK_REFERENCE.md](QUICK_REFERENCE.md)
- **Examples**: [../../mcp/examples/](../../mcp/examples/)
- **API Docs**: [../api/](../api/)
- **MCP Spec**: https://modelcontextprotocol.io

## 💡 Pro Tips

1. **Always check two error levels**: Framework error (`err`) and tool error (`!output.Success`)
2. **Use context timeouts**: Prevent hanging operations
3. **Store agent.ID**: Don't lose agent reference
4. **Defer cleanup**: Always `defer framework.Close()`
5. **Check capabilities**: List tools before executing
6. **Use retries**: Network operations can be flaky
7. **Rate limit**: Respect API limits
8. **Log everything**: Debugging is easier with logs

---

**Print this page for quick reference while coding! 📄**

**For detailed tutorials, see [README.md](README.md)**
