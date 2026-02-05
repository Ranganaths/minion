package selfimprove

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// SelfImprovingOrchestrator wraps an orchestrator with learning capabilities.
// It learns to decompose tasks better and select optimal workers.
type SelfImprovingOrchestrator struct {
	mu sync.RWMutex

	// Orchestrator ID
	orchestratorID string

	// Self-improvement components
	learningEngine  *LearningEngine
	experienceStore ExperienceStore

	// Configuration
	config *OrchestratorLearningConfig

	// Learned patterns
	decompositionPatterns map[string]*DecompositionPattern
	workerAffinities      map[string]map[string]float64 // taskType -> workerID -> affinity

	// Metrics
	metrics *OrchestratorMetrics

	// State
	enabled bool
}

// DecompositionPattern represents a learned task decomposition pattern.
type DecompositionPattern struct {
	TaskType     string            `json:"task_type"`
	InputPattern string            `json:"input_pattern"` // Regex or keyword matching
	Subtasks     []*SubtaskPattern `json:"subtasks"`
	SuccessRate  float64           `json:"success_rate"`
	AvgScore     float64           `json:"avg_score"`
	UsageCount   int               `json:"usage_count"`
	LastUsed     time.Time         `json:"last_used"`
}

// SubtaskPattern represents a pattern for a subtask in a decomposition.
type SubtaskPattern struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	RequiredCaps     []string `json:"required_capabilities"`
	Dependencies     []string `json:"dependencies"`
	Priority         int      `json:"priority"`
	EstimatedTokens  int      `json:"estimated_tokens"`
	EstimatedLatency int64    `json:"estimated_latency_ms"`
}

// WorkerAffinity tracks how well a worker performs on specific task types.
type WorkerAffinity struct {
	WorkerID      string  `json:"worker_id"`
	TaskType      string  `json:"task_type"`
	Affinity      float64 `json:"affinity"` // 0-1, higher is better
	Executions    int     `json:"executions"`
	SuccessRate   float64 `json:"success_rate"`
	AvgScore      float64 `json:"avg_score"`
	AvgLatency    int64   `json:"avg_latency_ms"`
}

// OrchestratorMetrics tracks orchestrator learning metrics.
type OrchestratorMetrics struct {
	mu sync.RWMutex

	TotalTasks              int64   `json:"total_tasks"`
	SuccessfulTasks         int64   `json:"successful_tasks"`
	PatternMatches          int64   `json:"pattern_matches"`
	PatternMisses           int64   `json:"pattern_misses"`
	AffinityBasedAssignments int64  `json:"affinity_based_assignments"`
	CapabilityAssignments   int64   `json:"capability_assignments"`
	AvgDecompositionScore   float64 `json:"avg_decomposition_score"`
	PatternsLearned         int     `json:"patterns_learned"`
}

// NewSelfImprovingOrchestrator creates a new self-improving orchestrator.
func NewSelfImprovingOrchestrator(
	orchestratorID string,
	config *OrchestratorLearningConfig,
	learningEngine *LearningEngine,
	experienceStore ExperienceStore,
) *SelfImprovingOrchestrator {
	if config == nil {
		config = DefaultOrchestratorLearningConfig()
	}

	return &SelfImprovingOrchestrator{
		orchestratorID:        orchestratorID,
		config:                config,
		learningEngine:        learningEngine,
		experienceStore:       experienceStore,
		decompositionPatterns: make(map[string]*DecompositionPattern),
		workerAffinities:      make(map[string]map[string]float64),
		metrics:               &OrchestratorMetrics{},
		enabled:               config.Enabled,
	}
}

// EnableLearning enables orchestrator learning.
func (o *SelfImprovingOrchestrator) EnableLearning() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.enabled = true
}

// DisableLearning disables orchestrator learning.
func (o *SelfImprovingOrchestrator) DisableLearning() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.enabled = false
}

// GetDecompositionPattern finds a matching decomposition pattern for a task.
func (o *SelfImprovingOrchestrator) GetDecompositionPattern(taskType string, input interface{}) *DecompositionPattern {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if !o.enabled || !o.config.LearnDecompositionPatterns {
		return nil
	}

	pattern, ok := o.decompositionPatterns[taskType]
	if !ok {
		o.metrics.mu.Lock()
		o.metrics.PatternMisses++
		o.metrics.mu.Unlock()
		return nil
	}

	// Check if pattern has good enough success rate
	if pattern.SuccessRate < 0.7 {
		return nil
	}

	o.metrics.mu.Lock()
	o.metrics.PatternMatches++
	o.metrics.mu.Unlock()

	return pattern
}

