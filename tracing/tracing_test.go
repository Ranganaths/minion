package tracing

import (
	"context"
	"testing"
	"time"

	"github.com/Ranganaths/minion/agents"
)

func TestInMemoryTraceStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTraceStore()

	// Create a test trace
	trace := &Trace{
		ID:        "test-trace-1",
		AgentID:   "agent-1",
		AgentName: "TestAgent",
		SessionID: "session-1",
		Input:     "What is 2+2?",
		Output:    "4",
		Status:    SpanStatusOK,
		StartTime: time.Now(),
		Duration:  100,
		RootSpanID: "span-1",
		TotalTokens: TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
		TotalCost:      0.001,
		ToolCallCount:  1,
		IterationCount: 1,
		Spans: []*Span{
			{
				ID:        "span-1",
				TraceID:   "test-trace-1",
				Type:      SpanTypeAgentExecution,
				Name:      "agent.TestAgent.execute",
				Status:    SpanStatusOK,
				StartTime: time.Now(),
				Duration:  100,
			},
		},
	}

	// Test SaveTrace
	err := store.SaveTrace(ctx, trace)
	if err != nil {
		t.Fatalf("SaveTrace failed: %v", err)
	}

	// Test GetTrace
	retrieved, err := store.GetTrace(ctx, "test-trace-1")
	if err != nil {
		t.Fatalf("GetTrace failed: %v", err)
	}
	if retrieved.ID != trace.ID {
		t.Errorf("Expected trace ID %s, got %s", trace.ID, retrieved.ID)
	}
	if retrieved.AgentName != trace.AgentName {
		t.Errorf("Expected agent name %s, got %s", trace.AgentName, retrieved.AgentName)
	}

	// Test GetTracesByAgent
	traces, err := store.GetTracesByAgent(ctx, "agent-1", 10, 0)
	if err != nil {
		t.Fatalf("GetTracesByAgent failed: %v", err)
	}
	if len(traces) != 1 {
		t.Errorf("Expected 1 trace, got %d", len(traces))
	}

	// Test GetTracesBySession
	traces, err = store.GetTracesBySession(ctx, "session-1", 10, 0)
	if err != nil {
		t.Fatalf("GetTracesBySession failed: %v", err)
	}
	if len(traces) != 1 {
		t.Errorf("Expected 1 trace, got %d", len(traces))
	}

	// Test QueryTraces
	result, err := store.QueryTraces(ctx, &TraceQuery{
		Filter: TraceFilter{
			AgentID: "agent-1",
		},
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("QueryTraces failed: %v", err)
	}
	if result.TotalCount != 1 {
		t.Errorf("Expected 1 trace, got %d", result.TotalCount)
	}

	// Test GetTraceSummaries
	summaries, err := store.GetTraceSummaries(ctx, 10, 0)
	if err != nil {
		t.Fatalf("GetTraceSummaries failed: %v", err)
	}
	if len(summaries) != 1 {
		t.Errorf("Expected 1 summary, got %d", len(summaries))
	}

	// Test GetSpan
	span, err := store.GetSpan(ctx, "test-trace-1", "span-1")
	if err != nil {
		t.Fatalf("GetSpan failed: %v", err)
	}
	if span.ID != "span-1" {
		t.Errorf("Expected span ID span-1, got %s", span.ID)
	}

	// Test Stats
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.TotalTraces != 1 {
		t.Errorf("Expected 1 total trace, got %d", stats.TotalTraces)
	}

	// Test DeleteTrace
	err = store.DeleteTrace(ctx, "test-trace-1")
	if err != nil {
		t.Fatalf("DeleteTrace failed: %v", err)
	}

	_, err = store.GetTrace(ctx, "test-trace-1")
	if err == nil {
		t.Error("Expected error getting deleted trace")
	}
}

