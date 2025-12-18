package main

import (
	"context"
	"fmt"
	"log"
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

	// Configure remote MCP server via HTTP
	remoteConfig := &client.ClientConfig{
		ServerName: "remote-api",
		Transport:  client.TransportHTTP,
		URL:        "http://localhost:8080/mcp", // Example URL
		AuthType:   client.AuthNone,              // Or AuthBearer, AuthAPIKey, etc.
		// For authenticated servers:
		// AuthType: client.AuthBearer,
		// AuthToken: "your-api-token",
		ConnectTimeout: 10 * time.Second,
		RequestTimeout: 30 * time.Second,
	}

	// Connect to remote MCP server
	fmt.Println("Connecting to remote MCP server via HTTP...")
	if err := framework.ConnectMCPServer(ctx, remoteConfig); err != nil {
		log.Fatalf("Failed to connect to remote MCP server: %v", err)
	}
	fmt.Println("✓ Connected to remote MCP server")

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
		Name:         "Remote API Agent",
		Description:  "Agent with access to remote MCP tools",
		BehaviorType: "default",
		Capabilities: []string{
			"mcp_integration",  // Global MCP capability
			"mcp_remote-api",   // Server-specific capability
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
		if mcpTool, ok := tool.(interface{ Name() string; Description() string }); ok {
			fmt.Printf("  %d. %s - %s\n", i+1, mcpTool.Name(), mcpTool.Description())
		}
	}

	// Example: Call a remote tool
	/*
		fmt.Println("\nExecuting remote MCP tool...")
		output, err := framework.Execute(ctx, agent.ID, &models.Input{
			Content: map[string]interface{}{
				"tool": "mcp_remote-api_some_tool",
				"params": map[string]interface{}{
					"param1": "value1",
					"param2": "value2",
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

	// Disconnect from server
	fmt.Println("\nDisconnecting from remote MCP server...")
	if err := framework.DisconnectMCPServer("remote-api"); err != nil {
		log.Printf("Failed to disconnect: %v", err)
	} else {
		fmt.Println("✓ Disconnected from remote MCP server")
	}

	fmt.Println("\n=== HTTP MCP Integration Example Complete ===")
}
