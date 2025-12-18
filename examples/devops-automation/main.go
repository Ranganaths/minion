package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Ranganaths/minion/core"
	"github.com/Ranganaths/minion/models"
	"github.com/Ranganaths/minion/storage"
	"github.com/Ranganaths/minion/tools/domains"
)

/*
DevOps Automation Agent Example

This example demonstrates a complete DevOps workflow automation:
1. Monitor GitHub repository for new pull requests
2. Create Jira tickets for code review
3. Run automated checks
4. Send Slack notifications to the team
5. Auto-update Jira when PR is merged

Real-world use case:
- Reduces manual ticket creation
- Improves team communication
- Tracks code review process
- Automates status updates
*/

func main() {
	log.Println("🚀 Starting DevOps Automation Agent...")

	// Initialize framework
	framework := core.NewFramework(
		core.WithStorage(storage.NewInMemory()),
	)

	// Register all tools
	if err := domains.RegisterAllDomainTools(framework); err != nil {
		log.Fatal("Failed to register tools:", err)
	}

	ctx := context.Background()

	// Create DevOps Agent with multiple integration capabilities
	agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:        "DevOps Automation Agent",
		Description: "Automates GitHub → Jira → Slack workflows",
		Capabilities: []string{
			"github_integration",
			"jira_integration",
			"slack_integration",
			"project_management",
			"communication",
		},
		Metadata: map[string]interface{}{
			"team":        "engineering",
			"environment": "production",
			"version":     "1.0.0",
		},
	})

	if err != nil {
		log.Fatal("Failed to create agent:", err)
	}

	log.Printf("✅ Created agent: %s (ID: %s)\n", agent.Name, agent.ID)
	log.Printf("📋 Agent has access to %d tools\n", len(framework.GetToolsForAgent(agent)))

	// Run example workflows
	runPullRequestWorkflow(ctx, framework)
	runIncidentResponseWorkflow(ctx, framework)
	runDeploymentWorkflow(ctx, framework)

	log.Println("✅ DevOps automation completed successfully!")
}

// runPullRequestWorkflow demonstrates GitHub → Jira → Slack integration
func runPullRequestWorkflow(ctx context.Context, framework core.Framework) {
	log.Println("\n" + "="*60)
	log.Println("📝 WORKFLOW 1: Pull Request Automation")
	log.Println("="*60 + "\n")

	// Step 1: Simulate new PR detection (in real scenario, this would be a webhook)
	prData := map[string]interface{}{
		"title":  "Add user authentication feature",
		"author": "john.doe",
		"branch": "feature/user-auth",
		"files_changed": 15,
		"additions":     450,
		"deletions":     120,
	}

	log.Printf("🔍 Detected new PR: %s\n", prData["title"])

	// Step 2: Create Jira ticket for code review
	log.Println("\n📋 Creating Jira ticket for code review...")

	jiraOutput, err := framework.ExecuteTool(ctx, "jira_manage_issue", &models.ToolInput{
		Params: map[string]interface{}{
			"action":      "create",
			"project_key": "ENG",
			"issue_type":  "Code Review",
			"summary":     fmt.Sprintf("Code Review: %s", prData["title"]),
			"description": fmt.Sprintf(`
Code Review Request

PR Details:
- Author: %s
- Branch: %s
- Files Changed: %d
- Additions: %d
- Deletions: %d

Please review and approve before merging.
			`, prData["author"], prData["branch"], prData["files_changed"],
				prData["additions"], prData["deletions"]),
			"priority": "High",
		},
	})

	if err != nil || !jiraOutput.Success {
		log.Printf("❌ Failed to create Jira ticket: %v\n", err)
		return
	}

	jiraIssue := jiraOutput.Result.(map[string]interface{})
	issueKey := jiraIssue["key"].(string)
	log.Printf("✅ Created Jira ticket: %s\n", issueKey)

	// Step 3: Add PR to current sprint
	log.Println("\n🏃 Adding to current sprint...")

	sprintOutput, _ := framework.ExecuteTool(ctx, "jira_manage_sprint", &models.ToolInput{
		Params: map[string]interface{}{
			"action":    "add_issues",
			"sprint_id": "123",
			"issues":    []string{issueKey},
		},
	})

	if sprintOutput.Success {
		log.Println("✅ Added to current sprint")
	}

	// Step 4: Send Slack notification to team
	log.Println("\n💬 Sending Slack notification...")

	slackOutput, err := framework.ExecuteTool(ctx, "slack_send_message", &models.ToolInput{
		Params: map[string]interface{}{
			"channel": "#code-review",
			"message": fmt.Sprintf("🔔 New code review required!"),
			"attachments": []map[string]interface{}{
				{
					"color":  "#36a64f",
					"title":  prData["title"].(string),
					"fields": []map[string]interface{}{
						{
							"title": "Author",
							"value": prData["author"],
							"short": true,
						},
						{
							"title": "Jira Ticket",
							"value": issueKey,
							"short": true,
						},
						{
							"title": "Files Changed",
							"value": fmt.Sprintf("%d", prData["files_changed"]),
							"short": true,
						},
						{
							"title": "Lines",
							"value": fmt.Sprintf("+%d -%d", prData["additions"], prData["deletions"]),
							"short": true,
						},
					},
					"actions": []map[string]interface{}{
						{
							"type": "button",
							"text": "Review PR",
							"url":  "https://github.com/company/repo/pull/123",
						},
						{
							"type": "button",
							"text": "View Jira",
							"url":  fmt.Sprintf("https://company.atlassian.net/browse/%s", issueKey),
						},
					},
				},
			},
		},
	})

	if err != nil || !slackOutput.Success {
		log.Printf("❌ Failed to send Slack notification: %v\n", err)
		return
	}

	log.Printf("✅ Sent Slack notification to #code-review\n")

	// Step 5: Simulate code review process
	log.Println("\n⏳ Simulating code review process (3 seconds)...")
	time.Sleep(3 * time.Second)

	// Step 6: Update Jira when PR is approved
	log.Println("\n✅ Code review approved! Updating Jira...")

	_, _ = framework.ExecuteTool(ctx, "jira_manage_issue", &models.ToolInput{
		Params: map[string]interface{}{
			"action":    "update",
			"issue_key": issueKey,
			"fields": map[string]interface{}{
				"status": "Approved",
			},
		},
	})

	// Step 7: Notify on Slack about approval
	framework.ExecuteTool(ctx, "slack_send_message", &models.ToolInput{
		Params: map[string]interface{}{
			"channel": "#code-review",
			"message": fmt.Sprintf("✅ Code review %s has been approved and ready to merge!", issueKey),
		},
	})

	log.Println("\n✅ Pull request workflow completed!")
}

