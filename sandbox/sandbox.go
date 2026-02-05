// Package sandbox provides agent sandboxing and resource limiting capabilities
// for secure and controlled agent execution.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// ResourceLimits defines resource constraints for an agent
type ResourceLimits struct {
	// MaxMemoryMB is the maximum memory in megabytes
	MaxMemoryMB int64 `json:"max_memory_mb"`

	// MaxCPUPercent is the maximum CPU percentage (0-100 per core)
	MaxCPUPercent float64 `json:"max_cpu_percent"`

	// MaxExecutionTime is the maximum execution time
	MaxExecutionTime time.Duration `json:"max_execution_time"`

	// MaxConcurrentTasks is the maximum number of concurrent tasks
	MaxConcurrentTasks int `json:"max_concurrent_tasks"`

	// MaxAPICallsPerMinute is the rate limit for API calls
	MaxAPICallsPerMinute int `json:"max_api_calls_per_minute"`

	// MaxTokensPerMinute is the rate limit for LLM tokens
	MaxTokensPerMinute int `json:"max_tokens_per_minute"`

	// MaxFileSize is the maximum file size in bytes that can be read/written
	MaxFileSize int64 `json:"max_file_size"`

	// MaxNetworkBandwidth is the maximum network bandwidth in bytes/second
	MaxNetworkBandwidth int64 `json:"max_network_bandwidth"`
}

// DefaultResourceLimits returns sensible default limits
func DefaultResourceLimits() *ResourceLimits {
	return &ResourceLimits{
		MaxMemoryMB:          512,
		MaxCPUPercent:        80,
		MaxExecutionTime:     5 * time.Minute,
		MaxConcurrentTasks:   10,
		MaxAPICallsPerMinute: 100,
		MaxTokensPerMinute:   50000,
		MaxFileSize:          10 * 1024 * 1024, // 10MB
		MaxNetworkBandwidth:  1024 * 1024,      // 1MB/s
	}
}

// Permissions defines what actions an agent is allowed to perform
type Permissions struct {
	// AllowedTools is a whitelist of tool names the agent can use
	AllowedTools []string `json:"allowed_tools,omitempty"`

	// DeniedTools is a blacklist of tool names the agent cannot use
	DeniedTools []string `json:"denied_tools,omitempty"`

	// AllowedPaths is a list of filesystem paths the agent can access
	AllowedPaths []string `json:"allowed_paths,omitempty"`

	// DeniedPaths is a list of filesystem paths the agent cannot access
	DeniedPaths []string `json:"denied_paths,omitempty"`

	// AllowedHosts is a list of network hosts the agent can connect to
	AllowedHosts []string `json:"allowed_hosts,omitempty"`

	// DeniedHosts is a list of network hosts the agent cannot connect to
	DeniedHosts []string `json:"denied_hosts,omitempty"`

	// AllowFileRead allows the agent to read files
	AllowFileRead bool `json:"allow_file_read"`

	// AllowFileWrite allows the agent to write files
	AllowFileWrite bool `json:"allow_file_write"`

	// AllowNetworkAccess allows the agent to make network requests
	AllowNetworkAccess bool `json:"allow_network_access"`

	// AllowShellExecution allows the agent to execute shell commands
	AllowShellExecution bool `json:"allow_shell_execution"`

	// AllowCodeExecution allows the agent to execute code
	AllowCodeExecution bool `json:"allow_code_execution"`
}

// DefaultPermissions returns restrictive default permissions
func DefaultPermissions() *Permissions {
	return &Permissions{
		AllowFileRead:       true,
		AllowFileWrite:      false,
		AllowNetworkAccess:  false,
		AllowShellExecution: false,
		AllowCodeExecution:  false,
	}
}

