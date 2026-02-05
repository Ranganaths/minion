package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

// RedisClient interface for Redis operations
type RedisClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Keys(ctx context.Context, pattern string) ([]string, error)
	Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error)
	Exists(ctx context.Context, keys ...string) (int64, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
	Close() error
}

// RedisLLMCache implements LLMCache using Redis for distributed caching
type RedisLLMCache struct {
	client        RedisClient
	prefix        string
	ttl           time.Duration
	embedder      Embedder
	minSimilarity float64

	// Local embedding index for semantic search
	localIndex    *SemanticLLMCache

	// Stats
	hits         atomic.Int64
	misses       atomic.Int64
	semanticHits atomic.Int64
	tokensSaved  atomic.Int64
}

// RedisLLMCacheConfig configures the Redis LLM cache
type RedisLLMCacheConfig struct {
	Client        RedisClient
	Prefix        string        // Key prefix (default: "llm_cache:")
	TTL           time.Duration // Cache TTL (default: 24h)
	Embedder      Embedder      // For semantic caching
	MinSimilarity float64       // Minimum similarity for semantic matches (default: 0.95)
	LocalCacheSize int          // Size of local semantic index cache (default: 1000)
}

// NewRedisLLMCache creates a new Redis-based LLM cache
func NewRedisLLMCache(config RedisLLMCacheConfig) (*RedisLLMCache, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("redis client is required")
	}

	if config.Prefix == "" {
		config.Prefix = "llm_cache:"
	}
	if config.TTL == 0 {
		config.TTL = 24 * time.Hour
	}
	if config.MinSimilarity == 0 {
		config.MinSimilarity = 0.95
	}
	if config.LocalCacheSize == 0 {
		config.LocalCacheSize = 1000
	}

	cache := &RedisLLMCache{
		client:        config.Client,
		prefix:        config.Prefix,
		ttl:           config.TTL,
		embedder:      config.Embedder,
		minSimilarity: config.MinSimilarity,
	}

	// Create local index for semantic search
	if config.Embedder != nil {
		cache.localIndex = NewSemanticLLMCache(SemanticLLMCacheConfig{
			Embedder:      config.Embedder,
			TTL:           config.TTL,
			MaxSize:       config.LocalCacheSize,
			MinSimilarity: config.MinSimilarity,
		})
	}

	return cache, nil
}

// Get retrieves a cached response by exact match
func (c *RedisLLMCache) Get(ctx context.Context, prompt, model string) (*LLMCacheEntry, bool) {
	key := c.cacheKey(prompt, model)

	data, err := c.client.Get(ctx, key)
	if err != nil {
		c.misses.Add(1)
		return nil, false
	}

	var entry LLMCacheEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		c.misses.Add(1)
		return nil, false
	}

	// Check expiration (Redis handles this, but double-check)
	if time.Now().After(entry.ExpiresAt) {
		c.client.Del(ctx, key)
		c.misses.Add(1)
		return nil, false
	}

	// Update hit count (fire and forget)
	entry.HitCount++
	entry.LastAccessed = time.Now()
	go c.updateEntry(ctx, &entry)

	c.hits.Add(1)
	c.tokensSaved.Add(int64(entry.TokensUsed))

	return &entry, true
}

// GetSemantic retrieves a cached response by semantic similarity
func (c *RedisLLMCache) GetSemantic(ctx context.Context, prompt, model string, embedding []float32, threshold float64) (*LLMCacheEntry, bool) {
	// First try exact match
	if entry, ok := c.Get(ctx, prompt, model); ok {
		return entry, true
	}

	// Try local semantic index
	if c.localIndex != nil {
		if entry, ok := c.localIndex.GetSemantic(ctx, prompt, model, embedding, threshold); ok {
			c.semanticHits.Add(1)
			c.tokensSaved.Add(int64(entry.TokensUsed))
			return entry, true
		}
	}

	c.misses.Add(1)
	return nil, false
}

// Set stores a response in the cache
func (c *RedisLLMCache) Set(ctx context.Context, entry *LLMCacheEntry) error {
	if entry.PromptHash == "" {
		hash := CacheKey(entry.Prompt, entry.Model, nil)
		entry.PromptHash = hash
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
	entry.LastAccessed = time.Now()

	// Generate embedding if not provided
	if c.embedder != nil && entry.Embedding == nil {
		embedding, err := c.embedder.EmbedQuery(ctx, entry.Prompt)
		if err == nil {
			entry.Embedding = embedding
		}
	}

	// Serialize entry
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	// Store in Redis
	key := c.prefix + entry.PromptHash
	ttl := time.Until(entry.ExpiresAt)
	if ttl <= 0 {
		ttl = c.ttl
	}

	if err := c.client.Set(ctx, key, string(data), ttl); err != nil {
		return fmt.Errorf("failed to store in Redis: %w", err)
	}

	// Store in local index for semantic search
	if c.localIndex != nil && entry.Embedding != nil {
		c.localIndex.Set(ctx, entry)
	}

	return nil
}

// Delete removes a cached entry
func (c *RedisLLMCache) Delete(ctx context.Context, promptHash string) error {
	key := c.prefix + promptHash
	if err := c.client.Del(ctx, key); err != nil {
		return fmt.Errorf("failed to delete from Redis: %w", err)
	}

	// Also delete from local index
	if c.localIndex != nil {
		c.localIndex.Delete(ctx, promptHash)
	}

	return nil
}

// Clear removes all cached entries
func (c *RedisLLMCache) Clear(ctx context.Context) error {
	// Scan and delete all keys with prefix
	var cursor uint64
	for {
		keys, newCursor, err := c.client.Scan(ctx, cursor, c.prefix+"*", 100)
		if err != nil {
			return fmt.Errorf("failed to scan Redis: %w", err)
		}

		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...); err != nil {
				return fmt.Errorf("failed to delete keys: %w", err)
			}
		}

		cursor = newCursor
		if cursor == 0 {
			break
		}
	}

	// Clear local index
	if c.localIndex != nil {
		c.localIndex.Clear(ctx)
	}

	return nil
}

