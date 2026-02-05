package selfimprove

import (
	"context"
	"time"
)

// Experience represents a recorded execution with its outcome.
// Experiences are the raw data used by the learning engine to improve agent behavior.
type Experience struct {
	// ID is the unique identifier for this experience
	ID string `json:"id"`

	// TraceID links to the original execution trace
	TraceID string `json:"trace_id,omitempty"`

	// AgentID identifies which agent had this experience
	AgentID string `json:"agent_id"`

	// TaskType categorizes the task for pattern matching
	TaskType string `json:"task_type"`

	// Input is the original task input
	Input interface{} `json:"input"`

	// Output is the agent's response
	Output interface{} `json:"output"`

	// SystemPrompt used for this execution
	SystemPrompt string `json:"system_prompt"`

	// UserPrompt used for this execution
	UserPrompt string `json:"user_prompt"`

	// Success indicates whether the task was successful
	Success bool `json:"success"`

	// Score is the overall evaluation score (0.0-1.0)
	Score float64 `json:"score"`

	// Subscores contains detailed scores by category
	Subscores map[string]float64 `json:"subscores,omitempty"`

	// HumanRating is the optional human-provided rating
	HumanRating *float64 `json:"human_rating,omitempty"`

	// HumanFeedback is optional textual feedback from humans
	HumanFeedback *string `json:"human_feedback,omitempty"`

	// Correction is the optional corrected output provided by a human
	Correction *string `json:"correction,omitempty"`

	// Timestamp when this experience was recorded
	Timestamp time.Time `json:"timestamp"`

	// TokensUsed for this execution
	TokensUsed int `json:"tokens_used"`

	// LatencyMs is the execution latency in milliseconds
	LatencyMs int64 `json:"latency_ms"`

	// Model used for this execution
	Model string `json:"model"`

	// ToolsUsed lists the tools that were invoked
	ToolsUsed []string `json:"tools_used,omitempty"`

	// IterationCount is the number of iterations taken
	IterationCount int `json:"iteration_count"`

	// Embedding for semantic similarity search
	Embedding []float32 `json:"embedding,omitempty"`

	// Metadata contains additional context
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// PromptVersion tracks which prompt version was used
	PromptVersion string `json:"prompt_version,omitempty"`

	// ImprovementID links to any improvement that was applied
	ImprovementID *string `json:"improvement_id,omitempty"`
}

// IsSuccessful returns true if this experience meets the success threshold.
func (e *Experience) IsSuccessful(minScore float64) bool {
	return e.Success && e.Score >= minScore
}

// IsFailed returns true if this experience is below the failure threshold.
func (e *Experience) IsFailed(maxScore float64) bool {
	return !e.Success || e.Score <= maxScore
}

// HasHumanFeedback returns true if this experience has human feedback.
func (e *Experience) HasHumanFeedback() bool {
	return e.HumanRating != nil || e.HumanFeedback != nil || e.Correction != nil
}

// ExperienceStats contains aggregated statistics about experiences.
type ExperienceStats struct {
	// TotalCount is the total number of experiences
	TotalCount int `json:"total_count"`

	// SuccessCount is the number of successful experiences
	SuccessCount int `json:"success_count"`

	// FailureCount is the number of failed experiences
	FailureCount int `json:"failure_count"`

	// AvgScore is the average score across all experiences
	AvgScore float64 `json:"avg_score"`

	// AvgLatencyMs is the average latency in milliseconds
	AvgLatencyMs float64 `json:"avg_latency_ms"`

	// AvgTokensUsed is the average tokens used per execution
	AvgTokensUsed float64 `json:"avg_tokens_used"`

	// SuccessRate is the percentage of successful experiences
	SuccessRate float64 `json:"success_rate"`

	// ScoresByTaskType maps task types to their average scores
	ScoresByTaskType map[string]float64 `json:"scores_by_task_type,omitempty"`

	// CountByTaskType maps task types to their counts
	CountByTaskType map[string]int `json:"count_by_task_type,omitempty"`

	// RecentTrend indicates if scores are improving, declining, or stable
	RecentTrend TrendDirection `json:"recent_trend"`

	// FirstExperience is the timestamp of the oldest experience
	FirstExperience *time.Time `json:"first_experience,omitempty"`

	// LastExperience is the timestamp of the most recent experience
	LastExperience *time.Time `json:"last_experience,omitempty"`
}

// TrendDirection indicates the direction of a metric trend.
type TrendDirection string

