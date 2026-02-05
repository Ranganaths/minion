// Package workflow provides DAG-based parallel workflow execution for multi-agent systems.
package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// DAG represents a directed acyclic graph workflow
type DAG struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Nodes       map[string]*Node       `json:"nodes"`
	Edges       []*Edge                `json:"edges"`
	EntryNodes  []string               `json:"entry_nodes"`
	ExitNodes   []string               `json:"exit_nodes"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`

	mu sync.RWMutex
}

// Edge represents a connection between nodes
type Edge struct {
	From     string                 `json:"from"`
	To       string                 `json:"to"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NewDAG creates a new empty DAG
func NewDAG(id, name string) *DAG {
	return &DAG{
		ID:         id,
		Name:       name,
		Nodes:      make(map[string]*Node),
		Edges:      make([]*Edge, 0),
		EntryNodes: make([]string, 0),
		ExitNodes:  make([]string, 0),
		Metadata:   make(map[string]interface{}),
	}
}

// AddNode adds a node to the DAG
func (d *DAG) AddNode(node *Node) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if node.ID == "" {
		return errors.New("node ID cannot be empty")
	}

	if _, exists := d.Nodes[node.ID]; exists {
		return fmt.Errorf("node %s already exists", node.ID)
	}

	d.Nodes[node.ID] = node
	return nil
}

// AddEdge adds an edge between two nodes
func (d *DAG) AddEdge(from, to string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.Nodes[from]; !exists {
		return fmt.Errorf("source node %s does not exist", from)
	}
	if _, exists := d.Nodes[to]; !exists {
		return fmt.Errorf("target node %s does not exist", to)
	}

	// Check for duplicate edge
	for _, edge := range d.Edges {
		if edge.From == from && edge.To == to {
			return fmt.Errorf("edge from %s to %s already exists", from, to)
		}
	}

	d.Edges = append(d.Edges, &Edge{From: from, To: to})
	return nil
}

// RemoveNode removes a node and its associated edges
func (d *DAG) RemoveNode(nodeID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.Nodes[nodeID]; !exists {
		return fmt.Errorf("node %s does not exist", nodeID)
	}

	delete(d.Nodes, nodeID)

	// Remove associated edges
	filteredEdges := make([]*Edge, 0)
	for _, edge := range d.Edges {
		if edge.From != nodeID && edge.To != nodeID {
			filteredEdges = append(filteredEdges, edge)
		}
	}
	d.Edges = filteredEdges

	// Update entry/exit nodes
	d.EntryNodes = removeFromSlice(d.EntryNodes, nodeID)
	d.ExitNodes = removeFromSlice(d.ExitNodes, nodeID)

	return nil
}

// GetNode returns a node by ID
func (d *DAG) GetNode(nodeID string) (*Node, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	node, exists := d.Nodes[nodeID]
	return node, exists
}

// GetParents returns the parent nodes of a given node
func (d *DAG) GetParents(nodeID string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	parents := make([]string, 0)
	for _, edge := range d.Edges {
		if edge.To == nodeID {
			parents = append(parents, edge.From)
		}
	}
	return parents
}

// GetChildren returns the child nodes of a given node
func (d *DAG) GetChildren(nodeID string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	children := make([]string, 0)
	for _, edge := range d.Edges {
		if edge.From == nodeID {
			children = append(children, edge.To)
		}
	}
	return children
}

// Validate checks if the DAG is valid (no cycles, all references exist)
func (d *DAG) Validate() error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.Nodes) == 0 {
		return errors.New("DAG has no nodes")
	}

	// Check all edge references exist
	for _, edge := range d.Edges {
		if _, exists := d.Nodes[edge.From]; !exists {
			return fmt.Errorf("edge references non-existent source node: %s", edge.From)
		}
		if _, exists := d.Nodes[edge.To]; !exists {
			return fmt.Errorf("edge references non-existent target node: %s", edge.To)
		}
	}

	// Check for cycles using DFS
	if hasCycle := d.detectCycle(); hasCycle {
		return errors.New("DAG contains a cycle")
	}

	// Validate entry nodes
	for _, entryID := range d.EntryNodes {
		if _, exists := d.Nodes[entryID]; !exists {
			return fmt.Errorf("entry node %s does not exist", entryID)
		}
	}

	// Validate exit nodes
	for _, exitID := range d.ExitNodes {
		if _, exists := d.Nodes[exitID]; !exists {
			return fmt.Errorf("exit node %s does not exist", exitID)
		}
	}

	// Validate each node
	for _, node := range d.Nodes {
		if err := node.Validate(); err != nil {
			return fmt.Errorf("invalid node %s: %w", node.ID, err)
		}
	}

	return nil
}

// detectCycle performs cycle detection using DFS
func (d *DAG) detectCycle() bool {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(nodeID string) bool
	dfs = func(nodeID string) bool {
		visited[nodeID] = true
		recStack[nodeID] = true

		for _, child := range d.GetChildren(nodeID) {
			if !visited[child] {
				if dfs(child) {
					return true
				}
			} else if recStack[child] {
				return true
			}
		}

		recStack[nodeID] = false
		return false
	}

	for nodeID := range d.Nodes {
		if !visited[nodeID] {
			if dfs(nodeID) {
				return true
			}
		}
	}

	return false
}

// TopologicalSort returns nodes in topological order
func (d *DAG) TopologicalSort() ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Calculate in-degrees
	inDegree := make(map[string]int)
	for nodeID := range d.Nodes {
		inDegree[nodeID] = 0
	}
	for _, edge := range d.Edges {
		inDegree[edge.To]++
	}

	// Start with nodes that have no incoming edges
	queue := make([]string, 0)
	for nodeID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, nodeID)
		}
	}

	// Sort queue for deterministic ordering
	sort.Strings(queue)

	result := make([]string, 0, len(d.Nodes))

	for len(queue) > 0 {
		// Pop from queue
		nodeID := queue[0]
		queue = queue[1:]
		result = append(result, nodeID)

		// Reduce in-degree of children
		children := d.GetChildren(nodeID)
		for _, child := range children {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
			}
		}
		// Sort for determinism
		sort.Strings(queue)
	}

	if len(result) != len(d.Nodes) {
		return nil, errors.New("cycle detected during topological sort")
	}

	return result, nil
}

// FindEntryNodes finds nodes with no incoming edges
func (d *DAG) FindEntryNodes() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	hasIncoming := make(map[string]bool)
	for _, edge := range d.Edges {
		hasIncoming[edge.To] = true
	}

	entryNodes := make([]string, 0)
	for nodeID := range d.Nodes {
		if !hasIncoming[nodeID] {
			entryNodes = append(entryNodes, nodeID)
		}
	}

	sort.Strings(entryNodes)
	return entryNodes
}

// FindExitNodes finds nodes with no outgoing edges
func (d *DAG) FindExitNodes() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	hasOutgoing := make(map[string]bool)
	for _, edge := range d.Edges {
		hasOutgoing[edge.From] = true
	}

	exitNodes := make([]string, 0)
	for nodeID := range d.Nodes {
		if !hasOutgoing[nodeID] {
			exitNodes = append(exitNodes, nodeID)
		}
	}

	sort.Strings(exitNodes)
	return exitNodes
}

// AutoDetectEntryExitNodes sets entry and exit nodes automatically
func (d *DAG) AutoDetectEntryExitNodes() {
	d.EntryNodes = d.FindEntryNodes()
	d.ExitNodes = d.FindExitNodes()
}

// Clone creates a deep copy of the DAG
func (d *DAG) Clone() *DAG {
	d.mu.RLock()
	defer d.mu.RUnlock()

	clone := NewDAG(d.ID, d.Name)
	clone.Description = d.Description
	clone.Version = d.Version

	// Clone nodes
	for id, node := range d.Nodes {
		clone.Nodes[id] = node.Clone()
	}

	// Clone edges
	for _, edge := range d.Edges {
		clone.Edges = append(clone.Edges, &Edge{
			From:     edge.From,
			To:       edge.To,
			Metadata: copyMap(edge.Metadata),
		})
	}

	clone.EntryNodes = append(clone.EntryNodes, d.EntryNodes...)
	clone.ExitNodes = append(clone.ExitNodes, d.ExitNodes...)
	clone.Metadata = copyMap(d.Metadata)

	return clone
}

// ToDOT exports the DAG to DOT format for visualization
func (d *DAG) ToDOT() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("digraph %q {\n", d.Name))
	sb.WriteString("  rankdir=TB;\n")
	sb.WriteString("  node [shape=box];\n\n")

	// Add nodes with styling based on type
	for _, node := range d.Nodes {
		style := d.getNodeStyle(node)
		sb.WriteString(fmt.Sprintf("  %q [label=%q%s];\n", node.ID, node.Name, style))
	}

	sb.WriteString("\n")

	// Add edges
	for _, edge := range d.Edges {
		sb.WriteString(fmt.Sprintf("  %q -> %q;\n", edge.From, edge.To))
	}

	sb.WriteString("}\n")
	return sb.String()
}

func (d *DAG) getNodeStyle(node *Node) string {
	switch node.Type {
	case NodeTypeParallel:
		return ", shape=diamond, style=filled, fillcolor=lightblue"
	case NodeTypeJoin:
		return ", shape=diamond, style=filled, fillcolor=lightgreen"
	case NodeTypeCondition:
		return ", shape=diamond, style=filled, fillcolor=lightyellow"
	case NodeTypeLoop:
		return ", shape=oval, style=filled, fillcolor=lightpink"
	case NodeTypeSubDAG:
		return ", shape=box3d, style=filled, fillcolor=lavender"
	default:
		return ""
	}
}

// ToJSON exports the DAG to JSON
func (d *DAG) ToJSON() ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return json.MarshalIndent(d, "", "  ")
}

// FromJSON imports a DAG from JSON
func FromJSON(data []byte) (*DAG, error) {
	dag := &DAG{}
	if err := json.Unmarshal(data, dag); err != nil {
		return nil, fmt.Errorf("failed to unmarshal DAG: %w", err)
	}

	// Initialize nil maps
	if dag.Nodes == nil {
		dag.Nodes = make(map[string]*Node)
	}
	if dag.Metadata == nil {
		dag.Metadata = make(map[string]interface{})
	}

	return dag, nil
}

// Helper functions

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
