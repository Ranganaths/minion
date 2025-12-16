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

Originally built for AgentQL's semantic layer, Minion is now a standalone framework that can be used in any Go project.

## ✨ Features

- 🤖 **Agent Management** - Complete CRUD operations with metrics and activity tracking
- 🧠 **Pluggable Behaviors** - Define custom processing logic for specialized agents
- 🛠️ **Tool System** - Extensible tools with capability-based filtering
- 💾 **Storage Abstraction** - In-memory, PostgreSQL, or custom backends
- 🔌 **LLM Provider Support** - OpenAI, Anthropic, or custom providers
- 📊 **Built-in Observability** - Metrics, activity logs, and performance tracking
- ⚡ **Thread-Safe** - Concurrent operations with proper synchronization
- 🎨 **Highly Extensible** - Easy to add new behaviors, tools, and providers
- 🤝 **Multi-Agent Collaboration** - Research-based orchestrator pattern with specialized workers (NEW!)

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

## 🔌 LLM Providers

### OpenAI

```go
import "github.com/ranganaths/minion/llm"

provider := llm.NewOpenAI(apiKey)
framework := core.NewFramework(
    core.WithLLMProvider(provider),
)
```

### Custom Provider

```go
type MyLLMProvider struct{}

func (p *MyLLMProvider) GenerateCompletion(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
    // Your implementation
    return &llm.CompletionResponse{
        Text:       response,
        TokensUsed: tokens,
        Model:      "my-model",
    }, nil
}

framework := core.NewFramework(
    core.WithLLMProvider(&MyLLMProvider{}),
)
```

## 💾 Storage Backends

### In-Memory (Development)

```go
import "github.com/agentql/agentql/pkg/minion/storage"

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

Check out the `examples/` directory:

- **`examples/basic/`** - Simple agent creation and execution
- **`examples/with_tools/`** - Custom tools with capability filtering
- **`examples/custom_behavior/`** - Specialized agent behaviors

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

## 🛣️ Roadmap

- [ ] Additional LLM providers (Anthropic, Google Gemini, local models)
- [ ] PostgreSQL storage backend
- [ ] Streaming responses
- [x] **Multi-agent collaboration** ✅ **COMPLETED!**
- [ ] Advanced observability (distributed tracing)
- [ ] Web UI for agent management
- [ ] Plugin system

## 📄 License

MIT License - see LICENSE file for details

## 🙏 Acknowledgments

Minion was originally developed as part of the [AgentQL](https://github.com/agentql/agentql) project and extracted into a standalone framework for broader use.

## 📞 Support

- **Documentation**: Check the `examples/` directory and inline code comments
- **Issues**: [GitHub Issues](https://github.com/agentql/agentql/issues)
- **Discussions**: [GitHub Discussions](https://github.com/agentql/agentql/discussions)

---

**Built with ❤️ by the AgentQL team**

*Minion - Your loyal AI agent framework*
