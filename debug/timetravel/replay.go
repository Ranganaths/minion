// Package timetravel provides execution replay capabilities.
package timetravel

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/Ranganaths/minion/debug/snapshot"
)

// ReplayEngine enables re-execution from any checkpoint.
type ReplayEngine struct {
	store         snapshot.SnapshotStore
	timeline      *ExecutionTimeline
	reconstructor *StateReconstructor

	// Execution handlers
	toolExecutor ToolExecutor
	llmExecutor  LLMExecutor
}

// ToolExecutor is a function that executes a tool during replay.
type ToolExecutor func(ctx context.Context, toolName string, input any) (any, error)

// LLMExecutor is a function that executes an LLM call during replay.
type LLMExecutor func(ctx context.Context, provider, model string, input any) (any, error)

// NewReplayEngine creates a new replay engine.
func NewReplayEngine(store snapshot.SnapshotStore, timeline *ExecutionTimeline) *ReplayEngine {
	return &ReplayEngine{
		store:         store,
		timeline:      timeline,
		reconstructor: NewStateReconstructor(timeline),
	}
}

// SetToolExecutor sets the tool executor for replay.
func (r *ReplayEngine) SetToolExecutor(executor ToolExecutor) {
	r.toolExecutor = executor
}

// SetLLMExecutor sets the LLM executor for replay.
func (r *ReplayEngine) SetLLMExecutor(executor LLMExecutor) {
	r.llmExecutor = executor
}

// ReplayOptions configures replay behavior.
type ReplayOptions struct {
	// Replay mode
	Mode ReplayMode

	// Stop conditions
	StopAtCheckpoint snapshot.CheckpointType
	StopAtSequence   int64
	MaxSteps         int
	Timeout          time.Duration

	// Modification to apply
	Modification *Modification

	// Comparison mode
	CompareWithOriginal bool

	// Callbacks
	OnStep func(step *ReplayStep)
}

// ReplayMode specifies how to handle operations during replay.
type ReplayMode string

const (
	// ReplayModeSimulate simulates execution without calling real tools/LLMs.
	// Uses recorded outputs from original execution.
	ReplayModeSimulate ReplayMode = "simulate"

	// ReplayModeExecute actually executes tools and LLM calls.
	ReplayModeExecute ReplayMode = "execute"

	// ReplayModeHybrid uses recorded outputs but executes modified steps.
	ReplayModeHybrid ReplayMode = "hybrid"
)

// Modification represents a change to apply during replay.
type Modification struct {
	Type string // "input", "workspace", "tool_response", "llm_response"
	Path string // JSON path or identifier
	Value any   // New value
}

// ReplayResult contains the result of a replay operation.
type ReplayResult struct {
	// Identification
	OriginalExecutionID string `json:"original_execution_id"`
	ReplayExecutionID   string `json:"replay_execution_id"`

	// Timing
	ReplayStartSeq  int64         `json:"replay_start_seq"`
	ReplayStartTime time.Time     `json:"replay_start_time"`
	ReplayEndTime   time.Time     `json:"replay_end_time"`
	Duration        time.Duration `json:"duration"`

	// Result
	Success      bool   `json:"success"`
	StoppedAt    int64  `json:"stopped_at,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	Output       any    `json:"output,omitempty"`
	Error        error  `json:"error,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`

	// Statistics
	StepsReplayed int `json:"steps_replayed"`
	ToolCalls     int `json:"tool_calls"`
	LLMCalls      int `json:"llm_calls"`

	// For comparison mode
	Differences []*StateDifference `json:"differences,omitempty"`
}

// ReplayStep represents a single step during replay.
type ReplayStep struct {
	SequenceNum    int64                  `json:"sequence_num"`
	CheckpointType snapshot.CheckpointType `json:"checkpoint_type"`
	Input          any                    `json:"input,omitempty"`
	Output         any                    `json:"output,omitempty"`
	Duration       time.Duration          `json:"duration"`
	Simulated      bool                   `json:"simulated"`
	Modified       bool                   `json:"modified"`
	Error          error                  `json:"error,omitempty"`
}

// StateDifference represents a difference between original and replay.
type StateDifference struct {
	SequenceNum int64  `json:"sequence_num"`
	Path        string `json:"path"`
	Original    any    `json:"original"`
	Replayed    any    `json:"replayed"`
	Type        string `json:"type"` // "added", "removed", "changed"
}

