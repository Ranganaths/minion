# Agent Swarms

Multi-agent swarm orchestration for parallel task execution.

## Quick Start

```go
import "github.com/Ranganaths/minion/core/multiagent/swarm"

// Create a swarm
ctx := context.Background()
config := swarm.DefaultSwarmConfig()
config.Name = "analysis-swarm"
config.Tags = []swarm.Tag{"analysis", "parallel"}

s, err := swarm.NewSwarm(ctx, config)

// Register agents
for i := 0; i < 3; i++ {
    agent := swarm.AgentFromHandler(&myHandler{})
    s.RegisterAgent(ctx, agent)
}

// Execute task in parallel
result, err := s.Execute(ctx, &swarm.Task{
    Type:    "analysis",
    Payload: input,
})

// Results aggregated based on strategy
fmt.Println(result.Output)
```

## Aggregation Strategies

| Strategy | Description |
|----------|-------------|
| `StrategyFirstComplete` | Return first result |
| `StrategyAllComplete` | Wait for all, return all |
| `StrategyMajority` | Return most common answer |
| `StrategyBestRanked` | Return highest scored result |
| `StrategyAverage` | Numeric average |
| `StrategyVoting` | Return all votes |

## Tag-Based Discovery

Agents can be tagged and discovered:

```go
// Register with tags
s.RegisterAgent(ctx, taggedAgent)

// Find agents by tag
securityAgents := s.FindByTag("security")

// Find by multiple tags (must have ALL)
specialized := s.FindByTags("critical", "billing")
```

## Swarm Registry

Manage multiple swarms:

```go
registry := swarm.NewSwarmRegistry(nil)

// Create swarms
s1, _ := registry.CreateSwarm(ctx, &swarm.SwarmConfig{
    Name: "analytics",
    Tags: []swarm.Tag{"analytics"},
})

// Find by tag
analytics := registry.FindSwarmsByTags("analytics")
```

## Integration with Coordinator

The swarm package can be used alongside the existing multi-agent system:

```go
// Use SwarmPool for handler-based workers
pool := swarm.NewSwarmPool(nil, registry)

pool.RegisterHandler(ctx, "analyzer", myAnalyzerHandler, []swarm.Tag{"analysis"})

result, _ := pool.Execute(ctx, "analyzer", &swarm.Task{Type: "analyze"}, "")
```

## Configuration

```go
config := &swarm.SwarmConfig{
    Name:                  "my-swarm",
    Tags:                  []swarm.Tag{"production"},
    MinAgents:             2,
    MaxAgents:             10,
    TaskTimeout:           30 * time.Second,
    MaxConcurrentTasks:    5,
    DiscoveryPolicy:       swarm.DiscoveryAll,
    AggregationStrategy:   swarm.StrategyBestRanked,
    RetryPolicy: &swarm.RetryPolicy{
        MaxAttempts:  3,
        InitialDelay: 100 * time.Millisecond,
        MaxDelay:    5 * time.Second,
        Multiplier:   2.0,
    },
}
```

## API Reference

### Swarm Interface

```go
type Swarm interface {
    ID() SwarmID
    Name() string
    Tags() []Tag
    
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
    
    Stats() *SwarmStats
    Health(ctx context.Context) *HealthStatus
    Shutdown(ctx context.Context) error
}
```

### Agent Interface

```go
type Agent interface {
    ID() AgentID
    Name() string
    Tags() []Tag
    Capabilities() []Capability
    Execute(ctx context.Context, task *Task) (*Result, error)
    HealthCheck(ctx context.Context) error
}
```

### TaskHandler (for easy agent creation)

```go
type TaskHandler interface {
    HandleTask(ctx context.Context, task *Task) (*Result, error)
    GetCapabilities() []string
    GetName() string
}

// Convert to Agent
agent := AgentFromHandler(myHandler)
```
