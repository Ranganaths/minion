package selfimprove

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// TaskHandler defines how a worker processes tasks.
// This interface matches multiagent.TaskHandler.
type TaskHandler interface {
	HandleTask(ctx context.Context, task interface{}) (interface{}, error)
	GetCapabilities() []string
	GetName() string
}

// Task represents a task to be processed.
// This is a minimal interface to avoid circular dependencies.
type Task interface {
	GetID() string
	GetType() string
	GetInput() interface{}
	GetMetadata() map[string]interface{}
}

// taskWrapper wraps any task to provide the Task interface.
type taskWrapper struct {
	id       string
	taskType string
	input    interface{}
	metadata map[string]interface{}
}

func (t *taskWrapper) GetID() string                    { return t.id }
func (t *taskWrapper) GetType() string                  { return t.taskType }
func (t *taskWrapper) GetInput() interface{}            { return t.input }
func (t *taskWrapper) GetMetadata() map[string]interface{} { return t.metadata }

// SelfImprovingAgent wraps any agent with self-improvement capabilities.
type SelfImprovingAgent struct {
	mu sync.RWMutex

	// Underlying agent
	baseHandler TaskHandler
	agentID     string

	// Self-improvement components
	learningEngine  *LearningEngine
	experienceStore ExperienceStore

	// Configuration
	config *SelfImprovementConfig

	// State
	currentPrompt   string
	fewShotExamples []*Experience
	executionCount  atomic.Int64
	enabled         atomic.Bool

	// Metrics tracking
	metrics *SelfImprovementMetrics

	// Callbacks
	onExperience func(ctx context.Context, exp *Experience)
}

// SelfImprovingAgentOption configures a SelfImprovingAgent.
type SelfImprovingAgentOption func(*SelfImprovingAgent)

// WithExperienceStore sets the experience store.
func WithExperienceStore(store ExperienceStore) SelfImprovingAgentOption {
	return func(a *SelfImprovingAgent) {
		a.experienceStore = store
	}
}

// WithLearningEngine sets the learning engine.
func WithLearningEngine(engine *LearningEngine) SelfImprovingAgentOption {
	return func(a *SelfImprovingAgent) {
		a.learningEngine = engine
	}
}

// WithConfig sets the self-improvement configuration.
func WithConfig(config *SelfImprovementConfig) SelfImprovingAgentOption {
	return func(a *SelfImprovingAgent) {
		a.config = config
	}
}

// WithOnExperience sets a callback for new experiences.
func WithOnExperience(fn func(ctx context.Context, exp *Experience)) SelfImprovingAgentOption {
	return func(a *SelfImprovingAgent) {
		a.onExperience = fn
	}
}

// NewSelfImprovingAgent creates a new self-improving agent wrapper.
func NewSelfImprovingAgent(
	baseHandler TaskHandler,
	agentID string,
	opts ...SelfImprovingAgentOption,
) *SelfImprovingAgent {
	agent := &SelfImprovingAgent{
		baseHandler: baseHandler,
		agentID:     agentID,
		config:      DefaultConfig(),
		metrics:     NewSelfImprovementMetrics(agentID),
	}

	// Apply options
	for _, opt := range opts {
		opt(agent)
	}

	// Set enabled based on config
	agent.enabled.Store(agent.config.Enabled)

	return agent
}

// HandleTask wraps the base handler's task handling with learning.
func (a *SelfImprovingAgent) HandleTask(ctx context.Context, task interface{}) (interface{}, error) {
	startTime := time.Now()
	a.executionCount.Add(1)

	// Check if self-improvement is enabled
	if !a.isEnabled() {
		return a.baseHandler.HandleTask(ctx, task)
	}

	// Enhance task with learned context
	enhancedTask := a.enhanceTask(ctx, task)

	// Execute with base handler
	result, err := a.baseHandler.HandleTask(ctx, enhancedTask)

	// Record experience asynchronously
	go a.recordExperience(ctx, task, result, err, time.Since(startTime))

	// Check if learning should be triggered
	if a.shouldLearn() {
		go a.triggerLearning(ctx)
	}

	return result, err
}

// GetCapabilities returns the capabilities of the underlying handler.
func (a *SelfImprovingAgent) GetCapabilities() []string {
	return a.baseHandler.GetCapabilities()
}

// GetName returns the name of the agent.
func (a *SelfImprovingAgent) GetName() string {
	return fmt.Sprintf("self-improving-%s", a.baseHandler.GetName())
}

// EnableLearning enables self-improvement.
func (a *SelfImprovingAgent) EnableLearning() {
	a.enabled.Store(true)
}

// DisableLearning disables self-improvement.
func (a *SelfImprovingAgent) DisableLearning() {
	a.enabled.Store(false)
}

// IsLearningEnabled returns whether self-improvement is enabled.
func (a *SelfImprovingAgent) IsLearningEnabled() bool {
	return a.isEnabled()
}

// GetMetrics returns the self-improvement metrics.
func (a *SelfImprovingAgent) GetMetrics() *SelfImprovementMetrics {
	return a.metrics
}

