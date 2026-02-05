package evaluators

import (
	"context"

	"github.com/Ranganaths/minion/evaluation"
	"github.com/Ranganaths/minion/tracing"
)

const (
	// ToolUsageEvaluatorID is the ID for the tool usage evaluator
	ToolUsageEvaluatorID = "tool_usage"
	// ToolUsageEvaluatorName is the name for the tool usage evaluator
	ToolUsageEvaluatorName = "Tool Usage Evaluator"
)

// ToolUsageEvaluator evaluates how effectively an agent uses its tools
type ToolUsageEvaluator struct {
	*evaluation.BaseEvaluator
}

// NewToolUsageEvaluator creates a new tool usage evaluator
func NewToolUsageEvaluator() *ToolUsageEvaluator {
	return &ToolUsageEvaluator{
		BaseEvaluator: evaluation.NewBaseEvaluator(
			ToolUsageEvaluatorID,
			ToolUsageEvaluatorName,
			evaluation.TypeProductivity, // Tool usage is a productivity dimension
		),
	}
}

// Evaluate evaluates tool usage metrics for a trace
func (e *ToolUsageEvaluator) Evaluate(ctx context.Context, trace *tracing.Trace) (*evaluation.Evaluation, error) {
	metrics := e.extractMetrics(trace)
	subscores := e.calculateSubscores(trace, metrics)
	score := e.calculateScore(subscores)

	eval := e.CreateEvaluation(trace, score)
	eval.Metrics = metrics
	eval.Subscores = subscores

	return eval, nil
}

// EvaluateBatch evaluates multiple traces
func (e *ToolUsageEvaluator) EvaluateBatch(ctx context.Context, traces []*tracing.Trace) ([]*evaluation.Evaluation, error) {
	return e.BaseEvaluator.EvaluateBatch(ctx, traces, e.Evaluate)
}

// extractMetrics extracts tool usage metrics from a trace
func (e *ToolUsageEvaluator) extractMetrics(trace *tracing.Trace) *evaluation.EvaluationMetrics {
	metrics := &evaluation.EvaluationMetrics{
		TaskCompleted:  trace.Status == tracing.SpanStatusOK,
		IterationsUsed: trace.IterationCount,
	}

	// Analyze tool calls
	toolCalls := trace.GetToolCalls()
	toolStats := make(map[string]*toolCallStats)

	for _, span := range toolCalls {
		if span.ToolDetails == nil {
			continue
		}

		toolName := span.ToolDetails.ToolName
		if toolStats[toolName] == nil {
			toolStats[toolName] = &toolCallStats{}
		}

		stats := toolStats[toolName]
		stats.totalCalls++

		if span.Status == tracing.SpanStatusOK {
			stats.successfulCalls++
			metrics.SuccessfulToolCalls++
		} else {
			stats.failedCalls++
			metrics.FailedToolCalls++
		}

		if span.ToolDetails.Retries > 0 {
			stats.retriedCalls++
			metrics.RetryCount += span.ToolDetails.Retries
		}

		if span.ToolDetails.Timeout {
			stats.timeoutCalls++
		}
	}

	metrics.ToolCallsCount = metrics.SuccessfulToolCalls + metrics.FailedToolCalls

	// Calculate error count
	metrics.ErrorCount = metrics.FailedToolCalls

	// Calculate recovery rate (successful after retry)
	if metrics.RetryCount > 0 {
		// If there were retries and the task completed, some recovery happened
		if metrics.TaskCompleted && metrics.FailedToolCalls < metrics.ToolCallsCount {
			metrics.RecoveryRate = float64(metrics.RetryCount-metrics.FailedToolCalls) / float64(metrics.RetryCount)
			if metrics.RecoveryRate < 0 {
				metrics.RecoveryRate = 0
			}
		}
	}

	return metrics
}

type toolCallStats struct {
	totalCalls      int
	successfulCalls int
	failedCalls     int
	retriedCalls    int
	timeoutCalls    int
}

// calculateSubscores calculates individual dimension scores
func (e *ToolUsageEvaluator) calculateSubscores(trace *tracing.Trace, metrics *evaluation.EvaluationMetrics) map[string]float64 {
	subscores := make(map[string]float64)

	// Tool success rate
	if metrics.ToolCallsCount > 0 {
		subscores["tool_success_rate"] = float64(metrics.SuccessfulToolCalls) / float64(metrics.ToolCallsCount)
	} else {
		subscores["tool_success_rate"] = 1.0 // No tools needed
	}

	// Tool efficiency (fewer calls is better)
	subscores["tool_efficiency"] = e.calculateToolEfficiency(trace, metrics)

	// Tool diversity (using appropriate variety of tools)
	subscores["tool_diversity"] = e.calculateToolDiversity(trace)

	// Retry efficiency (successful recovery from failures)
	subscores["retry_efficiency"] = e.calculateRetryEfficiency(metrics)

	// Tool selection (using right tools for the task)
	subscores["tool_selection"] = e.calculateToolSelection(trace, metrics)

	// Parameter quality (estimated from success rate and retries)
	subscores["parameter_quality"] = e.calculateParameterQuality(metrics)

	return subscores
}

