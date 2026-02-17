package agui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEventEmitter(t *testing.T) {
	emitter := NewEventEmitter("thread-1", "run-1", 10)
	defer emitter.Close()

	// Test run started
	emitter.EmitRunStarted()
	event := <-emitter.Events()
	if event.Type != EventRunStarted {
		t.Errorf("expected event type RUN_STARTED, got %s", event.Type)
	}

	// Test text message
	msgID := emitter.EmitTextMessageStart("", RoleAssistant)
	event = <-emitter.Events()
	if event.Type != EventTextMessageStart {
		t.Errorf("expected event type TEXT_MESSAGE_START, got %s", event.Type)
	}

	emitter.EmitTextMessageContent(msgID, "Hello ")
	event = <-emitter.Events()
	if event.Type != EventTextMessageContent {
		t.Errorf("expected event type TEXT_MESSAGE_CONTENT, got %s", event.Type)
	}

	emitter.EmitTextMessageEnd(msgID)
	event = <-emitter.Events()
	if event.Type != EventTextMessageEnd {
		t.Errorf("expected event type TEXT_MESSAGE_END, got %s", event.Type)
	}

	// Test run finished
	emitter.EmitRunFinished()
	event = <-emitter.Events()
	if event.Type != EventRunFinished {
		t.Errorf("expected event type RUN_FINISHED, got %s", event.Type)
	}
}

func TestStreamingMessage(t *testing.T) {
	emitter := NewEventEmitter("thread-1", "run-1", 10)
	defer emitter.Close()

	msg := emitter.NewStreamingMessage(RoleAssistant)

	// Consume start event
	<-emitter.Events()

	msg.WriteString("Hello ")
	<-emitter.Events()

	msg.WriteString("World!")
	<-emitter.Events()

	msg.End()
	<-emitter.Events()

	if msg.Content() != "Hello World!" {
		t.Errorf("expected content 'Hello World!', got '%s'", msg.Content())
	}

	if msg.MessageID() == "" {
		t.Error("expected message ID to be set")
	}
}

func TestStreamingToolCall(t *testing.T) {
	emitter := NewEventEmitter("thread-1", "run-1", 10)
	defer emitter.Close()

	toolCall := emitter.NewStreamingToolCall("test_tool", "msg-1")

	// Consume start event
	<-emitter.Events()

	toolCall.WriteArgs(`{"param": "value"}`)
	<-emitter.Events()

	toolCall.End("result data")
	<-emitter.Events()

	if toolCall.Args() != `{"param": "value"}` {
		t.Errorf("expected args, got '%s'", toolCall.Args())
	}

	if toolCall.ToolCallID() == "" {
		t.Error("expected tool call ID to be set")
	}
}

func TestStateManager(t *testing.T) {
	emitter := NewEventEmitter("thread-1", "run-1", 100)
	defer emitter.Close()

	sm := NewStateManager(emitter)

	// Test Set
	err := sm.Set("user.name", "Alice")
	if err != nil {
		t.Fatalf("failed to set state: %v", err)
	}

	// Consume state delta event
	<-emitter.Events()

	// Test Get
	value, ok := sm.Get("user.name")
	if !ok {
		t.Error("expected to find value")
	}
	if value != "Alice" {
		t.Errorf("expected 'Alice', got '%v'", value)
	}

	// Test nested set
	err = sm.Set("user.age", 30)
	if err != nil {
		t.Fatalf("failed to set nested state: %v", err)
	}
	<-emitter.Events()

	// Test GetAll
	all := sm.GetAll()
	user, ok := all["user"].(map[string]any)
	if !ok {
		t.Fatal("expected user to be a map")
	}
	if user["name"] != "Alice" {
		t.Errorf("expected name 'Alice', got '%v'", user["name"])
	}

	// Test Remove
	err = sm.Remove("user.age")
	if err != nil {
		t.Fatalf("failed to remove state: %v", err)
	}
	<-emitter.Events()

	_, ok = sm.Get("user.age")
	if ok {
		t.Error("expected value to be removed")
	}
}

func TestStateManagerPatch(t *testing.T) {
	emitter := NewEventEmitter("thread-1", "run-1", 100)
	defer emitter.Close()

	initial := map[string]any{
		"counter": 0,
		"items":   []any{"a", "b"},
	}
	sm := NewStateManagerWithInitial(emitter, initial)

	// Apply patch
	ops := []JSONPatchOperation{
		{Op: "replace", Path: "/counter", Value: 1},
		{Op: "add", Path: "/newKey", Value: "newValue"},
	}

	err := sm.Patch(ops)
	if err != nil {
		t.Fatalf("failed to apply patch: %v", err)
	}

	// Consume delta event
	<-emitter.Events()

	// Verify
	counter, _ := sm.Get("counter")
	// After JSON roundtrip through deepCopy, numbers become float64
	// But direct patch application keeps the original type
	if counter != 1 && counter != float64(1) {
		t.Errorf("expected counter 1, got %v (type: %T)", counter, counter)
	}

	newKey, ok := sm.Get("newKey")
	if !ok || newKey != "newValue" {
		t.Errorf("expected newKey 'newValue', got %v", newKey)
	}
}

