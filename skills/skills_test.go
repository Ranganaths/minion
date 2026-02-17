package skills

import (
	"context"
	"testing"

	"github.com/Ranganaths/minion/models"
)

func TestNewSkillRegistry(t *testing.T) {
	registry := NewSkillRegistry()
	if registry == nil {
		t.Fatal("expected non-nil registry")
	}
	if registry.Count() != 0 {
		t.Errorf("expected empty registry, got %d skills", registry.Count())
	}
}

func TestMarkdownSkillCreation(t *testing.T) {
	config := MarkdownSkillConfig{
		Name:        "test-skill",
		Description: "A test skill",
		Version:     "1.0.0",
		Content: `## System Instructions

You are a helpful assistant.

## Examples

### Example 1: Greeting

**Input:**
Hello

**Output:**
Hi there!`,
		Source: "test",
		Tags:   []string{"test", "example"},
	}

	skill, err := NewMarkdownSkill(config)
	if err != nil {
		t.Fatalf("failed to create skill: %v", err)
	}

	if skill.Name() != "test-skill" {
		t.Errorf("expected name 'test-skill', got '%s'", skill.Name())
	}
	if skill.Type() != SkillTypeMarkdown {
		t.Errorf("expected type markdown, got '%s'", skill.Type())
	}
	if skill.Scope() != ScopeFramework {
		t.Errorf("expected framework scope, got '%s'", skill.Scope())
	}
}

func TestSkillRegistration(t *testing.T) {
	registry := NewSkillRegistry()

	skill, _ := NewMarkdownSkill(MarkdownSkillConfig{
		Name:        "test-skill",
		Description: "A test skill",
		Source:      "test",
	})

	// Register the skill
	err := registry.Register(skill)
	if err != nil {
		t.Fatalf("failed to register skill: %v", err)
	}

	// Verify registration
	if registry.Count() != 1 {
		t.Errorf("expected 1 skill, got %d", registry.Count())
	}

	// Get the skill
	retrieved, err := registry.Get("test-skill")
	if err != nil {
		t.Fatalf("failed to get skill: %v", err)
	}
	if retrieved.Name() != "test-skill" {
		t.Errorf("expected name 'test-skill', got '%s'", retrieved.Name())
	}

	// Try duplicate registration
	err = registry.Register(skill)
	if err == nil {
		t.Error("expected error on duplicate registration")
	}

	// Unregister
	err = registry.Unregister("test-skill")
	if err != nil {
		t.Fatalf("failed to unregister skill: %v", err)
	}
	if registry.Count() != 0 {
		t.Errorf("expected 0 skills after unregister, got %d", registry.Count())
	}
}

func TestSkillExecution(t *testing.T) {
	skill, _ := NewMarkdownSkill(MarkdownSkillConfig{
		Name:        "test-skill",
		Description: "A test skill",
		Content:     "## Instructions\nBe helpful.",
		Source:      "test",
	})

	ctx := context.Background()
	input := &SkillInput{
		Query: "Hello",
	}

	output, err := skill.Execute(ctx, input)
	if err != nil {
		t.Fatalf("failed to execute skill: %v", err)
	}

	if !output.Success {
		t.Error("expected successful execution")
	}
	if output.Metadata["skill_name"] != "test-skill" {
		t.Errorf("expected skill_name in metadata, got %v", output.Metadata)
	}
}

func TestNativeToolSkillBuilder(t *testing.T) {
	skill, err := NewNativeToolSkillBuilder("weather").
		WithDescription("Get weather information").
		AddStringProperty("location", "City name", true).
		AddEnumProperty("units", "Temperature units", []string{"celsius", "fahrenheit"}, false).
		WithHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			location := args["location"].(string)
			return map[string]interface{}{
				"location":    location,
				"temperature": 22,
				"condition":   "sunny",
			}, nil
		}).
		Build()

	if err != nil {
		t.Fatalf("failed to build skill: %v", err)
	}

	if skill.Name() != "weather" {
		t.Errorf("expected name 'weather', got '%s'", skill.Name())
	}

	// Test execution
	ctx := context.Background()
	output, err := skill.Execute(ctx, &SkillInput{
		Parameters: map[string]interface{}{
			"location": "New York",
		},
	})

	if err != nil {
		t.Fatalf("failed to execute skill: %v", err)
	}
	if !output.Success {
		t.Error("expected successful execution")
	}
}

