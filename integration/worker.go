// Package integration provides workers that integrate the chain package with the multi-agent system.
// These workers implement the TaskHandler interface while using RAG capabilities.
//
// NOTE: The main integration workers are in rag_worker.go which imports core/multiagent.
// This file provides standalone types for testing and documentation purposes.
package integration

import (
	"context"
	"fmt"

	"github.com/Ranganaths/minion/chain"
	"github.com/Ranganaths/minion/embeddings"
	"github.com/Ranganaths/minion/llm"
	"github.com/Ranganaths/minion/rag"
	"github.com/Ranganaths/minion/retriever"
	"github.com/Ranganaths/minion/vectorstore"
)

// Task represents a task to be processed (mirrors multiagent.Task structure)
type Task struct {
	ID          string
	Name        string
	Description string
	Input       map[string]interface{}
	Priority    int
	Status      string
	Metadata    map[string]interface{}
}

// TaskHandler defines how a worker processes tasks (mirrors multiagent.TaskHandler)
type TaskHandler interface {
	HandleTask(ctx context.Context, task *Task) (interface{}, error)
	GetCapabilities() []string
	GetName() string
}

// StandaloneRAGWorker is a RAG worker that doesn't depend on the multiagent package
type StandaloneRAGWorker struct {
	pipeline    *rag.Pipeline
	llmProvider llm.Provider
	name        string
}

// StandaloneRAGWorkerConfig configures the standalone RAG worker
type StandaloneRAGWorkerConfig struct {
	Embedder    embeddings.Embedder
	LLM         llm.Provider
	VectorStore vectorstore.VectorStore
	Name        string
	RetrieverK  int
}

