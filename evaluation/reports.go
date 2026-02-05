package evaluation

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// ReportGenerator generates evaluation reports
type ReportGenerator struct {
	store EvaluationStore
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(store EvaluationStore) *ReportGenerator {
	return &ReportGenerator{store: store}
}

// AgentReport provides a comprehensive report for an agent
type AgentReport struct {
	// AgentID identifies the agent
	AgentID string `json:"agent_id"`

	// Period is the time period covered
	Period TimePeriod `json:"period"`

	// GeneratedAt is when the report was generated
	GeneratedAt time.Time `json:"generated_at"`

	// Summary is the evaluation summary
	Summary *EvaluationSummary `json:"summary"`

	// Trends shows performance over time
	Trends []TrendPoint `json:"trends,omitempty"`

	// TopIssues are the most common issues
	TopIssues []Issue `json:"top_issues,omitempty"`

	// Recommendations are improvement suggestions
	Recommendations []string `json:"recommendations,omitempty"`

	// BenchmarkResults are recent benchmark results
	BenchmarkResults []*BenchmarkSummary `json:"benchmark_results,omitempty"`

	// ScoreBreakdown shows scores by evaluation type
	ScoreBreakdown map[EvaluationType]ScoreDetail `json:"score_breakdown,omitempty"`
}

// TrendPoint represents a point in a time series
type TrendPoint struct {
	// Timestamp is the time for this data point
	Timestamp time.Time `json:"timestamp"`

	// Score is the average score at this time
	Score float64 `json:"score"`

	// Evaluations is the number of evaluations
	Evaluations int `json:"evaluations"`

	// TaskCompletionRate at this time
	TaskCompletionRate float64 `json:"task_completion_rate"`

	// AvgCost at this time
	AvgCost float64 `json:"avg_cost"`

	// AvgDurationMs at this time
	AvgDurationMs float64 `json:"avg_duration_ms"`
}

// Issue represents a detected issue
type Issue struct {
	// Type categorizes the issue
	Type string `json:"type"`

	// Description describes the issue
	Description string `json:"description"`

	// Severity is the issue severity (low, medium, high)
	Severity string `json:"severity"`

	// Occurrences is how often this issue occurred
	Occurrences int `json:"occurrences"`

	// Recommendation is how to fix the issue
	Recommendation string `json:"recommendation,omitempty"`
}

// ScoreDetail provides detailed breakdown of a score
type ScoreDetail struct {
	// Score is the average score
	Score float64 `json:"score"`

	// Count is the number of evaluations
	Count int `json:"count"`

	// Min is the minimum score
	Min float64 `json:"min"`

	// Max is the maximum score
	Max float64 `json:"max"`

	// Trend indicates score trend (-1 = declining, 0 = stable, 1 = improving)
	Trend int `json:"trend"`
}

// GenerateAgentReport generates a comprehensive report for an agent
func (g *ReportGenerator) GenerateAgentReport(ctx context.Context, agentID string, period TimePeriod) (*AgentReport, error) {
	// Get summary
	summary, err := g.store.GetAgentSummary(ctx, agentID, period)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent summary: %w", err)
	}

	report := &AgentReport{
		AgentID:        agentID,
		Period:         period,
		GeneratedAt:    time.Now(),
		Summary:        summary,
		ScoreBreakdown: make(map[EvaluationType]ScoreDetail),
	}

	// Get evaluations for detailed analysis
	startTime, endTime := GetTimePeriodRange(period)
	result, err := g.store.ListEvaluations(ctx, &EvaluationFilter{
		AgentID:   agentID,
		StartTime: &startTime,
		EndTime:   &endTime,
		Limit:     1000,
		OrderBy:   "created_at",
		OrderDesc: true,
	})
	if err != nil {
		return report, nil // Return partial report
	}

	// Calculate trends
	report.Trends = g.calculateTrends(result.Evaluations, period)

	// Calculate score breakdown
	report.ScoreBreakdown = g.calculateScoreBreakdown(result.Evaluations)

	// Identify issues
	report.TopIssues = g.identifyIssues(result.Evaluations, summary)

	// Generate recommendations
	report.Recommendations = g.generateRecommendations(summary, report.TopIssues)

	return report, nil
}

