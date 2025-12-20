// Package integration provides workers that integrate the chain package with the multi-agent system.
// These workers implement the TaskHandler interface while using RAG capabilities.
package integration

import (
	"context"
	"fmt"

	"github.com/Ranganaths/minion/chain"
	"github.com/Ranganaths/minion/core/multiagent"
	"github.com/Ranganaths/minion/embeddings"
	"github.com/Ranganaths/minion/llm"
	"github.com/Ranganaths/minion/rag"
	"github.com/Ranganaths/minion/retriever"
	"github.com/Ranganaths/minion/vectorstore"
)

// RAGWorker is a multi-agent worker that uses RAG for knowledge-grounded responses
type RAGWorker struct {
	pipeline    *rag.Pipeline
	llmProvider llm.Provider
	name        string
}

// RAGWorkerConfig configures the RAG worker
type RAGWorkerConfig struct {
	// Embedder is the embedding provider
	Embedder embeddings.Embedder

	// LLM is the language model provider
	LLM llm.Provider

	// VectorStore is an optional pre-configured vector store
	VectorStore vectorstore.VectorStore

	// Name is the worker name
	Name string

	// RetrieverK is the number of documents to retrieve
	RetrieverK int
}

// NewRAGWorker creates a new RAG-enabled worker
func NewRAGWorker(cfg RAGWorkerConfig) (*RAGWorker, error) {
	if cfg.Embedder == nil {
		return nil, fmt.Errorf("embedder is required")
	}
	if cfg.LLM == nil {
		return nil, fmt.Errorf("LLM provider is required")
	}

	name := cfg.Name
	if name == "" {
		name = "rag_worker"
	}

	retrieverK := cfg.RetrieverK
	if retrieverK <= 0 {
		retrieverK = 4
	}

	pipeline, err := rag.NewPipeline(rag.PipelineConfig{
		Embedder:      cfg.Embedder,
		VectorStore:   cfg.VectorStore,
		LLM:           cfg.LLM,
		RetrieverK:    retrieverK,
		ReturnSources: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create RAG pipeline: %w", err)
	}

	return &RAGWorker{
		pipeline:    pipeline,
		llmProvider: cfg.LLM,
		name:        name,
	}, nil
}

// HandleTask implements the TaskHandler interface
func (w *RAGWorker) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
	// Extract query from task
	query, err := w.extractQuery(task)
	if err != nil {
		return nil, err
	}

	// Check if task includes documents to add
	if docs, ok := task.Input["documents"].([]interface{}); ok {
		if err := w.addDocumentsFromInput(ctx, docs); err != nil {
			return nil, fmt.Errorf("failed to add documents: %w", err)
		}
	}

	// Perform RAG query
	answer, sources, err := w.pipeline.QueryWithSources(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("RAG query failed: %w", err)
	}

	// Build response
	result := map[string]interface{}{
		"answer":  answer,
		"query":   query,
		"sources": w.formatSources(sources),
	}

	return result, nil
}

// extractQuery gets the query from task input
func (w *RAGWorker) extractQuery(task *multiagent.Task) (string, error) {
	// Try different input keys
	if query, ok := task.Input["query"].(string); ok {
		return query, nil
	}
	if question, ok := task.Input["question"].(string); ok {
		return question, nil
	}
	if text, ok := task.Input["text"].(string); ok {
		return text, nil
	}
	if input, ok := task.Input["input"].(string); ok {
		return input, nil
	}

	// Use task description as fallback
	if task.Description != "" {
		return task.Description, nil
	}

	return "", fmt.Errorf("no query found in task input")
}

// addDocumentsFromInput adds documents from task input to the pipeline
func (w *RAGWorker) addDocumentsFromInput(ctx context.Context, docs []interface{}) error {
	var texts []string
	var metadatas []map[string]any

	for _, doc := range docs {
		switch v := doc.(type) {
		case string:
			texts = append(texts, v)
			metadatas = append(metadatas, nil)
		case map[string]interface{}:
			if content, ok := v["content"].(string); ok {
				texts = append(texts, content)
				meta := make(map[string]any)
				for k, val := range v {
					if k != "content" {
						meta[k] = val
					}
				}
				metadatas = append(metadatas, meta)
			}
		}
	}

	if len(texts) > 0 {
		return w.pipeline.AddTexts(ctx, texts, metadatas)
	}

	return nil
}

