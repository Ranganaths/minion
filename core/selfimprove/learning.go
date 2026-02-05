package selfimprove

import (
	"context"
	"errors"
	"sync"
	"time"
)

// LearningEngine orchestrates the learning process for self-improving agents.
// It manages strategies, coordinates learning triggers, and handles proposals.
type LearningEngine struct {
	mu sync.RWMutex

	// Configuration
	config *LearningConfig

	// Dependencies
	experienceStore ExperienceStore
	proposalStore   ProposalStore

	// Registered strategies
	strategies map[LearningStrategy]StrategyImpl

	// Learning state per agent
	agentState map[string]*AgentLearningState

	// Hooks for events
	hooks []LearningHook

	// Running state
	running bool
	stopCh  chan struct{}
}

// StrategyImpl defines the interface for learning strategy implementations.
type StrategyImpl interface {
	// Name returns the strategy identifier
	Name() LearningStrategy

	// Analyze examines experiences and proposes improvements
	Analyze(ctx context.Context, experiences []*Experience) (*ImprovementProposal, error)

	// Apply implements an approved improvement
	Apply(ctx context.Context, proposal *ImprovementProposal) error

	// IsApplicable checks if this strategy can be used
	IsApplicable(ctx context.Context, agentID string, taskType string) bool
}

// AgentLearningState tracks learning state for a specific agent.
type AgentLearningState struct {
	AgentID             string
	ExecutionCount      int
	LastLearningTime    time.Time
	ImprovementsToday   int
	LastImprovementTime time.Time
	CurrentPromptVersion string
	PendingProposals    []*ImprovementProposal
}

// LearningHook allows external code to observe learning events.
type LearningHook interface {
	OnNewExperience(ctx context.Context, exp *Experience)
	OnProposalCreated(ctx context.Context, proposal *ImprovementProposal)
	OnProposalApplied(ctx context.Context, proposal *ImprovementProposal)
	OnLearningTriggered(ctx context.Context, agentID string)
}

// NewLearningEngine creates a new learning engine.
func NewLearningEngine(config *LearningConfig, experienceStore ExperienceStore) *LearningEngine {
	if config == nil {
		config = DefaultLearningConfig()
	}

	return &LearningEngine{
		config:          config,
		experienceStore: experienceStore,
		proposalStore:   NewInMemoryProposalStore(),
		strategies:      make(map[LearningStrategy]StrategyImpl),
		agentState:      make(map[string]*AgentLearningState),
		hooks:           make([]LearningHook, 0),
		stopCh:          make(chan struct{}),
	}
}

// WithProposalStore sets the proposal store.
func (e *LearningEngine) WithProposalStore(store ProposalStore) *LearningEngine {
	e.proposalStore = store
	return e
}

// RegisterStrategy registers a learning strategy.
func (e *LearningEngine) RegisterStrategy(strategy StrategyImpl) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.strategies[strategy.Name()] = strategy
}

// AddHook adds a learning hook.
func (e *LearningEngine) AddHook(hook LearningHook) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.hooks = append(e.hooks, hook)
}

// Start begins the learning engine's background processes.
func (e *LearningEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return errors.New("learning engine already running")
	}
	e.running = true
	e.stopCh = make(chan struct{})
	e.mu.Unlock()

	// Start scheduled learning if configured
	if e.config.RefinementInterval > 0 {
		go e.scheduledLearningLoop(ctx)
	}

	return nil
}

// Stop halts the learning engine.
func (e *LearningEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		close(e.stopCh)
		e.running = false
	}
}

// OnNewExperience handles a new experience being recorded.
func (e *LearningEngine) OnNewExperience(ctx context.Context, exp *Experience) {
	// Notify hooks
	e.mu.RLock()
	hooks := make([]LearningHook, len(e.hooks))
	copy(hooks, e.hooks)
	e.mu.RUnlock()

	for _, hook := range hooks {
		hook.OnNewExperience(ctx, exp)
	}

	// Update agent state
	e.mu.Lock()
	state, ok := e.agentState[exp.AgentID]
	if !ok {
		state = &AgentLearningState{AgentID: exp.AgentID}
		e.agentState[exp.AgentID] = state
	}
	state.ExecutionCount++
	e.mu.Unlock()
}

