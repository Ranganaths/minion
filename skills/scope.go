package skills

import (
	"github.com/Ranganaths/minion/models"
)

// ScopeManager manages skill scopes and agent access
type ScopeManager struct {
	registry Registry
}

// NewScopeManager creates a new scope manager
func NewScopeManager(registry Registry) *ScopeManager {
	return &ScopeManager{
		registry: registry,
	}
}

// GetFrameworkSkills returns all framework-level skills
func (m *ScopeManager) GetFrameworkSkills() []Skill {
	return m.registry.GetForScope(ScopeFramework)
}

// GetAgentSkills returns all agent-level skills
func (m *ScopeManager) GetAgentSkills() []Skill {
	return m.registry.GetForScope(ScopeAgent)
}

// GetSkillsForAgent returns all skills available to a specific agent
func (m *ScopeManager) GetSkillsForAgent(agent *models.Agent) []Skill {
	return m.registry.GetForAgent(agent)
}

// CanAgentExecuteSkill checks if an agent can execute a specific skill
func (m *ScopeManager) CanAgentExecuteSkill(agent *models.Agent, skillName string) bool {
	skill, err := m.registry.Get(skillName)
	if err != nil {
		return false
	}
	return skill.CanExecute(agent)
}

// GetSkillsByAgentID returns all skills assigned to a specific agent ID
func (m *ScopeManager) GetSkillsByAgentID(agentID string) []Skill {
	// Create a minimal agent just for the lookup
	agent := &models.Agent{ID: agentID}
	return m.registry.GetForAgent(agent)
}

// AssignSkillToAgent assigns a skill to a specific agent
// This only works for agent-scoped skills
func (m *ScopeManager) AssignSkillToAgent(skillName, agentID string) error {
	skill, err := m.registry.Get(skillName)
	if err != nil {
		return err
	}

	// Only modify agent-scoped skills
	if skill.Scope() != ScopeAgent {
		return ErrInvalidSkill
	}

	// Get the underlying modifiable skill
	if base, ok := getBaseSkill(skill); ok {
		agents := base.AllowedAgents()
		// Check if already assigned
		for _, id := range agents {
			if id == agentID {
				return nil // Already assigned
			}
		}
		base.SetAllowedAgents(append(agents, agentID))
	}

	return nil
}

// RevokeSkillFromAgent removes a skill assignment from an agent
func (m *ScopeManager) RevokeSkillFromAgent(skillName, agentID string) error {
	skill, err := m.registry.Get(skillName)
	if err != nil {
		return err
	}

	// Only modify agent-scoped skills
	if skill.Scope() != ScopeAgent {
		return ErrInvalidSkill
	}

	// Get the underlying modifiable skill
	if base, ok := getBaseSkill(skill); ok {
		agents := base.AllowedAgents()
		newAgents := make([]string, 0, len(agents))
		for _, id := range agents {
			if id != agentID {
				newAgents = append(newAgents, id)
			}
		}
		base.SetAllowedAgents(newAgents)
	}

	return nil
}

// getBaseSkill extracts the BaseSkill from a skill if possible
func getBaseSkill(skill Skill) (*BaseSkill, bool) {
	switch s := skill.(type) {
	case *MarkdownSkill:
		return s.BaseSkill, true
	case *NativeToolSkill:
		return s.BaseSkill, true
	default:
		return nil, false
	}
}

// ScopeFilter filters skills by scope criteria
type ScopeFilter struct {
	IncludeFramework bool
	IncludeAgent     bool
	AgentIDs         []string
	Tags             []string
	Types            []SkillType
}

// NewScopeFilter creates a new scope filter
func NewScopeFilter() *ScopeFilter {
	return &ScopeFilter{
		IncludeFramework: true,
		IncludeAgent:     true,
	}
}

// WithFramework sets whether to include framework-scoped skills
func (f *ScopeFilter) WithFramework(include bool) *ScopeFilter {
	f.IncludeFramework = include
	return f
}