func TestTraceCollector(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTraceStore()
	collector := NewTraceCollector(store, DefaultCollectorConfig())

	// Start a trace
	ctx, traceID := collector.StartTrace(ctx, "agent-1", "TestAgent", "What is 2+2?")
	if traceID == "" {
		t.Fatal("Expected trace ID")
	}

	// Start an LLM span
	ctx, llmSpanID := collector.StartLLMSpan(ctx, "openai", "gpt-4", "You are helpful", "What is 2+2?", 0.7, 100)
	if llmSpanID == "" {
		t.Fatal("Expected LLM span ID")
	}

	// End LLM span
	collector.EndLLMSpan(ctx, llmSpanID, "4", 10, 5, 0.001, "stop", nil)

	// Start a tool span
	ctx, toolSpanID := collector.StartToolSpan(ctx, "calculator", "2+2")
	if toolSpanID == "" {
		t.Fatal("Expected tool span ID")
	}

	// End tool span
	collector.EndToolSpan(ctx, toolSpanID, "4", nil)

	// Record a thought
	thoughtSpanID := collector.RecordThought(ctx, "I need to calculate 2+2", 1)
	if thoughtSpanID == "" {
		t.Fatal("Expected thought span ID")
	}

	// Record a decision
	decisionSpanID := collector.RecordDecision(ctx, "", "4", "The answer is 4", 1, true)
	if decisionSpanID == "" {
		t.Fatal("Expected decision span ID")
	}

	// End the trace
	err := collector.EndTrace(ctx, "4", nil)
	if err != nil {
		t.Fatalf("EndTrace failed: %v", err)
	}

	// Verify the trace was saved
	trace := collector.GetTrace()
	if trace == nil {
		t.Fatal("Expected trace")
	}

	if trace.Status != SpanStatusOK {
		t.Errorf("Expected status OK, got %s", trace.Status)
	}

	if trace.Output != "4" {
		t.Errorf("Expected output '4', got '%s'", trace.Output)
	}

	if len(trace.Spans) < 4 {
		t.Errorf("Expected at least 4 spans, got %d", len(trace.Spans))
	}

	// Verify token tracking
	if trace.TotalTokens.PromptTokens != 10 {
		t.Errorf("Expected 10 prompt tokens, got %d", trace.TotalTokens.PromptTokens)
	}

	if trace.TotalTokens.CompletionTokens != 5 {
		t.Errorf("Expected 5 completion tokens, got %d", trace.TotalTokens.CompletionTokens)
	}

	// Verify tool count
	if trace.ToolCallCount != 1 {
		t.Errorf("Expected 1 tool call, got %d", trace.ToolCallCount)
	}

	// Retrieve from store
	retrieved, err := store.GetTrace(ctx, traceID)
	if err != nil {
		t.Fatalf("GetTrace failed: %v", err)
	}

	if retrieved.ID != trace.ID {
		t.Errorf("Expected trace ID %s, got %s", trace.ID, retrieved.ID)
	}
}

func TestExecutionContext(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTraceStore()
	collector := NewTraceCollector(store, DefaultCollectorConfig())

	// Start trace and create execution context
	ctx, _ = collector.StartTrace(ctx, "agent-1", "TestAgent", "test input")
	execCtx := NewExecutionContext(ctx, collector)

	// Test LLM call with context
	newCtx, spanID := execCtx.StartLLMCall("openai", "gpt-4", "system", "user", 0.7, 100)
	if spanID == "" {
		t.Fatal("Expected span ID")
	}
	newCtx.EndLLMCall(spanID, "response", 10, 5, 0.001, "stop", nil)

	// Test tool call with context
	newCtx, spanID = execCtx.StartToolCall("test_tool", "input")
	if spanID == "" {
		t.Fatal("Expected span ID")
	}
	newCtx.EndToolCall(spanID, "output", nil)

	// Test iteration
	newCtx, spanID = execCtx.StartIteration()
	if spanID == "" {
		t.Fatal("Expected span ID")
	}
	newCtx.EndIteration(spanID, nil)

	// Test adding events and attributes
	execCtx.AddEvent("test_event", map[string]interface{}{"key": "value"})
	execCtx.SetAttribute("test_attr", "test_value")
	execCtx.SetTraceMetadata("meta_key", "meta_value")

	// End trace
	collector.EndTrace(ctx, "output", nil)

	trace := execCtx.GetTrace()
	if trace == nil {
		t.Fatal("Expected trace")
	}

	if trace.Metadata["meta_key"] != "meta_value" {
		t.Errorf("Expected metadata key, got %v", trace.Metadata)
	}
}

