package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DAGExecutor executes workflows defined as DAGs
type DAGExecutor struct {
	dag            *DAG
	maxParallel    int
	defaultTimeout time.Duration
	hooks          []ExecutionHook
	eventChan      chan WorkflowEvent

	mu sync.RWMutex
}

// ExecutorConfig holds executor configuration
type ExecutorConfig struct {
	MaxParallel    int           // Max concurrent node executions (0 = unlimited)
	DefaultTimeout time.Duration // Default timeout for nodes without explicit timeout
	EventBufferSize int          // Size of event channel buffer
}

// DefaultExecutorConfig returns default executor configuration
func DefaultExecutorConfig() *ExecutorConfig {
	return &ExecutorConfig{
		MaxParallel:     10,
		DefaultTimeout:  5 * time.Minute,
		EventBufferSize: 100,
	}
}

// NewDAGExecutor creates a new DAG executor
func NewDAGExecutor(dag *DAG, config *ExecutorConfig) *DAGExecutor {
	if config == nil {
		config = DefaultExecutorConfig()
	}

	return &DAGExecutor{
		dag:            dag,
		maxParallel:    config.MaxParallel,
		defaultTimeout: config.DefaultTimeout,
		hooks:          make([]ExecutionHook, 0),
		eventChan:      make(chan WorkflowEvent, config.EventBufferSize),
	}
}

// AddHook adds an execution hook
func (e *DAGExecutor) AddHook(hook ExecutionHook) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.hooks = append(e.hooks, hook)
}

