# Implementation Plan: Chain Package & RAG Infrastructure

**Document Version:** 1.1
**Date:** 2025-12-20
**Status:** Ready for Implementation

---

## CRITICAL: Multi-Agent Compatibility

> **This implementation MUST NOT break the existing multi-agent system.**
>
> See `MULTIAGENT_INTEGRATION_GUIDE.md` for detailed integration requirements.

### Key Principles

1. **ADDITIVE ONLY** - New packages, no modifications to `core/multiagent/`
2. **INTERFACE COMPLIANCE** - New workers implement `TaskHandler` exactly
3. **ADAPTER PATTERN** - Use adapters for LLM interface compatibility
4. **CAPABILITY ROUTING** - Register proper capabilities for load balancer

### Forbidden Changes

Do NOT modify any files in:
- `core/multiagent/*.go`

Do NOT change these interfaces:
- `Protocol`, `TaskHandler`, `Message`, `Task`, `LLMProvider` (multi-agent)

---

## Overview

This document provides a detailed, step-by-step implementation plan for adding RAG (Retrieval-Augmented Generation) infrastructure to Minion, with the `chain` package as the central orchestration layer. The plan is organized into 7 phases (including multi-agent integration) with clear dependencies, file structures, and code specifications.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           MINION FRAMEWORK                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                      CHAIN PACKAGE (NEW)                         │    │
│  │  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌─────────────────┐  │    │
│  │  │  LLMChain │ │ RAGChain  │ │ Sequential│ │ Conversational  │  │    │
│  │  └─────┬─────┘ └─────┬─────┘ └─────┬─────┘ └────────┬────────┘  │    │
│  │        │             │             │                │           │    │
│  │        └─────────────┴─────────────┴────────────────┘           │    │
│  │                              │                                   │    │
│  └──────────────────────────────┼───────────────────────────────────┘    │
│                                 │                                        │
│  ┌──────────────────────────────┼───────────────────────────────────┐    │
│  │                              ▼                                    │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐               │    │
│  │  │  EMBEDDINGS │  │ VECTORSTORE │  │  RETRIEVER  │               │    │
│  │  │   (NEW)     │  │   (NEW)     │  │   (NEW)     │               │    │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘               │    │
│  │         │                │                │                       │    │
│  │  ┌──────┴──────┐  ┌──────┴──────┐  ┌──────┴──────┐               │    │
│  │  │ OpenAI     │  │ PgVector   │  │ VectorStore │               │    │
│  │  │ Ollama     │  │ Memory     │  │ MultiQuery  │               │    │
│  │  │ Cohere     │  │ Chroma     │  │ Ensemble    │               │    │
│  │  └────────────┘  └────────────┘  └─────────────┘               │    │
│  │                                                                   │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐               │    │
│  │  │  DOCUMENT   │  │   TEXT      │  │   OUTPUT    │               │    │
│  │  │  LOADERS    │  │  SPLITTER   │  │   PARSER    │               │    │
│  │  │   (NEW)     │  │   (NEW)     │  │   (NEW)     │               │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘               │    │
│  │                                                                   │    │
│  └───────────────────────────────────────────────────────────────────┘    │
│                                                                          │
│  ┌───────────────────────────────────────────────────────────────────┐   │
│  │                    EXISTING PACKAGES                               │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌───────────┐ ┌───────────┐  │   │
│  │  │   LLM   │ │  TOOLS  │ │  CORE   │ │OBSERVABIL.│ │RESILIENCE │  │   │
│  │  └─────────┘ └─────────┘ └─────────┘ └───────────┘ └───────────┘  │   │
│  └───────────────────────────────────────────────────────────────────┘   │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## New Package Structure

```
minion/
├── chain/                          # NEW - Core chain orchestration
│   ├── interface.go                # Chain interfaces
│   ├── base.go                     # Base chain implementation
│   ├── llm_chain.go                # LLM chain
│   ├── rag_chain.go                # RAG chain
│   ├── sequential_chain.go         # Sequential chain
│   ├── conversational_chain.go     # Conversational chain
│   ├── router_chain.go             # Router chain
│   ├── callbacks.go                # Chain callbacks
│   └── options.go                  # Chain options
│
├── embeddings/                     # NEW - Embedding providers
│   ├── interface.go                # Embedder interface
│   ├── openai.go                   # OpenAI embeddings
│   ├── ollama.go                   # Ollama embeddings
│   ├── cohere.go                   # Cohere embeddings
│   ├── huggingface.go              # HuggingFace embeddings
│   └── cache.go                    # Embedding cache
│
├── vectorstore/                    # NEW - Vector databases
│   ├── interface.go                # VectorStore interface
│   ├── document.go                 # Document types
│   ├── memory.go                   # In-memory store
│   ├── pgvector.go                 # PostgreSQL pgvector
│   ├── chroma.go                   # Chroma DB
│   ├── pinecone.go                 # Pinecone
│   ├── qdrant.go                   # Qdrant
│   └── options.go                  # Store options
│
├── retriever/                      # NEW - Retrieval strategies
│   ├── interface.go                # Retriever interface
│   ├── vectorstore.go              # VectorStore retriever
│   ├── multiquery.go               # Multi-query retriever
│   ├── selfquery.go                # Self-query retriever
│   ├── ensemble.go                 # Ensemble retriever
│   ├── contextual.go               # Contextual compression
│   └── parent_document.go          # Parent document retriever
│
├── documentloader/                 # NEW - Document loaders
│   ├── interface.go                # Loader interface
│   ├── text.go                     # Text files
│   ├── pdf.go                      # PDF files
│   ├── csv.go                      # CSV files
│   ├── html.go                     # HTML pages
│   ├── markdown.go                 # Markdown files
│   ├── json.go                     # JSON/JSONL files
│   ├── directory.go                # Directory loader
│   └── web.go                      # Web/URL loader
│
├── textsplitter/                   # NEW - Text splitting
│   ├── interface.go                # Splitter interface
│   ├── recursive.go                # Recursive character splitter
│   ├── token.go                    # Token-based splitter
│   ├── markdown.go                 # Markdown splitter
│   ├── code.go                     # Code-aware splitter
│   └── semantic.go                 # Semantic splitter
│
├── prompt/                         # NEW - Prompt templates
│   ├── interface.go                # Template interface
│   ├── template.go                 # String templates
│   ├── chat.go                     # Chat templates
│   ├── fewshot.go                  # Few-shot templates
│   └── selector.go                 # Example selectors
│
├── outputparser/                   # NEW - Output parsing
│   ├── interface.go                # Parser interface
│   ├── json.go                     # JSON parser
│   ├── structured.go               # Struct parser
│   ├── list.go                     # List parser
│   ├── regex.go                    # Regex parser
│   └── retry.go                    # Retry parser
│
├── rag/                            # NEW - RAG pipeline builder
│   ├── pipeline.go                 # Pipeline orchestration
│   ├── builder.go                  # Fluent builder
│   └── ingest.go                   # Document ingestion
│
└── llm/                            # EXISTING - Add new providers
    ├── interface.go                # (existing)
    ├── openai.go                   # (existing)
    ├── anthropic.go                # (existing)
    ├── ollama.go                   # (existing)
    ├── gemini.go                   # NEW - Google Gemini
    ├── bedrock.go                  # NEW - AWS Bedrock
    ├── cohere.go                   # NEW - Cohere
    ├── mistral.go                  # NEW - Mistral AI
    ├── groq.go                     # NEW - Groq
    └── streaming.go                # NEW - Streaming support
```

---

## Phase 1: Core Chain Package

**Duration:** Week 1-2
**Priority:** P0 (Critical)
**Dependencies:** Existing `llm` package

### 1.1 Chain Interface (`chain/interface.go`)

```go
package chain

import (
    "context"
)

// Chain is the core interface for all chains
type Chain interface {
    // Call executes the chain with inputs and returns outputs
    Call(ctx context.Context, inputs map[string]any) (map[string]any, error)

    // Stream executes the chain and streams outputs (optional)
    Stream(ctx context.Context, inputs map[string]any) (<-chan StreamEvent, error)

    // InputKeys returns the required input keys
    InputKeys() []string

    // OutputKeys returns the output keys produced
    OutputKeys() []string

    // Name returns the chain name for tracing/logging
    Name() string
}

// Runnable is a more flexible interface for chain components
type Runnable[I, O any] interface {
    Invoke(ctx context.Context, input I) (O, error)
    Stream(ctx context.Context, input I) (<-chan O, error)
    Batch(ctx context.Context, inputs []I) ([]O, error)
}

// StreamEvent represents a streaming event
type StreamEvent struct {
    Type    StreamEventType
    Content string
    Data    map[string]any
    Error   error
}

type StreamEventType string

const (
    StreamEventToken    StreamEventType = "token"
    StreamEventChunk    StreamEventType = "chunk"
    StreamEventComplete StreamEventType = "complete"
    StreamEventError    StreamEventType = "error"
)

// ChainCallback provides hooks into chain execution
type ChainCallback interface {
    OnChainStart(ctx context.Context, chainName string, inputs map[string]any)
    OnChainEnd(ctx context.Context, chainName string, outputs map[string]any)
    OnChainError(ctx context.Context, chainName string, err error)
    OnLLMStart(ctx context.Context, prompt string)
    OnLLMEnd(ctx context.Context, response string, tokens int)
    OnRetrieverStart(ctx context.Context, query string)
    OnRetrieverEnd(ctx context.Context, docs []Document)
}

// ChainOptions configures chain behavior
type ChainOptions struct {
    Callbacks   []ChainCallback
    Verbose     bool
    MaxRetries  int
    Timeout     time.Duration
    Metadata    map[string]any
}

// Document represents a document in the chain
type Document struct {
    PageContent string
    Metadata    map[string]any
}
```

### 1.2 Base Chain (`chain/base.go`)

```go
package chain

import (
    "context"
    "fmt"
    "time"

    "github.com/Ranganaths/minion/observability"
)

// BaseChain provides common functionality for all chains
type BaseChain struct {
    name      string
    callbacks []ChainCallback
    verbose   bool
    timeout   time.Duration
    metrics   *observability.Metrics
}

// NewBaseChain creates a new base chain
func NewBaseChain(name string, opts ...Option) *BaseChain {
    bc := &BaseChain{
        name:    name,
        timeout: 30 * time.Second,
    }
    for _, opt := range opts {
        opt(bc)
    }
    return bc
}

func (bc *BaseChain) Name() string {
    return bc.name
}

func (bc *BaseChain) notifyStart(ctx context.Context, inputs map[string]any) {
    for _, cb := range bc.callbacks {
        cb.OnChainStart(ctx, bc.name, inputs)
    }
}

func (bc *BaseChain) notifyEnd(ctx context.Context, outputs map[string]any) {
    for _, cb := range bc.callbacks {
        cb.OnChainEnd(ctx, bc.name, outputs)
    }
}

func (bc *BaseChain) notifyError(ctx context.Context, err error) {
    for _, cb := range bc.callbacks {
        cb.OnChainError(ctx, bc.name, err)
    }
}

// ValidateInputs checks required inputs are present
func (bc *BaseChain) ValidateInputs(inputs map[string]any, required []string) error {
    for _, key := range required {
        if _, ok := inputs[key]; !ok {
            return fmt.Errorf("missing required input: %s", key)
        }
    }
    return nil
}
```

### 1.3 LLM Chain (`chain/llm_chain.go`)

