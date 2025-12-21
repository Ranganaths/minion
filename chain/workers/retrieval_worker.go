package workers

import (
	"context"
	"fmt"

	"github.com/Ranganaths/minion/core/multiagent"
	"github.com/Ranganaths/minion/retriever"
	"github.com/Ranganaths/minion/vectorstore"
)

// RetrievalWorker is a specialized worker for document retrieval only (no LLM generation).
// Use this when you need to retrieve documents without generating an answer.
type RetrievalWorker struct {
	retriever retriever.Retriever
	topK      int
}

// RetrievalWorkerConfig configures the retrieval worker
type RetrievalWorkerConfig struct {
	// Retriever is the document retriever (required)
	Retriever retriever.Retriever

	// TopK is the number of documents to retrieve (default: 5)
	TopK int
}

// NewRetrievalWorker creates a retrieval-only worker for the multi-agent system.
func NewRetrievalWorker(cfg RetrievalWorkerConfig) (*RetrievalWorker, error) {
	if cfg.Retriever == nil {
		return nil, fmt.Errorf("retriever is required")
	}

	topK := cfg.TopK
	if topK <= 0 {
		topK = 5
	}

	return &RetrievalWorker{
		retriever: cfg.Retriever,
		topK:      topK,
	}, nil
}

// NewRetrievalWorkerFromVectorStore creates a retrieval worker from a vector store.
func NewRetrievalWorkerFromVectorStore(vs vectorstore.VectorStore, topK int) (*RetrievalWorker, error) {
	if vs == nil {
		return nil, fmt.Errorf("vector store is required")
	}

	if topK <= 0 {
		topK = 5
	}

	// Create a basic retriever from the vector store
	ret, err := retriever.NewVectorStoreRetriever(retriever.VectorStoreRetrieverConfig{
		VectorStore: vs,
		K:           topK,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create retriever: %w", err)
	}

	return &RetrievalWorker{
		retriever: ret,
		topK:      topK,
	}, nil
}

// GetName implements multiagent.TaskHandler
func (w *RetrievalWorker) GetName() string {
	return "RetrievalWorker"
}

// GetCapabilities implements multiagent.TaskHandler
func (w *RetrievalWorker) GetCapabilities() []string {
	return []string{
		"retrieval",
		"document_search",
		"semantic_search",
	}
}

// HandleTask implements multiagent.TaskHandler.
// It retrieves documents matching the query and returns them.
func (w *RetrievalWorker) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
	query, err := w.extractQuery(task.Input)
	if err != nil {
		return nil, err
	}

	docs, err := w.retriever.GetRelevantDocuments(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("retrieval error: %w", err)
	}

	// Convert to serializable format
	results := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		results = append(results, map[string]interface{}{
			"id":       doc.ID,
			"content":  doc.PageContent,
			"metadata": doc.Metadata,
		})
	}

	return map[string]interface{}{
		"documents": results,
		"count":     len(results),
		"query":     query,
	}, nil
}

// extractQuery extracts the query from task input
func (w *RetrievalWorker) extractQuery(input interface{}) (string, error) {
	switch v := input.(type) {
	case string:
		if v == "" {
			return "", fmt.Errorf("query cannot be empty")
		}
		return v, nil

	case map[string]interface{}:
		// Try common query keys
		for _, key := range []string{"query", "question", "q", "text", "input"} {
			if q, ok := v[key].(string); ok && q != "" {
				return q, nil
			}
		}
		return "", fmt.Errorf("no query found in task input; expected keys: query, question, q, text, or input")

	default:
		return "", fmt.Errorf("invalid task input format: expected string or map[string]interface{}, got %T", input)
	}
}

// Retriever returns the underlying retriever
func (w *RetrievalWorker) Retriever() retriever.Retriever {
	return w.retriever
}
