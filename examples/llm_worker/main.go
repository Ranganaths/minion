package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Ranganaths/minion/core/multiagent"
	"github.com/Ranganaths/minion/llm"
)

// LLMAdapter adapts the minion LLM provider to the multiagent interface
type LLMAdapter struct {
	provider llm.Provider
}

func (l *LLMAdapter) GenerateCompletion(ctx context.Context, req *multiagent.CompletionRequest) (*multiagent.CompletionResponse, error) {
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

func main() {
	ctx := context.Background()

	// Get API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	// Create LLM adapter
	llmProvider := &LLMAdapter{
		provider: llm.NewOpenAI(apiKey),
	}

	fmt.Println("🚀 LLM Worker Multi-Agent Example")
	fmt.Println("==================================")
	fmt.Println()

	// Create coordinator with default configuration
	coordinator := multiagent.NewCoordinator(llmProvider, nil)

	// Initialize with default workers
	if err := coordinator.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize coordinator: %v", err)
	}

	fmt.Println("✅ Multi-agent system initialized")
	fmt.Printf("   Workers registered: %d\n\n", len(coordinator.GetWorkers()))

	// List registered workers
	fmt.Println("📋 Registered Workers:")
	for _, worker := range coordinator.GetWorkers() {
		fmt.Printf("   - Agent %s (Role: %s)\n", worker.AgentID[:8], worker.Role)
		fmt.Printf("     Capabilities: %v\n", worker.Capabilities)
	}
	fmt.Println()

	// Task 1: Code generation using GPT-4
	fmt.Println("🚀 Task 1: Code Generation (GPT-4)")
	codeTask := &multiagent.TaskRequest{
		Name:        "Generate REST API",
		Description: "Create a simple REST API in Go for user management",
		Type:        "code_generation",
		Priority:    multiagent.PriorityHigh,
		Input: map[string]interface{}{
			"language":  "go",
			"framework": "net/http",
			"endpoints": []string{"/users", "/users/:id"},
			"features":  []string{"create", "read", "update", "delete"},
		},
	}

	fmt.Println("   Executing code generation task...")
	result, err := coordinator.ExecuteTask(ctx, codeTask)
	if err != nil {
		log.Printf("   ❌ Task failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Task completed: %s\n", result.Status)
		if output, ok := result.Output.(map[string]interface{}); ok {
			if text, exists := output["text"]; exists {
				// Print first 200 chars of response
				textStr := fmt.Sprintf("%v", text)
				if len(textStr) > 200 {
					textStr = textStr[:200] + "..."
				}
				fmt.Printf("   Response: %s\n", textStr)
			}
		}
	}
	fmt.Println()

	// Task 2: Data analysis
	fmt.Println("🚀 Task 2: Data Analysis")
	analysisTask := &multiagent.TaskRequest{
		Name:        "Analyze Sales Data",
		Description: "Analyze quarterly sales data and identify trends",
		Type:        "data_analysis",
		Priority:    multiagent.PriorityNormal,
		Input: map[string]interface{}{
			"data": map[string]interface{}{
				"Q1": 150000,
				"Q2": 180000,
				"Q3": 165000,
				"Q4": 220000,
			},
			"metrics": []string{"growth_rate", "trend", "forecast"},
		},
	}

	fmt.Println("   Executing data analysis task...")
	result, err = coordinator.ExecuteTask(ctx, analysisTask)
	if err != nil {
		log.Printf("   ❌ Task failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Task completed: %s\n", result.Status)
		if output, ok := result.Output.(map[string]interface{}); ok {
			if text, exists := output["text"]; exists {
				// Print first 200 chars of response
				textStr := fmt.Sprintf("%v", text)
				if len(textStr) > 200 {
					textStr = textStr[:200] + "..."
				}
				fmt.Printf("   Response: %s\n", textStr)
			}
		}
	}
	fmt.Println()

	// Task 3: Research task
	fmt.Println("🚀 Task 3: Research Task")
	researchTask := &multiagent.TaskRequest{
		Name:        "Research LLM Trends",
		Description: "Research the latest trends in Large Language Models",
		Type:        "research",
		Priority:    multiagent.PriorityNormal,
		Input: map[string]interface{}{
			"topic": "Large Language Model architectures",
			"focus": []string{"attention mechanisms", "training techniques", "efficiency"},
			"depth": "comprehensive",
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
				// Print first 200 chars of response
				textStr := fmt.Sprintf("%v", text)
				if len(textStr) > 200 {
					textStr = textStr[:200] + "..."
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

		if stats.ProtocolMetrics != nil {
			fmt.Printf("   Messages: Sent: %d, Received: %d, Failed: %d\n",
				stats.ProtocolMetrics.TotalMessagesSent,
				stats.ProtocolMetrics.TotalMessagesReceived,
				stats.ProtocolMetrics.TotalMessagesFailed)
		}
	}
	fmt.Println()

	// Health check
	fmt.Println("🏥 Health Check:")
	health := coordinator.HealthCheck(ctx)
	fmt.Printf("   Overall Status: %s\n", health.Status)
	fmt.Printf("   Components:\n")
	for component, status := range health.Components {
		fmt.Printf("     - %s: %s\n", component, status)
	}
	if len(health.Errors) > 0 {
		fmt.Printf("   Errors:\n")
		for _, err := range health.Errors {
			fmt.Printf("     - %s\n", err)
		}
	}
	fmt.Println()

	// Graceful shutdown
	fmt.Println("🛑 Shutting down...")
	if err := coordinator.Shutdown(ctx); err != nil {
		log.Printf("Error during shutdown: %v\n", err)
	}

	fmt.Println("✅ LLM worker example completed successfully")
}
