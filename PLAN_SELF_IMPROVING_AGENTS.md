# Self-Improving Multi-Agent System - Implementation Plan

## Overview

This plan outlines the implementation of **optional** self-improving capabilities for the minion multi-agent framework. The system will learn from evaluations, feedback, and execution history to continuously improve agent performance.

**Key Principle**: Self-improvement is **disabled by default** and requires explicit opt-in.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Self-Improving Agent System                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐                  │
│  │   Agents     │───▶│  Execution   │───▶│  Evaluation  │                  │
│  │ (Workers/    │    │   Traces     │    │   Pipeline   │                  │
│  │ Orchestrator)│    │              │    │              │                  │
│  └──────────────┘    └──────────────┘    └──────┬───────┘                  │
│         ▲                                        │                          │
│         │                                        ▼                          │
│  ┌──────┴───────┐    ┌──────────────┐    ┌──────────────┐                  │
│  │   Improved   │◀───│   Learning   │◀───│   Feedback   │                  │
│  │   Prompts/   │    │    Engine    │    │  Collection  │                  │
│  │   Behaviors  │    │              │    │              │                  │
│  └──────────────┘    └──────────────┘    └──────────────┘                  │
│                             │                                               │
│                             ▼                                               │
│                      ┌──────────────┐                                       │
│                      │  Experience  │                                       │
│                      │    Store     │                                       │
│                      │ (Successes/  │                                       │
│                      │  Failures)   │                                       │
│                      └──────────────┘                                       │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Components

### 1. Experience Store (`core/selfimprove/experience.go`)

Stores successful and failed execution patterns for learning.

```go
// Experience represents a recorded execution with its outcome
type Experience struct {
    ID            string                 `json:"id"`
    AgentID       string                 `json:"agent_id"`
    TaskType      string                 `json:"task_type"`
    Input         interface{}            `json:"input"`
    Output        interface{}            `json:"output"`

    // Execution context
    SystemPrompt  string                 `json:"system_prompt"`
    UserPrompt    string                 `json:"user_prompt"`

    // Outcome metrics
    Success       bool                   `json:"success"`
    Score         float64                `json:"score"`          // 0.0-1.0
    Subscores     map[string]float64     `json:"subscores"`      // quality, efficiency, etc.

    // Feedback (if any)
    HumanRating   *float64               `json:"human_rating,omitempty"`
    Correction    *string                `json:"correction,omitempty"`

    // Metadata
    Timestamp     time.Time              `json:"timestamp"`
    TokensUsed    int                    `json:"tokens_used"`
    LatencyMs     int64                  `json:"latency_ms"`
    Model         string                 `json:"model"`

    // For clustering similar experiences
    Embedding     []float32              `json:"embedding,omitempty"`
}

// ExperienceStore interface for storing and retrieving experiences
type ExperienceStore interface {
    // Store an experience
    Store(ctx context.Context, exp *Experience) error

    // Query experiences
    GetByAgent(ctx context.Context, agentID string, limit int) ([]*Experience, error)
    GetByTaskType(ctx context.Context, taskType string, limit int) ([]*Experience, error)
    GetSuccessful(ctx context.Context, minScore float64, limit int) ([]*Experience, error)
    GetFailed(ctx context.Context, maxScore float64, limit int) ([]*Experience, error)

    // Semantic search for similar experiences
    FindSimilar(ctx context.Context, embedding []float32, limit int) ([]*Experience, error)

    // Statistics
    GetStats(ctx context.Context, agentID string) (*ExperienceStats, error)

    // Pruning
    Prune(ctx context.Context, olderThan time.Time, keepTopN int) error
}

// In-memory implementation for simple use cases
type InMemoryExperienceStore struct { ... }

// PostgreSQL implementation for production
type PostgresExperienceStore struct { ... }
```

### 2. Learning Engine (`core/selfimprove/learning.go`)

Core learning algorithms that analyze experiences and generate improvements.

