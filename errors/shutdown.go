package errors

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Shutdowner is an interface for resources that can be shut down gracefully.
type Shutdowner interface {
	Shutdown(ctx context.Context) error
}

// ShutdownFunc is a function that performs shutdown.
type ShutdownFunc func(ctx context.Context) error

// Shutdown implements Shutdowner for a function.
func (f ShutdownFunc) Shutdown(ctx context.Context) error {
	return f(ctx)
}

// ShutdownManager coordinates graceful shutdown of multiple resources.
// Resources are shut down in reverse order of registration (LIFO).
// ShutdownManager is safe for concurrent use.
type ShutdownManager struct {
	mu        sync.Mutex
	resources []namedResource
	timeout   time.Duration
	logger    func(msg string)
}

type namedResource struct {
	name     string
	resource Shutdowner
}

// NewShutdownManager creates a new shutdown manager with the given timeout.
// If timeout is 0, a default of 30 seconds is used.
func NewShutdownManager(timeout time.Duration) *ShutdownManager {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &ShutdownManager{
		timeout: timeout,
		logger:  func(msg string) {}, // No-op logger by default
	}
}

// SetLogger sets the logger function for shutdown messages.
func (sm *ShutdownManager) SetLogger(logger func(msg string)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.logger = logger
}

// Register registers a resource for graceful shutdown.
// Resources are shut down in reverse order of registration (LIFO).
func (sm *ShutdownManager) Register(name string, resource Shutdowner) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.resources = append(sm.resources, namedResource{name: name, resource: resource})
}

// RegisterFunc registers a shutdown function.
func (sm *ShutdownManager) RegisterFunc(name string, fn func(ctx context.Context) error) {
	sm.Register(name, ShutdownFunc(fn))
}

// Shutdown shuts down all registered resources in reverse order.
// It returns the first error encountered, but continues shutting down all resources.
func (sm *ShutdownManager) Shutdown(ctx context.Context) error {
	sm.mu.Lock()
	resources := make([]namedResource, len(sm.resources))
	copy(resources, sm.resources)
	logger := sm.logger
	sm.mu.Unlock()

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, sm.timeout)
	defer cancel()

	var firstErr error

	// Shutdown in reverse order (LIFO)
	for i := len(resources) - 1; i >= 0; i-- {
		res := resources[i]
		logger("Shutting down: " + res.name)

		if err := res.resource.Shutdown(ctx); err != nil {
			logger("Failed to shutdown " + res.name + ": " + err.Error())
			if firstErr == nil {
				firstErr = Wrapf(err, "failed to shutdown %s", res.name)
			}
		} else {
			logger("Shutdown complete: " + res.name)
		}
	}

	return firstErr
}

// Clear removes all registered resources.
func (sm *ShutdownManager) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.resources = nil
}

// Count returns the number of registered resources.
func (sm *ShutdownManager) Count() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return len(sm.resources)
}

// WaitForSignal waits for SIGINT or SIGTERM and then performs shutdown.
// It returns the signal that triggered shutdown and any error from shutdown.
func (sm *ShutdownManager) WaitForSignal(ctx context.Context) (os.Signal, error) {
	return sm.WaitForSignals(ctx, syscall.SIGINT, syscall.SIGTERM)
}

// WaitForSignals waits for any of the specified signals and then performs shutdown.
func (sm *ShutdownManager) WaitForSignals(ctx context.Context, signals ...os.Signal) (os.Signal, error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, signals...)
	defer signal.Stop(sigCh)

	select {
	case sig := <-sigCh:
		sm.mu.Lock()
		logger := sm.logger
		sm.mu.Unlock()

		logger("Received signal: " + sig.String())
		return sig, sm.Shutdown(ctx)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GracefulServer is a helper that wraps an HTTP-like server with graceful shutdown.
type GracefulServer struct {
	// Start starts the server (blocks until error or shutdown)
	Start func() error
	// Shutdown gracefully shuts down the server
	Shutdown func(ctx context.Context) error
}

// Run starts the server and handles graceful shutdown on signals.
// It returns when the server has shut down.
func (gs *GracefulServer) Run(ctx context.Context, timeout time.Duration, signals ...os.Signal) error {
	if len(signals) == 0 {
		signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}

	// Channel to receive server errors
	errCh := make(chan error, 1)

	// Start server in background
	go func() {
		if err := gs.Start(); err != nil {
			errCh <- err
		}
	}()

	// Wait for signal or server error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, signals...)
	defer signal.Stop(sigCh)

	select {
	case err := <-errCh:
		return err
	case <-sigCh:
		// Shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return gs.Shutdown(ctx)
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return gs.Shutdown(ctx)
	}
}

// RunWithShutdown is a convenience function that runs a function and handles shutdown on signals.
// The function receives a context that is canceled when a shutdown signal is received.
func RunWithShutdown(fn func(ctx context.Context) error, timeout time.Duration) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to receive function result
	errCh := make(chan error, 1)

	// Run function in background
	go func() {
		errCh <- fn(ctx)
	}()

	// Wait for signal or function completion
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-errCh:
		return err
	case <-sigCh:
		// Cancel context and wait for function with timeout
		cancel()
		select {
		case err := <-errCh:
			return err
		case <-time.After(timeout):
			return ErrTimeout
		}
	}
}

// DefaultShutdownManager is a global shutdown manager.
var DefaultShutdownManager = NewShutdownManager(30 * time.Second)

// RegisterForShutdown registers a resource with the default shutdown manager.
func RegisterForShutdown(name string, resource Shutdowner) {
	DefaultShutdownManager.Register(name, resource)
}

// RegisterShutdownFunc registers a shutdown function with the default shutdown manager.
func RegisterShutdownFunc(name string, fn func(ctx context.Context) error) {
	DefaultShutdownManager.RegisterFunc(name, fn)
}

// ShutdownAll shuts down all resources registered with the default manager.
func ShutdownAll(ctx context.Context) error {
	return DefaultShutdownManager.Shutdown(ctx)
}

// WaitAndShutdown waits for shutdown signals and then shuts down all resources.
func WaitAndShutdown() error {
	_, err := DefaultShutdownManager.WaitForSignal(context.Background())
	return err
}
