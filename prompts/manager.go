package prompts

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PromptManager provides a unified interface for prompt management
type PromptManager struct {
	store          PromptStore
	engine         *TemplateEngine
	abManager      *ABTestManager
	versionControl *VersionController
	mu             sync.RWMutex
}

// PromptManagerConfig configures the prompt manager
type PromptManagerConfig struct {
	Store          PromptStore
	TemplateEngine *TemplateEngine
}

// NewPromptManager creates a new prompt manager
func NewPromptManager(config PromptManagerConfig) *PromptManager {
	if config.Store == nil {
		config.Store = NewMemoryPromptStore()
	}
	if config.TemplateEngine == nil {
		config.TemplateEngine = NewTemplateEngine(TemplateEngineConfig{})
	}

	pm := &PromptManager{
		store:  config.Store,
		engine: config.TemplateEngine,
		versionControl: &VersionController{
			store: config.Store,
		},
	}

	pm.abManager = NewABTestManager(ABTestManagerConfig{
		PromptStore:    config.Store,
		TemplateEngine: config.TemplateEngine,
	})

	return pm
}

// CreatePrompt creates a new prompt template
func (m *PromptManager) CreatePrompt(ctx context.Context, name, content string, variables []PromptVariable) (*PromptTemplate, error) {
	prompt := &PromptTemplate{
		ID:        uuid.New().String(),
		Name:      name,
		Content:   content,
		Version:   1,
		Status:    PromptStatusDraft,
		Variables: variables,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := m.store.Create(ctx, prompt); err != nil {
		return nil, err
	}

	return prompt, nil
}

// UpdatePrompt creates a new version of a prompt
func (m *PromptManager) UpdatePrompt(ctx context.Context, name, content string, variables []PromptVariable) (*PromptTemplate, error) {
	return m.versionControl.CreateVersion(ctx, name, content, variables, "")
}

// GetPrompt retrieves the active version of a prompt by name
func (m *PromptManager) GetPrompt(ctx context.Context, name string) (*PromptTemplate, error) {
	return m.store.GetByName(ctx, name)
}

// GetPromptVersion retrieves a specific version of a prompt
func (m *PromptManager) GetPromptVersion(ctx context.Context, name string, version int) (*PromptTemplate, error) {
	return m.store.GetVersion(ctx, name, version)
}

// Render renders a prompt with variables
func (m *PromptManager) Render(ctx context.Context, name string, variables map[string]interface{}) (string, error) {
	prompt, err := m.store.GetByName(ctx, name)
	if err != nil {
		return "", err
	}

	return m.engine.Render(prompt, variables)
}

// RenderWithVersion renders a specific version of a prompt
func (m *PromptManager) RenderWithVersion(ctx context.Context, name string, version int, variables map[string]interface{}) (string, error) {
	prompt, err := m.store.GetVersion(ctx, name, version)
	if err != nil {
		return "", err
	}

	return m.engine.Render(prompt, variables)
}

// Activate activates a prompt version
func (m *PromptManager) Activate(ctx context.Context, name string, version int) error {
	return m.versionControl.Activate(ctx, name, version)
}

// ListPrompts lists all prompts
func (m *PromptManager) ListPrompts(ctx context.Context, filter *PromptFilter) ([]*PromptTemplate, error) {
	return m.store.List(ctx, filter)
}

// GetHistory gets version history for a prompt
func (m *PromptManager) GetHistory(ctx context.Context, name string) ([]*PromptTemplate, error) {
	return m.store.GetHistory(ctx, name)
}

// RecordExecution records a prompt execution
func (m *PromptManager) RecordExecution(ctx context.Context, promptID string, variables map[string]interface{}, response string, tokensUsed int, latency time.Duration, success bool, errMsg string) error {
	prompt, err := m.store.Get(ctx, promptID)
	if err != nil {
		return err
	}

	rendered, _ := m.engine.Render(prompt, variables)

	execution := &PromptExecution{
		ID:            uuid.New().String(),
		PromptID:      promptID,
		PromptVersion: prompt.Version,
		Variables:     variables,
		RenderedText:  rendered,
		Response:      response,
		TokensUsed:    tokensUsed,
		Latency:       latency,
		Success:       success,
		Error:         errMsg,
		ExecutedAt:    time.Now(),
	}

	return m.store.RecordExecution(ctx, execution)
}

// GetExecutions gets executions for a prompt
func (m *PromptManager) GetExecutions(ctx context.Context, promptID string, limit int) ([]*PromptExecution, error) {
	return m.store.GetExecutions(ctx, promptID, limit)
}

// A/B Testing Methods

// CreateABTest creates a new A/B test
func (m *PromptManager) CreateABTest(ctx context.Context, name string, promptIDs []string, targetMetric string) (*ABTest, error) {
	if len(promptIDs) < 2 {
		return nil, errors.New("at least 2 prompts are required for A/B testing")
	}

	variants := make([]ABTestVariant, len(promptIDs))
	for i, pid := range promptIDs {
		prompt, err := m.store.Get(ctx, pid)
		if err != nil {
			return nil, fmt.Errorf("prompt %s not found", pid)
		}

		variants[i] = ABTestVariant{
			ID:       uuid.New().String(),
			Name:     prompt.Name,
			PromptID: pid,
			Weight:   1.0 / float64(len(promptIDs)), // Equal weights
		}
	}

	test := &ABTest{
		ID:           uuid.New().String(),
		Name:         name,
		Status:       ABTestStatusDraft,
		Variants:     variants,
		TargetMetric: targetMetric,
		MinSamples:   100,
		Confidence:   0.95,
		CreatedAt:    time.Now(),
	}

	if err := m.abManager.CreateTest(ctx, test); err != nil {
		return nil, err
	}

	return test, nil
}

// StartABTest starts an A/B test
func (m *PromptManager) StartABTest(ctx context.Context, testID string) error {
	return m.abManager.StartTest(ctx, testID)
}

// EndABTest ends an A/B test
func (m *PromptManager) EndABTest(ctx context.Context, testID string) error {
	return m.abManager.EndTest(ctx, testID)
}

// GetTestVariant selects a variant for a user
func (m *PromptManager) GetTestVariant(ctx context.Context, testID, userID string) (*ABTestVariant, error) {
	return m.abManager.SelectVariant(ctx, testID, userID)
}

// RecordTestResult records a result for an A/B test
func (m *PromptManager) RecordTestResult(ctx context.Context, testID, variantID, userID string, converted bool, rating *float64) error {
	result := &ABTestResult{
		TestID:     testID,
		VariantID:  variantID,
		UserID:     userID,
		Converted:  converted,
		Rating:     rating,
		RecordedAt: time.Now(),
	}
	return m.abManager.RecordResult(ctx, result)
}

// AnalyzeABTest analyzes an A/B test
func (m *PromptManager) AnalyzeABTest(ctx context.Context, testID string) (*ABTestAnalysis, error) {
	return m.abManager.AnalyzeTest(ctx, testID)
}

// GetABTest gets an A/B test
func (m *PromptManager) GetABTest(ctx context.Context, testID string) (*ABTest, error) {
	return m.abManager.GetTest(ctx, testID)
}

// ListABTests lists A/B tests
func (m *PromptManager) ListABTests(ctx context.Context, status ABTestStatus) ([]*ABTest, error) {
	return m.abManager.ListTests(ctx, status)
}

// RenderForTest renders a prompt for an A/B test
func (m *PromptManager) RenderForTest(ctx context.Context, testID, userID string, variables map[string]interface{}) (string, *ABTestVariant, error) {
	return m.abManager.GetPromptForTest(ctx, testID, userID, variables)
}

// VersionController manages prompt versions
type VersionController struct {
	store PromptStore
	mu    sync.Mutex
}

// CreateVersion creates a new version of a prompt
func (v *VersionController) CreateVersion(ctx context.Context, name, content string, variables []PromptVariable, createdBy string) (*PromptTemplate, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Get current version
	history, err := v.store.GetHistory(ctx, name)
	if err != nil {
		return nil, err
	}

	var latestVersion int
	var parentID string
	if len(history) > 0 {
		for _, p := range history {
			if p.Version > latestVersion {
				latestVersion = p.Version
				parentID = p.ID
			}
		}
	}

	// Create new version
	newPrompt := &PromptTemplate{
		ID:        uuid.New().String(),
		Name:      name,
		Content:   content,
		Version:   latestVersion + 1,
		Status:    PromptStatusDraft,
		Variables: variables,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: createdBy,
		ParentID:  parentID,
	}

	if err := v.store.Create(ctx, newPrompt); err != nil {
		return nil, err
	}

	return newPrompt, nil
}

// Activate activates a specific version and deactivates others
func (v *VersionController) Activate(ctx context.Context, name string, version int) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	history, err := v.store.GetHistory(ctx, name)
	if err != nil {
		return err
	}

	var targetPrompt *PromptTemplate
	for _, p := range history {
		if p.Version == version {
			targetPrompt = p
		}
	}

	if targetPrompt == nil {
		return errors.New("version not found")
	}

	// Deactivate all other versions
	for _, p := range history {
		if p.Status == PromptStatusActive && p.ID != targetPrompt.ID {
			p.Status = PromptStatusArchived
			v.store.Update(ctx, p)
		}
	}

	// Activate target version
	targetPrompt.Status = PromptStatusActive
	targetPrompt.UpdatedAt = time.Now()
	return v.store.Update(ctx, targetPrompt)
}

