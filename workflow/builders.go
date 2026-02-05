package workflow

import (
	"context"
	"time"
)

// DAGBuilder provides a fluent API for building DAGs
type DAGBuilder struct {
	dag          *DAG
	currentNode  *Node
	lastNodeID   string
	errors       []error
	nodeSequence []string
}

// NewDAGBuilder creates a new DAG builder
func NewDAGBuilder(id string) *DAGBuilder {
	return &DAGBuilder{
		dag: &DAG{
			ID:         id,
			Name:       id,
			Nodes:      make(map[string]*Node),
			Edges:      make([]*Edge, 0),
			EntryNodes: make([]string, 0),
			ExitNodes:  make([]string, 0),
			Metadata:   make(map[string]interface{}),
		},
		nodeSequence: make([]string, 0),
	}
}

// Name sets the DAG name
func (b *DAGBuilder) Name(name string) *DAGBuilder {
	b.dag.Name = name
	return b
}

// Description sets the DAG description
func (b *DAGBuilder) Description(desc string) *DAGBuilder {
	b.dag.Description = desc
	return b
}

// Version sets the DAG version
func (b *DAGBuilder) Version(version string) *DAGBuilder {
	b.dag.Version = version
	return b
}

// Task adds a task node and connects it to the previous node
func (b *DAGBuilder) Task(id, name string, handler TaskHandler) *DAGBuilder {
	node := NewTaskNode(id, name, handler)
	return b.addNode(node)
}

// Parallel adds a parallel fork node
func (b *DAGBuilder) Parallel(id string, branches ...*DAGBuilder) *DAGBuilder {
	node := NewParallelNode(id, "Parallel: "+id)
	b.addNode(node)

	// Add each branch and connect from parallel node
	for _, branch := range branches {
		for _, nodeID := range branch.nodeSequence {
			branchNode := branch.dag.Nodes[nodeID]
			if branchNode != nil {
				b.dag.Nodes[nodeID] = branchNode
			}
		}

		// Copy edges
		for _, edge := range branch.dag.Edges {
			b.dag.Edges = append(b.dag.Edges, edge)
		}

		// Connect parallel node to first node of each branch
		if len(branch.nodeSequence) > 0 {
			b.dag.Edges = append(b.dag.Edges, &Edge{
				From: id,
				To:   branch.nodeSequence[0],
			})
		}
	}

	return b
}

// ParallelTasks adds multiple task nodes that run in parallel
func (b *DAGBuilder) ParallelTasks(forkID string, tasks ...TaskDef) *DAGBuilder {
	// Create fork node
	forkNode := NewParallelNode(forkID, "Parallel: "+forkID)
	b.addNode(forkNode)

	// Add task nodes
	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskNode := NewTaskNode(task.ID, task.Name, task.Handler)
		if task.Timeout > 0 {
			taskNode.Timeout = task.Timeout
		}
		if task.RetryPolicy != nil {
			taskNode.RetryPolicy = task.RetryPolicy
		}
		b.dag.Nodes[task.ID] = taskNode

		// Connect fork to task
		b.dag.Edges = append(b.dag.Edges, &Edge{
			From: forkID,
			To:   task.ID,
		})

		taskIDs = append(taskIDs, task.ID)
	}

	// Create join node
	joinID := forkID + "_join"
	joinNode := NewJoinNode(joinID, "Join: "+forkID)
	b.dag.Nodes[joinID] = joinNode

	// Connect all tasks to join
	for _, taskID := range taskIDs {
		b.dag.Edges = append(b.dag.Edges, &Edge{
			From: taskID,
			To:   joinID,
		})
	}

	// Update last node to join
	b.lastNodeID = joinID
	b.nodeSequence = append(b.nodeSequence, joinID)

	return b
}

// Join adds a join node
func (b *DAGBuilder) Join(id string) *DAGBuilder {
	node := NewJoinNode(id, "Join: "+id)
	return b.addNode(node)
}

// Condition adds a condition node
func (b *DAGBuilder) Condition(id string, cond ConditionFunc) *ConditionBuilder {
	node := NewConditionNode(id, "Condition: "+id, cond, "", "")
	b.addNode(node)

	return &ConditionBuilder{
		parent:     b,
		condNodeID: id,
		condNode:   node,
	}
}

