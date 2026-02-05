package evaluation

import (
	"context"
	"time"

	"github.com/Ranganaths/minion/tracing"
)

// EvaluationStore defines the interface for evaluation persistence
type EvaluationStore interface {
	// Evaluation CRUD operations
	SaveEvaluation(ctx context.Context, eval *Evaluation) error
	GetEvaluation(ctx context.Context, id EvaluationID) (*Evaluation, error)
	ListEvaluations(ctx context.Context, filter *EvaluationFilter) (*EvaluationQueryResult, error)
	DeleteEvaluation(ctx context.Context, id EvaluationID) error

	// Benchmark CRUD operations
	SaveBenchmark(ctx context.Context, benchmark *Benchmark) error
	GetBenchmark(ctx context.Context, id string) (*Benchmark, error)
	ListBenchmarks(ctx context.Context) ([]*Benchmark, error)
	DeleteBenchmark(ctx context.Context, id string) error

	// Benchmark Run operations
	SaveBenchmarkRun(ctx context.Context, run *BenchmarkRun) error
	GetBenchmarkRun(ctx context.Context, id string) (*BenchmarkRun, error)
	UpdateBenchmarkRun(ctx context.Context, run *BenchmarkRun) error
	ListBenchmarkRuns(ctx context.Context, benchmarkID string) ([]*BenchmarkRun, error)

	// Aggregation operations
	GetAgentSummary(ctx context.Context, agentID string, period TimePeriod) (*EvaluationSummary, error)
	GetEvaluationsByTrace(ctx context.Context, traceID tracing.TraceID) ([]*Evaluation, error)
	GetEvaluationsByAgent(ctx context.Context, agentID string, limit int) ([]*Evaluation, error)

	// Statistics and maintenance
	GetStats(ctx context.Context) (*EvaluationStoreStats, error)
	Cleanup(ctx context.Context, olderThan time.Duration) (int64, error)
}

// EvaluationFilter defines filtering criteria for evaluations
type EvaluationFilter struct {
	// AgentID filters by agent
	AgentID string `json:"agent_id,omitempty"`

	// SessionID filters by session
	SessionID string `json:"session_id,omitempty"`

	// TraceID filters by trace
	TraceID tracing.TraceID `json:"trace_id,omitempty"`

	// BatchID filters by batch/benchmark run
	BatchID string `json:"batch_id,omitempty"`

	// Type filters by evaluation type
	Type EvaluationType `json:"type,omitempty"`

	// EvaluatorID filters by evaluator
	EvaluatorID string `json:"evaluator_id,omitempty"`

	// Scope filters by evaluation scope
	Scope EvaluationScope `json:"scope,omitempty"`

	// MinScore filters evaluations with score >= this value
	MinScore *float64 `json:"min_score,omitempty"`

	// MaxScore filters evaluations with score <= this value
	MaxScore *float64 `json:"max_score,omitempty"`

	// StartTime filters evaluations created after this time
	StartTime *time.Time `json:"start_time,omitempty"`

	// EndTime filters evaluations created before this time
	EndTime *time.Time `json:"end_time,omitempty"`

	// HasHumanFeedback filters evaluations with/without human feedback
	HasHumanFeedback *bool `json:"has_human_feedback,omitempty"`

	// Limit is the maximum number of results
	Limit int `json:"limit,omitempty"`

	// Offset is the number of results to skip
	Offset int `json:"offset,omitempty"`

	// OrderBy specifies the field to order by
	OrderBy string `json:"order_by,omitempty"`

	// OrderDesc specifies descending order
	OrderDesc bool `json:"order_desc,omitempty"`
}

// EvaluationQueryResult is the result of an evaluation query
type EvaluationQueryResult struct {
	// Evaluations are the matching evaluations
	Evaluations []*Evaluation `json:"evaluations"`

	// TotalCount is the total number of matching evaluations
	TotalCount int64 `json:"total_count"`

	// HasMore indicates if there are more results
	HasMore bool `json:"has_more"`
}

// DefaultLimit is the default limit for queries
const DefaultLimit = 100

// MaxLimit is the maximum limit for queries
const MaxLimit = 1000

// ApplyDefaults applies default values to the filter
func (f *EvaluationFilter) ApplyDefaults() {
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.OrderBy == "" {
		f.OrderBy = "created_at"
	}
}

// GetTimePeriodRange returns the start and end time for a time period
func GetTimePeriodRange(period TimePeriod) (start, end time.Time) {
	end = time.Now()
	switch period {
	case Last1Hour:
		start = end.Add(-1 * time.Hour)
	case Last24Hours:
		start = end.Add(-24 * time.Hour)
	case Last7Days:
		start = end.Add(-7 * 24 * time.Hour)
	case Last30Days:
		start = end.Add(-30 * 24 * time.Hour)
	case AllTime:
		start = time.Time{} // Zero time
	default:
		start = end.Add(-24 * time.Hour) // Default to 24 hours
	}
	return start, end
}
