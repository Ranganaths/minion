// Package agui implements the AG-UI (Agent-User Interface) protocol for
// connecting AI agents to frontend applications with real-time streaming.
//
// AG-UI is an open protocol developed by CopilotKit that standardizes how
// frontend applications communicate with AI agents, supporting streaming,
// frontend tools, shared state, and custom events.
//
// Specification: https://docs.ag-ui.com/
package agui

import (
	"encoding/json"
	"time"
)

// Protocol version
const (
	ProtocolVersion = "1.0"
	ProtocolName    = "ag-ui"
)

// EventType represents the type of AG-UI event
type EventType string

const (
	// Lifecycle events
	EventRunStarted   EventType = "RUN_STARTED"
	EventRunFinished  EventType = "RUN_FINISHED"
	EventRunError     EventType = "RUN_ERROR"
	EventStepStarted  EventType = "STEP_STARTED"
	EventStepFinished EventType = "STEP_FINISHED"

	// Message events
	EventTextMessageStart   EventType = "TEXT_MESSAGE_START"
	EventTextMessageContent EventType = "TEXT_MESSAGE_CONTENT"
	EventTextMessageEnd     EventType = "TEXT_MESSAGE_END"

	// Tool events
	EventToolCallStart EventType = "TOOL_CALL_START"
	EventToolCallArgs  EventType = "TOOL_CALL_ARGS"
	EventToolCallEnd   EventType = "TOOL_CALL_END"

	// State events
	EventStateSnapshot EventType = "STATE_SNAPSHOT"
	EventStateDelta    EventType = "STATE_DELTA"

	// Custom events
	EventCustom EventType = "CUSTOM"

	// Raw events (for debugging)
	EventRaw EventType = "RAW"
)

// MessageRole represents the role of a message sender
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
)

// BaseEvent contains common fields for all events
type BaseEvent struct {
	// Type of the event
	Type EventType `json:"type"`

	// Timestamp when the event was created
	Timestamp time.Time `json:"timestamp,omitempty"`

	// RawEvent contains the original event data (for debugging)
	RawEvent json.RawMessage `json:"rawEvent,omitempty"`
}

// RunStartedEvent signals the start of an agent run
type RunStartedEvent struct {
	BaseEvent
	// ThreadID identifies the conversation thread
	ThreadID string `json:"threadId,omitempty"`
	// RunID identifies this specific run
	RunID string `json:"runId"`
}

// RunFinishedEvent signals the completion of an agent run
type RunFinishedEvent struct {
	BaseEvent
	// ThreadID identifies the conversation thread
	ThreadID string `json:"threadId,omitempty"`
	// RunID identifies this specific run
	RunID string `json:"runId"`
}

// RunErrorEvent signals an error during the agent run
type RunErrorEvent struct {
	BaseEvent
	// Message describes the error
	Message string `json:"message"`
	// Code is an optional error code
	Code string `json:"code,omitempty"`
}

// StepStartedEvent signals the start of a processing step
type StepStartedEvent struct {
	BaseEvent
	// StepName identifies the step
	StepName string `json:"stepName"`
}

// StepFinishedEvent signals the completion of a processing step
type StepFinishedEvent struct {
	BaseEvent
	// StepName identifies the step
	StepName string `json:"stepName"`
}

// TextMessageStartEvent signals the start of a text message
type TextMessageStartEvent struct {
	BaseEvent
	// MessageID identifies the message
	MessageID string `json:"messageId"`
	// Role of the message sender
	Role MessageRole `json:"role"`
}

// TextMessageContentEvent contains a chunk of message content
type TextMessageContentEvent struct {
	BaseEvent
	// MessageID identifies the message
	MessageID string `json:"messageId"`
	// Delta is the content chunk
	Delta string `json:"delta"`
}

// TextMessageEndEvent signals the end of a text message
type TextMessageEndEvent struct {
	BaseEvent
	// MessageID identifies the message
	MessageID string `json:"messageId"`
}

// ToolCallStartEvent signals the start of a tool call
type ToolCallStartEvent struct {
	BaseEvent
	// ToolCallID identifies the tool call
	ToolCallID string `json:"toolCallId"`
	// ToolName is the name of the tool being called
	ToolName string `json:"toolName"`
	// ParentMessageID links to the parent message
	ParentMessageID string `json:"parentMessageId,omitempty"`
}

