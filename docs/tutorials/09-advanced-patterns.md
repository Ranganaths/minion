# Tutorial 9: Advanced Patterns

**Duration**: 2 hours
**Level**: Advanced
**Prerequisites**: Tutorials 1-8

## 🎯 Learning Objectives

By the end of this tutorial, you will:
- Implement retry strategies with exponential backoff
- Build request/response middleware
- Create composable tool chains
- Implement caching strategies
- Handle rate limiting across services
- Build event-driven architectures
- Implement the saga pattern for distributed transactions

## 📚 Advanced Patterns Overview

Production systems require sophisticated patterns:

| Pattern | Purpose | Use Case |
|---------|---------|----------|
| **Retry with Backoff** | Handle transient failures | Network glitches |
| **Middleware** | Cross-cutting concerns | Logging, auth, metrics |
| **Tool Chaining** | Compose complex workflows | Multi-step automation |
| **Adaptive Caching** | Intelligent cache invalidation | Dynamic data |
| **Rate Limiting** | Respect API quotas | External services |
| **Event-Driven** | Decouple components | Async workflows |
| **Saga Pattern** | Distributed transactions | Multi-service updates |

## 🛠️ Part 1: Retry Strategies

### Exponential Backoff

```go
package patterns

import (
	"context"
	"fmt"
	"math"
	"time"
)

type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetryableErrors []error
}

func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
	}
}

type Retrier struct {
	config *RetryConfig
}

func NewRetrier(config *RetryConfig) *Retrier {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &Retrier{config: config}
}

func (r *Retrier) Execute(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// Try operation
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.isRetryable(err) {
			return err
		}

		// Don't sleep on last attempt
		if attempt == r.config.MaxAttempts {
			break
		}

		// Calculate backoff delay
		delay := r.calculateBackoff(attempt)

		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", r.config.MaxAttempts, lastErr)
}

func (r *Retrier) calculateBackoff(attempt int) time.Duration {
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.BackoffFactor, float64(attempt-1))

	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}

	// Add jitter (±25%)
	jitter := delay * 0.25 * (2*rand.Float64() - 1)
	delay += jitter

	return time.Duration(delay)
}

func (r *Retrier) isRetryable(err error) bool {
	// Check if error is in retryable list
	for _, retryable := range r.config.RetryableErrors {
		if errors.Is(err, retryable) {
			return true
		}
	}

	// Default: retry on temporary errors
	if temp, ok := err.(interface{ Temporary() bool }); ok {
		return temp.Temporary()
	}

	return false
}
```

### Usage Example

```go
retrier := patterns.NewRetrier(patterns.DefaultRetryConfig())

err := retrier.Execute(ctx, func() error {
	_, err := manager.CallTool(ctx, "github", "create_issue", params)
	return err
})

if err != nil {
	log.Printf("Operation failed: %v", err)
}
```

## 🛠️ Part 2: Middleware Pattern

### Middleware Interface

```go
package patterns

import (
	"context"
	"time"

	"github.com/Ranganaths/minion/models"
)

type ToolExecutor interface {
	ExecuteTool(ctx context.Context, agentID string, input *models.ToolInput) (*models.ToolOutput, error)
}

type Middleware func(ToolExecutor) ToolExecutor

// Chain multiple middleware
func Chain(executor ToolExecutor, middleware ...Middleware) ToolExecutor {
	for i := len(middleware) - 1; i >= 0; i-- {
		executor = middleware[i](executor)
	}
	return executor
}
```

### Logging Middleware

```go
func LoggingMiddleware() Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return &loggingExecutor{next: next}
	}
}

type loggingExecutor struct {
	next ToolExecutor
}

func (e *loggingExecutor) ExecuteTool(ctx context.Context, agentID string, input *models.ToolInput) (*models.ToolOutput, error) {
	start := time.Now()

	log.Printf("→ Executing tool: %s (agent: %s)", input.ToolName, agentID)

	output, err := e.next.ExecuteTool(ctx, agentID, input)

	elapsed := time.Since(start)

	if err != nil {
		log.Printf("← Tool failed: %s (took: %v, error: %v)", input.ToolName, elapsed, err)
	} else if !output.Success {
		log.Printf("← Tool error: %s (took: %v, error: %s)", input.ToolName, elapsed, output.Error)
	} else {
		log.Printf("← Tool succeeded: %s (took: %v)", input.ToolName, elapsed)
	}

	return output, err
}
```

### Metrics Middleware

