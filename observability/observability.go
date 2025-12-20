package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/Ranganaths/minion/config"
)

// Observability is the main interface for the observability stack
type Observability struct {
	Logger      Logger
	Tracer      *Tracer
	Metrics     *MetricsCollector
	CostTracker *CostTracker
	config      *config.Config
}

// New creates a new observability stack
func New(cfg *config.Config) (*Observability, error) {
	// Initialize Logger
	loggerConfig := &LoggerConfig{
		Level:      LogLevel(cfg.App.LogLevel),
		JSONOutput: cfg.Observability.Logging.Format == "json",
		WithCaller: true,
	}

	logger := NewLogger(loggerConfig)

	// Initialize Tracer
	tracingConfig := TracingConfig{
		Enabled:       cfg.Observability.Tracing.Enabled,
		ServiceName:   cfg.Observability.Tracing.ServiceName,
		Environment:   cfg.App.Env,
		Exporter:      cfg.Observability.Tracing.Exporter,
		JaegerURL:     cfg.Observability.Tracing.JaegerURL,
		OTLPEndpoint:  cfg.Observability.Tracing.OTLPEndpoint,
		SamplingRatio: cfg.Observability.Tracing.SamplingRatio,
	}

	tracer, err := NewTracer(tracingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tracer: %w", err)
	}

	// Set as global tracer
	if err := InitGlobalTracer(tracingConfig); err != nil {
		return nil, fmt.Errorf("failed to initialize global tracer: %w", err)
	}

	if tracingConfig.Enabled {
		logger.Info(fmt.Sprintf("Tracer initialized successfully (exporter: %s)", tracingConfig.Exporter))
	}

	// Initialize Metrics
	metricsConfig := MetricsConfig{
		Enabled:           cfg.Observability.Metrics.Enabled,
		Port:              cfg.Observability.Metrics.Port,
		Path:              cfg.Observability.Metrics.Path,
		PrometheusEnabled: cfg.Observability.Metrics.PrometheusEnabled,
	}

	metrics := NewMetricsCollector(metricsConfig, nil)

	// Set as global metrics
	if err := InitGlobalMetrics(metricsConfig); err != nil {
		return nil, fmt.Errorf("failed to initialize global metrics: %w", err)
	}

	if metricsConfig.Enabled {
		logger.Info(fmt.Sprintf("Metrics collector initialized successfully (port: %d)", metricsConfig.Port))
	}

	// Initialize Cost Tracker
	costConfig := CostConfig{
		Enabled:              cfg.Observability.Cost.Enabled,
		PricingFile:          cfg.Observability.Cost.PricingFile,
		BudgetAlertThreshold: cfg.Observability.Cost.BudgetAlertThreshold,
		Currency:             cfg.Observability.Cost.Currency,
	}

	costTracker, err := NewCostTracker(costConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cost tracker: %w", err)
	}

	// Set as global cost tracker
	if err := InitGlobalCostTracker(costConfig); err != nil {
		return nil, fmt.Errorf("failed to initialize global cost tracker: %w", err)
	}

	if costConfig.Enabled {
		logger.Info(fmt.Sprintf("Cost tracker initialized successfully (budget: $%.2f/day)", costConfig.BudgetAlertThreshold))
	}

	return &Observability{
		Logger:      logger,
		Tracer:      tracer,
		Metrics:     metrics,
		CostTracker: costTracker,
		config:      cfg,
	}, nil
}

// Close gracefully shuts down the observability stack
func (o *Observability) Close(ctx context.Context) error {
	o.Logger.Info("Shutting down observability stack")

	// Shutdown tracer
	if err := o.Tracer.Close(ctx); err != nil {
		o.Logger.Error("Failed to shutdown tracer", Err(err))
		return err
	}

	// Export cost records before shutting down
	if o.config.Observability.Cost.Enabled {
		filename := fmt.Sprintf("cost_export_%s.json", o.config.App.Env)
		if err := o.CostTracker.ExportRecords(filename); err != nil {
			o.Logger.Warn(fmt.Sprintf("Failed to export cost records: %v", err))
		} else {
			o.Logger.Info(fmt.Sprintf("Cost records exported to %s", filename))
		}
	}

	o.Logger.Info("Observability stack shutdown complete")
	return nil
}

// StartMetricsServer starts the Prometheus metrics HTTP server
// This should be run in a separate goroutine
func (o *Observability) StartMetricsServer() error {
	if !o.config.Observability.Metrics.Enabled {
		return nil
	}

	o.Logger.Info(fmt.Sprintf("Starting metrics server on port %d", o.config.Observability.Metrics.Port))
	return o.Metrics.StartMetricsServer()
}

// Helper methods for common observability operations

