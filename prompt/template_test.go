package prompt

import (
	"testing"
)

// TestTemplate tests the basic template
func TestTemplate(t *testing.T) {
	t.Run("GoTemplate", func(t *testing.T) {
		tmpl, err := NewTemplate(TemplateConfig{
			Template: "Hello {{.name}}, welcome to {{.place}}!",
		})
		if err != nil {
			t.Fatalf("failed to create template: %v", err)
		}

		result, err := tmpl.Format(map[string]any{
			"name":  "Alice",
			"place": "Wonderland",
		})
		if err != nil {
			t.Fatalf("format failed: %v", err)
		}

		expected := "Hello Alice, welcome to Wonderland!"
		if result != expected {
			t.Errorf("expected '%s', got '%s'", expected, result)
		}
	})

	t.Run("FStringTemplate", func(t *testing.T) {
		tmpl, err := NewTemplate(TemplateConfig{
			Template:     "Hello {name}, welcome to {place}!",
			TemplateType: TemplateTypeFString,
		})
		if err != nil {
			t.Fatalf("failed to create template: %v", err)
		}

		result, err := tmpl.Format(map[string]any{
			"name":  "Bob",
			"place": "Paradise",
		})
		if err != nil {
			t.Fatalf("format failed: %v", err)
		}

		expected := "Hello Bob, welcome to Paradise!"
		if result != expected {
			t.Errorf("expected '%s', got '%s'", expected, result)
		}
	})

	t.Run("MissingVariable", func(t *testing.T) {
		tmpl, err := NewTemplate(TemplateConfig{
			Template: "Hello {{.name}}!",
		})
		if err != nil {
			t.Fatalf("failed to create template: %v", err)
		}

		_, err = tmpl.Format(map[string]any{})
		if err == nil {
			t.Error("expected error for missing variable")
		}
	})

	t.Run("PartialVariables", func(t *testing.T) {
		tmpl, err := NewTemplate(TemplateConfig{
			Template: "Hello {{.name}} from {{.company}}!",
			PartialVariables: map[string]any{
				"company": "Acme Inc",
			},
		})
		if err != nil {
			t.Fatalf("failed to create template: %v", err)
		}

		result, err := tmpl.Format(map[string]any{
			"name": "Charlie",
		})
		if err != nil {
			t.Fatalf("format failed: %v", err)
		}

		expected := "Hello Charlie from Acme Inc!"
		if result != expected {
			t.Errorf("expected '%s', got '%s'", expected, result)
		}
	})

	t.Run("InputVariables", func(t *testing.T) {
		tmpl, err := NewTemplate(TemplateConfig{
			Template: "{{.a}} and {{.b}} and {{.c}}",
		})
		if err != nil {
			t.Fatalf("failed to create template: %v", err)
		}

		vars := tmpl.InputVariables()
		if len(vars) != 3 {
			t.Errorf("expected 3 input variables, got %d", len(vars))
		}
	})

	t.Run("PartialFormat", func(t *testing.T) {
		tmpl, err := NewTemplate(TemplateConfig{
			Template: "{{.a}} and {{.b}}",
		})
		if err != nil {
			t.Fatalf("failed to create template: %v", err)
		}

		partial, err := tmpl.PartialFormat(map[string]any{"a": "first"})
		if err != nil {
			t.Fatalf("partial format failed: %v", err)
		}

		if len(partial.InputVariables()) != 1 {
			t.Errorf("expected 1 remaining variable, got %d", len(partial.InputVariables()))
		}

		result, err := partial.Format(map[string]any{"b": "second"})
		if err != nil {
			t.Fatalf("format failed: %v", err)
		}

		expected := "first and second"
		if result != expected {
			t.Errorf("expected '%s', got '%s'", expected, result)
		}
	})
}

