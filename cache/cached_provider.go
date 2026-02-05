package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// CompletionRequest mirrors the LLM completion request
type CompletionRequest struct {
	Model         string
	SystemPrompt  string
	UserPrompt    string
	MaxTokens     int
	Temperature   float64
	TopP          float64
	Stop          []string
}

// CompletionResponse mirrors the LLM completion response
type CompletionResponse struct {
	Text         string
	TokensUsed   int
	FinishReason string
	Model        string
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string
	Content string
}

// ChatRequest mirrors the LLM chat request
type ChatRequest struct {
	Model        string
	Messages     []ChatMessage
	MaxTokens    int
	Temperature  float64
	TopP         float64
	Stop         []string
	SystemPrompt string
}

// ChatResponse mirrors the LLM chat response
type ChatResponse struct {
	Message      ChatMessage
	TokensUsed   int
	FinishReason string
	Model        string
}

// LLMProvider interface for LLM providers
type LLMProvider interface {
	GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
	GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	Name() string
}

// CachedProvider wraps an LLM provider with caching
type CachedProvider struct {
	provider        LLMProvider
	cache           LLMCache
	embedder        Embedder
	cacheableModels map[string]bool // Models that should be cached
	enableSemantic  bool
	similarityThreshold float64
}

// CachedProviderConfig configures the cached provider
type CachedProviderConfig struct {
	Provider            LLMProvider
	Cache               LLMCache
	Embedder            Embedder // For semantic caching
	CacheableModels     []string // Models to cache (empty = all)
	EnableSemantic      bool
	SimilarityThreshold float64 // Default: 0.95
}

// NewCachedProvider creates a new cached LLM provider
func NewCachedProvider(config CachedProviderConfig) *CachedProvider {
	cacheableModels := make(map[string]bool)
	for _, model := range config.CacheableModels {
		cacheableModels[model] = true
	}

	threshold := config.SimilarityThreshold
	if threshold == 0 {
		threshold = 0.95
	}

	return &CachedProvider{
		provider:            config.Provider,
		cache:               config.Cache,
		embedder:            config.Embedder,
		cacheableModels:     cacheableModels,
		enableSemantic:      config.EnableSemantic,
		similarityThreshold: threshold,
	}
}

// GenerateCompletion generates a completion with caching
func (p *CachedProvider) GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// Check if this model should be cached
	if !p.shouldCache(req.Model) {
		return p.provider.GenerateCompletion(ctx, req)
	}

	// Build cache key from request
	prompt := p.buildCompletionPrompt(req)

	// Try cache first
	var entry *LLMCacheEntry
	var ok bool

	if p.enableSemantic && p.embedder != nil {
		// Generate embedding for semantic search
		embedding, err := p.embedder.EmbedQuery(ctx, prompt)
		if err == nil {
			entry, ok = p.cache.GetSemantic(ctx, prompt, req.Model, embedding, p.similarityThreshold)
		}
	}

	if !ok {
		entry, ok = p.cache.Get(ctx, prompt, req.Model)
	}

	if ok && entry != nil {
		// Cache hit
		return &CompletionResponse{
			Text:         entry.Response,
			TokensUsed:   entry.TokensUsed,
			FinishReason: "cached",
			Model:        entry.Model,
		}, nil
	}

	// Cache miss - call provider
	resp, err := p.provider.GenerateCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	// Cache the response
	cacheEntry := &LLMCacheEntry{
		Prompt:     prompt,
		Response:   resp.Text,
		Model:      req.Model,
		TokensUsed: resp.TokensUsed,
		Metadata: map[string]interface{}{
			"temperature": req.Temperature,
			"max_tokens":  req.MaxTokens,
		},
	}

	// Add embedding if semantic caching is enabled
	if p.enableSemantic && p.embedder != nil {
		if embedding, err := p.embedder.EmbedQuery(ctx, prompt); err == nil {
			cacheEntry.Embedding = embedding
		}
	}

	// Store in cache (don't fail on cache errors)
	p.cache.Set(ctx, cacheEntry)

	return resp, nil
}

