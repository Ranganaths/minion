package tracing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/Ranganaths/minion/agents"
)

// TraceCollector implements agents.AgentCallback and collects trace data during execution.
// It serves as middleware that wraps agent execution to capture all relevant events.
type TraceCollector struct {
	mu sync.RWMutex

	// Current trace being collected
	trace *Trace

	// Span stack for managing parent-child relationships
	spanStack []*Span

	// Current iteration number
	iterationNum int

	// Configuration
	config CollectorConfig

	// Storage backend for persisting traces
	store TraceStore

	// Hooks for real-time notifications
	hooks []TraceHook
}

// CollectorConfig configures the trace collector
type CollectorConfig struct {
	// MaxPromptLength truncates prompts longer than this (0 = no limit)
	MaxPromptLength int

	// MaxResponseLength truncates responses longer than this (0 = no limit)
	MaxResponseLength int

	// MaxToolOutputLength truncates tool outputs longer than this (0 = no limit)
	MaxToolOutputLength int

	// CaptureSensitiveData controls whether to capture potentially sensitive data
	CaptureSensitiveData bool

	// EnableRealTimeHooks enables real-time notifications via hooks
	EnableRealTimeHooks bool
}

// DefaultCollectorConfig returns sensible defaults
func DefaultCollectorConfig() CollectorConfig {
	return CollectorConfig{
		MaxPromptLength:      10000,
		MaxResponseLength:    10000,
		MaxToolOutputLength:  5000,
		CaptureSensitiveData: false,
		EnableRealTimeHooks:  true,
	}
}

// TraceHook is called in real-time as trace events occur
type TraceHook interface {
	OnTraceStart(trace *Trace)
	OnSpanStart(span *Span)
	OnSpanEnd(span *Span)
	OnTraceEnd(trace *Trace)
}

// NewTraceCollector creates a new trace collector
func NewTraceCollector(store TraceStore, config CollectorConfig) *TraceCollector {
	return &TraceCollector{
		config:    config,
		store:     store,
		spanStack: make([]*Span, 0),
		hooks:     make([]TraceHook, 0),
	}
}

// AddHook adds a real-time notification hook
func (c *TraceCollector) AddHook(hook TraceHook) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hooks = append(c.hooks, hook)
}

// StartTrace begins a new trace for an agent execution
func (c *TraceCollector) StartTrace(ctx context.Context, agentID, agentName, input string) (context.Context, TraceID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	traceID := TraceID(uuid.New().String())
	spanID := SpanID(uuid.New().String())

	c.trace = &Trace{
		ID:           traceID,
		AgentID:      agentID,
		AgentName:    agentName,
		Input:        input,
		Status:       SpanStatusRunning,
		StartTime:    time.Now(),
		RootSpanID:   spanID,
		Spans:        make([]*Span, 0),
		TotalTokens:  TokenUsage{},
		Metadata:     make(map[string]interface{}),
	}

	// Create root span
	rootSpan := &Span{
		ID:           spanID,
		TraceID:      traceID,
		Type:         SpanTypeAgentExecution,
		Name:         fmt.Sprintf("agent.%s.execute", agentName),
		Status:       SpanStatusRunning,
		StartTime:    time.Now(),
		Input:        &SpanInput{Raw: c.truncateString(input, c.config.MaxPromptLength)},
		Attributes:   make(map[string]interface{}),
		ChildSpanIDs: make([]SpanID, 0),
	}
	rootSpan.Attributes["agent_id"] = agentID
	rootSpan.Attributes["agent_name"] = agentName

	c.trace.Spans = append(c.trace.Spans, rootSpan)
	c.spanStack = []*Span{rootSpan}
	c.iterationNum = 0

	// Notify hooks
	if c.config.EnableRealTimeHooks {
		for _, hook := range c.hooks {
			hook.OnTraceStart(c.trace)
			hook.OnSpanStart(rootSpan)
		}
	}

	// Inject trace context
	ctx = ContextWithTrace(ctx, c.trace)
	ctx = ContextWithSpan(ctx, rootSpan)

	return ctx, traceID
}

