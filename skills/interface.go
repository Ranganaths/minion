// Package skills provides a comprehensive skill system for the Minion framework.
// It supports markdown-based skills, native LLM tool/function calling, and a central registry
// with dynamic loading and scope management. The skill system is provider-agnostic and works
// with any LLM that supports tool/function calling (OpenAI, Anthropic, Google, etc.).
package skills

import (
	"context"
	"embed"
	"time"

	"github.com/Ranganaths/minion/models"
)

// SkillType represents the type of skill
type SkillType string

const (
	// SkillTypeMarkdown is a markdown-based instruction skill
	SkillTypeMarkdown SkillType = "markdown"
	// SkillTypeNativeTool is a native LLM tool/function calling skill
	// Compatible with OpenAI functions, Anthropic tools, Google tools, etc.
	SkillTypeNativeTool SkillType = "native_tool"
	// SkillTypeBehavior is a behavior-enhancing skill
	SkillTypeBehavior SkillType = "behavior"
)

// SkillScope defines where a skill is available
type SkillScope string

const (
	// ScopeFramework makes the skill available to all agents
	ScopeFramework SkillScope = "framework"
	// ScopeAgent makes the skill available only to specific agents
	ScopeAgent SkillScope = "agent"
)

// Skill is the core interface that all skills must implement
type Skill interface {
	// Name returns the unique identifier for this skill
	Name() string

	// Description returns a human-readable description of what this skill does
	Description() string

	// Type returns the skill type (markdown, native_tool, or behavior)
	Type() SkillType

	// Version returns the semantic version of this skill
	Version() string

	// Scope returns whether this skill is framework-wide or agent-specific
	Scope() SkillScope

	// AllowedAgents returns the list of agent IDs that can use this skill
	// Empty slice means all agents (for framework scope)
	AllowedAgents() []string

	// CanExecute checks if the given agent is allowed to execute this skill
	CanExecute(agent *models.Agent) bool

	// Execute runs the skill with the given input
	Execute(ctx context.Context, input *SkillInput) (*SkillOutput, error)

	// Metadata returns additional metadata about this skill
	Metadata() SkillMetadata

	// Dependencies returns the names of skills this skill depends on
	Dependencies() []string
}

// SkillInput represents input to a skill
type SkillInput struct {
	// Query is the user's query or prompt
	Query string `json:"query"`

	// Parameters are structured parameters for the skill
	Parameters map[string]interface{} `json:"parameters"`

	// Context provides execution context information
	Context map[string]interface{} `json:"context"`

	// Agent is the agent executing this skill (can be nil for framework-level execution)
	Agent *models.Agent `json:"agent,omitempty"`

	// DependencyResults contains results from dependency skill executions
	DependencyResults map[string]*SkillOutput `json:"dependency_results,omitempty"`
}

// SkillOutput represents output from a skill
type SkillOutput struct {
	// Content is the main text content output
	Content string `json:"content"`

	// ToolCalls contains any tool calls requested by the skill (for native_tool type)
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// Prompt is an enhanced prompt (for markdown skills that enhance prompts)
	Prompt string `json:"prompt,omitempty"`

	// Instructions are execution instructions for the LLM
	Instructions string `json:"instructions,omitempty"`

	// Metadata contains additional output metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Success indicates whether the skill executed successfully
	Success bool `json:"success"`

	// Error contains error message if Success is false
	Error string `json:"error,omitempty"`

	// ExecutionTime is the time taken to execute in milliseconds
	ExecutionTime int64 `json:"execution_time_ms"`
}

// ToolCall represents a native tool/function call request.
// This is a provider-agnostic format that works with any LLM's tool calling capability.
type ToolCall struct {
	// ID is a unique identifier for this tool call
	ID string `json:"id"`

	// Name is the name of the tool to call
	Name string `json:"name"`

	// Arguments are the arguments to pass to the tool
	Arguments map[string]interface{} `json:"arguments"`
}

