package resilience

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitState represents the state of the circuit breaker.
type CircuitState int32

const (
	// StateClosed means the circuit is operating normally.
	StateClosed CircuitState = iota
	// StateOpen means the circuit is open and blocking requests.
	StateOpen
	// StateHalfOpen means the circuit is testing if the service has recovered.
	StateHalfOpen
)

// String returns the string representation of the state.
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// ErrTooManyRequests is returned when too many requests are made in half-open state.
var ErrTooManyRequests = errors.New("too many requests in half-open state")

// CircuitBreaker implements the circuit breaker pattern.
// It prevents cascading failures by temporarily blocking requests to a failing service.
// CircuitBreaker is safe for concurrent use.
type CircuitBreaker struct {
	name string
	cfg  CircuitBreakerConfig

	mu              sync.RWMutex
	state           CircuitState
	failures        int
	successes       int
	lastFailureTime time.Time
	lastStateChange time.Time

	// Metrics
	totalRequests   atomic.Int64
	totalFailures   atomic.Int64
	totalSuccesses  atomic.Int64
	totalRejected   atomic.Int64

	// Callbacks
	onStateChange func(name string, from, to CircuitState)
}

// CircuitBreakerConfig configures the circuit breaker.
type CircuitBreakerConfig struct {
	// Name identifies the circuit breaker (for logging/metrics).
	Name string

	// FailureThreshold is the number of failures before opening the circuit.
	// Default: 5
	FailureThreshold int

	// SuccessThreshold is the number of successes in half-open state before closing.
	// Default: 2
	SuccessThreshold int

	// Timeout is the duration the circuit stays open before transitioning to half-open.
	// Default: 30 seconds
	Timeout time.Duration

	// OnStateChange is called when the circuit state changes.
	OnStateChange func(name string, from, to CircuitState)

	// IsFailure determines if an error should count as a failure.
	// If nil, all non-nil errors count as failures.
	IsFailure func(error) bool
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.IsFailure == nil {
		cfg.IsFailure = func(err error) bool { return err != nil }
	}

	return &CircuitBreaker{
		name:            cfg.Name,
		cfg:             cfg,
		state:           StateClosed,
		lastStateChange: time.Now(),
		onStateChange:   cfg.OnStateChange,
	}
}

// Execute runs the given function through the circuit breaker.
// If the circuit is open, ErrCircuitOpen is returned immediately.
// If the function returns an error (and IsFailure returns true), it counts as a failure.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := cb.Allow(); err != nil {
		return err
	}

	cb.totalRequests.Add(1)

	err := fn(ctx)
	cb.Record(err)
	return err
}

// ExecuteWithResult runs a function that returns a value through the circuit breaker.
func (cb *CircuitBreaker) ExecuteWithResult(ctx context.Context, fn func(ctx context.Context) (any, error)) (any, error) {
	if err := cb.Allow(); err != nil {
		return nil, err
	}

	cb.totalRequests.Add(1)

	result, err := fn(ctx)
	cb.Record(err)
	return result, err
}

// Allow checks if a request should be allowed through.
// Returns ErrCircuitOpen if the circuit is open.
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case StateClosed:
		return nil

	case StateOpen:
		// Check if timeout has elapsed
		if now.Sub(cb.lastStateChange) >= cb.cfg.Timeout {
			cb.transitionTo(StateHalfOpen)
			return nil
		}
		cb.totalRejected.Add(1)
		return ErrCircuitOpen

	case StateHalfOpen:
		// Allow limited requests through
		return nil
	}

	return nil
}

// Record records the result of a request.
func (cb *CircuitBreaker) Record(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	isFailure := cb.cfg.IsFailure(err)

	if isFailure {
		cb.totalFailures.Add(1)
		cb.recordFailure()
	} else {
		cb.totalSuccesses.Add(1)
		cb.recordSuccess()
	}
}

// recordFailure handles a failure. Must be called with lock held.
func (cb *CircuitBreaker) recordFailure() {
	cb.failures++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.cfg.FailureThreshold {
			cb.transitionTo(StateOpen)
		}

	case StateHalfOpen:
		// Any failure in half-open returns to open
		cb.transitionTo(StateOpen)
	}
}

