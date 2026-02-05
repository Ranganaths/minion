package evaluators

import (
	"context"

	"github.com/Ranganaths/minion/evaluation"
	"github.com/Ranganaths/minion/tracing"
)

const (
	// ErrorEvaluatorID is the ID for the error evaluator
	ErrorEvaluatorID = "error"
	// ErrorEvaluatorName is the name for the error evaluator
	ErrorEvaluatorName = "Error Evaluator"
)

// ErrorEvaluator evaluates error rate and recovery metrics
type ErrorEvaluator struct {
	*evaluation.BaseEvaluator
}

// NewErrorEvaluator creates a new error evaluator
func NewErrorEvaluator() *ErrorEvaluator {
	return &ErrorEvaluator{
		BaseEvaluator: evaluation.NewBaseEvaluator(
			ErrorEvaluatorID,
			ErrorEvaluatorName,
			evaluation.TypeError,
		),
	}
}

// Evaluate evaluates error metrics for a trace
func (e *ErrorEvaluator) Evaluate(ctx context.Context, trace *tracing.Trace) (*evaluation.Evaluation, error) {
	metrics := e.extractMetrics(trace)
	score := e.calculateScore(metrics, trace)

	eval := e.CreateEvaluation(trace, score)
	eval.Metrics = metrics
	eval.Subscores = e.calculateSubscores(metrics, trace)

	return eval, nil
}

// EvaluateBatch evaluates multiple traces
func (e *ErrorEvaluator) EvaluateBatch(ctx context.Context, traces []*tracing.Trace) ([]*evaluation.Evaluation, error) {
	return e.BaseEvaluator.EvaluateBatch(ctx, traces, e.Evaluate)
}

// extractMetrics extracts error metrics from a trace
func (e *ErrorEvaluator) extractMetrics(trace *tracing.Trace) *evaluation.EvaluationMetrics {
	metrics := &evaluation.EvaluationMetrics{
		TaskCompleted:   trace.Status == tracing.SpanStatusOK,
		IterationsUsed:  trace.IterationCount,
		TotalTokens:     trace.TotalTokens.TotalTokens,
		TotalCost:       trace.TotalCost,
		TotalDurationMs: trace.Duration,
	}

	// Count errors and analyze recovery
	var errorCount, retryCount int
	var toolErrors, llmErrors int
	var recoveredErrors int
	var prevSpanWasError bool

	for _, span := range trace.Spans {
		if span.Status == tracing.SpanStatusError {
			errorCount++
			prevSpanWasError = true

			// Categorize error type
			if span.Type == tracing.SpanTypeToolCall {
				toolErrors++
			} else if span.Type == tracing.SpanTypeLLMCall {
				llmErrors++
			}
		} else if prevSpanWasError {
			// This span succeeded after an error - potential recovery
			recoveredErrors++
			prevSpanWasError = false
		}

		// Count retries (detected by same tool being called multiple times)
		if span.ToolDetails != nil && span.ToolDetails.Retries > 0 {
			retryCount += span.ToolDetails.Retries
		}
	}

	metrics.ErrorCount = errorCount
	metrics.RetryCount = retryCount

	// Calculate recovery rate
	if errorCount > 0 {
		metrics.RecoveryRate = float64(recoveredErrors) / float64(errorCount)
	} else {
		metrics.RecoveryRate = 1.0 // No errors = perfect recovery
	}

	// Count tool calls
	for _, span := range trace.Spans {
		if span.Type == tracing.SpanTypeToolCall {
			if span.Status == tracing.SpanStatusOK {
				metrics.SuccessfulToolCalls++
			} else {
				metrics.FailedToolCalls++
			}
		}
	}
	metrics.ToolCallsCount = metrics.SuccessfulToolCalls + metrics.FailedToolCalls

	return metrics
}

// calculateScore calculates the overall error resilience score
func (e *ErrorEvaluator) calculateScore(metrics *evaluation.EvaluationMetrics, trace *tracing.Trace) float64 {
	// Weights
	errorFreeWeight := e.GetConfigFloat("error_free_weight", 0.4)
	recoveryWeight := e.GetConfigFloat("recovery_weight", 0.3)
	completionWeight := e.GetConfigFloat("completion_weight", 0.3)

	var score float64

	// Error-free score
	maxAcceptableErrors := e.GetConfigInt("max_acceptable_errors", 3)
	if metrics.ErrorCount == 0 {
		score += 1.0 * errorFreeWeight
	} else if metrics.ErrorCount <= maxAcceptableErrors {
		errorRatio := float64(metrics.ErrorCount) / float64(maxAcceptableErrors)
		score += (1.0 - errorRatio) * errorFreeWeight
	}
	// else: 0 contribution from error-free

	// Recovery score
	score += metrics.RecoveryRate * recoveryWeight

	// Completion despite errors score
	if metrics.TaskCompleted {
		score += 1.0 * completionWeight
	} else if metrics.ErrorCount > 0 {
		// Partial credit for partial completion with errors
		score += 0.3 * completionWeight
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

// calculateSubscores calculates individual dimension scores
func (e *ErrorEvaluator) calculateSubscores(metrics *evaluation.EvaluationMetrics, trace *tracing.Trace) map[string]float64 {
	subscores := make(map[string]float64)

	// Error-free score
	if metrics.ErrorCount == 0 {
		subscores["error_free"] = 1.0
	} else {
		maxErrors := e.GetConfigInt("max_acceptable_errors", 3)
		if metrics.ErrorCount >= maxErrors {
			subscores["error_free"] = 0.0
		} else {
			subscores["error_free"] = 1.0 - float64(metrics.ErrorCount)/float64(maxErrors)
		}
	}

	// Recovery rate
	subscores["recovery_rate"] = metrics.RecoveryRate

	// Completion despite errors
	if metrics.TaskCompleted {
		subscores["resilience"] = 1.0
	} else if metrics.ErrorCount == 0 {
		subscores["resilience"] = 0.5 // Failed without errors - different issue
	} else {
		subscores["resilience"] = 0.0
	}

	// Tool reliability
	if metrics.ToolCallsCount > 0 {
		subscores["tool_reliability"] = float64(metrics.SuccessfulToolCalls) / float64(metrics.ToolCallsCount)
	} else {
		subscores["tool_reliability"] = 1.0 // No tools = no tool errors
	}

	// Retry efficiency (lower retries is better if task completed)
	maxRetries := e.GetConfigInt("max_retries", 5)
	if metrics.RetryCount == 0 {
		subscores["retry_efficiency"] = 1.0
	} else if metrics.RetryCount >= maxRetries {
		subscores["retry_efficiency"] = 0.0
	} else {
		subscores["retry_efficiency"] = 1.0 - float64(metrics.RetryCount)/float64(maxRetries)
	}

	// Graceful degradation (completed despite errors)
	if metrics.ErrorCount > 0 && metrics.TaskCompleted {
		subscores["graceful_degradation"] = 1.0
	} else if metrics.ErrorCount > 0 && !metrics.TaskCompleted {
		subscores["graceful_degradation"] = 0.0
	} else {
		subscores["graceful_degradation"] = 0.5 // No errors to degrade from
	}

	return subscores
}

// Ensure ErrorEvaluator implements Evaluator
var _ evaluation.Evaluator = (*ErrorEvaluator)(nil)
