# Minion Debugging & Time-Travel Implementation Plan

## Executive Summary

This document outlines the implementation plan for adding **debugging** and **time-travel** capabilities to the Minion framework, bringing it to feature parity with LangGraph Studio while leveraging Go's performance advantages.

---

## Current State Analysis

### What Already Exists ✅

| Component | Status | Location |
|-----------|--------|----------|
| OpenTelemetry Tracing | ✅ Complete | `/observability/tracing.go` |
| Prometheus Metrics | ✅ Complete | `/observability/metrics.go` |
| Structured Logging | ✅ Complete | `/observability/logger.go` |
| Cost Tracking | ✅ Complete | `/observability/cost_tracker.go` |
| Session History | ✅ Complete | `/core/session.go` |
| Task Ledger | ✅ Complete | `/core/multiagent/ledger.go` |
| Progress Ledger | ✅ Complete | `/core/multiagent/ledger.go` |
| Chain Callbacks | ✅ Complete | `/chain/tracing_callback.go` |

### What's Missing ❌

| Component | Gap |
|-----------|-----|
| Execution Snapshots | No point-in-time state capture |
| Time-Travel Engine | No replay/rewind capability |
| Unified Query API | No cross-backend query interface |
| Visual Debugger | No Studio-like UI |
| Checkpoint/Resume | No execution branching |
| Causality Tracking | No "what caused this" analysis |

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Minion Debug Studio                          │
│  (Web UI - React/Next.js or Terminal UI - Bubble Tea)              │
├─────────────────────────────────────────────────────────────────────┤
│                        Debug API Server                             │
│  (HTTP/gRPC endpoints for queries, replay, branching)              │
├─────────────────────────────────────────────────────────────────────┤
│                      Time-Travel Engine                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                │
│  │  Snapshot   │  │   Replay    │  │  Branching  │                │
│  │   Store     │  │   Engine    │  │   Engine    │                │
│  └─────────────┘  └─────────────┘  └─────────────┘                │
├─────────────────────────────────────────────────────────────────────┤
│                    Execution Recorder                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                │
│  │ Checkpoint  │  │   Event     │  │   State     │                │
│  │  Manager    │  │  Collector  │  │  Differ     │                │
│  └─────────────┘  └─────────────┘  └─────────────┘                │
├─────────────────────────────────────────────────────────────────────┤
│                   Existing Infrastructure                           │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐              │
│  │ Tracing  │ │ Metrics  │ │ Ledgers  │ │ Sessions │              │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘              │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Execution Snapshot System

### 1.1 Snapshot Model

**File: `/debug/snapshot/types.go`**

```go
package snapshot

import "time"

// ExecutionSnapshot captures complete state at a point in time
type ExecutionSnapshot struct {
    ID            string                 `json:"id"`
    ExecutionID   string                 `json:"execution_id"`   // Links snapshots together
    SequenceNum   int64                  `json:"sequence_num"`   // Ordering within execution
    Timestamp     time.Time              `json:"timestamp"`

    // Checkpoint type
    CheckpointType CheckpointType        `json:"checkpoint_type"`

    // Agent/Task context
    AgentID       string                 `json:"agent_id,omitempty"`
    TaskID        string                 `json:"task_id,omitempty"`
    WorkerID      string                 `json:"worker_id,omitempty"`

    // State captures
    SessionState  *SessionSnapshot       `json:"session_state,omitempty"`
    TaskState     *TaskSnapshot          `json:"task_state,omitempty"`
    WorkspaceState map[string]any        `json:"workspace_state,omitempty"`

    // Execution context
    Action        *ActionSnapshot        `json:"action,omitempty"`
    Input         any                    `json:"input,omitempty"`
    Output        any                    `json:"output,omitempty"`

    // Observability links
    TraceID       string                 `json:"trace_id,omitempty"`
    SpanID        string                 `json:"span_id,omitempty"`
    ParentSpanID  string                 `json:"parent_span_id,omitempty"`

    // Error context (if any)
    Error         *ErrorSnapshot         `json:"error,omitempty"`

    // Metadata
    Metadata      map[string]any         `json:"metadata,omitempty"`
}

type CheckpointType string

const (
    CheckpointTaskCreated     CheckpointType = "task_created"
    CheckpointTaskAssigned    CheckpointType = "task_assigned"
    CheckpointTaskStarted     CheckpointType = "task_started"
    CheckpointTaskCompleted   CheckpointType = "task_completed"
    CheckpointTaskFailed      CheckpointType = "task_failed"
    CheckpointToolCallStart   CheckpointType = "tool_call_start"
    CheckpointToolCallEnd     CheckpointType = "tool_call_end"
    CheckpointLLMCallStart    CheckpointType = "llm_call_start"
    CheckpointLLMCallEnd      CheckpointType = "llm_call_end"
    CheckpointAgentStep       CheckpointType = "agent_step"
    CheckpointDecisionPoint   CheckpointType = "decision_point"
    CheckpointStateChange     CheckpointType = "state_change"
    CheckpointUserInput       CheckpointType = "user_input"
    CheckpointError           CheckpointType = "error"
)

type SessionSnapshot struct {
    ID        string    `json:"id"`
    History   []Message `json:"history"`
    Workspace map[string]any `json:"workspace"`
}

type TaskSnapshot struct {
    ID           string      `json:"id"`
    Name         string      `json:"name"`
    Status       string      `json:"status"`
    Dependencies []string    `json:"dependencies"`
    Input        any         `json:"input"`
    Output       any         `json:"output,omitempty"`
}

type ActionSnapshot struct {
    Type      string `json:"type"`       // "tool_call", "llm_call", "decision"
    Name      string `json:"name"`       // Tool name, model name, etc.
    Input     any    `json:"input"`
    Output    any    `json:"output,omitempty"`
    Duration  int64  `json:"duration_ms"`
}

type ErrorSnapshot struct {
    Type       string `json:"type"`
    Message    string `json:"message"`
    StackTrace string `json:"stack_trace,omitempty"`
    Cause      string `json:"cause,omitempty"`
}
```

