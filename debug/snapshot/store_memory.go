// Package snapshot provides an in-memory implementation of SnapshotStore.
package snapshot

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemorySnapshotStore is an in-memory implementation of SnapshotStore.
// Suitable for development, testing, and small-scale deployments.
type MemorySnapshotStore struct {
	mu sync.RWMutex

	// Primary storage
	snapshots map[string]*ExecutionSnapshot // snapshotID -> snapshot

	// Indexes for efficient querying
	byExecution map[string][]*ExecutionSnapshot // executionID -> ordered snapshots
	byAgent     map[string][]string             // agentID -> snapshotIDs
	byTask      map[string][]string             // taskID -> snapshotIDs
	byTime      []*ExecutionSnapshot            // ordered by timestamp

	// Configuration
	maxSnapshots    int
	retentionPeriod time.Duration
	batchSize       int

	// Statistics
	totalSaved  int64
	totalPurged int64
}

// NewMemorySnapshotStore creates a new in-memory snapshot store.
func NewMemorySnapshotStore(opts ...SnapshotStoreOption) *MemorySnapshotStore {
	s := &MemorySnapshotStore{
		snapshots:       make(map[string]*ExecutionSnapshot),
		byExecution:     make(map[string][]*ExecutionSnapshot),
		byAgent:         make(map[string][]string),
		byTask:          make(map[string][]string),
		byTime:          make([]*ExecutionSnapshot, 0),
		maxSnapshots:    100000, // Default 100k snapshots
		retentionPeriod: 24 * time.Hour * 7, // Default 7 days
		batchSize:       100,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// SetMaxSnapshots sets the maximum number of snapshots to retain.
func (s *MemorySnapshotStore) SetMaxSnapshots(max int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxSnapshots = max
}

// SetRetentionPeriod sets how long to retain snapshots.
func (s *MemorySnapshotStore) SetRetentionPeriod(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retentionPeriod = d
}

// SetBatchSize sets the batch size for bulk operations.
func (s *MemorySnapshotStore) SetBatchSize(size int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batchSize = size
}

// Save persists a single snapshot.
func (s *MemorySnapshotStore) Save(ctx context.Context, snapshot *ExecutionSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate ID if not set
	if snapshot.ID == "" {
		snapshot.ID = uuid.New().String()
	}

	// Set timestamp if not set
	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = time.Now()
	}

	// Store snapshot
	s.snapshots[snapshot.ID] = snapshot

	// Update execution index
	s.byExecution[snapshot.ExecutionID] = append(s.byExecution[snapshot.ExecutionID], snapshot)
	s.sortExecutionSnapshots(snapshot.ExecutionID)

	// Update agent index
	if snapshot.AgentID != "" {
		s.byAgent[snapshot.AgentID] = append(s.byAgent[snapshot.AgentID], snapshot.ID)
	}

	// Update task index
	if snapshot.TaskID != "" {
		s.byTask[snapshot.TaskID] = append(s.byTask[snapshot.TaskID], snapshot.ID)
	}

	// Update time index
	s.byTime = append(s.byTime, snapshot)
	s.sortByTime()

	s.totalSaved++

	// Evict if over limit
	if len(s.snapshots) > s.maxSnapshots {
		s.evictOldest()
	}

	return nil
}

// SaveBatch persists multiple snapshots efficiently.
func (s *MemorySnapshotStore) SaveBatch(ctx context.Context, snapshots []*ExecutionSnapshot) error {
	for _, snap := range snapshots {
		if err := s.Save(ctx, snap); err != nil {
			return err
		}
	}
	return nil
}

// GetByExecution returns all snapshots for an execution, ordered by sequence.
func (s *MemorySnapshotStore) GetByExecution(ctx context.Context, executionID string) ([]*ExecutionSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots, ok := s.byExecution[executionID]
	if !ok {
		return []*ExecutionSnapshot{}, nil
	}

	// Return a copy to prevent external modification
	result := make([]*ExecutionSnapshot, len(snapshots))
	copy(result, snapshots)
	return result, nil
}

// GetByExecutionRange returns snapshots within a sequence range.
func (s *MemorySnapshotStore) GetByExecutionRange(ctx context.Context, executionID string, fromSeq, toSeq int64) ([]*ExecutionSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots, ok := s.byExecution[executionID]
	if !ok {
		return []*ExecutionSnapshot{}, nil
	}

	var result []*ExecutionSnapshot
	for _, snap := range snapshots {
		if snap.SequenceNum >= fromSeq && snap.SequenceNum <= toSeq {
			result = append(result, snap)
		}
	}
	return result, nil
}

// GetByTimeRange returns snapshots within a time range.
func (s *MemorySnapshotStore) GetByTimeRange(ctx context.Context, from, to time.Time) ([]*ExecutionSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ExecutionSnapshot
	for _, snap := range s.byTime {
		if (snap.Timestamp.Equal(from) || snap.Timestamp.After(from)) &&
			(snap.Timestamp.Equal(to) || snap.Timestamp.Before(to)) {
			result = append(result, snap)
		}
	}
	return result, nil
}

// GetByCheckpointType returns snapshots of a specific checkpoint type within an execution.
func (s *MemorySnapshotStore) GetByCheckpointType(ctx context.Context, executionID string, cpType CheckpointType) ([]*ExecutionSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots, ok := s.byExecution[executionID]
	if !ok {
		return []*ExecutionSnapshot{}, nil
	}

	var result []*ExecutionSnapshot
	for _, snap := range snapshots {
		if snap.CheckpointType == cpType {
			result = append(result, snap)
		}
	}
	return result, nil
}

// Get retrieves a snapshot by ID.
func (s *MemorySnapshotStore) Get(ctx context.Context, snapshotID string) (*ExecutionSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap, ok := s.snapshots[snapshotID]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
	}
	return snap, nil
}

