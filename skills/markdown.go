package skills

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// MarkdownSkill is a skill defined by a markdown file with frontmatter
type MarkdownSkill struct {
	*BaseSkill
	content         string
	systemPrompt    string
	examples        []SkillExample
	additionalNotes string
	rawContent      string
}

// SkillExample represents an example in a markdown skill
type SkillExample struct {
	Title  string
	Input  string
	Output string
}

// MarkdownSkillConfig is the configuration for creating a markdown skill
type MarkdownSkillConfig struct {
	Name          string
	Description   string
	Version       string
	Scope         SkillScope
	AllowedAgents []string
	Content       string
	Source        string
	Author        string
	Tags          []string
	Dependencies  []string
	Config        SkillConfig
}

// NewMarkdownSkill creates a new markdown skill from configuration
func NewMarkdownSkill(config MarkdownSkillConfig) (*MarkdownSkill, error) {
	if config.Name == "" {
		return nil, fmt.Errorf("skill name is required")
	}
	if config.Description == "" {
		return nil, fmt.Errorf("skill description is required")
	}

	base := NewBaseSkill(config.Name, config.Description, SkillTypeMarkdown)

	if config.Version != "" {
		base.SetVersion(config.Version)
	}
	if config.Scope != "" {
		base.SetScope(config.Scope)
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

	base.SetConfig(config.Config)

	skill := &MarkdownSkill{
		BaseSkill:  base,
		rawContent: config.Content,
	}

	// Parse the content
	skill.parseContent(config.Content)

	return skill, nil
}

// NewMarkdownSkillFromParsed creates a markdown skill from a parsed file
func NewMarkdownSkillFromParsed(parsed *ParsedSkillFile) (*MarkdownSkill, error) {
	if parsed == nil || parsed.Frontmatter == nil {
		return nil, fmt.Errorf("parsed file or frontmatter is nil")
	}

	fm := parsed.Frontmatter
	parser := NewFrontmatterParser()

	config := MarkdownSkillConfig{
		Name:          fm.Name,
		Description:   fm.Description,
		Version:       fm.Version,
		Scope:         fm.Scope,
		AllowedAgents: fm.AllowedAgents,
		Content:       parsed.Content,
		Source:        parsed.Source,
		Author:        fm.Author,
		Tags:          fm.Tags,
		Dependencies:  fm.Dependencies,
		Config:        parser.ToSkillConfig(fm.Config),
	}

	return NewMarkdownSkill(config)
}

// parseContent parses the markdown content into structured sections
func (s *MarkdownSkill) parseContent(content string) {
	s.rawContent = content

	// Extract sections
	sections := s.extractSections(content)

	// Process system instructions
	if instructions, ok := sections["system instructions"]; ok {
		s.systemPrompt = strings.TrimSpace(instructions)
	} else if instructions, ok := sections["instructions"]; ok {
		s.systemPrompt = strings.TrimSpace(instructions)
	}

	// Process examples
	if examples, ok := sections["examples"]; ok {
		s.examples = s.parseExamples(examples)
	}

	// Process additional context/notes
	if notes, ok := sections["additional context"]; ok {
		s.additionalNotes = strings.TrimSpace(notes)
	} else if notes, ok := sections["notes"]; ok {
		s.additionalNotes = strings.TrimSpace(notes)
	}

	// If no structured content, use the whole thing as content
	if s.systemPrompt == "" && len(s.examples) == 0 {
		s.content = content
	}
}

// extractSections extracts sections from markdown content based on headers
func (s *MarkdownSkill) extractSections(content string) map[string]string {
	sections := make(map[string]string)

	// Pattern to match markdown headers (## Header)
	headerPattern := regexp.MustCompile(`(?m)^##\s+(.+)$`)
	matches := headerPattern.FindAllStringSubmatchIndex(content, -1)

	if len(matches) == 0 {
		// No sections found, return whole content as main section
		sections["main"] = content
		return sections
	}

	// Extract each section
	for i, match := range matches {
		headerStart := match[0]
		headerEnd := match[1]
		titleStart := match[2]
		titleEnd := match[3]

		title := strings.ToLower(strings.TrimSpace(content[titleStart:titleEnd]))

		// Find the end of this section (start of next section or end of content)
		var sectionEnd int
		if i+1 < len(matches) {
			sectionEnd = matches[i+1][0]
		} else {
			sectionEnd = len(content)
		}

		// Extract section content (excluding the header line)
		sectionContent := content[headerEnd:sectionEnd]
		sections[title] = strings.TrimSpace(sectionContent)

		// Handle content before the first header
		if i == 0 && headerStart > 0 {
			sections["preamble"] = strings.TrimSpace(content[:headerStart])
		}
	}

	return sections
}

// parseExamples parses example sections from content
func (s *MarkdownSkill) parseExamples(content string) []SkillExample {
	var examples []SkillExample

	// Pattern to match example subsections (### Example N: Title)
	examplePattern := regexp.MustCompile(`(?m)^###\s+Example\s*\d*:?\s*(.*)$`)
	matches := examplePattern.FindAllStringSubmatchIndex(content, -1)

	if len(matches) == 0 {
		// Try to parse as a single example with Input/Output
		example := s.parseInputOutput(content)
		if example.Input != "" || example.Output != "" {
			examples = append(examples, example)
		}
		return examples
	}

	for i, match := range matches {
		titleStart := match[2]
		titleEnd := match[3]
		title := strings.TrimSpace(content[titleStart:titleEnd])

		// Find the end of this example
		var exampleEnd int
		if i+1 < len(matches) {
			exampleEnd = matches[i+1][0]
		} else {
			exampleEnd = len(content)
		}

		exampleContent := content[match[1]:exampleEnd]
		example := s.parseInputOutput(exampleContent)
		example.Title = title

		if example.Input != "" || example.Output != "" {
			examples = append(examples, example)
		}
	}

	return examples
}

// parseInputOutput parses input/output from example content
func (s *MarkdownSkill) parseInputOutput(content string) SkillExample {
	example := SkillExample{}

	// Look for **Input:** and **Output:** patterns
	inputPattern := regexp.MustCompile(`(?is)\*\*Input:\*\*\s*(.+?)(?:\*\*(?:Output|Recommendation):\*\*|$)`)
	outputPattern := regexp.MustCompile(`(?is)\*\*(?:Output|Recommendation):\*\*\s*(.+?)(?:\*\*|$)`)

	if inputMatch := inputPattern.FindStringSubmatch(content); len(inputMatch) > 1 {
		example.Input = strings.TrimSpace(inputMatch[1])
	}

	if outputMatch := outputPattern.FindStringSubmatch(content); len(outputMatch) > 1 {
		example.Output = strings.TrimSpace(outputMatch[1])
	}

	return example
}

// Execute runs the markdown skill
func (s *MarkdownSkill) Execute(ctx context.Context, input *SkillInput) (*SkillOutput, error) {
	startTime := time.Now()

	// Build the output based on skill content
	output := &SkillOutput{
		Success:  true,
		Metadata: make(map[string]interface{}),
	}

	// Set the system prompt/instructions
	if s.systemPrompt != "" {
		output.Instructions = s.systemPrompt
	}

	// Build enhanced prompt with examples
	var promptBuilder strings.Builder

	if s.systemPrompt != "" {
		promptBuilder.WriteString(s.systemPrompt)
		promptBuilder.WriteString("\n\n")
	}

	// Add examples if present
	if len(s.examples) > 0 {
		promptBuilder.WriteString("## Examples\n\n")
		for i, ex := range s.examples {
			if ex.Title != "" {
				promptBuilder.WriteString(fmt.Sprintf("### Example %d: %s\n\n", i+1, ex.Title))
			} else {
				promptBuilder.WriteString(fmt.Sprintf("### Example %d\n\n", i+1))
			}
			if ex.Input != "" {
				promptBuilder.WriteString(fmt.Sprintf("**Input:**\n%s\n\n", ex.Input))
			}
			if ex.Output != "" {
				promptBuilder.WriteString(fmt.Sprintf("**Output:**\n%s\n\n", ex.Output))
			}
		}
	}

	// Add additional notes
	if s.additionalNotes != "" {
		promptBuilder.WriteString("## Additional Context\n\n")
		promptBuilder.WriteString(s.additionalNotes)
		promptBuilder.WriteString("\n\n")
	}

	// Add the user's query
	if input != nil && input.Query != "" {
		promptBuilder.WriteString("## Current Request\n\n")
		promptBuilder.WriteString(input.Query)
	}

	output.Prompt = promptBuilder.String()
	output.Content = s.rawContent

	// Add skill info to metadata
	output.Metadata["skill_name"] = s.Name()
	output.Metadata["skill_version"] = s.Version()
	output.Metadata["skill_type"] = string(s.Type())
	output.Metadata["has_examples"] = len(s.examples) > 0
	output.Metadata["example_count"] = len(s.examples)

	// Add dependency results if present
	if input != nil && len(input.DependencyResults) > 0 {
		output.Metadata["dependency_results"] = input.DependencyResults
	}

	output.ExecutionTime = time.Since(startTime).Milliseconds()

	return output, nil
}

// SystemPrompt returns the skill's system prompt
func (s *MarkdownSkill) SystemPrompt() string {
	return s.systemPrompt
}

// Examples returns the skill's examples
func (s *MarkdownSkill) Examples() []SkillExample {
	return s.examples
}

// AdditionalNotes returns the skill's additional notes
func (s *MarkdownSkill) AdditionalNotes() string {
	return s.additionalNotes
}

// RawContent returns the raw markdown content
func (s *MarkdownSkill) RawContent() string {
	return s.rawContent
}

// MarkdownParser parses markdown skill files
type MarkdownParser struct {
	frontmatterParser *FrontmatterParser
}

// NewMarkdownParser creates a new markdown parser
func NewMarkdownParser() *MarkdownParser {
	return &MarkdownParser{
		frontmatterParser: NewFrontmatterParser(),
	}
}

// Parse parses a markdown skill file
func (p *MarkdownParser) Parse(content []byte, source string) (Skill, error) {
	// Parse frontmatter
	parsed, err := p.frontmatterParser.Parse(content, source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Create appropriate skill based on type
	switch parsed.Frontmatter.Type {
	case SkillTypeMarkdown:
		return NewMarkdownSkillFromParsed(parsed)

	case SkillTypeNativeTool:
		// Native tool skills need a handler, so we can't create them from just markdown
		// Return a placeholder that will need a handler set later
		return p.parseNativeToolSkill(parsed)

	case SkillTypeBehavior:
		// Behavior skills are similar to markdown skills but used differently
		return NewMarkdownSkillFromParsed(parsed)

	default:
		return nil, fmt.Errorf("unsupported skill type: %s", parsed.Frontmatter.Type)
	}
}

// parseNativeToolSkill creates a native tool skill from parsed markdown
func (p *MarkdownParser) parseNativeToolSkill(parsed *ParsedSkillFile) (Skill, error) {
	fm := parsed.Frontmatter

	if fm.ToolDefinition == nil {
		return nil, fmt.Errorf("native_tool type requires tool_definition")
	}

	toolDef, err := p.frontmatterParser.ToNativeToolDefinition(fm.ToolDefinition)
	if err != nil {
		return nil, err
	}

	// Create native tool skill without handler (handler must be set separately)
	config := NativeToolSkillConfig{
		Definition:    *toolDef,
		Scope:         fm.Scope,
		AllowedAgents: fm.AllowedAgents,
		Version:       fm.Version,
		Author:        fm.Author,
		Tags:          fm.Tags,
		Dependencies:  fm.Dependencies,
		Source:        parsed.Source,
	}

	return NewNativeToolSkillWithoutHandler(config)
}

// HasSkill is a helper interface for checking skill existence
type HasSkill interface {
	HasSkill(name string) bool
}

// MarkdownSkillBuilder is a builder for markdown skills
type MarkdownSkillBuilder struct {
	config MarkdownSkillConfig
}

// NewMarkdownSkillBuilder creates a new markdown skill builder
func NewMarkdownSkillBuilder(name string) *MarkdownSkillBuilder {
	return &MarkdownSkillBuilder{
		config: MarkdownSkillConfig{
			Name:    name,
			Version: "1.0.0",
			Scope:   ScopeFramework,
		},
	}
}

// WithDescription sets the description
func (b *MarkdownSkillBuilder) WithDescription(desc string) *MarkdownSkillBuilder {
	b.config.Description = desc
	return b
}

// WithVersion sets the version
func (b *MarkdownSkillBuilder) WithVersion(version string) *MarkdownSkillBuilder {
	b.config.Version = version
	return b
}

// WithScope sets the scope
func (b *MarkdownSkillBuilder) WithScope(scope SkillScope) *MarkdownSkillBuilder {
	b.config.Scope = scope
	return b
}

// WithAllowedAgents sets the allowed agents
func (b *MarkdownSkillBuilder) WithAllowedAgents(agents []string) *MarkdownSkillBuilder {
	b.config.AllowedAgents = agents
	return b
}

// WithContent sets the markdown content
func (b *MarkdownSkillBuilder) WithContent(content string) *MarkdownSkillBuilder {
	b.config.Content = content
	return b
}

// WithSource sets the source
func (b *MarkdownSkillBuilder) WithSource(source string) *MarkdownSkillBuilder {
	b.config.Source = source
	return b
}

// WithAuthor sets the author
func (b *MarkdownSkillBuilder) WithAuthor(author string) *MarkdownSkillBuilder {
	b.config.Author = author
	return b
}

// WithTags sets the tags
func (b *MarkdownSkillBuilder) WithTags(tags []string) *MarkdownSkillBuilder {
	b.config.Tags = tags
	return b
}

// WithDependencies sets the dependencies
func (b *MarkdownSkillBuilder) WithDependencies(deps []string) *MarkdownSkillBuilder {
	b.config.Dependencies = deps
	return b
}

// WithConfig sets the skill config
func (b *MarkdownSkillBuilder) WithConfig(config SkillConfig) *MarkdownSkillBuilder {
	b.config.Config = config
	return b
}

// Build creates the markdown skill
func (b *MarkdownSkillBuilder) Build() (*MarkdownSkill, error) {
	return NewMarkdownSkill(b.config)
}
