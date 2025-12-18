# Minion Framework - Tools Guide

This guide provides comprehensive documentation and examples for all tools available in the Minion framework.

## Table of Contents

- [Data & Analytics Tools](#data--analytics-tools)
- [Customer & Support Tools](#customer--support-tools)
- [Financial Tools](#financial-tools)
- [Integration & External Tools](#integration--external-tools)
- [Sales Tools](#sales-tools)
- [Marketing Tools](#marketing-tools)

---

## Data & Analytics Tools

### 1. SQL Generator
Converts natural language queries into SQL statements.

**Tool Name:** `sql_generator`
**Capabilities:** `sql_generation`, `data_analytics`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "query": "Show me total revenue for the last 30 days",
        "dialect": "postgres",
        "schema": map[string]interface{}{
            "transactions": []string{"id", "amount", "date"},
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "sql_generator", input)
// Returns: SQL query, explanation, and metadata
```

### 2. Anomaly Detector
Identifies outliers in time-series data using statistical methods.

**Tool Name:** `anomaly_detector`
**Capabilities:** `anomaly_detection`, `data_analytics`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "data": []float64{100, 105, 98, 102, 250, 99, 103},
        "sensitivity": 2.0, // Standard deviations
    },
}

output, _ := framework.ExecuteTool(ctx, "anomaly_detector", input)
// Returns: Detected anomalies with indices, z-scores, and severity
```

### 3. Correlation Analyzer
Analyzes correlations between multiple metrics.

**Tool Name:** `correlation_analyzer`
**Capabilities:** `correlation_analysis`, `data_analytics`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "datasets": map[string][]float64{
            "revenue": {1000, 1200, 1100, 1300},
            "marketing_spend": {100, 150, 120, 170},
            "customer_count": {50, 60, 55, 65},
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "correlation_analyzer", input)
// Returns: Correlation pairs, strong/weak correlations, insights
```

### 4. Trend Predictor
Advanced forecasting with confidence intervals.

**Tool Name:** `trend_predictor`
**Capabilities:** `trend_prediction`, `data_analytics`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "data": []float64{100, 110, 105, 120, 115, 125},
        "periods": 5,
        "method": "linear",
    },
}

output, _ := framework.ExecuteTool(ctx, "trend_predictor", input)
// Returns: Predictions, confidence intervals, R-squared, trend analysis
```

### 5. Data Validator
Validates data quality and completeness.

**Tool Name:** `data_validator`
**Capabilities:** `data_validation`, `data_analytics`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "data": []map[string]interface{}{
            {"name": "John", "email": "john@example.com", "age": 30},
            {"name": "Jane", "email": nil},
        },
        "rules": map[string]interface{}{
            "required_fields": []string{"name", "email"},
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "data_validator", input)
// Returns: Validation results, errors, warnings, quality score
```

### 6. Report Generator
Creates automated business reports.

**Tool Name:** `report_generator`
**Capabilities:** `report_generation`, `data_analytics`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "data": map[string]interface{}{
            "revenue": 50000,
            "expenses": 30000,
            "customers": 150,
        },
        "report_type": "detailed",
    },
}

output, _ := framework.ExecuteTool(ctx, "report_generator", input)
// Returns: Report with summary, insights, and recommendations
```

### 7. Data Transformer
Performs ETL operations.

**Tool Name:** `data_transformer`
**Capabilities:** `data_transformation`, `data_analytics`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "data": []map[string]interface{}{
            {"name": " John ", "value": 100},
            {"name": " Jane ", "value": 200},
        },
        "operations": []string{"clean", "normalize", "deduplicate"},
    },
}

output, _ := framework.ExecuteTool(ctx, "data_transformer", input)
// Returns: Transformed data, quality score
```

### 8. Statistical Analyzer
Comprehensive statistical analysis.

**Tool Name:** `statistical_analyzer`
**Capabilities:** `statistical_analysis`, `data_analytics`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "data": []float64{10, 20, 15, 25, 30, 18, 22},
    },
}

output, _ := framework.ExecuteTool(ctx, "statistical_analyzer", input)
// Returns: Descriptive stats, distribution analysis, percentiles
```

### 9. Data Profiler
Profiles datasets for quality and structure.

**Tool Name:** `data_profiler`
**Capabilities:** `data_profiling`, `data_analytics`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "data": []map[string]interface{}{
            {"name": "Product A", "price": 99.99, "stock": 50},
            {"name": "Product B", "price": 149.99, "stock": nil},
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "data_profiler", input)
// Returns: Field statistics, completeness, type consistency
```

### 10. Time Series Analyzer
Analyzes time-series for patterns.

**Tool Name:** `timeseries_analyzer`
**Capabilities:** `timeseries_analysis`, `data_analytics`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "data": []float64{100, 105, 102, 110, 108, 115},
        "timestamps": []string{"2024-01", "2024-02", "2024-03", "2024-04", "2024-05", "2024-06"},
    },
}

output, _ := framework.ExecuteTool(ctx, "timeseries_analyzer", input)
// Returns: Trend, seasonality detection, volatility
```

---

## Customer & Support Tools

### 1. Sentiment Analyzer
Analyzes sentiment from text feedback.

**Tool Name:** `sentiment_analyzer`
**Capabilities:** `sentiment_analysis`, `customer_support`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "text": "The product is amazing! Love the new features.",
    },
}

output, _ := framework.ExecuteTool(ctx, "sentiment_analyzer", input)
// Returns: Sentiment (positive/negative/neutral), score, confidence
```

### 2. Ticket Classifier
Automatically classifies support tickets.

**Tool Name:** `ticket_classifier`
**Capabilities:** `ticket_classification`, `customer_support`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "ticket": map[string]interface{}{
            "subject": "Urgent: Login not working",
            "description": "I can't access my account. Getting error message.",
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "ticket_classifier", input)
// Returns: Category, priority, urgency, SLA hours, tags
```

### 3. Response Generator
Generates templated customer responses.

**Tool Name:** `response_generator`
**Capabilities:** `response_generation`, `customer_support`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "context": map[string]interface{}{
            "customer_name": "John Smith",
            "issue_type": "billing",
        },
        "response_type": "billing",
    },
}

