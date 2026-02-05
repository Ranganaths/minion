// Package main demonstrates the evaluation system for measuring agent productivity.
//
// This example shows:
// 1. Creating and configuring evaluators
// 2. Building benchmarks with the fluent API
// 3. Running evaluations on traces
// 4. Generating reports
// 5. Setting up the evaluation pipeline
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Ranganaths/minion/evaluation"
	"github.com/Ranganaths/minion/evaluation/evaluators"
	"github.com/Ranganaths/minion/tracing"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== Agent Evaluation System Demo ===")

	// 1. Create evaluation store
	store := evaluation.NewInMemoryEvaluationStore()
	fmt.Println("1. Created in-memory evaluation store")

	// 2. Create evaluators
	productivityEval := evaluators.NewProductivityEvaluator()
	latencyEval := evaluators.NewLatencyEvaluator()
	costEval := evaluators.NewCostEvaluator()
	errorEval := evaluators.NewErrorEvaluator()
	toolUsageEval := evaluators.NewToolUsageEvaluator()

	fmt.Println("2. Created evaluators:")
	fmt.Println("   - Productivity Evaluator")
	fmt.Println("   - Latency Evaluator")
	fmt.Println("   - Cost Evaluator")
	fmt.Println("   - Error Evaluator")
	fmt.Println("   - Tool Usage Evaluator")

	// 3. Create composite evaluator
	compositeEval := evaluators.NewCompositeEvaluator(
		evaluators.WeightedEvaluator{Evaluator: productivityEval, Weight: 0.30},
		evaluators.WeightedEvaluator{Evaluator: latencyEval, Weight: 0.20},
		evaluators.WeightedEvaluator{Evaluator: costEval, Weight: 0.20},
		evaluators.WeightedEvaluator{Evaluator: errorEval, Weight: 0.15},
		evaluators.WeightedEvaluator{Evaluator: toolUsageEval, Weight: 0.15},
	).SetParallel(true)

	fmt.Println("3. Created composite evaluator with weighted components")

	// 4. Create sample traces for demonstration
	traces := createSampleTraces()
	fmt.Printf("\n4. Created %d sample traces for evaluation\n", len(traces))

	// 5. Evaluate traces
	fmt.Println("\n5. Evaluating traces...")
	for _, trace := range traces {
		eval, err := compositeEval.Evaluate(ctx, trace)
		if err != nil {
			log.Printf("   Error evaluating trace %s: %v", trace.ID, err)
			continue
		}

		// Save evaluation
		if err := store.SaveEvaluation(ctx, eval); err != nil {
			log.Printf("   Error saving evaluation: %v", err)
			continue
		}

		fmt.Printf("   Trace %s: Score=%.2f, Type=%s\n",
			trace.ID[:8], eval.Score, eval.Type)
	}

	// 6. Build a benchmark
	fmt.Println("\n6. Building benchmark...")
	benchmark := evaluation.NewBenchmark("Agent Performance Benchmark").
		WithDescription("Tests agent performance across various dimensions").
		WithTags("performance", "regression").
		AddCase("simple-calculation", "What is 2 + 2?").
			WithExpectedOutput("4").
			WithMaxIterations(3).
			RequireCompletion().
			RequireMinScore(0.7).
			Done().
		AddCase("tool-usage", "Use the calculator to compute 15 * 7").
			WithExpectedTools("calculator").
			WithMaxIterations(5).
			RequireCompletion().
			RequireTools("calculator").
			Done().
		AddCase("complex-reasoning", "Explain why the sky is blue").
			WithMaxIterations(3).
			RequireMinScore(0.6).
			WithTimeout(60).
			Done().
		Build()

	fmt.Printf("   Created benchmark: %s\n", benchmark.Name)
	fmt.Printf("   Test cases: %d\n", len(benchmark.TestCases))

	// Save benchmark
	if err := store.SaveBenchmark(ctx, benchmark); err != nil {
		log.Printf("   Error saving benchmark: %v", err)
	}

	// 7. Create evaluation pipeline
	fmt.Println("\n7. Setting up evaluation pipeline...")
	pipeline := evaluation.NewPipelineWithOptions(
		evaluation.WithStore(store),
		evaluation.WithEvaluators(compositeEval),
		evaluation.WithParallel(true),
	)

	// Add hook for demonstration
	pipeline.AddHook(&DemoEvaluationHook{})

	// Evaluate a trace through the pipeline
	pipelineEvals, err := pipeline.EvaluateTrace(ctx, traces[0])
	if err != nil {
		log.Printf("   Pipeline evaluation error: %v", err)
	} else {
		fmt.Printf("   Pipeline produced %d evaluations\n", len(pipelineEvals))
	}

	// 8. Generate reports
	fmt.Println("\n8. Generating reports...")
	reportGen := evaluation.NewReportGenerator(store)

	agentReport, err := reportGen.GenerateAgentReport(ctx, "demo-agent", evaluation.Last24Hours)
	if err != nil {
		log.Printf("   Error generating report: %v", err)
	} else {
		fmt.Println("\n   === Agent Report ===")
		fmt.Printf("   Agent: %s\n", agentReport.AgentID)
		fmt.Printf("   Period: %s\n", agentReport.Period)
		fmt.Printf("   Total Evaluations: %d\n", agentReport.Summary.TotalEvaluations)
		fmt.Printf("   Average Score: %.2f\n", agentReport.Summary.AvgScore)
		fmt.Printf("   Task Completion Rate: %.1f%%\n", agentReport.Summary.TaskCompletionRate*100)
		fmt.Printf("   Error Rate: %.1f%%\n", agentReport.Summary.ErrorRate*100)

		if len(agentReport.Recommendations) > 0 {
			fmt.Println("\n   Recommendations:")
			for _, rec := range agentReport.Recommendations {
				fmt.Printf("   - %s\n", rec)
			}
		}

		if len(agentReport.TopIssues) > 0 {
			fmt.Println("\n   Top Issues:")
			for _, issue := range agentReport.TopIssues {
				fmt.Printf("   - [%s] %s\n", issue.Severity, issue.Description)
			}
		}
	}

	// 9. Get store statistics
	fmt.Println("\n9. Store Statistics:")
	stats, err := store.GetStats(ctx)
	if err != nil {
		log.Printf("   Error getting stats: %v", err)
	} else {
		fmt.Printf("   Total Evaluations: %d\n", stats.TotalEvaluations)
		fmt.Printf("   Total Benchmarks: %d\n", stats.TotalBenchmarks)
		fmt.Printf("   Average Score: %.2f\n", stats.AvgScore)
		fmt.Printf("   Evaluations by Type:\n")
		for t, count := range stats.EvaluationsByType {
			fmt.Printf("     - %s: %d\n", t, count)
		}
	}

	// 10. Query evaluations
	fmt.Println("\n10. Query Examples:")

	// Query high-scoring evaluations
	highScore := 0.7
	result, err := store.ListEvaluations(ctx, &evaluation.EvaluationFilter{
		MinScore: &highScore,
		Limit:    5,
		OrderBy:  "score",
		OrderDesc: true,
	})
	if err != nil {
		log.Printf("   Query error: %v", err)
	} else {
		fmt.Printf("   High-scoring evaluations (>0.7): %d found\n", result.TotalCount)
	}

	// 11. Start evaluation worker (demonstration)
	fmt.Println("\n11. Starting evaluation worker...")
	worker := evaluation.NewWorker(evaluation.WorkerConfig{
		Pipeline:    pipeline,
		QueueSize:   100,
		Concurrency: 2,
	})

	worker.Start(ctx)
	fmt.Println("   Worker started with concurrency=2")

	// Enqueue a trace
	if worker.Enqueue(traces[0]) {
		fmt.Println("   Trace enqueued for background evaluation")
	}

	// Give worker time to process
	time.Sleep(100 * time.Millisecond)

	worker.Stop()
	fmt.Println("   Worker stopped")

	fmt.Println("\n=== Demo Complete ===")
}

