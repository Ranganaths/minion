# Tutorial 2: MCP Integration Basics

**Duration**: 45 minutes
**Level**: Beginner
**Prerequisites**: Tutorial 1 completed

## 🎯 Learning Objectives

By the end of this tutorial, you will:
- Understand what Model Context Protocol (MCP) is
- Connect to external MCP servers
- Discover tools from external services
- Use external tools in your agents
- Understand MCP tool naming conventions

## 📚 What is MCP?

**Model Context Protocol (MCP)** is an open standard that enables AI applications to connect to external data sources and tools.

### Real-World Analogy

Think of MCP as **USB for AI agents**:

- **USB** lets you plug any device into your computer
- **MCP** lets you plug any service into your AI agent

```
Without MCP:
┌──────┐     ┌─────────┐
│Agent │ ──> │Custom   │
└──────┘     │Code for │
             │GitHub   │
             └─────────┘

             ┌─────────┐
         ──> │Custom   │
             │Code for │
             │Slack    │
             └─────────┘

With MCP:
┌──────┐     ┌─────────┐
│Agent │ ──> │   MCP   │ ──> 📁 GitHub
└──────┘     │Protocol │ ──> 💬 Slack
             └─────────┘ ──> 📧 Gmail
                         ──> 📊 Salesforce
                         ──> 1000+ services!
```

### Why Use MCP?

✅ **Standardized**: One protocol for all services
✅ **Extensible**: 1000+ tools available
✅ **No Custom Code**: Plug and play integration
✅ **Community**: Growing ecosystem of servers
✅ **Maintained**: By the Linux Foundation

## 🏗️ MCP Architecture

```
┌─────────────────────────────────────────────────┐
│           Minion Framework (Your App)            │
│                                                  │
│  ┌────────────────────────────────────────┐    │
│  │        MCP Client Manager              │    │
│  │  ┌──────────┐  ┌──────────┐           │    │
│  │  │GitHub    │  │  Slack   │  ...      │    │
│  │  │Client    │  │  Client  │           │    │
│  │  └────┬─────┘  └────┬─────┘           │    │
│  └───────┼─────────────┼──────────────────┘    │
└──────────┼─────────────┼───────────────────────┘
           │             │
           ▼             ▼
    ┌──────────┐  ┌──────────┐
    │ GitHub   │  │  Slack   │
    │MCP Server│  │MCP Server│
    └──────────┘  └──────────┘
         │             │
         ▼             ▼
    [GitHub API]  [Slack API]
```

## 🛠️ Transport Types

MCP supports two transport types:

### 1. Stdio Transport (Local)

Used for local MCP servers (npx-based):

```go
config := &client.ClientConfig{
	ServerName: "github",
	Transport:  client.TransportStdio,
	Command:    "npx",
	Args:       []string{"-y", "@modelcontextprotocol/server-github"},
}
```

**Use When**:
- Running MCP servers locally
- Development and testing
- Using official npm packages

### 2. HTTP Transport (Remote)

Used for remote MCP servers:

```go
config := &client.ClientConfig{
	ServerName: "api",
	Transport:  client.TransportHTTP,
	URL:        "https://api.example.com/mcp",
	AuthType:   client.AuthBearer,
	AuthToken:  "your-token",
}
```

**Use When**:
- Connecting to remote services
- Production deployments
- Custom enterprise APIs

## 🚀 Hands-On: Connect to GitHub

Let's connect to GitHub MCP server and use it!

### Prerequisites

1. **Node.js installed** (for npx):
```bash
node --version  # Should be v18+
```

2. **GitHub Personal Access Token**:
- Go to https://github.com/settings/tokens
- Generate new token (classic)
- Select scopes: `repo`, `read:org`
- Copy the token

### Step 1: Setup Environment

Create `.env` file:

```bash
GITHUB_TOKEN=your_github_personal_access_token_here
```

### Step 2: Import MCP Packages

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Ranganaths/minion/core"
	"github.com/Ranganaths/minion/models"
	"github.com/Ranganaths/minion/mcp/client"
)

