package observability

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsConfig contains metrics configuration
type MetricsConfig struct {
	Enabled           bool
	Port              int
	Path              string
	PrometheusEnabled bool
}

// MetricsCollector manages Prometheus metrics
type MetricsCollector struct {
	// Agent execution metrics
	agentExecutionsTotal *prometheus.CounterVec
	agentDurationSeconds *prometheus.HistogramVec
	agentErrorsTotal     *prometheus.CounterVec
	activeAgents         prometheus.Gauge

	// Session metrics
	sessionsActive *prometheus.GaugeVec
	sessionsTotal  *prometheus.CounterVec
	sessionDuration *prometheus.HistogramVec

	// Tool execution metrics
	toolCallsTotal    *prometheus.CounterVec
	toolDurationSeconds *prometheus.HistogramVec
	toolErrorsTotal   *prometheus.CounterVec

	// LLM metrics
	llmRequestsTotal      *prometheus.CounterVec
	llmLatencySeconds     *prometheus.HistogramVec
	llmTokensTotal        *prometheus.CounterVec
	llmCostTotal          *prometheus.CounterVec
	llmErrorsTotal        *prometheus.CounterVec

	// Storage metrics
	storageOperationsTotal *prometheus.CounterVec
	storageDurationSeconds *prometheus.HistogramVec
	storageErrorsTotal     *prometheus.CounterVec

	// Memory metrics
	memoriesTotal       *prometheus.GaugeVec
	memoryOperationsTotal *prometheus.CounterVec

	// System metrics
	healthStatus prometheus.Gauge

	// Multi-agent system metrics
	multiagentTasksTotal         *prometheus.CounterVec
	multiagentTaskDuration       *prometheus.HistogramVec
	multiagentMessagesTotal      *prometheus.CounterVec
	multiagentMessageLatency     *prometheus.HistogramVec
	multiagentWorkersTotal       *prometheus.CounterVec
	multiagentActiveWorkers      prometheus.Gauge
	multiagentPendingTasks       prometheus.Gauge
	multiagentQueueDepth         *prometheus.GaugeVec
	multiagentTaskLedgerSize     prometheus.Gauge
	multiagentProgressLedgerSize prometheus.Gauge
	multiagentErrorsTotal        *prometheus.CounterVec

	config MetricsConfig
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(config MetricsConfig, registry *prometheus.Registry) *MetricsCollector {
	if !config.Enabled {
		return &MetricsCollector{config: config}
	}

	if registry == nil {
		registry = prometheus.NewRegistry()
	}

	factory := promauto.With(registry)

	collector := &MetricsCollector{
		// Agent metrics
		agentExecutionsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_agent_executions_total",
				Help: "Total number of agent executions",
			},
			[]string{"agent_id", "agent_name", "status"},
		),
		agentDurationSeconds: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "minion_agent_duration_seconds",
				Help:    "Agent execution duration in seconds",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~100s
			},
			[]string{"agent_id", "agent_name"},
		),
		agentErrorsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_agent_errors_total",
				Help: "Total number of agent errors",
			},
			[]string{"agent_id", "error_type"},
		),
		activeAgents: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "minion_active_agents",
				Help: "Number of currently active agents",
			},
		),

		// Session metrics
		sessionsActive: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "minion_sessions_active",
				Help: "Number of active sessions",
			},
			[]string{"agent_id"},
		),
		sessionsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_sessions_total",
				Help: "Total number of sessions",
			},
			[]string{"agent_id", "status"},
		),
		sessionDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "minion_session_duration_seconds",
				Help:    "Session duration in seconds",
				Buckets: prometheus.ExponentialBuckets(10, 2, 12), // 10s to ~40000s
			},
			[]string{"agent_id"},
		),

		// Tool metrics
		toolCallsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_tool_calls_total",
				Help: "Total number of tool calls",
			},
			[]string{"tool_name", "status"},
		),
		toolDurationSeconds: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "minion_tool_duration_seconds",
				Help:    "Tool execution duration in seconds",
				Buckets: prometheus.ExponentialBuckets(0.01, 2, 10), // 10ms to ~10s
			},
			[]string{"tool_name"},
		),
		toolErrorsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_tool_errors_total",
				Help: "Total number of tool errors",
			},
			[]string{"tool_name", "error_type"},
		),

		// LLM metrics
		llmRequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_llm_requests_total",
				Help: "Total number of LLM API requests",
			},
			[]string{"provider", "model", "status"},
		),
		llmLatencySeconds: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "minion_llm_latency_seconds",
				Help:    "LLM API latency in seconds",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
			},
			[]string{"provider", "model"},
		),
		llmTokensTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_llm_tokens_total",
				Help: "Total number of LLM tokens used",
			},
			[]string{"provider", "model", "type"}, // type: prompt, completion
		),
		llmCostTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_llm_cost_total",
				Help: "Total LLM cost in USD",
			},
			[]string{"provider", "model"},
		),
		llmErrorsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_llm_errors_total",
				Help: "Total number of LLM errors",
			},
			[]string{"provider", "model", "error_type"},
		),

		// Storage metrics
		storageOperationsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_storage_operations_total",
				Help: "Total number of storage operations",
			},
			[]string{"operation", "table", "status"},
		),
		storageDurationSeconds: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "minion_storage_duration_seconds",
				Help:    "Storage operation duration in seconds",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to ~1s
			},
			[]string{"operation", "table"},
		),
		storageErrorsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_storage_errors_total",
				Help: "Total number of storage errors",
			},
			[]string{"operation", "error_type"},
		),

		// Memory metrics
		memoriesTotal: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "minion_memories_total",
				Help: "Total number of memories",
			},
			[]string{"agent_id", "type"},
		),
		memoryOperationsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_memory_operations_total",
				Help: "Total number of memory operations",
			},
			[]string{"operation", "type"},
		),

		// System metrics
		healthStatus: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "minion_health_status",
				Help: "Health status (1 = healthy, 0 = unhealthy)",
			},
		),

		// Multi-agent system metrics
		multiagentTasksTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_multiagent_tasks_total",
				Help: "Total number of multi-agent tasks by status",
			},
			[]string{"status"}, // started, completed, failed, pending
		),
		multiagentTaskDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "minion_multiagent_task_duration_seconds",
				Help:    "Multi-agent task execution duration in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
			},
			[]string{"type", "status"}, // type: code_generation/analysis/etc, status: completed/failed
		),
		multiagentMessagesTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_multiagent_messages_total",
				Help: "Total number of multi-agent messages by type and direction",
			},
			[]string{"type", "direction"}, // type: task/result/error, direction: sent/received
		),
		multiagentMessageLatency: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "minion_multiagent_message_latency_seconds",
				Help:    "Multi-agent message delivery latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"type"},
		),
		multiagentWorkersTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_multiagent_workers_total",
				Help: "Total number of multi-agent workers by status",
			},
			[]string{"status"}, // idle, busy, offline
		),
		multiagentActiveWorkers: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "minion_multiagent_active_workers",
				Help: "Number of currently active multi-agent workers",
			},
		),
		multiagentPendingTasks: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "minion_multiagent_pending_tasks",
				Help: "Number of multi-agent tasks pending execution",
			},
		),
		multiagentQueueDepth: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "minion_multiagent_queue_depth",
				Help: "Multi-agent message queue depth by agent",
			},
			[]string{"agent_id"},
		),
		multiagentTaskLedgerSize: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "minion_multiagent_task_ledger_size",
				Help: "Total number of tasks in the multi-agent task ledger",
			},
		),
		multiagentProgressLedgerSize: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "minion_multiagent_progress_ledger_size",
				Help: "Total number of entries in the multi-agent progress ledger",
			},
		),
		multiagentErrorsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "minion_multiagent_errors_total",
				Help: "Total number of multi-agent errors by component and type",
			},
			[]string{"component", "error_type"}, // component: orchestrator/worker/protocol
		),

		config: config,
	}

	// Set initial health status
	collector.healthStatus.Set(1)

	return collector
}

