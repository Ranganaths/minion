# Multi-Agent Integration Guide for Chain/RAG Features

**Document Version:** 1.0
**Date:** 2025-12-20
**Status:** Critical - Must Follow

---

## Executive Summary

This document ensures that the new Chain and RAG packages integrate seamlessly with Minion's existing multi-agent system without breaking any functionality. The multi-agent system is a production-ready, Magentic-One based architecture with KQML protocols, and must remain fully operational.

---

## Multi-Agent Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    EXISTING MULTI-AGENT SYSTEM              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ Coordinator │  │Orchestrator │  │    Worker Pool      │  │
│  │  (API)      │  │ (LLM Plan)  │  │  (5 Built-in Types) │  │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘  │
│         │                │                     │             │
│         └────────────────┴─────────────────────┘             │
│                          │                                   │
│  ┌───────────────────────┼───────────────────────────────┐  │
│  │               PROTOCOL LAYER                           │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐             │  │
│  │  │In-Memory │  │  Redis   │  │  Kafka   │             │  │
│  │  └──────────┘  └──────────┘  └──────────┘             │  │
│  └───────────────────────────────────────────────────────┘  │
│                          │                                   │
│  ┌───────────────────────┼───────────────────────────────┐  │
│  │              SUPPORT SERVICES                          │  │
│  │  TaskLedger | ProgressLedger | LoadBalancer | Scaler  │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ MUST NOT BREAK
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                 NEW CHAIN/RAG PACKAGES                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   chain/    │  │ vectorstore/│  │    retriever/       │  │
│  │   rag/      │  │ embeddings/ │  │ documentloader/     │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## Critical Interfaces - DO NOT MODIFY

### 1. Protocol Interface (`core/multiagent/protocol.go`)

```go
// DO NOT CHANGE - All workers and orchestrator depend on this
type Protocol interface {
    Send(ctx context.Context, msg *Message) error
    Receive(ctx context.Context, agentID string) ([]*Message, error)
    Broadcast(ctx context.Context, msg *Message, groupID string) error
    Subscribe(ctx context.Context, agentID string, messageTypes []MessageType) error
    Unsubscribe(ctx context.Context, agentID string) error
}
```

### 2. TaskHandler Interface (`core/multiagent/workers.go`)

```go
// DO NOT CHANGE - All workers implement this
type TaskHandler interface {
    HandleTask(ctx context.Context, task *Task) (interface{}, error)
    GetCapabilities() []string
    GetName() string
}
```

### 3. Message Structure (`core/multiagent/protocol.go`)

```go
// DO NOT CHANGE - Orchestrator matches results by InReplyTo
type Message struct {
    ID        string
    Type      MessageType
    From      string
    To        string
    InReplyTo string  // CRITICAL: Used for task-result matching
    Content   interface{}
    Metadata  map[string]interface{}
    CreatedAt time.Time
}
```

### 4. Task Structure (`core/multiagent/ledger.go`)

```go
// DO NOT CHANGE - Core task execution relies on this
type Task struct {
    ID           string
    Name         string
    Description  string
    Type         string  // Matches to worker capabilities
    Priority     TaskPriority
    AssignedTo   string
    Dependencies []string
    Input        interface{}
    Output       interface{}
    Status       TaskStatus
    // ... other fields
}
```

### 5. LLM Provider Interface (Multi-Agent)

```go
// DO NOT CHANGE - Workers use this for LLM calls
type LLMProvider interface {
    GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
}
```

---

## Safe Integration Patterns

### Pattern 1: RAG Worker (Recommended)

Create a new TaskHandler that uses chain/RAG functionality:

