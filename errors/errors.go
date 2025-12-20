// Package errors provides typed errors for the minion framework.
// All errors support errors.Is() and errors.As() for proper error handling.
package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for use with errors.Is()
var (
	// ErrInvalidConfig indicates invalid configuration
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrMissingRequired indicates a required field is missing
	ErrMissingRequired = errors.New("missing required field")

	// ErrInvalidInput indicates invalid input data
	ErrInvalidInput = errors.New("invalid input")

	// ErrNotFound indicates a resource was not found
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists indicates a resource already exists
	ErrAlreadyExists = errors.New("already exists")

	// ErrClosed indicates the resource has been closed
	ErrClosed = errors.New("resource closed")

	// ErrTimeout indicates an operation timed out
	ErrTimeout = errors.New("operation timed out")

	// ErrCanceled indicates an operation was canceled
	ErrCanceled = errors.New("operation canceled")

	// ErrRateLimited indicates rate limiting was triggered
	ErrRateLimited = errors.New("rate limited")

	// ErrQuotaExceeded indicates a quota was exceeded
	ErrQuotaExceeded = errors.New("quota exceeded")

	// ErrAuthFailed indicates authentication failed
	ErrAuthFailed = errors.New("authentication failed")

	// ErrPermissionDenied indicates insufficient permissions
	ErrPermissionDenied = errors.New("permission denied")

	// ErrDimensionMismatch indicates embedding dimensions don't match
	ErrDimensionMismatch = errors.New("embedding dimension mismatch")

	// ErrEmptyInput indicates empty input was provided
	ErrEmptyInput = errors.New("empty input")

	// ErrFileTooLarge indicates a file exceeds size limits
	ErrFileTooLarge = errors.New("file too large")

	// ErrUnsupportedOperation indicates an unsupported operation
	ErrUnsupportedOperation = errors.New("unsupported operation")

	// ErrRetryable indicates the error is retryable
	ErrRetryable = errors.New("retryable error")

	// ErrPermanent indicates a permanent error that should not be retried
	ErrPermanent = errors.New("permanent error")
)

// ChainError represents an error that occurred during chain execution
type ChainError struct {
	ChainName string
	Operation string
	Err       error
}

func (e *ChainError) Error() string {
	if e.Operation != "" {
		return fmt.Sprintf("chain %s: %s: %v", e.ChainName, e.Operation, e.Err)
	}
	return fmt.Sprintf("chain %s: %v", e.ChainName, e.Err)
}

func (e *ChainError) Unwrap() error {
	return e.Err
}

// NewChainError creates a new chain error
func NewChainError(chainName, operation string, err error) *ChainError {
	return &ChainError{
		ChainName: chainName,
		Operation: operation,
		Err:       err,
	}
}

// EmbeddingError represents an error during embedding operations
type EmbeddingError struct {
	Provider  string
	Operation string
	Err       error
	Retryable bool
}

func (e *EmbeddingError) Error() string {
	return fmt.Sprintf("embedding %s: %s: %v", e.Provider, e.Operation, e.Err)
}

func (e *EmbeddingError) Unwrap() error {
	return e.Err
}

func (e *EmbeddingError) Is(target error) bool {
	if e.Retryable && errors.Is(target, ErrRetryable) {
		return true
	}
	return false
}

// NewEmbeddingError creates a new embedding error
func NewEmbeddingError(provider, operation string, err error, retryable bool) *EmbeddingError {
	return &EmbeddingError{
		Provider:  provider,
		Operation: operation,
		Err:       err,
		Retryable: retryable,
	}
}

// VectorStoreError represents an error during vector store operations
type VectorStoreError struct {
	Store     string
	Operation string
	Err       error
}

func (e *VectorStoreError) Error() string {
	return fmt.Sprintf("vectorstore %s: %s: %v", e.Store, e.Operation, e.Err)
}

func (e *VectorStoreError) Unwrap() error {
	return e.Err
}

// NewVectorStoreError creates a new vector store error
func NewVectorStoreError(store, operation string, err error) *VectorStoreError {
	return &VectorStoreError{
		Store:     store,
		Operation: operation,
		Err:       err,
	}
}

// LLMError represents an error during LLM operations
type LLMError struct {
	Provider   string
	Model      string
	Operation  string
	StatusCode int
	Err        error
	Retryable  bool
}

func (e *LLMError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("llm %s/%s: %s (status %d): %v", e.Provider, e.Model, e.Operation, e.StatusCode, e.Err)
	}
	return fmt.Sprintf("llm %s/%s: %s: %v", e.Provider, e.Model, e.Operation, e.Err)
}

func (e *LLMError) Unwrap() error {
	return e.Err
}

func (e *LLMError) Is(target error) bool {
	if e.Retryable && errors.Is(target, ErrRetryable) {
		return true
	}
	if e.StatusCode == 429 && errors.Is(target, ErrRateLimited) {
		return true
	}
	if e.StatusCode == 401 && errors.Is(target, ErrAuthFailed) {
		return true
	}
	if e.StatusCode == 403 && errors.Is(target, ErrPermissionDenied) {
		return true
	}
	return false
}

// NewLLMError creates a new LLM error
func NewLLMError(provider, model, operation string, statusCode int, err error) *LLMError {
	retryable := statusCode == 429 || statusCode == 500 || statusCode == 502 || statusCode == 503 || statusCode == 504
	return &LLMError{
		Provider:   provider,
		Model:      model,
		Operation:  operation,
		StatusCode: statusCode,
		Err:        err,
		Retryable:  retryable,
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Value   any
	Message string
}

func (e *ValidationError) Error() string {
	if e.Value != nil {
		return fmt.Sprintf("validation error: %s: %s (got %v)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("validation error: %s: %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalidInput
}

// NewValidationError creates a new validation error
func NewValidationError(field string, value any, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// ConfigError represents a configuration error
type ConfigError struct {
	Component string
	Field     string
	Message   string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error: %s.%s: %s", e.Component, e.Field, e.Message)
}

func (e *ConfigError) Unwrap() error {
	return ErrInvalidConfig
}

// NewConfigError creates a new configuration error
func NewConfigError(component, field, message string) *ConfigError {
	return &ConfigError{
		Component: component,
		Field:     field,
		Message:   message,
	}
}

// RetryableError wraps an error and marks it as retryable
type RetryableError struct {
	Err         error
	MaxRetries  int
	RetryAfter  int // seconds, if known
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

func (e *RetryableError) Is(target error) bool {
	return errors.Is(target, ErrRetryable)
}

// NewRetryableError creates a new retryable error
func NewRetryableError(err error, maxRetries int) *RetryableError {
	return &RetryableError{
		Err:        err,
		MaxRetries: maxRetries,
	}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	return errors.Is(err, ErrRetryable)
}

// IsRateLimited checks if an error is due to rate limiting
func IsRateLimited(err error) bool {
	return errors.Is(err, ErrRateLimited)
}

// IsTimeout checks if an error is due to timeout
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// IsCanceled checks if an error is due to cancellation
func IsCanceled(err error) bool {
	return errors.Is(err, ErrCanceled)
}

// IsNotFound checks if an error is due to resource not found
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// Wrap wraps an error with additional context
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// Wrapf wraps an error with formatted context
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format+": %w", append(args, err)...)
}
