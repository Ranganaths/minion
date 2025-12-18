# Tutorial 5: Multi-Server Orchestration

**Duration**: 1 hour
**Level**: Intermediate
**Prerequisites**: Tutorials 1-4

## 🎯 Learning Objectives

By the end of this tutorial, you will:
- Connect and manage multiple MCP servers simultaneously
- Coordinate workflows across different services
- Handle cross-server dependencies
- Implement parallel and sequential execution patterns
- Build a real multi-server automation workflow

## 📚 What is Multi-Server Orchestration?

**Single Server**: Agent uses one service (e.g., only GitHub)

**Multi-Server Orchestration**: Agent coordinates multiple services together:
- Read from GitHub → Write to Slack
- Get email from Gmail → Create Notion page → Update Salesforce
- Parallel: Call GitHub AND Slack simultaneously

### Real-World Analogy

Single server = Having only a hammer (every problem looks like a nail)

Multi-server = Full toolbox (use the right tool for each job):
- **GitHub** = Source control
- **Slack** = Communication
- **Gmail** = Email
- **Notion** = Documentation
- **Salesforce** = CRM

## 🏗️ Multi-Server Architecture

```
┌─────────────────────────────────────────────────┐
│              Your Agent                          │
│         "SupportBot Orchestrator"                │
└──────┬──────────┬──────────┬──────────┬─────────┘
       │          │          │          │
   ┌───▼───┐  ┌──▼───┐  ┌───▼───┐  ┌──▼───┐
   │GitHub │  │Slack │  │Gmail  │  │Notion│
   │Server │  │Server│  │Server │  │Server│
   └───────┘  └──────┘  └───────┘  └──────┘
       │          │          │          │
   ┌───▼────────▼──────────▼──────────▼───┐
   │      External APIs & Services         │
   │  (GitHub, Slack, Gmail, Notion, ...)  │
   └───────────────────────────────────────┘
```

## 🛠️ Setup

### Step 1: Install Multiple MCP Servers

```bash
# Install MCP servers we'll use
npm install -g @modelcontextprotocol/server-github
npm install -g @modelcontextprotocol/server-slack
npm install -g @modelcontextprotocol/server-gmail
npm install -g @modelcontextprotocol/server-notion
```

### Step 2: Get API Tokens