// ToolCallArgsEvent contains arguments for the tool call
type ToolCallArgsEvent struct {
	BaseEvent
	// ToolCallID identifies the tool call
	ToolCallID string `json:"toolCallId"`
	// Delta is the argument chunk (for streaming)
	Delta string `json:"delta"`
}

// ToolCallEndEvent signals the completion of a tool call
type ToolCallEndEvent struct {
	BaseEvent
	// ToolCallID identifies the tool call
	ToolCallID string `json:"toolCallId"`
	// Result is the tool call result
	Result string `json:"result,omitempty"`
}

// StateSnapshotEvent contains a complete state snapshot
type StateSnapshotEvent struct {
	BaseEvent
	// Snapshot is the complete state
	Snapshot map[string]any `json:"snapshot"`
}

// JSONPatchOperation represents a single JSON Patch operation
type JSONPatchOperation struct {
	// Op is the operation type (add, remove, replace, move, copy, test)
	Op string `json:"op"`
	// Path is the JSON pointer to the target location
	Path string `json:"path"`
	// Value is the value for add/replace operations
	Value any `json:"value,omitempty"`
	// From is the source path for move/copy operations
	From string `json:"from,omitempty"`
}

// StateDeltaEvent contains a state delta (JSON Patch)
type StateDeltaEvent struct {
	BaseEvent
	// Delta is the JSON Patch operations
	Delta []JSONPatchOperation `json:"delta"`
}

// CustomEvent contains custom event data
type CustomEvent struct {
	BaseEvent
	// Name of the custom event
	Name string `json:"name"`
	// Value of the custom event
	Value any `json:"value"`
}

// Event is the union type for all AG-UI events
type Event struct {
	// Embed base event for common fields
	BaseEvent

	// Event-specific data (only one will be set)
	RunStarted         *RunStartedEvent         `json:"-"`
	RunFinished        *RunFinishedEvent        `json:"-"`
	RunError           *RunErrorEvent           `json:"-"`
	StepStarted        *StepStartedEvent        `json:"-"`
	StepFinished       *StepFinishedEvent       `json:"-"`
	TextMessageStart   *TextMessageStartEvent   `json:"-"`
	TextMessageContent *TextMessageContentEvent `json:"-"`
	TextMessageEnd     *TextMessageEndEvent     `json:"-"`
	ToolCallStart      *ToolCallStartEvent      `json:"-"`
	ToolCallArgs       *ToolCallArgsEvent       `json:"-"`
	ToolCallEnd        *ToolCallEndEvent        `json:"-"`
	StateSnapshot      *StateSnapshotEvent      `json:"-"`
	StateDelta         *StateDeltaEvent         `json:"-"`
	Custom             *CustomEvent             `json:"-"`
}

// MarshalJSON implements custom JSON marshaling
func (e *Event) MarshalJSON() ([]byte, error) {
	switch e.Type {
	case EventRunStarted:
		return json.Marshal(e.RunStarted)
	case EventRunFinished:
		return json.Marshal(e.RunFinished)
	case EventRunError:
		return json.Marshal(e.RunError)
	case EventStepStarted:
		return json.Marshal(e.StepStarted)
	case EventStepFinished:
		return json.Marshal(e.StepFinished)
	case EventTextMessageStart:
		return json.Marshal(e.TextMessageStart)
	case EventTextMessageContent:
		return json.Marshal(e.TextMessageContent)
	case EventTextMessageEnd:
		return json.Marshal(e.TextMessageEnd)
	case EventToolCallStart:
		return json.Marshal(e.ToolCallStart)
	case EventToolCallArgs:
		return json.Marshal(e.ToolCallArgs)
	case EventToolCallEnd:
		return json.Marshal(e.ToolCallEnd)
	case EventStateSnapshot:
		return json.Marshal(e.StateSnapshot)
	case EventStateDelta:
		return json.Marshal(e.StateDelta)
	case EventCustom:
		return json.Marshal(e.Custom)
	default:
		return json.Marshal(e.BaseEvent)
	}
}

