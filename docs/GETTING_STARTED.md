# Getting Started with Minion

This guide walks you through installing Minion and creating your first AI agent.

## Prerequisites

- **Go 1.24+** - [Download Go](https://golang.org/dl/)
- **LLM API key** - OpenAI, Anthropic, or local Ollama

## Installation

```bash
go get github.com/Ranganaths/minion
```

## Quick Start

### 1. Create Your Project

```bash
mkdir myagent && cd myagent
go mod init myagent
go get github.com/Ranganaths/minion
```

### 2. Set API Key

```bash
# OpenAI
export OPENAI_API_KEY="sk-..."

# Or Anthropic
export ANTHROPIC_API_KEY="sk-ant-..."

# Or use local Ollama (no key needed)
ollama serve
```

### 3. Create Your First Agent

Create `main.go`:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/Ranganaths/minion/core"
    "github.com/Ranganaths/minion/llm"
    "github.com/Ranganaths/minion/models"
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
        Name:        "Assistant",
        Description: "A helpful AI assistant",
    })

    // Activate agent
    activeStatus := models.StatusActive
    framework.UpdateAgent(ctx, agent.ID, &models.UpdateAgentRequest{
        Status: &activeStatus,
    })

    // Execute
    output, _ := framework.Execute(ctx, agent.ID, &models.Input{
        Raw: "What is the capital of France?",
    })

    fmt.Println(output.Result)
}
```

### 4. Run

```bash
go run main.go
# Output: Paris is the capital of France.
```

## Core Concepts

### Framework

The Framework is the central component that manages agents, tools, and execution:

```go
framework := core.NewFramework(
    core.WithStorage(storage.NewInMemory()),       // Storage backend
    core.WithLLMProvider(llm.NewOpenAI(apiKey)),   // LLM provider
    core.WithToolRegistry(tools.NewRegistry()),    // Tool registry
)
defer framework.Close()
```

### Agents

Agents are AI entities with configurable behavior:

```go
agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name:         "Analyst",
    Description:  "A data analysis agent",
    BehaviorType: "default",           // Or "react" for ReAct reasoning
    Config: models.AgentConfig{
        LLMProvider: "openai",
        LLMModel:    "gpt-4",
        Temperature: 0.7,
        MaxTokens:   2048,
    },
    Capabilities: []string{"data_analysis", "visualization"},
})
```

**Agent Status:**
- `draft` - Initial state, not executable
- `active` - Ready for execution
- `inactive` - Temporarily disabled
- `archived` - Soft deleted

### Execution

Execute an agent with input:

```go
output, err := framework.Execute(ctx, agent.ID, &models.Input{
    Raw:  "Analyze this data...",
    Type: "text",
    Context: map[string]interface{}{
        "data": myData,
    },
})

fmt.Println(output.Result)
fmt.Println(output.Metadata["tokens_used"])
```

### Tools

Agents can use tools to perform actions:

```go
// Register a custom tool
framework.RegisterTool(&MyCustomTool{})

// Execute a tool
output, err := framework.ExecuteTool(ctx, "my_tool", map[string]interface{}{
    "param1": "value1",
})
```

### LLM Providers

Switch between providers:

```go
// OpenAI
provider := llm.NewOpenAI(os.Getenv("OPENAI_API_KEY"))

// Anthropic
provider := llm.NewAnthropic(os.Getenv("ANTHROPIC_API_KEY"))

// Ollama (local)
provider := llm.NewOllama("http://localhost:11434")

// With tool support
provider := llm.NewOpenAIWithTools(os.Getenv("OPENAI_API_KEY"))
```

## Common Patterns

### Agent with Tools

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/Ranganaths/minion/core"
    "github.com/Ranganaths/minion/llm"
    "github.com/Ranganaths/minion/models"
    "github.com/Ranganaths/minion/storage"
)

func main() {
    // Create framework with tool-enabled provider
    framework := core.NewFramework(
        core.WithStorage(storage.NewInMemory()),
        core.WithLLMProvider(llm.NewOpenAIWithTools(os.Getenv("OPENAI_API_KEY"))),
    )
    defer framework.Close()

    ctx := context.Background()

    // Create agent with tool capabilities
    agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
        Name:         "Tool Agent",
        Description:  "An agent that uses tools",
        Capabilities: []string{"file_operations", "web"},
    })

    // List available tools
    tools := framework.GetToolsForAgent(agent)
    fmt.Printf("Available tools: %d\n", len(tools))

    // Execute tool directly
    output, _ := framework.ExecuteTool(ctx, "http_get", map[string]interface{}{
        "url": "https://api.example.com/data",
    })
    fmt.Println(output.Result)
}
```

### MCP Integration

Connect external tools via Model Context Protocol:

```go
package main

import (
    "context"
    "os"

    "github.com/Ranganaths/minion/core"
    "github.com/Ranganaths/minion/llm"
    "github.com/Ranganaths/minion/mcp/client"
    "github.com/Ranganaths/minion/storage"
)

func main() {
    framework := core.NewFramework(
        core.WithStorage(storage.NewInMemory()),
        core.WithLLMProvider(llm.NewOpenAI(os.Getenv("OPENAI_API_KEY"))),
    )
    defer framework.Close()

    ctx := context.Background()

    // Connect to MCP server (e.g., GitHub tools)
    framework.ConnectMCPServer(ctx, &client.ClientConfig{
        ServerName: "github",
        Command:    "npx",
        Args:       []string{"-y", "@modelcontextprotocol/server-github"},
        Env: map[string]string{
            "GITHUB_TOKEN": os.Getenv("GITHUB_TOKEN"),
        },
    })

    // MCP tools are now available
    tools := framework.ListTools()
    for _, name := range tools {
        if name[:7] == "github_" {
            fmt.Println(name)
        }
    }
}
```

