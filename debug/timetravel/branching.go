// Package timetravel provides execution branching for "what-if" analysis.
package timetravel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/Ranganaths/minion/debug/snapshot"
)

// BranchingEngine enables "what-if" analysis with execution branches.
type BranchingEngine struct {
	store    snapshot.SnapshotStore
	mu       sync.RWMutex
	branches map[string]*ExecutionBranch
}

// NewBranchingEngine creates a new branching engine.
func NewBranchingEngine(store snapshot.SnapshotStore) *BranchingEngine {
	return &BranchingEngine{
		store:    store,
		branches: make(map[string]*ExecutionBranch),
	}
}

// ExecutionBranch represents an alternative execution path.
type ExecutionBranch struct {
	ID                string           `json:"id"`
	Name              string           `json:"name,omitempty"`
	Description       string           `json:"description,omitempty"`
	ParentExecutionID string           `json:"parent_execution_id"`
	ParentBranchID    string           `json:"parent_branch_id,omitempty"` // For nested branches
	BranchPointSeq    int64            `json:"branch_point_seq"`
	Modification      *Modification    `json:"modification,omitempty"`
	Status            BranchStatus     `json:"status"`
	CreatedAt         time.Time        `json:"created_at"`
	ExecutedAt        *time.Time       `json:"executed_at,omitempty"`
	CompletedAt       *time.Time       `json:"completed_at,omitempty"`

	// Result after execution
	Timeline     *ExecutionTimeline `json:"-"` // Not serialized
	Result       *ReplayResult      `json:"result,omitempty"`

	// Comparison with parent
	Comparison   *BranchComparison  `json:"comparison,omitempty"`
}

// BranchStatus represents the status of a branch.
type BranchStatus string

const (
	BranchPending   BranchStatus = "pending"
	BranchRunning   BranchStatus = "running"
	BranchCompleted BranchStatus = "completed"
	BranchFailed    BranchStatus = "failed"
)

// BranchComparison compares two execution branches.
type BranchComparison struct {
	Branch1ID    string            `json:"branch1_id"`
	Branch2ID    string            `json:"branch2_id"`

	// High-level comparison
	Branch1Duration time.Duration  `json:"branch1_duration"`
	Branch2Duration time.Duration  `json:"branch2_duration"`
	DurationDelta   time.Duration  `json:"duration_delta"`

	Branch1Steps    int            `json:"branch1_steps"`
	Branch2Steps    int            `json:"branch2_steps"`
	StepsDelta      int            `json:"steps_delta"`

	Branch1Errors   int            `json:"branch1_errors"`
	Branch2Errors   int            `json:"branch2_errors"`

	// Detailed differences
	Differences     []*StateDifference `json:"differences,omitempty"`

	// Outcome comparison
	Branch1Success  bool           `json:"branch1_success"`
	Branch2Success  bool           `json:"branch2_success"`
	OutcomeSame     bool           `json:"outcome_same"`
}

// CreateBranchOptions configures branch creation.
type CreateBranchOptions struct {
	Name         string
	Description  string
	Modification *Modification
}

// CreateBranch creates a new execution branch from a checkpoint.
func (b *BranchingEngine) CreateBranch(ctx context.Context, executionID string, seqNum int64, opts *CreateBranchOptions) (*ExecutionBranch, error) {
	if opts == nil {
		opts = &CreateBranchOptions{}
	}

	// Verify the execution exists
	_, err := b.store.GetAtSequence(ctx, executionID, seqNum)
	if err != nil {
		return nil, fmt.Errorf("cannot create branch: %w", err)
	}

	branchID := fmt.Sprintf("branch-%s", uuid.New().String()[:8])

	branch := &ExecutionBranch{
		ID:                branchID,
		Name:              opts.Name,
		Description:       opts.Description,
		ParentExecutionID: executionID,
		BranchPointSeq:    seqNum,
		Modification:      opts.Modification,
		Status:            BranchPending,
		CreatedAt:         time.Now(),
	}

	b.mu.Lock()
	b.branches[branchID] = branch
	b.mu.Unlock()

	return branch, nil
}

