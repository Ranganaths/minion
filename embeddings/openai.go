package embeddings

import (
	"context"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"
)

// OpenAIModel represents available OpenAI embedding models
type OpenAIModel string

const (
	// TextEmbedding3Small is the small embedding model (1536 dimensions)
	TextEmbedding3Small OpenAIModel = "text-embedding-3-small"

	// TextEmbedding3Large is the large embedding model (3072 dimensions)
	TextEmbedding3Large OpenAIModel = "text-embedding-3-large"

	// TextEmbeddingAda002 is the legacy Ada model (1536 dimensions)
	TextEmbeddingAda002 OpenAIModel = "text-embedding-ada-002"
)

// OpenAIEmbedder implements Embedder using OpenAI's API
type OpenAIEmbedder struct {
	client    *openai.Client
	model     OpenAIModel
	dimension int
	config    EmbedderConfig
}

// OpenAIEmbedderConfig configures the OpenAI embedder
type OpenAIEmbedderConfig struct {
	// APIKey is the OpenAI API key
	APIKey string

	// Model is the embedding model to use
	Model OpenAIModel

	// BaseURL is an optional custom API base URL
	BaseURL string

	// Dimension overrides the embedding dimension (for models that support it)
	Dimension int

	// BatchSize is the batch size for document embedding
	BatchSize int

	// MaxRetries is the number of retries on failure
	MaxRetries int

	// Timeout is the timeout in seconds
	Timeout int
}

// NewOpenAIEmbedder creates a new OpenAI embedder
func NewOpenAIEmbedder(cfg OpenAIEmbedderConfig) (*OpenAIEmbedder, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	model := cfg.Model
	if model == "" {
		model = TextEmbedding3Small
	}

	// Determine dimension based on model
	dimension := cfg.Dimension
	if dimension == 0 {
		switch model {
		case TextEmbedding3Small:
			dimension = 1536
		case TextEmbedding3Large:
			dimension = 3072
		case TextEmbeddingAda002:
			dimension = 1536
		default:
			dimension = 1536
		}
	}

	// Create OpenAI client config
	clientCfg := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		clientCfg.BaseURL = cfg.BaseURL
	}

	embedderCfg := DefaultEmbedderConfig()
	if cfg.BatchSize > 0 {
		embedderCfg.BatchSize = cfg.BatchSize
	}
	if cfg.MaxRetries > 0 {
		embedderCfg.MaxRetries = cfg.MaxRetries
	}
	if cfg.Timeout > 0 {
		embedderCfg.Timeout = cfg.Timeout
	}
	embedderCfg.Model = string(model)

	return &OpenAIEmbedder{
		client:    openai.NewClientWithConfig(clientCfg),
		model:     model,
		dimension: dimension,
		config:    embedderCfg,
	}, nil
}

// EmbedQuery embeds a single query text
func (e *OpenAIEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := e.EmbedDocuments(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// EmbedDocuments embeds multiple document texts
func (e *OpenAIEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Apply timeout
	if e.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(e.config.Timeout)*time.Second)
		defer cancel()
	}

	// Process in batches if needed
	if len(texts) > e.config.BatchSize {
		return e.EmbedBatch(ctx, texts, e.config.BatchSize)
	}

	// Single batch request
	return e.embedBatchRequest(ctx, texts)
}

// EmbedBatch embeds texts in configurable batches
func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string, batchSize int) ([][]float32, error) {
	if batchSize <= 0 {
		batchSize = e.config.BatchSize
	}

	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, err := e.embedBatchRequest(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d-%d error: %w", i, end, err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// embedBatchRequest makes a single batch request to the API
func (e *OpenAIEmbedder) embedBatchRequest(ctx context.Context, texts []string) ([][]float32, error) {
	var lastErr error

	for attempt := 0; attempt <= e.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt*100) * time.Millisecond):
			}
		}

		req := openai.EmbeddingRequest{
			Input: texts,
			Model: openai.EmbeddingModel(e.model),
		}

		resp, err := e.client.CreateEmbeddings(ctx, req)
		if err != nil {
			lastErr = err
			continue
		}

		// Extract embeddings
		embeddings := make([][]float32, len(resp.Data))
		for _, item := range resp.Data {
			embeddings[item.Index] = item.Embedding
		}

		return embeddings, nil
	}

	return nil, fmt.Errorf("all retries failed: %w", lastErr)
}

// Dimension returns the embedding dimension size
func (e *OpenAIEmbedder) Dimension() int {
	return e.dimension
}

// Model returns the model name
func (e *OpenAIEmbedder) Model() string {
	return string(e.model)
}