func TestTraceSpanTree(t *testing.T) {
	now := time.Now()
	trace := &Trace{
		ID:         "trace-1",
		RootSpanID: "span-1",
		Spans: []*Span{
			{
				ID:           "span-1",
				TraceID:      "trace-1",
				Type:         SpanTypeAgentExecution,
				Name:         "root",
				StartTime:    now,
				ChildSpanIDs: []SpanID{"span-2", "span-3"},
			},
			{
				ID:           "span-2",
				TraceID:      "trace-1",
				ParentSpanID: "span-1",
				Type:         SpanTypeLLMCall,
				Name:         "llm-call",
				StartTime:    now,
				ChildSpanIDs: []SpanID{},
			},
			{
				ID:           "span-3",
				TraceID:      "trace-1",
				ParentSpanID: "span-1",
				Type:         SpanTypeToolCall,
				Name:         "tool-call",
				StartTime:    now,
				ChildSpanIDs: []SpanID{"span-4"},
			},
			{
				ID:           "span-4",
				TraceID:      "trace-1",
				ParentSpanID: "span-3",
				Type:         SpanTypeLLMCall,
				Name:         "nested-llm",
				StartTime:    now,
				ChildSpanIDs: []SpanID{},
			},
		},
	}

	tree := trace.SpanTree()
	if tree == nil {
		t.Fatal("Expected span tree")
	}

	if tree.Span.ID != "span-1" {
		t.Errorf("Expected root span ID span-1, got %s", tree.Span.ID)
	}

	if len(tree.Children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(tree.Children))
	}
}

func TestTraceFilter(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTraceStore()

	now := time.Now()

	// Create multiple traces
	traces := []*Trace{
		{
			ID:        "trace-1",
			AgentID:   "agent-1",
			AgentName: "Agent1",
			Status:    SpanStatusOK,
			StartTime: now.Add(-2 * time.Hour),
			Duration:  100,
			TotalTokens: TokenUsage{TotalTokens: 50},
			Spans: []*Span{{
				ID:        "span-1",
				TraceID:   "trace-1",
				Type:      SpanTypeToolCall,
				Name:      "tool.calculator",
				ToolDetails: &ToolSpanDetails{ToolName: "calculator"},
			}},
		},
		{
			ID:        "trace-2",
			AgentID:   "agent-2",
			AgentName: "Agent2",
			Status:    SpanStatusError,
			StartTime: now.Add(-1 * time.Hour),
			Duration:  500,
			TotalTokens: TokenUsage{TotalTokens: 100},
			Spans:     []*Span{},
		},
		{
			ID:        "trace-3",
			AgentID:   "agent-1",
			AgentName: "Agent1",
			Status:    SpanStatusOK,
			StartTime: now,
			Duration:  200,
			TotalTokens: TokenUsage{TotalTokens: 75},
			Spans:     []*Span{},
		},
	}

	for _, trace := range traces {
		store.SaveTrace(ctx, trace)
	}

	// Test filter by agent
	result, _ := store.QueryTraces(ctx, &TraceQuery{
		Filter: TraceFilter{AgentID: "agent-1"},
		Limit:  10,
	})
	if result.TotalCount != 2 {
		t.Errorf("Expected 2 traces for agent-1, got %d", result.TotalCount)
	}

	// Test filter by status
	hasError := true
	result, _ = store.QueryTraces(ctx, &TraceQuery{
		Filter: TraceFilter{HasError: &hasError},
		Limit:  10,
	})
	if result.TotalCount != 1 {
		t.Errorf("Expected 1 error trace, got %d", result.TotalCount)
	}

	// Test filter by duration
	result, _ = store.QueryTraces(ctx, &TraceQuery{
		Filter: TraceFilter{MinDuration: 200},
		Limit:  10,
	})
	if result.TotalCount != 2 {
		t.Errorf("Expected 2 traces with duration >= 200, got %d", result.TotalCount)
	}

	// Test filter by token count
	result, _ = store.QueryTraces(ctx, &TraceQuery{
		Filter: TraceFilter{MinTokens: 75},
		Limit:  10,
	})
	if result.TotalCount != 2 {
		t.Errorf("Expected 2 traces with tokens >= 75, got %d", result.TotalCount)
	}

	// Test filter by tool name
	result, _ = store.QueryTraces(ctx, &TraceQuery{
		Filter: TraceFilter{ToolName: "calculator"},
		Limit:  10,
	})
	if result.TotalCount != 1 {
		t.Errorf("Expected 1 trace with calculator tool, got %d", result.TotalCount)
	}
}