// recordSuccess handles a success. Must be called with lock held.
func (cb *CircuitBreaker) recordSuccess() {
	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		cb.failures = 0

	case StateHalfOpen:
		cb.successes++
		if cb.successes >= cb.cfg.SuccessThreshold {
			cb.transitionTo(StateClosed)
		}
	}
}

// transitionTo changes the circuit state. Must be called with lock held.
func (cb *CircuitBreaker) transitionTo(newState CircuitState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()
	cb.failures = 0
	cb.successes = 0

	if cb.onStateChange != nil {
		// Call callback in goroutine to avoid blocking
		go cb.onStateChange(cb.name, oldState, newState)
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns circuit breaker statistics.
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		Name:           cb.name,
		State:          cb.state,
		Failures:       cb.failures,
		Successes:      cb.successes,
		TotalRequests:  cb.totalRequests.Load(),
		TotalFailures:  cb.totalFailures.Load(),
		TotalSuccesses: cb.totalSuccesses.Load(),
		TotalRejected:  cb.totalRejected.Load(),
		LastFailure:    cb.lastFailureTime,
		LastStateChange: cb.lastStateChange,
	}
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionTo(StateClosed)
}

// CircuitBreakerStats contains circuit breaker statistics.
type CircuitBreakerStats struct {
	Name            string
	State           CircuitState
	Failures        int
	Successes       int
	TotalRequests   int64
	TotalFailures   int64
	TotalSuccesses  int64
	TotalRejected   int64
	LastFailure     time.Time
	LastStateChange time.Time
}

// CircuitBreakerRegistry manages multiple circuit breakers.
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	factory  func(name string) *CircuitBreaker
}

// NewCircuitBreakerRegistry creates a new registry with a factory function.
func NewCircuitBreakerRegistry(factory func(name string) *CircuitBreaker) *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		factory:  factory,
	}
}

// Get returns the circuit breaker for the given name, creating one if needed.
func (r *CircuitBreakerRegistry) Get(name string) *CircuitBreaker {
	r.mu.RLock()
	cb, ok := r.breakers[name]
	r.mu.RUnlock()

	if ok {
		return cb
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check
	if cb, ok = r.breakers[name]; ok {
		return cb
	}

	cb = r.factory(name)
	r.breakers[name] = cb
	return cb
}

// Stats returns stats for all circuit breakers.
func (r *CircuitBreakerRegistry) Stats() map[string]CircuitBreakerStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]CircuitBreakerStats, len(r.breakers))
	for name, cb := range r.breakers {
		stats[name] = cb.Stats()
	}
	return stats
}

// NewDefaultCircuitBreakerRegistry creates a registry with default configuration.
func NewDefaultCircuitBreakerRegistry() *CircuitBreakerRegistry {
	return NewCircuitBreakerRegistry(func(name string) *CircuitBreaker {
		return NewCircuitBreaker(CircuitBreakerConfig{
			Name:             name,
			FailureThreshold: 5,
			SuccessThreshold: 2,
			Timeout:          30 * time.Second,
		})
	})
}

// Do executes a function with both rate limiting and circuit breaking.
// This is a convenience function for common use cases.
func Do(ctx context.Context, limiter RateLimiter, cb *CircuitBreaker, fn func(ctx context.Context) error) error {
	// Wait for rate limit
	if limiter != nil {
		if err := limiter.Wait(ctx); err != nil {
			return err
		}
	}

	// Execute through circuit breaker
	if cb != nil {
		return cb.Execute(ctx, fn)
	}

	return fn(ctx)
}

// DoWithResult executes a function that returns a value with rate limiting and circuit breaking.
func DoWithResult[T any](ctx context.Context, limiter RateLimiter, cb *CircuitBreaker, fn func(ctx context.Context) (T, error)) (T, error) {
	var zero T

	// Wait for rate limit
	if limiter != nil {
		if err := limiter.Wait(ctx); err != nil {
			return zero, err
		}
	}

	// Execute through circuit breaker
	if cb != nil {
		result, err := cb.ExecuteWithResult(ctx, func(ctx context.Context) (any, error) {
			return fn(ctx)
		})
		if err != nil {
			return zero, err
		}
		return result.(T), nil
	}

	return fn(ctx)
}
