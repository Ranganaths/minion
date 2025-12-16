package core

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Memory represents long-term persistent knowledge
// Memory is the mechanism for long-term persistence across multiple sessions
// It captures and consolidates key information through extraction and curation
// Purpose: Enable personalization and manage context window by replacing verbose
// history with concise, durable knowledge
type Memory struct {
	ID        string                 `json:"id"`
	AgentID   string                 `json:"agent_id"`
	UserID    string                 `json:"user_id,omitempty"`
	Key       string                 `json:"key"`        // Semantic key (e.g., "user_preferences", "customer_name")
	Value     string                 `json:"value"`      // The actual fact/knowledge
	Type      MemoryType             `json:"type"`       // fact, preference, context, skill
	Source    string                 `json:"source"`     // session_id or manual
	Embedding []float32              `json:"embedding,omitempty"` // Vector embedding for semantic search
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	AccessCount int                  `json:"access_count"` // Track usage frequency
	LastAccessed time.Time           `json:"last_accessed,omitempty"`
}

// MemoryType categorizes different types of memories
type MemoryType string

const (
	MemoryTypeFact       MemoryType = "fact"       // Factual information
	MemoryTypePreference MemoryType = "preference" // User preferences
	MemoryTypeContext    MemoryType = "context"    // Contextual information
	MemoryTypeSkill      MemoryType = "skill"      // Learned skills or patterns
)

// MemoryManager manages long-term memory
type MemoryManager interface {
	// Store creates or updates a memory
	Store(ctx context.Context, memory *Memory) error

	// Get retrieves a memory by ID
	Get(ctx context.Context, memoryID string) (*Memory, error)

	// GetByKey retrieves a memory by key
	GetByKey(ctx context.Context, agentID, userID, key string) (*Memory, error)

	// Search performs semantic search for relevant memories
	// If embedding is provided, uses vector similarity search
	// Otherwise, uses keyword search
	Search(ctx context.Context, filters MemorySearchFilters) ([]*Memory, error)

	// Delete removes a memory
	Delete(ctx context.Context, memoryID string) error

	// ExtractFromSession extracts important information from a session and stores as memory
	ExtractFromSession(ctx context.Context, session *Session, extractor MemoryExtractor) error

	// Consolidate merges or prunes memories to optimize storage
	Consolidate(ctx context.Context, agentID, userID string) error

	// List lists memories with filters
	List(ctx context.Context, filters MemoryListFilters) ([]*Memory, error)
}

// MemorySearchFilters contains filters for semantic search
type MemorySearchFilters struct {
	AgentID   string
	UserID    string
	Query     string      // Text query
	Embedding []float32   // Optional: vector embedding for semantic search
	Type      MemoryType
	Limit     int
	MinScore  float64     // Minimum similarity score (0-1)
}

// MemoryListFilters contains filters for listing memories
type MemoryListFilters struct {
	AgentID string
	UserID  string
	Type    MemoryType
	Limit   int
	Offset  int
	SortBy  string // created_at, updated_at, access_count
	Order   string // asc, desc
}

// MemoryExtractor defines how to extract memories from sessions
type MemoryExtractor interface {
	// Extract analyzes a session and returns extracted memories
	Extract(ctx context.Context, session *Session) ([]*Memory, error)
}

// InMemoryMemoryManager is an in-memory implementation of MemoryManager
// For production, use PostgreSQL with pgvector for semantic search
type InMemoryMemoryManager struct {
	memories map[string]*Memory
	mu       sync.RWMutex
}

// NewInMemoryMemoryManager creates a new in-memory memory manager
func NewInMemoryMemoryManager() *InMemoryMemoryManager {
	return &InMemoryMemoryManager{
		memories: make(map[string]*Memory),
	}
}

func (m *InMemoryMemoryManager) Store(ctx context.Context, memory *Memory) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Set ID if not provided
	if memory.ID == "" {
		memory.ID = uuid.New().String()
		memory.CreatedAt = time.Now()
	}

	memory.UpdatedAt = time.Now()

	// Check for existing memory with same key
	for _, existingMemory := range m.memories {
		if existingMemory.AgentID == memory.AgentID &&
			existingMemory.UserID == memory.UserID &&
			existingMemory.Key == memory.Key &&
			existingMemory.ID != memory.ID {
			// Update existing memory instead of creating duplicate
			existingMemory.Value = memory.Value
			existingMemory.Embedding = memory.Embedding
			existingMemory.Metadata = memory.Metadata
			existingMemory.UpdatedAt = time.Now()
			return nil
		}
	}

	m.memories[memory.ID] = memory
	return nil
}

