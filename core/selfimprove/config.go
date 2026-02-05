// Package selfimprove provides optional self-improvement capabilities for agents.
//
// Self-improvement is DISABLED by default and requires explicit opt-in.
// When enabled, agents can learn from evaluation feedback, successful executions,
// and failures to continuously improve their performance.
//
// Key features:
// - Experience-based learning from past executions
// - Few-shot prompting with successful examples
// - Prompt refinement based on failure analysis
// - Agent reflection on mistakes
// - Automatic rollback on performance regression
//
// Example usage:
//
//	config := selfimprove.DefaultConfig()
//	config.Enabled = true // Opt-in to self-improvement
//
//	agent := selfimprove.NewSelfImprovingAgent(baseAgent, "agent-1", config)
package selfimprove

import (
	"time"
)

// LearningStrategy defines how the agent learns from experience.
type LearningStrategy string

const (
	// StrategyFewShot uses successful examples as few-shot prompts
	StrategyFewShot LearningStrategy = "few_shot"

	// StrategyPromptRefinement refines prompts based on feedback patterns
	StrategyPromptRefinement LearningStrategy = "prompt_refinement"

	// StrategyPatternExtraction extracts successful patterns for reuse
	StrategyPatternExtraction LearningStrategy = "pattern_extraction"

	// StrategyReflection has the agent reflect on failures and generate improvements
	StrategyReflection LearningStrategy = "reflection"

	// StrategyMetaLearning learns which strategies work for which task types
	StrategyMetaLearning LearningStrategy = "meta_learning"
)

// SelfImprovementConfig configures the self-improvement behavior for an agent.
type SelfImprovementConfig struct {
	// Enabled determines if self-improvement is active. Default: false
	Enabled bool `json:"enabled"`

	// LearningConfig contains learning-specific settings
	LearningConfig *LearningConfig `json:"learning_config,omitempty"`

	// LearnAfterEveryN triggers learning after N executions (0 = disabled)
	LearnAfterEveryN int `json:"learn_after_every_n"`

	// LearnOnSchedule triggers learning on a schedule (0 = disabled)
	LearnOnSchedule time.Duration `json:"learn_on_schedule"`

	// LearnOnFeedback triggers immediate learning when feedback is received
	LearnOnFeedback bool `json:"learn_on_feedback"`

	// MaxImprovementsPerDay limits the number of auto-applied improvements
	MaxImprovementsPerDay int `json:"max_improvements_per_day"`

	// RequireApprovalAbove requires human approval for changes with impact above this threshold
	RequireApprovalAbove float64 `json:"require_approval_above"`

	// AutoRollbackOnRegression automatically rolls back changes that cause regression
	AutoRollbackOnRegression bool `json:"auto_rollback_on_regression"`

	// RegressionThreshold is the score drop that triggers automatic rollback
	RegressionThreshold float64 `json:"regression_threshold"`
}

// LearningConfig configures the learning engine behavior.
type LearningConfig struct {
	// Strategies to use for learning
	Strategies []LearningStrategy `json:"strategies"`

	// MinScoreForSuccess is the minimum score to consider an experience successful
	MinScoreForSuccess float64 `json:"min_score_for_success"`

	// MaxScoreForFailure is the maximum score to consider an experience a failure
	MaxScoreForFailure float64 `json:"max_score_for_failure"`

	// MinExperiencesForLearn is the minimum number of experiences needed before learning
	MinExperiencesForLearn int `json:"min_experiences_for_learn"`

	// FewShotExamples is the number of examples to use for few-shot prompting
	FewShotExamples int `json:"few_shot_examples"`

	// RefinementInterval is how often to run prompt refinement
	RefinementInterval time.Duration `json:"refinement_interval"`

	// MaxPromptVersions is the maximum number of prompt versions to keep
	MaxPromptVersions int `json:"max_prompt_versions"`

	// EnableABTesting enables A/B testing of improvements
	EnableABTesting bool `json:"enable_ab_testing"`

	// ABTestMinSamples is the minimum samples needed for A/B test conclusions
	ABTestMinSamples int `json:"ab_test_min_samples"`

	// ABTestConfidence is the required confidence level for A/B test conclusions
	ABTestConfidence float64 `json:"ab_test_confidence"`

	// RequireHumanApproval requires human approval for all prompt changes
	RequireHumanApproval bool `json:"require_human_approval"`

	// MaxAutoImprovements is the maximum auto-improvements allowed per day
	MaxAutoImprovements int `json:"max_auto_improvements"`
}

