// Package humanloop provides human-in-the-loop workflow capabilities including
// approval gates, escalation policies, and interactive agent steering.
package humanloop

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ApprovalStatus represents the status of an approval request
type ApprovalStatus string

const (
	ApprovalStatusPending   ApprovalStatus = "pending"
	ApprovalStatusApproved  ApprovalStatus = "approved"
	ApprovalStatusRejected  ApprovalStatus = "rejected"
	ApprovalStatusTimeout   ApprovalStatus = "timeout"
	ApprovalStatusCancelled ApprovalStatus = "cancelled"
)

// ApprovalGate represents a human approval checkpoint
type ApprovalGate struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	Description   string               `json:"description,omitempty"`
	RequiredRoles []string             `json:"required_roles,omitempty"` // Roles allowed to approve
	Timeout       time.Duration        `json:"timeout,omitempty"`        // Auto-reject after timeout
	AutoApprove   *AutoApproveRule     `json:"auto_approve,omitempty"`   // Conditions for auto-approval
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// AutoApproveRule defines conditions for automatic approval
type AutoApproveRule struct {
	// Conditions that must be met for auto-approval
	Conditions []ApprovalCondition `json:"conditions"`

	// RequireAll requires all conditions to be true (AND logic)
	// If false, any condition being true triggers approval (OR logic)
	RequireAll bool `json:"require_all"`
}

// ApprovalCondition defines a single auto-approval condition
type ApprovalCondition struct {
	Field    string      `json:"field"`    // Field in context to check
	Operator string      `json:"operator"` // eq, ne, gt, lt, contains, exists
	Value    interface{} `json:"value"`
}