// GetLatest retrieves the most recent snapshot for an execution.
func (s *MemorySnapshotStore) GetLatest(ctx context.Context, executionID string) (*ExecutionSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots, ok := s.byExecution[executionID]
	if !ok || len(snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots found for execution: %s", executionID)
	}
	return snapshots[len(snapshots)-1], nil
}

// GetAtSequence retrieves a snapshot at a specific sequence number.
func (s *MemorySnapshotStore) GetAtSequence(ctx context.Context, executionID string, seqNum int64) (*ExecutionSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots, ok := s.byExecution[executionID]
	if !ok {
		return nil, fmt.Errorf("no snapshots found for execution: %s", executionID)
	}

	for _, snap := range snapshots {
		if snap.SequenceNum == seqNum {
			return snap, nil
		}
	}
	return nil, fmt.Errorf("snapshot not found at sequence %d for execution: %s", seqNum, executionID)
}

// Query executes a complex query with filters, pagination, and ordering.
func (s *MemorySnapshotStore) Query(ctx context.Context, query *SnapshotQuery) (*SnapshotQueryResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var candidates []*ExecutionSnapshot

	// Start with execution filter if specified
	if query.Filter.ExecutionID != "" {
		if snaps, ok := s.byExecution[query.Filter.ExecutionID]; ok {
			candidates = snaps
		}
	} else {
		// Use all snapshots
		for _, snap := range s.snapshots {
			candidates = append(candidates, snap)
		}
	}

	// Apply filters
	var filtered []*ExecutionSnapshot
	for _, snap := range candidates {
		if s.matchesFilter(snap, &query.Filter) {
			filtered = append(filtered, snap)
		}
	}

	// Sort
	s.sortSnapshots(filtered, query.OrderBy)

	// Count total before pagination
	totalCount := int64(len(filtered))

	// Apply pagination
	offset := query.Offset
	if offset > len(filtered) {
		offset = len(filtered)
	}
	filtered = filtered[offset:]

	limit := query.Limit
	if limit <= 0 {
		limit = 100 // Default limit
	}
	if limit > len(filtered) {
		limit = len(filtered)
	}
	filtered = filtered[:limit]

	return &SnapshotQueryResult{
		Snapshots:  filtered,
		TotalCount: totalCount,
		HasMore:    int64(query.Offset+limit) < totalCount,
	}, nil
}

