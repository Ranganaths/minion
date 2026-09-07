package swarm

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSwarmFull            = errors.New("swarm has reached maximum capacity")
	ErrAgentAlreadyExists   = errors.New("agent already registered in swarm")
	ErrAgentNotFound        = errors.New("agent not found in swarm")
	ErrNoEligibleAgents      = errors.New("no eligible agents for task")
	ErrInvalidConfig        = errors.New("invalid swarm configuration")
	ErrContextCancelled     = errors.New("context cancelled")
)

type swarmImpl struct {
	id       SwarmID
	name     string
	tags     []Tag
	config   *SwarmConfig

	mu       sync.RWMutex
	agents   map[AgentID]Agent
	healthy  map[AgentID]bool

	executor     TaskExecutor
	aggregator   ResultAggregator

	stats       SwarmStats
	statsMu     sync.Mutex

	logger      Logger
	metrics     MetricsCollector

	ctx        context.Context
	cancel     context.CancelFunc
	running    atomic.Bool
}

type Logger interface {
	Info(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Debug(msg string, fields ...Field)
}

type Field struct {
	Key   string
	Value interface{}
}

type MetricsCollector interface {
	RecordSwarmTask(ctx context.Context, swarmID, taskType, status string, duration time.Duration)
	RecordSwarmAgentEvent(ctx context.Context, swarmID, agentID, event string)
}

func NewSwarm(ctx context.Context, config *SwarmConfig, options ...SwarmOption) (Swarm, error) {
	if config == nil {
		config = DefaultSwarmConfig()
	}

	if config.ID == "" {
		config.ID = SwarmID(uuid.New().String())
	}

	if config.Name == "" {
		config.Name = fmt.Sprintf("swarm-%s", config.ID)
	}

	if config.TaskTimeout == 0 {
		config.TaskTimeout = 30 * time.Second
	}

	if config.MaxConcurrentTasks == 0 {
		config.MaxConcurrentTasks = 5
	}

	swarmCtx, cancel := context.WithCancel(ctx)

	s := &swarmImpl{
		id:       config.ID,
		name:     config.Name,
		tags:     config.Tags,
		config:   config,
		agents:   make(map[AgentID]Agent),
		healthy:  make(map[AgentID]bool),
		executor: NewTaskExecutor(config.MaxConcurrentTasks),
		aggregator: NewResultAggregator(),
		logger:    defaultLogger,
		stats: SwarmStats{
			TotalTasks: 0,
		},
		ctx:    swarmCtx,
		cancel: cancel,
	}

	for _, opt := range options {
		opt(s)
	}

	s.running.Store(true)

	return s, nil
}

type SwarmOption func(*swarmImpl)

func WithLogger(logger Logger) SwarmOption {
	return func(s *swarmImpl) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func WithMetrics(m MetricsCollector) SwarmOption {
	return func(s *swarmImpl) {
		if m != nil {
			s.metrics = m
		}
	}
}

func WithTaskExecutor(executor TaskExecutor) SwarmOption {
	return func(s *swarmImpl) {
		if executor != nil {
			s.executor = executor
		}
	}
}

func WithAggregator(aggregator ResultAggregator) SwarmOption {
	return func(s *swarmImpl) {
		if aggregator != nil {
			s.aggregator = aggregator
		}
	}
}

func (s *swarmImpl) ID() SwarmID {
	return s.id
}

func (s *swarmImpl) Name() string {
	return s.name
}

func (s *swarmImpl) Tags() []Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tags := make([]Tag, len(s.tags))
	copy(tags, s.tags)
	return tags
}

func (s *swarmImpl) RegisterAgent(ctx context.Context, agent Agent) error {
	if !s.running.Load() {
		return ErrContextCancelled
	}

	if agent == nil {
		return ErrInvalidConfig
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.agents) >= s.config.MaxAgents {
		return fmt.Errorf("%w: %d/%d", ErrSwarmFull, len(s.agents), s.config.MaxAgents)
	}

	agentID := agent.ID()
	if _, exists := s.agents[agentID]; exists {
		return fmt.Errorf("%w: %s", ErrAgentAlreadyExists, agentID)
	}

	s.agents[agentID] = agent
	s.healthy[agentID] = true

	s.logger.Info("Agent registered to swarm",
		String("swarm_id", string(s.id)),
		String("agent_id", agentID),
		String("agent_name", agent.Name()),
	)

	if s.metrics != nil {
		s.metrics.RecordSwarmAgentEvent(ctx, string(s.id), agentID, "registered")
	}

	return nil
}

func (s *swarmImpl) UnregisterAgent(ctx context.Context, agentID AgentID) error {
	if !s.running.Load() {
		return ErrContextCancelled
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	agent, exists := s.agents[agentID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrAgentNotFound, agentID)
	}

	delete(s.agents, agentID)
	delete(s.healthy, agentID)

	s.logger.Info("Agent unregistered from swarm",
		String("swarm_id", string(s.id)),
		String("agent_id", agentID),
		String("agent_name", agent.Name()),
	)

	if s.metrics != nil {
		s.metrics.RecordSwarmAgentEvent(ctx, string(s.id), agentID, "unregistered")
	}

	return nil
}

func (s *swarmImpl) GetAgent(agentID AgentID) (Agent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent, exists := s.agents[agentID]
	return agent, exists
}

func (s *swarmImpl) ListAgents() []Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]Agent, 0, len(s.agents))
	for _, agent := range s.agents {
		agents = append(agents, agent)
	}
	return agents
}