```go
// LearningStrategy defines how the agent learns
type LearningStrategy string

const (
    // StrategyFewShot: Use successful examples as few-shot prompts
    StrategyFewShot LearningStrategy = "few_shot"

    // StrategyPromptRefinement: Refine prompts based on feedback patterns
    StrategyPromptRefinement LearningStrategy = "prompt_refinement"

    // StrategyPatternExtraction: Extract successful patterns for reuse
    StrategyPatternExtraction LearningStrategy = "pattern_extraction"

    // StrategyReflection: Agent reflects on failures and generates improvements
    StrategyReflection LearningStrategy = "reflection"

    // StrategyMetaLearning: Learn which strategies work for which task types
    StrategyMetaLearning LearningStrategy = "meta_learning"
)

// LearningEngine orchestrates the learning process
type LearningEngine struct {
    config          *LearningConfig
    experienceStore ExperienceStore
    promptManager   *prompts.PromptManager
    llmProvider     llm.Provider
    evaluator       *evaluation.Pipeline
    strategies      map[LearningStrategy]LearningStrategyImpl
}

// LearningConfig configures the learning behavior
type LearningConfig struct {
    Enabled                bool              `json:"enabled"`
    Strategies             []LearningStrategy `json:"strategies"`

    // Thresholds
    MinScoreForSuccess     float64           `json:"min_score_for_success"`     // 0.7 default
    MaxScoreForFailure     float64           `json:"max_score_for_failure"`     // 0.4 default
    MinExperiencesForLearn int               `json:"min_experiences_for_learn"` // 10 default

    // Few-shot settings
    FewShotExamples        int               `json:"few_shot_examples"`         // 3 default

    // Prompt refinement settings
    RefinementInterval     time.Duration     `json:"refinement_interval"`       // 1 hour default
    MaxPromptVersions      int               `json:"max_prompt_versions"`       // 10 default

    // A/B testing
    EnableABTesting        bool              `json:"enable_ab_testing"`
    ABTestMinSamples       int               `json:"ab_test_min_samples"`       // 50 default
    ABTestConfidence       float64           `json:"ab_test_confidence"`        // 0.95 default

    // Safety
    RequireHumanApproval   bool              `json:"require_human_approval"`    // for prompt changes
    MaxAutoImprovements    int               `json:"max_auto_improvements"`     // per day
}

// LearningStrategyImpl interface for pluggable learning strategies
type LearningStrategyImpl interface {
    Name() LearningStrategy

    // Analyze experiences and propose improvements
    Analyze(ctx context.Context, experiences []*Experience) (*ImprovementProposal, error)

    // Apply an approved improvement
    Apply(ctx context.Context, proposal *ImprovementProposal) error

    // Check if this strategy is applicable
    IsApplicable(ctx context.Context, agentID string, taskType string) bool
}

// ImprovementProposal represents a proposed improvement
type ImprovementProposal struct {
    ID              string            `json:"id"`
    Strategy        LearningStrategy  `json:"strategy"`
    AgentID         string            `json:"agent_id"`
    TaskType        string            `json:"task_type"`

    // What's being improved
    ImprovementType ImprovementType   `json:"improvement_type"`

    // The improvement
    CurrentValue    string            `json:"current_value"`    // Current prompt/behavior
    ProposedValue   string            `json:"proposed_value"`   // Proposed change
    Rationale       string            `json:"rationale"`        // Why this improvement

    // Supporting evidence
    SupportingExperiences []string    `json:"supporting_experiences"` // Experience IDs
    ExpectedImprovement   float64     `json:"expected_improvement"`   // Estimated score gain
    Confidence            float64     `json:"confidence"`

    // Status
    Status          ProposalStatus    `json:"status"`
    CreatedAt       time.Time         `json:"created_at"`
    ApprovedAt      *time.Time        `json:"approved_at,omitempty"`
    ApprovedBy      *string           `json:"approved_by,omitempty"`  // "auto" or user ID
}

type ImprovementType string

const (
    ImprovementTypeSystemPrompt  ImprovementType = "system_prompt"
    ImprovementTypeUserPrompt    ImprovementType = "user_prompt"
    ImprovementTypeFewShotExamples ImprovementType = "few_shot_examples"
    ImprovementTypeTaskDecomposition ImprovementType = "task_decomposition"
    ImprovementTypeWorkerSelection ImprovementType = "worker_selection"
    ImprovementTypeRetryStrategy ImprovementType = "retry_strategy"
)

type ProposalStatus string

const (
    ProposalStatusPending   ProposalStatus = "pending"
    ProposalStatusApproved  ProposalStatus = "approved"
    ProposalStatusRejected  ProposalStatus = "rejected"
    ProposalStatusApplied   ProposalStatus = "applied"
    ProposalStatusRolledBack ProposalStatus = "rolled_back"
)
```