// SandboxConfig configures a sandbox
type SandboxConfig struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Limits      *ResourceLimits `json:"limits"`
	Permissions *Permissions    `json:"permissions"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Sandbox provides isolated execution environment for agents
type Sandbox struct {
	ID          string
	Name        string
	config      *SandboxConfig
	limits      *ResourceLimits
	permissions *Permissions
	usage       *ResourceUsage
	rateLimiter *RateLimiter
	state       SandboxState
	startTime   time.Time
	violations  []*Violation
	mu          sync.RWMutex
}

// SandboxState represents the state of a sandbox
type SandboxState string

const (
	SandboxStateCreated   SandboxState = "created"
	SandboxStateRunning   SandboxState = "running"
	SandboxStatePaused    SandboxState = "paused"
	SandboxStateStopped   SandboxState = "stopped"
	SandboxStateViolation SandboxState = "violation"
)

// ResourceUsage tracks current resource usage
type ResourceUsage struct {
	MemoryMB         int64         `json:"memory_mb"`
	CPUPercent       float64       `json:"cpu_percent"`
	ExecutionTime    time.Duration `json:"execution_time"`
	ConcurrentTasks  int32         `json:"concurrent_tasks"`
	APICallsThisMin  int32         `json:"api_calls_this_min"`
	TokensThisMin    int32         `json:"tokens_this_min"`
	BytesRead        int64         `json:"bytes_read"`
	BytesWritten     int64         `json:"bytes_written"`
	NetworkBytesSent int64         `json:"network_bytes_sent"`
	NetworkBytesRecv int64         `json:"network_bytes_recv"`
	mu               sync.RWMutex
}

// Violation records a security or resource violation
type Violation struct {
	ID          string                 `json:"id"`
	Type        ViolationType          `json:"type"`
	Description string                 `json:"description"`
	Severity    ViolationSeverity      `json:"severity"`
	Action      ViolationAction        `json:"action"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	OccurredAt  time.Time              `json:"occurred_at"`
}

// ViolationType represents the type of violation
type ViolationType string

const (
	ViolationTypeMemoryExceeded   ViolationType = "memory_exceeded"
	ViolationTypeCPUExceeded      ViolationType = "cpu_exceeded"
	ViolationTypeTimeExceeded     ViolationType = "time_exceeded"
	ViolationTypeRateLimitExceeded ViolationType = "rate_limit_exceeded"
	ViolationTypePermissionDenied ViolationType = "permission_denied"
	ViolationTypeUnauthorizedTool ViolationType = "unauthorized_tool"
	ViolationTypeUnauthorizedPath ViolationType = "unauthorized_path"
	ViolationTypeUnauthorizedHost ViolationType = "unauthorized_host"
)

// ViolationSeverity represents the severity of a violation
type ViolationSeverity string

const (
	ViolationSeverityLow      ViolationSeverity = "low"
	ViolationSeverityMedium   ViolationSeverity = "medium"
	ViolationSeverityHigh     ViolationSeverity = "high"
	ViolationSeverityCritical ViolationSeverity = "critical"
)

// ViolationAction represents what action was taken for a violation
type ViolationAction string

const (
	ViolationActionWarn     ViolationAction = "warn"
	ViolationActionThrottle ViolationAction = "throttle"
	ViolationActionBlock    ViolationAction = "block"
	ViolationActionTerminate ViolationAction = "terminate"
)

// NewSandbox creates a new sandbox with the given configuration
func NewSandbox(config *SandboxConfig) *Sandbox {
	if config.ID == "" {
		config.ID = uuid.New().String()
	}
	if config.Limits == nil {
		config.Limits = DefaultResourceLimits()
	}
	if config.Permissions == nil {
		config.Permissions = DefaultPermissions()
	}

	return &Sandbox{
		ID:          config.ID,
		Name:        config.Name,
		config:      config,
		limits:      config.Limits,
		permissions: config.Permissions,
		usage:       &ResourceUsage{},
		rateLimiter: NewRateLimiter(config.Limits.MaxAPICallsPerMinute, config.Limits.MaxTokensPerMinute),
		state:       SandboxStateCreated,
		violations:  make([]*Violation, 0),
	}
}

// Start starts the sandbox
func (s *Sandbox) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != SandboxStateCreated && s.state != SandboxStateStopped {
		return fmt.Errorf("cannot start sandbox in state: %s", s.state)
	}

	s.state = SandboxStateRunning
	s.startTime = time.Now()
	return nil
}

// Stop stops the sandbox
func (s *Sandbox) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = SandboxStateStopped
	return nil
}

// Pause pauses the sandbox
func (s *Sandbox) Pause() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != SandboxStateRunning {
		return errors.New("can only pause running sandbox")
	}

	s.state = SandboxStatePaused
	return nil
}

// Resume resumes a paused sandbox
func (s *Sandbox) Resume() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != SandboxStatePaused {
		return errors.New("can only resume paused sandbox")
	}

	s.state = SandboxStateRunning
	return nil
}

