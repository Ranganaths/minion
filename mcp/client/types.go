package client

import (
	"context"
	"sync"
	"time"
)

// TransportType defines the type of transport used for MCP communication
type TransportType string

const (
	TransportStdio TransportType = "stdio"
	TransportHTTP  TransportType = "http"
)

// AuthType defines the authentication method
type AuthType string

const (
	AuthNone   AuthType = "none"
	AuthBearer AuthType = "bearer"
	AuthOAuth  AuthType = "oauth"
	AuthAPIKey AuthType = "apikey"
)

// ConnectionState tracks the lifecycle of a client connection
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateError
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// MCPTool represents a tool available from an MCP server
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPCallToolRequest represents a request to call a tool
type MCPCallToolRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// MCPCallToolResult represents the result of a tool call
type MCPCallToolResult struct {
	Content []interface{} `json:"content"`
	IsError bool          `json:"isError"`
}

// ManagerConfig configures the MCPClientManager
type ManagerConfig struct {
	MaxServers          int
	DefaultTimeout      time.Duration
	EnableAutoReconnect bool
	MaxReconnectRetries int
	ReconnectBackoff    time.Duration
}

// DefaultManagerConfig returns the default manager configuration
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		MaxServers:          10,
		DefaultTimeout:      30 * time.Second,
		EnableAutoReconnect: true,
		MaxReconnectRetries: 3,
		ReconnectBackoff:    2 * time.Second,
	}
}

// ClientConfig configures a single MCP server connection
type ClientConfig struct {
	// Server identity
	ServerName  string
	Description string

	// Transport configuration
	Transport TransportType

	// For stdio transport
	Command    string
	Args       []string
	Env        map[string]string
	WorkingDir string

	// For HTTP transport
	URL string

	// Authentication
	AuthType   AuthType
	AuthConfig interface{}

	// Capabilities
	Capabilities []string

	// Timeouts
	ConnectTimeout time.Duration
	RequestTimeout time.Duration
}

// DefaultClientConfig returns a default client configuration
func DefaultClientConfig(serverName string) *ClientConfig {
	return &ClientConfig{
		ServerName:     serverName,
		Transport:      TransportStdio,
		AuthType:       AuthNone,
		ConnectTimeout: 30 * time.Second,
		RequestTimeout: 60 * time.Second,
		Env:            make(map[string]string),
		Capabilities:   []string{},
	}
}

// MCPClientManager manages connections to external MCP servers
type MCPClientManager struct {
	clients map[string]*MCPClient
	mu      sync.RWMutex
	config  *ManagerConfig
	ctx     context.Context
	cancel  context.CancelFunc
}

// MCPClient represents a connection to a single MCP server
type MCPClient struct {
	serverName string
	config     *ClientConfig

	// Connection state
	transport  Transport
	tools      []MCPTool
	toolsMu    sync.RWMutex
	connected  bool
	stateMu    sync.RWMutex

	// Reconnection tracking
	reconnectAttempts int
	lastConnectTime   time.Time

	// Metrics
	metrics *clientMetrics
}

// clientMetrics tracks client performance
type clientMetrics struct {
	toolCallsTotal     int64
	toolCallsSucceeded int64
	toolCallsFailed    int64
	avgResponseTime    float64
	lastError          string
	lastErrorTime      time.Time
	mu                 sync.RWMutex
}

// Transport defines the interface for MCP transports
type Transport interface {
	// Connect establishes the connection
	Connect(ctx context.Context) error

	// SendRequest sends a JSON-RPC request
	SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error)

	// Close closes the connection
	Close() error

	// IsConnected returns connection status
	IsConnected() bool
}

// MCPClientStatus represents the status of a client connection
type MCPClientStatus struct {
	ServerName      string
	Connected       bool
	Transport       string
	ToolsDiscovered int
	TotalCalls      int64
	SuccessCalls    int64
	FailedCalls     int64
	LastError       string
	LastErrorTime   time.Time
}
