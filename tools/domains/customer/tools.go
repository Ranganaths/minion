package customer

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Ranganaths/minion/models"
)

// SentimentAnalyzerTool analyzes sentiment from text and feedback
type SentimentAnalyzerTool struct{}

func (t *SentimentAnalyzerTool) Name() string {
	return "sentiment_analyzer"
}

func (t *SentimentAnalyzerTool) Description() string {
	return "Analyzes sentiment from customer feedback, reviews, and text content"
}

func (t *SentimentAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	text, ok := input.Params["text"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid text input",
		}, nil
	}

	analysis := analyzeSentiment(text)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *SentimentAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "sentiment_analysis") || hasCapability(agent, "customer_support")
}

// TicketClassifierTool auto-classifies support tickets
type TicketClassifierTool struct{}

func (t *TicketClassifierTool) Name() string {
	return "ticket_classifier"
}

func (t *TicketClassifierTool) Description() string {
	return "Automatically classifies support tickets by category, priority, and urgency"
}

func (t *TicketClassifierTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	ticket, ok := input.Params["ticket"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid ticket data",
		}, nil
	}

	classification := classifyTicket(ticket)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   classification,
	}, nil
}

func (t *TicketClassifierTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "ticket_classification") || hasCapability(agent, "customer_support")
}

// ResponseGeneratorTool generates templated responses
type ResponseGeneratorTool struct{}

func (t *ResponseGeneratorTool) Name() string {
	return "response_generator"
}

func (t *ResponseGeneratorTool) Description() string {
	return "Generates context-aware templated responses for customer support"
}

func (t *ResponseGeneratorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	context, ok := input.Params["context"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid context data",
		}, nil
	}

	responseType := "general"
	if rt, ok := input.Params["response_type"].(string); ok {
		responseType = rt
	}

	response := generateResponse(context, responseType)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   response,
	}, nil
}

func (t *ResponseGeneratorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "response_generation") || hasCapability(agent, "customer_support")
}

// CustomerHealthScorerTool provides comprehensive customer wellness score
type CustomerHealthScorerTool struct{}

func (t *CustomerHealthScorerTool) Name() string {
	return "customer_health_scorer"
}

func (t *CustomerHealthScorerTool) Description() string {
	return "Calculates comprehensive customer health score based on multiple factors"
}

func (t *CustomerHealthScorerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	customer, ok := input.Params["customer"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid customer data",
		}, nil
	}

	healthScore := calculateHealthScore(customer)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   healthScore,
	}, nil
}

func (t *CustomerHealthScorerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "customer_health") || hasCapability(agent, "customer_support")
}

// FeedbackAnalyzerTool analyzes customer feedback patterns
type FeedbackAnalyzerTool struct{}

func (t *FeedbackAnalyzerTool) Name() string {
	return "feedback_analyzer"
}

func (t *FeedbackAnalyzerTool) Description() string {
	return "Analyzes customer feedback to identify patterns, trends, and actionable insights"
}

func (t *FeedbackAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	feedback, ok := input.Params["feedback"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid feedback data",
		}, nil
	}

	analysis := analyzeFeedback(feedback)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *FeedbackAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "feedback_analysis") || hasCapability(agent, "customer_support")
}

// NPSCalculatorTool calculates Net Promoter Score
type NPSCalculatorTool struct{}

func (t *NPSCalculatorTool) Name() string {
	return "nps_calculator"
}

func (t *NPSCalculatorTool) Description() string {
	return "Calculates Net Promoter Score from customer survey responses"
}

func (t *NPSCalculatorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	scores, ok := input.Params["scores"].([]float64)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid scores data",
		}, nil
	}

	npsResult := calculateNPS(scores)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   npsResult,
	}, nil
}

func (t *NPSCalculatorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "nps_calculation") || hasCapability(agent, "customer_support")
}

// SupportMetricsAnalyzerTool analyzes support team performance
type SupportMetricsAnalyzerTool struct{}

func (t *SupportMetricsAnalyzerTool) Name() string {
	return "support_metrics_analyzer"
}

