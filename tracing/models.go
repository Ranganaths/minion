// Package tracing provides comprehensive execution tracing for agents.
// It captures LLM calls, tool invocations, reasoning steps, and timing data
// at execution time, enabling full observability of agent behavior.
package tracing

import (
	"encoding/json"
	"time"
)

// TraceID uniquely identifies a trace
type TraceID string

// SpanID uniquely identifies a span within a trace
type SpanID string

// SpanType categorizes the type of operation being traced
type SpanType string

const (
	SpanTypeAgentExecution SpanType = "agent_execution"
	SpanTypeLLMCall        SpanType = "llm_call"
	SpanTypeToolCall       SpanType = "tool_call"
	SpanTypeThought        SpanType = "thought"
	SpanTypeDecision       SpanType = "decision"
	SpanTypeIteration      SpanType = "iteration"
)

// SpanStatus indicates the outcome of a span
type SpanStatus string

const (
	SpanStatusUnset   SpanStatus = "unset"
	SpanStatusOK      SpanStatus = "ok"
	SpanStatusError   SpanStatus = "error"
	SpanStatusRunning SpanStatus = "running"
)

// Trace represents a complete execution trace for an agent run
type Trace struct {
	// ID uniquely identifies this trace
	ID TraceID `json:"id"`

	// AgentID is the ID of the agent being traced
	AgentID string `json:"agent_id"`

	// AgentName is the human-readable name of the agent
	AgentName string `json:"agent_name"`

	// SessionID links this trace to a conversation session
	SessionID string `json:"session_id,omitempty"`

	// Input is the original user input that triggered this execution
	Input string `json:"input"`

	// Output is the final response from the agent
	Output string `json:"output,omitempty"`

	// Status indicates overall execution status
	Status SpanStatus `json:"status"`

	// Error contains error details if status is error
	Error string `json:"error,omitempty"`

	// StartTime when the trace began
	StartTime time.Time `json:"start_time"`

	// EndTime when the trace completed
	EndTime *time.Time `json:"end_time,omitempty"`

	// Duration in milliseconds
	Duration int64 `json:"duration_ms"`

	// RootSpanID is the ID of the root span
	RootSpanID SpanID `json:"root_span_id"`

	// Spans contains all spans in execution order
	Spans []*Span `json:"spans"`

	// TotalTokens is the sum of all LLM tokens used
	TotalTokens TokenUsage `json:"total_tokens"`

	// TotalCost is the sum of all LLM costs
	TotalCost float64 `json:"total_cost"`

	// ToolCallCount is the number of tool invocations
	ToolCallCount int `json:"tool_call_count"`

	// IterationCount is the number of agent loop iterations
	IterationCount int `json:"iteration_count"`

	// Metadata contains additional trace metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Span represents a single operation within a trace
type Span struct {
	// ID uniquely identifies this span
	ID SpanID `json:"id"`

	// TraceID links this span to its parent trace
	TraceID TraceID `json:"trace_id"`

	// ParentSpanID is the ID of the parent span (empty for root)
	ParentSpanID SpanID `json:"parent_span_id,omitempty"`

	// Type categorizes this span
	Type SpanType `json:"type"`

	// Name is a human-readable name for this span
	Name string `json:"name"`

	// Status indicates the outcome
	Status SpanStatus `json:"status"`

	// StartTime when this span began
	StartTime time.Time `json:"start_time"`

	// EndTime when this span completed
	EndTime *time.Time `json:"end_time,omitempty"`

	// Duration in milliseconds
	Duration int64 `json:"duration_ms"`

	// Input is the input to this operation
	Input *SpanInput `json:"input,omitempty"`

	// Output is the output from this operation
	Output *SpanOutput `json:"output,omitempty"`

	// Error contains error details if status is error
	Error *SpanError `json:"error,omitempty"`

	// LLMDetails contains LLM-specific data (for llm_call spans)
	LLMDetails *LLMSpanDetails `json:"llm_details,omitempty"`

	// ToolDetails contains tool-specific data (for tool_call spans)
	ToolDetails *ToolSpanDetails `json:"tool_details,omitempty"`

	// ThoughtDetails contains reasoning data (for thought/decision spans)
	ThoughtDetails *ThoughtSpanDetails `json:"thought_details,omitempty"`

	// Events are timestamped events within this span
	Events []*SpanEvent `json:"events,omitempty"`

	// Attributes are key-value pairs for this span
	Attributes map[string]interface{} `json:"attributes,omitempty"`

	// ChildSpanIDs lists direct child spans
	ChildSpanIDs []SpanID `json:"child_span_ids,omitempty"`
}

// SpanInput represents input data for a span
type SpanInput struct {
	// Raw is the raw input string or data
	Raw string `json:"raw,omitempty"`

	// Structured contains structured input data
	Structured map[string]interface{} `json:"structured,omitempty"`

	// Truncated indicates if input was truncated for storage
	Truncated bool `json:"truncated,omitempty"`
}

// SpanOutput represents output data for a span
type SpanOutput struct {
	// Raw is the raw output string or data
	Raw string `json:"raw,omitempty"`

	// Structured contains structured output data
	Structured map[string]interface{} `json:"structured,omitempty"`

	// Truncated indicates if output was truncated for storage
	Truncated bool `json:"truncated,omitempty"`
}

// SpanError represents error information
type SpanError struct {
	// Type is the error type/category
	Type string `json:"type"`

	// Message is the error message
	Message string `json:"message"`

	// Stack is the stack trace if available
	Stack string `json:"stack,omitempty"`

	// Attributes are additional error attributes
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// SpanEvent is a timestamped event within a span
type SpanEvent struct {
	// Name is the event name
	Name string `json:"name"`

	// Timestamp when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Attributes are event attributes
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// LLMSpanDetails contains LLM-specific span data
type LLMSpanDetails struct {
	// Provider is the LLM provider (openai, anthropic, etc.)
	Provider string `json:"provider"`

	// Model is the model identifier
	Model string `json:"model"`

	// SystemPrompt is the system prompt sent to the LLM
	SystemPrompt string `json:"system_prompt,omitempty"`

	// UserPrompt is the user prompt sent to the LLM
	UserPrompt string `json:"user_prompt,omitempty"`

	// Response is the LLM response
	Response string `json:"response,omitempty"`

	// Temperature used for generation
	Temperature float64 `json:"temperature"`

	// MaxTokens limit requested
	MaxTokens int `json:"max_tokens,omitempty"`

	// Tokens is the token usage breakdown
	Tokens TokenUsage `json:"tokens"`

	// Cost is the estimated cost for this call
	Cost float64 `json:"cost"`

	// FinishReason from the LLM
	FinishReason string `json:"finish_reason,omitempty"`

	// PromptTruncated indicates if prompt was truncated for storage
	PromptTruncated bool `json:"prompt_truncated,omitempty"`

	// ResponseTruncated indicates if response was truncated for storage
	ResponseTruncated bool `json:"response_truncated,omitempty"`
}

// TokenUsage tracks token consumption
type TokenUsage struct {
	// PromptTokens is the number of tokens in the prompt
	PromptTokens int `json:"prompt_tokens"`

	// CompletionTokens is the number of tokens in the completion
	CompletionTokens int `json:"completion_tokens"`

	// TotalTokens is the total tokens used
	TotalTokens int `json:"total_tokens"`
}

// ToolSpanDetails contains tool-specific span data
type ToolSpanDetails struct {
	// ToolName is the name of the tool invoked
	ToolName string `json:"tool_name"`

	// ToolDescription describes what the tool does
	ToolDescription string `json:"tool_description,omitempty"`

	// Input is the input passed to the tool
	Input string `json:"input"`

	// Output is the output returned by the tool
	Output string `json:"output,omitempty"`

	// Retries is the number of retry attempts
	Retries int `json:"retries,omitempty"`

	// Timeout indicates if the tool timed out
	Timeout bool `json:"timeout,omitempty"`
}

// ThoughtSpanDetails contains reasoning/thought span data
type ThoughtSpanDetails struct {
	// Thought is the agent's reasoning text
	Thought string `json:"thought"`

	// Action is the decided action (if decision span)
	Action string `json:"action,omitempty"`

	// ActionInput is the input for the decided action
	ActionInput string `json:"action_input,omitempty"`

	// Confidence is the agent's confidence in this decision (0-1)
	Confidence float64 `json:"confidence,omitempty"`

	// Alternatives are alternative actions considered
	Alternatives []string `json:"alternatives,omitempty"`

	// Iteration is the iteration number within the agent loop
	Iteration int `json:"iteration"`

	// IsFinalAnswer indicates if this is the final answer decision
	IsFinalAnswer bool `json:"is_final_answer,omitempty"`
}

// TraceFilter defines filtering criteria for traces
type TraceFilter struct {
	// AgentID filters by agent
	AgentID string `json:"agent_id,omitempty"`

	// SessionID filters by session
	SessionID string `json:"session_id,omitempty"`

	// Status filters by status
	Status SpanStatus `json:"status,omitempty"`

	// MinDuration filters traces with duration >= this value (ms)
	MinDuration int64 `json:"min_duration_ms,omitempty"`

	// MaxDuration filters traces with duration <= this value (ms)
	MaxDuration int64 `json:"max_duration_ms,omitempty"`

	// StartTimeFrom filters traces started after this time
	StartTimeFrom *time.Time `json:"start_time_from,omitempty"`

	// StartTimeTo filters traces started before this time
	StartTimeTo *time.Time `json:"start_time_to,omitempty"`

	// HasError filters traces with/without errors
	HasError *bool `json:"has_error,omitempty"`

	// MinTokens filters traces with token usage >= this value
	MinTokens int `json:"min_tokens,omitempty"`

	// ToolName filters traces that used a specific tool
	ToolName string `json:"tool_name,omitempty"`
}

// TraceQuery combines filter with pagination
type TraceQuery struct {
	// Filter defines filtering criteria
	Filter TraceFilter `json:"filter"`

	// Limit is the maximum number of traces to return
	Limit int `json:"limit,omitempty"`

	// Offset is the number of traces to skip
	Offset int `json:"offset,omitempty"`

	// OrderBy specifies sort order (e.g., "start_time", "duration")
	OrderBy string `json:"order_by,omitempty"`

	// OrderDesc specifies descending order
	OrderDesc bool `json:"order_desc,omitempty"`
}

// TraceQueryResult is the result of a trace query
type TraceQueryResult struct {
	// Traces are the matching traces
	Traces []*Trace `json:"traces"`

	// TotalCount is the total number of matching traces
	TotalCount int64 `json:"total_count"`

	// HasMore indicates if there are more results
	HasMore bool `json:"has_more"`
}

// TraceSummary provides a high-level overview of a trace
type TraceSummary struct {
	// ID is the trace ID
	ID TraceID `json:"id"`

	// AgentID is the agent ID
	AgentID string `json:"agent_id"`

	// AgentName is the agent name
	AgentName string `json:"agent_name"`

	// Input is the input (may be truncated)
	Input string `json:"input"`

	// Output is the output (may be truncated)
	Output string `json:"output,omitempty"`

	// Status is the trace status
	Status SpanStatus `json:"status"`

	// StartTime is when the trace started
	StartTime time.Time `json:"start_time"`

	// Duration in milliseconds
	Duration int64 `json:"duration_ms"`

	// TotalTokens used
	TotalTokens int `json:"total_tokens"`

	// TotalCost of LLM calls
	TotalCost float64 `json:"total_cost"`

	// ToolCallCount is the number of tool calls
	ToolCallCount int `json:"tool_call_count"`

	// IterationCount is the number of iterations
	IterationCount int `json:"iteration_count"`

	// HasError indicates if the trace has an error
	HasError bool `json:"has_error"`
}

// ToSummary converts a Trace to TraceSummary
func (t *Trace) ToSummary() *TraceSummary {
	input := t.Input
	if len(input) > 200 {
		input = input[:200] + "..."
	}

	output := t.Output
	if len(output) > 200 {
		output = output[:200] + "..."
	}

	return &TraceSummary{
		ID:             t.ID,
		AgentID:        t.AgentID,
		AgentName:      t.AgentName,
		Input:          input,
		Output:         output,
		Status:         t.Status,
		StartTime:      t.StartTime,
		Duration:       t.Duration,
		TotalTokens:    t.TotalTokens.TotalTokens,
		TotalCost:      t.TotalCost,
		ToolCallCount:  t.ToolCallCount,
		IterationCount: t.IterationCount,
		HasError:       t.Status == SpanStatusError,
	}
}

// ToJSON serializes the trace to JSON
func (t *Trace) ToJSON() ([]byte, error) {
	return json.Marshal(t)
}

// SpanTree builds a tree structure from flat spans
func (t *Trace) SpanTree() *SpanNode {
	if len(t.Spans) == 0 {
		return nil
	}

	// Build a map for quick lookup
	spanMap := make(map[SpanID]*SpanNode)
	for _, span := range t.Spans {
		spanMap[span.ID] = &SpanNode{
			Span:     span,
			Children: []*SpanNode{},
		}
	}

	// Build tree relationships
	var root *SpanNode
	for _, span := range t.Spans {
		node := spanMap[span.ID]
		if span.ParentSpanID == "" {
			root = node
		} else if parent, ok := spanMap[span.ParentSpanID]; ok {
			parent.Children = append(parent.Children, node)
		}
	}

	return root
}

// SpanNode represents a node in the span tree
type SpanNode struct {
	Span     *Span       `json:"span"`
	Children []*SpanNode `json:"children,omitempty"`
}

// GetSpansByType returns all spans of a given type
func (t *Trace) GetSpansByType(spanType SpanType) []*Span {
	var result []*Span
	for _, span := range t.Spans {
		if span.Type == spanType {
			result = append(result, span)
		}
	}
	return result
}

// GetLLMCalls returns all LLM call spans
func (t *Trace) GetLLMCalls() []*Span {
	return t.GetSpansByType(SpanTypeLLMCall)
}

// GetToolCalls returns all tool call spans
func (t *Trace) GetToolCalls() []*Span {
	return t.GetSpansByType(SpanTypeToolCall)
}

// GetThoughts returns all thought spans
func (t *Trace) GetThoughts() []*Span {
	return t.GetSpansByType(SpanTypeThought)
}

// GetDecisions returns all decision spans
func (t *Trace) GetDecisions() []*Span {
	return t.GetSpansByType(SpanTypeDecision)
}
