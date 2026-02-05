package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// WASMConfig configures the WASM sandbox
type WASMConfig struct {
	MaxMemoryPages   uint32            // Maximum memory pages (64KB each)
	MaxTableSize     uint32            // Maximum table size
	MaxGlobals       uint32            // Maximum globals
	MaxFunctions     uint32            // Maximum functions
	AllowWASI        bool              // Allow WASI system calls
	AllowedWASICalls []string          // Whitelist of WASI calls
	Environment      map[string]string // Environment variables
	Args             []string          // Command line arguments
	PreopenDirs      []string          // Pre-opened directories for WASI
	Stdin            io.Reader         // Standard input
	Stdout           io.Writer         // Standard output
	Stderr           io.Writer         // Standard error
}

// WASMRuntime interface abstracts the WASM runtime for testing
type WASMRuntime interface {
	// Module operations
	CompileModule(ctx context.Context, wasmBytes []byte) (WASMModule, error)
	InstantiateModule(ctx context.Context, module WASMModule, config *WASMInstanceConfig) (WASMInstance, error)

	// Runtime info
	Name() string
	Version() string

	// Close releases runtime resources
	Close(ctx context.Context) error
}

// WASMModule represents a compiled WASM module
type WASMModule interface {
	// Name returns the module name
	Name() string

	// Exports returns exported functions
	Exports() []WASMExport

	// Close releases module resources
	Close(ctx context.Context) error
}

// WASMExport represents an exported function
type WASMExport struct {
	Name       string
	Kind       WASMExportKind
	ParamTypes []WASMValueType
	ReturnType []WASMValueType
}

// WASMExportKind represents the kind of export
type WASMExportKind string

const (
	WASMExportFunction WASMExportKind = "function"
	WASMExportMemory   WASMExportKind = "memory"
	WASMExportTable    WASMExportKind = "table"
	WASMExportGlobal   WASMExportKind = "global"
)

// WASMValueType represents WASM value types
type WASMValueType string

const (
	WASMValueI32 WASMValueType = "i32"
	WASMValueI64 WASMValueType = "i64"
	WASMValueF32 WASMValueType = "f32"
	WASMValueF64 WASMValueType = "f64"
)

// WASMInstance represents an instantiated WASM module
type WASMInstance interface {
	// Call invokes an exported function
	Call(ctx context.Context, funcName string, args ...interface{}) ([]interface{}, error)

	// GetMemory returns the instance's memory
	GetMemory() WASMMemory

	// GetGlobal gets a global variable value
	GetGlobal(name string) (interface{}, error)

	// SetGlobal sets a global variable value
	SetGlobal(name string, value interface{}) error

	// Close releases instance resources
	Close(ctx context.Context) error
}

// WASMMemory represents WASM linear memory
type WASMMemory interface {
	// Read reads bytes from memory
	Read(offset, length uint32) ([]byte, error)

	// Write writes bytes to memory
	Write(offset uint32, data []byte) error

	// Size returns memory size in bytes
	Size() uint32

	// Grow grows memory by specified number of pages
	Grow(pages uint32) error
}

// WASMInstanceConfig configures a WASM instance
type WASMInstanceConfig struct {
	MaxMemoryPages uint32
	MaxTableSize   uint32
	Environment    map[string]string
	Args           []string
	PreopenDirs    []string
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
	ImportFuncs    map[string]WASMHostFunc
}

// WASMHostFunc represents a host function callable from WASM
type WASMHostFunc func(ctx context.Context, args ...interface{}) ([]interface{}, error)

// WASMSandbox provides WASM-based isolation
type WASMSandbox struct {
	id          string
	runtime     WASMRuntime
	config      *WASMConfig
	module      WASMModule
	instance    WASMInstance
	limits      *ResourceLimits
	permissions *Permissions
	state       WASMSandboxState
	metrics     *WASMMetrics
	auditLog    *AuditLog
	mu          sync.RWMutex
}

// WASMSandboxState represents the state of a WASM sandbox
type WASMSandboxState string

const (
	WASMStateCreated    WASMSandboxState = "created"
	WASMStateRunning    WASMSandboxState = "running"
	WASMStatePaused     WASMSandboxState = "paused"
	WASMStateTerminated WASMSandboxState = "terminated"
)

