package integration

import (
	"context"
	"testing"

	"github.com/Ranganaths/minion/core/multiagent"
	"github.com/Ranganaths/minion/embeddings"
	"github.com/Ranganaths/minion/llm"
	"github.com/Ranganaths/minion/vectorstore"
)

// MockTask mirrors multiagent.Task for testing without the multiagent dependency
type MockTask struct {
	ID          string
	Name        string
	Description string
	Input       map[string]interface{}
}

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

// TestRAGWorker tests the RAG worker
func TestRAGWorker(t *testing.T) {
	t.Run("CreateRAGWorker", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		llmProvider := NewMockLLMProvider("This is the answer.")

		worker, err := NewRAGWorker(RAGWorkerConfig{
			Embedder: embedder,
			LLM:      llmProvider,
			Name:     "test_rag_worker",
		})
		if err != nil {
			t.Fatalf("failed to create RAG worker: %v", err)
		}

		if worker.GetName() != "test_rag_worker" {
			t.Errorf("expected name 'test_rag_worker', got '%s'", worker.GetName())
		}

		caps := worker.GetCapabilities()
		if len(caps) == 0 {
			t.Error("expected capabilities")
		}
	})

	t.Run("HandleTaskWithQuery", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		llmProvider := NewMockLLMProvider("The capital of France is Paris.")

		worker, err := NewRAGWorker(RAGWorkerConfig{
			Embedder: embedder,
			LLM:      llmProvider,
		})
		if err != nil {
			t.Fatalf("failed to create RAG worker: %v", err)
		}

		ctx := context.Background()

		// Add some documents
		err = worker.AddTexts(ctx, []string{
			"Paris is the capital of France.",
			"London is the capital of England.",
		}, nil)
		if err != nil {
			t.Fatalf("failed to add texts: %v", err)
		}

		// Create task
		task := &multiagent.Task{
			ID:   "task-1",
			Name: "qa",
			Input: map[string]interface{}{
				"query": "What is the capital of France?",
			},
		}

		result, err := worker.HandleTask(ctx, task)
		if err != nil {
			t.Fatalf("HandleTask failed: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map result, got %T", result)
		}

		if resultMap["answer"] == nil {
			t.Error("expected answer in result")
		}
	})

	t.Run("HandleTaskWithDocuments", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		llmProvider := NewMockLLMProvider("Answer based on documents.")

		worker, err := NewRAGWorker(RAGWorkerConfig{
			Embedder: embedder,
			LLM:      llmProvider,
		})
		if err != nil {
			t.Fatalf("failed to create RAG worker: %v", err)
		}

		ctx := context.Background()

		// Create task with documents
		task := &multiagent.Task{
			ID:   "task-2",
			Name: "qa_with_docs",
			Input: map[string]interface{}{
				"query": "What is mentioned in the documents?",
				"documents": []interface{}{
					"First document content",
					map[string]interface{}{
						"content": "Second document with metadata",
						"source":  "test.txt",
					},
				},
			},
		}

		result, err := worker.HandleTask(ctx, task)
		if err != nil {
			t.Fatalf("HandleTask failed: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map result, got %T", result)
		}

		if resultMap["answer"] == nil {
			t.Error("expected answer in result")
		}
	})

	t.Run("Search", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		llmProvider := NewMockLLMProvider("Answer")

		worker, err := NewRAGWorker(RAGWorkerConfig{
			Embedder: embedder,
			LLM:      llmProvider,
		})
		if err != nil {
			t.Fatalf("failed to create RAG worker: %v", err)
		}

		ctx := context.Background()

		err = worker.AddTexts(ctx, []string{"Hello world", "Goodbye world"}, nil)
		if err != nil {
			t.Fatalf("failed to add texts: %v", err)
		}

		docs, err := worker.Search(ctx, "Hello", 2)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(docs) == 0 {
			t.Error("expected search results")
		}
	})

	t.Run("MissingEmbedder", func(t *testing.T) {
		_, err := NewRAGWorker(RAGWorkerConfig{
			LLM: NewMockLLMProvider("Answer"),
		})
		if err == nil {
			t.Error("expected error for missing embedder")
		}
	})

	t.Run("MissingLLM", func(t *testing.T) {
		_, err := NewRAGWorker(RAGWorkerConfig{
			Embedder: NewMockEmbedder(128),
		})
		if err == nil {
			t.Error("expected error for missing LLM")
		}
	})
}

// TestChainWorker tests the chain worker
func TestChainWorker(t *testing.T) {
	t.Run("CreateWithLLM", func(t *testing.T) {
		llmProvider := NewMockLLMProvider("Response")

		worker, err := NewChainWorker(ChainWorkerConfig{
			LLM:  llmProvider,
			Name: "test_chain_worker",
		})
		if err != nil {
			t.Fatalf("failed to create chain worker: %v", err)
		}

		if worker.GetName() != "test_chain_worker" {
			t.Errorf("expected name 'test_chain_worker', got '%s'", worker.GetName())
		}
	})

	t.Run("HandleTask", func(t *testing.T) {
		llmProvider := NewMockLLMProvider("Generated response")

		worker, err := NewChainWorker(ChainWorkerConfig{
			LLM: llmProvider,
		})
		if err != nil {
			t.Fatalf("failed to create chain worker: %v", err)
		}

		ctx := context.Background()
		task := &multiagent.Task{
			ID:   "task-1",
			Name: "generate",
			Input: map[string]interface{}{
				"input": "Tell me a story",
			},
		}

		result, err := worker.HandleTask(ctx, task)
		if err != nil {
			t.Fatalf("HandleTask failed: %v", err)
		}

		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map result, got %T", result)
		}

		if resultMap["text"] == nil {
			t.Error("expected text in result")
		}
	})

	t.Run("MissingChainAndLLM", func(t *testing.T) {
		_, err := NewChainWorker(ChainWorkerConfig{})
		if err == nil {
			t.Error("expected error for missing Chain and LLM")
		}
	})
}

