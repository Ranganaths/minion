// Package metrics provides a metrics interface for the minion framework.
// This package allows users to integrate their preferred metrics/monitoring framework.
package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Labels represents metric labels/tags
type Labels map[string]string

// Counter is a metric that only increases
type Counter interface {
	// Inc increments the counter by 1
	Inc()

	// Add adds the given value to the counter
	Add(delta float64)
}

// Gauge is a metric that can go up or down
type Gauge interface {
	// Set sets the gauge to the given value
	Set(value float64)

	// Inc increments the gauge by 1
	Inc()

	// Dec decrements the gauge by 1
	Dec()

	// Add adds the given value to the gauge
	Add(delta float64)
}

// Histogram records observations and calculates statistics
type Histogram interface {
	// Observe records a value
	Observe(value float64)
}

// Timer is a convenience wrapper for timing operations
type Timer interface {
	// ObserveDuration records the duration since the timer was started
	ObserveDuration()
}

// Metrics is the interface for creating metric instruments
type Metrics interface {
	// Counter creates or gets a counter
	Counter(name string, labels Labels) Counter

	// Gauge creates or gets a gauge
	Gauge(name string, labels Labels) Gauge

	// Histogram creates or gets a histogram
	Histogram(name string, labels Labels) Histogram

	// NewTimer creates a timer that will record to the given histogram
	NewTimer(histogram Histogram) Timer
}

// default metrics provider
var (
	defaultMetrics Metrics = NewNopMetrics()
	metricsMu      sync.RWMutex
)

// SetMetrics sets the global metrics provider
func SetMetrics(m Metrics) {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	defaultMetrics = m
}

// GetMetrics returns the global metrics provider
func GetMetrics() Metrics {
	metricsMu.RLock()
	defer metricsMu.RUnlock()
	return defaultMetrics
}

// Convenience functions using global metrics

// NewCounter creates a counter using the global metrics provider
func NewCounter(name string, labels Labels) Counter {
	return GetMetrics().Counter(name, labels)
}

// NewGauge creates a gauge using the global metrics provider
func NewGauge(name string, labels Labels) Gauge {
	return GetMetrics().Gauge(name, labels)
}

// NewHistogram creates a histogram using the global metrics provider
func NewHistogram(name string, labels Labels) Histogram {
	return GetMetrics().Histogram(name, labels)
}

// NopMetrics is a no-op metrics implementation
type NopMetrics struct{}

// NewNopMetrics creates a no-op metrics provider
func NewNopMetrics() *NopMetrics {
	return &NopMetrics{}
}

func (m *NopMetrics) Counter(name string, labels Labels) Counter     { return &nopCounter{} }
func (m *NopMetrics) Gauge(name string, labels Labels) Gauge         { return &nopGauge{} }
func (m *NopMetrics) Histogram(name string, labels Labels) Histogram { return &nopHistogram{} }
func (m *NopMetrics) NewTimer(histogram Histogram) Timer             { return &nopTimer{} }

type nopCounter struct{}

func (c *nopCounter) Inc()            {}
func (c *nopCounter) Add(delta float64) {}

type nopGauge struct{}

func (g *nopGauge) Set(value float64)  {}
func (g *nopGauge) Inc()               {}
func (g *nopGauge) Dec()               {}
func (g *nopGauge) Add(delta float64) {}

type nopHistogram struct{}

func (h *nopHistogram) Observe(value float64) {}

type nopTimer struct{}

func (t *nopTimer) ObserveDuration() {}

// InMemoryMetrics is a simple in-memory metrics implementation for testing
type InMemoryMetrics struct {
	mu         sync.RWMutex
	counters   map[string]*InMemoryCounter
	gauges     map[string]*InMemoryGauge
	histograms map[string]*InMemoryHistogram
}

// NewInMemoryMetrics creates an in-memory metrics provider
func NewInMemoryMetrics() *InMemoryMetrics {
	return &InMemoryMetrics{
		counters:   make(map[string]*InMemoryCounter),
		gauges:     make(map[string]*InMemoryGauge),
		histograms: make(map[string]*InMemoryHistogram),
	}
}

func (m *InMemoryMetrics) Counter(name string, labels Labels) Counter {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := formatKey(name, labels)
	if c, ok := m.counters[key]; ok {
		return c
	}

	c := &InMemoryCounter{}
	m.counters[key] = c
	return c
}

func (m *InMemoryMetrics) Gauge(name string, labels Labels) Gauge {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := formatKey(name, labels)
	if g, ok := m.gauges[key]; ok {
		return g
	}

	g := &InMemoryGauge{}
	m.gauges[key] = g
	return g
}

