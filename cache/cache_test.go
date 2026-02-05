package cache

import (
	"context"
	"testing"
	"time"
)

// MockEmbedder for testing
type MockEmbedder struct {
	embeddings map[string][]float32
}

func NewMockEmbedder() *MockEmbedder {
	return &MockEmbedder{
		embeddings: make(map[string][]float32),
	}
}

func (m *MockEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	if emb, ok := m.embeddings[text]; ok {
		return emb, nil
	}
	// Generate a simple embedding based on text length
	emb := make([]float32, 128)
	for i := range emb {
		emb[i] = float32(i) * float32(len(text)) / 1000.0
	}
	m.embeddings[text] = emb
	return emb, nil
}

func (m *MockEmbedder) SetEmbedding(text string, embedding []float32) {
	m.embeddings[text] = embedding
}

// MockLLMProvider for testing
type MockLLMProvider struct {
	completionResponse *CompletionResponse
	chatResponse       *ChatResponse
	callCount          int
}

func NewMockLLMProvider() *MockLLMProvider {
	return &MockLLMProvider{
		completionResponse: &CompletionResponse{
			Text:         "mock response",
			TokensUsed:   100,
			FinishReason: "stop",
			Model:        "test-model",
		},
		chatResponse: &ChatResponse{
			Message:      ChatMessage{Role: "assistant", Content: "mock chat response"},
			TokensUsed:   100,
			FinishReason: "stop",
			Model:        "test-model",
		},
	}
}

func (m *MockLLMProvider) GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	m.callCount++
	return m.completionResponse, nil
}

func (m *MockLLMProvider) GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	m.callCount++
	return m.chatResponse, nil
}

func (m *MockLLMProvider) Name() string {
	return "mock"
}

// Tests for SemanticLLMCache

func TestSemanticLLMCacheBasic(t *testing.T) {
	cache := NewSemanticLLMCache(SemanticLLMCacheConfig{
		TTL:     time.Hour,
		MaxSize: 100,
	})
	defer cache.Close()

	ctx := context.Background()

	// Set an entry
	entry := &LLMCacheEntry{
		Prompt:     "What is Go?",
		Response:   "Go is a programming language.",
		Model:      "gpt-4",
		TokensUsed: 50,
	}

	err := cache.Set(ctx, entry)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get the entry
	retrieved, ok := cache.Get(ctx, "What is Go?", "gpt-4")
	if !ok {
		t.Fatal("Expected cache hit")
	}

	if retrieved.Response != "Go is a programming language." {
		t.Errorf("Expected response 'Go is a programming language.', got '%s'", retrieved.Response)
	}
}

func TestSemanticLLMCacheMiss(t *testing.T) {
	cache := NewSemanticLLMCache(SemanticLLMCacheConfig{
		TTL:     time.Hour,
		MaxSize: 100,
	})
	defer cache.Close()

	ctx := context.Background()

	// Try to get non-existent entry
	_, ok := cache.Get(ctx, "unknown prompt", "gpt-4")
	if ok {
		t.Fatal("Expected cache miss")
	}

	stats := cache.GetStats()
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
}

