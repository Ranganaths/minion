package workflow

import (
	"encoding/json"
	"sync"
	"time"
)

// WorkflowStatus represents the status of a workflow execution
type WorkflowStatus string

const (
	// WorkflowStatusPending means the workflow hasn't started
	WorkflowStatusPending WorkflowStatus = "pending"
	// WorkflowStatusRunning means the workflow is executing
	WorkflowStatusRunning WorkflowStatus = "running"
	// WorkflowStatusPaused means the workflow is paused
	WorkflowStatusPaused WorkflowStatus = "paused"
	// WorkflowStatusCompleted means the workflow finished successfully
	WorkflowStatusCompleted WorkflowStatus = "completed"
	// WorkflowStatusFailed means the workflow failed
	WorkflowStatusFailed WorkflowStatus = "failed"
	// WorkflowStatusCancelled means the workflow was cancelled
	WorkflowStatusCancelled WorkflowStatus = "cancelled"
	// WorkflowStatusTimedOut means the workflow timed out
	WorkflowStatusTimedOut WorkflowStatus = "timed_out"
)

// NodeStatus represents the status of a node execution
type NodeStatus string

const (
	// NodeStatusPending means the node hasn't started
	NodeStatusPending NodeStatus = "pending"
	// NodeStatusWaiting means the node is waiting for dependencies
	NodeStatusWaiting NodeStatus = "waiting"
	// NodeStatusRunning means the node is executing
	NodeStatusRunning NodeStatus = "running"
	// NodeStatusCompleted means the node finished successfully
	NodeStatusCompleted NodeStatus = "completed"
	// NodeStatusFailed means the node failed
	NodeStatusFailed NodeStatus = "failed"
	// NodeStatusSkipped means the node was skipped (e.g., condition branch not taken)
	NodeStatusSkipped NodeStatus = "skipped"
	// NodeStatusCancelled means the node was cancelled
	NodeStatusCancelled NodeStatus = "cancelled"
	// NodeStatusRetrying means the node is retrying after failure
	NodeStatusRetrying NodeStatus = "retrying"
)

// WorkflowState holds the execution state of a workflow
type WorkflowState struct {
	WorkflowID    string                    `json:"workflow_id"`
	RunID         string                    `json:"run_id"`
	Status        WorkflowStatus            `json:"status"`
	StartTime     time.Time                 `json:"start_time"`
	EndTime       *time.Time                `json:"end_time,omitempty"`
	Input         map[string]interface{}    `json:"input"`
	Output        map[string]interface{}    `json:"output,omitempty"`
	Error         string                    `json:"error,omitempty"`
	NodeStates    map[string]*NodeState     `json:"node_states"`
	Variables     map[string]interface{}    `json:"variables"`    // Shared workflow variables
	Checkpoints   []*Checkpoint             `json:"checkpoints"`  // State checkpoints for recovery
	Metrics       *WorkflowMetrics          `json:"metrics"`
	Metadata      map[string]interface{}    `json:"metadata,omitempty"`

	mu sync.RWMutex
}

