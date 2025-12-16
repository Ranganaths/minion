package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/agentql/agentql/pkg/minion/behaviors"
	"github.com/agentql/agentql/pkg/minion/core"
	"github.com/agentql/agentql/pkg/minion/llm"
	"github.com/agentql/agentql/pkg/minion/models"
	"github.com/agentql/agentql/pkg/minion/storage"
	"github.com/agentql/agentql/pkg/minion/tools/visualization"
)

func main() {
	fmt.Println("=== AgentQL Sales Analyst Agent Demo ===\n")

	// Get OpenAI API key
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

	// Register Sales Analyst behavior
	salesBehavior := behaviors.NewSalesAnalystBehavior()
	if err := framework.RegisterBehavior("sales_analyst", salesBehavior); err != nil {
		log.Fatalf("Failed to register sales behavior: %v", err)
	}

	// Register visualization tools
	if err := registerVisualizationTools(framework); err != nil {
		log.Fatalf("Failed to register visualization tools: %v", err)
	}

	ctx := context.Background()

	// Create Sales Analyst Agent
	agent, err := createSalesAgent(ctx, framework)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Printf("✅ Created Sales Analyst Agent: %s (ID: %s)\n\n", agent.Name, agent.ID)

	// Demo scenarios
	fmt.Println("=== Demo Scenarios ===\n")

	// Scenario 1: Revenue Trend Analysis
	fmt.Println("📈 Scenario 1: Analyze Revenue Trends")
	if err := demoRevenueTrends(ctx, framework, agent); err != nil {
		log.Printf("Revenue trends demo error: %v", err)
	}

	// Scenario 2: Top Customers Comparison
	fmt.Println("\n📊 Scenario 2: Compare Top Customers")
	if err := demoTopCustomers(ctx, framework, agent); err != nil {
		log.Printf("Top customers demo error: %v", err)
	}

	// Scenario 3: Revenue Distribution by Region
	fmt.Println("\n🥧 Scenario 3: Revenue Distribution by Region")
	if err := demoRevenueDistribution(ctx, framework, agent); err != nil {
		log.Printf("Revenue distribution demo error: %v", err)
	}

	// Scenario 4: Sales Pipeline Analysis
	fmt.Println("\n🔍 Scenario 4: Sales Pipeline Analysis")
	if err := demoPipelineAnalysis(ctx, framework, agent); err != nil {
		log.Printf("Pipeline analysis demo error: %v", err)
	}

	fmt.Println("\n=== Demo Complete ===")
}

func createSalesAgent(ctx context.Context, framework core.Framework) (*models.Agent, error) {
	return framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:         "Sales Intelligence Agent",
		Description:  "Expert sales analyst with SQL query generation, execution, and visualization capabilities",
		BehaviorType: "sales_analyst",
		Config: models.AgentConfig{
			LLMProvider: "openai",
			LLMModel:    "gpt-4",
			Temperature: 0.7,
			MaxTokens:   2000,
			Personality: "analytical",
			Custom: map[string]interface{}{
				"max_rows_returned": 1000,
				"query_timeout":     30000,
			},
		},
		Capabilities: []string{
			"sql_generation",
			"sql_execution",
			"visualization",
			"revenue_analysis",
			"trend_detection",
			"forecasting",
		},
		Metadata: map[string]interface{}{
			"domain": "sales",
			"type":   "analytical",
		},
	})
}

func registerVisualizationTools(framework core.Framework) error {
	tools := []interface{}{
		&visualization.BarChartTool{},
		&visualization.LineChartTool{},
		&visualization.PieChartTool{},
		&visualization.TableVisualizerTool{},
	}

	for _, tool := range tools {
		if err := framework.RegisterTool(tool); err != nil {
			return err
		}
	}

	return nil
}

