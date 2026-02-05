package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// ContainerConfig configures the container sandbox
type ContainerConfig struct {
	Image           string            // Docker image to use
	NetworkMode     string            // Network mode (none, bridge, host)
	Volumes         map[string]string // Host path -> Container path mappings
	Environment     map[string]string // Environment variables
	WorkingDir      string            // Working directory in container
	User            string            // User to run as
	Privileged      bool              // Run in privileged mode
	CapAdd          []string          // Linux capabilities to add
	CapDrop         []string          // Linux capabilities to drop
	SecurityOpt     []string          // Security options
	ReadonlyRootfs  bool              // Mount root filesystem as read-only
	AutoRemove      bool              // Remove container when it exits
	Labels          map[string]string // Container labels
}

// DockerClient interface abstracts Docker operations for testing
type DockerClient interface {
	// Container lifecycle
	CreateContainer(ctx context.Context, config *ContainerCreateConfig) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string, timeout time.Duration) error
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	WaitContainer(ctx context.Context, containerID string) (int, error)

	// Container execution
	ExecCreate(ctx context.Context, containerID string, cmd []string, env []string) (string, error)
	ExecStart(ctx context.Context, execID string) (io.Reader, error)
	ExecInspect(ctx context.Context, execID string) (*ExecInspectResult, error)

	// Container info
	InspectContainer(ctx context.Context, containerID string) (*ContainerInfo, error)
	ContainerLogs(ctx context.Context, containerID string, tail int) (string, error)
	ContainerStats(ctx context.Context, containerID string) (*ContainerStats, error)

	// Image operations
	PullImage(ctx context.Context, image string) error
	ImageExists(ctx context.Context, image string) (bool, error)

	// Network operations
	CreateNetwork(ctx context.Context, name string) (string, error)
	RemoveNetwork(ctx context.Context, networkID string) error
}

// ContainerCreateConfig holds container creation configuration
type ContainerCreateConfig struct {
	Image        string
	Cmd          []string
	Env          []string
	WorkingDir   string
	User         string
	NetworkMode  string
	Binds        []string // Volume bindings
	CapAdd       []string
	CapDrop      []string
	SecurityOpt  []string
	Memory       int64 // Memory limit in bytes
	MemorySwap   int64 // Memory + swap limit
	CPUShares    int64 // CPU shares
	CPUPeriod    int64 // CPU CFS period
	CPUQuota     int64 // CPU CFS quota
	PidsLimit    int64 // Process limit
	Privileged   bool
	ReadonlyRoot bool
	AutoRemove   bool
	Labels       map[string]string
}

// ExecInspectResult holds exec inspection results
type ExecInspectResult struct {
	ExitCode int
	Running  bool
	Pid      int
}

// ContainerInfo holds container information
type ContainerInfo struct {
	ID      string
	Name    string
	Image   string
	State   string // created, running, paused, restarting, removing, exited, dead
	Created time.Time
	Started time.Time
	Pid     int
}

// ContainerStats holds container resource statistics
type ContainerStats struct {
	CPUPercent    float64
	MemoryUsage   int64
	MemoryLimit   int64
	MemoryPercent float64
	NetworkRx     int64
	NetworkTx     int64
	BlockRead     int64
	BlockWrite    int64
	PidCount      int
}

// ContainerSandbox provides Docker container-based isolation
type ContainerSandbox struct {
	id          string
	containerID string
	client      DockerClient
	config      *ContainerConfig
	limits      *ResourceLimits
	permissions *Permissions
	state       ContainerSandboxState
	metrics     *ContainerMetrics
	auditLog    *AuditLog
	mu          sync.RWMutex
}

// NewContainerSandbox creates a new container-based sandbox
func NewContainerSandbox(client DockerClient, config *ContainerConfig) *ContainerSandbox {
	if config == nil {
		config = &ContainerConfig{
			Image:          "alpine:latest",
			NetworkMode:    "none",
			ReadonlyRootfs: true,
			AutoRemove:     true,
		}
	}

	return &ContainerSandbox{
		id:     generateID(),
		client: client,
		config: config,
		state:  ContainerStateCreated,
		metrics: &ContainerMetrics{
			StartTime: time.Now(),
		},
	}
}

