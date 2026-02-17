package a2a

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client is an A2A protocol client for communicating with A2A agents
type Client struct {
	httpClient *http.Client
	baseURL    string
	agentCard  *AgentCard
	config     ClientConfig
	mu         sync.RWMutex
}

// ClientConfig configures the A2A client
type ClientConfig struct {
	// Timeout for requests
	Timeout time.Duration

	// Headers to include in requests
	Headers map[string]string

	// BearerToken for authentication
	BearerToken string

	// RetryAttempts for failed requests
	RetryAttempts int

	// RetryDelay between retries
	RetryDelay time.Duration

	// FetchAgentCard determines if the agent card should be fetched on connect
	FetchAgentCard bool
}

// DefaultClientConfig returns default client configuration
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Timeout:        30 * time.Second,
		Headers:        make(map[string]string),
		RetryAttempts:  3,
		RetryDelay:     time.Second,
		FetchAgentCard: true,
	}
}

// NewClient creates a new A2A client
func NewClient(baseURL string, config ClientConfig) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		baseURL: baseURL,
		config:  config,
	}
}

// Connect establishes connection and fetches agent card
func (c *Client) Connect(ctx context.Context) error {
	if !c.config.FetchAgentCard {
		return nil
	}

	card, err := c.GetAgentCard(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch agent card: %w", err)
	}

	c.mu.Lock()
	c.agentCard = card
	c.mu.Unlock()

	return nil
}

// GetAgentCard fetches the agent card from the server
func (c *Client) GetAgentCard(ctx context.Context) (*AgentCard, error) {
	url := c.baseURL + "/.well-known/agent.json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	c.addHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch agent card: status %d", resp.StatusCode)
	}

	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, err
	}

	return &card, nil
}

// AgentCard returns the cached agent card
func (c *Client) AgentCard() *AgentCard {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agentCard
}

// SendTask sends a task and waits for completion
func (c *Client) SendTask(ctx context.Context, params TaskSendParams) (*Task, error) {
	return c.call(ctx, MethodTaskSend, params)
}

// SendTaskSubscribe sends a task with subscription for updates
func (c *Client) SendTaskSubscribe(ctx context.Context, params TaskSendParams) (*Task, error) {
	return c.call(ctx, MethodTaskSendSubscribe, params)
}

// SendTaskStream sends a task and returns a channel of updates
func (c *Client) SendTaskStream(ctx context.Context, params TaskSendParams) (<-chan TaskUpdate, error) {
	rpcReq, err := NewJSONRPCRequest(1, MethodTaskSendSubscribe, params)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	c.addHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Check if we got SSE response
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		defer resp.Body.Close()
		// Fall back to regular response
		var rpcResp JSONRPCResponse
		if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
			return nil, err
		}
		if rpcResp.Error != nil {
			return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
		}

		// Convert result to task and return single update
		taskData, _ := json.Marshal(rpcResp.Result)
		var task Task
		json.Unmarshal(taskData, &task)

		updates := make(chan TaskUpdate, 1)
		updates <- NewStatusUpdate(task.Status.State, task.Status.Message)
		close(updates)
		return updates, nil
	}

	// Return SSE stream
	return c.readSSEStream(ctx, resp.Body), nil
}

// GetTask retrieves a task by ID
func (c *Client) GetTask(ctx context.Context, taskID string, historyLength int) (*Task, error) {
	params := TaskQueryParams{
		ID:            taskID,
		HistoryLength: historyLength,
	}
	return c.call(ctx, MethodTaskGet, params)
}

// CancelTask cancels a task
func (c *Client) CancelTask(ctx context.Context, taskID string) (*Task, error) {
	params := TaskCancelParams{
		ID: taskID,
	}
	return c.call(ctx, MethodTaskCancel, params)
}

// Resubscribe resubscribes to task updates via SSE
func (c *Client) Resubscribe(ctx context.Context, taskID string) (<-chan TaskUpdate, error) {
	params := TaskQueryParams{
		ID: taskID,
	}

	rpcReq, err := NewJSONRPCRequest(1, MethodTaskResubscribe, params)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	c.addHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		resp.Body.Close()
		return nil, fmt.Errorf("server does not support SSE")
	}

	return c.readSSEStream(ctx, resp.Body), nil
}

// call makes a JSON-RPC call
func (c *Client) call(ctx context.Context, method string, params any) (*Task, error) {
	rpcReq, err := NewJSONRPCRequest(1, method, params)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= c.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.config.RetryDelay):
			}
		}

		task, err := c.doCall(ctx, body)
		if err == nil {
			return task, nil
		}
		lastErr = err
	}

	return nil, lastErr
}

// doCall performs a single JSON-RPC call
func (c *Client) doCall(ctx context.Context, body []byte) (*Task, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	c.addHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	// Convert result to Task
	taskData, err := json.Marshal(rpcResp.Result)
	if err != nil {
		return nil, err
	}

	var task Task
	if err := json.Unmarshal(taskData, &task); err != nil {
		return nil, err
	}

	return &task, nil
}

// addHeaders adds configured headers to a request
func (c *Client) addHeaders(req *http.Request) {
	for k, v := range c.config.Headers {
		req.Header.Set(k, v)
	}

	if c.config.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.BearerToken)
	}
}

