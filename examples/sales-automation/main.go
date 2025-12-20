package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Ranganaths/minion/core"
	"github.com/Ranganaths/minion/models"
	"github.com/Ranganaths/minion/storage"
	"github.com/Ranganaths/minion/tools/domains"
)

/*
Sales Pipeline Automation Agent Example

This example demonstrates intelligent sales automation:
1. Lead scoring and qualification
2. Automated follow-up emails
3. Deal progression tracking
4. Revenue forecasting
5. Slack notifications for sales team
6. CRM data synchronization

Real-world benefits:
- Automated lead nurturing
- Never miss a follow-up
- Data-driven deal prioritization
- Real-time revenue insights
- Improved sales team collaboration
*/

func main() {
	log.Println("💰 Starting Sales Automation Agent...")

	// Initialize framework
	framework := core.NewFramework(
		core.WithStorage(storage.NewInMemory()),
	)

	// Register all tools
	if err := domains.RegisterAllDomainTools(framework); err != nil {
		log.Fatal("Failed to register tools:", err)
	}

	ctx := context.Background()

	// Create Sales Automation Agent
	agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:        "Sales AI Agent",
		Description: "Automates sales pipeline and lead management",
		Capabilities: []string{
			"gmail_integration",
			"slack_integration",
			"revenue_analysis",
			"pipeline_analysis",
			"deal_scoring",
			"forecasting",
			"sales_analytics",
			"communication",
		},
		Metadata: map[string]interface{}{
			"team":   "sales",
			"region": "north-america",
			"quota":  1000000,
		},
	})

	if err != nil {
		log.Fatal("Failed to create agent:", err)
	}

	log.Printf("✅ Created agent: %s (ID: %s)\n", agent.Name, agent.ID)

	// Run sales workflows
	runLeadQualificationWorkflow(ctx, framework)
	runDealScoringWorkflow(ctx, framework)
	runRevenueForecasting(ctx, framework)
	runAutomatedFollowUpWorkflow(ctx, framework)

	log.Println("\n✅ Sales automation completed successfully!")
}

func printSeparator(title string) {
	sep := strings.Repeat("=", 60)
	log.Println("\n" + sep)
	log.Println(title)
	log.Println(sep + "\n")
}