// TestChatTemplate tests chat templates
func TestChatTemplate(t *testing.T) {
	t.Run("BasicChatTemplate", func(t *testing.T) {
		tmpl, err := NewChatTemplate(ChatTemplateConfig{
			SystemTemplate: "You are a helpful assistant named {{.name}}.",
			HumanTemplate:  "{{.question}}",
		})
		if err != nil {
			t.Fatalf("failed to create chat template: %v", err)
		}

		messages, err := tmpl.FormatMessages(map[string]any{
			"name":     "Claude",
			"question": "What is 2+2?",
		})
		if err != nil {
			t.Fatalf("format failed: %v", err)
		}

		if len(messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(messages))
		}

		if messages[0].Role != "system" {
			t.Errorf("expected first message to be system")
		}

		if messages[1].Role != "user" {
			t.Errorf("expected second message to be user")
		}
	})

	t.Run("AllMessageTypes", func(t *testing.T) {
		tmpl, err := NewChatTemplate(ChatTemplateConfig{
			SystemTemplate: "System message",
			HumanTemplate:  "Human message",
			AITemplate:     "AI message",
		})
		if err != nil {
			t.Fatalf("failed to create chat template: %v", err)
		}

		messages, err := tmpl.FormatMessages(map[string]any{})
		if err != nil {
			t.Fatalf("format failed: %v", err)
		}

		if len(messages) != 3 {
			t.Errorf("expected 3 messages, got %d", len(messages))
		}
	})

	t.Run("InputVariables", func(t *testing.T) {
		tmpl, err := NewChatTemplate(ChatTemplateConfig{
			SystemTemplate: "{{.system_var}}",
			HumanTemplate:  "{{.human_var}}",
		})
		if err != nil {
			t.Fatalf("failed to create chat template: %v", err)
		}

		vars := tmpl.InputVariables()
		if len(vars) != 2 {
			t.Errorf("expected 2 input variables, got %d", len(vars))
		}
	})
}

// TestFewShotTemplate tests few-shot templates
func TestFewShotTemplate(t *testing.T) {
	t.Run("BasicFewShot", func(t *testing.T) {
		tmpl, err := NewFewShotTemplate(FewShotTemplateConfig{
			Prefix: "Here are some examples:",
			Suffix: "Now answer: {{.question}}",
			Examples: []map[string]any{
				{"input": "2+2", "output": "4"},
				{"input": "3+3", "output": "6"},
			},
			ExampleTemplate: "Input: {{.input}}\nOutput: {{.output}}",
			Separator:       "\n\n",
		})
		if err != nil {
			t.Fatalf("failed to create few-shot template: %v", err)
		}

		result, err := tmpl.Format(map[string]any{
			"question": "5+5",
		})
		if err != nil {
			t.Fatalf("format failed: %v", err)
		}

		if !containsString(result, "Here are some examples:") {
			t.Error("expected prefix in result")
		}
		if !containsString(result, "Input: 2+2") {
			t.Error("expected first example in result")
		}
		if !containsString(result, "Now answer: 5+5") {
			t.Error("expected suffix in result")
		}
	})

	t.Run("InputVariables", func(t *testing.T) {
		tmpl, err := NewFewShotTemplate(FewShotTemplateConfig{
			Suffix:          "Answer: {{.question}}",
			Examples:        []map[string]any{},
			ExampleTemplate: "{{.x}}",
		})
		if err != nil {
			t.Fatalf("failed to create few-shot template: %v", err)
		}

		vars := tmpl.InputVariables()
		if len(vars) != 1 || vars[0] != "question" {
			t.Errorf("expected [question], got %v", vars)
		}
	})
}

// TestExtractVariables tests variable extraction
func TestExtractVariables(t *testing.T) {
	t.Run("GoTemplateVariables", func(t *testing.T) {
		vars := extractVariables("{{.a}} and {{.b}} and {{.a}}", TemplateTypeGoTemplate)
		if len(vars) != 2 {
			t.Errorf("expected 2 unique variables, got %d", len(vars))
		}
	})

	t.Run("FStringVariables", func(t *testing.T) {
		vars := extractVariables("{a} and {b} and {a}", TemplateTypeFString)
		if len(vars) != 2 {
			t.Errorf("expected 2 unique variables, got %d", len(vars))
		}
	})
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
