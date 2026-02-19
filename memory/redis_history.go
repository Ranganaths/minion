// Package memory provides conversation memory management for chains.
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// RedisListClient extends basic Redis operations with list commands.
// This is an optional interface for Redis-backed chat history.
// Users can implement this interface to enable distributed conversation history.
//
// Note: This interface is separate from cache.RedisClient to avoid breaking
// existing implementations. If you already have a cache.RedisClient, you can
// create a wrapper that also implements the list operations.
type RedisListClient interface {
	// Basic operations (compatible with cache.RedisClient)
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Expire(ctx context.Context, key string, expiration time.Duration) error
	Close() error

	// List operations (additional for chat history)
	LPush(ctx context.Context, key string, values ...string) error
	RPush(ctx context.Context, key string, values ...string) error
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	LLen(ctx context.Context, key string) (int64, error)
	LTrim(ctx context.Context, key string, start, stop int64) error
}

// RedisChatMessageHistory stores chat messages in Redis for distributed access.
// It implements the ChatMessageHistory interface using Redis lists.
type RedisChatMessageHistory struct {
	client    RedisListClient
	sessionID string
	keyPrefix string
	ttl       time.Duration
	maxLength int // Maximum number of messages to keep (0 = unlimited)
}

// RedisChatMessageHistoryConfig configures the Redis chat message history
type RedisChatMessageHistoryConfig struct {
	// Client is the Redis client to use (must implement RedisListClient)
	Client RedisListClient

	// SessionID uniquely identifies this conversation session
	SessionID string

	// KeyPrefix is the prefix for Redis keys (default: "chat_history:")
	KeyPrefix string

	// TTL is the time-to-live for the history (default: 24 hours, 0 = no expiration)
	TTL time.Duration

	// MaxLength is the maximum number of messages to keep (default: 0 = unlimited)
	MaxLength int
}

// NewRedisChatMessageHistory creates a new Redis-backed chat message history
func NewRedisChatMessageHistory(config RedisChatMessageHistoryConfig) (*RedisChatMessageHistory, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("redis client is required")
	}
	if config.SessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	if config.KeyPrefix == "" {
		config.KeyPrefix = "chat_history:"
	}
	if config.TTL == 0 {
		config.TTL = 24 * time.Hour
	}

	return &RedisChatMessageHistory{
		client:    config.Client,
		sessionID: config.SessionID,
		keyPrefix: config.KeyPrefix,
		ttl:       config.TTL,
		maxLength: config.MaxLength,
	}, nil
}

// key returns the Redis key for this session's history
func (h *RedisChatMessageHistory) key() string {
	return h.keyPrefix + h.sessionID
}

// AddUserMessage adds a user/human message to the history
func (h *RedisChatMessageHistory) AddUserMessage(ctx context.Context, message string) error {
	return h.AddMessage(ctx, NewHumanMessage(message))
}

// AddAIMessage adds an AI/assistant message to the history
func (h *RedisChatMessageHistory) AddAIMessage(ctx context.Context, message string) error {
	return h.AddMessage(ctx, NewAIMessage(message))
}

// AddMessage adds any message type to the history
func (h *RedisChatMessageHistory) AddMessage(ctx context.Context, message ChatMessage) error {
	// Serialize the message
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	key := h.key()

	// Push to the end of the list
	if err := h.client.RPush(ctx, key, string(data)); err != nil {
		return fmt.Errorf("failed to add message to Redis: %w", err)
	}

	// Trim if max length is set
	if h.maxLength > 0 {
		if err := h.client.LTrim(ctx, key, int64(-h.maxLength), -1); err != nil {
			return fmt.Errorf("failed to trim history: %w", err)
		}
	}

	// Set/refresh TTL
	if h.ttl > 0 {
		if err := h.client.Expire(ctx, key, h.ttl); err != nil {
			return fmt.Errorf("failed to set TTL: %w", err)
		}
	}

	return nil
}

