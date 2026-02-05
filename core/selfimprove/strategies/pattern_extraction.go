package strategies

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Ranganaths/minion/core/selfimprove"
)

// PatternExtractionStrategy extracts successful patterns from experiences.
// It identifies common patterns in successful executions and suggests
// encoding them into agent behavior.
type PatternExtractionStrategy struct {
	mu sync.RWMutex

	// Configuration
	config *PatternExtractionConfig

	// Experience store
	experienceStore selfimprove.ExperienceStore

	// Extracted patterns per task type
	patterns map[string][]*ExtractedPattern

	// Pattern statistics
	stats *PatternStats
}

// PatternExtractionConfig configures pattern extraction behavior.
type PatternExtractionConfig struct {
	// MinPatternOccurrences is the minimum times a pattern must appear
	MinPatternOccurrences int `json:"min_pattern_occurrences"`

	// MinPatternSuccessRate is the minimum success rate for a pattern
	MinPatternSuccessRate float64 `json:"min_pattern_success_rate"`

	// MaxPatternsPerType limits patterns extracted per task type
	MaxPatternsPerType int `json:"max_patterns_per_type"`

	// PatternSimilarityThreshold for deduplication
	PatternSimilarityThreshold float64 `json:"pattern_similarity_threshold"`

	// AnalysisWindowDays is how far back to analyze
	AnalysisWindowDays int `json:"analysis_window_days"`

	// ExtractInputPatterns enables input pattern extraction
	ExtractInputPatterns bool `json:"extract_input_patterns"`

	// ExtractOutputPatterns enables output pattern extraction
	ExtractOutputPatterns bool `json:"extract_output_patterns"`

	// ExtractToolPatterns enables tool usage pattern extraction
	ExtractToolPatterns bool `json:"extract_tool_patterns"`
}

// ExtractedPattern represents a pattern extracted from experiences.
type ExtractedPattern struct {
	ID           string        `json:"id"`
	TaskType     string        `json:"task_type"`
	PatternType  PatternType   `json:"pattern_type"`
	Pattern      string        `json:"pattern"`        // The pattern itself (regex, template, etc.)
	Description  string        `json:"description"`
	Examples     []string      `json:"examples"`       // Concrete examples of the pattern
	Occurrences  int           `json:"occurrences"`
	SuccessRate  float64       `json:"success_rate"`
	AvgScore     float64       `json:"avg_score"`
	FirstSeen    time.Time     `json:"first_seen"`
	LastSeen     time.Time     `json:"last_seen"`
	Confidence   float64       `json:"confidence"`
	Keywords     []string      `json:"keywords,omitempty"`
	ToolSequence []string      `json:"tool_sequence,omitempty"` // For tool patterns
}

// PatternType represents the type of extracted pattern.
type PatternType string

const (
	PatternTypeInput    PatternType = "input"
	PatternTypeOutput   PatternType = "output"
	PatternTypeTool     PatternType = "tool"
	PatternTypeWorkflow PatternType = "workflow"
)

// PatternStats tracks pattern extraction statistics.
type PatternStats struct {
	TotalPatterns       int       `json:"total_patterns"`
	PatternsPerType     map[string]int `json:"patterns_per_type"`
	HighConfidenceCount int       `json:"high_confidence_count"`
	LastExtractionTime  time.Time `json:"last_extraction_time"`
}

// NewPatternExtractionStrategy creates a new pattern extraction strategy.
func NewPatternExtractionStrategy(
	config *PatternExtractionConfig,
	experienceStore selfimprove.ExperienceStore,
) *PatternExtractionStrategy {
	if config == nil {
		config = DefaultPatternExtractionConfig()
	}

	return &PatternExtractionStrategy{
		config:          config,
		experienceStore: experienceStore,
		patterns:        make(map[string][]*ExtractedPattern),
		stats: &PatternStats{
			PatternsPerType: make(map[string]int),
		},
	}
}

// DefaultPatternExtractionConfig returns default configuration.
func DefaultPatternExtractionConfig() *PatternExtractionConfig {
	return &PatternExtractionConfig{
		MinPatternOccurrences:      5,
		MinPatternSuccessRate:      0.8,
		MaxPatternsPerType:         10,
		PatternSimilarityThreshold: 0.85,
		AnalysisWindowDays:         30,
		ExtractInputPatterns:       true,
		ExtractOutputPatterns:      true,
		ExtractToolPatterns:        true,
	}
}

// Name returns the strategy name.
func (s *PatternExtractionStrategy) Name() selfimprove.LearningStrategy {
	return selfimprove.StrategyPatternExtraction
}

