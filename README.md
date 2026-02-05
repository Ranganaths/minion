# 🤖 Minion

> A powerful, modular agent framework for building AI-powered agents in Go

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Minion** is a standalone, production-ready agent framework that provides everything you need to build intelligent AI agents with custom behaviors, tools, and capabilities.

## 🎯 What is Minion?

Minion is a complete system for creating, managing, and executing AI agents. It provides:

- **Complete Agent Lifecycle** - Create, configure, execute, and monitor agents
- **Pluggable Architecture** - Swap storage, LLM providers, and behaviors easily
- **Production Ready** - Thread-safe, observable, and battle-tested patterns
- **Framework Agnostic** - Use standalone or integrate with existing systems

Minion is a standalone framework that can be used in any Go project for building multi-agent AI systems.

## ✨ Features

### Core Framework
- 🤖 **Agent Management** - Complete CRUD operations with metrics and activity tracking
- 🧠 **Pluggable Behaviors** - Define custom processing logic for specialized agents
- 🛠️ **Tool System** - Extensible tools with capability-based filtering
- 💾 **Storage Abstraction** - In-memory, PostgreSQL (with full transaction support), or custom backends
- 📊 **Built-in Observability** - Metrics, activity logs, and performance tracking
- ⚡ **Thread-Safe** - Concurrent operations with proper synchronization
- 🎨 **Highly Extensible** - Easy to add new behaviors, tools, and providers

### Multi-Agent System
- 🤝 **Multi-Agent Collaboration** - Research-based orchestrator pattern with specialized workers
- 🔄 **KQML Protocol** - Industry-standard inter-agent communication
- 📋 **Task Decomposition** - LLM-powered planning and task breakdown
- 👷 **Specialized Workers** - Coder, Analyst, Researcher, Writer, Reviewer agents

### LLM Providers
- 🔌 **OpenAI** - GPT-4, GPT-3.5-turbo support
- 🔌 **Anthropic** - Claude models support
- 🔌 **TupleLeap** - TupleLeap AI integration
- 🔌 **Custom Providers** - Easy to add your own

### Production Features
- 🔒 **HTTP Authentication** - Bearer, API Key, and OAuth support for MCP
- 🔄 **Connection Pooling** - Efficient resource management with graceful shutdown
- ✅ **Schema Validation** - JSON Schema validation with regex pattern support
- 🛡️ **Error Handling** - Safe environment config with error returns (no panics)
- 📈 **Chain System** - LangChain-style chains for RAG and workflows
- 🔄 **LLM Request Validation** - Built-in validation for temperature, tokens, and model
- 🏥 **Health Checks** - Provider health verification with `HealthCheckProvider` interface
- 🛡️ **Safe Type Assertions** - Helper functions to prevent runtime panics
- ⚡ **Goroutine Safety** - Context-aware streaming with proper cleanup

### Observability & Tracing (NEW!)
- 📊 **OpenTelemetry Tracing** - Full distributed tracing with Jaeger/OTLP export
- 📈 **Prometheus Metrics** - Production-ready metrics with HTTP endpoint
- 🔍 **Agent Traceability** - Track every agent execution, LLM call, and tool invocation
- 📉 **Multi-Agent Observability** - Trace orchestrator planning, worker assignment, task completion
- 🎯 **Graceful Shutdown** - Proper span flushing before process exit

### Debug & Time-Travel (NEW!)
- 🔍 **Execution Snapshots** - Capture complete state at checkpoints
- ⏪ **Time-Travel Debugging** - Navigate forward/backward through execution
- 🔀 **What-If Analysis** - Branch execution and compare outcomes
- 🖥️ **Debug Studio TUI** - Interactive terminal UI for debugging
- 🌐 **Debug API Server** - HTTP API for external debugging tools
- 📊 **State Reconstruction** - Rebuild session, task, workspace at any point

## 📦 Installation

```bash
go get github.com/ranganaths/minion
```

## 🚀 Quick Start

### Hello, Minion!

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/ranganaths/minion/core"
    "github.com/ranganaths/minion/models"
    "github.com/ranganaths/minion/storage"
    "github.com/ranganaths/minion/llm"
)

