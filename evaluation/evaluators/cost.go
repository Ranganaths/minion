package evaluators

import (
	"context"

	"github.com/Ranganaths/minion/evaluation"
	"github.com/Ranganaths/minion/tracing"
)

const (
	// CostEvaluatorID is the ID for the cost evaluator
	CostEvaluatorID = "cost"
	// CostEvaluatorName is the name for the cost evaluator
	CostEvaluatorName = "Cost Evaluator"
)

// CostEvaluator evaluates cost efficiency metrics
type CostEvaluator struct {
	*evaluation.BaseEvaluator
}

// NewCostEvaluator creates a new cost evaluator
func NewCostEvaluator() *CostEvaluator {
	return &CostEvaluator{
		BaseEvaluator: evaluation.NewBaseEvaluator(
			CostEvaluatorID,
			CostEvaluatorName,
			evaluation.TypeCost,
		),
	}
}

// Evaluate evaluates cost metrics for a trace
func (e *CostEvaluator) Evaluate(ctx context.Context, trace *tracing.Trace) (*evaluation.Evaluation, error) {
	metrics := e.extractMetrics(trace)
	score := e.calculateScore(metrics, trace)

	eval := e.CreateEvaluation(trace, score)
	eval.Metrics = metrics
	eval.Subscores = e.calculateSubscores(metrics, trace)

	return eval, nil
}

// EvaluateBatch evaluates multiple traces
func (e *CostEvaluator) EvaluateBatch(ctx context.Context, traces []*tracing.Trace) ([]*evaluation.Evaluation, error) {
	return e.BaseEvaluator.EvaluateBatch(ctx, traces, e.Evaluate)
}

// extractMetrics extracts cost metrics from a trace
func (e *CostEvaluator) extractMetrics(trace *tracing.Trace) *evaluation.EvaluationMetrics {
	metrics := &evaluation.EvaluationMetrics{
		TaskCompleted:    trace.Status == tracing.SpanStatusOK,
		TotalCost:        trace.TotalCost,
		TotalTokens:      trace.TotalTokens.TotalTokens,
		PromptTokens:     trace.TotalTokens.PromptTokens,
		CompletionTokens: trace.TotalTokens.CompletionTokens,
		IterationsUsed:   trace.IterationCount,
		TotalDurationMs:  trace.Duration,
	}

	// Calculate cost per token
	if metrics.TotalTokens > 0 {
		metrics.CostPerToken = metrics.TotalCost / float64(metrics.TotalTokens)
	}

	// Count LLM calls
	for _, span := range trace.Spans {
		if span.Type == tracing.SpanTypeLLMCall {
			metrics.LLMCallCount++
		}
	}

	return metrics
}

// calculateScore calculates the overall cost efficiency score
func (e *CostEvaluator) calculateScore(metrics *evaluation.EvaluationMetrics, trace *tracing.Trace) float64 {
	// Get budget thresholds from config
	budgetLimit := e.GetConfigFloat("budget_limit", 0.10)   // $0.10 default
	optimalBudget := e.GetConfigFloat("optimal_budget", 0.02) // $0.02 default

	if !metrics.TaskCompleted {
		// Failed task - penalize but consider cost
		if metrics.TotalCost <= optimalBudget {
			return 0.3 // Low cost failure
		}
		return 0.1 // High cost failure
	}

	// Task completed - score based on cost
	if metrics.TotalCost <= 0 {
		return 1.0 // Free is best
	}

	if metrics.TotalCost <= optimalBudget {
		return 1.0 // Within optimal budget
	}

	if metrics.TotalCost >= budgetLimit {
		return 0.3 // Over budget
	}

	// Linear interpolation between optimal and limit
	costRange := budgetLimit - optimalBudget
	costOver := metrics.TotalCost - optimalBudget
	score := 1.0 - (costOver/costRange)*0.7 // Scale to [0.3, 1.0]

	return score
}

// calculateSubscores calculates individual dimension scores
func (e *CostEvaluator) calculateSubscores(metrics *evaluation.EvaluationMetrics, trace *tracing.Trace) map[string]float64 {
	subscores := make(map[string]float64)

	budgetLimit := e.GetConfigFloat("budget_limit", 0.10)
	optimalBudget := e.GetConfigFloat("optimal_budget", 0.02)

	// Absolute cost score
	if metrics.TotalCost <= 0 {
		subscores["absolute_cost"] = 1.0
	} else if metrics.TotalCost <= optimalBudget {
		subscores["absolute_cost"] = 1.0
	} else if metrics.TotalCost >= budgetLimit {
		subscores["absolute_cost"] = 0.0
	} else {
		subscores["absolute_cost"] = 1.0 - (metrics.TotalCost-optimalBudget)/(budgetLimit-optimalBudget)
	}

	// Cost per token score
	maxCostPerToken := e.GetConfigFloat("max_cost_per_token", 0.0001) // $0.0001 per token
	if metrics.CostPerToken <= 0 {
		subscores["cost_per_token"] = 1.0
	} else if metrics.CostPerToken >= maxCostPerToken {
		subscores["cost_per_token"] = 0.0
	} else {
		subscores["cost_per_token"] = 1.0 - metrics.CostPerToken/maxCostPerToken
	}

	// Cost efficiency (cost vs completion)
	if metrics.TaskCompleted && metrics.TotalCost <= optimalBudget {
		subscores["cost_efficiency"] = 1.0
	} else if metrics.TaskCompleted {
		// Completed but over optimal
		subscores["cost_efficiency"] = 0.7
	} else if metrics.TotalCost <= optimalBudget {
		// Failed but cheap
		subscores["cost_efficiency"] = 0.3
	} else {
		// Failed and expensive
		subscores["cost_efficiency"] = 0.0
	}

	// Token distribution score (prompt vs completion ratio)
	// Ideal ratio depends on use case, but generally balanced is good
	if metrics.TotalTokens > 0 {
		promptRatio := float64(metrics.PromptTokens) / float64(metrics.TotalTokens)
		// Penalize extreme ratios (too much prompt or too much completion)
		if promptRatio >= 0.3 && promptRatio <= 0.7 {
			subscores["token_distribution"] = 1.0
		} else if promptRatio < 0.1 || promptRatio > 0.9 {
			subscores["token_distribution"] = 0.5
		} else {
			subscores["token_distribution"] = 0.75
		}
	} else {
		subscores["token_distribution"] = 0.5
	}

	return subscores
}

// Ensure CostEvaluator implements Evaluator
var _ evaluation.Evaluator = (*CostEvaluator)(nil)
