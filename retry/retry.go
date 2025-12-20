// Package retry provides retry functionality with exponential backoff.
// It supports context cancellation, custom retry conditions, and configurable backoff strategies.
package retry

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/Ranganaths/minion/errors"
)

// Config configures retry behavior
type Config struct {
	// MaxRetries is the maximum number of retry attempts (0 = no retries)
	MaxRetries int

	// InitialDelay is the initial delay before the first retry
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// Multiplier is the factor by which delay increases after each retry
	Multiplier float64

	// Jitter adds randomness to delays (0.0 to 1.0)
	Jitter float64

	// RetryIf determines if an error should trigger a retry
	RetryIf func(error) bool
}

// DefaultConfig returns a default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
		RetryIf:      DefaultRetryIf,
	}
}

// DefaultRetryIf returns true for retryable errors
func DefaultRetryIf(err error) bool {
	return errors.IsRetryable(err) || errors.IsRateLimited(err) || errors.IsTimeout(err)
}

// Option configures retry behavior
type Option func(*Config)

// WithMaxRetries sets the maximum number of retries
func WithMaxRetries(n int) Option {
	return func(c *Config) {
		c.MaxRetries = n
	}
}

// WithInitialDelay sets the initial delay
func WithInitialDelay(d time.Duration) Option {
	return func(c *Config) {
		c.InitialDelay = d
	}
}

// WithMaxDelay sets the maximum delay
func WithMaxDelay(d time.Duration) Option {
	return func(c *Config) {
		c.MaxDelay = d
	}
}

// WithMultiplier sets the backoff multiplier
func WithMultiplier(m float64) Option {
	return func(c *Config) {
		c.Multiplier = m
	}
}

// WithJitter sets the jitter factor
func WithJitter(j float64) Option {
	return func(c *Config) {
		c.Jitter = j
	}
}

// WithRetryIf sets the retry condition function
func WithRetryIf(f func(error) bool) Option {
	return func(c *Config) {
		c.RetryIf = f
	}
}

// RetryAlways always retries on error
func RetryAlways(err error) bool {
	return err != nil
}

// RetryNever never retries
func RetryNever(err error) bool {
	return false
}

// Do executes the function with retries
func Do[T any](ctx context.Context, fn func() (T, error), opts ...Option) (T, error) {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	var result T
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			return result, errors.Wrap(ctx.Err(), "retry canceled")
		default:
		}

		result, lastErr = fn()
		if lastErr == nil {
			return result, nil
		}

		// Check if we should retry
		if !cfg.RetryIf(lastErr) {
			return result, lastErr
		}

		// Don't sleep after the last attempt
		if attempt < cfg.MaxRetries {
			delay := calculateDelay(cfg, attempt)
			select {
			case <-ctx.Done():
				return result, errors.Wrap(ctx.Err(), "retry canceled during backoff")
			case <-time.After(delay):
			}
		}
	}

	return result, errors.Wrapf(lastErr, "max retries (%d) exceeded", cfg.MaxRetries)
}

// DoVoid executes a function that returns only an error with retries
func DoVoid(ctx context.Context, fn func() error, opts ...Option) error {
	_, err := Do(ctx, func() (struct{}, error) {
		return struct{}{}, fn()
	}, opts...)
	return err
}

// calculateDelay calculates the delay for a given attempt
func calculateDelay(cfg Config, attempt int) time.Duration {
	// Calculate exponential delay
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt))

	// Apply max delay cap
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	// Apply jitter
	if cfg.Jitter > 0 {
		jitterRange := delay * cfg.Jitter
		delay = delay - jitterRange + (rand.Float64() * 2 * jitterRange)
	}

	return time.Duration(delay)
}

// Backoff represents a backoff strategy
type Backoff struct {
	cfg     Config
	attempt int
}

// NewBackoff creates a new backoff instance
func NewBackoff(opts ...Option) *Backoff {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Backoff{cfg: cfg}
}

// Next returns the next backoff duration and increments the attempt counter
func (b *Backoff) Next() time.Duration {
	delay := calculateDelay(b.cfg, b.attempt)
	b.attempt++
	return delay
}

// Reset resets the backoff to its initial state
func (b *Backoff) Reset() {
	b.attempt = 0
}

// Attempt returns the current attempt number
func (b *Backoff) Attempt() int {
	return b.attempt
}

// Exhausted returns true if all retries have been used
func (b *Backoff) Exhausted() bool {
	return b.attempt > b.cfg.MaxRetries
}

// Wait waits for the next backoff duration, respecting context cancellation
func (b *Backoff) Wait(ctx context.Context) error {
	delay := b.Next()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// WithRetry is a helper that wraps a function with retry logic
type WithRetry[T any] struct {
	fn   func() (T, error)
	opts []Option
}

// NewWithRetry creates a new WithRetry wrapper
func NewWithRetry[T any](fn func() (T, error), opts ...Option) *WithRetry[T] {
	return &WithRetry[T]{fn: fn, opts: opts}
}

// Run executes the function with retries
func (w *WithRetry[T]) Run(ctx context.Context) (T, error) {
	return Do(ctx, w.fn, w.opts...)
}

// OnRetry is called before each retry attempt
type OnRetryFunc func(attempt int, err error, delay time.Duration)

// DoWithCallback executes with retries and calls the callback before each retry
func DoWithCallback[T any](ctx context.Context, fn func() (T, error), onRetry OnRetryFunc, opts ...Option) (T, error) {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	var result T
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return result, errors.Wrap(ctx.Err(), "retry canceled")
		default:
		}

		result, lastErr = fn()
		if lastErr == nil {
			return result, nil
		}

		if !cfg.RetryIf(lastErr) {
			return result, lastErr
		}

		if attempt < cfg.MaxRetries {
			delay := calculateDelay(cfg, attempt)
			if onRetry != nil {
				onRetry(attempt+1, lastErr, delay)
			}
			select {
			case <-ctx.Done():
				return result, errors.Wrap(ctx.Err(), "retry canceled during backoff")
			case <-time.After(delay):
			}
		}
	}

	return result, errors.Wrapf(lastErr, "max retries (%d) exceeded", cfg.MaxRetries)
}
