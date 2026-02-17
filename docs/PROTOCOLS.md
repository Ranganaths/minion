# Protocol Support

Minion supports three major agent protocols for interoperability and integration:

| Protocol | Purpose | Organization |
|----------|---------|--------------|
| **A2A** | Agent-to-Agent communication | Google/Linux Foundation |
| **AG-UI** | Agent-User Interface streaming | CopilotKit |
| **MCP** | Model Context Protocol for tools | Anthropic |

## A2A Protocol

The Agent-to-Agent (A2A) protocol enables standardized communication between AI agents, regardless of their underlying implementation.

### Overview

```
┌──────────────┐         JSON-RPC / SSE        ┌──────────────┐
│   Agent A    │ ◄────────────────────────────► │   Agent B    │
│              │                                │              │
│ Agent Card   │    tasks/send                  │ Agent Card   │
│ Skills       │    tasks/get                   │ Skills       │
│ Capabilities │    tasks/cancel                │ Capabilities │
└──────────────┘    tasks/subscribe             └──────────────┘
```

### Key Features

- **Agent Discovery** via Agent Cards at `/.well-known/agent.json`
- **Task Management** with states: submitted → working → completed/failed/canceled
- **Streaming Updates** via Server-Sent Events (SSE)
- **JSON-RPC 2.0** for request/response communication

### Creating an A2A Server

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/Ranganaths/minion/core"
    "github.com/Ranganaths/minion/llm"
    "github.com/Ranganaths/minion/models"
    "github.com/Ranganaths/minion/protocols/a2a"
    "github.com/Ranganaths/minion/storage"
)

func main() {
    // Create framework
    framework := core.NewFramework(
        core.WithStorage(storage.NewInMemory()),
        core.WithLLMProvider(llm.NewOpenAI(os.Getenv("OPENAI_API_KEY"))),
    )
    defer framework.Close()

    // Create agent
    ctx := context.Background()
    agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
        Name:        "A2A Agent",
        Description: "An agent accessible via A2A protocol",
        Capabilities: []string{"chat", "analysis"},
    })

    // Configure A2A server
    config := a2a.DefaultServerConfig()
    config.EnableCORS = true
    config.MaxConcurrentTasks = 100

    // Create and start server
    server, _ := a2a.NewA2AServer(framework, agent.ID, "http://localhost:8080", config)

    log.Println("A2A Server: http://localhost:8080")
    log.Println("Agent Card: http://localhost:8080/.well-known/agent.json")
    log.Fatal(server.ListenAndServe(":8080"))
}
```

### Agent Card

The Agent Card is a discovery document that describes the agent's capabilities:

```json
{
  "name": "A2A Agent",
  "description": "An agent accessible via A2A protocol",
  "url": "http://localhost:8080",
  "version": "1.0.0",
  "capabilities": {
    "streaming": true,
    "pushNotifications": false
  },
  "skills": [
    {
      "id": "chat",
      "name": "Chat",
      "description": "Conversational interaction"
    }
  ],
  "provider": {
    "organization": "My Company"
  }
}
```

### JSON-RPC Methods

| Method | Description |
|--------|-------------|
| `tasks/send` | Send a new task and wait for completion |
| `tasks/sendSubscribe` | Send task with SSE streaming |
| `tasks/get` | Get task status and history |
| `tasks/cancel` | Cancel a running task |
| `tasks/resubscribe` | Re-subscribe to task updates |

### Client Usage

```go
package main

import (
    "context"
    "fmt"

    "github.com/Ranganaths/minion/protocols/a2a"
)

func main() {
    // Create client
    config := a2a.DefaultClientConfig()
    client := a2a.NewClient("http://localhost:8080", config)

    // Connect and fetch agent card
    ctx := context.Background()
    client.Connect(ctx)

    card := client.AgentCard()
    fmt.Printf("Connected to: %s\n", card.Name)

    // Send task
    params := a2a.TaskSendParams{
        Message: a2a.NewTextMessage(a2a.MessageRoleUser, "Hello, agent!"),
    }
    task, _ := client.SendTask(ctx, params)
    fmt.Printf("Task %s: %s\n", task.ID, task.Status.State)

    // Stream task updates
    updates, _ := client.SendTaskStream(ctx, params)
    for update := range updates {
        switch update.Type {
        case a2a.TaskUpdateTypeStatus:
            fmt.Printf("Status: %s\n", update.Status.State)
        case a2a.TaskUpdateTypeMessage:
            fmt.Printf("Message: %s\n", update.Message.GetText())
        }
    }
}
```

### Multi-Agent Client

Connect to multiple A2A agents:

```go
// Create multi-agent client
mac := a2a.NewMultiAgentClient(a2a.DefaultClientConfig())

// Add agents
ctx := context.Background()
mac.AddAgent(ctx, "analyst", "http://analyst-agent:8080")
mac.AddAgent(ctx, "coder", "http://coder-agent:8080")
mac.AddAgent(ctx, "researcher", "http://researcher-agent:8080")

