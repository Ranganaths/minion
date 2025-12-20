package vectorstore

import (
	"context"
	"math"
	"testing"

	"github.com/Ranganaths/minion/embeddings"
)

// MockEmbedder is a mock embedder for testing
type MockEmbedder struct {
	dimension int
}

func NewMockEmbedder(dimension int) *MockEmbedder {
	return &MockEmbedder{dimension: dimension}
}

func (m *MockEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	// Generate a simple deterministic embedding based on text
	embedding := make([]float32, m.dimension)
	for i := 0; i < m.dimension && i < len(text); i++ {
		embedding[i] = float32(text[i]) / 255.0
	}
	return embeddings.NormalizeEmbedding(embedding), nil
}

func (m *MockEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.EmbedQuery(ctx, text)
		if err != nil {
			return nil, err
		}
		result[i] = emb
	}
	return result, nil
}

func (m *MockEmbedder) Dimension() int {
	return m.dimension
}

// TestDocument tests document creation
func TestDocument(t *testing.T) {
	t.Run("NewDocument", func(t *testing.T) {
		doc := NewDocument("Hello World")
		if doc.PageContent != "Hello World" {
			t.Errorf("expected 'Hello World', got '%s'", doc.PageContent)
		}
		if doc.Metadata == nil {
			t.Error("expected metadata to be initialized")
		}
	})

	t.Run("NewDocumentWithMetadata", func(t *testing.T) {
		metadata := map[string]any{"source": "test.txt", "page": 1}
		doc := NewDocumentWithMetadata("Content", metadata)
		if doc.PageContent != "Content" {
			t.Errorf("expected 'Content', got '%s'", doc.PageContent)
		}
		if doc.Metadata["source"] != "test.txt" {
			t.Errorf("expected source 'test.txt', got '%v'", doc.Metadata["source"])
		}
	})

	t.Run("WithID", func(t *testing.T) {
		doc := NewDocument("Content").WithID("doc-123")
		if doc.ID != "doc-123" {
			t.Errorf("expected ID 'doc-123', got '%s'", doc.ID)
		}
	})

	t.Run("WithMetadata", func(t *testing.T) {
		doc := NewDocument("Content").
			WithMetadata("key1", "value1").
			WithMetadata("key2", 123)

		if doc.Metadata["key1"] != "value1" {
			t.Errorf("expected 'value1', got '%v'", doc.Metadata["key1"])
		}
		if doc.Metadata["key2"] != 123 {
			t.Errorf("expected 123, got '%v'", doc.Metadata["key2"])
		}
	})

	t.Run("GetMetadata", func(t *testing.T) {
		doc := NewDocument("Content").WithMetadata("key", "value")

		val, ok := doc.GetMetadata("key")
		if !ok {
			t.Error("expected to find 'key'")
		}
		if val != "value" {
			t.Errorf("expected 'value', got '%v'", val)
		}

		_, ok = doc.GetMetadata("missing")
		if ok {
			t.Error("expected not to find 'missing'")
		}
	})

	t.Run("GetMetadataString", func(t *testing.T) {
		doc := NewDocument("Content").
			WithMetadata("string", "hello").
			WithMetadata("number", 123)

		if doc.GetMetadataString("string") != "hello" {
			t.Errorf("expected 'hello', got '%s'", doc.GetMetadataString("string"))
		}
		if doc.GetMetadataString("number") != "" {
			t.Error("expected empty string for non-string value")
		}
		if doc.GetMetadataString("missing") != "" {
			t.Error("expected empty string for missing key")
		}
	})
}

