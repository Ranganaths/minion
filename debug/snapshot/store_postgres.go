// Package snapshot provides a PostgreSQL implementation of SnapshotStore.
package snapshot

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// PostgresSnapshotStore is a PostgreSQL-backed implementation of SnapshotStore.
// Suitable for production deployments requiring persistence and durability.
type PostgresSnapshotStore struct {
	db *sql.DB

	// Prepared statements
	stmtInsert             *sql.Stmt
	stmtGet                *sql.Stmt
	stmtGetByExecution     *sql.Stmt
	stmtGetByExecutionRange *sql.Stmt
	stmtGetLatest          *sql.Stmt
	stmtGetAtSequence      *sql.Stmt
	stmtGetByCheckpoint    *sql.Stmt
	stmtPurgeOld           *sql.Stmt
	stmtPurgeExecution     *sql.Stmt

	// Configuration
	batchSize       int
	retentionPeriod time.Duration
}

// PostgresConfig holds configuration for PostgreSQL connection.
type PostgresConfig struct {
	Host            string
	Port            int
	Database        string
	User            string
	Password        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	BatchSize       int
	RetentionPeriod time.Duration
}

// DefaultPostgresConfig returns a default PostgreSQL configuration.
func DefaultPostgresConfig() PostgresConfig {
	return PostgresConfig{
		Host:            "localhost",
		Port:            5432,
		Database:        "minion_debug",
		User:            "minion",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		BatchSize:       100,
		RetentionPeriod: 7 * 24 * time.Hour,
	}
}

