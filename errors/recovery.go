package errors

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
)

// PanicError represents a recovered panic.
type PanicError struct {
	Value      any
	StackTrace string
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("panic: %v", e.Value)
}

// NewPanicError creates a new PanicError with the current stack trace.
func NewPanicError(value any) *PanicError {
	return &PanicError{
		Value:      value,
		StackTrace: string(debug.Stack()),
	}
}

// RecoverFunc is a function that handles recovered panics.
type RecoverFunc func(panicErr *PanicError)

// DefaultRecoverFunc logs the panic using fmt.Println.
// In production, replace this with proper logging.
var DefaultRecoverFunc RecoverFunc = func(panicErr *PanicError) {
	fmt.Printf("recovered from panic: %v\n%s\n", panicErr.Value, panicErr.StackTrace)
}

// Go runs a function in a goroutine with panic recovery.
// If the function panics, the panic is recovered and passed to the recover function.
// If recoverFn is nil, DefaultRecoverFunc is used.
func Go(fn func(), recoverFn RecoverFunc) {
	if recoverFn == nil {
		recoverFn = DefaultRecoverFunc
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				recoverFn(NewPanicError(r))
			}
		}()
		fn()
	}()
}

// GoWithContext runs a function in a goroutine with panic recovery and context support.
// The function is called with a done channel that is closed when the goroutine exits.
func GoWithContext(ctx context.Context, fn func(ctx context.Context), recoverFn RecoverFunc) <-chan struct{} {
	if recoverFn == nil {
		recoverFn = DefaultRecoverFunc
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				recoverFn(NewPanicError(r))
			}
		}()
		fn(ctx)
	}()
	return done
}

// GoWithError runs a function in a goroutine with panic recovery and error return.
// Panics are converted to errors and sent on the returned error channel.
func GoWithError(fn func() error, recoverFn RecoverFunc) <-chan error {
	errCh := make(chan error, 1)
	if recoverFn == nil {
		recoverFn = DefaultRecoverFunc
	}

	go func() {
		defer close(errCh)
		defer func() {
			if r := recover(); r != nil {
				panicErr := NewPanicError(r)
				recoverFn(panicErr)
				errCh <- panicErr
			}
		}()
		if err := fn(); err != nil {
			errCh <- err
		}
	}()
	return errCh
}

// SafeGo runs a function in a goroutine with panic recovery.
// Panics are silently recovered. Use when you don't care about panics.
func SafeGo(fn func()) {
	go func() {
		defer func() {
			_ = recover()
		}()
		fn()
	}()
}

// SafeFunc wraps a function with panic recovery, converting panics to errors.
func SafeFunc[T any](fn func() (T, error)) func() (T, error) {
	return func() (result T, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = NewPanicError(r)
			}
		}()
		return fn()
	}
}

// SafeFuncVoid wraps a void function with panic recovery, converting panics to errors.
func SafeFuncVoid(fn func() error) func() error {
	return func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = NewPanicError(r)
			}
		}()
		return fn()
	}
}

// WorkerPool runs multiple goroutines with panic recovery.
// It returns when all workers have finished.
type WorkerPool struct {
	wg        sync.WaitGroup
	recoverFn RecoverFunc
}

// NewWorkerPool creates a new worker pool with the given recovery function.
func NewWorkerPool(recoverFn RecoverFunc) *WorkerPool {
	if recoverFn == nil {
		recoverFn = DefaultRecoverFunc
	}
	return &WorkerPool{recoverFn: recoverFn}
}

// Go runs a function in the worker pool with panic recovery.
func (p *WorkerPool) Go(fn func()) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				p.recoverFn(NewPanicError(r))
			}
		}()
		fn()
	}()
}

// GoWithContext runs a function in the worker pool with panic recovery and context.
func (p *WorkerPool) GoWithContext(ctx context.Context, fn func(ctx context.Context)) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				p.recoverFn(NewPanicError(r))
			}
		}()
		fn(ctx)
	}()
}

// Wait waits for all workers in the pool to finish.
func (p *WorkerPool) Wait() {
	p.wg.Wait()
}

// Recover recovers from a panic and calls the handler function.
// Use this in a defer statement at the top of functions that might panic.
//
//	defer errors.Recover(func(err *PanicError) {
//	    log.Printf("recovered: %v", err)
//	})
func Recover(handler RecoverFunc) {
	if r := recover(); r != nil {
		if handler != nil {
			handler(NewPanicError(r))
		}
	}
}

// RecoverToError recovers from a panic and assigns it to the given error pointer.
// Use this to convert panics to errors in functions that return errors.
//
//	func MyFunc() (err error) {
//	    defer errors.RecoverToError(&err)
//	    // ... code that might panic
//	}
func RecoverToError(errPtr *error) {
	if r := recover(); r != nil {
		*errPtr = NewPanicError(r)
	}
}