// GetExecutionCount returns the number of executions.
func (a *SelfImprovingAgent) GetExecutionCount() int64 {
	return a.executionCount.Load()
}

// GetCurrentPrompt returns the current enhanced prompt.
func (a *SelfImprovingAgent) GetCurrentPrompt() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentPrompt
}

// SetCurrentPrompt sets the current enhanced prompt.
func (a *SelfImprovingAgent) SetCurrentPrompt(prompt string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.currentPrompt = prompt
}

// GetFewShotExamples returns the current few-shot examples.
func (a *SelfImprovingAgent) GetFewShotExamples() []*Experience {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.fewShotExamples
}

// SetFewShotExamples sets the few-shot examples.
func (a *SelfImprovingAgent) SetFewShotExamples(examples []*Experience) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fewShotExamples = examples
}

// isEnabled checks if self-improvement is active.
func (a *SelfImprovingAgent) isEnabled() bool {
	if IsGloballyDisabled() {
		return false
	}
	return a.enabled.Load()
}

// enhanceTask adds few-shot examples and learned context to the task.
func (a *SelfImprovingAgent) enhanceTask(ctx context.Context, task interface{}) interface{} {
	if !a.isEnabled() {
		return task
	}

	a.mu.RLock()
	fewShot := a.fewShotExamples
	currentPrompt := a.currentPrompt
	a.mu.RUnlock()

	// Try to enhance based on task type
	switch t := task.(type) {
	case map[string]interface{}:
		enhanced := make(map[string]interface{})
		for k, v := range t {
			enhanced[k] = v
		}

		// Add few-shot examples
		if len(fewShot) > 0 {
			formatter := NewDefaultExperienceFormatter()
			enhanced["few_shot_examples"] = formatter.FormatAsFewShot(fewShot)
		}

		// Add enhanced prompt
		if currentPrompt != "" {
			enhanced["enhanced_prompt"] = currentPrompt
		}

		return enhanced

	default:
		// Return task as-is if we can't enhance it
		return task
	}
}

// recordExperience records the task execution as an experience.
func (a *SelfImprovingAgent) recordExperience(
	ctx context.Context,
	task interface{},
	result interface{},
	err error,
	duration time.Duration,
) {
	if a.experienceStore == nil {
		return
	}

	// Extract task information
	taskInfo := extractTaskInfo(task)

	// Determine success
	success := err == nil

	// Create experience
	exp := &Experience{
		ID:             uuid.New().String(),
		AgentID:        a.agentID,
		TaskType:       taskInfo.taskType,
		Input:          taskInfo.input,
		Output:         result,
		Success:        success,
		Score:          0, // Will be set by evaluator
		Timestamp:      time.Now(),
		LatencyMs:      duration.Milliseconds(),
		IterationCount: 1,
		Metadata:       taskInfo.metadata,
	}

	if err != nil {
		errStr := err.Error()
		exp.HumanFeedback = &errStr
	}

	// Store the experience
	if storeErr := a.experienceStore.Store(ctx, exp); storeErr != nil {
		// Log error but don't fail the main operation
		return
	}

	// Notify learning engine
	if a.learningEngine != nil {
		a.learningEngine.OnNewExperience(ctx, exp)
	}

	// Call custom callback
	if a.onExperience != nil {
		a.onExperience(ctx, exp)
	}

	// Update metrics
	a.metrics.RecordExecution(success, exp.Score, duration)
}

// shouldLearn determines if learning should be triggered.
func (a *SelfImprovingAgent) shouldLearn() bool {
	if a.learningEngine == nil {
		return false
	}

	count := a.executionCount.Load()

	// Check execution count threshold
	if a.config.LearnAfterEveryN > 0 {
		if count%int64(a.config.LearnAfterEveryN) == 0 {
			return true
		}
	}

	return false
}

// triggerLearning initiates a learning cycle.
func (a *SelfImprovingAgent) triggerLearning(ctx context.Context) {
	if a.learningEngine == nil {
		return
	}

	if err := a.learningEngine.TriggerLearning(ctx, a.agentID); err != nil {
		// Log error but continue
		return
	}

	// Update metrics
	a.metrics.RecordLearningCycle()
}

// ApplyImprovement applies an improvement to this agent.
func (a *SelfImprovingAgent) ApplyImprovement(proposal *ImprovementProposal) error {
	if !a.isEnabled() {
		return fmt.Errorf("self-improvement is disabled")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	switch proposal.ImprovementType {
	case ImprovementTypeSystemPrompt, ImprovementTypeUserPrompt:
		a.currentPrompt = proposal.ProposedValue
		a.metrics.RecordImprovement(true)
		return nil

	case ImprovementTypeFewShotExamples:
		// Few-shot examples are applied through the strategy
		a.metrics.RecordImprovement(true)
		return nil

	default:
		return fmt.Errorf("unsupported improvement type: %s", proposal.ImprovementType)
	}
}

// RollbackImprovement rolls back a previously applied improvement.
func (a *SelfImprovingAgent) RollbackImprovement(proposal *ImprovementProposal) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch proposal.ImprovementType {
	case ImprovementTypeSystemPrompt, ImprovementTypeUserPrompt:
		a.currentPrompt = proposal.CurrentValue
		a.metrics.RecordRollback()
		return nil

	case ImprovementTypeFewShotExamples:
		// Would need to restore previous examples
		a.metrics.RecordRollback()
		return nil

	default:
		return fmt.Errorf("unsupported improvement type: %s", proposal.ImprovementType)
	}
}

