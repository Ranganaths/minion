package multiagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yourusername/minion/observability"
)

// ScalingAction defines the type of scaling action
type ScalingAction string

const (
	ScaleUp   ScalingAction = "scale_up"
	ScaleDown ScalingAction = "scale_down"
	NoAction  ScalingAction = "no_action"
)

// ScalingDecision represents a scaling decision
type ScalingDecision struct {
	Action     ScalingAction
	Count      int
	Capability string
	Reason     string
	Timestamp  time.Time
}

// ScalingPolicy defines rules for auto-scaling
type ScalingPolicy struct {
	// Scale up triggers
	MaxQueueDepth         int     // Scale up if queue > this
	MaxUtilization        float64 // Scale up if utilization > this %
	MinIdleWorkers        int     // Maintain minimum idle workers
	ScaleUpThreshold      int     // Number of consecutive evaluations before scaling up

	// Scale down triggers
	MinQueueDepth         int     // Scale down if queue < this
	MinUtilization        float64 // Scale down if utilization < this %
	MaxIdleTime           time.Duration
	ScaleDownThreshold    int     // Number of consecutive evaluations before scaling down

	// Limits
	MinWorkers            int
	MaxWorkers            int
	ScaleUpStep           int     // Add N workers at a time
	ScaleDownStep         int     // Remove N workers at a time

	// Cooldown periods
	ScaleUpCooldown       time.Duration
	ScaleDownCooldown     time.Duration

	// Worker lifecycle
	WorkerStartupTimeout  time.Duration
	WorkerShutdownTimeout time.Duration

	// Evaluation
	EvaluationInterval    time.Duration
}

// DefaultScalingPolicy returns default scaling policy
func DefaultScalingPolicy() *ScalingPolicy {
	return &ScalingPolicy{
		// Scale up triggers
		MaxQueueDepth:         50,
		MaxUtilization:        0.80, // 80%
		MinIdleWorkers:        2,
		ScaleUpThreshold:      3, // 3 consecutive evaluations

		// Scale down triggers
		MinQueueDepth:         10,
		MinUtilization:        0.30, // 30%
		MaxIdleTime:           5 * time.Minute,
		ScaleDownThreshold:    5, // 5 consecutive evaluations

		// Limits
		MinWorkers:            2,
		MaxWorkers:            20,
		ScaleUpStep:           2,
		ScaleDownStep:         1,

		// Cooldown
		ScaleUpCooldown:       2 * time.Minute,
		ScaleDownCooldown:     5 * time.Minute,

		// Lifecycle
		WorkerStartupTimeout:  30 * time.Second,
		WorkerShutdownTimeout: 30 * time.Second,

		// Evaluation
		EvaluationInterval:    30 * time.Second,
	}
}

// WorkerStats contains statistics about workers
type WorkerStats struct {
	TotalWorkers   int
	BusyWorkers    int
	IdleWorkers    int
	PendingTasks   int
	Utilization    float64
	Capabilities   map[string]int // capability -> worker count
}

// Autoscaler manages automatic worker scaling
type Autoscaler struct {
	policy             *ScalingPolicy
	pool               *WorkerPool
	metrics            *observability.MetricsCollector
	logger             *observability.Logger

	// State tracking
	mu                 sync.RWMutex
	lastScaleUp        time.Time
	lastScaleDown      time.Time
	scaleUpCounter     int
	scaleDownCounter   int
	lastDecision       *ScalingDecision

	// Control
	ctx                context.Context
	cancel             context.CancelFunc
	running            bool
}

