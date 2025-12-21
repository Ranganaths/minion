package chain

import (
	"context"
	"fmt"
	"time"
)

// BaseChain provides common functionality for all chains
type BaseChain struct {
	name      string
	options   *Options
	callbacks *CallbackManager
}

// NewBaseChain creates a new base chain with the given name and options
func NewBaseChain(name string, opts ...Option) *BaseChain {
	options := ApplyOptions(opts...)

	bc := &BaseChain{
		name:      name,
		options:   options,
		callbacks: NewCallbackManager(options.Callbacks...),
	}

	return bc
}

// Name returns the chain name
func (bc *BaseChain) Name() string {
	return bc.name
}

// Options returns the chain options
func (bc *BaseChain) Options() *Options {
	return bc.options
}

// Callbacks returns the callback manager
func (bc *BaseChain) Callbacks() *CallbackManager {
	return bc.callbacks
}

// NotifyStart notifies callbacks that the chain has started
func (bc *BaseChain) NotifyStart(ctx context.Context, inputs map[string]any) {
	bc.callbacks.OnChainStart(ctx, bc.name, inputs)
	if bc.options.Verbose {
		fmt.Printf("[%s] Chain started with inputs: %v\n", bc.name, inputs)
	}
}

// NotifyEnd notifies callbacks that the chain has completed
func (bc *BaseChain) NotifyEnd(ctx context.Context, outputs map[string]any) {
	bc.callbacks.OnChainEnd(ctx, bc.name, outputs)
	if bc.options.Verbose {
		fmt.Printf("[%s] Chain completed with outputs: %v\n", bc.name, outputs)
	}
}

// NotifyError notifies callbacks that the chain encountered an error
func (bc *BaseChain) NotifyError(ctx context.Context, err error) {
	bc.callbacks.OnChainError(ctx, bc.name, err)
	if bc.options.Verbose {
		fmt.Printf("[%s] Chain error: %v\n", bc.name, err)
	}
}

// NotifyLLMStart notifies callbacks that an LLM call is starting
func (bc *BaseChain) NotifyLLMStart(ctx context.Context, prompt string) {
	bc.callbacks.OnLLMStart(ctx, prompt)
	if bc.options.Verbose {
		fmt.Printf("[%s] LLM call starting with prompt length: %d\n", bc.name, len(prompt))
	}
}

// NotifyLLMEnd notifies callbacks that an LLM call has completed
func (bc *BaseChain) NotifyLLMEnd(ctx context.Context, response string, tokens int) {
	bc.callbacks.OnLLMEnd(ctx, response, tokens)
	if bc.options.Verbose {
		fmt.Printf("[%s] LLM call completed with %d tokens\n", bc.name, tokens)
	}
}

// NotifyRetrieverStart notifies callbacks that retrieval is starting
func (bc *BaseChain) NotifyRetrieverStart(ctx context.Context, query string) {
	bc.callbacks.OnRetrieverStart(ctx, query)
	if bc.options.Verbose {
		fmt.Printf("[%s] Retriever starting with query: %s\n", bc.name, query)
	}
}

// NotifyRetrieverEnd notifies callbacks that retrieval has completed
func (bc *BaseChain) NotifyRetrieverEnd(ctx context.Context, docs []Document) {
	bc.callbacks.OnRetrieverEnd(ctx, docs)
	if bc.options.Verbose {
		fmt.Printf("[%s] Retriever completed with %d documents\n", bc.name, len(docs))
	}
}

// ValidateInputs checks that all required inputs are present
func (bc *BaseChain) ValidateInputs(inputs map[string]any, required []string) error {
	for _, key := range required {
		if _, ok := inputs[key]; !ok {
			return fmt.Errorf("missing required input: %s", key)
		}
	}
	return nil
}

// GetString extracts a string value from inputs
func (bc *BaseChain) GetString(inputs map[string]any, key string) (string, error) {
	val, ok := inputs[key]
	if !ok {
		return "", fmt.Errorf("missing input: %s", key)
	}
	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("input %s is not a string: %T", key, val)
	}
	return str, nil
}

// GetStringOr extracts a string value from inputs or returns a default
func (bc *BaseChain) GetStringOr(inputs map[string]any, key, defaultVal string) string {
	val, ok := inputs[key]
	if !ok {
		return defaultVal
	}
	str, ok := val.(string)
	if !ok {
		return defaultVal
	}
	return str
}

