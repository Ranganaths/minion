package memory

import (
	"context"
	"testing"
	"time"
)

func TestTieredMemoryManager(t *testing.T) {
	hot := NewMemoryHotStore()
	warm := NewMemoryWarmStore()

	manager := NewTieredMemoryManager(hot, warm, nil, DefaultTieredMemoryConfig())

	ctx := context.Background()

	// Test Set
	item := &TieredMemoryItem{
		Key:      "test-key",
		Value:    []byte("test-value"),
		AgentID:  "agent-1",
		ItemType: "message",
	}

	err := manager.Set(ctx, item)
	if err != nil {
		t.Fatalf("Failed to set item: %v", err)
	}

	// Test Get from hot tier
	retrieved, err := manager.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Failed to get item: %v", err)
	}

	if string(retrieved.Value) != "test-value" {
		t.Errorf("Expected value 'test-value', got '%s'", string(retrieved.Value))
	}

	if retrieved.Tier != TierHot {
		t.Errorf("Expected tier hot, got %s", retrieved.Tier)
	}

	// Check metrics
	metrics := manager.GetMetrics()
	if metrics.HotHits != 1 {
		t.Errorf("Expected 1 hot hit, got %d", metrics.HotHits)
	}
}

func TestTieredMemoryManagerWarmFallback(t *testing.T) {
	hot := NewMemoryHotStore()
	warm := NewMemoryWarmStore()

	config := DefaultTieredMemoryConfig()
	config.PromoteOnAccess = true
	manager := NewTieredMemoryManager(hot, warm, nil, config)

	ctx := context.Background()

	// Store directly in warm tier (simulating demotion)
	warmItem := &TieredMemoryItem{
		Key:        "warm-key",
		Value:      []byte("warm-value"),
		Tier:       TierWarm,
		AgentID:    "agent-1",
		ItemType:   "message",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		AccessedAt: time.Now(),
	}
	warm.Set(ctx, warmItem)

	// Get should find it in warm tier
	retrieved, err := manager.Get(ctx, "warm-key")
	if err != nil {
		t.Fatalf("Failed to get from warm tier: %v", err)
	}

	if string(retrieved.Value) != "warm-value" {
		t.Errorf("Expected value 'warm-value', got '%s'", string(retrieved.Value))
	}

	// Check metrics
	metrics := manager.GetMetrics()
	if metrics.HotMisses != 1 {
		t.Errorf("Expected 1 hot miss, got %d", metrics.HotMisses)
	}
	if metrics.WarmHits != 1 {
		t.Errorf("Expected 1 warm hit, got %d", metrics.WarmHits)
	}

	// Item should be promoted to hot tier
	if metrics.Promotions != 1 {
		t.Errorf("Expected 1 promotion, got %d", metrics.Promotions)
	}
}

func TestTieredMemoryManagerDelete(t *testing.T) {
	hot := NewMemoryHotStore()
	warm := NewMemoryWarmStore()

	manager := NewTieredMemoryManager(hot, warm, nil, DefaultTieredMemoryConfig())

	ctx := context.Background()

	// Set item
	item := &TieredMemoryItem{
		Key:   "delete-key",
		Value: []byte("delete-value"),
	}
	manager.Set(ctx, item)

	// Delete
	err := manager.Delete(ctx, "delete-key")
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	// Should not be found
	_, err = manager.Get(ctx, "delete-key")
	if err == nil {
		t.Error("Expected error getting deleted item")
	}
}

func TestTieredMemoryManagerQuery(t *testing.T) {
	hot := NewMemoryHotStore()
	warm := NewMemoryWarmStore()

	manager := NewTieredMemoryManager(hot, warm, nil, DefaultTieredMemoryConfig())

	ctx := context.Background()

	// Set multiple items
	items := []*TieredMemoryItem{
		{Key: "query-1", Value: []byte("value-1"), AgentID: "agent-1", ItemType: "message"},
		{Key: "query-2", Value: []byte("value-2"), AgentID: "agent-1", ItemType: "message"},
		{Key: "query-3", Value: []byte("value-3"), AgentID: "agent-2", ItemType: "message"},
	}

	for _, item := range items {
		manager.Set(ctx, item)
	}

	// Query by agent ID
	results, err := manager.Query(ctx, &TieredMemoryQuery{
		AgentID: "agent-1",
	})
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results for agent-1, got %d", len(results))
	}
}

