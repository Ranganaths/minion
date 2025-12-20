package errors_test

import (
	"errors"
	"fmt"

	minerrors "github.com/Ranganaths/minion/errors"
)

func ExampleIsRetryable() {
	// Check if an error is retryable
	err := minerrors.NewRetryableError(errors.New("connection failed"), 3)
	if minerrors.IsRetryable(err) {
		fmt.Println("Error is retryable")
	}
	// Output: Error is retryable
}

func ExampleNewChainError() {
	// Create a chain error with context
	underlying := errors.New("LLM API failed")
	err := minerrors.NewChainError("summarization", "invoke_llm", underlying)
	fmt.Println(err.Error())
	// Output: chain summarization: invoke_llm: LLM API failed
}

func ExampleNewLLMError() {
	// Create an LLM error with status code
	underlying := errors.New("rate limit exceeded")
	err := minerrors.NewLLMError("openai", "gpt-4", "chat", 429, underlying)
	fmt.Println(err.Error())

	// Check if it's rate limited
	if errors.Is(err, minerrors.ErrRateLimited) {
		fmt.Println("Error is rate limited")
	}
	// Output:
	// llm openai/gpt-4: chat (status 429): rate limit exceeded
	// Error is rate limited
}

func ExampleNewValidationError() {
	// Create a validation error
	err := minerrors.NewValidationError("temperature", 2.5, "must be between 0 and 1")
	fmt.Println(err.Error())

	// Check if it's an invalid input error
	if errors.Is(err, minerrors.ErrInvalidInput) {
		fmt.Println("This is an input validation error")
	}
	// Output:
	// validation error: temperature: must be between 0 and 1 (got 2.5)
	// This is an input validation error
}

func ExampleWrap() {
	// Wrap an error with additional context
	underlying := minerrors.ErrTimeout
	wrapped := minerrors.Wrap(underlying, "failed to fetch embeddings")
	fmt.Println(wrapped.Error())

	// The underlying error can still be found with errors.Is
	if errors.Is(wrapped, minerrors.ErrTimeout) {
		fmt.Println("Original error was a timeout")
	}
	// Output:
	// failed to fetch embeddings: operation timed out
	// Original error was a timeout
}

func ExampleWrapf() {
	// Wrap an error with formatted context
	underlying := minerrors.ErrNotFound
	wrapped := minerrors.Wrapf(underlying, "document %s not found in collection %s", "doc-123", "my-collection")
	fmt.Println(wrapped.Error())
	// Output: document doc-123 not found in collection my-collection: not found
}

func ExampleGo() {
	// Run a goroutine with panic recovery
	result := make(chan string, 1)
	minerrors.Go(func() {
		// This panic will be recovered
		panic("something went wrong")
	}, func(err *minerrors.PanicError) {
		result <- fmt.Sprintf("Recovered panic: %v", err.Value)
	})
	fmt.Println(<-result)
	// Output: Recovered panic: something went wrong
}

func ExampleSafeFunc() {
	// Wrap a function to convert panics to errors
	fn := minerrors.SafeFunc(func() (int, error) {
		panic("unexpected error")
	})

	result, err := fn()
	fmt.Printf("Result: %d, Error: %v\n", result, err != nil)
	// Output: Result: 0, Error: true
}

func ExampleRecoverToError() {
	fn := func() (err error) {
		defer minerrors.RecoverToError(&err)
		panic("something failed")
	}

	err := fn()
	if err != nil {
		fmt.Println("Function returned an error from panic")
	}
	// Output: Function returned an error from panic
}
