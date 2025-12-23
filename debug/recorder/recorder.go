// Package recorder provides execution recording capabilities for debugging and time-travel.
package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/Ranganaths/minion/debug/snapshot"
)

// ExecutionRecorder captures snapshots during execution for debugging and time-travel.
type ExecutionRecorder struct {
	store       snapshot.SnapshotStore
	config      RecorderConfig
	enabled     atomic.Bool
	executionID string
	agentID     string
	sequenceNum atomic.Int64
	startTime   time.Time

	mu       sync.RWMutex
	metadata map[string]any

	// Callbacks for external integration
	onCheckpoint []CheckpointCallback
}

// RecorderConfig configures what the recorder captures.
type RecorderConfig struct {
	// What to capture
	CaptureSessionState   bool
	CaptureTaskState      bool
	CaptureWorkspace      bool
	CaptureInputOutput    bool
	CaptureFullLLMContext bool // Can be expensive - captures full prompts/responses

	// Checkpoint filters - nil means all enabled
	EnabledCheckpoints map[snapshot.CheckpointType]bool

	// Sampling (for high-frequency checkpoints)
	SampleRate float64 // 0.0 to 1.0, 1.0 = capture all

	// Size limits for truncation
	MaxInputSize  int // Max bytes for input capture
	MaxOutputSize int // Max bytes for output capture

	// Auto-flush settings
	FlushInterval time.Duration
	BatchSize     int

	// Retention
	AutoPurgeAge time.Duration // 0 = never auto-purge
}

// DefaultRecorderConfig returns sensible default configuration.
func DefaultRecorderConfig() RecorderConfig {
	return RecorderConfig{
		CaptureSessionState:   true,
		CaptureTaskState:      true,
		CaptureWorkspace:      true,
		CaptureInputOutput:    true,
		CaptureFullLLMContext: false,
		EnabledCheckpoints:    nil, // All enabled by default
		SampleRate:            1.0, // Capture all
		MaxInputSize:          64 * 1024,  // 64KB
		MaxOutputSize:         64 * 1024,  // 64KB
		FlushInterval:         time.Second,
		BatchSize:             100,
		AutoPurgeAge:          7 * 24 * time.Hour, // 7 days
	}
}

// CheckpointCallback is called when a checkpoint is recorded.
type CheckpointCallback func(ctx context.Context, snap *snapshot.ExecutionSnapshot)

// Checkpoint represents a point in execution to capture.
type Checkpoint struct {
	Type      snapshot.CheckpointType
	AgentID   string
	TaskID    string
	WorkerID  string
	SessionID string

	// State to capture
	Session   *SessionState
	Task      *TaskState
	Workspace map[string]any

	// Action details
	Action *ActionInfo

	// Input/Output
	Input  any
	Output any

	// Error if any
	Error error

	// Trace context
	TraceID      string
	SpanID       string
	ParentSpanID string

	// Additional metadata
	Metadata map[string]any
}

// SessionState represents session state to capture.
type SessionState struct {
	ID        string
	AgentID   string
	UserID    string
	Status    string
	History   []MessageInfo
	Workspace map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
}

// MessageInfo represents a message in session history.
type MessageInfo struct {
	Role      string
	Content   string
	Name      string
	ToolCalls []ToolCallInfo
	Timestamp time.Time
}

// ToolCallInfo represents a tool call in a message.
type ToolCallInfo struct {
	ID        string
	Name      string
	Arguments string
}

// TaskState represents task state to capture.
type TaskState struct {
	ID           string
	Name         string
	Description  string
	Type         string
	Priority     string
	Status       string
	AssignedTo   string
	CreatedBy    string
	Dependencies []string
	Input        any
	Output       any
	Error        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ActionInfo represents an action being taken.
type ActionInfo struct {
	Type    string // "tool_call", "llm_call", "decision", "message"
	Name    string // Tool name, model name, etc.
	Input   any
	Output  any
	Started time.Time
	Ended   time.Time
	Success bool

	// LLM-specific
	Model            string
	Provider         string
	PromptTokens     int
	CompletionTokens int
	Cost             float64

	// Tool-specific
	ToolName string
}

// NewExecutionRecorder creates a new execution recorder.
func NewExecutionRecorder(store snapshot.SnapshotStore, config RecorderConfig) *ExecutionRecorder {
	r := &ExecutionRecorder{
		store:    store,
		config:   config,
		metadata: make(map[string]any),
	}
	r.enabled.Store(true)
	return r
}

// StartExecution begins recording a new execution.
func (r *ExecutionRecorder) StartExecution(ctx context.Context, agentID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.executionID = uuid.New().String()
	r.agentID = agentID
	r.sequenceNum.Store(0)
	r.startTime = time.Now()
	r.metadata = make(map[string]any)
	r.enabled.Store(true)

	return r.executionID
}

// StartExecutionWithID begins recording with a specific execution ID.
func (r *ExecutionRecorder) StartExecutionWithID(ctx context.Context, executionID, agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.executionID = executionID
	r.agentID = agentID
	r.sequenceNum.Store(0)
	r.startTime = time.Now()
	r.metadata = make(map[string]any)
	r.enabled.Store(true)
}

// EndExecution marks the execution as complete.
func (r *ExecutionRecorder) EndExecution(ctx context.Context) {
	r.enabled.Store(false)
}

// GetExecutionID returns the current execution ID.
func (r *ExecutionRecorder) GetExecutionID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.executionID
}