// CreateBranchFromBranch creates a branch from another branch.
func (b *BranchingEngine) CreateBranchFromBranch(ctx context.Context, parentBranchID string, seqNum int64, opts *CreateBranchOptions) (*ExecutionBranch, error) {
	b.mu.RLock()
	parent, ok := b.branches[parentBranchID]
	b.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("parent branch not found: %s", parentBranchID)
	}

	if parent.Result == nil {
		return nil, fmt.Errorf("parent branch has not been executed")
	}

	branch, err := b.CreateBranch(ctx, parent.Result.ReplayExecutionID, seqNum, opts)
	if err != nil {
		return nil, err
	}

	branch.ParentBranchID = parentBranchID
	return branch, nil
}

// GetBranch returns a branch by ID.
func (b *BranchingEngine) GetBranch(branchID string) (*ExecutionBranch, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	branch, ok := b.branches[branchID]
	if !ok {
		return nil, fmt.Errorf("branch not found: %s", branchID)
	}
	return branch, nil
}

// ListBranches returns all branches for an execution.
func (b *BranchingEngine) ListBranches(executionID string) []*ExecutionBranch {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var branches []*ExecutionBranch
	for _, branch := range b.branches {
		if branch.ParentExecutionID == executionID {
			branches = append(branches, branch)
		}
	}
	return branches
}

// ListAllBranches returns all branches.
func (b *BranchingEngine) ListAllBranches() []*ExecutionBranch {
	b.mu.RLock()
	defer b.mu.RUnlock()

	branches := make([]*ExecutionBranch, 0, len(b.branches))
	for _, branch := range b.branches {
		branches = append(branches, branch)
	}
	return branches
}

// ExecuteBranch runs the branch execution.
func (b *BranchingEngine) ExecuteBranch(ctx context.Context, branchID string, opts *ReplayOptions) (*ReplayResult, error) {
	b.mu.Lock()
	branch, ok := b.branches[branchID]
	if !ok {
		b.mu.Unlock()
		return nil, fmt.Errorf("branch not found: %s", branchID)
	}
	branch.Status = BranchRunning
	now := time.Now()
	branch.ExecutedAt = &now
	b.mu.Unlock()

	// Load parent timeline
	parentTimeline, err := NewExecutionTimeline(ctx, b.store, branch.ParentExecutionID)
	if err != nil {
		b.setBranchStatus(branchID, BranchFailed)
		return nil, fmt.Errorf("failed to load parent timeline: %w", err)
	}

	// Create replay engine
	replayEngine := NewReplayEngine(b.store, parentTimeline)

	// Configure replay options
	if opts == nil {
		opts = &ReplayOptions{}
	}
	opts.Modification = branch.Modification
	if opts.Mode == "" {
		opts.Mode = ReplayModeHybrid
	}

	// Execute replay
	result, err := replayEngine.ReplayFrom(ctx, branch.BranchPointSeq, opts)
	if err != nil {
		b.setBranchStatus(branchID, BranchFailed)
		return nil, fmt.Errorf("replay failed: %w", err)
	}

	// Update branch
	b.mu.Lock()
	branch.Result = result
	branch.Status = BranchCompleted
	completedAt := time.Now()
	branch.CompletedAt = &completedAt

	// Try to load the new timeline
	if result.ReplayExecutionID != "" {
		timeline, err := NewExecutionTimeline(ctx, b.store, result.ReplayExecutionID)
		if err == nil {
			branch.Timeline = timeline
		}
	}
	b.mu.Unlock()

	return result, nil
}

// ExecuteBranchAsync runs the branch execution asynchronously.
func (b *BranchingEngine) ExecuteBranchAsync(ctx context.Context, branchID string, opts *ReplayOptions) <-chan *ReplayResult {
	resultChan := make(chan *ReplayResult, 1)

	go func() {
		defer close(resultChan)
		result, err := b.ExecuteBranch(ctx, branchID, opts)
		if err != nil {
			result = &ReplayResult{
				Error:        err,
				ErrorMessage: err.Error(),
			}
		}
		resultChan <- result
	}()

	return resultChan
}

