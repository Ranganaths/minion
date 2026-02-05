package strategies

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Ranganaths/minion/core/selfimprove"
)

// ReflectionStrategy has the agent reflect on its own failures to generate improvements.
// It uses the LLM to analyze what went wrong and propose specific changes.
type ReflectionStrategy struct {
	*BaseStrategy

	config      *selfimprove.ReflectionConfig
	llmProvider LLMProvider

	// Store reflections for analysis
	reflections map[string][]*Reflection
}

// Reflection represents a single reflection on a failure.
type Reflection struct {
	ExperienceID    string    `json:"experience_id"`
	AgentID         string    `json:"agent_id"`
	TaskType        string    `json:"task_type"`
	Score           float64   `json:"score"`
	WhatWentWrong   string    `json:"what_went_wrong"`
	MissingInfo     string    `json:"missing_info"`
	WouldDoDifferent string   `json:"would_do_different"`
	SuggestedChanges string   `json:"suggested_changes"`
	CreatedAt       time.Time `json:"created_at"`
}

// NewReflectionStrategy creates a new reflection strategy.
func NewReflectionStrategy(
	config *selfimprove.ReflectionConfig,
	llmProvider LLMProvider,
) *ReflectionStrategy {
	if config == nil {
		config = selfimprove.DefaultReflectionConfig()
	}

	return &ReflectionStrategy{
		BaseStrategy: NewBaseStrategy("", nil),
		config:       config,
		llmProvider:  llmProvider,
		reflections:  make(map[string][]*Reflection),
	}
}

// Name returns the strategy identifier.
func (s *ReflectionStrategy) Name() selfimprove.LearningStrategy {
	return selfimprove.StrategyReflection
}

// Analyze examines failures through agent reflection.
func (s *ReflectionStrategy) Analyze(ctx context.Context, experiences []*selfimprove.Experience) (*selfimprove.ImprovementProposal, error) {
	if len(experiences) == 0 || s.llmProvider == nil {
		return nil, nil
	}

	// Get agent ID from first experience
	agentID := experiences[0].AgentID

	// Filter for failures worth reflecting on
	failures := FilterFailed(experiences, s.config.MinScoreForReflection)
	if len(failures) == 0 {
		return nil, nil
	}

	// Limit batch size
	if len(failures) > s.config.MaxReflectionsPerBatch {
		failures = failures[:s.config.MaxReflectionsPerBatch]
	}

	// Generate reflections for each failure
	var reflections []*Reflection
	for _, exp := range failures {
		reflection, err := s.reflectOnFailure(ctx, exp)
		if err != nil {
			continue // Skip failures we can't reflect on
		}
		reflections = append(reflections, reflection)
	}

	if len(reflections) == 0 {
		return nil, nil
	}

	// Store reflections
	s.reflections[agentID] = append(s.reflections[agentID], reflections...)

	// Synthesize reflections into actionable improvements
	synthesis, err := s.synthesizeReflections(ctx, reflections)
	if err != nil {
		return nil, err
	}

	if synthesis.improvement == "" {
		return nil, nil
	}

	// Generate unique ID
	proposalID := fmt.Sprintf("reflection-%s-%d", agentID, time.Now().UnixNano())

	return &selfimprove.ImprovementProposal{
		ID:                    proposalID,
		Strategy:              selfimprove.StrategyReflection,
		AgentID:               agentID,
		ImprovementType:       selfimprove.ImprovementTypeSystemPrompt,
		CurrentValue:          extractCurrentPrompt(experiences),
		ProposedValue:         synthesis.improvement,
		Rationale:             synthesis.rationale,
		SupportingExperiences: extractReflectionExperienceIDs(reflections),
		ExpectedImprovement:   synthesis.expectedImprovement,
		Confidence:            synthesis.confidence,
		Status:                selfimprove.ProposalStatusPending,
		Priority:              len(reflections) * 10,
		CreatedAt:             time.Now(),
		Metadata: map[string]interface{}{
			"num_reflections":    len(reflections),
			"common_issues":      synthesis.commonIssues,
			"synthesis_tokens":   synthesis.tokensUsed,
		},
	}, nil
}