// Initialize creates and starts the container
func (s *ContainerSandbox) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Pull image if needed
	exists, err := s.client.ImageExists(ctx, s.config.Image)
	if err != nil {
		return fmt.Errorf("failed to check image: %w", err)
	}

	if !exists {
		if err := s.client.PullImage(ctx, s.config.Image); err != nil {
			return fmt.Errorf("failed to pull image: %w", err)
		}
	}

	// Build container config
	createConfig := s.buildCreateConfig()

	// Create container
	containerID, err := s.client.CreateContainer(ctx, createConfig)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	s.containerID = containerID

	// Start container
	if err := s.client.StartContainer(ctx, containerID); err != nil {
		s.client.RemoveContainer(ctx, containerID, true)
		return fmt.Errorf("failed to start container: %w", err)
	}

	s.state = ContainerStateRunning
	return nil
}

// Execute runs a task in the container
func (s *ContainerSandbox) Execute(ctx context.Context, task *ContainerTask) (*ContainerResult, error) {
	s.mu.RLock()
	if s.state != ContainerStateRunning {
		s.mu.RUnlock()
		return nil, errors.New("sandbox not running")
	}
	containerID := s.containerID
	s.mu.RUnlock()

	startTime := time.Now()

	// Build command
	cmd := task.Command
	if len(cmd) == 0 {
		return nil, errors.New("no command specified")
	}
	agentID := task.AgentID

	// Build environment
	var env []string
	for k, v := range task.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Apply timeout
	execCtx := ctx
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	// Create exec
	execID, err := s.client.ExecCreate(execCtx, containerID, cmd, env)
	if err != nil {
		return &ContainerResult{
			Success:   false,
			Error:     fmt.Errorf("failed to create exec: %w", err),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, nil
	}

	// Start exec and capture output
	reader, err := s.client.ExecStart(execCtx, execID)
	if err != nil {
		return &ContainerResult{
			Success:   false,
			Error:     fmt.Errorf("failed to start exec: %w", err),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, nil
	}

	// Read output
	output, err := io.ReadAll(reader)
	if err != nil && err != context.DeadlineExceeded {
		return &ContainerResult{
			Success:   false,
			Error:     fmt.Errorf("failed to read output: %w", err),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, nil
	}

	// Get exit code
	inspect, err := s.client.ExecInspect(execCtx, execID)
	if err != nil {
		return &ContainerResult{
			Success:   false,
			Output:    string(output),
			Error:     fmt.Errorf("failed to inspect exec: %w", err),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, nil
	}

	endTime := time.Now()

	// Update metrics
	s.mu.Lock()
	s.metrics.TasksExecuted++
	s.metrics.TotalExecutionTime += endTime.Sub(startTime)
	s.mu.Unlock()

	// Audit log
	if s.auditLog != nil {
		result := AuditResultAllowed
		if inspect.ExitCode != 0 {
			result = AuditResultDenied
		}
		s.auditLog.Record(s.id, "execute", "container:"+containerID, result,
			fmt.Sprintf("cmd=%v, exit_code=%d", cmd, inspect.ExitCode),
			map[string]interface{}{"agent_id": agentID, "output": truncateString(string(output), 1000)})
	}

	return &ContainerResult{
		Success:   inspect.ExitCode == 0,
		Output:    string(output),
		ExitCode:  inspect.ExitCode,
		StartTime: startTime,
		EndTime:   endTime,
	}, nil
}

// SetLimits sets resource limits for the container
func (s *ContainerSandbox) SetLimits(limits *ResourceLimits) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.limits = limits
	return nil
}

// SetPermissions sets permissions for the container
func (s *ContainerSandbox) SetPermissions(perms *Permissions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.permissions = perms
	return nil
}

// GetMetrics returns sandbox metrics
func (s *ContainerSandbox) GetMetrics() *ContainerMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get container stats
	if s.containerID != "" && s.state == ContainerStateRunning {
		stats, err := s.client.ContainerStats(context.Background(), s.containerID)
		if err == nil {
			s.metrics.CPUUsage = stats.CPUPercent
			s.metrics.MemoryUsage = stats.MemoryUsage
			s.metrics.NetworkBytesIn = stats.NetworkRx
			s.metrics.NetworkBytesOut = stats.NetworkTx
		}
	}

	return s.metrics
}

// IsRunning returns true if the sandbox is running
func (s *ContainerSandbox) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == ContainerStateRunning
}

// GetState returns the sandbox state
func (s *ContainerSandbox) GetState() ContainerSandboxState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Pause pauses the container
func (s *ContainerSandbox) Pause(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != ContainerStateRunning {
		return errors.New("sandbox not running")
	}

	s.state = ContainerStatePaused
	return nil
}

// Resume resumes the container
func (s *ContainerSandbox) Resume(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != ContainerStatePaused {
		return errors.New("sandbox not paused")
	}

	s.state = ContainerStateRunning
	return nil
}

// Terminate stops and removes the container
func (s *ContainerSandbox) Terminate() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.containerID == "" {
		return nil
	}

	ctx := context.Background()

	// Stop container
	if err := s.client.StopContainer(ctx, s.containerID, 10*time.Second); err != nil {
		// Force remove
		s.client.RemoveContainer(ctx, s.containerID, true)
	} else if !s.config.AutoRemove {
		s.client.RemoveContainer(ctx, s.containerID, false)
	}

	s.state = ContainerStateTerminated
	s.containerID = ""

	return nil
}