func main() {
	// We'll add code here
}
```

### Step 3: Initialize Framework

```go
func main() {
	ctx := context.Background()

	// Initialize framework
	framework := core.NewFramework()
	defer framework.Close()

	fmt.Println("✅ Framework initialized")
}
```

### Step 4: Connect to GitHub MCP Server

```go
// After framework initialization

// Load GitHub token from environment
githubToken := os.Getenv("GITHUB_TOKEN")
if githubToken == "" {
	log.Fatal("GITHUB_TOKEN environment variable not set")
}

// Configure GitHub MCP server
mcpConfig := &client.ClientConfig{
	ServerName: "github",
	Transport:  client.TransportStdio,
	Command:    "npx",
	Args:       []string{"-y", "@modelcontextprotocol/server-github"},
	Env: map[string]string{
		"GITHUB_PERSONAL_ACCESS_TOKEN": githubToken,
	},
}

// Connect to GitHub MCP server
fmt.Println("\n🔌 Connecting to GitHub MCP server...")
err := framework.ConnectMCPServer(ctx, mcpConfig)
if err != nil {
	log.Fatalf("Failed to connect to GitHub: %v", err)
}

fmt.Println("✅ Connected to GitHub!")
```

### Step 5: Discover GitHub Tools

```go
// List all MCP servers
servers := framework.ListMCPServers()
fmt.Printf("\n📡 Connected MCP servers: %v\n", servers)

// Create agent with MCP capabilities
agentReq := &models.CreateAgentRequest{
	Name:        "GitHubBot",
	Description: "Agent with GitHub access",
	Capabilities: []string{
		"mcp_github",  // Access to all GitHub tools
	},
}

agent, err := framework.CreateAgent(ctx, agentReq)
if err != nil {
	log.Fatalf("Failed to create agent: %v", err)
}

// Discover GitHub tools
tools := framework.GetToolsForAgent(agent)
fmt.Printf("\n🔧 Available tools for %s:\n", agent.Name)
for _, tool := range tools {
	fmt.Printf("  • %s - %s\n", tool.Name(), tool.Description())
}
```

### Step 6: Use a GitHub Tool

```go
// Example: List your repositories
fmt.Println("\n📂 Fetching your repositories...")

input := &models.ToolInput{
	ToolName: "mcp_github_list_repos",
	Params: map[string]interface{}{
		"owner": "your-github-username",
	},
}

output, err := framework.ExecuteTool(ctx, agent.ID, input)
if err != nil {
	log.Fatalf("Tool execution failed: %v", err)
}

if output.Success {
	fmt.Println("✅ Repositories:")
	// output.Result contains the list of repos
	fmt.Printf("%v\n", output.Result)
} else {
	fmt.Printf("❌ Failed: %s\n", output.Error)
}
```

### Complete Example

Here's the complete GitHub integration:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Ranganaths/minion/core"
	"github.com/Ranganaths/minion/models"
	"github.com/Ranganaths/minion/mcp/client"
)

func main() {
	// 1. Setup
	ctx := context.Background()
	framework := core.NewFramework()
	defer framework.Close()
	fmt.Println("✅ Framework initialized")

	// 2. Load credentials
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		log.Fatal("❌ GITHUB_TOKEN environment variable not set")
	}

	// 3. Connect to GitHub MCP
	fmt.Println("\n🔌 Connecting to GitHub MCP server...")
	mcpConfig := &client.ClientConfig{
		ServerName: "github",
		Transport:  client.TransportStdio,
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-github"},
		Env: map[string]string{
			"GITHUB_PERSONAL_ACCESS_TOKEN": githubToken,
		},
	}

	if err := framework.ConnectMCPServer(ctx, mcpConfig); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	fmt.Println("✅ Connected to GitHub!")

	// 4. Create agent
	agentReq := &models.CreateAgentRequest{
		Name:         "GitHubBot",
		Description:  "Agent with GitHub access",
		Capabilities: []string{"mcp_github"},
	}

	agent, err := framework.CreateAgent(ctx, agentReq)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	fmt.Printf("✅ Created agent: %s\n", agent.Name)

	// 5. List available tools
	tools := framework.GetToolsForAgent(agent)
	fmt.Printf("\n🔧 Available GitHub tools: %d\n", len(tools))
	for i, tool := range tools {
		if i < 5 {
			fmt.Printf("  • %s\n", tool.Name())
		}
	}

	// 6. Use a tool
	fmt.Println("\n📊 Getting repository info...")
	input := &models.ToolInput{
		ToolName: "mcp_github_get_repo",
		Params: map[string]interface{}{
			"owner": "your-username",
			"repo":  "your-repo",
		},
	}

	output, err := framework.ExecuteTool(ctx, agent.ID, input)
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	if output.Success {
		fmt.Println("✅ Repository info retrieved!")
		fmt.Printf("Result: %v\n", output.Result)
	} else {
		fmt.Printf("❌ Failed: %s\n", output.Error)
	}

	// 7. Disconnect
	fmt.Println("\n🔌 Disconnecting...")
	framework.DisconnectMCPServer("github")
	fmt.Println("✅ Disconnected")
}
```