// CompareBranches compares two branches.
func (b *BranchingEngine) CompareBranches(ctx context.Context, branchID1, branchID2 string) (*BranchComparison, error) {
	b.mu.RLock()
	branch1, ok1 := b.branches[branchID1]
	branch2, ok2 := b.branches[branchID2]
	b.mu.RUnlock()

	if !ok1 {
		return nil, fmt.Errorf("branch not found: %s", branchID1)
	}
	if !ok2 {
		return nil, fmt.Errorf("branch not found: %s", branchID2)
	}

	if branch1.Status != BranchCompleted || branch2.Status != BranchCompleted {
		return nil, fmt.Errorf("both branches must be completed")
	}

	comparison := &BranchComparison{
		Branch1ID: branchID1,
		Branch2ID: branchID2,
	}

	// Compare durations
	if branch1.Result != nil {
		comparison.Branch1Duration = branch1.Result.Duration
		comparison.Branch1Steps = branch1.Result.StepsReplayed
		comparison.Branch1Success = branch1.Result.Success
		comparison.Branch1Errors = len(branch1.Result.Differences)
	}
	if branch2.Result != nil {
		comparison.Branch2Duration = branch2.Result.Duration
		comparison.Branch2Steps = branch2.Result.StepsReplayed
		comparison.Branch2Success = branch2.Result.Success
		comparison.Branch2Errors = len(branch2.Result.Differences)
	}

	comparison.DurationDelta = comparison.Branch2Duration - comparison.Branch1Duration
	comparison.StepsDelta = comparison.Branch2Steps - comparison.Branch1Steps
	comparison.OutcomeSame = comparison.Branch1Success == comparison.Branch2Success

	// Compare timelines if available
	if branch1.Timeline != nil && branch2.Timeline != nil {
		comparison.Differences = b.compareTimelines(branch1.Timeline, branch2.Timeline)
	}

	return comparison, nil
}

// CompareWithParent compares a branch with its parent execution.
func (b *BranchingEngine) CompareWithParent(ctx context.Context, branchID string) (*BranchComparison, error) {
	b.mu.RLock()
	branch, ok := b.branches[branchID]
	b.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("branch not found: %s", branchID)
	}

	if branch.Status != BranchCompleted {
		return nil, fmt.Errorf("branch must be completed")
	}

	// Load parent timeline
	parentTimeline, err := NewExecutionTimeline(ctx, b.store, branch.ParentExecutionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load parent timeline: %w", err)
	}

	comparison := &BranchComparison{
		Branch1ID: branch.ParentExecutionID,
		Branch2ID: branchID,
	}

	// Parent stats
	parentSummary := parentTimeline.Summary()
	if parentSummary != nil {
		comparison.Branch1Duration = parentSummary.Duration
		comparison.Branch1Steps = parentSummary.TotalSteps
		comparison.Branch1Errors = parentSummary.ErrorCount
		comparison.Branch1Success = parentSummary.Status == "completed"
	}

	// Branch stats
	if branch.Result != nil {
		comparison.Branch2Duration = branch.Result.Duration
		comparison.Branch2Steps = branch.Result.StepsReplayed
		comparison.Branch2Success = branch.Result.Success
		comparison.Branch2Errors = 0
		if branch.Result.Error != nil {
			comparison.Branch2Errors = 1
		}
	}

	comparison.DurationDelta = comparison.Branch2Duration - comparison.Branch1Duration
	comparison.StepsDelta = comparison.Branch2Steps - comparison.Branch1Steps
	comparison.OutcomeSame = comparison.Branch1Success == comparison.Branch2Success

	// Compare timelines
	if branch.Timeline != nil {
		comparison.Differences = b.compareTimelines(parentTimeline, branch.Timeline)
	}

	// Store comparison in branch
	b.mu.Lock()
	branch.Comparison = comparison
	b.mu.Unlock()

	return comparison, nil
}

// DeleteBranch removes a branch.
func (b *BranchingEngine) DeleteBranch(branchID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.branches[branchID]; !ok {
		return fmt.Errorf("branch not found: %s", branchID)
	}

	delete(b.branches, branchID)
	return nil
}

// GetBranchTree returns the branch hierarchy for an execution.
func (b *BranchingEngine) GetBranchTree(executionID string) *BranchTree {
	b.mu.RLock()
	defer b.mu.RUnlock()

	tree := &BranchTree{
		RootExecutionID: executionID,
		Branches:        make(map[string]*BranchNode),
	}

	// Build tree
	for _, branch := range b.branches {
		if branch.ParentExecutionID == executionID || branch.ParentBranchID != "" {
			node := &BranchNode{
				Branch:   branch,
				Children: make([]*BranchNode, 0),
			}
			tree.Branches[branch.ID] = node
		}
	}

	// Link children
	for _, node := range tree.Branches {
		if node.Branch.ParentBranchID != "" {
			if parent, ok := tree.Branches[node.Branch.ParentBranchID]; ok {
				parent.Children = append(parent.Children, node)
			}
		} else {
			tree.RootBranches = append(tree.RootBranches, node)
		}
	}

	return tree
}

