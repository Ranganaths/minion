package skills

import (
	"context"
	"fmt"

	"github.com/Ranganaths/minion/models"
	"github.com/Ranganaths/minion/tools"
)

// SkillToolWrapper wraps a Skill as a tools.Tool for compatibility with the existing tool system
type SkillToolWrapper struct {
	skill Skill
}

// Ensure SkillToolWrapper implements tools.Tool
var _ tools.Tool = (*SkillToolWrapper)(nil)

// WrapSkillAsTool wraps a Skill as a tools.Tool
func WrapSkillAsTool(skill Skill) tools.Tool {
	return &SkillToolWrapper{skill: skill}
}

// Name returns the tool name (prefixed with "skill_")
func (w *SkillToolWrapper) Name() string {
	return "skill_" + w.skill.Name()
}

// Description returns the tool description
func (w *SkillToolWrapper) Description() string {
	return fmt.Sprintf("[Skill] %s", w.skill.Description())
}

// Execute runs the skill through the tool interface
func (w *SkillToolWrapper) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	// Convert ToolInput to SkillInput
	skillInput := &SkillInput{
		Parameters: input.Params,
		Context:    input.Context,
	}

	// Extract query from params if present
	if query, ok := input.Params["query"].(string); ok {
		skillInput.Query = query
	}

	// Extract data as query if present and query wasn't set
	if skillInput.Query == "" {
		if data, ok := input.Data.(string); ok {
			skillInput.Query = data
		}
	}

	// Execute the skill
	output, err := w.skill.Execute(ctx, skillInput)
	if err != nil {
		return &models.ToolOutput{
			ToolName: w.Name(),
			Success:  false,
			Error:    err.Error(),
		}, nil
	}

	// Convert SkillOutput to ToolOutput
	return &models.ToolOutput{
		ToolName:      w.Name(),
		Success:       output.Success,
		Result:        output.Content,
		Metadata:      output.Metadata,
		ExecutionTime: output.ExecutionTime,
		Error:         output.Error,
	}, nil
}

// CanExecute checks if the skill can be executed for the given agent
func (w *SkillToolWrapper) CanExecute(agent *models.Agent) bool {
	return w.skill.CanExecute(agent)
}

// Skill returns the underlying skill
func (w *SkillToolWrapper) Skill() Skill {
	return w.skill
}

// ToolSkillWrapper wraps a tools.Tool as a Skill for reverse compatibility
type ToolSkillWrapper struct {
	*BaseSkill
	tool tools.Tool
}

// WrapToolAsSkill wraps a tools.Tool as a Skill
func WrapToolAsSkill(tool tools.Tool) Skill {
	base := NewBaseSkill(tool.Name(), tool.Description(), SkillTypeNativeTool)
	base.SetMetadata(SkillMetadata{
		Source: "tool_wrapper",
	})

	return &ToolSkillWrapper{
		BaseSkill: base,
		tool:      tool,
	}
}

// Execute runs the tool through the skill interface
func (w *ToolSkillWrapper) Execute(ctx context.Context, input *SkillInput) (*SkillOutput, error) {
	// Convert SkillInput to ToolInput
	toolInput := &models.ToolInput{
		Data:    input.Query,
		Params:  input.Parameters,
		Context: input.Context,
	}

	// Execute the tool
	output, err := w.tool.Execute(ctx, toolInput)
	if err != nil {
		return &SkillOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Convert result to string content
	var content string
	if s, ok := output.Result.(string); ok {
		content = s
	} else {
		content = fmt.Sprintf("%v", output.Result)
	}

	return &SkillOutput{
		Content:       content,
		Success:       output.Success,
		Metadata:      output.Metadata,
		ExecutionTime: output.ExecutionTime,
		Error:         output.Error,
	}, nil
}

// CanExecute checks if the tool can be executed for the given agent
func (w *ToolSkillWrapper) CanExecute(agent *models.Agent) bool {
	return w.tool.CanExecute(agent)
}

// Tool returns the underlying tool
func (w *ToolSkillWrapper) Tool() tools.Tool {
	return w.tool
}

// SkillsToTools converts a slice of skills to tools
func SkillsToTools(skills []Skill) []tools.Tool {
	result := make([]tools.Tool, len(skills))
	for i, skill := range skills {
		result[i] = WrapSkillAsTool(skill)
	}
	return result
}

// ToolsToSkills converts a slice of tools to skills
func ToolsToSkills(toolList []tools.Tool) []Skill {
	result := make([]Skill, len(toolList))
	for i, tool := range toolList {
		result[i] = WrapToolAsSkill(tool)
	}
	return result
}

// RegisterSkillsAsTools registers skills as tools in a tool registry
func RegisterSkillsAsTools(skillRegistry Registry, toolRegistry tools.Registry, agent *models.Agent) error {
	skills := skillRegistry.GetForAgent(agent)
	for _, skill := range skills {
		tool := WrapSkillAsTool(skill)
		if err := toolRegistry.Register(tool); err != nil {
			return fmt.Errorf("failed to register skill %s as tool: %w", skill.Name(), err)
		}
	}
	return nil
}

// RegisterToolsAsSkills registers tools as skills in a skill registry
func RegisterToolsAsSkills(toolRegistry tools.Registry, skillRegistry Registry) error {
	toolNames := toolRegistry.List()
	for _, name := range toolNames {
		tool, err := toolRegistry.Get(name)
		if err != nil {
			continue
		}
		skill := WrapToolAsSkill(tool)
		if err := skillRegistry.Register(skill); err != nil {
			return fmt.Errorf("failed to register tool %s as skill: %w", name, err)
		}
	}
	return nil
}

// SkillToolAdapter adapts between skill and tool execution
type SkillToolAdapter struct {
	skillRegistry Registry
	toolRegistry  tools.Registry
}

// NewSkillToolAdapter creates a new skill-tool adapter
func NewSkillToolAdapter(skillRegistry Registry, toolRegistry tools.Registry) *SkillToolAdapter {
	return &SkillToolAdapter{
		skillRegistry: skillRegistry,
		toolRegistry:  toolRegistry,
	}
}

// GetAvailableTools returns all available tools (both native tools and wrapped skills)
func (a *SkillToolAdapter) GetAvailableTools(agent *models.Agent) []tools.Tool {
	var result []tools.Tool

	// Get native tools
	nativeTools := a.toolRegistry.GetToolsForAgent(agent)
	result = append(result, nativeTools...)

	// Get skills as tools
	skills := a.skillRegistry.GetForAgent(agent)
	for _, skill := range skills {
		result = append(result, WrapSkillAsTool(skill))
	}

	return result
}

// ExecuteByName executes a tool or skill by name
func (a *SkillToolAdapter) ExecuteByName(ctx context.Context, name string, input *models.ToolInput) (*models.ToolOutput, error) {
	// Try tool registry first
	tool, err := a.toolRegistry.Get(name)
	if err == nil {
		return tool.Execute(ctx, input)
	}

	// Try skill registry (with "skill_" prefix stripped if present)
	skillName := name
	if len(name) > 6 && name[:6] == "skill_" {
		skillName = name[6:]
	}

	skill, err := a.skillRegistry.Get(skillName)
	if err != nil {
		return nil, fmt.Errorf("tool or skill not found: %s", name)
	}

	// Execute skill through wrapper
	wrapper := WrapSkillAsTool(skill)
	return wrapper.Execute(ctx, input)
}
