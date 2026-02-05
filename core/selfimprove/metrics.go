package selfimprove

import (
	"context"
	"sync"
	"time"
)

// MetricsCollector collects and exposes self-improvement metrics.
type MetricsCollector struct {
	mu sync.RWMutex

	// Per-agent metrics
	agentMetrics map[string]*AgentMetrics

	// Global metrics
	globalMetrics *GlobalMetrics

	// Hooks for external metrics systems
	hooks []MetricsHook
}

// AgentMetrics tracks metrics for a single agent.
type AgentMetrics struct {
	AgentID string `json:"agent_id"`

	// Execution counters
	TotalExecutions   int64 `json:"total_executions"`
	SuccessfulExecs   int64 `json:"successful_executions"`
	FailedExecs       int64 `json:"failed_executions"`

	// Score tracking
	TotalScore        float64 `json:"total_score"`
	MinScore          float64 `json:"min_score"`
	MaxScore          float64 `json:"max_score"`
	RecentScores      []float64 `json:"-"` // Not serialized

	// Latency tracking (in milliseconds)
	TotalLatencyMs    int64 `json:"total_latency_ms"`
	MinLatencyMs      int64 `json:"min_latency_ms"`
	MaxLatencyMs      int64 `json:"max_latency_ms"`

	// Token tracking
	TotalTokensUsed   int64 `json:"total_tokens_used"`

	// Learning metrics
	LearningCycles    int64 `json:"learning_cycles"`
	ProposalsCreated  int64 `json:"proposals_created"`
	ProposalsApproved int64 `json:"proposals_approved"`
	ProposalsRejected int64 `json:"proposals_rejected"`
	ProposalsApplied  int64 `json:"proposals_applied"`
	Rollbacks         int64 `json:"rollbacks"`

	// Experience metrics
	ExperiencesStored int64 `json:"experiences_stored"`

	// Time tracking
	FirstExecution    time.Time `json:"first_execution"`
	LastExecution     time.Time `json:"last_execution"`
	LastLearningCycle time.Time `json:"last_learning_cycle"`
}

// GlobalMetrics tracks metrics across all agents.
type GlobalMetrics struct {
	// Agent count
	TotalAgents       int64 `json:"total_agents"`
	ActiveAgents      int64 `json:"active_agents"`

	// Aggregate execution metrics
	TotalExecutions   int64 `json:"total_executions"`
	TotalSuccesses    int64 `json:"total_successes"`
	TotalFailures     int64 `json:"total_failures"`

	// Aggregate learning metrics
	TotalLearningCycles  int64 `json:"total_learning_cycles"`
	TotalProposals       int64 `json:"total_proposals"`
	TotalApplied         int64 `json:"total_applied"`
	TotalRollbacks       int64 `json:"total_rollbacks"`

	// System health
	LastActivity      time.Time `json:"last_activity"`
	UptimeSecs        int64     `json:"uptime_secs"`
	startTime         time.Time
}

// MetricsHook allows external systems to receive metrics updates.
type MetricsHook interface {
	OnExecutionRecorded(agentID string, success bool, score float64, latencyMs int64)
	OnLearningCycleCompleted(agentID string)
	OnProposalCreated(agentID string, strategy LearningStrategy)
	OnProposalApplied(agentID string, proposalID string)
	OnRollback(agentID string, proposalID string)
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		agentMetrics:  make(map[string]*AgentMetrics),
		globalMetrics: &GlobalMetrics{
			startTime: time.Now(),
		},
		hooks: make([]MetricsHook, 0),
	}
}

// AddHook adds a metrics hook.
func (c *MetricsCollector) AddHook(hook MetricsHook) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hooks = append(c.hooks, hook)
}

