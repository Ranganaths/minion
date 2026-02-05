package observability

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SLOType represents the type of SLO
type SLOType string

const (
	SLOTypeAvailability SLOType = "availability"
	SLOTypeLatency      SLOType = "latency"
	SLOTypeThroughput   SLOType = "throughput"
	SLOTypeErrorRate    SLOType = "error_rate"
	SLOTypeQuality      SLOType = "quality"
)

// SLO represents a Service Level Objective
type SLO struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Type        SLOType           `json:"type"`
	Target      float64           `json:"target"`      // Target percentage (e.g., 99.9)
	Window      time.Duration     `json:"window"`      // Rolling window (e.g., 30 days)
	Labels      map[string]string `json:"labels,omitempty"`
	BurnRate    *BurnRateConfig   `json:"burn_rate,omitempty"`
	AlertRules  []SLOAlertRule    `json:"alert_rules,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// BurnRateConfig configures burn rate alerting
type BurnRateConfig struct {
	ShortWindow    time.Duration `json:"short_window"`     // e.g., 1 hour
	LongWindow     time.Duration `json:"long_window"`      // e.g., 6 hours
	FastBurnRate   float64       `json:"fast_burn_rate"`   // e.g., 14.4 (critical)
	SlowBurnRate   float64       `json:"slow_burn_rate"`   // e.g., 6 (warning)
}

// SLOAlertRule defines when to alert on SLO violations
type SLOAlertRule struct {
	Name      string        `json:"name"`
	Severity  string        `json:"severity"` // critical, warning, info
	Threshold float64       `json:"threshold"`
	Duration  time.Duration `json:"duration"`
	Message   string        `json:"message"`
}

// SLI represents a Service Level Indicator measurement
type SLI struct {
	SLOID       string    `json:"slo_id"`
	Timestamp   time.Time `json:"timestamp"`
	Total       int64     `json:"total"`       // Total events
	Good        int64     `json:"good"`        // Events meeting the objective
	Value       float64   `json:"value"`       // Computed SLI value (0-1)
}

// SLOStatus represents the current status of an SLO
type SLOStatus struct {
	SLO             *SLO      `json:"slo"`
	CurrentValue    float64   `json:"current_value"`    // Current SLI as percentage
	Target          float64   `json:"target"`           // Target percentage
	ErrorBudget     float64   `json:"error_budget"`     // Remaining error budget %
	BurnRate        float64   `json:"burn_rate"`        // Current burn rate
	IsHealthy       bool      `json:"is_healthy"`
	Alerts          []string  `json:"alerts,omitempty"`
	LastUpdated     time.Time `json:"last_updated"`
}

// SLOManager manages SLOs and tracks their status
type SLOManager struct {
	slos        map[string]*SLO
	measurements map[string][]*SLI
	metrics     *MetricsCollector
	alertCh     chan *SLOAlert
	mu          sync.RWMutex
}

// SLOAlert represents an SLO alert
type SLOAlert struct {
	SLOID      string    `json:"slo_id"`
	SLOName    string    `json:"slo_name"`
	Severity   string    `json:"severity"`
	Message    string    `json:"message"`
	Value      float64   `json:"value"`
	Target     float64   `json:"target"`
	BurnRate   float64   `json:"burn_rate,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

// NewSLOManager creates a new SLO manager
func NewSLOManager(metrics *MetricsCollector) *SLOManager {
	return &SLOManager{
		slos:        make(map[string]*SLO),
		measurements: make(map[string][]*SLI),
		metrics:     metrics,
		alertCh:     make(chan *SLOAlert, 100),
	}
}

// RegisterSLO registers a new SLO
func (m *SLOManager) RegisterSLO(slo *SLO) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if slo.ID == "" {
		return fmt.Errorf("SLO ID is required")
	}

	if slo.CreatedAt.IsZero() {
		slo.CreatedAt = time.Now()
	}
	slo.UpdatedAt = time.Now()

	m.slos[slo.ID] = slo
	return nil
}

// GetSLO retrieves an SLO by ID
func (m *SLOManager) GetSLO(id string) (*SLO, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	slo, ok := m.slos[id]
	if !ok {
		return nil, fmt.Errorf("SLO not found: %s", id)
	}
	return slo, nil
}

