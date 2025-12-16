# Multi-Agent Framework for Minion

A production-ready multi-agent system implementation based on AI agent protocols research and Microsoft's AutoGen Magentic-One architecture.

## Overview

This multi-agent framework extends Minion to support complex task orchestration across multiple specialized AI agents. It implements industry-standard communication protocols and architectural patterns from cutting-edge research.

## Research Foundation

### 1. AI Agent Protocols Survey (arXiv:2504.16736)

This implementation follows the systematic framework proposed in the "Survey of AI Agent Protocols" paper:

**Two-Dimensional Classification:**
- **Context-oriented protocols** - Agent ↔ Environment communication
- **Inter-agent protocols** - Agent ↔ Agent communication
- **General-purpose protocols** - Flexible, domain-agnostic messaging
- **Domain-specific protocols** - Optimized for specific use cases

**Key Features Implemented:**
- KQML-inspired message protocol
- Security policies (authentication, rate limiting)
- Scalability considerations
- Latency optimization

### 2. Magentic-One Architecture (arXiv:2411.04468)

Inspired by Microsoft Research's generalist multi-agent system:

**Components:**
- **Orchestrator Agent** - Plans, tracks progress, re-plans on errors
- **Specialized Workers** - Domain-specific agents (Coder, Analyst, etc.)
- **Task Ledger** - High-level planning and decomposition
- **Progress Ledger** - Step-by-step execution tracking
- **Dynamic Workflow** - Adaptive task routing and recovery

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                  Multi-Agent Coordinator                     │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │              Orchestrator Agent                     │    │
│  │  • Task Planning (LLM-powered)                     │    │
│  │  • Worker Selection & Assignment                   │    │
│  │  • Dependency Management                           │    │
│  │  • Error Recovery & Replanning                     │    │
│  └───────────────────┬────────────────────────────────┘    │
│                      │                                       │
│                      ▼                                       │
│  ┌────────────────────────────────────────────────────┐    │
│  │          Communication Protocol (KQML-based)        │    │
│  │  • Message Types (Task, Query, Delegate, Result)   │    │
│  │  • Security (Auth, Rate Limiting, Encryption)      │    │
│  │  • Subscriptions & Broadcasting                    │    │
│  └───────────────────┬────────────────────────────────┘    │
│                      │                                       │
│                      ▼                                       │
│  ┌────────────────────────────────────────────────────┐    │
│  │              Specialized Workers                    │    │
│  │  ┌─────────┐ ┌─────────┐ ┌──────────┐            │    │
│  │  │  Coder  │ │ Analyst │ │Researcher│ ...         │    │
│  │  └─────────┘ └─────────┘ └──────────┘            │    │
│  └────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌──────────────┐              ┌──────────────────┐        │
│  │ Task Ledger  │              │ Progress Ledger  │        │
│  │ (Planning)   │              │  (Execution)     │        │
│  └──────────────┘              └──────────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Protocol Layer (`protocol.go`, `protocol_impl.go`)

Inter-agent communication system based on KQML:

```go
type MessageType string
const (
    MessageTypeTask     // Task assignment
    MessageTypeQuery    // Information query
    MessageTypeInform   // Information sharing
    MessageTypeDelegate // Task delegation
    MessageTypeResult   // Task completion
    MessageTypeError    // Error notification
)

type Protocol interface {
    Send(ctx context.Context, msg *Message) error
    Receive(ctx context.Context, agentID string) ([]*Message, error)
    Broadcast(ctx context.Context, msg *Message, groupID string) error
    Subscribe(ctx context.Context, agentID string, messageTypes []MessageType) error
}
```

**Security Features:**
- Authentication enforcement
- Message size limits
- Rate limiting (messages/second)
- Agent allow/deny lists
- Encryption support (configurable)

### 2. Ledger System (`ledger.go`)

Dual-ledger architecture inspired by Magentic-One:

**Task Ledger:**
- High-level task decomposition
- Dependency tracking
- Task status management
- History maintenance

**Progress Ledger:**
- Step-by-step execution tracking
- Agent action logging
- Error tracking
- Progress retrieval

```go
// Task Ledger
taskLedger := NewTaskLedger()
taskLedger.CreateTask(ctx, task)
taskLedger.CompleteTask(ctx, taskID, output)

// Progress Ledger
progressLedger := NewProgressLedger()
progressLedger.AddEntry(ctx, &ProgressEntry{
    TaskID:      taskID,
    AgentID:     agentID,
    Action:      "analysis",
    Description: "Analyzing data...",
})
```

