package prompts

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ABTest represents an A/B test between prompt variants
type ABTest struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	Status       ABTestStatus           `json:"status"`
	Variants     []ABTestVariant        `json:"variants"`
	TargetMetric string                 `json:"target_metric"` // e.g., "success_rate", "user_rating"
	MinSamples   int                    `json:"min_samples"`   // Minimum samples before declaring winner
	Confidence   float64                `json:"confidence"`    // Required confidence level (e.g., 0.95)
	StartTime    time.Time              `json:"start_time"`
	EndTime      *time.Time             `json:"end_time,omitempty"`
	Winner       string                 `json:"winner,omitempty"` // Winning variant ID
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	CreatedBy    string                 `json:"created_by,omitempty"`
}

// ABTestStatus represents the status of an A/B test
type ABTestStatus string

const (
	ABTestStatusDraft     ABTestStatus = "draft"
	ABTestStatusRunning   ABTestStatus = "running"
	ABTestStatusPaused    ABTestStatus = "paused"
	ABTestStatusCompleted ABTestStatus = "completed"
	ABTestStatusCancelled ABTestStatus = "cancelled"
)

// ABTestVariant represents a variant in an A/B test
type ABTestVariant struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	PromptID      string                 `json:"prompt_id"`
	Weight        float64                `json:"weight"` // Traffic allocation weight (0-1)
	Impressions   int64                  `json:"impressions"`
	Conversions   int64                  `json:"conversions"`
	TotalRating   float64                `json:"total_rating"`
	SumSquaredDev float64                `json:"sum_squared_dev"` // For variance calculation
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ABTestResult records a result for an A/B test
type ABTestResult struct {
	ID          string                 `json:"id"`
	TestID      string                 `json:"test_id"`
	VariantID   string                 `json:"variant_id"`
	UserID      string                 `json:"user_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	Converted   bool                   `json:"converted"`
	Rating      *float64               `json:"rating,omitempty"`
	Latency     time.Duration          `json:"latency,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	RecordedAt  time.Time              `json:"recorded_at"`
}

// ABTestAnalysis contains statistical analysis of test results
type ABTestAnalysis struct {
	TestID             string                    `json:"test_id"`
	VariantStats       map[string]*VariantStats  `json:"variant_stats"`
	WinningVariant     string                    `json:"winning_variant,omitempty"`
	Confidence         float64                   `json:"confidence"`
	SignificantResult  bool                      `json:"significant_result"`
	RecommendedAction  string                    `json:"recommended_action"`
	AnalyzedAt         time.Time                 `json:"analyzed_at"`
}

// VariantStats contains statistics for a variant
type VariantStats struct {
	VariantID       string  `json:"variant_id"`
	Impressions     int64   `json:"impressions"`
	Conversions     int64   `json:"conversions"`
	ConversionRate  float64 `json:"conversion_rate"`
	AverageRating   float64 `json:"average_rating,omitempty"`
	Variance        float64 `json:"variance,omitempty"`
	StandardError   float64 `json:"standard_error,omitempty"`
	ConfidenceMin   float64 `json:"confidence_min,omitempty"`
	ConfidenceMax   float64 `json:"confidence_max,omitempty"`
	Improvement     float64 `json:"improvement,omitempty"` // vs control
}

// ABTestManager manages A/B tests
type ABTestManager struct {
	tests         map[string]*ABTest
	results       map[string][]*ABTestResult
	promptStore   PromptStore
	templateEngine *TemplateEngine
	rng           *rand.Rand
	mu            sync.RWMutex
}

// ABTestManagerConfig configures the A/B test manager
type ABTestManagerConfig struct {
	PromptStore    PromptStore
	TemplateEngine *TemplateEngine
}