// createSampleTraces creates sample traces for demonstration
func createSampleTraces() []*tracing.Trace {
	now := time.Now()

	traces := []*tracing.Trace{
		{
			ID:             tracing.TraceID("trace-001"),
			AgentID:        "demo-agent",
			AgentName:      "Demo Agent",
			Input:          "What is 2 + 2?",
			Output:         "The answer is 4.",
			Status:         tracing.SpanStatusOK,
			StartTime:      now.Add(-5 * time.Minute),
			Duration:       2500,
			IterationCount: 2,
			ToolCallCount:  1,
			TotalTokens:    tracing.TokenUsage{PromptTokens: 50, CompletionTokens: 30, TotalTokens: 80},
			TotalCost:      0.001,
			Spans: []*tracing.Span{
				{
					ID:       "span-001",
					Type:     tracing.SpanTypeToolCall,
					Name:     "Calculator",
					Status:   tracing.SpanStatusOK,
					Duration: 100,
					ToolDetails: &tracing.ToolSpanDetails{
						ToolName: "calculator",
						Input:    "2 + 2",
						Output:   "4",
					},
				},
			},
		},
		{
			ID:             tracing.TraceID("trace-002"),
			AgentID:        "demo-agent",
			AgentName:      "Demo Agent",
			Input:          "Search for weather in New York",
			Output:         "The weather in New York is sunny, 72°F.",
			Status:         tracing.SpanStatusOK,
			StartTime:      now.Add(-10 * time.Minute),
			Duration:       5000,
			IterationCount: 3,
			ToolCallCount:  2,
			TotalTokens:    tracing.TokenUsage{PromptTokens: 100, CompletionTokens: 80, TotalTokens: 180},
			TotalCost:      0.003,
			Spans: []*tracing.Span{
				{
					ID:       "span-002",
					Type:     tracing.SpanTypeToolCall,
					Name:     "Search",
					Status:   tracing.SpanStatusOK,
					Duration: 1500,
					ToolDetails: &tracing.ToolSpanDetails{
						ToolName: "search",
						Input:    "weather New York",
						Output:   "sunny, 72°F",
					},
				},
			},
		},
		{
			ID:             tracing.TraceID("trace-003"),
			AgentID:        "demo-agent",
			AgentName:      "Demo Agent",
			Input:          "Perform complex analysis",
			Output:         "",
			Status:         tracing.SpanStatusError,
			Error:          "max iterations exceeded",
			StartTime:      now.Add(-15 * time.Minute),
			Duration:       30000,
			IterationCount: 15,
			ToolCallCount:  5,
			TotalTokens:    tracing.TokenUsage{PromptTokens: 500, CompletionTokens: 400, TotalTokens: 900},
			TotalCost:      0.015,
			Spans: []*tracing.Span{
				{
					ID:       "span-003",
					Type:     tracing.SpanTypeToolCall,
					Name:     "Analysis",
					Status:   tracing.SpanStatusError,
					Duration: 5000,
					ToolDetails: &tracing.ToolSpanDetails{
						ToolName: "analysis",
						Input:    "complex data",
						Retries:  3,
					},
				},
			},
		},
	}

	return traces
}

// DemoEvaluationHook is a demonstration hook
type DemoEvaluationHook struct{}

func (h *DemoEvaluationHook) OnEvaluationStart(ctx context.Context, trace *tracing.Trace) {
	fmt.Printf("   [Hook] Starting evaluation for trace %s\n", trace.ID[:8])
}

func (h *DemoEvaluationHook) OnEvaluationComplete(ctx context.Context, trace *tracing.Trace, evals []*evaluation.Evaluation) {
	fmt.Printf("   [Hook] Completed evaluation for trace %s: %d results\n", trace.ID[:8], len(evals))
}

func (h *DemoEvaluationHook) OnEvaluationError(ctx context.Context, trace *tracing.Trace, err error) {
	fmt.Printf("   [Hook] Error evaluating trace %s: %v\n", trace.ID[:8], err)
}
