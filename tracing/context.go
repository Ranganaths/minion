package tracing

import (
	"context"
)

// Context keys for trace propagation
type contextKey string

const (
	traceContextKey contextKey = "minion_trace"
	spanContextKey  contextKey = "minion_span"
	collectorKey    contextKey = "minion_collector"
)

// ContextWithTrace returns a context with the trace attached
func ContextWithTrace(ctx context.Context, trace *Trace) context.Context {
	return context.WithValue(ctx, traceContextKey, trace)
}

// TraceFromContext retrieves the trace from context
func TraceFromContext(ctx context.Context) *Trace {
	if trace, ok := ctx.Value(traceContextKey).(*Trace); ok {
		return trace
	}
	return nil
}

// ContextWithSpan returns a context with the current span attached
func ContextWithSpan(ctx context.Context, span *Span) context.Context {
	return context.WithValue(ctx, spanContextKey, span)
}

// SpanFromContext retrieves the current span from context
func SpanFromContext(ctx context.Context) *Span {
	if span, ok := ctx.Value(spanContextKey).(*Span); ok {
		return span
	}
	return nil
}

// ContextWithCollector returns a context with the trace collector attached
func ContextWithCollector(ctx context.Context, collector *TraceCollector) context.Context {
	return context.WithValue(ctx, collectorKey, collector)
}

// CollectorFromContext retrieves the trace collector from context
func CollectorFromContext(ctx context.Context) *TraceCollector {
	if collector, ok := ctx.Value(collectorKey).(*TraceCollector); ok {
		return collector
	}
	return nil
}

// GetTraceID retrieves the trace ID from context
func GetTraceID(ctx context.Context) TraceID {
	if trace := TraceFromContext(ctx); trace != nil {
		return trace.ID
	}
	return ""
}

// GetSpanID retrieves the current span ID from context
func GetSpanID(ctx context.Context) SpanID {
	if span := SpanFromContext(ctx); span != nil {
		return span.ID
	}
	return ""
}

// ExecutionContext provides a rich context for traced execution.
// It wraps the standard context and provides convenience methods
// for trace operations.
type ExecutionContext struct {
	context.Context
	collector *TraceCollector
}

// NewExecutionContext creates a new execution context with tracing
func NewExecutionContext(ctx context.Context, collector *TraceCollector) *ExecutionContext {
	ctx = ContextWithCollector(ctx, collector)
	return &ExecutionContext{
		Context:   ctx,
		collector: collector,
	}
}

// StartLLMCall begins tracking an LLM call and returns an updated context
func (e *ExecutionContext) StartLLMCall(provider, model, systemPrompt, userPrompt string, temperature float64, maxTokens int) (*ExecutionContext, SpanID) {
	ctx, spanID := e.collector.StartLLMSpan(e.Context, provider, model, systemPrompt, userPrompt, temperature, maxTokens)
	return &ExecutionContext{Context: ctx, collector: e.collector}, spanID
}

// EndLLMCall completes tracking an LLM call
func (e *ExecutionContext) EndLLMCall(spanID SpanID, response string, promptTokens, completionTokens int, cost float64, finishReason string, err error) {
	e.collector.EndLLMSpan(e.Context, spanID, response, promptTokens, completionTokens, cost, finishReason, err)
}

// StartToolCall begins tracking a tool invocation and returns an updated context
func (e *ExecutionContext) StartToolCall(toolName, input string) (*ExecutionContext, SpanID) {
	ctx, spanID := e.collector.StartToolSpan(e.Context, toolName, input)
	return &ExecutionContext{Context: ctx, collector: e.collector}, spanID
}

// EndToolCall completes tracking a tool invocation
func (e *ExecutionContext) EndToolCall(spanID SpanID, output string, err error) {
	e.collector.EndToolSpan(e.Context, spanID, output, err)
}

// StartIteration begins tracking an iteration
func (e *ExecutionContext) StartIteration() (*ExecutionContext, SpanID) {
	ctx, spanID := e.collector.StartIteration(e.Context)
	return &ExecutionContext{Context: ctx, collector: e.collector}, spanID
}

// EndIteration completes tracking an iteration
func (e *ExecutionContext) EndIteration(spanID SpanID, err error) {
	e.collector.EndIteration(e.Context, spanID, err)
}

// RecordThought records an agent's reasoning
func (e *ExecutionContext) RecordThought(thought string, iteration int) SpanID {
	return e.collector.RecordThought(e.Context, thought, iteration)
}

// RecordDecision records an agent's decision
func (e *ExecutionContext) RecordDecision(action, actionInput, thought string, iteration int, isFinal bool) SpanID {
	return e.collector.RecordDecision(e.Context, action, actionInput, thought, iteration, isFinal)
}

// AddEvent adds a timestamped event to the current span
func (e *ExecutionContext) AddEvent(name string, attrs map[string]interface{}) {
	e.collector.AddEvent(e.Context, name, attrs)
}

// SetAttribute sets an attribute on the current span
func (e *ExecutionContext) SetAttribute(key string, value interface{}) {
	e.collector.SetAttribute(e.Context, key, value)
}

// SetTraceMetadata sets metadata on the trace
func (e *ExecutionContext) SetTraceMetadata(key string, value interface{}) {
	e.collector.SetTraceMetadata(key, value)
}

// GetTrace returns the current trace
func (e *ExecutionContext) GetTrace() *Trace {
	return e.collector.GetTrace()
}

// GetTraceID returns the current trace ID
func (e *ExecutionContext) GetTraceID() TraceID {
	return e.collector.GetTraceID()
}

// GetCollector returns the trace collector
func (e *ExecutionContext) GetCollector() *TraceCollector {
	return e.collector
}

// Unwrap returns the underlying context
func (e *ExecutionContext) Unwrap() context.Context {
	return e.Context
}

// TracedFunc is a function that runs within a traced context
type TracedFunc func(ctx *ExecutionContext) error

// TracedLLMFunc is a function that performs an LLM call
type TracedLLMFunc func(ctx *ExecutionContext) (response string, promptTokens, completionTokens int, cost float64, finishReason string, err error)

// TracedToolFunc is a function that performs a tool call
type TracedToolFunc func(ctx *ExecutionContext) (output string, err error)

// WithLLMTrace wraps an LLM call with automatic tracing
func WithLLMTrace(ctx *ExecutionContext, provider, model, systemPrompt, userPrompt string, temperature float64, maxTokens int, fn TracedLLMFunc) (string, error) {
	tracedCtx, spanID := ctx.StartLLMCall(provider, model, systemPrompt, userPrompt, temperature, maxTokens)
	response, promptTokens, completionTokens, cost, finishReason, err := fn(tracedCtx)
	ctx.EndLLMCall(spanID, response, promptTokens, completionTokens, cost, finishReason, err)
	return response, err
}

// WithToolTrace wraps a tool call with automatic tracing
func WithToolTrace(ctx *ExecutionContext, toolName, input string, fn TracedToolFunc) (string, error) {
	tracedCtx, spanID := ctx.StartToolCall(toolName, input)
	output, err := fn(tracedCtx)
	ctx.EndToolCall(spanID, output, err)
	return output, err
}

// WithIterationTrace wraps an iteration with automatic tracing
func WithIterationTrace(ctx *ExecutionContext, fn TracedFunc) error {
	tracedCtx, spanID := ctx.StartIteration()
	err := fn(tracedCtx)
	ctx.EndIteration(spanID, err)
	return err
}