// RecordExecution records a task execution.
func (c *MetricsCollector) RecordExecution(agentID string, success bool, score float64, latencyMs int64, tokensUsed int) {
	c.mu.Lock()

	// Get or create agent metrics
	am := c.getOrCreateAgentMetrics(agentID)

	// Update counters
	am.TotalExecutions++
	if success {
		am.SuccessfulExecs++
		c.globalMetrics.TotalSuccesses++
	} else {
		am.FailedExecs++
		c.globalMetrics.TotalFailures++
	}

	// Update score tracking
	am.TotalScore += score
	if am.MinScore == 0 || score < am.MinScore {
		am.MinScore = score
	}
	if score > am.MaxScore {
		am.MaxScore = score
	}
	am.RecentScores = appendWithLimit(am.RecentScores, score, 100)

	// Update latency tracking
	am.TotalLatencyMs += latencyMs
	if am.MinLatencyMs == 0 || latencyMs < am.MinLatencyMs {
		am.MinLatencyMs = latencyMs
	}
	if latencyMs > am.MaxLatencyMs {
		am.MaxLatencyMs = latencyMs
	}

	// Update token tracking
	am.TotalTokensUsed += int64(tokensUsed)

	// Update time tracking
	now := time.Now()
	if am.FirstExecution.IsZero() {
		am.FirstExecution = now
	}
	am.LastExecution = now

	// Update global metrics
	c.globalMetrics.TotalExecutions++
	c.globalMetrics.LastActivity = now

	hooks := make([]MetricsHook, len(c.hooks))
	copy(hooks, c.hooks)
	c.mu.Unlock()

	// Notify hooks
	for _, hook := range hooks {
		hook.OnExecutionRecorded(agentID, success, score, latencyMs)
	}
}

// RecordLearningCycle records a learning cycle.
func (c *MetricsCollector) RecordLearningCycle(agentID string) {
	c.mu.Lock()

	am := c.getOrCreateAgentMetrics(agentID)
	am.LearningCycles++
	am.LastLearningCycle = time.Now()

	c.globalMetrics.TotalLearningCycles++
	c.globalMetrics.LastActivity = time.Now()

	hooks := make([]MetricsHook, len(c.hooks))
	copy(hooks, c.hooks)
	c.mu.Unlock()

	// Notify hooks
	for _, hook := range hooks {
		hook.OnLearningCycleCompleted(agentID)
	}
}

// RecordProposalCreated records a proposal creation.
func (c *MetricsCollector) RecordProposalCreated(agentID string, strategy LearningStrategy) {
	c.mu.Lock()

	am := c.getOrCreateAgentMetrics(agentID)
	am.ProposalsCreated++

	c.globalMetrics.TotalProposals++
	c.globalMetrics.LastActivity = time.Now()

	hooks := make([]MetricsHook, len(c.hooks))
	copy(hooks, c.hooks)
	c.mu.Unlock()

	// Notify hooks
	for _, hook := range hooks {
		hook.OnProposalCreated(agentID, strategy)
	}
}

// RecordProposalApproved records a proposal approval.
func (c *MetricsCollector) RecordProposalApproved(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	am := c.getOrCreateAgentMetrics(agentID)
	am.ProposalsApproved++

	c.globalMetrics.LastActivity = time.Now()
}

// RecordProposalRejected records a proposal rejection.
func (c *MetricsCollector) RecordProposalRejected(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	am := c.getOrCreateAgentMetrics(agentID)
	am.ProposalsRejected++

	c.globalMetrics.LastActivity = time.Now()
}

// RecordProposalApplied records a proposal application.
func (c *MetricsCollector) RecordProposalApplied(agentID string, proposalID string) {
	c.mu.Lock()

	am := c.getOrCreateAgentMetrics(agentID)
	am.ProposalsApplied++

	c.globalMetrics.TotalApplied++
	c.globalMetrics.LastActivity = time.Now()

	hooks := make([]MetricsHook, len(c.hooks))
	copy(hooks, c.hooks)
	c.mu.Unlock()

	// Notify hooks
	for _, hook := range hooks {
		hook.OnProposalApplied(agentID, proposalID)
	}
}

