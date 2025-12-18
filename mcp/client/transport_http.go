package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
)

// httpTransport implements MCP over HTTP
type httpTransport struct {
	config    *ClientConfig
	client    *http.Client
	requestID atomic.Int64
	connected bool
}

// newHTTPTransport creates a new HTTP transport
func newHTTPTransport(config *ClientConfig) (Transport, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("URL is required for HTTP transport")
	}

	httpClient := &http.Client{
		Timeout: config.RequestTimeout,
	}

	// TODO: Add authentication support

	return &httpTransport{
		config: config,
		client: httpClient,
	}, nil
}

// Connect establishes the connection
func (t *httpTransport) Connect(ctx context.Context) error {
	// For HTTP, we just mark as connected
	// Actual connection happens on first request
	t.connected = true
	return nil
}

// SendRequest sends a JSON-RPC request over HTTP
func (t *httpTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	if !t.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Generate request ID
	id := t.requestID.Add(1)

	// Create JSON-RPC request
	req := &jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Marshal request
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", t.config.URL, bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	// Read response body
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse JSON-RPC response
	var jsonResp jsonrpcResponse
	if err := json.Unmarshal(respData, &jsonResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for JSON-RPC error
	if jsonResp.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error %d: %s", jsonResp.Error.Code, jsonResp.Error.Message)
	}

	// Parse result based on method
	return t.parseResult(method, jsonResp.Result)
}

// Close closes the connection
func (t *httpTransport) Close() error {
	t.connected = false
	return nil
}

// IsConnected returns connection status
func (t *httpTransport) IsConnected() bool {
	return t.connected
}

// parseResult parses the result based on the method
func (t *httpTransport) parseResult(method string, result json.RawMessage) (interface{}, error) {
	switch method {
	case "tools/list":
		var toolsList struct {
			Tools []map[string]interface{} `json:"tools"`
		}
		if err := json.Unmarshal(result, &toolsList); err != nil {
			return nil, fmt.Errorf("failed to parse tools list: %w", err)
		}
		return map[string]interface{}{
			"tools": convertToInterfaceSlice(toolsList.Tools),
		}, nil

	case "tools/call":
		var toolResult MCPCallToolResult
		if err := json.Unmarshal(result, &toolResult); err != nil {
			return nil, fmt.Errorf("failed to parse tool result: %w", err)
		}
		return &toolResult, nil

	default:
		// Generic response
		var generic interface{}
		if err := json.Unmarshal(result, &generic); err != nil {
			return nil, fmt.Errorf("failed to parse result: %w", err)
		}
		return generic, nil
	}
}
