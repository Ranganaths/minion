package llm

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// FailoverProvider wraps multiple providers with automatic failover capabilities
type FailoverProvider struct {
	providers     []Provider
	healthMonitor *HealthMonitor
	strategy      FailoverStrategy
	metrics       *FailoverMetrics
	retryConfig   *RetryConfig
	mu            sync.RWMutex
}

// FailoverConfig configures the failover provider
type FailoverConfig struct {
	// Providers are the LLM providers in priority order
	Providers []Provider

	// Strategy determines how to select providers
	Strategy FailoverStrategy

	// HealthCheckInterval is how often to check provider health
	HealthCheckInterval time.Duration

	// RetryConfig configures retry behavior
	RetryConfig *RetryConfig

	// EnableMetrics enables failover metrics collection
	EnableMetrics bool
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetryableErrors []string // Error substrings that trigger retry
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []string{
			"rate limit",
			"timeout",
			"connection refused",
			"temporary failure",
			"503",
			"502",
			"429",
		},
	}
}

// FailoverMetrics tracks failover statistics
type FailoverMetrics struct {
	TotalRequests     int64
	FailedRequests    int64
	FailoverCount     int64
	ProviderLatencies map[string][]time.Duration
	ProviderErrors    map[string]int64
	mu                sync.RWMutex
}

// NewFailoverMetrics creates new failover metrics
func NewFailoverMetrics() *FailoverMetrics {
	return &FailoverMetrics{
		ProviderLatencies: make(map[string][]time.Duration),
		ProviderErrors:    make(map[string]int64),
	}
}

// RecordRequest records a request
func (m *FailoverMetrics) RecordRequest() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalRequests++
}

// RecordFailure records a failure
func (m *FailoverMetrics) RecordFailure(providerName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailedRequests++
	m.ProviderErrors[providerName]++
}

// RecordFailover records a failover event
func (m *FailoverMetrics) RecordFailover() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailoverCount++
}

// RecordLatency records provider latency
func (m *FailoverMetrics) RecordLatency(providerName string, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	latencies := m.ProviderLatencies[providerName]
	// Keep only last 100 measurements
	if len(latencies) >= 100 {
		latencies = latencies[1:]
	}
	m.ProviderLatencies[providerName] = append(latencies, latency)
}

// GetStats returns current metrics
func (m *FailoverMetrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	avgLatencies := make(map[string]time.Duration)
	for provider, latencies := range m.ProviderLatencies {
		if len(latencies) > 0 {
			var total time.Duration
			for _, l := range latencies {
				total += l
			}
			avgLatencies[provider] = total / time.Duration(len(latencies))
		}
	}

	return map[string]interface{}{
		"total_requests":  m.TotalRequests,
		"failed_requests": m.FailedRequests,
		"failover_count":  m.FailoverCount,
		"provider_errors": m.ProviderErrors,
		"avg_latencies":   avgLatencies,
	}
}

// NewFailoverProvider creates a new failover provider
func NewFailoverProvider(config FailoverConfig) (*FailoverProvider, error) {
	if len(config.Providers) == 0 {
		return nil, errors.New("at least one provider is required")
	}

	if config.Strategy == nil {
		config.Strategy = NewPriorityFailover()
	}

	if config.RetryConfig == nil {
		config.RetryConfig = DefaultRetryConfig()
	}

	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 30 * time.Second
	}

	fp := &FailoverProvider{
		providers:   config.Providers,
		strategy:    config.Strategy,
		retryConfig: config.RetryConfig,
	}

	if config.EnableMetrics {
		fp.metrics = NewFailoverMetrics()
	}

	// Create health monitor
	fp.healthMonitor = NewHealthMonitor(config.Providers, config.HealthCheckInterval)

	return fp, nil
}

// Start starts the health monitor
func (f *FailoverProvider) Start(ctx context.Context) {
	f.healthMonitor.Start(ctx)
}

// Stop stops the health monitor
func (f *FailoverProvider) Stop() {
	f.healthMonitor.Stop()
}