// Execute runs the workflow synchronously
func (e *DAGExecutor) Execute(ctx context.Context, input map[string]interface{}) (*WorkflowResult, error) {
	// Validate DAG
	if err := e.dag.Validate(); err != nil {
		return nil, fmt.Errorf("invalid DAG: %w", err)
	}

	// Create run ID and state
	runID := uuid.New().String()
	state := NewWorkflowState(e.dag.ID, runID, input)
	state.Metrics.TotalNodes = len(e.dag.Nodes)

	// Initialize node states
	for nodeID := range e.dag.Nodes {
		state.SetNodeStatus(nodeID, NodeStatusPending)
	}

	// Create scheduler
	scheduler, err := NewScheduler(e.dag, state, &SchedulerConfig{
		MaxConcurrency: e.maxParallel,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	// Start execution
	state.SetStatus(WorkflowStatusRunning)
	e.emitEvent(WorkflowEventStarted, state, "", nil)
	e.callHooks(func(h ExecutionHook) { h.OnWorkflowStart(ctx, e.dag, state) })

	// Create dependency resolver
	resolver := NewDependencyResolver(e.dag, state)

	// Execute nodes using a worker pool pattern
	var wg sync.WaitGroup
	errChan := make(chan error, 1)
	var execErr error
	var execErrMu sync.Mutex

	// Worker loop - keeps fetching and executing tasks
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		for {
			select {
			case <-ctx.Done():
				scheduler.Stop()
				return
			default:
			}

			nodeID, err := scheduler.GetNextTask(ctx)
			if err != nil {
				if errors.Is(err, ErrNoMoreTasks) || errors.Is(err, ErrSchedulerStopped) {
					return
				}
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				select {
				case errChan <- err:
				default:
				}
				return
			}

			wg.Add(1)
			go func(nid string) {
				defer wg.Done()
				if nodeErr := e.executeNode(ctx, nid, state, scheduler, resolver); nodeErr != nil {
					execErrMu.Lock()
					if execErr == nil {
						execErr = nodeErr
					}
					execErrMu.Unlock()
				}
			}(nodeID)
		}
	}()

	// Wait for worker to finish (either all tasks done or error)
	<-workerDone

	// Wait for any remaining in-flight tasks
	wg.Wait()

	// Check for context cancellation
	select {
	case <-ctx.Done():
		state.SetStatus(WorkflowStatusCancelled)
		execErr = ctx.Err()
	default:
		// Check for errors from scheduler
		select {
		case err := <-errChan:
			state.SetStatus(WorkflowStatusFailed)
			state.Error = err.Error()
			execErr = err
		default:
			// Check execution error
			execErrMu.Lock()
			if execErr != nil {
				state.SetStatus(WorkflowStatusFailed)
				state.Error = execErr.Error()
			} else if scheduler.HasFailed() {
				state.SetStatus(WorkflowStatusFailed)
			} else {
				state.SetStatus(WorkflowStatusCompleted)
				state.Output = e.collectOutputs(state)
			}
			execErrMu.Unlock()
		}
	}

	// Call hooks
	e.callHooks(func(h ExecutionHook) { h.OnWorkflowEnd(ctx, e.dag, state) })
	e.emitEvent(WorkflowEventCompleted, state, "", nil)

	return &WorkflowResult{
		RunID:    runID,
		Status:   state.Status,
		Output:   state.Output,
		Error:    execErr,
		Metrics:  state.Metrics,
		State:    state,
		Duration: state.Metrics.TotalDuration,
	}, execErr
}

// Stream returns a channel of execution events
func (e *DAGExecutor) Stream(ctx context.Context, input map[string]interface{}) (<-chan WorkflowEvent, error) {
	eventChan := make(chan WorkflowEvent, 100)

	go func() {
		defer close(eventChan)

		result, _ := e.Execute(ctx, input)

		// Final event
		eventChan <- WorkflowEvent{
			Type:      WorkflowEventCompleted,
			Timestamp: time.Now(),
			State:     result.State,
		}
	}()

	return eventChan, nil
}

// executeNode executes a single node
func (e *DAGExecutor) executeNode(
	ctx context.Context,
	nodeID string,
	state *WorkflowState,
	scheduler *Scheduler,
	resolver *DependencyResolver,
) error {
	node, ok := e.dag.GetNode(nodeID)
	if !ok {
		return fmt.Errorf("node %s not found", nodeID)
	}

	// Get input for node
	input := resolver.GetInputForNode(nodeID)
	state.SetNodeInput(nodeID, input)

	// Emit node start event
	e.emitEvent(WorkflowEventNodeStarted, state, nodeID, nil)
	e.callHooks(func(h ExecutionHook) { h.OnNodeStart(ctx, node, state) })

	// Determine timeout
	timeout := node.Timeout
	if timeout == 0 {
		timeout = e.defaultTimeout
	}

	// Create context with timeout
	nodeCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		nodeCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Execute based on node type
	var output map[string]interface{}
	var err error

	switch node.Type {
	case NodeTypeTask:
		output, err = e.executeTaskNode(nodeCtx, node, input, state)

	case NodeTypeCondition:
		output, err = e.executeConditionNode(nodeCtx, node, input, state, scheduler)

	case NodeTypeLoop:
		output, err = e.executeLoopNode(nodeCtx, node, input, state, scheduler, resolver)

	case NodeTypeParallel:
		output, err = e.executeParallelNode(nodeCtx, node, input, state, scheduler)

	case NodeTypeJoin:
		output, err = e.executeJoinNode(nodeCtx, node, input, state)

	case NodeTypeSubDAG:
		output, err = e.executeSubDAGNode(nodeCtx, node, input, state)

	default:
		err = fmt.Errorf("unknown node type: %s", node.Type)
	}

	// Handle result
	if err != nil {
		// Check if we should retry
		if node.RetryPolicy != nil && e.shouldRetry(state, nodeID, node.RetryPolicy, err) {
			state.IncrementRetry(nodeID)
			e.emitEvent(WorkflowEventNodeRetrying, state, nodeID, err)
			scheduler.RetryTask(nodeID)
			return nil
		}

		state.SetNodeError(nodeID, err)
		scheduler.CompleteTask(nodeID, false)
		e.emitEvent(WorkflowEventNodeFailed, state, nodeID, err)
		e.callHooks(func(h ExecutionHook) { h.OnNodeEnd(ctx, node, state) })
		return fmt.Errorf("node %s failed: %w", nodeID, err)
	}

	state.SetNodeOutput(nodeID, output)
	scheduler.CompleteTask(nodeID, true)
	e.emitEvent(WorkflowEventNodeCompleted, state, nodeID, nil)
	e.callHooks(func(h ExecutionHook) { h.OnNodeEnd(ctx, node, state) })

	return nil
}

// executeTaskNode executes a task node
func (e *DAGExecutor) executeTaskNode(
	ctx context.Context,
	node *Node,
	input map[string]interface{},
	state *WorkflowState,
) (map[string]interface{}, error) {
	if node.Handler == nil {
		return nil, errors.New("task node has no handler")
	}

	attemptStart := time.Now()
	output, err := node.Handler(ctx, input)
	attemptEnd := time.Now()

	// Record attempt
	nodeState := state.GetNodeState(node.ID)
	attemptNum := 1
	if nodeState != nil {
		attemptNum = nodeState.RetryCount + 1
	}

	status := NodeStatusCompleted
	errMsg := ""
	if err != nil {
		status = NodeStatusFailed
		errMsg = err.Error()
	}

	state.RecordAttempt(node.ID, &NodeAttempt{
		AttemptNumber: attemptNum,
		StartTime:     attemptStart,
		EndTime:       attemptEnd,
		Status:        status,
		Output:        output,
		Error:         errMsg,
	})

	return output, err
}

// executeConditionNode evaluates a condition and schedules the appropriate branch
func (e *DAGExecutor) executeConditionNode(
	ctx context.Context,
	node *Node,
	input map[string]interface{},
	state *WorkflowState,
	scheduler *Scheduler,
) (map[string]interface{}, error) {
	if node.Condition == nil {
		return nil, errors.New("condition node has no condition function")
	}

	result, err := node.Condition(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("condition evaluation failed: %w", err)
	}

	output := map[string]interface{}{
		"condition_result": result,
	}

	// Skip the branch not taken
	if result {
		if node.FalseBranch != "" {
			scheduler.SkipNode(node.FalseBranch)
		}
		output["branch_taken"] = node.TrueBranch
	} else {
		if node.TrueBranch != "" {
			scheduler.SkipNode(node.TrueBranch)
		}
		output["branch_taken"] = node.FalseBranch
	}

	return output, nil
}

// executeLoopNode executes a loop node
func (e *DAGExecutor) executeLoopNode(
	ctx context.Context,
	node *Node,
	input map[string]interface{},
	state *WorkflowState,
	scheduler *Scheduler,
	resolver *DependencyResolver,
) (map[string]interface{}, error) {
	config := node.LoopConfig
	if config == nil {
		return nil, errors.New("loop node has no loop configuration")
	}

	// Initialize loop state
	loopState := &LoopState{
		CurrentIteration: 0,
		MaxIterations:    config.MaxIterations,
	}
	if ns := state.GetNodeState(node.ID); ns != nil {
		ns.LoopState = loopState
	}

	results := make([]map[string]interface{}, 0)

	switch config.Type {
	case LoopTypeWhile, LoopTypeUntil:
		for {
			if config.MaxIterations > 0 && loopState.CurrentIteration >= config.MaxIterations {
				break
			}

			// Check condition
			condResult, err := config.Condition(ctx, input)
			if err != nil {
				return nil, fmt.Errorf("loop condition failed: %w", err)
			}

			// While: continue if true, Until: continue if false
			shouldContinue := condResult
			if config.Type == LoopTypeUntil {
				shouldContinue = !condResult
			}

			if !shouldContinue {
				break
			}

			// Execute loop body (if there's a handler)
			if node.Handler != nil {
				iterInput := copyMapInterface(input)
				iterInput["_loop_iteration"] = loopState.CurrentIteration

				iterOutput, err := node.Handler(ctx, iterInput)
				if err != nil {
					if !config.ContinueOnFail {
						return nil, fmt.Errorf("loop iteration %d failed: %w", loopState.CurrentIteration, err)
					}
					loopState.FailedIterations = append(loopState.FailedIterations, loopState.CurrentIteration)
				} else {
					results = append(results, iterOutput)
					// Update input with output for next iteration
					for k, v := range iterOutput {
						input[k] = v
					}
				}
			}

			loopState.CurrentIteration++
			state.Metrics.LoopIterations++
		}

	case LoopTypeCount:
		for i := 0; i < config.MaxIterations; i++ {
			loopState.CurrentIteration = i

			if node.Handler != nil {
				iterInput := copyMapInterface(input)
				iterInput["_loop_iteration"] = i

				iterOutput, err := node.Handler(ctx, iterInput)
				if err != nil {
					if !config.ContinueOnFail {
						return nil, fmt.Errorf("loop iteration %d failed: %w", i, err)
					}
					loopState.FailedIterations = append(loopState.FailedIterations, i)
				} else {
					results = append(results, iterOutput)
				}
			}

			state.Metrics.LoopIterations++
		}

	case LoopTypeForEach:
		collection, ok := input[config.Collection]
		if !ok {
			return nil, fmt.Errorf("collection %s not found in input", config.Collection)
		}

		items, ok := collection.([]interface{})
		if !ok {
			return nil, fmt.Errorf("collection %s is not a slice", config.Collection)
		}

		loopState.CollectionSize = len(items)

		if config.Parallelism > 0 {
			// Parallel forEach
			results = e.executeParallelForEach(ctx, node, items, config, input, state, loopState)
		} else {
			// Sequential forEach
			for i, item := range items {
				loopState.CurrentIteration = i

				iterInput := copyMapInterface(input)
				iterInput[config.ItemVariable] = item
				if config.IndexVariable != "" {
					iterInput[config.IndexVariable] = i
				}

				if node.Handler != nil {
					iterOutput, err := node.Handler(ctx, iterInput)
					if err != nil {
						if !config.ContinueOnFail {
							return nil, fmt.Errorf("forEach iteration %d failed: %w", i, err)
						}
						loopState.FailedIterations = append(loopState.FailedIterations, i)
					} else {
						results = append(results, iterOutput)
					}
				}

				state.Metrics.LoopIterations++
			}
		}
	}

	loopState.IterationResults = results

	return map[string]interface{}{
		"iterations":        loopState.CurrentIteration,
		"results":          results,
		"failed_iterations": loopState.FailedIterations,
	}, nil
}

// executeParallelForEach executes forEach iterations in parallel
func (e *DAGExecutor) executeParallelForEach(
	ctx context.Context,
	node *Node,
	items []interface{},
	config *LoopConfig,
	input map[string]interface{},
	state *WorkflowState,
	loopState *LoopState,
) []map[string]interface{} {
	results := make([]map[string]interface{}, len(items))
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, config.Parallelism)

	for i, item := range items {
		wg.Add(1)
		go func(idx int, it interface{}) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			iterInput := copyMapInterface(input)
			iterInput[config.ItemVariable] = it
			if config.IndexVariable != "" {
				iterInput[config.IndexVariable] = idx
			}

			if node.Handler != nil {
				iterOutput, err := node.Handler(ctx, iterInput)
				mu.Lock()
				if err != nil {
					loopState.FailedIterations = append(loopState.FailedIterations, idx)
				} else {
					results[idx] = iterOutput
				}
				mu.Unlock()
			}

			state.Metrics.LoopIterations++
		}(i, item)
	}

	wg.Wait()
	return results
}

