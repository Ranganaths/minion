package strategies

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Ranganaths/minion/core/selfimprove"
)

// PromptRefinementStrategy uses LLM to analyze failures and refine prompts.
// It compares successful and failed executions to identify patterns and
// generate improved prompts.
type PromptRefinementStrategy struct {
	*BaseStrategy

	config      *selfimprove.PromptRefinementConfig
	llmProvider LLMProvider

	// Track prompt versions per agent
	promptVersions map[string][]*PromptVersion
}

// PromptVersion tracks a prompt and its performance.
type PromptVersion struct {
	Version     string    `json:"version"`
	Prompt      string    `json:"prompt"`
	CreatedAt   time.Time `json:"created_at"`
	Executions  int       `json:"executions"`
	AvgScore    float64   `json:"avg_score"`
	SuccessRate float64   `json:"success_rate"`
}

// NewPromptRefinementStrategy creates a new prompt refinement strategy.
func NewPromptRefinementStrategy(
	config *selfimprove.PromptRefinementConfig,
	llmProvider LLMProvider,
) *PromptRefinementStrategy {
	if config == nil {
		config = selfimprove.DefaultPromptRefinementConfig()
	}

	return &PromptRefinementStrategy{
		BaseStrategy:   NewBaseStrategy("", nil),
		config:         config,
		llmProvider:    llmProvider,
		promptVersions: make(map[string][]*PromptVersion),
	}
}

// Name returns the strategy identifier.
func (s *PromptRefinementStrategy) Name() selfimprove.LearningStrategy {
	return selfimprove.StrategyPromptRefinement
}

