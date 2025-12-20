package resilience

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenBucketLimiter(t *testing.T) {
	t.Run("basic acquire", func(t *testing.T) {
		l := NewTokenBucketLimiter(TokenBucketConfig{
			Rate:      10,
			BurstSize: 5,
		})

		// Should be able to acquire burst size immediately
		for i := 0; i < 5; i++ {
			if !l.TryAcquire() {
				t.Errorf("expected acquire %d to succeed", i)
			}
		}

		// Next should fail
		if l.TryAcquire() {
			t.Error("expected acquire to fail after burst exhausted")
		}
	})

	t.Run("refill", func(t *testing.T) {
		l := NewTokenBucketLimiter(TokenBucketConfig{
			Rate:      100, // 100 per second = 10 per 100ms
			BurstSize: 1,
		})

		// Exhaust the bucket
		l.TryAcquire()
		if l.TryAcquire() {
			t.Error("expected bucket to be empty")
		}

		// Wait for refill
		time.Sleep(50 * time.Millisecond)

		// Should have some tokens now
		if l.Available() < 0.5 {
			t.Errorf("expected tokens to refill, got %f", l.Available())
		}
	})

	t.Run("wait", func(t *testing.T) {
		l := NewTokenBucketLimiter(TokenBucketConfig{
			Rate:        100, // Fast refill
			BurstSize:   1,
			WaitTimeout: time.Second,
		})

		// Exhaust
		l.TryAcquire()

		ctx := context.Background()
		start := time.Now()
		err := l.Wait(ctx)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("expected wait to succeed, got %v", err)
		}
		if elapsed > 100*time.Millisecond {
			t.Errorf("wait took too long: %v", elapsed)
		}
	})

	t.Run("wait timeout", func(t *testing.T) {
		l := NewTokenBucketLimiter(TokenBucketConfig{
			Rate:        0.1, // Very slow refill
			BurstSize:   1,
			WaitTimeout: 50 * time.Millisecond,
		})

		// Exhaust
		l.TryAcquire()

		ctx := context.Background()
		err := l.Wait(ctx)

		if err == nil {
			t.Error("expected wait to timeout")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		l := NewTokenBucketLimiter(TokenBucketConfig{
			Rate:        0.1,
			BurstSize:   1,
			WaitTimeout: 10 * time.Second,
		})

		// Exhaust
		l.TryAcquire()

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := l.Wait(ctx)
		if err == nil {
			t.Error("expected wait to be canceled")
		}
	})

	t.Run("limit", func(t *testing.T) {
		l := NewTokenBucketLimiter(TokenBucketConfig{
			Rate:      42.5,
			BurstSize: 10,
		})

		if l.Limit() != 42.5 {
			t.Errorf("expected limit 42.5, got %f", l.Limit())
		}
	})

	t.Run("concurrent access", func(t *testing.T) {
		l := NewTokenBucketLimiter(TokenBucketConfig{
			Rate:      1000,
			BurstSize: 100,
		})

		var acquired atomic.Int32
		var wg sync.WaitGroup

		// Start all goroutines at once
		start := make(chan struct{})
		for i := 0; i < 200; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if l.TryAcquire() {
					acquired.Add(1)
				}
			}()
		}

		close(start)
		wg.Wait()

		// Should have acquired at most burst size + some refill
		// Allow some slack for refill during execution
		if acquired.Load() > 110 {
			t.Errorf("acquired significantly more than burst size: %d", acquired.Load())
		}
	})
}

