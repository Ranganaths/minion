package embeddings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Ranganaths/minion/metrics"
)

// CachedEmbedder wraps an Embedder with caching capabilities.
// CachedEmbedder is safe for concurrent use by multiple goroutines.
type CachedEmbedder struct {
	embedder  Embedder
	cache     Cache
	keyPrefix string
	ttl       time.Duration
	hitCount  atomic.Int64
	missCount atomic.Int64
	// Metrics
	cacheHits   metrics.Counter
	cacheMisses metrics.Counter
	duration    metrics.Histogram
}

// Cache is the interface for embedding cache storage
type Cache interface {
	// Get retrieves an embedding from the cache
	Get(ctx context.Context, key string) ([]float32, bool)

	// Set stores an embedding in the cache
	Set(ctx context.Context, key string, embedding []float32, ttl time.Duration) error

	// Delete removes an embedding from the cache
	Delete(ctx context.Context, key string) error

	// Clear removes all embeddings from the cache
	Clear(ctx context.Context) error
}

// CachedEmbedderConfig configures the cached embedder
type CachedEmbedderConfig struct {
	// Embedder is the underlying embedder
	Embedder Embedder

	// Cache is the cache implementation
	Cache Cache

	// KeyPrefix is a prefix for cache keys
	KeyPrefix string

	// TTL is the cache entry time-to-live
	TTL time.Duration
}

// NewCachedEmbedder creates a new cached embedder
func NewCachedEmbedder(cfg CachedEmbedderConfig) *CachedEmbedder {
	ttl := cfg.TTL
	if ttl == 0 {
		ttl = 24 * time.Hour // Default to 24 hours
	}

	m := metrics.GetMetrics()
	return &CachedEmbedder{
		embedder:    cfg.Embedder,
		cache:       cfg.Cache,
		keyPrefix:   cfg.KeyPrefix,
		ttl:         ttl,
		cacheHits:   m.Counter(metrics.MetricEmbeddingCacheHits, nil),
		cacheMisses: m.Counter(metrics.MetricEmbeddingCacheMisses, nil),
		duration:    m.Histogram(metrics.MetricEmbeddingDuration, nil),
	}
}

// EmbedQuery embeds a single query text with caching
func (e *CachedEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	start := time.Now()
	defer func() {
		e.duration.Observe(time.Since(start).Seconds())
	}()

	key := e.cacheKey(text)

	// Try cache first
	if embedding, ok := e.cache.Get(ctx, key); ok {
		e.hitCount.Add(1)
		e.cacheHits.Inc()
		return embedding, nil
	}

	e.missCount.Add(1)
	e.cacheMisses.Inc()

	// Get from embedder
	embedding, err := e.embedder.EmbedQuery(ctx, text)
	if err != nil {
		return nil, err
	}

	// Store in cache (log errors but don't fail the operation)
	if err := e.cache.Set(ctx, key, embedding, e.ttl); err != nil {
		log.Printf("embeddings: cache set failed for key %s: %v", key, err)
	}

	return embedding, nil
}

// EmbedDocuments embeds multiple document texts with caching
func (e *CachedEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	start := time.Now()
	defer func() {
		e.duration.Observe(time.Since(start).Seconds())
	}()

	embeddings := make([][]float32, len(texts))
	var uncachedTexts []string
	var uncachedIndices []int

	// Check cache for each text
	for i, text := range texts {
		key := e.cacheKey(text)
		if embedding, ok := e.cache.Get(ctx, key); ok {
			embeddings[i] = embedding
			e.hitCount.Add(1)
			e.cacheHits.Inc()
		} else {
			uncachedTexts = append(uncachedTexts, text)
			uncachedIndices = append(uncachedIndices, i)
			e.missCount.Add(1)
			e.cacheMisses.Inc()
		}
	}

	// Embed uncached texts
	if len(uncachedTexts) > 0 {
		newEmbeddings, err := e.embedder.EmbedDocuments(ctx, uncachedTexts)
		if err != nil {
			return nil, err
		}

		// Store results and update cache
		for i, embedding := range newEmbeddings {
			idx := uncachedIndices[i]
			embeddings[idx] = embedding

			key := e.cacheKey(texts[idx])
			if err := e.cache.Set(ctx, key, embedding, e.ttl); err != nil {
				log.Printf("embeddings: cache set failed for key %s: %v", key, err)
			}
		}
	}

	return embeddings, nil
}