// NewAutoscaler creates a new autoscaler
func NewAutoscaler(policy *ScalingPolicy, pool *WorkerPool) *Autoscaler {
	if policy == nil {
		policy = DefaultScalingPolicy()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Autoscaler{
		policy:  policy,
		pool:    pool,
		metrics: observability.GetMetrics(),
		logger:  observability.GetLogger(),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts the autoscaler
func (a *Autoscaler) Start() error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("autoscaler already running")
	}
	a.running = true
	a.mu.Unlock()

	a.logger.Info("Starting autoscaler",
		observability.String("policy", fmt.Sprintf("%+v", a.policy)),
	)

	go a.evaluationLoop()

	return nil
}

// Stop stops the autoscaler
func (a *Autoscaler) Stop() error {
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return fmt.Errorf("autoscaler not running")
	}
	a.running = false
	a.mu.Unlock()

	a.logger.Info("Stopping autoscaler")
	a.cancel()

	return nil
}

// evaluationLoop continuously evaluates scaling needs
func (a *Autoscaler) evaluationLoop() {
	ticker := time.NewTicker(a.policy.EvaluationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.evaluate()
		case <-a.ctx.Done():
			return
		}
	}
}

// evaluate evaluates current state and makes scaling decisions
func (a *Autoscaler) evaluate() {
	stats := a.pool.GetStats()

	decision := a.evaluateScaling(stats)

	if decision.Action != NoAction {
		a.logger.Info("Scaling decision",
			observability.String("action", string(decision.Action)),
			observability.Int("count", decision.Count),
			observability.String("reason", decision.Reason),
		)

		a.executeScaling(decision)
	}

	// Record metrics
	a.recordMetrics(stats, decision)
}

// evaluateScaling determines if scaling is needed
func (a *Autoscaler) evaluateScaling(stats *WorkerStats) *ScalingDecision {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()

	// Check scale up conditions
	shouldScaleUp := false
	scaleUpReason := ""

	if stats.PendingTasks > a.policy.MaxQueueDepth {
		shouldScaleUp = true
		scaleUpReason = fmt.Sprintf("queue depth %d > %d", stats.PendingTasks, a.policy.MaxQueueDepth)
	} else if stats.Utilization > a.policy.MaxUtilization {
		shouldScaleUp = true
		scaleUpReason = fmt.Sprintf("utilization %.2f%% > %.2f%%", stats.Utilization*100, a.policy.MaxUtilization*100)
	} else if stats.IdleWorkers < a.policy.MinIdleWorkers {
		shouldScaleUp = true
		scaleUpReason = fmt.Sprintf("idle workers %d < %d", stats.IdleWorkers, a.policy.MinIdleWorkers)
	}

	// Check scale down conditions
	shouldScaleDown := false
	scaleDownReason := ""

	if stats.PendingTasks < a.policy.MinQueueDepth && stats.Utilization < a.policy.MinUtilization {
		shouldScaleDown = true
		scaleDownReason = fmt.Sprintf("queue depth %d < %d and utilization %.2f%% < %.2f%%",
			stats.PendingTasks, a.policy.MinQueueDepth, stats.Utilization*100, a.policy.MinUtilization*100)
	}

	// Check cooldown periods
	if shouldScaleUp && now.Sub(a.lastScaleUp) < a.policy.ScaleUpCooldown {
		return &ScalingDecision{Action: NoAction, Reason: "scale up cooldown active", Timestamp: now}
	}

	if shouldScaleDown && now.Sub(a.lastScaleDown) < a.policy.ScaleDownCooldown {
		return &ScalingDecision{Action: NoAction, Reason: "scale down cooldown active", Timestamp: now}
	}

	// Check worker limits
	if shouldScaleUp && stats.TotalWorkers >= a.policy.MaxWorkers {
		return &ScalingDecision{Action: NoAction, Reason: "max workers reached", Timestamp: now}
	}

	if shouldScaleDown && stats.TotalWorkers <= a.policy.MinWorkers {
		return &ScalingDecision{Action: NoAction, Reason: "min workers reached", Timestamp: now}
	}

	// Increment counters for threshold-based scaling
	if shouldScaleUp {
		a.scaleUpCounter++
		a.scaleDownCounter = 0

		if a.scaleUpCounter >= a.policy.ScaleUpThreshold {
			count := a.policy.ScaleUpStep
			if stats.TotalWorkers+count > a.policy.MaxWorkers {
				count = a.policy.MaxWorkers - stats.TotalWorkers
			}

			a.scaleUpCounter = 0
			a.lastScaleUp = now

			return &ScalingDecision{
				Action:    ScaleUp,
				Count:     count,
				Reason:    scaleUpReason,
				Timestamp: now,
			}
		}
	} else if shouldScaleDown {
		a.scaleDownCounter++
		a.scaleUpCounter = 0

		if a.scaleDownCounter >= a.policy.ScaleDownThreshold {
			count := a.policy.ScaleDownStep
			if stats.TotalWorkers-count < a.policy.MinWorkers {
				count = stats.TotalWorkers - a.policy.MinWorkers
			}

			a.scaleDownCounter = 0
			a.lastScaleDown = now

			return &ScalingDecision{
				Action:    ScaleDown,
				Count:     count,
				Reason:    scaleDownReason,
				Timestamp: now,
			}
		}
	} else {
		// Reset counters if conditions not met
		a.scaleUpCounter = 0
		a.scaleDownCounter = 0
	}

	return &ScalingDecision{Action: NoAction, Timestamp: now}
}

