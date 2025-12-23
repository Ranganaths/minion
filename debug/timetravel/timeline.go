// Package timetravel provides time-travel debugging capabilities for execution replay and analysis.
package timetravel

import (
	"context"
	"fmt"
	"time"

	"github.com/Ranganaths/minion/debug/snapshot"
)

// ExecutionTimeline provides time-travel navigation over an execution.
type ExecutionTimeline struct {
	executionID string
	store       snapshot.SnapshotStore
	snapshots   []*snapshot.ExecutionSnapshot // Cached, ordered by sequence
	cursor      int                           // Current position (0-indexed)
	summary     *snapshot.ExecutionSummary
}

// NewExecutionTimeline creates a timeline for an execution.
func NewExecutionTimeline(ctx context.Context, store snapshot.SnapshotStore, executionID string) (*ExecutionTimeline, error) {
	snapshots, err := store.GetByExecution(ctx, executionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load execution snapshots: %w", err)
	}

	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots found for execution: %s", executionID)
	}

	t := &ExecutionTimeline{
		executionID: executionID,
		store:       store,
		snapshots:   snapshots,
		cursor:      len(snapshots) - 1, // Start at end (most recent)
	}

	t.summary = t.buildSummary()

	return t, nil
}

// ExecutionID returns the execution ID.
func (t *ExecutionTimeline) ExecutionID() string {
	return t.executionID
}

// Length returns the total number of snapshots.
func (t *ExecutionTimeline) Length() int {
	return len(t.snapshots)
}

// Position returns the current cursor position (0-indexed).
func (t *ExecutionTimeline) Position() int {
	return t.cursor
}

// Summary returns the execution summary.
func (t *ExecutionTimeline) Summary() *snapshot.ExecutionSummary {
	return t.summary
}

// Navigation methods

// Current returns the snapshot at current cursor position.
func (t *ExecutionTimeline) Current() *snapshot.ExecutionSnapshot {
	if t.cursor < 0 || t.cursor >= len(t.snapshots) {
		return nil
	}
	return t.snapshots[t.cursor]
}

// First moves to and returns the first snapshot.
func (t *ExecutionTimeline) First() *snapshot.ExecutionSnapshot {
	if len(t.snapshots) == 0 {
		return nil
	}
	t.cursor = 0
	return t.snapshots[0]
}

// Last moves to and returns the last snapshot.
func (t *ExecutionTimeline) Last() *snapshot.ExecutionSnapshot {
	if len(t.snapshots) == 0 {
		return nil
	}
	t.cursor = len(t.snapshots) - 1
	return t.snapshots[t.cursor]
}

// StepForward moves one step forward and returns the new current snapshot.
func (t *ExecutionTimeline) StepForward() *snapshot.ExecutionSnapshot {
	if t.cursor < len(t.snapshots)-1 {
		t.cursor++
	}
	return t.Current()
}

// StepBackward moves one step backward and returns the new current snapshot.
func (t *ExecutionTimeline) StepBackward() *snapshot.ExecutionSnapshot {
	if t.cursor > 0 {
		t.cursor--
	}
	return t.Current()
}

// StepForwardN moves N steps forward and returns the new current snapshot.
func (t *ExecutionTimeline) StepForwardN(n int) *snapshot.ExecutionSnapshot {
	t.cursor += n
	if t.cursor >= len(t.snapshots) {
		t.cursor = len(t.snapshots) - 1
	}
	return t.Current()
}

// StepBackwardN moves N steps backward and returns the new current snapshot.
func (t *ExecutionTimeline) StepBackwardN(n int) *snapshot.ExecutionSnapshot {
	t.cursor -= n
	if t.cursor < 0 {
		t.cursor = 0
	}
	return t.Current()
}

// JumpTo moves to specific sequence number and returns the snapshot.
func (t *ExecutionTimeline) JumpTo(seqNum int64) *snapshot.ExecutionSnapshot {
	for i, snap := range t.snapshots {
		if snap.SequenceNum == seqNum {
			t.cursor = i
			return snap
		}
	}
	return nil
}

// JumpToIndex moves to a specific index position.
func (t *ExecutionTimeline) JumpToIndex(index int) *snapshot.ExecutionSnapshot {
	if index < 0 {
		index = 0
	}
	if index >= len(t.snapshots) {
		index = len(t.snapshots) - 1
	}
	t.cursor = index
	return t.Current()
}

// JumpToTimestamp moves to the snapshot closest to the given timestamp.
func (t *ExecutionTimeline) JumpToTimestamp(ts time.Time) *snapshot.ExecutionSnapshot {
	if len(t.snapshots) == 0 {
		return nil
	}

	// Binary search for closest timestamp
	closest := 0
	minDiff := t.snapshots[0].Timestamp.Sub(ts)
	if minDiff < 0 {
		minDiff = -minDiff
	}

	for i, snap := range t.snapshots {
		diff := snap.Timestamp.Sub(ts)
		if diff < 0 {
			diff = -diff
		}
		if diff < minDiff {
			minDiff = diff
			closest = i
		}
	}

	t.cursor = closest
	return t.Current()
}

