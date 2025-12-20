package embeddings

import (
	"context"
	"math"
	"testing"
	"time"
)

// MockEmbedder is a mock embedder for testing
type MockEmbedder struct {
	dimension   int
	embedFunc   func(text string) []float32
	callCount   int
}

func NewMockEmbedder(dimension int) *MockEmbedder {
	return &MockEmbedder{
		dimension: dimension,
		embedFunc: func(text string) []float32 {
			// Generate a simple deterministic embedding based on text
			embedding := make([]float32, dimension)
			for i := 0; i < dimension && i < len(text); i++ {
				embedding[i] = float32(text[i]) / 255.0
			}
			return embedding
		},
	}
}

func (m *MockEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	m.callCount++
	return m.embedFunc(text), nil
}

func (m *MockEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embedding, err := m.EmbedQuery(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}

func (m *MockEmbedder) Dimension() int {
	return m.dimension
}

// TestCosineSimilarity tests the cosine similarity function
func TestCosineSimilarity(t *testing.T) {
	t.Run("IdenticalVectors", func(t *testing.T) {
		a := []float32{1, 0, 0}
		b := []float32{1, 0, 0}
		sim := CosineSimilarity(a, b)
		if math.Abs(float64(sim-1.0)) > 0.001 {
			t.Errorf("expected similarity ~1.0, got %f", sim)
		}
	})

	t.Run("OrthogonalVectors", func(t *testing.T) {
		a := []float32{1, 0, 0}
		b := []float32{0, 1, 0}
		sim := CosineSimilarity(a, b)
		if math.Abs(float64(sim)) > 0.001 {
			t.Errorf("expected similarity ~0.0, got %f", sim)
		}
	})

	t.Run("OppositeVectors", func(t *testing.T) {
		a := []float32{1, 0, 0}
		b := []float32{-1, 0, 0}
		sim := CosineSimilarity(a, b)
		if math.Abs(float64(sim+1.0)) > 0.001 {
			t.Errorf("expected similarity ~-1.0, got %f", sim)
		}
	})

	t.Run("DifferentLengths", func(t *testing.T) {
		a := []float32{1, 0}
		b := []float32{1, 0, 0}
		sim := CosineSimilarity(a, b)
		if sim != 0 {
			t.Errorf("expected 0 for different lengths, got %f", sim)
		}
	})

	t.Run("ZeroVector", func(t *testing.T) {
		a := []float32{0, 0, 0}
		b := []float32{1, 0, 0}
		sim := CosineSimilarity(a, b)
		if sim != 0 {
			t.Errorf("expected 0 for zero vector, got %f", sim)
		}
	})
}

// TestEuclideanDistance tests the Euclidean distance function
func TestEuclideanDistance(t *testing.T) {
	t.Run("IdenticalVectors", func(t *testing.T) {
		a := []float32{1, 0, 0}
		b := []float32{1, 0, 0}
		dist := EuclideanDistance(a, b)
		if math.Abs(float64(dist)) > 0.001 {
			t.Errorf("expected distance ~0.0, got %f", dist)
		}
	})

	t.Run("SimpleDistance", func(t *testing.T) {
		a := []float32{0, 0, 0}
		b := []float32{3, 4, 0}
		dist := EuclideanDistance(a, b)
		if math.Abs(float64(dist-5.0)) > 0.001 {
			t.Errorf("expected distance ~5.0, got %f", dist)
		}
	})
}

// TestDotProduct tests the dot product function
func TestDotProduct(t *testing.T) {
	t.Run("SimpleProduct", func(t *testing.T) {
		a := []float32{1, 2, 3}
		b := []float32{4, 5, 6}
		dot := DotProduct(a, b)
		expected := float32(1*4 + 2*5 + 3*6)
		if math.Abs(float64(dot-expected)) > 0.001 {
			t.Errorf("expected %f, got %f", expected, dot)
		}
	})
}

// TestNormalizeEmbedding tests the normalize function
func TestNormalizeEmbedding(t *testing.T) {
	t.Run("NormalizeVector", func(t *testing.T) {
		embedding := []float32{3, 4}
		normalized := NormalizeEmbedding(embedding)

		// Check unit length
		var norm float32
		for _, v := range normalized {
			norm += v * v
		}
		norm = sqrt(norm)

		if math.Abs(float64(norm-1.0)) > 0.001 {
			t.Errorf("expected unit length, got %f", norm)
		}
	})

	t.Run("ZeroVector", func(t *testing.T) {
		embedding := []float32{0, 0, 0}
		normalized := NormalizeEmbedding(embedding)
		if len(normalized) != 3 {
			t.Errorf("expected length 3, got %d", len(normalized))
		}
	})
}

// TestAverageEmbeddings tests the average embeddings function
func TestAverageEmbeddings(t *testing.T) {
	t.Run("SimpleAverage", func(t *testing.T) {
		embeddings := [][]float32{
			{2, 4, 6},
			{4, 6, 8},
		}
		avg := AverageEmbeddings(embeddings)
		expected := []float32{3, 5, 7}
		for i, v := range avg {
			if math.Abs(float64(v-expected[i])) > 0.001 {
				t.Errorf("expected %f at index %d, got %f", expected[i], i, v)
			}
		}
	})

	t.Run("EmptyInput", func(t *testing.T) {
		avg := AverageEmbeddings(nil)
		if avg != nil {
			t.Errorf("expected nil for empty input")
		}
	})
}

// TestFindMostSimilar tests finding the most similar embedding
func TestFindMostSimilar(t *testing.T) {
	query := []float32{1, 0, 0}
	candidates := [][]float32{
		{0, 1, 0},  // Orthogonal
		{1, 0, 0},  // Identical
		{0.5, 0.5, 0}, // Partial match
	}

	idx, score := FindMostSimilar(query, candidates)
	if idx != 1 {
		t.Errorf("expected index 1, got %d", idx)
	}
	if math.Abs(float64(score-1.0)) > 0.001 {
		t.Errorf("expected score ~1.0, got %f", score)
	}
}

// TestTopKSimilar tests finding top K similar embeddings
func TestTopKSimilar(t *testing.T) {
	query := []float32{1, 0, 0}
	candidates := [][]float32{
		{0, 1, 0},     // Orthogonal (score ~0)
		{1, 0, 0},     // Identical (score = 1)
		{0.5, 0.5, 0}, // Partial (score ~0.7)
		{0.9, 0.1, 0}, // High match (score ~0.99)
	}

	results := TopKSimilar(query, candidates, 2)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if results[0].Index != 1 {
		t.Errorf("expected index 1 as top match, got %d", results[0].Index)
	}
}

// TestMemoryCache tests the in-memory cache
func TestMemoryCache(t *testing.T) {
	t.Run("GetSet", func(t *testing.T) {
		cache := NewMemoryCache(MemoryCacheConfig{MaxSize: 100})
		ctx := context.Background()

		embedding := []float32{1, 2, 3}
		err := cache.Set(ctx, "key1", embedding, time.Hour)
		if err != nil {
			t.Fatalf("failed to set: %v", err)
		}

		result, ok := cache.Get(ctx, "key1")
		if !ok {
			t.Fatal("expected to find key1")
		}
		if len(result) != 3 || result[0] != 1 {
			t.Errorf("unexpected embedding: %v", result)
		}
	})

	t.Run("Expiration", func(t *testing.T) {
		cache := NewMemoryCache(MemoryCacheConfig{MaxSize: 100})
		ctx := context.Background()

		embedding := []float32{1, 2, 3}
		err := cache.Set(ctx, "key1", embedding, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("failed to set: %v", err)
		}

		time.Sleep(20 * time.Millisecond)

		_, ok := cache.Get(ctx, "key1")
		if ok {
			t.Error("expected key to be expired")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		cache := NewMemoryCache(MemoryCacheConfig{MaxSize: 100})
		ctx := context.Background()

		embedding := []float32{1, 2, 3}
		_ = cache.Set(ctx, "key1", embedding, time.Hour)
		_ = cache.Delete(ctx, "key1")

		_, ok := cache.Get(ctx, "key1")
		if ok {
			t.Error("expected key to be deleted")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		cache := NewMemoryCache(MemoryCacheConfig{MaxSize: 100})
		ctx := context.Background()

		_ = cache.Set(ctx, "key1", []float32{1}, time.Hour)
		_ = cache.Set(ctx, "key2", []float32{2}, time.Hour)
		_ = cache.Clear(ctx)

		if cache.Size() != 0 {
			t.Errorf("expected empty cache, got size %d", cache.Size())
		}
	})

	t.Run("MaxSize", func(t *testing.T) {
		cache := NewMemoryCache(MemoryCacheConfig{MaxSize: 2})
		ctx := context.Background()

		_ = cache.Set(ctx, "key1", []float32{1}, time.Hour)
		_ = cache.Set(ctx, "key2", []float32{2}, time.Hour)
		_ = cache.Set(ctx, "key3", []float32{3}, time.Hour)

		if cache.Size() > 2 {
			t.Errorf("expected max size 2, got %d", cache.Size())
		}
	})
}

// TestCachedEmbedder tests the cached embedder wrapper
func TestCachedEmbedder(t *testing.T) {
	t.Run("CacheHit", func(t *testing.T) {
		mockEmbedder := NewMockEmbedder(128)
		cache := NewMemoryCache(MemoryCacheConfig{MaxSize: 100})
		cachedEmbedder := NewCachedEmbedder(CachedEmbedderConfig{
			Embedder:  mockEmbedder,
			Cache:     cache,
			KeyPrefix: "test_",
			TTL:       time.Hour,
		})

		ctx := context.Background()

		// First call should miss cache
		_, err := cachedEmbedder.EmbedQuery(ctx, "hello")
		if err != nil {
			t.Fatalf("first embed failed: %v", err)
		}

		// Second call should hit cache
		_, err = cachedEmbedder.EmbedQuery(ctx, "hello")
		if err != nil {
			t.Fatalf("second embed failed: %v", err)
		}

		hits, misses := cachedEmbedder.Stats()
		if hits != 1 || misses != 1 {
			t.Errorf("expected 1 hit and 1 miss, got %d hits and %d misses", hits, misses)
		}

		if mockEmbedder.callCount != 1 {
			t.Errorf("expected 1 embedder call, got %d", mockEmbedder.callCount)
		}
	})

	t.Run("DocumentCaching", func(t *testing.T) {
		mockEmbedder := NewMockEmbedder(128)
		cache := NewMemoryCache(MemoryCacheConfig{MaxSize: 100})
		cachedEmbedder := NewCachedEmbedder(CachedEmbedderConfig{
			Embedder: mockEmbedder,
			Cache:    cache,
			TTL:      time.Hour,
		})

		ctx := context.Background()

		texts := []string{"hello", "world", "test"}
		_, err := cachedEmbedder.EmbedDocuments(ctx, texts)
		if err != nil {
			t.Fatalf("first embed failed: %v", err)
		}

		// Second call with some overlap
		texts2 := []string{"hello", "new", "world"}
		_, err = cachedEmbedder.EmbedDocuments(ctx, texts2)
		if err != nil {
			t.Fatalf("second embed failed: %v", err)
		}

		// "hello" and "world" should be cached, only "new" should be a new call
		if mockEmbedder.callCount != 4 { // 3 initial + 1 new
			t.Errorf("expected 4 embedder calls, got %d", mockEmbedder.callCount)
		}
	})

	t.Run("Dimension", func(t *testing.T) {
		mockEmbedder := NewMockEmbedder(256)
		cache := NewMemoryCache(MemoryCacheConfig{MaxSize: 100})
		cachedEmbedder := NewCachedEmbedder(CachedEmbedderConfig{
			Embedder: mockEmbedder,
			Cache:    cache,
		})

		if cachedEmbedder.Dimension() != 256 {
			t.Errorf("expected dimension 256, got %d", cachedEmbedder.Dimension())
		}
	})
}

// TestDefaultConfig tests default configuration
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultEmbedderConfig()
	if cfg.BatchSize != 100 {
		t.Errorf("expected batch size 100, got %d", cfg.BatchSize)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", cfg.MaxRetries)
	}
	if cfg.Timeout != 60 {
		t.Errorf("expected timeout 60, got %d", cfg.Timeout)
	}
}