// Loop adds a loop node
func (b *DAGBuilder) Loop(id string, config *LoopConfig, handler TaskHandler) *DAGBuilder {
	node := NewLoopNode(id, "Loop: "+id, config)
	node.Handler = handler
	return b.addNode(node)
}

// While adds a while loop
func (b *DAGBuilder) While(id string, condition ConditionFunc, handler TaskHandler) *DAGBuilder {
	return b.Loop(id, &LoopConfig{
		Type:          LoopTypeWhile,
		Condition:     condition,
		MaxIterations: 1000, // Safety limit
	}, handler)
}

// Until adds an until loop
func (b *DAGBuilder) Until(id string, condition ConditionFunc, handler TaskHandler) *DAGBuilder {
	return b.Loop(id, &LoopConfig{
		Type:          LoopTypeUntil,
		Condition:     condition,
		MaxIterations: 1000,
	}, handler)
}

// ForEach adds a forEach loop
func (b *DAGBuilder) ForEach(id, collection, itemVar string, handler TaskHandler) *ForEachBuilder {
	return &ForEachBuilder{
		parent:     b,
		id:         id,
		collection: collection,
		itemVar:    itemVar,
		handler:    handler,
	}
}

// Times adds a count loop
func (b *DAGBuilder) Times(id string, count int, handler TaskHandler) *DAGBuilder {
	return b.Loop(id, &LoopConfig{
		Type:          LoopTypeCount,
		MaxIterations: count,
	}, handler)
}

// SubWorkflow adds a sub-workflow node
func (b *DAGBuilder) SubWorkflow(id string, subDAG *DAG) *DAGBuilder {
	node := NewSubDAGNode(id, "SubWorkflow: "+id, subDAG)
	return b.addNode(node)
}

// Connect explicitly connects two nodes
func (b *DAGBuilder) Connect(from, to string) *DAGBuilder {
	b.dag.Edges = append(b.dag.Edges, &Edge{From: from, To: to})
	return b
}

// WithTimeout sets the timeout for the last added node
func (b *DAGBuilder) WithTimeout(timeout time.Duration) *DAGBuilder {
	if b.currentNode != nil {
		b.currentNode.Timeout = timeout
	}
	return b
}

// WithRetry sets the retry policy for the last added node
func (b *DAGBuilder) WithRetry(maxRetries int, initialInterval time.Duration) *DAGBuilder {
	if b.currentNode != nil {
		b.currentNode.RetryPolicy = &RetryPolicy{
			MaxRetries:      maxRetries,
			InitialInterval: initialInterval,
			MaxInterval:     30 * time.Second,
			Multiplier:      2.0,
		}
	}
	return b
}

// WithRetryPolicy sets a custom retry policy for the last added node
func (b *DAGBuilder) WithRetryPolicy(policy *RetryPolicy) *DAGBuilder {
	if b.currentNode != nil {
		b.currentNode.RetryPolicy = policy
	}
	return b
}

// WithMetadata adds metadata to the last added node
func (b *DAGBuilder) WithMetadata(key string, value interface{}) *DAGBuilder {
	if b.currentNode != nil {
		b.currentNode.WithMetadata(key, value)
	}
	return b
}

// Build validates and returns the DAG
func (b *DAGBuilder) Build() (*DAG, error) {
	// Auto-detect entry and exit nodes
	b.dag.AutoDetectEntryExitNodes()

	// Validate
	if err := b.dag.Validate(); err != nil {
		return nil, err
	}

	return b.dag, nil
}

// MustBuild validates and returns the DAG, panicking on error
func (b *DAGBuilder) MustBuild() *DAG {
	dag, err := b.Build()
	if err != nil {
		panic(err)
	}
	return dag
}

// addNode adds a node and connects it to the previous node
func (b *DAGBuilder) addNode(node *Node) *DAGBuilder {
	b.dag.Nodes[node.ID] = node

	// Connect from previous node if exists
	if b.lastNodeID != "" {
		b.dag.Edges = append(b.dag.Edges, &Edge{
			From: b.lastNodeID,
			To:   node.ID,
		})
	}

	b.currentNode = node
	b.lastNodeID = node.ID
	b.nodeSequence = append(b.nodeSequence, node.ID)

	return b
}