// SkillMetadata contains metadata about a skill
type SkillMetadata struct {
	// Author is the skill author
	Author string `json:"author,omitempty"`

	// CreatedAt is when the skill was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the skill was last updated
	UpdatedAt time.Time `json:"updated_at"`

	// Source is where the skill was loaded from (file path, URL, or "embedded")
	Source string `json:"source"`

	// Tags are searchable tags for the skill
	Tags []string `json:"tags,omitempty"`

	// Custom contains any custom metadata
	Custom map[string]string `json:"custom,omitempty"`
}

// SkillInfo provides summary information about a skill
type SkillInfo struct {
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Type         SkillType  `json:"type"`
	Scope        SkillScope `json:"scope"`
	Version      string     `json:"version"`
	Source       string     `json:"source"`
	Dependencies []string   `json:"dependencies,omitempty"`
	Tags         []string   `json:"tags,omitempty"`
}

// Registry manages skill registration and retrieval
type Registry interface {
	// Register adds a skill to the registry
	Register(skill Skill) error

	// RegisterFromFile loads and registers a skill from a file path
	RegisterFromFile(path string) error

	// RegisterFromURL loads and registers a skill from a URL
	RegisterFromURL(url string) error

	// RegisterFromEmbed loads and registers skills from an embedded filesystem
	RegisterFromEmbed(fs embed.FS, pattern string) (int, error)

	// Unregister removes a skill from the registry
	Unregister(name string) error

	// Get retrieves a skill by name
	Get(name string) (Skill, error)

	// GetByType returns all skills of a specific type
	GetByType(skillType SkillType) []Skill

	// GetForAgent returns all skills available to an agent
	GetForAgent(agent *models.Agent) []Skill

	// GetForScope returns all skills with a specific scope
	GetForScope(scope SkillScope) []Skill

	// List returns info about all registered skills
	List() []SkillInfo

	// Count returns the number of registered skills
	Count() int

	// LoadDirectory loads all skills from a directory
	LoadDirectory(path string) (int, error)

	// WatchDirectory starts watching a directory for skill changes (hot-reload)
	WatchDirectory(path string) error

	// StopWatching stops all directory watchers
	StopWatching() error

	// Validate validates a skill without registering it
	Validate(skill Skill) error

	// GetDependencies returns all dependencies for a skill (resolved recursively)
	GetDependencies(name string) ([]Skill, error)
}

// NativeToolDefinition represents a provider-agnostic tool definition.
// This format can be converted to provider-specific formats (OpenAI functions,
// Anthropic tools, Google function declarations, etc.).
type NativeToolDefinition struct {
	// Name is the tool name (must be unique)
	Name string `json:"name"`

	// Description describes what the tool does
	Description string `json:"description"`

	// InputSchema is the JSON Schema for tool parameters
	InputSchema JSONSchema `json:"input_schema"`
}

// JSONSchema represents a JSON Schema for tool parameters
type JSONSchema struct {
	// Type is the schema type (always "object" for tools)
	Type string `json:"type"`

	// Properties defines the parameter properties
	Properties map[string]PropertySchema `json:"properties,omitempty"`

	// Required lists required property names
	Required []string `json:"required,omitempty"`

	// Description is an optional schema description
	Description string `json:"description,omitempty"`

	// AdditionalProperties controls whether extra properties are allowed
	AdditionalProperties *bool `json:"additionalProperties,omitempty"`
}

// PropertySchema defines a single property in a JSON Schema
type PropertySchema struct {
	// Type is the property type (string, number, integer, boolean, array, object)
	Type string `json:"type"`

	// Description describes the property
	Description string `json:"description,omitempty"`

	// Enum lists allowed values (for string types)
	Enum []string `json:"enum,omitempty"`

	// Default is the default value
	Default interface{} `json:"default,omitempty"`

	// Items defines the schema for array items (for array types)
	Items *PropertySchema `json:"items,omitempty"`

	// Properties defines nested properties (for object types)
	Properties map[string]PropertySchema `json:"properties,omitempty"`

	// Required lists required nested properties (for object types)
	Required []string `json:"required,omitempty"`

	// Minimum is the minimum value (for number/integer types)
	Minimum *float64 `json:"minimum,omitempty"`

	// Maximum is the maximum value (for number/integer types)
	Maximum *float64 `json:"maximum,omitempty"`

	// MinLength is the minimum string length
	MinLength *int `json:"minLength,omitempty"`

	// MaxLength is the maximum string length
	MaxLength *int `json:"maxLength,omitempty"`

	// Pattern is a regex pattern (for string types)
	Pattern string `json:"pattern,omitempty"`
}