### Multi-Agent System

```go
package main

import (
    "context"
    "os"

    "github.com/Ranganaths/minion/agents/multiagent"
    "github.com/Ranganaths/minion/agents/workers"
    "github.com/Ranganaths/minion/llm"
)

func main() {
    provider := llm.NewOpenAI(os.Getenv("OPENAI_API_KEY"))

    // Create orchestrator
    orchestrator := multiagent.NewOrchestrator(provider)

    // Create coordinator with specialized workers
    coordinator := multiagent.NewCoordinator(
        multiagent.WithOrchestrator(orchestrator),
        multiagent.WithWorkers(
            workers.NewCoderWorker(provider),
            workers.NewAnalystWorker(provider),
            workers.NewResearcherWorker(provider),
        ),
    )

    // Execute complex task
    ctx := context.Background()
    result, _ := coordinator.Execute(ctx, "Build a REST API for user management")
    fmt.Println(result)
}
```

### Protocol Server

Expose your agent via A2A or AG-UI protocol:

```go
// A2A Server (Agent-to-Agent)
package main

import (
    "log"
    "github.com/Ranganaths/minion/protocols/a2a"
)

func main() {
    // ... create framework and agent ...

    config := a2a.DefaultServerConfig()
    server, _ := a2a.NewA2AServer(framework, agent.ID, "http://localhost:8080", config)

    log.Println("A2A server at http://localhost:8080")
    log.Println("Agent card: http://localhost:8080/.well-known/agent.json")
    log.Fatal(server.ListenAndServe(":8080"))
}
```

```go
// AG-UI Server (for frontend streaming)
package main

import (
    "log"
    "github.com/Ranganaths/minion/protocols/agui"
)

func main() {
    // ... create framework and agent ...

    config := agui.DefaultServerConfig()
    config.EnableCORS = true
    server, _ := agui.NewAGUIServer(framework, agent.ID, config)

    log.Println("AG-UI server at http://localhost:8081")
    log.Fatal(server.ListenAndServe(":8081"))
}
```

## Configuration

### Agent Configuration

```go
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name:         "My Agent",
    Description:  "Agent description",
    BehaviorType: "default",  // or "react"
    Config: models.AgentConfig{
        LLMProvider: "openai",
        LLMModel:    "gpt-4",
        Temperature: 0.7,         // 0.0-1.0 creativity
        MaxTokens:   2048,        // Max response tokens
        Personality: "professional",
        Language:    "en",
        Custom: map[string]interface{}{
            "custom_setting": "value",
        },
    },
    Capabilities: []string{
        "file_operations",
        "web",
        "data_analysis",
    },
    Metadata: map[string]interface{}{
        "team":    "engineering",
        "version": "1.0",
    },
})
```

### Storage Configuration

```go
// In-Memory (development)
store := storage.NewInMemory()

// PostgreSQL (production)
store, _ := storage.NewPostgreSQL(storage.PostgreSQLConfig{
    Host:     "localhost",
    Port:     5432,
    User:     "minion",
    Password: "password",
    Database: "minion",
})
```

### Observability

```go
import (
    "github.com/Ranganaths/minion/observability"
    "github.com/Ranganaths/minion/metrics"
)

// Enable tracing
observability.InitTracer(observability.Config{
    ServiceName: "my-agent",
    Endpoint:    "http://localhost:4318",  // OTLP endpoint
})

// Enable metrics
metrics.InitMetrics(metrics.Config{
    Endpoint: ":9090",  // Prometheus endpoint
})
```

## Error Handling

```go
output, err := framework.Execute(ctx, agent.ID, input)
if err != nil {
    // Framework-level error
    switch {
    case errors.Is(err, storage.ErrNotFound):
        log.Println("Agent not found")
    case errors.Is(err, llm.ErrRateLimit):
        log.Println("Rate limited, retry later")
    default:
        log.Printf("Execution error: %v", err)
    }
    return
}

// Check metadata for warnings
if output.Metadata != nil {
    if warning, ok := output.Metadata["warning"]; ok {
        log.Printf("Warning: %v", warning)
    }
}
```

## Best Practices

1. **Always close the framework** - Use `defer framework.Close()`
2. **Use contexts** - Pass context for cancellation and tracing
3. **Handle errors** - Check both execution errors and output metadata
4. **Limit capabilities** - Give agents only needed permissions
5. **Configure timeouts** - Set appropriate LLM timeouts
6. **Use structured logging** - Log agent IDs and trace IDs

## Next Steps

- [Architecture](ARCHITECTURE.md) - Deep dive into system design
- [API Reference](API_REFERENCE.md) - Complete API documentation
- [Protocols](PROTOCOLS.md) - A2A, AG-UI, MCP integration
- [Examples](EXAMPLES.md) - More code examples

## Troubleshooting

### "storage not configured"

```go
// Add storage to framework
framework := core.NewFramework(
    core.WithStorage(storage.NewInMemory()),
    // ...
)
```

### "LLM provider not configured"

```go
// Add LLM provider
framework := core.NewFramework(
    core.WithLLMProvider(llm.NewOpenAI(apiKey)),
    // ...
)
```

### "agent is not active"

```go
// Activate the agent
activeStatus := models.StatusActive
framework.UpdateAgent(ctx, agent.ID, &models.UpdateAgentRequest{
    Status: &activeStatus,
})
```

### "invalid behavior type"

```go
// Use "default" or register custom behavior
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    BehaviorType: "default",  // Built-in behavior
    // ...
})
```

## Resources

- [GitHub Repository](https://github.com/Ranganaths/minion)
- [Tutorials](tutorials/)
- [Examples](../examples/)
