package llm

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// FailoverStrategy defines how to select providers during failover
type FailoverStrategy interface {
	// SelectProvider selects a provider from the available providers
	SelectProvider(ctx context.Context, providers []Provider, attempted map[string]bool) (Provider, error)

	// OnFailure is called when a provider fails
	OnFailure(provider Provider, err error)

	// OnSuccess is called when a provider succeeds
	OnSuccess(provider Provider, latency time.Duration)
}

// PriorityFailover tries providers in priority order
type PriorityFailover struct{}

// NewPriorityFailover creates a new priority failover strategy
func NewPriorityFailover() *PriorityFailover {
	return &PriorityFailover{}
}

// SelectProvider selects the first non-attempted provider
func (f *PriorityFailover) SelectProvider(ctx context.Context, providers []Provider, attempted map[string]bool) (Provider, error) {
	for _, p := range providers {
		if !attempted[p.Name()] {
			return p, nil
		}
	}
	return nil, errors.New("no more providers available")
}

func (f *PriorityFailover) OnFailure(provider Provider, err error) {}
func (f *PriorityFailover) OnSuccess(provider Provider, latency time.Duration) {}

// RoundRobinFailover distributes requests across providers
type RoundRobinFailover struct {
	counter uint64
}

// NewRoundRobinFailover creates a new round-robin failover strategy
func NewRoundRobinFailover() *RoundRobinFailover {
	return &RoundRobinFailover{}
}

// SelectProvider selects the next provider in round-robin order
func (f *RoundRobinFailover) SelectProvider(ctx context.Context, providers []Provider, attempted map[string]bool) (Provider, error) {
	if len(providers) == 0 {
		return nil, errors.New("no providers available")
	}

	// Filter out attempted providers
	available := make([]Provider, 0)
	for _, p := range providers {
		if !attempted[p.Name()] {
			available = append(available, p)
		}
	}

	if len(available) == 0 {
		return nil, errors.New("no more providers available")
	}

	idx := atomic.AddUint64(&f.counter, 1) % uint64(len(available))
	return available[idx], nil
}

func (f *RoundRobinFailover) OnFailure(provider Provider, err error)            {}
func (f *RoundRobinFailover) OnSuccess(provider Provider, latency time.Duration) {}

// LatencyBasedFailover selects providers based on average latency
type LatencyBasedFailover struct {
	latencies map[string]time.Duration
	mu        sync.RWMutex
}

// NewLatencyBasedFailover creates a new latency-based failover strategy
func NewLatencyBasedFailover() *LatencyBasedFailover {
	return &LatencyBasedFailover{
		latencies: make(map[string]time.Duration),
	}
}

