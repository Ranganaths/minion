// Package prompts provides prompt management, versioning, and A/B testing
// capabilities for LLM applications.
package prompts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
)

// PromptTemplate represents a versioned prompt template
type PromptTemplate struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Content     string                 `json:"content"`
	Version     int                    `json:"version"`
	Status      PromptStatus           `json:"status"`
	Variables   []PromptVariable       `json:"variables,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CreatedBy   string                 `json:"created_by,omitempty"`
	ParentID    string                 `json:"parent_id,omitempty"` // Previous version
}

// PromptStatus represents the status of a prompt
type PromptStatus string

const (
	PromptStatusDraft     PromptStatus = "draft"
	PromptStatusActive    PromptStatus = "active"
	PromptStatusArchived  PromptStatus = "archived"
	PromptStatusTesting   PromptStatus = "testing"
)

// PromptVariable defines a variable in the prompt template
type PromptVariable struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // string, int, float, bool, list
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Description string      `json:"description,omitempty"`
	Validation  string      `json:"validation,omitempty"` // Regex pattern for validation
}

// PromptExecution records a prompt execution
type PromptExecution struct {
	ID           string                 `json:"id"`
	PromptID     string                 `json:"prompt_id"`
	PromptVersion int                   `json:"prompt_version"`
	Variables    map[string]interface{} `json:"variables"`
	RenderedText string                 `json:"rendered_text"`
	ModelUsed    string                 `json:"model_used,omitempty"`
	Response     string                 `json:"response,omitempty"`
	TokensUsed   int                    `json:"tokens_used"`
	Latency      time.Duration          `json:"latency"`
	Success      bool                   `json:"success"`
	Error        string                 `json:"error,omitempty"`
	Feedback     *PromptFeedback        `json:"feedback,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ExecutedAt   time.Time              `json:"executed_at"`
}

// PromptFeedback represents user feedback on a prompt execution
type PromptFeedback struct {
	Rating     float64 `json:"rating,omitempty"`      // 1-5
	Helpful    *bool   `json:"helpful,omitempty"`     // Thumbs up/down
	Correction string  `json:"correction,omitempty"`  // User's correction
	Comment    string  `json:"comment,omitempty"`
}

// TemplateEngine renders prompt templates
type TemplateEngine struct {
	funcMap   template.FuncMap
	leftDelim  string
	rightDelim string
}

// TemplateEngineConfig configures the template engine
type TemplateEngineConfig struct {
	FuncMap    template.FuncMap
	LeftDelim  string // Default: "{{"
	RightDelim string // Default: "}}"
}

// NewTemplateEngine creates a new template engine
func NewTemplateEngine(config TemplateEngineConfig) *TemplateEngine {
	funcMap := defaultFuncMap()
	for k, v := range config.FuncMap {
		funcMap[k] = v
	}

	leftDelim := config.LeftDelim
	if leftDelim == "" {
		leftDelim = "{{"
	}
	rightDelim := config.RightDelim
	if rightDelim == "" {
		rightDelim = "}}"
	}

	return &TemplateEngine{
		funcMap:    funcMap,
		leftDelim:  leftDelim,
		rightDelim: rightDelim,
	}
}

