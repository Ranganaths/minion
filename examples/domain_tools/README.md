# Domain-Specific Tools Example

This example demonstrates Minion's domain-specific tools for Sales and Marketing analysts.

## Overview

This example shows:

1. **Sales Analyst Agent** - Using revenue analysis and forecasting tools
2. **Marketing Analyst Agent** - Using campaign ROI, funnel analysis, and CAC calculation tools
3. **Capability-Based Filtering** - How agents get access to tools based on their capabilities

## Domain Tools

### Sales Analyst Tools (8 tools)

1. **Revenue Analyzer** - Analyzes revenue trends, growth rates, and forecasts
2. **Pipeline Analyzer** - Analyzes sales pipeline health and bottlenecks
3. **Customer Segmentation** - Segments customers by revenue and behavior
4. **Deal Scoring** - Scores deals by probability to close
5. **Sales Forecasting** - Generates sales forecasts using various methods
6. **Conversion Rate Analyzer** - Analyzes conversion rates between funnel stages
7. **Churn Predictor** - Predicts customer churn risk
8. **Quota Attainment** - Analyzes quota performance by rep

### Marketing Analyst Tools (9 tools)

1. **Campaign ROI Calculator** - Calculates ROI, ROAS, and profit margins
2. **Funnel Analyzer** - Analyzes marketing funnel stages and drop-offs
3. **CAC Calculator** - Calculates Customer Acquisition Cost and LTV:CAC ratio
4. **Attribution Analyzer** - Multi-touch attribution analysis
5. **A/B Test Analyzer** - Analyzes A/B test results with statistical significance
6. **Engagement Scorer** - Scores content engagement
7. **Content Performance** - Analyzes content performance by channel
8. **Lead Scoring** - Scores leads by behavior and demographics
9. **Email Campaign Analyzer** - Analyzes email campaign metrics

## Prerequisites

- Go 1.21+
- OpenAI API key

## Setup

1. Set your OpenAI API key:
```bash
export OPENAI_API_KEY="your-api-key-here"
```

2. Run the example:
```bash
cd pkg/minion/examples/domain_tools
go run main.go
```

## What the Example Does

### Demo 1: Sales Analyst

Creates a Sales Analyst agent with revenue analysis and forecasting capabilities:

```go
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name:        "Sales Revenue Analyst",
    Description: "Analyzes sales revenue trends and forecasts future performance",
    Capabilities: []string{
        "revenue_analysis",
        "forecasting",
        "trend_detection",
    },
})
```

Then demonstrates:
- **Revenue Analysis**: Analyzes monthly revenue data to detect trends, growth rates, and volatility
- **Sales Forecasting**: Forecasts future revenue using moving average method

### Demo 2: Marketing Analyst

Creates a Marketing Analyst agent with campaign and funnel analysis capabilities:

```go
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name:        "Marketing Campaign Analyst",
    Description: "Analyzes marketing campaigns and customer acquisition metrics",
    Capabilities: []string{
        "campaign_analysis",
        "funnel_analysis",
        "roi_calculation",
    },
})
```

Then demonstrates:
- **Campaign ROI**: Calculates ROI, ROAS, profit margin for a Q4 email campaign
- **Funnel Analysis**: Analyzes conversion rates between marketing funnel stages
- **CAC Calculation**: Calculates Customer Acquisition Cost and LTV:CAC ratio

### Demo 3: Capability Filtering

Creates two agents with different capabilities and shows how tool access is filtered:

- **Limited Agent**: Only has `basic_analysis` capability → Limited tool access
- **Full Agent**: Has multiple capabilities → Access to all relevant tools

This demonstrates Minion's capability-based access control system.

## Expected Output

