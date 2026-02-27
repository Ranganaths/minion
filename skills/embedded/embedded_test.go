package embedded

import (
	"testing"

	"github.com/Ranganaths/minion/skills"
)

func TestEmbeddedSkillsExist(t *testing.T) {
	entries, err := SkillsFS.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read embedded skills directory: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("no embedded skills found")
	}

	t.Logf("found %d embedded skill files", len(entries))
	for _, entry := range entries {
		t.Logf("  - %s", entry.Name())
	}
}

func TestLoadEmbeddedSkills(t *testing.T) {
	registry := skills.NewSkillRegistry()
	loader := skills.NewSkillLoader(registry)

	// Load from embedded FS
	count, err := loader.LoadFromEmbed(SkillsFS, SkillsPattern)
	if err != nil {
		t.Fatalf("failed to load embedded skills: %v", err)
	}

	if count == 0 {
		t.Fatal("no skills loaded from embedded FS")
	}

	t.Logf("loaded %d embedded skills", count)

	// Verify we can retrieve the skills
	skillsList := registry.List()
	if len(skillsList) != count {
		t.Errorf("expected %d skills in registry, got %d", count, len(skillsList))
	}

	// Test individual embedded skills
	expectedSkills := []string{"code-review", "data-analysis", "api-design"}
	for _, name := range expectedSkills {
		skill, err := registry.Get(name)
		if err != nil {
			t.Errorf("expected embedded skill '%s' not found: %v", name, err)
			continue
		}

		if skill.Name() != name {
			t.Errorf("skill name mismatch: expected '%s', got '%s'", name, skill.Name())
		}

		if skill.Type() != skills.SkillTypeMarkdown {
			t.Errorf("expected markdown skill type for '%s', got '%s'", name, skill.Type())
		}

		t.Logf("verified embedded skill: %s - %s", skill.Name(), skill.Description())
	}
}

func TestEmbeddedSkillContent(t *testing.T) {
	registry := skills.NewSkillRegistry()
	loader := skills.NewSkillLoader(registry)
	loader.LoadFromEmbed(SkillsFS, SkillsPattern)

	// Test code-review skill
	t.Run("code-review skill", func(t *testing.T) {
		skill, err := registry.Get("code-review")
		if err != nil {
			t.Fatalf("code-review skill not found: %v", err)
		}

		mdSkill, ok := skill.(*skills.MarkdownSkill)
		if !ok {
			t.Fatal("expected MarkdownSkill type")
		}

		// Verify it has system prompt
		systemPrompt := mdSkill.SystemPrompt()
		if systemPrompt == "" {
			t.Error("code-review skill has empty system prompt")
		}

		// Verify it has examples
		examples := mdSkill.Examples()
		if len(examples) == 0 {
			t.Error("code-review skill has no examples")
		}

		t.Logf("code-review skill has %d examples", len(examples))
	})

	// Test data-analysis skill
	t.Run("data-analysis skill", func(t *testing.T) {
		skill, err := registry.Get("data-analysis")
		if err != nil {
			t.Fatalf("data-analysis skill not found: %v", err)
		}

		mdSkill, ok := skill.(*skills.MarkdownSkill)
		if !ok {
			t.Fatal("expected MarkdownSkill type")
		}

		systemPrompt := mdSkill.SystemPrompt()
		if systemPrompt == "" {
			t.Error("data-analysis skill has empty system prompt")
		}
	})

	// Test api-design skill
	t.Run("api-design skill", func(t *testing.T) {
		skill, err := registry.Get("api-design")
		if err != nil {
			t.Fatalf("api-design skill not found: %v", err)
		}

		mdSkill, ok := skill.(*skills.MarkdownSkill)
		if !ok {
			t.Fatal("expected MarkdownSkill type")
		}

		systemPrompt := mdSkill.SystemPrompt()
		if systemPrompt == "" {
			t.Error("api-design skill has empty system prompt")
		}
	})
}
