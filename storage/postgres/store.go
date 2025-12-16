package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/yourusername/minion/models"
	"github.com/yourusername/minion/storage"
)

// PostgresStore implements the storage.Store interface using PostgreSQL
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &PostgresStore{db: db}, nil
}

// NewPostgresStoreFromDB creates a store from an existing database connection
func NewPostgresStoreFromDB(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// Close closes the database connection
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// Begin starts a new transaction
func (s *PostgresStore) Begin(ctx context.Context) (storage.Transaction, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &PostgresTransaction{tx: tx}, nil
}

// Ping checks if the database connection is alive
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// ============================================================================
// AGENT CRUD OPERATIONS
// ============================================================================

// Create creates a new agent
func (s *PostgresStore) Create(ctx context.Context, agent *models.Agent) error {
	query := `
		INSERT INTO agents (
			id, name, description, personality, language, status,
			llm_provider, llm_model, temperature, max_tokens,
			behavior_name, capabilities, metadata,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
	`

	capabilitiesJSON, err := json.Marshal(agent.Capabilities)
	if err != nil {
		return fmt.Errorf("failed to marshal capabilities: %w", err)
	}

	metadataJSON, err := json.Marshal(agent.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, query,
		agent.ID,
		agent.Name,
		agent.Description,
		agent.Personality,
		agent.Language,
		agent.Status,
		agent.LLMProvider,
		agent.LLMModel,
		agent.Temperature,
		agent.MaxTokens,
		agent.BehaviorName,
		capabilitiesJSON,
		metadataJSON,
		agent.CreatedAt,
		agent.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	return nil
}

// Get retrieves an agent by ID
func (s *PostgresStore) Get(ctx context.Context, id string) (*models.Agent, error) {
	query := `
		SELECT
			id, name, description, personality, language, status,
			llm_provider, llm_model, temperature, max_tokens,
			behavior_name, capabilities, metadata,
			created_at, updated_at
		FROM agents
		WHERE id = $1
	`

	var agent models.Agent
	var capabilitiesJSON, metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&agent.ID,
		&agent.Name,
		&agent.Description,
		&agent.Personality,
		&agent.Language,
		&agent.Status,
		&agent.LLMProvider,
		&agent.LLMModel,
		&agent.Temperature,
		&agent.MaxTokens,
		&agent.BehaviorName,
		&capabilitiesJSON,
		&metadataJSON,
		&agent.CreatedAt,
		&agent.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	if err := json.Unmarshal(capabilitiesJSON, &agent.Capabilities); err != nil {
		return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &agent.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &agent, nil
}

// Update updates an existing agent
func (s *PostgresStore) Update(ctx context.Context, agent *models.Agent) error {
	query := `
		UPDATE agents SET
			name = $2,
			description = $3,
			personality = $4,
			language = $5,
			status = $6,
			llm_provider = $7,
			llm_model = $8,
			temperature = $9,
			max_tokens = $10,
			behavior_name = $11,
			capabilities = $12,
			metadata = $13,
			updated_at = $14
		WHERE id = $1
	`

	capabilitiesJSON, err := json.Marshal(agent.Capabilities)
	if err != nil {
		return fmt.Errorf("failed to marshal capabilities: %w", err)
	}

	metadataJSON, err := json.Marshal(agent.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	agent.UpdatedAt = time.Now()

	result, err := s.db.ExecContext(ctx, query,
		agent.ID,
		agent.Name,
		agent.Description,
		agent.Personality,
		agent.Language,
		agent.Status,
		agent.LLMProvider,
		agent.LLMModel,
		agent.Temperature,
		agent.MaxTokens,
		agent.BehaviorName,
		capabilitiesJSON,
		metadataJSON,
		agent.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("agent not found: %s", agent.ID)
	}

	return nil
}

// Delete deletes an agent
func (s *PostgresStore) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM agents WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("agent not found: %s", id)
	}

	return nil
}

// List lists agents with pagination and filtering
func (s *PostgresStore) List(ctx context.Context, filter *models.ListAgentsRequest) ([]*models.Agent, int, error) {
	// Build query with filters
	query := `
		SELECT
			id, name, description, personality, language, status,
			llm_provider, llm_model, temperature, max_tokens,
			behavior_name, capabilities, metadata,
			created_at, updated_at
		FROM agents
		WHERE 1=1
	`
	countQuery := `SELECT COUNT(*) FROM agents WHERE 1=1`
	args := []interface{}{}
	argPos := 1

	// Apply filters
	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argPos)
		countQuery += fmt.Sprintf(" AND status = $%d", argPos)
		args = append(args, *filter.Status)
		argPos++
	}

	if filter.BehaviorType != nil {
		query += fmt.Sprintf(" AND behavior_name = $%d", argPos)
		countQuery += fmt.Sprintf(" AND behavior_name = $%d", argPos)
		args = append(args, *filter.BehaviorType)
		argPos++
	}

	// Count total
	var total int
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count agents: %w", err)
	}

	// Apply pagination
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, filter.PageSize, filter.Offset)

	// Execute query
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list agents: %w", err)
	}
	defer rows.Close()

	agents := make([]*models.Agent, 0)
	for rows.Next() {
		var agent models.Agent
		var capabilitiesJSON, metadataJSON []byte

		err := rows.Scan(
			&agent.ID,
			&agent.Name,
			&agent.Description,
			&agent.Personality,
			&agent.Language,
			&agent.Status,
			&agent.LLMProvider,
			&agent.LLMModel,
			&agent.Temperature,
			&agent.MaxTokens,
			&agent.BehaviorName,
			&capabilitiesJSON,
			&metadataJSON,
			&agent.CreatedAt,
			&agent.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan agent: %w", err)
		}

		if err := json.Unmarshal(capabilitiesJSON, &agent.Capabilities); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &agent.Metadata); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		agents = append(agents, &agent)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating agents: %w", err)
	}

	return agents, total, nil
}

