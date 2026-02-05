package sandbox

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// IsolationLevel defines the level of isolation
type IsolationLevel string

const (
	// IsolationLevelNone provides no isolation (for trusted agents)
	IsolationLevelNone IsolationLevel = "none"

	// IsolationLevelBasic provides permission checking only
	IsolationLevelBasic IsolationLevel = "basic"

	// IsolationLevelStandard provides permission and resource limits
	IsolationLevelStandard IsolationLevel = "standard"

	// IsolationLevelStrict provides all protections plus audit logging
	IsolationLevelStrict IsolationLevel = "strict"

	// IsolationLevelMaximum provides maximum isolation (for untrusted agents)
	IsolationLevelMaximum IsolationLevel = "maximum"
)

// IsolationPolicy defines isolation settings for different scenarios
type IsolationPolicy struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	Level           IsolationLevel  `json:"level"`
	Limits          *ResourceLimits `json:"limits"`
	Permissions     *Permissions    `json:"permissions"`
	AuditEnabled    bool            `json:"audit_enabled"`
	EnforceOnError  bool            `json:"enforce_on_error"` // Enforce limits even on errors
	AlertThresholds *AlertThresholds `json:"alert_thresholds,omitempty"`
}

// AlertThresholds defines when to send alerts
type AlertThresholds struct {
	MemoryUsagePercent   float64 `json:"memory_usage_percent"`   // Alert when memory usage exceeds this %
	CPUUsagePercent      float64 `json:"cpu_usage_percent"`      // Alert when CPU usage exceeds this %
	ExecutionTimePercent float64 `json:"execution_time_percent"` // Alert when execution time exceeds this %
	ViolationCount       int     `json:"violation_count"`        // Alert after this many violations
}

// DefaultAlertThresholds returns default alert thresholds
func DefaultAlertThresholds() *AlertThresholds {
	return &AlertThresholds{
		MemoryUsagePercent:   80,
		CPUUsagePercent:      80,
		ExecutionTimePercent: 80,
		ViolationCount:       3,
	}
}

// IsolationPolicyFromLevel creates an isolation policy from a level
func IsolationPolicyFromLevel(level IsolationLevel) *IsolationPolicy {
	policy := &IsolationPolicy{
		Level:        level,
		AuditEnabled: level == IsolationLevelStrict || level == IsolationLevelMaximum,
	}

	switch level {
	case IsolationLevelNone:
		policy.Limits = nil
		policy.Permissions = &Permissions{
			AllowFileRead:       true,
			AllowFileWrite:      true,
			AllowNetworkAccess:  true,
			AllowShellExecution: true,
			AllowCodeExecution:  true,
		}

	case IsolationLevelBasic:
		policy.Limits = &ResourceLimits{
			MaxExecutionTime:     30 * time.Minute,
			MaxConcurrentTasks:   50,
			MaxAPICallsPerMinute: 1000,
		}
		policy.Permissions = &Permissions{
			AllowFileRead:       true,
			AllowFileWrite:      true,
			AllowNetworkAccess:  true,
			AllowShellExecution: false,
			AllowCodeExecution:  false,
		}

	case IsolationLevelStandard:
		policy.Limits = DefaultResourceLimits()
		policy.Permissions = &Permissions{
			AllowFileRead:       true,
			AllowFileWrite:      false,
			AllowNetworkAccess:  true,
			AllowShellExecution: false,
			AllowCodeExecution:  false,
		}
		policy.AlertThresholds = DefaultAlertThresholds()

	case IsolationLevelStrict:
		policy.Limits = &ResourceLimits{
			MaxMemoryMB:          256,
			MaxCPUPercent:        50,
			MaxExecutionTime:     2 * time.Minute,
			MaxConcurrentTasks:   5,
			MaxAPICallsPerMinute: 50,
			MaxTokensPerMinute:   10000,
			MaxFileSize:          1024 * 1024, // 1MB
		}
		policy.Permissions = &Permissions{
			AllowFileRead:       true,
			AllowFileWrite:      false,
			AllowNetworkAccess:  false,
			AllowShellExecution: false,
			AllowCodeExecution:  false,
		}
		policy.AlertThresholds = &AlertThresholds{
			MemoryUsagePercent:   60,
			CPUUsagePercent:      60,
			ExecutionTimePercent: 60,
			ViolationCount:       1,
		}
		policy.EnforceOnError = true

	case IsolationLevelMaximum:
		policy.Limits = &ResourceLimits{
			MaxMemoryMB:          128,
			MaxCPUPercent:        25,
			MaxExecutionTime:     30 * time.Second,
			MaxConcurrentTasks:   2,
			MaxAPICallsPerMinute: 20,
			MaxTokensPerMinute:   5000,
			MaxFileSize:          100 * 1024, // 100KB
		}
		policy.Permissions = DefaultPermissions()
		policy.AlertThresholds = &AlertThresholds{
			MemoryUsagePercent:   50,
			CPUUsagePercent:      50,
			ExecutionTimePercent: 50,
			ViolationCount:       1,
		}
		policy.EnforceOnError = true
	}

	return policy
}

