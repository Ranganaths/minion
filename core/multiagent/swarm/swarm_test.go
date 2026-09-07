package swarm

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAgent struct {
	id           AgentID
	name         string
	tags         []Tag
	capabilities []Capability
	execDelay    time.Duration
	shouldFail   bool
	execCount    int32
	result       interface{}
}

func newMockAgent(id, name string, tags []Tag, capabilities []Capability) *mockAgent {
	return &mockAgent{
		id:           AgentID(id),
		name:         name,
		tags:         tags,
		capabilities: capabilities,
		result:       "mock result",
	}
}

func (a *mockAgent) ID() AgentID                   { return a.id }
func (a *mockAgent) Name() string                 { return a.name }
func (a *mockAgent) Tags() []Tag                  { return a.tags }
func (a *mockAgent) Capabilities() []Capability    { return a.capabilities }

func (a *mockAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
	atomic.AddInt32(&a.execCount, 1)

	if a.execDelay > 0 {
		select {
		case <-time.After(a.execDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if a.shouldFail {
		return &Result{
			AgentID:      a.id,
			AgentName:    a.name,
			TaskID:       task.ID,
			Success:      false,
			ErrorMessage: "mock failure",
		}, nil
	}

	return &Result{
		AgentID:   a.id,
		AgentName: a.name,
		TaskID:    task.ID,
		Success:   true,
		Output:    a.result,
		Score:     1.0,
	}, nil
}

func (a *mockAgent) HealthCheck(ctx context.Context) error {
	return nil
}

func (a *mockAgent) ExecutionCount() int {
	return int(atomic.LoadInt32(&a.execCount))
}

type mockTaskHandler struct {
	name        string
	capabilities []Capability
	execDelay   time.Duration
	shouldFail  bool
	execCount   int32
	result      interface{}
}

func (h *mockTaskHandler) GetName() string        { return h.name }
func (h *mockTaskHandler) GetCapabilities() []string {
	caps := make([]string, len(h.capabilities))
	for i, c := range h.capabilities {
		caps[i] = string(c)
	}
	return caps
}

func (h *mockTaskHandler) HandleTask(ctx context.Context, task *Task) (*Result, error) {
	atomic.AddInt32(&h.execCount, 1)

	if h.execDelay > 0 {
		select {
		case <-time.After(h.execDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if h.shouldFail {
		return &Result{
			Success:      false,
			ErrorMessage: "handler failure",
		}, nil
	}

	return &Result{
		Success: true,
		Output:  h.result,
	}, nil
}

func (h *mockTaskHandler) ExecutionCount() int {
	return int(atomic.LoadInt32(&h.execCount))
}

func TestSwarm_CreateAndRegister(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateSwarm", func(t *testing.T) {
		config := DefaultSwarmConfig()
		config.Name = "test-swarm"
		config.Tags = []Tag{"test", "unit"}

		swarm, err := NewSwarm(ctx, config)
		require.NoError(t, err)
		assert.NotEmpty(t, swarm.ID())
		assert.Equal(t, "test-swarm", swarm.Name())
		assert.Equal(t, []Tag{"test", "unit"}, swarm.Tags())
	})

	t.Run("RegisterAgent", func(t *testing.T) {
		swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())
		agent := newMockAgent("agent-1", "Agent One", []Tag{"test"}, nil)

		err := swarm.RegisterAgent(ctx, agent)
		require.NoError(t, err)
		assert.Equal(t, 1, swarm.AgentCount())

		retrieved, ok := swarm.GetAgent("agent-1")
		assert.True(t, ok)
		assert.Equal(t, "Agent One", retrieved.Name())
	})

	t.Run("RegisterDuplicateAgent", func(t *testing.T) {
		swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())
		agent := newMockAgent("agent-dup", "Duplicate", nil, nil)

		err := swarm.RegisterAgent(ctx, agent)
		require.NoError(t, err)

		err = swarm.RegisterAgent(ctx, agent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("UnregisterAgent", func(t *testing.T) {
		swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())
		agent := newMockAgent("agent-remove", "Remove Me", nil, nil)

		swarm.RegisterAgent(ctx, agent)
		err := swarm.UnregisterAgent(ctx, "agent-remove")
		require.NoError(t, err)
		assert.Equal(t, 0, swarm.AgentCount())
	})

	t.Run("MaxAgentsLimit", func(t *testing.T) {
		config := DefaultSwarmConfig()
		config.MaxAgents = 2

		swarm, _ := NewSwarm(ctx, config)

		agent1 := newMockAgent("a1", "Agent 1", nil, nil)
		agent2 := newMockAgent("a2", "Agent 2", nil, nil)
		agent3 := newMockAgent("a3", "Agent 3", nil, nil)

		require.NoError(t, swarm.RegisterAgent(ctx, agent1))
		require.NoError(t, swarm.RegisterAgent(ctx, agent2))

		err := swarm.RegisterAgent(ctx, agent3)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "maximum capacity")
	})
}

func TestSwarm_Discovery(t *testing.T) {
	ctx := context.Background()
	swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())

	agents := []*mockAgent{
		newMockAgent("a1", "Alpha", []Tag{"security", "critical"}, nil),
		newMockAgent("a2", "Beta", []Tag{"analytics", "reporting"}, nil),
		newMockAgent("a3", "Gamma", []Tag{"security", "billing"}, nil),
		newMockAgent("a4", "Delta", []Tag{"analytics"}, nil),
	}

	for _, a := range agents {
		swarm.RegisterAgent(ctx, a)
	}

	t.Run("FindByTag", func(t *testing.T) {
		results := swarm.FindByTag("security")
		assert.Len(t, results, 2)
	})

	t.Run("FindByMultipleTags", func(t *testing.T) {
		results := swarm.FindByTags("security", "critical")
		assert.Len(t, results, 1)
		assert.Equal(t, "Alpha", results[0].Name())
	})

	t.Run("FindByCapabilities", func(t *testing.T) {
		agent := newMockAgent("cap-agent", "Cap Agent", nil, []Capability{"compute", "storage"})
		swarm.RegisterAgent(ctx, agent)

		results := swarm.FindByCapabilities("compute")
		assert.GreaterOrEqual(t, len(results), 1)
	})
}

func TestSwarm_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("ExecuteWithAllComplete", func(t *testing.T) {
		swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())

		for i := 0; i < 3; i++ {
			agent := newMockAgent("agent-"+string(rune('A'+i)), "Agent "+string(rune('A'+i)), nil, nil)
			swarm.RegisterAgent(ctx, agent)
		}

		task := &Task{
			Type:    "test",
			Payload: "test data",
		}

		result, err := swarm.Execute(ctx, task)
		require.NoError(t, err)
		assert.Equal(t, 3, result.TotalAgents)
		assert.Equal(t, 3, result.SuccessCount)
		assert.NotNil(t, result.Output)
	})

	t.Run("ExecuteWithFirstComplete", func(t *testing.T) {
		swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())

		agent1 := newMockAgent("fast", "Fast Agent", nil, nil)
		agent1.execDelay = 1 * time.Millisecond

		agent2 := newMockAgent("slow", "Slow Agent", nil, nil)
		agent2.execDelay = 100 * time.Millisecond

		swarm.RegisterAgent(ctx, agent1)
		swarm.RegisterAgent(ctx, agent2)

		task := &Task{Type: "test"}

		result, err := swarm.ExecuteWithStrategy(ctx, task, StrategyFirstComplete)
		require.NoError(t, err)
		assert.Equal(t, 2, result.TotalAgents)
	})

	t.Run("ExecuteWithPartialFailure", func(t *testing.T) {
		swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())

		successAgent := newMockAgent("success", "Success", nil, nil)
		failAgent := newMockAgent("fail", "Fail", nil, nil)
		failAgent.shouldFail = true

		swarm.RegisterAgent(ctx, successAgent)
		swarm.RegisterAgent(ctx, failAgent)

		task := &Task{Type: "test"}

		result, err := swarm.Execute(ctx, task)
		require.NoError(t, err)
		assert.Equal(t, 2, result.TotalAgents)
		assert.Equal(t, 1, result.SuccessCount)
		assert.Equal(t, 1, result.FailureCount)
	})

	t.Run("ExecuteWithNoEligibleAgents", func(t *testing.T) {
		swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())
		swarm.RegisterAgent(ctx, newMockAgent("a", "A", []Tag{"tag1"}, nil))

		task := &Task{
			Type:          "test",
			RequiredTags:  []Tag{"nonexistent"},
		}

		_, err := swarm.Execute(ctx, task)
		assert.Error(t, err)
	})
}