// GenerateChat generates a chat response with caching
func (p *CachedProvider) GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Check if this model should be cached
	if !p.shouldCache(req.Model) {
		return p.provider.GenerateChat(ctx, req)
	}

	// Build cache key from request
	prompt := p.buildChatPrompt(req)

	// Try cache first
	var entry *LLMCacheEntry
	var ok bool

	if p.enableSemantic && p.embedder != nil {
		// Generate embedding for semantic search
		embedding, err := p.embedder.EmbedQuery(ctx, prompt)
		if err == nil {
			entry, ok = p.cache.GetSemantic(ctx, prompt, req.Model, embedding, p.similarityThreshold)
		}
	}

	if !ok {
		entry, ok = p.cache.Get(ctx, prompt, req.Model)
	}

	if ok && entry != nil {
		// Cache hit
		return &ChatResponse{
			Message:      ChatMessage{Role: "assistant", Content: entry.Response},
			TokensUsed:   entry.TokensUsed,
			FinishReason: "cached",
			Model:        entry.Model,
		}, nil
	}

	// Cache miss - call provider
	resp, err := p.provider.GenerateChat(ctx, req)
	if err != nil {
		return nil, err
	}

	// Cache the response
	cacheEntry := &LLMCacheEntry{
		Prompt:     prompt,
		Response:   resp.Message.Content,
		Model:      req.Model,
		TokensUsed: resp.TokensUsed,
		Metadata: map[string]interface{}{
			"temperature":   req.Temperature,
			"max_tokens":    req.MaxTokens,
			"message_count": len(req.Messages),
		},
	}

	// Add embedding if semantic caching is enabled
	if p.enableSemantic && p.embedder != nil {
		if embedding, err := p.embedder.EmbedQuery(ctx, prompt); err == nil {
			cacheEntry.Embedding = embedding
		}
	}

	// Store in cache (don't fail on cache errors)
	p.cache.Set(ctx, cacheEntry)

	return resp, nil
}

// Name returns the provider name
func (p *CachedProvider) Name() string {
	return fmt.Sprintf("cached_%s", p.provider.Name())
}

// GetStats returns cache statistics
func (p *CachedProvider) GetStats() *CacheStats {
	return p.cache.GetStats()
}

// ClearCache clears the cache
func (p *CachedProvider) ClearCache(ctx context.Context) error {
	return p.cache.Clear(ctx)
}

// shouldCache checks if a model should be cached
func (p *CachedProvider) shouldCache(model string) bool {
	if len(p.cacheableModels) == 0 {
		return true // Cache all if no specific models configured
	}
	return p.cacheableModels[model]
}

// buildCompletionPrompt builds a cache key from a completion request
func (p *CachedProvider) buildCompletionPrompt(req *CompletionRequest) string {
	// Include system prompt and user prompt
	if req.SystemPrompt != "" {
		return fmt.Sprintf("system:%s\nuser:%s", req.SystemPrompt, req.UserPrompt)
	}
	return req.UserPrompt
}

// buildChatPrompt builds a cache key from a chat request
func (p *CachedProvider) buildChatPrompt(req *ChatRequest) string {
	// Build a string from all messages
	data := struct {
		SystemPrompt string        `json:"system,omitempty"`
		Messages     []ChatMessage `json:"messages"`
	}{
		SystemPrompt: req.SystemPrompt,
		Messages:     req.Messages,
	}

	jsonData, _ := json.Marshal(data)
	return string(jsonData)
}

// CacheKeyBuilder helps build cache keys with various options
type CacheKeyBuilder struct {
	model       string
	prompt      string
	params      map[string]interface{}
	includeHash bool
}

// NewCacheKeyBuilder creates a new cache key builder
func NewCacheKeyBuilder() *CacheKeyBuilder {
	return &CacheKeyBuilder{
		params: make(map[string]interface{}),
	}
}

// Model sets the model
func (b *CacheKeyBuilder) Model(model string) *CacheKeyBuilder {
	b.model = model
	return b
}

// Prompt sets the prompt
func (b *CacheKeyBuilder) Prompt(prompt string) *CacheKeyBuilder {
	b.prompt = prompt
	return b
}

// Param adds a parameter
func (b *CacheKeyBuilder) Param(key string, value interface{}) *CacheKeyBuilder {
	b.params[key] = value
	return b
}

// IncludeParams includes specific parameters that affect output
func (b *CacheKeyBuilder) IncludeParams(temperature float64, maxTokens int) *CacheKeyBuilder {
	// Only include temperature if it affects output (non-zero)
	if temperature > 0 {
		b.params["temperature"] = temperature
	}
	return b
}

// Build builds the cache key
func (b *CacheKeyBuilder) Build() string {
	data := map[string]interface{}{
		"model":  b.model,
		"prompt": b.prompt,
	}
	if len(b.params) > 0 {
		data["params"] = b.params
	}

	jsonData, _ := json.Marshal(data)
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:])
}

// Ensure CachedProvider implements LLMProvider
var _ LLMProvider = (*CachedProvider)(nil)