// FindByBehaviorType finds agents by behavior type
func (s *PostgresStore) FindByBehaviorType(ctx context.Context, behaviorType string) ([]*models.Agent, error) {
	filter := &models.ListAgentsRequest{
		BehaviorType: &behaviorType,
		PageSize:     100,
		Offset:       0,
	}
	agents, _, err := s.List(ctx, filter)
	return agents, err
}

// FindByStatus finds agents by status
func (s *PostgresStore) FindByStatus(ctx context.Context, status models.AgentStatus) ([]*models.Agent, error) {
	filter := &models.ListAgentsRequest{
		Status:   &status,
		PageSize: 100,
		Offset:   0,
	}
	agents, _, err := s.List(ctx, filter)
	return agents, err
}

// ============================================================================
// METRICS OPERATIONS
// ============================================================================

// GetMetrics retrieves metrics for an agent
func (s *PostgresStore) GetMetrics(ctx context.Context, agentID string) (*models.Metrics, error) {
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) as successful,
			COALESCE(SUM(CASE WHEN status = 'failure' THEN 1 ELSE 0 END), 0) as failed,
			COALESCE(AVG(duration_ms), 0) as avg_time,
			COALESCE(MAX(created_at), NOW()) as last_execution
		FROM activities
		WHERE agent_id = $1
	`

	var metrics models.Metrics
	metrics.AgentID = agentID

	var lastExec time.Time
	err := s.db.QueryRowContext(ctx, query, agentID).Scan(
		&metrics.SuccessfulExecutions,
		&metrics.FailedExecutions,
		&metrics.AverageExecutionTime,
		&lastExec,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	metrics.TotalExecutions = metrics.SuccessfulExecutions + metrics.FailedExecutions
	metrics.LastExecution = lastExec

	return &metrics, nil
}

// UpdateMetrics updates metrics (no-op for aggregated metrics)
func (s *PostgresStore) UpdateMetrics(ctx context.Context, metrics *models.Metrics) error {
	// Metrics are aggregated from activities, so this is a no-op
	return nil
}

// CreateMetrics creates metrics (no-op for aggregated metrics)
func (s *PostgresStore) CreateMetrics(ctx context.Context, metrics *models.Metrics) error {
	// Metrics are aggregated from activities, so this is a no-op
	return nil
}

// ============================================================================
// ACTIVITY OPERATIONS
// ============================================================================

// RecordActivity records an activity
func (s *PostgresStore) RecordActivity(ctx context.Context, activity *models.Activity) error {
	query := `
		INSERT INTO activities (
			id, agent_id, session_id, action, status,
			input, output, error, duration_ms, token_count, cost,
			tools_used, metadata, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
	`

	inputJSON, _ := json.Marshal(activity.Input)
	outputJSON, _ := json.Marshal(activity.Output)
	toolsJSON, _ := json.Marshal(activity.ToolsUsed)
	metadataJSON, _ := json.Marshal(activity.Metadata)

	var sessionID *string
	if activity.SessionID != "" {
		sessionID = &activity.SessionID
	}

	var errorStr *string
	if activity.Error != "" {
		errorStr = &activity.Error
	}

	_, err := s.db.ExecContext(ctx, query,
		activity.ID,
		activity.AgentID,
		sessionID,
		activity.Action,
		activity.Status,
		inputJSON,
		outputJSON,
		errorStr,
		activity.DurationMS,
		activity.TokenCount,
		activity.Cost,
		toolsJSON,
		metadataJSON,
		activity.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to record activity: %w", err)
	}

	return nil
}

// GetActivities retrieves activities for an agent
func (s *PostgresStore) GetActivities(ctx context.Context, agentID string, limit int) ([]*models.Activity, error) {
	query := `
		SELECT
			id, agent_id, COALESCE(session_id, ''), action, status,
			input, output, COALESCE(error, ''), duration_ms,
			COALESCE(token_count, 0), COALESCE(cost, 0),
			tools_used, metadata, created_at
		FROM activities
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := s.db.QueryContext(ctx, query, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get activities: %w", err)
	}
	defer rows.Close()

	activities := make([]*models.Activity, 0)
	for rows.Next() {
		var activity models.Activity
		var inputJSON, outputJSON, toolsJSON, metadataJSON []byte

		err := rows.Scan(
			&activity.ID,
			&activity.AgentID,
			&activity.SessionID,
			&activity.Action,
			&activity.Status,
			&inputJSON,
			&outputJSON,
			&activity.Error,
			&activity.DurationMS,
			&activity.TokenCount,
			&activity.Cost,
			&toolsJSON,
			&metadataJSON,
			&activity.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan activity: %w", err)
		}

		_ = json.Unmarshal(inputJSON, &activity.Input)
		_ = json.Unmarshal(outputJSON, &activity.Output)
		_ = json.Unmarshal(toolsJSON, &activity.ToolsUsed)
		_ = json.Unmarshal(metadataJSON, &activity.Metadata)

		activities = append(activities, &activity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating activities: %w", err)
	}

	return activities, nil
}

// GetActivityByID retrieves a specific activity
func (s *PostgresStore) GetActivityByID(ctx context.Context, id string) (*models.Activity, error) {
	query := `
		SELECT
			id, agent_id, COALESCE(session_id, ''), action, status,
			input, output, COALESCE(error, ''), duration_ms,
			COALESCE(token_count, 0), COALESCE(cost, 0),
			tools_used, metadata, created_at
		FROM activities
		WHERE id = $1
	`

	var activity models.Activity
	var inputJSON, outputJSON, toolsJSON, metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&activity.ID,
		&activity.AgentID,
		&activity.SessionID,
		&activity.Action,
		&activity.Status,
		&inputJSON,
		&outputJSON,
		&activity.Error,
		&activity.DurationMS,
		&activity.TokenCount,
		&activity.Cost,
		&toolsJSON,
		&metadataJSON,
		&activity.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("activity not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get activity: %w", err)
	}

	_ = json.Unmarshal(inputJSON, &activity.Input)
	_ = json.Unmarshal(outputJSON, &activity.Output)
	_ = json.Unmarshal(toolsJSON, &activity.ToolsUsed)
	_ = json.Unmarshal(metadataJSON, &activity.Metadata)

	return &activity, nil
}

