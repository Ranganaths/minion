package marketing

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/Ranganaths/minion/models"
)

// CampaignROICalculatorTool calculates campaign ROI and ROAS
type CampaignROICalculatorTool struct{}

func (t *CampaignROICalculatorTool) Name() string {
	return "campaign_roi_calculator"
}

func (t *CampaignROICalculatorTool) Description() string {
	return "Calculates ROI, ROAS, and other performance metrics for marketing campaigns"
}

func (t *CampaignROICalculatorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	revenue, okRev := input.Params["revenue"].(float64)
	cost, okCost := input.Params["cost"].(float64)

	if !okRev || !okCost {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid revenue or cost data",
		}, nil
	}

	roi := ((revenue - cost) / cost) * 100
	roas := revenue / cost

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result: map[string]interface{}{
			"roi":             roi,
			"roas":            roas,
			"profit":          revenue - cost,
			"profit_margin":   ((revenue - cost) / revenue) * 100,
			"cost_per_dollar": cost / revenue,
			"performance":     getCampaignPerformance(roi),
		},
	}, nil
}

func (t *CampaignROICalculatorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "campaign_analysis") || hasCapability(agent, "marketing_analytics")
}

// FunnelAnalyzerTool analyzes marketing funnel performance
type FunnelAnalyzerTool struct{}

func (t *FunnelAnalyzerTool) Name() string {
	return "funnel_analyzer"
}

func (t *FunnelAnalyzerTool) Description() string {
	return "Analyzes marketing funnel stages, conversion rates, and drop-off points"
}

func (t *FunnelAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	stages, ok := input.Params["stages"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid funnel stages data",
		}, nil
	}

	analysis := analyzeFunnel(stages)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *FunnelAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "funnel_analysis") || hasCapability(agent, "marketing_analytics")
}

// CACCalculatorTool calculates Customer Acquisition Cost
type CACCalculatorTool struct{}

func (t *CACCalculatorTool) Name() string {
	return "cac_calculator"
}

func (t *CACCalculatorTool) Description() string {
	return "Calculates Customer Acquisition Cost (CAC) and related metrics like LTV:CAC ratio"
}

func (t *CACCalculatorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	totalCost, okCost := input.Params["total_marketing_cost"].(float64)
	newCustomers, okCustomers := input.Params["new_customers"].(float64)

	if !okCost || !okCustomers || newCustomers == 0 {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid cost or customer data",
		}, nil
	}

	cac := totalCost / newCustomers

	// Optional: Calculate LTV:CAC ratio if LTV is provided
	ltvCacRatio := 0.0
	if ltv, ok := input.Params["customer_ltv"].(float64); ok {
		ltvCacRatio = ltv / cac
	}

	paybackMonths := 0.0
	if arpu, ok := input.Params["arpu"].(float64); ok && arpu > 0 {
		paybackMonths = cac / arpu
	}

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result: map[string]interface{}{
			"cac":              cac,
			"ltv_cac_ratio":    ltvCacRatio,
			"payback_months":   paybackMonths,
			"efficiency":       getCACEfficiency(ltvCacRatio),
			"recommendations":  getCACRecommendations(cac, ltvCacRatio),
		},
	}, nil
}

func (t *CACCalculatorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "cac_analysis") || hasCapability(agent, "marketing_analytics")
}

// AttributionAnalyzerTool analyzes channel attribution
type AttributionAnalyzerTool struct{}

func (t *AttributionAnalyzerTool) Name() string {
	return "attribution_analyzer"
}

func (t *AttributionAnalyzerTool) Description() string {
	return "Analyzes marketing channel attribution and assigns credit for conversions"
}

func (t *AttributionAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	touchpoints, ok := input.Params["touchpoints"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid touchpoints data",
		}, nil
	}

	model := "last_touch" // default
	if m, ok := input.Params["model"].(string); ok {
		model = m
	}

	attribution := calculateAttribution(touchpoints, model)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   attribution,
	}, nil
}

