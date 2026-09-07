package swarm

import (
	"context"
	"time"
)

type (
	AgentID       = string
	SwarmID       = string
	Tag           = string
	TaskID        = string
	ExecutionID   = string
	Capability    = string
)

type Agent interface {
	ID() AgentID
	Name() string
	Tags() []Tag
	Capabilities() []Capability
	Execute(ctx context.Context, task *Task) (*Result, error)
	HealthCheck(ctx context.Context) error
}

type TaskHandler interface {
	HandleTask(ctx context.Context, task *Task) (*Result, error)
	GetCapabilities() []Capability
	GetName() string
}

type SwarmConfig struct {
	ID                    SwarmID
	Name                  string
	Tags                  []Tag
	MinAgents             int
	MaxAgents             int
	TaskTimeout           time.Duration
	EnableResultTracking  bool
	MaxConcurrentTasks    int
	RetryPolicy           *RetryPolicy
	DiscoveryPolicy       DiscoveryPolicy
	AggregationStrategy   AggregationStrategy
}

func DefaultSwarmConfig() *SwarmConfig {
	return &SwarmConfig{
		MinAgents:            1,
		MaxAgents:            10,
		TaskTimeout:          30 * time.Second,
		EnableResultTracking: true,
		MaxConcurrentTasks:   5,
		RetryPolicy:          DefaultRetryPolicy(),
		DiscoveryPolicy:      DiscoveryAll,
		AggregationStrategy:  StrategyAllComplete,
	}
}

type RetryPolicy struct {
	MaxAttempts    int
	InitialDelay   time.Duration
	MaxDelay       time.Duration
	Multiplier     float64
	ShouldRetry    func(error) bool
}

func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  2,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		ShouldRetry: func(err error) bool {
			return err != nil
		},
	}
}

type DiscoveryPolicy string

const (
	DiscoveryAll     DiscoveryPolicy = "all"
	DiscoveryRandom  DiscoveryPolicy = "random"
	DiscoveryCapable DiscoveryPolicy = "capable"
	DiscoveryTagged  DiscoveryPolicy = "tagged"
)

type AggregationStrategy string

const (
	StrategyFirstComplete AggregationStrategy = "first_complete"
	StrategyAllComplete   AggregationStrategy = "all_complete"
	StrategyMajority      AggregationStrategy = "majority"
	StrategyBestRanked   AggregationStrategy = "best_ranked"
	StrategyAverage       AggregationStrategy = "average"
	StrategyVoting        AggregationStrategy = "voting"
	StrategyCustom        AggregationStrategy = "custom"
)

type Task struct {
	ID          TaskID
	Type        string
	Payload     interface{}
	RequiredTags []Tag
	RequiredCapabilities []Capability
	Priority    int
	Metadata    map[string]interface{}
	CreatedAt   time.Time
	Deadline    time.Time
}

type Result struct {
	AgentID      AgentID
	AgentName    string
	TaskID       TaskID
	ExecutionID  ExecutionID
	Success      bool
	Output       interface{}
	Error        error
	ErrorMessage string
	Score        float64
	Duration     time.Duration
	Metadata     map[string]interface{}
	CompletedAt  time.Time
}

type AggregatedResult struct {
	TaskID       TaskID
	Strategy     AggregationStrategy
	TotalAgents  int
	SuccessCount int
	FailureCount int
	Results      []*Result
	Output       interface{}
	Metadata     map[string]interface{}
}

type SwarmStats struct {
	TotalTasks       int64
	CompletedTasks   int64
	FailedTasks      int64
	AverageLatency   time.Duration
	SuccessRate      float64
	ActiveAgents     int
	TotalAgents      int
}

type HealthStatus struct {
	Status     string
	Components map[string]string
	Errors    []string
}

type Swarm interface {
	ID() SwarmID
	Name() string
	Tags() []Tag
	Stats() *SwarmStats
	Health(ctx context.Context) *HealthStatus

	RegisterAgent(ctx context.Context, agent Agent) error
	UnregisterAgent(ctx context.Context, agentID AgentID) error
	GetAgent(agentID AgentID) (Agent, bool)
	ListAgents() []Agent
	AgentCount() int

	Execute(ctx context.Context, task *Task) (*AggregatedResult, error)
	ExecuteWithStrategy(ctx context.Context, task *Task, strategy AggregationStrategy) (*AggregatedResult, error)

	FindByTag(tag Tag) []Agent
	FindByTags(tags ...Tag) []Agent
	FindByCapabilities(capabilities ...Capability) []Agent

	Shutdown(ctx context.Context) error
}

type SwarmRegistry interface {
	CreateSwarm(ctx context.Context, config *SwarmConfig, options ...SwarmOption) (Swarm, error)
	GetSwarm(swarmID SwarmID) (Swarm, bool)
	DeleteSwarm(ctx context.Context, swarmID SwarmID) error
	ListSwarms() []Swarm
	FindSwarmsByTag(tag Tag) []Swarm
	FindSwarmsByTags(tags ...Tag) []Swarm
	Shutdown(ctx context.Context) error
}

type ResultAggregator interface {
	Aggregate(results []*Result, strategy AggregationStrategy) (*AggregatedResult, error)
}

type TaskExecutor interface {
	Execute(ctx context.Context, task *Task, agents []Agent) (<-chan *Result, error)
	ExecuteWithTimeout(ctx context.Context, task *Task, agents []Agent, timeout time.Duration) (<-chan *Result, error)
}
