package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewTask(t *testing.T) {
	task := NewTask("test-123")

	if task.ID != "test-123" {
		t.Errorf("expected ID 'test-123', got '%s'", task.ID)
	}

	if task.Status.State != TaskStateSubmitted {
		t.Errorf("expected state 'submitted', got '%s'", task.Status.State)
	}

	if len(task.History) != 0 {
		t.Errorf("expected empty history, got %d items", len(task.History))
	}
}

func TestTaskUpdateState(t *testing.T) {
	task := NewTask("test-123")

	msg := &Message{
		Role:  MessageRoleAgent,
		Parts: []Part{NewTextPart("Working on it...")},
	}

	task.UpdateState(TaskStateWorking, msg)

	if task.Status.State != TaskStateWorking {
		t.Errorf("expected state 'working', got '%s'", task.Status.State)
	}

	if task.Status.Message == nil {
		t.Error("expected status message to be set")
	}
}

func TestTaskAddMessage(t *testing.T) {
	task := NewTask("test-123")

	msg := NewTextMessage(MessageRoleUser, "Hello, agent!")
	task.AddMessage(msg)

	if len(task.History) != 1 {
		t.Errorf("expected 1 message in history, got %d", len(task.History))
	}

	if task.History[0].GetText() != "Hello, agent!" {
		t.Errorf("expected message text 'Hello, agent!', got '%s'", task.History[0].GetText())
	}
}

func TestAgentCardBuilder(t *testing.T) {
	card := NewAgentCardBuilder("Test Agent", "A test agent", "http://localhost:8080").
		WithVersion("1.0.0").
		WithStreaming(true).
		WithPushNotifications(false).
		WithProvider("Test Org", "http://test.org", "test@test.org").
		AddSkill(AgentSkill{
			ID:          "skill-1",
			Name:        "Test Skill",
			Description: "A test skill",
		}).
		Build()

	if card.Name != "Test Agent" {
		t.Errorf("expected name 'Test Agent', got '%s'", card.Name)
	}

	if card.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", card.Version)
	}

	if !card.Capabilities.Streaming {
		t.Error("expected streaming to be enabled")
	}

	if card.Capabilities.PushNotifications {
		t.Error("expected push notifications to be disabled")
	}

	if len(card.Skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(card.Skills))
	}
}