// Checkpoint navigation

// JumpToNextCheckpoint moves to the next checkpoint of the given type.
func (t *ExecutionTimeline) JumpToNextCheckpoint(cpType snapshot.CheckpointType) *snapshot.ExecutionSnapshot {
	for i := t.cursor + 1; i < len(t.snapshots); i++ {
		if t.snapshots[i].CheckpointType == cpType {
			t.cursor = i
			return t.snapshots[i]
		}
	}
	return nil
}

// JumpToPrevCheckpoint moves to the previous checkpoint of the given type.
func (t *ExecutionTimeline) JumpToPrevCheckpoint(cpType snapshot.CheckpointType) *snapshot.ExecutionSnapshot {
	for i := t.cursor - 1; i >= 0; i-- {
		if t.snapshots[i].CheckpointType == cpType {
			t.cursor = i
			return t.snapshots[i]
		}
	}
	return nil
}

// JumpToNextError moves to the next error snapshot.
func (t *ExecutionTimeline) JumpToNextError() *snapshot.ExecutionSnapshot {
	for i := t.cursor + 1; i < len(t.snapshots); i++ {
		if t.snapshots[i].Error != nil {
			t.cursor = i
			return t.snapshots[i]
		}
	}
	return nil
}

// JumpToPrevError moves to the previous error snapshot.
func (t *ExecutionTimeline) JumpToPrevError() *snapshot.ExecutionSnapshot {
	for i := t.cursor - 1; i >= 0; i-- {
		if t.snapshots[i].Error != nil {
			t.cursor = i
			return t.snapshots[i]
		}
	}
	return nil
}

// JumpToNextLLMCall moves to the next LLM call checkpoint.
func (t *ExecutionTimeline) JumpToNextLLMCall() *snapshot.ExecutionSnapshot {
	for i := t.cursor + 1; i < len(t.snapshots); i++ {
		if t.snapshots[i].CheckpointType == snapshot.CheckpointLLMCallStart ||
			t.snapshots[i].CheckpointType == snapshot.CheckpointLLMCallEnd {
			t.cursor = i
			return t.snapshots[i]
		}
	}
	return nil
}

// JumpToNextToolCall moves to the next tool call checkpoint.
func (t *ExecutionTimeline) JumpToNextToolCall() *snapshot.ExecutionSnapshot {
	for i := t.cursor + 1; i < len(t.snapshots); i++ {
		if t.snapshots[i].CheckpointType == snapshot.CheckpointToolCallStart ||
			t.snapshots[i].CheckpointType == snapshot.CheckpointToolCallEnd {
			t.cursor = i
			return t.snapshots[i]
		}
	}
	return nil
}

// Query methods

// GetRange returns snapshots between two sequence numbers (inclusive).
func (t *ExecutionTimeline) GetRange(fromSeq, toSeq int64) []*snapshot.ExecutionSnapshot {
	var result []*snapshot.ExecutionSnapshot
	for _, snap := range t.snapshots {
		if snap.SequenceNum >= fromSeq && snap.SequenceNum <= toSeq {
			result = append(result, snap)
		}
	}
	return result
}

// GetIndexRange returns snapshots between two indices (inclusive).
func (t *ExecutionTimeline) GetIndexRange(fromIdx, toIdx int) []*snapshot.ExecutionSnapshot {
	if fromIdx < 0 {
		fromIdx = 0
	}
	if toIdx >= len(t.snapshots) {
		toIdx = len(t.snapshots) - 1
	}
	if fromIdx > toIdx {
		return nil
	}
	return t.snapshots[fromIdx : toIdx+1]
}

// GetByType returns all snapshots of a given checkpoint type.
func (t *ExecutionTimeline) GetByType(cpType snapshot.CheckpointType) []*snapshot.ExecutionSnapshot {
	var result []*snapshot.ExecutionSnapshot
	for _, snap := range t.snapshots {
		if snap.CheckpointType == cpType {
			result = append(result, snap)
		}
	}
	return result
}

// GetByTypes returns all snapshots matching any of the given checkpoint types.
func (t *ExecutionTimeline) GetByTypes(cpTypes ...snapshot.CheckpointType) []*snapshot.ExecutionSnapshot {
	typeSet := make(map[snapshot.CheckpointType]bool)
	for _, ct := range cpTypes {
		typeSet[ct] = true
	}

	var result []*snapshot.ExecutionSnapshot
	for _, snap := range t.snapshots {
		if typeSet[snap.CheckpointType] {
			result = append(result, snap)
		}
	}
	return result
}

