package workflow

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"
)

// Scheduler manages task scheduling and dependency resolution
type Scheduler struct {
	dag            *DAG
	state          *WorkflowState
	readyQueue     *priorityQueue
	inDegree       map[string]int    // Remaining dependencies
	nodeOrder      map[string]int    // Topological order for priority
	maxConcurrency int
	runningCount   int

	mu      sync.Mutex
	cond    *sync.Cond
	stopped bool
}

// SchedulerConfig holds scheduler configuration
type SchedulerConfig struct {
	MaxConcurrency int // Max concurrent tasks (0 = unlimited)
}

// NewScheduler creates a new scheduler for a DAG
func NewScheduler(dag *DAG, state *WorkflowState, config *SchedulerConfig) (*Scheduler, error) {
	// Compute topological order
	order, err := dag.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("failed to compute topological order: %w", err)
	}

	nodeOrder := make(map[string]int, len(order))
	for i, nodeID := range order {
		nodeOrder[nodeID] = i
	}

	// Compute initial in-degrees
	inDegree := make(map[string]int, len(dag.Nodes))
	for nodeID := range dag.Nodes {
		inDegree[nodeID] = len(dag.GetParents(nodeID))
	}

	maxConcurrency := 0
	if config != nil {
		maxConcurrency = config.MaxConcurrency
	}

	s := &Scheduler{
		dag:            dag,
		state:          state,
		readyQueue:     newPriorityQueue(),
		inDegree:       inDegree,
		nodeOrder:      nodeOrder,
		maxConcurrency: maxConcurrency,
	}
	s.cond = sync.NewCond(&s.mu)

	// Initialize ready queue with entry nodes
	for nodeID, degree := range inDegree {
		if degree == 0 {
			s.enqueue(nodeID)
		}
	}

	return s, nil
}

// GetNextTask returns the next task to execute, blocking if necessary
func (s *Scheduler) GetNextTask(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		// Check if stopped
		if s.stopped {
			return "", ErrSchedulerStopped
		}

		// Check context
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// Check concurrency limit
		if s.maxConcurrency > 0 && s.runningCount >= s.maxConcurrency {
			s.waitWithContext(ctx)
			continue
		}

		// Try to get a task
		if s.readyQueue.Len() > 0 {
			item := heap.Pop(s.readyQueue).(*queueItem)
			s.runningCount++
			s.state.SetNodeStatus(item.nodeID, NodeStatusRunning)
			return item.nodeID, nil
		}

		// No tasks available, check if all done
		if s.runningCount == 0 {
			// Nothing running, nothing ready - workflow is complete or stuck
			return "", ErrNoMoreTasks
		}

		// Wait for a task to complete
		s.waitWithContext(ctx)
	}
}

// CompleteTask marks a task as completed and schedules dependent tasks
func (s *Scheduler) CompleteTask(nodeID string, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.runningCount--

	if success {
		s.state.SetNodeStatus(nodeID, NodeStatusCompleted)

		// Schedule dependent nodes
		for _, childID := range s.dag.GetChildren(nodeID) {
			s.inDegree[childID]--
			if s.inDegree[childID] == 0 {
				// Check if node should be skipped (e.g., condition not met)
				childState := s.state.GetNodeState(childID)
				if childState == nil || childState.Status != NodeStatusSkipped {
					s.enqueue(childID)
				}
			}
		}
	} else {
		s.state.SetNodeStatus(nodeID, NodeStatusFailed)
	}

	s.cond.Broadcast()
}

// SkipNode marks a node as skipped and handles dependent nodes
func (s *Scheduler) SkipNode(nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.SetNodeStatus(nodeID, NodeStatusSkipped)

	// Handle dependent nodes - they need one less dependency
	for _, childID := range s.dag.GetChildren(nodeID) {
		s.inDegree[childID]--
		if s.inDegree[childID] == 0 {
			s.enqueue(childID)
		}
	}

	s.cond.Broadcast()
}

// RetryTask re-queues a task for retry
func (s *Scheduler) RetryTask(nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.runningCount--
	s.state.SetNodeStatus(nodeID, NodeStatusRetrying)
	s.enqueue(nodeID)
	s.cond.Broadcast()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopped = true
	s.cond.Broadcast()
}

// IsComplete returns true if all nodes are processed
func (s *Scheduler) IsComplete() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.runningCount > 0 {
		return false
	}

	if s.readyQueue.Len() > 0 {
		return false
	}

	// Check if all nodes are in terminal states
	for nodeID := range s.dag.Nodes {
		state := s.state.GetNodeState(nodeID)
		if state == nil {
			continue
		}
		switch state.Status {
		case NodeStatusCompleted, NodeStatusFailed, NodeStatusSkipped, NodeStatusCancelled:
			continue
		default:
			return false
		}
	}

	return true
}

// HasFailed returns true if any node has failed
func (s *Scheduler) HasFailed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, state := range s.state.NodeStates {
		if state.Status == NodeStatusFailed {
			return true
		}
	}
	return false
}

// GetProgress returns workflow progress metrics
func (s *Scheduler) GetProgress() (completed, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	total = len(s.dag.Nodes)
	for _, state := range s.state.NodeStates {
		if state.Status == NodeStatusCompleted || state.Status == NodeStatusSkipped {
			completed++
		}
	}
	return completed, total
}

// enqueue adds a node to the ready queue (must be called with lock held)
func (s *Scheduler) enqueue(nodeID string) {
	priority := s.nodeOrder[nodeID]
	heap.Push(s.readyQueue, &queueItem{
		nodeID:   nodeID,
		priority: priority,
	})
	s.state.SetNodeStatus(nodeID, NodeStatusWaiting)
}

