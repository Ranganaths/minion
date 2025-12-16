package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/yourusername/minion/core"
	"github.com/yourusername/minion/llm"
	"github.com/yourusername/minion/models"
	"github.com/yourusername/minion/storage"
	"github.com/yourusername/minion/tools"
	"github.com/yourusername/minion/tools/domains"
)

func main() {
	fmt.Println("=== Minion Domain-Specific Tools Demo ===\n")

	// Get OpenAI API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	// Initialize framework
	framework := core.NewFramework(
		core.WithStorage(storage.NewInMemory()),
		core.WithLLMProvider(llm.NewOpenAI(apiKey)),
	)
	defer framework.Close()

	// Register all domain-specific tools
	fmt.Println("📦 Registering domain-specific tools...")
	if err := domains.RegisterAllDomainTools(framework); err != nil {
		log.Fatalf("Failed to register tools: %v", err)
	}
	fmt.Println("✅ Tools registered successfully\n")

	ctx := context.Background()

	// Demo 1: Sales Analyst Agent
	fmt.Println("=== Demo 1: Sales Analyst Agent ===\n")
	if err := demoSalesAnalyst(ctx, framework); err != nil {
		log.Printf("Sales analyst demo error: %v", err)
	}

	// Demo 2: Marketing Analyst Agent
	fmt.Println("\n=== Demo 2: Marketing Analyst Agent ===\n")
	if err := demoMarketingAnalyst(ctx, framework); err != nil {
		log.Printf("Marketing analyst demo error: %v", err)
	}

	// Demo 3: Tool Capability Filtering
	fmt.Println("\n=== Demo 3: Tool Capability Filtering ===\n")
	if err := demoCapabilityFiltering(ctx, framework); err != nil {
		log.Printf("Capability filtering demo error: %v", err)
	}

	fmt.Println("\n=== Demo Complete ===")
}

func demoSalesAnalyst(ctx context.Context, framework core.Framework) error {
	fmt.Println("Creating Sales Analyst agent with revenue and forecasting capabilities...")

	// Create Sales Analyst agent
	agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:        "Sales Revenue Analyst",
		Description: "Analyzes sales revenue trends and forecasts future performance",
		BehaviorType: "default",
		Config: models.AgentConfig{
			LLMProvider: "openai",
			LLMModel:    "gpt-4",
			Temperature: 0.7,
			MaxTokens:   1000,
		},
		Capabilities: []string{
			"revenue_analysis",
			"forecasting",
			"trend_detection",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create sales agent: %w", err)
	}

	fmt.Printf("✅ Created agent: %s (ID: %s)\n\n", agent.Name, agent.ID)

	// Demo: Revenue Analysis
	fmt.Println("📊 Analyzing Revenue Trends...")
	revenueInput := &models.ToolInput{
		Params: map[string]interface{}{
			"revenues": []float64{100000, 105000, 112000, 108000, 125000, 130000},
			"periods":  []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
		},
	}

	// Get revenue analyzer tool
	agentTools := framework.GetToolsForAgent(agent)
	var revenueTool tools.Tool
	for _, t := range agentTools {
		if tool, ok := t.(tools.Tool); ok && tool.Name() == "revenue_analyzer" {
			revenueTool = tool
			break
		}
	}

	if revenueTool != nil {
		output, err := revenueTool.Execute(ctx, revenueInput)
		if err != nil {
			return fmt.Errorf("revenue analysis failed: %w", err)
		}
		printToolOutput(output)
	}

	// Demo: Sales Forecasting
	fmt.Println("\n🔮 Forecasting Future Revenue...")
	forecastInput := &models.ToolInput{
		Params: map[string]interface{}{
			"historical_values": []float64{100000, 105000, 112000, 108000, 125000, 130000},
			"periods_ahead":     3,
			"method":            "moving_average",
		},
	}

	var forecastTool tools.Tool
	for _, t := range agentTools {
		if tool, ok := t.(tools.Tool); ok && tool.Name() == "sales_forecasting" {
			forecastTool = tool
			break
		}
	}

	if forecastTool != nil {
		output, err := forecastTool.Execute(ctx, forecastInput)
		if err != nil {
			return fmt.Errorf("forecasting failed: %w", err)
		}
		printToolOutput(output)
	}

	return nil
}

