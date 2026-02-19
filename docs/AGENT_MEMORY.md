# Agent Memory Architecture

This document describes how agents persist context and memory in Minion, including current implementations, storage options, and recommendations for different use cases.

## Overview

Agent memory in Minion serves three primary purposes:

1. **Session Context** - Active conversation history and working state
2. **Execution Snapshots** - Point-in-time captures for debugging and time-travel
3. **Long-term Memory** - Semantic memories and knowledge for cross-session learning

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Agent Memory Architecture                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐          │
│  │   Context    │    │  Execution   │    │  Long-term   │          │
│  │   Window     │    │  Snapshots   │    │   Memory     │          │
│  │              │    │              │    │              │          │
│  │ - Messages   │    │ - Session    │    │ - Semantic   │          │
│  │ - Tools      │    │ - Task       │    │ - Knowledge  │          │
│  │ - Documents  │    │ - Workspace  │    │   Graph      │          │
│  │ - System     │    │ - Actions    │    │ - Entities   │          │
│  └──────────────┘    └──────────────┘    └──────────────┘          │
│         │                   │                   │                   │
│         ▼                   ▼                   ▼                   │
│  ┌──────────────────────────────────────────────────────┐          │
│  │                   Storage Layer                       │          │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐ │          │
│  │  │In-Memory│  │PostgreSQL│ │  Redis  │  │VectorDB │ │          │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘ │          │
│  └──────────────────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────────────────┘
```

## Current Implementations

### 1. Context Management (`memory/context.go`)

The `ContextManager` manages active context windows for agents with token-aware eviction.

```go
type ContextManager struct {
    windows        map[string]*ContextWindow
    semanticMemory *SemanticMemory
    knowledgeGraph *KnowledgeGraph
    tokenEstimator TokenEstimator
    summarizer     Summarizer
}
```

**Context Window Features:**
- Token-limited sliding window
- Priority-based eviction (lowest priority items evicted first)
- Automatic compression via summarization before eviction
- Integration with semantic memory for context enrichment

**Context Item Types:**
| Type | Description |
|------|-------------|
| `message` | Conversation messages |
| `memory` | Retrieved semantic memories |
| `document` | Knowledge graph entities |
| `tool` | Tool call results |
| `system` | System prompts (never evicted) |

**Usage:**
```go
cm := memory.NewContextManager(memory.ContextManagerConfig{
    SemanticMemory: semanticMem,
    KnowledgeGraph: kg,
    TokenEstimator: memory.NewSimpleTokenEstimator(),
    Summarizer:     summarizer,
})

window := cm.CreateWindow(agentID, 4096) // 4k token limit

cm.AddItem(ctx, window.ID, &memory.ContextItem{
    Type:     memory.ContextItemTypeMessage,
    Content:  "User message here",
    Priority: 1.0,
})