```go
// chain/workers/rag_worker.go
package workers

import (
    "context"

    "github.com/Ranganaths/minion/chain"
    "github.com/Ranganaths/minion/core/multiagent"
    "github.com/Ranganaths/minion/retriever"
    "github.com/Ranganaths/minion/vectorstore"
)

// RAGWorker implements TaskHandler for RAG tasks
type RAGWorker struct {
    llmProvider  multiagent.LLMProvider
    vectorStore  vectorstore.VectorStore
    retriever    retriever.Retriever
    ragChain     *chain.RAGChain
}

// NewRAGWorker creates a RAG-capable worker
func NewRAGWorker(
    llm multiagent.LLMProvider,
    vs vectorstore.VectorStore,
    ret retriever.Retriever,
) *RAGWorker {
    return &RAGWorker{
        llmProvider: llm,
        vectorStore: vs,
        retriever:   ret,
    }
}

// GetName returns the worker name
func (w *RAGWorker) GetName() string {
    return "RAGWorker"
}

// GetCapabilities returns worker capabilities
func (w *RAGWorker) GetCapabilities() []string {
    return []string{
        "rag",
        "retrieval",
        "question_answering",
        "document_search",
        "knowledge_base",
    }
}

// HandleTask processes RAG tasks
func (w *RAGWorker) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
    // Extract query from task input
    input, ok := task.Input.(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("invalid task input format")
    }

    query, _ := input["query"].(string)
    if query == "" {
        return nil, fmt.Errorf("query is required")
    }

    // Use retriever to get relevant documents
    docs, err := w.retriever.GetRelevantDocuments(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("retrieval error: %w", err)
    }

    // Build context from documents
    var contextBuilder strings.Builder
    for _, doc := range docs {
        contextBuilder.WriteString(doc.PageContent)
        contextBuilder.WriteString("\n\n")
    }

    // Generate answer using LLM
    prompt := fmt.Sprintf(`Use the following context to answer the question.

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

    // Return result in expected format
    return map[string]interface{}{
        "answer":      resp.Text,
        "sources":     docs,
        "tokens_used": resp.TokensUsed,
    }, nil
}
```

### Pattern 2: Register RAG Worker with Coordinator

```go
// Example usage in application
func setupRAGMultiAgent(coordinator *multiagent.Coordinator) error {
    // Initialize RAG components
    embedder := embeddings.NewOpenAI(os.Getenv("OPENAI_API_KEY"))
    store := vectorstore.NewMemory(embedder)
    ret := retriever.NewVectorStoreRetriever(store)

    // Create RAG worker
    ragWorker := workers.NewRAGWorker(llmAdapter, store, ret)

    // Register with coordinator using existing API
    err := coordinator.CreateCustomWorker(ragWorker)
    if err != nil {
        return err
    }

    return nil
}
```

### Pattern 3: Chain-Enabled Specialist Worker

```go
// chain/workers/chain_worker.go
package workers

import (
    "context"

    "github.com/Ranganaths/minion/chain"
    "github.com/Ranganaths/minion/core/multiagent"
)

// ChainWorker wraps any Chain as a TaskHandler
type ChainWorker struct {
    name         string
    capabilities []string
    chain        chain.Chain
}

// NewChainWorker creates a worker from a chain
func NewChainWorker(name string, capabilities []string, c chain.Chain) *ChainWorker {
    return &ChainWorker{
        name:         name,
        capabilities: capabilities,
        chain:        c,
    }
}

func (w *ChainWorker) GetName() string {
    return w.name
}

func (w *ChainWorker) GetCapabilities() []string {
    return w.capabilities
}

func (w *ChainWorker) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
    // Convert task input to chain inputs
    inputs, ok := task.Input.(map[string]interface{})
    if !ok {
        inputs = map[string]interface{}{
            "input": task.Input,
        }
    }

    // Execute chain
    outputs, err := w.chain.Call(ctx, inputs)
    if err != nil {
        return nil, err
    }

    return outputs, nil
}
```

---

## Architecture: Proper Integration Layer

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           APPLICATION LAYER                              │
└────────────────────────────────────┬────────────────────────────────────┘
                                     │
         ┌───────────────────────────┼───────────────────────────┐
         │                           │                           │
         ▼                           ▼                           ▼
┌─────────────────┐       ┌─────────────────┐       ┌─────────────────┐
│  Multi-Agent    │       │   Direct Chain  │       │   RAG Pipeline  │
│  Coordinator    │       │   Execution     │       │   (Standalone)  │
│  (Existing)     │       │   (New)         │       │   (New)         │
└────────┬────────┘       └────────┬────────┘       └────────┬────────┘
         │                         │                         │
         │                         │                         │
         ▼                         │                         │
┌─────────────────────────┐        │                         │
│  Custom Workers         │        │                         │
│  ┌───────────────────┐  │        │                         │
│  │ RAGWorker         │◀─┼────────┴─────────────────────────┘
│  │ ChainWorker       │  │
│  │ RetrievalWorker   │  │
│  └───────────────────┘  │
│  ┌───────────────────┐  │
│  │ Built-in Workers  │  │  (Unchanged)
│  │ • CoderWorker     │  │
│  │ • AnalystWorker   │  │
│  │ • ResearcherWorker│  │
│  │ • WriterWorker    │  │
│  │ • ReviewerWorker  │  │
│  └───────────────────┘  │
└────────┬────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        SHARED INFRASTRUCTURE                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │
│  │   chain/    │  │ embeddings/ │  │ vectorstore/│  │   retriever/    │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────┘ │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │
│  │documentload/│  │textsplitter/│  │   prompt/   │  │  outputparser/  │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        EXISTING INFRASTRUCTURE                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │
│  │    llm/     │  │   tools/    │  │observability│  │   resilience/   │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## LLM Provider Adapter

The multi-agent system has its own LLM interface. Create an adapter:

```go
// adapters/llm_adapter.go
package adapters

