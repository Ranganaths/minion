package multiagent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// PostgresLedger implements LedgerBackend using PostgreSQL
type PostgresLedger struct {
	config *PostgresLedgerConfig
	db     *sql.DB

	// Prepared statements
	stmtCreateTask     *sql.Stmt
	stmtGetTask        *sql.Stmt
	stmtUpdateTask     *sql.Stmt
	stmtDeleteTask     *sql.Stmt
	stmtRecordProgress *sql.Stmt
}

// NewPostgresLedger creates a new PostgreSQL ledger backend
func NewPostgresLedger(config *PostgresLedgerConfig) (*PostgresLedger, error) {
	if config == nil {
		config = DefaultPostgresConfig()
	}

	// Build connection string
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password,
		config.Database, config.SSLMode,
	)

	// Add statement timeout if configured
	if config.StatementTimeout > 0 {
		connStr += fmt.Sprintf(" statement_timeout=%d", config.StatementTimeout.Milliseconds())
	}

	// Open database connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	pl := &PostgresLedger{
		config: config,
		db:     db,
	}

	// Prepare statements if enabled
	if config.PreparedStmts {
		if err := pl.prepareStatements(ctx); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to prepare statements: %w", err)
		}
	}

	return pl, nil
}

// prepareStatements prepares frequently used SQL statements
func (pl *PostgresLedger) prepareStatements(ctx context.Context) error {
	var err error

	// Create task
	pl.stmtCreateTask, err = pl.db.PrepareContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (
			id, name, description, type, priority, assigned_to, created_by,
			dependencies, input, status, metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, pl.config.TasksTable))
	if err != nil {
		return fmt.Errorf("failed to prepare create task: %w", err)
	}

	// Get task
	pl.stmtGetTask, err = pl.db.PrepareContext(ctx, fmt.Sprintf(`
		SELECT id, name, description, type, priority, assigned_to, created_by,
		       dependencies, input, output, status, error, metadata,
		       created_at, updated_at, completed_at
		FROM %s WHERE id = $1
	`, pl.config.TasksTable))
	if err != nil {
		return fmt.Errorf("failed to prepare get task: %w", err)
	}

	// Update task
	pl.stmtUpdateTask, err = pl.db.PrepareContext(ctx, fmt.Sprintf(`
		UPDATE %s SET
			name = $2, description = $3, type = $4, priority = $5,
			assigned_to = $6, dependencies = $7, input = $8, output = $9,
			status = $10, error = $11, metadata = $12, updated_at = $13,
			completed_at = $14
		WHERE id = $1
	`, pl.config.TasksTable))
	if err != nil {
		return fmt.Errorf("failed to prepare update task: %w", err)
	}

	// Delete task
	pl.stmtDeleteTask, err = pl.db.PrepareContext(ctx, fmt.Sprintf(`
		DELETE FROM %s WHERE id = $1
	`, pl.config.TasksTable))
	if err != nil {
		return fmt.Errorf("failed to prepare delete task: %w", err)
	}

	// Record progress
	pl.stmtRecordProgress, err = pl.db.PrepareContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (task_id, agent_id, status, message, metadata, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, pl.config.ProgressTable))
	if err != nil {
		return fmt.Errorf("failed to prepare record progress: %w", err)
	}

	return nil
}

