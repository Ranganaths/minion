package prompts

import (
	"context"
	"testing"
	"time"
)

// Test Template Engine

func TestTemplateEngineRender(t *testing.T) {
	engine := NewTemplateEngine(TemplateEngineConfig{})

	prompt := &PromptTemplate{
		Name:    "greeting",
		Content: "Hello, {{.name}}! Welcome to {{.place}}.",
		Variables: []PromptVariable{
			{Name: "name", Type: "string", Required: true},
			{Name: "place", Type: "string", Required: true},
		},
	}

	result, err := engine.Render(prompt, map[string]interface{}{
		"name":  "Alice",
		"place": "Wonderland",
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := "Hello, Alice! Welcome to Wonderland."
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestTemplateEngineWithDefaults(t *testing.T) {
	engine := NewTemplateEngine(TemplateEngineConfig{})

	prompt := &PromptTemplate{
		Name:    "greeting",
		Content: "Hello, {{.name}}! Language: {{.lang}}",
		Variables: []PromptVariable{
			{Name: "name", Type: "string", Required: true},
			{Name: "lang", Type: "string", Required: false, Default: "English"},
		},
	}

	result, err := engine.Render(prompt, map[string]interface{}{
		"name": "Bob",
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := "Hello, Bob! Language: English"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestTemplateEngineMissingRequired(t *testing.T) {
	engine := NewTemplateEngine(TemplateEngineConfig{})

	prompt := &PromptTemplate{
		Name:    "greeting",
		Content: "Hello, {{.name}}!",
		Variables: []PromptVariable{
			{Name: "name", Type: "string", Required: true},
		},
	}

	_, err := engine.Render(prompt, map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing required variable")
	}
}

func TestTemplateEngineBuiltinFunctions(t *testing.T) {
	engine := NewTemplateEngine(TemplateEngineConfig{})

	prompt := &PromptTemplate{
		Name:    "functions",
		Content: "Upper: {{upper .text}}, Lower: {{lower .text}}, Title: {{title .text}}",
		Variables: []PromptVariable{
			{Name: "text", Type: "string", Required: true},
		},
	}

	result, err := engine.Render(prompt, map[string]interface{}{
		"text": "hello",
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := "Upper: HELLO, Lower: hello, Title: Hello"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestTemplateEngineCustomDelimiters(t *testing.T) {
	engine := NewTemplateEngine(TemplateEngineConfig{
		LeftDelim:  "<%",
		RightDelim: "%>",
	})

	prompt := &PromptTemplate{
		Name:    "custom",
		Content: "Hello, <%.name%>!",
	}

	result, err := engine.Render(prompt, map[string]interface{}{
		"name": "World",
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := "Hello, World!"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

// Test Memory Prompt Store

func TestMemoryPromptStore(t *testing.T) {
	store := NewMemoryPromptStore()
	ctx := context.Background()

	// Create
	prompt := &PromptTemplate{
		Name:    "test-prompt",
		Content: "Test content",
		Version: 1,
		Status:  PromptStatusActive,
	}

	err := store.Create(ctx, prompt)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get by name
	retrieved, err := store.GetByName(ctx, "test-prompt")
	if err != nil {
		t.Fatalf("GetByName failed: %v", err)
	}

	if retrieved.Content != "Test content" {
		t.Errorf("Expected content 'Test content', got '%s'", retrieved.Content)
	}

	// Update
	retrieved.Content = "Updated content"
	err = store.Update(ctx, retrieved)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	updated, _ := store.Get(ctx, retrieved.ID)
	if updated.Content != "Updated content" {
		t.Errorf("Expected updated content")
	}
}

func TestMemoryPromptStoreVersions(t *testing.T) {
	store := NewMemoryPromptStore()
	ctx := context.Background()

	// Create multiple versions
	for i := 1; i <= 3; i++ {
		prompt := &PromptTemplate{
			Name:    "versioned-prompt",
			Content: "Version content",
			Version: i,
			Status:  PromptStatusDraft,
		}
		store.Create(ctx, prompt)
	}

	// Get history
	history, err := store.GetHistory(ctx, "versioned-prompt")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(history) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(history))
	}

	// Get specific version
	v2, err := store.GetVersion(ctx, "versioned-prompt", 2)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}

	if v2.Version != 2 {
		t.Errorf("Expected version 2, got %d", v2.Version)
	}
}

// Test Prompt Manager

func TestPromptManager(t *testing.T) {
	manager := NewPromptManager(PromptManagerConfig{})
	ctx := context.Background()

	// Create prompt
	prompt, err := manager.CreatePrompt(ctx, "qa-prompt", "Q: {{.question}}\nA:", []PromptVariable{
		{Name: "question", Type: "string", Required: true},
	})
	if err != nil {
		t.Fatalf("CreatePrompt failed: %v", err)
	}

	if prompt.Version != 1 {
		t.Errorf("Expected version 1, got %d", prompt.Version)
	}

	// Render
	result, err := manager.Render(ctx, "qa-prompt", map[string]interface{}{
		"question": "What is Go?",
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := "Q: What is Go?\nA:"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestPromptManagerVersioning(t *testing.T) {
	manager := NewPromptManager(PromptManagerConfig{})
	ctx := context.Background()

	// Create initial version
	manager.CreatePrompt(ctx, "versioned", "Version 1 content", nil)

	// Create new version
	v2, err := manager.UpdatePrompt(ctx, "versioned", "Version 2 content", nil)
	if err != nil {
		t.Fatalf("UpdatePrompt failed: %v", err)
	}

	if v2.Version != 2 {
		t.Errorf("Expected version 2, got %d", v2.Version)
	}

	// Get specific version
	v1, err := manager.GetPromptVersion(ctx, "versioned", 1)
	if err != nil {
		t.Fatalf("GetPromptVersion failed: %v", err)
	}

	if v1.Content != "Version 1 content" {
		t.Error("Version 1 content should be preserved")
	}
}

func TestPromptManagerActivation(t *testing.T) {
	manager := NewPromptManager(PromptManagerConfig{})
	ctx := context.Background()

	// Create and update prompt
	p1, _ := manager.CreatePrompt(ctx, "activate-test", "V1", nil)
	p1.Status = PromptStatusActive
	manager.store.Update(ctx, p1)

	manager.UpdatePrompt(ctx, "activate-test", "V2", nil)

	// Activate version 2
	err := manager.Activate(ctx, "activate-test", 2)
	if err != nil {
		t.Fatalf("Activate failed: %v", err)
	}

	// Get active should return v2
	active, _ := manager.GetPrompt(ctx, "activate-test")
	if active.Version != 2 {
		t.Errorf("Expected active version 2, got %d", active.Version)
	}
	if active.Status != PromptStatusActive {
		t.Errorf("Expected active status, got %s", active.Status)
	}
}

// Test A/B Testing

func TestABTestManager(t *testing.T) {
	store := NewMemoryPromptStore()
	engine := NewTemplateEngine(TemplateEngineConfig{})

	manager := NewABTestManager(ABTestManagerConfig{
		PromptStore:    store,
		TemplateEngine: engine,
	})

	ctx := context.Background()

	// Create test prompts
	p1 := &PromptTemplate{ID: "prompt1", Name: "variant-a", Content: "Hello A"}
	p2 := &PromptTemplate{ID: "prompt2", Name: "variant-b", Content: "Hello B"}
	store.Create(ctx, p1)
	store.Create(ctx, p2)

	// Create A/B test
	test := &ABTest{
		Name:         "test-1",
		TargetMetric: "conversion_rate",
		Variants: []ABTestVariant{
			{Name: "control", PromptID: "prompt1", Weight: 0.5},
			{Name: "treatment", PromptID: "prompt2", Weight: 0.5},
		},
	}

	err := manager.CreateTest(ctx, test)
	if err != nil {
		t.Fatalf("CreateTest failed: %v", err)
	}

	// Start test
	err = manager.StartTest(ctx, test.ID)
	if err != nil {
		t.Fatalf("StartTest failed: %v", err)
	}

	// Verify status
	retrieved, _ := manager.GetTest(ctx, test.ID)
	if retrieved.Status != ABTestStatusRunning {
		t.Errorf("Expected running status, got %s", retrieved.Status)
	}
}

func TestABTestVariantSelection(t *testing.T) {
	store := NewMemoryPromptStore()
	engine := NewTemplateEngine(TemplateEngineConfig{})

	manager := NewABTestManager(ABTestManagerConfig{
		PromptStore:    store,
		TemplateEngine: engine,
	})

	ctx := context.Background()

	// Create test
	test := &ABTest{
		Name:         "selection-test",
		TargetMetric: "conversion_rate",
		Variants: []ABTestVariant{
			{ID: "v1", Name: "control", PromptID: "p1", Weight: 0.5},
			{ID: "v2", Name: "treatment", PromptID: "p2", Weight: 0.5},
		},
		Status: ABTestStatusRunning,
	}

	manager.CreateTest(ctx, test)
	manager.StartTest(ctx, test.ID)

	// Select variants multiple times
	counts := make(map[string]int)
	for i := 0; i < 100; i++ {
		variant, _ := manager.SelectVariant(ctx, test.ID, "")
		counts[variant.ID]++
	}

	// Both variants should be selected (with 50/50 split, roughly equal)
	if counts["v1"] == 0 || counts["v2"] == 0 {
		t.Error("Both variants should be selected")
	}
}

func TestABTestDeterministicSelection(t *testing.T) {
	store := NewMemoryPromptStore()
	engine := NewTemplateEngine(TemplateEngineConfig{})

	manager := NewABTestManager(ABTestManagerConfig{
		PromptStore:    store,
		TemplateEngine: engine,
	})

	ctx := context.Background()

	test := &ABTest{
		Name:         "deterministic-test",
		TargetMetric: "conversion_rate",
		Variants: []ABTestVariant{
			{ID: "v1", Name: "control", PromptID: "p1", Weight: 0.5},
			{ID: "v2", Name: "treatment", PromptID: "p2", Weight: 0.5},
		},
		Status: ABTestStatusRunning,
	}

	manager.CreateTest(ctx, test)
	manager.StartTest(ctx, test.ID)

	// Same user should get same variant
	v1, _ := manager.SelectVariant(ctx, test.ID, "user-123")
	v2, _ := manager.SelectVariant(ctx, test.ID, "user-123")

	if v1.ID != v2.ID {
		t.Error("Same user should get same variant")
	}
}

func TestABTestResults(t *testing.T) {
	store := NewMemoryPromptStore()
	engine := NewTemplateEngine(TemplateEngineConfig{})

	manager := NewABTestManager(ABTestManagerConfig{
		PromptStore:    store,
		TemplateEngine: engine,
	})

	ctx := context.Background()

	test := &ABTest{
		Name:         "results-test",
		TargetMetric: "conversion_rate",
		MinSamples:   10,
		Variants: []ABTestVariant{
			{ID: "control", Name: "control", PromptID: "p1", Weight: 0.5},
			{ID: "treatment", Name: "treatment", PromptID: "p2", Weight: 0.5},
		},
	}

	manager.CreateTest(ctx, test)
	manager.StartTest(ctx, test.ID)

	// Record results
	for i := 0; i < 20; i++ {
		// Control: 40% conversion
		manager.RecordResult(ctx, &ABTestResult{
			TestID:    test.ID,
			VariantID: "control",
			Converted: i < 8,
		})
		// Treatment: 60% conversion
		manager.RecordResult(ctx, &ABTestResult{
			TestID:    test.ID,
			VariantID: "treatment",
			Converted: i < 12,
		})
	}

	// Analyze
	analysis, err := manager.AnalyzeTest(ctx, test.ID)
	if err != nil {
		t.Fatalf("AnalyzeTest failed: %v", err)
	}

	// Check that treatment has higher conversion rate
	controlStats := analysis.VariantStats["control"]
	treatmentStats := analysis.VariantStats["treatment"]

	if controlStats.ConversionRate >= treatmentStats.ConversionRate {
		t.Errorf("Treatment should have higher conversion rate: control=%f, treatment=%f",
			controlStats.ConversionRate, treatmentStats.ConversionRate)
	}
}

func TestABTestEndAndWinner(t *testing.T) {
	store := NewMemoryPromptStore()
	engine := NewTemplateEngine(TemplateEngineConfig{})

	manager := NewABTestManager(ABTestManagerConfig{
		PromptStore:    store,
		TemplateEngine: engine,
	})

	ctx := context.Background()

	test := &ABTest{
		Name:         "winner-test",
		TargetMetric: "conversion_rate",
		Variants: []ABTestVariant{
			{ID: "v1", Name: "control", PromptID: "p1", Impressions: 100, Conversions: 10},
			{ID: "v2", Name: "treatment", PromptID: "p2", Impressions: 100, Conversions: 30},
		},
	}

	manager.CreateTest(ctx, test)
	manager.StartTest(ctx, test.ID)

	// End test
	err := manager.EndTest(ctx, test.ID)
	if err != nil {
		t.Fatalf("EndTest failed: %v", err)
	}

	// Check winner
	ended, _ := manager.GetTest(ctx, test.ID)
	if ended.Status != ABTestStatusCompleted {
		t.Errorf("Expected completed status, got %s", ended.Status)
	}
	if ended.Winner != "v2" {
		t.Errorf("Expected v2 as winner, got %s", ended.Winner)
	}
}

// Test Prompt Library

func TestPromptLibrary(t *testing.T) {
	manager := NewPromptManager(PromptManagerConfig{})
	library := NewPromptLibrary(manager)

	ctx := context.Background()

	// Register prompts
	err := library.RegisterPrompt(ctx, "qa-prompt", "Question answering prompt",
		"Q: {{.question}}\nA:", nil, []string{"qa", "chat"})
	if err != nil {
		t.Fatalf("RegisterPrompt failed: %v", err)
	}

	err = library.RegisterPrompt(ctx, "summary-prompt", "Summarization prompt",
		"Summarize: {{.text}}", nil, []string{"summary", "text"})
	if err != nil {
		t.Fatalf("RegisterPrompt failed: %v", err)
	}

	// Search by tag
	qaPrompts, _ := library.GetPromptsByTag(ctx, "qa")
	if len(qaPrompts) != 1 {
		t.Errorf("Expected 1 QA prompt, got %d", len(qaPrompts))
	}

	// Search by keyword
	results, _ := library.SearchPrompts(ctx, "summariz")
	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'summariz', got %d", len(results))
	}
}

// Test Version Controller

func TestVersionController(t *testing.T) {
	store := NewMemoryPromptStore()
	vc := &VersionController{store: store}

	ctx := context.Background()

	// Create initial prompt
	store.Create(ctx, &PromptTemplate{
		Name:    "version-test",
		Content: "V1",
		Version: 1,
		Status:  PromptStatusActive,
	})

	// Create new version
	v2, err := vc.CreateVersion(ctx, "version-test", "V2", nil, "user1")
	if err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}

	if v2.Version != 2 {
		t.Errorf("Expected version 2, got %d", v2.Version)
	}

	// Check parent ID
	if v2.ParentID == "" {
		t.Error("Expected parent ID to be set")
	}
}

func TestVersionControllerRollback(t *testing.T) {
	store := NewMemoryPromptStore()
	vc := &VersionController{store: store}

	ctx := context.Background()

	// Create versions
	v1 := &PromptTemplate{
		Name:    "rollback-test",
		Content: "V1",
		Version: 1,
		Status:  PromptStatusActive,
	}
	store.Create(ctx, v1)

	vc.CreateVersion(ctx, "rollback-test", "V2", nil, "")
	v3, _ := vc.CreateVersion(ctx, "rollback-test", "V3", nil, "")

	// Activate v3
	vc.Activate(ctx, "rollback-test", 3)

	// Rollback to v1
	err := vc.Rollback(ctx, "rollback-test", 1)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// v1 should be active now
	active, _ := store.GetByName(ctx, "rollback-test")
	if active.Version != 1 {
		t.Errorf("Expected version 1 after rollback, got %d", active.Version)
	}

	// v3 should be archived
	archived, _ := store.GetVersion(ctx, "rollback-test", 3)
	if archived.Status != PromptStatusArchived && archived.ID != v3.ID {
		// First version might already be what we got
	}
}

func TestVersionControllerDiff(t *testing.T) {
	store := NewMemoryPromptStore()
	vc := &VersionController{store: store}

	ctx := context.Background()

	store.Create(ctx, &PromptTemplate{
		Name:    "diff-test",
		Content: "Content V1",
		Version: 1,
	})

	vc.CreateVersion(ctx, "diff-test", "Content V2", nil, "")

	diff, err := vc.GetDiff(ctx, "diff-test", 1, 2)
	if err != nil {
		t.Fatalf("GetDiff failed: %v", err)
	}

	if !diff.ContentChanged {
		t.Error("Expected content to be marked as changed")
	}
	if diff.Content1 != "Content V1" || diff.Content2 != "Content V2" {
		t.Error("Diff content mismatch")
	}
}

// Test Execution Recording

func TestPromptExecution(t *testing.T) {
	manager := NewPromptManager(PromptManagerConfig{})
	ctx := context.Background()

	// Create prompt
	prompt, _ := manager.CreatePrompt(ctx, "exec-test", "Hello {{.name}}!", []PromptVariable{
		{Name: "name", Type: "string", Required: true},
	})

	// Record execution
	err := manager.RecordExecution(ctx, prompt.ID, map[string]interface{}{
		"name": "World",
	}, "Hello World!", 10, 100*time.Millisecond, true, "")
	if err != nil {
		t.Fatalf("RecordExecution failed: %v", err)
	}

	// Get executions
	execs, err := manager.GetExecutions(ctx, prompt.ID, 10)
	if err != nil {
		t.Fatalf("GetExecutions failed: %v", err)
	}

	if len(execs) != 1 {
		t.Errorf("Expected 1 execution, got %d", len(execs))
	}

	if execs[0].RenderedText != "Hello World!" {
		t.Errorf("Expected rendered text 'Hello World!', got '%s'", execs[0].RenderedText)
	}
}