// RecordRollback records a rollback.
func (c *MetricsCollector) RecordRollback(agentID string, proposalID string) {
	c.mu.Lock()

	am := c.getOrCreateAgentMetrics(agentID)
	am.Rollbacks++

	c.globalMetrics.TotalRollbacks++
	c.globalMetrics.LastActivity = time.Now()

	hooks := make([]MetricsHook, len(c.hooks))
	copy(hooks, c.hooks)
	c.mu.Unlock()

	// Notify hooks
	for _, hook := range hooks {
		hook.OnRollback(agentID, proposalID)
	}
}

// RecordExperienceStored records an experience being stored.
func (c *MetricsCollector) RecordExperienceStored(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	am := c.getOrCreateAgentMetrics(agentID)
	am.ExperiencesStored++

	c.globalMetrics.LastActivity = time.Now()
}

// GetAgentMetrics returns metrics for a specific agent.
func (c *MetricsCollector) GetAgentMetrics(agentID string) *AgentMetricsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	am, ok := c.agentMetrics[agentID]
	if !ok {
		return nil
	}

	return c.createAgentSnapshot(am)
}

// GetAllAgentMetrics returns metrics for all agents.
func (c *MetricsCollector) GetAllAgentMetrics() map[string]*AgentMetricsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*AgentMetricsSnapshot)
	for agentID, am := range c.agentMetrics {
		result[agentID] = c.createAgentSnapshot(am)
	}
	return result
}

// GetGlobalMetrics returns global metrics.
func (c *MetricsCollector) GetGlobalMetrics() *GlobalMetricsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	gm := c.globalMetrics
	return &GlobalMetricsSnapshot{
		TotalAgents:          int64(len(c.agentMetrics)),
		ActiveAgents:         c.countActiveAgents(),
		TotalExecutions:      gm.TotalExecutions,
		TotalSuccesses:       gm.TotalSuccesses,
		TotalFailures:        gm.TotalFailures,
		SuccessRate:          safeDiv(float64(gm.TotalSuccesses), float64(gm.TotalExecutions)),
		TotalLearningCycles:  gm.TotalLearningCycles,
		TotalProposals:       gm.TotalProposals,
		TotalApplied:         gm.TotalApplied,
		TotalRollbacks:       gm.TotalRollbacks,
		ApprovalRate:         safeDiv(float64(gm.TotalApplied), float64(gm.TotalProposals)),
		RollbackRate:         safeDiv(float64(gm.TotalRollbacks), float64(gm.TotalApplied)),
		LastActivity:         gm.LastActivity,
		UptimeSecs:           int64(time.Since(gm.startTime).Seconds()),
	}
}

// AgentMetricsSnapshot is a read-only snapshot of agent metrics.
type AgentMetricsSnapshot struct {
	AgentID            string    `json:"agent_id"`
	TotalExecutions    int64     `json:"total_executions"`
	SuccessfulExecs    int64     `json:"successful_executions"`
	FailedExecs        int64     `json:"failed_executions"`
	SuccessRate        float64   `json:"success_rate"`
	AvgScore           float64   `json:"avg_score"`
	MinScore           float64   `json:"min_score"`
	MaxScore           float64   `json:"max_score"`
	RecentAvgScore     float64   `json:"recent_avg_score"`
	AvgLatencyMs       float64   `json:"avg_latency_ms"`
	MinLatencyMs       int64     `json:"min_latency_ms"`
	MaxLatencyMs       int64     `json:"max_latency_ms"`
	TotalTokensUsed    int64     `json:"total_tokens_used"`
	AvgTokensPerExec   float64   `json:"avg_tokens_per_execution"`
	LearningCycles     int64     `json:"learning_cycles"`
	ProposalsCreated   int64     `json:"proposals_created"`
	ProposalsApproved  int64     `json:"proposals_approved"`
	ProposalsRejected  int64     `json:"proposals_rejected"`
	ProposalsApplied   int64     `json:"proposals_applied"`
	Rollbacks          int64     `json:"rollbacks"`
	ExperiencesStored  int64     `json:"experiences_stored"`
	FirstExecution     time.Time `json:"first_execution"`
	LastExecution      time.Time `json:"last_execution"`
	LastLearningCycle  time.Time `json:"last_learning_cycle"`
	ScoreTrend         string    `json:"score_trend"` // "improving", "declining", "stable"
}

