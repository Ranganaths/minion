package sandbox

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSandboxCreation(t *testing.T) {
	sandbox := NewSandbox(&SandboxConfig{
		Name: "test-sandbox",
	})

	if sandbox.ID == "" {
		t.Error("Expected sandbox ID to be generated")
	}

	if sandbox.Name != "test-sandbox" {
		t.Errorf("Expected name 'test-sandbox', got '%s'", sandbox.Name)
	}

	if sandbox.GetState() != SandboxStateCreated {
		t.Errorf("Expected state 'created', got '%s'", sandbox.GetState())
	}
}

func TestSandboxLifecycle(t *testing.T) {
	sandbox := NewSandbox(&SandboxConfig{Name: "lifecycle-test"})

	// Start
	err := sandbox.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if sandbox.GetState() != SandboxStateRunning {
		t.Errorf("Expected running state after start")
	}

	// Pause
	err = sandbox.Pause()
	if err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	if sandbox.GetState() != SandboxStatePaused {
		t.Errorf("Expected paused state after pause")
	}

	// Resume
	err = sandbox.Resume()
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	if sandbox.GetState() != SandboxStateRunning {
		t.Errorf("Expected running state after resume")
	}

	// Stop
	err = sandbox.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if sandbox.GetState() != SandboxStateStopped {
		t.Errorf("Expected stopped state after stop")
	}
}

func TestSandboxPermissions(t *testing.T) {
	sandbox := NewSandbox(&SandboxConfig{
		Name: "permission-test",
		Permissions: &Permissions{
			AllowFileRead:       true,
			AllowFileWrite:      false,
			AllowNetworkAccess:  false,
			AllowShellExecution: false,
			AllowCodeExecution:  false,
			AllowedPaths:        []string{"/tmp/*", "/home/user/*"},
			DeniedPaths:         []string{"/etc/*", "/root/*"},
		},
	})

	// File read should be allowed
	err := sandbox.CheckPermission(PermissionActionReadFile, "/tmp/test.txt")
	if err != nil {
		t.Errorf("File read should be allowed: %v", err)
	}

	// File write should be denied
	err = sandbox.CheckPermission(PermissionActionWriteFile, "/tmp/test.txt")
	if err == nil {
		t.Error("File write should be denied")
	}

	// Network access should be denied
	err = sandbox.CheckPermission(PermissionActionNetworkAccess, "example.com")
	if err == nil {
		t.Error("Network access should be denied")
	}

	// Shell execution should be denied
	err = sandbox.CheckPermission(PermissionActionShellExec, "ls")
	if err == nil {
		t.Error("Shell execution should be denied")
	}
}

func TestSandboxToolPermissions(t *testing.T) {
	sandbox := NewSandbox(&SandboxConfig{
		Name: "tool-permission-test",
		Permissions: &Permissions{
			AllowedTools: []string{"search", "read_file", "calculator"},
			DeniedTools:  []string{"delete_file", "execute_code"},
		},
	})

	// Allowed tools should pass
	err := sandbox.CheckPermission(PermissionActionUseTool, "search")
	if err != nil {
		t.Errorf("Tool 'search' should be allowed: %v", err)
	}

	err = sandbox.CheckPermission(PermissionActionUseTool, "read_file")
	if err != nil {
		t.Errorf("Tool 'read_file' should be allowed: %v", err)
	}

	// Denied tools should fail
	err = sandbox.CheckPermission(PermissionActionUseTool, "delete_file")
	if err == nil {
		t.Error("Tool 'delete_file' should be denied")
	}

	// Non-listed tools should fail (whitelist mode)
	err = sandbox.CheckPermission(PermissionActionUseTool, "unknown_tool")
	if err == nil {
		t.Error("Unknown tool should be denied when whitelist is set")
	}
}

func TestSandboxPathPermissions(t *testing.T) {
	sandbox := NewSandbox(&SandboxConfig{
		Name: "path-permission-test",
		Permissions: &Permissions{
			AllowFileRead: true,
			AllowedPaths:  []string{"/allowed/*"},
			DeniedPaths:   []string{"/allowed/secret/*"},
		},
	})

	// Allowed path
	err := sandbox.CheckPermission(PermissionActionReadFile, "/allowed/file.txt")
	if err != nil {
		t.Errorf("Path should be allowed: %v", err)
	}

	// Denied subpath
	err = sandbox.CheckPermission(PermissionActionReadFile, "/allowed/secret/passwords.txt")
	if err == nil {
		t.Error("Secret path should be denied")
	}

	// Non-allowed path
	err = sandbox.CheckPermission(PermissionActionReadFile, "/other/file.txt")
	if err == nil {
		t.Error("Non-allowed path should be denied")
	}
}

