// Example demonstrating the tracing and observability system for agents.
//
// This example shows how to:
// 1. Create a trace store (in-memory or PostgreSQL)
// 2. Wrap an agent executor with tracing
// 3. Start the tracing API server
// 4. Execute traced agent runs
// 5. Query and analyze traces
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ranganaths/minion/agents"
	"github.com/Ranganaths/minion/llm"
	"github.com/Ranganaths/minion/tracing"
)

func main() {
	ctx := context.Background()

	// ========================================================================
	// Step 1: Create a trace store
	// ========================================================================
	// For development, use in-memory store
	store := tracing.NewInMemoryTraceStore()

	// For production, use PostgreSQL:
	// store, err := tracing.NewPostgresTraceStore("postgres://user:pass@localhost/minion?sslmode=disable")
	// if err != nil {
	//     log.Fatalf("Failed to create PostgreSQL store: %v", err)
	// }
	// defer store.Close()

	fmt.Println("✓ Trace store initialized")

	// ========================================================================
	// Step 2: Start the Tracing API Server
	// ========================================================================
	apiConfig := tracing.DefaultAPIConfig()
	apiConfig.Addr = ":8081"
	apiServer := tracing.NewTracingAPIServer(store, apiConfig)

	go func() {
		fmt.Printf("✓ Tracing API server starting on %s\n", apiConfig.Addr)
		if err := apiServer.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("API server error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// ========================================================================
	// Step 3: Create a simple mock agent for demonstration
	// ========================================================================
	// In a real application, you would use your actual agent executor
	mockAgent := &MockAgent{}
	mockTools := []agents.Tool{
		&MockCalculatorTool{},
	}

	baseExecutor, err := agents.NewAgentExecutor(agents.AgentExecutorConfig{
		Agent:         mockAgent,
		Tools:         mockTools,
		MaxIterations: 10,
		Verbose:       false,
	})
	if err != nil {
		log.Fatalf("Failed to create executor: %v", err)
	}

	// ========================================================================
	// Step 4: Wrap the executor with tracing
	// ========================================================================
	// Create a traced executor that automatically captures all execution details
	tracedExecutor, err := tracing.NewTracedAgentExecutor(tracing.TracedExecutorConfig{
		Executor:        baseExecutor,
		Store:           store,
		CollectorConfig: tracing.DefaultCollectorConfig(),
		AgentID:         "demo-agent-001",
		AgentName:       "DemoCalculatorAgent",
		Hooks: []tracing.TraceHook{
			// Add console hook for real-time output
			&tracing.ConsoleTraceHook{Verbose: true},
		},
	})
	if err != nil {
		log.Fatalf("Failed to create traced executor: %v", err)
	}

	fmt.Println("✓ Traced executor created")

	// ========================================================================
	// Step 5: Execute some agent runs
	// ========================================================================
	fmt.Println("\n--- Executing Agent Runs ---")

	// Set a session ID for grouping related traces
	tracedExecutor.SetSessionID("demo-session-001")

	// Run 1: Simple calculation
	output, err := tracedExecutor.Run(ctx, "What is 2 + 2?")
	if err != nil {
		log.Printf("Run 1 error: %v", err)
	} else {
		fmt.Printf("Run 1 output: %s\n\n", output)
	}

	// Run 2: Another calculation
	output, err = tracedExecutor.Run(ctx, "Calculate 10 * 5")
	if err != nil {
		log.Printf("Run 2 error: %v", err)
	} else {
		fmt.Printf("Run 2 output: %s\n\n", output)
	}

	// Run 3: Simulate an error
	output, err = tracedExecutor.Run(ctx, "Divide by zero")
	if err != nil {
		log.Printf("Run 3 error (expected): %v", err)
	} else {
		fmt.Printf("Run 3 output: %s\n\n", output)
	}

	// ========================================================================
	// Step 6: Query and analyze traces
	// ========================================================================
	fmt.Println("\n--- Trace Analysis ---")

	// Get all trace summaries
	summaries, err := store.GetTraceSummaries(ctx, 10, 0)
	if err != nil {
		log.Printf("Failed to get summaries: %v", err)
	} else {
		fmt.Printf("Total traces: %d\n", len(summaries))
		for i, s := range summaries {
			fmt.Printf("  %d. [%s] %s - %dms, %d tokens, $%.4f\n",
				i+1, s.Status, s.Input[:min(50, len(s.Input))], s.Duration, s.TotalTokens, s.TotalCost)
		}
	}

	// Query traces with filters
	result, err := store.QueryTraces(ctx, &tracing.TraceQuery{
		Filter: tracing.TraceFilter{
			AgentID: "demo-agent-001",
			Status:  tracing.SpanStatusOK,
		},
		Limit: 10,
	})
	if err != nil {
		log.Printf("Failed to query traces: %v", err)
	} else {
		fmt.Printf("\nSuccessful traces: %d\n", result.TotalCount)
	}

	// Get store statistics
	stats, err := store.Stats(ctx)
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
	} else {
		fmt.Printf("\n--- Store Statistics ---\n")
		fmt.Printf("Total Traces: %d\n", stats.TotalTraces)
		fmt.Printf("Total Spans: %d\n", stats.TotalSpans)
		fmt.Printf("Total Tokens: %d\n", stats.TotalTokens)
		fmt.Printf("Total Cost: $%.4f\n", stats.TotalCost)
		fmt.Printf("Avg Duration: %.2fms\n", stats.AvgDuration)
		fmt.Printf("Avg Tokens/Trace: %.1f\n", stats.AvgTokensPerTrace)
	}

	// ========================================================================
	// Step 7: Demonstrate API endpoints
	// ========================================================================
	fmt.Println("\n--- API Endpoints Available ---")
	fmt.Println("  GET  http://localhost:8081/api/v1/traces/health    - Health check")
	fmt.Println("  GET  http://localhost:8081/api/v1/traces/stats     - Store statistics")
	fmt.Println("  GET  http://localhost:8081/api/v1/traces           - List traces")
	fmt.Println("  GET  http://localhost:8081/api/v1/traces/{id}      - Get trace by ID")
	fmt.Println("  GET  http://localhost:8081/api/v1/traces/{id}/tree - Get trace span tree")
	fmt.Println("  POST http://localhost:8081/api/v1/traces           - Query traces with filters")
	fmt.Println("  GET  http://localhost:8081/api/v1/traces/by-agent/{agent_id}   - Traces by agent")
	fmt.Println("  GET  http://localhost:8081/api/v1/traces/by-session/{session_id} - Traces by session")

	// ========================================================================
	// Step 8: Wait for shutdown signal
	// ========================================================================
	fmt.Println("\nPress Ctrl+C to exit...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Graceful shutdown
	fmt.Println("\nShutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("API server shutdown error: %v", err)
	}

	fmt.Println("✓ Shutdown complete")
}

// ============================================================================
// Mock implementations for demonstration
// ============================================================================

// MockAgent is a simple agent that always uses the calculator tool
type MockAgent struct{}

func (a *MockAgent) Plan(ctx context.Context, input agents.AgentInput) (agents.AgentAction, error) {
	// Simulate thinking time
	time.Sleep(10 * time.Millisecond)

	// If we already have a result, return final answer
	if len(input.IntermediateSteps) > 0 {
		lastStep := input.IntermediateSteps[len(input.IntermediateSteps)-1]
		return agents.AgentAction{
			Finish:      true,
			FinalAnswer: fmt.Sprintf("The result is: %s", lastStep.Observation),
			Log:         "I have the answer from the calculator.",
		}, nil
	}

	// Simulate an error for "divide by zero"
	if input.Input == "Divide by zero" {
		return agents.AgentAction{
			Finish:      true,
			FinalAnswer: "",
			Log:         "Cannot divide by zero!",
		}, fmt.Errorf("cannot divide by zero")
	}

	// Otherwise, use the calculator
	return agents.AgentAction{
		Tool:      "calculator",
		ToolInput: input.Input,
		Log:       fmt.Sprintf("I need to calculate: %s. Using the calculator tool.", input.Input),
	}, nil
}

func (a *MockAgent) InputKeys() []string  { return []string{"input"} }
func (a *MockAgent) OutputKeys() []string { return []string{"output"} }

// MockCalculatorTool simulates a calculator tool
type MockCalculatorTool struct{}

func (t *MockCalculatorTool) Name() string        { return "calculator" }
func (t *MockCalculatorTool) Description() string { return "Performs mathematical calculations" }

func (t *MockCalculatorTool) Call(ctx context.Context, input string) (string, error) {
	// Simulate some processing time
	time.Sleep(5 * time.Millisecond)

	// Return mock results
	switch {
	case contains(input, "2") && contains(input, "2"):
		return "4", nil
	case contains(input, "10") && contains(input, "5"):
		return "50", nil
	default:
		return "42", nil // The answer to everything
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Mock LLM provider for demonstration
func init() {
	// Register a mock provider factory if needed
	_ = llm.ProviderFactory{}
}
