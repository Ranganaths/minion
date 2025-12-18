# Composio-Style Integration Tools

This document provides a comprehensive overview of all third-party integration tools added to the Minion framework, similar to Composio's extensive platform integrations.

## Overview

**Total New Integration Tools: 80+**
**Total Domains: 10**

---

## 1. Communication & Collaboration Tools (9 tools) ✅ IMPLEMENTED

### Slack Integration
- **slack_send_message** - Send messages with rich formatting, attachments, mentions
- **slack_manage_channel** - Create, archive, invite users to channels

### Microsoft Teams
- **teams_send_message** - Send messages with adaptive cards

### Discord
- **discord_send_message** - Send messages with embeds and reactions

### Gmail
- **gmail_send_email** - Send emails with attachments and HTML
- **gmail_search** - Search emails with advanced filters

### Zoom
- **zoom_manage_meeting** - Create, update, delete Zoom meetings

### Twilio
- **twilio_send_sms** - Send SMS messages
- **twilio_make_call** - Make phone calls with TwiML

**Files Created:**
- `/tools/domains/communication/tools.go` ✅

---

## 2. Project Management Tools (9 tools) ✅ IMPLEMENTED

### Jira
- **jira_manage_issue** - Create, update, search, transition issues
- **jira_manage_sprint** - Create, start, complete sprints

### Asana
- **asana_manage_task** - Create, update, complete tasks
- **asana_manage_project** - Create and list projects

### Trello
- **trello_manage_card** - Create, update, move cards, add checklists
- **trello_manage_board** - Create and list boards

### Linear
- **linear_manage_issue** - Create, update, search issues

### ClickUp
- **clickup_manage_task** - Create and update tasks

### Monday.com
- **monday_manage_item** - Create, update, query items

**Files Created:**
- `/tools/domains/projectmgmt/tools.go` ✅

---

## 3. CRM & Sales Tools (8 tools)

### Salesforce
- **salesforce_manage_lead** - Create, update, convert leads
- **salesforce_manage_opportunity** - Create, update opportunities
- **salesforce_manage_account** - Manage customer accounts
- **salesforce_query** - Execute SOQL queries

### HubSpot
- **hubspot_manage_contact** - Create, update, search contacts
- **hubspot_manage_deal** - Create, update, track deals
- **hubspot_create_note** - Add notes to CRM records

### Pipedrive
- **pipedrive_manage_deal** - Create, update deals in pipeline

**Files to Create:**
- `/tools/domains/crm/tools.go`

---

## 4. Development & DevOps Tools (12 tools)

### GitHub
- **github_manage_issue** - Create, update, close issues
- **github_manage_pr** - Create, merge, review pull requests
- **github_manage_repo** - Create repositories, manage settings
- **github_run_workflow** - Trigger GitHub Actions workflows

### GitLab
- **gitlab_manage_issue** - Create, update GitLab issues
- **gitlab_manage_mr** - Create, merge merge requests
- **gitlab_run_pipeline** - Trigger CI/CD pipelines

### Jenkins
- **jenkins_trigger_build** - Trigger Jenkins builds
- **jenkins_get_build_status** - Check build status

### Docker Hub
- **docker_push_image** - Push Docker images
- **docker_list_images** - List repository images

### CircleCI
- **circleci_trigger_pipeline** - Trigger pipelines

**Files to Create:**
- `/tools/domains/devops/tools.go`

---

## 5. Document Management Tools (10 tools)

### Google Drive
- **gdrive_upload_file** - Upload files to Google Drive
- **gdrive_create_folder** - Create folders
- **gdrive_share_file** - Share files with permissions
- **gdrive_search** - Search for files

### Dropbox
- **dropbox_upload_file** - Upload files
- **dropbox_create_folder** - Create folders
- **dropbox_share_link** - Generate share links

### Notion
- **notion_create_page** - Create Notion pages
- **notion_update_page** - Update page content
- **notion_query_database** - Query Notion databases

### Confluence
- **confluence_create_page** - Create Confluence pages

**Files to Create:**
- `/tools/domains/documents/tools.go`

---

## 6. Social Media Tools (10 tools)

### Twitter/X
- **twitter_post_tweet** - Post tweets
- **twitter_search_tweets** - Search tweets
- **twitter_get_mentions** - Get mentions

### LinkedIn
- **linkedin_create_post** - Create LinkedIn posts
- **linkedin_get_profile** - Get profile information

### Facebook
- **facebook_create_post** - Create Facebook posts
- **facebook_get_page_insights** - Get page analytics

### Instagram
- **instagram_create_post** - Post to Instagram
- **instagram_get_media** - Get media information

