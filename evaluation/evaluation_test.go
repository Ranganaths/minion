package evaluation

import (
	"context"
	"testing"
	"time"

	"github.com/Ranganaths/minion/tracing"
)

// TestInMemoryEvaluationStore tests the in-memory store implementation
func TestInMemoryEvaluationStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryEvaluationStore()

	// Test SaveEvaluation
	t.Run("SaveEvaluation", func(t *testing.T) {
		eval := &Evaluation{
			ID:          NewEvaluationID(),
			AgentID:     "agent-1",
			Type:        TypeProductivity,
			EvaluatorID: "productivity",
			Score:       0.85,
			Scope:       ScopeTrace,
			TraceID:     "trace-1",
			CreatedAt:   time.Now(),
		}

		err := store.SaveEvaluation(ctx, eval)
		if err != nil {
			t.Fatalf("SaveEvaluation failed: %v", err)
		}
	})

	// Test GetEvaluation
	t.Run("GetEvaluation", func(t *testing.T) {
		eval := &Evaluation{
			ID:          NewEvaluationID(),
			AgentID:     "agent-1",
			Type:        TypeLatency,
			EvaluatorID: "latency",
			Score:       0.75,
			Scope:       ScopeTrace,
			CreatedAt:   time.Now(),
		}
		store.SaveEvaluation(ctx, eval)

		retrieved, err := store.GetEvaluation(ctx, eval.ID)
		if err != nil {
			t.Fatalf("GetEvaluation failed: %v", err)
		}
		if retrieved.Score != 0.75 {
			t.Errorf("Expected score 0.75, got %f", retrieved.Score)
		}
	})

	// Test ListEvaluations with filter
	t.Run("ListEvaluations", func(t *testing.T) {
		// Add more evaluations
		for i := 0; i < 5; i++ {
			eval := &Evaluation{
				ID:          NewEvaluationID(),
				AgentID:     "agent-2",
				Type:        TypeProductivity,
				EvaluatorID: "productivity",
				Score:       float64(i) * 0.2,
				Scope:       ScopeTrace,
				CreatedAt:   time.Now(),
			}
			store.SaveEvaluation(ctx, eval)
		}

		// Filter by agent
		result, err := store.ListEvaluations(ctx, &EvaluationFilter{
			AgentID: "agent-2",
		})
		if err != nil {
			t.Fatalf("ListEvaluations failed: %v", err)
		}
		if len(result.Evaluations) != 5 {
			t.Errorf("Expected 5 evaluations, got %d", len(result.Evaluations))
		}

		// Filter by min score (scores are 0, 0.2, 0.4, 0.6, 0.8 - only 0.6 and 0.8 are >= 0.5)
		minScore := 0.5
		result, err = store.ListEvaluations(ctx, &EvaluationFilter{
			AgentID:  "agent-2",
			MinScore: &minScore,
		})
		if err != nil {
			t.Fatalf("ListEvaluations failed: %v", err)
		}
		if len(result.Evaluations) != 2 {
			t.Errorf("Expected 2 evaluations with score >= 0.5, got %d", len(result.Evaluations))
		}
	})

	// Test DeleteEvaluation
	t.Run("DeleteEvaluation", func(t *testing.T) {
		eval := &Evaluation{
			ID:          NewEvaluationID(),
			AgentID:     "agent-delete",
			Type:        TypeCost,
			EvaluatorID: "cost",
			Score:       0.5,
			Scope:       ScopeTrace,
			CreatedAt:   time.Now(),
		}
		store.SaveEvaluation(ctx, eval)

		err := store.DeleteEvaluation(ctx, eval.ID)
		if err != nil {
			t.Fatalf("DeleteEvaluation failed: %v", err)
		}

		_, err = store.GetEvaluation(ctx, eval.ID)
		if err == nil {
			t.Error("Expected error after deletion")
		}
	})

	// Test GetAgentSummary
	t.Run("GetAgentSummary", func(t *testing.T) {
		summary, err := store.GetAgentSummary(ctx, "agent-2", Last24Hours)
		if err != nil {
			t.Fatalf("GetAgentSummary failed: %v", err)
		}
		if summary.AgentID != "agent-2" {
			t.Errorf("Expected agent-2, got %s", summary.AgentID)
		}
		if summary.TotalEvaluations == 0 {
			t.Error("Expected non-zero evaluations")
		}
	})

	// Test GetStats
	t.Run("GetStats", func(t *testing.T) {
		stats, err := store.GetStats(ctx)
		if err != nil {
			t.Fatalf("GetStats failed: %v", err)
		}
		if stats.TotalEvaluations == 0 {
			t.Error("Expected non-zero evaluations")
		}
	})
}