### 3. Orchestrator (`orchestrator.go`)

Central coordinator implementing the orchestrator pattern:

**Key Capabilities:**
- LLM-powered task planning
- Intelligent worker selection
- Dependency resolution
- Timeout management
- Automatic retry with exponential backoff
- Replanning on failures

```go
orchestrator := NewOrchestrator(protocol, llmProvider, config)
orchestrator.RegisterWorker(workerMetadata)

result, err := orchestrator.ExecuteTask(ctx, &TaskRequest{
    Name:        "Complex Task",
    Description: "Multi-step task requiring coordination",
    Priority:    PriorityHigh,
})
```

**Configuration:**
```go
type OrchestratorConfig struct {
    MaxRetries         int           // Retry attempts
    RetryDelay         time.Duration // Delay between retries
    MaxConcurrentTasks int           // Parallel execution limit
    TaskTimeout        time.Duration // Per-task timeout
    EnableReplanning   bool          // Auto-replan on errors
}
```

### 4. Workers (`workers.go`)

Specialized agents for domain-specific tasks:

#### Pre-built Workers:

1. **CoderWorker**
   - Capabilities: code_generation, code_review, debugging, refactoring
   - Use cases: Generate code, review PRs, debug issues

2. **AnalystWorker**
   - Capabilities: data_analysis, statistical_analysis, forecasting, visualization
   - Use cases: Analyze trends, generate insights, create forecasts

3. **ResearcherWorker**
   - Capabilities: research, information_gathering, synthesis, summarization
   - Use cases: Gather information, synthesize findings, create summaries

4. **WriterWorker**
   - Capabilities: content_creation, editing, copywriting, technical_writing
   - Use cases: Create documentation, write articles, edit content

5. **ReviewerWorker**
   - Capabilities: code_review, content_review, quality_assurance, testing
   - Use cases: Review work products, QA testing, validation

#### Custom Workers:

```go
type CustomWorker struct {
    llmProvider LLMProvider
}

func (w *CustomWorker) HandleTask(ctx context.Context, task *Task) (interface{}, error) {
    // Your task handling logic
    return result, nil
}

func (w *CustomWorker) GetCapabilities() []string {
    return []string{"custom_capability"}
}

func (w *CustomWorker) GetName() string {
    return "custom_worker"
}
```

### 5. Coordinator (`coordinator.go`)

Main API for the multi-agent system:

```go
// Initialize
coordinator := NewCoordinator(llmProvider, config)
coordinator.Initialize(ctx) // Registers default workers

// Execute tasks
result, err := coordinator.ExecuteTask(ctx, &TaskRequest{
    Name:        "Generate Report",
    Description: "Create comprehensive sales report",
    Type:        "analysis",
    Priority:    PriorityHigh,
    Input:       salesData,
})

// Custom workers
worker, err := coordinator.CreateCustomWorker(
    ctx,
    "SQL Specialist",
    RoleSpecialist,
    customHandler,
)

// Monitoring
stats, _ := coordinator.GetMonitoringStats(ctx)
health := coordinator.HealthCheck(ctx)

// Shutdown
coordinator.Shutdown(ctx)
```

## Message Flow Example

```
┌─────────┐         ┌──────────────┐         ┌────────┐
│  User   │────────>│ Coordinator  │         │ Worker │
└─────────┘         └──────────────┘         └────────┘
                           │
    1. Submit Task         │
    ──────────────────────>│
                           │
                           │ 2. Plan with LLM
                           │────┐
                           │<───┘
                           │
                           │ 3. Select Worker
                           │────┐
                           │<───┘
                           │
                           │ 4. Send Task Message
                           │──────────────────────>│
                           │                       │
                           │                       │ 5. Process Task
                           │                       │────┐
                           │                       │<───┘
                           │                       │
                           │<──────────────────────│
                           │    6. Return Result
                           │
    7. Task Complete       │
    <──────────────────────│
```

## Configuration

### Security Policy

```go
security := &SecurityPolicy{
    RequireAuthentication: true,
    RequireEncryption:     true,
    AllowedAgents:         []string{"agent-1", "agent-2"},
    MaxMessageSize:        1024 * 1024, // 1MB
    RateLimitPerSecond:    100,
}
```

### Coordinator Configuration

```go
config := &CoordinatorConfig{
    ProtocolSecurity:   security,
    OrchestratorConfig: orchestratorConfig,
    MaxWorkers:         10,
    DefaultGroupID:     "production",
}
```

## Monitoring & Observability

### Monitoring Stats