// ResourceMonitor monitors resource usage in real-time
type ResourceMonitor struct {
	sandbox      *Sandbox
	interval     time.Duration
	handlers     []MonitorHandler
	alertHandler AlertHandler
	stopCh       chan struct{}
	mu           sync.RWMutex
}

// MonitorHandler handles monitoring events
type MonitorHandler func(usage *ResourceUsage, limits *ResourceLimits)

// AlertHandler handles alerts
type AlertHandler func(alert *ResourceAlert)

// ResourceAlert represents a resource usage alert
type ResourceAlert struct {
	SandboxID   string                 `json:"sandbox_id"`
	AlertType   AlertType              `json:"alert_type"`
	Resource    ResourceType           `json:"resource"`
	CurrentValue float64               `json:"current_value"`
	Threshold   float64                `json:"threshold"`
	Message     string                 `json:"message"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	OccurredAt  time.Time              `json:"occurred_at"`
}

// AlertType represents the type of alert
type AlertType string

const (
	AlertTypeWarning  AlertType = "warning"
	AlertTypeCritical AlertType = "critical"
)

// MonitorConfig configures the resource monitor
type MonitorConfig struct {
	Sandbox      *Sandbox
	Interval     time.Duration
	AlertHandler AlertHandler
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(config MonitorConfig) *ResourceMonitor {
	if config.Interval == 0 {
		config.Interval = 5 * time.Second
	}

	return &ResourceMonitor{
		sandbox:      config.Sandbox,
		interval:     config.Interval,
		handlers:     make([]MonitorHandler, 0),
		alertHandler: config.AlertHandler,
		stopCh:       make(chan struct{}),
	}
}

// Start starts the resource monitor
func (m *ResourceMonitor) Start(ctx context.Context) {
	go m.monitorLoop(ctx)
}

// Stop stops the resource monitor
func (m *ResourceMonitor) Stop() {
	close(m.stopCh)
}

// AddHandler adds a monitoring handler
func (m *ResourceMonitor) AddHandler(handler MonitorHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

// monitorLoop runs the monitoring loop
func (m *ResourceMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkResources()
		}
	}
}

// checkResources checks current resource usage
func (m *ResourceMonitor) checkResources() {
	usage := m.sandbox.GetUsage()
	limits := m.sandbox.limits

	// Call handlers
	m.mu.RLock()
	handlers := make([]MonitorHandler, len(m.handlers))
	copy(handlers, m.handlers)
	m.mu.RUnlock()

	for _, handler := range handlers {
		handler(usage, limits)
	}

	// Check for alerts if alert handler is configured
	if m.alertHandler == nil {
		return
	}

	// Check memory
	if limits.MaxMemoryMB > 0 {
		memoryPercent := float64(usage.MemoryMB) / float64(limits.MaxMemoryMB) * 100
		if memoryPercent >= 90 {
			m.alertHandler(&ResourceAlert{
				SandboxID:    m.sandbox.ID,
				AlertType:    AlertTypeCritical,
				Resource:     ResourceTypeMemory,
				CurrentValue: memoryPercent,
				Threshold:    90,
				Message:      fmt.Sprintf("Memory usage at %.1f%%", memoryPercent),
				OccurredAt:   time.Now(),
			})
		} else if memoryPercent >= 80 {
			m.alertHandler(&ResourceAlert{
				SandboxID:    m.sandbox.ID,
				AlertType:    AlertTypeWarning,
				Resource:     ResourceTypeMemory,
				CurrentValue: memoryPercent,
				Threshold:    80,
				Message:      fmt.Sprintf("Memory usage at %.1f%%", memoryPercent),
				OccurredAt:   time.Now(),
			})
		}
	}

	// Check execution time
	if limits.MaxExecutionTime > 0 {
		timePercent := float64(usage.ExecutionTime) / float64(limits.MaxExecutionTime) * 100
		if timePercent >= 90 {
			m.alertHandler(&ResourceAlert{
				SandboxID:    m.sandbox.ID,
				AlertType:    AlertTypeCritical,
				Resource:     ResourceTypeExecutionTime,
				CurrentValue: timePercent,
				Threshold:    90,
				Message:      fmt.Sprintf("Execution time at %.1f%%", timePercent),
				OccurredAt:   time.Now(),
			})
		}
	}
}

// AuditLog records audit events
type AuditLog struct {
	entries   []*AuditEntry
	maxSize   int
	handler   AuditHandler
	mu        sync.RWMutex
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	ID          string                 `json:"id"`
	SandboxID   string                 `json:"sandbox_id"`
	Action      string                 `json:"action"`
	Resource    string                 `json:"resource,omitempty"`
	Result      AuditResult            `json:"result"`
	Details     string                 `json:"details,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// AuditResult represents the result of an audited action
type AuditResult string

const (
	AuditResultAllowed AuditResult = "allowed"
	AuditResultDenied  AuditResult = "denied"
	AuditResultLimited AuditResult = "limited"
)

// AuditHandler handles audit events
type AuditHandler func(entry *AuditEntry)

// NewAuditLog creates a new audit log
func NewAuditLog(maxSize int, handler AuditHandler) *AuditLog {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &AuditLog{
		entries: make([]*AuditEntry, 0),
		maxSize: maxSize,
		handler: handler,
	}
}

// Record records an audit entry
func (a *AuditLog) Record(sandboxID, action, resource string, result AuditResult, details string, metadata map[string]interface{}) {
	entry := &AuditEntry{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		SandboxID: sandboxID,
		Action:    action,
		Resource:  resource,
		Result:    result,
		Details:   details,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}

	a.mu.Lock()
	a.entries = append(a.entries, entry)
	if len(a.entries) > a.maxSize {
		a.entries = a.entries[len(a.entries)-a.maxSize:]
	}
	a.mu.Unlock()

	if a.handler != nil {
		a.handler(entry)
	}
}

// GetEntries returns audit entries
func (a *AuditLog) GetEntries(sandboxID string, limit int) []*AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	results := make([]*AuditEntry, 0)
	for i := len(a.entries) - 1; i >= 0 && len(results) < limit; i-- {
		if sandboxID == "" || a.entries[i].SandboxID == sandboxID {
			results = append(results, a.entries[i])
		}
	}
	return results
}