// TestMemoryVectorStore tests the in-memory vector store
func TestMemoryVectorStore(t *testing.T) {
	t.Run("AddAndSearch", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
			Embedder: embedder,
		})
		if err != nil {
			t.Fatalf("failed to create vector store: %v", err)
		}

		docs := []Document{
			NewDocument("The cat sat on the mat"),
			NewDocument("The dog ran in the park"),
			NewDocument("A cat is a small animal"),
		}

		ctx := context.Background()
		ids, err := vs.AddDocuments(ctx, docs)
		if err != nil {
			t.Fatalf("failed to add documents: %v", err)
		}
		if len(ids) != 3 {
			t.Errorf("expected 3 IDs, got %d", len(ids))
		}

		if vs.Count() != 3 {
			t.Errorf("expected count 3, got %d", vs.Count())
		}

		results, err := vs.SimilaritySearch(ctx, "cat", 2)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("SimilaritySearchWithScore", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
			Embedder: embedder,
		})
		if err != nil {
			t.Fatalf("failed to create vector store: %v", err)
		}

		ctx := context.Background()
		docs := []Document{
			NewDocument("Hello world"),
			NewDocument("Goodbye world"),
		}
		_, _ = vs.AddDocuments(ctx, docs)

		results, err := vs.SimilaritySearchWithScore(ctx, "Hello", 2)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}

		// First result should have higher score
		if results[0].Score < results[1].Score {
			t.Error("expected first result to have higher score")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
			Embedder: embedder,
		})
		if err != nil {
			t.Fatalf("failed to create vector store: %v", err)
		}

		ctx := context.Background()
		docs := []Document{
			NewDocument("Doc 1").WithID("id1"),
			NewDocument("Doc 2").WithID("id2"),
		}
		_, _ = vs.AddDocuments(ctx, docs)

		if vs.Count() != 2 {
			t.Errorf("expected count 2, got %d", vs.Count())
		}

		err = vs.Delete(ctx, []string{"id1"})
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}

		if vs.Count() != 1 {
			t.Errorf("expected count 1 after delete, got %d", vs.Count())
		}

		_, ok := vs.GetDocument("id1")
		if ok {
			t.Error("expected document id1 to be deleted")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
			Embedder: embedder,
		})
		if err != nil {
			t.Fatalf("failed to create vector store: %v", err)
		}

		ctx := context.Background()
		docs := []Document{
			NewDocument("Doc 1"),
			NewDocument("Doc 2"),
		}
		_, _ = vs.AddDocuments(ctx, docs)

		vs.Clear()
		if vs.Count() != 0 {
			t.Errorf("expected count 0 after clear, got %d", vs.Count())
		}
	})

	t.Run("GetDocument", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
			Embedder: embedder,
		})
		if err != nil {
			t.Fatalf("failed to create vector store: %v", err)
		}

		ctx := context.Background()
		docs := []Document{
			NewDocument("Test content").WithID("test-id").WithMetadata("key", "value"),
		}
		_, _ = vs.AddDocuments(ctx, docs)

		doc, ok := vs.GetDocument("test-id")
		if !ok {
			t.Fatal("expected to find document")
		}
		if doc.PageContent != "Test content" {
			t.Errorf("expected 'Test content', got '%s'", doc.PageContent)
		}
		if doc.Metadata["key"] != "value" {
			t.Errorf("expected metadata 'value', got '%v'", doc.Metadata["key"])
		}
	})

	t.Run("GetAllDocuments", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
			Embedder: embedder,
		})
		if err != nil {
			t.Fatalf("failed to create vector store: %v", err)
		}

		ctx := context.Background()
		docs := []Document{
			NewDocument("Doc 1"),
			NewDocument("Doc 2"),
			NewDocument("Doc 3"),
		}
		_, _ = vs.AddDocuments(ctx, docs)

		allDocs := vs.GetAllDocuments()
		if len(allDocs) != 3 {
			t.Errorf("expected 3 documents, got %d", len(allDocs))
		}
	})

	t.Run("AutoGenerateID", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
			Embedder: embedder,
		})
		if err != nil {
			t.Fatalf("failed to create vector store: %v", err)
		}

		ctx := context.Background()
		docs := []Document{
			NewDocument("Doc without ID"),
		}
		ids, _ := vs.AddDocuments(ctx, docs)

		if ids[0] == "" {
			t.Error("expected auto-generated ID")
		}
	})

	t.Run("PreComputedEmbedding", func(t *testing.T) {
		embedder := NewMockEmbedder(3)
		vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
			Embedder: embedder,
		})
		if err != nil {
			t.Fatalf("failed to create vector store: %v", err)
		}

		ctx := context.Background()
		docs := []Document{
			{
				PageContent: "Pre-embedded doc",
				Embedding:   []float32{1, 0, 0},
			},
		}
		_, err = vs.AddDocuments(ctx, docs)
		if err != nil {
			t.Fatalf("failed to add document: %v", err)
		}

		doc, ok := vs.GetDocument("doc_1")
		if !ok {
			t.Fatal("expected to find document")
		}
		if len(doc.Embedding) != 3 || doc.Embedding[0] != 1 {
			t.Error("expected pre-computed embedding to be preserved")
		}
	})
}

