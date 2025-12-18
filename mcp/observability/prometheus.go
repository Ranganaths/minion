package observability

import (
	"fmt"
	"sync"
	"time"

	"github.com/Ranganaths/minion/mcp/client"
)

// PrometheusMetrics exports MCP metrics in Prometheus format
type PrometheusMetrics struct {
	manager *client.MCPClientManager
	cache   *client.ToolCache
	pool    *client.ConnectionPool
	mu      sync.RWMutex

	// Metric storage
	metrics map[string]float64
	labels  map[string]map[string]string
}

// NewPrometheusMetrics creates a new Prometheus metrics exporter
func NewPrometheusMetrics(manager *client.MCPClientManager) *PrometheusMetrics {
	return &PrometheusMetrics{
		manager: manager,
		metrics: make(map[string]float64),
		labels:  make(map[string]map[string]string),
	}
}

// WithCache adds cache metrics
func (pm *PrometheusMetrics) WithCache(cache *client.ToolCache) *PrometheusMetrics {
	pm.cache = cache
	return pm
}

// WithPool adds pool metrics
func (pm *PrometheusMetrics) WithPool(pool *client.ConnectionPool) *PrometheusMetrics{
	pm.pool = pool
	return pm
}

// Collect gathers all metrics
func (pm *PrometheusMetrics) Collect() MetricsSnapshot {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	snapshot := MetricsSnapshot{
		Timestamp: time.Now(),
		Clients:   make(map[string]ClientMetricsSnapshot),
	}

	// Collect client metrics
	if pm.manager != nil {
		status := pm.manager.GetStatus()
		for serverName, clientStatus := range status {
			snapshot.Clients[serverName] = ClientMetricsSnapshot{
				ServerName:       serverName,
				Connected:        boolToFloat(clientStatus.Connected),
				ToolsDiscovered:  float64(clientStatus.ToolsDiscovered),
				TotalCalls:       float64(clientStatus.TotalCalls),
				SuccessCalls:     float64(clientStatus.SuccessCalls),
				FailedCalls:      float64(clientStatus.FailedCalls),
				ErrorRate:        calculateErrorRate(clientStatus),
			}
		}
	}

	// Collect cache metrics
	if pm.cache != nil {
		cacheMetrics := pm.cache.GetMetrics()
		snapshot.Cache = CacheMetricsSnapshot{
			Hits:        float64(cacheMetrics.Hits),
			Misses:      float64(cacheMetrics.Misses),
			HitRate:     cacheMetrics.HitRate,
			Evictions:   float64(cacheMetrics.Evictions),
			CurrentSize: float64(cacheMetrics.CurrentSize),
		}
	}

	// Collect pool metrics
	if pm.pool != nil {
		poolMetrics := pm.pool.GetMetrics()
		snapshot.Pool = PoolMetricsSnapshot{
			TotalConns:   float64(poolMetrics.TotalConns),
			IdleConns:    float64(poolMetrics.IdleConns),
			ActiveConns:  float64(poolMetrics.ActiveConns),
			WaitCount:    float64(poolMetrics.WaitCount),
			WaitDuration: poolMetrics.WaitDuration.Seconds(),
		}
	}

	return snapshot
}

// MetricsSnapshot contains a point-in-time snapshot of all metrics
type MetricsSnapshot struct {
	Timestamp time.Time
	Clients   map[string]ClientMetricsSnapshot
	Cache     CacheMetricsSnapshot
	Pool      PoolMetricsSnapshot
}

// ClientMetricsSnapshot contains metrics for a single client
type ClientMetricsSnapshot struct {
	ServerName      string
	Connected       float64
	ToolsDiscovered float64
	TotalCalls      float64
	SuccessCalls    float64
	FailedCalls     float64
	ErrorRate       float64
}

// CacheMetricsSnapshot contains cache metrics
type CacheMetricsSnapshot struct {
	Hits        float64
	Misses      float64
	HitRate     float64
	Evictions   float64
	CurrentSize float64
}

// PoolMetricsSnapshot contains pool metrics
type PoolMetricsSnapshot struct {
	TotalConns   float64
	IdleConns    float64
	ActiveConns  float64
	WaitCount    float64
	WaitDuration float64 // seconds
}

