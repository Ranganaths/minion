# Minion Framework - Quick Reference Guide

**Quick lookup for common patterns and API calls**

## 📖 Table of Contents

- [Framework Setup](#framework-setup)
- [Agent Creation](#agent-creation)
- [MCP Connections](#mcp-connections)
- [Tool Execution](#tool-execution)
- [Capabilities](#capabilities)
- [Error Handling](#error-handling)
- [Common Patterns](#common-patterns)
- [MCP Servers](#mcp-servers)

---

## Framework Setup

### Initialize Framework

```go
import "github.com/yourusername/minion/core"

framework := core.NewFramework()
defer framework.Close()
```

### With Context

```go
ctx := context.Background()
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
```

---

## Agent Creation

### Basic Agent

```go
agentReq := &models.CreateAgentRequest{
    Name:        "MyAgent",
    Description: "Agent description",
    Capabilities: []string{"basic_operations"},
}

agent, err := framework.CreateAgent(ctx, agentReq)
```

### Agent with Multiple Capabilities

```go
Capabilities: []string{
    "file_operations",
    "http_operations",
    "mcp_github",
    "mcp_slack",
}
```

### Agent with All Access

```go
Capabilities: []string{"*"}  // Caution: Use sparingly!
```

---

## MCP Connections

### Stdio Transport (Local MCP Server)

```go
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
```

### HTTP Transport (Remote MCP Server)

```go
config := &client.ClientConfig{
    ServerName: "api",
    Transport:  client.TransportHTTP,
    URL:        "https://api.example.com/mcp",
    AuthType:   client.AuthBearer,
    AuthToken:  os.Getenv("API_TOKEN"),
}

err := framework.ConnectMCPServer(ctx, config)
```

### Check Connection Status

```go
// List all connected servers
servers := framework.ListMCPServers()

// Get detailed status
status := framework.GetMCPServerStatus()
for name, s := range status {
    fmt.Printf("%s: Connected=%v, Tools=%d\n",
        name, s.Connected, s.ToolsDiscovered)
}
```

### Disconnect

```go
// Disconnect specific server
err := framework.DisconnectMCPServer("github")

// Disconnect all
framework.DisconnectAllMCPServers()
```

---

## Tool Execution

### Execute Tool

```go
input := &models.ToolInput{
    ToolName: "tool_name",
    Params: map[string]interface{}{
        "param1": "value1",
        "param2": 123,
    },
}

output, err := framework.ExecuteTool(ctx, agent.ID, input)
```

### Check Result

```go
if err != nil {
    // Framework-level error
    log.Fatal(err)
}

if !output.Success {
    // Tool-level error
    fmt.Println(output.Error)
} else {
    // Success
    fmt.Println(output.Result)
}
```

### List Available Tools

```go
// For specific agent
tools := framework.GetToolsForAgent(agent)

// Tool details
for _, tool := range tools {
    fmt.Printf("Name: %s\n", tool.Name())
    fmt.Printf("Description: %s\n", tool.Description())
    fmt.Printf("Parameters: %v\n", tool.Parameters())
}
```

---

## Capabilities

### Capability Levels

```go
// Level 1: Global MCP access
[]string{"mcp_integration"}

// Level 2: Server-specific
[]string{"mcp_github", "mcp_slack"}

// Level 3: Tool-specific
[]string{"mcp_github_create_issue", "mcp_slack_post_message"}

// Mix native and MCP
[]string{"file_operations", "mcp_github"}

// All access
[]string{"*"}
```

### Check Agent Capabilities

```go
hasCapability := framework.AgentHasCapability(agent.ID, "mcp_github")
```

---

## Error Handling

### Two-Level Error Checking

```go
output, err := framework.ExecuteTool(ctx, agentID, input)

// Level 1: Framework error
if err != nil {
    switch {
    case errors.Is(err, ErrAgentNotFound):
        // Handle agent not found
    case errors.Is(err, ErrToolNotFound):
        // Handle tool not found
    default:
        // Handle other errors
    }
    return err
}

// Level 2: Tool error
if !output.Success {
    log.Printf("Tool failed: %s", output.Error)
    return fmt.Errorf(output.Error)
}

// Success
return nil
```

### Retry Pattern

```go
func executeWithRetry(fw *core.Framework, agentID string, input *models.ToolInput, maxRetries int) error {
    for attempt := 1; attempt <= maxRetries; attempt++ {
        output, err := fw.ExecuteTool(ctx, agentID, input)
        if err == nil && output.Success {
            return nil
        }

        if attempt < maxRetries {
            time.Sleep(time.Duration(attempt) * time.Second)
        }
    }
    return fmt.Errorf("failed after %d attempts", maxRetries)
}
```

---

## Common Patterns

### Multi-Server Agent

```go
// Connect multiple servers
framework.ConnectMCPServer(ctx, githubConfig)
framework.ConnectMCPServer(ctx, slackConfig)
framework.ConnectMCPServer(ctx, gmailConfig)

// Create agent with all capabilities
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name: "MultiBot",
    Capabilities: []string{
        "mcp_github",
        "mcp_slack",
        "mcp_gmail",
    },
})

// Use tools from different servers
framework.ExecuteTool(ctx, agent.ID, &models.ToolInput{
    ToolName: "mcp_github_create_issue",
    Params:   githubParams,
})

framework.ExecuteTool(ctx, agent.ID, &models.ToolInput{
    ToolName: "mcp_slack_post_message",
    Params:   slackParams,
})
```

### Workflow Orchestration

```go
type Workflow struct {
    framework *core.Framework
    agent     *models.Agent
}

func (w *Workflow) Execute(ctx context.Context) error {
    // Step 1
    output1, err := w.framework.ExecuteTool(ctx, w.agent.ID, step1Input)
    if err != nil || !output1.Success {
        return fmt.Errorf("step 1 failed")
    }

    // Step 2 (uses output from step 1)
    step2Input := &models.ToolInput{
        ToolName: "tool2",
        Params: map[string]interface{}{
            "data": output1.Result,
        },
    }
    output2, err := w.framework.ExecuteTool(ctx, w.agent.ID, step2Input)
    if err != nil || !output2.Success {
        return fmt.Errorf("step 2 failed")
    }

    return nil
}
```

### Parallel Execution

```go
func executeParallel(fw *core.Framework, agentID string, inputs []*models.ToolInput) []error {
    var wg sync.WaitGroup
    errors := make([]error, len(inputs))

    for i, input := range inputs {
        wg.Add(1)
        go func(idx int, in *models.ToolInput) {
            defer wg.Done()
            _, errors[idx] = fw.ExecuteTool(ctx, agentID, in)
        }(i, input)
    }

    wg.Wait()
    return errors
}
```

### Rate Limiting

```go
import "golang.org/x/time/rate"

limiter := rate.NewLimiter(rate.Every(time.Second), 10) // 10 req/sec

func executeWithRateLimit(fw *core.Framework, agentID string, input *models.ToolInput) error {
    if err := limiter.Wait(ctx); err != nil {
        return err
    }
    output, err := fw.ExecuteTool(ctx, agentID, input)
    // ... handle result
}
```

---

## MCP Servers

### GitHub

**Package**: `@modelcontextprotocol/server-github`

**Environment**:
```bash
GITHUB_PERSONAL_ACCESS_TOKEN=ghp_...
```

**Common Tools**:
- `mcp_github_create_issue` - Create GitHub issue
- `mcp_github_create_pr` - Create pull request
- `mcp_github_list_repos` - List repositories
- `mcp_github_get_repo` - Get repository info
- `mcp_github_search_code` - Search code

**Example**:
```go
input := &models.ToolInput{
    ToolName: "mcp_github_create_issue",
    Params: map[string]interface{}{
        "owner": "username",
        "repo":  "repository",
        "title": "Issue title",
        "body":  "Issue description",
    },
}
```

---

### Slack

**Package**: `@modelcontextprotocol/server-slack`

**Environment**:
```bash
SLACK_BOT_TOKEN=xoxb-...
```

**Common Tools**:
- `mcp_slack_post_message` - Post message
- `mcp_slack_list_channels` - List channels
- `mcp_slack_get_channel_history` - Get messages

**Example**:
```go
input := &models.ToolInput{
    ToolName: "mcp_slack_post_message",
    Params: map[string]interface{}{
        "channel": "#general",
        "text":    "Hello from Minion!",
    },
}
```

---

### Gmail

**Package**: `@modelcontextprotocol/server-gmail`

**Environment**:
```bash
GMAIL_CREDENTIALS=/path/to/credentials.json
```

**Common Tools**:
- `mcp_gmail_send_message` - Send email
- `mcp_gmail_get_message` - Get email
- `mcp_gmail_list_messages` - List emails
- `mcp_gmail_search` - Search emails

**Example**:
```go
input := &models.ToolInput{
    ToolName: "mcp_gmail_send_message",
    Params: map[string]interface{}{
        "to":      "user@example.com",
        "subject": "Hello",
        "body":    "Email body",
    },
}
```

---

### Salesforce

**Package**: `@modelcontextprotocol/server-salesforce`

**Environment**:
```bash
SALESFORCE_INSTANCE_URL=https://yourinstance.salesforce.com
SALESFORCE_CLIENT_ID=...
SALESFORCE_CLIENT_SECRET=...
SALESFORCE_USERNAME=...
SALESFORCE_PASSWORD=...
```

**Common Tools**:
- `mcp_salesforce_create_lead` - Create lead
- `mcp_salesforce_update_lead` - Update lead
- `mcp_salesforce_get_lead` - Get lead info
- `mcp_salesforce_create_opportunity` - Create opportunity

---

### Notion

**Package**: `@modelcontextprotocol/server-notion`

**Environment**:
```bash
NOTION_API_KEY=secret_...
```

**Common Tools**:
- `mcp_notion_search` - Search pages
- `mcp_notion_create_page` - Create page
- `mcp_notion_get_page` - Get page
- `mcp_notion_query_database` - Query database

---

## Quick Troubleshooting

### Problem: "Agent not found"

```go
// Store agent reference after creation
agent, _ := framework.CreateAgent(ctx, req)

// Use agent.ID (not agent.Name)
output, _ := framework.ExecuteTool(ctx, agent.ID, input)
```

### Problem: "Tool not available"

```go
// Check agent capabilities
agent := &models.CreateAgentRequest{
    Capabilities: []string{"mcp_github"}, // Must match tool prefix
}

// List what agent can use
tools := framework.GetToolsForAgent(agent)
```

### Problem: "MCP server connection failed"

```go
// Verify npx is available
exec.Command("npx", "--version").Run()

// Check environment variables
token := os.Getenv("GITHUB_TOKEN")
if token == "" {
    log.Fatal("Token not set")
}

// Add timeout
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
```

### Problem: "Tool execution timeout"

```go
// Add timeout to context
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

output, err := framework.ExecuteTool(ctx, agentID, input)
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("Tool timed out")
}
```

---

## Advanced Features (Phase 3)

### Connection Pool

```go
import "github.com/yourusername/minion/mcp/client"

poolConfig := client.DefaultPoolConfig()
poolConfig.MaxOpenConns = 20
pool := client.NewConnectionPool(poolConfig)

// Acquire connection
pooled, _ := pool.Acquire(ctx, "server-name", clientConfig)
defer pool.Release(pooled, "server-name")

// Use connection
client := pooled.GetClient()
```

### Tool Cache

```go
cacheConfig := client.DefaultCacheConfig()
cacheConfig.EvictionPolicy = client.CachePolicyLRU
cache := client.NewToolCache(cacheConfig)

// Check cache
tools, found := cache.Get("server-name")
if !found {
    // Fetch and cache
    tools = fetchTools()
    cache.Set("server-name", tools)
}
```

### Circuit Breaker

```go
cb := client.NewCircuitBreaker(client.DefaultCircuitBreakerConfig())

err := cb.Execute(ctx, func(ctx context.Context) error {
    // Your operation
    return operation()
})

if cb.IsOpen() {
    log.Println("Circuit breaker is open - service unavailable")
}
```

---

## Environment Variables

### Common Variables

```bash
# GitHub
GITHUB_TOKEN=ghp_your_token_here

# Slack
SLACK_BOT_TOKEN=xoxb-your-token-here

# Gmail
GMAIL_CREDENTIALS=/path/to/credentials.json

# Salesforce
SALESFORCE_INSTANCE_URL=https://yourinstance.salesforce.com
SALESFORCE_CLIENT_ID=your_client_id
SALESFORCE_CLIENT_SECRET=your_client_secret
SALESFORCE_USERNAME=your_username
SALESFORCE_PASSWORD=your_password

# Notion
NOTION_API_KEY=secret_your_key_here

# Custom
LOG_LEVEL=info
TIMEOUT=30s
```

---

## Code Templates

### Minimal Agent

```go
package main

import (
    "context"
    "github.com/yourusername/minion/core"
    "github.com/yourusername/minion/models"
)

func main() {
    ctx := context.Background()
    fw := core.NewFramework()
    defer fw.Close()

    agent, _ := fw.CreateAgent(ctx, &models.CreateAgentRequest{
        Name:         "MyAgent",
        Capabilities: []string{"basic_operations"},
    })

    output, _ := fw.ExecuteTool(ctx, agent.ID, &models.ToolInput{
        ToolName: "echo",
        Params:   map[string]interface{}{"message": "Hello"},
    })

    println(output.Result)
}
```

### MCP-Enabled Agent

```go
package main

import (
    "context"
    "os"
    "github.com/yourusername/minion/core"
    "github.com/yourusername/minion/models"
    "github.com/yourusername/minion/mcp/client"
)

func main() {
    ctx := context.Background()
    fw := core.NewFramework()
    defer fw.Close()

    // Connect MCP
    fw.ConnectMCPServer(ctx, &client.ClientConfig{
        ServerName: "github",
        Command:    "npx",
        Args:       []string{"-y", "@modelcontextprotocol/server-github"},
        Env:        map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": os.Getenv("GITHUB_TOKEN")},
    })

    // Create agent
    agent, _ := fw.CreateAgent(ctx, &models.CreateAgentRequest{
        Name:         "GitHubBot",
        Capabilities: []string{"mcp_github"},
    })

    // Use GitHub tool
    fw.ExecuteTool(ctx, agent.ID, &models.ToolInput{
        ToolName: "mcp_github_list_repos",
        Params:   map[string]interface{}{"owner": "username"},
    })
}
```

---

**For more details, see the [full tutorials](README.md)**
