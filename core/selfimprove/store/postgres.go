// Package store provides implementations of the ExperienceStore interface.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Ranganaths/minion/core/selfimprove"
)

// PostgresExperienceStore implements ExperienceStore using PostgreSQL.
// This is suitable for production deployments requiring persistence and scalability.
type PostgresExperienceStore struct {
	db        *sql.DB
	tableName string
}

// PostgresConfig configures the PostgreSQL store.
type PostgresConfig struct {
	// ConnectionString is the PostgreSQL connection string
	ConnectionString string `json:"connection_string"`

	// TableName is the table name for experiences (default: "experiences")
	TableName string `json:"table_name"`

	// MaxConnections is the maximum number of connections
	MaxConnections int `json:"max_connections"`

	// ConnMaxLifetime is the maximum connection lifetime
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
}

// DefaultPostgresConfig returns default PostgreSQL configuration.
func DefaultPostgresConfig() *PostgresConfig {
	return &PostgresConfig{
		TableName:       "selfimprove_experiences",
		MaxConnections:  10,
		ConnMaxLifetime: time.Hour,
	}
}

// NewPostgresExperienceStore creates a new PostgreSQL experience store.
// Note: This requires the github.com/lib/pq driver to be imported in the calling code.
func NewPostgresExperienceStore(db *sql.DB, config *PostgresConfig) (*PostgresExperienceStore, error) {
	if db == nil {
		return nil, errors.New("database connection is required")
	}

	if config == nil {
		config = DefaultPostgresConfig()
	}

	store := &PostgresExperienceStore{
		db:        db,
		tableName: config.TableName,
	}

	// Create table if not exists
	if err := store.createTable(); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return store, nil
}