func (t *SupportMetricsAnalyzerTool) Description() string {
	return "Analyzes support team metrics including response time, resolution rate, and CSAT"
}

func (t *SupportMetricsAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	metrics, ok := input.Params["metrics"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid metrics data",
		}, nil
	}

	analysis := analyzeSupportMetrics(metrics)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *SupportMetricsAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "support_metrics") || hasCapability(agent, "customer_support")
}

// CSATAnalyzerTool analyzes Customer Satisfaction scores
type CSATAnalyzerTool struct{}

func (t *CSATAnalyzerTool) Name() string {
	return "csat_analyzer"
}

func (t *CSATAnalyzerTool) Description() string {
	return "Analyzes Customer Satisfaction (CSAT) scores and identifies improvement areas"
}

func (t *CSATAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	responses, ok := input.Params["responses"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid responses data",
		}, nil
	}

	analysis := analyzeCSAT(responses)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *CSATAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "csat_analysis") || hasCapability(agent, "customer_support")
}

// TicketRoutingTool intelligently routes tickets to appropriate agents
type TicketRoutingTool struct{}

func (t *TicketRoutingTool) Name() string {
	return "ticket_router"
}

func (t *TicketRoutingTool) Description() string {
	return "Intelligently routes support tickets to the most appropriate agent or team"
}

func (t *TicketRoutingTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	ticket, ok := input.Params["ticket"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid ticket data",
		}, nil
	}

	agents, _ := input.Params["available_agents"].([]map[string]interface{})

	routing := routeTicket(ticket, agents)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   routing,
	}, nil
}

func (t *TicketRoutingTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "ticket_routing") || hasCapability(agent, "customer_support")
}

// KnowledgeBaseSearchTool searches knowledge base for relevant articles
type KnowledgeBaseSearchTool struct{}

func (t *KnowledgeBaseSearchTool) Name() string {
	return "kb_search"
}

func (t *KnowledgeBaseSearchTool) Description() string {
	return "Searches knowledge base for relevant articles and solutions"
}

func (t *KnowledgeBaseSearchTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	query, ok := input.Params["query"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid query string",
		}, nil
	}

	limit := 5
	if l, ok := input.Params["limit"].(int); ok {
		limit = l
	}

	results := searchKnowledgeBase(query, limit)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   results,
	}, nil
}

func (t *KnowledgeBaseSearchTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "kb_search") || hasCapability(agent, "customer_support")
}

// CustomerJourneyAnalyzerTool analyzes customer journey touchpoints
type CustomerJourneyAnalyzerTool struct{}

func (t *CustomerJourneyAnalyzerTool) Name() string {
	return "customer_journey_analyzer"
}

func (t *CustomerJourneyAnalyzerTool) Description() string {
	return "Analyzes customer journey across touchpoints to identify pain points and opportunities"
}

func (t *CustomerJourneyAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	journey, ok := input.Params["journey"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid journey data",
		}, nil
	}

	analysis := analyzeJourney(journey)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *CustomerJourneyAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "journey_analysis") || hasCapability(agent, "customer_support")
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

func analyzeSentiment(text string) map[string]interface{} {
	text = strings.ToLower(text)

	// Simple keyword-based sentiment analysis
	positiveWords := []string{"great", "excellent", "happy", "love", "amazing", "wonderful", "fantastic", "good", "best"}
	negativeWords := []string{"bad", "terrible", "hate", "awful", "poor", "worst", "disappointing", "frustrated", "angry"}

	positiveCount := 0
	negativeCount := 0

	for _, word := range positiveWords {
		positiveCount += strings.Count(text, word)
	}

	for _, word := range negativeWords {
		negativeCount += strings.Count(text, word)
	}

	totalWords := positiveCount + negativeCount
	sentiment := "neutral"
	score := 0.0

	if totalWords > 0 {
		score = float64(positiveCount-negativeCount) / float64(totalWords)
		if score > 0.2 {
			sentiment = "positive"
		} else if score < -0.2 {
			sentiment = "negative"
		}
	}

	confidence := math.Min(1.0, float64(totalWords)/10.0)

	return map[string]interface{}{
		"sentiment":       sentiment,
		"score":           score,
		"confidence":      confidence,
		"positive_count":  positiveCount,
		"negative_count":  negativeCount,
		"analysis": map[string]interface{}{
			"polarity":  getSentimentPolarity(score),
			"intensity": getSentimentIntensity(score),
		},
	}
}

