// Package recorder provides integration hooks for the Minion framework.
package recorder

import (
	"context"
	"time"

	"github.com/Ranganaths/minion/debug/snapshot"
)

// FrameworkHooks provides integration hooks for recording framework events.
type FrameworkHooks struct {
	recorder *ExecutionRecorder
}

// NewFrameworkHooks creates hooks for framework integration.
func NewFrameworkHooks(recorder *ExecutionRecorder) *FrameworkHooks {
	return &FrameworkHooks{recorder: recorder}
}

// Recorder returns the underlying recorder.
func (h *FrameworkHooks) Recorder() *ExecutionRecorder {
	return h.recorder
}

// AgentHooks provides hooks for agent execution.
type AgentHooks struct {
	hooks   *FrameworkHooks
	agentID string
}

// ForAgent creates agent-specific hooks.
func (h *FrameworkHooks) ForAgent(agentID string) *AgentHooks {
	return &AgentHooks{
		hooks:   h,
		agentID: agentID,
	}
}

// OnExecutionStart records the start of agent execution.
func (h *AgentHooks) OnExecutionStart(ctx context.Context, input any) error {
	h.hooks.recorder.StartExecution(ctx, h.agentID)
	return h.hooks.recorder.RecordCheckpoint(ctx, &Checkpoint{
		Type:    snapshot.CheckpointTaskStarted,
		AgentID: h.agentID,
		Input:   input,
	})
}

// OnExecutionEnd records the end of agent execution.
func (h *AgentHooks) OnExecutionEnd(ctx context.Context, output any, err error) error {
	cpType := snapshot.CheckpointTaskCompleted
	if err != nil {
		cpType = snapshot.CheckpointTaskFailed
	}

	recordErr := h.hooks.recorder.RecordCheckpoint(ctx, &Checkpoint{
		Type:    cpType,
		AgentID: h.agentID,
		Output:  output,
		Error:   err,
	})

	h.hooks.recorder.EndExecution(ctx)
	return recordErr
}

// OnStep records an agent step.
func (h *AgentHooks) OnStep(ctx context.Context, stepNum int, action, observation string) error {
	return h.hooks.recorder.RecordAgentStep(ctx, stepNum, action, observation)
}

// OnPlan records an agent planning action.
func (h *AgentHooks) OnPlan(ctx context.Context, plan any) error {
	return h.hooks.recorder.RecordCheckpoint(ctx, &Checkpoint{
		Type:    snapshot.CheckpointAgentPlan,
		AgentID: h.agentID,
		Output:  plan,
		Metadata: map[string]any{
			"type": "plan",
		},
	})
}

// OnAction records an agent action.
func (h *AgentHooks) OnAction(ctx context.Context, action string, input any) error {
	return h.hooks.recorder.RecordCheckpoint(ctx, &Checkpoint{
		Type:    snapshot.CheckpointAgentAction,
		AgentID: h.agentID,
		Action: &ActionInfo{
			Type:  "agent_action",
			Name:  action,
			Input: input,
		},
		Input: input,
	})
}

// ToolHooks provides hooks for tool execution.
type ToolHooks struct {
	hooks     *FrameworkHooks
	toolName  string
	startTime time.Time
}

// ForTool creates tool-specific hooks.
func (h *FrameworkHooks) ForTool(toolName string) *ToolHooks {
	return &ToolHooks{
		hooks:    h,
		toolName: toolName,
	}
}

// OnStart records the start of a tool call.
func (h *ToolHooks) OnStart(ctx context.Context, input any) error {
	h.startTime = time.Now()
	return h.hooks.recorder.RecordToolCallStart(ctx, h.toolName, input)
}

// OnEnd records the end of a tool call.
func (h *ToolHooks) OnEnd(ctx context.Context, output any, err error) error {
	duration := time.Since(h.startTime)
	return h.hooks.recorder.RecordToolCallEnd(ctx, h.toolName, output, duration, err)
}

// LLMHooks provides hooks for LLM calls.
type LLMHooks struct {
	hooks     *FrameworkHooks
	provider  string
	model     string
	startTime time.Time
}

// ForLLM creates LLM-specific hooks.
func (h *FrameworkHooks) ForLLM(provider, model string) *LLMHooks {
	return &LLMHooks{
		hooks:    h,
		provider: provider,
		model:    model,
	}
}

