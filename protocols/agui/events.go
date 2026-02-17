package agui

import (
	"time"

	"github.com/google/uuid"
)

// EventEmitter provides methods for emitting AG-UI events
type EventEmitter struct {
	events   chan Event
	threadID string
	runID    string
}

// NewEventEmitter creates a new event emitter
func NewEventEmitter(threadID, runID string, bufferSize int) *EventEmitter {
	if runID == "" {
		runID = uuid.New().String()
	}
	return &EventEmitter{
		events:   make(chan Event, bufferSize),
		threadID: threadID,
		runID:    runID,
	}
}

// Events returns the event channel
func (e *EventEmitter) Events() <-chan Event {
	return e.events
}

// Close closes the event channel
func (e *EventEmitter) Close() {
	close(e.events)
}

// emit sends an event
func (e *EventEmitter) emit(event Event) {
	event.Timestamp = time.Now()
	select {
	case e.events <- event:
	default:
		// Channel full, drop event (or we could block)
	}
}

// EmitRunStarted emits a run started event
func (e *EventEmitter) EmitRunStarted() {
	e.emit(Event{
		BaseEvent: BaseEvent{Type: EventRunStarted},
		RunStarted: &RunStartedEvent{
			BaseEvent: BaseEvent{Type: EventRunStarted, Timestamp: time.Now()},
			ThreadID:  e.threadID,
			RunID:     e.runID,
		},
	})
}

// EmitRunFinished emits a run finished event
func (e *EventEmitter) EmitRunFinished() {
	e.emit(Event{
		BaseEvent: BaseEvent{Type: EventRunFinished},
		RunFinished: &RunFinishedEvent{
			BaseEvent: BaseEvent{Type: EventRunFinished, Timestamp: time.Now()},
			ThreadID:  e.threadID,
			RunID:     e.runID,
		},
	})
}

// EmitRunError emits a run error event
func (e *EventEmitter) EmitRunError(message string, code string) {
	e.emit(Event{
		BaseEvent: BaseEvent{Type: EventRunError},
		RunError: &RunErrorEvent{
			BaseEvent: BaseEvent{Type: EventRunError, Timestamp: time.Now()},
			Message:   message,
			Code:      code,
		},
	})
}

// EmitStepStarted emits a step started event
func (e *EventEmitter) EmitStepStarted(stepName string) {
	e.emit(Event{
		BaseEvent: BaseEvent{Type: EventStepStarted},
		StepStarted: &StepStartedEvent{
			BaseEvent: BaseEvent{Type: EventStepStarted, Timestamp: time.Now()},
			StepName:  stepName,
		},
	})
}

// EmitStepFinished emits a step finished event
func (e *EventEmitter) EmitStepFinished(stepName string) {
	e.emit(Event{
		BaseEvent: BaseEvent{Type: EventStepFinished},
		StepFinished: &StepFinishedEvent{
			BaseEvent: BaseEvent{Type: EventStepFinished, Timestamp: time.Now()},
			StepName:  stepName,
		},
	})
}

// EmitTextMessageStart emits a text message start event
func (e *EventEmitter) EmitTextMessageStart(messageID string, role MessageRole) string {
	if messageID == "" {
		messageID = uuid.New().String()
	}
	e.emit(Event{
		BaseEvent: BaseEvent{Type: EventTextMessageStart},
		TextMessageStart: &TextMessageStartEvent{
			BaseEvent: BaseEvent{Type: EventTextMessageStart, Timestamp: time.Now()},
			MessageID: messageID,
			Role:      role,
		},
	})
	return messageID
}

// EmitTextMessageContent emits a text message content event
func (e *EventEmitter) EmitTextMessageContent(messageID string, delta string) {
	e.emit(Event{
		BaseEvent: BaseEvent{Type: EventTextMessageContent},
		TextMessageContent: &TextMessageContentEvent{
			BaseEvent: BaseEvent{Type: EventTextMessageContent, Timestamp: time.Now()},
			MessageID: messageID,
			Delta:     delta,
		},
	})
}

// EmitTextMessageEnd emits a text message end event
func (e *EventEmitter) EmitTextMessageEnd(messageID string) {
	e.emit(Event{
		BaseEvent: BaseEvent{Type: EventTextMessageEnd},
		TextMessageEnd: &TextMessageEndEvent{
			BaseEvent: BaseEvent{Type: EventTextMessageEnd, Timestamp: time.Now()},
			MessageID: messageID,
		},
	})
}

// EmitToolCallStart emits a tool call start event
func (e *EventEmitter) EmitToolCallStart(toolCallID, toolName, parentMessageID string) string {
	if toolCallID == "" {
		toolCallID = uuid.New().String()
	}
	e.emit(Event{
		BaseEvent: BaseEvent{Type: EventToolCallStart},
		ToolCallStart: &ToolCallStartEvent{
			BaseEvent:       BaseEvent{Type: EventToolCallStart, Timestamp: time.Now()},
			ToolCallID:      toolCallID,
			ToolName:        toolName,
			ParentMessageID: parentMessageID,
		},
	})
	return toolCallID
}