func (s *swarmImpl) AgentCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.agents)
}

func (s *swarmImpl) Execute(ctx context.Context, task *Task) (*AggregatedResult, error) {
	return s.ExecuteWithStrategy(ctx, task, s.config.AggregationStrategy)
}

func (s *swarmImpl) ExecuteWithStrategy(ctx context.Context, task *Task, strategy AggregationStrategy) (*AggregatedResult, error) {
	start := time.Now()

	if !s.running.Load() {
		return nil, ErrContextCancelled
	}

	if task == nil {
		return nil, ErrInvalidConfig
	}

	if task.ID == "" {
		task.ID = TaskID(uuid.New().String())
	}

	s.logger.Info("Executing swarm task",
		String("swarm_id", string(s.id)),
		String("task_id", string(task.ID)),
		String("task_type", task.Type),
		String("strategy", string(strategy)),
	)

	agents := s.selectAgents(task)
	if len(agents) < s.config.MinAgents {
		return nil, fmt.Errorf("%w: found %d, need %d", ErrNoEligibleAgents, len(agents), s.config.MinAgents)
	}

	executionID := ExecutionID(uuid.New().String())
	execCtx, cancel := context.WithTimeout(ctx, s.config.TaskTimeout)
	defer cancel()

	resultCh, err := s.executor.ExecuteWithTimeout(execCtx, task, agents, s.config.TaskTimeout/time.Duration(len(agents)))
	if err != nil {
		return nil, fmt.Errorf("failed to execute task: %w", err)
	}

	var results []*Result
	for result := range resultCh {
		result.ExecutionID = executionID
		if result.Success {
			s.recordSuccess(result.Duration)
		} else {
			s.recordFailure()
		}
		results = append(results, result)
	}

	aggregated, err := s.aggregator.Aggregate(results, strategy)
	if err != nil {
		return nil, fmt.Errorf("aggregation failed: %w", err)
	}

	aggregated.TaskID = task.ID
	aggregated.Strategy = strategy

	duration := time.Since(start)
	s.logger.Info("Swarm task completed",
		String("swarm_id", string(s.id)),
		String("task_id", string(task.ID)),
		Int("total_agents", len(agents)),
		Int("success_count", aggregated.SuccessCount),
		Int("failure_count", aggregated.FailureCount),
		Duration("duration", duration),
	)

	if s.metrics != nil {
		status := "success"
		if aggregated.FailureCount == aggregated.TotalAgents {
			status = "failed"
		} else if aggregated.FailureCount > 0 {
			status = "partial"
		}
		s.metrics.RecordSwarmTask(ctx, string(s.id), task.Type, status, duration)
	}

	return aggregated, nil
}

