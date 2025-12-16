package multiagent

import (
	"context"
	"testing"
)

func TestInMemoryProtocol_SendReceive(t *testing.T) {
	ctx := context.Background()
	protocol := NewInMemoryProtocol(nil)

	// Subscribe agent
	err := protocol.Subscribe(ctx, "agent-1", []MessageType{MessageTypeTask})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Send message
	msg := &Message{
		Type:    MessageTypeTask,
		From:    "orchestrator",
		To:      "agent-1",
		Content: "test task",
	}

	err = protocol.Send(ctx, msg)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Receive message
	messages, err := protocol.Receive(ctx, "agent-1")
	if err != nil {
		t.Fatalf("Failed to receive messages: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].Content != "test task" {
		t.Errorf("Expected 'test task', got %v", messages[0].Content)
	}
}

func TestInMemoryProtocol_Broadcast(t *testing.T) {
	ctx := context.Background()
	impl := NewInMemoryProtocol(nil)

	// Register group
	agentIDs := []string{"agent-1", "agent-2", "agent-3"}
	err := impl.RegisterGroup("group-1", agentIDs)
	if err != nil {
		t.Fatalf("Failed to register group: %v", err)
	}

	// Subscribe agents
	for _, id := range agentIDs {
		impl.Subscribe(ctx, id, []MessageType{MessageTypeInform})
	}

	// Broadcast message
	msg := &Message{
		Type:    MessageTypeInform,
		From:    "orchestrator",
		Content: "broadcast message",
	}

	err = impl.Broadcast(ctx, msg, "group-1")
	if err != nil {
		t.Fatalf("Failed to broadcast: %v", err)
	}

	// Check each agent received the message
	for _, id := range agentIDs {
		messages, _ := impl.Receive(ctx, id)
		if len(messages) != 1 {
			t.Errorf("Agent %s: expected 1 message, got %d", id, len(messages))
		}
	}
}

func TestInMemoryProtocol_Security(t *testing.T) {
	ctx := context.Background()

	security := &SecurityPolicy{
		AllowedAgents: []string{"agent-1"},
	}

	protocol := NewInMemoryProtocol(security)

	// Try to send from allowed agent
	msg := &Message{
		Type:    MessageTypeTask,
		From:    "agent-1",
		To:      "agent-2",
		Content: "test",
	}

	err := protocol.Send(ctx, msg)
	if err != nil {
		t.Errorf("Allowed agent should be able to send: %v", err)
	}

	// Try to send from denied agent
	msg2 := &Message{
		Type:    MessageTypeTask,
		From:    "agent-3",
		To:      "agent-2",
		Content: "test",
	}

	err = protocol.Send(ctx, msg2)
	if err == nil {
		t.Error("Denied agent should not be able to send")
	}
}

func TestTaskLedger_CRUD(t *testing.T) {
	ctx := context.Background()
	ledger := NewTaskLedger()

	// Create task
	task := &Task{
		Name:        "Test Task",
		Description: "Test description",
		Type:        "test",
		Priority:    PriorityNormal,
		Status:      TaskStatusPending,
	}

	err := ledger.CreateTask(ctx, task)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	if task.ID == "" {
		t.Error("Task ID should be generated")
	}

	// Get task
	retrieved, err := ledger.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if retrieved.Name != task.Name {
		t.Errorf("Expected name %s, got %s", task.Name, retrieved.Name)
	}

	// Complete task
	output := map[string]string{"result": "success"}
	err = ledger.CompleteTask(ctx, task.ID, output)
	if err != nil {
		t.Fatalf("Failed to complete task: %v", err)
	}

	// Verify completion
	completed, _ := ledger.GetTask(ctx, task.ID)
	if completed.Status != TaskStatusCompleted {
		t.Errorf("Task should be completed, got %s", completed.Status)
	}

	if completed.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
}

func TestProgressLedger_Tracking(t *testing.T) {
	ctx := context.Background()
	ledger := NewProgressLedger()

	taskID := "task-1"

	// Add entries
	for i := 1; i <= 5; i++ {
		entry := &ProgressEntry{
			TaskID:      taskID,
			AgentID:     "agent-1",
			Action:      "step",
			Description: "Step description",
			Status:      "in_progress",
		}

		err := ledger.AddEntry(ctx, entry)
		if err != nil {
			t.Fatalf("Failed to add entry: %v", err)
		}
	}

	// Get progress
	entries, err := ledger.GetProgress(ctx, taskID)
	if err != nil {
		t.Fatalf("Failed to get progress: %v", err)
	}

	if len(entries) != 5 {
		t.Errorf("Expected 5 entries, got %d", len(entries))
	}

	// Get current step
	step := ledger.GetCurrentStep(ctx, taskID)
	if step != 5 {
		t.Errorf("Expected step 5, got %d", step)
	}

	// Get latest entry
	latest, err := ledger.GetLatestEntry(ctx, taskID)
	if err != nil {
		t.Fatalf("Failed to get latest entry: %v", err)
	}

	if latest.Step != 5 {
		t.Errorf("Latest entry should be step 5, got %d", latest.Step)
	}
}

func TestProtocol_Metrics(t *testing.T) {
	ctx := context.Background()
	impl := NewInMemoryProtocol(nil)

	// Send some messages
	for i := 0; i < 10; i++ {
		msg := &Message{
			Type:    MessageTypeTask,
			From:    "sender",
			To:      "receiver",
			Content: i,
		}
		impl.Send(ctx, msg)
	}

	// Get metrics
	metrics := impl.GetMetrics()

	if metrics.TotalMessagesSent != 10 {
		t.Errorf("Expected 10 messages sent, got %d", metrics.TotalMessagesSent)
	}

	// Receive messages
	impl.Subscribe(ctx, "receiver", []MessageType{MessageTypeTask})
	impl.Receive(ctx, "receiver")

	metrics = impl.GetMetrics()
	if metrics.TotalMessagesReceived != 10 {
		t.Errorf("Expected 10 messages received, got %d", metrics.TotalMessagesReceived)
	}
}

func TestTaskLedger_Concurrency(t *testing.T) {
	ctx := context.Background()
	ledger := NewTaskLedger()

	// Create tasks concurrently
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(id int) {
			task := &Task{
				Name:     "Concurrent Task",
				Type:     "test",
				Priority: PriorityNormal,
			}
			ledger.CreateTask(ctx, task)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Check task count
	ledger.mu.RLock()
	count := len(ledger.tasks)
	ledger.mu.RUnlock()

	if count != 100 {
		t.Errorf("Expected 100 tasks, got %d", count)
	}
}

func BenchmarkProtocol_Send(b *testing.B) {
	ctx := context.Background()
	protocol := NewInMemoryProtocol(nil)

	msg := &Message{
		Type:    MessageTypeTask,
		From:    "sender",
		To:      "receiver",
		Content: "benchmark message",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		protocol.Send(ctx, msg)
	}
}

func BenchmarkLedger_CreateTask(b *testing.B) {
	ctx := context.Background()
	ledger := NewTaskLedger()

	task := &Task{
		Name:     "Benchmark Task",
		Type:     "test",
		Priority: PriorityNormal,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ledger.CreateTask(ctx, task)
	}
}