// ApprovalRequest represents a pending approval
type ApprovalRequest struct {
	ID          string                 `json:"id"`
	GateID      string                 `json:"gate_id"`
	GateName    string                 `json:"gate_name"`
	WorkflowID  string                 `json:"workflow_id,omitempty"`
	TaskID      string                 `json:"task_id,omitempty"`
	AgentID     string                 `json:"agent_id,omitempty"`
	Context     map[string]interface{} `json:"context"`
	Status      ApprovalStatus         `json:"status"`
	RequestedAt time.Time              `json:"requested_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
	ResolvedBy  string                 `json:"resolved_by,omitempty"`
	Comment     string                 `json:"comment,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ApprovalResult contains the result of an approval request
type ApprovalResult struct {
	Approved   bool                   `json:"approved"`
	ResolvedBy string                 `json:"resolved_by,omitempty"`
	Comment    string                 `json:"comment,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ApprovalStore stores and retrieves approval requests
type ApprovalStore interface {
	Create(ctx context.Context, request *ApprovalRequest) error
	Get(ctx context.Context, id string) (*ApprovalRequest, error)
	Update(ctx context.Context, request *ApprovalRequest) error
	ListPending(ctx context.Context, filter *ApprovalFilter) ([]*ApprovalRequest, error)
	Delete(ctx context.Context, id string) error
}

// ApprovalFilter filters approval requests
type ApprovalFilter struct {
	GateID     string
	WorkflowID string
	AgentID    string
	Status     ApprovalStatus
	Limit      int
	Offset     int
}

// ApprovalWorkflow manages the approval process
type ApprovalWorkflow struct {
	store       ApprovalStore
	notifiers   []Notifier
	timeout     time.Duration
	gates       map[string]*ApprovalGate
	listeners   map[string][]chan *ApprovalResult
	mu          sync.RWMutex
}

// ApprovalWorkflowConfig configures the approval workflow
type ApprovalWorkflowConfig struct {
	Store          ApprovalStore
	Notifiers      []Notifier
	DefaultTimeout time.Duration
}

// NewApprovalWorkflow creates a new approval workflow
func NewApprovalWorkflow(config ApprovalWorkflowConfig) *ApprovalWorkflow {
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 24 * time.Hour
	}

	return &ApprovalWorkflow{
		store:     config.Store,
		notifiers: config.Notifiers,
		timeout:   config.DefaultTimeout,
		gates:     make(map[string]*ApprovalGate),
		listeners: make(map[string][]chan *ApprovalResult),
	}
}

// RegisterGate registers an approval gate
func (a *ApprovalWorkflow) RegisterGate(gate *ApprovalGate) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.gates[gate.ID] = gate
}

// GetGate returns a registered gate
func (a *ApprovalWorkflow) GetGate(id string) (*ApprovalGate, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	gate, ok := a.gates[id]
	return gate, ok
}

// RequestApproval creates a new approval request and waits for resolution
func (a *ApprovalWorkflow) RequestApproval(ctx context.Context, gateID string, context map[string]interface{}) (*ApprovalResult, error) {
	request, err := a.CreateApprovalRequest(ctx, gateID, context)
	if err != nil {
		return nil, err
	}

	return a.WaitForApproval(ctx, request.ID)
}

// CreateApprovalRequest creates a new approval request without waiting
func (a *ApprovalWorkflow) CreateApprovalRequest(ctx context.Context, gateID string, approvalContext map[string]interface{}) (*ApprovalRequest, error) {
	gate, ok := a.GetGate(gateID)
	if !ok {
		return nil, fmt.Errorf("gate %s not found", gateID)
	}

	// Check auto-approval rules
	if gate.AutoApprove != nil && a.checkAutoApprove(gate.AutoApprove, approvalContext) {
		return &ApprovalRequest{
			ID:          uuid.New().String(),
			GateID:      gateID,
			GateName:    gate.Name,
			Context:     approvalContext,
			Status:      ApprovalStatusApproved,
			RequestedAt: time.Now(),
		}, nil
	}

	// Create request
	now := time.Now()
	timeout := a.timeout
	if gate.Timeout > 0 {
		timeout = gate.Timeout
	}
	expiresAt := now.Add(timeout)

	request := &ApprovalRequest{
		ID:          uuid.New().String(),
		GateID:      gateID,
		GateName:    gate.Name,
		Context:     approvalContext,
		Status:      ApprovalStatusPending,
		RequestedAt: now,
		ExpiresAt:   &expiresAt,
		Metadata:    make(map[string]interface{}),
	}

	// Store request
	if a.store != nil {
		if err := a.store.Create(ctx, request); err != nil {
			return nil, fmt.Errorf("failed to store approval request: %w", err)
		}
	}

	// Send notifications
	a.sendNotifications(ctx, request, gate)

	return request, nil
}

// WaitForApproval waits for an approval request to be resolved
func (a *ApprovalWorkflow) WaitForApproval(ctx context.Context, requestID string) (*ApprovalResult, error) {
	// Create a listener channel
	resultChan := make(chan *ApprovalResult, 1)

	a.mu.Lock()
	a.listeners[requestID] = append(a.listeners[requestID], resultChan)
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		delete(a.listeners, requestID)
		a.mu.Unlock()
	}()

	// Get the request to check timeout
	var request *ApprovalRequest
	var err error
	if a.store != nil {
		request, err = a.store.Get(ctx, requestID)
		if err != nil {
			return nil, err
		}
	}

	// Set up timeout
	var timeoutChan <-chan time.Time
	if request != nil && request.ExpiresAt != nil {
		remaining := time.Until(*request.ExpiresAt)
		if remaining > 0 {
			timeoutChan = time.After(remaining)
		} else {
			// Already expired
			return nil, errors.New("approval request has expired")
		}
	}

	// Wait for result, timeout, or context cancellation
	select {
	case result := <-resultChan:
		return result, nil
	case <-timeoutChan:
		// Mark as timeout
		if a.store != nil && request != nil {
			now := time.Now()
			request.Status = ApprovalStatusTimeout
			request.ResolvedAt = &now
			a.store.Update(ctx, request)
		}
		return nil, errors.New("approval request timed out")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Approve approves an approval request
func (a *ApprovalWorkflow) Approve(ctx context.Context, requestID, userID, comment string) error {
	return a.resolve(ctx, requestID, userID, comment, ApprovalStatusApproved)
}

// Reject rejects an approval request
func (a *ApprovalWorkflow) Reject(ctx context.Context, requestID, userID, comment string) error {
	return a.resolve(ctx, requestID, userID, comment, ApprovalStatusRejected)
}

// resolve resolves an approval request
func (a *ApprovalWorkflow) resolve(ctx context.Context, requestID, userID, comment string, status ApprovalStatus) error {
	var request *ApprovalRequest

	if a.store != nil {
		var err error
		request, err = a.store.Get(ctx, requestID)
		if err != nil {
			return fmt.Errorf("failed to get approval request: %w", err)
		}
		if request == nil {
			return errors.New("approval request not found")
		}
	}

	if request != nil {
		if request.Status != ApprovalStatusPending {
			return fmt.Errorf("approval request is already %s", request.Status)
		}

		// Check if expired
		if request.ExpiresAt != nil && time.Now().After(*request.ExpiresAt) {
			return errors.New("approval request has expired")
		}

		// Update request
		now := time.Now()
		request.Status = status
		request.ResolvedAt = &now
		request.ResolvedBy = userID
		request.Comment = comment

		if a.store != nil {
			if err := a.store.Update(ctx, request); err != nil {
				return fmt.Errorf("failed to update approval request: %w", err)
			}
		}
	}

	// Notify listeners
	result := &ApprovalResult{
		Approved:   status == ApprovalStatusApproved,
		ResolvedBy: userID,
		Comment:    comment,
	}

	a.mu.RLock()
	listeners := a.listeners[requestID]
	a.mu.RUnlock()

	for _, ch := range listeners {
		select {
		case ch <- result:
		default:
		}
	}

	return nil
}

// Cancel cancels an approval request
func (a *ApprovalWorkflow) Cancel(ctx context.Context, requestID string) error {
	return a.resolve(ctx, requestID, "", "cancelled", ApprovalStatusCancelled)
}

// GetPendingApprovals returns all pending approval requests
func (a *ApprovalWorkflow) GetPendingApprovals(ctx context.Context, filter *ApprovalFilter) ([]*ApprovalRequest, error) {
	if a.store == nil {
		return nil, nil
	}

	if filter == nil {
		filter = &ApprovalFilter{}
	}
	filter.Status = ApprovalStatusPending

	return a.store.ListPending(ctx, filter)
}

// checkAutoApprove checks if auto-approval conditions are met
func (a *ApprovalWorkflow) checkAutoApprove(rule *AutoApproveRule, context map[string]interface{}) bool {
	if len(rule.Conditions) == 0 {
		return false
	}

	for _, cond := range rule.Conditions {
		met := a.checkCondition(cond, context)
		if rule.RequireAll && !met {
			return false
		}
		if !rule.RequireAll && met {
			return true
		}
	}

	return rule.RequireAll
}

// checkCondition checks a single condition
func (a *ApprovalWorkflow) checkCondition(cond ApprovalCondition, context map[string]interface{}) bool {
	value, exists := context[cond.Field]

	switch cond.Operator {
	case "exists":
		return exists
	case "not_exists":
		return !exists
	case "eq":
		return value == cond.Value
	case "ne":
		return value != cond.Value
	case "gt", "lt", "gte", "lte":
		return a.compareNumeric(value, cond.Value, cond.Operator)
	case "contains":
		if str, ok := value.(string); ok {
			if substr, ok := cond.Value.(string); ok {
				return containsString(str, substr)
			}
		}
		return false
	default:
		return false
	}
}

// compareNumeric compares numeric values
func (a *ApprovalWorkflow) compareNumeric(a1, b interface{}, op string) bool {
	var av, bv float64

	switch v := a1.(type) {
	case int:
		av = float64(v)
	case int64:
		av = float64(v)
	case float64:
		av = v
	default:
		return false
	}

	switch v := b.(type) {
	case int:
		bv = float64(v)
	case int64:
		bv = float64(v)
	case float64:
		bv = v
	default:
		return false
	}

	switch op {
	case "gt":
		return av > bv
	case "lt":
		return av < bv
	case "gte":
		return av >= bv
	case "lte":
		return av <= bv
	default:
		return false
	}
}

// sendNotifications sends notifications about the approval request
func (a *ApprovalWorkflow) sendNotifications(ctx context.Context, request *ApprovalRequest, gate *ApprovalGate) {
	notification := &Notification{
		ID:        uuid.New().String(),
		Type:      NotificationTypeApprovalRequired,
		Title:     fmt.Sprintf("Approval Required: %s", gate.Name),
		Message:   gate.Description,
		Priority:  NotificationPriorityHigh,
		Metadata:  map[string]interface{}{"request_id": request.ID, "gate_id": gate.ID},
		CreatedAt: time.Now(),
	}

	for _, notifier := range a.notifiers {
		go func(n Notifier) {
			_ = n.Notify(ctx, notification)
		}(notifier)
	}
}

// Helper functions

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// MemoryApprovalStore is an in-memory implementation of ApprovalStore
type MemoryApprovalStore struct {
	requests map[string]*ApprovalRequest
	mu       sync.RWMutex
}

// NewMemoryApprovalStore creates a new in-memory approval store
func NewMemoryApprovalStore() *MemoryApprovalStore {
	return &MemoryApprovalStore{
		requests: make(map[string]*ApprovalRequest),
	}
}

func (s *MemoryApprovalStore) Create(ctx context.Context, request *ApprovalRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests[request.ID] = request
	return nil
}

func (s *MemoryApprovalStore) Get(ctx context.Context, id string) (*ApprovalRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	request, ok := s.requests[id]
	if !ok {
		return nil, nil
	}
	return request, nil
}

func (s *MemoryApprovalStore) Update(ctx context.Context, request *ApprovalRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests[request.ID] = request
	return nil
}

func (s *MemoryApprovalStore) ListPending(ctx context.Context, filter *ApprovalFilter) ([]*ApprovalRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]*ApprovalRequest, 0)
	for _, req := range s.requests {
		if filter.Status != "" && req.Status != filter.Status {
			continue
		}
		if filter.GateID != "" && req.GateID != filter.GateID {
			continue
		}
		if filter.WorkflowID != "" && req.WorkflowID != filter.WorkflowID {
			continue
		}
		if filter.AgentID != "" && req.AgentID != filter.AgentID {
			continue
		}
		results = append(results, req)
	}

	// Apply pagination
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results, nil
}

func (s *MemoryApprovalStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.requests, id)
	return nil
}

// Ensure MemoryApprovalStore implements ApprovalStore
var _ ApprovalStore = (*MemoryApprovalStore)(nil)