### 1.2 Snapshot Store Interface

**File: `/debug/snapshot/store.go`**

```go
package snapshot

import (
    "context"
    "time"
)

// SnapshotStore persists and retrieves execution snapshots
type SnapshotStore interface {
    // Write operations
    Save(ctx context.Context, snapshot *ExecutionSnapshot) error
    SaveBatch(ctx context.Context, snapshots []*ExecutionSnapshot) error

    // Read operations - by execution
    GetByExecution(ctx context.Context, executionID string) ([]*ExecutionSnapshot, error)
    GetByExecutionRange(ctx context.Context, executionID string, fromSeq, toSeq int64) ([]*ExecutionSnapshot, error)

    // Read operations - by time
    GetByTimeRange(ctx context.Context, from, to time.Time) ([]*ExecutionSnapshot, error)

    // Read operations - by checkpoint
    GetByCheckpointType(ctx context.Context, executionID string, cpType CheckpointType) ([]*ExecutionSnapshot, error)

    // Read operations - specific snapshot
    Get(ctx context.Context, snapshotID string) (*ExecutionSnapshot, error)
    GetLatest(ctx context.Context, executionID string) (*ExecutionSnapshot, error)
    GetAtSequence(ctx context.Context, executionID string, seqNum int64) (*ExecutionSnapshot, error)

    // Query operations
    Query(ctx context.Context, query *SnapshotQuery) (*SnapshotQueryResult, error)

    // Maintenance
    PurgeOlderThan(ctx context.Context, age time.Duration) (int64, error)
    Stats(ctx context.Context) (*StoreStats, error)

    Close() error
}

type SnapshotQuery struct {
    ExecutionID    string         `json:"execution_id,omitempty"`
    AgentID        string         `json:"agent_id,omitempty"`
    TaskID         string         `json:"task_id,omitempty"`
    CheckpointType CheckpointType `json:"checkpoint_type,omitempty"`
    FromTime       *time.Time     `json:"from_time,omitempty"`
    ToTime         *time.Time     `json:"to_time,omitempty"`
    HasError       *bool          `json:"has_error,omitempty"`
    Limit          int            `json:"limit,omitempty"`
    Offset         int            `json:"offset,omitempty"`
    OrderBy        string         `json:"order_by,omitempty"` // "sequence_asc", "sequence_desc", "time_asc", "time_desc"
}

type SnapshotQueryResult struct {
    Snapshots  []*ExecutionSnapshot `json:"snapshots"`
    TotalCount int64                `json:"total_count"`
    HasMore    bool                 `json:"has_more"`
}

type StoreStats struct {
    TotalSnapshots    int64     `json:"total_snapshots"`
    TotalExecutions   int64     `json:"total_executions"`
    OldestSnapshot    time.Time `json:"oldest_snapshot"`
    NewestSnapshot    time.Time `json:"newest_snapshot"`
    StorageSizeBytes  int64     `json:"storage_size_bytes"`
}
```

### 1.3 Snapshot Store Implementations

**1.3.1 In-Memory Store (for development/testing)**

**File: `/debug/snapshot/store_memory.go`**

```go
package snapshot

import (
    "context"
    "sort"
    "sync"
    "time"

    "github.com/google/uuid"
)

type MemorySnapshotStore struct {
    mu         sync.RWMutex
    snapshots  map[string]*ExecutionSnapshot           // snapshotID -> snapshot
    byExecution map[string][]*ExecutionSnapshot        // executionID -> ordered snapshots
    maxSize    int                                      // max snapshots to keep
}

func NewMemorySnapshotStore(maxSize int) *MemorySnapshotStore {
    if maxSize <= 0 {
        maxSize = 10000
    }
    return &MemorySnapshotStore{
        snapshots:   make(map[string]*ExecutionSnapshot),
        byExecution: make(map[string][]*ExecutionSnapshot),
        maxSize:     maxSize,
    }
}

func (s *MemorySnapshotStore) Save(ctx context.Context, snapshot *ExecutionSnapshot) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if snapshot.ID == "" {
        snapshot.ID = uuid.New().String()
    }

    s.snapshots[snapshot.ID] = snapshot
    s.byExecution[snapshot.ExecutionID] = append(s.byExecution[snapshot.ExecutionID], snapshot)

    // Sort by sequence number
    sort.Slice(s.byExecution[snapshot.ExecutionID], func(i, j int) bool {
        return s.byExecution[snapshot.ExecutionID][i].SequenceNum <
               s.byExecution[snapshot.ExecutionID][j].SequenceNum
    })

    // Evict old snapshots if over limit
    if len(s.snapshots) > s.maxSize {
        s.evictOldest()
    }

    return nil
}

// ... additional methods implementation
```

**1.3.2 PostgreSQL Store (for production)**

**File: `/debug/snapshot/store_postgres.go`**

```go
package snapshot

import (
    "context"
    "database/sql"
    "encoding/json"
    "time"

    _ "github.com/lib/pq"
)

type PostgresSnapshotStore struct {
    db              *sql.DB
    stmtInsert      *sql.Stmt
    stmtGetByExec   *sql.Stmt
    // ... more prepared statements
}

// Schema for PostgreSQL
const schemaSQL = `
CREATE TABLE IF NOT EXISTS execution_snapshots (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id    VARCHAR(255) NOT NULL,
    sequence_num    BIGINT NOT NULL,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    checkpoint_type VARCHAR(50) NOT NULL,
    agent_id        VARCHAR(255),
    task_id         VARCHAR(255),
    worker_id       VARCHAR(255),
    session_state   JSONB,
    task_state      JSONB,
    workspace_state JSONB,
    action          JSONB,
    input           JSONB,
    output          JSONB,
    trace_id        VARCHAR(64),
    span_id         VARCHAR(32),
    parent_span_id  VARCHAR(32),
    error           JSONB,
    metadata        JSONB,

    CONSTRAINT unique_execution_sequence UNIQUE (execution_id, sequence_num)
);