```go
func MetricsMiddleware(collector *MetricsCollector) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return &metricsExecutor{
			next:      next,
			collector: collector,
		}
	}
}

type metricsExecutor struct {
	next      ToolExecutor
	collector *MetricsCollector
}

func (e *metricsExecutor) ExecuteTool(ctx context.Context, agentID string, input *models.ToolInput) (*models.ToolOutput, error) {
	start := time.Now()

	output, err := e.next.ExecuteTool(ctx, agentID, input)

	elapsed := time.Since(start)

	// Record metrics
	e.collector.RecordToolExecution(input.ToolName, elapsed, err == nil && output.Success)

	return output, err
}
```

### Timeout Middleware

```go
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return &timeoutExecutor{
			next:    next,
			timeout: timeout,
		}
	}
}

type timeoutExecutor struct {
	next    ToolExecutor
	timeout time.Duration
}

func (e *timeoutExecutor) ExecuteTool(ctx context.Context, agentID string, input *models.ToolInput) (*models.ToolOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	return e.next.ExecuteTool(ctx, agentID, input)
}
```

### Usage Example

```go
// Wrap framework with middleware
executor := patterns.Chain(
	framework,
	patterns.LoggingMiddleware(),
	patterns.MetricsMiddleware(collector),
	patterns.TimeoutMiddleware(30*time.Second),
)

// Use wrapped executor
output, err := executor.ExecuteTool(ctx, agentID, input)
```

## 🛠️ Part 3: Tool Chaining

### Composable Workflow Builder

```go
package patterns

import (
	"context"
	"fmt"
)

type Step struct {
	Name      string
	ToolName  string
	Transform func(prev interface{}) map[string]interface{}
}

type Workflow struct {
	steps    []Step
	executor ToolExecutor
	agentID  string
}

func NewWorkflow(executor ToolExecutor, agentID string) *Workflow {
	return &Workflow{
		steps:    make([]Step, 0),
		executor: executor,
		agentID:  agentID,
	}
}

func (w *Workflow) AddStep(name, toolName string, transform func(prev interface{}) map[string]interface{}) *Workflow {
	w.steps = append(w.steps, Step{
		Name:      name,
		ToolName:  toolName,
		Transform: transform,
	})
	return w
}

func (w *Workflow) Execute(ctx context.Context, initialInput map[string]interface{}) (interface{}, error) {
	var result interface{} = initialInput

	for i, step := range w.steps {
		log.Printf("Step %d/%d: %s", i+1, len(w.steps), step.Name)

		// Transform previous result into params for this step
		params := step.Transform(result)

		// Execute step
		output, err := w.executor.ExecuteTool(ctx, w.agentID, &models.ToolInput{
			ToolName: step.ToolName,
			Params:   params,
		})

		if err != nil {
			return nil, fmt.Errorf("step %s failed: %w", step.Name, err)
		}

		if !output.Success {
			return nil, fmt.Errorf("step %s error: %s", step.Name, output.Error)
		}

		result = output.Result
	}

	return result, nil
}
```

### Usage Example

```go
workflow := patterns.NewWorkflow(framework, agentID)

workflow.
	AddStep("Create Issue", "mcp_github_create_issue", func(prev interface{}) map[string]interface{} {
		return map[string]interface{}{
			"owner": "company",
			"repo":  "product",
			"title": "Bug Report",
			"body":  "Description",
		}
	}).
	AddStep("Post to Slack", "mcp_slack_post_message", func(prev interface{}) map[string]interface{} {
		issue := prev.(map[string]interface{})
		return map[string]interface{}{
			"channel": "#bugs",
			"text":    fmt.Sprintf("New issue: %s", issue["html_url"]),
		}
	}).
	AddStep("Send Email", "mcp_gmail_send_message", func(prev interface{}) map[string]interface{} {
		return map[string]interface{}{
			"to":      "team@company.com",
			"subject": "Bug Report Created",
			"body":    "A new bug report has been filed",
		}
	})

result, err := workflow.Execute(ctx, nil)
```

## 🛠️ Part 4: Adaptive Caching

### Smart Cache with TTL and Invalidation