// TaskDef defines a task for parallel execution
type TaskDef struct {
	ID          string
	Name        string
	Handler     TaskHandler
	Timeout     time.Duration
	RetryPolicy *RetryPolicy
}

// ConditionBuilder helps build conditional branches
type ConditionBuilder struct {
	parent       *DAGBuilder
	condNodeID   string
	condNode     *Node
	trueBranch   *DAGBuilder
	falseBranch  *DAGBuilder
}

// Then sets the true branch
func (c *ConditionBuilder) Then(branch *DAGBuilder) *ConditionBuilder {
	c.trueBranch = branch
	if len(branch.nodeSequence) > 0 {
		c.condNode.TrueBranch = branch.nodeSequence[0]
	}
	return c
}

// ThenTask sets a single task as the true branch
func (c *ConditionBuilder) ThenTask(id, name string, handler TaskHandler) *ConditionBuilder {
	branch := NewDAGBuilder(id + "_branch")
	branch.Task(id, name, handler)
	return c.Then(branch)
}

// Else sets the false branch
func (c *ConditionBuilder) Else(branch *DAGBuilder) *ConditionBuilder {
	c.falseBranch = branch
	if len(branch.nodeSequence) > 0 {
		c.condNode.FalseBranch = branch.nodeSequence[0]
	}
	return c
}

// ElseTask sets a single task as the false branch
func (c *ConditionBuilder) ElseTask(id, name string, handler TaskHandler) *ConditionBuilder {
	branch := NewDAGBuilder(id + "_branch")
	branch.Task(id, name, handler)
	return c.Else(branch)
}

// End finishes the condition and returns to the parent builder
func (c *ConditionBuilder) End() *DAGBuilder {
	// Add true branch nodes and edges
	if c.trueBranch != nil {
		for nodeID, node := range c.trueBranch.dag.Nodes {
			c.parent.dag.Nodes[nodeID] = node
		}
		for _, edge := range c.trueBranch.dag.Edges {
			c.parent.dag.Edges = append(c.parent.dag.Edges, edge)
		}

		// Connect condition to true branch
		if len(c.trueBranch.nodeSequence) > 0 {
			c.parent.dag.Edges = append(c.parent.dag.Edges, &Edge{
				From: c.condNodeID,
				To:   c.trueBranch.nodeSequence[0],
			})
		}
	}

	// Add false branch nodes and edges
	if c.falseBranch != nil {
		for nodeID, node := range c.falseBranch.dag.Nodes {
			c.parent.dag.Nodes[nodeID] = node
		}
		for _, edge := range c.falseBranch.dag.Edges {
			c.parent.dag.Edges = append(c.parent.dag.Edges, edge)
		}

		// Connect condition to false branch
		if len(c.falseBranch.nodeSequence) > 0 {
			c.parent.dag.Edges = append(c.parent.dag.Edges, &Edge{
				From: c.condNodeID,
				To:   c.falseBranch.nodeSequence[0],
			})
		}
	}

	return c.parent
}

// EndWithJoin finishes the condition with a join node
func (c *ConditionBuilder) EndWithJoin(joinID string) *DAGBuilder {
	c.End()

	// Create join node
	joinNode := NewJoinNode(joinID, "Join: "+joinID)
	c.parent.dag.Nodes[joinID] = joinNode

	// Connect branch ends to join
	if c.trueBranch != nil && len(c.trueBranch.nodeSequence) > 0 {
		lastTrue := c.trueBranch.nodeSequence[len(c.trueBranch.nodeSequence)-1]
		c.parent.dag.Edges = append(c.parent.dag.Edges, &Edge{
			From: lastTrue,
			To:   joinID,
		})
	}

	if c.falseBranch != nil && len(c.falseBranch.nodeSequence) > 0 {
		lastFalse := c.falseBranch.nodeSequence[len(c.falseBranch.nodeSequence)-1]
		c.parent.dag.Edges = append(c.parent.dag.Edges, &Edge{
			From: lastFalse,
			To:   joinID,
		})
	}

	c.parent.lastNodeID = joinID
	c.parent.nodeSequence = append(c.parent.nodeSequence, joinID)

	return c.parent
}

