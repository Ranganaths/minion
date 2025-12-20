// Package outputparser provides parsers for structured LLM output.
// Output parsers convert raw LLM text into structured data formats.
package outputparser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// OutputParser is the interface for parsing LLM output
type OutputParser interface {
	// Parse parses the LLM output into structured data
	Parse(text string) (any, error)

	// GetFormatInstructions returns instructions for the LLM
	GetFormatInstructions() string
}

// StringOutputParser returns the output as-is
type StringOutputParser struct{}

// NewStringOutputParser creates a new string output parser
func NewStringOutputParser() *StringOutputParser {
	return &StringOutputParser{}
}

// Parse returns the text as-is
func (p *StringOutputParser) Parse(text string) (any, error) {
	return strings.TrimSpace(text), nil
}

// GetFormatInstructions returns empty instructions
func (p *StringOutputParser) GetFormatInstructions() string {
	return ""
}

// JSONOutputParser parses JSON output
type JSONOutputParser struct {
	schema map[string]any
}

// JSONOutputParserConfig configures the JSON parser
type JSONOutputParserConfig struct {
	// Schema is an optional JSON schema to validate against
	Schema map[string]any
}

// NewJSONOutputParser creates a new JSON output parser
func NewJSONOutputParser(cfg JSONOutputParserConfig) *JSONOutputParser {
	return &JSONOutputParser{
		schema: cfg.Schema,
	}
}

// Parse parses JSON from the output
func (p *JSONOutputParser) Parse(text string) (any, error) {
	// Try to extract JSON from the text
	text = strings.TrimSpace(text)

	// Try to find JSON in code blocks
	jsonRegex := regexp.MustCompile("```(?:json)?\\s*([\\s\\S]*?)```")
	matches := jsonRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		text = strings.TrimSpace(matches[1])
	}

	// Try to find JSON object or array
	startIdx := strings.IndexAny(text, "{[")
	if startIdx >= 0 {
		// Find matching end
		var endChar byte
		if text[startIdx] == '{' {
			endChar = '}'
		} else {
			endChar = ']'
		}

		depth := 0
		for i := startIdx; i < len(text); i++ {
			if text[i] == text[startIdx] {
				depth++
			} else if text[i] == endChar {
				depth--
				if depth == 0 {
					text = text[startIdx : i+1]
					break
				}
			}
		}
	}

	var result any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w\nRaw output: %s", err, text)
	}

	return result, nil
}

// GetFormatInstructions returns JSON format instructions
func (p *JSONOutputParser) GetFormatInstructions() string {
	if p.schema != nil {
		schemaBytes, _ := json.MarshalIndent(p.schema, "", "  ")
		return fmt.Sprintf("Return your response as a valid JSON object matching this schema:\n```json\n%s\n```", string(schemaBytes))
	}
	return "Return your response as a valid JSON object."
}

// ListOutputParser parses comma-separated or newline-separated lists
type ListOutputParser struct {
	separator string
}

// ListOutputParserConfig configures the list parser
type ListOutputParserConfig struct {
	// Separator is the list item separator (default: auto-detect)
	Separator string
}

// NewListOutputParser creates a new list output parser
func NewListOutputParser(cfg ListOutputParserConfig) *ListOutputParser {
	return &ListOutputParser{
		separator: cfg.Separator,
	}
}

// Parse parses a list from the output
func (p *ListOutputParser) Parse(text string) (any, error) {
	text = strings.TrimSpace(text)

	var items []string
	if p.separator != "" {
		items = strings.Split(text, p.separator)
	} else {
		// Auto-detect separator
		if strings.Contains(text, "\n") {
			items = strings.Split(text, "\n")
		} else if strings.Contains(text, ",") {
			items = strings.Split(text, ",")
		} else {
			items = []string{text}
		}
	}

	// Clean up items
	var result []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		// Remove common list prefixes
		item = strings.TrimPrefix(item, "- ")
		item = strings.TrimPrefix(item, "* ")
		item = regexp.MustCompile(`^\d+\.\s*`).ReplaceAllString(item, "")

		if item != "" {
			result = append(result, item)
		}
	}

	return result, nil
}

// GetFormatInstructions returns list format instructions
func (p *ListOutputParser) GetFormatInstructions() string {
	if p.separator != "" {
		return fmt.Sprintf("Return your response as a list of items separated by '%s'.", p.separator)
	}
	return "Return your response as a list, with each item on a new line."
}

// BooleanOutputParser parses yes/no or true/false output
type BooleanOutputParser struct{}

// NewBooleanOutputParser creates a new boolean output parser
func NewBooleanOutputParser() *BooleanOutputParser {
	return &BooleanOutputParser{}
}

// Parse parses a boolean from the output
func (p *BooleanOutputParser) Parse(text string) (any, error) {
	text = strings.ToLower(strings.TrimSpace(text))

	// Check for true values
	trueValues := []string{"yes", "true", "1", "correct", "affirmative", "y"}
	for _, v := range trueValues {
		if strings.HasPrefix(text, v) {
			return true, nil
		}
	}

	// Check for false values
	falseValues := []string{"no", "false", "0", "incorrect", "negative", "n"}
	for _, v := range falseValues {
		if strings.HasPrefix(text, v) {
			return false, nil
		}
	}

	return nil, fmt.Errorf("could not parse boolean from: %s", text)
}

