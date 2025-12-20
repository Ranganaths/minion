package outputparser

import (
	"testing"
)

// TestStringOutputParser tests the string parser
func TestStringOutputParser(t *testing.T) {
	parser := NewStringOutputParser()

	t.Run("BasicParse", func(t *testing.T) {
		result, err := parser.Parse("  Hello World  ")
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		if result != "Hello World" {
			t.Errorf("expected 'Hello World', got '%v'", result)
		}
	})

	t.Run("EmptyInstructions", func(t *testing.T) {
		instructions := parser.GetFormatInstructions()
		if instructions != "" {
			t.Errorf("expected empty instructions, got '%s'", instructions)
		}
	})
}

// TestJSONOutputParser tests the JSON parser
func TestJSONOutputParser(t *testing.T) {
	t.Run("ParseObject", func(t *testing.T) {
		parser := NewJSONOutputParser(JSONOutputParserConfig{})

		result, err := parser.Parse(`{"name": "Alice", "age": 30}`)
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		obj, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}

		if obj["name"] != "Alice" {
			t.Errorf("expected name 'Alice', got '%v'", obj["name"])
		}
	})

	t.Run("ParseArray", func(t *testing.T) {
		parser := NewJSONOutputParser(JSONOutputParserConfig{})

		result, err := parser.Parse(`["a", "b", "c"]`)
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected array, got %T", result)
		}

		if len(arr) != 3 {
			t.Errorf("expected 3 items, got %d", len(arr))
		}
	})

	t.Run("ParseFromCodeBlock", func(t *testing.T) {
		parser := NewJSONOutputParser(JSONOutputParserConfig{})

		text := "Here's the JSON:\n```json\n{\"key\": \"value\"}\n```"
		result, err := parser.Parse(text)
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		obj, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}

		if obj["key"] != "value" {
			t.Errorf("expected 'value', got '%v'", obj["key"])
		}
	})

	t.Run("ParseEmbedded", func(t *testing.T) {
		parser := NewJSONOutputParser(JSONOutputParserConfig{})

		text := "The result is: {\"answer\": 42} as expected."
		result, err := parser.Parse(text)
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		obj, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}

		if obj["answer"] != float64(42) {
			t.Errorf("expected 42, got '%v'", obj["answer"])
		}
	})

	t.Run("FormatInstructions", func(t *testing.T) {
		parser := NewJSONOutputParser(JSONOutputParserConfig{
			Schema: map[string]any{
				"name": "string",
				"age":  "number",
			},
		})

		instructions := parser.GetFormatInstructions()
		if instructions == "" {
			t.Error("expected non-empty instructions")
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		parser := NewJSONOutputParser(JSONOutputParserConfig{})

		_, err := parser.Parse("not json at all")
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

// TestListOutputParser tests the list parser
func TestListOutputParser(t *testing.T) {
	t.Run("NewlineSeparated", func(t *testing.T) {
		parser := NewListOutputParser(ListOutputParserConfig{})

		text := "item1\nitem2\nitem3"
		result, err := parser.Parse(text)
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		list, ok := result.([]string)
		if !ok {
			t.Fatalf("expected []string, got %T", result)
		}

		if len(list) != 3 {
			t.Errorf("expected 3 items, got %d", len(list))
		}
	})

	t.Run("CommaSeparated", func(t *testing.T) {
		parser := NewListOutputParser(ListOutputParserConfig{})

		text := "item1, item2, item3"
		result, err := parser.Parse(text)
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		list, ok := result.([]string)
		if !ok {
			t.Fatalf("expected []string, got %T", result)
		}

		if len(list) != 3 {
			t.Errorf("expected 3 items, got %d", len(list))
		}
	})

	t.Run("BulletedList", func(t *testing.T) {
		parser := NewListOutputParser(ListOutputParserConfig{})

		text := "- item1\n- item2\n- item3"
		result, err := parser.Parse(text)
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		list, ok := result.([]string)
		if !ok {
			t.Fatalf("expected []string, got %T", result)
		}

		if list[0] != "item1" {
			t.Errorf("expected 'item1', got '%s'", list[0])
		}
	})

	t.Run("NumberedList", func(t *testing.T) {
		parser := NewListOutputParser(ListOutputParserConfig{})

		text := "1. item1\n2. item2\n3. item3"
		result, err := parser.Parse(text)
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		list, ok := result.([]string)
		if !ok {
			t.Fatalf("expected []string, got %T", result)
		}

		if list[0] != "item1" {
			t.Errorf("expected 'item1', got '%s'", list[0])
		}
	})

	t.Run("CustomSeparator", func(t *testing.T) {
		parser := NewListOutputParser(ListOutputParserConfig{
			Separator: ";",
		})

		text := "item1;item2;item3"
		result, err := parser.Parse(text)
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		list, ok := result.([]string)
		if !ok {
			t.Fatalf("expected []string, got %T", result)
		}

		if len(list) != 3 {
			t.Errorf("expected 3 items, got %d", len(list))
		}
	})
}

// TestBooleanOutputParser tests the boolean parser
func TestBooleanOutputParser(t *testing.T) {
	parser := NewBooleanOutputParser()

	testCases := []struct {
		input    string
		expected bool
		shouldError bool
	}{
		{"yes", true, false},
		{"Yes", true, false},
		{"YES", true, false},
		{"true", true, false},
		{"True", true, false},
		{"1", true, false},
		{"no", false, false},
		{"No", false, false},
		{"false", false, false},
		{"False", false, false},
		{"0", false, false},
		{"maybe", false, true},
		{"unknown", false, true},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := parser.Parse(tc.input)

			if tc.shouldError {
				if err == nil {
					t.Errorf("expected error for input '%s'", tc.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for '%s': %v", tc.input, err)
			}

			if result != tc.expected {
				t.Errorf("expected %v for '%s', got %v", tc.expected, tc.input, result)
			}
		})
	}
}

// TestRegexOutputParser tests the regex parser
func TestRegexOutputParser(t *testing.T) {
	t.Run("NamedGroups", func(t *testing.T) {
		parser, err := NewRegexOutputParser(RegexOutputParserConfig{
			Pattern:    `Answer: (?P<answer>\d+)`,
			OutputKeys: []string{"answer"},
		})
		if err != nil {
			t.Fatalf("failed to create parser: %v", err)
		}

		result, err := parser.Parse("The result is: Answer: 42")
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		obj, ok := result.(map[string]string)
		if !ok {
			t.Fatalf("expected map[string]string, got %T", result)
		}

		if obj["answer"] != "42" {
			t.Errorf("expected '42', got '%s'", obj["answer"])
		}
	})

	t.Run("NoMatch", func(t *testing.T) {
		parser, err := NewRegexOutputParser(RegexOutputParserConfig{
			Pattern: `Answer: (?P<answer>\d+)`,
		})
		if err != nil {
			t.Fatalf("failed to create parser: %v", err)
		}

		_, err = parser.Parse("No answer here")
		if err == nil {
			t.Error("expected error for no match")
		}
	})

	t.Run("InvalidPattern", func(t *testing.T) {
		_, err := NewRegexOutputParser(RegexOutputParserConfig{
			Pattern: `[invalid`,
		})
		if err == nil {
			t.Error("expected error for invalid pattern")
		}
	})
}

// TestStructuredOutputParser tests the structured parser
func TestStructuredOutputParser(t *testing.T) {
	t.Run("ParseJSON", func(t *testing.T) {
		parser := NewStructuredOutputParser(StructuredOutputParserConfig{
			Fields: []FieldSchema{
				{Name: "name", Type: "string"},
				{Name: "age", Type: "number"},
			},
		})

		result, err := parser.Parse(`{"name": "Alice", "age": 30}`)
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		obj, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}

		if obj["name"] != "Alice" {
			t.Errorf("expected 'Alice', got '%v'", obj["name"])
		}
	})

	t.Run("ParseKeyValue", func(t *testing.T) {
		parser := NewStructuredOutputParser(StructuredOutputParserConfig{
			Fields: []FieldSchema{
				{Name: "name", Type: "string"},
				{Name: "age", Type: "number"},
				{Name: "active", Type: "boolean"},
			},
		})

		text := "name: Alice\nage: 30\nactive: true"
		result, err := parser.Parse(text)
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		obj, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}

		if obj["name"] != "Alice" {
			t.Errorf("expected 'Alice', got '%v'", obj["name"])
		}
		if obj["age"] != float64(30) {
			t.Errorf("expected 30, got '%v'", obj["age"])
		}
		if obj["active"] != true {
			t.Errorf("expected true, got '%v'", obj["active"])
		}
	})

	t.Run("FormatInstructions", func(t *testing.T) {
		parser := NewStructuredOutputParser(StructuredOutputParserConfig{
			Fields: []FieldSchema{
				{Name: "name", Type: "string", Description: "The name", Required: true},
			},
		})

		instructions := parser.GetFormatInstructions()
		if instructions == "" {
			t.Error("expected non-empty instructions")
		}
	})
}

// TestCompositeOutputParser tests the composite parser
func TestCompositeOutputParser(t *testing.T) {
	t.Run("FirstSuccess", func(t *testing.T) {
		parser := NewCompositeOutputParser(
			NewJSONOutputParser(JSONOutputParserConfig{}),
			NewStringOutputParser(),
		)

		result, err := parser.Parse(`{"key": "value"}`)
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		obj, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}

		if obj["key"] != "value" {
			t.Errorf("expected 'value', got '%v'", obj["key"])
		}
	})

	t.Run("FallbackToSecond", func(t *testing.T) {
		parser := NewCompositeOutputParser(
			NewJSONOutputParser(JSONOutputParserConfig{}),
			NewStringOutputParser(),
		)

		result, err := parser.Parse("not json")
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		str, ok := result.(string)
		if !ok {
			t.Fatalf("expected string, got %T", result)
		}

		if str != "not json" {
			t.Errorf("expected 'not json', got '%s'", str)
		}
	})

	t.Run("AllFail", func(t *testing.T) {
		// Create a regex parser that will fail on non-matching input
		regexParser, _ := NewRegexOutputParser(RegexOutputParserConfig{
			Pattern: `^EXACT_MATCH_REQUIRED$`,
		})

		parser := NewCompositeOutputParser(regexParser)

		_, err := parser.Parse("this won't match")
		if err == nil {
			t.Error("expected error when all parsers fail")
		}
	})
}
