package errors

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestPanicError(t *testing.T) {
	err := NewPanicError("test panic")

	if err.Value != "test panic" {
		t.Errorf("expected Value 'test panic', got %v", err.Value)
	}

	if err.StackTrace == "" {
		t.Error("expected StackTrace to be populated")
	}

	expectedMsg := "panic: test panic"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestGo(t *testing.T) {
	t.Run("normal execution", func(t *testing.T) {
		done := make(chan struct{})
		Go(func() {
			close(done)
		}, nil)

		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Error("timeout waiting for goroutine")
		}
	})

	t.Run("panic recovery", func(t *testing.T) {
		recovered := make(chan *PanicError, 1)
		Go(func() {
			panic("test panic")
		}, func(err *PanicError) {
			recovered <- err
		})

		select {
		case err := <-recovered:
			if err.Value != "test panic" {
				t.Errorf("expected 'test panic', got %v", err.Value)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for recovery")
		}
	})
}

func TestGoWithContext(t *testing.T) {
	t.Run("normal execution", func(t *testing.T) {
		ctx := context.Background()
		done := GoWithContext(ctx, func(ctx context.Context) {
			// Do nothing
		}, nil)

		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Error("timeout waiting for goroutine")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		done := GoWithContext(ctx, func(ctx context.Context) {
			<-ctx.Done()
		}, nil)

		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Error("timeout waiting for context cancellation")
		}
	})

	t.Run("panic recovery", func(t *testing.T) {
		recovered := make(chan *PanicError, 1)
		ctx := context.Background()
		done := GoWithContext(ctx, func(ctx context.Context) {
			panic("context panic")
		}, func(err *PanicError) {
			recovered <- err
		})

		select {
		case <-done:
			// Goroutine finished
		case <-time.After(time.Second):
			t.Error("timeout waiting for goroutine")
		}

		select {
		case err := <-recovered:
			if err.Value != "context panic" {
				t.Errorf("expected 'context panic', got %v", err.Value)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("expected panic to be recovered")
		}
	})
}

func TestGoWithError(t *testing.T) {
	t.Run("normal execution without error", func(t *testing.T) {
		errCh := GoWithError(func() error {
			return nil
		}, nil)

		select {
		case err := <-errCh:
			if err != nil {
				t.Errorf("expected nil error, got %v", err)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for goroutine")
		}
	})

	t.Run("normal execution with error", func(t *testing.T) {
		errCh := GoWithError(func() error {
			return ErrTimeout
		}, nil)

		select {
		case err := <-errCh:
			if !IsTimeout(err) {
				t.Errorf("expected timeout error, got %v", err)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for goroutine")
		}
	})

	t.Run("panic recovery", func(t *testing.T) {
		recovered := make(chan *PanicError, 1)
		errCh := GoWithError(func() error {
			panic("error panic")
		}, func(err *PanicError) {
			recovered <- err
		})

		select {
		case err := <-errCh:
			if _, ok := err.(*PanicError); !ok {
				t.Errorf("expected PanicError, got %T", err)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for goroutine")
		}
	})
}

func TestSafeGo(t *testing.T) {
	done := make(chan struct{})
	SafeGo(func() {
		defer close(done)
		panic("safe go panic")
	})

	select {
	case <-done:
		// Success - panic was recovered
	case <-time.After(time.Second):
		t.Error("timeout waiting for goroutine")
	}
}

func TestSafeFunc(t *testing.T) {
	t.Run("normal execution", func(t *testing.T) {
		fn := SafeFunc(func() (int, error) {
			return 42, nil
		})

		result, err := fn()
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if result != 42 {
			t.Errorf("expected 42, got %d", result)
		}
	})

	t.Run("panic recovery", func(t *testing.T) {
		fn := SafeFunc(func() (int, error) {
			panic("safe func panic")
		})

		result, err := fn()
		if err == nil {
			t.Error("expected error from panic")
		}
		if result != 0 {
			t.Errorf("expected zero value, got %d", result)
		}
		if _, ok := err.(*PanicError); !ok {
			t.Errorf("expected PanicError, got %T", err)
		}
	})
}

func TestSafeFuncVoid(t *testing.T) {
	t.Run("normal execution", func(t *testing.T) {
		fn := SafeFuncVoid(func() error {
			return nil
		})

		err := fn()
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("panic recovery", func(t *testing.T) {
		fn := SafeFuncVoid(func() error {
			panic("safe func void panic")
		})

		err := fn()
		if err == nil {
			t.Error("expected error from panic")
		}
		if _, ok := err.(*PanicError); !ok {
			t.Errorf("expected PanicError, got %T", err)
		}
	})
}

func TestWorkerPool(t *testing.T) {
	t.Run("all workers complete", func(t *testing.T) {
		pool := NewWorkerPool(nil)
		counter := atomic.Int32{}

		for i := 0; i < 10; i++ {
			pool.Go(func() {
				counter.Add(1)
			})
		}

		pool.Wait()

		if counter.Load() != 10 {
			t.Errorf("expected counter 10, got %d", counter.Load())
		}
	})

	t.Run("panic in worker is recovered", func(t *testing.T) {
		recoveredCount := atomic.Int32{}
		pool := NewWorkerPool(func(err *PanicError) {
			recoveredCount.Add(1)
		})

		counter := atomic.Int32{}

		for i := 0; i < 10; i++ {
			idx := i
			pool.Go(func() {
				if idx%2 == 0 {
					panic("worker panic")
				}
				counter.Add(1)
			})
		}

		pool.Wait()

		if counter.Load() != 5 {
			t.Errorf("expected counter 5, got %d", counter.Load())
		}
		if recoveredCount.Load() != 5 {
			t.Errorf("expected 5 recoveries, got %d", recoveredCount.Load())
		}
	})

	t.Run("with context", func(t *testing.T) {
		pool := NewWorkerPool(nil)
		ctx := context.Background()
		counter := atomic.Int32{}

		for i := 0; i < 10; i++ {
			pool.GoWithContext(ctx, func(ctx context.Context) {
				counter.Add(1)
			})
		}

		pool.Wait()

		if counter.Load() != 10 {
			t.Errorf("expected counter 10, got %d", counter.Load())
		}
	})
}

func TestRecover(t *testing.T) {
	recovered := false

	func() {
		defer Recover(func(err *PanicError) {
			recovered = true
		})
		panic("test recover")
	}()

	if !recovered {
		t.Error("expected panic to be recovered")
	}
}

func TestRecoverToError(t *testing.T) {
	fn := func() (err error) {
		defer RecoverToError(&err)
		panic("recover to error")
	}

	err := fn()
	if err == nil {
		t.Error("expected error")
	}

	panicErr, ok := err.(*PanicError)
	if !ok {
		t.Errorf("expected PanicError, got %T", err)
	}

	if panicErr.Value != "recover to error" {
		t.Errorf("expected 'recover to error', got %v", panicErr.Value)
	}
}
