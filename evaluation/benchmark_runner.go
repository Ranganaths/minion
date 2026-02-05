package evaluation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Ranganaths/minion/tracing"
	"github.com/google/uuid"
)

// AgentExecutor interface for running agents
// This abstracts the agent executor to avoid circular imports
type AgentExecutor interface {
	Run(ctx context.Context, input string) (string, error)
}

// TracedAgentExecutor interface extends AgentExecutor with trace access
type TracedAgentExecutor interface {
	AgentExecutor
	GetLastTrace() *tracing.Trace
}

// BenchmarkRunner executes benchmarks against agents
type BenchmarkRunner struct {
	store       EvaluationStore
	traceStore  tracing.TraceStore
	evaluators  []Evaluator
	parallelism int
}

// BenchmarkRunnerConfig configures the benchmark runner
type BenchmarkRunnerConfig struct {
	// Store is the evaluation store for persisting results
	Store EvaluationStore

	// TraceStore is the trace store for persisting traces (optional)
	TraceStore tracing.TraceStore

	// Evaluators are the evaluators to run on each trace
	Evaluators []Evaluator

	// Parallelism is the max concurrent test cases (default: 1)
	Parallelism int
}

// NewBenchmarkRunner creates a new benchmark runner
func NewBenchmarkRunner(config BenchmarkRunnerConfig) *BenchmarkRunner {
	parallelism := config.Parallelism
	if parallelism <= 0 {
		parallelism = 1
	}

	return &BenchmarkRunner{
		store:       config.Store,
		traceStore:  config.TraceStore,
		evaluators:  config.Evaluators,
		parallelism: parallelism,
	}
}

// RunOptions configures a benchmark run
type RunOptions struct {
	// Parallel enables parallel test execution
	Parallel bool

	// MaxParallel is the maximum parallel executions
	MaxParallel int

	// StopOnFailure stops the run on first failure
	StopOnFailure bool

	// IncludeQuality includes LLM-as-Judge evaluation
	IncludeQuality bool

	// Timeout is the overall run timeout
	Timeout time.Duration

	// CaseTimeout is the timeout per test case
	CaseTimeout time.Duration
}

// DefaultRunOptions returns default run options
func DefaultRunOptions() *RunOptions {
	return &RunOptions{
		Parallel:    false,
		MaxParallel: 4,
		Timeout:     30 * time.Minute,
		CaseTimeout: 5 * time.Minute,
	}
}

// Run executes a benchmark against an agent
func (r *BenchmarkRunner) Run(ctx context.Context, benchmark *Benchmark, executor AgentExecutor, opts *RunOptions) (*BenchmarkRun, error) {
	if benchmark == nil {
		return nil, fmt.Errorf("benchmark cannot be nil")
	}
	if executor == nil {
		return nil, fmt.Errorf("executor cannot be nil")
	}
	if opts == nil {
		opts = DefaultRunOptions()
	}

	// Apply timeout
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Create benchmark run
	run := &BenchmarkRun{
		ID:            uuid.New().String(),
		BenchmarkID:   benchmark.ID,
		BenchmarkName: benchmark.Name,
		Status:        RunStatusRunning,
		StartedAt:     time.Now(),
		Config: &BenchmarkRunConfig{
			Parallel:       opts.Parallel,
			MaxParallel:    opts.MaxParallel,
			StopOnFailure:  opts.StopOnFailure,
			IncludeQuality: opts.IncludeQuality,
		},
	}

	// Save initial run state
	if r.store != nil {
		if err := r.store.SaveBenchmarkRun(ctx, run); err != nil {
			return nil, fmt.Errorf("failed to save benchmark run: %w", err)
		}
	}

	// Execute test cases
	var results []BenchmarkCaseResult
	var runErr error

	if opts.Parallel && opts.MaxParallel > 1 {
		results, runErr = r.runParallel(ctx, benchmark, executor, opts, run)
	} else {
		results, runErr = r.runSequential(ctx, benchmark, executor, opts, run)
	}

	run.Results = results

	// Calculate summary
	run.Summary = r.calculateSummary(results)

	// Update status
	now := time.Now()
	run.CompletedAt = &now

	if runErr != nil {
		run.Status = RunStatusFailed
		run.Error = runErr.Error()
	} else {
		run.Status = RunStatusCompleted
	}

	// Save final run state
	if r.store != nil {
		if err := r.store.UpdateBenchmarkRun(ctx, run); err != nil {
			return run, fmt.Errorf("failed to update benchmark run: %w", err)
		}
	}

	return run, runErr
}