// EmitToolCallArgs emits a tool call args event
func (e *EventEmitter) EmitToolCallArgs(toolCallID string, delta string) {
	e.emit(Event{
		BaseEvent: BaseEvent{Type: EventToolCallArgs},
		ToolCallArgs: &ToolCallArgsEvent{
			BaseEvent:  BaseEvent{Type: EventToolCallArgs, Timestamp: time.Now()},
			ToolCallID: toolCallID,
			Delta:      delta,
		},
	})
}

// EmitToolCallEnd emits a tool call end event
func (e *EventEmitter) EmitToolCallEnd(toolCallID string, result string) {
	e.emit(Event{
		BaseEvent: BaseEvent{Type: EventToolCallEnd},
		ToolCallEnd: &ToolCallEndEvent{
			BaseEvent:  BaseEvent{Type: EventToolCallEnd, Timestamp: time.Now()},
			ToolCallID: toolCallID,
			Result:     result,
		},
	})
}

// EmitStateSnapshot emits a state snapshot event
func (e *EventEmitter) EmitStateSnapshot(snapshot map[string]any) {
	e.emit(Event{
		BaseEvent: BaseEvent{Type: EventStateSnapshot},
		StateSnapshot: &StateSnapshotEvent{
			BaseEvent: BaseEvent{Type: EventStateSnapshot, Timestamp: time.Now()},
			Snapshot:  snapshot,
		},
	})
}

// EmitStateDelta emits a state delta event
func (e *EventEmitter) EmitStateDelta(delta []JSONPatchOperation) {
	e.emit(Event{
		BaseEvent: BaseEvent{Type: EventStateDelta},
		StateDelta: &StateDeltaEvent{
			BaseEvent: BaseEvent{Type: EventStateDelta, Timestamp: time.Now()},
			Delta:     delta,
		},
	})
}

// EmitCustom emits a custom event
func (e *EventEmitter) EmitCustom(name string, value any) {
	e.emit(Event{
		BaseEvent: BaseEvent{Type: EventCustom},
		Custom: &CustomEvent{
			BaseEvent: BaseEvent{Type: EventCustom, Timestamp: time.Now()},
			Name:      name,
			Value:     value,
		},
	})
}

// StreamingMessage provides a builder for streaming text messages
type StreamingMessage struct {
	emitter   *EventEmitter
	messageID string
	role      MessageRole
	content   string
}

// NewStreamingMessage creates a new streaming message
func (e *EventEmitter) NewStreamingMessage(role MessageRole) *StreamingMessage {
	messageID := e.EmitTextMessageStart("", role)
	return &StreamingMessage{
		emitter:   e,
		messageID: messageID,
		role:      role,
	}
}

// Write implements io.Writer for streaming content
func (m *StreamingMessage) Write(p []byte) (n int, err error) {
	delta := string(p)
	m.content += delta
	m.emitter.EmitTextMessageContent(m.messageID, delta)
	return len(p), nil
}

// WriteString writes a string to the message
func (m *StreamingMessage) WriteString(s string) {
	m.content += s
	m.emitter.EmitTextMessageContent(m.messageID, s)
}

// End ends the streaming message
func (m *StreamingMessage) End() {
	m.emitter.EmitTextMessageEnd(m.messageID)
}

// Content returns the accumulated content
func (m *StreamingMessage) Content() string {
	return m.content
}

// MessageID returns the message ID
func (m *StreamingMessage) MessageID() string {
	return m.messageID
}

// StreamingToolCall provides a builder for streaming tool calls
type StreamingToolCall struct {
	emitter    *EventEmitter
	toolCallID string
	toolName   string
	args       string
}

// NewStreamingToolCall creates a new streaming tool call
func (e *EventEmitter) NewStreamingToolCall(toolName, parentMessageID string) *StreamingToolCall {
	toolCallID := e.EmitToolCallStart("", toolName, parentMessageID)
	return &StreamingToolCall{
		emitter:    e,
		toolCallID: toolCallID,
		toolName:   toolName,
	}
}

// WriteArgs writes argument data
func (t *StreamingToolCall) WriteArgs(delta string) {
	t.args += delta
	t.emitter.EmitToolCallArgs(t.toolCallID, delta)
}

// End ends the tool call with a result
func (t *StreamingToolCall) End(result string) {
	t.emitter.EmitToolCallEnd(t.toolCallID, result)
}

// ToolCallID returns the tool call ID
func (t *StreamingToolCall) ToolCallID() string {
	return t.toolCallID
}

// Args returns the accumulated arguments
func (t *StreamingToolCall) Args() string {
	return t.args
}