// Enrich with relevant memories
cm.EnrichContext(ctx, window.ID, "search query")
```

### 2. Conversation Memory (`memory/buffer.go`)

Three conversation memory implementations:

#### ConversationBufferMemory
Stores full conversation history in memory.

```go
mem := memory.NewConversationBufferMemory()
mem.SaveContext(ctx, inputs, outputs)
vars, _ := mem.LoadMemoryVariables(ctx)
```

#### ConversationBufferWindowMemory
Keeps only the last K conversation turns.

```go
mem := memory.NewConversationBufferWindowMemory(5) // Last 5 turns
```

#### ConversationSummaryMemory
Maintains a running summary instead of full history.

```go
mem := memory.NewConversationSummaryMemory(memory.ConversationSummaryMemoryConfig{
    SummarizeFunc: func(ctx context.Context, existing string, new []ChatMessage) (string, error) {
        // Use LLM to summarize
        return summarizedText, nil
    },
})
```

### 3. Execution Snapshots (`debug/snapshot/`)

Captures complete agent state at checkpoints for debugging and time-travel.

#### ExecutionSnapshot Structure

```go
type ExecutionSnapshot struct {
    // Identity
    ID          string
    ExecutionID string

    // Ordering
    SequenceNum int64
    Timestamp   time.Time

    // Checkpoint type
    CheckpointType CheckpointType

    // Context identifiers
    AgentID   string
    TaskID    string
    SessionID string

    // State captures
    SessionState   *SessionSnapshot   // Full conversation history
    TaskState      *TaskSnapshot      // Current task details
    WorkspaceState map[string]any     // Agent workspace

    // Execution context
    Action *ActionSnapshot
    Input  any
    Output any
    Error  *ErrorSnapshot

    // Observability
    TraceID      string
    SpanID       string
}
```

#### Checkpoint Types

| Category | Checkpoint Types |
|----------|-----------------|
| Task Lifecycle | `task_created`, `task_assigned`, `task_started`, `task_completed`, `task_failed`, `task_retry` |
| Tool Execution | `tool_call_start`, `tool_call_end` |
| LLM Calls | `llm_call_start`, `llm_call_end` |
| Agent Steps | `agent_step`, `agent_plan`, `agent_action`, `decision_point` |
| State Changes | `state_change`, `session_update`, `workspace_update` |
| Communication | `message_sent`, `message_received` |
| User Interaction | `user_input`, `user_output` |
| Errors | `error` |

#### SnapshotStore Interface

```go
type SnapshotStore interface {
    // Write operations
    Save(ctx context.Context, snapshot *ExecutionSnapshot) error
    SaveBatch(ctx context.Context, snapshots []*ExecutionSnapshot) error

    // Read operations
    Get(ctx context.Context, snapshotID string) (*ExecutionSnapshot, error)
    GetByExecution(ctx context.Context, executionID string) ([]*ExecutionSnapshot, error)
    GetLatest(ctx context.Context, executionID string) (*ExecutionSnapshot, error)
    GetAtSequence(ctx context.Context, executionID string, seqNum int64) (*ExecutionSnapshot, error)

    // Query operations
    Query(ctx context.Context, query *SnapshotQuery) (*SnapshotQueryResult, error)
    ListExecutions(ctx context.Context, limit, offset int) ([]*ExecutionSummary, error)

    // Maintenance
    PurgeOlderThan(ctx context.Context, age time.Duration) (int64, error)
    PurgeExecution(ctx context.Context, executionID string) (int64, error)

    Close() error
}
```

### 4. Storage Implementations

#### In-Memory Store (`debug/snapshot/store_memory.go`)

Best for: Development, testing, single-process deployments

```go
store := snapshot.NewMemorySnapshotStore(
    snapshot.WithMaxSnapshots(100000),
    snapshot.WithRetentionPeriod(7 * 24 * time.Hour),
)
```

**Features:**
- Zero configuration
- Multiple indexes (by execution, agent, task, time)
- Automatic eviction when limit reached
- ~2KB per snapshot memory footprint

**Limitations:**
- Data lost on restart
- Single-process only
- Memory-bound

#### PostgreSQL Store (`debug/snapshot/store_postgres.go`)

Best for: Production deployments, durability requirements

```go
store, err := snapshot.NewPostgresSnapshotStore(snapshot.PostgresConfig{
    Host:            "localhost",
    Port:            5432,
    Database:        "minion_debug",
    User:            "minion",
    MaxOpenConns:    25,
    RetentionPeriod: 7 * 24 * time.Hour,
})
```

**Features:**
- ACID compliance
- Prepared statements for performance
- JSONB columns for flexible state storage
- Comprehensive indexing
- Automatic schema migration

**Schema:**
```sql
CREATE TABLE execution_snapshots (
    id              UUID PRIMARY KEY,
    execution_id    VARCHAR(255) NOT NULL,
    sequence_num    BIGINT NOT NULL,
    timestamp       TIMESTAMPTZ NOT NULL,
    checkpoint_type VARCHAR(50) NOT NULL,
    agent_id        VARCHAR(255),
    task_id         VARCHAR(255),
    session_state   JSONB,
    task_state      JSONB,
    workspace_state JSONB,
    action          JSONB,
    error           JSONB,
    metadata        JSONB,

    CONSTRAINT unique_execution_sequence UNIQUE (execution_id, sequence_num)
);
```

### 5. Semantic Memory (`memory/semantic.go`)

Vector-based memory for similarity search and retrieval.

```go
type SemanticMemory struct {
    vectorStore VectorStore
    embedder    Embedder
}