import (
    "context"

    "github.com/Ranganaths/minion/core/multiagent"
    "github.com/Ranganaths/minion/llm"
)

// MultiAgentLLMAdapter adapts minion/llm to multiagent.LLMProvider
type MultiAgentLLMAdapter struct {
    provider llm.Provider
}

// NewMultiAgentLLMAdapter creates a new adapter
func NewMultiAgentLLMAdapter(provider llm.Provider) *MultiAgentLLMAdapter {
    return &MultiAgentLLMAdapter{provider: provider}
}

// GenerateCompletion implements multiagent.LLMProvider
func (a *MultiAgentLLMAdapter) GenerateCompletion(
    ctx context.Context,
    req *multiagent.CompletionRequest,
) (*multiagent.CompletionResponse, error) {
    // Convert request
    llmReq := &llm.CompletionRequest{
        SystemPrompt: req.SystemPrompt,
        UserPrompt:   req.Prompt,
        Temperature:  req.Temperature,
        MaxTokens:    req.MaxTokens,
        Model:        req.Model,
    }

    // Call underlying provider
    resp, err := a.provider.GenerateCompletion(ctx, llmReq)
    if err != nil {
        return nil, err
    }

    // Convert response
    return &multiagent.CompletionResponse{
        Text:       resp.Text,
        TokensUsed: resp.TokensUsed,
    }, nil
}

// ChainLLMAdapter adapts multiagent.LLMProvider to chain's needs
type ChainLLMAdapter struct {
    provider multiagent.LLMProvider
}

// NewChainLLMAdapter creates adapter for chains
func NewChainLLMAdapter(provider multiagent.LLMProvider) *ChainLLMAdapter {
    return &ChainLLMAdapter{provider: provider}
}