func TestSwarm_AggregationStrategies(t *testing.T) {
	ctx := context.Background()

	t.Run("AllComplete", func(t *testing.T) {
		swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())
		swarm.RegisterAgent(ctx, newMockAgent("a1", "A1", nil, nil))
		swarm.RegisterAgent(ctx, newMockAgent("a2", "A2", nil, nil))

		result, err := swarm.ExecuteWithStrategy(ctx, &Task{Type: "test"}, StrategyAllComplete)
		require.NoError(t, err)
		assert.Equal(t, 2, result.SuccessCount)
		assert.Len(t, result.Output.([]*Result), 2)
	})

	t.Run("BestRanked", func(t *testing.T) {
		swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())

		agent1 := newMockAgent("a1", "A1", nil, nil)
		agent1.result = "low quality"
		agent1.shouldFail = true

		agent2 := newMockAgent("a2", "A2", nil, nil)
		agent2.result = "high quality"
		agent2.execDelay = 1 * time.Millisecond

		swarm.RegisterAgent(ctx, agent1)
		swarm.RegisterAgent(ctx, agent2)

		result, err := swarm.ExecuteWithStrategy(ctx, &Task{Type: "test"}, StrategyBestRanked)
		require.NoError(t, err)
		assert.Equal(t, 1, result.SuccessCount)
		assert.Equal(t, "high quality", result.Output)
	})
}