func TestSandboxResourceLimits(t *testing.T) {
	sandbox := NewSandbox(&SandboxConfig{
		Name: "resource-limit-test",
		Limits: &ResourceLimits{
			MaxMemoryMB:          100,
			MaxExecutionTime:     time.Second,
			MaxAPICallsPerMinute: 5,
			MaxTokensPerMinute:   100,
		},
	})

	sandbox.Start()

	// Memory within limit
	sandbox.RecordUsage(ResourceTypeMemory, 50)
	err := sandbox.CheckResourceLimit(ResourceTypeMemory, 40)
	if err != nil {
		t.Errorf("Memory should be within limit: %v", err)
	}

	// Memory exceeds limit
	err = sandbox.CheckResourceLimit(ResourceTypeMemory, 60)
	if err == nil {
		t.Error("Memory should exceed limit")
	}
}

func TestSandboxRateLimiting(t *testing.T) {
	sandbox := NewSandbox(&SandboxConfig{
		Name: "rate-limit-test",
		Limits: &ResourceLimits{
			MaxAPICallsPerMinute: 3,
			MaxTokensPerMinute:   10,
		},
	})

	// API calls within limit
	for i := 0; i < 3; i++ {
		err := sandbox.CheckResourceLimit(ResourceTypeAPICall, 1)
		if err != nil {
			t.Errorf("API call %d should be allowed: %v", i+1, err)
		}
	}

	// API call exceeds limit
	err := sandbox.CheckResourceLimit(ResourceTypeAPICall, 1)
	if err == nil {
		t.Error("API call should exceed rate limit")
	}
}

func TestSandboxConcurrentTasks(t *testing.T) {
	sandbox := NewSandbox(&SandboxConfig{
		Name: "concurrent-test",
		Limits: &ResourceLimits{
			MaxConcurrentTasks: 2,
		},
	})

	// First two tasks should succeed
	err := sandbox.IncrementConcurrentTasks()
	if err != nil {
		t.Errorf("First task should succeed: %v", err)
	}

	err = sandbox.IncrementConcurrentTasks()
	if err != nil {
		t.Errorf("Second task should succeed: %v", err)
	}

	// Third task should fail
	err = sandbox.IncrementConcurrentTasks()
	if err == nil {
		t.Error("Third task should fail (max concurrent exceeded)")
	}

	// After decrement, should allow new task
	sandbox.DecrementConcurrentTasks()
	err = sandbox.IncrementConcurrentTasks()
	if err != nil {
		t.Errorf("Task should succeed after decrement: %v", err)
	}
}

func TestSandboxViolations(t *testing.T) {
	sandbox := NewSandbox(&SandboxConfig{
		Name: "violation-test",
		Permissions: &Permissions{
			AllowShellExecution: false,
		},
	})

	// Cause a violation
	sandbox.CheckPermission(PermissionActionShellExec, "ls")

	violations := sandbox.GetViolations()
	if len(violations) != 1 {
		t.Errorf("Expected 1 violation, got %d", len(violations))
	}

	if violations[0].Type != ViolationTypePermissionDenied {
		t.Errorf("Expected permission denied violation, got %s", violations[0].Type)
	}
}

func TestSandboxManager(t *testing.T) {
	manager := NewSandboxManager()

	// Create sandbox
	sandbox, err := manager.CreateSandbox(&SandboxConfig{Name: "managed-sandbox"})
	if err != nil {
		t.Fatalf("CreateSandbox failed: %v", err)
	}

	// Get sandbox
	retrieved, err := manager.GetSandbox(sandbox.ID)
	if err != nil {
		t.Fatalf("GetSandbox failed: %v", err)
	}
	if retrieved.Name != "managed-sandbox" {
		t.Errorf("Expected name 'managed-sandbox', got '%s'", retrieved.Name)
	}

	// List sandboxes
	list := manager.ListSandboxes()
	if len(list) != 1 {
		t.Errorf("Expected 1 sandbox, got %d", len(list))
	}

	// Delete sandbox
	err = manager.DeleteSandbox(sandbox.ID)
	if err != nil {
		t.Fatalf("DeleteSandbox failed: %v", err)
	}

	list = manager.ListSandboxes()
	if len(list) != 0 {
		t.Errorf("Expected 0 sandboxes after delete, got %d", len(list))
	}
}

func TestSandboxManagerExecuteInSandbox(t *testing.T) {
	manager := NewSandboxManager()

	sandbox, _ := manager.CreateSandbox(&SandboxConfig{
		Name: "execute-test",
		Limits: &ResourceLimits{
			MaxExecutionTime:   5 * time.Second,
			MaxConcurrentTasks: 2,
		},
	})

	sandbox.Start()

	ctx := context.Background()
	executed := false

	err := manager.ExecuteInSandbox(ctx, sandbox.ID, func(ctx context.Context, s *Sandbox) error {
		executed = true
		return nil
	})

	if err != nil {
		t.Fatalf("ExecuteInSandbox failed: %v", err)
	}

	if !executed {
		t.Error("Function should have been executed")
	}
}

func TestIsolationLevels(t *testing.T) {
	tests := []struct {
		level       IsolationLevel
		expectAudit bool
	}{
		{IsolationLevelNone, false},
		{IsolationLevelBasic, false},
		{IsolationLevelStandard, false},
		{IsolationLevelStrict, true},
		{IsolationLevelMaximum, true},
	}

	for _, tt := range tests {
		policy := IsolationPolicyFromLevel(tt.level)
		if policy.AuditEnabled != tt.expectAudit {
			t.Errorf("Level %s: expected audit=%v, got %v", tt.level, tt.expectAudit, policy.AuditEnabled)
		}
	}
}