```go
package chain

import (
    "context"
    "fmt"

    "github.com/Ranganaths/minion/llm"
    "github.com/Ranganaths/minion/prompt"
    "github.com/Ranganaths/minion/outputparser"
)

// LLMChain is a simple prompt -> LLM -> output chain
type LLMChain struct {
    *BaseChain
    llm          llm.Provider
    prompt       prompt.Template
    outputParser outputparser.Parser[any]
    outputKey    string
}

// LLMChainConfig configures the LLM chain
type LLMChainConfig struct {
    LLM          llm.Provider
    Prompt       prompt.Template
    OutputParser outputparser.Parser[any]
    OutputKey    string
    Options      []Option
}

// NewLLMChain creates a new LLM chain
func NewLLMChain(cfg LLMChainConfig) *LLMChain {
    outputKey := cfg.OutputKey
    if outputKey == "" {
        outputKey = "text"
    }

    return &LLMChain{
        BaseChain:    NewBaseChain("llm_chain", cfg.Options...),
        llm:          cfg.LLM,
        prompt:       cfg.Prompt,
        outputParser: cfg.OutputParser,
        outputKey:    outputKey,
    }
}

func (c *LLMChain) InputKeys() []string {
    return c.prompt.InputVariables()
}

func (c *LLMChain) OutputKeys() []string {
    return []string{c.outputKey}
}

func (c *LLMChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
    c.notifyStart(ctx, inputs)

    // Validate inputs
    if err := c.ValidateInputs(inputs, c.InputKeys()); err != nil {
        c.notifyError(ctx, err)
        return nil, err
    }

    // Format prompt
    promptText, err := c.prompt.Format(inputs)
    if err != nil {
        c.notifyError(ctx, fmt.Errorf("prompt format error: %w", err))
        return nil, err
    }

    // Call LLM
    resp, err := c.llm.GenerateCompletion(ctx, &llm.CompletionRequest{
        UserPrompt: promptText,
    })
    if err != nil {
        c.notifyError(ctx, fmt.Errorf("llm error: %w", err))
        return nil, err
    }

    // Parse output if parser provided
    var result any = resp.Text
    if c.outputParser != nil {
        parsed, err := c.outputParser.Parse(resp.Text)
        if err != nil {
            c.notifyError(ctx, fmt.Errorf("parse error: %w", err))
            return nil, err
        }
        result = parsed
    }

    outputs := map[string]any{
        c.outputKey: result,
    }
    c.notifyEnd(ctx, outputs)
    return outputs, nil
}

func (c *LLMChain) Stream(ctx context.Context, inputs map[string]any) (<-chan StreamEvent, error) {
    // Implementation for streaming
    ch := make(chan StreamEvent)
    go func() {
        defer close(ch)
        result, err := c.Call(ctx, inputs)
        if err != nil {
            ch <- StreamEvent{Type: StreamEventError, Error: err}
            return
        }
        ch <- StreamEvent{Type: StreamEventComplete, Data: result}
    }()
    return ch, nil
}
```

### 1.4 Sequential Chain (`chain/sequential_chain.go`)

```go
package chain

import (
    "context"
    "fmt"
)

// SequentialChain executes chains in sequence
type SequentialChain struct {
    *BaseChain
    chains     []Chain
    inputKeys  []string
    outputKeys []string
}

// SequentialChainConfig configures the sequential chain
type SequentialChainConfig struct {
    Chains     []Chain
    InputKeys  []string  // Override auto-detection
    OutputKeys []string  // Override auto-detection
    Options    []Option
}

// NewSequentialChain creates a new sequential chain
func NewSequentialChain(cfg SequentialChainConfig) *SequentialChain {
    return &SequentialChain{
        BaseChain:  NewBaseChain("sequential_chain", cfg.Options...),
        chains:     cfg.Chains,
        inputKeys:  cfg.InputKeys,
        outputKeys: cfg.OutputKeys,
    }
}

func (c *SequentialChain) InputKeys() []string {
    if len(c.inputKeys) > 0 {
        return c.inputKeys
    }
    if len(c.chains) > 0 {
        return c.chains[0].InputKeys()
    }
    return nil
}

func (c *SequentialChain) OutputKeys() []string {
    if len(c.outputKeys) > 0 {
        return c.outputKeys
    }
    if len(c.chains) > 0 {
        return c.chains[len(c.chains)-1].OutputKeys()
    }
    return nil
}

func (c *SequentialChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
    c.notifyStart(ctx, inputs)

    // Accumulate all outputs
    allOutputs := make(map[string]any)
    for k, v := range inputs {
        allOutputs[k] = v
    }

    // Execute each chain
    for i, chain := range c.chains {
        outputs, err := chain.Call(ctx, allOutputs)
        if err != nil {
            c.notifyError(ctx, fmt.Errorf("chain %d (%s) error: %w", i, chain.Name(), err))
            return nil, err
        }
        // Merge outputs
        for k, v := range outputs {
            allOutputs[k] = v
        }
    }

    // Filter to requested output keys
    result := make(map[string]any)
    for _, key := range c.OutputKeys() {
        if v, ok := allOutputs[key]; ok {
            result[key] = v
        }
    }

    c.notifyEnd(ctx, result)
    return result, nil
}

func (c *SequentialChain) Stream(ctx context.Context, inputs map[string]any) (<-chan StreamEvent, error) {
    ch := make(chan StreamEvent)
    go func() {
        defer close(ch)
        result, err := c.Call(ctx, inputs)
        if err != nil {
            ch <- StreamEvent{Type: StreamEventError, Error: err}
            return
        }
        ch <- StreamEvent{Type: StreamEventComplete, Data: result}
    }()
    return ch, nil
}
```

### 1.5 Implementation Tasks

| Task | File | Effort | Dependencies |
|------|------|--------|--------------|
| 1.1 | `chain/interface.go` | 2h | None |
| 1.2 | `chain/base.go` | 3h | 1.1 |
| 1.3 | `chain/options.go` | 1h | 1.1 |
| 1.4 | `chain/callbacks.go` | 2h | 1.1 |
| 1.5 | `chain/llm_chain.go` | 4h | 1.2, prompt pkg |
| 1.6 | `chain/sequential_chain.go` | 3h | 1.2 |
| 1.7 | `chain/router_chain.go` | 4h | 1.2, 1.5 |
| 1.8 | Unit tests | 6h | All above |

**Phase 1 Total: 25 hours**

---

## Phase 2: Embeddings Package

**Duration:** Week 2
**Priority:** P0 (Critical)
**Dependencies:** None

### 2.1 Embeddings Interface (`embeddings/interface.go`)

```go
package embeddings

import (
    "context"
)

// Embedder generates embeddings for text
type Embedder interface {
    // EmbedDocuments embeds multiple documents
    EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)

    // EmbedQuery embeds a single query (may use different strategy)
    EmbedQuery(ctx context.Context, text string) ([]float32, error)

    // Dimensions returns the embedding dimension size
    Dimensions() int

    // Name returns the embedder name
    Name() string
}

// EmbedderOption configures an embedder
type EmbedderOption func(*EmbedderConfig)

// EmbedderConfig holds embedder configuration
type EmbedderConfig struct {
    Model       string
    Dimensions  int
    BatchSize   int
    MaxRetries  int
    Timeout     time.Duration
    APIKey      string
    BaseURL     string
    EnableCache bool
    CacheTTL    time.Duration
}

// EmbeddingResult contains embedding with metadata
type EmbeddingResult struct {
    Embedding  []float32
    Index      int
    TokenCount int
}

// BatchResult contains results from batch embedding
type BatchResult struct {
    Embeddings []EmbeddingResult
    TotalTokens int
}

// WithModel sets the embedding model
func WithModel(model string) EmbedderOption {
    return func(c *EmbedderConfig) {
        c.Model = model
    }
}

// WithDimensions sets output dimensions (if supported)
func WithDimensions(dims int) EmbedderOption {
    return func(c *EmbedderConfig) {
        c.Dimensions = dims
    }
}

// WithBatchSize sets the batch size
func WithBatchSize(size int) EmbedderOption {
    return func(c *EmbedderConfig) {
        c.BatchSize = size
    }
}

// WithCache enables embedding caching
func WithCache(ttl time.Duration) EmbedderOption {
    return func(c *EmbedderConfig) {
        c.EnableCache = true
        c.CacheTTL = ttl
    }
}
```

### 2.2 OpenAI Embeddings (`embeddings/openai.go`)

```go
package embeddings

import (
    "context"
    "fmt"

    "github.com/sashabaranov/go-openai"
)

const (
    OpenAIModelSmall = "text-embedding-3-small"
    OpenAIModelLarge = "text-embedding-3-large"
    OpenAIModelAda   = "text-embedding-ada-002"
)

// OpenAIEmbedder implements embeddings using OpenAI
type OpenAIEmbedder struct {
    client     *openai.Client
    config     EmbedderConfig
    cache      *EmbeddingCache
}

// NewOpenAI creates a new OpenAI embedder
func NewOpenAI(apiKey string, opts ...EmbedderOption) *OpenAIEmbedder {
    cfg := EmbedderConfig{
        Model:      OpenAIModelSmall,
        Dimensions: 1536,
        BatchSize:  512,
        MaxRetries: 3,
        Timeout:    30 * time.Second,
    }
    for _, opt := range opts {
        opt(&cfg)
    }

    client := openai.NewClient(apiKey)

    e := &OpenAIEmbedder{
        client: client,
        config: cfg,
    }

    if cfg.EnableCache {
        e.cache = NewEmbeddingCache(cfg.CacheTTL)
    }

    return e
}

func (e *OpenAIEmbedder) Name() string {
    return "openai"
}

func (e *OpenAIEmbedder) Dimensions() int {
    return e.config.Dimensions
}

func (e *OpenAIEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
    if len(texts) == 0 {
        return nil, nil
    }

    // Check cache first
    results := make([][]float32, len(texts))
    uncachedIndices := make([]int, 0)
    uncachedTexts := make([]string, 0)

    if e.cache != nil {
        for i, text := range texts {
            if cached, ok := e.cache.Get(text); ok {
                results[i] = cached
            } else {
                uncachedIndices = append(uncachedIndices, i)
                uncachedTexts = append(uncachedTexts, text)
            }
        }
    } else {
        uncachedIndices = make([]int, len(texts))
        for i := range texts {
            uncachedIndices[i] = i
        }
        uncachedTexts = texts
    }

    // Embed uncached texts in batches
    for start := 0; start < len(uncachedTexts); start += e.config.BatchSize {
        end := start + e.config.BatchSize
        if end > len(uncachedTexts) {
            end = len(uncachedTexts)
        }

        batch := uncachedTexts[start:end]
        embeddings, err := e.embedBatch(ctx, batch)
        if err != nil {
            return nil, err
        }

        for i, emb := range embeddings {
            idx := uncachedIndices[start+i]
            results[idx] = emb
            if e.cache != nil {
                e.cache.Set(batch[i], emb)
            }
        }
    }

    return results, nil
}

func (e *OpenAIEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
    results, err := e.EmbedDocuments(ctx, []string{text})
    if err != nil {
        return nil, err
    }
    if len(results) == 0 {
        return nil, fmt.Errorf("no embedding returned")
    }
    return results[0], nil
}

func (e *OpenAIEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
    req := openai.EmbeddingRequest{
        Input: texts,
        Model: openai.EmbeddingModel(e.config.Model),
    }

    // Set dimensions if using embedding-3 models
    if e.config.Model == OpenAIModelSmall || e.config.Model == OpenAIModelLarge {
        req.Dimensions = e.config.Dimensions
    }

    resp, err := e.client.CreateEmbeddings(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("openai embedding error: %w", err)
    }

    results := make([][]float32, len(resp.Data))
    for i, data := range resp.Data {
        results[i] = data.Embedding
    }

    return results, nil
}
```

### 2.3 Ollama Embeddings (`embeddings/ollama.go`)

