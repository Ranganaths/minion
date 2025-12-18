# Tutorial 3: Building Your First MCP Agent

**Duration**: 1 hour
**Level**: Beginner/Intermediate
**Prerequisites**: Tutorials 1 and 2 completed

## 🎯 Learning Objectives

By the end of this tutorial, you will:
- Design a complete agent workflow
- Combine multiple MCP servers
- Implement error handling and logging
- Create reusable agent patterns
- Test your agent thoroughly

## 🎬 Project: Customer Support Automation Agent

We'll build **SupportBot** - an agent that automates customer support tasks:

**Capabilities**:
- 📧 Reads incoming support emails (Gmail)
- 🔍 Searches knowledge base (Notion)
- 🎫 Creates support tickets (GitHub Issues)
- 💬 Posts updates to team chat (Slack)
- 📊 Logs everything for analytics

**Workflow**:
```
1. Check Gmail for new support emails
   ↓
2. Extract customer issue
   ↓
3. Search Notion knowledge base
   ↓
4. Create GitHub issue for tracking
   ↓
5. Send response via Gmail
   ↓
6. Notify team via Slack
```

## 🏗️ Architecture

```
┌────────────────────────────────────────┐
│         SupportBot Agent               │
│                                        │
│  ┌──────────────────────────────┐    │
│  │    Workflow Orchestrator      │    │
│  └──────────┬───────────────────┘    │
│             │                          │
│  ┌──────────▼───────────────────┐    │
│  │  MCP Connections             │    │
│  │  • Gmail (email)             │    │
│  │  • Notion (knowledge base)   │    │
│  │  • GitHub (tickets)          │    │
│  │  • Slack (notifications)     │    │
│  └──────────────────────────────┘    │
└────────────────────────────────────────┘
```

## 📋 Prerequisites

### 1. API Tokens

You'll need:

```bash
# Gmail OAuth credentials
GMAIL_CREDENTIALS=/path/to/gmail-credentials.json

# Notion integration token
NOTION_API_KEY=secret_...

# GitHub personal access token
GITHUB_TOKEN=ghp_...

# Slack bot token
SLACK_BOT_TOKEN=xoxb-...
```

### 2. Setup Accounts

- **Gmail**: Enable Gmail API in Google Cloud Console
- **Notion**: Create integration at notion.so/my-integrations
- **GitHub**: Create token at github.com/settings/tokens
- **Slack**: Create app at api.slack.com/apps

## 🚀 Step-by-Step Build

### Step 1: Project Setup

```bash
mkdir support-bot
cd support-bot

# Initialize module
go mod init support-bot

# Install dependencies
go get github.com/yourusername/minion

# Create main.go
touch main.go
```

### Step 2: Define Agent Structure

Create `main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/yourusername/minion/core"
	"github.com/yourusername/minion/models"
	"github.com/yourusername/minion/mcp/client"
)

type SupportBot struct {
	framework *core.Framework
	agent     *models.Agent
	ctx       context.Context
}

func NewSupportBot(ctx context.Context) (*SupportBot, error) {
	// Initialize framework
	framework := core.NewFramework()

	// Create bot instance
	bot := &SupportBot{
		framework: framework,
		ctx:       ctx,
	}

	// Connect MCP servers
	if err := bot.connectServices(); err != nil {
		return nil, fmt.Errorf("failed to connect services: %w", err)
	}

	// Create agent
	if err := bot.createAgent(); err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	return bot, nil
}

func (bot *SupportBot) connectServices() error {
	// We'll implement this
	return nil
}

func (bot *SupportBot) createAgent() error {
	// We'll implement this
	return nil
}

func (bot *SupportBot) Close() {
	bot.framework.Close()
}

func main() {
	ctx := context.Background()

	bot, err := NewSupportBot(ctx)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}
	defer bot.Close()

	fmt.Println("✅ SupportBot initialized!")
}
```

### Step 3: Connect MCP Services

