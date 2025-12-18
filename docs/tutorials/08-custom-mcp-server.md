# Tutorial 8: Building a Custom MCP Server

**Duration**: 1.5 hours
**Level**: Advanced
**Prerequisites**: Tutorials 1-7

## 🎯 Learning Objectives

By the end of this tutorial, you will:
- Understand the MCP Server specification
- Build a custom MCP server from scratch
- Implement tool discovery and execution
- Handle stdio and HTTP transports
- Add resource management
- Test your custom server with Minion

## 📚 What is an MCP Server?

An **MCP Server** exposes tools, resources, and prompts that AI agents can use:

### MCP Server Components:

1. **Tools**: Functions that agents can execute
2. **Resources**: Data that agents can read
3. **Prompts**: Templates for AI conversations
4. **Transports**: stdio (local) or HTTP (remote)

### Real-World Examples:
- **Database Server**: Exposes query, insert, update, delete tools
- **Analytics Server**: Exposes metrics, reports, dashboards
- **Deployment Server**: Exposes deploy, rollback, scale tools
- **Internal API Server**: Exposes company-specific operations

## 🏗️ MCP Server Architecture

```
┌─────────────────────────────────────────────────┐
│          Your Custom MCP Server                  │
│                                                  │
│  ┌────────────────────────────────────────┐    │
│  │         Tool Registry                   │    │
│  │  • tool_1: Do something                 │    │
│  │  • tool_2: Do something else            │    │
│  │  • tool_N: Do another thing             │    │
│  └────────────────────────────────────────┘    │
│                                                  │
│  ┌────────────────────────────────────────┐    │
│  │       Resource Registry                 │    │
│  │  • resource_1: Some data                │    │
│  │  • resource_2: More data                │    │
│  └────────────────────────────────────────┘    │
│                                                  │
│  ┌────────────────────────────────────────┐    │
│  │         Transport Layer                 │    │
│  │  • Stdio (stdin/stdout)                 │    │
│  │  • HTTP (REST API)                      │    │
│  └────────────────────────────────────────┘    │
└─────────────────────────────────────────────────┘
```

## 🛠️ Part 1: MCP Protocol Basics

### Message Format

All MCP messages use JSON-RPC 2.0:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list",
  "params": {}
}
```

### Core Methods

```
Initialization:
- initialize: Handshake and capability exchange
- initialized: Confirmation of initialization

Tools:
- tools/list: Get available tools
- tools/call: Execute a tool

Resources:
- resources/list: Get available resources
- resources/read: Read a resource

Prompts:
- prompts/list: Get available prompts
- prompts/get: Get a specific prompt
```

## 🛠️ Part 2: Building a Database MCP Server

We'll build a server that exposes database operations as tools.

### Step 1: Project Setup

```bash
mkdir custom-mcp-server
cd custom-mcp-server

go mod init github.com/yourusername/mcp-db-server
```

### Step 2: Define Data Structures

Create `types.go`:

```go
package main

import "encoding/json"

// JSON-RPC Request
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSON-RPC Response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// JSON-RPC Error
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Initialize Request
type InitializeRequest struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Initialize Response
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

type Capabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe"`
	ListChanged bool `json:"listChanged"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool Definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Tools List Result
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// Tool Call Request
type ToolCallRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// Tool Call Result
type ToolCallResult struct {
	Content []Content `json:"content"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
```

### Step 3: Implement the Server

Create `server.go`:

```go
package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

type MCPServer struct {
	db      *sql.DB
	scanner *bufio.Scanner
	tools   []Tool
}

func NewMCPServer(dbPath string) (*MCPServer, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	server := &MCPServer{
		db:      db,
		scanner: bufio.NewScanner(os.Stdin),
		tools:   make([]Tool, 0),
	}

	// Register tools
	server.registerTools()

	return server, nil
}

func (s *MCPServer) registerTools() {
	s.tools = []Tool{
		{
			Name:        "db_query",
			Description: "Execute a SELECT query on the database",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "SQL SELECT query to execute",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "db_execute",
			Description: "Execute an INSERT, UPDATE, or DELETE query",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "SQL query to execute",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "db_tables",
			Description: "List all tables in the database",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "db_schema",
			Description: "Get the schema of a table",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table": map[string]interface{}{
						"type":        "string",
						"description": "Table name",
					},
				},
				"required": []string{"table"},
			},
		},
	}
}

