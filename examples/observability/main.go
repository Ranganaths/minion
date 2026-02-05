// Example demonstrating OpenTelemetry tracing and Prometheus metrics integration.
//
// This example shows how to:
// 1. Initialize OpenTelemetry tracing with Jaeger export
// 2. Set up Prometheus metrics with HTTP endpoint
// 3. Execute traced agent runs with full observability
// 4. View traces in Jaeger UI and metrics in Prometheus
//
// Prerequisites:
//   - Jaeger: docker run -d -p 16686:16686 -p 14268:14268 jaegertracing/all-in-one:latest
//   - Prometheus: Configure to scrape localhost:9090/metrics
//
// Run with: go run otel_example.go
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

	"github.com/Ranganaths/minion/core"
	"github.com/Ranganaths/minion/llm"
	"github.com/Ranganaths/minion/metrics"
	"github.com/Ranganaths/minion/models"
	"github.com/Ranganaths/minion/observability"
	"github.com/Ranganaths/minion/storage"
)

func main() {
	ctx := context.Background()

	// ========================================================================
	// Step 1: Initialize OpenTelemetry Tracing
	// ========================================================================
	fmt.Println("Initializing OpenTelemetry tracing...")

	tracingConfig := observability.TracingConfig{
		Enabled:       true,
		ServiceName:   "minion-tracing-example",
		Environment:   "development",
		Exporter:      "jaeger", // Options: "jaeger", "otlp", "stdout"
		JaegerURL:     "http://localhost:14268/api/traces",
		SamplingRatio: 1.0, // Sample all traces in development
	}

	if err := observability.InitGlobalTracer(tracingConfig); err != nil {
		log.Printf("Warning: Failed to initialize tracing (Jaeger may not be running): %v", err)
		// Continue without tracing for demo purposes
		tracingConfig.Enabled = false
		observability.InitGlobalTracer(tracingConfig)
	} else {
		fmt.Println("✓ OpenTelemetry tracing initialized")
		fmt.Println("  View traces at: http://localhost:16686")
	}

	// ========================================================================
	// Step 2: Initialize Prometheus Metrics
	// ========================================================================
	fmt.Println("\nInitializing Prometheus metrics...")

	promConfig := metrics.DefaultPrometheusConfig()
	promConfig.Namespace = "minion"
	promConfig.EnableGoCollector = true
	promConfig.EnableProcessCollector = true

	promMetrics := metrics.InitPrometheusMetrics(promConfig)

	// Start metrics HTTP server
	metricsServer := &http.Server{
		Addr:    ":9090",
		Handler: promMetrics.Handler(),
	}

	go func() {
		fmt.Println("✓ Prometheus metrics server started")
		fmt.Println("  View metrics at: http://localhost:9090/metrics")
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// ========================================================================
	// Step 3: Create Framework with Observability
	// ========================================================================
	fmt.Println("\nCreating agent framework...")

	// Create a mock LLM provider for demonstration
	llmProvider := &MockLLMProvider{}

	framework := core.NewFramework(
		core.WithStorage(storage.NewInMemory()),
		core.WithLLMProvider(llmProvider),
	)
	defer framework.Close()

	fmt.Println("✓ Framework created with observability")

	// ========================================================================
	// Step 4: Create an Agent
	// ========================================================================
	fmt.Println("\nCreating agent...")

	agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:         "Traced Demo Agent",
		Description:  "An agent with full tracing and metrics",
		BehaviorType: "default",
		Config: models.AgentConfig{
			LLMProvider: "mock",
			LLMModel:    "demo-model",
			Temperature: 0.7,
			MaxTokens:   500,
		},
		Capabilities: []string{"tracing_demo"},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Activate the agent
	activeStatus := models.StatusActive
	agent, _ = framework.UpdateAgent(ctx, agent.ID, &models.UpdateAgentRequest{
		Status: &activeStatus,
	})

	fmt.Printf("✓ Agent created: %s (ID: %s)\n", agent.Name, agent.ID)

	// ========================================================================
	// Step 5: Execute Agent with Tracing
	// ========================================================================
	fmt.Println("\n--- Executing Traced Agent Runs ---")

	queries := []string{
		"What is the meaning of life?",
		"Explain quantum computing in simple terms.",
		"Write a haiku about programming.",
	}

	for i, query := range queries {
		fmt.Printf("\nRun %d: %s\n", i+1, query)

		startTime := time.Now()
		output, err := framework.Execute(ctx, agent.ID, &models.Input{
			Raw:  query,
			Type: "text",
		})
		duration := time.Since(startTime)

		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else {
			// Truncate output for display
			result := fmt.Sprintf("%v", output.Result)
			if len(result) > 100 {
				result = result[:100] + "..."
			}
			fmt.Printf("  Output: %s\n", result)
			fmt.Printf("  Duration: %v\n", duration)

			// Show trace ID from output metadata
			if traceID, ok := output.Metadata["trace_id"].(string); ok && traceID != "" {
				fmt.Printf("  Trace ID: %s\n", traceID)
				fmt.Printf("  View in Jaeger: http://localhost:16686/trace/%s\n", traceID)
			}
		}
	}

	// ========================================================================
	// Step 6: Display Metrics
	// ========================================================================
	fmt.Println("\n--- Current Metrics ---")
	fmt.Println("Query Prometheus at http://localhost:9090/metrics")
	fmt.Println("\nKey metrics to look for:")
	fmt.Println("  - minion_llm_calls_total")
	fmt.Println("  - minion_llm_tokens_total")
	fmt.Println("  - minion_llm_call_duration_seconds")
	fmt.Println("  - minion_agent_executions_total")

	// ========================================================================
	// Step 7: Wait for Shutdown
	// ========================================================================
	fmt.Println("\n--- Server Running ---")
	fmt.Println("Services available:")
	fmt.Println("  - Prometheus metrics: http://localhost:9090/metrics")
	fmt.Println("  - Jaeger UI: http://localhost:16686")
	fmt.Println("\nPress Ctrl+C to exit...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Graceful shutdown
	fmt.Println("\nShutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Flush traces before shutdown
	if err := observability.GracefulShutdown(10 * time.Second); err != nil {
		log.Printf("Tracer shutdown error: %v", err)
	}

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Metrics server shutdown error: %v", err)
	}

	fmt.Println("✓ Shutdown complete")
}

// ============================================================================
// Mock LLM Provider for demonstration
// ============================================================================

type MockLLMProvider struct{}

func (p *MockLLMProvider) Name() string {
	return "mock"
}

func (p *MockLLMProvider) GenerateCompletion(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	// Simulate processing time
	time.Sleep(100 * time.Millisecond)

	// Generate mock response
	responses := map[string]string{
		"What is the meaning of life?":               "The meaning of life is a philosophical question. Many believe it's about finding purpose, happiness, and connection with others.",
		"Explain quantum computing in simple terms.": "Quantum computing uses quantum bits (qubits) that can be both 0 and 1 simultaneously, allowing for parallel processing of complex problems.",
		"Write a haiku about programming.":           "Code flows like water\nBugs hide in logic's shadow\nDebug brings the light",
	}

	response := "This is a mock response for demonstration purposes."
	for key, value := range responses {
		if stringContains(req.UserPrompt, key) {
			response = value
			break
		}
	}

	return &llm.CompletionResponse{
		Text:         response,
		TokensUsed:   150,
		Model:        "mock-model",
		FinishReason: "stop",
	}, nil
}

func (p *MockLLMProvider) GenerateChat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{
		Message:      llm.Message{Role: "assistant", Content: "Mock chat response"},
		TokensUsed:   50,
		Model:        "mock-model",
		FinishReason: "stop",
	}, nil
}

func (p *MockLLMProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && stringContainsHelper(s, substr))
}

func stringContainsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
