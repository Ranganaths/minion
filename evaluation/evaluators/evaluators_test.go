package evaluators

import (
	"context"
	"testing"
	"time"

	"github.com/Ranganaths/minion/tracing"
)

func TestProductivityEvaluator(t *testing.T) {
	ctx := context.Background()
	eval := NewProductivityEvaluator()

	tests := []struct {
		name     string
		trace    *tracing.Trace
		wantMin  float64
		wantMax  float64
	}{
		{
			name: "successful task with low iterations",
			trace: &tracing.Trace{
				ID:             "trace-1",
				AgentID:        "agent-1",
				Status:         tracing.SpanStatusOK,
				IterationCount: 2,
				Duration:       1000,
				TotalTokens:    tracing.TokenUsage{TotalTokens: 100},
				Spans: []*tracing.Span{
					{Type: tracing.SpanTypeToolCall, Status: tracing.SpanStatusOK},
				},
			},
			wantMin: 0.7,
			wantMax: 1.0,
		},
		{
			name: "failed task",
			trace: &tracing.Trace{
				ID:             "trace-2",
				AgentID:        "agent-1",
				Status:         tracing.SpanStatusError,
				IterationCount: 15,
				Duration:       30000,
				TotalTokens:    tracing.TokenUsage{TotalTokens: 1000},
				Spans: []*tracing.Span{
					{Type: tracing.SpanTypeToolCall, Status: tracing.SpanStatusError},
				},
			},
			wantMin: 0.0,
			wantMax: 0.4,
		},
		{
			name: "task with no tools",
			trace: &tracing.Trace{
				ID:             "trace-3",
				AgentID:        "agent-1",
				Status:         tracing.SpanStatusOK,
				IterationCount: 1,
				Duration:       500,
				TotalTokens:    tracing.TokenUsage{TotalTokens: 50},
				Spans:          []*tracing.Span{},
			},
			wantMin: 0.5,
			wantMax: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.Evaluate(ctx, tt.trace)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}

			if result.Score < tt.wantMin || result.Score > tt.wantMax {
				t.Errorf("Score = %v, want between %v and %v", result.Score, tt.wantMin, tt.wantMax)
			}

			if result.Metrics == nil {
				t.Error("Metrics should not be nil")
			}
		})
	}
}