// executeParallelNode handles a parallel fork node
func (e *DAGExecutor) executeParallelNode(
	ctx context.Context,
	node *Node,
	input map[string]interface{},
	state *WorkflowState,
	scheduler *Scheduler,
) (map[string]interface{}, error) {
	// Parallel node just passes through - actual parallelism is handled by the scheduler
	// and the structure of the DAG (children of parallel node run concurrently)
	state.Metrics.ParallelBranches++
	return map[string]interface{}{
		"parallel_started": true,
		"node_id":          node.ID,
	}, nil
}

// executeJoinNode handles a join node
func (e *DAGExecutor) executeJoinNode(
	ctx context.Context,
	node *Node,
	input map[string]interface{},
	state *WorkflowState,
) (map[string]interface{}, error) {
	// Join node collects outputs from all parent nodes
	// Input already contains merged parent outputs from resolver
	return map[string]interface{}{
		"join_completed": true,
		"merged_input":   input,
	}, nil
}

// executeSubDAGNode executes a nested workflow
func (e *DAGExecutor) executeSubDAGNode(
	ctx context.Context,
	node *Node,
	input map[string]interface{},
	state *WorkflowState,
) (map[string]interface{}, error) {
	if node.SubDAG == nil {
		return nil, errors.New("subdag node has no sub-workflow")
	}

	// Create executor for sub-workflow
	subExecutor := NewDAGExecutor(node.SubDAG, &ExecutorConfig{
		MaxParallel:    e.maxParallel,
		DefaultTimeout: e.defaultTimeout,
	})

	// Execute sub-workflow
	result, err := subExecutor.Execute(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("sub-workflow failed: %w", err)
	}

	return result.Output, nil
}