// Apply implements the reflection improvement.
func (s *ReflectionStrategy) Apply(ctx context.Context, proposal *selfimprove.ImprovementProposal) error {
	if proposal.Strategy != selfimprove.StrategyReflection {
		return fmt.Errorf("invalid strategy: expected %s, got %s",
			selfimprove.StrategyReflection, proposal.Strategy)
	}

	// The improvement is applied by updating the agent's prompt
	// This is typically done by the learning engine or agent wrapper
	return nil
}

// Configure updates the strategy configuration.
func (s *ReflectionStrategy) Configure(config interface{}) error {
	if cfg, ok := config.(*selfimprove.ReflectionConfig); ok {
		s.config = cfg
		return nil
	}
	return fmt.Errorf("invalid config type: expected *ReflectionConfig")
}

// GetReflections returns stored reflections for an agent.
func (s *ReflectionStrategy) GetReflections(agentID string) []*Reflection {
	return s.reflections[agentID]
}

// reflectOnFailure generates a reflection for a single failure.
func (s *ReflectionStrategy) reflectOnFailure(ctx context.Context, exp *selfimprove.Experience) (*Reflection, error) {
	prompt := s.buildReflectionPrompt(exp)

	response, err := s.llmProvider.GenerateCompletion(ctx, &CompletionRequest{
		SystemPrompt: reflectionSystemPrompt,
		UserPrompt:   prompt,
		Temperature:  s.config.Temperature,
		MaxTokens:    1000,
	})
	if err != nil {
		return nil, err
	}

	reflection := s.parseReflectionResponse(response.Text, exp)
	reflection.ExperienceID = exp.ID
	reflection.AgentID = exp.AgentID
	reflection.TaskType = exp.TaskType
	reflection.Score = exp.Score
	reflection.CreatedAt = time.Now()

	return reflection, nil
}