// GetState returns the current sandbox state
func (s *Sandbox) GetState() SandboxState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// GetUsage returns current resource usage
func (s *Sandbox) GetUsage() *ResourceUsage {
	s.usage.mu.RLock()
	defer s.usage.mu.RUnlock()

	// Calculate execution time
	s.mu.RLock()
	if s.state == SandboxStateRunning {
		s.usage.ExecutionTime = time.Since(s.startTime)
	}
	s.mu.RUnlock()

	return &ResourceUsage{
		MemoryMB:         s.usage.MemoryMB,
		CPUPercent:       s.usage.CPUPercent,
		ExecutionTime:    s.usage.ExecutionTime,
		ConcurrentTasks:  s.usage.ConcurrentTasks,
		APICallsThisMin:  s.usage.APICallsThisMin,
		TokensThisMin:    s.usage.TokensThisMin,
		BytesRead:        s.usage.BytesRead,
		BytesWritten:     s.usage.BytesWritten,
		NetworkBytesSent: s.usage.NetworkBytesSent,
		NetworkBytesRecv: s.usage.NetworkBytesRecv,
	}
}

// CheckPermission checks if an action is permitted
func (s *Sandbox) CheckPermission(action PermissionAction, resource string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch action {
	case PermissionActionUseTool:
		return s.checkToolPermission(resource)
	case PermissionActionReadFile:
		if !s.permissions.AllowFileRead {
			return s.recordViolation(ViolationTypePermissionDenied, "file read not allowed", ViolationSeverityMedium)
		}
		return s.checkPathPermission(resource)
	case PermissionActionWriteFile:
		if !s.permissions.AllowFileWrite {
			return s.recordViolation(ViolationTypePermissionDenied, "file write not allowed", ViolationSeverityHigh)
		}
		return s.checkPathPermission(resource)
	case PermissionActionNetworkAccess:
		if !s.permissions.AllowNetworkAccess {
			return s.recordViolation(ViolationTypePermissionDenied, "network access not allowed", ViolationSeverityHigh)
		}
		return s.checkHostPermission(resource)
	case PermissionActionShellExec:
		if !s.permissions.AllowShellExecution {
			return s.recordViolation(ViolationTypePermissionDenied, "shell execution not allowed", ViolationSeverityCritical)
		}
		return nil
	case PermissionActionCodeExec:
		if !s.permissions.AllowCodeExecution {
			return s.recordViolation(ViolationTypePermissionDenied, "code execution not allowed", ViolationSeverityCritical)
		}
		return nil
	default:
		return errors.New("unknown permission action")
	}
}

// PermissionAction represents an action that requires permission
type PermissionAction string

const (
	PermissionActionUseTool       PermissionAction = "use_tool"
	PermissionActionReadFile      PermissionAction = "read_file"
	PermissionActionWriteFile     PermissionAction = "write_file"
	PermissionActionNetworkAccess PermissionAction = "network_access"
	PermissionActionShellExec     PermissionAction = "shell_exec"
	PermissionActionCodeExec      PermissionAction = "code_exec"
)

// checkToolPermission checks if a tool is allowed
func (s *Sandbox) checkToolPermission(tool string) error {
	// Check denied list first
	for _, denied := range s.permissions.DeniedTools {
		if matchPattern(tool, denied) {
			return s.recordViolation(ViolationTypeUnauthorizedTool, fmt.Sprintf("tool %s is denied", tool), ViolationSeverityHigh)
		}
	}

	// If allowed list is specified, check it
	if len(s.permissions.AllowedTools) > 0 {
		for _, allowed := range s.permissions.AllowedTools {
			if matchPattern(tool, allowed) {
				return nil
			}
		}
		return s.recordViolation(ViolationTypeUnauthorizedTool, fmt.Sprintf("tool %s is not in allowed list", tool), ViolationSeverityHigh)
	}

	return nil
}

// checkPathPermission checks if a path is allowed
func (s *Sandbox) checkPathPermission(path string) error {
	// Check denied list first
	for _, denied := range s.permissions.DeniedPaths {
		if matchPathPattern(path, denied) {
			return s.recordViolation(ViolationTypeUnauthorizedPath, fmt.Sprintf("path %s is denied", path), ViolationSeverityHigh)
		}
	}

	// If allowed list is specified, check it
	if len(s.permissions.AllowedPaths) > 0 {
		for _, allowed := range s.permissions.AllowedPaths {
			if matchPathPattern(path, allowed) {
				return nil
			}
		}
		return s.recordViolation(ViolationTypeUnauthorizedPath, fmt.Sprintf("path %s is not in allowed list", path), ViolationSeverityHigh)
	}

	return nil
}