// NewPostgresSnapshotStore creates a new PostgreSQL-backed snapshot store.
func NewPostgresSnapshotStore(config PostgresConfig) (*PostgresSnapshotStore, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		config.Host, config.Port, config.Database, config.User, config.Password, config.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &PostgresSnapshotStore{
		db:              db,
		batchSize:       config.BatchSize,
		retentionPeriod: config.RetentionPeriod,
	}

	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	if err := store.prepareStatements(); err != nil {
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	return store, nil
}

// NewPostgresSnapshotStoreFromDB creates a store from an existing database connection.
func NewPostgresSnapshotStoreFromDB(db *sql.DB) (*PostgresSnapshotStore, error) {
	store := &PostgresSnapshotStore{
		db:              db,
		batchSize:       100,
		retentionPeriod: 7 * 24 * time.Hour,
	}

	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	if err := store.prepareStatements(); err != nil {
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	return store, nil
}

func (s *PostgresSnapshotStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS execution_snapshots (
		id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		execution_id    VARCHAR(255) NOT NULL,
		sequence_num    BIGINT NOT NULL,
		timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		checkpoint_type VARCHAR(50) NOT NULL,
		agent_id        VARCHAR(255),
		task_id         VARCHAR(255),
		worker_id       VARCHAR(255),
		session_id      VARCHAR(255),
		session_state   JSONB,
		task_state      JSONB,
		workspace_state JSONB,
		action          JSONB,
		input           JSONB,
		output          JSONB,
		trace_id        VARCHAR(64),
		span_id         VARCHAR(32),
		parent_span_id  VARCHAR(32),
		error           JSONB,
		metadata        JSONB,

		CONSTRAINT unique_execution_sequence UNIQUE (execution_id, sequence_num)
	);

	CREATE INDEX IF NOT EXISTS idx_snapshots_execution ON execution_snapshots(execution_id);
	CREATE INDEX IF NOT EXISTS idx_snapshots_timestamp ON execution_snapshots(timestamp);
	CREATE INDEX IF NOT EXISTS idx_snapshots_checkpoint ON execution_snapshots(checkpoint_type);
	CREATE INDEX IF NOT EXISTS idx_snapshots_agent ON execution_snapshots(agent_id) WHERE agent_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_snapshots_task ON execution_snapshots(task_id) WHERE task_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_snapshots_session ON execution_snapshots(session_id) WHERE session_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_snapshots_trace ON execution_snapshots(trace_id) WHERE trace_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_snapshots_has_error ON execution_snapshots((error IS NOT NULL));
	`

	_, err := s.db.Exec(schema)
	return err
}

func (s *PostgresSnapshotStore) prepareStatements() error {
	var err error

	s.stmtInsert, err = s.db.Prepare(`
		INSERT INTO execution_snapshots (
			id, execution_id, sequence_num, timestamp, checkpoint_type,
			agent_id, task_id, worker_id, session_id,
			session_state, task_state, workspace_state,
			action, input, output,
			trace_id, span_id, parent_span_id,
			error, metadata
		) VALUES (
			COALESCE(NULLIF($1, ''), gen_random_uuid()::text), $2, $3, $4, $5,
			NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''),
			$10, $11, $12,
			$13, $14, $15,
			NULLIF($16, ''), NULLIF($17, ''), NULLIF($18, ''),
			$19, $20
		)
		ON CONFLICT (execution_id, sequence_num) DO UPDATE SET
			timestamp = EXCLUDED.timestamp,
			checkpoint_type = EXCLUDED.checkpoint_type,
			agent_id = EXCLUDED.agent_id,
			task_id = EXCLUDED.task_id,
			worker_id = EXCLUDED.worker_id,
			session_id = EXCLUDED.session_id,
			session_state = EXCLUDED.session_state,
			task_state = EXCLUDED.task_state,
			workspace_state = EXCLUDED.workspace_state,
			action = EXCLUDED.action,
			input = EXCLUDED.input,
			output = EXCLUDED.output,
			trace_id = EXCLUDED.trace_id,
			span_id = EXCLUDED.span_id,
			parent_span_id = EXCLUDED.parent_span_id,
			error = EXCLUDED.error,
			metadata = EXCLUDED.metadata
		RETURNING id
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}

	s.stmtGet, err = s.db.Prepare(`
		SELECT id, execution_id, sequence_num, timestamp, checkpoint_type,
			agent_id, task_id, worker_id, session_id,
			session_state, task_state, workspace_state,
			action, input, output,
			trace_id, span_id, parent_span_id,
			error, metadata
		FROM execution_snapshots
		WHERE id = $1
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare get statement: %w", err)
	}

	s.stmtGetByExecution, err = s.db.Prepare(`
		SELECT id, execution_id, sequence_num, timestamp, checkpoint_type,
			agent_id, task_id, worker_id, session_id,
			session_state, task_state, workspace_state,
			action, input, output,
			trace_id, span_id, parent_span_id,
			error, metadata
		FROM execution_snapshots
		WHERE execution_id = $1
		ORDER BY sequence_num ASC
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare get by execution statement: %w", err)
	}

	s.stmtGetByExecutionRange, err = s.db.Prepare(`
		SELECT id, execution_id, sequence_num, timestamp, checkpoint_type,
			agent_id, task_id, worker_id, session_id,
			session_state, task_state, workspace_state,
			action, input, output,
			trace_id, span_id, parent_span_id,
			error, metadata
		FROM execution_snapshots
		WHERE execution_id = $1 AND sequence_num >= $2 AND sequence_num <= $3
		ORDER BY sequence_num ASC
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare get by execution range statement: %w", err)
	}

	s.stmtGetLatest, err = s.db.Prepare(`
		SELECT id, execution_id, sequence_num, timestamp, checkpoint_type,
			agent_id, task_id, worker_id, session_id,
			session_state, task_state, workspace_state,
			action, input, output,
			trace_id, span_id, parent_span_id,
			error, metadata
		FROM execution_snapshots
		WHERE execution_id = $1
		ORDER BY sequence_num DESC
		LIMIT 1
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare get latest statement: %w", err)
	}

	s.stmtGetAtSequence, err = s.db.Prepare(`
		SELECT id, execution_id, sequence_num, timestamp, checkpoint_type,
			agent_id, task_id, worker_id, session_id,
			session_state, task_state, workspace_state,
			action, input, output,
			trace_id, span_id, parent_span_id,
			error, metadata
		FROM execution_snapshots
		WHERE execution_id = $1 AND sequence_num = $2
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare get at sequence statement: %w", err)
	}

	s.stmtGetByCheckpoint, err = s.db.Prepare(`
		SELECT id, execution_id, sequence_num, timestamp, checkpoint_type,
			agent_id, task_id, worker_id, session_id,
			session_state, task_state, workspace_state,
			action, input, output,
			trace_id, span_id, parent_span_id,
			error, metadata
		FROM execution_snapshots
		WHERE execution_id = $1 AND checkpoint_type = $2
		ORDER BY sequence_num ASC
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare get by checkpoint statement: %w", err)
	}

	s.stmtPurgeOld, err = s.db.Prepare(`
		DELETE FROM execution_snapshots
		WHERE timestamp < $1
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare purge old statement: %w", err)
	}

	s.stmtPurgeExecution, err = s.db.Prepare(`
		DELETE FROM execution_snapshots
		WHERE execution_id = $1
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare purge execution statement: %w", err)
	}

	return nil
}

