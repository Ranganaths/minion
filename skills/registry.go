package skills

import (
	"embed"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/Ranganaths/minion/models"
)

// Common errors for the skill registry
var (
	ErrSkillNotFound      = errors.New("skill not found")
	ErrSkillAlreadyExists = errors.New("skill already exists")
	ErrInvalidSkill       = errors.New("invalid skill")
	ErrCircularDependency = errors.New("circular dependency detected")
)

// InMemorySkillRegistry is an in-memory implementation of the Registry interface
type InMemorySkillRegistry struct {
	skills      map[string]Skill
	byType      map[SkillType][]string
	byScope     map[SkillScope][]string
	agentSkills map[string][]string // AgentID -> skill names
	mu          sync.RWMutex
	validator   *SkillValidator
	loader      *SkillLoader
	watcher     *SkillWatcher
	resolver    *DependencyResolver
}

// NewSkillRegistry creates a new in-memory skill registry
func NewSkillRegistry() *InMemorySkillRegistry {
	r := &InMemorySkillRegistry{
		skills:      make(map[string]Skill),
		byType:      make(map[SkillType][]string),
		byScope:     make(map[SkillScope][]string),
		agentSkills: make(map[string][]string),
	}
	r.validator = NewSkillValidator()
	r.loader = NewSkillLoader(r)
	r.resolver = NewDependencyResolver(r)
	return r
}

// Register adds a skill to the registry
func (r *InMemorySkillRegistry) Register(skill Skill) error {
	if skill == nil {
		return ErrInvalidSkill
	}

	// Validate the skill
	if err := r.validator.Validate(skill); err != nil {
		return fmt.Errorf("skill validation failed: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	name := skill.Name()
	if _, exists := r.skills[name]; exists {
		return fmt.Errorf("%w: %s", ErrSkillAlreadyExists, name)
	}

	// Register in main map
	r.skills[name] = skill

	// Index by type
	r.byType[skill.Type()] = append(r.byType[skill.Type()], name)

	// Index by scope
	r.byScope[skill.Scope()] = append(r.byScope[skill.Scope()], name)

	// Index by allowed agents (for agent-scoped skills)
	if skill.Scope() == ScopeAgent {
		for _, agentID := range skill.AllowedAgents() {
			r.agentSkills[agentID] = append(r.agentSkills[agentID], name)
		}
	}

	return nil
}

// RegisterFromFile loads and registers a skill from a file path
func (r *InMemorySkillRegistry) RegisterFromFile(path string) error {
	return r.loader.LoadFromFile(path)
}

// RegisterFromURL loads and registers a skill from a URL
func (r *InMemorySkillRegistry) RegisterFromURL(url string) error {
	return r.loader.LoadFromURL(url)
}

// RegisterFromEmbed loads and registers skills from an embedded filesystem
func (r *InMemorySkillRegistry) RegisterFromEmbed(fs embed.FS, pattern string) (int, error) {
	return r.loader.LoadFromEmbed(fs, pattern)
}

// Unregister removes a skill from the registry
func (r *InMemorySkillRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	skill, exists := r.skills[name]
	if !exists {
		return fmt.Errorf("%w: %s", ErrSkillNotFound, name)
	}

	// Remove from main map
	delete(r.skills, name)

	// Remove from type index
	r.byType[skill.Type()] = removeFromSlice(r.byType[skill.Type()], name)

	// Remove from scope index
	r.byScope[skill.Scope()] = removeFromSlice(r.byScope[skill.Scope()], name)

	// Remove from agent index
	if skill.Scope() == ScopeAgent {
		for _, agentID := range skill.AllowedAgents() {
			r.agentSkills[agentID] = removeFromSlice(r.agentSkills[agentID], name)
		}
	}

	return nil
}

// Get retrieves a skill by name
func (r *InMemorySkillRegistry) Get(name string) (Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, exists := r.skills[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrSkillNotFound, name)
	}

	return skill, nil
}

// GetByType returns all skills of a specific type
func (r *InMemorySkillRegistry) GetByType(skillType SkillType) []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := r.byType[skillType]
	skills := make([]Skill, 0, len(names))
	for _, name := range names {
		if skill, exists := r.skills[name]; exists {
			skills = append(skills, skill)
		}
	}

	return skills
}

// GetForAgent returns all skills available to an agent
func (r *InMemorySkillRegistry) GetForAgent(agent *models.Agent) []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Skill

	// Add all framework-level skills
	for _, name := range r.byScope[ScopeFramework] {
		if skill, exists := r.skills[name]; exists {
			if skill.CanExecute(agent) {
				result = append(result, skill)
			}
		}
	}

	// Add agent-specific skills
	if agent != nil {
		for _, name := range r.agentSkills[agent.ID] {
			if skill, exists := r.skills[name]; exists {
				if skill.CanExecute(agent) {
					result = append(result, skill)
				}
			}
		}
	}

	return result
}

// GetForScope returns all skills with a specific scope
func (r *InMemorySkillRegistry) GetForScope(scope SkillScope) []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := r.byScope[scope]
	skills := make([]Skill, 0, len(names))
	for _, name := range names {
		if skill, exists := r.skills[name]; exists {
			skills = append(skills, skill)
		}
	}

	return skills
}

