package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Ranganaths/minion/core/multiagent"
	"github.com/Ranganaths/minion/llm"
)

/*
Business Automation Example

This example demonstrates a business automation workflow using the multi-agent system:
1. Sales Lead Analysis - Analyze incoming sales leads
2. Content Generation - Generate marketing content
3. Data Analysis - Analyze business metrics
4. Code Generation - Generate automation scripts

Each task is handled by specialized worker agents coordinated by the system.
*/

// LLMAdapter adapts the minion LLM provider to the multiagent interface
type LLMAdapter struct {
	provider llm.Provider
}

func (l *LLMAdapter) GenerateCompletion(ctx context.Context, req *multiagent.CompletionRequest) (*multiagent.CompletionResponse, error) {
	resp, err := l.provider.GenerateCompletion(ctx, &llm.CompletionRequest{
		SystemPrompt: req.SystemPrompt,
		UserPrompt:   req.UserPrompt,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		Model:        req.Model,
	})
	if err != nil {
		return nil, err
	}

	return &multiagent.CompletionResponse{
		Text:         resp.Text,
		TokensUsed:   resp.TokensUsed,
		Model:        resp.Model,
		FinishReason: resp.FinishReason,
	}, nil
}

func main() {
	ctx := context.Background()

	// Get API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	// Create LLM adapter
	llmProvider := &LLMAdapter{
		provider: llm.NewOpenAI(apiKey),
	}

	fmt.Println("🏢 Business Automation Multi-Agent System")
	fmt.Println("=========================================")
	fmt.Println()

	// Create coordinator with default configuration
	coordinator := multiagent.NewCoordinator(llmProvider, nil)

	// Initialize with default workers
	if err := coordinator.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize coordinator: %v", err)
	}

	fmt.Println("✅ Multi-agent system initialized")
	fmt.Printf("   Workers registered: %d\n\n", len(coordinator.GetWorkers()))

	// List registered workers
	fmt.Println("📋 Available Workers:")
	for _, worker := range coordinator.GetWorkers() {
		fmt.Printf("   - Agent %s (Role: %s)\n", worker.AgentID[:8], worker.Role)
		fmt.Printf("     Capabilities: %v\n", worker.Capabilities)
	}
	fmt.Println()

	// Run business automation workflows
	runSalesLeadAnalysis(ctx, coordinator)
	runContentGeneration(ctx, coordinator)
	runBusinessMetricsAnalysis(ctx, coordinator)
	runAutomationScriptGeneration(ctx, coordinator)

	// Final statistics
	printFinalStats(ctx, coordinator)

	// Graceful shutdown
	fmt.Println("\n🛑 Shutting down...")
	if err := coordinator.Shutdown(ctx); err != nil {
		log.Printf("Error during shutdown: %v\n", err)
	}

	fmt.Println("✅ Business automation completed successfully")
}

func runSalesLeadAnalysis(ctx context.Context, coordinator *multiagent.Coordinator) {
	fmt.Println("\n📊 WORKFLOW 1: Sales Lead Analysis")
	fmt.Println("===================================")

	// Sales lead data
	leads := []map[string]interface{}{
		{
			"company":    "TechCorp Inc",
			"contact":    "John Smith",
			"budget":     "$50,000",
			"timeline":   "Q1 2025",
			"pain_point": "Manual data processing taking too long",
		},
		{
			"company":    "DataFlow LLC",
			"contact":    "Sarah Johnson",
			"budget":     "$75,000",
			"timeline":   "Q2 2025",
			"pain_point": "Need to automate customer support",
		},
	}

	for i, lead := range leads {
		fmt.Printf("\n📋 Analyzing Lead %d: %s\n", i+1, lead["company"])

		task := &multiagent.TaskRequest{
			Name:        fmt.Sprintf("Analyze Lead: %s", lead["company"]),
			Description: "Analyze sales lead and provide recommendations",
			Type:        "data_analysis",
			Priority:    multiagent.PriorityHigh,
			Input: map[string]interface{}{
				"lead_data": lead,
				"analysis_focus": []string{
					"qualification_score",
					"recommended_approach",
					"potential_objections",
					"next_steps",
				},
			},
		}

		result, err := coordinator.ExecuteTask(ctx, task)
		if err != nil {
			log.Printf("   ❌ Analysis failed: %v\n", err)
			continue
		}

		fmt.Printf("   ✅ Analysis completed: %s\n", result.Status)
		if output, ok := result.Output.(map[string]interface{}); ok {
			if text, exists := output["text"]; exists {
				textStr := fmt.Sprintf("%v", text)
				if len(textStr) > 200 {
					textStr = textStr[:200] + "..."
				}
				fmt.Printf("   Insights: %s\n", textStr)
			}
		}
	}
}