// SetMetadata sets execution-level metadata.
func (r *ExecutionRecorder) SetMetadata(key string, value any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metadata[key] = value
}

// GetMetadata gets execution-level metadata.
func (r *ExecutionRecorder) GetMetadata(key string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.metadata[key]
	return v, ok
}

// Enable enables recording.
func (r *ExecutionRecorder) Enable() {
	r.enabled.Store(true)
}

// Disable disables recording.
func (r *ExecutionRecorder) Disable() {
	r.enabled.Store(false)
}

// IsEnabled returns whether recording is enabled.
func (r *ExecutionRecorder) IsEnabled() bool {
	return r.enabled.Load()
}

// OnCheckpoint registers a callback for checkpoint events.
func (r *ExecutionRecorder) OnCheckpoint(cb CheckpointCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onCheckpoint = append(r.onCheckpoint, cb)
}

// RecordCheckpoint captures a snapshot at the current point.
func (r *ExecutionRecorder) RecordCheckpoint(ctx context.Context, cp *Checkpoint) error {
	if !r.enabled.Load() {
		return nil
	}

	if !r.shouldCapture(cp.Type) {
		return nil
	}

	// Build snapshot
	snap := &snapshot.ExecutionSnapshot{
		ExecutionID:    r.executionID,
		SequenceNum:    r.sequenceNum.Add(1),
		Timestamp:      time.Now(),
		CheckpointType: cp.Type,
		AgentID:        r.agentID,
		TaskID:         cp.TaskID,
		WorkerID:       cp.WorkerID,
		SessionID:      cp.SessionID,
		TraceID:        cp.TraceID,
		SpanID:         cp.SpanID,
		ParentSpanID:   cp.ParentSpanID,
	}

	// Use checkpoint's agent ID if specified
	if cp.AgentID != "" {
		snap.AgentID = cp.AgentID
	}

	// Capture session state
	if r.config.CaptureSessionState && cp.Session != nil {
		snap.SessionState = r.captureSessionState(cp.Session)
	}

	// Capture task state
	if r.config.CaptureTaskState && cp.Task != nil {
		snap.TaskState = r.captureTaskState(cp.Task)
	}

	// Capture workspace
	if r.config.CaptureWorkspace && cp.Workspace != nil {
		snap.WorkspaceState = cp.Workspace
	}

	// Capture action
	if cp.Action != nil {
		snap.Action = r.captureAction(cp.Action)
	}

	// Capture input/output
	if r.config.CaptureInputOutput {
		snap.Input = r.truncateValue(cp.Input, r.config.MaxInputSize)
		snap.Output = r.truncateValue(cp.Output, r.config.MaxOutputSize)
	}

	// Capture error
	if cp.Error != nil {
		snap.Error = &snapshot.ErrorSnapshot{
			Type:    fmt.Sprintf("%T", cp.Error),
			Message: cp.Error.Error(),
		}
	}

	// Add metadata
	snap.Metadata = r.mergeMetadata(cp.Metadata)

	// Save snapshot
	if err := r.store.Save(ctx, snap); err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	// Notify callbacks
	r.notifyCallbacks(ctx, snap)

	return nil
}

// Convenience methods for common checkpoint types

// RecordTaskCreated records a task creation checkpoint.
func (r *ExecutionRecorder) RecordTaskCreated(ctx context.Context, task *TaskState, input any) error {
	return r.RecordCheckpoint(ctx, &Checkpoint{
		Type:  snapshot.CheckpointTaskCreated,
		Task:  task,
		Input: input,
	})
}

// RecordTaskStarted records a task start checkpoint.
func (r *ExecutionRecorder) RecordTaskStarted(ctx context.Context, task *TaskState) error {
	return r.RecordCheckpoint(ctx, &Checkpoint{
		Type: snapshot.CheckpointTaskStarted,
		Task: task,
	})
}

// RecordTaskCompleted records a task completion checkpoint.
func (r *ExecutionRecorder) RecordTaskCompleted(ctx context.Context, task *TaskState, output any) error {
	return r.RecordCheckpoint(ctx, &Checkpoint{
		Type:   snapshot.CheckpointTaskCompleted,
		Task:   task,
		Output: output,
	})
}