```
=== Minion Domain-Specific Tools Demo ===

📦 Registering domain-specific tools...
✅ Tools registered successfully

=== Demo 1: Sales Analyst Agent ===

Creating Sales Analyst agent with revenue and forecasting capabilities...
✅ Created agent: Sales Revenue Analyst (ID: ...)

📊 Analyzing Revenue Trends...
Tool: revenue_analyzer
Success: true
Results:
  total_revenue: 680000
  average: 113333.33
  growth_rate: 30.0
  trend: increasing
  volatility: 8.5
  recommendation: Revenue shows strong growth with moderate volatility...

🔮 Forecasting Future Revenue...
Tool: sales_forecasting
Success: true
Results:
  forecast: [135000, 138000, 141000]
  confidence: high
  method: moving_average
  ...

=== Demo 2: Marketing Analyst Agent ===

Creating Marketing Analyst agent with campaign and funnel analysis capabilities...
✅ Created agent: Marketing Campaign Analyst (ID: ...)

💰 Calculating Campaign ROI...
Tool: campaign_roi_calculator
Success: true
Results:
  roi: 233.33
  roas: 3.33
  profit: 35000
  profit_margin: 70.0
  ...

🔍 Analyzing Marketing Funnel...
Tool: funnel_analyzer
Success: true
Results:
  total_conversion_rate: 2.5
  stage_conversion_rates: {...}
  bottleneck: consideration → intent
  ...

📈 Calculating Customer Acquisition Cost...
Tool: cac_calculator
Success: true
Results:
  cac: 320.0
  ltv_cac_ratio: 3.75
  payback_period: 3.2
  ...

=== Demo 3: Tool Capability Filtering ===

Demonstrating capability-based tool filtering...

Limited Agent (Limited Agent) has access to 0 tools:

Full Capability Agent (Full Capability Agent) has access to 17 tools:
  - revenue_analyzer
  - sales_forecasting
  - pipeline_analyzer
  - customer_segmentation
  - campaign_roi_calculator
  - funnel_analyzer
  - cac_calculator
  ...

✅ Capability filtering working correctly!
   Limited agent: 0 tools
   Full agent: 17 tools

=== Demo Complete ===
```

## Customization

### Adding Your Own Tools

1. Create a new tool in the appropriate domain package:

```go
type MyCustomTool struct{}

func (t *MyCustomTool) Name() string {
    return "my_custom_tool"
}

func (t *MyCustomTool) Description() string {
    return "Performs custom analysis"
}

func (t *MyCustomTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
    // Your tool logic here
    return &models.ToolOutput{
        ToolName: t.Name(),
        Success:  true,
        Result:   yourResult,
    }, nil
}

func (t *MyCustomTool) CanExecute(agent *models.Agent) bool {
    return containsCapability(agent.Capabilities, "your_capability")
}
```

2. Register your tool:

```go
framework.RegisterTool(&MyCustomTool{})
```

### Creating Agents with Different Capabilities

```go
// Specialized forecasting agent
forecastAgent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name: "Forecast Specialist",
    Capabilities: []string{
        "forecasting",
        "trend_detection",
    },
})

// Full-featured sales agent
fullSalesAgent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name: "Complete Sales Analyst",
    Capabilities: []string{
        "revenue_analysis",
        "forecasting",
        "pipeline_analysis",
        "customer_segmentation",
        "deal_scoring",
        "churn_prediction",
        "quota_analysis",
    },
})
```

## Architecture

```
pkg/minion/
├── tools/
│   └── domains/
│       ├── register.go          # Tool registration helpers
│       ├── sales/
│       │   └── tools.go         # 8 Sales tools
│       └── marketing/
│           └── tools.go         # 9 Marketing tools
└── examples/
    └── domain_tools/
        ├── main.go              # This example
        └── README.md            # This file
```

## Key Concepts

### Capability-Based Access Control

Tools check agent capabilities to determine if they can execute:

```go
func (t *RevenueAnalyzerTool) CanExecute(agent *models.Agent) bool {
    return containsCapability(agent.Capabilities, "revenue_analysis")
}
```

This ensures:
- Agents only get tools relevant to their role
- Fine-grained access control
- Easy to audit and manage permissions

### Tool Input/Output Structure

All tools use consistent input/output structures:

```go
// Input
input := &models.ToolInput{
    Params: map[string]interface{}{
        "param1": value1,
        "param2": value2,
    },
}

// Output
output := &models.ToolOutput{
    ToolName:      "tool_name",
    Success:       true,
    Result:        result,
    Metadata:      metadata,
    ExecutionTime: duration,
}
```

### Domain Organization

Tools are organized by business domain:
- `sales/` - Sales-specific analysis tools
- `marketing/` - Marketing-specific analysis tools
- Future: `finance/`, `hr/`, `operations/`, etc.

## Next Steps

1. Add more domain tools (Finance, HR, Operations)
2. Create specialized behaviors for each domain
3. Integrate tools with LLM for natural language queries
4. Add data source connectors (databases, APIs)
5. Create dashboards for tool outputs

## Learn More

- [Minion Framework Documentation](../../README.md)
- [AgentQL Integration Guide](../../../../MINION_INTEGRATION.md)
- [Adding New Tools Guide](../../../../ADDING_NEW_TOOLS.md)