// Send to specific agent
task, _ := mac.SendToAgent(ctx, "analyst", params)

// Stream from agent
updates, _ := mac.SendToAgentStream(ctx, "coder", params)

// Discover agents
cards, _ := mac.DiscoverAgents(ctx, []string{
    "http://agent1:8080",
    "http://agent2:8080",
})
```

### Task Lifecycle

```
submitted → working → completed
              │
              ├─────→ failed
              │
              └─────→ canceled
```

## AG-UI Protocol

The Agent-User Interface (AG-UI) protocol enables real-time streaming from agents to frontend applications.

### Overview

```
┌─────────────┐          SSE Events          ┌─────────────┐
│   Agent     │ ─────────────────────────────► │  Frontend   │
│   Server    │  RUN_STARTED                  │  (React)    │
│             │  TEXT_MESSAGE_*               │             │
│             │  TOOL_CALL_*                  │             │
│             │  STATE_DELTA                  │             │
│             │  RUN_FINISHED                 │             │
└─────────────┘                               └─────────────┘
```

### Key Features

- **Server-Sent Events (SSE)** for real-time streaming
- **Token-by-token streaming** for text generation
- **Tool call visualization** with progress updates
- **State synchronization** via JSON Patch (RFC 6902)
- **Frontend SDK compatibility** (CopilotKit, React)

### Creating an AG-UI Server

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/Ranganaths/minion/core"
    "github.com/Ranganaths/minion/llm"
    "github.com/Ranganaths/minion/models"
    "github.com/Ranganaths/minion/protocols/agui"
    "github.com/Ranganaths/minion/storage"
)

func main() {
    // Create framework
    framework := core.NewFramework(
        core.WithStorage(storage.NewInMemory()),
        core.WithLLMProvider(llm.NewOpenAI(os.Getenv("OPENAI_API_KEY"))),
    )
    defer framework.Close()

    // Create agent
    ctx := context.Background()
    agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
        Name:        "AG-UI Agent",
        Description: "An agent with streaming UI support",
        Capabilities: []string{"chat", "streaming"},
    })

    // Configure AG-UI server
    config := agui.DefaultServerConfig()
    config.EnableCORS = true

    // Create and start server
    server, _ := agui.NewAGUIServer(framework, agent.ID, config)

    log.Println("AG-UI Server: http://localhost:8081")
    log.Fatal(server.ListenAndServe(":8081"))
}
```

### Event Types

| Event | Description |
|-------|-------------|
| `RUN_STARTED` | Run has begun |
| `TEXT_MESSAGE_START` | New message started |
| `TEXT_MESSAGE_CONTENT` | Message content (token) |
| `TEXT_MESSAGE_END` | Message completed |
| `TOOL_CALL_START` | Tool invocation started |
| `TOOL_CALL_ARGS` | Tool arguments (streaming) |
| `TOOL_CALL_END` | Tool completed with result |
| `STATE_DELTA` | State change (JSON Patch) |
| `RUN_FINISHED` | Run completed |
| `RUN_ERROR` | Error occurred |

### Custom Handler

Implement custom streaming logic:

```go
type MyHandler struct {
    framework *core.FrameworkImpl
    agentID   string
}

func (h *MyHandler) HandleRun(ctx context.Context, req agui.RunRequest, emitter *agui.EventEmitter) error {
    // Emit run started
    emitter.EmitRunStarted()

    // Create streaming message
    msg := emitter.NewStreamingMessage(agui.RoleAssistant)

    // Stream tokens
    for _, token := range []string{"Hello", " ", "World", "!"} {
        msg.WriteString(token)
        time.Sleep(100 * time.Millisecond)
    }
    msg.End()

    // Emit run finished
    emitter.EmitRunFinished()
    return nil
}

// Use custom handler
handler := &MyHandler{framework: framework, agentID: agent.ID}
server := agui.NewServer(handler, config)
```

### State Management

Synchronize state between agent and frontend:

```go
// Create state manager
stateManager := agui.NewStateManagerWithInitial(emitter, map[string]any{
    "counter": 0,
    "items":   []string{},
})

// Set state (dot notation)
stateManager.Set("user.name", "Alice")
stateManager.Set("user.preferences.theme", "dark")

// Get state
name, _ := stateManager.Get("user.name")

// Apply JSON Patch operations
stateManager.Patch([]agui.JSONPatchOperation{
    {Op: "replace", Path: "/counter", Value: 1},
    {Op: "add", Path: "/items/-", Value: "new item"},
})

// Remove state
stateManager.Remove("user.preferences")
```

### Client-Side Integration

With React and CopilotKit:

```typescript
// Using AG-UI client SDK
import { AGUIClient } from '@ag-ui/client';

const client = new AGUIClient('http://localhost:8081');

// Send run request
const response = await client.run({
  messages: [
    { role: 'user', content: 'Hello!' }
  ],
  state: { counter: 0 }
});

// Handle SSE events
for await (const event of response.events) {
  switch (event.type) {
    case 'TEXT_MESSAGE_CONTENT':
      console.log('Token:', event.delta);
      break;
    case 'STATE_DELTA':
      applyPatch(state, event.delta);
      break;
  }
}
```