func TestStateDiff(t *testing.T) {
	old := map[string]any{
		"name":   "Alice",
		"age":    30,
		"active": true,
	}

	new := map[string]any{
		"name":   "Alice",
		"age":    31,
		"status": "online",
	}

	ops := Diff(old, new)

	// Should have: replace age, add status, remove active
	if len(ops) != 3 {
		t.Errorf("expected 3 operations, got %d", len(ops))
	}

	// Check for expected operations
	hasReplace := false
	hasAdd := false
	hasRemove := false

	for _, op := range ops {
		switch op.Op {
		case "replace":
			if op.Path == "/age" {
				hasReplace = true
			}
		case "add":
			if op.Path == "/status" {
				hasAdd = true
			}
		case "remove":
			if op.Path == "/active" {
				hasRemove = true
			}
		}
	}

	if !hasReplace {
		t.Error("expected replace operation for age")
	}
	if !hasAdd {
		t.Error("expected add operation for status")
	}
	if !hasRemove {
		t.Error("expected remove operation for active")
	}
}

func TestJSONPointerConversion(t *testing.T) {
	tests := []struct {
		dotPath     string
		jsonPointer string
	}{
		{"name", "/name"},
		{"user.name", "/user/name"},
		{"items.0.value", "/items/0/value"},
	}

	for _, tt := range tests {
		result := toJSONPointer(tt.dotPath)
		if result != tt.jsonPointer {
			t.Errorf("toJSONPointer(%s) = %s, want %s", tt.dotPath, result, tt.jsonPointer)
		}

		back := fromJSONPointer(tt.jsonPointer)
		if back != tt.dotPath {
			t.Errorf("fromJSONPointer(%s) = %s, want %s", tt.jsonPointer, back, tt.dotPath)
		}
	}
}

func TestEventMarshaling(t *testing.T) {
	// Test marshaling the specific event type directly
	textEvent := TextMessageContentEvent{
		BaseEvent: BaseEvent{Type: EventTextMessageContent, Timestamp: time.Now()},
		MessageID: "msg-1",
		Delta:     "Hello",
	}

	data, err := json.Marshal(textEvent)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var unmarshaled TextMessageContentEvent
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if unmarshaled.Type != EventTextMessageContent {
		t.Errorf("expected type TEXT_MESSAGE_CONTENT, got %s", unmarshaled.Type)
	}

	if unmarshaled.Delta != "Hello" {
		t.Errorf("expected delta 'Hello', got '%s'", unmarshaled.Delta)
	}

	if unmarshaled.MessageID != "msg-1" {
		t.Errorf("expected message ID 'msg-1', got '%s'", unmarshaled.MessageID)
	}
}

// Mock handler for testing
type mockHandler struct {
	response string
}

func (h *mockHandler) HandleRun(ctx context.Context, req RunRequest, emitter *EventEmitter) error {
	emitter.EmitRunStarted()

	msgID := emitter.EmitTextMessageStart("", RoleAssistant)
	emitter.EmitTextMessageContent(msgID, h.response)
	emitter.EmitTextMessageEnd(msgID)

	emitter.EmitRunFinished()
	return nil
}

func TestServerRun(t *testing.T) {
	handler := &mockHandler{response: "Hello from AG-UI!"}
	config := DefaultServerConfig()
	server := NewServer(handler, config)

	// Create run request
	runReq := RunRequest{
		Messages: []Message{
			{ID: "1", Role: RoleUser, Content: "Hello"},
		},
	}
	body, _ := json.Marshal(runReq)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	// Use a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Run in goroutine since it's SSE
	done := make(chan bool)
	go func() {
		server.Handler().ServeHTTP(w, req)
		done <- true
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		cancel()
		<-done
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("expected content type 'text/event-stream', got '%s'", contentType)
	}
}

func TestRunRequest(t *testing.T) {
	req := RunRequest{
		ThreadID: "thread-1",
		RunID:    "run-1",
		Messages: []Message{
			{ID: "1", Role: RoleUser, Content: "Hello"},
		},
		Tools: []Tool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		State: map[string]any{
			"key": "value",
		},
	}

	if len(req.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(req.Messages))
	}

	if len(req.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(req.Tools))
	}
}

func TestMessage(t *testing.T) {
	msg := Message{
		ID:      "msg-1",
		Role:    RoleAssistant,
		Content: "Hello, world!",
		ToolCalls: []ToolCall{
			{ID: "tc-1", Name: "test", Arguments: `{"key": "value"}`},
		},
	}

	if msg.Role != RoleAssistant {
		t.Errorf("expected role 'assistant', got '%s'", msg.Role)
	}

	if len(msg.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
}

func TestCollectMessages(t *testing.T) {
	events := []Event{
		{
			BaseEvent: BaseEvent{Type: EventTextMessageStart},
			TextMessageStart: &TextMessageStartEvent{
				MessageID: "msg-1",
				Role:      RoleAssistant,
			},
		},
		{
			BaseEvent: BaseEvent{Type: EventTextMessageContent},
			TextMessageContent: &TextMessageContentEvent{
				MessageID: "msg-1",
				Delta:     "Hello ",
			},
		},
		{
			BaseEvent: BaseEvent{Type: EventTextMessageContent},
			TextMessageContent: &TextMessageContentEvent{
				MessageID: "msg-1",
				Delta:     "World!",
			},
		},
		{
			BaseEvent: BaseEvent{Type: EventTextMessageEnd},
			TextMessageEnd: &TextMessageEndEvent{
				MessageID: "msg-1",
			},
		},
	}

	messages := CollectMessages(events)

	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Content != "Hello World!" {
		t.Errorf("expected content 'Hello World!', got '%s'", messages[0].Content)
	}
}

func TestServerStats(t *testing.T) {
	handler := &mockHandler{response: "Hello!"}
	config := DefaultServerConfig()
	server := NewServer(handler, config)

	stats := server.Stats()
	if stats.ActiveRuns != 0 {
		t.Errorf("expected 0 active runs, got %d", stats.ActiveRuns)
	}
}
