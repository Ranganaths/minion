package workflow

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDAGValidation(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *DAG
		wantErr bool
	}{
		{
			name: "valid simple DAG",
			setup: func() *DAG {
				dag := NewDAG("test", "Test DAG")
				dag.AddNode(NewTaskNode("a", "Task A", PassThrough()))
				dag.AddNode(NewTaskNode("b", "Task B", PassThrough()))
				dag.AddEdge("a", "b")
				return dag
			},
			wantErr: false,
		},
		{
			name: "empty DAG",
			setup: func() *DAG {
				return NewDAG("test", "Test DAG")
			},
			wantErr: true,
		},
		{
			name: "DAG with cycle",
			setup: func() *DAG {
				dag := NewDAG("test", "Test DAG")
				dag.AddNode(NewTaskNode("a", "Task A", PassThrough()))
				dag.AddNode(NewTaskNode("b", "Task B", PassThrough()))
				dag.AddNode(NewTaskNode("c", "Task C", PassThrough()))
				dag.AddEdge("a", "b")
				dag.AddEdge("b", "c")
				dag.AddEdge("c", "a") // Creates cycle
				return dag
			},
			wantErr: true,
		},
		{
			name: "DAG with invalid edge",
			setup: func() *DAG {
				dag := NewDAG("test", "Test DAG")
				dag.AddNode(NewTaskNode("a", "Task A", PassThrough()))
				dag.Edges = append(dag.Edges, &Edge{From: "a", To: "nonexistent"})
				return dag
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dag := tt.setup()
			err := dag.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTopologicalSort(t *testing.T) {
	dag := NewDAG("test", "Test DAG")
	dag.AddNode(NewTaskNode("a", "Task A", PassThrough()))
	dag.AddNode(NewTaskNode("b", "Task B", PassThrough()))
	dag.AddNode(NewTaskNode("c", "Task C", PassThrough()))
	dag.AddNode(NewTaskNode("d", "Task D", PassThrough()))

	dag.AddEdge("a", "b")
	dag.AddEdge("a", "c")
	dag.AddEdge("b", "d")
	dag.AddEdge("c", "d")

	order, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() error = %v", err)
	}

	// Verify order: a must come first, d must come last
	if order[0] != "a" {
		t.Errorf("First node should be 'a', got %s", order[0])
	}
	if order[len(order)-1] != "d" {
		t.Errorf("Last node should be 'd', got %s", order[len(order)-1])
	}
}

func TestSimpleWorkflowExecution(t *testing.T) {
	ctx := context.Background()

	var executionOrder []string
	dag, err := NewDAGBuilder("simple-workflow").
		Name("Simple Workflow").
		Task("step1", "Step 1", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			executionOrder = append(executionOrder, "step1")
			return map[string]interface{}{"step1_done": true}, nil
		}).
		Task("step2", "Step 2", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			executionOrder = append(executionOrder, "step2")
			return map[string]interface{}{"step2_done": true}, nil
		}).
		Task("step3", "Step 3", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			executionOrder = append(executionOrder, "step3")
			return map[string]interface{}{"result": "complete"}, nil
		}).
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	executor := NewDAGExecutor(dag, nil)
	result, err := executor.Execute(ctx, map[string]interface{}{"initial": "value"})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Workflow should have succeeded")
	}

	if len(executionOrder) != 3 {
		t.Errorf("Expected 3 executions, got %d", len(executionOrder))
	}

	// Verify sequential order
	for i, step := range []string{"step1", "step2", "step3"} {
		if executionOrder[i] != step {
			t.Errorf("Expected %s at position %d, got %s", step, i, executionOrder[i])
		}
	}
}