// ToPrometheusFormat converts metrics to Prometheus exposition format
func (snapshot *MetricsSnapshot) ToPrometheusFormat() string {
	output := ""

	// Client metrics
	for serverName, client := range snapshot.Clients {
		output += formatMetric("mcp_client_connected", client.Connected, map[string]string{"server": serverName})
		output += formatMetric("mcp_client_tools_discovered", client.ToolsDiscovered, map[string]string{"server": serverName})
		output += formatMetric("mcp_client_calls_total", client.TotalCalls, map[string]string{"server": serverName})
		output += formatMetric("mcp_client_calls_success", client.SuccessCalls, map[string]string{"server": serverName})
		output += formatMetric("mcp_client_calls_failed", client.FailedCalls, map[string]string{"server": serverName})
		output += formatMetric("mcp_client_error_rate", client.ErrorRate, map[string]string{"server": serverName})
	}

	// Cache metrics
	output += formatMetric("mcp_cache_hits_total", snapshot.Cache.Hits, nil)
	output += formatMetric("mcp_cache_misses_total", snapshot.Cache.Misses, nil)
	output += formatMetric("mcp_cache_hit_rate", snapshot.Cache.HitRate, nil)
	output += formatMetric("mcp_cache_evictions_total", snapshot.Cache.Evictions, nil)
	output += formatMetric("mcp_cache_size", snapshot.Cache.CurrentSize, nil)

	// Pool metrics
	output += formatMetric("mcp_pool_connections_total", snapshot.Pool.TotalConns, nil)
	output += formatMetric("mcp_pool_connections_idle", snapshot.Pool.IdleConns, nil)
	output += formatMetric("mcp_pool_connections_active", snapshot.Pool.ActiveConns, nil)
	output += formatMetric("mcp_pool_wait_count", snapshot.Pool.WaitCount, nil)
	output += formatMetric("mcp_pool_wait_duration_seconds", snapshot.Pool.WaitDuration, nil)

	return output
}

// formatMetric formats a single metric in Prometheus format
func formatMetric(name string, value float64, labels map[string]string) string {
	if labels == nil || len(labels) == 0 {
		return fmt.Sprintf("%s %f\n", name, value)
	}

	labelStr := ""
	first := true
	for k, v := range labels {
		if !first {
			labelStr += ","
		}
		labelStr += fmt.Sprintf("%s=\"%s\"", k, v)
		first = false
	}

	return fmt.Sprintf("%s{%s} %f\n", name, labelStr, value)
}

// ToJSON converts metrics to JSON format
func (snapshot *MetricsSnapshot) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"timestamp": snapshot.Timestamp.Unix(),
		"clients":   snapshot.Clients,
		"cache":     snapshot.Cache,
		"pool":      snapshot.Pool,
	}
}

// MetricsCollector continuously collects and stores metrics
type MetricsCollector struct {
	prometheus *PrometheusMetrics
	interval   time.Duration
	stopChan   chan struct{}
	running    bool
	mu         sync.Mutex

	// History storage (last N snapshots)
	history    []MetricsSnapshot
	maxHistory int
	historyMu  sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(prometheus *PrometheusMetrics, interval time.Duration) *MetricsCollector {
	return &MetricsCollector{
		prometheus: prometheus,
		interval:   interval,
		stopChan:   make(chan struct{}),
		running:    false,
		history:    make([]MetricsSnapshot, 0),
		maxHistory: 100, // Keep last 100 snapshots
	}
}

// Start begins collecting metrics
func (mc *MetricsCollector) Start() {
	mc.mu.Lock()
	if mc.running {
		mc.mu.Unlock()
		return
	}
	mc.running = true
	mc.mu.Unlock()

	go mc.collectLoop()
}

// Stop stops collecting metrics
func (mc *MetricsCollector) Stop() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if !mc.running {
		return
	}

	mc.running = false
	close(mc.stopChan)
}

// collectLoop periodically collects metrics
func (mc *MetricsCollector) collectLoop() {
	ticker := time.NewTicker(mc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			snapshot := mc.prometheus.Collect()
			mc.addToHistory(snapshot)

		case <-mc.stopChan:
			return
		}
	}
}

// addToHistory adds a snapshot to history
func (mc *MetricsCollector) addToHistory(snapshot MetricsSnapshot) {
	mc.historyMu.Lock()
	defer mc.historyMu.Unlock()

	mc.history = append(mc.history, snapshot)

	// Trim if exceeds max
	if len(mc.history) > mc.maxHistory {
		mc.history = mc.history[1:]
	}
}

// GetLatest returns the most recent snapshot
func (mc *MetricsCollector) GetLatest() *MetricsSnapshot {
	mc.historyMu.RLock()
	defer mc.historyMu.RUnlock()

	if len(mc.history) == 0 {
		return nil
	}

	return &mc.history[len(mc.history)-1]
}

// GetHistory returns all stored snapshots
func (mc *MetricsCollector) GetHistory() []MetricsSnapshot {
	mc.historyMu.RLock()
	defer mc.historyMu.RUnlock()

	// Return copy
	history := make([]MetricsSnapshot, len(mc.history))
	copy(history, mc.history)

	return history
}

// GetHistorySince returns snapshots since a given time
func (mc *MetricsCollector) GetHistorySince(since time.Time) []MetricsSnapshot {
	mc.historyMu.RLock()
	defer mc.historyMu.RUnlock()

	result := make([]MetricsSnapshot, 0)
	for _, snapshot := range mc.history {
		if snapshot.Timestamp.After(since) {
			result = append(result, snapshot)
		}
	}

	return result
}

// Helper functions

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

func calculateErrorRate(status *client.MCPClientStatus) float64 {
	if status.TotalCalls == 0 {
		return 0.0
	}
	return float64(status.FailedCalls) / float64(status.TotalCalls) * 100
}