// SetAuditLog sets the audit log
func (s *ContainerSandbox) SetAuditLog(logger *AuditLog) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditLog = logger
}

// GetLogs returns container logs
func (s *ContainerSandbox) GetLogs(ctx context.Context, tail int) (string, error) {
	s.mu.RLock()
	containerID := s.containerID
	s.mu.RUnlock()

	if containerID == "" {
		return "", errors.New("container not running")
	}

	return s.client.ContainerLogs(ctx, containerID, tail)
}

func (s *ContainerSandbox) buildCreateConfig() *ContainerCreateConfig {
	config := &ContainerCreateConfig{
		Image:        s.config.Image,
		Cmd:          []string{"sleep", "infinity"}, // Keep container running
		WorkingDir:   s.config.WorkingDir,
		User:         s.config.User,
		NetworkMode:  s.config.NetworkMode,
		CapAdd:       s.config.CapAdd,
		CapDrop:      s.config.CapDrop,
		SecurityOpt:  s.config.SecurityOpt,
		Privileged:   s.config.Privileged,
		ReadonlyRoot: s.config.ReadonlyRootfs,
		AutoRemove:   s.config.AutoRemove,
		Labels:       s.config.Labels,
	}

	// Add environment variables
	for k, v := range s.config.Environment {
		config.Env = append(config.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Add volume bindings
	for hostPath, containerPath := range s.config.Volumes {
		config.Binds = append(config.Binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// Apply resource limits
	if s.limits != nil {
		config.Memory = s.limits.MaxMemoryMB * 1024 * 1024 // Convert MB to bytes
		config.MemorySwap = config.Memory * 2

		if s.limits.MaxCPUPercent > 0 {
			config.CPUPeriod = 100000
			config.CPUQuota = int64(s.limits.MaxCPUPercent / 100 * 100000)
		}
	}

	// Default security options if not specified
	if len(config.SecurityOpt) == 0 && !config.Privileged {
		config.SecurityOpt = []string{
			"no-new-privileges:true",
		}
	}

	// Default capabilities to drop
	if len(config.CapDrop) == 0 {
		config.CapDrop = []string{
			"ALL",
		}
	}

	return config
}

// ContainerSandboxPool manages a pool of container sandboxes
type ContainerSandboxPool struct {
	client     DockerClient
	config     *ContainerConfig
	pool       []*ContainerSandbox
	maxSize    int
	mu         sync.Mutex
}

// NewContainerSandboxPool creates a new container sandbox pool
func NewContainerSandboxPool(client DockerClient, config *ContainerConfig, maxSize int) *ContainerSandboxPool {
	return &ContainerSandboxPool{
		client:  client,
		config:  config,
		pool:    make([]*ContainerSandbox, 0),
		maxSize: maxSize,
	}
}

// Acquire gets a sandbox from the pool or creates a new one
func (p *ContainerSandboxPool) Acquire(ctx context.Context) (*ContainerSandbox, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Try to get from pool
	for i, sandbox := range p.pool {
		if sandbox.IsRunning() {
			// Remove from pool
			p.pool = append(p.pool[:i], p.pool[i+1:]...)
			return sandbox, nil
		}
	}

	// Create new sandbox
	sandbox := NewContainerSandbox(p.client, p.config)
	if err := sandbox.Initialize(ctx); err != nil {
		return nil, err
	}

	return sandbox, nil
}

// Release returns a sandbox to the pool
func (p *ContainerSandboxPool) Release(sandbox *ContainerSandbox) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.pool) >= p.maxSize {
		// Pool full, terminate sandbox
		sandbox.Terminate()
		return
	}

	p.pool = append(p.pool, sandbox)
}

// Cleanup terminates all sandboxes in the pool
func (p *ContainerSandboxPool) Cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, sandbox := range p.pool {
		sandbox.Terminate()
	}

	p.pool = make([]*ContainerSandbox, 0)
}

// DockerSandboxFactory creates container sandboxes
type DockerSandboxFactory struct {
	client DockerClient
	config *ContainerConfig
}

// NewDockerSandboxFactory creates a new Docker sandbox factory
func NewDockerSandboxFactory(client DockerClient, config *ContainerConfig) *DockerSandboxFactory {
	return &DockerSandboxFactory{
		client: client,
		config: config,
	}
}

// Create creates a new container sandbox
func (f *DockerSandboxFactory) Create(ctx context.Context) (*ContainerSandbox, error) {
	sandbox := NewContainerSandbox(f.client, f.config)
	if err := sandbox.Initialize(ctx); err != nil {
		return nil, err
	}
	return sandbox, nil
}

// ContainerNetworkIsolation provides network isolation for containers
type ContainerNetworkIsolation struct {
	client    DockerClient
	networkID string
	name      string
}

// NewContainerNetworkIsolation creates an isolated network
func NewContainerNetworkIsolation(client DockerClient, name string) (*ContainerNetworkIsolation, error) {
	ctx := context.Background()
	networkID, err := client.CreateNetwork(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}

	return &ContainerNetworkIsolation{
		client:    client,
		networkID: networkID,
		name:      name,
	}, nil
}

// GetNetworkID returns the network ID
func (n *ContainerNetworkIsolation) GetNetworkID() string {
	return n.networkID
}

// Cleanup removes the network
func (n *ContainerNetworkIsolation) Cleanup() error {
	return n.client.RemoveNetwork(context.Background(), n.networkID)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func generateID() string {
	return fmt.Sprintf("sandbox-%d", time.Now().UnixNano())
}

// ContainerTask represents a task to execute in the container
type ContainerTask struct {
	AgentID     string
	Command     []string
	Environment map[string]string
	WorkingDir  string
	Timeout     time.Duration
	Input       string
}

// ContainerResult represents the result of a container execution
type ContainerResult struct {
	Success   bool
	Output    string
	Error     error
	ExitCode  int
	StartTime time.Time
	EndTime   time.Time
}

// MarshalJSON implements json.Marshaler for ContainerResult
func (r *ContainerResult) MarshalJSON() ([]byte, error) {
	type Alias ContainerResult
	errStr := ""
	if r.Error != nil {
		errStr = r.Error.Error()
	}
	return json.Marshal(&struct {
		*Alias
		Error string `json:"error,omitempty"`
	}{
		Alias: (*Alias)(r),
		Error: errStr,
	})
}

// ContainerSandboxState represents the state of a container sandbox
type ContainerSandboxState string

const (
	ContainerStateCreated    ContainerSandboxState = "created"
	ContainerStateRunning    ContainerSandboxState = "running"
	ContainerStatePaused     ContainerSandboxState = "paused"
	ContainerStateTerminated ContainerSandboxState = "terminated"
)

// ContainerMetrics holds container sandbox metrics
type ContainerMetrics struct {
	StartTime          time.Time
	TasksExecuted      int
	TotalExecutionTime time.Duration
	CPUUsage           float64
	MemoryUsage        int64
	NetworkBytesIn     int64
	NetworkBytesOut    int64
}
