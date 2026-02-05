package tracing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// PostgresTraceStore implements TraceStore using PostgreSQL
type PostgresTraceStore struct {
	db *sql.DB
}

// NewPostgresTraceStore creates a new PostgreSQL trace store
func NewPostgresTraceStore(dsn string) (*PostgresTraceStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	store := &PostgresTraceStore{db: db}

	// Ensure tables exist
	if err := store.ensureTables(); err != nil {
		return nil, fmt.Errorf("failed to ensure tables: %w", err)
	}

	return store, nil
}

// NewPostgresTraceStoreFromDB creates a store from an existing database connection
func NewPostgresTraceStoreFromDB(db *sql.DB) (*PostgresTraceStore, error) {
	store := &PostgresTraceStore{db: db}
	if err := store.ensureTables(); err != nil {
		return nil, fmt.Errorf("failed to ensure tables: %w", err)
	}
	return store, nil
}

// ensureTables creates the necessary tables if they don't exist
func (s *PostgresTraceStore) ensureTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS traces (
			id VARCHAR(36) PRIMARY KEY,
			agent_id VARCHAR(255) NOT NULL,
			agent_name VARCHAR(255) NOT NULL,
			session_id VARCHAR(255),
			input TEXT NOT NULL,
			output TEXT,
			status VARCHAR(50) NOT NULL,
			error TEXT,
			start_time TIMESTAMP WITH TIME ZONE NOT NULL,
			end_time TIMESTAMP WITH TIME ZONE,
			duration_ms BIGINT NOT NULL DEFAULT 0,
			root_span_id VARCHAR(36) NOT NULL,
			total_prompt_tokens INTEGER NOT NULL DEFAULT 0,
			total_completion_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			total_cost DECIMAL(10, 6) NOT NULL DEFAULT 0,
			tool_call_count INTEGER NOT NULL DEFAULT 0,
			iteration_count INTEGER NOT NULL DEFAULT 0,
			metadata JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS spans (
			id VARCHAR(36) PRIMARY KEY,
			trace_id VARCHAR(36) NOT NULL REFERENCES traces(id) ON DELETE CASCADE,
			parent_span_id VARCHAR(36),
			type VARCHAR(50) NOT NULL,
			name VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL,
			start_time TIMESTAMP WITH TIME ZONE NOT NULL,
			end_time TIMESTAMP WITH TIME ZONE,
			duration_ms BIGINT NOT NULL DEFAULT 0,
			input JSONB,
			output JSONB,
			error JSONB,
			llm_details JSONB,
			tool_details JSONB,
			thought_details JSONB,
			events JSONB,
			attributes JSONB,
			child_span_ids JSONB
		)`,
		`CREATE INDEX IF NOT EXISTS idx_traces_agent_id ON traces(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_traces_session_id ON traces(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_traces_status ON traces(status)`,
		`CREATE INDEX IF NOT EXISTS idx_traces_start_time ON traces(start_time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_spans_trace_id ON spans(trace_id)`,
		`CREATE INDEX IF NOT EXISTS idx_spans_type ON spans(type)`,
		`CREATE INDEX IF NOT EXISTS idx_spans_parent_span_id ON spans(parent_span_id)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

// SaveTrace persists a complete trace
func (s *PostgresTraceStore) SaveTrace(ctx context.Context, trace *Trace) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	metadataJSON, _ := json.Marshal(trace.Metadata)

	// Insert or update trace
	_, err = tx.ExecContext(ctx, `
		INSERT INTO traces (
			id, agent_id, agent_name, session_id, input, output, status, error,
			start_time, end_time, duration_ms, root_span_id,
			total_prompt_tokens, total_completion_tokens, total_tokens, total_cost,
			tool_call_count, iteration_count, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
		)
		ON CONFLICT (id) DO UPDATE SET
			output = EXCLUDED.output,
			status = EXCLUDED.status,
			error = EXCLUDED.error,
			end_time = EXCLUDED.end_time,
			duration_ms = EXCLUDED.duration_ms,
			total_prompt_tokens = EXCLUDED.total_prompt_tokens,
			total_completion_tokens = EXCLUDED.total_completion_tokens,
			total_tokens = EXCLUDED.total_tokens,
			total_cost = EXCLUDED.total_cost,
			tool_call_count = EXCLUDED.tool_call_count,
			iteration_count = EXCLUDED.iteration_count,
			metadata = EXCLUDED.metadata
	`,
		trace.ID,
		trace.AgentID,
		trace.AgentName,
		nullString(trace.SessionID),
		trace.Input,
		nullString(trace.Output),
		trace.Status,
		nullString(trace.Error),
		trace.StartTime,
		nullTime(trace.EndTime),
		trace.Duration,
		trace.RootSpanID,
		trace.TotalTokens.PromptTokens,
		trace.TotalTokens.CompletionTokens,
		trace.TotalTokens.TotalTokens,
		trace.TotalCost,
		trace.ToolCallCount,
		trace.IterationCount,
		metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to insert trace: %w", err)
	}

	// Delete existing spans (for update case)
	_, err = tx.ExecContext(ctx, "DELETE FROM spans WHERE trace_id = $1", trace.ID)
	if err != nil {
		return fmt.Errorf("failed to delete existing spans: %w", err)
	}

	// Insert spans
	for _, span := range trace.Spans {
		inputJSON, _ := json.Marshal(span.Input)
		outputJSON, _ := json.Marshal(span.Output)
		errorJSON, _ := json.Marshal(span.Error)
		llmJSON, _ := json.Marshal(span.LLMDetails)
		toolJSON, _ := json.Marshal(span.ToolDetails)
		thoughtJSON, _ := json.Marshal(span.ThoughtDetails)
		eventsJSON, _ := json.Marshal(span.Events)
		attrsJSON, _ := json.Marshal(span.Attributes)
		childIdsJSON, _ := json.Marshal(span.ChildSpanIDs)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO spans (
				id, trace_id, parent_span_id, type, name, status,
				start_time, end_time, duration_ms,
				input, output, error,
				llm_details, tool_details, thought_details,
				events, attributes, child_span_ids
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
			)
		`,
			span.ID,
			trace.ID,
			nullString(string(span.ParentSpanID)),
			span.Type,
			span.Name,
			span.Status,
			span.StartTime,
			nullTime(span.EndTime),
			span.Duration,
			inputJSON,
			outputJSON,
			errorJSON,
			llmJSON,
			toolJSON,
			thoughtJSON,
			eventsJSON,
			attrsJSON,
			childIdsJSON,
		)
		if err != nil {
			return fmt.Errorf("failed to insert span: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetTrace retrieves a trace by ID
func (s *PostgresTraceStore) GetTrace(ctx context.Context, traceID TraceID) (*Trace, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			id, agent_id, agent_name, session_id, input, output, status, error,
			start_time, end_time, duration_ms, root_span_id,
			total_prompt_tokens, total_completion_tokens, total_tokens, total_cost,
			tool_call_count, iteration_count, metadata
		FROM traces
		WHERE id = $1
	`, traceID)

	trace, err := s.scanTrace(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("trace not found: %s", traceID)
		}
		return nil, fmt.Errorf("failed to get trace: %w", err)
	}

	// Get spans
	spans, err := s.getSpansForTrace(ctx, traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get spans: %w", err)
	}
	trace.Spans = spans

	return trace, nil
}

// GetTracesByAgent retrieves traces for a specific agent
func (s *PostgresTraceStore) GetTracesByAgent(ctx context.Context, agentID string, limit, offset int) ([]*Trace, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id, agent_id, agent_name, session_id, input, output, status, error,
			start_time, end_time, duration_ms, root_span_id,
			total_prompt_tokens, total_completion_tokens, total_tokens, total_cost,
			tool_call_count, iteration_count, metadata
		FROM traces
		WHERE agent_id = $1
		ORDER BY start_time DESC
		LIMIT $2 OFFSET $3
	`, agentID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query traces: %w", err)
	}
	defer rows.Close()

	return s.scanTracesWithSpans(ctx, rows)
}

// GetTracesBySession retrieves traces for a specific session
func (s *PostgresTraceStore) GetTracesBySession(ctx context.Context, sessionID string, limit, offset int) ([]*Trace, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id, agent_id, agent_name, session_id, input, output, status, error,
			start_time, end_time, duration_ms, root_span_id,
			total_prompt_tokens, total_completion_tokens, total_tokens, total_cost,
			tool_call_count, iteration_count, metadata
		FROM traces
		WHERE session_id = $1
		ORDER BY start_time DESC
		LIMIT $2 OFFSET $3
	`, sessionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query traces: %w", err)
	}
	defer rows.Close()

	return s.scanTracesWithSpans(ctx, rows)
}

// QueryTraces queries traces with filters
func (s *PostgresTraceStore) QueryTraces(ctx context.Context, query *TraceQuery) (*TraceQueryResult, error) {
	// Build WHERE clause
	var conditions []string
	var args []interface{}
	argNum := 1

	if query.Filter.AgentID != "" {
		conditions = append(conditions, fmt.Sprintf("agent_id = $%d", argNum))
		args = append(args, query.Filter.AgentID)
		argNum++
	}
	if query.Filter.SessionID != "" {
		conditions = append(conditions, fmt.Sprintf("session_id = $%d", argNum))
		args = append(args, query.Filter.SessionID)
		argNum++
	}
	if query.Filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argNum))
		args = append(args, query.Filter.Status)
		argNum++
	}
	if query.Filter.MinDuration > 0 {
		conditions = append(conditions, fmt.Sprintf("duration_ms >= $%d", argNum))
		args = append(args, query.Filter.MinDuration)
		argNum++
	}
	if query.Filter.MaxDuration > 0 {
		conditions = append(conditions, fmt.Sprintf("duration_ms <= $%d", argNum))
		args = append(args, query.Filter.MaxDuration)
		argNum++
	}
	if query.Filter.StartTimeFrom != nil {
		conditions = append(conditions, fmt.Sprintf("start_time >= $%d", argNum))
		args = append(args, query.Filter.StartTimeFrom)
		argNum++
	}
	if query.Filter.StartTimeTo != nil {
		conditions = append(conditions, fmt.Sprintf("start_time <= $%d", argNum))
		args = append(args, query.Filter.StartTimeTo)
		argNum++
	}
	if query.Filter.HasError != nil {
		if *query.Filter.HasError {
			conditions = append(conditions, "status = 'error'")
		} else {
			conditions = append(conditions, "status != 'error'")
		}
	}
	if query.Filter.MinTokens > 0 {
		conditions = append(conditions, fmt.Sprintf("total_tokens >= $%d", argNum))
		args = append(args, query.Filter.MinTokens)
		argNum++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Get total count
	var totalCount int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM traces %s", whereClause)
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count traces: %w", err)
	}

	// Build ORDER BY clause
	orderBy := "start_time"
	if query.OrderBy != "" {
		orderBy = query.OrderBy
	}
	orderDir := "DESC"
	if !query.OrderDesc {
		orderDir = "ASC"
	}

	// Add pagination
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	args = append(args, limit, offset)

	selectQuery := fmt.Sprintf(`
		SELECT
			id, agent_id, agent_name, session_id, input, output, status, error,
			start_time, end_time, duration_ms, root_span_id,
			total_prompt_tokens, total_completion_tokens, total_tokens, total_cost,
			tool_call_count, iteration_count, metadata
		FROM traces
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, orderDir, argNum, argNum+1)

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query traces: %w", err)
	}
	defer rows.Close()

	traces, err := s.scanTracesWithSpans(ctx, rows)
	if err != nil {
		return nil, err
	}

	return &TraceQueryResult{
		Traces:     traces,
		TotalCount: totalCount,
		HasMore:    int64(offset+len(traces)) < totalCount,
	}, nil
}

// DeleteTrace deletes a trace by ID
func (s *PostgresTraceStore) DeleteTrace(ctx context.Context, traceID TraceID) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM traces WHERE id = $1", traceID)
	if err != nil {
		return fmt.Errorf("failed to delete trace: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("trace not found: %s", traceID)
	}

	return nil
}

// DeleteTracesBefore deletes traces older than a given time
func (s *PostgresTraceStore) DeleteTracesBefore(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM traces WHERE start_time < $1", before)
	if err != nil {
		return 0, fmt.Errorf("failed to delete traces: %w", err)
	}

	return result.RowsAffected()
}

// GetTraceSummaries retrieves trace summaries for listing
func (s *PostgresTraceStore) GetTraceSummaries(ctx context.Context, limit, offset int) ([]*TraceSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id, agent_id, agent_name, input, output, status, start_time, duration_ms,
			total_tokens, total_cost, tool_call_count, iteration_count
		FROM traces
		ORDER BY start_time DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query traces: %w", err)
	}
	defer rows.Close()

	var summaries []*TraceSummary
	for rows.Next() {
		var summary TraceSummary
		var input, output sql.NullString
		var id string

		err := rows.Scan(
			&id,
			&summary.AgentID,
			&summary.AgentName,
			&input,
			&output,
			&summary.Status,
			&summary.StartTime,
			&summary.Duration,
			&summary.TotalTokens,
			&summary.TotalCost,
			&summary.ToolCallCount,
			&summary.IterationCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan summary: %w", err)
		}

		summary.ID = TraceID(id)
		if input.Valid {
			summary.Input = input.String
			if len(summary.Input) > 200 {
				summary.Input = summary.Input[:200] + "..."
			}
		}
		if output.Valid {
			summary.Output = output.String
			if len(summary.Output) > 200 {
				summary.Output = summary.Output[:200] + "..."
			}
		}
		summary.HasError = summary.Status == SpanStatusError

		summaries = append(summaries, &summary)
	}

	return summaries, nil
}

// GetSpan retrieves a specific span
func (s *PostgresTraceStore) GetSpan(ctx context.Context, traceID TraceID, spanID SpanID) (*Span, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			id, trace_id, parent_span_id, type, name, status,
			start_time, end_time, duration_ms,
			input, output, error,
			llm_details, tool_details, thought_details,
			events, attributes, child_span_ids
		FROM spans
		WHERE trace_id = $1 AND id = $2
	`, traceID, spanID)

	span, err := s.scanSpan(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("span not found: %s", spanID)
		}
		return nil, fmt.Errorf("failed to get span: %w", err)
	}

	return span, nil
}