```go
package embeddings

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
)

const (
    OllamaModelNomic    = "nomic-embed-text"
    OllamaModelMxbai    = "mxbai-embed-large"
    OllamaModelAllMiniLM = "all-minilm"
)

// OllamaEmbedder implements embeddings using Ollama
type OllamaEmbedder struct {
    baseURL    string
    model      string
    dimensions int
    client     *http.Client
}

// NewOllama creates a new Ollama embedder
func NewOllama(opts ...EmbedderOption) *OllamaEmbedder {
    cfg := EmbedderConfig{
        BaseURL:    "http://localhost:11434",
        Model:      OllamaModelNomic,
        Dimensions: 768,
        Timeout:    60 * time.Second,
    }
    for _, opt := range opts {
        opt(&cfg)
    }

    return &OllamaEmbedder{
        baseURL:    cfg.BaseURL,
        model:      cfg.Model,
        dimensions: cfg.Dimensions,
        client:     &http.Client{Timeout: cfg.Timeout},
    }
}

func (e *OllamaEmbedder) Name() string {
    return "ollama"
}

func (e *OllamaEmbedder) Dimensions() int {
    return e.dimensions
}

type ollamaEmbedRequest struct {
    Model  string `json:"model"`
    Prompt string `json:"prompt"`
}

type ollamaEmbedResponse struct {
    Embedding []float32 `json:"embedding"`
}

func (e *OllamaEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
    results := make([][]float32, len(texts))
    for i, text := range texts {
        emb, err := e.embed(ctx, text)
        if err != nil {
            return nil, fmt.Errorf("embedding text %d: %w", i, err)
        }
        results[i] = emb
    }
    return results, nil
}

func (e *OllamaEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
    return e.embed(ctx, text)
}

func (e *OllamaEmbedder) embed(ctx context.Context, text string) ([]float32, error) {
    reqBody := ollamaEmbedRequest{
        Model:  e.model,
        Prompt: text,
    }

    body, err := json.Marshal(reqBody)
    if err != nil {
        return nil, err
    }

    req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/api/embeddings", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := e.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("ollama error: status %d", resp.StatusCode)
    }

    var result ollamaEmbedResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return result.Embedding, nil
}
```

### 2.4 Embedding Cache (`embeddings/cache.go`)

```go
package embeddings

import (
    "crypto/sha256"
    "encoding/hex"
    "sync"
    "time"
)

// EmbeddingCache caches embeddings to avoid redundant API calls
type EmbeddingCache struct {
    mu      sync.RWMutex
    entries map[string]*cacheEntry
    ttl     time.Duration
}

type cacheEntry struct {
    embedding []float32
    expiresAt time.Time
}

// NewEmbeddingCache creates a new embedding cache
func NewEmbeddingCache(ttl time.Duration) *EmbeddingCache {
    c := &EmbeddingCache{
        entries: make(map[string]*cacheEntry),
        ttl:     ttl,
    }
    go c.cleanup()
    return c
}

func (c *EmbeddingCache) hash(text string) string {
    h := sha256.Sum256([]byte(text))
    return hex.EncodeToString(h[:])
}

func (c *EmbeddingCache) Get(text string) ([]float32, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    key := c.hash(text)
    entry, ok := c.entries[key]
    if !ok || time.Now().After(entry.expiresAt) {
        return nil, false
    }
    return entry.embedding, true
}

func (c *EmbeddingCache) Set(text string, embedding []float32) {
    c.mu.Lock()
    defer c.mu.Unlock()

    key := c.hash(text)
    c.entries[key] = &cacheEntry{
        embedding: embedding,
        expiresAt: time.Now().Add(c.ttl),
    }
}

func (c *EmbeddingCache) cleanup() {
    ticker := time.NewTicker(c.ttl / 2)
    for range ticker.C {
        c.mu.Lock()
        now := time.Now()
        for key, entry := range c.entries {
            if now.After(entry.expiresAt) {
                delete(c.entries, key)
            }
        }
        c.mu.Unlock()
    }
}
```

### 2.5 Implementation Tasks

| Task | File | Effort | Dependencies |
|------|------|--------|--------------|
| 2.1 | `embeddings/interface.go` | 2h | None |
| 2.2 | `embeddings/openai.go` | 4h | 2.1 |
| 2.3 | `embeddings/ollama.go` | 3h | 2.1 |
| 2.4 | `embeddings/cohere.go` | 3h | 2.1 |
| 2.5 | `embeddings/huggingface.go` | 4h | 2.1 |
| 2.6 | `embeddings/cache.go` | 2h | 2.1 |
| 2.7 | Unit tests | 4h | All above |

**Phase 2 Total: 22 hours**

---

## Phase 3: Vector Store Package

**Duration:** Week 3-4
**Priority:** P0 (Critical)
**Dependencies:** Phase 2 (Embeddings)

### 3.1 Vector Store Interface (`vectorstore/interface.go`)

```go
package vectorstore

import (
    "context"

    "github.com/Ranganaths/minion/embeddings"
)

// VectorStore is the interface for vector databases
type VectorStore interface {
    // AddDocuments adds documents with embeddings
    AddDocuments(ctx context.Context, docs []Document, embedder embeddings.Embedder) ([]string, error)

    // AddDocumentsWithEmbeddings adds pre-embedded documents
    AddDocumentsWithEmbeddings(ctx context.Context, docs []Document, embeddings [][]float32) ([]string, error)

    // SimilaritySearch finds similar documents
    SimilaritySearch(ctx context.Context, query string, k int, opts ...SearchOption) ([]Document, error)

    // SimilaritySearchWithScore includes similarity scores
    SimilaritySearchWithScore(ctx context.Context, query string, k int, opts ...SearchOption) ([]ScoredDocument, error)

    // SimilaritySearchByVector searches using a pre-computed vector
    SimilaritySearchByVector(ctx context.Context, vector []float32, k int, opts ...SearchOption) ([]ScoredDocument, error)

    // Delete removes documents by IDs
    Delete(ctx context.Context, ids []string) error

    // DeleteCollection removes entire collection
    DeleteCollection(ctx context.Context) error
}

// Document represents a document in the store
type Document struct {
    ID          string
    PageContent string
    Metadata    map[string]any
    Embedding   []float32 // Optional pre-computed embedding
}

// ScoredDocument includes similarity score
type ScoredDocument struct {
    Document
    Score float32
}

// SearchOptions configures similarity search
type SearchOptions struct {
    ScoreThreshold float32
    Filter         map[string]any
    Namespace      string
    IncludeVectors bool
}

// SearchOption is a function that modifies SearchOptions
type SearchOption func(*SearchOptions)

// WithScoreThreshold sets minimum similarity score
func WithScoreThreshold(threshold float32) SearchOption {
    return func(o *SearchOptions) {
        o.ScoreThreshold = threshold
    }
}

// WithFilter sets metadata filter
func WithFilter(filter map[string]any) SearchOption {
    return func(o *SearchOptions) {
        o.Filter = filter
    }
}

// WithNamespace sets the namespace/collection
func WithNamespace(ns string) SearchOption {
    return func(o *SearchOptions) {
        o.Namespace = ns
    }
}

// VectorStoreRetriever wraps VectorStore as a Retriever
type VectorStoreRetriever struct {
    store    VectorStore
    embedder embeddings.Embedder
    k        int
    options  []SearchOption
}

// AsRetriever converts VectorStore to Retriever interface
func (vs *VectorStoreRetriever) GetRelevantDocuments(ctx context.Context, query string) ([]Document, error) {
    return vs.store.SimilaritySearch(ctx, query, vs.k, vs.options...)
}
```

### 3.2 In-Memory Store (`vectorstore/memory.go`)

```go
package vectorstore

import (
    "context"
    "fmt"
    "math"
    "sort"
    "sync"

    "github.com/google/uuid"
    "github.com/Ranganaths/minion/embeddings"
)

// MemoryVectorStore is an in-memory vector store
type MemoryVectorStore struct {
    mu        sync.RWMutex
    documents map[string]Document
    embedder  embeddings.Embedder
}

// NewMemory creates a new in-memory vector store
func NewMemory(embedder embeddings.Embedder) *MemoryVectorStore {
    return &MemoryVectorStore{
        documents: make(map[string]Document),
        embedder:  embedder,
    }
}

func (m *MemoryVectorStore) AddDocuments(ctx context.Context, docs []Document, embedder embeddings.Embedder) ([]string, error) {
    texts := make([]string, len(docs))
    for i, doc := range docs {
        texts[i] = doc.PageContent
    }

    embeddings, err := embedder.EmbedDocuments(ctx, texts)
    if err != nil {
        return nil, err
    }

    return m.AddDocumentsWithEmbeddings(ctx, docs, embeddings)
}

func (m *MemoryVectorStore) AddDocumentsWithEmbeddings(ctx context.Context, docs []Document, embs [][]float32) ([]string, error) {
    if len(docs) != len(embs) {
        return nil, fmt.Errorf("documents and embeddings count mismatch")
    }

    m.mu.Lock()
    defer m.mu.Unlock()

    ids := make([]string, len(docs))
    for i, doc := range docs {
        id := doc.ID
        if id == "" {
            id = uuid.New().String()
        }
        ids[i] = id

        doc.ID = id
        doc.Embedding = embs[i]
        m.documents[id] = doc
    }

    return ids, nil
}

func (m *MemoryVectorStore) SimilaritySearch(ctx context.Context, query string, k int, opts ...SearchOption) ([]Document, error) {
    scored, err := m.SimilaritySearchWithScore(ctx, query, k, opts...)
    if err != nil {
        return nil, err
    }

    docs := make([]Document, len(scored))
    for i, sd := range scored {
        docs[i] = sd.Document
    }
    return docs, nil
}

func (m *MemoryVectorStore) SimilaritySearchWithScore(ctx context.Context, query string, k int, opts ...SearchOption) ([]ScoredDocument, error) {
    queryEmb, err := m.embedder.EmbedQuery(ctx, query)
    if err != nil {
        return nil, err
    }
    return m.SimilaritySearchByVector(ctx, queryEmb, k, opts...)
}

func (m *MemoryVectorStore) SimilaritySearchByVector(ctx context.Context, vector []float32, k int, opts ...SearchOption) ([]ScoredDocument, error) {
    options := &SearchOptions{}
    for _, opt := range opts {
        opt(options)
    }

    m.mu.RLock()
    defer m.mu.RUnlock()

    scored := make([]ScoredDocument, 0, len(m.documents))
    for _, doc := range m.documents {
        // Apply metadata filter
        if options.Filter != nil && !matchesFilter(doc.Metadata, options.Filter) {
            continue
        }

        score := cosineSimilarity(vector, doc.Embedding)
        if options.ScoreThreshold > 0 && score < options.ScoreThreshold {
            continue
        }

        scored = append(scored, ScoredDocument{
            Document: doc,
            Score:    score,
        })
    }

    // Sort by score descending
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].Score > scored[j].Score
    })

    if k > 0 && len(scored) > k {
        scored = scored[:k]
    }

    return scored, nil
}

func (m *MemoryVectorStore) Delete(ctx context.Context, ids []string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    for _, id := range ids {
        delete(m.documents, id)
    }
    return nil
}

func (m *MemoryVectorStore) DeleteCollection(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.documents = make(map[string]Document)
    return nil
}

// cosineSimilarity calculates cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
    if len(a) != len(b) {
        return 0
    }

    var dotProduct, normA, normB float64
    for i := range a {
        dotProduct += float64(a[i]) * float64(b[i])
        normA += float64(a[i]) * float64(a[i])
        normB += float64(b[i]) * float64(b[i])
    }

    if normA == 0 || normB == 0 {
        return 0
    }

    return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// matchesFilter checks if metadata matches filter criteria
func matchesFilter(metadata, filter map[string]any) bool {
    for key, value := range filter {
        if metadata[key] != value {
            return false
        }
    }
    return true
}
```

### 3.3 PgVector Store (`vectorstore/pgvector.go`)

