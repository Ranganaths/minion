package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// NativeToolSkill is a skill that wraps a provider-agnostic tool/function.
// It works with any LLM that supports tool/function calling (OpenAI, Anthropic, Google, etc.).
type NativeToolSkill struct {
	*BaseSkill
	definition NativeToolDefinition
	handler    ToolHandler
}

// NativeToolSkillConfig is the configuration for creating a native tool skill
type NativeToolSkillConfig struct {
	Definition    NativeToolDefinition
	Handler       ToolHandler
	Scope         SkillScope
	AllowedAgents []string
	Version       string
	Author        string
	Tags          []string
	Dependencies  []string
	Source        string
}

// NewNativeToolSkill creates a new native tool skill
func NewNativeToolSkill(config NativeToolSkillConfig) (*NativeToolSkill, error) {
	if config.Definition.Name == "" {
		return nil, fmt.Errorf("tool name is required")
	}
	if config.Definition.Description == "" {
		return nil, fmt.Errorf("tool description is required")
	}
	if config.Handler == nil {
		return nil, fmt.Errorf("tool handler is required")
	}

	base := NewBaseSkill(config.Definition.Name, config.Definition.Description, SkillTypeNativeTool)

	if config.Version != "" {
		base.SetVersion(config.Version)
	}
	if config.Scope != "" {
		base.SetScope(config.Scope)
	} else {
		base.SetScope(ScopeFramework)
	}
	if len(config.AllowedAgents) > 0 {
		base.SetAllowedAgents(config.AllowedAgents)
	}
	if len(config.Dependencies) > 0 {
		base.SetDependencies(config.Dependencies)
	}

	source := config.Source
	if source == "" {
		source = "programmatic"
	}

	base.SetMetadata(SkillMetadata{
		Author:    config.Author,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Source:    source,
		Tags:      config.Tags,
	})

	return &NativeToolSkill{
		BaseSkill:  base,
		definition: config.Definition,
		handler:    config.Handler,
	}, nil
}

// NewNativeToolSkillWithoutHandler creates a native tool skill without a handler
// The handler must be set separately using SetHandler
func NewNativeToolSkillWithoutHandler(config NativeToolSkillConfig) (*NativeToolSkill, error) {
	if config.Definition.Name == "" {
		return nil, fmt.Errorf("tool name is required")
	}
	if config.Definition.Description == "" {
		return nil, fmt.Errorf("tool description is required")
	}

	base := NewBaseSkill(config.Definition.Name, config.Definition.Description, SkillTypeNativeTool)

	if config.Version != "" {
		base.SetVersion(config.Version)
	}
	if config.Scope != "" {
		base.SetScope(config.Scope)
	} else {
		base.SetScope(ScopeFramework)
	}
	if len(config.AllowedAgents) > 0 {
		base.SetAllowedAgents(config.AllowedAgents)
	}
	if len(config.Dependencies) > 0 {
		base.SetDependencies(config.Dependencies)
	}

	source := config.Source
	if source == "" {
		source = "programmatic"
	}

	base.SetMetadata(SkillMetadata{
		Author:    config.Author,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Source:    source,
		Tags:      config.Tags,
	})

	return &NativeToolSkill{
		BaseSkill:  base,
		definition: config.Definition,
		handler:    nil, // Handler must be set separately
	}, nil
}