// List returns info about all registered skills
func (r *InMemorySkillRegistry) List() []SkillInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]SkillInfo, 0, len(r.skills))
	for _, skill := range r.skills {
		infos = append(infos, SkillInfo{
			Name:         skill.Name(),
			Description:  skill.Description(),
			Type:         skill.Type(),
			Scope:        skill.Scope(),
			Version:      skill.Version(),
			Source:       skill.Metadata().Source,
			Dependencies: skill.Dependencies(),
			Tags:         skill.Metadata().Tags,
		})
	}

	return infos
}

// Count returns the number of registered skills
func (r *InMemorySkillRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.skills)
}

// LoadDirectory loads all skills from a directory
func (r *InMemorySkillRegistry) LoadDirectory(path string) (int, error) {
	return r.loader.LoadDirectory(path)
}

// WatchDirectory starts watching a directory for skill changes (hot-reload)
func (r *InMemorySkillRegistry) WatchDirectory(path string) error {
	if r.watcher == nil {
		var err error
		r.watcher, err = NewSkillWatcher(r, r.loader)
		if err != nil {
			return fmt.Errorf("failed to create skill watcher: %w", err)
		}
	}

	return r.watcher.Watch(path)
}

// StopWatching stops all directory watchers
func (r *InMemorySkillRegistry) StopWatching() error {
	if r.watcher != nil {
		return r.watcher.Stop()
	}
	return nil
}

// Validate validates a skill without registering it
func (r *InMemorySkillRegistry) Validate(skill Skill) error {
	return r.validator.Validate(skill)
}

// GetDependencies returns all dependencies for a skill (resolved recursively)
func (r *InMemorySkillRegistry) GetDependencies(name string) ([]Skill, error) {
	return r.resolver.Resolve(name)
}

// UpdateSkill updates an existing skill (for hot-reload)
func (r *InMemorySkillRegistry) UpdateSkill(skill Skill) error {
	if skill == nil {
		return ErrInvalidSkill
	}

	// Validate the skill
	if err := r.validator.Validate(skill); err != nil {
		return fmt.Errorf("skill validation failed: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	name := skill.Name()
	oldSkill, exists := r.skills[name]

	// Update in main map
	r.skills[name] = skill

	// If this is a new skill or type changed, update type index
	if !exists || oldSkill.Type() != skill.Type() {
		if exists {
			r.byType[oldSkill.Type()] = removeFromSlice(r.byType[oldSkill.Type()], name)
		}
		r.byType[skill.Type()] = append(r.byType[skill.Type()], name)
	}

	// If this is a new skill or scope changed, update scope index
	if !exists || oldSkill.Scope() != skill.Scope() {
		if exists {
			r.byScope[oldSkill.Scope()] = removeFromSlice(r.byScope[oldSkill.Scope()], name)
		}
		r.byScope[skill.Scope()] = append(r.byScope[skill.Scope()], name)
	}

	// Update agent index for agent-scoped skills
	if exists && oldSkill.Scope() == ScopeAgent {
		for _, agentID := range oldSkill.AllowedAgents() {
			r.agentSkills[agentID] = removeFromSlice(r.agentSkills[agentID], name)
		}
	}
	if skill.Scope() == ScopeAgent {
		for _, agentID := range skill.AllowedAgents() {
			r.agentSkills[agentID] = append(r.agentSkills[agentID], name)
		}
	}

	return nil
}

// GetSkillsByTag returns all skills with a specific tag
func (r *InMemorySkillRegistry) GetSkillsByTag(tag string) []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Skill
	for _, skill := range r.skills {
		for _, t := range skill.Metadata().Tags {
			if t == tag {
				result = append(result, skill)
				break
			}
		}
	}

	return result
}

// Clear removes all skills from the registry
func (r *InMemorySkillRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.skills = make(map[string]Skill)
	r.byType = make(map[SkillType][]string)
	r.byScope = make(map[SkillScope][]string)
	r.agentSkills = make(map[string][]string)
}

// GetNames returns the names of all registered skills
func (r *InMemorySkillRegistry) GetNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}

	return names
}

// HasSkill checks if a skill with the given name exists
func (r *InMemorySkillRegistry) HasSkill(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.skills[name]
	return exists
}

// GetSkillNames returns all skill names (for path tracking in loader)
func (r *InMemorySkillRegistry) GetSkillNames() []string {
	return r.GetNames()
}

// removeFromSlice removes an element from a slice
func removeFromSlice(slice []string, element string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != element {
			result = append(result, s)
		}
	}
	return result
}

// SkillPath tracks which skill was loaded from which path (for hot-reload)
type SkillPath struct {
	Name string
	Path string
}

// GetLoader returns the skill loader (for testing and advanced usage)
func (r *InMemorySkillRegistry) GetLoader() *SkillLoader {
	return r.loader
}

// GetValidator returns the skill validator (for testing and advanced usage)
func (r *InMemorySkillRegistry) GetValidator() *SkillValidator {
	return r.validator
}

// GetResolver returns the dependency resolver (for testing and advanced usage)
func (r *InMemorySkillRegistry) GetResolver() *DependencyResolver {
	return r.resolver
}

// GetWatcher returns the skill watcher (for testing and advanced usage)
func (r *InMemorySkillRegistry) GetWatcher() *SkillWatcher {
	return r.watcher
}

// FindSkillByPath finds a skill by its source path
func (r *InMemorySkillRegistry) FindSkillByPath(path string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	absPath, _ := filepath.Abs(path)

	for _, skill := range r.skills {
		sourcePath, _ := filepath.Abs(skill.Metadata().Source)
		if sourcePath == absPath {
			return skill, true
		}
	}

	return nil, false
}
