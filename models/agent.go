package models

import "time"

// Agent represents a Minion framework agent
type Agent struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	BehaviorType string                 `json:"behavior_type"` // e.g., "analytical", "conversational"
	Status       AgentStatus            `json:"status"`
	Config       AgentConfig            `json:"config"`
	Capabilities []string               `json:"capabilities"` // Generic capabilities
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// AgentStatus represents the lifecycle status of an agent
type AgentStatus string

const (
	StatusDraft    AgentStatus = "draft"
	StatusActive   AgentStatus = "active"
	StatusInactive AgentStatus = "inactive"
	StatusArchived AgentStatus = "archived"
)

// AgentConfig holds configuration for an agent
type AgentConfig struct {
	// LLM configuration
	LLMProvider string  `json:"llm_provider"` // "openai", "anthropic", etc.
	LLMModel    string  `json:"llm_model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`

	// Behavior configuration
	Personality string `json:"personality"` // "professional", "friendly", "concise"
	Language    string `json:"language"`    // "en", "es", etc.

	// Custom configuration (behavior-specific)
	Custom map[string]interface{} `json:"custom"`
}

// CreateAgentRequest is used to create a new agent
type CreateAgentRequest struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	BehaviorType string                 `json:"behavior_type"`
	Config       AgentConfig            `json:"config"`
	Capabilities []string               `json:"capabilities"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// UpdateAgentRequest is used to update an existing agent
type UpdateAgentRequest struct {
	Name         *string                 `json:"name,omitempty"`
	Description  *string                 `json:"description,omitempty"`
	Status       *AgentStatus            `json:"status,omitempty"`
	Config       *AgentConfig            `json:"config,omitempty"`
	Capabilities *[]string               `json:"capabilities,omitempty"`
	Metadata     *map[string]interface{} `json:"metadata,omitempty"`
}

// ListAgentsRequest is used to filter agents
type ListAgentsRequest struct {
	BehaviorType *string      `json:"behavior_type,omitempty"`
	Status       *AgentStatus `json:"status,omitempty"`
	Search       string       `json:"search,omitempty"`
	Page         int          `json:"page"`
	PageSize     int          `json:"page_size"`
}

// ListAgentsResponse contains paginated agent results
type ListAgentsResponse struct {
	Agents     []Agent `json:"agents"`
	Total      int     `json:"total"`
	Page       int     `json:"page"`
	PageSize   int     `json:"page_size"`
	TotalPages int     `json:"total_pages"`
}