// Stats returns storage statistics
func (s *PostgresTraceStore) Stats(ctx context.Context) (*TraceStoreStats, error) {
	stats := &TraceStoreStats{
		TracesByAgent:  make(map[string]int64),
		TracesByStatus: make(map[string]int64),
	}

	// Get totals
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(total_cost), 0),
			COALESCE(AVG(duration_ms), 0),
			MIN(start_time),
			MAX(start_time)
		FROM traces
	`).Scan(
		&stats.TotalTraces,
		&stats.TotalTokens,
		&stats.TotalCost,
		&stats.AvgDuration,
		&stats.OldestTrace,
		&stats.NewestTrace,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// Get span count
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM spans").Scan(&stats.TotalSpans)
	if err != nil {
		return nil, fmt.Errorf("failed to get span count: %w", err)
	}

	// Get traces by agent
	rows, err := s.db.QueryContext(ctx, "SELECT agent_id, COUNT(*) FROM traces GROUP BY agent_id")
	if err != nil {
		return nil, fmt.Errorf("failed to get traces by agent: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var agentID string
		var count int64
		if err := rows.Scan(&agentID, &count); err != nil {
			return nil, fmt.Errorf("failed to scan agent count: %w", err)
		}
		stats.TracesByAgent[agentID] = count
	}

	// Get traces by status
	rows, err = s.db.QueryContext(ctx, "SELECT status, COUNT(*) FROM traces GROUP BY status")
	if err != nil {
		return nil, fmt.Errorf("failed to get traces by status: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan status count: %w", err)
		}
		stats.TracesByStatus[status] = count
	}

	if stats.TotalTraces > 0 {
		stats.AvgTokensPerTrace = float64(stats.TotalTokens) / float64(stats.TotalTraces)
	}

	return stats, nil
}

// Close closes the database connection
func (s *PostgresTraceStore) Close() error {
	return s.db.Close()
}

// Helper methods

func (s *PostgresTraceStore) scanTrace(row *sql.Row) (*Trace, error) {
	var trace Trace
	var sessionID, output, errStr sql.NullString
	var endTime sql.NullTime
	var metadataJSON []byte
	var id, rootSpanID string

	err := row.Scan(
		&id,
		&trace.AgentID,
		&trace.AgentName,
		&sessionID,
		&trace.Input,
		&output,
		&trace.Status,
		&errStr,
		&trace.StartTime,
		&endTime,
		&trace.Duration,
		&rootSpanID,
		&trace.TotalTokens.PromptTokens,
		&trace.TotalTokens.CompletionTokens,
		&trace.TotalTokens.TotalTokens,
		&trace.TotalCost,
		&trace.ToolCallCount,
		&trace.IterationCount,
		&metadataJSON,
	)
	if err != nil {
		return nil, err
	}

	trace.ID = TraceID(id)
	trace.RootSpanID = SpanID(rootSpanID)
	if sessionID.Valid {
		trace.SessionID = sessionID.String
	}
	if output.Valid {
		trace.Output = output.String
	}
	if errStr.Valid {
		trace.Error = errStr.String
	}
	if endTime.Valid {
		trace.EndTime = &endTime.Time
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &trace.Metadata)
	}

	return &trace, nil
}

func (s *PostgresTraceStore) scanSpan(row *sql.Row) (*Span, error) {
	var span Span
	var parentSpanID sql.NullString
	var endTime sql.NullTime
	var inputJSON, outputJSON, errorJSON, llmJSON, toolJSON, thoughtJSON, eventsJSON, attrsJSON, childIdsJSON []byte
	var id, traceID string

	err := row.Scan(
		&id,
		&traceID,
		&parentSpanID,
		&span.Type,
		&span.Name,
		&span.Status,
		&span.StartTime,
		&endTime,
		&span.Duration,
		&inputJSON,
		&outputJSON,
		&errorJSON,
		&llmJSON,
		&toolJSON,
		&thoughtJSON,
		&eventsJSON,
		&attrsJSON,
		&childIdsJSON,
	)
	if err != nil {
		return nil, err
	}

	span.ID = SpanID(id)
	span.TraceID = TraceID(traceID)
	if parentSpanID.Valid {
		span.ParentSpanID = SpanID(parentSpanID.String)
	}
	if endTime.Valid {
		span.EndTime = &endTime.Time
	}

	if len(inputJSON) > 0 {
		json.Unmarshal(inputJSON, &span.Input)
	}
	if len(outputJSON) > 0 {
		json.Unmarshal(outputJSON, &span.Output)
	}
	if len(errorJSON) > 0 {
		json.Unmarshal(errorJSON, &span.Error)
	}
	if len(llmJSON) > 0 {
		json.Unmarshal(llmJSON, &span.LLMDetails)
	}
	if len(toolJSON) > 0 {
		json.Unmarshal(toolJSON, &span.ToolDetails)
	}
	if len(thoughtJSON) > 0 {
		json.Unmarshal(thoughtJSON, &span.ThoughtDetails)
	}
	if len(eventsJSON) > 0 {
		json.Unmarshal(eventsJSON, &span.Events)
	}
	if len(attrsJSON) > 0 {
		json.Unmarshal(attrsJSON, &span.Attributes)
	}
	if len(childIdsJSON) > 0 {
		json.Unmarshal(childIdsJSON, &span.ChildSpanIDs)
	}

	return &span, nil
}

func (s *PostgresTraceStore) getSpansForTrace(ctx context.Context, traceID TraceID) ([]*Span, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id, trace_id, parent_span_id, type, name, status,
			start_time, end_time, duration_ms,
			input, output, error,
			llm_details, tool_details, thought_details,
			events, attributes, child_span_ids
		FROM spans
		WHERE trace_id = $1
		ORDER BY start_time ASC
	`, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var spans []*Span
	for rows.Next() {
		var span Span
		var parentSpanID sql.NullString
		var endTime sql.NullTime
		var inputJSON, outputJSON, errorJSON, llmJSON, toolJSON, thoughtJSON, eventsJSON, attrsJSON, childIdsJSON []byte
		var id, tid string

		err := rows.Scan(
			&id,
			&tid,
			&parentSpanID,
			&span.Type,
			&span.Name,
			&span.Status,
			&span.StartTime,
			&endTime,
			&span.Duration,
			&inputJSON,
			&outputJSON,
			&errorJSON,
			&llmJSON,
			&toolJSON,
			&thoughtJSON,
			&eventsJSON,
			&attrsJSON,
			&childIdsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan span: %w", err)
		}

		span.ID = SpanID(id)
		span.TraceID = TraceID(tid)
		if parentSpanID.Valid {
			span.ParentSpanID = SpanID(parentSpanID.String)
		}
		if endTime.Valid {
			span.EndTime = &endTime.Time
		}

		if len(inputJSON) > 0 {
			json.Unmarshal(inputJSON, &span.Input)
		}
		if len(outputJSON) > 0 {
			json.Unmarshal(outputJSON, &span.Output)
		}
		if len(errorJSON) > 0 {
			json.Unmarshal(errorJSON, &span.Error)
		}
		if len(llmJSON) > 0 {
			json.Unmarshal(llmJSON, &span.LLMDetails)
		}
		if len(toolJSON) > 0 {
			json.Unmarshal(toolJSON, &span.ToolDetails)
		}
		if len(thoughtJSON) > 0 {
			json.Unmarshal(thoughtJSON, &span.ThoughtDetails)
		}
		if len(eventsJSON) > 0 {
			json.Unmarshal(eventsJSON, &span.Events)
		}
		if len(attrsJSON) > 0 {
			json.Unmarshal(attrsJSON, &span.Attributes)
		}
		if len(childIdsJSON) > 0 {
			json.Unmarshal(childIdsJSON, &span.ChildSpanIDs)
		}

		spans = append(spans, &span)
	}

	return spans, nil
}