func TestSemanticLLMCacheExpiration(t *testing.T) {
	cache := NewSemanticLLMCache(SemanticLLMCacheConfig{
		TTL:     100 * time.Millisecond,
		MaxSize: 100,
	})
	defer cache.Close()

	ctx := context.Background()

	entry := &LLMCacheEntry{
		Prompt:     "test",
		Response:   "response",
		Model:      "gpt-4",
		TokensUsed: 10,
	}

	cache.Set(ctx, entry)

	// Should hit immediately
	_, ok := cache.Get(ctx, "test", "gpt-4")
	if !ok {
		t.Fatal("Expected cache hit")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should miss after expiration
	_, ok = cache.Get(ctx, "test", "gpt-4")
	if ok {
		t.Fatal("Expected cache miss after expiration")
	}
}

func TestSemanticLLMCacheWithEmbeddings(t *testing.T) {
	embedder := NewMockEmbedder()

	// Set up similar embeddings for similar prompts
	emb1 := make([]float32, 128)
	emb2 := make([]float32, 128)
	for i := range emb1 {
		emb1[i] = float32(i) / 128.0
		emb2[i] = float32(i) / 128.0 + 0.001 // Very similar
	}

	embedder.SetEmbedding("What is Go programming?", emb1)
	embedder.SetEmbedding("Tell me about Go programming", emb2)

	cache := NewSemanticLLMCache(SemanticLLMCacheConfig{
		Embedder:      embedder,
		TTL:           time.Hour,
		MaxSize:       100,
		MinSimilarity: 0.95,
	})
	defer cache.Close()

	ctx := context.Background()

	// Set an entry with embedding
	entry := &LLMCacheEntry{
		Prompt:     "What is Go programming?",
		Response:   "Go is a statically typed programming language.",
		Model:      "gpt-4",
		TokensUsed: 50,
	}

	cache.Set(ctx, entry)

	// Try semantic search with a similar prompt
	retrieved, ok := cache.GetSemantic(ctx, "Tell me about Go programming", "gpt-4", emb2, 0.95)
	if !ok {
		t.Fatal("Expected semantic cache hit for similar prompt")
	}

	if retrieved.Response != "Go is a statically typed programming language." {
		t.Errorf("Unexpected response: %s", retrieved.Response)
	}

	stats := cache.GetStats()
	if stats.SemanticHits != 1 {
		t.Errorf("Expected 1 semantic hit, got %d", stats.SemanticHits)
	}
}

func TestSemanticLLMCacheEviction(t *testing.T) {
	cache := NewSemanticLLMCache(SemanticLLMCacheConfig{
		TTL:     time.Hour,
		MaxSize: 3, // Small size to test eviction
	})
	defer cache.Close()

	ctx := context.Background()

	// Add 3 entries
	for i := 1; i <= 3; i++ {
		entry := &LLMCacheEntry{
			Prompt:     string(rune('a' + i)),
			Response:   "response",
			Model:      "gpt-4",
			TokensUsed: 10,
		}
		cache.Set(ctx, entry)
		// Access all except the first one
		if i > 1 {
			cache.Get(ctx, string(rune('a'+i)), "gpt-4")
		}
	}

	// Add a 4th entry, should evict the least recently used (first one)
	entry := &LLMCacheEntry{
		Prompt:     "e",
		Response:   "response",
		Model:      "gpt-4",
		TokensUsed: 10,
	}
	cache.Set(ctx, entry)

	stats := cache.GetStats()
	if stats.Size != 3 {
		t.Errorf("Expected size 3, got %d", stats.Size)
	}
}

func TestSemanticLLMCacheClear(t *testing.T) {
	cache := NewSemanticLLMCache(SemanticLLMCacheConfig{
		TTL:     time.Hour,
		MaxSize: 100,
	})
	defer cache.Close()

	ctx := context.Background()

	// Add entries
	for i := 0; i < 5; i++ {
		entry := &LLMCacheEntry{
			Prompt:     string(rune('a' + i)),
			Response:   "response",
			Model:      "gpt-4",
			TokensUsed: 10,
		}
		cache.Set(ctx, entry)
	}

	// Clear
	cache.Clear(ctx)

	stats := cache.GetStats()
	if stats.Size != 0 {
		t.Errorf("Expected size 0 after clear, got %d", stats.Size)
	}
}

// Tests for RedisLLMCache

func TestRedisLLMCacheBasic(t *testing.T) {
	mockRedis := NewMockRedisClient()
	cache, err := NewRedisLLMCache(RedisLLMCacheConfig{
		Client: mockRedis,
		TTL:    time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	entry := &LLMCacheEntry{
		Prompt:     "test prompt",
		Response:   "test response",
		Model:      "gpt-4",
		TokensUsed: 100,
	}

	// Set
	err = cache.Set(ctx, entry)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get
	retrieved, ok := cache.Get(ctx, "test prompt", "gpt-4")
	if !ok {
		t.Fatal("Expected cache hit")
	}

	if retrieved.Response != "test response" {
		t.Errorf("Expected 'test response', got '%s'", retrieved.Response)
	}
}

func TestRedisLLMCacheWithSemanticIndex(t *testing.T) {
	mockRedis := NewMockRedisClient()
	embedder := NewMockEmbedder()

	cache, err := NewRedisLLMCache(RedisLLMCacheConfig{
		Client:         mockRedis,
		TTL:            time.Hour,
		Embedder:       embedder,
		LocalCacheSize: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	entry := &LLMCacheEntry{
		Prompt:     "What is Python?",
		Response:   "Python is a programming language.",
		Model:      "gpt-4",
		TokensUsed: 50,
	}

	cache.Set(ctx, entry)

	// Get with embedding
	emb, _ := embedder.EmbedQuery(ctx, "What is Python?")
	retrieved, ok := cache.GetSemantic(ctx, "What is Python?", "gpt-4", emb, 0.95)
	if !ok {
		t.Fatal("Expected cache hit")
	}

	if retrieved.Response != "Python is a programming language." {
		t.Errorf("Unexpected response: %s", retrieved.Response)
	}
}

func TestRedisLLMCacheDelete(t *testing.T) {
	mockRedis := NewMockRedisClient()
	cache, _ := NewRedisLLMCache(RedisLLMCacheConfig{
		Client: mockRedis,
		TTL:    time.Hour,
	})
	defer cache.Close()

	ctx := context.Background()

	entry := &LLMCacheEntry{
		Prompt:     "to delete",
		Response:   "response",
		Model:      "gpt-4",
		TokensUsed: 10,
	}

	cache.Set(ctx, entry)

	// Delete
	err := cache.Delete(ctx, entry.PromptHash)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should miss now
	_, ok := cache.Get(ctx, "to delete", "gpt-4")
	if ok {
		t.Fatal("Expected cache miss after delete")
	}
}

// Tests for CachedProvider

func TestCachedProviderCompletion(t *testing.T) {
	mockProvider := NewMockLLMProvider()
	cache := NewSemanticLLMCache(SemanticLLMCacheConfig{
		TTL:     time.Hour,
		MaxSize: 100,
	})
	defer cache.Close()

	cachedProvider := NewCachedProvider(CachedProviderConfig{
		Provider: mockProvider,
		Cache:    cache,
	})

	ctx := context.Background()
	req := &CompletionRequest{
		Model:      "gpt-4",
		UserPrompt: "What is AI?",
	}

	// First call - should call provider
	resp1, err := cachedProvider.GenerateCompletion(ctx, req)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	if mockProvider.callCount != 1 {
		t.Errorf("Expected 1 provider call, got %d", mockProvider.callCount)
	}

	// Second call - should hit cache
	resp2, err := cachedProvider.GenerateCompletion(ctx, req)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	if mockProvider.callCount != 1 {
		t.Errorf("Expected provider call count to stay at 1, got %d", mockProvider.callCount)
	}

	if resp2.FinishReason != "cached" {
		t.Error("Expected cached response")
	}

	if resp1.Text != resp2.Text {
		t.Error("Responses should match")
	}
}

func TestCachedProviderChat(t *testing.T) {
	mockProvider := NewMockLLMProvider()
	cache := NewSemanticLLMCache(SemanticLLMCacheConfig{
		TTL:     time.Hour,
		MaxSize: 100,
	})
	defer cache.Close()

	cachedProvider := NewCachedProvider(CachedProviderConfig{
		Provider: mockProvider,
		Cache:    cache,
	})

	ctx := context.Background()
	req := &ChatRequest{
		Model: "gpt-4",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello!"},
		},
	}

	// First call
	_, err := cachedProvider.GenerateChat(ctx, req)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	// Second call - should hit cache
	resp2, err := cachedProvider.GenerateChat(ctx, req)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	if mockProvider.callCount != 1 {
		t.Errorf("Expected 1 provider call, got %d", mockProvider.callCount)
	}

	if resp2.FinishReason != "cached" {
		t.Error("Expected cached response")
	}
}

func TestCachedProviderWithSemanticCaching(t *testing.T) {
	mockProvider := NewMockLLMProvider()
	embedder := NewMockEmbedder()

	// Set up similar embeddings
	emb1 := make([]float32, 128)
	emb2 := make([]float32, 128)
	for i := range emb1 {
		emb1[i] = float32(i) / 128.0
		emb2[i] = float32(i) / 128.0 + 0.0001
	}
	embedder.SetEmbedding("What is machine learning?", emb1)
	embedder.SetEmbedding("Explain machine learning", emb2)

	cache := NewSemanticLLMCache(SemanticLLMCacheConfig{
		Embedder:      embedder,
		TTL:           time.Hour,
		MaxSize:       100,
		MinSimilarity: 0.99,
	})
	defer cache.Close()

	cachedProvider := NewCachedProvider(CachedProviderConfig{
		Provider:            mockProvider,
		Cache:               cache,
		Embedder:            embedder,
		EnableSemantic:      true,
		SimilarityThreshold: 0.99,
	})

	ctx := context.Background()

	// First call
	_, err := cachedProvider.GenerateCompletion(ctx, &CompletionRequest{
		Model:      "gpt-4",
		UserPrompt: "What is machine learning?",
	})
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	// Second call with similar prompt
	resp2, err := cachedProvider.GenerateCompletion(ctx, &CompletionRequest{
		Model:      "gpt-4",
		UserPrompt: "Explain machine learning",
	})
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	// Should hit semantic cache
	if mockProvider.callCount != 1 {
		t.Errorf("Expected 1 provider call (semantic hit), got %d", mockProvider.callCount)
	}

	if resp2.FinishReason != "cached" {
		t.Error("Expected cached response from semantic match")
	}
}

func TestCachedProviderModelFiltering(t *testing.T) {
	mockProvider := NewMockLLMProvider()
	cache := NewSemanticLLMCache(SemanticLLMCacheConfig{
		TTL:     time.Hour,
		MaxSize: 100,
	})
	defer cache.Close()

	cachedProvider := NewCachedProvider(CachedProviderConfig{
		Provider:        mockProvider,
		Cache:           cache,
		CacheableModels: []string{"gpt-4"}, // Only cache gpt-4
	})

	ctx := context.Background()

	// GPT-4 should be cached
	cachedProvider.GenerateCompletion(ctx, &CompletionRequest{
		Model:      "gpt-4",
		UserPrompt: "test",
	})
	cachedProvider.GenerateCompletion(ctx, &CompletionRequest{
		Model:      "gpt-4",
		UserPrompt: "test",
	})

	// Should only have 1 call (second was cached)
	if mockProvider.callCount != 1 {
		t.Errorf("Expected 1 call for gpt-4, got %d", mockProvider.callCount)
	}

	// GPT-3.5 should NOT be cached
	mockProvider.callCount = 0
	cachedProvider.GenerateCompletion(ctx, &CompletionRequest{
		Model:      "gpt-3.5-turbo",
		UserPrompt: "test",
	})
	cachedProvider.GenerateCompletion(ctx, &CompletionRequest{
		Model:      "gpt-3.5-turbo",
		UserPrompt: "test",
	})

	// Should have 2 calls (not cached)
	if mockProvider.callCount != 2 {
		t.Errorf("Expected 2 calls for gpt-3.5, got %d", mockProvider.callCount)
	}
}

func TestCachedProviderStats(t *testing.T) {
	mockProvider := NewMockLLMProvider()
	cache := NewSemanticLLMCache(SemanticLLMCacheConfig{
		TTL:     time.Hour,
		MaxSize: 100,
	})
	defer cache.Close()

	cachedProvider := NewCachedProvider(CachedProviderConfig{
		Provider: mockProvider,
		Cache:    cache,
	})

	ctx := context.Background()
	req := &CompletionRequest{
		Model:      "gpt-4",
		UserPrompt: "test",
	}

	// Generate some activity
	cachedProvider.GenerateCompletion(ctx, req)
	cachedProvider.GenerateCompletion(ctx, req) // Cache hit
	cachedProvider.GenerateCompletion(ctx, req) // Cache hit

	stats := cachedProvider.GetStats()
	if stats.Hits != 2 {
		t.Errorf("Expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
}

// Tests for CacheKeyBuilder

func TestCacheKeyBuilder(t *testing.T) {
	key1 := NewCacheKeyBuilder().
		Model("gpt-4").
		Prompt("Hello").
		Build()

	key2 := NewCacheKeyBuilder().
		Model("gpt-4").
		Prompt("Hello").
		Build()

	if key1 != key2 {
		t.Error("Same inputs should produce same key")
	}

	key3 := NewCacheKeyBuilder().
		Model("gpt-4").
		Prompt("Goodbye").
		Build()

	if key1 == key3 {
		t.Error("Different prompts should produce different keys")
	}
}

func TestCosineSimilarity(t *testing.T) {
	// Test identical vectors
	a := []float32{1, 2, 3}
	b := []float32{1, 2, 3}
	sim := cosineSimilarity(a, b)
	if sim != 1.0 {
		t.Errorf("Expected similarity 1.0 for identical vectors, got %f", sim)
	}

	// Test orthogonal vectors
	c := []float32{1, 0, 0}
	d := []float32{0, 1, 0}
	sim = cosineSimilarity(c, d)
	if sim != 0.0 {
		t.Errorf("Expected similarity 0.0 for orthogonal vectors, got %f", sim)
	}

	// Test opposite vectors
	e := []float32{1, 0}
	f := []float32{-1, 0}
	sim = cosineSimilarity(e, f)
	if sim != -1.0 {
		t.Errorf("Expected similarity -1.0 for opposite vectors, got %f", sim)
	}
}