// Clear clears the audit log
func (a *AuditLog) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = make([]*AuditEntry, 0)
}

// SecureSandbox wraps a sandbox with additional security features
type SecureSandbox struct {
	*Sandbox
	policy   *IsolationPolicy
	monitor  *ResourceMonitor
	auditLog *AuditLog
}

// SecureSandboxConfig configures a secure sandbox
type SecureSandboxConfig struct {
	Name         string
	Policy       *IsolationPolicy
	AlertHandler AlertHandler
	AuditHandler AuditHandler
}

// NewSecureSandbox creates a new secure sandbox
func NewSecureSandbox(config SecureSandboxConfig) *SecureSandbox {
	if config.Policy == nil {
		config.Policy = IsolationPolicyFromLevel(IsolationLevelStandard)
	}

	sandbox := NewSandbox(&SandboxConfig{
		Name:        config.Name,
		Limits:      config.Policy.Limits,
		Permissions: config.Policy.Permissions,
	})

	ss := &SecureSandbox{
		Sandbox: sandbox,
		policy:  config.Policy,
	}

	// Set up monitoring
	ss.monitor = NewResourceMonitor(MonitorConfig{
		Sandbox:      sandbox,
		AlertHandler: config.AlertHandler,
	})

	// Set up audit logging
	if config.Policy.AuditEnabled {
		ss.auditLog = NewAuditLog(10000, config.AuditHandler)
	}

	return ss
}

// Start starts the secure sandbox
func (s *SecureSandbox) Start(ctx context.Context) error {
	if err := s.Sandbox.Start(); err != nil {
		return err
	}

	s.monitor.Start(ctx)
	return nil
}

// Stop stops the secure sandbox
func (s *SecureSandbox) Stop() error {
	s.monitor.Stop()
	return s.Sandbox.Stop()
}

// CheckPermission checks permission with audit logging
func (s *SecureSandbox) CheckPermission(action PermissionAction, resource string) error {
	err := s.Sandbox.CheckPermission(action, resource)

	if s.auditLog != nil {
		result := AuditResultAllowed
		details := ""
		if err != nil {
			result = AuditResultDenied
			details = err.Error()
		}
		s.auditLog.Record(s.ID, string(action), resource, result, details, nil)
	}

	return err
}

// GetAuditLog returns the audit log
func (s *SecureSandbox) GetAuditLog() *AuditLog {
	return s.auditLog
}

// GetPolicy returns the isolation policy
func (s *SecureSandbox) GetPolicy() *IsolationPolicy {
	return s.policy
}
