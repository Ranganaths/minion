# Sales Analyst Agent Example

This example demonstrates a complete Sales Analyst Agent that can analyze sales data and create visualizations.

## Overview

The Sales Analyst Agent combines three powerful capabilities:

1. **Data Analysis** - Analyzes sales metrics and trends
2. **Query Processing** - Processes data queries and requests
3. **Data Visualization** - Creates charts, graphs, and tables

## Features

### Agent Capabilities

- **Revenue Analysis** - Track revenue trends, growth rates, and forecasts
- **Customer Intelligence** - Top customers, segmentation, lifetime value
- **Pipeline Analysis** - Sales pipeline health, bottlenecks, conversion rates
- **Visualization** - Bar charts, line charts, pie charts, and tables
- **Insights & Recommendations** - Actionable insights from data

### Visualization Types

1. **Bar Charts** - Compare values across categories
   - Top customers by revenue
   - Sales by product/region
   - Monthly comparisons

2. **Line Charts** - Show trends over time
   - Revenue trends
   - Growth trajectories
   - KPI tracking

3. **Pie Charts** - Display distributions
   - Revenue by region
   - Market share
   - Customer segments

4. **Tables** - Detailed data listings
   - Transaction details
   - Customer lists
   - Deal pipelines

## Prerequisites

- Go 1.21+
- OpenAI API key
- PostgreSQL database (optional)

## Setup

1. Set your OpenAI API key:
```bash
export OPENAI_API_KEY="your-api-key-here"
```

2. (Optional) Configure database connection for data queries:
```bash
export DATABASE_URL="postgresql://user:pass@localhost:5432/sales_db"
```

3. Run the example:
```bash
cd pkg/minion/examples/sales_agent
go run main.go
```

## Usage

### Creating a Sales Agent

```go
import (
    "github.com/Ranganaths/minion/behaviors"
    "github.com/Ranganaths/minion/core"
    "github.com/Ranganaths/minion/tools/visualization"
)

// Initialize framework
framework := core.NewFramework(
    core.WithStorage(storage.NewInMemory()),
    core.WithLLMProvider(llm.NewOpenAI(apiKey)),
)

// Register Sales Analyst behavior
salesBehavior := behaviors.NewSalesAnalystBehavior()
framework.RegisterBehavior("sales_analyst", salesBehavior)

// Register visualization tools
framework.RegisterTool(&visualization.BarChartTool{})
framework.RegisterTool(&visualization.LineChartTool{})
framework.RegisterTool(&visualization.PieChartTool{})
framework.RegisterTool(&visualization.TableVisualizerTool{})

// Create agent
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name:         "Sales Intelligence Agent",
    BehaviorType: "sales_analyst",
    Capabilities: []string{
        "sql_generation",
        "sql_execution",
        "visualization",
        "revenue_analysis",
    },
})
```

### Making Queries

The agent can handle natural language queries:

```go
// Trend analysis
"Show revenue trends for the last 6 months"
"How has our MRR grown this year?"

// Comparisons
"Show me top 10 customers by revenue"
"Compare sales by region this quarter"

// Distributions
"Show revenue distribution by product"
"What's our market share breakdown?"

// Pipeline analysis
"Analyze our sales pipeline by stage"
"What's the health of our pipeline?"
```

## Demo Scenarios

The example includes 4 complete scenarios:

### Scenario 1: Revenue Trends

**Query**: "Show revenue trends for the last 6 months"

**Output**:
- Line chart showing monthly revenue
- Growth rate calculation
- Key insights about trend
- Recommendations for improvement

**Insights**:
- Revenue growth: 28.9% over 6 months
- Best performing month identified
- Anomalies highlighted
- Forecast suggestions

### Scenario 2: Top Customers

**Query**: "Show me top 10 customers by revenue this quarter"

**Output**:
- Bar chart comparing customer revenues
- Revenue concentration analysis
- Customer segmentation
- Strategic recommendations

**Insights**:
- Top customer contribution percentage
- Customer concentration risk
- Upsell opportunities
- Relationship management priorities

### Scenario 3: Revenue Distribution

**Query**: "Show revenue distribution by region"

**Output**:
- Pie chart showing regional breakdown
- Percentage of total for each region
- Market presence analysis
- Growth opportunities

**Insights**:
- Dominant markets identified
- Emerging market opportunities
- Geographic diversification status
- Regional strategy recommendations

### Scenario 4: Pipeline Analysis

**Query**: "Analyze our current sales pipeline by stage"

**Output**:
- Bar chart showing deals and value by stage
- Conversion rates between stages
- Bottleneck identification
- Pipeline health metrics

**Insights**:
- Total pipeline value
- Stage-by-stage conversion rates
- Bottlenecks and drop-off points
- Deal velocity indicators
- Close rate predictions

## Expected Output

```
=== Sales Analyst Agent Demo ===

✅ Created Sales Analyst Agent: Sales Intelligence Agent (ID: abc123)

=== Demo Scenarios ===

📈 Scenario 1: Analyze Revenue Trends
Query: 'Show revenue trends for the last 6 months'

✅ Visualization Generated

Visualization Data:
{
  "type": "line",
  "title": "Revenue Trends - Last 6 Months",
  "description": "Monthly revenue showing growth trend",
  "xAxis": {
    "label": "Month",
    "type": "category",
    "data": ["Jul", "Aug", "Sep", "Oct", "Nov", "Dec"]
  },
  "yAxis": {
    "label": "Revenue ($)",
    "type": "value"
  },
  "series": [
    {
      "name": "Revenue",
      "type": "line",
      "data": [450000, 480000, 520000, 495000, 550000, 580000],
      "color": "#10b981"
    }
  ],
  "legend": false,
  "options": {
    "smooth": true
  }
}

💡 Key Insights:
  • Revenue grew 28.9% over 6 months (from $450K to $580K)
  • Strong growth trajectory with minor dip in October
  • Average monthly revenue: $512K
  • Best month: December ($580K)

📋 Recommendations:
  • Investigate October dip for process improvements
  • Analyze December success factors for replication
  • Forecast Q1 revenue based on current growth rate

[... additional scenarios ...]

=== Demo Complete ===
```

