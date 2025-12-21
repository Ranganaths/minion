// Package config provides environment variable configuration helpers for the minion framework.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Env provides helpers for reading environment variables with defaults and type conversion.
type Env struct {
	prefix string
}

// NewEnv creates a new Env helper with an optional prefix.
// If prefix is provided, all environment variable lookups will be prefixed with it.
// For example, if prefix is "MINION", GetString("API_KEY") will look for "MINION_API_KEY".
func NewEnv(prefix string) *Env {
	return &Env{prefix: prefix}
}

// DefaultEnv is a global Env helper with "MINION" prefix.
var DefaultEnv = NewEnv("MINION")

// key returns the full environment variable name with prefix.
func (e *Env) key(name string) string {
	if e.prefix == "" {
		return name
	}
	return e.prefix + "_" + name
}

// GetString returns the value of the environment variable or the default.
func (e *Env) GetString(name, defaultValue string) string {
	if value := os.Getenv(e.key(name)); value != "" {
		return value
	}
	return defaultValue
}

// GetInt returns the integer value of the environment variable or the default.
func (e *Env) GetInt(name string, defaultValue int) int {
	if value := os.Getenv(e.key(name)); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// GetInt64 returns the int64 value of the environment variable or the default.
func (e *Env) GetInt64(name string, defaultValue int64) int64 {
	if value := os.Getenv(e.key(name)); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// GetFloat64 returns the float64 value of the environment variable or the default.
func (e *Env) GetFloat64(name string, defaultValue float64) float64 {
	if value := os.Getenv(e.key(name)); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// GetBool returns the boolean value of the environment variable or the default.
// Values "true", "1", "yes", "on" (case-insensitive) are considered true.
// Values "false", "0", "no", "off" (case-insensitive) are considered false.
func (e *Env) GetBool(name string, defaultValue bool) bool {
	if value := os.Getenv(e.key(name)); value != "" {
		switch strings.ToLower(value) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return defaultValue
}

// GetDuration returns the duration value of the environment variable or the default.
// The value should be a valid Go duration string (e.g., "10s", "5m", "1h").
func (e *Env) GetDuration(name string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(e.key(name)); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// GetStringSlice returns a slice of strings from a comma-separated environment variable.
func (e *Env) GetStringSlice(name string, defaultValue []string) []string {
	if value := os.Getenv(e.key(name)); value != "" {
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

// MustGetString returns the value of the environment variable or panics if not set.
// DEPRECATED: Use RequireString for production code to avoid panics.
func (e *Env) MustGetString(name string) string {
	if value := os.Getenv(e.key(name)); value != "" {
		return value
	}
	panic("required environment variable " + e.key(name) + " is not set")
}

// RequireString returns the value of the environment variable or an error if not set.
// This is the safer alternative to MustGetString for production use.
func (e *Env) RequireString(name string) (string, error) {
	if value := os.Getenv(e.key(name)); value != "" {
		return value, nil
	}
	return "", &EnvError{
		Name:    e.key(name),
		Message: "required environment variable is not set",
	}
}

// RequireInt returns the integer value of the environment variable or an error.
func (e *Env) RequireInt(name string) (int, error) {
	value := os.Getenv(e.key(name))
	if value == "" {
		return 0, &EnvError{
			Name:    e.key(name),
			Message: "required environment variable is not set",
		}
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, &EnvError{
			Name:    e.key(name),
			Message: "invalid integer value: " + value,
		}
	}
	return intValue, nil
}

// RequireBool returns the boolean value of the environment variable or an error.
func (e *Env) RequireBool(name string) (bool, error) {
	value := os.Getenv(e.key(name))
	if value == "" {
		return false, &EnvError{
			Name:    e.key(name),
			Message: "required environment variable is not set",
		}
	}
	switch strings.ToLower(value) {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off":
		return false, nil
	default:
		return false, &EnvError{
			Name:    e.key(name),
			Message: "invalid boolean value: " + value,
		}
	}
}

// RequireFloat64 returns the float64 value of the environment variable or an error.
func (e *Env) RequireFloat64(name string) (float64, error) {
	value := os.Getenv(e.key(name))
	if value == "" {
		return 0, &EnvError{
			Name:    e.key(name),
			Message: "required environment variable is not set",
		}
	}
	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, &EnvError{
			Name:    e.key(name),
			Message: "invalid float value: " + value,
		}
	}
	return floatValue, nil
}

// RequireDuration returns the duration value of the environment variable or an error.
func (e *Env) RequireDuration(name string) (time.Duration, error) {
	value := os.Getenv(e.key(name))
	if value == "" {
		return 0, &EnvError{
			Name:    e.key(name),
			Message: "required environment variable is not set",
		}
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, &EnvError{
			Name:    e.key(name),
			Message: "invalid duration value: " + value,
		}
	}
	return duration, nil
}

// EnvError represents an environment variable error
type EnvError struct {
	Name    string
	Message string
}

// Error implements the error interface
func (e *EnvError) Error() string {
	return e.Name + ": " + e.Message
}

// IsSet returns true if the environment variable is set (even if empty).
func (e *Env) IsSet(name string) bool {
	_, exists := os.LookupEnv(e.key(name))
	return exists
}

// Convenience functions using DefaultEnv

// GetString returns the value of the environment variable with MINION prefix or the default.
func GetString(name, defaultValue string) string {
	return DefaultEnv.GetString(name, defaultValue)
}

// GetInt returns the integer value of the environment variable with MINION prefix or the default.
func GetInt(name string, defaultValue int) int {
	return DefaultEnv.GetInt(name, defaultValue)
}

// GetInt64 returns the int64 value of the environment variable with MINION prefix or the default.
func GetInt64(name string, defaultValue int64) int64 {
	return DefaultEnv.GetInt64(name, defaultValue)
}

// GetFloat64 returns the float64 value of the environment variable with MINION prefix or the default.
func GetFloat64(name string, defaultValue float64) float64 {
	return DefaultEnv.GetFloat64(name, defaultValue)
}

// GetBool returns the boolean value of the environment variable with MINION prefix or the default.
func GetBool(name string, defaultValue bool) bool {
	return DefaultEnv.GetBool(name, defaultValue)
}

// GetDuration returns the duration value of the environment variable with MINION prefix or the default.
func GetDuration(name string, defaultValue time.Duration) time.Duration {
	return DefaultEnv.GetDuration(name, defaultValue)
}

// GetStringSlice returns a slice of strings from a comma-separated environment variable with MINION prefix.
func GetStringSlice(name string, defaultValue []string) []string {
	return DefaultEnv.GetStringSlice(name, defaultValue)
}

// MustGetString returns the value of the environment variable with MINION prefix or panics.
// DEPRECATED: Use RequireString for production code to avoid panics.
func MustGetString(name string) string {
	return DefaultEnv.MustGetString(name)
}

// RequireString returns the value of the environment variable with MINION prefix or an error.
func RequireString(name string) (string, error) {
	return DefaultEnv.RequireString(name)
}

// RequireInt returns the integer value of the environment variable with MINION prefix or an error.
func RequireInt(name string) (int, error) {
	return DefaultEnv.RequireInt(name)
}

// IsSet returns true if the environment variable with MINION prefix is set.
func IsSet(name string) bool {
	return DefaultEnv.IsSet(name)
}

// Common environment variable names used in the minion framework
const (
	// API Keys
	EnvOpenAIAPIKey      = "OPENAI_API_KEY"
	EnvAnthropicAPIKey   = "ANTHROPIC_API_KEY"
	EnvCohereAPIKey      = "COHERE_API_KEY"
	EnvHuggingFaceAPIKey = "HUGGINGFACE_API_KEY"

	// LLM Configuration
	EnvLLMProvider      = "LLM_PROVIDER"
	EnvLLMModel         = "LLM_MODEL"
	EnvLLMTemperature   = "LLM_TEMPERATURE"
	EnvLLMMaxTokens     = "LLM_MAX_TOKENS"
	EnvLLMTimeout       = "LLM_TIMEOUT"

	// Embedding Configuration
	EnvEmbeddingProvider   = "EMBEDDING_PROVIDER"
	EnvEmbeddingModel      = "EMBEDDING_MODEL"
	EnvEmbeddingDimension  = "EMBEDDING_DIMENSION"
	EnvEmbeddingBatchSize  = "EMBEDDING_BATCH_SIZE"

	// Vector Store Configuration
	EnvVectorStoreType     = "VECTORSTORE_TYPE"
	EnvVectorStoreURL      = "VECTORSTORE_URL"
	EnvVectorStoreDimension = "VECTORSTORE_DIMENSION"

	// Retry Configuration
	EnvRetryMaxRetries     = "RETRY_MAX_RETRIES"
	EnvRetryInitialDelay   = "RETRY_INITIAL_DELAY"
	EnvRetryMaxDelay       = "RETRY_MAX_DELAY"
	EnvRetryMultiplier     = "RETRY_MULTIPLIER"
	EnvRetryJitter         = "RETRY_JITTER"

	// Cache Configuration
	EnvCacheEnabled        = "CACHE_ENABLED"
	EnvCacheMaxSize        = "CACHE_MAX_SIZE"
	EnvCacheTTL            = "CACHE_TTL"
	EnvCacheCleanupInterval = "CACHE_CLEANUP_INTERVAL"

	// Logging Configuration
	EnvLogLevel            = "LOG_LEVEL"
	EnvLogFormat           = "LOG_FORMAT"

	// Metrics Configuration
	EnvMetricsEnabled      = "METRICS_ENABLED"
	EnvMetricsPort         = "METRICS_PORT"

	// Tracing Configuration
	EnvTracingEnabled      = "TRACING_ENABLED"
	EnvTracingExporter     = "TRACING_EXPORTER"
	EnvTracingEndpoint     = "TRACING_ENDPOINT"
	EnvTracingSamplingRatio = "TRACING_SAMPLING_RATIO"
)