// BranchTree represents the hierarchy of branches.
type BranchTree struct {
	RootExecutionID string                  `json:"root_execution_id"`
	RootBranches    []*BranchNode           `json:"root_branches"`
	Branches        map[string]*BranchNode  `json:"-"`
}

// BranchNode represents a node in the branch tree.
type BranchNode struct {
	Branch   *ExecutionBranch `json:"branch"`
	Children []*BranchNode    `json:"children,omitempty"`
}

// Helper methods

func (b *BranchingEngine) setBranchStatus(branchID string, status BranchStatus) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if branch, ok := b.branches[branchID]; ok {
		branch.Status = status
	}
}

func (b *BranchingEngine) compareTimelines(t1, t2 *ExecutionTimeline) []*StateDifference {
	var diffs []*StateDifference

	snaps1 := t1.All()
	snaps2 := t2.All()

	// Create maps by sequence for comparison
	map1 := make(map[int64]*snapshot.ExecutionSnapshot)
	map2 := make(map[int64]*snapshot.ExecutionSnapshot)

	for _, s := range snaps1 {
		map1[s.SequenceNum] = s
	}
	for _, s := range snaps2 {
		map2[s.SequenceNum] = s
	}

	// Find differences
	allSeqs := make(map[int64]bool)
	for seq := range map1 {
		allSeqs[seq] = true
	}
	for seq := range map2 {
		allSeqs[seq] = true
	}

	for seq := range allSeqs {
		s1, in1 := map1[seq]
		s2, in2 := map2[seq]

		if in1 && !in2 {
			diffs = append(diffs, &StateDifference{
				SequenceNum: seq,
				Path:        "snapshot",
				Original:    s1,
				Replayed:    nil,
				Type:        "removed",
			})
		} else if !in1 && in2 {
			diffs = append(diffs, &StateDifference{
				SequenceNum: seq,
				Path:        "snapshot",
				Original:    nil,
				Replayed:    s2,
				Type:        "added",
			})
		} else if in1 && in2 {
			// Compare snapshots
			if s1.CheckpointType != s2.CheckpointType {
				diffs = append(diffs, &StateDifference{
					SequenceNum: seq,
					Path:        "checkpoint_type",
					Original:    s1.CheckpointType,
					Replayed:    s2.CheckpointType,
					Type:        "changed",
				})
			}
			if !equalValues(s1.Output, s2.Output) {
				diffs = append(diffs, &StateDifference{
					SequenceNum: seq,
					Path:        "output",
					Original:    s1.Output,
					Replayed:    s2.Output,
					Type:        "changed",
				})
			}
			if (s1.Error == nil) != (s2.Error == nil) {
				diffs = append(diffs, &StateDifference{
					SequenceNum: seq,
					Path:        "error",
					Original:    s1.Error,
					Replayed:    s2.Error,
					Type:        "changed",
				})
			}
		}
	}

	return diffs
}

// WhatIf is a convenience method to quickly test a modification.
func (b *BranchingEngine) WhatIf(ctx context.Context, executionID string, seqNum int64, mod *Modification) (*BranchComparison, error) {
	// Create branch
	branch, err := b.CreateBranch(ctx, executionID, seqNum, &CreateBranchOptions{
		Name:         "what-if",
		Modification: mod,
	})
	if err != nil {
		return nil, err
	}

	// Execute branch
	_, err = b.ExecuteBranch(ctx, branch.ID, &ReplayOptions{
		Mode:                ReplayModeHybrid,
		CompareWithOriginal: true,
	})
	if err != nil {
		return nil, err
	}

	// Compare with parent
	return b.CompareWithParent(ctx, branch.ID)
}

// WhatIfMultiple tests multiple modifications in parallel.
func (b *BranchingEngine) WhatIfMultiple(ctx context.Context, executionID string, seqNum int64, mods []*Modification) ([]*BranchComparison, error) {
	var wg sync.WaitGroup
	results := make([]*BranchComparison, len(mods))
	errors := make([]error, len(mods))

	for i, mod := range mods {
		wg.Add(1)
		go func(idx int, m *Modification) {
			defer wg.Done()
			comparison, err := b.WhatIf(ctx, executionID, seqNum, m)
			if err != nil {
				errors[idx] = err
			} else {
				results[idx] = comparison
			}
		}(i, mod)
	}

	wg.Wait()

	// Check for errors
	for _, err := range errors {
		if err != nil {
			return results, err
		}
	}

	return results, nil
}