## Architecture

```
Sales Agent Workflow:

1. User Query (Natural Language)
   ↓
2. Sales Analyst Behavior
   - Detect intent (trend, comparison, distribution)
   - Enhance context with domain knowledge
   - Suggest appropriate visualization type
   ↓
3. SQL Generation
   - Generate appropriate SQL query
   - Apply domain-specific optimizations
   ↓
4. Query Execution
   - Execute against semantic layer
   - Retrieve structured results
   ↓
5. Visualization
   - Create appropriate chart/table
   - Format data for display
   ↓
6. Insights & Recommendations
   - Analyze results
   - Provide actionable insights
   - Suggest next steps
```

## Database Integration

### With Real Database

```go
import (
    "database/sql"
)

// Connect to database
db, _ := sql.Open("postgres", os.Getenv("DATABASE_URL"))

// Create SQL executor tool for data queries
// Register custom tools that can query your database
framework.RegisterTool(customSQLTool)

// Now agent can execute real queries
input := &models.Input{
    Raw: "Show top 10 customers by revenue",
    Context: map[string]interface{}{
        "schema": schemaMetadata,
    },
}

output, _ := framework.Execute(ctx, agentID, input)
```

### Schema Context

Provide schema information for better SQL generation:

```go
schemaContext := map[string]interface{}{
    "tables": []string{
        "customers",
        "orders",
        "products",
        "sales_transactions",
    },
    "relationships": map[string]interface{}{
        "orders": "customer_id → customers.id",
        "sales_transactions": "order_id → orders.id",
    },
    "metrics": []string{
        "revenue",
        "mrr",
        "arr",
        "ltv",
    },
}

input := &models.Input{
    Raw: "Show monthly revenue trends",
    Context: map[string]interface{}{
        "schema": schemaContext,
    },
}
```

## Customization

### Adding Custom Visualizations

```go
type CustomChartTool struct{}

func (t *CustomChartTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
    // Your custom visualization logic
    return &models.ToolOutput{
        ToolName: "custom_chart",
        Success:  true,
        Result:   customVisualization,
    }, nil
}

framework.RegisterTool(&CustomChartTool{})
```

### Extending Sales Behavior

```go
// Create custom behavior based on SalesAnalystBehavior
type CustomSalesBehavior struct {
    *behaviors.SalesAnalystBehavior
}

func (b *CustomSalesBehavior) ProcessInput(ctx context.Context, agent *models.Agent, input *models.Input) (*models.ProcessedInput, error) {
    // Add custom preprocessing
    processed, _ := b.SalesAnalystBehavior.ProcessInput(ctx, agent, input)

    // Your custom enhancements
    processed.Context["custom_field"] = "custom_value"

    return processed, nil
}
```

## Best Practices

### Query Optimization

1. **Limit Result Sets**
   - Default to top 10/20 for comparisons
   - Use pagination for large datasets
   - Set reasonable row limits (1000 default)

2. **Time Ranges**
   - Default to recent periods (last 30 days, quarter)
   - Allow user to specify custom ranges
   - Optimize for common time windows

3. **Aggregation**
   - Pre-aggregate when possible
   - Use appropriate GROUP BY clauses
   - Calculate percentages and rankings in SQL

### Visualization Selection

1. **Line Charts** - For time series (trends over time)
2. **Bar Charts** - For comparisons (top N, categories)
3. **Pie Charts** - For distributions (percentages, proportions)
4. **Tables** - For detailed listings (when precision matters)

### Insights Generation

1. **Calculate Key Metrics**
   - Growth rates
   - Percentages
   - Averages and totals
   - Rankings

2. **Identify Patterns**
   - Trends (increasing, decreasing, stable)
   - Anomalies (spikes, dips)
   - Seasonality
   - Correlations

3. **Provide Context**
   - Compare to previous periods
   - Benchmark against industry standards
   - Show progression over time
   - Highlight significant changes

## Troubleshooting

### Common Issues

1. **"sql_executor tool not found"**
   - Ensure SQL executor is registered with framework
   - Check agent has "sql_execution" capability

2. **"Visualization tool not available"**
   - Register all visualization tools before creating agent
   - Verify agent has "visualization" capability

3. **"Query timeout"**
   - Increase timeout in agent config
   - Optimize SQL queries
   - Check database performance

## Next Steps

1. **Add More Visualizations**
   - Scatter plots for correlations
   - Heatmaps for patterns
   - Funnel charts for conversion flows

2. **Enhance Analysis**
   - Forecasting and predictions
   - Anomaly detection
   - Cohort analysis
   - Attribution modeling

3. **Integrate with UI**
   - Render charts in web interface
   - Interactive filtering and drill-down
   - Export capabilities (PDF, Excel)
   - Real-time updates

4. **Expand Capabilities**
   - Multi-query analysis
   - Cross-domain insights
   - Automated reporting
   - Alert generation

## Learn More

- [Minion Framework Documentation](../../README.md)
- [Visualization Tools](../../tools/visualization/)
- [Sales Analyst Behavior](../../behaviors/sales_analyst.go)