// Rollback rolls back to a previous version
func (v *VersionController) Rollback(ctx context.Context, name string, version int) error {
	return v.Activate(ctx, name, version)
}

// GetDiff returns the difference between two versions (simplified)
func (v *VersionController) GetDiff(ctx context.Context, name string, version1, version2 int) (*VersionDiff, error) {
	p1, err := v.store.GetVersion(ctx, name, version1)
	if err != nil {
		return nil, err
	}

	p2, err := v.store.GetVersion(ctx, name, version2)
	if err != nil {
		return nil, err
	}

	return &VersionDiff{
		Name:          name,
		Version1:      version1,
		Version2:      version2,
		Content1:      p1.Content,
		Content2:      p2.Content,
		ContentChanged: p1.Content != p2.Content,
	}, nil
}

// VersionDiff represents the difference between two versions
type VersionDiff struct {
	Name           string `json:"name"`
	Version1       int    `json:"version1"`
	Version2       int    `json:"version2"`
	Content1       string `json:"content1"`
	Content2       string `json:"content2"`
	ContentChanged bool   `json:"content_changed"`
}

// PromptLibrary provides a collection of reusable prompts
type PromptLibrary struct {
	manager *PromptManager
}

// NewPromptLibrary creates a new prompt library
func NewPromptLibrary(manager *PromptManager) *PromptLibrary {
	return &PromptLibrary{manager: manager}
}