// taskInfo holds extracted task information.
type taskInfo struct {
	id       string
	taskType string
	input    interface{}
	metadata map[string]interface{}
}

// extractTaskInfo extracts information from a task of any type.
func extractTaskInfo(task interface{}) taskInfo {
	info := taskInfo{
		id:       uuid.New().String(),
		taskType: "unknown",
		metadata: make(map[string]interface{}),
	}

	switch t := task.(type) {
	case Task:
		info.id = t.GetID()
		info.taskType = t.GetType()
		info.input = t.GetInput()
		info.metadata = t.GetMetadata()

	case map[string]interface{}:
		if id, ok := t["id"].(string); ok {
			info.id = id
		}
		if taskType, ok := t["type"].(string); ok {
			info.taskType = taskType
		}
		if input, ok := t["input"]; ok {
			info.input = input
		}
		info.metadata = t

	default:
		info.input = task
	}

	return info
}

// SelfImprovementMetrics tracks metrics for self-improvement.
type SelfImprovementMetrics struct {
	mu sync.RWMutex

	AgentID string

	// Execution metrics
	TotalExecutions   int64
	SuccessfulExecs   int64
	FailedExecs       int64
	TotalLatencyMs    int64
	TotalScore        float64

	// Learning metrics
	LearningCycles    int64
	ImprovementsApplied int64
	Rollbacks         int64

	// Calculated metrics
	AvgScoreBefore    float64
	AvgScoreAfter     float64

	// Score history for trend analysis
	recentScores []float64
	maxScoreHistory int
}

// NewSelfImprovementMetrics creates new metrics for an agent.
func NewSelfImprovementMetrics(agentID string) *SelfImprovementMetrics {
	return &SelfImprovementMetrics{
		AgentID:         agentID,
		recentScores:    make([]float64, 0, 100),
		maxScoreHistory: 100,
	}
}

// RecordExecution records a task execution.
func (m *SelfImprovementMetrics) RecordExecution(success bool, score float64, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalExecutions++
	if success {
		m.SuccessfulExecs++
	} else {
		m.FailedExecs++
	}

	m.TotalLatencyMs += duration.Milliseconds()
	m.TotalScore += score

	// Track score history
	m.recentScores = append(m.recentScores, score)
	if len(m.recentScores) > m.maxScoreHistory {
		m.recentScores = m.recentScores[1:]
	}
}

// RecordLearningCycle records a learning cycle.
func (m *SelfImprovementMetrics) RecordLearningCycle() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.LearningCycles++

	// Calculate before average from older scores
	if len(m.recentScores) > 20 {
		var sum float64
		for i := 0; i < len(m.recentScores)/2; i++ {
			sum += m.recentScores[i]
		}
		m.AvgScoreBefore = sum / float64(len(m.recentScores)/2)
	}
}

// RecordImprovement records an applied improvement.
func (m *SelfImprovementMetrics) RecordImprovement(success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if success {
		m.ImprovementsApplied++
	}

	// Calculate after average from recent scores
	if len(m.recentScores) > 20 {
		var sum float64
		start := len(m.recentScores) / 2
		for i := start; i < len(m.recentScores); i++ {
			sum += m.recentScores[i]
		}
		m.AvgScoreAfter = sum / float64(len(m.recentScores)-start)
	}
}

// RecordRollback records a rollback.
func (m *SelfImprovementMetrics) RecordRollback() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Rollbacks++
}

// GetStats returns current statistics.
func (m *SelfImprovementMetrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	avgLatency := float64(0)
	if m.TotalExecutions > 0 {
		avgLatency = float64(m.TotalLatencyMs) / float64(m.TotalExecutions)
	}

	avgScore := float64(0)
	if m.TotalExecutions > 0 {
		avgScore = m.TotalScore / float64(m.TotalExecutions)
	}

	successRate := float64(0)
	if m.TotalExecutions > 0 {
		successRate = float64(m.SuccessfulExecs) / float64(m.TotalExecutions)
	}

	return map[string]interface{}{
		"agent_id":             m.AgentID,
		"total_executions":     m.TotalExecutions,
		"successful_executions": m.SuccessfulExecs,
		"failed_executions":    m.FailedExecs,
		"success_rate":         successRate,
		"avg_latency_ms":       avgLatency,
		"avg_score":            avgScore,
		"learning_cycles":      m.LearningCycles,
		"improvements_applied": m.ImprovementsApplied,
		"rollbacks":            m.Rollbacks,
		"avg_score_before":     m.AvgScoreBefore,
		"avg_score_after":      m.AvgScoreAfter,
		"score_improvement":    m.AvgScoreAfter - m.AvgScoreBefore,
	}
}