func (s *MCPServer) Start() {
	log.Println("MCP Database Server started")

	for s.scanner.Scan() {
		line := s.scanner.Bytes()
		s.handleRequest(line)
	}

	if err := s.scanner.Err(); err != nil {
		log.Fatalf("Scanner error: %v", err)
	}
}

func (s *MCPServer) handleRequest(data []byte) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		s.sendError(nil, -32700, "Parse error", nil)
		return
	}

	var result interface{}
	var err error

	switch req.Method {
	case "initialize":
		result, err = s.handleInitialize(req.Params)
	case "initialized":
		// Acknowledge
		return
	case "tools/list":
		result, err = s.handleToolsList()
	case "tools/call":
		result, err = s.handleToolCall(req.Params)
	default:
		s.sendError(req.ID, -32601, "Method not found", nil)
		return
	}

	if err != nil {
		s.sendError(req.ID, -32603, err.Error(), nil)
		return
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleInitialize(params json.RawMessage) (*InitializeResult, error) {
	var req InitializeRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, err
	}

	return &InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "mcp-db-server",
			Version: "1.0.0",
		},
	}, nil
}

func (s *MCPServer) handleToolsList() (*ToolsListResult, error) {
	return &ToolsListResult{
		Tools: s.tools,
	}, nil
}

func (s *MCPServer) handleToolCall(params json.RawMessage) (*ToolCallResult, error) {
	var req ToolCallRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, err
	}

	var text string
	var err error

	switch req.Name {
	case "db_query":
		text, err = s.executeQuery(req.Arguments["query"].(string))
	case "db_execute":
		text, err = s.executeSQL(req.Arguments["query"].(string))
	case "db_tables":
		text, err = s.listTables()
	case "db_schema":
		text, err = s.getSchema(req.Arguments["table"].(string))
	default:
		return nil, fmt.Errorf("unknown tool: %s", req.Name)
	}

	if err != nil {
		return nil, err
	}

	return &ToolCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: text,
			},
		},
	}, nil
}

func (s *MCPServer) executeQuery(query string) (string, error) {
	rows, err := s.db.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	results := make([]map[string]interface{}, 0)

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return "", err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (s *MCPServer) executeSQL(query string) (string, error) {
	result, err := s.db.Exec(query)
	if err != nil {
		return "", err
	}

	rowsAffected, _ := result.RowsAffected()
	return fmt.Sprintf("Query executed successfully. Rows affected: %d", rowsAffected), nil
}

func (s *MCPServer) listTables() (string, error) {
	query := "SELECT name FROM sqlite_master WHERE type='table'"
	rows, err := s.db.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	tables := make([]string, 0)
	for rows.Next() {
		var table string
		rows.Scan(&table)
		tables = append(tables, table)
	}

	data, _ := json.MarshalIndent(tables, "", "  ")
	return string(data), nil
}

func (s *MCPServer) getSchema(table string) (string, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", table)
	rows, err := s.db.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	schema := make([]map[string]interface{}, 0)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt interface{}

		rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk)

		schema = append(schema, map[string]interface{}{
			"column": name,
			"type":   typ,
			"pk":     pk == 1,
		})
	}

	data, _ := json.MarshalIndent(schema, "", "  ")
	return string(data), nil
}

func (s *MCPServer) sendResponse(id interface{}, result interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func (s *MCPServer) sendError(id interface{}, code int, message string, data interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	respData, _ := json.Marshal(resp)
	fmt.Println(string(respData))
}

func (s *MCPServer) Close() {
	s.db.Close()
}
```

### Step 4: Main Entry Point

Create `main.go`:

```go
package main

import (
	"log"
	"os"
)

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./test.db"
	}

	server, err := NewMCPServer(dbPath)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	server.Start()
}
```

### Step 5: Build the Server

```bash
go get github.com/mattn/go-sqlite3
go build -o mcp-db-server
```

## 🛠️ Part 3: Testing Your Custom Server

### Create Test Database

```bash
sqlite3 test.db <<EOF
CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  email TEXT NOT NULL
);

