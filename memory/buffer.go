package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// ConversationBufferMemory stores the full conversation history in memory.
// It's the simplest memory type that remembers all previous interactions.
type ConversationBufferMemory struct {
	mu      sync.RWMutex
	history *InMemoryChatMessageHistory
	config  MemoryConfig
}

// ConversationBufferMemoryConfig configures the conversation buffer memory
type ConversationBufferMemoryConfig struct {
	MemoryConfig
}

// NewConversationBufferMemory creates a new conversation buffer memory
func NewConversationBufferMemory(cfg ...ConversationBufferMemoryConfig) *ConversationBufferMemory {
	config := DefaultMemoryConfig()
	if len(cfg) > 0 {
		if cfg[0].MemoryKey != "" {
			config.MemoryKey = cfg[0].MemoryKey
		}
		if cfg[0].InputKey != "" {
			config.InputKey = cfg[0].InputKey
		}
		if cfg[0].OutputKey != "" {
			config.OutputKey = cfg[0].OutputKey
		}
		if cfg[0].HumanPrefix != "" {
			config.HumanPrefix = cfg[0].HumanPrefix
		}
		if cfg[0].AIPrefix != "" {
			config.AIPrefix = cfg[0].AIPrefix
		}
		config.ReturnMessages = cfg[0].ReturnMessages
	}

	return &ConversationBufferMemory{
		history: NewInMemoryChatMessageHistory(),
		config:  config,
	}
}

// LoadMemoryVariables returns the conversation history as a variable
func (m *ConversationBufferMemory) LoadMemoryVariables(ctx context.Context) (map[string]any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	messages, err := m.history.Messages(ctx)
	if err != nil {
		return nil, err
	}

	if m.config.ReturnMessages {
		return map[string]any{
			m.config.MemoryKey: messages,
		}, nil
	}

	// Format as string
	formatted := m.formatMessages(messages)
	return map[string]any{
		m.config.MemoryKey: formatted,
	}, nil
}

// SaveContext saves the input/output from a chain run
func (m *ConversationBufferMemory) SaveContext(ctx context.Context, inputs map[string]any, outputs map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get input message
	inputVal, ok := inputs[m.config.InputKey]
	if ok {
		inputStr, _ := inputVal.(string)
		if inputStr != "" {
			if err := m.history.AddUserMessage(ctx, inputStr); err != nil {
				return err
			}
		}
	}

	// Get output message
	outputVal, ok := outputs[m.config.OutputKey]
	if ok {
		outputStr, _ := outputVal.(string)
		if outputStr != "" {
			if err := m.history.AddAIMessage(ctx, outputStr); err != nil {
				return err
			}
		}
	}

	return nil
}

// Clear clears all memory
func (m *ConversationBufferMemory) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.history.Clear(ctx)
}

// MemoryVariables returns the keys this memory adds to chain inputs
func (m *ConversationBufferMemory) MemoryVariables() []string {
	return []string{m.config.MemoryKey}
}

// ChatHistory returns the underlying chat message history
func (m *ConversationBufferMemory) ChatHistory() ChatMessageHistory {
	return m.history
}

// formatMessages formats messages as a string
func (m *ConversationBufferMemory) formatMessages(messages []ChatMessage) string {
	var parts []string
	for _, msg := range messages {
		prefix := m.getPrefixForRole(msg.Role)
		parts = append(parts, fmt.Sprintf("%s: %s", prefix, msg.Content))
	}
	return strings.Join(parts, "\n")
}

func (m *ConversationBufferMemory) getPrefixForRole(role MessageRole) string {
	switch role {
	case RoleHuman:
		return m.config.HumanPrefix
	case RoleAI:
		return m.config.AIPrefix
	case RoleSystem:
		return "System"
	case RoleFunction:
		return "Function"
	default:
		return string(role)
	}
}

// ConversationBufferWindowMemory stores only the last K conversation turns.
// This is useful for limiting context size while maintaining recent history.
type ConversationBufferWindowMemory struct {
	*ConversationBufferMemory
	k int // Number of conversation turns to keep
}

