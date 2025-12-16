package multiagent

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// LedgerFactory creates ledger instances based on configuration
type LedgerFactory struct {
	config *LedgerConfig
}

// NewLedgerFactory creates a new ledger factory
func NewLedgerFactory(config *LedgerConfig) *LedgerFactory {
	if config == nil {
		config = DefaultLedgerConfig()
	}

	return &LedgerFactory{
		config: config,
	}
}

// NewLedgerFactoryFromEnv creates a ledger factory from environment variables
func NewLedgerFactoryFromEnv() *LedgerFactory {
	config := &LedgerConfig{
		Type: LedgerType(getEnvOrDefaultString("LEDGER_TYPE", string(LedgerTypeInMemory))),
	}

	// Configure based on type
	switch config.Type {
	case LedgerTypePostgres:
		config.PostgresConfig = PostgresConfigFromEnv()
	case LedgerTypeInMemory:
		// No additional config needed
	case LedgerTypeHybrid:
		config.PostgresConfig = PostgresConfigFromEnv()
		config.EnableCache = getEnvOrDefaultBool("LEDGER_ENABLE_CACHE", true)
		config.CacheTTL = getEnvOrDefaultDuration("LEDGER_CACHE_TTL", 5*time.Minute)
	}

	// Auto-purge configuration
	config.AutoPurgeEnabled = getEnvOrDefaultBool("LEDGER_AUTO_PURGE", false)
	config.PurgeInterval = getEnvOrDefaultDuration("LEDGER_PURGE_INTERVAL", 1*time.Hour)
	config.RetainDuration = getEnvOrDefaultDuration("LEDGER_RETAIN_DURATION", 7*24*time.Hour)

	return NewLedgerFactory(config)
}

// CreateLedger creates a ledger instance based on configuration
func (lf *LedgerFactory) CreateLedger() (LedgerBackend, error) {
	switch lf.config.Type {
	case LedgerTypeInMemory:
		return lf.createInMemoryLedger()

	case LedgerTypePostgres:
		return lf.createPostgresLedger()

	case LedgerTypeHybrid:
		return lf.createHybridLedger()

	default:
		return nil, fmt.Errorf("unknown ledger type: %s", lf.config.Type)
	}
}

// createInMemoryLedger creates an in-memory ledger
func (lf *LedgerFactory) createInMemoryLedger() (LedgerBackend, error) {
	// Return the existing in-memory implementation
	// Note: Need to update ledger.go to implement LedgerBackend interface
	return NewInMemoryLedger(), nil
}

// createPostgresLedger creates a PostgreSQL ledger
func (lf *LedgerFactory) createPostgresLedger() (LedgerBackend, error) {
	config := lf.config.PostgresConfig
	if config == nil {
		config = DefaultPostgresConfig()
	}

	ledger, err := NewPostgresLedger(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL ledger: %w", err)
	}

	return ledger, nil
}

// createHybridLedger creates a hybrid ledger (in-memory cache + PostgreSQL)
func (lf *LedgerFactory) createHybridLedger() (LedgerBackend, error) {
	// Create PostgreSQL backend
	postgresLedger, err := lf.createPostgresLedger()
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL ledger for hybrid: %w", err)
	}

	// Create hybrid wrapper
	hybrid := &HybridLedger{
		postgres:  postgresLedger.(*PostgresLedger),
		cache:     make(map[string]*Task),
		cacheTTL:  lf.config.CacheTTL,
		cacheTime: make(map[string]time.Time),
	}

	return hybrid, nil
}

// Validate validates the ledger factory configuration
func (lf *LedgerFactory) Validate() error {
	switch lf.config.Type {
	case LedgerTypeInMemory:
		// Always valid
		return nil

	case LedgerTypePostgres:
		if lf.config.PostgresConfig == nil {
			return fmt.Errorf("PostgreSQL configuration required")
		}
		if lf.config.PostgresConfig.Host == "" {
			return fmt.Errorf("PostgreSQL host required")
		}
		if lf.config.PostgresConfig.Database == "" {
			return fmt.Errorf("PostgreSQL database required")
		}
		return nil

	case LedgerTypeHybrid:
		if lf.config.PostgresConfig == nil {
			return fmt.Errorf("PostgreSQL configuration required for hybrid ledger")
		}
		return nil

	default:
		return fmt.Errorf("unknown ledger type: %s", lf.config.Type)
	}
}

// PostgresConfigFromEnv creates PostgreSQL configuration from environment variables
func PostgresConfigFromEnv() *PostgresLedgerConfig {
	config := DefaultPostgresConfig()

	if host := os.Getenv("POSTGRES_HOST"); host != "" {
		config.Host = host
	}

	if port := os.Getenv("POSTGRES_PORT"); port != "" {
		if portNum, err := strconv.Atoi(port); err == nil {
			config.Port = portNum
		}
	}

	if db := os.Getenv("POSTGRES_DB"); db != "" {
		config.Database = db
	}

	if user := os.Getenv("POSTGRES_USER"); user != "" {
		config.User = user
	}

	if password := os.Getenv("POSTGRES_PASSWORD"); password != "" {
		config.Password = password
	}

	if sslMode := os.Getenv("POSTGRES_SSL_MODE"); sslMode != "" {
		config.SSLMode = sslMode
	}

	if maxConns := os.Getenv("POSTGRES_MAX_CONNS"); maxConns != "" {
		if num, err := strconv.Atoi(maxConns); err == nil {
			config.MaxOpenConns = num
		}
	}

	if idleConns := os.Getenv("POSTGRES_IDLE_CONNS"); idleConns != "" {
		if num, err := strconv.Atoi(idleConns); err == nil {
			config.MaxIdleConns = num
		}
	}

	if timeout := os.Getenv("POSTGRES_STATEMENT_TIMEOUT"); timeout != "" {
		if duration, err := time.ParseDuration(timeout); err == nil {
			config.StatementTimeout = duration
		}
	}

	return config
}

