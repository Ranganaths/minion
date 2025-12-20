package rag

import (
	"context"
	"testing"

	"github.com/Ranganaths/minion/embeddings"
	"github.com/Ranganaths/minion/llm"
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

// MockLLMProvider is a mock LLM provider for testing
type MockLLMProvider struct {
	response string
}

func NewMockLLMProvider(response string) *MockLLMProvider {
	return &MockLLMProvider{response: response}
}

func (m *MockLLMProvider) GenerateCompletion(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return &llm.CompletionResponse{
		Text:       m.response,
		TokensUsed: 10,
	}, nil
}

func (m *MockLLMProvider) GenerateChat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{
		Message: llm.Message{
			Role:    "assistant",
			Content: m.response,
		},
		TokensUsed: 10,
	}, nil
}

func (m *MockLLMProvider) Name() string {
	return "mock"
}

// TestPipeline tests the RAG pipeline
func TestPipeline(t *testing.T) {
	t.Run("CreatePipeline", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		llmProvider := NewMockLLMProvider("This is the answer.")

		pipeline, err := NewPipeline(PipelineConfig{
			Embedder: embedder,
			LLM:      llmProvider,
		})
		if err != nil {
			t.Fatalf("failed to create pipeline: %v", err)
		}

		if pipeline.VectorStore() == nil {
			t.Error("expected vector store to be created")
		}

		if pipeline.Retriever() == nil {
			t.Error("expected retriever to be created")
		}
	})

	t.Run("AddAndQuery", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		llmProvider := NewMockLLMProvider("The capital of France is Paris.")

		pipeline, err := NewPipeline(PipelineConfig{
			Embedder:      embedder,
			LLM:           llmProvider,
			ReturnSources: true,
		})
		if err != nil {
			t.Fatalf("failed to create pipeline: %v", err)
		}

		ctx := context.Background()

		// Add documents
		docs := []vectorstore.Document{
			vectorstore.NewDocumentWithMetadata("Paris is the capital of France.", map[string]any{"source": "doc1.txt"}),
			vectorstore.NewDocumentWithMetadata("London is the capital of England.", map[string]any{"source": "doc2.txt"}),
			vectorstore.NewDocumentWithMetadata("Berlin is the capital of Germany.", map[string]any{"source": "doc3.txt"}),
		}

		err = pipeline.AddDocuments(ctx, docs)
		if err != nil {
			t.Fatalf("failed to add documents: %v", err)
		}

		// Query
		answer, err := pipeline.Query(ctx, "What is the capital of France?")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}

		if answer == "" {
			t.Error("expected non-empty answer")
		}
	})

	t.Run("AddTexts", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		llmProvider := NewMockLLMProvider("Answer")

		pipeline, err := NewPipeline(PipelineConfig{
			Embedder: embedder,
			LLM:      llmProvider,
		})
		if err != nil {
			t.Fatalf("failed to create pipeline: %v", err)
		}

		ctx := context.Background()

		texts := []string{"First document", "Second document"}
		metadatas := []map[string]any{
			{"source": "text1"},
			{"source": "text2"},
		}

		err = pipeline.AddTexts(ctx, texts, metadatas)
		if err != nil {
			t.Fatalf("failed to add texts: %v", err)
		}
	})

	t.Run("Search", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		llmProvider := NewMockLLMProvider("Answer")

		pipeline, err := NewPipeline(PipelineConfig{
			Embedder: embedder,
			LLM:      llmProvider,
		})
		if err != nil {
			t.Fatalf("failed to create pipeline: %v", err)
		}

		ctx := context.Background()

		err = pipeline.AddTexts(ctx, []string{"Hello world", "Goodbye world"}, nil)
		if err != nil {
			t.Fatalf("failed to add texts: %v", err)
		}

		results, err := pipeline.Search(ctx, "Hello", 2)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}

		if len(results) == 0 {
			t.Error("expected search results")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		llmProvider := NewMockLLMProvider("Answer")

		pipeline, err := NewPipeline(PipelineConfig{
			Embedder: embedder,
			LLM:      llmProvider,
		})
		if err != nil {
			t.Fatalf("failed to create pipeline: %v", err)
		}

		ctx := context.Background()

		err = pipeline.AddTexts(ctx, []string{"Test"}, nil)
		if err != nil {
			t.Fatalf("failed to add texts: %v", err)
		}

		pipeline.Clear()

		results, _ := pipeline.Search(ctx, "Test", 10)
		if len(results) != 0 {
			t.Error("expected no results after clear")
		}
	})

	t.Run("MissingEmbedder", func(t *testing.T) {
		llmProvider := NewMockLLMProvider("Answer")

		_, err := NewPipeline(PipelineConfig{
			LLM: llmProvider,
		})
		if err == nil {
			t.Error("expected error for missing embedder")
		}
	})

	t.Run("MissingLLM", func(t *testing.T) {
		embedder := NewMockEmbedder(128)

		_, err := NewPipeline(PipelineConfig{
			Embedder: embedder,
		})
		if err == nil {
			t.Error("expected error for missing LLM")
		}
	})
}

// TestPipelineBuilder tests the builder pattern
func TestPipelineBuilder(t *testing.T) {
	t.Run("BuildPipeline", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		llmProvider := NewMockLLMProvider("Answer")

		pipeline, err := NewPipelineBuilder().
			WithEmbedder(embedder).
			WithLLM(llmProvider).
			WithRetrieverK(5).
			WithReturnSources(true).
			Build()

		if err != nil {
			t.Fatalf("failed to build pipeline: %v", err)
		}

		if pipeline == nil {
			t.Error("expected non-nil pipeline")
		}
	})

	t.Run("WithCustomVectorStore", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		llmProvider := NewMockLLMProvider("Answer")

		vs, _ := vectorstore.NewMemoryVectorStore(vectorstore.MemoryVectorStoreConfig{
			Embedder: embedder,
		})

		pipeline, err := NewPipelineBuilder().
			WithEmbedder(embedder).
			WithLLM(llmProvider).
			WithVectorStore(vs).
			Build()

		if err != nil {
			t.Fatalf("failed to build pipeline: %v", err)
		}

		if pipeline.VectorStore() != vs {
			t.Error("expected provided vector store to be used")
		}
	})
}

// TestQueryWithSources tests query with source documents
func TestQueryWithSources(t *testing.T) {
	embedder := NewMockEmbedder(128)
	llmProvider := NewMockLLMProvider("The answer is 42.")

	pipeline, err := NewPipeline(PipelineConfig{
		Embedder:      embedder,
		LLM:           llmProvider,
		ReturnSources: true,
	})
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	ctx := context.Background()

	docs := []vectorstore.Document{
		vectorstore.NewDocumentWithMetadata("The answer to life is 42.", map[string]any{"source": "guide.txt"}),
	}

	err = pipeline.AddDocuments(ctx, docs)
	if err != nil {
		t.Fatalf("failed to add documents: %v", err)
	}

	answer, sources, err := pipeline.QueryWithSources(ctx, "What is the answer?")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if answer == "" {
		t.Error("expected non-empty answer")
	}

	// Sources may or may not be returned depending on retrieval
	_ = sources
}
