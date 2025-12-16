package resilience

import (
	"context"
	"errors"
	"time"
)

// ErrTimeout is returned when an operation times out
var ErrTimeout = errors.New("operation timed out")

// WithTimeout executes a function with a timeout
func WithTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Channel to receive the result
	done := make(chan error, 1)

	// Execute function in goroutine
	go func() {
		done <- fn(ctx)
	}()

	// Wait for completion or timeout
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ErrTimeout
		}
		return ctx.Err()
	}
}

// WithTimeoutResult executes a function with a timeout and returns a result
func WithTimeoutResult[T any](ctx context.Context, timeout time.Duration, fn func(context.Context) (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Result channel
	type result struct {
		value T
		err   error
	}
	done := make(chan result, 1)

	// Execute function in goroutine
	go func() {
		val, err := fn(ctx)
		done <- result{value: val, err: err}
	}()

	// Wait for completion or timeout
	select {
	case res := <-done:
		return res.value, res.err
	case <-ctx.Done():
		var zeroValue T
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return zeroValue, ErrTimeout
		}
		return zeroValue, ctx.Err()
	}
}

// TimeoutConfig configures timeout behavior
type TimeoutConfig struct {
	// Default timeout for operations
	DefaultTimeout time.Duration

	// SlowOperationThreshold for logging warnings
	SlowOperationThreshold time.Duration

	// OnTimeout callback when timeout occurs
	OnTimeout func(operation string, duration time.Duration)

	// OnSlowOperation callback when operation is slow but completes
	OnSlowOperation func(operation string, duration time.Duration)
}

// DefaultTimeoutConfig returns default timeout configuration
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		DefaultTimeout:         30 * time.Second,
		SlowOperationThreshold: 10 * time.Second,
	}
}

// TimeoutManager manages timeouts for operations
type TimeoutManager struct {
	config *TimeoutConfig
}

// NewTimeoutManager creates a new timeout manager
func NewTimeoutManager(config *TimeoutConfig) *TimeoutManager {
	if config == nil {
		config = DefaultTimeoutConfig()
	}

	return &TimeoutManager{
		config: config,
	}
}

// Execute runs a function with timeout management
func (tm *TimeoutManager) Execute(ctx context.Context, operation string, timeout time.Duration, fn func(context.Context) error) error {
	if timeout == 0 {
		timeout = tm.config.DefaultTimeout
	}

	start := time.Now()
	err := WithTimeout(ctx, timeout, fn)
	duration := time.Since(start)

	// Check for timeout
	if errors.Is(err, ErrTimeout) {
		if tm.config.OnTimeout != nil {
			tm.config.OnTimeout(operation, duration)
		}
		return err
	}

	// Check for slow operation
	if err == nil && duration > tm.config.SlowOperationThreshold {
		if tm.config.OnSlowOperation != nil {
			tm.config.OnSlowOperation(operation, duration)
		}
	}

	return err
}

// ExecuteWithResult runs a function with timeout management and returns a result
func ExecuteWithResult[T any](tm *TimeoutManager, ctx context.Context, operation string, timeout time.Duration, fn func(context.Context) (T, error)) (T, error) {
	if timeout == 0 {
		timeout = tm.config.DefaultTimeout
	}

	start := time.Now()
	result, err := WithTimeoutResult(ctx, timeout, fn)
	duration := time.Since(start)

	// Check for timeout
	if errors.Is(err, ErrTimeout) {
		if tm.config.OnTimeout != nil {
			tm.config.OnTimeout(operation, duration)
		}
		return result, err
	}

	// Check for slow operation
	if err == nil && duration > tm.config.SlowOperationThreshold {
		if tm.config.OnSlowOperation != nil {
			tm.config.OnSlowOperation(operation, duration)
		}
	}

	return result, err
}

// TimeoutDecorator wraps a function with timeout behavior
type TimeoutDecorator struct {
	timeout time.Duration
	name    string
}

// NewTimeoutDecorator creates a timeout decorator
func NewTimeoutDecorator(name string, timeout time.Duration) *TimeoutDecorator {
	return &TimeoutDecorator{
		timeout: timeout,
		name:    name,
	}
}

// Wrap wraps a function with timeout
func (td *TimeoutDecorator) Wrap(fn func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		return WithTimeout(ctx, td.timeout, fn)
	}
}

// WrapWithResult wraps a function that returns a result with timeout
func WrapWithResult[T any](td *TimeoutDecorator, fn func(context.Context) (T, error)) func(context.Context) (T, error) {
	return func(ctx context.Context) (T, error) {
		return WithTimeoutResult(ctx, td.timeout, fn)
	}
}

// AdaptiveTimeout implements adaptive timeout based on historical performance
type AdaptiveTimeout struct {
	baseTimeout    time.Duration
	successHistory []time.Duration
	maxHistory     int
	percentile     float64 // e.g., 0.95 for p95
}

// NewAdaptiveTimeout creates an adaptive timeout manager
func NewAdaptiveTimeout(baseTimeout time.Duration, percentile float64) *AdaptiveTimeout {
	return &AdaptiveTimeout{
		baseTimeout:    baseTimeout,
		successHistory: make([]time.Duration, 0, 100),
		maxHistory:     100,
		percentile:     percentile,
	}
}

// RecordSuccess records a successful operation duration
func (at *AdaptiveTimeout) RecordSuccess(duration time.Duration) {
	at.successHistory = append(at.successHistory, duration)

	// Keep only last N entries
	if len(at.successHistory) > at.maxHistory {
		at.successHistory = at.successHistory[1:]
	}
}

// GetTimeout returns the calculated timeout based on history
func (at *AdaptiveTimeout) GetTimeout() time.Duration {
	if len(at.successHistory) < 10 {
		// Not enough history, use base timeout
		return at.baseTimeout
	}

	// Calculate percentile from history
	// This is a simplified implementation
	// In production, use a proper percentile calculation
	total := time.Duration(0)
	for _, d := range at.successHistory {
		total += d
	}
	avg := total / time.Duration(len(at.successHistory))

	// Use percentile multiplier (e.g., p95 = avg * 2)
	multiplier := 1.0 + at.percentile
	calculated := time.Duration(float64(avg) * multiplier)

	// Ensure minimum and maximum bounds
	minTimeout := at.baseTimeout / 2
	maxTimeout := at.baseTimeout * 3

	if calculated < minTimeout {
		return minTimeout
	}
	if calculated > maxTimeout {
		return maxTimeout
	}

	return calculated
}

// Execute runs a function with adaptive timeout
func (at *AdaptiveTimeout) Execute(ctx context.Context, fn func(context.Context) error) error {
	timeout := at.GetTimeout()
	start := time.Now()

	err := WithTimeout(ctx, timeout, fn)

	// Record success for adaptation
	if err == nil {
		at.RecordSuccess(time.Since(start))
	}

	return err
}
