package sales

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/yourusername/minion/models"
)

// RevenueAnalyzerTool analyzes revenue metrics and trends
type RevenueAnalyzerTool struct{}

func (t *RevenueAnalyzerTool) Name() string {
	return "revenue_analyzer"
}

func (t *RevenueAnalyzerTool) Description() string {
	return "Analyzes revenue data including trends, growth rates, and forecasts"
}

func (t *RevenueAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	revenues, ok := input.Params["revenues"].([]float64)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid revenues data",
		}, nil
	}

	analysis := map[string]interface{}{
		"total_revenue": sum(revenues),
		"average":       average(revenues),
		"growth_rate":   calculateGrowthRate(revenues),
		"trend":         determineTrend(revenues),
		"volatility":    calculateVolatility(revenues),
	}

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *RevenueAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "revenue_analysis") || hasCapability(agent, "sales_analytics")
}

// PipelineAnalyzerTool analyzes sales pipeline health
type PipelineAnalyzerTool struct{}

func (t *PipelineAnalyzerTool) Name() string {
	return "pipeline_analyzer"
}

func (t *PipelineAnalyzerTool) Description() string {
	return "Analyzes sales pipeline health, conversion rates, and bottlenecks"
}

func (t *PipelineAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	stages, ok := input.Params["stages"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid pipeline stages data",
		}, nil
	}

	analysis := analyzePipeline(stages)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *PipelineAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "pipeline_analysis") || hasCapability(agent, "sales_analytics")
}

// CustomerSegmentationTool segments customers based on various criteria
type CustomerSegmentationTool struct{}

func (t *CustomerSegmentationTool) Name() string {
	return "customer_segmentation"
}

func (t *CustomerSegmentationTool) Description() string {
	return "Segments customers by revenue, behavior, or custom criteria"
}

func (t *CustomerSegmentationTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	customers, ok := input.Params["customers"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid customer data",
		}, nil
	}

	criteria := "revenue" // default
	if c, ok := input.Params["criteria"].(string); ok {
		criteria = c
	}

	segments := segmentCustomers(customers, criteria)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   segments,
	}, nil
}

func (t *CustomerSegmentationTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "customer_segmentation") || hasCapability(agent, "sales_analytics")
}

// DealScoringTool scores deals based on probability to close
type DealScoringTool struct{}

func (t *DealScoringTool) Name() string {
	return "deal_scoring"
}

func (t *DealScoringTool) Description() string {
	return "Scores deals based on likelihood to close and priority"
}

func (t *DealScoringTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	deal, ok := input.Params["deal"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid deal data",
		}, nil
	}

	score := calculateDealScore(deal)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result: map[string]interface{}{
			"score":       score,
			"priority":    getPriority(score),
			"factors":     getScoreFactors(deal),
			"recommended": getRecommendations(score, deal),
		},
	}, nil
}

func (t *DealScoringTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "deal_scoring") || hasCapability(agent, "sales_analytics")
}

// ForecastingTool generates sales forecasts
type ForecastingTool struct{}

func (t *ForecastingTool) Name() string {
	return "sales_forecasting"
}

func (t *ForecastingTool) Description() string {
	return "Generates sales forecasts based on historical data and trends"
}

func (t *ForecastingTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	historicalData, ok := input.Params["historical_data"].([]float64)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid historical data",
		}, nil
	}

	periods := 3 // default forecast periods
	if p, ok := input.Params["periods"].(int); ok {
		periods = p
	}

	forecast := generateForecast(historicalData, periods)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   forecast,
	}, nil
}

func (t *ForecastingTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "forecasting") || hasCapability(agent, "sales_analytics")
}

// ConversionRateAnalyzerTool analyzes conversion rates across pipeline stages
type ConversionRateAnalyzerTool struct{}

func (t *ConversionRateAnalyzerTool) Name() string {
	return "conversion_rate_analyzer"
}

func (t *ConversionRateAnalyzerTool) Description() string {
	return "Analyzes conversion rates between pipeline stages and identifies bottlenecks"
}

