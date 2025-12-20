// Package validation provides request validation for the Minion framework.
// It includes validators for LLM prompts, batch sizes, and other common request parameters.
package validation

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ValidationError represents a validation error with details.
type ValidationError struct {
	Field   string
	Message string
	Value   interface{}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s: %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	var sb strings.Builder
	sb.WriteString("multiple validation errors: ")
	for i, err := range e {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(err.Error())
	}
	return sb.String()
}

// HasErrors returns true if there are any validation errors.
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Validator validates request parameters.
type Validator struct {
	errors ValidationErrors
}

// NewValidator creates a new validator.
func NewValidator() *Validator {
	return &Validator{
		errors: make(ValidationErrors, 0),
	}
}

// AddError adds a validation error.
func (v *Validator) AddError(field, message string, value interface{}) {
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// Errors returns all validation errors.
func (v *Validator) Errors() ValidationErrors {
	return v.errors
}

// Validate returns an error if there are any validation errors.
func (v *Validator) Validate() error {
	if v.errors.HasErrors() {
		return v.errors
	}
	return nil
}

// Required validates that a string is not empty.
func (v *Validator) Required(field, value string) *Validator {
	if strings.TrimSpace(value) == "" {
		v.AddError(field, "is required", value)
	}
	return v
}

// MinLength validates minimum string length.
func (v *Validator) MinLength(field, value string, min int) *Validator {
	if utf8.RuneCountInString(value) < min {
		v.AddError(field, fmt.Sprintf("must be at least %d characters", min), value)
	}
	return v
}

// MaxLength validates maximum string length.
func (v *Validator) MaxLength(field, value string, max int) *Validator {
	if utf8.RuneCountInString(value) > max {
		v.AddError(field, fmt.Sprintf("must be at most %d characters", max), value)
	}
	return v
}

// Range validates that an integer is within a range.
func (v *Validator) Range(field string, value, min, max int) *Validator {
	if value < min || value > max {
		v.AddError(field, fmt.Sprintf("must be between %d and %d", min, max), value)
	}
	return v
}

// Min validates minimum integer value.
func (v *Validator) Min(field string, value, min int) *Validator {
	if value < min {
		v.AddError(field, fmt.Sprintf("must be at least %d", min), value)
	}
	return v
}

// Max validates maximum integer value.
func (v *Validator) Max(field string, value, max int) *Validator {
	if value > max {
		v.AddError(field, fmt.Sprintf("must be at most %d", max), value)
	}
	return v
}

// Positive validates that an integer is positive.
func (v *Validator) Positive(field string, value int) *Validator {
	if value <= 0 {
		v.AddError(field, "must be positive", value)
	}
	return v
}

// NonNegative validates that an integer is non-negative.
func (v *Validator) NonNegative(field string, value int) *Validator {
	if value < 0 {
		v.AddError(field, "must be non-negative", value)
	}
	return v
}

// FloatRange validates that a float is within a range.
func (v *Validator) FloatRange(field string, value, min, max float64) *Validator {
	if value < min || value > max {
		v.AddError(field, fmt.Sprintf("must be between %f and %f", min, max), value)
	}
	return v
}

// PromptLimits defines limits for LLM prompts.
type PromptLimits struct {
	// MaxPromptLength is the maximum length in characters.
	MaxPromptLength int
	// MaxTokens is an approximate maximum token count.
	MaxTokens int
	// MaxMessages is the maximum number of messages in a conversation.
	MaxMessages int
}

// DefaultPromptLimits returns default limits for LLM prompts.
func DefaultPromptLimits() PromptLimits {
	return PromptLimits{
		MaxPromptLength: 100000,  // 100K characters
		MaxTokens:       32000,   // 32K tokens
		MaxMessages:     100,     // 100 messages max
	}
}

// ProviderPromptLimits returns prompt limits for specific providers.
var ProviderPromptLimits = map[string]PromptLimits{
	"openai": {
		MaxPromptLength: 128000 * 4, // GPT-4 Turbo context
		MaxTokens:       128000,
		MaxMessages:     100,
	},
	"anthropic": {
		MaxPromptLength: 200000 * 4, // Claude 3 context
		MaxTokens:       200000,
		MaxMessages:     100,
	},
	"google": {
		MaxPromptLength: 32000 * 4, // Gemini Pro
		MaxTokens:       32000,
		MaxMessages:     100,
	},
	"cohere": {
		MaxPromptLength: 4096 * 4,
		MaxTokens:       4096,
		MaxMessages:     50,
	},
}

// GetPromptLimits returns limits for a provider, or defaults if not found.
func GetPromptLimits(provider string) PromptLimits {
	if limits, ok := ProviderPromptLimits[provider]; ok {
		return limits
	}
	return DefaultPromptLimits()
}

// LLMRequestValidator validates LLM request parameters.
type LLMRequestValidator struct {
	*Validator
	limits PromptLimits
}

// NewLLMRequestValidator creates a new LLM request validator.
func NewLLMRequestValidator(provider string) *LLMRequestValidator {
	return &LLMRequestValidator{
		Validator: NewValidator(),
		limits:    GetPromptLimits(provider),
	}
}

// NewLLMRequestValidatorWithLimits creates a validator with custom limits.
func NewLLMRequestValidatorWithLimits(limits PromptLimits) *LLMRequestValidator {
	return &LLMRequestValidator{
		Validator: NewValidator(),
		limits:    limits,
	}
}

// ValidatePrompt validates a prompt string.
func (v *LLMRequestValidator) ValidatePrompt(prompt string) *LLMRequestValidator {
	v.Required("prompt", prompt)
	v.MaxLength("prompt", prompt, v.limits.MaxPromptLength)
	return v
}

// ValidateMessages validates a list of messages.
func (v *LLMRequestValidator) ValidateMessages(messages []Message) *LLMRequestValidator {
	if len(messages) == 0 {
		v.AddError("messages", "at least one message is required", messages)
		return v
	}

	if len(messages) > v.limits.MaxMessages {
		v.AddError("messages", fmt.Sprintf("must have at most %d messages", v.limits.MaxMessages), len(messages))
	}

	totalLength := 0
	for i, msg := range messages {
		if msg.Content == "" {
			v.AddError(fmt.Sprintf("messages[%d].content", i), "is required", msg.Content)
		}
		if msg.Role == "" {
			v.AddError(fmt.Sprintf("messages[%d].role", i), "is required", msg.Role)
		}
		if !isValidRole(msg.Role) {
			v.AddError(fmt.Sprintf("messages[%d].role", i), "must be 'system', 'user', or 'assistant'", msg.Role)
		}
		totalLength += len(msg.Content)
	}

	if totalLength > v.limits.MaxPromptLength {
		v.AddError("messages", fmt.Sprintf("total content length must be at most %d characters", v.limits.MaxPromptLength), totalLength)
	}

	return v
}

// ValidateMaxTokens validates the max_tokens parameter.
func (v *LLMRequestValidator) ValidateMaxTokens(maxTokens int) *LLMRequestValidator {
	if maxTokens > 0 && maxTokens > v.limits.MaxTokens {
		v.AddError("max_tokens", fmt.Sprintf("must be at most %d", v.limits.MaxTokens), maxTokens)
	}
	return v
}

// ValidateTemperature validates the temperature parameter.
func (v *LLMRequestValidator) ValidateTemperature(temperature float64) *LLMRequestValidator {
	if temperature < 0 || temperature > 2.0 {
		v.AddError("temperature", "must be between 0 and 2.0", temperature)
	}
	return v
}

// ValidateTopP validates the top_p parameter.
func (v *LLMRequestValidator) ValidateTopP(topP float64) *LLMRequestValidator {
	if topP < 0 || topP > 1.0 {
		v.AddError("top_p", "must be between 0 and 1.0", topP)
	}
	return v
}

// Message represents a chat message for validation.
type Message struct {
	Role    string
	Content string
}

func isValidRole(role string) bool {
	switch role {
	case "system", "user", "assistant", "function", "tool":
		return true
	default:
		return false
	}
}

// BatchValidator validates batch operation parameters.
type BatchValidator struct {
	*Validator
	maxBatchSize int
}

// NewBatchValidator creates a new batch validator.
func NewBatchValidator(maxBatchSize int) *BatchValidator {
	return &BatchValidator{
		Validator:    NewValidator(),
		maxBatchSize: maxBatchSize,
	}
}

// ValidateBatchSize validates the batch size.
func (v *BatchValidator) ValidateBatchSize(size int) *BatchValidator {
	v.Positive("batch_size", size)
	v.Max("batch_size", size, v.maxBatchSize)
	return v
}

// ValidateItems validates that items is not empty and within batch size.
func (v *BatchValidator) ValidateItems(items interface{}, length int) *BatchValidator {
	if length == 0 {
		v.AddError("items", "at least one item is required", items)
	}
	if length > v.maxBatchSize {
		v.AddError("items", fmt.Sprintf("must have at most %d items", v.maxBatchSize), length)
	}
	return v
}

// EmbeddingValidator validates embedding request parameters.
type EmbeddingValidator struct {
	*Validator
	maxInputLength int
	maxBatchSize   int
}

// NewEmbeddingValidator creates a new embedding validator.
func NewEmbeddingValidator(maxInputLength, maxBatchSize int) *EmbeddingValidator {
	return &EmbeddingValidator{
		Validator:      NewValidator(),
		maxInputLength: maxInputLength,
		maxBatchSize:   maxBatchSize,
	}
}

// DefaultEmbeddingValidator creates a validator with default limits.
func DefaultEmbeddingValidator() *EmbeddingValidator {
	return NewEmbeddingValidator(8192, 100)
}

// ValidateInput validates a single embedding input.
func (v *EmbeddingValidator) ValidateInput(input string) *EmbeddingValidator {
	v.Required("input", input)
	v.MaxLength("input", input, v.maxInputLength)
	return v
}

// ValidateInputs validates multiple embedding inputs.
func (v *EmbeddingValidator) ValidateInputs(inputs []string) *EmbeddingValidator {
	if len(inputs) == 0 {
		v.AddError("inputs", "at least one input is required", inputs)
		return v
	}
	if len(inputs) > v.maxBatchSize {
		v.AddError("inputs", fmt.Sprintf("must have at most %d inputs", v.maxBatchSize), len(inputs))
	}

	for i, input := range inputs {
		if input == "" {
			v.AddError(fmt.Sprintf("inputs[%d]", i), "is required", input)
		} else if utf8.RuneCountInString(input) > v.maxInputLength {
			v.AddError(fmt.Sprintf("inputs[%d]", i), fmt.Sprintf("must be at most %d characters", v.maxInputLength), len(input))
		}
	}

	return v
}

// VectorStoreValidator validates vector store parameters.
type VectorStoreValidator struct {
	*Validator
}

// NewVectorStoreValidator creates a new vector store validator.
func NewVectorStoreValidator() *VectorStoreValidator {
	return &VectorStoreValidator{
		Validator: NewValidator(),
	}
}

// ValidateK validates the k parameter for similarity search.
func (v *VectorStoreValidator) ValidateK(k int) *VectorStoreValidator {
	v.Positive("k", k)
	v.Max("k", k, 1000) // Reasonable upper limit
	return v
}

// ValidateFetchK validates the fetch_k parameter for MMR search.
func (v *VectorStoreValidator) ValidateFetchK(fetchK, k int) *VectorStoreValidator {
	v.Positive("fetch_k", fetchK)
	if fetchK < k {
		v.AddError("fetch_k", "must be at least k", fetchK)
	}
	return v
}

// ValidateLambda validates the lambda parameter for MMR search.
func (v *VectorStoreValidator) ValidateLambda(lambda float64) *VectorStoreValidator {
	v.FloatRange("lambda", lambda, 0.0, 1.0)
	return v
}

// ValidateScoreThreshold validates a score threshold.
func (v *VectorStoreValidator) ValidateScoreThreshold(threshold float64) *VectorStoreValidator {
	v.FloatRange("score_threshold", threshold, 0.0, 1.0)
	return v
}

// TextSplitterValidator validates text splitter parameters.
type TextSplitterValidator struct {
	*Validator
}

// NewTextSplitterValidator creates a new text splitter validator.
func NewTextSplitterValidator() *TextSplitterValidator {
	return &TextSplitterValidator{
		Validator: NewValidator(),
	}
}

// ValidateChunkSize validates chunk size.
func (v *TextSplitterValidator) ValidateChunkSize(chunkSize int) *TextSplitterValidator {
	v.Positive("chunk_size", chunkSize)
	v.Max("chunk_size", chunkSize, 100000) // 100K max
	return v
}

// ValidateChunkOverlap validates chunk overlap relative to chunk size.
func (v *TextSplitterValidator) ValidateChunkOverlap(overlap, chunkSize int) *TextSplitterValidator {
	v.NonNegative("chunk_overlap", overlap)
	if overlap >= chunkSize {
		v.AddError("chunk_overlap", "must be less than chunk_size", overlap)
	}
	return v
}

// Quick validation functions

// ValidatePromptQuick quickly validates a prompt for a provider.
func ValidatePromptQuick(provider, prompt string) error {
	return NewLLMRequestValidator(provider).ValidatePrompt(prompt).Validate()
}

// ValidateMessagesQuick quickly validates messages for a provider.
func ValidateMessagesQuick(provider string, messages []Message) error {
	return NewLLMRequestValidator(provider).ValidateMessages(messages).Validate()
}

// ValidateBatchQuick quickly validates batch parameters.
func ValidateBatchQuick(size, maxSize int) error {
	return NewBatchValidator(maxSize).ValidateBatchSize(size).Validate()
}

// ValidateEmbeddingInputQuick quickly validates embedding input.
func ValidateEmbeddingInputQuick(input string) error {
	return DefaultEmbeddingValidator().ValidateInput(input).Validate()
}
