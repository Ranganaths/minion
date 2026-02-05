package selfimprove

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// ABTestManager manages A/B tests for improvement proposals.
type ABTestManager struct {
	mu sync.RWMutex

	// Active tests
	activeTests map[string]*ABTest

	// Completed tests
	completedTests map[string]*ABTest

	// Configuration
	config *ABTestConfig

	// Random source for assignment
	rng *rand.Rand
}

// ABTestConfig configures A/B testing behavior.
type ABTestConfig struct {
	// MinSamples is the minimum samples needed per variant
	MinSamples int `json:"min_samples"`

	// MaxSamples is the maximum samples before forced conclusion
	MaxSamples int `json:"max_samples"`

	// ConfidenceLevel required for statistical significance (e.g., 0.95)
	ConfidenceLevel float64 `json:"confidence_level"`

	// TrafficSplit is the percentage of traffic for treatment (0-1)
	TrafficSplit float64 `json:"traffic_split"`

	// MaxDuration is the maximum test duration before forced conclusion
	MaxDuration time.Duration `json:"max_duration"`

	// MinEffectSize is the minimum detectable effect (e.g., 0.05 for 5%)
	MinEffectSize float64 `json:"min_effect_size"`
}

// ABTest represents an active A/B test.
type ABTest struct {
	ID           string         `json:"id"`
	ProposalID   string         `json:"proposal_id"`
	AgentID      string         `json:"agent_id"`
	Status       ABTestStatus   `json:"status"`
	StartedAt    time.Time      `json:"started_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`

	// Variants
	Control   *ABTestVariant `json:"control"`
	Treatment *ABTestVariant `json:"treatment"`

	// Configuration used for this test
	Config *ABTestConfig `json:"config"`

	// Result
	Winner     string  `json:"winner,omitempty"` // "control", "treatment", "inconclusive"
	PValue     float64 `json:"p_value,omitempty"`
	EffectSize float64 `json:"effect_size,omitempty"`
}

// ABTestStatus represents the status of an A/B test.
type ABTestStatus string

const (
	ABTestStatusRunning     ABTestStatus = "running"
	ABTestStatusCompleted   ABTestStatus = "completed"
	ABTestStatusCancelled   ABTestStatus = "cancelled"
)

// ABTestVariant represents one variant in an A/B test.
type ABTestVariant struct {
	Name        string    `json:"name"`
	Value       string    `json:"value"`      // The prompt or config being tested
	Samples     int       `json:"samples"`
	TotalScore  float64   `json:"total_score"`
	Scores      []float64 `json:"-"`          // Individual scores for variance calculation
	Successes   int       `json:"successes"`
}

// NewABTestManager creates a new A/B test manager.
func NewABTestManager(config *ABTestConfig) *ABTestManager {
	if config == nil {
		config = DefaultABTestConfig()
	}

	return &ABTestManager{
		activeTests:    make(map[string]*ABTest),
		completedTests: make(map[string]*ABTest),
		config:         config,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// DefaultABTestConfig returns default A/B test configuration.
func DefaultABTestConfig() *ABTestConfig {
	return &ABTestConfig{
		MinSamples:      50,
		MaxSamples:      500,
		ConfidenceLevel: 0.95,
		TrafficSplit:    0.5,
		MaxDuration:     7 * 24 * time.Hour, // 1 week
		MinEffectSize:   0.05,
	}
}

// StartTest starts a new A/B test for a proposal.
func (m *ABTestManager) StartTest(proposal *ImprovementProposal) (*ABTest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	testID := "abtest-" + proposal.ID

	test := &ABTest{
		ID:         testID,
		ProposalID: proposal.ID,
		AgentID:    proposal.AgentID,
		Status:     ABTestStatusRunning,
		StartedAt:  time.Now(),
		Control: &ABTestVariant{
			Name:   "control",
			Value:  proposal.CurrentValue,
			Scores: make([]float64, 0),
		},
		Treatment: &ABTestVariant{
			Name:   "treatment",
			Value:  proposal.ProposedValue,
			Scores: make([]float64, 0),
		},
		Config: m.config,
	}

	m.activeTests[testID] = test
	return test, nil
}

// AssignVariant assigns a variant for a new execution.
func (m *ABTestManager) AssignVariant(testID string) (variant string, value string, err error) {
	m.mu.RLock()
	test, ok := m.activeTests[testID]
	m.mu.RUnlock()

	if !ok {
		return "", "", nil // No active test
	}

	// Random assignment based on traffic split
	if m.rng.Float64() < m.config.TrafficSplit {
		return "treatment", test.Treatment.Value, nil
	}
	return "control", test.Control.Value, nil
}

// RecordResult records a result for an A/B test.
func (m *ABTestManager) RecordResult(testID string, variant string, score float64, success bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	test, ok := m.activeTests[testID]
	if !ok {
		return nil // Test not found or not active
	}

	var v *ABTestVariant
	switch variant {
	case "control":
		v = test.Control
	case "treatment":
		v = test.Treatment
	default:
		return nil
	}

	v.Samples++
	v.TotalScore += score
	v.Scores = append(v.Scores, score)
	if success {
		v.Successes++
	}

	// Check if test should conclude
	if m.shouldConclude(test) {
		m.concludeTestLocked(test)
	}

	return nil
}

// GetActiveTest returns an active test for an agent.
func (m *ABTestManager) GetActiveTest(agentID string) *ABTest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, test := range m.activeTests {
		if test.AgentID == agentID && test.Status == ABTestStatusRunning {
			return test
		}
	}
	return nil
}

// GetTest returns a test by ID.
func (m *ABTestManager) GetTest(testID string) *ABTest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if test, ok := m.activeTests[testID]; ok {
		return test
	}
	if test, ok := m.completedTests[testID]; ok {
		return test
	}
	return nil
}

