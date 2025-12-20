package retry

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	minerrors "github.com/Ranganaths/minion/errors"
)

func TestDo(t *testing.T) {
	t.Run("success on first try", func(t *testing.T) {
		attempts := 0
		result, err := Do(context.Background(), func() (string, error) {
			attempts++
			return "success", nil
		})

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if result != "success" {
			t.Errorf("expected 'success', got %q", result)
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("success after retries", func(t *testing.T) {
		attempts := 0
		result, err := Do(context.Background(), func() (string, error) {
			attempts++
			if attempts < 3 {
				return "", minerrors.NewRetryableError(errors.New("transient"), 3)
			}
			return "success", nil
		}, WithMaxRetries(5), WithInitialDelay(time.Millisecond))

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if result != "success" {
			t.Errorf("expected 'success', got %q", result)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		attempts := 0
		_, err := Do(context.Background(), func() (string, error) {
			attempts++
			return "", minerrors.NewRetryableError(errors.New("transient"), 5)
		}, WithMaxRetries(3), WithInitialDelay(time.Millisecond))

		if err == nil {
			t.Error("expected error, got nil")
		}
		if attempts != 4 { // initial + 3 retries
			t.Errorf("expected 4 attempts, got %d", attempts)
		}
	})

	t.Run("non-retryable error", func(t *testing.T) {
		attempts := 0
		_, err := Do(context.Background(), func() (string, error) {
			attempts++
			return "", errors.New("permanent error")
		}, WithMaxRetries(3), WithInitialDelay(time.Millisecond))

		if err == nil {
			t.Error("expected error, got nil")
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt for non-retryable error, got %d", attempts)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0

		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		_, err := Do(ctx, func() (string, error) {
			attempts++
			return "", minerrors.NewRetryableError(errors.New("transient"), 10)
		}, WithMaxRetries(10), WithInitialDelay(50*time.Millisecond))

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("retry always", func(t *testing.T) {
		attempts := 0
		_, err := Do(context.Background(), func() (string, error) {
			attempts++
			return "", errors.New("any error")
		}, WithMaxRetries(2), WithInitialDelay(time.Millisecond), WithRetryIf(RetryAlways))

		if err == nil {
			t.Error("expected error, got nil")
		}
		if attempts != 3 { // initial + 2 retries
			t.Errorf("expected 3 attempts with RetryAlways, got %d", attempts)
		}
	})

	t.Run("retry never", func(t *testing.T) {
		attempts := 0
		_, err := Do(context.Background(), func() (string, error) {
			attempts++
			return "", minerrors.NewRetryableError(errors.New("transient"), 10)
		}, WithMaxRetries(5), WithRetryIf(RetryNever))

		if err == nil {
			t.Error("expected error, got nil")
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt with RetryNever, got %d", attempts)
		}
	})
}

func TestDoVoid(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		attempts := 0
		err := DoVoid(context.Background(), func() error {
			attempts++
			return nil
		})

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("with retries", func(t *testing.T) {
		attempts := 0
		err := DoVoid(context.Background(), func() error {
			attempts++
			if attempts < 2 {
				return minerrors.NewRetryableError(errors.New("transient"), 3)
			}
			return nil
		}, WithMaxRetries(3), WithInitialDelay(time.Millisecond))

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})
}

func TestDoWithCallback(t *testing.T) {
	t.Run("callback called on retries", func(t *testing.T) {
		attempts := 0
		callbackCount := 0
		var callbackAttempts []int
		var callbackDelays []time.Duration

		_, err := DoWithCallback(context.Background(), func() (string, error) {
			attempts++
			if attempts < 3 {
				return "", minerrors.NewRetryableError(errors.New("transient"), 5)
			}
			return "success", nil
		}, func(attempt int, err error, delay time.Duration) {
			callbackCount++
			callbackAttempts = append(callbackAttempts, attempt)
			callbackDelays = append(callbackDelays, delay)
		}, WithMaxRetries(5), WithInitialDelay(time.Millisecond))

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if callbackCount != 2 {
			t.Errorf("expected 2 callbacks, got %d", callbackCount)
		}
		if len(callbackAttempts) != 2 || callbackAttempts[0] != 1 || callbackAttempts[1] != 2 {
			t.Errorf("unexpected callback attempts: %v", callbackAttempts)
		}
	})

	t.Run("nil callback is safe", func(t *testing.T) {
		_, err := DoWithCallback(context.Background(), func() (string, error) {
			return "", minerrors.NewRetryableError(errors.New("transient"), 3)
		}, nil, WithMaxRetries(1), WithInitialDelay(time.Millisecond))

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestBackoff(t *testing.T) {
	t.Run("exponential backoff", func(t *testing.T) {
		b := NewBackoff(
			WithInitialDelay(100*time.Millisecond),
			WithMultiplier(2.0),
			WithMaxDelay(10*time.Second),
			WithJitter(0),
		)

		d1 := b.Next()
		d2 := b.Next()
		d3 := b.Next()

		if d1 != 100*time.Millisecond {
			t.Errorf("expected first delay 100ms, got %v", d1)
		}
		if d2 != 200*time.Millisecond {
			t.Errorf("expected second delay 200ms, got %v", d2)
		}
		if d3 != 400*time.Millisecond {
			t.Errorf("expected third delay 400ms, got %v", d3)
		}
	})

	t.Run("max delay cap", func(t *testing.T) {
		b := NewBackoff(
			WithInitialDelay(100*time.Millisecond),
			WithMultiplier(10.0),
			WithMaxDelay(500*time.Millisecond),
			WithJitter(0),
		)

		b.Next() // 100ms
		b.Next() // 1000ms -> capped to 500ms
		d := b.Next()

		if d != 500*time.Millisecond {
			t.Errorf("expected delay capped at 500ms, got %v", d)
		}
	})

	t.Run("reset", func(t *testing.T) {
		b := NewBackoff(WithInitialDelay(100*time.Millisecond), WithJitter(0))

		b.Next()
		b.Next()
		if b.Attempt() != 2 {
			t.Errorf("expected attempt 2, got %d", b.Attempt())
		}

		b.Reset()
		if b.Attempt() != 0 {
			t.Errorf("expected attempt 0 after reset, got %d", b.Attempt())
		}

		d := b.Next()
		if d != 100*time.Millisecond {
			t.Errorf("expected delay reset to 100ms, got %v", d)
		}
	})

	t.Run("exhausted", func(t *testing.T) {
		b := NewBackoff(WithMaxRetries(2))

		if b.Exhausted() {
			t.Error("should not be exhausted initially")
		}

		b.Next() // attempt 0
		b.Next() // attempt 1
		b.Next() // attempt 2

		if !b.Exhausted() {
			t.Error("should be exhausted after max retries")
		}
	})

	t.Run("wait with context", func(t *testing.T) {
		b := NewBackoff(WithInitialDelay(time.Millisecond))

		ctx := context.Background()
		start := time.Now()
		err := b.Wait(ctx)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if elapsed < time.Millisecond {
			t.Error("expected wait to take at least 1ms")
		}
	})

	t.Run("wait with canceled context", func(t *testing.T) {
		b := NewBackoff(WithInitialDelay(10 * time.Second))

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := b.Wait(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

func TestWithRetry(t *testing.T) {
	attempts := int32(0)
	w := NewWithRetry(func() (string, error) {
		if atomic.AddInt32(&attempts, 1) < 2 {
			return "", minerrors.NewRetryableError(errors.New("transient"), 3)
		}
		return "success", nil
	}, WithMaxRetries(3), WithInitialDelay(time.Millisecond))

	result, err := w.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got %q", result)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.MaxRetries)
	}
	if cfg.InitialDelay != 100*time.Millisecond {
		t.Errorf("expected InitialDelay 100ms, got %v", cfg.InitialDelay)
	}
	if cfg.MaxDelay != 30*time.Second {
		t.Errorf("expected MaxDelay 30s, got %v", cfg.MaxDelay)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("expected Multiplier 2.0, got %f", cfg.Multiplier)
	}
	if cfg.Jitter != 0.1 {
		t.Errorf("expected Jitter 0.1, got %f", cfg.Jitter)
	}
	if cfg.RetryIf == nil {
		t.Error("expected RetryIf to be set")
	}
}

func TestDefaultRetryIf(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"retryable error", minerrors.NewRetryableError(errors.New("test"), 3), true},
		{"rate limited", minerrors.Wrap(minerrors.ErrRateLimited, "context"), true},
		{"timeout", minerrors.Wrap(minerrors.ErrTimeout, "context"), true},
		{"regular error", errors.New("regular"), false},
		{"auth failed", minerrors.Wrap(minerrors.ErrAuthFailed, "context"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DefaultRetryIf(tc.err)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestJitter(t *testing.T) {
	// Run multiple times to test jitter randomness
	b := NewBackoff(
		WithInitialDelay(100*time.Millisecond),
		WithJitter(0.5),
	)

	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		b.Reset()
		delays[i] = b.Next()
	}

	// Check that at least some delays are different (jitter is random)
	allSame := true
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("expected jitter to produce varying delays")
	}

	// Check that all delays are within expected range (100ms +/- 50%)
	for i, d := range delays {
		if d < 50*time.Millisecond || d > 150*time.Millisecond {
			t.Errorf("delay %d out of expected range: %v", i, d)
		}
	}
}

func TestOptions(t *testing.T) {
	cfg := Config{}

	WithMaxRetries(5)(&cfg)
	if cfg.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", cfg.MaxRetries)
	}

	WithInitialDelay(500 * time.Millisecond)(&cfg)
	if cfg.InitialDelay != 500*time.Millisecond {
		t.Errorf("expected InitialDelay 500ms, got %v", cfg.InitialDelay)
	}

	WithMaxDelay(1 * time.Minute)(&cfg)
	if cfg.MaxDelay != time.Minute {
		t.Errorf("expected MaxDelay 1m, got %v", cfg.MaxDelay)
	}

	WithMultiplier(3.0)(&cfg)
	if cfg.Multiplier != 3.0 {
		t.Errorf("expected Multiplier 3.0, got %f", cfg.Multiplier)
	}

	WithJitter(0.25)(&cfg)
	if cfg.Jitter != 0.25 {
		t.Errorf("expected Jitter 0.25, got %f", cfg.Jitter)
	}

	customRetryIf := func(err error) bool { return true }
	WithRetryIf(customRetryIf)(&cfg)
	if cfg.RetryIf == nil {
		t.Error("expected RetryIf to be set")
	}
}