// OrchestratorLearningConfig extends SelfImprovementConfig for orchestrators.
type OrchestratorLearningConfig struct {
	*SelfImprovementConfig

	// LearnDecompositionPatterns enables learning task decomposition patterns
	LearnDecompositionPatterns bool `json:"learn_decomposition_patterns"`

	// LearnWorkerAffinities enables learning worker-task affinities
	LearnWorkerAffinities bool `json:"learn_worker_affinities"`

	// LearnRetryStrategies enables learning optimal retry strategies
	LearnRetryStrategies bool `json:"learn_retry_strategies"`
}

// FewShotConfig configures the few-shot learning strategy.
type FewShotConfig struct {
	// NumExamples is the number of examples to include
	NumExamples int `json:"num_examples"`

	// MinScore is the minimum score for an example to be used
	MinScore float64 `json:"min_score"`

	// DiversityWeight balances diversity vs relevance (0-1)
	DiversityWeight float64 `json:"diversity_weight"`

	// MaxExampleLength is the maximum length per example in tokens
	MaxExampleLength int `json:"max_example_length"`
}

// PromptRefinementConfig configures the prompt refinement strategy.
type PromptRefinementConfig struct {
	// MinFailures is the minimum number of failures before attempting refinement
	MinFailures int `json:"min_failures"`

	// MinSuccesses is the minimum number of successes for comparison
	MinSuccesses int `json:"min_successes"`

	// Temperature for the refinement LLM call
	Temperature float64 `json:"temperature"`

	// MaxRefinementsPerSession limits refinements in a single session
	MaxRefinementsPerSession int `json:"max_refinements_per_session"`
}

// ReflectionConfig configures the reflection strategy.
type ReflectionConfig struct {
	// MaxReflectionsPerBatch limits reflections processed together
	MaxReflectionsPerBatch int `json:"max_reflections_per_batch"`

	// MinScoreForReflection is the maximum score that triggers reflection
	MinScoreForReflection float64 `json:"min_score_for_reflection"`

	// Temperature for reflection LLM calls
	Temperature float64 `json:"temperature"`
}

// DefaultConfig returns a SelfImprovementConfig with sensible defaults.
// Note: Enabled is false by default - self-improvement must be explicitly enabled.
func DefaultConfig() *SelfImprovementConfig {
	return &SelfImprovementConfig{
		Enabled:                  false, // Must be explicitly enabled
		LearningConfig:           DefaultLearningConfig(),
		LearnAfterEveryN:         100,
		LearnOnSchedule:          0, // Disabled by default
		LearnOnFeedback:          true,
		MaxImprovementsPerDay:    5,
		RequireApprovalAbove:     0.5, // High-impact changes need approval
		AutoRollbackOnRegression: true,
		RegressionThreshold:      0.1, // 10% drop triggers rollback
	}
}

// DefaultLearningConfig returns a LearningConfig with sensible defaults.
func DefaultLearningConfig() *LearningConfig {
	return &LearningConfig{
		Strategies: []LearningStrategy{
			StrategyFewShot,
			StrategyPromptRefinement,
		},
		MinScoreForSuccess:     0.7,
		MaxScoreForFailure:     0.4,
		MinExperiencesForLearn: 10,
		FewShotExamples:        3,
		RefinementInterval:     time.Hour,
		MaxPromptVersions:      10,
		EnableABTesting:        false,
		ABTestMinSamples:       50,
		ABTestConfidence:       0.95,
		RequireHumanApproval:   false,
		MaxAutoImprovements:    10,
	}
}

