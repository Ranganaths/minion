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
	config      *ClientConfig
	client      *http.Client
	requestID   atomic.Int64
	connected   bool
	authHeaders map[string]string // Pre-computed auth headers
}

// BearerAuthConfig contains bearer token authentication configuration
type BearerAuthConfig struct {
	Token string `json:"token"`
}

// APIKeyAuthConfig contains API key authentication configuration
type APIKeyAuthConfig struct {
	Key       string `json:"key"`
	HeaderKey string `json:"header_key"` // Header name, defaults to "X-API-Key"
}

// OAuthAuthConfig contains OAuth authentication configuration
type OAuthAuthConfig struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"` // Usually "Bearer"
	RefreshToken string `json:"refresh_token,omitempty"`
}

// newHTTPTransport creates a new HTTP transport
func newHTTPTransport(config *ClientConfig) (Transport, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("URL is required for HTTP transport")
	}

	httpClient := &http.Client{
		Timeout: config.RequestTimeout,
	}

	// Build authentication headers
	authHeaders, err := buildAuthHeaders(config)
	if err != nil {
		return nil, fmt.Errorf("failed to configure authentication: %w", err)
	}

	return &httpTransport{
		config:      config,
		client:      httpClient,
		authHeaders: authHeaders,
	}, nil
}

// buildAuthHeaders builds authentication headers based on config
func buildAuthHeaders(config *ClientConfig) (map[string]string, error) {
	headers := make(map[string]string)

	switch config.AuthType {
	case AuthNone, "":
		// No authentication required
		return headers, nil

	case AuthBearer:
		bearerConfig, ok := config.AuthConfig.(*BearerAuthConfig)
		if !ok {
			// Try map conversion for flexibility
			if m, ok := config.AuthConfig.(map[string]interface{}); ok {
				if token, ok := m["token"].(string); ok {
					bearerConfig = &BearerAuthConfig{Token: token}
				}
			}
		}
		if bearerConfig == nil || bearerConfig.Token == "" {
			return nil, fmt.Errorf("bearer token is required for bearer authentication")
		}
		headers["Authorization"] = "Bearer " + bearerConfig.Token
		return headers, nil

	case AuthAPIKey:
		apiKeyConfig, ok := config.AuthConfig.(*APIKeyAuthConfig)
		if !ok {
			// Try map conversion for flexibility
			if m, ok := config.AuthConfig.(map[string]interface{}); ok {
				apiKeyConfig = &APIKeyAuthConfig{}
				if key, ok := m["key"].(string); ok {
					apiKeyConfig.Key = key
				}
				if headerKey, ok := m["header_key"].(string); ok {
					apiKeyConfig.HeaderKey = headerKey
				}
			}
		}
		if apiKeyConfig == nil || apiKeyConfig.Key == "" {
			return nil, fmt.Errorf("API key is required for API key authentication")
		}
		headerKey := apiKeyConfig.HeaderKey
		if headerKey == "" {
			headerKey = "X-API-Key" // Default header name
		}
		headers[headerKey] = apiKeyConfig.Key
		return headers, nil

	case AuthOAuth:
		oauthConfig, ok := config.AuthConfig.(*OAuthAuthConfig)
		if !ok {
			// Try map conversion for flexibility
			if m, ok := config.AuthConfig.(map[string]interface{}); ok {
				oauthConfig = &OAuthAuthConfig{}
				if token, ok := m["access_token"].(string); ok {
					oauthConfig.AccessToken = token
				}
				if tokenType, ok := m["token_type"].(string); ok {
					oauthConfig.TokenType = tokenType
				}
			}
		}
		if oauthConfig == nil || oauthConfig.AccessToken == "" {
			return nil, fmt.Errorf("access token is required for OAuth authentication")
		}
		tokenType := oauthConfig.TokenType
		if tokenType == "" {
			tokenType = "Bearer"
		}
		headers["Authorization"] = tokenType + " " + oauthConfig.AccessToken
		return headers, nil

	default:
		return nil, fmt.Errorf("unsupported authentication type: %s", config.AuthType)
	}
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

	// Add authentication headers
	for key, value := range t.authHeaders {
		httpReq.Header.Set(key, value)
	}

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
