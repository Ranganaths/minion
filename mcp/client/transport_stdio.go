package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
)

// stdioTransport implements MCP over stdio (standard input/output)
type stdioTransport struct {
	config *ClientConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	// Request tracking
	requestID  atomic.Int64
	pending    map[int64]chan *jsonrpcResponse
	pendingMu  sync.RWMutex

	// Connection state
	connected bool
	mu        sync.RWMutex

	// Reader goroutines
	readerDone chan struct{}
	stderrDone chan struct{}
}

// jsonrpcRequest represents a JSON-RPC 2.0 request
type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonrpcResponse represents a JSON-RPC 2.0 response
type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

// jsonrpcError represents a JSON-RPC error
type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// newStdioTransport creates a new stdio transport
func newStdioTransport(config *ClientConfig) (Transport, error) {
	if config.Command == "" {
		return nil, fmt.Errorf("command is required for stdio transport")
	}

	return &stdioTransport{
		config:     config,
		pending:    make(map[int64]chan *jsonrpcResponse),
		readerDone: make(chan struct{}),
		stderrDone: make(chan struct{}),
	}, nil
}

// Connect establishes the connection
func (t *stdioTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return fmt.Errorf("already connected")
	}

	// Create command
	t.cmd = exec.CommandContext(ctx, t.config.Command, t.config.Args...)

	// Set environment variables
	if len(t.config.Env) > 0 {
		env := os.Environ()
		for k, v := range t.config.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		t.cmd.Env = env
	}

	// Set working directory
	if t.config.WorkingDir != "" {
		t.cmd.Dir = t.config.WorkingDir
	}

	// Set up pipes
	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	t.stdin = stdin

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	t.stdout = stdout

	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	t.stderr = stderr

	// Start the command
	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Start reader goroutine
	go t.readLoop()

	// Start stderr reader (for debugging)
	go t.readStderr()

	t.connected = true

	return nil
}

// SendRequest sends a JSON-RPC request
func (t *stdioTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	if !t.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	// Generate request ID
	id := t.requestID.Add(1)

	// Create response channel
	respChan := make(chan *jsonrpcResponse, 1)
	t.pendingMu.Lock()
	t.pending[id] = respChan
	t.pendingMu.Unlock()

	// Clean up on exit
	defer func() {
		t.pendingMu.Lock()
		delete(t.pending, id)
		t.pendingMu.Unlock()
		close(respChan)
	}()

	// Create request
	req := &jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Send request
	if err := t.sendMessage(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, fmt.Errorf("JSON-RPC error %d: %s", resp.Error.Code, resp.Error.Message)
		}

		// Parse result based on method
		return t.parseResult(method, resp.Result)
	}
}

// Close closes the connection
func (t *stdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	// Close stdin to signal end
	if t.stdin != nil {
		t.stdin.Close()
	}

	// Wait for command to finish
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
		t.cmd.Wait()
	}

	// Wait for reader goroutines to finish
	<-t.readerDone
	<-t.stderrDone

	t.connected = false

	return nil
}

// IsConnected returns connection status
func (t *stdioTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

// sendMessage sends a message over stdio
func (t *stdioTransport) sendMessage(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Add newline delimiter
	data = append(data, '\n')

	if _, err := t.stdin.Write(data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// readLoop reads messages from stdout
func (t *stdioTransport) readLoop() {
	defer close(t.readerDone)

	scanner := bufio.NewScanner(t.stdout)
	// Set max buffer size for large responses
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Parse JSON-RPC response
		var resp jsonrpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			// Log error but continue
			continue
		}

		// Deliver response to waiting request
		t.pendingMu.RLock()
		respChan, exists := t.pending[resp.ID]
		t.pendingMu.RUnlock()

		if exists {
			select {
			case respChan <- &resp:
			default:
				// Channel full, skip
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		// Log error
	}
}

// readStderr reads and logs stderr output
func (t *stdioTransport) readStderr() {
	defer close(t.stderrDone)

	scanner := bufio.NewScanner(t.stderr)
	for scanner.Scan() {
		// In production, you might want to log this
		// For now, we'll just ignore it
		_ = scanner.Text()
	}
}

// parseResult parses the result based on the method
func (t *stdioTransport) parseResult(method string, result json.RawMessage) (interface{}, error) {
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

// Helper to convert []map[string]interface{} to []interface{}
func convertToInterfaceSlice(maps []map[string]interface{}) []interface{} {
	result := make([]interface{}, len(maps))
	for i, m := range maps {
		result[i] = m
	}
	return result
}