// ListSLOs returns all registered SLOs
func (m *SLOManager) ListSLOs() []*SLO {
	m.mu.RLock()
	defer m.mu.RUnlock()

	slos := make([]*SLO, 0, len(m.slos))
	for _, slo := range m.slos {
		slos = append(slos, slo)
	}
	return slos
}

// RecordMeasurement records an SLI measurement
func (m *SLOManager) RecordMeasurement(sloID string, total, good int64) error {
	m.mu.Lock()

	slo, ok := m.slos[sloID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("SLO not found: %s", sloID)
	}

	sli := &SLI{
		SLOID:     sloID,
		Timestamp: time.Now(),
		Total:     total,
		Good:      good,
	}

	if total > 0 {
		sli.Value = float64(good) / float64(total)
	}

	m.measurements[sloID] = append(m.measurements[sloID], sli)

	// Prune old measurements outside the window
	cutoff := time.Now().Add(-slo.Window)
	pruned := make([]*SLI, 0)
	for _, measurement := range m.measurements[sloID] {
		if measurement.Timestamp.After(cutoff) {
			pruned = append(pruned, measurement)
		}
	}
	m.measurements[sloID] = pruned

	// Compute status while holding the lock to avoid deadlock
	status := m.computeStatusLocked(sloID, slo)

	m.mu.Unlock()

	// Check for alerts (outside of lock, using pre-computed status)
	m.checkAlertsWithStatus(slo, status)

	return nil
}

// GetStatus returns the current status of an SLO
func (m *SLOManager) GetStatus(sloID string) (*SLOStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	slo, ok := m.slos[sloID]
	if !ok {
		return nil, fmt.Errorf("SLO not found: %s", sloID)
	}

	return m.computeStatusLocked(sloID, slo), nil
}

// computeStatusLocked computes SLO status while caller holds the lock
func (m *SLOManager) computeStatusLocked(sloID string, slo *SLO) *SLOStatus {
	measurements := m.measurements[sloID]
	if len(measurements) == 0 {
		return &SLOStatus{
			SLO:          slo,
			CurrentValue: 100.0,
			Target:       slo.Target,
			ErrorBudget:  100.0,
			IsHealthy:    true,
			LastUpdated:  time.Now(),
		}
	}

	// Calculate current SLI
	var totalGood, totalEvents int64
	for _, measurement := range measurements {
		totalGood += measurement.Good
		totalEvents += measurement.Total
	}

	currentValue := 100.0
	if totalEvents > 0 {
		currentValue = (float64(totalGood) / float64(totalEvents)) * 100
	}

	// Calculate error budget
	// Error budget = (current - target) / (100 - target) * 100
	targetRate := slo.Target / 100
	currentRate := currentValue / 100
	maxErrorRate := 1 - targetRate
	currentErrorRate := 1 - currentRate
	errorBudget := 100.0
	if maxErrorRate > 0 {
		errorBudget = ((maxErrorRate - currentErrorRate) / maxErrorRate) * 100
	}

	// Calculate burn rate
	burnRate := m.calculateBurnRate(sloID, slo)

	var alerts []string
	isHealthy := currentValue >= slo.Target && errorBudget > 0

	if !isHealthy {
		alerts = append(alerts, fmt.Sprintf("SLO violation: current %.2f%% < target %.2f%%", currentValue, slo.Target))
	}
	if burnRate > 1.0 {
		alerts = append(alerts, fmt.Sprintf("Elevated burn rate: %.2fx", burnRate))
	}

	return &SLOStatus{
		SLO:          slo,
		CurrentValue: currentValue,
		Target:       slo.Target,
		ErrorBudget:  errorBudget,
		BurnRate:     burnRate,
		IsHealthy:    isHealthy,
		Alerts:       alerts,
		LastUpdated:  time.Now(),
	}
}

