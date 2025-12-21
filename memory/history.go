package memory

import (
	"context"
	"sync"
)

// InMemoryChatMessageHistory stores chat messages in memory
type InMemoryChatMessageHistory struct {
	mu       sync.RWMutex
	messages []ChatMessage
}

// NewInMemoryChatMessageHistory creates a new in-memory chat history
func NewInMemoryChatMessageHistory() *InMemoryChatMessageHistory {
	return &InMemoryChatMessageHistory{
		messages: make([]ChatMessage, 0),
	}
}

// AddUserMessage adds a user/human message
func (h *InMemoryChatMessageHistory) AddUserMessage(ctx context.Context, message string) error {
	return h.AddMessage(ctx, NewHumanMessage(message))
}

// AddAIMessage adds an AI/assistant message
func (h *InMemoryChatMessageHistory) AddAIMessage(ctx context.Context, message string) error {
	return h.AddMessage(ctx, NewAIMessage(message))
}

// AddMessage adds any message type
func (h *InMemoryChatMessageHistory) AddMessage(ctx context.Context, message ChatMessage) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, message)
	return nil
}

// Messages returns all messages
func (h *InMemoryChatMessageHistory) Messages(ctx context.Context) ([]ChatMessage, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]ChatMessage, len(h.messages))
	copy(result, h.messages)
	return result, nil
}

// Clear removes all messages
func (h *InMemoryChatMessageHistory) Clear(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = make([]ChatMessage, 0)
	return nil
}

// Len returns the number of messages
func (h *InMemoryChatMessageHistory) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.messages)
}

// LastMessage returns the last message, if any
func (h *InMemoryChatMessageHistory) LastMessage() (ChatMessage, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.messages) == 0 {
		return ChatMessage{}, false
	}
	return h.messages[len(h.messages)-1], true
}

// FileChatMessageHistory stores chat messages in a file (placeholder for future implementation)
type FileChatMessageHistory struct {
	*InMemoryChatMessageHistory
	filePath string
}

// NewFileChatMessageHistory creates a new file-based chat history
func NewFileChatMessageHistory(filePath string) *FileChatMessageHistory {
	return &FileChatMessageHistory{
		InMemoryChatMessageHistory: NewInMemoryChatMessageHistory(),
		filePath:                   filePath,
	}
}