func (s *swarmImpl) selectAgents(task *Task) []Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var candidates []Agent

	for _, agent := range s.agents {
		if !s.healthy[agent.ID()] {
			continue
		}
		if s.isAgentEligible(agent, task) {
			candidates = append(candidates, agent)
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	switch s.config.DiscoveryPolicy {
	case DiscoveryAll:
		return candidates

	case DiscoveryRandom:
		shuffleAgents(candidates)
		count := min(len(candidates), s.config.MaxAgents)
		return candidates[:count]

	case DiscoveryCapable:
		if len(task.RequiredCapabilities) > 0 {
			var capable []Agent
			for _, agent := range candidates {
				if hasCapabilities(agent, task.RequiredCapabilities) {
					capable = append(capable, agent)
				}
			}
			if len(capable) > 0 {
				return capable
			}
		}
		return candidates

	case DiscoveryTagged:
		if len(task.RequiredTags) > 0 {
			var tagged []Agent
			for _, agent := range candidates {
				if hasTags(agent, task.RequiredTags) {
					tagged = append(tagged, agent)
				}
			}
			if len(tagged) > 0 {
				return tagged
			}
		}
		return candidates

	default:
		return candidates
	}
}

func (s *swarmImpl) isAgentEligible(agent Agent, task *Task) bool {
	if len(task.RequiredTags) > 0 && !hasAnyTag(agent, task.RequiredTags) {
		return false
	}

	if len(task.RequiredCapabilities) > 0 && !hasCapabilities(agent, task.RequiredCapabilities) {
		return false
	}

	return true
}

func (s *swarmImpl) FindByTag(tag Tag) []Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []Agent
	for _, agent := range s.agents {
		if hasTag(agent, tag) {
			results = append(results, agent)
		}
	}
	return results
}

func (s *swarmImpl) FindByTags(tags ...Tag) []Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []Agent
	for _, agent := range s.agents {
		if hasTags(agent, tags) {
			results = append(results, agent)
		}
	}
	return results
}

func (s *swarmImpl) FindByCapabilities(capabilities ...Capability) []Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []Agent
	for _, agent := range s.agents {
		if hasCapabilities(agent, capabilities) {
			results = append(results, agent)
		}
	}
	return results
}

func (s *swarmImpl) Stats() *SwarmStats {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	s.mu.RLock()
	active := len(s.healthy)
	total := len(s.agents)
	s.mu.RUnlock()

	stats := s.stats
	stats.ActiveAgents = active
	stats.TotalAgents = total

	if stats.TotalTasks > 0 {
		stats.SuccessRate = float64(stats.CompletedTasks) / float64(stats.TotalTasks)
	}

	return &stats
}

func (s *swarmImpl) Health(ctx context.Context) *HealthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &HealthStatus{
		Status:     "healthy",
		Components: make(map[string]string),
		Errors:     []string{},
	}

	for agentID, isHealthy := range s.healthy {
		if !isHealthy {
			status.Components["agent:"+agentID] = "unhealthy"
			status.Errors = append(status.Errors, fmt.Sprintf("agent %s is unhealthy", agentID))
		}
	}

	if len(status.Errors) > 0 {
		status.Status = "degraded"
	}

	if !s.running.Load() {
		status.Status = "shutdown"
	}

	return status
}

func (s *swarmImpl) Shutdown(ctx context.Context) error {
	if !s.running.CompareAndSwap(true, false) {
		return nil
	}

	s.cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	for agentID := range s.agents {
		delete(s.healthy, agentID)
	}

	s.logger.Info("Swarm shutdown complete",
		String("swarm_id", string(s.id)),
	)

	return nil
}

