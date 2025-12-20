package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Ranganaths/minion/core/multiagent"
	"github.com/Ranganaths/minion/llm"
)

func main() {
	ctx := context.Background()

	// Example 1: Direct TupleLeap usage
	fmt.Println("=== Example 1: Direct TupleLeap Usage ===")
	fmt.Println()
	directUsage(ctx)

	fmt.Println()
	fmt.Println("=== Example 2: TupleLeap with Minion Multi-Agent System ===")
	fmt.Println()
	withMinionCoordinator(ctx)
}

func directUsage(ctx context.Context) {
	// Get API key from environment
	apiKey := os.Getenv("TUPLELEAP_API_KEY")
	if apiKey == "" {
		log.Println("TUPLELEAP_API_KEY not set, skipping direct usage example")
		return
	}

	// Create TupleLeap provider
	provider := llm.NewTupleLeap(apiKey)

	// Example 1: Simple completion
	fmt.Println("1. Simple Completion:")
	resp, err := provider.GenerateCompletion(ctx, &llm.CompletionRequest{
		SystemPrompt: "You are a helpful AI assistant.",
		UserPrompt:   "What are the benefits of using Go for backend development?",
		Temperature:  0.7,
		MaxTokens:    200,
		Model:        "tupleleap-default",
	})

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Response: %s\n", resp.Text)
		fmt.Printf("Tokens: %d\n", resp.TokensUsed)
		fmt.Printf("Model: %s\n\n", resp.Model)
	}

	// Example 2: Chat conversation
	fmt.Println("2. Chat Conversation:")
	chatResp, err := provider.GenerateChat(ctx, &llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are an expert software architect."},
			{Role: "user", Content: "What is microservices architecture?"},
			{Role: "assistant", Content: "Microservices architecture is a design approach where..."},
			{Role: "user", Content: "What are its main advantages?"},
		},
		Temperature: 0.7,
		MaxTokens:   300,
		Model:       "tupleleap-default",
	})

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Response: %s\n", chatResp.Message.Content)
		fmt.Printf("Tokens: %d\n", chatResp.TokensUsed)
	}
}

// TupleLeapAdapter adapts the TupleLeap provider to the multiagent interface
type TupleLeapAdapter struct {
	provider *llm.TupleLeapProvider
}

func (l *TupleLeapAdapter) GenerateCompletion(ctx context.Context, req *multiagent.CompletionRequest) (*multiagent.CompletionResponse, error) {
	resp, err := l.provider.GenerateCompletion(ctx, &llm.CompletionRequest{
		SystemPrompt: req.SystemPrompt,
		UserPrompt:   req.UserPrompt,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		Model:        req.Model,
	})
	if err != nil {
		return nil, err
	}

	return &multiagent.CompletionResponse{
		Text:         resp.Text,
		TokensUsed:   resp.TokensUsed,
		Model:        resp.Model,
		FinishReason: resp.FinishReason,
	}, nil
}