// readSSEStream reads SSE events and converts them to TaskUpdates
func (c *Client) readSSEStream(ctx context.Context, body io.ReadCloser) <-chan TaskUpdate {
	updates := make(chan TaskUpdate, 100)

	go func() {
		defer close(updates)
		defer body.Close()

		reader := bufio.NewReader(body)
		var eventType, data string

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					updates <- NewErrorUpdate(err)
				}
				return
			}

			line = line[:len(line)-1] // Remove newline
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1] // Remove carriage return
			}

			// Empty line signals end of event
			if line == "" {
				if eventType != "" && data != "" {
					update := c.parseSSEEvent(SSEEventType(eventType), data)
					if update != nil {
						updates <- *update

						// Check for done event
						if SSEEventType(eventType) == SSEEventDone {
							return
						}
					}
				}
				eventType = ""
				data = ""
				continue
			}

			// Skip comments
			if line[0] == ':' {
				continue
			}

			// Parse field
			colonIdx := -1
			for i, ch := range line {
				if ch == ':' {
					colonIdx = i
					break
				}
			}

			if colonIdx == -1 {
				continue
			}

			field := line[:colonIdx]
			value := line[colonIdx+1:]
			if len(value) > 0 && value[0] == ' ' {
				value = value[1:]
			}

			switch field {
			case "event":
				eventType = value
			case "data":
				data = value
			}
		}
	}()

	return updates
}

// parseSSEEvent converts an SSE event to a TaskUpdate
func (c *Client) parseSSEEvent(eventType SSEEventType, data string) *TaskUpdate {
	switch eventType {
	case SSEEventTaskStatus:
		var statusUpdate TaskStatusUpdate
		if err := json.Unmarshal([]byte(data), &statusUpdate); err != nil {
			return &TaskUpdate{Type: TaskUpdateTypeError, Error: err}
		}
		return &TaskUpdate{
			Type:   TaskUpdateTypeStatus,
			Status: &statusUpdate.Status,
		}

	case SSEEventTaskArtifact:
		var artifactUpdate TaskArtifactUpdate
		if err := json.Unmarshal([]byte(data), &artifactUpdate); err != nil {
			return &TaskUpdate{Type: TaskUpdateTypeError, Error: err}
		}
		return &TaskUpdate{
			Type:     TaskUpdateTypeArtifact,
			Artifact: &artifactUpdate.Artifact,
		}

	case SSEEventTaskMessage:
		var message Message
		if err := json.Unmarshal([]byte(data), &message); err != nil {
			return &TaskUpdate{Type: TaskUpdateTypeError, Error: err}
		}
		return &TaskUpdate{
			Type:    TaskUpdateTypeMessage,
			Message: &message,
		}

	case SSEEventError:
		var errData map[string]string
		if err := json.Unmarshal([]byte(data), &errData); err != nil {
			return &TaskUpdate{Type: TaskUpdateTypeError, Error: fmt.Errorf("%s", data)}
		}
		return &TaskUpdate{
			Type:  TaskUpdateTypeError,
			Error: fmt.Errorf("%s", errData["error"]),
		}

	case SSEEventDone:
		return nil

	default:
		return nil
	}
}

// MultiAgentClient manages connections to multiple A2A agents
type MultiAgentClient struct {
	mu      sync.RWMutex
	clients map[string]*Client
	config  ClientConfig
}

// NewMultiAgentClient creates a new multi-agent client
func NewMultiAgentClient(config ClientConfig) *MultiAgentClient {
	return &MultiAgentClient{
		clients: make(map[string]*Client),
		config:  config,
	}
}

// AddAgent adds an agent by URL
func (m *MultiAgentClient) AddAgent(ctx context.Context, name, url string) error {
	client := NewClient(url, m.config)
	if err := client.Connect(ctx); err != nil {
		return err
	}

	m.mu.Lock()
	m.clients[name] = client
	m.mu.Unlock()

	return nil
}

// RemoveAgent removes an agent
func (m *MultiAgentClient) RemoveAgent(name string) {
	m.mu.Lock()
	delete(m.clients, name)
	m.mu.Unlock()
}

// GetAgent retrieves a client by name
func (m *MultiAgentClient) GetAgent(name string) (*Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, ok := m.clients[name]
	return client, ok
}

// ListAgents returns all registered agent names
func (m *MultiAgentClient) ListAgents() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	return names
}

// SendToAgent sends a task to a specific agent
func (m *MultiAgentClient) SendToAgent(ctx context.Context, agentName string, params TaskSendParams) (*Task, error) {
	client, ok := m.GetAgent(agentName)
	if !ok {
		return nil, fmt.Errorf("agent %s not found", agentName)
	}
	return client.SendTask(ctx, params)
}

// SendToAgentStream sends a task with streaming to a specific agent
func (m *MultiAgentClient) SendToAgentStream(ctx context.Context, agentName string, params TaskSendParams) (<-chan TaskUpdate, error) {
	client, ok := m.GetAgent(agentName)
	if !ok {
		return nil, fmt.Errorf("agent %s not found", agentName)
	}
	return client.SendTaskStream(ctx, params)
}

// DiscoverAgents discovers agents from a list of URLs
func (m *MultiAgentClient) DiscoverAgents(ctx context.Context, urls []string) ([]AgentCard, error) {
	var cards []AgentCard
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			card, err := FetchAgentCardWithTimeout(url, 10*time.Second)
			if err != nil {
				return // Skip failed fetches
			}

			mu.Lock()
			cards = append(cards, *card)
			mu.Unlock()
		}(url)
	}

	wg.Wait()
	return cards, nil
}