func (s *PostgresTraceStore) scanTracesWithSpans(ctx context.Context, rows *sql.Rows) ([]*Trace, error) {
	var traces []*Trace

	for rows.Next() {
		var trace Trace
		var sessionID, output, errStr sql.NullString
		var endTime sql.NullTime
		var metadataJSON []byte
		var id, rootSpanID string

		err := rows.Scan(
			&id,
			&trace.AgentID,
			&trace.AgentName,
			&sessionID,
			&trace.Input,
			&output,
			&trace.Status,
			&errStr,
			&trace.StartTime,
			&endTime,
			&trace.Duration,
			&rootSpanID,
			&trace.TotalTokens.PromptTokens,
			&trace.TotalTokens.CompletionTokens,
			&trace.TotalTokens.TotalTokens,
			&trace.TotalCost,
			&trace.ToolCallCount,
			&trace.IterationCount,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trace: %w", err)
		}

		trace.ID = TraceID(id)
		trace.RootSpanID = SpanID(rootSpanID)
		if sessionID.Valid {
			trace.SessionID = sessionID.String
		}
		if output.Valid {
			trace.Output = output.String
		}
		if errStr.Valid {
			trace.Error = errStr.String
		}
		if endTime.Valid {
			trace.EndTime = &endTime.Time
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &trace.Metadata)
		}

		// Get spans for this trace
		spans, err := s.getSpansForTrace(ctx, trace.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get spans: %w", err)
		}
		trace.Spans = spans

		traces = append(traces, &trace)
	}

	return traces, nil
}

// Helper functions for null handling

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// Ensure PostgresTraceStore implements TraceStore
var _ TraceStore = (*PostgresTraceStore)(nil)
