package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ContextWindow manages the agent's active context
type ContextWindow struct {
	ID           string                 `json:"id"`
	AgentID      string                 `json:"agent_id"`
	MaxTokens    int                    `json:"max_tokens"`
	CurrentTokens int                   `json:"current_tokens"`
	Items        []*ContextItem         `json:"items"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// ContextItemType represents the type of context item
type ContextItemType string

const (
	ContextItemTypeMessage   ContextItemType = "message"
	ContextItemTypeMemory    ContextItemType = "memory"
	ContextItemTypeDocument  ContextItemType = "document"
	ContextItemTypeTool      ContextItemType = "tool"
	ContextItemTypeSystem    ContextItemType = "system"
)

// ContextItem represents an item in the context window
type ContextItem struct {
	ID         string                 `json:"id"`
	Type       ContextItemType        `json:"type"`
	Content    string                 `json:"content"`
	Tokens     int                    `json:"tokens"`
	Priority   float64                `json:"priority"` // Higher = more important to keep
	Embedding  []float32              `json:"embedding,omitempty"`
	Source     string                 `json:"source,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	AddedAt    time.Time              `json:"added_at"`
	Compressed bool                   `json:"compressed"` // Whether this item has been compressed
}

// ContextManager manages context windows and memory retrieval
type ContextManager struct {
	windows        map[string]*ContextWindow
	semanticMemory *SemanticMemory
	knowledgeGraph *KnowledgeGraph
	tokenEstimator TokenEstimator
	summarizer     Summarizer
	mu             sync.RWMutex
}

// TokenEstimator estimates token counts
type TokenEstimator interface {
	EstimateTokens(text string) int
}

// Summarizer summarizes text
type Summarizer interface {
	Summarize(ctx context.Context, text string, maxTokens int) (string, error)
}

// ContextManagerConfig configures the context manager
type ContextManagerConfig struct {
	SemanticMemory *SemanticMemory
	KnowledgeGraph *KnowledgeGraph
	TokenEstimator TokenEstimator
	Summarizer     Summarizer
}

// NewContextManager creates a new context manager
func NewContextManager(config ContextManagerConfig) *ContextManager {
	return &ContextManager{
		windows:        make(map[string]*ContextWindow),
		semanticMemory: config.SemanticMemory,
		knowledgeGraph: config.KnowledgeGraph,
		tokenEstimator: config.TokenEstimator,
		summarizer:     config.Summarizer,
	}
}

