package multiagent

import (
	"context"
	"sync"
	"time"
)

// ExecutionEvent represents a single execution event in the history
type ExecutionEvent struct {
	ID            string                 `json:"id"`
	Type          ExecutionEventType     `json:"type"`
	TaskID        string                 `json:"task_id"`
	AgentID       string                 `json:"agent_id"`
	AgentRole     AgentRole              `json:"agent_role"`
	Action        string                 `json:"action"`
	Input         interface{}            `json:"input,omitempty"`
	Output        interface{}            `json:"output,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Duration      time.Duration          `json:"duration"`
	Timestamp     time.Time              `json:"timestamp"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	TraceContext  *TraceContext          `json:"trace_context,omitempty"`
}

// ExecutionEventType defines types of execution events
type ExecutionEventType string

const (
	EventTypeTaskCreated     ExecutionEventType = "task_created"
	EventTypeTaskAssigned    ExecutionEventType = "task_assigned"
	EventTypeTaskStarted     ExecutionEventType = "task_started"
	EventTypeTaskCompleted   ExecutionEventType = "task_completed"
	EventTypeTaskFailed      ExecutionEventType = "task_failed"
	EventTypeSubtaskCreated  ExecutionEventType = "subtask_created"
	EventTypeWorkerStarted   ExecutionEventType = "worker_started"
	EventTypeWorkerCompleted ExecutionEventType = "worker_completed"
	EventTypeLLMCall         ExecutionEventType = "llm_call"
	EventTypeToolCall        ExecutionEventType = "tool_call"
	EventTypeMessageSent     ExecutionEventType = "message_sent"
	EventTypeMessageReceived ExecutionEventType = "message_received"
	EventTypePlanning        ExecutionEventType = "planning"
	EventTypeReplanning      ExecutionEventType = "replanning"
)