// WASMMetrics holds WASM sandbox metrics
type WASMMetrics struct {
	StartTime          time.Time
	TasksExecuted      int
	TotalExecutionTime time.Duration
	MemoryUsage        int64
}

// NewWASMSandbox creates a new WASM-based sandbox
func NewWASMSandbox(runtime WASMRuntime, config *WASMConfig) *WASMSandbox {
	if config == nil {
		config = &WASMConfig{
			MaxMemoryPages: 256, // 16MB
			MaxTableSize:   1024,
			AllowWASI:      true,
		}
	}

	return &WASMSandbox{
		id:      generateID(),
		runtime: runtime,
		config:  config,
		state:   WASMStateCreated,
		metrics: &WASMMetrics{
			StartTime: time.Now(),
		},
	}
}

// LoadModule compiles and instantiates a WASM module
func (s *WASMSandbox) LoadModule(ctx context.Context, wasmBytes []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Compile module
	module, err := s.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("failed to compile module: %w", err)
	}

	s.module = module

	// Create instance config
	instanceConfig := &WASMInstanceConfig{
		MaxMemoryPages: s.config.MaxMemoryPages,
		MaxTableSize:   s.config.MaxTableSize,
		Environment:    s.config.Environment,
		Args:           s.config.Args,
		PreopenDirs:    s.config.PreopenDirs,
		Stdin:          s.config.Stdin,
		Stdout:         s.config.Stdout,
		Stderr:         s.config.Stderr,
	}

	// Add host functions if needed
	if s.config.AllowWASI {
		instanceConfig.ImportFuncs = s.createWASIFuncs()
	}

	// Instantiate module
	instance, err := s.runtime.InstantiateModule(ctx, module, instanceConfig)
	if err != nil {
		module.Close(ctx)
		return fmt.Errorf("failed to instantiate module: %w", err)
	}

	s.instance = instance
	s.state = WASMStateRunning

	return nil
}

// Execute runs a task in the WASM sandbox
func (s *WASMSandbox) Execute(ctx context.Context, task *WASMTask) (*WASMResult, error) {
	s.mu.RLock()
	if s.state != WASMStateRunning {
		s.mu.RUnlock()
		return nil, errors.New("sandbox not running")
	}
	instance := s.instance
	s.mu.RUnlock()

	if instance == nil {
		return nil, errors.New("no module loaded")
	}

	startTime := time.Now()

	// Get function name from command
	if len(task.Command) == 0 {
		return nil, errors.New("no function specified")
	}

	funcName := task.Command[0]
	var args []interface{}
	for _, arg := range task.Command[1:] {
		args = append(args, arg)
	}

	// Apply timeout
	execCtx := ctx
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	// Call function
	results, err := instance.Call(execCtx, funcName, args...)
	endTime := time.Now()

	// Update metrics
	s.mu.Lock()
	s.metrics.TasksExecuted++
	s.metrics.TotalExecutionTime += endTime.Sub(startTime)
	s.mu.Unlock()

	// Audit log
	if s.auditLog != nil {
		auditResult := AuditResultAllowed
		if err != nil {
			auditResult = AuditResultDenied
		}
		s.auditLog.Record(s.id, "wasm_call", funcName, auditResult,
			fmt.Sprintf("args=%v, results=%v", args, results),
			map[string]interface{}{"agent_id": task.AgentID})
	}

	if err != nil {
		return &WASMResult{
			Success:   false,
			Error:     err,
			StartTime: startTime,
			EndTime:   endTime,
		}, nil
	}

	return &WASMResult{
		Success:   true,
		Output:    fmt.Sprintf("%v", results),
		StartTime: startTime,
		EndTime:   endTime,
	}, nil
}

// WASMTask represents a task to execute in the WASM sandbox
type WASMTask struct {
	AgentID     string
	Command     []string
	Environment map[string]string
	Timeout     time.Duration
}

// WASMResult represents the result of a WASM execution
type WASMResult struct {
	Success   bool
	Output    string
	Error     error
	StartTime time.Time
	EndTime   time.Time
}