// ============================================================================
// TRANSACTION IMPLEMENTATION
// ============================================================================

// PostgresTransaction implements storage.Transaction
type PostgresTransaction struct {
	tx *sql.Tx
}

func (t *PostgresTransaction) Commit() error {
	return t.tx.Commit()
}

func (t *PostgresTransaction) Rollback() error {
	return t.tx.Rollback()
}

// Transaction methods mirror the store methods but use tx instead of db
// For brevity, only showing the structure - full implementation would be similar

func (t *PostgresTransaction) Create(ctx context.Context, agent *models.Agent) error {
	// Similar to PostgresStore.Create but using t.tx
	return fmt.Errorf("not implemented")
}

func (t *PostgresTransaction) Get(ctx context.Context, id string) (*models.Agent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (t *PostgresTransaction) Update(ctx context.Context, agent *models.Agent) error {
	return fmt.Errorf("not implemented")
}

func (t *PostgresTransaction) Delete(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented")
}

func (t *PostgresTransaction) List(ctx context.Context, filter *models.ListAgentsRequest) ([]*models.Agent, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}

func (t *PostgresTransaction) FindByBehaviorType(ctx context.Context, behaviorType string) ([]*models.Agent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (t *PostgresTransaction) FindByStatus(ctx context.Context, status models.AgentStatus) ([]*models.Agent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (t *PostgresTransaction) GetMetrics(ctx context.Context, agentID string) (*models.Metrics, error) {
	return nil, fmt.Errorf("not implemented")
}

func (t *PostgresTransaction) UpdateMetrics(ctx context.Context, metrics *models.Metrics) error {
	return fmt.Errorf("not implemented")
}

func (t *PostgresTransaction) CreateMetrics(ctx context.Context, metrics *models.Metrics) error {
	return fmt.Errorf("not implemented")
}

func (t *PostgresTransaction) RecordActivity(ctx context.Context, activity *models.Activity) error {
	return fmt.Errorf("not implemented")
}

func (t *PostgresTransaction) GetActivities(ctx context.Context, agentID string, limit int) ([]*models.Activity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (t *PostgresTransaction) GetActivityByID(ctx context.Context, id string) (*models.Activity, error) {
	return nil, fmt.Errorf("not implemented")
}