// NewStandaloneRAGWorker creates a new standalone RAG worker
func NewStandaloneRAGWorker(cfg StandaloneRAGWorkerConfig) (*StandaloneRAGWorker, error) {
	if cfg.Embedder == nil {
		return nil, fmt.Errorf("embedder is required")
	}
	if cfg.LLM == nil {
		return nil, fmt.Errorf("LLM provider is required")
	}

	name := cfg.Name
	if name == "" {
		name = "standalone_rag_worker"
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

	return &StandaloneRAGWorker{
		pipeline:    pipeline,
		llmProvider: cfg.LLM,
		name:        name,
	}, nil
}

// HandleTask implements the TaskHandler interface
func (w *StandaloneRAGWorker) HandleTask(ctx context.Context, task *Task) (interface{}, error) {
	query, err := w.extractQuery(task)
	if err != nil {
		return nil, err
	}

	if docs, ok := task.Input["documents"].([]interface{}); ok {
		if err := w.addDocumentsFromInput(ctx, docs); err != nil {
			return nil, fmt.Errorf("failed to add documents: %w", err)
		}
	}

	answer, sources, err := w.pipeline.QueryWithSources(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("RAG query failed: %w", err)
	}

	return map[string]interface{}{
		"answer":  answer,
		"query":   query,
		"sources": w.formatSources(sources),
	}, nil
}

func (w *StandaloneRAGWorker) extractQuery(task *Task) (string, error) {
	if query, ok := task.Input["query"].(string); ok {
		return query, nil
	}
	if question, ok := task.Input["question"].(string); ok {
		return question, nil
	}
	if task.Description != "" {
		return task.Description, nil
	}
	return "", fmt.Errorf("no query found in task input")
}

func (w *StandaloneRAGWorker) addDocumentsFromInput(ctx context.Context, docs []interface{}) error {
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

func (w *StandaloneRAGWorker) formatSources(docs []vectorstore.Document) []map[string]interface{} {
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
func (w *StandaloneRAGWorker) GetCapabilities() []string {
	return []string{"rag", "knowledge_retrieval", "question_answering"}
}

// GetName implements the TaskHandler interface
func (w *StandaloneRAGWorker) GetName() string {
	return w.name
}

// AddDocuments adds documents to the RAG worker's knowledge base
func (w *StandaloneRAGWorker) AddDocuments(ctx context.Context, docs []vectorstore.Document) error {
	return w.pipeline.AddDocuments(ctx, docs)
}

// AddTexts adds text content to the knowledge base
func (w *StandaloneRAGWorker) AddTexts(ctx context.Context, texts []string, metadatas []map[string]any) error {
	return w.pipeline.AddTexts(ctx, texts, metadatas)
}

// Search performs a similarity search without generation
func (w *StandaloneRAGWorker) Search(ctx context.Context, query string, k int) ([]vectorstore.Document, error) {
	return w.pipeline.Search(ctx, query, k)
}

// Clear clears the knowledge base
func (w *StandaloneRAGWorker) Clear() {
	w.pipeline.Clear()
}

// StandaloneChainWorker is a chain-based worker that doesn't depend on multiagent
type StandaloneChainWorker struct {
	chain       chain.Chain
	llmProvider llm.Provider
	name        string
}

// StandaloneChainWorkerConfig configures the standalone chain worker
type StandaloneChainWorkerConfig struct {
	Chain chain.Chain
	LLM   llm.Provider
	Name  string
}

// NewStandaloneChainWorker creates a new standalone chain worker
func NewStandaloneChainWorker(cfg StandaloneChainWorkerConfig) (*StandaloneChainWorker, error) {
	if cfg.Chain == nil && cfg.LLM == nil {
		return nil, fmt.Errorf("either Chain or LLM is required")
	}

	var c chain.Chain
	if cfg.Chain != nil {
		c = cfg.Chain
	} else {
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
		name = "standalone_chain_worker"
	}

	return &StandaloneChainWorker{
		chain:       c,
		llmProvider: cfg.LLM,
		name:        name,
	}, nil
}

// HandleTask implements the TaskHandler interface
func (w *StandaloneChainWorker) HandleTask(ctx context.Context, task *Task) (interface{}, error) {
	inputs := make(map[string]any)
	for k, v := range task.Input {
		inputs[k] = v
	}
	if task.Name != "" {
		inputs["task_name"] = task.Name
	}
	if task.Description != "" {
		inputs["task_description"] = task.Description
	}

	return w.chain.Call(ctx, inputs)
}

// GetCapabilities implements the TaskHandler interface
func (w *StandaloneChainWorker) GetCapabilities() []string {
	return []string{"chain_execution", "llm_processing", "text_generation"}
}

// GetName implements the TaskHandler interface
func (w *StandaloneChainWorker) GetName() string {
	return w.name
}

// StandaloneRetrieverWorker is a retriever worker that doesn't depend on multiagent
type StandaloneRetrieverWorker struct {
	retriever   retriever.Retriever
	vectorStore vectorstore.VectorStore
	embedder    embeddings.Embedder
	name        string
}

// StandaloneRetrieverWorkerConfig configures the standalone retriever worker
type StandaloneRetrieverWorkerConfig struct {
	Embedder    embeddings.Embedder
	VectorStore vectorstore.VectorStore
	K           int
	Name        string
}

// NewStandaloneRetrieverWorker creates a new standalone retriever worker
func NewStandaloneRetrieverWorker(cfg StandaloneRetrieverWorkerConfig) (*StandaloneRetrieverWorker, error) {
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
		name = "standalone_retriever_worker"
	}

	return &StandaloneRetrieverWorker{
		retriever:   ret,
		vectorStore: vs,
		embedder:    cfg.Embedder,
		name:        name,
	}, nil
}

// HandleTask implements the TaskHandler interface
func (w *StandaloneRetrieverWorker) HandleTask(ctx context.Context, task *Task) (interface{}, error) {
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

func (w *StandaloneRetrieverWorker) handleSearch(ctx context.Context, task *Task) (interface{}, error) {
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

func (w *StandaloneRetrieverWorker) handleAddDocuments(ctx context.Context, task *Task) (interface{}, error) {
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

	return map[string]interface{}{"ids": ids, "count": len(ids)}, nil
}

func (w *StandaloneRetrieverWorker) handleDeleteDocuments(ctx context.Context, task *Task) (interface{}, error) {
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

	if err := w.vectorStore.Delete(ctx, deleteIDs); err != nil {
		return nil, fmt.Errorf("failed to delete documents: %w", err)
	}

	return map[string]interface{}{"deleted": deleteIDs, "count": len(deleteIDs)}, nil
}

func (w *StandaloneRetrieverWorker) extractQuery(task *Task) (string, error) {
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

func (w *StandaloneRetrieverWorker) formatDocuments(docs []vectorstore.Document) []map[string]interface{} {
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
func (w *StandaloneRetrieverWorker) GetCapabilities() []string {
	return []string{"document_retrieval", "semantic_search", "document_indexing"}
}

// GetName implements the TaskHandler interface
func (w *StandaloneRetrieverWorker) GetName() string {
	return w.name
}
