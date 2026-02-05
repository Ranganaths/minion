// Package evaluators provides built-in evaluator implementations for measuring
// agent productivity across multiple dimensions.
//
// Available evaluators:
//   - ProductivityEvaluator: Measures task completion and efficiency
//   - LatencyEvaluator: Measures response time and throughput
//   - CostEvaluator: Measures cost efficiency and token usage
//   - ErrorEvaluator: Measures error rate and recovery
//   - QualityEvaluator: LLM-as-Judge quality assessment
//   - ToolUsageEvaluator: Measures tool selection and efficiency
//   - CompositeEvaluator: Combines multiple evaluators with weights
//
// Example usage:
//
//	// Create individual evaluators
//	productivity := evaluators.NewProductivityEvaluator()
//	latency := evaluators.NewLatencyEvaluator()
//
//	// Create composite evaluator
//	composite := evaluators.NewCompositeEvaluator(
//	    evaluators.WeightedEvaluator{Evaluator: productivity, Weight: 0.5},
//	    evaluators.WeightedEvaluator{Evaluator: latency, Weight: 0.5},
//	)
//
//	// Evaluate a trace
//	eval, err := composite.Evaluate(ctx, trace)
package evaluators

import (
	"github.com/Ranganaths/minion/evaluation"
	"github.com/Ranganaths/minion/llm"
)

// RegisterAll registers all built-in evaluators with the default registry
func RegisterAll() {
	evaluation.MustRegister(NewProductivityEvaluator())
	evaluation.MustRegister(NewLatencyEvaluator())
	evaluation.MustRegister(NewCostEvaluator())
	evaluation.MustRegister(NewErrorEvaluator())
	evaluation.MustRegister(NewToolUsageEvaluator())
}

// RegisterWithQuality registers all evaluators including quality evaluator
func RegisterWithQuality(provider llm.Provider, judgeModel string) {
	RegisterAll()
	evaluation.MustRegister(NewQualityEvaluator(provider, judgeModel))
}

// DefaultEvaluators returns a slice of all standard evaluators
func DefaultEvaluators() []evaluation.Evaluator {
	return []evaluation.Evaluator{
		NewProductivityEvaluator(),
		NewLatencyEvaluator(),
		NewCostEvaluator(),
		NewErrorEvaluator(),
		NewToolUsageEvaluator(),
	}
}

// DefaultEvaluatorsWithQuality returns all evaluators including quality
func DefaultEvaluatorsWithQuality(provider llm.Provider, judgeModel string) []evaluation.Evaluator {
	return []evaluation.Evaluator{
		NewProductivityEvaluator(),
		NewLatencyEvaluator(),
		NewCostEvaluator(),
		NewErrorEvaluator(),
		NewToolUsageEvaluator(),
		NewQualityEvaluator(provider, judgeModel),
	}
}

// StandardWeights returns the recommended weights for combining evaluators
func StandardWeights() map[string]float64 {
	return map[string]float64{
		ProductivityEvaluatorID: 0.30,
		LatencyEvaluatorID:      0.15,
		CostEvaluatorID:         0.15,
		ErrorEvaluatorID:        0.20,
		ToolUsageEvaluatorID:    0.10,
		QualityEvaluatorID:      0.10,
	}
}

// StandardWeightsWithQuality returns weights when quality evaluation is included
func StandardWeightsWithQuality() map[string]float64 {
	return map[string]float64{
		ProductivityEvaluatorID: 0.25,
		LatencyEvaluatorID:      0.15,
		CostEvaluatorID:         0.15,
		ErrorEvaluatorID:        0.15,
		ToolUsageEvaluatorID:    0.10,
		QualityEvaluatorID:      0.20,
	}
}
