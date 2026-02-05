package evaluators

import (
	"context"
	"fmt"
	"sync"

	"github.com/Ranganaths/minion/evaluation"
	"github.com/Ranganaths/minion/llm"
	"github.com/Ranganaths/minion/tracing"
)

const (
	// CompositeEvaluatorID is the ID for the composite evaluator
	CompositeEvaluatorID = "composite"
	// CompositeEvaluatorName is the name for the composite evaluator
	CompositeEvaluatorName = "Composite Evaluator"
)

// WeightedEvaluator represents an evaluator with an associated weight
type WeightedEvaluator struct {
	Evaluator evaluation.Evaluator
	Weight    float64
}

// CompositeEvaluator combines multiple evaluators with configurable weights
type CompositeEvaluator struct {
	*evaluation.BaseEvaluator
	evaluators []WeightedEvaluator
	parallel   bool
}

// CompositeEvaluatorConfig holds configuration for the composite evaluator
type CompositeEvaluatorConfig struct {
	// ID is the unique identifier (optional, defaults to "composite")
	ID string
	// Name is the human-readable name (optional)
	Name string
	// Evaluators are the weighted evaluators to combine
	Evaluators []WeightedEvaluator
	// Parallel enables parallel evaluation
	Parallel bool
}

// NewCompositeEvaluator creates a new composite evaluator with the given weighted evaluators
func NewCompositeEvaluator(evaluators ...WeightedEvaluator) *CompositeEvaluator {
	return NewCompositeEvaluatorWithConfig(CompositeEvaluatorConfig{
		Evaluators: evaluators,
	})
}

// NewCompositeEvaluatorWithConfig creates a new composite evaluator with configuration
func NewCompositeEvaluatorWithConfig(config CompositeEvaluatorConfig) *CompositeEvaluator {
	id := config.ID
	if id == "" {
		id = CompositeEvaluatorID
	}
	name := config.Name
	if name == "" {
		name = CompositeEvaluatorName
	}

	return &CompositeEvaluator{
		BaseEvaluator: evaluation.NewBaseEvaluator(id, name, evaluation.TypeComposite),
		evaluators:    config.Evaluators,
		parallel:      config.Parallel,
	}
}

// AddEvaluator adds a weighted evaluator
func (e *CompositeEvaluator) AddEvaluator(evaluator evaluation.Evaluator, weight float64) *CompositeEvaluator {
	e.evaluators = append(e.evaluators, WeightedEvaluator{
		Evaluator: evaluator,
		Weight:    weight,
	})
	return e
}

// SetParallel enables or disables parallel evaluation
func (e *CompositeEvaluator) SetParallel(parallel bool) *CompositeEvaluator {
	e.parallel = parallel
	return e
}

// Evaluate runs all evaluators and combines their results
func (e *CompositeEvaluator) Evaluate(ctx context.Context, trace *tracing.Trace) (*evaluation.Evaluation, error) {
	if len(e.evaluators) == 0 {
		return nil, fmt.Errorf("no evaluators configured")
	}

	// Normalize weights
	totalWeight := e.normalizeWeights()
	if totalWeight == 0 {
		return nil, fmt.Errorf("total weight is zero")
	}

	// Run evaluators
	var results []*evaluatorResult
	var err error

	if e.parallel {
		results, err = e.evaluateParallel(ctx, trace)
	} else {
		results, err = e.evaluateSequential(ctx, trace)
	}

	if err != nil {
		return nil, err
	}

	// Combine results
	return e.combineResults(trace, results, totalWeight)
}

// EvaluateBatch evaluates multiple traces
func (e *CompositeEvaluator) EvaluateBatch(ctx context.Context, traces []*tracing.Trace) ([]*evaluation.Evaluation, error) {
	return e.BaseEvaluator.EvaluateBatch(ctx, traces, e.Evaluate)
}

type evaluatorResult struct {
	evaluator  WeightedEvaluator
	evaluation *evaluation.Evaluation
	err        error
}

// evaluateSequential runs evaluators one at a time
func (e *CompositeEvaluator) evaluateSequential(ctx context.Context, trace *tracing.Trace) ([]*evaluatorResult, error) {
	results := make([]*evaluatorResult, 0, len(e.evaluators))

	for _, weighted := range e.evaluators {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		eval, err := weighted.Evaluator.Evaluate(ctx, trace)
		results = append(results, &evaluatorResult{
			evaluator:  weighted,
			evaluation: eval,
			err:        err,
		})
	}

	return results, nil
}

// evaluateParallel runs evaluators concurrently
func (e *CompositeEvaluator) evaluateParallel(ctx context.Context, trace *tracing.Trace) ([]*evaluatorResult, error) {
	results := make([]*evaluatorResult, len(e.evaluators))
	var wg sync.WaitGroup

	for i, weighted := range e.evaluators {
		wg.Add(1)
		go func(idx int, w WeightedEvaluator) {
			defer wg.Done()

			eval, err := w.Evaluator.Evaluate(ctx, trace)
			results[idx] = &evaluatorResult{
				evaluator:  w,
				evaluation: eval,
				err:        err,
			}
		}(i, weighted)
	}

	wg.Wait()

	// Check for context cancellation
	if ctx.Err() != nil {
		return results, ctx.Err()
	}

	return results, nil
}