func TestContextFunctions(t *testing.T) {
	ctx := context.Background()

	// Test with no trace
	if TracingEnabled(ctx) {
		t.Error("Expected tracing not enabled")
	}

	if CurrentTraceID(ctx) != "" {
		t.Error("Expected empty trace ID")
	}

	// Add trace to context
	trace := &Trace{ID: "test-trace"}
	ctx = ContextWithTrace(ctx, trace)

	if !TracingEnabled(ctx) {
		t.Error("Expected tracing enabled")
	}

	if CurrentTraceID(ctx) != "test-trace" {
		t.Errorf("Expected trace ID test-trace, got %s", CurrentTraceID(ctx))
	}

	// Add span to context
	span := &Span{ID: "test-span"}
	ctx = ContextWithSpan(ctx, span)

	if CurrentSpanID(ctx) != "test-span" {
		t.Errorf("Expected span ID test-span, got %s", CurrentSpanID(ctx))
	}
}

func TestTraceSummary(t *testing.T) {
	trace := &Trace{
		ID:        "test-trace",
		AgentID:   "agent-1",
		AgentName: "TestAgent",
		Input:     "This is a very long input that should be truncated in the summary because it exceeds the maximum length allowed for summaries which is typically around 200 characters to keep the summaries concise and readable",
		Output:    "Short output",
		Status:    SpanStatusOK,
		StartTime: time.Now(),
		Duration:  100,
		TotalTokens: TokenUsage{
			TotalTokens: 50,
		},
		TotalCost:      0.001,
		ToolCallCount:  2,
		IterationCount: 3,
	}

	summary := trace.ToSummary()

	if summary.ID != trace.ID {
		t.Errorf("Expected ID %s, got %s", trace.ID, summary.ID)
	}

	if len(summary.Input) > 203 { // 200 + "..."
		t.Errorf("Expected truncated input, got length %d", len(summary.Input))
	}

	if summary.HasError {
		t.Error("Expected no error")
	}
}

func TestInstrumentedAgentExecutor(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTraceStore()

	// Create a mock agent
	mockAgent := &mockTestAgent{
		planFunc: func(ctx context.Context, input agents.AgentInput) (agents.AgentAction, error) {
			// Return final answer immediately
			return agents.AgentAction{
				Finish:      true,
				FinalAnswer: "The answer is 42",
				Log:         "Computed the answer",
			}, nil
		},
	}

	executor, err := NewInstrumentedAgentExecutor(InstrumentedExecutorConfig{
		Agent:           mockAgent,
		Tools:           []agents.Tool{},
		Store:           store,
		CollectorConfig: DefaultCollectorConfig(),
		AgentID:         "test-agent",
		AgentName:       "TestAgent",
	})
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	// Run the agent
	output, err := executor.Run(ctx, "What is the meaning of life?")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if output != "The answer is 42" {
		t.Errorf("Expected 'The answer is 42', got '%s'", output)
	}

	// Verify trace was saved
	summaries, err := store.GetTraceSummaries(ctx, 10, 0)
	if err != nil {
		t.Fatalf("Failed to get summaries: %v", err)
	}

	if len(summaries) != 1 {
		t.Errorf("Expected 1 trace, got %d", len(summaries))
	}

	if summaries[0].Status != SpanStatusOK {
		t.Errorf("Expected status OK, got %s", summaries[0].Status)
	}
}