// TestBenchmarkStore tests benchmark CRUD operations
func TestBenchmarkStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryEvaluationStore()

	// Test SaveBenchmark
	t.Run("SaveBenchmark", func(t *testing.T) {
		benchmark := &Benchmark{
			ID:          "bench-1",
			Name:        "Test Benchmark",
			Description: "A test benchmark",
			TestCases: []BenchmarkCase{
				{ID: "case-1", Name: "Test 1", Input: "input 1"},
				{ID: "case-2", Name: "Test 2", Input: "input 2"},
			},
			Tags: []string{"test"},
		}

		err := store.SaveBenchmark(ctx, benchmark)
		if err != nil {
			t.Fatalf("SaveBenchmark failed: %v", err)
		}
	})

	// Test GetBenchmark
	t.Run("GetBenchmark", func(t *testing.T) {
		retrieved, err := store.GetBenchmark(ctx, "bench-1")
		if err != nil {
			t.Fatalf("GetBenchmark failed: %v", err)
		}
		if retrieved.Name != "Test Benchmark" {
			t.Errorf("Expected 'Test Benchmark', got %s", retrieved.Name)
		}
		if len(retrieved.TestCases) != 2 {
			t.Errorf("Expected 2 test cases, got %d", len(retrieved.TestCases))
		}
	})

	// Test ListBenchmarks
	t.Run("ListBenchmarks", func(t *testing.T) {
		benchmarks, err := store.ListBenchmarks(ctx)
		if err != nil {
			t.Fatalf("ListBenchmarks failed: %v", err)
		}
		if len(benchmarks) != 1 {
			t.Errorf("Expected 1 benchmark, got %d", len(benchmarks))
		}
	})

	// Test DeleteBenchmark
	t.Run("DeleteBenchmark", func(t *testing.T) {
		err := store.DeleteBenchmark(ctx, "bench-1")
		if err != nil {
			t.Fatalf("DeleteBenchmark failed: %v", err)
		}

		_, err = store.GetBenchmark(ctx, "bench-1")
		if err == nil {
			t.Error("Expected error after deletion")
		}
	})
}

// TestBenchmarkBuilder tests the fluent benchmark builder
func TestBenchmarkBuilder(t *testing.T) {
	benchmark := NewBenchmark("My Benchmark").
		WithDescription("Test description").
		WithTags("tag1", "tag2").
		AddCase("case-1", "What is 2+2?").
			WithExpectedOutput("4").
			WithMaxIterations(5).
			RequireCompletion().
			RequireMinScore(0.8).
			Done().
		AddCase("case-2", "Search for weather").
			WithExpectedTools("search").
			WithTimeout(60).
			RequireTools("search").
			Done().
		Build()

	if benchmark.Name != "My Benchmark" {
		t.Errorf("Expected name 'My Benchmark', got %s", benchmark.Name)
	}

	if len(benchmark.TestCases) != 2 {
		t.Errorf("Expected 2 test cases, got %d", len(benchmark.TestCases))
	}

	// Check first case
	if benchmark.TestCases[0].ExpectedOutput != "4" {
		t.Error("Expected output not set correctly")
	}
	if benchmark.TestCases[0].PassCriteria == nil {
		t.Error("Pass criteria not set")
	}
	if benchmark.TestCases[0].PassCriteria.MinScore != 0.8 {
		t.Errorf("Expected min score 0.8, got %f", benchmark.TestCases[0].PassCriteria.MinScore)
	}

	// Check second case
	if len(benchmark.TestCases[1].ExpectedTools) != 1 {
		t.Error("Expected tools not set correctly")
	}
}

