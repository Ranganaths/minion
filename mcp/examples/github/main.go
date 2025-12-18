package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Ranganaths/minion/core"
	"github.com/Ranganaths/minion/mcp/client"
	"github.com/Ranganaths/minion/models"
)

func main() {
	// Initialize framework
	framework := core.NewFramework()
	defer framework.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Configure GitHub MCP server connection
	githubConfig := &client.ClientConfig{
		ServerName: "github",
		Transport:  client.TransportStdio,
		Command:    "npx",
		Args: []string{
			"-y",
			"@modelcontextprotocol/server-github",
		},
		Env: map[string]string{
			"GITHUB_PERSONAL_ACCESS_TOKEN": os.Getenv("GITHUB_TOKEN"),
		},
		ConnectTimeout: 10 * time.Second,
		RequestTimeout: 30 * time.Second,
	}

	// Connect to GitHub MCP server
	fmt.Println("Connecting to GitHub MCP server...")
	if err := framework.ConnectMCPServer(ctx, githubConfig); err != nil {
		log.Fatalf("Failed to connect to GitHub MCP server: %v", err)
	}
	fmt.Println("✓ Connected to GitHub MCP server")

	// List connected servers
	servers := framework.ListMCPServers()
	fmt.Printf("\nConnected MCP servers: %v\n", servers)

	// Get server status
	status := framework.GetMCPServerStatus()
	for name, s := range status {
		fmt.Printf("\nServer: %s\n", name)
		fmt.Printf("Status: %+v\n", s)
	}

	// Create an agent with MCP capabilities
	fmt.Println("\nCreating agent with MCP capabilities...")
	agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:         "GitHub Integration Agent",
		Description:  "Agent with access to GitHub MCP tools",
		BehaviorType: "default",
		Capabilities: []string{
			"mcp_integration", // Global MCP capability
			"mcp_github",      // GitHub-specific capability
		},
		Config: models.AgentConfig{
			LLMModel:    "gpt-4",
			Temperature: 0.7,
			MaxTokens:   2000,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	fmt.Printf("✓ Created agent: %s (ID: %s)\n", agent.Name, agent.ID)

	// List tools available to agent
	tools := framework.GetToolsForAgent(agent)
	fmt.Printf("\nAvailable tools for agent: %d\n", len(tools))
	for i, tool := range tools {
		if mcpTool, ok := tool.(interface{ Name() string }); ok {
			fmt.Printf("  %d. %s\n", i+1, mcpTool.Name())
		}
	}

	// Example: Create a GitHub issue (commented out - requires valid repo)
	/*
		fmt.Println("\nExecuting MCP tool: create_issue...")
		output, err := framework.Execute(ctx, agent.ID, &models.Input{
			Content: map[string]interface{}{
				"tool":   "mcp_github_create_issue",
				"params": map[string]interface{}{
					"owner": "your-username",
					"repo":  "your-repo",
					"title": "Test issue from Minion MCP integration",
					"body":  "This issue was created using Minion's MCP integration with GitHub",
				},
			},
		})
		if err != nil {
			log.Printf("Tool execution failed: %v", err)
		} else {
			fmt.Printf("✓ Tool executed successfully\n")
			fmt.Printf("Result: %+v\n", output.Result)
		}
	*/

	// Refresh tools (useful if server updates its tool list)
	fmt.Println("\nRefreshing MCP tools...")
	if err := framework.RefreshMCPTools(ctx, "github"); err != nil {
		log.Printf("Failed to refresh tools: %v", err)
	} else {
		fmt.Println("✓ Tools refreshed")
	}

	// Disconnect from server
	fmt.Println("\nDisconnecting from GitHub MCP server...")
	if err := framework.DisconnectMCPServer("github"); err != nil {
		log.Printf("Failed to disconnect: %v", err)
	} else {
		fmt.Println("✓ Disconnected from GitHub MCP server")
	}

	fmt.Println("\n=== GitHub MCP Integration Example Complete ===")
}