### 3. Learning Strategies Implementation

#### 3.1 Few-Shot Learning (`core/selfimprove/strategies/few_shot.go`)

```go
// FewShotStrategy uses successful examples to improve future responses
type FewShotStrategy struct {
    experienceStore ExperienceStore
    embeddingProvider embeddings.Provider
    config *FewShotConfig
}

type FewShotConfig struct {
    NumExamples       int     `json:"num_examples"`       // 3
    MinScore          float64 `json:"min_score"`          // 0.8
    DiversityWeight   float64 `json:"diversity_weight"`   // 0.3 (vs relevance)
    MaxExampleLength  int     `json:"max_example_length"` // tokens
}

func (s *FewShotStrategy) Analyze(ctx context.Context, experiences []*Experience) (*ImprovementProposal, error) {
    // 1. Filter high-quality experiences
    successful := filterByScore(experiences, s.config.MinScore)

    // 2. Cluster by task type
    clusters := clusterByTaskType(successful)

    // 3. Select diverse, representative examples
    examples := selectDiverseExamples(clusters, s.config.NumExamples)

    // 4. Generate few-shot prompt section
    fewShotSection := formatFewShotExamples(examples)

    return &ImprovementProposal{
        Strategy:        StrategyFewShot,
        ImprovementType: ImprovementTypeFewShotExamples,
        ProposedValue:   fewShotSection,
        Rationale:       fmt.Sprintf("Selected %d high-quality examples (avg score: %.2f)", len(examples), avgScore(examples)),
        Confidence:      calculateConfidence(examples),
    }, nil
}
```

#### 3.2 Prompt Refinement (`core/selfimprove/strategies/prompt_refinement.go`)

```go
// PromptRefinementStrategy uses LLM to analyze failures and refine prompts
type PromptRefinementStrategy struct {
    llmProvider llm.Provider
    promptManager *prompts.PromptManager
    config *PromptRefinementConfig
}

func (s *PromptRefinementStrategy) Analyze(ctx context.Context, experiences []*Experience) (*ImprovementProposal, error) {
    // 1. Separate successes and failures
    successes := filterByScore(experiences, 0.7)
    failures := filterByMaxScore(experiences, 0.5)

    if len(failures) == 0 {
        return nil, nil // Nothing to improve
    }

    // 2. Analyze failure patterns using LLM
    analysisPrompt := buildFailureAnalysisPrompt(successes, failures)
    analysis, err := s.llmProvider.GenerateCompletion(ctx, &llm.CompletionRequest{
        SystemPrompt: systemPromptForAnalysis,
        UserPrompt:   analysisPrompt,
        Temperature:  0.3,
    })

    // 3. Generate improved prompt
    refinedPrompt, err := s.generateRefinedPrompt(ctx, currentPrompt, analysis)

    return &ImprovementProposal{
        Strategy:        StrategyPromptRefinement,
        ImprovementType: ImprovementTypeSystemPrompt,
        CurrentValue:    currentPrompt,
        ProposedValue:   refinedPrompt,
        Rationale:       analysis.Text,
    }, nil
}

const systemPromptForAnalysis = `You are an AI system optimizer. Analyze the following successful and failed agent executions to identify patterns.

For each failure, identify:
1. What went wrong
2. What the successful examples did differently
3. Specific prompt modifications that could prevent this failure

Be specific and actionable. Focus on patterns, not individual cases.`
```

#### 3.3 Reflection Strategy (`core/selfimprove/strategies/reflection.go`)

```go
// ReflectionStrategy has the agent reflect on its own failures
type ReflectionStrategy struct {
    llmProvider llm.Provider
}

func (s *ReflectionStrategy) Analyze(ctx context.Context, experiences []*Experience) (*ImprovementProposal, error) {
    // For each failure, ask the agent to reflect
    reflections := make([]string, 0)

    for _, exp := range filterFailed(experiences) {
        reflection, err := s.reflectOnFailure(ctx, exp)
        if err != nil {
            continue
        }
        reflections = append(reflections, reflection)
    }

    // Synthesize reflections into actionable improvements
    synthesis, err := s.synthesizeReflections(ctx, reflections)

    return &ImprovementProposal{
        Strategy:  StrategyReflection,
        Rationale: synthesis,
    }, nil
}

const reflectionPrompt = `You are reviewing a past execution that did not meet quality standards.