func TestSwarm_Stats(t *testing.T) {
	ctx := context.Background()
	swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())

	swarm.RegisterAgent(ctx, newMockAgent("a1", "A1", nil, nil))
	swarm.RegisterAgent(ctx, newMockAgent("a2", "A2", nil, nil))

	for i := 0; i < 3; i++ {
		_, _ = swarm.Execute(ctx, &Task{Type: "test"})
	}

	stats := swarm.Stats()
	assert.GreaterOrEqual(t, stats.TotalTasks, int64(3))
	assert.GreaterOrEqual(t, stats.CompletedTasks, int64(3))
	assert.Equal(t, 2, stats.TotalAgents)
}

func TestSwarm_Health(t *testing.T) {
	ctx := context.Background()
	swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())

	swarm.RegisterAgent(ctx, newMockAgent("a1", "A1", nil, nil))
	swarm.RegisterAgent(ctx, newMockAgent("a2", "A2", nil, nil))

	health := swarm.Health(ctx)
	assert.Equal(t, "healthy", health.Status)
}

func TestSwarm_Shutdown(t *testing.T) {
	ctx := context.Background()
	swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())

	swarm.RegisterAgent(ctx, newMockAgent("a1", "A1", nil, nil))

	err := swarm.Shutdown(ctx)
	require.NoError(t, err)

	health := swarm.Health(ctx)
	assert.Equal(t, "shutdown", health.Status)

	err = swarm.RegisterAgent(ctx, newMockAgent("a2", "A2", nil, nil))
	assert.Error(t, err)
}

func TestSwarmRegistry(t *testing.T) {
	ctx := context.Background()
	registry := NewSwarmRegistry(nil)

	t.Run("CreateAndGetSwarm", func(t *testing.T) {
		config := DefaultSwarmConfig()
		config.Name = "registry-swarm"

		swarm, err := registry.CreateSwarm(ctx, config)
		require.NoError(t, err)

		retrieved, ok := registry.GetSwarm(swarm.ID())
		assert.True(t, ok)
		assert.Equal(t, "registry-swarm", retrieved.Name())
	})

	t.Run("DeleteSwarm", func(t *testing.T) {
		swarm, _ := registry.CreateSwarm(ctx, DefaultSwarmConfig())
		swarmID := swarm.ID()

		err := registry.DeleteSwarm(ctx, swarmID)
		require.NoError(t, err)

		_, ok := registry.GetSwarm(swarmID)
		assert.False(t, ok)
	})

	t.Run("FindSwarmsByTags", func(t *testing.T) {
		s1, _ := registry.CreateSwarm(ctx, &SwarmConfig{Name: "s1", Tags: []Tag{"tag-a"}})
		s2, _ := registry.CreateSwarm(ctx, &SwarmConfig{Name: "s2", Tags: []Tag{"tag-a", "tag-b"}})
		_, _ = registry.CreateSwarm(ctx, &SwarmConfig{Name: "s3", Tags: []Tag{"tag-c"}})

		_ = s1.RegisterAgent(ctx, newMockAgent("a1", "A1", nil, nil))
		_ = s2.RegisterAgent(ctx, newMockAgent("a2", "A2", nil, nil))

		swarms := registry.FindSwarmsByTags("tag-a")
		assert.Len(t, swarms, 2)
	})

	t.Run("ListSwarms", func(t *testing.T) {
		before := len(registry.ListSwarms())
		_, _ = registry.CreateSwarm(ctx, DefaultSwarmConfig())

		assert.Equal(t, before+1, len(registry.ListSwarms()))
	})
}