// Analyze analyzes experiences and generates improvement proposals.
// Returns the highest confidence proposal or nil if none found.
func (s *PatternExtractionStrategy) Analyze(
	ctx context.Context,
	experiences []*selfimprove.Experience,
) (*selfimprove.ImprovementProposal, error) {
	if len(experiences) < s.config.MinPatternOccurrences {
		return nil, nil
	}

	// Get agent ID from first experience
	agentID := ""
	if len(experiences) > 0 {
		agentID = experiences[0].AgentID
	}

	var bestProposal *selfimprove.ImprovementProposal

	// Group experiences by task type
	byTaskType := make(map[string][]*selfimprove.Experience)
	for _, exp := range experiences {
		byTaskType[exp.TaskType] = append(byTaskType[exp.TaskType], exp)
	}

	// Extract patterns for each task type
	for taskType, taskExps := range byTaskType {
		patterns := s.extractPatterns(ctx, taskType, taskExps)

		for _, pattern := range patterns {
			proposal := s.patternToProposal(agentID, pattern)
			if proposal != nil {
				// Return the highest confidence proposal
				if bestProposal == nil || proposal.Confidence > bestProposal.Confidence {
					bestProposal = proposal
				}
			}
		}
	}

	return bestProposal, nil
}

// Apply applies an improvement (pattern encoding).
func (s *PatternExtractionStrategy) Apply(
	ctx context.Context,
	proposal *selfimprove.ImprovementProposal,
) error {
	// Pattern proposals are typically encoded into system prompts or tools
	// The actual application depends on the improvement type
	return nil
}

// IsApplicable checks if this strategy can be used.
func (s *PatternExtractionStrategy) IsApplicable(ctx context.Context, agentID string, taskType string) bool {
	return true // Pattern extraction is generally applicable
}

// extractPatterns extracts patterns from task-specific experiences.
func (s *PatternExtractionStrategy) extractPatterns(
	ctx context.Context,
	taskType string,
	experiences []*selfimprove.Experience,
) []*ExtractedPattern {
	var patterns []*ExtractedPattern

	// Separate successful and failed experiences
	var successful, failed []*selfimprove.Experience
	for _, exp := range experiences {
		if exp.Success && exp.Score >= 0.7 {
			successful = append(successful, exp)
		} else {
			failed = append(failed, exp)
		}
	}

	if len(successful) < s.config.MinPatternOccurrences {
		return nil
	}

	// Extract input patterns
	if s.config.ExtractInputPatterns {
		inputPatterns := s.extractInputPatterns(taskType, successful)
		patterns = append(patterns, inputPatterns...)
	}

	// Extract output patterns
	if s.config.ExtractOutputPatterns {
		outputPatterns := s.extractOutputPatterns(taskType, successful)
		patterns = append(patterns, outputPatterns...)
	}

	// Extract tool usage patterns
	if s.config.ExtractToolPatterns {
		toolPatterns := s.extractToolPatterns(taskType, successful)
		patterns = append(patterns, toolPatterns...)
	}

	// Calculate success rates
	for _, pattern := range patterns {
		pattern.SuccessRate = s.calculatePatternSuccessRate(pattern, successful, failed)
		pattern.Confidence = s.calculatePatternConfidence(pattern)
	}

	// Filter by success rate and occurrences
	filtered := make([]*ExtractedPattern, 0)
	for _, p := range patterns {
		if p.Occurrences >= s.config.MinPatternOccurrences &&
			p.SuccessRate >= s.config.MinPatternSuccessRate {
			filtered = append(filtered, p)
		}
	}

	// Sort by confidence and limit
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Confidence > filtered[j].Confidence
	})

	if len(filtered) > s.config.MaxPatternsPerType {
		filtered = filtered[:s.config.MaxPatternsPerType]
	}

	// Update stats
	s.mu.Lock()
	s.patterns[taskType] = filtered
	s.stats.TotalPatterns = 0
	for _, ps := range s.patterns {
		s.stats.TotalPatterns += len(ps)
		s.stats.PatternsPerType[taskType] = len(ps)
	}
	s.stats.LastExtractionTime = time.Now()
	s.mu.Unlock()

	return filtered
}