func getSentimentPolarity(score float64) string {
	if score > 0 {
		return "positive"
	} else if score < 0 {
		return "negative"
	}
	return "neutral"
}

func getSentimentIntensity(score float64) string {
	absScore := math.Abs(score)
	if absScore > 0.6 {
		return "strong"
	} else if absScore > 0.3 {
		return "moderate"
	}
	return "weak"
}

func classifyTicket(ticket map[string]interface{}) map[string]interface{} {
	subject, _ := ticket["subject"].(string)
	description, _ := ticket["description"].(string)
	fullText := strings.ToLower(subject + " " + description)

	// Classify category
	category := "general"
	if strings.Contains(fullText, "bug") || strings.Contains(fullText, "error") || strings.Contains(fullText, "broken") {
		category = "technical"
	} else if strings.Contains(fullText, "billing") || strings.Contains(fullText, "payment") || strings.Contains(fullText, "invoice") {
		category = "billing"
	} else if strings.Contains(fullText, "feature") || strings.Contains(fullText, "request") {
		category = "feature_request"
	} else if strings.Contains(fullText, "account") || strings.Contains(fullText, "login") || strings.Contains(fullText, "password") {
		category = "account"
	}

	// Classify priority
	priority := "medium"
	if strings.Contains(fullText, "urgent") || strings.Contains(fullText, "critical") || strings.Contains(fullText, "asap") {
		priority = "high"
	} else if strings.Contains(fullText, "when you can") || strings.Contains(fullText, "no rush") {
		priority = "low"
	}

	// Classify urgency
	urgency := "normal"
	if strings.Contains(fullText, "down") || strings.Contains(fullText, "not working") || strings.Contains(fullText, "broken") {
		urgency = "high"
	}

	// Estimate effort
	effort := estimateEffort(category, fullText)

	return map[string]interface{}{
		"category":    category,
		"priority":    priority,
		"urgency":     urgency,
		"effort":      effort,
		"tags":        extractTags(fullText),
		"routing":     suggestRouting(category),
		"sla_hours":   calculateSLA(priority, urgency),
	}
}

func estimateEffort(category, text string) string {
	if category == "feature_request" {
		return "high"
	}
	if strings.Contains(text, "simple") || strings.Contains(text, "quick") {
		return "low"
	}
	return "medium"
}

func extractTags(text string) []string {
	tags := []string{}

	tagMap := map[string]string{
		"bug":      "bug",
		"feature":  "feature",
		"urgent":   "urgent",
		"billing":  "billing",
		"security": "security",
		"api":      "api",
		"ui":       "ui",
	}

	for keyword, tag := range tagMap {
		if strings.Contains(text, keyword) {
			tags = append(tags, tag)
		}
	}

	return tags
}

func suggestRouting(category string) string {
	routingMap := map[string]string{
		"technical":       "engineering_team",
		"billing":         "finance_team",
		"feature_request": "product_team",
		"account":         "support_team",
		"general":         "support_team",
	}

	if team, ok := routingMap[category]; ok {
		return team
	}
	return "support_team"
}

func calculateSLA(priority, urgency string) int {
	// SLA in hours
	if priority == "high" || urgency == "high" {
		return 4
	} else if priority == "medium" {
		return 24
	}
	return 48
}

