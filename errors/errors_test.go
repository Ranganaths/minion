package errors

import (
	"errors"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrInvalidConfig", ErrInvalidConfig, "invalid configuration"},
		{"ErrMissingRequired", ErrMissingRequired, "missing required field"},
		{"ErrInvalidInput", ErrInvalidInput, "invalid input"},
		{"ErrNotFound", ErrNotFound, "not found"},
		{"ErrAlreadyExists", ErrAlreadyExists, "already exists"},
		{"ErrClosed", ErrClosed, "resource closed"},
		{"ErrTimeout", ErrTimeout, "operation timed out"},
		{"ErrCanceled", ErrCanceled, "operation canceled"},
		{"ErrRateLimited", ErrRateLimited, "rate limited"},
		{"ErrQuotaExceeded", ErrQuotaExceeded, "quota exceeded"},
		{"ErrAuthFailed", ErrAuthFailed, "authentication failed"},
		{"ErrPermissionDenied", ErrPermissionDenied, "permission denied"},
		{"ErrDimensionMismatch", ErrDimensionMismatch, "embedding dimension mismatch"},
		{"ErrEmptyInput", ErrEmptyInput, "empty input"},
		{"ErrFileTooLarge", ErrFileTooLarge, "file too large"},
		{"ErrUnsupportedOperation", ErrUnsupportedOperation, "unsupported operation"},
		{"ErrRetryable", ErrRetryable, "retryable error"},
		{"ErrPermanent", ErrPermanent, "permanent error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Error() != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, tc.err.Error())
			}
		})
	}
}

func TestChainError(t *testing.T) {
	t.Run("with operation", func(t *testing.T) {
		underlying := errors.New("connection failed")
		err := NewChainError("mychain", "execute", underlying)

		if err.Error() != "chain mychain: execute: connection failed" {
			t.Errorf("unexpected error message: %s", err.Error())
		}

		if !errors.Is(err, underlying) {
			t.Error("expected errors.Is to find underlying error")
		}
	})

	t.Run("without operation", func(t *testing.T) {
		underlying := errors.New("connection failed")
		err := NewChainError("mychain", "", underlying)

		if err.Error() != "chain mychain: connection failed" {
			t.Errorf("unexpected error message: %s", err.Error())
		}
	})
}

func TestEmbeddingError(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		underlying := errors.New("API error")
		err := NewEmbeddingError("openai", "embed", underlying, false)

		if err.Error() != "embedding openai: embed: API error" {
			t.Errorf("unexpected error message: %s", err.Error())
		}

		if !errors.Is(err, underlying) {
			t.Error("expected errors.Is to find underlying error")
		}
	})

	t.Run("retryable", func(t *testing.T) {
		underlying := errors.New("rate limited")
		err := NewEmbeddingError("openai", "embed", underlying, true)

		if !errors.Is(err, ErrRetryable) {
			t.Error("expected errors.Is(err, ErrRetryable) to return true")
		}
	})

	t.Run("not retryable", func(t *testing.T) {
		underlying := errors.New("invalid key")
		err := NewEmbeddingError("openai", "embed", underlying, false)

		if errors.Is(err, ErrRetryable) {
			t.Error("expected errors.Is(err, ErrRetryable) to return false")
		}
	})
}

func TestVectorStoreError(t *testing.T) {
	underlying := errors.New("disk full")
	err := NewVectorStoreError("memory", "add", underlying)

	if err.Error() != "vectorstore memory: add: disk full" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	if !errors.Is(err, underlying) {
		t.Error("expected errors.Is to find underlying error")
	}
}

func TestLLMError(t *testing.T) {
	t.Run("with status code", func(t *testing.T) {
		underlying := errors.New("server error")
		err := NewLLMError("anthropic", "claude-3", "generate", 500, underlying)

		if err.Error() != "llm anthropic/claude-3: generate (status 500): server error" {
			t.Errorf("unexpected error message: %s", err.Error())
		}
	})

	t.Run("without status code", func(t *testing.T) {
		underlying := errors.New("network error")
		err := NewLLMError("anthropic", "claude-3", "generate", 0, underlying)

		if err.Error() != "llm anthropic/claude-3: generate: network error" {
			t.Errorf("unexpected error message: %s", err.Error())
		}
	})

	t.Run("rate limited", func(t *testing.T) {
		underlying := errors.New("too many requests")
		err := NewLLMError("openai", "gpt-4", "chat", 429, underlying)

		if !errors.Is(err, ErrRateLimited) {
			t.Error("expected errors.Is(err, ErrRateLimited) to return true")
		}
		if !errors.Is(err, ErrRetryable) {
			t.Error("expected 429 to be retryable")
		}
	})

	t.Run("auth failed", func(t *testing.T) {
		underlying := errors.New("invalid key")
		err := NewLLMError("openai", "gpt-4", "chat", 401, underlying)

		if !errors.Is(err, ErrAuthFailed) {
			t.Error("expected errors.Is(err, ErrAuthFailed) to return true")
		}
	})

	t.Run("permission denied", func(t *testing.T) {
		underlying := errors.New("access denied")
		err := NewLLMError("openai", "gpt-4", "chat", 403, underlying)

		if !errors.Is(err, ErrPermissionDenied) {
			t.Error("expected errors.Is(err, ErrPermissionDenied) to return true")
		}
	})

	t.Run("retryable status codes", func(t *testing.T) {
		retryableCodes := []int{429, 500, 502, 503, 504}
		for _, code := range retryableCodes {
			err := NewLLMError("openai", "gpt-4", "chat", code, errors.New("error"))
			if !err.Retryable {
				t.Errorf("expected status code %d to be retryable", code)
			}
		}

		nonRetryableCodes := []int{400, 401, 403, 404}
		for _, code := range nonRetryableCodes {
			err := NewLLMError("openai", "gpt-4", "chat", code, errors.New("error"))
			if err.Retryable {
				t.Errorf("expected status code %d to not be retryable", code)
			}
		}
	})
}