// calculateToolEfficiency scores based on number of tool calls per iteration
func (e *ToolUsageEvaluator) calculateToolEfficiency(trace *tracing.Trace, metrics *evaluation.EvaluationMetrics) float64 {
	if metrics.IterationsUsed == 0 || metrics.ToolCallsCount == 0 {
		return 0.5 // Neutral if no tools or iterations
	}

	// Calculate calls per iteration
	callsPerIteration := float64(metrics.ToolCallsCount) / float64(metrics.IterationsUsed)

	// Baseline: 1-2 tool calls per iteration is optimal
	optimalCallsPerIteration := e.GetConfigFloat("optimal_calls_per_iteration", 1.5)
	maxCallsPerIteration := e.GetConfigFloat("max_calls_per_iteration", 5.0)

	if callsPerIteration <= optimalCallsPerIteration {
		return 1.0
	}

	if callsPerIteration >= maxCallsPerIteration {
		return 0.0
	}

	// Linear interpolation
	return 1.0 - (callsPerIteration-optimalCallsPerIteration)/(maxCallsPerIteration-optimalCallsPerIteration)
}

// calculateToolDiversity scores based on variety of tools used
func (e *ToolUsageEvaluator) calculateToolDiversity(trace *tracing.Trace) float64 {
	toolCalls := trace.GetToolCalls()
	if len(toolCalls) == 0 {
		return 0.5 // Neutral if no tools
	}

	// Count unique tools
	uniqueTools := make(map[string]bool)
	for _, span := range toolCalls {
		if span.ToolDetails != nil {
			uniqueTools[span.ToolDetails.ToolName] = true
		}
	}

	totalCalls := len(toolCalls)
	uniqueCount := len(uniqueTools)

	if totalCalls == uniqueCount {
		return 1.0 // Perfect: no redundant tool calls
	}

	// Calculate diversity ratio
	// Higher ratio means more diverse tool usage
	ratio := float64(uniqueCount) / float64(totalCalls)

	// We want some repetition for complex tasks, so don't penalize too much
	// Score > 0.3 ratio as acceptable
	if ratio >= 0.3 {
		return 0.7 + (ratio-0.3)*0.43 // Scale 0.3-1.0 to 0.7-1.0
	}

	return ratio * 2.33 // Scale 0-0.3 to 0-0.7
}

// calculateRetryEfficiency scores based on recovery from failures
func (e *ToolUsageEvaluator) calculateRetryEfficiency(metrics *evaluation.EvaluationMetrics) float64 {
	if metrics.FailedToolCalls == 0 {
		return 1.0 // No failures
	}

	// If task completed despite failures, that's good
	if metrics.TaskCompleted {
		// Penalize based on number of failures
		failureRatio := float64(metrics.FailedToolCalls) / float64(metrics.ToolCallsCount)
		return 1.0 - (failureRatio * 0.5) // Max 50% penalty for failures if completed
	}

	// Task didn't complete
	if metrics.RetryCount > 0 {
		// At least tried to recover
		return 0.3
	}

	return 0.0
}

// calculateToolSelection scores based on appropriate tool usage
func (e *ToolUsageEvaluator) calculateToolSelection(trace *tracing.Trace, metrics *evaluation.EvaluationMetrics) float64 {
	// This is a heuristic based on:
	// 1. Task completion with tool usage
	// 2. No unnecessary tool calls (approximated by low failure rate)

	if !metrics.TaskCompleted {
		return 0.3 // Didn't complete, tool selection might be wrong
	}

	// Check for potential issues
	toolCalls := trace.GetToolCalls()
	if len(toolCalls) == 0 {
		// Task completed without tools - could be good or bad depending on task
		// Treat as neutral
		return 0.6
	}

	// Base score on success rate
	successRate := float64(metrics.SuccessfulToolCalls) / float64(metrics.ToolCallsCount)

	// Bonus for completing without failures
	if metrics.FailedToolCalls == 0 {
		return successRate + 0.1
	}

	// Small penalty for failures
	return successRate * 0.9
}

// calculateParameterQuality scores based on tool call success patterns
func (e *ToolUsageEvaluator) calculateParameterQuality(metrics *evaluation.EvaluationMetrics) float64 {
	if metrics.ToolCallsCount == 0 {
		return 0.5 // Neutral
	}

	// High success rate with low retries indicates good parameters
	successRate := float64(metrics.SuccessfulToolCalls) / float64(metrics.ToolCallsCount)

	// Penalize for retries (suggests parameter issues)
	retryPenalty := 0.0
	if metrics.RetryCount > 0 {
		retryRatio := float64(metrics.RetryCount) / float64(metrics.ToolCallsCount)
		retryPenalty = retryRatio * 0.3 // Up to 30% penalty for retries
	}

	score := successRate - retryPenalty
	if score < 0 {
		score = 0
	}
	return score
}

// calculateScore calculates the overall tool usage score
func (e *ToolUsageEvaluator) calculateScore(subscores map[string]float64) float64 {
	weights := map[string]float64{
		"tool_success_rate":  0.30,
		"tool_efficiency":    0.20,
		"tool_diversity":     0.10,
		"retry_efficiency":   0.15,
		"tool_selection":     0.15,
		"parameter_quality":  0.10,
	}

	var score float64
	var totalWeight float64

	for key, weight := range weights {
		if value, ok := subscores[key]; ok {
			score += value * weight
			totalWeight += weight
		}
	}

	if totalWeight > 0 {
		score = score / totalWeight
	}

	// Clamp to [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// Ensure ToolUsageEvaluator implements Evaluator
var _ evaluation.Evaluator = (*ToolUsageEvaluator)(nil)