// runSequential runs test cases one at a time
func (r *BenchmarkRunner) runSequential(ctx context.Context, benchmark *Benchmark, executor AgentExecutor, opts *RunOptions, run *BenchmarkRun) ([]BenchmarkCaseResult, error) {
	results := make([]BenchmarkCaseResult, 0, len(benchmark.TestCases))

	for _, testCase := range benchmark.TestCases {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		result := r.runCase(ctx, &testCase, executor, opts, run.ID)
		results = append(results, result)

		// Check stop on failure
		if opts.StopOnFailure && !result.Passed {
			return results, nil
		}
	}

	return results, nil
}

// runParallel runs test cases concurrently
func (r *BenchmarkRunner) runParallel(ctx context.Context, benchmark *Benchmark, executor AgentExecutor, opts *RunOptions, run *BenchmarkRun) ([]BenchmarkCaseResult, error) {
	parallelism := opts.MaxParallel
	if parallelism <= 0 {
		parallelism = r.parallelism
	}

	results := make([]BenchmarkCaseResult, len(benchmark.TestCases))
	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	var stopped bool

	for i, testCase := range benchmark.TestCases {
		wg.Add(1)
		go func(idx int, tc BenchmarkCase) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[idx] = BenchmarkCaseResult{
					CaseID:   tc.ID,
					CaseName: tc.Name,
					Passed:   false,
					Error:    ctx.Err().Error(),
				}
				return
			}

			// Check if stopped
			mu.Lock()
			if stopped {
				mu.Unlock()
				results[idx] = BenchmarkCaseResult{
					CaseID:   tc.ID,
					CaseName: tc.Name,
					Passed:   false,
					Error:    "run stopped",
				}
				return
			}
			mu.Unlock()

			result := r.runCase(ctx, &tc, executor, opts, run.ID)
			results[idx] = result

			// Handle stop on failure
			if opts.StopOnFailure && !result.Passed {
				mu.Lock()
				if !stopped {
					stopped = true
					if firstErr == nil {
						firstErr = fmt.Errorf("test case failed: %s", tc.ID)
					}
				}
				mu.Unlock()
			}
		}(i, testCase)
	}

	wg.Wait()
	return results, firstErr
}

// runCase executes a single test case
func (r *BenchmarkRunner) runCase(ctx context.Context, testCase *BenchmarkCase, executor AgentExecutor, opts *RunOptions, batchID string) BenchmarkCaseResult {
	result := BenchmarkCaseResult{
		CaseID:   testCase.ID,
		CaseName: testCase.Name,
	}

	// Apply case timeout
	caseTimeout := opts.CaseTimeout
	if testCase.TimeoutSeconds > 0 {
		caseTimeout = time.Duration(testCase.TimeoutSeconds) * time.Second
	}
	if caseTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, caseTimeout)
		defer cancel()
	}

	// Execute agent
	startTime := time.Now()
	output, err := executor.Run(ctx, testCase.Input)
	duration := time.Since(startTime)

	result.DurationMs = duration.Milliseconds()
	result.ActualOutput = output

	if err != nil {
		result.Passed = false
		result.Error = err.Error()
		result.FailReasons = append(result.FailReasons, fmt.Sprintf("execution error: %v", err))
		return result
	}

	// Get trace if available
	var trace *tracing.Trace
	if tracedExec, ok := executor.(TracedAgentExecutor); ok {
		trace = tracedExec.GetLastTrace()
		if trace != nil {
			result.TraceID = trace.ID
			result.TokensUsed = trace.TotalTokens.TotalTokens
			result.Cost = trace.TotalCost
		}
	}

	// Run evaluators if trace is available
	if trace != nil && len(r.evaluators) > 0 {
		var evals []*Evaluation
		for _, evaluator := range r.evaluators {
			eval, evalErr := evaluator.Evaluate(ctx, trace)
			if evalErr != nil {
				continue
			}
			eval.BatchID = batchID
			evals = append(evals, eval)

			// Store evaluation
			if r.store != nil {
				r.store.SaveEvaluation(ctx, eval)
			}
		}

		// Use first evaluation as the main one
		if len(evals) > 0 {
			result.Evaluation = evals[0]
		}
	}

	// Check pass criteria
	result.Passed, result.FailReasons = r.checkPassCriteria(testCase, &result, trace)

	return result
}