// ListExecutions returns a list of unique execution IDs with summaries.
func (s *MemorySnapshotStore) ListExecutions(ctx context.Context, limit, offset int) ([]*ExecutionSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect unique execution IDs
	executionIDs := make([]string, 0, len(s.byExecution))
	for execID := range s.byExecution {
		executionIDs = append(executionIDs, execID)
	}

	// Sort by most recent first
	sort.Slice(executionIDs, func(i, j int) bool {
		snapsI := s.byExecution[executionIDs[i]]
		snapsJ := s.byExecution[executionIDs[j]]
		if len(snapsI) == 0 || len(snapsJ) == 0 {
			return false
		}
		return snapsI[len(snapsI)-1].Timestamp.After(snapsJ[len(snapsJ)-1].Timestamp)
	})

	// Apply pagination
	if offset > len(executionIDs) {
		offset = len(executionIDs)
	}
	executionIDs = executionIDs[offset:]

	if limit <= 0 {
		limit = 100
	}
	if limit > len(executionIDs) {
		limit = len(executionIDs)
	}
	executionIDs = executionIDs[:limit]

	// Build summaries
	summaries := make([]*ExecutionSummary, 0, len(executionIDs))
	for _, execID := range executionIDs {
		summary := s.buildSummary(execID)
		if summary != nil {
			summaries = append(summaries, summary)
		}
	}

	return summaries, nil
}

// GetExecutionSummary returns a summary for a specific execution.
func (s *MemorySnapshotStore) GetExecutionSummary(ctx context.Context, executionID string) (*ExecutionSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := s.buildSummary(executionID)
	if summary == nil {
		return nil, fmt.Errorf("execution not found: %s", executionID)
	}
	return summary, nil
}

// PurgeOlderThan removes snapshots older than the specified age.
func (s *MemorySnapshotStore) PurgeOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-age)
	var purged int64

	for id, snap := range s.snapshots {
		if snap.Timestamp.Before(cutoff) {
			s.removeSnapshot(id)
			purged++
		}
	}

	s.totalPurged += purged
	return purged, nil
}

// PurgeExecution removes all snapshots for a specific execution.
func (s *MemorySnapshotStore) PurgeExecution(ctx context.Context, executionID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshots, ok := s.byExecution[executionID]
	if !ok {
		return 0, nil
	}

	var purged int64
	for _, snap := range snapshots {
		s.removeSnapshot(snap.ID)
		purged++
	}

	delete(s.byExecution, executionID)
	s.totalPurged += purged
	return purged, nil
}

// Stats returns statistics about the store.
func (s *MemorySnapshotStore) Stats(ctx context.Context) (*StoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &StoreStats{
		TotalSnapshots:  int64(len(s.snapshots)),
		TotalExecutions: int64(len(s.byExecution)),
	}

	if len(s.byTime) > 0 {
		stats.OldestSnapshot = s.byTime[0].Timestamp
		stats.NewestSnapshot = s.byTime[len(s.byTime)-1].Timestamp
	}

	// Estimate storage size (rough approximation)
	stats.StorageSizeBytes = int64(len(s.snapshots)) * 2048 // ~2KB per snapshot estimate

	return stats, nil
}

// Close closes the store and releases resources.
func (s *MemorySnapshotStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshots = nil
	s.byExecution = nil
	s.byAgent = nil
	s.byTask = nil
	s.byTime = nil

	return nil
}

// Helper methods

func (s *MemorySnapshotStore) sortExecutionSnapshots(executionID string) {
	snapshots := s.byExecution[executionID]
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].SequenceNum < snapshots[j].SequenceNum
	})
}

func (s *MemorySnapshotStore) sortByTime() {
	sort.Slice(s.byTime, func(i, j int) bool {
		return s.byTime[i].Timestamp.Before(s.byTime[j].Timestamp)
	})
}

func (s *MemorySnapshotStore) sortSnapshots(snapshots []*ExecutionSnapshot, orderBy string) {
	switch orderBy {
	case "sequence_desc":
		sort.Slice(snapshots, func(i, j int) bool {
			return snapshots[i].SequenceNum > snapshots[j].SequenceNum
		})
	case "time_asc":
		sort.Slice(snapshots, func(i, j int) bool {
			return snapshots[i].Timestamp.Before(snapshots[j].Timestamp)
		})
	case "time_desc":
		sort.Slice(snapshots, func(i, j int) bool {
			return snapshots[i].Timestamp.After(snapshots[j].Timestamp)
		})
	default: // sequence_asc
		sort.Slice(snapshots, func(i, j int) bool {
			return snapshots[i].SequenceNum < snapshots[j].SequenceNum
		})
	}
}