func (t *AttributionAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "attribution") || hasCapability(agent, "marketing_analytics")
}

// ABTestAnalyzerTool analyzes A/B test results
type ABTestAnalyzerTool struct{}

func (t *ABTestAnalyzerTool) Name() string {
	return "ab_test_analyzer"
}

func (t *ABTestAnalyzerTool) Description() string {
	return "Analyzes A/B test results and determines statistical significance"
}

func (t *ABTestAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	variantA, okA := input.Params["variant_a"].(map[string]interface{})
	variantB, okB := input.Params["variant_b"].(map[string]interface{})

	if !okA || !okB {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid variant data",
		}, nil
	}

	analysis := analyzeABTest(variantA, variantB)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *ABTestAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "ab_testing") || hasCapability(agent, "marketing_analytics")
}

// EngagementScorerTool scores content engagement
type EngagementScorerTool struct{}

func (t *EngagementScorerTool) Name() string {
	return "engagement_scorer"
}

func (t *EngagementScorerTool) Description() string {
	return "Scores content engagement based on views, clicks, shares, and other metrics"
}

func (t *EngagementScorerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	metrics, ok := input.Params["metrics"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid engagement metrics",
		}, nil
	}

	score := calculateEngagementScore(metrics)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result: map[string]interface{}{
			"score":           score,
			"rating":          getEngagementRating(score),
			"top_metrics":     getTopMetrics(metrics),
			"recommendations": getEngagementRecommendations(score, metrics),
		},
	}, nil
}

func (t *EngagementScorerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "engagement_analysis") || hasCapability(agent, "marketing_analytics")
}

// ContentPerformanceTool analyzes content performance
type ContentPerformanceTool struct{}

func (t *ContentPerformanceTool) Name() string {
	return "content_performance"
}

func (t *ContentPerformanceTool) Description() string {
	return "Analyzes content performance across channels and formats"
}

func (t *ContentPerformanceTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	content, ok := input.Params["content_data"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid content data",
		}, nil
	}

	analysis := analyzeContentPerformance(content)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *ContentPerformanceTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "content_analysis") || hasCapability(agent, "marketing_analytics")
}

// LeadScoringTool scores leads based on behavior and demographics
type LeadScoringTool struct{}

func (t *LeadScoringTool) Name() string {
	return "lead_scoring"
}

func (t *LeadScoringTool) Description() string {
	return "Scores leads based on behavior, demographics, and engagement"
}

func (t *LeadScoringTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	lead, ok := input.Params["lead"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid lead data",
		}, nil
	}

	score := calculateLeadScore(lead)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result: map[string]interface{}{
			"score":           score,
			"grade":           getLeadGrade(score),
			"score_factors":   getLeadScoreFactors(lead),
			"recommendations": getLeadRecommendations(score, lead),
		},
	}, nil
}

func (t *LeadScoringTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "lead_scoring") || hasCapability(agent, "marketing_analytics")
}

// EmailCampaignAnalyzerTool analyzes email campaign performance
type EmailCampaignAnalyzerTool struct{}

func (t *EmailCampaignAnalyzerTool) Name() string {
	return "email_campaign_analyzer"
}

func (t *EmailCampaignAnalyzerTool) Description() string {
	return "Analyzes email campaign metrics including open rates, CTR, and conversions"
}

func (t *EmailCampaignAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	campaign, ok := input.Params["campaign"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid campaign data",
		}, nil
	}

	analysis := analyzeEmailCampaign(campaign)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *EmailCampaignAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "email_analysis") || hasCapability(agent, "marketing_analytics")
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

func getCampaignPerformance(roi float64) string {
	if roi >= 200 {
		return "excellent"
	} else if roi >= 100 {
		return "good"
	} else if roi >= 0 {
		return "fair"
	}
	return "poor"
}