// calculateTrends calculates performance trends over time
func (g *ReportGenerator) calculateTrends(evals []*Evaluation, period TimePeriod) []TrendPoint {
	if len(evals) == 0 {
		return nil
	}

	// Determine bucket size based on period
	var bucketDuration time.Duration
	var numBuckets int

	switch period {
	case Last1Hour:
		bucketDuration = 5 * time.Minute
		numBuckets = 12
	case Last24Hours:
		bucketDuration = time.Hour
		numBuckets = 24
	case Last7Days:
		bucketDuration = 24 * time.Hour
		numBuckets = 7
	case Last30Days:
		bucketDuration = 24 * time.Hour
		numBuckets = 30
	default:
		bucketDuration = 24 * time.Hour
		numBuckets = 30
	}

	// Create buckets
	now := time.Now()
	buckets := make(map[int][]evalData)

	for _, eval := range evals {
		bucketIdx := int(now.Sub(eval.CreatedAt) / bucketDuration)
		if bucketIdx >= 0 && bucketIdx < numBuckets {
			data := evalData{score: eval.Score}
			if eval.Metrics != nil {
				data.completed = eval.Metrics.TaskCompleted
				data.cost = eval.Metrics.TotalCost
				data.durationMs = float64(eval.Metrics.TotalDurationMs)
			}
			buckets[bucketIdx] = append(buckets[bucketIdx], data)
		}
	}

	// Convert to trend points
	var trends []TrendPoint
	for i := numBuckets - 1; i >= 0; i-- {
		data := buckets[i]
		if len(data) == 0 {
			continue
		}

		point := TrendPoint{
			Timestamp:   now.Add(-time.Duration(i) * bucketDuration),
			Evaluations: len(data),
		}

		var scoreSum, costSum, durationSum float64
		var completedCount int
		for _, d := range data {
			scoreSum += d.score
			costSum += d.cost
			durationSum += d.durationMs
			if d.completed {
				completedCount++
			}
		}

		point.Score = scoreSum / float64(len(data))
		point.AvgCost = costSum / float64(len(data))
		point.AvgDurationMs = durationSum / float64(len(data))
		point.TaskCompletionRate = float64(completedCount) / float64(len(data))

		trends = append(trends, point)
	}

	return trends
}

type evalData struct {
	score      float64
	completed  bool
	cost       float64
	durationMs float64
}

// calculateScoreBreakdown calculates detailed score breakdown by type
func (g *ReportGenerator) calculateScoreBreakdown(evals []*Evaluation) map[EvaluationType]ScoreDetail {
	breakdown := make(map[EvaluationType]ScoreDetail)

	// Group by type
	byType := make(map[EvaluationType][]*Evaluation)
	for _, eval := range evals {
		byType[eval.Type] = append(byType[eval.Type], eval)
	}

	for evalType, typeEvals := range byType {
		if len(typeEvals) == 0 {
			continue
		}

		// Sort by time for trend calculation
		sort.Slice(typeEvals, func(i, j int) bool {
			return typeEvals[i].CreatedAt.Before(typeEvals[j].CreatedAt)
		})

		var sum, min, max float64
		min = 1.0
		for _, eval := range typeEvals {
			sum += eval.Score
			if eval.Score < min {
				min = eval.Score
			}
			if eval.Score > max {
				max = eval.Score
			}
		}

		avg := sum / float64(len(typeEvals))

		// Calculate trend
		trend := 0
		if len(typeEvals) >= 4 {
			// Compare first half to second half
			mid := len(typeEvals) / 2
			var firstHalf, secondHalf float64
			for i := 0; i < mid; i++ {
				firstHalf += typeEvals[i].Score
			}
			for i := mid; i < len(typeEvals); i++ {
				secondHalf += typeEvals[i].Score
			}
			firstAvg := firstHalf / float64(mid)
			secondAvg := secondHalf / float64(len(typeEvals)-mid)

			if secondAvg > firstAvg+0.05 {
				trend = 1 // Improving
			} else if secondAvg < firstAvg-0.05 {
				trend = -1 // Declining
			}
		}

		breakdown[evalType] = ScoreDetail{
			Score: avg,
			Count: len(typeEvals),
			Min:   min,
			Max:   max,
			Trend: trend,
		}
	}

	return breakdown
}

