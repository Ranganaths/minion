package workers

import (
	"context"
	"testing"

	"github.com/Ranganaths/minion/chain"
	"github.com/Ranganaths/minion/core/multiagent"
	"github.com/Ranganaths/minion/retriever"
	"github.com/Ranganaths/minion/vectorstore"
)

// mockLLMProvider implements multiagent.LLMProvider for testing
type mockLLMProvider struct {
	response *multiagent.CompletionResponse
	err      error
}

func (m *mockLLMProvider) GenerateCompletion(ctx context.Context, req *multiagent.CompletionRequest) (*multiagent.CompletionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.response != nil {
		return m.response, nil
	}
	return &multiagent.CompletionResponse{
		Text:       "Mock response to: " + req.UserPrompt,
		TokensUsed: 10,
	}, nil
}

// mockEmbedder implements embeddings.Embedder for testing
type mockEmbedder struct {
	dimensions int
}

func (m *mockEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = make([]float32, m.dimensions)
		for j := range result[i] {
			result[i][j] = float32(i+j) * 0.1
		}
	}
	return result, nil
}

func (m *mockEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	result := make([]float32, m.dimensions)
	for i := range result {
		result[i] = float32(i) * 0.1
	}
	return result, nil
}

func (m *mockEmbedder) Dimension() int {
	return m.dimensions
}

func (m *mockEmbedder) Name() string {
	return "mock"
}

// mockChain implements chain.Chain for testing
type mockChain struct {
	name       string
	inputKeys  []string
	outputKeys []string
	outputs    map[string]any
	err        error
}

func (m *mockChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.outputs != nil {
		return m.outputs, nil
	}
	return map[string]any{
		"output": "processed",
	}, nil
}

func (m *mockChain) Stream(ctx context.Context, inputs map[string]any) (<-chan chain.StreamEvent, error) {
	ch := make(chan chain.StreamEvent)
	go func() {
		defer close(ch)
		result, err := m.Call(ctx, inputs)
		if err != nil {
			ch <- chain.StreamEvent{Type: chain.StreamEventError, Error: err}
			return
		}
		ch <- chain.StreamEvent{Type: chain.StreamEventComplete, Data: result}
	}()
	return ch, nil
}

func (m *mockChain) InputKeys() []string {
	if m.inputKeys != nil {
		return m.inputKeys
	}
	return []string{"input"}
}

func (m *mockChain) OutputKeys() []string {
	if m.outputKeys != nil {
		return m.outputKeys
	}
	return []string{"output"}
}

func (m *mockChain) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock_chain"
}

func TestRAGWorker(t *testing.T) {
	ctx := context.Background()

	// Create mock components
	embedder := &mockEmbedder{dimensions: 384}
	vs, _ := vectorstore.NewMemoryVectorStore(vectorstore.MemoryVectorStoreConfig{
		Embedder: embedder,
	})

	// Add some test documents
	vs.AddDocuments(ctx, []vectorstore.Document{
		{ID: "doc1", PageContent: "The sky is blue because of Rayleigh scattering."},
		{ID: "doc2", PageContent: "Water is composed of hydrogen and oxygen atoms."},
	})

	llmProvider := &mockLLMProvider{
		response: &multiagent.CompletionResponse{
			Text:       "The sky is blue due to Rayleigh scattering of sunlight.",
			TokensUsed: 15,
		},
	}

	t.Run("successful RAG query", func(t *testing.T) {
		worker, err := NewRAGWorker(RAGWorkerConfig{
			LLMProvider: llmProvider,
			VectorStore: vs,
			TopK:        2,
		})
		if err != nil {
			t.Fatalf("failed to create worker: %v", err)
		}

		if worker.GetName() != "RAGWorker" {
			t.Errorf("expected name 'RAGWorker', got '%s'", worker.GetName())
		}

		capabilities := worker.GetCapabilities()
		if len(capabilities) == 0 {
			t.Error("expected capabilities, got none")
		}

		task := &multiagent.Task{
			ID:    "task1",
			Input: map[string]interface{}{"query": "Why is the sky blue?"},
		}

		result, err := worker.HandleTask(ctx, task)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatal("expected map[string]interface{} result")
		}

		answer, ok := resultMap["answer"].(string)
		if !ok || answer == "" {
			t.Error("expected answer in result")
		}

		sources, ok := resultMap["sources"].([]map[string]interface{})
		if !ok {
			t.Error("expected sources in result")
		}
		if len(sources) == 0 {
			t.Error("expected at least one source")
		}
	})

	t.Run("string input", func(t *testing.T) {
		worker, _ := NewRAGWorker(RAGWorkerConfig{
			LLMProvider: llmProvider,
			VectorStore: vs,
		})

		task := &multiagent.Task{
			ID:    "task2",
			Input: "What is water made of?",
		}

		result, err := worker.HandleTask(ctx, task)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatal("expected map[string]interface{} result")
		}

		if _, ok := resultMap["answer"]; !ok {
			t.Error("expected answer in result")
		}
	})

	t.Run("missing LLMProvider returns error", func(t *testing.T) {
		_, err := NewRAGWorker(RAGWorkerConfig{
			VectorStore: vs,
		})
		if err == nil {
			t.Error("expected error for missing LLMProvider")
		}
	})

	t.Run("missing VectorStore returns error", func(t *testing.T) {
		_, err := NewRAGWorker(RAGWorkerConfig{
			LLMProvider: llmProvider,
		})
		if err == nil {
			t.Error("expected error for missing VectorStore")
		}
	})
}

