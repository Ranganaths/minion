// Package main demonstrates the chain package features including:
// - Safe type assertions with GetInt, GetFloat, GetBool, GetStringSlice, GetMap
// - Context-aware streaming with proper goroutine cleanup
// - Sequential and RAG chain usage
// - LLM request validation
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Ranganaths/minion/chain"
	"github.com/Ranganaths/minion/config"
	"github.com/Ranganaths/minion/llm"
)

func main() {
	fmt.Println("=== Minion Chain Features Example ===")
	fmt.Println()

	// Demonstrate configuration with safe error handling
	demonstrateConfig()

	// Demonstrate LLM request validation
	demonstrateLLMValidation()

	// Demonstrate safe type assertions
	demonstrateSafeTypeAssertions()

	// Demonstrate chain streaming with context cancellation
	demonstrateStreamingWithContext()

	fmt.Println("\n=== All examples completed successfully! ===")
}

// demonstrateConfig shows the new non-panicking config methods
func demonstrateConfig() {
	fmt.Println("--- Config Package: Safe Environment Variables ---")

	// Create a custom env helper
	env := config.NewEnv("MYAPP")

	// Safe retrieval with defaults
	apiKey := env.GetString("API_KEY", "default-key")
	maxRetries := env.GetInt("MAX_RETRIES", 3)
	timeout := env.GetDuration("TIMEOUT", 30*time.Second)
	debug := env.GetBool("DEBUG", false)

	fmt.Printf("API Key: %s (from env or default)\n", apiKey)
	fmt.Printf("Max Retries: %d\n", maxRetries)
	fmt.Printf("Timeout: %v\n", timeout)
	fmt.Printf("Debug: %v\n", debug)

	// Safe required value retrieval (no panics!)
	_, err := env.RequireString("REQUIRED_KEY")
	if err != nil {
		fmt.Printf("RequireString returned error (expected): %v\n", err)
	}

	_, err = env.RequireInt("REQUIRED_INT")
	if err != nil {
		fmt.Printf("RequireInt returned error (expected): %v\n", err)
	}

	_, err = env.RequireBool("REQUIRED_BOOL")
	if err != nil {
		fmt.Printf("RequireBool returned error (expected): %v\n", err)
	}

	fmt.Println()
}

// demonstrateLLMValidation shows the new validation features
func demonstrateLLMValidation() {
	fmt.Println("--- LLM Package: Request Validation ---")

	// Valid request
	validReq := &llm.CompletionRequest{
		Model:       "gpt-4",
		UserPrompt:  "Hello, world!",
		Temperature: 0.7,
		MaxTokens:   100,
	}

	if err := validReq.Validate(); err != nil {
		log.Fatalf("Unexpected validation error: %v", err)
	}
	fmt.Println("Valid request passed validation")

	// Invalid request - missing model
	invalidReq1 := &llm.CompletionRequest{
		UserPrompt:  "Hello!",
		Temperature: 0.7,
	}
	if err := invalidReq1.Validate(); err != nil {
		fmt.Printf("Invalid request 1: %v\n", err)
	}

	// Invalid request - temperature out of range
	invalidReq2 := &llm.CompletionRequest{
		Model:       "gpt-4",
		UserPrompt:  "Hello!",
		Temperature: 3.0, // Max is 2.0
	}
	if err := invalidReq2.Validate(); err != nil {
		fmt.Printf("Invalid request 2: %v\n", err)
	}

	// Apply defaults
	reqWithoutDefaults := &llm.CompletionRequest{
		UserPrompt: "Hello!",
	}
	reqWithDefaults := reqWithoutDefaults.WithDefaults("gpt-4", 1000)
	fmt.Printf("After WithDefaults: Model=%s, MaxTokens=%d\n",
		reqWithDefaults.Model, reqWithDefaults.MaxTokens)

	// Chat request validation
	chatReq := &llm.ChatRequest{
		Model: "gpt-4",
		Messages: []llm.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello!"},
		},
	}
	if err := chatReq.Validate(); err != nil {
		log.Fatalf("Unexpected chat validation error: %v", err)
	}
	fmt.Println("Valid chat request passed validation")

	// Invalid chat - bad role
	invalidChat := &llm.ChatRequest{
		Model: "gpt-4",
		Messages: []llm.Message{
			{Role: "invalid_role", Content: "Hello!"},
		},
	}
	if err := invalidChat.Validate(); err != nil {
		fmt.Printf("Invalid chat request: %v\n", err)
	}

	fmt.Println()
}