// Helper functions

func getEnvOrDefaultString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvOrDefaultBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvOrDefaultDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// HybridLedger combines in-memory cache with PostgreSQL persistence
type HybridLedger struct {
	postgres  *PostgresLedger
	cache     map[string]*Task
	cacheTime map[string]time.Time
	cacheTTL  time.Duration
}

// CreateTask creates a task in both cache and database
func (hl *HybridLedger) CreateTask(ctx context.Context, task *Task) error {
	// Write to PostgreSQL first
	if err := hl.postgres.CreateTask(ctx, task); err != nil {
		return err
	}

	// Update cache
	hl.cache[task.ID] = task
	hl.cacheTime[task.ID] = time.Now()

	return nil
}

// GetTask retrieves task from cache or database
func (hl *HybridLedger) GetTask(ctx context.Context, taskID string) (*Task, error) {
	// Check cache first
	if task, ok := hl.cache[taskID]; ok {
		// Check if cache is still valid
		if time.Since(hl.cacheTime[taskID]) < hl.cacheTTL {
			return task, nil
		}
		// Cache expired, remove
		delete(hl.cache, taskID)
		delete(hl.cacheTime, taskID)
	}

	// Fetch from database
	task, err := hl.postgres.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Update cache
	hl.cache[taskID] = task
	hl.cacheTime[taskID] = time.Now()

	return task, nil
}

// UpdateTask updates task in both cache and database
func (hl *HybridLedger) UpdateTask(ctx context.Context, task *Task) error {
	if err := hl.postgres.UpdateTask(ctx, task); err != nil {
		return err
	}

	hl.cache[task.ID] = task
	hl.cacheTime[task.ID] = time.Now()

	return nil
}

// CompleteTask marks task as completed
func (hl *HybridLedger) CompleteTask(ctx context.Context, taskID string, result interface{}) error {
	if err := hl.postgres.CompleteTask(ctx, taskID, result); err != nil {
		return err
	}

	// Invalidate cache
	delete(hl.cache, taskID)
	delete(hl.cacheTime, taskID)

	return nil
}

// FailTask marks task as failed
func (hl *HybridLedger) FailTask(ctx context.Context, taskID string, errMsg string) error {
	if err := hl.postgres.FailTask(ctx, taskID, errMsg); err != nil {
		return err
	}

	// Invalidate cache
	delete(hl.cache, taskID)
	delete(hl.cacheTime, taskID)

	return nil
}

// DeleteTask deletes task from both cache and database
func (hl *HybridLedger) DeleteTask(ctx context.Context, taskID string) error {
	if err := hl.postgres.DeleteTask(ctx, taskID); err != nil {
		return err
	}

	delete(hl.cache, taskID)
	delete(hl.cacheTime, taskID)

	return nil
}

// ListTasks lists tasks from database (no caching for lists)
func (hl *HybridLedger) ListTasks(ctx context.Context, filter *TaskFilter) ([]*Task, error) {
	return hl.postgres.ListTasks(ctx, filter)
}

// RecordProgress records progress update
func (hl *HybridLedger) RecordProgress(ctx context.Context, progress *ProgressUpdate) error {
	return hl.postgres.RecordProgress(ctx, progress)
}

// GetProgress retrieves progress updates
func (hl *HybridLedger) GetProgress(ctx context.Context, taskID string) ([]*ProgressUpdate, error) {
	return hl.postgres.GetProgress(ctx, taskID)
}

// GetLatestProgress retrieves latest progress update
func (hl *HybridLedger) GetLatestProgress(ctx context.Context, taskID string) (*ProgressUpdate, error) {
	return hl.postgres.GetLatestProgress(ctx, taskID)
}

// PurgeCompletedTasks purges old completed tasks
func (hl *HybridLedger) PurgeCompletedTasks(ctx context.Context, olderThan time.Duration) (int64, error) {
	count, err := hl.postgres.PurgeCompletedTasks(ctx, olderThan)
	if err != nil {
		return 0, err
	}

	// Clear entire cache (simpler than selective removal)
	hl.cache = make(map[string]*Task)
	hl.cacheTime = make(map[string]time.Time)

	return count, nil
}

// PurgeProgress purges old progress updates
func (hl *HybridLedger) PurgeProgress(ctx context.Context, olderThan time.Duration) (int64, error) {
	return hl.postgres.PurgeProgress(ctx, olderThan)
}

// Health checks database health
func (hl *HybridLedger) Health(ctx context.Context) error {
	return hl.postgres.Health(ctx)
}

// Stats returns ledger statistics
func (hl *HybridLedger) Stats(ctx context.Context) (*LedgerStats, error) {
	return hl.postgres.Stats(ctx)
}

// Close closes the database connection
func (hl *HybridLedger) Close() error {
	return hl.postgres.Close()
}