// OnStart records the start of an LLM call.
func (h *LLMHooks) OnStart(ctx context.Context, input any) error {
	h.startTime = time.Now()
	return h.hooks.recorder.RecordLLMCallStart(ctx, h.provider, h.model, input)
}

// OnEnd records the end of an LLM call.
func (h *LLMHooks) OnEnd(ctx context.Context, output any, promptTokens, completionTokens int, cost float64, err error) error {
	return h.hooks.recorder.RecordLLMCallEnd(ctx, h.provider, h.model, output, promptTokens, completionTokens, cost, err)
}

// TaskHooks provides hooks for multi-agent task execution.
type TaskHooks struct {
	hooks  *FrameworkHooks
	taskID string
}

// ForTask creates task-specific hooks.
func (h *FrameworkHooks) ForTask(taskID string) *TaskHooks {
	return &TaskHooks{
		hooks:  h,
		taskID: taskID,
	}
}

// OnCreated records task creation.
func (h *TaskHooks) OnCreated(ctx context.Context, task *TaskState) error {
	return h.hooks.recorder.RecordTaskCreated(ctx, task, task.Input)
}

// OnAssigned records task assignment.
func (h *TaskHooks) OnAssigned(ctx context.Context, task *TaskState, workerID string) error {
	task.AssignedTo = workerID
	return h.hooks.recorder.RecordCheckpoint(ctx, &Checkpoint{
		Type:     snapshot.CheckpointTaskAssigned,
		TaskID:   h.taskID,
		WorkerID: workerID,
		Task:     task,
		Metadata: map[string]any{
			"assigned_to": workerID,
		},
	})
}

// OnStarted records task start.
func (h *TaskHooks) OnStarted(ctx context.Context, task *TaskState) error {
	return h.hooks.recorder.RecordTaskStarted(ctx, task)
}

// OnCompleted records task completion.
func (h *TaskHooks) OnCompleted(ctx context.Context, task *TaskState, output any) error {
	return h.hooks.recorder.RecordTaskCompleted(ctx, task, output)
}

// OnFailed records task failure.
func (h *TaskHooks) OnFailed(ctx context.Context, task *TaskState, err error) error {
	return h.hooks.recorder.RecordTaskFailed(ctx, task, err)
}

// OnRetry records a task retry.
func (h *TaskHooks) OnRetry(ctx context.Context, task *TaskState, attempt int, err error) error {
	return h.hooks.recorder.RecordCheckpoint(ctx, &Checkpoint{
		Type:   snapshot.CheckpointTaskRetry,
		TaskID: h.taskID,
		Task:   task,
		Error:  err,
		Metadata: map[string]any{
			"attempt": attempt,
		},
	})
}

// SessionHooks provides hooks for session updates.
type SessionHooks struct {
	hooks     *FrameworkHooks
	sessionID string
}

// ForSession creates session-specific hooks.
func (h *FrameworkHooks) ForSession(sessionID string) *SessionHooks {
	return &SessionHooks{
		hooks:     h,
		sessionID: sessionID,
	}
}

// OnUpdate records a session update.
func (h *SessionHooks) OnUpdate(ctx context.Context, session *SessionState) error {
	return h.hooks.recorder.RecordSessionUpdate(ctx, session)
}

// OnWorkspaceUpdate records a workspace update.
func (h *SessionHooks) OnWorkspaceUpdate(ctx context.Context, workspace map[string]any) error {
	return h.hooks.recorder.RecordWorkspaceUpdate(ctx, h.sessionID, workspace)
}

// OnMessage records a message added to the session.
func (h *SessionHooks) OnMessage(ctx context.Context, role, content string) error {
	return h.hooks.recorder.RecordCheckpoint(ctx, &Checkpoint{
		Type:      snapshot.CheckpointSessionUpdate,
		SessionID: h.sessionID,
		Metadata: map[string]any{
			"role":    role,
			"content": truncateContent(content, 500),
		},
	})
}

// MessageHooks provides hooks for inter-agent messages.
type MessageHooks struct {
	hooks *FrameworkHooks
}

// ForMessages creates message hooks.
func (h *FrameworkHooks) ForMessages() *MessageHooks {
	return &MessageHooks{hooks: h}
}

