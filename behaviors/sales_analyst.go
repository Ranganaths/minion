package behaviors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentql/agentql/pkg/minion/models"
)

// SalesAnalystBehavior provides integrated sales analysis with SQL generation,
// execution, and visualization capabilities
type SalesAnalystBehavior struct {
	domain string
}

// NewSalesAnalystBehavior creates a new sales analyst behavior
func NewSalesAnalystBehavior() *SalesAnalystBehavior {
	return &SalesAnalystBehavior{
		domain: "sales",
	}
}

func (b *SalesAnalystBehavior) GetSystemPrompt(agent *models.Agent) string {
	return fmt.Sprintf(`You are %s, an expert Sales Analyst Agent powered by AgentQL.

**Your Role**: You help analyze sales data through natural language queries. You can:
1. Generate SQL queries for the sales semantic layer
2. Execute queries and retrieve data
3. Create visualizations (charts, graphs, tables)
4. Provide actionable insights and recommendations

**Your Capabilities**:
- Revenue analysis and forecasting
- Sales pipeline health monitoring
- Customer segmentation and behavior analysis
- Deal scoring and prioritization
- Quota attainment tracking
- Churn prediction and retention strategies

**Sales Domain Expertise**:
- **Key Metrics**: Revenue, MRR, ARR, ACV, LTV, CAC, Win Rate, Deal Velocity
- **Pipeline Stages**: Lead → Qualified → Opportunity → Proposal → Negotiation → Closed
- **Analysis Types**: Trend analysis, cohort analysis, funnel analysis, attribution
- **Common Queries**:
  - "Show top customers by revenue this quarter"
  - "Analyze revenue trends over the past 6 months"
  - "What's our sales pipeline value by stage?"
  - "Which products are performing best?"

**Your Workflow**:
1. **Understand** the user's question and intent
2. **Generate** appropriate SQL query for the semantic layer
3. **Execute** the query and retrieve results
4. **Visualize** the data using appropriate chart types:
   - Bar charts for comparisons (top customers, products by revenue)
   - Line charts for trends (revenue over time, growth rates)
   - Pie charts for distributions (revenue by region, market share)
   - Tables for detailed data listings
5. **Analyze** results and provide insights
6. **Recommend** next actions or follow-up analyses

**Best Practices**:
- Always format currency values with $ and proper decimals
- Show percentages with % symbol and 1-2 decimal places
- Include time periods in chart titles (e.g., "Q4 2024")
- Provide context: compare to previous periods, industry benchmarks
- Highlight anomalies, trends, and actionable insights
- Suggest follow-up questions or deeper analyses

**Important Guidelines**:
- Only query tables/fields that exist in the schema
- Respect row limits (default 1000 rows)
- Timeout queries after 30 seconds
- Handle NULL values gracefully
- Validate date ranges and filters

**Response Format**:
When answering queries, provide:
1. SQL Query (what you're executing)
2. Visualization (chart or table)
3. Key Insights (3-5 bullet points)
4. Recommendations (next steps or actions)

You are helpful, analytical, and focused on driving business decisions through data.
`, agent.Name)
}

func (b *SalesAnalystBehavior) ProcessInput(ctx context.Context, agent *models.Agent, input *models.Input) (*models.ProcessedInput, error) {
	// Extract and enhance the input with sales-specific context
	query := input.Raw

	// Detect intent from query
	intent := b.detectIntent(query)

	// Add sales-specific context
	enhancedContext := make(map[string]interface{})
	if input.Context != nil {
		for k, v := range input.Context {
			enhancedContext[k] = v
		}
	}

	enhancedContext["domain"] = "sales"
	enhancedContext["intent"] = intent
	enhancedContext["requires_visualization"] = b.requiresVisualization(query)
	enhancedContext["suggested_chart_type"] = b.suggestChartType(query, intent)

	// Create enhanced instructions
	instructions := b.buildInstructions(query, intent)

	return &models.ProcessedInput{
		Original:     input,
		Processed:    query,
		ExtraContext: enhancedContext,
		Instructions: instructions,
	}, nil
}

func (b *SalesAnalystBehavior) ProcessOutput(ctx context.Context, agent *models.Agent, output *models.Output) (*models.ProcessedOutput, error) {
	// Parse and enhance the output
	result := output.Result

	// Try to parse as JSON if it's a string
	if resultStr, ok := result.(string); ok {
		var parsed interface{}
		if err := json.Unmarshal([]byte(resultStr), &parsed); err == nil {
			result = parsed
		}
	}

	// Create structured response
	response := map[string]interface{}{
		"original_query": output.Result,
		"domain":         "sales",
	}

	// Check if we have query results
	if resultMap, ok := result.(map[string]interface{}); ok {
		// Extract SQL if present
		if sql, ok := resultMap["sql"].(string); ok {
			response["sql"] = sql
		}

		// Extract data rows if present
		if rows, ok := resultMap["rows"].([]interface{}); ok {
			response["data"] = rows
			response["row_count"] = len(rows)
		}

		// Extract visualization if present
		if viz, ok := resultMap["visualization"]; ok {
			response["visualization"] = viz
		}

		// Extract insights if present
		if insights, ok := resultMap["insights"]; ok {
			response["insights"] = insights
		}
	}

	return &models.ProcessedOutput{
		Original:  output,
		Processed: result,
	}, nil
}

