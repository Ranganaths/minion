package selfimprove

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RollbackMonitor monitors agent performance and triggers rollbacks when needed.
type RollbackMonitor struct {
	mu sync.RWMutex

	// Version manager for prompt rollbacks
	versionManager *PromptVersionManager

	// Experience store for performance data
	experienceStore ExperienceStore

	// Proposal store for tracking applied proposals
	proposalStore ProposalStore

	// Configuration
	config *RollbackConfig

	// Active monitors per agent
	monitors map[string]*AgentMonitor

	// Rollback history
	rollbackHistory []*RollbackEvent

	// Event handlers
	handlers []RollbackHandler

	// Metrics
	metrics *RollbackMetrics
}

// RollbackConfig configures rollback behavior.
type RollbackConfig struct {
	// Enabled determines if automatic rollback is enabled
	Enabled bool `json:"enabled"`

	// MonitoringWindowMinutes is the window for performance monitoring
	MonitoringWindowMinutes int `json:"monitoring_window_minutes"`

	// MinExecutionsForRollback is minimum executions before considering rollback
	MinExecutionsForRollback int `json:"min_executions_for_rollback"`

	// PerformanceDropThreshold triggers rollback if score drops by this much
	PerformanceDropThreshold float64 `json:"performance_drop_threshold"`

	// SuccessRateDropThreshold triggers rollback if success rate drops by this much
	SuccessRateDropThreshold float64 `json:"success_rate_drop_threshold"`

	// ErrorRateThreshold triggers rollback if error rate exceeds this
	ErrorRateThreshold float64 `json:"error_rate_threshold"`

	// CooldownMinutes is time to wait after a rollback before another
	CooldownMinutes int `json:"cooldown_minutes"`

	// MaxRollbacksPerDay limits rollbacks per agent per day
	MaxRollbacksPerDay int `json:"max_rollbacks_per_day"`

	// AlertOnRollback sends alerts when rollback occurs
	AlertOnRollback bool `json:"alert_on_rollback"`

	// GracePeriodMinutes is time after applying changes before monitoring
	GracePeriodMinutes int `json:"grace_period_minutes"`
}

// AgentMonitor tracks performance for a single agent.
type AgentMonitor struct {
	AgentID          string
	BaselineScore    float64
	BaselineSuccess  float64
	CurrentScore     float64
	CurrentSuccess   float64
	ExecutionCount   int
	ErrorCount       int
	LastRollback     time.Time
	RollbacksToday   int
	MonitoringStart  time.Time
	LastProposalID   string
	InGracePeriod    bool
	GraceEndTime     time.Time
}

// RollbackEvent represents a rollback occurrence.
type RollbackEvent struct {
	ID             string    `json:"id"`
	AgentID        string    `json:"agent_id"`
	Timestamp      time.Time `json:"timestamp"`
	Reason         string    `json:"reason"`
	FromVersionID  string    `json:"from_version_id"`
	ToVersionID    string    `json:"to_version_id"`
	ProposalID     string    `json:"proposal_id,omitempty"`
	ScoreBefore    float64   `json:"score_before"`
	ScoreAfter     float64   `json:"score_after"`
	SuccessBefore  float64   `json:"success_before"`
	SuccessAfter   float64   `json:"success_after"`
	Automatic      bool      `json:"automatic"`
}

// RollbackHandler handles rollback events.
type RollbackHandler interface {
	OnRollback(ctx context.Context, event *RollbackEvent) error
}

// RollbackMetrics tracks rollback statistics.
type RollbackMetrics struct {
	TotalRollbacks      int64              `json:"total_rollbacks"`
	AutomaticRollbacks  int64              `json:"automatic_rollbacks"`
	ManualRollbacks     int64              `json:"manual_rollbacks"`
	RollbacksPerAgent   map[string]int     `json:"rollbacks_per_agent"`
	AverageRecoveryTime float64            `json:"average_recovery_time_minutes"`
	PreventedRegressions int64             `json:"prevented_regressions"`
}

