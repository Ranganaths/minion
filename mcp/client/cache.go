package client

import (
	"sync"
	"time"
)

// CachePolicy defines cache eviction policies
type CachePolicy string

const (
	CachePolicyLRU  CachePolicy = "lru"  // Least Recently Used
	CachePolicyLFU  CachePolicy = "lfu"  // Least Frequently Used
	CachePolicyFIFO CachePolicy = "fifo" // First In First Out
	CachePolicyTTL  CachePolicy = "ttl"  // Time To Live only
)

// ToolCache caches discovered tools with TTL and eviction
type ToolCache struct {
	entries map[string]*cacheEntry
	mu      sync.RWMutex
	config  *CacheConfig
	metrics *cacheMetrics
}

// CacheConfig configures the cache behavior
type CacheConfig struct {
	Enabled       bool
	TTL           time.Duration
	MaxSize       int
	EvictionPolicy CachePolicy
	CleanupPeriod time.Duration
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:        true,
		TTL:            5 * time.Minute,
		MaxSize:        100,
		EvictionPolicy: CachePolicyLRU,
		CleanupPeriod:  1 * time.Minute,
	}
}

// cacheEntry represents a cached item
type cacheEntry struct {
	tools       []MCPTool
	cachedAt    time.Time
	lastAccess  time.Time
	accessCount int64
	serverName  string
}

// cacheMetrics tracks cache performance
type cacheMetrics struct {
	hits          int64
	misses        int64
	evictions     int64
	totalInserts  int64
	totalSize     int
	mu            sync.RWMutex
}

// NewToolCache creates a new tool cache
func NewToolCache(config *CacheConfig) *ToolCache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	cache := &ToolCache{
		entries: make(map[string]*cacheEntry),
		config:  config,
		metrics: &cacheMetrics{},
	}

	if config.Enabled {
		go cache.cleanupLoop()
	}

	return cache
}

// Get retrieves tools from cache
func (c *ToolCache) Get(serverName string) ([]MCPTool, bool) {
	if !c.config.Enabled {
		return nil, false
	}

	c.mu.RLock()
	entry, exists := c.entries[serverName]
	c.mu.RUnlock()

	if !exists {
		c.recordMiss()
		return nil, false
	}

	// Check if expired
	if c.isExpired(entry) {
		c.mu.Lock()
		delete(c.entries, serverName)
		c.mu.Unlock()
		c.recordMiss()
		return nil, false
	}

	// Update access metadata
	c.mu.Lock()
	entry.lastAccess = time.Now()
	entry.accessCount++
	c.mu.Unlock()

	c.recordHit()

	// Return copy of tools
	tools := make([]MCPTool, len(entry.tools))
	copy(tools, entry.tools)

	return tools, true
}

// Set stores tools in cache
func (c *ToolCache) Set(serverName string, tools []MCPTool) {
	if !c.config.Enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict
	if c.config.MaxSize > 0 && len(c.entries) >= c.config.MaxSize {
		c.evict()
	}

	// Store in cache
	c.entries[serverName] = &cacheEntry{
		tools:       tools,
		cachedAt:    time.Now(),
		lastAccess:  time.Now(),
		accessCount: 0,
		serverName:  serverName,
	}

	c.metrics.mu.Lock()
	c.metrics.totalInserts++
	c.metrics.totalSize = len(c.entries)
	c.metrics.mu.Unlock()
}

// Invalidate removes tools for a server from cache
func (c *ToolCache) Invalidate(serverName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, serverName)

	c.metrics.mu.Lock()
	c.metrics.totalSize = len(c.entries)
	c.metrics.mu.Unlock()
}

// Clear removes all entries from cache
func (c *ToolCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)

	c.metrics.mu.Lock()
	c.metrics.totalSize = 0
	c.metrics.mu.Unlock()
}

// GetMetrics returns cache metrics
func (c *ToolCache) GetMetrics() CacheMetrics {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	hitRate := 0.0
	total := c.metrics.hits + c.metrics.misses
	if total > 0 {
		hitRate = float64(c.metrics.hits) / float64(total) * 100
	}

	return CacheMetrics{
		Hits:         c.metrics.hits,
		Misses:       c.metrics.misses,
		Evictions:    c.metrics.evictions,
		TotalInserts: c.metrics.totalInserts,
		CurrentSize:  c.metrics.totalSize,
		HitRate:      hitRate,
	}
}

