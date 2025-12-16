package observability

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Logger defines the logging interface for the multi-agent system
type Logger interface {
	// Debug logs a debug message
	Debug(msg string, fields ...Field)

	// Info logs an info message
	Info(msg string, fields ...Field)

	// Warn logs a warning message
	Warn(msg string, fields ...Field)

	// Error logs an error message
	Error(msg string, fields ...Field)

	// With returns a logger with additional fields
	With(fields ...Field) Logger

	// WithContext returns a logger with context
	WithContext(ctx context.Context) Logger
}

// Field represents a log field
type Field struct {
	Key   string
	Value interface{}
}

// LogLevel represents the log level
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LoggerConfig configures the logger
type LoggerConfig struct {
	Level      LogLevel
	JSONOutput bool
	Output     io.Writer
	WithCaller bool
}

// DefaultLoggerConfig returns default logger configuration
func DefaultLoggerConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:      LogLevelInfo,
		JSONOutput: true,
		Output:     os.Stdout,
		WithCaller: true,
	}
}

// ZerologLogger is a zerolog-based logger implementation
type ZerologLogger struct {
	logger zerolog.Logger
}

// NewLogger creates a new logger
func NewLogger(config *LoggerConfig) Logger {
	if config == nil {
		config = DefaultLoggerConfig()
	}

	// Configure zerolog
	zerolog.TimeFieldFormat = time.RFC3339Nano

	var output io.Writer = config.Output
	if !config.JSONOutput {
		output = zerolog.ConsoleWriter{
			Out:        config.Output,
			TimeFormat: time.RFC3339,
		}
	}

	// Set log level
	var level zerolog.Level
	switch config.Level {
	case LogLevelDebug:
		level = zerolog.DebugLevel
	case LogLevelInfo:
		level = zerolog.InfoLevel
	case LogLevelWarn:
		level = zerolog.WarnLevel
	case LogLevelError:
		level = zerolog.ErrorLevel
	default:
		level = zerolog.InfoLevel
	}

	logger := zerolog.New(output).
		Level(level).
		With().
		Timestamp()

	if config.WithCaller {
		logger = logger.Caller()
	}

	return &ZerologLogger{
		logger: logger.Logger(),
	}
}

// Debug logs a debug message
func (l *ZerologLogger) Debug(msg string, fields ...Field) {
	event := l.logger.Debug()
	for _, field := range fields {
		event = event.Interface(field.Key, field.Value)
	}
	event.Msg(msg)
}

// Info logs an info message
func (l *ZerologLogger) Info(msg string, fields ...Field) {
	event := l.logger.Info()
	for _, field := range fields {
		event = event.Interface(field.Key, field.Value)
	}
	event.Msg(msg)
}

// Warn logs a warning message
func (l *ZerologLogger) Warn(msg string, fields ...Field) {
	event := l.logger.Warn()
	for _, field := range fields {
		event = event.Interface(field.Key, field.Value)
	}
	event.Msg(msg)
}

// Error logs an error message
func (l *ZerologLogger) Error(msg string, fields ...Field) {
	event := l.logger.Error()
	for _, field := range fields {
		event = event.Interface(field.Key, field.Value)
	}
	event.Msg(msg)
}

// With returns a logger with additional fields
func (l *ZerologLogger) With(fields ...Field) Logger {
	ctx := l.logger.With()
	for _, field := range fields {
		ctx = ctx.Interface(field.Key, field.Value)
	}
	return &ZerologLogger{
		logger: ctx.Logger(),
	}
}

// WithContext returns a logger with context
func (l *ZerologLogger) WithContext(ctx context.Context) Logger {
	// Extract request ID and other context values
	newLogger := l.logger

	if requestID := ctx.Value("request_id"); requestID != nil {
		newLogger = newLogger.With().Str("request_id", requestID.(string)).Logger()
	}

	if taskID := ctx.Value("task_id"); taskID != nil {
		newLogger = newLogger.With().Str("task_id", taskID.(string)).Logger()
	}

	if agentID := ctx.Value("agent_id"); agentID != nil {
		newLogger = newLogger.With().Str("agent_id", agentID.(string)).Logger()
	}

	return &ZerologLogger{
		logger: newLogger,
	}
}

// Helper functions for common field types

// String creates a string field
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an int field
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 creates an int64 field
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Float64 creates a float64 field
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a bool field
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Err creates an error field
func Err(err error) Field {
	return Field{Key: "error", Value: err.Error()}
}

// Duration creates a duration field
func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value.String()}
}

// Any creates a field with any value
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// NoOpLogger is a logger that does nothing (for testing/benchmarks)
type NoOpLogger struct{}

// NewNoOpLogger creates a no-op logger
func NewNoOpLogger() Logger {
	return &NoOpLogger{}
}

func (l *NoOpLogger) Debug(msg string, fields ...Field) {}
func (l *NoOpLogger) Info(msg string, fields ...Field)  {}
func (l *NoOpLogger) Warn(msg string, fields ...Field)  {}
func (l *NoOpLogger) Error(msg string, fields ...Field) {}
func (l *NoOpLogger) With(fields ...Field) Logger       { return l }
func (l *NoOpLogger) WithContext(ctx context.Context) Logger { return l }
