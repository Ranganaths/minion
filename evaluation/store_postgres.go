package evaluation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Ranganaths/minion/tracing"
)

// PostgresEvaluationStore is a PostgreSQL implementation of EvaluationStore
type PostgresEvaluationStore struct {
	db *sql.DB
}

// NewPostgresEvaluationStore creates a new PostgreSQL evaluation store
func NewPostgresEvaluationStore(db *sql.DB) *PostgresEvaluationStore {
	return &PostgresEvaluationStore{db: db}
}

// InitSchema creates the required database tables
func (s *PostgresEvaluationStore) InitSchema(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS evaluations (
		id VARCHAR(36) PRIMARY KEY,
		trace_id VARCHAR(36),
		agent_id VARCHAR(255) NOT NULL,
		session_id VARCHAR(255),
		batch_id VARCHAR(255),
		scope VARCHAR(50) NOT NULL,
		type VARCHAR(50) NOT NULL,
		evaluator_id VARCHAR(255) NOT NULL,
		score DECIMAL(10, 6) NOT NULL,
		subscores JSONB,
		metrics JSONB,
		quality_assessment JSONB,
		human_feedback JSONB,
		metadata JSONB,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_evaluations_agent_id ON evaluations(agent_id);
	CREATE INDEX IF NOT EXISTS idx_evaluations_trace_id ON evaluations(trace_id);
	CREATE INDEX IF NOT EXISTS idx_evaluations_session_id ON evaluations(session_id);
	CREATE INDEX IF NOT EXISTS idx_evaluations_batch_id ON evaluations(batch_id);
	CREATE INDEX IF NOT EXISTS idx_evaluations_type ON evaluations(type);
	CREATE INDEX IF NOT EXISTS idx_evaluations_created_at ON evaluations(created_at);
	CREATE INDEX IF NOT EXISTS idx_evaluations_score ON evaluations(score);

	CREATE TABLE IF NOT EXISTS benchmarks (
		id VARCHAR(255) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		description TEXT,
		test_cases JSONB NOT NULL,
		tags JSONB,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS benchmark_runs (
		id VARCHAR(36) PRIMARY KEY,
		benchmark_id VARCHAR(255) NOT NULL REFERENCES benchmarks(id) ON DELETE CASCADE,
		benchmark_name VARCHAR(255) NOT NULL,
		agent_id VARCHAR(255) NOT NULL,
		agent_name VARCHAR(255),
		status VARCHAR(50) NOT NULL,
		results JSONB,
		summary JSONB,
		config JSONB,
		started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		completed_at TIMESTAMP WITH TIME ZONE,
		error TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_benchmark_runs_benchmark_id ON benchmark_runs(benchmark_id);
	CREATE INDEX IF NOT EXISTS idx_benchmark_runs_agent_id ON benchmark_runs(agent_id);
	CREATE INDEX IF NOT EXISTS idx_benchmark_runs_status ON benchmark_runs(status);
	`

	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// SaveEvaluation saves an evaluation
func (s *PostgresEvaluationStore) SaveEvaluation(ctx context.Context, eval *Evaluation) error {
	if eval == nil {
		return fmt.Errorf("evaluation cannot be nil")
	}
	if eval.ID == "" {
		eval.ID = NewEvaluationID()
	}

	subscoresJSON, _ := json.Marshal(eval.Subscores)
	metricsJSON, _ := json.Marshal(eval.Metrics)
	qualityJSON, _ := json.Marshal(eval.QualityAssessment)
	feedbackJSON, _ := json.Marshal(eval.HumanFeedback)
	metadataJSON, _ := json.Marshal(eval.Metadata)

	query := `
		INSERT INTO evaluations (
			id, trace_id, agent_id, session_id, batch_id, scope, type,
			evaluator_id, score, subscores, metrics, quality_assessment,
			human_feedback, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (id) DO UPDATE SET
			score = EXCLUDED.score,
			subscores = EXCLUDED.subscores,
			metrics = EXCLUDED.metrics,
			quality_assessment = EXCLUDED.quality_assessment,
			human_feedback = EXCLUDED.human_feedback,
			metadata = EXCLUDED.metadata
	`

	_, err := s.db.ExecContext(ctx, query,
		eval.ID, eval.TraceID, eval.AgentID, eval.SessionID, eval.BatchID,
		eval.Scope, eval.Type, eval.EvaluatorID, eval.Score,
		subscoresJSON, metricsJSON, qualityJSON, feedbackJSON, metadataJSON,
		eval.CreatedAt,
	)
	return err
}

// GetEvaluation retrieves an evaluation by ID
func (s *PostgresEvaluationStore) GetEvaluation(ctx context.Context, id EvaluationID) (*Evaluation, error) {
	query := `
		SELECT id, trace_id, agent_id, session_id, batch_id, scope, type,
			   evaluator_id, score, subscores, metrics, quality_assessment,
			   human_feedback, metadata, created_at
		FROM evaluations WHERE id = $1
	`

	eval := &Evaluation{}
	var subscoresJSON, metricsJSON, qualityJSON, feedbackJSON, metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&eval.ID, &eval.TraceID, &eval.AgentID, &eval.SessionID, &eval.BatchID,
		&eval.Scope, &eval.Type, &eval.EvaluatorID, &eval.Score,
		&subscoresJSON, &metricsJSON, &qualityJSON, &feedbackJSON, &metadataJSON,
		&eval.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("evaluation not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON fields
	if len(subscoresJSON) > 0 {
		json.Unmarshal(subscoresJSON, &eval.Subscores)
	}
	if len(metricsJSON) > 0 {
		json.Unmarshal(metricsJSON, &eval.Metrics)
	}
	if len(qualityJSON) > 0 {
		json.Unmarshal(qualityJSON, &eval.QualityAssessment)
	}
	if len(feedbackJSON) > 0 {
		json.Unmarshal(feedbackJSON, &eval.HumanFeedback)
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &eval.Metadata)
	}

	return eval, nil
}

// ListEvaluations lists evaluations matching the filter
func (s *PostgresEvaluationStore) ListEvaluations(ctx context.Context, filter *EvaluationFilter) (*EvaluationQueryResult, error) {
	if filter == nil {
		filter = &EvaluationFilter{}
	}
	filter.ApplyDefaults()

	// Build WHERE clause
	conditions := []string{}
	args := []interface{}{}
	argNum := 1

	if filter.AgentID != "" {
		conditions = append(conditions, fmt.Sprintf("agent_id = $%d", argNum))
		args = append(args, filter.AgentID)
		argNum++
	}
	if filter.SessionID != "" {
		conditions = append(conditions, fmt.Sprintf("session_id = $%d", argNum))
		args = append(args, filter.SessionID)
		argNum++
	}
	if filter.TraceID != "" {
		conditions = append(conditions, fmt.Sprintf("trace_id = $%d", argNum))
		args = append(args, filter.TraceID)
		argNum++
	}
	if filter.BatchID != "" {
		conditions = append(conditions, fmt.Sprintf("batch_id = $%d", argNum))
		args = append(args, filter.BatchID)
		argNum++
	}
	if filter.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argNum))
		args = append(args, filter.Type)
		argNum++
	}
	if filter.EvaluatorID != "" {
		conditions = append(conditions, fmt.Sprintf("evaluator_id = $%d", argNum))
		args = append(args, filter.EvaluatorID)
		argNum++
	}
	if filter.Scope != "" {
		conditions = append(conditions, fmt.Sprintf("scope = $%d", argNum))
		args = append(args, filter.Scope)
		argNum++
	}
	if filter.MinScore != nil {
		conditions = append(conditions, fmt.Sprintf("score >= $%d", argNum))
		args = append(args, *filter.MinScore)
		argNum++
	}
	if filter.MaxScore != nil {
		conditions = append(conditions, fmt.Sprintf("score <= $%d", argNum))
		args = append(args, *filter.MaxScore)
		argNum++
	}
	if filter.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argNum))
		args = append(args, *filter.StartTime)
		argNum++
	}
	if filter.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argNum))
		args = append(args, *filter.EndTime)
		argNum++
	}
	if filter.HasHumanFeedback != nil {
		if *filter.HasHumanFeedback {
			conditions = append(conditions, "human_feedback IS NOT NULL")
		} else {
			conditions = append(conditions, "human_feedback IS NULL")
		}
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM evaluations %s", whereClause)
	var totalCount int64
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, err
	}

	// Build ORDER BY
	orderColumn := "created_at"
	switch filter.OrderBy {
	case "score", "agent_id", "type", "created_at":
		orderColumn = filter.OrderBy
	}
	orderDir := "ASC"
	if filter.OrderDesc {
		orderDir = "DESC"
	}

	// Query with pagination
	query := fmt.Sprintf(`
		SELECT id, trace_id, agent_id, session_id, batch_id, scope, type,
			   evaluator_id, score, subscores, metrics, quality_assessment,
			   human_feedback, metadata, created_at
		FROM evaluations %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderColumn, orderDir, argNum, argNum+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var evaluations []*Evaluation
	for rows.Next() {
		eval := &Evaluation{}
		var subscoresJSON, metricsJSON, qualityJSON, feedbackJSON, metadataJSON []byte

		err := rows.Scan(
			&eval.ID, &eval.TraceID, &eval.AgentID, &eval.SessionID, &eval.BatchID,
			&eval.Scope, &eval.Type, &eval.EvaluatorID, &eval.Score,
			&subscoresJSON, &metricsJSON, &qualityJSON, &feedbackJSON, &metadataJSON,
			&eval.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal JSON fields
		if len(subscoresJSON) > 0 {
			json.Unmarshal(subscoresJSON, &eval.Subscores)
		}
		if len(metricsJSON) > 0 {
			json.Unmarshal(metricsJSON, &eval.Metrics)
		}
		if len(qualityJSON) > 0 {
			json.Unmarshal(qualityJSON, &eval.QualityAssessment)
		}
		if len(feedbackJSON) > 0 {
			json.Unmarshal(feedbackJSON, &eval.HumanFeedback)
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &eval.Metadata)
		}

		evaluations = append(evaluations, eval)
	}

	return &EvaluationQueryResult{
		Evaluations: evaluations,
		TotalCount:  totalCount,
		HasMore:     int64(filter.Offset+len(evaluations)) < totalCount,
	}, nil
}

// DeleteEvaluation deletes an evaluation
func (s *PostgresEvaluationStore) DeleteEvaluation(ctx context.Context, id EvaluationID) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM evaluations WHERE id = $1", id)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("evaluation not found: %s", id)
	}
	return nil
}

// SaveBenchmark saves a benchmark
func (s *PostgresEvaluationStore) SaveBenchmark(ctx context.Context, benchmark *Benchmark) error {
	if benchmark == nil {
		return fmt.Errorf("benchmark cannot be nil")
	}

	testCasesJSON, _ := json.Marshal(benchmark.TestCases)
	tagsJSON, _ := json.Marshal(benchmark.Tags)

	query := `
		INSERT INTO benchmarks (id, name, description, test_cases, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			test_cases = EXCLUDED.test_cases,
			tags = EXCLUDED.tags,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	if benchmark.CreatedAt.IsZero() {
		benchmark.CreatedAt = now
	}
	benchmark.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, query,
		benchmark.ID, benchmark.Name, benchmark.Description,
		testCasesJSON, tagsJSON, benchmark.CreatedAt, benchmark.UpdatedAt,
	)
	return err
}

// GetBenchmark retrieves a benchmark by ID
func (s *PostgresEvaluationStore) GetBenchmark(ctx context.Context, id string) (*Benchmark, error) {
	query := `
		SELECT id, name, description, test_cases, tags, created_at, updated_at
		FROM benchmarks WHERE id = $1
	`

	benchmark := &Benchmark{}
	var testCasesJSON, tagsJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&benchmark.ID, &benchmark.Name, &benchmark.Description,
		&testCasesJSON, &tagsJSON, &benchmark.CreatedAt, &benchmark.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("benchmark not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(testCasesJSON, &benchmark.TestCases)
	json.Unmarshal(tagsJSON, &benchmark.Tags)

	return benchmark, nil
}

// ListBenchmarks lists all benchmarks
func (s *PostgresEvaluationStore) ListBenchmarks(ctx context.Context) ([]*Benchmark, error) {
	query := `
		SELECT id, name, description, test_cases, tags, created_at, updated_at
		FROM benchmarks ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var benchmarks []*Benchmark
	for rows.Next() {
		benchmark := &Benchmark{}
		var testCasesJSON, tagsJSON []byte

		err := rows.Scan(
			&benchmark.ID, &benchmark.Name, &benchmark.Description,
			&testCasesJSON, &tagsJSON, &benchmark.CreatedAt, &benchmark.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(testCasesJSON, &benchmark.TestCases)
		json.Unmarshal(tagsJSON, &benchmark.Tags)

		benchmarks = append(benchmarks, benchmark)
	}

	return benchmarks, nil
}

// DeleteBenchmark deletes a benchmark
func (s *PostgresEvaluationStore) DeleteBenchmark(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM benchmarks WHERE id = $1", id)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("benchmark not found: %s", id)
	}
	return nil
}

// SaveBenchmarkRun saves a benchmark run
func (s *PostgresEvaluationStore) SaveBenchmarkRun(ctx context.Context, run *BenchmarkRun) error {
	if run == nil {
		return fmt.Errorf("benchmark run cannot be nil")
	}

	resultsJSON, _ := json.Marshal(run.Results)
	summaryJSON, _ := json.Marshal(run.Summary)
	configJSON, _ := json.Marshal(run.Config)

	query := `
		INSERT INTO benchmark_runs (
			id, benchmark_id, benchmark_name, agent_id, agent_name,
			status, results, summary, config, started_at, completed_at, error
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := s.db.ExecContext(ctx, query,
		run.ID, run.BenchmarkID, run.BenchmarkName, run.AgentID, run.AgentName,
		run.Status, resultsJSON, summaryJSON, configJSON,
		run.StartedAt, run.CompletedAt, run.Error,
	)
	return err
}

// GetBenchmarkRun retrieves a benchmark run by ID
func (s *PostgresEvaluationStore) GetBenchmarkRun(ctx context.Context, id string) (*BenchmarkRun, error) {
	query := `
		SELECT id, benchmark_id, benchmark_name, agent_id, agent_name,
			   status, results, summary, config, started_at, completed_at, error
		FROM benchmark_runs WHERE id = $1
	`

	run := &BenchmarkRun{}
	var resultsJSON, summaryJSON, configJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&run.ID, &run.BenchmarkID, &run.BenchmarkName, &run.AgentID, &run.AgentName,
		&run.Status, &resultsJSON, &summaryJSON, &configJSON,
		&run.StartedAt, &run.CompletedAt, &run.Error,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("benchmark run not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(resultsJSON, &run.Results)
	json.Unmarshal(summaryJSON, &run.Summary)
	json.Unmarshal(configJSON, &run.Config)

	return run, nil
}

// UpdateBenchmarkRun updates a benchmark run
func (s *PostgresEvaluationStore) UpdateBenchmarkRun(ctx context.Context, run *BenchmarkRun) error {
	if run == nil {
		return fmt.Errorf("benchmark run cannot be nil")
	}

	resultsJSON, _ := json.Marshal(run.Results)
	summaryJSON, _ := json.Marshal(run.Summary)

	query := `
		UPDATE benchmark_runs SET
			status = $2, results = $3, summary = $4,
			completed_at = $5, error = $6
		WHERE id = $1
	`

	result, err := s.db.ExecContext(ctx, query,
		run.ID, run.Status, resultsJSON, summaryJSON,
		run.CompletedAt, run.Error,
	)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("benchmark run not found: %s", run.ID)
	}
	return nil
}

// ListBenchmarkRuns lists benchmark runs for a benchmark
func (s *PostgresEvaluationStore) ListBenchmarkRuns(ctx context.Context, benchmarkID string) ([]*BenchmarkRun, error) {
	query := `
		SELECT id, benchmark_id, benchmark_name, agent_id, agent_name,
			   status, results, summary, config, started_at, completed_at, error
		FROM benchmark_runs
	`
	var args []interface{}

	if benchmarkID != "" {
		query += " WHERE benchmark_id = $1"
		args = append(args, benchmarkID)
	}

	query += " ORDER BY started_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*BenchmarkRun
	for rows.Next() {
		run := &BenchmarkRun{}
		var resultsJSON, summaryJSON, configJSON []byte

		err := rows.Scan(
			&run.ID, &run.BenchmarkID, &run.BenchmarkName, &run.AgentID, &run.AgentName,
			&run.Status, &resultsJSON, &summaryJSON, &configJSON,
			&run.StartedAt, &run.CompletedAt, &run.Error,
		)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(resultsJSON, &run.Results)
		json.Unmarshal(summaryJSON, &run.Summary)
		json.Unmarshal(configJSON, &run.Config)

		runs = append(runs, run)
	}

	return runs, nil
}

// GetAgentSummary calculates an evaluation summary for an agent
func (s *PostgresEvaluationStore) GetAgentSummary(ctx context.Context, agentID string, period TimePeriod) (*EvaluationSummary, error) {
	startTime, endTime := GetTimePeriodRange(period)

	summary := &EvaluationSummary{
		AgentID:               agentID,
		Period:                period,
		StartTime:             startTime,
		EndTime:               endTime,
		ScoresByType:          make(map[EvaluationType]float64),
		IterationDistribution: make(map[int]int),
	}

	// Main aggregation query
	query := `
		SELECT
			COUNT(*) as total_evaluations,
			COUNT(DISTINCT trace_id) as total_traces,
			AVG(score) as avg_score,
			SUM((metrics->>'total_tokens')::int) as total_tokens,
			SUM((metrics->>'total_cost')::float) as total_cost,
			AVG((metrics->>'total_duration_ms')::float) as avg_duration_ms,
			COUNT(*) FILTER (WHERE (metrics->>'task_completed')::boolean = true) as completed_tasks,
			COUNT(*) FILTER (WHERE (metrics->>'error_count')::int > 0) as error_tasks,
			AVG((quality_assessment->>'overall_score')::float) FILTER (WHERE quality_assessment IS NOT NULL) as avg_quality_score,
			COUNT(*) FILTER (WHERE human_feedback IS NOT NULL) as human_feedback_count,
			AVG((human_feedback->>'rating')::int) FILTER (WHERE human_feedback->>'rating' IS NOT NULL) as avg_human_rating
		FROM evaluations
		WHERE agent_id = $1
	`
	args := []interface{}{agentID}
	argNum := 2

	if !startTime.IsZero() {
		query += fmt.Sprintf(" AND created_at >= $%d", argNum)
		args = append(args, startTime)
		argNum++
	}
	query += fmt.Sprintf(" AND created_at <= $%d", argNum)
	args = append(args, endTime)

	var (
		totalTokens     sql.NullInt64
		totalCost       sql.NullFloat64
		avgDuration     sql.NullFloat64
		completedTasks  sql.NullInt64
		errorTasks      sql.NullInt64
		avgQualityScore sql.NullFloat64
		avgHumanRating  sql.NullFloat64
	)

	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&summary.TotalEvaluations,
		&summary.TotalTraces,
		&summary.AvgScore,
		&totalTokens,
		&totalCost,
		&avgDuration,
		&completedTasks,
		&errorTasks,
		&avgQualityScore,
		&summary.HumanFeedbackCount,
		&avgHumanRating,
	)
	if err != nil {
		return nil, err
	}

	if totalTokens.Valid {
		summary.TotalTokens = totalTokens.Int64
	}
	if totalCost.Valid {
		summary.TotalCost = totalCost.Float64
	}
	if avgDuration.Valid {
		summary.AvgDurationMs = avgDuration.Float64
	}
	if avgQualityScore.Valid {
		summary.AvgQualityScore = avgQualityScore.Float64
	}
	if avgHumanRating.Valid {
		summary.AvgHumanRating = avgHumanRating.Float64
	}

	// Calculate rates
	if summary.TotalTraces > 0 {
		if completedTasks.Valid {
			summary.TaskCompletionRate = float64(completedTasks.Int64) / float64(summary.TotalTraces)
		}
		if errorTasks.Valid {
			summary.ErrorRate = float64(errorTasks.Int64) / float64(summary.TotalTraces)
		}
		summary.AvgTokensPerTask = float64(summary.TotalTokens) / float64(summary.TotalTraces)
		summary.AvgCostPerTask = summary.TotalCost / float64(summary.TotalTraces)
	}

	// Get scores by type
	typeQuery := `
		SELECT type, AVG(score)
		FROM evaluations
		WHERE agent_id = $1
	`
	typeArgs := []interface{}{agentID}
	if !startTime.IsZero() {
		typeQuery += " AND created_at >= $2 AND created_at <= $3"
		typeArgs = append(typeArgs, startTime, endTime)
	} else {
		typeQuery += " AND created_at <= $2"
		typeArgs = append(typeArgs, endTime)
	}
	typeQuery += " GROUP BY type"

	rows, err := s.db.QueryContext(ctx, typeQuery, typeArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var evalType EvaluationType
		var avgScore float64
		if err := rows.Scan(&evalType, &avgScore); err != nil {
			return nil, err
		}
		summary.ScoresByType[evalType] = avgScore
	}

	return summary, nil
}

// GetEvaluationsByTrace retrieves all evaluations for a trace
func (s *PostgresEvaluationStore) GetEvaluationsByTrace(ctx context.Context, traceID tracing.TraceID) ([]*Evaluation, error) {
	result, err := s.ListEvaluations(ctx, &EvaluationFilter{
		TraceID: traceID,
		OrderBy: "created_at",
	})
	if err != nil {
		return nil, err
	}
	return result.Evaluations, nil
}

// GetEvaluationsByAgent retrieves evaluations for an agent
func (s *PostgresEvaluationStore) GetEvaluationsByAgent(ctx context.Context, agentID string, limit int) ([]*Evaluation, error) {
	result, err := s.ListEvaluations(ctx, &EvaluationFilter{
		AgentID:   agentID,
		Limit:     limit,
		OrderBy:   "created_at",
		OrderDesc: true,
	})
	if err != nil {
		return nil, err
	}
	return result.Evaluations, nil
}

// GetStats returns store statistics
func (s *PostgresEvaluationStore) GetStats(ctx context.Context) (*EvaluationStoreStats, error) {
	stats := &EvaluationStoreStats{
		EvaluationsByType:  make(map[EvaluationType]int64),
		EvaluationsByAgent: make(map[string]int64),
	}

	// Get total counts and avg score
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(AVG(score), 0), MIN(created_at), MAX(created_at)
		FROM evaluations
	`).Scan(&stats.TotalEvaluations, &stats.AvgScore, &stats.OldestEvaluation, &stats.NewestEvaluation)
	if err != nil {
		return nil, err
	}

	// Get benchmark counts
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM benchmarks").Scan(&stats.TotalBenchmarks)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM benchmark_runs").Scan(&stats.TotalBenchmarkRuns)

	// Get counts by type
	rows, err := s.db.QueryContext(ctx, "SELECT type, COUNT(*) FROM evaluations GROUP BY type")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var evalType EvaluationType
		var count int64
		if err := rows.Scan(&evalType, &count); err != nil {
			return nil, err
		}
		stats.EvaluationsByType[evalType] = count
	}

	// Get counts by agent
	rows, err = s.db.QueryContext(ctx, "SELECT agent_id, COUNT(*) FROM evaluations GROUP BY agent_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var agentID string
		var count int64
		if err := rows.Scan(&agentID, &count); err != nil {
			return nil, err
		}
		stats.EvaluationsByAgent[agentID] = count
	}

	return stats, nil
}

// Cleanup removes evaluations older than the specified duration
func (s *PostgresEvaluationStore) Cleanup(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, "DELETE FROM evaluations WHERE created_at < $1", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Ensure PostgresEvaluationStore implements EvaluationStore
var _ EvaluationStore = (*PostgresEvaluationStore)(nil)
