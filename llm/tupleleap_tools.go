package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// TupleLeapToolUseProvider extends TupleLeapProvider with native tool/function calling support.
// This implements the provider-agnostic ToolUseProvider interface for TupleLeap AI.
type TupleLeapToolUseProvider struct {
	*TupleLeapProvider
}

// Ensure TupleLeapToolUseProvider implements ToolUseProvider
var _ ToolUseProvider = (*TupleLeapToolUseProvider)(nil)

// NewTupleLeapWithTools creates a new TupleLeap provider with tool use support
func NewTupleLeapWithTools(apiKey string) *TupleLeapToolUseProvider {
	return &TupleLeapToolUseProvider{
		TupleLeapProvider: NewTupleLeap(apiKey),
	}
}

// NewTupleLeapToolUseProviderWithBaseURL creates a provider with custom base URL
func NewTupleLeapToolUseProviderWithBaseURL(apiKey, baseURL string) *TupleLeapToolUseProvider {
	return &TupleLeapToolUseProvider{
		TupleLeapProvider: NewTupleLeapWithBaseURL(apiKey, baseURL),
	}
}

// NewTupleLeapToolUseProviderWithClient creates a provider with a custom HTTP client
func NewTupleLeapToolUseProviderWithClient(apiKey string, client *http.Client) *TupleLeapToolUseProvider {
	provider := NewTupleLeapWithTools(apiKey)
	provider.httpClient = client
	return provider
}

// SupportsToolUse returns true since this provider supports native tool use
func (p *TupleLeapToolUseProvider) SupportsToolUse() bool {
	return true
}

// TupleLeap tool use request/response structures
type tupleLeapToolUseRequest struct {
	Model       string               `json:"model"`
	Messages    []tupleLeapToolMsg   `json:"messages"`
	MaxTokens   int                  `json:"max_tokens"`
	Temperature float64              `json:"temperature,omitempty"`
	Tools       []tupleLeapTool      `json:"tools,omitempty"`
	ToolChoice  interface{}          `json:"tool_choice,omitempty"`
	Stream      bool                 `json:"stream"`
}