### YouTube
- **youtube_upload_video** - Upload videos
- **youtube_get_analytics** - Get channel analytics

**Files to Create:**
- `/tools/domains/social/tools.go`

---

## 7. Calendar & Scheduling Tools (6 tools)

### Google Calendar
- **gcalendar_create_event** - Create calendar events
- **gcalendar_update_event** - Update events
- **gcalendar_list_events** - List upcoming events

### Outlook Calendar
- **outlook_create_event** - Create Outlook events
- **outlook_list_events** - List calendar events

### Calendly
- **calendly_get_events** - Get scheduled events

**Files to Create:**
- `/tools/domains/calendar/tools.go`

---

## 8. E-commerce & Payment Tools (10 tools)

### Shopify
- **shopify_create_product** - Create products
- **shopify_manage_order** - Manage orders
- **shopify_get_inventory** - Check inventory

### Stripe
- **stripe_create_payment** - Create payment intents
- **stripe_create_customer** - Create customers
- **stripe_create_subscription** - Create subscriptions
- **stripe_get_transactions** - Get transaction history

### PayPal
- **paypal_create_invoice** - Create invoices
- **paypal_capture_payment** - Capture payments

### WooCommerce
- **woocommerce_manage_product** - Manage products

**Files to Create:**
- `/tools/domains/ecommerce/tools.go`

---

## 9. Analytics & Monitoring Tools (8 tools)

### Google Analytics
- **ga_get_metrics** - Get analytics metrics
- **ga_create_report** - Create custom reports

### Mixpanel
- **mixpanel_track_event** - Track events
- **mixpanel_get_insights** - Get user insights

### Datadog
- **datadog_send_metric** - Send custom metrics
- **datadog_create_monitor** - Create monitors
- **datadog_get_alerts** - Get active alerts

### New Relic
- **newrelic_get_metrics** - Get application metrics

### Sentry
- **sentry_log_error** - Log error events

**Files to Create:**
- `/tools/domains/monitoring/tools.go`

---

## 10. HR & Productivity Tools (8 tools)

### BambooHR
- **bamboohr_get_employee** - Get employee information
- **bamboohr_create_employee** - Add new employees

### Airtable
- **airtable_create_record** - Create records
- **airtable_query_records** - Query tables
- **airtable_update_record** - Update records

### Google Sheets
- **gsheets_read_range** - Read cell ranges
- **gsheets_write_range** - Write to cells
- **gsheets_create_sheet** - Create new sheets

### Typeform
- **typeform_get_responses** - Get form responses

**Files to Create:**
- `/tools/domains/productivity/tools.go`

---

## Implementation Status

| Domain | Tools Count | Status |
|--------|-------------|--------|
| Communication & Collaboration | 9 | ✅ Implemented |
| Project Management | 9 | ✅ Implemented |
| CRM & Sales | 8 | 📝 Documented |
| Development & DevOps | 12 | 📝 Documented |
| Document Management | 10 | 📝 Documented |
| Social Media | 10 | 📝 Documented |
| Calendar & Scheduling | 6 | 📝 Documented |
| E-commerce & Payment | 10 | 📝 Documented |
| Analytics & Monitoring | 8 | 📝 Documented |
| HR & Productivity | 8 | 📝 Documented |

**Total: 90 integration tools**

---

## Quick Start Examples

### Example 1: Slack + Jira Integration

```go
// Create agent with Slack and Jira capabilities
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name: "DevOps Assistant",
    Capabilities: []string{
        "slack_integration",
        "jira_integration",
        "communication",
        "project_management",
    },
})

// Create a Jira issue
jiraOutput, _ := framework.ExecuteTool(ctx, "jira_manage_issue", &models.ToolInput{
    Params: map[string]interface{}{
        "action": "create",
        "project_key": "PROJ",
        "issue_type": "Bug",
        "summary": "Critical bug in payment flow",
        "description": "Users unable to complete checkout",
    },
})

issueKey := jiraOutput.Result.(map[string]interface{})["key"]

// Send Slack notification
framework.ExecuteTool(ctx, "slack_send_message", &models.ToolInput{
    Params: map[string]interface{}{
        "channel": "#engineering",
        "message": fmt.Sprintf("🚨 Critical bug created: %s", issueKey),
    },
})
```

### Example 2: Gmail + Google Calendar Integration

```go
// Search for meeting invite emails
emailResults, _ := framework.ExecuteTool(ctx, "gmail_search", &models.ToolInput{
    Params: map[string]interface{}{
        "query": "subject:meeting after:2024/01/01",
        "max_results": 10,
    },
})

// Create calendar event
framework.ExecuteTool(ctx, "gcalendar_create_event", &models.ToolInput{
    Params: map[string]interface{}{
        "summary": "Team Sync",
        "start_time": "2024-02-01T14:00:00Z",
        "end_time": "2024-02-01T15:00:00Z",
        "attendees": []string{"team@company.com"},
    },
})
```

