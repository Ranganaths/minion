package client

import (
	"testing"
	"time"
)

func TestToolCache_GetSet(t *testing.T) {
	cache := NewToolCache(DefaultCacheConfig())

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
		{Name: "tool2", Description: "Test tool 2"},
	}

	// Cache miss
	retrieved, found := cache.Get("test-server")
	if found {
		t.Error("Expected cache miss for non-existent key")
	}
	if retrieved != nil {
		t.Error("Expected nil tools for cache miss")
	}

	// Set tools
	cache.Set("test-server", tools)

	// Cache hit
	retrieved, found = cache.Get("test-server")
	if !found {
		t.Error("Expected cache hit")
	}

	if len(retrieved) != len(tools) {
		t.Errorf("Expected %d tools, got %d", len(tools), len(retrieved))
	}

	if retrieved[0].Name != tools[0].Name {
		t.Errorf("Expected tool name %s, got %s", tools[0].Name, retrieved[0].Name)
	}
}

func TestToolCache_TTLExpiration(t *testing.T) {
	config := &CacheConfig{
		Enabled:        true,
		TTL:            100 * time.Millisecond,
		MaxSize:        10,
		EvictionPolicy: CachePolicyTTL,
		CleanupPeriod:  50 * time.Millisecond,
	}
	cache := NewToolCache(config)

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
	}

	cache.Set("test-server", tools)

	// Should be cached
	_, found := cache.Get("test-server")
	if !found {
		t.Error("Expected cache hit before expiration")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, found = cache.Get("test-server")
	if found {
		t.Error("Expected cache miss after expiration")
	}
}

func TestToolCache_LRUEviction(t *testing.T) {
	config := &CacheConfig{
		Enabled:        true,
		TTL:            0, // No expiration
		MaxSize:        3,
		EvictionPolicy: CachePolicyLRU,
		CleanupPeriod:  1 * time.Minute,
	}
	cache := NewToolCache(config)

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
	}

	// Fill cache to max
	cache.Set("server1", tools)
	cache.Set("server2", tools)
	cache.Set("server3", tools)

	// Access server1 to make it recently used
	cache.Get("server1")
	time.Sleep(10 * time.Millisecond)

	// Access server3
	cache.Get("server3")
	time.Sleep(10 * time.Millisecond)

	// Add new entry - should evict server2 (least recently used)
	cache.Set("server4", tools)

	// server2 should be evicted
	_, found := cache.Get("server2")
	if found {
		t.Error("Expected server2 to be evicted (LRU)")
	}

	// server1 and server3 should still be cached
	_, found = cache.Get("server1")
	if !found {
		t.Error("Expected server1 to still be cached")
	}

	_, found = cache.Get("server3")
	if !found {
		t.Error("Expected server3 to still be cached")
	}
}

func TestToolCache_LFUEviction(t *testing.T) {
	config := &CacheConfig{
		Enabled:        true,
		TTL:            0,
		MaxSize:        3,
		EvictionPolicy: CachePolicyLFU,
		CleanupPeriod:  1 * time.Minute,
	}
	cache := NewToolCache(config)

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
	}

	// Fill cache
	cache.Set("server1", tools)
	cache.Set("server2", tools)
	cache.Set("server3", tools)

	// Access server1 multiple times
	cache.Get("server1")
	cache.Get("server1")
	cache.Get("server1")

	// Access server3 twice
	cache.Get("server3")
	cache.Get("server3")

	// server2 has been accessed 0 times (just set)
	// Add new entry - should evict server2 (least frequently used)
	cache.Set("server4", tools)

	// server2 should be evicted
	_, found := cache.Get("server2")
	if found {
		t.Error("Expected server2 to be evicted (LFU)")
	}

	// server1 should still be cached
	_, found = cache.Get("server1")
	if !found {
		t.Error("Expected server1 to still be cached")
	}
}

