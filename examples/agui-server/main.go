// Example: AG-UI Protocol Server
//
// This example demonstrates how to expose a Minion agent via the AG-UI protocol,
// enabling real-time streaming to frontend applications like React or Angular.
//
// The AG-UI (Agent-User Interface) protocol enables:
// - Real-time token streaming via SSE
// - Shared state synchronization with JSON Patch
// - Tool call visualization
// - Frontend integration with CopilotKit and other AG-UI clients
//
// Run this example:
//
//	go run main.go
//
// Then interact with the agent:
//
//	# Send a run request with SSE streaming
//	curl -X POST http://localhost:8081 \
//	  -H "Content-Type: application/json" \
//	  -H "Accept: text/event-stream" \
//	  -d '{"messages":[{"role":"user","content":"Tell me a short joke"}]}'
//
// For frontend integration, use the AG-UI client SDK:
// https://docs.ag-ui.com/
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Ranganaths/minion/core"
	"github.com/Ranganaths/minion/llm"
	"github.com/Ranganaths/minion/models"
	"github.com/Ranganaths/minion/protocols/agui"
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
		Name:        "AG-UI Demo Agent",
		Description: "A helpful AI assistant with real-time streaming via AG-UI protocol",
		Capabilities: []string{
			"chat",
			"streaming",
			"tools",
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

	// Configure AG-UI server
	serverConfig := agui.DefaultServerConfig()
	serverConfig.EnableCORS = true

	// Create AG-UI server
	server, err := agui.NewAGUIServer(framework, agent.ID, serverConfig)
	if err != nil {
		log.Fatalf("Failed to create AG-UI server: %v", err)
	}

	// Print startup info
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║             AG-UI Protocol Server Started                  ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Server: http://localhost:8081                             ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Endpoints:                                                ║")
	fmt.Println("║    POST / - Run request with SSE streaming                 ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Event Types:                                              ║")
	fmt.Println("║    RUN_STARTED         - Run began                         ║")
	fmt.Println("║    TEXT_MESSAGE_START  - Message started                   ║")
	fmt.Println("║    TEXT_MESSAGE_CONTENT- Token/chunk streamed              ║")
	fmt.Println("║    TEXT_MESSAGE_END    - Message completed                 ║")
	fmt.Println("║    TOOL_CALL_START     - Tool invocation started           ║")
	fmt.Println("║    TOOL_CALL_END       - Tool invocation completed         ║")
	fmt.Println("║    STATE_DELTA         - State change (JSON Patch)         ║")
	fmt.Println("║    RUN_FINISHED        - Run completed                     ║")
	fmt.Println("║    RUN_ERROR           - Error occurred                    ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	// Start server
	log.Fatal(server.ListenAndServe(":8081"))
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