// calculateBurnRate calculates the current burn rate
func (m *SLOManager) calculateBurnRate(sloID string, slo *SLO) float64 {
	measurements := m.measurements[sloID]
	if len(measurements) == 0 || slo.BurnRate == nil {
		return 0
	}

	// Get measurements in short window
	shortWindowStart := time.Now().Add(-slo.BurnRate.ShortWindow)
	var shortGood, shortTotal int64
	for _, measurement := range measurements {
		if measurement.Timestamp.After(shortWindowStart) {
			shortGood += measurement.Good
			shortTotal += measurement.Total
		}
	}

	if shortTotal == 0 {
		return 0
	}

	// Calculate error rate in short window
	errorRate := 1 - (float64(shortGood) / float64(shortTotal))

	// Acceptable error rate = 1 - target
	acceptableErrorRate := 1 - (slo.Target / 100)
	if acceptableErrorRate <= 0 {
		return 0
	}

	// Burn rate = actual error rate / acceptable error rate
	return errorRate / acceptableErrorRate
}

// checkAlerts checks if any alert rules are triggered (deprecated, use checkAlertsWithStatus)
func (m *SLOManager) checkAlerts(slo *SLO) {
	status, _ := m.GetStatus(slo.ID)
	if status == nil {
		return
	}
	m.checkAlertsWithStatus(slo, status)
}

// checkAlertsWithStatus checks if any alert rules are triggered using pre-computed status
func (m *SLOManager) checkAlertsWithStatus(slo *SLO, status *SLOStatus) {
	if status == nil {
		return
	}

	for _, rule := range slo.AlertRules {
		triggered := false
		var message string

		switch slo.Type {
		case SLOTypeAvailability, SLOTypeErrorRate:
			if status.CurrentValue < rule.Threshold {
				triggered = true
				message = fmt.Sprintf("%s: current %.2f%% < threshold %.2f%%", rule.Name, status.CurrentValue, rule.Threshold)
			}
		case SLOTypeLatency:
			// For latency, higher is worse
			if status.CurrentValue > rule.Threshold {
				triggered = true
				message = fmt.Sprintf("%s: current %.2fms > threshold %.2fms", rule.Name, status.CurrentValue, rule.Threshold)
			}
		}

		// Check burn rate alerts
		if slo.BurnRate != nil && status.BurnRate > 0 {
			if status.BurnRate >= slo.BurnRate.FastBurnRate {
				triggered = true
				message = fmt.Sprintf("Fast burn rate: %.2fx (threshold: %.2fx)", status.BurnRate, slo.BurnRate.FastBurnRate)
			} else if status.BurnRate >= slo.BurnRate.SlowBurnRate && rule.Severity == "warning" {
				triggered = true
				message = fmt.Sprintf("Slow burn rate: %.2fx (threshold: %.2fx)", status.BurnRate, slo.BurnRate.SlowBurnRate)
			}
		}

		if triggered {
			alert := &SLOAlert{
				SLOID:      slo.ID,
				SLOName:    slo.Name,
				Severity:   rule.Severity,
				Message:    message,
				Value:      status.CurrentValue,
				Target:     status.Target,
				BurnRate:   status.BurnRate,
				OccurredAt: time.Now(),
			}

			select {
			case m.alertCh <- alert:
			default:
				// Channel full, drop alert
			}
		}
	}
}

// GetAlertChannel returns the alert channel
func (m *SLOManager) GetAlertChannel() <-chan *SLOAlert {
	return m.alertCh
}

// GetAllStatuses returns the status of all SLOs
func (m *SLOManager) GetAllStatuses() []*SLOStatus {
	m.mu.RLock()
	sloIDs := make([]string, 0, len(m.slos))
	for id := range m.slos {
		sloIDs = append(sloIDs, id)
	}
	m.mu.RUnlock()

	statuses := make([]*SLOStatus, 0, len(sloIDs))
	for _, id := range sloIDs {
		status, err := m.GetStatus(id)
		if err == nil {
			statuses = append(statuses, status)
		}
	}
	return statuses
}