// CreateWindow creates a new context window
func (cm *ContextManager) CreateWindow(agentID string, maxTokens int) *ContextWindow {
	window := &ContextWindow{
		ID:        uuid.New().String(),
		AgentID:   agentID,
		MaxTokens: maxTokens,
		Items:     make([]*ContextItem, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	cm.mu.Lock()
	cm.windows[window.ID] = window
	cm.mu.Unlock()

	return window
}

// GetWindow retrieves a context window
func (cm *ContextManager) GetWindow(windowID string) (*ContextWindow, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	window, ok := cm.windows[windowID]
	return window, ok
}

// AddItem adds an item to the context window
func (cm *ContextManager) AddItem(ctx context.Context, windowID string, item *ContextItem) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	window, ok := cm.windows[windowID]
	if !ok {
		return nil // Window not found
	}

	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.AddedAt.IsZero() {
		item.AddedAt = time.Now()
	}

	// Estimate tokens if not provided
	if item.Tokens == 0 && cm.tokenEstimator != nil {
		item.Tokens = cm.tokenEstimator.EstimateTokens(item.Content)
	}

	// Check if we need to make room
	for window.CurrentTokens+item.Tokens > window.MaxTokens {
		if !cm.evictLowestPriority(ctx, window) {
			break
		}
	}

	window.Items = append(window.Items, item)
	window.CurrentTokens += item.Tokens
	window.UpdatedAt = time.Now()

	return nil
}

// RemoveItem removes an item from the context window
func (cm *ContextManager) RemoveItem(windowID, itemID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	window, ok := cm.windows[windowID]
	if !ok {
		return
	}

	for i, item := range window.Items {
		if item.ID == itemID {
			window.CurrentTokens -= item.Tokens
			window.Items = append(window.Items[:i], window.Items[i+1:]...)
			window.UpdatedAt = time.Now()
			return
		}
	}
}

// evictLowestPriority evicts the lowest priority item
func (cm *ContextManager) evictLowestPriority(ctx context.Context, window *ContextWindow) bool {
	if len(window.Items) == 0 {
		return false
	}

	// Find lowest priority non-system item
	minIdx := -1
	minPriority := float64(1000)
	for i, item := range window.Items {
		if item.Type == ContextItemTypeSystem {
			continue // Don't evict system items
		}
		if item.Priority < minPriority {
			minPriority = item.Priority
			minIdx = i
		}
	}

	if minIdx < 0 {
		return false
	}

	item := window.Items[minIdx]

	// Try to compress before evicting if not already compressed
	if !item.Compressed && cm.summarizer != nil {
		compressed, err := cm.summarizer.Summarize(ctx, item.Content, item.Tokens/2)
		if err == nil && len(compressed) < len(item.Content) {
			// Replace with compressed version
			oldTokens := item.Tokens
			item.Content = compressed
			if cm.tokenEstimator != nil {
				item.Tokens = cm.tokenEstimator.EstimateTokens(compressed)
			} else {
				item.Tokens = oldTokens / 2
			}
			item.Compressed = true
			window.CurrentTokens = window.CurrentTokens - oldTokens + item.Tokens
			return true
		}
	}

	// Evict the item
	window.CurrentTokens -= item.Tokens
	window.Items = append(window.Items[:minIdx], window.Items[minIdx+1:]...)
	return true
}

// EnrichContext enriches the context with relevant memories and knowledge
func (cm *ContextManager) EnrichContext(ctx context.Context, windowID string, query string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	window, ok := cm.windows[windowID]
	if !ok {
		return nil
	}

	// Retrieve relevant memories
	if cm.semanticMemory != nil {
		memories, err := cm.semanticMemory.Recall(ctx, window.AgentID, query, 5)
		if err == nil {
			for _, result := range memories {
				tokens := 0
				if cm.tokenEstimator != nil {
					tokens = cm.tokenEstimator.EstimateTokens(result.Memory.Content)
				}

				item := &ContextItem{
					ID:       uuid.New().String(),
					Type:     ContextItemTypeMemory,
					Content:  result.Memory.Content,
					Tokens:   tokens,
					Priority: result.Relevance,
					Source:   result.Memory.ID,
					AddedAt:  time.Now(),
				}

				if window.CurrentTokens+tokens <= window.MaxTokens {
					window.Items = append(window.Items, item)
					window.CurrentTokens += tokens
				}
			}
		}
	}

	// Retrieve relevant knowledge graph entities
	if cm.knowledgeGraph != nil {
		entities, err := cm.knowledgeGraph.FindSimilarEntities(ctx, query, 5, 0.7)
		if err == nil {
			for _, entity := range entities {
				content := entity.Name
				if entity.Description != "" {
					content += ": " + entity.Description
				}

				tokens := 0
				if cm.tokenEstimator != nil {
					tokens = cm.tokenEstimator.EstimateTokens(content)
				}

				item := &ContextItem{
					ID:       uuid.New().String(),
					Type:     ContextItemTypeDocument,
					Content:  content,
					Tokens:   tokens,
					Priority: 0.5,
					Source:   entity.ID,
					AddedAt:  time.Now(),
				}

				if window.CurrentTokens+tokens <= window.MaxTokens {
					window.Items = append(window.Items, item)
					window.CurrentTokens += tokens
				}
			}
		}
	}

	window.UpdatedAt = time.Now()
	return nil
}

// GetContextContent returns the formatted context content
func (cm *ContextManager) GetContextContent(windowID string) string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	window, ok := cm.windows[windowID]
	if !ok {
		return ""
	}

	// Sort by priority (highest first)
	items := make([]*ContextItem, len(window.Items))
	copy(items, window.Items)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Priority > items[j].Priority
	})

	var content string
	for _, item := range items {
		content += item.Content + "\n\n"
	}

	return content
}

// ClearWindow clears a context window
func (cm *ContextManager) ClearWindow(windowID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	window, ok := cm.windows[windowID]
	if !ok {
		return
	}

	window.Items = make([]*ContextItem, 0)
	window.CurrentTokens = 0
	window.UpdatedAt = time.Now()
}

// DeleteWindow deletes a context window
func (cm *ContextManager) DeleteWindow(windowID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.windows, windowID)
}

// SimpleTokenEstimator provides a simple token estimation
type SimpleTokenEstimator struct {
	// Average characters per token (roughly 4 for English)
	CharsPerToken float64
}

// NewSimpleTokenEstimator creates a new simple token estimator
func NewSimpleTokenEstimator() *SimpleTokenEstimator {
	return &SimpleTokenEstimator{
		CharsPerToken: 4.0,
	}
}

// EstimateTokens estimates the number of tokens in text
func (e *SimpleTokenEstimator) EstimateTokens(text string) int {
	return int(float64(len(text)) / e.CharsPerToken)
}

// MockSummarizer provides a mock summarizer for testing
type MockSummarizer struct{}

// NewMockSummarizer creates a new mock summarizer
func NewMockSummarizer() *MockSummarizer {
	return &MockSummarizer{}
}

// Summarize returns a truncated version of the text
func (s *MockSummarizer) Summarize(ctx context.Context, text string, maxTokens int) (string, error) {
	// Simple truncation for testing
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text, nil
	}
	return text[:maxChars] + "...", nil
}