// EndTrace completes the current trace
func (c *TraceCollector) EndTrace(ctx context.Context, output string, err error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.trace == nil {
		return fmt.Errorf("no active trace")
	}

	now := time.Now()

	// Close all remaining spans
	for i := len(c.spanStack) - 1; i >= 0; i-- {
		span := c.spanStack[i]
		if span.EndTime == nil {
			span.EndTime = &now
			span.Duration = now.Sub(span.StartTime).Milliseconds()
			if err != nil && span.Status == SpanStatusRunning {
				span.Status = SpanStatusError
			} else if span.Status == SpanStatusRunning {
				span.Status = SpanStatusOK
			}
		}
	}

	// Complete trace
	c.trace.Output = output
	c.trace.EndTime = &now
	c.trace.Duration = now.Sub(c.trace.StartTime).Milliseconds()

	if err != nil {
		c.trace.Status = SpanStatusError
		c.trace.Error = err.Error()
	} else {
		c.trace.Status = SpanStatusOK
	}

	// Update root span output
	if len(c.trace.Spans) > 0 {
		rootSpan := c.trace.Spans[0]
		rootSpan.Output = &SpanOutput{Raw: c.truncateString(output, c.config.MaxResponseLength)}
		if err != nil {
			rootSpan.Error = &SpanError{
				Type:    "execution_error",
				Message: err.Error(),
			}
		}
	}

	// Notify hooks
	if c.config.EnableRealTimeHooks {
		for _, hook := range c.hooks {
			hook.OnTraceEnd(c.trace)
		}
	}

	// Persist trace if store is available
	if c.store != nil {
		if storeErr := c.store.SaveTrace(ctx, c.trace); storeErr != nil {
			return fmt.Errorf("failed to save trace: %w", storeErr)
		}
	}

	return nil
}

// GetTrace returns the current trace
func (c *TraceCollector) GetTrace() *Trace {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.trace
}

// GetTraceID returns the current trace ID
func (c *TraceCollector) GetTraceID() TraceID {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.trace == nil {
		return ""
	}
	return c.trace.ID
}

// StartLLMSpan begins tracking an LLM call
func (c *TraceCollector) StartLLMSpan(ctx context.Context, provider, model, systemPrompt, userPrompt string, temperature float64, maxTokens int) (context.Context, SpanID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.trace == nil {
		return ctx, ""
	}

	spanID := SpanID(uuid.New().String())
	parentSpan := c.currentSpan()

	span := &Span{
		ID:           spanID,
		TraceID:      c.trace.ID,
		ParentSpanID: parentSpan.ID,
		Type:         SpanTypeLLMCall,
		Name:         fmt.Sprintf("llm.%s.%s", provider, model),
		Status:       SpanStatusRunning,
		StartTime:    time.Now(),
		LLMDetails: &LLMSpanDetails{
			Provider:        provider,
			Model:           model,
			SystemPrompt:    c.truncateString(systemPrompt, c.config.MaxPromptLength),
			UserPrompt:      c.truncateString(userPrompt, c.config.MaxPromptLength),
			Temperature:     temperature,
			MaxTokens:       maxTokens,
			PromptTruncated: len(systemPrompt) > c.config.MaxPromptLength || len(userPrompt) > c.config.MaxPromptLength,
		},
		Attributes:   make(map[string]interface{}),
		ChildSpanIDs: make([]SpanID, 0),
	}

	c.trace.Spans = append(c.trace.Spans, span)
	parentSpan.ChildSpanIDs = append(parentSpan.ChildSpanIDs, spanID)
	c.spanStack = append(c.spanStack, span)

	// Notify hooks
	if c.config.EnableRealTimeHooks {
		for _, hook := range c.hooks {
			hook.OnSpanStart(span)
		}
	}

	return ContextWithSpan(ctx, span), spanID
}

