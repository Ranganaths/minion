package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AnthropicToolUseProvider extends AnthropicProvider with native tool use support
type AnthropicToolUseProvider struct {
	*AnthropicProvider
}

// Ensure AnthropicToolUseProvider implements ToolUseProvider
var _ ToolUseProvider = (*AnthropicToolUseProvider)(nil)

// NewAnthropicWithTools creates a new Anthropic provider with tool use support
func NewAnthropicWithTools(apiKey string) *AnthropicToolUseProvider {
	return &AnthropicToolUseProvider{
		AnthropicProvider: NewAnthropic(apiKey),
	}
}

// NewAnthropicToolUseProviderWithClient creates a provider with a custom HTTP client
func NewAnthropicToolUseProviderWithClient(apiKey string, client *http.Client) *AnthropicToolUseProvider {
	provider := NewAnthropicWithTools(apiKey)
	provider.httpClient = client
	return provider
}

// SupportsToolUse returns true since this provider supports native tool use
func (p *AnthropicToolUseProvider) SupportsToolUse() bool {
	return true
}

// anthropicToolUseRequest is the request format for Anthropic's tool use API
type anthropicToolUseRequest struct {
	Model       string                `json:"model"`
	Messages    []anthropicToolMsg    `json:"messages"`
	MaxTokens   int                   `json:"max_tokens"`
	Temperature float64               `json:"temperature,omitempty"`
	System      string                `json:"system,omitempty"`
	Tools       []anthropicTool       `json:"tools,omitempty"`
	ToolChoice  *anthropicToolChoice  `json:"tool_choice,omitempty"`
}

// anthropicToolMsg represents a message in the Anthropic tool use format
type anthropicToolMsg struct {
	Role    string        `json:"role"`
	Content []interface{} `json:"content"` // Can contain text blocks, tool_use blocks, or tool_result blocks
}

// anthropicTextBlock represents a text content block
type anthropicTextBlock struct {
	Type string `json:"type"` // Always "text"
	Text string `json:"text"`
}