CREATE INDEX idx_snapshots_execution ON execution_snapshots(execution_id);
CREATE INDEX idx_snapshots_timestamp ON execution_snapshots(timestamp);
CREATE INDEX idx_snapshots_checkpoint ON execution_snapshots(checkpoint_type);
CREATE INDEX idx_snapshots_agent ON execution_snapshots(agent_id);
CREATE INDEX idx_snapshots_task ON execution_snapshots(task_id);
CREATE INDEX idx_snapshots_trace ON execution_snapshots(trace_id);
`
```

### 1.4 Snapshot Recorder (Integration Layer)

**File: `/debug/recorder/recorder.go`**

```go
package recorder

import (
    "context"
    "sync/atomic"
    "time"

    "github.com/tmc/minion/debug/snapshot"
    "github.com/tmc/minion/observability"
)

// ExecutionRecorder captures snapshots during execution
type ExecutionRecorder struct {
    store         snapshot.SnapshotStore
    tracer        *observability.Tracer
    enabled       atomic.Bool
    executionID   string
    sequenceNum   atomic.Int64
    config        RecorderConfig
}

type RecorderConfig struct {
    // What to capture
    CaptureSessionState   bool
    CaptureTaskState      bool
    CaptureWorkspace      bool
    CaptureInputOutput    bool
    CaptureFullLLMContext bool  // Can be expensive

    // Checkpoint filters
    EnabledCheckpoints    map[snapshot.CheckpointType]bool

    // Sampling (for high-frequency checkpoints)
    SampleRate            float64  // 0.0 to 1.0

    // Size limits
    MaxInputSize          int      // Truncate large inputs
    MaxOutputSize         int      // Truncate large outputs
}

func DefaultRecorderConfig() RecorderConfig {
    return RecorderConfig{
        CaptureSessionState:   true,
        CaptureTaskState:      true,
        CaptureWorkspace:      true,
        CaptureInputOutput:    true,
        CaptureFullLLMContext: false,
        EnabledCheckpoints:    nil,  // All enabled by default
        SampleRate:            1.0,  // Capture all
        MaxInputSize:          64 * 1024,  // 64KB
        MaxOutputSize:         64 * 1024,
    }
}

func NewExecutionRecorder(store snapshot.SnapshotStore, config RecorderConfig) *ExecutionRecorder {
    return &ExecutionRecorder{
        store:  store,
        config: config,
    }
}

// StartExecution begins a new execution recording
func (r *ExecutionRecorder) StartExecution(ctx context.Context, executionID string) {
    r.executionID = executionID
    r.sequenceNum.Store(0)
    r.enabled.Store(true)

    // Extract trace context
    if r.tracer != nil {
        // Link to OpenTelemetry trace
    }
}

// RecordCheckpoint captures a snapshot at the current point
func (r *ExecutionRecorder) RecordCheckpoint(ctx context.Context, cp *Checkpoint) error {
    if !r.enabled.Load() {
        return nil
    }

    if !r.shouldCapture(cp.Type) {
        return nil
    }

    snap := &snapshot.ExecutionSnapshot{
        ExecutionID:    r.executionID,
        SequenceNum:    r.sequenceNum.Add(1),
        Timestamp:      time.Now(),
        CheckpointType: cp.Type,
        AgentID:        cp.AgentID,
        TaskID:         cp.TaskID,
        WorkerID:       cp.WorkerID,
        Action:         r.captureAction(cp),
        Input:          r.truncate(cp.Input, r.config.MaxInputSize),
        Output:         r.truncate(cp.Output, r.config.MaxOutputSize),
        Metadata:       cp.Metadata,
    }

    // Capture state if configured
    if r.config.CaptureSessionState && cp.Session != nil {
        snap.SessionState = r.captureSessionState(cp.Session)
    }
    if r.config.CaptureTaskState && cp.Task != nil {
        snap.TaskState = r.captureTaskState(cp.Task)
    }
    if r.config.CaptureWorkspace && cp.Workspace != nil {
        snap.WorkspaceState = cp.Workspace
    }

    // Extract trace context
    if traceID := observability.GetTraceID(ctx); traceID != "" {
        snap.TraceID = traceID
    }
    if spanID := observability.GetSpanID(ctx); spanID != "" {
        snap.SpanID = spanID
    }

    // Capture error if present
    if cp.Error != nil {
        snap.Error = &snapshot.ErrorSnapshot{
            Type:    fmt.Sprintf("%T", cp.Error),
            Message: cp.Error.Error(),
        }
    }

    return r.store.Save(ctx, snap)
}

// Checkpoint represents a point to capture
type Checkpoint struct {
    Type      snapshot.CheckpointType
    AgentID   string
    TaskID    string
    WorkerID  string
    Session   *models.Session
    Task      *multiagent.Task
    Workspace map[string]any
    Action    *ActionInfo
    Input     any
    Output    any
    Error     error
    Metadata  map[string]any
}
```

---

## Phase 2: Time-Travel Engine

### 2.1 Execution Timeline

**File: `/debug/timetravel/timeline.go`**