// CacheMetrics contains cache statistics
type CacheMetrics struct {
	Hits         int64
	Misses       int64
	Evictions    int64
	TotalInserts int64
	CurrentSize  int
	HitRate      float64 // Percentage
}

// isExpired checks if an entry has expired
func (c *ToolCache) isExpired(entry *cacheEntry) bool {
	if c.config.TTL == 0 {
		return false // No expiration
	}
	return time.Since(entry.cachedAt) > c.config.TTL
}

// evict removes an entry based on eviction policy
func (c *ToolCache) evict() {
	if len(c.entries) == 0 {
		return
	}

	var victimKey string

	switch c.config.EvictionPolicy {
	case CachePolicyLRU:
		victimKey = c.findLRU()
	case CachePolicyLFU:
		victimKey = c.findLFU()
	case CachePolicyFIFO:
		victimKey = c.findFIFO()
	case CachePolicyTTL:
		victimKey = c.findExpired()
	default:
		victimKey = c.findLRU()
	}

	if victimKey != "" {
		delete(c.entries, victimKey)
		c.metrics.mu.Lock()
		c.metrics.evictions++
		c.metrics.totalSize = len(c.entries)
		c.metrics.mu.Unlock()
	}
}

// findLRU finds the least recently used entry
func (c *ToolCache) findLRU() string {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.lastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.lastAccess
		}
	}

	return oldestKey
}

// findLFU finds the least frequently used entry
func (c *ToolCache) findLFU() string {
	var victimKey string
	var minCount int64 = -1

	for key, entry := range c.entries {
		if minCount == -1 || entry.accessCount < minCount {
			victimKey = key
			minCount = entry.accessCount
		}
	}

	return victimKey
}

// findFIFO finds the oldest entry
func (c *ToolCache) findFIFO() string {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.cachedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.cachedAt
		}
	}

	return oldestKey
}

// findExpired finds an expired entry
func (c *ToolCache) findExpired() string {
	for key, entry := range c.entries {
		if c.isExpired(entry) {
			return key
		}
	}
	// If no expired, fallback to LRU
	return c.findLRU()
}

// cleanupLoop periodically removes expired entries
func (c *ToolCache) cleanupLoop() {
	ticker := time.NewTicker(c.config.CleanupPeriod)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired entries
func (c *ToolCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	toRemove := []string{}

	for key, entry := range c.entries {
		if c.isExpired(entry) {
			toRemove = append(toRemove, key)
		}
	}

	for _, key := range toRemove {
		delete(c.entries, key)
		c.metrics.mu.Lock()
		c.metrics.evictions++
		c.metrics.totalSize = len(c.entries)
		c.metrics.mu.Unlock()
	}
}

// recordHit increments hit counter
func (c *ToolCache) recordHit() {
	c.metrics.mu.Lock()
	c.metrics.hits++
	c.metrics.mu.Unlock()
}

// recordMiss increments miss counter
func (c *ToolCache) recordMiss() {
	c.metrics.mu.Lock()
	c.metrics.misses++
	c.metrics.mu.Unlock()
}

// Size returns the current cache size
func (c *ToolCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Has checks if a key exists in cache (without updating access)
func (c *ToolCache) Has(serverName string) bool {
	if !c.config.Enabled {
		return false
	}

	c.mu.RLock()
	entry, exists := c.entries[serverName]
	c.mu.RUnlock()

	if !exists {
		return false
	}

	return !c.isExpired(entry)
}

// GetCachedAt returns when the entry was cached
func (c *ToolCache) GetCachedAt(serverName string) (time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[serverName]
	if !exists {
		return time.Time{}, false
	}

	return entry.cachedAt, true
}

// GetAccessCount returns access count for an entry
func (c *ToolCache) GetAccessCount(serverName string) int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[serverName]
	if !exists {
		return 0
	}

	return entry.accessCount
}