output, _ := framework.ExecuteTool(ctx, "response_generator", input)
// Returns: Response template, suggestions
```

### 4. Customer Health Scorer
Comprehensive customer wellness scoring.

**Tool Name:** `customer_health_scorer`
**Capabilities:** `customer_health`, `customer_support`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "customer": map[string]interface{}{
            "last_activity_days": 15,
            "support_tickets": 2,
            "nps_score": 9,
            "usage_percentage": 75,
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "customer_health_scorer", input)
// Returns: Health score, risk level, factors, recommendations
```

### 5. Feedback Analyzer
Analyzes customer feedback patterns.

**Tool Name:** `feedback_analyzer`
**Capabilities:** `feedback_analysis`, `customer_support`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "feedback": []map[string]interface{}{
            {"text": "Great product!", "rating": 5},
            {"text": "Needs improvement", "rating": 3},
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "feedback_analyzer", input)
// Returns: Sentiment distribution, themes, insights, action items
```

### 6. NPS Calculator
Calculates Net Promoter Score.

**Tool Name:** `nps_calculator`
**Capabilities:** `nps_calculation`, `customer_support`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "scores": []float64{9, 10, 7, 8, 9, 6, 10, 5},
    },
}

output, _ := framework.ExecuteTool(ctx, "nps_calculator", input)
// Returns: NPS score, promoters/passives/detractors, category
```

### 7. Support Metrics Analyzer
Analyzes support team performance.

**Tool Name:** `support_metrics_analyzer`
**Capabilities:** `support_metrics`, `customer_support`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "metrics": map[string]interface{}{
            "first_response_time_hours": 3.5,
            "avg_resolution_time_hours": 18.0,
            "resolution_rate": 92.0,
            "csat_score": 4.3,
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "support_metrics_analyzer", input)
// Returns: Performance analysis, status, recommendations
```

### 8. CSAT Analyzer
Analyzes Customer Satisfaction scores.

**Tool Name:** `csat_analyzer`
**Capabilities:** `csat_analysis`, `customer_support`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "responses": []map[string]interface{}{
            {"score": 5, "category": "support"},
            {"score": 4, "category": "product"},
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "csat_analyzer", input)
// Returns: Average CSAT, distribution, insights
```

### 9. Ticket Router
Routes tickets to appropriate agents.