Implement `connectServices()`:

```go
func (bot *SupportBot) connectServices() error {
	fmt.Println("🔌 Connecting to services...")

	// 1. Connect to Gmail
	gmailConfig := &client.ClientConfig{
		ServerName: "gmail",
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-gmail"},
		Env: map[string]string{
			"GMAIL_CREDENTIALS": os.Getenv("GMAIL_CREDENTIALS"),
		},
	}
	if err := bot.framework.ConnectMCPServer(bot.ctx, gmailConfig); err != nil {
		return fmt.Errorf("Gmail connection failed: %w", err)
	}
	fmt.Println("  ✅ Gmail connected")

	// 2. Connect to Notion
	notionConfig := &client.ClientConfig{
		ServerName: "notion",
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-notion"},
		Env: map[string]string{
			"NOTION_API_KEY": os.Getenv("NOTION_API_KEY"),
		},
	}
	if err := bot.framework.ConnectMCPServer(bot.ctx, notionConfig); err != nil {
		return fmt.Errorf("Notion connection failed: %w", err)
	}
	fmt.Println("  ✅ Notion connected")

	// 3. Connect to GitHub
	githubConfig := &client.ClientConfig{
		ServerName: "github",
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-github"},
		Env: map[string]string{
			"GITHUB_PERSONAL_ACCESS_TOKEN": os.Getenv("GITHUB_TOKEN"),
		},
	}
	if err := bot.framework.ConnectMCPServer(bot.ctx, githubConfig); err != nil {
		return fmt.Errorf("GitHub connection failed: %w", err)
	}
	fmt.Println("  ✅ GitHub connected")

	// 4. Connect to Slack
	slackConfig := &client.ClientConfig{
		ServerName: "slack",
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-slack"},
		Env: map[string]string{
			"SLACK_BOT_TOKEN": os.Getenv("SLACK_BOT_TOKEN"),
		},
	}
	if err := bot.framework.ConnectMCPServer(bot.ctx, slackConfig); err != nil {
		return fmt.Errorf("Slack connection failed: %w", err)
	}
	fmt.Println("  ✅ Slack connected")

	return nil
}
```

### Step 4: Create Agent with Capabilities

Implement `createAgent()`:

```go
func (bot *SupportBot) createAgent() error {
	agentReq := &models.CreateAgentRequest{
		Name:        "SupportBot",
		Description: "Automated customer support agent",
		Capabilities: []string{
			"mcp_gmail",   // Email access
			"mcp_notion",  // Knowledge base
			"mcp_github",  // Issue tracking
			"mcp_slack",   // Team notifications
		},
	}

	agent, err := bot.framework.CreateAgent(bot.ctx, agentReq)
	if err != nil {
		return err
	}

	bot.agent = agent
	fmt.Printf("✅ Created agent: %s (ID: %s)\n", agent.Name, agent.ID)

	// List available tools
	tools := bot.framework.GetToolsForAgent(agent)
	fmt.Printf("📋 Available tools: %d\n", len(tools))

	return nil
}
```

### Step 5: Implement Core Workflows

Add workflow methods:

