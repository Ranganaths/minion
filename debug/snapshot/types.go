// Package snapshot provides execution snapshot types for debugging and time-travel.
package snapshot

import (
	"time"
)

// CheckpointType represents the type of checkpoint in an execution.
type CheckpointType string

const (
	// Task lifecycle checkpoints
	CheckpointTaskCreated   CheckpointType = "task_created"
	CheckpointTaskAssigned  CheckpointType = "task_assigned"
	CheckpointTaskStarted   CheckpointType = "task_started"
	CheckpointTaskCompleted CheckpointType = "task_completed"
	CheckpointTaskFailed    CheckpointType = "task_failed"
	CheckpointTaskRetry     CheckpointType = "task_retry"

	// Tool checkpoints
	CheckpointToolCallStart CheckpointType = "tool_call_start"
	CheckpointToolCallEnd   CheckpointType = "tool_call_end"

	// LLM checkpoints
	CheckpointLLMCallStart CheckpointType = "llm_call_start"
	CheckpointLLMCallEnd   CheckpointType = "llm_call_end"

	// Agent checkpoints
	CheckpointAgentStep      CheckpointType = "agent_step"
	CheckpointAgentPlan      CheckpointType = "agent_plan"
	CheckpointAgentAction    CheckpointType = "agent_action"
	CheckpointDecisionPoint  CheckpointType = "decision_point"

	// State checkpoints
	CheckpointStateChange   CheckpointType = "state_change"
	CheckpointSessionUpdate CheckpointType = "session_update"
	CheckpointWorkspaceUpdate CheckpointType = "workspace_update"

	// Communication checkpoints
	CheckpointMessageSent     CheckpointType = "message_sent"
	CheckpointMessageReceived CheckpointType = "message_received"

	// User interaction checkpoints
	CheckpointUserInput  CheckpointType = "user_input"
	CheckpointUserOutput CheckpointType = "user_output"

	// Error checkpoint
	CheckpointError CheckpointType = "error"
)

// ExecutionSnapshot captures complete state at a point in time during execution.
type ExecutionSnapshot struct {
	// Identity
	ID          string `json:"id"`
	ExecutionID string `json:"execution_id"` // Links snapshots together

	// Ordering
	SequenceNum int64     `json:"sequence_num"` // Ordering within execution
	Timestamp   time.Time `json:"timestamp"`

	// Checkpoint classification
	CheckpointType CheckpointType `json:"checkpoint_type"`

	// Context identifiers
	AgentID  string `json:"agent_id,omitempty"`
	TaskID   string `json:"task_id,omitempty"`
	WorkerID string `json:"worker_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`

	// State captures
	SessionState   *SessionSnapshot   `json:"session_state,omitempty"`
	TaskState      *TaskSnapshot      `json:"task_state,omitempty"`
	WorkspaceState map[string]any     `json:"workspace_state,omitempty"`

	// Execution context
	Action *ActionSnapshot `json:"action,omitempty"`
	Input  any             `json:"input,omitempty"`
	Output any             `json:"output,omitempty"`

	// Observability links
	TraceID      string `json:"trace_id,omitempty"`
	SpanID       string `json:"span_id,omitempty"`
	ParentSpanID string `json:"parent_span_id,omitempty"`

	// Error context (if any)
	Error *ErrorSnapshot `json:"error,omitempty"`

	// Metadata for extensibility
	Metadata map[string]any `json:"metadata,omitempty"`
}

// SessionSnapshot captures session state at a point in time.
type SessionSnapshot struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	UserID    string    `json:"user_id,omitempty"`
	Status    string    `json:"status"`
	History   []Message `json:"history"`
	Workspace map[string]any `json:"workspace"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message represents a conversation message in session history.
type Message struct {
	Role      string         `json:"role"` // user, assistant, system, tool
	Content   string         `json:"content"`
	Name      string         `json:"name,omitempty"`
	ToolCalls []ToolCall     `json:"tool_calls,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// ToolCall represents a tool invocation in a message.
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Arguments string `json:"arguments"`
}