// TestPipeline tests the evaluation pipeline
func TestPipeline(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryEvaluationStore()

	// Create a simple test evaluator
	testEval := &testEvaluator{score: 0.9}

	pipeline := NewPipelineWithOptions(
		WithStore(store),
		WithEvaluators(testEval),
	)

	trace := &tracing.Trace{
		ID:             "trace-1",
		AgentID:        "agent-1",
		Status:         tracing.SpanStatusOK,
		IterationCount: 2,
	}

	evals, err := pipeline.EvaluateTrace(ctx, trace)
	if err != nil {
		t.Fatalf("EvaluateTrace failed: %v", err)
	}

	if len(evals) != 1 {
		t.Errorf("Expected 1 evaluation, got %d", len(evals))
	}

	if evals[0].Score != 0.9 {
		t.Errorf("Expected score 0.9, got %f", evals[0].Score)
	}

	// Check it was stored
	stored, err := store.GetEvaluationsByTrace(ctx, "trace-1")
	if err != nil {
		t.Fatalf("GetEvaluationsByTrace failed: %v", err)
	}
	if len(stored) != 1 {
		t.Errorf("Expected 1 stored evaluation, got %d", len(stored))
	}
}

// TestPipelineParallel tests parallel evaluation
func TestPipelineParallel(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryEvaluationStore()

	pipeline := NewPipelineWithOptions(
		WithStore(store),
		WithEvaluators(
			&testEvaluator{id: "eval-1", score: 0.8},
			&testEvaluator{id: "eval-2", score: 0.9},
			&testEvaluator{id: "eval-3", score: 0.7},
		),
		WithParallel(true),
	)

	trace := &tracing.Trace{
		ID:             "trace-parallel",
		AgentID:        "agent-1",
		Status:         tracing.SpanStatusOK,
		IterationCount: 2,
	}

	evals, err := pipeline.EvaluateTrace(ctx, trace)
	if err != nil {
		t.Fatalf("EvaluateTrace failed: %v", err)
	}

	if len(evals) != 3 {
		t.Errorf("Expected 3 evaluations, got %d", len(evals))
	}
}