func generateResponse(context map[string]interface{}, responseType string) map[string]interface{} {
	templates := map[string]string{
		"general":      "Thank you for contacting us. We've received your inquiry and will get back to you shortly.",
		"bug_report":   "Thank you for reporting this issue. Our engineering team has been notified and will investigate. We'll keep you updated on the progress.",
		"feature_request": "Thank you for your feature suggestion. We've added it to our product roadmap for consideration.",
		"billing":      "Thank you for reaching out about billing. Our finance team will review your account and respond within 24 hours.",
		"resolution":   "We're glad we could help resolve your issue. Please don't hesitate to reach out if you need anything else.",
	}

	template := templates[responseType]
	if template == "" {
		template = templates["general"]
	}

	// Personalize if customer name is available
	if name, ok := context["customer_name"].(string); ok {
		template = fmt.Sprintf("Hi %s, ", name) + template
	}

	return map[string]interface{}{
		"response":      template,
		"response_type": responseType,
		"tone":          "professional",
		"suggestions": []string{
			"Add specific details about the customer's issue",
			"Include timeline expectations",
			"Provide next steps",
		},
	}
}

func calculateHealthScore(customer map[string]interface{}) map[string]interface{} {
	score := 100.0
	factors := []map[string]interface{}{}

	// Engagement factor
	if lastActivity, ok := customer["last_activity_days"].(float64); ok {
		if lastActivity > 30 {
			score -= 20
			factors = append(factors, map[string]interface{}{
				"factor": "low_engagement",
				"impact": -20,
			})
		} else if lastActivity > 14 {
			score -= 10
			factors = append(factors, map[string]interface{}{
				"factor": "moderate_engagement",
				"impact": -10,
			})
		}
	}

	// Support tickets factor
	if tickets, ok := customer["support_tickets"].(float64); ok {
		if tickets > 5 {
			score -= 15
			factors = append(factors, map[string]interface{}{
				"factor": "high_support_volume",
				"impact": -15,
			})
		}
	}

	// NPS factor
	if nps, ok := customer["nps_score"].(float64); ok {
		if nps >= 9 {
			score += 10
			factors = append(factors, map[string]interface{}{
				"factor": "promoter",
				"impact": 10,
			})
		} else if nps <= 6 {
			score -= 15
			factors = append(factors, map[string]interface{}{
				"factor": "detractor",
				"impact": -15,
			})
		}
	}

	// Usage factor
	if usage, ok := customer["usage_percentage"].(float64); ok {
		if usage < 30 {
			score -= 15
			factors = append(factors, map[string]interface{}{
				"factor": "low_usage",
				"impact": -15,
			})
		} else if usage > 80 {
			score += 10
			factors = append(factors, map[string]interface{}{
				"factor": "high_usage",
				"impact": 10,
			})
		}
	}

	score = math.Max(0, math.Min(100, score))

	return map[string]interface{}{
		"health_score": score,
		"health_level": getHealthLevel(score),
		"factors":      factors,
		"trend":        calculateHealthTrend(customer),
		"recommendations": getHealthRecommendations(score, factors),
		"risk_level":   getRiskLevel(score),
	}
}

func getHealthLevel(score float64) string {
	if score >= 80 {
		return "excellent"
	} else if score >= 60 {
		return "good"
	} else if score >= 40 {
		return "fair"
	}
	return "poor"
}

func calculateHealthTrend(customer map[string]interface{}) string {
	// Simplified trend calculation
	if previousScore, ok := customer["previous_health_score"].(float64); ok {
		if currentScore, ok := customer["current_health_score"].(float64); ok {
			if currentScore > previousScore+10 {
				return "improving"
			} else if currentScore < previousScore-10 {
				return "declining"
			}
		}
	}
	return "stable"
}

func getHealthRecommendations(score float64, factors []map[string]interface{}) []string {
	recommendations := []string{}

	if score < 60 {
		recommendations = append(recommendations, "Schedule check-in call with customer")
	}

	for _, factor := range factors {
		if impact, ok := factor["impact"].(int); ok && impact < 0 {
			factorName, _ := factor["factor"].(string)
			switch factorName {
			case "low_engagement":
				recommendations = append(recommendations, "Send re-engagement campaign")
			case "high_support_volume":
				recommendations = append(recommendations, "Provide additional training")
			case "low_usage":
				recommendations = append(recommendations, "Share use case examples")
			}
		}
	}

	return recommendations
}

