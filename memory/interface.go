// Package memory provides conversation memory management for chains.
// Memory stores and retrieves conversation history for context-aware interactions.
package memory

import (
	"context"
)

// Memory is the interface for conversation memory management.
// It stores conversation history and provides it as context for chains.
type Memory interface {
	// LoadMemoryVariables returns the memory variables to inject into chain inputs
	LoadMemoryVariables(ctx context.Context) (map[string]any, error)

	// SaveContext saves the input/output context from a chain run
	SaveContext(ctx context.Context, inputs map[string]any, outputs map[string]any) error

	// Clear clears all memory contents
	Clear(ctx context.Context) error

	// MemoryVariables returns the keys this memory will add to chain inputs
	MemoryVariables() []string
}

// ChatMessageHistory stores a sequence of chat messages
type ChatMessageHistory interface {
	// AddUserMessage adds a user message to history
	AddUserMessage(ctx context.Context, message string) error

	// AddAIMessage adds an AI message to history
	AddAIMessage(ctx context.Context, message string) error

	// AddMessage adds a message with specified role
	AddMessage(ctx context.Context, message ChatMessage) error

	// Messages returns all messages in history
	Messages(ctx context.Context) ([]ChatMessage, error)

	// Clear removes all messages from history
	Clear(ctx context.Context) error
}

// ChatMessage represents a single message in a conversation
type ChatMessage struct {
	// Role is the message sender role (human, ai, system, function)
	Role MessageRole

	// Content is the message text content
	Content string

	// Name is an optional name for the sender
	Name string

	// Metadata contains additional message data
	Metadata map[string]any
}

// MessageRole represents the role of a message sender
type MessageRole string

const (
	// RoleHuman represents a human/user message
	RoleHuman MessageRole = "human"

	// RoleAI represents an AI/assistant message
	RoleAI MessageRole = "ai"

	// RoleSystem represents a system message
	RoleSystem MessageRole = "system"

	// RoleFunction represents a function/tool result
	RoleFunction MessageRole = "function"
)

// NewHumanMessage creates a new human message
func NewHumanMessage(content string) ChatMessage {
	return ChatMessage{Role: RoleHuman, Content: content}
}

// NewAIMessage creates a new AI message
func NewAIMessage(content string) ChatMessage {
	return ChatMessage{Role: RoleAI, Content: content}
}

// NewSystemMessage creates a new system message
func NewSystemMessage(content string) ChatMessage {
	return ChatMessage{Role: RoleSystem, Content: content}
}

// NewFunctionMessage creates a new function result message
func NewFunctionMessage(name, content string) ChatMessage {
	return ChatMessage{Role: RoleFunction, Content: content, Name: name}
}

// MemoryConfig holds common configuration for memory implementations
type MemoryConfig struct {
	// MemoryKey is the key used to store memory in chain inputs (default: "history")
	MemoryKey string

	// InputKey is the key for human input in chain inputs (default: "input")
	InputKey string

	// OutputKey is the key for AI output in chain outputs (default: "output")
	OutputKey string

	// ReturnMessages returns messages instead of formatted string
	ReturnMessages bool

	// HumanPrefix is the prefix for human messages (default: "Human")
	HumanPrefix string

	// AIPrefix is the prefix for AI messages (default: "AI")
	AIPrefix string
}

// DefaultMemoryConfig returns default memory configuration
func DefaultMemoryConfig() MemoryConfig {
	return MemoryConfig{
		MemoryKey:      "history",
		InputKey:       "input",
		OutputKey:      "output",
		ReturnMessages: false,
		HumanPrefix:    "Human",
		AIPrefix:       "AI",
	}
}