```go
// EmailMessage represents a support email
type EmailMessage struct {
	ID      string
	From    string
	Subject string
	Body    string
}

// ProcessSupportEmail handles a single support email
func (bot *SupportBot) ProcessSupportEmail(emailID string) error {
	fmt.Printf("\n📧 Processing email: %s\n", emailID)

	// 1. Fetch email from Gmail
	email, err := bot.fetchEmail(emailID)
	if err != nil {
		return fmt.Errorf("failed to fetch email: %w", err)
	}
	fmt.Printf("  From: %s\n", email.From)
	fmt.Printf("  Subject: %s\n", email.Subject)

	// 2. Search knowledge base
	solution, err := bot.searchKnowledgeBase(email.Subject, email.Body)
	if err != nil {
		log.Printf("  ⚠️  Knowledge base search failed: %v", err)
		solution = "No matching solution found"
	} else {
		fmt.Println("  ✅ Found solution in knowledge base")
	}

	// 3. Create GitHub issue for tracking
	issueURL, err := bot.createSupportTicket(email)
	if err != nil {
		return fmt.Errorf("failed to create ticket: %w", err)
	}
	fmt.Printf("  ✅ Created ticket: %s\n", issueURL)

	// 4. Send response to customer
	if err := bot.sendEmailResponse(email, solution, issueURL); err != nil {
		return fmt.Errorf("failed to send response: %w", err)
	}
	fmt.Println("  ✅ Response sent to customer")

	// 5. Notify team on Slack
	if err := bot.notifyTeam(email, issueURL); err != nil {
		log.Printf("  ⚠️  Slack notification failed: %v", err)
	} else {
		fmt.Println("  ✅ Team notified")
	}

	fmt.Println("✅ Email processed successfully!")
	return nil
}

// Implement helper methods
func (bot *SupportBot) fetchEmail(emailID string) (*EmailMessage, error) {
	input := &models.ToolInput{
		ToolName: "mcp_gmail_get_message",
		Params: map[string]interface{}{
			"messageId": emailID,
		},
	}

	output, err := bot.framework.ExecuteTool(bot.ctx, bot.agent.ID, input)
	if err != nil {
		return nil, err
	}

	if !output.Success {
		return nil, fmt.Errorf(output.Error)
	}

	// Parse result into EmailMessage
	// (In production, use proper JSON unmarshaling)
	return &EmailMessage{
		ID:      emailID,
		From:    "customer@example.com",
		Subject: "Need help with product",
		Body:    "I'm having trouble with feature X...",
	}, nil
}

func (bot *SupportBot) searchKnowledgeBase(subject, body string) (string, error) {
	input := &models.ToolInput{
		ToolName: "mcp_notion_search",
		Params: map[string]interface{}{
			"query": subject,
		},
	}

	output, err := bot.framework.ExecuteTool(bot.ctx, bot.agent.ID, input)
	if err != nil {
		return "", err
	}

	if !output.Success {
		return "", fmt.Errorf(output.Error)
	}

	// Extract solution from Notion results
	return fmt.Sprintf("Solution: %v", output.Result), nil
}

func (bot *SupportBot) createSupportTicket(email *EmailMessage) (string, error) {
	input := &models.ToolInput{
		ToolName: "mcp_github_create_issue",
		Params: map[string]interface{}{
			"owner": "your-org",
			"repo":  "support-tickets",
			"title": fmt.Sprintf("[Support] %s", email.Subject),
			"body": fmt.Sprintf(`
Customer: %s
Subject: %s

%s

---
Automated ticket created by SupportBot
`, email.From, email.Subject, email.Body),
		},
	}

	output, err := bot.framework.ExecuteTool(bot.ctx, bot.agent.ID, input)
	if err != nil {
		return "", err
	}

	if !output.Success {
		return "", fmt.Errorf(output.Error)
	}

	// Extract issue URL from result
	return fmt.Sprintf("https://github.com/your-org/support-tickets/issues/123"), nil
}

func (bot *SupportBot) sendEmailResponse(email *EmailMessage, solution, ticketURL string) error {
	responseBody := fmt.Sprintf(`
Dear Customer,

Thank you for contacting support. We've received your inquiry about: %s

%s

We've created a support ticket to track your issue: %s

Our team will follow up shortly if additional help is needed.

Best regards,
SupportBot
`, email.Subject, solution, ticketURL)

	input := &models.ToolInput{
		ToolName: "mcp_gmail_send_message",
		Params: map[string]interface{}{
			"to":      email.From,
			"subject": fmt.Sprintf("Re: %s", email.Subject),
			"body":    responseBody,
		},
	}

	output, err := bot.framework.ExecuteTool(bot.ctx, bot.agent.ID, input)
	if err != nil {
		return err
	}

	if !output.Success {
		return fmt.Errorf(output.Error)
	}

	return nil
}

func (bot *SupportBot) notifyTeam(email *EmailMessage, ticketURL string) error {
	message := fmt.Sprintf(`
