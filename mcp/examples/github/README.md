# GitHub MCP Integration Example

This example demonstrates how to integrate GitHub functionality into Minion agents using the Model Context Protocol (MCP).

## Prerequisites

1. **GitHub Personal Access Token**: You need a GitHub token with appropriate permissions
2. **Node.js and npx**: Required to run the GitHub MCP server
3. **GitHub MCP Server**: Installed via npx (auto-installed on first run)

## Setup

1. Export your GitHub token:
```bash
export GITHUB_TOKEN=your_github_personal_access_token
```

2. Install dependencies:
```bash
go mod download
```

## Running the Example

```bash
cd mcp/examples/github
go run main.go
```

## What This Example Does

1. **Connect to GitHub MCP Server**: Establishes connection via stdio transport
2. **List Available Tools**: Shows all GitHub tools exposed by the MCP server
3. **Create Agent with MCP Capabilities**: Creates an agent with:
   - `mcp_integration`: Global MCP capability
   - `mcp_github`: GitHub-specific capability
4. **List Tools for Agent**: Shows which tools the agent can access
5. **Refresh Tools**: Demonstrates tool list refresh
6. **Disconnect**: Properly closes the MCP connection

## Available GitHub Tools

The GitHub MCP server typically provides tools like:
- `create_issue`: Create a new issue
- `create_pull_request`: Create a pull request
- `get_issue`: Get issue details
- `list_issues`: List repository issues
- `comment_on_issue`: Add a comment to an issue
- And more...

## Using GitHub Tools

To use a GitHub tool in your agent, call it with the `mcp_github_` prefix:

```go
output, err := framework.Execute(ctx, agent.ID, &models.Input{
    Content: map[string]interface{}{
        "tool": "mcp_github_create_issue",
        "params": map[string]interface{}{
            "owner": "your-username",
            "repo": "your-repo",
            "title": "Issue title",
            "body": "Issue description",
        },
    },
})
```

## Troubleshooting

### Connection Timeout
If connection fails, check:
- Node.js is installed: `node --version`
- npx is available: `npx --version`
- GitHub token is valid and exported

### Tool Execution Fails
Verify:
- Agent has required capabilities: `mcp_integration` or `mcp_github`
- Tool parameters match the MCP server's schema
- GitHub token has necessary permissions

## References

- [MCP GitHub Server](https://github.com/modelcontextprotocol/servers/tree/main/src/github)
- [Model Context Protocol](https://modelcontextprotocol.io)
- [Minion MCP Integration Plan](../../../MCP_INTEGRATION_PLAN.md)
