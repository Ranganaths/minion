// Package metrics provides Prometheus metrics integration for the minion framework.
package metrics

import (
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusMetrics implements the Metrics interface using Prometheus
type PrometheusMetrics struct {
	mu         sync.RWMutex
	registry   *prometheus.Registry
	counters   map[string]*prometheus.CounterVec
	gauges     map[string]*prometheus.GaugeVec
	histograms map[string]*prometheus.HistogramVec

	// Default histogram buckets for different metric types
	defaultBuckets   []float64
	durationBuckets  []float64
	tokenBuckets     []float64
}

// PrometheusConfig contains configuration for Prometheus metrics
type PrometheusConfig struct {
	// Namespace is the prefix for all metrics (default: "minion")
	Namespace string

	// Subsystem is an optional second-level prefix
	Subsystem string

	// DefaultBuckets for histograms
	DefaultBuckets []float64

	// DurationBuckets for timing histograms (in seconds)
	DurationBuckets []float64

	// TokenBuckets for token count histograms
	TokenBuckets []float64

	// EnableGoCollector enables Go runtime metrics
	EnableGoCollector bool

	// EnableProcessCollector enables process metrics
	EnableProcessCollector bool
}

// DefaultPrometheusConfig returns sensible defaults
func DefaultPrometheusConfig() *PrometheusConfig {
	return &PrometheusConfig{
		Namespace:              "minion",
		Subsystem:              "",
		DefaultBuckets:         prometheus.DefBuckets,
		DurationBuckets:        []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
		TokenBuckets:           []float64{10, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 25000, 50000},
		EnableGoCollector:      true,
		EnableProcessCollector: true,
	}
}

// NewPrometheusMetrics creates a new Prometheus metrics provider
func NewPrometheusMetrics(config *PrometheusConfig) *PrometheusMetrics {
	if config == nil {
		config = DefaultPrometheusConfig()
	}

	registry := prometheus.NewRegistry()

	// Register standard collectors if enabled
	if config.EnableGoCollector {
		registry.MustRegister(collectors.NewGoCollector())
	}
	if config.EnableProcessCollector {
		registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}

	return &PrometheusMetrics{
		registry:        registry,
		counters:        make(map[string]*prometheus.CounterVec),
		gauges:          make(map[string]*prometheus.GaugeVec),
		histograms:      make(map[string]*prometheus.HistogramVec),
		defaultBuckets:  config.DefaultBuckets,
		durationBuckets: config.DurationBuckets,
		tokenBuckets:    config.TokenBuckets,
	}
}

// Counter creates or gets a counter with the given labels
func (p *PrometheusMetrics) Counter(name string, labels Labels) Counter {
	p.mu.Lock()
	defer p.mu.Unlock()

	labelNames := getLabelNames(labels)
	key := name + "_" + strings.Join(labelNames, "_")

	counter, exists := p.counters[key]
	if !exists {
		counter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: name,
				Help: "Counter for " + name,
			},
			labelNames,
		)
		p.registry.MustRegister(counter)
		p.counters[key] = counter
	}

	return &prometheusCounter{
		counter: counter.With(prometheus.Labels(labels)),
	}
}

// Gauge creates or gets a gauge with the given labels
func (p *PrometheusMetrics) Gauge(name string, labels Labels) Gauge {
	p.mu.Lock()
	defer p.mu.Unlock()

	labelNames := getLabelNames(labels)
	key := name + "_" + strings.Join(labelNames, "_")

	gauge, exists := p.gauges[key]
	if !exists {
		gauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: name,
				Help: "Gauge for " + name,
			},
			labelNames,
		)
		p.registry.MustRegister(gauge)
		p.gauges[key] = gauge
	}

	return &prometheusGauge{
		gauge: gauge.With(prometheus.Labels(labels)),
	}
}