const (
	TrendImproving TrendDirection = "improving"
	TrendDeclining TrendDirection = "declining"
	TrendStable    TrendDirection = "stable"
	TrendUnknown   TrendDirection = "unknown"
)

// ExperienceQuery defines filters for querying experiences.
type ExperienceQuery struct {
	// AgentID filters by agent
	AgentID string `json:"agent_id,omitempty"`

	// TaskType filters by task type
	TaskType string `json:"task_type,omitempty"`

	// MinScore filters for minimum score
	MinScore *float64 `json:"min_score,omitempty"`

	// MaxScore filters for maximum score
	MaxScore *float64 `json:"max_score,omitempty"`

	// Success filters for success/failure
	Success *bool `json:"success,omitempty"`

	// HasHumanFeedback filters for human feedback presence
	HasHumanFeedback *bool `json:"has_human_feedback,omitempty"`

	// Since filters for experiences after this time
	Since *time.Time `json:"since,omitempty"`

	// Until filters for experiences before this time
	Until *time.Time `json:"until,omitempty"`

	// PromptVersion filters by prompt version
	PromptVersion string `json:"prompt_version,omitempty"`

	// Limit is the maximum number of results
	Limit int `json:"limit,omitempty"`

	// Offset for pagination
	Offset int `json:"offset,omitempty"`

	// OrderBy specifies the sort field
	OrderBy string `json:"order_by,omitempty"`

	// OrderDesc specifies descending order
	OrderDesc bool `json:"order_desc,omitempty"`
}

// ExperienceStore defines the interface for storing and retrieving experiences.
type ExperienceStore interface {
	// Store saves an experience
	Store(ctx context.Context, exp *Experience) error

	// Get retrieves an experience by ID
	Get(ctx context.Context, id string) (*Experience, error)

	// GetByTraceID retrieves an experience by its trace ID
	GetByTraceID(ctx context.Context, traceID string) (*Experience, error)

	// Query retrieves experiences matching the query
	Query(ctx context.Context, query *ExperienceQuery) ([]*Experience, error)

	// GetByAgent retrieves experiences for a specific agent
	GetByAgent(ctx context.Context, agentID string, limit int) ([]*Experience, error)

	// GetByTaskType retrieves experiences for a specific task type
	GetByTaskType(ctx context.Context, taskType string, limit int) ([]*Experience, error)

	// GetSuccessful retrieves successful experiences above a score threshold
	GetSuccessful(ctx context.Context, minScore float64, limit int) ([]*Experience, error)

	// GetFailed retrieves failed experiences below a score threshold
	GetFailed(ctx context.Context, maxScore float64, limit int) ([]*Experience, error)

	// FindSimilar finds experiences with similar embeddings
	FindSimilar(ctx context.Context, embedding []float32, limit int) ([]*Experience, error)

	// GetStats returns aggregated statistics for an agent
	GetStats(ctx context.Context, agentID string) (*ExperienceStats, error)

	// GetGlobalStats returns aggregated statistics across all agents
	GetGlobalStats(ctx context.Context) (*ExperienceStats, error)

	// Update updates an existing experience
	Update(ctx context.Context, exp *Experience) error

	// Delete removes an experience
	Delete(ctx context.Context, id string) error

	// Prune removes old experiences while keeping top performers
	Prune(ctx context.Context, olderThan time.Time, keepTopN int) (int, error)

	// Count returns the total number of experiences
	Count(ctx context.Context) (int, error)
}

// EmbeddingProvider generates embeddings for semantic search.
type EmbeddingProvider interface {
	// GenerateEmbedding creates an embedding vector for the given text
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)

	// GenerateBatchEmbeddings creates embeddings for multiple texts
	GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
}

// ExperienceFormatter formats experiences for use in prompts.
type ExperienceFormatter interface {
	// FormatAsFewShot formats experiences as few-shot examples
	FormatAsFewShot(experiences []*Experience) string

	// FormatAsContext formats experiences as context information
	FormatAsContext(experiences []*Experience) string

	// FormatForAnalysis formats experiences for failure analysis
	FormatForAnalysis(successes, failures []*Experience) string
}

// DefaultExperienceFormatter implements ExperienceFormatter with a standard format.
type DefaultExperienceFormatter struct {
	// MaxLength is the maximum length per formatted experience
	MaxLength int
	// IncludeMetadata determines if metadata is included
	IncludeMetadata bool
}