**Tool Name:** `ticket_router`
**Capabilities:** `ticket_routing`, `customer_support`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "ticket": map[string]interface{}{
            "subject": "Bug in payment system",
            "priority": "high",
        },
        "available_agents": []map[string]interface{}{
            {"name": "Agent A", "specialties": []string{"technical"}},
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "ticket_router", input)
// Returns: Recommended agent, routing reason, estimated response time
```

### 10. Knowledge Base Search
Searches knowledge base for articles.

**Tool Name:** `kb_search`
**Capabilities:** `kb_search`, `customer_support`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "query": "reset password",
        "limit": 5,
    },
}

output, _ := framework.ExecuteTool(ctx, "kb_search", input)
// Returns: Relevant articles with relevance scores
```

### 11. Customer Journey Analyzer
Analyzes customer journey touchpoints.

**Tool Name:** `customer_journey_analyzer`
**Capabilities:** `journey_analysis`, `customer_support`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "journey": []map[string]interface{}{
            {"type": "signup", "satisfaction": 4.5},
            {"type": "onboarding", "satisfaction": 3.0},
            {"type": "first_purchase", "satisfaction": 4.8},
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "customer_journey_analyzer", input)
// Returns: Pain points, optimization opportunities, journey health
```

---

## Financial Tools

### 1. Invoice Generator
Creates professional invoices.

**Tool Name:** `invoice_generator`
**Capabilities:** `invoice_generation`, `financial`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "order_data": map[string]interface{}{
            "customer": "Acme Corp",
            "items": []map[string]interface{}{
                {"name": "Service A", "price": 1000.0, "quantity": 2.0},
            },
            "tax_rate": 0.08,
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "invoice_generator", input)
// Returns: Complete invoice with number, dates, totals
```

### 2. Financial Ratio Calculator
Calculates key financial ratios.

**Tool Name:** `financial_ratio_calculator`
**Capabilities:** `financial_analysis`, `financial`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "financials": map[string]interface{}{
            "current_assets": 150000.0,
            "current_liabilities": 75000.0,
            "revenue": 500000.0,
            "net_income": 50000.0,
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "financial_ratio_calculator", input)
// Returns: Liquidity, profitability, leverage ratios with interpretations
```

### 3. Cash Flow Analyzer
Analyzes and projects cash flows.

**Tool Name:** `cash_flow_analyzer`
**Capabilities:** `cash_flow_analysis`, `financial`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "cash_flow_data": []map[string]interface{}{
            {"period": "Jan", "inflow": 50000.0, "outflow": 40000.0},
            {"period": "Feb", "inflow": 55000.0, "outflow": 42000.0},
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "cash_flow_analyzer", input)
// Returns: Net cash flow, runway, forecast, recommendations
```

### 4. Tax Calculator
Calculates tax implications.

**Tool Name:** `tax_calculator`
**Capabilities:** `tax_calculation`, `financial`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "income": 85000.0,
        "jurisdiction": "US",
    },
}

output, _ := framework.ExecuteTool(ctx, "tax_calculator", input)
// Returns: Tax owed, effective rate, breakdown, deduction suggestions
```

### 5. Pricing Optimizer
Optimizes pricing strategies.

**Tool Name:** `pricing_optimizer`
**Capabilities:** `pricing_optimization`, `financial`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "product": map[string]interface{}{
            "cost": 50.0,
            "current_price": 100.0,
        },
        "market_data": map[string]interface{}{
            "average_price": 110.0,
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "pricing_optimizer", input)
// Returns: Optimal price, scenarios, rationale
```

### 6. Budget Analyzer
Tracks budget vs actual spending.

**Tool Name:** `budget_analyzer`
**Capabilities:** `budget_analysis`, `financial`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "budget_data": map[string]interface{}{
            "budget": 100000.0,
            "actual": 95000.0,
            "categories": []map[string]interface{}{
                {"name": "Marketing", "budget": 30000.0, "actual": 35000.0},
            },
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "budget_analyzer", input)
// Returns: Variance analysis, status, forecast, recommendations
```

### 7. ROI Calculator
Calculates return on investment.

**Tool Name:** `roi_calculator`
**Capabilities:** `roi_calculation`, `financial`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "investment": map[string]interface{}{
            "initial_investment": 50000.0,
            "cash_flows": []float64{15000, 20000, 25000},
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "roi_calculator", input)
// Returns: ROI percentage, payback period, IRR, assessment
```