// TestMMRSearch tests Max Marginal Relevance search
func TestMMRSearch(t *testing.T) {
	t.Run("BasicMMR", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
			Embedder: embedder,
		})
		if err != nil {
			t.Fatalf("failed to create vector store: %v", err)
		}

		ctx := context.Background()
		docs := []Document{
			NewDocument("Machine learning is great"),
			NewDocument("Deep learning is a subset of ML"),
			NewDocument("Natural language processing"),
			NewDocument("Computer vision applications"),
			NewDocument("Machine learning algorithms"),
		}
		_, _ = vs.AddDocuments(ctx, docs)

		results, err := vs.MaxMarginalRelevanceSearch(ctx, "machine learning", 3, 10, 0.5)
		if err != nil {
			t.Fatalf("MMR search failed: %v", err)
		}

		if len(results) != 3 {
			t.Errorf("expected 3 results, got %d", len(results))
		}
	})

	t.Run("MMRWithLambda", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
			Embedder: embedder,
		})
		if err != nil {
			t.Fatalf("failed to create vector store: %v", err)
		}

		ctx := context.Background()
		docs := []Document{
			NewDocument("Topic A content 1"),
			NewDocument("Topic A content 2"),
			NewDocument("Topic B content 1"),
		}
		_, _ = vs.AddDocuments(ctx, docs)

		// High lambda = more relevance focused
		results1, _ := vs.MaxMarginalRelevanceSearch(ctx, "Topic A", 2, 10, 0.9)

		// Low lambda = more diversity focused
		results2, _ := vs.MaxMarginalRelevanceSearch(ctx, "Topic A", 2, 10, 0.1)

		// Both should return 2 results
		if len(results1) != 2 || len(results2) != 2 {
			t.Error("expected 2 results for each search")
		}
	})
}