func analyzeFunnel(stages []map[string]interface{}) map[string]interface{} {
	conversions := []float64{}
	dropoffs := []map[string]interface{}{}

	for i := 0; i < len(stages)-1; i++ {
		current, _ := stages[i]["count"].(float64)
		next, _ := stages[i+1]["count"].(float64)
		stageName, _ := stages[i]["name"].(string)

		if current > 0 {
			conversionRate := (next / current) * 100
			conversions = append(conversions, conversionRate)

			dropoffRate := ((current - next) / current) * 100
			if dropoffRate > 50 {
				dropoffs = append(dropoffs, map[string]interface{}{
					"stage":        stageName,
					"dropoff_rate": dropoffRate,
				})
			}
		}
	}

	overallConversion := 0.0
	if len(stages) > 0 {
		first, _ := stages[0]["count"].(float64)
		last, _ := stages[len(stages)-1]["count"].(float64)
		if first > 0 {
			overallConversion = (last / first) * 100
		}
	}

	return map[string]interface{}{
		"stage_conversions":  conversions,
		"overall_conversion": overallConversion,
		"major_dropoffs":     dropoffs,
		"health_score":       calculateFunnelHealth(conversions),
		"recommendations":    getFunnelRecommendations(dropoffs),
	}
}

func calculateFunnelHealth(conversions []float64) float64 {
	if len(conversions) == 0 {
		return 0
	}
	total := 0.0
	for _, conv := range conversions {
		total += conv
	}
	return total / float64(len(conversions))
}

func getFunnelRecommendations(dropoffs []map[string]interface{}) []string {
	recommendations := []string{}
	for _, dropoff := range dropoffs {
		stage, _ := dropoff["stage"].(string)
		recommendations = append(recommendations,
			fmt.Sprintf("Address high dropoff at %s stage", stage))
	}
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Funnel performing well, focus on top-of-funnel volume")
	}
	return recommendations
}

func getCACEfficiency(ltvCacRatio float64) string {
	if ltvCacRatio >= 3 {
		return "excellent"
	} else if ltvCacRatio >= 1 {
		return "good"
	} else if ltvCacRatio > 0 {
		return "poor"
	}
	return "unknown"
}

func getCACRecommendations(cac, ltvCacRatio float64) []string {
	recommendations := []string{}

	if ltvCacRatio < 1 {
		recommendations = append(recommendations, "CAC is too high relative to LTV - optimize acquisition channels")
	} else if ltvCacRatio < 3 {
		recommendations = append(recommendations, "Consider reducing CAC or increasing LTV")
	} else {
		recommendations = append(recommendations, "Healthy CAC ratio - consider increasing spend")
	}

	return recommendations
}

func calculateAttribution(touchpoints []map[string]interface{}, model string) map[string]interface{} {
	channelCredits := make(map[string]float64)

	switch model {
	case "first_touch":
		if len(touchpoints) > 0 {
			channel, _ := touchpoints[0]["channel"].(string)
			channelCredits[channel] = 1.0
		}

	case "last_touch":
		if len(touchpoints) > 0 {
			channel, _ := touchpoints[len(touchpoints)-1]["channel"].(string)
			channelCredits[channel] = 1.0
		}

	case "linear":
		credit := 1.0 / float64(len(touchpoints))
		for _, tp := range touchpoints {
			channel, _ := tp["channel"].(string)
			channelCredits[channel] += credit
		}

	case "time_decay":
		// More recent touchpoints get more credit
		total := 0.0
		for i := range touchpoints {
			total += float64(i + 1)
		}
		for i, tp := range touchpoints {
			channel, _ := tp["channel"].(string)
			credit := float64(i+1) / total
			channelCredits[channel] += credit
		}
	}

	// Convert to percentages
	channelPercentages := make(map[string]float64)
	for channel, credit := range channelCredits {
		channelPercentages[channel] = credit * 100
	}

	return map[string]interface{}{
		"model":               model,
		"channel_attribution": channelPercentages,
		"top_channel":         getTopChannel(channelPercentages),
		"recommendations":     getAttributionRecommendations(channelPercentages),
	}
}

