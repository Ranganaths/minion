package client

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// HealthCheck represents the result of a health check
type HealthCheck struct {
	Status      HealthStatus
	Message     string
	LastChecked time.Time
	Details     map[string]interface{}
}

// HealthChecker performs health checks on MCP clients
type HealthChecker struct {
	manager    *MCPClientManager
	interval   time.Duration
	mu         sync.RWMutex
	lastChecks map[string]*HealthCheck
	stopChan   chan struct{}
	running    bool
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(manager *MCPClientManager, interval time.Duration) *HealthChecker {
	if interval == 0 {
		interval = 30 * time.Second
	}

	return &HealthChecker{
		manager:    manager,
		interval:   interval,
		lastChecks: make(map[string]*HealthCheck),
		stopChan:   make(chan struct{}),
	}
}

// Start begins periodic health checks
func (h *HealthChecker) Start(ctx context.Context) {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return
	}
	h.running = true
	h.mu.Unlock()

	go h.healthCheckLoop(ctx)
}

// Stop stops the health checker
func (h *HealthChecker) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	h.running = false
	close(h.stopChan)
}

// healthCheckLoop runs periodic health checks
func (h *HealthChecker) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	// Initial check
	h.performChecks(ctx)

	for {
		select {
		case <-ticker.C:
			h.performChecks(ctx)
		case <-h.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// performChecks performs health checks on all connected servers
func (h *HealthChecker) performChecks(ctx context.Context) {
	servers := h.manager.ListServers()

	for _, serverName := range servers {
		check := h.checkServer(ctx, serverName)

		h.mu.Lock()
		h.lastChecks[serverName] = check
		h.mu.Unlock()
	}
}

// checkServer performs a health check on a specific server
func (h *HealthChecker) checkServer(ctx context.Context, serverName string) *HealthCheck {
	client, err := h.manager.GetClient(serverName)
	if err != nil {
		return &HealthCheck{
			Status:      HealthStatusUnhealthy,
			Message:     fmt.Sprintf("Client not found: %v", err),
			LastChecked: time.Now(),
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	// Check if connected
	if !client.IsConnected() {
		return &HealthCheck{
			Status:      HealthStatusUnhealthy,
			Message:     "Client is disconnected",
			LastChecked: time.Now(),
			Details: map[string]interface{}{
				"connected": false,
			},
		}
	}

	// Get client status
	status := client.GetStatus()

	// Determine health based on metrics
	var healthStatus HealthStatus
	var message string

	// Calculate error rate
	var errorRate float64
	if status.TotalCalls > 0 {
		errorRate = float64(status.FailedCalls) / float64(status.TotalCalls)
	}

	// Health criteria
	if errorRate > 0.5 {
		// More than 50% errors
		healthStatus = HealthStatusUnhealthy
		message = fmt.Sprintf("High error rate: %.1f%%", errorRate*100)
	} else if errorRate > 0.2 {
		// More than 20% errors
		healthStatus = HealthStatusDegraded
		message = fmt.Sprintf("Elevated error rate: %.1f%%", errorRate*100)
	} else if status.ToolsDiscovered == 0 {
		// No tools discovered
		healthStatus = HealthStatusDegraded
		message = "No tools discovered"
	} else {
		healthStatus = HealthStatusHealthy
		message = "All checks passed"
	}

	// Check last error time
	if !status.LastErrorTime.IsZero() {
		timeSinceError := time.Since(status.LastErrorTime)
		if timeSinceError < 1*time.Minute {
			if healthStatus == HealthStatusHealthy {
				healthStatus = HealthStatusDegraded
			}
			message = fmt.Sprintf("%s (recent error: %s)", message, status.LastError)
		}
	}

	return &HealthCheck{
		Status:      healthStatus,
		Message:     message,
		LastChecked: time.Now(),
		Details: map[string]interface{}{
			"connected":        status.Connected,
			"tools_discovered": status.ToolsDiscovered,
			"total_calls":      status.TotalCalls,
			"success_calls":    status.SuccessCalls,
			"failed_calls":     status.FailedCalls,
			"error_rate":       errorRate,
			"last_error":       status.LastError,
			"last_error_time":  status.LastErrorTime,
		},
	}
}

// GetHealth returns the latest health check for a server
func (h *HealthChecker) GetHealth(serverName string) *HealthCheck {
	h.mu.RLock()
	defer h.mu.RUnlock()

	check, exists := h.lastChecks[serverName]
	if !exists {
		return &HealthCheck{
			Status:      HealthStatusUnknown,
			Message:     "No health check performed yet",
			LastChecked: time.Time{},
			Details:     map[string]interface{}{},
		}
	}

	return check
}

// GetAllHealth returns health checks for all servers
func (h *HealthChecker) GetAllHealth() map[string]*HealthCheck {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Return a copy
	result := make(map[string]*HealthCheck)
	for name, check := range h.lastChecks {
		result[name] = check
	}

	return result
}

// IsHealthy returns true if the server is healthy
func (h *HealthChecker) IsHealthy(serverName string) bool {
	check := h.GetHealth(serverName)
	return check.Status == HealthStatusHealthy
}

// AllHealthy returns true if all servers are healthy
func (h *HealthChecker) AllHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, check := range h.lastChecks {
		if check.Status != HealthStatusHealthy {
			return false
		}
	}

	return true
}

// CheckNow performs an immediate health check on a specific server
func (h *HealthChecker) CheckNow(ctx context.Context, serverName string) *HealthCheck {
	check := h.checkServer(ctx, serverName)

	h.mu.Lock()
	h.lastChecks[serverName] = check
	h.mu.Unlock()

	return check
}

// GetUnhealthyServers returns a list of unhealthy server names
func (h *HealthChecker) GetUnhealthyServers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	unhealthy := []string{}
	for name, check := range h.lastChecks {
		if check.Status == HealthStatusUnhealthy {
			unhealthy = append(unhealthy, name)
		}
	}

	return unhealthy
}

// GetDegradedServers returns a list of degraded server names
func (h *HealthChecker) GetDegradedServers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	degraded := []string{}
	for name, check := range h.lastChecks {
		if check.Status == HealthStatusDegraded {
			degraded = append(degraded, name)
		}
	}

	return degraded
}

// Summary returns a summary of health across all servers
func (h *HealthChecker) Summary() map[string]int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	summary := map[string]int{
		"healthy":   0,
		"degraded":  0,
		"unhealthy": 0,
		"unknown":   0,
	}

	for _, check := range h.lastChecks {
		switch check.Status {
		case HealthStatusHealthy:
			summary["healthy"]++
		case HealthStatusDegraded:
			summary["degraded"]++
		case HealthStatusUnhealthy:
			summary["unhealthy"]++
		case HealthStatusUnknown:
			summary["unknown"]++
		}
	}

	return summary
}
