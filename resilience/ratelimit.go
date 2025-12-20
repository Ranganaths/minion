// Package resilience provides resilience patterns for the minion framework.
// This includes rate limiting, circuit breakers, and other fault tolerance mechanisms.
package resilience

import (
	"context"
	"sync"
	"time"

	"github.com/Ranganaths/minion/errors"
)

// RateLimiter limits the rate of operations.
type RateLimiter interface {
	// Wait blocks until the operation is allowed or the context is canceled.
	// Returns an error if the context is canceled or the wait times out.
	Wait(ctx context.Context) error

	// TryAcquire attempts to acquire permission without blocking.
	// Returns true if permission was granted, false otherwise.
	TryAcquire() bool

	// Limit returns the current rate limit (requests per second).
	Limit() float64
}

// TokenBucketLimiter implements a token bucket rate limiter.
// It allows bursts up to the bucket size and refills at a steady rate.
// TokenBucketLimiter is safe for concurrent use.
type TokenBucketLimiter struct {
	mu           sync.Mutex
	tokens       float64
	maxTokens    float64
	refillRate   float64 // tokens per second
	lastRefill   time.Time
	waitTimeout  time.Duration
}

// TokenBucketConfig configures the token bucket rate limiter.
type TokenBucketConfig struct {
	// Rate is the number of requests allowed per second.
	Rate float64

	// BurstSize is the maximum number of requests that can be made at once.
	// If 0, defaults to Rate.
	BurstSize int

	// WaitTimeout is the maximum time to wait for a token.
	// If 0, defaults to 30 seconds.
	WaitTimeout time.Duration
}

// NewTokenBucketLimiter creates a new token bucket rate limiter.
func NewTokenBucketLimiter(cfg TokenBucketConfig) *TokenBucketLimiter {
	burstSize := cfg.BurstSize
	if burstSize == 0 {
		burstSize = int(cfg.Rate)
		if burstSize < 1 {
			burstSize = 1
		}
	}

	waitTimeout := cfg.WaitTimeout
	if waitTimeout == 0 {
		waitTimeout = 30 * time.Second
	}

	return &TokenBucketLimiter{
		tokens:      float64(burstSize),
		maxTokens:   float64(burstSize),
		refillRate:  cfg.Rate,
		lastRefill:  time.Now(),
		waitTimeout: waitTimeout,
	}
}

// Wait blocks until a token is available or the context is canceled.
func (l *TokenBucketLimiter) Wait(ctx context.Context) error {
	// Create a timeout context
	ctx, cancel := context.WithTimeout(ctx, l.waitTimeout)
	defer cancel()

	for {
		if l.TryAcquire() {
			return nil
		}

		// Calculate wait time until next token
		l.mu.Lock()
		waitTime := time.Duration(float64(time.Second) / l.refillRate)
		l.mu.Unlock()

		// Use a shorter poll interval
		pollInterval := waitTime / 10
		if pollInterval < time.Millisecond {
			pollInterval = time.Millisecond
		}
		if pollInterval > 100*time.Millisecond {
			pollInterval = 100 * time.Millisecond
		}

		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "rate limit wait canceled")
		case <-time.After(pollInterval):
			// Try again
		}
	}
}

// TryAcquire attempts to acquire a token without blocking.
func (l *TokenBucketLimiter) TryAcquire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()

	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}

// Limit returns the rate limit in requests per second.
func (l *TokenBucketLimiter) Limit() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.refillRate
}

// refill adds tokens based on elapsed time. Must be called with lock held.
func (l *TokenBucketLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(l.lastRefill).Seconds()
	l.lastRefill = now

	l.tokens += elapsed * l.refillRate
	if l.tokens > l.maxTokens {
		l.tokens = l.maxTokens
	}
}

// Available returns the current number of available tokens.
func (l *TokenBucketLimiter) Available() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	return l.tokens
}

// SlidingWindowLimiter implements a sliding window rate limiter.
// It tracks requests in a sliding time window for more accurate rate limiting.
// SlidingWindowLimiter is safe for concurrent use.
type SlidingWindowLimiter struct {
	mu          sync.Mutex
	timestamps  []time.Time
	maxRequests int
	window      time.Duration
	waitTimeout time.Duration
}

// SlidingWindowConfig configures the sliding window rate limiter.
type SlidingWindowConfig struct {
	// MaxRequests is the maximum number of requests allowed in the window.
	MaxRequests int

	// Window is the time window for rate limiting.
	Window time.Duration

	// WaitTimeout is the maximum time to wait for permission.
	// If 0, defaults to the window duration.
	WaitTimeout time.Duration
}

// NewSlidingWindowLimiter creates a new sliding window rate limiter.
func NewSlidingWindowLimiter(cfg SlidingWindowConfig) *SlidingWindowLimiter {
	waitTimeout := cfg.WaitTimeout
	if waitTimeout == 0 {
		waitTimeout = cfg.Window
	}

	return &SlidingWindowLimiter{
		timestamps:  make([]time.Time, 0, cfg.MaxRequests),
		maxRequests: cfg.MaxRequests,
		window:      cfg.Window,
		waitTimeout: waitTimeout,
	}
}