// NewDefaultExperienceFormatter creates a new formatter with defaults.
func NewDefaultExperienceFormatter() *DefaultExperienceFormatter {
	return &DefaultExperienceFormatter{
		MaxLength:       1000,
		IncludeMetadata: false,
	}
}

// FormatAsFewShot formats experiences as few-shot examples.
func (f *DefaultExperienceFormatter) FormatAsFewShot(experiences []*Experience) string {
	if len(experiences) == 0 {
		return ""
	}

	result := "Here are some examples of successful task completions:\n\n"
	for i, exp := range experiences {
		result += formatSingleExample(i+1, exp, f.MaxLength)
	}
	return result
}

// FormatAsContext formats experiences as context information.
func (f *DefaultExperienceFormatter) FormatAsContext(experiences []*Experience) string {
	if len(experiences) == 0 {
		return ""
	}

	result := "Relevant past experiences:\n\n"
	for _, exp := range experiences {
		result += formatExperienceContext(exp, f.MaxLength)
	}
	return result
}

// FormatForAnalysis formats experiences for failure analysis.
func (f *DefaultExperienceFormatter) FormatForAnalysis(successes, failures []*Experience) string {
	result := "=== SUCCESSFUL EXECUTIONS ===\n\n"
	for i, exp := range successes {
		result += formatAnalysisExample(i+1, exp, "SUCCESS", f.MaxLength)
	}

	result += "\n=== FAILED EXECUTIONS ===\n\n"
	for i, exp := range failures {
		result += formatAnalysisExample(i+1, exp, "FAILURE", f.MaxLength)
	}

	return result
}

func formatSingleExample(num int, exp *Experience, maxLen int) string {
	input := truncateString(formatInterface(exp.Input), maxLen/2)
	output := truncateString(formatInterface(exp.Output), maxLen/2)

	return formatString("Example %d (Score: %.2f):\nInput: %s\nOutput: %s\n\n",
		num, exp.Score, input, output)
}

func formatExperienceContext(exp *Experience, maxLen int) string {
	input := truncateString(formatInterface(exp.Input), maxLen/2)
	outcome := "succeeded"
	if !exp.Success {
		outcome = "failed"
	}

	return formatString("- Task type '%s' %s with score %.2f. Input: %s\n",
		exp.TaskType, outcome, exp.Score, input)
}

func formatAnalysisExample(num int, exp *Experience, label string, maxLen int) string {
	input := truncateString(formatInterface(exp.Input), maxLen/3)
	output := truncateString(formatInterface(exp.Output), maxLen/3)

	result := formatString("[%s #%d] Score: %.2f\n", label, num, exp.Score)
	result += formatString("Input: %s\n", input)
	result += formatString("Output: %s\n", output)

	if exp.HumanFeedback != nil {
		feedback := truncateString(*exp.HumanFeedback, maxLen/3)
		result += formatString("Feedback: %s\n", feedback)
	}

	if exp.Correction != nil {
		correction := truncateString(*exp.Correction, maxLen/3)
		result += formatString("Correction: %s\n", correction)
	}

	result += "\n"
	return result
}

func formatInterface(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return formatString("%v", v)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatString(format string, args ...interface{}) string {
	// Simple sprintf replacement to avoid import
	result := format
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			result = replaceFirst(result, "%s", v)
		case int:
			result = replaceFirst(result, "%d", intToString(v))
		case float64:
			result = replaceFirst(result, "%.2f", floatToString(v, 2))
		default:
			result = replaceFirst(result, "%v", formatDefault(v))
		}
	}
	return result
}

func replaceFirst(s, old, new string) string {
	for i := 0; i <= len(s)-len(old); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	digits := make([]byte, 0, 20)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// Reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

func floatToString(f float64, precision int) string {
	negative := f < 0
	if negative {
		f = -f
	}
	intPart := int(f)
	fracPart := f - float64(intPart)
	for i := 0; i < precision; i++ {
		fracPart *= 10
	}
	fracInt := int(fracPart + 0.5)

	result := intToString(intPart) + "."
	fracStr := intToString(fracInt)
	// Pad with zeros
	for len(fracStr) < precision {
		fracStr = "0" + fracStr
	}
	result += fracStr

	if negative {
		return "-" + result
	}
	return result
}

func formatDefault(v interface{}) string {
	if v == nil {
		return "<nil>"
	}
	if s, ok := v.(string); ok {
		return s
	}
	return "<value>"
}
