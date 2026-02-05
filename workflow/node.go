package workflow

import (
	"context"
	"errors"
	"time"
)

// NodeType defines the type of workflow node
type NodeType string

const (
	// NodeTypeTask represents a single task execution
	NodeTypeTask NodeType = "task"
	// NodeTypeParallel represents a fork - execute children in parallel
	NodeTypeParallel NodeType = "parallel"
	// NodeTypeJoin represents a join - wait for all parents
	NodeTypeJoin NodeType = "join"
	// NodeTypeCondition represents if/else branching
	NodeTypeCondition NodeType = "condition"
	// NodeTypeLoop represents while/for loops
	NodeTypeLoop NodeType = "loop"
	// NodeTypeSubDAG represents a nested workflow
	NodeTypeSubDAG NodeType = "subdag"
)

// TaskHandler is the function signature for task execution
type TaskHandler func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)

// ConditionFunc evaluates a condition and returns true/false
type ConditionFunc func(ctx context.Context, input map[string]interface{}) (bool, error)

// Node represents a workflow node
type Node struct {
	ID          string                 `json:"id"`
	Type        NodeType               `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Timeout     time.Duration          `json:"timeout,omitempty"`
	RetryPolicy *RetryPolicy           `json:"retry_policy,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`

	// For task nodes
	Handler TaskHandler `json:"-"`

	// For condition nodes
	Condition     ConditionFunc `json:"-"`
	TrueBranch    string        `json:"true_branch,omitempty"`
	FalseBranch   string        `json:"false_branch,omitempty"`
	ConditionExpr string        `json:"condition_expr,omitempty"` // For serialization

	// For loop nodes
	LoopConfig *LoopConfig `json:"loop_config,omitempty"`

	// For subdag nodes
	SubDAG *DAG `json:"sub_dag,omitempty"`

	// For parallel nodes
	ParallelConfig *ParallelConfig `json:"parallel_config,omitempty"`
}

// RetryPolicy defines retry behavior for a node
type RetryPolicy struct {
	MaxRetries      int           `json:"max_retries"`
	InitialInterval time.Duration `json:"initial_interval"`
	MaxInterval     time.Duration `json:"max_interval"`
	Multiplier      float64       `json:"multiplier"`
	RetryOn         []string      `json:"retry_on,omitempty"` // Error types to retry on
}

// LoopConfig defines loop behavior
type LoopConfig struct {
	Type          LoopType      `json:"type"`
	MaxIterations int           `json:"max_iterations"`
	Condition     ConditionFunc `json:"-"`
	ConditionExpr string        `json:"condition_expr,omitempty"`

	// For forEach loops
	Collection     string `json:"collection,omitempty"`      // Key in input containing collection
	ItemVariable   string `json:"item_variable,omitempty"`   // Variable name for current item
	IndexVariable  string `json:"index_variable,omitempty"`  // Variable name for index
	Parallelism    int    `json:"parallelism,omitempty"`     // Max parallel iterations (0 = sequential)
	ContinueOnFail bool   `json:"continue_on_fail,omitempty"` // Continue if one iteration fails
}

// LoopType defines the type of loop
type LoopType string

const (
	// LoopTypeWhile loops while condition is true
	LoopTypeWhile LoopType = "while"
	// LoopTypeUntil loops until condition is true
	LoopTypeUntil LoopType = "until"
	// LoopTypeForEach loops over a collection
	LoopTypeForEach LoopType = "foreach"
	// LoopTypeCount loops a fixed number of times
	LoopTypeCount LoopType = "count"
)

// ParallelConfig defines parallel execution behavior
type ParallelConfig struct {
	MaxConcurrency int  `json:"max_concurrency,omitempty"` // Max parallel executions (0 = unlimited)
	FailFast       bool `json:"fail_fast,omitempty"`       // Stop on first failure
	WaitAll        bool `json:"wait_all,omitempty"`        // Wait for all to complete even on failure
}

// NewTaskNode creates a new task node
func NewTaskNode(id, name string, handler TaskHandler) *Node {
	return &Node{
		ID:       id,
		Type:     NodeTypeTask,
		Name:     name,
		Handler:  handler,
		Metadata: make(map[string]interface{}),
	}
}

// NewParallelNode creates a new parallel (fork) node
func NewParallelNode(id, name string) *Node {
	return &Node{
		ID:   id,
		Type: NodeTypeParallel,
		Name: name,
		ParallelConfig: &ParallelConfig{
			MaxConcurrency: 0, // Unlimited by default
			FailFast:       false,
			WaitAll:        true,
		},
		Metadata: make(map[string]interface{}),
	}
}

// NewJoinNode creates a new join node
func NewJoinNode(id, name string) *Node {
	return &Node{
		ID:       id,
		Type:     NodeTypeJoin,
		Name:     name,
		Metadata: make(map[string]interface{}),
	}
}

// NewConditionNode creates a new condition node
func NewConditionNode(id, name string, condition ConditionFunc, trueBranch, falseBranch string) *Node {
	return &Node{
		ID:          id,
		Type:        NodeTypeCondition,
		Name:        name,
		Condition:   condition,
		TrueBranch:  trueBranch,
		FalseBranch: falseBranch,
		Metadata:    make(map[string]interface{}),
	}
}

// NewLoopNode creates a new loop node
func NewLoopNode(id, name string, config *LoopConfig) *Node {
	return &Node{
		ID:         id,
		Type:       NodeTypeLoop,
		Name:       name,
		LoopConfig: config,
		Metadata:   make(map[string]interface{}),
	}
}

// NewSubDAGNode creates a new sub-workflow node
func NewSubDAGNode(id, name string, subDAG *DAG) *Node {
	return &Node{
		ID:       id,
		Type:     NodeTypeSubDAG,
		Name:     name,
		SubDAG:   subDAG,
		Metadata: make(map[string]interface{}),
	}
}

