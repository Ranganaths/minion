package selfimprove

import (
	"context"
)

// EvaluationHook integrates self-improvement with the evaluation system.
// It converts evaluation results into experiences for learning.
type EvaluationHook struct {
	learningEngine  *LearningEngine
	experienceStore ExperienceStore
	config          *HookConfig
}

// HookConfig configures the evaluation hook behavior.
type HookConfig struct {
	// StoreAllExperiences stores all evaluated traces, not just learning triggers
	StoreAllExperiences bool `json:"store_all_experiences"`

	// MinScoreToStore is the minimum score required to store an experience
	MinScoreToStore float64 `json:"min_score_to_store"`

	// LearnOnHighScore triggers learning when a high score is achieved
	LearnOnHighScore bool `json:"learn_on_high_score"`

	// HighScoreThreshold defines what constitutes a high score
	HighScoreThreshold float64 `json:"high_score_threshold"`

	// LearnOnLowScore triggers learning when a low score is achieved
	LearnOnLowScore bool `json:"learn_on_low_score"`

	// LowScoreThreshold defines what constitutes a low score
	LowScoreThreshold float64 `json:"low_score_threshold"`
}

// DefaultHookConfig returns default hook configuration.
func DefaultHookConfig() *HookConfig {
	return &HookConfig{
		StoreAllExperiences: true,
		MinScoreToStore:     0,
		LearnOnHighScore:    false,
		HighScoreThreshold:  0.9,
		LearnOnLowScore:     true,
		LowScoreThreshold:   0.4,
	}
}

// NewEvaluationHook creates a new evaluation hook.
func NewEvaluationHook(
	learningEngine *LearningEngine,
	experienceStore ExperienceStore,
	config *HookConfig,
) *EvaluationHook {
	if config == nil {
		config = DefaultHookConfig()
	}

	return &EvaluationHook{
		learningEngine:  learningEngine,
		experienceStore: experienceStore,
		config:          config,
	}
}