// tupleLeapToolMsg represents a message in the TupleLeap tool use format
type tupleLeapToolMsg struct {
	Role       string        `json:"role"`
	Content    interface{}   `json:"content"` // Can be string or array of content blocks
	ToolCalls  []tupleLeapToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

// tupleLeapTool represents a tool definition for TupleLeap
type tupleLeapTool struct {
	Type     string                  `json:"type"` // Always "function"
	Function tupleLeapFunctionDef    `json:"function"`
}

// tupleLeapFunctionDef represents a function definition
type tupleLeapFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// tupleLeapToolCall represents a tool call in the response
type tupleLeapToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // Always "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

// tupleLeapToolChoice represents tool choice configuration
type tupleLeapToolChoice struct {
	Type     string                   `json:"type,omitempty"` // "function" when specifying a tool
	Function *tupleLeapToolFunction   `json:"function,omitempty"`
}

// tupleLeapToolFunction represents function choice
type tupleLeapToolFunction struct {
	Name string `json:"name"`
}

// tupleLeapToolUseResponse is the response format from TupleLeap's tool use API
type tupleLeapToolUseResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string              `json:"role"`
			Content   string              `json:"content"`
			ToolCalls []tupleLeapToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// GenerateWithTools generates a response that may include tool calls.
// This implements the provider-agnostic ToolUseProvider interface using
// TupleLeap's native function calling capability.
func (p *TupleLeapToolUseProvider) GenerateWithTools(ctx context.Context, req *ToolUseRequest) (*ToolUseResponse, error) {
	// Build the TupleLeap request
	tupleLeapReq := tupleLeapToolUseRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      false,
	}

	// Convert messages
	messages := make([]tupleLeapToolMsg, 0, len(req.Messages)+1)

	// Add system message if present
	if req.System != "" {
		messages = append(messages, tupleLeapToolMsg{
			Role:    "system",
			Content: req.System,
		})
	}

	for _, msg := range req.Messages {
		// Skip system messages (handled above)
		if msg.Role == "system" {
			continue
		}
		messages = append(messages, tupleLeapToolMsg{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	tupleLeapReq.Messages = messages

	// Convert tools to TupleLeap function definitions
	for _, tool := range req.Tools {
		tupleLeapReq.Tools = append(tupleLeapReq.Tools, tupleLeapTool{
			Type: "function",
			Function: tupleLeapFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}

	// Convert tool choice
	if req.ToolChoice != nil {
		switch req.ToolChoice.Type {
		case "auto":
			tupleLeapReq.ToolChoice = "auto"
		case "none":
			tupleLeapReq.ToolChoice = "none"
		case "any":
			// TupleLeap uses "required" for this (same as OpenAI)
			tupleLeapReq.ToolChoice = "required"
		case "tool":
			// Specific tool
			tupleLeapReq.ToolChoice = tupleLeapToolChoice{
				Type: "function",
				Function: &tupleLeapToolFunction{
					Name: req.ToolChoice.Name,
				},
			}
		}
	}

	// Make API call
	resp, err := p.callToolUseAPI(ctx, tupleLeapReq)
	if err != nil {
		return nil, err
	}

	// Parse response
	return p.parseToolUseResponse(resp)
}

// callToolUseAPI makes the API call to TupleLeap
func (p *TupleLeapToolUseProvider) callToolUseAPI(ctx context.Context, req tupleLeapToolUseRequest) (*tupleLeapToolUseResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tupleleap API error (status %d): %s", httpResp.StatusCode, string(body))
	}

	var resp tupleLeapToolUseResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// parseToolUseResponse parses the TupleLeap response into our provider-agnostic format
func (p *TupleLeapToolUseProvider) parseToolUseResponse(resp *tupleLeapToolUseResponse) (*ToolUseResponse, error) {
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from TupleLeap")
	}

	choice := resp.Choices[0]

	result := &ToolUseResponse{
		Content:    choice.Message.Content,
		StopReason: choice.FinishReason,
		TokensUsed: resp.Usage.TotalTokens,
		Model:      resp.Model,
	}

	// Extract tool calls
	for _, toolCall := range choice.Message.ToolCalls {
		if toolCall.Type == "function" {
			// Parse function arguments from JSON string
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				// If parsing fails, store raw arguments
				args = map[string]interface{}{
					"raw": toolCall.Function.Arguments,
				}
			}

			result.ToolUse = append(result.ToolUse, ToolUse{
				ID:    toolCall.ID,
				Name:  toolCall.Function.Name,
				Input: args,
			})
		}
	}

	return result, nil
}

// ContinueWithToolResults continues a conversation after tool execution.
// This sends the tool results back to TupleLeap to get the final response.
func (p *TupleLeapToolUseProvider) ContinueWithToolResults(
	ctx context.Context,
	originalReq *ToolUseRequest,
	assistantResponse *ToolUseResponse,
	toolResults []ToolResultMessage,
) (*ToolUseResponse, error) {
	// Build messages with conversation history
	messages := make([]tupleLeapToolMsg, 0)

	// Add system message if present
	if originalReq.System != "" {
		messages = append(messages, tupleLeapToolMsg{
			Role:    "system",
			Content: originalReq.System,
		})
	}

	// Add original messages
	for _, msg := range originalReq.Messages {
		if msg.Role == "system" {
			continue
		}
		messages = append(messages, tupleLeapToolMsg{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Add assistant response with tool calls
	assistantMsg := tupleLeapToolMsg{
		Role:    "assistant",
		Content: assistantResponse.Content,
	}
	for _, tu := range assistantResponse.ToolUse {
		argsBytes, _ := json.Marshal(tu.Input)
		assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, tupleLeapToolCall{
			ID:   tu.ID,
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      tu.Name,
				Arguments: string(argsBytes),
			},
		})
	}
	messages = append(messages, assistantMsg)

	// Add tool results as tool messages
	for _, result := range toolResults {
		contentStr := ""
		switch v := result.Content.(type) {
		case string:
			contentStr = v
		default:
			contentBytes, _ := json.Marshal(v)
			contentStr = string(contentBytes)
		}

		messages = append(messages, tupleLeapToolMsg{
			Role:       "tool",
			Content:    contentStr,
			ToolCallID: result.ToolUseID,
		})
	}

	// Convert tools
	tools := make([]tupleLeapTool, len(originalReq.Tools))
	for i, tool := range originalReq.Tools {
		tools[i] = tupleLeapTool{
			Type: "function",
			Function: tupleLeapFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
	}

	// Build request
	tupleLeapReq := tupleLeapToolUseRequest{
		Model:       originalReq.Model,
		Messages:    messages,
		MaxTokens:   originalReq.MaxTokens,
		Temperature: originalReq.Temperature,
		Tools:       tools,
		Stream:      false,
	}

	// Make API call
	resp, err := p.callToolUseAPI(ctx, tupleLeapReq)
	if err != nil {
		return nil, fmt.Errorf("tupleleap continuation error: %w", err)
	}

	return p.parseToolUseResponse(resp)
}

// TupleLeapToolUseConversation helps manage multi-turn tool use conversations
type TupleLeapToolUseConversation struct {
	provider    *TupleLeapToolUseProvider
	request     *ToolUseRequest
	history     []tupleLeapToolMsg
	toolResults []ToolResultMessage
}

// NewTupleLeapToolUseConversation creates a new tool use conversation
func NewTupleLeapToolUseConversation(provider *TupleLeapToolUseProvider, req *ToolUseRequest) *TupleLeapToolUseConversation {
	conv := &TupleLeapToolUseConversation{
		provider: provider,
		request:  req,
		history:  make([]tupleLeapToolMsg, 0),
	}

	// Add system message if present
	if req.System != "" {
		conv.history = append(conv.history, tupleLeapToolMsg{
			Role:    "system",
			Content: req.System,
		})
	}

	return conv
}

// Send sends a message and returns the response
func (c *TupleLeapToolUseConversation) Send(ctx context.Context, content string) (*ToolUseResponse, error) {
	c.history = append(c.history, tupleLeapToolMsg{
		Role:    "user",
		Content: content,
	})

	req := &ToolUseRequest{
		Model:       c.request.Model,
		MaxTokens:   c.request.MaxTokens,
		Temperature: c.request.Temperature,
		System:      c.request.System,
		Tools:       c.request.Tools,
		ToolChoice:  c.request.ToolChoice,
	}

	// Convert history to Message format
	for _, msg := range c.history {
		if msg.Role == "system" {
			continue
		}
		contentStr := ""
		switch v := msg.Content.(type) {
		case string:
			contentStr = v
		default:
			contentBytes, _ := json.Marshal(v)
			contentStr = string(contentBytes)
		}
		req.Messages = append(req.Messages, Message{
			Role:    msg.Role,
			Content: contentStr,
		})
	}

	resp, err := c.provider.GenerateWithTools(ctx, req)
	if err != nil {
		return nil, err
	}

	// Add assistant response to history
	c.history = append(c.history, tupleLeapToolMsg{
		Role:    "assistant",
		Content: resp.Content,
	})

	return resp, nil
}

// AddToolResult adds a tool result to be sent with the next message
func (c *TupleLeapToolUseConversation) AddToolResult(toolUseID string, content interface{}, isError bool) {
	c.toolResults = append(c.toolResults, ToolResultMessage{
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	})
}

// SendToolResults sends tool results and continues the conversation
func (c *TupleLeapToolUseConversation) SendToolResults(ctx context.Context) (*ToolUseResponse, error) {
	if len(c.toolResults) == 0 {
		return nil, fmt.Errorf("no tool results to send")
	}

	// Add tool results to history
	for _, result := range c.toolResults {
		contentStr := ""
		switch v := result.Content.(type) {
		case string:
			contentStr = v
		default:
			contentBytes, _ := json.Marshal(v)
			contentStr = string(contentBytes)
		}

		c.history = append(c.history, tupleLeapToolMsg{
			Role:       "tool",
			Content:    contentStr,
			ToolCallID: result.ToolUseID,
		})
	}

	// Clear tool results
	c.toolResults = nil

	req := &ToolUseRequest{
		Model:       c.request.Model,
		MaxTokens:   c.request.MaxTokens,
		Temperature: c.request.Temperature,
		System:      c.request.System,
		Tools:       c.request.Tools,
		ToolChoice:  c.request.ToolChoice,
	}

	// Convert history to Message format
	for _, msg := range c.history {
		if msg.Role == "system" {
			continue
		}
		contentStr := ""
		switch v := msg.Content.(type) {
		case string:
			contentStr = v
		default:
			contentBytes, _ := json.Marshal(v)
			contentStr = string(contentBytes)
		}
		req.Messages = append(req.Messages, Message{
			Role:    msg.Role,
			Content: contentStr,
		})
	}

	resp, err := c.provider.GenerateWithTools(ctx, req)
	if err != nil {
		return nil, err
	}

	c.history = append(c.history, tupleLeapToolMsg{
		Role:    "assistant",
		Content: resp.Content,
	})

	return resp, nil
}

// GetHistory returns the conversation history as generic Messages
func (c *TupleLeapToolUseConversation) GetHistory() []Message {
	messages := make([]Message, 0, len(c.history))
	for _, msg := range c.history {
		contentStr := ""
		switch v := msg.Content.(type) {
		case string:
			contentStr = v
		default:
			contentBytes, _ := json.Marshal(v)
			contentStr = string(contentBytes)
		}
		messages = append(messages, Message{
			Role:    msg.Role,
			Content: contentStr,
		})
	}
	return messages
}

// Clear clears the conversation history
func (c *TupleLeapToolUseConversation) Clear() {
	c.history = nil
	c.toolResults = nil

	// Re-add system message if present
	if c.request.System != "" {
		c.history = append(c.history, tupleLeapToolMsg{
			Role:    "system",
			Content: c.request.System,
		})
	}
}