```go
package patterns

import (
	"context"
	"sync"
	"time"
)

type CacheEntry struct {
	Value      interface{}
	Expiry     time.Time
	AccessCount int
	LastAccess time.Time
}

type AdaptiveCache struct {
	entries map[string]*CacheEntry
	mu      sync.RWMutex

	// Adaptive parameters
	minTTL        time.Duration
	maxTTL        time.Duration
	hitRateTarget float64
}

func NewAdaptiveCache() *AdaptiveCache {
	return &AdaptiveCache{
		entries:       make(map[string]*CacheEntry),
		minTTL:        1 * time.Minute,
		maxTTL:        1 * time.Hour,
		hitRateTarget: 0.8,
	}
}

func (c *AdaptiveCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	// Check expiry
	if time.Now().After(entry.Expiry) {
		return nil, false
	}

	// Update stats
	entry.AccessCount++
	entry.LastAccess = time.Now()

	return entry.Value, true
}

func (c *AdaptiveCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Adapt TTL based on access patterns
	if entry, exists := c.entries[key]; exists {
		ttl = c.adaptTTL(entry, ttl)
	}

	c.entries[key] = &CacheEntry{
		Value:      value,
		Expiry:     time.Now().Add(ttl),
		AccessCount: 0,
		LastAccess: time.Now(),
	}
}

func (c *AdaptiveCache) adaptTTL(entry *CacheEntry, baseTTL time.Duration) time.Duration {
	// High access count → longer TTL
	if entry.AccessCount > 100 {
		ttl := baseTTL * 2
		if ttl > c.maxTTL {
			ttl = c.maxTTL
		}
		return ttl
	}

	// Low access count → shorter TTL
	if entry.AccessCount < 10 {
		ttl := baseTTL / 2
		if ttl < c.minTTL {
			ttl = c.minTTL
		}
		return ttl
	}

	return baseTTL
}

func (c *AdaptiveCache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

func (c *AdaptiveCache) InvalidatePattern(pattern string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.entries {
		if strings.Contains(key, pattern) {
			delete(c.entries, key)
		}
	}
}

func (c *AdaptiveCache) EvictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.Expiry) {
			delete(c.entries, key)
		}
	}
}
```

## 🛠️ Part 5: Rate Limiting

### Token Bucket Rate Limiter

```go
package patterns

import (
	"context"
	"sync"
	"time"
)

type RateLimiter struct {
	tokens     float64
	capacity   float64
	refillRate float64
	lastRefill time.Time
	mu         sync.Mutex
}

func NewRateLimiter(requestsPerSecond float64, burst int) *RateLimiter {
	return &RateLimiter{
		tokens:     float64(burst),
		capacity:   float64(burst),
		refillRate: requestsPerSecond,
		lastRefill: time.Now(),
	}
}

func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		allowed, waitTime := rl.allow()
		if allowed {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			continue
		}
	}
}

func (rl *RateLimiter) allow() (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()

	// Refill tokens
	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.capacity {
		rl.tokens = rl.capacity
	}
	rl.lastRefill = now

	// Check if we have tokens
	if rl.tokens >= 1 {
		rl.tokens--
		return true, 0
	}

	// Calculate wait time
	tokensNeeded := 1 - rl.tokens
	waitTime := time.Duration(tokensNeeded/rl.refillRate*1000) * time.Millisecond

	return false, waitTime
}
```

### Multi-Service Rate Limiter

```go
type MultiServiceRateLimiter struct {
	limiters map[string]*RateLimiter
	mu       sync.RWMutex
}

func NewMultiServiceRateLimiter() *MultiServiceRateLimiter {
	return &MultiServiceRateLimiter{
		limiters: make(map[string]*RateLimiter),
	}
}

func (m *MultiServiceRateLimiter) AddService(name string, requestsPerSecond float64, burst int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.limiters[name] = NewRateLimiter(requestsPerSecond, burst)
}

func (m *MultiServiceRateLimiter) Wait(ctx context.Context, service string) error {
	m.mu.RLock()
	limiter, exists := m.limiters[service]
	m.mu.RUnlock()

	if !exists {
		return nil // No limit for this service
	}

	return limiter.Wait(ctx)
}
```

### Usage Example

```go
rateLimiter := patterns.NewMultiServiceRateLimiter()
rateLimiter.AddService("github", 5, 10)    // 5 req/s, burst of 10
rateLimiter.AddService("slack", 1, 3)      // 1 req/s, burst of 3

// Before calling tool
if err := rateLimiter.Wait(ctx, "github"); err != nil {
	return err
}

output, err := manager.CallTool(ctx, "github", "create_issue", params)
```

## 🛠️ Part 6: Event-Driven Architecture

### Event Bus

