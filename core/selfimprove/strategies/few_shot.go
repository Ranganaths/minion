package strategies

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Ranganaths/minion/core/selfimprove"
)

// FewShotStrategy uses successful examples as few-shot prompts to improve future responses.
type FewShotStrategy struct {
	*BaseStrategy

	config          *selfimprove.FewShotConfig
	experienceStore selfimprove.ExperienceStore
	formatter       selfimprove.ExperienceFormatter

	// Current few-shot examples per agent
	currentExamples map[string][]*selfimprove.Experience
}

// NewFewShotStrategy creates a new few-shot learning strategy.
func NewFewShotStrategy(
	config *selfimprove.FewShotConfig,
	experienceStore selfimprove.ExperienceStore,
) *FewShotStrategy {
	if config == nil {
		config = selfimprove.DefaultFewShotConfig()
	}

	return &FewShotStrategy{
		BaseStrategy:    NewBaseStrategy("", nil),
		config:          config,
		experienceStore: experienceStore,
		formatter:       selfimprove.NewDefaultExperienceFormatter(),
		currentExamples: make(map[string][]*selfimprove.Experience),
	}
}

// WithFormatter sets a custom experience formatter.
func (s *FewShotStrategy) WithFormatter(formatter selfimprove.ExperienceFormatter) *FewShotStrategy {
	s.formatter = formatter
	return s
}

// Name returns the strategy identifier.
func (s *FewShotStrategy) Name() selfimprove.LearningStrategy {
	return selfimprove.StrategyFewShot
}

// Analyze examines experiences and proposes few-shot example updates.
func (s *FewShotStrategy) Analyze(ctx context.Context, experiences []*selfimprove.Experience) (*selfimprove.ImprovementProposal, error) {
	if len(experiences) == 0 {
		return nil, nil
	}

	// Get agent ID from first experience
	agentID := experiences[0].AgentID

	// Filter for high-quality successful experiences
	successful := FilterSuccessful(experiences, s.config.MinScore)
	if len(successful) == 0 {
		return nil, nil
	}

	// Select diverse, representative examples
	selected := s.selectBestExamples(successful)
	if len(selected) == 0 {
		return nil, nil
	}

	// Check if this is different from current examples
	currentExamples := s.currentExamples[agentID]
	if areSameExamples(currentExamples, selected) {
		return nil, nil // No change needed
	}

	// Format the examples
	fewShotSection := s.formatter.FormatAsFewShot(selected)

	// Calculate expected improvement
	currentAvg := CalculateAverageScore(currentExamples)
	selectedAvg := CalculateAverageScore(selected)
	expectedImprovement := (selectedAvg - currentAvg) * 0.5 // Conservative estimate

	// Build rationale
	rationale := s.buildRationale(selected, currentExamples)

	// Generate unique ID
	proposalID := fmt.Sprintf("few-shot-%s-%d", agentID, time.Now().UnixNano())

	return &selfimprove.ImprovementProposal{
		ID:                    proposalID,
		Strategy:              selfimprove.StrategyFewShot,
		AgentID:               agentID,
		ImprovementType:       selfimprove.ImprovementTypeFewShotExamples,
		CurrentValue:          s.formatter.FormatAsFewShot(currentExamples),
		ProposedValue:         fewShotSection,
		Rationale:             rationale,
		SupportingExperiences: ExtractExperienceIDs(selected),
		ExpectedImprovement:   expectedImprovement,
		Confidence:            CalculateConfidence(selected),
		Status:                selfimprove.ProposalStatusPending,
		Priority:              calculatePriority(expectedImprovement, len(selected)),
		CreatedAt:             time.Now(),
		Metadata: map[string]interface{}{
			"num_examples":      len(selected),
			"avg_score":         selectedAvg,
			"task_types":        getTaskTypes(selected),
			"previous_examples": len(currentExamples),
		},
	}, nil
}

// Apply implements the few-shot improvement by storing the new examples.
func (s *FewShotStrategy) Apply(ctx context.Context, proposal *selfimprove.ImprovementProposal) error {
	if proposal.Strategy != selfimprove.StrategyFewShot {
		return fmt.Errorf("invalid strategy: expected %s, got %s",
			selfimprove.StrategyFewShot, proposal.Strategy)
	}

	// Retrieve the experiences from the proposal
	var examples []*selfimprove.Experience
	for _, expID := range proposal.SupportingExperiences {
		exp, err := s.experienceStore.Get(ctx, expID)
		if err != nil {
			continue // Skip missing experiences
		}
		examples = append(examples, exp)
	}

	// Update current examples for this agent
	s.currentExamples[proposal.AgentID] = examples

	return nil
}

// Configure updates the strategy configuration.
func (s *FewShotStrategy) Configure(config interface{}) error {
	if cfg, ok := config.(*selfimprove.FewShotConfig); ok {
		s.config = cfg
		return nil
	}
	return fmt.Errorf("invalid config type: expected *FewShotConfig")
}