// TestWorker tests the evaluation worker
func TestWorker(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := NewInMemoryEvaluationStore()
	pipeline := NewPipelineWithOptions(
		WithStore(store),
		WithEvaluators(&testEvaluator{score: 0.85}),
	)

	worker := NewWorker(WorkerConfig{
		Pipeline:    pipeline,
		QueueSize:   10,
		Concurrency: 2,
	})

	worker.Start(ctx)

	// Enqueue a trace
	trace := &tracing.Trace{
		ID:             "trace-worker",
		AgentID:        "agent-1",
		Status:         tracing.SpanStatusOK,
		IterationCount: 1,
	}

	if !worker.Enqueue(trace) {
		t.Error("Failed to enqueue trace")
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	worker.Stop()

	// Check evaluation was created
	evals, err := store.GetEvaluationsByTrace(ctx, "trace-worker")
	if err != nil {
		t.Fatalf("GetEvaluationsByTrace failed: %v", err)
	}
	if len(evals) != 1 {
		t.Errorf("Expected 1 evaluation, got %d", len(evals))
	}
}

// TestReportGenerator tests report generation
func TestReportGenerator(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryEvaluationStore()

	// Add test data
	for i := 0; i < 10; i++ {
		eval := &Evaluation{
			ID:          NewEvaluationID(),
			AgentID:     "agent-report",
			TraceID:     tracing.TraceID("trace-" + string(rune('0'+i))),
			Type:        TypeProductivity,
			EvaluatorID: "productivity",
			Score:       float64(i) * 0.1,
			Scope:       ScopeTrace,
			Metrics: &EvaluationMetrics{
				TaskCompleted:  i > 3,
				TotalTokens:    100 * (i + 1),
				TotalCost:      0.01 * float64(i+1),
				TotalDurationMs: int64(1000 * (i + 1)),
				ErrorCount:     i % 3,
			},
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		}
		store.SaveEvaluation(ctx, eval)
	}

	reportGen := NewReportGenerator(store)

	// Test GenerateAgentReport
	t.Run("GenerateAgentReport", func(t *testing.T) {
		report, err := reportGen.GenerateAgentReport(ctx, "agent-report", Last24Hours)
		if err != nil {
			t.Fatalf("GenerateAgentReport failed: %v", err)
		}

		if report.AgentID != "agent-report" {
			t.Errorf("Expected agent-report, got %s", report.AgentID)
		}

		if report.Summary == nil {
			t.Error("Summary should not be nil")
		}

		if len(report.Recommendations) == 0 {
			t.Error("Expected recommendations")
		}
	})
}

// TestAPIServer tests the API server handlers
func TestAPIServer(t *testing.T) {
	store := NewInMemoryEvaluationStore()

	// Add test data
	ctx := context.Background()
	eval := &Evaluation{
		ID:          NewEvaluationID(),
		AgentID:     "agent-api",
		Type:        TypeProductivity,
		EvaluatorID: "productivity",
		Score:       0.85,
		Scope:       ScopeTrace,
		CreatedAt:   time.Now(),
	}
	store.SaveEvaluation(ctx, eval)

	server := NewAPIServer(APIConfig{
		Store: store,
	})

	// Verify server was created
	if server == nil {
		t.Fatal("Server should not be nil")
	}

	// Handler should be available
	handler := server.Handler()
	if handler == nil {
		t.Fatal("Handler should not be nil")
	}
}

// testEvaluator is a simple evaluator for testing
type testEvaluator struct {
	id    string
	score float64
}

func (e *testEvaluator) ID() string {
	if e.id == "" {
		return "test-evaluator"
	}
	return e.id
}

func (e *testEvaluator) Name() string {
	return "Test Evaluator"
}

func (e *testEvaluator) Type() EvaluationType {
	return TypeProductivity
}

func (e *testEvaluator) Evaluate(ctx context.Context, trace *tracing.Trace) (*Evaluation, error) {
	return &Evaluation{
		ID:          NewEvaluationID(),
		TraceID:     trace.ID,
		AgentID:     trace.AgentID,
		Type:        TypeProductivity,
		EvaluatorID: e.ID(),
		Score:       e.score,
		Scope:       ScopeTrace,
		CreatedAt:   time.Now(),
	}, nil
}

func (e *testEvaluator) EvaluateBatch(ctx context.Context, traces []*tracing.Trace) ([]*Evaluation, error) {
	var evals []*Evaluation
	for _, trace := range traces {
		eval, _ := e.Evaluate(ctx, trace)
		evals = append(evals, eval)
	}
	return evals, nil
}

func (e *testEvaluator) Configure(config map[string]interface{}) error {
	return nil
}

// Test time period range calculation
func TestGetTimePeriodRange(t *testing.T) {
	tests := []struct {
		period      TimePeriod
		minDuration time.Duration
		maxDuration time.Duration
	}{
		{Last1Hour, 59 * time.Minute, 61 * time.Minute},
		{Last24Hours, 23 * time.Hour, 25 * time.Hour},
		{Last7Days, 6*24*time.Hour + 23*time.Hour, 7*24*time.Hour + time.Hour},
		{Last30Days, 29*24*time.Hour + 23*time.Hour, 30*24*time.Hour + time.Hour},
	}

	for _, tt := range tests {
		t.Run(string(tt.period), func(t *testing.T) {
			start, end := GetTimePeriodRange(tt.period)
			duration := end.Sub(start)

			if duration < tt.minDuration || duration > tt.maxDuration {
				t.Errorf("Period %s: duration %v not in expected range [%v, %v]",
					tt.period, duration, tt.minDuration, tt.maxDuration)
			}
		})
	}
}

// Test EvaluationFilter defaults
func TestEvaluationFilterDefaults(t *testing.T) {
	filter := &EvaluationFilter{}
	filter.ApplyDefaults()

	if filter.Limit != DefaultLimit {
		t.Errorf("Expected default limit %d, got %d", DefaultLimit, filter.Limit)
	}

	if filter.OrderBy != "created_at" {
		t.Errorf("Expected default order_by 'created_at', got %s", filter.OrderBy)
	}

	// Test max limit enforcement
	filter = &EvaluationFilter{Limit: 10000}
	filter.ApplyDefaults()
	if filter.Limit != MaxLimit {
		t.Errorf("Expected max limit %d, got %d", MaxLimit, filter.Limit)
	}
}

// Test benchmark run store operations
func TestBenchmarkRunStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryEvaluationStore()

	// First save a benchmark
	benchmark := &Benchmark{
		ID:   "bench-run-test",
		Name: "Run Test Benchmark",
	}
	store.SaveBenchmark(ctx, benchmark)

	// Save a run
	run := &BenchmarkRun{
		ID:            "run-1",
		BenchmarkID:   "bench-run-test",
		BenchmarkName: "Run Test Benchmark",
		AgentID:       "agent-1",
		Status:        RunStatusRunning,
		StartedAt:     time.Now(),
	}

	err := store.SaveBenchmarkRun(ctx, run)
	if err != nil {
		t.Fatalf("SaveBenchmarkRun failed: %v", err)
	}

	// Get the run
	retrieved, err := store.GetBenchmarkRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetBenchmarkRun failed: %v", err)
	}
	if retrieved.Status != RunStatusRunning {
		t.Errorf("Expected status running, got %s", retrieved.Status)
	}

	// Update the run
	run.Status = RunStatusCompleted
	now := time.Now()
	run.CompletedAt = &now
	err = store.UpdateBenchmarkRun(ctx, run)
	if err != nil {
		t.Fatalf("UpdateBenchmarkRun failed: %v", err)
	}

	// Verify update
	retrieved, _ = store.GetBenchmarkRun(ctx, "run-1")
	if retrieved.Status != RunStatusCompleted {
		t.Errorf("Expected status completed, got %s", retrieved.Status)
	}

	// List runs
	runs, err := store.ListBenchmarkRuns(ctx, "bench-run-test")
	if err != nil {
		t.Fatalf("ListBenchmarkRuns failed: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("Expected 1 run, got %d", len(runs))
	}
}