```go
package timetravel

import (
    "context"
    "time"

    "github.com/tmc/minion/debug/snapshot"
)

// ExecutionTimeline provides time-travel navigation over an execution
type ExecutionTimeline struct {
    executionID string
    store       snapshot.SnapshotStore
    snapshots   []*snapshot.ExecutionSnapshot  // Cached, ordered by sequence
    cursor      int                             // Current position
}

func NewExecutionTimeline(ctx context.Context, store snapshot.SnapshotStore, executionID string) (*ExecutionTimeline, error) {
    snapshots, err := store.GetByExecution(ctx, executionID)
    if err != nil {
        return nil, err
    }

    return &ExecutionTimeline{
        executionID: executionID,
        store:       store,
        snapshots:   snapshots,
        cursor:      len(snapshots) - 1, // Start at end
    }, nil
}

// Navigation methods

// Current returns the snapshot at current cursor position
func (t *ExecutionTimeline) Current() *snapshot.ExecutionSnapshot {
    if t.cursor < 0 || t.cursor >= len(t.snapshots) {
        return nil
    }
    return t.snapshots[t.cursor]
}

// StepForward moves one step forward
func (t *ExecutionTimeline) StepForward() *snapshot.ExecutionSnapshot {
    if t.cursor < len(t.snapshots)-1 {
        t.cursor++
    }
    return t.Current()
}

// StepBackward moves one step backward
func (t *ExecutionTimeline) StepBackward() *snapshot.ExecutionSnapshot {
    if t.cursor > 0 {
        t.cursor--
    }
    return t.Current()
}

// JumpTo moves to specific sequence number
func (t *ExecutionTimeline) JumpTo(seqNum int64) *snapshot.ExecutionSnapshot {
    for i, snap := range t.snapshots {
        if snap.SequenceNum == seqNum {
            t.cursor = i
            return snap
        }
    }
    return nil
}

// JumpToCheckpoint moves to next checkpoint of given type
func (t *ExecutionTimeline) JumpToCheckpoint(cpType snapshot.CheckpointType, forward bool) *snapshot.ExecutionSnapshot {
    if forward {
        for i := t.cursor + 1; i < len(t.snapshots); i++ {
            if t.snapshots[i].CheckpointType == cpType {
                t.cursor = i
                return t.snapshots[i]
            }
        }
    } else {
        for i := t.cursor - 1; i >= 0; i-- {
            if t.snapshots[i].CheckpointType == cpType {
                t.cursor = i
                return t.snapshots[i]
            }
        }
    }
    return nil
}

// JumpToError jumps to next/previous error
func (t *ExecutionTimeline) JumpToError(forward bool) *snapshot.ExecutionSnapshot {
    if forward {
        for i := t.cursor + 1; i < len(t.snapshots); i++ {
            if t.snapshots[i].Error != nil {
                t.cursor = i
                return t.snapshots[i]
            }
        }
    } else {
        for i := t.cursor - 1; i >= 0; i-- {
            if t.snapshots[i].Error != nil {
                t.cursor = i
                return t.snapshots[i]
            }
        }
    }
    return nil
}

// Query methods

// GetRange returns snapshots between two sequence numbers
func (t *ExecutionTimeline) GetRange(fromSeq, toSeq int64) []*snapshot.ExecutionSnapshot {
    var result []*snapshot.ExecutionSnapshot
    for _, snap := range t.snapshots {
        if snap.SequenceNum >= fromSeq && snap.SequenceNum <= toSeq {
            result = append(result, snap)
        }
    }
    return result
}

// GetByType returns all snapshots of a given type
func (t *ExecutionTimeline) GetByType(cpType snapshot.CheckpointType) []*snapshot.ExecutionSnapshot {
    var result []*snapshot.ExecutionSnapshot
    for _, snap := range t.snapshots {
        if snap.CheckpointType == cpType {
            result = append(result, snap)
        }
    }
    return result
}

// GetErrors returns all error snapshots
func (t *ExecutionTimeline) GetErrors() []*snapshot.ExecutionSnapshot {
    var result []*snapshot.ExecutionSnapshot
    for _, snap := range t.snapshots {
        if snap.Error != nil {
            result = append(result, snap)
        }
    }
    return result
}

// Analysis methods

// Duration returns total execution duration
func (t *ExecutionTimeline) Duration() time.Duration {
    if len(t.snapshots) < 2 {
        return 0
    }
    first := t.snapshots[0].Timestamp
    last := t.snapshots[len(t.snapshots)-1].Timestamp
    return last.Sub(first)
}

// Summary returns execution summary
func (t *ExecutionTimeline) Summary() *ExecutionSummary {
    summary := &ExecutionSummary{
        ExecutionID:    t.executionID,
        TotalSteps:     len(t.snapshots),
        CheckpointCounts: make(map[snapshot.CheckpointType]int),
    }

    if len(t.snapshots) > 0 {
        summary.StartTime = t.snapshots[0].Timestamp
        summary.EndTime = t.snapshots[len(t.snapshots)-1].Timestamp
        summary.Duration = summary.EndTime.Sub(summary.StartTime)
    }

    for _, snap := range t.snapshots {
        summary.CheckpointCounts[snap.CheckpointType]++
        if snap.Error != nil {
            summary.ErrorCount++
        }
    }

    return summary
}

type ExecutionSummary struct {
    ExecutionID      string
    StartTime        time.Time
    EndTime          time.Time
    Duration         time.Duration
    TotalSteps       int
    ErrorCount       int
    CheckpointCounts map[snapshot.CheckpointType]int
}
```

### 2.2 State Reconstructor

**File: `/debug/timetravel/reconstructor.go`**