// TriggerLearning initiates a learning cycle for an agent.
func (e *LearningEngine) TriggerLearning(ctx context.Context, agentID string) error {
	if IsGloballyDisabled() {
		return errors.New("self-improvement is globally disabled")
	}

	// Notify hooks
	e.mu.RLock()
	hooks := make([]LearningHook, len(e.hooks))
	copy(hooks, e.hooks)
	e.mu.RUnlock()

	for _, hook := range hooks {
		hook.OnLearningTriggered(ctx, agentID)
	}

	// Get recent experiences for this agent
	experiences, err := e.experienceStore.GetByAgent(ctx, agentID, 100)
	if err != nil {
		return err
	}

	// Check minimum experiences threshold
	if len(experiences) < e.config.MinExperiencesForLearn {
		return nil // Not enough data to learn
	}

	// Run each enabled strategy
	var proposals []*ImprovementProposal

	e.mu.RLock()
	enabledStrategies := e.config.Strategies
	strategies := make(map[LearningStrategy]StrategyImpl)
	for k, v := range e.strategies {
		strategies[k] = v
	}
	e.mu.RUnlock()

	for _, strategyName := range enabledStrategies {
		strategy, ok := strategies[strategyName]
		if !ok {
			continue
		}

		if !strategy.IsApplicable(ctx, agentID, "") {
			continue
		}

		proposal, err := strategy.Analyze(ctx, experiences)
		if err != nil {
			continue // Log error but continue with other strategies
		}

		if proposal != nil {
			proposal.AgentID = agentID
			proposals = append(proposals, proposal)
		}
	}

	// Store proposals
	for _, proposal := range proposals {
		if err := e.proposalStore.Save(proposal); err != nil {
			continue
		}

		// Notify hooks
		for _, hook := range hooks {
			hook.OnProposalCreated(ctx, proposal)
		}
	}

	// Update learning state
	e.mu.Lock()
	if state, ok := e.agentState[agentID]; ok {
		state.LastLearningTime = time.Now()
		state.PendingProposals = proposals
	}
	e.mu.Unlock()

	return nil
}

// AnalyzeAndPropose runs analysis and returns proposals without applying them.
func (e *LearningEngine) AnalyzeAndPropose(ctx context.Context, agentID string) ([]*ImprovementProposal, error) {
	if IsGloballyDisabled() {
		return nil, errors.New("self-improvement is globally disabled")
	}

	// Get recent experiences
	experiences, err := e.experienceStore.GetByAgent(ctx, agentID, 100)
	if err != nil {
		return nil, err
	}

	if len(experiences) < e.config.MinExperiencesForLearn {
		return nil, nil
	}

	var proposals []*ImprovementProposal

	e.mu.RLock()
	enabledStrategies := e.config.Strategies
	strategies := make(map[LearningStrategy]StrategyImpl)
	for k, v := range e.strategies {
		strategies[k] = v
	}
	e.mu.RUnlock()

	for _, strategyName := range enabledStrategies {
		strategy, ok := strategies[strategyName]
		if !ok {
			continue
		}

		if !strategy.IsApplicable(ctx, agentID, "") {
			continue
		}

		proposal, err := strategy.Analyze(ctx, experiences)
		if err != nil {
			continue
		}

		if proposal != nil {
			proposal.AgentID = agentID
			proposals = append(proposals, proposal)
		}
	}

	return proposals, nil
}