func TestValidationError(t *testing.T) {
	t.Run("with value", func(t *testing.T) {
		err := NewValidationError("temperature", 2.5, "must be between 0 and 1")

		if err.Error() != "validation error: temperature: must be between 0 and 1 (got 2.5)" {
			t.Errorf("unexpected error message: %s", err.Error())
		}

		if !errors.Is(err, ErrInvalidInput) {
			t.Error("expected errors.Is(err, ErrInvalidInput) to return true")
		}
	})

	t.Run("without value", func(t *testing.T) {
		err := NewValidationError("name", nil, "is required")

		if err.Error() != "validation error: name: is required" {
			t.Errorf("unexpected error message: %s", err.Error())
		}
	})
}

func TestConfigError(t *testing.T) {
	err := NewConfigError("embeddings", "APIKey", "is required")

	if err.Error() != "config error: embeddings.APIKey: is required" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Error("expected errors.Is(err, ErrInvalidConfig) to return true")
	}
}

func TestRetryableError(t *testing.T) {
	underlying := errors.New("transient failure")
	err := NewRetryableError(underlying, 3)

	if err.Error() != "transient failure" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	if !errors.Is(err, ErrRetryable) {
		t.Error("expected errors.Is(err, ErrRetryable) to return true")
	}

	if !errors.Is(err, underlying) {
		t.Error("expected errors.Is to find underlying error")
	}

	if err.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", err.MaxRetries)
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("IsRetryable", func(t *testing.T) {
		if !IsRetryable(NewRetryableError(errors.New("test"), 1)) {
			t.Error("expected IsRetryable to return true")
		}
		if IsRetryable(errors.New("test")) {
			t.Error("expected IsRetryable to return false")
		}
	})

	t.Run("IsRateLimited", func(t *testing.T) {
		if !IsRateLimited(Wrap(ErrRateLimited, "context")) {
			t.Error("expected IsRateLimited to return true")
		}
		if IsRateLimited(errors.New("test")) {
			t.Error("expected IsRateLimited to return false")
		}
	})

	t.Run("IsTimeout", func(t *testing.T) {
		if !IsTimeout(Wrap(ErrTimeout, "context")) {
			t.Error("expected IsTimeout to return true")
		}
		if IsTimeout(errors.New("test")) {
			t.Error("expected IsTimeout to return false")
		}
	})

	t.Run("IsCanceled", func(t *testing.T) {
		if !IsCanceled(Wrap(ErrCanceled, "context")) {
			t.Error("expected IsCanceled to return true")
		}
		if IsCanceled(errors.New("test")) {
			t.Error("expected IsCanceled to return false")
		}
	})

	t.Run("IsNotFound", func(t *testing.T) {
		if !IsNotFound(Wrap(ErrNotFound, "context")) {
			t.Error("expected IsNotFound to return true")
		}
		if IsNotFound(errors.New("test")) {
			t.Error("expected IsNotFound to return false")
		}
	})
}

func TestWrap(t *testing.T) {
	t.Run("wrap error", func(t *testing.T) {
		underlying := errors.New("original")
		wrapped := Wrap(underlying, "context")

		if wrapped.Error() != "context: original" {
			t.Errorf("unexpected error message: %s", wrapped.Error())
		}

		if !errors.Is(wrapped, underlying) {
			t.Error("expected errors.Is to find underlying error")
		}
	})

	t.Run("wrap nil", func(t *testing.T) {
		wrapped := Wrap(nil, "context")
		if wrapped != nil {
			t.Error("expected nil when wrapping nil")
		}
	})
}

func TestWrapf(t *testing.T) {
	t.Run("wrapf error", func(t *testing.T) {
		underlying := errors.New("original")
		wrapped := Wrapf(underlying, "context %d", 42)

		if wrapped.Error() != "context 42: original" {
			t.Errorf("unexpected error message: %s", wrapped.Error())
		}

		if !errors.Is(wrapped, underlying) {
			t.Error("expected errors.Is to find underlying error")
		}
	})

	t.Run("wrapf nil", func(t *testing.T) {
		wrapped := Wrapf(nil, "context %d", 42)
		if wrapped != nil {
			t.Error("expected nil when wrapping nil")
		}
	})
}

func TestErrorsAs(t *testing.T) {
	t.Run("ChainError", func(t *testing.T) {
		err := Wrap(NewChainError("test", "op", errors.New("inner")), "outer")

		var chainErr *ChainError
		if !errors.As(err, &chainErr) {
			t.Error("expected errors.As to find ChainError")
		}
		if chainErr.ChainName != "test" {
			t.Errorf("expected ChainName 'test', got %q", chainErr.ChainName)
		}
	})

	t.Run("LLMError", func(t *testing.T) {
		err := Wrap(NewLLMError("openai", "gpt-4", "chat", 429, errors.New("inner")), "outer")

		var llmErr *LLMError
		if !errors.As(err, &llmErr) {
			t.Error("expected errors.As to find LLMError")
		}
		if llmErr.StatusCode != 429 {
			t.Errorf("expected StatusCode 429, got %d", llmErr.StatusCode)
		}
	})
}