```go
package timetravel

import (
    "context"

    "github.com/tmc/minion/debug/snapshot"
    "github.com/tmc/minion/models"
    "github.com/tmc/minion/core/multiagent"
)

// StateReconstructor rebuilds execution state at any point
type StateReconstructor struct {
    timeline *ExecutionTimeline
}

func NewStateReconstructor(timeline *ExecutionTimeline) *StateReconstructor {
    return &StateReconstructor{timeline: timeline}
}

// ReconstructSessionAt rebuilds session state at given sequence number
func (r *StateReconstructor) ReconstructSessionAt(seqNum int64) (*models.Session, error) {
    // Find the latest session snapshot at or before seqNum
    var sessionSnap *snapshot.SessionSnapshot

    for _, snap := range r.timeline.snapshots {
        if snap.SequenceNum > seqNum {
            break
        }
        if snap.SessionState != nil {
            sessionSnap = snap.SessionState
        }
    }

    if sessionSnap == nil {
        return nil, nil
    }

    return &models.Session{
        ID:        sessionSnap.ID,
        History:   sessionSnap.History,
        Workspace: sessionSnap.Workspace,
    }, nil
}

// ReconstructTaskAt rebuilds task state at given sequence number
func (r *StateReconstructor) ReconstructTaskAt(seqNum int64) (*multiagent.Task, error) {
    var taskSnap *snapshot.TaskSnapshot

    for _, snap := range r.timeline.snapshots {
        if snap.SequenceNum > seqNum {
            break
        }
        if snap.TaskState != nil {
            taskSnap = snap.TaskState
        }
    }

    if taskSnap == nil {
        return nil, nil
    }

    return &multiagent.Task{
        ID:           taskSnap.ID,
        Name:         taskSnap.Name,
        Status:       multiagent.TaskStatus(taskSnap.Status),
        Dependencies: taskSnap.Dependencies,
        Input:        taskSnap.Input,
        Output:       taskSnap.Output,
    }, nil
}

// ReconstructWorkspaceAt rebuilds workspace at given sequence number
func (r *StateReconstructor) ReconstructWorkspaceAt(seqNum int64) (map[string]any, error) {
    var workspace map[string]any

    for _, snap := range r.timeline.snapshots {
        if snap.SequenceNum > seqNum {
            break
        }
        if snap.WorkspaceState != nil {
            workspace = snap.WorkspaceState
        }
    }

    return workspace, nil
}

// FullStateAt returns complete reconstructed state at a point
func (r *StateReconstructor) FullStateAt(seqNum int64) (*ReconstructedState, error) {
    session, _ := r.ReconstructSessionAt(seqNum)
    task, _ := r.ReconstructTaskAt(seqNum)
    workspace, _ := r.ReconstructWorkspaceAt(seqNum)

    // Get the snapshot at this sequence
    snap := r.timeline.JumpTo(seqNum)

    return &ReconstructedState{
        SequenceNum: seqNum,
        Timestamp:   snap.Timestamp,
        Session:     session,
        Task:        task,
        Workspace:   workspace,
        Snapshot:    snap,
    }, nil
}

type ReconstructedState struct {
    SequenceNum int64
    Timestamp   time.Time
    Session     *models.Session
    Task        *multiagent.Task
    Workspace   map[string]any
    Snapshot    *snapshot.ExecutionSnapshot
}
```

### 2.3 Replay Engine

**File: `/debug/timetravel/replay.go`**

```go
package timetravel

import (
    "context"
    "time"

    "github.com/tmc/minion/debug/snapshot"
    "github.com/tmc/minion/core"
)

// ReplayEngine enables re-execution from any checkpoint
type ReplayEngine struct {
    framework     core.Framework
    timeline      *ExecutionTimeline
    reconstructor *StateReconstructor
}

func NewReplayEngine(framework core.Framework, timeline *ExecutionTimeline) *ReplayEngine {
    return &ReplayEngine{
        framework:     framework,
        timeline:      timeline,
        reconstructor: NewStateReconstructor(timeline),
    }
}

// ReplayFrom re-executes from a given sequence number
func (r *ReplayEngine) ReplayFrom(ctx context.Context, seqNum int64, opts ReplayOptions) (*ReplayResult, error) {
    // Reconstruct state at the checkpoint
    state, err := r.reconstructor.FullStateAt(seqNum)
    if err != nil {
        return nil, err
    }

    // Create new execution context with reconstructed state
    replayCtx := r.createReplayContext(ctx, state, opts)

    result := &ReplayResult{
        OriginalExecutionID: r.timeline.executionID,
        ReplayStartSeq:      seqNum,
        ReplayStartTime:     time.Now(),
        OriginalSnapshots:   r.timeline.GetRange(seqNum, int64(len(r.timeline.snapshots))),
    }

    // Execute with the framework
    // This depends on what we're replaying (agent, task, etc.)
    if state.Task != nil {
        output, err := r.replayTask(replayCtx, state, opts)
        result.Output = output
        result.Error = err
    } else if state.Session != nil {
        output, err := r.replaySession(replayCtx, state, opts)
        result.Output = output
        result.Error = err
    }

    result.ReplayEndTime = time.Now()
    result.Duration = result.ReplayEndTime.Sub(result.ReplayStartTime)

    return result, nil
}

// ReplayWithModification replays with modified input
func (r *ReplayEngine) ReplayWithModification(ctx context.Context, seqNum int64, modification *Modification) (*ReplayResult, error) {
    state, err := r.reconstructor.FullStateAt(seqNum)
    if err != nil {
        return nil, err
    }

    // Apply modification
    modifiedState := r.applyModification(state, modification)

    return r.ReplayFrom(ctx, seqNum, ReplayOptions{
        ModifiedState: modifiedState,
    })
}

type ReplayOptions struct {
    // Override inputs
    ModifiedState *ReconstructedState

    // Control execution
    StopAtCheckpoint snapshot.CheckpointType
    MaxSteps         int
    Timeout          time.Duration

    // Comparison mode
    CompareWithOriginal bool
}

type ReplayResult struct {
    OriginalExecutionID string
    ReplayExecutionID   string
    ReplayStartSeq      int64
    ReplayStartTime     time.Time
    ReplayEndTime       time.Time
    Duration            time.Duration
    Output              any
    Error               error

    // For comparison
    OriginalSnapshots   []*snapshot.ExecutionSnapshot
    ReplaySnapshots     []*snapshot.ExecutionSnapshot
    Differences         []*StateDifference
}

type Modification struct {
    Type   string // "input", "workspace", "tool_response"
    Path   string // JSON path to modify
    Value  any    // New value
}

type StateDifference struct {
    SequenceNum int64
    Path        string
    Original    any
    Replayed    any
    Type        string // "added", "removed", "changed"
}
```

### 2.4 Branching Engine (What-If Analysis)

**File: `/debug/timetravel/branching.go`**

