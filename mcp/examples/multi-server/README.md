# Multi-Server MCP Integration Example

This example demonstrates how to connect to multiple MCP servers simultaneously and create agents that can use tools from all connected servers.

## Overview

The multi-server pattern allows you to:
- Connect to multiple MCP servers at once (GitHub, Slack, Gmail, etc.)
- Create agents with capabilities spanning all servers
- Use tools from different servers in a single workflow
- Build powerful automation that crosses service boundaries

## Prerequisites

Depending on which servers you want to connect to:

1. **GitHub Server**: `GITHUB_TOKEN` environment variable
2. **Slack Server**: `SLACK_TOKEN` environment variable
3. **Gmail Server**: Gmail API credentials
4. **Node.js and npx**: For stdio-based MCP servers

## Setup

```bash
# Export tokens for the services you want to use
export GITHUB_TOKEN=your_github_token
export SLACK_TOKEN=your_slack_bot_token

# Install dependencies
go mod download
```

## Running the Example

```bash
cd mcp/examples/multi-server
go run main.go
```

The example will:
1. Attempt to connect to GitHub, Slack, and Gmail MCP servers
2. Continue even if some servers fail (graceful degradation)
3. Create an agent with capabilities for all connected servers
4. List all available tools grouped by server
5. Disconnect from all servers on exit

## Architecture

```
┌─────────────────────────────────────────┐
│         Minion Framework                │
│  ┌───────────────────────────────────┐  │
│  │   MCP Client Manager              │  │
│  │  ┌──────────┐  ┌──────────┐      │  │
│  │  │ GitHub   │  │  Slack   │ ...  │  │
│  │  │ Client   │  │  Client  │      │  │
│  │  └────┬─────┘  └────┬─────┘      │  │
│  └───────┼─────────────┼─────────────┘  │
│          │             │                 │
│  ┌───────┼─────────────┼─────────────┐  │
│  │       ▼             ▼             │  │
│  │    MCP Tool Bridge                │  │
│  │  ┌─────────┐    ┌─────────┐      │  │
│  │  │ GitHub  │    │ Slack   │      │  │
│  │  │ Tools   │    │ Tools   │ ...  │  │
│  │  └────┬────┘    └────┬────┘      │  │
│  └───────┼──────────────┼────────────┘  │
│          │              │                │
│  ┌───────▼──────────────▼─────────────┐ │
│  │      Agent Tool Registry           │ │
│  │   mcp_github_*, mcp_slack_*, ...   │ │
│  └────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

## Capability Model

Agents can have three levels of MCP access:

1. **Global**: `mcp_integration` - Access to all MCP tools
2. **Server-specific**: `mcp_github` - Access to all GitHub tools
3. **Tool-specific**: `mcp_github_create_issue` - Access to specific tool only

Example multi-server capabilities:
```go
capabilities := []string{
    "mcp_integration",  // Global access
    "mcp_github",       // All GitHub tools
    "mcp_slack",        // All Slack tools
    "mcp_gmail",        // All Gmail tools
}
```

## Example Workflow

Imagine an agent that automates a development workflow:

1. **Monitor GitHub** for new issues
2. **Create Slack notification** when high-priority issue is created
3. **Send email** to stakeholders with issue details
4. **Create calendar event** for planning discussion

All of this is possible with a single agent using multiple MCP servers!

## Available MCP Servers

Common MCP servers you can integrate:

| Server | Transport | Tools |
|--------|-----------|-------|
| **GitHub** | stdio | Issues, PRs, code search, repos |
| **Slack** | stdio | Messages, channels, users |
| **Gmail** | stdio | Send/read emails, labels |
| **Google Drive** | stdio | Files, folders, sharing |
| **Google Calendar** | stdio | Events, scheduling |
| **Notion** | stdio | Pages, databases |
| **Jira** | stdio | Issues, projects |
| **Custom APIs** | HTTP | Your own tools |

## Performance Considerations

1. **Connection Overhead**: Each server adds startup time
2. **Resource Usage**: More connections = more memory/processes
3. **Error Handling**: One server failure shouldn't crash others
4. **Timeout Management**: Set appropriate timeouts per server

## Best Practices

1. **Graceful Degradation**: Continue if some servers fail
2. **Connection Pooling**: Reuse connections across agents
3. **Capability Scoping**: Give agents minimal required capabilities
4. **Error Isolation**: Handle server failures independently
5. **Monitoring**: Track health of each server connection
6. **Resource Cleanup**: Always disconnect on shutdown

## Troubleshooting

### Some Servers Fail to Connect
- Check required environment variables are set
- Verify API tokens are valid
- Ensure npx can install required packages
- Check network connectivity

### High Memory Usage
- Reduce number of concurrent servers
- Disconnect unused servers
- Monitor process count with `ps aux | grep node`

### Tool Discovery Slow
- Each server discovery adds latency
- Consider caching tool lists
- Use asynchronous connection pattern

## References

- [MCP Server Implementations](https://github.com/modelcontextprotocol/servers)
- [Minion MCP Integration Plan](../../../MCP_INTEGRATION_PLAN.md)
- [MCP Detailed Design](../../../MCP_DETAILED_DESIGN.md)