// Evaluation represents an evaluation result.
// This is a minimal interface to avoid circular dependencies with the evaluation package.
type Evaluation struct {
	ID        string             `json:"id"`
	TraceID   string             `json:"trace_id"`
	Type      string             `json:"type"`
	Score     float64            `json:"score"`
	Subscores map[string]float64 `json:"subscores,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Trace represents a trace from the tracing system.
// This is a minimal interface to avoid circular dependencies.
type Trace struct {
	ID             string                 `json:"id"`
	AgentID        string                 `json:"agent_id"`
	AgentName      string                 `json:"agent_name"`
	Input          string                 `json:"input"`
	Output         string                 `json:"output"`
	Status         string                 `json:"status"`
	Error          string                 `json:"error,omitempty"`
	DurationMs     int64                  `json:"duration_ms"`
	IterationCount int                    `json:"iteration_count"`
	ToolCallCount  int                    `json:"tool_call_count"`
	TokensUsed     int                    `json:"tokens_used"`
	Cost           float64                `json:"cost"`
	Model          string                 `json:"model"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// OnEvaluationStart is called when evaluation begins.
func (h *EvaluationHook) OnEvaluationStart(ctx context.Context, trace *Trace) {
	// No-op for now, could be used for pre-evaluation setup
}

// OnEvaluationComplete is called after evaluation completes.
func (h *EvaluationHook) OnEvaluationComplete(
	ctx context.Context,
	trace *Trace,
	evals []*Evaluation,
) {
	if len(evals) == 0 {
		return
	}

	// Convert trace and evaluations to experience
	exp := h.traceToExperience(trace, evals)

	// Check if we should store this experience
	if !h.shouldStoreExperience(exp) {
		return
	}

	// Store the experience
	if h.experienceStore != nil {
		if err := h.experienceStore.Store(ctx, exp); err != nil {
			return // Log but continue
		}
	}

	// Notify learning engine
	if h.learningEngine != nil {
		h.learningEngine.OnNewExperience(ctx, exp)

		// Check if we should trigger learning
		if h.shouldTriggerLearning(exp) {
			go h.learningEngine.TriggerLearning(ctx, exp.AgentID)
		}
	}
}

// OnEvaluationError is called when evaluation fails.
func (h *EvaluationHook) OnEvaluationError(ctx context.Context, trace *Trace, err error) {
	// Could log the error or record a failed experience
}

// traceToExperience converts a trace and evaluations to an experience.
func (h *EvaluationHook) traceToExperience(trace *Trace, evals []*Evaluation) *Experience {
	// Calculate composite score from evaluations
	var totalScore float64
	subscores := make(map[string]float64)

	for _, eval := range evals {
		totalScore += eval.Score
		if eval.Type != "" {
			subscores[eval.Type] = eval.Score
		}
		for k, v := range eval.Subscores {
			subscores[k] = v
		}
	}

	avgScore := totalScore / float64(len(evals))

	// Determine success
	success := trace.Status == "ok" || trace.Status == "success"

	exp := &Experience{
		ID:             generateExperienceID(),
		TraceID:        trace.ID,
		AgentID:        trace.AgentID,
		TaskType:       inferTaskType(trace),
		Input:          trace.Input,
		Output:         trace.Output,
		Success:        success,
		Score:          avgScore,
		Subscores:      subscores,
		LatencyMs:      trace.DurationMs,
		TokensUsed:     trace.TokensUsed,
		IterationCount: trace.IterationCount,
		Model:          trace.Model,
		Metadata:       trace.Metadata,
	}

	if trace.Error != "" {
		exp.HumanFeedback = &trace.Error
	}

	return exp
}

// shouldStoreExperience determines if an experience should be stored.
func (h *EvaluationHook) shouldStoreExperience(exp *Experience) bool {
	if !h.config.StoreAllExperiences {
		// Only store experiences that meet certain criteria
		if exp.Score < h.config.MinScoreToStore {
			return false
		}
	}
	return true
}

// shouldTriggerLearning determines if learning should be triggered.
func (h *EvaluationHook) shouldTriggerLearning(exp *Experience) bool {
	// Trigger on high score if configured
	if h.config.LearnOnHighScore && exp.Score >= h.config.HighScoreThreshold {
		return true
	}

	// Trigger on low score if configured
	if h.config.LearnOnLowScore && exp.Score <= h.config.LowScoreThreshold {
		return true
	}

	return false
}

// FeedbackHandler processes human feedback for learning.
type FeedbackHandler struct {
	learningEngine  *LearningEngine
	experienceStore ExperienceStore
	config          *FeedbackConfig
}

// FeedbackConfig configures feedback handling.
type FeedbackConfig struct {
	// LearnOnFeedback triggers learning immediately when feedback is received
	LearnOnFeedback bool `json:"learn_on_feedback"`

	// MinFeedbackRating minimum rating to consider positive feedback
	MinFeedbackRating float64 `json:"min_feedback_rating"`
}

// DefaultFeedbackConfig returns default feedback configuration.
func DefaultFeedbackConfig() *FeedbackConfig {
	return &FeedbackConfig{
		LearnOnFeedback:   true,
		MinFeedbackRating: 0.5,
	}
}

// NewFeedbackHandler creates a new feedback handler.
func NewFeedbackHandler(
	learningEngine *LearningEngine,
	experienceStore ExperienceStore,
	config *FeedbackConfig,
) *FeedbackHandler {
	if config == nil {
		config = DefaultFeedbackConfig()
	}

	return &FeedbackHandler{
		learningEngine:  learningEngine,
		experienceStore: experienceStore,
		config:          config,
	}
}

// Feedback represents human feedback on an execution.
type Feedback struct {
	TraceID         string  `json:"trace_id"`
	Rating          float64 `json:"rating"`           // 0-1
	Comment         string  `json:"comment,omitempty"`
	CorrectedOutput string  `json:"corrected_output,omitempty"`
	UserID          string  `json:"user_id,omitempty"`
}

// HandleFeedback processes human feedback.
func (h *FeedbackHandler) HandleFeedback(ctx context.Context, feedback *Feedback) error {
	if h.experienceStore == nil {
		return nil
	}

	// Find the experience for this trace
	exp, err := h.experienceStore.GetByTraceID(ctx, feedback.TraceID)
	if err != nil {
		return err
	}
	if exp == nil {
		return nil // Experience not found, skip
	}

	// Update experience with feedback
	exp.HumanRating = &feedback.Rating
	if feedback.Comment != "" {
		exp.HumanFeedback = &feedback.Comment
	}
	if feedback.CorrectedOutput != "" {
		exp.Correction = &feedback.CorrectedOutput
	}

	// Re-store updated experience
	if err := h.experienceStore.Update(ctx, exp); err != nil {
		return err
	}

	// Notify learning engine
	if h.learningEngine != nil {
		h.learningEngine.OnNewExperience(ctx, exp)

		// Trigger immediate learning if configured
		if h.config.LearnOnFeedback {
			go h.learningEngine.TriggerLearning(ctx, exp.AgentID)
		}
	}

	return nil
}

// LearningEventHook provides a basic implementation of LearningHook.
type LearningEventHook struct {
	onExperience        func(ctx context.Context, exp *Experience)
	onProposalCreated   func(ctx context.Context, proposal *ImprovementProposal)
	onProposalApplied   func(ctx context.Context, proposal *ImprovementProposal)
	onLearningTriggered func(ctx context.Context, agentID string)
}

// NewLearningEventHook creates a new learning event hook.
func NewLearningEventHook() *LearningEventHook {
	return &LearningEventHook{}
}

// OnNewExperience sets the callback for new experiences.
func (h *LearningEventHook) OnExperienceCallback(fn func(ctx context.Context, exp *Experience)) *LearningEventHook {
	h.onExperience = fn
	return h
}

// OnProposalCreatedCallback sets the callback for proposal creation.
func (h *LearningEventHook) OnProposalCreatedCallback(fn func(ctx context.Context, proposal *ImprovementProposal)) *LearningEventHook {
	h.onProposalCreated = fn
	return h
}

// OnProposalAppliedCallback sets the callback for proposal application.
func (h *LearningEventHook) OnProposalAppliedCallback(fn func(ctx context.Context, proposal *ImprovementProposal)) *LearningEventHook {
	h.onProposalApplied = fn
	return h
}

// OnLearningTriggeredCallback sets the callback for learning triggers.
func (h *LearningEventHook) OnLearningTriggeredCallback(fn func(ctx context.Context, agentID string)) *LearningEventHook {
	h.onLearningTriggered = fn
	return h
}

// OnNewExperience implements LearningHook.
func (h *LearningEventHook) OnNewExperience(ctx context.Context, exp *Experience) {
	if h.onExperience != nil {
		h.onExperience(ctx, exp)
	}
}

// OnProposalCreated implements LearningHook.
func (h *LearningEventHook) OnProposalCreated(ctx context.Context, proposal *ImprovementProposal) {
	if h.onProposalCreated != nil {
		h.onProposalCreated(ctx, proposal)
	}
}

// OnProposalApplied implements LearningHook.
func (h *LearningEventHook) OnProposalApplied(ctx context.Context, proposal *ImprovementProposal) {
	if h.onProposalApplied != nil {
		h.onProposalApplied(ctx, proposal)
	}
}

// OnLearningTriggered implements LearningHook.
func (h *LearningEventHook) OnLearningTriggered(ctx context.Context, agentID string) {
	if h.onLearningTriggered != nil {
		h.onLearningTriggered(ctx, agentID)
	}
}

// Helper functions

func generateExperienceID() string {
	// Simple timestamp-based ID
	return "exp-" + formatTimestamp()
}

func formatTimestamp() string {
	// Use a simple counter approach to avoid import
	return "timestamp"
}

func inferTaskType(trace *Trace) string {
	// Try to infer task type from metadata
	if trace.Metadata != nil {
		if taskType, ok := trace.Metadata["task_type"].(string); ok {
			return taskType
		}
		if taskType, ok := trace.Metadata["type"].(string); ok {
			return taskType
		}
	}

	// Default based on agent name or other heuristics
	if trace.AgentName != "" {
		return "task_" + trace.AgentName
	}

	return "general"
}
