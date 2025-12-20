// Package rag provides a complete RAG (Retrieval-Augmented Generation) pipeline.
// This package integrates document loading, splitting, embedding, storage, retrieval, and generation.
package rag

import (
	"context"
	"fmt"

	"github.com/Ranganaths/minion/chain"
	"github.com/Ranganaths/minion/documentloader"
	"github.com/Ranganaths/minion/embeddings"
	"github.com/Ranganaths/minion/llm"
	"github.com/Ranganaths/minion/retriever"
	"github.com/Ranganaths/minion/textsplitter"
	"github.com/Ranganaths/minion/vectorstore"
)

// Pipeline is a complete RAG pipeline
type Pipeline struct {
	vectorStore  vectorstore.VectorStore
	embedder     embeddings.Embedder
	retriever    retriever.Retriever
	llmProvider  llm.Provider
	splitter     textsplitter.TextSplitter
	ragChain     *chain.RAGChain
}

// PipelineConfig configures the RAG pipeline
type PipelineConfig struct {
	// Embedder is the embedding provider
	Embedder embeddings.Embedder

	// VectorStore is the vector store (optional, creates in-memory if not provided)
	VectorStore vectorstore.VectorStore

	// LLM is the language model provider
	LLM llm.Provider

	// Splitter is the text splitter (optional, uses default if not provided)
	Splitter textsplitter.TextSplitter

	// RetrieverK is the number of documents to retrieve (default: 4)
	RetrieverK int

	// ReturnSources includes source documents in response
	ReturnSources bool

	// PromptFunc customizes the RAG prompt
	PromptFunc chain.RAGPromptFunc

	// ChainOptions are options for the RAG chain
	ChainOptions []chain.Option
}

