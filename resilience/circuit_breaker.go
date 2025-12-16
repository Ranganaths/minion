package resilience

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	// StateClosed means the circuit is closed and requests pass through
	StateClosed CircuitState = iota
	// StateOpen means the circuit is open and requests are blocked
	StateOpen
	// StateHalfOpen means the circuit is testing if the service recovered
	StateHalfOpen
)

// String returns string representation of the state
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

// CircuitBreakerConfig configures a circuit breaker
type CircuitBreakerConfig struct {
	// MaxFailures is the number of failures before opening the circuit
	MaxFailures int

	// Timeout is how long to wait before attempting to close the circuit
	Timeout time.Duration

	// ResetTimeout is how long to wait in half-open state before closing
	ResetTimeout time.Duration

	// OnStateChange is called when the circuit state changes
	OnStateChange func(from, to CircuitState)
}

// DefaultCircuitBreakerConfig returns default configuration
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxFailures:  5,
		Timeout:      10 * time.Second,
		ResetTimeout: 30 * time.Second,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu           sync.RWMutex
	config       *CircuitBreakerConfig
	state        CircuitState
	failureCount int
	lastFailure  time.Time
	lastSuccess  time.Time
	halfOpenTime time.Time
}

// ErrCircuitOpen is returned when the circuit breaker is open
var ErrCircuitOpen = errors.New("circuit breaker is open")

// ErrTooManyRequests is returned when too many requests in half-open state
var ErrTooManyRequests = errors.New("too many requests in half-open state")

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// Execute runs a function through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	// Check if we can proceed
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Execute the function
	err := fn()

	// Record the result
	cb.afterRequest(err)

	return err
}

// beforeRequest checks if the request can proceed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case StateClosed:
		// Allow request
		return nil

	case StateOpen:
		// Check if timeout has elapsed
		if now.Sub(cb.lastFailure) > cb.config.Timeout {
			// Transition to half-open
			cb.setState(StateHalfOpen)
			cb.halfOpenTime = now
			return nil
		}
		// Circuit is still open
		return ErrCircuitOpen

	case StateHalfOpen:
		// In half-open state, allow one request at a time
		// If another request comes in, reject it
		if !cb.halfOpenTime.IsZero() && now.Sub(cb.halfOpenTime) < time.Second {
			return ErrTooManyRequests
		}
		return nil

	default:
		return ErrCircuitOpen
	}
}

// afterRequest records the result of a request
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	if err != nil {
		// Request failed
		cb.failureCount++
		cb.lastFailure = now

		switch cb.state {
		case StateClosed:
			// Check if we should open the circuit
			if cb.failureCount >= cb.config.MaxFailures {
				cb.setState(StateOpen)
			}

		case StateHalfOpen:
			// Failed in half-open state, go back to open
			cb.setState(StateOpen)
		}
	} else {
		// Request succeeded
		cb.lastSuccess = now

		switch cb.state {
		case StateClosed:
			// Reset failure count on success
			if cb.failureCount > 0 {
				cb.failureCount = 0
			}

		case StateHalfOpen:
			// Success in half-open state
			// Check if enough time has passed to close the circuit
			if now.Sub(cb.halfOpenTime) > cb.config.ResetTimeout {
				cb.setState(StateClosed)
				cb.failureCount = 0
			}
		}
	}
}

// setState changes the circuit state and calls the callback
func (cb *CircuitBreaker) setState(newState CircuitState) {
	oldState := cb.state
	cb.state = newState

	if cb.config.OnStateChange != nil && oldState != newState {
		// Call callback without holding lock
		go cb.config.OnStateChange(oldState, newState)
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetFailureCount returns the current failure count
func (cb *CircuitBreaker) GetFailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failureCount
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := cb.state
	cb.state = StateClosed
	cb.failureCount = 0
	cb.lastFailure = time.Time{}
	cb.lastSuccess = time.Time{}
	cb.halfOpenTime = time.Time{}

	if cb.config.OnStateChange != nil && oldState != StateClosed {
		go cb.config.OnStateChange(oldState, StateClosed)
	}
}

// Stats returns statistics about the circuit breaker
type CircuitBreakerStats struct {
	State        CircuitState
	FailureCount int
	LastFailure  time.Time
	LastSuccess  time.Time
}

// GetStats returns current statistics
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		State:        cb.state,
		FailureCount: cb.failureCount,
		LastFailure:  cb.lastFailure,
		LastSuccess:  cb.lastSuccess,
	}
}