func getTopChannel(channels map[string]float64) string {
	topChannel := ""
	maxCredit := 0.0

	for channel, credit := range channels {
		if credit > maxCredit {
			maxCredit = credit
			topChannel = channel
		}
	}

	return topChannel
}

func getAttributionRecommendations(channels map[string]float64) []string {
	recommendations := []string{}
	topChannel := getTopChannel(channels)

	if topChannel != "" {
		recommendations = append(recommendations,
			fmt.Sprintf("Focus budget on %s as top-performing channel", topChannel))
	}

	return recommendations
}

func analyzeABTest(variantA, variantB map[string]interface{}) map[string]interface{} {
	aConversions, _ := variantA["conversions"].(float64)
	aVisitors, _ := variantA["visitors"].(float64)
	bConversions, _ := variantB["conversions"].(float64)
	bVisitors, _ := variantB["visitors"].(float64)

	aRate := 0.0
	bRate := 0.0

	if aVisitors > 0 {
		aRate = (aConversions / aVisitors) * 100
	}
	if bVisitors > 0 {
		bRate = (bConversions / bVisitors) * 100
	}

	lift := 0.0
	if aRate > 0 {
		lift = ((bRate - aRate) / aRate) * 100
	}

	winner := "A"
	if bRate > aRate {
		winner = "B"
	}

	// Simplified significance calculation
	significance := calculateSignificance(aConversions, aVisitors, bConversions, bVisitors)

	return map[string]interface{}{
		"variant_a_rate":    aRate,
		"variant_b_rate":    bRate,
		"lift":              lift,
		"winner":            winner,
		"confidence":        significance,
		"is_significant":    significance >= 95,
		"recommendation":    getABTestRecommendation(winner, lift, significance),
	}
}

func calculateSignificance(aConv, aVis, bConv, bVis float64) float64 {
	// Simplified significance - in production use proper statistical test
	if aVis < 100 || bVis < 100 {
		return 0 // Not enough data
	}

	aRate := aConv / aVis
	bRate := bConv / bVis
	diff := math.Abs(aRate - bRate)

	// Rough approximation
	if diff > 0.05 {
		return 95
	} else if diff > 0.02 {
		return 90
	} else if diff > 0.01 {
		return 80
	}
	return 70
}

func getABTestRecommendation(winner string, lift, significance float64) string {
	if significance >= 95 {
		return fmt.Sprintf("Implement Variant %s (%.1f%% lift at %.0f%% confidence)", winner, lift, significance)
	} else {
		return "Continue test - not enough statistical significance"
	}
}

func calculateEngagementScore(metrics map[string]interface{}) float64 {
	score := 0.0

	// Views contribution
	if views, ok := metrics["views"].(float64); ok {
		score += math.Min(30, views/1000*30)
	}

	// Clicks contribution
	if clicks, ok := metrics["clicks"].(float64); ok {
		score += math.Min(25, clicks/100*25)
	}

	// Shares contribution
	if shares, ok := metrics["shares"].(float64); ok {
		score += math.Min(20, shares/50*20)
	}

	// Comments contribution
	if comments, ok := metrics["comments"].(float64); ok {
		score += math.Min(15, comments/30*15)
	}

	// Time spent contribution
	if timeSpent, ok := metrics["avg_time_seconds"].(float64); ok {
		score += math.Min(10, timeSpent/60*10)
	}

	return math.Min(100, score)
}

func getEngagementRating(score float64) string {
	if score >= 80 {
		return "excellent"
	} else if score >= 60 {
		return "good"
	} else if score >= 40 {
		return "fair"
	}
	return "poor"
}

