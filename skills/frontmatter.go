package skills

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// FrontmatterDelimiter is the delimiter used to separate frontmatter from content
const FrontmatterDelimiter = "---"

// ErrNoFrontmatter indicates that no frontmatter was found
var ErrNoFrontmatter = errors.New("no frontmatter found")

// ErrInvalidFrontmatter indicates that the frontmatter is invalid
var ErrInvalidFrontmatter = errors.New("invalid frontmatter")

// SkillFrontmatter represents the YAML frontmatter of a skill file
type SkillFrontmatter struct {
	// Required fields
	Name        string `yaml:"name"`
	Description string `yaml:"description"`

	// Optional fields with defaults
	Version string     `yaml:"version,omitempty"`
	Type    SkillType  `yaml:"type,omitempty"`
	Scope   SkillScope `yaml:"scope,omitempty"`

	// Agent scope configuration
	AllowedAgents []string `yaml:"allowed_agents,omitempty"`

	// Metadata
	Author       string   `yaml:"author,omitempty"`
	Tags         []string `yaml:"tags,omitempty"`
	Dependencies []string `yaml:"dependencies,omitempty"`

	// Native tool definition (for native_tool type)
	ToolDefinition *ToolDefinitionFrontmatter `yaml:"tool_definition,omitempty"`

	// Execution configuration
	Config *SkillConfigFrontmatter `yaml:"config,omitempty"`
}

// ToolDefinitionFrontmatter represents the tool_definition section of frontmatter
type ToolDefinitionFrontmatter struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	InputSchema map[string]interface{} `yaml:"input_schema"`
}

// SkillConfigFrontmatter represents the config section of frontmatter
type SkillConfigFrontmatter struct {
	Temperature     float64                `yaml:"temperature,omitempty"`
	MaxTokens       int                    `yaml:"max_tokens,omitempty"`
	ModelPreference string                 `yaml:"model_preference,omitempty"`
	Custom          map[string]interface{} `yaml:"custom,omitempty"`
}

// ParsedSkillFile represents a fully parsed skill file
type ParsedSkillFile struct {
	Frontmatter *SkillFrontmatter
	Content     string
	Source      string
}

// FrontmatterParser parses YAML frontmatter from markdown files
type FrontmatterParser struct{}

// NewFrontmatterParser creates a new frontmatter parser
func NewFrontmatterParser() *FrontmatterParser {
	return &FrontmatterParser{}
}

// Parse extracts and parses frontmatter from content
func (p *FrontmatterParser) Parse(content []byte, source string) (*ParsedSkillFile, error) {
	// Find frontmatter boundaries
	frontmatter, body, err := p.extractFrontmatter(content)
	if err != nil {
		return nil, err
	}

	// Parse YAML frontmatter
	var fm SkillFrontmatter
	if err := yaml.Unmarshal(frontmatter, &fm); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFrontmatter, err)
	}

	// Apply defaults
	p.applyDefaults(&fm)

	// Validate required fields
	if err := p.validateFrontmatter(&fm); err != nil {
		return nil, err
	}

	return &ParsedSkillFile{
		Frontmatter: &fm,
		Content:     strings.TrimSpace(string(body)),
		Source:      source,
	}, nil
}

// extractFrontmatter extracts frontmatter and body from content
func (p *FrontmatterParser) extractFrontmatter(content []byte) (frontmatter, body []byte, err error) {
	content = bytes.TrimSpace(content)

	// Check if content starts with frontmatter delimiter
	if !bytes.HasPrefix(content, []byte(FrontmatterDelimiter)) {
		return nil, nil, ErrNoFrontmatter
	}

	// Find the end of the frontmatter
	rest := content[len(FrontmatterDelimiter):]

	// Skip any whitespace/newline after the opening delimiter
	rest = bytes.TrimLeft(rest, " \t")
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	// Find the closing delimiter
	endIndex := bytes.Index(rest, []byte("\n"+FrontmatterDelimiter))
	if endIndex == -1 {
		// Try with \r\n
		endIndex = bytes.Index(rest, []byte("\r\n"+FrontmatterDelimiter))
		if endIndex == -1 {
			return nil, nil, fmt.Errorf("%w: missing closing delimiter", ErrNoFrontmatter)
		}
	}

	frontmatter = rest[:endIndex]

	// Get the body after the closing delimiter
	remaining := rest[endIndex:]
	// Skip the newline and delimiter
	for i := 0; i < len(remaining); i++ {
		if remaining[i] == '-' {
			// Found the start of the delimiter, skip it
			remaining = remaining[i+len(FrontmatterDelimiter):]
			break
		}
	}
	body = bytes.TrimSpace(remaining)

	return frontmatter, body, nil
}

