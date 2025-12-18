# Minion Framework - Quick Reference

One-page reference for Minion Framework tools and examples.

## 🚀 Quick Start

```bash
# Clone & Setup
git clone https://github.com/yourusername/minion.git
cd minion && go mod download

# Run Examples
cd examples && ./run-all-examples.sh
```

## 📦 Framework Stats

| Metric | Count |
|--------|-------|
| **Total Tools** | 80+ |
| **Domains** | 16 |
| **Platform Integrations** | 40+ |
| **Ready Examples** | 3 |

## 🛠️ Tool Categories

### Communication (9 tools)
- Slack, Teams, Discord, Gmail, Zoom, Twilio

### Project Management (9 tools)
- Jira, Asana, Trello, Linear, ClickUp, Monday

### Data & Analytics (10 tools)
- SQL, Anomaly Detection, Forecasting, Validation

### Customer Support (11 tools)
- Sentiment Analysis, Tickets, NPS, CSAT, Health

### Financial (12 tools)
- Invoices, Ratios, Cash Flow, ROI, Forecasting

### Integration (12 tools)
- API, File Parsing, Webhooks, Cloud Storage

## 💡 Example Use Cases

### DevOps Automation
```go
// GitHub PR → Jira Ticket → Slack Notification
framework.ExecuteTool(ctx, "jira_manage_issue", input)
framework.ExecuteTool(ctx, "slack_send_message", input)
```

### Customer Support
```go
// Email → Sentiment Analysis → Auto-Response
framework.ExecuteTool(ctx, "sentiment_analyzer", input)
framework.ExecuteTool(ctx, "gmail_send_email", input)
```

### Sales Pipeline
```go
// Lead → Scoring → Qualification → Notification
framework.ExecuteTool(ctx, "deal_scoring", input)
framework.ExecuteTool(ctx, "slack_send_message", input)
```

## 📝 Basic Code Template

```go
package main

import (
    "context"
    "github.com/yourusername/minion/core"
    "github.com/yourusername/minion/models"
    "github.com/yourusername/minion/storage"
    "github.com/yourusername/minion/tools/domains"
)

func main() {
    // 1. Initialize
    framework := core.NewFramework(
        core.WithStorage(storage.NewInMemory()),
    )

    // 2. Register tools
    domains.RegisterAllDomainTools(framework)

    ctx := context.Background()

    // 3. Create agent
    agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
        Name: "My Agent",
        Capabilities: []string{
            "slack_integration",
            "jira_integration",
        },
    })

    // 4. Execute tools
    output, _ := framework.ExecuteTool(ctx, "tool_name", &models.ToolInput{
        Params: map[string]interface{}{
            "param": "value",
        },
    })

    // 5. Use results
    result := output.Result
}
```

## 🔧 Common Patterns

### Pattern 1: Notification
```go
framework.ExecuteTool(ctx, "slack_send_message", &models.ToolInput{
    Params: map[string]interface{}{
        "channel": "#team",
        "message": "Alert!",
    },
})
```

### Pattern 2: Data Processing
```go
// Analyze → Process → Store
sentimentOutput, _ := framework.ExecuteTool(ctx, "sentiment_analyzer", input)
// Process results...
```

### Pattern 3: Multi-Tool Workflow
```go
// Tool 1 → Tool 2 → Tool 3
output1, _ := framework.ExecuteTool(ctx, "tool1", input1)
input2.Params["data"] = output1.Result
output2, _ := framework.ExecuteTool(ctx, "tool2", input2)
```

## 📊 Agent Capabilities

Add these to agent creation:

```go
Capabilities: []string{
    // Communication
    "slack_integration",
    "gmail_integration",
    "teams_integration",

    // Project Management
    "jira_integration",
    "asana_integration",
    "trello_integration",

    // Analytics
    "data_analytics",
    "sentiment_analysis",
    "revenue_analysis",

    // Financial
    "financial_analysis",
    "roi_calculation",

    // General
    "communication",
    "project_management",
    "customer_support",
}
```

## 🎯 Tool Parameters

### Slack
```go
Params: map[string]interface{}{
    "channel": "#channel-name",
    "message": "text",
    "attachments": []map[string]interface{}{...},
}
```

### Jira
```go
Params: map[string]interface{}{
    "action": "create",
    "project_key": "PROJ",
    "issue_type": "Task",
    "summary": "Title",
}
```

### Gmail
```go
Params: map[string]interface{}{
    "to": "email@example.com",
    "subject": "Subject",
    "body": "Message",
}
```

## 🐛 Troubleshooting

| Issue | Solution |
|-------|----------|
| Tool not found | `domains.RegisterAllDomainTools(framework)` |
| Agent can't execute | Add capability to agent |
| Invalid params | Check parameter types |
| Connection failed | Verify API credentials |

## 📚 Documentation

- **Main Guide:** `README.md`
- **Tools Guide:** `TOOLS_GUIDE.md`
- **Composio Tools:** `COMPOSIO_STYLE_TOOLS.md`
- **Examples:** `examples/README.md`
- **Integration Guide:** `INTEGRATION_EXAMPLES.md`

## 🔗 Quick Links

```bash
# Run specific example
cd examples/devops-automation && go run main.go
cd examples/customer-support && go run main.go
cd examples/sales-automation && go run main.go

# Run all examples
cd examples && ./run-all-examples.sh

# Test framework
go test ./...
```

## 💻 Environment Variables

```bash
# .env file
SLACK_BOT_TOKEN=xoxb-...
JIRA_API_TOKEN=...
GMAIL_API_KEY=...
TWILIO_AUTH_TOKEN=...
```

## 📈 Performance

| Operation | Time | Memory |
|-----------|------|--------|
| Tool Execution | <100ms | ~5MB |
| Agent Creation | <50ms | ~2MB |
| Full Workflow | 5-10s | ~40MB |

## 🎓 Learning Path

1. ✅ Run examples
2. ✅ Modify parameters
3. ✅ Add new workflows
4. ✅ Connect real APIs
5. ✅ Build custom tools
6. ✅ Deploy to production

## 🤝 Support

- **GitHub:** https://github.com/yourusername/minion
- **Discord:** https://discord.gg/minion
- **Docs:** https://docs.minion-framework.com

---

**Ready to automate? Start with the examples! 🚀**