### Run It!

```bash
# Set GitHub token
export GITHUB_TOKEN=your_token_here

# Run
go run main.go
```

## 🎓 MCP Tool Naming

MCP tools follow a consistent naming pattern:

```
Format: mcp_<server>_<tool>

Examples:
- mcp_github_create_issue
- mcp_slack_send_message
- mcp_gmail_send_email
- mcp_salesforce_create_lead
```

### Why This Pattern?

1. **Namespace separation**: Avoid name conflicts
2. **Clear origin**: Know which server provides the tool
3. **Easy filtering**: Group tools by server

## 🎯 MCP Capability System

You can control access at three levels:

### Level 1: Global MCP Access

```go
Capabilities: []string{"mcp_integration"}
// Agent can use ALL tools from ALL MCP servers
```

### Level 2: Server-Specific Access

```go
Capabilities: []string{
	"mcp_github",
	"mcp_slack",
}
// Agent can use all GitHub and Slack tools
```

### Level 3: Tool-Specific Access

```go
Capabilities: []string{
	"mcp_github_create_issue",
	"mcp_github_create_pr",
}
// Agent can ONLY create issues and PRs
```

## 📊 Common MCP Servers

| Server | Package | Tools | Use Case |
|--------|---------|-------|----------|
| **GitHub** | `@modelcontextprotocol/server-github` | 15+ | Code, issues, PRs |
| **Slack** | `@modelcontextprotocol/server-slack` | 10+ | Messages, channels |
| **Gmail** | `@modelcontextprotocol/server-gmail` | 8+ | Email management |
| **Google Drive** | `@modelcontextprotocol/server-gdrive` | 12+ | File operations |
| **Salesforce** | `@modelcontextprotocol/server-salesforce` | 20+ | CRM operations |
| **Notion** | `@modelcontextprotocol/server-notion` | 10+ | Notes, databases |

## 🏋️ Practice Exercises

### Exercise 1: Connect Multiple Servers

Connect to both GitHub and Slack:

```go
// 1. Connect to GitHub
// 2. Connect to Slack
// 3. Create agent with access to both
// 4. List tools from each server
```

<details>
<summary>Click to see solution</summary>

```go
// Connect GitHub
githubConfig := &client.ClientConfig{
	ServerName: "github",
	Command:    "npx",
	Args:       []string{"-y", "@modelcontextprotocol/server-github"},
	Env: map[string]string{
		"GITHUB_PERSONAL_ACCESS_TOKEN": os.Getenv("GITHUB_TOKEN"),
	},
}
framework.ConnectMCPServer(ctx, githubConfig)

// Connect Slack
slackConfig := &client.ClientConfig{
	ServerName: "slack",
	Command:    "npx",
	Args:       []string{"-y", "@modelcontextprotocol/server-slack"},
	Env: map[string]string{
		"SLACK_BOT_TOKEN": os.Getenv("SLACK_TOKEN"),
	},
}
framework.ConnectMCPServer(ctx, slackConfig)

// Create agent with both
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
	Name:         "MultiBot",
	Capabilities: []string{"mcp_github", "mcp_slack"},
})

// List tools
tools := framework.GetToolsForAgent(agent)
fmt.Printf("Total tools: %d\n", len(tools))
```
</details>

