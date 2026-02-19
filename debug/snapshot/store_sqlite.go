// Package snapshot provides a SQLite implementation of SnapshotStore.
package snapshot

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteSnapshotStore is a SQLite-backed implementation of SnapshotStore.
// Suitable for embedded deployments, edge computing, and single-process applications.
type SQLiteSnapshotStore struct {
	db *sql.DB

	// Prepared statements
	stmtInsert              *sql.Stmt
	stmtGet                 *sql.Stmt
	stmtGetByExecution      *sql.Stmt
	stmtGetByExecutionRange *sql.Stmt
	stmtGetLatest           *sql.Stmt
	stmtGetAtSequence       *sql.Stmt
	stmtGetByCheckpoint     *sql.Stmt
	stmtPurgeOld            *sql.Stmt
	stmtPurgeExecution      *sql.Stmt

	// Configuration
	batchSize       int
	retentionPeriod time.Duration
}

// SQLiteConfig holds configuration for SQLite connection.
type SQLiteConfig struct {
	// Path to the SQLite database file (use ":memory:" for in-memory)
	Path string

	// BatchSize for bulk operations (default: 100)
	BatchSize int

	// RetentionPeriod for automatic cleanup (default: 7 days)
	RetentionPeriod time.Duration

	// WALMode enables Write-Ahead Logging for better concurrency (default: true)
	WALMode bool

	// BusyTimeout in milliseconds (default: 5000)
	BusyTimeout int
}

// DefaultSQLiteConfig returns a default SQLite configuration.
func DefaultSQLiteConfig() SQLiteConfig {
	return SQLiteConfig{
		Path:            "minion_snapshots.db",
		BatchSize:       100,
		RetentionPeriod: 7 * 24 * time.Hour,
		WALMode:         true,
		BusyTimeout:     5000,
	}
}

