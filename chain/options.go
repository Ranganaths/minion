package chain

import (
	"time"
)

// Options configures chain behavior
type Options struct {
	// Callbacks for observability
	Callbacks []ChainCallback

	// Verbose enables detailed logging
	Verbose bool

	// MaxRetries for failed operations
	MaxRetries int

	// Timeout for chain execution
	Timeout time.Duration

	// Metadata for tracing and logging
	Metadata map[string]any

	// StopSequences to halt LLM generation
	StopSequences []string

	// Temperature for LLM calls (overrides default)
	Temperature *float64

	// MaxTokens for LLM calls (overrides default)
	MaxTokens *int
}

// Option is a function that modifies Options
type Option func(*Options)

// DefaultOptions returns default chain options
func DefaultOptions() *Options {
	return &Options{
		Callbacks:  make([]ChainCallback, 0),
		Verbose:    false,
		MaxRetries: 3,
		Timeout:    60 * time.Second,
		Metadata:   make(map[string]any),
	}
}

// WithCallbacks adds callbacks for observability
func WithCallbacks(callbacks ...ChainCallback) Option {
	return func(o *Options) {
		o.Callbacks = append(o.Callbacks, callbacks...)
	}
}

// WithVerbose enables verbose logging
func WithVerbose(verbose bool) Option {
	return func(o *Options) {
		o.Verbose = verbose
	}
}

// WithMaxRetries sets the maximum number of retries
func WithMaxRetries(retries int) Option {
	return func(o *Options) {
		o.MaxRetries = retries
	}
}

// WithTimeout sets the execution timeout
func WithTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.Timeout = timeout
	}
}

// WithMetadata adds metadata for tracing
func WithMetadata(metadata map[string]any) Option {
	return func(o *Options) {
		for k, v := range metadata {
			o.Metadata[k] = v
		}
	}
}

// WithStopSequences sets stop sequences for LLM generation
func WithStopSequences(sequences ...string) Option {
	return func(o *Options) {
		o.StopSequences = sequences
	}
}

// WithTemperature overrides the LLM temperature
func WithTemperature(temp float64) Option {
	return func(o *Options) {
		o.Temperature = &temp
	}
}

// WithMaxTokens overrides the max tokens for LLM
func WithMaxTokens(tokens int) Option {
	return func(o *Options) {
		o.MaxTokens = &tokens
	}
}

// ApplyOptions applies all options to a base Options struct
func ApplyOptions(opts ...Option) *Options {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}
	return options
}