// buildReflectionPrompt creates the prompt for self-reflection.
func (s *ReflectionStrategy) buildReflectionPrompt(exp *selfimprove.Experience) string {
	var sb strings.Builder

	sb.WriteString("## Task That Did Not Meet Standards\n\n")
	sb.WriteString(fmt.Sprintf("**Task Type:** %s\n", exp.TaskType))
	sb.WriteString(fmt.Sprintf("**Score:** %.2f (threshold: %.2f)\n\n", exp.Score, s.config.MinScoreForReflection))

	sb.WriteString("**Original Input:**\n")
	sb.WriteString(truncateString(formatInput(exp.Input), 1000))
	sb.WriteString("\n\n")

	sb.WriteString("**Your Response:**\n")
	sb.WriteString(truncateString(formatOutput(exp.Output), 1000))
	sb.WriteString("\n\n")

	if exp.HumanFeedback != nil {
		sb.WriteString("**Human Feedback:**\n")
		sb.WriteString(*exp.HumanFeedback)
		sb.WriteString("\n\n")
	}

	if exp.Correction != nil {
		sb.WriteString("**Expected/Corrected Output:**\n")
		sb.WriteString(*exp.Correction)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Reflection Request\n\n")
	sb.WriteString("Reflect on this execution and provide:\n")
	sb.WriteString("1. **WHAT_WENT_WRONG**: What specifically did you misunderstand or do incorrectly?\n")
	sb.WriteString("2. **MISSING_INFO**: What information were you missing that led to the error?\n")
	sb.WriteString("3. **WOULD_DO_DIFFERENT**: What would you do differently next time?\n")
	sb.WriteString("4. **SUGGESTED_CHANGES**: What changes to your instructions would help prevent this?\n")

	return sb.String()
}

// parseReflectionResponse extracts structured reflection from LLM response.
func (s *ReflectionStrategy) parseReflectionResponse(response string, exp *selfimprove.Experience) *Reflection {
	reflection := &Reflection{}

	// Extract each section
	reflection.WhatWentWrong = extractSection(response, "WHAT_WENT_WRONG")
	reflection.MissingInfo = extractSection(response, "MISSING_INFO")
	reflection.WouldDoDifferent = extractSection(response, "WOULD_DO_DIFFERENT")
	reflection.SuggestedChanges = extractSection(response, "SUGGESTED_CHANGES")

	// If parsing failed, use the whole response as general feedback
	if reflection.WhatWentWrong == "" && reflection.SuggestedChanges == "" {
		reflection.WhatWentWrong = "Unable to parse structured reflection"
		reflection.SuggestedChanges = response
	}

	return reflection
}

// reflectionSynthesis holds the synthesized result of multiple reflections.
type reflectionSynthesis struct {
	improvement         string
	rationale           string
	commonIssues        []string
	expectedImprovement float64
	confidence          float64
	tokensUsed          int
}

// synthesizeReflections combines multiple reflections into actionable improvements.
func (s *ReflectionStrategy) synthesizeReflections(ctx context.Context, reflections []*Reflection) (*reflectionSynthesis, error) {
	if len(reflections) == 0 {
		return &reflectionSynthesis{}, nil
	}

	prompt := s.buildSynthesisPrompt(reflections)

	response, err := s.llmProvider.GenerateCompletion(ctx, &CompletionRequest{
		SystemPrompt: synthesisSystemPrompt,
		UserPrompt:   prompt,
		Temperature:  0.3,
		MaxTokens:    2000,
	})
	if err != nil {
		return nil, err
	}

	synthesis := s.parseSynthesisResponse(response.Text)
	synthesis.tokensUsed = response.TokensUsed
	synthesis.confidence = s.calculateSynthesisConfidence(reflections)
	synthesis.expectedImprovement = s.estimateSynthesisImprovement(reflections)

	return synthesis, nil
}

// buildSynthesisPrompt creates the prompt for synthesizing reflections.
func (s *ReflectionStrategy) buildSynthesisPrompt(reflections []*Reflection) string {
	var sb strings.Builder

	sb.WriteString("## Reflections on Past Failures\n\n")

	for i, ref := range reflections {
		sb.WriteString(fmt.Sprintf("### Reflection %d (Task: %s, Score: %.2f)\n", i+1, ref.TaskType, ref.Score))
		sb.WriteString(fmt.Sprintf("**What went wrong:** %s\n", ref.WhatWentWrong))
		sb.WriteString(fmt.Sprintf("**Missing info:** %s\n", ref.MissingInfo))
		sb.WriteString(fmt.Sprintf("**Would do different:** %s\n", ref.WouldDoDifferent))
		sb.WriteString(fmt.Sprintf("**Suggested changes:** %s\n\n", ref.SuggestedChanges))
	}

	sb.WriteString("## Synthesis Request\n\n")
	sb.WriteString("Analyze these reflections and provide:\n")
	sb.WriteString("1. **COMMON_ISSUES**: List the most common failure patterns (comma-separated)\n")
	sb.WriteString("2. **ROOT_CAUSES**: Identify root causes across reflections\n")
	sb.WriteString("3. **IMPROVEMENT**: Specific prompt additions or modifications to address these issues\n")

	return sb.String()
}

// parseSynthesisResponse extracts the synthesis from LLM response.
func (s *ReflectionStrategy) parseSynthesisResponse(response string) *reflectionSynthesis {
	synthesis := &reflectionSynthesis{}

	// Extract common issues
	commonIssuesStr := extractSection(response, "COMMON_ISSUES")
	if commonIssuesStr != "" {
		issues := strings.Split(commonIssuesStr, ",")
		for _, issue := range issues {
			issue = strings.TrimSpace(issue)
			if issue != "" {
				synthesis.commonIssues = append(synthesis.commonIssues, issue)
			}
		}
	}

	// Extract root causes as rationale
	synthesis.rationale = extractSection(response, "ROOT_CAUSES")

	// Extract improvement
	synthesis.improvement = extractSection(response, "IMPROVEMENT")

	return synthesis
}

// calculateSynthesisConfidence determines confidence based on reflection consistency.
func (s *ReflectionStrategy) calculateSynthesisConfidence(reflections []*Reflection) float64 {
	if len(reflections) == 0 {
		return 0
	}

	// Base confidence on number of reflections
	countConfidence := 1.0 - (1.0 / float64(1+len(reflections)))

	// Check for consistency in suggested changes
	changePatterns := make(map[string]int)
	for _, ref := range reflections {
		// Simple word matching for pattern detection
		words := strings.Fields(strings.ToLower(ref.SuggestedChanges))
		for _, word := range words {
			if len(word) > 4 { // Only count significant words
				changePatterns[word]++
			}
		}
	}

	// More repeated patterns = higher confidence
	var patternScore float64
	for _, count := range changePatterns {
		if count > 1 {
			patternScore += float64(count) / float64(len(reflections))
		}
	}
	patternConfidence := minFloat(patternScore/5, 1.0)

	return (countConfidence + patternConfidence) / 2
}

// estimateSynthesisImprovement estimates expected improvement from reflections.
func (s *ReflectionStrategy) estimateSynthesisImprovement(reflections []*Reflection) float64 {
	if len(reflections) == 0 {
		return 0
	}

	// Average score of failures
	var avgScore float64
	for _, ref := range reflections {
		avgScore += ref.Score
	}
	avgScore /= float64(len(reflections))

	// Estimate we can improve to 0.7 from current average
	// Conservative estimate of closing 40% of the gap
	gap := 0.7 - avgScore
	return gap * 0.4
}

// Helper functions

func extractSection(text, sectionName string) string {
	// Try with ** markers
	markerStart := fmt.Sprintf("**%s**:", sectionName)
	idx := strings.Index(text, markerStart)
	if idx == -1 {
		// Try without ** markers
		markerStart = fmt.Sprintf("%s:", sectionName)
		idx = strings.Index(text, markerStart)
	}

	if idx == -1 {
		return ""
	}

	// Find the end (next section or end of text)
	start := idx + len(markerStart)
	content := text[start:]

	// Find next section marker
	nextIdx := -1
	for _, marker := range []string{"**WHAT_WENT_WRONG**", "**MISSING_INFO**", "**WOULD_DO_DIFFERENT**", "**SUGGESTED_CHANGES**", "**COMMON_ISSUES**", "**ROOT_CAUSES**", "**IMPROVEMENT**"} {
		if marker != "**"+sectionName+"**" {
			if i := strings.Index(content, marker); i != -1 && (nextIdx == -1 || i < nextIdx) {
				nextIdx = i
			}
		}
	}

	if nextIdx != -1 {
		content = content[:nextIdx]
	}

	return strings.TrimSpace(content)
}

func extractReflectionExperienceIDs(reflections []*Reflection) []string {
	ids := make([]string, len(reflections))
	for i, ref := range reflections {
		ids[i] = ref.ExperienceID
	}
	return ids
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

const reflectionSystemPrompt = `You are reviewing a past execution that did not meet quality standards.

Your task is to honestly reflect on what went wrong and how to improve.

Be specific and actionable in your analysis. Focus on:
- Concrete errors in understanding or execution
- Specific information gaps
- Practical changes that would help

Format your response with the requested sections.`

const synthesisSystemPrompt = `You are an AI improvement analyst.

Your task is to synthesize multiple self-reflections into actionable improvements.

Focus on:
- Identifying patterns across failures
- Finding root causes, not just symptoms
- Proposing specific, implementable changes

Be concise and practical in your recommendations.`