### Exercise 2: Create a GitHub Issue

Use the GitHub MCP server to create an issue:

```go
// Parameters needed:
// - owner: repository owner
// - repo: repository name
// - title: issue title
// - body: issue description
```

<details>
<summary>Click to see solution</summary>

```go
input := &models.ToolInput{
	ToolName: "mcp_github_create_issue",
	Params: map[string]interface{}{
		"owner": "your-username",
		"repo":  "your-repo",
		"title": "Test issue from Minion agent",
		"body":  "This issue was created programmatically using MCP!",
	},
}

output, err := framework.ExecuteTool(ctx, agent.ID, input)
if err != nil {
	log.Fatal(err)
}

if output.Success {
	fmt.Println("✅ Issue created!")
	fmt.Printf("URL: %v\n", output.Result)
} else {
	fmt.Printf("❌ Failed: %s\n", output.Error)
}
```
</details>

### Exercise 3: Check MCP Server Status

Monitor the health of connected MCP servers:

```go
// Get status of all MCP servers
// Display connection state
// Show tool count for each
```

<details>
<summary>Click to see solution</summary>

```go
status := framework.GetMCPServerStatus()

fmt.Println("\n📊 MCP Server Status:")
for serverName, serverStatus := range status {
	fmt.Printf("\n  Server: %s\n", serverName)
	fmt.Printf("  Connected: %v\n", serverStatus.Connected)
	fmt.Printf("  Tools: %d\n", serverStatus.ToolsDiscovered)
	fmt.Printf("  Success Rate: %.2f%%\n",
		float64(serverStatus.SuccessCalls)/float64(serverStatus.TotalCalls)*100)
}
```
</details>

## 🐛 Troubleshooting

### Issue 1: "npx command not found"

**Problem**: Node.js/npx not installed

**Solution**:
```bash
# macOS
brew install node

# Linux
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install -y nodejs
```

### Issue 2: "Authentication failed"

**Problem**: Invalid or missing API token

**Solution**:
```bash
# Verify token is set
echo $GITHUB_TOKEN

# Regenerate token if needed
# GitHub: Settings → Developer settings → Personal access tokens
```

### Issue 3: "Tool not found: mcp_github_xyz"

**Problem**: Tool name incorrect or not available

**Solution**:
```go
// List available tools first
tools := framework.GetToolsForAgent(agent)
for _, tool := range tools {
	fmt.Println(tool.Name())
}

// Use exact name from the list
```

### Issue 4: "Connection timeout"

**Problem**: MCP server taking too long to start

**Solution**:
```go
// Add timeout to context
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := framework.ConnectMCPServer(ctx, config)
```

## 📝 Summary

Congratulations! You've learned:

✅ What Model Context Protocol is and why it matters
✅ How to connect to external MCP servers
✅ How to discover tools from external services
✅ How to execute MCP tools from agents
✅ MCP naming conventions and capability system

### Key Takeaways

1. **MCP = USB for AI** - Universal connector for services
2. **Two transports** - Stdio (local) and HTTP (remote)
3. **Tool naming** - `mcp_<server>_<tool>` pattern
4. **Three capability levels** - Global, server, tool-specific
5. **Always disconnect** - Clean up MCP connections

## 🎯 Next Steps

You're now ready for:

**[Tutorial 3: Building Your First MCP Agent →](03-first-mcp-agent.md)**

Build a complete agent that combines multiple MCP servers!

### Additional Resources

- [MCP Specification](https://modelcontextprotocol.io)
- [Available MCP Servers](https://github.com/modelcontextprotocol/servers)
- [MCP Examples](../../mcp/examples/)
- [GitHub Example](../../mcp/examples/github/)

## 💬 Questions?

- [Open an Issue](https://github.com/Ranganaths/minion/issues)
- [Join Discussions](https://github.com/Ranganaths/minion/discussions)

---

**Excellent work! 🎉 Continue to [Tutorial 3](03-first-mcp-agent.md) when ready.**