// RecordTaskFailed records a task failure checkpoint.
func (r *ExecutionRecorder) RecordTaskFailed(ctx context.Context, task *TaskState, err error) error {
	return r.RecordCheckpoint(ctx, &Checkpoint{
		Type:  snapshot.CheckpointTaskFailed,
		Task:  task,
		Error: err,
	})
}

// RecordToolCallStart records the start of a tool call.
func (r *ExecutionRecorder) RecordToolCallStart(ctx context.Context, toolName string, input any) error {
	return r.RecordCheckpoint(ctx, &Checkpoint{
		Type: snapshot.CheckpointToolCallStart,
		Action: &ActionInfo{
			Type:     "tool_call",
			Name:     toolName,
			ToolName: toolName,
			Input:    input,
			Started:  time.Now(),
		},
		Input: input,
	})
}

// RecordToolCallEnd records the end of a tool call.
func (r *ExecutionRecorder) RecordToolCallEnd(ctx context.Context, toolName string, output any, duration time.Duration, err error) error {
	cp := &Checkpoint{
		Type: snapshot.CheckpointToolCallEnd,
		Action: &ActionInfo{
			Type:     "tool_call",
			Name:     toolName,
			ToolName: toolName,
			Output:   output,
			Ended:    time.Now(),
			Success:  err == nil,
		},
		Output: output,
	}
	if err != nil {
		cp.Error = err
	}
	return r.RecordCheckpoint(ctx, cp)
}

// RecordLLMCallStart records the start of an LLM call.
func (r *ExecutionRecorder) RecordLLMCallStart(ctx context.Context, provider, model string, input any) error {
	return r.RecordCheckpoint(ctx, &Checkpoint{
		Type: snapshot.CheckpointLLMCallStart,
		Action: &ActionInfo{
			Type:     "llm_call",
			Name:     model,
			Provider: provider,
			Model:    model,
			Input:    input,
			Started:  time.Now(),
		},
		Input: input,
	})
}

// RecordLLMCallEnd records the end of an LLM call.
func (r *ExecutionRecorder) RecordLLMCallEnd(ctx context.Context, provider, model string, output any, promptTokens, completionTokens int, cost float64, err error) error {
	cp := &Checkpoint{
		Type: snapshot.CheckpointLLMCallEnd,
		Action: &ActionInfo{
			Type:             "llm_call",
			Name:             model,
			Provider:         provider,
			Model:            model,
			Output:           output,
			Ended:            time.Now(),
			Success:          err == nil,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			Cost:             cost,
		},
		Output: output,
	}
	if err != nil {
		cp.Error = err
	}
	return r.RecordCheckpoint(ctx, cp)
}

// RecordAgentStep records an agent step.
func (r *ExecutionRecorder) RecordAgentStep(ctx context.Context, stepNum int, action, observation string) error {
	return r.RecordCheckpoint(ctx, &Checkpoint{
		Type: snapshot.CheckpointAgentStep,
		Action: &ActionInfo{
			Type: "agent_step",
			Name: fmt.Sprintf("step_%d", stepNum),
		},
		Metadata: map[string]any{
			"step_num":    stepNum,
			"action":      action,
			"observation": observation,
		},
	})
}

// RecordDecisionPoint records a decision point.
func (r *ExecutionRecorder) RecordDecisionPoint(ctx context.Context, decision string, options []string, chosen string) error {
	return r.RecordCheckpoint(ctx, &Checkpoint{
		Type: snapshot.CheckpointDecisionPoint,
		Action: &ActionInfo{
			Type: "decision",
			Name: decision,
		},
		Metadata: map[string]any{
			"decision": decision,
			"options":  options,
			"chosen":   chosen,
		},
	})
}

// RecordError records an error.
func (r *ExecutionRecorder) RecordError(ctx context.Context, err error, context map[string]any) error {
	return r.RecordCheckpoint(ctx, &Checkpoint{
		Type:     snapshot.CheckpointError,
		Error:    err,
		Metadata: context,
	})
}

// RecordMessage records a message sent or received.
func (r *ExecutionRecorder) RecordMessage(ctx context.Context, direction string, from, to string, content any) error {
	cpType := snapshot.CheckpointMessageSent
	if direction == "received" {
		cpType = snapshot.CheckpointMessageReceived
	}
	return r.RecordCheckpoint(ctx, &Checkpoint{
		Type:  cpType,
		Input: content,
		Metadata: map[string]any{
			"direction": direction,
			"from":      from,
			"to":        to,
		},
	})
}

