# Multi-Agent System Examples

This directory contains examples demonstrating the multi-agent framework in Minion, based on AI agent protocols research and AutoGen's Magentic-One architecture.

## Overview

The multi-agent system implements:

- **Agent Communication Protocol** - Based on KQML (Knowledge Query and Manipulation Language)
- **Orchestrator Pattern** - Coordinates multiple specialized worker agents
- **Task & Progress Ledgers** - Tracks planning and execution
- **Specialized Workers** - Domain-specific agents (Coder, Analyst, Researcher, Writer, Reviewer)

## Architecture

```
┌─────────────────────────────────────────────────┐
│           Multi-Agent Coordinator                │
├─────────────────────────────────────────────────┤
│                                                  │
│  ┌──────────────┐        ┌─────────────────┐   │
│  │ Orchestrator │◄──────►│    Protocol     │   │
│  │    Agent     │        │   (Messages)    │   │
│  └──────┬───────┘        └─────────────────┘   │
│         │                                        │
│         │ Coordinates                            │
│         ▼                                        │
│  ┌─────────────────────────────────────────┐   │
│  │        Specialized Workers               │   │
│  │  ┌──────┐ ┌────────┐ ┌────────┐         │   │
│  │  │Coder │ │Analyst │ │Research│ ...     │   │
│  │  └──────┘ └────────┘ └────────┘         │   │
│  └─────────────────────────────────────────┘   │
│                                                  │
│  ┌──────────────┐  ┌──────────────────────┐   │
│  │ Task Ledger  │  │  Progress Ledger     │   │
│  └──────────────┘  └──────────────────────┘   │
└─────────────────────────────────────────────────┘
```

## Examples

### 1. Basic Example (`basic_example.go`)

Demonstrates core multi-agent functionality:

```bash
cd examples/multiagent
export OPENAI_API_KEY="your-key"
go run basic_example.go
```

Features:
- Initialize coordinator with default workers
- Execute code generation task
- Execute data analysis task
- Execute research task
- Monitor system stats
- Health checks

### 2. Custom Worker Example (`custom_worker_example.go`)

Shows how to create custom specialized workers:

```bash
cd examples/multiagent
export OPENAI_API_KEY="your-key"
go run custom_worker_example.go
```

Features:
- Create custom SQL specialist worker
- Register custom workers at runtime
- Execute domain-specific tasks
- Extend the multi-agent system

## Key Components

### 1. Protocol (`protocol.go`)

Implements inter-agent communication:

```go
// Message types
MessageTypeTask     // Task assignment
MessageTypeQuery    // Query for information
MessageTypeInform   // Information sharing
MessageTypeDelegate // Task delegation
MessageTypeResult   // Task result
MessageTypeError    // Error notification

// Security features
- Authentication
- Rate limiting
- Message size limits
- Agent allow/deny lists
```

### 2. Orchestrator (`orchestrator.go`)

Coordinates multi-agent workflows:

```go
// Features
- Task planning using LLM
- Task decomposition
- Worker selection
- Dependency management
- Retry logic
- Automatic replanning on errors
```

### 3. Workers (`workers.go`)

Specialized agents:

- **CoderWorker** - Code generation, review, debugging, refactoring
- **AnalystWorker** - Data analysis, statistics, forecasting, visualization
- **ResearcherWorker** - Research, information gathering, synthesis
- **WriterWorker** - Content creation, editing, technical writing
- **ReviewerWorker** - Code review, QA, testing

### 4. Ledgers (`ledger.go`)

Track execution:

- **TaskLedger** - High-level task planning and decomposition
- **ProgressLedger** - Step-by-step execution tracking

### 5. Coordinator (`coordinator.go`)

Main API:

```go
// Initialize system
coordinator := multiagent.NewCoordinator(llmProvider, config)
coordinator.Initialize(ctx)

// Execute tasks
result, err := coordinator.ExecuteTask(ctx, taskRequest)

// Monitor
stats := coordinator.GetMonitoringStats(ctx)
health := coordinator.HealthCheck(ctx)

// Shutdown
coordinator.Shutdown(ctx)
```

## Creating Custom Workers

Implement the `TaskHandler` interface:

```go
type CustomWorker struct {
    llmProvider multiagent.LLMProvider
}

func (w *CustomWorker) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
    // Your task processing logic
    return result, nil
}

func (w *CustomWorker) GetCapabilities() []string {
    return []string{"capability1", "capability2"}
}

func (w *CustomWorker) GetName() string {
    return "custom_worker"
}

// Register
worker, err := coordinator.CreateCustomWorker(
    ctx,
    "Custom Worker",
    multiagent.RoleSpecialist,
    &CustomWorker{llmProvider},
)
```

## Configuration

### Coordinator Config

```go
config := &multiagent.CoordinatorConfig{
    ProtocolSecurity: &multiagent.SecurityPolicy{
        RequireAuthentication: true,
        MaxMessageSize:        1024 * 1024,
        RateLimitPerSecond:    1000,
    },
    OrchestratorConfig: &multiagent.OrchestratorConfig{
        MaxRetries:         3,
        RetryDelay:         time.Second * 2,
        MaxConcurrentTasks: 5,
        TaskTimeout:        time.Minute * 5,
        EnableReplanning:   true,
    },
    MaxWorkers: 10,
}
```

## Task Execution Flow

1. **Task Submission** → Coordinator receives task request
2. **Planning** → Orchestrator decomposes task using LLM
3. **Worker Selection** → Best worker chosen based on capabilities
4. **Task Assignment** → Task sent via protocol messages
5. **Execution** → Worker processes task
6. **Result Handling** → Results aggregated and returned
7. **Monitoring** → Progress tracked in ledgers

## Monitoring

```go
// Get statistics
stats, _ := coordinator.GetMonitoringStats(ctx)
fmt.Printf("Workers: %d, Tasks: %d\n", stats.TotalWorkers, stats.TotalTasks)

// Health check
health := coordinator.HealthCheck(ctx)
fmt.Printf("Status: %s\n", health.Status)

// Protocol metrics
metrics := coordinator.GetMetrics()
fmt.Printf("Messages sent: %d\n", metrics.TotalMessagesSent)
```

## Research Foundation

This implementation is based on:

1. **"A Survey of AI Agent Protocols"** (arXiv:2504.16736)
   - Two-dimensional protocol classification
   - Security and scalability considerations
   - KQML message protocol

2. **"Magentic-One"** (Microsoft Research, arXiv:2411.04468)
   - Orchestrator pattern
   - Specialized worker agents
   - Task and progress ledgers
   - AutoGen framework

## Production Considerations

1. **Scalability**
   - Replace in-memory protocol with distributed message queue (Redis, Kafka)
   - Implement worker pooling and load balancing
   - Add horizontal scaling support

2. **Reliability**
   - Persist ledgers to database
   - Implement circuit breakers
   - Add comprehensive error handling
   - Implement task checkpointing

3. **Security**
   - Enable authentication and encryption
   - Implement role-based access control
   - Add audit logging
   - Validate all inputs

4. **Monitoring**
   - Add distributed tracing (OpenTelemetry)
   - Implement metrics collection (Prometheus)
   - Add alerting
   - Dashboard visualization

## Next Steps

1. Explore the basic example
2. Run the custom worker example
3. Create your own specialized workers
4. Integrate with your application
5. Configure for production use

## References

- AI Agent Protocols Survey: https://arxiv.org/abs/2504.16736
- Magentic-One: https://arxiv.org/abs/2411.04468
- AutoGen Framework: https://microsoft.github.io/autogen/
- Minion Documentation: ../../README.md