// EndLLMSpan completes an LLM call span
func (c *TraceCollector) EndLLMSpan(ctx context.Context, spanID SpanID, response string, promptTokens, completionTokens int, cost float64, finishReason string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	span := c.findSpan(spanID)
	if span == nil {
		return
	}

	now := time.Now()
	span.EndTime = &now
	span.Duration = now.Sub(span.StartTime).Milliseconds()

	if span.LLMDetails != nil {
		span.LLMDetails.Response = c.truncateString(response, c.config.MaxResponseLength)
		span.LLMDetails.ResponseTruncated = len(response) > c.config.MaxResponseLength
		span.LLMDetails.Tokens = TokenUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
		span.LLMDetails.Cost = cost
		span.LLMDetails.FinishReason = finishReason
	}

	// Update trace totals
	c.trace.TotalTokens.PromptTokens += promptTokens
	c.trace.TotalTokens.CompletionTokens += completionTokens
	c.trace.TotalTokens.TotalTokens += promptTokens + completionTokens
	c.trace.TotalCost += cost

	if err != nil {
		span.Status = SpanStatusError
		span.Error = &SpanError{
			Type:    "llm_error",
			Message: err.Error(),
		}
	} else {
		span.Status = SpanStatusOK
	}

	span.Output = &SpanOutput{Raw: c.truncateString(response, c.config.MaxResponseLength)}

	// Pop from stack
	c.popSpan(spanID)

	// Notify hooks
	if c.config.EnableRealTimeHooks {
		for _, hook := range c.hooks {
			hook.OnSpanEnd(span)
		}
	}
}

// StartToolSpan begins tracking a tool invocation
func (c *TraceCollector) StartToolSpan(ctx context.Context, toolName, toolInput string) (context.Context, SpanID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.trace == nil {
		return ctx, ""
	}

	spanID := SpanID(uuid.New().String())
	parentSpan := c.currentSpan()

	span := &Span{
		ID:           spanID,
		TraceID:      c.trace.ID,
		ParentSpanID: parentSpan.ID,
		Type:         SpanTypeToolCall,
		Name:         fmt.Sprintf("tool.%s", toolName),
		Status:       SpanStatusRunning,
		StartTime:    time.Now(),
		Input:        &SpanInput{Raw: c.truncateString(toolInput, c.config.MaxToolOutputLength)},
		ToolDetails: &ToolSpanDetails{
			ToolName: toolName,
			Input:    c.truncateString(toolInput, c.config.MaxToolOutputLength),
		},
		Attributes:   make(map[string]interface{}),
		ChildSpanIDs: make([]SpanID, 0),
	}

	c.trace.Spans = append(c.trace.Spans, span)
	parentSpan.ChildSpanIDs = append(parentSpan.ChildSpanIDs, spanID)
	c.spanStack = append(c.spanStack, span)
	c.trace.ToolCallCount++

	// Notify hooks
	if c.config.EnableRealTimeHooks {
		for _, hook := range c.hooks {
			hook.OnSpanStart(span)
		}
	}

	return ContextWithSpan(ctx, span), spanID
}

// EndToolSpan completes a tool invocation span
func (c *TraceCollector) EndToolSpan(ctx context.Context, spanID SpanID, output string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	span := c.findSpan(spanID)
	if span == nil {
		return
	}

	now := time.Now()
	span.EndTime = &now
	span.Duration = now.Sub(span.StartTime).Milliseconds()

	if span.ToolDetails != nil {
		span.ToolDetails.Output = c.truncateString(output, c.config.MaxToolOutputLength)
	}

	span.Output = &SpanOutput{Raw: c.truncateString(output, c.config.MaxToolOutputLength)}

	if err != nil {
		span.Status = SpanStatusError
		span.Error = &SpanError{
			Type:    "tool_error",
			Message: err.Error(),
		}
	} else {
		span.Status = SpanStatusOK
	}

	// Pop from stack
	c.popSpan(spanID)

	// Notify hooks
	if c.config.EnableRealTimeHooks {
		for _, hook := range c.hooks {
			hook.OnSpanEnd(span)
		}
	}
}