// RecordSessionUpdate records a session update.
func (r *ExecutionRecorder) RecordSessionUpdate(ctx context.Context, session *SessionState) error {
	return r.RecordCheckpoint(ctx, &Checkpoint{
		Type:      snapshot.CheckpointSessionUpdate,
		Session:   session,
		SessionID: session.ID,
	})
}

// RecordWorkspaceUpdate records a workspace update.
func (r *ExecutionRecorder) RecordWorkspaceUpdate(ctx context.Context, sessionID string, workspace map[string]any) error {
	return r.RecordCheckpoint(ctx, &Checkpoint{
		Type:      snapshot.CheckpointWorkspaceUpdate,
		SessionID: sessionID,
		Workspace: workspace,
	})
}

// Helper methods

func (r *ExecutionRecorder) shouldCapture(cpType snapshot.CheckpointType) bool {
	// Check if this checkpoint type is enabled
	if r.config.EnabledCheckpoints != nil {
		if enabled, ok := r.config.EnabledCheckpoints[cpType]; ok && !enabled {
			return false
		}
	}

	// Apply sampling
	if r.config.SampleRate < 1.0 {
		// Simple deterministic sampling based on sequence number
		// This ensures consistent sampling for replay
		if float64(r.sequenceNum.Load()%100)/100.0 >= r.config.SampleRate {
			return false
		}
	}

	return true
}

func (r *ExecutionRecorder) captureSessionState(session *SessionState) *snapshot.SessionSnapshot {
	if session == nil {
		return nil
	}

	snap := &snapshot.SessionSnapshot{
		ID:        session.ID,
		AgentID:   session.AgentID,
		UserID:    session.UserID,
		Status:    session.Status,
		Workspace: session.Workspace,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
	}

	// Convert messages
	for _, msg := range session.History {
		m := snapshot.Message{
			Role:      msg.Role,
			Content:   msg.Content,
			Name:      msg.Name,
			Timestamp: msg.Timestamp,
		}
		for _, tc := range msg.ToolCalls {
			m.ToolCalls = append(m.ToolCalls, snapshot.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
		}
		snap.History = append(snap.History, m)
	}

	return snap
}

func (r *ExecutionRecorder) captureTaskState(task *TaskState) *snapshot.TaskSnapshot {
	if task == nil {
		return nil
	}

	return &snapshot.TaskSnapshot{
		ID:           task.ID,
		Name:         task.Name,
		Description:  task.Description,
		Type:         task.Type,
		Priority:     task.Priority,
		Status:       task.Status,
		AssignedTo:   task.AssignedTo,
		CreatedBy:    task.CreatedBy,
		Dependencies: task.Dependencies,
		Input:        task.Input,
		Output:       task.Output,
		Error:        task.Error,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}
}

func (r *ExecutionRecorder) captureAction(action *ActionInfo) *snapshot.ActionSnapshot {
	if action == nil {
		return nil
	}

	durationMs := int64(0)
	if !action.Ended.IsZero() && !action.Started.IsZero() {
		durationMs = action.Ended.Sub(action.Started).Milliseconds()
	}

	return &snapshot.ActionSnapshot{
		Type:             action.Type,
		Name:             action.Name,
		Input:            action.Input,
		Output:           action.Output,
		DurationMs:       durationMs,
		Success:          action.Success,
		Model:            action.Model,
		Provider:         action.Provider,
		PromptTokens:     action.PromptTokens,
		CompletionTokens: action.CompletionTokens,
		Cost:             action.Cost,
		ToolName:         action.ToolName,
	}
}

func (r *ExecutionRecorder) truncateValue(value any, maxSize int) any {
	if value == nil || maxSize <= 0 {
		return value
	}

	// Try to serialize and check size
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}

	if len(data) <= maxSize {
		return value
	}

	// Truncate the JSON and add indicator
	truncated := string(data[:maxSize-50]) + `..."[TRUNCATED]"`
	var result any
	json.Unmarshal([]byte(truncated), &result)
	if result == nil {
		// If unmarshaling fails, return a simple truncation indicator
		return map[string]any{
			"_truncated": true,
			"_size":      len(data),
			"_maxSize":   maxSize,
		}
	}
	return result
}

func (r *ExecutionRecorder) mergeMetadata(cpMetadata map[string]any) map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]any)

	// Copy execution-level metadata
	for k, v := range r.metadata {
		result[k] = v
	}

	// Override with checkpoint-level metadata
	for k, v := range cpMetadata {
		result[k] = v
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func (r *ExecutionRecorder) notifyCallbacks(ctx context.Context, snap *snapshot.ExecutionSnapshot) {
	r.mu.RLock()
	callbacks := r.onCheckpoint
	r.mu.RUnlock()

	for _, cb := range callbacks {
		cb(ctx, snap)
	}
}