// extractInputPatterns extracts patterns from inputs.
func (s *PatternExtractionStrategy) extractInputPatterns(
	taskType string,
	successful []*selfimprove.Experience,
) []*ExtractedPattern {
	var patterns []*ExtractedPattern

	// Extract keyword patterns
	keywordFreq := make(map[string]int)
	for _, exp := range successful {
		inputStr := interfaceToString(exp.Input)
		words := s.tokenize(inputStr)
		for _, word := range words {
			if len(word) > 3 { // Skip short words
				keywordFreq[word]++
			}
		}
	}

	// Find significant keywords
	var significantKeywords []string
	for word, count := range keywordFreq {
		if count >= s.config.MinPatternOccurrences {
			significantKeywords = append(significantKeywords, word)
		}
	}

	if len(significantKeywords) > 0 {
		patterns = append(patterns, &ExtractedPattern{
			ID:          fmt.Sprintf("pattern-input-keywords-%s", taskType),
			TaskType:    taskType,
			PatternType: PatternTypeInput,
			Pattern:     strings.Join(significantKeywords, "|"),
			Description: fmt.Sprintf("Common keywords in successful %s tasks", taskType),
			Keywords:    significantKeywords,
			Occurrences: len(successful),
			FirstSeen:   s.earliestTimestamp(successful),
			LastSeen:    s.latestTimestamp(successful),
		})
	}

	// Extract structural patterns
	structurePatterns := s.extractStructuralPatterns(successful)
	for i, sp := range structurePatterns {
		patterns = append(patterns, &ExtractedPattern{
			ID:          fmt.Sprintf("pattern-input-structure-%s-%d", taskType, i),
			TaskType:    taskType,
			PatternType: PatternTypeInput,
			Pattern:     sp.pattern,
			Description: sp.description,
			Examples:    sp.examples,
			Occurrences: sp.count,
			FirstSeen:   s.earliestTimestamp(successful),
			LastSeen:    s.latestTimestamp(successful),
		})
	}

	return patterns
}

// extractOutputPatterns extracts patterns from outputs.
func (s *PatternExtractionStrategy) extractOutputPatterns(
	taskType string,
	successful []*selfimprove.Experience,
) []*ExtractedPattern {
	var patterns []*ExtractedPattern

	// Extract format patterns
	formatPatterns := make(map[string]int)
	for _, exp := range successful {
		outputStr := interfaceToString(exp.Output)
		format := s.detectOutputFormat(outputStr)
		formatPatterns[format]++
	}

	for format, count := range formatPatterns {
		if count >= s.config.MinPatternOccurrences {
			patterns = append(patterns, &ExtractedPattern{
				ID:          fmt.Sprintf("pattern-output-format-%s-%s", taskType, format),
				TaskType:    taskType,
				PatternType: PatternTypeOutput,
				Pattern:     format,
				Description: fmt.Sprintf("Successful outputs use %s format", format),
				Occurrences: count,
				FirstSeen:   s.earliestTimestamp(successful),
				LastSeen:    s.latestTimestamp(successful),
			})
		}
	}

	return patterns
}

// extractToolPatterns extracts tool usage patterns.
func (s *PatternExtractionStrategy) extractToolPatterns(
	taskType string,
	successful []*selfimprove.Experience,
) []*ExtractedPattern {
	var patterns []*ExtractedPattern

	// Extract tool sequences
	seqFreq := make(map[string]int)
	seqExamples := make(map[string][]string)

	for _, exp := range successful {
		if len(exp.ToolsUsed) > 0 {
			seq := strings.Join(exp.ToolsUsed, " -> ")
			seqFreq[seq]++
			if len(seqExamples[seq]) < 3 {
				seqExamples[seq] = append(seqExamples[seq], interfaceToString(exp.Input))
			}
		}
	}

	for seq, count := range seqFreq {
		if count >= s.config.MinPatternOccurrences {
			tools := strings.Split(seq, " -> ")
			patterns = append(patterns, &ExtractedPattern{
				ID:           fmt.Sprintf("pattern-tools-%s-%d", taskType, len(patterns)),
				TaskType:     taskType,
				PatternType:  PatternTypeTool,
				Pattern:      seq,
				Description:  fmt.Sprintf("Successful tool sequence: %s", seq),
				Examples:     seqExamples[seq],
				ToolSequence: tools,
				Occurrences:  count,
				FirstSeen:    s.earliestTimestamp(successful),
				LastSeen:     s.latestTimestamp(successful),
			})
		}
	}

	return patterns
}

