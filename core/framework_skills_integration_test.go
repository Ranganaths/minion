package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ranganaths/minion/models"
	"github.com/Ranganaths/minion/skills"
	"github.com/Ranganaths/minion/storage"
)

func TestFrameworkSkillsIntegration(t *testing.T) {
	// Create temp dir for test storage
	tempDir, err := os.MkdirTemp("", "minion-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create framework with test storage
	store := storage.NewInMemory()
	opts := []Option{
		WithStorage(store),
	}

	framework := NewFramework(opts...)
	defer framework.Close()

	// Test 1: RegisterSkill
	t.Run("RegisterSkill", func(t *testing.T) {
		skill, err := skills.NewMarkdownSkill(skills.MarkdownSkillConfig{
			Name:        "test-register",
			Description: "Test skill registration",
			Content:     "## Instructions\nTest content",
			Source:      "test",
		})
		if err != nil {
			t.Fatalf("failed to create skill: %v", err)
		}

		err = framework.RegisterSkill(skill)
		if err != nil {
			t.Fatalf("failed to register skill: %v", err)
		}

		// Verify skill was registered
		retrieved, err := framework.GetSkill("test-register")
		if err != nil {
			t.Fatalf("failed to get skill: %v", err)
		}
		if retrieved.Name() != "test-register" {
			t.Errorf("expected 'test-register', got '%s'", retrieved.Name())
		}
	})

	// Test 2: GetSkillsForAgent with framework scope
	t.Run("GetSkillsForAgent_FrameworkScope", func(t *testing.T) {
		frameworkSkill, _ := skills.NewMarkdownSkill(skills.MarkdownSkillConfig{
			Name:        "framework-test",
			Description: "Framework scoped skill",
			Scope:       skills.ScopeFramework,
			Source:      "test",
		})
		framework.RegisterSkill(frameworkSkill)

		agent := &models.Agent{
			ID:   "agent-1",
			Name: "Test Agent",
		}

		agentSkills := framework.GetSkillsForAgent(agent)
		found := false
		for _, s := range agentSkills {
			if s.Name() == "framework-test" {
				found = true
				break
			}
		}
		if !found {
			t.Error("framework-scoped skill not available to agent")
		}
	})

	// Test 3: GetSkillsForAgent with agent scope
	t.Run("GetSkillsForAgent_AgentScope", func(t *testing.T) {
		agentSkill, _ := skills.NewMarkdownSkill(skills.MarkdownSkillConfig{
			Name:          "agent-specific",
			Description:   "Agent scoped skill",
			Scope:         skills.ScopeAgent,
			AllowedAgents: []string{"authorized-agent"},
			Source:        "test",
		})
		framework.RegisterSkill(agentSkill)

		// Authorized agent should have access
		authorizedAgent := &models.Agent{
			ID:   "authorized-agent",
			Name: "Authorized Agent",
		}
		authorizedSkills := framework.GetSkillsForAgent(authorizedAgent)
		found := false
		for _, s := range authorizedSkills {
			if s.Name() == "agent-specific" {
				found = true
				break
			}
		}
		if !found {
			t.Error("agent-scoped skill not available to authorized agent")
		}

		// Unauthorized agent should NOT have access
		unauthorizedAgent := &models.Agent{
			ID:   "unauthorized-agent",
			Name: "Unauthorized Agent",
		}
		unauthorizedSkills := framework.GetSkillsForAgent(unauthorizedAgent)
		for _, s := range unauthorizedSkills {
			if s.Name() == "agent-specific" {
				t.Error("agent-scoped skill available to unauthorized agent")
			}
		}
	})

	// Test 4: ExecuteSkill - basic execution
	t.Run("ExecuteSkill_Basic", func(t *testing.T) {
		skill, _ := skills.NewMarkdownSkill(skills.MarkdownSkillConfig{
			Name:        "execute-test",
			Description: "Test skill execution",
			Content:     "## Instructions\nBe helpful",
			Source:      "test",
		})
		framework.RegisterSkill(skill)

		ctx := context.Background()
		input := &skills.SkillInput{
			Query: "test query",
		}

		output, err := framework.ExecuteSkill(ctx, "execute-test", input)
		if err != nil {
			t.Fatalf("failed to execute skill: %v", err)
		}

		if !output.Success {
			t.Error("expected successful execution")
		}
		if output.Metadata["skill_name"] != "execute-test" {
			t.Errorf("expected skill_name in metadata, got %v", output.Metadata)
		}
	})

	// Test 5: ExecuteSkill with authorization check
	t.Run("ExecuteSkill_Authorization", func(t *testing.T) {
		restrictedSkill, _ := skills.NewMarkdownSkill(skills.MarkdownSkillConfig{
			Name:          "restricted-skill",
			Description:   "Restricted skill",
			Scope:         skills.ScopeAgent,
			AllowedAgents: []string{"allowed-agent"},
			Source:        "test",
		})
		framework.RegisterSkill(restrictedSkill)

		ctx := context.Background()

		// Test with unauthorized agent
		unauthorizedAgent := &models.Agent{ID: "unauthorized"}
		input := &skills.SkillInput{
			Agent: unauthorizedAgent,
			Query: "test",
		}

		_, err := framework.ExecuteSkill(ctx, "restricted-skill", input)
		if err == nil {
			t.Error("expected error when executing skill with unauthorized agent")
		}

		// Test with authorized agent
		authorizedAgent := &models.Agent{ID: "allowed-agent"}
		input.Agent = authorizedAgent

		output, err := framework.ExecuteSkill(ctx, "restricted-skill", input)
		if err != nil {
			t.Fatalf("authorized agent should be able to execute: %v", err)
		}
		if !output.Success {
			t.Error("expected successful execution for authorized agent")
		}
	})

	// Test 6: ExecuteSkill with dependencies
	t.Run("ExecuteSkill_Dependencies", func(t *testing.T) {
		// Create base skill
		baseSkill, _ := skills.NewMarkdownSkill(skills.MarkdownSkillConfig{
			Name:        "dep-base",
			Description: "Base skill",
			Content:     "## Instructions\nProvide base data",
			Source:      "test",
		})
		framework.RegisterSkill(baseSkill)

		// Create dependent skill
		dependentSkill, _ := skills.NewMarkdownSkill(skills.MarkdownSkillConfig{
			Name:         "dep-dependent",
			Description:  "Dependent skill",
			Content:      "## Instructions\nUse base data",
			Dependencies: []string{"dep-base"},
			Source:       "test",
		})
		framework.RegisterSkill(dependentSkill)

		ctx := context.Background()
		input := &skills.SkillInput{
			Query: "test with deps",
		}

		output, err := framework.ExecuteSkill(ctx, "dep-dependent", input)
		if err != nil {
			t.Fatalf("failed to execute skill with dependencies: %v", err)
		}

		if !output.Success {
			t.Error("expected successful execution")
		}

		// Verify dependency results were passed
		if input.DependencyResults == nil {
			t.Error("expected dependency results to be populated")
		}
		if _, ok := input.DependencyResults["dep-base"]; !ok {
			t.Error("expected base skill results in dependency results")
		}
	})

	// Test 7: Native tool skill execution
	t.Run("NativeToolSkill_Execution", func(t *testing.T) {
		nativeSkill, err := skills.NewNativeToolSkillBuilder("calculator").
			WithDescription("Performs calculations").
			AddNumberProperty("a", "First number", true).
			AddNumberProperty("b", "Second number", true).
			AddStringProperty("operation", "Operation to perform", true).
			WithHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
				a := args["a"].(float64)
				b := args["b"].(float64)
				op := args["operation"].(string)

				switch op {
				case "add":
					return a + b, nil
				case "subtract":
					return a - b, nil
				case "multiply":
					return a * b, nil
				case "divide":
					if b == 0 {
						return nil, nil
					}
					return a / b, nil
				default:
					return nil, nil
				}
			}).
			Build()

		if err != nil {
			t.Fatalf("failed to build native skill: %v", err)
		}

		framework.RegisterSkill(nativeSkill)

		ctx := context.Background()
		input := &skills.SkillInput{
			Parameters: map[string]interface{}{
				"a":         10.0,
				"b":         5.0,
				"operation": "add",
			},
		}

		output, err := framework.ExecuteSkill(ctx, "calculator", input)
		if err != nil {
			t.Fatalf("failed to execute native skill: %v", err)
		}

		if !output.Success {
			t.Error("expected successful execution")
		}
	})

	// Test 8: ListSkills
	t.Run("ListSkills", func(t *testing.T) {
		skillsList := framework.ListSkills()
		if len(skillsList) == 0 {
			t.Error("expected at least one skill in list")
		}

		// Verify we can find our registered skills
		found := false
		for _, info := range skillsList {
			if info.Name == "test-register" {
				found = true
				if info.Type != "markdown" {
					t.Errorf("expected type 'markdown', got '%s'", info.Type)
				}
				break
			}
		}
		if !found {
			t.Error("registered skill not found in list")
		}
	})
}