```go
package vectorstore

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "strings"

    "github.com/google/uuid"
    "github.com/lib/pq"
    "github.com/Ranganaths/minion/embeddings"
)

// PgVectorStore implements VectorStore using PostgreSQL with pgvector
type PgVectorStore struct {
    db         *sql.DB
    embedder   embeddings.Embedder
    collection string
    dimensions int
}

// PgVectorConfig configures the PgVector store
type PgVectorConfig struct {
    DB         *sql.DB
    Embedder   embeddings.Embedder
    Collection string
    Dimensions int
}

// NewPgVector creates a new PgVector store
func NewPgVector(cfg PgVectorConfig) (*PgVectorStore, error) {
    store := &PgVectorStore{
        db:         cfg.DB,
        embedder:   cfg.Embedder,
        collection: cfg.Collection,
        dimensions: cfg.Dimensions,
    }

    if err := store.ensureTable(); err != nil {
        return nil, err
    }

    return store, nil
}

func (p *PgVectorStore) ensureTable() error {
    query := fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS minion_embeddings_%s (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            content TEXT NOT NULL,
            metadata JSONB DEFAULT '{}',
            embedding vector(%d),
            created_at TIMESTAMPTZ DEFAULT NOW()
        );

        CREATE INDEX IF NOT EXISTS idx_%s_embedding
        ON minion_embeddings_%s
        USING hnsw (embedding vector_cosine_ops);

        CREATE INDEX IF NOT EXISTS idx_%s_metadata
        ON minion_embeddings_%s
        USING gin (metadata);
    `, p.collection, p.dimensions, p.collection, p.collection, p.collection, p.collection)

    _, err := p.db.Exec(query)
    return err
}

func (p *PgVectorStore) AddDocuments(ctx context.Context, docs []Document, embedder embeddings.Embedder) ([]string, error) {
    texts := make([]string, len(docs))
    for i, doc := range docs {
        texts[i] = doc.PageContent
    }

    embs, err := embedder.EmbedDocuments(ctx, texts)
    if err != nil {
        return nil, err
    }

    return p.AddDocumentsWithEmbeddings(ctx, docs, embs)
}

func (p *PgVectorStore) AddDocumentsWithEmbeddings(ctx context.Context, docs []Document, embs [][]float32) ([]string, error) {
    if len(docs) != len(embs) {
        return nil, fmt.Errorf("documents and embeddings count mismatch")
    }

    tx, err := p.db.BeginTx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    query := fmt.Sprintf(`
        INSERT INTO minion_embeddings_%s (id, content, metadata, embedding)
        VALUES ($1, $2, $3, $4)
    `, p.collection)

    stmt, err := tx.PrepareContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer stmt.Close()

    ids := make([]string, len(docs))
    for i, doc := range docs {
        id := doc.ID
        if id == "" {
            id = uuid.New().String()
        }
        ids[i] = id

        metadata, _ := json.Marshal(doc.Metadata)
        embedding := vectorToString(embs[i])

        _, err := stmt.ExecContext(ctx, id, doc.PageContent, metadata, embedding)
        if err != nil {
            return nil, err
        }
    }

    if err := tx.Commit(); err != nil {
        return nil, err
    }

    return ids, nil
}

func (p *PgVectorStore) SimilaritySearch(ctx context.Context, query string, k int, opts ...SearchOption) ([]Document, error) {
    scored, err := p.SimilaritySearchWithScore(ctx, query, k, opts...)
    if err != nil {
        return nil, err
    }

    docs := make([]Document, len(scored))
    for i, sd := range scored {
        docs[i] = sd.Document
    }
    return docs, nil
}

func (p *PgVectorStore) SimilaritySearchWithScore(ctx context.Context, query string, k int, opts ...SearchOption) ([]ScoredDocument, error) {
    queryEmb, err := p.embedder.EmbedQuery(ctx, query)
    if err != nil {
        return nil, err
    }
    return p.SimilaritySearchByVector(ctx, queryEmb, k, opts...)
}

func (p *PgVectorStore) SimilaritySearchByVector(ctx context.Context, vector []float32, k int, opts ...SearchOption) ([]ScoredDocument, error) {
    options := &SearchOptions{}
    for _, opt := range opts {
        opt(options)
    }

    embStr := vectorToString(vector)

    whereClause := ""
    args := []any{embStr, k}

    if options.Filter != nil {
        conditions := make([]string, 0)
        argIdx := 3
        for key, value := range options.Filter {
            conditions = append(conditions, fmt.Sprintf("metadata->>'%s' = $%d", key, argIdx))
            args = append(args, value)
            argIdx++
        }
        if len(conditions) > 0 {
            whereClause = "WHERE " + strings.Join(conditions, " AND ")
        }
    }

    query := fmt.Sprintf(`
        SELECT id, content, metadata, 1 - (embedding <=> $1) as score
        FROM minion_embeddings_%s
        %s
        ORDER BY embedding <=> $1
        LIMIT $2
    `, p.collection, whereClause)

    rows, err := p.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var results []ScoredDocument
    for rows.Next() {
        var id, content string
        var metadataJSON []byte
        var score float32

        if err := rows.Scan(&id, &content, &metadataJSON, &score); err != nil {
            return nil, err
        }

        if options.ScoreThreshold > 0 && score < options.ScoreThreshold {
            continue
        }

        var metadata map[string]any
        json.Unmarshal(metadataJSON, &metadata)

        results = append(results, ScoredDocument{
            Document: Document{
                ID:          id,
                PageContent: content,
                Metadata:    metadata,
            },
            Score: score,
        })
    }

    return results, nil
}

func (p *PgVectorStore) Delete(ctx context.Context, ids []string) error {
    query := fmt.Sprintf(`DELETE FROM minion_embeddings_%s WHERE id = ANY($1)`, p.collection)
    _, err := p.db.ExecContext(ctx, query, pq.Array(ids))
    return err
}

func (p *PgVectorStore) DeleteCollection(ctx context.Context) error {
    query := fmt.Sprintf(`DROP TABLE IF EXISTS minion_embeddings_%s`, p.collection)
    _, err := p.db.ExecContext(ctx, query)
    return err
}

func vectorToString(v []float32) string {
    strs := make([]string, len(v))
    for i, f := range v {
        strs[i] = fmt.Sprintf("%f", f)
    }
    return "[" + strings.Join(strs, ",") + "]"
}
```

### 3.4 Implementation Tasks

| Task | File | Effort | Dependencies |
|------|------|--------|--------------|
| 3.1 | `vectorstore/interface.go` | 2h | None |
| 3.2 | `vectorstore/document.go` | 1h | 3.1 |
| 3.3 | `vectorstore/memory.go` | 4h | 3.1, Phase 2 |
| 3.4 | `vectorstore/pgvector.go` | 8h | 3.1, Phase 2 |
| 3.5 | `vectorstore/chroma.go` | 6h | 3.1 |
| 3.6 | `vectorstore/pinecone.go` | 6h | 3.1 |
| 3.7 | `vectorstore/qdrant.go` | 6h | 3.1 |
| 3.8 | `vectorstore/options.go` | 1h | 3.1 |
| 3.9 | Unit & integration tests | 8h | All above |

**Phase 3 Total: 42 hours**

---

## Phase 4: Document Loaders & Text Splitters

**Duration:** Week 4-5
**Priority:** P0 (Critical)
**Dependencies:** Phase 3 (for Document type)

### 4.1 Document Loader Interface (`documentloader/interface.go`)

```go
package documentloader

import (
    "context"
    "io"

    "github.com/Ranganaths/minion/textsplitter"
    "github.com/Ranganaths/minion/vectorstore"
)

// Loader loads documents from a source
type Loader interface {
    // Load returns all documents from the source
    Load(ctx context.Context) ([]vectorstore.Document, error)

    // LoadAndSplit loads and splits documents
    LoadAndSplit(ctx context.Context, splitter textsplitter.Splitter) ([]vectorstore.Document, error)
}

// ReaderLoader loads from an io.Reader
type ReaderLoader interface {
    Loader
    // LoadFromReader loads from a reader
    LoadFromReader(ctx context.Context, r io.Reader) ([]vectorstore.Document, error)
}
```

### 4.2 Text Splitter Interface (`textsplitter/interface.go`)

```go
package textsplitter

import (
    "github.com/Ranganaths/minion/vectorstore"
)

// Splitter splits text into chunks
type Splitter interface {
    // SplitText splits text into chunks
    SplitText(text string) ([]string, error)

    // SplitDocuments splits documents preserving metadata
    SplitDocuments(docs []vectorstore.Document) ([]vectorstore.Document, error)
}

// SplitterConfig configures a text splitter
type SplitterConfig struct {
    ChunkSize      int
    ChunkOverlap   int
    Separators     []string
    LengthFunction func(string) int
    KeepSeparator  bool
}

// DefaultConfig returns default splitter configuration
func DefaultConfig() SplitterConfig {
    return SplitterConfig{
        ChunkSize:      1000,
        ChunkOverlap:   200,
        Separators:     []string{"\n\n", "\n", " ", ""},
        LengthFunction: func(s string) int { return len(s) },
        KeepSeparator:  false,
    }
}
```

### 4.3 Recursive Character Splitter (`textsplitter/recursive.go`)

```go
package textsplitter

import (
    "strings"

    "github.com/Ranganaths/minion/vectorstore"
)

// RecursiveCharacterSplitter splits text recursively by separators
type RecursiveCharacterSplitter struct {
    config SplitterConfig
}

// NewRecursive creates a new recursive character splitter
func NewRecursive(opts ...Option) *RecursiveCharacterSplitter {
    cfg := DefaultConfig()
    for _, opt := range opts {
        opt(&cfg)
    }
    return &RecursiveCharacterSplitter{config: cfg}
}

func (s *RecursiveCharacterSplitter) SplitText(text string) ([]string, error) {
    return s.splitText(text, s.config.Separators)
}

func (s *RecursiveCharacterSplitter) splitText(text string, separators []string) ([]string, error) {
    var chunks []string

    // Find the appropriate separator
    separator := separators[len(separators)-1]
    newSeparators := []string{}

    for i, sep := range separators {
        if sep == "" || strings.Contains(text, sep) {
            separator = sep
            newSeparators = separators[i+1:]
            break
        }
    }

    // Split by separator
    var splits []string
    if separator != "" {
        splits = strings.Split(text, separator)
    } else {
        splits = strings.Split(text, "")
    }

    // Merge splits into chunks
    goodSplits := []string{}
    for _, split := range splits {
        if s.config.LengthFunction(split) < s.config.ChunkSize {
            goodSplits = append(goodSplits, split)
        } else {
            if len(goodSplits) > 0 {
                merged := s.mergeSplits(goodSplits, separator)
                chunks = append(chunks, merged...)
                goodSplits = []string{}
            }
            if len(newSeparators) == 0 {
                chunks = append(chunks, split)
            } else {
                subChunks, _ := s.splitText(split, newSeparators)
                chunks = append(chunks, subChunks...)
            }
        }
    }

    if len(goodSplits) > 0 {
        merged := s.mergeSplits(goodSplits, separator)
        chunks = append(chunks, merged...)
    }

    return chunks, nil
}

func (s *RecursiveCharacterSplitter) mergeSplits(splits []string, separator string) []string {
    var chunks []string
    var current strings.Builder
    var currentLen int

    for _, split := range splits {
        splitLen := s.config.LengthFunction(split)

        if currentLen+splitLen > s.config.ChunkSize {
            if current.Len() > 0 {
                chunks = append(chunks, strings.TrimSpace(current.String()))
            }
            // Handle overlap
            current.Reset()
            currentLen = 0
        }

        if current.Len() > 0 && separator != "" {
            current.WriteString(separator)
            currentLen += len(separator)
        }
        current.WriteString(split)
        currentLen += splitLen
    }

    if current.Len() > 0 {
        chunks = append(chunks, strings.TrimSpace(current.String()))
    }

    return chunks
}