// NewRollbackMonitor creates a new rollback monitor.
func NewRollbackMonitor(
	versionManager *PromptVersionManager,
	experienceStore ExperienceStore,
	proposalStore ProposalStore,
	config *RollbackConfig,
) *RollbackMonitor {
	if config == nil {
		config = DefaultRollbackConfig()
	}

	return &RollbackMonitor{
		versionManager:  versionManager,
		experienceStore: experienceStore,
		proposalStore:   proposalStore,
		config:          config,
		monitors:        make(map[string]*AgentMonitor),
		rollbackHistory: make([]*RollbackEvent, 0),
		handlers:        make([]RollbackHandler, 0),
		metrics: &RollbackMetrics{
			RollbacksPerAgent: make(map[string]int),
		},
	}
}

// DefaultRollbackConfig returns default rollback configuration.
func DefaultRollbackConfig() *RollbackConfig {
	return &RollbackConfig{
		Enabled:                  true,
		MonitoringWindowMinutes:  60,
		MinExecutionsForRollback: 10,
		PerformanceDropThreshold: 0.15, // 15% drop triggers rollback
		SuccessRateDropThreshold: 0.10, // 10% drop triggers rollback
		ErrorRateThreshold:       0.20, // 20% error rate triggers rollback
		CooldownMinutes:          30,
		MaxRollbacksPerDay:       3,
		AlertOnRollback:          true,
		GracePeriodMinutes:       5,
	}
}

// RegisterHandler registers a rollback handler.
func (m *RollbackMonitor) RegisterHandler(handler RollbackHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

// StartMonitoring starts monitoring an agent after a change is applied.
func (m *RollbackMonitor) StartMonitoring(
	ctx context.Context,
	agentID string,
	proposalID string,
	baselineScore float64,
	baselineSuccessRate float64,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil
	}

	now := time.Now()
	m.monitors[agentID] = &AgentMonitor{
		AgentID:         agentID,
		BaselineScore:   baselineScore,
		BaselineSuccess: baselineSuccessRate,
		MonitoringStart: now,
		LastProposalID:  proposalID,
		InGracePeriod:   true,
		GraceEndTime:    now.Add(time.Duration(m.config.GracePeriodMinutes) * time.Minute),
	}

	return nil
}

// RecordExecution records an execution for monitoring.
func (m *RollbackMonitor) RecordExecution(
	ctx context.Context,
	agentID string,
	score float64,
	success bool,
	isError bool,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil
	}

	monitor, ok := m.monitors[agentID]
	if !ok {
		return nil // Not monitoring this agent
	}

	// Check if still in grace period
	if monitor.InGracePeriod {
		if time.Now().Before(monitor.GraceEndTime) {
			return nil // Still in grace period
		}
		monitor.InGracePeriod = false
	}

	// Update metrics
	monitor.ExecutionCount++
	if isError {
		monitor.ErrorCount++
	}

	// Update running averages
	alpha := 0.1 // Exponential moving average weight
	if monitor.CurrentScore == 0 {
		monitor.CurrentScore = score
	} else {
		monitor.CurrentScore = monitor.CurrentScore*(1-alpha) + score*alpha
	}

	successVal := 0.0
	if success {
		successVal = 1.0
	}
	if monitor.CurrentSuccess == 0 {
		monitor.CurrentSuccess = successVal
	} else {
		monitor.CurrentSuccess = monitor.CurrentSuccess*(1-alpha) + successVal*alpha
	}

	// Check for rollback conditions
	if monitor.ExecutionCount >= m.config.MinExecutionsForRollback {
		if reason := m.shouldRollback(monitor); reason != "" {
			return m.triggerRollback(ctx, agentID, reason, true)
		}
	}

	return nil
}

// shouldRollback checks if rollback conditions are met.
func (m *RollbackMonitor) shouldRollback(monitor *AgentMonitor) string {
	// Check cooldown
	if !monitor.LastRollback.IsZero() {
		cooldown := time.Duration(m.config.CooldownMinutes) * time.Minute
		if time.Since(monitor.LastRollback) < cooldown {
			return ""
		}
	}

	// Check max rollbacks per day
	if monitor.RollbacksToday >= m.config.MaxRollbacksPerDay {
		return ""
	}

	// Check performance drop
	if monitor.BaselineScore > 0 {
		scoreDrop := (monitor.BaselineScore - monitor.CurrentScore) / monitor.BaselineScore
		if scoreDrop > m.config.PerformanceDropThreshold {
			return fmt.Sprintf("Performance dropped by %.1f%% (threshold: %.1f%%)",
				scoreDrop*100, m.config.PerformanceDropThreshold*100)
		}
	}

	// Check success rate drop
	if monitor.BaselineSuccess > 0 {
		successDrop := (monitor.BaselineSuccess - monitor.CurrentSuccess) / monitor.BaselineSuccess
		if successDrop > m.config.SuccessRateDropThreshold {
			return fmt.Sprintf("Success rate dropped by %.1f%% (threshold: %.1f%%)",
				successDrop*100, m.config.SuccessRateDropThreshold*100)
		}
	}

	// Check error rate
	if monitor.ExecutionCount > 0 {
		errorRate := float64(monitor.ErrorCount) / float64(monitor.ExecutionCount)
		if errorRate > m.config.ErrorRateThreshold {
			return fmt.Sprintf("Error rate %.1f%% exceeds threshold %.1f%%",
				errorRate*100, m.config.ErrorRateThreshold*100)
		}
	}

	return ""
}

