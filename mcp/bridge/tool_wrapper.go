package bridge

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yourusername/minion/mcp/client"
	"github.com/yourusername/minion/models"
	"github.com/yourusername/minion/tools"
)

// MCPToolWrapper wraps an external MCP tool as a Minion Tool
type MCPToolWrapper struct {
	// Server information
	serverName string

	// MCP tool metadata
	mcpTool client.MCPTool

	// Client manager reference
	clientManager *client.MCPClientManager

	// Capability mapping
	requiredCapabilities []string

	// Schema validation
	validator        *client.SchemaValidator
	validateSchema   bool
}

// Ensure MCPToolWrapper implements tools.Tool interface
var _ tools.Tool = (*MCPToolWrapper)(nil)

// NewMCPToolWrapper creates a new MCP tool wrapper
func NewMCPToolWrapper(
	serverName string,
	mcpTool client.MCPTool,
	clientManager *client.MCPClientManager,
) *MCPToolWrapper {
	return &MCPToolWrapper{
		serverName:           serverName,
		mcpTool:              mcpTool,
		clientManager:        clientManager,
		requiredCapabilities: []string{},
		validator:            client.NewSchemaValidator(false), // Non-strict mode by default
		validateSchema:       true,                              // Enable validation by default
	}
}

// WithSchemaValidation enables or disables schema validation
func (w *MCPToolWrapper) WithSchemaValidation(enabled bool) *MCPToolWrapper {
	w.validateSchema = enabled
	return w
}

// WithStrictValidation enables strict schema validation
func (w *MCPToolWrapper) WithStrictValidation(strict bool) *MCPToolWrapper {
	w.validator = client.NewSchemaValidator(strict)
	return w
}

// Name returns the qualified tool name
func (w *MCPToolWrapper) Name() string {
	// Format: mcp_<server>_<tool>
	// Example: mcp_github_create_issue
	return fmt.Sprintf("mcp_%s_%s", w.serverName, w.mcpTool.Name)
}

// Description returns tool description with MCP prefix
func (w *MCPToolWrapper) Description() string {
	return fmt.Sprintf("[MCP:%s] %s", w.serverName, w.mcpTool.Description)
}

// Execute executes the MCP tool via the client
func (w *MCPToolWrapper) Execute(
	ctx context.Context,
	input *models.ToolInput,
) (*models.ToolOutput, error) {
	// Prepare output structure
	output := &models.ToolOutput{
		ToolName: w.Name(),
	}

	// Validate input parameters against schema if enabled
	if w.validateSchema && w.mcpTool.InputSchema != nil {
		if err := w.validator.ValidateToolCall(&w.mcpTool, input.Params); err != nil {
			output.Success = false
			output.Error = fmt.Sprintf("Input validation failed: %v", err)
			return output, nil
		}
	}

	// Get MCP client for this server
	mcpClient, err := w.clientManager.GetClient(w.serverName)
	if err != nil {
		output.Success = false
		output.Error = fmt.Sprintf("Server not connected: %v", err)
		return output, nil
	}

	// Execute tool on remote server
	result, err := mcpClient.CallTool(ctx, w.mcpTool.Name, input.Params)
	if err != nil {
		output.Success = false
		output.Error = fmt.Sprintf("Tool execution failed: %v", err)
		return output, nil
	}

	// Check if MCP tool reported an error
	if result.IsError {
		output.Success = false
		output.Error = w.extractErrorMessage(result)
		return output, nil
	}

	// Extract result from MCP response
	output.Success = true
	output.Result = w.extractResult(result)

	return output, nil
}

// CanExecute checks if agent has required capabilities
func (w *MCPToolWrapper) CanExecute(agent *models.Agent) bool {
	// Check for MCP integration capability
	hasMCPCapability := false
	hasServerCapability := false

	for _, cap := range agent.Capabilities {
		if cap == "mcp_integration" {
			hasMCPCapability = true
		}
		if cap == fmt.Sprintf("mcp_%s", w.serverName) {
			hasServerCapability = true
		}
	}

	// Agent needs either global MCP capability or server-specific capability
	if !hasMCPCapability && !hasServerCapability {
		return false
	}

	// Check additional required capabilities
	if len(w.requiredCapabilities) > 0 {
		capMap := make(map[string]bool)
		for _, cap := range agent.Capabilities {
			capMap[cap] = true
		}

		for _, required := range w.requiredCapabilities {
			if !capMap[required] {
				return false
			}
		}
	}

	return true
}

// extractResult extracts result data from MCP response
func (w *MCPToolWrapper) extractResult(result *client.MCPCallToolResult) interface{} {
	if len(result.Content) == 0 {
		return nil
	}

	// Handle different content types
	content := result.Content[0]

	switch v := content.(type) {
	case map[string]interface{}:
		// Extract text content
		if text, ok := v["text"].(string); ok {
			// Try to parse as JSON for structured data
			var structured interface{}
			if err := json.Unmarshal([]byte(text), &structured); err == nil {
				return structured
			}
			return text
		}
		return v
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// extractErrorMessage extracts error message from MCP result
func (w *MCPToolWrapper) extractErrorMessage(result *client.MCPCallToolResult) string {
	if len(result.Content) == 0 {
		return "Unknown error"
	}

	content := result.Content[0]

	switch v := content.(type) {
	case map[string]interface{}:
		if text, ok := v["text"].(string); ok {
			return text
		}
		return fmt.Sprintf("%v", v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
