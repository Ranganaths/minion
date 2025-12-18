package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/minion/core"
	"github.com/yourusername/minion/models"
	"github.com/yourusername/minion/storage"
	"github.com/yourusername/minion/tools/domains"
)

/*
Customer Support Automation Agent Example

This example demonstrates an intelligent customer support automation system:
1. Monitor Gmail for customer inquiries
2. Analyze sentiment and classify tickets
3. Route to appropriate support agents
4. Send automated responses
5. Track customer health scores
6. Send follow-up emails and satisfaction surveys

Real-world benefits:
- 24/7 automated initial response
- Intelligent ticket routing
- Proactive customer health monitoring
- Reduced response times
- Improved customer satisfaction
*/

func main() {
	log.Println("🎧 Starting Customer Support Automation Agent...")

	// Initialize framework
	framework := core.NewFramework(
		core.WithStorage(storage.NewInMemory()),
	)

	// Register all tools
	if err := domains.RegisterAllDomainTools(framework); err != nil {
		log.Fatal("Failed to register tools:", err)
	}

	ctx := context.Background()

	// Create Customer Support Agent
	agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:        "Customer Support AI Agent",
		Description: "Automates customer support workflows with intelligent routing",
		Capabilities: []string{
			"gmail_integration",
			"slack_integration",
			"sentiment_analysis",
			"ticket_classification",
			"customer_health",
			"response_generation",
			"communication",
			"customer_support",
		},
		Metadata: map[string]interface{}{
			"team":       "customer-success",
			"languages":  []string{"en", "es", "fr"},
			"work_hours": "24/7",
		},
	})

	if err != nil {
		log.Fatal("Failed to create agent:", err)
	}

	log.Printf("✅ Created agent: %s (ID: %s)\n", agent.Name, agent.ID)

	// Run support workflows
	runIncomingEmailWorkflow(ctx, framework)
	runTicketRoutingWorkflow(ctx, framework)
	runCustomerHealthMonitoring(ctx, framework)
	runSatisfactionSurveyWorkflow(ctx, framework)

	log.Println("\n✅ Customer support automation completed!")
}

// runIncomingEmailWorkflow handles incoming customer emails
func runIncomingEmailWorkflow(ctx context.Context, framework core.Framework) {
	log.Println("\n" + "="*60)
	log.Println("📧 WORKFLOW 1: Incoming Email Processing")
	log.Println("="*60 + "\n")

	// Step 1: Search for unread customer emails
	log.Println("🔍 Searching for new customer emails...")

	emailOutput, _ := framework.ExecuteTool(ctx, "gmail_search", &models.ToolInput{
		Params: map[string]interface{}{
			"query":       "is:unread label:customer-support",
			"max_results": 5,
		},
	})

	emails := emailOutput.Result.(map[string]interface{})["messages"].([]map[string]interface{})
	log.Printf("📬 Found %d new customer emails\n", len(emails))

	// Process each email
	for i, email := range emails {
		log.Printf("\n--- Processing Email %d/%d ---\n", i+1, len(emails))

		customerEmail := email["from"].(string)
		subject := email["subject"].(string)
		snippet := email["snippet"].(string)

		log.Printf("From: %s\n", customerEmail)
		log.Printf("Subject: %s\n", subject)

		// Step 2: Analyze sentiment
		log.Println("\n🎭 Analyzing customer sentiment...")

		sentimentOutput, _ := framework.ExecuteTool(ctx, "sentiment_analyzer", &models.ToolInput{
			Params: map[string]interface{}{
				"text": snippet,
			},
		})

		sentiment := sentimentOutput.Result.(map[string]interface{})
		sentimentType := sentiment["sentiment"].(string)
		score := sentiment["score"].(float64)

		log.Printf("Sentiment: %s (score: %.2f)\n", sentimentType, score)

		// Step 3: Classify the ticket
		log.Println("\n🏷️  Classifying ticket...")

		ticketOutput, _ := framework.ExecuteTool(ctx, "ticket_classifier", &models.ToolInput{
			Params: map[string]interface{}{
				"ticket": map[string]interface{}{
					"subject":     subject,
					"description": snippet,
				},
			},
		})

		classification := ticketOutput.Result.(map[string]interface{})
		category := classification["category"].(string)
		priority := classification["priority"].(string)
		urgency := classification["urgency"].(string)

		log.Printf("Category: %s | Priority: %s | Urgency: %s\n", category, priority, urgency)

		// Step 4: Generate automated response
		log.Println("\n✍️  Generating automated response...")

		responseOutput, _ := framework.ExecuteTool(ctx, "response_generator", &models.ToolInput{
			Params: map[string]interface{}{
				"context": map[string]interface{}{
					"customer_name": "Valued Customer",
					"issue_type":    category,
					"sentiment":     sentimentType,
				},
				"response_type": category,
			},
		})

		response := responseOutput.Result.(map[string]interface{})["response"].(string)

		// Step 5: Send automated reply
		log.Println("\n📤 Sending automated acknowledgment...")

		framework.ExecuteTool(ctx, "gmail_send_email", &models.ToolInput{
			Params: map[string]interface{}{
				"to":      customerEmail,
				"subject": "Re: " + subject,
				"body":    response + "\n\nTicket ID: SUPP-" + fmt.Sprintf("%d", time.Now().Unix()),
				"is_html": false,
			},
		})

		log.Println("✅ Sent automated response")

		// Step 6: Notify support team on Slack if urgent
		if urgency == "high" || sentimentType == "negative" {
			log.Println("\n🚨 High priority - Notifying support team...")

			framework.ExecuteTool(ctx, "slack_send_message", &models.ToolInput{
				Params: map[string]interface{}{
					"channel": "#support-urgent",
					"message": "🚨 Urgent customer ticket requires immediate attention!",
					"attachments": []map[string]interface{}{
						{
							"color": "#ff0000",
							"title": subject,
							"fields": []map[string]interface{}{
								{
									"title": "Customer",
									"value": customerEmail,
									"short": true,
								},
								{
									"title": "Category",
									"value": category,
									"short": true,
								},
								{
									"title": "Sentiment",
									"value": sentimentType,
									"short": true,
								},
								{
									"title": "Priority",
									"value": priority,
									"short": true,
								},
							},
						},
					},
				},
			})

			log.Println("✅ Support team notified")
		}
	}

	log.Println("\n✅ Email processing workflow completed!")
}

