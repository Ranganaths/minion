package evaluators

import (
	"context"

	"github.com/Ranganaths/minion/evaluation"
	"github.com/Ranganaths/minion/tracing"
)

const (
	// LatencyEvaluatorID is the ID for the latency evaluator
	LatencyEvaluatorID = "latency"
	// LatencyEvaluatorName is the name for the latency evaluator
	LatencyEvaluatorName = "Latency Evaluator"
)

// LatencyEvaluator evaluates response time and throughput metrics
type LatencyEvaluator struct {
	*evaluation.BaseEvaluator
}

// NewLatencyEvaluator creates a new latency evaluator
func NewLatencyEvaluator() *LatencyEvaluator {
	return &LatencyEvaluator{
		BaseEvaluator: evaluation.NewBaseEvaluator(
			LatencyEvaluatorID,
			LatencyEvaluatorName,
			evaluation.TypeLatency,
		),
	}
}

// Evaluate evaluates latency metrics for a trace
func (e *LatencyEvaluator) Evaluate(ctx context.Context, trace *tracing.Trace) (*evaluation.Evaluation, error) {
	metrics := e.extractMetrics(trace)
	score := e.calculateScore(metrics, trace)

	eval := e.CreateEvaluation(trace, score)
	eval.Metrics = metrics
	eval.Subscores = e.calculateSubscores(metrics, trace)

	return eval, nil
}

// EvaluateBatch evaluates multiple traces
func (e *LatencyEvaluator) EvaluateBatch(ctx context.Context, traces []*tracing.Trace) ([]*evaluation.Evaluation, error) {
	return e.BaseEvaluator.EvaluateBatch(ctx, traces, e.Evaluate)
}

// extractMetrics extracts latency metrics from a trace
func (e *LatencyEvaluator) extractMetrics(trace *tracing.Trace) *evaluation.EvaluationMetrics {
	metrics := &evaluation.EvaluationMetrics{
		TaskCompleted:   trace.Status == tracing.SpanStatusOK,
		TotalDurationMs: trace.Duration,
		IterationsUsed:  trace.IterationCount,
		TotalTokens:     trace.TotalTokens.TotalTokens,
		TotalCost:       trace.TotalCost,
	}

	// Calculate average iteration duration
	if metrics.IterationsUsed > 0 {
		metrics.AvgIterationDurationMs = float64(metrics.TotalDurationMs) / float64(metrics.IterationsUsed)
	}

	// Extract LLM call metrics
	var llmTotalDuration int64
	var firstLLMFound bool
	for _, span := range trace.Spans {
		if span.Type == tracing.SpanTypeLLMCall {
			metrics.LLMCallCount++
			llmTotalDuration += span.Duration

			// First LLM call latency (approximation of first token latency)
			if !firstLLMFound {
				metrics.FirstTokenLatencyMs = span.Duration
				firstLLMFound = true
			}
		}
	}

	// Calculate average LLM latency
	if metrics.LLMCallCount > 0 {
		metrics.AvgLLMLatencyMs = float64(llmTotalDuration) / float64(metrics.LLMCallCount)
	}

	return metrics
}