// checkHostPermission checks if a host is allowed
func (s *Sandbox) checkHostPermission(host string) error {
	// Check denied list first
	for _, denied := range s.permissions.DeniedHosts {
		if matchPattern(host, denied) {
			return s.recordViolation(ViolationTypeUnauthorizedHost, fmt.Sprintf("host %s is denied", host), ViolationSeverityHigh)
		}
	}

	// If allowed list is specified, check it
	if len(s.permissions.AllowedHosts) > 0 {
		for _, allowed := range s.permissions.AllowedHosts {
			if matchPattern(host, allowed) {
				return nil
			}
		}
		return s.recordViolation(ViolationTypeUnauthorizedHost, fmt.Sprintf("host %s is not in allowed list", host), ViolationSeverityHigh)
	}

	return nil
}

// CheckResourceLimit checks if a resource limit would be exceeded
func (s *Sandbox) CheckResourceLimit(resource ResourceType, amount int64) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch resource {
	case ResourceTypeMemory:
		if s.usage.MemoryMB+amount > s.limits.MaxMemoryMB {
			return s.recordViolation(ViolationTypeMemoryExceeded, "memory limit exceeded", ViolationSeverityHigh)
		}
	case ResourceTypeAPICall:
		if !s.rateLimiter.AllowAPICall() {
			return s.recordViolation(ViolationTypeRateLimitExceeded, "API rate limit exceeded", ViolationSeverityMedium)
		}
	case ResourceTypeTokens:
		if !s.rateLimiter.AllowTokens(int(amount)) {
			return s.recordViolation(ViolationTypeRateLimitExceeded, "token rate limit exceeded", ViolationSeverityMedium)
		}
	case ResourceTypeExecutionTime:
		if time.Since(s.startTime) > s.limits.MaxExecutionTime {
			return s.recordViolation(ViolationTypeTimeExceeded, "execution time limit exceeded", ViolationSeverityCritical)
		}
	}

	return nil
}

// ResourceType represents a type of resource
type ResourceType string

const (
	ResourceTypeMemory        ResourceType = "memory"
	ResourceTypeCPU           ResourceType = "cpu"
	ResourceTypeAPICall       ResourceType = "api_call"
	ResourceTypeTokens        ResourceType = "tokens"
	ResourceTypeExecutionTime ResourceType = "execution_time"
	ResourceTypeConcurrent    ResourceType = "concurrent"
)

// RecordUsage records resource usage
func (s *Sandbox) RecordUsage(resource ResourceType, amount int64) {
	s.usage.mu.Lock()
	defer s.usage.mu.Unlock()

	switch resource {
	case ResourceTypeMemory:
		s.usage.MemoryMB = amount
	case ResourceTypeAPICall:
		atomic.AddInt32(&s.usage.APICallsThisMin, int32(amount))
	case ResourceTypeTokens:
		atomic.AddInt32(&s.usage.TokensThisMin, int32(amount))
	}
}

// IncrementConcurrentTasks increments the concurrent task count
func (s *Sandbox) IncrementConcurrentTasks() error {
	current := atomic.AddInt32(&s.usage.ConcurrentTasks, 1)
	if int(current) > s.limits.MaxConcurrentTasks {
		atomic.AddInt32(&s.usage.ConcurrentTasks, -1)
		return errors.New("max concurrent tasks exceeded")
	}
	return nil
}

// DecrementConcurrentTasks decrements the concurrent task count
func (s *Sandbox) DecrementConcurrentTasks() {
	atomic.AddInt32(&s.usage.ConcurrentTasks, -1)
}

// recordViolation records a violation and returns an error
func (s *Sandbox) recordViolation(violationType ViolationType, description string, severity ViolationSeverity) error {
	violation := &Violation{
		ID:          uuid.New().String(),
		Type:        violationType,
		Description: description,
		Severity:    severity,
		Action:      s.determineAction(severity),
		OccurredAt:  time.Now(),
	}

	s.violations = append(s.violations, violation)

	// Take action based on severity
	if violation.Action == ViolationActionTerminate {
		s.state = SandboxStateViolation
	}

	return errors.New(description)
}

// determineAction determines the action to take for a violation
func (s *Sandbox) determineAction(severity ViolationSeverity) ViolationAction {
	switch severity {
	case ViolationSeverityLow:
		return ViolationActionWarn
	case ViolationSeverityMedium:
		return ViolationActionThrottle
	case ViolationSeverityHigh:
		return ViolationActionBlock
	case ViolationSeverityCritical:
		return ViolationActionTerminate
	default:
		return ViolationActionWarn
	}
}