// GlobalMetricsSnapshot is a read-only snapshot of global metrics.
type GlobalMetricsSnapshot struct {
	TotalAgents         int64     `json:"total_agents"`
	ActiveAgents        int64     `json:"active_agents"`
	TotalExecutions     int64     `json:"total_executions"`
	TotalSuccesses      int64     `json:"total_successes"`
	TotalFailures       int64     `json:"total_failures"`
	SuccessRate         float64   `json:"success_rate"`
	TotalLearningCycles int64     `json:"total_learning_cycles"`
	TotalProposals      int64     `json:"total_proposals"`
	TotalApplied        int64     `json:"total_applied"`
	TotalRollbacks      int64     `json:"total_rollbacks"`
	ApprovalRate        float64   `json:"approval_rate"`
	RollbackRate        float64   `json:"rollback_rate"`
	LastActivity        time.Time `json:"last_activity"`
	UptimeSecs          int64     `json:"uptime_secs"`
}

// Helper methods

func (c *MetricsCollector) getOrCreateAgentMetrics(agentID string) *AgentMetrics {
	am, ok := c.agentMetrics[agentID]
	if !ok {
		am = &AgentMetrics{
			AgentID:      agentID,
			RecentScores: make([]float64, 0, 100),
		}
		c.agentMetrics[agentID] = am
		c.globalMetrics.TotalAgents++
	}
	return am
}

func (c *MetricsCollector) createAgentSnapshot(am *AgentMetrics) *AgentMetricsSnapshot {
	snapshot := &AgentMetricsSnapshot{
		AgentID:           am.AgentID,
		TotalExecutions:   am.TotalExecutions,
		SuccessfulExecs:   am.SuccessfulExecs,
		FailedExecs:       am.FailedExecs,
		SuccessRate:       safeDiv(float64(am.SuccessfulExecs), float64(am.TotalExecutions)),
		AvgScore:          safeDiv(am.TotalScore, float64(am.TotalExecutions)),
		MinScore:          am.MinScore,
		MaxScore:          am.MaxScore,
		RecentAvgScore:    calculateRecentAvg(am.RecentScores, 20),
		AvgLatencyMs:      safeDiv(float64(am.TotalLatencyMs), float64(am.TotalExecutions)),
		MinLatencyMs:      am.MinLatencyMs,
		MaxLatencyMs:      am.MaxLatencyMs,
		TotalTokensUsed:   am.TotalTokensUsed,
		AvgTokensPerExec:  safeDiv(float64(am.TotalTokensUsed), float64(am.TotalExecutions)),
		LearningCycles:    am.LearningCycles,
		ProposalsCreated:  am.ProposalsCreated,
		ProposalsApproved: am.ProposalsApproved,
		ProposalsRejected: am.ProposalsRejected,
		ProposalsApplied:  am.ProposalsApplied,
		Rollbacks:         am.Rollbacks,
		ExperiencesStored: am.ExperiencesStored,
		FirstExecution:    am.FirstExecution,
		LastExecution:     am.LastExecution,
		LastLearningCycle: am.LastLearningCycle,
		ScoreTrend:        calculateTrend(am.RecentScores),
	}
	return snapshot
}

func (c *MetricsCollector) countActiveAgents() int64 {
	cutoff := time.Now().Add(-5 * time.Minute)
	var count int64
	for _, am := range c.agentMetrics {
		if am.LastExecution.After(cutoff) {
			count++
		}
	}
	return count
}