INSERT INTO users (name, email) VALUES
  ('Alice', 'alice@example.com'),
  ('Bob', 'bob@example.com'),
  ('Charlie', 'charlie@example.com');
EOF
```

### Test with Minion

Create `test_client.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/yourusername/minion/core"
	"github.com/yourusername/minion/mcp/client"
	"github.com/yourusername/minion/models"
)

func main() {
	ctx := context.Background()

	// Create framework
	fw := core.NewFramework()
	defer fw.Close()

	// Connect to custom MCP server
	err := fw.ConnectMCPServer(ctx, &client.ClientConfig{
		ServerName: "database",
		Transport:  client.TransportStdio,
		Command:    "./mcp-db-server",
		Args:       []string{},
		Env: map[string]string{
			"DB_PATH": "./test.db",
		},
	})

	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Create agent with database access
	agent, err := fw.CreateAgent(ctx, &models.CreateAgentRequest{
		Name:         "DBAgent",
		Capabilities: []string{"mcp_database"},
	})

	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// List tools
	tools := fw.GetToolsForAgent(agent)
	fmt.Println("Available tools:")
	for _, tool := range tools {
		fmt.Printf("  - %s: %s\n", tool.Name(), tool.Description())
	}

	// Test 1: List tables
	fmt.Println("\n=== Test 1: List Tables ===")
	result, _ := fw.ExecuteTool(ctx, agent.ID, &models.ToolInput{
		ToolName: "mcp_database_db_tables",
		Params:   map[string]interface{}{},
	})
	fmt.Printf("Result: %v\n", result.Result)

	// Test 2: Get schema
	fmt.Println("\n=== Test 2: Get Schema ===")
	result, _ = fw.ExecuteTool(ctx, agent.ID, &models.ToolInput{
		ToolName: "mcp_database_db_schema",
		Params: map[string]interface{}{
			"table": "users",
		},
	})
	fmt.Printf("Result: %v\n", result.Result)

	// Test 3: Query data
	fmt.Println("\n=== Test 3: Query Users ===")
	result, _ = fw.ExecuteTool(ctx, agent.ID, &models.ToolInput{
		ToolName: "mcp_database_db_query",
		Params: map[string]interface{}{
			"query": "SELECT * FROM users",
		},
	})
	fmt.Printf("Result: %v\n", result.Result)

	// Test 4: Insert data
	fmt.Println("\n=== Test 4: Insert User ===")
	result, _ = fw.ExecuteTool(ctx, agent.ID, &models.ToolInput{
		ToolName: "mcp_database_db_execute",
		Params: map[string]interface{}{
			"query": "INSERT INTO users (name, email) VALUES ('David', 'david@example.com')",
		},
	})
	fmt.Printf("Result: %v\n", result.Result)

	// Test 5: Query again
	fmt.Println("\n=== Test 5: Query After Insert ===")
	result, _ = fw.ExecuteTool(ctx, agent.ID, &models.ToolInput{
		ToolName: "mcp_database_db_query",
		Params: map[string]interface{}{
			"query": "SELECT * FROM users WHERE name = 'David'",
		},
	})
	fmt.Printf("Result: %v\n", result.Result)

	fmt.Println("\n✅ All tests completed!")
}
```

### Run Tests

```bash
go run test_client.go
```

**Expected Output:**
```
Available tools:
  - mcp_database_db_query: Execute a SELECT query on the database
  - mcp_database_db_execute: Execute an INSERT, UPDATE, or DELETE query
  - mcp_database_db_tables: List all tables in the database
  - mcp_database_db_schema: Get the schema of a table

=== Test 1: List Tables ===
Result: ["users"]

=== Test 2: Get Schema ===
Result: [{"column":"id","type":"INTEGER","pk":true},{"column":"name","type":"TEXT","pk":false},{"column":"email","type":"TEXT","pk":false}]