### 8. Expense Categorizer
Categorizes and analyzes expenses.

**Tool Name:** `expense_categorizer`
**Capabilities:** `expense_categorization`, `financial`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "expenses": []map[string]interface{}{
            {"description": "Office supplies", "amount": 250.0},
            {"description": "Flight to conference", "amount": 450.0},
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "expense_categorizer", input)
// Returns: Categorized expenses, totals by category, insights
```

### 9. Break-Even Analyzer
Calculates break-even points.

**Tool Name:** `breakeven_analyzer`
**Capabilities:** `breakeven_analysis`, `financial`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "cost_data": map[string]interface{}{
            "fixed_costs": 50000.0,
            "variable_cost_per_unit": 20.0,
            "price_per_unit": 50.0,
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "breakeven_analyzer", input)
// Returns: Break-even units/revenue, margin of safety, sensitivity analysis
```

### 10. Profitability Analyzer
Analyzes profitability by segment.

**Tool Name:** `profitability_analyzer`
**Capabilities:** `profitability_analysis`, `financial`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "data": []map[string]interface{}{
            {"segment": "Product A", "revenue": 100000.0, "cost": 60000.0},
            {"segment": "Product B", "revenue": 80000.0, "cost": 55000.0},
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "profitability_analyzer", input)
// Returns: Profitability by segment, margins, most/least profitable
```

### 11. Financial Forecaster
Generates financial forecasts.

**Tool Name:** `financial_forecaster`
**Capabilities:** `financial_forecasting`, `financial`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "historical_data": []map[string]interface{}{
            {"revenue": 100000.0, "expenses": 70000.0},
            {"revenue": 110000.0, "expenses": 75000.0},
        },
        "periods": 6,
    },
}

output, _ := framework.ExecuteTool(ctx, "financial_forecaster", input)
// Returns: Base/optimistic/pessimistic scenarios
```

### 12. Payment Terms Optimizer
Optimizes payment terms for cash flow.

**Tool Name:** `payment_terms_optimizer`
**Capabilities:** `payment_optimization`, `financial`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "accounts_data": map[string]interface{}{
            "current_dso": 45.0,
            "annual_revenue": 1000000.0,
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "payment_terms_optimizer", input)
// Returns: Recommended payment terms, cash improvement estimates
```

---

## Integration & External Tools

### 1. API Caller
Calls external APIs with authentication.

**Tool Name:** `api_caller`
**Capabilities:** `api_integration`, `integration`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "url": "https://api.example.com/data",
        "method": "GET",
        "headers": map[string]string{
            "Authorization": "Bearer token123",
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "api_caller", input)
// Returns: Status code, headers, response body
```

### 2. File Parser
Parses CSV, JSON, Excel files.

**Tool Name:** `file_parser`
**Capabilities:** `file_parsing`, `integration`

**Example:**
```go
csvContent := `name,price,stock
Product A,99.99,50
Product B,149.99,30`

input := &models.ToolInput{
    Params: map[string]interface{}{
        "content": csvContent,
        "file_type": "csv",
    },
}

output, _ := framework.ExecuteTool(ctx, "file_parser", input)
// Returns: Parsed data as structured records
```

### 3. Web Scraper
Extracts data from websites.

**Tool Name:** `web_scraper`
**Capabilities:** `web_scraping`, `integration`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "url": "https://example.com/products",
        "selectors": map[string]string{
            "title": ".product-title",
            "price": ".product-price",
        },
    },
}

output, _ := framework.ExecuteTool(ctx, "web_scraper", input)
// Returns: Scraped data by selector
```

### 4. Database Connector
Queries external databases.

**Tool Name:** `database_connector`
**Capabilities:** `database_access`, `integration`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "connection_string": "postgres://user:pass@host:5432/db",
        "query": "SELECT * FROM users LIMIT 10",
        "db_type": "postgres",
    },
}

output, _ := framework.ExecuteTool(ctx, "database_connector", input)
// Returns: Query results, row count, execution time
```

### 5. Data Sync
Synchronizes data between systems.