func getRiskLevel(score float64) string {
	if score < 40 {
		return "high"
	} else if score < 60 {
		return "medium"
	}
	return "low"
}

func analyzeFeedback(feedback []map[string]interface{}) map[string]interface{} {
	totalFeedback := len(feedback)
	sentimentCounts := map[string]int{
		"positive": 0,
		"neutral":  0,
		"negative": 0,
	}
	themes := map[string]int{}
	averageRating := 0.0

	for _, item := range feedback {
		// Analyze sentiment
		if text, ok := item["text"].(string); ok {
			sentiment := analyzeSentiment(text)
			if s, ok := sentiment["sentiment"].(string); ok {
				sentimentCounts[s]++
			}

			// Extract themes
			extractedThemes := extractThemes(text)
			for _, theme := range extractedThemes {
				themes[theme]++
			}
		}

		// Calculate average rating
		if rating, ok := item["rating"].(float64); ok {
			averageRating += rating
		}
	}

	if totalFeedback > 0 {
		averageRating /= float64(totalFeedback)
	}

	// Sort themes by frequency
	topThemes := getTopThemes(themes, 5)

	return map[string]interface{}{
		"total_feedback":   totalFeedback,
		"sentiment_distribution": sentimentCounts,
		"average_rating":   averageRating,
		"top_themes":       topThemes,
		"insights":         generateFeedbackInsights(sentimentCounts, averageRating),
		"action_items":     generateActionItems(sentimentCounts, topThemes),
	}
}

func extractThemes(text string) []string {
	text = strings.ToLower(text)
	themes := []string{}

	themeKeywords := map[string][]string{
		"performance": {"slow", "fast", "speed", "performance"},
		"usability":   {"easy", "difficult", "intuitive", "confusing", "user-friendly"},
		"features":    {"feature", "functionality", "capability"},
		"support":     {"support", "help", "service"},
		"pricing":     {"price", "cost", "expensive", "cheap"},
		"reliability": {"reliable", "crash", "bug", "stable"},
	}

	for theme, keywords := range themeKeywords {
		for _, keyword := range keywords {
			if strings.Contains(text, keyword) {
				themes = append(themes, theme)
				break
			}
		}
	}

	return themes
}

func getTopThemes(themes map[string]int, limit int) []map[string]interface{} {
	type themeCount struct {
		theme string
		count int
	}

	var themeCounts []themeCount
	for theme, count := range themes {
		themeCounts = append(themeCounts, themeCount{theme, count})
	}

	sort.Slice(themeCounts, func(i, j int) bool {
		return themeCounts[i].count > themeCounts[j].count
	})

	result := []map[string]interface{}{}
	for i := 0; i < len(themeCounts) && i < limit; i++ {
		result = append(result, map[string]interface{}{
			"theme": themeCounts[i].theme,
			"count": themeCounts[i].count,
		})
	}

	return result
}

func generateFeedbackInsights(sentimentCounts map[string]int, avgRating float64) []string {
	insights := []string{}

	total := sentimentCounts["positive"] + sentimentCounts["neutral"] + sentimentCounts["negative"]
	if total > 0 {
		positivePercent := float64(sentimentCounts["positive"]) / float64(total) * 100
		negativePercent := float64(sentimentCounts["negative"]) / float64(total) * 100

		if positivePercent > 60 {
			insights = append(insights, fmt.Sprintf("Strong positive sentiment (%.1f%%)", positivePercent))
		} else if negativePercent > 40 {
			insights = append(insights, fmt.Sprintf("Concerning negative sentiment (%.1f%%)", negativePercent))
		}
	}

	if avgRating >= 4.0 {
		insights = append(insights, "Excellent average rating")
	} else if avgRating < 3.0 {
		insights = append(insights, "Below average rating requires attention")
	}

	return insights
}

func generateActionItems(sentimentCounts map[string]int, topThemes []map[string]interface{}) []string {
	actions := []string{}

	if sentimentCounts["negative"] > sentimentCounts["positive"] {
		actions = append(actions, "Investigate root causes of negative feedback")
	}

	for _, theme := range topThemes {
		if themeName, ok := theme["theme"].(string); ok {
			if themeName == "support" {
				actions = append(actions, "Review support team training and processes")
			} else if themeName == "performance" {
				actions = append(actions, "Conduct performance optimization review")
			}
		}
	}

	return actions
}