// ObserveAgentExecution provides a complete observability wrapper for agent execution
func (o *Observability) ObserveAgentExecution(
	ctx context.Context,
	agentID, agentName, action string,
	fn func(ctx context.Context) error,
) error {
	// Start tracing span
	ctx, span := o.Tracer.StartAgentSpan(ctx, agentID, agentName, action)
	defer span.End()

	// Inject trace context for logging
	ctx = o.Tracer.InjectTraceContext(ctx)

	// Log start
	logger := o.Logger.WithContext(ctx)
	logger.Info("Starting agent execution", String("agent_id", agentID), String("action", action))

	// Execute function with timing
	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start)

	// Record metrics
	o.Metrics.RecordAgentExecution(agentID, agentName, duration, err)

	// Log completion
	if err != nil {
		logger.Error("Agent execution failed", String("agent_id", agentID), String("action", action), Duration("duration", duration), Err(err))
	} else {
		logger.Info("Agent execution completed", String("agent_id", agentID), String("action", action), Duration("duration", duration))
	}

	// Record error in span if present
	if err != nil {
		o.Tracer.RecordError(span, err, "agent_execution_error")
	}

	return err
}

// ObserveToolCall provides a complete observability wrapper for tool execution
func (o *Observability) ObserveToolCall(
	ctx context.Context,
	toolName string,
	input map[string]interface{},
	fn func(ctx context.Context) error,
) error {
	// Start tracing span
	ctx, span := o.Tracer.StartToolSpan(ctx, toolName, input)
	defer span.End()

	// Log start
	logger := o.Logger.WithContext(ctx)
	logger.Info("Starting tool call", String("tool_name", toolName))

	// Execute function with timing
	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start)

	// Record metrics
	o.Metrics.RecordToolCall(toolName, duration, err)

	// Log completion
	if err != nil {
		logger.Error("Tool call failed", String("tool_name", toolName), Duration("duration", duration), Err(err))
	} else {
		logger.Info("Tool call completed", String("tool_name", toolName), Duration("duration", duration))
	}

	// Record error in span if present
	if err != nil {
		o.Tracer.RecordError(span, err, "tool_execution_error")
	}

	return err
}

// ObserveLLMCall provides a complete observability wrapper for LLM API calls
func (o *Observability) ObserveLLMCall(
	ctx context.Context,
	agentID, sessionID, provider, model string,
	fn func(ctx context.Context) (promptTokens, completionTokens int, err error),
) error {
	// Start tracing span
	ctx, span := o.Tracer.StartLLMSpan(ctx, provider, model)
	defer span.End()

	// Log start
	logger := o.Logger.WithContext(ctx)
	logger.Info("Starting LLM call", String("provider", provider), String("model", model))

	// Execute function with timing
	start := time.Now()
	promptTokens, completionTokens, err := fn(ctx)
	duration := time.Since(start)

	// Calculate cost
	cost := o.CostTracker.RecordCost(ctx, agentID, sessionID, provider, model, promptTokens, completionTokens)

	// Record token usage in span
	o.Tracer.RecordLLMTokens(span, promptTokens, completionTokens, cost)

	// Record metrics
	o.Metrics.RecordLLMRequest(provider, model, duration, promptTokens, completionTokens, cost, err)

	// Log completion
	if err != nil {
		logger.Error("LLM call failed", String("provider", provider), String("model", model), Duration("duration", duration), Err(err))
	} else {
		logger.Info("LLM call completed",
			String("provider", provider),
			String("model", model),
			Int("prompt_tokens", promptTokens),
			Int("completion_tokens", completionTokens),
			Float64("cost", cost),
			Duration("duration", duration))
	}

	// Record error in span if present
	if err != nil {
		o.Tracer.RecordError(span, err, "llm_api_error")
	}

	return err
}

// ObserveStorageOperation provides a complete observability wrapper for storage operations
func (o *Observability) ObserveStorageOperation(
	ctx context.Context,
	operation, table string,
	fn func(ctx context.Context) error,
) error {
	// Start tracing span
	ctx, span := o.Tracer.StartStorageSpan(ctx, operation, table)
	defer span.End()

	// Execute function with timing
	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start)

	// Record metrics
	o.Metrics.RecordStorageOperation(operation, table, duration, err)

	// Log operation (only on error)
	if err != nil {
		logger := o.Logger.WithContext(ctx)
		logger.Error("Storage operation failed", String("operation", operation), String("table", table), Duration("duration", duration), Err(err))
	}

	// Record error in span if present
	if err != nil {
		o.Tracer.RecordError(span, err, "storage_error")
	}

	return err
}

// GetLogger returns the logger with context
func (o *Observability) GetLogger(ctx context.Context) Logger {
	return o.Logger.WithContext(ctx)
}

// GetTraceID returns the trace ID from context
func (o *Observability) GetTraceID(ctx context.Context) string {
	return o.Tracer.GetTraceID(ctx)
}

// LogSecurityEvent logs a security-related event
func (o *Observability) LogSecurityEvent(ctx context.Context, eventType, description, severity string) {
	o.Logger.WithContext(ctx).Warn("Security event",
		String("event_type", eventType),
		String("description", description),
		String("severity", severity))
}

// GetCostSummary returns the daily cost summary
func (o *Observability) GetCostSummary() *CostSummary {
	return o.CostTracker.GetDailySummary()
}
