// Example demonstrating multi-agent traceability and execution history.
//
// This example shows how to:
// 1. Create an orchestrator with trace context propagation
// 2. Register workers that extract and use parent trace context
// 3. Track execution history across agent boundaries
// 4. Query and visualize the execution trace tree
//
// Run with: go run main.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Ranganaths/minion/core/multiagent"
	"github.com/Ranganaths/minion/observability"
)

func main() {
	ctx := context.Background()

	// ========================================================================
	// Step 1: Initialize OpenTelemetry Tracing (optional, for external tracing)
	// ========================================================================
	fmt.Println("Initializing tracing...")

	tracingConfig := observability.TracingConfig{
		Enabled:       true,
		ServiceName:   "multiagent-tracing-example",
		Environment:   "development",
		Exporter:      "stdout", // Use stdout for demo, change to "jaeger" for production
		SamplingRatio: 1.0,
	}

	if err := observability.InitGlobalTracer(tracingConfig); err != nil {
		log.Printf("Warning: Failed to initialize tracing: %v", err)
	} else {
		fmt.Println("✓ OpenTelemetry tracing initialized")
	}
	defer observability.GracefulShutdown(5 * time.Second)

	// ========================================================================
	// Step 2: Initialize Execution History
	// ========================================================================
	fmt.Println("Initializing execution history...")

	history := multiagent.NewInMemoryExecutionHistory()
	multiagent.InitExecutionHistory(history)
	fmt.Println("✓ Execution history initialized")

	// ========================================================================
	// Step 3: Create Protocol and LLM Provider
	// ========================================================================
	fmt.Println("Creating protocol and LLM provider...")

	protocol := multiagent.NewInMemoryProtocol(nil)
	llmProvider := &MockLLMProvider{}

	fmt.Println("✓ Protocol and LLM provider created")

	// ========================================================================
	// Step 4: Create Orchestrator
	// ========================================================================
	fmt.Println("Creating orchestrator...")

	orchestratorConfig := multiagent.DefaultOrchestratorConfig()
	orchestratorConfig.TaskTimeout = 30 * time.Second

	orchestrator := multiagent.NewOrchestrator(protocol, llmProvider, orchestratorConfig)
	if err := orchestrator.Start(ctx); err != nil {
		log.Fatalf("Failed to start orchestrator: %v", err)
	}
	fmt.Printf("✓ Orchestrator created: %s\n", orchestrator.GetID())

	// ========================================================================
	// Step 5: Create and Register Workers
	// ========================================================================
	fmt.Println("Registering workers...")

	// Create coder worker
	coderMeta := &multiagent.AgentMetadata{
		AgentID:      "coder-worker-001",
		Role:         multiagent.RoleWorker,
		Capabilities: []string{"code_generation", "debugging"},
		Priority:     10,
		Status:       multiagent.StatusIdle,
	}
	coderHandler := multiagent.NewCoderWorker(llmProvider)
	coderWorker := multiagent.NewWorkerAgent(coderMeta, protocol, coderHandler)
	coderWorker.Start(ctx)
	if err := orchestrator.RegisterWorker(ctx, coderMeta); err != nil {
		log.Fatalf("Failed to register coder worker: %v", err)
	}
	fmt.Printf("✓ Coder worker registered: %s\n", coderMeta.AgentID)

	// Create analyst worker
	analystMeta := &multiagent.AgentMetadata{
		AgentID:      "analyst-worker-001",
		Role:         multiagent.RoleWorker,
		Capabilities: []string{"data_analysis", "research"},
		Priority:     8,
		Status:       multiagent.StatusIdle,
	}
	analystHandler := multiagent.NewAnalystWorker(llmProvider)
	analystWorker := multiagent.NewWorkerAgent(analystMeta, protocol, analystHandler)
	analystWorker.Start(ctx)
	if err := orchestrator.RegisterWorker(ctx, analystMeta); err != nil {
		log.Fatalf("Failed to register analyst worker: %v", err)
	}
	fmt.Printf("✓ Analyst worker registered: %s\n", analystMeta.AgentID)

	// ========================================================================
	// Step 6: Execute Task with Full Traceability
	// ========================================================================
	fmt.Println("\n--- Executing Task with Traceability ---")

	taskReq := &multiagent.TaskRequest{
		Name:        "Analyze and Generate Code",
		Description: "Analyze the market data and generate a simple trading algorithm",
		Type:        "complex_task",
		Priority:    multiagent.PriorityHigh,
		Input: map[string]interface{}{
			"market":   "stocks",
			"strategy": "moving_average",
		},
	}

	result, err := orchestrator.ExecuteTask(ctx, taskReq)
	if err != nil {
		log.Printf("Task execution failed: %v", err)
	} else {
		fmt.Printf("\n✓ Task completed successfully!\n")
		fmt.Printf("  Task ID: %s\n", result.TaskID)
		fmt.Printf("  Status: %s\n", result.Status)
		fmt.Printf("  Completed at: %v\n", result.CompletedAt)
	}

	// ========================================================================
	// Step 7: Query Execution History
	// ========================================================================
	fmt.Println("\n--- Execution History ---")

	// Query all traces
	traces, err := history.QueryTraces(ctx, &multiagent.ExecutionHistoryQuery{
		Limit: 10,
	})
	if err != nil {
		log.Printf("Failed to query traces: %v", err)
	} else {
		fmt.Printf("Total traces: %d\n", len(traces))
		for _, trace := range traces {
			fmt.Printf("\n  Execution ID: %s\n", trace.ExecutionID)
			fmt.Printf("  Root Trace ID: %s\n", trace.RootTraceID)
			fmt.Printf("  Orchestrator: %s\n", trace.OrchestratorID)
			fmt.Printf("  Status: %s\n", trace.Status)
			fmt.Printf("  Duration: %v\n", trace.Duration)
			fmt.Printf("  Events: %d\n", len(trace.Events))
		}
	}

	// Query events for specific agent
	fmt.Println("\n--- Coder Worker Events ---")
	coderEvents, err := history.GetAgentHistory(ctx, "coder-worker-001")
	if err != nil {
		log.Printf("Failed to get coder events: %v", err)
	} else {
		for _, event := range coderEvents {
			fmt.Printf("  [%s] %s - %s (duration: %v)\n",
				event.Type, event.Action, event.TaskID, event.Duration)
			if event.TraceContext != nil {
				fmt.Printf("    Root Trace: %s, Execution: %s\n",
					event.TraceContext.RootTraceID, event.TraceContext.ExecutionID)
			}
		}
	}

	// ========================================================================
	// Step 8: Get Execution Metrics
	// ========================================================================
	fmt.Println("\n--- Execution Metrics ---")

	for _, trace := range traces {
		metrics, err := history.GetExecutionMetrics(ctx, trace.ExecutionID)
		if err != nil {
			log.Printf("Failed to get metrics: %v", err)
			continue
		}

		if metrics != nil {
			fmt.Printf("Execution: %s\n", metrics.ExecutionID)
			fmt.Printf("  Total Duration: %v\n", metrics.TotalDuration)
			fmt.Printf("  Task Count: %d\n", metrics.TaskCount)
			fmt.Printf("  Subtask Count: %d\n", metrics.SubtaskCount)
			fmt.Printf("  Completed: %d, Failed: %d\n", metrics.CompletedTasks, metrics.FailedTasks)
			fmt.Printf("  Workers Used: %d\n", metrics.WorkerCount)
			fmt.Printf("  LLM Calls: %d (total duration: %v)\n", metrics.LLMCalls, metrics.TotalLLMDuration)
			fmt.Printf("  Messages: %d\n", metrics.MessageCount)
		}
	}

	// ========================================================================
	// Step 9: Build Task Tree
	// ========================================================================
	fmt.Println("\n--- Task Tree ---")

	for _, trace := range traces {
		tree, err := history.BuildTaskTree(ctx, trace.ExecutionID)
		if err != nil {
			log.Printf("Failed to build tree: %v", err)
			continue
		}

		if tree != nil {
			printTaskTree(tree, 0)
		}
	}

	// ========================================================================
	// Cleanup
	// ========================================================================
	fmt.Println("\n--- Cleanup ---")
	coderWorker.Stop(ctx)
	analystWorker.Stop(ctx)
	fmt.Println("✓ Workers stopped")
	fmt.Println("✓ Example complete")
}