// Save persists a single snapshot.
func (s *PostgresSnapshotStore) Save(ctx context.Context, snapshot *ExecutionSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot cannot be nil")
	}

	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = time.Now()
	}

	sessionStateJSON, _ := json.Marshal(snapshot.SessionState)
	taskStateJSON, _ := json.Marshal(snapshot.TaskState)
	workspaceStateJSON, _ := json.Marshal(snapshot.WorkspaceState)
	actionJSON, _ := json.Marshal(snapshot.Action)
	inputJSON, _ := json.Marshal(snapshot.Input)
	outputJSON, _ := json.Marshal(snapshot.Output)
	errorJSON, _ := json.Marshal(snapshot.Error)
	metadataJSON, _ := json.Marshal(snapshot.Metadata)

	var id string
	err := s.stmtInsert.QueryRowContext(ctx,
		snapshot.ID, snapshot.ExecutionID, snapshot.SequenceNum, snapshot.Timestamp, snapshot.CheckpointType,
		snapshot.AgentID, snapshot.TaskID, snapshot.WorkerID, snapshot.SessionID,
		nullableJSON(sessionStateJSON), nullableJSON(taskStateJSON), nullableJSON(workspaceStateJSON),
		nullableJSON(actionJSON), nullableJSON(inputJSON), nullableJSON(outputJSON),
		snapshot.TraceID, snapshot.SpanID, snapshot.ParentSpanID,
		nullableJSON(errorJSON), nullableJSON(metadataJSON),
	).Scan(&id)

	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	snapshot.ID = id
	return nil
}

// SaveBatch persists multiple snapshots efficiently.
func (s *PostgresSnapshotStore) SaveBatch(ctx context.Context, snapshots []*ExecutionSnapshot) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt := tx.StmtContext(ctx, s.stmtInsert)

	for _, snapshot := range snapshots {
		if snapshot.Timestamp.IsZero() {
			snapshot.Timestamp = time.Now()
		}

		sessionStateJSON, _ := json.Marshal(snapshot.SessionState)
		taskStateJSON, _ := json.Marshal(snapshot.TaskState)
		workspaceStateJSON, _ := json.Marshal(snapshot.WorkspaceState)
		actionJSON, _ := json.Marshal(snapshot.Action)
		inputJSON, _ := json.Marshal(snapshot.Input)
		outputJSON, _ := json.Marshal(snapshot.Output)
		errorJSON, _ := json.Marshal(snapshot.Error)
		metadataJSON, _ := json.Marshal(snapshot.Metadata)

		var id string
		err := stmt.QueryRowContext(ctx,
			snapshot.ID, snapshot.ExecutionID, snapshot.SequenceNum, snapshot.Timestamp, snapshot.CheckpointType,
			snapshot.AgentID, snapshot.TaskID, snapshot.WorkerID, snapshot.SessionID,
			nullableJSON(sessionStateJSON), nullableJSON(taskStateJSON), nullableJSON(workspaceStateJSON),
			nullableJSON(actionJSON), nullableJSON(inputJSON), nullableJSON(outputJSON),
			snapshot.TraceID, snapshot.SpanID, snapshot.ParentSpanID,
			nullableJSON(errorJSON), nullableJSON(metadataJSON),
		).Scan(&id)

		if err != nil {
			return fmt.Errorf("failed to save snapshot in batch: %w", err)
		}
		snapshot.ID = id
	}

	return tx.Commit()
}