func TestAgentCardValidation(t *testing.T) {
	tests := []struct {
		name    string
		card    AgentCard
		wantErr bool
	}{
		{
			name: "valid card",
			card: AgentCard{
				Name:        "Test",
				URL:         "http://localhost",
				Description: "Test agent",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			card: AgentCard{
				URL:         "http://localhost",
				Description: "Test agent",
			},
			wantErr: true,
		},
		{
			name: "missing URL",
			card: AgentCard{
				Name:        "Test",
				Description: "Test agent",
			},
			wantErr: true,
		},
		{
			name: "missing description",
			card: AgentCard{
				Name: "Test",
				URL:  "http://localhost",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.card.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJSONRPCRequestResponse(t *testing.T) {
	// Test request creation
	params := TaskSendParams{
		Message: NewTextMessage(MessageRoleUser, "Hello"),
	}

	req, err := NewJSONRPCRequest(1, MethodTaskSend, params)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc '2.0', got '%s'", req.JSONRPC)
	}

	if req.Method != MethodTaskSend {
		t.Errorf("expected method '%s', got '%s'", MethodTaskSend, req.Method)
	}

	// Test response creation
	task := NewTask("test-123")
	resp := NewJSONRPCResponse(1, task)

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc '2.0', got '%s'", resp.JSONRPC)
	}

	if resp.Error != nil {
		t.Error("expected no error in response")
	}

	// Test error response
	errResp := NewJSONRPCError(1, ErrorCodeTaskNotFound, "Task not found", nil)

	if errResp.Error == nil {
		t.Error("expected error in response")
	}

	if errResp.Error.Code != ErrorCodeTaskNotFound {
		t.Errorf("expected error code %d, got %d", ErrorCodeTaskNotFound, errResp.Error.Code)
	}
}

// Mock task handler for testing
type mockTaskHandler struct {
	response string
}

func (h *mockTaskHandler) HandleTask(ctx context.Context, task *Task) error {
	msg := NewTextMessage(MessageRoleAgent, h.response)
	task.AddMessage(msg)
	task.UpdateState(TaskStateCompleted, &msg)
	return nil
}

func (h *mockTaskHandler) HandleTaskStream(ctx context.Context, task *Task, updates chan<- TaskUpdate) error {
	updates <- NewStatusUpdate(TaskStateWorking, nil)

	msg := NewTextMessage(MessageRoleAgent, h.response)
	task.AddMessage(msg)
	updates <- NewMessageUpdate(&msg)

	updates <- NewStatusUpdate(TaskStateCompleted, &msg)
	return nil
}

func (h *mockTaskHandler) SupportsStreaming() bool {
	return true
}

func TestTaskManager(t *testing.T) {
	handler := &mockTaskHandler{response: "Hello from agent!"}
	config := DefaultTaskManagerConfig()
	manager := NewTaskManager(handler, config)

	// Create task
	params := TaskSendParams{
		Message: NewTextMessage(MessageRoleUser, "Hello"),
	}

	task, err := manager.CreateTask(params)
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	if task.Status.State != TaskStateSubmitted {
		t.Errorf("expected state 'submitted', got '%s'", task.Status.State)
	}

	// Process task
	ctx := context.Background()
	err = manager.ProcessTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("failed to process task: %v", err)
	}

	// Get updated task
	task, err = manager.GetTask(task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if task.Status.State != TaskStateCompleted {
		t.Errorf("expected state 'completed', got '%s'", task.Status.State)
	}

	if len(task.History) != 2 {
		t.Errorf("expected 2 messages in history, got %d", len(task.History))
	}
}

func TestServerAgentCard(t *testing.T) {
	handler := &mockTaskHandler{response: "Hello!"}
	config := DefaultTaskManagerConfig()
	manager := NewTaskManager(handler, config)

	card := NewAgentCardBuilder("Test Agent", "Test description", "http://localhost:8080").
		WithStreaming(true).
		Build()

	serverConfig := DefaultServerConfig()
	server := NewServer(&card, manager, serverConfig)

	// Test agent card endpoint
	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent.json", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var respCard AgentCard
	if err := json.NewDecoder(w.Body).Decode(&respCard); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if respCard.Name != "Test Agent" {
		t.Errorf("expected name 'Test Agent', got '%s'", respCard.Name)
	}
}

func TestServerTaskSend(t *testing.T) {
	handler := &mockTaskHandler{response: "Hello from agent!"}
	config := DefaultTaskManagerConfig()
	manager := NewTaskManager(handler, config)

	card := NewAgentCardBuilder("Test Agent", "Test description", "http://localhost:8080").Build()
	serverConfig := DefaultServerConfig()
	server := NewServer(&card, manager, serverConfig)

	// Create JSON-RPC request
	params := TaskSendParams{
		Message: NewTextMessage(MessageRoleUser, "Hello"),
	}
	rpcReq, _ := NewJSONRPCRequest(1, MethodTaskSend, params)
	body, _ := json.Marshal(rpcReq)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}

func TestNewTextPart(t *testing.T) {
	part := NewTextPart("Hello, world!")

	if part.Type != PartTypeText {
		t.Errorf("expected type 'text', got '%s'", part.Type)
	}

	if part.Text != "Hello, world!" {
		t.Errorf("expected text 'Hello, world!', got '%s'", part.Text)
	}
}

func TestNewFilePart(t *testing.T) {
	part := NewFilePart("test.txt", "text/plain", "file:///test.txt")

	if part.Type != PartTypeFile {
		t.Errorf("expected type 'file', got '%s'", part.Type)
	}

	if part.Name != "test.txt" {
		t.Errorf("expected name 'test.txt', got '%s'", part.Name)
	}

	if part.MimeType != "text/plain" {
		t.Errorf("expected mime type 'text/plain', got '%s'", part.MimeType)
	}
}

func TestTaskStatus(t *testing.T) {
	status := TaskStatus{
		State:     TaskStateWorking,
		Timestamp: time.Now(),
	}

	if status.State != TaskStateWorking {
		t.Errorf("expected state 'working', got '%s'", status.State)
	}
}

func TestAgentCardRegistry(t *testing.T) {
	registry := NewAgentCardRegistry()

	card := &AgentCard{
		Name:        "Test Agent",
		URL:         "http://localhost:8080",
		Description: "Test agent",
	}

	// Register
	err := registry.Register(card)
	if err != nil {
		t.Fatalf("failed to register card: %v", err)
	}

	// Get
	retrieved, ok := registry.Get("http://localhost:8080")
	if !ok {
		t.Error("expected to find registered card")
	}

	if retrieved.Name != "Test Agent" {
		t.Errorf("expected name 'Test Agent', got '%s'", retrieved.Name)
	}

	// List
	cards := registry.List()
	if len(cards) != 1 {
		t.Errorf("expected 1 card, got %d", len(cards))
	}

	// Remove
	registry.Remove("http://localhost:8080")
	_, ok = registry.Get("http://localhost:8080")
	if ok {
		t.Error("expected card to be removed")
	}
}