// formatSources formats source documents for output
func (w *RAGWorker) formatSources(docs []vectorstore.Document) []map[string]interface{} {
	sources := make([]map[string]interface{}, len(docs))
	for i, doc := range docs {
		sources[i] = map[string]interface{}{
			"id":       doc.ID,
			"content":  doc.PageContent,
			"metadata": doc.Metadata,
		}
	}
	return sources
}

// GetCapabilities implements the TaskHandler interface
func (w *RAGWorker) GetCapabilities() []string {
	return []string{
		"rag",
		"knowledge_retrieval",
		"question_answering",
		"document_search",
		"semantic_search",
	}
}

// GetName implements the TaskHandler interface
func (w *RAGWorker) GetName() string {
	return w.name
}

// AddDocuments adds documents to the RAG worker's knowledge base
func (w *RAGWorker) AddDocuments(ctx context.Context, docs []vectorstore.Document) error {
	return w.pipeline.AddDocuments(ctx, docs)
}

// AddTexts adds text content to the knowledge base
func (w *RAGWorker) AddTexts(ctx context.Context, texts []string, metadatas []map[string]any) error {
	return w.pipeline.AddTexts(ctx, texts, metadatas)
}

// Search performs a similarity search without generation
func (w *RAGWorker) Search(ctx context.Context, query string, k int) ([]vectorstore.Document, error) {
	return w.pipeline.Search(ctx, query, k)
}

// Clear clears the knowledge base
func (w *RAGWorker) Clear() {
	w.pipeline.Clear()
}

// ChainWorker is a multi-agent worker that uses chains for processing
type ChainWorker struct {
	chain       chain.Chain
	llmProvider llm.Provider
	name        string
}

// ChainWorkerConfig configures the chain worker
type ChainWorkerConfig struct {
	// Chain is the chain to execute
	Chain chain.Chain

	// LLM is the language model provider (for creating default chains)
	LLM llm.Provider

	// Name is the worker name
	Name string
}

// NewChainWorker creates a new chain-based worker
func NewChainWorker(cfg ChainWorkerConfig) (*ChainWorker, error) {
	if cfg.Chain == nil && cfg.LLM == nil {
		return nil, fmt.Errorf("either Chain or LLM is required")
	}

	var c chain.Chain
	if cfg.Chain != nil {
		c = cfg.Chain
	} else {
		// Create a default LLM chain
		var err error
		c, err = chain.NewLLMChain(chain.LLMChainConfig{
			LLM:            cfg.LLM,
			PromptTemplate: "{{.input}}",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create default chain: %w", err)
		}
	}

	name := cfg.Name
	if name == "" {
		name = "chain_worker"
	}

	return &ChainWorker{
		chain:       c,
		llmProvider: cfg.LLM,
		name:        name,
	}, nil
}

// HandleTask implements the TaskHandler interface
func (w *ChainWorker) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
	// Convert task input to chain input format
	inputs := make(map[string]any)

	// Copy all task inputs
	for k, v := range task.Input {
		inputs[k] = v
	}

	// Add task metadata as potential inputs
	if task.Name != "" {
		inputs["task_name"] = task.Name
	}
	if task.Description != "" {
		inputs["task_description"] = task.Description
	}

	// Execute chain
	result, err := w.chain.Call(ctx, inputs)
	if err != nil {
		return nil, fmt.Errorf("chain execution failed: %w", err)
	}

	return result, nil
}

// GetCapabilities implements the TaskHandler interface
func (w *ChainWorker) GetCapabilities() []string {
	return []string{
		"chain_execution",
		"llm_processing",
		"text_generation",
	}
}

// GetName implements the TaskHandler interface
func (w *ChainWorker) GetName() string {
	return w.name
}

// RetrieverWorker is a worker specialized for document retrieval
type RetrieverWorker struct {
	retriever   retriever.Retriever
	vectorStore vectorstore.VectorStore
	embedder    embeddings.Embedder
	name        string
}

// RetrieverWorkerConfig configures the retriever worker
type RetrieverWorkerConfig struct {
	// Embedder is the embedding provider
	Embedder embeddings.Embedder

	// VectorStore is an optional pre-configured vector store
	VectorStore vectorstore.VectorStore

	// K is the number of documents to retrieve
	K int

	// Name is the worker name
	Name string
}

