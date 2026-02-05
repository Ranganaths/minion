// Package evaluation provides agent evaluation and benchmarking capabilities.
// It measures agent productivity through multiple dimensions: task completion,
// token efficiency, cost optimization, quality assessment, and error handling.
package evaluation

import (
	"time"

	"github.com/Ranganaths/minion/tracing"
	"github.com/google/uuid"
)

// EvaluationID uniquely identifies an evaluation
type EvaluationID string

// NewEvaluationID generates a new unique evaluation ID
func NewEvaluationID() EvaluationID {
	return EvaluationID(uuid.New().String())
}

// EvaluationScope defines the scope of an evaluation
type EvaluationScope string

const (
	// ScopeTrace evaluates a single trace/execution
	ScopeTrace EvaluationScope = "trace"
	// ScopeAgent evaluates aggregated agent performance
	ScopeAgent EvaluationScope = "agent"
	// ScopeSession evaluates a conversation session
	ScopeSession EvaluationScope = "session"
	// ScopeBatch evaluates a batch of traces (e.g., benchmark run)
	ScopeBatch EvaluationScope = "batch"
)

// EvaluationType defines the type of evaluation
type EvaluationType string

const (
	// TypeProductivity measures task completion and efficiency
	TypeProductivity EvaluationType = "productivity"
	// TypeCost measures cost efficiency
	TypeCost EvaluationType = "cost"
	// TypeQuality measures response quality (LLM-as-Judge)
	TypeQuality EvaluationType = "quality"
	// TypeLatency measures response time and throughput
	TypeLatency EvaluationType = "latency"
	// TypeError measures error rate and recovery
	TypeError EvaluationType = "error"
	// TypeComposite combines multiple evaluation types
	TypeComposite EvaluationType = "composite"
)

// TimePeriod defines time periods for summary queries
type TimePeriod string

const (
	// Last1Hour summarizes the last hour
	Last1Hour TimePeriod = "1h"
	// Last24Hours summarizes the last 24 hours
	Last24Hours TimePeriod = "24h"
	// Last7Days summarizes the last 7 days
	Last7Days TimePeriod = "7d"
	// Last30Days summarizes the last 30 days
	Last30Days TimePeriod = "30d"
	// AllTime summarizes all available data
	AllTime TimePeriod = "all"
)

// Evaluation represents the result of evaluating an agent's performance
type Evaluation struct {
	// ID uniquely identifies this evaluation
	ID EvaluationID `json:"id"`

	// TraceID links to the trace being evaluated (for trace-scope evaluations)
	TraceID tracing.TraceID `json:"trace_id,omitempty"`

	// AgentID identifies the agent being evaluated
	AgentID string `json:"agent_id"`

	// SessionID links to the conversation session (optional)
	SessionID string `json:"session_id,omitempty"`

	// BatchID links to a batch evaluation (e.g., benchmark run)
	BatchID string `json:"batch_id,omitempty"`

	// Scope defines what this evaluation covers
	Scope EvaluationScope `json:"scope"`

	// Type defines the evaluation type
	Type EvaluationType `json:"type"`

	// EvaluatorID identifies which evaluator produced this evaluation
	EvaluatorID string `json:"evaluator_id"`

	// Score is the normalized score (0.0 - 1.0)
	Score float64 `json:"score"`

	// Subscores provides breakdown by dimension
	Subscores map[string]float64 `json:"subscores,omitempty"`

	// Metrics contains detailed evaluation metrics
	Metrics *EvaluationMetrics `json:"metrics,omitempty"`

	// QualityAssessment contains LLM-as-Judge quality evaluation
	QualityAssessment *QualityAssessment `json:"quality_assessment,omitempty"`

	// HumanFeedback contains human feedback if provided
	HumanFeedback *HumanFeedback `json:"human_feedback,omitempty"`

	// Metadata contains additional evaluation-specific data
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// CreatedAt is when this evaluation was created
	CreatedAt time.Time `json:"created_at"`
}