// CallFunction calls a specific WASM function
func (s *WASMSandbox) CallFunction(ctx context.Context, funcName string, args ...interface{}) ([]interface{}, error) {
	s.mu.RLock()
	if s.state != WASMStateRunning {
		s.mu.RUnlock()
		return nil, errors.New("sandbox not running")
	}
	instance := s.instance
	s.mu.RUnlock()

	if instance == nil {
		return nil, errors.New("no module loaded")
	}

	return instance.Call(ctx, funcName, args...)
}

// ReadMemory reads from WASM memory
func (s *WASMSandbox) ReadMemory(offset, length uint32) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.instance == nil {
		return nil, errors.New("no module loaded")
	}

	memory := s.instance.GetMemory()
	if memory == nil {
		return nil, errors.New("module has no memory")
	}

	return memory.Read(offset, length)
}

// WriteMemory writes to WASM memory
func (s *WASMSandbox) WriteMemory(offset uint32, data []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.instance == nil {
		return errors.New("no module loaded")
	}

	memory := s.instance.GetMemory()
	if memory == nil {
		return errors.New("module has no memory")
	}

	return memory.Write(offset, data)
}

// GetExports returns the module's exports
func (s *WASMSandbox) GetExports() []WASMExport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.module == nil {
		return nil
	}

	return s.module.Exports()
}

// SetLimits sets resource limits
func (s *WASMSandbox) SetLimits(limits *ResourceLimits) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.limits = limits
	return nil
}

// SetPermissions sets permissions
func (s *WASMSandbox) SetPermissions(perms *Permissions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.permissions = perms
	return nil
}

// GetMetrics returns sandbox metrics
func (s *WASMSandbox) GetMetrics() *WASMMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Update memory usage if instance exists
	if s.instance != nil {
		memory := s.instance.GetMemory()
		if memory != nil {
			s.metrics.MemoryUsage = int64(memory.Size())
		}
	}

	return s.metrics
}

// GetState returns the sandbox state
func (s *WASMSandbox) GetState() WASMSandboxState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Pause pauses the sandbox
func (s *WASMSandbox) Pause(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != WASMStateRunning {
		return errors.New("sandbox not running")
	}

	s.state = WASMStatePaused
	return nil
}

// Resume resumes the sandbox
func (s *WASMSandbox) Resume(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != WASMStatePaused {
		return errors.New("sandbox not paused")
	}

	s.state = WASMStateRunning
	return nil
}

// Terminate terminates the sandbox
func (s *WASMSandbox) Terminate() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := context.Background()

	if s.instance != nil {
		s.instance.Close(ctx)
		s.instance = nil
	}

	if s.module != nil {
		s.module.Close(ctx)
		s.module = nil
	}

	s.state = WASMStateTerminated
	return nil
}

// SetAuditLog sets the audit log
func (s *WASMSandbox) SetAuditLog(logger *AuditLog) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditLog = logger
}

func (s *WASMSandbox) createWASIFuncs() map[string]WASMHostFunc {
	funcs := make(map[string]WASMHostFunc)

	// Check if function is allowed
	isAllowed := func(name string) bool {
		if len(s.config.AllowedWASICalls) == 0 {
			return true // Allow all if no whitelist
		}
		for _, allowed := range s.config.AllowedWASICalls {
			if allowed == name {
				return true
			}
		}
		return false
	}

	// fd_write - write to file descriptor
	if isAllowed("fd_write") {
		funcs["wasi_snapshot_preview1:fd_write"] = func(ctx context.Context, args ...interface{}) ([]interface{}, error) {
			// Basic implementation that writes to stdout/stderr
			return []interface{}{int32(0)}, nil // Return bytes written
		}
	}

	// fd_read - read from file descriptor
	if isAllowed("fd_read") {
		funcs["wasi_snapshot_preview1:fd_read"] = func(ctx context.Context, args ...interface{}) ([]interface{}, error) {
			return []interface{}{int32(0)}, nil
		}
	}

	// clock_time_get - get current time
	if isAllowed("clock_time_get") {
		funcs["wasi_snapshot_preview1:clock_time_get"] = func(ctx context.Context, args ...interface{}) ([]interface{}, error) {
			return []interface{}{int64(time.Now().UnixNano())}, nil
		}
	}

	// random_get - get random bytes
	if isAllowed("random_get") {
		funcs["wasi_snapshot_preview1:random_get"] = func(ctx context.Context, args ...interface{}) ([]interface{}, error) {
			return []interface{}{int32(0)}, nil
		}
	}

	// environ_sizes_get - get environment sizes
	if isAllowed("environ_sizes_get") {
		funcs["wasi_snapshot_preview1:environ_sizes_get"] = func(ctx context.Context, args ...interface{}) ([]interface{}, error) {
			envCount := len(s.config.Environment)
			var envSize int
			for k, v := range s.config.Environment {
				envSize += len(k) + len(v) + 2 // key=value\0
			}
			return []interface{}{int32(envCount), int32(envSize)}, nil
		}
	}

	// proc_exit - exit process
	if isAllowed("proc_exit") {
		funcs["wasi_snapshot_preview1:proc_exit"] = func(ctx context.Context, args ...interface{}) ([]interface{}, error) {
			// Trigger sandbox termination
			return nil, errors.New("process exit requested")
		}
	}

	return funcs
}