// patternToProposal converts an extracted pattern to an improvement proposal.
func (s *PatternExtractionStrategy) patternToProposal(
	agentID string,
	pattern *ExtractedPattern,
) *selfimprove.ImprovementProposal {
	if pattern.Confidence < 0.7 {
		return nil
	}

	var improvementType selfimprove.ImprovementType
	var proposedValue string

	switch pattern.PatternType {
	case PatternTypeInput:
		improvementType = selfimprove.ImprovementTypeSystemPrompt
		proposedValue = s.generateInputGuidance(pattern)
	case PatternTypeOutput:
		improvementType = selfimprove.ImprovementTypeSystemPrompt
		proposedValue = s.generateOutputGuidance(pattern)
	case PatternTypeTool:
		improvementType = selfimprove.ImprovementTypeToolConfig
		proposedValue = s.generateToolGuidance(pattern)
	default:
		return nil
	}

	return &selfimprove.ImprovementProposal{
		ID:              fmt.Sprintf("proposal-%s", pattern.ID),
		Strategy:        selfimprove.StrategyPatternExtraction,
		AgentID:         agentID,
		ImprovementType: improvementType,
		Description:     pattern.Description,
		ProposedValue:   proposedValue,
		Confidence:      pattern.Confidence,
		Evidence:        pattern.Examples,
		SampleSize:      pattern.Occurrences,
		Status:          selfimprove.ProposalStatusPending,
		CreatedAt:       time.Now(),
	}
}

// generateInputGuidance generates guidance from input patterns.
func (s *PatternExtractionStrategy) generateInputGuidance(pattern *ExtractedPattern) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("For %s tasks:\n", pattern.TaskType))

	if len(pattern.Keywords) > 0 {
		sb.WriteString("- Look for key terms: ")
		sb.WriteString(strings.Join(pattern.Keywords[:minInt(5, len(pattern.Keywords))], ", "))
		sb.WriteString("\n")
	}

	if pattern.Pattern != "" && pattern.PatternType == PatternTypeInput {
		sb.WriteString(fmt.Sprintf("- Pattern: %s\n", pattern.Description))
	}

	return sb.String()
}

// generateOutputGuidance generates guidance from output patterns.
func (s *PatternExtractionStrategy) generateOutputGuidance(pattern *ExtractedPattern) string {
	return fmt.Sprintf("For %s tasks, format output as: %s\nThis format has %.0f%% success rate.",
		pattern.TaskType, pattern.Pattern, pattern.SuccessRate*100)
}

// generateToolGuidance generates guidance from tool patterns.
func (s *PatternExtractionStrategy) generateToolGuidance(pattern *ExtractedPattern) string {
	return fmt.Sprintf("For %s tasks, consider this tool sequence: %s\nUsed successfully in %d cases with %.0f%% success rate.",
		pattern.TaskType, strings.Join(pattern.ToolSequence, " -> "),
		pattern.Occurrences, pattern.SuccessRate*100)
}

// calculatePatternSuccessRate calculates success rate for a pattern.
func (s *PatternExtractionStrategy) calculatePatternSuccessRate(
	pattern *ExtractedPattern,
	successful, failed []*selfimprove.Experience,
) float64 {
	matchSuccessful := 0
	matchFailed := 0

	for _, exp := range successful {
		if s.matchesPattern(pattern, exp) {
			matchSuccessful++
		}
	}
	for _, exp := range failed {
		if s.matchesPattern(pattern, exp) {
			matchFailed++
		}
	}

	total := matchSuccessful + matchFailed
	if total == 0 {
		return 0
	}

	return float64(matchSuccessful) / float64(total)
}