func TestParallelExecution(t *testing.T) {
	ctx := context.Background()

	var counter int32

	dag, err := NewDAGBuilder("parallel-workflow").
		ParallelTasks("parallel",
			TaskDef{ID: "task1", Name: "Task 1", Handler: func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
				atomic.AddInt32(&counter, 1)
				time.Sleep(50 * time.Millisecond)
				return map[string]interface{}{"task1": "done"}, nil
			}},
			TaskDef{ID: "task2", Name: "Task 2", Handler: func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
				atomic.AddInt32(&counter, 1)
				time.Sleep(50 * time.Millisecond)
				return map[string]interface{}{"task2": "done"}, nil
			}},
			TaskDef{ID: "task3", Name: "Task 3", Handler: func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
				atomic.AddInt32(&counter, 1)
				time.Sleep(50 * time.Millisecond)
				return map[string]interface{}{"task3": "done"}, nil
			}},
		).
		Task("final", "Final", PassThrough()).
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	executor := NewDAGExecutor(dag, &ExecutorConfig{
		MaxParallel:    10,
		DefaultTimeout: time.Minute,
	})

	start := time.Now()
	result, err := executor.Execute(ctx, nil)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Workflow should have succeeded")
	}

	if atomic.LoadInt32(&counter) != 3 {
		t.Errorf("Expected 3 task executions, got %d", counter)
	}

	// With parallel execution, should complete faster than 150ms
	if duration > 200*time.Millisecond {
		t.Errorf("Parallel execution took too long: %v", duration)
	}
}

func TestConditionalExecution(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		conditionKey  string
		conditionVal  bool
		expectBranch  string
	}{
		{"true branch", "should_process", true, "true_branch"},
		{"false branch", "should_process", false, "false_branch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var executedBranch string

			dag, err := NewDAGBuilder("conditional-workflow").
				Task("start", "Start", PassThrough()).
				Condition("check", KeyTrue("should_process")).
					ThenTask("true_branch", "True Branch", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
						executedBranch = "true_branch"
						return nil, nil
					}).
					ElseTask("false_branch", "False Branch", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
						executedBranch = "false_branch"
						return nil, nil
					}).
				End().
				Build()

			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			executor := NewDAGExecutor(dag, nil)
			result, err := executor.Execute(ctx, map[string]interface{}{
				tt.conditionKey: tt.conditionVal,
			})

			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if !result.IsSuccess() {
				t.Errorf("Workflow should have succeeded")
			}

			if executedBranch != tt.expectBranch {
				t.Errorf("Expected branch %s, got %s", tt.expectBranch, executedBranch)
			}
		})
	}
}

func TestLoopExecution(t *testing.T) {
	ctx := context.Background()

	t.Run("count loop", func(t *testing.T) {
		var iterations int

		dag, err := NewDAGBuilder("count-loop").
			Times("loop", 5, func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
				iterations++
				return nil, nil
			}).
			Build()

		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}

		executor := NewDAGExecutor(dag, nil)
		result, err := executor.Execute(ctx, nil)

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if !result.IsSuccess() {
			t.Errorf("Workflow should have succeeded")
		}

		if iterations != 5 {
			t.Errorf("Expected 5 iterations, got %d", iterations)
		}
	})

	t.Run("forEach loop", func(t *testing.T) {
		var processedItems []string

		dag, err := NewDAGBuilder("foreach-loop").
			ForEach("process", "items", "item", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
				item := input["item"].(string)
				processedItems = append(processedItems, item)
				return nil, nil
			}).End().
			Build()

		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}

		executor := NewDAGExecutor(dag, nil)
		result, err := executor.Execute(ctx, map[string]interface{}{
			"items": []interface{}{"a", "b", "c"},
		})

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if !result.IsSuccess() {
			t.Errorf("Workflow should have succeeded")
		}

		if len(processedItems) != 3 {
			t.Errorf("Expected 3 processed items, got %d", len(processedItems))
		}
	})
}

func TestRetryPolicy(t *testing.T) {
	ctx := context.Background()

	var attempts int

	dag, err := NewDAGBuilder("retry-workflow").
		Task("flaky", "Flaky Task", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("temporary error")
			}
			return map[string]interface{}{"success": true}, nil
		}).
		WithRetry(5, 10*time.Millisecond).
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	executor := NewDAGExecutor(dag, nil)
	result, err := executor.Execute(ctx, nil)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Workflow should have succeeded after retries")
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestTimeoutHandling(t *testing.T) {
	ctx := context.Background()

	dag, err := NewDAGBuilder("timeout-workflow").
		Task("slow", "Slow Task", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			select {
			case <-time.After(5 * time.Second):
				return nil, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}).
		WithTimeout(100 * time.Millisecond).
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	executor := NewDAGExecutor(dag, nil)
	result, err := executor.Execute(ctx, nil)

	if err == nil {
		t.Error("Expected timeout error")
	}

	if result.IsSuccess() {
		t.Error("Workflow should have failed due to timeout")
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	dag, err := NewDAGBuilder("cancel-workflow").
		Task("task1", "Task 1", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			cancel() // Cancel after first task
			return nil, nil
		}).
		Task("task2", "Task 2", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			time.Sleep(100 * time.Millisecond)
			return nil, nil
		}).
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	executor := NewDAGExecutor(dag, nil)
	result, _ := executor.Execute(ctx, nil)

	if result.Status != WorkflowStatusCancelled && result.Status != WorkflowStatusFailed {
		t.Errorf("Expected cancelled or failed status, got %s", result.Status)
	}
}

