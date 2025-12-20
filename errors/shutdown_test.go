package errors

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

type mockResource struct {
	name       string
	shutdownFn func(ctx context.Context) error
	called     atomic.Bool
	order      *[]string
}

func (m *mockResource) Shutdown(ctx context.Context) error {
	m.called.Store(true)
	if m.order != nil {
		*m.order = append(*m.order, m.name)
	}
	if m.shutdownFn != nil {
		return m.shutdownFn(ctx)
	}
	return nil
}

func TestShutdownManager_RegisterAndShutdown(t *testing.T) {
	sm := NewShutdownManager(5 * time.Second)

	order := []string{}
	r1 := &mockResource{name: "r1", order: &order}
	r2 := &mockResource{name: "r2", order: &order}
	r3 := &mockResource{name: "r3", order: &order}

	sm.Register("r1", r1)
	sm.Register("r2", r2)
	sm.Register("r3", r3)

	if sm.Count() != 3 {
		t.Errorf("expected count 3, got %d", sm.Count())
	}

	err := sm.Shutdown(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check all were called
	if !r1.called.Load() || !r2.called.Load() || !r3.called.Load() {
		t.Error("not all resources were shut down")
	}

	// Check LIFO order
	expected := []string{"r3", "r2", "r1"}
	if len(order) != len(expected) {
		t.Errorf("expected order %v, got %v", expected, order)
	}
	for i, name := range expected {
		if order[i] != name {
			t.Errorf("expected order[%d]=%s, got %s", i, name, order[i])
		}
	}
}

func TestShutdownManager_RegisterFunc(t *testing.T) {
	sm := NewShutdownManager(5 * time.Second)

	called := false
	sm.RegisterFunc("func", func(ctx context.Context) error {
		called = true
		return nil
	})

	err := sm.Shutdown(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !called {
		t.Error("function was not called")
	}
}

func TestShutdownManager_ErrorHandling(t *testing.T) {
	sm := NewShutdownManager(5 * time.Second)

	r1 := &mockResource{name: "r1", shutdownFn: func(ctx context.Context) error {
		return fmt.Errorf("r1 error")
	}}
	r2 := &mockResource{name: "r2"}
	r3 := &mockResource{name: "r3", shutdownFn: func(ctx context.Context) error {
		return fmt.Errorf("r3 error")
	}}

	sm.Register("r1", r1)
	sm.Register("r2", r2)
	sm.Register("r3", r3)

	err := sm.Shutdown(context.Background())

	// Should return first error (r3 in LIFO order)
	if err == nil {
		t.Error("expected error")
	}
	if !r1.called.Load() || !r2.called.Load() || !r3.called.Load() {
		t.Error("all resources should be attempted even with errors")
	}
}

func TestShutdownManager_Timeout(t *testing.T) {
	sm := NewShutdownManager(100 * time.Millisecond)

	sm.RegisterFunc("slow", func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	})

	start := time.Now()
	err := sm.Shutdown(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("shutdown took too long: %v", elapsed)
	}
}

func TestShutdownManager_Clear(t *testing.T) {
	sm := NewShutdownManager(5 * time.Second)

	sm.RegisterFunc("r1", func(ctx context.Context) error { return nil })
	sm.RegisterFunc("r2", func(ctx context.Context) error { return nil })

	if sm.Count() != 2 {
		t.Errorf("expected count 2, got %d", sm.Count())
	}

	sm.Clear()

	if sm.Count() != 0 {
		t.Errorf("expected count 0 after clear, got %d", sm.Count())
	}
}

func TestShutdownManager_SetLogger(t *testing.T) {
	sm := NewShutdownManager(5 * time.Second)

	logs := []string{}
	sm.SetLogger(func(msg string) {
		logs = append(logs, msg)
	})

	sm.RegisterFunc("test", func(ctx context.Context) error { return nil })
	sm.Shutdown(context.Background())

	if len(logs) < 2 {
		t.Errorf("expected at least 2 log messages, got %d", len(logs))
	}
}

func TestShutdownFunc(t *testing.T) {
	called := false
	fn := ShutdownFunc(func(ctx context.Context) error {
		called = true
		return nil
	})

	err := fn.Shutdown(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("function was not called")
	}
}

func TestGracefulServer_Run(t *testing.T) {
	t.Run("context cancellation", func(t *testing.T) {
		started := make(chan struct{})
		shutdownCalled := atomic.Bool{}

		gs := &GracefulServer{
			Start: func() error {
				close(started)
				time.Sleep(5 * time.Second)
				return nil
			},
			Shutdown: func(ctx context.Context) error {
				shutdownCalled.Store(true)
				return nil
			},
		}

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			<-started
			cancel()
		}()

		err := gs.Run(ctx, 100*time.Millisecond)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !shutdownCalled.Load() {
			t.Error("shutdown should have been called")
		}
	})
}

func TestDefaultShutdownManager(t *testing.T) {
	// Clear any existing registrations
	DefaultShutdownManager.Clear()

	called := false
	RegisterShutdownFunc("test", func(ctx context.Context) error {
		called = true
		return nil
	})

	if DefaultShutdownManager.Count() != 1 {
		t.Errorf("expected count 1, got %d", DefaultShutdownManager.Count())
	}

	err := ShutdownAll(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !called {
		t.Error("shutdown function was not called")
	}

	// Clean up
	DefaultShutdownManager.Clear()
}