func withMinionCoordinator(ctx context.Context) {
	// Check if API key is set
	apiKey := os.Getenv("TUPLELEAP_API_KEY")
	if apiKey == "" {
		log.Println("TUPLELEAP_API_KEY not set, skipping Minion example")
		return
	}

	// Create TupleLeap adapter for multiagent
	llmProvider := &TupleLeapAdapter{
		provider: llm.NewTupleLeap(apiKey),
	}

	// Create coordinator with default configuration
	coordinator := multiagent.NewCoordinator(llmProvider, nil)

	// Initialize with default workers
	if err := coordinator.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize coordinator: %v", err)
	}

	fmt.Println("✅ Multi-agent system initialized with TupleLeap")
	fmt.Printf("   Workers registered: %d\n\n", len(coordinator.GetWorkers()))

	// List registered workers
	fmt.Println("📋 Registered Workers:")
	for _, worker := range coordinator.GetWorkers() {
		fmt.Printf("   - Agent %s (Role: %s)\n", worker.AgentID[:8], worker.Role)
		fmt.Printf("     Capabilities: %v\n", worker.Capabilities)
	}
	fmt.Println()

	// Task 1: Code Review using TupleLeap
	fmt.Println("🚀 Task 1: Code Review")
	codeReviewTask := &multiagent.TaskRequest{
		Name:        "Review Go Code",
		Description: "Review the provided Go code snippet for best practices",
		Type:        "code_review",
		Priority:    multiagent.PriorityHigh,
		Input: map[string]interface{}{
			"code": `func Add(a, b int) int {
    return a + b
}`,
			"language": "go",
			"focus":    []string{"best practices", "error handling", "documentation"},
		},
	}

	fmt.Println("   Executing code review task...")
	result, err := coordinator.ExecuteTask(ctx, codeReviewTask)
	if err != nil {
		log.Printf("   ❌ Task failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Task completed: %s\n", result.Status)
		if output, ok := result.Output.(map[string]interface{}); ok {
			if text, exists := output["text"]; exists {
				textStr := fmt.Sprintf("%v", text)
				if len(textStr) > 300 {
					textStr = textStr[:300] + "..."
				}
				fmt.Printf("   Response: %s\n", textStr)
			}
		}
	}
	fmt.Println()

	// Task 2: Documentation Generation
	fmt.Println("🚀 Task 2: Documentation Generation")
	docTask := &multiagent.TaskRequest{
		Name:        "Generate README",
		Description: "Write a brief README for a REST API project in Go",
		Type:        "content_generation",
		Priority:    multiagent.PriorityNormal,
		Input: map[string]interface{}{
			"type":     "readme",
			"language": "go",
			"project":  "REST API",
			"sections": []string{"overview", "installation", "usage", "api endpoints"},
		},
	}

	fmt.Println("   Executing documentation task...")
	result, err = coordinator.ExecuteTask(ctx, docTask)
	if err != nil {
		log.Printf("   ❌ Task failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Task completed: %s\n", result.Status)
		if output, ok := result.Output.(map[string]interface{}); ok {
			if text, exists := output["text"]; exists {
				textStr := fmt.Sprintf("%v", text)
				if len(textStr) > 300 {
					textStr = textStr[:300] + "..."
				}
				fmt.Printf("   Response: %s\n", textStr)
			}
		}
	}
	fmt.Println()

	// Task 3: Research task
	fmt.Println("🚀 Task 3: Research - API Design")
	researchTask := &multiagent.TaskRequest{
		Name:        "Research API Design",
		Description: "Research best practices for RESTful API design",
		Type:        "research",
		Priority:    multiagent.PriorityNormal,
		Input: map[string]interface{}{
			"topic": "RESTful API design best practices",
			"focus": []string{"endpoints", "versioning", "authentication", "error handling"},
		},
	}

	fmt.Println("   Executing research task...")
	result, err = coordinator.ExecuteTask(ctx, researchTask)
	if err != nil {
		log.Printf("   ❌ Task failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Task completed: %s\n", result.Status)
		if output, ok := result.Output.(map[string]interface{}); ok {
			if text, exists := output["text"]; exists {
				textStr := fmt.Sprintf("%v", text)
				if len(textStr) > 300 {
					textStr = textStr[:300] + "..."
				}
				fmt.Printf("   Response: %s\n", textStr)
			}
		}
	}
	fmt.Println()

	// Get monitoring stats
	fmt.Println("📊 System Monitoring:")
	stats, err := coordinator.GetMonitoringStats(ctx)
	if err != nil {
		log.Printf("   Failed to get stats: %v\n", err)
	} else {
		fmt.Printf("   Total Workers: %d (Idle: %d, Busy: %d)\n",
			stats.TotalWorkers, stats.IdleWorkers, stats.BusyWorkers)
		fmt.Printf("   Total Tasks: %d (Completed: %d, Failed: %d, Pending: %d)\n",
			stats.TotalTasks, stats.CompletedTasks, stats.FailedTasks, stats.PendingTasks)
	}
	fmt.Println()

	// Health check
	fmt.Println("🏥 Health Check:")
	health := coordinator.HealthCheck(ctx)
	fmt.Printf("   Overall Status: %s\n", health.Status)
	for component, status := range health.Components {
		fmt.Printf("   - %s: %s\n", component, status)
	}
	fmt.Println()

	// Graceful shutdown
	fmt.Println("🛑 Shutting down...")
	if err := coordinator.Shutdown(ctx); err != nil {
		log.Printf("Error during shutdown: %v\n", err)
	}

	fmt.Println("✅ TupleLeap example completed successfully")
}