// ReplayFrom replays execution from a given sequence number.
func (r *ReplayEngine) ReplayFrom(ctx context.Context, seqNum int64, opts *ReplayOptions) (*ReplayResult, error) {
	if opts == nil {
		opts = &ReplayOptions{
			Mode: ReplayModeSimulate,
		}
	}

	result := &ReplayResult{
		OriginalExecutionID: r.timeline.ExecutionID(),
		ReplayExecutionID:   uuid.New().String(),
		ReplayStartSeq:      seqNum,
		ReplayStartTime:     time.Now(),
	}

	// Set up timeout
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Get snapshots from the start point
	allSnapshots := r.timeline.All()
	var snapshotsToReplay []*snapshot.ExecutionSnapshot
	for _, snap := range allSnapshots {
		if snap.SequenceNum >= seqNum {
			snapshotsToReplay = append(snapshotsToReplay, snap)
		}
	}

	if len(snapshotsToReplay) == 0 {
		result.Error = fmt.Errorf("no snapshots found from sequence %d", seqNum)
		result.ErrorMessage = result.Error.Error()
		return result, result.Error
	}

	// Reconstruct initial state
	initialState, err := r.reconstructor.ReconstructAt(seqNum)
	if err != nil {
		result.Error = err
		result.ErrorMessage = err.Error()
		return result, err
	}

	// Replay each step
	stepsReplayed := 0
	for _, snap := range snapshotsToReplay {
		// Check stop conditions
		if opts.MaxSteps > 0 && stepsReplayed >= opts.MaxSteps {
			result.StoppedAt = snap.SequenceNum
			result.StopReason = "max_steps"
			break
		}

		if opts.StopAtSequence > 0 && snap.SequenceNum >= opts.StopAtSequence {
			result.StoppedAt = snap.SequenceNum
			result.StopReason = "sequence"
			break
		}

		if opts.StopAtCheckpoint != "" && snap.CheckpointType == opts.StopAtCheckpoint {
			result.StoppedAt = snap.SequenceNum
			result.StopReason = "checkpoint"
			break
		}

		// Check context
		select {
		case <-ctx.Done():
			result.StoppedAt = snap.SequenceNum
			result.StopReason = "timeout"
			result.Error = ctx.Err()
			result.ErrorMessage = ctx.Err().Error()
			break
		default:
		}

		// Replay step
		step, err := r.replayStep(ctx, snap, initialState, opts)
		if err != nil {
			result.Error = err
			result.ErrorMessage = err.Error()
			result.StoppedAt = snap.SequenceNum
			result.StopReason = "error"
			break
		}

		// Track statistics
		stepsReplayed++
		if snap.CheckpointType == snapshot.CheckpointToolCallStart ||
			snap.CheckpointType == snapshot.CheckpointToolCallEnd {
			result.ToolCalls++
		}
		if snap.CheckpointType == snapshot.CheckpointLLMCallStart ||
			snap.CheckpointType == snapshot.CheckpointLLMCallEnd {
			result.LLMCalls++
		}

		// Notify callback
		if opts.OnStep != nil {
			opts.OnStep(step)
		}

		// Compare with original if requested
		if opts.CompareWithOriginal {
			diff := r.compareStepWithOriginal(snap, step)
			if diff != nil {
				result.Differences = append(result.Differences, diff...)
			}
		}

		// Store output from final step
		result.Output = step.Output
	}

	result.StepsReplayed = stepsReplayed
	result.ReplayEndTime = time.Now()
	result.Duration = result.ReplayEndTime.Sub(result.ReplayStartTime)
	result.Success = result.Error == nil

	return result, nil
}

// ReplayWithModification replays with a modification applied.
func (r *ReplayEngine) ReplayWithModification(ctx context.Context, seqNum int64, mod *Modification, opts *ReplayOptions) (*ReplayResult, error) {
	if opts == nil {
		opts = &ReplayOptions{}
	}
	opts.Modification = mod
	opts.Mode = ReplayModeHybrid

	return r.ReplayFrom(ctx, seqNum, opts)
}

// SimulateFrom simulates execution without actually calling tools/LLMs.
func (r *ReplayEngine) SimulateFrom(ctx context.Context, seqNum int64, opts *ReplayOptions) (*ReplayResult, error) {
	if opts == nil {
		opts = &ReplayOptions{}
	}
	opts.Mode = ReplayModeSimulate

	return r.ReplayFrom(ctx, seqNum, opts)
}

// WalkThrough steps through execution one checkpoint at a time.
func (r *ReplayEngine) WalkThrough(ctx context.Context, seqNum int64) (*ReplayWalker, error) {
	return &ReplayWalker{
		engine:      r,
		ctx:         ctx,
		startSeqNum: seqNum,
		timeline:    r.timeline,
	}, nil
}

// ReplayWalker allows stepping through execution interactively.
type ReplayWalker struct {
	engine      *ReplayEngine
	ctx         context.Context
	startSeqNum int64
	timeline    *ExecutionTimeline
	currentStep int
	steps       []*ReplayStep
}

// Current returns the current step.
func (w *ReplayWalker) Current() *ReplayStep {
	if w.currentStep < 0 || w.currentStep >= len(w.steps) {
		return nil
	}
	return w.steps[w.currentStep]
}

// Next advances to and replays the next step.
func (w *ReplayWalker) Next() (*ReplayStep, error) {
	snap := w.timeline.StepForward()
	if snap == nil {
		return nil, fmt.Errorf("no more steps")
	}

	initialState, _ := w.engine.reconstructor.ReconstructAt(snap.SequenceNum)
	step, err := w.engine.replayStep(w.ctx, snap, initialState, &ReplayOptions{Mode: ReplayModeSimulate})
	if err != nil {
		return nil, err
	}

	w.steps = append(w.steps, step)
	w.currentStep = len(w.steps) - 1
	return step, nil
}