func (s *RecursiveCharacterSplitter) SplitDocuments(docs []vectorstore.Document) ([]vectorstore.Document, error) {
    var result []vectorstore.Document

    for _, doc := range docs {
        chunks, err := s.SplitText(doc.PageContent)
        if err != nil {
            return nil, err
        }

        for i, chunk := range chunks {
            newDoc := vectorstore.Document{
                PageContent: chunk,
                Metadata:    copyMetadata(doc.Metadata),
            }
            newDoc.Metadata["chunk_index"] = i
            newDoc.Metadata["source_id"] = doc.ID
            result = append(result, newDoc)
        }
    }

    return result, nil
}

func copyMetadata(m map[string]any) map[string]any {
    if m == nil {
        return make(map[string]any)
    }
    result := make(map[string]any, len(m))
    for k, v := range m {
        result[k] = v
    }
    return result
}
```

### 4.4 PDF Loader (`documentloader/pdf.go`)

```go
package documentloader

import (
    "context"
    "fmt"
    "io"
    "os"

    "github.com/ledongthuc/pdf"
    "github.com/Ranganaths/minion/textsplitter"
    "github.com/Ranganaths/minion/vectorstore"
)

// PDFLoader loads PDF documents
type PDFLoader struct {
    path     string
    password string
}

// NewPDF creates a new PDF loader
func NewPDF(path string, opts ...PDFOption) *PDFLoader {
    loader := &PDFLoader{path: path}
    for _, opt := range opts {
        opt(loader)
    }
    return loader
}

type PDFOption func(*PDFLoader)

func WithPassword(password string) PDFOption {
    return func(l *PDFLoader) {
        l.password = password
    }
}

func (l *PDFLoader) Load(ctx context.Context) ([]vectorstore.Document, error) {
    f, r, err := pdf.Open(l.path)
    if err != nil {
        return nil, fmt.Errorf("open pdf: %w", err)
    }
    defer f.Close()

    totalPages := r.NumPage()
    docs := make([]vectorstore.Document, 0, totalPages)

    for pageNum := 1; pageNum <= totalPages; pageNum++ {
        page := r.Page(pageNum)
        if page.V.IsNull() {
            continue
        }

        text, err := page.GetPlainText(nil)
        if err != nil {
            continue
        }

        docs = append(docs, vectorstore.Document{
            PageContent: text,
            Metadata: map[string]any{
                "source":      l.path,
                "page":        pageNum,
                "total_pages": totalPages,
            },
        })
    }

    return docs, nil
}

func (l *PDFLoader) LoadAndSplit(ctx context.Context, splitter textsplitter.Splitter) ([]vectorstore.Document, error) {
    docs, err := l.Load(ctx)
    if err != nil {
        return nil, err
    }
    return splitter.SplitDocuments(docs)
}
```

### 4.5 Implementation Tasks

| Task | File | Effort | Dependencies |
|------|------|--------|--------------|
| 4.1 | `textsplitter/interface.go` | 1h | None |
| 4.2 | `textsplitter/recursive.go` | 4h | 4.1 |
| 4.3 | `textsplitter/token.go` | 4h | 4.1 |
| 4.4 | `textsplitter/markdown.go` | 3h | 4.1 |
| 4.5 | `textsplitter/code.go` | 4h | 4.1 |
| 4.6 | `documentloader/interface.go` | 1h | None |
| 4.7 | `documentloader/text.go` | 2h | 4.6 |
| 4.8 | `documentloader/pdf.go` | 4h | 4.6 |
| 4.9 | `documentloader/csv.go` | 3h | 4.6 |
| 4.10 | `documentloader/html.go` | 3h | 4.6 |
| 4.11 | `documentloader/markdown.go` | 2h | 4.6 |
| 4.12 | `documentloader/json.go` | 2h | 4.6 |
| 4.13 | `documentloader/directory.go` | 4h | 4.6-4.12 |
| 4.14 | `documentloader/web.go` | 4h | 4.6, 4.10 |
| 4.15 | Unit tests | 6h | All above |

**Phase 4 Total: 47 hours**

---

## Phase 5: Retrievers & RAG Chain

**Duration:** Week 5-6
**Priority:** P0 (Critical)
**Dependencies:** Phase 3, Phase 4

### 5.1 Retriever Interface (`retriever/interface.go`)

```go
package retriever

import (
    "context"

    "github.com/Ranganaths/minion/vectorstore"
)

// Retriever retrieves relevant documents for a query
type Retriever interface {
    // GetRelevantDocuments retrieves documents for a query
    GetRelevantDocuments(ctx context.Context, query string) ([]vectorstore.Document, error)
}

// RetrieverConfig configures a retriever
type RetrieverConfig struct {
    TopK           int
    ScoreThreshold float32
    Filter         map[string]any
}
```

### 5.2 Vector Store Retriever (`retriever/vectorstore.go`)

```go
package retriever

import (
    "context"

    "github.com/Ranganaths/minion/vectorstore"
)

// VectorStoreRetriever retrieves documents from a vector store
type VectorStoreRetriever struct {
    store   vectorstore.VectorStore
    config  RetrieverConfig
}

// NewVectorStoreRetriever creates a new vector store retriever
func NewVectorStoreRetriever(store vectorstore.VectorStore, opts ...Option) *VectorStoreRetriever {
    cfg := RetrieverConfig{TopK: 4}
    for _, opt := range opts {
        opt(&cfg)
    }
    return &VectorStoreRetriever{store: store, config: cfg}
}

func (r *VectorStoreRetriever) GetRelevantDocuments(ctx context.Context, query string) ([]vectorstore.Document, error) {
    opts := []vectorstore.SearchOption{}
    if r.config.ScoreThreshold > 0 {
        opts = append(opts, vectorstore.WithScoreThreshold(r.config.ScoreThreshold))
    }
    if r.config.Filter != nil {
        opts = append(opts, vectorstore.WithFilter(r.config.Filter))
    }
    return r.store.SimilaritySearch(ctx, query, r.config.TopK, opts...)
}
```

### 5.3 Multi-Query Retriever (`retriever/multiquery.go`)

```go
package retriever

import (
    "context"
    "strings"

    "github.com/Ranganaths/minion/llm"
    "github.com/Ranganaths/minion/vectorstore"
)

// MultiQueryRetriever generates multiple query variations
type MultiQueryRetriever struct {
    baseRetriever Retriever
    llm           llm.Provider
    numQueries    int
}

// NewMultiQueryRetriever creates a new multi-query retriever
func NewMultiQueryRetriever(base Retriever, llmProvider llm.Provider, numQueries int) *MultiQueryRetriever {
    return &MultiQueryRetriever{
        baseRetriever: base,
        llm:           llmProvider,
        numQueries:    numQueries,
    }
}

func (r *MultiQueryRetriever) GetRelevantDocuments(ctx context.Context, query string) ([]vectorstore.Document, error) {
    // Generate query variations
    queries, err := r.generateQueries(ctx, query)
    if err != nil {
        return nil, err
    }

    // Collect documents from all queries
    seen := make(map[string]bool)
    var allDocs []vectorstore.Document

    for _, q := range queries {
        docs, err := r.baseRetriever.GetRelevantDocuments(ctx, q)
        if err != nil {
            continue
        }
        for _, doc := range docs {
            if !seen[doc.ID] {
                seen[doc.ID] = true
                allDocs = append(allDocs, doc)
            }
        }
    }

    return allDocs, nil
}

func (r *MultiQueryRetriever) generateQueries(ctx context.Context, query string) ([]string, error) {
    prompt := `Generate %d different versions of the following question to help retrieve relevant documents.
Each version should approach the question from a different angle or use different keywords.

Original question: %s

Output each query on a new line, without numbering.`

    resp, err := r.llm.GenerateCompletion(ctx, &llm.CompletionRequest{
        UserPrompt: strings.ReplaceAll(strings.ReplaceAll(prompt, "%d", string(rune(r.numQueries))), "%s", query),
    })
    if err != nil {
        return []string{query}, nil // Fallback to original
    }

    queries := []string{query}
    for _, line := range strings.Split(resp.Text, "\n") {
        line = strings.TrimSpace(line)
        if line != "" && line != query {
            queries = append(queries, line)
        }
    }

    return queries, nil
}
```

### 5.4 RAG Chain (`chain/rag_chain.go`)

```go
package chain

import (
    "context"
    "fmt"
    "strings"

    "github.com/Ranganaths/minion/llm"
    "github.com/Ranganaths/minion/prompt"
    "github.com/Ranganaths/minion/retriever"
    "github.com/Ranganaths/minion/vectorstore"
)

const defaultRAGPrompt = `Use the following pieces of context to answer the question at the end.
If you don't know the answer, just say that you don't know, don't try to make up an answer.

Context:
{{.context}}

Question: {{.question}}

Answer:`

// RAGChain implements Retrieval-Augmented Generation
type RAGChain struct {
    *BaseChain
    llm           llm.Provider
    retriever     retriever.Retriever
    prompt        prompt.Template
    returnSources bool
    inputKey      string
    outputKey     string
}

// RAGChainConfig configures the RAG chain
type RAGChainConfig struct {
    LLM           llm.Provider
    Retriever     retriever.Retriever
    Prompt        prompt.Template
    ReturnSources bool
    InputKey      string
    OutputKey     string
    Options       []Option
}

// NewRAGChain creates a new RAG chain
func NewRAGChain(cfg RAGChainConfig) *RAGChain {
    inputKey := cfg.InputKey
    if inputKey == "" {
        inputKey = "question"
    }
    outputKey := cfg.OutputKey
    if outputKey == "" {
        outputKey = "answer"
    }

    promptTmpl := cfg.Prompt
    if promptTmpl == nil {
        promptTmpl, _ = prompt.NewTemplate(defaultRAGPrompt)
    }

    return &RAGChain{
        BaseChain:     NewBaseChain("rag_chain", cfg.Options...),
        llm:           cfg.LLM,
        retriever:     cfg.Retriever,
        prompt:        promptTmpl,
        returnSources: cfg.ReturnSources,
        inputKey:      inputKey,
        outputKey:     outputKey,
    }
}

func (c *RAGChain) InputKeys() []string {
    return []string{c.inputKey}
}

func (c *RAGChain) OutputKeys() []string {
    keys := []string{c.outputKey}
    if c.returnSources {
        keys = append(keys, "sources")
    }
    return keys
}

func (c *RAGChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
    c.notifyStart(ctx, inputs)

    question, ok := inputs[c.inputKey].(string)
    if !ok {
        err := fmt.Errorf("input key %s must be a string", c.inputKey)
        c.notifyError(ctx, err)
        return nil, err
    }

    // Retrieve relevant documents
    docs, err := c.retriever.GetRelevantDocuments(ctx, question)
    if err != nil {
        c.notifyError(ctx, fmt.Errorf("retrieval error: %w", err))
        return nil, err
    }

    // Format context from documents
    contextParts := make([]string, len(docs))
    for i, doc := range docs {
        contextParts[i] = doc.PageContent
    }
    context := strings.Join(contextParts, "\n\n")

    // Format prompt
    promptText, err := c.prompt.Format(map[string]any{
        "context":  context,
        "question": question,
    })
    if err != nil {
        c.notifyError(ctx, fmt.Errorf("prompt format error: %w", err))
        return nil, err
    }

    // Generate answer
    resp, err := c.llm.GenerateCompletion(ctx, &llm.CompletionRequest{
        UserPrompt: promptText,
    })
    if err != nil {
        c.notifyError(ctx, fmt.Errorf("llm error: %w", err))
        return nil, err
    }

    outputs := map[string]any{
        c.outputKey: resp.Text,
    }

    if c.returnSources {
        outputs["sources"] = docs
    }

    c.notifyEnd(ctx, outputs)
    return outputs, nil
}

