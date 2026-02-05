// Package tracing provides comprehensive execution tracing for the Minion agent framework.
//
// The tracing package captures execution details at runtime, including:
//   - LLM calls (prompts, responses, tokens, costs)
//   - Tool invocations (inputs, outputs, errors)
//   - Agent reasoning and decisions
//   - Timing data for all operations
//
// # Architecture
//
// The package consists of several key components:
//
//   - Trace: A complete execution trace containing all spans
//   - Span: A single operation within a trace (LLM call, tool call, etc.)
//   - TraceCollector: Middleware that collects trace data during execution
//   - ExecutionContext: Carries trace context through the execution
//   - TraceStore: Interface for persisting traces (in-memory, PostgreSQL)
//   - TracingAPIServer: HTTP API for serving traces to frontends
//
// # Basic Usage
//
// Create a trace collector and wrap your agent executor:
//
//	store := tracing.NewInMemoryTraceStore()
//	executor, _ := tracing.NewTracedAgentExecutor(tracing.TracedExecutorConfig{
//	    Executor:  myExecutor,
//	    Store:     store,
//	    AgentID:   "agent-123",
//	    AgentName: "MyAgent",
//	})
//
//	// Execute with automatic tracing
//	output, err := executor.Run(ctx, "What is 2+2?")
//
// # API Server
//
// Start an API server to serve traces:
//
//	apiServer := tracing.NewTracingAPIServer(store, tracing.DefaultAPIConfig())
//	go apiServer.Start()
//
// # Real-time Hooks
//
// Add hooks for real-time trace updates:
//
//	hook := &tracing.ConsoleTraceHook{Verbose: true}
//	executor.GetCollector().AddHook(hook)
//
// # PostgreSQL Storage
//
// Use PostgreSQL for persistent storage:
//
//	store, _ := tracing.NewPostgresTraceStore("postgres://user:pass@host/db")
package tracing

import (
	"context"
	"database/sql"
)

// Version is the current package version
const Version = "1.0.0"

// DefaultMaxPromptLength is the default maximum length for prompts
const DefaultMaxPromptLength = 10000

// DefaultMaxResponseLength is the default maximum length for responses
const DefaultMaxResponseLength = 10000

// DefaultMaxToolOutputLength is the default maximum length for tool outputs
const DefaultMaxToolOutputLength = 5000

// NewStore creates a new trace store based on the provided configuration.
// If dsn is empty, returns an in-memory store.
// If dsn starts with "postgres://", returns a PostgreSQL store.
func NewStore(dsn string) (TraceStore, error) {
	if dsn == "" {
		return NewInMemoryTraceStore(), nil
	}

	return NewPostgresTraceStore(dsn)
}

// NewStoreFromDB creates a new PostgreSQL trace store from an existing database connection
func NewStoreFromDB(db *sql.DB) (TraceStore, error) {
	return NewPostgresTraceStoreFromDB(db)
}

// StartAPIServer is a convenience function to start the tracing API server
func StartAPIServer(store TraceStore, addr string) (*TracingAPIServer, error) {
	config := DefaultAPIConfig()
	config.Addr = addr

	server := NewTracingAPIServer(store, config)

	go func() {
		if err := server.Start(); err != nil {
			// Log error - in production you'd want proper error handling
		}
	}()

	return server, nil
}

// QuickTrace is a convenience function for one-off tracing
func QuickTrace(ctx context.Context, agentID, agentName, input string, fn func(ctx *ExecutionContext) (string, error)) (*Trace, string, error) {
	store := NewInMemoryTraceStore()
	collector := NewTraceCollector(store, DefaultCollectorConfig())

	ctx, _ = collector.StartTrace(ctx, agentID, agentName, input)
	execCtx := NewExecutionContext(ctx, collector)

	output, err := fn(execCtx)

	collector.EndTrace(ctx, output, err)

	return collector.GetTrace(), output, err
}

// WrapWithTracing wraps an existing execution function with tracing
func WrapWithTracing(store TraceStore, agentID, agentName string, fn func(ctx context.Context) (string, error)) func(ctx context.Context, input string) (string, *Trace, error) {
	return func(ctx context.Context, input string) (string, *Trace, error) {
		collector := NewTraceCollector(store, DefaultCollectorConfig())

		ctx, _ = collector.StartTrace(ctx, agentID, agentName, input)

		output, err := fn(ctx)

		collector.EndTrace(ctx, output, err)

		return output, collector.GetTrace(), err
	}
}

// MustNewStore creates a new store and panics on error
func MustNewStore(dsn string) TraceStore {
	store, err := NewStore(dsn)
	if err != nil {
		panic(err)
	}
	return store
}

// TracingEnabled checks if tracing context is present
func TracingEnabled(ctx context.Context) bool {
	return TraceFromContext(ctx) != nil
}

// CurrentTraceID returns the current trace ID from context, or empty string if not tracing
func CurrentTraceID(ctx context.Context) string {
	if trace := TraceFromContext(ctx); trace != nil {
		return string(trace.ID)
	}
	return ""
}

// CurrentSpanID returns the current span ID from context, or empty string if not tracing
func CurrentSpanID(ctx context.Context) string {
	if span := SpanFromContext(ctx); span != nil {
		return string(span.ID)
	}
	return ""
}