func TestSlidingWindowLimiter(t *testing.T) {
	t.Run("basic acquire", func(t *testing.T) {
		l := NewSlidingWindowLimiter(SlidingWindowConfig{
			MaxRequests: 5,
			Window:      time.Second,
		})

		// Should be able to acquire max requests
		for i := 0; i < 5; i++ {
			if !l.TryAcquire() {
				t.Errorf("expected acquire %d to succeed", i)
			}
		}

		// Next should fail
		if l.TryAcquire() {
			t.Error("expected acquire to fail after max reached")
		}
	})

	t.Run("window expiry", func(t *testing.T) {
		l := NewSlidingWindowLimiter(SlidingWindowConfig{
			MaxRequests: 2,
			Window:      100 * time.Millisecond,
		})

		// Exhaust
		l.TryAcquire()
		l.TryAcquire()

		if l.TryAcquire() {
			t.Error("expected acquire to fail")
		}

		// Wait for window to expire
		time.Sleep(150 * time.Millisecond)

		if !l.TryAcquire() {
			t.Error("expected acquire to succeed after window expiry")
		}
	})

	t.Run("available", func(t *testing.T) {
		l := NewSlidingWindowLimiter(SlidingWindowConfig{
			MaxRequests: 5,
			Window:      time.Second,
		})

		if l.Available() != 5 {
			t.Errorf("expected 5 available, got %d", l.Available())
		}

		l.TryAcquire()
		l.TryAcquire()

		if l.Available() != 3 {
			t.Errorf("expected 3 available, got %d", l.Available())
		}
	})

	t.Run("wait", func(t *testing.T) {
		l := NewSlidingWindowLimiter(SlidingWindowConfig{
			MaxRequests: 1,
			Window:      100 * time.Millisecond,
			WaitTimeout: time.Second,
		})

		l.TryAcquire()

		ctx := context.Background()
		start := time.Now()
		err := l.Wait(ctx)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("expected wait to succeed, got %v", err)
		}
		if elapsed < 50*time.Millisecond {
			t.Errorf("wait was too fast: %v", elapsed)
		}
	})

	t.Run("limit", func(t *testing.T) {
		l := NewSlidingWindowLimiter(SlidingWindowConfig{
			MaxRequests: 60,
			Window:      time.Minute,
		})

		// 60 per minute = 1 per second
		if l.Limit() != 1.0 {
			t.Errorf("expected limit 1.0, got %f", l.Limit())
		}
	})
}

func TestMultiLimiter(t *testing.T) {
	t.Run("all must allow", func(t *testing.T) {
		l1 := NewTokenBucketLimiter(TokenBucketConfig{Rate: 10, BurstSize: 5})
		l2 := NewTokenBucketLimiter(TokenBucketConfig{Rate: 10, BurstSize: 3})

		ml := NewMultiLimiter(l1, l2)

		// Should be limited by l2 (smaller burst)
		for i := 0; i < 3; i++ {
			if !ml.TryAcquire() {
				t.Errorf("expected acquire %d to succeed", i)
			}
		}

		// l2 is now exhausted
		if ml.TryAcquire() {
			t.Error("expected acquire to fail")
		}
	})

	t.Run("limit returns minimum", func(t *testing.T) {
		l1 := NewTokenBucketLimiter(TokenBucketConfig{Rate: 100, BurstSize: 10})
		l2 := NewTokenBucketLimiter(TokenBucketConfig{Rate: 50, BurstSize: 10})

		ml := NewMultiLimiter(l1, l2)

		if ml.Limit() != 50 {
			t.Errorf("expected limit 50, got %f", ml.Limit())
		}
	})
}

func TestProviderLimiters(t *testing.T) {
	t.Run("creates limiters on demand", func(t *testing.T) {
		created := make(map[string]bool)
		pl := NewProviderLimiters(func(provider string) RateLimiter {
			created[provider] = true
			return NewTokenBucketLimiter(TokenBucketConfig{Rate: 10, BurstSize: 5})
		})

		pl.Get("openai")
		pl.Get("anthropic")
		pl.Get("openai") // Should reuse

		if !created["openai"] || !created["anthropic"] {
			t.Error("expected both providers to be created")
		}
	})

	t.Run("wait delegates to provider limiter", func(t *testing.T) {
		pl := NewProviderLimiters(func(provider string) RateLimiter {
			return NewTokenBucketLimiter(TokenBucketConfig{Rate: 100, BurstSize: 10})
		})

		ctx := context.Background()
		err := pl.Wait(ctx, "openai")
		if err != nil {
			t.Errorf("expected wait to succeed, got %v", err)
		}
	})
}

func TestDefaultProviderLimiters(t *testing.T) {
	pl := NewDefaultProviderLimiters()

	// Known providers should have specific limits
	openaiLimiter := pl.Get("openai")
	if openaiLimiter.Limit() != 60 {
		t.Errorf("expected openai limit 60, got %f", openaiLimiter.Limit())
	}

	// Unknown providers should get default limit
	unknownLimiter := pl.Get("unknown-provider")
	if unknownLimiter.Limit() != 0.5 {
		t.Errorf("expected unknown limit 0.5, got %f", unknownLimiter.Limit())
	}
}