func TestWorkflowState(t *testing.T) {
	state := NewWorkflowState("test-wf", "run-1", map[string]interface{}{"key": "value"})

	// Test status transitions
	state.SetStatus(WorkflowStatusRunning)
	if state.Status != WorkflowStatusRunning {
		t.Errorf("Expected running status")
	}

	// Test node status
	state.SetNodeStatus("node1", NodeStatusRunning)
	ns := state.GetNodeState("node1")
	if ns == nil || ns.Status != NodeStatusRunning {
		t.Errorf("Expected node status running")
	}

	// Test variables
	state.SetVariable("var1", "value1")
	v, ok := state.GetVariable("var1")
	if !ok || v != "value1" {
		t.Errorf("Expected variable value1")
	}

	// Test checkpoint
	cp := state.CreateCheckpoint("cp1", "test checkpoint")
	if cp == nil {
		t.Error("Expected checkpoint to be created")
	}

	// Test restore
	state.SetVariable("var1", "changed")
	state.RestoreFromCheckpoint(cp)
	v, _ = state.GetVariable("var1")
	if v != "value1" {
		t.Errorf("Expected restored variable value1, got %v", v)
	}
}

func TestDAGCloneAndSerialization(t *testing.T) {
	original := NewDAG("test", "Test DAG")
	original.AddNode(NewTaskNode("a", "Task A", PassThrough()))
	original.AddNode(NewTaskNode("b", "Task B", PassThrough()))
	original.AddEdge("a", "b")
	original.Metadata["key"] = "value"

	// Test clone
	cloned := original.Clone()
	if cloned.ID != original.ID {
		t.Errorf("Clone ID mismatch")
	}
	if len(cloned.Nodes) != len(original.Nodes) {
		t.Errorf("Clone nodes count mismatch")
	}

	// Modify clone shouldn't affect original
	cloned.Metadata["key"] = "modified"
	if original.Metadata["key"] != "value" {
		t.Errorf("Clone modification affected original")
	}

	// Test JSON serialization
	jsonData, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}

	restored, err := FromJSON(jsonData)
	if err != nil {
		t.Fatalf("FromJSON error: %v", err)
	}

	if restored.ID != original.ID {
		t.Errorf("Restored ID mismatch")
	}
}

func TestDOTExport(t *testing.T) {
	dag := NewDAG("test", "Test Workflow")
	dag.AddNode(NewTaskNode("start", "Start", PassThrough()))
	dag.AddNode(NewParallelNode("parallel", "Parallel"))
	dag.AddNode(NewTaskNode("task1", "Task 1", PassThrough()))
	dag.AddNode(NewTaskNode("task2", "Task 2", PassThrough()))
	dag.AddNode(NewJoinNode("join", "Join"))

	dag.AddEdge("start", "parallel")
	dag.AddEdge("parallel", "task1")
	dag.AddEdge("parallel", "task2")
	dag.AddEdge("task1", "join")
	dag.AddEdge("task2", "join")

	dot := dag.ToDOT()

	if dot == "" {
		t.Error("DOT output should not be empty")
	}

	// Check for expected content
	expectedContent := []string{
		"digraph",
		"start",
		"parallel",
		"task1",
		"task2",
		"join",
		"->",
	}

	for _, expected := range expectedContent {
		if !containsSubstring(dot, expected) {
			t.Errorf("DOT output missing expected content: %s", expected)
		}
	}
}

