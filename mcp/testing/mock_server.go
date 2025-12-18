package testing

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/Ranganaths/minion/mcp/client"
)

// MockMCPServer is a mock MCP server for testing
type MockMCPServer struct {
	tools     []client.MCPTool
	callCount map[string]int
	mu        sync.RWMutex
	server    *http.Server
	responses map[string]interface{} // tool name → response
}

// NewMockMCPServer creates a new mock MCP server
func NewMockMCPServer() *MockMCPServer {
	return &MockMCPServer{
		tools:     []client.MCPTool{},
		callCount: make(map[string]int),
		responses: make(map[string]interface{}),
	}
}

// AddTool adds a tool to the mock server
func (m *MockMCPServer) AddTool(tool client.MCPTool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools = append(m.tools, tool)
}

// SetToolResponse sets a predefined response for a tool
func (m *MockMCPServer) SetToolResponse(toolName string, response interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[toolName] = response
}

// GetCallCount returns the number of times a tool was called
func (m *MockMCPServer) GetCallCount(toolName string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount[toolName]
}

// ResetCallCounts resets all call counters
func (m *MockMCPServer) ResetCallCounts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount = make(map[string]int)
}

// Start starts the HTTP server
func (m *MockMCPServer) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", m.handleRequest)

	m.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Mock server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the HTTP server
func (m *MockMCPServer) Stop() error {
	if m.server != nil {
		return m.server.Close()
	}
	return nil
}

// handleRequest handles incoming JSON-RPC requests
func (m *MockMCPServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse JSON-RPC request
	var request struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int64           `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}

	if err := json.Unmarshal(body, &request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Handle different methods
	var result interface{}
	var rpcErr *jsonrpcError

	switch request.Method {
	case "tools/list":
		result = m.handleToolsList()
	case "tools/call":
		result, rpcErr = m.handleToolsCall(request.Params)
	default:
		rpcErr = &jsonrpcError{
			Code:    -32601,
			Message: "Method not found",
		}
	}

	// Build response
	response := struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int64           `json:"id"`
		Result  interface{}     `json:"result,omitempty"`
		Error   *jsonrpcError   `json:"error,omitempty"`
	}{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result:  result,
		Error:   rpcErr,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleToolsList returns the list of available tools
func (m *MockMCPServer) handleToolsList() interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"tools": m.tools,
	}
}

// handleToolsCall handles a tool execution request
func (m *MockMCPServer) handleToolsCall(params json.RawMessage) (interface{}, *jsonrpcError) {
	var callRequest struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(params, &callRequest); err != nil {
		return nil, &jsonrpcError{
			Code:    -32602,
			Message: "Invalid params",
		}
	}

	// Increment call count
	m.mu.Lock()
	m.callCount[callRequest.Name]++
	m.mu.Unlock()

	// Check if tool exists
	m.mu.RLock()
	toolExists := false
	for _, tool := range m.tools {
		if tool.Name == callRequest.Name {
			toolExists = true
			break
		}
	}

	// Get predefined response
	response, hasResponse := m.responses[callRequest.Name]
	m.mu.RUnlock()

	if !toolExists {
		return nil, &jsonrpcError{
			Code:    -32602,
			Message: fmt.Sprintf("Tool not found: %s", callRequest.Name),
		}
	}

	// Return predefined response or default success
	if hasResponse {
		return response, nil
	}

	// Default success response
	return &client.MCPCallToolResult{
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("Tool %s executed successfully", callRequest.Name),
			},
		},
		IsError: false,
	}, nil
}

type jsonrpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// CreateTestTools creates a set of test tools
func CreateTestTools() []client.MCPTool {
	return []client.MCPTool{
		{
			Name:        "echo",
			Description: "Echoes the input message",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{
						"type":        "string",
						"description": "Message to echo",
					},
				},
				"required": []interface{}{"message"},
			},
		},
		{
			Name:        "calculate",
			Description: "Performs a calculation",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"operation": map[string]interface{}{
						"type": "string",
						"enum": []interface{}{"add", "subtract", "multiply", "divide"},
					},
					"a": map[string]interface{}{
						"type": "number",
					},
					"b": map[string]interface{}{
						"type": "number",
					},
				},
				"required": []interface{}{"operation", "a", "b"},
			},
		},
		{
			Name:        "get_status",
			Description: "Returns the server status",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

// CreateMockToolResponse creates a mock tool response
func CreateMockToolResponse(content string, isError bool) *client.MCPCallToolResult {
	return &client.MCPCallToolResult{
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": content,
			},
		},
		IsError: isError,
	}
}

// CreateMockJSONResponse creates a mock JSON response
func CreateMockJSONResponse(data interface{}) *client.MCPCallToolResult {
	jsonData, _ := json.Marshal(data)
	return &client.MCPCallToolResult{
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": string(jsonData),
			},
		},
		IsError: false,
	}
}
