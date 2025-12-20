package validation

import (
	"errors"
	"strings"
	"testing"
)

func TestValidator(t *testing.T) {
	t.Run("required passes", func(t *testing.T) {
		v := NewValidator()
		v.Required("name", "John")
		if v.Errors().HasErrors() {
			t.Error("expected no errors")
		}
	})

	t.Run("required fails on empty", func(t *testing.T) {
		v := NewValidator()
		v.Required("name", "")
		if !v.Errors().HasErrors() {
			t.Error("expected error for empty string")
		}
	})

	t.Run("required fails on whitespace", func(t *testing.T) {
		v := NewValidator()
		v.Required("name", "   ")
		if !v.Errors().HasErrors() {
			t.Error("expected error for whitespace string")
		}
	})

	t.Run("min length passes", func(t *testing.T) {
		v := NewValidator()
		v.MinLength("name", "John", 3)
		if v.Errors().HasErrors() {
			t.Error("expected no errors")
		}
	})

	t.Run("min length fails", func(t *testing.T) {
		v := NewValidator()
		v.MinLength("name", "Jo", 3)
		if !v.Errors().HasErrors() {
			t.Error("expected error for short string")
		}
	})

	t.Run("max length passes", func(t *testing.T) {
		v := NewValidator()
		v.MaxLength("name", "John", 10)
		if v.Errors().HasErrors() {
			t.Error("expected no errors")
		}
	})

	t.Run("max length fails", func(t *testing.T) {
		v := NewValidator()
		v.MaxLength("name", "John Doe Smith", 10)
		if !v.Errors().HasErrors() {
			t.Error("expected error for long string")
		}
	})

	t.Run("range passes", func(t *testing.T) {
		v := NewValidator()
		v.Range("age", 25, 18, 65)
		if v.Errors().HasErrors() {
			t.Error("expected no errors")
		}
	})

	t.Run("range fails below", func(t *testing.T) {
		v := NewValidator()
		v.Range("age", 15, 18, 65)
		if !v.Errors().HasErrors() {
			t.Error("expected error for below range")
		}
	})

	t.Run("range fails above", func(t *testing.T) {
		v := NewValidator()
		v.Range("age", 70, 18, 65)
		if !v.Errors().HasErrors() {
			t.Error("expected error for above range")
		}
	})

	t.Run("positive passes", func(t *testing.T) {
		v := NewValidator()
		v.Positive("count", 5)
		if v.Errors().HasErrors() {
			t.Error("expected no errors")
		}
	})

	t.Run("positive fails on zero", func(t *testing.T) {
		v := NewValidator()
		v.Positive("count", 0)
		if !v.Errors().HasErrors() {
			t.Error("expected error for zero")
		}
	})

	t.Run("positive fails on negative", func(t *testing.T) {
		v := NewValidator()
		v.Positive("count", -1)
		if !v.Errors().HasErrors() {
			t.Error("expected error for negative")
		}
	})

	t.Run("non negative passes on zero", func(t *testing.T) {
		v := NewValidator()
		v.NonNegative("count", 0)
		if v.Errors().HasErrors() {
			t.Error("expected no errors")
		}
	})

	t.Run("non negative fails", func(t *testing.T) {
		v := NewValidator()
		v.NonNegative("count", -1)
		if !v.Errors().HasErrors() {
			t.Error("expected error for negative")
		}
	})

	t.Run("float range passes", func(t *testing.T) {
		v := NewValidator()
		v.FloatRange("temperature", 0.7, 0.0, 2.0)
		if v.Errors().HasErrors() {
			t.Error("expected no errors")
		}
	})

	t.Run("float range fails", func(t *testing.T) {
		v := NewValidator()
		v.FloatRange("temperature", 2.5, 0.0, 2.0)
		if !v.Errors().HasErrors() {
			t.Error("expected error for out of range")
		}
	})

	t.Run("chaining works", func(t *testing.T) {
		v := NewValidator()
		err := v.Required("name", "Jo").
			MinLength("name", "Jo", 3).
			Validate()

		if err == nil {
			t.Error("expected validation error")
		}

		if len(v.Errors()) != 1 {
			t.Errorf("expected 1 error, got %d", len(v.Errors()))
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		v := NewValidator()
		v.Required("name", "")
		v.Positive("age", -5)

		if len(v.Errors()) != 2 {
			t.Errorf("expected 2 errors, got %d", len(v.Errors()))
		}

		errStr := v.Errors().Error()
		if !strings.Contains(errStr, "multiple validation errors") {
			t.Errorf("expected 'multiple validation errors', got %s", errStr)
		}
	})
}

func TestLLMRequestValidator(t *testing.T) {
	t.Run("valid prompt", func(t *testing.T) {
		v := NewLLMRequestValidator("openai")
		err := v.ValidatePrompt("Hello, how are you?").Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty prompt fails", func(t *testing.T) {
		v := NewLLMRequestValidator("openai")
		err := v.ValidatePrompt("").Validate()
		if err == nil {
			t.Error("expected error for empty prompt")
		}
	})

	t.Run("valid messages", func(t *testing.T) {
		v := NewLLMRequestValidator("openai")
		messages := []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		}
		err := v.ValidateMessages(messages).Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty messages fails", func(t *testing.T) {
		v := NewLLMRequestValidator("openai")
		err := v.ValidateMessages([]Message{}).Validate()
		if err == nil {
			t.Error("expected error for empty messages")
		}
	})

	t.Run("invalid role fails", func(t *testing.T) {
		v := NewLLMRequestValidator("openai")
		messages := []Message{
			{Role: "invalid", Content: "Hello"},
		}
		err := v.ValidateMessages(messages).Validate()
		if err == nil {
			t.Error("expected error for invalid role")
		}
	})

	t.Run("empty content fails", func(t *testing.T) {
		v := NewLLMRequestValidator("openai")
		messages := []Message{
			{Role: "user", Content: ""},
		}
		err := v.ValidateMessages(messages).Validate()
		if err == nil {
			t.Error("expected error for empty content")
		}
	})

	t.Run("valid temperature", func(t *testing.T) {
		v := NewLLMRequestValidator("openai")
		err := v.ValidateTemperature(0.7).Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid temperature", func(t *testing.T) {
		v := NewLLMRequestValidator("openai")
		err := v.ValidateTemperature(2.5).Validate()
		if err == nil {
			t.Error("expected error for invalid temperature")
		}
	})

	t.Run("valid top_p", func(t *testing.T) {
		v := NewLLMRequestValidator("openai")
		err := v.ValidateTopP(0.9).Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid top_p", func(t *testing.T) {
		v := NewLLMRequestValidator("openai")
		err := v.ValidateTopP(1.5).Validate()
		if err == nil {
			t.Error("expected error for invalid top_p")
		}
	})

	t.Run("max tokens within limit", func(t *testing.T) {
		v := NewLLMRequestValidator("openai")
		err := v.ValidateMaxTokens(4096).Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("max tokens exceeds limit", func(t *testing.T) {
		v := NewLLMRequestValidatorWithLimits(PromptLimits{MaxTokens: 4096})
		err := v.ValidateMaxTokens(10000).Validate()
		if err == nil {
			t.Error("expected error for exceeding max tokens")
		}
	})
}

func TestProviderLimits(t *testing.T) {
	t.Run("openai limits", func(t *testing.T) {
		limits := GetPromptLimits("openai")
		if limits.MaxTokens != 128000 {
			t.Errorf("expected 128000, got %d", limits.MaxTokens)
		}
	})

	t.Run("anthropic limits", func(t *testing.T) {
		limits := GetPromptLimits("anthropic")
		if limits.MaxTokens != 200000 {
			t.Errorf("expected 200000, got %d", limits.MaxTokens)
		}
	})

	t.Run("unknown provider uses defaults", func(t *testing.T) {
		limits := GetPromptLimits("unknown")
		defaults := DefaultPromptLimits()
		if limits.MaxTokens != defaults.MaxTokens {
			t.Errorf("expected default %d, got %d", defaults.MaxTokens, limits.MaxTokens)
		}
	})
}

func TestBatchValidator(t *testing.T) {
	t.Run("valid batch size", func(t *testing.T) {
		v := NewBatchValidator(100)
		err := v.ValidateBatchSize(50).Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("batch size exceeds max", func(t *testing.T) {
		v := NewBatchValidator(100)
		err := v.ValidateBatchSize(150).Validate()
		if err == nil {
			t.Error("expected error for exceeding batch size")
		}
	})

	t.Run("batch size zero fails", func(t *testing.T) {
		v := NewBatchValidator(100)
		err := v.ValidateBatchSize(0).Validate()
		if err == nil {
			t.Error("expected error for zero batch size")
		}
	})

	t.Run("valid items", func(t *testing.T) {
		v := NewBatchValidator(100)
		err := v.ValidateItems([]string{"a", "b"}, 2).Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty items fails", func(t *testing.T) {
		v := NewBatchValidator(100)
		err := v.ValidateItems([]string{}, 0).Validate()
		if err == nil {
			t.Error("expected error for empty items")
		}
	})
}

func TestEmbeddingValidator(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		v := DefaultEmbeddingValidator()
		err := v.ValidateInput("Hello world").Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty input fails", func(t *testing.T) {
		v := DefaultEmbeddingValidator()
		err := v.ValidateInput("").Validate()
		if err == nil {
			t.Error("expected error for empty input")
		}
	})

	t.Run("input too long fails", func(t *testing.T) {
		v := NewEmbeddingValidator(100, 10)
		longInput := strings.Repeat("a", 200)
		err := v.ValidateInput(longInput).Validate()
		if err == nil {
			t.Error("expected error for long input")
		}
	})

	t.Run("valid inputs", func(t *testing.T) {
		v := DefaultEmbeddingValidator()
		err := v.ValidateInputs([]string{"Hello", "World"}).Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty inputs fails", func(t *testing.T) {
		v := DefaultEmbeddingValidator()
		err := v.ValidateInputs([]string{}).Validate()
		if err == nil {
			t.Error("expected error for empty inputs")
		}
	})

	t.Run("too many inputs fails", func(t *testing.T) {
		v := NewEmbeddingValidator(1000, 2)
		err := v.ValidateInputs([]string{"a", "b", "c"}).Validate()
		if err == nil {
			t.Error("expected error for too many inputs")
		}
	})
}

func TestVectorStoreValidator(t *testing.T) {
	t.Run("valid k", func(t *testing.T) {
		v := NewVectorStoreValidator()
		err := v.ValidateK(10).Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("k zero fails", func(t *testing.T) {
		v := NewVectorStoreValidator()
		err := v.ValidateK(0).Validate()
		if err == nil {
			t.Error("expected error for zero k")
		}
	})

	t.Run("k too large fails", func(t *testing.T) {
		v := NewVectorStoreValidator()
		err := v.ValidateK(2000).Validate()
		if err == nil {
			t.Error("expected error for k > 1000")
		}
	})

	t.Run("valid lambda", func(t *testing.T) {
		v := NewVectorStoreValidator()
		err := v.ValidateLambda(0.5).Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("lambda out of range fails", func(t *testing.T) {
		v := NewVectorStoreValidator()
		err := v.ValidateLambda(1.5).Validate()
		if err == nil {
			t.Error("expected error for lambda > 1.0")
		}
	})

	t.Run("valid fetch_k", func(t *testing.T) {
		v := NewVectorStoreValidator()
		err := v.ValidateFetchK(20, 10).Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("fetch_k less than k fails", func(t *testing.T) {
		v := NewVectorStoreValidator()
		err := v.ValidateFetchK(5, 10).Validate()
		if err == nil {
			t.Error("expected error for fetch_k < k")
		}
	})
}

func TestTextSplitterValidator(t *testing.T) {
	t.Run("valid chunk size", func(t *testing.T) {
		v := NewTextSplitterValidator()
		err := v.ValidateChunkSize(1000).Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("chunk size zero fails", func(t *testing.T) {
		v := NewTextSplitterValidator()
		err := v.ValidateChunkSize(0).Validate()
		if err == nil {
			t.Error("expected error for zero chunk size")
		}
	})

	t.Run("valid overlap", func(t *testing.T) {
		v := NewTextSplitterValidator()
		err := v.ValidateChunkOverlap(100, 1000).Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("overlap >= chunk size fails", func(t *testing.T) {
		v := NewTextSplitterValidator()
		err := v.ValidateChunkOverlap(1000, 1000).Validate()
		if err == nil {
			t.Error("expected error for overlap >= chunk_size")
		}
	})
}

func TestQuickValidation(t *testing.T) {
	t.Run("ValidatePromptQuick", func(t *testing.T) {
		err := ValidatePromptQuick("openai", "Hello")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		err = ValidatePromptQuick("openai", "")
		if err == nil {
			t.Error("expected error for empty prompt")
		}
	})

	t.Run("ValidateMessagesQuick", func(t *testing.T) {
		err := ValidateMessagesQuick("openai", []Message{{Role: "user", Content: "Hello"}})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("ValidateBatchQuick", func(t *testing.T) {
		err := ValidateBatchQuick(50, 100)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		err = ValidateBatchQuick(150, 100)
		if err == nil {
			t.Error("expected error for exceeding batch size")
		}
	})

	t.Run("ValidateEmbeddingInputQuick", func(t *testing.T) {
		err := ValidateEmbeddingInputQuick("Hello world")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestValidationError(t *testing.T) {
	t.Run("error message", func(t *testing.T) {
		err := &ValidationError{
			Field:   "name",
			Message: "is required",
			Value:   "",
		}

		if err.Error() != "validation error: name: is required" {
			t.Errorf("unexpected error message: %s", err.Error())
		}
	})

	t.Run("errors as interface", func(t *testing.T) {
		v := NewValidator()
		v.Required("name", "")
		err := v.Validate()

		var validationErrors ValidationErrors
		if !errors.As(err, &validationErrors) {
			t.Error("expected ValidationErrors")
		}
	})
}