// RegisterPrompt registers a prompt in the library
func (l *PromptLibrary) RegisterPrompt(ctx context.Context, name, description, content string, variables []PromptVariable, tags []string) error {
	prompt, err := l.manager.CreatePrompt(ctx, name, content, variables)
	if err != nil {
		return err
	}

	prompt.Description = description
	prompt.Tags = tags
	prompt.Status = PromptStatusActive

	return l.manager.store.Update(ctx, prompt)
}

// GetPromptsByTag gets prompts by tag
func (l *PromptLibrary) GetPromptsByTag(ctx context.Context, tag string) ([]*PromptTemplate, error) {
	all, err := l.manager.ListPrompts(ctx, nil)
	if err != nil {
		return nil, err
	}

	results := make([]*PromptTemplate, 0)
	for _, p := range all {
		for _, t := range p.Tags {
			if t == tag {
				results = append(results, p)
				break
			}
		}
	}
	return results, nil
}

// SearchPrompts searches prompts by keyword in name or description
func (l *PromptLibrary) SearchPrompts(ctx context.Context, keyword string) ([]*PromptTemplate, error) {
	all, err := l.manager.ListPrompts(ctx, nil)
	if err != nil {
		return nil, err
	}

	results := make([]*PromptTemplate, 0)
	for _, p := range all {
		if containsIgnoreCase(p.Name, keyword) || containsIgnoreCase(p.Description, keyword) {
			results = append(results, p)
		}
	}
	return results, nil
}

// containsIgnoreCase checks if s contains substr (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1, c2 := s[i+j], substr[j]
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c1 != c2 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