// Previous goes back to the previous step.
func (w *ReplayWalker) Previous() *ReplayStep {
	if w.currentStep > 0 {
		w.currentStep--
		w.timeline.StepBackward()
	}
	return w.Current()
}

// JumpTo jumps to a specific sequence number.
func (w *ReplayWalker) JumpTo(seqNum int64) (*ReplayStep, error) {
	snap := w.timeline.JumpTo(seqNum)
	if snap == nil {
		return nil, fmt.Errorf("sequence not found: %d", seqNum)
	}

	initialState, _ := w.engine.reconstructor.ReconstructAt(snap.SequenceNum)
	step, err := w.engine.replayStep(w.ctx, snap, initialState, &ReplayOptions{Mode: ReplayModeSimulate})
	if err != nil {
		return nil, err
	}

	w.steps = append(w.steps, step)
	w.currentStep = len(w.steps) - 1
	return step, nil
}

// GetState returns the reconstructed state at the current position.
func (w *ReplayWalker) GetState() (*ReconstructedState, error) {
	current := w.timeline.Current()
	if current == nil {
		return nil, fmt.Errorf("no current snapshot")
	}
	return w.engine.reconstructor.ReconstructAt(current.SequenceNum)
}

// Internal methods

func (r *ReplayEngine) replayStep(ctx context.Context, snap *snapshot.ExecutionSnapshot, state *ReconstructedState, opts *ReplayOptions) (*ReplayStep, error) {
	step := &ReplayStep{
		SequenceNum:    snap.SequenceNum,
		CheckpointType: snap.CheckpointType,
		Input:          snap.Input,
	}

	startTime := time.Now()

	switch opts.Mode {
	case ReplayModeSimulate:
		// Use recorded output
		step.Output = snap.Output
		step.Simulated = true

	case ReplayModeExecute:
		// Actually execute
		output, err := r.executeStep(ctx, snap, state, opts)
		if err != nil {
			step.Error = err
			return step, err
		}
		step.Output = output
		step.Simulated = false

	case ReplayModeHybrid:
		// Execute if modified, otherwise simulate
		if r.isStepModified(snap, opts.Modification) {
			output, err := r.executeStep(ctx, snap, state, opts)
			if err != nil {
				step.Error = err
				return step, err
			}
			step.Output = output
			step.Simulated = false
			step.Modified = true
		} else {
			step.Output = snap.Output
			step.Simulated = true
		}
	}

	step.Duration = time.Since(startTime)
	return step, nil
}

func (r *ReplayEngine) executeStep(ctx context.Context, snap *snapshot.ExecutionSnapshot, state *ReconstructedState, opts *ReplayOptions) (any, error) {
	// Apply modification if applicable
	input := snap.Input
	if opts.Modification != nil && opts.Modification.Type == "input" {
		input = opts.Modification.Value
	}

	switch snap.CheckpointType {
	case snapshot.CheckpointToolCallStart, snapshot.CheckpointToolCallEnd:
		if r.toolExecutor == nil {
			return snap.Output, nil // Fall back to recorded output
		}
		toolName := ""
		if snap.Action != nil {
			toolName = snap.Action.ToolName
		}
		return r.toolExecutor(ctx, toolName, input)

	case snapshot.CheckpointLLMCallStart, snapshot.CheckpointLLMCallEnd:
		if r.llmExecutor == nil {
			return snap.Output, nil // Fall back to recorded output
		}
		provider, model := "", ""
		if snap.Action != nil {
			provider = snap.Action.Provider
			model = snap.Action.Model
		}
		return r.llmExecutor(ctx, provider, model, input)

	default:
		// For other checkpoints, use recorded output
		return snap.Output, nil
	}
}

func (r *ReplayEngine) isStepModified(snap *snapshot.ExecutionSnapshot, mod *Modification) bool {
	if mod == nil {
		return false
	}

	// Check if this step matches the modification criteria
	switch mod.Type {
	case "input":
		// Any step with input could be modified
		return snap.Input != nil
	case "tool_response":
		// Tool call steps
		return snap.CheckpointType == snapshot.CheckpointToolCallStart ||
			snap.CheckpointType == snapshot.CheckpointToolCallEnd
	case "llm_response":
		// LLM call steps
		return snap.CheckpointType == snapshot.CheckpointLLMCallStart ||
			snap.CheckpointType == snapshot.CheckpointLLMCallEnd
	}

	return false
}

func (r *ReplayEngine) compareStepWithOriginal(original *snapshot.ExecutionSnapshot, replayed *ReplayStep) []*StateDifference {
	var diffs []*StateDifference

	// Compare output
	if !equalValues(original.Output, replayed.Output) {
		diffs = append(diffs, &StateDifference{
			SequenceNum: original.SequenceNum,
			Path:        "output",
			Original:    original.Output,
			Replayed:    replayed.Output,
			Type:        "changed",
		})
	}

	return diffs
}