func TestToolCache_FIFOEviction(t *testing.T) {
	config := &CacheConfig{
		Enabled:        true,
		TTL:            0,
		MaxSize:        3,
		EvictionPolicy: CachePolicyFIFO,
		CleanupPeriod:  1 * time.Minute,
	}
	cache := NewToolCache(config)

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
	}

	// Fill cache in order
	cache.Set("server1", tools)
	time.Sleep(10 * time.Millisecond)
	cache.Set("server2", tools)
	time.Sleep(10 * time.Millisecond)
	cache.Set("server3", tools)

	// Add new entry - should evict server1 (first in)
	cache.Set("server4", tools)

	// server1 should be evicted
	_, found := cache.Get("server1")
	if found {
		t.Error("Expected server1 to be evicted (FIFO)")
	}

	// server2 and server3 should still be cached
	_, found = cache.Get("server2")
	if !found {
		t.Error("Expected server2 to still be cached")
	}

	_, found = cache.Get("server3")
	if !found {
		t.Error("Expected server3 to still be cached")
	}
}

func TestToolCache_Metrics(t *testing.T) {
	cache := NewToolCache(DefaultCacheConfig())

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
	}

	// Initial metrics
	metrics := cache.GetMetrics()
	if metrics.Hits != 0 || metrics.Misses != 0 {
		t.Error("Expected zero initial metrics")
	}

	// Cache miss
	cache.Get("test-server")

	metrics = cache.GetMetrics()
	if metrics.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", metrics.Misses)
	}

	// Set and hit
	cache.Set("test-server", tools)
	cache.Get("test-server")

	metrics = cache.GetMetrics()
	if metrics.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", metrics.Hits)
	}

	if metrics.TotalInserts != 1 {
		t.Errorf("Expected 1 insert, got %d", metrics.TotalInserts)
	}

	if metrics.CurrentSize != 1 {
		t.Errorf("Expected size 1, got %d", metrics.CurrentSize)
	}

	// Hit rate
	if metrics.HitRate < 49.0 || metrics.HitRate > 51.0 {
		t.Errorf("Expected hit rate ~50%%, got %.2f%%", metrics.HitRate)
	}
}

func TestToolCache_Invalidate(t *testing.T) {
	cache := NewToolCache(DefaultCacheConfig())

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
	}

	cache.Set("test-server", tools)

	// Verify cached
	_, found := cache.Get("test-server")
	if !found {
		t.Error("Expected cache hit")
	}

	// Invalidate
	cache.Invalidate("test-server")

	// Should be gone
	_, found = cache.Get("test-server")
	if found {
		t.Error("Expected cache miss after invalidation")
	}

	metrics := cache.GetMetrics()
	if metrics.CurrentSize != 0 {
		t.Errorf("Expected size 0 after invalidation, got %d", metrics.CurrentSize)
	}
}

func TestToolCache_Clear(t *testing.T) {
	cache := NewToolCache(DefaultCacheConfig())

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
	}

	// Add multiple entries
	cache.Set("server1", tools)
	cache.Set("server2", tools)
	cache.Set("server3", tools)

	metrics := cache.GetMetrics()
	if metrics.CurrentSize != 3 {
		t.Errorf("Expected size 3, got %d", metrics.CurrentSize)
	}

	// Clear
	cache.Clear()

	// All should be gone
	_, found := cache.Get("server1")
	if found {
		t.Error("Expected cache miss after clear")
	}

	metrics = cache.GetMetrics()
	if metrics.CurrentSize != 0 {
		t.Errorf("Expected size 0 after clear, got %d", metrics.CurrentSize)
	}
}

func TestToolCache_Has(t *testing.T) {
	cache := NewToolCache(DefaultCacheConfig())

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
	}

	// Not cached
	if cache.Has("test-server") {
		t.Error("Expected Has to return false for non-existent key")
	}

	// Cache it
	cache.Set("test-server", tools)

	// Should exist
	if !cache.Has("test-server") {
		t.Error("Expected Has to return true for cached key")
	}

	// Has should not update access metrics
	accessCount := cache.GetAccessCount("test-server")
	cache.Has("test-server")
	if cache.GetAccessCount("test-server") != accessCount {
		t.Error("Expected Has to not update access count")
	}
}