func (c *RAGChain) Stream(ctx context.Context, inputs map[string]any) (<-chan StreamEvent, error) {
    ch := make(chan StreamEvent)
    go func() {
        defer close(ch)
        result, err := c.Call(ctx, inputs)
        if err != nil {
            ch <- StreamEvent{Type: StreamEventError, Error: err}
            return
        }
        ch <- StreamEvent{Type: StreamEventComplete, Data: result}
    }()
    return ch, nil
}
```

### 5.5 Conversational RAG Chain (`chain/conversational_chain.go`)

```go
package chain

import (
    "context"
    "fmt"
    "strings"

    "github.com/Ranganaths/minion/llm"
    "github.com/Ranganaths/minion/prompt"
    "github.com/Ranganaths/minion/retriever"
)

const condenseQuestionPrompt = `Given the following conversation and a follow up question, rephrase the follow up question to be a standalone question.

Chat History:
{{.chat_history}}

Follow Up Input: {{.question}}

Standalone question:`

// ConversationalRAGChain handles multi-turn conversations with RAG
type ConversationalRAGChain struct {
    *BaseChain
    ragChain           *RAGChain
    condenseChain      *LLMChain
    memoryKey          string
    inputKey           string
    outputKey          string
    returnSourceDocs   bool
}

// ConversationalRAGChainConfig configures the conversational RAG chain
type ConversationalRAGChainConfig struct {
    LLM           llm.Provider
    Retriever     retriever.Retriever
    RAGPrompt     prompt.Template
    MemoryKey     string
    InputKey      string
    OutputKey     string
    ReturnSources bool
    Options       []Option
}

// NewConversationalRAGChain creates a new conversational RAG chain
func NewConversationalRAGChain(cfg ConversationalRAGChainConfig) *ConversationalRAGChain {
    memoryKey := cfg.MemoryKey
    if memoryKey == "" {
        memoryKey = "chat_history"
    }
    inputKey := cfg.InputKey
    if inputKey == "" {
        inputKey = "question"
    }
    outputKey := cfg.OutputKey
    if outputKey == "" {
        outputKey = "answer"
    }

    // Create condense question chain
    condenseTmpl, _ := prompt.NewTemplate(condenseQuestionPrompt)
    condenseChain := NewLLMChain(LLMChainConfig{
        LLM:       cfg.LLM,
        Prompt:    condenseTmpl,
        OutputKey: "standalone_question",
    })

    // Create RAG chain
    ragChain := NewRAGChain(RAGChainConfig{
        LLM:           cfg.LLM,
        Retriever:     cfg.Retriever,
        Prompt:        cfg.RAGPrompt,
        ReturnSources: cfg.ReturnSources,
        InputKey:      "question",
        OutputKey:     outputKey,
    })

    return &ConversationalRAGChain{
        BaseChain:        NewBaseChain("conversational_rag_chain", cfg.Options...),
        ragChain:         ragChain,
        condenseChain:    condenseChain,
        memoryKey:        memoryKey,
        inputKey:         inputKey,
        outputKey:        outputKey,
        returnSourceDocs: cfg.ReturnSources,
    }
}

func (c *ConversationalRAGChain) InputKeys() []string {
    return []string{c.inputKey, c.memoryKey}
}

func (c *ConversationalRAGChain) OutputKeys() []string {
    keys := []string{c.outputKey}
    if c.returnSourceDocs {
        keys = append(keys, "sources")
    }
    return keys
}

func (c *ConversationalRAGChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
    c.notifyStart(ctx, inputs)

    question, _ := inputs[c.inputKey].(string)
    chatHistory, _ := inputs[c.memoryKey].(string)

    // If there's chat history, condense the question
    standaloneQuestion := question
    if chatHistory != "" {
        condenseResult, err := c.condenseChain.Call(ctx, map[string]any{
            "question":     question,
            "chat_history": chatHistory,
        })
        if err == nil {
            if sq, ok := condenseResult["standalone_question"].(string); ok {
                standaloneQuestion = sq
            }
        }
    }

    // Run RAG chain with standalone question
    ragResult, err := c.ragChain.Call(ctx, map[string]any{
        "question": standaloneQuestion,
    })
    if err != nil {
        c.notifyError(ctx, err)
        return nil, err
    }

    c.notifyEnd(ctx, ragResult)
    return ragResult, nil
}

func (c *ConversationalRAGChain) Stream(ctx context.Context, inputs map[string]any) (<-chan StreamEvent, error) {
    ch := make(chan StreamEvent)
    go func() {
        defer close(ch)
        result, err := c.Call(ctx, inputs)
        if err != nil {
            ch <- StreamEvent{Type: StreamEventError, Error: err}
            return
        }
        ch <- StreamEvent{Type: StreamEventComplete, Data: result}
    }()
    return ch, nil
}
```

### 5.6 Implementation Tasks

| Task | File | Effort | Dependencies |
|------|------|--------|--------------|
| 5.1 | `retriever/interface.go` | 1h | None |
| 5.2 | `retriever/vectorstore.go` | 2h | 5.1, Phase 3 |
| 5.3 | `retriever/multiquery.go` | 4h | 5.1 |
| 5.4 | `retriever/selfquery.go` | 6h | 5.1 |
| 5.5 | `retriever/ensemble.go` | 3h | 5.1 |
| 5.6 | `retriever/contextual.go` | 4h | 5.1 |
| 5.7 | `chain/rag_chain.go` | 6h | Phase 1, 5.1 |
| 5.8 | `chain/conversational_chain.go` | 5h | 5.7 |
| 5.9 | Unit & integration tests | 8h | All above |

**Phase 5 Total: 39 hours**

---

## Phase 6: Prompt Templates & Output Parsers

**Duration:** Week 6
**Priority:** P1 (High)
**Dependencies:** Phase 1

### 6.1 Prompt Template (`prompt/template.go`)

```go
package prompt

import (
    "bytes"
    "fmt"
    "text/template"
)

// Template is a prompt template interface
type Template interface {
    Format(variables map[string]any) (string, error)
    InputVariables() []string
    PartialFormat(variables map[string]any) (Template, error)
}

// StringTemplate is a Go template-based prompt template
type StringTemplate struct {
    template  *template.Template
    variables []string
    partial   map[string]any
}

// NewTemplate creates a new string template
func NewTemplate(tmpl string, inputVars ...string) (*StringTemplate, error) {
    t, err := template.New("prompt").Parse(tmpl)
    if err != nil {
        return nil, fmt.Errorf("parse template: %w", err)
    }

    // Extract variables if not provided
    vars := inputVars
    if len(vars) == 0 {
        vars = extractVariables(tmpl)
    }

    return &StringTemplate{
        template:  t,
        variables: vars,
        partial:   make(map[string]any),
    }, nil
}

func (t *StringTemplate) Format(variables map[string]any) (string, error) {
    // Merge partial variables
    merged := make(map[string]any)
    for k, v := range t.partial {
        merged[k] = v
    }
    for k, v := range variables {
        merged[k] = v
    }

    var buf bytes.Buffer
    if err := t.template.Execute(&buf, merged); err != nil {
        return "", fmt.Errorf("execute template: %w", err)
    }
    return buf.String(), nil
}

func (t *StringTemplate) InputVariables() []string {
    // Return variables not in partial
    var result []string
    for _, v := range t.variables {
        if _, ok := t.partial[v]; !ok {
            result = append(result, v)
        }
    }
    return result
}

func (t *StringTemplate) PartialFormat(variables map[string]any) (Template, error) {
    newPartial := make(map[string]any)
    for k, v := range t.partial {
        newPartial[k] = v
    }
    for k, v := range variables {
        newPartial[k] = v
    }
    return &StringTemplate{
        template:  t.template,
        variables: t.variables,
        partial:   newPartial,
    }, nil
}

func extractVariables(tmpl string) []string {
    // Simple extraction - looks for {{.varname}}
    var vars []string
    // Implementation would parse template for variable references
    return vars
}
```

### 6.2 JSON Output Parser (`outputparser/json.go`)

```go
package outputparser

import (
    "encoding/json"
    "fmt"
    "regexp"
    "strings"
)

// Parser parses LLM output into structured format
type Parser[T any] interface {
    Parse(text string) (T, error)
    GetFormatInstructions() string
}

// JSONParser parses JSON from LLM output
type JSONParser[T any] struct {
    schema string
}

// NewJSONParser creates a new JSON parser
func NewJSONParser[T any]() *JSONParser[T] {
    return &JSONParser[T]{}
}

func (p *JSONParser[T]) Parse(text string) (T, error) {
    var result T

    // Try to extract JSON from markdown code blocks
    jsonStr := extractJSON(text)

    if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
        return result, fmt.Errorf("json parse error: %w", err)
    }

    return result, nil
}

func (p *JSONParser[T]) GetFormatInstructions() string {
    return `Your response should be a JSON object. Do not include any text before or after the JSON.`
}

// extractJSON extracts JSON from text, handling markdown code blocks
func extractJSON(text string) string {
    // Try to extract from code block
    re := regexp.MustCompile("```(?:json)?\\s*([\\s\\S]*?)```")
    matches := re.FindStringSubmatch(text)
    if len(matches) > 1 {
        return strings.TrimSpace(matches[1])
    }

    // Try to find JSON object or array
    text = strings.TrimSpace(text)
    if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
        return text
    }

    return text
}

// StructuredParser parses into a Go struct with schema validation
type StructuredParser[T any] struct {
    example T
}

// NewStructuredParser creates a parser for a specific struct type
func NewStructuredParser[T any](example T) *StructuredParser[T] {
    return &StructuredParser[T]{example: example}
}

func (p *StructuredParser[T]) Parse(text string) (T, error) {
    var result T
    jsonStr := extractJSON(text)
    if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
        return result, err
    }
    return result, nil
}

func (p *StructuredParser[T]) GetFormatInstructions() string {
    schema, _ := json.MarshalIndent(p.example, "", "  ")
    return fmt.Sprintf(`Your response should be a JSON object matching this schema:
%s`, string(schema))
}

// ListParser parses a list from LLM output
type ListParser struct {
    separator string
}

// NewListParser creates a new list parser
func NewListParser(separator string) *ListParser {
    if separator == "" {
        separator = "\n"
    }
    return &ListParser{separator: separator}
}

func (p *ListParser) Parse(text string) ([]string, error) {
    lines := strings.Split(text, p.separator)
    var result []string
    for _, line := range lines {
        line = strings.TrimSpace(line)
        // Remove common list prefixes
        line = strings.TrimPrefix(line, "- ")
        line = strings.TrimPrefix(line, "* ")
        line = regexp.MustCompile(`^\d+\.\s*`).ReplaceAllString(line, "")
        if line != "" {
            result = append(result, line)
        }
    }
    return result, nil
}

func (p *ListParser) GetFormatInstructions() string {
    return `Your response should be a list of items, one per line.`
}
```

### 6.3 Implementation Tasks

| Task | File | Effort | Dependencies |
|------|------|--------|--------------|
| 6.1 | `prompt/interface.go` | 1h | None |
| 6.2 | `prompt/template.go` | 3h | 6.1 |
| 6.3 | `prompt/chat.go` | 3h | 6.1 |
| 6.4 | `prompt/fewshot.go` | 4h | 6.1 |
| 6.5 | `outputparser/interface.go` | 1h | None |
| 6.6 | `outputparser/json.go` | 3h | 6.5 |
| 6.7 | `outputparser/structured.go` | 3h | 6.5 |
| 6.8 | `outputparser/list.go` | 2h | 6.5 |
| 6.9 | `outputparser/retry.go` | 3h | 6.5 |
| 6.10 | Unit tests | 4h | All above |

**Phase 6 Total: 27 hours**

---

## Phase 7: RAG Pipeline Builder & Integration

**Duration:** Week 7
**Priority:** P1 (High)
**Dependencies:** All previous phases

### 7.1 RAG Pipeline (`rag/pipeline.go`)

```go
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