// OnSent records a message being sent.
func (h *MessageHooks) OnSent(ctx context.Context, from, to string, content any) error {
	return h.hooks.recorder.RecordMessage(ctx, "sent", from, to, content)
}

// OnReceived records a message being received.
func (h *MessageHooks) OnReceived(ctx context.Context, from, to string, content any) error {
	return h.hooks.recorder.RecordMessage(ctx, "received", from, to, content)
}

// DecisionHooks provides hooks for decision points.
type DecisionHooks struct {
	hooks *FrameworkHooks
}

// ForDecisions creates decision hooks.
func (h *FrameworkHooks) ForDecisions() *DecisionHooks {
	return &DecisionHooks{hooks: h}
}

// OnDecision records a decision point.
func (h *DecisionHooks) OnDecision(ctx context.Context, decision string, options []string, chosen string) error {
	return h.hooks.recorder.RecordDecisionPoint(ctx, decision, options, chosen)
}

// ErrorHooks provides hooks for error recording.
type ErrorHooks struct {
	hooks *FrameworkHooks
}

// ForErrors creates error hooks.
func (h *FrameworkHooks) ForErrors() *ErrorHooks {
	return &ErrorHooks{hooks: h}
}

// OnError records an error.
func (h *ErrorHooks) OnError(ctx context.Context, err error, context map[string]any) error {
	return h.hooks.recorder.RecordError(ctx, err, context)
}

// Helper function to truncate content
func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ChainCallback implements chain.ChainCallback for automatic recording.
type ChainCallback struct {
	hooks *FrameworkHooks
}

// NewChainCallback creates a chain callback that records to the debug system.
func NewChainCallback(hooks *FrameworkHooks) *ChainCallback {
	return &ChainCallback{hooks: hooks}
}

// OnChainStart is called when a chain starts.
func (c *ChainCallback) OnChainStart(ctx context.Context, chainName string, inputs map[string]any) {
	c.hooks.recorder.RecordCheckpoint(ctx, &Checkpoint{
		Type: snapshot.CheckpointTaskStarted,
		Metadata: map[string]any{
			"chain_name": chainName,
			"inputs":     inputs,
		},
	})
}

// OnChainEnd is called when a chain ends.
func (c *ChainCallback) OnChainEnd(ctx context.Context, chainName string, outputs map[string]any) {
	c.hooks.recorder.RecordCheckpoint(ctx, &Checkpoint{
		Type: snapshot.CheckpointTaskCompleted,
		Metadata: map[string]any{
			"chain_name": chainName,
			"outputs":    outputs,
		},
	})
}

// OnChainError is called when a chain errors.
func (c *ChainCallback) OnChainError(ctx context.Context, chainName string, err error) {
	c.hooks.recorder.RecordCheckpoint(ctx, &Checkpoint{
		Type:  snapshot.CheckpointTaskFailed,
		Error: err,
		Metadata: map[string]any{
			"chain_name": chainName,
		},
	})
}

// OnLLMStart is called when an LLM call starts.
func (c *ChainCallback) OnLLMStart(ctx context.Context, prompt string) {
	c.hooks.recorder.RecordLLMCallStart(ctx, "chain", "llm", prompt)
}

// OnLLMEnd is called when an LLM call ends.
func (c *ChainCallback) OnLLMEnd(ctx context.Context, response string) {
	c.hooks.recorder.RecordLLMCallEnd(ctx, "chain", "llm", response, 0, 0, 0, nil)
}

// OnRetrieverStart is called when a retriever starts.
func (c *ChainCallback) OnRetrieverStart(ctx context.Context, query string) {
	c.hooks.recorder.RecordCheckpoint(ctx, &Checkpoint{
		Type: snapshot.CheckpointToolCallStart,
		Action: &ActionInfo{
			Type:  "retriever",
			Name:  "vector_search",
			Input: query,
		},
	})
}

// OnRetrieverEnd is called when a retriever ends.
func (c *ChainCallback) OnRetrieverEnd(ctx context.Context, documents []any) {
	c.hooks.recorder.RecordCheckpoint(ctx, &Checkpoint{
		Type: snapshot.CheckpointToolCallEnd,
		Action: &ActionInfo{
			Type:   "retriever",
			Name:   "vector_search",
			Output: documents,
		},
	})
}