```go
package timetravel

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/tmc/minion/debug/snapshot"
)

// BranchingEngine enables "what-if" analysis with execution branches
type BranchingEngine struct {
    store   snapshot.SnapshotStore
    replay  *ReplayEngine
    branches map[string]*ExecutionBranch
}

// ExecutionBranch represents an alternative execution path
type ExecutionBranch struct {
    ID               string
    ParentExecutionID string
    BranchPointSeq   int64
    Modification     *Modification
    Timeline         *ExecutionTimeline
    Status           BranchStatus
}

type BranchStatus string

const (
    BranchPending   BranchStatus = "pending"
    BranchRunning   BranchStatus = "running"
    BranchCompleted BranchStatus = "completed"
    BranchFailed    BranchStatus = "failed"
)

func NewBranchingEngine(store snapshot.SnapshotStore, replay *ReplayEngine) *BranchingEngine {
    return &BranchingEngine{
        store:    store,
        replay:   replay,
        branches: make(map[string]*ExecutionBranch),
    }
}

// CreateBranch creates a new execution branch from a checkpoint
func (b *BranchingEngine) CreateBranch(ctx context.Context, executionID string, seqNum int64, mod *Modification) (*ExecutionBranch, error) {
    branchID := fmt.Sprintf("branch-%s", uuid.New().String()[:8])

    branch := &ExecutionBranch{
        ID:               branchID,
        ParentExecutionID: executionID,
        BranchPointSeq:   seqNum,
        Modification:     mod,
        Status:           BranchPending,
    }

    b.branches[branchID] = branch
    return branch, nil
}

// ExecuteBranch runs the branch execution
func (b *BranchingEngine) ExecuteBranch(ctx context.Context, branchID string) (*ReplayResult, error) {
    branch, ok := b.branches[branchID]
    if !ok {
        return nil, fmt.Errorf("branch not found: %s", branchID)
    }

    branch.Status = BranchRunning

    // Load parent timeline
    parentTimeline, err := NewExecutionTimeline(ctx, b.store, branch.ParentExecutionID)
    if err != nil {
        branch.Status = BranchFailed
        return nil, err
    }

    // Replay with modification
    result, err := b.replay.ReplayWithModification(ctx, branch.BranchPointSeq, branch.Modification)
    if err != nil {
        branch.Status = BranchFailed
        return nil, err
    }

    // Create timeline for the branch
    branchTimeline, err := NewExecutionTimeline(ctx, b.store, result.ReplayExecutionID)
    if err == nil {
        branch.Timeline = branchTimeline
    }

    branch.Status = BranchCompleted
    return result, nil
}

// CompareBranches compares two execution branches
func (b *BranchingEngine) CompareBranches(ctx context.Context, branchID1, branchID2 string) (*BranchComparison, error) {
    branch1, ok := b.branches[branchID1]
    if !ok {
        return nil, fmt.Errorf("branch not found: %s", branchID1)
    }
    branch2, ok := b.branches[branchID2]
    if !ok {
        return nil, fmt.Errorf("branch not found: %s", branchID2)
    }

    return &BranchComparison{
        Branch1:     branch1,
        Branch2:     branch2,
        Differences: b.computeDifferences(branch1.Timeline, branch2.Timeline),
    }, nil
}

type BranchComparison struct {
    Branch1     *ExecutionBranch
    Branch2     *ExecutionBranch
    Differences []*StateDifference
}
```

---

## Phase 3: Debug API Server

### 3.1 API Types

**File: `/debug/api/types.go`**

```go
package api

import (
    "time"

    "github.com/tmc/minion/debug/snapshot"
    "github.com/tmc/minion/debug/timetravel"
)

// Execution listing
type ExecutionListRequest struct {
    AgentID   string     `json:"agent_id,omitempty"`
    FromTime  *time.Time `json:"from_time,omitempty"`
    ToTime    *time.Time `json:"to_time,omitempty"`
    HasError  *bool      `json:"has_error,omitempty"`
    Limit     int        `json:"limit,omitempty"`
    Offset    int        `json:"offset,omitempty"`
}

type ExecutionListResponse struct {
    Executions []ExecutionInfo `json:"executions"`
    TotalCount int64           `json:"total_count"`
    HasMore    bool            `json:"has_more"`
}

type ExecutionInfo struct {
    ID          string                        `json:"id"`
    AgentID     string                        `json:"agent_id"`
    StartTime   time.Time                     `json:"start_time"`
    EndTime     time.Time                     `json:"end_time"`
    Duration    time.Duration                 `json:"duration"`
    StepCount   int                           `json:"step_count"`
    ErrorCount  int                           `json:"error_count"`
    Status      string                        `json:"status"`
}

// Timeline navigation
type TimelineRequest struct {
    ExecutionID string `json:"execution_id"`
}

type TimelineResponse struct {
    ExecutionID string                          `json:"execution_id"`
    Summary     *timetravel.ExecutionSummary    `json:"summary"`
    Snapshots   []*snapshot.ExecutionSnapshot   `json:"snapshots"`
}

// Step navigation
type StepRequest struct {
    ExecutionID string `json:"execution_id"`
    Direction   string `json:"direction"` // "forward", "backward", "jump"
    TargetSeq   int64  `json:"target_seq,omitempty"`
    Checkpoint  string `json:"checkpoint,omitempty"` // Jump to checkpoint type
}

type StepResponse struct {
    Current     *snapshot.ExecutionSnapshot     `json:"current"`
    Position    int                             `json:"position"`
    Total       int                             `json:"total"`
    CanForward  bool                            `json:"can_forward"`
    CanBackward bool                            `json:"can_backward"`
}

// State reconstruction
type StateRequest struct {
    ExecutionID string `json:"execution_id"`
    SequenceNum int64  `json:"sequence_num"`
}

type StateResponse struct {
    State *timetravel.ReconstructedState `json:"state"`
}

// Replay
type ReplayRequest struct {
    ExecutionID   string                         `json:"execution_id"`
    FromSequence  int64                          `json:"from_sequence"`
    Modification  *timetravel.Modification       `json:"modification,omitempty"`
    Options       *timetravel.ReplayOptions      `json:"options,omitempty"`
}

type ReplayResponse struct {
    Result *timetravel.ReplayResult `json:"result"`
}

// Branching
type BranchRequest struct {
    ExecutionID  string                    `json:"execution_id"`
    SequenceNum  int64                     `json:"sequence_num"`
    Modification *timetravel.Modification  `json:"modification"`
}

type BranchResponse struct {
    BranchID string                      `json:"branch_id"`
    Branch   *timetravel.ExecutionBranch `json:"branch"`
}

// Comparison
type CompareRequest struct {
    ExecutionID1 string `json:"execution_id_1"`
    ExecutionID2 string `json:"execution_id_2"`
}

type CompareResponse struct {
    Comparison *timetravel.BranchComparison `json:"comparison"`
}

// Search
type SearchRequest struct {
    Query       string            `json:"query"`
    Filters     map[string]string `json:"filters,omitempty"`
    Limit       int               `json:"limit,omitempty"`
}

type SearchResponse struct {
    Results    []SearchResult `json:"results"`
    TotalCount int64          `json:"total_count"`
}

type SearchResult struct {
    ExecutionID string                       `json:"execution_id"`
    SequenceNum int64                        `json:"sequence_num"`
    Snapshot    *snapshot.ExecutionSnapshot  `json:"snapshot"`
    Score       float64                      `json:"score"`
    Highlights  []string                     `json:"highlights"`
}
```