func appendWithLimit(slice []float64, value float64, limit int) []float64 {
	slice = append(slice, value)
	if len(slice) > limit {
		return slice[len(slice)-limit:]
	}
	return slice
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func calculateRecentAvg(scores []float64, n int) float64 {
	if len(scores) == 0 {
		return 0
	}

	start := len(scores) - n
	if start < 0 {
		start = 0
	}

	var sum float64
	for i := start; i < len(scores); i++ {
		sum += scores[i]
	}
	return sum / float64(len(scores)-start)
}

func calculateTrend(scores []float64) string {
	if len(scores) < 10 {
		return "unknown"
	}

	// Compare first half to second half
	mid := len(scores) / 2
	var firstHalf, secondHalf float64

	for i := 0; i < mid; i++ {
		firstHalf += scores[i]
	}
	firstHalf /= float64(mid)

	for i := mid; i < len(scores); i++ {
		secondHalf += scores[i]
	}
	secondHalf /= float64(len(scores) - mid)

	diff := secondHalf - firstHalf
	if diff > 0.05 {
		return "improving"
	} else if diff < -0.05 {
		return "declining"
	}
	return "stable"
}

// PrometheusMetricsHook exports metrics in Prometheus format.
type PrometheusMetricsHook struct {
	// Counters and gauges would be defined here for Prometheus integration
	// This is a placeholder for actual Prometheus metrics
}

// NewPrometheusMetricsHook creates a new Prometheus metrics hook.
func NewPrometheusMetricsHook() *PrometheusMetricsHook {
	return &PrometheusMetricsHook{}
}

func (h *PrometheusMetricsHook) OnExecutionRecorded(agentID string, success bool, score float64, latencyMs int64) {
	// Would update Prometheus counters/histograms
}

func (h *PrometheusMetricsHook) OnLearningCycleCompleted(agentID string) {
	// Would increment Prometheus counter
}

func (h *PrometheusMetricsHook) OnProposalCreated(agentID string, strategy LearningStrategy) {
	// Would increment Prometheus counter
}

func (h *PrometheusMetricsHook) OnProposalApplied(agentID string, proposalID string) {
	// Would increment Prometheus counter
}

func (h *PrometheusMetricsHook) OnRollback(agentID string, proposalID string) {
	// Would increment Prometheus counter
}

// Global metrics collector instance
var globalMetrics *MetricsCollector
var globalMetricsOnce sync.Once

// GetGlobalMetricsCollector returns the global metrics collector.
func GetGlobalMetricsCollector() *MetricsCollector {
	globalMetricsOnce.Do(func() {
		globalMetrics = NewMetricsCollector()
	})
	return globalMetrics
}

// LearningMetricsHook implements LearningHook to collect metrics.
type LearningMetricsHook struct {
	collector *MetricsCollector
}

// NewLearningMetricsHook creates a new learning metrics hook.
func NewLearningMetricsHook(collector *MetricsCollector) *LearningMetricsHook {
	if collector == nil {
		collector = GetGlobalMetricsCollector()
	}
	return &LearningMetricsHook{collector: collector}
}

func (h *LearningMetricsHook) OnNewExperience(ctx context.Context, exp *Experience) {
	h.collector.RecordExperienceStored(exp.AgentID)
	h.collector.RecordExecution(exp.AgentID, exp.Success, exp.Score, exp.LatencyMs, exp.TokensUsed)
}

func (h *LearningMetricsHook) OnProposalCreated(ctx context.Context, proposal *ImprovementProposal) {
	h.collector.RecordProposalCreated(proposal.AgentID, proposal.Strategy)
}

func (h *LearningMetricsHook) OnProposalApplied(ctx context.Context, proposal *ImprovementProposal) {
	h.collector.RecordProposalApplied(proposal.AgentID, proposal.ID)
}

func (h *LearningMetricsHook) OnLearningTriggered(ctx context.Context, agentID string) {
	h.collector.RecordLearningCycle(agentID)
}