// RecordThought records an agent's reasoning/thought
func (c *TraceCollector) RecordThought(ctx context.Context, thought string, iteration int) SpanID {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.trace == nil {
		return ""
	}

	spanID := SpanID(uuid.New().String())
	parentSpan := c.currentSpan()
	now := time.Now()

	span := &Span{
		ID:           spanID,
		TraceID:      c.trace.ID,
		ParentSpanID: parentSpan.ID,
		Type:         SpanTypeThought,
		Name:         fmt.Sprintf("thought.iteration_%d", iteration),
		Status:       SpanStatusOK,
		StartTime:    now,
		EndTime:      &now,
		Duration:     0,
		ThoughtDetails: &ThoughtSpanDetails{
			Thought:   thought,
			Iteration: iteration,
		},
		Attributes: make(map[string]interface{}),
	}

	c.trace.Spans = append(c.trace.Spans, span)
	parentSpan.ChildSpanIDs = append(parentSpan.ChildSpanIDs, spanID)

	// Notify hooks
	if c.config.EnableRealTimeHooks {
		for _, hook := range c.hooks {
			hook.OnSpanStart(span)
			hook.OnSpanEnd(span)
		}
	}

	return spanID
}

// RecordDecision records an agent's decision
func (c *TraceCollector) RecordDecision(ctx context.Context, action, actionInput, thought string, iteration int, isFinal bool) SpanID {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.trace == nil {
		return ""
	}

	spanID := SpanID(uuid.New().String())
	parentSpan := c.currentSpan()
	now := time.Now()

	name := fmt.Sprintf("decision.iteration_%d", iteration)
	if isFinal {
		name = "decision.final_answer"
	}

	span := &Span{
		ID:           spanID,
		TraceID:      c.trace.ID,
		ParentSpanID: parentSpan.ID,
		Type:         SpanTypeDecision,
		Name:         name,
		Status:       SpanStatusOK,
		StartTime:    now,
		EndTime:      &now,
		Duration:     0,
		ThoughtDetails: &ThoughtSpanDetails{
			Thought:       thought,
			Action:        action,
			ActionInput:   actionInput,
			Iteration:     iteration,
			IsFinalAnswer: isFinal,
		},
		Attributes: make(map[string]interface{}),
	}

	c.trace.Spans = append(c.trace.Spans, span)
	parentSpan.ChildSpanIDs = append(parentSpan.ChildSpanIDs, spanID)

	// Notify hooks
	if c.config.EnableRealTimeHooks {
		for _, hook := range c.hooks {
			hook.OnSpanStart(span)
			hook.OnSpanEnd(span)
		}
	}

	return spanID
}

// StartIteration records the start of an agent loop iteration
func (c *TraceCollector) StartIteration(ctx context.Context) (context.Context, SpanID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.trace == nil {
		return ctx, ""
	}

	c.iterationNum++
	spanID := SpanID(uuid.New().String())
	parentSpan := c.currentSpan()

	span := &Span{
		ID:           spanID,
		TraceID:      c.trace.ID,
		ParentSpanID: parentSpan.ID,
		Type:         SpanTypeIteration,
		Name:         fmt.Sprintf("iteration_%d", c.iterationNum),
		Status:       SpanStatusRunning,
		StartTime:    time.Now(),
		Attributes:   map[string]interface{}{"iteration": c.iterationNum},
		ChildSpanIDs: make([]SpanID, 0),
	}

	c.trace.Spans = append(c.trace.Spans, span)
	parentSpan.ChildSpanIDs = append(parentSpan.ChildSpanIDs, spanID)
	c.spanStack = append(c.spanStack, span)
	c.trace.IterationCount = c.iterationNum

	// Notify hooks
	if c.config.EnableRealTimeHooks {
		for _, hook := range c.hooks {
			hook.OnSpanStart(span)
		}
	}

	return ContextWithSpan(ctx, span), spanID
}