// applyDefaults applies default values to frontmatter
func (p *FrontmatterParser) applyDefaults(fm *SkillFrontmatter) {
	if fm.Version == "" {
		fm.Version = "1.0.0"
	}
	if fm.Type == "" {
		fm.Type = SkillTypeMarkdown
	}
	if fm.Scope == "" {
		fm.Scope = ScopeFramework
	}
}

// validateFrontmatter validates required frontmatter fields
func (p *FrontmatterParser) validateFrontmatter(fm *SkillFrontmatter) error {
	if fm.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidFrontmatter)
	}
	if fm.Description == "" {
		return fmt.Errorf("%w: description is required", ErrInvalidFrontmatter)
	}

	// Validate type
	switch fm.Type {
	case SkillTypeMarkdown, SkillTypeNativeTool, SkillTypeBehavior:
		// Valid
	default:
		return fmt.Errorf("%w: invalid type '%s'", ErrInvalidFrontmatter, fm.Type)
	}

	// Validate scope
	switch fm.Scope {
	case ScopeFramework, ScopeAgent:
		// Valid
	default:
		return fmt.Errorf("%w: invalid scope '%s'", ErrInvalidFrontmatter, fm.Scope)
	}

	// Validate agent scope has allowed agents
	if fm.Scope == ScopeAgent && len(fm.AllowedAgents) == 0 {
		return fmt.Errorf("%w: agent scope requires allowed_agents", ErrInvalidFrontmatter)
	}

	// Validate native_tool has tool_definition
	if fm.Type == SkillTypeNativeTool && fm.ToolDefinition == nil {
		return fmt.Errorf("%w: native_tool type requires tool_definition", ErrInvalidFrontmatter)
	}

	return nil
}

// ToSkillMetadata converts frontmatter to SkillMetadata
func (p *FrontmatterParser) ToSkillMetadata(fm *SkillFrontmatter, source string) SkillMetadata {
	return SkillMetadata{
		Author:    fm.Author,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Source:    source,
		Tags:      fm.Tags,
	}
}

// ToSkillConfig converts frontmatter config to SkillConfig
func (p *FrontmatterParser) ToSkillConfig(fc *SkillConfigFrontmatter) SkillConfig {
	if fc == nil {
		return SkillConfig{}
	}

	return SkillConfig{
		Temperature:     fc.Temperature,
		MaxTokens:       fc.MaxTokens,
		ModelPreference: fc.ModelPreference,
		Custom:          fc.Custom,
	}
}

// ToNativeToolDefinition converts frontmatter tool definition to NativeToolDefinition
func (p *FrontmatterParser) ToNativeToolDefinition(td *ToolDefinitionFrontmatter) (*NativeToolDefinition, error) {
	if td == nil {
		return nil, nil
	}

	schema, err := p.parseInputSchema(td.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input_schema: %w", err)
	}

	return &NativeToolDefinition{
		Name:        td.Name,
		Description: td.Description,
		InputSchema: *schema,
	}, nil
}

// parseInputSchema parses the input_schema map into a JSONSchema
func (p *FrontmatterParser) parseInputSchema(schemaMap map[string]interface{}) (*JSONSchema, error) {
	schema := &JSONSchema{
		Type:       "object",
		Properties: make(map[string]PropertySchema),
	}

	// Get type
	if t, ok := schemaMap["type"].(string); ok {
		schema.Type = t
	}

	// Get description
	if d, ok := schemaMap["description"].(string); ok {
		schema.Description = d
	}

	// Get required
	if req, ok := schemaMap["required"].([]interface{}); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}

	// Get properties
	if props, ok := schemaMap["properties"].(map[string]interface{}); ok {
		for name, propValue := range props {
			propMap, ok := propValue.(map[string]interface{})
			if !ok {
				continue
			}
			prop, err := p.parsePropertySchema(propMap)
			if err != nil {
				return nil, fmt.Errorf("failed to parse property '%s': %w", name, err)
			}
			schema.Properties[name] = *prop
		}
	}

	return schema, nil
}