// GetCurrentExamples returns the current few-shot examples for an agent.
func (s *FewShotStrategy) GetCurrentExamples(agentID string) []*selfimprove.Experience {
	return s.currentExamples[agentID]
}

// GetFormattedExamples returns formatted few-shot examples for an agent.
func (s *FewShotStrategy) GetFormattedExamples(agentID string) string {
	examples := s.currentExamples[agentID]
	if len(examples) == 0 {
		return ""
	}
	return s.formatter.FormatAsFewShot(examples)
}

// selectBestExamples selects the best diverse examples from successful experiences.
func (s *FewShotStrategy) selectBestExamples(experiences []*selfimprove.Experience) []*selfimprove.Experience {
	if len(experiences) <= s.config.NumExamples {
		return experiences
	}

	// Group by task type
	groups := GroupByTaskType(experiences)

	// Sort each group by score (descending)
	for taskType := range groups {
		sort.Slice(groups[taskType], func(i, j int) bool {
			return groups[taskType][i].Score > groups[taskType][j].Score
		})
	}

	// Calculate how many examples we want vs how many task types
	numExamples := s.config.NumExamples
	numTypes := len(groups)

	if numTypes == 0 {
		return nil
	}

	// Determine examples per type
	examplesPerType := numExamples / numTypes
	if examplesPerType == 0 {
		examplesPerType = 1
	}

	// Apply diversity weight - balance between top performers and task variety
	diversityCount := int(float64(numExamples) * s.config.DiversityWeight)
	topCount := numExamples - diversityCount

	// First, select top performers overall
	var allSorted []*selfimprove.Experience
	for _, exps := range groups {
		allSorted = append(allSorted, exps...)
	}
	sort.Slice(allSorted, func(i, j int) bool {
		return allSorted[i].Score > allSorted[j].Score
	})

	selected := make(map[string]bool)
	var result []*selfimprove.Experience

	// Add top performers
	for i := 0; i < topCount && i < len(allSorted); i++ {
		exp := allSorted[i]
		if !selected[exp.ID] {
			result = append(result, exp)
			selected[exp.ID] = true
		}
	}

	// Add diverse examples from each task type
	for len(result) < numExamples {
		added := false
		for _, exps := range groups {
			if len(result) >= numExamples {
				break
			}
			for _, exp := range exps {
				if !selected[exp.ID] {
					result = append(result, exp)
					selected[exp.ID] = true
					added = true
					break
				}
			}
		}
		if !added {
			break // No more examples to add
		}
	}

	return result
}

// buildRationale generates a human-readable explanation for the proposal.
func (s *FewShotStrategy) buildRationale(selected, current []*selfimprove.Experience) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Selected %d high-quality examples for few-shot prompting",
		len(selected)))

	avgScore := CalculateAverageScore(selected)
	parts = append(parts, fmt.Sprintf("Average score of selected examples: %.2f", avgScore))

	// Describe task type coverage
	taskTypes := getTaskTypes(selected)
	if len(taskTypes) > 1 {
		parts = append(parts, fmt.Sprintf("Covers %d task types: %s",
			len(taskTypes), strings.Join(taskTypes, ", ")))
	} else if len(taskTypes) == 1 {
		parts = append(parts, fmt.Sprintf("Task type: %s", taskTypes[0]))
	}

	// Compare with current
	if len(current) > 0 {
		currentAvg := CalculateAverageScore(current)
		improvement := avgScore - currentAvg
		if improvement > 0 {
			parts = append(parts, fmt.Sprintf("Improvement over current examples: +%.2f score", improvement))
		}
	} else {
		parts = append(parts, "No previous few-shot examples - this is the initial selection")
	}

	return strings.Join(parts, ". ")
}

// Helper functions

func areSameExamples(a, b []*selfimprove.Experience) bool {
	if len(a) != len(b) {
		return false
	}

	aIDs := make(map[string]bool)
	for _, exp := range a {
		aIDs[exp.ID] = true
	}

	for _, exp := range b {
		if !aIDs[exp.ID] {
			return false
		}
	}

	return true
}

func getTaskTypes(experiences []*selfimprove.Experience) []string {
	types := make(map[string]bool)
	for _, exp := range experiences {
		if exp.TaskType != "" {
			types[exp.TaskType] = true
		}
	}

	result := make([]string, 0, len(types))
	for t := range types {
		result = append(result, t)
	}
	sort.Strings(result)
	return result
}

func calculatePriority(expectedImprovement float64, numExamples int) int {
	// Base priority on expected improvement and evidence strength
	priority := int(expectedImprovement * 100)

	// Boost priority with more examples
	if numExamples >= 5 {
		priority += 10
	} else if numExamples >= 3 {
		priority += 5
	}

	return priority
}
