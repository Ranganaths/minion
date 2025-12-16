package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Session represents a single conversation context
// A session is the container for a continuous conversation, holding:
// - Immediate dialogue history (turn-by-turn record)
// - Working memory/scratchpad (e.g., shopping cart items)
// Purpose: Provide short-term context for coherent dialogue within a single interaction
type Session struct {
	ID        string                 `json:"id"`
	AgentID   string                 `json:"agent_id"`
	UserID    string                 `json:"user_id,omitempty"`
	Status    SessionStatus          `json:"status"`
	History   []Message              `json:"history"`
	Workspace map[string]interface{} `json:"workspace"` // Working memory/scratchpad
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	ExpiresAt time.Time              `json:"expires_at"`
}

// SessionStatus represents the status of a session
type SessionStatus string

const (
	SessionStatusActive   SessionStatus = "active"
	SessionStatusClosed   SessionStatus = "closed"
	SessionStatusExpired  SessionStatus = "expired"
	SessionStatusArchived SessionStatus = "archived"
)

// Message represents a single message in the conversation
type Message struct {
	ID        string                 `json:"id"`
	Role      MessageRole            `json:"role"`  // user, assistant, system, tool
	Content   string                 `json:"content"`
	ToolCalls []ToolCall             `json:"tool_calls,omitempty"`
	ToolCallID string                `json:"tool_call_id,omitempty"` // For tool responses
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// MessageRole defines the role of a message
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
	MessageRoleTool      MessageRole = "tool"
)

// ToolCall represents a tool call made by the assistant
type ToolCall struct {
	ID       string                 `json:"id"`
	ToolName string                 `json:"tool_name"`
	Input    map[string]interface{} `json:"input"`
}

// SessionManager manages session lifecycle
type SessionManager interface {
	// Create creates a new session
	Create(ctx context.Context, agentID, userID string, timeout time.Duration) (*Session, error)

	// Get retrieves a session by ID
	Get(ctx context.Context, sessionID string) (*Session, error)

	// Append adds a message to the session history
	Append(ctx context.Context, sessionID string, message Message) error

	// SetWorkspace updates the workspace (working memory)
	SetWorkspace(ctx context.Context, sessionID string, key string, value interface{}) error

	// GetWorkspace retrieves a value from the workspace
	GetWorkspace(ctx context.Context, sessionID string, key string) (interface{}, error)

	// GetHistory retrieves the conversation history (with optional limit)
	GetHistory(ctx context.Context, sessionID string, limit int) ([]Message, error)

	// Close closes a session (marks as completed)
	Close(ctx context.Context, sessionID string) error

	// Delete permanently deletes a session
	Delete(ctx context.Context, sessionID string) error

	// CleanupExpired removes expired sessions
	CleanupExpired(ctx context.Context) (int, error)

	// List lists sessions for an agent or user
	List(ctx context.Context, filters SessionFilters) ([]*Session, error)
}

// SessionFilters contains filters for listing sessions
type SessionFilters struct {
	AgentID string
	UserID  string
	Status  SessionStatus
	Limit   int
	Offset  int
}

// InMemorySessionManager is an in-memory implementation of SessionManager
// For production, use a persistent store (PostgreSQL, Redis, etc.)
type InMemorySessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewInMemorySessionManager creates a new in-memory session manager
func NewInMemorySessionManager() *InMemorySessionManager {
	return &InMemorySessionManager{
		sessions: make(map[string]*Session),
	}
}

func (m *InMemorySessionManager) Create(ctx context.Context, agentID, userID string, timeout time.Duration) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := &Session{
		ID:        uuid.New().String(),
		AgentID:   agentID,
		UserID:    userID,
		Status:    SessionStatusActive,
		History:   []Message{},
		Workspace: make(map[string]interface{}),
		Metadata:  make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(timeout),
	}

	m.sessions[session.ID] = session
	return session, nil
}

func (m *InMemorySessionManager) Get(ctx context.Context, sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Check if expired
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired: %s", sessionID)
	}

	return session, nil
}

func (m *InMemorySessionManager) Append(ctx context.Context, sessionID string, message Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status != SessionStatusActive {
		return fmt.Errorf("cannot append to non-active session: %s", sessionID)
	}

	// Set message ID and timestamp if not provided
	if message.ID == "" {
		message.ID = uuid.New().String()
	}
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	session.History = append(session.History, message)
	session.UpdatedAt = time.Now()

	return nil
}

func (m *InMemorySessionManager) SetWorkspace(ctx context.Context, sessionID string, key string, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.Workspace[key] = value
	session.UpdatedAt = time.Now()

	return nil
}

func (m *InMemorySessionManager) GetWorkspace(ctx context.Context, sessionID string, key string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	value, ok := session.Workspace[key]
	if !ok {
		return nil, fmt.Errorf("workspace key not found: %s", key)
	}

	return value, nil
}

func (m *InMemorySessionManager) GetHistory(ctx context.Context, sessionID string, limit int) ([]Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	history := session.History
	if limit > 0 && len(history) > limit {
		// Return the last N messages
		history = history[len(history)-limit:]
	}

	return history, nil
}

func (m *InMemorySessionManager) Close(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.Status = SessionStatusClosed
	session.UpdatedAt = time.Now()

	return nil
}

func (m *InMemorySessionManager) Delete(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
	return nil
}

func (m *InMemorySessionManager) CleanupExpired(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	now := time.Now()

	for id, session := range m.sessions {
		if now.After(session.ExpiresAt) {
			session.Status = SessionStatusExpired
			delete(m.sessions, id)
			count++
		}
	}

	return count, nil
}

func (m *InMemorySessionManager) List(ctx context.Context, filters SessionFilters) ([]*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Session

	for _, session := range m.sessions {
		// Apply filters
		if filters.AgentID != "" && session.AgentID != filters.AgentID {
			continue
		}
		if filters.UserID != "" && session.UserID != filters.UserID {
			continue
		}
		if filters.Status != "" && session.Status != filters.Status {
			continue
		}

		results = append(results, session)
	}

	// Apply pagination
	if filters.Offset > 0 {
		if filters.Offset >= len(results) {
			return []*Session{}, nil
		}
		results = results[filters.Offset:]
	}

	if filters.Limit > 0 && len(results) > filters.Limit {
		results = results[:filters.Limit]
	}

	return results, nil
}

// MarshalJSON custom JSON marshaling for Session
func (s *Session) MarshalJSON() ([]byte, error) {
	type Alias Session
	return json.Marshal(&struct {
		*Alias
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		ExpiresAt string `json:"expires_at"`
	}{
		Alias:     (*Alias)(s),
		CreatedAt: s.CreatedAt.Format(time.RFC3339),
		UpdatedAt: s.UpdatedAt.Format(time.RFC3339),
		ExpiresAt: s.ExpiresAt.Format(time.RFC3339),
	})
}

// GetHistorySummary returns a concise summary of the session history
// Useful for reducing context window usage
func (s *Session) GetHistorySummary(maxMessages int) []Message {
	if len(s.History) <= maxMessages {
		return s.History
	}

	// Return the first message (usually system) + last N messages
	summary := make([]Message, 0, maxMessages)
	if len(s.History) > 0 {
		summary = append(summary, s.History[0]) // System message
	}

	startIdx := len(s.History) - (maxMessages - 1)
	if startIdx < 1 {
		startIdx = 1
	}

	summary = append(summary, s.History[startIdx:]...)
	return summary
}
