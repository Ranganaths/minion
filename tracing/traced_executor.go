package tracing

import (
	"context"
	"fmt"

	"github.com/Ranganaths/minion/agents"
)

// TracedAgentExecutor wraps an AgentExecutor with tracing capabilities
type TracedAgentExecutor struct {
	executor  agents.AgentExecutor
	collector *TraceCollector
	agentID   string
	agentName string
}

// TracedExecutorConfig configures the traced executor
type TracedExecutorConfig struct {
	// Executor is the underlying agent executor (required)
	Executor agents.AgentExecutor

	// Store is the trace storage backend (required)
	Store TraceStore

	// CollectorConfig configures the trace collector
	CollectorConfig CollectorConfig

	// AgentID identifies the agent
	AgentID string

	// AgentName is the human-readable agent name
	AgentName string

	// Hooks are real-time notification hooks
	Hooks []TraceHook
}

// NewTracedAgentExecutor creates a new traced agent executor
func NewTracedAgentExecutor(cfg TracedExecutorConfig) (*TracedAgentExecutor, error) {
	if cfg.Executor == nil {
		return nil, fmt.Errorf("executor is required")
	}
	if cfg.Store == nil {
		return nil, fmt.Errorf("store is required")
	}

	collector := NewTraceCollector(cfg.Store, cfg.CollectorConfig)

	// Add hooks
	for _, hook := range cfg.Hooks {
		collector.AddHook(hook)
	}

	return &TracedAgentExecutor{
		executor:  cfg.Executor,
		collector: collector,
		agentID:   cfg.AgentID,
		agentName: cfg.AgentName,
	}, nil
}

// Run executes the agent with tracing
func (e *TracedAgentExecutor) Run(ctx context.Context, input string) (string, error) {
	return e.RunWithHistory(ctx, input, "")
}

// RunWithHistory executes the agent with conversation history and tracing
func (e *TracedAgentExecutor) RunWithHistory(ctx context.Context, input string, history string) (string, error) {
	// Start trace
	ctx, _ = e.collector.StartTrace(ctx, e.agentID, e.agentName, input)

	// Execute the underlying agent
	output, err := e.executor.RunWithHistory(ctx, input, history)

	// End trace
	if traceErr := e.collector.EndTrace(ctx, output, err); traceErr != nil {
		// Log trace error but don't fail the execution
		fmt.Printf("Warning: failed to save trace: %v\n", traceErr)
	}

	return output, err
}

// Stream executes and streams intermediate steps with tracing
func (e *TracedAgentExecutor) Stream(ctx context.Context, input string) (<-chan agents.AgentStreamEvent, error) {
	// Start trace
	ctx, _ = e.collector.StartTrace(ctx, e.agentID, e.agentName, input)

	// Create a wrapper channel
	innerCh, err := e.executor.Stream(ctx, input)
	if err != nil {
		e.collector.EndTrace(ctx, "", err)
		return nil, err
	}

	outputCh := make(chan agents.AgentStreamEvent)

	go func() {
		defer close(outputCh)

		var finalOutput string
		var finalErr error

		for event := range innerCh {
			// Record event in trace
			switch event.Type {
			case agents.AgentEventThought:
				if event.Step != nil {
					e.collector.RecordThought(ctx, event.Step.Action.Log, e.collector.iterationNum)
				}
			case agents.AgentEventAction:
				if event.Step != nil {
					e.collector.RecordDecision(ctx, event.Step.Action.Tool, event.Step.Action.ToolInput, event.Step.Action.Log, e.collector.iterationNum, false)
				}
			case agents.AgentEventFinish:
				finalOutput = event.FinalAnswer
				e.collector.RecordDecision(ctx, "", finalOutput, "", e.collector.iterationNum, true)
			case agents.AgentEventError:
				finalErr = event.Error
			}

			// Forward event
			outputCh <- event
		}

		// End trace
		if traceErr := e.collector.EndTrace(ctx, finalOutput, finalErr); traceErr != nil {
			fmt.Printf("Warning: failed to save trace: %v\n", traceErr)
		}
	}()

	return outputCh, nil
}

// GetCollector returns the trace collector for advanced usage
func (e *TracedAgentExecutor) GetCollector() *TraceCollector {
	return e.collector
}

// GetLastTraceID returns the ID of the last completed trace
func (e *TracedAgentExecutor) GetLastTraceID() TraceID {
	return e.collector.GetTraceID()
}

// SetSessionID sets the session ID for subsequent traces
func (e *TracedAgentExecutor) SetSessionID(sessionID string) {
	e.collector.SetSessionID(sessionID)
}

// Ensure TracedAgentExecutor implements AgentExecutor
var _ agents.AgentExecutor = (*TracedAgentExecutor)(nil)

// TracedAgent wraps an Agent with tracing for the Plan method
type TracedAgent struct {
	agent     agents.Agent
	collector *TraceCollector
}

// NewTracedAgent creates a new traced agent wrapper
func NewTracedAgent(agent agents.Agent, collector *TraceCollector) *TracedAgent {
	return &TracedAgent{
		agent:     agent,
		collector: collector,
	}
}

