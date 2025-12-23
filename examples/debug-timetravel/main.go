// Example demonstrating the Minion Debug Studio with time-travel debugging capabilities.
//
// This example shows how to:
// 1. Set up execution recording
// 2. Record checkpoints during agent execution
// 3. Use the Debug API server
// 4. Launch the Terminal UI debugger
// 5. Replay and branch executions
//
// Run with: go run main.go [mode]
// Modes:
//   - record: Record a sample execution
//   - api: Start the Debug API server
//   - tui: Launch the Terminal UI
//   - replay: Replay the last execution
//   - branch: Create and execute a branch
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Ranganaths/minion/debug/api"
	"github.com/Ranganaths/minion/debug/recorder"
	"github.com/Ranganaths/minion/debug/snapshot"
	"github.com/Ranganaths/minion/debug/studio/tui"
	"github.com/Ranganaths/minion/debug/timetravel"
)

func main() {
	mode := "record"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	// Create in-memory snapshot store
	store := snapshot.NewMemorySnapshotStore()
	defer store.Close()

	switch mode {
	case "record":
		runRecordExample(store)
	case "api":
		runAPIServer(store)
	case "tui":
		runTUI(store)
	case "replay":
		runReplayExample(store)
	case "branch":
		runBranchExample(store)
	case "demo":
		runFullDemo(store)
	default:
		fmt.Println("Usage: go run main.go [record|api|tui|replay|branch|demo]")
	}
}

// runRecordExample demonstrates recording execution snapshots
func runRecordExample(store snapshot.SnapshotStore) {
	fmt.Println("=== Recording Example Execution ===")
	ctx := context.Background()

	// Create recorder
	rec := recorder.NewExecutionRecorder(store, recorder.DefaultRecorderConfig())

	// Create hooks
	hooks := recorder.NewFrameworkHooks(rec)

	// Simulate an agent execution
	agentID := "example-agent"
	agentHooks := hooks.ForAgent(agentID)

	// Start execution
	fmt.Println("Starting agent execution...")
	agentHooks.OnExecutionStart(ctx, map[string]any{
		"query": "What is the weather in San Francisco?",
	})

	// Simulate some steps
	fmt.Println("Recording steps...")

	// Step 1: Planning
	agentHooks.OnPlan(ctx, map[string]any{
		"plan": "1. Call weather API\n2. Format response",
	})
	time.Sleep(100 * time.Millisecond)

	// Step 2: Tool call
	toolHooks := hooks.ForTool("weather_api")
	toolHooks.OnStart(ctx, map[string]any{
		"city": "San Francisco",
	})
	time.Sleep(200 * time.Millisecond)
	toolHooks.OnEnd(ctx, map[string]any{
		"temperature": 65,
		"condition":   "sunny",
	}, nil)

	// Step 3: LLM call
	llmHooks := hooks.ForLLM("openai", "gpt-4")
	llmHooks.OnStart(ctx, "Format the weather data for the user")
	time.Sleep(300 * time.Millisecond)
	llmHooks.OnEnd(ctx, "The weather in San Francisco is sunny with a temperature of 65°F.", 50, 30, 0.001, nil)

	// Step 4: Record a decision point
	decisionHooks := hooks.ForDecisions()
	decisionHooks.OnDecision(ctx, "response_format", []string{"brief", "detailed", "emoji"}, "brief")

	// Complete execution
	agentHooks.OnExecutionEnd(ctx, map[string]any{
		"response": "The weather in San Francisco is sunny with a temperature of 65°F.",
	}, nil)

	fmt.Println("Execution recorded!")
	fmt.Printf("Execution ID: %s\n", rec.GetExecutionID())

	// Print summary
	summary, err := store.GetExecutionSummary(ctx, rec.GetExecutionID())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Total steps: %d\n", summary.TotalSteps)
	fmt.Printf("Duration: %v\n", summary.Duration)
	fmt.Printf("Status: %s\n", summary.Status)
}