func demoRevenueTrends(ctx context.Context, framework core.Framework, agent *models.Agent) error {
	fmt.Println("Query: 'Show revenue trends for the last 6 months'")
	fmt.Println()

	// Simulate revenue data for the last 6 months
	months := []interface{}{"Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	revenue := []interface{}{450000.0, 480000.0, 520000.0, 495000.0, 550000.0, 580000.0}

	// Create line chart visualization
	tools := framework.GetToolsForAgent(agent)
	var lineChartTool *visualization.LineChartTool
	for _, t := range tools {
		if lct, ok := t.(*visualization.LineChartTool); ok {
			lineChartTool = lct
			break
		}
	}

	if lineChartTool == nil {
		return fmt.Errorf("line chart tool not available")
	}

	output, err := lineChartTool.Execute(ctx, &models.ToolInput{
		Params: map[string]interface{}{
			"title":       "Revenue Trends - Last 6 Months",
			"description": "Monthly revenue showing growth trend",
			"x_data":      months,
			"series": []interface{}{
				map[string]interface{}{
					"name":  "Revenue",
					"data":  revenue,
					"color": "#10b981",
				},
			},
			"x_axis_label": "Month",
			"y_axis_label": "Revenue ($)",
			"smooth":       true,
		},
	})

	if err != nil {
		return err
	}

	printVisualization(output)

	// Print insights
	fmt.Println("\n💡 Key Insights:")
	fmt.Println("  • Revenue grew 28.9% over 6 months (from $450K to $580K)")
	fmt.Println("  • Strong growth trajectory with minor dip in October")
	fmt.Println("  • Average monthly revenue: $512K")
	fmt.Println("  • Best month: December ($580K)")

	fmt.Println("\n📋 Recommendations:")
	fmt.Println("  • Investigate October dip for process improvements")
	fmt.Println("  • Analyze December success factors for replication")
	fmt.Println("  • Forecast Q1 revenue based on current growth rate")

	return nil
}

func demoTopCustomers(ctx context.Context, framework core.Framework, agent *models.Agent) error {
	fmt.Println("Query: 'Show me top 10 customers by revenue this quarter'")
	fmt.Println()

	// Simulate customer data
	customers := []interface{}{
		"Acme Corp", "TechStart Inc", "Global Systems", "Innovation Labs", "Enterprise Co",
		"Digital Solutions", "Cloud Services", "Data Analytics", "Software Partners", "IT Consultants",
	}
	revenues := []interface{}{
		250000.0, 180000.0, 165000.0, 145000.0, 132000.0,
		118000.0, 105000.0, 98000.0, 87000.0, 75000.0,
	}

	// Create bar chart visualization
	tools := framework.GetToolsForAgent(agent)
	var barChartTool *visualization.BarChartTool
	for _, t := range tools {
		if bct, ok := t.(*visualization.BarChartTool); ok {
			barChartTool = bct
			break
		}
	}

	if barChartTool == nil {
		return fmt.Errorf("bar chart tool not available")
	}

	output, err := barChartTool.Execute(ctx, &models.ToolInput{
		Params: map[string]interface{}{
			"title":       "Top 10 Customers by Revenue - Q4 2024",
			"description": "Customer revenue comparison for current quarter",
			"categories":  customers,
			"series": []interface{}{
				map[string]interface{}{
					"name":  "Revenue",
					"data":  revenues,
					"color": "#3b82f6",
				},
			},
			"x_axis_label": "Customer",
			"y_axis_label": "Revenue ($)",
		},
	})

	if err != nil {
		return err
	}

	printVisualization(output)

	// Print insights
	fmt.Println("\n💡 Key Insights:")
	fmt.Println("  • Top customer (Acme Corp) accounts for $250K revenue")
	fmt.Println("  • Top 3 customers contribute 43% of total revenue ($595K)")
	fmt.Println("  • Significant gap between #1 and #2 ($70K difference)")
	fmt.Println("  • Long tail: customers 6-10 each under $120K")

	fmt.Println("\n📋 Recommendations:")
	fmt.Println("  • Strengthen relationships with top 3 customers")
	fmt.Println("  • Develop upsell strategy for customers 4-10")
	fmt.Println("  • Reduce dependency on single customer (Acme Corp)")
	fmt.Println("  • Analyze what makes Acme Corp successful for replication")

	return nil
}

func demoRevenueDistribution(ctx context.Context, framework core.Framework, agent *models.Agent) error {
	fmt.Println("Query: 'Show revenue distribution by region'")
	fmt.Println()

	// Simulate regional data
	regionData := []interface{}{
		map[string]interface{}{"name": "North America", "value": 580000.0},
		map[string]interface{}{"name": "Europe", "value": 420000.0},
		map[string]interface{}{"name": "Asia Pacific", "value": 350000.0},
		map[string]interface{}{"name": "Latin America", "value": 180000.0},
		map[string]interface{}{"name": "Middle East", "value": 120000.0},
	}

	// Create pie chart visualization
	tools := framework.GetToolsForAgent(agent)
	var pieChartTool *visualization.PieChartTool
	for _, t := range tools {
		if pct, ok := t.(*visualization.PieChartTool); ok {
			pieChartTool = pct
			break
		}
	}

	if pieChartTool == nil {
		return fmt.Errorf("pie chart tool not available")
	}

	output, err := pieChartTool.Execute(ctx, &models.ToolInput{
		Params: map[string]interface{}{
			"title":           "Revenue Distribution by Region - 2024",
			"description":     "Global revenue breakdown showing market concentration",
			"data":            regionData,
			"show_percentage": true,
		},
	})

	if err != nil {
		return err
	}

	printVisualization(output)

	// Print insights
	fmt.Println("\n💡 Key Insights:")
	fmt.Println("  • North America dominates with 35.0% of total revenue ($580K)")
	fmt.Println("  • Europe and Asia Pacific combined: 46.5% ($770K)")
	fmt.Println("  • Emerging markets (LATAM + ME): only 18.5% ($300K)")
	fmt.Println("  • Total global revenue: $1.65M")

	fmt.Println("\n📋 Recommendations:")
	fmt.Println("  • Expand presence in high-growth Asia Pacific market")
	fmt.Println("  • Develop growth strategy for emerging markets (LATAM, ME)")
	fmt.Println("  • Maintain strong position in North America")
	fmt.Println("  • Consider regional pricing strategies to optimize revenue")

	return nil
}

func demoPipelineAnalysis(ctx context.Context, framework core.Framework, agent *models.Agent) error {
	fmt.Println("Query: 'Analyze our current sales pipeline by stage'")
	fmt.Println()

	// Simulate pipeline data
	stages := []interface{}{"Lead", "Qualified", "Opportunity", "Proposal", "Negotiation", "Closed Won"}
	dealCounts := []interface{}{250.0, 120.0, 65.0, 32.0, 18.0, 12.0}
	pipelineValues := []interface{}{500000.0, 720000.0, 975000.0, 640000.0, 540000.0, 360000.0}

	// Create stacked bar chart visualization
	tools := framework.GetToolsForAgent(agent)
	var barChartTool *visualization.BarChartTool
	for _, t := range tools {
		if bct, ok := t.(*visualization.BarChartTool); ok {
			barChartTool = bct
			break
		}
	}

	if barChartTool == nil {
		return fmt.Errorf("bar chart tool not available")
	}

	output, err := barChartTool.Execute(ctx, &models.ToolInput{
		Params: map[string]interface{}{
			"title":       "Sales Pipeline Analysis - Current State",
			"description": "Pipeline value and deal count by stage",
			"categories":  stages,
			"series": []interface{}{
				map[string]interface{}{
					"name":  "Deal Count",
					"data":  dealCounts,
					"color": "#8b5cf6",
				},
				map[string]interface{}{
					"name":  "Pipeline Value ($K)",
					"data":  pipelineValues,
					"color": "#10b981",
				},
			},
			"x_axis_label": "Pipeline Stage",
			"y_axis_label": "Count / Value",
		},
	})

	if err != nil {
		return err
	}

	printVisualization(output)

	// Print insights
	fmt.Println("\n💡 Key Insights:")
	fmt.Println("  • Total pipeline value: $3.735M across 497 deals")
	fmt.Println("  • Average deal size increases through funnel ($2K → $30K)")
	fmt.Println("  • Conversion rate Lead→Qualified: 48% (250→120)")
	fmt.Println("  • Strong bottleneck at Opportunity→Proposal: 49% drop")
	fmt.Println("  • Healthy close rate: 67% from Negotiation→Closed Won")

	fmt.Println("\n📋 Recommendations:")
	fmt.Println("  • Focus on Opportunity→Proposal conversion (key bottleneck)")
	fmt.Println("  • Improve proposal quality and delivery speed")
	fmt.Println("  • Maintain momentum in negotiation stage (performing well)")
	fmt.Println("  • Consider lead quality vs quantity trade-off")
	fmt.Println("  • Forecast $360K in closed deals if current rates hold")

	return nil
}

func printVisualization(output *models.ToolOutput) {
	if !output.Success {
		fmt.Printf("❌ Visualization failed: %s\n", output.Error)
		return
	}

	fmt.Println("✅ Visualization Generated")

	// Pretty print the result
	resultJSON, err := json.MarshalIndent(output.Result, "", "  ")
	if err != nil {
		fmt.Printf("Result: %v\n", output.Result)
		return
	}

	fmt.Printf("\nVisualization Data:\n%s\n", string(resultJSON))

	if output.Metadata != nil {
		fmt.Printf("\nMetadata: chart_type=%v, count=%v\n",
			output.Metadata["chart_type"],
			output.Metadata["series_count"],
		)
	}
}
