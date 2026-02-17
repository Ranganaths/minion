package skills

import (
	"fmt"
)

// DependencyResolver resolves skill dependencies using topological sort
type DependencyResolver struct {
	registry skillGetter
}

// skillGetter is an interface for getting skills (allows testing)
type skillGetter interface {
	Get(name string) (Skill, error)
	HasSkill(name string) bool
}

// NewDependencyResolver creates a new dependency resolver
func NewDependencyResolver(registry skillGetter) *DependencyResolver {
	return &DependencyResolver{
		registry: registry,
	}
}

// Resolve returns all dependencies for a skill in execution order (topologically sorted)
// The returned slice includes only dependencies, not the skill itself
func (r *DependencyResolver) Resolve(skillName string) ([]Skill, error) {
	skill, err := r.registry.Get(skillName)
	if err != nil {
		return nil, err
	}

	// Build dependency graph
	graph := make(map[string][]string)
	visited := make(map[string]bool)

	if err := r.buildGraph(skillName, graph, visited); err != nil {
		return nil, err
	}

	// Detect cycles
	if err := r.detectCycles(skillName, graph); err != nil {
		return nil, err
	}

	// Topological sort
	order, err := r.topologicalSort(graph)
	if err != nil {
		return nil, err
	}

	// Remove the original skill from the order (we only want dependencies)
	deps := make([]Skill, 0, len(order)-1)
	for _, name := range order {
		if name != skill.Name() {
			dep, err := r.registry.Get(name)
			if err != nil {
				return nil, err
			}
			deps = append(deps, dep)
		}
	}

	return deps, nil
}

// buildGraph recursively builds the dependency graph
func (r *DependencyResolver) buildGraph(skillName string, graph map[string][]string, visited map[string]bool) error {
	if visited[skillName] {
		return nil
	}
	visited[skillName] = true

	skill, err := r.registry.Get(skillName)
	if err != nil {
		return fmt.Errorf("dependency not found: %s", skillName)
	}

	deps := skill.Dependencies()
	graph[skillName] = deps

	for _, dep := range deps {
		if !r.registry.HasSkill(dep) {
			return fmt.Errorf("dependency '%s' of skill '%s' not found in registry", dep, skillName)
		}
		if err := r.buildGraph(dep, graph, visited); err != nil {
			return err
		}
	}

	return nil
}

// detectCycles detects cycles in the dependency graph using DFS
func (r *DependencyResolver) detectCycles(startSkill string, graph map[string][]string) error {
	// States: 0 = unvisited, 1 = in current path, 2 = completed
	state := make(map[string]int)

	var dfs func(node string, path []string) error
	dfs = func(node string, path []string) error {
		state[node] = 1 // Mark as in current path

		for _, dep := range graph[node] {
			if state[dep] == 1 {
				// Found a cycle - build the cycle path for error message
				cyclePath := append(path, node, dep)
				return fmt.Errorf("%w: %v", ErrCircularDependency, cyclePath)
			}
			if state[dep] == 0 {
				if err := dfs(dep, append(path, node)); err != nil {
					return err
				}
			}
		}

		state[node] = 2 // Mark as completed
		return nil
	}

	return dfs(startSkill, nil)
}

// topologicalSort performs a DFS-based topological sort
// Returns nodes in dependency order (dependencies first, then dependents)
func (r *DependencyResolver) topologicalSort(graph map[string][]string) ([]string, error) {
	visited := make(map[string]bool)
	var result []string

	// DFS function to visit nodes
	var visit func(node string) error
	visit = func(node string) error {
		if visited[node] {
			return nil
		}
		visited[node] = true

		// Visit all dependencies first
		for _, dep := range graph[node] {
			if _, exists := graph[dep]; exists {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}

		// Add this node after all its dependencies
		result = append(result, node)
		return nil
	}

	// Visit all nodes in the graph
	for node := range graph {
		if err := visit(node); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// TopologicalSort performs a topological sort on a list of skill names
// Returns the skills in dependency order (dependencies first)
func (r *DependencyResolver) TopologicalSort(skillNames []string) ([]string, error) {
	// Build combined graph for all skills
	graph := make(map[string][]string)
	visited := make(map[string]bool)

	for _, name := range skillNames {
		if err := r.buildGraph(name, graph, visited); err != nil {
			return nil, err
		}
	}

	// Detect cycles across all skills
	for _, name := range skillNames {
		if err := r.detectCycles(name, graph); err != nil {
			return nil, err
		}
	}

	return r.topologicalSort(graph)
}

// DetectCycles checks if adding a skill would create a circular dependency
func (r *DependencyResolver) DetectCycles(skillName string) error {
	if !r.registry.HasSkill(skillName) {
		return fmt.Errorf("skill not found: %s", skillName)
	}

	graph := make(map[string][]string)
	visited := make(map[string]bool)

	if err := r.buildGraph(skillName, graph, visited); err != nil {
		return err
	}

	return r.detectCycles(skillName, graph)
}

// GetDependencyChain returns the full dependency chain for a skill
// including transitive dependencies, in order from leaf to root
func (r *DependencyResolver) GetDependencyChain(skillName string) ([]string, error) {
	if !r.registry.HasSkill(skillName) {
		return nil, fmt.Errorf("skill not found: %s", skillName)
	}

	graph := make(map[string][]string)
	visited := make(map[string]bool)

	if err := r.buildGraph(skillName, graph, visited); err != nil {
		return nil, err
	}

	if err := r.detectCycles(skillName, graph); err != nil {
		return nil, err
	}

	order, err := r.topologicalSort(graph)
	if err != nil {
		return nil, err
	}

	// Remove the original skill
	result := make([]string, 0, len(order)-1)
	for _, name := range order {
		if name != skillName {
			result = append(result, name)
		}
	}

	return result, nil
}

// ValidateDependencies checks if all dependencies for a skill exist and are acyclic
func (r *DependencyResolver) ValidateDependencies(skill Skill) error {
	deps := skill.Dependencies()
	if len(deps) == 0 {
		return nil
	}

	// Check all dependencies exist
	for _, dep := range deps {
		if !r.registry.HasSkill(dep) {
			return fmt.Errorf("dependency '%s' not found in registry", dep)
		}
	}

	// Check for self-dependency
	for _, dep := range deps {
		if dep == skill.Name() {
			return fmt.Errorf("skill '%s' cannot depend on itself", skill.Name())
		}
	}

	return nil
}

// GetReverseDependencies returns all skills that depend on the given skill
func (r *DependencyResolver) GetReverseDependencies(skillName string) ([]Skill, error) {
	if !r.registry.HasSkill(skillName) {
		return nil, fmt.Errorf("skill not found: %s", skillName)
	}

	// This requires iterating all skills to find reverse dependencies
	// In a production system, you might want to maintain a reverse index
	var dependents []Skill

	registryWithList, ok := r.registry.(*InMemorySkillRegistry)
	if !ok {
		return nil, fmt.Errorf("registry does not support listing skills")
	}

	for _, info := range registryWithList.List() {
		skill, err := r.registry.Get(info.Name)
		if err != nil {
			continue
		}

		for _, dep := range skill.Dependencies() {
			if dep == skillName {
				dependents = append(dependents, skill)
				break
			}
		}
	}

	return dependents, nil
}
