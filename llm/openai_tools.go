package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// OpenAIToolUseProvider extends OpenAIProvider with native function calling support.
// OpenAI calls this "function calling" but it serves the same purpose as tool use
// in other providers (Anthropic, Google, etc.).
type OpenAIToolUseProvider struct {
	*OpenAIProvider
}

// Ensure OpenAIToolUseProvider implements ToolUseProvider
var _ ToolUseProvider = (*OpenAIToolUseProvider)(nil)

// NewOpenAIWithTools creates a new OpenAI provider with function calling support
func NewOpenAIWithTools(apiKey string) *OpenAIToolUseProvider {
	return &OpenAIToolUseProvider{
		OpenAIProvider: NewOpenAI(apiKey),
	}
}

// SupportsToolUse returns true since this provider supports native function calling
func (p *OpenAIToolUseProvider) SupportsToolUse() bool {
	return true
}

// GenerateWithTools generates a response that may include function calls.
// This implements the provider-agnostic ToolUseProvider interface using
// OpenAI's native function calling capability.
func (p *OpenAIToolUseProvider) GenerateWithTools(ctx context.Context, req *ToolUseRequest) (*ToolUseResponse, error) {
	// Convert messages
	messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages)+1)

	// Add system message if present
	if req.System != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: req.System,
		})
	}

	for _, msg := range req.Messages {
		// Skip system messages (handled above)
		if msg.Role == "system" {
			continue
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Convert tools to OpenAI function definitions
	tools := make([]openai.Tool, len(req.Tools))
	for i, tool := range req.Tools {
		tools[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
	}

	// Build request
	chatReq := openai.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: float32(req.Temperature),
		MaxTokens:   req.MaxTokens,
		Tools:       tools,
	}

	// Convert tool choice
	if req.ToolChoice != nil {
		switch req.ToolChoice.Type {
		case "auto":
			chatReq.ToolChoice = "auto"
		case "none":
			chatReq.ToolChoice = "none"
		case "any":
			// OpenAI uses "required" for this
			chatReq.ToolChoice = "required"
		case "tool":
			// Specific tool
			chatReq.ToolChoice = openai.ToolChoice{
				Type: openai.ToolTypeFunction,
				Function: openai.ToolFunction{
					Name: req.ToolChoice.Name,
				},
			}
		}
	}

	// Make API call
	resp, err := p.client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("openai function call error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from OpenAI")
	}

	choice := resp.Choices[0]

	// Build response
	result := &ToolUseResponse{
		Content:    choice.Message.Content,
		StopReason: string(choice.FinishReason),
		TokensUsed: resp.Usage.TotalTokens,
		Model:      resp.Model,
	}

	// Extract tool calls
	for _, toolCall := range choice.Message.ToolCalls {
		if toolCall.Type == openai.ToolTypeFunction {
			// Parse function arguments
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
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

// ContinueWithToolResults continues a conversation after function execution.
// This sends the function results back to OpenAI to get the final response.
func (p *OpenAIToolUseProvider) ContinueWithToolResults(
	ctx context.Context,
	originalReq *ToolUseRequest,
	assistantResponse *ToolUseResponse,
	toolResults []ToolResultMessage,
) (*ToolUseResponse, error) {
	// Build messages with conversation history
	messages := make([]openai.ChatCompletionMessage, 0)

	// Add system message if present
	if originalReq.System != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: originalReq.System,
		})
	}

	// Add original messages
	for _, msg := range originalReq.Messages {
		if msg.Role == "system" {
			continue
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Add assistant response with tool calls
	assistantMsg := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: assistantResponse.Content,
	}
	for _, tu := range assistantResponse.ToolUse {
		argsBytes, _ := json.Marshal(tu.Input)
		assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, openai.ToolCall{
			ID:   tu.ID,
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
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

		messages = append(messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    contentStr,
			ToolCallID: result.ToolUseID,
		})
	}

	// Convert tools
	tools := make([]openai.Tool, len(originalReq.Tools))
	for i, tool := range originalReq.Tools {
		tools[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
	}

	// Build request
	chatReq := openai.ChatCompletionRequest{
		Model:       originalReq.Model,
		Messages:    messages,
		Temperature: float32(originalReq.Temperature),
		MaxTokens:   originalReq.MaxTokens,
		Tools:       tools,
	}

	// Make API call
	resp, err := p.client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("openai continuation error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from OpenAI")
	}

	choice := resp.Choices[0]

	// Build response
	result := &ToolUseResponse{
		Content:    choice.Message.Content,
		StopReason: string(choice.FinishReason),
		TokensUsed: resp.Usage.TotalTokens,
		Model:      resp.Model,
	}

	// Extract any additional tool calls
	for _, toolCall := range choice.Message.ToolCalls {
		if toolCall.Type == openai.ToolTypeFunction {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
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

// OpenAIToolUseConversation helps manage multi-turn function calling conversations
type OpenAIToolUseConversation struct {
	provider    *OpenAIToolUseProvider
	request     *ToolUseRequest
	history     []openai.ChatCompletionMessage
	toolResults []ToolResultMessage
}

// NewOpenAIToolUseConversation creates a new function calling conversation
func NewOpenAIToolUseConversation(provider *OpenAIToolUseProvider, req *ToolUseRequest) *OpenAIToolUseConversation {
	conv := &OpenAIToolUseConversation{
		provider: provider,
		request:  req,
		history:  make([]openai.ChatCompletionMessage, 0),
	}

	// Add system message if present
	if req.System != "" {
		conv.history = append(conv.history, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: req.System,
		})
	}

	return conv
}

// Send sends a message and returns the response
func (c *OpenAIToolUseConversation) Send(ctx context.Context, content string) (*ToolUseResponse, error) {
	c.history = append(c.history, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
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
		if msg.Role == openai.ChatMessageRoleSystem {
			continue
		}
		req.Messages = append(req.Messages, Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	resp, err := c.provider.GenerateWithTools(ctx, req)
	if err != nil {
		return nil, err
	}

	// Add assistant response to history
	c.history = append(c.history, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: resp.Content,
	})

	return resp, nil
}

// AddToolResult adds a tool result to be sent with the next message
func (c *OpenAIToolUseConversation) AddToolResult(toolUseID string, content interface{}, isError bool) {
	c.toolResults = append(c.toolResults, ToolResultMessage{
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	})
}

// GetHistory returns the conversation history as generic Messages
func (c *OpenAIToolUseConversation) GetHistory() []Message {
	messages := make([]Message, 0, len(c.history))
	for _, msg := range c.history {
		messages = append(messages, Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	return messages
}

// Clear clears the conversation history
func (c *OpenAIToolUseConversation) Clear() {
	c.history = nil
	c.toolResults = nil

	// Re-add system message if present
	if c.request.System != "" {
		c.history = append(c.history, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: c.request.System,
		})
	}
}