// runLeadQualificationWorkflow demonstrates lead scoring and qualification
func runLeadQualificationWorkflow(ctx context.Context, framework *core.FrameworkImpl) {
	printSeparator("🎯 WORKFLOW 1: Lead Qualification & Scoring")

	// Simulate incoming leads
	leads := []map[string]interface{}{
		{
			"email":        "ceo@bigcorp.com",
			"company":      "BigCorp Inc",
			"job_title":    "CEO",
			"company_size": "1000+",
			"budget":       "high",
			"engagement":   85.0,
			"source":       "referral",
		},
		{
			"email":        "manager@smallbiz.com",
			"company":      "Small Business",
			"job_title":    "Manager",
			"company_size": "10-50",
			"budget":       "low",
			"engagement":   45.0,
			"source":       "website",
		},
		{
			"email":        "director@midsize.com",
			"company":      "MidSize Co",
			"job_title":    "Director",
			"company_size": "200-500",
			"budget":       "medium",
			"engagement":   70.0,
			"source":       "webinar",
		},
	}

	log.Printf("📋 Processing %d new leads...\n", len(leads))

	qualifiedLeads := []map[string]interface{}{}

	for i, lead := range leads {
		log.Printf("\n--- Lead %d/%d ---\n", i+1, len(leads))
		log.Printf("Company: %s\n", lead["company"])
		log.Printf("Contact: %s (%s)\n", lead["email"], lead["job_title"])

		// Score the lead
		log.Println("\n🎯 Scoring lead...")

		// Create lead data for scoring tool
		leadData := map[string]interface{}{
			"company_size": getCompanySizeScore(lead["company_size"].(string)),
			"job_title":    getTitleScore(lead["job_title"].(string)),
			"budget":       getBudgetScore(lead["budget"].(string)),
			"engagement":   lead["engagement"].(float64),
			"source":       getSourceScore(lead["source"].(string)),
		}

		// Calculate lead score (simplified scoring logic)
		totalScore := 0.0
		for _, v := range leadData {
			if score, ok := v.(float64); ok {
				totalScore += score
			}
		}
		totalScore = totalScore / float64(len(leadData))

		leadQuality := getLeadQuality(totalScore)

		log.Printf("Lead Score: %.1f/100\n", totalScore)
		log.Printf("Quality: %s\n", leadQuality)

		// Add metadata to lead
		lead["score"] = totalScore
		lead["quality"] = leadQuality

		// Qualify high-scoring leads
		if totalScore >= 70 {
			qualifiedLeads = append(qualifiedLeads, lead)

			log.Println("\n✅ Lead qualified! Creating opportunity...")

			// Send welcome email
			_, _ = framework.ExecuteTool(ctx, "gmail_send_email", map[string]interface{}{
				"to":      lead["email"].(string),
				"subject": fmt.Sprintf("Welcome %s!", lead["company"]),
				"body": fmt.Sprintf(`
Hi,

Thank you for your interest in our solution! Based on your profile, I believe we can provide significant value to %s.

I'd love to schedule a quick 15-minute call to understand your needs better.

Are you available this week?

Best regards,
Sales Team
				`, lead["company"]),
			})

			log.Println("📧 Sent welcome email")

			// Notify sales team on Slack
			_, _ = framework.ExecuteTool(ctx, "slack_send_message", map[string]interface{}{
				"channel": "#sales-qualified-leads",
				"message": "🎯 New Qualified Lead!",
				"attachments": []map[string]interface{}{
					{
						"color": "#00ff00",
						"title": lead["company"].(string),
						"fields": []map[string]interface{}{
							{
								"title": "Contact",
								"value": fmt.Sprintf("%s (%s)", lead["email"], lead["job_title"]),
								"short": false,
							},
							{
								"title": "Lead Score",
								"value": fmt.Sprintf("%.1f/100", totalScore),
								"short": true,
							},
							{
								"title": "Quality",
								"value": leadQuality,
								"short": true,
							},
							{
								"title": "Company Size",
								"value": lead["company_size"].(string),
								"short": true,
							},
							{
								"title": "Source",
								"value": lead["source"].(string),
								"short": true,
							},
						},
						"footer": "Sales Automation",
						"ts":     time.Now().Unix(),
					},
				},
			})

			log.Println("✅ Sales team notified")

		} else {
			log.Printf("❌ Lead score too low (%.1f < 70) - Added to nurture campaign\n", totalScore)

			// Add to nurture email campaign
			_, _ = framework.ExecuteTool(ctx, "gmail_send_email", map[string]interface{}{
				"to":      lead["email"].(string),
				"subject": "Resources that might interest you",
				"body": `
Hi,

Thank you for your interest! Here are some resources that might help:

• Case Studies
• Product Demo Video
• ROI Calculator

Feel free to reach out if you have any questions!

Best regards,
Marketing Team
				`,
			})

			log.Println("📧 Added to nurture campaign")
		}
	}

	log.Printf("\n📊 Summary: %d/%d leads qualified\n", len(qualifiedLeads), len(leads))
	log.Println("✅ Lead qualification workflow completed!")
}

// runDealScoringWorkflow prioritizes deals in the pipeline
func runDealScoringWorkflow(ctx context.Context, framework *core.FrameworkImpl) {
	printSeparator("💼 WORKFLOW 2: Deal Scoring & Prioritization")

	// Simulate active deals
	deals := []map[string]interface{}{
		{
			"id":       "DEAL-001",
			"company":  "Enterprise Corp",
			"value":    150000.0,
			"stage":    "negotiation",
			"age_days": 25.0,
		},
		{
			"id":       "DEAL-002",
			"company":  "Startup Inc",
			"value":    25000.0,
			"stage":    "qualification",
			"age_days": 95.0,
		},
		{
			"id":       "DEAL-003",
			"company":  "MidMarket LLC",
			"value":    75000.0,
			"stage":    "proposal",
			"age_days": 40.0,
		},
	}

	log.Printf("📋 Scoring %d deals in pipeline...\n", len(deals))

	for _, deal := range deals {
		log.Printf("\n--- %s: %s ---\n", deal["id"], deal["company"])

		// Score the deal
		scoreOutput, _ := framework.ExecuteTool(ctx, "deal_scoring", map[string]interface{}{
			"deal": deal,
		})

		if scoreOutput == nil || scoreOutput.Result == nil {
			log.Println("❌ Failed to score deal")
			continue
		}

		result := scoreOutput.Result.(map[string]interface{})
		score := result["score"].(float64)
		priority := result["priority"].(string)

		log.Printf("Deal Score: %.1f/100\n", score)
		log.Printf("Priority: %s\n", priority)

		deal["score"] = score
		deal["priority"] = priority

		// Show recommendations if available
		if recs, ok := result["recommended"].([]string); ok && len(recs) > 0 {
			log.Println("\n💡 Recommendations:")
			for _, rec := range recs {
				log.Printf("  • %s\n", rec)
			}
		}

		// Alert for high-priority deals
		if priority == "high" {
			_, _ = framework.ExecuteTool(ctx, "slack_send_message", map[string]interface{}{
				"channel": "#sales-team",
				"message": "🚀 High Priority Deal Alert!",
				"attachments": []map[string]interface{}{
					{
						"color": "#ff9900",
						"title": fmt.Sprintf("%s - $%.0f", deal["company"], deal["value"]),
						"fields": []map[string]interface{}{
							{
								"title": "Deal ID",
								"value": deal["id"],
								"short": true,
							},
							{
								"title": "Score",
								"value": fmt.Sprintf("%.1f/100", score),
								"short": true,
							},
							{
								"title": "Stage",
								"value": deal["stage"],
								"short": true,
							},
							{
								"title": "Age",
								"value": fmt.Sprintf("%.0f days", deal["age_days"]),
								"short": true,
							},
						},
					},
				},
			})
		}
	}

	log.Println("\n✅ Deal scoring workflow completed!")
}