// parsePropertySchema parses a property schema from a map
func (p *FrontmatterParser) parsePropertySchema(propMap map[string]interface{}) (*PropertySchema, error) {
	prop := &PropertySchema{}

	// Type
	if t, ok := propMap["type"].(string); ok {
		prop.Type = t
	}

	// Description
	if d, ok := propMap["description"].(string); ok {
		prop.Description = d
	}

	// Enum
	if enum, ok := propMap["enum"].([]interface{}); ok {
		for _, e := range enum {
			if s, ok := e.(string); ok {
				prop.Enum = append(prop.Enum, s)
			}
		}
	}

	// Default
	if d, ok := propMap["default"]; ok {
		prop.Default = d
	}

	// Items (for arrays)
	if items, ok := propMap["items"].(map[string]interface{}); ok {
		itemsProp, err := p.parsePropertySchema(items)
		if err != nil {
			return nil, err
		}
		prop.Items = itemsProp
	}

	// Properties (for objects)
	if props, ok := propMap["properties"].(map[string]interface{}); ok {
		prop.Properties = make(map[string]PropertySchema)
		for name, propValue := range props {
			nestedMap, ok := propValue.(map[string]interface{})
			if !ok {
				continue
			}
			nestedProp, err := p.parsePropertySchema(nestedMap)
			if err != nil {
				return nil, err
			}
			prop.Properties[name] = *nestedProp
		}
	}

	// Required (for objects)
	if req, ok := propMap["required"].([]interface{}); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				prop.Required = append(prop.Required, s)
			}
		}
	}

	// Numeric constraints
	if min, ok := propMap["minimum"].(float64); ok {
		prop.Minimum = &min
	}
	if max, ok := propMap["maximum"].(float64); ok {
		prop.Maximum = &max
	}

	// String constraints
	if minLen, ok := propMap["minLength"].(int); ok {
		prop.MinLength = &minLen
	}
	if maxLen, ok := propMap["maxLength"].(int); ok {
		prop.MaxLength = &maxLen
	}
	if pattern, ok := propMap["pattern"].(string); ok {
		prop.Pattern = pattern
	}

	return prop, nil
}

// SerializeFrontmatter serializes a skill's frontmatter to YAML
func (p *FrontmatterParser) SerializeFrontmatter(skill Skill) ([]byte, error) {
	fm := SkillFrontmatter{
		Name:          skill.Name(),
		Description:   skill.Description(),
		Version:       skill.Version(),
		Type:          skill.Type(),
		Scope:         skill.Scope(),
		AllowedAgents: skill.AllowedAgents(),
		Author:        skill.Metadata().Author,
		Tags:          skill.Metadata().Tags,
		Dependencies:  skill.Dependencies(),
	}

	// Add config if available - check for types that embed BaseSkill
	if configGetter, ok := skill.(interface{ Config() SkillConfig }); ok {
		config := configGetter.Config()
		if config.Temperature != 0 || config.MaxTokens != 0 || config.ModelPreference != "" {
			fm.Config = &SkillConfigFrontmatter{
				Temperature:     config.Temperature,
				MaxTokens:       config.MaxTokens,
				ModelPreference: config.ModelPreference,
				Custom:          config.Custom,
			}
		}
	}

	return yaml.Marshal(fm)
}

// FormatSkillFile formats a complete skill file with frontmatter and content
func (p *FrontmatterParser) FormatSkillFile(skill Skill, content string) ([]byte, error) {
	fm, err := p.SerializeFrontmatter(skill)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteString(FrontmatterDelimiter)
	buf.WriteString("\n")
	buf.Write(fm)
	buf.WriteString(FrontmatterDelimiter)
	buf.WriteString("\n\n")
	buf.WriteString(content)
	buf.WriteString("\n")

	return buf.Bytes(), nil
}