// runAPIServer starts the Debug API server
func runAPIServer(store snapshot.SnapshotStore) {
	fmt.Println("=== Starting Debug API Server ===")

	// First record some data
	recordSampleData(store)

	config := api.DefaultServerConfig()
	config.Addr = ":8080"

	server := api.NewDebugServer(store, config)

	fmt.Println("Debug API server running on http://localhost:8080")
	fmt.Println("Endpoints:")
	fmt.Println("  GET  /health              - Health check")
	fmt.Println("  GET  /stats               - Store statistics")
	fmt.Println("  GET  /api/v1/executions   - List executions")
	fmt.Println("  GET  /api/v1/timeline/:id - Get timeline")
	fmt.Println("  POST /api/v1/step         - Step through timeline")
	fmt.Println("  POST /api/v1/replay       - Replay execution")
	fmt.Println("  POST /api/v1/branches     - Create branch")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")

	if err := server.Start(); err != nil {
		log.Printf("Server stopped: %v", err)
	}
}

// runTUI launches the Terminal UI
func runTUI(store snapshot.SnapshotStore) {
	fmt.Println("=== Launching Debug Studio TUI ===")

	// First record some data
	recordSampleData(store)

	fmt.Println("Launching Terminal UI...")
	if err := tui.Run(store); err != nil {
		log.Fatal(err)
	}
}