func (t *ConversionRateAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	stageData, ok := input.Params["stage_data"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid stage data",
		}, nil
	}

	analysis := analyzeConversionRates(stageData)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *ConversionRateAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "conversion_analysis") || hasCapability(agent, "sales_analytics")
}

// ChurnPredictorTool predicts customer churn risk
type ChurnPredictorTool struct{}

func (t *ChurnPredictorTool) Name() string {
	return "churn_predictor"
}

func (t *ChurnPredictorTool) Description() string {
	return "Predicts customer churn risk based on engagement and usage patterns"
}

func (t *ChurnPredictorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	customer, ok := input.Params["customer"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid customer data",
		}, nil
	}

	churnRisk := calculateChurnRisk(customer)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result: map[string]interface{}{
			"churn_risk":     churnRisk,
			"risk_level":     getRiskLevel(churnRisk),
			"risk_factors":   identifyRiskFactors(customer),
			"recommendations": getRetentionRecommendations(churnRisk, customer),
		},
	}, nil
}

func (t *ChurnPredictorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "churn_prediction") || hasCapability(agent, "customer_analytics")
}

// QuotaAttainmentTool analyzes quota attainment and performance
type QuotaAttainmentTool struct{}

func (t *QuotaAttainmentTool) Name() string {
	return "quota_attainment"
}

func (t *QuotaAttainmentTool) Description() string {
	return "Analyzes sales team quota attainment and performance metrics"
}

func (t *QuotaAttainmentTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	salesData, ok := input.Params["sales_data"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid sales data",
		}, nil
	}

	analysis := analyzeQuotaAttainment(salesData)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *QuotaAttainmentTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "quota_analysis") || hasCapability(agent, "sales_analytics")
}

// Helper functions

func hasCapability(agent *models.Agent, capability string) bool {
	for _, cap := range agent.Capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

func sum(values []float64) float64 {
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return sum(values) / float64(len(values))
}

func calculateGrowthRate(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	initial := values[0]
	final := values[len(values)-1]
	if initial == 0 {
		return 0
	}
	return ((final - initial) / initial) * 100
}

func determineTrend(values []float64) string {
	if len(values) < 2 {
		return "insufficient_data"
	}

	increasing := 0
	decreasing := 0
	for i := 1; i < len(values); i++ {
		if values[i] > values[i-1] {
			increasing++
		} else if values[i] < values[i-1] {
			decreasing++
		}
	}

	if increasing > decreasing {
		return "upward"
	} else if decreasing > increasing {
		return "downward"
	}
	return "stable"
}

func calculateVolatility(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	avg := average(values)
	variance := 0.0
	for _, v := range values {
		variance += math.Pow(v-avg, 2)
	}
	return math.Sqrt(variance / float64(len(values)))
}

func analyzePipeline(stages map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"health_score":    calculatePipelineHealth(stages),
		"bottlenecks":     identifyBottlenecks(stages),
		"recommendations": getPipelineRecommendations(stages),
		"velocity":        calculateVelocity(stages),
	}
}

func calculatePipelineHealth(stages map[string]interface{}) float64 {
	// Simplified health calculation
	return 75.0 // Placeholder
}

func identifyBottlenecks(stages map[string]interface{}) []string {
	return []string{"negotiation", "proposal"} // Placeholder
}

func getPipelineRecommendations(stages map[string]interface{}) []string {
	return []string{
		"Focus on accelerating negotiation stage",
		"Improve proposal quality and response time",
	}
}

func calculateVelocity(stages map[string]interface{}) float64 {
	return 14.5 // days average
}