// Pipeline orchestrates the complete RAG workflow
type Pipeline struct {
    loader      documentloader.Loader
    splitter    textsplitter.Splitter
    embedder    embeddings.Embedder
    vectorStore vectorstore.VectorStore
    retriever   retriever.Retriever
    chain       chain.Chain
    llm         llm.Provider
}

// Ingest loads, splits, embeds, and stores documents
func (p *Pipeline) Ingest(ctx context.Context) (int, error) {
    // Load documents
    docs, err := p.loader.Load(ctx)
    if err != nil {
        return 0, fmt.Errorf("load documents: %w", err)
    }

    // Split documents
    if p.splitter != nil {
        docs, err = p.splitter.SplitDocuments(docs)
        if err != nil {
            return 0, fmt.Errorf("split documents: %w", err)
        }
    }

    // Add to vector store
    ids, err := p.vectorStore.AddDocuments(ctx, docs, p.embedder)
    if err != nil {
        return 0, fmt.Errorf("add to vector store: %w", err)
    }

    return len(ids), nil
}

// Query performs RAG query
func (p *Pipeline) Query(ctx context.Context, question string) (string, []vectorstore.Document, error) {
    result, err := p.chain.Call(ctx, map[string]any{
        "question": question,
    })
    if err != nil {
        return "", nil, err
    }

    answer, _ := result["answer"].(string)
    sources, _ := result["sources"].([]vectorstore.Document)

    return answer, sources, nil
}

// QueryWithHistory performs conversational RAG query
func (p *Pipeline) QueryWithHistory(ctx context.Context, question, chatHistory string) (string, []vectorstore.Document, error) {
    result, err := p.chain.Call(ctx, map[string]any{
        "question":     question,
        "chat_history": chatHistory,
    })
    if err != nil {
        return "", nil, err
    }

    answer, _ := result["answer"].(string)
    sources, _ := result["sources"].([]vectorstore.Document)

    return answer, sources, nil
}
```

### 7.2 Pipeline Builder (`rag/builder.go`)

```go
package rag

import (
    "database/sql"

    "github.com/Ranganaths/minion/chain"
    "github.com/Ranganaths/minion/documentloader"
    "github.com/Ranganaths/minion/embeddings"
    "github.com/Ranganaths/minion/llm"
    "github.com/Ranganaths/minion/prompt"
    "github.com/Ranganaths/minion/retriever"
    "github.com/Ranganaths/minion/textsplitter"
    "github.com/Ranganaths/minion/vectorstore"
)

// Builder builds RAG pipelines with a fluent API
type Builder struct {
    pipeline *Pipeline
    err      error
}

// NewBuilder creates a new pipeline builder
func NewBuilder() *Builder {
    return &Builder{
        pipeline: &Pipeline{},
    }
}

// WithLoader sets the document loader
func (b *Builder) WithLoader(loader documentloader.Loader) *Builder {
    if b.err != nil {
        return b
    }
    b.pipeline.loader = loader
    return b
}

// WithDirectoryLoader sets a directory loader
func (b *Builder) WithDirectoryLoader(path string, extensions ...string) *Builder {
    if b.err != nil {
        return b
    }
    b.pipeline.loader = documentloader.NewDirectory(path,
        documentloader.WithExtensions(extensions...),
        documentloader.WithRecursive(true),
    )
    return b
}

// WithPDFLoader sets a PDF loader
func (b *Builder) WithPDFLoader(path string) *Builder {
    if b.err != nil {
        return b
    }
    b.pipeline.loader = documentloader.NewPDF(path)
    return b
}

// WithSplitter sets the text splitter
func (b *Builder) WithSplitter(splitter textsplitter.Splitter) *Builder {
    if b.err != nil {
        return b
    }
    b.pipeline.splitter = splitter
    return b
}

// WithRecursiveSplitter sets a recursive character splitter
func (b *Builder) WithRecursiveSplitter(chunkSize, overlap int) *Builder {
    if b.err != nil {
        return b
    }
    b.pipeline.splitter = textsplitter.NewRecursive(
        textsplitter.WithChunkSize(chunkSize),
        textsplitter.WithChunkOverlap(overlap),
    )
    return b
}

// WithEmbedder sets the embedder
func (b *Builder) WithEmbedder(embedder embeddings.Embedder) *Builder {
    if b.err != nil {
        return b
    }
    b.pipeline.embedder = embedder
    return b
}

// WithOpenAIEmbeddings sets OpenAI embeddings
func (b *Builder) WithOpenAIEmbeddings(apiKey string, model string) *Builder {
    if b.err != nil {
        return b
    }
    b.pipeline.embedder = embeddings.NewOpenAI(apiKey, embeddings.WithModel(model))
    return b
}

// WithVectorStore sets the vector store
func (b *Builder) WithVectorStore(store vectorstore.VectorStore) *Builder {
    if b.err != nil {
        return b
    }
    b.pipeline.vectorStore = store
    return b
}

// WithMemoryVectorStore sets an in-memory vector store
func (b *Builder) WithMemoryVectorStore() *Builder {
    if b.err != nil {
        return b
    }
    if b.pipeline.embedder == nil {
        b.err = fmt.Errorf("embedder must be set before vector store")
        return b
    }
    b.pipeline.vectorStore = vectorstore.NewMemory(b.pipeline.embedder)
    return b
}

// WithPgVectorStore sets a PostgreSQL vector store
func (b *Builder) WithPgVectorStore(db *sql.DB, collection string) *Builder {
    if b.err != nil {
        return b
    }
    if b.pipeline.embedder == nil {
        b.err = fmt.Errorf("embedder must be set before vector store")
        return b
    }
    store, err := vectorstore.NewPgVector(vectorstore.PgVectorConfig{
        DB:         db,
        Embedder:   b.pipeline.embedder,
        Collection: collection,
        Dimensions: b.pipeline.embedder.Dimensions(),
    })
    if err != nil {
        b.err = err
        return b
    }
    b.pipeline.vectorStore = store
    return b
}

// WithRetriever sets the retriever
func (b *Builder) WithRetriever(r retriever.Retriever) *Builder {
    if b.err != nil {
        return b
    }
    b.pipeline.retriever = r
    return b
}

// WithVectorStoreRetriever sets a vector store retriever
func (b *Builder) WithVectorStoreRetriever(topK int) *Builder {
    if b.err != nil {
        return b
    }
    if b.pipeline.vectorStore == nil {
        b.err = fmt.Errorf("vector store must be set before retriever")
        return b
    }
    b.pipeline.retriever = retriever.NewVectorStoreRetriever(
        b.pipeline.vectorStore,
        retriever.WithTopK(topK),
    )
    return b
}

// WithLLM sets the LLM provider
func (b *Builder) WithLLM(provider llm.Provider) *Builder {
    if b.err != nil {
        return b
    }
    b.pipeline.llm = provider
    return b
}

// WithOpenAI sets OpenAI as the LLM
func (b *Builder) WithOpenAI(apiKey, model string) *Builder {
    if b.err != nil {
        return b
    }
    b.pipeline.llm = llm.NewOpenAI(apiKey, model)
    return b
}

// WithRAGChain builds a RAG chain
func (b *Builder) WithRAGChain(opts ...chain.Option) *Builder {
    if b.err != nil {
        return b
    }
    if b.pipeline.llm == nil {
        b.err = fmt.Errorf("LLM must be set before chain")
        return b
    }
    if b.pipeline.retriever == nil {
        b.err = fmt.Errorf("retriever must be set before chain")
        return b
    }
    b.pipeline.chain = chain.NewRAGChain(chain.RAGChainConfig{
        LLM:           b.pipeline.llm,
        Retriever:     b.pipeline.retriever,
        ReturnSources: true,
        Options:       opts,
    })
    return b
}

// WithConversationalChain builds a conversational RAG chain
func (b *Builder) WithConversationalChain(opts ...chain.Option) *Builder {
    if b.err != nil {
        return b
    }
    if b.pipeline.llm == nil {
        b.err = fmt.Errorf("LLM must be set before chain")
        return b
    }
    if b.pipeline.retriever == nil {
        b.err = fmt.Errorf("retriever must be set before chain")
        return b
    }
    b.pipeline.chain = chain.NewConversationalRAGChain(chain.ConversationalRAGChainConfig{
        LLM:           b.pipeline.llm,
        Retriever:     b.pipeline.retriever,
        ReturnSources: true,
        Options:       opts,
    })
    return b
}

// Build constructs the pipeline
func (b *Builder) Build() (*Pipeline, error) {
    if b.err != nil {
        return nil, b.err
    }
    return b.pipeline, nil
}
```

### 7.3 Implementation Tasks

| Task | File | Effort | Dependencies |
|------|------|--------|--------------|
| 7.1 | `rag/pipeline.go` | 4h | All phases |
| 7.2 | `rag/builder.go` | 4h | 7.1 |
| 7.3 | `rag/ingest.go` | 3h | 7.1 |
| 7.4 | Integration with Minion core | 4h | 7.1-7.3 |
| 7.5 | Observability integration | 4h | 7.4 |
| 7.6 | Example applications | 4h | All above |
| 7.7 | Documentation | 4h | All above |
| 7.8 | Integration tests | 6h | All above |

**Phase 7 Total: 33 hours**

---

## Phase 8: Multi-Agent Integration Workers

**Duration:** Week 8
**Priority:** P0 (Critical)
**Dependencies:** Phases 1-7, Existing Multi-Agent System

> **CRITICAL:** This phase ensures Chain/RAG features integrate with the multi-agent system without breaking existing functionality.

### 8.1 LLM Adapters (`adapters/llm_adapter.go`)

```go
package adapters

import (
    "context"

    "github.com/Ranganaths/minion/core/multiagent"
    "github.com/Ranganaths/minion/llm"
)

// MultiAgentLLMAdapter adapts minion/llm.Provider to multiagent.LLMProvider
type MultiAgentLLMAdapter struct {
    provider llm.Provider
}

// NewMultiAgentLLMAdapter creates adapter for multi-agent use
func NewMultiAgentLLMAdapter(provider llm.Provider) *MultiAgentLLMAdapter {
    return &MultiAgentLLMAdapter{provider: provider}
}

// GenerateCompletion implements multiagent.LLMProvider
func (a *MultiAgentLLMAdapter) GenerateCompletion(
    ctx context.Context,
    req *multiagent.CompletionRequest,
) (*multiagent.CompletionResponse, error) {
    llmReq := &llm.CompletionRequest{
        SystemPrompt: req.SystemPrompt,
        UserPrompt:   req.Prompt,
        Temperature:  req.Temperature,
        MaxTokens:    req.MaxTokens,
        Model:        req.Model,
    }

    resp, err := a.provider.GenerateCompletion(ctx, llmReq)
    if err != nil {
        return nil, err
    }

    return &multiagent.CompletionResponse{
        Text:       resp.Text,
        TokensUsed: resp.TokensUsed,
    }, nil
}

// ChainLLMAdapter adapts multiagent.LLMProvider to llm.Provider for chains
type ChainLLMAdapter struct {
    provider multiagent.LLMProvider
}

// NewChainLLMAdapter creates adapter for chain use
func NewChainLLMAdapter(provider multiagent.LLMProvider) *ChainLLMAdapter {
    return &ChainLLMAdapter{provider: provider}
}