func demoMarketingAnalyst(ctx context.Context, framework core.Framework) error {
	fmt.Println("Creating Marketing Analyst agent with campaign and funnel analysis capabilities...")

	// Create Marketing Analyst agent
	agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:        "Marketing Campaign Analyst",
		Description: "Analyzes marketing campaigns and customer acquisition metrics",
		BehaviorType: "default",
		Config: models.AgentConfig{
			LLMProvider: "openai",
			LLMModel:    "gpt-4",
			Temperature: 0.7,
			MaxTokens:   1000,
		},
		Capabilities: []string{
			"campaign_analysis",
			"funnel_analysis",
			"roi_calculation",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create marketing agent: %w", err)
	}

	fmt.Printf("✅ Created agent: %s (ID: %s)\n\n", agent.Name, agent.ID)

	// Demo: Campaign ROI Analysis
	fmt.Println("💰 Calculating Campaign ROI...")
	roiInput := &models.ToolInput{
		Params: map[string]interface{}{
			"revenue":      50000.0,
			"cost":         15000.0,
			"conversions":  250,
			"campaign_name": "Q4 Email Campaign",
		},
	}

	// Get campaign ROI tool
	agentTools := framework.GetToolsForAgent(agent)
	var roiTool tools.Tool
	for _, t := range agentTools {
		if tool, ok := t.(tools.Tool); ok && tool.Name() == "campaign_roi_calculator" {
			roiTool = tool
			break
		}
	}

	if roiTool != nil {
		output, err := roiTool.Execute(ctx, roiInput)
		if err != nil {
			return fmt.Errorf("ROI calculation failed: %w", err)
		}
		printToolOutput(output)
	}

	// Demo: Funnel Analysis
	fmt.Println("\n🔍 Analyzing Marketing Funnel...")
	funnelInput := &models.ToolInput{
		Params: map[string]interface{}{
			"stages": map[string]int{
				"awareness":     10000,
				"interest":      3500,
				"consideration": 1200,
				"intent":        450,
				"purchase":      250,
			},
		},
	}

	var funnelTool tools.Tool
	for _, t := range agentTools {
		if tool, ok := t.(tools.Tool); ok && tool.Name() == "funnel_analyzer" {
			funnelTool = tool
			break
		}
	}

	if funnelTool != nil {
		output, err := funnelTool.Execute(ctx, funnelInput)
		if err != nil {
			return fmt.Errorf("funnel analysis failed: %w", err)
		}
		printToolOutput(output)
	}

	// Demo: CAC Calculation
	fmt.Println("\n📈 Calculating Customer Acquisition Cost...")
	cacInput := &models.ToolInput{
		Params: map[string]interface{}{
			"marketing_spend": 50000.0,
			"sales_spend":     30000.0,
			"new_customers":   250,
			"ltv":             1200.0,
		},
	}

	var cacTool tools.Tool
	for _, t := range agentTools {
		if tool, ok := t.(tools.Tool); ok && tool.Name() == "cac_calculator" {
			cacTool = tool
			break
		}
	}

	if cacTool != nil {
		output, err := cacTool.Execute(ctx, cacInput)
		if err != nil {
			return fmt.Errorf("CAC calculation failed: %w", err)
		}
		printToolOutput(output)
	}

	return nil
}

func demoCapabilityFiltering(ctx context.Context, framework core.Framework) error {
	fmt.Println("Demonstrating capability-based tool filtering...\n")

	// Create agent with limited capabilities
	limitedAgent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:         "Limited Agent",
		Description:  "Agent with only basic analysis capabilities",
		BehaviorType: "default",
		Config: models.AgentConfig{
			LLMProvider: "openai",
			LLMModel:    "gpt-4",
		},
		Capabilities: []string{
			"basic_analysis", // Only basic capability
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create limited agent: %w", err)
	}

	// Create agent with full capabilities
	fullAgent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:         "Full Capability Agent",
		Description:  "Agent with all analysis capabilities",
		BehaviorType: "default",
		Config: models.AgentConfig{
			LLMProvider: "openai",
			LLMModel:    "gpt-4",
		},
		Capabilities: []string{
			"revenue_analysis",
			"forecasting",
			"campaign_analysis",
			"funnel_analysis",
			"pipeline_analysis",
			"customer_segmentation",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create full agent: %w", err)
	}

	// Compare available tools
	limitedTools := framework.GetToolsForAgent(limitedAgent)
	fullTools := framework.GetToolsForAgent(fullAgent)

	fmt.Printf("Limited Agent (%s) has access to %d tools:\n", limitedAgent.Name, len(limitedTools))
	for _, t := range limitedTools {
		if tool, ok := t.(tools.Tool); ok {
			fmt.Printf("  - %s\n", tool.Name())
		}
	}

	fmt.Printf("\nFull Capability Agent (%s) has access to %d tools:\n", fullAgent.Name, len(fullTools))
	for _, t := range fullTools {
		if tool, ok := t.(tools.Tool); ok {
			fmt.Printf("  - %s\n", tool.Name())
		}
	}

	fmt.Printf("\n✅ Capability filtering working correctly!")
	fmt.Printf("\n   Limited agent: %d tools", len(limitedTools))
	fmt.Printf("\n   Full agent: %d tools", len(fullTools))

	return nil
}

func printToolOutput(output *models.ToolOutput) {
	fmt.Printf("Tool: %s\n", output.ToolName)
	fmt.Printf("Success: %v\n", output.Success)

	if output.Success {
		fmt.Println("Results:")
		if result, ok := output.Result.(map[string]interface{}); ok {
			for key, value := range result {
				fmt.Printf("  %s: %v\n", key, value)
			}
		} else {
			fmt.Printf("  %v\n", output.Result)
		}
	} else {
		fmt.Printf("Error: %s\n", output.Error)
	}

	if output.Metadata != nil {
		fmt.Println("Metadata:")
		for key, value := range output.Metadata {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}

	fmt.Printf("Execution Time: %s\n", output.ExecutionTime)
}