// ExecutionTrace represents a complete execution trace
type ExecutionTrace struct {
	ExecutionID   string                 `json:"execution_id"`
	RootTaskID    string                 `json:"root_task_id"`
	RootTraceID   string                 `json:"root_trace_id"`
	OrchestratorID string                `json:"orchestrator_id"`
	Status        string                 `json:"status"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	Duration      time.Duration          `json:"duration,omitempty"`
	Events        []*ExecutionEvent      `json:"events"`
	TaskTree      *TaskTreeNode          `json:"task_tree,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// TaskTreeNode represents a node in the task hierarchy
type TaskTreeNode struct {
	Task       *Task          `json:"task"`
	Children   []*TaskTreeNode `json:"children,omitempty"`
	Events     []*ExecutionEvent `json:"events,omitempty"`
	WorkerID   string         `json:"worker_id,omitempty"`
	Duration   time.Duration  `json:"duration,omitempty"`
}

// ExecutionHistoryQuery defines query parameters for searching execution history
type ExecutionHistoryQuery struct {
	// Filter by execution ID
	ExecutionID string `json:"execution_id,omitempty"`

	// Filter by root trace ID (for OpenTelemetry correlation)
	RootTraceID string `json:"root_trace_id,omitempty"`

	// Filter by orchestrator ID
	OrchestratorID string `json:"orchestrator_id,omitempty"`

	// Filter by agent ID
	AgentID string `json:"agent_id,omitempty"`

	// Filter by task ID
	TaskID string `json:"task_id,omitempty"`

	// Filter by event type
	EventTypes []ExecutionEventType `json:"event_types,omitempty"`

	// Filter by time range
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`

	// Filter by status
	Status string `json:"status,omitempty"`

	// Pagination
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`

	// Include nested events (subtasks, worker events)
	IncludeNested bool `json:"include_nested,omitempty"`
}

// ExecutionHistory provides an interface for tracking and querying agent execution history
type ExecutionHistory interface {
	// RecordEvent records an execution event
	RecordEvent(ctx context.Context, event *ExecutionEvent) error

	// GetTrace retrieves a complete execution trace by execution ID
	GetTrace(ctx context.Context, executionID string) (*ExecutionTrace, error)

	// GetTraceByRootTraceID retrieves trace by OpenTelemetry root trace ID
	GetTraceByRootTraceID(ctx context.Context, rootTraceID string) (*ExecutionTrace, error)

	// QueryEvents queries execution events with filters
	QueryEvents(ctx context.Context, query *ExecutionHistoryQuery) ([]*ExecutionEvent, error)

	// QueryTraces queries execution traces with filters
	QueryTraces(ctx context.Context, query *ExecutionHistoryQuery) ([]*ExecutionTrace, error)

	// GetTaskHistory retrieves all events for a specific task
	GetTaskHistory(ctx context.Context, taskID string) ([]*ExecutionEvent, error)

	// GetAgentHistory retrieves all events for a specific agent
	GetAgentHistory(ctx context.Context, agentID string) ([]*ExecutionEvent, error)

	// BuildTaskTree builds a hierarchical view of task execution
	BuildTaskTree(ctx context.Context, executionID string) (*TaskTreeNode, error)

	// GetExecutionMetrics returns metrics for an execution
	GetExecutionMetrics(ctx context.Context, executionID string) (*ExecutionMetrics, error)

	// Prune removes old execution history
	Prune(ctx context.Context, olderThan time.Time) error
}

// ExecutionMetrics contains aggregated metrics for an execution
type ExecutionMetrics struct {
	ExecutionID      string        `json:"execution_id"`
	TotalDuration    time.Duration `json:"total_duration"`
	TaskCount        int           `json:"task_count"`
	SubtaskCount     int           `json:"subtask_count"`
	CompletedTasks   int           `json:"completed_tasks"`
	FailedTasks      int           `json:"failed_tasks"`
	WorkerCount      int           `json:"worker_count"`
	LLMCalls         int           `json:"llm_calls"`
	TotalLLMDuration time.Duration `json:"total_llm_duration"`
	ToolCalls        int           `json:"tool_calls"`
	MessageCount     int           `json:"message_count"`
	ReplanningCount  int           `json:"replanning_count"`
}

// InMemoryExecutionHistory provides an in-memory implementation of ExecutionHistory
type InMemoryExecutionHistory struct {
	mu             sync.RWMutex
	events         []*ExecutionEvent
	traces         map[string]*ExecutionTrace // executionID -> trace
	tracesByRoot   map[string]string          // rootTraceID -> executionID
	eventsByTask   map[string][]*ExecutionEvent
	eventsByAgent  map[string][]*ExecutionEvent
}

// NewInMemoryExecutionHistory creates a new in-memory execution history
func NewInMemoryExecutionHistory() *InMemoryExecutionHistory {
	return &InMemoryExecutionHistory{
		events:        make([]*ExecutionEvent, 0),
		traces:        make(map[string]*ExecutionTrace),
		tracesByRoot:  make(map[string]string),
		eventsByTask:  make(map[string][]*ExecutionEvent),
		eventsByAgent: make(map[string][]*ExecutionEvent),
	}
}

// RecordEvent records an execution event
func (h *InMemoryExecutionHistory) RecordEvent(ctx context.Context, event *ExecutionEvent) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.events = append(h.events, event)

	// Index by task ID
	if event.TaskID != "" {
		h.eventsByTask[event.TaskID] = append(h.eventsByTask[event.TaskID], event)
	}

	// Index by agent ID
	if event.AgentID != "" {
		h.eventsByAgent[event.AgentID] = append(h.eventsByAgent[event.AgentID], event)
	}

	// Update or create trace if trace context exists
	if event.TraceContext != nil && event.TraceContext.ExecutionID != "" {
		trace, exists := h.traces[event.TraceContext.ExecutionID]
		if !exists {
			trace = &ExecutionTrace{
				ExecutionID:    event.TraceContext.ExecutionID,
				RootTraceID:    event.TraceContext.RootTraceID,
				OrchestratorID: event.TraceContext.OrchestratorID,
				Status:         "in_progress",
				StartTime:      event.Timestamp,
				Events:         make([]*ExecutionEvent, 0),
				Metadata:       make(map[string]interface{}),
			}
			h.traces[event.TraceContext.ExecutionID] = trace

			// Index by root trace ID
			if event.TraceContext.RootTraceID != "" {
				h.tracesByRoot[event.TraceContext.RootTraceID] = event.TraceContext.ExecutionID
			}
		}

		trace.Events = append(trace.Events, event)

		// Update trace status based on event
		if event.Type == EventTypeTaskCompleted && event.TaskID == trace.RootTaskID {
			trace.Status = "completed"
			now := time.Now()
			trace.EndTime = &now
			trace.Duration = now.Sub(trace.StartTime)
		} else if event.Type == EventTypeTaskFailed && event.TaskID == trace.RootTaskID {
			trace.Status = "failed"
			now := time.Now()
			trace.EndTime = &now
			trace.Duration = now.Sub(trace.StartTime)
		}

		// Set root task ID from first task created event
		if event.Type == EventTypeTaskCreated && trace.RootTaskID == "" {
			trace.RootTaskID = event.TaskID
		}
	}

	return nil
}

