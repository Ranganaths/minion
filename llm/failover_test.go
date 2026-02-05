package llm

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// MockProvider is a mock LLM provider for testing
type MockProvider struct {
	name           string
	shouldFail     bool
	failureCount   int32
	latency        time.Duration
	healthCheckErr error
}

func NewMockProvider(name string) *MockProvider {
	return &MockProvider{name: name}
}

func (m *MockProvider) GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	if m.latency > 0 {
		select {
		case <-time.After(m.latency):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.shouldFail {
		atomic.AddInt32(&m.failureCount, 1)
		return nil, errors.New("provider error: temporary failure")
	}

	return &CompletionResponse{
		Text:         "mock response",
		TokensUsed:   10,
		FinishReason: "stop",
		Model:        req.Model,
	}, nil
}

func (m *MockProvider) GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if m.latency > 0 {
		select {
		case <-time.After(m.latency):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.shouldFail {
		atomic.AddInt32(&m.failureCount, 1)
		return nil, errors.New("provider error: temporary failure")
	}

	return &ChatResponse{
		Message:      Message{Role: "assistant", Content: "mock response"},
		TokensUsed:   10,
		FinishReason: "stop",
		Model:        req.Model,
	}, nil
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) HealthCheck(ctx context.Context) error {
	return m.healthCheckErr
}

func (m *MockProvider) SetFail(fail bool) {
	m.shouldFail = fail
}

func (m *MockProvider) SetLatency(d time.Duration) {
	m.latency = d
}

func (m *MockProvider) GetFailureCount() int32 {
	return atomic.LoadInt32(&m.failureCount)
}

func TestFailoverProviderBasic(t *testing.T) {
	p1 := NewMockProvider("provider1")
	p2 := NewMockProvider("provider2")

	fp, err := NewFailoverProvider(FailoverConfig{
		Providers: []Provider{p1, p2},
	})
	if err != nil {
		t.Fatalf("Failed to create failover provider: %v", err)
	}

	ctx := context.Background()
	req := &CompletionRequest{
		Model:      "test-model",
		UserPrompt: "test",
	}

	resp, err := fp.GenerateCompletion(ctx, req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response, got nil")
	}
}

func TestFailoverOnPrimaryFailure(t *testing.T) {
	p1 := NewMockProvider("provider1")
	p1.SetFail(true)
	p2 := NewMockProvider("provider2")

	fp, err := NewFailoverProvider(FailoverConfig{
		Providers:     []Provider{p1, p2},
		EnableMetrics: true,
	})
	if err != nil {
		t.Fatalf("Failed to create failover provider: %v", err)
	}

	ctx := context.Background()
	req := &CompletionRequest{
		Model:      "test-model",
		UserPrompt: "test",
	}

	resp, err := fp.GenerateCompletion(ctx, req)
	if err != nil {
		t.Fatalf("Expected failover to succeed, got error: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response from failover")
	}

	// Check that p1 was attempted
	if p1.GetFailureCount() == 0 {
		t.Error("Primary provider should have been attempted")
	}

	// Check metrics
	metrics := fp.GetMetrics()
	if metrics == nil {
		t.Fatal("Expected metrics")
	}
	if metrics["failover_count"].(int64) == 0 {
		t.Error("Expected failover count > 0")
	}
}

func TestAllProvidersFail(t *testing.T) {
	p1 := NewMockProvider("provider1")
	p1.SetFail(true)
	p2 := NewMockProvider("provider2")
	p2.SetFail(true)

	fp, err := NewFailoverProvider(FailoverConfig{
		Providers: []Provider{p1, p2},
		RetryConfig: &RetryConfig{
			MaxRetries:      1,
			InitialDelay:    time.Millisecond,
			MaxDelay:        10 * time.Millisecond,
			BackoffFactor:   2.0,
			RetryableErrors: []string{"temporary"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create failover provider: %v", err)
	}

	ctx := context.Background()
	req := &CompletionRequest{
		Model:      "test-model",
		UserPrompt: "test",
	}

	_, err = fp.GenerateCompletion(ctx, req)
	if err == nil {
		t.Fatal("Expected error when all providers fail")
	}
}

func TestPriorityFailover(t *testing.T) {
	p1 := NewMockProvider("provider1")
	p2 := NewMockProvider("provider2")
	p3 := NewMockProvider("provider3")

	strategy := NewPriorityFailover()

	ctx := context.Background()
	attempted := make(map[string]bool)

	// First selection should be provider1
	selected, err := strategy.SelectProvider(ctx, []Provider{p1, p2, p3}, attempted)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if selected.Name() != "provider1" {
		t.Errorf("Expected provider1, got %s", selected.Name())
	}

	// Mark provider1 as attempted
	attempted["provider1"] = true

	// Second selection should be provider2
	selected, err = strategy.SelectProvider(ctx, []Provider{p1, p2, p3}, attempted)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if selected.Name() != "provider2" {
		t.Errorf("Expected provider2, got %s", selected.Name())
	}
}

func TestRoundRobinFailover(t *testing.T) {
	p1 := NewMockProvider("provider1")
	p2 := NewMockProvider("provider2")

	strategy := NewRoundRobinFailover()
	ctx := context.Background()

	// Track selections
	selections := make(map[string]int)
	for i := 0; i < 100; i++ {
		selected, err := strategy.SelectProvider(ctx, []Provider{p1, p2}, make(map[string]bool))
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		selections[selected.Name()]++
	}

	// Should be roughly equal distribution
	if selections["provider1"] < 40 || selections["provider2"] < 40 {
		t.Errorf("Round robin should distribute evenly: %v", selections)
	}
}

func TestLatencyBasedFailover(t *testing.T) {
	p1 := NewMockProvider("provider1")
	p2 := NewMockProvider("provider2")

	strategy := NewLatencyBasedFailover()

	// Record latencies
	strategy.OnSuccess(p1, 100*time.Millisecond)
	strategy.OnSuccess(p2, 50*time.Millisecond)

	ctx := context.Background()
	selected, err := strategy.SelectProvider(ctx, []Provider{p1, p2}, make(map[string]bool))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should select provider2 (lower latency)
	if selected.Name() != "provider2" {
		t.Errorf("Expected provider2 (lower latency), got %s", selected.Name())
	}
}

func TestWeightedFailover(t *testing.T) {
	p1 := NewMockProvider("provider1")
	p2 := NewMockProvider("provider2")

	weights := map[string]float64{
		"provider1": 1.0,
		"provider2": 9.0, // 9x weight for provider2
	}

	strategy := NewWeightedFailover(weights)
	ctx := context.Background()

	// Track selections
	selections := make(map[string]int)
	for i := 0; i < 1000; i++ {
		selected, err := strategy.SelectProvider(ctx, []Provider{p1, p2}, make(map[string]bool))
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		selections[selected.Name()]++
	}

	// provider2 should be selected much more often (approximately 9:1 ratio)
	if selections["provider2"] < 800 {
		t.Errorf("Weighted selection should favor provider2: got %d vs %d", selections["provider2"], selections["provider1"])
	}
}

func TestHealthMonitor(t *testing.T) {
	p1 := NewMockProvider("provider1")
	p2 := NewMockProvider("provider2")
	p2.healthCheckErr = errors.New("unhealthy")

	monitor := NewHealthMonitorWithConfig([]Provider{p1, p2}, &HealthMonitorConfig{
		CheckInterval:      50 * time.Millisecond,
		UnhealthyThreshold: 2,
		RecoveryThreshold:  1,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)

	// Wait for health checks
	time.Sleep(200 * time.Millisecond)

	healthy := monitor.GetHealthyProviders()

	// Should only have provider1
	hasP1 := false
	hasP2 := false
	for _, p := range healthy {
		if p.Name() == "provider1" {
			hasP1 = true
		}
		if p.Name() == "provider2" {
			hasP2 = true
		}
	}

	if !hasP1 {
		t.Error("Expected provider1 to be healthy")
	}
	if hasP2 {
		t.Error("Expected provider2 to be unhealthy")
	}

	monitor.Stop()
}

func TestHealthMonitorRecovery(t *testing.T) {
	p1 := NewMockProvider("provider1")

	monitor := NewHealthMonitorWithConfig([]Provider{p1}, &HealthMonitorConfig{
		CheckInterval:      50 * time.Millisecond,
		UnhealthyThreshold: 2,
		RecoveryThreshold:  2,
	})

	// Record failures to mark unhealthy
	monitor.RecordFailure("provider1", errors.New("error"))
	monitor.RecordFailure("provider1", errors.New("error"))

	health := monitor.GetProviderHealth("provider1")
	if health.IsHealthy() {
		t.Error("Provider should be unhealthy after consecutive failures")
	}

	// Record successes to recover
	monitor.RecordSuccess("provider1", 100*time.Millisecond)
	monitor.RecordSuccess("provider1", 100*time.Millisecond)

	if !health.IsHealthy() {
		t.Error("Provider should recover after consecutive successes")
	}
}

func TestBudget(t *testing.T) {
	budget := NewBudget(10.0, 5.0, 1.0)

	// Should be able to spend within budget
	if !budget.CanSpend(0.5) {
		t.Error("Should be able to spend 0.5 with 10.0 daily limit")
	}

	// Per-request max
	if budget.CanSpend(2.0) {
		t.Error("Should not be able to spend 2.0 with 1.0 per-request max")
	}

	// Spend some
	budget.Spend(3.0)

	daily, hourly := budget.GetRemaining()
	if daily != 7.0 {
		t.Errorf("Expected 7.0 daily remaining, got %f", daily)
	}
	if hourly != 2.0 {
		t.Errorf("Expected 2.0 hourly remaining, got %f", hourly)
	}

	// Spend more than hourly limit
	budget.Spend(2.5)

	// Should not be able to spend more (hourly limit exceeded)
	if budget.CanSpend(0.1) {
		t.Error("Should not be able to spend when hourly limit exceeded")
	}
}

func TestCostBasedFailover(t *testing.T) {
	p1 := NewMockProvider("provider1")
	p2 := NewMockProvider("provider2")

	pricing := map[string]*ModelPricing{
		"provider1": {InputPricePerK: 0.01, OutputPricePerK: 0.02},
		"provider2": {InputPricePerK: 0.001, OutputPricePerK: 0.002},
	}

	strategy := NewCostBasedFailover(pricing, nil)
	ctx := context.Background()

	selected, err := strategy.SelectProvider(ctx, []Provider{p1, p2}, make(map[string]bool))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should select provider2 (cheaper)
	if selected.Name() != "provider2" {
		t.Errorf("Expected provider2 (cheaper), got %s", selected.Name())
	}
}

func TestChatFailover(t *testing.T) {
	p1 := NewMockProvider("provider1")
	p1.SetFail(true)
	p2 := NewMockProvider("provider2")

	fp, err := NewFailoverProvider(FailoverConfig{
		Providers: []Provider{p1, p2},
	})
	if err != nil {
		t.Fatalf("Failed to create failover provider: %v", err)
	}

	ctx := context.Background()
	req := &ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
	}

	resp, err := fp.GenerateChat(ctx, req)
	if err != nil {
		t.Fatalf("Expected failover to succeed for chat: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected chat response from failover")
	}
}

func TestContextCancellation(t *testing.T) {
	p1 := NewMockProvider("provider1")
	p1.SetLatency(5 * time.Second) // Long latency

	fp, err := NewFailoverProvider(FailoverConfig{
		Providers: []Provider{p1},
	})
	if err != nil {
		t.Fatalf("Failed to create failover provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := &CompletionRequest{
		Model:      "test-model",
		UserPrompt: "test",
	}

	_, err = fp.GenerateCompletion(ctx, req)
	if err == nil {
		t.Fatal("Expected timeout error")
	}
}

// Ensure MockProvider implements the interfaces
var _ Provider = (*MockProvider)(nil)
var _ HealthCheckProvider = (*MockProvider)(nil)