// Messages returns all messages in the history
func (h *RedisChatMessageHistory) Messages(ctx context.Context) ([]ChatMessage, error) {
	key := h.key()

	// Get all messages from the list
	data, err := h.client.LRange(ctx, key, 0, -1)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages from Redis: %w", err)
	}

	messages := make([]ChatMessage, 0, len(data))
	for _, d := range data {
		var msg ChatMessage
		if err := json.Unmarshal([]byte(d), &msg); err != nil {
			// Skip malformed messages
			continue
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// Clear removes all messages from the history
func (h *RedisChatMessageHistory) Clear(ctx context.Context) error {
	if err := h.client.Del(ctx, h.key()); err != nil {
		return fmt.Errorf("failed to clear history: %w", err)
	}
	return nil
}

// Len returns the number of messages in the history
func (h *RedisChatMessageHistory) Len(ctx context.Context) (int64, error) {
	return h.client.LLen(ctx, h.key())
}

// LastMessages returns the last n messages
func (h *RedisChatMessageHistory) LastMessages(ctx context.Context, n int) ([]ChatMessage, error) {
	key := h.key()

	// Get last n messages
	data, err := h.client.LRange(ctx, key, int64(-n), -1)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages from Redis: %w", err)
	}

	messages := make([]ChatMessage, 0, len(data))
	for _, d := range data {
		var msg ChatMessage
		if err := json.Unmarshal([]byte(d), &msg); err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// SetTTL updates the TTL for this history
func (h *RedisChatMessageHistory) SetTTL(ctx context.Context, ttl time.Duration) error {
	h.ttl = ttl
	if ttl > 0 {
		return h.client.Expire(ctx, h.key(), ttl)
	}
	return nil
}

// SessionID returns the session ID for this history
func (h *RedisChatMessageHistory) SessionID() string {
	return h.sessionID
}

// RedisConversationBufferMemory wraps RedisChatMessageHistory to implement the Memory interface
type RedisConversationBufferMemory struct {
	history *RedisChatMessageHistory
	config  MemoryConfig
}

// RedisConversationBufferMemoryConfig configures the Redis conversation buffer memory
type RedisConversationBufferMemoryConfig struct {
	MemoryConfig
	RedisChatMessageHistoryConfig
}

// NewRedisConversationBufferMemory creates a new Redis-backed conversation buffer memory
func NewRedisConversationBufferMemory(config RedisConversationBufferMemoryConfig) (*RedisConversationBufferMemory, error) {
	history, err := NewRedisChatMessageHistory(config.RedisChatMessageHistoryConfig)
	if err != nil {
		return nil, err
	}

	memConfig := DefaultMemoryConfig()
	if config.MemoryKey != "" {
		memConfig.MemoryKey = config.MemoryKey
	}
	if config.InputKey != "" {
		memConfig.InputKey = config.InputKey
	}
	if config.OutputKey != "" {
		memConfig.OutputKey = config.OutputKey
	}
	if config.HumanPrefix != "" {
		memConfig.HumanPrefix = config.HumanPrefix
	}
	if config.AIPrefix != "" {
		memConfig.AIPrefix = config.AIPrefix
	}
	memConfig.ReturnMessages = config.ReturnMessages

	return &RedisConversationBufferMemory{
		history: history,
		config:  memConfig,
	}, nil
}

// LoadMemoryVariables returns the conversation history as a variable
func (m *RedisConversationBufferMemory) LoadMemoryVariables(ctx context.Context) (map[string]any, error) {
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
	formatted := formatMessagesWithConfig(messages, m.config)
	return map[string]any{
		m.config.MemoryKey: formatted,
	}, nil
}

// SaveContext saves the input/output from a chain run
func (m *RedisConversationBufferMemory) SaveContext(ctx context.Context, inputs map[string]any, outputs map[string]any) error {
	// Get input message
	if inputVal, ok := inputs[m.config.InputKey]; ok {
		if inputStr, ok := inputVal.(string); ok && inputStr != "" {
			if err := m.history.AddUserMessage(ctx, inputStr); err != nil {
				return err
			}
		}
	}

	// Get output message
	if outputVal, ok := outputs[m.config.OutputKey]; ok {
		if outputStr, ok := outputVal.(string); ok && outputStr != "" {
			if err := m.history.AddAIMessage(ctx, outputStr); err != nil {
				return err
			}
		}
	}

	return nil
}

// Clear clears all memory
func (m *RedisConversationBufferMemory) Clear(ctx context.Context) error {
	return m.history.Clear(ctx)
}

// MemoryVariables returns the keys this memory adds to chain inputs
func (m *RedisConversationBufferMemory) MemoryVariables() []string {
	return []string{m.config.MemoryKey}
}

// ChatHistory returns the underlying chat message history
func (m *RedisConversationBufferMemory) ChatHistory() ChatMessageHistory {
	return m.history
}

// formatMessagesWithConfig formats messages as a string using the provided config
func formatMessagesWithConfig(messages []ChatMessage, config MemoryConfig) string {
	var result string
	for i, msg := range messages {
		if i > 0 {
			result += "\n"
		}
		prefix := config.HumanPrefix
		switch msg.Role {
		case RoleAI:
			prefix = config.AIPrefix
		case RoleSystem:
			prefix = "System"
		case RoleFunction:
			prefix = "Function"
		}
		result += fmt.Sprintf("%s: %s", prefix, msg.Content)
	}
	return result
}

// Ensure interfaces are implemented
var _ ChatMessageHistory = (*RedisChatMessageHistory)(nil)
var _ Memory = (*RedisConversationBufferMemory)(nil)