func TestSwarmManager(t *testing.T) {
	ctx := context.Background()
	registry := NewSwarmRegistry(nil)
	manager := NewSwarmManager(registry)

	t.Run("CreateAndGetOrCreate", func(t *testing.T) {
		s1, err := manager.CreateSwarm(ctx, "my-swarm", []Tag{"test"})
		require.NoError(t, err)

		s2, err := manager.GetOrCreateSwarm(ctx, "my-swarm", nil)
		require.NoError(t, err)
		assert.Equal(t, s1.ID(), s2.ID())
	})

	t.Run("JoinAndLeaveSwarm", func(t *testing.T) {
		swarm, _ := manager.CreateSwarm(ctx, "join-test", nil)
		agent := newMockAgent("join-agent", "Join Agent", nil, nil)

		err := manager.JoinSwarm(ctx, swarm.ID(), agent)
		require.NoError(t, err)

		err = manager.LeaveSwarm(ctx, swarm.ID(), agent.ID())
		require.NoError(t, err)
	})

	t.Run("FindSwarms", func(t *testing.T) {
		_, _ = manager.CreateSwarm(ctx, "find-1", []Tag{"findable"})
		_, _ = manager.CreateSwarm(ctx, "find-2", []Tag{"findable", "other"})

		swarms := manager.FindSwarms(ctx, "findable")
		assert.Len(t, swarms, 2)
	})
}

func TestAgentFromHandler(t *testing.T) {
	ctx := context.Background()

	handler := &mockTaskHandler{
		name:         "test-handler",
		capabilities: []Capability{"cap1", "cap2"},
		result:       "handler result",
	}

	agent := AgentFromHandler(handler)
	assert.Equal(t, "test-handler", agent.Name())
	assert.Len(t, agent.Capabilities(), 2)

	result, err := agent.Execute(ctx, &Task{ID: "test-task", Type: "test"})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "handler result", result.Output)
}

func TestTaskExecutor(t *testing.T) {
	ctx := context.Background()
	executor := NewTaskExecutor(3)

	t.Run("ParallelExecution", func(t *testing.T) {
		agent1 := newMockAgent("p1", "Parallel 1", nil, nil)
		agent1.execDelay = 5 * time.Millisecond

		agent2 := newMockAgent("p2", "Parallel 2", nil, nil)
		agent2.execDelay = 5 * time.Millisecond

		task := &Task{Type: "parallel"}

		resultCh, err := executor.Execute(ctx, task, []Agent{agent1, agent2})
		require.NoError(t, err)

		var results []*Result
		for r := range resultCh {
			results = append(results, r)
		}

		assert.Len(t, results, 2)
	})

	t.Run("SequentialFallback", func(t *testing.T) {
		agent := newMockAgent("seq", "Sequential", nil, nil)

		task := &Task{Type: "seq"}

		resultCh, err := executor.Execute(ctx, task, []Agent{agent})
		require.NoError(t, err)

		var results []*Result
		for r := range resultCh {
			results = append(results, r)
		}

		assert.Len(t, results, 1)
		assert.True(t, results[0].Success)
	})

	t.Run("Timeout", func(t *testing.T) {
		slowAgent := newMockAgent("slow", "Slow", nil, nil)
		slowAgent.execDelay = 500 * time.Millisecond

		task := &Task{Type: "timeout"}

		resultCh, err := executor.ExecuteWithTimeout(ctx, task, []Agent{slowAgent}, 10*time.Millisecond)
		require.NoError(t, err)

		var results []*Result
		for r := range resultCh {
			results = append(results, r)
		}

		assert.Len(t, results, 1)
		assert.False(t, results[0].Success)
	})
}