// waitWithContext waits on condition variable with context support
func (s *Scheduler) waitWithContext(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			s.cond.Broadcast()
		case <-done:
		}
	}()

	s.cond.Wait()
	close(done)
}

// Errors
var (
	ErrSchedulerStopped = fmt.Errorf("scheduler has been stopped")
	ErrNoMoreTasks      = fmt.Errorf("no more tasks to execute")
)

// Priority Queue implementation for task scheduling

type queueItem struct {
	nodeID   string
	priority int // Lower is higher priority
	index    int // Index in heap
}

type priorityQueue []*queueItem

func newPriorityQueue() *priorityQueue {
	pq := make(priorityQueue, 0)
	heap.Init(&pq)
	return &pq
}

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].priority < pq[j].priority
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*queueItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // Avoid memory leak
	item.index = -1 // For safety
	*pq = old[0 : n-1]
	return item
}

// ScheduleGroup manages a group of nodes to be executed together (e.g., parallel fork)
type ScheduleGroup struct {
	ID        string
	NodeIDs   []string
	Type      NodeType
	Completed map[string]bool
	Failed    map[string]bool
	Results   map[string]map[string]interface{}
	mu        sync.Mutex
}

// NewScheduleGroup creates a new schedule group
func NewScheduleGroup(id string, nodeIDs []string, nodeType NodeType) *ScheduleGroup {
	return &ScheduleGroup{
		ID:        id,
		NodeIDs:   nodeIDs,
		Type:      nodeType,
		Completed: make(map[string]bool),
		Failed:    make(map[string]bool),
		Results:   make(map[string]map[string]interface{}),
	}
}

// MarkCompleted marks a node in the group as completed
func (g *ScheduleGroup) MarkCompleted(nodeID string, result map[string]interface{}) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Completed[nodeID] = true
	g.Results[nodeID] = result
}

// MarkFailed marks a node in the group as failed
func (g *ScheduleGroup) MarkFailed(nodeID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Failed[nodeID] = true
}

// IsComplete returns true if all nodes in the group are done
func (g *ScheduleGroup) IsComplete() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, nodeID := range g.NodeIDs {
		if !g.Completed[nodeID] && !g.Failed[nodeID] {
			return false
		}
	}
	return true
}

// HasFailed returns true if any node in the group failed
func (g *ScheduleGroup) HasFailed() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.Failed) > 0
}

// GetResults returns combined results from all nodes
func (g *ScheduleGroup) GetResults() map[string]interface{} {
	g.mu.Lock()
	defer g.mu.Unlock()

	combined := make(map[string]interface{})
	for nodeID, result := range g.Results {
		combined[nodeID] = result
	}
	return combined
}

// DependencyResolver helps resolve node dependencies
type DependencyResolver struct {
	dag   *DAG
	state *WorkflowState
}

// NewDependencyResolver creates a new dependency resolver
func NewDependencyResolver(dag *DAG, state *WorkflowState) *DependencyResolver {
	return &DependencyResolver{
		dag:   dag,
		state: state,
	}
}

// AreDependenciesMet checks if all dependencies of a node are satisfied
func (r *DependencyResolver) AreDependenciesMet(nodeID string) bool {
	parents := r.dag.GetParents(nodeID)
	for _, parentID := range parents {
		parentState := r.state.GetNodeState(parentID)
		if parentState == nil {
			return false
		}
		// Parent must be completed or skipped
		if parentState.Status != NodeStatusCompleted && parentState.Status != NodeStatusSkipped {
			return false
		}
	}
	return true
}

// GetDependencyOutputs collects outputs from all parent nodes
func (r *DependencyResolver) GetDependencyOutputs(nodeID string) map[string]interface{} {
	outputs := make(map[string]interface{})
	parents := r.dag.GetParents(nodeID)

	for _, parentID := range parents {
		parentState := r.state.GetNodeState(parentID)
		if parentState != nil && parentState.Output != nil {
			// Merge parent outputs
			for k, v := range parentState.Output {
				outputs[k] = v
			}
			// Also store by parent ID for disambiguation
			outputs["_parent_"+parentID] = parentState.Output
		}
	}

	return outputs
}

// GetInputForNode prepares the input for a node based on dependencies and workflow variables
func (r *DependencyResolver) GetInputForNode(nodeID string) map[string]interface{} {
	input := make(map[string]interface{})

	// Start with workflow input
	for k, v := range r.state.Input {
		input[k] = v
	}

	// Add workflow variables
	r.state.mu.RLock()
	for k, v := range r.state.Variables {
		input[k] = v
	}
	r.state.mu.RUnlock()

	// Add dependency outputs
	depOutputs := r.GetDependencyOutputs(nodeID)
	for k, v := range depOutputs {
		input[k] = v
	}

	return input
}

// ExecutionTimer tracks execution timing
type ExecutionTimer struct {
	start    time.Time
	timeout  time.Duration
	deadline time.Time
}

// NewExecutionTimer creates a new execution timer
func NewExecutionTimer(timeout time.Duration) *ExecutionTimer {
	now := time.Now()
	return &ExecutionTimer{
		start:    now,
		timeout:  timeout,
		deadline: now.Add(timeout),
	}
}

// Elapsed returns the elapsed time
func (t *ExecutionTimer) Elapsed() time.Duration {
	return time.Since(t.start)
}

// Remaining returns the remaining time
func (t *ExecutionTimer) Remaining() time.Duration {
	remaining := time.Until(t.deadline)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsExpired returns true if the timer has expired
func (t *ExecutionTimer) IsExpired() bool {
	return time.Now().After(t.deadline)
}

// Context returns a context with the deadline
func (t *ExecutionTimer) Context(parent context.Context) (context.Context, context.CancelFunc) {
	if t.timeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithDeadline(parent, t.deadline)
}