// WithAgent sets whether to include agent-scoped skills
func (f *ScopeFilter) WithAgent(include bool) *ScopeFilter {
	f.IncludeAgent = include
	return f
}

// WithAgentIDs sets specific agent IDs to filter for
func (f *ScopeFilter) WithAgentIDs(ids ...string) *ScopeFilter {
	f.AgentIDs = ids
	return f
}

// WithTags sets tags to filter for
func (f *ScopeFilter) WithTags(tags ...string) *ScopeFilter {
	f.Tags = tags
	return f
}

// WithTypes sets types to filter for
func (f *ScopeFilter) WithTypes(types ...SkillType) *ScopeFilter {
	f.Types = types
	return f
}

// Filter filters a list of skills based on the filter criteria
func (f *ScopeFilter) Filter(skills []Skill) []Skill {
	var result []Skill

	for _, skill := range skills {
		if f.matches(skill) {
			result = append(result, skill)
		}
	}

	return result
}

// matches checks if a skill matches the filter criteria
func (f *ScopeFilter) matches(skill Skill) bool {
	// Check scope
	switch skill.Scope() {
	case ScopeFramework:
		if !f.IncludeFramework {
			return false
		}
	case ScopeAgent:
		if !f.IncludeAgent {
			return false
		}
		// Check agent IDs if specified
		if len(f.AgentIDs) > 0 {
			if !f.matchesAgentIDs(skill.AllowedAgents()) {
				return false
			}
		}
	}

	// Check types if specified
	if len(f.Types) > 0 {
		if !f.matchesType(skill.Type()) {
			return false
		}
	}

	// Check tags if specified
	if len(f.Tags) > 0 {
		if !f.matchesTags(skill.Metadata().Tags) {
			return false
		}
	}

	return true
}

// matchesAgentIDs checks if any of the skill's agent IDs match the filter
func (f *ScopeFilter) matchesAgentIDs(skillAgentIDs []string) bool {
	for _, filterID := range f.AgentIDs {
		for _, skillID := range skillAgentIDs {
			if filterID == skillID {
				return true
			}
		}
	}
	return false
}

// matchesType checks if the skill type matches any of the filter types
func (f *ScopeFilter) matchesType(skillType SkillType) bool {
	for _, t := range f.Types {
		if t == skillType {
			return true
		}
	}
	return false
}

// matchesTags checks if any of the skill's tags match the filter tags
func (f *ScopeFilter) matchesTags(skillTags []string) bool {
	for _, filterTag := range f.Tags {
		for _, skillTag := range skillTags {
			if filterTag == skillTag {
				return true
			}
		}
	}
	return false
}

// ScopeStats provides statistics about skill scopes
type ScopeStats struct {
	TotalSkills      int
	FrameworkScoped  int
	AgentScoped      int
	MarkdownSkills   int
	NativeToolSkills int
	BehaviorSkills   int
	UniqueAgents     int
}

// GetScopeStats returns statistics about skill scopes in the registry
func (m *ScopeManager) GetScopeStats() *ScopeStats {
	skills := m.registry.List()

	stats := &ScopeStats{
		TotalSkills: len(skills),
	}

	agentSet := make(map[string]bool)

	for _, info := range skills {
		// Count by scope
		switch info.Scope {
		case ScopeFramework:
			stats.FrameworkScoped++
		case ScopeAgent:
			stats.AgentScoped++
		}

		// Count by type
		switch info.Type {
		case SkillTypeMarkdown:
			stats.MarkdownSkills++
		case SkillTypeNativeTool:
			stats.NativeToolSkills++
		case SkillTypeBehavior:
			stats.BehaviorSkills++
		}
	}

	// Count unique agents (need to get full skills for this)
	for _, info := range skills {
		skill, err := m.registry.Get(info.Name)
		if err != nil {
			continue
		}
		for _, agentID := range skill.AllowedAgents() {
			agentSet[agentID] = true
		}
	}

	stats.UniqueAgents = len(agentSet)

	return stats
}