// Plan wraps the agent's Plan method with tracing
func (a *TracedAgent) Plan(ctx context.Context, input agents.AgentInput) (agents.AgentAction, error) {
	// Record iteration start
	ctx, iterSpanID := a.collector.StartIteration(ctx)
	defer func() {
		a.collector.EndIteration(ctx, iterSpanID, nil)
	}()

	// Execute plan
	action, err := a.agent.Plan(ctx, input)

	if err != nil {
		return action, err
	}

	// Record the decision
	if action.Finish {
		a.collector.RecordDecision(ctx, "", action.FinalAnswer, action.Log, a.collector.iterationNum, true)
	} else {
		a.collector.RecordDecision(ctx, action.Tool, action.ToolInput, action.Log, a.collector.iterationNum, false)
	}

	return action, nil
}

// InputKeys delegates to the underlying agent
func (a *TracedAgent) InputKeys() []string {
	return a.agent.InputKeys()
}

// OutputKeys delegates to the underlying agent
func (a *TracedAgent) OutputKeys() []string {
	return a.agent.OutputKeys()
}

// Ensure TracedAgent implements Agent
var _ agents.Agent = (*TracedAgent)(nil)

// WebSocketTraceHook implements TraceHook for real-time WebSocket updates
type WebSocketTraceHook struct {
	// SendFunc is called to send updates
	SendFunc func(event interface{})
}

// OnTraceStart is called when a trace starts
func (h *WebSocketTraceHook) OnTraceStart(trace *Trace) {
	if h.SendFunc != nil {
		h.SendFunc(map[string]interface{}{
			"type":      "trace_start",
			"traceID":   trace.ID,
			"agentID":   trace.AgentID,
			"agentName": trace.AgentName,
			"input":     trace.Input,
			"startTime": trace.StartTime,
		})
	}
}

// OnSpanStart is called when a span starts
func (h *WebSocketTraceHook) OnSpanStart(span *Span) {
	if h.SendFunc != nil {
		h.SendFunc(map[string]interface{}{
			"type":         "span_start",
			"spanID":       span.ID,
			"traceID":      span.TraceID,
			"parentSpanID": span.ParentSpanID,
			"spanType":     span.Type,
			"name":         span.Name,
			"startTime":    span.StartTime,
		})
	}
}

// OnSpanEnd is called when a span ends
func (h *WebSocketTraceHook) OnSpanEnd(span *Span) {
	if h.SendFunc != nil {
		h.SendFunc(map[string]interface{}{
			"type":      "span_end",
			"spanID":    span.ID,
			"traceID":   span.TraceID,
			"spanType":  span.Type,
			"name":      span.Name,
			"status":    span.Status,
			"duration":  span.Duration,
			"endTime":   span.EndTime,
		})
	}
}

// OnTraceEnd is called when a trace ends
func (h *WebSocketTraceHook) OnTraceEnd(trace *Trace) {
	if h.SendFunc != nil {
		h.SendFunc(map[string]interface{}{
			"type":       "trace_end",
			"traceID":    trace.ID,
			"status":     trace.Status,
			"output":     trace.Output,
			"error":      trace.Error,
			"duration":   trace.Duration,
			"endTime":    trace.EndTime,
			"totalTokens": trace.TotalTokens,
			"totalCost":  trace.TotalCost,
		})
	}
}

// Ensure WebSocketTraceHook implements TraceHook
var _ TraceHook = (*WebSocketTraceHook)(nil)

// ConsoleTraceHook implements TraceHook for console output
type ConsoleTraceHook struct {
	// Verbose enables verbose output
	Verbose bool
}

// OnTraceStart is called when a trace starts
func (h *ConsoleTraceHook) OnTraceStart(trace *Trace) {
	fmt.Printf("\n[TRACE START] %s - Agent: %s\n", trace.ID, trace.AgentName)
	if h.Verbose {
		fmt.Printf("  Input: %s\n", truncate(trace.Input, 100))
	}
}

// OnSpanStart is called when a span starts
func (h *ConsoleTraceHook) OnSpanStart(span *Span) {
	if h.Verbose {
		fmt.Printf("  [%s] %s started\n", span.Type, span.Name)
	}
}

// OnSpanEnd is called when a span ends
func (h *ConsoleTraceHook) OnSpanEnd(span *Span) {
	if h.Verbose {
		statusIcon := "✓"
		if span.Status == SpanStatusError {
			statusIcon = "✗"
		}
		fmt.Printf("  [%s] %s %s (%dms)\n", span.Type, span.Name, statusIcon, span.Duration)
	}
}

// OnTraceEnd is called when a trace ends
func (h *ConsoleTraceHook) OnTraceEnd(trace *Trace) {
	statusIcon := "✓"
	if trace.Status == SpanStatusError {
		statusIcon = "✗"
	}
	fmt.Printf("[TRACE END] %s %s - %dms, %d tokens, $%.4f\n",
		trace.ID, statusIcon, trace.Duration, trace.TotalTokens.TotalTokens, trace.TotalCost)
	if h.Verbose && trace.Output != "" {
		fmt.Printf("  Output: %s\n", truncate(trace.Output, 100))
	}
	if trace.Error != "" {
		fmt.Printf("  Error: %s\n", trace.Error)
	}
	fmt.Println()
}

// Ensure ConsoleTraceHook implements TraceHook
var _ TraceHook = (*ConsoleTraceHook)(nil)

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