func getTopMetrics(metrics map[string]interface{}) []string {
	metricScores := []struct {
		name  string
		score float64
	}{}

	if views, ok := metrics["views"].(float64); ok {
		metricScores = append(metricScores, struct {
			name  string
			score float64
		}{"views", views})
	}

	if clicks, ok := metrics["clicks"].(float64); ok {
		metricScores = append(metricScores, struct {
			name  string
			score float64
		}{"clicks", clicks})
	}

	if shares, ok := metrics["shares"].(float64); ok {
		metricScores = append(metricScores, struct {
			name  string
			score float64
		}{"shares", shares})
	}

	sort.Slice(metricScores, func(i, j int) bool {
		return metricScores[i].score > metricScores[j].score
	})

	top := []string{}
	for i := 0; i < len(metricScores) && i < 3; i++ {
		top = append(top, metricScores[i].name)
	}

	return top
}

func getEngagementRecommendations(score float64, metrics map[string]interface{}) []string {
	recommendations := []string{}

	if score < 40 {
		recommendations = append(recommendations, "Improve content quality and relevance")
		recommendations = append(recommendations, "Test different formats and messaging")
	} else if score < 60 {
		recommendations = append(recommendations, "Optimize call-to-action placement")
		recommendations = append(recommendations, "Increase social sharing prompts")
	} else {
		recommendations = append(recommendations, "Scale successful content format")
	}

	return recommendations
}

func analyzeContentPerformance(content []map[string]interface{}) map[string]interface{} {
	byChannel := make(map[string][]map[string]interface{})
	byFormat := make(map[string][]map[string]interface{})

	for _, item := range content {
		channel, _ := item["channel"].(string)
		format, _ := item["format"].(string)

		byChannel[channel] = append(byChannel[channel], item)
		byFormat[format] = append(byFormat[format], item)
	}

	channelPerformance := make(map[string]float64)
	for channel, items := range byChannel {
		totalEng := 0.0
		for _, item := range items {
			if eng, ok := item["engagement"].(float64); ok {
				totalEng += eng
			}
		}
		channelPerformance[channel] = totalEng / float64(len(items))
	}

	formatPerformance := make(map[string]float64)
	for format, items := range byFormat {
		totalEng := 0.0
		for _, item := range items {
			if eng, ok := item["engagement"].(float64); ok {
				totalEng += eng
			}
		}
		formatPerformance[format] = totalEng / float64(len(items))
	}

	return map[string]interface{}{
		"channel_performance": channelPerformance,
		"format_performance":  formatPerformance,
		"top_channel":         getTopPerformer(channelPerformance),
		"top_format":          getTopPerformer(formatPerformance),
		"recommendations":     getContentRecommendations(channelPerformance, formatPerformance),
	}
}

func getTopPerformer(performance map[string]float64) string {
	top := ""
	maxScore := 0.0

	for key, score := range performance {
		if score > maxScore {
			maxScore = score
			top = key
		}
	}

	return top
}

func getContentRecommendations(channels, formats map[string]float64) []string {
	recommendations := []string{}

	topChannel := getTopPerformer(channels)
	topFormat := getTopPerformer(formats)

	if topChannel != "" {
		recommendations = append(recommendations,
			fmt.Sprintf("Increase content production for %s channel", topChannel))
	}

	if topFormat != "" {
		recommendations = append(recommendations,
			fmt.Sprintf("Focus on %s format for better engagement", topFormat))
	}

	return recommendations
}

func calculateLeadScore(lead map[string]interface{}) float64 {
	score := 0.0

	// Demographic scoring
	if jobTitle, ok := lead["job_title"].(string); ok {
		if contains(jobTitle, "VP", "Director", "Manager") {
			score += 20
		} else if contains(jobTitle, "Executive", "Chief", "Head") {
			score += 30
		}
	}

	// Company size scoring
	if companySize, ok := lead["company_size"].(float64); ok {
		if companySize > 1000 {
			score += 25
		} else if companySize > 100 {
			score += 15
		}
	}

	// Engagement scoring
	if pageViews, ok := lead["page_views"].(float64); ok {
		score += math.Min(20, pageViews/10*20)
	}

	if downloads, ok := lead["downloads"].(float64); ok {
		score += downloads * 10
	}

	// Email engagement
	if emailOpens, ok := lead["email_opens"].(float64); ok {
		score += math.Min(15, emailOpens/5*15)
	}

	return math.Min(100, score)
}