// DefaultOrchestratorLearningConfig returns an OrchestratorLearningConfig with defaults.
func DefaultOrchestratorLearningConfig() *OrchestratorLearningConfig {
	return &OrchestratorLearningConfig{
		SelfImprovementConfig:      DefaultConfig(),
		LearnDecompositionPatterns: true,
		LearnWorkerAffinities:      true,
		LearnRetryStrategies:       false, // More experimental
	}
}

// DefaultFewShotConfig returns a FewShotConfig with sensible defaults.
func DefaultFewShotConfig() *FewShotConfig {
	return &FewShotConfig{
		NumExamples:      3,
		MinScore:         0.8,
		DiversityWeight:  0.3,
		MaxExampleLength: 1000,
	}
}

// DefaultPromptRefinementConfig returns a PromptRefinementConfig with defaults.
func DefaultPromptRefinementConfig() *PromptRefinementConfig {
	return &PromptRefinementConfig{
		MinFailures:             5,
		MinSuccesses:            5,
		Temperature:             0.3,
		MaxRefinementsPerSession: 3,
	}
}

// DefaultReflectionConfig returns a ReflectionConfig with sensible defaults.
func DefaultReflectionConfig() *ReflectionConfig {
	return &ReflectionConfig{
		MaxReflectionsPerBatch:  10,
		MinScoreForReflection:   0.5,
		Temperature:             0.5,
	}
}

// Validate validates the configuration and returns an error if invalid.
func (c *SelfImprovementConfig) Validate() error {
	if c.LearningConfig != nil {
		if err := c.LearningConfig.Validate(); err != nil {
			return err
		}
	}

	if c.RegressionThreshold < 0 || c.RegressionThreshold > 1 {
		return &ConfigValidationError{
			Field:   "RegressionThreshold",
			Message: "must be between 0 and 1",
		}
	}

	if c.RequireApprovalAbove < 0 || c.RequireApprovalAbove > 1 {
		return &ConfigValidationError{
			Field:   "RequireApprovalAbove",
			Message: "must be between 0 and 1",
		}
	}

	return nil
}

// Validate validates the learning configuration.
func (c *LearningConfig) Validate() error {
	if c.MinScoreForSuccess < 0 || c.MinScoreForSuccess > 1 {
		return &ConfigValidationError{
			Field:   "MinScoreForSuccess",
			Message: "must be between 0 and 1",
		}
	}

	if c.MaxScoreForFailure < 0 || c.MaxScoreForFailure > 1 {
		return &ConfigValidationError{
			Field:   "MaxScoreForFailure",
			Message: "must be between 0 and 1",
		}
	}

	if c.MaxScoreForFailure >= c.MinScoreForSuccess {
		return &ConfigValidationError{
			Field:   "MaxScoreForFailure",
			Message: "must be less than MinScoreForSuccess",
		}
	}

	if c.ABTestConfidence < 0 || c.ABTestConfidence > 1 {
		return &ConfigValidationError{
			Field:   "ABTestConfidence",
			Message: "must be between 0 and 1",
		}
	}

	return nil
}

// ConfigValidationError represents a configuration validation error.
type ConfigValidationError struct {
	Field   string
	Message string
}

func (e *ConfigValidationError) Error() string {
	return "invalid config: " + e.Field + " " + e.Message
}

// global kill switch
var globallyDisabled bool

// DisableGlobally disables all self-improvement across all agents.
// This is a safety kill switch for emergencies.
func DisableGlobally() {
	globallyDisabled = true
}

// EnableGlobally re-enables self-improvement after a global disable.
func EnableGlobally() {
	globallyDisabled = false
}

// IsGloballyDisabled returns true if self-improvement is globally disabled.
func IsGloballyDisabled() bool {
	return globallyDisabled
}