func TestResultAggregator(t *testing.T) {
	aggregator := NewResultAggregator()

	t.Run("EmptyResults", func(t *testing.T) {
		result, err := aggregator.Aggregate([]*Result{}, StrategyAllComplete)
		require.NoError(t, err)
		assert.Equal(t, 0, result.TotalAgents)
	})

	t.Run("AllSuccess", func(t *testing.T) {
		results := []*Result{
			{Success: true, Output: "a"},
			{Success: true, Output: "a"},
			{Success: true, Output: "b"},
		}

		result, err := aggregator.Aggregate(results, StrategyAllComplete)
		require.NoError(t, err)
		assert.Equal(t, 3, result.SuccessCount)
		assert.Len(t, result.Output.([]*Result), 3)
	})

	t.Run("MajorityVote", func(t *testing.T) {
		results := []*Result{
			{Success: true, Output: "a"},
			{Success: true, Output: "a"},
			{Success: true, Output: "b"},
		}

		result, err := aggregator.Aggregate(results, StrategyMajority)
		require.NoError(t, err)
		assert.Equal(t, "a", result.Output)
	})

	t.Run("Average", func(t *testing.T) {
		results := []*Result{
			{Success: true, Output: float64(10)},
			{Success: true, Output: float64(20)},
			{Success: true, Output: float64(30)},
		}

		result, err := aggregator.Aggregate(results, StrategyAverage)
		require.NoError(t, err)
		assert.Equal(t, float64(20), result.Output)
	})
}

func TestSwarmPool(t *testing.T) {
	ctx := context.Background()
	registry := NewSwarmRegistry(nil)
	pool := NewSwarmPool(nil, registry)

	handler := &mockTaskHandler{
		name:    "pool-handler",
		result:  "pool result",
	}

	t.Run("RegisterHandler", func(t *testing.T) {
		err := pool.RegisterHandler(ctx, "compute", handler, []Tag{"compute"})
		require.NoError(t, err)

		swarm, ok := pool.GetSwarm("compute")
		assert.True(t, ok)
		assert.Equal(t, 1, swarm.AgentCount())
	})

	t.Run("Execute", func(t *testing.T) {
		result, err := pool.Execute(ctx, "compute", &Task{Type: "test"}, "")
		require.NoError(t, err)
		assert.Equal(t, 1, result.SuccessCount)
	})

	t.Run("ScaleUp", func(t *testing.T) {
		err := pool.ScaleUp(ctx, "compute", 2)
		require.NoError(t, err)

		swarm, _ := pool.GetSwarm("compute")
		assert.Equal(t, 3, swarm.AgentCount())
	})

	t.Run("ScaleDown", func(t *testing.T) {
		err := pool.ScaleDown(ctx, "compute", 1)
		require.NoError(t, err)

		swarm, _ := pool.GetSwarm("compute")
		assert.Equal(t, 2, swarm.AgentCount())
	})

	t.Run("GetStats", func(t *testing.T) {
		stats := pool.GetStats("compute")
		assert.NotNil(t, stats)
		assert.Equal(t, int64(1), stats.TasksProcessed)
	})
}

func TestSwarm_ContextCancellation(t *testing.T) {
	swarm, _ := NewSwarm(context.Background(), DefaultSwarmConfig())

	fastAgent := newMockAgent("fast", "Fast", nil, nil)
	swarm.RegisterAgent(context.Background(), fastAgent)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := swarm.Execute(ctx, &Task{Type: "test"})
	assert.NoError(t, err)
}

func TestSwarm_ConcurrentRegistration(t *testing.T) {
	ctx := context.Background()
	swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			agent := newMockAgent("concurrent-"+string(rune('0'+id)), "Concurrent", nil, nil)
			err := swarm.RegisterAgent(ctx, agent)
			done <- (err == nil)
		}(i)
	}

	successCount := 0
	for i := 0; i < 10; i++ {
		if <-done {
			successCount++
		}
	}

	assert.GreaterOrEqual(t, successCount, 1)
	assert.LessOrEqual(t, swarm.AgentCount(), 10)
}

func TestSwarm_ConcurrentExecution(t *testing.T) {
	ctx := context.Background()
	swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())

	swarm.RegisterAgent(ctx, newMockAgent("exec", "Exec", nil, nil))

	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_, err := swarm.Execute(ctx, &Task{Type: "concurrent"})
			done <- (err == nil)
		}()
	}

	for i := 0; i < 5; i++ {
		assert.True(t, <-done)
	}
}