// checkPassCriteria checks if the result meets the pass criteria
func (r *BenchmarkRunner) checkPassCriteria(testCase *BenchmarkCase, result *BenchmarkCaseResult, trace *tracing.Trace) (bool, []string) {
	criteria := testCase.PassCriteria
	if criteria == nil {
		// Default: just check completion
		if result.Error != "" {
			return false, []string{"execution error: " + result.Error}
		}
		return true, nil
	}

	var failReasons []string
	passed := true

	// Check completion
	if criteria.RequireCompletion {
		if result.Error != "" {
			passed = false
			failReasons = append(failReasons, "task did not complete")
		}
	}

	// Check no errors
	if criteria.RequireNoErrors {
		if result.Error != "" {
			passed = false
			failReasons = append(failReasons, "execution had errors")
		}
	}

	// Check score
	if criteria.MinScore > 0 && result.Evaluation != nil {
		if result.Evaluation.Score < criteria.MinScore {
			passed = false
			failReasons = append(failReasons, fmt.Sprintf("score %.2f below minimum %.2f", result.Evaluation.Score, criteria.MinScore))
		}
	}

	// Check tokens
	if criteria.MaxTokens > 0 && result.TokensUsed > criteria.MaxTokens {
		passed = false
		failReasons = append(failReasons, fmt.Sprintf("tokens %d exceeded max %d", result.TokensUsed, criteria.MaxTokens))
	}

	// Check cost
	if criteria.MaxCost > 0 && result.Cost > criteria.MaxCost {
		passed = false
		failReasons = append(failReasons, fmt.Sprintf("cost %.4f exceeded max %.4f", result.Cost, criteria.MaxCost))
	}

	// Check duration
	if criteria.MaxDurationMs > 0 && result.DurationMs > criteria.MaxDurationMs {
		passed = false
		failReasons = append(failReasons, fmt.Sprintf("duration %dms exceeded max %dms", result.DurationMs, criteria.MaxDurationMs))
	}

	// Check required tools
	if len(criteria.RequiredTools) > 0 && trace != nil {
		usedTools := make(map[string]bool)
		for _, span := range trace.GetToolCalls() {
			if span.ToolDetails != nil {
				usedTools[span.ToolDetails.ToolName] = true
			}
		}
		for _, required := range criteria.RequiredTools {
			if !usedTools[required] {
				passed = false
				failReasons = append(failReasons, fmt.Sprintf("required tool not used: %s", required))
			}
		}
	}

	// Check expected tools
	if len(testCase.ExpectedTools) > 0 && trace != nil {
		usedTools := make(map[string]bool)
		for _, span := range trace.GetToolCalls() {
			if span.ToolDetails != nil {
				usedTools[span.ToolDetails.ToolName] = true
			}
		}
		for _, expected := range testCase.ExpectedTools {
			if !usedTools[expected] {
				passed = false
				failReasons = append(failReasons, fmt.Sprintf("expected tool not used: %s", expected))
			}
		}
	}

	// Check max iterations
	if testCase.MaxIterations > 0 && trace != nil {
		if trace.IterationCount > testCase.MaxIterations {
			passed = false
			failReasons = append(failReasons, fmt.Sprintf("iterations %d exceeded max %d", trace.IterationCount, testCase.MaxIterations))
		}
	}

	return passed, failReasons
}

// calculateSummary calculates the benchmark summary from results
func (r *BenchmarkRunner) calculateSummary(results []BenchmarkCaseResult) *BenchmarkSummary {
	summary := &BenchmarkSummary{
		TotalCases:     len(results),
		ScoresByTag:    make(map[string]float64),
		FailureReasons: make(map[string]int),
	}

	var totalScore float64
	var totalQualityScore float64
	var qualityCount int
	var scoreCount int

	for _, result := range results {
		if result.Passed {
			summary.PassedCases++
		} else {
			summary.FailedCases++
			for _, reason := range result.FailReasons {
				summary.FailureReasons[reason]++
			}
		}

		summary.TotalTokens += result.TokensUsed
		summary.TotalCost += result.Cost
		summary.TotalDurationMs += result.DurationMs

		if result.Evaluation != nil {
			totalScore += result.Evaluation.Score
			scoreCount++

			if result.Evaluation.QualityAssessment != nil {
				totalQualityScore += result.Evaluation.QualityAssessment.OverallScore
				qualityCount++
			}
		}
	}

	// Calculate rates and averages
	if summary.TotalCases > 0 {
		summary.PassRate = float64(summary.PassedCases) / float64(summary.TotalCases)
		summary.AvgDurationMs = float64(summary.TotalDurationMs) / float64(summary.TotalCases)
	}

	if scoreCount > 0 {
		summary.AvgScore = totalScore / float64(scoreCount)
	}

	if qualityCount > 0 {
		summary.AvgQualityScore = totalQualityScore / float64(qualityCount)
	}

	return summary
}

// AddEvaluator adds an evaluator to the runner
func (r *BenchmarkRunner) AddEvaluator(evaluator Evaluator) {
	r.evaluators = append(r.evaluators, evaluator)
}

// SetEvaluators sets the evaluators
func (r *BenchmarkRunner) SetEvaluators(evaluators []Evaluator) {
	r.evaluators = evaluators
}