// triggerRollback triggers a rollback for an agent.
func (m *RollbackMonitor) triggerRollback(
	ctx context.Context,
	agentID string,
	reason string,
	automatic bool,
) error {
	monitor := m.monitors[agentID]

	// Get version info before rollback
	currentVersion := m.versionManager.GetActiveVersion(agentID)
	var fromVersionID string
	if currentVersion != nil {
		fromVersionID = currentVersion.ID
	}

	// Perform rollback
	if err := m.versionManager.RollbackToPrevious(agentID, reason); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	// Get new version info
	newVersion := m.versionManager.GetActiveVersion(agentID)
	var toVersionID string
	if newVersion != nil {
		toVersionID = newVersion.ID
	}

	// Record rollback event
	event := &RollbackEvent{
		ID:            fmt.Sprintf("rollback-%s-%d", agentID, time.Now().UnixNano()),
		AgentID:       agentID,
		Timestamp:     time.Now(),
		Reason:        reason,
		FromVersionID: fromVersionID,
		ToVersionID:   toVersionID,
		ProposalID:    monitor.LastProposalID,
		ScoreBefore:   monitor.CurrentScore,
		ScoreAfter:    monitor.BaselineScore, // Expected to return to baseline
		SuccessBefore: monitor.CurrentSuccess,
		SuccessAfter:  monitor.BaselineSuccess,
		Automatic:     automatic,
	}

	m.rollbackHistory = append(m.rollbackHistory, event)

	// Update metrics
	m.metrics.TotalRollbacks++
	if automatic {
		m.metrics.AutomaticRollbacks++
	} else {
		m.metrics.ManualRollbacks++
	}
	m.metrics.RollbacksPerAgent[agentID]++
	m.metrics.PreventedRegressions++

	// Update monitor
	monitor.LastRollback = time.Now()
	monitor.RollbacksToday++
	monitor.ExecutionCount = 0
	monitor.ErrorCount = 0
	monitor.CurrentScore = 0
	monitor.CurrentSuccess = 0

	// Mark proposal as rolled back
	if monitor.LastProposalID != "" {
		if proposal, err := m.proposalStore.Get(monitor.LastProposalID); err == nil && proposal != nil {
			proposal.Rollback(reason)
			m.proposalStore.Save(proposal)
		}
	}

	// Notify handlers
	for _, handler := range m.handlers {
		handler.OnRollback(ctx, event)
	}

	return nil
}

// ManualRollback triggers a manual rollback.
func (m *RollbackMonitor) ManualRollback(
	ctx context.Context,
	agentID string,
	reason string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure monitor exists
	if _, ok := m.monitors[agentID]; !ok {
		m.monitors[agentID] = &AgentMonitor{
			AgentID: agentID,
		}
	}

	return m.triggerRollback(ctx, agentID, "Manual: "+reason, false)
}

// StopMonitoring stops monitoring an agent.
func (m *RollbackMonitor) StopMonitoring(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.monitors, agentID)
}

// GetMonitor returns the monitor for an agent.
func (m *RollbackMonitor) GetMonitor(agentID string) *AgentMonitor {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if monitor, ok := m.monitors[agentID]; ok {
		// Return a copy
		copy := *monitor
		return &copy
	}
	return nil
}

// GetRollbackHistory returns rollback history.
func (m *RollbackMonitor) GetRollbackHistory(agentID string, limit int) []*RollbackEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*RollbackEvent
	for _, event := range m.rollbackHistory {
		if agentID == "" || event.AgentID == agentID {
			result = append(result, event)
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}

	return result
}