// runTicketRoutingWorkflow demonstrates intelligent ticket routing
func runTicketRoutingWorkflow(ctx context.Context, framework core.Framework) {
	log.Println("\n" + "="*60)
	log.Println("🎯 WORKFLOW 2: Intelligent Ticket Routing")
	log.Println("="*60 + "\n")

	// Simulate incoming tickets with different categories
	tickets := []map[string]interface{}{
		{
			"id":      "TICK-001",
			"subject": "Cannot access my account after password reset",
			"customer": "john@example.com",
		},
		{
			"id":      "TICK-002",
			"subject": "Billing: Charged twice for subscription",
			"customer": "jane@example.com",
		},
		{
			"id":      "TICK-003",
			"subject": "Feature request: Dark mode support",
			"customer": "alice@example.com",
		},
	}

	// Available support agents
	agents := []map[string]interface{}{
		{
			"name":        "Agent Smith",
			"specialties": []string{"technical", "account"},
			"current_load": 3,
		},
		{
			"name":        "Agent Jones",
			"specialties": []string{"billing", "payments"},
			"current_load": 5,
		},
		{
			"name":        "Agent Brown",
			"specialties": []string{"feature_request", "product"},
			"current_load": 2,
		},
	}

	log.Printf("📋 Processing %d tickets for routing...\n", len(tickets))

	for _, ticket := range tickets {
		log.Printf("\n--- Routing Ticket %s ---\n", ticket["id"])
		log.Printf("Subject: %s\n", ticket["subject"])

		// Classify the ticket
		classifyOutput, _ := framework.ExecuteTool(ctx, "ticket_classifier", &models.ToolInput{
			Params: map[string]interface{}{
				"ticket": ticket,
			},
		})

		classification := classifyOutput.Result.(map[string]interface{})
		category := classification["category"].(string)

		log.Printf("Category: %s\n", category)

		// Route to best agent
		routingOutput, _ := framework.ExecuteTool(ctx, "ticket_router", &models.ToolInput{
			Params: map[string]interface{}{
				"ticket":           ticket,
				"available_agents": agents,
			},
		})

		routing := routingOutput.Result.(map[string]interface{})
		recommendedAgent := routing["recommended_agent"].(map[string]interface{})

		log.Printf("✅ Routed to: %s\n", recommendedAgent["name"])
		log.Printf("Reason: %s\n", routing["routing_reason"])

		// Notify assigned agent via Slack
		framework.ExecuteTool(ctx, "slack_send_message", &models.ToolInput{
			Params: map[string]interface{}{
				"channel": "@" + recommendedAgent["name"].(string),
				"message": fmt.Sprintf("📋 New ticket assigned: %s\n\nSubject: %s\nCustomer: %s\nCategory: %s",
					ticket["id"], ticket["subject"], ticket["customer"], category),
			},
		})
	}

	log.Println("\n✅ Ticket routing workflow completed!")
}

