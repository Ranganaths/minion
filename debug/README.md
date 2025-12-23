# Debug & Time-Travel Package

> Comprehensive debugging infrastructure for the Minion agent framework with time-travel capabilities.

## Overview

The `debug` package provides LangGraph Studio-style debugging for Minion agents, including:

- **Execution Snapshots**: Capture complete state at 22+ checkpoint types
- **Time-Travel Debugging**: Navigate forward/backward through execution
- **State Reconstruction**: Rebuild session, task, workspace at any point
- **What-If Analysis**: Branch execution and compare outcomes
- **Debug API Server**: HTTP REST API for external tools
- **Debug Studio TUI**: Interactive terminal debugger

## Package Structure

```
debug/
├── snapshot/           # Snapshot types and storage
│   ├── types.go       # Checkpoint types, ExecutionSnapshot, etc.
│   ├── store.go       # SnapshotStore interface
│   ├── store_memory.go # In-memory implementation
│   └── store_postgres.go # PostgreSQL implementation
├── recorder/          # Execution recording
│   ├── recorder.go    # ExecutionRecorder
│   └── hooks.go       # Framework integration hooks
├── timetravel/        # Time-travel capabilities
│   ├── timeline.go    # ExecutionTimeline navigation
│   ├── reconstructor.go # State reconstruction
│   ├── replay.go      # Replay engine
│   └── branching.go   # What-if analysis
├── api/               # Debug API server
│   ├── types.go       # Request/response types
│   └── server.go      # HTTP server
└── studio/            # Debug Studio
    └── tui/
        └── app.go     # Terminal UI (Bubble Tea)
```

## Quick Start

### 1. Set Up Snapshot Store

```go
import "github.com/Ranganaths/minion/debug/snapshot"

// In-memory (development)
store := snapshot.NewMemorySnapshotStore()
defer store.Close()

// PostgreSQL (production)
store, err := snapshot.NewPostgresSnapshotStore(ctx, "postgres://...")
```

### 2. Create Recorder with Hooks

```go
import (
    "github.com/Ranganaths/minion/debug/recorder"
)

rec := recorder.NewExecutionRecorder(store, recorder.DefaultRecorderConfig())
hooks := recorder.NewFrameworkHooks(rec)
```

### 3. Record Agent Execution

```go
agentHooks := hooks.ForAgent("my-agent")

// Start execution
agentHooks.OnExecutionStart(ctx, map[string]any{"query": "Hello"})

// Record tool calls
toolHooks := hooks.ForTool("my_tool")
toolHooks.OnStart(ctx, input)
toolHooks.OnEnd(ctx, output, nil)

// Record LLM calls
llmHooks := hooks.ForLLM("openai", "gpt-4")
llmHooks.OnStart(ctx, prompt)
llmHooks.OnEnd(ctx, response, inputTokens, outputTokens, cost, nil)

// Record decisions
decisionHooks := hooks.ForDecisions()
decisionHooks.OnDecision(ctx, "choice", []string{"a", "b", "c"}, "b")

// End execution
agentHooks.OnExecutionEnd(ctx, output, nil)
```

### 4. Time-Travel Through Execution

```go
import "github.com/Ranganaths/minion/debug/timetravel"

// Load timeline
timeline, err := timetravel.NewExecutionTimeline(ctx, store, executionID)

// Navigate
timeline.StepForward()   // Next checkpoint
timeline.StepBackward()  // Previous checkpoint
timeline.JumpToNextError() // Find errors
timeline.JumpToCheckpoint(snapshot.CheckpointLLMCallStart) // Find LLM calls

// Get current state
current := timeline.Current()
fmt.Printf("At: %s - %s\n", current.CheckpointType, current.Timestamp)
```

### 5. Reconstruct State

```go
reconstructor := timetravel.NewStateReconstructor(timeline)

// Rebuild state at any point
state, err := reconstructor.ReconstructAt(sequenceNum)
fmt.Printf("Session: %+v\n", state.Session)
fmt.Printf("Task: %+v\n", state.Task)
fmt.Printf("Workspace: %+v\n", state.Workspace)

// Compare states
comparison, err := reconstructor.CompareStates(seq1, seq2)
fmt.Printf("Time delta: %v\n", comparison.TimeDelta)
fmt.Printf("Actions between: %d\n", len(comparison.ActionsBetween))
```

### 6. What-If Analysis

```go
branching := timetravel.NewBranchingEngine(store)

// Create a branch with modification
branch, err := branching.CreateBranch(ctx, executionID, 5, &timetravel.CreateBranchOptions{
    Name: "alternative-input",
    Modification: &timetravel.Modification{
        Type:  "input",
        Value: "different_value",
    },
})

// Execute and compare
comparison, err := branching.CompareWithParent(ctx, branch.ID)
fmt.Printf("Duration delta: %v\n", comparison.DurationDelta)
fmt.Printf("Outcome same: %v\n", comparison.OutcomeSame)

// Quick what-if
result, err := branching.WhatIf(ctx, executionID, 5, &timetravel.Modification{
    Type:  "input",
    Value: "test_value",
})
```

