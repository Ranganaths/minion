package models

import "time"

// Activity represents an agent execution activity
type Activity struct {
	ID        string                 `json:"id"`
	AgentID   string                 `json:"agent_id"`
	Action    string                 `json:"action"` // "execute", "configure", etc.
	Input     *Input                 `json:"input,omitempty"`
	Output    *Output                `json:"output,omitempty"`
	Status    string                 `json:"status"` // "success", "failure", "partial"
	Duration  int64                  `json:"duration_ms"`
	ToolsUsed []string               `json:"tools_used,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Error     string                 `json:"error,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// Metrics represents agent performance metrics
type Metrics struct {
	AgentID           string     `json:"agent_id"`
	TotalExecutions   int        `json:"total_executions"`
	SuccessfulExecutions int     `json:"successful_executions"`
	FailedExecutions  int        `json:"failed_executions"`
	AvgExecutionTime  float64    `json:"avg_execution_time_ms"`
	LastExecutionAt   *time.Time `json:"last_execution_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// MetricsSnapshot represents a point-in-time snapshot
type MetricsSnapshot struct {
	Timestamp        time.Time              `json:"timestamp"`
	Metrics          *Metrics               `json:"metrics"`
	CustomMetrics    map[string]interface{} `json:"custom_metrics,omitempty"`
}
