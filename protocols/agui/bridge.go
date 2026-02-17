package agui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Ranganaths/minion/core"
	"github.com/Ranganaths/minion/models"
	"github.com/google/uuid"
)

// MinionBridge bridges the Minion framework with the AG-UI protocol
type MinionBridge struct {
	framework core.Framework
	agentID   string
	tools     []Tool
}

// NewMinionBridge creates a new bridge between Minion and AG-UI
func NewMinionBridge(framework core.Framework, agentID string) *MinionBridge {
	return &MinionBridge{
		framework: framework,
		agentID:   agentID,
	}
}

// WithTools adds available tools to the bridge
func (b *MinionBridge) WithTools(tools []Tool) *MinionBridge {
	b.tools = tools
	return b
}

// HandleRun implements Handler for AG-UI
func (b *MinionBridge) HandleRun(ctx context.Context, req RunRequest, emitter *EventEmitter) error {
	// Emit run started
	emitter.EmitRunStarted()

	// Convert AG-UI messages to Minion input
	input := b.requestToInput(req)

	// Start message streaming
	emitter.EmitStepStarted("processing")

	// Execute via Minion framework
	// Note: In a full implementation, we'd integrate with Minion's streaming
	// capabilities to emit token-by-token updates
	output, err := b.framework.Execute(ctx, b.agentID, input)

	emitter.EmitStepFinished("processing")

	if err != nil {
		emitter.EmitRunError(err.Error(), "EXECUTION_ERROR")
		return err
	}

	// Stream the response
	messageID := emitter.EmitTextMessageStart("", RoleAssistant)

	// Extract response from Result
	response := ""
	if output.Result != nil {
		response = fmt.Sprintf("%v", output.Result)
	}

	// For now, emit the entire response as one chunk
	// In a full implementation, this would be streamed token by token
	emitter.EmitTextMessageContent(messageID, response)
	emitter.EmitTextMessageEnd(messageID)

	// Handle any tool calls in the output
	if output.Metadata != nil {
		if toolCalls, ok := output.Metadata["tool_calls"].([]interface{}); ok {
			for _, tc := range toolCalls {
				if tcMap, ok := tc.(map[string]interface{}); ok {
					toolCallID := emitter.EmitToolCallStart(
						"",
						fmt.Sprintf("%v", tcMap["name"]),
						messageID,
					)
					if args, ok := tcMap["arguments"].(string); ok {
						emitter.EmitToolCallArgs(toolCallID, args)
					}
					if result, ok := tcMap["result"].(string); ok {
						emitter.EmitToolCallEnd(toolCallID, result)
					} else {
						emitter.EmitToolCallEnd(toolCallID, "")
					}
				}
			}
		}
	}

	// Emit run finished
	emitter.EmitRunFinished()

	return nil
}

// requestToInput converts AG-UI request to Minion input
func (b *MinionBridge) requestToInput(req RunRequest) *models.Input {
	// Find the last user message
	var lastUserMsg *Message
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == RoleUser {
			lastUserMsg = &req.Messages[i]
			break
		}
	}

	if lastUserMsg == nil {
		return &models.Input{Raw: "", Type: "text"}
	}

	// Build conversation context
	contextData := make(map[string]interface{})
	var history []map[string]string
	for _, msg := range req.Messages {
		if msg.ID == lastUserMsg.ID {
			continue // Skip the current message
		}
		role := string(msg.Role)
		if msg.Role == RoleAssistant {
			role = "assistant"
		}
		history = append(history, map[string]string{
			"role":    role,
			"content": msg.Content,
		})
	}
	contextData["history"] = history
	contextData["agui_thread_id"] = req.ThreadID
	contextData["agui_run_id"] = req.RunID

	// Add forwarded props
	if req.ForwardedProps != nil {
		for k, v := range req.ForwardedProps {
			contextData["forwarded_"+k] = v
		}
	}

	// Add state
	if req.State != nil {
		contextData["agui_state"] = req.State
	}

	return &models.Input{
		Raw:     lastUserMsg.Content,
		Type:    "text",
		Context: contextData,
	}
}

// ToolsFromFramework creates AG-UI tools from framework tools
func ToolsFromFramework(framework core.Framework) []Tool {
	frameworkTools := framework.ListTools()
	tools := make([]Tool, 0, len(frameworkTools))

	for _, name := range frameworkTools {
		tools = append(tools, Tool{
			Name:        name,
			Description: fmt.Sprintf("Tool: %s", name),
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		})
	}

	return tools
}

// AGUIServer wraps the AG-UI server with Minion integration
type AGUIServer struct {
	*Server
	framework core.Framework
	agentID   string
	bridge    *MinionBridge
}

// NewAGUIServer creates an AG-UI server integrated with Minion
func NewAGUIServer(framework core.Framework, agentID string, config ServerConfig) (*AGUIServer, error) {
	// Create bridge
	bridge := NewMinionBridge(framework, agentID)
	bridge.WithTools(ToolsFromFramework(framework))

	// Create server
	server := NewServer(bridge, config)

	return &AGUIServer{
		Server:    server,
		framework: framework,
		agentID:   agentID,
		bridge:    bridge,
	}, nil
}

// StreamingMinionBridge provides token-level streaming integration
type StreamingMinionBridge struct {
	*MinionBridge
	tokenBuffer chan string
}