// NodeState holds the execution state of a node
type NodeState struct {
	NodeID      string                 `json:"node_id"`
	Status      NodeStatus             `json:"status"`
	StartTime   *time.Time             `json:"start_time,omitempty"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	Input       map[string]interface{} `json:"input,omitempty"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	RetryCount  int                    `json:"retry_count"`
	Attempts    []*NodeAttempt         `json:"attempts,omitempty"`
	LoopState   *LoopState             `json:"loop_state,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NodeAttempt records a single execution attempt of a node
type NodeAttempt struct {
	AttemptNumber int                    `json:"attempt_number"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       time.Time              `json:"end_time"`
	Status        NodeStatus             `json:"status"`
	Output        map[string]interface{} `json:"output,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

// LoopState tracks the state of a loop node
type LoopState struct {
	CurrentIteration int                      `json:"current_iteration"`
	MaxIterations    int                      `json:"max_iterations"`
	IterationResults []map[string]interface{} `json:"iteration_results,omitempty"`
	CollectionSize   int                      `json:"collection_size,omitempty"`
	FailedIterations []int                    `json:"failed_iterations,omitempty"`
}

// Checkpoint represents a snapshot of workflow state for recovery
type Checkpoint struct {
	ID           string                 `json:"id"`
	CreatedAt    time.Time              `json:"created_at"`
	NodeStates   map[string]*NodeState  `json:"node_states"`
	Variables    map[string]interface{} `json:"variables"`
	LastNodeID   string                 `json:"last_node_id"`
	Description  string                 `json:"description,omitempty"`
}

// WorkflowMetrics tracks execution metrics
type WorkflowMetrics struct {
	TotalNodes        int           `json:"total_nodes"`
	CompletedNodes    int           `json:"completed_nodes"`
	FailedNodes       int           `json:"failed_nodes"`
	SkippedNodes      int           `json:"skipped_nodes"`
	TotalDuration     time.Duration `json:"total_duration"`
	TotalRetries      int           `json:"total_retries"`
	ParallelBranches  int           `json:"parallel_branches"`
	LoopIterations    int           `json:"loop_iterations"`
}

// NewWorkflowState creates a new workflow state
func NewWorkflowState(workflowID, runID string, input map[string]interface{}) *WorkflowState {
	return &WorkflowState{
		WorkflowID:  workflowID,
		RunID:       runID,
		Status:      WorkflowStatusPending,
		Input:       input,
		NodeStates:  make(map[string]*NodeState),
		Variables:   make(map[string]interface{}),
		Checkpoints: make([]*Checkpoint, 0),
		Metrics:     &WorkflowMetrics{},
		Metadata:    make(map[string]interface{}),
	}
}

// SetStatus updates the workflow status
func (s *WorkflowState) SetStatus(status WorkflowStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status

	if status == WorkflowStatusRunning && s.StartTime.IsZero() {
		s.StartTime = time.Now()
	}

	if status == WorkflowStatusCompleted || status == WorkflowStatusFailed ||
		status == WorkflowStatusCancelled || status == WorkflowStatusTimedOut {
		now := time.Now()
		s.EndTime = &now
		s.Metrics.TotalDuration = now.Sub(s.StartTime)
	}
}

// GetNodeState returns the state of a node
func (s *WorkflowState) GetNodeState(nodeID string) *NodeState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.NodeStates[nodeID]
}

// SetNodeStatus updates a node's status
func (s *WorkflowState) SetNodeStatus(nodeID string, status NodeStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.NodeStates[nodeID] == nil {
		s.NodeStates[nodeID] = &NodeState{
			NodeID:   nodeID,
			Metadata: make(map[string]interface{}),
		}
	}

	ns := s.NodeStates[nodeID]
	ns.Status = status

	if status == NodeStatusRunning && ns.StartTime == nil {
		now := time.Now()
		ns.StartTime = &now
	}

	if status == NodeStatusCompleted || status == NodeStatusFailed ||
		status == NodeStatusSkipped || status == NodeStatusCancelled {
		now := time.Now()
		ns.EndTime = &now

		// Update metrics
		switch status {
		case NodeStatusCompleted:
			s.Metrics.CompletedNodes++
		case NodeStatusFailed:
			s.Metrics.FailedNodes++
		case NodeStatusSkipped:
			s.Metrics.SkippedNodes++
		}
	}
}

// SetNodeInput sets the input for a node
func (s *WorkflowState) SetNodeInput(nodeID string, input map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.NodeStates[nodeID] == nil {
		s.NodeStates[nodeID] = &NodeState{
			NodeID:   nodeID,
			Metadata: make(map[string]interface{}),
		}
	}
	s.NodeStates[nodeID].Input = input
}

// SetNodeOutput sets the output for a node
func (s *WorkflowState) SetNodeOutput(nodeID string, output map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.NodeStates[nodeID] == nil {
		s.NodeStates[nodeID] = &NodeState{
			NodeID:   nodeID,
			Metadata: make(map[string]interface{}),
		}
	}
	s.NodeStates[nodeID].Output = output
}

// SetNodeError sets the error for a node
func (s *WorkflowState) SetNodeError(nodeID string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.NodeStates[nodeID] == nil {
		s.NodeStates[nodeID] = &NodeState{
			NodeID:   nodeID,
			Metadata: make(map[string]interface{}),
		}
	}
	if err != nil {
		s.NodeStates[nodeID].Error = err.Error()
	}
}

// IncrementRetry increments the retry count for a node
func (s *WorkflowState) IncrementRetry(nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.NodeStates[nodeID] == nil {
		s.NodeStates[nodeID] = &NodeState{
			NodeID:   nodeID,
			Metadata: make(map[string]interface{}),
		}
	}
	s.NodeStates[nodeID].RetryCount++
	s.Metrics.TotalRetries++
}

// RecordAttempt records an execution attempt for a node
func (s *WorkflowState) RecordAttempt(nodeID string, attempt *NodeAttempt) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.NodeStates[nodeID] == nil {
		s.NodeStates[nodeID] = &NodeState{
			NodeID:   nodeID,
			Metadata: make(map[string]interface{}),
		}
	}
	s.NodeStates[nodeID].Attempts = append(s.NodeStates[nodeID].Attempts, attempt)
}

// SetVariable sets a workflow variable
func (s *WorkflowState) SetVariable(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Variables[key] = value
}

// GetVariable gets a workflow variable
func (s *WorkflowState) GetVariable(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.Variables[key]
	return v, ok
}

// CreateCheckpoint creates a state checkpoint
func (s *WorkflowState) CreateCheckpoint(id, description string) *Checkpoint {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Deep copy node states
	nodeStatesCopy := make(map[string]*NodeState, len(s.NodeStates))
	for k, v := range s.NodeStates {
		nodeStatesCopy[k] = v.Clone()
	}

	// Deep copy variables
	varsCopy := make(map[string]interface{}, len(s.Variables))
	for k, v := range s.Variables {
		varsCopy[k] = v
	}

	checkpoint := &Checkpoint{
		ID:          id,
		CreatedAt:   time.Now(),
		NodeStates:  nodeStatesCopy,
		Variables:   varsCopy,
		Description: description,
	}

	s.Checkpoints = append(s.Checkpoints, checkpoint)
	return checkpoint
}

// RestoreFromCheckpoint restores state from a checkpoint
func (s *WorkflowState) RestoreFromCheckpoint(checkpoint *Checkpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Restore node states
	s.NodeStates = make(map[string]*NodeState, len(checkpoint.NodeStates))
	for k, v := range checkpoint.NodeStates {
		s.NodeStates[k] = v.Clone()
	}

	// Restore variables
	s.Variables = make(map[string]interface{}, len(checkpoint.Variables))
	for k, v := range checkpoint.Variables {
		s.Variables[k] = v
	}

	s.Status = WorkflowStatusRunning
}

// GetLastCheckpoint returns the most recent checkpoint
func (s *WorkflowState) GetLastCheckpoint() *Checkpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.Checkpoints) == 0 {
		return nil
	}
	return s.Checkpoints[len(s.Checkpoints)-1]
}

// IsTerminal returns true if the workflow is in a terminal state
func (s *WorkflowState) IsTerminal() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Status == WorkflowStatusCompleted ||
		s.Status == WorkflowStatusFailed ||
		s.Status == WorkflowStatusCancelled ||
		s.Status == WorkflowStatusTimedOut
}

// GetPendingNodes returns nodes that are pending execution
func (s *WorkflowState) GetPendingNodes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pending := make([]string, 0)
	for nodeID, state := range s.NodeStates {
		if state.Status == NodeStatusPending || state.Status == NodeStatusWaiting {
			pending = append(pending, nodeID)
		}
	}
	return pending
}

// GetRunningNodes returns nodes that are currently running
func (s *WorkflowState) GetRunningNodes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	running := make([]string, 0)
	for nodeID, state := range s.NodeStates {
		if state.Status == NodeStatusRunning {
			running = append(running, nodeID)
		}
	}
	return running
}

// Clone creates a deep copy of the node state
func (ns *NodeState) Clone() *NodeState {
	clone := &NodeState{
		NodeID:     ns.NodeID,
		Status:     ns.Status,
		Error:      ns.Error,
		RetryCount: ns.RetryCount,
	}

	if ns.StartTime != nil {
		t := *ns.StartTime
		clone.StartTime = &t
	}
	if ns.EndTime != nil {
		t := *ns.EndTime
		clone.EndTime = &t
	}

	if ns.Input != nil {
		clone.Input = make(map[string]interface{})
		for k, v := range ns.Input {
			clone.Input[k] = v
		}
	}

	if ns.Output != nil {
		clone.Output = make(map[string]interface{})
		for k, v := range ns.Output {
			clone.Output[k] = v
		}
	}

	if ns.Attempts != nil {
		clone.Attempts = make([]*NodeAttempt, len(ns.Attempts))
		for i, a := range ns.Attempts {
			clone.Attempts[i] = a.Clone()
		}
	}

	if ns.LoopState != nil {
		clone.LoopState = ns.LoopState.Clone()
	}

	if ns.Metadata != nil {
		clone.Metadata = make(map[string]interface{})
		for k, v := range ns.Metadata {
			clone.Metadata[k] = v
		}
	}

	return clone
}

// Clone creates a deep copy of a node attempt
func (a *NodeAttempt) Clone() *NodeAttempt {
	clone := &NodeAttempt{
		AttemptNumber: a.AttemptNumber,
		StartTime:     a.StartTime,
		EndTime:       a.EndTime,
		Status:        a.Status,
		Error:         a.Error,
	}

	if a.Output != nil {
		clone.Output = make(map[string]interface{})
		for k, v := range a.Output {
			clone.Output[k] = v
		}
	}

	return clone
}

// Clone creates a deep copy of loop state
func (ls *LoopState) Clone() *LoopState {
	clone := &LoopState{
		CurrentIteration: ls.CurrentIteration,
		MaxIterations:    ls.MaxIterations,
		CollectionSize:   ls.CollectionSize,
	}

	if ls.IterationResults != nil {
		clone.IterationResults = make([]map[string]interface{}, len(ls.IterationResults))
		for i, r := range ls.IterationResults {
			clone.IterationResults[i] = make(map[string]interface{})
			for k, v := range r {
				clone.IterationResults[i][k] = v
			}
		}
	}

	if ls.FailedIterations != nil {
		clone.FailedIterations = make([]int, len(ls.FailedIterations))
		copy(clone.FailedIterations, ls.FailedIterations)
	}

	return clone
}

// ToJSON serializes the workflow state to JSON
func (s *WorkflowState) ToJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(s, "", "  ")
}

// FromStateJSON deserializes workflow state from JSON
func FromStateJSON(data []byte) (*WorkflowState, error) {
	state := &WorkflowState{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, err
	}
	return state, nil
}