// Dimension returns the embedding dimension size
func (e *CachedEmbedder) Dimension() int {
	return e.embedder.Dimension()
}

// cacheKey generates a cache key for the given text
func (e *CachedEmbedder) cacheKey(text string) string {
	hash := sha256.Sum256([]byte(text))
	return e.keyPrefix + hex.EncodeToString(hash[:])
}

// Stats returns cache hit/miss statistics
func (e *CachedEmbedder) Stats() (hits, misses int64) {
	return e.hitCount.Load(), e.missCount.Load()
}

// ResetStats resets the cache hit/miss statistics
func (e *CachedEmbedder) ResetStats() {
	e.hitCount.Store(0)
	e.missCount.Store(0)
}

// ClearCache clears the embedding cache
func (e *CachedEmbedder) ClearCache(ctx context.Context) error {
	return e.cache.Clear(ctx)
}

// MemoryCache is an in-memory cache implementation.
// MemoryCache is safe for concurrent use by multiple goroutines.
// Call Close() to stop the background cleanup goroutine when done.
type MemoryCache struct {
	entries         map[string]*cacheEntry
	maxSize         int
	cleanupInterval time.Duration
	mu              sync.RWMutex
	stopCh          chan struct{}
	stopped         atomic.Bool
}

type cacheEntry struct {
	embedding []float32
	expiresAt time.Time
}

// MemoryCacheConfig configures the memory cache
type MemoryCacheConfig struct {
	// MaxSize is the maximum number of entries (default: 10000)
	MaxSize int

	// CleanupInterval is how often to run cleanup (default: 5 minutes)
	CleanupInterval time.Duration
}

// NewMemoryCache creates a new in-memory cache.
// Call Close() when done to release resources.
func NewMemoryCache(cfg MemoryCacheConfig) *MemoryCache {
	maxSize := cfg.MaxSize
	if maxSize <= 0 {
		maxSize = 10000
	}

	cleanupInterval := cfg.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}

	cache := &MemoryCache{
		entries:         make(map[string]*cacheEntry),
		maxSize:         maxSize,
		cleanupInterval: cleanupInterval,
		stopCh:          make(chan struct{}),
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// Close stops the background cleanup goroutine and releases resources.
// It is safe to call Close multiple times.
func (c *MemoryCache) Close() error {
	if c.stopped.CompareAndSwap(false, true) {
		close(c.stopCh)
	}
	return nil
}

// Get retrieves an embedding from the cache
func (c *MemoryCache) Get(ctx context.Context, key string) ([]float32, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.embedding, true
}

// Set stores an embedding in the cache.
// Returns an error if the cache has been closed.
func (c *MemoryCache) Set(ctx context.Context, key string, embedding []float32, ttl time.Duration) error {
	if c.stopped.Load() {
		return fmt.Errorf("cache is closed")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at max size
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[key] = &cacheEntry{
		embedding: embedding,
		expiresAt: time.Now().Add(ttl),
	}

	return nil
}

// Delete removes an embedding from the cache
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
	return nil
}

// Clear removes all embeddings from the cache
func (c *MemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
	return nil
}

// evictOldest removes the oldest entry (simple LRU-like eviction)
func (c *MemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.expiresAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// cleanupLoop periodically removes expired entries
func (c *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(c.cleanupInterval)
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
func (c *MemoryCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}

// Size returns the current cache size
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
