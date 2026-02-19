# Minion Architecture

This document describes the architecture of the Minion AI agent framework, its components, design patterns, and how they work together.

## Overview

Minion is a layered framework for building AI agents in Go. It follows a modular architecture where components can be composed and configured independently.

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

## Core Components

### 1. Framework (`core/framework.go`)

The Framework is the central orchestrator that ties all components together. It uses the functional options pattern for configuration.

```go
type Framework interface {
    // Agent CRUD operations
    CreateAgent(ctx context.Context, req *models.CreateAgentRequest) (*models.Agent, error)
    GetAgent(ctx context.Context, id string) (*models.Agent, error)
    UpdateAgent(ctx context.Context, id string, req *models.UpdateAgentRequest) (*models.Agent, error)
    DeleteAgent(ctx context.Context, id string) error

    // Execution
    Execute(ctx context.Context, agentID string, input *models.Input) (*models.Output, error)

    // Tool operations
    RegisterTool(tool interface{}) error
    ExecuteTool(ctx context.Context, toolName string, params map[string]interface{}) (*models.ToolOutput, error)

    // Skill operations
    RegisterSkill(skill skills.Skill) error
    ExecuteSkill(ctx context.Context, skillName string, input *skills.SkillInput) (*skills.SkillOutput, error)

    // MCP operations
    ConnectMCPServer(ctx context.Context, config interface{}) error
    DisconnectMCPServer(serverName string) error
}
```

**Key responsibilities:**
- Agent lifecycle management (create, update, delete, list)
- Execution orchestration (behavior → LLM → output processing)
- Tool and skill registry management
- MCP server connections
- Metrics and activity recording

### 2. Agent (`models/agent.go`)

Agents are the fundamental units of work in Minion:

```go
type Agent struct {
    ID           string                 // Unique identifier
    Name         string                 // Human-readable name
    Description  string                 // Purpose description
    BehaviorType string                 // Behavior strategy ("default", "react", etc.)
    Status       AgentStatus            // draft, active, inactive, archived
    Config       AgentConfig            // LLM and behavior configuration
    Capabilities []string               // What the agent can do
    Metadata     map[string]interface{} // Custom key-value data
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

**Agent Status Lifecycle:**
```
draft → active → inactive → archived
         ↑          │
         └──────────┘
```

### 3. Behaviors (`core/interfaces.go`)

Behaviors define how agents process input and output. They control the "personality" and processing logic of an agent.

```go
type Behavior interface {
    // GetSystemPrompt generates the system prompt for the agent
    GetSystemPrompt(agent *models.Agent) string

    // ProcessInput prepares input before LLM execution
    ProcessInput(ctx context.Context, agent *models.Agent, input *models.Input) (*models.ProcessedInput, error)

    // ProcessOutput enhances output after LLM execution
    ProcessOutput(ctx context.Context, agent *models.Agent, output *models.Output) (*models.ProcessedOutput, error)
}
```

**Built-in behaviors:**
- `default` - Standard conversational behavior
- `react` - ReAct (Reasoning + Acting) pattern
- Custom behaviors can be registered

### 4. Tools (`tools/`)

Tools extend agent capabilities with executable functions. Each tool implements a simple interface:

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]interface{}  // JSON Schema
    Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error)
    RequiredCapabilities() []string
}
```

**Tool categories (80+ built-in):**
- Data: CSV, JSON, XML processing
- Database: SQL execution, migrations
- Cloud: AWS, GCP, Azure operations
- Communication: Email, Slack, webhooks
- Development: Git, code formatting
- Security: JWT, encryption, hashing

### 5. Skills (`skills/`)

Skills are higher-level, composable units that can be defined in code or markdown files:

```go
type Skill interface {
    Name() string
    Description() string
    Category() string
    Dependencies() []string
    CanExecute(agent *models.Agent) bool
    Execute(ctx context.Context, input *SkillInput) (*SkillOutput, error)
}
```

**Features:**
- Hot-reload from filesystem
- Markdown skill definitions
- Dependency resolution
- Capability-based filtering

### 6. LLM Providers (`llm/`)

Abstraction layer for different LLM backends:

```go
type Provider interface {
    GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
    GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    Name() string
}
```

**Supported providers:**
- OpenAI (GPT-4, GPT-3.5)
- Anthropic (Claude)
- Ollama (local models)
- TupleLeap (custom)

### 7. Storage (`storage/`)

Persistent storage for agents, metrics, and activities:

