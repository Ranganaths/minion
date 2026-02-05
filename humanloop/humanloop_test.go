package humanloop

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// Test Approval Workflow

func TestApprovalWorkflowBasic(t *testing.T) {
	store := NewMemoryApprovalStore()
	workflow := NewApprovalWorkflow(ApprovalWorkflowConfig{
		Store:          store,
		DefaultTimeout: 1 * time.Hour,
	})

	// Register a gate
	gate := &ApprovalGate{
		ID:   "deploy-gate",
		Name: "Deployment Approval",
	}
	workflow.RegisterGate(gate)

	// Verify gate is registered
	retrievedGate, ok := workflow.GetGate("deploy-gate")
	if !ok {
		t.Fatal("Expected gate to be registered")
	}
	if retrievedGate.Name != "Deployment Approval" {
		t.Errorf("Expected gate name 'Deployment Approval', got '%s'", retrievedGate.Name)
	}
}

func TestApprovalWorkflowAutoApprove(t *testing.T) {
	workflow := NewApprovalWorkflow(ApprovalWorkflowConfig{
		DefaultTimeout: 1 * time.Hour,
	})

	// Register a gate with auto-approve rules
	gate := &ApprovalGate{
		ID:   "auto-gate",
		Name: "Auto Approval Gate",
		AutoApprove: &AutoApproveRule{
			Conditions: []ApprovalCondition{
				{Field: "risk_level", Operator: "lt", Value: float64(3)},
			},
			RequireAll: true,
		},
	}
	workflow.RegisterGate(gate)

	ctx := context.Background()

	// Should auto-approve when risk_level < 3
	request, err := workflow.CreateApprovalRequest(ctx, "auto-gate", map[string]interface{}{
		"risk_level": float64(2),
	})
	if err != nil {
		t.Fatalf("Failed to create approval request: %v", err)
	}

	if request.Status != ApprovalStatusApproved {
		t.Errorf("Expected auto-approval, got status %s", request.Status)
	}
}

func TestApprovalWorkflowResolve(t *testing.T) {
	store := NewMemoryApprovalStore()
	workflow := NewApprovalWorkflow(ApprovalWorkflowConfig{
		Store:          store,
		DefaultTimeout: 1 * time.Hour,
	})

	gate := &ApprovalGate{
		ID:   "test-gate",
		Name: "Test Gate",
	}
	workflow.RegisterGate(gate)

	ctx := context.Background()

	// Create request
	request, err := workflow.CreateApprovalRequest(ctx, "test-gate", map[string]interface{}{
		"action": "test",
	})
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Approve the request
	err = workflow.Approve(ctx, request.ID, "user1", "Looks good")
	if err != nil {
		t.Fatalf("Failed to approve: %v", err)
	}

	// Verify status
	stored, _ := store.Get(ctx, request.ID)
	if stored.Status != ApprovalStatusApproved {
		t.Errorf("Expected approved status, got %s", stored.Status)
	}
	if stored.ResolvedBy != "user1" {
		t.Errorf("Expected resolved by 'user1', got '%s'", stored.ResolvedBy)
	}
}

