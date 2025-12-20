package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// CostConfig contains cost tracking configuration
type CostConfig struct {
	Enabled              bool
	PricingFile          string
	BudgetAlertThreshold float64 // USD per day
	Currency             string
}

// ModelPricing contains pricing information for LLM models
type ModelPricing struct {
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	PromptPricePer1K float64 `json:"prompt_price_per_1k"`  // USD per 1K prompt tokens
	CompletionPricePer1K float64 `json:"completion_price_per_1k"` // USD per 1K completion tokens
	LastUpdated      string  `json:"last_updated"`
}

// CostRecord represents a single cost record
type CostRecord struct {
	Timestamp        time.Time `json:"timestamp"`
	AgentID          string    `json:"agent_id"`
	SessionID        string    `json:"session_id"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	Cost             float64   `json:"cost"`
	Currency         string    `json:"currency"`
}

// CostSummary represents aggregated cost statistics
type CostSummary struct {
	TotalCost          float64          `json:"total_cost"`
	TotalTokens        int              `json:"total_tokens"`
	TotalRequests      int              `json:"total_requests"`
	CostByProvider     map[string]float64 `json:"cost_by_provider"`
	CostByModel        map[string]float64 `json:"cost_by_model"`
	CostByAgent        map[string]float64 `json:"cost_by_agent"`
	TokensByModel      map[string]int   `json:"tokens_by_model"`
	Currency           string           `json:"currency"`
	StartTime          time.Time        `json:"start_time"`
	EndTime            time.Time        `json:"end_time"`
}

// CostTracker tracks and manages LLM costs
type CostTracker struct {
	config    CostConfig
	pricing   map[string]ModelPricing // key: provider:model
	records   []CostRecord
	mu        sync.RWMutex
	startTime time.Time
}

// NewCostTracker creates a new cost tracker
func NewCostTracker(config CostConfig) (*CostTracker, error) {
	tracker := &CostTracker{
		config:    config,
		pricing:   make(map[string]ModelPricing),
		records:   make([]CostRecord, 0),
		startTime: time.Now(),
	}

	if config.Enabled && config.PricingFile != "" {
		if err := tracker.LoadPricing(config.PricingFile); err != nil {
			return nil, fmt.Errorf("failed to load pricing: %w", err)
		}
	} else {
		// Load default pricing
		tracker.LoadDefaultPricing()
	}

	return tracker, nil
}

// LoadPricing loads pricing from a JSON file
func (t *CostTracker) LoadPricing(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open pricing file: %w", err)
	}
	defer file.Close()

	var pricingList []ModelPricing
	if err := json.NewDecoder(file).Decode(&pricingList); err != nil {
		return fmt.Errorf("failed to decode pricing file: %w", err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	for _, p := range pricingList {
		key := fmt.Sprintf("%s:%s", p.Provider, p.Model)
		t.pricing[key] = p
	}

	return nil
}

// LoadDefaultPricing loads default pricing for common models
func (t *CostTracker) LoadDefaultPricing() {
	t.mu.Lock()
	defer t.mu.Unlock()

	// OpenAI pricing (as of 2024)
	t.pricing["openai:gpt-4-turbo-preview"] = ModelPricing{
		Provider:             "openai",
		Model:                "gpt-4-turbo-preview",
		PromptPricePer1K:     0.01,  // $0.01 per 1K prompt tokens
		CompletionPricePer1K: 0.03,  // $0.03 per 1K completion tokens
		LastUpdated:          "2024-01-01",
	}

	t.pricing["openai:gpt-4"] = ModelPricing{
		Provider:             "openai",
		Model:                "gpt-4",
		PromptPricePer1K:     0.03,
		CompletionPricePer1K: 0.06,
		LastUpdated:          "2024-01-01",
	}

	t.pricing["openai:gpt-3.5-turbo"] = ModelPricing{
		Provider:             "openai",
		Model:                "gpt-3.5-turbo",
		PromptPricePer1K:     0.0015,
		CompletionPricePer1K: 0.002,
		LastUpdated:          "2024-01-01",
	}

	// Anthropic pricing
	t.pricing["anthropic:claude-3-opus"] = ModelPricing{
		Provider:             "anthropic",
		Model:                "claude-3-opus",
		PromptPricePer1K:     0.015,
		CompletionPricePer1K: 0.075,
		LastUpdated:          "2024-01-01",
	}

	t.pricing["anthropic:claude-3-sonnet"] = ModelPricing{
		Provider:             "anthropic",
		Model:                "claude-3-sonnet",
		PromptPricePer1K:     0.003,
		CompletionPricePer1K: 0.015,
		LastUpdated:          "2024-01-01",
	}

	t.pricing["anthropic:claude-3-haiku"] = ModelPricing{
		Provider:             "anthropic",
		Model:                "claude-3-haiku",
		PromptPricePer1K:     0.00025,
		CompletionPricePer1K: 0.00125,
		LastUpdated:          "2024-01-01",
	}

	// Google Gemini pricing
	t.pricing["gemini:gemini-pro"] = ModelPricing{
		Provider:             "gemini",
		Model:                "gemini-pro",
		PromptPricePer1K:     0.00025,
		CompletionPricePer1K: 0.0005,
		LastUpdated:          "2024-01-01",
	}
}

// CalculateCost calculates the cost for a given number of tokens
func (t *CostTracker) CalculateCost(provider, model string, promptTokens, completionTokens int) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", provider, model)
	pricing, ok := t.pricing[key]
	if !ok {
		// If pricing not found, return 0 (or you could use a default)
		return 0
	}

	promptCost := (float64(promptTokens) / 1000.0) * pricing.PromptPricePer1K
	completionCost := (float64(completionTokens) / 1000.0) * pricing.CompletionPricePer1K

	return promptCost + completionCost
}

// RecordCost records a cost entry
func (t *CostTracker) RecordCost(ctx context.Context, agentID, sessionID, provider, model string, promptTokens, completionTokens int) float64 {
	if !t.config.Enabled {
		return 0
	}

	cost := t.CalculateCost(provider, model, promptTokens, completionTokens)

	record := CostRecord{
		Timestamp:        time.Now(),
		AgentID:          agentID,
		SessionID:        sessionID,
		Provider:         provider,
		Model:            model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		Cost:             cost,
		Currency:         t.config.Currency,
	}

	t.mu.Lock()
	t.records = append(t.records, record)
	t.mu.Unlock()

	// Check budget alert
	if t.config.BudgetAlertThreshold > 0 {
		t.checkBudgetAlert()
	}

	// Record to metrics
	RecordLLMRequest(provider, model, 0, promptTokens, completionTokens, cost, nil)

	return cost
}

// GetSummary returns a cost summary for the specified time range
func (t *CostTracker) GetSummary(startTime, endTime time.Time) *CostSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	summary := &CostSummary{
		CostByProvider: make(map[string]float64),
		CostByModel:    make(map[string]float64),
		CostByAgent:    make(map[string]float64),
		TokensByModel:  make(map[string]int),
		Currency:       t.config.Currency,
		StartTime:      startTime,
		EndTime:        endTime,
	}

	for _, record := range t.records {
		if record.Timestamp.Before(startTime) || record.Timestamp.After(endTime) {
			continue
		}

		summary.TotalCost += record.Cost
		summary.TotalTokens += record.PromptTokens + record.CompletionTokens
		summary.TotalRequests++

		summary.CostByProvider[record.Provider] += record.Cost
		summary.CostByModel[record.Model] += record.Cost
		summary.CostByAgent[record.AgentID] += record.Cost
		summary.TokensByModel[record.Model] += record.PromptTokens + record.CompletionTokens
	}

	return summary
}

// GetDailySummary returns a summary for the current day
func (t *CostTracker) GetDailySummary() *CostSummary {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	return t.GetSummary(startOfDay, endOfDay)
}

// GetMonthlySummary returns a summary for the current month
func (t *CostTracker) GetMonthlySummary() *CostSummary {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	return t.GetSummary(startOfMonth, endOfMonth)
}

// GetTotalSummary returns a summary for all recorded costs
func (t *CostTracker) GetTotalSummary() *CostSummary {
	return t.GetSummary(t.startTime, time.Now())
}

// checkBudgetAlert checks if daily cost exceeds budget threshold
func (t *CostTracker) checkBudgetAlert() {
	dailySummary := t.GetDailySummary()

	if dailySummary.TotalCost > t.config.BudgetAlertThreshold {
		// Log alert using fmt (logger may not be available in this context)
		fmt.Printf("[WARN] Daily budget threshold exceeded: $%.2f / $%.2f\n",
			dailySummary.TotalCost,
			t.config.BudgetAlertThreshold,
		)

		// You could also send notifications here (email, Slack, PagerDuty, etc.)
	}
}

// ExportRecords exports cost records to a JSON file
func (t *CostTracker) ExportRecords(filename string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(t.records); err != nil {
		return fmt.Errorf("failed to encode records: %w", err)
	}

	return nil
}

// PruneOldRecords removes records older than the specified duration
func (t *CostTracker) PruneOldRecords(maxAge time.Duration) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	newRecords := make([]CostRecord, 0)

	for _, record := range t.records {
		if record.Timestamp.After(cutoff) {
			newRecords = append(newRecords, record)
		}
	}

	pruned := len(t.records) - len(newRecords)
	t.records = newRecords

	return pruned
}

// GetPricing returns the pricing for a specific model
func (t *CostTracker) GetPricing(provider, model string) (ModelPricing, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", provider, model)
	pricing, ok := t.pricing[key]
	return pricing, ok
}

// SetPricing sets or updates the pricing for a specific model
func (t *CostTracker) SetPricing(pricing ModelPricing) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := fmt.Sprintf("%s:%s", pricing.Provider, pricing.Model)
	t.pricing[key] = pricing
}

// Global cost tracker
var globalCostTracker *CostTracker

// InitGlobalCostTracker initializes the global cost tracker
func InitGlobalCostTracker(config CostConfig) error {
	tracker, err := NewCostTracker(config)
	if err != nil {
		return err
	}
	globalCostTracker = tracker
	return nil
}

// GetCostTracker returns the global cost tracker
func GetCostTracker() *CostTracker {
	if globalCostTracker == nil {
		_ = InitGlobalCostTracker(CostConfig{
			Enabled:              false,
			BudgetAlertThreshold: 100.0,
			Currency:             "USD",
		})
	}
	return globalCostTracker
}

// Convenience functions using global cost tracker

// RecordLLMCost records LLM cost using global tracker
func RecordLLMCost(ctx context.Context, agentID, sessionID, provider, model string, promptTokens, completionTokens int) float64 {
	return GetCostTracker().RecordCost(ctx, agentID, sessionID, provider, model, promptTokens, completionTokens)
}

// GetDailyCostSummary returns daily summary using global tracker
func GetDailyCostSummary() *CostSummary {
	return GetCostTracker().GetDailySummary()
}

// GetMonthlyCostSummary returns monthly summary using global tracker
func GetMonthlyCostSummary() *CostSummary {
	return GetCostTracker().GetMonthlySummary()
}