// Helper methods

func (b *SalesAnalystBehavior) detectIntent(query string) string {
	queryLower := strings.ToLower(query)

	// Trend analysis
	if strings.Contains(queryLower, "trend") || strings.Contains(queryLower, "over time") ||
		strings.Contains(queryLower, "growth") || strings.Contains(queryLower, "history") {
		return "trend_analysis"
	}

	// Comparison
	if strings.Contains(queryLower, "compare") || strings.Contains(queryLower, "vs") ||
		strings.Contains(queryLower, "versus") || strings.Contains(queryLower, "top") {
		return "comparison"
	}

	// Distribution
	if strings.Contains(queryLower, "distribution") || strings.Contains(queryLower, "breakdown") ||
		strings.Contains(queryLower, "by region") || strings.Contains(queryLower, "by product") {
		return "distribution"
	}

	// Aggregation
	if strings.Contains(queryLower, "total") || strings.Contains(queryLower, "sum") ||
		strings.Contains(queryLower, "average") || strings.Contains(queryLower, "count") {
		return "aggregation"
	}

	// Detail view
	if strings.Contains(queryLower, "list") || strings.Contains(queryLower, "show all") ||
		strings.Contains(queryLower, "details") {
		return "detail"
	}

	// Forecast
	if strings.Contains(queryLower, "forecast") || strings.Contains(queryLower, "predict") ||
		strings.Contains(queryLower, "projection") {
		return "forecast"
	}

	return "general"
}

func (b *SalesAnalystBehavior) requiresVisualization(query string) bool {
	queryLower := strings.ToLower(query)

	// Explicit visualization requests
	if strings.Contains(queryLower, "chart") || strings.Contains(queryLower, "graph") ||
		strings.Contains(queryLower, "visualize") || strings.Contains(queryLower, "plot") {
		return true
	}

	// Queries that benefit from visualization
	if strings.Contains(queryLower, "trend") || strings.Contains(queryLower, "compare") ||
		strings.Contains(queryLower, "top") || strings.Contains(queryLower, "distribution") {
		return true
	}

	return false
}

func (b *SalesAnalystBehavior) suggestChartType(query string, intent string) string {
	queryLower := strings.ToLower(query)

	// Explicit chart type requests
	if strings.Contains(queryLower, "bar chart") || strings.Contains(queryLower, "bar graph") {
		return "bar"
	}
	if strings.Contains(queryLower, "line chart") || strings.Contains(queryLower, "line graph") {
		return "line"
	}
	if strings.Contains(queryLower, "pie chart") || strings.Contains(queryLower, "pie graph") {
		return "pie"
	}

	// Suggest based on intent
	switch intent {
	case "trend_analysis":
		return "line"
	case "comparison":
		return "bar"
	case "distribution":
		return "pie"
	case "detail":
		return "table"
	default:
		// Check if comparing over time
		if strings.Contains(queryLower, "over time") || strings.Contains(queryLower, "by month") ||
			strings.Contains(queryLower, "by quarter") || strings.Contains(queryLower, "by year") {
			return "line"
		}
		return "bar"
	}
}

func (b *SalesAnalystBehavior) buildInstructions(query string, intent string) string {
	instructions := []string{
		fmt.Sprintf("User query: %s", query),
		fmt.Sprintf("Detected intent: %s", intent),
	}

	switch intent {
	case "trend_analysis":
		instructions = append(instructions,
			"Generate SQL to retrieve time-series data",
			"Order by date/time ascending",
			"Create a line chart showing the trend",
			"Calculate growth rate or change percentage",
			"Identify any significant changes or anomalies",
		)
	case "comparison":
		instructions = append(instructions,
			"Generate SQL to compare entities (customers, products, regions, etc.)",
			"Order by the comparison metric descending",
			"Create a bar chart for visual comparison",
			"Show top 10 by default unless specified",
			"Include percentage of total if relevant",
		)
	case "distribution":
		instructions = append(instructions,
			"Generate SQL to calculate distribution across categories",
			"Calculate both absolute values and percentages",
			"Create a pie chart showing proportions",
			"Highlight the largest segments",
		)
	case "detail":
		instructions = append(instructions,
			"Generate SQL to retrieve detailed records",
			"Include all relevant columns",
			"Create a formatted table",
			"Apply appropriate sorting",
		)
	case "forecast":
		instructions = append(instructions,
			"Retrieve historical data for forecasting",
			"Use sales forecasting tool",
			"Show forecast with confidence intervals",
			"Create line chart with historical and forecast data",
		)
	}

	instructions = append(instructions,
		"Format currency values with $ and 2 decimals",
		"Format percentages with % and 1-2 decimals",
		"Provide 3-5 key insights from the data",
		"Suggest follow-up questions or actions",
	)

	return strings.Join(instructions, "\n- ")
}