// ApproveAndApply approves a proposal and applies it.
func (e *LearningEngine) ApproveAndApply(ctx context.Context, proposalID string, approver string) error {
	if IsGloballyDisabled() {
		return errors.New("self-improvement is globally disabled")
	}

	// Get the proposal
	proposal, err := e.proposalStore.Get(proposalID)
	if err != nil {
		return err
	}
	if proposal == nil {
		return errors.New("proposal not found")
	}

	// Check daily limit
	e.mu.RLock()
	state := e.agentState[proposal.AgentID]
	e.mu.RUnlock()

	if state != nil && state.ImprovementsToday >= e.config.MaxAutoImprovements {
		return errors.New("daily improvement limit reached")
	}

	// Approve the proposal
	proposal.Approve(approver)

	// Get the strategy to apply the proposal
	e.mu.RLock()
	strategy, ok := e.strategies[proposal.Strategy]
	e.mu.RUnlock()

	if !ok {
		return errors.New("strategy not found")
	}

	// Apply the improvement
	if err := strategy.Apply(ctx, proposal); err != nil {
		proposal.Reject("failed to apply: " + err.Error())
		e.proposalStore.Update(proposal)
		return err
	}

	// Mark as applied
	proposal.MarkApplied()
	if err := e.proposalStore.Update(proposal); err != nil {
		return err
	}

	// Update state
	e.mu.Lock()
	if state == nil {
		state = &AgentLearningState{AgentID: proposal.AgentID}
		e.agentState[proposal.AgentID] = state
	}
	state.ImprovementsToday++
	state.LastImprovementTime = time.Now()
	e.mu.Unlock()

	// Notify hooks
	e.mu.RLock()
	hooks := make([]LearningHook, len(e.hooks))
	copy(hooks, e.hooks)
	e.mu.RUnlock()

	for _, hook := range hooks {
		hook.OnProposalApplied(ctx, proposal)
	}

	return nil
}

// RejectProposal rejects a proposal with a reason.
func (e *LearningEngine) RejectProposal(proposalID string, reason string) error {
	proposal, err := e.proposalStore.Get(proposalID)
	if err != nil {
		return err
	}
	if proposal == nil {
		return errors.New("proposal not found")
	}

	proposal.Reject(reason)
	return e.proposalStore.Update(proposal)
}

// GetPendingProposals returns pending proposals for review.
func (e *LearningEngine) GetPendingProposals(limit int) ([]*ImprovementProposal, error) {
	return e.proposalStore.GetPending(limit)
}

// GetAgentProposals returns proposals for a specific agent.
func (e *LearningEngine) GetAgentProposals(agentID string, status *ProposalStatus, limit int) ([]*ImprovementProposal, error) {
	return e.proposalStore.GetByAgent(agentID, status, limit)
}

// GetAgentState returns the learning state for an agent.
func (e *LearningEngine) GetAgentState(agentID string) *AgentLearningState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.agentState[agentID]
}

// ResetDailyLimits resets the daily improvement counters.
func (e *LearningEngine) ResetDailyLimits() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, state := range e.agentState {
		state.ImprovementsToday = 0
	}
}

// scheduledLearningLoop runs learning on a schedule.
func (e *LearningEngine) scheduledLearningLoop(ctx context.Context) {
	ticker := time.NewTicker(e.config.RefinementInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.runScheduledLearning(ctx)
		}
	}
}

// runScheduledLearning triggers learning for all agents.
func (e *LearningEngine) runScheduledLearning(ctx context.Context) {
	e.mu.RLock()
	agents := make([]string, 0, len(e.agentState))
	for agentID := range e.agentState {
		agents = append(agents, agentID)
	}
	e.mu.RUnlock()

	for _, agentID := range agents {
		_ = e.TriggerLearning(ctx, agentID)
	}
}

// ShouldLearn checks if learning should be triggered for an agent.
func (e *LearningEngine) ShouldLearn(agentID string, learnAfterEveryN int) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	state, ok := e.agentState[agentID]
	if !ok {
		return false
	}

	// Check execution count threshold
	if learnAfterEveryN > 0 && state.ExecutionCount >= learnAfterEveryN {
		return true
	}

	return false
}

// GetExperienceStore returns the experience store.
func (e *LearningEngine) GetExperienceStore() ExperienceStore {
	return e.experienceStore
}

// GetConfig returns the learning configuration.
func (e *LearningEngine) GetConfig() *LearningConfig {
	return e.config
}