```go
type Store interface {
    // Agent CRUD
    Create(ctx context.Context, agent *models.Agent) error
    Get(ctx context.Context, id string) (*models.Agent, error)
    Update(ctx context.Context, agent *models.Agent) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, req *models.ListAgentsRequest) ([]*models.Agent, int, error)

    // Metrics and activities
    CreateMetrics(ctx context.Context, metrics *models.Metrics) error
    GetMetrics(ctx context.Context, agentID string) (*models.Metrics, error)
    RecordActivity(ctx context.Context, activity *models.Activity) error
}
```

**Implementations:**
- `InMemory` - For development and testing
- `PostgreSQL` - For production with transactions

## Protocol Implementations

### A2A Protocol (`protocols/a2a/`)

Google's Agent-to-Agent protocol for agent interoperability:

```
┌──────────────┐         JSON-RPC / SSE        ┌──────────────┐
│   Agent A    │ ◄────────────────────────────► │   Agent B    │
│              │                                │              │
│ Agent Card   │    tasks/send                  │ Agent Card   │
│ Skills       │    tasks/get                   │ Skills       │
│ Capabilities │    tasks/cancel                │ Capabilities │
└──────────────┘    tasks/subscribe             └──────────────┘
```

**Components:**
- `AgentCard` - Discovery document at `/.well-known/agent.json`
- `TaskManager` - Task lifecycle management
- `Server` - HTTP server with JSON-RPC
- `Client` - Client for remote A2A agents

### AG-UI Protocol (`protocols/agui/`)

CopilotKit's Agent-User Interface protocol for frontend integration:

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

**Components:**
- `EventEmitter` - SSE event generation
- `StateManager` - JSON Patch state synchronization
- `Server` - HTTP server with SSE streaming
- `StreamingMessage` - Token-by-token streaming

### MCP Protocol (`mcp/`)

Anthropic's Model Context Protocol for tool integration:

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

## Execution Flow

### Standard Execution

```
1. Execute(ctx, agentID, input)
       │
       ▼
2. Load Agent from Storage
       │
       ▼
3. Get Behavior by Type
       │
       ▼
4. ProcessInput (behavior)
       │
       ▼
5. Generate System Prompt
       │
       ▼
6. Call LLM Provider
       │
       ▼
7. ProcessOutput (behavior)
       │
       ▼
8. Record Activity & Metrics
       │
       ▼
9. Return Output
```

### ReAct Pattern Execution

```
1. Receive Task
       │
       ▼
2. ┌─► Think: Analyze current state
   │       │
   │       ▼
   │   Act: Select and execute tool
   │       │
   │       ▼
   │   Observe: Process tool output
   │       │
   │       ▼
   └───┬── Continue? ──► No ──► Return Final Answer
       │
      Yes
```

## Memory Systems (`memory/`)

Minion provides a comprehensive memory architecture for agent context persistence. See [Agent Memory](AGENT_MEMORY.md) for detailed documentation.

### Buffer Memory
Simple conversation history with configurable window size:
```go
memory := memory.NewBufferMemory(10) // Last 10 messages
memory.Add(ctx, message)
history := memory.GetHistory(ctx)
```

### Semantic Memory
Vector-based memory with similarity search:
```go
memory := memory.NewSemanticMemory(embedder, vectorStore)
memory.Store(ctx, content, metadata)
results := memory.Search(ctx, query, topK)
```

### Knowledge Graph Memory
Graph-based memory using Neo4j:
```go
memory := memory.NewKnowledgeGraphMemory(neo4jDriver)
memory.AddEntity(ctx, entity)
memory.AddRelation(ctx, from, relation, to)
path := memory.FindPath(ctx, start, end)
```

### Execution Snapshots
Point-in-time state capture for debugging and time-travel:
```go
store := snapshot.NewPostgresSnapshotStore(config)
store.Save(ctx, &ExecutionSnapshot{
    ExecutionID:    execID,
    CheckpointType: CheckpointAgentStep,
    SessionState:   sessionSnapshot,
})
```

## Observability

### Tracing (`observability/`)

OpenTelemetry-based distributed tracing:

```go
tracing.InitTracer(tracing.Config{
    ServiceName: "my-agent",
    Endpoint:    "http://localhost:4318",
})

ctx, span := observability.StartAgentSpan(ctx, agentID, agentName, "execute")
defer span.End()
```

### Metrics (`metrics/`)

Prometheus-compatible metrics:

```go
metrics.InitMetrics(metrics.Config{
    Endpoint: ":9090",
})

// Automatic metrics collection:
// - agent_executions_total
// - llm_calls_total
// - llm_tokens_used
// - execution_duration_seconds
```

