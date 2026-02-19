package memory

import (
	"context"
	"testing"
	"time"
)

// MockRedisClientForHistory implements RedisListClient for testing
type MockRedisClientForHistory struct {
	data map[string][]string
	ttls map[string]time.Duration
}

func NewMockRedisClientForHistory() *MockRedisClientForHistory {
	return &MockRedisClientForHistory{
		data: make(map[string][]string),
		ttls: make(map[string]time.Duration),
	}
}

func (m *MockRedisClientForHistory) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (m *MockRedisClientForHistory) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	return nil
}

func (m *MockRedisClientForHistory) Del(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func (m *MockRedisClientForHistory) LPush(ctx context.Context, key string, values ...string) error {
	for _, v := range values {
		m.data[key] = append([]string{v}, m.data[key]...)
	}
	return nil
}

func (m *MockRedisClientForHistory) RPush(ctx context.Context, key string, values ...string) error {
	m.data[key] = append(m.data[key], values...)
	return nil
}

func (m *MockRedisClientForHistory) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	list := m.data[key]
	if list == nil {
		return []string{}, nil
	}

	length := int64(len(list))
	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop {
		return []string{}, nil
	}

	return list[start : stop+1], nil
}

func (m *MockRedisClientForHistory) LLen(ctx context.Context, key string) (int64, error) {
	return int64(len(m.data[key])), nil
}

func (m *MockRedisClientForHistory) LTrim(ctx context.Context, key string, start, stop int64) error {
	list := m.data[key]
	if list == nil {
		return nil
	}

	length := int64(len(list))
	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop {
		m.data[key] = []string{}
		return nil
	}

	m.data[key] = list[start : stop+1]
	return nil
}

func (m *MockRedisClientForHistory) Expire(ctx context.Context, key string, expiration time.Duration) error {
	m.ttls[key] = expiration
	return nil
}

func (m *MockRedisClientForHistory) Close() error {
	return nil
}

func TestRedisChatMessageHistory(t *testing.T) {
	ctx := context.Background()
	client := NewMockRedisClientForHistory()

	history, err := NewRedisChatMessageHistory(RedisChatMessageHistoryConfig{
		Client:    client,
		SessionID: "test-session",
		TTL:       time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}

	// Test adding messages
	err = history.AddUserMessage(ctx, "Hello")
	if err != nil {
		t.Fatalf("Failed to add user message: %v", err)
	}

	err = history.AddAIMessage(ctx, "Hi there!")
	if err != nil {
		t.Fatalf("Failed to add AI message: %v", err)
	}

	// Test retrieving messages
	messages, err := history.Messages(ctx)
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	if messages[0].Role != RoleHuman {
		t.Errorf("Expected first message role to be human, got %s", messages[0].Role)
	}

	if messages[1].Role != RoleAI {
		t.Errorf("Expected second message role to be AI, got %s", messages[1].Role)
	}

	// Test length
	length, err := history.Len(ctx)
	if err != nil {
		t.Fatalf("Failed to get length: %v", err)
	}
	if length != 2 {
		t.Errorf("Expected length 2, got %d", length)
	}

	// Test clear
	err = history.Clear(ctx)
	if err != nil {
		t.Fatalf("Failed to clear history: %v", err)
	}

	messages, _ = history.Messages(ctx)
	if len(messages) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(messages))
	}
}

func TestRedisChatMessageHistoryMaxLength(t *testing.T) {
	ctx := context.Background()
	client := NewMockRedisClientForHistory()

	history, err := NewRedisChatMessageHistory(RedisChatMessageHistoryConfig{
		Client:    client,
		SessionID: "test-session-max",
		MaxLength: 3,
	})
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}

	// Add 5 messages
	for i := 0; i < 5; i++ {
		history.AddUserMessage(ctx, "Message "+string(rune('A'+i)))
	}

	// Should only have last 3
	messages, _ := history.Messages(ctx)
	if len(messages) != 3 {
		t.Errorf("Expected 3 messages (max length), got %d", len(messages))
	}
}

func TestRedisConversationBufferMemory(t *testing.T) {
	ctx := context.Background()
	client := NewMockRedisClientForHistory()

	mem, err := NewRedisConversationBufferMemory(RedisConversationBufferMemoryConfig{
		RedisChatMessageHistoryConfig: RedisChatMessageHistoryConfig{
			Client:    client,
			SessionID: "test-buffer",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create memory: %v", err)
	}

	// Save context
	err = mem.SaveContext(ctx,
		map[string]any{"input": "What is 2+2?"},
		map[string]any{"output": "4"},
	)
	if err != nil {
		t.Fatalf("Failed to save context: %v", err)
	}

	// Load memory variables
	vars, err := mem.LoadMemoryVariables(ctx)
	if err != nil {
		t.Fatalf("Failed to load memory variables: %v", err)
	}

	history, ok := vars["history"].(string)
	if !ok {
		t.Fatalf("Expected history to be string")
	}

	if history == "" {
		t.Error("Expected non-empty history")
	}

	// Check memory variables
	keys := mem.MemoryVariables()
	if len(keys) != 1 || keys[0] != "history" {
		t.Errorf("Unexpected memory variables: %v", keys)
	}
}

func TestRedisChatMessageHistoryValidation(t *testing.T) {
	// Test missing client
	_, err := NewRedisChatMessageHistory(RedisChatMessageHistoryConfig{
		SessionID: "test",
	})
	if err == nil {
		t.Error("Expected error for missing client")
	}

	// Test missing session ID
	_, err = NewRedisChatMessageHistory(RedisChatMessageHistoryConfig{
		Client: NewMockRedisClientForHistory(),
	})
	if err == nil {
		t.Error("Expected error for missing session ID")
	}
}