### Tool Call Streaming

```go
func (h *MyHandler) HandleRun(ctx context.Context, req agui.RunRequest, emitter *agui.EventEmitter) error {
    emitter.EmitRunStarted()

    // Start tool call
    toolCall := emitter.NewStreamingToolCall("search", "msg-1")

    // Stream arguments
    toolCall.WriteArgs(`{"query": "weather in paris"}`)

    // End with result
    toolCall.End(`{"temperature": 22, "condition": "sunny"}`)

    emitter.EmitRunFinished()
    return nil
}
```

## MCP Protocol

The Model Context Protocol (MCP) allows connecting external tool servers to Minion.

### Overview

```
┌─────────────┐        stdio/HTTP         ┌─────────────┐
│   Minion    │ ◄────────────────────────► │ MCP Server  │
│  Framework  │                           │ (external)  │
│             │   tools/list              │             │
│             │   tools/call              │ GitHub      │
│             │   resources/list          │ Filesystem  │
│             │   prompts/list            │ Database    │
└─────────────┘                           └─────────────┘
```

### Connecting MCP Servers

```go
import (
    "github.com/Ranganaths/minion/mcp/client"
)

// Connect to GitHub MCP server
framework.ConnectMCPServer(ctx, &client.ClientConfig{
    ServerName: "github",
    Command:    "npx",
    Args:       []string{"-y", "@modelcontextprotocol/server-github"},
    Env: map[string]string{
        "GITHUB_TOKEN": os.Getenv("GITHUB_TOKEN"),
    },
})

// Connect to filesystem server
framework.ConnectMCPServer(ctx, &client.ClientConfig{
    ServerName: "filesystem",
    Command:    "npx",
    Args:       []string{"-y", "@modelcontextprotocol/server-filesystem", "/path/to/dir"},
})

// Connect to custom MCP server
framework.ConnectMCPServer(ctx, &client.ClientConfig{
    ServerName: "custom",
    Command:    "./my-mcp-server",
    Args:       []string{"--port", "3000"},
    Env:        map[string]string{"API_KEY": apiKey},
})
```

### Using MCP Tools

```go
// List all tools (including MCP tools)
tools := framework.ListTools()
for _, name := range tools {
    fmt.Println(name)
}

// Execute MCP tool
output, _ := framework.ExecuteTool(ctx, "github_create_issue", map[string]interface{}{
    "owner": "my-org",
    "repo":  "my-repo",
    "title": "Bug report",
    "body":  "Description of the bug",
})
```

### Managing MCP Connections

```go
// List connected servers
servers := framework.ListMCPServers()
// ["github", "filesystem", "custom"]

// Get server status
status := framework.GetMCPServerStatus()
// {"github": {"connected": true, "tools": 15}, ...}

// Refresh tools from server
framework.RefreshMCPTools(ctx, "github")

// Disconnect server
framework.DisconnectMCPServer("custom")
```

### Popular MCP Servers

| Server | Package | Purpose |
|--------|---------|---------|
| GitHub | `@modelcontextprotocol/server-github` | GitHub API operations |
| Filesystem | `@modelcontextprotocol/server-filesystem` | File system access |
| Postgres | `@modelcontextprotocol/server-postgres` | PostgreSQL queries |
| Brave Search | `@modelcontextprotocol/server-brave-search` | Web search |
| Slack | `@modelcontextprotocol/server-slack` | Slack messaging |
| Google Drive | `@modelcontextprotocol/server-gdrive` | Google Drive access |

## Protocol Comparison

| Feature | A2A | AG-UI | MCP |
|---------|-----|-------|-----|
| **Purpose** | Agent communication | UI streaming | Tool integration |
| **Transport** | HTTP + SSE | SSE | stdio/HTTP |
| **Discovery** | Agent Card | - | Tool list |
| **Streaming** | Yes | Yes | No |
| **State sync** | No | JSON Patch | No |
| **Bidirectional** | Yes | No (server→client) | Yes |

## Best Practices

### A2A
- Always implement proper error handling in task handlers
- Use streaming for long-running tasks
- Include meaningful skills in Agent Card
- Implement rate limiting for public endpoints

### AG-UI
- Emit `RUN_STARTED` and `RUN_FINISHED` events
- Use streaming messages for better UX
- Keep state updates atomic and small
- Handle client disconnects gracefully

### MCP
- Set appropriate environment variables securely
- Handle server disconnects and reconnects
- Refresh tools when server capabilities change
- Use server names that are descriptive

## Next Steps

- [Architecture](ARCHITECTURE.md) - System design overview
- [API Reference](API_REFERENCE.md) - Complete API documentation
- [Examples](EXAMPLES.md) - Protocol examples
