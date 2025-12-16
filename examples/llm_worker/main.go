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

	// Create protocol and ledger
	protocol := multiagent.NewInMemoryProtocol(nil)
	ledger := multiagent.NewInMemoryLedger()

	// Create multi-provider factory with all available providers
	providerFactory := llm.CreateDefaultProviders()

	fmt.Println("Available LLM providers:", providerFactory.ListProviders())

	// Create orchestrator
	orchestrator := multiagent.NewOrchestratorAgent(
		"orchestrator-1",
		protocol,
		ledger,
	)

	// Create LLM workers for different providers
	workers := []*multiagent.WorkerAgent{
		createLLMWorker("openai-worker", "openai", providerFactory, protocol, ledger),
		createLLMWorker("anthropic-worker", "anthropic", providerFactory, protocol, ledger),
		createLLMWorker("ollama-worker", "ollama", providerFactory, protocol, ledger),
	}

	// Start all agents
	if err := orchestrator.Start(ctx); err != nil {
		log.Fatal(err)
	}

	for _, worker := range workers {
		if err := worker.Start(ctx); err != nil {
			log.Printf("Failed to start %s: %v", worker.GetMetadata().AgentID, err)
			continue
		}
		orchestrator.RegisterWorker(worker)
		fmt.Printf("Started %s with provider %s\n",
			worker.GetMetadata().AgentID,
			worker.GetMetadata().Capabilities[0])
	}

	// Create workflow with tasks for different providers
	workflow := &multiagent.Workflow{
		ID:   "llm-comparison",
		Name: "Compare LLM Providers",
		Tasks: []*multiagent.Task{
			{
				ID:          "openai-task",
				Name:        "OpenAI Analysis",
				Description: "Analyze text using GPT-4",
				Type:        "openai",
				Priority:    multiagent.PriorityHigh,
				Input: map[string]interface{}{
					"system_prompt": "You are an expert text analyzer.",
					"user_prompt":   "Analyze the sentiment of: 'This product exceeded my expectations!'",
					"model":         "gpt-4",
					"temperature":   0.7,
					"max_tokens":    200,
				},
			},
			{
				ID:          "anthropic-task",
				Name:        "Anthropic Analysis",
				Description: "Analyze text using Claude",
				Type:        "anthropic",
				Priority:    multiagent.PriorityHigh,
				Input: map[string]interface{}{
					"system_prompt": "You are an expert text analyzer.",
					"user_prompt":   "Analyze the sentiment of: 'This product exceeded my expectations!'",
					"model":         "claude-3-sonnet-20240229",
					"temperature":   0.7,
					"max_tokens":    200,
				},
			},
			{
				ID:          "ollama-task",
				Name:        "Ollama Analysis",
				Description: "Analyze text using local model",
				Type:        "ollama",
				Priority:    multiagent.PriorityMedium,
				Input: map[string]interface{}{
					"system_prompt": "You are an expert text analyzer.",
					"user_prompt":   "Analyze the sentiment of: 'This product exceeded my expectations!'",
					"model":         "llama2",
					"temperature":   0.7,
					"max_tokens":    200,
				},
			},
		},
	}

	// Execute workflow
	fmt.Printf("\nExecuting workflow: %s\n\n", workflow.Name)
	if err := orchestrator.ExecuteWorkflow(ctx, workflow); err != nil {
		log.Fatal(err)
	}

	// Display results
	fmt.Println("\n=== Results ===\n")
	for _, task := range workflow.Tasks {
		taskDetails, _ := ledger.GetTask(ctx, task.ID)
		fmt.Printf("%s (%s):\n", taskDetails.Name, taskDetails.Type)
		fmt.Printf("  Status: %s\n", taskDetails.Status)
		if taskDetails.Result != nil {
			if resultMap, ok := taskDetails.Result.(map[string]interface{}); ok {
				fmt.Printf("  Response: %s\n", resultMap["text"])
				fmt.Printf("  Tokens: %d\n", resultMap["tokens_used"])
				fmt.Printf("  Model: %s\n", resultMap["model"])
			}
		}
		fmt.Println()
	}

	// Cleanup
	for _, worker := range workers {
		worker.Stop(ctx)
	}
	orchestrator.Stop(ctx)
}

func createLLMWorker(
	workerID string,
	providerName string,
	providerFactory *llm.MultiProviderFactory,
	protocol multiagent.Protocol,
	ledger multiagent.LedgerBackend,
) *multiagent.WorkerAgent {
	// Get provider from factory
	provider, err := providerFactory.GetProvider(providerName)
	if err != nil {
		log.Printf("Provider %s not available: %v", providerName, err)
		return nil
	}

	// Create worker
	worker := multiagent.NewWorkerAgent(
		workerID,
		[]string{providerName}, // Capability matches provider name
		protocol,
		ledger,
	)

	// Register LLM task handler
	worker.RegisterHandler(providerName, func(task *multiagent.Task) (*multiagent.Result, error) {
		ctx := context.Background()

		// Extract input
		systemPrompt := task.Input["system_prompt"].(string)
		userPrompt := task.Input["user_prompt"].(string)
		model := task.Input["model"].(string)
		temperature := task.Input["temperature"].(float64)
		maxTokens := task.Input["max_tokens"].(int)

		// Call LLM
		resp, err := provider.GenerateCompletion(ctx, &llm.CompletionRequest{
			SystemPrompt: systemPrompt,
			UserPrompt:   userPrompt,
			Temperature:  temperature,
			MaxTokens:    maxTokens,
			Model:        model,
		})

		if err != nil {
			return nil, fmt.Errorf("LLM error: %w", err)
		}

		return &multiagent.Result{
			Status: "success",
			Data: map[string]interface{}{
				"text":        resp.Text,
				"tokens_used": resp.TokensUsed,
				"model":       resp.Model,
				"provider":    provider.Name(),
			},
		}, nil
	})

	return worker
}