### Example 3: GitHub + Slack Notifications

```go
// Create GitHub pull request
prOutput, _ := framework.ExecuteTool(ctx, "github_manage_pr", &models.ToolInput{
    Params: map[string]interface{}{
        "action": "create",
        "repo": "company/product",
        "title": "Add new feature",
        "head": "feature-branch",
        "base": "main",
    },
})

prNumber := prOutput.Result.(map[string]interface{})["number"]

// Notify team in Slack
framework.ExecuteTool(ctx, "slack_send_message", &models.ToolInput{
    Params: map[string]interface{}{
        "channel": "#code-review",
        "message": fmt.Sprintf("New PR ready for review: #%v", prNumber),
        "attachments": []map[string]interface{}{
            {
                "color": "#36a64f",
                "title": "View Pull Request",
                "title_link": fmt.Sprintf("https://github.com/company/product/pull/%v", prNumber),
            },
        },
    },
})
```

### Example 4: Salesforce + HubSpot Sync

```go
// Get leads from Salesforce
sfLeads, _ := framework.ExecuteTool(ctx, "salesforce_query", &models.ToolInput{
    Params: map[string]interface{}{
        "query": "SELECT Id, Email, FirstName, LastName FROM Lead WHERE CreatedDate = TODAY",
    },
})

// Sync to HubSpot
for _, lead := range sfLeads.Result.([]interface{}) {
    framework.ExecuteTool(ctx, "hubspot_manage_contact", &models.ToolInput{
        Params: map[string]interface{}{
            "action": "create",
            "email": lead["Email"],
            "firstname": lead["FirstName"],
            "lastname": lead["LastName"],
            "source": "Salesforce",
        },
    })
}
```

---

## Authentication Patterns

All tools support multiple authentication methods:

### 1. OAuth 2.0
```go
params := map[string]interface{}{
    "access_token": "oauth_token_here",
    // tool-specific params
}
```

### 2. API Keys
```go
params := map[string]interface{}{
    "api_key": "your_api_key",
    // tool-specific params
}
```

### 3. Basic Auth
```go
params := map[string]interface{}{
    "username": "user",
    "password": "pass",
    // tool-specific params
}
```

---

## Error Handling

All integration tools follow consistent error handling:

```go
output, err := framework.ExecuteTool(ctx, "tool_name", input)
if err != nil {
    // Framework-level error
    log.Printf("Execution failed: %v", err)
    return
}

if !output.Success {
    // Tool-level error
    log.Printf("Tool error: %v", output.Error)
    return
}

// Success
result := output.Result
```

---

## Rate Limiting & Retry Logic

Integration tools include built-in rate limiting awareness:

```go
// Tools automatically handle rate limits
output, _ := framework.ExecuteTool(ctx, "github_manage_issue", input)

// Check for rate limit in metadata
if rateLimitRemaining, ok := output.Metadata["rate_limit_remaining"].(int); ok {
    log.Printf("API calls remaining: %d", rateLimitRemaining)
}
```

---

## Webhook Support

Many integrations support webhooks for real-time events:

```go
// Set up webhook handler
framework.ExecuteTool(ctx, "webhook_handler", &models.ToolInput{
    Params: map[string]interface{}{
        "payload": webhookPayload,
        "signature": webhookSignature,
        "secret": "webhook_secret",
    },
})
```

---

## Best Practices

### 1. Use Capability-Based Access
```go
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Capabilities: []string{
        "slack_integration",
        "jira_integration",
        // Only grant necessary capabilities
    },
})
```

### 2. Handle Pagination
```go
page := 1
for {
    results, _ := framework.ExecuteTool(ctx, "github_manage_issue", &models.ToolInput{
        Params: map[string]interface{}{
            "action": "search",
            "page": page,
        },
    })

    if len(results.Result.([]interface{})) == 0 {
        break
    }
    page++
}
```

### 3. Batch Operations
```go
// Batch create multiple items
for _, item := range items {
    go framework.ExecuteTool(ctx, "tool_name", item)
}
```

### 4. Secure Credentials
```go
// Use environment variables or secret managers
apiKey := os.Getenv("SLACK_API_KEY")
```

---

## Contributing

To add new integration tools:

1. Create tool file in appropriate domain
2. Implement the Tool interface
3. Add to domain registration
4. Add examples to this documentation
5. Test with actual API (or mocks)

---

## License

Minion Framework - Production-Ready AI Agent Framework
Copyright (c) 2024