// SelectProvider selects the provider with lowest average latency
func (f *LatencyBasedFailover) SelectProvider(ctx context.Context, providers []Provider, attempted map[string]bool) (Provider, error) {
	if len(providers) == 0 {
		return nil, errors.New("no providers available")
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Filter out attempted providers and sort by latency
	type providerLatency struct {
		provider Provider
		latency  time.Duration
	}

	available := make([]providerLatency, 0)
	for _, p := range providers {
		if !attempted[p.Name()] {
			latency := f.latencies[p.Name()]
			if latency == 0 {
				latency = time.Second // Default for unknown providers
			}
			available = append(available, providerLatency{p, latency})
		}
	}

	if len(available) == 0 {
		return nil, errors.New("no more providers available")
	}

	// Sort by latency (lowest first)
	sort.Slice(available, func(i, j int) bool {
		return available[i].latency < available[j].latency
	})

	return available[0].provider, nil
}

func (f *LatencyBasedFailover) OnFailure(provider Provider, err error) {}

func (f *LatencyBasedFailover) OnSuccess(provider Provider, latency time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Exponential moving average
	name := provider.Name()
	current := f.latencies[name]
	if current == 0 {
		f.latencies[name] = latency
	} else {
		// EMA with alpha = 0.3
		f.latencies[name] = time.Duration(float64(current)*0.7 + float64(latency)*0.3)
	}
}

// WeightedFailover selects providers based on configurable weights
type WeightedFailover struct {
	weights map[string]float64
	mu      sync.RWMutex
	rng     *rand.Rand
}

// NewWeightedFailover creates a new weighted failover strategy
func NewWeightedFailover(weights map[string]float64) *WeightedFailover {
	return &WeightedFailover{
		weights: weights,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SelectProvider selects a provider based on weights
func (f *WeightedFailover) SelectProvider(ctx context.Context, providers []Provider, attempted map[string]bool) (Provider, error) {
	if len(providers) == 0 {
		return nil, errors.New("no providers available")
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Calculate total weight of available providers
	var totalWeight float64
	available := make([]Provider, 0)
	weights := make([]float64, 0)

	for _, p := range providers {
		if !attempted[p.Name()] {
			available = append(available, p)
			weight := f.weights[p.Name()]
			if weight <= 0 {
				weight = 1.0 // Default weight
			}
			weights = append(weights, weight)
			totalWeight += weight
		}
	}

	if len(available) == 0 {
		return nil, errors.New("no more providers available")
	}

	// Random selection based on weights
	r := f.rng.Float64() * totalWeight
	var cumulative float64
	for i, weight := range weights {
		cumulative += weight
		if r <= cumulative {
			return available[i], nil
		}
	}

	return available[len(available)-1], nil
}

func (f *WeightedFailover) OnFailure(provider Provider, err error)            {}
func (f *WeightedFailover) OnSuccess(provider Provider, latency time.Duration) {}

// SetWeight updates the weight for a provider
func (f *WeightedFailover) SetWeight(name string, weight float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.weights[name] = weight
}

// CostBasedFailover selects providers based on cost
type CostBasedFailover struct {
	pricing map[string]*ModelPricing
	budget  *Budget
	mu      sync.RWMutex
}

// ModelPricing defines pricing for a model
type ModelPricing struct {
	ProviderName     string
	ModelName        string
	InputPricePerK   float64 // Price per 1K input tokens
	OutputPricePerK  float64 // Price per 1K output tokens
}

// Budget tracks spending limits
type Budget struct {
	DailyLimit    float64
	HourlyLimit   float64
	PerRequestMax float64
	spent         float64
	hourlySpent   float64
	lastReset     time.Time
	lastHourReset time.Time
	mu            sync.Mutex
}

// NewBudget creates a new budget tracker
func NewBudget(dailyLimit, hourlyLimit, perRequestMax float64) *Budget {
	now := time.Now()
	return &Budget{
		DailyLimit:    dailyLimit,
		HourlyLimit:   hourlyLimit,
		PerRequestMax: perRequestMax,
		lastReset:     now,
		lastHourReset: now,
	}
}

// CanSpend checks if we can spend the given amount
func (b *Budget) CanSpend(amount float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.checkResets()

	if b.PerRequestMax > 0 && amount > b.PerRequestMax {
		return false
	}
	if b.DailyLimit > 0 && b.spent+amount > b.DailyLimit {
		return false
	}
	if b.HourlyLimit > 0 && b.hourlySpent+amount > b.HourlyLimit {
		return false
	}

	return true
}

// Spend records spending
func (b *Budget) Spend(amount float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.checkResets()
	b.spent += amount
	b.hourlySpent += amount
}

// checkResets checks and performs periodic resets
func (b *Budget) checkResets() {
	now := time.Now()

	// Daily reset
	if now.Sub(b.lastReset) >= 24*time.Hour {
		b.spent = 0
		b.lastReset = now
	}

	// Hourly reset
	if now.Sub(b.lastHourReset) >= time.Hour {
		b.hourlySpent = 0
		b.lastHourReset = now
	}
}

// GetRemaining returns remaining budget
func (b *Budget) GetRemaining() (daily, hourly float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.checkResets()

	daily = b.DailyLimit - b.spent
	if daily < 0 {
		daily = 0
	}
	hourly = b.HourlyLimit - b.hourlySpent
	if hourly < 0 {
		hourly = 0
	}

	return daily, hourly
}

// NewCostBasedFailover creates a new cost-based failover strategy
func NewCostBasedFailover(pricing map[string]*ModelPricing, budget *Budget) *CostBasedFailover {
	return &CostBasedFailover{
		pricing: pricing,
		budget:  budget,
	}
}

// SelectProvider selects the cheapest provider within budget
func (f *CostBasedFailover) SelectProvider(ctx context.Context, providers []Provider, attempted map[string]bool) (Provider, error) {
	if len(providers) == 0 {
		return nil, errors.New("no providers available")
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Sort providers by cost (cheapest first)
	type providerCost struct {
		provider Provider
		cost     float64
	}

	available := make([]providerCost, 0)
	for _, p := range providers {
		if !attempted[p.Name()] {
			pricing := f.pricing[p.Name()]
			cost := 0.0
			if pricing != nil {
				// Estimate cost for typical request (assume 1K tokens)
				cost = pricing.InputPricePerK + pricing.OutputPricePerK
			}
			available = append(available, providerCost{p, cost})
		}
	}

	if len(available) == 0 {
		return nil, errors.New("no more providers available")
	}

	// Sort by cost
	sort.Slice(available, func(i, j int) bool {
		return available[i].cost < available[j].cost
	})

	// Check budget for each provider
	for _, pc := range available {
		if f.budget == nil || f.budget.CanSpend(pc.cost) {
			return pc.provider, nil
		}
	}

	// Return cheapest even if over budget (let caller handle)
	return available[0].provider, nil
}

func (f *CostBasedFailover) OnFailure(provider Provider, err error) {}

func (f *CostBasedFailover) OnSuccess(provider Provider, latency time.Duration) {
	// Could track actual costs here if available
}

// SetPricing updates pricing for a provider
func (f *CostBasedFailover) SetPricing(name string, pricing *ModelPricing) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pricing[name] = pricing
}

// AdaptiveFailover combines multiple strategies with dynamic adjustment
type AdaptiveFailover struct {
	latencyBased *LatencyBasedFailover
	costBased    *CostBasedFailover
	latencyWeight float64 // Weight for latency (0-1), cost weight is 1-latencyWeight
	mu            sync.RWMutex
}

// NewAdaptiveFailover creates a new adaptive failover strategy
func NewAdaptiveFailover(pricing map[string]*ModelPricing, budget *Budget, latencyWeight float64) *AdaptiveFailover {
	if latencyWeight < 0 {
		latencyWeight = 0
	}
	if latencyWeight > 1 {
		latencyWeight = 1
	}

	return &AdaptiveFailover{
		latencyBased:  NewLatencyBasedFailover(),
		costBased:     NewCostBasedFailover(pricing, budget),
		latencyWeight: latencyWeight,
	}
}

// SelectProvider selects provider based on combined latency and cost score
func (f *AdaptiveFailover) SelectProvider(ctx context.Context, providers []Provider, attempted map[string]bool) (Provider, error) {
	if len(providers) == 0 {
		return nil, errors.New("no providers available")
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Calculate combined scores
	type scored struct {
		provider Provider
		score    float64
	}

	available := make([]scored, 0)

	// Get latency stats
	f.latencyBased.mu.RLock()
	latencies := make(map[string]time.Duration)
	for k, v := range f.latencyBased.latencies {
		latencies[k] = v
	}
	f.latencyBased.mu.RUnlock()

	// Get cost stats
	f.costBased.mu.RLock()
	pricing := make(map[string]*ModelPricing)
	for k, v := range f.costBased.pricing {
		pricing[k] = v
	}
	f.costBased.mu.RUnlock()

	// Normalize scores
	var maxLatency time.Duration
	var maxCost float64

	for _, p := range providers {
		if !attempted[p.Name()] {
			lat := latencies[p.Name()]
			if lat > maxLatency {
				maxLatency = lat
			}
			if pr := pricing[p.Name()]; pr != nil {
				cost := pr.InputPricePerK + pr.OutputPricePerK
				if cost > maxCost {
					maxCost = cost
				}
			}
		}
	}

	if maxLatency == 0 {
		maxLatency = time.Second
	}
	if maxCost == 0 {
		maxCost = 0.1
	}

	for _, p := range providers {
		if !attempted[p.Name()] {
			// Normalize latency (0-1, lower is better)
			lat := latencies[p.Name()]
			if lat == 0 {
				lat = time.Second
			}
			latScore := 1.0 - float64(lat)/float64(maxLatency)

			// Normalize cost (0-1, lower is better)
			costScore := 1.0
			if pr := pricing[p.Name()]; pr != nil {
				cost := pr.InputPricePerK + pr.OutputPricePerK
				costScore = 1.0 - cost/maxCost
			}

			// Combined score
			score := f.latencyWeight*latScore + (1-f.latencyWeight)*costScore
			available = append(available, scored{p, score})
		}
	}

	if len(available) == 0 {
		return nil, errors.New("no more providers available")
	}

	// Sort by score (highest first)
	sort.Slice(available, func(i, j int) bool {
		return available[i].score > available[j].score
	})

	return available[0].provider, nil
}

func (f *AdaptiveFailover) OnFailure(provider Provider, err error) {
	f.latencyBased.OnFailure(provider, err)
	f.costBased.OnFailure(provider, err)
}

func (f *AdaptiveFailover) OnSuccess(provider Provider, latency time.Duration) {
	f.latencyBased.OnSuccess(provider, latency)
	f.costBased.OnSuccess(provider, latency)
}

// SetLatencyWeight adjusts the latency vs cost weight
func (f *AdaptiveFailover) SetLatencyWeight(weight float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if weight < 0 {
		weight = 0
	}
	if weight > 1 {
		weight = 1
	}
	f.latencyWeight = weight
}