// GetErrors returns all error snapshots.
func (t *ExecutionTimeline) GetErrors() []*snapshot.ExecutionSnapshot {
	var result []*snapshot.ExecutionSnapshot
	for _, snap := range t.snapshots {
		if snap.Error != nil {
			result = append(result, snap)
		}
	}
	return result
}

// GetLLMCalls returns all LLM call snapshots.
func (t *ExecutionTimeline) GetLLMCalls() []*snapshot.ExecutionSnapshot {
	return t.GetByTypes(snapshot.CheckpointLLMCallStart, snapshot.CheckpointLLMCallEnd)
}

// GetToolCalls returns all tool call snapshots.
func (t *ExecutionTimeline) GetToolCalls() []*snapshot.ExecutionSnapshot {
	return t.GetByTypes(snapshot.CheckpointToolCallStart, snapshot.CheckpointToolCallEnd)
}

// GetTaskSnapshots returns all task-related snapshots.
func (t *ExecutionTimeline) GetTaskSnapshots() []*snapshot.ExecutionSnapshot {
	return t.GetByTypes(
		snapshot.CheckpointTaskCreated,
		snapshot.CheckpointTaskAssigned,
		snapshot.CheckpointTaskStarted,
		snapshot.CheckpointTaskCompleted,
		snapshot.CheckpointTaskFailed,
		snapshot.CheckpointTaskRetry,
	)
}

// GetByTask returns all snapshots for a specific task ID.
func (t *ExecutionTimeline) GetByTask(taskID string) []*snapshot.ExecutionSnapshot {
	var result []*snapshot.ExecutionSnapshot
	for _, snap := range t.snapshots {
		if snap.TaskID == taskID {
			result = append(result, snap)
		}
	}
	return result
}

// GetByAgent returns all snapshots for a specific agent ID.
func (t *ExecutionTimeline) GetByAgent(agentID string) []*snapshot.ExecutionSnapshot {
	var result []*snapshot.ExecutionSnapshot
	for _, snap := range t.snapshots {
		if snap.AgentID == agentID {
			result = append(result, snap)
		}
	}
	return result
}

// Status methods

// CanStepForward returns true if we can step forward.
func (t *ExecutionTimeline) CanStepForward() bool {
	return t.cursor < len(t.snapshots)-1
}

// CanStepBackward returns true if we can step backward.
func (t *ExecutionTimeline) CanStepBackward() bool {
	return t.cursor > 0
}

// IsAtStart returns true if at the first snapshot.
func (t *ExecutionTimeline) IsAtStart() bool {
	return t.cursor == 0
}

// IsAtEnd returns true if at the last snapshot.
func (t *ExecutionTimeline) IsAtEnd() bool {
	return t.cursor == len(t.snapshots)-1
}

// Progress returns the current progress as a percentage (0-100).
func (t *ExecutionTimeline) Progress() float64 {
	if len(t.snapshots) <= 1 {
		return 100.0
	}
	return float64(t.cursor) / float64(len(t.snapshots)-1) * 100.0
}

// Analysis methods

// Duration returns total execution duration.
func (t *ExecutionTimeline) Duration() time.Duration {
	if len(t.snapshots) < 2 {
		return 0
	}
	first := t.snapshots[0].Timestamp
	last := t.snapshots[len(t.snapshots)-1].Timestamp
	return last.Sub(first)
}

// DurationUntilCurrent returns duration from start to current position.
func (t *ExecutionTimeline) DurationUntilCurrent() time.Duration {
	if len(t.snapshots) == 0 || t.cursor <= 0 {
		return 0
	}
	first := t.snapshots[0].Timestamp
	current := t.snapshots[t.cursor].Timestamp
	return current.Sub(first)
}

// GetTimeBetween returns the duration between two snapshots.
func (t *ExecutionTimeline) GetTimeBetween(fromSeq, toSeq int64) time.Duration {
	var fromSnap, toSnap *snapshot.ExecutionSnapshot
	for _, snap := range t.snapshots {
		if snap.SequenceNum == fromSeq {
			fromSnap = snap
		}
		if snap.SequenceNum == toSeq {
			toSnap = snap
		}
	}
	if fromSnap == nil || toSnap == nil {
		return 0
	}
	return toSnap.Timestamp.Sub(fromSnap.Timestamp)
}

// CountCheckpoints returns counts of each checkpoint type.
func (t *ExecutionTimeline) CountCheckpoints() map[snapshot.CheckpointType]int {
	counts := make(map[snapshot.CheckpointType]int)
	for _, snap := range t.snapshots {
		counts[snap.CheckpointType]++
	}
	return counts
}