func (a *ChainLLMAdapter) GenerateCompletion(
    ctx context.Context,
    req *llm.CompletionRequest,
) (*llm.CompletionResponse, error) {
    maReq := &multiagent.CompletionRequest{
        Prompt:       req.UserPrompt,
        SystemPrompt: req.SystemPrompt,
        Temperature:  req.Temperature,
        MaxTokens:    req.MaxTokens,
    }

    resp, err := a.provider.GenerateCompletion(ctx, maReq)
    if err != nil {
        return nil, err
    }

    return &llm.CompletionResponse{
        Text:       resp.Text,
        TokensUsed: resp.TokensUsed,
    }, nil
}

func (a *ChainLLMAdapter) GenerateChat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
    // Build prompt from messages
    var prompt string
    for _, msg := range req.Messages {
        prompt += msg.Role + ": " + msg.Content + "\n"
    }

    resp, err := a.provider.GenerateCompletion(ctx, &multiagent.CompletionRequest{
        Prompt:      prompt,
        Temperature: req.Temperature,
        MaxTokens:   req.MaxTokens,
    })
    if err != nil {
        return nil, err
    }

    return &llm.ChatResponse{
        Message: llm.Message{
            Role:    "assistant",
            Content: resp.Text,
        },
        TokensUsed: resp.TokensUsed,
    }, nil
}

func (a *ChainLLMAdapter) Name() string {
    return "multiagent_adapter"
}
```

### 8.2 RAG Worker (`chain/workers/rag_worker.go`)

```go
package workers

import (
    "context"
    "fmt"
    "strings"

    "github.com/Ranganaths/minion/core/multiagent"
    "github.com/Ranganaths/minion/retriever"
    "github.com/Ranganaths/minion/vectorstore"
)

// RAGWorker implements multiagent.TaskHandler for RAG tasks
type RAGWorker struct {
    llmProvider multiagent.LLMProvider
    vectorStore vectorstore.VectorStore
    retriever   retriever.Retriever
    topK        int
}

// NewRAGWorker creates a RAG-capable worker for multi-agent system
func NewRAGWorker(
    llm multiagent.LLMProvider,
    vs vectorstore.VectorStore,
    ret retriever.Retriever,
) *RAGWorker {
    return &RAGWorker{
        llmProvider: llm,
        vectorStore: vs,
        retriever:   ret,
        topK:        5,
    }
}

// GetName implements multiagent.TaskHandler
func (w *RAGWorker) GetName() string {
    return "RAGWorker"
}

// GetCapabilities implements multiagent.TaskHandler
// These capabilities are used by the orchestrator for task routing
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

// HandleTask implements multiagent.TaskHandler
func (w *RAGWorker) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
    // Extract query from task input
    input, ok := task.Input.(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("invalid task input format: expected map[string]interface{}")
    }

    query, _ := input["query"].(string)
    if query == "" {
        // Try alternate key
        query, _ = input["question"].(string)
    }
    if query == "" {
        return nil, fmt.Errorf("query/question is required in task input")
    }

    // Use retriever to get relevant documents
    docs, err := w.retriever.GetRelevantDocuments(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("retrieval error: %w", err)
    }

    // Build context from documents
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

    // Generate answer using LLM
    prompt := fmt.Sprintf(`Use the following context to answer the question. If you cannot find the answer in the context, say "I don't have enough information to answer this question."

Context:
%s

Question: %s

Answer:`, contextBuilder.String(), query)

    resp, err := w.llmProvider.GenerateCompletion(ctx, &multiagent.CompletionRequest{
        Prompt:      prompt,
        MaxTokens:   1000,
        Temperature: 0.3,
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
```

### 8.3 Chain Worker (`chain/workers/chain_worker.go`)

```go
package workers

import (
    "context"
    "fmt"

    "github.com/Ranganaths/minion/chain"
    "github.com/Ranganaths/minion/core/multiagent"
)

// ChainWorker wraps any chain.Chain as a multiagent.TaskHandler
type ChainWorker struct {
    name         string
    capabilities []string
    chain        chain.Chain
}

// NewChainWorker creates a worker from any chain
func NewChainWorker(name string, capabilities []string, c chain.Chain) *ChainWorker {
    return &ChainWorker{
        name:         name,
        capabilities: capabilities,
        chain:        c,
    }
}

// GetName implements multiagent.TaskHandler
func (w *ChainWorker) GetName() string {
    return w.name
}

// GetCapabilities implements multiagent.TaskHandler
func (w *ChainWorker) GetCapabilities() []string {
    return w.capabilities
}

// HandleTask implements multiagent.TaskHandler
func (w *ChainWorker) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
    // Convert task input to chain inputs
    var inputs map[string]interface{}

    switch v := task.Input.(type) {
    case map[string]interface{}:
        inputs = v
    case string:
        // Assume it's the primary input
        inputs = map[string]interface{}{
            w.chain.InputKeys()[0]: v,
        }
    default:
        return nil, fmt.Errorf("unsupported task input type: %T", task.Input)
    }

    // Execute chain
    outputs, err := w.chain.Call(ctx, inputs)
    if err != nil {
        return nil, fmt.Errorf("chain execution error: %w", err)
    }

    return outputs, nil
}
```

### 8.4 Retrieval Worker (`chain/workers/retrieval_worker.go`)

```go
package workers

import (
    "context"
    "fmt"

    "github.com/Ranganaths/minion/core/multiagent"
    "github.com/Ranganaths/minion/retriever"
)

// RetrievalWorker is a specialized worker for document retrieval only
type RetrievalWorker struct {
    retriever retriever.Retriever
    topK      int
}

// NewRetrievalWorker creates a retrieval-only worker
func NewRetrievalWorker(ret retriever.Retriever, topK int) *RetrievalWorker {
    return &RetrievalWorker{
        retriever: ret,
        topK:      topK,
    }
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

// HandleTask implements multiagent.TaskHandler
func (w *RetrievalWorker) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
    input, ok := task.Input.(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("invalid task input format")
    }

    query, _ := input["query"].(string)
    if query == "" {
        return nil, fmt.Errorf("query is required")
    }

    docs, err := w.retriever.GetRelevantDocuments(ctx, query)
    if err != nil {
        return nil, err
    }

    // Convert to serializable format
    var results []map[string]interface{}
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
    }, nil
}
```

### 8.5 Implementation Tasks

| Task | File | Effort | Dependencies |
|------|------|--------|--------------|
| 8.1 | `adapters/llm_adapter.go` | 3h | None |
| 8.2 | `chain/workers/rag_worker.go` | 4h | 8.1, Phase 5 |
| 8.3 | `chain/workers/chain_worker.go` | 3h | 8.1, Phase 1 |
| 8.4 | `chain/workers/retrieval_worker.go` | 2h | Phase 5 |
| 8.5 | `chain/workers/ingestion_worker.go` | 3h | Phase 4, Phase 3 |
| 8.6 | Integration tests with existing workers | 6h | All above |
| 8.7 | Multi-agent + RAG example | 4h | All above |
| 8.8 | Documentation | 3h | All above |

**Phase 8 Total: 28 hours**

---

## Summary

### Total Effort by Phase

| Phase | Description | Effort (hours) |
|-------|-------------|----------------|
| 1 | Core Chain Package | 25 |
| 2 | Embeddings Package | 22 |
| 3 | Vector Store Package | 42 |
| 4 | Document Loaders & Splitters | 47 |
| 5 | Retrievers & RAG Chain | 39 |
| 6 | Prompt Templates & Output Parsers | 27 |
| 7 | RAG Pipeline & Integration | 33 |
| 8 | **Multi-Agent Integration Workers** | **28** |
| **Total** | | **263 hours** |

### Timeline

| Week | Phases | Focus |
|------|--------|-------|
| 1 | Phase 1 | Chain interfaces and base chains |
| 2 | Phase 1-2 | LLM chain, Sequential chain, Embeddings |
| 3 | Phase 3 | Vector stores (Memory, PgVector) |
| 4 | Phase 3-4 | Additional vector stores, Document loaders |
| 5 | Phase 4-5 | Text splitters, Retrievers |
| 6 | Phase 5-6 | RAG chain, Prompt templates, Output parsers |
| 7 | Phase 7 | RAG pipeline builder, Integration, Testing |
| 8 | **Phase 8** | **Multi-Agent Workers, Adapters, Integration Tests** |

### Deliverables

1. **`chain/`** - Complete chain orchestration framework
2. **`embeddings/`** - 4+ embedding providers with caching
3. **`vectorstore/`** - 5+ vector store implementations
4. **`documentloader/`** - 8+ document format loaders
5. **`textsplitter/`** - 4+ text splitting strategies
6. **`retriever/`** - 5+ retriever implementations
7. **`prompt/`** - Prompt template system
8. **`outputparser/`** - Output parsing utilities
9. **`rag/`** - High-level RAG pipeline builder
10. **`adapters/`** - LLM interface adapters for multi-agent compatibility
11. **`chain/workers/`** - Multi-agent compatible workers (RAG, Chain, Retrieval)
12. **Examples** - Complete working examples including multi-agent + RAG
13. **Tests** - 80%+ code coverage, including multi-agent integration tests
14. **Documentation** - API docs, tutorials, and integration guide

---

## Multi-Agent Testing Requirements

### Must Pass Before Merge

1. **Existing Multi-Agent Tests**
   - All tests in `core/multiagent/*_test.go` pass unchanged
   - No modifications to existing test files

2. **Protocol Compatibility**
   - New workers work with In-Memory protocol
   - New workers work with Redis protocol
   - New workers work with Kafka protocol

3. **Load Balancer Compatibility**
   - RAGWorker correctly matched by capability
   - ChainWorker correctly matched by capability
   - Mixed workloads distribute correctly

4. **Orchestrator Compatibility**
   - Complex tasks decompose to include RAG subtasks
   - Task dependencies work with RAG workers
   - Results aggregate correctly

### Integration Test Examples

```go
func TestRAGWorkerWithExistingWorkers(t *testing.T) {
    coordinator := setupTestCoordinator(t)

    // Register RAG worker alongside built-in workers
    coordinator.CreateCustomWorker(setupRAGWorker(t))

    // Execute mixed workload
    result, err := coordinator.ExecuteTask(ctx, multiagent.TaskRequest{
        Name:        "Research and summarize",
        Description: "Find information and write summary",
        Type:        "complex",
        Input: map[string]interface{}{
            "topic": "AI agents",
        },
    })

    // Orchestrator should:
    // 1. Use RAGWorker for retrieval
    // 2. Use WriterWorker for summary
    // 3. Aggregate results

    assert.NoError(t, err)
    assert.NotNil(t, result.Output)
}

func TestAllProtocolsWithRAGWorker(t *testing.T) {
    protocols := []string{"inmemory", "redis", "kafka"}

    for _, proto := range protocols {
        t.Run(proto, func(t *testing.T) {
            coordinator := setupCoordinatorWithProtocol(t, proto)
            coordinator.CreateCustomWorker(setupRAGWorker(t))

            result, err := coordinator.ExecuteTask(ctx, multiagent.TaskRequest{
                Type: "rag",
                Input: map[string]interface{}{"query": "test"},
            })

            assert.NoError(t, err)
            assert.Equal(t, multiagent.TaskStatusCompleted, result.Status)
        })
    }
}
```

---

## Next Steps

1. **Review and approve this plan**
2. **Review `MULTIAGENT_INTEGRATION_GUIDE.md`**
3. **Set up new package directories**
4. **Begin Phase 1: Chain interface and base implementation**
5. **Create CI pipeline for new packages**
6. **Add multi-agent integration tests to CI**
7. **Weekly progress reviews**

---

## Related Documents

- `MULTIAGENT_INTEGRATION_GUIDE.md` - Detailed integration requirements
- `PRD_RAG_AND_LANGCHAIN_PARITY.md` - Full PRD with feature comparison

---

*End of Implementation Plan*