// NewSQLiteSnapshotStore creates a new SQLite-backed snapshot store.
func NewSQLiteSnapshotStore(config SQLiteConfig) (*SQLiteSnapshotStore, error) {
	// Build connection string with pragmas
	connStr := config.Path
	if config.Path != ":memory:" {
		connStr = fmt.Sprintf("file:%s?_busy_timeout=%d", config.Path, config.BusyTimeout)
		if config.WALMode {
			connStr += "&_journal_mode=WAL"
		}
	}

	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings for SQLite
	db.SetMaxOpenConns(1) // SQLite only supports one writer at a time
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &SQLiteSnapshotStore{
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

// NewSQLiteSnapshotStoreFromDB creates a store from an existing database connection.
func NewSQLiteSnapshotStoreFromDB(db *sql.DB) (*SQLiteSnapshotStore, error) {
	store := &SQLiteSnapshotStore{
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

func (s *SQLiteSnapshotStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS execution_snapshots (
		id              TEXT PRIMARY KEY,
		execution_id    TEXT NOT NULL,
		sequence_num    INTEGER NOT NULL,
		timestamp       TEXT NOT NULL,
		checkpoint_type TEXT NOT NULL,
		agent_id        TEXT,
		task_id         TEXT,
		worker_id       TEXT,
		session_id      TEXT,
		session_state   TEXT,
		task_state      TEXT,
		workspace_state TEXT,
		action          TEXT,
		input           TEXT,
		output          TEXT,
		trace_id        TEXT,
		span_id         TEXT,
		parent_span_id  TEXT,
		error           TEXT,
		metadata        TEXT,

		UNIQUE(execution_id, sequence_num)
	);

	CREATE INDEX IF NOT EXISTS idx_snapshots_execution ON execution_snapshots(execution_id);
	CREATE INDEX IF NOT EXISTS idx_snapshots_timestamp ON execution_snapshots(timestamp);
	CREATE INDEX IF NOT EXISTS idx_snapshots_checkpoint ON execution_snapshots(checkpoint_type);
	CREATE INDEX IF NOT EXISTS idx_snapshots_agent ON execution_snapshots(agent_id) WHERE agent_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_snapshots_task ON execution_snapshots(task_id) WHERE task_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_snapshots_session ON execution_snapshots(session_id) WHERE session_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_snapshots_trace ON execution_snapshots(trace_id) WHERE trace_id IS NOT NULL;
	`

	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteSnapshotStore) prepareStatements() error {
	var err error

	s.stmtInsert, err = s.db.Prepare(`
		INSERT OR REPLACE INTO execution_snapshots (
			id, execution_id, sequence_num, timestamp, checkpoint_type,
			agent_id, task_id, worker_id, session_id,
			session_state, task_state, workspace_state,
			action, input, output,
			trace_id, span_id, parent_span_id,
			error, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		WHERE id = ?
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
		WHERE execution_id = ?
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
		WHERE execution_id = ? AND sequence_num >= ? AND sequence_num <= ?
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
		WHERE execution_id = ?
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
		WHERE execution_id = ? AND sequence_num = ?
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
		WHERE execution_id = ? AND checkpoint_type = ?
		ORDER BY sequence_num ASC
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare get by checkpoint statement: %w", err)
	}

	s.stmtPurgeOld, err = s.db.Prepare(`
		DELETE FROM execution_snapshots
		WHERE timestamp < ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare purge old statement: %w", err)
	}

	s.stmtPurgeExecution, err = s.db.Prepare(`
		DELETE FROM execution_snapshots
		WHERE execution_id = ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare purge execution statement: %w", err)
	}

	return nil
}

// Save persists a single snapshot.
func (s *SQLiteSnapshotStore) Save(ctx context.Context, snapshot *ExecutionSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot cannot be nil")
	}

	if snapshot.ID == "" {
		snapshot.ID = generateUUID()
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

	_, err := s.stmtInsert.ExecContext(ctx,
		snapshot.ID, snapshot.ExecutionID, snapshot.SequenceNum, snapshot.Timestamp.Format(time.RFC3339Nano), snapshot.CheckpointType,
		nullableString(snapshot.AgentID), nullableString(snapshot.TaskID), nullableString(snapshot.WorkerID), nullableString(snapshot.SessionID),
		nullableJSONString(sessionStateJSON), nullableJSONString(taskStateJSON), nullableJSONString(workspaceStateJSON),
		nullableJSONString(actionJSON), nullableJSONString(inputJSON), nullableJSONString(outputJSON),
		nullableString(snapshot.TraceID), nullableString(snapshot.SpanID), nullableString(snapshot.ParentSpanID),
		nullableJSONString(errorJSON), nullableJSONString(metadataJSON),
	)

	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	return nil
}

// SaveBatch persists multiple snapshots efficiently.
func (s *SQLiteSnapshotStore) SaveBatch(ctx context.Context, snapshots []*ExecutionSnapshot) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt := tx.StmtContext(ctx, s.stmtInsert)

	for _, snapshot := range snapshots {
		if snapshot.ID == "" {
			snapshot.ID = generateUUID()
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

		_, err := stmt.ExecContext(ctx,
			snapshot.ID, snapshot.ExecutionID, snapshot.SequenceNum, snapshot.Timestamp.Format(time.RFC3339Nano), snapshot.CheckpointType,
			nullableString(snapshot.AgentID), nullableString(snapshot.TaskID), nullableString(snapshot.WorkerID), nullableString(snapshot.SessionID),
			nullableJSONString(sessionStateJSON), nullableJSONString(taskStateJSON), nullableJSONString(workspaceStateJSON),
			nullableJSONString(actionJSON), nullableJSONString(inputJSON), nullableJSONString(outputJSON),
			nullableString(snapshot.TraceID), nullableString(snapshot.SpanID), nullableString(snapshot.ParentSpanID),
			nullableJSONString(errorJSON), nullableJSONString(metadataJSON),
		)

		if err != nil {
			return fmt.Errorf("failed to save snapshot in batch: %w", err)
		}
	}

	return tx.Commit()
}

// GetByExecution returns all snapshots for an execution, ordered by sequence.
func (s *SQLiteSnapshotStore) GetByExecution(ctx context.Context, executionID string) ([]*ExecutionSnapshot, error) {
	rows, err := s.stmtGetByExecution.QueryContext(ctx, executionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	return s.scanSnapshots(rows)
}

// GetByExecutionRange returns snapshots within a sequence range.
func (s *SQLiteSnapshotStore) GetByExecutionRange(ctx context.Context, executionID string, fromSeq, toSeq int64) ([]*ExecutionSnapshot, error) {
	rows, err := s.stmtGetByExecutionRange.QueryContext(ctx, executionID, fromSeq, toSeq)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	return s.scanSnapshots(rows)
}

// GetByTimeRange returns snapshots within a time range.
func (s *SQLiteSnapshotStore) GetByTimeRange(ctx context.Context, from, to time.Time) ([]*ExecutionSnapshot, error) {
	query := `
		SELECT id, execution_id, sequence_num, timestamp, checkpoint_type,
			agent_id, task_id, worker_id, session_id,
			session_state, task_state, workspace_state,
			action, input, output,
			trace_id, span_id, parent_span_id,
			error, metadata
		FROM execution_snapshots
		WHERE timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp ASC
	`
	rows, err := s.db.QueryContext(ctx, query, from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano))
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	return s.scanSnapshots(rows)
}

// GetByCheckpointType returns snapshots of a specific checkpoint type within an execution.
func (s *SQLiteSnapshotStore) GetByCheckpointType(ctx context.Context, executionID string, cpType CheckpointType) ([]*ExecutionSnapshot, error) {
	rows, err := s.stmtGetByCheckpoint.QueryContext(ctx, executionID, cpType)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	return s.scanSnapshots(rows)
}

// Get retrieves a snapshot by ID.
func (s *SQLiteSnapshotStore) Get(ctx context.Context, snapshotID string) (*ExecutionSnapshot, error) {
	row := s.stmtGet.QueryRowContext(ctx, snapshotID)
	return s.scanSnapshot(row)
}

// GetLatest retrieves the most recent snapshot for an execution.
func (s *SQLiteSnapshotStore) GetLatest(ctx context.Context, executionID string) (*ExecutionSnapshot, error) {
	row := s.stmtGetLatest.QueryRowContext(ctx, executionID)
	return s.scanSnapshot(row)
}

// GetAtSequence retrieves a snapshot at a specific sequence number.
func (s *SQLiteSnapshotStore) GetAtSequence(ctx context.Context, executionID string, seqNum int64) (*ExecutionSnapshot, error) {
	row := s.stmtGetAtSequence.QueryRowContext(ctx, executionID, seqNum)
	return s.scanSnapshot(row)
}

// Query executes a complex query with filters, pagination, and ordering.
func (s *SQLiteSnapshotStore) Query(ctx context.Context, query *SnapshotQuery) (*SnapshotQueryResult, error) {
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
func (s *SQLiteSnapshotStore) ListExecutions(ctx context.Context, limit, offset int) ([]*ExecutionSummary, error) {
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
			SUM(CASE WHEN error IS NOT NULL AND error != 'null' THEN 1 ELSE 0 END) as error_count
		FROM execution_snapshots
		GROUP BY execution_id
		ORDER BY MAX(timestamp) DESC
		LIMIT ? OFFSET ?
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
		var startTimeStr, endTimeStr string
		err := rows.Scan(
			&summary.ExecutionID,
			&agentID,
			&startTimeStr,
			&endTimeStr,
			&summary.TotalSteps,
			&summary.ErrorCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan execution summary: %w", err)
		}
		if agentID.Valid {
			summary.AgentID = agentID.String
		}
		summary.StartTime, _ = time.Parse(time.RFC3339Nano, startTimeStr)
		summary.EndTime, _ = time.Parse(time.RFC3339Nano, endTimeStr)
		summary.Duration = summary.EndTime.Sub(summary.StartTime)
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// GetExecutionSummary returns a summary for a specific execution.
func (s *SQLiteSnapshotStore) GetExecutionSummary(ctx context.Context, executionID string) (*ExecutionSummary, error) {
	query := `
		SELECT
			execution_id,
			MIN(agent_id) as agent_id,
			MIN(timestamp) as start_time,
			MAX(timestamp) as end_time,
			COUNT(*) as total_steps,
			SUM(CASE WHEN error IS NOT NULL AND error != 'null' THEN 1 ELSE 0 END) as error_count
		FROM execution_snapshots
		WHERE execution_id = ?
		GROUP BY execution_id
	`

	summary := &ExecutionSummary{
		CheckpointCounts: make(map[CheckpointType]int),
	}
	var agentID sql.NullString
	var startTimeStr, endTimeStr string
	err := s.db.QueryRowContext(ctx, query, executionID).Scan(
		&summary.ExecutionID,
		&agentID,
		&startTimeStr,
		&endTimeStr,
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
	summary.StartTime, _ = time.Parse(time.RFC3339Nano, startTimeStr)
	summary.EndTime, _ = time.Parse(time.RFC3339Nano, endTimeStr)
	summary.Duration = summary.EndTime.Sub(summary.StartTime)

	// Get checkpoint counts
	countQuery := `
		SELECT checkpoint_type, COUNT(*) as count
		FROM execution_snapshots
		WHERE execution_id = ?
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
func (s *SQLiteSnapshotStore) PurgeOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	cutoff := time.Now().Add(-age)
	result, err := s.stmtPurgeOld.ExecContext(ctx, cutoff.Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("failed to purge old snapshots: %w", err)
	}
	return result.RowsAffected()
}

// PurgeExecution removes all snapshots for a specific execution.
func (s *SQLiteSnapshotStore) PurgeExecution(ctx context.Context, executionID string) (int64, error) {
	result, err := s.stmtPurgeExecution.ExecContext(ctx, executionID)
	if err != nil {
		return 0, fmt.Errorf("failed to purge execution: %w", err)
	}
	return result.RowsAffected()
}

// Stats returns statistics about the store.
func (s *SQLiteSnapshotStore) Stats(ctx context.Context) (*StoreStats, error) {
	stats := &StoreStats{}

	// Get counts
	var oldestStr, newestStr string
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total_snapshots,
			COUNT(DISTINCT execution_id) as total_executions,
			COALESCE(MIN(timestamp), datetime('now')) as oldest,
			COALESCE(MAX(timestamp), datetime('now')) as newest
		FROM execution_snapshots
	`).Scan(&stats.TotalSnapshots, &stats.TotalExecutions, &oldestStr, &newestStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// Parse timestamps
	stats.OldestSnapshot, _ = time.Parse(time.RFC3339Nano, oldestStr)
	stats.NewestSnapshot, _ = time.Parse(time.RFC3339Nano, newestStr)

	// Get storage size (SQLite specific)
	var pageCount, pageSize int64
	s.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount)
	s.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize)
	stats.StorageSizeBytes = pageCount * pageSize

	return stats, nil
}

// Close closes the store and releases resources.
func (s *SQLiteSnapshotStore) Close() error {
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

// Vacuum performs SQLite VACUUM to reclaim space.
func (s *SQLiteSnapshotStore) Vacuum(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "VACUUM")
	return err
}

// Helper methods

func (s *SQLiteSnapshotStore) buildWhereClause(filter *SnapshotFilter) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	if filter.ExecutionID != "" {
		conditions = append(conditions, "execution_id = ?")
		args = append(args, filter.ExecutionID)
	}
	if filter.AgentID != "" {
		conditions = append(conditions, "agent_id = ?")
		args = append(args, filter.AgentID)
	}
	if filter.TaskID != "" {
		conditions = append(conditions, "task_id = ?")
		args = append(args, filter.TaskID)
	}
	if filter.SessionID != "" {
		conditions = append(conditions, "session_id = ?")
		args = append(args, filter.SessionID)
	}
	if filter.CheckpointType != "" {
		conditions = append(conditions, "checkpoint_type = ?")
		args = append(args, filter.CheckpointType)
	}
	if filter.FromTime != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, filter.FromTime.Format(time.RFC3339Nano))
	}
	if filter.ToTime != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, filter.ToTime.Format(time.RFC3339Nano))
	}
	if filter.FromSequence != nil {
		conditions = append(conditions, "sequence_num >= ?")
		args = append(args, *filter.FromSequence)
	}
	if filter.ToSequence != nil {
		conditions = append(conditions, "sequence_num <= ?")
		args = append(args, *filter.ToSequence)
	}
	if filter.HasError != nil {
		if *filter.HasError {
			conditions = append(conditions, "error IS NOT NULL AND error != 'null'")
		} else {
			conditions = append(conditions, "(error IS NULL OR error = 'null')")
		}
	}
	if filter.TraceID != "" {
		conditions = append(conditions, "trace_id = ?")
		args = append(args, filter.TraceID)
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return " WHERE " + strings.Join(conditions, " AND "), args
}

func (s *SQLiteSnapshotStore) scanSnapshot(row *sql.Row) (*ExecutionSnapshot, error) {
	snap := &ExecutionSnapshot{}
	var (
		agentID, taskID, workerID, sessionID                       sql.NullString
		traceID, spanID, parentSpanID                              sql.NullString
		sessionStateJSON, taskStateJSON, workspaceStateJSON        sql.NullString
		actionJSON, inputJSON, outputJSON, errorJSON, metadataJSON sql.NullString
		timestampStr                                               string
	)

	err := row.Scan(
		&snap.ID, &snap.ExecutionID, &snap.SequenceNum, &timestampStr, &snap.CheckpointType,
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

	snap.Timestamp, _ = time.Parse(time.RFC3339Nano, timestampStr)
	snap.AgentID = agentID.String
	snap.TaskID = taskID.String
	snap.WorkerID = workerID.String
	snap.SessionID = sessionID.String
	snap.TraceID = traceID.String
	snap.SpanID = spanID.String
	snap.ParentSpanID = parentSpanID.String

	if sessionStateJSON.Valid && sessionStateJSON.String != "null" {
		json.Unmarshal([]byte(sessionStateJSON.String), &snap.SessionState)
	}
	if taskStateJSON.Valid && taskStateJSON.String != "null" {
		json.Unmarshal([]byte(taskStateJSON.String), &snap.TaskState)
	}
	if workspaceStateJSON.Valid && workspaceStateJSON.String != "null" {
		json.Unmarshal([]byte(workspaceStateJSON.String), &snap.WorkspaceState)
	}
	if actionJSON.Valid && actionJSON.String != "null" {
		json.Unmarshal([]byte(actionJSON.String), &snap.Action)
	}
	if inputJSON.Valid && inputJSON.String != "null" {
		json.Unmarshal([]byte(inputJSON.String), &snap.Input)
	}
	if outputJSON.Valid && outputJSON.String != "null" {
		json.Unmarshal([]byte(outputJSON.String), &snap.Output)
	}
	if errorJSON.Valid && errorJSON.String != "null" {
		json.Unmarshal([]byte(errorJSON.String), &snap.Error)
	}
	if metadataJSON.Valid && metadataJSON.String != "null" {
		json.Unmarshal([]byte(metadataJSON.String), &snap.Metadata)
	}

	return snap, nil
}

func (s *SQLiteSnapshotStore) scanSnapshots(rows *sql.Rows) ([]*ExecutionSnapshot, error) {
	var snapshots []*ExecutionSnapshot

	for rows.Next() {
		snap := &ExecutionSnapshot{}
		var (
			agentID, taskID, workerID, sessionID                       sql.NullString
			traceID, spanID, parentSpanID                              sql.NullString
			sessionStateJSON, taskStateJSON, workspaceStateJSON        sql.NullString
			actionJSON, inputJSON, outputJSON, errorJSON, metadataJSON sql.NullString
			timestampStr                                               string
		)

		err := rows.Scan(
			&snap.ID, &snap.ExecutionID, &snap.SequenceNum, &timestampStr, &snap.CheckpointType,
			&agentID, &taskID, &workerID, &sessionID,
			&sessionStateJSON, &taskStateJSON, &workspaceStateJSON,
			&actionJSON, &inputJSON, &outputJSON,
			&traceID, &spanID, &parentSpanID,
			&errorJSON, &metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan snapshot: %w", err)
		}

		snap.Timestamp, _ = time.Parse(time.RFC3339Nano, timestampStr)
		snap.AgentID = agentID.String
		snap.TaskID = taskID.String
		snap.WorkerID = workerID.String
		snap.SessionID = sessionID.String
		snap.TraceID = traceID.String
		snap.SpanID = spanID.String
		snap.ParentSpanID = parentSpanID.String

		if sessionStateJSON.Valid && sessionStateJSON.String != "null" {
			json.Unmarshal([]byte(sessionStateJSON.String), &snap.SessionState)
		}
		if taskStateJSON.Valid && taskStateJSON.String != "null" {
			json.Unmarshal([]byte(taskStateJSON.String), &snap.TaskState)
		}
		if workspaceStateJSON.Valid && workspaceStateJSON.String != "null" {
			json.Unmarshal([]byte(workspaceStateJSON.String), &snap.WorkspaceState)
		}
		if actionJSON.Valid && actionJSON.String != "null" {
			json.Unmarshal([]byte(actionJSON.String), &snap.Action)
		}
		if inputJSON.Valid && inputJSON.String != "null" {
			json.Unmarshal([]byte(inputJSON.String), &snap.Input)
		}
		if outputJSON.Valid && outputJSON.String != "null" {
			json.Unmarshal([]byte(outputJSON.String), &snap.Output)
		}
		if errorJSON.Valid && errorJSON.String != "null" {
			json.Unmarshal([]byte(errorJSON.String), &snap.Error)
		}
		if metadataJSON.Valid && metadataJSON.String != "null" {
			json.Unmarshal([]byte(metadataJSON.String), &snap.Metadata)
		}

		snapshots = append(snapshots, snap)
	}

	return snapshots, nil
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullableJSONString(data []byte) interface{} {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return string(data)
}

func generateUUID() string {
	// Simple UUID v4 generation
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() >> (i * 8))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Ensure SQLiteSnapshotStore implements SnapshotStore
var _ SnapshotStore = (*SQLiteSnapshotStore)(nil)