// calculateScore calculates the overall latency score
func (e *LatencyEvaluator) calculateScore(metrics *evaluation.EvaluationMetrics, trace *tracing.Trace) float64 {
	// Get latency thresholds from config
	targetLatencyMs := e.GetConfigFloat("target_latency_ms", 5000)    // 5 seconds target
	maxLatencyMs := e.GetConfigFloat("max_latency_ms", 30000)         // 30 seconds max
	targetFirstTokenMs := e.GetConfigFloat("target_first_token_ms", 1000) // 1 second

	// Weights
	totalDurationWeight := e.GetConfigFloat("total_duration_weight", 0.5)
	firstTokenWeight := e.GetConfigFloat("first_token_weight", 0.3)
	consistencyWeight := e.GetConfigFloat("consistency_weight", 0.2)

	var score float64

	// Total duration score
	durationScore := e.calculateDurationScore(float64(metrics.TotalDurationMs), targetLatencyMs, maxLatencyMs)
	score += durationScore * totalDurationWeight

	// First token latency score
	firstTokenScore := e.calculateDurationScore(float64(metrics.FirstTokenLatencyMs), targetFirstTokenMs, targetFirstTokenMs*5)
	score += firstTokenScore * firstTokenWeight

	// Consistency score (based on iteration variance - lower is better)
	var consistencyScore float64
	if metrics.IterationsUsed > 1 && metrics.AvgIterationDurationMs > 0 {
		// Simple consistency metric: how close is avg to expected?
		expectedIterDuration := targetLatencyMs / float64(metrics.IterationsUsed)
		if metrics.AvgIterationDurationMs <= expectedIterDuration {
			consistencyScore = 1.0
		} else {
			ratio := expectedIterDuration / metrics.AvgIterationDurationMs
			consistencyScore = ratio
			if consistencyScore < 0 {
				consistencyScore = 0
			}
		}
	} else {
		consistencyScore = 0.5 // Neutral for single iteration
	}
	score += consistencyScore * consistencyWeight

	// Penalty for not completing
	if !metrics.TaskCompleted {
		score *= 0.5
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

// calculateDurationScore calculates a score for a duration value
func (e *LatencyEvaluator) calculateDurationScore(duration, target, max float64) float64 {
	if duration <= 0 {
		return 1.0 // Instant is best
	}
	if duration <= target {
		return 1.0 // Within target
	}
	if duration >= max {
		return 0.0 // Over max
	}
	// Linear interpolation
	return 1.0 - (duration-target)/(max-target)
}

// calculateSubscores calculates individual dimension scores
func (e *LatencyEvaluator) calculateSubscores(metrics *evaluation.EvaluationMetrics, trace *tracing.Trace) map[string]float64 {
	subscores := make(map[string]float64)

	targetLatencyMs := e.GetConfigFloat("target_latency_ms", 5000)
	maxLatencyMs := e.GetConfigFloat("max_latency_ms", 30000)
	targetFirstTokenMs := e.GetConfigFloat("target_first_token_ms", 1000)
	targetLLMLatencyMs := e.GetConfigFloat("target_llm_latency_ms", 2000)

	// Total duration score
	subscores["total_duration"] = e.calculateDurationScore(
		float64(metrics.TotalDurationMs),
		targetLatencyMs,
		maxLatencyMs,
	)

	// First token latency score
	subscores["first_token_latency"] = e.calculateDurationScore(
		float64(metrics.FirstTokenLatencyMs),
		targetFirstTokenMs,
		targetFirstTokenMs*5,
	)

	// Average LLM latency score
	subscores["avg_llm_latency"] = e.calculateDurationScore(
		metrics.AvgLLMLatencyMs,
		targetLLMLatencyMs,
		targetLLMLatencyMs*3,
	)

	// Average iteration duration score
	targetIterDuration := targetLatencyMs / float64(max(metrics.IterationsUsed, 1))
	subscores["avg_iteration_duration"] = e.calculateDurationScore(
		metrics.AvgIterationDurationMs,
		targetIterDuration,
		targetIterDuration*3,
	)

	// Throughput score (tokens per second)
	if metrics.TotalDurationMs > 0 {
		tokensPerSecond := float64(metrics.TotalTokens) / (float64(metrics.TotalDurationMs) / 1000.0)
		targetThroughput := e.GetConfigFloat("target_throughput", 50.0) // 50 tokens/sec
		if tokensPerSecond >= targetThroughput {
			subscores["throughput"] = 1.0
		} else {
			subscores["throughput"] = tokensPerSecond / targetThroughput
		}
	} else {
		subscores["throughput"] = 0.5
	}

	return subscores
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Ensure LatencyEvaluator implements Evaluator
var _ evaluation.Evaluator = (*LatencyEvaluator)(nil)