// demonstrateSafeTypeAssertions shows the new safe type assertion helpers
func demonstrateSafeTypeAssertions() {
	fmt.Println("--- Chain Package: Safe Type Assertions ---")

	// Create a base chain to access the helper methods
	baseChain := chain.NewBaseChain("demo_chain")

	// Simulate inputs from various sources (JSON, user input, etc.)
	inputs := map[string]any{
		"count":      42,
		"rate":       3.14,
		"enabled":    true,
		"tags":       []string{"go", "minion", "chain"},
		"metadata":   map[string]any{"version": "1.0", "author": "minion"},
		"float_int":  float64(100), // JSON numbers are float64
		"string_val": "hello",
	}

	// Safe integer extraction (handles int, int64, float64)
	count, err := baseChain.GetInt(inputs, "count")
	if err != nil {
		log.Fatalf("GetInt error: %v", err)
	}
	fmt.Printf("GetInt('count'): %d\n", count)

	// GetInt handles float64 from JSON
	floatAsInt, err := baseChain.GetInt(inputs, "float_int")
	if err != nil {
		log.Fatalf("GetInt(float_int) error: %v", err)
	}
	fmt.Printf("GetInt('float_int'): %d (from float64)\n", floatAsInt)

	// GetIntOr with default
	missing := baseChain.GetIntOr(inputs, "missing_key", 999)
	fmt.Printf("GetIntOr('missing_key', 999): %d\n", missing)

	// Safe float extraction
	rate, err := baseChain.GetFloat(inputs, "rate")
	if err != nil {
		log.Fatalf("GetFloat error: %v", err)
	}
	fmt.Printf("GetFloat('rate'): %.2f\n", rate)

	// Safe bool extraction
	enabled, err := baseChain.GetBool(inputs, "enabled")
	if err != nil {
		log.Fatalf("GetBool error: %v", err)
	}
	fmt.Printf("GetBool('enabled'): %v\n", enabled)

	// Safe string slice extraction
	tags, err := baseChain.GetStringSlice(inputs, "tags")
	if err != nil {
		log.Fatalf("GetStringSlice error: %v", err)
	}
	fmt.Printf("GetStringSlice('tags'): %v\n", tags)

	// Safe map extraction
	metadata, err := baseChain.GetMap(inputs, "metadata")
	if err != nil {
		log.Fatalf("GetMap error: %v", err)
	}
	fmt.Printf("GetMap('metadata'): %v\n", metadata)

	// String helpers
	fmt.Printf("AsString(42): %s\n", chain.AsString(42))
	fmt.Printf("AsString(nil): '%s'\n", chain.AsString(nil))

	// Slice helpers
	mixedSlice := []interface{}{"a", "b", "c"}
	fmt.Printf("AsStringSlice(mixed): %v\n", chain.AsStringSlice(mixedSlice))

	// Error cases (safe - no panics!)
	_, err = baseChain.GetInt(inputs, "string_val")
	if err != nil {
		fmt.Printf("GetInt('string_val') error (expected): %v\n", err)
	}

	_, err = baseChain.GetBool(inputs, "count")
	if err != nil {
		fmt.Printf("GetBool('count') error (expected): %v\n", err)
	}

	fmt.Println()
}

// demonstrateStreamingWithContext shows context-aware streaming
func demonstrateStreamingWithContext() {
	fmt.Println("--- Chain Package: Context-Aware Streaming ---")

	// Create a simple mock chain for demonstration
	// In real usage, you would use LLMChain, RAGChain, etc.
	mockChain := &MockStreamingChain{
		BaseChain: chain.NewBaseChain("mock_streaming"),
	}

	// Normal streaming - consume all events
	fmt.Println("\n1. Normal streaming (consume all events):")
	ctx := context.Background()
	streamCh, err := mockChain.Stream(ctx, map[string]any{"query": "test"})
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	for event := range streamCh {
		fmt.Printf("   Event: type=%s, content=%s\n", event.Type, event.Content)
	}

	// Streaming with cancellation - goroutine properly cleaned up
	fmt.Println("\n2. Streaming with early cancellation:")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	streamCh, err = mockChain.Stream(ctx, map[string]any{"query": "test"})
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	eventCount := 0
	for event := range streamCh {
		eventCount++
		fmt.Printf("   Event %d: type=%s\n", eventCount, event.Type)
		if eventCount >= 2 {
			fmt.Println("   (stopping early - context will cancel)")
			cancel()
		}
	}
	fmt.Printf("   Received %d events before cancellation\n", eventCount)

	fmt.Println()
}

// MockStreamingChain demonstrates the streaming pattern used in all chains
type MockStreamingChain struct {
	*chain.BaseChain
}

func (c *MockStreamingChain) InputKeys() []string {
	return []string{"query"}
}

func (c *MockStreamingChain) OutputKeys() []string {
	return []string{"result"}
}

func (c *MockStreamingChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	return map[string]any{"result": "mock result"}, nil
}

func (c *MockStreamingChain) Stream(ctx context.Context, inputs map[string]any) (<-chan chain.StreamEvent, error) {
	ch := make(chan chain.StreamEvent, 10)

	go func() {
		defer close(ch)

		// Context-aware send helper - prevents goroutine leaks
		send := func(event chain.StreamEvent) bool {
			select {
			case <-ctx.Done():
				return false
			case ch <- event:
				return true
			}
		}

		// Emit start event
		if !send(chain.MakeStreamEvent(chain.StreamEventStart, "starting", nil, nil)) {
			return
		}

		// Simulate streaming chunks
		for i := 0; i < 5; i++ {
			select {
			case <-ctx.Done():
				// Context cancelled - exit cleanly
				send(chain.MakeStreamEvent(chain.StreamEventError, "", nil, ctx.Err()))
				return
			case <-time.After(20 * time.Millisecond):
				// Simulate work
			}

			if !send(chain.MakeStreamEvent(chain.StreamEventChunk, fmt.Sprintf("chunk-%d", i), nil, nil)) {
				return
			}
		}

		// Emit completion event
		send(chain.MakeStreamEvent(chain.StreamEventComplete, "", map[string]any{"result": "done"}, nil))
	}()

	return ch, nil
}

func init() {
	// Ensure we don't need actual API keys for this demo
	if os.Getenv("OPENAI_API_KEY") == "" {
		// Set a dummy value for validation examples
		// Real examples would require actual keys
	}
}