// runIncidentResponseWorkflow demonstrates incident management automation
func runIncidentResponseWorkflow(ctx context.Context, framework core.Framework) {
	log.Println("\n" + "="*60)
	log.Println("🚨 WORKFLOW 2: Incident Response Automation")
	log.Println("="*60 + "\n")

	// Simulate incident detection
	incident := map[string]interface{}{
		"title":    "Production API Error Rate Spike",
		"severity": "Critical",
		"service":  "payment-api",
		"error_rate": 15.5,
		"affected_users": 1250,
	}

	log.Printf("🚨 Incident detected: %s\n", incident["title"])

	// Create high-priority Jira incident
	log.Println("\n📋 Creating Jira incident ticket...")

	jiraOutput, _ := framework.ExecuteTool(ctx, "jira_manage_issue", &models.ToolInput{
		Params: map[string]interface{}{
			"action":      "create",
			"project_key": "OPS",
			"issue_type":  "Incident",
			"summary":     incident["title"],
			"description": fmt.Sprintf(`
**INCIDENT ALERT**

Service: %s
Severity: %s
Error Rate: %.1f%%
Affected Users: %d

Immediate action required!
			`, incident["service"], incident["severity"],
				incident["error_rate"], incident["affected_users"]),
			"priority": "Critical",
			"labels":   []string{"incident", "production", "api"},
		},
	})

	incidentKey := jiraOutput.Result.(map[string]interface{})["key"].(string)
	log.Printf("✅ Created incident ticket: %s\n", incidentKey)

	// Send urgent Slack alert
	log.Println("\n🚨 Sending urgent Slack alert to on-call team...")

	framework.ExecuteTool(ctx, "slack_send_message", &models.ToolInput{
		Params: map[string]interface{}{
			"channel": "#incidents",
			"message": "@channel 🚨 CRITICAL INCIDENT",
			"attachments": []map[string]interface{}{
				{
					"color": "#ff0000",
					"title": incident["title"].(string),
					"text":  fmt.Sprintf("Service: %s | Error Rate: %.1f%% | Affected Users: %d",
						incident["service"], incident["error_rate"], incident["affected_users"]),
					"fields": []map[string]interface{}{
						{
							"title": "Severity",
							"value": incident["severity"],
							"short": true,
						},
						{
							"title": "Jira Ticket",
							"value": incidentKey,
							"short": true,
						},
					},
					"footer": "Incident Response System",
					"ts":     time.Now().Unix(),
				},
			},
		},
	})

	log.Println("✅ Incident response workflow initiated!")

	// Send SMS to on-call engineer via Twilio
	log.Println("\n📱 Sending SMS to on-call engineer...")

	framework.ExecuteTool(ctx, "twilio_send_sms", &models.ToolInput{
		Params: map[string]interface{}{
			"to":   "+1234567890",
			"message": fmt.Sprintf("🚨 CRITICAL: %s - Check Slack #incidents and Jira %s immediately!",
				incident["title"], incidentKey),
		},
	})

	log.Println("✅ SMS sent to on-call engineer")
}