func segmentCustomers(customers []map[string]interface{}, criteria string) map[string]interface{} {
	segments := map[string][]map[string]interface{}{
		"high_value":   {},
		"medium_value": {},
		"low_value":    {},
	}

	for _, customer := range customers {
		revenue := 0.0
		if r, ok := customer["revenue"].(float64); ok {
			revenue = r
		}

		if revenue > 100000 {
			segments["high_value"] = append(segments["high_value"], customer)
		} else if revenue > 10000 {
			segments["medium_value"] = append(segments["medium_value"], customer)
		} else {
			segments["low_value"] = append(segments["low_value"], customer)
		}
	}

	return map[string]interface{}{
		"segments": segments,
		"summary": map[string]int{
			"high_value":   len(segments["high_value"]),
			"medium_value": len(segments["medium_value"]),
			"low_value":    len(segments["low_value"]),
		},
	}
}

func calculateDealScore(deal map[string]interface{}) float64 {
	score := 50.0 // base score

	// Adjust based on deal value
	if value, ok := deal["value"].(float64); ok {
		if value > 100000 {
			score += 20
		} else if value > 50000 {
			score += 10
		}
	}

	// Adjust based on stage
	if stage, ok := deal["stage"].(string); ok {
		switch stage {
		case "negotiation":
			score += 15
		case "proposal":
			score += 10
		case "qualification":
			score += 5
		}
	}

	// Adjust based on age
	if age, ok := deal["age_days"].(float64); ok {
		if age < 30 {
			score += 10
		} else if age > 90 {
			score -= 20
		}
	}

	return math.Min(100, math.Max(0, score))
}

func getPriority(score float64) string {
	if score >= 80 {
		return "high"
	} else if score >= 50 {
		return "medium"
	}
	return "low"
}

func getScoreFactors(deal map[string]interface{}) []string {
	factors := []string{}
	if value, ok := deal["value"].(float64); ok && value > 50000 {
		factors = append(factors, "high_value")
	}
	if stage, ok := deal["stage"].(string); ok && stage == "negotiation" {
		factors = append(factors, "advanced_stage")
	}
	return factors
}

func getRecommendations(score float64, deal map[string]interface{}) []string {
	recommendations := []string{}
	if score >= 80 {
		recommendations = append(recommendations, "Prioritize this deal for closing")
	} else if score < 50 {
		recommendations = append(recommendations, "Consider reallocating resources")
	}
	return recommendations
}

func generateForecast(historical []float64, periods int) map[string]interface{} {
	if len(historical) < 2 {
		return map[string]interface{}{
			"forecast": []float64{},
			"error":    "insufficient_data",
		}
	}

	// Simple linear forecast
	trend := (historical[len(historical)-1] - historical[0]) / float64(len(historical)-1)
	lastValue := historical[len(historical)-1]

	forecast := make([]float64, periods)
	for i := 0; i < periods; i++ {
		forecast[i] = lastValue + trend*float64(i+1)
	}

	return map[string]interface{}{
		"forecast":        forecast,
		"confidence":      0.75,
		"trend":           trend,
		"method":          "linear",
		"historical_avg":  average(historical),
		"forecast_avg":    average(forecast),
	}
}

func analyzeConversionRates(stageData []map[string]interface{}) map[string]interface{} {
	rates := make(map[string]float64)
	bottlenecks := []string{}

	for i := 0; i < len(stageData)-1; i++ {
		currentStage := stageData[i]
		nextStage := stageData[i+1]

		stageName, _ := currentStage["name"].(string)
		currentCount, _ := currentStage["count"].(float64)
		nextCount, _ := nextStage["count"].(float64)

		if currentCount > 0 {
			rate := (nextCount / currentCount) * 100
			rates[stageName] = rate

			if rate < 50 {
				bottlenecks = append(bottlenecks, stageName)
			}
		}
	}

	return map[string]interface{}{
		"conversion_rates": rates,
		"bottlenecks":      bottlenecks,
		"overall_rate":     calculateOverallConversion(stageData),
		"recommendations":  getConversionRecommendations(bottlenecks),
	}
}

func calculateOverallConversion(stageData []map[string]interface{}) float64 {
	if len(stageData) < 2 {
		return 0
	}
	first, _ := stageData[0]["count"].(float64)
	last, _ := stageData[len(stageData)-1]["count"].(float64)
	if first == 0 {
		return 0
	}
	return (last / first) * 100
}

