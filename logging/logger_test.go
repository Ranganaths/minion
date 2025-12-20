package logging

import (
	"bytes"
	"context"
	"log"
	"testing"
)

func TestStdLogger(t *testing.T) {
	t.Run("Basic logging", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewStdLoggerWithOutput(LevelDebug, log.New(&buf, "", 0))

		ctx := context.Background()
		logger.Debug(ctx, "debug message")
		logger.Info(ctx, "info message")
		logger.Warn(ctx, "warn message")
		logger.Error(ctx, "error message")

		output := buf.String()
		if !contains(output, "DEBUG debug message") {
			t.Errorf("expected DEBUG message in output")
		}
		if !contains(output, "INFO info message") {
			t.Errorf("expected INFO message in output")
		}
		if !contains(output, "WARN warn message") {
			t.Errorf("expected WARN message in output")
		}
		if !contains(output, "ERROR error message") {
			t.Errorf("expected ERROR message in output")
		}
	})

	t.Run("Log level filtering", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewStdLoggerWithOutput(LevelWarn, log.New(&buf, "", 0))

		ctx := context.Background()
		logger.Debug(ctx, "debug message")
		logger.Info(ctx, "info message")
		logger.Warn(ctx, "warn message")
		logger.Error(ctx, "error message")

		output := buf.String()
		if contains(output, "DEBUG") {
			t.Errorf("DEBUG message should be filtered")
		}
		if contains(output, "INFO") {
			t.Errorf("INFO message should be filtered")
		}
		if !contains(output, "WARN") {
			t.Errorf("expected WARN message in output")
		}
		if !contains(output, "ERROR") {
			t.Errorf("expected ERROR message in output")
		}
	})

	t.Run("With fields", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewStdLoggerWithOutput(LevelInfo, log.New(&buf, "", 0))

		ctx := context.Background()
		logger.Info(ctx, "message", F("key1", "value1"), F("key2", 42))

		output := buf.String()
		if !contains(output, "key1") || !contains(output, "value1") {
			t.Errorf("expected field key1=value1 in output: %s", output)
		}
	})

	t.Run("WithFields method", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewStdLoggerWithOutput(LevelInfo, log.New(&buf, "", 0))

		ctx := context.Background()
		childLogger := logger.WithFields(F("component", "test"))
		childLogger.Info(ctx, "message")

		output := buf.String()
		if !contains(output, "component") {
			t.Errorf("expected preset field in output: %s", output)
		}
	})

	t.Run("WithName method", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewStdLoggerWithOutput(LevelInfo, log.New(&buf, "", 0))

		ctx := context.Background()
		childLogger := logger.WithName("mycomponent")
		childLogger.Info(ctx, "message")

		output := buf.String()
		if !contains(output, "[mycomponent]") {
			t.Errorf("expected name prefix in output: %s", output)
		}
	})

	t.Run("Nested names", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewStdLoggerWithOutput(LevelInfo, log.New(&buf, "", 0))

		ctx := context.Background()
		childLogger := logger.WithName("parent").WithName("child")
		childLogger.Info(ctx, "message")

		output := buf.String()
		if !contains(output, "[parent.child]") {
			t.Errorf("expected nested name in output: %s", output)
		}
	})
}

func TestNopLogger(t *testing.T) {
	logger := NewNopLogger()
	ctx := context.Background()

	// Should not panic
	logger.Debug(ctx, "message")
	logger.Info(ctx, "message")
	logger.Warn(ctx, "message")
	logger.Error(ctx, "message")

	child := logger.WithFields(F("key", "value"))
	child.Info(ctx, "message")

	named := logger.WithName("test")
	named.Info(ctx, "message")
}

func TestGlobalLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewStdLoggerWithOutput(LevelInfo, log.New(&buf, "", 0))

	SetLogger(logger)
	ctx := context.Background()

	Info(ctx, "test message")

	output := buf.String()
	if !contains(output, "INFO test message") {
		t.Errorf("expected message via global logger: %s", output)
	}

	// Reset to default
	SetLogger(NewStdLogger(LevelInfo))
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tc := range tests {
		if tc.level.String() != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, tc.level.String())
		}
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