// NewConversationBufferWindowMemory creates a new windowed buffer memory
func NewConversationBufferWindowMemory(k int, cfg ...ConversationBufferMemoryConfig) *ConversationBufferWindowMemory {
	if k <= 0 {
		k = 5
	}
	return &ConversationBufferWindowMemory{
		ConversationBufferMemory: NewConversationBufferMemory(cfg...),
		k:                        k,
	}
}

// LoadMemoryVariables returns only the last K conversation turns
func (m *ConversationBufferWindowMemory) LoadMemoryVariables(ctx context.Context) (map[string]any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	messages, err := m.history.Messages(ctx)
	if err != nil {
		return nil, err
	}

	// Keep only last k*2 messages (k turns = k human + k AI messages)
	maxMessages := m.k * 2
	if len(messages) > maxMessages {
		messages = messages[len(messages)-maxMessages:]
	}

	if m.config.ReturnMessages {
		return map[string]any{
			m.config.MemoryKey: messages,
		}, nil
	}

	formatted := m.formatMessages(messages)
	return map[string]any{
		m.config.MemoryKey: formatted,
	}, nil
}

// ConversationSummaryMemory summarizes the conversation to save tokens.
// It uses an LLM to create running summaries of the conversation.
type ConversationSummaryMemory struct {
	mu            sync.RWMutex
	history       *InMemoryChatMessageHistory
	config        MemoryConfig
	summary       string
	summarizeFunc SummarizeFunc
}

// SummarizeFunc is a function that summarizes conversation history
type SummarizeFunc func(ctx context.Context, existingSummary string, newMessages []ChatMessage) (string, error)

// ConversationSummaryMemoryConfig configures summary memory
type ConversationSummaryMemoryConfig struct {
	MemoryConfig
	SummarizeFunc SummarizeFunc
}

// NewConversationSummaryMemory creates a new conversation summary memory
func NewConversationSummaryMemory(cfg ConversationSummaryMemoryConfig) *ConversationSummaryMemory {
	config := DefaultMemoryConfig()
	if cfg.MemoryKey != "" {
		config.MemoryKey = cfg.MemoryKey
	}
	if cfg.InputKey != "" {
		config.InputKey = cfg.InputKey
	}
	if cfg.OutputKey != "" {
		config.OutputKey = cfg.OutputKey
	}

	return &ConversationSummaryMemory{
		history:       NewInMemoryChatMessageHistory(),
		config:        config,
		summarizeFunc: cfg.SummarizeFunc,
	}
}

// LoadMemoryVariables returns the conversation summary
func (m *ConversationSummaryMemory) LoadMemoryVariables(ctx context.Context) (map[string]any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]any{
		m.config.MemoryKey: m.summary,
	}, nil
}

// SaveContext saves and summarizes the conversation
func (m *ConversationSummaryMemory) SaveContext(ctx context.Context, inputs map[string]any, outputs map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var newMessages []ChatMessage

	// Get input message
	if inputVal, ok := inputs[m.config.InputKey]; ok {
		if inputStr, ok := inputVal.(string); ok && inputStr != "" {
			msg := NewHumanMessage(inputStr)
			m.history.AddMessage(ctx, msg)
			newMessages = append(newMessages, msg)
		}
	}

	// Get output message
	if outputVal, ok := outputs[m.config.OutputKey]; ok {
		if outputStr, ok := outputVal.(string); ok && outputStr != "" {
			msg := NewAIMessage(outputStr)
			m.history.AddMessage(ctx, msg)
			newMessages = append(newMessages, msg)
		}
	}

	// Update summary if we have a summarize function
	if m.summarizeFunc != nil && len(newMessages) > 0 {
		newSummary, err := m.summarizeFunc(ctx, m.summary, newMessages)
		if err != nil {
			return err
		}
		m.summary = newSummary
	}

	return nil
}

// Clear clears all memory including summary
func (m *ConversationSummaryMemory) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.summary = ""
	return m.history.Clear(ctx)
}

// MemoryVariables returns the keys this memory adds
func (m *ConversationSummaryMemory) MemoryVariables() []string {
	return []string{m.config.MemoryKey}
}

// GetSummary returns the current summary
func (m *ConversationSummaryMemory) GetSummary() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.summary
}