// NewStreamingMinionBridge creates a streaming bridge
func NewStreamingMinionBridge(framework core.Framework, agentID string) *StreamingMinionBridge {
	return &StreamingMinionBridge{
		MinionBridge: NewMinionBridge(framework, agentID),
		tokenBuffer:  make(chan string, 1000),
	}
}

// HandleRun implements Handler with streaming support
func (b *StreamingMinionBridge) HandleRun(ctx context.Context, req RunRequest, emitter *EventEmitter) error {
	// Emit run started
	emitter.EmitRunStarted()

	// Convert AG-UI messages to Minion input
	input := b.requestToInput(req)

	// Start streaming message
	messageID := emitter.EmitTextMessageStart("", RoleAssistant)

	emitter.EmitStepStarted("generating")

	// Execute and stream
	// In a full implementation, integrate with Minion's chain streaming
	output, err := b.framework.Execute(ctx, b.agentID, input)
	if err != nil {
		emitter.EmitStepFinished("generating")
		emitter.EmitRunError(err.Error(), "EXECUTION_ERROR")
		return err
	}

	// Extract response from Result
	response := ""
	if output.Result != nil {
		response = fmt.Sprintf("%v", output.Result)
	}

	// Simulate token streaming (in production, use actual LLM streaming)
	tokens := tokenize(response)
	for _, token := range tokens {
		select {
		case <-ctx.Done():
			emitter.EmitTextMessageEnd(messageID)
			emitter.EmitStepFinished("generating")
			emitter.EmitRunError("cancelled", "CANCELLED")
			return ctx.Err()
		default:
			emitter.EmitTextMessageContent(messageID, token)
		}
	}

	emitter.EmitTextMessageEnd(messageID)
	emitter.EmitStepFinished("generating")

	// Process tool calls
	b.processToolCalls(ctx, output, emitter, messageID)

	emitter.EmitRunFinished()
	return nil
}

// processToolCalls handles tool calls from output
func (b *StreamingMinionBridge) processToolCalls(ctx context.Context, output *models.Output, emitter *EventEmitter, parentMsgID string) {
	if output.Metadata == nil {
		return
	}

	toolCalls, ok := output.Metadata["tool_calls"].([]interface{})
	if !ok {
		return
	}

	for _, tc := range toolCalls {
		tcMap, ok := tc.(map[string]interface{})
		if !ok {
			continue
		}

		toolName := fmt.Sprintf("%v", tcMap["name"])
		emitter.EmitStepStarted(fmt.Sprintf("tool:%s", toolName))

		toolCall := emitter.NewStreamingToolCall(toolName, parentMsgID)

		// Stream arguments
		if args, ok := tcMap["arguments"].(string); ok {
			toolCall.WriteArgs(args)
		} else if args, ok := tcMap["arguments"].(map[string]interface{}); ok {
			argsJSON, _ := json.Marshal(args)
			toolCall.WriteArgs(string(argsJSON))
		}

		// Execute tool if needed
		result := ""
		if res, ok := tcMap["result"].(string); ok {
			result = res
		}

		toolCall.End(result)
		emitter.EmitStepFinished(fmt.Sprintf("tool:%s", toolName))
	}
}

// tokenize splits text into tokens (simple word-based tokenization)
func tokenize(text string) []string {
	// Simple tokenization - split on spaces but keep punctuation
	var tokens []string
	current := ""

	for _, r := range text {
		if r == ' ' {
			if current != "" {
				tokens = append(tokens, current)
			}
			tokens = append(tokens, " ")
			current = ""
		} else if r == '\n' {
			if current != "" {
				tokens = append(tokens, current)
			}
			tokens = append(tokens, "\n")
			current = ""
		} else {
			current += string(r)
		}
	}

	if current != "" {
		tokens = append(tokens, current)
	}

	return tokens
}

// ConvertChainEventsToAGUI converts Minion chain events to AG-UI events
func ConvertChainEventsToAGUI(chainEvents <-chan interface{}, emitter *EventEmitter) {
	messageID := ""

	for event := range chainEvents {
		switch e := event.(type) {
		case map[string]interface{}:
			eventType, _ := e["type"].(string)

			switch eventType {
			case "token":
				if messageID == "" {
					messageID = emitter.EmitTextMessageStart("", RoleAssistant)
				}
				if content, ok := e["content"].(string); ok {
					emitter.EmitTextMessageContent(messageID, content)
				}

			case "complete":
				if messageID != "" {
					emitter.EmitTextMessageEnd(messageID)
					messageID = ""
				}

			case "tool_start":
				toolName, _ := e["tool"].(string)
				emitter.EmitToolCallStart(uuid.New().String(), toolName, messageID)

			case "tool_end":
				toolCallID, _ := e["tool_call_id"].(string)
				result, _ := e["result"].(string)
				emitter.EmitToolCallEnd(toolCallID, result)

			case "error":
				errMsg, _ := e["error"].(string)
				emitter.EmitRunError(errMsg, "CHAIN_ERROR")
			}
		}
	}

	// Ensure message is ended
	if messageID != "" {
		emitter.EmitTextMessageEnd(messageID)
	}
}

// MessagesToConversation converts AG-UI messages to a conversation string
func MessagesToConversation(messages []Message) string {
	var parts []string
	for _, msg := range messages {
		role := strings.Title(string(msg.Role))
		parts = append(parts, fmt.Sprintf("%s: %s", role, msg.Content))
	}
	return strings.Join(parts, "\n\n")
}