// Implements llm.Provider for use in chains
func (a *ChainLLMAdapter) GenerateCompletion(
    ctx context.Context,
    req *llm.CompletionRequest,
) (*llm.CompletionResponse, error) {
    // Convert and call
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
```

---

## Integration Checklist

### Before Implementation

- [ ] Review all multi-agent interfaces (listed above)
- [ ] Understand task flow through orchestrator
- [ ] Understand worker registration pattern
- [ ] Understand protocol message matching (InReplyTo)

### During Implementation

- [ ] New packages are ADDITIVE only
- [ ] No modifications to `core/multiagent/*.go`
- [ ] No changes to existing LLM provider interface
- [ ] New workers implement `TaskHandler` exactly
- [ ] Use adapters for interface compatibility

### Testing Requirements

- [ ] Run existing multi-agent tests after changes
- [ ] Test new RAG worker with existing orchestrator
- [ ] Test chain worker registration
- [ ] Test mixed workloads (RAG + existing workers)
- [ ] Test all three protocol backends with new workers
- [ ] Test load balancing with new worker capabilities

---

## Updated Package Structure

```
minion/
├── core/
│   └── multiagent/          # UNCHANGED - Do not modify
│       ├── coordinator.go
│       ├── orchestrator.go
│       ├── workers.go
│       ├── protocol.go
│       ├── protocol_impl.go
│       ├── protocol_redis.go
│       ├── protocol_kafka.go
│       └── ...
│
├── chain/                    # NEW - Chain orchestration
│   ├── interface.go
│   ├── base.go
│   ├── llm_chain.go
│   ├── rag_chain.go
│   ├── sequential_chain.go
│   ├── conversational_chain.go
│   └── workers/              # NEW - Multi-agent compatible workers
│       ├── rag_worker.go     # Implements TaskHandler
│       ├── chain_worker.go   # Implements TaskHandler
│       └── retrieval_worker.go
│
├── adapters/                 # NEW - Interface adapters
│   ├── llm_adapter.go        # LLM interface adapters
│   └── tool_adapter.go       # Tool interface adapters
│
├── embeddings/               # NEW - Embedding providers
├── vectorstore/              # NEW - Vector stores
├── retriever/                # NEW - Retrievers
├── documentloader/           # NEW - Document loaders
├── textsplitter/             # NEW - Text splitters
├── prompt/                   # NEW - Prompt templates
├── outputparser/             # NEW - Output parsers
└── rag/                      # NEW - RAG pipeline
```

---

## Example: Complete Multi-Agent RAG Setup

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/Ranganaths/minion/adapters"
    "github.com/Ranganaths/minion/chain/workers"
    "github.com/Ranganaths/minion/core/multiagent"
    "github.com/Ranganaths/minion/embeddings"
    "github.com/Ranganaths/minion/llm"
    "github.com/Ranganaths/minion/retriever"
    "github.com/Ranganaths/minion/vectorstore"
)

func main() {
    ctx := context.Background()

    // 1. Create LLM provider (existing)
    llmProvider := llm.NewOpenAI(os.Getenv("OPENAI_API_KEY"), "gpt-4")

    // 2. Create adapter for multi-agent
    maLLMAdapter := adapters.NewMultiAgentLLMAdapter(llmProvider)

    // 3. Create protocol (existing)
    protocol := multiagent.NewInMemoryProtocol()

    // 4. Create coordinator with existing workers
    coordinator, err := multiagent.NewCoordinator(
        maLLMAdapter,
        protocol,
        multiagent.WithDefaultWorkers(),  // Built-in workers still work
    )
    if err != nil {
        log.Fatal(err)
    }

    // 5. Setup RAG infrastructure (new)
    embedder := embeddings.NewOpenAI(os.Getenv("OPENAI_API_KEY"))
    store := vectorstore.NewMemory(embedder)
    ret := retriever.NewVectorStoreRetriever(store, retriever.WithTopK(5))

    // 6. Create and register RAG worker (new, but uses existing pattern)
    ragWorker := workers.NewRAGWorker(maLLMAdapter, store, ret)
    err = coordinator.CreateCustomWorker(ragWorker)
    if err != nil {
        log.Fatal(err)
    }

    // 7. Execute tasks - orchestrator automatically routes to appropriate workers

    // This goes to RAGWorker (has "rag" capability)
    ragResult, err := coordinator.ExecuteTask(ctx, multiagent.TaskRequest{
        Name:        "Answer question from knowledge base",
        Description: "Use RAG to answer the question",
        Type:        "rag",  // Matches RAGWorker capability
        Input: map[string]interface{}{
            "query": "What are the main features of the product?",
        },
    })

    // This goes to CoderWorker (built-in, has "code_generation" capability)
    codeResult, err := coordinator.ExecuteTask(ctx, multiagent.TaskRequest{
        Name:        "Generate code",
        Description: "Write a function",
        Type:        "code_generation",  // Matches CoderWorker
        Input: map[string]interface{}{
            "task": "Write a hello world function in Go",
        },
    })

    // Complex task - orchestrator decomposes and uses multiple workers
    complexResult, err := coordinator.ExecuteTask(ctx, multiagent.TaskRequest{
        Name:        "Research and document a topic",
        Description: "Research the topic using knowledge base, then write documentation",
        Type:        "complex",
        Input: map[string]interface{}{
            "topic": "How to use the new RAG features",
        },
    })
    // Orchestrator LLM will:
    // 1. Create subtask for RAGWorker to retrieve information
    // 2. Create subtask for WriterWorker to write documentation
    // 3. Create subtask for ReviewerWorker to review
}
```

---

## Testing Strategy

### Unit Tests for New Workers

```go
// chain/workers/rag_worker_test.go
func TestRAGWorkerImplementsTaskHandler(t *testing.T) {
    var _ multiagent.TaskHandler = (*RAGWorker)(nil)
}

func TestRAGWorkerCapabilities(t *testing.T) {
    worker := NewRAGWorker(mockLLM, mockStore, mockRetriever)
    caps := worker.GetCapabilities()

    assert.Contains(t, caps, "rag")
    assert.Contains(t, caps, "retrieval")
}

func TestRAGWorkerHandleTask(t *testing.T) {
    // Setup mocks
    mockLLM := &MockLLMProvider{}
    mockStore := &MockVectorStore{}
    mockRetriever := &MockRetriever{}

    worker := NewRAGWorker(mockLLM, mockStore, mockRetriever)

    task := &multiagent.Task{
        ID:   "test-task",
        Type: "rag",
        Input: map[string]interface{}{
            "query": "test question",
        },
    }

    result, err := worker.HandleTask(context.Background(), task)

    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

### Integration Tests

```go
// integration/multiagent_rag_test.go
func TestMultiAgentWithRAGWorker(t *testing.T) {
    // Create full coordinator with RAG worker
    coordinator := setupTestCoordinator(t)

    // Register RAG worker
    ragWorker := setupRAGWorker(t)
    err := coordinator.CreateCustomWorker(ragWorker)
    require.NoError(t, err)

    // Execute RAG task
    result, err := coordinator.ExecuteTask(context.Background(), multiagent.TaskRequest{
        Name: "RAG Query",
        Type: "rag",
        Input: map[string]interface{}{"query": "test"},
    })

    assert.NoError(t, err)
    assert.Equal(t, multiagent.TaskStatusCompleted, result.Status)
}

func TestMixedWorkload(t *testing.T) {
    coordinator := setupTestCoordinator(t)

    // Add RAG worker alongside built-in workers
    coordinator.CreateCustomWorker(setupRAGWorker(t))

    // Execute mixed tasks
    tasks := []multiagent.TaskRequest{
        {Type: "rag", Input: map[string]interface{}{"query": "Q1"}},
        {Type: "code_generation", Input: map[string]interface{}{"task": "hello"}},
        {Type: "rag", Input: map[string]interface{}{"query": "Q2"}},
        {Type: "research", Input: map[string]interface{}{"topic": "AI"}},
    }

    for _, task := range tasks {
        result, err := coordinator.ExecuteTask(context.Background(), task)
        assert.NoError(t, err)
        assert.Equal(t, multiagent.TaskStatusCompleted, result.Status)
    }
}
```

---

## Forbidden Changes

### DO NOT modify these files:

```
core/multiagent/protocol.go         # Protocol interface
core/multiagent/protocol_impl.go    # In-memory protocol
core/multiagent/protocol_redis.go   # Redis protocol
core/multiagent/protocol_kafka.go   # Kafka protocol
core/multiagent/coordinator.go      # Coordinator API
core/multiagent/orchestrator.go     # Task orchestration
core/multiagent/workers.go          # Built-in workers
core/multiagent/ledger.go           # Task/Progress ledgers
core/multiagent/loadbalancer.go     # Load balancing
core/multiagent/autoscaler.go       # Auto-scaling
core/multiagent/deduplication.go    # Message dedup
```

### DO NOT change these interfaces:

- `Protocol` interface
- `TaskHandler` interface
- `Message` struct (especially `InReplyTo` field)
- `Task` struct (especially `Dependencies` field)
- `LLMProvider` interface (multi-agent version)

---

## Summary

The Chain/RAG integration follows these principles:

1. **Additive Only**: New packages don't modify existing code
2. **Interface Compliance**: New workers implement `TaskHandler` exactly
3. **Adapter Pattern**: Use adapters for interface compatibility
4. **Capability-Based Routing**: Register capabilities for load balancer
5. **Protocol Agnostic**: Works with all three protocol backends
6. **Observable**: Integrates with existing metrics and logging

By following this guide, the new Chain/RAG features will enhance Minion without breaking the production-ready multi-agent system.

---

*End of Integration Guide*