// TaskSnapshot captures task state at a point in time.
type TaskSnapshot struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Type         string   `json:"type,omitempty"`
	Priority     string   `json:"priority,omitempty"`
	Status       string   `json:"status"`
	AssignedTo   string   `json:"assigned_to,omitempty"`
	CreatedBy    string   `json:"created_by,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Input        any      `json:"input,omitempty"`
	Output       any      `json:"output,omitempty"`
	Error        string   `json:"error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ActionSnapshot captures details of an action taken during execution.
type ActionSnapshot struct {
	Type       string `json:"type"`        // "tool_call", "llm_call", "decision", "message"
	Name       string `json:"name"`        // Tool name, model name, etc.
	Input      any    `json:"input,omitempty"`
	Output     any    `json:"output,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	Success    bool   `json:"success"`

	// LLM-specific fields
	Model           string `json:"model,omitempty"`
	Provider        string `json:"provider,omitempty"`
	PromptTokens    int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int   `json:"completion_tokens,omitempty"`
	Cost            float64 `json:"cost,omitempty"`

	// Tool-specific fields
	ToolName string `json:"tool_name,omitempty"`
}

// ErrorSnapshot captures error details for debugging.
type ErrorSnapshot struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	Code       string `json:"code,omitempty"`
	StackTrace string `json:"stack_trace,omitempty"`
	Cause      string `json:"cause,omitempty"`
	Retryable  bool   `json:"retryable"`
}

// ExecutionSummary provides a summary of an entire execution.
type ExecutionSummary struct {
	ExecutionID      string                     `json:"execution_id"`
	AgentID          string                     `json:"agent_id,omitempty"`
	StartTime        time.Time                  `json:"start_time"`
	EndTime          time.Time                  `json:"end_time"`
	Duration         time.Duration              `json:"duration"`
	TotalSteps       int                        `json:"total_steps"`
	ErrorCount       int                        `json:"error_count"`
	CheckpointCounts map[CheckpointType]int     `json:"checkpoint_counts"`
	Status           string                     `json:"status"` // "running", "completed", "failed"
	FinalOutput      any                        `json:"final_output,omitempty"`
	FinalError       *ErrorSnapshot             `json:"final_error,omitempty"`
}

// SnapshotFilter defines criteria for querying snapshots.
type SnapshotFilter struct {
	ExecutionID    string           `json:"execution_id,omitempty"`
	AgentID        string           `json:"agent_id,omitempty"`
	TaskID         string           `json:"task_id,omitempty"`
	SessionID      string           `json:"session_id,omitempty"`
	CheckpointType CheckpointType   `json:"checkpoint_type,omitempty"`
	CheckpointTypes []CheckpointType `json:"checkpoint_types,omitempty"`
	FromTime       *time.Time       `json:"from_time,omitempty"`
	ToTime         *time.Time       `json:"to_time,omitempty"`
	FromSequence   *int64           `json:"from_sequence,omitempty"`
	ToSequence     *int64           `json:"to_sequence,omitempty"`
	HasError       *bool            `json:"has_error,omitempty"`
	TraceID        string           `json:"trace_id,omitempty"`
}

// SnapshotQuery combines filter with pagination and ordering.
type SnapshotQuery struct {
	Filter  SnapshotFilter `json:"filter"`
	Limit   int            `json:"limit,omitempty"`
	Offset  int            `json:"offset,omitempty"`
	OrderBy string         `json:"order_by,omitempty"` // "sequence_asc", "sequence_desc", "time_asc", "time_desc"
}

// SnapshotQueryResult contains query results with pagination info.
type SnapshotQueryResult struct {
	Snapshots  []*ExecutionSnapshot `json:"snapshots"`
	TotalCount int64                `json:"total_count"`
	HasMore    bool                 `json:"has_more"`
}

// StoreStats provides statistics about the snapshot store.
type StoreStats struct {
	TotalSnapshots   int64     `json:"total_snapshots"`
	TotalExecutions  int64     `json:"total_executions"`
	OldestSnapshot   time.Time `json:"oldest_snapshot"`
	NewestSnapshot   time.Time `json:"newest_snapshot"`
	StorageSizeBytes int64     `json:"storage_size_bytes"`
}
