package humanloop

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// NotificationType represents the type of notification
type NotificationType string

const (
	NotificationTypeApprovalRequired NotificationType = "approval_required"
	NotificationTypeEscalation       NotificationType = "escalation"
	NotificationTypeAgentError       NotificationType = "agent_error"
	NotificationTypeTaskComplete     NotificationType = "task_complete"
	NotificationTypeFeedbackRequest  NotificationType = "feedback_request"
	NotificationTypeWarning          NotificationType = "warning"
	NotificationTypeInfo             NotificationType = "info"
)

// NotificationPriority represents the priority of a notification
type NotificationPriority string

const (
	NotificationPriorityLow      NotificationPriority = "low"
	NotificationPriorityMedium   NotificationPriority = "medium"
	NotificationPriorityHigh     NotificationPriority = "high"
	NotificationPriorityCritical NotificationPriority = "critical"
)

// Notification represents a notification to be sent
type Notification struct {
	ID         string                 `json:"id"`
	Type       NotificationType       `json:"type"`
	Title      string                 `json:"title"`
	Message    string                 `json:"message"`
	Priority   NotificationPriority   `json:"priority"`
	Recipients []string               `json:"recipients,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	ExpiresAt  *time.Time             `json:"expires_at,omitempty"`
}

// Notifier is the interface for notification providers
type Notifier interface {
	// Notify sends a notification
	Notify(ctx context.Context, notification *Notification) error

	// Name returns the notifier name
	Name() string
}

// NotificationManager manages multiple notifiers
type NotificationManager struct {
	notifiers   map[string]Notifier
	routingRules []NotificationRoutingRule
	history     []*NotificationRecord
	maxHistory  int
	mu          sync.RWMutex
}

// NotificationRoutingRule defines how notifications are routed
type NotificationRoutingRule struct {
	// Name of the rule
	Name string

	// Types to match (empty matches all)
	Types []NotificationType

	// MinPriority is the minimum priority to match
	MinPriority NotificationPriority

	// Notifiers to send to
	NotifierNames []string

	// Filter is an optional filter function
	Filter func(*Notification) bool
}

// NotificationRecord records a sent notification
type NotificationRecord struct {
	Notification *Notification
	NotifierName string
	SentAt       time.Time
	Error        error
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager() *NotificationManager {
	return &NotificationManager{
		notifiers:  make(map[string]Notifier),
		history:    make([]*NotificationRecord, 0),
		maxHistory: 1000,
	}
}

// RegisterNotifier registers a notifier
func (m *NotificationManager) RegisterNotifier(notifier Notifier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifiers[notifier.Name()] = notifier
}

// UnregisterNotifier removes a notifier
func (m *NotificationManager) UnregisterNotifier(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.notifiers, name)
}

// AddRoutingRule adds a notification routing rule
func (m *NotificationManager) AddRoutingRule(rule NotificationRoutingRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routingRules = append(m.routingRules, rule)
}

// Notify sends a notification using routing rules
func (m *NotificationManager) Notify(ctx context.Context, notification *Notification) error {
	if notification.ID == "" {
		notification.ID = uuid.New().String()
	}
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now()
	}

	m.mu.RLock()
	rules := m.routingRules
	notifiers := make(map[string]Notifier)
	for k, v := range m.notifiers {
		notifiers[k] = v
	}
	m.mu.RUnlock()

	// Find matching notifiers
	targetNotifiers := m.findMatchingNotifiers(notification, rules, notifiers)

	if len(targetNotifiers) == 0 {
		// No routing rules matched, send to all notifiers
		for _, n := range notifiers {
			targetNotifiers = append(targetNotifiers, n)
		}
	}

	// Send to all matched notifiers
	var firstErr error
	var wg sync.WaitGroup

	for _, notifier := range targetNotifiers {
		wg.Add(1)
		go func(n Notifier) {
			defer wg.Done()

			err := n.Notify(ctx, notification)

			// Record history
			m.recordNotification(notification, n.Name(), err)

			if err != nil && firstErr == nil {
				firstErr = err
			}
		}(notifier)
	}

	wg.Wait()
	return firstErr
}

// findMatchingNotifiers finds notifiers that match the notification
func (m *NotificationManager) findMatchingNotifiers(
	notification *Notification,
	rules []NotificationRoutingRule,
	notifiers map[string]Notifier,
) []Notifier {
	matched := make(map[string]Notifier)

	for _, rule := range rules {
		if m.ruleMatches(rule, notification) {
			for _, name := range rule.NotifierNames {
				if n, ok := notifiers[name]; ok {
					matched[name] = n
				}
			}
		}
	}

	result := make([]Notifier, 0, len(matched))
	for _, n := range matched {
		result = append(result, n)
	}
	return result
}

// ruleMatches checks if a rule matches a notification
func (m *NotificationManager) ruleMatches(rule NotificationRoutingRule, notification *Notification) bool {
	// Check type filter
	if len(rule.Types) > 0 {
		typeMatched := false
		for _, t := range rule.Types {
			if t == notification.Type {
				typeMatched = true
				break
			}
		}
		if !typeMatched {
			return false
		}
	}

	// Check priority
	if rule.MinPriority != "" {
		if !priorityMeetsMinimum(notification.Priority, rule.MinPriority) {
			return false
		}
	}

	// Check custom filter
	if rule.Filter != nil && !rule.Filter(notification) {
		return false
	}

	return true
}

// recordNotification records a notification in history
func (m *NotificationManager) recordNotification(notification *Notification, notifierName string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record := &NotificationRecord{
		Notification: notification,
		NotifierName: notifierName,
		SentAt:       time.Now(),
		Error:        err,
	}

	m.history = append(m.history, record)

	// Trim history if needed
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
}

// GetHistory returns notification history
func (m *NotificationManager) GetHistory(limit int) []*NotificationRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	// Return most recent first
	result := make([]*NotificationRecord, limit)
	for i := 0; i < limit; i++ {
		result[i] = m.history[len(m.history)-1-i]
	}

	return result
}

// priorityMeetsMinimum checks if a priority meets the minimum threshold
func priorityMeetsMinimum(priority, minimum NotificationPriority) bool {
	priorities := map[NotificationPriority]int{
		NotificationPriorityLow:      1,
		NotificationPriorityMedium:   2,
		NotificationPriorityHigh:     3,
		NotificationPriorityCritical: 4,
	}

	return priorities[priority] >= priorities[minimum]
}

// LogNotifier logs notifications (useful for development/testing)
type LogNotifier struct {
	name    string
	logFunc func(string)
}

// NewLogNotifier creates a notifier that logs notifications
func NewLogNotifier(name string, logFunc func(string)) *LogNotifier {
	if logFunc == nil {
		logFunc = func(msg string) {
			fmt.Println(msg)
		}
	}
	return &LogNotifier{name: name, logFunc: logFunc}
}

func (n *LogNotifier) Notify(ctx context.Context, notification *Notification) error {
	msg := fmt.Sprintf("[%s] %s: %s - %s",
		notification.Priority,
		notification.Type,
		notification.Title,
		notification.Message)
	n.logFunc(msg)
	return nil
}

func (n *LogNotifier) Name() string {
	return n.name
}

// ChannelNotifier sends notifications to a channel (useful for testing)
type ChannelNotifier struct {
	name string
	ch   chan *Notification
}

// NewChannelNotifier creates a notifier that sends to a channel
func NewChannelNotifier(name string, bufferSize int) *ChannelNotifier {
	return &ChannelNotifier{
		name: name,
		ch:   make(chan *Notification, bufferSize),
	}
}

func (n *ChannelNotifier) Notify(ctx context.Context, notification *Notification) error {
	select {
	case n.ch <- notification:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("notification channel is full")
	}
}

func (n *ChannelNotifier) Name() string {
	return n.name
}

// Channel returns the notification channel
func (n *ChannelNotifier) Channel() <-chan *Notification {
	return n.ch
}

// WebhookNotifier sends notifications via HTTP webhook
type WebhookNotifier struct {
	name    string
	url     string
	headers map[string]string
	client  HTTPClient
}

// HTTPClient interface for HTTP requests
type HTTPClient interface {
	Post(ctx context.Context, url string, headers map[string]string, body []byte) error
}

// NewWebhookNotifier creates a webhook notifier
func NewWebhookNotifier(name, url string, headers map[string]string, client HTTPClient) *WebhookNotifier {
	return &WebhookNotifier{
		name:    name,
		url:     url,
		headers: headers,
		client:  client,
	}
}

func (n *WebhookNotifier) Notify(ctx context.Context, notification *Notification) error {
	if n.client == nil {
		return fmt.Errorf("HTTP client not configured")
	}

	// Simple JSON encoding
	body := fmt.Sprintf(`{"id":"%s","type":"%s","title":"%s","message":"%s","priority":"%s"}`,
		notification.ID,
		notification.Type,
		notification.Title,
		notification.Message,
		notification.Priority)

	headers := make(map[string]string)
	for k, v := range n.headers {
		headers[k] = v
	}
	headers["Content-Type"] = "application/json"

	return n.client.Post(ctx, n.url, headers, []byte(body))
}

func (n *WebhookNotifier) Name() string {
	return n.name
}

// Ensure implementations satisfy interfaces
var _ Notifier = (*LogNotifier)(nil)
var _ Notifier = (*ChannelNotifier)(nil)
var _ Notifier = (*WebhookNotifier)(nil)