func TestSwarm_DiscoveryPolicy(t *testing.T) {
	ctx := context.Background()

	t.Run("DiscoveryRandom", func(t *testing.T) {
		config := DefaultSwarmConfig()
		config.DiscoveryPolicy = DiscoveryRandom
		swarm, _ := NewSwarm(ctx, config)

		for i := 0; i < 5; i++ {
			swarm.RegisterAgent(ctx, newMockAgent("r"+string(rune('0'+i)), "R", nil, nil))
		}

		selected := make(map[AgentID]bool)
		for i := 0; i < 20; i++ {
			task := &Task{Type: "test"}
			ag := swarm.(*swarmImpl)
			agents := ag.selectAgents(task)
			for _, a := range agents {
				selected[a.ID()] = true
			}
		}

		assert.NotEmpty(t, selected)
	})
}

func TestSwarm_CustomOptions(t *testing.T) {
	ctx := context.Background()

	customLogger := &testLogger{}
	metrics := NewMetricsCollector()

	config := DefaultSwarmConfig()
	config.Name = "custom-options"

	swarm, err := NewSwarm(ctx, config,
		WithLogger(customLogger),
		WithMetrics(metrics),
	)
	require.NoError(t, err)

	swarm.RegisterAgent(ctx, newMockAgent("a", "A", nil, nil))
	swarm.Execute(ctx, &Task{Type: "test"})

	assert.Equal(t, int64(1), metrics.GetCounter("swarm.task.success"))
}

type testLogger struct {
	infoCalled bool
}

func (l *testLogger) Info(msg string, fields ...Field)   { l.infoCalled = true }
func (l *testLogger) Error(msg string, fields ...Field) {}
func (l *testLogger) Warn(msg string, fields ...Field)  {}
func (l *testLogger) Debug(msg string, fields ...Field) {}

func TestSwarm_RetryPolicy(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateWithRetry", func(t *testing.T) {
		config := DefaultSwarmConfig()
		config.RetryPolicy = &RetryPolicy{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
		}

		swarm, err := CreateSwarmWithRetry(ctx, config)
		require.NoError(t, err)
		assert.NotNil(t, swarm)
	})
}

func TestSwarm_Errors(t *testing.T) {
	assert.Error(t, ErrSwarmFull)
	assert.Error(t, ErrAgentAlreadyExists)
	assert.Error(t, ErrAgentNotFound)
	assert.Error(t, ErrNoEligibleAgents)
	assert.Error(t, ErrInvalidConfig)
}

func TestSwarm_TaskWithRequirements(t *testing.T) {
	ctx := context.Background()
	swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())

	agent1 := newMockAgent("sec", "Security", []Tag{"security"}, []Capability{"auth"})
	agent2 := newMockAgent("eng", "Engineer", []Tag{"engineering"}, []Capability{"code"})
	agent3 := newMockAgent("sec-eng", "Security Engineer", []Tag{"security", "engineering"}, []Capability{"auth", "code"})

	swarm.RegisterAgent(ctx, agent1)
	swarm.RegisterAgent(ctx, agent2)
	swarm.RegisterAgent(ctx, agent3)

	t.Run("RequiredTagsFilter", func(t *testing.T) {
		task := &Task{
			Type:         "secure",
			RequiredTags: []Tag{"security"},
		}

		ag := swarm.(*swarmImpl)
		agents := ag.selectAgents(task)
		assert.Len(t, agents, 2)
	})

	t.Run("RequiredCapabilitiesFilter", func(t *testing.T) {
		task := &Task{
			Type:                  "auth-code",
			RequiredCapabilities:   []Capability{"auth", "code"},
		}

		ag := swarm.(*swarmImpl)
		agents := ag.selectAgents(task)
		assert.Len(t, agents, 1)
		assert.Equal(t, "Security Engineer", agents[0].Name())
	})
}

func TestSwarm_Metadata(t *testing.T) {
	ctx := context.Background()
	swarm, _ := NewSwarm(ctx, DefaultSwarmConfig())

	agent := newMockAgent("meta", "Metadata", nil, nil)
	swarm.RegisterAgent(ctx, agent)

	task := &Task{
		Type:    "metadata-test",
		Payload: map[string]interface{}{"key": "value"},
		Metadata: map[string]interface{}{
			"source": "test",
		},
	}

	result, err := swarm.Execute(ctx, task)
	require.NoError(t, err)
	assert.Equal(t, 1, result.SuccessCount)
}