```go
package patterns

import (
	"context"
	"sync"
)

type Event struct {
	Type    string
	Payload interface{}
}

type EventHandler func(ctx context.Context, event *Event) error

type EventBus struct {
	handlers map[string][]EventHandler
	mu       sync.RWMutex
}

func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[string][]EventHandler),
	}
}

func (eb *EventBus) Subscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
}

func (eb *EventBus) Publish(ctx context.Context, event *Event) error {
	eb.mu.RLock()
	handlers := eb.handlers[event.Type]
	eb.mu.RUnlock()

	var wg sync.WaitGroup
	errors := make([]error, 0)
	var mu sync.Mutex

	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()

			if err := h(ctx, event); err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}(handler)
	}

	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("%d handlers failed", len(errors))
	}

	return nil
}
```

### Usage Example

```go
bus := patterns.NewEventBus()

// Subscribe to events
bus.Subscribe("lead.created", func(ctx context.Context, event *patterns.Event) error {
	leadID := event.Payload.(string)
	log.Printf("New lead created: %s", leadID)

	// Send email
	return sendWelcomeEmail(leadID)
})

bus.Subscribe("lead.created", func(ctx context.Context, event *patterns.Event) error {
	leadID := event.Payload.(string)

	// Update analytics
	return updateAnalytics(leadID)
})

// Publish event
bus.Publish(ctx, &patterns.Event{
	Type:    "lead.created",
	Payload: "lead-12345",
})
```

## 🛠️ Part 7: Saga Pattern

### Distributed Transaction Coordinator

```go
package patterns

import (
	"context"
	"fmt"
)

type SagaStep struct {
	Name        string
	Execute     func(ctx context.Context) (interface{}, error)
	Compensate  func(ctx context.Context, result interface{}) error
}

type Saga struct {
	steps   []SagaStep
	results []interface{}
}

func NewSaga() *Saga {
	return &Saga{
		steps:   make([]SagaStep, 0),
		results: make([]interface{}, 0),
	}
}

func (s *Saga) AddStep(name string, execute func(ctx context.Context) (interface{}, error), compensate func(ctx context.Context, result interface{}) error) *Saga {
	s.steps = append(s.steps, SagaStep{
		Name:       name,
		Execute:    execute,
		Compensate: compensate,
	})
	return s
}

func (s *Saga) Execute(ctx context.Context) error {
	// Execute all steps
	for i, step := range s.steps {
		log.Printf("Executing step %d/%d: %s", i+1, len(s.steps), step.Name)

		result, err := step.Execute(ctx)
		if err != nil {
			log.Printf("Step %s failed: %v", step.Name, err)

			// Compensate previous steps
			return s.compensate(ctx, i)
		}

		s.results = append(s.results, result)
	}

	log.Println("✅ Saga completed successfully")
	return nil
}

func (s *Saga) compensate(ctx context.Context, failedStepIndex int) error {
	log.Printf("⚠️ Compensating %d completed steps...", failedStepIndex)

	// Compensate in reverse order
	for i := failedStepIndex - 1; i >= 0; i-- {
		step := s.steps[i]
		result := s.results[i]

		log.Printf("Compensating step: %s", step.Name)

		if err := step.Compensate(ctx, result); err != nil {
			log.Printf("⚠️ Compensation failed for %s: %v", step.Name, err)
			// Continue compensating other steps
		}
	}

	return fmt.Errorf("saga failed at step %d", failedStepIndex+1)
}
```

### Usage Example: Multi-Service Order

```go
saga := patterns.NewSaga()

var orderID, paymentID, inventoryID string

saga.
	AddStep("Create Order",
		func(ctx context.Context) (interface{}, error) {
			result, err := manager.CallTool(ctx, "orders", "create_order", orderData)
			if err == nil {
				orderID = result["id"].(string)
			}
			return result, err
		},
		func(ctx context.Context, result interface{}) error {
			_, err := manager.CallTool(ctx, "orders", "cancel_order", map[string]interface{}{
				"id": orderID,
			})
			return err
		},
	).
	AddStep("Process Payment",
		func(ctx context.Context) (interface{}, error) {
			result, err := manager.CallTool(ctx, "payments", "charge", paymentData)
			if err == nil {
				paymentID = result["id"].(string)
			}
			return result, err
		},
		func(ctx context.Context, result interface{}) error {
			_, err := manager.CallTool(ctx, "payments", "refund", map[string]interface{}{
				"id": paymentID,
			})
			return err
		},
	).
	AddStep("Reserve Inventory",
		func(ctx context.Context) (interface{}, error) {
			result, err := manager.CallTool(ctx, "inventory", "reserve", inventoryData)
			if err == nil {
				inventoryID = result["id"].(string)
			}
			return result, err
		},
		func(ctx context.Context, result interface{}) error {
			_, err := manager.CallTool(ctx, "inventory", "release", map[string]interface{}{
				"id": inventoryID,
			})
			return err
		},
	)

// Execute saga (will auto-compensate on failure)
if err := saga.Execute(ctx); err != nil {
	log.Printf("Order failed: %v", err)
}
```