func TestChainWorker(t *testing.T) {
	ctx := context.Background()

	t.Run("successful chain execution", func(t *testing.T) {
		mockChain := &mockChain{
			outputs: map[string]any{
				"result": "processed data",
			},
		}

		worker, err := NewChainWorker(ChainWorkerConfig{
			Name:         "TestChainWorker",
			Capabilities: []string{"processing", "analysis"},
			Chain:        mockChain,
		})
		if err != nil {
			t.Fatalf("failed to create worker: %v", err)
		}

		if worker.GetName() != "TestChainWorker" {
			t.Errorf("expected name 'TestChainWorker', got '%s'", worker.GetName())
		}

		capabilities := worker.GetCapabilities()
		if len(capabilities) != 2 {
			t.Errorf("expected 2 capabilities, got %d", len(capabilities))
		}

		task := &multiagent.Task{
			ID:    "task1",
			Input: map[string]interface{}{"input": "test data"},
		}

		result, err := worker.HandleTask(ctx, task)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatal("expected map[string]any result")
		}

		if resultMap["result"] != "processed data" {
			t.Errorf("unexpected result: %v", resultMap)
		}
	})

	t.Run("string input converts to map", func(t *testing.T) {
		mockChain := &mockChain{
			inputKeys: []string{"text"},
		}

		worker, _ := NewChainWorker(ChainWorkerConfig{
			Name:         "StringInputWorker",
			Capabilities: []string{"text_processing"},
			Chain:        mockChain,
		})

		task := &multiagent.Task{
			ID:    "task2",
			Input: "some text input",
		}

		_, err := worker.HandleTask(ctx, task)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing name returns error", func(t *testing.T) {
		_, err := NewChainWorker(ChainWorkerConfig{
			Capabilities: []string{"test"},
			Chain:        &mockChain{},
		})
		if err == nil {
			t.Error("expected error for missing name")
		}
	})

	t.Run("missing capabilities returns error", func(t *testing.T) {
		_, err := NewChainWorker(ChainWorkerConfig{
			Name:  "TestWorker",
			Chain: &mockChain{},
		})
		if err == nil {
			t.Error("expected error for missing capabilities")
		}
	})
}

