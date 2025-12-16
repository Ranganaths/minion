package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingConfig contains tracing configuration
type TracingConfig struct {
	Enabled       bool
	ServiceName   string
	Environment   string
	Exporter      string  // jaeger, otlp, stdout
	JaegerURL     string  // e.g., http://localhost:14268/api/traces
	OTLPEndpoint  string  // e.g., localhost:4317
	SamplingRatio float64 // 0.0 to 1.0
}

// Tracer wraps OpenTelemetry tracer
type Tracer struct {
	tracer   trace.Tracer
	provider *sdktrace.TracerProvider
	config   TracingConfig
}

// SpanKind represents the type of span
type SpanKind string

const (
	SpanKindAgent      SpanKind = "agent"
	SpanKindTool       SpanKind = "tool"
	SpanKindLLM        SpanKind = "llm"
	SpanKindStorage    SpanKind = "storage"
	SpanKindSession    SpanKind = "session"
	SpanKindMemory     SpanKind = "memory"
	SpanKindMultiAgent SpanKind = "multiagent"
	SpanKindWorker     SpanKind = "worker"
	SpanKindProtocol   SpanKind = "protocol"
)

// Common attribute keys
const (
	AttrAgentID        = "agent.id"
	AttrAgentName      = "agent.name"
	AttrSessionID      = "session.id"
	AttrUserID         = "user.id"
	AttrToolName       = "tool.name"
	AttrLLMProvider    = "llm.provider"
	AttrLLMModel       = "llm.model"
	AttrLLMPromptTokens = "llm.prompt_tokens"
	AttrLLMCompletionTokens = "llm.completion_tokens"
	AttrLLMTotalTokens = "llm.total_tokens"
	AttrLLMCost        = "llm.cost"
	AttrStorageOperation = "storage.operation"
	AttrStorageTable   = "storage.table"
	AttrMemoryType     = "memory.type"
	AttrMemoryCount    = "memory.count"
	AttrErrorType      = "error.type"
	AttrErrorMessage   = "error.message"

	// Multi-agent specific attributes
	AttrTaskID         = "multiagent.task.id"
	AttrTaskName       = "multiagent.task.name"
	AttrTaskType       = "multiagent.task.type"
	AttrTaskPriority   = "multiagent.task.priority"
	AttrWorkerID       = "multiagent.worker.id"
	AttrWorkerCapability = "multiagent.worker.capability"
	AttrMessageType    = "multiagent.message.type"
	AttrMessageID      = "multiagent.message.id"
	AttrOrchestratorID = "multiagent.orchestrator.id"
	AttrSubtaskCount   = "multiagent.subtask.count"
)

// NewTracer creates a new tracer instance
func NewTracer(config TracingConfig) (*Tracer, error) {
	if !config.Enabled {
		// Return a no-op tracer
		return &Tracer{
			tracer:   otel.Tracer("minion-noop"),
			provider: nil,
			config:   config,
		}, nil
	}

	// Create exporter based on configuration
	var exporter sdktrace.SpanExporter
	var err error

	switch config.Exporter {
	case "jaeger":
		exporter, err = jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(config.JaegerURL)))
		if err != nil {
			return nil, fmt.Errorf("failed to create Jaeger exporter: %w", err)
		}
	case "otlp":
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		exporter, err = otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(config.OTLPEndpoint),
			otlptracegrpc.WithInsecure(), // Use WithTLSCredentials() in production
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
	case "stdout":
		// For development: log to stdout
		exporter, err = stdoutExporter()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown exporter type: %s", config.Exporter)
	}

	// Create resource with service information
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String("1.0.0"),
			attribute.String("environment", config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create trace provider with sampling
	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(config.SamplingRatio))

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set as global provider
	otel.SetTracerProvider(provider)

	// Get tracer
	tracer := provider.Tracer("minion-agent")

	return &Tracer{
		tracer:   tracer,
		provider: provider,
		config:   config,
	}, nil
}

// Close shuts down the tracer provider
func (t *Tracer) Close(ctx context.Context) error {
	if t.provider != nil {
		return t.provider.Shutdown(ctx)
	}
	return nil
}

// StartSpan starts a new span
func (t *Tracer) StartSpan(ctx context.Context, name string, kind SpanKind, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	// Add span kind as attribute
	attrs = append(attrs, attribute.String("span.kind", string(kind)))

	ctx, span := t.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
	return ctx, span
}

// StartAgentSpan starts a span for agent execution
func (t *Tracer) StartAgentSpan(ctx context.Context, agentID, agentName, action string) (context.Context, trace.Span) {
	return t.StartSpan(ctx, fmt.Sprintf("agent.%s", action), SpanKindAgent,
		attribute.String(AttrAgentID, agentID),
		attribute.String(AttrAgentName, agentName),
		attribute.String("action", action),
	)
}