// EndIteration completes an iteration span
func (c *TraceCollector) EndIteration(ctx context.Context, spanID SpanID, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	span := c.findSpan(spanID)
	if span == nil {
		return
	}

	now := time.Now()
	span.EndTime = &now
	span.Duration = now.Sub(span.StartTime).Milliseconds()

	if err != nil {
		span.Status = SpanStatusError
		span.Error = &SpanError{
			Type:    "iteration_error",
			Message: err.Error(),
		}
	} else {
		span.Status = SpanStatusOK
	}

	// Pop from stack
	c.popSpan(spanID)

	// Notify hooks
	if c.config.EnableRealTimeHooks {
		for _, hook := range c.hooks {
			hook.OnSpanEnd(span)
		}
	}
}

// AddEvent adds a timestamped event to the current span
func (c *TraceCollector) AddEvent(ctx context.Context, name string, attrs map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	span := c.currentSpan()
	if span == nil {
		return
	}

	event := &SpanEvent{
		Name:       name,
		Timestamp:  time.Now(),
		Attributes: attrs,
	}

	span.Events = append(span.Events, event)
}

// SetAttribute sets an attribute on the current span
func (c *TraceCollector) SetAttribute(ctx context.Context, key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	span := c.currentSpan()
	if span == nil {
		return
	}

	if span.Attributes == nil {
		span.Attributes = make(map[string]interface{})
	}
	span.Attributes[key] = value
}

// SetTraceMetadata sets metadata on the trace
func (c *TraceCollector) SetTraceMetadata(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.trace == nil {
		return
	}

	if c.trace.Metadata == nil {
		c.trace.Metadata = make(map[string]interface{})
	}
	c.trace.Metadata[key] = value
}

// SetSessionID sets the session ID on the trace
func (c *TraceCollector) SetSessionID(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.trace != nil {
		c.trace.SessionID = sessionID
	}
}

// Implement agents.AgentCallback interface

// OnAgentAction is called when the agent decides on an action
func (c *TraceCollector) OnAgentAction(ctx context.Context, action agents.AgentAction) {
	if action.Finish {
		c.RecordDecision(ctx, "", action.FinalAnswer, action.Log, c.iterationNum, true)
	} else {
		c.RecordDecision(ctx, action.Tool, action.ToolInput, action.Log, c.iterationNum, false)
	}
}

// OnAgentFinish is called when the agent completes
func (c *TraceCollector) OnAgentFinish(ctx context.Context, output string) {
	// The EndTrace will be called separately with full context
	c.AddEvent(ctx, "agent_finish", map[string]interface{}{
		"output_length": len(output),
	})
}

// OnToolStart is called before tool execution
func (c *TraceCollector) OnToolStart(ctx context.Context, tool string, input string) {
	// StartToolSpan should be called explicitly for proper span management
	// This is a fallback for callbacks
	c.AddEvent(ctx, "tool_start", map[string]interface{}{
		"tool":  tool,
		"input": input,
	})
}

// OnToolEnd is called after tool execution
func (c *TraceCollector) OnToolEnd(ctx context.Context, tool string, output string) {
	// EndToolSpan should be called explicitly for proper span management
	// This is a fallback for callbacks
	c.AddEvent(ctx, "tool_end", map[string]interface{}{
		"tool":   tool,
		"output": output,
	})
}

// OnToolError is called when a tool errors
func (c *TraceCollector) OnToolError(ctx context.Context, tool string, err error) {
	c.AddEvent(ctx, "tool_error", map[string]interface{}{
		"tool":  tool,
		"error": err.Error(),
	})
}

// Helper methods

func (c *TraceCollector) currentSpan() *Span {
	if len(c.spanStack) == 0 {
		return nil
	}
	return c.spanStack[len(c.spanStack)-1]
}

func (c *TraceCollector) findSpan(spanID SpanID) *Span {
	for _, span := range c.trace.Spans {
		if span.ID == spanID {
			return span
		}
	}
	return nil
}

func (c *TraceCollector) popSpan(spanID SpanID) {
	for i := len(c.spanStack) - 1; i >= 0; i-- {
		if c.spanStack[i].ID == spanID {
			c.spanStack = append(c.spanStack[:i], c.spanStack[i+1:]...)
			return
		}
	}
}

func (c *TraceCollector) truncateString(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Ensure TraceCollector implements AgentCallback
var _ agents.AgentCallback = (*TraceCollector)(nil)