// identifyIssues identifies common issues from evaluations
func (g *ReportGenerator) identifyIssues(evals []*Evaluation, summary *EvaluationSummary) []Issue {
	var issues []Issue

	// Check task completion rate
	if summary.TaskCompletionRate < 0.8 {
		severity := "medium"
		if summary.TaskCompletionRate < 0.5 {
			severity = "high"
		}
		issues = append(issues, Issue{
			Type:           "low_completion_rate",
			Description:    fmt.Sprintf("Task completion rate is %.1f%%, below 80%% threshold", summary.TaskCompletionRate*100),
			Severity:       severity,
			Occurrences:    int((1 - summary.TaskCompletionRate) * float64(summary.TotalTraces)),
			Recommendation: "Review failed tasks to identify common failure patterns",
		})
	}

	// Check error rate
	if summary.ErrorRate > 0.1 {
		severity := "medium"
		if summary.ErrorRate > 0.2 {
			severity = "high"
		}
		issues = append(issues, Issue{
			Type:           "high_error_rate",
			Description:    fmt.Sprintf("Error rate is %.1f%%, above 10%% threshold", summary.ErrorRate*100),
			Severity:       severity,
			Occurrences:    int(summary.ErrorRate * float64(summary.TotalTraces)),
			Recommendation: "Investigate error logs and add better error handling",
		})
	}

	// Check cost efficiency
	if summary.AvgCostPerTask > 0.1 {
		issues = append(issues, Issue{
			Type:           "high_cost",
			Description:    fmt.Sprintf("Average cost per task ($%.4f) is high", summary.AvgCostPerTask),
			Severity:       "medium",
			Occurrences:    summary.TotalTraces,
			Recommendation: "Consider using a smaller model or optimizing prompts",
		})
	}

	// Check latency
	if summary.AvgDurationMs > 30000 {
		issues = append(issues, Issue{
			Type:           "high_latency",
			Description:    fmt.Sprintf("Average duration (%.1fs) exceeds 30s target", summary.AvgDurationMs/1000),
			Severity:       "medium",
			Occurrences:    summary.TotalTraces,
			Recommendation: "Optimize agent iterations or use faster tools",
		})
	}

	// Analyze individual evaluations for patterns
	lowScoreCount := 0
	for _, eval := range evals {
		if eval.Score < 0.5 {
			lowScoreCount++
		}
	}

	if lowScoreCount > len(evals)/4 {
		issues = append(issues, Issue{
			Type:           "low_scores",
			Description:    fmt.Sprintf("%d evaluations (%.1f%%) scored below 0.5", lowScoreCount, float64(lowScoreCount)/float64(len(evals))*100),
			Severity:       "high",
			Occurrences:    lowScoreCount,
			Recommendation: "Review low-scoring tasks to identify improvement areas",
		})
	}

	// Sort by severity
	severityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
	sort.Slice(issues, func(i, j int) bool {
		return severityOrder[issues[i].Severity] < severityOrder[issues[j].Severity]
	})

	return issues
}