// RecordAgentExecution records an agent execution
func (m *MetricsCollector) RecordAgentExecution(agentID, agentName string, duration time.Duration, err error) {
	if !m.config.Enabled {
		return
	}

	status := "success"
	if err != nil {
		status = "error"
		m.agentErrorsTotal.WithLabelValues(agentID, "execution_error").Inc()
	}

	m.agentExecutionsTotal.WithLabelValues(agentID, agentName, status).Inc()
	m.agentDurationSeconds.WithLabelValues(agentID, agentName).Observe(duration.Seconds())
}

// RecordToolCall records a tool call
func (m *MetricsCollector) RecordToolCall(toolName string, duration time.Duration, err error) {
	if !m.config.Enabled {
		return
	}

	status := "success"
	if err != nil {
		status = "error"
		m.toolErrorsTotal.WithLabelValues(toolName, "execution_error").Inc()
	}

	m.toolCallsTotal.WithLabelValues(toolName, status).Inc()
	m.toolDurationSeconds.WithLabelValues(toolName).Observe(duration.Seconds())
}

// RecordLLMRequest records an LLM API request
func (m *MetricsCollector) RecordLLMRequest(provider, model string, duration time.Duration, promptTokens, completionTokens int, cost float64, err error) {
	if !m.config.Enabled {
		return
	}

	status := "success"
	if err != nil {
		status = "error"
		m.llmErrorsTotal.WithLabelValues(provider, model, "api_error").Inc()
	}

	m.llmRequestsTotal.WithLabelValues(provider, model, status).Inc()
	m.llmLatencySeconds.WithLabelValues(provider, model).Observe(duration.Seconds())

	if status == "success" {
		m.llmTokensTotal.WithLabelValues(provider, model, "prompt").Add(float64(promptTokens))
		m.llmTokensTotal.WithLabelValues(provider, model, "completion").Add(float64(completionTokens))
		m.llmCostTotal.WithLabelValues(provider, model).Add(cost)
	}
}