// Execute runs the native tool skill
func (s *NativeToolSkill) Execute(ctx context.Context, input *SkillInput) (*SkillOutput, error) {
	startTime := time.Now()

	if s.handler == nil {
		return &SkillOutput{
			Success:       false,
			Error:         "tool handler not configured",
			ExecutionTime: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// Get arguments from input parameters
	args := make(map[string]interface{})
	if input != nil && input.Parameters != nil {
		args = input.Parameters
	}

	// Execute the handler
	result, err := s.handler(ctx, args)
	if err != nil {
		return &SkillOutput{
			Success:       false,
			Error:         err.Error(),
			ExecutionTime: time.Since(startTime).Milliseconds(),
			Metadata: map[string]interface{}{
				"skill_name": s.Name(),
				"skill_type": string(s.Type()),
			},
		}, nil
	}

	// Convert result to string content
	var content string
	switch v := result.(type) {
	case string:
		content = v
	case []byte:
		content = string(v)
	default:
		// JSON encode the result
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			content = fmt.Sprintf("%v", result)
		} else {
			content = string(jsonBytes)
		}
	}

	return &SkillOutput{
		Content: content,
		Success: true,
		Metadata: map[string]interface{}{
			"skill_name":    s.Name(),
			"skill_type":    string(s.Type()),
			"skill_version": s.Version(),
			"result":        result,
		},
		ExecutionTime: time.Since(startTime).Milliseconds(),
	}, nil
}

// SetHandler sets the tool handler
func (s *NativeToolSkill) SetHandler(handler ToolHandler) {
	s.handler = handler
}

// HasHandler returns true if the skill has a handler configured
func (s *NativeToolSkill) HasHandler() bool {
	return s.handler != nil
}

// Definition returns the tool definition
func (s *NativeToolSkill) Definition() NativeToolDefinition {
	return s.definition
}

// ToToolDefinition converts to an LLM tool definition format
func (s *NativeToolSkill) ToToolDefinition() map[string]interface{} {
	return map[string]interface{}{
		"name":         s.definition.Name,
		"description":  s.definition.Description,
		"input_schema": s.schemaToMap(s.definition.InputSchema),
	}
}

// schemaToMap converts JSONSchema to a map for JSON serialization
func (s *NativeToolSkill) schemaToMap(schema JSONSchema) map[string]interface{} {
	result := map[string]interface{}{
		"type": schema.Type,
	}

	if schema.Description != "" {
		result["description"] = schema.Description
	}

	if len(schema.Properties) > 0 {
		props := make(map[string]interface{})
		for name, prop := range schema.Properties {
			props[name] = s.propertyToMap(prop)
		}
		result["properties"] = props
	}

	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}

	if schema.AdditionalProperties != nil {
		result["additionalProperties"] = *schema.AdditionalProperties
	}

	return result
}

// propertyToMap converts PropertySchema to a map
func (s *NativeToolSkill) propertyToMap(prop PropertySchema) map[string]interface{} {
	result := map[string]interface{}{
		"type": prop.Type,
	}

	if prop.Description != "" {
		result["description"] = prop.Description
	}

	if len(prop.Enum) > 0 {
		result["enum"] = prop.Enum
	}

	if prop.Default != nil {
		result["default"] = prop.Default
	}

	if prop.Items != nil {
		result["items"] = s.propertyToMap(*prop.Items)
	}

	if len(prop.Properties) > 0 {
		props := make(map[string]interface{})
		for name, p := range prop.Properties {
			props[name] = s.propertyToMap(p)
		}
		result["properties"] = props
	}

	if len(prop.Required) > 0 {
		result["required"] = prop.Required
	}

	if prop.Minimum != nil {
		result["minimum"] = *prop.Minimum
	}

	if prop.Maximum != nil {
		result["maximum"] = *prop.Maximum
	}

	if prop.MinLength != nil {
		result["minLength"] = *prop.MinLength
	}

	if prop.MaxLength != nil {
		result["maxLength"] = *prop.MaxLength
	}

	if prop.Pattern != "" {
		result["pattern"] = prop.Pattern
	}

	return result
}

// NativeToolSkillBuilder is a builder for native tool skills
type NativeToolSkillBuilder struct {
	config NativeToolSkillConfig
}

// NewNativeToolSkillBuilder creates a new native tool skill builder
func NewNativeToolSkillBuilder(name string) *NativeToolSkillBuilder {
	return &NativeToolSkillBuilder{
		config: NativeToolSkillConfig{
			Definition: NativeToolDefinition{
				Name: name,
				InputSchema: JSONSchema{
					Type:       "object",
					Properties: make(map[string]PropertySchema),
				},
			},
			Scope:   ScopeFramework,
			Version: "1.0.0",
		},
	}
}

// WithDescription sets the tool description
func (b *NativeToolSkillBuilder) WithDescription(desc string) *NativeToolSkillBuilder {
	b.config.Definition.Description = desc
	return b
}

// WithHandler sets the tool handler
func (b *NativeToolSkillBuilder) WithHandler(handler ToolHandler) *NativeToolSkillBuilder {
	b.config.Handler = handler
	return b
}

// WithScope sets the skill scope
func (b *NativeToolSkillBuilder) WithScope(scope SkillScope) *NativeToolSkillBuilder {
	b.config.Scope = scope
	return b
}

// WithAllowedAgents sets the allowed agents
func (b *NativeToolSkillBuilder) WithAllowedAgents(agents []string) *NativeToolSkillBuilder {
	b.config.AllowedAgents = agents
	return b
}