// runDeploymentWorkflow demonstrates deployment automation
func runDeploymentWorkflow(ctx context.Context, framework core.Framework) {
	log.Println("\n" + "="*60)
	log.Println("🚀 WORKFLOW 3: Deployment Automation")
	log.Println("="*60 + "\n")

	deployment := map[string]interface{}{
		"version":     "v2.5.0",
		"environment": "production",
		"service":     "api-gateway",
		"commit":      "abc123def456",
	}

	log.Printf("🚀 Starting deployment: %s to %s\n", deployment["version"], deployment["environment"])

	// Create deployment Jira task
	log.Println("\n📋 Creating deployment task in Jira...")

	jiraOutput, _ := framework.ExecuteTool(ctx, "jira_manage_issue", &models.ToolInput{
		Params: map[string]interface{}{
			"action":      "create",
			"project_key": "OPS",
			"issue_type":  "Deployment",
			"summary":     fmt.Sprintf("Deploy %s %s to %s",
				deployment["service"], deployment["version"], deployment["environment"]),
			"description": fmt.Sprintf(`
Deployment Details:
- Version: %s
- Environment: %s
- Service: %s
- Commit: %s

Deployment checklist:
- [ ] Pre-deployment health check
- [ ] Database migrations
- [ ] Deploy new version
- [ ] Health check verification
- [ ] Monitoring verification
			`, deployment["version"], deployment["environment"],
				deployment["service"], deployment["commit"]),
		},
	})

	deploymentKey := jiraOutput.Result.(map[string]interface{})["key"].(string)
	log.Printf("✅ Created deployment task: %s\n", deploymentKey)

	// Announce deployment start
	log.Println("\n💬 Announcing deployment start on Slack...")

	framework.ExecuteTool(ctx, "slack_send_message", &models.ToolInput{
		Params: map[string]interface{}{
			"channel": "#deployments",
			"message": "🚀 Deployment Started",
			"attachments": []map[string]interface{}{
				{
					"color": "#ffa500",
					"title": fmt.Sprintf("Deploying %s %s", deployment["service"], deployment["version"]),
					"fields": []map[string]interface{}{
						{
							"title": "Environment",
							"value": deployment["environment"],
							"short": true,
						},
						{
							"title": "Version",
							"value": deployment["version"],
							"short": true,
						},
						{
							"title": "Commit",
							"value": deployment["commit"],
							"short": true,
						},
						{
							"title": "Jira",
							"value": deploymentKey,
							"short": true,
						},
					},
					"footer": "Deployment System",
					"ts":     time.Now().Unix(),
				},
			},
		},
	})

	// Simulate deployment process
	log.Println("\n⏳ Running deployment steps...")
	steps := []string{
		"Pre-deployment health check",
		"Running database migrations",
		"Deploying new version",
		"Running health checks",
		"Verifying monitoring",
	}

	for i, step := range steps {
		log.Printf("  [%d/%d] %s...\n", i+1, len(steps), step)
		time.Sleep(1 * time.Second)
	}

	// Update Jira on successful deployment
	log.Println("\n✅ Deployment successful! Updating Jira...")

	framework.ExecuteTool(ctx, "jira_manage_issue", &models.ToolInput{
		Params: map[string]interface{}{
			"action":    "update",
			"issue_key": deploymentKey,
			"fields": map[string]interface{}{
				"status": "Done",
			},
		},
	})

	// Announce successful deployment
	framework.ExecuteTool(ctx, "slack_send_message", &models.ToolInput{
		Params: map[string]interface{}{
			"channel": "#deployments",
			"message": fmt.Sprintf("✅ Deployment %s completed successfully! Service %s is now running version %s in %s",
				deploymentKey, deployment["service"], deployment["version"], deployment["environment"]),
		},
	})

	log.Println("✅ Deployment workflow completed!")
}