// runReplayExample demonstrates replaying an execution
func runReplayExample(store snapshot.SnapshotStore) {
	fmt.Println("=== Replay Example ===")
	ctx := context.Background()

	// First record some data
	executionID := recordSampleData(store)

	// Create timeline
	timeline, err := timetravel.NewExecutionTimeline(ctx, store, executionID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Loaded timeline with %d snapshots\n", timeline.Length())
	fmt.Printf("Duration: %v\n", timeline.Duration())

	// Navigate timeline
	fmt.Println("\n--- Timeline Navigation ---")

	// Go to start
	snap := timeline.First()
	fmt.Printf("First: seq=%d type=%s\n", snap.SequenceNum, snap.CheckpointType)

	// Step forward
	for i := 0; i < 3; i++ {
		snap = timeline.StepForward()
		if snap != nil {
			fmt.Printf("Step %d: seq=%d type=%s\n", i+1, snap.SequenceNum, snap.CheckpointType)
		}
	}

	// Jump to error (if any)
	if errSnap := timeline.JumpToNextError(); errSnap != nil {
		fmt.Printf("Found error at seq=%d: %s\n", errSnap.SequenceNum, errSnap.Error.Message)
	}

	// Create replay engine
	replayEngine := timetravel.NewReplayEngine(store, timeline)

	// Simulate replay from sequence 2
	fmt.Println("\n--- Simulated Replay ---")
	result, err := replayEngine.SimulateFrom(ctx, 2, &timetravel.ReplayOptions{
		MaxSteps: 5,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Replay completed:\n")
	fmt.Printf("  Steps replayed: %d\n", result.StepsReplayed)
	fmt.Printf("  Duration: %v\n", result.Duration)
	fmt.Printf("  Success: %v\n", result.Success)
}

// runBranchExample demonstrates branching and what-if analysis
func runBranchExample(store snapshot.SnapshotStore) {
	fmt.Println("=== Branching Example ===")
	ctx := context.Background()

	// First record some data
	executionID := recordSampleData(store)

	// Create branching engine
	branching := timetravel.NewBranchingEngine(store)

	// Create a branch with a modification
	fmt.Println("\n--- Creating Branch ---")
	branch, err := branching.CreateBranch(ctx, executionID, 3, &timetravel.CreateBranchOptions{
		Name:        "alternative-response",
		Description: "What if we used a different response format?",
		Modification: &timetravel.Modification{
			Type:  "input",
			Path:  "response_format",
			Value: "detailed",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Created branch: %s\n", branch.ID)
	fmt.Printf("  Name: %s\n", branch.Name)
	fmt.Printf("  Branch point: sequence %d\n", branch.BranchPointSeq)

	// Execute the branch
	fmt.Println("\n--- Executing Branch ---")
	result, err := branching.ExecuteBranch(ctx, branch.ID, &timetravel.ReplayOptions{
		Mode: timetravel.ReplayModeSimulate,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Branch executed:\n")
	fmt.Printf("  Steps: %d\n", result.StepsReplayed)
	fmt.Printf("  Duration: %v\n", result.Duration)

	// Compare with parent
	fmt.Println("\n--- Comparing with Parent ---")
	comparison, err := branching.CompareWithParent(ctx, branch.ID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Comparison:\n")
	fmt.Printf("  Parent duration: %v\n", comparison.Branch1Duration)
	fmt.Printf("  Branch duration: %v\n", comparison.Branch2Duration)
	fmt.Printf("  Duration delta: %v\n", comparison.DurationDelta)
	fmt.Printf("  Steps delta: %d\n", comparison.StepsDelta)
	fmt.Printf("  Outcome same: %v\n", comparison.OutcomeSame)
}

// runFullDemo runs a complete demonstration
func runFullDemo(store snapshot.SnapshotStore) {
	fmt.Println("=== Full Debug Demo ===")
	ctx := context.Background()

	// Record multiple executions
	fmt.Println("\n1. Recording executions...")
	exec1 := recordSampleData(store)
	exec2 := recordSampleDataWithError(store)
	exec3 := recordMultiAgentData(store)

	fmt.Printf("   Recorded: %s (success)\n", exec1[:8])
	fmt.Printf("   Recorded: %s (with error)\n", exec2[:8])
	fmt.Printf("   Recorded: %s (multi-agent)\n", exec3[:8])

	// List all executions
	fmt.Println("\n2. Listing executions...")
	executions, _ := store.ListExecutions(ctx, 10, 0)
	for _, exec := range executions {
		fmt.Printf("   - %s: %s (%d steps, %v)\n",
			exec.ExecutionID[:8], exec.Status, exec.TotalSteps, exec.Duration.Round(time.Millisecond))
	}

	// Explore timeline of successful execution
	fmt.Println("\n3. Exploring timeline...")
	timeline, _ := timetravel.NewExecutionTimeline(ctx, store, exec1)
	fmt.Printf("   Total checkpoints: %d\n", timeline.Length())

	// Count checkpoint types
	counts := timeline.CountCheckpoints()
	for cpType, count := range counts {
		fmt.Printf("   - %s: %d\n", cpType, count)
	}

	// Find slowest operations
	fmt.Println("\n4. Finding slowest operations...")
	slowest := timeline.FindSlowestOperations(3)
	for i, snap := range slowest {
		if snap.Action != nil {
			fmt.Printf("   %d. %s (%dms)\n", i+1, snap.Action.Name, snap.Action.DurationMs)
		}
	}

	// Reconstruct state at a point
	fmt.Println("\n5. Reconstructing state...")
	reconstructor := timetravel.NewStateReconstructor(timeline)
	state, _ := reconstructor.ReconstructAt(3)
	if state != nil {
		fmt.Printf("   At sequence 3:\n")
		fmt.Printf("   - Checkpoint: %s\n", state.Snapshot.CheckpointType)
		fmt.Printf("   - Actions so far: %d\n", len(state.PreviousActions))
	}

	// Create what-if analysis
	fmt.Println("\n6. What-if analysis...")
	branching := timetravel.NewBranchingEngine(store)
	comparison, err := branching.WhatIf(ctx, exec1, 3, &timetravel.Modification{
		Type:  "input",
		Value: "modified_input",
	})
	if err == nil && comparison != nil {
		fmt.Printf("   Original duration: %v\n", comparison.Branch1Duration)
		fmt.Printf("   Modified duration: %v\n", comparison.Branch2Duration)
	}

	// Store statistics
	fmt.Println("\n7. Store statistics...")
	stats, _ := store.Stats(ctx)
	fmt.Printf("   Total snapshots: %d\n", stats.TotalSnapshots)
	fmt.Printf("   Total executions: %d\n", stats.TotalExecutions)

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("\nNext steps:")
	fmt.Println("  - Run 'go run main.go api' to start the API server")
	fmt.Println("  - Run 'go run main.go tui' to launch the Terminal UI")
}

// Helper functions to record sample data

func recordSampleData(store snapshot.SnapshotStore) string {
	ctx := context.Background()
	rec := recorder.NewExecutionRecorder(store, recorder.DefaultRecorderConfig())
	hooks := recorder.NewFrameworkHooks(rec)
	agentHooks := hooks.ForAgent("sample-agent")

	agentHooks.OnExecutionStart(ctx, map[string]any{"query": "Sample query"})

	hooks.ForTool("sample_tool").OnStart(ctx, "input")
	time.Sleep(50 * time.Millisecond)
	hooks.ForTool("sample_tool").OnEnd(ctx, "output", nil)

	hooks.ForLLM("openai", "gpt-4").OnStart(ctx, "prompt")
	time.Sleep(100 * time.Millisecond)
	hooks.ForLLM("openai", "gpt-4").OnEnd(ctx, "response", 10, 20, 0.001, nil)

	hooks.ForDecisions().OnDecision(ctx, "choice", []string{"a", "b"}, "a")

	agentHooks.OnExecutionEnd(ctx, "final output", nil)

	return rec.GetExecutionID()
}

func recordSampleDataWithError(store snapshot.SnapshotStore) string {
	ctx := context.Background()
	rec := recorder.NewExecutionRecorder(store, recorder.DefaultRecorderConfig())
	hooks := recorder.NewFrameworkHooks(rec)
	agentHooks := hooks.ForAgent("error-agent")

	agentHooks.OnExecutionStart(ctx, map[string]any{"query": "Failing query"})

	hooks.ForTool("failing_tool").OnStart(ctx, "bad input")
	time.Sleep(50 * time.Millisecond)
	hooks.ForTool("failing_tool").OnEnd(ctx, nil, fmt.Errorf("tool execution failed"))

	hooks.ForErrors().OnError(ctx, fmt.Errorf("critical error occurred"), map[string]any{
		"component": "failing_tool",
	})

	agentHooks.OnExecutionEnd(ctx, nil, fmt.Errorf("execution failed"))

	return rec.GetExecutionID()
}

func recordMultiAgentData(store snapshot.SnapshotStore) string {
	ctx := context.Background()
	rec := recorder.NewExecutionRecorder(store, recorder.DefaultRecorderConfig())
	hooks := recorder.NewFrameworkHooks(rec)

	// Orchestrator
	orchHooks := hooks.ForAgent("orchestrator")
	orchHooks.OnExecutionStart(ctx, map[string]any{"task": "Multi-agent task"})

	// Create and assign tasks
	taskHooks := hooks.ForTask("task-1")
	taskHooks.OnCreated(ctx, &recorder.TaskState{
		ID:     "task-1",
		Name:   "Research",
		Status: "pending",
	})
	taskHooks.OnAssigned(ctx, &recorder.TaskState{
		ID:     "task-1",
		Name:   "Research",
		Status: "assigned",
	}, "researcher-agent")

	// Worker execution
	workerHooks := hooks.ForAgent("researcher-agent")
	workerHooks.OnExecutionStart(ctx, "research topic")
	time.Sleep(100 * time.Millisecond)
	workerHooks.OnExecutionEnd(ctx, "research results", nil)

	// Complete task
	taskHooks.OnCompleted(ctx, &recorder.TaskState{
		ID:     "task-1",
		Name:   "Research",
		Status: "completed",
	}, "research results")

	// Message passing
	msgHooks := hooks.ForMessages()
	msgHooks.OnSent(ctx, "orchestrator", "researcher-agent", "Please research X")
	msgHooks.OnReceived(ctx, "researcher-agent", "orchestrator", "Research complete")

	orchHooks.OnExecutionEnd(ctx, "multi-agent task complete", nil)

	return rec.GetExecutionID()
}