// Store a memory
mem.Store(ctx, agentID, "Important fact about user preferences", metadata)

// Recall relevant memories
results, _ := mem.Recall(ctx, agentID, "What does the user prefer?", topK)
```

### 6. Knowledge Graph (`memory/knowledge_graph.go`)

Entity and relationship storage for structured knowledge.

```go
kg := memory.NewKnowledgeGraph(config)

// Add entities
kg.AddEntity(ctx, entity)

// Add relationships
kg.AddRelationship(ctx, fromID, "works_at", toID, metadata)

// Find similar entities
entities, _ := kg.FindSimilarEntities(ctx, query, limit, threshold)
```

### 7. AG-UI State Management (`protocols/agui/state.go`)

Real-time state synchronization for UI integration.

```go
sm := agui.NewStateManager(emitter)

// Set state (emits delta to UI)
sm.Set("user.name", "Alice")

// Apply JSON Patch
sm.Patch([]JSONPatchOperation{
    {Op: "add", Path: "/tasks/0", Value: newTask},
})

// Emit full snapshot
sm.EmitSnapshot()
```

## Vector Store Options (`vectorstore/`)

### Available Implementations

| Store | File | Best For |
|-------|------|----------|
| In-Memory | `vectorstore/memory.go` | Testing, small datasets |
| pgvector | `vectorstore/pgvector.go` | PostgreSQL users, moderate scale |
| Pinecone | `vectorstore/pinecone.go` | Managed service, large scale |
| **Qdrant** | `vectorstore/qdrant.go` | Self-hosted, filtering, high performance |
| Hybrid | `vectorstore/hybrid.go` | Combined keyword + vector search |

### VectorStore Interface

```go
type VectorStore interface {
    AddDocuments(ctx context.Context, docs []Document) ([]string, error)
    SimilaritySearch(ctx context.Context, query string, k int) ([]Document, error)
    SimilaritySearchWithScore(ctx context.Context, query string, k int) ([]SearchResult, error)
    Delete(ctx context.Context, ids []string) error
}
```

## Caching Layer (`cache/`)

### Redis LLM Cache

Distributed caching for LLM responses with semantic matching.

```go
cache, _ := cache.NewRedisLLMCache(cache.RedisLLMCacheConfig{
    Client:        redisClient,
    Prefix:        "llm_cache:",
    TTL:           24 * time.Hour,
    Embedder:      embedder,       // For semantic matching
    MinSimilarity: 0.95,
})

// Check cache (exact + semantic)
entry, ok := cache.GetSemantic(ctx, prompt, model, embedding, 0.95)

// Store response
cache.Set(ctx, &LLMCacheEntry{
    Prompt:   prompt,
    Model:    model,
    Response: response,
})
```

## Recommended Architecture by Use Case

### Development / Single Agent

```
┌─────────────────────────────────────────┐
│            Single Process                │
├─────────────────────────────────────────┤
│  ┌─────────────────────────────────┐   │
│  │     In-Memory Everything         │   │
│  │  - MemorySnapshotStore          │   │
│  │  - ConversationBufferMemory     │   │
│  │  - MemoryVectorStore            │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

### Production / Multi-Agent

```
┌─────────────────────────────────────────────────────────────────┐
│                     Production Architecture                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Hot Path (Sub-ms latency)                                      │
│  ┌────────────────────────────────────────────────────────┐    │
│  │                        Redis                            │    │
│  │  - Active session context                               │    │
│  │  - Recent conversation turns                            │    │
│  │  - LLM response cache                                   │    │
│  │  - Inter-agent messaging (pub/sub)                      │    │
│  └────────────────────────────────────────────────────────┘    │
│                              │                                   │
│                              ▼                                   │
│  Warm Path (10-100ms latency)                                   │
│  ┌────────────────────────────────────────────────────────┐    │
│  │                     PostgreSQL                          │    │
│  │  - Execution snapshots                                  │    │
│  │  - Structured task/session history                      │    │
│  │  - Agent configurations                                 │    │
│  │  - Audit trails                                         │    │
│  └────────────────────────────────────────────────────────┘    │
│                              │                                   │
│                              ▼                                   │
│  Cold Path (50-200ms latency)                                   │
│  ┌────────────────────────────────────────────────────────┐    │
│  │              Vector Database (Qdrant/Pinecone)          │    │
│  │  - Long-term semantic memories                          │    │
│  │  - Knowledge graph embeddings                           │    │
│  │  - Cross-session learning                               │    │
│  │  - Document retrieval (RAG)                             │    │
│  └────────────────────────────────────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Edge / Embedded Deployments

```
┌─────────────────────────────────────────┐
│           Embedded Architecture          │
├─────────────────────────────────────────┤
│  ┌─────────────────────────────────┐   │
│  │           SQLite                 │   │
│  │  - Snapshots                     │   │
│  │  - Conversation history          │   │
│  │  - Agent state                   │   │
│  └─────────────────────────────────┘   │
│                  +                       │
│  ┌─────────────────────────────────┐   │
│  │        sqlite-vec / FAISS       │   │
│  │  - Local vector search          │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

