package humanloop

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EscalationLevel represents the severity of an escalation
type EscalationLevel int

const (
	EscalationLevelInfo EscalationLevel = iota
	EscalationLevelWarning
	EscalationLevelError
	EscalationLevelCritical
)

func (l EscalationLevel) String() string {
	switch l {
	case EscalationLevelInfo:
		return "info"
	case EscalationLevelWarning:
		return "warning"
	case EscalationLevelError:
		return "error"
	case EscalationLevelCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// EscalationStatus represents the status of an escalation
type EscalationStatus string

const (
	EscalationStatusOpen       EscalationStatus = "open"
	EscalationStatusAcknowledged EscalationStatus = "acknowledged"
	EscalationStatusResolved   EscalationStatus = "resolved"
	EscalationStatusEscalated  EscalationStatus = "escalated"
)

// Escalation represents an escalation event
type Escalation struct {
	ID            string                 `json:"id"`
	Level         EscalationLevel        `json:"level"`
	Status        EscalationStatus       `json:"status"`
	Title         string                 `json:"title"`
	Description   string                 `json:"description"`
	Source        string                 `json:"source"` // Agent or task ID
	Error         error                  `json:"-"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	Context       map[string]interface{} `json:"context,omitempty"`
	CurrentTier   int                    `json:"current_tier"`
	MaxTier       int                    `json:"max_tier"`
	CreatedAt     time.Time              `json:"created_at"`
	AcknowledgedAt *time.Time            `json:"acknowledged_at,omitempty"`
	AcknowledgedBy string                `json:"acknowledged_by,omitempty"`
	ResolvedAt    *time.Time             `json:"resolved_at,omitempty"`
	ResolvedBy    string                 `json:"resolved_by,omitempty"`
	Resolution    string                 `json:"resolution,omitempty"`
}

// EscalationTier defines a tier in the escalation chain
type EscalationTier struct {
	// Level is the tier number (0-indexed)
	Level int

	// Name is the tier name (e.g., "L1 Support", "Engineering")
	Name string

	// NotifyGroups are the groups to notify at this tier
	NotifyGroups []string

	// TimeoutBefore escalating to next tier
	Timeout time.Duration

	// AutoResolve automatically resolves if no action after timeout
	// If false, escalates to next tier
	AutoResolve bool
}

// EscalationPolicy defines how escalations are handled
type EscalationPolicy struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Tiers       []EscalationTier  `json:"tiers"`
	MinLevel    EscalationLevel   `json:"min_level"` // Minimum level to trigger this policy
}

// EscalationManager manages escalations
type EscalationManager struct {
	policies        map[string]*EscalationPolicy
	defaultPolicy   *EscalationPolicy
	escalations     map[string]*Escalation
	notificationMgr *NotificationManager
	onEscalation    func(*Escalation)
	onResolution    func(*Escalation)
	stopChan        chan struct{}
	mu              sync.RWMutex
}

// EscalationManagerConfig configures the escalation manager
type EscalationManagerConfig struct {
	NotificationManager *NotificationManager
	DefaultPolicy       *EscalationPolicy
	OnEscalation        func(*Escalation)
	OnResolution        func(*Escalation)
}

// NewEscalationManager creates a new escalation manager
func NewEscalationManager(config EscalationManagerConfig) *EscalationManager {
	if config.DefaultPolicy == nil {
		config.DefaultPolicy = DefaultEscalationPolicy()
	}

	em := &EscalationManager{
		policies:        make(map[string]*EscalationPolicy),
		defaultPolicy:   config.DefaultPolicy,
		escalations:     make(map[string]*Escalation),
		notificationMgr: config.NotificationManager,
		onEscalation:    config.OnEscalation,
		onResolution:    config.OnResolution,
		stopChan:        make(chan struct{}),
	}

	em.policies[config.DefaultPolicy.ID] = config.DefaultPolicy

	return em
}

// DefaultEscalationPolicy returns a default escalation policy
func DefaultEscalationPolicy() *EscalationPolicy {
	return &EscalationPolicy{
		ID:          "default",
		Name:        "Default Escalation Policy",
		Description: "Standard 3-tier escalation policy",
		Tiers: []EscalationTier{
			{
				Level:    0,
				Name:     "Initial Alert",
				Timeout:  15 * time.Minute,
			},
			{
				Level:    1,
				Name:     "L1 Escalation",
				Timeout:  30 * time.Minute,
			},
			{
				Level:       2,
				Name:        "L2 Escalation",
				Timeout:     1 * time.Hour,
				AutoResolve: true,
			},
		},
		MinLevel: EscalationLevelWarning,
	}
}

// RegisterPolicy registers an escalation policy
func (m *EscalationManager) RegisterPolicy(policy *EscalationPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policies[policy.ID] = policy
}

// SetDefaultPolicy sets the default escalation policy
func (m *EscalationManager) SetDefaultPolicy(policyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, ok := m.policies[policyID]
	if !ok {
		return fmt.Errorf("policy %s not found", policyID)
	}
	m.defaultPolicy = policy
	return nil
}

// Escalate creates a new escalation
func (m *EscalationManager) Escalate(ctx context.Context, title, description, source string, level EscalationLevel, err error, context map[string]interface{}) (*Escalation, error) {
	return m.EscalateWithPolicy(ctx, "", title, description, source, level, err, context)
}

// EscalateWithPolicy creates a new escalation with a specific policy
func (m *EscalationManager) EscalateWithPolicy(ctx context.Context, policyID, title, description, source string, level EscalationLevel, err error, escalationContext map[string]interface{}) (*Escalation, error) {
	m.mu.Lock()
	policy := m.defaultPolicy
	if policyID != "" {
		if p, ok := m.policies[policyID]; ok {
			policy = p
		}
	}
	m.mu.Unlock()

	if policy == nil {
		return nil, errors.New("no escalation policy found")
	}

	// Check if level meets minimum for policy
	if level < policy.MinLevel {
		return nil, fmt.Errorf("escalation level %s is below policy minimum %s", level, policy.MinLevel)
	}

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	escalation := &Escalation{
		ID:           uuid.New().String(),
		Level:        level,
		Status:       EscalationStatusOpen,
		Title:        title,
		Description:  description,
		Source:       source,
		Error:        err,
		ErrorMessage: errMsg,
		Context:      escalationContext,
		CurrentTier:  0,
		MaxTier:      len(policy.Tiers) - 1,
		CreatedAt:    time.Now(),
	}

	m.mu.Lock()
	m.escalations[escalation.ID] = escalation
	m.mu.Unlock()

	// Send initial notification
	m.notifyEscalation(ctx, escalation, policy.Tiers[0])

	// Call escalation handler
	if m.onEscalation != nil {
		m.onEscalation(escalation)
	}

	// Start escalation timer
	go m.escalationTimer(ctx, escalation.ID, policy)

	return escalation, nil
}

// Acknowledge acknowledges an escalation
func (m *EscalationManager) Acknowledge(ctx context.Context, escalationID, userID string) error {
	m.mu.Lock()
	escalation, ok := m.escalations[escalationID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("escalation %s not found", escalationID)
	}

	if escalation.Status != EscalationStatusOpen && escalation.Status != EscalationStatusEscalated {
		m.mu.Unlock()
		return fmt.Errorf("escalation is already %s", escalation.Status)
	}

	now := time.Now()
	escalation.Status = EscalationStatusAcknowledged
	escalation.AcknowledgedAt = &now
	escalation.AcknowledgedBy = userID
	m.mu.Unlock()

	return nil
}

// Resolve resolves an escalation
func (m *EscalationManager) Resolve(ctx context.Context, escalationID, userID, resolution string) error {
	m.mu.Lock()
	escalation, ok := m.escalations[escalationID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("escalation %s not found", escalationID)
	}

	if escalation.Status == EscalationStatusResolved {
		m.mu.Unlock()
		return errors.New("escalation is already resolved")
	}

	now := time.Now()
	escalation.Status = EscalationStatusResolved
	escalation.ResolvedAt = &now
	escalation.ResolvedBy = userID
	escalation.Resolution = resolution
	m.mu.Unlock()

	// Notify resolution
	m.notifyResolution(ctx, escalation)

	// Call resolution handler
	if m.onResolution != nil {
		m.onResolution(escalation)
	}

	return nil
}

// GetEscalation returns an escalation by ID
func (m *EscalationManager) GetEscalation(id string) (*Escalation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.escalations[id]
	return e, ok
}

// GetOpenEscalations returns all open escalations
func (m *EscalationManager) GetOpenEscalations() []*Escalation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Escalation, 0)
	for _, e := range m.escalations {
		if e.Status == EscalationStatusOpen || e.Status == EscalationStatusEscalated {
			result = append(result, e)
		}
	}
	return result
}

// escalationTimer handles escalation timing
func (m *EscalationManager) escalationTimer(ctx context.Context, escalationID string, policy *EscalationPolicy) {
	for {
		m.mu.RLock()
		escalation, ok := m.escalations[escalationID]
		if !ok {
			m.mu.RUnlock()
			return
		}

		// Check if resolved or acknowledged
		if escalation.Status == EscalationStatusResolved || escalation.Status == EscalationStatusAcknowledged {
			m.mu.RUnlock()
			return
		}

		currentTier := escalation.CurrentTier
		m.mu.RUnlock()

		if currentTier >= len(policy.Tiers) {
			return
		}

		tier := policy.Tiers[currentTier]

		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-time.After(tier.Timeout):
			m.mu.Lock()
			escalation, ok = m.escalations[escalationID]
			if !ok {
				m.mu.Unlock()
				return
			}

			// Check if already resolved
			if escalation.Status == EscalationStatusResolved || escalation.Status == EscalationStatusAcknowledged {
				m.mu.Unlock()
				return
			}

			// Check if at max tier
			if currentTier >= len(policy.Tiers)-1 {
				if tier.AutoResolve {
					now := time.Now()
					escalation.Status = EscalationStatusResolved
					escalation.ResolvedAt = &now
					escalation.Resolution = "Auto-resolved after timeout"
				}
				m.mu.Unlock()
				return
			}

			// Escalate to next tier
			escalation.CurrentTier++
			escalation.Status = EscalationStatusEscalated
			m.mu.Unlock()

			// Notify next tier
			nextTier := policy.Tiers[escalation.CurrentTier]
			m.notifyEscalation(ctx, escalation, nextTier)
		}
	}
}

// notifyEscalation sends notification for an escalation
func (m *EscalationManager) notifyEscalation(ctx context.Context, escalation *Escalation, tier EscalationTier) {
	if m.notificationMgr == nil {
		return
	}

	priority := NotificationPriorityMedium
	switch escalation.Level {
	case EscalationLevelInfo:
		priority = NotificationPriorityLow
	case EscalationLevelWarning:
		priority = NotificationPriorityMedium
	case EscalationLevelError:
		priority = NotificationPriorityHigh
	case EscalationLevelCritical:
		priority = NotificationPriorityCritical
	}

	notification := &Notification{
		ID:       uuid.New().String(),
		Type:     NotificationTypeEscalation,
		Title:    fmt.Sprintf("[%s] %s: %s", tier.Name, escalation.Level.String(), escalation.Title),
		Message:  escalation.Description,
		Priority: priority,
		Metadata: map[string]interface{}{
			"escalation_id": escalation.ID,
			"tier":          tier.Level,
			"tier_name":     tier.Name,
			"source":        escalation.Source,
			"level":         escalation.Level.String(),
		},
		Recipients: tier.NotifyGroups,
		CreatedAt:  time.Now(),
	}

	m.notificationMgr.Notify(ctx, notification)
}

// notifyResolution sends notification for a resolved escalation
func (m *EscalationManager) notifyResolution(ctx context.Context, escalation *Escalation) {
	if m.notificationMgr == nil {
		return
	}

	notification := &Notification{
		ID:       uuid.New().String(),
		Type:     NotificationTypeInfo,
		Title:    fmt.Sprintf("Resolved: %s", escalation.Title),
		Message:  escalation.Resolution,
		Priority: NotificationPriorityLow,
		Metadata: map[string]interface{}{
			"escalation_id": escalation.ID,
			"resolved_by":   escalation.ResolvedBy,
		},
		CreatedAt: time.Now(),
	}

	m.notificationMgr.Notify(ctx, notification)
}

// Stop stops the escalation manager
func (m *EscalationManager) Stop() {
	close(m.stopChan)
}

// ErrorEscalator provides easy error escalation for agents
type ErrorEscalator struct {
	manager      *EscalationManager
	source       string
	defaultLevel EscalationLevel
}

// NewErrorEscalator creates a new error escalator
func NewErrorEscalator(manager *EscalationManager, source string) *ErrorEscalator {
	return &ErrorEscalator{
		manager:      manager,
		source:       source,
		defaultLevel: EscalationLevelError,
	}
}

// SetDefaultLevel sets the default escalation level
func (e *ErrorEscalator) SetDefaultLevel(level EscalationLevel) {
	e.defaultLevel = level
}

// EscalateError escalates an error
func (e *ErrorEscalator) EscalateError(ctx context.Context, err error, context map[string]interface{}) (*Escalation, error) {
	return e.manager.Escalate(ctx, err.Error(), "Error occurred during agent execution", e.source, e.defaultLevel, err, context)
}

// EscalateWithDetails escalates with custom title and description
func (e *ErrorEscalator) EscalateWithDetails(ctx context.Context, title, description string, err error, level EscalationLevel, context map[string]interface{}) (*Escalation, error) {
	return e.manager.Escalate(ctx, title, description, e.source, level, err, context)
}