// Histogram creates or gets a histogram with the given labels
func (p *PrometheusMetrics) Histogram(name string, labels Labels) Histogram {
	p.mu.Lock()
	defer p.mu.Unlock()

	labelNames := getLabelNames(labels)
	key := name + "_" + strings.Join(labelNames, "_")

	histogram, exists := p.histograms[key]
	if !exists {
		// Choose buckets based on metric name
		buckets := p.defaultBuckets
		if strings.Contains(name, "duration") || strings.Contains(name, "seconds") {
			buckets = p.durationBuckets
		} else if strings.Contains(name, "tokens") {
			buckets = p.tokenBuckets
		}

		histogram = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    name,
				Help:    "Histogram for " + name,
				Buckets: buckets,
			},
			labelNames,
		)
		p.registry.MustRegister(histogram)
		p.histograms[key] = histogram
	}

	return &prometheusHistogram{
		histogram: histogram.With(prometheus.Labels(labels)).(prometheus.Observer),
	}
}

// NewTimer creates a timer that will record to the given histogram
func (p *PrometheusMetrics) NewTimer(histogram Histogram) Timer {
	return &prometheusTimer{
		histogram: histogram,
		start:     time.Now(),
	}
}

// Handler returns an HTTP handler for the /metrics endpoint
func (p *PrometheusMetrics) Handler() http.Handler {
	return promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// Registry returns the underlying Prometheus registry
func (p *PrometheusMetrics) Registry() *prometheus.Registry {
	return p.registry
}

// Helper to get sorted label names
func getLabelNames(labels Labels) []string {
	names := make([]string, 0, len(labels))
	for k := range labels {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// prometheusCounter wraps a Prometheus counter
type prometheusCounter struct {
	counter prometheus.Counter
}

func (c *prometheusCounter) Inc() {
	c.counter.Inc()
}

func (c *prometheusCounter) Add(delta float64) {
	c.counter.Add(delta)
}

// prometheusGauge wraps a Prometheus gauge
type prometheusGauge struct {
	gauge prometheus.Gauge
}

func (g *prometheusGauge) Set(value float64) {
	g.gauge.Set(value)
}

func (g *prometheusGauge) Inc() {
	g.gauge.Inc()
}

func (g *prometheusGauge) Dec() {
	g.gauge.Dec()
}

func (g *prometheusGauge) Add(delta float64) {
	g.gauge.Add(delta)
}

// prometheusHistogram wraps a Prometheus histogram
type prometheusHistogram struct {
	histogram prometheus.Observer
}

func (h *prometheusHistogram) Observe(value float64) {
	h.histogram.Observe(value)
}

// prometheusTimer implements Timer for Prometheus
type prometheusTimer struct {
	histogram Histogram
	start     time.Time
}

func (t *prometheusTimer) ObserveDuration() {
	t.histogram.Observe(time.Since(t.start).Seconds())
}

// Pre-defined metrics for common framework operations

// AgentMetrics provides agent-specific metrics
type AgentMetrics struct {
	ExecutionsTotal    Counter
	ExecutionErrors    Counter
	ExecutionDuration  Histogram
	ActiveAgents       Gauge
}

// NewAgentMetrics creates metrics for agent operations
func NewAgentMetrics(m Metrics, agentID string) *AgentMetrics {
	labels := Labels{"agent_id": agentID}
	return &AgentMetrics{
		ExecutionsTotal:   m.Counter("minion_agent_executions_total", labels),
		ExecutionErrors:   m.Counter("minion_agent_execution_errors_total", labels),
		ExecutionDuration: m.Histogram("minion_agent_execution_duration_seconds", labels),
		ActiveAgents:      m.Gauge("minion_agent_active", labels),
	}
}

// MultiAgentMetrics provides multi-agent orchestration metrics
type MultiAgentMetrics struct {
	TasksTotal         Counter
	TasksCompleted     Counter
	TasksFailed        Counter
	SubtasksTotal      Counter
	TaskDuration       Histogram
	ActiveWorkers      Gauge
	PlanningDuration   Histogram
	ReplanAttempts     Counter
}

// NewMultiAgentMetrics creates metrics for multi-agent operations
func NewMultiAgentMetrics(m Metrics, orchestratorID string) *MultiAgentMetrics {
	labels := Labels{"orchestrator_id": orchestratorID}
	return &MultiAgentMetrics{
		TasksTotal:       m.Counter("minion_multiagent_tasks_total", labels),
		TasksCompleted:   m.Counter("minion_multiagent_tasks_completed_total", labels),
		TasksFailed:      m.Counter("minion_multiagent_tasks_failed_total", labels),
		SubtasksTotal:    m.Counter("minion_multiagent_subtasks_total", labels),
		TaskDuration:     m.Histogram("minion_multiagent_task_duration_seconds", labels),
		ActiveWorkers:    m.Gauge("minion_multiagent_active_workers", labels),
		PlanningDuration: m.Histogram("minion_multiagent_planning_duration_seconds", labels),
		ReplanAttempts:   m.Counter("minion_multiagent_replan_attempts_total", labels),
	}
}

// LLMMetrics provides LLM-specific metrics
type LLMMetrics struct {
	CallsTotal      Counter
	CallErrors      Counter
	CallDuration    Histogram
	TokensUsed      Counter
	PromptTokens    Counter
	CompletionTokens Counter
	EstimatedCost   Counter
}

// NewLLMMetrics creates metrics for LLM operations
func NewLLMMetrics(m Metrics, provider, model string) *LLMMetrics {
	labels := Labels{"provider": provider, "model": model}
	return &LLMMetrics{
		CallsTotal:       m.Counter("minion_llm_calls_total", labels),
		CallErrors:       m.Counter("minion_llm_call_errors_total", labels),
		CallDuration:     m.Histogram("minion_llm_call_duration_seconds", labels),
		TokensUsed:       m.Counter("minion_llm_tokens_total", labels),
		PromptTokens:     m.Counter("minion_llm_prompt_tokens_total", labels),
		CompletionTokens: m.Counter("minion_llm_completion_tokens_total", labels),
		EstimatedCost:    m.Counter("minion_llm_estimated_cost_dollars", labels),
	}
}

// ToolMetrics provides tool execution metrics
type ToolMetrics struct {
	ExecutionsTotal   Counter
	ExecutionErrors   Counter
	ExecutionDuration Histogram
}

// NewToolMetrics creates metrics for tool operations
func NewToolMetrics(m Metrics, toolName string) *ToolMetrics {
	labels := Labels{"tool_name": toolName}
	return &ToolMetrics{
		ExecutionsTotal:   m.Counter("minion_tool_executions_total", labels),
		ExecutionErrors:   m.Counter("minion_tool_execution_errors_total", labels),
		ExecutionDuration: m.Histogram("minion_tool_execution_duration_seconds", labels),
	}
}

// MemoryMetrics provides memory system metrics
type MemoryMetrics struct {
	OperationsTotal  Counter
	OperationErrors  Counter
	OperationDuration Histogram
	ItemsStored      Gauge
	CacheHits        Counter
	CacheMisses      Counter
}

// NewMemoryMetrics creates metrics for memory operations
func NewMemoryMetrics(m Metrics, memoryType string) *MemoryMetrics {
	labels := Labels{"memory_type": memoryType}
	return &MemoryMetrics{
		OperationsTotal:   m.Counter("minion_memory_operations_total", labels),
		OperationErrors:   m.Counter("minion_memory_operation_errors_total", labels),
		OperationDuration: m.Histogram("minion_memory_operation_duration_seconds", labels),
		ItemsStored:       m.Gauge("minion_memory_items_stored", labels),
		CacheHits:         m.Counter("minion_memory_cache_hits_total", labels),
		CacheMisses:       m.Counter("minion_memory_cache_misses_total", labels),
	}
}

// Global Prometheus metrics instance
var globalPrometheusMetrics *PrometheusMetrics

// InitPrometheusMetrics initializes the global Prometheus metrics provider
func InitPrometheusMetrics(config *PrometheusConfig) *PrometheusMetrics {
	globalPrometheusMetrics = NewPrometheusMetrics(config)
	SetMetrics(globalPrometheusMetrics)
	return globalPrometheusMetrics
}

// GetPrometheusMetrics returns the global Prometheus metrics provider
func GetPrometheusMetrics() *PrometheusMetrics {
	return globalPrometheusMetrics
}

// MetricsHandler returns the HTTP handler for /metrics endpoint
// This is a convenience function for setting up the metrics endpoint
func MetricsHandler() http.Handler {
	if globalPrometheusMetrics != nil {
		return globalPrometheusMetrics.Handler()
	}
	// Fallback to default registry
	return promhttp.Handler()
}
