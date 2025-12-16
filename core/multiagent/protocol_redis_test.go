package multiagent

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestRedisProtocol_Connection tests Redis connection
func TestRedisProtocol_Connection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultRedisConfig()
	config.Addr = "localhost:6379"

	protocol, err := NewRedisProtocol(config)
	if err != nil {
		t.Skip("Redis not available:", err)
	}
	defer protocol.Close()

	ctx := context.Background()
	if err := protocol.Health(ctx); err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
}

// TestRedisProtocol_SendReceive tests basic message sending and receiving
func TestRedisProtocol_SendReceive(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultRedisConfig()
	protocol, err := NewRedisProtocol(config)
	if err != nil {
		t.Skip("Redis not available:", err)
	}
	defer protocol.Close()

	ctx := context.Background()

	// Subscribe agent
	agentID := "test-agent-1"
	if err := protocol.Subscribe(ctx, agentID, []MessageType{MessageTypeTask}); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Send message
	msg := &Message{
		Type:    MessageTypeTask,
		From:    "sender",
		To:      agentID,
		Content: "Test message",
	}

	if err := protocol.Send(ctx, msg); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Give Redis time to process
	time.Sleep(100 * time.Millisecond)

	// Receive message
	messages, err := protocol.Receive(ctx, agentID)
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if len(messages) == 0 {
		t.Fatal("Expected to receive message, got none")
	}

	if messages[0].Content != msg.Content {
		t.Errorf("Expected content %v, got %v", msg.Content, messages[0].Content)
	}

	// Cleanup
	cleanup(t, protocol, agentID)
}

// TestRedisProtocol_MultipleMessages tests sending multiple messages
func TestRedisProtocol_MultipleMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultRedisConfig()
	protocol, err := NewRedisProtocol(config)
	if err != nil {
		t.Skip("Redis not available:", err)
	}
	defer protocol.Close()

	ctx := context.Background()

	agentID := "test-agent-multi"
	if err := protocol.Subscribe(ctx, agentID, []MessageType{MessageTypeTask}); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Send 5 messages
	for i := 0; i < 5; i++ {
		msg := &Message{
			Type:    MessageTypeTask,
			From:    "sender",
			To:      agentID,
			Content: i,
		}

		if err := protocol.Send(ctx, msg); err != nil {
			t.Fatalf("Send failed: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	// Receive messages
	messages, err := protocol.Receive(ctx, agentID)
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if len(messages) != 5 {
		t.Errorf("Expected 5 messages, got %d", len(messages))
	}

	cleanup(t, protocol, agentID)
}

// TestRedisProtocol_Subscribe tests message type filtering
func TestRedisProtocol_Subscribe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultRedisConfig()
	protocol, err := NewRedisProtocol(config)
	if err != nil {
		t.Skip("Redis not available:", err)
	}
	defer protocol.Close()

	ctx := context.Background()

	agentID := "test-agent-filter"

	// Subscribe only to Task messages
	if err := protocol.Subscribe(ctx, agentID, []MessageType{MessageTypeTask}); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Send Task message
	taskMsg := &Message{
		Type:    MessageTypeTask,
		From:    "sender",
		To:      agentID,
		Content: "task",
	}
	if err := protocol.Send(ctx, taskMsg); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Send Query message (should be filtered)
	queryMsg := &Message{
		Type:    MessageTypeQuery,
		From:    "sender",
		To:      agentID,
		Content: "query",
	}
	if err := protocol.Send(ctx, queryMsg); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Receive - should only get Task message
	messages, err := protocol.Receive(ctx, agentID)
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if len(messages) != 1 {
		t.Errorf("Expected 1 message after filtering, got %d", len(messages))
	}

	if len(messages) > 0 && messages[0].Type != MessageTypeTask {
		t.Errorf("Expected Task message, got %v", messages[0].Type)
	}

	cleanup(t, protocol, agentID)
}

// TestRedisProtocol_Metrics tests metrics collection
func TestRedisProtocol_Metrics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultRedisConfig()
	protocol, err := NewRedisProtocol(config)
	if err != nil {
		t.Skip("Redis not available:", err)
	}
	defer protocol.Close()

	ctx := context.Background()

	agentID := "test-agent-metrics"
	if err := protocol.Subscribe(ctx, agentID, nil); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Get initial metrics
	metrics := protocol.GetMetrics()
	initialSent := metrics.TotalMessagesSent

	// Send message
	msg := &Message{
		Type:    MessageTypeTask,
		From:    "sender",
		To:      agentID,
		Content: "test",
	}
	if err := protocol.Send(ctx, msg); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Check metrics updated
	metrics = protocol.GetMetrics()
	if metrics.TotalMessagesSent != initialSent+1 {
		t.Errorf("Expected sent count %d, got %d", initialSent+1, metrics.TotalMessagesSent)
	}

	if metrics.AverageLatency == 0 {
		t.Error("Expected latency to be recorded")
	}

	cleanup(t, protocol, agentID)
}

// TestRedisProtocol_Cleanup tests message cleanup
func TestRedisProtocol_Cleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultRedisConfig()
	config.MessageTTL = 1 * time.Second // Short TTL for testing

	protocol, err := NewRedisProtocol(config)
	if err != nil {
		t.Skip("Redis not available:", err)
	}
	defer protocol.Close()

	ctx := context.Background()

	agentID := "test-agent-cleanup"
	if err := protocol.Subscribe(ctx, agentID, nil); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Send message
	msg := &Message{
		Type:    MessageTypeTask,
		From:    "sender",
		To:      agentID,
		Content: "test",
	}
	if err := protocol.Send(ctx, msg); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Run cleanup
	if err := protocol.CleanupOldMessages(ctx, 0); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	cleanup(t, protocol, agentID)
}

// Benchmark tests
func BenchmarkRedisProtocol_Send(b *testing.B) {
	config := DefaultRedisConfig()
	protocol, err := NewRedisProtocol(config)
	if err != nil {
		b.Skip("Redis not available:", err)
	}
	defer protocol.Close()

	ctx := context.Background()
	msg := &Message{
		Type:    MessageTypeTask,
		From:    "sender",
		To:      "test-agent",
		Content: "benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		protocol.Send(ctx, msg)
	}
}

func BenchmarkRedisProtocol_Receive(b *testing.B) {
	config := DefaultRedisConfig()
	protocol, err := NewRedisProtocol(config)
	if err != nil {
		b.Skip("Redis not available:", err)
	}
	defer protocol.Close()

	ctx := context.Background()
	agentID := "test-agent-bench"

	// Subscribe
	protocol.Subscribe(ctx, agentID, nil)

	// Send some messages
	for i := 0; i < 100; i++ {
		msg := &Message{
			Type:    MessageTypeTask,
			From:    "sender",
			To:      agentID,
			Content: i,
		}
		protocol.Send(ctx, msg)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		protocol.Receive(ctx, agentID)
	}
}

// cleanup removes test data from Redis
func cleanup(t *testing.T, protocol *RedisProtocol, agentID string) {
	ctx := context.Background()

	// Delete stream
	streamKey := protocol.getStreamKey(agentID)
	if err := protocol.client.Del(ctx, streamKey).Err(); err != nil && err != redis.Nil {
		t.Logf("Warning: failed to cleanup stream: %v", err)
	}

	// Delete consumer group
	groupName := protocol.config.ConsumerGroup
	if err := protocol.client.XGroupDestroy(ctx, streamKey, groupName).Err(); err != nil && err != redis.Nil {
		// Ignore error - group may not exist
	}
}
