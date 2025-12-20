// Package chain provides a composable chain framework for LLM-based applications.
// Chains are sequences of operations that can be combined to create complex workflows.
package chain

import (
	"context"
	"sync"
	"time"
)

// Chain is the core interface for all chains.
// A chain takes inputs, processes them through one or more steps, and produces outputs.
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

// Runnable is a more flexible interface for chain components with generics
type Runnable[I, O any] interface {
	// Invoke executes with typed input and output
	Invoke(ctx context.Context, input I) (O, error)

	// Stream executes and streams typed outputs
	Stream(ctx context.Context, input I) (<-chan O, error)

	// Batch executes for multiple inputs
	Batch(ctx context.Context, inputs []I) ([]O, error)
}

// StreamEvent represents a streaming event from chain execution
type StreamEvent struct {
	// Type indicates the kind of event
	Type StreamEventType

	// Content contains streamed text content (for token events)
	Content string

	// Data contains structured data (for chunk/complete events)
	Data map[string]any

	// Error contains error information (for error events)
	Error error

	// Timestamp when the event occurred
	Timestamp time.Time
}

// StreamEventType indicates the type of streaming event
type StreamEventType string

const (
	// StreamEventToken indicates a single token was generated
	StreamEventToken StreamEventType = "token"

	// StreamEventChunk indicates a chunk of data is available
	StreamEventChunk StreamEventType = "chunk"

	// StreamEventComplete indicates the stream is complete
	StreamEventComplete StreamEventType = "complete"

	// StreamEventError indicates an error occurred
	StreamEventError StreamEventType = "error"

	// StreamEventStart indicates the chain started
	StreamEventStart StreamEventType = "start"

	// StreamEventRetrieval indicates documents were retrieved
	StreamEventRetrieval StreamEventType = "retrieval"
)

// Document represents a document in the chain
type Document struct {
	// ID is the unique identifier for the document
	ID string

	// PageContent is the main text content
	PageContent string

	// Metadata contains additional information about the document
	Metadata map[string]any
}

// ChainCallback provides hooks into chain execution for observability
type ChainCallback interface {
	// OnChainStart is called when a chain begins execution
	OnChainStart(ctx context.Context, chainName string, inputs map[string]any)

	// OnChainEnd is called when a chain completes successfully
	OnChainEnd(ctx context.Context, chainName string, outputs map[string]any)

	// OnChainError is called when a chain encounters an error
	OnChainError(ctx context.Context, chainName string, err error)

	// OnLLMStart is called before an LLM call
	OnLLMStart(ctx context.Context, prompt string)

	// OnLLMEnd is called after an LLM call completes
	OnLLMEnd(ctx context.Context, response string, tokens int)

	// OnRetrieverStart is called before document retrieval
	OnRetrieverStart(ctx context.Context, query string)

	// OnRetrieverEnd is called after documents are retrieved
	OnRetrieverEnd(ctx context.Context, docs []Document)
}

// CallbackManager manages multiple callbacks.
// CallbackManager is safe for concurrent use by multiple goroutines.
type CallbackManager struct {
	mu        sync.RWMutex
	callbacks []ChainCallback
}

// NewCallbackManager creates a new callback manager
func NewCallbackManager(callbacks ...ChainCallback) *CallbackManager {
	return &CallbackManager{callbacks: callbacks}
}

// Add adds a callback to the manager.
// This method is safe for concurrent use.
func (m *CallbackManager) Add(cb ChainCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, cb)
}

// getCallbacks returns a snapshot of current callbacks for safe iteration
func (m *CallbackManager) getCallbacks() []ChainCallback {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Return a copy to allow safe iteration
	result := make([]ChainCallback, len(m.callbacks))
	copy(result, m.callbacks)
	return result
}

// OnChainStart notifies all callbacks of chain start
func (m *CallbackManager) OnChainStart(ctx context.Context, chainName string, inputs map[string]any) {
	for _, cb := range m.getCallbacks() {
		cb.OnChainStart(ctx, chainName, inputs)
	}
}

// OnChainEnd notifies all callbacks of chain end
func (m *CallbackManager) OnChainEnd(ctx context.Context, chainName string, outputs map[string]any) {
	for _, cb := range m.getCallbacks() {
		cb.OnChainEnd(ctx, chainName, outputs)
	}
}

// OnChainError notifies all callbacks of chain error
func (m *CallbackManager) OnChainError(ctx context.Context, chainName string, err error) {
	for _, cb := range m.getCallbacks() {
		cb.OnChainError(ctx, chainName, err)
	}
}

// OnLLMStart notifies all callbacks of LLM start
func (m *CallbackManager) OnLLMStart(ctx context.Context, prompt string) {
	for _, cb := range m.getCallbacks() {
		cb.OnLLMStart(ctx, prompt)
	}
}

// OnLLMEnd notifies all callbacks of LLM end
func (m *CallbackManager) OnLLMEnd(ctx context.Context, response string, tokens int) {
	for _, cb := range m.getCallbacks() {
		cb.OnLLMEnd(ctx, response, tokens)
	}
}

// OnRetrieverStart notifies all callbacks of retriever start
func (m *CallbackManager) OnRetrieverStart(ctx context.Context, query string) {
	for _, cb := range m.getCallbacks() {
		cb.OnRetrieverStart(ctx, query)
	}
}

// OnRetrieverEnd notifies all callbacks of retriever end
func (m *CallbackManager) OnRetrieverEnd(ctx context.Context, docs []Document) {
	for _, cb := range m.getCallbacks() {
		cb.OnRetrieverEnd(ctx, docs)
	}
}

// NoopCallback is a callback that does nothing (useful for embedding)
type NoopCallback struct{}

func (NoopCallback) OnChainStart(ctx context.Context, chainName string, inputs map[string]any)  {}
func (NoopCallback) OnChainEnd(ctx context.Context, chainName string, outputs map[string]any)   {}
func (NoopCallback) OnChainError(ctx context.Context, chainName string, err error)             {}
func (NoopCallback) OnLLMStart(ctx context.Context, prompt string)                             {}
func (NoopCallback) OnLLMEnd(ctx context.Context, response string, tokens int)                 {}
func (NoopCallback) OnRetrieverStart(ctx context.Context, query string)                        {}
func (NoopCallback) OnRetrieverEnd(ctx context.Context, docs []Document)                       {}