// anthropicToolUseBlock represents a tool use content block
type anthropicToolUseBlock struct {
	Type  string                 `json:"type"` // Always "tool_use"
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// anthropicToolResultBlock represents a tool result content block
type anthropicToolResultBlock struct {
	Type      string      `json:"type"` // Always "tool_result"
	ToolUseID string      `json:"tool_use_id"`
	Content   interface{} `json:"content"`
	IsError   bool        `json:"is_error,omitempty"`
}

// anthropicTool represents a tool definition for Anthropic
type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// anthropicToolChoice represents tool choice configuration
type anthropicToolChoice struct {
	Type string `json:"type"`          // "auto", "any", "tool", "none"
	Name string `json:"name,omitempty"` // Required when type is "tool"
}

// anthropicToolUseResponse is the response format from Anthropic's tool use API
type anthropicToolUseResponse struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Role       string `json:"role"`
	Content    []json.RawMessage `json:"content"` // Can contain text or tool_use blocks
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// contentBlock is used to detect the type of content block
type contentBlock struct {
	Type string `json:"type"`
}

// GenerateWithTools generates a response that may include tool calls
func (p *AnthropicToolUseProvider) GenerateWithTools(ctx context.Context, req *ToolUseRequest) (*ToolUseResponse, error) {
	// Build the Anthropic request
	anthropicReq := anthropicToolUseRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		System:      req.System,
	}

	// Convert messages
	messages, err := p.convertMessages(req.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}
	anthropicReq.Messages = messages

	// Convert tools
	for _, tool := range req.Tools {
		anthropicReq.Tools = append(anthropicReq.Tools, anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}

	// Convert tool choice
	if req.ToolChoice != nil {
		anthropicReq.ToolChoice = &anthropicToolChoice{
			Type: req.ToolChoice.Type,
			Name: req.ToolChoice.Name,
		}
	}

	// Make API call
	resp, err := p.callToolUseAPI(ctx, anthropicReq)
	if err != nil {
		return nil, err
	}

	// Parse response content
	return p.parseToolUseResponse(resp)
}

// convertMessages converts generic messages to Anthropic format
func (p *AnthropicToolUseProvider) convertMessages(messages []Message) ([]anthropicToolMsg, error) {
	var result []anthropicToolMsg

	for _, msg := range messages {
		// Skip system messages (handled separately)
		if msg.Role == "system" {
			continue
		}

		// Simple text content
		content := []interface{}{
			anthropicTextBlock{
				Type: "text",
				Text: msg.Content,
			},
		}

		result = append(result, anthropicToolMsg{
			Role:    msg.Role,
			Content: content,
		})
	}

	return result, nil
}

// callToolUseAPI makes the API call to Anthropic
func (p *AnthropicToolUseProvider) callToolUseAPI(ctx context.Context, req anthropicToolUseRequest) (*anthropicToolUseResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

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
		return nil, fmt.Errorf("anthropic API error (status %d): %s", httpResp.StatusCode, string(body))
	}

	var resp anthropicToolUseResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// parseToolUseResponse parses the Anthropic response into our format
func (p *AnthropicToolUseProvider) parseToolUseResponse(resp *anthropicToolUseResponse) (*ToolUseResponse, error) {
	result := &ToolUseResponse{
		StopReason: resp.StopReason,
		TokensUsed: resp.Usage.InputTokens + resp.Usage.OutputTokens,
		Model:      resp.Model,
	}

	var textContent string

	for _, rawBlock := range resp.Content {
		// First, determine the block type
		var block contentBlock
		if err := json.Unmarshal(rawBlock, &block); err != nil {
			continue
		}

		switch block.Type {
		case "text":
			var textBlock anthropicTextBlock
			if err := json.Unmarshal(rawBlock, &textBlock); err != nil {
				continue
			}
			if textContent != "" {
				textContent += "\n"
			}
			textContent += textBlock.Text

		case "tool_use":
			var toolUseBlock anthropicToolUseBlock
			if err := json.Unmarshal(rawBlock, &toolUseBlock); err != nil {
				continue
			}
			result.ToolUse = append(result.ToolUse, ToolUse{
				ID:    toolUseBlock.ID,
				Name:  toolUseBlock.Name,
				Input: toolUseBlock.Input,
			})
		}
	}

	result.Content = textContent

	return result, nil
}

// CreateToolResultMessage creates a message containing tool results to send back
func (p *AnthropicToolUseProvider) CreateToolResultMessage(results []ToolResultMessage) anthropicToolMsg {
	var content []interface{}

	for _, result := range results {
		content = append(content, anthropicToolResultBlock{
			Type:      "tool_result",
			ToolUseID: result.ToolUseID,
			Content:   result.Content,
			IsError:   result.IsError,
		})
	}

	return anthropicToolMsg{
		Role:    "user",
		Content: content,
	}
}

// ContinueWithToolResults continues a conversation after tool execution
func (p *AnthropicToolUseProvider) ContinueWithToolResults(
	ctx context.Context,
	originalReq *ToolUseRequest,
	assistantResponse *ToolUseResponse,
	toolResults []ToolResultMessage,
) (*ToolUseResponse, error) {
	// Build new request with conversation history
	newReq := &ToolUseRequest{
		Model:       originalReq.Model,
		MaxTokens:   originalReq.MaxTokens,
		Temperature: originalReq.Temperature,
		System:      originalReq.System,
		Tools:       originalReq.Tools,
		ToolChoice:  originalReq.ToolChoice,
	}

	// Copy original messages
	newReq.Messages = make([]Message, len(originalReq.Messages))
	copy(newReq.Messages, originalReq.Messages)

	// Add assistant response with tool use
	assistantMsg := Message{
		Role:    "assistant",
		Content: assistantResponse.Content,
	}
	newReq.Messages = append(newReq.Messages, assistantMsg)

	// Add tool results as user message
	// Note: For proper tool result handling, we need to use the raw API format
	// This is a simplified version that concatenates tool results as text
	var resultContent string
	for _, result := range toolResults {
		if resultContent != "" {
			resultContent += "\n\n"
		}
		if result.IsError {
			resultContent += fmt.Sprintf("Error from tool %s: %v", result.ToolUseID, result.Content)
		} else {
			resultContent += fmt.Sprintf("Result from tool: %v", result.Content)
		}
	}

	newReq.Messages = append(newReq.Messages, Message{
		Role:    "user",
		Content: resultContent,
	})

	return p.GenerateWithTools(ctx, newReq)
}

// ToolUseConversation helps manage multi-turn tool use conversations
type ToolUseConversation struct {
	provider    *AnthropicToolUseProvider
	request     *ToolUseRequest
	history     []Message
	toolResults []ToolResultMessage
}

// NewToolUseConversation creates a new tool use conversation
func NewToolUseConversation(provider *AnthropicToolUseProvider, req *ToolUseRequest) *ToolUseConversation {
	return &ToolUseConversation{
		provider: provider,
		request:  req,
		history:  make([]Message, 0),
	}
}

// Send sends a message and returns the response
func (c *ToolUseConversation) Send(ctx context.Context, content string) (*ToolUseResponse, error) {
	c.history = append(c.history, Message{
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
		Messages:    c.history,
	}

	resp, err := c.provider.GenerateWithTools(ctx, req)
	if err != nil {
		return nil, err
	}

	// Add assistant response to history
	c.history = append(c.history, Message{
		Role:    "assistant",
		Content: resp.Content,
	})

	return resp, nil
}

// AddToolResult adds a tool result to be sent with the next message
func (c *ToolUseConversation) AddToolResult(toolUseID string, content interface{}, isError bool) {
	c.toolResults = append(c.toolResults, ToolResultMessage{
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	})
}

// SendToolResults sends tool results and continues the conversation
func (c *ToolUseConversation) SendToolResults(ctx context.Context) (*ToolUseResponse, error) {
	if len(c.toolResults) == 0 {
		return nil, fmt.Errorf("no tool results to send")
	}

	// Build tool results message
	var resultContent string
	for _, result := range c.toolResults {
		if resultContent != "" {
			resultContent += "\n\n"
		}
		resultContent += fmt.Sprintf("Result: %v", result.Content)
	}

	c.history = append(c.history, Message{
		Role:    "user",
		Content: resultContent,
	})

	// Clear tool results
	c.toolResults = nil

	req := &ToolUseRequest{
		Model:       c.request.Model,
		MaxTokens:   c.request.MaxTokens,
		Temperature: c.request.Temperature,
		System:      c.request.System,
		Tools:       c.request.Tools,
		ToolChoice:  c.request.ToolChoice,
		Messages:    c.history,
	}

	resp, err := c.provider.GenerateWithTools(ctx, req)
	if err != nil {
		return nil, err
	}

	c.history = append(c.history, Message{
		Role:    "assistant",
		Content: resp.Content,
	})

	return resp, nil
}

// GetHistory returns the conversation history
func (c *ToolUseConversation) GetHistory() []Message {
	return c.history
}

// Clear clears the conversation history
func (c *ToolUseConversation) Clear() {
	c.history = nil
	c.toolResults = nil
}