func calculateNPS(scores []float64) map[string]interface{} {
	if len(scores) == 0 {
		return map[string]interface{}{
			"error": "No scores provided",
		}
	}

	promoters := 0
	passives := 0
	detractors := 0

	for _, score := range scores {
		if score >= 9 {
			promoters++
		} else if score >= 7 {
			passives++
		} else {
			detractors++
		}
	}

	total := len(scores)
	nps := (float64(promoters) - float64(detractors)) / float64(total) * 100

	return map[string]interface{}{
		"nps":           nps,
		"promoters":     promoters,
		"passives":      passives,
		"detractors":    detractors,
		"total_responses": total,
		"distribution": map[string]float64{
			"promoters_pct":  float64(promoters) / float64(total) * 100,
			"passives_pct":   float64(passives) / float64(total) * 100,
			"detractors_pct": float64(detractors) / float64(total) * 100,
		},
		"category":      getNPSCategory(nps),
		"benchmark":     "Industry average: 30-40",
		"recommendations": getNPSRecommendations(nps),
	}
}

func getNPSCategory(nps float64) string {
	if nps > 50 {
		return "excellent"
	} else if nps > 30 {
		return "good"
	} else if nps > 0 {
		return "needs_improvement"
	}
	return "critical"
}

func getNPSRecommendations(nps float64) []string {
	if nps < 30 {
		return []string{
			"Conduct customer interviews to understand pain points",
			"Implement immediate improvements to critical issues",
			"Launch customer success initiative",
		}
	} else if nps < 50 {
		return []string{
			"Identify and address detractor concerns",
			"Convert passives to promoters through engagement",
		}
	}
	return []string{
		"Maintain current service quality",
		"Encourage promoters to provide referrals",
	}
}

func analyzeSupportMetrics(metrics map[string]interface{}) map[string]interface{} {
	analysis := map[string]interface{}{}

	// First response time
	if frt, ok := metrics["first_response_time_hours"].(float64); ok {
		analysis["first_response_time"] = map[string]interface{}{
			"value":  frt,
			"status": getResponseTimeStatus(frt, 4.0),
			"target": 4.0,
		}
	}

	// Resolution time
	if rt, ok := metrics["avg_resolution_time_hours"].(float64); ok {
		analysis["resolution_time"] = map[string]interface{}{
			"value":  rt,
			"status": getResponseTimeStatus(rt, 24.0),
			"target": 24.0,
		}
	}

	// Resolution rate
	if rr, ok := metrics["resolution_rate"].(float64); ok {
		analysis["resolution_rate"] = map[string]interface{}{
			"value":  rr,
			"status": getResolutionRateStatus(rr),
			"target": 90.0,
		}
	}

	// CSAT
	if csat, ok := metrics["csat_score"].(float64); ok {
		analysis["csat"] = map[string]interface{}{
			"value":  csat,
			"status": getCSATStatus(csat),
			"target": 4.0,
		}
	}

	// Overall performance
	analysis["overall_performance"] = calculateOverallPerformance(analysis)
	analysis["recommendations"] = generateSupportRecommendations(analysis)

	return analysis
}

func getResponseTimeStatus(actual, target float64) string {
	if actual <= target {
		return "meeting_target"
	} else if actual <= target*1.5 {
		return "below_target"
	}
	return "needs_improvement"
}

func getResolutionRateStatus(rate float64) string {
	if rate >= 90 {
		return "excellent"
	} else if rate >= 75 {
		return "good"
	}
	return "needs_improvement"
}

func getCSATStatus(score float64) string {
	if score >= 4.5 {
		return "excellent"
	} else if score >= 4.0 {
		return "good"
	} else if score >= 3.5 {
		return "fair"
	}
	return "poor"
}