// Analyze examines experiences and proposes prompt refinements.
func (s *PromptRefinementStrategy) Analyze(ctx context.Context, experiences []*selfimprove.Experience) (*selfimprove.ImprovementProposal, error) {
	if len(experiences) == 0 || s.llmProvider == nil {
		return nil, nil
	}

	// Get agent ID from first experience
	agentID := experiences[0].AgentID

	// Separate successes and failures
	successes := FilterSuccessful(experiences, 0.7)
	failures := FilterFailed(experiences, 0.5)

	// Check minimum thresholds
	if len(failures) < s.config.MinFailures {
		return nil, nil // Not enough failures to analyze
	}
	if len(successes) < s.config.MinSuccesses {
		return nil, nil // Not enough successes for comparison
	}

	// Get current prompt from most recent experience
	currentPrompt := extractCurrentPrompt(experiences)
	if currentPrompt == "" {
		return nil, nil // No prompt to refine
	}

	// Build analysis prompt
	analysisPrompt := s.buildAnalysisPrompt(successes, failures, currentPrompt)

	// Call LLM to analyze and suggest refinements
	response, err := s.llmProvider.GenerateCompletion(ctx, &CompletionRequest{
		SystemPrompt: systemPromptForAnalysis,
		UserPrompt:   analysisPrompt,
		Temperature:  s.config.Temperature,
		MaxTokens:    2000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to analyze failures: %w", err)
	}

	// Parse the analysis to extract refined prompt
	refinedPrompt, rationale := s.parseAnalysisResponse(response.Text, currentPrompt)
	if refinedPrompt == "" || refinedPrompt == currentPrompt {
		return nil, nil // No meaningful refinement suggested
	}

	// Calculate expected improvement based on failure patterns
	expectedImprovement := s.estimateImprovement(successes, failures)

	// Generate unique ID
	proposalID := fmt.Sprintf("prompt-refine-%s-%d", agentID, time.Now().UnixNano())

	return &selfimprove.ImprovementProposal{
		ID:                    proposalID,
		Strategy:              selfimprove.StrategyPromptRefinement,
		AgentID:               agentID,
		ImprovementType:       selfimprove.ImprovementTypeSystemPrompt,
		CurrentValue:          currentPrompt,
		ProposedValue:         refinedPrompt,
		Rationale:             rationale,
		SupportingExperiences: append(ExtractExperienceIDs(successes), ExtractExperienceIDs(failures)...),
		ExpectedImprovement:   expectedImprovement,
		Confidence:            s.calculateConfidence(successes, failures),
		Status:                selfimprove.ProposalStatusPending,
		Priority:              s.calculatePriority(failures, expectedImprovement),
		CreatedAt:             time.Now(),
		Metadata: map[string]interface{}{
			"num_successes":       len(successes),
			"num_failures":        len(failures),
			"avg_success_score":   CalculateAverageScore(successes),
			"avg_failure_score":   CalculateAverageScore(failures),
			"analysis_tokens":     response.TokensUsed,
		},
	}, nil
}

// Apply implements the prompt refinement by updating the agent's prompt.
func (s *PromptRefinementStrategy) Apply(ctx context.Context, proposal *selfimprove.ImprovementProposal) error {
	if proposal.Strategy != selfimprove.StrategyPromptRefinement {
		return fmt.Errorf("invalid strategy: expected %s, got %s",
			selfimprove.StrategyPromptRefinement, proposal.Strategy)
	}

	// Track the new prompt version
	version := &PromptVersion{
		Version:   fmt.Sprintf("v%d", len(s.promptVersions[proposal.AgentID])+1),
		Prompt:    proposal.ProposedValue,
		CreatedAt: time.Now(),
	}

	s.promptVersions[proposal.AgentID] = append(s.promptVersions[proposal.AgentID], version)

	return nil
}

// Configure updates the strategy configuration.
func (s *PromptRefinementStrategy) Configure(config interface{}) error {
	if cfg, ok := config.(*selfimprove.PromptRefinementConfig); ok {
		s.config = cfg
		return nil
	}
	return fmt.Errorf("invalid config type: expected *PromptRefinementConfig")
}

// GetPromptVersions returns prompt version history for an agent.
func (s *PromptRefinementStrategy) GetPromptVersions(agentID string) []*PromptVersion {
	return s.promptVersions[agentID]
}

// GetCurrentPromptVersion returns the current prompt version for an agent.
func (s *PromptRefinementStrategy) GetCurrentPromptVersion(agentID string) *PromptVersion {
	versions := s.promptVersions[agentID]
	if len(versions) == 0 {
		return nil
	}
	return versions[len(versions)-1]
}

// buildAnalysisPrompt constructs the prompt for failure analysis.
func (s *PromptRefinementStrategy) buildAnalysisPrompt(
	successes, failures []*selfimprove.Experience,
	currentPrompt string,
) string {
	var sb strings.Builder

	sb.WriteString("## Current System Prompt\n\n")
	sb.WriteString("```\n")
	sb.WriteString(currentPrompt)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Successful Executions (Score >= 0.7)\n\n")
	for i, exp := range limitExperiences(successes, 5) {
		sb.WriteString(fmt.Sprintf("### Success %d (Score: %.2f)\n", i+1, exp.Score))
		sb.WriteString(fmt.Sprintf("**Input:** %s\n", truncateString(formatInput(exp.Input), 500)))
		sb.WriteString(fmt.Sprintf("**Output:** %s\n\n", truncateString(formatOutput(exp.Output), 500)))
	}

	sb.WriteString("## Failed Executions (Score <= 0.5)\n\n")
	for i, exp := range limitExperiences(failures, 5) {
		sb.WriteString(fmt.Sprintf("### Failure %d (Score: %.2f)\n", i+1, exp.Score))
		sb.WriteString(fmt.Sprintf("**Input:** %s\n", truncateString(formatInput(exp.Input), 500)))
		sb.WriteString(fmt.Sprintf("**Output:** %s\n", truncateString(formatOutput(exp.Output), 500)))
		if exp.HumanFeedback != nil {
			sb.WriteString(fmt.Sprintf("**Feedback:** %s\n", *exp.HumanFeedback))
		}
		if exp.Correction != nil {
			sb.WriteString(fmt.Sprintf("**Expected:** %s\n", *exp.Correction))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Analysis Request\n\n")
	sb.WriteString("Based on the patterns above, analyze what causes failures and suggest specific improvements to the system prompt.\n\n")
	sb.WriteString("Respond with:\n")
	sb.WriteString("1. **ANALYSIS**: Brief analysis of failure patterns (2-3 sentences)\n")
	sb.WriteString("2. **REFINED_PROMPT**: The complete improved system prompt\n")

	return sb.String()
}

// parseAnalysisResponse extracts the refined prompt and rationale from LLM response.
func (s *PromptRefinementStrategy) parseAnalysisResponse(response, currentPrompt string) (refinedPrompt, rationale string) {
	// Look for ANALYSIS section
	analysisIdx := strings.Index(response, "**ANALYSIS**:")
	if analysisIdx == -1 {
		analysisIdx = strings.Index(response, "ANALYSIS:")
	}

	// Look for REFINED_PROMPT section
	promptIdx := strings.Index(response, "**REFINED_PROMPT**:")
	if promptIdx == -1 {
		promptIdx = strings.Index(response, "REFINED_PROMPT:")
	}

	// Extract rationale
	if analysisIdx != -1 {
		endIdx := promptIdx
		if endIdx == -1 {
			endIdx = len(response)
		}
		rationale = strings.TrimSpace(response[analysisIdx+len("**ANALYSIS**:") : endIdx])
		rationale = strings.TrimPrefix(rationale, ":")
		rationale = strings.TrimSpace(rationale)
	}

	// Extract refined prompt
	if promptIdx != -1 {
		refinedPrompt = strings.TrimSpace(response[promptIdx+len("**REFINED_PROMPT**:"):])
		refinedPrompt = strings.TrimPrefix(refinedPrompt, ":")
		refinedPrompt = strings.TrimSpace(refinedPrompt)

		// Remove markdown code blocks if present
		refinedPrompt = strings.TrimPrefix(refinedPrompt, "```")
		refinedPrompt = strings.TrimSuffix(refinedPrompt, "```")
		refinedPrompt = strings.TrimSpace(refinedPrompt)
	}

	// If no structured response, try to use the whole response as rationale
	if rationale == "" && refinedPrompt == "" {
		rationale = "LLM analysis did not follow expected format"
	}

	return refinedPrompt, rationale
}

// estimateImprovement estimates the expected score improvement.
func (s *PromptRefinementStrategy) estimateImprovement(successes, failures []*selfimprove.Experience) float64 {
	if len(successes) == 0 || len(failures) == 0 {
		return 0
	}

	avgSuccess := CalculateAverageScore(successes)
	avgFailure := CalculateAverageScore(failures)

	// Estimate improvement as a fraction of the gap
	gap := avgSuccess - avgFailure
	// Conservative estimate: we expect to close 30% of the gap
	return gap * 0.3
}

// calculateConfidence calculates confidence in the refinement.
func (s *PromptRefinementStrategy) calculateConfidence(successes, failures []*selfimprove.Experience) float64 {
	// Base confidence on sample sizes
	totalSamples := len(successes) + len(failures)
	sampleConfidence := 1.0 - (1.0 / float64(1+totalSamples/5))

	// Adjust based on score variance in failures
	failureVariance := calculateVariance(failures)
	varianceConfidence := 1.0 - (failureVariance * 2)
	if varianceConfidence < 0.3 {
		varianceConfidence = 0.3
	}

	// Combine confidences
	return (sampleConfidence + varianceConfidence) / 2
}

// calculatePriority determines proposal priority based on failure impact.
func (s *PromptRefinementStrategy) calculatePriority(failures []*selfimprove.Experience, expectedImprovement float64) int {
	// Base priority on number of failures and expected improvement
	priority := len(failures) * 5
	priority += int(expectedImprovement * 100)

	return priority
}

// Helper functions

func extractCurrentPrompt(experiences []*selfimprove.Experience) string {
	// Find most recent experience with a system prompt
	for i := len(experiences) - 1; i >= 0; i-- {
		if experiences[i].SystemPrompt != "" {
			return experiences[i].SystemPrompt
		}
	}
	return ""
}

func limitExperiences(experiences []*selfimprove.Experience, max int) []*selfimprove.Experience {
	if len(experiences) <= max {
		return experiences
	}
	return experiences[:max]
}

func formatInput(input interface{}) string {
	if input == nil {
		return ""
	}
	if s, ok := input.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", input)
}

func formatOutput(output interface{}) string {
	if output == nil {
		return ""
	}
	if s, ok := output.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", output)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func calculateVariance(experiences []*selfimprove.Experience) float64 {
	if len(experiences) == 0 {
		return 0
	}

	avg := CalculateAverageScore(experiences)
	var variance float64
	for _, exp := range experiences {
		diff := exp.Score - avg
		variance += diff * diff
	}
	return variance / float64(len(experiences))
}

const systemPromptForAnalysis = `You are an AI system optimizer specializing in prompt engineering.

Your task is to analyze execution patterns and improve system prompts.

Guidelines:
1. Identify specific patterns in failures that don't occur in successes
2. Look for missing context, unclear instructions, or ambiguous wording
3. Suggest concrete, actionable improvements
4. Keep the core functionality while addressing failure modes
5. Be specific about what changes you're making and why

Format your response as:
**ANALYSIS**: [2-3 sentence analysis of failure patterns]
**REFINED_PROMPT**: [The complete improved system prompt]`