// CancelTest cancels an active test.
func (m *ABTestManager) CancelTest(testID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	test, ok := m.activeTests[testID]
	if !ok {
		return nil
	}

	test.Status = ABTestStatusCancelled
	now := time.Now()
	test.CompletedAt = &now
	test.Winner = "cancelled"

	delete(m.activeTests, testID)
	m.completedTests[testID] = test

	return nil
}

// shouldConclude checks if a test should be concluded.
func (m *ABTestManager) shouldConclude(test *ABTest) bool {
	// Check minimum samples
	if test.Control.Samples < test.Config.MinSamples ||
		test.Treatment.Samples < test.Config.MinSamples {
		return false
	}

	// Check maximum samples
	if test.Control.Samples >= test.Config.MaxSamples ||
		test.Treatment.Samples >= test.Config.MaxSamples {
		return true
	}

	// Check maximum duration
	if time.Since(test.StartedAt) >= test.Config.MaxDuration {
		return true
	}

	// Check statistical significance
	pValue := m.calculatePValue(test)
	if pValue < (1 - test.Config.ConfidenceLevel) {
		return true
	}

	return false
}

// concludeTestLocked concludes a test (must hold lock).
func (m *ABTestManager) concludeTestLocked(test *ABTest) {
	now := time.Now()
	test.CompletedAt = &now
	test.Status = ABTestStatusCompleted

	// Calculate statistics
	test.PValue = m.calculatePValue(test)
	test.EffectSize = m.calculateEffectSize(test)

	// Determine winner
	test.Winner = m.determineWinner(test)

	// Move to completed
	delete(m.activeTests, test.ID)
	m.completedTests[test.ID] = test
}

// calculatePValue calculates the p-value using Welch's t-test.
func (m *ABTestManager) calculatePValue(test *ABTest) float64 {
	n1 := float64(test.Control.Samples)
	n2 := float64(test.Treatment.Samples)

	if n1 < 2 || n2 < 2 {
		return 1.0
	}

	mean1 := test.Control.TotalScore / n1
	mean2 := test.Treatment.TotalScore / n2

	var1 := m.calculateVariance(test.Control.Scores, mean1)
	var2 := m.calculateVariance(test.Treatment.Scores, mean2)

	// Welch's t-test
	se := math.Sqrt(var1/n1 + var2/n2)
	if se == 0 {
		return 1.0
	}

	t := (mean2 - mean1) / se

	// Degrees of freedom (Welch-Satterthwaite)
	num := math.Pow(var1/n1+var2/n2, 2)
	denom := math.Pow(var1/n1, 2)/(n1-1) + math.Pow(var2/n2, 2)/(n2-1)
	df := num / denom

	// Convert t to p-value (two-tailed)
	// Using approximation for simplicity
	pValue := 2 * (1 - m.tCDF(math.Abs(t), df))

	return pValue
}

// calculateVariance calculates sample variance.
func (m *ABTestManager) calculateVariance(scores []float64, mean float64) float64 {
	if len(scores) < 2 {
		return 0
	}

	var sum float64
	for _, s := range scores {
		diff := s - mean
		sum += diff * diff
	}
	return sum / float64(len(scores)-1)
}

// tCDF approximates the cumulative distribution function of t-distribution.
func (m *ABTestManager) tCDF(t, df float64) float64 {
	// Use normal approximation for large df
	if df > 30 {
		return m.normalCDF(t)
	}

	// Simplified approximation for smaller df
	x := df / (df + t*t)
	return 1 - 0.5*m.betaIncomplete(df/2, 0.5, x)
}

// normalCDF approximates the standard normal CDF.
func (m *ABTestManager) normalCDF(x float64) float64 {
	// Approximation using error function
	return 0.5 * (1 + m.erf(x/math.Sqrt(2)))
}

