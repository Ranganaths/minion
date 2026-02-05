// Package strategies provides learning strategy implementations for self-improving agents.
package strategies

import (
	"context"

	"github.com/Ranganaths/minion/core/selfimprove"
)

// Strategy defines the interface for learning strategies.
// Each strategy analyzes experiences and proposes improvements.
type Strategy interface {
	// Name returns the strategy identifier
	Name() selfimprove.LearningStrategy

	// Analyze examines experiences and proposes improvements
	Analyze(ctx context.Context, experiences []*selfimprove.Experience) (*selfimprove.ImprovementProposal, error)

	// Apply implements an approved improvement
	Apply(ctx context.Context, proposal *selfimprove.ImprovementProposal) error

	// IsApplicable checks if this strategy can be used for the given context
	IsApplicable(ctx context.Context, agentID string, taskType string) bool

	// Configure updates the strategy configuration
	Configure(config interface{}) error
}

// BaseStrategy provides common functionality for strategy implementations.
type BaseStrategy struct {
	agentID   string
	taskTypes map[string]bool
}

// NewBaseStrategy creates a new base strategy.
func NewBaseStrategy(agentID string, taskTypes []string) *BaseStrategy {
	types := make(map[string]bool)
	for _, t := range taskTypes {
		types[t] = true
	}
	return &BaseStrategy{
		agentID:   agentID,
		taskTypes: types,
	}
}

// IsApplicable checks if the strategy applies to the given context.
func (s *BaseStrategy) IsApplicable(ctx context.Context, agentID string, taskType string) bool {
	// If agentID is set, it must match
	if s.agentID != "" && s.agentID != agentID {
		return false
	}

	// If task types are specified, the task type must be in the list
	if len(s.taskTypes) > 0 && taskType != "" {
		return s.taskTypes[taskType]
	}

	return true
}

// FilterByScore filters experiences by score range.
func FilterByScore(experiences []*selfimprove.Experience, minScore, maxScore float64) []*selfimprove.Experience {
	var filtered []*selfimprove.Experience
	for _, exp := range experiences {
		if exp.Score >= minScore && exp.Score <= maxScore {
			filtered = append(filtered, exp)
		}
	}
	return filtered
}

// FilterSuccessful filters for successful experiences above a threshold.
func FilterSuccessful(experiences []*selfimprove.Experience, minScore float64) []*selfimprove.Experience {
	var filtered []*selfimprove.Experience
	for _, exp := range experiences {
		if exp.Success && exp.Score >= minScore {
			filtered = append(filtered, exp)
		}
	}
	return filtered
}

// FilterFailed filters for failed experiences below a threshold.
func FilterFailed(experiences []*selfimprove.Experience, maxScore float64) []*selfimprove.Experience {
	var filtered []*selfimprove.Experience
	for _, exp := range experiences {
		if !exp.Success || exp.Score <= maxScore {
			filtered = append(filtered, exp)
		}
	}
	return filtered
}

// FilterByTaskType filters experiences by task type.
func FilterByTaskType(experiences []*selfimprove.Experience, taskType string) []*selfimprove.Experience {
	var filtered []*selfimprove.Experience
	for _, exp := range experiences {
		if exp.TaskType == taskType {
			filtered = append(filtered, exp)
		}
	}
	return filtered
}

// GroupByTaskType groups experiences by task type.
func GroupByTaskType(experiences []*selfimprove.Experience) map[string][]*selfimprove.Experience {
	groups := make(map[string][]*selfimprove.Experience)
	for _, exp := range experiences {
		groups[exp.TaskType] = append(groups[exp.TaskType], exp)
	}
	return groups
}

// CalculateAverageScore calculates the average score of experiences.
func CalculateAverageScore(experiences []*selfimprove.Experience) float64 {
	if len(experiences) == 0 {
		return 0
	}
	var sum float64
	for _, exp := range experiences {
		sum += exp.Score
	}
	return sum / float64(len(experiences))
}

// CalculateConfidence calculates confidence based on sample size and variance.
func CalculateConfidence(experiences []*selfimprove.Experience) float64 {
	if len(experiences) == 0 {
		return 0
	}

	// Base confidence on sample size (diminishing returns after 50)
	sampleConfidence := 1.0 - (1.0 / float64(1+len(experiences)/10))

	// Adjust based on score variance
	avg := CalculateAverageScore(experiences)
	var variance float64
	for _, exp := range experiences {
		diff := exp.Score - avg
		variance += diff * diff
	}
	variance /= float64(len(experiences))

	// Lower variance = higher confidence
	varianceConfidence := 1.0 - (variance * 2) // variance is 0-0.25 for 0-1 scores
	if varianceConfidence < 0 {
		varianceConfidence = 0
	}

	// Combined confidence
	return (sampleConfidence + varianceConfidence) / 2
}

// ExtractExperienceIDs extracts the IDs from a slice of experiences.
func ExtractExperienceIDs(experiences []*selfimprove.Experience) []string {
	ids := make([]string, len(experiences))
	for i, exp := range experiences {
		ids[i] = exp.ID
	}
	return ids
}

// SelectDiverse selects diverse examples based on task type distribution.
func SelectDiverse(experiences []*selfimprove.Experience, n int) []*selfimprove.Experience {
	if len(experiences) <= n {
		return experiences
	}

	// Group by task type
	groups := GroupByTaskType(experiences)

	// Select round-robin from each group
	var selected []*selfimprove.Experience
	indices := make(map[string]int)

	for len(selected) < n {
		for taskType, exps := range groups {
			if len(selected) >= n {
				break
			}
			idx := indices[taskType]
			if idx < len(exps) {
				selected = append(selected, exps[idx])
				indices[taskType] = idx + 1
			}
		}

		// If we've exhausted all groups, break
		exhausted := true
		for taskType, exps := range groups {
			if indices[taskType] < len(exps) {
				exhausted = false
				break
			}
		}
		if exhausted {
			break
		}
	}

	return selected
}

// LLMProvider defines the interface for LLM operations needed by strategies.
type LLMProvider interface {
	// GenerateCompletion generates a completion from the LLM
	GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
}

// CompletionRequest represents a request to the LLM.
type CompletionRequest struct {
	SystemPrompt string  `json:"system_prompt"`
	UserPrompt   string  `json:"user_prompt"`
	Temperature  float64 `json:"temperature"`
	MaxTokens    int     `json:"max_tokens,omitempty"`
}

// CompletionResponse represents a response from the LLM.
type CompletionResponse struct {
	Text       string `json:"text"`
	TokensUsed int    `json:"tokens_used"`
}