// runCustomerHealthMonitoring monitors and scores customer health
func runCustomerHealthMonitoring(ctx context.Context, framework core.Framework) {
	log.Println("\n" + "="*60)
	log.Println("💚 WORKFLOW 3: Customer Health Monitoring")
	log.Println("="*60 + "\n")

	// Simulate customer data
	customers := []map[string]interface{}{
		{
			"id":                   "CUST-001",
			"name":                 "Acme Corp",
			"last_activity_days":   5,
			"support_tickets":      1,
			"nps_score":            9.0,
			"usage_percentage":     85.0,
		},
		{
			"id":                   "CUST-002",
			"name":                 "Beta Inc",
			"last_activity_days":   45,
			"support_tickets":      8,
			"nps_score":            5.0,
			"usage_percentage":     25.0,
		},
		{
			"id":                   "CUST-003",
			"name":                 "Gamma LLC",
			"last_activity_days":   15,
			"support_tickets":      3,
			"nps_score":            7.0,
			"usage_percentage":     60.0,
		},
	}

	log.Printf("🔍 Analyzing health scores for %d customers...\n", len(customers))

	atRiskCustomers := []map[string]interface{}{}

	for _, customer := range customers {
		log.Printf("\n--- Customer: %s ---\n", customer["name"])

		// Calculate health score
		healthOutput, _ := framework.ExecuteTool(ctx, "customer_health_scorer", &models.ToolInput{
			Params: map[string]interface{}{
				"customer": customer,
			},
		})

		health := healthOutput.Result.(map[string]interface{})
		score := health["health_score"].(float64)
		level := health["health_level"].(string)
		riskLevel := health["risk_level"].(string)

		log.Printf("Health Score: %.1f/100 (%s)\n", score, level)
		log.Printf("Risk Level: %s\n", riskLevel)

		// Check for at-risk customers
		if riskLevel == "high" || riskLevel == "medium" {
			atRiskCustomers = append(atRiskCustomers, customer)

			// Get recommendations
			recommendations := health["recommendations"].([]string)
			log.Println("\n📋 Recommended Actions:")
			for _, rec := range recommendations {
				log.Printf("  • %s\n", rec)
			}

			// Send alert to customer success team
			framework.ExecuteTool(ctx, "slack_send_message", &models.ToolInput{
				Params: map[string]interface{}{
					"channel": "#customer-success",
					"message": fmt.Sprintf("⚠️  Customer health alert: %s", customer["name"]),
					"attachments": []map[string]interface{}{
						{
							"color": getColorForRisk(riskLevel),
							"fields": []map[string]interface{}{
								{
									"title": "Health Score",
									"value": fmt.Sprintf("%.1f", score),
									"short": true,
								},
								{
									"title": "Risk Level",
									"value": riskLevel,
									"short": true,
								},
								{
									"title": "Last Activity",
									"value": fmt.Sprintf("%d days ago", int(customer["last_activity_days"].(float64))),
									"short": true,
								},
								{
									"title": "Support Tickets",
									"value": fmt.Sprintf("%d", int(customer["support_tickets"].(float64))),
									"short": true,
								},
							},
						},
					},
				},
			})
		}
	}

	log.Printf("\n📊 Summary: %d/%d customers need attention\n", len(atRiskCustomers), len(customers))
	log.Println("✅ Customer health monitoring completed!")
}

// runSatisfactionSurveyWorkflow sends automated satisfaction surveys
func runSatisfactionSurveyWorkflow(ctx context.Context, framework core.Framework) {
	log.Println("\n" + "="*60)
	log.Println("📊 WORKFLOW 4: Customer Satisfaction Survey")
	log.Println("="*60 + "\n")

	// Simulate recently closed tickets
	closedTickets := []map[string]interface{}{
		{
			"id":       "TICK-101",
			"customer": "satisfied@example.com",
			"subject":  "Password reset issue",
			"resolved_by": "Agent Smith",
			"resolution_time_hours": 2,
		},
		{
			"id":       "TICK-102",
			"customer": "happy@example.com",
			"subject":  "Billing question",
			"resolved_by": "Agent Jones",
			"resolution_time_hours": 4,
		},
	}

	log.Printf("📋 Sending satisfaction surveys for %d resolved tickets...\n", len(closedTickets))

	for _, ticket := range closedTickets {
		log.Printf("\n--- Ticket %s ---\n", ticket["id"])

		surveyLink := fmt.Sprintf("https://survey.company.com/%s", ticket["id"])

		// Send survey email
		log.Println("📧 Sending satisfaction survey...")

		framework.ExecuteTool(ctx, "gmail_send_email", &models.ToolInput{
			Params: map[string]interface{}{
				"to":      ticket["customer"].(string),
				"subject": "How was your support experience?",
				"body": fmt.Sprintf(`
Dear Customer,

Thank you for contacting our support team. Your ticket "%s" has been resolved by %s.

We'd love to hear about your experience! Please take a moment to complete our quick survey:

%s

Your feedback helps us improve our service.

Best regards,
Customer Support Team
				`, ticket["subject"], ticket["resolved_by"], surveyLink),
				"is_html": false,
			},
		})

		log.Printf("✅ Survey sent to %s\n", ticket["customer"])

		// Log to Slack
		framework.ExecuteTool(ctx, "slack_send_message", &models.ToolInput{
			Params: map[string]interface{}{
				"channel": "#support-metrics",
				"message": fmt.Sprintf("📊 CSAT survey sent for ticket %s (resolved in %d hours)",
					ticket["id"], int(ticket["resolution_time_hours"].(float64))),
			},
		})
	}

	log.Println("\n✅ Satisfaction survey workflow completed!")
}

// Helper function
func getColorForRisk(riskLevel string) string {
	switch riskLevel {
	case "high":
		return "#ff0000"
	case "medium":
		return "#ffa500"
	default:
		return "#00ff00"
	}
}