// WithTimeout wraps a context with the chain's timeout
func (bc *BaseChain) WithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if bc.options.Timeout > 0 {
		return context.WithTimeout(ctx, bc.options.Timeout)
	}
	return ctx, func() {}
}

// MakeStreamEvent creates a new stream event with current timestamp
func MakeStreamEvent(eventType StreamEventType, content string, data map[string]any, err error) StreamEvent {
	return StreamEvent{
		Type:      eventType,
		Content:   content,
		Data:      data,
		Error:     err,
		Timestamp: time.Now(),
	}
}

// CopyInputs creates a shallow copy of inputs map
func CopyInputs(inputs map[string]any) map[string]any {
	result := make(map[string]any, len(inputs))
	for k, v := range inputs {
		result[k] = v
	}
	return result
}

// MergeInputs merges multiple input maps, later values override earlier ones
func MergeInputs(maps ...map[string]any) map[string]any {
	result := make(map[string]any)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// --- Safe Type Assertion Helpers ---
// These functions provide safe type assertions with proper error handling
// to avoid runtime panics from type assertion failures.

// GetInt extracts an int value from inputs with safe type assertion
func (bc *BaseChain) GetInt(inputs map[string]any, key string) (int, error) {
	val, ok := inputs[key]
	if !ok {
		return 0, fmt.Errorf("missing input: %s", key)
	}
	switch v := val.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("input %s is not a number: %T", key, val)
	}
}

// GetIntOr extracts an int value from inputs or returns a default
func (bc *BaseChain) GetIntOr(inputs map[string]any, key string, defaultVal int) int {
	val, err := bc.GetInt(inputs, key)
	if err != nil {
		return defaultVal
	}
	return val
}

// GetFloat extracts a float64 value from inputs with safe type assertion
func (bc *BaseChain) GetFloat(inputs map[string]any, key string) (float64, error) {
	val, ok := inputs[key]
	if !ok {
		return 0, fmt.Errorf("missing input: %s", key)
	}
	switch v := val.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("input %s is not a number: %T", key, val)
	}
}

// GetFloatOr extracts a float64 value from inputs or returns a default
func (bc *BaseChain) GetFloatOr(inputs map[string]any, key string, defaultVal float64) float64 {
	val, err := bc.GetFloat(inputs, key)
	if err != nil {
		return defaultVal
	}
	return val
}

// GetBool extracts a bool value from inputs with safe type assertion
func (bc *BaseChain) GetBool(inputs map[string]any, key string) (bool, error) {
	val, ok := inputs[key]
	if !ok {
		return false, fmt.Errorf("missing input: %s", key)
	}
	b, ok := val.(bool)
	if !ok {
		return false, fmt.Errorf("input %s is not a bool: %T", key, val)
	}
	return b, nil
}

// GetBoolOr extracts a bool value from inputs or returns a default
func (bc *BaseChain) GetBoolOr(inputs map[string]any, key string, defaultVal bool) bool {
	val, err := bc.GetBool(inputs, key)
	if err != nil {
		return defaultVal
	}
	return val
}

// GetStringSlice extracts a string slice from inputs with safe type assertion
func (bc *BaseChain) GetStringSlice(inputs map[string]any, key string) ([]string, error) {
	val, ok := inputs[key]
	if !ok {
		return nil, fmt.Errorf("missing input: %s", key)
	}
	switch v := val.(type) {
	case []string:
		return v, nil
	case []interface{}:
		result := make([]string, 0, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("input %s[%d] is not a string: %T", key, i, item)
			}
			result = append(result, str)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("input %s is not a string slice: %T", key, val)
	}
}

// GetMap extracts a map from inputs with safe type assertion
func (bc *BaseChain) GetMap(inputs map[string]any, key string) (map[string]any, error) {
	val, ok := inputs[key]
	if !ok {
		return nil, fmt.Errorf("missing input: %s", key)
	}
	// map[string]any and map[string]interface{} are the same type
	if m, ok := val.(map[string]any); ok {
		return m, nil
	}
	return nil, fmt.Errorf("input %s is not a map: %T", key, val)
}

// AsString safely converts any value to string representation
func AsString(val any) string {
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", val)
}

// AsStringSlice safely converts any value to string slice
func AsStringSlice(val any) []string {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			result = append(result, AsString(item))
		}
		return result
	default:
		return []string{AsString(val)}
	}
}