// RecordStorageOperation records a storage operation
func (m *MetricsCollector) RecordStorageOperation(operation, table string, duration time.Duration, err error) {
	if !m.config.Enabled {
		return
	}

	status := "success"
	if err != nil {
		status = "error"
		m.storageErrorsTotal.WithLabelValues(operation, "query_error").Inc()
	}

	m.storageOperationsTotal.WithLabelValues(operation, table, status).Inc()
	m.storageDurationSeconds.WithLabelValues(operation, table).Observe(duration.Seconds())
}

// RecordSessionCreated records a new session
func (m *MetricsCollector) RecordSessionCreated(agentID string) {
	if !m.config.Enabled {
		return
	}

	m.sessionsTotal.WithLabelValues(agentID, "created").Inc()
	m.sessionsActive.WithLabelValues(agentID).Inc()
}

// RecordSessionClosed records a closed session
func (m *MetricsCollector) RecordSessionClosed(agentID string, duration time.Duration) {
	if !m.config.Enabled {
		return
	}

	m.sessionsTotal.WithLabelValues(agentID, "closed").Inc()
	m.sessionsActive.WithLabelValues(agentID).Dec()
	m.sessionDuration.WithLabelValues(agentID).Observe(duration.Seconds())
}

// RecordMemoryOperation records a memory operation
func (m *MetricsCollector) RecordMemoryOperation(operation, memoryType string, count int) {
	if !m.config.Enabled {
		return
	}

	m.memoryOperationsTotal.WithLabelValues(operation, memoryType).Add(float64(count))
}

// SetMemoriesCount sets the current number of memories
func (m *MetricsCollector) SetMemoriesCount(agentID, memoryType string, count int) {
	if !m.config.Enabled {
		return
	}

	m.memoriesTotal.WithLabelValues(agentID, memoryType).Set(float64(count))
}

// SetActiveAgents sets the number of active agents
func (m *MetricsCollector) SetActiveAgents(count int) {
	if !m.config.Enabled {
		return
	}

	m.activeAgents.Set(float64(count))
}

// SetHealthStatus sets the health status
func (m *MetricsCollector) SetHealthStatus(healthy bool) {
	if !m.config.Enabled {
		return
	}

	if healthy {
		m.healthStatus.Set(1)
	} else {
		m.healthStatus.Set(0)
	}
}

