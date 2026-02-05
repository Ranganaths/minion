package memory

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryType represents the type of memory
type MemoryType string

const (
	MemoryTypeEpisodic   MemoryType = "episodic"   // Specific events/experiences
	MemoryTypeSemantic   MemoryType = "semantic"   // General knowledge/facts
	MemoryTypeProcedural MemoryType = "procedural" // How to do things
	MemoryTypeWorking    MemoryType = "working"    // Short-term active memory
)

// MemoryEntry represents a memory entry
type MemoryEntry struct {
	ID          string                 `json:"id"`
	AgentID     string                 `json:"agent_id"`
	Type        MemoryType             `json:"type"`
	Content     string                 `json:"content"`
	Embedding   []float32              `json:"embedding,omitempty"`
	Importance  float64                `json:"importance"` // 0-1 importance score
	AccessCount int                    `json:"access_count"`
	LastAccess  time.Time              `json:"last_access"`
	CreatedAt   time.Time              `json:"created_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Associations []string              `json:"associations,omitempty"` // Related memory IDs
	Tags        []string               `json:"tags,omitempty"`
}

// MemoryQuery represents a query for memories
type MemoryQuery struct {
	AgentID      string
	Query        string
	Embedding    []float32
	Types        []MemoryType
	Tags         []string
	MinImportance float64
	Limit        int
	IncludeExpired bool
}

// MemorySearchResult represents a search result
type MemorySearchResult struct {
	Memory     *MemoryEntry `json:"memory"`
	Similarity float64      `json:"similarity"`
	Relevance  float64      `json:"relevance"` // Combined score
}

// Embedder generates embeddings for text
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// MemoryStore stores memories
type MemoryStore interface {
	Store(ctx context.Context, memory *MemoryEntry) error
	Get(ctx context.Context, id string) (*MemoryEntry, error)
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, query *MemoryQuery) ([]*MemorySearchResult, error)
	Update(ctx context.Context, memory *MemoryEntry) error
	GetByAgent(ctx context.Context, agentID string, limit int) ([]*MemoryEntry, error)
	DeleteExpired(ctx context.Context) (int, error)
}

// SemanticMemory provides semantic memory capabilities
type SemanticMemory struct {
	store          MemoryStore
	embedder       Embedder
	decayRate      float64 // Memory importance decay rate
	minImportance  float64 // Minimum importance before forgetting
	consolidateAge time.Duration
	mu             sync.RWMutex
}

// SemanticMemoryConfig configures semantic memory
type SemanticMemoryConfig struct {
	Store          MemoryStore
	Embedder       Embedder
	DecayRate      float64       // How fast memories decay (0-1)
	MinImportance  float64       // Minimum importance threshold
	ConsolidateAge time.Duration // Age before consolidation
}

// NewSemanticMemory creates a new semantic memory
func NewSemanticMemory(config SemanticMemoryConfig) *SemanticMemory {
	if config.DecayRate == 0 {
		config.DecayRate = 0.01
	}
	if config.MinImportance == 0 {
		config.MinImportance = 0.1
	}
	if config.ConsolidateAge == 0 {
		config.ConsolidateAge = 24 * time.Hour
	}

	return &SemanticMemory{
		store:          config.Store,
		embedder:       config.Embedder,
		decayRate:      config.DecayRate,
		minImportance:  config.MinImportance,
		consolidateAge: config.ConsolidateAge,
	}
}

// Remember stores a new memory
func (m *SemanticMemory) Remember(ctx context.Context, agentID string, content string, memType MemoryType, importance float64, metadata map[string]interface{}) (*MemoryEntry, error) {
	// Generate embedding
	embedding, err := m.embedder.Embed(ctx, content)
	if err != nil {
		return nil, err
	}

	memory := &MemoryEntry{
		ID:          uuid.New().String(),
		AgentID:     agentID,
		Type:        memType,
		Content:     content,
		Embedding:   embedding,
		Importance:  importance,
		AccessCount: 0,
		LastAccess:  time.Now(),
		CreatedAt:   time.Now(),
		Metadata:    metadata,
	}

	// Set expiry for working memory
	if memType == MemoryTypeWorking {
		expiry := time.Now().Add(time.Hour)
		memory.ExpiresAt = &expiry
	}

	if err := m.store.Store(ctx, memory); err != nil {
		return nil, err
	}

	return memory, nil
}

// Recall retrieves memories similar to a query
func (m *SemanticMemory) Recall(ctx context.Context, agentID string, query string, limit int) ([]*MemorySearchResult, error) {
	// Generate embedding for query
	embedding, err := m.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	memQuery := &MemoryQuery{
		AgentID:   agentID,
		Query:     query,
		Embedding: embedding,
		Limit:     limit,
	}

	results, err := m.store.Search(ctx, memQuery)
	if err != nil {
		return nil, err
	}

	// Update access patterns
	for _, result := range results {
		result.Memory.AccessCount++
		result.Memory.LastAccess = time.Now()
		// Boost importance on recall (memory strengthening)
		result.Memory.Importance = math.Min(1.0, result.Memory.Importance*1.1)
		m.store.Update(ctx, result.Memory)
	}

	return results, nil
}

// Forget removes a memory
func (m *SemanticMemory) Forget(ctx context.Context, memoryID string) error {
	return m.store.Delete(ctx, memoryID)
}

// Associate creates an association between memories
func (m *SemanticMemory) Associate(ctx context.Context, memoryID1, memoryID2 string) error {
	mem1, err := m.store.Get(ctx, memoryID1)
	if err != nil {
		return err
	}

	mem2, err := m.store.Get(ctx, memoryID2)
	if err != nil {
		return err
	}

	// Add bidirectional associations
	mem1.Associations = appendUnique(mem1.Associations, memoryID2)
	mem2.Associations = appendUnique(mem2.Associations, memoryID1)

	if err := m.store.Update(ctx, mem1); err != nil {
		return err
	}
	return m.store.Update(ctx, mem2)
}

// Decay applies time-based decay to memories
func (m *SemanticMemory) Decay(ctx context.Context, agentID string) (int, error) {
	memories, err := m.store.GetByAgent(ctx, agentID, 1000)
	if err != nil {
		return 0, err
	}

	forgotten := 0
	for _, mem := range memories {
		// Calculate decay based on time since last access
		timeSinceAccess := time.Since(mem.LastAccess)
		decayFactor := math.Exp(-m.decayRate * timeSinceAccess.Hours())
		mem.Importance *= decayFactor

		if mem.Importance < m.minImportance {
			// Forget unimportant memories
			if err := m.store.Delete(ctx, mem.ID); err == nil {
				forgotten++
			}
		} else {
			m.store.Update(ctx, mem)
		}
	}

	return forgotten, nil
}

// Consolidate consolidates old episodic memories into semantic memories
func (m *SemanticMemory) Consolidate(ctx context.Context, agentID string) ([]*MemoryEntry, error) {
	memories, err := m.store.GetByAgent(ctx, agentID, 1000)
	if err != nil {
		return nil, err
	}

	// Find old episodic memories
	var oldEpisodic []*MemoryEntry
	for _, mem := range memories {
		if mem.Type == MemoryTypeEpisodic && time.Since(mem.CreatedAt) > m.consolidateAge {
			oldEpisodic = append(oldEpisodic, mem)
		}
	}

	// Group similar memories
	clusters := m.clusterMemories(oldEpisodic)
	var consolidated []*MemoryEntry

	for _, cluster := range clusters {
		if len(cluster) < 2 {
			continue
		}

		// Create consolidated semantic memory
		content := m.summarizeCluster(cluster)
		importance := 0.0
		for _, mem := range cluster {
			importance += mem.Importance
		}
		importance /= float64(len(cluster))

		embedding, err := m.embedder.Embed(ctx, content)
		if err != nil {
			continue
		}

		newMem := &MemoryEntry{
			ID:          uuid.New().String(),
			AgentID:     agentID,
			Type:        MemoryTypeSemantic,
			Content:     content,
			Embedding:   embedding,
			Importance:  importance,
			AccessCount: 0,
			LastAccess:  time.Now(),
			CreatedAt:   time.Now(),
			Metadata: map[string]interface{}{
				"consolidated_from": len(cluster),
			},
		}

		if err := m.store.Store(ctx, newMem); err == nil {
			consolidated = append(consolidated, newMem)
			// Remove old episodic memories
			for _, mem := range cluster {
				m.store.Delete(ctx, mem.ID)
			}
		}
	}

	return consolidated, nil
}

// clusterMemories groups similar memories
func (m *SemanticMemory) clusterMemories(memories []*MemoryEntry) [][]*MemoryEntry {
	if len(memories) == 0 {
		return nil
	}

	const similarityThreshold = 0.8
	used := make(map[string]bool)
	var clusters [][]*MemoryEntry

	for _, mem := range memories {
		if used[mem.ID] {
			continue
		}

		cluster := []*MemoryEntry{mem}
		used[mem.ID] = true

		for _, other := range memories {
			if used[other.ID] {
				continue
			}
			sim := cosineSimilarity(mem.Embedding, other.Embedding)
			if sim >= similarityThreshold {
				cluster = append(cluster, other)
				used[other.ID] = true
			}
		}

		clusters = append(clusters, cluster)
	}

	return clusters
}

// summarizeCluster creates a summary of clustered memories
func (m *SemanticMemory) summarizeCluster(cluster []*MemoryEntry) string {
	if len(cluster) == 0 {
		return ""
	}
	// Simple summary: use the most important memory's content
	sort.Slice(cluster, func(i, j int) bool {
		return cluster[i].Importance > cluster[j].Importance
	})
	return cluster[0].Content
}

// GetRecentMemories gets recent memories for an agent
func (m *SemanticMemory) GetRecentMemories(ctx context.Context, agentID string, limit int) ([]*MemoryEntry, error) {
	return m.store.GetByAgent(ctx, agentID, limit)
}

// InMemoryMemoryStore is an in-memory implementation of MemoryStore
type InMemoryMemoryStore struct {
	memories map[string]*MemoryEntry
	mu       sync.RWMutex
}

// NewInMemoryMemoryStore creates a new in-memory memory store
func NewInMemoryMemoryStore() *InMemoryMemoryStore {
	return &InMemoryMemoryStore{
		memories: make(map[string]*MemoryEntry),
	}
}

// Store stores a memory
func (s *InMemoryMemoryStore) Store(ctx context.Context, memory *MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memories[memory.ID] = memory
	return nil
}

// Get retrieves a memory by ID
func (s *InMemoryMemoryStore) Get(ctx context.Context, id string) (*MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mem, ok := s.memories[id]
	if !ok {
		return nil, errors.New("memory not found")
	}
	return mem, nil
}

// Delete deletes a memory
func (s *InMemoryMemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.memories, id)
	return nil
}

// Update updates a memory
func (s *InMemoryMemoryStore) Update(ctx context.Context, memory *MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memories[memory.ID] = memory
	return nil
}

// GetByAgent gets memories for an agent
func (s *InMemoryMemoryStore) GetByAgent(ctx context.Context, agentID string, limit int) ([]*MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*MemoryEntry
	for _, mem := range s.memories {
		if mem.AgentID == agentID {
			results = append(results, mem)
		}
	}

	// Sort by last access (most recent first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].LastAccess.After(results[j].LastAccess)
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// DeleteExpired deletes expired memories
func (s *InMemoryMemoryStore) DeleteExpired(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	deleted := 0
	now := time.Now()
	for id, mem := range s.memories {
		if mem.ExpiresAt != nil && mem.ExpiresAt.Before(now) {
			delete(s.memories, id)
			deleted++
		}
	}

	return deleted, nil
}

// Search searches memories
func (s *InMemoryMemoryStore) Search(ctx context.Context, query *MemoryQuery) ([]*MemorySearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*MemorySearchResult
	now := time.Now()

	for _, mem := range s.memories {
		// Filter by agent
		if query.AgentID != "" && mem.AgentID != query.AgentID {
			continue
		}

		// Filter by type
		if len(query.Types) > 0 {
			found := false
			for _, t := range query.Types {
				if mem.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by importance
		if mem.Importance < query.MinImportance {
			continue
		}

		// Filter expired
		if !query.IncludeExpired && mem.ExpiresAt != nil && mem.ExpiresAt.Before(now) {
			continue
		}

		// Calculate similarity if embedding provided
		similarity := 0.0
		if len(query.Embedding) > 0 && len(mem.Embedding) > 0 {
			similarity = cosineSimilarity(query.Embedding, mem.Embedding)
		}

		// Calculate relevance (combine similarity and recency)
		recencyScore := 1.0 / (1.0 + time.Since(mem.LastAccess).Hours()/24.0)
		relevance := similarity*0.7 + recencyScore*0.2 + mem.Importance*0.1

		results = append(results, &MemorySearchResult{
			Memory:     mem,
			Similarity: similarity,
			Relevance:  relevance,
		})
	}

	// Sort by relevance
	sort.Slice(results, func(i, j int) bool {
		return results[i].Relevance > results[j].Relevance
	})

	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	return results, nil
}

// Helper functions

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
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

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