### 3.2 HTTP Server

**File: `/debug/api/server.go`**

```go
package api

import (
    "context"
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/tmc/minion/debug/snapshot"
    "github.com/tmc/minion/debug/timetravel"
)

type DebugServer struct {
    store     snapshot.SnapshotStore
    timelines map[string]*timetravel.ExecutionTimeline
    branching *timetravel.BranchingEngine
    router    chi.Router
    server    *http.Server
}

type DebugServerConfig struct {
    Addr          string
    Store         snapshot.SnapshotStore
    EnableCORS    bool
    EnableSwagger bool
}

func NewDebugServer(config DebugServerConfig) *DebugServer {
    s := &DebugServer{
        store:     config.Store,
        timelines: make(map[string]*timetravel.ExecutionTimeline),
    }

    r := chi.NewRouter()

    // Middleware
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(middleware.RequestID)
    if config.EnableCORS {
        r.Use(corsMiddleware)
    }

    // Routes
    r.Route("/api/v1", func(r chi.Router) {
        // Executions
        r.Get("/executions", s.listExecutions)
        r.Get("/executions/{executionID}", s.getExecution)

        // Timeline
        r.Get("/executions/{executionID}/timeline", s.getTimeline)
        r.Post("/executions/{executionID}/step", s.step)

        // State
        r.Get("/executions/{executionID}/state/{seqNum}", s.getState)

        // Replay
        r.Post("/executions/{executionID}/replay", s.replay)

        // Branching
        r.Post("/executions/{executionID}/branch", s.createBranch)
        r.Get("/branches/{branchID}", s.getBranch)
        r.Post("/branches/{branchID}/execute", s.executeBranch)
        r.Get("/branches/compare", s.compareBranches)

        // Search
        r.Post("/search", s.search)

        // Export
        r.Get("/executions/{executionID}/export", s.exportExecution)
    })

    // Swagger/OpenAPI
    if config.EnableSwagger {
        r.Get("/swagger/*", httpSwagger.Handler())
    }

    s.router = r
    s.server = &http.Server{
        Addr:    config.Addr,
        Handler: r,
    }

    return s
}

func (s *DebugServer) Start() error {
    return s.server.ListenAndServe()
}

func (s *DebugServer) Shutdown(ctx context.Context) error {
    return s.server.Shutdown(ctx)
}

// Handler implementations

func (s *DebugServer) listExecutions(w http.ResponseWriter, r *http.Request) {
    var req ExecutionListRequest
    // Parse query params into req...

    // Query store
    query := &snapshot.SnapshotQuery{
        AgentID:  req.AgentID,
        FromTime: req.FromTime,
        ToTime:   req.ToTime,
        Limit:    req.Limit,
        Offset:   req.Offset,
    }

    result, err := s.store.Query(r.Context(), query)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Group by execution ID and build response
    // ...

    json.NewEncoder(w).Encode(response)
}

func (s *DebugServer) getTimeline(w http.ResponseWriter, r *http.Request) {
    executionID := chi.URLParam(r, "executionID")

    timeline, err := s.getOrCreateTimeline(r.Context(), executionID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    response := TimelineResponse{
        ExecutionID: executionID,
        Summary:     timeline.Summary(),
        Snapshots:   timeline.GetRange(0, int64(len(timeline.snapshots))),
    }

    json.NewEncoder(w).Encode(response)
}

func (s *DebugServer) step(w http.ResponseWriter, r *http.Request) {
    executionID := chi.URLParam(r, "executionID")

    var req StepRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    timeline, err := s.getOrCreateTimeline(r.Context(), executionID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    var current *snapshot.ExecutionSnapshot
    switch req.Direction {
    case "forward":
        current = timeline.StepForward()
    case "backward":
        current = timeline.StepBackward()
    case "jump":
        current = timeline.JumpTo(req.TargetSeq)
    }

    response := StepResponse{
        Current:     current,
        Position:    timeline.cursor,
        Total:       len(timeline.snapshots),
        CanForward:  timeline.cursor < len(timeline.snapshots)-1,
        CanBackward: timeline.cursor > 0,
    }

    json.NewEncoder(w).Encode(response)
}

// ... more handlers
```

---

## Phase 4: Visualization (Minion Studio)

### 4.1 Terminal UI (Bubble Tea)

For a Go-native debugging experience:

**File: `/debug/studio/tui/main.go`**

```go
package tui

import (
    "fmt"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/tmc/minion/debug/api"
)

type Model struct {
    client      *api.Client
    executions  []api.ExecutionInfo
    timeline    *TimelineView
    stateView   *StateView
    cursor      int
    width       int
    height      int
    mode        ViewMode
}

type ViewMode int

const (
    ModeExecutionList ViewMode = iota
    ModeTimeline
    ModeStateInspector
    ModeDiff
)

func (m Model) Init() tea.Cmd {
    return m.fetchExecutions
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "j", "down":
            m.cursor++
        case "k", "up":
            m.cursor--
        case "enter":
            return m, m.selectExecution
        case "left", "h":
            return m, m.stepBackward
        case "right", "l":
            return m, m.stepForward
        case "e":
            return m, m.jumpToError
        case "r":
            return m, m.startReplay
        case "b":
            return m, m.createBranch
        case "tab":
            m.mode = (m.mode + 1) % 4
        }
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    case ExecutionsMsg:
        m.executions = msg.Executions
    case TimelineMsg:
        m.timeline = NewTimelineView(msg.Timeline)
    }
    return m, nil
}

func (m Model) View() string {
    switch m.mode {
    case ModeExecutionList:
        return m.renderExecutionList()
    case ModeTimeline:
        return m.renderTimeline()
    case ModeStateInspector:
        return m.renderStateInspector()
    case ModeDiff:
        return m.renderDiff()
    default:
        return ""
    }
}

func (m Model) renderTimeline() string {
    // Render timeline with current position marker
    // [====|=====X=====|====]
    //       ^cursor

    var s string
    s += m.renderHeader()
    s += "\n"
    s += m.timeline.Render(m.width)
    s += "\n"
    s += m.renderCurrentSnapshot()
    s += "\n"
    s += m.renderControls()
    return s
}
```

