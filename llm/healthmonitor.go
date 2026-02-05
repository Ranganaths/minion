package llm

import (
	"context"
	"sync"
	"time"
)

// ProviderHealth tracks the health status of a provider
type ProviderHealth struct {
	Provider         Provider
	Healthy          bool
	ConsecutiveFails int
	ConsecutiveOK    int
	LastCheck        time.Time
	LastError        error
	AvgLatency       time.Duration
	ErrorRate        float64
	TotalRequests    int64
	FailedRequests   int64
	latencies        []time.Duration
	mu               sync.RWMutex
}

// HealthMonitor monitors the health of multiple providers
type HealthMonitor struct {
	providers         []Provider
	health            map[string]*ProviderHealth
	checkInterval     time.Duration
	unhealthyThreshold int
	recoveryThreshold  int
	ctx               context.Context
	cancel            context.CancelFunc
	mu                sync.RWMutex
}

// HealthMonitorConfig configures the health monitor
type HealthMonitorConfig struct {
	// CheckInterval is how often to check provider health
	CheckInterval time.Duration

	// UnhealthyThreshold is how many consecutive failures mark a provider as unhealthy
	UnhealthyThreshold int

	// RecoveryThreshold is how many consecutive successes mark a provider as recovered
	RecoveryThreshold int
}

// DefaultHealthMonitorConfig returns default configuration
func DefaultHealthMonitorConfig() *HealthMonitorConfig {
	return &HealthMonitorConfig{
		CheckInterval:      30 * time.Second,
		UnhealthyThreshold: 3,
		RecoveryThreshold:  2,
	}
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(providers []Provider, checkInterval time.Duration) *HealthMonitor {
	if checkInterval == 0 {
		checkInterval = 30 * time.Second
	}

	health := make(map[string]*ProviderHealth)
	for _, p := range providers {
		health[p.Name()] = &ProviderHealth{
			Provider:  p,
			Healthy:   true, // Assume healthy initially
			latencies: make([]time.Duration, 0, 100),
		}
	}

	return &HealthMonitor{
		providers:          providers,
		health:             health,
		checkInterval:      checkInterval,
		unhealthyThreshold: 3,
		recoveryThreshold:  2,
	}
}

// NewHealthMonitorWithConfig creates a health monitor with custom config
func NewHealthMonitorWithConfig(providers []Provider, config *HealthMonitorConfig) *HealthMonitor {
	if config == nil {
		config = DefaultHealthMonitorConfig()
	}

	health := make(map[string]*ProviderHealth)
	for _, p := range providers {
		health[p.Name()] = &ProviderHealth{
			Provider:  p,
			Healthy:   true,
			latencies: make([]time.Duration, 0, 100),
		}
	}

	return &HealthMonitor{
		providers:          providers,
		health:             health,
		checkInterval:      config.CheckInterval,
		unhealthyThreshold: config.UnhealthyThreshold,
		recoveryThreshold:  config.RecoveryThreshold,
	}
}

// Start begins periodic health checks
func (m *HealthMonitor) Start(ctx context.Context) {
	m.mu.Lock()
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.mu.Unlock()

	go m.healthCheckLoop()
}

// Stop stops the health monitor
func (m *HealthMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
	}
}

// healthCheckLoop runs periodic health checks
func (m *HealthMonitor) healthCheckLoop() {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	// Initial check
	m.checkAllProviders()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAllProviders()
		}
	}
}

// checkAllProviders checks the health of all providers
func (m *HealthMonitor) checkAllProviders() {
	var wg sync.WaitGroup

	for _, provider := range m.providers {
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()
			m.checkProvider(p)
		}(provider)
	}

	wg.Wait()
}