func TestSkillValidation(t *testing.T) {
	validator := NewSkillValidator()

	tests := []struct {
		name    string
		skill   Skill
		wantErr bool
	}{
		{
			name:    "nil skill",
			skill:   nil,
			wantErr: true,
		},
		{
			name: "valid skill",
			skill: func() Skill {
				s, _ := NewMarkdownSkill(MarkdownSkillConfig{
					Name:        "valid-skill",
					Description: "A valid skill",
					Source:      "test",
				})
				return s
			}(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.skill)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSkillScope(t *testing.T) {
	registry := NewSkillRegistry()

	// Framework-scoped skill
	frameworkSkill, _ := NewMarkdownSkill(MarkdownSkillConfig{
		Name:        "framework-skill",
		Description: "Available to all",
		Scope:       ScopeFramework,
		Source:      "test",
	})

	// Agent-scoped skill
	agentSkill, _ := NewMarkdownSkill(MarkdownSkillConfig{
		Name:          "agent-skill",
		Description:   "Available to specific agent",
		Scope:         ScopeAgent,
		AllowedAgents: []string{"agent-1"},
		Source:        "test",
	})

	registry.Register(frameworkSkill)
	registry.Register(agentSkill)

	// Test with agent-1
	agent1 := &models.Agent{ID: "agent-1"}
	skills1 := registry.GetForAgent(agent1)
	if len(skills1) != 2 {
		t.Errorf("agent-1 should have 2 skills, got %d", len(skills1))
	}

	// Test with agent-2
	agent2 := &models.Agent{ID: "agent-2"}
	skills2 := registry.GetForAgent(agent2)
	if len(skills2) != 1 {
		t.Errorf("agent-2 should have 1 skill (framework only), got %d", len(skills2))
	}
}

func TestFrontmatterParsing(t *testing.T) {
	content := `---
name: test-skill
description: "Test skill description"
version: "1.0.0"
type: markdown
scope: framework
tags:
  - test
  - example
---

# Test Skill

## System Instructions

Be helpful.
`

	parser := NewFrontmatterParser()
	parsed, err := parser.Parse([]byte(content), "test-file.md")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if parsed.Frontmatter.Name != "test-skill" {
		t.Errorf("expected name 'test-skill', got '%s'", parsed.Frontmatter.Name)
	}
	if parsed.Frontmatter.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", parsed.Frontmatter.Version)
	}
	if len(parsed.Frontmatter.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(parsed.Frontmatter.Tags))
	}
}

func TestDependencyResolution(t *testing.T) {
	registry := NewSkillRegistry()

	// Create skills with dependencies
	baseSkill, _ := NewMarkdownSkill(MarkdownSkillConfig{
		Name:        "base-skill",
		Description: "Base skill with no dependencies",
		Source:      "test",
	})

	dependentSkill, _ := NewMarkdownSkill(MarkdownSkillConfig{
		Name:         "dependent-skill",
		Description:  "Skill that depends on base",
		Dependencies: []string{"base-skill"},
		Source:       "test",
	})

	registry.Register(baseSkill)
	registry.Register(dependentSkill)

	// Resolve dependencies
	deps, err := registry.GetDependencies("dependent-skill")
	if err != nil {
		t.Fatalf("failed to resolve dependencies: %v", err)
	}

	if len(deps) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(deps))
	}
	if deps[0].Name() != "base-skill" {
		t.Errorf("expected 'base-skill', got '%s'", deps[0].Name())
	}
}

func TestMarkdownParser(t *testing.T) {
	content := `---
name: parsed-skill
description: "Parsed from markdown"
version: "2.0.0"
type: markdown
---

## System Instructions

Follow these rules.

## Examples

### Example 1: Basic

**Input:**
test input

**Output:**
test output
`

	parser := NewMarkdownParser()
	skill, err := parser.Parse([]byte(content), "parsed.md")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	mdSkill, ok := skill.(*MarkdownSkill)
	if !ok {
		t.Fatal("expected MarkdownSkill type")
	}

	if mdSkill.SystemPrompt() == "" {
		t.Error("expected non-empty system prompt")
	}

	examples := mdSkill.Examples()
	if len(examples) == 0 {
		t.Error("expected at least one example")
	}
}

func TestSkillToolWrapper(t *testing.T) {
	skill, _ := NewMarkdownSkill(MarkdownSkillConfig{
		Name:        "wrapped-skill",
		Description: "A skill to wrap",
		Source:      "test",
	})

	tool := WrapSkillAsTool(skill)

	if tool.Name() != "skill_wrapped-skill" {
		t.Errorf("expected 'skill_wrapped-skill', got '%s'", tool.Name())
	}
	if tool.Description() != "[Skill] A skill to wrap" {
		t.Errorf("unexpected description: %s", tool.Description())
	}
}