// WASMSandboxFactory creates WASM sandboxes
type WASMSandboxFactory struct {
	runtime WASMRuntime
	config  *WASMConfig
}

// NewWASMSandboxFactory creates a new WASM sandbox factory
func NewWASMSandboxFactory(runtime WASMRuntime, config *WASMConfig) *WASMSandboxFactory {
	return &WASMSandboxFactory{
		runtime: runtime,
		config:  config,
	}
}

// Create creates a new WASM sandbox
func (f *WASMSandboxFactory) Create(ctx context.Context) (*WASMSandbox, error) {
	return NewWASMSandbox(f.runtime, f.config), nil
}

// WASMSandboxPool manages a pool of WASM sandboxes
type WASMSandboxPool struct {
	runtime  WASMRuntime
	config   *WASMConfig
	pool     []*WASMSandbox
	maxSize  int
	mu       sync.Mutex
}

// NewWASMSandboxPool creates a new WASM sandbox pool
func NewWASMSandboxPool(runtime WASMRuntime, config *WASMConfig, maxSize int) *WASMSandboxPool {
	return &WASMSandboxPool{
		runtime: runtime,
		config:  config,
		pool:    make([]*WASMSandbox, 0),
		maxSize: maxSize,
	}
}

// Acquire gets a sandbox from the pool or creates a new one
func (p *WASMSandboxPool) Acquire(ctx context.Context) (*WASMSandbox, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Try to get from pool
	for i, sandbox := range p.pool {
		if sandbox.GetState() == WASMStateRunning {
			p.pool = append(p.pool[:i], p.pool[i+1:]...)
			return sandbox, nil
		}
	}

	// Create new sandbox
	return NewWASMSandbox(p.runtime, p.config), nil
}

// Release returns a sandbox to the pool
func (p *WASMSandboxPool) Release(sandbox *WASMSandbox) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.pool) >= p.maxSize {
		sandbox.Terminate()
		return
	}

	p.pool = append(p.pool, sandbox)
}

// Cleanup terminates all sandboxes in the pool
func (p *WASMSandboxPool) Cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, sandbox := range p.pool {
		sandbox.Terminate()
	}

	p.pool = make([]*WASMSandbox, 0)
}

// WASMModuleCache caches compiled WASM modules
type WASMModuleCache struct {
	runtime WASMRuntime
	modules map[string]WASMModule
	mu      sync.RWMutex
}

// NewWASMModuleCache creates a new module cache
func NewWASMModuleCache(runtime WASMRuntime) *WASMModuleCache {
	return &WASMModuleCache{
		runtime: runtime,
		modules: make(map[string]WASMModule),
	}
}

// Get gets a module from cache or compiles it
func (c *WASMModuleCache) Get(ctx context.Context, name string, wasmBytes []byte) (WASMModule, error) {
	c.mu.RLock()
	if module, ok := c.modules[name]; ok {
		c.mu.RUnlock()
		return module, nil
	}
	c.mu.RUnlock()

	// Compile module
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if module, ok := c.modules[name]; ok {
		return module, nil
	}

	module, err := c.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, err
	}

	c.modules[name] = module
	return module, nil
}

// Clear clears the module cache
func (c *WASMModuleCache) Clear(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, module := range c.modules {
		module.Close(ctx)
	}

	c.modules = make(map[string]WASMModule)
}
