// Package prompt provides prompt template functionality for LLM applications.
// Prompt templates allow dynamic construction of prompts with variable substitution.
package prompt

import (
	"fmt"
	"regexp"
	"strings"
)

// Template represents a prompt template
type Template struct {
	template     string
	inputVars    []string
	partialVars  map[string]any
	templateType TemplateType
}

// TemplateType represents the template format type
type TemplateType string

const (
	// TemplateTypeGoTemplate uses Go template syntax: {{.variable}}
	TemplateTypeGoTemplate TemplateType = "go_template"

	// TemplateTypeFString uses Python f-string style: {variable}
	TemplateTypeFString TemplateType = "f_string"
)

// TemplateConfig configures a prompt template
type TemplateConfig struct {
	// Template is the template string
	Template string

	// InputVariables are the required input variables
	InputVariables []string

	// PartialVariables are pre-filled variables
	PartialVariables map[string]any

	// TemplateType is the format type (default: go_template)
	TemplateType TemplateType
}

// NewTemplate creates a new prompt template
func NewTemplate(cfg TemplateConfig) (*Template, error) {
	templateType := cfg.TemplateType
	if templateType == "" {
		templateType = TemplateTypeGoTemplate
	}

	// Auto-detect input variables if not provided
	inputVars := cfg.InputVariables
	if len(inputVars) == 0 {
		inputVars = extractVariables(cfg.Template, templateType)
	}

	return &Template{
		template:     cfg.Template,
		inputVars:    inputVars,
		partialVars:  cfg.PartialVariables,
		templateType: templateType,
	}, nil
}

// Format formats the template with the given variables
func (t *Template) Format(vars map[string]any) (string, error) {
	// Merge partial variables with provided variables
	mergedVars := make(map[string]any)
	for k, v := range t.partialVars {
		mergedVars[k] = v
	}
	for k, v := range vars {
		mergedVars[k] = v
	}

	// Check for missing variables
	for _, key := range t.inputVars {
		if _, ok := mergedVars[key]; !ok {
			return "", fmt.Errorf("missing required variable: %s", key)
		}
	}

	switch t.templateType {
	case TemplateTypeFString:
		return t.formatFString(mergedVars)
	default:
		return t.formatGoTemplate(mergedVars)
	}
}

// formatGoTemplate formats using Go template syntax
func (t *Template) formatGoTemplate(vars map[string]any) (string, error) {
	result := t.template
	for key, val := range vars {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		strVal := fmt.Sprintf("%v", val)
		result = strings.ReplaceAll(result, placeholder, strVal)
	}
	return result, nil
}

// formatFString formats using f-string syntax
func (t *Template) formatFString(vars map[string]any) (string, error) {
	result := t.template
	for key, val := range vars {
		placeholder := fmt.Sprintf("{%s}", key)
		strVal := fmt.Sprintf("%v", val)
		result = strings.ReplaceAll(result, placeholder, strVal)
	}
	return result, nil
}

// InputVariables returns the required input variables
func (t *Template) InputVariables() []string {
	return t.inputVars
}

// Template returns the template string
func (t *Template) Template() string {
	return t.template
}

// PartialFormat creates a new template with some variables filled in
func (t *Template) PartialFormat(vars map[string]any) (*Template, error) {
	newPartials := make(map[string]any)
	for k, v := range t.partialVars {
		newPartials[k] = v
	}
	for k, v := range vars {
		newPartials[k] = v
	}

	// Remove filled variables from input variables
	var remainingVars []string
	for _, v := range t.inputVars {
		if _, ok := newPartials[v]; !ok {
			remainingVars = append(remainingVars, v)
		}
	}

	return &Template{
		template:     t.template,
		inputVars:    remainingVars,
		partialVars:  newPartials,
		templateType: t.templateType,
	}, nil
}

// extractVariables extracts variable names from template
func extractVariables(template string, templateType TemplateType) []string {
	var pattern *regexp.Regexp

	switch templateType {
	case TemplateTypeFString:
		pattern = regexp.MustCompile(`\{(\w+)\}`)
	default:
		pattern = regexp.MustCompile(`\{\{\.\s*(\w+)\s*\}\}`)
	}

	matches := pattern.FindAllStringSubmatch(template, -1)
	seen := make(map[string]bool)
	var vars []string

	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			vars = append(vars, match[1])
			seen[match[1]] = true
		}
	}

	return vars
}

// ChatTemplate represents a chat message template
type ChatTemplate struct {
	systemTemplate  *Template
	humanTemplate   *Template
	aiTemplate      *Template
	inputVars       []string
}

// ChatTemplateConfig configures a chat template
type ChatTemplateConfig struct {
	// SystemTemplate is the system message template
	SystemTemplate string

	// HumanTemplate is the human message template
	HumanTemplate string

	// AITemplate is the AI message template (optional)
	AITemplate string

	// InputVariables are the required input variables
	InputVariables []string
}