// printTaskTree recursively prints the task tree
func printTaskTree(node *multiagent.TaskTreeNode, depth int) {
	if node == nil || node.Task == nil {
		return
	}

	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}

	fmt.Printf("%s└─ %s (%s)\n", indent, node.Task.Name, node.Task.Status)
	if node.WorkerID != "" {
		fmt.Printf("%s   Worker: %s\n", indent, node.WorkerID)
	}
	if node.Duration > 0 {
		fmt.Printf("%s   Duration: %v\n", indent, node.Duration)
	}

	for _, child := range node.Children {
		printTaskTree(child, depth+1)
	}
}

// ============================================================================
// Mock LLM Provider for demonstration
// ============================================================================

type MockLLMProvider struct{}

func (p *MockLLMProvider) GenerateCompletion(ctx context.Context, req *multiagent.CompletionRequest) (*multiagent.CompletionResponse, error) {
	// Simulate LLM processing time
	time.Sleep(50 * time.Millisecond)

	// Generate mock planning response
	if isPlanning(req.SystemPrompt) {
		return &multiagent.CompletionResponse{
			Text: `{
				"subtasks": [
					{
						"name": "Analyze Market Data",
						"description": "Analyze the provided market data to identify trends",
						"assigned_to": "data_analysis",
						"dependencies": [],
						"priority": 8,
						"input": "Analyze stock market trends"
					},
					{
						"name": "Generate Trading Algorithm",
						"description": "Generate a moving average trading algorithm based on the analysis",
						"assigned_to": "code_generation",
						"dependencies": ["Analyze Market Data"],
						"priority": 9,
						"input": "Generate moving average trading algorithm"
					}
				]
			}`,
			TokensUsed:   250,
			Model:        "mock-gpt-4",
			FinishReason: "stop",
		}, nil
	}

	// Generate mock worker response
	return &multiagent.CompletionResponse{
		Text:         "Task completed successfully with mock results.",
		TokensUsed:   100,
		Model:        "mock-gpt-4",
		FinishReason: "stop",
	}, nil
}

func isPlanning(prompt string) bool {
	return len(prompt) > 100 && (containsStr(prompt, "orchestrator") || containsStr(prompt, "decompose"))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