## 🏋️ Practice Exercises

### Exercise 1: Build Retry Middleware

Combine retry pattern with middleware.

<details>
<summary>Click to see solution</summary>

```go
func RetryMiddleware(config *RetryConfig) Middleware {
	retrier := NewRetrier(config)

	return func(next ToolExecutor) ToolExecutor {
		return &retryExecutor{
			next:    next,
			retrier: retrier,
		}
	}
}

type retryExecutor struct {
	next    ToolExecutor
	retrier *Retrier
}

func (e *retryExecutor) ExecuteTool(ctx context.Context, agentID string, input *models.ToolInput) (*models.ToolOutput, error) {
	var output *models.ToolOutput
	var err error

	retryErr := e.retrier.Execute(ctx, func() error {
		output, err = e.next.ExecuteTool(ctx, agentID, input)
		if err != nil {
			return err
		}
		if !output.Success {
			return fmt.Errorf("tool error: %s", output.Error)
		}
		return nil
	})

	if retryErr != nil {
		return nil, retryErr
	}

	return output, nil
}
```
</details>

### Exercise 2: Implement Circuit Breaker Middleware

Add circuit breaker as middleware.

<details>
<summary>Click to see solution</summary>

```go
func CircuitBreakerMiddleware(cb *client.CircuitBreaker) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return &circuitBreakerExecutor{
			next: next,
			cb:   cb,
		}
	}
}

type circuitBreakerExecutor struct {
	next ToolExecutor
	cb   *client.CircuitBreaker
}

func (e *circuitBreakerExecutor) ExecuteTool(ctx context.Context, agentID string, input *models.ToolInput) (*models.ToolOutput, error) {
	var output *models.ToolOutput
	var err error

	cbErr := e.cb.Execute(ctx, func(ctx context.Context) error {
		output, err = e.next.ExecuteTool(ctx, agentID, input)
		if err != nil {
			return err
		}
		if !output.Success {
			return fmt.Errorf("tool failed: %s", output.Error)
		}
		return nil
	})

	if cbErr != nil {
		return nil, cbErr
	}

	return output, nil
}
```
</details>

## 📝 Summary

Congratulations! You've mastered:

✅ Retry strategies with exponential backoff
✅ Middleware pattern for cross-cutting concerns
✅ Tool chaining for complex workflows
✅ Adaptive caching with intelligent invalidation
✅ Rate limiting across multiple services
✅ Event-driven architecture
✅ Saga pattern for distributed transactions

### Pattern Selection Guide

| Scenario | Pattern | Why |
|----------|---------|-----|
| Network failures | Retry + Backoff | Handle transient errors |
| Logging/Metrics | Middleware | Cross-cutting concerns |
| Multi-step tasks | Tool Chaining | Compose workflows |
| Frequently accessed data | Adaptive Cache | Reduce latency |
| API quotas | Rate Limiting | Respect limits |
| Async processing | Event-Driven | Decouple components |
| Multi-service updates | Saga | Maintain consistency |

### Production Stack

```go
executor := patterns.Chain(
	framework,
	patterns.LoggingMiddleware(),
	patterns.MetricsMiddleware(collector),
	patterns.CircuitBreakerMiddleware(cb),
	patterns.RetryMiddleware(retryConfig),
	patterns.TimeoutMiddleware(30*time.Second),
	patterns.RateLimitMiddleware(rateLimiter),
)
```

## 🎯 Conclusion

You've completed all 9 tutorials! You now know:

1. Framework basics
2. MCP integration
3. Building complete agents
4. Advanced features (pool, cache, circuit breaker, metrics)
5. Multi-server orchestration
6. Production deployment
7. Building a Virtual SDR
8. Creating custom MCP servers
9. Advanced patterns

### What's Next?

- Build your own production agents
- Contribute to the Minion framework
- Share your custom MCP servers
- Join the community discussions

---

**Congratulations! 🎉 You've mastered the Minion Framework!**