// GetStats returns cache statistics
func (c *RedisLLMCache) GetStats() *CacheStats {
	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return &CacheStats{
		Hits:         hits,
		Misses:       misses,
		SemanticHits: c.semanticHits.Load(),
		TotalTokens:  c.tokensSaved.Load(),
		HitRate:      hitRate,
	}
}

// Close closes the cache
func (c *RedisLLMCache) Close() error {
	if c.localIndex != nil {
		c.localIndex.Close()
	}
	return c.client.Close()
}

// cacheKey generates a cache key
func (c *RedisLLMCache) cacheKey(prompt, model string) string {
	hash := CacheKey(prompt, model, nil)
	return c.prefix + hash
}

// updateEntry updates an entry in Redis (for hit count tracking)
func (c *RedisLLMCache) updateEntry(ctx context.Context, entry *LLMCacheEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	key := c.prefix + entry.PromptHash
	ttl, err := c.client.TTL(ctx, key)
	if err != nil || ttl <= 0 {
		ttl = c.ttl
	}

	c.client.Set(ctx, key, string(data), ttl)
}

// RefreshLocalIndex rebuilds the local semantic index from Redis
func (c *RedisLLMCache) RefreshLocalIndex(ctx context.Context) error {
	if c.localIndex == nil {
		return nil
	}

	// Clear local index
	c.localIndex.Clear(ctx)

	// Scan Redis and populate local index
	var cursor uint64
	for {
		keys, newCursor, err := c.client.Scan(ctx, cursor, c.prefix+"*", 100)
		if err != nil {
			return fmt.Errorf("failed to scan Redis: %w", err)
		}

		for _, key := range keys {
			data, err := c.client.Get(ctx, key)
			if err != nil {
				continue
			}

			var entry LLMCacheEntry
			if err := json.Unmarshal([]byte(data), &entry); err != nil {
				continue
			}

			if entry.Embedding != nil {
				c.localIndex.Set(ctx, &entry)
			}
		}

		cursor = newCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

// Ensure RedisLLMCache implements LLMCache
var _ LLMCache = (*RedisLLMCache)(nil)

// MockRedisClient is a simple in-memory mock for testing
type MockRedisClient struct {
	data map[string]mockEntry
}

type mockEntry struct {
	value     string
	expiresAt time.Time
}

// NewMockRedisClient creates a mock Redis client
func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]mockEntry),
	}
}

func (m *MockRedisClient) Get(ctx context.Context, key string) (string, error) {
	entry, ok := m.data[key]
	if !ok {
		return "", fmt.Errorf("key not found")
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		delete(m.data, key)
		return "", fmt.Errorf("key expired")
	}
	return entry.value, nil
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	var expiresAt time.Time
	if expiration > 0 {
		expiresAt = time.Now().Add(expiration)
	}
	m.data[key] = mockEntry{value: value, expiresAt: expiresAt}
	return nil
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func (m *MockRedisClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	keys := make([]string, 0)
	for key := range m.data {
		keys = append(keys, key)
	}
	return keys, nil
}

func (m *MockRedisClient) Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	keys := make([]string, 0)
	for key := range m.data {
		keys = append(keys, key)
	}
	return keys, 0, nil
}

func (m *MockRedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	var count int64
	for _, key := range keys {
		if _, ok := m.data[key]; ok {
			count++
		}
	}
	return count, nil
}

func (m *MockRedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	entry, ok := m.data[key]
	if !ok {
		return -2, nil
	}
	if entry.expiresAt.IsZero() {
		return -1, nil
	}
	ttl := time.Until(entry.expiresAt)
	if ttl < 0 {
		return -2, nil
	}
	return ttl, nil
}

func (m *MockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	entry, ok := m.data[key]
	if !ok {
		return fmt.Errorf("key not found")
	}
	entry.expiresAt = time.Now().Add(expiration)
	m.data[key] = entry
	return nil
}

func (m *MockRedisClient) Close() error {
	return nil
}

// Ensure MockRedisClient implements RedisClient
var _ RedisClient = (*MockRedisClient)(nil)