// GetByExecution returns all snapshots for an execution, ordered by sequence.
func (s *PostgresSnapshotStore) GetByExecution(ctx context.Context, executionID string) ([]*ExecutionSnapshot, error) {
	rows, err := s.stmtGetByExecution.QueryContext(ctx, executionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	return s.scanSnapshots(rows)
}

// GetByExecutionRange returns snapshots within a sequence range.
func (s *PostgresSnapshotStore) GetByExecutionRange(ctx context.Context, executionID string, fromSeq, toSeq int64) ([]*ExecutionSnapshot, error) {
	rows, err := s.stmtGetByExecutionRange.QueryContext(ctx, executionID, fromSeq, toSeq)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	return s.scanSnapshots(rows)
}

// GetByTimeRange returns snapshots within a time range.
func (s *PostgresSnapshotStore) GetByTimeRange(ctx context.Context, from, to time.Time) ([]*ExecutionSnapshot, error) {
	query := `
		SELECT id, execution_id, sequence_num, timestamp, checkpoint_type,
			agent_id, task_id, worker_id, session_id,
			session_state, task_state, workspace_state,
			action, input, output,
			trace_id, span_id, parent_span_id,
			error, metadata
		FROM execution_snapshots
		WHERE timestamp >= $1 AND timestamp <= $2
		ORDER BY timestamp ASC
	`
	rows, err := s.db.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	return s.scanSnapshots(rows)
}

// GetByCheckpointType returns snapshots of a specific checkpoint type within an execution.
func (s *PostgresSnapshotStore) GetByCheckpointType(ctx context.Context, executionID string, cpType CheckpointType) ([]*ExecutionSnapshot, error) {
	rows, err := s.stmtGetByCheckpoint.QueryContext(ctx, executionID, cpType)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	return s.scanSnapshots(rows)
}

// Get retrieves a snapshot by ID.
func (s *PostgresSnapshotStore) Get(ctx context.Context, snapshotID string) (*ExecutionSnapshot, error) {
	row := s.stmtGet.QueryRowContext(ctx, snapshotID)
	return s.scanSnapshot(row)
}

// GetLatest retrieves the most recent snapshot for an execution.
func (s *PostgresSnapshotStore) GetLatest(ctx context.Context, executionID string) (*ExecutionSnapshot, error) {
	row := s.stmtGetLatest.QueryRowContext(ctx, executionID)
	return s.scanSnapshot(row)
}

// GetAtSequence retrieves a snapshot at a specific sequence number.
func (s *PostgresSnapshotStore) GetAtSequence(ctx context.Context, executionID string, seqNum int64) (*ExecutionSnapshot, error) {
	row := s.stmtGetAtSequence.QueryRowContext(ctx, executionID, seqNum)
	return s.scanSnapshot(row)
}

// Query executes a complex query with filters, pagination, and ordering.
func (s *PostgresSnapshotStore) Query(ctx context.Context, query *SnapshotQuery) (*SnapshotQueryResult, error) {
	whereClause, args := s.buildWhereClause(&query.Filter)

	// Count total
	countQuery := "SELECT COUNT(*) FROM execution_snapshots" + whereClause
	var totalCount int64
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, fmt.Errorf("failed to count snapshots: %w", err)
	}

	// Determine order
	orderBy := "sequence_num ASC"
	switch query.OrderBy {
	case "sequence_desc":
		orderBy = "sequence_num DESC"
	case "time_asc":
		orderBy = "timestamp ASC"
	case "time_desc":
		orderBy = "timestamp DESC"
	}

	// Build query
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}

	selectQuery := fmt.Sprintf(`
		SELECT id, execution_id, sequence_num, timestamp, checkpoint_type,
			agent_id, task_id, worker_id, session_id,
			session_state, task_state, workspace_state,
			action, input, output,
			trace_id, span_id, parent_span_id,
			error, metadata
		FROM execution_snapshots%s
		ORDER BY %s
		LIMIT %d OFFSET %d
	`, whereClause, orderBy, limit, query.Offset)

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	snapshots, err := s.scanSnapshots(rows)
	if err != nil {
		return nil, err
	}

	return &SnapshotQueryResult{
		Snapshots:  snapshots,
		TotalCount: totalCount,
		HasMore:    int64(query.Offset+len(snapshots)) < totalCount,
	}, nil
}

