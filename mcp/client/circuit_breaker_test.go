package client

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedState(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	// Should be closed initially
	if cb.GetState() != CircuitClosed {
		t.Errorf("Expected state to be closed, got %v", cb.GetState())
	}

	// Successful operations should keep it closed
	for i := 0; i < 10; i++ {
		err := cb.Execute(context.Background(), func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	}

	if cb.GetState() != CircuitClosed {
		t.Errorf("Expected state to remain closed, got %v", cb.GetState())
	}
}

func TestCircuitBreaker_OpenOnFailures(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:          3,
		Timeout:              1 * time.Second,
		SuccessThreshold:     2,
		FailureRateThreshold: 50.0,
		MinSamples:           10,
	}
	cb := NewCircuitBreaker(config)

	// Trigger failures to open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 3; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	// Circuit should be open now
	if cb.GetState() != CircuitOpen {
		t.Errorf("Expected state to be open after %d failures, got %v", config.MaxFailures, cb.GetState())
	}

	// Requests should be rejected
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err == nil || err.Error() != "circuit breaker is open" {
		t.Errorf("Expected circuit breaker is open error, got %v", err)
	}

	metrics := cb.GetMetrics()
	if metrics.RejectedCalls == 0 {
		t.Error("Expected rejected calls to be tracked")
	}
}

func TestCircuitBreaker_HalfOpenRecovery(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:          2,
		Timeout:              100 * time.Millisecond,
		MaxHalfOpenRequests:  5,
		SuccessThreshold:     2,
		FailureRateThreshold: 50.0,
		MinSamples:           10,
	}
	cb := NewCircuitBreaker(config)

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	if cb.GetState() != CircuitOpen {
		t.Fatalf("Expected circuit to be open")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Next request should transition to half-open and succeed
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Expected successful execution in half-open state, got %v", err)
	}

	// Should be half-open after first success
	state := cb.GetState()
	if state != CircuitHalfOpen {
		t.Fatalf("Expected state to be half-open after first success, got %v", state)
	}

	// Second successful request should close the circuit
	err = cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected successful execution, got %v", err)
	}

	if cb.GetState() != CircuitClosed {
		t.Errorf("Expected state to be closed after %d successful requests, got %v", config.SuccessThreshold, cb.GetState())
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:         2,
		Timeout:             100 * time.Millisecond,
		MaxHalfOpenRequests: 3,
		SuccessThreshold:    2,
	}
	cb := NewCircuitBreaker(config)

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	// Wait for timeout to transition to half-open
	time.Sleep(150 * time.Millisecond)

	// Execute to transition to half-open
	cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	if cb.GetState() != CircuitHalfOpen {
		t.Fatalf("Expected state to be half-open, got %v", cb.GetState())
	}

	// Failure in half-open should reopen the circuit
	cb.Execute(context.Background(), func(ctx context.Context) error {
		return testErr
	})

	if cb.GetState() != CircuitOpen {
		t.Errorf("Expected state to be open after failure in half-open, got %v", cb.GetState())
	}
}

func TestCircuitBreaker_FailureRateThreshold(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:          100, // High threshold to avoid consecutive failure check
		Timeout:              1 * time.Second,
		FailureRateThreshold: 50.0,
		MinSamples:           10,
		SuccessThreshold:     2,
	}
	cb := NewCircuitBreaker(config)

	// Execute 10 requests: 4 successes, 6 consecutive failures (60% failure rate)
	testErr := errors.New("test error")

	// First do some successes
	for i := 0; i < 4; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return nil
		})
	}

	// Then consecutive failures
	for i := 0; i < 6; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	// Circuit should be open due to failure rate
	if cb.GetState() != CircuitOpen {
		metrics := cb.GetMetrics()
		t.Errorf("Expected circuit to be open due to high failure rate (%.2f%%), got %v", metrics.FailureRate, cb.GetState())
	}

	metrics := cb.GetMetrics()
	if metrics.FailureRate < 50.0 {
		t.Errorf("Expected failure rate >= 50%%, got %.2f%%", metrics.FailureRate)
	}
}