// UnmarshalJSON implements custom JSON unmarshaling
func (e *Event) UnmarshalJSON(data []byte) error {
	// First unmarshal to get the type
	var base BaseEvent
	if err := json.Unmarshal(data, &base); err != nil {
		return err
	}
	e.BaseEvent = base

	// Then unmarshal the specific event type
	switch base.Type {
	case EventRunStarted:
		e.RunStarted = &RunStartedEvent{}
		return json.Unmarshal(data, e.RunStarted)
	case EventRunFinished:
		e.RunFinished = &RunFinishedEvent{}
		return json.Unmarshal(data, e.RunFinished)
	case EventRunError:
		e.RunError = &RunErrorEvent{}
		return json.Unmarshal(data, e.RunError)
	case EventStepStarted:
		e.StepStarted = &StepStartedEvent{}
		return json.Unmarshal(data, e.StepStarted)
	case EventStepFinished:
		e.StepFinished = &StepFinishedEvent{}
		return json.Unmarshal(data, e.StepFinished)
	case EventTextMessageStart:
		e.TextMessageStart = &TextMessageStartEvent{}
		return json.Unmarshal(data, e.TextMessageStart)
	case EventTextMessageContent:
		e.TextMessageContent = &TextMessageContentEvent{}
		return json.Unmarshal(data, e.TextMessageContent)
	case EventTextMessageEnd:
		e.TextMessageEnd = &TextMessageEndEvent{}
		return json.Unmarshal(data, e.TextMessageEnd)
	case EventToolCallStart:
		e.ToolCallStart = &ToolCallStartEvent{}
		return json.Unmarshal(data, e.ToolCallStart)
	case EventToolCallArgs:
		e.ToolCallArgs = &ToolCallArgsEvent{}
		return json.Unmarshal(data, e.ToolCallArgs)
	case EventToolCallEnd:
		e.ToolCallEnd = &ToolCallEndEvent{}
		return json.Unmarshal(data, e.ToolCallEnd)
	case EventStateSnapshot:
		e.StateSnapshot = &StateSnapshotEvent{}
		return json.Unmarshal(data, e.StateSnapshot)
	case EventStateDelta:
		e.StateDelta = &StateDeltaEvent{}
		return json.Unmarshal(data, e.StateDelta)
	case EventCustom:
		e.Custom = &CustomEvent{}
		return json.Unmarshal(data, e.Custom)
	}
	return nil
}

// Message represents a chat message
type Message struct {
	// ID uniquely identifies the message
	ID string `json:"id"`
	// Role of the message sender
	Role MessageRole `json:"role"`
	// Content of the message
	Content string `json:"content"`
	// Name is optional sender name
	Name string `json:"name,omitempty"`
	// ToolCalls for assistant messages
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	// ToolCallID for tool response messages
	ToolCallID string `json:"toolCallId,omitempty"`
	// CreatedAt timestamp
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

// ToolCall represents a tool call in a message
type ToolCall struct {
	// ID uniquely identifies the tool call
	ID string `json:"id"`
	// Name of the tool
	Name string `json:"name"`
	// Arguments as JSON string
	Arguments string `json:"arguments"`
}

// Tool represents an available tool
type Tool struct {
	// Name of the tool
	Name string `json:"name"`
	// Description of what the tool does
	Description string `json:"description"`
	// Parameters schema (JSON Schema)
	Parameters map[string]any `json:"parameters"`
}

// RunRequest is sent to start an agent run
type RunRequest struct {
	// ThreadID identifies the conversation thread
	ThreadID string `json:"threadId,omitempty"`
	// RunID for this specific run (generated if not provided)
	RunID string `json:"runId,omitempty"`
	// Messages in the conversation
	Messages []Message `json:"messages"`
	// Tools available to the agent
	Tools []Tool `json:"tools,omitempty"`
	// State to pass to the agent
	State map[string]any `json:"state,omitempty"`
	// ForwardedProps are additional properties
	ForwardedProps map[string]any `json:"forwardedProps,omitempty"`
	// Config for the run
	Config *RunConfig `json:"config,omitempty"`
}

// RunConfig contains configuration for a run
type RunConfig struct {
	// Model to use
	Model string `json:"model,omitempty"`
	// Temperature for generation
	Temperature float64 `json:"temperature,omitempty"`
	// MaxTokens to generate
	MaxTokens int `json:"maxTokens,omitempty"`
	// Custom config options
	Custom map[string]any `json:"custom,omitempty"`
}

// RunResponse contains the final state after a run
type RunResponse struct {
	// ThreadID identifies the conversation thread
	ThreadID string `json:"threadId,omitempty"`
	// RunID for this specific run
	RunID string `json:"runId"`
	// Messages including the assistant's response
	Messages []Message `json:"messages"`
	// State after the run
	State map[string]any `json:"state,omitempty"`
}
