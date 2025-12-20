package chain

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracingCallback is a chain callback that creates OpenTelemetry spans for chain execution.
// It provides distributed tracing for chain operations including LLM calls and document retrieval.
// TracingCallback is safe for concurrent use by multiple goroutines.
type TracingCallback struct {
	tracer trace.Tracer
	// Track active spans by chain name
	mu    sync.RWMutex
	spans map[string]trace.Span
}

// NewTracingCallback creates a new tracing callback using the global OpenTelemetry tracer.
func NewTracingCallback() *TracingCallback {
	return &TracingCallback{
		tracer: otel.Tracer("minion-chain"),
		spans:  make(map[string]trace.Span),
	}
}

// NewTracingCallbackWithTracer creates a tracing callback with a specific tracer.
func NewTracingCallbackWithTracer(tracer trace.Tracer) *TracingCallback {
	return &TracingCallback{
		tracer: tracer,
		spans:  make(map[string]trace.Span),
	}
}

// OnChainStart is called when a chain begins execution.
func (tc *TracingCallback) OnChainStart(ctx context.Context, chainName string, inputs map[string]any) {
	attrs := []attribute.KeyValue{
		attribute.String("chain.name", chainName),
		attribute.Int("chain.inputs.count", len(inputs)),
	}

	// Add input keys as attributes (not values to avoid PII)
	for key := range inputs {
		attrs = append(attrs, attribute.String("chain.input."+key, "present"))
	}

	_, span := tc.tracer.Start(ctx, "chain."+chainName, trace.WithAttributes(attrs...))
	tc.mu.Lock()
	tc.spans[chainName] = span
	tc.mu.Unlock()
}

// OnChainEnd is called when a chain completes successfully.
func (tc *TracingCallback) OnChainEnd(ctx context.Context, chainName string, outputs map[string]any) {
	tc.mu.Lock()
	span, ok := tc.spans[chainName]
	if ok {
		delete(tc.spans, chainName)
	}
	tc.mu.Unlock()

	if ok {
		span.SetAttributes(attribute.Int("chain.outputs.count", len(outputs)))
		span.SetStatus(codes.Ok, "")
		span.End()
	}
}

// OnChainError is called when a chain encounters an error.
func (tc *TracingCallback) OnChainError(ctx context.Context, chainName string, err error) {
	tc.mu.Lock()
	span, ok := tc.spans[chainName]
	if ok {
		delete(tc.spans, chainName)
	}
	tc.mu.Unlock()

	if ok {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		span.End()
	}
}

// OnLLMStart is called before an LLM call.
func (tc *TracingCallback) OnLLMStart(ctx context.Context, prompt string) {
	attrs := []attribute.KeyValue{
		attribute.Int("llm.prompt.length", len(prompt)),
	}
	_, span := tc.tracer.Start(ctx, "llm.call", trace.WithAttributes(attrs...))
	tc.mu.Lock()
	tc.spans["_llm"] = span
	tc.mu.Unlock()
}

// OnLLMEnd is called after an LLM call completes.
func (tc *TracingCallback) OnLLMEnd(ctx context.Context, response string, tokens int) {
	tc.mu.Lock()
	span, ok := tc.spans["_llm"]
	if ok {
		delete(tc.spans, "_llm")
	}
	tc.mu.Unlock()

	if ok {
		span.SetAttributes(
			attribute.Int("llm.response.length", len(response)),
			attribute.Int("llm.tokens.total", tokens),
		)
		span.SetStatus(codes.Ok, "")
		span.End()
	}
}

// OnRetrieverStart is called before document retrieval.
func (tc *TracingCallback) OnRetrieverStart(ctx context.Context, query string) {
	attrs := []attribute.KeyValue{
		attribute.String("retriever.query", truncateString(query, 100)),
		attribute.Int("retriever.query.length", len(query)),
	}
	_, span := tc.tracer.Start(ctx, "retriever.search", trace.WithAttributes(attrs...))
	tc.mu.Lock()
	tc.spans["_retriever"] = span
	tc.mu.Unlock()
}

// OnRetrieverEnd is called after documents are retrieved.
func (tc *TracingCallback) OnRetrieverEnd(ctx context.Context, docs []Document) {
	tc.mu.Lock()
	span, ok := tc.spans["_retriever"]
	if ok {
		delete(tc.spans, "_retriever")
	}
	tc.mu.Unlock()

	if ok {
		span.SetAttributes(attribute.Int("retriever.documents.count", len(docs)))
		span.SetStatus(codes.Ok, "")
		span.End()
	}
}

// truncateString truncates a string to the specified max length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Ensure TracingCallback implements ChainCallback
var _ ChainCallback = (*TracingCallback)(nil)