// executeScaling executes a scaling decision
func (a *Autoscaler) executeScaling(decision *ScalingDecision) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	var err error

	switch decision.Action {
	case ScaleUp:
		err = a.pool.ScaleUp(ctx, decision.Count, decision.Capability)
		if err != nil {
			a.logger.Error("Failed to scale up",
				observability.String("error", err.Error()),
				observability.Int("count", decision.Count),
			)
		} else {
			a.logger.Info("Scaled up workers",
				observability.Int("count", decision.Count),
				observability.String("reason", decision.Reason),
			)
		}

	case ScaleDown:
		err = a.pool.ScaleDown(ctx, decision.Count, decision.Capability)
		if err != nil {
			a.logger.Error("Failed to scale down",
				observability.String("error", err.Error()),
				observability.Int("count", decision.Count),
			)
		} else {
			a.logger.Info("Scaled down workers",
				observability.Int("count", decision.Count),
				observability.String("reason", decision.Reason),
			)
		}
	}

	a.mu.Lock()
	a.lastDecision = decision
	a.mu.Unlock()
}

// recordMetrics records autoscaler metrics
func (a *Autoscaler) recordMetrics(stats *WorkerStats, decision *ScalingDecision) {
	// Record worker metrics
	if a.metrics != nil {
		// These would integrate with existing metrics collector
		// For now, just log the stats
		a.logger.Debug("Worker stats",
			observability.Int("total_workers", stats.TotalWorkers),
			observability.Int("busy_workers", stats.BusyWorkers),
			observability.Int("idle_workers", stats.IdleWorkers),
			observability.Int("pending_tasks", stats.PendingTasks),
			observability.Float64("utilization", stats.Utilization),
		)
	}
}

// GetLastDecision returns the last scaling decision
func (a *Autoscaler) GetLastDecision() *ScalingDecision {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastDecision
}

// GetPolicy returns the current scaling policy
func (a *Autoscaler) GetPolicy() *ScalingPolicy {
	return a.policy
}

// UpdatePolicy updates the scaling policy
func (a *Autoscaler) UpdatePolicy(policy *ScalingPolicy) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.policy = policy
	a.logger.Info("Updated scaling policy",
		observability.String("policy", fmt.Sprintf("%+v", policy)),
	)
}

// GetStats returns current autoscaler statistics
func (a *Autoscaler) GetStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"running":            a.running,
		"last_scale_up":      a.lastScaleUp,
		"last_scale_down":    a.lastScaleDown,
		"scale_up_counter":   a.scaleUpCounter,
		"scale_down_counter": a.scaleDownCounter,
		"last_decision":      a.lastDecision,
	}
}