// combineResults combines individual evaluation results into a composite evaluation
func (e *CompositeEvaluator) combineResults(trace *tracing.Trace, results []*evaluatorResult, totalWeight float64) (*evaluation.Evaluation, error) {
	var (
		weightedScore float64
		successCount  int
		combinedSubscores = make(map[string]float64)
		combinedMetrics   = &evaluation.EvaluationMetrics{}
		combinedQuality   *evaluation.QualityAssessment
		errors            []string
	)

	for _, result := range results {
		if result.err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", result.evaluator.Evaluator.ID(), result.err))
			continue
		}
		if result.evaluation == nil {
			continue
		}

		eval := result.evaluation
		weight := result.evaluator.Weight

		// Accumulate weighted score
		weightedScore += eval.Score * weight
		successCount++

		// Merge subscores with evaluator prefix
		prefix := string(eval.Type) + "_"
		for key, value := range eval.Subscores {
			combinedSubscores[prefix+key] = value
		}

		// Aggregate metrics
		if eval.Metrics != nil {
			e.mergeMetrics(combinedMetrics, eval.Metrics)
		}

		// Use quality assessment if available
		if eval.QualityAssessment != nil && combinedQuality == nil {
			combinedQuality = eval.QualityAssessment
		}
	}

	if successCount == 0 {
		if len(errors) > 0 {
			return nil, fmt.Errorf("all evaluators failed: %v", errors)
		}
		return nil, fmt.Errorf("all evaluators failed")
	}

	// Calculate final score
	finalScore := weightedScore / totalWeight

	// Create composite evaluation
	eval := e.CreateEvaluation(trace, finalScore)
	eval.Subscores = combinedSubscores
	eval.Metrics = combinedMetrics
	eval.QualityAssessment = combinedQuality

	// Store component evaluations in metadata
	componentScores := make(map[string]float64)
	for _, result := range results {
		if result.evaluation != nil {
			componentScores[result.evaluator.Evaluator.ID()] = result.evaluation.Score
		}
	}
	eval.Metadata = map[string]interface{}{
		"component_scores":  componentScores,
		"evaluators_run":    successCount,
		"evaluators_failed": len(errors),
	}

	if len(errors) > 0 {
		eval.Metadata["errors"] = errors
	}

	return eval, nil
}

// mergeMetrics merges source metrics into destination
func (e *CompositeEvaluator) mergeMetrics(dst, src *evaluation.EvaluationMetrics) {
	// Task completion - use OR (any completion counts)
	if src.TaskCompleted {
		dst.TaskCompleted = true
	}

	// Iterations - take max
	if src.IterationsUsed > dst.IterationsUsed {
		dst.IterationsUsed = src.IterationsUsed
	}
	if src.MaxIterations > dst.MaxIterations {
		dst.MaxIterations = src.MaxIterations
	}

	// Tool calls - sum
	dst.ToolCallsCount += src.ToolCallsCount
	dst.SuccessfulToolCalls += src.SuccessfulToolCalls
	dst.FailedToolCalls += src.FailedToolCalls

	// Tokens - take max (they should be the same for same trace)
	if src.TotalTokens > dst.TotalTokens {
		dst.TotalTokens = src.TotalTokens
		dst.PromptTokens = src.PromptTokens
		dst.CompletionTokens = src.CompletionTokens
		dst.TokensPerIteration = src.TokensPerIteration
	}

	// Cost - take max
	if src.TotalCost > dst.TotalCost {
		dst.TotalCost = src.TotalCost
		dst.CostPerToken = src.CostPerToken
	}

	// Latency - take max
	if src.TotalDurationMs > dst.TotalDurationMs {
		dst.TotalDurationMs = src.TotalDurationMs
		dst.AvgIterationDurationMs = src.AvgIterationDurationMs
		dst.FirstTokenLatencyMs = src.FirstTokenLatencyMs
		dst.LLMCallCount = src.LLMCallCount
		dst.AvgLLMLatencyMs = src.AvgLLMLatencyMs
	}

	// Errors - sum
	dst.ErrorCount += src.ErrorCount
	dst.RetryCount += src.RetryCount

	// Recovery rate - average
	if src.RecoveryRate > 0 {
		if dst.RecoveryRate > 0 {
			dst.RecoveryRate = (dst.RecoveryRate + src.RecoveryRate) / 2
		} else {
			dst.RecoveryRate = src.RecoveryRate
		}
	}
}

// normalizeWeights calculates the total weight for normalization
func (e *CompositeEvaluator) normalizeWeights() float64 {
	var total float64
	for _, w := range e.evaluators {
		total += w.Weight
	}
	return total
}

// DefaultCompositeEvaluator creates a composite evaluator with all standard evaluators
func DefaultCompositeEvaluator() *CompositeEvaluator {
	return NewCompositeEvaluator(
		WeightedEvaluator{Evaluator: NewProductivityEvaluator(), Weight: 0.30},
		WeightedEvaluator{Evaluator: NewLatencyEvaluator(), Weight: 0.20},
		WeightedEvaluator{Evaluator: NewCostEvaluator(), Weight: 0.20},
		WeightedEvaluator{Evaluator: NewErrorEvaluator(), Weight: 0.30},
	)
}

// DefaultCompositeEvaluatorWithQuality creates a composite evaluator including quality evaluation
func DefaultCompositeEvaluatorWithQuality(provider llm.Provider, judgeModel string) *CompositeEvaluator {
	return NewCompositeEvaluator(
		WeightedEvaluator{Evaluator: NewProductivityEvaluator(), Weight: 0.25},
		WeightedEvaluator{Evaluator: NewLatencyEvaluator(), Weight: 0.15},
		WeightedEvaluator{Evaluator: NewCostEvaluator(), Weight: 0.15},
		WeightedEvaluator{Evaluator: NewErrorEvaluator(), Weight: 0.20},
		WeightedEvaluator{Evaluator: NewQualityEvaluator(provider, judgeModel), Weight: 0.25},
	)
}

// Ensure CompositeEvaluator implements Evaluator
var _ evaluation.Evaluator = (*CompositeEvaluator)(nil)