## Storage Selection Guide

| Requirement | Recommended Storage |
|-------------|-------------------|
| Fast session access | Redis |
| Durable history | PostgreSQL |
| Semantic search | Vector DB (Qdrant, Pinecone, pgvector) |
| Offline/embedded | SQLite + sqlite-vec |
| Large scale RAG | Pinecone, Weaviate, Milvus |
| Hybrid search | Weaviate, Hybrid VectorStore |
| Graph queries | Neo4j, or PostgreSQL with recursive CTEs |

## New Implementations

The following storage backends have been implemented:

### 1. RedisChatMessageHistory (`memory/redis_history.go`)

Distributed conversation history using Redis lists.

```go
history, _ := memory.NewRedisChatMessageHistory(memory.RedisChatMessageHistoryConfig{
    Client:    redisClient,
    SessionID: "session-123",
    TTL:       24 * time.Hour,
    MaxLength: 100, // Keep last 100 messages
})

// Add messages
history.AddUserMessage(ctx, "Hello!")
history.AddAIMessage(ctx, "Hi there!")

// Get all messages
messages, _ := history.Messages(ctx)

// Get last N messages
recent, _ := history.LastMessages(ctx, 10)

// Use with Memory interface
mem, _ := memory.NewRedisConversationBufferMemory(memory.RedisConversationBufferMemoryConfig{
    RedisChatMessageHistoryConfig: config,
})
vars, _ := mem.LoadMemoryVariables(ctx)
```

### 2. SQLiteSnapshotStore (`debug/snapshot/store_sqlite.go`)

Embedded persistent storage for execution snapshots.

```go
store, _ := snapshot.NewSQLiteSnapshotStore(snapshot.SQLiteConfig{
    Path:            "snapshots.db",  // Use ":memory:" for in-memory
    WALMode:         true,            // Better concurrency
    RetentionPeriod: 7 * 24 * time.Hour,
})
defer store.Close()

// Save snapshot
store.Save(ctx, &snapshot.ExecutionSnapshot{
    ExecutionID:    "exec-1",
    SequenceNum:    1,
    CheckpointType: snapshot.CheckpointAgentStep,
    AgentID:        "agent-1",
})

// Query snapshots
result, _ := store.Query(ctx, &snapshot.SnapshotQuery{
    Filter: snapshot.SnapshotFilter{AgentID: "agent-1"},
    Limit:  100,
})

// Maintenance
store.PurgeOlderThan(ctx, 30 * 24 * time.Hour)
store.Vacuum(ctx) // Reclaim space
```

### 3. QdrantVectorStore (`vectorstore/qdrant.go`)

High-performance vector search using Qdrant.

