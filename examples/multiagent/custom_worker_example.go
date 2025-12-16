package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/agentql/agentql/pkg/minion/core/multiagent"
	"github.com/agentql/agentql/pkg/minion/llm"
)

// CustomSQLWorker is a custom worker that handles SQL-related tasks
type CustomSQLWorker struct {
	llmProvider multiagent.LLMProvider
}

func NewCustomSQLWorker(llmProvider multiagent.LLMProvider) *CustomSQLWorker {
	return &CustomSQLWorker{
		llmProvider: llmProvider,
	}
}

func (w *CustomSQLWorker) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
	systemPrompt := `You are an expert SQL developer. Generate efficient, secure SQL queries.
Follow best practices: use parameterized queries, avoid SQL injection, optimize for performance.`

	userPrompt := fmt.Sprintf(`Task: %s

Description: %s

Requirements: %v

Generate the SQL query with explanation.`,
		task.Name, task.Description, task.Input)

	resp, err := w.llmProvider.GenerateCompletion(ctx, &multiagent.CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.2,
		MaxTokens:    1000,
		Model:        "gpt-4",
	})
	if err != nil {
		return nil, fmt.Errorf("SQL generation failed: %w", err)
	}

	return map[string]interface{}{
		"sql":         resp.Text,
		"dialect":     "postgresql",
		"tokens_used": resp.TokensUsed,
	}, nil
}

func (w *CustomSQLWorker) GetCapabilities() []string {
	return []string{"sql_generation", "query_optimization", "schema_design", "data_migration"}
}

func (w *CustomSQLWorker) GetName() string {
	return "sql_specialist"
}

// LLMAdapter adapts the minion LLM provider to the multiagent interface
type LLMAdapter struct {
	provider *llm.OpenAIProvider
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
	// Initialize LLM provider
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	llmProvider := &LLMAdapter{
		provider: llm.NewOpenAI(apiKey),
	}

	// Create coordinator
	coordinator := multiagent.NewCoordinator(llmProvider, nil)
	ctx := context.Background()

	// Initialize with default workers
	if err := coordinator.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize coordinator: %v", err)
	}

	fmt.Println("✅ Multi-agent system initialized")

	// Create and register custom SQL worker
	fmt.Println("🔧 Registering custom SQL worker...")
	sqlHandler := NewCustomSQLWorker(llmProvider)
	sqlWorker, err := coordinator.CreateCustomWorker(
		ctx,
		"SQL Specialist",
		multiagent.RoleSpecialist,
		sqlHandler,
	)
	if err != nil {
		log.Fatalf("Failed to register SQL worker: %v", err)
	}

	fmt.Printf("   ✅ SQL worker registered (ID: %s)\n", sqlWorker.GetMetadata().AgentID[:8])
	fmt.Printf("   Capabilities: %v\n\n", sqlHandler.GetCapabilities())

	// Example task: Generate SQL query
	fmt.Println("🚀 Executing SQL Generation Task")
	sqlTask := &multiagent.TaskRequest{
		Name:        "Generate User Query",
		Description: "Create a SQL query to fetch active users with their last login dates",
		Type:        "sql_generation",
		Priority:    multiagent.PriorityHigh,
		Input: map[string]interface{}{
			"table":  "users",
			"fields": []string{"id", "username", "email", "last_login"},
			"conditions": map[string]interface{}{
				"status":          "active",
				"last_login_days": 30,
			},
			"order_by": "last_login DESC",
			"limit":    100,
		},
	}

	result, err := coordinator.ExecuteTask(ctx, sqlTask)
	if err != nil {
		log.Printf("   ❌ Task failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Task completed: %s\n", result.Status)
		fmt.Printf("   Output: %v\n\n", result.Output)
	}

	// List all workers including custom
	fmt.Println("📋 All Registered Workers:")
	for i, worker := range coordinator.GetWorkers() {
		fmt.Printf("   %d. Agent %s (Role: %s)\n", i+1, worker.AgentID[:8], worker.Role)
		fmt.Printf("      Capabilities: %v\n", worker.Capabilities)
	}
	fmt.Println()

	// Shutdown
	fmt.Println("🛑 Shutting down...")
	if err := coordinator.Shutdown(ctx); err != nil {
		log.Printf("Error during shutdown: %v\n", err)
	}

	fmt.Println("✅ System shut down successfully")
}