func (s *swarmImpl) recordSuccess(duration time.Duration) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	s.stats.TotalTasks++
	s.stats.CompletedTasks++

	if s.stats.CompletedTasks > 1 {
		currentAvg := s.stats.AverageLatency
		count := float64(s.stats.CompletedTasks)
		s.stats.AverageLatency = time.Duration((float64(currentAvg) * (count - 1) / count) + (float64(duration) / count))
	} else {
		s.stats.AverageLatency = duration
	}
}

func (s *swarmImpl) recordFailure() {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	s.stats.TotalTasks++
	s.stats.FailedTasks++
}

func hasTag(agent Agent, tag Tag) bool {
	for _, t := range agent.Tags() {
		if t == tag {
			return true
		}
	}
	return false
}

func hasTags(agent Agent, tags []Tag) bool {
	if len(tags) == 0 {
		return true
	}

	agentTags := make(map[Tag]bool)
	for _, t := range agent.Tags() {
		agentTags[t] = true
	}

	for _, tag := range tags {
		if !agentTags[tag] {
			return false
		}
	}
	return true
}

func hasAnyTag(agent Agent, tags []Tag) bool {
	for _, t := range agent.Tags() {
		for _, tag := range tags {
			if t == tag {
				return true
			}
		}
	}
	return false
}

func hasCapabilities(agent Agent, caps []Capability) bool {
	if len(caps) == 0 {
		return true
	}

	agentCaps := make(map[Capability]bool)
	for _, c := range agent.Capabilities() {
		agentCaps[c] = true
	}

	for _, cap := range caps {
		if !agentCaps[cap] {
			return false
		}
	}
	return true
}

func shuffleAgents(agents []Agent) {
	rand.Shuffle(len(agents), func(i, j int) {
		agents[i], agents[j] = agents[j], agents[i]
	})
}

var defaultLogger Logger = &stdLogger{}

type stdLogger struct{}

func (l *stdLogger) Info(msg string, fields ...Field)  {}
func (l *stdLogger) Error(msg string, fields ...Field) {}
func (l *stdLogger) Warn(msg string, fields ...Field)  {}
func (l *stdLogger) Debug(msg string, fields ...Field) {}

func String(key, val string) Field {
	return Field{Key: key, Value: val}
}

func Int(key string, val int) Field {
	return Field{Key: key, Value: val}
}

