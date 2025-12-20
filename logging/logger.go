// Package logging provides a structured logging interface for the minion framework.
// This package allows users to integrate their preferred logging framework.
package logging

import (
	"context"
	"log"
	"os"
	"sync"
)

// Level represents the logging level
type Level int

const (
	// LevelDebug is for debug messages
	LevelDebug Level = iota
	// LevelInfo is for informational messages
	LevelInfo
	// LevelWarn is for warning messages
	LevelWarn
	// LevelError is for error messages
	LevelError
)

// String returns the string representation of the level
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Field represents a structured log field
type Field struct {
	Key   string
	Value any
}

// F creates a new field
func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// Logger is the interface for structured logging
type Logger interface {
	// Debug logs a debug message
	Debug(ctx context.Context, msg string, fields ...Field)

	// Info logs an informational message
	Info(ctx context.Context, msg string, fields ...Field)

	// Warn logs a warning message
	Warn(ctx context.Context, msg string, fields ...Field)

	// Error logs an error message
	Error(ctx context.Context, msg string, fields ...Field)

	// WithFields returns a logger with preset fields
	WithFields(fields ...Field) Logger

	// WithName returns a logger with a name prefix
	WithName(name string) Logger
}

// defaultLogger is the package-level logger
var (
	defaultLogger Logger = NewStdLogger(LevelInfo)
	loggerMu      sync.RWMutex
)

// SetLogger sets the global logger
func SetLogger(l Logger) {
	loggerMu.Lock()
	defer loggerMu.Unlock()
	defaultLogger = l
}

// GetLogger returns the global logger
func GetLogger() Logger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return defaultLogger
}

// Debug logs a debug message using the global logger
func Debug(ctx context.Context, msg string, fields ...Field) {
	GetLogger().Debug(ctx, msg, fields...)
}

// Info logs an informational message using the global logger
func Info(ctx context.Context, msg string, fields ...Field) {
	GetLogger().Info(ctx, msg, fields...)
}

// Warn logs a warning message using the global logger
func Warn(ctx context.Context, msg string, fields ...Field) {
	GetLogger().Warn(ctx, msg, fields...)
}

// Error logs an error message using the global logger
func Error(ctx context.Context, msg string, fields ...Field) {
	GetLogger().Error(ctx, msg, fields...)
}

// StdLogger is a simple logger that uses the standard library
type StdLogger struct {
	level  Level
	logger *log.Logger
	fields []Field
	name   string
}

// NewStdLogger creates a new standard library logger
func NewStdLogger(level Level) *StdLogger {
	return &StdLogger{
		level:  level,
		logger: log.New(os.Stderr, "", log.LstdFlags),
	}
}

// NewStdLoggerWithOutput creates a logger with custom output
func NewStdLoggerWithOutput(level Level, logger *log.Logger) *StdLogger {
	return &StdLogger{
		level:  level,
		logger: logger,
	}
}

func (l *StdLogger) log(level Level, ctx context.Context, msg string, fields ...Field) {
	if level < l.level {
		return
	}

	// Combine preset fields with provided fields
	allFields := make([]Field, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)

	// Build log message
	prefix := ""
	if l.name != "" {
		prefix = "[" + l.name + "] "
	}

	if len(allFields) == 0 {
		l.logger.Printf("%s %s%s", level.String(), prefix, msg)
	} else {
		l.logger.Printf("%s %s%s %v", level.String(), prefix, msg, fieldsToMap(allFields))
	}
}

// Debug logs a debug message
func (l *StdLogger) Debug(ctx context.Context, msg string, fields ...Field) {
	l.log(LevelDebug, ctx, msg, fields...)
}

// Info logs an informational message
func (l *StdLogger) Info(ctx context.Context, msg string, fields ...Field) {
	l.log(LevelInfo, ctx, msg, fields...)
}

// Warn logs a warning message
func (l *StdLogger) Warn(ctx context.Context, msg string, fields ...Field) {
	l.log(LevelWarn, ctx, msg, fields...)
}

// Error logs an error message
func (l *StdLogger) Error(ctx context.Context, msg string, fields ...Field) {
	l.log(LevelError, ctx, msg, fields...)
}

// WithFields returns a logger with preset fields
func (l *StdLogger) WithFields(fields ...Field) Logger {
	newFields := make([]Field, len(l.fields)+len(fields))
	copy(newFields, l.fields)
	copy(newFields[len(l.fields):], fields)

	return &StdLogger{
		level:  l.level,
		logger: l.logger,
		fields: newFields,
		name:   l.name,
	}
}

// WithName returns a logger with a name prefix
func (l *StdLogger) WithName(name string) Logger {
	newName := name
	if l.name != "" {
		newName = l.name + "." + name
	}

	return &StdLogger{
		level:  l.level,
		logger: l.logger,
		fields: l.fields,
		name:   newName,
	}
}

func fieldsToMap(fields []Field) map[string]any {
	m := make(map[string]any, len(fields))
	for _, f := range fields {
		m[f.Key] = f.Value
	}
	return m
}

// NopLogger is a logger that does nothing
type NopLogger struct{}

// NewNopLogger creates a no-op logger
func NewNopLogger() *NopLogger {
	return &NopLogger{}
}

func (l *NopLogger) Debug(ctx context.Context, msg string, fields ...Field) {}
func (l *NopLogger) Info(ctx context.Context, msg string, fields ...Field)  {}
func (l *NopLogger) Warn(ctx context.Context, msg string, fields ...Field)  {}
func (l *NopLogger) Error(ctx context.Context, msg string, fields ...Field) {}
func (l *NopLogger) WithFields(fields ...Field) Logger                      { return l }
func (l *NopLogger) WithName(name string) Logger                            { return l }
