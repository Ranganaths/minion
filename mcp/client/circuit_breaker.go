package client

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation
	CircuitOpen                          // Failing, reject requests
	CircuitHalfOpen                      // Testing if service recovered
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config  *CircuitBreakerConfig
	state   CircuitState
	mu      sync.RWMutex
	metrics *circuitMetrics

	// State tracking
	lastStateChange time.Time
	consecutiveFails int
	consecutiveSuccesses int
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	MaxFailures          int           // Failures before opening
	Timeout              time.Duration // Time to wait before half-open
	MaxHalfOpenRequests  int           // Max requests in half-open
	SuccessThreshold     int           // Successes to close from half-open
	FailureRateThreshold float64       // Failure rate % to open (0-100)
	MinSamples           int           // Min samples before checking failure rate
}

// DefaultCircuitBreakerConfig returns default configuration
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxFailures:          5,
		Timeout:              30 * time.Second,
		MaxHalfOpenRequests:  3,
		SuccessThreshold:     2,
		FailureRateThreshold: 50.0,
		MinSamples:           10,
	}
}

// circuitMetrics tracks circuit breaker metrics
type circuitMetrics struct {
	totalCalls       int64
	successfulCalls  int64
	failedCalls      int64
	rejectedCalls    int64
	stateChanges     int64
	lastFailureTime  time.Time
	lastSuccessTime  time.Time
	mu               sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	return &CircuitBreaker{
		config:          config,
		state:           CircuitClosed,
		lastStateChange: time.Now(),
		metrics:         &circuitMetrics{},
	}
}

// Execute runs an operation through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, operation func(ctx context.Context) error) error {
	// Check if circuit allows execution
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Execute operation
	err := operation(ctx)

	// Record result
	cb.afterRequest(err)

	return err
}

// beforeRequest checks if request should be allowed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.RLock()
	state := cb.state
	cb.mu.RUnlock()

	switch state {
	case CircuitClosed:
		// Allow request
		cb.recordCall()
		return nil

	case CircuitOpen:
		// Check if timeout has passed
		cb.mu.RLock()
		timeSinceOpen := time.Since(cb.lastStateChange)
		cb.mu.RUnlock()

		if timeSinceOpen >= cb.config.Timeout {
			// Transition to half-open
			cb.mu.Lock()
			cb.state = CircuitHalfOpen
			cb.consecutiveSuccesses = 0
			cb.lastStateChange = time.Now()
			cb.metrics.stateChanges++
			cb.mu.Unlock()

			cb.recordCall()
			return nil
		}

		// Circuit is open, reject request
		cb.recordRejection()
		return fmt.Errorf("circuit breaker is open")

	case CircuitHalfOpen:
		// Allow limited requests in half-open state
		cb.mu.RLock()
		halfOpenRequests := int(cb.metrics.totalCalls) - int(cb.metrics.rejectedCalls)
		cb.mu.RUnlock()

		if halfOpenRequests < cb.config.MaxHalfOpenRequests {
			cb.recordCall()
			return nil
		}

		// Too many half-open requests, reject
		cb.recordRejection()
		return fmt.Errorf("circuit breaker half-open limit reached")

	default:
		return fmt.Errorf("unknown circuit breaker state")
	}
}