// CreateTask creates a new task in the database
func (pl *PostgresLedger) CreateTask(ctx context.Context, task *Task) error {
	// Serialize JSON fields
	dependencies, err := json.Marshal(task.Dependencies)
	if err != nil {
		return fmt.Errorf("failed to marshal dependencies: %w", err)
	}

	input, err := json.Marshal(task.Input)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	metadata, err := json.Marshal(task.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Use prepared statement if available
	if pl.stmtCreateTask != nil {
		_, err = pl.stmtCreateTask.ExecContext(ctx,
			task.ID, task.Name, task.Description, task.Type, task.Priority,
			task.AssignedTo, task.CreatedBy, dependencies, input,
			task.Status, metadata, task.CreatedAt, task.UpdatedAt,
		)
	} else {
		query := fmt.Sprintf(`
			INSERT INTO %s (
				id, name, description, type, priority, assigned_to, created_by,
				dependencies, input, status, metadata, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, pl.config.TasksTable)

		_, err = pl.db.ExecContext(ctx, query,
			task.ID, task.Name, task.Description, task.Type, task.Priority,
			task.AssignedTo, task.CreatedBy, dependencies, input,
			task.Status, metadata, task.CreatedAt, task.UpdatedAt,
		)
	}

	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	return nil
}

// GetTask retrieves a task by ID
func (pl *PostgresLedger) GetTask(ctx context.Context, taskID string) (*Task, error) {
	task := &Task{}
	var dependencies, input, output, metadata []byte
	var completedAt sql.NullTime

	var err error
	if pl.stmtGetTask != nil {
		err = pl.stmtGetTask.QueryRowContext(ctx, taskID).Scan(
			&task.ID, &task.Name, &task.Description, &task.Type, &task.Priority,
			&task.AssignedTo, &task.CreatedBy, &dependencies, &input, &output,
			&task.Status, &task.Error, &metadata, &task.CreatedAt, &task.UpdatedAt,
			&completedAt,
		)
	} else {
		query := fmt.Sprintf(`
			SELECT id, name, description, type, priority, assigned_to, created_by,
			       dependencies, input, output, status, error, metadata,
			       created_at, updated_at, completed_at
			FROM %s WHERE id = $1
		`, pl.config.TasksTable)

		err = pl.db.QueryRowContext(ctx, query, taskID).Scan(
			&task.ID, &task.Name, &task.Description, &task.Type, &task.Priority,
			&task.AssignedTo, &task.CreatedBy, &dependencies, &input, &output,
			&task.Status, &task.Error, &metadata, &task.CreatedAt, &task.UpdatedAt,
			&completedAt,
		)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found: %s", taskID)
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// Deserialize JSON fields
	if len(dependencies) > 0 {
		if err := json.Unmarshal(dependencies, &task.Dependencies); err != nil {
			return nil, fmt.Errorf("failed to unmarshal dependencies: %w", err)
		}
	}

	if len(input) > 0 {
		if err := json.Unmarshal(input, &task.Input); err != nil {
			return nil, fmt.Errorf("failed to unmarshal input: %w", err)
		}
	}

	if len(output) > 0 {
		if err := json.Unmarshal(output, &task.Output); err != nil {
			return nil, fmt.Errorf("failed to unmarshal output: %w", err)
		}
	}

	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &task.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}

	return task, nil
}

// UpdateTask updates an existing task
func (pl *PostgresLedger) UpdateTask(ctx context.Context, task *Task) error {
	// Serialize JSON fields
	dependencies, err := json.Marshal(task.Dependencies)
	if err != nil {
		return fmt.Errorf("failed to marshal dependencies: %w", err)
	}

	input, err := json.Marshal(task.Input)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	output, err := json.Marshal(task.Output)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	metadata, err := json.Marshal(task.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	task.UpdatedAt = time.Now()

	// Use prepared statement if available
	if pl.stmtUpdateTask != nil {
		_, err = pl.stmtUpdateTask.ExecContext(ctx,
			task.ID, task.Name, task.Description, task.Type, task.Priority,
			task.AssignedTo, dependencies, input, output, task.Status,
			task.Error, metadata, task.UpdatedAt, task.CompletedAt,
		)
	} else {
		query := fmt.Sprintf(`
			UPDATE %s SET
				name = $2, description = $3, type = $4, priority = $5,
				assigned_to = $6, dependencies = $7, input = $8, output = $9,
				status = $10, error = $11, metadata = $12, updated_at = $13,
				completed_at = $14
			WHERE id = $1
		`, pl.config.TasksTable)

		_, err = pl.db.ExecContext(ctx, query,
			task.ID, task.Name, task.Description, task.Type, task.Priority,
			task.AssignedTo, dependencies, input, output, task.Status,
			task.Error, metadata, task.UpdatedAt, task.CompletedAt,
		)
	}

	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	return nil
}

// CompleteTask marks a task as completed
func (pl *PostgresLedger) CompleteTask(ctx context.Context, taskID string, result interface{}) error {
	output, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	now := time.Now()
	query := fmt.Sprintf(`
		UPDATE %s SET
			status = $2, output = $3, updated_at = $4, completed_at = $5
		WHERE id = $1
	`, pl.config.TasksTable)

	_, err = pl.db.ExecContext(ctx, query,
		taskID, TaskStatusCompleted, output, now, now,
	)

	if err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}

	return nil
}

// FailTask marks a task as failed
func (pl *PostgresLedger) FailTask(ctx context.Context, taskID string, errMsg string) error {
	now := time.Now()
	query := fmt.Sprintf(`
		UPDATE %s SET
			status = $2, error = $3, updated_at = $4, completed_at = $5
		WHERE id = $1
	`, pl.config.TasksTable)

	_, err := pl.db.ExecContext(ctx, query,
		taskID, TaskStatusFailed, errMsg, now, now,
	)

	if err != nil {
		return fmt.Errorf("failed to fail task: %w", err)
	}

	return nil
}

// DeleteTask deletes a task
func (pl *PostgresLedger) DeleteTask(ctx context.Context, taskID string) error {
	var err error
	if pl.stmtDeleteTask != nil {
		_, err = pl.stmtDeleteTask.ExecContext(ctx, taskID)
	} else {
		query := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, pl.config.TasksTable)
		_, err = pl.db.ExecContext(ctx, query, taskID)
	}

	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	return nil
}

// ListTasks lists tasks matching the filter
func (pl *PostgresLedger) ListTasks(ctx context.Context, filter *TaskFilter) ([]*Task, error) {
	query := fmt.Sprintf(`
		SELECT id, name, description, type, priority, assigned_to, created_by,
		       dependencies, input, output, status, error, metadata,
		       created_at, updated_at, completed_at
		FROM %s
	`, pl.config.TasksTable)

	var conditions []string
	var args []interface{}
	argPos := 1

	// Build WHERE clause
	if filter != nil {
		if len(filter.Statuses) > 0 {
			placeholders := make([]string, len(filter.Statuses))
			for i, status := range filter.Statuses {
				placeholders[i] = fmt.Sprintf("$%d", argPos)
				args = append(args, status)
				argPos++
			}
			conditions = append(conditions, fmt.Sprintf("status IN (%s)", strings.Join(placeholders, ",")))
		}

		if filter.AssignedTo != "" {
			conditions = append(conditions, fmt.Sprintf("assigned_to = $%d", argPos))
			args = append(args, filter.AssignedTo)
			argPos++
		}

		if filter.CreatedBy != "" {
			conditions = append(conditions, fmt.Sprintf("created_by = $%d", argPos))
			args = append(args, filter.CreatedBy)
			argPos++
		}

		if filter.CreatedAfter != nil {
			conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argPos))
			args = append(args, filter.CreatedAfter)
			argPos++
		}

		if filter.CreatedBefore != nil {
			conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argPos))
			args = append(args, filter.CreatedBefore)
			argPos++
		}

		if filter.UpdatedAfter != nil {
			conditions = append(conditions, fmt.Sprintf("updated_at >= $%d", argPos))
			args = append(args, filter.UpdatedAfter)
			argPos++
		}

		if filter.UpdatedBefore != nil {
			conditions = append(conditions, fmt.Sprintf("updated_at <= $%d", argPos))
			args = append(args, filter.UpdatedBefore)
			argPos++
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Add sorting
	if filter != nil && filter.SortBy != "" {
		order := "DESC"
		if filter.SortOrder == "asc" {
			order = "ASC"
		}
		query += fmt.Sprintf(" ORDER BY %s %s", filter.SortBy, order)
	} else {
		query += " ORDER BY created_at DESC"
	}

	// Add pagination
	if filter != nil && filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filter.Limit)
		argPos++

		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", argPos)
			args = append(args, filter.Offset)
		}
	}

	// Execute query
	rows, err := pl.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		task := &Task{}
		var dependencies, input, output, metadata []byte
		var completedAt sql.NullTime

		err := rows.Scan(
			&task.ID, &task.Name, &task.Description, &task.Type, &task.Priority,
			&task.AssignedTo, &task.CreatedBy, &dependencies, &input, &output,
			&task.Status, &task.Error, &metadata, &task.CreatedAt, &task.UpdatedAt,
			&completedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		// Deserialize JSON fields
		if len(dependencies) > 0 {
			json.Unmarshal(dependencies, &task.Dependencies)
		}
		if len(input) > 0 {
			json.Unmarshal(input, &task.Input)
		}
		if len(output) > 0 {
			json.Unmarshal(output, &task.Output)
		}
		if len(metadata) > 0 {
			json.Unmarshal(metadata, &task.Metadata)
		}
		if completedAt.Valid {
			task.CompletedAt = &completedAt.Time
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tasks: %w", err)
	}

	return tasks, nil
}

// RecordProgress records a progress update
func (pl *PostgresLedger) RecordProgress(ctx context.Context, progress *ProgressUpdate) error {
	if progress.RecordedAt.IsZero() {
		progress.RecordedAt = time.Now()
	}

	metadata, err := json.Marshal(progress.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if pl.stmtRecordProgress != nil {
		_, err = pl.stmtRecordProgress.ExecContext(ctx,
			progress.TaskID, progress.AgentID, progress.Status,
			progress.Message, metadata, progress.RecordedAt,
		)
	} else {
		query := fmt.Sprintf(`
			INSERT INTO %s (task_id, agent_id, status, message, metadata, recorded_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, pl.config.ProgressTable)

		_, err = pl.db.ExecContext(ctx, query,
			progress.TaskID, progress.AgentID, progress.Status,
			progress.Message, metadata, progress.RecordedAt,
		)
	}

	if err != nil {
		return fmt.Errorf("failed to record progress: %w", err)
	}

	return nil
}

// GetProgress retrieves all progress updates for a task
func (pl *PostgresLedger) GetProgress(ctx context.Context, taskID string) ([]*ProgressUpdate, error) {
	query := fmt.Sprintf(`
		SELECT id, task_id, agent_id, status, message, metadata, recorded_at
		FROM %s
		WHERE task_id = $1
		ORDER BY recorded_at ASC
	`, pl.config.ProgressTable)

	rows, err := pl.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get progress: %w", err)
	}
	defer rows.Close()

	var updates []*ProgressUpdate
	for rows.Next() {
		update := &ProgressUpdate{}
		var metadata []byte

		err := rows.Scan(
			&update.ID, &update.TaskID, &update.AgentID, &update.Status,
			&update.Message, &metadata, &update.RecordedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan progress: %w", err)
		}

		if len(metadata) > 0 {
			json.Unmarshal(metadata, &update.Metadata)
		}

		updates = append(updates, update)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating progress: %w", err)
	}

	return updates, nil
}

// GetLatestProgress retrieves the latest progress update for a task
func (pl *PostgresLedger) GetLatestProgress(ctx context.Context, taskID string) (*ProgressUpdate, error) {
	query := fmt.Sprintf(`
		SELECT id, task_id, agent_id, status, message, metadata, recorded_at
		FROM %s
		WHERE task_id = $1
		ORDER BY recorded_at DESC
		LIMIT 1
	`, pl.config.ProgressTable)

	update := &ProgressUpdate{}
	var metadata []byte

	err := pl.db.QueryRowContext(ctx, query, taskID).Scan(
		&update.ID, &update.TaskID, &update.AgentID, &update.Status,
		&update.Message, &metadata, &update.RecordedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No progress yet
		}
		return nil, fmt.Errorf("failed to get latest progress: %w", err)
	}

	if len(metadata) > 0 {
		json.Unmarshal(metadata, &update.Metadata)
	}

	return update, nil
}