// GetHandler returns the HTTP handler for Prometheus metrics
func (m *MetricsCollector) GetHandler() http.Handler {
	return promhttp.Handler()
}

// StartMetricsServer starts the metrics HTTP server
func (m *MetricsCollector) StartMetricsServer() error {
	if !m.config.Enabled {
		return nil
	}

	http.Handle(m.config.Path, m.GetHandler())

	addr := fmt.Sprintf(":%d", m.config.Port)
	fmt.Printf("Starting metrics server on %s%s\n", addr, m.config.Path)

	return http.ListenAndServe(addr, nil)
}

// Global metrics collector
var globalMetrics *MetricsCollector

// InitGlobalMetrics initializes the global metrics collector
func InitGlobalMetrics(config MetricsConfig) error {
	globalMetrics = NewMetricsCollector(config, prometheus.DefaultRegisterer.(*prometheus.Registry))
	return nil
}

// GetMetrics returns the global metrics collector
func GetMetrics() *MetricsCollector {
	if globalMetrics == nil {
		_ = InitGlobalMetrics(MetricsConfig{
			Enabled: false,
			Port:    9090,
			Path:    "/metrics",
		})
	}
	return globalMetrics
}

// Convenience functions using global metrics

// RecordAgentExecution records an agent execution using global metrics
func RecordAgentExecution(agentID, agentName string, duration time.Duration, err error) {
	GetMetrics().RecordAgentExecution(agentID, agentName, duration, err)
}

// RecordToolCall records a tool call using global metrics
func RecordToolCall(toolName string, duration time.Duration, err error) {
	GetMetrics().RecordToolCall(toolName, duration, err)
}

// RecordLLMRequest records an LLM request using global metrics
func RecordLLMRequest(provider, model string, duration time.Duration, promptTokens, completionTokens int, cost float64, err error) {
	GetMetrics().RecordLLMRequest(provider, model, duration, promptTokens, completionTokens, cost, err)
}

// RecordStorageOperation records a storage operation using global metrics
func RecordStorageOperation(operation, table string, duration time.Duration, err error) {
	GetMetrics().RecordStorageOperation(operation, table, duration, err)
}

// Multi-agent system metrics methods

// RecordMultiagentTaskStarted records when a multi-agent task starts
func (m *MetricsCollector) RecordMultiagentTaskStarted() {
	if !m.config.Enabled {
		return
	}
	if m.multiagentTasksTotal != nil {
		m.multiagentTasksTotal.WithLabelValues("started").Inc()
	}
	if m.multiagentPendingTasks != nil {
		m.multiagentPendingTasks.Inc()
	}
}

// RecordMultiagentTaskCompleted records when a multi-agent task completes successfully
func (m *MetricsCollector) RecordMultiagentTaskCompleted(taskType string, duration time.Duration) {
	if !m.config.Enabled {
		return
	}
	if m.multiagentTasksTotal != nil {
		m.multiagentTasksTotal.WithLabelValues("completed").Inc()
	}
	if m.multiagentTaskDuration != nil {
		m.multiagentTaskDuration.WithLabelValues(taskType, "completed").Observe(duration.Seconds())
	}
	if m.multiagentPendingTasks != nil {
		m.multiagentPendingTasks.Dec()
	}
}

// RecordMultiagentTaskFailed records when a multi-agent task fails
func (m *MetricsCollector) RecordMultiagentTaskFailed(taskType string, duration time.Duration) {
	if !m.config.Enabled {
		return
	}
	if m.multiagentTasksTotal != nil {
		m.multiagentTasksTotal.WithLabelValues("failed").Inc()
	}
	if m.multiagentTaskDuration != nil {
		m.multiagentTaskDuration.WithLabelValues(taskType, "failed").Observe(duration.Seconds())
	}
	if m.multiagentPendingTasks != nil {
		m.multiagentPendingTasks.Dec()
	}
}

// RecordMultiagentMessageSent records when a multi-agent message is sent
func (m *MetricsCollector) RecordMultiagentMessageSent(messageType string, latency time.Duration) {
	if !m.config.Enabled {
		return
	}
	if m.multiagentMessagesTotal != nil {
		m.multiagentMessagesTotal.WithLabelValues(messageType, "sent").Inc()
	}
	if m.multiagentMessageLatency != nil {
		m.multiagentMessageLatency.WithLabelValues(messageType).Observe(latency.Seconds())
	}
}