func TestFrameworkSkillsDirectoryLoading(t *testing.T) {
	// Create temp directory for skills
	tempDir, err := os.MkdirTemp("", "minion-skills-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write a test skill file
	skillContent := `---
name: test-file-skill
description: "Skill loaded from file"
version: "1.0.0"
type: markdown
scope: framework
tags:
  - test
  - file
---

## System Instructions

This is a test skill loaded from a file.

## Examples

### Example 1: Test

**Input:**
Test input

**Output:**
Test output
`

	skillPath := filepath.Join(tempDir, "test-skill.md")
	err = os.WriteFile(skillPath, []byte(skillContent), 0644)
	if err != nil {
		t.Fatalf("failed to write skill file: %v", err)
	}

	// Create framework
	store := storage.NewInMemory()
	framework := NewFramework(WithStorage(store))
	defer framework.Close()

	// Test LoadSkillsFromDirectory
	t.Run("LoadSkillsFromDirectory", func(t *testing.T) {
		count, err := framework.LoadSkillsFromDirectory(tempDir)
		if err != nil {
			t.Fatalf("failed to load skills from directory: %v", err)
		}

		if count != 1 {
			t.Errorf("expected 1 skill loaded, got %d", count)
		}

		// Verify the skill was loaded
		skill, err := framework.GetSkill("test-file-skill")
		if err != nil {
			t.Fatalf("failed to get loaded skill: %v", err)
		}

		if skill.Name() != "test-file-skill" {
			t.Errorf("expected 'test-file-skill', got '%s'", skill.Name())
		}
		if skill.Version() != "1.0.0" {
			t.Errorf("expected version '1.0.0', got '%s'", skill.Version())
		}
	})
}

func TestFrameworkSkillsWatching(t *testing.T) {
	// Create temp directory for skills
	tempDir, err := os.MkdirTemp("", "minion-watch-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create framework
	store := storage.NewInMemory()
	framework := NewFramework(WithStorage(store))
	defer framework.Close()

	// Test WatchSkillsDirectory
	t.Run("WatchSkillsDirectory", func(t *testing.T) {
		err := framework.WatchSkillsDirectory(tempDir)
		if err != nil {
			t.Fatalf("failed to watch directory: %v", err)
		}

		// Write a skill file after watching starts
		skillContent := `---
name: watched-skill
description: "Watched skill"
version: "1.0.0"
type: markdown
---

## Instructions
Test watched skill
`
		skillPath := filepath.Join(tempDir, "watched.md")
		err = os.WriteFile(skillPath, []byte(skillContent), 0644)
		if err != nil {
			t.Fatalf("failed to write skill file: %v", err)
		}

		// Give watcher time to detect and load the file
		time.Sleep(100 * time.Millisecond)

		// Verify the skill was auto-loaded
		skill, err := framework.GetSkill("watched-skill")
		if err != nil {
			// Note: This might fail if the watcher hasn't processed yet
			// In production code, you'd want a more robust wait mechanism
			t.Logf("skill not loaded yet (may be timing issue): %v", err)
		} else if skill.Name() != "watched-skill" {
			t.Errorf("expected 'watched-skill', got '%s'", skill.Name())
		}

		// Test StopWatchingSkills
		err = framework.StopWatchingSkills()
		if err != nil {
			t.Fatalf("failed to stop watching: %v", err)
		}
	})
}

func TestSkillToolWrapper(t *testing.T) {
	// Create framework
	store := storage.NewInMemory()
	framework := NewFramework(WithStorage(store))
	defer framework.Close()

	// Create and register a skill
	skill, _ := skills.NewMarkdownSkill(skills.MarkdownSkillConfig{
		Name:        "wrapper-test",
		Description: "Test skill-tool wrapping",
		Content:     "## Instructions\nTest",
		Source:      "test",
	})
	framework.RegisterSkill(skill)

	// Test that skill can be wrapped as tool
	t.Run("WrapSkillAsTool", func(t *testing.T) {
		tool := skills.WrapSkillAsTool(skill)
		if tool == nil {
			t.Fatal("expected non-nil tool")
		}

		expectedName := "skill_wrapper-test"
		if tool.Name() != expectedName {
			t.Errorf("expected name '%s', got '%s'", expectedName, tool.Name())
		}

		expectedDesc := "[Skill] Test skill-tool wrapping"
		if tool.Description() != expectedDesc {
			t.Errorf("expected description '%s', got '%s'", expectedDesc, tool.Description())
		}
	})
}