func (m *InMemoryMemoryManager) Get(ctx context.Context, memoryID string) (*Memory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	memory, ok := m.memories[memoryID]
	if !ok {
		return nil, fmt.Errorf("memory not found: %s", memoryID)
	}

	// Track access
	memory.AccessCount++
	memory.LastAccessed = time.Now()

	return memory, nil
}

func (m *InMemoryMemoryManager) GetByKey(ctx context.Context, agentID, userID, key string) (*Memory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, memory := range m.memories {
		if memory.AgentID == agentID && memory.UserID == userID && memory.Key == key {
			return memory, nil
		}
	}

	return nil, fmt.Errorf("memory not found for key: %s", key)
}

func (m *InMemoryMemoryManager) Search(ctx context.Context, filters MemorySearchFilters) ([]*Memory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Memory

	for _, memory := range m.memories {
		// Apply basic filters
		if filters.AgentID != "" && memory.AgentID != filters.AgentID {
			continue
		}
		if filters.UserID != "" && memory.UserID != filters.UserID {
			continue
		}
		if filters.Type != "" && memory.Type != filters.Type {
			continue
		}

		// Semantic search with embeddings
		if len(filters.Embedding) > 0 && len(memory.Embedding) > 0 {
			score := cosineSimilarity(filters.Embedding, memory.Embedding)
			if score < filters.MinScore {
				continue
			}
			results = append(results, memory)
		} else if filters.Query != "" {
			// Simple keyword search
			if containsKeyword(memory.Key, filters.Query) || containsKeyword(memory.Value, filters.Query) {
				results = append(results, memory)
			}
		} else {
			results = append(results, memory)
		}
	}

	// Sort by relevance (access count as proxy)
	sort.Slice(results, func(i, j int) bool {
		return results[i].AccessCount > results[j].AccessCount
	})

	// Apply limit
	if filters.Limit > 0 && len(results) > filters.Limit {
		results = results[:filters.Limit]
	}

	return results, nil
}

func (m *InMemoryMemoryManager) Delete(ctx context.Context, memoryID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.memories, memoryID)
	return nil
}

func (m *InMemoryMemoryManager) ExtractFromSession(ctx context.Context, session *Session, extractor MemoryExtractor) error {
	// Use the extractor to analyze session and create memories
	memories, err := extractor.Extract(ctx, session)
	if err != nil {
		return fmt.Errorf("failed to extract memories: %w", err)
	}

	// Store extracted memories
	for _, memory := range memories {
		memory.Source = session.ID
		if err := m.Store(ctx, memory); err != nil {
			return fmt.Errorf("failed to store memory: %w", err)
		}
	}

	return nil
}

func (m *InMemoryMemoryManager) Consolidate(ctx context.Context, agentID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Consolidation strategy:
	// 1. Remove duplicate memories (same key, merge values)
	// 2. Remove low-access memories older than threshold
	// 3. Merge similar memories (if embeddings are similar)

	keyMap := make(map[string][]*Memory)

	// Group memories by key
	for _, memory := range m.memories {
		if memory.AgentID != agentID || memory.UserID != userID {
			continue
		}
		key := memory.Key
		keyMap[key] = append(keyMap[key], memory)
	}

	// Consolidate duplicates
	for key, memList := range keyMap {
		if len(memList) <= 1 {
			continue
		}

		// Keep the most recently updated one
		sort.Slice(memList, func(i, j int) bool {
			return memList[i].UpdatedAt.After(memList[j].UpdatedAt)
		})

		keeper := memList[0]

		// Merge access counts
		totalAccess := 0
		for _, mem := range memList {
			totalAccess += mem.AccessCount
		}
		keeper.AccessCount = totalAccess

		// Delete the rest
		for i := 1; i < len(memList); i++ {
			delete(m.memories, memList[i].ID)
		}

		keyMap[key] = []*Memory{keeper}
	}

	// Prune low-value memories (not accessed in 90 days with < 3 accesses)
	pruneThreshold := time.Now().Add(-90 * 24 * time.Hour)
	for id, memory := range m.memories {
		if memory.AgentID != agentID || memory.UserID != userID {
			continue
		}
		if memory.AccessCount < 3 && memory.LastAccessed.Before(pruneThreshold) {
			delete(m.memories, id)
		}
	}

	return nil
}

