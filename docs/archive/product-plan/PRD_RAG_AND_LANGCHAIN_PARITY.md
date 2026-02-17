# Product Requirements Document: RAG Infrastructure & LangChainGo Feature Parity

**Document Version:** 1.0
**Date:** 2025-12-20
**Author:** Minion Framework Team
**Status:** Draft

---

## Executive Summary

This PRD outlines the requirements for extending the Minion Agent Framework to achieve feature parity with LangChainGo, focusing primarily on RAG (Retrieval-Augmented Generation) infrastructure. While Minion excels in multi-agent orchestration, observability, and domain-specific tooling, it currently lacks the foundational components for building RAG applications that LangChainGo provides.

### Current State Analysis

| Capability | Minion | LangChainGo | Gap |
|------------|--------|-------------|-----|
| LLM Providers | 4 | 15+ | -11 |
| Vector Stores | 0 | 14+ | -14 |
| Embeddings | 0 | 7+ | -7 |
| Document Loaders | 0 | 7+ | -7 |
| Text Splitters | 0 | 4 | -4 |
| Retrievers | 0 | 7+ | -7 |
| Chains | 0 | 7+ | -7 |
| Output Parsers | 0 | 6+ | -6 |
| Prompt Templates | Basic | Comprehensive | Partial |

### Minion's Competitive Advantages (to preserve)
- Multi-agent Magentic-One architecture
- 80+ domain-specific tools
- MCP (Model Context Protocol) integration
- Advanced observability (Prometheus, OpenTelemetry)
- Resilience patterns (Circuit breaker, retry, rate limiting)
- Pluggable behavior system

---

## Goals and Objectives

### Primary Goals

1. **Enable RAG Applications**: Provide complete infrastructure for building production-grade RAG systems
2. **Achieve LangChainGo Parity**: Match core LangChainGo capabilities while maintaining Minion's unique strengths
3. **Maintain Composability**: Ensure new components integrate seamlessly with existing Minion architecture
4. **Production-Ready**: All components must meet enterprise-grade quality, observability, and resilience standards

### Success Metrics

| Metric | Target |
|--------|--------|
| Vector Store Integrations | ≥5 providers |
| Embedding Providers | ≥4 providers |
| Document Loaders | ≥6 formats |
| Text Splitters | ≥3 strategies |
| Test Coverage | ≥80% |
| Documentation Completeness | 100% API docs |

---

## Detailed Requirements

---

## Phase 1: Embeddings Infrastructure

### 1.1 Embeddings Interface

**Priority:** P0 (Critical)
**Effort:** 8 hours
**Dependencies:** None

#### Requirements

```go
// embeddings/interface.go
type Embedder interface {
    // EmbedDocuments embeds a list of documents
    EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)

    // EmbedQuery embeds a single query (may use different model/prompt)
    EmbedQuery(ctx context.Context, text string) ([]float32, error)

    // Dimensions returns the embedding dimension size
    Dimensions() int

    // Name returns the embedder name for metrics/logging
    Name() string
}

type EmbeddingResult struct {
    Embedding  []float32
    TokenCount int
    Metadata   map[string]interface{}
}
```

#### Functional Requirements
- FR-1.1.1: Support batch embedding with configurable batch sizes
- FR-1.1.2: Support async/concurrent embedding operations
- FR-1.1.3: Automatic retry with exponential backoff on failures
- FR-1.1.4: Token counting and cost tracking integration
- FR-1.1.5: Caching layer for repeated embeddings (optional)

#### Non-Functional Requirements
- NFR-1.1.1: Latency < 100ms for single embedding (excluding network)
- NFR-1.1.2: Support embeddings up to 8192 dimensions
- NFR-1.1.3: Memory-efficient batch processing

### 1.2 OpenAI Embeddings Provider

**Priority:** P0 (Critical)
**Effort:** 6 hours
**Dependencies:** 1.1

#### Requirements
- Support models: `text-embedding-3-small`, `text-embedding-3-large`, `text-embedding-ada-002`
- Configurable dimensions (for text-embedding-3-* models)
- Automatic batching (max 2048 inputs per request)
- Rate limiting integration
- Cost tracking per embedding