func calculateOverallPerformance(analysis map[string]interface{}) string {
	score := 0
	total := 0

	if frt, ok := analysis["first_response_time"].(map[string]interface{}); ok {
		if status, ok := frt["status"].(string); ok {
			total++
			if status == "meeting_target" {
				score++
			}
		}
	}

	if rr, ok := analysis["resolution_rate"].(map[string]interface{}); ok {
		if status, ok := rr["status"].(string); ok {
			total++
			if status == "excellent" || status == "good" {
				score++
			}
		}
	}

	if total == 0 {
		return "unknown"
	}

	percentage := float64(score) / float64(total) * 100
	if percentage >= 80 {
		return "excellent"
	} else if percentage >= 60 {
		return "good"
	}
	return "needs_improvement"
}

func generateSupportRecommendations(analysis map[string]interface{}) []string {
	recommendations := []string{}

	if frt, ok := analysis["first_response_time"].(map[string]interface{}); ok {
		if status, ok := frt["status"].(string); ok && status != "meeting_target" {
			recommendations = append(recommendations, "Improve first response time through better staffing or automation")
		}
	}

	if rr, ok := analysis["resolution_rate"].(map[string]interface{}); ok {
		if status, ok := rr["status"].(string); ok && status == "needs_improvement" {
			recommendations = append(recommendations, "Provide additional training to improve resolution rates")
		}
	}

	return recommendations
}

func analyzeCSAT(responses []map[string]interface{}) map[string]interface{} {
	if len(responses) == 0 {
		return map[string]interface{}{
			"error": "No responses provided",
		}
	}

	totalScore := 0.0
	scoreCounts := map[int]int{}
	categoryBreakdown := map[string]float64{}

	for _, response := range responses {
		if score, ok := response["score"].(float64); ok {
			totalScore += score
			scoreCounts[int(score)]++

			if category, ok := response["category"].(string); ok {
				categoryBreakdown[category] += score
			}
		}
	}

	avgCSAT := totalScore / float64(len(responses))

	return map[string]interface{}{
		"average_csat":       avgCSAT,
		"total_responses":    len(responses),
		"score_distribution": scoreCounts,
		"category_breakdown": categoryBreakdown,
		"status":             getCSATStatus(avgCSAT),
		"trend":              "stable",
		"insights":           generateCSATInsights(avgCSAT, scoreCounts),
	}
}

func generateCSATInsights(avgCSAT float64, scoreCounts map[int]int) []string {
	insights := []string{}

	if avgCSAT >= 4.5 {
		insights = append(insights, "Exceptional customer satisfaction")
	} else if avgCSAT < 3.5 {
		insights = append(insights, "Customer satisfaction below acceptable levels")
	}

	// Check for polarization
	if scoreCounts[5] > 0 && scoreCounts[1] > 0 {
		insights = append(insights, "Mixed feedback indicates inconsistent service quality")
	}

	return insights
}

func routeTicket(ticket map[string]interface{}, agents []map[string]interface{}) map[string]interface{} {
	// Get ticket classification
	classification := classifyTicket(ticket)
	category, _ := classification["category"].(string)
	priority, _ := classification["priority"].(string)

	// Find best agent
	bestAgent := findBestAgent(agents, category, priority)

	return map[string]interface{}{
		"recommended_agent": bestAgent,
		"routing_reason":    fmt.Sprintf("Best match for %s category with %s priority", category, priority),
		"estimated_response_time": estimateResponseTime(bestAgent, priority),
		"alternative_agents": findAlternativeAgents(agents, category),
	}
}

func findBestAgent(agents []map[string]interface{}, category, priority string) map[string]interface{} {
	if len(agents) == 0 {
		return map[string]interface{}{
			"name": "unassigned",
		}
	}

	// Simplified agent matching
	for _, agent := range agents {
		if specialties, ok := agent["specialties"].([]string); ok {
			for _, specialty := range specialties {
				if specialty == category {
					return agent
				}
			}
		}
	}

	// Return first available agent
	return agents[0]
}

func estimateResponseTime(agent map[string]interface{}, priority string) string {
	if priority == "high" {
		return "2-4 hours"
	} else if priority == "medium" {
		return "8-12 hours"
	}
	return "24-48 hours"
}