Original Task: %s
Your Response: %s
Score: %.2f
Feedback: %s

Reflect on what went wrong and how you could improve. Be specific:
1. What did you misunderstand about the task?
2. What information were you missing?
3. What would you do differently next time?
4. What changes to your instructions would help?`
```

### 4. Self-Improving Agent Wrapper (`core/selfimprove/agent.go`)

```go
// SelfImprovingAgent wraps any agent with self-improvement capabilities
type SelfImprovingAgent struct {
    // Underlying agent
    baseAgent       multiagent.TaskHandler
    agentID         string

    // Self-improvement components
    learningEngine  *LearningEngine
    experienceStore ExperienceStore

    // Configuration
    config          *SelfImprovementConfig

    // State
    currentPrompt   string
    fewShotExamples []*Experience

    // Metrics
    metrics         *SelfImprovementMetrics
}

type SelfImprovementConfig struct {
    Enabled                bool
    LearningConfig         *LearningConfig

    // When to learn
    LearnAfterEveryN       int           // Learn after N executions (0 = disabled)
    LearnOnSchedule        time.Duration // Learn on schedule (0 = disabled)
    LearnOnFeedback        bool          // Learn immediately when feedback received

    // Safety limits
    MaxImprovementsPerDay  int
    RequireApprovalAbove   float64       // Score threshold requiring human approval

    // Rollback
    AutoRollbackOnRegression bool
    RegressionThreshold      float64     // Score drop to trigger rollback
}

// HandleTask wraps the base agent's task handling with learning
func (a *SelfImprovingAgent) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
    startTime := time.Now()

    // 1. Enhance prompt with learned context
    enhancedTask := a.enhanceTask(ctx, task)

    // 2. Execute with base agent
    result, err := a.baseAgent.HandleTask(ctx, enhancedTask)

    // 3. Record experience (async)
    go a.recordExperience(ctx, task, result, err, time.Since(startTime))

    // 4. Check if learning is needed
    if a.shouldLearn() {
        go a.triggerLearning(ctx)
    }

    return result, err
}

// enhanceTask adds few-shot examples and learned context to the task
func (a *SelfImprovingAgent) enhanceTask(ctx context.Context, task *multiagent.Task) *multiagent.Task {
    if !a.config.Enabled {
        return task
    }

    enhanced := *task

    // Add few-shot examples if available
    if len(a.fewShotExamples) > 0 {
        enhanced.Metadata["few_shot_examples"] = formatExamples(a.fewShotExamples)
    }

    // Add any learned context
    if a.currentPrompt != "" {
        enhanced.Metadata["enhanced_prompt"] = a.currentPrompt
    }

    return &enhanced
}
```

### 5. Self-Improving Orchestrator (`core/selfimprove/orchestrator.go`)