You'll need:
- **GitHub**: Personal Access Token (https://github.com/settings/tokens)
- **Slack**: Bot Token (https://api.slack.com/apps)
- **Gmail**: OAuth2 Credentials (https://console.cloud.google.com)
- **Notion**: API Key (https://www.notion.so/my-integrations)

### Step 3: Set Environment Variables

```bash
export GITHUB_PERSONAL_ACCESS_TOKEN="ghp_your_token_here"
export SLACK_BOT_TOKEN="xoxb-your-token-here"
export GMAIL_CREDENTIALS="/path/to/gmail-credentials.json"
export NOTION_API_KEY="secret_your_key_here"
```

## 📖 Part 1: Connecting Multiple Servers

### Basic Multi-Server Setup

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Ranganaths/minion/mcp/client"
)

func main() {
	ctx := context.Background()

	// 1. Create MCP Client Manager
	manager := client.NewMCPClientManager()
	defer manager.Close()

	fmt.Println("📡 Connecting to multiple MCP servers...\n")

	// 2. Define Server Configurations
	servers := []struct {
		name   string
		config *client.ClientConfig
	}{
		{
			name: "GitHub",
			config: &client.ClientConfig{
				ServerName: "github",
				Transport:  client.TransportStdio,
				Command:    "npx",
				Args:       []string{"-y", "@modelcontextprotocol/server-github"},
				Env: map[string]string{
					"GITHUB_PERSONAL_ACCESS_TOKEN": os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN"),
				},
			},
		},
		{
			name: "Slack",
			config: &client.ClientConfig{
				ServerName: "slack",
				Transport:  client.TransportStdio,
				Command:    "npx",
				Args:       []string{"-y", "@modelcontextprotocol/server-slack"},
				Env: map[string]string{
					"SLACK_BOT_TOKEN": os.Getenv("SLACK_BOT_TOKEN"),
				},
			},
		},
		{
			name: "Gmail",
			config: &client.ClientConfig{
				ServerName: "gmail",
				Transport:  client.TransportStdio,
				Command:    "npx",
				Args:       []string{"-y", "@modelcontextprotocol/server-gmail"},
				Env: map[string]string{
					"GMAIL_CREDENTIALS": os.Getenv("GMAIL_CREDENTIALS"),
				},
			},
		},
		{
			name: "Notion",
			config: &client.ClientConfig{
				ServerName: "notion",
				Transport:  client.TransportStdio,
				Command:    "npx",
				Args:       []string{"-y", "@modelcontextprotocol/server-notion"},
				Env: map[string]string{
					"NOTION_API_KEY": os.Getenv("NOTION_API_KEY"),
				},
			},
		},
	}

	// 3. Connect to All Servers
	for _, server := range servers {
		fmt.Printf("Connecting to %s...\n", server.name)

		err := manager.Connect(ctx, server.config)
		if err != nil {
			log.Printf("⚠️ Failed to connect to %s: %v\n", server.name, err)
			continue
		}

		fmt.Printf("✅ Connected to %s\n\n", server.name)
	}

	// 4. List Connected Servers
	connected := manager.ListServers()
	fmt.Printf("📋 Connected Servers: %v\n", connected)

	// 5. Get Tools from Each Server
	for _, serverName := range connected {
		tools, err := manager.GetServerTools(serverName)
		if err != nil {
			continue
		}

		fmt.Printf("\n%s Tools (%d):\n", serverName, len(tools))
		for i, tool := range tools {
			if i < 3 { // Show first 3
				fmt.Printf("  • %s\n", tool.Name)
			}
		}
		if len(tools) > 3 {
			fmt.Printf("  ... and %d more\n", len(tools)-3)
		}
	}
}
```

**Output:**
```
📡 Connecting to multiple MCP servers...

Connecting to GitHub...
✅ Connected to GitHub

Connecting to Slack...
✅ Connected to Slack

Connecting to Gmail...
✅ Connected to Gmail

Connecting to Notion...
✅ Connected to Notion

📋 Connected Servers: [github slack gmail notion]

github Tools (15):
  • mcp_github_create_issue
  • mcp_github_create_pr
  • mcp_github_list_repos
  ... and 12 more

slack Tools (8):
  • mcp_slack_post_message
  • mcp_slack_list_channels
  • mcp_slack_get_channel_history
  ... and 5 more
```

## 📖 Part 2: Sequential Workflows

### Pattern: A → B → C

Execute tools in order, passing data between steps.

### Example: GitHub Issue → Slack Notification

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Ranganaths/minion/mcp/client"
)

func main() {
	ctx := context.Background()

	// Setup (from Part 1)
	manager := client.NewMCPClientManager()
	defer manager.Close()

	// Connect GitHub and Slack
	connectServers(ctx, manager)

	// Sequential Workflow
	fmt.Println("\n🔄 Starting Sequential Workflow\n")

	// Step 1: Create GitHub Issue
	fmt.Println("Step 1: Creating GitHub issue...")

	issueResult, err := manager.CallTool(ctx, "github", "mcp_github_create_issue", map[string]interface{}{
		"owner": "yourusername",
		"repo":  "yourrepo",
		"title": "Bug: Login button not working",
		"body":  "The login button returns a 500 error when clicked.",
	})

	if err != nil {
		log.Fatalf("Failed to create issue: %v", err)
	}

	// Extract issue number from result
	issueNumber := issueResult["number"].(int)
	issueURL := issueResult["html_url"].(string)

	fmt.Printf("✅ Created issue #%d\n", issueNumber)
	fmt.Printf("   URL: %s\n\n", issueURL)

	// Step 2: Post to Slack
	fmt.Println("Step 2: Posting notification to Slack...")

	slackMessage := fmt.Sprintf(
		"🐛 New Bug Report\n"+
			"Issue #%d: Bug: Login button not working\n"+
			"View: %s",
		issueNumber,
		issueURL,
	)

	slackResult, err := manager.CallTool(ctx, "slack", "mcp_slack_post_message", map[string]interface{}{
		"channel": "#bugs",
		"text":    slackMessage,
	})

	if err != nil {
		log.Fatalf("Failed to post to Slack: %v", err)
	}

	fmt.Printf("✅ Posted to Slack channel #bugs\n")
	fmt.Printf("   Message ID: %s\n", slackResult["ts"])

	fmt.Println("\n✅ Sequential workflow completed!")
}
```

### Example: Email → Notion → Salesforce

```go
// Read email, extract lead info, save to Notion, create Salesforce lead

// Step 1: Get latest email
emailResult, _ := manager.CallTool(ctx, "gmail", "mcp_gmail_get_latest", map[string]interface{}{
	"query": "from:potential-customer@example.com",
})

email := emailResult["message"].(map[string]interface{})
sender := email["from"].(string)
subject := email["subject"].(string)

// Step 2: Create Notion page
notionResult, _ := manager.CallTool(ctx, "notion", "mcp_notion_create_page", map[string]interface{}{
	"database_id": "your-database-id",
	"properties": map[string]interface{}{
		"Lead Email": sender,
		"Subject":    subject,
		"Status":     "New",
	},
})

notionPageID := notionResult["id"].(string)

// Step 3: Create Salesforce lead
salesforceResult, _ := manager.CallTool(ctx, "salesforce", "mcp_salesforce_create_lead", map[string]interface{}{
	"email":    sender,
	"company":  extractCompany(sender),
	"notes":    fmt.Sprintf("Notion: %s", notionPageID),
	"source":   "Email Inbound",
})

fmt.Printf("✅ Lead created: %s\n", salesforceResult["id"])
```

## 📖 Part 3: Parallel Workflows

### Pattern: Execute A, B, C Simultaneously

Use goroutines to call multiple servers at once.

### Example: Post to Slack AND Email AND Notion

```go
package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/Ranganaths/minion/mcp/client"
)

func main() {
	ctx := context.Background()

	manager := client.NewMCPClientManager()
	defer manager.Close()

	connectServers(ctx, manager)

	// Parallel Workflow
	fmt.Println("\n⚡ Starting Parallel Workflow\n")

	announcement := "🚀 Version 2.0 Released! Check out the new features."

	var wg sync.WaitGroup
	results := make(map[string]interface{})
	var mu sync.Mutex

	// Launch 3 parallel operations
	wg.Add(3)

	// 1. Post to Slack
	go func() {
		defer wg.Done()
		fmt.Println("📤 Posting to Slack...")

		result, err := manager.CallTool(ctx, "slack", "mcp_slack_post_message", map[string]interface{}{
			"channel": "#announcements",
			"text":    announcement,
		})

		mu.Lock()
		if err != nil {
			results["slack"] = fmt.Sprintf("Error: %v", err)
		} else {
			results["slack"] = "Success"
		}
		mu.Unlock()

		fmt.Println("✅ Slack done")
	}()

	// 2. Send Email
	go func() {
		defer wg.Done()
		fmt.Println("📤 Sending email...")

		result, err := manager.CallTool(ctx, "gmail", "mcp_gmail_send_message", map[string]interface{}{
			"to":      "team@company.com",
			"subject": "Version 2.0 Released",
			"body":    announcement,
		})

		mu.Lock()
		if err != nil {
			results["gmail"] = fmt.Sprintf("Error: %v", err)
		} else {
			results["gmail"] = "Success"
		}
		mu.Unlock()

		fmt.Println("✅ Email done")
	}()

	// 3. Create Notion page
	go func() {
		defer wg.Done()
		fmt.Println("📤 Creating Notion page...")

		result, err := manager.CallTool(ctx, "notion", "mcp_notion_create_page", map[string]interface{}{
			"database_id": "announcements-db",
			"properties": map[string]interface{}{
				"Title":   "Version 2.0 Released",
				"Content": announcement,
				"Date":    time.Now().Format("2006-01-02"),
			},
		})

		mu.Lock()
		if err != nil {
			results["notion"] = fmt.Sprintf("Error: %v", err)
		} else {
			results["notion"] = "Success"
		}
		mu.Unlock()

		fmt.Println("✅ Notion done")
	}()

	// Wait for all to complete
	wg.Wait()

	fmt.Println("\n📊 Results:")
	for service, result := range results {
		fmt.Printf("  %s: %v\n", service, result)
	}

	fmt.Println("\n✅ Parallel workflow completed!")
}
```

**Output:**
```
⚡ Starting Parallel Workflow

📤 Posting to Slack...
📤 Sending email...
📤 Creating Notion page...
✅ Slack done
✅ Notion done
✅ Email done

📊 Results:
  slack: Success
  gmail: Success
  notion: Success

✅ Parallel workflow completed!
```

## 📖 Part 4: Complex Orchestration

### Real-World Example: Support Ticket Handler

Combines sequential and parallel execution.

```go
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Ranganaths/minion/mcp/client"
)

type SupportTicket struct {
	ID       int
	Title    string
	Body     string
	Customer string
	Priority string
}

func handleSupportTicket(ctx context.Context, manager *client.MCPClientManager, ticket *SupportTicket) error {
	fmt.Printf("\n🎫 Handling Support Ticket #%d\n", ticket.ID)
	fmt.Printf("   Customer: %s\n", ticket.Customer)
	fmt.Printf("   Priority: %s\n\n", ticket.Priority)

	// Step 1: Create GitHub Issue (sequential)
	fmt.Println("Step 1: Creating GitHub issue...")

	issueResult, err := manager.CallTool(ctx, "github", "mcp_github_create_issue", map[string]interface{}{
		"owner": "company",
		"repo":  "support",
		"title": ticket.Title,
		"body": fmt.Sprintf(
			"**Customer:** %s\n**Priority:** %s\n\n%s",
			ticket.Customer,
			ticket.Priority,
			ticket.Body,
		),
		"labels": []string{"support", ticket.Priority},
	})

	if err != nil {
		return fmt.Errorf("failed to create issue: %w", err)
	}

	issueNumber := issueResult["number"].(int)
	issueURL := issueResult["html_url"].(string)

	fmt.Printf("✅ Created issue #%d\n\n", issueNumber)

	// Step 2: Parallel notifications
	fmt.Println("Step 2: Sending notifications (parallel)...")

	var wg sync.WaitGroup
	errors := make([]error, 0)
	var mu sync.Mutex

	// 2a. Notify Slack
	wg.Add(1)
	go func() {
		defer wg.Done()

		slackMsg := fmt.Sprintf(
			"🎫 New Support Ticket #%d\n"+
				"Customer: %s\n"+
				"Priority: %s\n"+
				"GitHub: %s",
			ticket.ID,
			ticket.Customer,
			ticket.Priority,
			issueURL,
		)

		_, err := manager.CallTool(ctx, "slack", "mcp_slack_post_message", map[string]interface{}{
			"channel": "#support",
			"text":    slackMsg,
		})

		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("slack notification failed: %w", err))
			mu.Unlock()
		} else {
			fmt.Println("  ✅ Slack notified")
		}
	}()

	// 2b. Send confirmation email
	wg.Add(1)
	go func() {
		defer wg.Done()

		emailBody := fmt.Sprintf(
			"Dear %s,\n\n"+
				"We have received your support ticket and created issue #%d.\n"+
				"Our team will respond within 24 hours.\n\n"+
				"Track your issue: %s\n\n"+
				"Best regards,\n"+
				"Support Team",
			ticket.Customer,
			issueNumber,
			issueURL,
		)

		_, err := manager.CallTool(ctx, "gmail", "mcp_gmail_send_message", map[string]interface{}{
			"to":      ticket.Customer,
			"subject": fmt.Sprintf("Support Ticket #%d Received", ticket.ID),
			"body":    emailBody,
		})

		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("email failed: %w", err))
			mu.Unlock()
		} else {
			fmt.Println("  ✅ Email sent")
		}
	}()

	// 2c. Log to Notion
	wg.Add(1)
	go func() {
		defer wg.Done()

		_, err := manager.CallTool(ctx, "notion", "mcp_notion_create_page", map[string]interface{}{
			"database_id": "support-tickets-db",
			"properties": map[string]interface{}{
				"Ticket ID":     ticket.ID,
				"Customer":      ticket.Customer,
				"Priority":      ticket.Priority,
				"GitHub Issue":  issueNumber,
				"Status":        "Open",
				"Created":       time.Now().Format("2006-01-02"),
			},
		})

		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("notion logging failed: %w", err))
			mu.Unlock()
		} else {
			fmt.Println("  ✅ Notion logged")
		}
	}()

	wg.Wait()

	if len(errors) > 0 {
		fmt.Printf("\n⚠️ Completed with %d errors:\n", len(errors))
		for _, err := range errors {
			fmt.Printf("  - %v\n", err)
		}
	} else {
		fmt.Println("\n✅ All notifications sent successfully")
	}

	// Step 3: Update internal CRM (sequential, depends on previous steps)
	fmt.Println("\nStep 3: Updating CRM...")

	// (If using Salesforce MCP server)
	_, err = manager.CallTool(ctx, "salesforce", "mcp_salesforce_create_case", map[string]interface{}{
		"subject":     ticket.Title,
		"description": ticket.Body,
		"priority":    ticket.Priority,
		"origin":      "Web",
		"status":      "New",
	})

	if err != nil {
		fmt.Printf("⚠️ CRM update failed: %v\n", err)
	} else {
		fmt.Println("✅ CRM updated")
	}

	fmt.Printf("\n✅ Ticket #%d fully processed!\n", ticket.ID)
	return nil
}

func main() {
	ctx := context.Background()

	manager := client.NewMCPClientManager()
	defer manager.Close()

	// Connect all required servers
	connectServers(ctx, manager)

	// Process a support ticket
	ticket := &SupportTicket{
		ID:       12345,
		Title:    "Cannot access dashboard",
		Body:     "When I try to access the dashboard, I get a 404 error.",
		Customer: "customer@example.com",
		Priority: "high",
	}

	if err := handleSupportTicket(ctx, manager, ticket); err != nil {
		log.Fatalf("Failed to handle ticket: %v", err)
	}
}
```

## 📖 Part 5: Error Handling in Multi-Server Workflows

### Strategy 1: Fail Fast

```go
// If any step fails, stop immediately
result1, err := manager.CallTool(ctx, "github", "tool1", params1)
if err != nil {
	return fmt.Errorf("step 1 failed: %w", err)
}

result2, err := manager.CallTool(ctx, "slack", "tool2", params2)
if err != nil {
	return fmt.Errorf("step 2 failed: %w", err)
}
```

### Strategy 2: Collect All Errors

```go
// Try all operations, collect errors
errors := make([]error, 0)

_, err1 := manager.CallTool(ctx, "github", "tool1", params1)
if err1 != nil {
	errors = append(errors, err1)
}

_, err2 := manager.CallTool(ctx, "slack", "tool2", params2)
if err2 != nil {
	errors = append(errors, err2)
}

if len(errors) > 0 {
	return fmt.Errorf("workflow had %d errors", len(errors))
}
```

### Strategy 3: Retry with Fallback

```go
func callWithRetry(ctx context.Context, manager *client.MCPClientManager, serverName, toolName string, params map[string]interface{}, maxRetries int) (interface{}, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		result, err := manager.CallTool(ctx, serverName, toolName, params)
		if err == nil {
			return result, nil
		}

		if attempt < maxRetries {
			fmt.Printf("⚠️ Attempt %d failed, retrying...\n", attempt)
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return nil, fmt.Errorf("failed after %d attempts", maxRetries)
}

// Use with fallback
result, err := callWithRetry(ctx, manager, "github", "create_issue", params, 3)
if err != nil {
	// Fallback: Log to local file instead
	logToFile(params)
}
```

## 🏋️ Practice Exercises

### Exercise 1: GitHub to Slack Pipeline

Create a workflow that:
1. Gets recent GitHub pull requests
2. Posts a summary to Slack

<details>
<summary>Click to see solution</summary>

```go
// Step 1: Get recent PRs
prsResult, _ := manager.CallTool(ctx, "github", "mcp_github_list_prs", map[string]interface{}{
	"owner": "company",
	"repo":  "product",
	"state": "open",
})

prs := prsResult["prs"].([]interface{})

// Step 2: Build summary
summary := fmt.Sprintf("📋 Open Pull Requests (%d):\n", len(prs))
for i, pr := range prs {
	prMap := pr.(map[string]interface{})
	summary += fmt.Sprintf("%d. %s by %s\n", i+1, prMap["title"], prMap["author"])
}

// Step 3: Post to Slack
manager.CallTool(ctx, "slack", "mcp_slack_post_message", map[string]interface{}{
	"channel": "#dev",
	"text":    summary,
})
```
</details>

### Exercise 2: Parallel Notifications

Send the same message to Slack, Email, and Notion simultaneously.

<details>
<summary>Click to see solution</summary>

```go
message := "🎉 Q4 Goals Achieved!"

var wg sync.WaitGroup
wg.Add(3)

go func() {
	defer wg.Done()
	manager.CallTool(ctx, "slack", "mcp_slack_post_message", map[string]interface{}{
		"channel": "#all-hands",
		"text":    message,
	})
}()

go func() {
	defer wg.Done()
	manager.CallTool(ctx, "gmail", "mcp_gmail_send_message", map[string]interface{}{
		"to":      "team@company.com",
		"subject": "Q4 Update",
		"body":    message,
	})
}()

go func() {
	defer wg.Done()
	manager.CallTool(ctx, "notion", "mcp_notion_create_page", map[string]interface{}{
		"database_id": "updates-db",
		"properties": map[string]interface{}{
			"Title":   "Q4 Goals Achieved",
			"Content": message,
		},
	})
}()

wg.Wait()
```
</details>

### Exercise 3: Build a Release Workflow

When a new version is released:
1. Create GitHub release
2. Post to Slack (parallel)
3. Send email newsletter (parallel)
4. Update documentation in Notion (parallel)

<details>
<summary>Click to see solution</summary>

```go
version := "v2.0.0"

// Step 1: Create GitHub release
releaseResult, _ := manager.CallTool(ctx, "github", "mcp_github_create_release", map[string]interface{}{
	"owner": "company",
	"repo":  "product",
	"tag":   version,
	"name":  fmt.Sprintf("Release %s", version),
	"body":  "New features and bug fixes",
})

releaseURL := releaseResult["html_url"].(string)

// Step 2-4: Parallel notifications
var wg sync.WaitGroup
wg.Add(3)

go func() {
	defer wg.Done()
	manager.CallTool(ctx, "slack", "mcp_slack_post_message", map[string]interface{}{
		"channel": "#announcements",
		"text":    fmt.Sprintf("🚀 %s released! %s", version, releaseURL),
	})
}()

go func() {
	defer wg.Done()
	manager.CallTool(ctx, "gmail", "mcp_gmail_send_message", map[string]interface{}{
		"to":      "users@company.com",
		"subject": fmt.Sprintf("Version %s Released", version),
		"body":    fmt.Sprintf("Check out the new release: %s", releaseURL),
	})
}()

go func() {
	defer wg.Done()
	manager.CallTool(ctx, "notion", "mcp_notion_create_page", map[string]interface{}{
		"database_id": "releases-db",
		"properties": map[string]interface{}{
			"Version": version,
			"URL":     releaseURL,
			"Date":    time.Now().Format("2006-01-02"),
		},
	})
}()

wg.Wait()
```
</details>

## 📝 Summary

Congratulations! You've learned:

✅ How to connect multiple MCP servers
✅ Sequential workflow patterns (A → B → C)
✅ Parallel execution for performance
✅ Complex orchestration combining both
✅ Error handling strategies
✅ Real-world multi-server automation

### Key Patterns

| Pattern | Use Case | Example |
|---------|----------|---------|
| **Sequential** | Steps depend on each other | Create issue → Post to Slack |
| **Parallel** | Independent operations | Notify all channels at once |
| **Hybrid** | Complex workflows | Support ticket handler |

### Performance Tips

1. **Use parallel execution** when operations are independent
2. **Add timeouts** to prevent hanging
3. **Implement retries** for flaky services
4. **Pool connections** for frequently used servers
5. **Cache tool lists** to reduce discovery overhead

## 🎯 Next Steps

**[Tutorial 6: Production Deployment →](06-production-deployment.md)**

Learn how to deploy your multi-server agents to production!

---

**Great job! 🎉 Continue to [Tutorial 6](06-production-deployment.md) when ready.**