// NewPipeline creates a new RAG pipeline
func NewPipeline(cfg PipelineConfig) (*Pipeline, error) {
	if cfg.Embedder == nil {
		return nil, fmt.Errorf("embedder is required")
	}
	if cfg.LLM == nil {
		return nil, fmt.Errorf("LLM provider is required")
	}

	// Create vector store if not provided
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

	// Create splitter if not provided
	splitter := cfg.Splitter
	if splitter == nil {
		splitter = textsplitter.NewRecursiveCharacterTextSplitter(textsplitter.RecursiveCharacterTextSplitterConfig{
			ChunkSize:    1000,
			ChunkOverlap: 200,
		})
	}

	// Create retriever
	retrieverK := cfg.RetrieverK
	if retrieverK <= 0 {
		retrieverK = 4
	}

	ret, err := retriever.NewVectorStoreRetriever(retriever.VectorStoreRetrieverConfig{
		VectorStore: vs,
		K:           retrieverK,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create retriever: %w", err)
	}

	// Create RAG chain
	ragChain, err := chain.NewRAGChain(chain.RAGChainConfig{
		Retriever:     ret,
		LLM:           cfg.LLM,
		PromptFunc:    cfg.PromptFunc,
		ReturnSources: cfg.ReturnSources,
		Options:       cfg.ChainOptions,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create RAG chain: %w", err)
	}

	return &Pipeline{
		vectorStore: vs,
		embedder:    cfg.Embedder,
		retriever:   ret,
		llmProvider: cfg.LLM,
		splitter:    splitter,
		ragChain:    ragChain,
	}, nil
}

// AddDocuments adds documents to the pipeline
func (p *Pipeline) AddDocuments(ctx context.Context, docs []vectorstore.Document) error {
	// Split documents
	splitDocs := p.splitter.SplitDocuments(docs)

	// Add to vector store
	_, err := p.vectorStore.AddDocuments(ctx, splitDocs)
	return err
}

// AddTexts adds text content to the pipeline
func (p *Pipeline) AddTexts(ctx context.Context, texts []string, metadatas []map[string]any) error {
	docs := make([]vectorstore.Document, len(texts))
	for i, text := range texts {
		var metadata map[string]any
		if i < len(metadatas) {
			metadata = metadatas[i]
		}
		docs[i] = vectorstore.NewDocumentWithMetadata(text, metadata)
	}

	return p.AddDocuments(ctx, docs)
}

// LoadDocuments loads and adds documents from a loader
func (p *Pipeline) LoadDocuments(ctx context.Context, loader documentloader.Loader) error {
	docs, err := loader.LoadAndSplit(ctx, p.splitter)
	if err != nil {
		return fmt.Errorf("failed to load documents: %w", err)
	}

	_, err = p.vectorStore.AddDocuments(ctx, docs)
	return err
}

// Query runs a RAG query
func (p *Pipeline) Query(ctx context.Context, question string) (string, error) {
	result, err := p.ragChain.Call(ctx, map[string]any{
		"question": question,
	})
	if err != nil {
		return "", err
	}

	answer, ok := result["answer"].(string)
	if !ok {
		return "", fmt.Errorf("unexpected answer type: %T", result["answer"])
	}

	return answer, nil
}

// QueryWithSources runs a RAG query and returns source documents
func (p *Pipeline) QueryWithSources(ctx context.Context, question string) (string, []vectorstore.Document, error) {
	result, err := p.ragChain.Call(ctx, map[string]any{
		"question": question,
	})
	if err != nil {
		return "", nil, err
	}

	answer, ok := result["answer"].(string)
	if !ok {
		return "", nil, fmt.Errorf("unexpected answer type: %T", result["answer"])
	}

	var sources []vectorstore.Document
	if srcDocs, ok := result["source_documents"].([]vectorstore.Document); ok {
		sources = srcDocs
	}

	return answer, sources, nil
}

// Search performs a similarity search without LLM generation
func (p *Pipeline) Search(ctx context.Context, query string, k int) ([]vectorstore.Document, error) {
	return p.vectorStore.SimilaritySearch(ctx, query, k)
}

// VectorStore returns the underlying vector store
func (p *Pipeline) VectorStore() vectorstore.VectorStore {
	return p.vectorStore
}

// Retriever returns the underlying retriever
func (p *Pipeline) Retriever() retriever.Retriever {
	return p.retriever
}

// Clear removes all documents from the pipeline
func (p *Pipeline) Clear() {
	if memStore, ok := p.vectorStore.(*vectorstore.MemoryVectorStore); ok {
		memStore.Clear()
	}
}

// PipelineBuilder helps construct a pipeline with a fluent API
type PipelineBuilder struct {
	config PipelineConfig
}

// NewPipelineBuilder creates a new pipeline builder
func NewPipelineBuilder() *PipelineBuilder {
	return &PipelineBuilder{
		config: PipelineConfig{
			RetrieverK:    4,
			ReturnSources: true,
		},
	}
}

// WithEmbedder sets the embedder
func (b *PipelineBuilder) WithEmbedder(e embeddings.Embedder) *PipelineBuilder {
	b.config.Embedder = e
	return b
}

// WithVectorStore sets the vector store
func (b *PipelineBuilder) WithVectorStore(vs vectorstore.VectorStore) *PipelineBuilder {
	b.config.VectorStore = vs
	return b
}

// WithLLM sets the LLM provider
func (b *PipelineBuilder) WithLLM(l llm.Provider) *PipelineBuilder {
	b.config.LLM = l
	return b
}

// WithSplitter sets the text splitter
func (b *PipelineBuilder) WithSplitter(s textsplitter.TextSplitter) *PipelineBuilder {
	b.config.Splitter = s
	return b
}

// WithRetrieverK sets the number of documents to retrieve
func (b *PipelineBuilder) WithRetrieverK(k int) *PipelineBuilder {
	b.config.RetrieverK = k
	return b
}

// WithReturnSources sets whether to return source documents
func (b *PipelineBuilder) WithReturnSources(returnSources bool) *PipelineBuilder {
	b.config.ReturnSources = returnSources
	return b
}

// WithPromptFunc sets a custom prompt function
func (b *PipelineBuilder) WithPromptFunc(fn chain.RAGPromptFunc) *PipelineBuilder {
	b.config.PromptFunc = fn
	return b
}

// WithChainOptions sets chain options
func (b *PipelineBuilder) WithChainOptions(opts ...chain.Option) *PipelineBuilder {
	b.config.ChainOptions = opts
	return b
}

// Build creates the pipeline
func (b *PipelineBuilder) Build() (*Pipeline, error) {
	return NewPipeline(b.config)
}
