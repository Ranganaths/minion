package multiagent

import (
	"context"
	"time"
)

// MessageType defines the type of inter-agent message
type MessageType string

const (
	// Context-oriented messages (agent <-> environment)
	MessageTypeRequest  MessageType = "request"  // Request for action/information
	MessageTypeResponse MessageType = "response" // Response to a request
	MessageTypeEvent    MessageType = "event"    // Event notification

	// Inter-agent messages (agent <-> agent)
	MessageTypeTask     MessageType = "task"     // Task assignment
	MessageTypeQuery    MessageType = "query"    // Query for information
	MessageTypeInform   MessageType = "inform"   // Information sharing
	MessageTypeDelegate MessageType = "delegate" // Task delegation
	MessageTypeResult   MessageType = "result"   // Task result
	MessageTypeError    MessageType = "error"    // Error notification
)

// Message represents a communication message between agents
// Based on KQML (Knowledge Query and Manipulation Language) protocol
type Message struct {
	ID        string                 `json:"id"`
	Type      MessageType            `json:"type"`
	From      string                 `json:"from"`       // Sender agent ID
	To        string                 `json:"to"`         // Recipient agent ID (empty for broadcast)
	InReplyTo string                 `json:"in_reply_to,omitempty"`
	Content   interface{}            `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// Protocol defines the communication protocol for multi-agent systems
type Protocol interface {
	// Send sends a message to an agent
	Send(ctx context.Context, msg *Message) error

	// Receive receives messages for an agent
	Receive(ctx context.Context, agentID string) ([]*Message, error)

	// Broadcast sends a message to all agents in a group
	Broadcast(ctx context.Context, msg *Message, groupID string) error

	// Subscribe subscribes an agent to messages of specific types
	Subscribe(ctx context.Context, agentID string, messageTypes []MessageType) error

	// Unsubscribe removes subscription
	Unsubscribe(ctx context.Context, agentID string) error
}

// AgentRole defines the role of an agent in the multi-agent system
type AgentRole string

const (
	RoleOrchestrator AgentRole = "orchestrator" // Coordinates other agents
	RoleWorker       AgentRole = "worker"       // Executes specific tasks
	RoleSpecialist   AgentRole = "specialist"   // Domain-specific expert
	RoleMonitor      AgentRole = "monitor"      // Monitors and reports
)

// MultiAgentCapability defines capabilities for multi-agent operations
type MultiAgentCapability struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Roles       []AgentRole `json:"roles"` // Which roles can use this capability
	InputTypes  []string `json:"input_types"`
	OutputTypes []string `json:"output_types"`
}

// AgentMetadata extends agent information for multi-agent systems
type AgentMetadata struct {
	AgentID      string                 `json:"agent_id"`
	Role         AgentRole              `json:"role"`
	Capabilities []string               `json:"capabilities"`
	GroupID      string                 `json:"group_id,omitempty"` // For agent grouping
	Priority     int                    `json:"priority"`           // For task assignment
	Status       AgentStatus            `json:"status"`
	CustomData   map[string]interface{} `json:"custom_data,omitempty"`
}

// AgentStatus defines the current status of an agent
type AgentStatus string

const (
	StatusIdle       AgentStatus = "idle"        // Agent is idle
	StatusBusy       AgentStatus = "busy"        // Agent is executing a task
	StatusWaiting    AgentStatus = "waiting"     // Agent is waiting for dependencies
	StatusFailed     AgentStatus = "failed"      // Agent has failed
	StatusOffline    AgentStatus = "offline"     // Agent is offline
)

// TaskPriority defines the priority level for tasks
type TaskPriority int

const (
	PriorityLow    TaskPriority = 1
	PriorityNormal TaskPriority = 5
	PriorityHigh   TaskPriority = 8
	PriorityCritical TaskPriority = 10
)

// Task represents a unit of work in the multi-agent system
type Task struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Type         string                 `json:"type"`
	Priority     TaskPriority           `json:"priority"`
	AssignedTo   string                 `json:"assigned_to,omitempty"` // Agent ID
	CreatedBy    string                 `json:"created_by"`            // Agent ID
	Dependencies []string               `json:"dependencies,omitempty"` // Task IDs
	Input        interface{}            `json:"input"`
	Output       interface{}            `json:"output,omitempty"`
	Status       TaskStatus             `json:"status"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
}

// TaskStatus defines the status of a task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusAssigned   TaskStatus = "assigned"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// SecurityPolicy defines security requirements for agent communication
type SecurityPolicy struct {
	RequireAuthentication bool     `json:"require_authentication"`
	RequireEncryption     bool     `json:"require_encryption"`
	AllowedAgents         []string `json:"allowed_agents,omitempty"` // Agent IDs
	DeniedAgents          []string `json:"denied_agents,omitempty"`  // Agent IDs
	MaxMessageSize        int64    `json:"max_message_size"`         // In bytes
	RateLimitPerSecond    int      `json:"rate_limit_per_second"`
}

// ProtocolMetrics tracks protocol performance
type ProtocolMetrics struct {
	TotalMessagesSent     int64         `json:"total_messages_sent"`
	TotalMessagesReceived int64         `json:"total_messages_received"`
	TotalMessagesFailed   int64         `json:"total_messages_failed"`
	AverageLatency        time.Duration `json:"average_latency"`
	LastUpdated           time.Time     `json:"last_updated"`
}