// runRevenueForecasting generates revenue forecasts
func runRevenueForecasting(ctx context.Context, framework *core.FrameworkImpl) {
	printSeparator("📈 WORKFLOW 3: Revenue Forecasting")

	// Historical revenue data (monthly)
	historicalRevenue := []float64{
		850000, 920000, 880000, 950000, 1020000, 980000,
	}

	log.Println("📊 Generating revenue forecast...")
	log.Printf("Historical data: %d months\n", len(historicalRevenue))

	// Generate forecast
	forecastOutput, _ := framework.ExecuteTool(ctx, "sales_forecasting", map[string]interface{}{
		"historical_data": historicalRevenue,
		"periods":         3,
	})

	if forecastOutput == nil || forecastOutput.Result == nil {
		log.Println("❌ Failed to generate forecast")
		return
	}

	forecast := forecastOutput.Result.(map[string]interface{})
	predictions, _ := forecast["forecast"].([]float64)
	confidence, _ := forecast["confidence"].(float64)
	trend, _ := forecast["trend"].(float64)

	if predictions == nil || len(predictions) < 3 {
		log.Println("❌ Invalid forecast data")
		return
	}

	log.Println("\n📈 Forecast Results:")
	log.Printf("Confidence: %.1f%%\n", confidence*100)
	log.Printf("Trend: $%.0f/month\n", trend)
	log.Println("\nNext 3 months forecast:")

	totalForecast := 0.0
	for i, pred := range predictions {
		log.Printf("  Month %d: $%.0f\n", i+1, pred)
		totalForecast += pred
	}

	log.Printf("\nTotal forecasted revenue (Q): $%.0f\n", totalForecast)

	// Send forecast to leadership
	log.Println("\n📧 Sending forecast to sales leadership...")

	_, _ = framework.ExecuteTool(ctx, "gmail_send_email", map[string]interface{}{
		"to":      "sales-leadership@company.com",
		"subject": fmt.Sprintf("Q%d Revenue Forecast - $%.0fK", time.Now().Month()/3+1, totalForecast/1000),
		"body": fmt.Sprintf(`
Sales Leadership Team,

Based on the last 6 months of data, here is the revenue forecast for the next quarter:

Historical Average: $%.0fK/month
Forecast Q Total: $%.0fK
Monthly Trend: $%.0fK
Confidence Level: %.1f%%

Month-by-month breakdown:
• Month 1: $%.0fK
• Month 2: $%.0fK
• Month 3: $%.0fK

The upward trend suggests we're on track to meet our quarterly target.

Best regards,
Sales Analytics
		`, sum(historicalRevenue)/float64(len(historicalRevenue))/1000,
			totalForecast/1000, trend/1000, confidence*100,
			predictions[0]/1000, predictions[1]/1000, predictions[2]/1000),
		"is_html": false,
	})

	// Post to Slack
	_, _ = framework.ExecuteTool(ctx, "slack_send_message", map[string]interface{}{
		"channel": "#sales-analytics",
		"message": "📈 Revenue Forecast Updated",
		"attachments": []map[string]interface{}{
			{
				"color": "#0000ff",
				"title": fmt.Sprintf("Q%d Forecast: $%.0fK", time.Now().Month()/3+1, totalForecast/1000),
				"fields": []map[string]interface{}{
					{
						"title": "Confidence",
						"value": fmt.Sprintf("%.1f%%", confidence*100),
						"short": true,
					},
					{
						"title": "Monthly Trend",
						"value": fmt.Sprintf("$%.0fK", trend/1000),
						"short": true,
					},
				},
			},
		},
	})

	log.Println("✅ Revenue forecasting workflow completed!")
}