### 4.2 Web UI (React)

For a full-featured studio experience:

**Structure:**
```
/debug/studio/web/
├── src/
│   ├── components/
│   │   ├── ExecutionList.tsx
│   │   ├── Timeline.tsx
│   │   ├── StateInspector.tsx
│   │   ├── DiffViewer.tsx
│   │   ├── BranchManager.tsx
│   │   └── SearchPanel.tsx
│   ├── hooks/
│   │   ├── useExecution.ts
│   │   ├── useTimeline.ts
│   │   └── useDebugAPI.ts
│   ├── pages/
│   │   ├── Dashboard.tsx
│   │   ├── ExecutionDetail.tsx
│   │   └── Compare.tsx
│   └── App.tsx
├── package.json
└── vite.config.ts
```

---

## Phase 5: Integration with Existing Code

### 5.1 Framework Integration

**File: `/core/framework.go` modifications**

```go
// Add to FrameworkImpl
type FrameworkImpl struct {
    // ... existing fields
    debugRecorder *recorder.ExecutionRecorder
    debugEnabled  bool
}

// Add option
func WithDebugRecorder(rec *recorder.ExecutionRecorder) Option {
    return func(f *FrameworkImpl) {
        f.debugRecorder = rec
        f.debugEnabled = true
    }
}

// Modify Execute to record checkpoints
func (f *FrameworkImpl) Execute(ctx context.Context, agentID string, input *models.Input) (*models.Output, error) {
    executionID := uuid.New().String()

    if f.debugEnabled {
        f.debugRecorder.StartExecution(ctx, executionID)
        defer f.debugRecorder.EndExecution(ctx)
    }

    // Record input checkpoint
    if f.debugEnabled {
        f.debugRecorder.RecordCheckpoint(ctx, &recorder.Checkpoint{
            Type:    snapshot.CheckpointTaskStarted,
            AgentID: agentID,
            Input:   input,
        })
    }

    // ... existing execution logic with checkpoints at key points
}
```

### 5.2 Multi-Agent Integration

**File: `/core/multiagent/orchestrator.go` modifications**

```go
// Add recorder to Orchestrator
type Orchestrator struct {
    // ... existing fields
    debugRecorder *recorder.ExecutionRecorder
}

// Record checkpoints in ExecuteTask
func (o *Orchestrator) ExecuteTask(ctx context.Context, req *TaskRequest) (*TaskResult, error) {
    // Record task creation
    if o.debugRecorder != nil {
        o.debugRecorder.RecordCheckpoint(ctx, &recorder.Checkpoint{
            Type:   snapshot.CheckpointTaskCreated,
            TaskID: task.ID,
            Input:  req,
        })
    }

    // ... existing logic with more checkpoints
}
```

---

## Implementation Timeline

### Phase 1: Snapshot System (Foundation)
- Snapshot types and interfaces
- Memory and PostgreSQL stores
- Execution recorder
- Integration hooks

### Phase 2: Time-Travel Engine (Core)
- Timeline navigation
- State reconstruction
- Replay engine
- Branching engine

### Phase 3: Debug API (Access Layer)
- HTTP/gRPC server
- Query interface
- Export functionality
- WebSocket for real-time updates

### Phase 4: Visualization (User Interface)
- Terminal UI (Bubble Tea)
- Web UI (React)
- Graph visualization
- Diff viewer

### Phase 5: Advanced Features
- Full-text search
- Anomaly detection
- Performance profiling
- Cost analysis dashboard

---

## Directory Structure

```
/debug/
├── snapshot/
│   ├── types.go           # Snapshot data types
│   ├── store.go           # Store interface
│   ├── store_memory.go    # In-memory implementation
│   └── store_postgres.go  # PostgreSQL implementation
├── recorder/
│   ├── recorder.go        # Execution recorder
│   └── hooks.go           # Integration hooks
├── timetravel/
│   ├── timeline.go        # Timeline navigation
│   ├── reconstructor.go   # State reconstruction
│   ├── replay.go          # Replay engine
│   └── branching.go       # Branching/what-if
├── api/
│   ├── types.go           # API request/response types
│   ├── server.go          # HTTP server
│   ├── handlers.go        # Route handlers
│   └── websocket.go       # Real-time updates
├── studio/
│   ├── tui/               # Terminal UI
│   │   ├── main.go
│   │   └── views/
│   └── web/               # React Web UI
│       ├── src/
│       └── package.json
└── README.md
```

---

## Key Differentiators from LangGraph Studio

| Feature | LangGraph Studio | Minion Debug Studio |
|---------|------------------|---------------------|
| **Performance** | Python overhead | Go native speed |
| **Deployment** | Cloud-dependent | Self-hosted option |
| **Protocol** | LangChain-specific | KQML standard |
| **Branching** | Limited | Full what-if analysis |
| **Terminal UI** | ❌ | ✅ Native TUI |
| **Cost Tracking** | Separate tool | Integrated |
| **Multi-Agent** | Graph-based | Orchestrator-based |
| **State Capture** | Checkpointing | Full snapshots |
| **Replay** | Time-travel | Replay + modification |

This implementation brings debugging and time-travel capabilities to Minion while leveraging Go's performance advantages and the framework's existing observability infrastructure.