// DefaultAgentSLOs returns default SLOs for agent operations
func DefaultAgentSLOs() []*SLO {
	thirtyDays := 30 * 24 * time.Hour

	return []*SLO{
		{
			ID:          "agent-availability",
			Name:        "Agent Availability",
			Description: "Percentage of successful agent executions",
			Type:        SLOTypeAvailability,
			Target:      99.5,
			Window:      thirtyDays,
			BurnRate: &BurnRateConfig{
				ShortWindow:  time.Hour,
				LongWindow:   6 * time.Hour,
				FastBurnRate: 14.4,
				SlowBurnRate: 6.0,
			},
			AlertRules: []SLOAlertRule{
				{
					Name:      "Critical Availability",
					Severity:  "critical",
					Threshold: 99.0,
					Duration:  5 * time.Minute,
					Message:   "Agent availability critically low",
				},
				{
					Name:      "Warning Availability",
					Severity:  "warning",
					Threshold: 99.5,
					Duration:  15 * time.Minute,
					Message:   "Agent availability below target",
				},
			},
		},
		{
			ID:          "agent-latency-p99",
			Name:        "Agent P99 Latency",
			Description: "99th percentile agent response time",
			Type:        SLOTypeLatency,
			Target:      5000, // 5 seconds
			Window:      thirtyDays,
			AlertRules: []SLOAlertRule{
				{
					Name:      "High Latency",
					Severity:  "warning",
					Threshold: 10000, // 10 seconds
					Duration:  10 * time.Minute,
					Message:   "Agent latency exceeds threshold",
				},
			},
		},
		{
			ID:          "llm-error-rate",
			Name:        "LLM Error Rate",
			Description: "Percentage of successful LLM API calls",
			Type:        SLOTypeErrorRate,
			Target:      99.0,
			Window:      thirtyDays,
			BurnRate: &BurnRateConfig{
				ShortWindow:  time.Hour,
				LongWindow:   6 * time.Hour,
				FastBurnRate: 14.4,
				SlowBurnRate: 6.0,
			},
			AlertRules: []SLOAlertRule{
				{
					Name:      "High LLM Error Rate",
					Severity:  "critical",
					Threshold: 98.0,
					Duration:  5 * time.Minute,
					Message:   "LLM error rate exceeds threshold",
				},
			},
		},
		{
			ID:          "tool-success-rate",
			Name:        "Tool Success Rate",
			Description: "Percentage of successful tool executions",
			Type:        SLOTypeAvailability,
			Target:      99.0,
			Window:      thirtyDays,
			AlertRules: []SLOAlertRule{
				{
					Name:      "Low Tool Success Rate",
					Severity:  "warning",
					Threshold: 98.0,
					Duration:  10 * time.Minute,
					Message:   "Tool success rate below target",
				},
			},
		},
	}
}

// SLOReporter generates SLO reports
type SLOReporter struct {
	manager *SLOManager
}

// NewSLOReporter creates a new SLO reporter
func NewSLOReporter(manager *SLOManager) *SLOReporter {
	return &SLOReporter{manager: manager}
}

// GenerateReport generates a report for all SLOs
func (r *SLOReporter) GenerateReport(ctx context.Context) *SLOReport {
	statuses := r.manager.GetAllStatuses()

	report := &SLOReport{
		GeneratedAt: time.Now(),
		Statuses:    statuses,
	}

	// Calculate summary
	var healthy, unhealthy int
	var totalBudget float64
	for _, status := range statuses {
		if status.IsHealthy {
			healthy++
		} else {
			unhealthy++
		}
		totalBudget += status.ErrorBudget
	}

	report.Summary = SLOReportSummary{
		TotalSLOs:          len(statuses),
		HealthySLOs:        healthy,
		UnhealthySLOs:      unhealthy,
		AverageErrorBudget: totalBudget / float64(len(statuses)),
	}

	return report
}

// SLOReport represents an SLO report
type SLOReport struct {
	GeneratedAt time.Time         `json:"generated_at"`
	Summary     SLOReportSummary  `json:"summary"`
	Statuses    []*SLOStatus      `json:"statuses"`
}

// SLOReportSummary contains report summary
type SLOReportSummary struct {
	TotalSLOs          int     `json:"total_slos"`
	HealthySLOs        int     `json:"healthy_slos"`
	UnhealthySLOs      int     `json:"unhealthy_slos"`
	AverageErrorBudget float64 `json:"average_error_budget"`
}