// NewABTestManager creates a new A/B test manager
func NewABTestManager(config ABTestManagerConfig) *ABTestManager {
	return &ABTestManager{
		tests:          make(map[string]*ABTest),
		results:        make(map[string][]*ABTestResult),
		promptStore:    config.PromptStore,
		templateEngine: config.TemplateEngine,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// CreateTest creates a new A/B test
func (m *ABTestManager) CreateTest(ctx context.Context, test *ABTest) error {
	if test.ID == "" {
		test.ID = uuid.New().String()
	}
	if test.CreatedAt.IsZero() {
		test.CreatedAt = time.Now()
	}
	if test.Status == "" {
		test.Status = ABTestStatusDraft
	}
	if test.Confidence == 0 {
		test.Confidence = 0.95
	}
	if test.MinSamples == 0 {
		test.MinSamples = 100
	}

	// Validate variants
	if len(test.Variants) < 2 {
		return errors.New("at least 2 variants are required")
	}

	// Assign IDs to variants if missing
	for i := range test.Variants {
		if test.Variants[i].ID == "" {
			test.Variants[i].ID = uuid.New().String()
		}
	}

	// Normalize weights
	totalWeight := 0.0
	for _, v := range test.Variants {
		totalWeight += v.Weight
	}
	if totalWeight == 0 {
		// Equal weights if not specified
		for i := range test.Variants {
			test.Variants[i].Weight = 1.0 / float64(len(test.Variants))
		}
	} else {
		for i := range test.Variants {
			test.Variants[i].Weight /= totalWeight
		}
	}

	m.mu.Lock()
	m.tests[test.ID] = test
	m.mu.Unlock()

	return nil
}

// StartTest starts an A/B test
func (m *ABTestManager) StartTest(ctx context.Context, testID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	test, ok := m.tests[testID]
	if !ok {
		return errors.New("test not found")
	}

	test.Status = ABTestStatusRunning
	test.StartTime = time.Now()

	return nil
}

// PauseTest pauses an A/B test
func (m *ABTestManager) PauseTest(ctx context.Context, testID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	test, ok := m.tests[testID]
	if !ok {
		return errors.New("test not found")
	}

	test.Status = ABTestStatusPaused
	return nil
}

// EndTest ends an A/B test and determines the winner
func (m *ABTestManager) EndTest(ctx context.Context, testID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	test, ok := m.tests[testID]
	if !ok {
		return errors.New("test not found")
	}

	now := time.Now()
	test.EndTime = &now
	test.Status = ABTestStatusCompleted

	// Determine winner
	var bestVariant string
	var bestMetric float64

	for _, v := range test.Variants {
		metric := m.calculateMetric(v, test.TargetMetric)
		if metric > bestMetric {
			bestMetric = metric
			bestVariant = v.ID
		}
	}

	test.Winner = bestVariant
	return nil
}

// SelectVariant selects a variant for a user/session based on weights
func (m *ABTestManager) SelectVariant(ctx context.Context, testID string, userID string) (*ABTestVariant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	test, ok := m.tests[testID]
	if !ok {
		return nil, errors.New("test not found")
	}

	if test.Status != ABTestStatusRunning {
		return nil, errors.New("test is not running")
	}

	// Deterministic selection based on user ID for consistency
	// or random selection if no user ID
	var selected *ABTestVariant
	if userID != "" {
		// Hash user ID for deterministic assignment
		hash := hashString(userID + testID)
		point := float64(hash%10000) / 10000.0

		cumulative := 0.0
		for i := range test.Variants {
			cumulative += test.Variants[i].Weight
			if point < cumulative {
				selected = &test.Variants[i]
				break
			}
		}
	}

	if selected == nil {
		// Random selection based on weights
		point := m.rng.Float64()
		cumulative := 0.0
		for i := range test.Variants {
			cumulative += test.Variants[i].Weight
			if point < cumulative {
				selected = &test.Variants[i]
				break
			}
		}
	}

	if selected == nil {
		selected = &test.Variants[0]
	}

	// Record impression
	for i := range test.Variants {
		if test.Variants[i].ID == selected.ID {
			test.Variants[i].Impressions++
			break
		}
	}

	return selected, nil
}

// RecordResult records a result for an A/B test
func (m *ABTestManager) RecordResult(ctx context.Context, result *ABTestResult) error {
	if result.ID == "" {
		result.ID = uuid.New().String()
	}
	if result.RecordedAt.IsZero() {
		result.RecordedAt = time.Now()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	test, ok := m.tests[result.TestID]
	if !ok {
		return errors.New("test not found")
	}

	// Update variant stats
	for i := range test.Variants {
		if test.Variants[i].ID == result.VariantID {
			test.Variants[i].Impressions++
			if result.Converted {
				test.Variants[i].Conversions++
			}
			if result.Rating != nil {
				oldMean := test.Variants[i].TotalRating / float64(max(test.Variants[i].Impressions, 1))
				test.Variants[i].TotalRating += *result.Rating
				newMean := test.Variants[i].TotalRating / float64(test.Variants[i].Impressions)
				// Welford's online algorithm for variance
				test.Variants[i].SumSquaredDev += (*result.Rating - oldMean) * (*result.Rating - newMean)
			}
			break
		}
	}

	// Store result
	m.results[result.TestID] = append(m.results[result.TestID], result)

	return nil
}

// GetTest retrieves an A/B test
func (m *ABTestManager) GetTest(ctx context.Context, testID string) (*ABTest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	test, ok := m.tests[testID]
	if !ok {
		return nil, errors.New("test not found")
	}
	return test, nil
}

// ListTests lists A/B tests
func (m *ABTestManager) ListTests(ctx context.Context, status ABTestStatus) ([]*ABTest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]*ABTest, 0)
	for _, test := range m.tests {
		if status == "" || test.Status == status {
			results = append(results, test)
		}
	}
	return results, nil
}

// AnalyzeTest performs statistical analysis on a test
func (m *ABTestManager) AnalyzeTest(ctx context.Context, testID string) (*ABTestAnalysis, error) {
	m.mu.RLock()
	test, ok := m.tests[testID]
	if !ok {
		m.mu.RUnlock()
		return nil, errors.New("test not found")
	}
	m.mu.RUnlock()

	analysis := &ABTestAnalysis{
		TestID:       testID,
		VariantStats: make(map[string]*VariantStats),
		Confidence:   test.Confidence,
		AnalyzedAt:   time.Now(),
	}

	// Calculate stats for each variant
	var controlStats *VariantStats
	var bestStats *VariantStats

	for _, v := range test.Variants {
		stats := &VariantStats{
			VariantID:   v.ID,
			Impressions: v.Impressions,
			Conversions: v.Conversions,
		}

		if v.Impressions > 0 {
			stats.ConversionRate = float64(v.Conversions) / float64(v.Impressions)
			stats.AverageRating = v.TotalRating / float64(v.Impressions)

			// Calculate variance and standard error
			if v.Impressions > 1 {
				stats.Variance = v.SumSquaredDev / float64(v.Impressions-1)
				stats.StandardError = sqrt(stats.Variance / float64(v.Impressions))

				// 95% confidence interval (using z=1.96)
				margin := 1.96 * stats.StandardError
				stats.ConfidenceMin = stats.ConversionRate - margin
				stats.ConfidenceMax = stats.ConversionRate + margin
			}
		}

		analysis.VariantStats[v.ID] = stats

		// First variant is considered control
		if controlStats == nil {
			controlStats = stats
		} else {
			// Calculate improvement vs control
			if controlStats.ConversionRate > 0 {
				stats.Improvement = (stats.ConversionRate - controlStats.ConversionRate) / controlStats.ConversionRate
			}
		}

		if bestStats == nil || m.calculateMetric(v, test.TargetMetric) > m.calculateMetric(findVariant(test, bestStats.VariantID), test.TargetMetric) {
			bestStats = stats
		}
	}

	// Determine significance
	if bestStats != nil && controlStats != nil && bestStats.VariantID != controlStats.VariantID {
		// Check if confidence intervals don't overlap
		if bestStats.ConfidenceMin > controlStats.ConfidenceMax {
			analysis.SignificantResult = true
			analysis.WinningVariant = bestStats.VariantID
		}
	}

	// Check minimum samples
	minReached := true
	for _, v := range test.Variants {
		if v.Impressions < int64(test.MinSamples) {
			minReached = false
			break
		}
	}

	// Generate recommendation
	if !minReached {
		analysis.RecommendedAction = "Continue test - minimum samples not reached"
	} else if analysis.SignificantResult {
		analysis.RecommendedAction = "Implement winning variant: " + analysis.WinningVariant
	} else {
		analysis.RecommendedAction = "Continue test - no significant difference yet"
	}

	return analysis, nil
}

// calculateMetric calculates the metric value for a variant
func (m *ABTestManager) calculateMetric(v ABTestVariant, metric string) float64 {
	if v.Impressions == 0 {
		return 0
	}

	switch metric {
	case "success_rate", "conversion_rate":
		return float64(v.Conversions) / float64(v.Impressions)
	case "user_rating", "rating":
		return v.TotalRating / float64(v.Impressions)
	default:
		return float64(v.Conversions) / float64(v.Impressions)
	}
}

// GetPromptForTest gets the rendered prompt for a test variant
func (m *ABTestManager) GetPromptForTest(ctx context.Context, testID string, userID string, variables map[string]interface{}) (string, *ABTestVariant, error) {
	variant, err := m.SelectVariant(ctx, testID, userID)
	if err != nil {
		return "", nil, err
	}

	// Get the prompt template
	prompt, err := m.promptStore.Get(ctx, variant.PromptID)
	if err != nil {
		return "", nil, err
	}

	// Render the prompt
	rendered, err := m.templateEngine.Render(prompt, variables)
	if err != nil {
		return "", nil, err
	}

	return rendered, variant, nil
}

// Helper functions

func hashString(s string) uint32 {
	var hash uint32 = 5381
	for i := 0; i < len(s); i++ {
		hash = ((hash << 5) + hash) + uint32(s[i])
	}
	return hash
}

func findVariant(test *ABTest, variantID string) ABTestVariant {
	for _, v := range test.Variants {
		if v.ID == variantID {
			return v
		}
	}
	return ABTestVariant{}
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 20; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
