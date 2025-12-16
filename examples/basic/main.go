package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/yourusername/minion/core"
	"github.com/yourusername/minion/llm"
	"github.com/yourusername/minion/models"
	"github.com/yourusername/minion/storage"
)

func main() {
	fmt.Println("🤖 Agent Framework - Basic Example")
	fmt.Println("===================================\n")

	// Check for OpenAI API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	// 1. Create framework with in-memory storage and OpenAI provider
	fmt.Println("1. Initializing framework...")
	framework := core.NewFramework(
		core.WithStorage(storage.NewInMemory()),
		core.WithLLMProvider(llm.NewOpenAI(apiKey)),
	)
	defer framework.Close()
	fmt.Println("   ✓ Framework initialized\n")

	// 2. Create an agent
	fmt.Println("2. Creating agent...")
	agent, err := framework.CreateAgent(context.Background(), &models.CreateAgentRequest{
		Name:         "My First Agent",
		Description:  "A helpful assistant that answers questions",
		BehaviorType: "default",
		Config: models.AgentConfig{
			LLMProvider: "openai",
			LLMModel:    "gpt-4",
			Temperature: 0.7,
			MaxTokens:   500,
			Personality: "professional",
			Language:    "en",
		},
		Capabilities: []string{"general_knowledge", "question_answering"},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	fmt.Printf("   ✓ Agent created: %s (ID: %s)\n", agent.Name, agent.ID)
	fmt.Printf("   - Behavior: %s\n", agent.BehaviorType)
	fmt.Printf("   - Status: %s\n\n", agent.Status)

	// 3. Activate the agent
	fmt.Println("3. Activating agent...")
	activeStatus := models.StatusActive
	agent, err = framework.UpdateAgent(context.Background(), agent.ID, &models.UpdateAgentRequest{
		Status: &activeStatus,
	})
	if err != nil {
		log.Fatalf("Failed to activate agent: %v", err)
	}
	fmt.Printf("   ✓ Agent activated: %s\n\n", agent.Status)

	// 4. Execute agent with various inputs
	questions := []string{
		"What is 2 + 2?",
		"Explain quantum computing in one sentence.",
		"What are the primary colors?",
	}

	fmt.Println("4. Executing agent with questions...")
	for i, question := range questions {
		fmt.Printf("\n   Question %d: %s\n", i+1, question)

		output, err := framework.Execute(context.Background(), agent.ID, &models.Input{
			Raw:  question,
			Type: "text",
		})
		if err != nil {
			log.Printf("   ✗ Execution failed: %v", err)
			continue
		}

		fmt.Printf("   Answer: %v\n", output.Result)
		if output.Metadata != nil {
			if tokens, ok := output.Metadata["tokens_used"].(int); ok {
				fmt.Printf("   Tokens used: %d\n", tokens)
			}
		}
	}

	// 5. Get agent metrics
	fmt.Println("\n5. Retrieving agent metrics...")
	metrics, err := framework.GetMetrics(context.Background(), agent.ID)
	if err != nil {
		log.Printf("   Warning: Failed to get metrics: %v", err)
	} else {
		fmt.Printf("   ✓ Metrics retrieved:\n")
		fmt.Printf("   - Total executions: %d\n", metrics.TotalExecutions)
		fmt.Printf("   - Successful: %d\n", metrics.SuccessfulExecutions)
		fmt.Printf("   - Failed: %d\n", metrics.FailedExecutions)
		fmt.Printf("   - Avg execution time: %.2fms\n", metrics.AvgExecutionTime)
	}

	// 6. Get recent activities
	fmt.Println("\n6. Retrieving recent activities...")
	activities, err := framework.GetActivities(context.Background(), agent.ID, 5)
	if err != nil {
		log.Printf("   Warning: Failed to get activities: %v", err)
	} else {
		fmt.Printf("   ✓ Found %d activities:\n", len(activities))
		for i, activity := range activities {
			fmt.Printf("   %d. Action: %s | Status: %s | Duration: %dms\n",
				i+1, activity.Action, activity.Status, activity.Duration)
		}
	}

	// 7. List all agents
	fmt.Println("\n7. Listing all agents...")
	agentList, err := framework.ListAgents(context.Background(), &models.ListAgentsRequest{
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		log.Printf("   Warning: Failed to list agents: %v", err)
	} else {
		fmt.Printf("   ✓ Found %d agent(s):\n", len(agentList.Agents))
		for i, a := range agentList.Agents {
			fmt.Printf("   %d. %s (%s) - Status: %s\n", i+1, a.Name, a.ID, a.Status)
		}
	}

	fmt.Println("\n✅ Example completed successfully!")
}