// NewRetrieverWorker creates a new retriever worker
func NewRetrieverWorker(cfg RetrieverWorkerConfig) (*RetrieverWorker, error) {
	if cfg.Embedder == nil {
		return nil, fmt.Errorf("embedder is required")
	}

	vs := cfg.VectorStore
	if vs == nil {
		var err error
		vs, err = vectorstore.NewMemoryVectorStore(vectorstore.MemoryVectorStoreConfig{
			Embedder: cfg.Embedder,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create vector store: %w", err)
		}
	}

	k := cfg.K
	if k <= 0 {
		k = 4
	}

	ret, err := retriever.NewVectorStoreRetriever(retriever.VectorStoreRetrieverConfig{
		VectorStore: vs,
		K:           k,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create retriever: %w", err)
	}

	name := cfg.Name
	if name == "" {
		name = "retriever_worker"
	}

	return &RetrieverWorker{
		retriever:   ret,
		vectorStore: vs,
		embedder:    cfg.Embedder,
		name:        name,
	}, nil
}

// HandleTask implements the TaskHandler interface
func (w *RetrieverWorker) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
	// Handle different operations based on task
	operation, _ := task.Input["operation"].(string)

	switch operation {
	case "add", "index":
		return w.handleAddDocuments(ctx, task)
	case "delete", "remove":
		return w.handleDeleteDocuments(ctx, task)
	default:
		return w.handleSearch(ctx, task)
	}
}

// handleSearch handles search operations
func (w *RetrieverWorker) handleSearch(ctx context.Context, task *multiagent.Task) (interface{}, error) {
	query, err := w.extractQuery(task)
	if err != nil {
		return nil, err
	}

	docs, err := w.retriever.GetRelevantDocuments(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("retrieval failed: %w", err)
	}

	return map[string]interface{}{
		"query":     query,
		"documents": w.formatDocuments(docs),
		"count":     len(docs),
	}, nil
}

// handleAddDocuments handles document indexing
func (w *RetrieverWorker) handleAddDocuments(ctx context.Context, task *multiagent.Task) (interface{}, error) {
	texts, ok := task.Input["texts"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("texts not provided")
	}

	var docs []vectorstore.Document
	for _, t := range texts {
		if content, ok := t.(string); ok {
			docs = append(docs, vectorstore.NewDocument(content))
		}
	}

	ids, err := w.vectorStore.AddDocuments(ctx, docs)
	if err != nil {
		return nil, fmt.Errorf("failed to add documents: %w", err)
	}

	return map[string]interface{}{
		"ids":   ids,
		"count": len(ids),
	}, nil
}

// handleDeleteDocuments handles document deletion
func (w *RetrieverWorker) handleDeleteDocuments(ctx context.Context, task *multiagent.Task) (interface{}, error) {
	ids, ok := task.Input["ids"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("ids not provided")
	}

	var deleteIDs []string
	for _, id := range ids {
		if idStr, ok := id.(string); ok {
			deleteIDs = append(deleteIDs, idStr)
		}
	}

	err := w.vectorStore.Delete(ctx, deleteIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to delete documents: %w", err)
	}

	return map[string]interface{}{
		"deleted": deleteIDs,
		"count":   len(deleteIDs),
	}, nil
}

// extractQuery gets the query from task input
func (w *RetrieverWorker) extractQuery(task *multiagent.Task) (string, error) {
	if query, ok := task.Input["query"].(string); ok {
		return query, nil
	}
	if question, ok := task.Input["question"].(string); ok {
		return question, nil
	}
	if task.Description != "" {
		return task.Description, nil
	}
	return "", fmt.Errorf("no query found")
}

// formatDocuments formats documents for output
func (w *RetrieverWorker) formatDocuments(docs []vectorstore.Document) []map[string]interface{} {
	result := make([]map[string]interface{}, len(docs))
	for i, doc := range docs {
		result[i] = map[string]interface{}{
			"id":       doc.ID,
			"content":  doc.PageContent,
			"metadata": doc.Metadata,
		}
	}
	return result
}

// GetCapabilities implements the TaskHandler interface
func (w *RetrieverWorker) GetCapabilities() []string {
	return []string{
		"document_retrieval",
		"semantic_search",
		"document_indexing",
		"similarity_search",
	}
}

// GetName implements the TaskHandler interface
func (w *RetrieverWorker) GetName() string {
	return w.name
}