// calculatePatternConfidence calculates confidence in a pattern.
func (s *PatternExtractionStrategy) calculatePatternConfidence(pattern *ExtractedPattern) float64 {
	confidence := 0.5 // Base confidence

	// More occurrences increase confidence
	if pattern.Occurrences >= 20 {
		confidence += 0.3
	} else if pattern.Occurrences >= 10 {
		confidence += 0.2
	} else if pattern.Occurrences >= 5 {
		confidence += 0.1
	}

	// Higher success rate increases confidence
	confidence += pattern.SuccessRate * 0.2

	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// matchesPattern checks if an experience matches a pattern.
func (s *PatternExtractionStrategy) matchesPattern(
	pattern *ExtractedPattern,
	exp *selfimprove.Experience,
) bool {
	switch pattern.PatternType {
	case PatternTypeInput:
		return s.matchesKeywords(interfaceToString(exp.Input), pattern.Keywords)
	case PatternTypeOutput:
		return s.detectOutputFormat(interfaceToString(exp.Output)) == pattern.Pattern
	case PatternTypeTool:
		return s.matchesToolSequence(exp.ToolsUsed, pattern.ToolSequence)
	}
	return false
}

// matchesKeywords checks if text contains keywords.
func (s *PatternExtractionStrategy) matchesKeywords(text string, keywords []string) bool {
	textLower := strings.ToLower(text)
	matchCount := 0
	for _, kw := range keywords {
		if strings.Contains(textLower, strings.ToLower(kw)) {
			matchCount++
		}
	}
	return matchCount >= len(keywords)/2
}

// matchesToolSequence checks if tools match a sequence.
func (s *PatternExtractionStrategy) matchesToolSequence(tools, sequence []string) bool {
	if len(tools) < len(sequence) {
		return false
	}
	for i, tool := range sequence {
		if i >= len(tools) || tools[i] != tool {
			return false
		}
	}
	return true
}

// tokenize splits text into tokens.
func (s *PatternExtractionStrategy) tokenize(text string) []string {
	re := regexp.MustCompile(`\w+`)
	return re.FindAllString(strings.ToLower(text), -1)
}

// detectOutputFormat detects the format of an output.
func (s *PatternExtractionStrategy) detectOutputFormat(output string) string {
	output = strings.TrimSpace(output)

	// Check for JSON
	if strings.HasPrefix(output, "{") || strings.HasPrefix(output, "[") {
		return "json"
	}

	// Check for markdown
	if strings.Contains(output, "```") || strings.Contains(output, "##") {
		return "markdown"
	}

	// Check for list format
	if strings.HasPrefix(output, "- ") || strings.HasPrefix(output, "1.") {
		return "list"
	}

	// Check for structured format
	if strings.Contains(output, ":") && strings.Contains(output, "\n") {
		return "structured"
	}

	return "plain"
}

type structuralPattern struct {
	pattern     string
	description string
	examples    []string
	count       int
}

// extractStructuralPatterns extracts structural patterns from inputs.
func (s *PatternExtractionStrategy) extractStructuralPatterns(
	experiences []*selfimprove.Experience,
) []structuralPattern {
	var patterns []structuralPattern

	// Detect question patterns
	questionCount := 0
	commandCount := 0
	descriptionCount := 0

	for _, exp := range experiences {
		input := strings.TrimSpace(interfaceToString(exp.Input))
		if strings.HasSuffix(input, "?") {
			questionCount++
		} else if s.isCommandLike(input) {
			commandCount++
		} else {
			descriptionCount++
		}
	}

	if questionCount >= s.config.MinPatternOccurrences {
		patterns = append(patterns, structuralPattern{
			pattern:     "question",
			description: "Inputs are phrased as questions",
			count:       questionCount,
		})
	}

	if commandCount >= s.config.MinPatternOccurrences {
		patterns = append(patterns, structuralPattern{
			pattern:     "command",
			description: "Inputs are command-like instructions",
			count:       commandCount,
		})
	}

	return patterns
}

// isCommandLike checks if text looks like a command.
func (s *PatternExtractionStrategy) isCommandLike(text string) bool {
	commandStarts := []string{
		"create", "make", "build", "generate", "write",
		"fix", "update", "delete", "remove", "add",
		"run", "execute", "start", "stop", "deploy",
	}

	textLower := strings.ToLower(text)
	for _, cmd := range commandStarts {
		if strings.HasPrefix(textLower, cmd) {
			return true
		}
	}
	return false
}

// earliestTimestamp finds the earliest timestamp in experiences.
func (s *PatternExtractionStrategy) earliestTimestamp(experiences []*selfimprove.Experience) time.Time {
	if len(experiences) == 0 {
		return time.Now()
	}
	earliest := experiences[0].Timestamp
	for _, exp := range experiences {
		if exp.Timestamp.Before(earliest) {
			earliest = exp.Timestamp
		}
	}
	return earliest
}

// latestTimestamp finds the latest timestamp in experiences.
func (s *PatternExtractionStrategy) latestTimestamp(experiences []*selfimprove.Experience) time.Time {
	if len(experiences) == 0 {
		return time.Now()
	}
	latest := experiences[0].Timestamp
	for _, exp := range experiences {
		if exp.Timestamp.After(latest) {
			latest = exp.Timestamp
		}
	}
	return latest
}

// GetPatterns returns extracted patterns for a task type.
func (s *PatternExtractionStrategy) GetPatterns(taskType string) []*ExtractedPattern {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.patterns[taskType]
}

// GetAllPatterns returns all extracted patterns.
func (s *PatternExtractionStrategy) GetAllPatterns() map[string][]*ExtractedPattern {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string][]*ExtractedPattern)
	for k, v := range s.patterns {
		result[k] = v
	}
	return result
}

// GetStats returns pattern extraction statistics.
func (s *PatternExtractionStrategy) GetStats() *PatternStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// interfaceToString converts an interface{} to string.
func interfaceToString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