// createTable creates the experiences table if it doesn't exist.
func (s *PostgresExperienceStore) createTable() error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id VARCHAR(255) PRIMARY KEY,
			trace_id VARCHAR(255),
			agent_id VARCHAR(255) NOT NULL,
			task_type VARCHAR(255),
			input JSONB,
			output JSONB,
			system_prompt TEXT,
			user_prompt TEXT,
			success BOOLEAN NOT NULL DEFAULT false,
			score DECIMAL(5,4) NOT NULL DEFAULT 0,
			subscores JSONB,
			human_rating DECIMAL(5,4),
			human_feedback TEXT,
			correction TEXT,
			timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			tokens_used INTEGER DEFAULT 0,
			latency_ms BIGINT DEFAULT 0,
			model VARCHAR(255),
			tools_used TEXT[],
			iteration_count INTEGER DEFAULT 0,
			embedding VECTOR(1536),
			metadata JSONB,
			prompt_version VARCHAR(255),
			improvement_id VARCHAR(255),
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_%s_agent_id ON %s(agent_id);
		CREATE INDEX IF NOT EXISTS idx_%s_task_type ON %s(task_type);
		CREATE INDEX IF NOT EXISTS idx_%s_trace_id ON %s(trace_id);
		CREATE INDEX IF NOT EXISTS idx_%s_timestamp ON %s(timestamp);
		CREATE INDEX IF NOT EXISTS idx_%s_score ON %s(score);
		CREATE INDEX IF NOT EXISTS idx_%s_success ON %s(success);
	`, s.tableName,
		s.tableName, s.tableName,
		s.tableName, s.tableName,
		s.tableName, s.tableName,
		s.tableName, s.tableName,
		s.tableName, s.tableName,
		s.tableName, s.tableName)

	_, err := s.db.Exec(query)
	return err
}

// Store saves an experience to the database.
func (s *PostgresExperienceStore) Store(ctx context.Context, exp *selfimprove.Experience) error {
	if exp == nil {
		return errors.New("experience cannot be nil")
	}
	if exp.ID == "" {
		return errors.New("experience ID cannot be empty")
	}

	inputJSON, _ := json.Marshal(exp.Input)
	outputJSON, _ := json.Marshal(exp.Output)
	subscoresJSON, _ := json.Marshal(exp.Subscores)
	metadataJSON, _ := json.Marshal(exp.Metadata)

	query := fmt.Sprintf(`
		INSERT INTO %s (
			id, trace_id, agent_id, task_type, input, output,
			system_prompt, user_prompt, success, score, subscores,
			human_rating, human_feedback, correction, timestamp,
			tokens_used, latency_ms, model, tools_used, iteration_count,
			metadata, prompt_version, improvement_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23
		)
		ON CONFLICT (id) DO UPDATE SET
			trace_id = EXCLUDED.trace_id,
			input = EXCLUDED.input,
			output = EXCLUDED.output,
			success = EXCLUDED.success,
			score = EXCLUDED.score,
			subscores = EXCLUDED.subscores,
			human_rating = EXCLUDED.human_rating,
			human_feedback = EXCLUDED.human_feedback,
			correction = EXCLUDED.correction,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
	`, s.tableName)

	_, err := s.db.ExecContext(ctx, query,
		exp.ID, exp.TraceID, exp.AgentID, exp.TaskType,
		inputJSON, outputJSON, exp.SystemPrompt, exp.UserPrompt,
		exp.Success, exp.Score, subscoresJSON,
		exp.HumanRating, exp.HumanFeedback, exp.Correction,
		exp.Timestamp, exp.TokensUsed, exp.LatencyMs, exp.Model,
		pqArray(exp.ToolsUsed), exp.IterationCount,
		metadataJSON, exp.PromptVersion, exp.ImprovementID,
	)

	return err
}

// Get retrieves an experience by ID.
func (s *PostgresExperienceStore) Get(ctx context.Context, id string) (*selfimprove.Experience, error) {
	query := fmt.Sprintf(`
		SELECT id, trace_id, agent_id, task_type, input, output,
			system_prompt, user_prompt, success, score, subscores,
			human_rating, human_feedback, correction, timestamp,
			tokens_used, latency_ms, model, tools_used, iteration_count,
			metadata, prompt_version, improvement_id
		FROM %s WHERE id = $1
	`, s.tableName)

	row := s.db.QueryRowContext(ctx, query, id)
	return s.scanExperience(row)
}

// GetByTraceID retrieves an experience by its trace ID.
func (s *PostgresExperienceStore) GetByTraceID(ctx context.Context, traceID string) (*selfimprove.Experience, error) {
	query := fmt.Sprintf(`
		SELECT id, trace_id, agent_id, task_type, input, output,
			system_prompt, user_prompt, success, score, subscores,
			human_rating, human_feedback, correction, timestamp,
			tokens_used, latency_ms, model, tools_used, iteration_count,
			metadata, prompt_version, improvement_id
		FROM %s WHERE trace_id = $1
	`, s.tableName)

	row := s.db.QueryRowContext(ctx, query, traceID)
	return s.scanExperience(row)
}

// Query retrieves experiences matching the query criteria.
func (s *PostgresExperienceStore) Query(ctx context.Context, query *selfimprove.ExperienceQuery) ([]*selfimprove.Experience, error) {
	whereClause, args := s.buildWhereClause(query)

	orderBy := "timestamp"
	if query.OrderBy != "" {
		orderBy = query.OrderBy
	}
	orderDir := "ASC"
	if query.OrderDesc {
		orderDir = "DESC"
	}

	limit := 100
	if query.Limit > 0 {
		limit = query.Limit
	}

	sqlQuery := fmt.Sprintf(`
		SELECT id, trace_id, agent_id, task_type, input, output,
			system_prompt, user_prompt, success, score, subscores,
			human_rating, human_feedback, correction, timestamp,
			tokens_used, latency_ms, model, tools_used, iteration_count,
			metadata, prompt_version, improvement_id
		FROM %s
		%s
		ORDER BY %s %s
		LIMIT %d OFFSET %d
	`, s.tableName, whereClause, orderBy, orderDir, limit, query.Offset)

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanExperiences(rows)
}

// GetByAgent retrieves experiences for a specific agent.
func (s *PostgresExperienceStore) GetByAgent(ctx context.Context, agentID string, limit int) ([]*selfimprove.Experience, error) {
	if limit <= 0 {
		limit = 100
	}

	query := fmt.Sprintf(`
		SELECT id, trace_id, agent_id, task_type, input, output,
			system_prompt, user_prompt, success, score, subscores,
			human_rating, human_feedback, correction, timestamp,
			tokens_used, latency_ms, model, tools_used, iteration_count,
			metadata, prompt_version, improvement_id
		FROM %s
		WHERE agent_id = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`, s.tableName)

	rows, err := s.db.QueryContext(ctx, query, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanExperiences(rows)
}

// GetByTaskType retrieves experiences for a specific task type.
func (s *PostgresExperienceStore) GetByTaskType(ctx context.Context, taskType string, limit int) ([]*selfimprove.Experience, error) {
	if limit <= 0 {
		limit = 100
	}

	query := fmt.Sprintf(`
		SELECT id, trace_id, agent_id, task_type, input, output,
			system_prompt, user_prompt, success, score, subscores,
			human_rating, human_feedback, correction, timestamp,
			tokens_used, latency_ms, model, tools_used, iteration_count,
			metadata, prompt_version, improvement_id
		FROM %s
		WHERE task_type = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`, s.tableName)

	rows, err := s.db.QueryContext(ctx, query, taskType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanExperiences(rows)
}

// GetSuccessful retrieves successful experiences above a score threshold.
func (s *PostgresExperienceStore) GetSuccessful(ctx context.Context, minScore float64, limit int) ([]*selfimprove.Experience, error) {
	if limit <= 0 {
		limit = 100
	}

	query := fmt.Sprintf(`
		SELECT id, trace_id, agent_id, task_type, input, output,
			system_prompt, user_prompt, success, score, subscores,
			human_rating, human_feedback, correction, timestamp,
			tokens_used, latency_ms, model, tools_used, iteration_count,
			metadata, prompt_version, improvement_id
		FROM %s
		WHERE success = true AND score >= $1
		ORDER BY score DESC
		LIMIT $2
	`, s.tableName)

	rows, err := s.db.QueryContext(ctx, query, minScore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanExperiences(rows)
}

// GetFailed retrieves failed experiences below a score threshold.
func (s *PostgresExperienceStore) GetFailed(ctx context.Context, maxScore float64, limit int) ([]*selfimprove.Experience, error) {
	if limit <= 0 {
		limit = 100
	}

	query := fmt.Sprintf(`
		SELECT id, trace_id, agent_id, task_type, input, output,
			system_prompt, user_prompt, success, score, subscores,
			human_rating, human_feedback, correction, timestamp,
			tokens_used, latency_ms, model, tools_used, iteration_count,
			metadata, prompt_version, improvement_id
		FROM %s
		WHERE success = false OR score <= $1
		ORDER BY timestamp DESC
		LIMIT $2
	`, s.tableName)

	rows, err := s.db.QueryContext(ctx, query, maxScore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanExperiences(rows)
}

// FindSimilar finds experiences with similar embeddings using cosine similarity.
// Requires pgvector extension.
func (s *PostgresExperienceStore) FindSimilar(ctx context.Context, embedding []float32, limit int) ([]*selfimprove.Experience, error) {
	if len(embedding) == 0 {
		return []*selfimprove.Experience{}, nil
	}
	if limit <= 0 {
		limit = 10
	}

	// Convert embedding to string format for pgvector
	embeddingStr := formatEmbedding(embedding)

	query := fmt.Sprintf(`
		SELECT id, trace_id, agent_id, task_type, input, output,
			system_prompt, user_prompt, success, score, subscores,
			human_rating, human_feedback, correction, timestamp,
			tokens_used, latency_ms, model, tools_used, iteration_count,
			metadata, prompt_version, improvement_id
		FROM %s
		WHERE embedding IS NOT NULL
		ORDER BY embedding <-> $1
		LIMIT $2
	`, s.tableName)

	rows, err := s.db.QueryContext(ctx, query, embeddingStr, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanExperiences(rows)
}

// GetStats returns aggregated statistics for an agent.
func (s *PostgresExperienceStore) GetStats(ctx context.Context, agentID string) (*selfimprove.ExperienceStats, error) {
	query := fmt.Sprintf(`
		SELECT
			COUNT(*) as total_count,
			COALESCE(SUM(CASE WHEN success THEN 1 ELSE 0 END), 0) as success_count,
			COALESCE(SUM(CASE WHEN NOT success THEN 1 ELSE 0 END), 0) as failure_count,
			COALESCE(AVG(score), 0) as avg_score,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms,
			COALESCE(AVG(tokens_used), 0) as avg_tokens_used,
			MIN(timestamp) as first_experience,
			MAX(timestamp) as last_experience
		FROM %s
		WHERE agent_id = $1
	`, s.tableName)

	var stats selfimprove.ExperienceStats
	var firstExp, lastExp sql.NullTime

	err := s.db.QueryRowContext(ctx, query, agentID).Scan(
		&stats.TotalCount,
		&stats.SuccessCount,
		&stats.FailureCount,
		&stats.AvgScore,
		&stats.AvgLatencyMs,
		&stats.AvgTokensUsed,
		&firstExp,
		&lastExp,
	)
	if err != nil {
		return nil, err
	}

	if stats.TotalCount > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalCount)
	}

	if firstExp.Valid {
		stats.FirstExperience = &firstExp.Time
	}
	if lastExp.Valid {
		stats.LastExperience = &lastExp.Time
	}

	// Get scores by task type
	stats.ScoresByTaskType = make(map[string]float64)
	stats.CountByTaskType = make(map[string]int)

	taskQuery := fmt.Sprintf(`
		SELECT task_type, AVG(score), COUNT(*)
		FROM %s
		WHERE agent_id = $1 AND task_type IS NOT NULL
		GROUP BY task_type
	`, s.tableName)

	rows, err := s.db.QueryContext(ctx, taskQuery, agentID)
	if err != nil {
		return &stats, nil // Return partial stats
	}
	defer rows.Close()

	for rows.Next() {
		var taskType string
		var avgScore float64
		var count int
		if err := rows.Scan(&taskType, &avgScore, &count); err == nil {
			stats.ScoresByTaskType[taskType] = avgScore
			stats.CountByTaskType[taskType] = count
		}
	}

	// Calculate trend
	stats.RecentTrend = s.calculateTrend(ctx, agentID)

	return &stats, nil
}

// GetGlobalStats returns aggregated statistics across all agents.
func (s *PostgresExperienceStore) GetGlobalStats(ctx context.Context) (*selfimprove.ExperienceStats, error) {
	query := fmt.Sprintf(`
		SELECT
			COUNT(*) as total_count,
			COALESCE(SUM(CASE WHEN success THEN 1 ELSE 0 END), 0) as success_count,
			COALESCE(SUM(CASE WHEN NOT success THEN 1 ELSE 0 END), 0) as failure_count,
			COALESCE(AVG(score), 0) as avg_score,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms,
			COALESCE(AVG(tokens_used), 0) as avg_tokens_used,
			MIN(timestamp) as first_experience,
			MAX(timestamp) as last_experience
		FROM %s
	`, s.tableName)

	var stats selfimprove.ExperienceStats
	var firstExp, lastExp sql.NullTime

	err := s.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalCount,
		&stats.SuccessCount,
		&stats.FailureCount,
		&stats.AvgScore,
		&stats.AvgLatencyMs,
		&stats.AvgTokensUsed,
		&firstExp,
		&lastExp,
	)
	if err != nil {
		return nil, err
	}

	if stats.TotalCount > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalCount)
	}

	if firstExp.Valid {
		stats.FirstExperience = &firstExp.Time
	}
	if lastExp.Valid {
		stats.LastExperience = &lastExp.Time
	}

	return &stats, nil
}

// Update updates an existing experience.
func (s *PostgresExperienceStore) Update(ctx context.Context, exp *selfimprove.Experience) error {
	return s.Store(ctx, exp) // Upsert handles this
}

// Delete removes an experience.
func (s *PostgresExperienceStore) Delete(ctx context.Context, id string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", s.tableName)
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

// Prune removes old experiences while keeping top performers.
func (s *PostgresExperienceStore) Prune(ctx context.Context, olderThan time.Time, keepTopN int) (int, error) {
	// First, identify IDs to keep (top N by score)
	keepQuery := fmt.Sprintf(`
		SELECT id FROM %s
		ORDER BY score DESC
		LIMIT $1
	`, s.tableName)

	rows, err := s.db.QueryContext(ctx, keepQuery, keepTopN)
	if err != nil {
		return 0, err
	}

	keepIDs := make([]string, 0, keepTopN)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			keepIDs = append(keepIDs, id)
		}
	}
	rows.Close()

	// Delete old experiences not in the keep list
	deleteQuery := fmt.Sprintf(`
		DELETE FROM %s
		WHERE timestamp < $1
		AND id NOT IN (%s)
	`, s.tableName, placeholders(keepIDs))

	args := make([]interface{}, 0, len(keepIDs)+1)
	args = append(args, olderThan)
	for _, id := range keepIDs {
		args = append(args, id)
	}

	result, err := s.db.ExecContext(ctx, deleteQuery, args...)
	if err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// Count returns the total number of experiences.
func (s *PostgresExperienceStore) Count(ctx context.Context) (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", s.tableName)
	var count int
	err := s.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

// Helper methods

func (s *PostgresExperienceStore) buildWhereClause(query *selfimprove.ExperienceQuery) (string, []interface{}) {
	if query == nil {
		return "", nil
	}

	var conditions []string
	var args []interface{}
	argNum := 1

	if query.AgentID != "" {
		conditions = append(conditions, fmt.Sprintf("agent_id = $%d", argNum))
		args = append(args, query.AgentID)
		argNum++
	}

	if query.TaskType != "" {
		conditions = append(conditions, fmt.Sprintf("task_type = $%d", argNum))
		args = append(args, query.TaskType)
		argNum++
	}

	if query.MinScore != nil {
		conditions = append(conditions, fmt.Sprintf("score >= $%d", argNum))
		args = append(args, *query.MinScore)
		argNum++
	}

	if query.MaxScore != nil {
		conditions = append(conditions, fmt.Sprintf("score <= $%d", argNum))
		args = append(args, *query.MaxScore)
		argNum++
	}

	if query.Success != nil {
		conditions = append(conditions, fmt.Sprintf("success = $%d", argNum))
		args = append(args, *query.Success)
		argNum++
	}

	if query.Since != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argNum))
		args = append(args, *query.Since)
		argNum++
	}

	if query.Until != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", argNum))
		args = append(args, *query.Until)
		argNum++
	}

	if query.PromptVersion != "" {
		conditions = append(conditions, fmt.Sprintf("prompt_version = $%d", argNum))
		args = append(args, query.PromptVersion)
		argNum++
	}

	if query.HasHumanFeedback != nil && *query.HasHumanFeedback {
		conditions = append(conditions, "(human_rating IS NOT NULL OR human_feedback IS NOT NULL OR correction IS NOT NULL)")
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return "WHERE " + strings.Join(conditions, " AND "), args
}

func (s *PostgresExperienceStore) scanExperience(row *sql.Row) (*selfimprove.Experience, error) {
	var exp selfimprove.Experience
	var inputJSON, outputJSON, subscoresJSON, metadataJSON []byte
	var toolsUsed []string
	var traceID, taskType, systemPrompt, userPrompt, model, promptVersion sql.NullString
	var humanRating sql.NullFloat64
	var humanFeedback, correction, improvementID sql.NullString

	err := row.Scan(
		&exp.ID, &traceID, &exp.AgentID, &taskType,
		&inputJSON, &outputJSON, &systemPrompt, &userPrompt,
		&exp.Success, &exp.Score, &subscoresJSON,
		&humanRating, &humanFeedback, &correction,
		&exp.Timestamp, &exp.TokensUsed, &exp.LatencyMs, &model,
		pqArray(&toolsUsed), &exp.IterationCount,
		&metadataJSON, &promptVersion, &improvementID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("experience not found")
		}
		return nil, err
	}

	// Handle nullable fields
	if traceID.Valid {
		exp.TraceID = traceID.String
	}
	if taskType.Valid {
		exp.TaskType = taskType.String
	}
	if systemPrompt.Valid {
		exp.SystemPrompt = systemPrompt.String
	}
	if userPrompt.Valid {
		exp.UserPrompt = userPrompt.String
	}
	if model.Valid {
		exp.Model = model.String
	}
	if promptVersion.Valid {
		exp.PromptVersion = promptVersion.String
	}
	if humanRating.Valid {
		exp.HumanRating = &humanRating.Float64
	}
	if humanFeedback.Valid {
		exp.HumanFeedback = &humanFeedback.String
	}
	if correction.Valid {
		exp.Correction = &correction.String
	}
	if improvementID.Valid {
		exp.ImprovementID = &improvementID.String
	}

	// Unmarshal JSON fields
	if len(inputJSON) > 0 {
		json.Unmarshal(inputJSON, &exp.Input)
	}
	if len(outputJSON) > 0 {
		json.Unmarshal(outputJSON, &exp.Output)
	}
	if len(subscoresJSON) > 0 {
		json.Unmarshal(subscoresJSON, &exp.Subscores)
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &exp.Metadata)
	}

	exp.ToolsUsed = toolsUsed

	return &exp, nil
}

func (s *PostgresExperienceStore) scanExperiences(rows *sql.Rows) ([]*selfimprove.Experience, error) {
	var experiences []*selfimprove.Experience

	for rows.Next() {
		var exp selfimprove.Experience
		var inputJSON, outputJSON, subscoresJSON, metadataJSON []byte
		var toolsUsed []string
		var traceID, taskType, systemPrompt, userPrompt, model, promptVersion sql.NullString
		var humanRating sql.NullFloat64
		var humanFeedback, correction, improvementID sql.NullString

		err := rows.Scan(
			&exp.ID, &traceID, &exp.AgentID, &taskType,
			&inputJSON, &outputJSON, &systemPrompt, &userPrompt,
			&exp.Success, &exp.Score, &subscoresJSON,
			&humanRating, &humanFeedback, &correction,
			&exp.Timestamp, &exp.TokensUsed, &exp.LatencyMs, &model,
			pqArray(&toolsUsed), &exp.IterationCount,
			&metadataJSON, &promptVersion, &improvementID,
		)
		if err != nil {
			continue
		}

		// Handle nullable fields
		if traceID.Valid {
			exp.TraceID = traceID.String
		}
		if taskType.Valid {
			exp.TaskType = taskType.String
		}
		if systemPrompt.Valid {
			exp.SystemPrompt = systemPrompt.String
		}
		if userPrompt.Valid {
			exp.UserPrompt = userPrompt.String
		}
		if model.Valid {
			exp.Model = model.String
		}
		if promptVersion.Valid {
			exp.PromptVersion = promptVersion.String
		}
		if humanRating.Valid {
			exp.HumanRating = &humanRating.Float64
		}
		if humanFeedback.Valid {
			exp.HumanFeedback = &humanFeedback.String
		}
		if correction.Valid {
			exp.Correction = &correction.String
		}
		if improvementID.Valid {
			exp.ImprovementID = &improvementID.String
		}

		// Unmarshal JSON fields
		if len(inputJSON) > 0 {
			json.Unmarshal(inputJSON, &exp.Input)
		}
		if len(outputJSON) > 0 {
			json.Unmarshal(outputJSON, &exp.Output)
		}
		if len(subscoresJSON) > 0 {
			json.Unmarshal(subscoresJSON, &exp.Subscores)
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &exp.Metadata)
		}

		exp.ToolsUsed = toolsUsed
		experiences = append(experiences, &exp)
	}

	return experiences, nil
}

func (s *PostgresExperienceStore) calculateTrend(ctx context.Context, agentID string) selfimprove.TrendDirection {
	query := fmt.Sprintf(`
		WITH recent AS (
			SELECT score, timestamp
			FROM %s
			WHERE agent_id = $1
			ORDER BY timestamp DESC
			LIMIT 100
		),
		halves AS (
			SELECT
				AVG(CASE WHEN rn <= cnt/2 THEN score END) as recent_avg,
				AVG(CASE WHEN rn > cnt/2 THEN score END) as older_avg
			FROM (
				SELECT score, ROW_NUMBER() OVER (ORDER BY timestamp DESC) as rn,
					COUNT(*) OVER () as cnt
				FROM recent
			) t
		)
		SELECT recent_avg, older_avg FROM halves
	`, s.tableName)

	var recentAvg, olderAvg sql.NullFloat64
	err := s.db.QueryRowContext(ctx, query, agentID).Scan(&recentAvg, &olderAvg)
	if err != nil || !recentAvg.Valid || !olderAvg.Valid {
		return selfimprove.TrendUnknown
	}

	diff := recentAvg.Float64 - olderAvg.Float64
	if diff > 0.05 {
		return selfimprove.TrendImproving
	} else if diff < -0.05 {
		return selfimprove.TrendDeclining
	}
	return selfimprove.TrendStable
}

// Helper functions

func formatEmbedding(embedding []float32) string {
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func placeholders(ids []string) string {
	if len(ids) == 0 {
		return "''"
	}
	parts := make([]string, len(ids))
	for i := range ids {
		parts[i] = fmt.Sprintf("$%d", i+2) // $1 is olderThan
	}
	return strings.Join(parts, ",")
}

// pqArray is a helper for PostgreSQL arrays.
// This is a simplified version - in production, use github.com/lib/pq.Array
type pqArrayType []string

func pqArray(a interface{}) interface{} {
	switch v := a.(type) {
	case []string:
		return pqArrayType(v)
	case *[]string:
		return (*pqArrayType)(v)
	default:
		return a
	}
}

func (a pqArrayType) Value() (interface{}, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	return "{" + strings.Join(a, ",") + "}", nil
}

func (a *pqArrayType) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}

	switch v := src.(type) {
	case []byte:
		return a.scanString(string(v))
	case string:
		return a.scanString(v)
	default:
		return fmt.Errorf("unsupported type: %T", src)
	}
}

func (a *pqArrayType) scanString(s string) error {
	s = strings.Trim(s, "{}")
	if s == "" {
		*a = nil
		return nil
	}
	*a = strings.Split(s, ",")
	return nil
}