// shouldRetry determines if a node should be retried
func (e *DAGExecutor) shouldRetry(state *WorkflowState, nodeID string, policy *RetryPolicy, err error) bool {
	nodeState := state.GetNodeState(nodeID)
	if nodeState == nil {
		return policy.MaxRetries > 0
	}

	if nodeState.RetryCount >= policy.MaxRetries {
		return false
	}

	// Check error types if specified
	if len(policy.RetryOn) > 0 {
		errStr := err.Error()
		for _, retryErr := range policy.RetryOn {
			if errStr == retryErr || contains(errStr, retryErr) {
				return true
			}
		}
		return false
	}

	return true
}

// collectOutputs gathers outputs from exit nodes
func (e *DAGExecutor) collectOutputs(state *WorkflowState) map[string]interface{} {
	outputs := make(map[string]interface{})

	exitNodes := e.dag.ExitNodes
	if len(exitNodes) == 0 {
		exitNodes = e.dag.FindExitNodes()
	}

	for _, nodeID := range exitNodes {
		nodeState := state.GetNodeState(nodeID)
		if nodeState != nil && nodeState.Output != nil {
			for k, v := range nodeState.Output {
				outputs[k] = v
			}
		}
	}

	return outputs
}

// emitEvent sends an event to the event channel
func (e *DAGExecutor) emitEvent(eventType WorkflowEventType, state *WorkflowState, nodeID string, err error) {
	event := WorkflowEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		State:     state,
		NodeID:    nodeID,
	}
	if err != nil {
		event.Error = err.Error()
	}

	select {
	case e.eventChan <- event:
	default:
		// Channel full, drop event
	}
}

