package evaluation

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Ranganaths/minion/tracing"
)

// InMemoryEvaluationStore is an in-memory implementation of EvaluationStore
type InMemoryEvaluationStore struct {
	mu            sync.RWMutex
	evaluations   map[EvaluationID]*Evaluation
	benchmarks    map[string]*Benchmark
	benchmarkRuns map[string]*BenchmarkRun
}

// NewInMemoryEvaluationStore creates a new in-memory evaluation store
func NewInMemoryEvaluationStore() *InMemoryEvaluationStore {
	return &InMemoryEvaluationStore{
		evaluations:   make(map[EvaluationID]*Evaluation),
		benchmarks:    make(map[string]*Benchmark),
		benchmarkRuns: make(map[string]*BenchmarkRun),
	}
}

// SaveEvaluation saves an evaluation
func (s *InMemoryEvaluationStore) SaveEvaluation(ctx context.Context, eval *Evaluation) error {
	if eval == nil {
		return fmt.Errorf("evaluation cannot be nil")
	}
	if eval.ID == "" {
		eval.ID = NewEvaluationID()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.evaluations[eval.ID] = eval
	return nil
}

// GetEvaluation retrieves an evaluation by ID
func (s *InMemoryEvaluationStore) GetEvaluation(ctx context.Context, id EvaluationID) (*Evaluation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	eval, ok := s.evaluations[id]
	if !ok {
		return nil, fmt.Errorf("evaluation not found: %s", id)
	}
	return eval, nil
}

// ListEvaluations lists evaluations matching the filter
func (s *InMemoryEvaluationStore) ListEvaluations(ctx context.Context, filter *EvaluationFilter) (*EvaluationQueryResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if filter == nil {
		filter = &EvaluationFilter{}
	}
	filter.ApplyDefaults()

	// Collect matching evaluations
	var matches []*Evaluation
	for _, eval := range s.evaluations {
		if s.matchesFilter(eval, filter) {
			matches = append(matches, eval)
		}
	}

	// Sort
	s.sortEvaluations(matches, filter.OrderBy, filter.OrderDesc)

	// Calculate total before pagination
	totalCount := int64(len(matches))

	// Apply pagination
	start := filter.Offset
	if start > len(matches) {
		start = len(matches)
	}
	end := start + filter.Limit
	if end > len(matches) {
		end = len(matches)
	}

	return &EvaluationQueryResult{
		Evaluations: matches[start:end],
		TotalCount:  totalCount,
		HasMore:     end < len(matches),
	}, nil
}

// DeleteEvaluation deletes an evaluation
func (s *InMemoryEvaluationStore) DeleteEvaluation(ctx context.Context, id EvaluationID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.evaluations[id]; !ok {
		return fmt.Errorf("evaluation not found: %s", id)
	}
	delete(s.evaluations, id)
	return nil
}

// SaveBenchmark saves a benchmark
func (s *InMemoryEvaluationStore) SaveBenchmark(ctx context.Context, benchmark *Benchmark) error {
	if benchmark == nil {
		return fmt.Errorf("benchmark cannot be nil")
	}
	if benchmark.ID == "" {
		return fmt.Errorf("benchmark ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	benchmark.UpdatedAt = time.Now()
	if benchmark.CreatedAt.IsZero() {
		benchmark.CreatedAt = benchmark.UpdatedAt
	}

	s.benchmarks[benchmark.ID] = benchmark
	return nil
}

// GetBenchmark retrieves a benchmark by ID
func (s *InMemoryEvaluationStore) GetBenchmark(ctx context.Context, id string) (*Benchmark, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	benchmark, ok := s.benchmarks[id]
	if !ok {
		return nil, fmt.Errorf("benchmark not found: %s", id)
	}
	return benchmark, nil
}

// ListBenchmarks lists all benchmarks
func (s *InMemoryEvaluationStore) ListBenchmarks(ctx context.Context) ([]*Benchmark, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	benchmarks := make([]*Benchmark, 0, len(s.benchmarks))
	for _, b := range s.benchmarks {
		benchmarks = append(benchmarks, b)
	}

	// Sort by created time descending
	sort.Slice(benchmarks, func(i, j int) bool {
		return benchmarks[i].CreatedAt.After(benchmarks[j].CreatedAt)
	})

	return benchmarks, nil
}

// DeleteBenchmark deletes a benchmark
func (s *InMemoryEvaluationStore) DeleteBenchmark(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.benchmarks[id]; !ok {
		return fmt.Errorf("benchmark not found: %s", id)
	}
	delete(s.benchmarks, id)
	return nil
}

// SaveBenchmarkRun saves a benchmark run
func (s *InMemoryEvaluationStore) SaveBenchmarkRun(ctx context.Context, run *BenchmarkRun) error {
	if run == nil {
		return fmt.Errorf("benchmark run cannot be nil")
	}
	if run.ID == "" {
		return fmt.Errorf("benchmark run ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.benchmarkRuns[run.ID] = run
	return nil
}

// GetBenchmarkRun retrieves a benchmark run by ID
func (s *InMemoryEvaluationStore) GetBenchmarkRun(ctx context.Context, id string) (*BenchmarkRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	run, ok := s.benchmarkRuns[id]
	if !ok {
		return nil, fmt.Errorf("benchmark run not found: %s", id)
	}
	return run, nil
}

// UpdateBenchmarkRun updates a benchmark run
func (s *InMemoryEvaluationStore) UpdateBenchmarkRun(ctx context.Context, run *BenchmarkRun) error {
	if run == nil {
		return fmt.Errorf("benchmark run cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.benchmarkRuns[run.ID]; !ok {
		return fmt.Errorf("benchmark run not found: %s", run.ID)
	}

	s.benchmarkRuns[run.ID] = run
	return nil
}

// ListBenchmarkRuns lists benchmark runs for a benchmark
func (s *InMemoryEvaluationStore) ListBenchmarkRuns(ctx context.Context, benchmarkID string) ([]*BenchmarkRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var runs []*BenchmarkRun
	for _, run := range s.benchmarkRuns {
		if benchmarkID == "" || run.BenchmarkID == benchmarkID {
			runs = append(runs, run)
		}
	}

	// Sort by started time descending
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})

	return runs, nil
}

// GetAgentSummary calculates an evaluation summary for an agent
func (s *InMemoryEvaluationStore) GetAgentSummary(ctx context.Context, agentID string, period TimePeriod) (*EvaluationSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	startTime, endTime := GetTimePeriodRange(period)

	summary := &EvaluationSummary{
		AgentID:               agentID,
		Period:                period,
		StartTime:             startTime,
		EndTime:               endTime,
		ScoresByType:          make(map[EvaluationType]float64),
		IterationDistribution: make(map[int]int),
	}

	var (
		totalScore        float64
		totalTokens       int64
		totalCost         float64
		totalDuration     int64
		totalQualityScore float64
		qualityCount      int
		totalHumanRating  float64
		humanRatingCount  int
		completedTasks    int
		errorTasks        int
		traceIDs          = make(map[tracing.TraceID]bool)
		scoresByType      = make(map[EvaluationType][]float64)
	)

	for _, eval := range s.evaluations {
		// Filter by agent and time
		if eval.AgentID != agentID {
			continue
		}
		if !startTime.IsZero() && eval.CreatedAt.Before(startTime) {
			continue
		}
		if eval.CreatedAt.After(endTime) {
			continue
		}

		summary.TotalEvaluations++
		totalScore += eval.Score
		scoresByType[eval.Type] = append(scoresByType[eval.Type], eval.Score)

		// Track unique traces
		if eval.TraceID != "" {
			traceIDs[eval.TraceID] = true
		}

		// Aggregate metrics
		if eval.Metrics != nil {
			totalTokens += int64(eval.Metrics.TotalTokens)
			totalCost += eval.Metrics.TotalCost
			totalDuration += eval.Metrics.TotalDurationMs

			if eval.Metrics.TaskCompleted {
				completedTasks++
			}
			if eval.Metrics.ErrorCount > 0 {
				errorTasks++
			}

			// Track iteration distribution
			if eval.Metrics.IterationsUsed > 0 {
				summary.IterationDistribution[eval.Metrics.IterationsUsed]++
			}
		}

		// Quality scores
		if eval.QualityAssessment != nil {
			totalQualityScore += eval.QualityAssessment.OverallScore
			qualityCount++
		}

		// Human feedback
		if eval.HumanFeedback != nil {
			summary.HumanFeedbackCount++
			if eval.HumanFeedback.Rating > 0 {
				totalHumanRating += float64(eval.HumanFeedback.Rating)
				humanRatingCount++
			}
		}
	}

	summary.TotalTraces = len(traceIDs)
	summary.TotalTokens = totalTokens
	summary.TotalCost = totalCost

	// Calculate averages
	if summary.TotalEvaluations > 0 {
		summary.AvgScore = totalScore / float64(summary.TotalEvaluations)
		summary.AvgDurationMs = float64(totalDuration) / float64(summary.TotalEvaluations)
	}

	if summary.TotalTraces > 0 {
		summary.TaskCompletionRate = float64(completedTasks) / float64(summary.TotalTraces)
		summary.ErrorRate = float64(errorTasks) / float64(summary.TotalTraces)
		summary.AvgTokensPerTask = float64(totalTokens) / float64(summary.TotalTraces)
		summary.AvgCostPerTask = totalCost / float64(summary.TotalTraces)
	}

	if qualityCount > 0 {
		summary.AvgQualityScore = totalQualityScore / float64(qualityCount)
	}

	if humanRatingCount > 0 {
		summary.AvgHumanRating = totalHumanRating / float64(humanRatingCount)
	}

	// Calculate average scores by type
	for evalType, scores := range scoresByType {
		if len(scores) > 0 {
			var sum float64
			for _, score := range scores {
				sum += score
			}
			summary.ScoresByType[evalType] = sum / float64(len(scores))
		}
	}

	return summary, nil
}

// GetEvaluationsByTrace retrieves all evaluations for a trace
func (s *InMemoryEvaluationStore) GetEvaluationsByTrace(ctx context.Context, traceID tracing.TraceID) ([]*Evaluation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var evals []*Evaluation
	for _, eval := range s.evaluations {
		if eval.TraceID == traceID {
			evals = append(evals, eval)
		}
	}

	// Sort by created time
	sort.Slice(evals, func(i, j int) bool {
		return evals[i].CreatedAt.Before(evals[j].CreatedAt)
	})

	return evals, nil
}

// GetEvaluationsByAgent retrieves evaluations for an agent
func (s *InMemoryEvaluationStore) GetEvaluationsByAgent(ctx context.Context, agentID string, limit int) ([]*Evaluation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var evals []*Evaluation
	for _, eval := range s.evaluations {
		if eval.AgentID == agentID {
			evals = append(evals, eval)
		}
	}

	// Sort by created time descending
	sort.Slice(evals, func(i, j int) bool {
		return evals[i].CreatedAt.After(evals[j].CreatedAt)
	})

	// Apply limit
	if limit > 0 && len(evals) > limit {
		evals = evals[:limit]
	}

	return evals, nil
}

// GetStats returns store statistics
func (s *InMemoryEvaluationStore) GetStats(ctx context.Context) (*EvaluationStoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &EvaluationStoreStats{
		TotalEvaluations:   int64(len(s.evaluations)),
		TotalBenchmarks:    int64(len(s.benchmarks)),
		TotalBenchmarkRuns: int64(len(s.benchmarkRuns)),
		EvaluationsByType:  make(map[EvaluationType]int64),
		EvaluationsByAgent: make(map[string]int64),
	}

	var totalScore float64
	var oldest, newest time.Time

	for _, eval := range s.evaluations {
		// Count by type
		stats.EvaluationsByType[eval.Type]++

		// Count by agent
		stats.EvaluationsByAgent[eval.AgentID]++

		// Accumulate scores
		totalScore += eval.Score

		// Track time range
		if oldest.IsZero() || eval.CreatedAt.Before(oldest) {
			oldest = eval.CreatedAt
		}
		if newest.IsZero() || eval.CreatedAt.After(newest) {
			newest = eval.CreatedAt
		}
	}

	if stats.TotalEvaluations > 0 {
		stats.AvgScore = totalScore / float64(stats.TotalEvaluations)
	}

	if !oldest.IsZero() {
		stats.OldestEvaluation = &oldest
	}
	if !newest.IsZero() {
		stats.NewestEvaluation = &newest
	}

	return stats, nil
}

// Cleanup removes evaluations older than the specified duration
func (s *InMemoryEvaluationStore) Cleanup(ctx context.Context, olderThan time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	var deleted int64

	for id, eval := range s.evaluations {
		if eval.CreatedAt.Before(cutoff) {
			delete(s.evaluations, id)
			deleted++
		}
	}

	return deleted, nil
}

// matchesFilter checks if an evaluation matches the filter criteria
func (s *InMemoryEvaluationStore) matchesFilter(eval *Evaluation, filter *EvaluationFilter) bool {
	if filter.AgentID != "" && eval.AgentID != filter.AgentID {
		return false
	}
	if filter.SessionID != "" && eval.SessionID != filter.SessionID {
		return false
	}
	if filter.TraceID != "" && eval.TraceID != filter.TraceID {
		return false
	}
	if filter.BatchID != "" && eval.BatchID != filter.BatchID {
		return false
	}
	if filter.Type != "" && eval.Type != filter.Type {
		return false
	}
	if filter.EvaluatorID != "" && eval.EvaluatorID != filter.EvaluatorID {
		return false
	}
	if filter.Scope != "" && eval.Scope != filter.Scope {
		return false
	}
	if filter.MinScore != nil && eval.Score < *filter.MinScore {
		return false
	}
	if filter.MaxScore != nil && eval.Score > *filter.MaxScore {
		return false
	}
	if filter.StartTime != nil && eval.CreatedAt.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && eval.CreatedAt.After(*filter.EndTime) {
		return false
	}
	if filter.HasHumanFeedback != nil {
		hasFeedback := eval.HumanFeedback != nil
		if *filter.HasHumanFeedback != hasFeedback {
			return false
		}
	}

	return true
}

// sortEvaluations sorts evaluations by the specified field
func (s *InMemoryEvaluationStore) sortEvaluations(evals []*Evaluation, orderBy string, desc bool) {
	sort.Slice(evals, func(i, j int) bool {
		var less bool
		switch orderBy {
		case "score":
			less = evals[i].Score < evals[j].Score
		case "agent_id":
			less = evals[i].AgentID < evals[j].AgentID
		case "type":
			less = evals[i].Type < evals[j].Type
		case "created_at":
			fallthrough
		default:
			less = evals[i].CreatedAt.Before(evals[j].CreatedAt)
		}
		if desc {
			return !less
		}
		return less
	})
}

// Ensure InMemoryEvaluationStore implements EvaluationStore
var _ EvaluationStore = (*InMemoryEvaluationStore)(nil)
