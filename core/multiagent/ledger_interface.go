package multiagent

import (
	"context"
	"time"
)

// LedgerBackend defines the interface for task and progress ledger storage
type LedgerBackend interface {
	// Task operations
	CreateTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, taskID string) (*Task, error)
	UpdateTask(ctx context.Context, task *Task) error
	CompleteTask(ctx context.Context, taskID string, result interface{}) error
	FailTask(ctx context.Context, taskID string, errMsg string) error
	DeleteTask(ctx context.Context, taskID string) error
	ListTasks(ctx context.Context, filter *TaskFilter) ([]*Task, error)

	// Progress operations
	RecordProgress(ctx context.Context, progress *ProgressUpdate) error
	GetProgress(ctx context.Context, taskID string) ([]*ProgressUpdate, error)
	GetLatestProgress(ctx context.Context, taskID string) (*ProgressUpdate, error)

	// Cleanup and maintenance
	PurgeCompletedTasks(ctx context.Context, olderThan time.Duration) (int64, error)
	PurgeProgress(ctx context.Context, olderThan time.Duration) (int64, error)

	// Health and metrics
	Health(ctx context.Context) error
	Stats(ctx context.Context) (*LedgerStats, error)

	// Lifecycle
	Close() error
}

// TaskFilter defines criteria for filtering tasks
type TaskFilter struct {
	// Status filters
	Statuses []TaskStatus

	// Assignment filters
	AssignedTo string
	CreatedBy  string

	// Time filters
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	UpdatedAfter  *time.Time
	UpdatedBefore *time.Time

	// Pagination
	Limit  int
	Offset int

	// Sorting
	SortBy    string // "created_at", "updated_at", "priority"
	SortOrder string // "asc", "desc"
}

// ProgressUpdate represents a progress update for a task
type ProgressUpdate struct {
	ID         int64                  `json:"id,omitempty"`
	TaskID     string                 `json:"task_id"`
	AgentID    string                 `json:"agent_id"`
	Status     TaskStatus             `json:"status"`
	Message    string                 `json:"message,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	RecordedAt time.Time              `json:"recorded_at"`
}

// LedgerStats contains statistics about the ledger
type LedgerStats struct {
	TotalTasks       int64     `json:"total_tasks"`
	PendingTasks     int64     `json:"pending_tasks"`
	InProgressTasks  int64     `json:"in_progress_tasks"`
	CompletedTasks   int64     `json:"completed_tasks"`
	FailedTasks      int64     `json:"failed_tasks"`
	TotalProgress    int64     `json:"total_progress"`
	OldestTask       time.Time `json:"oldest_task,omitempty"`
	NewestTask       time.Time `json:"newest_task,omitempty"`
	StorageSize      int64     `json:"storage_size,omitempty"` // in bytes
}

// LedgerType defines the type of ledger backend
type LedgerType string

const (
	LedgerTypeInMemory  LedgerType = "inmemory"
	LedgerTypePostgres  LedgerType = "postgres"
	LedgerTypeHybrid    LedgerType = "hybrid"
)

// LedgerConfig contains configuration for ledger creation
type LedgerConfig struct {
	Type LedgerType

	// PostgreSQL config
	PostgresConfig *PostgresLedgerConfig

	// Hybrid config (in-memory + persistent)
	EnableCache      bool
	CacheTTL         time.Duration

	// Auto-cleanup
	AutoPurgeEnabled bool
	PurgeInterval    time.Duration
	RetainDuration   time.Duration
}

// PostgresLedgerConfig configures PostgreSQL ledger backend
type PostgresLedgerConfig struct {
	// Connection
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SSLMode  string // disable, require, verify-ca, verify-full

	// Connection pooling
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// Performance
	BatchSize         int           // Bulk insert size
	PreparedStmts     bool          // Use prepared statements
	StatementTimeout  time.Duration // Query timeout

	// Schema
	TasksTable    string
	ProgressTable string
	SchemaName    string
}

// DefaultPostgresConfig returns default PostgreSQL configuration
func DefaultPostgresConfig() *PostgresLedgerConfig {
	return &PostgresLedgerConfig{
		Host:              "localhost",
		Port:              5432,
		Database:          "minion",
		User:              "minion",
		Password:          "minion",
		SSLMode:           "disable",
		MaxOpenConns:      25,
		MaxIdleConns:      5,
		ConnMaxLifetime:   5 * time.Minute,
		ConnMaxIdleTime:   1 * time.Minute,
		BatchSize:         100,
		PreparedStmts:     true,
		StatementTimeout:  30 * time.Second,
		TasksTable:        "tasks",
		ProgressTable:     "task_progress",
		SchemaName:        "public",
	}
}

// DefaultLedgerConfig returns default ledger configuration
func DefaultLedgerConfig() *LedgerConfig {
	return &LedgerConfig{
		Type:             LedgerTypeInMemory,
		EnableCache:      false,
		CacheTTL:         5 * time.Minute,
		AutoPurgeEnabled: false,
		PurgeInterval:    1 * time.Hour,
		RetainDuration:   7 * 24 * time.Hour, // 7 days
	}
}
