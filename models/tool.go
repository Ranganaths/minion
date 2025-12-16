package models

// ToolInput represents input to a tool
type ToolInput struct {
	Data    interface{}            `json:"data"`
	Params  map[string]interface{} `json:"params"`
	Context map[string]interface{} `json:"context"`
}

// ToolOutput represents output from a tool
type ToolOutput struct {
	ToolName       string                 `json:"tool_name"`
	Success        bool                   `json:"success"`
	Result         interface{}            `json:"result"`
	Metadata       map[string]interface{} `json:"metadata"`
	ExecutionTime  int64                  `json:"execution_time_ms"`
	Error          string                 `json:"error,omitempty"`
}

// ToolCapability represents what a tool can do
type ToolCapability struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Required    []string `json:"required"` // Required agent capabilities
}