// TestRetrieverWorker tests the retriever worker
func TestRetrieverWorker(t *testing.T) {
	t.Run("CreateRetrieverWorker", func(t *testing.T) {
		embedder := NewMockEmbedder(128)

		worker, err := NewRetrieverWorker(RetrieverWorkerConfig{
			Embedder: embedder,
			Name:     "test_retriever",
		})
		if err != nil {
			t.Fatalf("failed to create retriever worker: %v", err)
		}

		if worker.GetName() != "test_retriever" {
			t.Errorf("expected name 'test_retriever', got '%s'", worker.GetName())
		}

		caps := worker.GetCapabilities()
		if len(caps) == 0 {
			t.Error("expected capabilities")
		}
	})

	t.Run("HandleSearchTask", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		vs, _ := vectorstore.NewMemoryVectorStore(vectorstore.MemoryVectorStoreConfig{
			Embedder: embedder,
		})

		// Add some documents
		ctx := context.Background()
		vs.AddDocuments(ctx, []vectorstore.Document{
			vectorstore.NewDocument("First document"),
			vectorstore.NewDocument("Second document"),
		})

		worker, err := NewRetrieverWorker(RetrieverWorkerConfig{
			Embedder:    embedder,
			VectorStore: vs,
		})
		if err != nil {
			t.Fatalf("failed to create retriever worker: %v", err)
		}

		task := &multiagent.Task{
			ID: "task-1",
			Input: map[string]interface{}{
				"query": "First",
			},
		}

		result, err := worker.HandleTask(ctx, task)
		if err != nil {
			t.Fatalf("HandleTask failed: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map result, got %T", result)
		}

		if resultMap["documents"] == nil {
			t.Error("expected documents in result")
		}
	})

	t.Run("HandleAddTask", func(t *testing.T) {
		embedder := NewMockEmbedder(128)

		worker, err := NewRetrieverWorker(RetrieverWorkerConfig{
			Embedder: embedder,
		})
		if err != nil {
			t.Fatalf("failed to create retriever worker: %v", err)
		}

		ctx := context.Background()
		task := &multiagent.Task{
			ID: "task-1",
			Input: map[string]interface{}{
				"operation": "add",
				"texts":     []interface{}{"Document 1", "Document 2"},
			},
		}

		result, err := worker.HandleTask(ctx, task)
		if err != nil {
			t.Fatalf("HandleTask failed: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map result, got %T", result)
		}

		if resultMap["count"].(int) != 2 {
			t.Errorf("expected count 2, got %v", resultMap["count"])
		}
	})

	t.Run("HandleDeleteTask", func(t *testing.T) {
		embedder := NewMockEmbedder(128)
		vs, _ := vectorstore.NewMemoryVectorStore(vectorstore.MemoryVectorStoreConfig{
			Embedder: embedder,
		})

		// Add some documents
		ctx := context.Background()
		vs.AddDocuments(ctx, []vectorstore.Document{
			vectorstore.NewDocument("Doc 1").WithID("id1"),
			vectorstore.NewDocument("Doc 2").WithID("id2"),
		})

		worker, err := NewRetrieverWorker(RetrieverWorkerConfig{
			Embedder:    embedder,
			VectorStore: vs,
		})
		if err != nil {
			t.Fatalf("failed to create retriever worker: %v", err)
		}

		task := &multiagent.Task{
			ID: "task-1",
			Input: map[string]interface{}{
				"operation": "delete",
				"ids":       []interface{}{"id1"},
			},
		}

		result, err := worker.HandleTask(ctx, task)
		if err != nil {
			t.Fatalf("HandleTask failed: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map result, got %T", result)
		}

		if resultMap["count"].(int) != 1 {
			t.Errorf("expected count 1, got %v", resultMap["count"])
		}
	})

	t.Run("MissingEmbedder", func(t *testing.T) {
		_, err := NewRetrieverWorker(RetrieverWorkerConfig{})
		if err == nil {
			t.Error("expected error for missing embedder")
		}
	})
}

// TestTaskHandlerInterface verifies all workers implement TaskHandler
func TestTaskHandlerInterface(t *testing.T) {
	embedder := NewMockEmbedder(128)
	llmProvider := NewMockLLMProvider("Response")

	ragWorker, _ := NewRAGWorker(RAGWorkerConfig{
		Embedder: embedder,
		LLM:      llmProvider,
	})

	chainWorker, _ := NewChainWorker(ChainWorkerConfig{
		LLM: llmProvider,
	})

	retrieverWorker, _ := NewRetrieverWorker(RetrieverWorkerConfig{
		Embedder: embedder,
	})

	// Verify they all implement TaskHandler
	var _ multiagent.TaskHandler = ragWorker
	var _ multiagent.TaskHandler = chainWorker
	var _ multiagent.TaskHandler = retrieverWorker
}