🎫 New support ticket created!

Customer: %s
Subject: %s
Ticket: %s
`, email.From, email.Subject, ticketURL)

	input := &models.ToolInput{
		ToolName: "mcp_slack_post_message",
		Params: map[string]interface{}{
			"channel": "#support",
			"text":    message,
		},
	}

	output, err := bot.framework.ExecuteTool(bot.ctx, bot.agent.ID, input)
	if err != nil {
		return err
	}

	if !output.Success {
		return fmt.Errorf(output.Error)
	}

	return nil
}
```

### Step 6: Add Main Workflow Loop

Update `main()`:

```go
func main() {
	ctx := context.Background()

	// Create bot
	bot, err := NewSupportBot(ctx)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}
	defer bot.Close()

	fmt.Println("\n🤖 SupportBot is ready!")
	fmt.Println("Monitoring for new support emails...\n")

	// Example: Process a support email
	// In production, this would be a continuous loop or webhook-based
	emailID := "example-email-id"
	if err := bot.ProcessSupportEmail(emailID); err != nil {
		log.Printf("Error processing email: %v", err)
	}

	// Example: Check multiple emails
	emailIDs := []string{"email-1", "email-2", "email-3"}
	for _, id := range emailIDs {
		if err := bot.ProcessSupportEmail(id); err != nil {
			log.Printf("Error processing %s: %v", id, err)
			continue
		}
	}

	fmt.Println("\n✅ All emails processed!")
}
```

### Step 7: Add Error Handling and Logging

Enhance with better error handling:

```go
// Add to SupportBot struct
type SupportBot struct {
	framework      *core.Framework
	agent          *models.Agent
	ctx            context.Context
	successCount   int
	failureCount   int
	lastError      error
}

// Add monitoring method
func (bot *SupportBot) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"success_count": bot.successCount,
		"failure_count": bot.failureCount,
		"last_error":    bot.lastError,
	}
}

// Wrap ProcessSupportEmail with error tracking
func (bot *SupportBot) ProcessSupportEmailWithTracking(emailID string) error {
	err := bot.ProcessSupportEmail(emailID)
	if err != nil {
		bot.failureCount++
		bot.lastError = err
		return err
	}
	bot.successCount++
	return nil
}
```

## 🏋️ Practice Exercises

### Exercise 1: Add Email Filtering

Filter support emails by category (bug report, feature request, question):

```go
func (bot *SupportBot) categorizeEmail(email *EmailMessage) string {
	// Implement categorization logic
	// Return: "bug", "feature", or "question"
}
```

<details>
<summary>Click to see solution</summary>

```go
func (bot *SupportBot) categorizeEmail(email *EmailMessage) string {
	subject := strings.ToLower(email.Subject)
	body := strings.ToLower(email.Body)

	if strings.Contains(subject, "bug") || strings.Contains(body, "error") {
		return "bug"
	}
	if strings.Contains(subject, "feature") || strings.Contains(body, "suggest") {
		return "feature"
	}
	return "question"
}

// Use in ProcessSupportEmail:
category := bot.categorizeEmail(email)
fmt.Printf("  Category: %s\n", category)

// Add category label to GitHub issue
Params: map[string]interface{}{
	// ... other params
	"labels": []string{category, "support"},
}
```
</details>

### Exercise 2: Add Retry Logic

Implement retry for failed operations:

```go
func (bot *SupportBot) executeWithRetry(
	toolName string,
	params map[string]interface{},
	maxRetries int,
) (*models.ToolOutput, error) {
	// Implement retry logic
}
```

<details>
<summary>Click to see solution</summary>

