// Package timetravel provides state reconstruction capabilities.
package timetravel

import (
	"time"

	"github.com/Ranganaths/minion/debug/snapshot"
)

// StateReconstructor rebuilds execution state at any point in the timeline.
type StateReconstructor struct {
	timeline *ExecutionTimeline
}

// NewStateReconstructor creates a new state reconstructor for a timeline.
func NewStateReconstructor(timeline *ExecutionTimeline) *StateReconstructor {
	return &StateReconstructor{timeline: timeline}
}

// ReconstructedState represents the complete state at a point in execution.
type ReconstructedState struct {
	// Position in timeline
	SequenceNum int64     `json:"sequence_num"`
	Index       int       `json:"index"`
	Timestamp   time.Time `json:"timestamp"`

	// Snapshot at this point
	Snapshot *snapshot.ExecutionSnapshot `json:"snapshot"`

	// Reconstructed states
	Session   *snapshot.SessionSnapshot `json:"session,omitempty"`
	Task      *snapshot.TaskSnapshot    `json:"task,omitempty"`
	Workspace map[string]any            `json:"workspace,omitempty"`

	// Context
	AgentID   string `json:"agent_id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`

	// Trace context
	TraceID string `json:"trace_id,omitempty"`
	SpanID  string `json:"span_id,omitempty"`

	// History leading to this point
	PreviousActions []*snapshot.ActionSnapshot `json:"previous_actions,omitempty"`
	ErrorHistory    []*snapshot.ErrorSnapshot  `json:"error_history,omitempty"`
}

// ReconstructAt rebuilds the complete state at a given sequence number.
func (r *StateReconstructor) ReconstructAt(seqNum int64) (*ReconstructedState, error) {
	snap := r.timeline.JumpTo(seqNum)
	if snap == nil {
		return nil, nil
	}

	return r.reconstructFromSnapshot(snap, r.timeline.Position())
}

// ReconstructAtIndex rebuilds the complete state at a given index.
func (r *StateReconstructor) ReconstructAtIndex(index int) (*ReconstructedState, error) {
	snap := r.timeline.JumpToIndex(index)
	if snap == nil {
		return nil, nil
	}

	return r.reconstructFromSnapshot(snap, index)
}

// ReconstructCurrent rebuilds the state at the current timeline position.
func (r *StateReconstructor) ReconstructCurrent() (*ReconstructedState, error) {
	snap := r.timeline.Current()
	if snap == nil {
		return nil, nil
	}

	return r.reconstructFromSnapshot(snap, r.timeline.Position())
}

// ReconstructSessionAt rebuilds session state at given sequence number.
func (r *StateReconstructor) ReconstructSessionAt(seqNum int64) *snapshot.SessionSnapshot {
	snapshots := r.timeline.All()

	// Find the latest session snapshot at or before seqNum
	var sessionSnap *snapshot.SessionSnapshot

	for _, snap := range snapshots {
		if snap.SequenceNum > seqNum {
			break
		}
		if snap.SessionState != nil {
			sessionSnap = snap.SessionState
		}
	}

	return sessionSnap
}

// ReconstructTaskAt rebuilds task state at given sequence number.
func (r *StateReconstructor) ReconstructTaskAt(seqNum int64) *snapshot.TaskSnapshot {
	snapshots := r.timeline.All()

	// Find the latest task snapshot at or before seqNum
	var taskSnap *snapshot.TaskSnapshot

	for _, snap := range snapshots {
		if snap.SequenceNum > seqNum {
			break
		}
		if snap.TaskState != nil {
			taskSnap = snap.TaskState
		}
	}

	return taskSnap
}

// ReconstructWorkspaceAt rebuilds workspace state at given sequence number.
func (r *StateReconstructor) ReconstructWorkspaceAt(seqNum int64) map[string]any {
	snapshots := r.timeline.All()

	// Accumulate workspace changes
	workspace := make(map[string]any)

	for _, snap := range snapshots {
		if snap.SequenceNum > seqNum {
			break
		}
		if snap.WorkspaceState != nil {
			// Merge workspace state
			for k, v := range snap.WorkspaceState {
				workspace[k] = v
			}
		}
	}

	if len(workspace) == 0 {
		return nil
	}
	return workspace
}