// GetViolations returns all recorded violations
func (s *Sandbox) GetViolations() []*Violation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Violation, len(s.violations))
	copy(result, s.violations)
	return result
}

// Pattern matching helpers

func matchPattern(value, pattern string) bool {
	if pattern == "*" {
		return true
	}
	// Simple wildcard matching
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(value) >= len(prefix) && value[:len(prefix)] == prefix
	}
	return value == pattern
}

func matchPathPattern(path, pattern string) bool {
	if pattern == "*" || pattern == "/**" {
		return true
	}
	// Check if path starts with pattern
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(path) >= len(prefix) && path[:len(prefix)] == prefix
	}
	return path == pattern
}

// RateLimiter implements rate limiting
type RateLimiter struct {
	apiCallsPerMin int
	tokensPerMin   int
	apiCalls       int32
	tokens         int32
	lastReset      time.Time
	mu             sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(apiCallsPerMin, tokensPerMin int) *RateLimiter {
	return &RateLimiter{
		apiCallsPerMin: apiCallsPerMin,
		tokensPerMin:   tokensPerMin,
		lastReset:      time.Now(),
	}
}

// AllowAPICall checks if an API call is allowed
func (r *RateLimiter) AllowAPICall() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.checkReset()

	if int(r.apiCalls) >= r.apiCallsPerMin {
		return false
	}

	r.apiCalls++
	return true
}

// AllowTokens checks if token usage is allowed
func (r *RateLimiter) AllowTokens(count int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.checkReset()

	if int(r.tokens)+count > r.tokensPerMin {
		return false
	}

	r.tokens += int32(count)
	return true
}

// checkReset resets counters if a minute has passed
func (r *RateLimiter) checkReset() {
	if time.Since(r.lastReset) >= time.Minute {
		r.apiCalls = 0
		r.tokens = 0
		r.lastReset = time.Now()
	}
}

// GetStats returns rate limiter stats
func (r *RateLimiter) GetStats() (apiCalls, tokens int32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checkReset()
	return r.apiCalls, r.tokens
}

// SandboxManager manages multiple sandboxes
type SandboxManager struct {
	sandboxes map[string]*Sandbox
	mu        sync.RWMutex
}

// NewSandboxManager creates a new sandbox manager
func NewSandboxManager() *SandboxManager {
	return &SandboxManager{
		sandboxes: make(map[string]*Sandbox),
	}
}

// CreateSandbox creates a new sandbox
func (m *SandboxManager) CreateSandbox(config *SandboxConfig) (*Sandbox, error) {
	sandbox := NewSandbox(config)

	m.mu.Lock()
	m.sandboxes[sandbox.ID] = sandbox
	m.mu.Unlock()

	return sandbox, nil
}

// GetSandbox retrieves a sandbox by ID
func (m *SandboxManager) GetSandbox(id string) (*Sandbox, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sandbox, ok := m.sandboxes[id]
	if !ok {
		return nil, fmt.Errorf("sandbox %s not found", id)
	}
	return sandbox, nil
}

// ListSandboxes lists all sandboxes
func (m *SandboxManager) ListSandboxes() []*Sandbox {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Sandbox, 0, len(m.sandboxes))
	for _, s := range m.sandboxes {
		result = append(result, s)
	}
	return result
}

// DeleteSandbox deletes a sandbox
func (m *SandboxManager) DeleteSandbox(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sandbox, ok := m.sandboxes[id]
	if !ok {
		return fmt.Errorf("sandbox %s not found", id)
	}

	sandbox.Stop()
	delete(m.sandboxes, id)
	return nil
}

// ExecuteInSandbox executes a function within a sandbox context
func (m *SandboxManager) ExecuteInSandbox(ctx context.Context, sandboxID string, fn func(context.Context, *Sandbox) error) error {
	sandbox, err := m.GetSandbox(sandboxID)
	if err != nil {
		return err
	}

	if sandbox.GetState() != SandboxStateRunning {
		return errors.New("sandbox is not running")
	}

	// Increment concurrent tasks
	if err := sandbox.IncrementConcurrentTasks(); err != nil {
		return err
	}
	defer sandbox.DecrementConcurrentTasks()

	// Check execution time limit
	if err := sandbox.CheckResourceLimit(ResourceTypeExecutionTime, 0); err != nil {
		return err
	}

	// Create cancellable context with timeout
	execCtx, cancel := context.WithTimeout(ctx, sandbox.limits.MaxExecutionTime)
	defer cancel()

	// Execute the function
	return fn(execCtx, sandbox)
}