// ListExecutions returns a list of unique execution IDs with summaries.
func (s *PostgresSnapshotStore) ListExecutions(ctx context.Context, limit, offset int) ([]*ExecutionSummary, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT
			execution_id,
			MIN(agent_id) as agent_id,
			MIN(timestamp) as start_time,
			MAX(timestamp) as end_time,
			COUNT(*) as total_steps,
			COUNT(*) FILTER (WHERE error IS NOT NULL) as error_count
		FROM execution_snapshots
		GROUP BY execution_id
		ORDER BY MAX(timestamp) DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}
	defer rows.Close()

	var summaries []*ExecutionSummary
	for rows.Next() {
		summary := &ExecutionSummary{
			CheckpointCounts: make(map[CheckpointType]int),
		}
		var agentID sql.NullString
		err := rows.Scan(
			&summary.ExecutionID,
			&agentID,
			&summary.StartTime,
			&summary.EndTime,
			&summary.TotalSteps,
			&summary.ErrorCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan execution summary: %w", err)
		}
		if agentID.Valid {
			summary.AgentID = agentID.String
		}
		summary.Duration = summary.EndTime.Sub(summary.StartTime)
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// GetExecutionSummary returns a summary for a specific execution.
func (s *PostgresSnapshotStore) GetExecutionSummary(ctx context.Context, executionID string) (*ExecutionSummary, error) {
	query := `
		SELECT
			execution_id,
			MIN(agent_id) as agent_id,
			MIN(timestamp) as start_time,
			MAX(timestamp) as end_time,
			COUNT(*) as total_steps,
			COUNT(*) FILTER (WHERE error IS NOT NULL) as error_count
		FROM execution_snapshots
		WHERE execution_id = $1
		GROUP BY execution_id
	`

	summary := &ExecutionSummary{
		CheckpointCounts: make(map[CheckpointType]int),
	}
	var agentID sql.NullString
	err := s.db.QueryRowContext(ctx, query, executionID).Scan(
		&summary.ExecutionID,
		&agentID,
		&summary.StartTime,
		&summary.EndTime,
		&summary.TotalSteps,
		&summary.ErrorCount,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("execution not found: %s", executionID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get execution summary: %w", err)
	}
	if agentID.Valid {
		summary.AgentID = agentID.String
	}
	summary.Duration = summary.EndTime.Sub(summary.StartTime)

	// Get checkpoint counts
	countQuery := `
		SELECT checkpoint_type, COUNT(*) as count
		FROM execution_snapshots
		WHERE execution_id = $1
		GROUP BY checkpoint_type
	`
	rows, err := s.db.QueryContext(ctx, countQuery, executionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cpType string
		var count int
		if err := rows.Scan(&cpType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan checkpoint count: %w", err)
		}
		summary.CheckpointCounts[CheckpointType(cpType)] = count
	}

	return summary, nil
}

// PurgeOlderThan removes snapshots older than the specified age.
func (s *PostgresSnapshotStore) PurgeOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	cutoff := time.Now().Add(-age)
	result, err := s.stmtPurgeOld.ExecContext(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to purge old snapshots: %w", err)
	}
	return result.RowsAffected()
}

// PurgeExecution removes all snapshots for a specific execution.
func (s *PostgresSnapshotStore) PurgeExecution(ctx context.Context, executionID string) (int64, error) {
	result, err := s.stmtPurgeExecution.ExecContext(ctx, executionID)
	if err != nil {
		return 0, fmt.Errorf("failed to purge execution: %w", err)
	}
	return result.RowsAffected()
}

// Stats returns statistics about the store.
func (s *PostgresSnapshotStore) Stats(ctx context.Context) (*StoreStats, error) {
	stats := &StoreStats{}

	// Get counts
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total_snapshots,
			COUNT(DISTINCT execution_id) as total_executions,
			COALESCE(MIN(timestamp), NOW()) as oldest,
			COALESCE(MAX(timestamp), NOW()) as newest
		FROM execution_snapshots
	`).Scan(&stats.TotalSnapshots, &stats.TotalExecutions, &stats.OldestSnapshot, &stats.NewestSnapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// Get storage size estimate
	err = s.db.QueryRowContext(ctx, `
		SELECT pg_total_relation_size('execution_snapshots')
	`).Scan(&stats.StorageSizeBytes)
	if err != nil {
		// Ignore error, size is optional
		stats.StorageSizeBytes = 0
	}

	return stats, nil
}

// Close closes the store and releases resources.
func (s *PostgresSnapshotStore) Close() error {
	if s.stmtInsert != nil {
		s.stmtInsert.Close()
	}
	if s.stmtGet != nil {
		s.stmtGet.Close()
	}
	if s.stmtGetByExecution != nil {
		s.stmtGetByExecution.Close()
	}
	if s.stmtGetByExecutionRange != nil {
		s.stmtGetByExecutionRange.Close()
	}
	if s.stmtGetLatest != nil {
		s.stmtGetLatest.Close()
	}
	if s.stmtGetAtSequence != nil {
		s.stmtGetAtSequence.Close()
	}
	if s.stmtGetByCheckpoint != nil {
		s.stmtGetByCheckpoint.Close()
	}
	if s.stmtPurgeOld != nil {
		s.stmtPurgeOld.Close()
	}
	if s.stmtPurgeExecution != nil {
		s.stmtPurgeExecution.Close()
	}
	return s.db.Close()
}

// Helper methods

func (s *PostgresSnapshotStore) buildWhereClause(filter *SnapshotFilter) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argNum := 1

	if filter.ExecutionID != "" {
		conditions = append(conditions, fmt.Sprintf("execution_id = $%d", argNum))
		args = append(args, filter.ExecutionID)
		argNum++
	}
	if filter.AgentID != "" {
		conditions = append(conditions, fmt.Sprintf("agent_id = $%d", argNum))
		args = append(args, filter.AgentID)
		argNum++
	}
	if filter.TaskID != "" {
		conditions = append(conditions, fmt.Sprintf("task_id = $%d", argNum))
		args = append(args, filter.TaskID)
		argNum++
	}
	if filter.SessionID != "" {
		conditions = append(conditions, fmt.Sprintf("session_id = $%d", argNum))
		args = append(args, filter.SessionID)
		argNum++
	}
	if filter.CheckpointType != "" {
		conditions = append(conditions, fmt.Sprintf("checkpoint_type = $%d", argNum))
		args = append(args, filter.CheckpointType)
		argNum++
	}
	if filter.FromTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argNum))
		args = append(args, *filter.FromTime)
		argNum++
	}
	if filter.ToTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", argNum))
		args = append(args, *filter.ToTime)
		argNum++
	}
	if filter.FromSequence != nil {
		conditions = append(conditions, fmt.Sprintf("sequence_num >= $%d", argNum))
		args = append(args, *filter.FromSequence)
		argNum++
	}
	if filter.ToSequence != nil {
		conditions = append(conditions, fmt.Sprintf("sequence_num <= $%d", argNum))
		args = append(args, *filter.ToSequence)
		argNum++
	}
	if filter.HasError != nil {
		if *filter.HasError {
			conditions = append(conditions, "error IS NOT NULL")
		} else {
			conditions = append(conditions, "error IS NULL")
		}
	}
	if filter.TraceID != "" {
		conditions = append(conditions, fmt.Sprintf("trace_id = $%d", argNum))
		args = append(args, filter.TraceID)
		argNum++
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return " WHERE " + strings.Join(conditions, " AND "), args
}

func (s *PostgresSnapshotStore) scanSnapshot(row *sql.Row) (*ExecutionSnapshot, error) {
	snap := &ExecutionSnapshot{}
	var (
		agentID, taskID, workerID, sessionID                        sql.NullString
		traceID, spanID, parentSpanID                               sql.NullString
		sessionStateJSON, taskStateJSON, workspaceStateJSON         []byte
		actionJSON, inputJSON, outputJSON, errorJSON, metadataJSON  []byte
	)

	err := row.Scan(
		&snap.ID, &snap.ExecutionID, &snap.SequenceNum, &snap.Timestamp, &snap.CheckpointType,
		&agentID, &taskID, &workerID, &sessionID,
		&sessionStateJSON, &taskStateJSON, &workspaceStateJSON,
		&actionJSON, &inputJSON, &outputJSON,
		&traceID, &spanID, &parentSpanID,
		&errorJSON, &metadataJSON,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("snapshot not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan snapshot: %w", err)
	}

	snap.AgentID = agentID.String
	snap.TaskID = taskID.String
	snap.WorkerID = workerID.String
	snap.SessionID = sessionID.String
	snap.TraceID = traceID.String
	snap.SpanID = spanID.String
	snap.ParentSpanID = parentSpanID.String

	if len(sessionStateJSON) > 0 {
		json.Unmarshal(sessionStateJSON, &snap.SessionState)
	}
	if len(taskStateJSON) > 0 {
		json.Unmarshal(taskStateJSON, &snap.TaskState)
	}
	if len(workspaceStateJSON) > 0 {
		json.Unmarshal(workspaceStateJSON, &snap.WorkspaceState)
	}
	if len(actionJSON) > 0 {
		json.Unmarshal(actionJSON, &snap.Action)
	}
	if len(inputJSON) > 0 {
		json.Unmarshal(inputJSON, &snap.Input)
	}
	if len(outputJSON) > 0 {
		json.Unmarshal(outputJSON, &snap.Output)
	}
	if len(errorJSON) > 0 {
		json.Unmarshal(errorJSON, &snap.Error)
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &snap.Metadata)
	}

	return snap, nil
}

func (s *PostgresSnapshotStore) scanSnapshots(rows *sql.Rows) ([]*ExecutionSnapshot, error) {
	var snapshots []*ExecutionSnapshot

	for rows.Next() {
		snap := &ExecutionSnapshot{}
		var (
			agentID, taskID, workerID, sessionID                        sql.NullString
			traceID, spanID, parentSpanID                               sql.NullString
			sessionStateJSON, taskStateJSON, workspaceStateJSON         []byte
			actionJSON, inputJSON, outputJSON, errorJSON, metadataJSON  []byte
		)

		err := rows.Scan(
			&snap.ID, &snap.ExecutionID, &snap.SequenceNum, &snap.Timestamp, &snap.CheckpointType,
			&agentID, &taskID, &workerID, &sessionID,
			&sessionStateJSON, &taskStateJSON, &workspaceStateJSON,
			&actionJSON, &inputJSON, &outputJSON,
			&traceID, &spanID, &parentSpanID,
			&errorJSON, &metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan snapshot: %w", err)
		}

		snap.AgentID = agentID.String
		snap.TaskID = taskID.String
		snap.WorkerID = workerID.String
		snap.SessionID = sessionID.String
		snap.TraceID = traceID.String
		snap.SpanID = spanID.String
		snap.ParentSpanID = parentSpanID.String

		if len(sessionStateJSON) > 0 {
			json.Unmarshal(sessionStateJSON, &snap.SessionState)
		}
		if len(taskStateJSON) > 0 {
			json.Unmarshal(taskStateJSON, &snap.TaskState)
		}
		if len(workspaceStateJSON) > 0 {
			json.Unmarshal(workspaceStateJSON, &snap.WorkspaceState)
		}
		if len(actionJSON) > 0 {
			json.Unmarshal(actionJSON, &snap.Action)
		}
		if len(inputJSON) > 0 {
			json.Unmarshal(inputJSON, &snap.Input)
		}
		if len(outputJSON) > 0 {
			json.Unmarshal(outputJSON, &snap.Output)
		}
		if len(errorJSON) > 0 {
			json.Unmarshal(errorJSON, &snap.Error)
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &snap.Metadata)
		}

		snapshots = append(snapshots, snap)
	}

	return snapshots, nil
}

func nullableJSON(data []byte) interface{} {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return data
}