```go
store, _ := vectorstore.NewQdrantStore(vectorstore.QdrantConfig{
    Host:           "localhost",
    Port:           6333,
    CollectionName: "memories",
    Dimension:      1536,
    Distance:       vectorstore.QdrantDistanceCosine,
    HNSWConfig: &vectorstore.QdrantHNSWConfig{
        M:           16,
        EfConstruct: 100,
    },
}, embedder)

// Ensure collection exists
store.EnsureCollection(ctx)

// Add documents
ids, _ := store.AddDocuments(ctx, []vectorstore.Document{
    {PageContent: "Important information", Metadata: map[string]any{"type": "fact"}},
})

// Similarity search
results, _ := store.SimilaritySearchWithScore(ctx, "query", 10)

// Search with filters
docs, _ := store.SimilaritySearchWithFilters(ctx, "query", 10, []vectorstore.Filter{
    {Field: "type", Operator: vectorstore.FilterEquals, Value: "fact"},
})

// MMR search for diversity
diverse, _ := store.MaxMarginalRelevanceSearch(ctx, "query", 5, 20, 0.5)

// Snapshots for backup
snapshotName, _ := store.CreateSnapshot(ctx)
```

### 4. TieredMemoryManager (`memory/tiered.go`)

Automatic hot/warm/cold tiering with background compaction.

```go
// Create tiered manager
manager := memory.NewTieredMemoryManager(
    memory.NewMemoryHotStore(),  // Or Redis-backed
    memory.NewMemoryWarmStore(), // Or SnapshotStoreWarmAdapter
    nil,                         // Optional cold store
    memory.TieredMemoryConfig{
        HotTTL:             1 * time.Hour,
        WarmTTL:            7 * 24 * time.Hour,
        HotMaxItems:        10000,
        PromoteOnAccess:    true,
        CompactionInterval: 1 * time.Hour,
    },
)

// Start background workers
manager.Start()
defer manager.Close()

// Store items (automatically goes to hot tier)
manager.Set(ctx, &memory.TieredMemoryItem{
    Key:      "memory-1",
    Value:    []byte("important memory"),
    AgentID:  "agent-1",
    ItemType: "episodic",
})

// Get items (checks hot → warm → cold, auto-promotes)
item, _ := manager.Get(ctx, "memory-1")

// Query warm tier
results, _ := manager.Query(ctx, &memory.TieredMemoryQuery{
    AgentID:  "agent-1",
    ItemType: "episodic",
    Limit:    100,
})

// Get metrics
metrics := manager.GetMetrics()
fmt.Printf("Hot hits: %d, Warm hits: %d, Promotions: %d\n",
    metrics.HotHits, metrics.WarmHits, metrics.Promotions)
```

#### Using with Snapshot Store

```go
// Adapt existing snapshot store as warm tier
warmStore := memory.NewSnapshotStoreWarmAdapter(
    snapshot.NewPostgresSnapshotStore(config),
)

manager := memory.NewTieredMemoryManager(
    redisHotStore,
    warmStore,
    s3ColdStore,
    config,
)
```

## Best Practices

### 1. Context Window Management

```go
// Set appropriate priorities
item := &ContextItem{
    Type:     ContextItemTypeMessage,
    Content:  content,
    Priority: calculatePriority(recency, importance),
}

// System prompts should never be evicted
systemItem := &ContextItem{
    Type:     ContextItemTypeSystem,
    Content:  systemPrompt,
    Priority: 1000, // High priority
}
```

### 2. Snapshot Frequency

```go
// Snapshot at key decision points, not every step
recorder.Snapshot(ctx, ExecutionSnapshot{
    CheckpointType: CheckpointDecisionPoint,
    // ... state
})

// Batch snapshots for performance
store.SaveBatch(ctx, snapshots)
```

### 3. Memory Cleanup

```go
// Regular purging prevents unbounded growth
go func() {
    ticker := time.NewTicker(1 * time.Hour)
    for range ticker.C {
        store.PurgeOlderThan(ctx, 7*24*time.Hour)
    }
}()
```

### 4. Vector Store Indexing

```go
// Use appropriate index type for your scale
config := IndexConfig{
    Name:           "memories",
    Dimension:      1536,
    DistanceMetric: DistanceCosine,
    IndexType:      IndexTypeHNSW,
    HNSW: &HNSWConfig{
        M:              16,  // Connections per layer
        EfConstruction: 64,  // Build-time quality
        EfSearch:       40,  // Query-time quality
    },
}
```

## Related Documentation

- [Architecture](ARCHITECTURE.md) - Overall system architecture
- [API Reference](API_REFERENCE.md) - Complete API documentation
- [Protocols](PROTOCOLS.md) - A2A, AG-UI, MCP integration
