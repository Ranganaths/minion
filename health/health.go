// Package health provides health check functionality for the Minion framework.
// It enables monitoring of service health through configurable health checks
// and readiness/liveness probes.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status of a component.
type Status string

const (
	// StatusHealthy indicates the component is functioning correctly.
	StatusHealthy Status = "healthy"
	// StatusUnhealthy indicates the component is not functioning correctly.
	StatusUnhealthy Status = "unhealthy"
	// StatusDegraded indicates the component is functioning but with issues.
	StatusDegraded Status = "degraded"
	// StatusUnknown indicates the health status cannot be determined.
	StatusUnknown Status = "unknown"
)

// CheckResult represents the result of a health check.
type CheckResult struct {
	Status    Status                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Duration  time.Duration          `json:"duration_ms"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// Check is a function that performs a health check.
// It should return nil for healthy, or an error describing the issue.
type Check func(ctx context.Context) error

// CheckConfig configures a health check.
type CheckConfig struct {
	// Name is the identifier for this health check.
	Name string

	// Check is the function that performs the health check.
	Check Check

	// Timeout is the maximum time allowed for the check.
	// Default: 5 seconds.
	Timeout time.Duration

	// Critical indicates whether this check failing means the service is unhealthy.
	// If false, a failure results in degraded status instead.
	Critical bool
}

// Checker manages multiple health checks.
type Checker struct {
	mu       sync.RWMutex
	checks   map[string]CheckConfig
	results  map[string]CheckResult
	interval time.Duration
	stopCh   chan struct{}
	running  bool
}

// NewChecker creates a new health checker.
func NewChecker() *Checker {
	return &Checker{
		checks:   make(map[string]CheckConfig),
		results:  make(map[string]CheckResult),
		interval: 30 * time.Second,
	}
}

// Register adds a health check to the checker.
func (c *Checker) Register(cfg CheckConfig) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[cfg.Name] = cfg
}

// RegisterFunc is a convenience method to register a check function.
func (c *Checker) RegisterFunc(name string, check Check, critical bool) {
	c.Register(CheckConfig{
		Name:     name,
		Check:    check,
		Critical: critical,
	})
}

// Unregister removes a health check from the checker.
func (c *Checker) Unregister(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.checks, name)
	delete(c.results, name)
}

// Check performs a single health check and returns the result.
func (c *Checker) Check(ctx context.Context, name string) CheckResult {
	c.mu.RLock()
	cfg, ok := c.checks[name]
	c.mu.RUnlock()

	if !ok {
		return CheckResult{
			Status:    StatusUnknown,
			Message:   "check not found",
			Timestamp: time.Now(),
		}
	}

	return c.runCheck(ctx, cfg)
}

// CheckAll performs all registered health checks and returns the results.
func (c *Checker) CheckAll(ctx context.Context) map[string]CheckResult {
	c.mu.RLock()
	checks := make(map[string]CheckConfig, len(c.checks))
	for name, cfg := range c.checks {
		checks[name] = cfg
	}
	c.mu.RUnlock()

	results := make(map[string]CheckResult, len(checks))
	var wg sync.WaitGroup

	resultsMu := sync.Mutex{}

	for name, cfg := range checks {
		wg.Add(1)
		go func(name string, cfg CheckConfig) {
			defer wg.Done()
			result := c.runCheck(ctx, cfg)
			resultsMu.Lock()
			results[name] = result
			resultsMu.Unlock()
		}(name, cfg)
	}

	wg.Wait()

	// Store results
	c.mu.Lock()
	for name, result := range results {
		c.results[name] = result
	}
	c.mu.Unlock()

	return results
}

// runCheck executes a single health check with timeout.
func (c *Checker) runCheck(ctx context.Context, cfg CheckConfig) CheckResult {
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	start := time.Now()

	errCh := make(chan error, 1)
	go func() {
		errCh <- cfg.Check(ctx)
	}()

	var err error
	select {
	case err = <-errCh:
	case <-ctx.Done():
		err = ctx.Err()
	}

	duration := time.Since(start)

	if err != nil {
		status := StatusUnhealthy
		if !cfg.Critical {
			status = StatusDegraded
		}
		return CheckResult{
			Status:    status,
			Message:   err.Error(),
			Duration:  duration,
			Timestamp: time.Now(),
		}
	}

	return CheckResult{
		Status:    StatusHealthy,
		Duration:  duration,
		Timestamp: time.Now(),
	}
}

// OverallStatus returns the overall health status based on all checks.
func (c *Checker) OverallStatus(ctx context.Context) (Status, map[string]CheckResult) {
	results := c.CheckAll(ctx)

	status := StatusHealthy
	for _, result := range results {
		switch result.Status {
		case StatusUnhealthy:
			return StatusUnhealthy, results
		case StatusDegraded:
			status = StatusDegraded
		}
	}

	return status, results
}

// GetLastResults returns the last cached results without running checks.
func (c *Checker) GetLastResults() map[string]CheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make(map[string]CheckResult, len(c.results))
	for name, result := range c.results {
		results[name] = result
	}
	return results
}

// StartBackground starts background health checking at the specified interval.
func (c *Checker) StartBackground(interval time.Duration) {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.interval = interval
	c.stopCh = make(chan struct{})
	c.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Run initial check
		c.CheckAll(context.Background())

		for {
			select {
			case <-ticker.C:
				c.CheckAll(context.Background())
			case <-c.stopCh:
				return
			}
		}
	}()
}

// StopBackground stops background health checking.
func (c *Checker) StopBackground() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running && c.stopCh != nil {
		close(c.stopCh)
		c.running = false
	}
}

// HealthResponse is the JSON response for health endpoints.
type HealthResponse struct {
	Status    Status                  `json:"status"`
	Timestamp time.Time               `json:"timestamp"`
	Checks    map[string]CheckResult  `json:"checks,omitempty"`
	Version   string                  `json:"version,omitempty"`
}

// Handler returns an HTTP handler for health checks.
func (c *Checker) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		status, results := c.OverallStatus(ctx)

		response := HealthResponse{
			Status:    status,
			Timestamp: time.Now(),
			Checks:    results,
		}

		w.Header().Set("Content-Type", "application/json")

		switch status {
		case StatusHealthy:
			w.WriteHeader(http.StatusOK)
		case StatusDegraded:
			w.WriteHeader(http.StatusOK) // Still OK, just degraded
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(response)
	})
}

// LivenessHandler returns an HTTP handler for liveness probes.
// This is a simple check that the service is running.
func (c *Checker) LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := HealthResponse{
			Status:    StatusHealthy,
			Timestamp: time.Now(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})
}

// ReadinessHandler returns an HTTP handler for readiness probes.
// This checks if the service is ready to accept traffic.
func (c *Checker) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		status, results := c.OverallStatus(ctx)

		response := HealthResponse{
			Status:    status,
			Timestamp: time.Now(),
			Checks:    results,
		}

		w.Header().Set("Content-Type", "application/json")

		if status == StatusHealthy || status == StatusDegraded {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(response)
	})
}

// Common health checks

// PingCheck creates a simple ping health check.
func PingCheck() Check {
	return func(ctx context.Context) error {
		return nil
	}
}

// HTTPCheck creates a health check that makes an HTTP GET request.
func HTTPCheck(url string, timeout time.Duration) Check {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			return &HTTPError{StatusCode: resp.StatusCode}
		}

		return nil
	}
}

// HTTPError represents an HTTP error response.
type HTTPError struct {
	StatusCode int
}

func (e *HTTPError) Error() string {
	return http.StatusText(e.StatusCode)
}

// ThresholdCheck creates a check that fails if a value exceeds a threshold.
func ThresholdCheck(name string, getValue func() float64, threshold float64) Check {
	return func(ctx context.Context) error {
		value := getValue()
		if value > threshold {
			return &ThresholdError{Name: name, Value: value, Threshold: threshold}
		}
		return nil
	}
}

// ThresholdError represents a threshold violation.
type ThresholdError struct {
	Name      string
	Value     float64
	Threshold float64
}

func (e *ThresholdError) Error() string {
	return e.Name + " exceeded threshold"
}

// CircuitBreakerCheck creates a health check for a circuit breaker.
// It returns unhealthy if the circuit is open.
func CircuitBreakerCheck(name string, getState func() string) Check {
	return func(ctx context.Context) error {
		state := getState()
		if state == "open" {
			return &CircuitOpenError{Name: name}
		}
		return nil
	}
}

// CircuitOpenError represents a circuit breaker being open.
type CircuitOpenError struct {
	Name string
}

func (e *CircuitOpenError) Error() string {
	return "circuit breaker " + e.Name + " is open"
}
