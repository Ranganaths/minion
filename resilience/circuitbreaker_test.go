package resilience

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCircuitBreaker(t *testing.T) {
	t.Run("starts closed", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Name: "test",
		})

		if cb.State() != StateClosed {
			t.Errorf("expected closed, got %s", cb.State())
		}
	})

	t.Run("opens after threshold failures", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: 3,
		})

		ctx := context.Background()
		testErr := errors.New("test error")

		// Record failures
		for i := 0; i < 3; i++ {
			cb.Execute(ctx, func(ctx context.Context) error {
				return testErr
			})
		}

		if cb.State() != StateOpen {
			t.Errorf("expected open, got %s", cb.State())
		}
	})

	t.Run("rejects requests when open", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: 1,
			Timeout:          time.Hour, // Long timeout so it doesn't transition
		})

		ctx := context.Background()

		// Open the circuit
		cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("error")
		})

		// Should reject
		err := cb.Execute(ctx, func(ctx context.Context) error {
			return nil
		})

		if err != ErrCircuitOpen {
			t.Errorf("expected ErrCircuitOpen, got %v", err)
		}

		stats := cb.Stats()
		if stats.TotalRejected != 1 {
			t.Errorf("expected 1 rejected, got %d", stats.TotalRejected)
		}
	})

	t.Run("transitions to half-open after timeout", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: 1,
			Timeout:          50 * time.Millisecond,
		})

		ctx := context.Background()

		// Open the circuit
		cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("error")
		})

		if cb.State() != StateOpen {
			t.Errorf("expected open, got %s", cb.State())
		}

		// Wait for timeout
		time.Sleep(100 * time.Millisecond)

		// Should allow request (transitions to half-open)
		err := cb.Allow()
		if err != nil {
			t.Errorf("expected allow after timeout, got %v", err)
		}

		if cb.State() != StateHalfOpen {
			t.Errorf("expected half-open, got %s", cb.State())
		}
	})

	t.Run("closes after successful requests in half-open", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: 1,
			SuccessThreshold: 2,
			Timeout:          50 * time.Millisecond,
		})

		ctx := context.Background()

		// Open the circuit
		cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("error")
		})

		// Wait for timeout
		time.Sleep(100 * time.Millisecond)

		// Successful requests
		for i := 0; i < 2; i++ {
			cb.Execute(ctx, func(ctx context.Context) error {
				return nil
			})
		}

		if cb.State() != StateClosed {
			t.Errorf("expected closed, got %s", cb.State())
		}
	})

	t.Run("returns to open on failure in half-open", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: 1,
			Timeout:          50 * time.Millisecond,
		})

		ctx := context.Background()

		// Open the circuit
		cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("error")
		})

		// Wait for timeout
		time.Sleep(100 * time.Millisecond)

		// Trigger half-open
		cb.Allow()

		// Fail in half-open
		cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("error")
		})

		if cb.State() != StateOpen {
			t.Errorf("expected open, got %s", cb.State())
		}
	})

	t.Run("resets failure count on success", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: 3,
		})

		ctx := context.Background()

		// 2 failures
		for i := 0; i < 2; i++ {
			cb.Execute(ctx, func(ctx context.Context) error {
				return errors.New("error")
			})
		}

		// 1 success resets counter
		cb.Execute(ctx, func(ctx context.Context) error {
			return nil
		})

		// 2 more failures should not open
		for i := 0; i < 2; i++ {
			cb.Execute(ctx, func(ctx context.Context) error {
				return errors.New("error")
			})
		}

		if cb.State() != StateClosed {
			t.Errorf("expected closed, got %s", cb.State())
		}
	})

	t.Run("callback on state change", func(t *testing.T) {
		var mu sync.Mutex
		var changes []CircuitState
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: 1,
			Timeout:          50 * time.Millisecond,
			OnStateChange: func(name string, from, to CircuitState) {
				mu.Lock()
				changes = append(changes, to)
				mu.Unlock()
			},
		})

		ctx := context.Background()

		// Open
		cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("error")
		})

		// Wait for callback
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		changesCopy := make([]CircuitState, len(changes))
		copy(changesCopy, changes)
		mu.Unlock()

		if len(changesCopy) == 0 || changesCopy[0] != StateOpen {
			t.Errorf("expected callback with StateOpen, got %v", changesCopy)
		}
	})

	t.Run("custom IsFailure", func(t *testing.T) {
		retryableErr := errors.New("retryable")
		permanentErr := errors.New("permanent")

		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: 1,
			IsFailure: func(err error) bool {
				// Only permanent errors count as failures
				return err == permanentErr
			},
		})

		ctx := context.Background()

		// Retryable errors don't count
		cb.Execute(ctx, func(ctx context.Context) error {
			return retryableErr
		})

		if cb.State() != StateClosed {
			t.Errorf("expected closed after retryable error, got %s", cb.State())
		}

		// Permanent error opens
		cb.Execute(ctx, func(ctx context.Context) error {
			return permanentErr
		})

		if cb.State() != StateOpen {
			t.Errorf("expected open after permanent error, got %s", cb.State())
		}
	})

	t.Run("ExecuteWithResult", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Name: "test",
		})

		ctx := context.Background()

		result, err := cb.ExecuteWithResult(ctx, func(ctx context.Context) (any, error) {
			return 42, nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.(int) != 42 {
			t.Errorf("expected 42, got %v", result)
		}
	})

	t.Run("Reset", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: 1,
		})

		ctx := context.Background()

		// Open
		cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("error")
		})

		if cb.State() != StateOpen {
			t.Errorf("expected open, got %s", cb.State())
		}

		cb.Reset()

		if cb.State() != StateClosed {
			t.Errorf("expected closed after reset, got %s", cb.State())
		}
	})

	t.Run("Stats", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: 5,
		})

		ctx := context.Background()

		// Some successes
		for i := 0; i < 3; i++ {
			cb.Execute(ctx, func(ctx context.Context) error {
				return nil
			})
		}

		// Some failures
		for i := 0; i < 2; i++ {
			cb.Execute(ctx, func(ctx context.Context) error {
				return errors.New("error")
			})
		}

		stats := cb.Stats()
		if stats.TotalRequests != 5 {
			t.Errorf("expected 5 total requests, got %d", stats.TotalRequests)
		}
		if stats.TotalSuccesses != 3 {
			t.Errorf("expected 3 successes, got %d", stats.TotalSuccesses)
		}
		if stats.TotalFailures != 2 {
			t.Errorf("expected 2 failures, got %d", stats.TotalFailures)
		}
	})
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tc := range tests {
		if tc.state.String() != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, tc.state.String())
		}
	}
}