func TestApprovalWorkflowWithListener(t *testing.T) {
	store := NewMemoryApprovalStore()
	workflow := NewApprovalWorkflow(ApprovalWorkflowConfig{
		Store:          store,
		DefaultTimeout: 10 * time.Second,
	})

	gate := &ApprovalGate{
		ID:   "listener-gate",
		Name: "Listener Test Gate",
	}
	workflow.RegisterGate(gate)

	ctx := context.Background()

	// Create request in background and wait for approval
	var result *ApprovalResult
	var waitErr error
	done := make(chan struct{})

	go func() {
		result, waitErr = workflow.RequestApproval(ctx, "listener-gate", map[string]interface{}{})
		close(done)
	}()

	// Wait a bit for request to be created
	time.Sleep(50 * time.Millisecond)

	// Get pending approvals and approve
	pending, _ := workflow.GetPendingApprovals(ctx, nil)
	if len(pending) == 0 {
		t.Fatal("Expected pending approval")
	}

	err := workflow.Approve(ctx, pending[0].ID, "approver", "Approved")
	if err != nil {
		t.Fatalf("Failed to approve: %v", err)
	}

	// Wait for result
	select {
	case <-done:
		if waitErr != nil {
			t.Fatalf("Wait returned error: %v", waitErr)
		}
		if result == nil || !result.Approved {
			t.Error("Expected approved result")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for approval result")
	}
}

// Test Notification System

func TestNotificationManager(t *testing.T) {
	manager := NewNotificationManager()

	// Create a channel notifier for testing
	notifier := NewChannelNotifier("test", 10)
	manager.RegisterNotifier(notifier)

	ctx := context.Background()
	notification := &Notification{
		Type:     NotificationTypeInfo,
		Title:    "Test Notification",
		Message:  "This is a test",
		Priority: NotificationPriorityMedium,
	}

	err := manager.Notify(ctx, notification)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	// Check the notification was received
	select {
	case received := <-notifier.Channel():
		if received.Title != "Test Notification" {
			t.Errorf("Expected title 'Test Notification', got '%s'", received.Title)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for notification")
	}
}

func TestNotificationRouting(t *testing.T) {
	manager := NewNotificationManager()

	highPriorityNotifier := NewChannelNotifier("high-priority", 10)
	allNotifier := NewChannelNotifier("all", 10)

	manager.RegisterNotifier(highPriorityNotifier)
	manager.RegisterNotifier(allNotifier)

	// Add routing rule for high priority
	manager.AddRoutingRule(NotificationRoutingRule{
		Name:          "high-priority-route",
		MinPriority:   NotificationPriorityHigh,
		NotifierNames: []string{"high-priority"},
	})

	// Add routing rule for all
	manager.AddRoutingRule(NotificationRoutingRule{
		Name:          "all-route",
		NotifierNames: []string{"all"},
	})

	ctx := context.Background()

	// Send high priority notification
	err := manager.Notify(ctx, &Notification{
		Type:     NotificationTypeWarning,
		Title:    "High Priority",
		Priority: NotificationPriorityHigh,
	})
	if err != nil {
		t.Fatalf("Failed to send: %v", err)
	}

	// Both should receive high priority
	select {
	case <-highPriorityNotifier.Channel():
		// OK
	case <-time.After(time.Second):
		t.Fatal("High priority notifier should receive")
	}

	select {
	case <-allNotifier.Channel():
		// OK
	case <-time.After(time.Second):
		t.Fatal("All notifier should receive")
	}
}

func TestNotificationHistory(t *testing.T) {
	manager := NewNotificationManager()

	logNotifier := NewLogNotifier("log", func(msg string) {
		// Suppress output
	})
	manager.RegisterNotifier(logNotifier)

	ctx := context.Background()

	// Send multiple notifications
	for i := 0; i < 5; i++ {
		manager.Notify(ctx, &Notification{
			Type:     NotificationTypeInfo,
			Title:    "Test",
			Priority: NotificationPriorityLow,
		})
	}

	// Check history
	history := manager.GetHistory(3)
	if len(history) != 3 {
		t.Errorf("Expected 3 history records, got %d", len(history))
	}
}

// Test Escalation System

func TestEscalationBasic(t *testing.T) {
	manager := NewEscalationManager(EscalationManagerConfig{})

	ctx := context.Background()

	escalation, err := manager.Escalate(ctx, "Test Error", "Something went wrong", "agent-1", EscalationLevelError, nil, nil)
	if err != nil {
		t.Fatalf("Failed to escalate: %v", err)
	}

	if escalation.Status != EscalationStatusOpen {
		t.Errorf("Expected open status, got %s", escalation.Status)
	}

	// Acknowledge
	err = manager.Acknowledge(ctx, escalation.ID, "user1")
	if err != nil {
		t.Fatalf("Failed to acknowledge: %v", err)
	}

	retrieved, ok := manager.GetEscalation(escalation.ID)
	if !ok {
		t.Fatal("Escalation not found")
	}
	if retrieved.Status != EscalationStatusAcknowledged {
		t.Errorf("Expected acknowledged status, got %s", retrieved.Status)
	}

	// Resolve
	err = manager.Resolve(ctx, escalation.ID, "user1", "Fixed the issue")
	if err != nil {
		t.Fatalf("Failed to resolve: %v", err)
	}

	retrieved, _ = manager.GetEscalation(escalation.ID)
	if retrieved.Status != EscalationStatusResolved {
		t.Errorf("Expected resolved status, got %s", retrieved.Status)
	}

	manager.Stop()
}

func TestEscalationLevelFiltering(t *testing.T) {
	manager := NewEscalationManager(EscalationManagerConfig{
		DefaultPolicy: &EscalationPolicy{
			ID:       "test-policy",
			Name:     "Test Policy",
			MinLevel: EscalationLevelWarning,
			Tiers: []EscalationTier{
				{Level: 0, Name: "Initial", Timeout: time.Hour},
			},
		},
	})

	ctx := context.Background()

	// Info level should fail (below minimum)
	_, err := manager.Escalate(ctx, "Info", "Info message", "agent", EscalationLevelInfo, nil, nil)
	if err == nil {
		t.Error("Expected error for info level below minimum")
	}

	// Warning level should succeed
	escalation, err := manager.Escalate(ctx, "Warning", "Warning message", "agent", EscalationLevelWarning, nil, nil)
	if err != nil {
		t.Errorf("Warning level should succeed: %v", err)
	}
	if escalation == nil {
		t.Error("Expected escalation")
	}

	manager.Stop()
}

func TestEscalationOpenList(t *testing.T) {
	manager := NewEscalationManager(EscalationManagerConfig{})

	ctx := context.Background()

	// Create multiple escalations
	manager.Escalate(ctx, "Error 1", "First", "agent", EscalationLevelError, nil, nil)
	e2, _ := manager.Escalate(ctx, "Error 2", "Second", "agent", EscalationLevelError, nil, nil)
	manager.Escalate(ctx, "Error 3", "Third", "agent", EscalationLevelError, nil, nil)

	// Resolve one
	manager.Resolve(ctx, e2.ID, "user", "Fixed")

	// Check open list
	open := manager.GetOpenEscalations()
	if len(open) != 2 {
		t.Errorf("Expected 2 open escalations, got %d", len(open))
	}

	manager.Stop()
}

// Test Interaction Handler

func TestInteractionConfirm(t *testing.T) {
	handler := NewInteractionHandler(InteractionHandlerConfig{
		DefaultTimeout: 10 * time.Second,
	})

	ctx := context.Background()

	// Run confirmation in background
	var confirmed bool
	var confirmErr error
	done := make(chan struct{})

	go func() {
		confirmed, confirmErr = handler.Confirm(ctx, "Confirm Action", "Do you want to proceed?", nil)
		close(done)
	}()

	// Wait for request
	time.Sleep(50 * time.Millisecond)

	// Get pending and respond
	pending := handler.GetPendingRequests()
	if len(pending) == 0 {
		t.Fatal("Expected pending request")
	}

	err := handler.Respond(ctx, pending[0].ID, "yes", "user1")
	if err != nil {
		t.Fatalf("Failed to respond: %v", err)
	}

	select {
	case <-done:
		if confirmErr != nil {
			t.Fatalf("Confirm returned error: %v", confirmErr)
		}
		if !confirmed {
			t.Error("Expected confirmed to be true")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for confirmation")
	}
}

func TestInteractionChoice(t *testing.T) {
	handler := NewInteractionHandler(InteractionHandlerConfig{
		DefaultTimeout: 10 * time.Second,
	})

	ctx := context.Background()
	options := []InteractionOption{
		{Value: "option1", Label: "Option 1"},
		{Value: "option2", Label: "Option 2"},
		{Value: "option3", Label: "Option 3"},
	}

	var response *InteractionResponse
	var choiceErr error
	done := make(chan struct{})

	go func() {
		response, choiceErr = handler.Choose(ctx, "Select Option", "Choose one", options, nil)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	pending := handler.GetPendingRequests()
	if len(pending) == 0 {
		t.Fatal("Expected pending request")
	}

	// Respond with valid choice
	err := handler.Respond(ctx, pending[0].ID, "option2", "user1")
	if err != nil {
		t.Fatalf("Failed to respond: %v", err)
	}

	select {
	case <-done:
		if choiceErr != nil {
			t.Fatalf("Choose returned error: %v", choiceErr)
		}
		if response.Value != "option2" {
			t.Errorf("Expected 'option2', got %v", response.Value)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout")
	}
}

func TestInteractionInvalidChoice(t *testing.T) {
	handler := NewInteractionHandler(InteractionHandlerConfig{
		DefaultTimeout: 10 * time.Second,
	})

	ctx := context.Background()
	options := []InteractionOption{
		{Value: "valid1", Label: "Valid 1"},
		{Value: "valid2", Label: "Valid 2"},
	}

	done := make(chan struct{})
	go func() {
		handler.Choose(ctx, "Test", "Test", options, nil)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	pending := handler.GetPendingRequests()
	if len(pending) == 0 {
		t.Fatal("Expected pending request")
	}

	// Try invalid choice
	err := handler.Respond(ctx, pending[0].ID, "invalid", "user1")
	if err == nil {
		t.Error("Expected error for invalid choice")
	}

	// Cleanup - respond with valid choice
	handler.Respond(ctx, pending[0].ID, "valid1", "user1")
	<-done
}

func TestInteractionCancel(t *testing.T) {
	handler := NewInteractionHandler(InteractionHandlerConfig{
		DefaultTimeout: 10 * time.Second,
	})

	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		handler.Query(ctx, "Test", "Test query", nil)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	pending := handler.GetPendingRequests()
	if len(pending) == 0 {
		t.Fatal("Expected pending request")
	}

	err := handler.Cancel(ctx, pending[0].ID)
	if err != nil {
		t.Fatalf("Failed to cancel: %v", err)
	}

	// Verify cancelled
	req, ok := handler.GetRequest(pending[0].ID)
	if !ok {
		t.Fatal("Request not found")
	}
	if req.Status != InteractionStatusCancelled {
		t.Errorf("Expected cancelled status, got %s", req.Status)
	}

	<-done // Wait for goroutine to finish
}

// Test Feedback Collector

func TestFeedbackCollectorRating(t *testing.T) {
	store := NewMemoryFeedbackStore()
	collector := NewFeedbackCollector(FeedbackCollectorConfig{
		Store:     store,
		MinRating: 3.0,
	})

	ctx := context.Background()

	// Collect a rating
	feedback, err := collector.CollectRating(ctx, "agent-1", "output data", 4.5, "user1")
	if err != nil {
		t.Fatalf("Failed to collect rating: %v", err)
	}

	if feedback.Type != FeedbackTypeRating {
		t.Errorf("Expected rating type, got %s", feedback.Type)
	}
	if feedback.Value != 4.5 {
		t.Errorf("Expected value 4.5, got %v", feedback.Value)
	}

	// Verify stored
	stored, _ := store.Get(ctx, feedback.ID)
	if stored == nil {
		t.Error("Feedback not stored")
	}
}

func TestFeedbackCollectorBinary(t *testing.T) {
	store := NewMemoryFeedbackStore()
	collector := NewFeedbackCollector(FeedbackCollectorConfig{
		Store: store,
	})

	ctx := context.Background()

	// Positive feedback
	feedback, err := collector.CollectBinary(ctx, "agent-1", "output", true, "user1")
	if err != nil {
		t.Fatalf("Failed to collect: %v", err)
	}

	if v, ok := feedback.Value.(bool); !ok || !v {
		t.Error("Expected positive feedback")
	}
}

func TestFeedbackCollectorCorrection(t *testing.T) {
	store := NewMemoryFeedbackStore()
	collector := NewFeedbackCollector(FeedbackCollectorConfig{
		Store: store,
	})

	ctx := context.Background()

	original := "Hello wrold"
	corrected := "Hello world"

	feedback, err := collector.CollectCorrection(ctx, "agent-1", "input", original, corrected, "user1")
	if err != nil {
		t.Fatalf("Failed to collect: %v", err)
	}

	if feedback.OutputData != original {
		t.Error("Original output not stored correctly")
	}
	if feedback.Correction != corrected {
		t.Error("Correction not stored correctly")
	}
}

func TestFeedbackCollectorPreference(t *testing.T) {
	store := NewMemoryFeedbackStore()
	collector := NewFeedbackCollector(FeedbackCollectorConfig{
		Store: store,
	})

	ctx := context.Background()

	options := []interface{}{"Option A", "Option B", "Option C"}
	feedback, err := collector.CollectPreference(ctx, "agent-1", "input", options, 1, "B is better", "user1")
	if err != nil {
		t.Fatalf("Failed to collect: %v", err)
	}

	if feedback.Preference == nil {
		t.Fatal("Preference data not stored")
	}
	if feedback.Preference.Chosen != 1 {
		t.Errorf("Expected chosen index 1, got %d", feedback.Preference.Chosen)
	}
}

func TestFeedbackStats(t *testing.T) {
	store := NewMemoryFeedbackStore()
	collector := NewFeedbackCollector(FeedbackCollectorConfig{
		Store: store,
	})

	ctx := context.Background()

	// Collect various feedback
	collector.CollectRating(ctx, "agent-1", "out1", 5.0, "user1")
	collector.CollectRating(ctx, "agent-1", "out2", 4.0, "user1")
	collector.CollectRating(ctx, "agent-1", "out3", 3.0, "user1")
	collector.CollectBinary(ctx, "agent-1", "out4", true, "user1")
	collector.CollectBinary(ctx, "agent-1", "out5", false, "user1")

	stats, err := collector.GetStats(ctx, &FeedbackFilter{AgentID: "agent-1"})
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalCount != 5 {
		t.Errorf("Expected 5 total, got %d", stats.TotalCount)
	}
	if stats.AverageRating != 4.0 {
		t.Errorf("Expected average 4.0, got %.2f", stats.AverageRating)
	}
	if stats.PositiveCount != 1 {
		t.Errorf("Expected 1 positive, got %d", stats.PositiveCount)
	}
	if stats.NegativeCount != 1 {
		t.Errorf("Expected 1 negative, got %d", stats.NegativeCount)
	}
}

func TestRLHFExporter(t *testing.T) {
	store := NewMemoryFeedbackStore()
	collector := NewFeedbackCollector(FeedbackCollectorConfig{
		Store: store,
	})

	ctx := context.Background()

	// Add preference feedback
	collector.CollectPreference(ctx, "agent-1", "question 1",
		[]interface{}{"answer A", "answer B"}, 0, "A is correct", "user1")
	collector.CollectPreference(ctx, "agent-1", "question 2",
		[]interface{}{"answer X", "answer Y"}, 1, "Y is better", "user1")

	// Add correction feedback
	collector.CollectCorrection(ctx, "agent-1", "input", "wrong output", "right output", "user1")

	exporter := NewRLHFDataExporter(store)

	// Export preferences
	preferences, err := exporter.ExportPreferences(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to export preferences: %v", err)
	}
	if len(preferences) != 2 {
		t.Errorf("Expected 2 preferences, got %d", len(preferences))
	}

	// Export corrections
	corrections, err := exporter.ExportCorrections(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to export corrections: %v", err)
	}
	if len(corrections) != 1 {
		t.Errorf("Expected 1 correction, got %d", len(corrections))
	}
}

// Test Agent Steerer

func TestAgentSteerer(t *testing.T) {
	handler := NewInteractionHandler(InteractionHandlerConfig{
		DefaultTimeout: 10 * time.Second,
	})

	steerer := NewAgentSteerer(handler, "test-agent")

	ctx := context.Background()

	// Test asking human
	var answer string
	var askErr error
	done := make(chan struct{})

	go func() {
		answer, askErr = steerer.AskHuman(ctx, "What is the answer?", nil)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	pending := handler.GetPendingRequests()
	if len(pending) == 0 {
		t.Fatal("Expected pending request")
	}

	handler.Respond(ctx, pending[0].ID, "42", "user1")

	select {
	case <-done:
		if askErr != nil {
			t.Fatalf("AskHuman error: %v", askErr)
		}
		if answer != "42" {
			t.Errorf("Expected '42', got '%s'", answer)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout")
	}
}

// Test Error Escalator

func TestErrorEscalator(t *testing.T) {
	manager := NewEscalationManager(EscalationManagerConfig{})
	escalator := NewErrorEscalator(manager, "test-agent")

	ctx := context.Background()

	testErr := errors.New("test error occurred")
	escalation, err := escalator.EscalateError(ctx, testErr, map[string]interface{}{
		"task_id": "task-123",
	})

	if err != nil {
		t.Fatalf("Failed to escalate: %v", err)
	}

	if escalation.Title != "test error occurred" {
		t.Errorf("Expected error message as title")
	}
	if escalation.Source != "test-agent" {
		t.Errorf("Expected source 'test-agent', got '%s'", escalation.Source)
	}

	manager.Stop()
}

// Concurrency tests

func TestApprovalWorkflowConcurrency(t *testing.T) {
	store := NewMemoryApprovalStore()
	workflow := NewApprovalWorkflow(ApprovalWorkflowConfig{
		Store:          store,
		DefaultTimeout: 10 * time.Second,
	})

	gate := &ApprovalGate{ID: "concurrent-gate", Name: "Concurrent Gate"}
	workflow.RegisterGate(gate)

	ctx := context.Background()
	var wg sync.WaitGroup
	requestCount := 10

	// Create multiple requests concurrently
	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := workflow.CreateApprovalRequest(ctx, "concurrent-gate", map[string]interface{}{
				"index": i,
			})
			if err != nil {
				t.Errorf("Request %d failed: %v", i, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all requests created
	pending, _ := workflow.GetPendingApprovals(ctx, nil)
	if len(pending) != requestCount {
		t.Errorf("Expected %d pending, got %d", requestCount, len(pending))
	}
}

func TestFeedbackStoreConcurrency(t *testing.T) {
	store := NewMemoryFeedbackStore()
	collector := NewFeedbackCollector(FeedbackCollectorConfig{
		Store: store,
	})

	ctx := context.Background()
	var wg sync.WaitGroup
	feedbackCount := 50

	for i := 0; i < feedbackCount; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			collector.CollectRating(ctx, "agent-1", "output", float64(i%5)+1, "user")
		}(i)
	}

	wg.Wait()

	// Verify all stored
	list, _ := store.List(ctx, nil)
	if len(list) != feedbackCount {
		t.Errorf("Expected %d feedback items, got %d", feedbackCount, len(list))
	}
}

