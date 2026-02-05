package evaluators

import (
	"context"

	"github.com/Ranganaths/minion/evaluation"
	"github.com/Ranganaths/minion/tracing"
)

const (
	// ProductivityEvaluatorID is the ID for the productivity evaluator
	ProductivityEvaluatorID = "productivity"
	// ProductivityEvaluatorName is the name for the productivity evaluator
	ProductivityEvaluatorName = "Productivity Evaluator"
)

// ProductivityEvaluator evaluates task completion and efficiency metrics
type ProductivityEvaluator struct {
	*evaluation.BaseEvaluator
}

// NewProductivityEvaluator creates a new productivity evaluator
func NewProductivityEvaluator() *ProductivityEvaluator {
	return &ProductivityEvaluator{
		BaseEvaluator: evaluation.NewBaseEvaluator(
			ProductivityEvaluatorID,
			ProductivityEvaluatorName,
			evaluation.TypeProductivity,
		),
	}
}

// Evaluate evaluates productivity metrics for a trace
func (e *ProductivityEvaluator) Evaluate(ctx context.Context, trace *tracing.Trace) (*evaluation.Evaluation, error) {
	metrics := e.extractMetrics(trace)
	score := e.calculateScore(metrics, trace)

	eval := e.CreateEvaluation(trace, score)
	eval.Metrics = metrics
	eval.Subscores = e.calculateSubscores(metrics, trace)

	return eval, nil
}

// EvaluateBatch evaluates multiple traces
func (e *ProductivityEvaluator) EvaluateBatch(ctx context.Context, traces []*tracing.Trace) ([]*evaluation.Evaluation, error) {
	return e.BaseEvaluator.EvaluateBatch(ctx, traces, e.Evaluate)
}

// extractMetrics extracts productivity metrics from a trace
func (e *ProductivityEvaluator) extractMetrics(trace *tracing.Trace) *evaluation.EvaluationMetrics {
	metrics := &evaluation.EvaluationMetrics{
		TaskCompleted:    trace.Status == tracing.SpanStatusOK,
		IterationsUsed:   trace.IterationCount,
		MaxIterations:    e.GetConfigInt("max_iterations", 15),
		TotalTokens:      trace.TotalTokens.TotalTokens,
		PromptTokens:     trace.TotalTokens.PromptTokens,
		CompletionTokens: trace.TotalTokens.CompletionTokens,
		TotalCost:        trace.TotalCost,
		TotalDurationMs:  trace.Duration,
	}

	// Count tool calls
	var successfulTools, failedTools int
	for _, span := range trace.Spans {
		if span.Type == tracing.SpanTypeToolCall {
			if span.Status == tracing.SpanStatusOK {
				successfulTools++
			} else {
				failedTools++
			}
		}
	}
	metrics.ToolCallsCount = successfulTools + failedTools
	metrics.SuccessfulToolCalls = successfulTools
	metrics.FailedToolCalls = failedTools

	// Calculate tokens per iteration
	if metrics.IterationsUsed > 0 {
		metrics.TokensPerIteration = float64(metrics.TotalTokens) / float64(metrics.IterationsUsed)
	}

	// Calculate average iteration duration
	if metrics.IterationsUsed > 0 {
		metrics.AvgIterationDurationMs = float64(metrics.TotalDurationMs) / float64(metrics.IterationsUsed)
	}

	return metrics
}

// calculateScore calculates the overall productivity score
func (e *ProductivityEvaluator) calculateScore(metrics *evaluation.EvaluationMetrics, trace *tracing.Trace) float64 {
	// Weights for different factors
	completionWeight := e.GetConfigFloat("completion_weight", 0.4)
	efficiencyWeight := e.GetConfigFloat("efficiency_weight", 0.3)
	toolSuccessWeight := e.GetConfigFloat("tool_success_weight", 0.3)

	var score float64

	// Completion score (0 or 1)
	var completionScore float64
	if metrics.TaskCompleted {
		completionScore = 1.0
	}
	score += completionScore * completionWeight

	// Efficiency score (based on iterations used vs max)
	var efficiencyScore float64
	if metrics.MaxIterations > 0 && metrics.TaskCompleted {
		// Better score for fewer iterations
		iterationRatio := float64(metrics.IterationsUsed) / float64(metrics.MaxIterations)
		efficiencyScore = 1.0 - iterationRatio
		if efficiencyScore < 0 {
			efficiencyScore = 0
		}
	}
	score += efficiencyScore * efficiencyWeight

	// Tool success score
	var toolSuccessScore float64
	if metrics.ToolCallsCount > 0 {
		toolSuccessScore = float64(metrics.SuccessfulToolCalls) / float64(metrics.ToolCallsCount)
	} else {
		// No tools used - neutral score
		toolSuccessScore = 0.5
	}
	score += toolSuccessScore * toolSuccessWeight

	// Clamp to [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// calculateSubscores calculates individual dimension scores
func (e *ProductivityEvaluator) calculateSubscores(metrics *evaluation.EvaluationMetrics, trace *tracing.Trace) map[string]float64 {
	subscores := make(map[string]float64)

	// Completion score
	if metrics.TaskCompleted {
		subscores["completion"] = 1.0
	} else {
		subscores["completion"] = 0.0
	}

	// Iteration efficiency
	if metrics.MaxIterations > 0 && metrics.TaskCompleted {
		iterationRatio := float64(metrics.IterationsUsed) / float64(metrics.MaxIterations)
		subscores["iteration_efficiency"] = 1.0 - iterationRatio
		if subscores["iteration_efficiency"] < 0 {
			subscores["iteration_efficiency"] = 0
		}
	} else if !metrics.TaskCompleted {
		subscores["iteration_efficiency"] = 0.0
	} else {
		subscores["iteration_efficiency"] = 0.5
	}

	// Tool success rate
	if metrics.ToolCallsCount > 0 {
		subscores["tool_success_rate"] = float64(metrics.SuccessfulToolCalls) / float64(metrics.ToolCallsCount)
	} else {
		subscores["tool_success_rate"] = 1.0 // No tools needed
	}

	// Token efficiency (compared to baseline)
	baselineTokens := e.GetConfigInt("baseline_tokens", 1000)
	if metrics.TotalTokens > 0 && baselineTokens > 0 && metrics.TaskCompleted {
		tokenRatio := float64(metrics.TotalTokens) / float64(baselineTokens)
		if tokenRatio <= 1 {
			subscores["token_efficiency"] = 1.0
		} else if tokenRatio >= 3 {
			subscores["token_efficiency"] = 0.0
		} else {
			subscores["token_efficiency"] = 1.0 - (tokenRatio-1)/2
		}
	} else {
		subscores["token_efficiency"] = 0.5
	}

	return subscores
}

// Ensure ProductivityEvaluator implements Evaluator
var _ evaluation.Evaluator = (*ProductivityEvaluator)(nil)