func TestCircuitBreakerRegistry(t *testing.T) {
	t.Run("creates breakers on demand", func(t *testing.T) {
		var created atomic.Int32
		registry := NewCircuitBreakerRegistry(func(name string) *CircuitBreaker {
			created.Add(1)
			return NewCircuitBreaker(CircuitBreakerConfig{Name: name})
		})

		registry.Get("service-a")
		registry.Get("service-b")
		registry.Get("service-a") // Should reuse

		if created.Load() != 2 {
			t.Errorf("expected 2 created, got %d", created.Load())
		}
	})

	t.Run("Stats returns all breaker stats", func(t *testing.T) {
		registry := NewDefaultCircuitBreakerRegistry()

		registry.Get("service-a")
		registry.Get("service-b")

		stats := registry.Stats()
		if len(stats) != 2 {
			t.Errorf("expected 2 stats, got %d", len(stats))
		}
	})
}

func TestDo(t *testing.T) {
	t.Run("with rate limiter and circuit breaker", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(TokenBucketConfig{Rate: 100, BurstSize: 10})
		cb := NewCircuitBreaker(CircuitBreakerConfig{Name: "test"})

		ctx := context.Background()
		var called bool

		err := Do(ctx, limiter, cb, func(ctx context.Context) error {
			called = true
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !called {
			t.Error("function was not called")
		}
	})

	t.Run("nil limiter and circuit breaker", func(t *testing.T) {
		ctx := context.Background()
		var called bool

		err := Do(ctx, nil, nil, func(ctx context.Context) error {
			called = true
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !called {
			t.Error("function was not called")
		}
	})
}

func TestDoWithResult(t *testing.T) {
	limiter := NewTokenBucketLimiter(TokenBucketConfig{Rate: 100, BurstSize: 10})
	cb := NewCircuitBreaker(CircuitBreakerConfig{Name: "test"})

	ctx := context.Background()

	result, err := DoWithResult(ctx, limiter, cb, func(ctx context.Context) (string, error) {
		return "hello", nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got '%s'", result)
	}
}