func TestToolCache_GetCachedAt(t *testing.T) {
	cache := NewToolCache(DefaultCacheConfig())

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
	}

	// Not cached
	_, found := cache.GetCachedAt("test-server")
	if found {
		t.Error("Expected GetCachedAt to return false for non-existent key")
	}

	// Cache it
	before := time.Now()
	cache.Set("test-server", tools)
	after := time.Now()

	// Get cached time
	cachedAt, found := cache.GetCachedAt("test-server")
	if !found {
		t.Error("Expected GetCachedAt to return true for cached key")
	}

	if cachedAt.Before(before) || cachedAt.After(after) {
		t.Error("Expected cached time to be between before and after")
	}
}

func TestToolCache_GetAccessCount(t *testing.T) {
	cache := NewToolCache(DefaultCacheConfig())

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
	}

	// Not cached
	count := cache.GetAccessCount("test-server")
	if count != 0 {
		t.Errorf("Expected 0 access count for non-existent key, got %d", count)
	}

	// Cache and access
	cache.Set("test-server", tools)

	// Access multiple times
	cache.Get("test-server")
	cache.Get("test-server")
	cache.Get("test-server")

	count = cache.GetAccessCount("test-server")
	if count != 3 {
		t.Errorf("Expected 3 accesses, got %d", count)
	}
}

func TestToolCache_Size(t *testing.T) {
	cache := NewToolCache(DefaultCacheConfig())

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
	}

	if cache.Size() != 0 {
		t.Errorf("Expected initial size 0, got %d", cache.Size())
	}

	cache.Set("server1", tools)
	if cache.Size() != 1 {
		t.Errorf("Expected size 1, got %d", cache.Size())
	}

	cache.Set("server2", tools)
	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}

	cache.Invalidate("server1")
	if cache.Size() != 1 {
		t.Errorf("Expected size 1 after invalidation, got %d", cache.Size())
	}
}

func TestToolCache_Disabled(t *testing.T) {
	config := &CacheConfig{
		Enabled:        false,
		TTL:            5 * time.Minute,
		MaxSize:        100,
		EvictionPolicy: CachePolicyLRU,
		CleanupPeriod:  1 * time.Minute,
	}
	cache := NewToolCache(config)

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
	}

	// Set should do nothing
	cache.Set("test-server", tools)

	// Get should always miss
	_, found := cache.Get("test-server")
	if found {
		t.Error("Expected cache miss when disabled")
	}

	// Has should return false
	if cache.Has("test-server") {
		t.Error("Expected Has to return false when disabled")
	}
}

func TestToolCache_CleanupLoop(t *testing.T) {
	config := &CacheConfig{
		Enabled:        true,
		TTL:            50 * time.Millisecond,
		MaxSize:        100,
		EvictionPolicy: CachePolicyTTL,
		CleanupPeriod:  50 * time.Millisecond,
	}
	cache := NewToolCache(config)

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
	}

	// Add multiple entries
	cache.Set("server1", tools)
	cache.Set("server2", tools)
	cache.Set("server3", tools)

	if cache.Size() != 3 {
		t.Fatalf("Expected size 3, got %d", cache.Size())
	}

	// Wait for cleanup to run (TTL + cleanup period)
	time.Sleep(150 * time.Millisecond)

	// All should be cleaned up
	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after cleanup, got %d", cache.Size())
	}

	metrics := cache.GetMetrics()
	if metrics.Evictions != 3 {
		t.Errorf("Expected 3 evictions from cleanup, got %d", metrics.Evictions)
	}
}

func TestToolCache_ConcurrentAccess(t *testing.T) {
	cache := NewToolCache(DefaultCacheConfig())

	tools := []MCPTool{
		{Name: "tool1", Description: "Test tool 1"},
	}

	done := make(chan bool)

	// Concurrent sets
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				cache.Set("test-server", tools)
				cache.Get("test-server")
			}
			done <- true
		}(i)
	}

	// Wait for completion
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify cache still works
	_, found := cache.Get("test-server")
	if !found {
		t.Error("Expected cache to work after concurrent access")
	}

	metrics := cache.GetMetrics()
	if metrics.Hits == 0 {
		t.Error("Expected some hits from concurrent access")
	}
}