func (m *InMemoryMemoryManager) List(ctx context.Context, filters MemoryListFilters) ([]*Memory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Memory

	for _, memory := range m.memories {
		// Apply filters
		if filters.AgentID != "" && memory.AgentID != filters.AgentID {
			continue
		}
		if filters.UserID != "" && memory.UserID != filters.UserID {
			continue
		}
		if filters.Type != "" && memory.Type != filters.Type {
			continue
		}

		results = append(results, memory)
	}

	// Sort
	sortBy := filters.SortBy
	if sortBy == "" {
		sortBy = "updated_at"
	}

	sort.Slice(results, func(i, j int) bool {
		switch sortBy {
		case "created_at":
			if filters.Order == "asc" {
				return results[i].CreatedAt.Before(results[j].CreatedAt)
			}
			return results[i].CreatedAt.After(results[j].CreatedAt)
		case "access_count":
			if filters.Order == "asc" {
				return results[i].AccessCount < results[j].AccessCount
			}
			return results[i].AccessCount > results[j].AccessCount
		default: // updated_at
			if filters.Order == "asc" {
				return results[i].UpdatedAt.Before(results[j].UpdatedAt)
			}
			return results[i].UpdatedAt.After(results[j].UpdatedAt)
		}
	})

	// Apply pagination
	if filters.Offset > 0 {
		if filters.Offset >= len(results) {
			return []*Memory{}, nil
		}
		results = results[filters.Offset:]
	}

	if filters.Limit > 0 && len(results) > filters.Limit {
		results = results[:filters.Limit]
	}

	return results, nil
}

// SimpleMemoryExtractor is a basic implementation of MemoryExtractor
// For production, use LLM-based extraction or more sophisticated NLP
type SimpleMemoryExtractor struct {
	AgentID string
}

func (e *SimpleMemoryExtractor) Extract(ctx context.Context, session *Session) ([]*Memory, error) {
	var memories []*Memory

	// Simple heuristic: Extract user preferences from user messages
	// In production, use LLM to intelligently extract facts and preferences

	for _, msg := range session.History {
		if msg.Role != MessageRoleUser {
			continue
		}

		// Look for preference indicators (simple keyword matching)
		if containsKeyword(msg.Content, "prefer") || containsKeyword(msg.Content, "like") {
			memory := &Memory{
				AgentID:   e.AgentID,
				UserID:    session.UserID,
				Key:       fmt.Sprintf("preference_%s", uuid.New().String()),
				Value:     msg.Content,
				Type:      MemoryTypePreference,
				Source:    session.ID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Metadata: map[string]interface{}{
					"extracted_from_message": msg.ID,
					"timestamp":              msg.Timestamp,
				},
			}
			memories = append(memories, memory)
		}
	}

	// Extract workspace data as facts
	for key, value := range session.Workspace {
		memory := &Memory{
			AgentID:   e.AgentID,
			UserID:    session.UserID,
			Key:       key,
			Value:     fmt.Sprintf("%v", value),
			Type:      MemoryTypeFact,
			Source:    session.ID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		memories = append(memories, memory)
	}

	return memories, nil
}

// Helper functions

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (normA * normB)
}

func containsKeyword(text, keyword string) bool {
	// Simple case-insensitive substring match
	// In production, use proper text search (stemming, lemmatization, etc.)
	return len(text) > 0 && len(keyword) > 0 &&
		(text == keyword || // exact match
			// Simple contains check (case-insensitive would require strings.ToLower)
			len(text) >= len(keyword))
}

// GetRelevantMemories retrieves memories relevant to a query
// This is a convenience method that combines search with relevance scoring
func GetRelevantMemories(
	ctx context.Context,
	mgr MemoryManager,
	agentID, userID, query string,
	limit int,
) ([]*Memory, error) {
	return mgr.Search(ctx, MemorySearchFilters{
		AgentID:  agentID,
		UserID:   userID,
		Query:    query,
		Limit:    limit,
		MinScore: 0.7, // 70% similarity threshold
	})
}