// TestSearchWithFilter tests filtered search
func TestSearchWithFilter(t *testing.T) {
	t.Run("EqualsFilter", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
			Embedder: embedder,
		})
		if err != nil {
			t.Fatalf("failed to create vector store: %v", err)
		}

		ctx := context.Background()
		docs := []Document{
			NewDocument("Doc 1").WithMetadata("category", "A"),
			NewDocument("Doc 2").WithMetadata("category", "B"),
			NewDocument("Doc 3").WithMetadata("category", "A"),
		}
		_, _ = vs.AddDocuments(ctx, docs)

		filters := []Filter{
			{Field: "category", Operator: FilterEquals, Value: "A"},
		}
		results, err := vs.SearchWithFilter(ctx, "Doc", 10, filters)
		if err != nil {
			t.Fatalf("filtered search failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("NotEqualsFilter", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
			Embedder: embedder,
		})
		if err != nil {
			t.Fatalf("failed to create vector store: %v", err)
		}

		ctx := context.Background()
		docs := []Document{
			NewDocument("Doc 1").WithMetadata("status", "active"),
			NewDocument("Doc 2").WithMetadata("status", "inactive"),
			NewDocument("Doc 3").WithMetadata("status", "active"),
		}
		_, _ = vs.AddDocuments(ctx, docs)

		filters := []Filter{
			{Field: "status", Operator: FilterNotEquals, Value: "inactive"},
		}
		results, err := vs.SearchWithFilter(ctx, "Doc", 10, filters)
		if err != nil {
			t.Fatalf("filtered search failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("ContainsFilter", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
			Embedder: embedder,
		})
		if err != nil {
			t.Fatalf("failed to create vector store: %v", err)
		}

		ctx := context.Background()
		docs := []Document{
			NewDocument("Doc 1").WithMetadata("tags", "machine-learning"),
			NewDocument("Doc 2").WithMetadata("tags", "deep-learning"),
			NewDocument("Doc 3").WithMetadata("tags", "machine-vision"),
		}
		_, _ = vs.AddDocuments(ctx, docs)

		filters := []Filter{
			{Field: "tags", Operator: FilterContains, Value: "machine"},
		}
		results, err := vs.SearchWithFilter(ctx, "Doc", 10, filters)
		if err != nil {
			t.Fatalf("filtered search failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})
}

// TestDistanceMetrics tests different distance metrics
func TestDistanceMetrics(t *testing.T) {
	embedder := NewMockEmbedder(128)
	ctx := context.Background()

	metrics := []DistanceMetric{DistanceCosine, DistanceEuclidean, DistanceDotProduct}

	for _, metric := range metrics {
		t.Run(string(metric), func(t *testing.T) {
			vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
				Embedder:       embedder,
				DistanceMetric: metric,
			})
			if err != nil {
				t.Fatalf("failed to create vector store: %v", err)
			}

			docs := []Document{
				NewDocument("Hello world"),
				NewDocument("Goodbye world"),
			}
			_, _ = vs.AddDocuments(ctx, docs)

			results, err := vs.SimilaritySearchWithScore(ctx, "Hello", 2)
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}

			if len(results) != 2 {
				t.Errorf("expected 2 results, got %d", len(results))
			}

			// Score should be a valid number
			if math.IsNaN(float64(results[0].Score)) {
				t.Error("score should not be NaN")
			}
		})
	}
}

// TestEmptyVectorStore tests operations on empty store
func TestEmptyVectorStore(t *testing.T) {
	embedder := NewMockEmbedder(128)
	vs, err := NewMemoryVectorStore(MemoryVectorStoreConfig{
		Embedder: embedder,
	})
	if err != nil {
		t.Fatalf("failed to create vector store: %v", err)
	}

	ctx := context.Background()

	t.Run("SearchEmpty", func(t *testing.T) {
		results, err := vs.SimilaritySearch(ctx, "query", 5)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("DeleteEmpty", func(t *testing.T) {
		err := vs.Delete(ctx, []string{"nonexistent"})
		if err != nil {
			t.Errorf("delete should not fail on nonexistent: %v", err)
		}
	})

	t.Run("AddEmptyDocs", func(t *testing.T) {
		ids, err := vs.AddDocuments(ctx, nil)
		if err != nil {
			t.Errorf("add empty docs should not fail: %v", err)
		}
		if len(ids) != 0 {
			t.Errorf("expected 0 IDs, got %d", len(ids))
		}
	})
}

// TestDefaultSearchOptions tests default search options
func TestDefaultSearchOptions(t *testing.T) {
	opts := DefaultSearchOptions()
	if opts.K != 4 {
		t.Errorf("expected K=4, got %d", opts.K)
	}
	if opts.ScoreThreshold != 0.0 {
		t.Errorf("expected ScoreThreshold=0.0, got %f", opts.ScoreThreshold)
	}
	if !opts.IncludeMetadata {
		t.Error("expected IncludeMetadata=true")
	}
}