// FindSlowestOperations returns snapshots with the longest durations.
func (t *ExecutionTimeline) FindSlowestOperations(limit int) []*snapshot.ExecutionSnapshot {
	// Find snapshots with action durations
	type snapWithDuration struct {
		snap     *snapshot.ExecutionSnapshot
		duration time.Duration
	}

	var withDurations []snapWithDuration
	for _, snap := range t.snapshots {
		if snap.Action != nil && snap.Action.DurationMs > 0 {
			withDurations = append(withDurations, snapWithDuration{
				snap:     snap,
				duration: time.Duration(snap.Action.DurationMs) * time.Millisecond,
			})
		}
	}

	// Sort by duration descending
	for i := 0; i < len(withDurations); i++ {
		for j := i + 1; j < len(withDurations); j++ {
			if withDurations[j].duration > withDurations[i].duration {
				withDurations[i], withDurations[j] = withDurations[j], withDurations[i]
			}
		}
	}

	// Return top N
	if limit > len(withDurations) {
		limit = len(withDurations)
	}

	result := make([]*snapshot.ExecutionSnapshot, limit)
	for i := 0; i < limit; i++ {
		result[i] = withDurations[i].snap
	}
	return result
}

// GetCriticalPath identifies the critical path through the execution.
func (t *ExecutionTimeline) GetCriticalPath() []*snapshot.ExecutionSnapshot {
	// Return major checkpoints: task starts/completions and errors
	criticalTypes := map[snapshot.CheckpointType]bool{
		snapshot.CheckpointTaskCreated:   true,
		snapshot.CheckpointTaskStarted:   true,
		snapshot.CheckpointTaskCompleted: true,
		snapshot.CheckpointTaskFailed:    true,
		snapshot.CheckpointError:         true,
		snapshot.CheckpointDecisionPoint: true,
	}

	var result []*snapshot.ExecutionSnapshot
	for _, snap := range t.snapshots {
		if criticalTypes[snap.CheckpointType] || snap.Error != nil {
			result = append(result, snap)
		}
	}
	return result
}

// Export methods

// All returns all snapshots.
func (t *ExecutionTimeline) All() []*snapshot.ExecutionSnapshot {
	result := make([]*snapshot.ExecutionSnapshot, len(t.snapshots))
	copy(result, t.snapshots)
	return result
}

// ToJSON exports the timeline as a serializable structure.
func (t *ExecutionTimeline) ToJSON() *TimelineExport {
	return &TimelineExport{
		ExecutionID: t.executionID,
		Summary:     t.summary,
		Snapshots:   t.snapshots,
		Position:    t.cursor,
	}
}

// TimelineExport is a serializable representation of a timeline.
type TimelineExport struct {
	ExecutionID string                       `json:"execution_id"`
	Summary     *snapshot.ExecutionSummary   `json:"summary"`
	Snapshots   []*snapshot.ExecutionSnapshot `json:"snapshots"`
	Position    int                          `json:"position"`
}

// Internal methods

func (t *ExecutionTimeline) buildSummary() *snapshot.ExecutionSummary {
	if len(t.snapshots) == 0 {
		return nil
	}

	summary := &snapshot.ExecutionSummary{
		ExecutionID:      t.executionID,
		TotalSteps:       len(t.snapshots),
		CheckpointCounts: make(map[snapshot.CheckpointType]int),
		StartTime:        t.snapshots[0].Timestamp,
		EndTime:          t.snapshots[len(t.snapshots)-1].Timestamp,
	}

	summary.Duration = summary.EndTime.Sub(summary.StartTime)

	// Get agent ID from first snapshot
	if t.snapshots[0].AgentID != "" {
		summary.AgentID = t.snapshots[0].AgentID
	}

	// Count checkpoints and errors
	for _, snap := range t.snapshots {
		summary.CheckpointCounts[snap.CheckpointType]++
		if snap.Error != nil {
			summary.ErrorCount++
		}
	}

	// Determine status
	lastSnap := t.snapshots[len(t.snapshots)-1]
	switch lastSnap.CheckpointType {
	case snapshot.CheckpointTaskCompleted:
		summary.Status = "completed"
		summary.FinalOutput = lastSnap.Output
	case snapshot.CheckpointTaskFailed, snapshot.CheckpointError:
		summary.Status = "failed"
		summary.FinalError = lastSnap.Error
	default:
		summary.Status = "running"
	}

	return summary
}

// Refresh reloads snapshots from the store.
func (t *ExecutionTimeline) Refresh(ctx context.Context) error {
	snapshots, err := t.store.GetByExecution(ctx, t.executionID)
	if err != nil {
		return fmt.Errorf("failed to refresh snapshots: %w", err)
	}

	t.snapshots = snapshots
	t.summary = t.buildSummary()

	// Keep cursor in bounds
	if t.cursor >= len(t.snapshots) {
		t.cursor = len(t.snapshots) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}

	return nil
}