// generateRecommendations generates improvement recommendations
func (g *ReportGenerator) generateRecommendations(summary *EvaluationSummary, issues []Issue) []string {
	var recommendations []string

	// Based on overall score
	if summary.AvgScore < 0.6 {
		recommendations = append(recommendations, "Consider reviewing the agent's system prompt and tool configurations")
	} else if summary.AvgScore < 0.8 {
		recommendations = append(recommendations, "Agent is performing adequately but has room for improvement")
	}

	// Based on scores by type
	for evalType, score := range summary.ScoresByType {
		if score < 0.6 {
			switch evalType {
			case TypeProductivity:
				recommendations = append(recommendations, "Focus on improving task completion and iteration efficiency")
			case TypeLatency:
				recommendations = append(recommendations, "Optimize for faster response times")
			case TypeCost:
				recommendations = append(recommendations, "Review token usage and consider model optimization")
			case TypeError:
				recommendations = append(recommendations, "Implement better error handling and recovery")
			case TypeQuality:
				recommendations = append(recommendations, "Review response quality and accuracy")
			}
		}
	}

	// Based on issues
	hasHighSeverity := false
	for _, issue := range issues {
		if issue.Severity == "high" {
			hasHighSeverity = true
			break
		}
	}

	if hasHighSeverity {
		recommendations = append(recommendations, "Address high-severity issues immediately to prevent degradation")
	}

	// General recommendations
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Agent is performing well. Continue monitoring for any changes.")
	}

	return recommendations
}

// ComparisonReport compares multiple agents
type ComparisonReport struct {
	// AgentIDs are the compared agents
	AgentIDs []string `json:"agent_ids"`

	// Period is the comparison period
	Period TimePeriod `json:"period"`

	// GeneratedAt is when the report was generated
	GeneratedAt time.Time `json:"generated_at"`

	// Summaries are the individual agent summaries
	Summaries map[string]*EvaluationSummary `json:"summaries"`

	// Comparisons are metric comparisons
	Comparisons []MetricComparison `json:"comparisons"`

	// Rankings show agent rankings
	Rankings map[string]map[string]int `json:"rankings"`

	// Winner is the best performing agent
	Winner string `json:"winner,omitempty"`

	// Insights are comparison insights
	Insights []string `json:"insights,omitempty"`
}

// MetricComparison compares a single metric across agents
type MetricComparison struct {
	// Metric name
	Metric string `json:"metric"`

	// Values by agent
	Values map[string]float64 `json:"values"`

	// Best agent for this metric
	Best string `json:"best"`

	// BestValue is the best value
	BestValue float64 `json:"best_value"`

	// Difference between best and worst
	Difference float64 `json:"difference"`

	// HigherIsBetter indicates the metric direction
	HigherIsBetter bool `json:"higher_is_better"`
}

// CompareAgents compares multiple agents
func (g *ReportGenerator) CompareAgents(ctx context.Context, agentIDs []string, period TimePeriod) (*ComparisonReport, error) {
	if len(agentIDs) < 2 {
		return nil, fmt.Errorf("at least 2 agents required for comparison")
	}

	report := &ComparisonReport{
		AgentIDs:    agentIDs,
		Period:      period,
		GeneratedAt: time.Now(),
		Summaries:   make(map[string]*EvaluationSummary),
		Rankings:    make(map[string]map[string]int),
	}

	// Get summaries for all agents
	for _, agentID := range agentIDs {
		summary, err := g.store.GetAgentSummary(ctx, agentID, period)
		if err != nil {
			continue
		}
		report.Summaries[agentID] = summary
	}

	if len(report.Summaries) < 2 {
		return nil, fmt.Errorf("insufficient data for comparison")
	}

	// Compare metrics
	report.Comparisons = g.compareMetrics(report.Summaries)

	// Calculate rankings
	report.Rankings = g.calculateRankings(report.Comparisons)

	// Determine winner
	report.Winner = g.determineWinner(report.Rankings)

	// Generate insights
	report.Insights = g.generateComparisonInsights(report)

	return report, nil
}