// GetActionHistoryAt returns all actions taken up to the given sequence number.
func (r *StateReconstructor) GetActionHistoryAt(seqNum int64) []*snapshot.ActionSnapshot {
	snapshots := r.timeline.All()

	var actions []*snapshot.ActionSnapshot
	for _, snap := range snapshots {
		if snap.SequenceNum > seqNum {
			break
		}
		if snap.Action != nil {
			actions = append(actions, snap.Action)
		}
	}

	return actions
}

// GetErrorHistoryAt returns all errors up to the given sequence number.
func (r *StateReconstructor) GetErrorHistoryAt(seqNum int64) []*snapshot.ErrorSnapshot {
	snapshots := r.timeline.All()

	var errors []*snapshot.ErrorSnapshot
	for _, snap := range snapshots {
		if snap.SequenceNum > seqNum {
			break
		}
		if snap.Error != nil {
			errors = append(errors, snap.Error)
		}
	}

	return errors
}

// GetConversationAt reconstructs the conversation history at a given sequence number.
func (r *StateReconstructor) GetConversationAt(seqNum int64) []snapshot.Message {
	session := r.ReconstructSessionAt(seqNum)
	if session == nil {
		return nil
	}
	return session.History
}

// GetInputOutputHistoryAt returns input/output pairs up to the given sequence number.
func (r *StateReconstructor) GetInputOutputHistoryAt(seqNum int64) []InputOutputPair {
	snapshots := r.timeline.All()

	var pairs []InputOutputPair
	for _, snap := range snapshots {
		if snap.SequenceNum > seqNum {
			break
		}
		if snap.Input != nil || snap.Output != nil {
			pairs = append(pairs, InputOutputPair{
				SequenceNum:    snap.SequenceNum,
				CheckpointType: snap.CheckpointType,
				Input:          snap.Input,
				Output:         snap.Output,
				Timestamp:      snap.Timestamp,
			})
		}
	}

	return pairs
}