func main() {
    // 1. Initialize Minion
    framework := core.NewFramework(
        core.WithStorage(storage.NewInMemory()),
        core.WithLLMProvider(llm.NewOpenAI(os.Getenv("OPENAI_API_KEY"))),
    )
    defer framework.Close()

    // 2. Create an agent
    agent, err := framework.CreateAgent(context.Background(), &models.CreateAgentRequest{
        Name:         "My First Minion",
        Description:  "A helpful AI assistant",
        BehaviorType: "default",
        Config: models.AgentConfig{
            LLMProvider: "openai",
            LLMModel:    "gpt-4",
            Temperature: 0.7,
            MaxTokens:   500,
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // 3. Activate the agent
    activeStatus := models.StatusActive
    agent, _ = framework.UpdateAgent(context.Background(), agent.ID, &models.UpdateAgentRequest{
        Status: &activeStatus,
    })

    // 4. Execute!
    output, err := framework.Execute(context.Background(), agent.ID, &models.Input{
        Raw:  "What is 2 + 2?",
        Type: "text",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Agent: %v\n", output.Result)
}
```

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────┐
│                    Minion Framework                  │
├─────────────────────────────────────────────────────┤
│                                                      │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐   │
│  │   Agent    │  │  Behavior  │  │   Tools    │   │
│  │  Registry  │  │  Registry  │  │  Registry  │   │
│  └────────────┘  └────────────┘  └────────────┘   │
│                                                      │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐   │
│  │  Storage   │  │    LLM     │  │  Metrics   │   │
│  │  Backend   │  │  Provider  │  │  Tracker   │   │
│  └────────────┘  └────────────┘  └────────────┘   │
│                                                      │
└─────────────────────────────────────────────────────┘
```

## 📚 Core Concepts

### 🤖 Agents

Agents are autonomous entities that process input using LLMs and tools:

```go
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name:         "Customer Service Agent",
    Description:  "Handles customer inquiries",
    BehaviorType: "conversational",
    Capabilities: []string{"sentiment_analysis", "knowledge_base"},
})
```

### 🧠 Behaviors

Behaviors define how agents process information:

```go
type SentimentBehavior struct{}

func (b *SentimentBehavior) GetSystemPrompt(agent *models.Agent) string {
    return "You are a sentiment analysis expert..."
}

func (b *SentimentBehavior) ProcessInput(ctx context.Context, agent *models.Agent, input *models.Input) (*models.ProcessedInput, error) {
    // Pre-process input before LLM
    return &models.ProcessedInput{
        Original:     input,
        Processed:    enhancedInput,
        Instructions: "Analyze sentiment...",
    }, nil
}

func (b *SentimentBehavior) ProcessOutput(ctx context.Context, agent *models.Agent, output *models.Output) (*models.ProcessedOutput, error) {
    // Post-process LLM output
    return &models.ProcessedOutput{
        Original:  output,
        Processed: enhancedOutput,
    }, nil
}

// Register the behavior
framework.RegisterBehavior("sentiment_analysis", &SentimentBehavior{})
```

### 🛠️ Tools

Tools are capabilities that agents can use:

```go
type CalculatorTool struct{}

func (t *CalculatorTool) Name() string {
    return "calculator"
}

func (t *CalculatorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
    result := performCalculation(input.Params)
    return &models.ToolOutput{
        ToolName: "calculator",
        Success:  true,
        Result:   result,
    }, nil
}

func (t *CalculatorTool) CanExecute(agent *models.Agent) bool {
    // Only available to agents with "math" capability
    for _, cap := range agent.Capabilities {
        if cap == "math" {
            return true
        }
    }
    return false
}

// Register the tool
framework.RegisterTool(&CalculatorTool{})
```

## 📊 Observability

Track agent performance and activity:

```go
// Get metrics
metrics, _ := framework.GetMetrics(ctx, agentID)
fmt.Printf("Total: %d | Success: %d | Failed: %d\n",
    metrics.TotalExecutions,
    metrics.SuccessfulExecutions,
    metrics.FailedExecutions)
fmt.Printf("Avg time: %.2fms\n", metrics.AvgExecutionTime)

// Get recent activities
activities, _ := framework.GetActivities(ctx, agentID, 10)
for _, activity := range activities {
    fmt.Printf("[%s] %s - %s (%dms)\n",
        activity.CreatedAt,
        activity.Action,
        activity.Status,
        activity.Duration)
}
```

### OpenTelemetry Tracing

Enable distributed tracing for full agent observability:

```go
import "github.com/Ranganaths/minion/observability"

// Initialize tracing
err := observability.InitGlobalTracer(observability.TracingConfig{
    Enabled:       true,
    ServiceName:   "my-agent-service",
    Environment:   "production",
    Exporter:      "otlp",  // or "jaeger", "stdout"
    OTLPEndpoint:  "localhost:4317",
    SamplingRatio: 0.1,
})
if err != nil {
    log.Fatal(err)
}
defer observability.GracefulShutdown(30 * time.Second)

// All agent executions are now traced automatically!
output, err := framework.Execute(ctx, agentID, input)
// Traces include: agent.execute, llm.openai.gpt-4, tool.*, etc.
```

### Prometheus Metrics

Expose production metrics via HTTP endpoint:

```go
import "github.com/Ranganaths/minion/metrics"

// Initialize Prometheus metrics
promMetrics := metrics.InitPrometheusMetrics(&metrics.PrometheusConfig{
    Namespace:              "minion",
    EnableGoCollector:      true,
    EnableProcessCollector: true,
})

// Expose /metrics endpoint
http.Handle("/metrics", metrics.MetricsHandler())
http.ListenAndServe(":9090", nil)
```

**Available Metrics:**
- `minion_agent_executions_total` - Agent execution count
- `minion_llm_calls_total` - LLM API calls
- `minion_llm_tokens_total` - Total tokens used
- `minion_llm_call_duration_seconds` - LLM latency histogram
- `minion_multiagent_tasks_total` - Multi-agent task count
- `minion_tool_executions_total` - Tool invocation count

## 🔌 LLM Providers

### OpenAI

```go
import "github.com/Ranganaths/minion/llm"

provider := llm.NewOpenAI(os.Getenv("OPENAI_API_KEY"))
framework := core.NewFramework(
    core.WithLLMProvider(provider),
)

// With request validation (recommended for production)
req := &llm.CompletionRequest{
    Model:       "gpt-4",
    UserPrompt:  "Hello!",
    Temperature: 0.7,
    MaxTokens:   100,
}
if err := req.Validate(); err != nil {
    log.Fatalf("Invalid request: %v", err)
}
```

### Anthropic (Claude)

```go
provider := llm.NewAnthropic(os.Getenv("ANTHROPIC_API_KEY"))
framework := core.NewFramework(
    core.WithLLMProvider(provider),
)
```

### TupleLeap

```go
provider := llm.NewTupleLeap(os.Getenv("TUPLELEAP_API_KEY"))
framework := core.NewFramework(
    core.WithLLMProvider(provider),
)
```

### Custom Provider

```go
type MyLLMProvider struct{}

func (p *MyLLMProvider) Name() string {
    return "my-provider"
}

func (p *MyLLMProvider) GenerateCompletion(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
    // Validate request first (recommended)
    if err := req.Validate(); err != nil {
        return nil, err
    }

    // Your implementation
    return &llm.CompletionResponse{
        Text:       response,
        TokensUsed: tokens,
        Model:      "my-model",
    }, nil
}

func (p *MyLLMProvider) GenerateChat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
    if err := req.Validate(); err != nil {
        return nil, err
    }
    // Your implementation
}

// Optional: Implement HealthCheckProvider for health monitoring
func (p *MyLLMProvider) HealthCheck(ctx context.Context) error {
    // Check connectivity to your LLM service
    return nil
}

framework := core.NewFramework(
    core.WithLLMProvider(&MyLLMProvider{}),
)
```

## 💾 Storage Backends

### In-Memory (Development)

```go
import "github.com/Ranganaths/minion/storage"

store := storage.NewInMemory()
framework := core.NewFramework(core.WithStorage(store))
```

### Custom Storage

```go
type MyStorage struct{}

func (s *MyStorage) Create(ctx context.Context, agent *models.Agent) error {
    // Your implementation
}

// Implement other storage.Store methods...

framework := core.NewFramework(
    core.WithStorage(&MyStorage{}),
)
```

## 📖 Examples

Check out the `examples/` directory for 13 comprehensive examples:

### Core Examples
- **`examples/basic/`** - Simple agent creation and execution
- **`examples/with_tools/`** - Custom tools with capability filtering
- **`examples/custom_behavior/`** - Specialized agent behaviors

### Multi-Agent Examples
- **`examples/multiagent-basic/`** - Basic multi-agent coordinator usage
- **`examples/multiagent-custom/`** - Custom worker agents
- **`examples/llm_worker/`** - LLM-powered worker agents

### Business Domain Examples
- **`examples/sales_agent/`** - Sales analyst with visualization tools
- **`examples/sales-automation/`** - Automated sales workflows
- **`examples/business_automation/`** - Business process automation
- **`examples/customer-support/`** - Customer support agent
- **`examples/devops-automation/`** - DevOps task automation

### Integration Examples
- **`examples/domain_tools/`** - Domain-specific tools (marketing, sales)
- **`examples/tupleleap_example/`** - TupleLeap LLM provider integration

### Observability Examples
- **`examples/tracing/`** - Agent tracing and trace analysis API
- **`examples/observability/`** - OpenTelemetry + Prometheus production setup

Run an example:

```bash
cd minion/examples/basic
export OPENAI_API_KEY="your-key"
go run main.go
```

## 🎨 Use Cases

### Customer Service Bot

```go
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name:         "Support Bot",
    BehaviorType: "customer_service",
    Capabilities: []string{"ticket_creation", "knowledge_base", "sentiment_analysis"},
})
```

### Data Analysis Agent

```go
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name:         "Data Analyst",
    BehaviorType: "analytical",
    Capabilities: []string{"sql_generation", "visualization", "forecasting"},
})
```

### Code Review Assistant

```go
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name:         "Code Reviewer",
    BehaviorType: "code_review",
    Capabilities: []string{"static_analysis", "security_scan", "best_practices"},
})
```

## 🧪 Testing

```go
func TestMinion(t *testing.T) {
    // Use in-memory storage for tests
    framework := core.NewFramework(
        core.WithStorage(storage.NewInMemory()),
    )

    agent, err := framework.CreateAgent(context.Background(), &models.CreateAgentRequest{
        Name: "Test Agent",
    })

    if err != nil {
        t.Fatalf("Failed to create agent: %v", err)
    }

    // Test execution
    output, err := framework.Execute(context.Background(), agent.ID, &models.Input{
        Raw: "test input",
    })

    assert.NoError(t, err)
    assert.NotNil(t, output)
}
```


## 🤝 Multi-Agent System (NEW!)

Minion now includes a production-ready multi-agent framework based on cutting-edge research:

- **Research Foundation**: Implements "Survey of AI Agent Protocols" (arXiv:2504.16736) and Microsoft's "Magentic-One" architecture (arXiv:2411.04468)
- **Orchestrator Pattern**: LLM-powered task decomposition and coordination
- **Specialized Workers**: Pre-built agents for coding, analysis, research, writing, and review
- **KQML Protocol**: Industry-standard agent communication
- **Task & Progress Ledgers**: Comprehensive execution tracking
- **Custom Workers**: Easily extend with domain-specific agents

**Quick Start:**
```go
// Initialize multi-agent system
coordinator := multiagent.NewCoordinator(llmProvider, nil)
coordinator.Initialize(ctx)

// Execute complex task
result, err := coordinator.ExecuteTask(ctx, &multiagent.TaskRequest{
    Name:        "Generate Sales Report",
    Description: "Analyze data and create comprehensive report",
    Type:        "analysis",
    Priority:    multiagent.PriorityHigh,
})
```

**Documentation:**
- [Multi-Agent Framework Documentation](core/multiagent/README.md)
- [Implementation Summary](MULTIAGENT_IMPLEMENTATION.md)
- [Examples](examples/multiagent/)

## 🔍 Debug & Time-Travel (NEW!)

Minion includes a powerful debugging system with time-travel capabilities, similar to LangGraph Studio:

### Features
- **Execution Snapshots**: Automatically capture state at 22+ checkpoint types
- **Timeline Navigation**: Step forward/backward through any execution
- **State Reconstruction**: Rebuild complete state at any point in time
- **What-If Analysis**: Create branches with modifications and compare outcomes
- **Debug API**: HTTP API for external tools and integrations
- **Terminal UI**: Interactive TUI built with Bubble Tea

### Quick Start

```go
import (
    "github.com/Ranganaths/minion/debug/snapshot"
    "github.com/Ranganaths/minion/debug/recorder"
    "github.com/Ranganaths/minion/debug/timetravel"
)

// Create snapshot store
store := snapshot.NewMemorySnapshotStore()

// Create recorder with hooks
rec := recorder.NewExecutionRecorder(store, recorder.DefaultRecorderConfig())
hooks := recorder.NewFrameworkHooks(rec)

// Record agent execution
agentHooks := hooks.ForAgent("my-agent")
agentHooks.OnExecutionStart(ctx, input)

// Record tool calls, LLM calls, decisions...
hooks.ForTool("my_tool").OnStart(ctx, input)
hooks.ForTool("my_tool").OnEnd(ctx, output, nil)

// End execution
agentHooks.OnExecutionEnd(ctx, output, nil)

// Time-travel through execution
timeline, _ := timetravel.NewExecutionTimeline(ctx, store, rec.GetExecutionID())
timeline.StepBackward()  // Go back
timeline.JumpToNextError()  // Find errors
timeline.JumpToCheckpoint(snapshot.CheckpointLLMCallStart)  // Find LLM calls

// What-if analysis
branching := timetravel.NewBranchingEngine(store)
comparison, _ := branching.WhatIf(ctx, executionID, 5, &timetravel.Modification{
    Type:  "input",
    Value: "modified_input",
})
```

### Debug API Server

```bash
# Start the debug API server
go run ./examples/debug-timetravel/main.go api
```

**Endpoints:**
- `GET /api/v1/executions` - List all executions
- `GET /api/v1/timeline/:id` - Get execution timeline
- `POST /api/v1/step` - Step through timeline
- `POST /api/v1/replay` - Replay from checkpoint
- `POST /api/v1/branches` - Create execution branch
- `POST /api/v1/what-if` - Run what-if analysis

### Terminal UI (Debug Studio)

```bash
# Launch interactive debugger
go run ./examples/debug-timetravel/main.go tui
```

**Keyboard shortcuts:**
- `j/k` - Navigate up/down
- `h/l` - Step backward/forward in timeline
- `e/E` - Jump to next/previous error
- `s` - Open state inspector
- `?` - Show help

**Documentation:**
- [Debug Implementation Plan](product-plan/debugging-timetravel-plan.md)
- [Debug Example](examples/debug-timetravel/)

## 🛣️ Roadmap

### Completed ✅
- [x] **OpenTelemetry Tracing** - Full distributed tracing with Jaeger/OTLP export
- [x] **Prometheus Metrics** - Production metrics with HTTP endpoint
- [x] **Agent Traceability** - Track every execution, LLM call, and tool invocation
- [x] **Debug & Time-Travel** - Execution snapshots, timeline navigation, what-if analysis
- [x] **Debug Studio TUI** - Interactive terminal debugger with Bubble Tea
- [x] **Debug API Server** - HTTP API for debugging and time-travel operations
- [x] **Multi-agent collaboration** - Research-based orchestrator with specialized workers
- [x] **Multiple LLM providers** - OpenAI, Anthropic, TupleLeap
- [x] **PostgreSQL storage** - Full transaction support
- [x] **MCP Integration** - Model Context Protocol with HTTP authentication
- [x] **Chain System** - LangChain-style RAG and workflow chains
- [x] **Production hardening** - Connection pooling, graceful shutdown, error handling

### In Progress
- [ ] Streaming responses (partial - chain streaming complete)
- [ ] Web UI for agent management
- [ ] Plugin system
- [ ] Google Gemini provider

### Recently Completed (v5.1)
- [x] **Debug & Time-Travel System** - Complete debugging infrastructure with snapshots
- [x] **Execution Recorder** - Capture checkpoints during agent/tool/LLM execution
- [x] **State Reconstructor** - Rebuild state at any point in execution
- [x] **Branching Engine** - What-if analysis with execution branching
- [x] **Debug API** - REST API with timeline navigation, replay, and branching
- [x] **Debug Studio TUI** - Terminal UI with execution list, timeline, state inspector

### Previously Completed (v5.0)
- [x] **LLM Request Validation** - `Validate()` and `WithDefaults()` methods
- [x] **Health Check Interface** - `HealthCheckProvider` for provider health monitoring
- [x] **Safe Type Assertions** - `GetInt`, `GetFloat`, `GetBool`, `GetMap` helpers
- [x] **Goroutine Leak Prevention** - Context-aware streaming in all chains
- [x] **Non-panicking Config** - `RequireString`, `RequireInt`, `RequireBool` methods
- [x] **Race Condition Fixes** - Atomic operations for thread-safe worker agents

## 📄 License

MIT License - see LICENSE file for details

## 📞 Support

- **Documentation**: Check the `examples/` directory and inline code comments
- **Issues**: [GitHub Issues](https://github.com/Ranganaths/minion/issues)
- **Discussions**: [GitHub Discussions](https://github.com/Ranganaths/minion/discussions)

---

*Minion - Your loyal AI agent framework*