// SelectWorker selects the best worker for a task based on learned affinities.
func (o *SelfImprovingOrchestrator) SelectWorker(taskType string, availableWorkers []string) string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if !o.enabled || !o.config.LearnWorkerAffinities {
		o.metrics.mu.Lock()
		o.metrics.CapabilityAssignments++
		o.metrics.mu.Unlock()
		return "" // Let default selection handle it
	}

	affinities, ok := o.workerAffinities[taskType]
	if !ok || len(affinities) == 0 {
		o.metrics.mu.Lock()
		o.metrics.CapabilityAssignments++
		o.metrics.mu.Unlock()
		return ""
	}

	// Find best available worker by affinity
	var bestWorker string
	var bestAffinity float64 = -1

	for _, workerID := range availableWorkers {
		if affinity, ok := affinities[workerID]; ok && affinity > bestAffinity {
			bestWorker = workerID
			bestAffinity = affinity
		}
	}

	if bestWorker != "" && bestAffinity >= 0.5 {
		o.metrics.mu.Lock()
		o.metrics.AffinityBasedAssignments++
		o.metrics.mu.Unlock()
		return bestWorker
	}

	o.metrics.mu.Lock()
	o.metrics.CapabilityAssignments++
	o.metrics.mu.Unlock()
	return ""
}