// ToolHandler is the function signature for handling native tool calls
type ToolHandler func(ctx context.Context, arguments map[string]interface{}) (interface{}, error)

// SkillConfig contains optional configuration for skill execution
type SkillConfig struct {
	// Temperature for LLM calls (0.0 - 2.0)
	Temperature float64 `json:"temperature,omitempty"`

	// MaxTokens is the maximum tokens for LLM responses
	MaxTokens int `json:"max_tokens,omitempty"`

	// ModelPreference is the preferred model to use
	ModelPreference string `json:"model_preference,omitempty"`

	// Custom contains any custom configuration
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// BaseSkill provides common functionality for skill implementations
type BaseSkill struct {
	name          string
	description   string
	skillType     SkillType
	version       string
	scope         SkillScope
	allowedAgents []string
	metadata      SkillMetadata
	dependencies  []string
	config        SkillConfig
}

// NewBaseSkill creates a new BaseSkill with the given configuration
func NewBaseSkill(name, description string, skillType SkillType) *BaseSkill {
	return &BaseSkill{
		name:          name,
		description:   description,
		skillType:     skillType,
		version:       "1.0.0",
		scope:         ScopeFramework,
		allowedAgents: []string{},
		metadata: SkillMetadata{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Source:    "programmatic",
		},
		dependencies: []string{},
	}
}

// Name returns the skill name
func (s *BaseSkill) Name() string {
	return s.name
}

// Description returns the skill description
func (s *BaseSkill) Description() string {
	return s.description
}

// Type returns the skill type
func (s *BaseSkill) Type() SkillType {
	return s.skillType
}

// Version returns the skill version
func (s *BaseSkill) Version() string {
	return s.version
}

// Scope returns the skill scope
func (s *BaseSkill) Scope() SkillScope {
	return s.scope
}

// AllowedAgents returns the list of allowed agent IDs
func (s *BaseSkill) AllowedAgents() []string {
	return s.allowedAgents
}

// Metadata returns the skill metadata
func (s *BaseSkill) Metadata() SkillMetadata {
	return s.metadata
}

// Dependencies returns the skill dependencies
func (s *BaseSkill) Dependencies() []string {
	return s.dependencies
}

// Config returns the skill configuration
func (s *BaseSkill) Config() SkillConfig {
	return s.config
}

// CanExecute checks if the given agent can execute this skill
func (s *BaseSkill) CanExecute(agent *models.Agent) bool {
	// Framework scope: all agents can execute
	if s.scope == ScopeFramework {
		return true
	}

	// Agent scope: check if agent is in allowed list
	if agent == nil {
		return false
	}

	for _, allowedID := range s.allowedAgents {
		if agent.ID == allowedID {
			return true
		}
	}

	return false
}

// SetVersion sets the skill version
func (s *BaseSkill) SetVersion(version string) {
	s.version = version
}

// SetScope sets the skill scope
func (s *BaseSkill) SetScope(scope SkillScope) {
	s.scope = scope
}

// SetAllowedAgents sets the allowed agent IDs
func (s *BaseSkill) SetAllowedAgents(agents []string) {
	s.allowedAgents = agents
}

// SetMetadata sets the skill metadata
func (s *BaseSkill) SetMetadata(metadata SkillMetadata) {
	s.metadata = metadata
}

// SetDependencies sets the skill dependencies
func (s *BaseSkill) SetDependencies(deps []string) {
	s.dependencies = deps
}

// SetConfig sets the skill configuration
func (s *BaseSkill) SetConfig(config SkillConfig) {
	s.config = config
}
