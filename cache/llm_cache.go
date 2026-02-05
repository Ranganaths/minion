// Package cache provides caching capabilities for LLM responses
// including exact match, semantic similarity, and distributed caching.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// LLMCacheEntry represents a cached LLM response
type LLMCacheEntry struct {
	ID           string                 `json:"id"`
	PromptHash   string                 `json:"prompt_hash"`
	Prompt       string                 `json:"prompt"`
	Response     string                 `json:"response"`
	Model        string                 `json:"model"`
	Embedding    []float32              `json:"embedding,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	TokensUsed   int                    `json:"tokens_used"`
	CreatedAt    time.Time              `json:"created_at"`
	ExpiresAt    time.Time              `json:"expires_at"`
	HitCount     int64                  `json:"hit_count"`
	LastAccessed time.Time              `json:"last_accessed"`
}

// CacheStats holds cache statistics
type CacheStats struct {
	Hits           int64   `json:"hits"`
	Misses         int64   `json:"misses"`
	SemanticHits   int64   `json:"semantic_hits"`
	Size           int64   `json:"size"`
	TotalTokens    int64   `json:"total_tokens_saved"`
	HitRate        float64 `json:"hit_rate"`
	AvgLatencySaved float64 `json:"avg_latency_saved_ms"`
}

// LLMCache defines the interface for LLM response caching
type LLMCache interface {
	// Get retrieves a cached response by exact match
	Get(ctx context.Context, prompt, model string) (*LLMCacheEntry, bool)

	// GetSemantic retrieves a cached response by semantic similarity
	GetSemantic(ctx context.Context, prompt, model string, embedding []float32, threshold float64) (*LLMCacheEntry, bool)

	// Set stores a response in the cache
	Set(ctx context.Context, entry *LLMCacheEntry) error

	// Delete removes a cached entry
	Delete(ctx context.Context, promptHash string) error

	// Clear removes all cached entries
	Clear(ctx context.Context) error

	// GetStats returns cache statistics
	GetStats() *CacheStats
}

// Embedder interface for generating embeddings
type Embedder interface {
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
}

// SemanticLLMCache provides semantic similarity caching for LLM responses
type SemanticLLMCache struct {
	exactCache    map[string]*LLMCacheEntry     // Hash -> Entry
	semanticIndex []*LLMCacheEntry               // All entries for semantic search
	embedder      Embedder
	ttl           time.Duration
	maxSize       int
	minSimilarity float64 // Minimum similarity threshold

	// Stats
	hits         atomic.Int64
	misses       atomic.Int64
	semanticHits atomic.Int64
	tokensSaved  atomic.Int64

	mu       sync.RWMutex
	stopCh   chan struct{}
	stopped  atomic.Bool
}

// SemanticLLMCacheConfig configures the semantic LLM cache
type SemanticLLMCacheConfig struct {
	Embedder      Embedder
	TTL           time.Duration
	MaxSize       int
	MinSimilarity float64 // Default: 0.95
	CleanupInterval time.Duration
}

// NewSemanticLLMCache creates a new semantic LLM cache
func NewSemanticLLMCache(config SemanticLLMCacheConfig) *SemanticLLMCache {
	if config.TTL == 0 {
		config.TTL = 24 * time.Hour
	}
	if config.MaxSize == 0 {
		config.MaxSize = 10000
	}
	if config.MinSimilarity == 0 {
		config.MinSimilarity = 0.95
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 5 * time.Minute
	}

	cache := &SemanticLLMCache{
		exactCache:    make(map[string]*LLMCacheEntry),
		semanticIndex: make([]*LLMCacheEntry, 0),
		embedder:      config.Embedder,
		ttl:           config.TTL,
		maxSize:       config.MaxSize,
		minSimilarity: config.MinSimilarity,
		stopCh:        make(chan struct{}),
	}

	// Start cleanup goroutine
	go cache.cleanupLoop(config.CleanupInterval)

	return cache
}

// Get retrieves a cached response by exact match
func (c *SemanticLLMCache) Get(ctx context.Context, prompt, model string) (*LLMCacheEntry, bool) {
	hash := c.hashPrompt(prompt, model)

	c.mu.RLock()
	entry, ok := c.exactCache[hash]
	c.mu.RUnlock()

	if !ok {
		c.misses.Add(1)
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		c.mu.Lock()
		delete(c.exactCache, hash)
		c.mu.Unlock()
		c.misses.Add(1)
		return nil, false
	}

	// Update stats
	c.mu.Lock()
	entry.HitCount++
	entry.LastAccessed = time.Now()
	c.mu.Unlock()

	c.hits.Add(1)
	c.tokensSaved.Add(int64(entry.TokensUsed))

	return entry, true
}

// GetSemantic retrieves a cached response by semantic similarity
func (c *SemanticLLMCache) GetSemantic(ctx context.Context, prompt, model string, embedding []float32, threshold float64) (*LLMCacheEntry, bool) {
	// First try exact match
	if entry, ok := c.Get(ctx, prompt, model); ok {
		return entry, true
	}

	if embedding == nil || len(embedding) == 0 {
		return nil, false
	}

	if threshold == 0 {
		threshold = c.minSimilarity
	}

	c.mu.RLock()
	entries := make([]*LLMCacheEntry, len(c.semanticIndex))
	copy(entries, c.semanticIndex)
	c.mu.RUnlock()

	var bestMatch *LLMCacheEntry
	var bestSimilarity float64

	now := time.Now()
	for _, entry := range entries {
		// Skip expired entries
		if now.After(entry.ExpiresAt) {
			continue
		}

		// Skip different models
		if entry.Model != model {
			continue
		}

		// Skip entries without embeddings
		if entry.Embedding == nil {
			continue
		}

		similarity := cosineSimilarity(embedding, entry.Embedding)
		if similarity >= threshold && similarity > bestSimilarity {
			bestSimilarity = similarity
			bestMatch = entry
		}
	}

	if bestMatch != nil {
		c.mu.Lock()
		bestMatch.HitCount++
		bestMatch.LastAccessed = time.Now()
		c.mu.Unlock()

		c.semanticHits.Add(1)
		c.tokensSaved.Add(int64(bestMatch.TokensUsed))
		return bestMatch, true
	}

	c.misses.Add(1)
	return nil, false
}

// Set stores a response in the cache
func (c *SemanticLLMCache) Set(ctx context.Context, entry *LLMCacheEntry) error {
	if c.stopped.Load() {
		return fmt.Errorf("cache is closed")
	}

	if entry.PromptHash == "" {
		entry.PromptHash = c.hashPrompt(entry.Prompt, entry.Model)
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if entry.ExpiresAt.IsZero() {
		entry.ExpiresAt = time.Now().Add(c.ttl)
	}
	if entry.ID == "" {
		entry.ID = entry.PromptHash
	}

	// Generate embedding if embedder is available and not provided
	if c.embedder != nil && entry.Embedding == nil {
		embedding, err := c.embedder.EmbedQuery(ctx, entry.Prompt)
		if err == nil {
			entry.Embedding = embedding
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at max size
	if len(c.exactCache) >= c.maxSize {
		c.evictLRU()
	}

	// Store in exact match cache
	c.exactCache[entry.PromptHash] = entry

	// Store in semantic index if has embedding
	if entry.Embedding != nil {
		c.semanticIndex = append(c.semanticIndex, entry)
	}

	return nil
}

// Delete removes a cached entry
func (c *SemanticLLMCache) Delete(ctx context.Context, promptHash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.exactCache, promptHash)

	// Remove from semantic index
	for i, entry := range c.semanticIndex {
		if entry.PromptHash == promptHash {
			c.semanticIndex = append(c.semanticIndex[:i], c.semanticIndex[i+1:]...)
			break
		}
	}

	return nil
}

// Clear removes all cached entries
func (c *SemanticLLMCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.exactCache = make(map[string]*LLMCacheEntry)
	c.semanticIndex = make([]*LLMCacheEntry, 0)

	return nil
}

// GetStats returns cache statistics
func (c *SemanticLLMCache) GetStats() *CacheStats {
	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	c.mu.RLock()
	size := int64(len(c.exactCache))
	c.mu.RUnlock()

	return &CacheStats{
		Hits:           hits,
		Misses:         misses,
		SemanticHits:   c.semanticHits.Load(),
		Size:           size,
		TotalTokens:    c.tokensSaved.Load(),
		HitRate:        hitRate,
	}
}

// Close stops the cache
func (c *SemanticLLMCache) Close() error {
	if c.stopped.CompareAndSwap(false, true) {
		close(c.stopCh)
	}
	return nil
}

// hashPrompt generates a hash for a prompt and model
func (c *SemanticLLMCache) hashPrompt(prompt, model string) string {
	data := fmt.Sprintf("%s|%s", model, prompt)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// evictLRU removes the least recently used entry
func (c *SemanticLLMCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.exactCache {
		if oldestKey == "" || entry.LastAccessed.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastAccessed
		}
	}

	if oldestKey != "" {
		delete(c.exactCache, oldestKey)
		// Also remove from semantic index
		for i, entry := range c.semanticIndex {
			if entry.PromptHash == oldestKey {
				c.semanticIndex = append(c.semanticIndex[:i], c.semanticIndex[i+1:]...)
				break
			}
		}
	}
}

// cleanupLoop periodically removes expired entries
func (c *SemanticLLMCache) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

// cleanup removes expired entries
func (c *SemanticLLMCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// Clean exact cache
	for key, entry := range c.exactCache {
		if now.After(entry.ExpiresAt) {
			delete(c.exactCache, key)
		}
	}

	// Clean semantic index
	validEntries := make([]*LLMCacheEntry, 0, len(c.semanticIndex))
	for _, entry := range c.semanticIndex {
		if !now.After(entry.ExpiresAt) {
			validEntries = append(validEntries, entry)
		}
	}
	c.semanticIndex = validEntries
}

// cosineSimilarity calculates cosine similarity between two vectors
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

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Ensure SemanticLLMCache implements LLMCache
var _ LLMCache = (*SemanticLLMCache)(nil)

// CacheKey generates a cache key from prompt and optional parameters
func CacheKey(prompt string, model string, params map[string]interface{}) string {
	data := map[string]interface{}{
		"prompt": prompt,
		"model":  model,
	}
	if params != nil {
		data["params"] = params
	}
	jsonData, _ := json.Marshal(data)
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:])
}