// InputOutputPair represents an input/output pair at a point in execution.
type InputOutputPair struct {
	SequenceNum    int64                  `json:"sequence_num"`
	CheckpointType snapshot.CheckpointType `json:"checkpoint_type"`
	Input          any                    `json:"input,omitempty"`
	Output         any                    `json:"output,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
}

// CompareStates compares two reconstructed states.
func (r *StateReconstructor) CompareStates(seqNum1, seqNum2 int64) (*StateComparison, error) {
	state1, err := r.ReconstructAt(seqNum1)
	if err != nil {
		return nil, err
	}

	state2, err := r.ReconstructAt(seqNum2)
	if err != nil {
		return nil, err
	}

	return r.compareReconstructedStates(state1, state2), nil
}

// StateComparison represents differences between two states.
type StateComparison struct {
	State1SeqNum int64 `json:"state1_sequence_num"`
	State2SeqNum int64 `json:"state2_sequence_num"`

	// Time between states
	TimeDelta time.Duration `json:"time_delta"`

	// Differences
	WorkspaceDiff   *WorkspaceDiff   `json:"workspace_diff,omitempty"`
	SessionDiff     *SessionDiff     `json:"session_diff,omitempty"`
	TaskDiff        *TaskDiff        `json:"task_diff,omitempty"`
	ActionsBetween  []*snapshot.ActionSnapshot `json:"actions_between,omitempty"`
	ErrorsBetween   []*snapshot.ErrorSnapshot  `json:"errors_between,omitempty"`
	SnapshotsBetween int `json:"snapshots_between"`
}

// WorkspaceDiff represents changes to workspace between two states.
type WorkspaceDiff struct {
	Added    map[string]any `json:"added,omitempty"`
	Removed  []string       `json:"removed,omitempty"`
	Modified map[string]ValueChange `json:"modified,omitempty"`
}

// ValueChange represents a changed value.
type ValueChange struct {
	Before any `json:"before"`
	After  any `json:"after"`
}

// SessionDiff represents changes to session between two states.
type SessionDiff struct {
	StatusChanged    bool   `json:"status_changed"`
	OldStatus        string `json:"old_status,omitempty"`
	NewStatus        string `json:"new_status,omitempty"`
	MessagesAdded    int    `json:"messages_added"`
	WorkspaceChanged bool   `json:"workspace_changed"`
}

// TaskDiff represents changes to task between two states.
type TaskDiff struct {
	StatusChanged   bool   `json:"status_changed"`
	OldStatus       string `json:"old_status,omitempty"`
	NewStatus       string `json:"new_status,omitempty"`
	AssigneeChanged bool   `json:"assignee_changed"`
	OldAssignee     string `json:"old_assignee,omitempty"`
	NewAssignee     string `json:"new_assignee,omitempty"`
	OutputChanged   bool   `json:"output_changed"`
	ErrorChanged    bool   `json:"error_changed"`
}

// FindStateTransitions finds all state transitions of a given type.
func (r *StateReconstructor) FindStateTransitions(field string) []StateTransition {
	snapshots := r.timeline.All()

	var transitions []StateTransition
	var lastValue any

	for _, snap := range snapshots {
		var currentValue any

		switch field {
		case "task_status":
			if snap.TaskState != nil {
				currentValue = snap.TaskState.Status
			}
		case "session_status":
			if snap.SessionState != nil {
				currentValue = snap.SessionState.Status
			}
		case "checkpoint_type":
			currentValue = snap.CheckpointType
		}

		if currentValue != nil && currentValue != lastValue {
			transitions = append(transitions, StateTransition{
				SequenceNum: snap.SequenceNum,
				Timestamp:   snap.Timestamp,
				Field:       field,
				FromValue:   lastValue,
				ToValue:     currentValue,
			})
			lastValue = currentValue
		}
	}

	return transitions
}

// StateTransition represents a state change.
type StateTransition struct {
	SequenceNum int64     `json:"sequence_num"`
	Timestamp   time.Time `json:"timestamp"`
	Field       string    `json:"field"`
	FromValue   any       `json:"from_value"`
	ToValue     any       `json:"to_value"`
}

// Internal methods

func (r *StateReconstructor) reconstructFromSnapshot(snap *snapshot.ExecutionSnapshot, index int) (*ReconstructedState, error) {
	state := &ReconstructedState{
		SequenceNum: snap.SequenceNum,
		Index:       index,
		Timestamp:   snap.Timestamp,
		Snapshot:    snap,
		AgentID:     snap.AgentID,
		TaskID:      snap.TaskID,
		SessionID:   snap.SessionID,
		TraceID:     snap.TraceID,
		SpanID:      snap.SpanID,
	}

	// Reconstruct session
	state.Session = r.ReconstructSessionAt(snap.SequenceNum)

	// Reconstruct task
	state.Task = r.ReconstructTaskAt(snap.SequenceNum)

	// Reconstruct workspace
	state.Workspace = r.ReconstructWorkspaceAt(snap.SequenceNum)

	// Get action history
	state.PreviousActions = r.GetActionHistoryAt(snap.SequenceNum)

	// Get error history
	state.ErrorHistory = r.GetErrorHistoryAt(snap.SequenceNum)

	return state, nil
}

func (r *StateReconstructor) compareReconstructedStates(state1, state2 *ReconstructedState) *StateComparison {
	if state1 == nil || state2 == nil {
		return nil
	}

	// Ensure state1 is before state2
	if state1.SequenceNum > state2.SequenceNum {
		state1, state2 = state2, state1
	}

	comparison := &StateComparison{
		State1SeqNum: state1.SequenceNum,
		State2SeqNum: state2.SequenceNum,
		TimeDelta:    state2.Timestamp.Sub(state1.Timestamp),
	}

	// Count snapshots between
	allSnapshots := r.timeline.All()
	for _, snap := range allSnapshots {
		if snap.SequenceNum > state1.SequenceNum && snap.SequenceNum < state2.SequenceNum {
			comparison.SnapshotsBetween++
			if snap.Action != nil {
				comparison.ActionsBetween = append(comparison.ActionsBetween, snap.Action)
			}
			if snap.Error != nil {
				comparison.ErrorsBetween = append(comparison.ErrorsBetween, snap.Error)
			}
		}
	}

	// Compare workspace
	comparison.WorkspaceDiff = r.compareWorkspaces(state1.Workspace, state2.Workspace)

	// Compare session
	comparison.SessionDiff = r.compareSessions(state1.Session, state2.Session)

	// Compare task
	comparison.TaskDiff = r.compareTasks(state1.Task, state2.Task)

	return comparison
}

func (r *StateReconstructor) compareWorkspaces(ws1, ws2 map[string]any) *WorkspaceDiff {
	if ws1 == nil && ws2 == nil {
		return nil
	}

	diff := &WorkspaceDiff{
		Added:    make(map[string]any),
		Modified: make(map[string]ValueChange),
	}

	// Find added and modified
	for k, v2 := range ws2 {
		if v1, exists := ws1[k]; exists {
			// Check if modified (simple comparison)
			if !equalValues(v1, v2) {
				diff.Modified[k] = ValueChange{Before: v1, After: v2}
			}
		} else {
			diff.Added[k] = v2
		}
	}

	// Find removed
	for k := range ws1 {
		if _, exists := ws2[k]; !exists {
			diff.Removed = append(diff.Removed, k)
		}
	}

	if len(diff.Added) == 0 && len(diff.Removed) == 0 && len(diff.Modified) == 0 {
		return nil
	}

	return diff
}

func (r *StateReconstructor) compareSessions(s1, s2 *snapshot.SessionSnapshot) *SessionDiff {
	if s1 == nil && s2 == nil {
		return nil
	}

	diff := &SessionDiff{}

	if s1 == nil {
		if s2 != nil {
			diff.NewStatus = s2.Status
			diff.StatusChanged = true
			diff.MessagesAdded = len(s2.History)
		}
		return diff
	}

	if s2 == nil {
		diff.OldStatus = s1.Status
		diff.StatusChanged = true
		return diff
	}

	if s1.Status != s2.Status {
		diff.StatusChanged = true
		diff.OldStatus = s1.Status
		diff.NewStatus = s2.Status
	}

	if len(s2.History) > len(s1.History) {
		diff.MessagesAdded = len(s2.History) - len(s1.History)
	}

	diff.WorkspaceChanged = !equalValues(s1.Workspace, s2.Workspace)

	return diff
}

func (r *StateReconstructor) compareTasks(t1, t2 *snapshot.TaskSnapshot) *TaskDiff {
	if t1 == nil && t2 == nil {
		return nil
	}

	diff := &TaskDiff{}

	if t1 == nil {
		if t2 != nil {
			diff.NewStatus = t2.Status
			diff.StatusChanged = true
			diff.NewAssignee = t2.AssignedTo
			if t2.AssignedTo != "" {
				diff.AssigneeChanged = true
			}
		}
		return diff
	}

	if t2 == nil {
		diff.OldStatus = t1.Status
		diff.StatusChanged = true
		return diff
	}

	if t1.Status != t2.Status {
		diff.StatusChanged = true
		diff.OldStatus = t1.Status
		diff.NewStatus = t2.Status
	}

	if t1.AssignedTo != t2.AssignedTo {
		diff.AssigneeChanged = true
		diff.OldAssignee = t1.AssignedTo
		diff.NewAssignee = t2.AssignedTo
	}

	diff.OutputChanged = !equalValues(t1.Output, t2.Output)
	diff.ErrorChanged = t1.Error != t2.Error

	return diff
}

// Simple value equality check
func equalValues(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// This is a simple comparison; for complex types, you might want deep comparison
	return a == b
}