// Test cleanup
func TestStoreCleanup(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryEvaluationStore()

	// Add old evaluations
	oldTime := time.Now().Add(-48 * time.Hour)
	for i := 0; i < 5; i++ {
		eval := &Evaluation{
			ID:          NewEvaluationID(),
			AgentID:     "agent-cleanup",
			Type:        TypeProductivity,
			EvaluatorID: "productivity",
			Score:       0.5,
			Scope:       ScopeTrace,
			CreatedAt:   oldTime,
		}
		store.SaveEvaluation(ctx, eval)
	}

	// Add recent evaluations
	for i := 0; i < 3; i++ {
		eval := &Evaluation{
			ID:          NewEvaluationID(),
			AgentID:     "agent-cleanup",
			Type:        TypeProductivity,
			EvaluatorID: "productivity",
			Score:       0.5,
			Scope:       ScopeTrace,
			CreatedAt:   time.Now(),
		}
		store.SaveEvaluation(ctx, eval)
	}

	// Cleanup old evaluations
	deleted, err := store.Cleanup(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	if deleted != 5 {
		t.Errorf("Expected 5 deleted, got %d", deleted)
	}

	// Verify remaining
	result, _ := store.ListEvaluations(ctx, &EvaluationFilter{AgentID: "agent-cleanup"})
	if len(result.Evaluations) != 3 {
		t.Errorf("Expected 3 remaining, got %d", len(result.Evaluations))
	}
}
