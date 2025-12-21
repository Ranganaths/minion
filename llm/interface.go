// Package llm provides interfaces and types for Large Language Model providers.
// It defines a common abstraction layer for interacting with various LLM services
// such as OpenAI, Anthropic, Google, and others.
package llm

import (
	"context"
	"fmt"
)

// Provider defines the interface for LLM providers.
// Implementations should handle authentication, rate limiting, and retries internally.
type Provider interface {
	// GenerateCompletion generates a text completion from the given prompt.
	// The request must be validated before calling this method.
	GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)

	// GenerateChat generates a chat response from a conversation history.
	// The request must be validated before calling this method.
	GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// Name returns the provider name (e.g., "openai", "anthropic", "google").
	Name() string
}

// HealthCheckProvider extends Provider with health check capability.
// Providers that support health checks should implement this interface.
type HealthCheckProvider interface {
	Provider
	// HealthCheck verifies connectivity to the LLM service.
	// Returns nil if the service is reachable and authenticated.
	HealthCheck(ctx context.Context) error
}

// ValidationError represents an error from request validation.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// CompletionRequest represents a completion request.
// Use Validate() to check the request before sending to a provider.
type CompletionRequest struct {
	SystemPrompt string
	UserPrompt   string
	Temperature  float64
	MaxTokens    int
	Model        string
}

// Validate checks if the completion request has valid parameters.
// Returns nil if valid, or a ValidationError describing the issue.
func (r *CompletionRequest) Validate() error {
	if r.Model == "" {
		return &ValidationError{Field: "Model", Message: "model name is required"}
	}
	if r.UserPrompt == "" && r.SystemPrompt == "" {
		return &ValidationError{Field: "UserPrompt", Message: "at least one of UserPrompt or SystemPrompt is required"}
	}
	if r.Temperature < 0 || r.Temperature > 2.0 {
		return &ValidationError{Field: "Temperature", Message: "temperature must be between 0 and 2.0"}
	}
	if r.MaxTokens < 0 {
		return &ValidationError{Field: "MaxTokens", Message: "max_tokens must be non-negative"}
	}
	return nil
}

// WithDefaults returns a copy of the request with default values applied.
// This does not modify the original request.
func (r *CompletionRequest) WithDefaults(defaultModel string, defaultMaxTokens int) *CompletionRequest {
	req := *r
	if req.Model == "" {
		req.Model = defaultModel
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = defaultMaxTokens
	}
	return &req
}

// CompletionResponse represents a completion response from an LLM provider.
type CompletionResponse struct {
	Text         string
	TokensUsed   int
	FinishReason string
	Model        string
}

// ChatRequest represents a chat request.
// Use Validate() to check the request before sending to a provider.
type ChatRequest struct {
	Messages    []Message
	Temperature float64
	MaxTokens   int
	Model       string
}

// Validate checks if the chat request has valid parameters.
// Returns nil if valid, or a ValidationError describing the issue.
func (r *ChatRequest) Validate() error {
	if r.Model == "" {
		return &ValidationError{Field: "Model", Message: "model name is required"}
	}
	if len(r.Messages) == 0 {
		return &ValidationError{Field: "Messages", Message: "at least one message is required"}
	}
	if r.Temperature < 0 || r.Temperature > 2.0 {
		return &ValidationError{Field: "Temperature", Message: "temperature must be between 0 and 2.0"}
	}
	if r.MaxTokens < 0 {
		return &ValidationError{Field: "MaxTokens", Message: "max_tokens must be non-negative"}
	}
	// Validate each message
	for i, msg := range r.Messages {
		if msg.Role == "" {
			return &ValidationError{Field: fmt.Sprintf("Messages[%d].Role", i), Message: "role is required"}
		}
		if msg.Role != "system" && msg.Role != "user" && msg.Role != "assistant" {
			return &ValidationError{Field: fmt.Sprintf("Messages[%d].Role", i), Message: "role must be 'system', 'user', or 'assistant'"}
		}
	}
	return nil
}

// WithDefaults returns a copy of the request with default values applied.
// This does not modify the original request.
func (r *ChatRequest) WithDefaults(defaultModel string, defaultMaxTokens int) *ChatRequest {
	req := *r
	if req.Model == "" {
		req.Model = defaultModel
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = defaultMaxTokens
	}
	// Deep copy messages
	req.Messages = make([]Message, len(r.Messages))
	copy(req.Messages, r.Messages)
	return &req
}

// Message represents a chat message in a conversation.
type Message struct {
	Role    string // "system", "user", or "assistant"
	Content string
}

// ChatResponse represents a chat response from an LLM provider.
type ChatResponse struct {
	Message      Message
	TokensUsed   int
	FinishReason string
	Model        string
}

// ValidateAndNormalize validates the request and applies defaults.
// This is a convenience function combining validation and defaults.
func ValidateCompletionRequest(req *CompletionRequest, defaultModel string, defaultMaxTokens int) (*CompletionRequest, error) {
	normalized := req.WithDefaults(defaultModel, defaultMaxTokens)
	if err := normalized.Validate(); err != nil {
		return nil, err
	}
	return normalized, nil
}

// ValidateChatRequest validates the request and applies defaults.
// This is a convenience function combining validation and defaults.
func ValidateChatRequest(req *ChatRequest, defaultModel string, defaultMaxTokens int) (*ChatRequest, error) {
	normalized := req.WithDefaults(defaultModel, defaultMaxTokens)
	if err := normalized.Validate(); err != nil {
		return nil, err
	}
	return normalized, nil
}
