# HTTP MCP Integration Example

This example demonstrates how to connect to remote MCP servers over HTTP.

## Overview

The HTTP transport allows you to connect to MCP servers running on remote machines, enabling distributed architectures where:
- MCP servers run on dedicated infrastructure
- Multiple Minion instances share the same MCP servers
- Services expose their functionality via MCP over HTTP

## Prerequisites

1. **Remote MCP Server**: An HTTP-based MCP server endpoint
2. **Network Access**: Connectivity to the remote server
3. **Authentication** (if required): API keys or bearer tokens

## Configuration

### Basic HTTP Connection
```go
config := &client.ClientConfig{
    ServerName: "remote-api",
    Transport:  client.TransportHTTP,
    URL:        "http://localhost:8080/mcp",
    AuthType:   client.AuthNone,
    ConnectTimeout: 10 * time.Second,
    RequestTimeout: 30 * time.Second,
}
```

### Authenticated Connection (Bearer Token)
```go
config := &client.ClientConfig{
    ServerName: "remote-api",
    Transport:  client.TransportHTTP,
    URL:        "https://api.example.com/mcp",
    AuthType:   client.AuthBearer,
    AuthToken:  "your-bearer-token",
    ConnectTimeout: 10 * time.Second,
    RequestTimeout: 30 * time.Second,
}
```

### Authenticated Connection (API Key)
```go
config := &client.ClientConfig{
    ServerName: "remote-api",
    Transport:  client.TransportHTTP,
    URL:        "https://api.example.com/mcp",
    AuthType:   client.AuthAPIKey,
    AuthToken:  "your-api-key",
    ConnectTimeout: 10 * time.Second,
    RequestTimeout: 30 * time.Second,
}
```

## Running the Example

```bash
cd mcp/examples/http
go run main.go
```

**Note**: This example requires a running HTTP MCP server. Update the URL in `main.go` to point to your server.

## HTTP vs Stdio Transport

| Feature | HTTP | Stdio |
|---------|------|-------|
| **Use Case** | Remote servers | Local subprocess |
| **Latency** | Higher (network) | Lower (IPC) |
| **Scalability** | Shared servers | Per-instance |
| **Authentication** | Built-in | OS-level |
| **Deployment** | Distributed | Monolithic |

## Security Considerations

1. **HTTPS**: Always use HTTPS in production
2. **Authentication**: Implement proper auth for public endpoints
3. **Rate Limiting**: Apply rate limits to prevent abuse
4. **Network Policies**: Use firewalls and network segmentation
5. **Token Rotation**: Regularly rotate authentication tokens

## Example MCP Server Endpoints

Common MCP servers that support HTTP:
- Custom enterprise tools (internal APIs)
- Cloud service integrations
- Database query interfaces
- Analytics platforms

## Troubleshooting

### Connection Refused
- Verify server is running: `curl http://localhost:8080/mcp`
- Check firewall rules
- Confirm correct URL and port

### Authentication Errors
- Verify token is valid
- Check token expiration
- Ensure correct AuthType

### Timeout Errors
- Increase ConnectTimeout or RequestTimeout
- Check network latency
- Verify server performance

## References

- [Model Context Protocol Specification](https://spec.modelcontextprotocol.io)
- [Minion MCP Integration Plan](../../../MCP_INTEGRATION_PLAN.md)
- [MCP Detailed Design](../../../MCP_DETAILED_DESIGN.md)