// afterRequest records the result of a request
func (cb *CircuitBreaker) afterRequest(err error) {
	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onSuccess handles successful request
func (cb *CircuitBreaker) onSuccess() {
	cb.metrics.mu.Lock()
	cb.metrics.successfulCalls++
	cb.metrics.lastSuccessTime = time.Now()
	cb.metrics.mu.Unlock()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFails = 0
	cb.consecutiveSuccesses++

	// State transitions
	switch cb.state {
	case CircuitHalfOpen:
		// Check if we can close the circuit
		if cb.consecutiveSuccesses >= cb.config.SuccessThreshold {
			cb.state = CircuitClosed
			cb.lastStateChange = time.Now()
			cb.consecutiveSuccesses = 0
			cb.metrics.stateChanges++
		}

	case CircuitClosed:
		// Already closed, nothing to do
	}
}

// onFailure handles failed request
func (cb *CircuitBreaker) onFailure() {
	cb.metrics.mu.Lock()
	cb.metrics.failedCalls++
	cb.metrics.lastFailureTime = time.Now()
	cb.metrics.mu.Unlock()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveSuccesses = 0
	cb.consecutiveFails++

	// State transitions
	switch cb.state {
	case CircuitClosed:
		// Check if we should open
		if cb.shouldOpen() {
			cb.state = CircuitOpen
			cb.lastStateChange = time.Now()
			cb.metrics.stateChanges++
		}

	case CircuitHalfOpen:
		// Single failure in half-open reopens the circuit
		cb.state = CircuitOpen
		cb.lastStateChange = time.Now()
		cb.consecutiveFails = 0
		cb.metrics.stateChanges++
	}
}

// shouldOpen determines if circuit should open
func (cb *CircuitBreaker) shouldOpen() bool {
	// Check consecutive failures
	if cb.consecutiveFails >= cb.config.MaxFailures {
		return true
	}

	// Check failure rate
	cb.metrics.mu.RLock()
	total := cb.metrics.totalCalls
	failed := cb.metrics.failedCalls
	cb.metrics.mu.RUnlock()

	if total >= int64(cb.config.MinSamples) {
		failureRate := float64(failed) / float64(total) * 100
		if failureRate >= cb.config.FailureRateThreshold {
			return true
		}
	}

	return false
}

// GetState returns current circuit state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetMetrics returns circuit breaker metrics
func (cb *CircuitBreaker) GetMetrics() CircuitBreakerMetrics {
	cb.metrics.mu.RLock()
	defer cb.metrics.mu.RUnlock()

	cb.mu.RLock()
	state := cb.state
	consecutiveFails := cb.consecutiveFails
	consecutiveSuccesses := cb.consecutiveSuccesses
	cb.mu.RUnlock()

	failureRate := 0.0
	if cb.metrics.totalCalls > 0 {
		failureRate = float64(cb.metrics.failedCalls) / float64(cb.metrics.totalCalls) * 100
	}

	return CircuitBreakerMetrics{
		State:                state.String(),
		TotalCalls:           cb.metrics.totalCalls,
		SuccessfulCalls:      cb.metrics.successfulCalls,
		FailedCalls:          cb.metrics.failedCalls,
		RejectedCalls:        cb.metrics.rejectedCalls,
		StateChanges:         cb.metrics.stateChanges,
		ConsecutiveFails:     consecutiveFails,
		ConsecutiveSuccesses: consecutiveSuccesses,
		FailureRate:          failureRate,
		LastFailure:          cb.metrics.lastFailureTime,
		LastSuccess:          cb.metrics.lastSuccessTime,
	}
}

// CircuitBreakerMetrics contains circuit breaker statistics
type CircuitBreakerMetrics struct {
	State                string
	TotalCalls           int64
	SuccessfulCalls      int64
	FailedCalls          int64
	RejectedCalls        int64
	StateChanges         int64
	ConsecutiveFails     int
	ConsecutiveSuccesses int
	FailureRate          float64
	LastFailure          time.Time
	LastSuccess          time.Time
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.consecutiveFails = 0
	cb.consecutiveSuccesses = 0
	cb.lastStateChange = time.Now()

	cb.metrics.mu.Lock()
	cb.metrics.totalCalls = 0
	cb.metrics.successfulCalls = 0
	cb.metrics.failedCalls = 0
	cb.metrics.rejectedCalls = 0
	cb.metrics.stateChanges = 0
	cb.metrics.mu.Unlock()
}

// ForceOpen forces the circuit to open state
func (cb *CircuitBreaker) ForceOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state != CircuitOpen {
		cb.state = CircuitOpen
		cb.lastStateChange = time.Now()
		cb.metrics.stateChanges++
	}
}

// ForceClose forces the circuit to closed state
func (cb *CircuitBreaker) ForceClose() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state != CircuitClosed {
		cb.state = CircuitClosed
		cb.consecutiveFails = 0
		cb.consecutiveSuccesses = 0
		cb.lastStateChange = time.Now()
		cb.metrics.stateChanges++
	}
}

// IsOpen returns true if circuit is open
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == CircuitOpen
}

// IsClosed returns true if circuit is closed
func (cb *CircuitBreaker) IsClosed() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == CircuitClosed
}

// IsHalfOpen returns true if circuit is half-open
func (cb *CircuitBreaker) IsHalfOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == CircuitHalfOpen
}

// recordCall increments total call counter
func (cb *CircuitBreaker) recordCall() {
	cb.metrics.mu.Lock()
	cb.metrics.totalCalls++
	cb.metrics.mu.Unlock()
}

// recordRejection increments rejection counter
func (cb *CircuitBreaker) recordRejection() {
	cb.metrics.mu.Lock()
	cb.metrics.rejectedCalls++
	cb.metrics.mu.Unlock()
}

// GetTimeSinceStateChange returns duration since last state change
func (cb *CircuitBreaker) GetTimeSinceStateChange() time.Duration {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return time.Since(cb.lastStateChange)
}