// runAutomatedFollowUpWorkflow manages deal follow-ups
func runAutomatedFollowUpWorkflow(ctx context.Context, framework *core.FrameworkImpl) {
	printSeparator("⏰ WORKFLOW 4: Automated Follow-Up Management")

	// Simulate deals requiring follow-up
	dealsToFollow := []map[string]interface{}{
		{
			"id":            "DEAL-001",
			"company":       "TechCorp",
			"contact_email": "buyer@techcorp.com",
			"contact_name":  "Jane Smith",
			"last_contact":  7, // days ago
			"stage":         "proposal",
			"deal_value":    120000.0,
		},
		{
			"id":            "DEAL-002",
			"company":       "InnovateLabs",
			"contact_email": "cto@innovatelabs.com",
			"contact_name":  "Bob Johnson",
			"last_contact":  14,
			"stage":         "negotiation",
			"deal_value":    85000.0,
		},
	}

	log.Printf("📋 Processing %d deals for follow-up...\n", len(dealsToFollow))

	for _, deal := range dealsToFollow {
		log.Printf("\n--- %s: %s ---\n", deal["id"], deal["company"])

		lastContact := deal["last_contact"].(int)
		log.Printf("Last contact: %d days ago\n", lastContact)

		// Determine follow-up message based on time elapsed
		var subject, body string

		if lastContact >= 14 {
			subject = fmt.Sprintf("Checking in - %s proposal", deal["company"])
			body = fmt.Sprintf(`
Hi %s,

I wanted to follow up on the proposal we sent for %s.

Have you had a chance to review it? I'd be happy to address any questions or concerns.

Would you be available for a quick call this week to discuss?

Best regards,
Sales Team
			`, deal["contact_name"], deal["company"])
		} else if lastContact >= 7 {
			subject = fmt.Sprintf("Quick question about %s", deal["company"])
			body = fmt.Sprintf(`
Hi %s,

Just wanted to check if you need any additional information about our proposal.

I'm here to help with any questions!

Best regards,
Sales Team
			`, deal["contact_name"])
		}

		log.Println("📧 Sending follow-up email...")

		// Send follow-up email
		_, _ = framework.ExecuteTool(ctx, "gmail_send_email", map[string]interface{}{
			"to":      deal["contact_email"].(string),
			"subject": subject,
			"body":    body,
		})

		log.Printf("✅ Follow-up sent to %s\n", deal["contact_name"])

		// Log activity to Slack
		_, _ = framework.ExecuteTool(ctx, "slack_send_message", map[string]interface{}{
			"channel": "#sales-activity",
			"message": fmt.Sprintf("📧 Follow-up sent: %s (%s) - Deal value: $%.0f",
				deal["company"], deal["id"], deal["deal_value"]),
		})

		// Schedule reminder for next follow-up
		nextFollowUp := 7 // days
		log.Printf("⏰ Next follow-up scheduled in %d days\n", nextFollowUp)
	}

	log.Println("\n✅ Follow-up workflow completed!")
}

// Helper functions

func getCompanySizeScore(size string) float64 {
	scores := map[string]float64{
		"1-10":    20,
		"10-50":   40,
		"50-200":  60,
		"200-500": 80,
		"500+":    90,
		"1000+":   100,
	}
	if score, ok := scores[size]; ok {
		return score
	}
	return 50
}

func getTitleScore(title string) float64 {
	title = fmt.Sprintf("%v", title)
	if containsIgnoreCase(title, "ceo") || containsIgnoreCase(title, "founder") {
		return 100
	} else if containsIgnoreCase(title, "cto") || containsIgnoreCase(title, "vp") || containsIgnoreCase(title, "director") {
		return 80
	} else if containsIgnoreCase(title, "manager") {
		return 60
	}
	return 40
}

func getBudgetScore(budget string) float64 {
	scores := map[string]float64{
		"low":    30,
		"medium": 60,
		"high":   100,
	}
	if score, ok := scores[budget]; ok {
		return score
	}
	return 50
}

func getSourceScore(source string) float64 {
	scores := map[string]float64{
		"referral": 100,
		"partner":  90,
		"webinar":  70,
		"event":    60,
		"website":  40,
		"cold":     20,
	}
	if score, ok := scores[source]; ok {
		return score
	}
	return 50
}

func getLeadQuality(score float64) string {
	if score >= 80 {
		return "Hot"
	} else if score >= 60 {
		return "Warm"
	}
	return "Cold"
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func sum(values []float64) float64 {
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total
}