// EvaluationMetrics contains detailed quantitative metrics
type EvaluationMetrics struct {
	// Productivity metrics
	TaskCompleted       bool `json:"task_completed"`
	IterationsUsed      int  `json:"iterations_used"`
	MaxIterations       int  `json:"max_iterations"`
	ToolCallsCount      int  `json:"tool_calls_count"`
	SuccessfulToolCalls int  `json:"successful_tool_calls"`
	FailedToolCalls     int  `json:"failed_tool_calls"`

	// Token efficiency metrics
	TotalTokens        int     `json:"total_tokens"`
	PromptTokens       int     `json:"prompt_tokens"`
	CompletionTokens   int     `json:"completion_tokens"`
	TokensPerIteration float64 `json:"tokens_per_iteration"`

	// Cost metrics
	TotalCost    float64 `json:"total_cost"`
	CostPerToken float64 `json:"cost_per_token"`

	// Latency metrics
	TotalDurationMs        int64   `json:"total_duration_ms"`
	AvgIterationDurationMs float64 `json:"avg_iteration_duration_ms"`
	FirstTokenLatencyMs    int64   `json:"first_token_latency_ms"`
	LLMCallCount           int     `json:"llm_call_count"`
	AvgLLMLatencyMs        float64 `json:"avg_llm_latency_ms"`

	// Error metrics
	ErrorCount   int     `json:"error_count"`
	RetryCount   int     `json:"retry_count"`
	RecoveryRate float64 `json:"recovery_rate"` // % of errors recovered from
}

// QualityAssessment contains LLM-as-Judge evaluation results
type QualityAssessment struct {
	// OverallScore is the aggregate quality score (0.0 - 1.0)
	OverallScore float64 `json:"overall_score"`

	// Relevance measures how relevant the answer is to the input
	Relevance float64 `json:"relevance"`

	// Coherence measures logical flow and consistency
	Coherence float64 `json:"coherence"`

	// Completeness measures coverage of requirements
	Completeness float64 `json:"completeness"`

	// Accuracy measures factual correctness
	Accuracy float64 `json:"accuracy"`

	// Helpfulness measures practical usefulness
	Helpfulness float64 `json:"helpfulness"`

	// Safety measures if response is safe and appropriate
	Safety float64 `json:"safety"`

	// JudgeModel identifies which model performed the evaluation
	JudgeModel string `json:"judge_model"`

	// JudgeReasoning contains the judge's explanation
	JudgeReasoning string `json:"judge_reasoning,omitempty"`

	// Confidence indicates how confident the judge is (0.0 - 1.0)
	Confidence float64 `json:"confidence"`
}

// HumanFeedback contains human-provided feedback
type HumanFeedback struct {
	// Rating is a 1-5 scale rating
	Rating int `json:"rating,omitempty"`

	// Thumbs is a simple thumbs up/down (true=up, false=down)
	Thumbs *bool `json:"thumbs,omitempty"`

	// Comment is optional text feedback
	Comment string `json:"comment,omitempty"`

	// Tags are categorization tags
	Tags []string `json:"tags,omitempty"`

	// ReviewerID identifies who provided the feedback
	ReviewerID string `json:"reviewer_id,omitempty"`

	// SubmittedAt is when the feedback was submitted
	SubmittedAt time.Time `json:"submitted_at"`
}

// EvaluationSummary provides aggregated evaluation statistics
type EvaluationSummary struct {
	// AgentID identifies the agent
	AgentID string `json:"agent_id"`

	// Period is the time period covered
	Period TimePeriod `json:"period"`

	// StartTime is the start of the period
	StartTime time.Time `json:"start_time"`

	// EndTime is the end of the period
	EndTime time.Time `json:"end_time"`

	// TotalEvaluations is the number of evaluations
	TotalEvaluations int `json:"total_evaluations"`

	// TotalTraces is the number of traces evaluated
	TotalTraces int `json:"total_traces"`

	// AvgScore is the average normalized score
	AvgScore float64 `json:"avg_score"`

	// TaskCompletionRate is the percentage of tasks completed
	TaskCompletionRate float64 `json:"task_completion_rate"`

	// AvgTokensPerTask is average tokens per task
	AvgTokensPerTask float64 `json:"avg_tokens_per_task"`

	// AvgCostPerTask is average cost per task
	AvgCostPerTask float64 `json:"avg_cost_per_task"`

	// TotalCost is total cost in the period
	TotalCost float64 `json:"total_cost"`

	// TotalTokens is total tokens used
	TotalTokens int64 `json:"total_tokens"`

	// AvgDurationMs is average execution duration
	AvgDurationMs float64 `json:"avg_duration_ms"`

	// ErrorRate is the percentage of tasks with errors
	ErrorRate float64 `json:"error_rate"`

	// AvgQualityScore is average LLM-as-Judge score
	AvgQualityScore float64 `json:"avg_quality_score"`

	// AvgHumanRating is average human rating
	AvgHumanRating float64 `json:"avg_human_rating"`

	// HumanFeedbackCount is number of human feedbacks
	HumanFeedbackCount int `json:"human_feedback_count"`

	// ScoresByType provides breakdown by evaluation type
	ScoresByType map[EvaluationType]float64 `json:"scores_by_type"`

	// IterationDistribution shows distribution of iterations used
	IterationDistribution map[int]int `json:"iteration_distribution,omitempty"`
}

