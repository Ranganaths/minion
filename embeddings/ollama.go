package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaEmbedder implements Embedder using Ollama's API
type OllamaEmbedder struct {
	baseURL   string
	model     string
	dimension int
	config    EmbedderConfig
	client    *http.Client
}

// OllamaEmbedderConfig configures the Ollama embedder
type OllamaEmbedderConfig struct {
	// BaseURL is the Ollama server URL (default: http://localhost:11434)
	BaseURL string

	// Model is the embedding model to use (e.g., "nomic-embed-text", "all-minilm")
	Model string

	// Dimension is the embedding dimension (auto-detected if not provided)
	Dimension int

	// BatchSize is the batch size for document embedding
	BatchSize int

	// MaxRetries is the number of retries on failure
	MaxRetries int

	// Timeout is the timeout in seconds
	Timeout int
}

// ollamaEmbeddingRequest is the request body for Ollama embeddings
type ollamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// ollamaEmbeddingResponse is the response from Ollama embeddings
type ollamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// NewOllamaEmbedder creates a new Ollama embedder
func NewOllamaEmbedder(cfg OllamaEmbedderConfig) (*OllamaEmbedder, error) {
	if cfg.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
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
	embedderCfg.Model = cfg.Model

	timeout := time.Duration(embedderCfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	embedder := &OllamaEmbedder{
		baseURL:   baseURL,
		model:     cfg.Model,
		dimension: cfg.Dimension,
		config:    embedderCfg,
		client: &http.Client{
			Timeout: timeout,
		},
	}

	// Auto-detect dimension if not provided
	if embedder.dimension == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		testEmbed, err := embedder.EmbedQuery(ctx, "test")
		if err == nil && len(testEmbed) > 0 {
			embedder.dimension = len(testEmbed)
		}
	}

	return embedder, nil
}

// EmbedQuery embeds a single query text
func (e *OllamaEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	var lastErr error

	for attempt := 0; attempt <= e.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt*100) * time.Millisecond):
			}
		}

		embedding, err := e.embedSingle(ctx, text)
		if err != nil {
			lastErr = err
			continue
		}

		return embedding, nil
	}

	return nil, fmt.Errorf("all retries failed: %w", lastErr)
}

// embedSingle makes a single embedding request
func (e *OllamaEmbedder) embedSingle(ctx context.Context, text string) ([]float32, error) {
	reqBody := ollamaEmbeddingRequest{
		Model:  e.model,
		Prompt: text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := e.baseURL + "/api/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result ollamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Embedding, nil
}

// EmbedDocuments embeds multiple document texts
func (e *OllamaEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		embedding, err := e.EmbedQuery(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to embed document %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

// EmbedBatch embeds texts in configurable batches (sequential for Ollama)
func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string, batchSize int) ([][]float32, error) {
	// Ollama doesn't support batch embeddings, so we process sequentially
	return e.EmbedDocuments(ctx, texts)
}

// Dimension returns the embedding dimension size
func (e *OllamaEmbedder) Dimension() int {
	return e.dimension
}

// Model returns the model name
func (e *OllamaEmbedder) Model() string {
	return e.model
}