func getConversionRecommendations(bottlenecks []string) []string {
	recommendations := []string{}
	for _, stage := range bottlenecks {
		recommendations = append(recommendations,
			fmt.Sprintf("Improve conversion at %s stage", stage))
	}
	return recommendations
}

func calculateChurnRisk(customer map[string]interface{}) float64 {
	risk := 0.0

	// Engagement factor
	if lastActivity, ok := customer["last_activity_days"].(float64); ok {
		if lastActivity > 30 {
			risk += 30
		} else if lastActivity > 14 {
			risk += 15
		}
	}

	// Support tickets factor
	if tickets, ok := customer["support_tickets"].(float64); ok {
		if tickets > 5 {
			risk += 25
		} else if tickets > 2 {
			risk += 10
		}
	}

	// Contract factor
	if contractDays, ok := customer["contract_days_remaining"].(float64); ok {
		if contractDays < 30 {
			risk += 20
		}
	}

	return math.Min(100, risk)
}

func getRiskLevel(risk float64) string {
	if risk >= 70 {
		return "high"
	} else if risk >= 40 {
		return "medium"
	}
	return "low"
}

func identifyRiskFactors(customer map[string]interface{}) []string {
	factors := []string{}

	if lastActivity, ok := customer["last_activity_days"].(float64); ok && lastActivity > 30 {
		factors = append(factors, "low_engagement")
	}

	if tickets, ok := customer["support_tickets"].(float64); ok && tickets > 5 {
		factors = append(factors, "high_support_volume")
	}

	return factors
}

func getRetentionRecommendations(risk float64, customer map[string]interface{}) []string {
	recommendations := []string{}

	if risk >= 70 {
		recommendations = append(recommendations, "Schedule immediate check-in call")
		recommendations = append(recommendations, "Review account health with success team")
	} else if risk >= 40 {
		recommendations = append(recommendations, "Send engagement survey")
		recommendations = append(recommendations, "Offer product training")
	}

	return recommendations
}

func analyzeQuotaAttainment(salesData []map[string]interface{}) map[string]interface{} {
	totalReps := len(salesData)
	metQuota := 0
	exceeded := 0
	totalAttainment := 0.0

	attainmentByRep := []map[string]interface{}{}

	for _, rep := range salesData {
		actual, _ := rep["actual"].(float64)
		quota, _ := rep["quota"].(float64)

		if quota > 0 {
			attainment := (actual / quota) * 100
			totalAttainment += attainment

			if attainment >= 100 {
				metQuota++
				if attainment >= 120 {
					exceeded++
				}
			}

			repName, _ := rep["name"].(string)
			attainmentByRep = append(attainmentByRep, map[string]interface{}{
				"name":       repName,
				"attainment": attainment,
				"actual":     actual,
				"quota":      quota,
			})
		}
	}

	// Sort by attainment
	sort.Slice(attainmentByRep, func(i, j int) bool {
		return attainmentByRep[i]["attainment"].(float64) > attainmentByRep[j]["attainment"].(float64)
	})

	avgAttainment := 0.0
	if totalReps > 0 {
		avgAttainment = totalAttainment / float64(totalReps)
	}

	return map[string]interface{}{
		"total_reps":     totalReps,
		"met_quota":      metQuota,
		"exceeded_quota": exceeded,
		"avg_attainment": avgAttainment,
		"top_performers": attainmentByRep[:min(5, len(attainmentByRep))],
		"recommendations": getQuotaRecommendations(avgAttainment, metQuota, totalReps),
	}
}

func getQuotaRecommendations(avgAttainment float64, metQuota, totalReps int) []string {
	recommendations := []string{}

	if avgAttainment < 80 {
		recommendations = append(recommendations, "Review sales process and training needs")
	}

	metRate := float64(metQuota) / float64(totalReps) * 100
	if metRate < 50 {
		recommendations = append(recommendations, "Consider quota adjustments or territory realignment")
	}

	return recommendations
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
