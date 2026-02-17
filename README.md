# Minion

A production-ready AI agent framework for Go.

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Minion provides everything you need to build intelligent AI agents: multi-agent orchestration, tool calling, memory systems, protocol support (A2A, AG-UI, MCP), and production-grade observability.

## Features

| Category | Features |
|----------|----------|
| **Agents** | Agent lifecycle, custom behaviors, ReAct reasoning, multi-agent orchestration |
| **LLM Providers** | OpenAI, Anthropic, Ollama, TupleLeap, with failover and load balancing |
| **Tools & Skills** | 80+ built-in tools, custom tool registry, MCP integration, hot-reload skills |
| **Memory** | Buffer, semantic search, knowledge graphs, Neo4j integration |
| **Protocols** | A2A (Google), AG-UI (CopilotKit), MCP (Anthropic) |
| **Chains** | LLM chains, RAG, sequential, router, transform chains with streaming |
| **Observability** | OpenTelemetry tracing, Prometheus metrics, time-travel debugging |
| **Storage** | In-memory, PostgreSQL with transactions |

## Quick Start

```bash
go get github.com/Ranganaths/minion
```

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

    // Create agent
    ctx := context.Background()
    agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
        Name:        "Assistant",
        Description: "A helpful AI assistant",
    })

    // Execute
    output, _ := framework.Execute(ctx, agent.ID, &models.Input{
        Raw: "What is the capital of France?",
    })

    fmt.Println(output.Result)
}
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Application Layer                           │
├─────────────────────────────────────────────────────────────────────┤
│   Protocols          │   Agents            │   Chains               │
│   ├─ A2A             │   ├─ ReAct          │   ├─ LLM Chain         │
│   ├─ AG-UI           │   ├─ Executor       │   ├─ RAG Chain         │
│   └─ MCP             │   └─ Multi-Agent    │   └─ Sequential        │
├─────────────────────────────────────────────────────────────────────┤
│   Core Framework                                                     │
│   ├─ Behaviors       ├─ Tools             ├─ Skills                 │
│   ├─ Registry        ├─ Capability Filter ├─ Hot-Reload             │
│   └─ Execution       └─ 80+ Built-in      └─ Markdown/Native        │
├─────────────────────────────────────────────────────────────────────┤
│   Infrastructure                                                     │
│   ├─ LLM Providers   ├─ Storage           ├─ Memory                 │
│   │  ├─ OpenAI       │  ├─ In-Memory      │  ├─ Buffer              │
│   │  ├─ Anthropic    │  └─ PostgreSQL     │  ├─ Semantic            │
│   │  ├─ Ollama       │                    │  └─ Knowledge Graph     │
│   │  └─ TupleLeap    │                    │                         │
│   ├─ Observability   ├─ Debug             │                         │
│   │  ├─ Tracing      │  ├─ Snapshots      │                         │
│   │  └─ Metrics      │  └─ Time-Travel    │                         │
└─────────────────────────────────────────────────────────────────────┘
```

## Documentation

| Document | Description |
|----------|-------------|
| [Getting Started](docs/GETTING_STARTED.md) | Installation, first agent, core concepts |
| [Architecture](docs/ARCHITECTURE.md) | System design, components, patterns |
| [API Reference](docs/API_REFERENCE.md) | Complete API documentation |
| [Protocols](docs/PROTOCOLS.md) | A2A, AG-UI, MCP integration |
| [Tools Guide](TOOLS_GUIDE.md) | All 80+ tools by domain |
| [Examples](docs/EXAMPLES.md) | Code examples for common use cases |

### Tutorials

1. [Framework Basics](docs/tutorials/01-framework-basics.md)
2. [MCP Integration](docs/tutorials/02-mcp-basics.md)
3. [First MCP Agent](docs/tutorials/03-first-mcp-agent.md)
4. [Advanced MCP](docs/tutorials/04-advanced-mcp-features.md)
5. [Multi-Server](docs/tutorials/05-multi-server-orchestration.md)
6. [Production Deployment](docs/tutorials/06-production-deployment.md)

## Examples

```bash
# Basic agent
go run examples/basic/main.go

# Agent with tools
go run examples/with_tools/main.go

# Multi-agent system
go run examples/multiagent-basic/main.go

# A2A protocol server
go run examples/a2a-server/main.go

# AG-UI protocol server
go run examples/agui-server/main.go
```

## LLM Providers

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

## Multi-Agent System

```go
// Create coordinator with specialized workers
coordinator := multiagent.NewCoordinator(
    multiagent.WithOrchestrator(orchestrator),
    multiagent.WithWorkers(
        workers.NewCoderWorker(llmProvider),
        workers.NewAnalystWorker(llmProvider),
        workers.NewResearcherWorker(llmProvider),
    ),
)

// Execute complex task
result, _ := coordinator.Execute(ctx, "Build a REST API for user management")
```

## Protocol Support

### A2A (Agent-to-Agent)

```go
// Create A2A server
server, _ := a2a.NewA2AServer(framework, agentID, "http://localhost:8080", config)
server.ListenAndServe(":8080")

// Agent card available at: /.well-known/agent.json
// JSON-RPC endpoint: POST /
```

### AG-UI (Agent-User Interface)

```go
// Create AG-UI server for frontend streaming
server, _ := agui.NewAGUIServer(framework, agentID, config)
server.ListenAndServe(":8081")

// SSE streaming endpoint: POST /
```

### MCP (Model Context Protocol)

```go
// Connect to MCP servers
framework.ConnectMCPServer(ctx, &mcp.ServerConfig{
    Name:    "github",
    Command: "npx",
    Args:    []string{"-y", "@modelcontextprotocol/server-github"},
})
```

## Observability

```go
// Enable tracing
tracing.InitTracer(tracing.Config{
    ServiceName: "my-agent",
    Endpoint:    "http://localhost:4318",
})

// Enable metrics
metrics.InitMetrics(metrics.Config{
    Endpoint: ":9090",
})

// Time-travel debugging
recorder := debug.NewRecorder(snapshotStore)
timeline := debug.NewTimeline(recorder)
timeline.StepBackward() // Navigate execution history
```

## Project Structure

```
minion/
├── core/           # Framework core, behaviors, interfaces
├── agents/         # ReAct agent, executor
├── llm/            # LLM providers (OpenAI, Anthropic, etc.)
├── tools/          # Tool registry, 80+ domain tools
├── skills/         # Skill system with hot-reload
├── chain/          # LLM chains, RAG, workflows
├── memory/         # Buffer, semantic, knowledge graph
├── storage/        # In-memory, PostgreSQL
├── protocols/      # A2A, AG-UI implementations
│   ├── a2a/        # Google A2A protocol
│   └── agui/       # CopilotKit AG-UI protocol
├── mcp/            # Model Context Protocol
├── observability/  # Tracing, metrics
├── debug/          # Time-travel debugging
├── workflow/       # DAG-based workflows
├── evaluation/     # Agent evaluation, benchmarks
└── examples/       # Example applications
```

## Requirements

- Go 1.24+
- PostgreSQL (optional, for persistent storage)
- LLM API key (OpenAI, Anthropic, or local Ollama)

## License

MIT License - see [LICENSE](LICENSE) for details.