// RecordMultiagentMessageReceived records when a multi-agent message is received
func (m *MetricsCollector) RecordMultiagentMessageReceived(messageType string) {
	if !m.config.Enabled {
		return
	}
	if m.multiagentMessagesTotal != nil {
		m.multiagentMessagesTotal.WithLabelValues(messageType, "received").Inc()
	}
}

// RecordMultiagentWorkerBusy records when a worker becomes busy
func (m *MetricsCollector) RecordMultiagentWorkerBusy() {
	if !m.config.Enabled {
		return
	}
	if m.multiagentWorkersTotal != nil {
		m.multiagentWorkersTotal.WithLabelValues("busy").Inc()
	}
}

// RecordMultiagentWorkerIdle records when a worker becomes idle
func (m *MetricsCollector) RecordMultiagentWorkerIdle() {
	if !m.config.Enabled {
		return
	}
	if m.multiagentWorkersTotal != nil {
		m.multiagentWorkersTotal.WithLabelValues("idle").Inc()
	}
}

// RecordMultiagentError records a multi-agent error
func (m *MetricsCollector) RecordMultiagentError(component, errorType string) {
	if !m.config.Enabled {
		return
	}
	if m.multiagentErrorsTotal != nil {
		m.multiagentErrorsTotal.WithLabelValues(component, errorType).Inc()
	}
}

// SetMultiagentActiveWorkers sets the number of active multi-agent workers
func (m *MetricsCollector) SetMultiagentActiveWorkers(count int) {
	if !m.config.Enabled {
		return
	}
	if m.multiagentActiveWorkers != nil {
		m.multiagentActiveWorkers.Set(float64(count))
	}
}

// SetMultiagentQueueDepth sets the queue depth for a multi-agent agent
func (m *MetricsCollector) SetMultiagentQueueDepth(agentID string, depth int) {
	if !m.config.Enabled {
		return
	}
	if m.multiagentQueueDepth != nil {
		m.multiagentQueueDepth.WithLabelValues(agentID).Set(float64(depth))
	}
}

// SetMultiagentTaskLedgerSize sets the task ledger size
func (m *MetricsCollector) SetMultiagentTaskLedgerSize(size int) {
	if !m.config.Enabled {
		return
	}
	if m.multiagentTaskLedgerSize != nil {
		m.multiagentTaskLedgerSize.Set(float64(size))
	}
}

// SetMultiagentProgressLedgerSize sets the progress ledger size
func (m *MetricsCollector) SetMultiagentProgressLedgerSize(size int) {
	if !m.config.Enabled {
		return
	}
	if m.multiagentProgressLedgerSize != nil {
		m.multiagentProgressLedgerSize.Set(float64(size))
	}
}

// Convenience functions for multi-agent metrics using global metrics

// RecordMultiagentTaskStarted records a multi-agent task start using global metrics
func RecordMultiagentTaskStarted() {
	GetMetrics().RecordMultiagentTaskStarted()
}

// RecordMultiagentTaskCompleted records a multi-agent task completion using global metrics
func RecordMultiagentTaskCompleted(taskType string, duration time.Duration) {
	GetMetrics().RecordMultiagentTaskCompleted(taskType, duration)
}

// RecordMultiagentTaskFailed records a multi-agent task failure using global metrics
func RecordMultiagentTaskFailed(taskType string, duration time.Duration) {
	GetMetrics().RecordMultiagentTaskFailed(taskType, duration)
}

// RecordMultiagentMessageSent records a multi-agent message sent using global metrics
func RecordMultiagentMessageSent(messageType string, latency time.Duration) {
	GetMetrics().RecordMultiagentMessageSent(messageType, latency)
}

// RecordMultiagentMessageReceived records a multi-agent message received using global metrics
func RecordMultiagentMessageReceived(messageType string) {
	GetMetrics().RecordMultiagentMessageReceived(messageType)
}