```go
stats, _ := coordinator.GetMonitoringStats(ctx)

// Worker stats
fmt.Printf("Workers: %d (Idle: %d, Busy: %d)\n",
    stats.TotalWorkers, stats.IdleWorkers, stats.BusyWorkers)

// Task stats
fmt.Printf("Tasks: Completed: %d, Failed: %d, Pending: %d\n",
    stats.CompletedTasks, stats.FailedTasks, stats.PendingTasks)

// Protocol metrics
fmt.Printf("Messages: Sent: %d, Received: %d, Failed: %d\n",
    stats.ProtocolMetrics.TotalMessagesSent,
    stats.ProtocolMetrics.TotalMessagesReceived,
    stats.ProtocolMetrics.TotalMessagesFailed)
```

### Health Checks

```go
health := coordinator.HealthCheck(ctx)

// Overall status
fmt.Printf("Status: %s\n", health.Status) // healthy, degraded, unhealthy

// Component status
for component, status := range health.Components {
    fmt.Printf("%s: %s\n", component, status)
}

// Errors
for _, err := range health.Errors {
    fmt.Printf("Error: %s\n", err)
}
```

## Best Practices

### 1. Task Design

- **Decomposable**: Break complex tasks into smaller subtasks
- **Independent**: Minimize inter-task dependencies
- **Idempotent**: Tasks should be safely retryable
- **Time-bounded**: Set appropriate timeouts

### 2. Worker Design

- **Single Responsibility**: Each worker should have a clear focus
- **Stateless**: Workers should not maintain state between tasks
- **Error Handling**: Return meaningful errors for debugging
- **Capability Declaration**: Accurately declare capabilities

### 3. Error Handling

- **Graceful Degradation**: Handle worker failures gracefully
- **Retry Logic**: Use exponential backoff for retries
- **Replanning**: Enable replanning for complex tasks
- **Logging**: Comprehensive error logging

### 4. Performance

- **Worker Pooling**: Maintain a pool of idle workers
- **Task Batching**: Batch small related tasks
- **Timeout Tuning**: Set appropriate timeouts based on task complexity
- **Resource Limits**: Configure max workers and concurrent tasks

### 5. Security

- **Authentication**: Enable authentication in production
- **Encryption**: Use encryption for sensitive data
- **Rate Limiting**: Prevent DoS attacks
- **Input Validation**: Validate all task inputs

## Production Deployment

### Scalability Enhancements

1. **Distributed Protocol**
   - Replace in-memory with Redis/Kafka
   - Implement message persistence
   - Add message deduplication

2. **Worker Scaling**
   - Implement worker auto-scaling
   - Add worker health monitoring
   - Support multi-region deployment

3. **Task Persistence**
   - Store ledgers in database (PostgreSQL, MongoDB)
   - Implement task checkpointing
   - Add task replay capability

### Monitoring Integration

```go
// Prometheus metrics
import "github.com/prometheus/client_golang/prometheus"

var (
    tasksTotal = prometheus.NewCounterVec(...)
    taskDuration = prometheus.NewHistogramVec(...)
    workerStatus = prometheus.NewGaugeVec(...)
)

// OpenTelemetry tracing
import "go.opentelemetry.io/otel"

ctx, span := tracer.Start(ctx, "execute-task")
defer span.End()
```

## Examples

See `examples/multiagent/` for complete examples:

1. `basic_example.go` - Basic multi-agent usage
2. `custom_worker_example.go` - Creating custom workers
3. `README.md` - Detailed examples documentation

## References

### Research Papers

1. **A Survey of AI Agent Protocols** (April 2025)
   - arXiv: https://arxiv.org/abs/2504.16736
   - Focus: Protocol classification, security, scalability

2. **Magentic-One: A Generalist Multi-Agent System** (November 2024)
   - arXiv: https://arxiv.org/abs/2411.04468
   - Focus: Orchestrator architecture, specialized workers

3. **AutoGen Framework**
   - GitHub: https://github.com/microsoft/autogen
   - Docs: https://microsoft.github.io/autogen/

### Additional Resources

- KQML Specification: http://www.cs.umbc.edu/kqml/
- FIPA Agent Communication Language: http://www.fipa.org/specs/fipa00061/
- Multi-Agent Systems Book: "An Introduction to MultiAgent Systems" by Michael Wooldridge

## Contributing

When contributing to the multi-agent framework:

1. Follow Go best practices and idioms
2. Add comprehensive tests
3. Update documentation
4. Include examples for new features
5. Ensure backward compatibility

## License

MIT License - Same as Minion framework