// GetTrace retrieves a complete execution trace by execution ID
func (h *InMemoryExecutionHistory) GetTrace(ctx context.Context, executionID string) (*ExecutionTrace, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	trace, exists := h.traces[executionID]
	if !exists {
		return nil, nil
	}

	return trace, nil
}

// GetTraceByRootTraceID retrieves trace by OpenTelemetry root trace ID
func (h *InMemoryExecutionHistory) GetTraceByRootTraceID(ctx context.Context, rootTraceID string) (*ExecutionTrace, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	executionID, exists := h.tracesByRoot[rootTraceID]
	if !exists {
		return nil, nil
	}

	return h.traces[executionID], nil
}

// QueryEvents queries execution events with filters
func (h *InMemoryExecutionHistory) QueryEvents(ctx context.Context, query *ExecutionHistoryQuery) ([]*ExecutionEvent, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*ExecutionEvent, 0)

	for _, event := range h.events {
		if h.matchesQuery(event, query) {
			result = append(result, event)
		}
	}

	// Apply pagination
	if query.Offset > 0 && query.Offset < len(result) {
		result = result[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(result) {
		result = result[:query.Limit]
	}

	return result, nil
}

// matchesQuery checks if an event matches the query filters
func (h *InMemoryExecutionHistory) matchesQuery(event *ExecutionEvent, query *ExecutionHistoryQuery) bool {
	// Filter by execution ID
	if query.ExecutionID != "" && (event.TraceContext == nil || event.TraceContext.ExecutionID != query.ExecutionID) {
		return false
	}

	// Filter by root trace ID
	if query.RootTraceID != "" && (event.TraceContext == nil || event.TraceContext.RootTraceID != query.RootTraceID) {
		return false
	}

	// Filter by orchestrator ID
	if query.OrchestratorID != "" && (event.TraceContext == nil || event.TraceContext.OrchestratorID != query.OrchestratorID) {
		return false
	}

	// Filter by agent ID
	if query.AgentID != "" && event.AgentID != query.AgentID {
		return false
	}

	// Filter by task ID
	if query.TaskID != "" && event.TaskID != query.TaskID {
		return false
	}

	// Filter by event types
	if len(query.EventTypes) > 0 {
		found := false
		for _, t := range query.EventTypes {
			if event.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by time range
	if query.StartTime != nil && event.Timestamp.Before(*query.StartTime) {
		return false
	}
	if query.EndTime != nil && event.Timestamp.After(*query.EndTime) {
		return false
	}

	return true
}

// QueryTraces queries execution traces with filters
func (h *InMemoryExecutionHistory) QueryTraces(ctx context.Context, query *ExecutionHistoryQuery) ([]*ExecutionTrace, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*ExecutionTrace, 0)

	for _, trace := range h.traces {
		if h.matchesTraceQuery(trace, query) {
			result = append(result, trace)
		}
	}

	// Apply pagination
	if query.Offset > 0 && query.Offset < len(result) {
		result = result[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(result) {
		result = result[:query.Limit]
	}

	return result, nil
}

// matchesTraceQuery checks if a trace matches the query filters
func (h *InMemoryExecutionHistory) matchesTraceQuery(trace *ExecutionTrace, query *ExecutionHistoryQuery) bool {
	if query.ExecutionID != "" && trace.ExecutionID != query.ExecutionID {
		return false
	}
	if query.RootTraceID != "" && trace.RootTraceID != query.RootTraceID {
		return false
	}
	if query.OrchestratorID != "" && trace.OrchestratorID != query.OrchestratorID {
		return false
	}
	if query.Status != "" && trace.Status != query.Status {
		return false
	}
	if query.StartTime != nil && trace.StartTime.Before(*query.StartTime) {
		return false
	}
	if query.EndTime != nil && trace.EndTime != nil && trace.EndTime.After(*query.EndTime) {
		return false
	}

	return true
}

// GetTaskHistory retrieves all events for a specific task
func (h *InMemoryExecutionHistory) GetTaskHistory(ctx context.Context, taskID string) ([]*ExecutionEvent, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	events, exists := h.eventsByTask[taskID]
	if !exists {
		return []*ExecutionEvent{}, nil
	}

	return events, nil
}

// GetAgentHistory retrieves all events for a specific agent
func (h *InMemoryExecutionHistory) GetAgentHistory(ctx context.Context, agentID string) ([]*ExecutionEvent, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	events, exists := h.eventsByAgent[agentID]
	if !exists {
		return []*ExecutionEvent{}, nil
	}

	return events, nil
}

// BuildTaskTree builds a hierarchical view of task execution
func (h *InMemoryExecutionHistory) BuildTaskTree(ctx context.Context, executionID string) (*TaskTreeNode, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	trace, exists := h.traces[executionID]
	if !exists {
		return nil, nil
	}

	// Build tree from events
	taskNodes := make(map[string]*TaskTreeNode)
	var rootNode *TaskTreeNode

	for _, event := range trace.Events {
		if event.Type == EventTypeTaskCreated || event.Type == EventTypeSubtaskCreated {
			task, ok := event.Input.(*Task)
			if !ok {
				continue
			}

			node := &TaskTreeNode{
				Task:     task,
				Children: make([]*TaskTreeNode, 0),
				Events:   make([]*ExecutionEvent, 0),
			}
			taskNodes[task.ID] = node

			// Check if this is root task
			parentID := task.GetParentTaskID()
			if parentID == "" {
				rootNode = node
			} else if parentNode, exists := taskNodes[parentID]; exists {
				parentNode.Children = append(parentNode.Children, node)
			}
		}

		// Add events to their corresponding task nodes
		if node, exists := taskNodes[event.TaskID]; exists {
			node.Events = append(node.Events, event)

			if event.Type == EventTypeTaskAssigned {
				if workerID, ok := event.Metadata["worker_id"].(string); ok {
					node.WorkerID = workerID
				}
			}
		}
	}

	return rootNode, nil
}

// GetExecutionMetrics returns metrics for an execution
func (h *InMemoryExecutionHistory) GetExecutionMetrics(ctx context.Context, executionID string) (*ExecutionMetrics, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	trace, exists := h.traces[executionID]
	if !exists {
		return nil, nil
	}

	metrics := &ExecutionMetrics{
		ExecutionID: executionID,
	}

	workerSet := make(map[string]bool)

	for _, event := range trace.Events {
		switch event.Type {
		case EventTypeTaskCreated:
			metrics.TaskCount++
		case EventTypeSubtaskCreated:
			metrics.SubtaskCount++
		case EventTypeTaskCompleted:
			metrics.CompletedTasks++
		case EventTypeTaskFailed:
			metrics.FailedTasks++
		case EventTypeLLMCall:
			metrics.LLMCalls++
			metrics.TotalLLMDuration += event.Duration
		case EventTypeToolCall:
			metrics.ToolCalls++
		case EventTypeMessageSent, EventTypeMessageReceived:
			metrics.MessageCount++
		case EventTypeReplanning:
			metrics.ReplanningCount++
		case EventTypeWorkerStarted:
			workerSet[event.AgentID] = true
		}
	}

	metrics.WorkerCount = len(workerSet)
	metrics.TotalDuration = trace.Duration

	return metrics, nil
}

// Prune removes old execution history
func (h *InMemoryExecutionHistory) Prune(ctx context.Context, olderThan time.Time) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove old events
	newEvents := make([]*ExecutionEvent, 0)
	for _, event := range h.events {
		if event.Timestamp.After(olderThan) {
			newEvents = append(newEvents, event)
		}
	}
	h.events = newEvents

	// Remove old traces
	for id, trace := range h.traces {
		if trace.StartTime.Before(olderThan) {
			delete(h.traces, id)
			if trace.RootTraceID != "" {
				delete(h.tracesByRoot, trace.RootTraceID)
			}
		}
	}

	// Rebuild indexes
	h.eventsByTask = make(map[string][]*ExecutionEvent)
	h.eventsByAgent = make(map[string][]*ExecutionEvent)
	for _, event := range h.events {
		if event.TaskID != "" {
			h.eventsByTask[event.TaskID] = append(h.eventsByTask[event.TaskID], event)
		}
		if event.AgentID != "" {
			h.eventsByAgent[event.AgentID] = append(h.eventsByAgent[event.AgentID], event)
		}
	}

	return nil
}

// Global execution history instance
var globalExecutionHistory ExecutionHistory

// InitExecutionHistory initializes the global execution history
func InitExecutionHistory(history ExecutionHistory) {
	globalExecutionHistory = history
}

// GetExecutionHistory returns the global execution history
func GetExecutionHistory() ExecutionHistory {
	if globalExecutionHistory == nil {
		globalExecutionHistory = NewInMemoryExecutionHistory()
	}
	return globalExecutionHistory
}

// RecordExecutionEvent is a convenience function to record events using the global history
func RecordExecutionEvent(ctx context.Context, event *ExecutionEvent) error {
	return GetExecutionHistory().RecordEvent(ctx, event)
}