// WithVersion sets the skill version
func (b *NativeToolSkillBuilder) WithVersion(version string) *NativeToolSkillBuilder {
	b.config.Version = version
	return b
}

// WithAuthor sets the skill author
func (b *NativeToolSkillBuilder) WithAuthor(author string) *NativeToolSkillBuilder {
	b.config.Author = author
	return b
}

// WithTags sets the skill tags
func (b *NativeToolSkillBuilder) WithTags(tags []string) *NativeToolSkillBuilder {
	b.config.Tags = tags
	return b
}

// WithDependencies sets the skill dependencies
func (b *NativeToolSkillBuilder) WithDependencies(deps []string) *NativeToolSkillBuilder {
	b.config.Dependencies = deps
	return b
}

// AddProperty adds a property to the input schema
func (b *NativeToolSkillBuilder) AddProperty(name string, prop PropertySchema) *NativeToolSkillBuilder {
	b.config.Definition.InputSchema.Properties[name] = prop
	return b
}

// AddStringProperty adds a string property
func (b *NativeToolSkillBuilder) AddStringProperty(name, description string, required bool) *NativeToolSkillBuilder {
	b.config.Definition.InputSchema.Properties[name] = PropertySchema{
		Type:        "string",
		Description: description,
	}
	if required {
		b.config.Definition.InputSchema.Required = append(b.config.Definition.InputSchema.Required, name)
	}
	return b
}

// AddIntegerProperty adds an integer property
func (b *NativeToolSkillBuilder) AddIntegerProperty(name, description string, required bool) *NativeToolSkillBuilder {
	b.config.Definition.InputSchema.Properties[name] = PropertySchema{
		Type:        "integer",
		Description: description,
	}
	if required {
		b.config.Definition.InputSchema.Required = append(b.config.Definition.InputSchema.Required, name)
	}
	return b
}

// AddNumberProperty adds a number property
func (b *NativeToolSkillBuilder) AddNumberProperty(name, description string, required bool) *NativeToolSkillBuilder {
	b.config.Definition.InputSchema.Properties[name] = PropertySchema{
		Type:        "number",
		Description: description,
	}
	if required {
		b.config.Definition.InputSchema.Required = append(b.config.Definition.InputSchema.Required, name)
	}
	return b
}

// AddBooleanProperty adds a boolean property
func (b *NativeToolSkillBuilder) AddBooleanProperty(name, description string, required bool) *NativeToolSkillBuilder {
	b.config.Definition.InputSchema.Properties[name] = PropertySchema{
		Type:        "boolean",
		Description: description,
	}
	if required {
		b.config.Definition.InputSchema.Required = append(b.config.Definition.InputSchema.Required, name)
	}
	return b
}

// AddEnumProperty adds an enum property
func (b *NativeToolSkillBuilder) AddEnumProperty(name, description string, values []string, required bool) *NativeToolSkillBuilder {
	b.config.Definition.InputSchema.Properties[name] = PropertySchema{
		Type:        "string",
		Description: description,
		Enum:        values,
	}
	if required {
		b.config.Definition.InputSchema.Required = append(b.config.Definition.InputSchema.Required, name)
	}
	return b
}

// AddArrayProperty adds an array property
func (b *NativeToolSkillBuilder) AddArrayProperty(name, description string, itemType string, required bool) *NativeToolSkillBuilder {
	b.config.Definition.InputSchema.Properties[name] = PropertySchema{
		Type:        "array",
		Description: description,
		Items: &PropertySchema{
			Type: itemType,
		},
	}
	if required {
		b.config.Definition.InputSchema.Required = append(b.config.Definition.InputSchema.Required, name)
	}
	return b
}

// Build creates the native tool skill
func (b *NativeToolSkillBuilder) Build() (*NativeToolSkill, error) {
	return NewNativeToolSkill(b.config)
}

// BuildWithoutHandler creates the native tool skill without a handler
func (b *NativeToolSkillBuilder) BuildWithoutHandler() (*NativeToolSkill, error) {
	return NewNativeToolSkillWithoutHandler(b.config)
}

// ToolResult represents the result of a tool call
type ToolResult struct {
	ToolUseID string      `json:"tool_use_id"`
	Content   interface{} `json:"content"`
	IsError   bool        `json:"is_error,omitempty"`
}

// NewToolResult creates a new tool result
func NewToolResult(toolUseID string, content interface{}, isError bool) *ToolResult {
	return &ToolResult{
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	}
}