#### Configuration
```go
type OpenAIEmbeddingsConfig struct {
    APIKey       string
    Model        string  // default: text-embedding-3-small
    Dimensions   int     // optional, for dimension reduction
    BatchSize    int     // default: 512
    MaxRetries   int     // default: 3
    Timeout      time.Duration
}
```

### 1.3 Ollama Embeddings Provider

**Priority:** P0 (Critical)
**Effort:** 4 hours
**Dependencies:** 1.1

#### Requirements
- Support local Ollama server
- Support models: `nomic-embed-text`, `mxbai-embed-large`, `all-minilm`
- Configurable base URL
- Health check integration

### 1.4 Additional Embedding Providers

**Priority:** P1 (High)
**Effort:** 16 hours total

| Provider | Models | Effort |
|----------|--------|--------|
| Cohere | embed-english-v3.0, embed-multilingual-v3.0 | 4h |
| Hugging Face | sentence-transformers/* | 4h |
| Google Vertex AI | textembedding-gecko | 4h |
| AWS Bedrock | amazon.titan-embed-text-v1 | 4h |

---

## Phase 2: Vector Store Infrastructure

### 2.1 Vector Store Interface

**Priority:** P0 (Critical)
**Effort:** 10 hours
**Dependencies:** 1.1

#### Requirements

```go
// vectorstore/interface.go
type VectorStore interface {
    // AddDocuments adds documents to the store
    AddDocuments(ctx context.Context, docs []Document, embedder Embedder) ([]string, error)

    // SimilaritySearch finds similar documents
    SimilaritySearch(ctx context.Context, query string, k int, opts ...SearchOption) ([]Document, error)

    // SimilaritySearchWithScore includes similarity scores
    SimilaritySearchWithScore(ctx context.Context, query string, k int, opts ...SearchOption) ([]ScoredDocument, error)

    // Delete removes documents by IDs
    Delete(ctx context.Context, ids []string) error

    // AsRetriever returns a Retriever interface
    AsRetriever(opts ...RetrieverOption) Retriever
}

type Document struct {
    ID        string
    Content   string
    Metadata  map[string]interface{}
    Embedding []float32 // optional, pre-computed
}

type ScoredDocument struct {
    Document
    Score float32
}

type SearchOption func(*SearchOptions)
type SearchOptions struct {
    ScoreThreshold float32
    Filter         map[string]interface{}
    NamespaceID    string
}
```

#### Functional Requirements
- FR-2.1.1: Support metadata filtering in similarity search
- FR-2.1.2: Support namespace/collection isolation
- FR-2.1.3: Batch insert with configurable batch sizes
- FR-2.1.4: Upsert capability (update if exists)
- FR-2.1.5: Hybrid search support (dense + sparse)

### 2.2 In-Memory Vector Store

**Priority:** P0 (Critical)
**Effort:** 6 hours
**Dependencies:** 2.1

#### Requirements
- Thread-safe operations
- Brute-force similarity search (for small datasets)
- Optional HNSW index for larger datasets
- Persistence to disk (JSON/binary)
- Useful for testing and development

### 2.3 PostgreSQL/pgvector Vector Store

**Priority:** P0 (Critical)
**Effort:** 12 hours
**Dependencies:** 2.1

#### Requirements
- Full pgvector extension support
- Index types: IVFFlat, HNSW
- Distance metrics: L2, cosine, inner product
- Connection pooling integration
- Schema migration support
- Metadata JSONB filtering

#### Schema
```sql
CREATE TABLE IF NOT EXISTS minion_embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB,
    embedding vector(1536),  -- configurable dimensions
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_embeddings_collection ON minion_embeddings(collection);
CREATE INDEX idx_embeddings_metadata ON minion_embeddings USING gin(metadata);
CREATE INDEX idx_embeddings_vector ON minion_embeddings USING hnsw (embedding vector_cosine_ops);
```

### 2.4 Additional Vector Stores

**Priority:** P1 (High)
**Effort:** 32 hours total

| Vector Store | Features | Effort |
|--------------|----------|--------|
| Chroma | Local/cloud, collections, metadata filtering | 8h |
| Pinecone | Serverless, namespaces, metadata filtering | 8h |
| Qdrant | Local/cloud, payload filtering, hybrid search | 8h |
| Weaviate | GraphQL, hybrid search, multi-tenancy | 8h |

### 2.5 Redis Vector Store

**Priority:** P2 (Medium)
**Effort:** 8 hours
**Dependencies:** 2.1

#### Requirements
- Redis Stack with RediSearch module
- JSON document storage
- Tag and numeric filtering
- Hybrid search (FT.SEARCH)

### 2.6 Milvus Vector Store

**Priority:** P2 (Medium)
**Effort:** 8 hours
**Dependencies:** 2.1

#### Requirements
- Milvus 2.x API
- Collection and partition management
- Index types: IVF_FLAT, IVF_SQ8, HNSW
- Scalar filtering

---

## Phase 3: Document Loaders

### 3.1 Document Loader Interface

**Priority:** P0 (Critical)
**Effort:** 4 hours
**Dependencies:** None

#### Requirements

```go
// documentloaders/interface.go
type Loader interface {
    // Load returns documents from the source
    Load(ctx context.Context) ([]Document, error)

    // LoadAndSplit loads and splits documents
    LoadAndSplit(ctx context.Context, splitter TextSplitter) ([]Document, error)
}

type Document struct {
    PageContent string
    Metadata    map[string]interface{}
}
```

### 3.2 Text Loader

**Priority:** P0 (Critical)
**Effort:** 2 hours
**Dependencies:** 3.1

#### Requirements
- Load from io.Reader, file path, or URL
- Encoding detection (UTF-8, UTF-16, etc.)
- Line-based loading option

### 3.3 PDF Loader

**Priority:** P0 (Critical)
**Effort:** 8 hours
**Dependencies:** 3.1

#### Requirements
- Extract text from PDF files
- Page-by-page extraction with page number metadata
- Support password-protected PDFs
- OCR fallback option (via external service)
- Handle multi-column layouts

#### Metadata
```go
metadata := map[string]interface{}{
    "source":      "document.pdf",
    "page":        1,
    "total_pages": 10,
    "author":      "...",
    "title":       "...",
    "created":     "...",
}
```

### 3.4 CSV Loader

**Priority:** P0 (Critical)
**Effort:** 4 hours
**Dependencies:** 3.1

#### Requirements
- Configurable delimiter, quote character
- Header row handling
- Column selection/filtering
- Row-per-document or combined modes
- Large file streaming support

### 3.5 HTML Loader

**Priority:** P0 (Critical)
**Effort:** 4 hours
**Dependencies:** 3.1

#### Requirements
- Parse and sanitize HTML
- Extract text content
- Preserve semantic structure (headings, lists)
- Remove scripts, styles, navigation
- Handle relative URLs

### 3.6 Markdown Loader

**Priority:** P1 (High)
**Effort:** 4 hours
**Dependencies:** 3.1

#### Requirements
- Parse markdown to structured content
- Extract frontmatter metadata (YAML)
- Handle code blocks specially
- Support GFM (GitHub Flavored Markdown)

### 3.7 JSON/JSONL Loader

**Priority:** P1 (High)
**Effort:** 4 hours
**Dependencies:** 3.1

#### Requirements
- Load from JSON arrays or JSONL files
- Configurable content/metadata field mapping
- JQ-style path selectors
- Streaming for large files

### 3.8 Directory Loader

**Priority:** P1 (High)
**Effort:** 6 hours
**Dependencies:** 3.2-3.7

#### Requirements
- Recursively load from directory
- Configurable file extensions
- Glob pattern support
- Max depth configuration
- Automatic loader selection by extension
- Progress callback for large directories

### 3.9 Web/URL Loader

**Priority:** P1 (High)
**Effort:** 6 hours
**Dependencies:** 3.5

#### Requirements
- Fetch and parse web pages
- Follow redirects
- Respect robots.txt (optional)
- Rate limiting
- User-agent configuration
- Cookie/auth support

### 3.10 Additional Loaders

**Priority:** P2 (Medium)
**Effort:** 20 hours total

| Loader | Description | Effort |
|--------|-------------|--------|
| Word/DOCX | Microsoft Word documents | 6h |
| Excel/XLSX | Spreadsheet files | 6h |
| PowerPoint/PPTX | Presentation slides | 4h |
| Audio Transcript | Via Whisper/AssemblyAI | 4h |

---

## Phase 4: Text Splitters

### 4.1 Text Splitter Interface

**Priority:** P0 (Critical)
**Effort:** 4 hours
**Dependencies:** None

#### Requirements

```go
// textsplitter/interface.go
type TextSplitter interface {
    // SplitText splits text into chunks
    SplitText(text string) ([]string, error)

    // SplitDocuments splits documents preserving metadata
    SplitDocuments(docs []Document) ([]Document, error)
}

type Options struct {
    ChunkSize      int      // target chunk size in characters
    ChunkOverlap   int      // overlap between chunks
    Separators     []string // split separators in order
    LengthFunction func(string) int // custom length function
    KeepSeparator  bool
}
```

### 4.2 Recursive Character Splitter

**Priority:** P0 (Critical)
**Effort:** 6 hours
**Dependencies:** 4.1

#### Requirements
- Recursively split by separators: `\n\n`, `\n`, ` `, ``
- Configurable chunk size and overlap
- Preserve semantic boundaries where possible
- Language-aware defaults (code vs prose)

### 4.3 Token-Based Splitter

**Priority:** P0 (Critical)
**Effort:** 8 hours
**Dependencies:** 4.1

#### Requirements
- Split based on token count (not characters)
- Support tiktoken for OpenAI models
- Support HuggingFace tokenizers
- Model-specific tokenization

### 4.4 Markdown Splitter

**Priority:** P1 (High)
**Effort:** 6 hours
**Dependencies:** 4.1

#### Requirements
- Split on markdown headers
- Preserve heading hierarchy in metadata
- Handle code blocks as atomic units
- Support configurable header levels

### 4.5 Code Splitter

**Priority:** P1 (High)
**Effort:** 8 hours
**Dependencies:** 4.1

#### Requirements
- Language-aware splitting (Go, Python, JS, etc.)
- Split on function/class boundaries
- Preserve imports and dependencies
- Handle nested structures

### 4.6 Semantic Splitter

**Priority:** P2 (Medium)
**Effort:** 12 hours
**Dependencies:** 4.1, 1.1

#### Requirements
- Use embeddings to find semantic breakpoints
- Combine similar sentences into chunks
- Configurable similarity threshold
- More expensive but higher quality

---

## Phase 5: Retrievers

### 5.1 Retriever Interface

**Priority:** P0 (Critical)
**Effort:** 4 hours
**Dependencies:** 2.1

#### Requirements

```go
// retrievers/interface.go
type Retriever interface {
    // GetRelevantDocuments retrieves documents for a query
    GetRelevantDocuments(ctx context.Context, query string) ([]Document, error)
}

type RetrieverOptions struct {
    TopK           int
    ScoreThreshold float32
    Filter         map[string]interface{}
}
```

### 5.2 Vector Store Retriever

**Priority:** P0 (Critical)
**Effort:** 4 hours
**Dependencies:** 5.1, 2.1

#### Requirements
- Wrap any VectorStore as a Retriever
- Configurable top-k
- Score threshold filtering
- Metadata filtering pass-through

### 5.3 Multi-Query Retriever

**Priority:** P1 (High)
**Effort:** 8 hours
**Dependencies:** 5.1

#### Requirements
- Use LLM to generate multiple query variations
- Execute all queries in parallel
- Deduplicate and merge results
- Configurable number of query variations

### 5.4 Self-Query Retriever

**Priority:** P1 (High)
**Effort:** 12 hours
**Dependencies:** 5.1

#### Requirements
- Use LLM to extract structured query + filters
- Support metadata attribute definitions
- Generate vector store filters from natural language
- Fallback to pure semantic search

### 5.5 Contextual Compression Retriever

**Priority:** P2 (Medium)
**Effort:** 8 hours
**Dependencies:** 5.1

#### Requirements
- Compress/filter retrieved documents
- LLM-based extraction of relevant portions
- Reduce context window usage
- Configurable compression strategy

### 5.6 Ensemble Retriever

**Priority:** P2 (Medium)
**Effort:** 6 hours
**Dependencies:** 5.1

#### Requirements
- Combine multiple retrievers
- Reciprocal Rank Fusion (RRF) scoring
- Weighted combination
- Configurable per-retriever weights

### 5.7 Parent Document Retriever

**Priority:** P2 (Medium)
**Effort:** 8 hours
**Dependencies:** 5.1, 4.1

#### Requirements
- Index small chunks for retrieval
- Return parent (larger) documents
- Document hierarchy tracking
- Storage for parent-child relationships

---

## Phase 6: Chains Framework

### 6.1 Chain Interface

**Priority:** P0 (Critical)
**Effort:** 8 hours
**Dependencies:** None

#### Requirements

```go
// chains/interface.go
type Chain interface {
    // Call executes the chain
    Call(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error)

    // InputKeys returns required input keys
    InputKeys() []string

    // OutputKeys returns output keys
    OutputKeys() []string

    // Name returns chain name for tracing
    Name() string
}

type ChainCallbacks interface {
    OnChainStart(ctx context.Context, inputs map[string]interface{})
    OnChainEnd(ctx context.Context, outputs map[string]interface{})
    OnChainError(ctx context.Context, err error)
}
```

### 6.2 LLM Chain

**Priority:** P0 (Critical)
**Effort:** 6 hours
**Dependencies:** 6.1

#### Requirements
- Basic prompt → LLM → output chain
- Support prompt templates
- Output parsing integration
- Streaming support

### 6.3 Retrieval QA Chain

**Priority:** P0 (Critical)
**Effort:** 10 hours
**Dependencies:** 6.1, 5.1

#### Requirements
- Query → Retrieve → Augment → Generate
- Configurable retriever
- Configurable prompt template
- Source document tracking
- Stuff, Map-Reduce, Refine strategies

### 6.4 Conversational Retrieval Chain

**Priority:** P1 (High)
**Effort:** 10 hours
**Dependencies:** 6.3

#### Requirements
- Memory integration for conversation history
- Question condensing (standalone question generation)
- Multi-turn conversations
- Session management integration

### 6.5 Sequential Chain

**Priority:** P1 (High)
**Effort:** 6 hours
**Dependencies:** 6.1

#### Requirements
- Execute chains in sequence
- Pass outputs to next chain inputs
- Named input/output mapping
- Error handling and short-circuit

### 6.6 Router Chain

**Priority:** P2 (Medium)
**Effort:** 8 hours
**Dependencies:** 6.1

#### Requirements
- Route to different chains based on input
- LLM-based routing
- Rule-based routing
- Default fallback chain

---

## Phase 7: Prompt Templates

### 7.1 Prompt Template Interface

**Priority:** P0 (Critical)
**Effort:** 4 hours
**Dependencies:** None

#### Requirements

```go
// prompts/interface.go
type PromptTemplate interface {
    // Format formats the template with variables
    Format(variables map[string]interface{}) (string, error)

    // InputVariables returns required variables
    InputVariables() []string

    // PartialFormat returns a new template with some variables filled
    PartialFormat(variables map[string]interface{}) (PromptTemplate, error)
}
```

### 7.2 String Prompt Template

**Priority:** P0 (Critical)
**Effort:** 4 hours
**Dependencies:** 7.1

#### Requirements
- Go template syntax support
- Variable validation
- Default values
- Partial formatting

### 7.3 Chat Prompt Template

**Priority:** P0 (Critical)
**Effort:** 6 hours
**Dependencies:** 7.1

#### Requirements
- System, Human, AI message templates
- Message placeholder support
- History injection
- Role-based formatting

### 7.4 Few-Shot Prompt Template

**Priority:** P1 (High)
**Effort:** 8 hours
**Dependencies:** 7.1

#### Requirements
- Example selector integration
- Dynamic example selection
- Formatting examples with template
- Max examples configuration

---

## Phase 8: Output Parsers

### 8.1 Output Parser Interface

**Priority:** P0 (Critical)
**Effort:** 4 hours
**Dependencies:** None

#### Requirements

```go
// outputparsers/interface.go
type OutputParser[T any] interface {
    // Parse parses LLM output into structured format
    Parse(text string) (T, error)

    // GetFormatInstructions returns instructions for the LLM
    GetFormatInstructions() string

    // ParseWithPrompt parses with access to original prompt
    ParseWithPrompt(text string, prompt string) (T, error)
}
```

### 8.2 JSON Output Parser

**Priority:** P0 (Critical)
**Effort:** 6 hours
**Dependencies:** 8.1

#### Requirements
- Parse JSON from LLM output
- Handle markdown code blocks
- Schema validation
- Partial JSON recovery

### 8.3 Structured Output Parser

**Priority:** P0 (Critical)
**Effort:** 8 hours
**Dependencies:** 8.1

#### Requirements
- Parse into Go structs
- JSON Schema generation from struct tags
- Field-level validation
- Nested struct support

### 8.4 List Output Parser

**Priority:** P1 (High)
**Effort:** 4 hours
**Dependencies:** 8.1

#### Requirements
- Parse comma/newline separated lists
- Configurable separators
- Trim and clean items
- Numbered list support

### 8.5 Retry Parser

**Priority:** P1 (High)
**Effort:** 6 hours
**Dependencies:** 8.1

#### Requirements
- Wrap other parsers
- Retry with LLM on parse failure
- Include error in retry prompt
- Max retry configuration

---

## Phase 9: Additional LLM Providers

### 9.1 Google AI (Gemini)

**Priority:** P1 (High)
**Effort:** 8 hours
**Dependencies:** Existing LLM interface

#### Requirements
- Support Gemini Pro, Gemini Ultra
- Multimodal support (text + images)
- Function calling
- Streaming responses

### 9.2 AWS Bedrock

**Priority:** P1 (High)
**Effort:** 12 hours
**Dependencies:** Existing LLM interface

#### Requirements
- Support Claude, Llama, Titan models
- AWS credential integration
- Cross-region support
- Model inference profiles

### 9.3 Cohere

**Priority:** P2 (Medium)
**Effort:** 6 hours
**Dependencies:** Existing LLM interface

#### Requirements
- Support Command models
- RAG-optimized endpoints
- Rerank API integration

### 9.4 Mistral AI

**Priority:** P2 (Medium)
**Effort:** 6 hours
**Dependencies:** Existing LLM interface

#### Requirements
- Support Mistral models
- Function calling
- JSON mode

### 9.5 Groq

**Priority:** P2 (Medium)
**Effort:** 4 hours
**Dependencies:** Existing LLM interface

#### Requirements
- Ultra-low latency inference
- OpenAI-compatible API
- Model selection (Llama, Mixtral)

---

## Phase 10: Integration & Polish

### 10.1 RAG Pipeline Builder

**Priority:** P1 (High)
**Effort:** 12 hours
**Dependencies:** Phases 1-8

#### Requirements

```go
// rag/pipeline.go
type RAGPipeline struct {
    Loader      Loader
    Splitter    TextSplitter
    Embedder    Embedder
    VectorStore VectorStore
    Retriever   Retriever
    Chain       Chain
}

func NewRAGPipeline() *RAGPipelineBuilder {
    return &RAGPipelineBuilder{}
}

// Fluent builder pattern
pipeline := rag.NewRAGPipeline().
    WithLoader(documentloaders.NewPDF("docs/")).
    WithSplitter(textsplitter.NewRecursive(1000, 200)).
    WithEmbedder(embeddings.NewOpenAI(apiKey)).
    WithVectorStore(vectorstore.NewPgVector(db)).
    Build()

// Ingest documents
err := pipeline.Ingest(ctx)

// Query
answer, sources, err := pipeline.Query(ctx, "What is...?")
```

### 10.2 Streaming Support

**Priority:** P1 (High)
**Effort:** 10 hours
**Dependencies:** 6.1

#### Requirements
- Token-by-token streaming from chains
- Streaming callbacks
- Stream to HTTP response
- Partial result handling

### 10.3 Observability Integration

**Priority:** P0 (Critical)
**Effort:** 8 hours
**Dependencies:** All phases

#### Requirements
- Prometheus metrics for all new components
- OpenTelemetry tracing spans
- Cost tracking for embeddings
- Latency histograms

### 10.4 Testing Infrastructure

**Priority:** P0 (Critical)
**Effort:** 16 hours
**Dependencies:** All phases

#### Requirements
- Unit tests for all components (80%+ coverage)
- Integration tests with real providers
- Mock implementations for testing
- Benchmark tests for performance

---

## Implementation Roadmap

### Sprint 1: Foundation (Weeks 1-2)
- Phase 1: Embeddings Infrastructure
- Phase 4: Text Splitters (4.1-4.3)
- Phase 3: Document Loaders (3.1-3.5)

### Sprint 2: Vector Storage (Weeks 3-4)
- Phase 2: Vector Store Infrastructure
- Phase 5: Retrievers (5.1-5.3)

### Sprint 3: Chains & RAG (Weeks 5-6)
- Phase 6: Chains Framework
- Phase 7: Prompt Templates
- Phase 8: Output Parsers

### Sprint 4: Additional Providers (Weeks 7-8)
- Phase 9: Additional LLM Providers
- Phase 3: Additional Document Loaders (3.6-3.10)
- Phase 4: Additional Text Splitters (4.4-4.6)

### Sprint 5: Polish & Integration (Weeks 9-10)
- Phase 10: Integration & Polish
- Phase 5: Advanced Retrievers (5.4-5.7)
- Phase 2: Additional Vector Stores (2.5-2.6)

---

## Effort Estimation Summary

| Phase | Description | Effort (hours) |
|-------|-------------|----------------|
| 1 | Embeddings Infrastructure | 34 |
| 2 | Vector Store Infrastructure | 76 |
| 3 | Document Loaders | 56 |
| 4 | Text Splitters | 44 |
| 5 | Retrievers | 50 |
| 6 | Chains Framework | 48 |
| 7 | Prompt Templates | 22 |
| 8 | Output Parsers | 28 |
| 9 | Additional LLM Providers | 36 |
| 10 | Integration & Polish | 46 |
| **Total** | | **440 hours** |

**Estimated Duration:** 10-12 weeks with 1-2 developers

---

## Technical Architecture

### Package Structure

```
minion/
├── embeddings/
│   ├── interface.go
│   ├── openai.go
│   ├── ollama.go
│   ├── cohere.go
│   └── huggingface.go
├── vectorstore/
│   ├── interface.go
│   ├── memory.go
│   ├── pgvector.go
│   ├── chroma.go
│   ├── pinecone.go
│   └── qdrant.go
├── documentloaders/
│   ├── interface.go
│   ├── text.go
│   ├── pdf.go
│   ├── csv.go
│   ├── html.go
│   ├── markdown.go
│   ├── json.go
│   └── directory.go
├── textsplitter/
│   ├── interface.go
│   ├── recursive.go
│   ├── token.go
│   ├── markdown.go
│   └── code.go
├── retrievers/
│   ├── interface.go
│   ├── vectorstore.go
│   ├── multiquery.go
│   ├── selfquery.go
│   └── ensemble.go
├── chains/
│   ├── interface.go
│   ├── llm.go
│   ├── retrieval_qa.go
│   ├── conversational.go
│   └── sequential.go
├── prompts/
│   ├── interface.go
│   ├── template.go
│   ├── chat.go
│   └── fewshot.go
├── outputparsers/
│   ├── interface.go
│   ├── json.go
│   ├── structured.go
│   └── list.go
└── rag/
    ├── pipeline.go
    └── builder.go
```

### Integration with Existing Minion

```
┌─────────────────────────────────────────────────────────────┐
│                      Minion Framework                        │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   Agents    │  │  Behaviors  │  │   Multi-Agent       │  │
│  │  (existing) │  │  (existing) │  │   (existing)        │  │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘  │
│         │                │                     │             │
│         ▼                ▼                     ▼             │
│  ┌─────────────────────────────────────────────────────────┐│
│  │                    RAG Pipeline (NEW)                    ││
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────┐  ││
│  │  │ Loaders  │→│Splitters │→│Embeddings│→│VectorStore │  ││
│  │  └──────────┘ └──────────┘ └──────────┘ └────────────┘  ││
│  │                      │                         │         ││
│  │                      ▼                         ▼         ││
│  │               ┌──────────┐              ┌───────────┐    ││
│  │               │  Chains  │←─────────────│ Retriever │    ││
│  │               └──────────┘              └───────────┘    ││
│  └─────────────────────────────────────────────────────────┘│
│         │                │                     │             │
│         ▼                ▼                     ▼             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │    Tools    │  │Observability│  │    Resilience       │  │
│  │  (existing) │  │  (existing) │  │    (existing)       │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## Risk Assessment

### Technical Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| pgvector performance at scale | High | Medium | Benchmark early, consider dedicated vector DBs |
| Token counting accuracy | Medium | Low | Use official tiktoken, test extensively |
| PDF parsing quality | Medium | Medium | Offer OCR fallback, document limitations |
| Memory usage with large documents | High | Medium | Streaming, chunked processing |

### Operational Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Breaking changes to provider APIs | Medium | Medium | Abstraction layers, version pinning |
| Cost overruns from embeddings | High | Medium | Caching, cost tracking, alerts |
| Complexity of testing RAG | Medium | High | Golden datasets, automated evaluation |

---

## Success Criteria

### Phase Completion Criteria

1. **All interfaces defined and documented**
2. **Core implementations complete with 80%+ test coverage**
3. **Integration tests passing**
4. **Metrics and tracing integrated**
5. **API documentation complete**
6. **Example applications working**

### Feature Parity Validation

- [ ] Can build a complete RAG application
- [ ] Can ingest documents from multiple sources
- [ ] Can query with semantic search
- [ ] Can use multiple vector stores interchangeably
- [ ] Chains integrate with existing agent behaviors
- [ ] Observability covers all new components

---

## Appendix A: LangChainGo Feature Matrix

| Feature | LangChainGo | Minion (Current) | Minion (Target) |
|---------|-------------|------------------|-----------------|
| OpenAI | ✅ | ✅ | ✅ |
| Anthropic | ✅ | ✅ | ✅ |
| Ollama | ✅ | ✅ | ✅ |
| Google AI | ✅ | ❌ | ✅ |
| AWS Bedrock | ✅ | ❌ | ✅ |
| Cohere | ✅ | ❌ | ✅ |
| Embeddings | ✅ | ❌ | ✅ |
| Vector Stores | ✅ | ❌ | ✅ |
| Document Loaders | ✅ | ❌ | ✅ |
| Text Splitters | ✅ | ❌ | ✅ |
| Retrievers | ✅ | ❌ | ✅ |
| Chains | ✅ | ❌ | ✅ |
| Output Parsers | ✅ | ❌ | ✅ |
| Multi-Agent | Basic | ✅ Advanced | ✅ Advanced |
| Observability | Basic | ✅ Advanced | ✅ Advanced |
| MCP Protocol | ❌ | ✅ | ✅ |
| Domain Tools (80+) | ❌ | ✅ | ✅ |

---

## Appendix B: Example Usage

### Complete RAG Application

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/Ranganaths/minion/documentloaders"
    "github.com/Ranganaths/minion/embeddings"
    "github.com/Ranganaths/minion/vectorstore"
    "github.com/Ranganaths/minion/retrievers"
    "github.com/Ranganaths/minion/chains"
    "github.com/Ranganaths/minion/textsplitter"
    "github.com/Ranganaths/minion/llm"
)

func main() {
    ctx := context.Background()

    // 1. Load documents
    loader := documentloaders.NewDirectory("./docs",
        documentloaders.WithExtensions(".pdf", ".md", ".txt"),
        documentloaders.WithRecursive(true),
    )
    docs, err := loader.Load(ctx)
    if err != nil {
        log.Fatal(err)
    }

    // 2. Split documents
    splitter := textsplitter.NewRecursive(
        textsplitter.WithChunkSize(1000),
        textsplitter.WithChunkOverlap(200),
    )
    chunks, err := splitter.SplitDocuments(docs)
    if err != nil {
        log.Fatal(err)
    }

    // 3. Create embeddings
    embedder := embeddings.NewOpenAI(
        embeddings.WithModel("text-embedding-3-small"),
    )

    // 4. Store in vector database
    store, err := vectorstore.NewPgVector(db,
        vectorstore.WithCollection("my-docs"),
        vectorstore.WithEmbedder(embedder),
    )
    if err != nil {
        log.Fatal(err)
    }

    ids, err := store.AddDocuments(ctx, chunks, embedder)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Indexed %d chunks\n", len(ids))

    // 5. Create retriever
    retriever := store.AsRetriever(
        retrievers.WithTopK(5),
        retrievers.WithScoreThreshold(0.7),
    )

    // 6. Create RAG chain
    llmProvider := llm.NewOpenAI("gpt-4")
    ragChain := chains.NewRetrievalQA(
        chains.WithLLM(llmProvider),
        chains.WithRetriever(retriever),
        chains.WithReturnSources(true),
    )

    // 7. Query
    result, err := ragChain.Call(ctx, map[string]interface{}{
        "question": "What are the main features of the system?",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Answer:", result["answer"])
    fmt.Println("Sources:", result["sources"])
}
```

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-12-20 | Minion Team | Initial PRD |

---

*End of Document*
