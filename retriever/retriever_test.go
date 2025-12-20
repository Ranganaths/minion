package retriever

import (
	"context"
	"testing"

	"github.com/Ranganaths/minion/embeddings"
	"github.com/Ranganaths/minion/vectorstore"
)

// MockEmbedder is a mock embedder for testing
type MockEmbedder struct {
	dimension int
}

func NewMockEmbedder(dimension int) *MockEmbedder {
	return &MockEmbedder{dimension: dimension}
}

func (m *MockEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
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

// setupTestVectorStore creates a test vector store with sample documents
func setupTestVectorStore(t *testing.T) *vectorstore.MemoryVectorStore {
	embedder := NewMockEmbedder(128)
	vs, err := vectorstore.NewMemoryVectorStore(vectorstore.MemoryVectorStoreConfig{
		Embedder: embedder,
	})
	if err != nil {
		t.Fatalf("failed to create vector store: %v", err)
	}

	docs := []vectorstore.Document{
		vectorstore.NewDocumentWithMetadata("Machine learning is a subset of artificial intelligence.", map[string]any{"category": "AI", "source": "doc1.txt"}),
		vectorstore.NewDocumentWithMetadata("Deep learning uses neural networks with many layers.", map[string]any{"category": "AI", "source": "doc2.txt"}),
		vectorstore.NewDocumentWithMetadata("Natural language processing deals with text understanding.", map[string]any{"category": "NLP", "source": "doc3.txt"}),
		vectorstore.NewDocumentWithMetadata("Computer vision focuses on image recognition.", map[string]any{"category": "CV", "source": "doc4.txt"}),
		vectorstore.NewDocumentWithMetadata("Reinforcement learning trains agents through rewards.", map[string]any{"category": "AI", "source": "doc5.txt"}),
	}

	ctx := context.Background()
	_, err = vs.AddDocuments(ctx, docs)
	if err != nil {
		t.Fatalf("failed to add documents: %v", err)
	}

	return vs
}

// TestVectorStoreRetriever tests the vector store retriever
func TestVectorStoreRetriever(t *testing.T) {
	t.Run("BasicRetrieval", func(t *testing.T) {
		vs := setupTestVectorStore(t)

		retriever, err := NewVectorStoreRetriever(VectorStoreRetrieverConfig{
			VectorStore: vs,
			K:           3,
		})
		if err != nil {
			t.Fatalf("failed to create retriever: %v", err)
		}

		ctx := context.Background()
		docs, err := retriever.GetRelevantDocuments(ctx, "machine learning artificial intelligence")
		if err != nil {
			t.Fatalf("retrieval failed: %v", err)
		}

		if len(docs) != 3 {
			t.Errorf("expected 3 documents, got %d", len(docs))
		}
	})

	t.Run("MMRRetrieval", func(t *testing.T) {
		vs := setupTestVectorStore(t)

		retriever, err := NewVectorStoreRetriever(VectorStoreRetrieverConfig{
			VectorStore: vs,
			K:           3,
			SearchType:  SearchTypeMMR,
			Lambda:      0.5,
			FetchK:      2.0,
		})
		if err != nil {
			t.Fatalf("failed to create retriever: %v", err)
		}

		ctx := context.Background()
		docs, err := retriever.GetRelevantDocuments(ctx, "learning")
		if err != nil {
			t.Fatalf("MMR retrieval failed: %v", err)
		}

		if len(docs) != 3 {
			t.Errorf("expected 3 documents, got %d", len(docs))
		}
	})

	t.Run("WithScoreThreshold", func(t *testing.T) {
		vs := setupTestVectorStore(t)

		retriever, err := NewVectorStoreRetriever(VectorStoreRetrieverConfig{
			VectorStore:    vs,
			K:              10,
			ScoreThreshold: 0.9, // High threshold
		})
		if err != nil {
			t.Fatalf("failed to create retriever: %v", err)
		}

		ctx := context.Background()
		docs, err := retriever.GetRelevantDocuments(ctx, "xyz completely unrelated query")
		if err != nil {
			t.Fatalf("retrieval failed: %v", err)
		}

		// With high threshold, should filter out low-scoring results
		if len(docs) > 5 {
			t.Errorf("expected filtered results, got %d", len(docs))
		}
	})

	t.Run("WithFilters", func(t *testing.T) {
		vs := setupTestVectorStore(t)

		retriever, err := NewVectorStoreRetriever(VectorStoreRetrieverConfig{
			VectorStore: vs,
			K:           10,
			Filters: []vectorstore.Filter{
				{Field: "category", Operator: vectorstore.FilterEquals, Value: "AI"},
			},
		})
		if err != nil {
			t.Fatalf("failed to create retriever: %v", err)
		}

		ctx := context.Background()
		docs, err := retriever.GetRelevantDocuments(ctx, "learning")
		if err != nil {
			t.Fatalf("retrieval failed: %v", err)
		}

		// Should only get AI category documents
		for _, doc := range docs {
			if doc.Metadata["category"] != "AI" {
				t.Errorf("expected category 'AI', got '%v'", doc.Metadata["category"])
			}
		}
	})

	t.Run("SetK", func(t *testing.T) {
		vs := setupTestVectorStore(t)

		retriever, err := NewVectorStoreRetriever(VectorStoreRetrieverConfig{
			VectorStore: vs,
			K:           2,
		})
		if err != nil {
			t.Fatalf("failed to create retriever: %v", err)
		}

		if retriever.K() != 2 {
			t.Errorf("expected K=2, got %d", retriever.K())
		}

		retriever.SetK(5)
		if retriever.K() != 5 {
			t.Errorf("expected K=5, got %d", retriever.K())
		}
	})

	t.Run("SetSearchType", func(t *testing.T) {
		vs := setupTestVectorStore(t)

		retriever, err := NewVectorStoreRetriever(VectorStoreRetrieverConfig{
			VectorStore: vs,
		})
		if err != nil {
			t.Fatalf("failed to create retriever: %v", err)
		}

		if retriever.SearchType() != SearchTypeSimilarity {
			t.Errorf("expected default search type to be similarity")
		}

		retriever.SetSearchType(SearchTypeMMR)
		if retriever.SearchType() != SearchTypeMMR {
			t.Errorf("expected search type to be MMR")
		}
	})

	t.Run("NilVectorStore", func(t *testing.T) {
		_, err := NewVectorStoreRetriever(VectorStoreRetrieverConfig{
			VectorStore: nil,
		})
		if err == nil {
			t.Error("expected error for nil vector store")
		}
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		cfg := DefaultRetrieverConfig()
		if cfg.K != 4 {
			t.Errorf("expected default K=4, got %d", cfg.K)
		}
		if cfg.SearchType != SearchTypeSimilarity {
			t.Errorf("expected default search type to be similarity")
		}
	})
}

// TestEmptyRetrieval tests retrieval from empty store
func TestEmptyRetrieval(t *testing.T) {
	embedder := NewMockEmbedder(128)
	vs, err := vectorstore.NewMemoryVectorStore(vectorstore.MemoryVectorStoreConfig{
		Embedder: embedder,
	})
	if err != nil {
		t.Fatalf("failed to create vector store: %v", err)
	}

	retriever, err := NewVectorStoreRetriever(VectorStoreRetrieverConfig{
		VectorStore: vs,
		K:           3,
	})
	if err != nil {
		t.Fatalf("failed to create retriever: %v", err)
	}

	ctx := context.Background()
	docs, err := retriever.GetRelevantDocuments(ctx, "query")
	if err != nil {
		t.Fatalf("retrieval failed: %v", err)
	}

	if len(docs) != 0 {
		t.Errorf("expected 0 documents from empty store, got %d", len(docs))
	}
}