func runContentGeneration(ctx context.Context, coordinator *multiagent.Coordinator) {
	fmt.Println("\n📝 WORKFLOW 2: Marketing Content Generation")
	fmt.Println("============================================")

	contentRequests := []map[string]interface{}{
		{
			"type":   "email_campaign",
			"topic":  "AI Automation Benefits",
			"target": "IT Directors",
			"tone":   "professional",
			"length": "300 words",
		},
		{
			"type":   "blog_post",
			"topic":  "Digital Transformation Success Stories",
			"target": "C-Suite Executives",
			"tone":   "thought leadership",
			"length": "500 words",
		},
	}

	for i, content := range contentRequests {
		fmt.Printf("\n📄 Generating Content %d: %s\n", i+1, content["type"])

		task := &multiagent.TaskRequest{
			Name:        fmt.Sprintf("Generate %s content", content["type"]),
			Description: fmt.Sprintf("Create %s about %s for %s", content["type"], content["topic"], content["target"]),
			Type:        "content_generation",
			Priority:    multiagent.PriorityNormal,
			Input:       content,
		}

		result, err := coordinator.ExecuteTask(ctx, task)
		if err != nil {
			log.Printf("   ❌ Content generation failed: %v\n", err)
			continue
		}

		fmt.Printf("   ✅ Content generated: %s\n", result.Status)
		if output, ok := result.Output.(map[string]interface{}); ok {
			if text, exists := output["text"]; exists {
				textStr := fmt.Sprintf("%v", text)
				if len(textStr) > 200 {
					textStr = textStr[:200] + "..."
				}
				fmt.Printf("   Preview: %s\n", textStr)
			}
		}
	}
}

func runBusinessMetricsAnalysis(ctx context.Context, coordinator *multiagent.Coordinator) {
	fmt.Println("\n📈 WORKFLOW 3: Business Metrics Analysis")
	fmt.Println("=========================================")

	metrics := map[string]interface{}{
		"monthly_revenue": map[string]int{
			"January":  120000,
			"February": 135000,
			"March":    145000,
			"April":    160000,
			"May":      175000,
			"June":     190000,
		},
		"customer_acquisition": map[string]int{
			"Q1": 45,
			"Q2": 62,
		},
		"churn_rate":        "2.5%",
		"customer_lifetime": "$15,000",
		"conversion_rate":   "3.2%",
	}

	task := &multiagent.TaskRequest{
		Name:        "Analyze Business Metrics",
		Description: "Perform comprehensive analysis of business metrics and provide actionable insights",
		Type:        "data_analysis",
		Priority:    multiagent.PriorityHigh,
		Input: map[string]interface{}{
			"metrics": metrics,
			"analysis_types": []string{
				"trend_analysis",
				"growth_projections",
				"risk_assessment",
				"recommendations",
			},
		},
	}

	fmt.Println("📊 Analyzing business metrics...")
	result, err := coordinator.ExecuteTask(ctx, task)
	if err != nil {
		log.Printf("   ❌ Analysis failed: %v\n", err)
		return
	}

	fmt.Printf("   ✅ Analysis completed: %s\n", result.Status)
	if output, ok := result.Output.(map[string]interface{}); ok {
		if text, exists := output["text"]; exists {
			textStr := fmt.Sprintf("%v", text)
			if len(textStr) > 300 {
				textStr = textStr[:300] + "..."
			}
			fmt.Printf("   Findings: %s\n", textStr)
		}
	}
}