func TestSecureSandbox(t *testing.T) {
	var auditHandlerCalled bool

	ss := NewSecureSandbox(SecureSandboxConfig{
		Name:   "secure-test",
		Policy: IsolationPolicyFromLevel(IsolationLevelStrict),
		AlertHandler: func(alert *ResourceAlert) {
			// Alert handler for resource monitoring
		},
		AuditHandler: func(entry *AuditEntry) {
			auditHandlerCalled = true
		},
	})

	ctx := context.Background()
	err := ss.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer ss.Stop()

	// Check permission - should be logged
	ss.CheckPermission(PermissionActionReadFile, "/tmp/test.txt")

	// Check audit log
	if ss.auditLog == nil {
		t.Fatal("Audit log should be enabled for strict level")
	}

	entries := ss.auditLog.GetEntries("", 10)
	if len(entries) == 0 {
		t.Error("Expected audit entries")
	}

	if !auditHandlerCalled {
		t.Error("Audit handler should have been called")
	}
}

func TestAuditLog(t *testing.T) {
	var handlerCalled bool
	log := NewAuditLog(100, func(entry *AuditEntry) {
		handlerCalled = true
	})

	log.Record("sandbox-1", "read_file", "/tmp/test.txt", AuditResultAllowed, "", nil)

	if !handlerCalled {
		t.Error("Handler should have been called")
	}

	entries := log.GetEntries("sandbox-1", 10)
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].Action != "read_file" {
		t.Errorf("Expected action 'read_file', got '%s'", entries[0].Action)
	}
}

func TestAuditLogMaxSize(t *testing.T) {
	log := NewAuditLog(5, nil)

	for i := 0; i < 10; i++ {
		log.Record("sandbox", "action", "", AuditResultAllowed, "", nil)
	}

	entries := log.GetEntries("", 100)
	if len(entries) > 5 {
		t.Errorf("Expected max 5 entries, got %d", len(entries))
	}
}

func TestResourceMonitor(t *testing.T) {
	sandbox := NewSandbox(&SandboxConfig{
		Name: "monitor-test",
		Limits: &ResourceLimits{
			MaxMemoryMB:      100,
			MaxExecutionTime: time.Minute,
		},
	})

	sandbox.Start()

	var usageReported bool
	monitor := NewResourceMonitor(MonitorConfig{
		Sandbox:  sandbox,
		Interval: 50 * time.Millisecond,
	})

	monitor.AddHandler(func(usage *ResourceUsage, limits *ResourceLimits) {
		usageReported = true
	})

	ctx, cancel := context.WithCancel(context.Background())
	monitor.Start(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()

	if !usageReported {
		t.Error("Usage should have been reported")
	}
}

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(5, 100)

	// Allow first 5 API calls
	for i := 0; i < 5; i++ {
		if !limiter.AllowAPICall() {
			t.Errorf("API call %d should be allowed", i+1)
		}
	}

	// 6th call should be denied
	if limiter.AllowAPICall() {
		t.Error("6th API call should be denied")
	}

	// Tokens within limit
	if !limiter.AllowTokens(50) {
		t.Error("50 tokens should be allowed")
	}

	if !limiter.AllowTokens(50) {
		t.Error("Another 50 tokens should be allowed")
	}

	// Exceeds limit
	if limiter.AllowTokens(50) {
		t.Error("Additional 50 tokens should exceed limit")
	}
}

func TestPatternMatching(t *testing.T) {
	tests := []struct {
		value   string
		pattern string
		match   bool
	}{
		{"hello", "hello", true},
		{"hello", "hello*", true},
		{"hello", "hel*", true},
		{"hello", "world", false},
		{"hello", "*", true},
		{"hello/world", "hello/*", true},
	}

	for _, tt := range tests {
		result := matchPattern(tt.value, tt.pattern)
		if result != tt.match {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.value, tt.pattern, result, tt.match)
		}
	}
}

func TestConcurrentSandboxAccess(t *testing.T) {
	sandbox := NewSandbox(&SandboxConfig{
		Name: "concurrent-access-test",
		Limits: &ResourceLimits{
			MaxConcurrentTasks: 100,
		},
	})

	sandbox.Start()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sandbox.IncrementConcurrentTasks(); err != nil {
				errors <- err
				return
			}
			time.Sleep(10 * time.Millisecond)
			sandbox.DecrementConcurrentTasks()
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}

func TestDefaultLimitsAndPermissions(t *testing.T) {
	limits := DefaultResourceLimits()
	if limits.MaxMemoryMB <= 0 {
		t.Error("Default MaxMemoryMB should be positive")
	}
	if limits.MaxExecutionTime <= 0 {
		t.Error("Default MaxExecutionTime should be positive")
	}

	perms := DefaultPermissions()
	if !perms.AllowFileRead {
		t.Error("Default should allow file read")
	}
	if perms.AllowShellExecution {
		t.Error("Default should not allow shell execution")
	}
}
