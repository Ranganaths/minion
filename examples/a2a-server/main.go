// Example: A2A Protocol Server
//
// This example demonstrates how to expose a Minion agent via the A2A protocol,
// enabling interoperability with other AI agents following the Google A2A standard.
//
// The A2A (Agent-to-Agent) protocol enables:
// - Agent discovery via Agent Cards
// - Task-based communication with structured messages
// - SSE streaming for real-time updates
// - Session management for multi-turn conversations
//
// Run this example:
//
//	go run main.go
//
// Then interact with the agent:
//
//	# Fetch agent card
//	curl http://localhost:8080/.well-known/agent.json
//
//	# Send a task
//	curl -X POST http://localhost:8080 \
//	  -H "Content-Type: application/json" \
//	  -d '{"jsonrpc":"2.0","id":1,"method":"tasks/send","params":{"message":{"role":"user","parts":[{"type":"text","text":"Hello, what can you help me with?"}]}}}'
//
//	# Send a task with SSE streaming
//	curl -X POST http://localhost:8080 \
//	  -H "Content-Type: application/json" \
//	  -H "Accept: text/event-stream" \
//	  -d '{"jsonrpc":"2.0","id":1,"method":"tasks/sendSubscribe","params":{"message":{"role":"user","parts":[{"type":"text","text":"Write a haiku about programming"}]}}}'
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Ranganaths/minion/core"
	"github.com/Ranganaths/minion/llm"
	"github.com/Ranganaths/minion/models"
	"github.com/Ranganaths/minion/protocols/a2a"
	"github.com/Ranganaths/minion/storage"
)

func main() {
	// Get API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		log.Println("Warning: No API key found. Set OPENAI_API_KEY or ANTHROPIC_API_KEY")
		log.Println("Using mock provider for demonstration")
	}

	// Create LLM provider
	var provider llm.Provider
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		provider = llm.NewAnthropic(os.Getenv("ANTHROPIC_API_KEY"))
	} else if os.Getenv("OPENAI_API_KEY") != "" {
		provider = llm.NewOpenAI(os.Getenv("OPENAI_API_KEY"))
	} else {
		// Use a mock provider for testing
		provider = &mockProvider{}
	}

	// Create storage
	store := storage.NewInMemory()

	// Create framework
	framework := core.NewFramework(
		core.WithLLMProvider(provider),
		core.WithStorage(store),
	)

	// Create an agent
	ctx := context.Background()
	agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:        "A2A Demo Agent",
		Description: "A helpful AI assistant accessible via the A2A protocol. Can assist with general questions, writing, and analysis.",
		Capabilities: []string{
			"general-assistance",
			"writing",
			"analysis",
			"conversation",
		},
		Config: models.AgentConfig{
			LLMProvider: "default",
			Temperature: 0.7,
			MaxTokens:   2048,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	log.Printf("Created agent: %s (%s)", agent.Name, agent.ID)

	// Configure A2A server
	serverConfig := a2a.DefaultServerConfig()
	serverConfig.EnableCORS = true
	serverConfig.EnableSSE = true

	// Create A2A server
	server, err := a2a.NewA2AServer(
		framework,
		agent.ID,
		"http://localhost:8080",
		serverConfig,
	)
	if err != nil {
		log.Fatalf("Failed to create A2A server: %v", err)
	}

	// Print startup info
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              A2A Protocol Server Started                   ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Server:     http://localhost:8080                         ║")
	fmt.Println("║  Agent Card: http://localhost:8080/.well-known/agent.json  ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Endpoints:                                                ║")
	fmt.Println("║    POST /           - JSON-RPC endpoint                    ║")
	fmt.Println("║    GET  /.well-known/agent.json - Agent discovery          ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Methods:                                                  ║")
	fmt.Println("║    tasks/send          - Send task (sync)                  ║")
	fmt.Println("║    tasks/sendSubscribe - Send task with SSE streaming      ║")
	fmt.Println("║    tasks/get           - Get task status                   ║")
	fmt.Println("║    tasks/cancel        - Cancel a task                     ║")
	fmt.Println("║    tasks/resubscribe   - Resubscribe to task updates       ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	// Start server
	log.Fatal(server.ListenAndServe(":8080"))
}

// mockProvider is a mock LLM provider for testing without API keys
type mockProvider struct{}

func (p *mockProvider) GenerateCompletion(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return &llm.CompletionResponse{
		Text:       "This is a mock response. Set OPENAI_API_KEY or ANTHROPIC_API_KEY for real responses.",
		TokensUsed: 20,
	}, nil
}

func (p *mockProvider) GenerateChat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	// Generate a simple response based on the last message
	lastMsg := ""
	if len(req.Messages) > 0 {
		lastMsg = req.Messages[len(req.Messages)-1].Content
	}

	response := fmt.Sprintf("I received your message: %q. This is a mock response - set OPENAI_API_KEY or ANTHROPIC_API_KEY for real AI responses.", lastMsg)

	return &llm.ChatResponse{
		Message: llm.Message{
			Role:    "assistant",
			Content: response,
		},
		TokensUsed: 50,
	}, nil
}

func (p *mockProvider) Name() string {
	return "mock"
}
