package resilience

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	// MaxAttempts is the maximum number of retry attempts (including first try)
	MaxAttempts int

	// InitialDelay is the delay before the first retry
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// Multiplier is the factor by which delay increases each retry
	Multiplier float64

	// Jitter adds randomness to delay to prevent thundering herd
	Jitter bool

	// RetryableErrors is a function that determines if an error is retryable
	// If nil, all errors are retryable
	RetryableErrors func(error) bool

	// OnRetry is called before each retry attempt
	OnRetry func(attempt int, err error, delay time.Duration)
}

// DefaultRetryPolicy returns a default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// ErrMaxRetriesExceeded is returned when max retries are exhausted
var ErrMaxRetriesExceeded = errors.New("max retries exceeded")

// Retry executes a function with retry logic
func Retry(ctx context.Context, policy *RetryPolicy, fn func() error) error {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}

	var lastErr error
	delay := policy.InitialDelay

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		// Execute the function
		err := fn()

		// Success!
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if policy.RetryableErrors != nil && !policy.RetryableErrors(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Check if we have more attempts
		if attempt >= policy.MaxAttempts {
			break
		}

		// Calculate delay for next retry
		currentDelay := delay
		if policy.Jitter {
			currentDelay = addJitter(delay)
		}

		// Call callback if provided
		if policy.OnRetry != nil {
			policy.OnRetry(attempt, err, currentDelay)
		}

		// Wait before retry (respecting context cancellation)
		select {
		case <-time.After(currentDelay):
			// Continue to retry
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		}

		// Increase delay for next iteration (exponential backoff)
		delay = time.Duration(float64(delay) * policy.Multiplier)
		if delay > policy.MaxDelay {
			delay = policy.MaxDelay
		}
	}

	return fmt.Errorf("%w: last error: %v", ErrMaxRetriesExceeded, lastErr)
}

// RetryWithResult executes a function that returns a result with retry logic
func RetryWithResult[T any](ctx context.Context, policy *RetryPolicy, fn func() (T, error)) (T, error) {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}

	var lastErr error
	var zeroValue T
	delay := policy.InitialDelay

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		// Execute the function
		result, err := fn()

		// Success!
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if policy.RetryableErrors != nil && !policy.RetryableErrors(err) {
			return zeroValue, fmt.Errorf("non-retryable error: %w", err)
		}

		// Check if we have more attempts
		if attempt >= policy.MaxAttempts {
			break
		}

		// Calculate delay for next retry
		currentDelay := delay
		if policy.Jitter {
			currentDelay = addJitter(delay)
		}

		// Call callback if provided
		if policy.OnRetry != nil {
			policy.OnRetry(attempt, err, currentDelay)
		}

		// Wait before retry (respecting context cancellation)
		select {
		case <-time.After(currentDelay):
			// Continue to retry
		case <-ctx.Done():
			return zeroValue, fmt.Errorf("retry cancelled: %w", ctx.Err())
		}

		// Increase delay for next iteration (exponential backoff)
		delay = time.Duration(float64(delay) * policy.Multiplier)
		if delay > policy.MaxDelay {
			delay = policy.MaxDelay
		}
	}

	return zeroValue, fmt.Errorf("%w: last error: %v", ErrMaxRetriesExceeded, lastErr)
}

// addJitter adds random jitter to prevent thundering herd
func addJitter(delay time.Duration) time.Duration {
	// Add up to 25% random jitter
	jitter := time.Duration(rand.Int63n(int64(float64(delay) * 0.25)))
	return delay + jitter
}

// CalculateDelay calculates the delay for a given attempt with exponential backoff
func CalculateDelay(attempt int, initialDelay time.Duration, multiplier float64, maxDelay time.Duration, jitter bool) time.Duration {
	// Calculate exponential delay: initialDelay * (multiplier ^ (attempt - 1))
	delay := time.Duration(float64(initialDelay) * math.Pow(multiplier, float64(attempt-1)))

	// Cap at max delay
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add jitter if enabled
	if jitter {
		delay = addJitter(delay)
	}

	return delay
}

// IsRetryableError checks common retryable error types
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common retryable errors
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return true
	case errors.Is(err, ErrCircuitOpen):
		return false // Don't retry if circuit is open
	case errors.Is(err, ErrTooManyRequests):
		return true // Retry rate limit errors
	default:
		// By default, consider network and temporary errors retryable
		// This is a simplified check - in production, use more sophisticated logic
		return true
	}
}

// RetryConfig provides a builder for RetryPolicy
type RetryConfig struct {
	policy *RetryPolicy
}

// NewRetryConfig creates a new retry configuration builder
func NewRetryConfig() *RetryConfig {
	return &RetryConfig{
		policy: DefaultRetryPolicy(),
	}
}

// WithMaxAttempts sets the maximum number of attempts
func (rc *RetryConfig) WithMaxAttempts(attempts int) *RetryConfig {
	rc.policy.MaxAttempts = attempts
	return rc
}

// WithInitialDelay sets the initial delay
func (rc *RetryConfig) WithInitialDelay(delay time.Duration) *RetryConfig {
	rc.policy.InitialDelay = delay
	return rc
}

// WithMaxDelay sets the maximum delay
func (rc *RetryConfig) WithMaxDelay(delay time.Duration) *RetryConfig {
	rc.policy.MaxDelay = delay
	return rc
}

// WithMultiplier sets the backoff multiplier
func (rc *RetryConfig) WithMultiplier(multiplier float64) *RetryConfig {
	rc.policy.Multiplier = multiplier
	return rc
}

// WithJitter enables or disables jitter
func (rc *RetryConfig) WithJitter(enabled bool) *RetryConfig {
	rc.policy.Jitter = enabled
	return rc
}

// WithRetryableErrors sets the retryable errors function
func (rc *RetryConfig) WithRetryableErrors(fn func(error) bool) *RetryConfig {
	rc.policy.RetryableErrors = fn
	return rc
}

// WithOnRetry sets the retry callback
func (rc *RetryConfig) WithOnRetry(fn func(attempt int, err error, delay time.Duration)) *RetryConfig {
	rc.policy.OnRetry = fn
	return rc
}

// Build returns the configured retry policy
func (rc *RetryConfig) Build() *RetryPolicy {
	return rc.policy
}