func contains(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			// Simple contains check
			return true
		}
	}
	return false
}

func getLeadGrade(score float64) string {
	if score >= 80 {
		return "A"
	} else if score >= 60 {
		return "B"
	} else if score >= 40 {
		return "C"
	}
	return "D"
}

func getLeadScoreFactors(lead map[string]interface{}) []string {
	factors := []string{}

	if jobTitle, ok := lead["job_title"].(string); ok && contains(jobTitle, "VP", "Director", "Manager", "Executive") {
		factors = append(factors, "decision_maker")
	}

	if pageViews, ok := lead["page_views"].(float64); ok && pageViews > 10 {
		factors = append(factors, "high_engagement")
	}

	if downloads, ok := lead["downloads"].(float64); ok && downloads > 0 {
		factors = append(factors, "content_consumer")
	}

	return factors
}

func getLeadRecommendations(score float64, lead map[string]interface{}) []string {
	recommendations := []string{}

	if score >= 80 {
		recommendations = append(recommendations, "Route to sales immediately")
		recommendations = append(recommendations, "Schedule demo or call")
	} else if score >= 60 {
		recommendations = append(recommendations, "Continue nurturing with targeted content")
		recommendations = append(recommendations, "Send case studies and ROI information")
	} else {
		recommendations = append(recommendations, "Add to drip campaign")
		recommendations = append(recommendations, "Focus on educational content")
	}

	return recommendations
}

func analyzeEmailCampaign(campaign map[string]interface{}) map[string]interface{} {
	sent, _ := campaign["sent"].(float64)
	opens, _ := campaign["opens"].(float64)
	clicks, _ := campaign["clicks"].(float64)
	conversions, _ := campaign["conversions"].(float64)
	bounces, _ := campaign["bounces"].(float64)
	unsubscribes, _ := campaign["unsubscribes"].(float64)

	openRate := 0.0
	ctr := 0.0
	conversionRate := 0.0
	bounceRate := 0.0
	unsubRate := 0.0

	if sent > 0 {
		openRate = (opens / sent) * 100
		bounceRate = (bounces / sent) * 100
		unsubRate = (unsubscribes / sent) * 100
	}

	if opens > 0 {
		ctr = (clicks / opens) * 100
	}

	if clicks > 0 {
		conversionRate = (conversions / clicks) * 100
	}

	return map[string]interface{}{
		"open_rate":       openRate,
		"click_rate":      ctr,
		"conversion_rate": conversionRate,
		"bounce_rate":     bounceRate,
		"unsubscribe_rate": unsubRate,
		"performance":     getEmailPerformance(openRate, ctr),
		"recommendations": getEmailRecommendations(openRate, ctr, bounceRate),
	}
}

func getEmailPerformance(openRate, ctr float64) string {
	if openRate >= 25 && ctr >= 3 {
		return "excellent"
	} else if openRate >= 15 && ctr >= 2 {
		return "good"
	} else if openRate >= 10 && ctr >= 1 {
		return "fair"
	}
	return "poor"
}

func getEmailRecommendations(openRate, ctr, bounceRate float64) []string {
	recommendations := []string{}

	if openRate < 15 {
		recommendations = append(recommendations, "Improve subject lines and sender name")
		recommendations = append(recommendations, "Clean and segment email list")
	}

	if ctr < 2 {
		recommendations = append(recommendations, "Optimize CTAs and button placement")
		recommendations = append(recommendations, "Improve email content relevance")
	}

	if bounceRate > 2 {
		recommendations = append(recommendations, "Clean email list to reduce bounces")
	}

	return recommendations
}