### 7. Start Debug API Server

```go
import "github.com/Ranganaths/minion/debug/api"

config := api.DefaultServerConfig()
config.Addr = ":8080"

server := api.NewDebugServer(store, config)
server.Start() // Blocks
```

**API Endpoints:**

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/stats` | Store statistics |
| GET | `/api/v1/executions` | List executions |
| GET | `/api/v1/timeline/:id` | Get execution timeline |
| POST | `/api/v1/step` | Step through timeline |
| POST | `/api/v1/replay` | Replay from checkpoint |
| POST | `/api/v1/branches` | Create execution branch |
| POST | `/api/v1/what-if` | Run what-if analysis |

### 8. Launch Debug Studio TUI

```go
import "github.com/Ranganaths/minion/debug/studio/tui"

err := tui.Run(store)
```

**Keyboard Shortcuts:**

| Key | Action |
|-----|--------|
| `j/k` | Navigate up/down |
| `h/l` | Step backward/forward |
| `e/E` | Jump to next/previous error |
| `t` | Jump to checkpoint type |
| `s` | Open state inspector |
| `r` | Replay from current |
| `b` | Create branch |
| `?` | Show help |
| `q` | Quit |

## Checkpoint Types

The system captures 22+ checkpoint types:

| Category | Checkpoints |
|----------|-------------|
| **Task** | `task_created`, `task_started`, `task_completed`, `task_failed` |
| **Agent** | `agent_step`, `agent_decision`, `agent_output` |
| **Tool** | `tool_call_start`, `tool_call_end` |
| **LLM** | `llm_call_start`, `llm_call_end` |
| **Session** | `session_start`, `session_end`, `session_state` |
| **Memory** | `memory_read`, `memory_write` |
| **Workflow** | `workflow_start`, `workflow_end`, `workflow_step` |
| **Other** | `message_sent`, `message_received`, `decision_point`, `error`, `custom` |

## Example

See `examples/debug-timetravel/` for a complete working example:

```bash
# Record execution
go run ./examples/debug-timetravel record

# Start API server
go run ./examples/debug-timetravel api

# Launch TUI
go run ./examples/debug-timetravel tui

# Replay execution
go run ./examples/debug-timetravel replay

# Branch analysis
go run ./examples/debug-timetravel branch

# Full demo
go run ./examples/debug-timetravel demo
```

## Configuration

### Recorder Config

```go
config := &recorder.RecorderConfig{
    MaxSnapshots:    1000,       // Max snapshots to keep
    AutoFlush:       true,       // Auto-save snapshots
    FlushInterval:   5*time.Second,
    RecordInputs:    true,       // Capture inputs
    RecordOutputs:   true,       // Capture outputs
    RecordMetadata:  true,       // Capture metadata
    SamplingRate:    1.0,        // 100% sampling
}
```

### Memory Store Config

```go
config := &snapshot.MemoryStoreConfig{
    MaxSnapshots:  10000,    // Max total snapshots
    MaxExecutions: 100,      // Max executions to keep
    AutoEvict:     true,     // Auto-evict old entries
}
```

### PostgreSQL Store

```go
store, err := snapshot.NewPostgresSnapshotStore(ctx, connString)
// Automatically creates schema on first run
```

## Integration with Minion Framework

### Agent Behavior Hooks

```go
type DebugBehavior struct {
    hooks *recorder.FrameworkHooks
}

func (b *DebugBehavior) ProcessInput(ctx context.Context, agent *models.Agent, input *models.Input) (*models.ProcessedInput, error) {
    b.hooks.ForAgent(agent.ID).OnPlan(ctx, map[string]any{
        "input": input.Raw,
    })
    // ... processing
}
```

### Tool Wrapper

```go
func WrapToolWithHooks(tool Tool, hooks *recorder.FrameworkHooks) Tool {
    toolHooks := hooks.ForTool(tool.Name())

    return &wrappedTool{
        Tool: tool,
        execute: func(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
            toolHooks.OnStart(ctx, input)
            output, err := tool.Execute(ctx, input)
            toolHooks.OnEnd(ctx, output, err)
            return output, err
        },
    }
}
```

## Performance Considerations

- **Sampling**: Use `SamplingRate < 1.0` in production to reduce overhead
- **Memory Store**: Set `MaxSnapshots` to prevent unbounded memory growth
- **PostgreSQL**: Use batch operations for high-throughput recording
- **Selective Recording**: Only record what you need with targeted hooks

## Troubleshooting

### "No snapshots found"

- Ensure recorder is properly initialized before agent execution
- Check that hooks are being called during execution
- Verify snapshot store is not closed prematurely

### "Timeline empty"

- Confirm execution ID is correct
- Check that execution has completed or has checkpoints
- Verify store contains the execution data

### High Memory Usage

- Reduce `MaxSnapshots` in memory store config
- Enable `AutoEvict` for automatic cleanup
- Consider using PostgreSQL store for large-scale debugging

## License

MIT License - Part of the Minion Framework