// compareMetrics compares metrics across agents
func (g *ReportGenerator) compareMetrics(summaries map[string]*EvaluationSummary) []MetricComparison {
	metrics := []struct {
		name           string
		getter         func(*EvaluationSummary) float64
		higherIsBetter bool
	}{
		{"avg_score", func(s *EvaluationSummary) float64 { return s.AvgScore }, true},
		{"task_completion_rate", func(s *EvaluationSummary) float64 { return s.TaskCompletionRate }, true},
		{"error_rate", func(s *EvaluationSummary) float64 { return s.ErrorRate }, false},
		{"avg_cost_per_task", func(s *EvaluationSummary) float64 { return s.AvgCostPerTask }, false},
		{"avg_duration_ms", func(s *EvaluationSummary) float64 { return s.AvgDurationMs }, false},
		{"avg_quality_score", func(s *EvaluationSummary) float64 { return s.AvgQualityScore }, true},
	}

	var comparisons []MetricComparison

	for _, m := range metrics {
		comp := MetricComparison{
			Metric:         m.name,
			Values:         make(map[string]float64),
			HigherIsBetter: m.higherIsBetter,
		}

		var best string
		var bestVal float64
		var worst float64
		first := true

		for agentID, summary := range summaries {
			val := m.getter(summary)
			comp.Values[agentID] = val

			if first {
				best = agentID
				bestVal = val
				worst = val
				first = false
				continue
			}

			if m.higherIsBetter {
				if val > bestVal {
					best = agentID
					bestVal = val
				}
				if val < worst {
					worst = val
				}
			} else {
				if val < bestVal {
					best = agentID
					bestVal = val
				}
				if val > worst {
					worst = val
				}
			}
		}

		comp.Best = best
		comp.BestValue = bestVal
		if m.higherIsBetter {
			comp.Difference = bestVal - worst
		} else {
			comp.Difference = worst - bestVal
		}

		comparisons = append(comparisons, comp)
	}

	return comparisons
}

// calculateRankings calculates rankings for each metric
func (g *ReportGenerator) calculateRankings(comparisons []MetricComparison) map[string]map[string]int {
	rankings := make(map[string]map[string]int)

	for _, comp := range comparisons {
		// Sort agents by value
		type agentVal struct {
			agent string
			val   float64
		}
		var sorted []agentVal
		for agent, val := range comp.Values {
			sorted = append(sorted, agentVal{agent, val})
		}

		if comp.HigherIsBetter {
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].val > sorted[j].val
			})
		} else {
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].val < sorted[j].val
			})
		}

		rankings[comp.Metric] = make(map[string]int)
		for i, av := range sorted {
			rankings[comp.Metric][av.agent] = i + 1
		}
	}

	return rankings
}

// determineWinner determines the overall winner
func (g *ReportGenerator) determineWinner(rankings map[string]map[string]int) string {
	scores := make(map[string]int)

	for _, metricRanks := range rankings {
		for agent, rank := range metricRanks {
			// Lower rank is better (1st place = 1 point, 2nd = 2 points, etc.)
			scores[agent] += rank
		}
	}

	var winner string
	bestScore := -1
	for agent, score := range scores {
		if bestScore < 0 || score < bestScore {
			winner = agent
			bestScore = score
		}
	}

	return winner
}

// generateComparisonInsights generates insights from the comparison
func (g *ReportGenerator) generateComparisonInsights(report *ComparisonReport) []string {
	var insights []string

	// Winner insight
	if report.Winner != "" {
		insights = append(insights, fmt.Sprintf("Agent '%s' performs best overall", report.Winner))
	}

	// Metric-specific insights
	for _, comp := range report.Comparisons {
		if comp.Difference > 0.1 && comp.Metric == "avg_score" {
			insights = append(insights, fmt.Sprintf("Significant score difference (%.2f) between best and worst agent", comp.Difference))
		}

		if comp.Metric == "avg_cost_per_task" && comp.Difference > 0.05 {
			insights = append(insights, fmt.Sprintf("Agent '%s' is most cost-efficient", comp.Best))
		}

		if comp.Metric == "task_completion_rate" && comp.BestValue > 0.9 {
			insights = append(insights, fmt.Sprintf("Agent '%s' has excellent completion rate (%.1f%%)", comp.Best, comp.BestValue*100))
		}
	}

	return insights
}
