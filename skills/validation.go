package skills

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationError represents a skill validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []*ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}

	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// SkillValidator validates skills before registration
type SkillValidator struct {
	namePattern    *regexp.Regexp
	versionPattern *regexp.Regexp
}

// NewSkillValidator creates a new skill validator
func NewSkillValidator() *SkillValidator {
	return &SkillValidator{
		// Skill names: lowercase letters, numbers, hyphens, underscores
		namePattern: regexp.MustCompile(`^[a-z][a-z0-9_-]*$`),
		// Semantic versioning pattern
		versionPattern: regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`),
	}
}

// Validate validates a skill and returns any validation errors
func (v *SkillValidator) Validate(skill Skill) error {
	if skill == nil {
		return &ValidationError{Field: "skill", Message: "skill cannot be nil"}
	}

	var errors ValidationErrors

	// Validate name
	if err := v.validateName(skill.Name()); err != nil {
		errors = append(errors, err)
	}

	// Validate description
	if err := v.validateDescription(skill.Description()); err != nil {
		errors = append(errors, err)
	}

	// Validate type
	if err := v.validateType(skill.Type()); err != nil {
		errors = append(errors, err)
	}

	// Validate version
	if err := v.validateVersion(skill.Version()); err != nil {
		errors = append(errors, err)
	}

	// Validate scope
	if err := v.validateScope(skill.Scope()); err != nil {
		errors = append(errors, err)
	}

	// Validate agent scope consistency
	if err := v.validateScopeConsistency(skill); err != nil {
		errors = append(errors, err)
	}

	// Validate dependencies
	if errs := v.validateDependencies(skill.Dependencies()); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Validate metadata
	if errs := v.validateMetadata(skill.Metadata()); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateName validates the skill name
func (v *SkillValidator) validateName(name string) *ValidationError {
	if name == "" {
		return &ValidationError{Field: "name", Message: "name cannot be empty"}
	}

	if len(name) > 64 {
		return &ValidationError{Field: "name", Message: "name cannot exceed 64 characters"}
	}

	if !v.namePattern.MatchString(name) {
		return &ValidationError{
			Field:   "name",
			Message: "name must start with a lowercase letter and contain only lowercase letters, numbers, hyphens, and underscores",
		}
	}

	return nil
}

// validateDescription validates the skill description
func (v *SkillValidator) validateDescription(desc string) *ValidationError {
	if desc == "" {
		return &ValidationError{Field: "description", Message: "description cannot be empty"}
	}

	if len(desc) > 1024 {
		return &ValidationError{Field: "description", Message: "description cannot exceed 1024 characters"}
	}

	return nil
}

// validateType validates the skill type
func (v *SkillValidator) validateType(skillType SkillType) *ValidationError {
	switch skillType {
	case SkillTypeMarkdown, SkillTypeNativeTool, SkillTypeBehavior:
		return nil
	default:
		return &ValidationError{
			Field:   "type",
			Message: fmt.Sprintf("invalid skill type: %s (must be one of: markdown, native_tool, behavior)", skillType),
		}
	}
}

// validateVersion validates the skill version
func (v *SkillValidator) validateVersion(version string) *ValidationError {
	if version == "" {
		return &ValidationError{Field: "version", Message: "version cannot be empty"}
	}

	if !v.versionPattern.MatchString(version) {
		return &ValidationError{
			Field:   "version",
			Message: "version must follow semantic versioning (e.g., 1.0.0, 2.1.0-beta.1)",
		}
	}

	return nil
}

// validateScope validates the skill scope
func (v *SkillValidator) validateScope(scope SkillScope) *ValidationError {
	switch scope {
	case ScopeFramework, ScopeAgent:
		return nil
	default:
		return &ValidationError{
			Field:   "scope",
			Message: fmt.Sprintf("invalid scope: %s (must be one of: framework, agent)", scope),
		}
	}
}

// validateScopeConsistency validates that agent-scoped skills have allowed agents
func (v *SkillValidator) validateScopeConsistency(skill Skill) *ValidationError {
	if skill.Scope() == ScopeAgent && len(skill.AllowedAgents()) == 0 {
		return &ValidationError{
			Field:   "allowed_agents",
			Message: "agent-scoped skills must specify at least one allowed agent",
		}
	}

	return nil
}

// validateDependencies validates the skill dependencies
func (v *SkillValidator) validateDependencies(deps []string) []*ValidationError {
	var errors []*ValidationError
	seen := make(map[string]bool)

	for i, dep := range deps {
		if dep == "" {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("dependencies[%d]", i),
				Message: "dependency name cannot be empty",
			})
			continue
		}

		if !v.namePattern.MatchString(dep) {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("dependencies[%d]", i),
				Message: fmt.Sprintf("invalid dependency name: %s", dep),
			})
			continue
		}

		if seen[dep] {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("dependencies[%d]", i),
				Message: fmt.Sprintf("duplicate dependency: %s", dep),
			})
			continue
		}

		seen[dep] = true
	}

	return errors
}

// validateMetadata validates the skill metadata
func (v *SkillValidator) validateMetadata(meta SkillMetadata) []*ValidationError {
	var errors []*ValidationError

	// Validate author length
	if len(meta.Author) > 256 {
		errors = append(errors, &ValidationError{
			Field:   "metadata.author",
			Message: "author cannot exceed 256 characters",
		})
	}

	// Validate source
	if meta.Source == "" {
		errors = append(errors, &ValidationError{
			Field:   "metadata.source",
			Message: "source cannot be empty",
		})
	}

	// Validate tags
	for i, tag := range meta.Tags {
		if tag == "" {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("metadata.tags[%d]", i),
				Message: "tag cannot be empty",
			})
		}
		if len(tag) > 64 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("metadata.tags[%d]", i),
				Message: "tag cannot exceed 64 characters",
			})
		}
	}

	return errors
}

// ValidateNativeToolDefinition validates a native tool definition
func (v *SkillValidator) ValidateNativeToolDefinition(def *NativeToolDefinition) error {
	if def == nil {
		return &ValidationError{Field: "tool_definition", Message: "tool definition cannot be nil"}
	}

	var errors ValidationErrors

	// Validate name
	if def.Name == "" {
		errors = append(errors, &ValidationError{Field: "tool_definition.name", Message: "name cannot be empty"})
	}

	// Validate description
	if def.Description == "" {
		errors = append(errors, &ValidationError{Field: "tool_definition.description", Message: "description cannot be empty"})
	}

	// Validate input schema
	if errs := v.validateJSONSchema(&def.InputSchema, "tool_definition.input_schema"); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateJSONSchema validates a JSON schema
func (v *SkillValidator) validateJSONSchema(schema *JSONSchema, prefix string) []*ValidationError {
	var errors []*ValidationError

	if schema.Type == "" {
		errors = append(errors, &ValidationError{
			Field:   prefix + ".type",
			Message: "type cannot be empty",
		})
	} else if schema.Type != "object" {
		errors = append(errors, &ValidationError{
			Field:   prefix + ".type",
			Message: "root schema type must be 'object'",
		})
	}

	// Validate properties
	for name, prop := range schema.Properties {
		propErrors := v.validatePropertySchema(&prop, fmt.Sprintf("%s.properties.%s", prefix, name))
		errors = append(errors, propErrors...)
	}

	// Check that required properties exist
	for _, req := range schema.Required {
		if _, exists := schema.Properties[req]; !exists {
			errors = append(errors, &ValidationError{
				Field:   prefix + ".required",
				Message: fmt.Sprintf("required property '%s' is not defined in properties", req),
			})
		}
	}

	return errors
}

// validatePropertySchema validates a property schema
func (v *SkillValidator) validatePropertySchema(prop *PropertySchema, prefix string) []*ValidationError {
	var errors []*ValidationError

	validTypes := map[string]bool{
		"string":  true,
		"number":  true,
		"integer": true,
		"boolean": true,
		"array":   true,
		"object":  true,
	}

	if prop.Type == "" {
		errors = append(errors, &ValidationError{
			Field:   prefix + ".type",
			Message: "type cannot be empty",
		})
	} else if !validTypes[prop.Type] {
		errors = append(errors, &ValidationError{
			Field:   prefix + ".type",
			Message: fmt.Sprintf("invalid type: %s", prop.Type),
		})
	}

	// Validate array items
	if prop.Type == "array" && prop.Items == nil {
		errors = append(errors, &ValidationError{
			Field:   prefix + ".items",
			Message: "array type must specify items schema",
		})
	} else if prop.Type == "array" && prop.Items != nil {
		itemErrors := v.validatePropertySchema(prop.Items, prefix+".items")
		errors = append(errors, itemErrors...)
	}

	// Validate nested object properties
	if prop.Type == "object" {
		for name, nested := range prop.Properties {
			nestedErrors := v.validatePropertySchema(&nested, fmt.Sprintf("%s.properties.%s", prefix, name))
			errors = append(errors, nestedErrors...)
		}
	}

	// Validate numeric constraints
	if prop.Minimum != nil && prop.Maximum != nil && *prop.Minimum > *prop.Maximum {
		errors = append(errors, &ValidationError{
			Field:   prefix,
			Message: "minimum cannot be greater than maximum",
		})
	}

	// Validate string constraints
	if prop.MinLength != nil && prop.MaxLength != nil && *prop.MinLength > *prop.MaxLength {
		errors = append(errors, &ValidationError{
			Field:   prefix,
			Message: "minLength cannot be greater than maxLength",
		})
	}

	// Validate pattern is valid regex
	if prop.Pattern != "" {
		if _, err := regexp.Compile(prop.Pattern); err != nil {
			errors = append(errors, &ValidationError{
				Field:   prefix + ".pattern",
				Message: fmt.Sprintf("invalid regex pattern: %v", err),
			})
		}
	}

	return errors
}

// ValidateSkillConfig validates a skill configuration
func (v *SkillValidator) ValidateSkillConfig(config *SkillConfig) error {
	if config == nil {
		return nil
	}

	var errors ValidationErrors

	// Validate temperature
	if config.Temperature < 0 || config.Temperature > 2 {
		errors = append(errors, &ValidationError{
			Field:   "config.temperature",
			Message: "temperature must be between 0 and 2",
		})
	}

	// Validate max tokens
	if config.MaxTokens < 0 {
		errors = append(errors, &ValidationError{
			Field:   "config.max_tokens",
			Message: "max_tokens cannot be negative",
		})
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}