func TestMemoryHotStore(t *testing.T) {
	store := NewMemoryHotStore()
	ctx := context.Background()

	// Test Set and Get
	err := store.Set(ctx, "key1", []byte("value1"), time.Hour)
	if err != nil {
		t.Fatalf("Failed to set: %v", err)
	}

	value, err := store.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Failed to get: %v", err)
	}

	if string(value) != "value1" {
		t.Errorf("Expected 'value1', got '%s'", string(value))
	}

	// Test Delete
	err = store.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	_, err = store.Get(ctx, "key1")
	if err == nil {
		t.Error("Expected error getting deleted key")
	}

	// Test Keys
	store.Set(ctx, "key-a", []byte("a"), 0)
	store.Set(ctx, "key-b", []byte("b"), 0)

	keys, err := store.Keys(ctx, "*")
	if err != nil {
		t.Fatalf("Failed to get keys: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}

func TestMemoryWarmStore(t *testing.T) {
	store := NewMemoryWarmStore()
	ctx := context.Background()

	// Test Set and Get
	item := &TieredMemoryItem{
		Key:       "warm-1",
		Value:     []byte("warm-value"),
		AgentID:   "agent-1",
		ItemType:  "snapshot",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := store.Set(ctx, item)
	if err != nil {
		t.Fatalf("Failed to set: %v", err)
	}

	retrieved, err := store.Get(ctx, "warm-1")
	if err != nil {
		t.Fatalf("Failed to get: %v", err)
	}

	if string(retrieved.Value) != "warm-value" {
		t.Errorf("Expected 'warm-value', got '%s'", string(retrieved.Value))
	}

	// Test Query
	store.Set(ctx, &TieredMemoryItem{
		Key:      "warm-2",
		Value:    []byte("value-2"),
		AgentID:  "agent-1",
		ItemType: "message",
	})
	store.Set(ctx, &TieredMemoryItem{
		Key:      "warm-3",
		Value:    []byte("value-3"),
		AgentID:  "agent-2",
		ItemType: "snapshot",
	})

	results, err := store.Query(ctx, &TieredMemoryQuery{
		AgentID: "agent-1",
	})
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Test GetOlderThan - create a new store to isolate the test
	store2 := NewMemoryWarmStore()

	// Add recent item
	recentItem := &TieredMemoryItem{
		Key:       "recent-item",
		Value:     []byte("recent"),
		UpdatedAt: time.Now(),
	}
	store2.Set(ctx, recentItem)

	// Add old item
	oldItem := &TieredMemoryItem{
		Key:       "old-item",
		Value:     []byte("old"),
		UpdatedAt: time.Now().Add(-48 * time.Hour),
	}
	store2.Set(ctx, oldItem)

	oldItems, err := store2.GetOlderThan(ctx, 24*time.Hour, 10)
	if err != nil {
		t.Fatalf("Failed to get older than: %v", err)
	}

	if len(oldItems) != 1 {
		t.Errorf("Expected 1 old item, got %d", len(oldItems))
	}
}

func TestTieredMemoryManagerStartStop(t *testing.T) {
	hot := NewMemoryHotStore()
	warm := NewMemoryWarmStore()

	config := DefaultTieredMemoryConfig()
	config.CompactionInterval = 100 * time.Millisecond
	manager := NewTieredMemoryManager(hot, warm, nil, config)

	// Start background workers
	manager.Start()

	// Give it time to run
	time.Sleep(200 * time.Millisecond)

	// Stop should not hang
	done := make(chan struct{})
	go func() {
		manager.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Stop timed out")
	}
}

func TestTieredMemoryManagerClose(t *testing.T) {
	hot := NewMemoryHotStore()
	warm := NewMemoryWarmStore()

	manager := NewTieredMemoryManager(hot, warm, nil, DefaultTieredMemoryConfig())
	manager.Start()

	err := manager.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestTieredMemoryItemMetadata(t *testing.T) {
	hot := NewMemoryHotStore()
	warm := NewMemoryWarmStore()

	manager := NewTieredMemoryManager(hot, warm, nil, DefaultTieredMemoryConfig())
	ctx := context.Background()

	// Set item with metadata
	item := &TieredMemoryItem{
		Key:      "meta-key",
		Value:    []byte("meta-value"),
		AgentID:  "agent-1",
		ItemType: "custom",
		Metadata: map[string]interface{}{
			"priority": 5,
			"tags":     []string{"important", "urgent"},
		},
	}

	err := manager.Set(ctx, item)
	if err != nil {
		t.Fatalf("Failed to set item with metadata: %v", err)
	}

	retrieved, err := manager.Get(ctx, "meta-key")
	if err != nil {
		t.Fatalf("Failed to get item: %v", err)
	}

	if retrieved.Metadata == nil {
		t.Error("Expected metadata to be preserved")
	}

	if retrieved.ItemType != "custom" {
		t.Errorf("Expected item type 'custom', got '%s'", retrieved.ItemType)
	}
}

// TestTieredMemoryManagerWithOptions tests the functional options API for backward compatibility
func TestTieredMemoryManagerWithOptions(t *testing.T) {
	ctx := context.Background()

	// Test using functional options - new, flexible API
	manager := NewTieredMemoryManagerWithOptions(
		WithInMemoryHotStore(),
		WithInMemoryWarmStore(),
		WithHotTTL(2*time.Hour),
		WithPromoteOnAccess(true),
	)

	// Test Set
	item := &TieredMemoryItem{
		Key:      "options-key",
		Value:    []byte("options-value"),
		AgentID:  "agent-1",
		ItemType: "message",
	}

	err := manager.Set(ctx, item)
	if err != nil {
		t.Fatalf("Failed to set item: %v", err)
	}

	// Test Get
	retrieved, err := manager.Get(ctx, "options-key")
	if err != nil {
		t.Fatalf("Failed to get item: %v", err)
	}

	if string(retrieved.Value) != "options-value" {
		t.Errorf("Expected value 'options-value', got '%s'", string(retrieved.Value))
	}

	manager.Close()
}

// TestTieredMemoryManagerWithOptionsMinimal tests creating manager with minimal options
func TestTieredMemoryManagerWithOptionsMinimal(t *testing.T) {
	// Create with just in-memory hot store - warm and cold are nil
	manager := NewTieredMemoryManagerWithOptions(
		WithInMemoryHotStore(),
	)

	ctx := context.Background()

	// Should still work with only hot tier
	item := &TieredMemoryItem{
		Key:   "minimal-key",
		Value: []byte("minimal-value"),
	}

	err := manager.Set(ctx, item)
	if err != nil {
		t.Fatalf("Failed to set item: %v", err)
	}

	retrieved, err := manager.Get(ctx, "minimal-key")
	if err != nil {
		t.Fatalf("Failed to get item: %v", err)
	}

	if string(retrieved.Value) != "minimal-value" {
		t.Errorf("Expected value 'minimal-value', got '%s'", string(retrieved.Value))
	}

	manager.Close()
}

// TestTieredMemoryManagerWithCustomStores tests using custom store implementations
func TestTieredMemoryManagerWithCustomStores(t *testing.T) {
	// Create custom stores
	customHot := NewMemoryHotStore()
	customWarm := NewMemoryWarmStore()

	// Use WithHotStore and WithWarmStore to inject custom implementations
	manager := NewTieredMemoryManagerWithOptions(
		WithHotStore(customHot),
		WithWarmStore(customWarm),
		WithHotTTL(30*time.Minute),
		WithWarmTTL(24*time.Hour),
	)

	ctx := context.Background()

	item := &TieredMemoryItem{
		Key:      "custom-key",
		Value:    []byte("custom-value"),
		AgentID:  "agent-custom",
		ItemType: "test",
	}

	err := manager.Set(ctx, item)
	if err != nil {
		t.Fatalf("Failed to set item: %v", err)
	}

	// Verify item is in the custom hot store directly
	data, err := customHot.Get(ctx, "custom-key")
	if err != nil {
		t.Fatalf("Item not found in custom hot store: %v", err)
	}
	if len(data) == 0 {
		t.Error("Expected non-empty data in custom hot store")
	}

	manager.Close()
}

// TestBackwardCompatibility ensures the original API still works
func TestBackwardCompatibility(t *testing.T) {
	hot := NewMemoryHotStore()
	warm := NewMemoryWarmStore()
	config := DefaultTieredMemoryConfig()

	// Original API - should still work exactly as before
	manager := NewTieredMemoryManager(hot, warm, nil, config)

	ctx := context.Background()

	item := &TieredMemoryItem{
		Key:      "compat-key",
		Value:    []byte("compat-value"),
		AgentID:  "agent-1",
		ItemType: "message",
	}

	err := manager.Set(ctx, item)
	if err != nil {
		t.Fatalf("Original API failed to set item: %v", err)
	}

	retrieved, err := manager.Get(ctx, "compat-key")
	if err != nil {
		t.Fatalf("Original API failed to get item: %v", err)
	}

	if string(retrieved.Value) != "compat-value" {
		t.Errorf("Expected value 'compat-value', got '%s'", string(retrieved.Value))
	}

	manager.Close()
}