```go
// SelfImprovingOrchestrator learns to decompose tasks better
type SelfImprovingOrchestrator struct {
    *multiagent.Orchestrator

    learningEngine  *LearningEngine
    experienceStore ExperienceStore
    config          *OrchestratorLearningConfig

    // Learned patterns
    decompositionPatterns map[string]*DecompositionPattern
    workerAffinities      map[string]map[string]float64 // taskType -> workerID -> score
}

type OrchestratorLearningConfig struct {
    *SelfImprovementConfig

    // Orchestrator-specific settings
    LearnDecompositionPatterns bool
    LearnWorkerAffinities      bool
    LearnRetryStrategies       bool
}

// DecompositionPattern represents a learned task decomposition pattern
type DecompositionPattern struct {
    TaskType        string            `json:"task_type"`
    InputPattern    string            `json:"input_pattern"`    // Regex or semantic matcher
    Decomposition   []*SubtaskPattern `json:"decomposition"`
    SuccessRate     float64           `json:"success_rate"`
    AvgScore        float64           `json:"avg_score"`
    UsageCount      int               `json:"usage_count"`
}

type SubtaskPattern struct {
    Name           string   `json:"name"`
    Description    string   `json:"description"`
    RequiredCaps   []string `json:"required_capabilities"`
    Dependencies   []string `json:"dependencies"`
    Priority       int      `json:"priority"`
}

// planTask overrides the base orchestrator's planning with learned patterns
func (o *SelfImprovingOrchestrator) planTask(ctx context.Context, task *multiagent.Task) ([]*multiagent.Task, error) {
    // 1. Check for matching learned pattern
    pattern := o.findMatchingPattern(task)
    if pattern != nil && pattern.SuccessRate > 0.8 {
        // Use learned pattern
        return o.applyPattern(ctx, task, pattern)
    }

    // 2. Fall back to LLM-based planning
    subtasks, err := o.Orchestrator.planTask(ctx, task)
    if err != nil {
        return nil, err
    }

    // 3. Record this decomposition for learning
    go o.recordDecomposition(ctx, task, subtasks)

    return subtasks, nil
}

// findWorkerForTask uses learned worker affinities
func (o *SelfImprovingOrchestrator) findWorkerForTask(task *multiagent.Task) (*multiagent.AgentMetadata, error) {
    taskType := task.Type

    // Check learned affinities
    if affinities, ok := o.workerAffinities[taskType]; ok {
        // Find worker with highest affinity
        bestWorker, bestScore := o.selectByAffinity(affinities)
        if bestScore > 0.7 {
            return bestWorker, nil
        }
    }

    // Fall back to capability-based selection
    return o.Orchestrator.findWorkerForTask(task)
}
```

### 6. Integration Hooks (`core/selfimprove/hooks.go`)

```go
// SelfImprovementHook implements evaluation.EvaluationHook
type SelfImprovementHook struct {
    learningEngine  *LearningEngine
    experienceStore ExperienceStore
    feedbackStore   *humanloop.FeedbackStore
}

// OnEvaluationComplete is called after each evaluation
func (h *SelfImprovementHook) OnEvaluationComplete(
    ctx context.Context,
    trace *tracing.Trace,
    evals []*evaluation.Evaluation,
) {
    // Convert evaluation to experience
    exp := h.traceToExperience(trace, evals)

    // Store experience
    h.experienceStore.Store(ctx, exp)

    // Notify learning engine
    h.learningEngine.OnNewExperience(ctx, exp)
}

// FeedbackHandler implements humanloop.FeedbackHandler
type SelfImprovementFeedbackHandler struct {
    learningEngine  *LearningEngine
    experienceStore ExperienceStore
}

// HandleFeedback processes human feedback for learning
func (h *SelfImprovementFeedbackHandler) HandleFeedback(
    ctx context.Context,
    feedback *humanloop.Feedback,
) error {
    // Update experience with feedback
    exp, err := h.experienceStore.GetByTraceID(ctx, feedback.TraceID)
    if err != nil {
        return err
    }

    // Apply feedback
    switch feedback.Type {
    case humanloop.FeedbackTypeRating:
        exp.HumanRating = &feedback.Rating
    case humanloop.FeedbackTypeCorrection:
        exp.Correction = &feedback.CorrectedOutput
    }

    // Re-store updated experience
    h.experienceStore.Store(ctx, exp)

    // Trigger immediate learning if configured
    if h.learningEngine.config.LearnOnFeedback {
        h.learningEngine.TriggerLearning(ctx, exp.AgentID)
    }

    return nil
}
```

---

## Implementation Phases

### Phase 1: Foundation (Week 1-2)
- [ ] Implement `ExperienceStore` interface and in-memory implementation
- [ ] Implement `LearningEngine` core structure
- [ ] Implement `SelfImprovementConfig` with defaults
- [ ] Add configuration flags to make everything optional

### Phase 2: Basic Learning (Week 3-4)
- [ ] Implement `FewShotStrategy`
- [ ] Implement `SelfImprovingAgent` wrapper
- [ ] Implement `SelfImprovementHook` for evaluation integration
- [ ] Add basic metrics collection

### Phase 3: Advanced Learning (Week 5-6)
- [ ] Implement `PromptRefinementStrategy`
- [ ] Implement `ReflectionStrategy`
- [ ] Implement `SelfImprovingOrchestrator`
- [ ] Add A/B testing integration

### Phase 4: Production Readiness (Week 7-8)
- [ ] Implement PostgreSQL `ExperienceStore`
- [ ] Add human approval workflow
- [ ] Implement automatic rollback
- [ ] Add comprehensive metrics and alerting
- [ ] Write documentation and examples

