package snapshot

import (
	"context"
	"testing"
	"time"
)

func TestSQLiteSnapshotStore(t *testing.T) {
	// Create in-memory store for testing
	store, err := NewSQLiteSnapshotStore(SQLiteConfig{
		Path: ":memory:",
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Test Save
	snapshot := &ExecutionSnapshot{
		ExecutionID:    "exec-1",
		SequenceNum:    1,
		CheckpointType: CheckpointAgentStep,
		AgentID:        "agent-1",
		TaskID:         "task-1",
		SessionID:      "session-1",
		Input:          map[string]interface{}{"query": "test"},
		Output:         map[string]interface{}{"result": "success"},
	}

	err = store.Save(ctx, snapshot)
	if err != nil {
		t.Fatalf("Failed to save snapshot: %v", err)
	}

	if snapshot.ID == "" {
		t.Error("Expected snapshot ID to be set")
	}

	// Test Get
	retrieved, err := store.Get(ctx, snapshot.ID)
	if err != nil {
		t.Fatalf("Failed to get snapshot: %v", err)
	}

	if retrieved.ExecutionID != snapshot.ExecutionID {
		t.Errorf("Expected execution ID %s, got %s", snapshot.ExecutionID, retrieved.ExecutionID)
	}

	if retrieved.AgentID != snapshot.AgentID {
		t.Errorf("Expected agent ID %s, got %s", snapshot.AgentID, retrieved.AgentID)
	}

	// Test GetByExecution
	snapshots, err := store.GetByExecution(ctx, "exec-1")
	if err != nil {
		t.Fatalf("Failed to get by execution: %v", err)
	}

	if len(snapshots) != 1 {
		t.Errorf("Expected 1 snapshot, got %d", len(snapshots))
	}

	// Test GetLatest
	latest, err := store.GetLatest(ctx, "exec-1")
	if err != nil {
		t.Fatalf("Failed to get latest: %v", err)
	}

	if latest.SequenceNum != 1 {
		t.Errorf("Expected sequence 1, got %d", latest.SequenceNum)
	}
}

func TestSQLiteSnapshotStoreBatch(t *testing.T) {
	store, err := NewSQLiteSnapshotStore(SQLiteConfig{
		Path: ":memory:",
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create batch of snapshots
	snapshots := []*ExecutionSnapshot{
		{ExecutionID: "exec-batch", SequenceNum: 1, CheckpointType: CheckpointAgentStep, AgentID: "agent-1"},
		{ExecutionID: "exec-batch", SequenceNum: 2, CheckpointType: CheckpointToolCallStart, AgentID: "agent-1"},
		{ExecutionID: "exec-batch", SequenceNum: 3, CheckpointType: CheckpointToolCallEnd, AgentID: "agent-1"},
	}

	err = store.SaveBatch(ctx, snapshots)
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	// Verify all saved
	retrieved, err := store.GetByExecution(ctx, "exec-batch")
	if err != nil {
		t.Fatalf("Failed to get by execution: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 snapshots, got %d", len(retrieved))
	}

	// Verify order
	for i, snap := range retrieved {
		if snap.SequenceNum != int64(i+1) {
			t.Errorf("Expected sequence %d, got %d", i+1, snap.SequenceNum)
		}
	}
}

func TestSQLiteSnapshotStoreQuery(t *testing.T) {
	store, err := NewSQLiteSnapshotStore(SQLiteConfig{
		Path: ":memory:",
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create snapshots
	for i := 1; i <= 5; i++ {
		store.Save(ctx, &ExecutionSnapshot{
			ExecutionID:    "exec-query",
			SequenceNum:    int64(i),
			CheckpointType: CheckpointAgentStep,
			AgentID:        "agent-1",
		})
	}

	// Test query with limit
	result, err := store.Query(ctx, &SnapshotQuery{
		Filter:  SnapshotFilter{ExecutionID: "exec-query"},
		Limit:   3,
		OrderBy: "sequence_asc",
	})
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if len(result.Snapshots) != 3 {
		t.Errorf("Expected 3 snapshots, got %d", len(result.Snapshots))
	}

	if result.TotalCount != 5 {
		t.Errorf("Expected total count 5, got %d", result.TotalCount)
	}

	if !result.HasMore {
		t.Error("Expected HasMore to be true")
	}

	// Test query with offset
	result, err = store.Query(ctx, &SnapshotQuery{
		Filter:  SnapshotFilter{ExecutionID: "exec-query"},
		Limit:   3,
		Offset:  3,
		OrderBy: "sequence_asc",
	})
	if err != nil {
		t.Fatalf("Failed to query with offset: %v", err)
	}

	if len(result.Snapshots) != 2 {
		t.Errorf("Expected 2 snapshots with offset, got %d", len(result.Snapshots))
	}
}

func TestSQLiteSnapshotStoreExecutionSummary(t *testing.T) {
	store, err := NewSQLiteSnapshotStore(SQLiteConfig{
		Path: ":memory:",
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create snapshots with error
	store.Save(ctx, &ExecutionSnapshot{
		ExecutionID:    "exec-summary",
		SequenceNum:    1,
		CheckpointType: CheckpointAgentStep,
		AgentID:        "agent-1",
	})
	store.Save(ctx, &ExecutionSnapshot{
		ExecutionID:    "exec-summary",
		SequenceNum:    2,
		CheckpointType: CheckpointError,
		AgentID:        "agent-1",
		Error:          &ErrorSnapshot{Message: "test error"},
	})

	summary, err := store.GetExecutionSummary(ctx, "exec-summary")
	if err != nil {
		t.Fatalf("Failed to get summary: %v", err)
	}

	if summary.TotalSteps != 2 {
		t.Errorf("Expected 2 steps, got %d", summary.TotalSteps)
	}

	if summary.ErrorCount != 1 {
		t.Errorf("Expected 1 error, got %d", summary.ErrorCount)
	}

	if summary.AgentID != "agent-1" {
		t.Errorf("Expected agent ID agent-1, got %s", summary.AgentID)
	}
}

func TestSQLiteSnapshotStorePurge(t *testing.T) {
	store, err := NewSQLiteSnapshotStore(SQLiteConfig{
		Path: ":memory:",
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create snapshots
	store.Save(ctx, &ExecutionSnapshot{
		ExecutionID:    "exec-purge",
		SequenceNum:    1,
		CheckpointType: CheckpointAgentStep,
		Timestamp:      time.Now().Add(-48 * time.Hour), // 2 days old
	})
	store.Save(ctx, &ExecutionSnapshot{
		ExecutionID:    "exec-purge",
		SequenceNum:    2,
		CheckpointType: CheckpointAgentStep,
		Timestamp:      time.Now(), // Current
	})

	// Purge older than 1 day
	count, err := store.PurgeOlderThan(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to purge: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 purged, got %d", count)
	}

	// Verify remaining
	snapshots, _ := store.GetByExecution(ctx, "exec-purge")
	if len(snapshots) != 1 {
		t.Errorf("Expected 1 remaining, got %d", len(snapshots))
	}
}

func TestSQLiteSnapshotStoreStats(t *testing.T) {
	store, err := NewSQLiteSnapshotStore(SQLiteConfig{
		Path: ":memory:",
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create snapshots
	store.Save(ctx, &ExecutionSnapshot{ExecutionID: "exec-1", SequenceNum: 1, CheckpointType: CheckpointAgentStep})
	store.Save(ctx, &ExecutionSnapshot{ExecutionID: "exec-2", SequenceNum: 1, CheckpointType: CheckpointAgentStep})

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalSnapshots != 2 {
		t.Errorf("Expected 2 total snapshots, got %d", stats.TotalSnapshots)
	}

	if stats.TotalExecutions != 2 {
		t.Errorf("Expected 2 total executions, got %d", stats.TotalExecutions)
	}
}

func TestSQLiteSnapshotStoreGetByCheckpointType(t *testing.T) {
	store, err := NewSQLiteSnapshotStore(SQLiteConfig{
		Path: ":memory:",
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create snapshots with different checkpoint types
	store.Save(ctx, &ExecutionSnapshot{ExecutionID: "exec-cp", SequenceNum: 1, CheckpointType: CheckpointAgentStep})
	store.Save(ctx, &ExecutionSnapshot{ExecutionID: "exec-cp", SequenceNum: 2, CheckpointType: CheckpointToolCallStart})
	store.Save(ctx, &ExecutionSnapshot{ExecutionID: "exec-cp", SequenceNum: 3, CheckpointType: CheckpointAgentStep})

	// Query by checkpoint type
	snapshots, err := store.GetByCheckpointType(ctx, "exec-cp", CheckpointAgentStep)
	if err != nil {
		t.Fatalf("Failed to get by checkpoint type: %v", err)
	}

	if len(snapshots) != 2 {
		t.Errorf("Expected 2 agent step snapshots, got %d", len(snapshots))
	}
}