func (s *MemorySnapshotStore) matchesFilter(snap *ExecutionSnapshot, filter *SnapshotFilter) bool {
	if filter.AgentID != "" && snap.AgentID != filter.AgentID {
		return false
	}
	if filter.TaskID != "" && snap.TaskID != filter.TaskID {
		return false
	}
	if filter.SessionID != "" && snap.SessionID != filter.SessionID {
		return false
	}
	if filter.CheckpointType != "" && snap.CheckpointType != filter.CheckpointType {
		return false
	}
	if len(filter.CheckpointTypes) > 0 {
		found := false
		for _, ct := range filter.CheckpointTypes {
			if snap.CheckpointType == ct {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if filter.FromTime != nil && snap.Timestamp.Before(*filter.FromTime) {
		return false
	}
	if filter.ToTime != nil && snap.Timestamp.After(*filter.ToTime) {
		return false
	}
	if filter.FromSequence != nil && snap.SequenceNum < *filter.FromSequence {
		return false
	}
	if filter.ToSequence != nil && snap.SequenceNum > *filter.ToSequence {
		return false
	}
	if filter.HasError != nil {
		hasError := snap.Error != nil
		if *filter.HasError != hasError {
			return false
		}
	}
	if filter.TraceID != "" && snap.TraceID != filter.TraceID {
		return false
	}
	return true
}

func (s *MemorySnapshotStore) buildSummary(executionID string) *ExecutionSummary {
	snapshots, ok := s.byExecution[executionID]
	if !ok || len(snapshots) == 0 {
		return nil
	}

	summary := &ExecutionSummary{
		ExecutionID:      executionID,
		TotalSteps:       len(snapshots),
		CheckpointCounts: make(map[CheckpointType]int),
		StartTime:        snapshots[0].Timestamp,
		EndTime:          snapshots[len(snapshots)-1].Timestamp,
	}

	summary.Duration = summary.EndTime.Sub(summary.StartTime)

	// Get agent ID from first snapshot
	if snapshots[0].AgentID != "" {
		summary.AgentID = snapshots[0].AgentID
	}

	// Count checkpoints and errors
	for _, snap := range snapshots {
		summary.CheckpointCounts[snap.CheckpointType]++
		if snap.Error != nil {
			summary.ErrorCount++
		}
	}

	// Determine status
	lastSnap := snapshots[len(snapshots)-1]
	switch lastSnap.CheckpointType {
	case CheckpointTaskCompleted:
		summary.Status = "completed"
		summary.FinalOutput = lastSnap.Output
	case CheckpointTaskFailed, CheckpointError:
		summary.Status = "failed"
		summary.FinalError = lastSnap.Error
	default:
		summary.Status = "running"
	}

	return summary
}

func (s *MemorySnapshotStore) evictOldest() {
	if len(s.byTime) == 0 {
		return
	}

	// Remove oldest 10% or at least 1
	removeCount := len(s.byTime) / 10
	if removeCount < 1 {
		removeCount = 1
	}

	for i := 0; i < removeCount && len(s.byTime) > 0; i++ {
		oldest := s.byTime[0]
		s.removeSnapshot(oldest.ID)
	}
}

func (s *MemorySnapshotStore) removeSnapshot(id string) {
	snap, ok := s.snapshots[id]
	if !ok {
		return
	}

	delete(s.snapshots, id)

	// Remove from execution index
	if snaps, ok := s.byExecution[snap.ExecutionID]; ok {
		for i, sn := range snaps {
			if sn.ID == id {
				s.byExecution[snap.ExecutionID] = append(snaps[:i], snaps[i+1:]...)
				break
			}
		}
	}

	// Remove from time index
	for i, sn := range s.byTime {
		if sn.ID == id {
			s.byTime = append(s.byTime[:i], s.byTime[i+1:]...)
			break
		}
	}

	// Remove from agent index
	if snap.AgentID != "" {
		if ids, ok := s.byAgent[snap.AgentID]; ok {
			for i, sid := range ids {
				if sid == id {
					s.byAgent[snap.AgentID] = append(ids[:i], ids[i+1:]...)
					break
				}
			}
		}
	}

	// Remove from task index
	if snap.TaskID != "" {
		if ids, ok := s.byTask[snap.TaskID]; ok {
			for i, sid := range ids {
				if sid == id {
					s.byTask[snap.TaskID] = append(ids[:i], ids[i+1:]...)
					break
				}
			}
		}
	}
}