=== Test 3: Query Users ===
Result: [{"id":1,"name":"Alice","email":"alice@example.com"},{"id":2,"name":"Bob","email":"bob@example.com"},{"id":3,"name":"Charlie","email":"charlie@example.com"}]

=== Test 4: Insert User ===
Result: Query executed successfully. Rows affected: 1

=== Test 5: Query After Insert ===
Result: [{"id":4,"name":"David","email":"david@example.com"}]

✅ All tests completed!
```

## 🛠️ Part 4: Adding HTTP Transport

Add HTTP support to your server:

```go
// Add to server.go

import (
	"net/http"
)

func (s *MCPServer) StartHTTP(addr string) error {
	http.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Process request (same as stdio)
		var result interface{}
		var err error

		switch req.Method {
		case "initialize":
			result, err = s.handleInitialize(req.Params)
		case "tools/list":
			result, err = s.handleToolsList()
		case "tools/call":
			result, err = s.handleToolCall(req.Params)
		default:
			http.Error(w, "Method not found", http.StatusNotFound)
			return
		}

		if err != nil {
			resp := Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &Error{
					Code:    -32603,
					Message: err.Error(),
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		resp := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	log.Printf("HTTP server listening on %s\n", addr)
	return http.ListenAndServe(addr, nil)
}
```

Update `main.go`:

```go
func main() {
	mode := os.Getenv("MODE")
	if mode == "http" {
		addr := os.Getenv("HTTP_ADDR")
		if addr == "" {
			addr = ":8080"
		}

		server, _ := NewMCPServer("./test.db")
		defer server.Close()

		log.Fatal(server.StartHTTP(addr))
	} else {
		// Stdio mode (default)
		server, _ := NewMCPServer("./test.db")
		defer server.Close()
		server.Start()
	}
}
```

### Test HTTP Mode

```bash
# Start server in HTTP mode
MODE=http HTTP_ADDR=:8080 ./mcp-db-server

# Connect from Minion
fw.ConnectMCPServer(ctx, &client.ClientConfig{
	ServerName: "database",
	Transport:  client.TransportHTTP,
	URL:        "http://localhost:8080/mcp",
})
```

## 🏋️ Practice Exercises

### Exercise 1: Add Resource Support

Add resources to expose database schemas.

<details>
<summary>Click to see solution</summary>

```go
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

func (s *MCPServer) handleResourcesList() (*ResourcesListResult, error) {
	return &ResourcesListResult{
		Resources: []Resource{
			{
				URI:         "db://schema",
				Name:        "Database Schema",
				Description: "Complete database schema",
				MimeType:    "application/json",
			},
		},
	}, nil
}

func (s *MCPServer) handleResourceRead(params json.RawMessage) (*ResourceReadResult, error) {
	var req ResourceReadRequest
	json.Unmarshal(params, &req)

	if req.URI == "db://schema" {
		schema, _ := s.getFullSchema()
		return &ResourceReadResult{
			Contents: []Content{
				{Type: "text", Text: schema},
			},
		}, nil
	}

	return nil, fmt.Errorf("resource not found")
}
```
</details>

### Exercise 2: Add Authentication

Add API key authentication to HTTP mode.

<details>
<summary>Click to see solution</summary>

```go
func (s *MCPServer) StartHTTP(addr, apiKey string) error {
	http.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		// Check API key
		if r.Header.Get("X-API-Key") != apiKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// ... rest of handler
	})

	return http.ListenAndServe(addr, nil)
}
```
</details>

## 📝 Summary

Congratulations! You've learned:

✅ MCP protocol specification
✅ Building custom MCP servers
✅ Implementing tools and resources
✅ Supporting stdio and HTTP transports
✅ Testing with Minion framework

### When to Build Custom Servers

Build a custom MCP server when:
- You have internal APIs to expose
- You need specialized domain tools
- You want to abstract complex operations
- You need custom authentication

## 🎯 Next Steps

**[Tutorial 9: Advanced Patterns →](09-advanced-patterns.md)**

Learn advanced patterns for building production systems!

---

**Great job! 🎉 Continue to [Tutorial 9](09-advanced-patterns.md) when ready.**