// StartToolSpan starts a span for tool execution
func (t *Tracer) StartToolSpan(ctx context.Context, toolName string, input map[string]interface{}) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrToolName, toolName),
	}

	// Add input parameters as attributes (be careful with PII)
	for k, v := range input {
		// Only add primitive types and limit string length
		switch val := v.(type) {
		case string:
			if len(val) < 100 {
				attrs = append(attrs, attribute.String(fmt.Sprintf("input.%s", k), val))
			}
		case int:
			attrs = append(attrs, attribute.Int(fmt.Sprintf("input.%s", k), val))
		case bool:
			attrs = append(attrs, attribute.Bool(fmt.Sprintf("input.%s", k), val))
		}
	}

	return t.StartSpan(ctx, fmt.Sprintf("tool.%s", toolName), SpanKindTool, attrs...)
}

// StartLLMSpan starts a span for LLM API call
func (t *Tracer) StartLLMSpan(ctx context.Context, provider, model string) (context.Context, trace.Span) {
	return t.StartSpan(ctx, fmt.Sprintf("llm.%s.%s", provider, model), SpanKindLLM,
		attribute.String(AttrLLMProvider, provider),
		attribute.String(AttrLLMModel, model),
	)
}

// RecordLLMTokens records token usage on an LLM span
func (t *Tracer) RecordLLMTokens(span trace.Span, promptTokens, completionTokens int, cost float64) {
	span.SetAttributes(
		attribute.Int(AttrLLMPromptTokens, promptTokens),
		attribute.Int(AttrLLMCompletionTokens, completionTokens),
		attribute.Int(AttrLLMTotalTokens, promptTokens+completionTokens),
		attribute.Float64(AttrLLMCost, cost),
	)
}

// StartStorageSpan starts a span for storage operation
func (t *Tracer) StartStorageSpan(ctx context.Context, operation, table string) (context.Context, trace.Span) {
	return t.StartSpan(ctx, fmt.Sprintf("storage.%s.%s", operation, table), SpanKindStorage,
		attribute.String(AttrStorageOperation, operation),
		attribute.String(AttrStorageTable, table),
	)
}

// StartSessionSpan starts a span for session operation
func (t *Tracer) StartSessionSpan(ctx context.Context, sessionID, operation string) (context.Context, trace.Span) {
	return t.StartSpan(ctx, fmt.Sprintf("session.%s", operation), SpanKindSession,
		attribute.String(AttrSessionID, sessionID),
		attribute.String("operation", operation),
	)
}

// StartMemorySpan starts a span for memory operation
func (t *Tracer) StartMemorySpan(ctx context.Context, operation, memoryType string) (context.Context, trace.Span) {
	return t.StartSpan(ctx, fmt.Sprintf("memory.%s", operation), SpanKindMemory,
		attribute.String("operation", operation),
		attribute.String(AttrMemoryType, memoryType),
	)
}

// RecordError records an error on a span
func (t *Tracer) RecordError(span trace.Span, err error, errorType string) {
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		span.SetAttributes(
			attribute.String(AttrErrorType, errorType),
			attribute.String(AttrErrorMessage, err.Error()),
		)
	}
}

// StartMultiAgentTaskSpan starts a span for multi-agent task execution
func (t *Tracer) StartMultiAgentTaskSpan(ctx context.Context, taskID, taskName, taskType string, priority int) (context.Context, trace.Span) {
	return t.StartSpan(ctx, fmt.Sprintf("multiagent.task.%s", taskType), SpanKindMultiAgent,
		attribute.String(AttrTaskID, taskID),
		attribute.String(AttrTaskName, taskName),
		attribute.String(AttrTaskType, taskType),
		attribute.Int(AttrTaskPriority, priority),
	)
}

// StartWorkerSpan starts a span for worker task processing
func (t *Tracer) StartWorkerSpan(ctx context.Context, workerID, capability string, taskID string) (context.Context, trace.Span) {
	return t.StartSpan(ctx, fmt.Sprintf("worker.%s", capability), SpanKindWorker,
		attribute.String(AttrWorkerID, workerID),
		attribute.String(AttrWorkerCapability, capability),
		attribute.String(AttrTaskID, taskID),
	)
}

// StartProtocolSpan starts a span for protocol message operations
func (t *Tracer) StartProtocolSpan(ctx context.Context, operation, messageType, messageID string) (context.Context, trace.Span) {
	return t.StartSpan(ctx, fmt.Sprintf("protocol.%s", operation), SpanKindProtocol,
		attribute.String("operation", operation),
		attribute.String(AttrMessageType, messageType),
		attribute.String(AttrMessageID, messageID),
	)
}