// PurgeCompletedTasks deletes completed tasks older than the specified duration
func (pl *PostgresLedger) PurgeCompletedTasks(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE status IN ($1, $2, $3) AND completed_at < $4
	`, pl.config.TasksTable)

	result, err := pl.db.ExecContext(ctx, query,
		TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled, cutoff,
	)

	if err != nil {
		return 0, fmt.Errorf("failed to purge tasks: %w", err)
	}

	count, _ := result.RowsAffected()
	return count, nil
}

// PurgeProgress deletes progress updates older than the specified duration
func (pl *PostgresLedger) PurgeProgress(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	query := fmt.Sprintf(`
		DELETE FROM %s WHERE recorded_at < $1
	`, pl.config.ProgressTable)

	result, err := pl.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to purge progress: %w", err)
	}

	count, _ := result.RowsAffected()
	return count, nil
}

// Health checks database connection health
func (pl *PostgresLedger) Health(ctx context.Context) error {
	return pl.db.PingContext(ctx)
}

// Stats returns ledger statistics
func (pl *PostgresLedger) Stats(ctx context.Context) (*LedgerStats, error) {
	stats := &LedgerStats{}

	// Get task counts
	query := fmt.Sprintf(`
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN status = $1 THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = $2 THEN 1 ELSE 0 END) as in_progress,
			SUM(CASE WHEN status = $3 THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN status = $4 THEN 1 ELSE 0 END) as failed,
			MIN(created_at) as oldest,
			MAX(created_at) as newest
		FROM %s
	`, pl.config.TasksTable)

	var oldest, newest sql.NullTime
	err := pl.db.QueryRowContext(ctx, query,
		TaskStatusPending, TaskStatusInProgress, TaskStatusCompleted, TaskStatusFailed,
	).Scan(
		&stats.TotalTasks, &stats.PendingTasks, &stats.InProgressTasks,
		&stats.CompletedTasks, &stats.FailedTasks, &oldest, &newest,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get task stats: %w", err)
	}

	if oldest.Valid {
		stats.OldestTask = oldest.Time
	}
	if newest.Valid {
		stats.NewestTask = newest.Time
	}

	// Get progress count
	progressQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, pl.config.ProgressTable)
	err = pl.db.QueryRowContext(ctx, progressQuery).Scan(&stats.TotalProgress)
	if err != nil {
		return nil, fmt.Errorf("failed to get progress stats: %w", err)
	}

	return stats, nil
}

// Close closes the database connection
func (pl *PostgresLedger) Close() error {
	// Close prepared statements
	if pl.stmtCreateTask != nil {
		pl.stmtCreateTask.Close()
	}
	if pl.stmtGetTask != nil {
		pl.stmtGetTask.Close()
	}
	if pl.stmtUpdateTask != nil {
		pl.stmtUpdateTask.Close()
	}
	if pl.stmtDeleteTask != nil {
		pl.stmtDeleteTask.Close()
	}
	if pl.stmtRecordProgress != nil {
		pl.stmtRecordProgress.Close()
	}

	return pl.db.Close()
}
