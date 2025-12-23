// Package snapshot provides the SnapshotStore interface for persisting execution snapshots.
package snapshot

import (
	"context"
	"time"
)

// SnapshotStore defines the interface for persisting and retrieving execution snapshots.
type SnapshotStore interface {
	// Write operations

	// Save persists a single snapshot.
	Save(ctx context.Context, snapshot *ExecutionSnapshot) error

	// SaveBatch persists multiple snapshots efficiently.
	SaveBatch(ctx context.Context, snapshots []*ExecutionSnapshot) error

	// Read operations - by execution

	// GetByExecution returns all snapshots for an execution, ordered by sequence.
	GetByExecution(ctx context.Context, executionID string) ([]*ExecutionSnapshot, error)

	// GetByExecutionRange returns snapshots within a sequence range.
	GetByExecutionRange(ctx context.Context, executionID string, fromSeq, toSeq int64) ([]*ExecutionSnapshot, error)

	// Read operations - by time

	// GetByTimeRange returns snapshots within a time range.
	GetByTimeRange(ctx context.Context, from, to time.Time) ([]*ExecutionSnapshot, error)

	// Read operations - by checkpoint type

	// GetByCheckpointType returns snapshots of a specific checkpoint type within an execution.
	GetByCheckpointType(ctx context.Context, executionID string, cpType CheckpointType) ([]*ExecutionSnapshot, error)

	// Read operations - specific snapshot

	// Get retrieves a snapshot by ID.
	Get(ctx context.Context, snapshotID string) (*ExecutionSnapshot, error)

	// GetLatest retrieves the most recent snapshot for an execution.
	GetLatest(ctx context.Context, executionID string) (*ExecutionSnapshot, error)

	// GetAtSequence retrieves a snapshot at a specific sequence number.
	GetAtSequence(ctx context.Context, executionID string, seqNum int64) (*ExecutionSnapshot, error)

	// Query operations

	// Query executes a complex query with filters, pagination, and ordering.
	Query(ctx context.Context, query *SnapshotQuery) (*SnapshotQueryResult, error)

	// ListExecutions returns a list of unique execution IDs with summaries.
	ListExecutions(ctx context.Context, limit, offset int) ([]*ExecutionSummary, error)

	// GetExecutionSummary returns a summary for a specific execution.
	GetExecutionSummary(ctx context.Context, executionID string) (*ExecutionSummary, error)

	// Maintenance operations

	// PurgeOlderThan removes snapshots older than the specified age.
	PurgeOlderThan(ctx context.Context, age time.Duration) (int64, error)

	// PurgeExecution removes all snapshots for a specific execution.
	PurgeExecution(ctx context.Context, executionID string) (int64, error)

	// Stats returns statistics about the store.
	Stats(ctx context.Context) (*StoreStats, error)

	// Close closes the store and releases resources.
	Close() error
}

// SnapshotStoreOption is a function that configures a SnapshotStore.
type SnapshotStoreOption func(interface{})

// WithMaxSnapshots sets the maximum number of snapshots to retain.
func WithMaxSnapshots(max int) SnapshotStoreOption {
	return func(s interface{}) {
		if ms, ok := s.(interface{ SetMaxSnapshots(int) }); ok {
			ms.SetMaxSnapshots(max)
		}
	}
}

// WithRetentionPeriod sets how long to retain snapshots.
func WithRetentionPeriod(d time.Duration) SnapshotStoreOption {
	return func(s interface{}) {
		if rs, ok := s.(interface{ SetRetentionPeriod(time.Duration) }); ok {
			rs.SetRetentionPeriod(d)
		}
	}
}

// WithBatchSize sets the batch size for bulk operations.
func WithBatchSize(size int) SnapshotStoreOption {
	return func(s interface{}) {
		if bs, ok := s.(interface{ SetBatchSize(int) }); ok {
			bs.SetBatchSize(size)
		}
	}
}