// StartOrchestratorSpan starts a span for orchestrator operations
func (t *Tracer) StartOrchestratorSpan(ctx context.Context, orchestratorID, operation string) (context.Context, trace.Span) {
	return t.StartSpan(ctx, fmt.Sprintf("orchestrator.%s", operation), SpanKindMultiAgent,
		attribute.String(AttrOrchestratorID, orchestratorID),
		attribute.String("operation", operation),
	)
}

// EndSpan ends a span with optional error
func (t *Tracer) EndSpan(span trace.Span, err error) {
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

// AddEvent adds an event to a span
func (t *Tracer) AddEvent(span trace.Span, name string, attrs ...attribute.KeyValue) {
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// GetTraceID extracts the trace ID from context
func (t *Tracer) GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// GetSpanID extracts the span ID from context
func (t *Tracer) GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasSpanID() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// InjectTraceContext injects trace context into a new context
func (t *Tracer) InjectTraceContext(ctx context.Context) context.Context {
	traceID := t.GetTraceID(ctx)
	spanID := t.GetSpanID(ctx)

	if traceID != "" {
		ctx = context.WithValue(ctx, TraceIDKey, traceID)
	}
	if spanID != "" {
		ctx = context.WithValue(ctx, SpanIDKey, spanID)
	}

	return ctx
}

// stdout exporter for development
type stdoutExporter struct{}

func (e *stdoutExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	for _, span := range spans {
		fmt.Printf("[TRACE] %s | %s | %v | %v\n",
			span.Name(),
			span.SpanContext().TraceID().String(),
			span.StartTime(),
			span.EndTime().Sub(span.StartTime()),
		)
	}
	return nil
}

func (e *stdoutExporter) Shutdown(ctx context.Context) error {
	return nil
}

func stdoutExporter() (sdktrace.SpanExporter, error) {
	return &stdoutExporter{}, nil
}

// Global tracer instance
var globalTracer *Tracer

// InitGlobalTracer initializes the global tracer
func InitGlobalTracer(config TracingConfig) error {
	tracer, err := NewTracer(config)
	if err != nil {
		return err
	}
	globalTracer = tracer
	return nil
}

// GetTracer returns the global tracer
func GetTracer() *Tracer {
	if globalTracer == nil {
		// Fallback to no-op tracer
		_ = InitGlobalTracer(TracingConfig{
			Enabled:     false,
			ServiceName: "minion",
			Environment: "development",
		})
	}
	return globalTracer
}

// ShutdownTracer shuts down the global tracer
func ShutdownTracer(ctx context.Context) error {
	if globalTracer != nil {
		return globalTracer.Close(ctx)
	}
	return nil
}

// Convenience functions using global tracer

// StartSpan starts a span using global tracer
func StartSpan(ctx context.Context, name string, kind SpanKind, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return GetTracer().StartSpan(ctx, name, kind, attrs...)
}

// StartAgentSpan starts an agent span using global tracer
func StartAgentSpan(ctx context.Context, agentID, agentName, action string) (context.Context, trace.Span) {
	return GetTracer().StartAgentSpan(ctx, agentID, agentName, action)
}

// StartToolSpan starts a tool span using global tracer
func StartToolSpan(ctx context.Context, toolName string, input map[string]interface{}) (context.Context, trace.Span) {
	return GetTracer().StartToolSpan(ctx, toolName, input)
}

// StartLLMSpan starts an LLM span using global tracer
func StartLLMSpan(ctx context.Context, provider, model string) (context.Context, trace.Span) {
	return GetTracer().StartLLMSpan(ctx, provider, model)
}

// RecordError records an error using global tracer
func RecordError(span trace.Span, err error, errorType string) {
	GetTracer().RecordError(span, err, errorType)
}

// EndSpan ends a span using global tracer
func EndSpan(span trace.Span, err error) {
	GetTracer().EndSpan(span, err)
}

// Multi-agent convenience functions using global tracer

// StartMultiAgentTaskSpan starts a multi-agent task span using global tracer
func StartMultiAgentTaskSpan(ctx context.Context, taskID, taskName, taskType string, priority int) (context.Context, trace.Span) {
	return GetTracer().StartMultiAgentTaskSpan(ctx, taskID, taskName, taskType, priority)
}

// StartWorkerSpan starts a worker span using global tracer
func StartWorkerSpan(ctx context.Context, workerID, capability string, taskID string) (context.Context, trace.Span) {
	return GetTracer().StartWorkerSpan(ctx, workerID, capability, taskID)
}

// StartProtocolSpan starts a protocol span using global tracer
func StartProtocolSpan(ctx context.Context, operation, messageType, messageID string) (context.Context, trace.Span) {
	return GetTracer().StartProtocolSpan(ctx, operation, messageType, messageID)
}

// StartOrchestratorSpan starts an orchestrator span using global tracer
func StartOrchestratorSpan(ctx context.Context, orchestratorID, operation string) (context.Context, trace.Span) {
	return GetTracer().StartOrchestratorSpan(ctx, orchestratorID, operation)
}
