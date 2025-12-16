package models

import "context"

// Input represents generic input to an agent
type Input struct {
	Raw     string                 `json:"raw"`     // Raw input (e.g., natural language query)
	Type    string                 `json:"type"`    // Input type (e.g., "text", "query", "command")
	Context map[string]interface{} `json:"context"` // Additional context
}

// Output represents generic output from an agent
type Output struct {
	Result   interface{}            `json:"result"`   // Main result
	Type     string                 `json:"type"`     // Output type
	Metadata map[string]interface{} `json:"metadata"` // Additional metadata
	Error    string                 `json:"error,omitempty"`
}

// ProcessedInput represents input after processing by behavior
type ProcessedInput struct {
	Original      *Input                 `json:"original"`
	Processed     interface{}            `json:"processed"`
	Instructions  string                 `json:"instructions"`
	ExtraContext  map[string]interface{} `json:"extra_context"`
}

// ProcessedOutput represents output after processing by behavior
type ProcessedOutput struct {
	Original  *Output                `json:"original"`
	Processed interface{}            `json:"processed"`
	Enhanced  map[string]interface{} `json:"enhanced"`
}

// ExecutionContext holds context for agent execution
type ExecutionContext struct {
	Context   context.Context
	AgentID   string
	InputType string
	Metadata  map[string]interface{}
}