// erf approximates the error function.
func (m *ABTestManager) erf(x float64) float64 {
	// Horner form coefficients
	a1 := 0.254829592
	a2 := -0.284496736
	a3 := 1.421413741
	a4 := -1.453152027
	a5 := 1.061405429
	p := 0.3275911

	sign := 1.0
	if x < 0 {
		sign = -1.0
	}
	x = math.Abs(x)

	t := 1.0 / (1.0 + p*x)
	y := 1.0 - (((((a5*t+a4)*t)+a3)*t+a2)*t+a1)*t*math.Exp(-x*x)

	return sign * y
}

// betaIncomplete approximates the incomplete beta function.
func (m *ABTestManager) betaIncomplete(a, b, x float64) float64 {
	// Simplified approximation
	if x == 0 || x == 1 {
		return x
	}

	// Use continued fraction approximation
	const maxIterations = 100
	const epsilon = 1e-10

	bt := math.Exp(math.Log(x)*a + math.Log(1-x)*b)

	// Use symmetry for better convergence
	if x > (a+1)/(a+b+2) {
		return 1 - m.betaIncomplete(b, a, 1-x)
	}

	// Continued fraction
	c := 1.0
	d := 1.0 / (1 - (a+b)*x/(a+1))
	h := d

	for i := 1; i <= maxIterations; i++ {
		m := float64(i)
		numerator := m * (b - m) * x / ((a + 2*m - 1) * (a + 2*m))
		d = 1 / (1 + numerator*d)
		c = 1 + numerator/c
		h *= d * c

		numerator = -(a + m) * (a + b + m) * x / ((a + 2*m) * (a + 2*m + 1))
		d = 1 / (1 + numerator*d)
		c = 1 + numerator/c
		delta := d * c
		h *= delta

		if math.Abs(delta-1) < epsilon {
			break
		}
	}

	return bt * h / a
}

// calculateEffectSize calculates Cohen's d effect size.
func (m *ABTestManager) calculateEffectSize(test *ABTest) float64 {
	n1 := float64(test.Control.Samples)
	n2 := float64(test.Treatment.Samples)

	if n1 < 2 || n2 < 2 {
		return 0
	}

	mean1 := test.Control.TotalScore / n1
	mean2 := test.Treatment.TotalScore / n2

	var1 := m.calculateVariance(test.Control.Scores, mean1)
	var2 := m.calculateVariance(test.Treatment.Scores, mean2)

	// Pooled standard deviation
	pooledSD := math.Sqrt(((n1-1)*var1 + (n2-1)*var2) / (n1 + n2 - 2))

	if pooledSD == 0 {
		return 0
	}

	return (mean2 - mean1) / pooledSD
}

// determineWinner determines the winner of the test.
func (m *ABTestManager) determineWinner(test *ABTest) string {
	// Check for statistical significance
	if test.PValue >= (1 - test.Config.ConfidenceLevel) {
		return "inconclusive"
	}

	// Check minimum effect size
	if math.Abs(test.EffectSize) < test.Config.MinEffectSize {
		return "inconclusive"
	}

	// Compare means
	controlMean := test.Control.TotalScore / float64(test.Control.Samples)
	treatmentMean := test.Treatment.TotalScore / float64(test.Treatment.Samples)

	if treatmentMean > controlMean {
		return "treatment"
	}
	return "control"
}

// GetResults returns the results of a completed test.
func (m *ABTestManager) GetResults(testID string) *ABTestResults {
	test := m.GetTest(testID)
	if test == nil {
		return nil
	}

	controlMean := 0.0
	treatmentMean := 0.0
	if test.Control.Samples > 0 {
		controlMean = test.Control.TotalScore / float64(test.Control.Samples)
	}
	if test.Treatment.Samples > 0 {
		treatmentMean = test.Treatment.TotalScore / float64(test.Treatment.Samples)
	}

	return &ABTestResults{
		ControlSamples:    test.Control.Samples,
		ControlAvgScore:   controlMean,
		TreatmentSamples:  test.Treatment.Samples,
		TreatmentAvgScore: treatmentMean,
		Improvement:       treatmentMean - controlMean,
		PValue:            test.PValue,
		Significant:       test.PValue < (1 - test.Config.ConfidenceLevel),
		ConfidenceLevel:   test.Config.ConfidenceLevel,
		Winner:            test.Winner,
		StartedAt:         test.StartedAt,
		CompletedAt:       test.CompletedAt,
	}
}

// ListActiveTests returns all active tests.
func (m *ABTestManager) ListActiveTests() []*ABTest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tests := make([]*ABTest, 0, len(m.activeTests))
	for _, test := range m.activeTests {
		tests = append(tests, test)
	}
	return tests
}

// ListCompletedTests returns completed tests.
func (m *ABTestManager) ListCompletedTests(limit int) []*ABTest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tests := make([]*ABTest, 0, len(m.completedTests))
	for _, test := range m.completedTests {
		tests = append(tests, test)
		if limit > 0 && len(tests) >= limit {
			break
		}
	}
	return tests
}
