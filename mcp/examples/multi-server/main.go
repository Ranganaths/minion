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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("=== Multi-Server MCP Integration Example ===")
	fmt.Println()

	// Configure multiple MCP servers
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
				Args: []string{
					"-y",
					"@modelcontextprotocol/server-github",
				},
				Env: map[string]string{
					"GITHUB_PERSONAL_ACCESS_TOKEN": os.Getenv("GITHUB_TOKEN"),
				},
				ConnectTimeout: 10 * time.Second,
				RequestTimeout: 30 * time.Second,
			},
		},
		{
			name: "Slack",
			config: &client.ClientConfig{
				ServerName: "slack",
				Transport:  client.TransportStdio,
				Command:    "npx",
				Args: []string{
					"-y",
					"@modelcontextprotocol/server-slack",
				},
				Env: map[string]string{
					"SLACK_BOT_TOKEN": os.Getenv("SLACK_TOKEN"),
				},
				ConnectTimeout: 10 * time.Second,
				RequestTimeout: 30 * time.Second,
			},
		},
		{
			name: "Gmail",
			config: &client.ClientConfig{
				ServerName: "gmail",
				Transport:  client.TransportStdio,
				Command:    "npx",
				Args: []string{
					"-y",
					"@modelcontextprotocol/server-gmail",
				},
				ConnectTimeout: 10 * time.Second,
				RequestTimeout: 30 * time.Second,
			},
		},
	}

	// Connect to all servers
	connectedServers := []string{}
	for _, srv := range servers {
		fmt.Printf("Connecting to %s MCP server...\n", srv.name)
		if err := framework.ConnectMCPServer(ctx, srv.config); err != nil {
			log.Printf("Warning: Failed to connect to %s: %v\n", srv.name, err)
			continue
		}
		fmt.Printf("✓ Connected to %s MCP server\n", srv.name)
		connectedServers = append(connectedServers, srv.config.ServerName)
	}

	if len(connectedServers) == 0 {
		log.Fatal("Failed to connect to any MCP servers")
	}

	// List all connected servers
	fmt.Printf("\n=== Connected Servers ===\n")
	allServers := framework.ListMCPServers()
	for i, name := range allServers {
		fmt.Printf("%d. %s\n", i+1, name)
	}

	// Get status of all servers
	fmt.Printf("\n=== Server Status ===\n")
	status := framework.GetMCPServerStatus()
	for name, s := range status {
		fmt.Printf("\nServer: %s\n", name)
		if clientStatus, ok := s.(*client.MCPClientStatus); ok {
			fmt.Printf("  Connected: %v\n", clientStatus.Connected)
			fmt.Printf("  Transport: %s\n", clientStatus.Transport)
			fmt.Printf("  Tools: %d\n", clientStatus.ToolsDiscovered)
			fmt.Printf("  Total Calls: %d\n", clientStatus.TotalCalls)
			fmt.Printf("  Success: %d\n", clientStatus.SuccessCalls)
			fmt.Printf("  Failed: %d\n", clientStatus.FailedCalls)
		}
	}

	// Create an agent with capabilities for all servers
	fmt.Println("\n=== Creating Multi-Server Agent ===")
	capabilities := []string{"mcp_integration"} // Global MCP capability
	for _, serverName := range connectedServers {
		capabilities = append(capabilities, fmt.Sprintf("mcp_%s", serverName))
	}

	agent, err := framework.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:         "Multi-Server Integration Agent",
		Description:  "Agent with access to multiple MCP servers",
		BehaviorType: "default",
		Capabilities: capabilities,
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
	fmt.Printf("  Capabilities: %v\n", agent.Capabilities)

	// List all tools available to agent
	fmt.Printf("\n=== Available Tools ===\n")
	tools := framework.GetToolsForAgent(agent)
	fmt.Printf("Total tools: %d\n\n", len(tools))

	// Group tools by server
	toolsByServer := make(map[string][]string)
	for _, tool := range tools {
		if mcpTool, ok := tool.(interface{ Name() string }); ok {
			toolName := mcpTool.Name()
			// Extract server name from tool name (format: mcp_<server>_<tool>)
			for _, serverName := range connectedServers {
				prefix := fmt.Sprintf("mcp_%s_", serverName)
				if len(toolName) > len(prefix) && toolName[:len(prefix)] == prefix {
					toolsByServer[serverName] = append(toolsByServer[serverName], toolName)
					break
				}
			}
		}
	}

	// Display tools grouped by server
	for serverName, serverTools := range toolsByServer {
		fmt.Printf("Tools from %s: (%d)\n", serverName, len(serverTools))
		for i, toolName := range serverTools {
			if i < 5 { // Show first 5 tools
				fmt.Printf("  - %s\n", toolName)
			} else if i == 5 {
				fmt.Printf("  ... and %d more\n", len(serverTools)-5)
				break
			}
		}
		fmt.Println()
	}

	// Example: Execute tools from different servers
	/*
		fmt.Println("=== Example Tool Executions ===\n")

		// GitHub: Create issue
		fmt.Println("1. Creating GitHub issue...")
		output, err := framework.Execute(ctx, agent.ID, &models.Input{
			Content: map[string]interface{}{
				"tool": "mcp_github_create_issue",
				"params": map[string]interface{}{
					"owner": "your-username",
					"repo":  "your-repo",
					"title": "Multi-server integration test",
					"body":  "Created from multi-server example",
				},
			},
		})
		if err != nil {
			log.Printf("GitHub tool failed: %v", err)
		} else {
			fmt.Printf("✓ GitHub issue created: %+v\n\n", output.Result)
		}

		// Slack: Send message
		fmt.Println("2. Sending Slack message...")
		output, err = framework.Execute(ctx, agent.ID, &models.Input{
			Content: map[string]interface{}{
				"tool": "mcp_slack_send_message",
				"params": map[string]interface{}{
					"channel": "#general",
					"text":    "Hello from Minion MCP integration!",
				},
			},
		})
		if err != nil {
			log.Printf("Slack tool failed: %v", err)
		} else {
			fmt.Printf("✓ Slack message sent: %+v\n\n", output.Result)
		}

		// Gmail: Send email
		fmt.Println("3. Sending email...")
		output, err = framework.Execute(ctx, agent.ID, &models.Input{
			Content: map[string]interface{}{
				"tool": "mcp_gmail_send_email",
				"params": map[string]interface{}{
					"to":      "recipient@example.com",
					"subject": "Test from Minion",
					"body":    "This is a test email from Minion MCP integration",
				},
			},
		})
		if err != nil {
			log.Printf("Gmail tool failed: %v", err)
		} else {
			fmt.Printf("✓ Email sent: %+v\n\n", output.Result)
		}
	*/

	// Disconnect from all servers
	fmt.Println("\n=== Cleanup ===")
	for _, serverName := range connectedServers {
		fmt.Printf("Disconnecting from %s...\n", serverName)
		if err := framework.DisconnectMCPServer(serverName); err != nil {
			log.Printf("Warning: Failed to disconnect from %s: %v", serverName, err)
		} else {
			fmt.Printf("✓ Disconnected from %s\n", serverName)
		}
	}

	fmt.Println("\n=== Multi-Server MCP Integration Example Complete ===")
}