// GenerateCompletion generates a completion with failover
func (f *FailoverProvider) GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	if f.metrics != nil {
		f.metrics.RecordRequest()
	}

	healthyProviders := f.healthMonitor.GetHealthyProviders()
	if len(healthyProviders) == 0 {
		// Fall back to all providers if none healthy
		healthyProviders = f.providers
	}

	var lastErr error
	attemptedProviders := make(map[string]bool)

	for attempt := 0; attempt <= f.retryConfig.MaxRetries; attempt++ {
		// Select provider
		provider, err := f.strategy.SelectProvider(ctx, healthyProviders, attemptedProviders)
		if err != nil {
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, err
		}

		providerName := provider.Name()
		attemptedProviders[providerName] = true

		// Execute with timeout
		start := time.Now()
		resp, err := provider.GenerateCompletion(ctx, req)
		latency := time.Since(start)

		if f.metrics != nil {
			f.metrics.RecordLatency(providerName, latency)
		}

		if err == nil {
			f.strategy.OnSuccess(provider, latency)
			return resp, nil
		}

		lastErr = err
		f.strategy.OnFailure(provider, err)

		if f.metrics != nil {
			f.metrics.RecordFailure(providerName)
			f.metrics.RecordFailover()
		}

		// Check if error is retryable
		if !f.isRetryable(err) {
			return nil, err
		}

		// Apply backoff delay
		delay := f.calculateBackoff(attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
}

// GenerateChat generates a chat response with failover
func (f *FailoverProvider) GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if f.metrics != nil {
		f.metrics.RecordRequest()
	}

	healthyProviders := f.healthMonitor.GetHealthyProviders()
	if len(healthyProviders) == 0 {
		healthyProviders = f.providers
	}

	var lastErr error
	attemptedProviders := make(map[string]bool)

	for attempt := 0; attempt <= f.retryConfig.MaxRetries; attempt++ {
		provider, err := f.strategy.SelectProvider(ctx, healthyProviders, attemptedProviders)
		if err != nil {
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, err
		}

		providerName := provider.Name()
		attemptedProviders[providerName] = true

		start := time.Now()
		resp, err := provider.GenerateChat(ctx, req)
		latency := time.Since(start)

		if f.metrics != nil {
			f.metrics.RecordLatency(providerName, latency)
		}

		if err == nil {
			f.strategy.OnSuccess(provider, latency)
			return resp, nil
		}

		lastErr = err
		f.strategy.OnFailure(provider, err)

		if f.metrics != nil {
			f.metrics.RecordFailure(providerName)
			f.metrics.RecordFailover()
		}

		if !f.isRetryable(err) {
			return nil, err
		}

		delay := f.calculateBackoff(attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
}

// Name returns the provider name
func (f *FailoverProvider) Name() string {
	return "failover"
}

// GetMetrics returns current metrics
func (f *FailoverProvider) GetMetrics() map[string]interface{} {
	if f.metrics == nil {
		return nil
	}
	return f.metrics.GetStats()
}

// GetHealthStatus returns the health status of all providers
func (f *FailoverProvider) GetHealthStatus() map[string]*ProviderHealth {
	return f.healthMonitor.GetAllHealth()
}

// isRetryable checks if an error is retryable
func (f *FailoverProvider) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	for _, retryable := range f.retryConfig.RetryableErrors {
		if containsIgnoreCase(errStr, retryable) {
			return true
		}
	}

	return false
}

// calculateBackoff calculates the backoff delay for a given attempt
func (f *FailoverProvider) calculateBackoff(attempt int) time.Duration {
	delay := float64(f.retryConfig.InitialDelay)
	for i := 0; i < attempt; i++ {
		delay *= f.retryConfig.BackoffFactor
	}

	if delay > float64(f.retryConfig.MaxDelay) {
		delay = float64(f.retryConfig.MaxDelay)
	}

	return time.Duration(delay)
}

// containsIgnoreCase checks if a string contains a substring (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalsFoldSlice(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalsFoldSlice(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// Ensure FailoverProvider implements Provider
var _ Provider = (*FailoverProvider)(nil)