func findAlternativeAgents(agents []map[string]interface{}, category string) []string {
	alternatives := []string{}

	for _, agent := range agents {
		if name, ok := agent["name"].(string); ok {
			alternatives = append(alternatives, name)
		}
	}

	if len(alternatives) > 3 {
		return alternatives[:3]
	}

	return alternatives
}

func searchKnowledgeBase(query string, limit int) map[string]interface{} {
	query = strings.ToLower(query)

	// Mock knowledge base articles
	articles := []map[string]interface{}{
		{
			"id":       "kb001",
			"title":    "How to reset your password",
			"category": "account",
			"views":    1500,
		},
		{
			"id":       "kb002",
			"title":    "Troubleshooting login issues",
			"category": "technical",
			"views":    1200,
		},
		{
			"id":       "kb003",
			"title":    "Billing and payment FAQ",
			"category": "billing",
			"views":    900,
		},
	}

	// Simple keyword matching
	results := []map[string]interface{}{}
	for _, article := range articles {
		title := strings.ToLower(article["title"].(string))
		if strings.Contains(title, query) || strings.Contains(query, "password") && strings.Contains(title, "password") {
			article["relevance_score"] = calculateRelevance(query, title)
			results = append(results, article)
		}
	}

	// Sort by relevance
	sort.Slice(results, func(i, j int) bool {
		return results[i]["relevance_score"].(float64) > results[j]["relevance_score"].(float64)
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return map[string]interface{}{
		"results":       results,
		"total_results": len(results),
		"query":         query,
	}
}

func calculateRelevance(query, title string) float64 {
	queryWords := strings.Fields(query)
	titleWords := strings.Fields(title)

	matches := 0
	for _, qw := range queryWords {
		for _, tw := range titleWords {
			if qw == tw {
				matches++
			}
		}
	}

	if len(queryWords) == 0 {
		return 0
	}

	return float64(matches) / float64(len(queryWords)) * 100
}

func analyzeJourney(journey []map[string]interface{}) map[string]interface{} {
	totalTouchpoints := len(journey)
	satisfactionScores := []float64{}
	touchpointTypes := map[string]int{}
	painPoints := []map[string]interface{}{}

	for _, touchpoint := range journey {
		if tpType, ok := touchpoint["type"].(string); ok {
			touchpointTypes[tpType]++
		}

		if satisfaction, ok := touchpoint["satisfaction"].(float64); ok {
			satisfactionScores = append(satisfactionScores, satisfaction)

			if satisfaction < 3.0 {
				painPoints = append(painPoints, touchpoint)
			}
		}
	}

	avgSatisfaction := 0.0
	if len(satisfactionScores) > 0 {
		for _, score := range satisfactionScores {
			avgSatisfaction += score
		}
		avgSatisfaction /= float64(len(satisfactionScores))
	}

	return map[string]interface{}{
		"total_touchpoints":        totalTouchpoints,
		"average_satisfaction":     avgSatisfaction,
		"touchpoint_distribution":  touchpointTypes,
		"pain_points":              painPoints,
		"pain_point_count":         len(painPoints),
		"optimization_opportunities": identifyOptimizations(touchpointTypes, painPoints),
		"journey_health":           getJourneyHealth(avgSatisfaction),
	}
}

func identifyOptimizations(touchpoints map[string]int, painPoints []map[string]interface{}) []string {
	optimizations := []string{}

	// Check for too many touchpoints
	if len(touchpoints) > 10 {
		optimizations = append(optimizations, "Simplify customer journey by reducing touchpoints")
	}

	// Check for pain points
	if len(painPoints) > 3 {
		optimizations = append(optimizations, "Address multiple pain points to improve satisfaction")
	}

	return optimizations
}

func getJourneyHealth(avgSatisfaction float64) string {
	if avgSatisfaction >= 4.0 {
		return "healthy"
	} else if avgSatisfaction >= 3.0 {
		return "moderate"
	}
	return "needs_attention"
}
