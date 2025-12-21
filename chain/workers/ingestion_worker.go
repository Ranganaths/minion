package workers

import (
	"context"
	"fmt"

	"github.com/Ranganaths/minion/core/multiagent"
	"github.com/Ranganaths/minion/documentloader"
	"github.com/Ranganaths/minion/embeddings"
	"github.com/Ranganaths/minion/textsplitter"
	"github.com/Ranganaths/minion/vectorstore"
)

// IngestionWorker is a specialized worker for document ingestion.
// It handles loading, splitting, embedding, and storing documents.
type IngestionWorker struct {
	vectorStore vectorstore.VectorStore
	embedder    embeddings.Embedder
	splitter    textsplitter.TextSplitter
}

// IngestionWorkerConfig configures the ingestion worker
type IngestionWorkerConfig struct {
	// VectorStore is the vector store for document storage (required)
	VectorStore vectorstore.VectorStore

	// Embedder is the embedding provider (required)
	Embedder embeddings.Embedder

	// Splitter is the text splitter (optional, uses default if not provided)
	Splitter textsplitter.TextSplitter
}

// NewIngestionWorker creates a document ingestion worker for the multi-agent system.
func NewIngestionWorker(cfg IngestionWorkerConfig) (*IngestionWorker, error) {
	if cfg.VectorStore == nil {
		return nil, fmt.Errorf("vectorStore is required")
	}
	if cfg.Embedder == nil {
		return nil, fmt.Errorf("embedder is required")
	}

	splitter := cfg.Splitter
	if splitter == nil {
		splitter = textsplitter.NewRecursiveCharacterTextSplitter(textsplitter.RecursiveCharacterTextSplitterConfig{
			ChunkSize:    1000,
			ChunkOverlap: 200,
		})
	}

	return &IngestionWorker{
		vectorStore: cfg.VectorStore,
		embedder:    cfg.Embedder,
		splitter:    splitter,
	}, nil
}

// GetName implements multiagent.TaskHandler
func (w *IngestionWorker) GetName() string {
	return "IngestionWorker"
}

// GetCapabilities implements multiagent.TaskHandler
func (w *IngestionWorker) GetCapabilities() []string {
	return []string{
		"ingestion",
		"document_ingestion",
		"indexing",
		"embedding",
	}
}

// HandleTask implements multiagent.TaskHandler.
// It processes ingestion tasks by loading, splitting, and storing documents.
func (w *IngestionWorker) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
	input, ok := task.Input.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid task input format: expected map[string]interface{}, got %T", task.Input)
	}

	// Determine ingestion type
	if texts, ok := input["texts"].([]interface{}); ok {
		return w.ingestTexts(ctx, texts, input["metadatas"])
	}

	if docs, ok := input["documents"].([]interface{}); ok {
		return w.ingestDocuments(ctx, docs)
	}

	if source, ok := input["source"].(string); ok {
		return w.ingestFromSource(ctx, source, input)
	}

	return nil, fmt.Errorf("no valid ingestion input found; expected 'texts', 'documents', or 'source'")
}

// ingestTexts ingests raw text strings
func (w *IngestionWorker) ingestTexts(ctx context.Context, textsRaw []interface{}, metadatasRaw interface{}) (interface{}, error) {
	texts := make([]string, 0, len(textsRaw))
	for _, t := range textsRaw {
		if text, ok := t.(string); ok {
			texts = append(texts, text)
		}
	}

	if len(texts) == 0 {
		return nil, fmt.Errorf("no valid texts to ingest")
	}

	// Convert to documents
	docs := make([]vectorstore.Document, len(texts))
	for i, text := range texts {
		docs[i] = vectorstore.NewDocument(text)
	}

	// Apply metadata if provided
	if metadatas, ok := metadatasRaw.([]interface{}); ok {
		for i, m := range metadatas {
			if i < len(docs) {
				if metadata, ok := m.(map[string]interface{}); ok {
					for k, v := range metadata {
						docs[i] = docs[i].WithMetadata(k, v)
					}
				}
			}
		}
	}

	// Split documents
	splitDocs := w.splitter.SplitDocuments(docs)

	// Add to vector store
	ids, err := w.vectorStore.AddDocuments(ctx, splitDocs)
	if err != nil {
		return nil, fmt.Errorf("failed to add documents: %w", err)
	}

	return map[string]interface{}{
		"ingested_count": len(ids),
		"chunk_count":    len(splitDocs),
		"document_ids":   ids,
	}, nil
}

// ingestDocuments ingests pre-formatted documents
func (w *IngestionWorker) ingestDocuments(ctx context.Context, docsRaw []interface{}) (interface{}, error) {
	docs := make([]vectorstore.Document, 0, len(docsRaw))

	for _, d := range docsRaw {
		docMap, ok := d.(map[string]interface{})
		if !ok {
			continue
		}

		content, _ := docMap["content"].(string)
		if content == "" {
			content, _ = docMap["page_content"].(string)
		}
		if content == "" {
			content, _ = docMap["text"].(string)
		}

		if content == "" {
			continue
		}

		doc := vectorstore.NewDocument(content)

		if id, ok := docMap["id"].(string); ok {
			doc = doc.WithID(id)
		}

		if metadata, ok := docMap["metadata"].(map[string]interface{}); ok {
			for k, v := range metadata {
				doc = doc.WithMetadata(k, v)
			}
		}

		docs = append(docs, doc)
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("no valid documents to ingest")
	}

	// Split documents
	splitDocs := w.splitter.SplitDocuments(docs)

	// Add to vector store
	ids, err := w.vectorStore.AddDocuments(ctx, splitDocs)
	if err != nil {
		return nil, fmt.Errorf("failed to add documents: %w", err)
	}

	return map[string]interface{}{
		"ingested_count": len(docs),
		"chunk_count":    len(splitDocs),
		"document_ids":   ids,
	}, nil
}

// ingestFromSource ingests documents from a file or URL source
func (w *IngestionWorker) ingestFromSource(ctx context.Context, source string, input map[string]interface{}) (interface{}, error) {
	// Create a text file loader as the default
	// In a full implementation, this would detect file type and use appropriate loader
	loader := documentloader.NewTextLoader(documentloader.TextLoaderConfig{
		Path: source,
	})

	// Load documents
	docs, err := loader.LoadAndSplit(ctx, w.splitter)
	if err != nil {
		return nil, fmt.Errorf("failed to load documents from %s: %w", source, err)
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents loaded from %s", source)
	}

	// Add to vector store
	ids, err := w.vectorStore.AddDocuments(ctx, docs)
	if err != nil {
		return nil, fmt.Errorf("failed to add documents: %w", err)
	}

	return map[string]interface{}{
		"source":         source,
		"ingested_count": len(docs),
		"document_ids":   ids,
	}, nil
}

// VectorStore returns the underlying vector store
func (w *IngestionWorker) VectorStore() vectorstore.VectorStore {
	return w.vectorStore
}

// Embedder returns the underlying embedder
func (w *IngestionWorker) Embedder() embeddings.Embedder {
	return w.embedder
}