// NewChatTemplate creates a new chat template
func NewChatTemplate(cfg ChatTemplateConfig) (*ChatTemplate, error) {
	var systemTemplate, humanTemplate, aiTemplate *Template
	var err error

	if cfg.SystemTemplate != "" {
		systemTemplate, err = NewTemplate(TemplateConfig{Template: cfg.SystemTemplate})
		if err != nil {
			return nil, fmt.Errorf("invalid system template: %w", err)
		}
	}

	if cfg.HumanTemplate != "" {
		humanTemplate, err = NewTemplate(TemplateConfig{Template: cfg.HumanTemplate})
		if err != nil {
			return nil, fmt.Errorf("invalid human template: %w", err)
		}
	}

	if cfg.AITemplate != "" {
		aiTemplate, err = NewTemplate(TemplateConfig{Template: cfg.AITemplate})
		if err != nil {
			return nil, fmt.Errorf("invalid AI template: %w", err)
		}
	}

	// Collect all input variables
	varsMap := make(map[string]bool)
	if systemTemplate != nil {
		for _, v := range systemTemplate.InputVariables() {
			varsMap[v] = true
		}
	}
	if humanTemplate != nil {
		for _, v := range humanTemplate.InputVariables() {
			varsMap[v] = true
		}
	}
	if aiTemplate != nil {
		for _, v := range aiTemplate.InputVariables() {
			varsMap[v] = true
		}
	}

	// Use provided input variables or extracted ones
	inputVars := cfg.InputVariables
	if len(inputVars) == 0 {
		for v := range varsMap {
			inputVars = append(inputVars, v)
		}
	}

	return &ChatTemplate{
		systemTemplate: systemTemplate,
		humanTemplate:  humanTemplate,
		aiTemplate:     aiTemplate,
		inputVars:      inputVars,
	}, nil
}

// ChatMessage represents a formatted chat message
type ChatMessage struct {
	Role    string
	Content string
}

// FormatMessages formats the chat template into messages
func (t *ChatTemplate) FormatMessages(vars map[string]any) ([]ChatMessage, error) {
	var messages []ChatMessage

	if t.systemTemplate != nil {
		content, err := t.systemTemplate.Format(vars)
		if err != nil {
			return nil, fmt.Errorf("system template error: %w", err)
		}
		messages = append(messages, ChatMessage{Role: "system", Content: content})
	}

	if t.humanTemplate != nil {
		content, err := t.humanTemplate.Format(vars)
		if err != nil {
			return nil, fmt.Errorf("human template error: %w", err)
		}
		messages = append(messages, ChatMessage{Role: "user", Content: content})
	}

	if t.aiTemplate != nil {
		content, err := t.aiTemplate.Format(vars)
		if err != nil {
			return nil, fmt.Errorf("AI template error: %w", err)
		}
		messages = append(messages, ChatMessage{Role: "assistant", Content: content})
	}

	return messages, nil
}

// InputVariables returns the required input variables
func (t *ChatTemplate) InputVariables() []string {
	return t.inputVars
}

// FewShotTemplate represents a few-shot prompt template
type FewShotTemplate struct {
	prefix      string
	suffix      string
	examples    []map[string]any
	exampleTemplate *Template
	separator   string
	inputVars   []string
}

// FewShotTemplateConfig configures a few-shot template
type FewShotTemplateConfig struct {
	// Prefix is the text before examples
	Prefix string

	// Suffix is the text after examples (usually contains the input)
	Suffix string

	// Examples are the few-shot examples
	Examples []map[string]any

	// ExampleTemplate formats each example
	ExampleTemplate string

	// Separator between examples (default: "\n\n")
	Separator string

	// InputVariables for the suffix
	InputVariables []string
}

// NewFewShotTemplate creates a new few-shot template
func NewFewShotTemplate(cfg FewShotTemplateConfig) (*FewShotTemplate, error) {
	exampleTemplate, err := NewTemplate(TemplateConfig{Template: cfg.ExampleTemplate})
	if err != nil {
		return nil, fmt.Errorf("invalid example template: %w", err)
	}

	separator := cfg.Separator
	if separator == "" {
		separator = "\n\n"
	}

	// Extract input variables from suffix
	inputVars := cfg.InputVariables
	if len(inputVars) == 0 {
		inputVars = extractVariables(cfg.Suffix, TemplateTypeGoTemplate)
	}

	return &FewShotTemplate{
		prefix:          cfg.Prefix,
		suffix:          cfg.Suffix,
		examples:        cfg.Examples,
		exampleTemplate: exampleTemplate,
		separator:       separator,
		inputVars:       inputVars,
	}, nil
}

// Format formats the few-shot template
func (t *FewShotTemplate) Format(vars map[string]any) (string, error) {
	var parts []string

	// Add prefix
	if t.prefix != "" {
		parts = append(parts, t.prefix)
	}

	// Format examples
	for _, example := range t.examples {
		formatted, err := t.exampleTemplate.Format(example)
		if err != nil {
			return "", fmt.Errorf("example format error: %w", err)
		}
		parts = append(parts, formatted)
	}

	// Format and add suffix
	if t.suffix != "" {
		suffixTemplate, err := NewTemplate(TemplateConfig{Template: t.suffix})
		if err != nil {
			return "", err
		}
		formatted, err := suffixTemplate.Format(vars)
		if err != nil {
			return "", fmt.Errorf("suffix format error: %w", err)
		}
		parts = append(parts, formatted)
	}

	return strings.Join(parts, t.separator), nil
}

// InputVariables returns the required input variables
func (t *FewShotTemplate) InputVariables() []string {
	return t.inputVars
}