func TestLatencyEvaluator(t *testing.T) {
	ctx := context.Background()
	eval := NewLatencyEvaluator()

	tests := []struct {
		name    string
		trace   *tracing.Trace
		wantMin float64
		wantMax float64
	}{
		{
			name: "fast response",
			trace: &tracing.Trace{
				ID:             "trace-1",
				AgentID:        "agent-1",
				Status:         tracing.SpanStatusOK,
				Duration:       1000, // 1 second
				IterationCount: 1,
				Spans: []*tracing.Span{
					{
						Type:     tracing.SpanTypeLLMCall,
						Duration: 500,
						LLMDetails: &tracing.LLMSpanDetails{
							Tokens: tracing.TokenUsage{TotalTokens: 100},
						},
					},
				},
			},
			wantMin: 0.7,
			wantMax: 1.0,
		},
		{
			name: "slow response",
			trace: &tracing.Trace{
				ID:             "trace-2",
				AgentID:        "agent-1",
				Status:         tracing.SpanStatusOK,
				Duration:       60000, // 60 seconds
				IterationCount: 10,
				Spans:          []*tracing.Span{},
			},
			wantMin: 0.0,
			wantMax: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.Evaluate(ctx, tt.trace)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}

			if result.Score < tt.wantMin || result.Score > tt.wantMax {
				t.Errorf("Score = %v, want between %v and %v", result.Score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCostEvaluator(t *testing.T) {
	ctx := context.Background()
	eval := NewCostEvaluator()

	tests := []struct {
		name    string
		trace   *tracing.Trace
		wantMin float64
		wantMax float64
	}{
		{
			name: "low cost task",
			trace: &tracing.Trace{
				ID:          "trace-1",
				AgentID:     "agent-1",
				Status:      tracing.SpanStatusOK,
				TotalCost:   0.001,
				TotalTokens: tracing.TokenUsage{TotalTokens: 100, PromptTokens: 60, CompletionTokens: 40},
			},
			wantMin: 0.7,
			wantMax: 1.0,
		},
		{
			name: "high cost task",
			trace: &tracing.Trace{
				ID:          "trace-2",
				AgentID:     "agent-1",
				Status:      tracing.SpanStatusOK,
				TotalCost:   0.5,
				TotalTokens: tracing.TokenUsage{TotalTokens: 10000, PromptTokens: 6000, CompletionTokens: 4000},
			},
			wantMin: 0.0,
			wantMax: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.Evaluate(ctx, tt.trace)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}

			if result.Score < tt.wantMin || result.Score > tt.wantMax {
				t.Errorf("Score = %v, want between %v and %v", result.Score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestErrorEvaluator(t *testing.T) {
	ctx := context.Background()
	eval := NewErrorEvaluator()

	tests := []struct {
		name    string
		trace   *tracing.Trace
		wantMin float64
		wantMax float64
	}{
		{
			name: "no errors",
			trace: &tracing.Trace{
				ID:      "trace-1",
				AgentID: "agent-1",
				Status:  tracing.SpanStatusOK,
				Spans: []*tracing.Span{
					{Type: tracing.SpanTypeToolCall, Status: tracing.SpanStatusOK},
					{Type: tracing.SpanTypeToolCall, Status: tracing.SpanStatusOK},
				},
			},
			wantMin: 0.9,
			wantMax: 1.0,
		},
		{
			name: "with errors but recovered",
			trace: &tracing.Trace{
				ID:      "trace-2",
				AgentID: "agent-1",
				Status:  tracing.SpanStatusOK,
				Spans: []*tracing.Span{
					{Type: tracing.SpanTypeToolCall, Status: tracing.SpanStatusError},
					{Type: tracing.SpanTypeToolCall, Status: tracing.SpanStatusOK},
				},
			},
			wantMin: 0.5,
			wantMax: 0.9,
		},
		{
			name: "failed with errors",
			trace: &tracing.Trace{
				ID:      "trace-3",
				AgentID: "agent-1",
				Status:  tracing.SpanStatusError,
				Error:   "task failed",
				Spans: []*tracing.Span{
					{Type: tracing.SpanTypeToolCall, Status: tracing.SpanStatusError},
					{Type: tracing.SpanTypeToolCall, Status: tracing.SpanStatusError},
				},
			},
			wantMin: 0.0,
			wantMax: 0.4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.Evaluate(ctx, tt.trace)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}

			if result.Score < tt.wantMin || result.Score > tt.wantMax {
				t.Errorf("Score = %v, want between %v and %v", result.Score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestToolUsageEvaluator(t *testing.T) {
	ctx := context.Background()
	eval := NewToolUsageEvaluator()

	tests := []struct {
		name    string
		trace   *tracing.Trace
		wantMin float64
		wantMax float64
	}{
		{
			name: "efficient tool usage",
			trace: &tracing.Trace{
				ID:             "trace-1",
				AgentID:        "agent-1",
				Status:         tracing.SpanStatusOK,
				IterationCount: 2,
				Spans: []*tracing.Span{
					{
						Type:        tracing.SpanTypeToolCall,
						Status:      tracing.SpanStatusOK,
						ToolDetails: &tracing.ToolSpanDetails{ToolName: "search"},
					},
					{
						Type:        tracing.SpanTypeToolCall,
						Status:      tracing.SpanStatusOK,
						ToolDetails: &tracing.ToolSpanDetails{ToolName: "calculator"},
					},
				},
			},
			wantMin: 0.7,
			wantMax: 1.0,
		},
		{
			name: "redundant tool calls",
			trace: &tracing.Trace{
				ID:             "trace-2",
				AgentID:        "agent-1",
				Status:         tracing.SpanStatusOK,
				IterationCount: 2,
				Spans: []*tracing.Span{
					{Type: tracing.SpanTypeToolCall, Status: tracing.SpanStatusOK, ToolDetails: &tracing.ToolSpanDetails{ToolName: "search"}},
					{Type: tracing.SpanTypeToolCall, Status: tracing.SpanStatusOK, ToolDetails: &tracing.ToolSpanDetails{ToolName: "search"}},
					{Type: tracing.SpanTypeToolCall, Status: tracing.SpanStatusOK, ToolDetails: &tracing.ToolSpanDetails{ToolName: "search"}},
					{Type: tracing.SpanTypeToolCall, Status: tracing.SpanStatusOK, ToolDetails: &tracing.ToolSpanDetails{ToolName: "search"}},
				},
			},
			wantMin: 0.6, // All tools succeeded, so score is decent despite redundancy
			wantMax: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.Evaluate(ctx, tt.trace)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}

			if result.Score < tt.wantMin || result.Score > tt.wantMax {
				t.Errorf("Score = %v, want between %v and %v", result.Score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCompositeEvaluator(t *testing.T) {
	ctx := context.Background()

	// Create composite with equal weights
	composite := NewCompositeEvaluator(
		WeightedEvaluator{Evaluator: NewProductivityEvaluator(), Weight: 0.5},
		WeightedEvaluator{Evaluator: NewErrorEvaluator(), Weight: 0.5},
	)

	trace := &tracing.Trace{
		ID:             "trace-1",
		AgentID:        "agent-1",
		Status:         tracing.SpanStatusOK,
		IterationCount: 2,
		Duration:       1000,
		TotalTokens:    tracing.TokenUsage{TotalTokens: 100},
		Spans: []*tracing.Span{
			{Type: tracing.SpanTypeToolCall, Status: tracing.SpanStatusOK},
		},
	}

	result, err := composite.Evaluate(ctx, trace)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if result.Score < 0.6 || result.Score > 1.0 {
		t.Errorf("Composite score = %v, expected between 0.6 and 1.0", result.Score)
	}

	// Check metadata
	if result.Metadata == nil {
		t.Error("Metadata should contain component scores")
	}
}

func TestCompositeEvaluatorParallel(t *testing.T) {
	ctx := context.Background()

	composite := NewCompositeEvaluator(
		WeightedEvaluator{Evaluator: NewProductivityEvaluator(), Weight: 0.25},
		WeightedEvaluator{Evaluator: NewLatencyEvaluator(), Weight: 0.25},
		WeightedEvaluator{Evaluator: NewCostEvaluator(), Weight: 0.25},
		WeightedEvaluator{Evaluator: NewErrorEvaluator(), Weight: 0.25},
	).SetParallel(true)

	trace := &tracing.Trace{
		ID:             "trace-1",
		AgentID:        "agent-1",
		Status:         tracing.SpanStatusOK,
		IterationCount: 2,
		Duration:       1000,
		TotalTokens:    tracing.TokenUsage{TotalTokens: 100},
		TotalCost:      0.001,
		Spans:          []*tracing.Span{},
	}

	result, err := composite.Evaluate(ctx, trace)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if result.Score < 0 || result.Score > 1 {
		t.Errorf("Score = %v, expected between 0 and 1", result.Score)
	}
}

func TestEvaluateBatch(t *testing.T) {
	ctx := context.Background()
	eval := NewProductivityEvaluator()

	traces := []*tracing.Trace{
		{
			ID:             "trace-1",
			AgentID:        "agent-1",
			Status:         tracing.SpanStatusOK,
			IterationCount: 2,
		},
		{
			ID:             "trace-2",
			AgentID:        "agent-1",
			Status:         tracing.SpanStatusOK,
			IterationCount: 3,
		},
	}

	results, err := eval.EvaluateBatch(ctx, traces)
	if err != nil {
		t.Fatalf("EvaluateBatch() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestDefaultEvaluators(t *testing.T) {
	evaluators := DefaultEvaluators()
	if len(evaluators) != 5 {
		t.Errorf("Expected 5 default evaluators, got %d", len(evaluators))
	}
}

func TestDefaultCompositeEvaluator(t *testing.T) {
	ctx := context.Background()
	composite := DefaultCompositeEvaluator()

	trace := &tracing.Trace{
		ID:             "trace-1",
		AgentID:        "agent-1",
		Status:         tracing.SpanStatusOK,
		IterationCount: 2,
		Duration:       1000,
		TotalTokens:    tracing.TokenUsage{TotalTokens: 100},
		TotalCost:      0.001,
		StartTime:      time.Now().Add(-time.Second),
		Spans:          []*tracing.Span{},
	}

	result, err := composite.Evaluate(ctx, trace)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if result.Type != "composite" {
		t.Errorf("Expected type 'composite', got %s", result.Type)
	}
}
