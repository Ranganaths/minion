package client

import (
	"context"
	"fmt"
	"math"
	"time"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
	Jitter         bool
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2.0,
		Jitter:         true,
	}
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func(ctx context.Context) error

// WithRetry executes an operation with exponential backoff retry
func WithRetry(ctx context.Context, config *RetryConfig, operation RetryableOperation) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	backoff := config.InitialBackoff

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Try the operation
		err := operation(ctx)
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Don't retry on last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Check if context is done
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}

		// Calculate backoff duration
		sleepDuration := backoff

		// Add jitter if enabled (randomize ±25%)
		if config.Jitter {
			jitter := float64(backoff) * 0.25 * (2*randFloat() - 1)
			sleepDuration = time.Duration(float64(backoff) + jitter)
		}

		// Ensure we don't exceed max backoff
		if sleepDuration > config.MaxBackoff {
			sleepDuration = config.MaxBackoff
		}

		// Sleep before next attempt
		select {
		case <-time.After(sleepDuration):
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled during backoff: %w", ctx.Err())
		}

		// Increase backoff for next iteration
		backoff = time.Duration(float64(backoff) * config.Multiplier)
		if backoff > config.MaxBackoff {
			backoff = config.MaxBackoff
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", config.MaxRetries, lastErr)
}

// RetryWithCallback executes an operation with retry and callbacks
func RetryWithCallback(
	ctx context.Context,
	config *RetryConfig,
	operation RetryableOperation,
	onRetry func(attempt int, err error, nextBackoff time.Duration),
) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	backoff := config.InitialBackoff

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Try the operation
		err := operation(ctx)
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Don't retry on last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Check if context is done
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}

		// Calculate backoff duration
		sleepDuration := backoff
		if config.Jitter {
			jitter := float64(backoff) * 0.25 * (2*randFloat() - 1)
			sleepDuration = time.Duration(float64(backoff) + jitter)
		}
		if sleepDuration > config.MaxBackoff {
			sleepDuration = config.MaxBackoff
		}

		// Callback before retry
		if onRetry != nil {
			onRetry(attempt, err, sleepDuration)
		}

		// Sleep before next attempt
		select {
		case <-time.After(sleepDuration):
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled during backoff: %w", ctx.Err())
		}

		// Increase backoff
		backoff = time.Duration(float64(backoff) * config.Multiplier)
		if backoff > config.MaxBackoff {
			backoff = config.MaxBackoff
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", config.MaxRetries, lastErr)
}

// calculateBackoff calculates the backoff duration for a given attempt
func calculateBackoff(attempt int, config *RetryConfig) time.Duration {
	backoff := float64(config.InitialBackoff) * math.Pow(config.Multiplier, float64(attempt))
	if backoff > float64(config.MaxBackoff) {
		backoff = float64(config.MaxBackoff)
	}

	if config.Jitter {
		jitter := backoff * 0.25 * (2*randFloat() - 1)
		backoff += jitter
	}

	return time.Duration(backoff)
}

// randFloat returns a pseudo-random float64 in [0.0, 1.0)
// This is a simple linear congruential generator for deterministic jitter
var randState uint64 = uint64(time.Now().UnixNano())

func randFloat() float64 {
	// Simple LCG: X(n+1) = (aX(n) + c) mod m
	randState = (randState*1103515245 + 12345) & 0x7fffffff
	return float64(randState) / float64(0x7fffffff)
}

// IsRetryable determines if an error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for common retryable errors
	errMsg := err.Error()

	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"network unreachable",
		"no route to host",
		"broken pipe",
		"EOF",
	}

	for _, pattern := range retryablePatterns {
		if contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsSub(s, substr))
}

func containsSub(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
