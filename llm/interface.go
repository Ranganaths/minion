package llm

import "context"

// Provider defines the interface for LLM providers
type Provider interface {
	// GenerateCompletion generates a text completion
	GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)

	// GenerateChat generates a chat response
	GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// Name returns the provider name
	Name() string
}

// CompletionRequest represents a completion request
type CompletionRequest struct {
	SystemPrompt string
	UserPrompt   string
	Temperature  float64
	MaxTokens    int
	Model        string
}

// CompletionResponse represents a completion response
type CompletionResponse struct {
	Text          string
	TokensUsed    int
	FinishReason  string
	Model         string
}

// ChatRequest represents a chat request
type ChatRequest struct {
	Messages    []Message
	Temperature float64
	MaxTokens   int
	Model       string
}

// Message represents a chat message
type Message struct {
	Role    string // "system", "user", "assistant"
	Content string
}

// ChatResponse represents a chat response
type ChatResponse struct {
	Message       Message
	TokensUsed    int
	FinishReason  string
	Model         string
}