// Benchmark defines a test suite for evaluating agents
type Benchmark struct {
	// ID uniquely identifies this benchmark
	ID string `json:"id"`

	// Name is the human-readable benchmark name
	Name string `json:"name"`

	// Description describes what this benchmark tests
	Description string `json:"description"`

	// TestCases are the individual test cases
	TestCases []BenchmarkCase `json:"test_cases"`

	// Tags categorize this benchmark
	Tags []string `json:"tags,omitempty"`

	// CreatedAt is when this benchmark was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when this benchmark was last updated
	UpdatedAt time.Time `json:"updated_at"`
}

// BenchmarkCase defines a single test case in a benchmark
type BenchmarkCase struct {
	// ID uniquely identifies this test case
	ID string `json:"id"`

	// Name is the human-readable test case name
	Name string `json:"name,omitempty"`

	// Input is the input to provide to the agent
	Input string `json:"input"`

	// ExpectedOutput is the expected output (for comparison)
	ExpectedOutput string `json:"expected_output,omitempty"`

	// ExpectedTools are tools expected to be used
	ExpectedTools []string `json:"expected_tools,omitempty"`

	// MaxIterations is the maximum allowed iterations
	MaxIterations int `json:"max_iterations,omitempty"`

	// MaxTokens is the maximum allowed tokens
	MaxTokens int `json:"max_tokens,omitempty"`

	// MaxCost is the maximum allowed cost
	MaxCost float64 `json:"max_cost,omitempty"`

	// TimeoutSeconds is the maximum execution time
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`

	// Tags categorize this test case
	Tags []string `json:"tags,omitempty"`

	// Weight determines this case's importance in scoring
	Weight float64 `json:"weight,omitempty"`

	// PassCriteria defines what constitutes passing
	PassCriteria *PassCriteria `json:"pass_criteria,omitempty"`
}

// PassCriteria defines the criteria for a test case to pass
type PassCriteria struct {
	// MinScore is the minimum required score
	MinScore float64 `json:"min_score,omitempty"`

	// RequireCompletion requires task completion
	RequireCompletion bool `json:"require_completion,omitempty"`

	// RequireNoErrors requires no errors
	RequireNoErrors bool `json:"require_no_errors,omitempty"`

	// MaxTokens is the maximum allowed tokens
	MaxTokens int `json:"max_tokens,omitempty"`

	// MaxCost is the maximum allowed cost
	MaxCost float64 `json:"max_cost,omitempty"`

	// MaxDurationMs is the maximum allowed duration
	MaxDurationMs int64 `json:"max_duration_ms,omitempty"`

	// RequiredTools are tools that must be used
	RequiredTools []string `json:"required_tools,omitempty"`

	// CustomCriteria allows custom pass/fail logic
	CustomCriteria map[string]interface{} `json:"custom_criteria,omitempty"`
}

// BenchmarkRun represents an execution of a benchmark
type BenchmarkRun struct {
	// ID uniquely identifies this benchmark run
	ID string `json:"id"`

	// BenchmarkID links to the benchmark definition
	BenchmarkID string `json:"benchmark_id"`

	// BenchmarkName is the benchmark name (denormalized)
	BenchmarkName string `json:"benchmark_name"`

	// AgentID identifies the agent being tested
	AgentID string `json:"agent_id"`

	// AgentName is the agent name (denormalized)
	AgentName string `json:"agent_name,omitempty"`

	// Status is the current run status
	Status BenchmarkRunStatus `json:"status"`

	// Results are the individual case results
	Results []BenchmarkCaseResult `json:"results,omitempty"`

	// Summary is the aggregated results
	Summary *BenchmarkSummary `json:"summary,omitempty"`

	// Config contains run configuration
	Config *BenchmarkRunConfig `json:"config,omitempty"`

	// StartedAt is when the run started
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when the run completed
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Error contains error message if run failed
	Error string `json:"error,omitempty"`
}

// BenchmarkRunStatus represents the status of a benchmark run
type BenchmarkRunStatus string

const (
	// RunStatusPending indicates the run is pending
	RunStatusPending BenchmarkRunStatus = "pending"
	// RunStatusRunning indicates the run is in progress
	RunStatusRunning BenchmarkRunStatus = "running"
	// RunStatusCompleted indicates the run completed successfully
	RunStatusCompleted BenchmarkRunStatus = "completed"
	// RunStatusFailed indicates the run failed
	RunStatusFailed BenchmarkRunStatus = "failed"
	// RunStatusCancelled indicates the run was cancelled
	RunStatusCancelled BenchmarkRunStatus = "cancelled"
)

// BenchmarkRunConfig contains configuration for a benchmark run
type BenchmarkRunConfig struct {
	// Parallel indicates whether to run cases in parallel
	Parallel bool `json:"parallel,omitempty"`

	// MaxParallel is the maximum parallel executions
	MaxParallel int `json:"max_parallel,omitempty"`

	// StopOnFailure stops the run on first failure
	StopOnFailure bool `json:"stop_on_failure,omitempty"`

	// IncludeQuality includes LLM-as-Judge evaluation
	IncludeQuality bool `json:"include_quality,omitempty"`

	// TimeoutSeconds is the overall run timeout
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

// BenchmarkCaseResult represents the result of a single test case
type BenchmarkCaseResult struct {
	// CaseID links to the test case
	CaseID string `json:"case_id"`

	// CaseName is the test case name (denormalized)
	CaseName string `json:"case_name,omitempty"`

	// TraceID links to the execution trace
	TraceID tracing.TraceID `json:"trace_id,omitempty"`

	// Evaluation is the evaluation result
	Evaluation *Evaluation `json:"evaluation,omitempty"`

	// Passed indicates whether the test case passed
	Passed bool `json:"passed"`

	// FailReasons lists why the case failed
	FailReasons []string `json:"fail_reasons,omitempty"`

	// ActualOutput is the agent's actual output
	ActualOutput string `json:"actual_output,omitempty"`

	// DurationMs is the execution duration
	DurationMs int64 `json:"duration_ms"`

	// TokensUsed is the tokens consumed
	TokensUsed int `json:"tokens_used"`

	// Cost is the execution cost
	Cost float64 `json:"cost"`

	// Error contains error message if execution failed
	Error string `json:"error,omitempty"`
}

// BenchmarkSummary provides aggregated benchmark results
type BenchmarkSummary struct {
	// TotalCases is the total number of test cases
	TotalCases int `json:"total_cases"`

	// PassedCases is the number of passed cases
	PassedCases int `json:"passed_cases"`

	// FailedCases is the number of failed cases
	FailedCases int `json:"failed_cases"`

	// SkippedCases is the number of skipped cases
	SkippedCases int `json:"skipped_cases"`

	// PassRate is the percentage of passed cases
	PassRate float64 `json:"pass_rate"`

	// AvgScore is the average evaluation score
	AvgScore float64 `json:"avg_score"`

	// AvgQualityScore is the average quality score
	AvgQualityScore float64 `json:"avg_quality_score,omitempty"`

	// TotalTokens is the total tokens used
	TotalTokens int `json:"total_tokens"`

	// TotalCost is the total cost
	TotalCost float64 `json:"total_cost"`

	// TotalDurationMs is the total execution time
	TotalDurationMs int64 `json:"total_duration_ms"`

	// AvgDurationMs is the average execution time per case
	AvgDurationMs float64 `json:"avg_duration_ms"`

	// ScoresByTag provides scores grouped by tag
	ScoresByTag map[string]float64 `json:"scores_by_tag,omitempty"`

	// FailureReasons provides common failure reasons
	FailureReasons map[string]int `json:"failure_reasons,omitempty"`
}

// EvaluationStoreStats contains store-wide statistics
type EvaluationStoreStats struct {
	// TotalEvaluations is the total number of evaluations
	TotalEvaluations int64 `json:"total_evaluations"`

	// TotalBenchmarks is the total number of benchmarks
	TotalBenchmarks int64 `json:"total_benchmarks"`

	// TotalBenchmarkRuns is the total number of benchmark runs
	TotalBenchmarkRuns int64 `json:"total_benchmark_runs"`

	// EvaluationsByType provides counts by evaluation type
	EvaluationsByType map[EvaluationType]int64 `json:"evaluations_by_type"`

	// EvaluationsByAgent provides counts by agent
	EvaluationsByAgent map[string]int64 `json:"evaluations_by_agent"`

	// AvgScore is the overall average score
	AvgScore float64 `json:"avg_score"`

	// OldestEvaluation is the oldest evaluation timestamp
	OldestEvaluation *time.Time `json:"oldest_evaluation,omitempty"`

	// NewestEvaluation is the newest evaluation timestamp
	NewestEvaluation *time.Time `json:"newest_evaluation,omitempty"`
}
