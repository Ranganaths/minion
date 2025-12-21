// Package workers provides multi-agent compatible workers for chain and RAG operations.
// These workers implement the multiagent.TaskHandler interface, allowing chains and
// RAG pipelines to be used within the multi-agent orchestration system.
package workers

import (
	"context"
	"fmt"
	"strings"

	"github.com/Ranganaths/minion/core/multiagent"
	"github.com/Ranganaths/minion/retriever"
	"github.com/Ranganaths/minion/vectorstore"
)

// RAGWorker implements multiagent.TaskHandler for RAG (Retrieval-Augmented Generation) tasks.
// It retrieves relevant documents from a vector store and uses an LLM to generate answers.
type RAGWorker struct {
	llmProvider multiagent.LLMProvider
	vectorStore vectorstore.VectorStore
	retriever   retriever.Retriever
	topK        int

	// Configuration
	maxTokens    int
	temperature  float64
	systemPrompt string
}

// RAGWorkerConfig configures the RAG worker
type RAGWorkerConfig struct {
	// LLMProvider is the LLM to use for generation (required)
	LLMProvider multiagent.LLMProvider

	// VectorStore is the vector store for document storage (required)
	VectorStore vectorstore.VectorStore

	// Retriever is the document retriever (optional, uses VectorStore if not provided)
	Retriever retriever.Retriever

	// TopK is the number of documents to retrieve (default: 5)
	TopK int

	// MaxTokens for LLM generation (default: 1000)
	MaxTokens int

	// Temperature for LLM generation (default: 0.3)
	Temperature float64

	// SystemPrompt customizes the RAG behavior (optional)
	SystemPrompt string
}

// NewRAGWorker creates a RAG-capable worker for the multi-agent system.
// The worker can handle RAG tasks by retrieving relevant documents and generating answers.
func NewRAGWorker(cfg RAGWorkerConfig) (*RAGWorker, error) {
	if cfg.LLMProvider == nil {
		return nil, fmt.Errorf("LLMProvider is required")
	}
	if cfg.VectorStore == nil {
		return nil, fmt.Errorf("VectorStore is required")
	}

	topK := cfg.TopK
	if topK <= 0 {
		topK = 5
	}

	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1000
	}

	temperature := cfg.Temperature
	if temperature <= 0 {
		temperature = 0.3
	}

	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a helpful assistant that answers questions based on the provided context. " +
			"If you cannot find the answer in the context, say \"I don't have enough information to answer this question.\""
	}

	return &RAGWorker{
		llmProvider:  cfg.LLMProvider,
		vectorStore:  cfg.VectorStore,
		retriever:    cfg.Retriever,
		topK:         topK,
		maxTokens:    maxTokens,
		temperature:  temperature,
		systemPrompt: systemPrompt,
	}, nil
}

// GetName implements multiagent.TaskHandler
func (w *RAGWorker) GetName() string {
	return "RAGWorker"
}

// GetCapabilities implements multiagent.TaskHandler.
// These capabilities are used by the orchestrator for task routing.
func (w *RAGWorker) GetCapabilities() []string {
	return []string{
		"rag",
		"retrieval",
		"question_answering",
		"document_search",
		"knowledge_base",
		"semantic_search",
	}
}

// HandleTask implements multiagent.TaskHandler.
// It processes RAG tasks by retrieving relevant documents and generating answers.
func (w *RAGWorker) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
	// Extract query from task input
	query, err := w.extractQuery(task.Input)
	if err != nil {
		return nil, err
	}

	// Retrieve relevant documents
	var docs []vectorstore.Document
	if w.retriever != nil {
		docs, err = w.retriever.GetRelevantDocuments(ctx, query)
	} else {
		docs, err = w.vectorStore.SimilaritySearch(ctx, query, w.topK)
	}
	if err != nil {
		return nil, fmt.Errorf("retrieval error: %w", err)
	}

	// Build context from documents
	context, sources := w.buildContext(docs)

	// Generate answer using LLM
	prompt := w.buildPrompt(context, query)

	resp, err := w.llmProvider.GenerateCompletion(ctx, &multiagent.CompletionRequest{
		SystemPrompt: w.systemPrompt,
		UserPrompt:   prompt,
		MaxTokens:    w.maxTokens,
		Temperature:  w.temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("llm error: %w", err)
	}

	// Return result in standard format
	return map[string]interface{}{
		"answer":         resp.Text,
		"sources":        sources,
		"tokens_used":    resp.TokensUsed,
		"documents_used": len(docs),
	}, nil
}

// extractQuery extracts the query from task input
func (w *RAGWorker) extractQuery(input interface{}) (string, error) {
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
		return "", fmt.Errorf("no query/question found in task input; expected keys: query, question, q, text, or input")

	default:
		return "", fmt.Errorf("invalid task input format: expected string or map[string]interface{}, got %T", input)
	}
}

// buildContext builds the context string and source list from documents
func (w *RAGWorker) buildContext(docs []vectorstore.Document) (string, []map[string]interface{}) {
	var contextBuilder strings.Builder
	var sources []map[string]interface{}

	for i, doc := range docs {
		contextBuilder.WriteString(fmt.Sprintf("Document %d:\n%s\n\n", i+1, doc.PageContent))
		sources = append(sources, map[string]interface{}{
			"id":       doc.ID,
			"content":  doc.PageContent,
			"metadata": doc.Metadata,
		})
	}

	return contextBuilder.String(), sources
}

// buildPrompt builds the LLM prompt from context and query
func (w *RAGWorker) buildPrompt(context, query string) string {
	return fmt.Sprintf(`Use the following context to answer the question.

Context:
%s

Question: %s

Answer:`, context, query)
}

// AddDocuments adds documents to the underlying vector store
func (w *RAGWorker) AddDocuments(ctx context.Context, docs []vectorstore.Document) error {
	_, err := w.vectorStore.AddDocuments(ctx, docs)
	return err
}