// GetMetrics returns rollback metrics.
func (m *RollbackMonitor) GetMetrics() *RollbackMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy
	return &RollbackMetrics{
		TotalRollbacks:       m.metrics.TotalRollbacks,
		AutomaticRollbacks:   m.metrics.AutomaticRollbacks,
		ManualRollbacks:      m.metrics.ManualRollbacks,
		RollbacksPerAgent:    m.metrics.RollbacksPerAgent,
		AverageRecoveryTime:  m.metrics.AverageRecoveryTime,
		PreventedRegressions: m.metrics.PreventedRegressions,
	}
}

// ResetDailyCounters resets daily rollback counters.
func (m *RollbackMonitor) ResetDailyCounters() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, monitor := range m.monitors {
		monitor.RollbacksToday = 0
	}
}

// HealthCheck performs a health check on all monitored agents.
func (m *RollbackMonitor) HealthCheck(ctx context.Context) *HealthReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &HealthReport{
		Timestamp:       time.Now(),
		AgentStatuses:   make(map[string]*AgentHealthStatus),
		TotalAgents:     len(m.monitors),
		HealthyAgents:   0,
		UnhealthyAgents: 0,
	}

	for agentID, monitor := range m.monitors {
		status := &AgentHealthStatus{
			AgentID:       agentID,
			IsHealthy:     true,
			CurrentScore:  monitor.CurrentScore,
			BaselineScore: monitor.BaselineScore,
			SuccessRate:   monitor.CurrentSuccess,
			ErrorRate:     0,
		}

		if monitor.ExecutionCount > 0 {
			status.ErrorRate = float64(monitor.ErrorCount) / float64(monitor.ExecutionCount)
		}

		// Determine health
		if monitor.ExecutionCount >= m.config.MinExecutionsForRollback {
			if reason := m.shouldRollback(monitor); reason != "" {
				status.IsHealthy = false
				status.HealthIssue = reason
			}
		}

		if status.IsHealthy {
			report.HealthyAgents++
		} else {
			report.UnhealthyAgents++
		}

		report.AgentStatuses[agentID] = status
	}

	return report
}

// HealthReport contains health check results.
type HealthReport struct {
	Timestamp       time.Time                      `json:"timestamp"`
	AgentStatuses   map[string]*AgentHealthStatus  `json:"agent_statuses"`
	TotalAgents     int                            `json:"total_agents"`
	HealthyAgents   int                            `json:"healthy_agents"`
	UnhealthyAgents int                            `json:"unhealthy_agents"`
}

// AgentHealthStatus contains health status for an agent.
type AgentHealthStatus struct {
	AgentID       string  `json:"agent_id"`
	IsHealthy     bool    `json:"is_healthy"`
	HealthIssue   string  `json:"health_issue,omitempty"`
	CurrentScore  float64 `json:"current_score"`
	BaselineScore float64 `json:"baseline_score"`
	SuccessRate   float64 `json:"success_rate"`
	ErrorRate     float64 `json:"error_rate"`
}

// LoggingRollbackHandler logs rollback events.
type LoggingRollbackHandler struct {
	LogFunc func(format string, args ...interface{})
}

// OnRollback logs the rollback event.
func (h *LoggingRollbackHandler) OnRollback(ctx context.Context, event *RollbackEvent) error {
	if h.LogFunc != nil {
		h.LogFunc("Rollback triggered for agent %s: %s (from: %s, to: %s, automatic: %v)",
			event.AgentID, event.Reason, event.FromVersionID, event.ToVersionID, event.Automatic)
	}
	return nil
}

// AlertingRollbackHandler sends alerts on rollback.
type AlertingRollbackHandler struct {
	AlertFunc func(title string, message string, severity string) error
}

// OnRollback sends an alert.
func (h *AlertingRollbackHandler) OnRollback(ctx context.Context, event *RollbackEvent) error {
	if h.AlertFunc != nil {
		severity := "warning"
		if event.Automatic {
			severity = "critical"
		}
		return h.AlertFunc(
			fmt.Sprintf("Agent Rollback: %s", event.AgentID),
			fmt.Sprintf("Reason: %s\nScore before: %.2f, after: %.2f",
				event.Reason, event.ScoreBefore, event.ScoreAfter),
			severity,
		)
	}
	return nil
}