**Tool Name:** `data_sync`
**Capabilities:** `data_sync`, `integration`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "source_data": []map[string]interface{}{
            {"id": "1", "name": "John", "updated_at": "2024-01-01"},
        },
        "target_data": []map[string]interface{}{
            {"id": "1", "name": "John", "updated_at": "2024-01-01"},
        },
        "sync_strategy": "merge",
    },
}

output, _ := framework.ExecuteTool(ctx, "data_sync", input)
// Returns: Added/updated/deleted counts, conflicts
```

### 6. Webhook Handler
Processes incoming webhooks.

**Tool Name:** `webhook_handler`
**Capabilities:** `webhook_processing`, `integration`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "payload": map[string]interface{}{
            "event_type": "payment.succeeded",
            "amount": 100.0,
        },
        "signature": "signature_hash",
        "secret": "webhook_secret",
    },
}

output, _ := framework.ExecuteTool(ctx, "webhook_handler", input)
// Returns: Validation status, processed payload
```

### 7. Email Sender
Sends emails via SMTP.

**Tool Name:** `email_sender`
**Capabilities:** `email_sending`, `integration`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "to": "customer@example.com",
        "subject": "Order Confirmation",
        "body": "Your order has been confirmed.",
    },
}

output, _ := framework.ExecuteTool(ctx, "email_sender", input)
// Returns: Message ID, delivery status
```

### 8. Slack Notifier
Sends Slack notifications.

**Tool Name:** `slack_notifier`
**Capabilities:** `slack_integration`, `integration`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "channel": "#alerts",
        "message": "System alert: High CPU usage detected",
    },
}

output, _ := framework.ExecuteTool(ctx, "slack_notifier", input)
// Returns: Message ID, timestamp
```

### 9. Cloud Storage
Manages files in cloud storage.

**Tool Name:** `cloud_storage`
**Capabilities:** `cloud_storage`, `integration`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "operation": "upload",
        "provider": "s3",
        "file_name": "report.pdf",
    },
}

output, _ := framework.ExecuteTool(ctx, "cloud_storage", input)
// Returns: File URL, size, success status
```

### 10. Data Exporter
Exports data to various formats.

**Tool Name:** `data_exporter`
**Capabilities:** `data_export`, `integration`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "data": []map[string]interface{}{
            {"name": "Product A", "price": 99.99},
        },
        "format": "csv",
    },
}

output, _ := framework.ExecuteTool(ctx, "data_exporter", input)
// Returns: Exported content in specified format
```

### 11. Event Stream Processor
Processes event streams.

**Tool Name:** `event_stream_processor`
**Capabilities:** `event_processing`, `integration`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "events": []map[string]interface{}{
            {"event_type": "user.signup", "user_id": "123"},
        },
        "processor": "filter",
    },
}

output, _ := framework.ExecuteTool(ctx, "event_stream_processor", input)
// Returns: Processed events, filtering stats
```

### 12. OAuth Authenticator
Handles OAuth flows.

**Tool Name:** `oauth_authenticator`
**Capabilities:** `oauth_authentication`, `integration`

**Example:**
```go
input := &models.ToolInput{
    Params: map[string]interface{}{
        "provider": "google",
        "client_id": "your_client_id",
        "client_secret": "your_client_secret",
    },
}

output, _ := framework.ExecuteTool(ctx, "oauth_authenticator", input)
// Returns: Access token, refresh token, expiration
```

---

## Quick Start

### 1. Initialize Framework

```go
package main

import (
    "context"
    "log"

    "github.com/Ranganaths/minion/core"
    "github.com/Ranganaths/minion/storage"
    "github.com/Ranganaths/minion/llm"
    "github.com/Ranganaths/minion/tools/domains"
)

func main() {
    // Create framework instance
    framework := core.NewFramework(
        core.WithStorage(storage.NewInMemory()),
        core.WithLLMProvider(llm.NewOpenAI("your-api-key")),
    )

    // Register all tools
    if err := domains.RegisterAllDomainTools(framework); err != nil {
        log.Fatal(err)
    }

    log.Printf("Framework initialized with %d tools", framework.ToolRegistry().Count())
}
```

### 2. Create an Agent with Specific Capabilities

```go
ctx := context.Background()