// RecordTaskExecution records a task execution for learning.
func (o *SelfImprovingOrchestrator) RecordTaskExecution(ctx context.Context, record *TaskExecutionRecord) {
	if !o.enabled {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	// Update metrics
	o.metrics.mu.Lock()
	o.metrics.TotalTasks++
	if record.Success {
		o.metrics.SuccessfulTasks++
	}
	o.metrics.mu.Unlock()

	// Update worker affinities
	if o.config.LearnWorkerAffinities && record.WorkerID != "" {
		o.updateWorkerAffinity(record)
	}

	// Update decomposition patterns
	if o.config.LearnDecompositionPatterns && len(record.Subtasks) > 0 {
		o.updateDecompositionPattern(record)
	}
}

// updateWorkerAffinity updates worker-task type affinity.
func (o *SelfImprovingOrchestrator) updateWorkerAffinity(record *TaskExecutionRecord) {
	taskType := record.TaskType
	workerID := record.WorkerID

	if _, ok := o.workerAffinities[taskType]; !ok {
		o.workerAffinities[taskType] = make(map[string]float64)
	}

	// Get current affinity or start at 0.5
	currentAffinity := o.workerAffinities[taskType][workerID]
	if currentAffinity == 0 {
		currentAffinity = 0.5
	}

	// Update using exponential moving average
	alpha := 0.1 // Learning rate
	newValue := record.Score
	if !record.Success {
		newValue = 0
	}

	newAffinity := currentAffinity*(1-alpha) + newValue*alpha
	o.workerAffinities[taskType][workerID] = newAffinity
}

// updateDecompositionPattern updates or creates a decomposition pattern.
func (o *SelfImprovingOrchestrator) updateDecompositionPattern(record *TaskExecutionRecord) {
	taskType := record.TaskType

	pattern, ok := o.decompositionPatterns[taskType]
	if !ok {
		// Create new pattern
		pattern = &DecompositionPattern{
			TaskType: taskType,
			Subtasks: record.Subtasks,
		}
		o.decompositionPatterns[taskType] = pattern
		o.metrics.mu.Lock()
		o.metrics.PatternsLearned++
		o.metrics.mu.Unlock()
	}

	// Update statistics using exponential moving average
	alpha := 0.1
	successValue := 0.0
	if record.Success {
		successValue = 1.0
	}

	pattern.SuccessRate = pattern.SuccessRate*(1-alpha) + successValue*alpha
	pattern.AvgScore = pattern.AvgScore*(1-alpha) + record.Score*alpha
	pattern.UsageCount++
	pattern.LastUsed = time.Now()

	// Update subtask patterns if the execution was successful and had better score
	if record.Success && record.Score > pattern.AvgScore && len(record.Subtasks) > 0 {
		pattern.Subtasks = record.Subtasks
	}
}

// TaskExecutionRecord holds information about a task execution for learning.
type TaskExecutionRecord struct {
	TaskID     string            `json:"task_id"`
	TaskType   string            `json:"task_type"`
	WorkerID   string            `json:"worker_id,omitempty"`
	Success    bool              `json:"success"`
	Score      float64           `json:"score"`
	LatencyMs  int64             `json:"latency_ms"`
	TokensUsed int               `json:"tokens_used"`
	Subtasks   []*SubtaskPattern `json:"subtasks,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// GetWorkerAffinities returns learned worker affinities for a task type.
func (o *SelfImprovingOrchestrator) GetWorkerAffinities(taskType string) []*WorkerAffinity {
	o.mu.RLock()
	defer o.mu.RUnlock()

	affinities, ok := o.workerAffinities[taskType]
	if !ok {
		return nil
	}

	result := make([]*WorkerAffinity, 0, len(affinities))
	for workerID, affinity := range affinities {
		result = append(result, &WorkerAffinity{
			WorkerID: workerID,
			TaskType: taskType,
			Affinity: affinity,
		})
	}

	// Sort by affinity descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Affinity > result[j].Affinity
	})

	return result
}

// GetAllPatterns returns all learned decomposition patterns.
func (o *SelfImprovingOrchestrator) GetAllPatterns() []*DecompositionPattern {
	o.mu.RLock()
	defer o.mu.RUnlock()

	patterns := make([]*DecompositionPattern, 0, len(o.decompositionPatterns))
	for _, pattern := range o.decompositionPatterns {
		patterns = append(patterns, pattern)
	}

	// Sort by usage count descending
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].UsageCount > patterns[j].UsageCount
	})

	return patterns
}

// GetMetrics returns orchestrator learning metrics.
func (o *SelfImprovingOrchestrator) GetMetrics() *OrchestratorMetrics {
	o.metrics.mu.RLock()
	defer o.metrics.mu.RUnlock()

	return &OrchestratorMetrics{
		TotalTasks:               o.metrics.TotalTasks,
		SuccessfulTasks:          o.metrics.SuccessfulTasks,
		PatternMatches:           o.metrics.PatternMatches,
		PatternMisses:            o.metrics.PatternMisses,
		AffinityBasedAssignments: o.metrics.AffinityBasedAssignments,
		CapabilityAssignments:    o.metrics.CapabilityAssignments,
		AvgDecompositionScore:    o.metrics.AvgDecompositionScore,
		PatternsLearned:          o.metrics.PatternsLearned,
	}
}

// ImportPattern imports a decomposition pattern.
func (o *SelfImprovingOrchestrator) ImportPattern(pattern *DecompositionPattern) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.decompositionPatterns[pattern.TaskType] = pattern
}

// ExportPatterns exports all learned patterns.
func (o *SelfImprovingOrchestrator) ExportPatterns() []*DecompositionPattern {
	return o.GetAllPatterns()
}

// ResetLearning resets all learned data.
func (o *SelfImprovingOrchestrator) ResetLearning() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.decompositionPatterns = make(map[string]*DecompositionPattern)
	o.workerAffinities = make(map[string]map[string]float64)
	o.metrics = &OrchestratorMetrics{}
}

// OptimizeDecomposition suggests an optimized decomposition based on history.
func (o *SelfImprovingOrchestrator) OptimizeDecomposition(taskType string, currentSubtasks []*SubtaskPattern) []*SubtaskPattern {
	o.mu.RLock()
	defer o.mu.RUnlock()

	pattern := o.decompositionPatterns[taskType]
	if pattern == nil || pattern.SuccessRate < 0.8 {
		return currentSubtasks // Use provided decomposition
	}

	// Use learned pattern if it has better success rate
	if len(pattern.Subtasks) > 0 {
		return pattern.Subtasks
	}

	return currentSubtasks
}

// PredictWorkerPerformance predicts a worker's performance for a task type.
func (o *SelfImprovingOrchestrator) PredictWorkerPerformance(workerID, taskType string) float64 {
	o.mu.RLock()
	defer o.mu.RUnlock()

	affinities, ok := o.workerAffinities[taskType]
	if !ok {
		return 0.5 // Unknown, return neutral
	}

	affinity, ok := affinities[workerID]
	if !ok {
		return 0.5 // No data for this worker
	}

	return affinity
}

// GetRecommendedWorkers returns workers ranked by predicted performance.
func (o *SelfImprovingOrchestrator) GetRecommendedWorkers(taskType string, availableWorkers []string) []string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	affinities, ok := o.workerAffinities[taskType]
	if !ok {
		return availableWorkers // No learning data, return as-is
	}

	// Score each worker
	type workerScore struct {
		id       string
		affinity float64
	}

	scores := make([]workerScore, len(availableWorkers))
	for i, workerID := range availableWorkers {
		affinity, ok := affinities[workerID]
		if !ok {
			affinity = 0.5 // Default for unknown workers
		}
		scores[i] = workerScore{id: workerID, affinity: affinity}
	}

	// Sort by affinity descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].affinity > scores[j].affinity
	})

	result := make([]string, len(scores))
	for i, ws := range scores {
		result[i] = ws.id
	}

	return result
}

// String returns a string representation of the orchestrator state.
func (o *SelfImprovingOrchestrator) String() string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return fmt.Sprintf("SelfImprovingOrchestrator{id=%s, patterns=%d, enabled=%v}",
		o.orchestratorID, len(o.decompositionPatterns), o.enabled)
}