func runAutomationScriptGeneration(ctx context.Context, coordinator *multiagent.Coordinator) {
	fmt.Println("\n💻 WORKFLOW 4: Automation Script Generation")
	fmt.Println("============================================")

	automationTasks := []map[string]interface{}{
		{
			"task":        "data_pipeline",
			"language":    "python",
			"description": "ETL pipeline to extract data from CSV, transform, and load to database",
			"features": []string{
				"error handling",
				"logging",
				"incremental updates",
			},
		},
		{
			"task":        "api_integration",
			"language":    "go",
			"description": "REST API client for CRM integration with retry logic",
			"features": []string{
				"authentication",
				"rate limiting",
				"error recovery",
			},
		},
	}

	for i, automation := range automationTasks {
		fmt.Printf("\n🔧 Generating Script %d: %s\n", i+1, automation["task"])

		task := &multiagent.TaskRequest{
			Name:        fmt.Sprintf("Generate %s script", automation["task"]),
			Description: automation["description"].(string),
			Type:        "code_generation",
			Priority:    multiagent.PriorityNormal,
			Input:       automation,
		}

		result, err := coordinator.ExecuteTask(ctx, task)
		if err != nil {
			log.Printf("   ❌ Script generation failed: %v\n", err)
			continue
		}

		fmt.Printf("   ✅ Script generated: %s\n", result.Status)
		if output, ok := result.Output.(map[string]interface{}); ok {
			if text, exists := output["text"]; exists {
				textStr := fmt.Sprintf("%v", text)
				if len(textStr) > 200 {
					textStr = textStr[:200] + "..."
				}
				fmt.Printf("   Code preview: %s\n", textStr)
			}
		}
	}
}

func printFinalStats(ctx context.Context, coordinator *multiagent.Coordinator) {
	fmt.Println("\n📊 FINAL STATISTICS")
	fmt.Println("===================")

	stats, err := coordinator.GetMonitoringStats(ctx)
	if err != nil {
		log.Printf("Failed to get stats: %v\n", err)
		return
	}

	fmt.Printf("   Total Workers: %d (Idle: %d, Busy: %d, Offline: %d)\n",
		stats.TotalWorkers, stats.IdleWorkers, stats.BusyWorkers, stats.OfflineWorkers)
	fmt.Printf("   Total Tasks: %d (Completed: %d, Failed: %d, Pending: %d)\n",
		stats.TotalTasks, stats.CompletedTasks, stats.FailedTasks, stats.PendingTasks)

	if stats.ProtocolMetrics != nil {
		fmt.Printf("   Messages: Sent: %d, Received: %d, Failed: %d\n",
			stats.ProtocolMetrics.TotalMessagesSent,
			stats.ProtocolMetrics.TotalMessagesReceived,
			stats.ProtocolMetrics.TotalMessagesFailed)
	}

	fmt.Println("\n   Workers by Role:")
	for role, count := range stats.WorkersByRole {
		fmt.Printf("     - %s: %d\n", role, count)
	}

	// Health check
	fmt.Println("\n🏥 Health Check:")
	health := coordinator.HealthCheck(ctx)
	fmt.Printf("   Overall Status: %s\n", health.Status)
	for component, status := range health.Components {
		fmt.Printf("   - %s: %s\n", component, status)
	}
	if len(health.Errors) > 0 {
		fmt.Println("   Errors:")
		for _, err := range health.Errors {
			fmt.Printf("     - %s\n", err)
		}
	}
}