// Create a Data Analytics Agent
analyticsAgent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name: "Analytics Agent",
    Capabilities: []string{
        "sql_generation",
        "anomaly_detection",
        "trend_prediction",
        "data_analytics",
    },
    Metadata: map[string]interface{}{
        "team": "data-science",
        "env": "production",
    },
})

if err != nil {
    log.Fatal(err)
}

log.Printf("Created agent: %s", analyticsAgent.ID)
```

### 3. Execute Tools

```go
// Get available tools for the agent
tools := framework.GetToolsForAgent(analyticsAgent)
log.Printf("Agent has access to %d tools", len(tools))

// Execute a specific tool
input := &models.ToolInput{
    Params: map[string]interface{}{
        "data": []float64{100, 105, 98, 102, 250, 99, 103},
        "sensitivity": 2.0,
    },
}

output, err := framework.ExecuteTool(ctx, "anomaly_detector", input)
if err != nil {
    log.Fatal(err)
}

log.Printf("Tool output: %+v", output.Result)
```

### 4. Complete Example: Financial Analysis Agent

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/Ranganaths/minion/core"
    "github.com/Ranganaths/minion/models"
    "github.com/Ranganaths/minion/storage"
    "github.com/Ranganaths/minion/tools/domains"
)

func main() {
    // Initialize framework
    framework := core.NewFramework(
        core.WithStorage(storage.NewInMemory()),
    )

    // Register all tools
    domains.RegisterAllDomainTools(framework)

    ctx := context.Background()

    // Create Financial Agent
    agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
        Name: "CFO Assistant",
        Capabilities: []string{
            "financial_analysis",
            "roi_calculation",
            "budget_analysis",
            "financial",
        },
    })

    // Calculate financial ratios
    ratiosInput := &models.ToolInput{
        Params: map[string]interface{}{
            "financials": map[string]interface{}{
                "current_assets": 200000.0,
                "current_liabilities": 80000.0,
                "revenue": 1000000.0,
                "net_income": 150000.0,
                "total_assets": 500000.0,
                "equity": 300000.0,
            },
        },
    }

    ratios, _ := framework.ExecuteTool(ctx, "financial_ratio_calculator", ratiosInput)
    fmt.Printf("Financial Health: %+v\n", ratios.Result)

    // Calculate ROI for an investment
    roiInput := &models.ToolInput{
        Params: map[string]interface{}{
            "investment": map[string]interface{}{
                "initial_investment": 100000.0,
                "cash_flows": []float64{30000, 40000, 50000},
            },
        },
    }

    roi, _ := framework.ExecuteTool(ctx, "roi_calculator", roiInput)
    fmt.Printf("ROI Analysis: %+v\n", roi.Result)
}
```

---

## Tool Capability Matrix

| Domain | Tools Count | Key Capabilities |
|--------|-------------|------------------|
| Data & Analytics | 10 | sql_generation, anomaly_detection, trend_prediction, data_validation |
| Customer & Support | 11 | sentiment_analysis, ticket_classification, nps_calculation, customer_health |
| Financial | 12 | financial_analysis, roi_calculation, budget_analysis, pricing_optimization |
| Integration | 12 | api_integration, file_parsing, web_scraping, cloud_storage |
| Sales | 8 | revenue_analysis, pipeline_analysis, forecasting, churn_prediction |
| Marketing | 9 | campaign_roi, funnel_analysis, attribution_analysis, lead_scoring |

**Total Tools: 62**

---

## Best Practices

### 1. Agent Design
- Assign specific capabilities that match your agent's role
- Use descriptive names and metadata for easy identification
- Group related capabilities together

### 2. Error Handling
```go
output, err := framework.ExecuteTool(ctx, "tool_name", input)
if err != nil {
    log.Printf("Tool execution failed: %v", err)
    return
}

if !output.Success {
    log.Printf("Tool reported failure: %v", output.Error)
    return
}
```

### 3. Performance
- Use in-memory storage for development
- Switch to PostgreSQL for production
- Monitor tool execution times via observability hooks

### 4. Security
- Validate all input parameters
- Use capability-based access control
- Never expose sensitive credentials in tool outputs

---

## Support & Contributing

For issues, feature requests, or contributions:
- GitHub: https://github.com/Ranganaths/minion
- Documentation: https://docs.minion-framework.com

---

## License

Minion Framework - Production-Ready AI Agent Framework
Copyright (c) 2024