// Wait blocks until a request is allowed or the context is canceled.
func (l *SlidingWindowLimiter) Wait(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, l.waitTimeout)
	defer cancel()

	for {
		if l.TryAcquire() {
			return nil
		}

		// Calculate wait time until oldest request expires
		l.mu.Lock()
		var waitTime time.Duration
		if len(l.timestamps) > 0 {
			oldest := l.timestamps[0]
			waitTime = l.window - time.Since(oldest)
			if waitTime < 0 {
				waitTime = time.Millisecond
			}
		} else {
			waitTime = time.Millisecond
		}
		l.mu.Unlock()

		// Cap wait time
		if waitTime > 100*time.Millisecond {
			waitTime = 100 * time.Millisecond
		}

		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "rate limit wait canceled")
		case <-time.After(waitTime):
			// Try again
		}
	}
}

// TryAcquire attempts to acquire permission without blocking.
func (l *SlidingWindowLimiter) TryAcquire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.cleanup()

	if len(l.timestamps) < l.maxRequests {
		l.timestamps = append(l.timestamps, time.Now())
		return true
	}
	return false
}

// Limit returns the rate limit in requests per second.
func (l *SlidingWindowLimiter) Limit() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return float64(l.maxRequests) / l.window.Seconds()
}

// cleanup removes expired timestamps. Must be called with lock held.
func (l *SlidingWindowLimiter) cleanup() {
	cutoff := time.Now().Add(-l.window)
	newStart := 0
	for i, ts := range l.timestamps {
		if ts.After(cutoff) {
			newStart = i
			break
		}
		newStart = i + 1
	}
	if newStart > 0 {
		l.timestamps = l.timestamps[newStart:]
	}
}

// Available returns the number of requests available in the current window.
func (l *SlidingWindowLimiter) Available() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cleanup()
	return l.maxRequests - len(l.timestamps)
}

// MultiLimiter combines multiple rate limiters.
// All limiters must allow the request for it to proceed.
type MultiLimiter struct {
	limiters []RateLimiter
}

// NewMultiLimiter creates a new multi-limiter from multiple limiters.
func NewMultiLimiter(limiters ...RateLimiter) *MultiLimiter {
	return &MultiLimiter{limiters: limiters}
}

// Wait blocks until all limiters allow the request.
func (m *MultiLimiter) Wait(ctx context.Context) error {
	for _, l := range m.limiters {
		if err := l.Wait(ctx); err != nil {
			return err
		}
	}
	return nil
}

// TryAcquire attempts to acquire from all limiters without blocking.
// If any limiter denies, none are consumed.
func (m *MultiLimiter) TryAcquire() bool {
	// First check if all would allow
	for _, l := range m.limiters {
		if !l.TryAcquire() {
			return false
		}
	}
	return true
}

// Limit returns the minimum rate limit across all limiters.
func (m *MultiLimiter) Limit() float64 {
	if len(m.limiters) == 0 {
		return 0
	}
	minLimit := m.limiters[0].Limit()
	for _, l := range m.limiters[1:] {
		if l.Limit() < minLimit {
			minLimit = l.Limit()
		}
	}
	return minLimit
}

// ProviderLimiters manages rate limiters for different providers.
type ProviderLimiters struct {
	mu       sync.RWMutex
	limiters map[string]RateLimiter
	factory  func(provider string) RateLimiter
}

// NewProviderLimiters creates a new provider limiter manager.
// The factory function is called to create a new limiter for each provider.
func NewProviderLimiters(factory func(provider string) RateLimiter) *ProviderLimiters {
	return &ProviderLimiters{
		limiters: make(map[string]RateLimiter),
		factory:  factory,
	}
}

// Get returns the rate limiter for the given provider, creating one if needed.
func (p *ProviderLimiters) Get(provider string) RateLimiter {
	p.mu.RLock()
	l, ok := p.limiters[provider]
	p.mu.RUnlock()

	if ok {
		return l
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if l, ok = p.limiters[provider]; ok {
		return l
	}

	l = p.factory(provider)
	p.limiters[provider] = l
	return l
}

// Wait waits for the rate limiter of the given provider.
func (p *ProviderLimiters) Wait(ctx context.Context, provider string) error {
	return p.Get(provider).Wait(ctx)
}

// DefaultProviderLimits returns default rate limits for common LLM providers.
var DefaultProviderLimits = map[string]TokenBucketConfig{
	"openai":    {Rate: 60, BurstSize: 10},   // 60 RPM, burst of 10
	"anthropic": {Rate: 60, BurstSize: 10},   // 60 RPM, burst of 10
	"cohere":    {Rate: 100, BurstSize: 20},  // 100 RPM, burst of 20
	"google":    {Rate: 60, BurstSize: 10},   // 60 RPM, burst of 10
}

// NewDefaultProviderLimiters creates provider limiters with default configurations.
func NewDefaultProviderLimiters() *ProviderLimiters {
	return NewProviderLimiters(func(provider string) RateLimiter {
		if cfg, ok := DefaultProviderLimits[provider]; ok {
			return NewTokenBucketLimiter(cfg)
		}
		// Default: 30 requests per minute
		return NewTokenBucketLimiter(TokenBucketConfig{Rate: 0.5, BurstSize: 5})
	})
}

// Ensure interfaces are implemented
var (
	_ RateLimiter = (*TokenBucketLimiter)(nil)
	_ RateLimiter = (*SlidingWindowLimiter)(nil)
	_ RateLimiter = (*MultiLimiter)(nil)
)