---

## API Examples

### Basic Usage

```go
// Create a self-improving worker
baseWorker := multiagent.NewCoderWorker(llmProvider)

selfImprovingWorker := selfimprove.NewSelfImprovingAgent(
    baseWorker,
    "coder-001",
    selfimprove.WithExperienceStore(experienceStore),
    selfimprove.WithLearningEngine(learningEngine),
    selfimprove.WithConfig(&selfimprove.SelfImprovementConfig{
        Enabled:             true,
        LearnAfterEveryN:    100,
        LearnOnFeedback:     true,
        MaxImprovementsPerDay: 5,
    }),
)

// Use like any other worker
worker := multiagent.NewWorkerAgent(metadata, protocol, selfImprovingWorker)
```

### Self-Improving Orchestrator

```go
// Create self-improving orchestrator
orchestrator := selfimprove.NewSelfImprovingOrchestrator(
    protocol,
    llmProvider,
    orchestratorConfig,
    selfimprove.WithOrchestratorLearning(&selfimprove.OrchestratorLearningConfig{
        SelfImprovementConfig: &selfimprove.SelfImprovementConfig{
            Enabled: true,
        },
        LearnDecompositionPatterns: true,
        LearnWorkerAffinities:      true,
    }),
)
```

### Manual Learning Trigger

```go
// Manually trigger learning
proposals, err := learningEngine.AnalyzeAndPropose(ctx, "coder-001")
for _, proposal := range proposals {
    fmt.Printf("Proposal: %s\n", proposal.Rationale)
    fmt.Printf("Expected improvement: %.2f\n", proposal.ExpectedImprovement)

    // Approve and apply
    if proposal.Confidence > 0.8 {
        learningEngine.ApproveAndApply(ctx, proposal.ID, "auto")
    }
}
```

### Monitoring Self-Improvement

```go
// Get self-improvement metrics
metrics := selfImprovingWorker.GetMetrics()
fmt.Printf("Total improvements: %d\n", metrics.TotalImprovements)
fmt.Printf("Avg score before: %.2f\n", metrics.AvgScoreBefore)
fmt.Printf("Avg score after: %.2f\n", metrics.AvgScoreAfter)
fmt.Printf("Rollbacks: %d\n", metrics.Rollbacks)
```

---

## Safety Considerations

### 1. Guardrails

- **Rate limiting**: Max improvements per day
- **Human approval**: For high-impact changes
- **Automatic rollback**: On score regression
- **Audit logging**: All changes tracked

### 2. Monitoring

- Score trends over time
- Improvement success rate
- Rollback frequency
- Human override frequency

### 3. Kill Switch

```go
// Disable all self-improvement immediately
selfimprove.DisableGlobally()

// Or per-agent
selfImprovingWorker.DisableLearning()
```

---

## File Structure

```
core/selfimprove/
├── config.go           # Configuration types
├── experience.go       # Experience storage
├── learning.go         # Learning engine
├── agent.go            # Self-improving agent wrapper
├── orchestrator.go     # Self-improving orchestrator
├── hooks.go            # Integration hooks
├── metrics.go          # Metrics collection
├── proposal.go         # Improvement proposals
├── strategies/
│   ├── interface.go    # Strategy interface
│   ├── few_shot.go     # Few-shot learning
│   ├── prompt_refinement.go # Prompt refinement
│   ├── reflection.go   # Agent reflection
│   └── meta_learning.go # Meta-learning
├── store/
│   ├── memory.go       # In-memory store
│   └── postgres.go     # PostgreSQL store
└── selfimprove_test.go # Tests
```

---

## Success Metrics

1. **Score Improvement**: Avg evaluation score increases over time
2. **Learning Efficiency**: Fewer experiences needed to improve
3. **Stability**: Low rollback rate (<5%)
4. **Human Alignment**: High approval rate for proposals (>90%)
5. **Cost Efficiency**: Improvement in tokens/task ratio

---

## Non-Goals (Out of Scope)

- Fine-tuning LLM weights (this uses in-context learning only)
- Real-time continuous learning (batch-based)
- Multi-tenant learning isolation (single-tenant focus)
- Federated learning across deployments