// GetFormatInstructions returns boolean format instructions
func (p *BooleanOutputParser) GetFormatInstructions() string {
	return "Respond with only 'yes' or 'no'."
}

// RegexOutputParser parses output using a regex pattern
type RegexOutputParser struct {
	pattern     *regexp.Regexp
	outputKeys  []string
	defaultVals map[string]string
}

// RegexOutputParserConfig configures the regex parser
type RegexOutputParserConfig struct {
	// Pattern is the regex pattern with named groups
	Pattern string

	// OutputKeys are the expected output keys (should match group names)
	OutputKeys []string

	// DefaultValues are default values for missing groups
	DefaultValues map[string]string
}

// NewRegexOutputParser creates a new regex output parser
func NewRegexOutputParser(cfg RegexOutputParserConfig) (*RegexOutputParser, error) {
	pattern, err := regexp.Compile(cfg.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	return &RegexOutputParser{
		pattern:     pattern,
		outputKeys:  cfg.OutputKeys,
		defaultVals: cfg.DefaultValues,
	}, nil
}

// Parse parses output using the regex
func (p *RegexOutputParser) Parse(text string) (any, error) {
	matches := p.pattern.FindStringSubmatch(text)
	if matches == nil {
		return nil, fmt.Errorf("regex did not match output: %s", text)
	}

	result := make(map[string]string)

	// Get group names
	names := p.pattern.SubexpNames()
	for i, name := range names {
		if name != "" && i < len(matches) {
			result[name] = matches[i]
		}
	}

	// Apply default values for missing keys
	for _, key := range p.outputKeys {
		if _, ok := result[key]; !ok {
			if def, ok := p.defaultVals[key]; ok {
				result[key] = def
			}
		}
	}

	return result, nil
}

// GetFormatInstructions returns empty instructions
func (p *RegexOutputParser) GetFormatInstructions() string {
	return ""
}

// StructuredOutputParser parses output into a struct-like map
type StructuredOutputParser struct {
	fields []FieldSchema
}

// FieldSchema describes a field in the structured output
type FieldSchema struct {
	Name        string
	Description string
	Type        string // "string", "number", "boolean", "list"
	Required    bool
}

// StructuredOutputParserConfig configures the structured parser
type StructuredOutputParserConfig struct {
	Fields []FieldSchema
}

// NewStructuredOutputParser creates a new structured output parser
func NewStructuredOutputParser(cfg StructuredOutputParserConfig) *StructuredOutputParser {
	return &StructuredOutputParser{
		fields: cfg.Fields,
	}
}

// Parse parses structured output
func (p *StructuredOutputParser) Parse(text string) (any, error) {
	// Try JSON first
	jsonParser := NewJSONOutputParser(JSONOutputParserConfig{})
	result, err := jsonParser.Parse(text)
	if err == nil {
		return result, nil
	}

	// Fall back to key: value parsing
	resultMap := make(map[string]any)
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])

		// Try to match with schema fields
		for _, field := range p.fields {
			if strings.EqualFold(key, field.Name) {
				switch field.Type {
				case "number":
					var num float64
					fmt.Sscanf(value, "%f", &num)
					resultMap[field.Name] = num
				case "boolean":
					resultMap[field.Name] = strings.ToLower(value) == "true" || strings.ToLower(value) == "yes"
				case "list":
					items := strings.Split(value, ",")
					var cleaned []string
					for _, item := range items {
						cleaned = append(cleaned, strings.TrimSpace(item))
					}
					resultMap[field.Name] = cleaned
				default:
					resultMap[field.Name] = value
				}
				break
			}
		}
	}

	return resultMap, nil
}

// GetFormatInstructions returns structured format instructions
func (p *StructuredOutputParser) GetFormatInstructions() string {
	var lines []string
	lines = append(lines, "Return your response in the following format:")
	lines = append(lines, "")

	for _, field := range p.fields {
		required := ""
		if field.Required {
			required = " (required)"
		}
		lines = append(lines, fmt.Sprintf("%s: <%s>%s - %s", field.Name, field.Type, required, field.Description))
	}

	return strings.Join(lines, "\n")
}

// CompositeOutputParser chains multiple parsers
type CompositeOutputParser struct {
	parsers []OutputParser
}

// NewCompositeOutputParser creates a parser that tries multiple parsers
func NewCompositeOutputParser(parsers ...OutputParser) *CompositeOutputParser {
	return &CompositeOutputParser{parsers: parsers}
}

// Parse tries each parser until one succeeds
func (p *CompositeOutputParser) Parse(text string) (any, error) {
	var lastErr error
	for _, parser := range p.parsers {
		result, err := parser.Parse(text)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("all parsers failed, last error: %w", lastErr)
}

// GetFormatInstructions returns the first parser's instructions
func (p *CompositeOutputParser) GetFormatInstructions() string {
	if len(p.parsers) > 0 {
		return p.parsers[0].GetFormatInstructions()
	}
	return ""
}