// callHooks calls a function on all hooks
func (e *DAGExecutor) callHooks(fn func(ExecutionHook)) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, hook := range e.hooks {
		fn(hook)
	}
}

// Helper functions

func copyMapInterface(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// WorkflowResult represents the result of a workflow execution
type WorkflowResult struct {
	RunID    string
	Status   WorkflowStatus
	Output   map[string]interface{}
	Error    error
	Metrics  *WorkflowMetrics
	State    *WorkflowState
	Duration time.Duration
}

// IsSuccess returns true if the workflow completed successfully
func (r *WorkflowResult) IsSuccess() bool {
	return r.Status == WorkflowStatusCompleted
}

// WorkflowEventType defines the type of workflow event
type WorkflowEventType string

const (
	WorkflowEventStarted      WorkflowEventType = "workflow_started"
	WorkflowEventCompleted    WorkflowEventType = "workflow_completed"
	WorkflowEventFailed       WorkflowEventType = "workflow_failed"
	WorkflowEventNodeStarted  WorkflowEventType = "node_started"
	WorkflowEventNodeCompleted WorkflowEventType = "node_completed"
	WorkflowEventNodeFailed   WorkflowEventType = "node_failed"
	WorkflowEventNodeRetrying WorkflowEventType = "node_retrying"
	WorkflowEventNodeSkipped  WorkflowEventType = "node_skipped"
)

// WorkflowEvent represents an event during workflow execution
type WorkflowEvent struct {
	Type      WorkflowEventType
	Timestamp time.Time
	State     *WorkflowState
	NodeID    string
	Error     string
}

// ExecutionHook allows customization of workflow execution
type ExecutionHook interface {
	OnWorkflowStart(ctx context.Context, dag *DAG, state *WorkflowState)
	OnWorkflowEnd(ctx context.Context, dag *DAG, state *WorkflowState)
	OnNodeStart(ctx context.Context, node *Node, state *WorkflowState)
	OnNodeEnd(ctx context.Context, node *Node, state *WorkflowState)
}

// BaseExecutionHook provides default no-op implementations
type BaseExecutionHook struct{}

func (h *BaseExecutionHook) OnWorkflowStart(ctx context.Context, dag *DAG, state *WorkflowState) {}
func (h *BaseExecutionHook) OnWorkflowEnd(ctx context.Context, dag *DAG, state *WorkflowState)   {}
func (h *BaseExecutionHook) OnNodeStart(ctx context.Context, node *Node, state *WorkflowState)   {}
func (h *BaseExecutionHook) OnNodeEnd(ctx context.Context, node *Node, state *WorkflowState)     {}