// ForEachBuilder helps build forEach loops
type ForEachBuilder struct {
	parent      *DAGBuilder
	id          string
	collection  string
	itemVar     string
	indexVar    string
	handler     TaskHandler
	parallelism int
	continueOnFail bool
	maxIterations int
}

// WithIndex sets the index variable name
func (f *ForEachBuilder) WithIndex(indexVar string) *ForEachBuilder {
	f.indexVar = indexVar
	return f
}

// Parallel sets the parallelism level
func (f *ForEachBuilder) Parallel(n int) *ForEachBuilder {
	f.parallelism = n
	return f
}

// ContinueOnFail allows the loop to continue even if some iterations fail
func (f *ForEachBuilder) ContinueOnFail() *ForEachBuilder {
	f.continueOnFail = true
	return f
}

// MaxIterations sets a safety limit on iterations
func (f *ForEachBuilder) MaxIterations(n int) *ForEachBuilder {
	f.maxIterations = n
	return f
}

// End finishes the forEach configuration
func (f *ForEachBuilder) End() *DAGBuilder {
	config := &LoopConfig{
		Type:           LoopTypeForEach,
		Collection:     f.collection,
		ItemVariable:   f.itemVar,
		IndexVariable:  f.indexVar,
		Parallelism:    f.parallelism,
		ContinueOnFail: f.continueOnFail,
		MaxIterations:  f.maxIterations,
	}

	node := NewLoopNode(f.id, "ForEach: "+f.id, config)
	node.Handler = f.handler

	return f.parent.addNode(node)
}

// Branch creates a new builder for a branch (used in parallel/condition)
func Branch() *DAGBuilder {
	return NewDAGBuilder("")
}

// Common handler factories

// PassThrough creates a handler that passes input to output
func PassThrough() TaskHandler {
	return func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return input, nil
	}
}

// SetOutput creates a handler that sets specific output values
func SetOutput(output map[string]interface{}) TaskHandler {
	return func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		result := make(map[string]interface{})
		for k, v := range input {
			result[k] = v
		}
		for k, v := range output {
			result[k] = v
		}
		return result, nil
	}
}

// Transform creates a handler that transforms input using a function
func Transform(fn func(map[string]interface{}) (map[string]interface{}, error)) TaskHandler {
	return func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return fn(input)
	}
}

// Common condition factories

// Always returns a condition that always returns true
func Always() ConditionFunc {
	return func(ctx context.Context, input map[string]interface{}) (bool, error) {
		return true, nil
	}
}

// Never returns a condition that always returns false
func Never() ConditionFunc {
	return func(ctx context.Context, input map[string]interface{}) (bool, error) {
		return false, nil
	}
}

// HasKey returns a condition that checks if a key exists in input
func HasKey(key string) ConditionFunc {
	return func(ctx context.Context, input map[string]interface{}) (bool, error) {
		_, ok := input[key]
		return ok, nil
	}
}

// KeyEquals returns a condition that checks if a key equals a value
func KeyEquals(key string, value interface{}) ConditionFunc {
	return func(ctx context.Context, input map[string]interface{}) (bool, error) {
		v, ok := input[key]
		if !ok {
			return false, nil
		}
		return v == value, nil
	}
}

// KeyTrue returns a condition that checks if a key is true
func KeyTrue(key string) ConditionFunc {
	return func(ctx context.Context, input map[string]interface{}) (bool, error) {
		v, ok := input[key]
		if !ok {
			return false, nil
		}
		b, ok := v.(bool)
		return ok && b, nil
	}
}

// Not negates a condition
func Not(cond ConditionFunc) ConditionFunc {
	return func(ctx context.Context, input map[string]interface{}) (bool, error) {
		result, err := cond(ctx, input)
		return !result, err
	}
}

// And combines conditions with AND
func And(conditions ...ConditionFunc) ConditionFunc {
	return func(ctx context.Context, input map[string]interface{}) (bool, error) {
		for _, cond := range conditions {
			result, err := cond(ctx, input)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
		return true, nil
	}
}

// Or combines conditions with OR
func Or(conditions ...ConditionFunc) ConditionFunc {
	return func(ctx context.Context, input map[string]interface{}) (bool, error) {
		for _, cond := range conditions {
			result, err := cond(ctx, input)
			if err != nil {
				return false, err
			}
			if result {
				return true, nil
			}
		}
		return false, nil
	}
}