func TestSchedulerBasics(t *testing.T) {
	dag := NewDAG("test", "Test DAG")
	dag.AddNode(NewTaskNode("a", "Task A", PassThrough()))
	dag.AddNode(NewTaskNode("b", "Task B", PassThrough()))
	dag.AddNode(NewTaskNode("c", "Task C", PassThrough()))

	dag.AddEdge("a", "b")
	dag.AddEdge("a", "c")

	state := NewWorkflowState(dag.ID, "run-1", nil)
	for nodeID := range dag.Nodes {
		state.SetNodeStatus(nodeID, NodeStatusPending)
	}

	scheduler, err := NewScheduler(dag, state, nil)
	if err != nil {
		t.Fatalf("NewScheduler error: %v", err)
	}

	ctx := context.Background()

	// First task should be 'a'
	taskID, err := scheduler.GetNextTask(ctx)
	if err != nil {
		t.Fatalf("GetNextTask error: %v", err)
	}
	if taskID != "a" {
		t.Errorf("Expected first task 'a', got %s", taskID)
	}

	// Complete 'a'
	scheduler.CompleteTask("a", true)

	// Now 'b' and 'c' should be available
	completed, total := scheduler.GetProgress()
	if completed != 1 || total != 3 {
		t.Errorf("Expected progress 1/3, got %d/%d", completed, total)
	}
}

func TestConditionFunctions(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		cond     ConditionFunc
		input    map[string]interface{}
		expected bool
	}{
		{"Always", Always(), nil, true},
		{"Never", Never(), nil, false},
		{"HasKey exists", HasKey("key"), map[string]interface{}{"key": "val"}, true},
		{"HasKey missing", HasKey("key"), map[string]interface{}{}, false},
		{"KeyEquals match", KeyEquals("key", "val"), map[string]interface{}{"key": "val"}, true},
		{"KeyEquals mismatch", KeyEquals("key", "val"), map[string]interface{}{"key": "other"}, false},
		{"KeyTrue true", KeyTrue("flag"), map[string]interface{}{"flag": true}, true},
		{"KeyTrue false", KeyTrue("flag"), map[string]interface{}{"flag": false}, false},
		{"Not Always", Not(Always()), nil, false},
		{"And both true", And(Always(), Always()), nil, true},
		{"And one false", And(Always(), Never()), nil, false},
		{"Or one true", Or(Always(), Never()), nil, true},
		{"Or both false", Or(Never(), Never()), nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.cond(ctx, tt.input)
			if err != nil {
				t.Fatalf("Condition error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSubWorkflow(t *testing.T) {
	ctx := context.Background()

	// Create sub-workflow
	subDAG, err := NewDAGBuilder("sub-workflow").
		Task("sub-task1", "Sub Task 1", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"from_sub": "value"}, nil
		}).
		Build()

	if err != nil {
		t.Fatalf("Sub-workflow build error: %v", err)
	}

	// Create main workflow with sub-workflow
	mainDAG, err := NewDAGBuilder("main-workflow").
		Task("start", "Start", PassThrough()).
		SubWorkflow("sub", subDAG).
		Task("end", "End", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			// Check sub-workflow output is available
			if v, ok := input["from_sub"]; !ok || v != "value" {
				return nil, errors.New("sub-workflow output not received")
			}
			return map[string]interface{}{"final": "done"}, nil
		}).
		Build()

	if err != nil {
		t.Fatalf("Main workflow build error: %v", err)
	}

	executor := NewDAGExecutor(mainDAG, nil)
	result, err := executor.Execute(ctx, nil)

	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if !result.IsSuccess() {
		t.Error("Workflow should have succeeded")
	}
}

func TestWorkflowMetrics(t *testing.T) {
	ctx := context.Background()

	dag, err := NewDAGBuilder("metrics-workflow").
		Task("task1", "Task 1", PassThrough()).
		Task("task2", "Task 2", PassThrough()).
		Task("task3", "Task 3", PassThrough()).
		Build()

	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	executor := NewDAGExecutor(dag, nil)
	result, err := executor.Execute(ctx, nil)

	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if result.Metrics.TotalNodes != 3 {
		t.Errorf("Expected 3 total nodes, got %d", result.Metrics.TotalNodes)
	}

	if result.Metrics.CompletedNodes != 3 {
		t.Errorf("Expected 3 completed nodes, got %d", result.Metrics.CompletedNodes)
	}

	if result.Metrics.TotalDuration <= 0 {
		t.Error("Expected positive duration")
	}
}