func (m *InMemoryMetrics) Histogram(name string, labels Labels) Histogram {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := formatKey(name, labels)
	if h, ok := m.histograms[key]; ok {
		return h
	}

	h := &InMemoryHistogram{}
	m.histograms[key] = h
	return h
}

func (m *InMemoryMetrics) NewTimer(histogram Histogram) Timer {
	return &inMemoryTimer{
		histogram: histogram,
		start:     time.Now(),
	}
}

// GetCounterValue returns the value of a counter for testing
func (m *InMemoryMetrics) GetCounterValue(name string, labels Labels) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := formatKey(name, labels)
	if c, ok := m.counters[key]; ok {
		return c.Value()
	}
	return 0
}

// GetGaugeValue returns the value of a gauge for testing
func (m *InMemoryMetrics) GetGaugeValue(name string, labels Labels) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := formatKey(name, labels)
	if g, ok := m.gauges[key]; ok {
		return g.Value()
	}
	return 0
}

// GetHistogramCount returns the observation count of a histogram for testing
func (m *InMemoryMetrics) GetHistogramCount(name string, labels Labels) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := formatKey(name, labels)
	if h, ok := m.histograms[key]; ok {
		return h.Count()
	}
	return 0
}

func formatKey(name string, labels Labels) string {
	key := name
	for k, v := range labels {
		key += "_" + k + "=" + v
	}
	return key
}

// InMemoryCounter is a thread-safe in-memory counter
type InMemoryCounter struct {
	value atomic.Int64
}

func (c *InMemoryCounter) Inc() {
	c.value.Add(1)
}

func (c *InMemoryCounter) Add(delta float64) {
	c.value.Add(int64(delta))
}

func (c *InMemoryCounter) Value() float64 {
	return float64(c.value.Load())
}

// InMemoryGauge is a thread-safe in-memory gauge
type InMemoryGauge struct {
	value atomic.Int64
}

func (g *InMemoryGauge) Set(value float64) {
	g.value.Store(int64(value * 1000)) // Store as millis for precision
}

func (g *InMemoryGauge) Inc() {
	g.value.Add(1000)
}

func (g *InMemoryGauge) Dec() {
	g.value.Add(-1000)
}

func (g *InMemoryGauge) Add(delta float64) {
	g.value.Add(int64(delta * 1000))
}

func (g *InMemoryGauge) Value() float64 {
	return float64(g.value.Load()) / 1000
}

// InMemoryHistogram is a thread-safe in-memory histogram
type InMemoryHistogram struct {
	mu     sync.Mutex
	values []float64
	count  atomic.Int64
	sum    atomic.Int64
}

func (h *InMemoryHistogram) Observe(value float64) {
	h.mu.Lock()
	h.values = append(h.values, value)
	h.mu.Unlock()

	h.count.Add(1)
	h.sum.Add(int64(value * 1000))
}

func (h *InMemoryHistogram) Count() int64 {
	return h.count.Load()
}

func (h *InMemoryHistogram) Sum() float64 {
	return float64(h.sum.Load()) / 1000
}

func (h *InMemoryHistogram) Values() []float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]float64, len(h.values))
	copy(result, h.values)
	return result
}

type inMemoryTimer struct {
	histogram Histogram
	start     time.Time
}

func (t *inMemoryTimer) ObserveDuration() {
	t.histogram.Observe(time.Since(t.start).Seconds())
}

// Common metric names used in the minion framework
const (
	// Chain metrics
	MetricChainCallsTotal    = "minion_chain_calls_total"
	MetricChainCallDuration  = "minion_chain_call_duration_seconds"
	MetricChainCallErrors    = "minion_chain_call_errors_total"

	// Embedding metrics
	MetricEmbeddingCacheHits   = "minion_embedding_cache_hits_total"
	MetricEmbeddingCacheMisses = "minion_embedding_cache_misses_total"
	MetricEmbeddingDuration    = "minion_embedding_duration_seconds"

	// VectorStore metrics
	MetricVectorStoreDocuments = "minion_vectorstore_documents_total"
	MetricVectorStoreSearches  = "minion_vectorstore_searches_total"
	MetricVectorStoreSearchDuration = "minion_vectorstore_search_duration_seconds"

	// LLM metrics
	MetricLLMTokensUsed     = "minion_llm_tokens_used_total"
	MetricLLMCallsTotal     = "minion_llm_calls_total"
	MetricLLMCallDuration   = "minion_llm_call_duration_seconds"
	MetricLLMCallErrors     = "minion_llm_call_errors_total"
)