func TestRetrievalWorker(t *testing.T) {
	ctx := context.Background()

	// Create mock components
	embedder := &mockEmbedder{dimensions: 384}
	vs, _ := vectorstore.NewMemoryVectorStore(vectorstore.MemoryVectorStoreConfig{
		Embedder: embedder,
	})

	// Add test documents
	vs.AddDocuments(ctx, []vectorstore.Document{
		{ID: "doc1", PageContent: "Information about topic A."},
		{ID: "doc2", PageContent: "Details about topic B."},
	})

	t.Run("successful retrieval", func(t *testing.T) {
		worker, err := NewRetrievalWorkerFromVectorStore(vs, 2)
		if err != nil {
			t.Fatalf("failed to create worker: %v", err)
		}

		if worker.GetName() != "RetrievalWorker" {
			t.Errorf("expected name 'RetrievalWorker', got '%s'", worker.GetName())
		}

		capabilities := worker.GetCapabilities()
		if len(capabilities) == 0 {
			t.Error("expected capabilities, got none")
		}

		task := &multiagent.Task{
			ID:    "task1",
			Input: map[string]interface{}{"query": "topic A"},
		}

		result, err := worker.HandleTask(ctx, task)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatal("expected map[string]interface{} result")
		}

		documents, ok := resultMap["documents"].([]map[string]interface{})
		if !ok {
			t.Error("expected documents in result")
		}

		count, ok := resultMap["count"].(int)
		if !ok || count == 0 {
			t.Error("expected non-zero count")
		}

		if len(documents) != count {
			t.Errorf("documents length %d doesn't match count %d", len(documents), count)
		}
	})

	t.Run("string input", func(t *testing.T) {
		worker, _ := NewRetrievalWorkerFromVectorStore(vs, 2)

		task := &multiagent.Task{
			ID:    "task2",
			Input: "search query",
		}

		result, err := worker.HandleTask(ctx, task)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]interface{})
		if resultMap["query"] != "search query" {
			t.Errorf("expected query in result, got %v", resultMap)
		}
	})

	t.Run("with custom retriever", func(t *testing.T) {
		ret, _ := retriever.NewVectorStoreRetriever(retriever.VectorStoreRetrieverConfig{
			VectorStore: vs,
			K:           1,
		})

		worker, err := NewRetrievalWorker(RetrievalWorkerConfig{
			Retriever: ret,
			TopK:      1,
		})
		if err != nil {
			t.Fatalf("failed to create worker: %v", err)
		}

		if worker.Retriever() != ret {
			t.Error("expected retriever to match")
		}
	})
}

func TestIngestionWorker(t *testing.T) {
	ctx := context.Background()

	// Create mock components
	embedder := &mockEmbedder{dimensions: 384}
	vs, _ := vectorstore.NewMemoryVectorStore(vectorstore.MemoryVectorStoreConfig{
		Embedder: embedder,
	})

	t.Run("ingest texts", func(t *testing.T) {
		worker, err := NewIngestionWorker(IngestionWorkerConfig{
			VectorStore: vs,
			Embedder:    embedder,
		})
		if err != nil {
			t.Fatalf("failed to create worker: %v", err)
		}

		if worker.GetName() != "IngestionWorker" {
			t.Errorf("expected name 'IngestionWorker', got '%s'", worker.GetName())
		}

		capabilities := worker.GetCapabilities()
		if len(capabilities) == 0 {
			t.Error("expected capabilities, got none")
		}

		task := &multiagent.Task{
			ID: "task1",
			Input: map[string]interface{}{
				"texts": []interface{}{
					"First document content.",
					"Second document content.",
				},
			},
		}

		result, err := worker.HandleTask(ctx, task)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatal("expected map[string]interface{} result")
		}

		if resultMap["ingested_count"].(int) != 2 {
			t.Errorf("expected 2 ingested, got %v", resultMap["ingested_count"])
		}
	})

	t.Run("ingest documents", func(t *testing.T) {
		worker, _ := NewIngestionWorker(IngestionWorkerConfig{
			VectorStore: vs,
			Embedder:    embedder,
		})

		task := &multiagent.Task{
			ID: "task2",
			Input: map[string]interface{}{
				"documents": []interface{}{
					map[string]interface{}{
						"id":      "custom-id",
						"content": "Document with custom ID.",
						"metadata": map[string]interface{}{
							"source": "test",
						},
					},
				},
			},
		}

		result, err := worker.HandleTask(ctx, task)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]interface{})
		if resultMap["ingested_count"].(int) != 1 {
			t.Errorf("expected 1 ingested, got %v", resultMap["ingested_count"])
		}
	})

	t.Run("missing vectorStore returns error", func(t *testing.T) {
		_, err := NewIngestionWorker(IngestionWorkerConfig{
			Embedder: embedder,
		})
		if err == nil {
			t.Error("expected error for missing vectorStore")
		}
	})

	t.Run("missing embedder returns error", func(t *testing.T) {
		_, err := NewIngestionWorker(IngestionWorkerConfig{
			VectorStore: vs,
		})
		if err == nil {
			t.Error("expected error for missing embedder")
		}
	})
}