// checkProvider checks the health of a single provider
func (m *HealthMonitor) checkProvider(provider Provider) {
	m.mu.RLock()
	health := m.health[provider.Name()]
	m.mu.RUnlock()

	if health == nil {
		return
	}

	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	var err error
	start := time.Now()

	// Check if provider supports health check
	if hcp, ok := provider.(HealthCheckProvider); ok {
		err = hcp.HealthCheck(ctx)
	} else {
		// Use a simple completion request as health check
		_, err = provider.GenerateCompletion(ctx, &CompletionRequest{
			Model:      "gpt-3.5-turbo", // Use a cheap model
			UserPrompt: "ping",
			MaxTokens:  1,
		})
	}

	latency := time.Since(start)

	health.mu.Lock()
	defer health.mu.Unlock()

	health.LastCheck = time.Now()
	health.TotalRequests++

	if err != nil {
		health.LastError = err
		health.FailedRequests++
		health.ConsecutiveFails++
		health.ConsecutiveOK = 0

		// Mark unhealthy if threshold exceeded
		if health.ConsecutiveFails >= m.unhealthyThreshold {
			health.Healthy = false
		}
	} else {
		health.LastError = nil
		health.ConsecutiveFails = 0
		health.ConsecutiveOK++

		// Record latency
		if len(health.latencies) >= 100 {
			health.latencies = health.latencies[1:]
		}
		health.latencies = append(health.latencies, latency)

		// Calculate average latency
		var total time.Duration
		for _, l := range health.latencies {
			total += l
		}
		health.AvgLatency = total / time.Duration(len(health.latencies))

		// Mark healthy if recovery threshold met
		if health.ConsecutiveOK >= m.recoveryThreshold {
			health.Healthy = true
		}
	}

	// Calculate error rate
	if health.TotalRequests > 0 {
		health.ErrorRate = float64(health.FailedRequests) / float64(health.TotalRequests)
	}
}

// GetHealthyProviders returns all healthy providers
func (m *HealthMonitor) GetHealthyProviders() []Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	healthy := make([]Provider, 0)
	for _, provider := range m.providers {
		if h := m.health[provider.Name()]; h != nil && h.IsHealthy() {
			healthy = append(healthy, provider)
		}
	}
	return healthy
}

// GetProviderHealth returns the health status of a specific provider
func (m *HealthMonitor) GetProviderHealth(name string) *ProviderHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.health[name]
}

// GetAllHealth returns the health status of all providers
func (m *HealthMonitor) GetAllHealth() map[string]*ProviderHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*ProviderHealth)
	for k, v := range m.health {
		result[k] = v
	}
	return result
}

// MarkHealthy manually marks a provider as healthy
func (m *HealthMonitor) MarkHealthy(name string) {
	m.mu.RLock()
	health := m.health[name]
	m.mu.RUnlock()

	if health != nil {
		health.mu.Lock()
		health.Healthy = true
		health.ConsecutiveFails = 0
		health.mu.Unlock()
	}
}

// MarkUnhealthy manually marks a provider as unhealthy
func (m *HealthMonitor) MarkUnhealthy(name string) {
	m.mu.RLock()
	health := m.health[name]
	m.mu.RUnlock()

	if health != nil {
		health.mu.Lock()
		health.Healthy = false
		health.mu.Unlock()
	}
}

// RecordSuccess records a successful request for a provider
func (m *HealthMonitor) RecordSuccess(name string, latency time.Duration) {
	m.mu.RLock()
	health := m.health[name]
	m.mu.RUnlock()

	if health != nil {
		health.mu.Lock()
		defer health.mu.Unlock()

		health.TotalRequests++
		health.ConsecutiveFails = 0
		health.ConsecutiveOK++

		// Record latency
		if len(health.latencies) >= 100 {
			health.latencies = health.latencies[1:]
		}
		health.latencies = append(health.latencies, latency)

		// Update average
		var total time.Duration
		for _, l := range health.latencies {
			total += l
		}
		health.AvgLatency = total / time.Duration(len(health.latencies))

		// Check recovery
		if health.ConsecutiveOK >= m.recoveryThreshold && !health.Healthy {
			health.Healthy = true
		}
	}
}

// RecordFailure records a failed request for a provider
func (m *HealthMonitor) RecordFailure(name string, err error) {
	m.mu.RLock()
	health := m.health[name]
	m.mu.RUnlock()

	if health != nil {
		health.mu.Lock()
		defer health.mu.Unlock()

		health.TotalRequests++
		health.FailedRequests++
		health.ConsecutiveFails++
		health.ConsecutiveOK = 0
		health.LastError = err

		// Update error rate
		health.ErrorRate = float64(health.FailedRequests) / float64(health.TotalRequests)

		// Check unhealthy threshold
		if health.ConsecutiveFails >= m.unhealthyThreshold {
			health.Healthy = false
		}
	}
}

// IsHealthy returns whether the provider is healthy
func (h *ProviderHealth) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.Healthy
}

// GetStats returns provider health statistics
func (h *ProviderHealth) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return map[string]interface{}{
		"healthy":           h.Healthy,
		"consecutive_fails": h.ConsecutiveFails,
		"consecutive_ok":    h.ConsecutiveOK,
		"last_check":        h.LastCheck,
		"avg_latency":       h.AvgLatency,
		"error_rate":        h.ErrorRate,
		"total_requests":    h.TotalRequests,
		"failed_requests":   h.FailedRequests,
	}
}
