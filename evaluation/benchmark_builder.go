package evaluation

import (
	"time"

	"github.com/google/uuid"
)

// BenchmarkBuilder provides a fluent API for building benchmarks
type BenchmarkBuilder struct {
	benchmark *Benchmark
}

// NewBenchmark creates a new benchmark builder
func NewBenchmark(name string) *BenchmarkBuilder {
	return &BenchmarkBuilder{
		benchmark: &Benchmark{
			ID:        uuid.New().String(),
			Name:      name,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

// NewBenchmarkWithID creates a new benchmark builder with a specific ID
func NewBenchmarkWithID(id, name string) *BenchmarkBuilder {
	return &BenchmarkBuilder{
		benchmark: &Benchmark{
			ID:        id,
			Name:      name,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

// WithDescription sets the benchmark description
func (b *BenchmarkBuilder) WithDescription(description string) *BenchmarkBuilder {
	b.benchmark.Description = description
	return b
}

// WithTags adds tags to the benchmark
func (b *BenchmarkBuilder) WithTags(tags ...string) *BenchmarkBuilder {
	b.benchmark.Tags = append(b.benchmark.Tags, tags...)
	return b
}

// AddCase adds a test case and returns a case builder
func (b *BenchmarkBuilder) AddCase(name, input string) *CaseBuilder {
	return &CaseBuilder{
		parent: b,
		testCase: &BenchmarkCase{
			ID:    uuid.New().String(),
			Name:  name,
			Input: input,
		},
	}
}

// AddCaseWithID adds a test case with a specific ID
func (b *BenchmarkBuilder) AddCaseWithID(id, name, input string) *CaseBuilder {
	return &CaseBuilder{
		parent: b,
		testCase: &BenchmarkCase{
			ID:    id,
			Name:  name,
			Input: input,
		},
	}
}

// AddCases adds multiple simple test cases
func (b *BenchmarkBuilder) AddCases(cases map[string]string) *BenchmarkBuilder {
	for name, input := range cases {
		b.benchmark.TestCases = append(b.benchmark.TestCases, BenchmarkCase{
			ID:    uuid.New().String(),
			Name:  name,
			Input: input,
		})
	}
	return b
}

// Build returns the completed benchmark
func (b *BenchmarkBuilder) Build() *Benchmark {
	b.benchmark.UpdatedAt = time.Now()
	return b.benchmark
}

// CaseBuilder provides a fluent API for building test cases
type CaseBuilder struct {
	parent   *BenchmarkBuilder
	testCase *BenchmarkCase
}

// WithExpectedOutput sets the expected output for comparison
func (c *CaseBuilder) WithExpectedOutput(output string) *CaseBuilder {
	c.testCase.ExpectedOutput = output
	return c
}

// WithExpectedTools sets the tools expected to be used
func (c *CaseBuilder) WithExpectedTools(tools ...string) *CaseBuilder {
	c.testCase.ExpectedTools = append(c.testCase.ExpectedTools, tools...)
	return c
}

// WithMaxIterations sets the maximum allowed iterations
func (c *CaseBuilder) WithMaxIterations(max int) *CaseBuilder {
	c.testCase.MaxIterations = max
	return c
}

// WithMaxTokens sets the maximum allowed tokens
func (c *CaseBuilder) WithMaxTokens(max int) *CaseBuilder {
	c.testCase.MaxTokens = max
	return c
}

// WithMaxCost sets the maximum allowed cost
func (c *CaseBuilder) WithMaxCost(max float64) *CaseBuilder {
	c.testCase.MaxCost = max
	return c
}

// WithTimeout sets the timeout in seconds
func (c *CaseBuilder) WithTimeout(seconds int) *CaseBuilder {
	c.testCase.TimeoutSeconds = seconds
	return c
}

// WithTimeoutDuration sets the timeout as a duration
func (c *CaseBuilder) WithTimeoutDuration(d time.Duration) *CaseBuilder {
	c.testCase.TimeoutSeconds = int(d.Seconds())
	return c
}

// WithTags adds tags to the test case
func (c *CaseBuilder) WithTags(tags ...string) *CaseBuilder {
	c.testCase.Tags = append(c.testCase.Tags, tags...)
	return c
}

// WithWeight sets the weight for scoring
func (c *CaseBuilder) WithWeight(weight float64) *CaseBuilder {
	c.testCase.Weight = weight
	return c
}

// WithPassCriteria sets custom pass criteria
func (c *CaseBuilder) WithPassCriteria(criteria *PassCriteria) *CaseBuilder {
	c.testCase.PassCriteria = criteria
	return c
}

// RequireCompletion adds completion requirement to pass criteria
func (c *CaseBuilder) RequireCompletion() *CaseBuilder {
	c.ensurePassCriteria()
	c.testCase.PassCriteria.RequireCompletion = true
	return c
}

// RequireNoErrors adds no-errors requirement to pass criteria
func (c *CaseBuilder) RequireNoErrors() *CaseBuilder {
	c.ensurePassCriteria()
	c.testCase.PassCriteria.RequireNoErrors = true
	return c
}

// RequireMinScore sets minimum score requirement
func (c *CaseBuilder) RequireMinScore(score float64) *CaseBuilder {
	c.ensurePassCriteria()
	c.testCase.PassCriteria.MinScore = score
	return c
}

// RequireMaxDuration sets maximum duration requirement
func (c *CaseBuilder) RequireMaxDuration(ms int64) *CaseBuilder {
	c.ensurePassCriteria()
	c.testCase.PassCriteria.MaxDurationMs = ms
	return c
}

// RequireMaxTokens sets maximum tokens requirement
func (c *CaseBuilder) RequireMaxTokens(tokens int) *CaseBuilder {
	c.ensurePassCriteria()
	c.testCase.PassCriteria.MaxTokens = tokens
	return c
}

// RequireMaxCost sets maximum cost requirement
func (c *CaseBuilder) RequireMaxCost(cost float64) *CaseBuilder {
	c.ensurePassCriteria()
	c.testCase.PassCriteria.MaxCost = cost
	return c
}

// RequireTools sets required tools
func (c *CaseBuilder) RequireTools(tools ...string) *CaseBuilder {
	c.ensurePassCriteria()
	c.testCase.PassCriteria.RequiredTools = append(c.testCase.PassCriteria.RequiredTools, tools...)
	return c
}

// ensurePassCriteria ensures pass criteria is initialized
func (c *CaseBuilder) ensurePassCriteria() {
	if c.testCase.PassCriteria == nil {
		c.testCase.PassCriteria = &PassCriteria{}
	}
}

// Done finishes the case and returns to the benchmark builder
func (c *CaseBuilder) Done() *BenchmarkBuilder {
	c.parent.benchmark.TestCases = append(c.parent.benchmark.TestCases, *c.testCase)
	return c.parent
}

// And is an alias for Done that reads more naturally in chains
func (c *CaseBuilder) And() *BenchmarkBuilder {
	return c.Done()
}

// PassCriteriaBuilder provides a fluent API for building pass criteria
type PassCriteriaBuilder struct {
	criteria *PassCriteria
}

// NewPassCriteria creates a new pass criteria builder
func NewPassCriteria() *PassCriteriaBuilder {
	return &PassCriteriaBuilder{
		criteria: &PassCriteria{},
	}
}

// WithMinScore sets minimum score
func (p *PassCriteriaBuilder) WithMinScore(score float64) *PassCriteriaBuilder {
	p.criteria.MinScore = score
	return p
}

// WithRequireCompletion sets completion requirement
func (p *PassCriteriaBuilder) WithRequireCompletion(require bool) *PassCriteriaBuilder {
	p.criteria.RequireCompletion = require
	return p
}

// WithRequireNoErrors sets no-errors requirement
func (p *PassCriteriaBuilder) WithRequireNoErrors(require bool) *PassCriteriaBuilder {
	p.criteria.RequireNoErrors = require
	return p
}

// WithMaxTokens sets maximum tokens
func (p *PassCriteriaBuilder) WithMaxTokens(max int) *PassCriteriaBuilder {
	p.criteria.MaxTokens = max
	return p
}

// WithMaxCost sets maximum cost
func (p *PassCriteriaBuilder) WithMaxCost(max float64) *PassCriteriaBuilder {
	p.criteria.MaxCost = max
	return p
}

// WithMaxDuration sets maximum duration in milliseconds
func (p *PassCriteriaBuilder) WithMaxDuration(ms int64) *PassCriteriaBuilder {
	p.criteria.MaxDurationMs = ms
	return p
}

// WithRequiredTools sets required tools
func (p *PassCriteriaBuilder) WithRequiredTools(tools ...string) *PassCriteriaBuilder {
	p.criteria.RequiredTools = tools
	return p
}

// WithCustomCriteria sets custom criteria
func (p *PassCriteriaBuilder) WithCustomCriteria(key string, value interface{}) *PassCriteriaBuilder {
	if p.criteria.CustomCriteria == nil {
		p.criteria.CustomCriteria = make(map[string]interface{})
	}
	p.criteria.CustomCriteria[key] = value
	return p
}

// Build returns the completed pass criteria
func (p *PassCriteriaBuilder) Build() *PassCriteria {
	return p.criteria
}

// QuickBenchmark creates a simple benchmark from input/output pairs
func QuickBenchmark(name string, cases map[string]string) *Benchmark {
	builder := NewBenchmark(name)
	for input, expected := range cases {
		builder.AddCase(input, input).
			WithExpectedOutput(expected).
			RequireCompletion().
			Done()
	}
	return builder.Build()
}

// MathBenchmark creates a benchmark for math operations
func MathBenchmark() *Benchmark {
	return NewBenchmark("Math Operations").
		WithDescription("Tests basic math capabilities").
		WithTags("math", "basic").
		AddCase("simple-addition", "What is 2 + 2?").
			WithExpectedOutput("4").
			WithMaxIterations(3).
			RequireCompletion().
			Done().
		AddCase("multiplication", "What is 7 * 8?").
			WithExpectedOutput("56").
			WithMaxIterations(3).
			RequireCompletion().
			Done().
		AddCase("complex-math", "Calculate (15 + 27) * 3 - 50").
			WithExpectedOutput("76").
			WithMaxIterations(5).
			RequireCompletion().
			Done().
		Build()
}

// ToolUsageBenchmark creates a benchmark for tool usage
func ToolUsageBenchmark() *Benchmark {
	return NewBenchmark("Tool Usage").
		WithDescription("Tests effective tool usage").
		WithTags("tools", "integration").
		AddCase("calculator-use", "Use the calculator to compute 123 * 456").
			WithExpectedTools("calculator").
			WithMaxIterations(3).
			RequireCompletion().
			RequireTools("calculator").
			Done().
		AddCase("search-use", "Search for the current weather in New York").
			WithExpectedTools("search").
			WithMaxIterations(5).
			RequireCompletion().
			Done().
		Build()
}
