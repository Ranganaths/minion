package swarm

import (
	"context"
	"sync"
	"time"
)

type taskExecutor struct {
	maxConcurrent int
	semaphore    chan struct{}
}

func NewTaskExecutor(maxConcurrent int) TaskExecutor {
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	return &taskExecutor{
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
	}
}

func (e *taskExecutor) Execute(ctx context.Context, task *Task, agents []Agent) (<-chan *Result, error) {
	return e.ExecuteWithTimeout(ctx, task, agents, 0)
}

func (e *taskExecutor) ExecuteWithTimeout(ctx context.Context, task *Task, agents []Agent, timeout time.Duration) (<-chan *Result, error) {
	resultCh := make(chan *Result, len(agents))
	doneCh := make(chan struct{})

	go func() {
		defer close(resultCh)
		defer close(doneCh)

		var wg sync.WaitGroup

		for _, agent := range agents {
			select {
			case e.semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}

			wg.Add(1)
			go func(a Agent) {
				defer wg.Done()
				defer func() { <-e.semaphore }()

				result := e.executeAgentTask(ctx, a, task, timeout)
				if result != nil {
					select {
					case resultCh <- result:
					case <-ctx.Done():
					}
				}
			}(agent)
		}

		wg.Wait()
	}()

	return resultCh, nil
}

func (e *taskExecutor) executeAgentTask(ctx context.Context, agent Agent, task *Task, timeout time.Duration) *Result {
	start := time.Now()

	execCtx := ctx
	var cancel context.CancelFunc

	if timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	result, err := agent.Execute(execCtx, task)

	duration := time.Since(start)

	if err != nil {
		return &Result{
			AgentID:      agent.ID(),
			AgentName:    agent.Name(),
			TaskID:       task.ID,
			Success:      false,
			Error:        err,
			ErrorMessage: err.Error(),
			Duration:     duration,
			CompletedAt:  time.Now(),
		}
	}

	if result != nil {
		result.AgentID = agent.ID()
		result.AgentName = agent.Name()
		result.TaskID = task.ID
		result.Duration = duration
		result.CompletedAt = time.Now()
		return result
	}

	return &Result{
		AgentID:      agent.ID(),
		AgentName:    agent.Name(),
		TaskID:       task.ID,
		Success:      false,
		ErrorMessage: "agent returned nil result",
		Duration:     duration,
		CompletedAt:  time.Now(),
	}
}

type resultAggregator struct{}

func NewResultAggregator() ResultAggregator {
	return &resultAggregator{}
}

func (a *resultAggregator) Aggregate(results []*Result, strategy AggregationStrategy) (*AggregatedResult, error) {
	if len(results) == 0 {
		return &AggregatedResult{
			Results: results,
		}, nil
	}

	switch strategy {
	case StrategyFirstComplete:
		return aggregateFirstComplete(results), nil

	case StrategyAllComplete:
		return aggregateAllComplete(results), nil

	case StrategyMajority:
		return aggregateMajorityResults(results), nil

	case StrategyBestRanked:
		return aggregateBestRanked(results), nil

	case StrategyAverage:
		return aggregateAverage(results), nil

	case StrategyVoting:
		return aggregateVoting(results), nil

	default:
		return aggregateAllComplete(results), nil
	}
}

func aggregateMajorityResults(results []*Result) *AggregatedResult {
	total := len(results)
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	votes := countVotes(results)
	var output interface{}
	var maxVotes int

	for _, v := range votes {
		if v.count > maxVotes {
			maxVotes = v.count
			output = v.value
		}
	}

	threshold := 0.5
	if len(results) > 0 {
		if float64(maxVotes)/float64(total) >= threshold {
			// Keep majority output
		} else {
			output = nil
		}
	}

	return &AggregatedResult{
		TotalAgents:  total,
		SuccessCount: successCount,
		FailureCount: total - successCount,
		Results:      results,
		Output:       output,
		Metadata: map[string]interface{}{
			"threshold": threshold,
			"max_votes": maxVotes,
			"vote_distribution": votes,
		},
	}
}

func aggregateVoting(results []*Result) *AggregatedResult {
	total := len(results)
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	votes := countVotes(results)

	return &AggregatedResult{
		TotalAgents:  total,
		SuccessCount: successCount,
		FailureCount: total - successCount,
		Results:      results,
		Output:       votes,
		Metadata: map[string]interface{}{
			"total_votes":     len(votes),
			"vote_distribution": votes,
		},
	}
}

type retryableExecutor struct {
	executor   TaskExecutor
	policy    *RetryPolicy
}

func NewRetryableExecutor(executor TaskExecutor, policy *RetryPolicy) TaskExecutor {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}
	return &retryableExecutor{
		executor: executor,
		policy:   policy,
	}
}

func (e *retryableExecutor) Execute(ctx context.Context, task *Task, agents []Agent) (<-chan *Result, error) {
	return e.executor.Execute(ctx, task, agents)
}

func (e *retryableExecutor) ExecuteWithTimeout(ctx context.Context, task *Task, agents []Agent, timeout time.Duration) (<-chan *Result, error) {
	resultCh, err := e.executor.ExecuteWithTimeout(ctx, task, agents, timeout)
	if err != nil {
		return nil, err
	}

	aggregatedCh := make(chan *Result)

	go func() {
		defer close(aggregatedCh)

		failedAgents := make(map[AgentID]Agent)
		for _, agent := range agents {
			failedAgents[agent.ID()] = agent
		}

		for result := range resultCh {
			if result.Success {
				aggregatedCh <- result
				delete(failedAgents, result.AgentID)
			}
		}

		if len(failedAgents) == 0 {
			return
		}

		e.retryFailedAgents(ctx, task, failedAgents, aggregatedCh, timeout)
	}()

	return aggregatedCh, nil
}

func (e *retryableExecutor) retryFailedAgents(ctx context.Context, task *Task, failedAgents map[AgentID]Agent, resultCh chan<- *Result, timeout time.Duration) {
	if e.policy.MaxAttempts <= 1 {
		return
	}

	delay := e.policy.InitialDelay
	agents := make([]Agent, 0, len(failedAgents))
	for _, agent := range failedAgents {
		agents = append(agents, agent)
	}

	for attempt := 2; attempt <= e.policy.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		retryCh, err := e.executor.ExecuteWithTimeout(ctx, task, agents, timeout)
		if err != nil {
			continue
		}

		for result := range retryCh {
			if result.Success {
				resultCh <- result
				delete(failedAgents, result.AgentID)
			}
		}

		if len(failedAgents) == 0 {
			return
		}

		delay = time.Duration(float64(delay) * e.policy.Multiplier)
		if delay > e.policy.MaxDelay {
			delay = e.policy.MaxDelay
		}
	}
}

func CreateSwarmWithRetry(ctx context.Context, config *SwarmConfig, options ...SwarmOption) (Swarm, error) {
	baseSwarm, err := NewSwarm(ctx, config, options...)
	if err != nil {
		return nil, err
	}

	s := baseSwarm.(*swarmImpl)
	s.executor = NewRetryableExecutor(s.executor, config.RetryPolicy)

	return s, nil
}
