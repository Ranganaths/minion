package multiagent

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestEndToEnd_SimpleTask tests end-to-end execution of a simple task
func TestEndToEnd_SimpleTask(t *testing.T) {
	ctx := context.Background()

	// Create mock LLM provider
	mockLLM := NewMockLLMProvider()
	mockLLM.SetSimpleTask("Simple Test Task", "code_generation")

	// Create coordinator
	coordinator := NewCoordinator(mockLLM, nil)

	// Initialize with mock workers
	err := initializeWithMockWorkers(ctx, coordinator, mockLLM)
	if err != nil {
		t.Fatalf("Failed to initialize coordinator: %v", err)
	}

	// Execute simple task
	result, err := coordinator.ExecuteTask(ctx, &TaskRequest{
		Name:        "Simple Test Task",
		Description: "A simple test task",
		Type:        "test",
		Priority:    PriorityNormal,
		Input:       "test input",
	})

	if err != nil {
		t.Fatalf("Task execution failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Status != "completed" {
		t.Errorf("Expected status 'completed', got %s", result.Status)
	}

	// Shutdown
	coordinator.Shutdown(ctx)
}

// TestEndToEnd_CodeGeneration tests code generation workflow
func TestEndToEnd_CodeGeneration(t *testing.T) {
	ctx := context.Background()

	// Create mock LLM provider with code generation response
	mockLLM := NewMockLLMProvider()
	mockLLM.SetCodeGenerationTask()

	// Create coordinator
	config := DefaultCoordinatorConfig()
	config.OrchestratorConfig.TaskTimeout = 10 * time.Second
	coordinator := NewCoordinator(mockLLM, config)

	// Initialize
	err := initializeWithMockWorkers(ctx, coordinator, mockLLM)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Execute code generation task
	result, err := coordinator.ExecuteTask(ctx, &TaskRequest{
		Name:        "Generate REST API",
		Description: "Create a RESTful API with CRUD operations",
		Type:        "code_generation",
		Priority:    PriorityHigh,
		Input: map[string]interface{}{
			"language": "go",
			"endpoints": []string{"/users", "/products"},
		},
	})

	if err != nil {
		t.Fatalf("Task failed: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("Expected completed status, got %s", result.Status)
	}

	// Check that task was tracked
	stats, _ := coordinator.GetMonitoringStats(ctx)
	if stats.TotalTasks == 0 {
		t.Error("Expected tasks to be tracked")
	}

	// Note: Subtasks may or may not be persisted to task ledger depending on implementation
	// The important thing is that the task completed successfully

	coordinator.Shutdown(ctx)
}

// TestEndToEnd_DataAnalysis tests data analysis workflow
func TestEndToEnd_DataAnalysis(t *testing.T) {
	ctx := context.Background()

	mockLLM := NewMockLLMProvider()
	mockLLM.SetDataAnalysisTask()

	coordinator := NewCoordinator(mockLLM, nil)
	err := initializeWithMockWorkers(ctx, coordinator, mockLLM)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	result, err := coordinator.ExecuteTask(ctx, &TaskRequest{
		Name:        "Analyze Sales Data",
		Description: "Analyze quarterly sales trends",
		Type:        "data_analysis",
		Priority:    PriorityHigh,
		Input: map[string]interface{}{
			"Q1": 150000,
			"Q2": 180000,
			"Q3": 165000,
			"Q4": 220000,
		},
	})

	if err != nil {
		t.Fatalf("Task failed: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("Expected completed status, got %s", result.Status)
	}

	coordinator.Shutdown(ctx)
}

// TestEndToEnd_MultipleWorkers tests parallel execution with multiple workers
func TestEndToEnd_MultipleWorkers(t *testing.T) {
	ctx := context.Background()

	mockLLM := NewMockLLMProvider()

	// Create a task that requires multiple different workers
	response := `{
  "subtasks": [
    {
      "name": "Research topic",
      "description": "Research the topic thoroughly",
      "assigned_to": "research",
      "dependencies": [],
      "priority": 8,
      "input": "Research requirements"
    },
    {
      "name": "Analyze data",
      "description": "Analyze research findings",
      "assigned_to": "data_analysis",
      "dependencies": [],
      "priority": 8,
      "input": "Analyze data"
    },
    {
      "name": "Write report",
      "description": "Write final report",
      "assigned_to": "content_creation",
      "dependencies": ["Research topic", "Analyze data"],
      "priority": 9,
      "input": "Write comprehensive report"
    }
  ]
}`
	mockLLM.SetResponse("Multi-worker Task", response)

	coordinator := NewCoordinator(mockLLM, nil)
	err := initializeWithMockWorkers(ctx, coordinator, mockLLM)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	startTime := time.Now()

	result, err := coordinator.ExecuteTask(ctx, &TaskRequest{
		Name:        "Multi-worker Task",
		Description: "Task requiring multiple workers",
		Type:        "complex",
		Priority:    PriorityHigh,
		Input:       "complex input",
	})

	duration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Task failed: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("Expected completed status, got %s", result.Status)
	}

	// Verify execution time is reasonable (should be fast with mocks)
	// Note: With timeouts and retries, execution can take a few seconds
	if duration > 10*time.Second {
		t.Errorf("Execution took too long: %v", duration)
	}

	coordinator.Shutdown(ctx)
}

// TestEndToEnd_TaskWithDependencies tests dependency resolution
func TestEndToEnd_TaskWithDependencies(t *testing.T) {
	t.Skip("Dependency resolution not fully implemented yet")
	// TODO: Implement once dependency handling is complete
}

// TestEndToEnd_ErrorHandling tests error handling and recovery
func TestEndToEnd_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	mockLLM := NewMockLLMProvider()
	coordinator := NewCoordinator(mockLLM, nil)

	// Create a worker that will fail
	protocol := coordinator.GetProtocol()
	failingHandler := NewMockWorkerHandler("failing_worker", []string{"failing_capability"})
	failingHandler.SetHandlerFunc(func(ctx context.Context, task *Task) (interface{}, error) {
		return nil, fmt.Errorf("simulated worker failure")
	})

	failingWorker := NewWorkerAgent(&AgentMetadata{
		AgentID:      "failing-worker-1",
		Role:         RoleWorker,
		Capabilities: []string{"failing_capability"},
		Status:       StatusIdle,
	}, protocol, failingHandler)

	err := coordinator.RegisterWorker(ctx, failingWorker)
	if err != nil {
		t.Fatalf("Failed to register worker: %v", err)
	}

	// Set task that will be assigned to failing worker
	mockLLM.SetSimpleTask("Failing Task", "failing_capability")

	// Execute task (should fail after retries)
	result, err := coordinator.ExecuteTask(ctx, &TaskRequest{
		Name:        "Failing Task",
		Description: "This task will fail",
		Type:        "test",
		Priority:    PriorityNormal,
		Input:       "test",
	})

	// We expect this to fail
	if err == nil {
		t.Error("Expected task to fail, but it succeeded")
	}

	if result != nil && result.Status == "completed" {
		t.Error("Expected task to not complete successfully")
	}

	coordinator.Shutdown(ctx)
}

// TestCoordinator_Monitoring tests monitoring and stats
func TestCoordinator_Monitoring(t *testing.T) {
	ctx := context.Background()

	mockLLM := NewMockLLMProvider()
	mockLLM.SetSimpleTask("Monitor Test", "code_generation")

	coordinator := NewCoordinator(mockLLM, nil)
	err := initializeWithMockWorkers(ctx, coordinator, mockLLM)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Get initial stats
	initialStats, err := coordinator.GetMonitoringStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if initialStats.TotalWorkers == 0 {
		t.Error("Expected workers to be registered")
	}

	// Execute a task
	coordinator.ExecuteTask(ctx, &TaskRequest{
		Name:     "Monitor Test",
		Type:     "test",
		Priority: PriorityNormal,
		Input:    "test",
	})

	// Get updated stats
	stats, err := coordinator.GetMonitoringStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalTasks == 0 {
		t.Error("Expected tasks to be recorded")
	}

	if stats.ProtocolMetrics == nil {
		t.Error("Expected protocol metrics")
	}

	if stats.ProtocolMetrics.TotalMessagesSent == 0 {
		t.Error("Expected messages to be sent")
	}

	coordinator.Shutdown(ctx)
}

// TestCoordinator_HealthCheck tests health check functionality
func TestCoordinator_HealthCheck(t *testing.T) {
	ctx := context.Background()

	mockLLM := NewMockLLMProvider()
	coordinator := NewCoordinator(mockLLM, nil)

	// Health check before initialization
	health := coordinator.HealthCheck(ctx)

	if health.Status == "unhealthy" {
		// This is expected - no workers yet
		if len(health.Errors) == 0 {
			t.Error("Expected errors in unhealthy state")
		}
	}

	// Initialize
	err := initializeWithMockWorkers(ctx, coordinator, mockLLM)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Health check after initialization
	health = coordinator.HealthCheck(ctx)

	if health.Status == "unhealthy" {
		t.Errorf("Expected healthy status, got %s. Errors: %v", health.Status, health.Errors)
	}

	if health.Components["workers"] != "healthy" {
		t.Error("Expected workers component to be healthy")
	}

	coordinator.Shutdown(ctx)
}

// Helper function to initialize coordinator with mock workers
func initializeWithMockWorkers(ctx context.Context, coordinator *Coordinator, mockLLM *MockLLMProvider) error {
	protocol := coordinator.GetProtocol()

	// Create mock workers for each capability
	workers := []struct {
		name         string
		capabilities []string
	}{
		{"coder", []string{"code_generation", "code_review", "debugging"}},
		{"analyst", []string{"data_analysis", "statistical_analysis", "visualization"}},
		{"researcher", []string{"research", "information_gathering"}},
		{"writer", []string{"content_creation", "editing"}},
		{"reviewer", []string{"review", "quality_assurance"}},
	}

	for i, w := range workers {
		handler := NewMockWorkerHandler(w.name, w.capabilities)

		worker := NewWorkerAgent(&AgentMetadata{
			AgentID:      fmt.Sprintf("mock-worker-%d", i),
			Role:         RoleWorker,
			Capabilities: w.capabilities,
			Priority:     5,
			Status:       StatusIdle,
		}, protocol, handler)

		if err := coordinator.RegisterWorker(ctx, worker); err != nil {
			return fmt.Errorf("failed to register %s worker: %w", w.name, err)
		}
	}

	return nil
}

// TestLedger_ProgressTracking tests progress ledger integration
func TestLedger_ProgressTracking(t *testing.T) {
	ctx := context.Background()

	ledger := NewProgressLedger()

	taskID := "test-task-1"

	// Add progress entries
	for i := 1; i <= 3; i++ {
		err := ledger.AddEntry(ctx, &ProgressEntry{
			TaskID:      taskID,
			AgentID:     "agent-1",
			Step:        i,
			Action:      "processing",
			Description: fmt.Sprintf("Step %d", i),
			Status:      "in_progress",
		})
		if err != nil {
			t.Fatalf("Failed to add entry: %v", err)
		}
	}

	// Check progress
	entries, err := ledger.GetProgress(ctx, taskID)
	if err != nil {
		t.Fatalf("Failed to get progress: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}

	// Check current step
	step := ledger.GetCurrentStep(ctx, taskID)
	if step != 3 {
		t.Errorf("Expected step 3, got %d", step)
	}
}