func Duration(key string, val time.Duration) Field {
	return Field{Key: key, Value: val}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type rankedResult struct {
	result *Result
	score  float64
}

type votingResult struct {
	value  string
	count  int
}

func rankResults(results []*Result) []rankedResult {
	var ranked []rankedResult
	for _, r := range results {
		if r.Success {
			ranked = append(ranked, rankedResult{result: r, score: r.Score})
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	return ranked
}

func countVotes(results []*Result) []votingResult {
	votes := make(map[string]*votingResult)

	for _, r := range results {
		if !r.Success {
			continue
		}

		val := fmt.Sprintf("%v", r.Output)
		if vr, ok := votes[val]; ok {
			vr.count++
		} else {
			votes[val] = &votingResult{value: val, count: 1}
		}
	}

	var results2 []votingResult
	for _, vr := range votes {
		results2 = append(results2, *vr)
	}

	sort.Slice(results2, func(i, j int) bool {
		return results2[i].count > results2[j].count
	})

	return results2
}

func aggregateMajority(results []*Result, threshold float64) *AggregatedResult {
	votes := countVotes(results)
	total := len(results)
	successCount := 0

	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	var output interface{}
	if len(votes) > 0 {
		topVote := votes[0]
		percentage := float64(topVote.count) / float64(total)
		if percentage >= threshold {
			output = topVote.value
		}
	}

	return &AggregatedResult{
		TotalAgents:  total,
		SuccessCount: successCount,
		FailureCount: total - successCount,
		Results:      results,
		Output:       output,
		Metadata: map[string]interface{}{
			"votes":       votes,
			"threshold":   threshold,
		},
	}
}

func aggregateAverage(results []*Result) *AggregatedResult {
	total := len(results)
	successCount := 0
	var sum float64
	var count float64

	for _, r := range results {
		if r.Success {
			successCount++
			if v, ok := r.Output.(float64); ok {
				sum += v
				count++
			} else if v, ok := r.Output.(int); ok {
				sum += float64(v)
				count++
			}
		}
	}

	var output interface{}
	if count > 0 {
		output = sum / count
	}

	return &AggregatedResult{
		TotalAgents:  total,
		SuccessCount: successCount,
		FailureCount: total - successCount,
		Results:      results,
		Output:       output,
		Metadata: map[string]interface{}{
			"sum":   sum,
			"count": count,
			"avg":   output,
		},
	}
}

func aggregateBestRanked(results []*Result) *AggregatedResult {
	ranked := rankResults(results)
	total := len(results)
	successCount := 0

	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	var output interface{}
	if len(ranked) > 0 {
		output = ranked[0].result.Output
	}

	return &AggregatedResult{
		TotalAgents:  total,
		SuccessCount: successCount,
		FailureCount: total - successCount,
		Results:      results,
		Output:       output,
		Metadata: map[string]interface{}{
			"top_score":  ranked[0].score,
			"top_agent":  ranked[0].result.AgentName,
		},
	}
}

func aggregateAllComplete(results []*Result) *AggregatedResult {
	total := len(results)
	successCount := 0

	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	return &AggregatedResult{
		TotalAgents:  total,
		SuccessCount: successCount,
		FailureCount: total - successCount,
		Results:      results,
		Output:       results,
	}
}

func aggregateFirstComplete(results []*Result) *AggregatedResult {
	total := len(results)
	successCount := 0
	var firstResult *Result

	for _, r := range results {
		if r.Success && firstResult == nil {
			firstResult = r
			successCount++
		} else if r.Success {
			successCount++
		}
	}

	return &AggregatedResult{
		TotalAgents:  total,
		SuccessCount: successCount,
		FailureCount: total - successCount,
		Results:      results,
		Output:       firstResult.Output,
		Metadata: map[string]interface{}{
			"first_agent": firstResult.AgentName,
		},
	}
}

type taskAdapter struct {
	handler TaskHandler
	agentID AgentID
}

func (t *taskAdapter) ID() AgentID        { return t.agentID }
func (t *taskAdapter) Name() string        { return t.handler.GetName() }
func (t *taskAdapter) Tags() []Tag         { return nil }
func (t *taskAdapter) Capabilities() []Capability {
	caps := t.handler.GetCapabilities()
	result := make([]Capability, len(caps))
	for i, c := range caps {
		result[i] = Capability(c)
	}
	return result
}
func (t *taskAdapter) Execute(ctx context.Context, task *Task) (*Result, error) {
	output, err := t.handler.HandleTask(ctx, task)
	if err != nil {
		return &Result{
			AgentID:      t.agentID,
			AgentName:    t.Name(),
			TaskID:       task.ID,
			Success:      false,
			Error:        err,
			ErrorMessage: err.Error(),
		}, nil
	}

	if output != nil && output.Success {
		return &Result{
			AgentID:   t.agentID,
			AgentName: t.Name(),
			TaskID:    task.ID,
			Success:   true,
			Output:    output.Output,
			Score:     output.Score,
		}, nil
	}

	return &Result{
		AgentID:      t.agentID,
		AgentName:    t.Name(),
		TaskID:       task.ID,
		Success:      false,
		ErrorMessage: "handler returned failure",
	}, nil
}
func (t *taskAdapter) HealthCheck(ctx context.Context) error {
	return nil
}

func AgentFromHandler(handler TaskHandler) Agent {
	return &taskAdapter{
		handler: handler,
		agentID: AgentID(uuid.New().String()),
	}
}