### Time-Travel Debugging (`debug/`)

Record and replay execution:

```go
recorder := debug.NewRecorder(snapshotStore)
recorder.Snapshot(ctx, state)

timeline := debug.NewTimeline(recorder)
timeline.StepBackward()
timeline.StepForward()
state := timeline.GetCurrentState()
```

## Chain System (`chain/`)

Composable processing pipelines:

### LLM Chain
```go
chain := chain.NewLLMChain(provider, prompt)
result, _ := chain.Run(ctx, input)
```

### RAG Chain
```go
chain := chain.NewRAGChain(retriever, llmChain)
result, _ := chain.Run(ctx, query)
```

### Sequential Chain
```go
chain := chain.NewSequentialChain(
    step1Chain,
    step2Chain,
    step3Chain,
)
result, _ := chain.Run(ctx, input)
```

### Router Chain
```go
chain := chain.NewRouterChain(
    chain.Route{Condition: "...", Chain: chainA},
    chain.Route{Condition: "...", Chain: chainB},
)
result, _ := chain.Run(ctx, input)
```

## Multi-Agent System (`agents/`)

Coordinated multi-agent execution:

```go
coordinator := multiagent.NewCoordinator(
    multiagent.WithOrchestrator(orchestrator),
    multiagent.WithWorkers(
        workers.NewCoderWorker(llmProvider),
        workers.NewAnalystWorker(llmProvider),
        workers.NewResearcherWorker(llmProvider),
    ),
)

result, _ := coordinator.Execute(ctx, "Build a REST API")
```

**Patterns:**
- **Orchestrator** - Single coordinator delegates to workers
- **Pipeline** - Sequential agent processing
- **Consensus** - Multiple agents vote on decisions
- **Hierarchical** - Nested coordinator-worker structures

## Design Patterns

### Functional Options
All major components use functional options for configuration:

```go
framework := core.NewFramework(
    core.WithStorage(storage.NewInMemory()),
    core.WithLLMProvider(llm.NewOpenAI(apiKey)),
    core.WithBehaviorRegistry(registry),
    core.WithToolRegistry(tools.NewRegistry()),
)
```

### Registry Pattern
Centralized management of typed components:

```go
// Tools
registry.Register(myTool)
tool, _ := registry.Get("my_tool")

// Behaviors
behaviorRegistry.Register("custom", customBehavior)
behavior, _ := behaviorRegistry.Get("custom")
```

### Bridge Pattern
Protocol adapters bridge between Minion core and external protocols:

```go
// A2A Bridge
bridge := a2a.NewBridge(framework)
bridge.HandleTask(ctx, task)

// AG-UI Bridge
bridge := agui.NewBridge(framework, agentID)
bridge.HandleRun(ctx, runRequest, emitter)
```

## Directory Structure

```
minion/
├── core/           # Framework core, behaviors, interfaces
├── agents/         # ReAct agent, executor, multi-agent
├── llm/            # LLM providers (OpenAI, Anthropic, etc.)
├── tools/          # Tool registry, 80+ domain tools
├── skills/         # Skill system with hot-reload
├── chain/          # LLM chains, RAG, workflows
├── memory/         # Buffer, semantic, knowledge graph
├── storage/        # In-memory, PostgreSQL
├── protocols/      # Protocol implementations
│   ├── a2a/        # Google A2A protocol
│   └── agui/       # CopilotKit AG-UI protocol
├── mcp/            # Model Context Protocol
├── observability/  # Tracing, metrics
├── debug/          # Time-travel debugging
├── workflow/       # DAG-based workflows
├── evaluation/     # Agent evaluation, benchmarks
├── models/         # Data models
└── examples/       # Example applications
```

## Thread Safety

The framework is designed to be thread-safe:

- Framework instance can be shared across goroutines
- Storage implementations use appropriate locking
- Tool execution is stateless
- Each execution gets its own context

## Error Handling

Minion uses wrapped errors with context:

```go
if err != nil {
    return nil, fmt.Errorf("failed to execute tool %s: %w", toolName, err)
}
```

Errors propagate through the execution chain with full context for debugging.

## Next Steps

- [Getting Started](GETTING_STARTED.md) - Installation and first agent
- [API Reference](API_REFERENCE.md) - Complete API documentation
- [Agent Memory](AGENT_MEMORY.md) - Memory architecture and storage options
- [Protocols](PROTOCOLS.md) - A2A, AG-UI, MCP integration
- [Examples](EXAMPLES.md) - Code examples for common use cases