func TestInstrumentedAgentExecutorWithTools(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTraceStore()

	callCount := 0
	mockAgent := &mockTestAgent{
		planFunc: func(ctx context.Context, input agents.AgentInput) (agents.AgentAction, error) {
			callCount++
			if callCount == 1 {
				// First call: use tool
				return agents.AgentAction{
					Tool:      "calculator",
					ToolInput: "2+2",
					Log:       "Need to calculate",
				}, nil
			}
			// Second call: return final answer
			return agents.AgentAction{
				Finish:      true,
				FinalAnswer: "The result is 4",
				Log:         "Got the answer",
			}, nil
		},
	}

	mockTool := &mockTestTool{
		name:        "calculator",
		description: "Does math",
		callFunc: func(ctx context.Context, input string) (string, error) {
			return "4", nil
		},
	}

	executor, err := NewInstrumentedAgentExecutor(InstrumentedExecutorConfig{
		Agent:           mockAgent,
		Tools:           []agents.Tool{mockTool},
		Store:           store,
		CollectorConfig: DefaultCollectorConfig(),
		AgentID:         "test-agent",
		AgentName:       "TestAgent",
	})
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	output, err := executor.Run(ctx, "What is 2+2?")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if output != "The result is 4" {
		t.Errorf("Expected 'The result is 4', got '%s'", output)
	}

	// Verify trace contains tool call
	traceID := executor.GetLastTraceID()
	trace, err := store.GetTrace(ctx, traceID)
	if err != nil {
		t.Fatalf("Failed to get trace: %v", err)
	}

	if trace.ToolCallCount != 1 {
		t.Errorf("Expected 1 tool call, got %d", trace.ToolCallCount)
	}

	// Find tool call span
	var foundToolSpan bool
	for _, span := range trace.Spans {
		if span.Type == SpanTypeToolCall {
			foundToolSpan = true
			if span.ToolDetails == nil {
				t.Error("Expected tool details")
			} else if span.ToolDetails.ToolName != "calculator" {
				t.Errorf("Expected tool name 'calculator', got '%s'", span.ToolDetails.ToolName)
			}
		}
	}
	if !foundToolSpan {
		t.Error("Expected to find tool call span")
	}
}

func TestTracedTool(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTraceStore()
	collector := NewTraceCollector(store, DefaultCollectorConfig())

	// Start a trace first
	ctx, _ = collector.StartTrace(ctx, "agent-1", "TestAgent", "test")

	mockTool := &mockTestTool{
		name:        "test_tool",
		description: "A test tool",
		callFunc: func(ctx context.Context, input string) (string, error) {
			return "result: " + input, nil
		},
	}

	tracedTool := NewTracedTool(mockTool, collector)

	if tracedTool.Name() != "test_tool" {
		t.Errorf("Expected name 'test_tool', got '%s'", tracedTool.Name())
	}

	if tracedTool.Description() != "A test tool" {
		t.Errorf("Expected description 'A test tool', got '%s'", tracedTool.Description())
	}

	result, err := tracedTool.Call(ctx, "hello")
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	if result != "result: hello" {
		t.Errorf("Expected 'result: hello', got '%s'", result)
	}

	// End trace and verify tool span was recorded
	collector.EndTrace(ctx, "done", nil)

	trace := collector.GetTrace()
	if trace == nil {
		t.Fatal("Expected trace")
	}

	var foundToolSpan bool
	for _, span := range trace.Spans {
		if span.Type == SpanTypeToolCall && span.Name == "tool.test_tool" {
			foundToolSpan = true
			break
		}
	}
	if !foundToolSpan {
		t.Error("Expected to find tool span")
	}
}

// Mock implementations for testing

type mockTestAgent struct {
	planFunc func(ctx context.Context, input agents.AgentInput) (agents.AgentAction, error)
}

func (a *mockTestAgent) Plan(ctx context.Context, input agents.AgentInput) (agents.AgentAction, error) {
	return a.planFunc(ctx, input)
}

func (a *mockTestAgent) InputKeys() []string  { return []string{"input"} }
func (a *mockTestAgent) OutputKeys() []string { return []string{"output"} }

type mockTestTool struct {
	name        string
	description string
	callFunc    func(ctx context.Context, input string) (string, error)
}

func (t *mockTestTool) Name() string        { return t.name }
func (t *mockTestTool) Description() string { return t.description }
func (t *mockTestTool) Call(ctx context.Context, input string) (string, error) {
	return t.callFunc(ctx, input)
}