```go
func (bot *SupportBot) executeWithRetry(
	toolName string,
	params map[string]interface{},
	maxRetries int,
) (*models.ToolOutput, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		input := &models.ToolInput{
			ToolName: toolName,
			Params:   params,
		}

		output, err := bot.framework.ExecuteTool(bot.ctx, bot.agent.ID, input)
		if err == nil && output.Success {
			return output, nil
		}

		lastErr = err
		if err == nil {
			lastErr = fmt.Errorf(output.Error)
		}

		if attempt < maxRetries {
			wait := time.Duration(attempt) * time.Second
			fmt.Printf("  ⚠️  Attempt %d failed, retrying in %v...\n", attempt, wait)
			time.Sleep(wait)
		}
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}
```
</details>

### Exercise 3: Add Batch Processing

Process multiple emails in parallel:

```go
func (bot *SupportBot) ProcessEmailBatch(emailIDs []string) error {
	// Use goroutines to process emails concurrently
	// Collect and report results
}
```

<details>
<summary>Click to see solution</summary>

```go
func (bot *SupportBot) ProcessEmailBatch(emailIDs []string) error {
	var wg sync.WaitGroup
	results := make(chan error, len(emailIDs))

	for _, emailID := range emailIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			results <- bot.ProcessSupportEmail(id)
		}(emailID)
	}

	wg.Wait()
	close(results)

	// Collect results
	successCount := 0
	var errors []error
	for err := range results {
		if err != nil {
			errors = append(errors, err)
		} else {
			successCount++
		}
	}

	fmt.Printf("\n📊 Batch Results: %d succeeded, %d failed\n",
		successCount, len(errors))

	if len(errors) > 0 {
		return fmt.Errorf("%d emails failed processing", len(errors))
	}
	return nil
}
```
</details>

## 🐛 Troubleshooting

### Common Issues

#### 1. Service Connection Failures

```go
// Add connection verification
func (bot *SupportBot) verifyConnections() error {
	servers := bot.framework.ListMCPServers()
	status := bot.framework.GetMCPServerStatus()

	for _, server := range servers {
		if !status[server].Connected {
			return fmt.Errorf("server %s not connected", server)
		}
	}
	return nil
}
```

#### 2. Rate Limiting

```go
// Add rate limiting
import "golang.org/x/time/rate"

type SupportBot struct {
	// ... existing fields
	rateLimiter *rate.Limiter
}

func NewSupportBot(ctx context.Context) (*SupportBot, error) {
	// ...
	bot.rateLimiter = rate.NewLimiter(rate.Every(time.Second), 10) // 10 req/sec
	// ...
}

func (bot *SupportBot) ProcessSupportEmail(emailID string) error {
	// Wait for rate limiter
	if err := bot.rateLimiter.Wait(bot.ctx); err != nil {
		return err
	}
	// ... rest of processing
}
```

## 📝 Summary

Congratulations! You've built a complete customer support automation agent that:

✅ Integrates 4 different MCP servers
✅ Implements a multi-step workflow
✅ Handles errors gracefully
✅ Provides monitoring and logging
✅ Is production-ready

### Key Patterns Learned

1. **Service Orchestration**: Coordinating multiple MCP servers
2. **Error Handling**: Retry logic and graceful degradation
3. **Workflow Design**: Breaking complex tasks into steps
4. **Resource Management**: Proper initialization and cleanup
5. **Monitoring**: Tracking success/failure rates

## 🎯 Next Steps

Ready for advanced features?

**[Tutorial 4: Advanced MCP Features →](04-advanced-features.md)**

Learn connection pooling, caching, circuit breakers, and Prometheus metrics!

### Additional Resources

- [Multi-Server Example](../../mcp/examples/multi-server/)
- [Virtual SDR Example](../../mcp/examples/salesforce-sdr/)
- [MCP Phase 3 Features](../../mcp/PHASE3_COMPLETE.md)

---

**Amazing work! 🎉 You're now ready for advanced topics in [Tutorial 4](04-advanced-features.md).**