// Render renders a prompt template with variables
func (e *TemplateEngine) Render(prompt *PromptTemplate, variables map[string]interface{}) (string, error) {
	// Validate required variables
	if err := e.validateVariables(prompt, variables); err != nil {
		return "", err
	}

	// Apply defaults for missing optional variables
	vars := e.applyDefaults(prompt, variables)

	// Parse and execute template
	tmpl, err := template.New(prompt.Name).
		Funcs(e.funcMap).
		Delims(e.leftDelim, e.rightDelim).
		Parse(prompt.Content)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// validateVariables validates that required variables are present
func (e *TemplateEngine) validateVariables(prompt *PromptTemplate, variables map[string]interface{}) error {
	for _, v := range prompt.Variables {
		if v.Required {
			if _, ok := variables[v.Name]; !ok {
				return fmt.Errorf("missing required variable: %s", v.Name)
			}
		}
	}
	return nil
}

// applyDefaults applies default values for missing optional variables
func (e *TemplateEngine) applyDefaults(prompt *PromptTemplate, variables map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range variables {
		result[k] = v
	}

	for _, v := range prompt.Variables {
		if _, ok := result[v.Name]; !ok && v.Default != nil {
			result[v.Name] = v.Default
		}
	}

	return result
}

// defaultFuncMap returns the default template functions
func defaultFuncMap() template.FuncMap {
	return template.FuncMap{
		"upper":    toUpper,
		"lower":    toLower,
		"title":    toTitle,
		"join":     joinStrings,
		"default":  defaultValue,
		"truncate": truncateString,
		"trim":     trimString,
	}
}

// Helper functions for templates
func toUpper(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func toTitle(s string) string {
	if len(s) == 0 {
		return s
	}
	result := make([]byte, len(s))
	result[0] = s[0]
	if result[0] >= 'a' && result[0] <= 'z' {
		result[0] -= 'a' - 'A'
	}
	for i := 1; i < len(s); i++ {
		result[i] = s[i]
	}
	return string(result)
}

func joinStrings(sep string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	result := items[0]
	for i := 1; i < len(items); i++ {
		result += sep + items[i]
	}
	return result
}

func defaultValue(def, val interface{}) interface{} {
	if val == nil || val == "" {
		return def
	}
	return val
}

func truncateString(length int, s string) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}

func trimString(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

// PromptStore stores and manages prompt templates
type PromptStore interface {
	// Create creates a new prompt template
	Create(ctx context.Context, prompt *PromptTemplate) error

	// Get retrieves a prompt by ID
	Get(ctx context.Context, id string) (*PromptTemplate, error)

	// GetByName retrieves the active version of a prompt by name
	GetByName(ctx context.Context, name string) (*PromptTemplate, error)

	// GetVersion retrieves a specific version of a prompt
	GetVersion(ctx context.Context, name string, version int) (*PromptTemplate, error)

	// List lists prompts matching a filter
	List(ctx context.Context, filter *PromptFilter) ([]*PromptTemplate, error)

	// Update updates a prompt (creates new version)
	Update(ctx context.Context, prompt *PromptTemplate) error

	// Delete archives a prompt
	Delete(ctx context.Context, id string) error

	// GetHistory gets version history for a prompt
	GetHistory(ctx context.Context, name string) ([]*PromptTemplate, error)

	// RecordExecution records a prompt execution
	RecordExecution(ctx context.Context, execution *PromptExecution) error

	// GetExecutions gets executions for a prompt
	GetExecutions(ctx context.Context, promptID string, limit int) ([]*PromptExecution, error)
}

// PromptFilter filters prompt queries
type PromptFilter struct {
	Name     string
	Status   PromptStatus
	Tags     []string
	Limit    int
	Offset   int
}

// MemoryPromptStore is an in-memory prompt store implementation
type MemoryPromptStore struct {
	prompts    map[string]*PromptTemplate
	byName     map[string][]string // name -> list of IDs (versions)
	executions map[string][]*PromptExecution
	mu         sync.RWMutex
}

// NewMemoryPromptStore creates a new in-memory prompt store
func NewMemoryPromptStore() *MemoryPromptStore {
	return &MemoryPromptStore{
		prompts:    make(map[string]*PromptTemplate),
		byName:     make(map[string][]string),
		executions: make(map[string][]*PromptExecution),
	}
}

func (s *MemoryPromptStore) Create(ctx context.Context, prompt *PromptTemplate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if prompt.ID == "" {
		prompt.ID = uuid.New().String()
	}
	if prompt.CreatedAt.IsZero() {
		prompt.CreatedAt = time.Now()
	}
	prompt.UpdatedAt = prompt.CreatedAt

	s.prompts[prompt.ID] = prompt
	s.byName[prompt.Name] = append(s.byName[prompt.Name], prompt.ID)

	return nil
}

func (s *MemoryPromptStore) Get(ctx context.Context, id string) (*PromptTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prompt, ok := s.prompts[id]
	if !ok {
		return nil, errors.New("prompt not found")
	}
	return prompt, nil
}

func (s *MemoryPromptStore) GetByName(ctx context.Context, name string) (*PromptTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.byName[name]
	if len(ids) == 0 {
		return nil, errors.New("prompt not found")
	}

	// Find the active version
	for i := len(ids) - 1; i >= 0; i-- {
		if prompt := s.prompts[ids[i]]; prompt.Status == PromptStatusActive {
			return prompt, nil
		}
	}

	// Return latest if no active version
	return s.prompts[ids[len(ids)-1]], nil
}

func (s *MemoryPromptStore) GetVersion(ctx context.Context, name string, version int) (*PromptTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, id := range s.byName[name] {
		if prompt := s.prompts[id]; prompt.Version == version {
			return prompt, nil
		}
	}
	return nil, errors.New("version not found")
}

func (s *MemoryPromptStore) List(ctx context.Context, filter *PromptFilter) ([]*PromptTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]*PromptTemplate, 0)
	for _, prompt := range s.prompts {
		if filter != nil {
			if filter.Name != "" && prompt.Name != filter.Name {
				continue
			}
			if filter.Status != "" && prompt.Status != filter.Status {
				continue
			}
		}
		results = append(results, prompt)
	}

	// Apply pagination
	if filter != nil {
		if filter.Offset > 0 && filter.Offset < len(results) {
			results = results[filter.Offset:]
		}
		if filter.Limit > 0 && filter.Limit < len(results) {
			results = results[:filter.Limit]
		}
	}

	return results, nil
}

func (s *MemoryPromptStore) Update(ctx context.Context, prompt *PromptTemplate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prompt.UpdatedAt = time.Now()
	s.prompts[prompt.ID] = prompt

	return nil
}

func (s *MemoryPromptStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if prompt, ok := s.prompts[id]; ok {
		prompt.Status = PromptStatusArchived
	}
	return nil
}

func (s *MemoryPromptStore) GetHistory(ctx context.Context, name string) ([]*PromptTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.byName[name]
	results := make([]*PromptTemplate, 0, len(ids))
	for _, id := range ids {
		if prompt := s.prompts[id]; prompt != nil {
			results = append(results, prompt)
		}
	}
	return results, nil
}

func (s *MemoryPromptStore) RecordExecution(ctx context.Context, execution *PromptExecution) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if execution.ID == "" {
		execution.ID = uuid.New().String()
	}
	if execution.ExecutedAt.IsZero() {
		execution.ExecutedAt = time.Now()
	}

	s.executions[execution.PromptID] = append(s.executions[execution.PromptID], execution)
	return nil
}

func (s *MemoryPromptStore) GetExecutions(ctx context.Context, promptID string, limit int) ([]*PromptExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	execs := s.executions[promptID]
	if limit > 0 && limit < len(execs) {
		execs = execs[len(execs)-limit:]
	}
	return execs, nil
}

// Ensure MemoryPromptStore implements PromptStore
var _ PromptStore = (*MemoryPromptStore)(nil)