// WithTimeout sets the timeout for the node
func (n *Node) WithTimeout(timeout time.Duration) *Node {
	n.Timeout = timeout
	return n
}

// WithRetryPolicy sets the retry policy for the node
func (n *Node) WithRetryPolicy(policy *RetryPolicy) *Node {
	n.RetryPolicy = policy
	return n
}

// WithMetadata adds metadata to the node
func (n *Node) WithMetadata(key string, value interface{}) *Node {
	if n.Metadata == nil {
		n.Metadata = make(map[string]interface{})
	}
	n.Metadata[key] = value
	return n
}

// Validate checks if the node is properly configured
func (n *Node) Validate() error {
	if n.ID == "" {
		return errors.New("node ID cannot be empty")
	}
	if n.Name == "" {
		return errors.New("node name cannot be empty")
	}

	switch n.Type {
	case NodeTypeTask:
		if n.Handler == nil {
			return errors.New("task node requires a handler")
		}

	case NodeTypeCondition:
		if n.Condition == nil && n.ConditionExpr == "" {
			return errors.New("condition node requires a condition function or expression")
		}
		if n.TrueBranch == "" {
			return errors.New("condition node requires a true branch")
		}

	case NodeTypeLoop:
		if n.LoopConfig == nil {
			return errors.New("loop node requires loop configuration")
		}
		if err := n.LoopConfig.Validate(); err != nil {
			return err
		}

	case NodeTypeSubDAG:
		if n.SubDAG == nil {
			return errors.New("subdag node requires a sub-workflow")
		}
		if err := n.SubDAG.Validate(); err != nil {
			return errors.New("invalid sub-workflow: " + err.Error())
		}

	case NodeTypeParallel, NodeTypeJoin:
		// These nodes don't require additional configuration

	default:
		return errors.New("unknown node type: " + string(n.Type))
	}

	// Validate retry policy if present
	if n.RetryPolicy != nil {
		if err := n.RetryPolicy.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Clone creates a deep copy of the node
func (n *Node) Clone() *Node {
	clone := &Node{
		ID:            n.ID,
		Type:          n.Type,
		Name:          n.Name,
		Description:   n.Description,
		Timeout:       n.Timeout,
		Handler:       n.Handler,
		Condition:     n.Condition,
		TrueBranch:    n.TrueBranch,
		FalseBranch:   n.FalseBranch,
		ConditionExpr: n.ConditionExpr,
		Metadata:      copyMap(n.Metadata),
	}

	if n.RetryPolicy != nil {
		clone.RetryPolicy = n.RetryPolicy.Clone()
	}

	if n.LoopConfig != nil {
		clone.LoopConfig = n.LoopConfig.Clone()
	}

	if n.SubDAG != nil {
		clone.SubDAG = n.SubDAG.Clone()
	}

	if n.ParallelConfig != nil {
		clone.ParallelConfig = n.ParallelConfig.Clone()
	}

	return clone
}

// Validate checks if the retry policy is valid
func (p *RetryPolicy) Validate() error {
	if p.MaxRetries < 0 {
		return errors.New("max retries cannot be negative")
	}
	if p.InitialInterval < 0 {
		return errors.New("initial interval cannot be negative")
	}
	if p.MaxInterval < 0 {
		return errors.New("max interval cannot be negative")
	}
	if p.Multiplier < 0 {
		return errors.New("multiplier cannot be negative")
	}
	return nil
}

// Clone creates a deep copy of the retry policy
func (p *RetryPolicy) Clone() *RetryPolicy {
	clone := &RetryPolicy{
		MaxRetries:      p.MaxRetries,
		InitialInterval: p.InitialInterval,
		MaxInterval:     p.MaxInterval,
		Multiplier:      p.Multiplier,
	}
	if p.RetryOn != nil {
		clone.RetryOn = make([]string, len(p.RetryOn))
		copy(clone.RetryOn, p.RetryOn)
	}
	return clone
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:      3,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		Multiplier:      2.0,
	}
}

// Validate checks if the loop config is valid
func (c *LoopConfig) Validate() error {
	switch c.Type {
	case LoopTypeWhile, LoopTypeUntil:
		if c.Condition == nil && c.ConditionExpr == "" {
			return errors.New("while/until loop requires a condition")
		}
	case LoopTypeForEach:
		if c.Collection == "" {
			return errors.New("forEach loop requires a collection key")
		}
		if c.ItemVariable == "" {
			return errors.New("forEach loop requires an item variable name")
		}
	case LoopTypeCount:
		if c.MaxIterations <= 0 {
			return errors.New("count loop requires positive max iterations")
		}
	default:
		return errors.New("unknown loop type: " + string(c.Type))
	}

	if c.MaxIterations < 0 {
		return errors.New("max iterations cannot be negative")
	}

	return nil
}

// Clone creates a deep copy of the loop config
func (c *LoopConfig) Clone() *LoopConfig {
	return &LoopConfig{
		Type:           c.Type,
		MaxIterations:  c.MaxIterations,
		Condition:      c.Condition,
		ConditionExpr:  c.ConditionExpr,
		Collection:     c.Collection,
		ItemVariable:   c.ItemVariable,
		IndexVariable:  c.IndexVariable,
		Parallelism:    c.Parallelism,
		ContinueOnFail: c.ContinueOnFail,
	}
}

// Clone creates a deep copy of the parallel config
func (c *ParallelConfig) Clone() *ParallelConfig {
	return &ParallelConfig{
		MaxConcurrency: c.MaxConcurrency,
		FailFast:       c.FailFast,
		WaitAll:        c.WaitAll,
	}
}
