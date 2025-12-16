package multiagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/agentql/agentql/pkg/minion/observability"
	"github.com/google/uuid"
)

// InMemoryProtocol implements Protocol interface with in-memory message queues
type InMemoryProtocol struct {
	mu               sync.RWMutex
	messageQueues    map[string][]*Message          // agentID -> messages
	subscriptions    map[string][]MessageType       // agentID -> message types
	groups           map[string][]string            // groupID -> agentIDs
	security         *SecurityPolicy
	metrics          *ProtocolMetrics
	maxQueueSize     int
	metricsCollector *observability.MetricsCollector
	tracer           *observability.Tracer
}

// NewInMemoryProtocol creates a new in-memory protocol implementation
// Accepts either *SecurityPolicy (legacy) or *InMemoryProtocolConfig
func NewInMemoryProtocol(config interface{}) *InMemoryProtocol {
	var security *SecurityPolicy
	var maxQueueSize int

	// Handle different config types for backward compatibility
	switch cfg := config.(type) {
	case *SecurityPolicy:
		security = cfg
		maxQueueSize = 1000
	case *InMemoryProtocolConfig:
		security = &SecurityPolicy{
			RequireAuthentication: false,
			RequireEncryption:     false,
			MaxMessageSize:        1024 * 1024, // 1MB
			RateLimitPerSecond:    1000,
		}
		if cfg != nil {
			maxQueueSize = cfg.MaxQueueSize
		} else {
			maxQueueSize = 1000
		}
	case nil:
		security = &SecurityPolicy{
			RequireAuthentication: false,
			RequireEncryption:     false,
			MaxMessageSize:        1024 * 1024, // 1MB
			RateLimitPerSecond:    1000,
		}
		maxQueueSize = 1000
	default:
		security = &SecurityPolicy{
			RequireAuthentication: false,
			RequireEncryption:     false,
			MaxMessageSize:        1024 * 1024, // 1MB
			RateLimitPerSecond:    1000,
		}
		maxQueueSize = 1000
	}

	return &InMemoryProtocol{
		messageQueues: make(map[string][]*Message),
		subscriptions: make(map[string][]MessageType),
		groups:        make(map[string][]string),
		security:      security,
		metrics: &ProtocolMetrics{
			LastUpdated: time.Now(),
		},
		maxQueueSize:     maxQueueSize,
		metricsCollector: observability.GetMetrics(),
		tracer:           observability.GetTracer(),
	}
}

// Send sends a message to an agent
func (p *InMemoryProtocol) Send(ctx context.Context, msg *Message) error {
	// Start tracing span
	msgID := msg.ID
	if msgID == "" {
		msgID = "pending"
	}
	ctx, span := p.tracer.StartProtocolSpan(ctx, "send", string(msg.Type), msgID)
	defer p.tracer.EndSpan(span, nil)

	start := time.Now()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Validate message
	if msg.To == "" {
		return fmt.Errorf("recipient (To) is required")
	}
	if msg.From == "" {
		return fmt.Errorf("sender (From) is required")
	}

	// Generate message ID if not provided
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}

	// Set timestamp
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	// Apply security policy
	if err := p.validateSecurity(msg); err != nil {
		p.metrics.TotalMessagesFailed++
		p.metricsCollector.RecordMultiagentError("protocol", "security_validation_failed")
		return fmt.Errorf("security validation failed: %w", err)
	}

	// Check subscription
	if !p.isSubscribed(msg.To, msg.Type) {
		// Store anyway but mark in metadata
		if msg.Metadata == nil {
			msg.Metadata = make(map[string]interface{})
		}
		msg.Metadata["unsubscribed"] = true
	}

	// Add to queue
	queue := p.messageQueues[msg.To]
	if len(queue) >= p.maxQueueSize {
		p.metricsCollector.RecordMultiagentError("protocol", "queue_full")
		return fmt.Errorf("message queue full for agent %s", msg.To)
	}

	p.messageQueues[msg.To] = append(queue, msg)
	p.metrics.TotalMessagesSent++

	// Record metrics
	latency := time.Since(start)
	p.metricsCollector.RecordMultiagentMessageSent(string(msg.Type), latency)
	p.metricsCollector.SetMultiagentQueueDepth(msg.To, len(p.messageQueues[msg.To]))

	return nil
}

// Receive receives messages for an agent
func (p *InMemoryProtocol) Receive(ctx context.Context, agentID string) ([]*Message, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	messages := p.messageQueues[agentID]

	// Clear the queue
	delete(p.messageQueues, agentID)

	p.metrics.TotalMessagesReceived += int64(len(messages))

	// Record metrics for each message received
	for _, msg := range messages {
		p.metricsCollector.RecordMultiagentMessageReceived(string(msg.Type))
	}

	// Update queue depth to 0 since we cleared it
	p.metricsCollector.SetMultiagentQueueDepth(agentID, 0)

	return messages, nil
}

// Broadcast sends a message to all agents in a group
func (p *InMemoryProtocol) Broadcast(ctx context.Context, msg *Message, groupID string) error {
	p.mu.RLock()
	agentIDs, exists := p.groups[groupID]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("group %s not found", groupID)
	}

	// Send to each agent in the group
	for _, agentID := range agentIDs {
		msgCopy := *msg
		msgCopy.To = agentID
		if err := p.Send(ctx, &msgCopy); err != nil {
			return fmt.Errorf("failed to send to agent %s: %w", agentID, err)
		}
	}

	return nil
}

// Subscribe subscribes an agent to messages of specific types
func (p *InMemoryProtocol) Subscribe(ctx context.Context, agentID string, messageTypes []MessageType) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.subscriptions[agentID] = messageTypes

	return nil
}

// Unsubscribe removes subscription
func (p *InMemoryProtocol) Unsubscribe(ctx context.Context, agentID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.subscriptions, agentID)
	delete(p.messageQueues, agentID)

	return nil
}

// RegisterGroup registers a group of agents
func (p *InMemoryProtocol) RegisterGroup(groupID string, agentIDs []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.groups[groupID] = agentIDs

	return nil
}

// GetMetrics returns protocol metrics
func (p *InMemoryProtocol) GetMetrics() *ProtocolMetrics {
	p.mu.RLock()
	defer p.mu.RUnlock()

	metricsCopy := *p.metrics
	return &metricsCopy
}

// validateSecurity validates message against security policy
func (p *InMemoryProtocol) validateSecurity(msg *Message) error {
	if p.security == nil {
		return nil
	}

	// Check allowed/denied agents
	if len(p.security.AllowedAgents) > 0 {
		allowed := false
		for _, id := range p.security.AllowedAgents {
			if id == msg.From || id == msg.To {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("agent not in allowed list")
		}
	}

	for _, id := range p.security.DeniedAgents {
		if id == msg.From || id == msg.To {
			return fmt.Errorf("agent is denied")
		}
	}

	return nil
}

// isSubscribed checks if an agent is subscribed to a message type
func (p *InMemoryProtocol) isSubscribed(agentID string, msgType MessageType) bool {
	types, exists := p.subscriptions[agentID]
	if !exists {
		return false
	}

	for _, t := range types {
		if t == msgType {
			return true
		}
	}

	return false
}
