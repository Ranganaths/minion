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

// ToolUseProvider extends Provider with native tool/function calling capability.
// Providers that support tool/function calling (OpenAI, Anthropic, Google, etc.)
// should implement this interface. The interface is provider-agnostic and works
// with any LLM that supports structured tool definitions and responses.
type ToolUseProvider interface {
	Provider

	// GenerateWithTools generates a response that may include tool calls.
	// The provider will return tool calls if the model decides to use tools.
	// Different providers may have different native formats, but this interface
	// normalizes them to a common structure.
	GenerateWithTools(ctx context.Context, req *ToolUseRequest) (*ToolUseResponse, error)

	// SupportsToolUse returns true if this provider supports native tool use.
	SupportsToolUse() bool
}

// ToolUseRequest represents a request with tool definitions.
type ToolUseRequest struct {
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools"`
	ToolChoice  *ToolChoice      `json:"tool_choice,omitempty"`
	Temperature float64          `json:"temperature"`
	MaxTokens   int              `json:"max_tokens"`
	Model       string           `json:"model"`
	System      string           `json:"system,omitempty"`
}

// Validate checks if the tool use request has valid parameters.
func (r *ToolUseRequest) Validate() error {
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
	// Validate tools
	for i, tool := range r.Tools {
		if tool.Name == "" {
			return &ValidationError{Field: fmt.Sprintf("Tools[%d].Name", i), Message: "tool name is required"}
		}
	}
	return nil
}

// WithDefaults returns a copy of the request with default values applied.
func (r *ToolUseRequest) WithDefaults(defaultModel string, defaultMaxTokens int) *ToolUseRequest {
	req := *r
	if req.Model == "" {
		req.Model = defaultModel
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = defaultMaxTokens
	}
	// Deep copy messages and tools
	req.Messages = make([]Message, len(r.Messages))
	copy(req.Messages, r.Messages)
	req.Tools = make([]ToolDefinition, len(r.Tools))
	copy(req.Tools, r.Tools)
	return &req
}

// ToolDefinition defines a tool that the model can call.
// This is a provider-agnostic format that can be converted to
// provider-specific formats (OpenAI functions, Anthropic tools, etc.).
type ToolDefinition struct {
	// Name is the unique identifier for the tool
	Name string `json:"name"`

	// Description explains what the tool does (helps the model decide when to use it)
	Description string `json:"description"`

	// InputSchema defines the JSON Schema for the tool's parameters
	// Uses standard JSON Schema format compatible with all major providers
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ToolChoice controls how the model uses tools.
type ToolChoice struct {
	// Type can be "auto", "any", "tool", or "none"
	// - "auto": Model decides whether to use tools
	// - "any": Model must use at least one tool
	// - "tool": Model must use the specific tool named in Name
	// - "none": Model should not use any tools
	Type string `json:"type"`

	// Name is required when Type is "tool"
	Name string `json:"name,omitempty"`
}

// ToolUseResponse represents a response that may include tool calls.
type ToolUseResponse struct {
	// Content contains the text content of the response (if any)
	Content string `json:"content,omitempty"`

	// ToolUse contains the tool calls made by the model
	ToolUse []ToolUse `json:"tool_use,omitempty"`

	// StopReason indicates why the model stopped
	// - "end_turn": Model finished naturally
	// - "tool_use": Model wants to use a tool
	// - "max_tokens": Hit token limit
	// - "stop_sequence": Hit a stop sequence
	StopReason string `json:"stop_reason"`

	// TokensUsed is the total tokens used in this request
	TokensUsed int `json:"tokens_used"`

	// Model is the model that was used
	Model string `json:"model"`
}

// ToolUse represents a tool call from the model.
type ToolUse struct {
	// ID is a unique identifier for this tool call
	ID string `json:"id"`

	// Name is the name of the tool to call
	Name string `json:"name"`

	// Input contains the arguments to pass to the tool
	Input map[string]interface{} `json:"input"`
}

// ToolResultMessage creates a message containing tool results to send back to the model.
type ToolResultMessage struct {
	// ToolUseID is the ID of the tool call this is responding to
	ToolUseID string `json:"tool_use_id"`

	// Content is the result of the tool call
	Content interface{} `json:"content"`

	// IsError indicates if the tool call resulted in an error
	IsError bool `json:"is_error,omitempty"`
}

// HasToolUse returns true if the response contains tool use blocks.
func (r *ToolUseResponse) HasToolUse() bool {
	return len(r.ToolUse) > 0
}

// ValidateToolUseRequest validates the request and applies defaults.
func ValidateToolUseRequest(req *ToolUseRequest, defaultModel string, defaultMaxTokens int) (*ToolUseRequest, error) {
	normalized := req.WithDefaults(defaultModel, defaultMaxTokens)
	if err := normalized.Validate(); err != nil {
		return nil, err
	}
	return normalized, nil
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
