package retry_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	minerrors "github.com/Ranganaths/minion/errors"
	"github.com/Ranganaths/minion/retry"
)

func ExampleDo() {
	ctx := context.Background()
	attempts := 0

	result, err := retry.Do(ctx, func() (string, error) {
		attempts++
		if attempts < 3 {
			// Simulate transient failure
			return "", minerrors.NewRetryableError(errors.New("temporary failure"), 5)
		}
		return "success", nil
	}, retry.WithMaxRetries(5), retry.WithInitialDelay(10*time.Millisecond))

	if err != nil {
		fmt.Printf("Failed after retries: %v\n", err)
		return
	}
	fmt.Printf("Result: %s, Attempts: %d\n", result, attempts)
	// Output: Result: success, Attempts: 3
}

func ExampleDoVoid() {
	ctx := context.Background()
	attempts := 0

	err := retry.DoVoid(ctx, func() error {
		attempts++
		if attempts < 2 {
			return minerrors.NewRetryableError(errors.New("temporary failure"), 5)
		}
		return nil
	}, retry.WithMaxRetries(3), retry.WithInitialDelay(10*time.Millisecond))

	if err != nil {
		fmt.Printf("Failed: %v\n", err)
		return
	}
	fmt.Printf("Success after %d attempts\n", attempts)
	// Output: Success after 2 attempts
}

func ExampleBackoff() {
	// Create a backoff strategy for manual retry loops
	b := retry.NewBackoff(
		retry.WithInitialDelay(100*time.Millisecond),
		retry.WithMultiplier(2.0),
		retry.WithMaxDelay(5*time.Second),
		retry.WithJitter(0),
	)

	// Get successive delays
	d1 := b.Next()
	d2 := b.Next()
	d3 := b.Next()

	fmt.Printf("Delay 1: %v\n", d1)
	fmt.Printf("Delay 2: %v\n", d2)
	fmt.Printf("Delay 3: %v\n", d3)
	fmt.Printf("Current attempt: %d\n", b.Attempt())
	// Output:
	// Delay 1: 100ms
	// Delay 2: 200ms
	// Delay 3: 400ms
	// Current attempt: 3
}

func ExampleDoWithCallback() {
	ctx := context.Background()
	attempts := 0

	result, err := retry.DoWithCallback(ctx, func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", minerrors.NewRetryableError(errors.New("temporary failure"), 5)
		}
		return "done", nil
	}, func(attempt int, err error, delay time.Duration) {
		fmt.Printf("Retry %d after error: %v (waiting %v)\n", attempt, err, delay)
	}, retry.WithMaxRetries(5), retry.WithInitialDelay(10*time.Millisecond), retry.WithJitter(0))

	if err != nil {
		fmt.Printf("Failed: %v\n", err)
		return
	}
	fmt.Printf("Result: %s\n", result)
	// Output:
	// Retry 1 after error: temporary failure (waiting 10ms)
	// Retry 2 after error: temporary failure (waiting 20ms)
	// Result: done
}

func ExampleWithRetryIf() {
	ctx := context.Background()
	attempts := 0

	// Only retry specific errors
	_, err := retry.Do(ctx, func() (string, error) {
		attempts++
		return "", errors.New("permanent error")
	}, retry.WithMaxRetries(5),
		retry.WithRetryIf(func(err error) bool {
			// Don't retry permanent errors
			return false
		}))

	fmt.Printf("Attempts: %d, Error: %v\n", attempts, err != nil)
	// Output: Attempts: 1, Error: true
}

func ExampleNewWithRetry() {
	ctx := context.Background()
	attempts := 0

	// Create a reusable retry wrapper
	w := retry.NewWithRetry(func() (int, error) {
		attempts++
		if attempts < 2 {
			return 0, minerrors.NewRetryableError(errors.New("temporary"), 3)
		}
		return 42, nil
	}, retry.WithMaxRetries(3), retry.WithInitialDelay(10*time.Millisecond))

	result, err := w.Run(ctx)
	if err != nil {
		fmt.Printf("Failed: %v\n", err)
		return
	}
	fmt.Printf("Result: %d\n", result)
	// Output: Result: 42
}
