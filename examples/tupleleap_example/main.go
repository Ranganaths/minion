package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/agentql/agentql/pkg/minion/core/multiagent"
	"github.com/agentql/agentql/pkg/minion/llm"
)

func main() {
	ctx := context.Background()

	// Example 1: Direct TupleLeap usage
	fmt.Println("=== Example 1: Direct TupleLeap Usage ===\n")
	directUsage(ctx)

	fmt.Println("\n=== Example 2: TupleLeap with Minion Workers ===\n")
	withMinionWorkers(ctx)
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

func withMinionWorkers(ctx context.Context) {
	// Check if API key is set
	apiKey := os.Getenv("TUPLELEAP_API_KEY")
	if apiKey == "" {
		log.Println("TUPLELEAP_API_KEY not set, skipping Minion example")
		return
	}

	// Create protocol and ledger
	protocol := multiagent.NewInMemoryProtocol(nil)
	ledger := multiagent.NewInMemoryLedger()

	// Create orchestrator
	orchestrator := multiagent.NewOrchestratorAgent(
		"orchestrator-1",
		protocol,
		ledger,
	)

	// Create TupleLeap-powered worker
	worker := createTupleLeapWorker("tupleleap-worker-1", apiKey, protocol, ledger)

	// Start agents
	if err := orchestrator.Start(ctx); err != nil {
		log.Fatal(err)
	}

	if err := worker.Start(ctx); err != nil {
		log.Fatal(err)
	}

	orchestrator.RegisterWorker(worker)
	fmt.Println("Started TupleLeap worker")

	// Create tasks
	tasks := []*multiagent.Task{
		{
			ID:          "task-1",
			Name:        "Code Review",
			Description: "Review code snippet",
			Type:        "tupleleap",
			Priority:    multiagent.PriorityHigh,
			Input: map[string]interface{}{
				"system_prompt": "You are an expert code reviewer.",
				"user_prompt": `Review this Go code:
func Add(a, b int) int {
    return a + b
}`,
				"model":       "tupleleap-default",
				"temperature": 0.7,
				"max_tokens":  300,
			},
		},
		{
			ID:          "task-2",
			Name:        "Documentation",
			Description: "Generate documentation",
			Type:        "tupleleap",
			Priority:    multiagent.PriorityMedium,
			Input: map[string]interface{}{
				"system_prompt": "You are a technical writer.",
				"user_prompt":   "Write a brief README for a REST API project in Go.",
				"model":         "tupleleap-default",
				"temperature":   0.7,
				"max_tokens":    400,
			},
		},
	}

	// Execute tasks
	for _, task := range tasks {
		fmt.Printf("\nExecuting task: %s\n", task.Name)

		result, err := orchestrator.ExecuteTask(ctx, task)
		if err != nil {
			log.Printf("Task failed: %v\n", err)
			continue
		}

		if resultMap, ok := result.(map[string]interface{}); ok {
			fmt.Printf("Status: %s\n", resultMap["status"])
			if data, ok := resultMap["data"].(map[string]interface{}); ok {
				fmt.Printf("Response: %s\n", data["text"])
				fmt.Printf("Tokens: %v\n", data["tokens_used"])
			}
		}
	}

	// Cleanup
	worker.Stop(ctx)
	orchestrator.Stop(ctx)
}

func createTupleLeapWorker(
	workerID string,
	apiKey string,
	protocol multiagent.Protocol,
	ledger multiagent.LedgerBackend,
) *multiagent.WorkerAgent {
	// Create worker
	worker := multiagent.NewWorkerAgent(
		workerID,
		[]string{"tupleleap"},
		protocol,
		ledger,
	)

	// Create TupleLeap provider
	provider := llm.NewTupleLeap(apiKey)

	// Register handler
	worker.RegisterHandler("tupleleap", func(task *multiagent.Task) (*multiagent.Result, error) {
		// Extract input
		systemPrompt := task.Input["system_prompt"].(string)
		userPrompt := task.Input["user_prompt"].(string)
		model := task.Input["model"].(string)
		temperature := task.Input["temperature"].(float64)
		maxTokens := task.Input["max_tokens"].(int)

		// Call TupleLeap
		resp, err := provider.GenerateCompletion(ctx, &llm.CompletionRequest{
			SystemPrompt: systemPrompt,
			UserPrompt:   userPrompt,
			Temperature:  temperature,
			MaxTokens:    maxTokens,
			Model:        model,
		})

		if err != nil {
			return nil, fmt.Errorf("TupleLeap error: %w", err)
		}

		return &multiagent.Result{
			Status: "success",
			Data: map[string]interface{}{
				"text":        resp.Text,
				"tokens_used": resp.TokensUsed,
				"model":       resp.Model,
				"provider":    "tupleleap",
			},
		}, nil
	})

	return worker
}