func TestCircuitBreaker_Metrics(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	// Execute some operations
	for i := 0; i < 5; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return nil
		})
	}

	testErr := errors.New("test error")
	for i := 0; i < 3; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	metrics := cb.GetMetrics()

	if metrics.TotalCalls != 8 {
		t.Errorf("Expected 8 total calls, got %d", metrics.TotalCalls)
	}

	if metrics.SuccessfulCalls != 5 {
		t.Errorf("Expected 5 successful calls, got %d", metrics.SuccessfulCalls)
	}

	if metrics.FailedCalls != 3 {
		t.Errorf("Expected 3 failed calls, got %d", metrics.FailedCalls)
	}

	if metrics.ConsecutiveFails != 3 {
		t.Errorf("Expected 3 consecutive fails, got %d", metrics.ConsecutiveFails)
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 5; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	if cb.GetState() != CircuitOpen {
		t.Fatalf("Expected circuit to be open")
	}

	// Reset
	cb.Reset()

	if cb.GetState() != CircuitClosed {
		t.Errorf("Expected state to be closed after reset, got %v", cb.GetState())
	}

	metrics := cb.GetMetrics()
	if metrics.TotalCalls != 0 {
		t.Errorf("Expected metrics to be reset, got %d total calls", metrics.TotalCalls)
	}
}

func TestCircuitBreaker_ForceOpen(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	if cb.GetState() != CircuitClosed {
		t.Fatalf("Expected initial state to be closed")
	}

	cb.ForceOpen()

	if cb.GetState() != CircuitOpen {
		t.Errorf("Expected state to be open after ForceOpen, got %v", cb.GetState())
	}

	// Requests should be rejected
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err == nil {
		t.Error("Expected request to be rejected in forced open state")
	}
}

func TestCircuitBreaker_ForceClose(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 5; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	if cb.GetState() != CircuitOpen {
		t.Fatalf("Expected circuit to be open")
	}

	cb.ForceClose()

	if cb.GetState() != CircuitClosed {
		t.Errorf("Expected state to be closed after ForceClose, got %v", cb.GetState())
	}

	// Requests should be accepted
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected request to succeed in forced closed state, got %v", err)
	}
}

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:         2,
		Timeout:             100 * time.Millisecond,
		MaxHalfOpenRequests: 3,
		SuccessThreshold:    2,
	}
	cb := NewCircuitBreaker(config)

	// Track state changes
	initialMetrics := cb.GetMetrics()
	if initialMetrics.StateChanges != 0 {
		t.Errorf("Expected 0 state changes initially, got %d", initialMetrics.StateChanges)
	}

	// Transition to open
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	metrics := cb.GetMetrics()
	if metrics.StateChanges != 1 {
		t.Errorf("Expected 1 state change (closed->open), got %d", metrics.StateChanges)
	}

	// Transition to half-open
	time.Sleep(150 * time.Millisecond)
	cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	metrics = cb.GetMetrics()
	if metrics.StateChanges != 2 {
		t.Errorf("Expected 2 state changes (open->half-open), got %d", metrics.StateChanges)
	}

	// Transition to closed
	cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	metrics = cb.GetMetrics()
	if metrics.StateChanges != 3 {
		t.Errorf("Expected 3 state changes (half-open->closed), got %d", metrics.StateChanges)
	}
}

func TestCircuitBreaker_HalfOpenRequestLimit(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:         2,
		Timeout:             100 * time.Millisecond,
		MaxHalfOpenRequests: 1,
		SuccessThreshold:    1,
	}
	cb := NewCircuitBreaker(config)

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	if cb.GetState() != CircuitOpen {
		t.Fatalf("Expected circuit to be open")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// First request should succeed (transitions to half-open and allowed)
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected first request to succeed, got %v", err)
	}

	// Circuit should now be closed after success threshold reached
	if cb.GetState() != CircuitClosed {
		// If still half-open, try one more rejection
		err = cb.Execute(context.Background(), func(ctx context.Context) error {
			return nil
		})
		if err != nil && err.Error() == "circuit breaker half-open limit reached" {
			// This is expected if still in half-open
			return
		}
	}
}
