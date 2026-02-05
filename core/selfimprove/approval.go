package selfimprove

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ApprovalManager manages human approval workflows for improvements.
type ApprovalManager struct {
	mu sync.RWMutex

	// Proposal store
	proposalStore ProposalStore

	// Pending approvals by agent
	pendingByAgent map[string][]*ImprovementProposal

	// Approval handlers
	handlers []ApprovalHandler

	// Configuration
	config *ApprovalConfig

	// Notification channels
	notificationCh chan *ApprovalNotification
}

// ApprovalConfig configures the approval workflow.
type ApprovalConfig struct {
	// RequireApproval determines if human approval is required
	RequireApproval bool `json:"require_approval"`

	// AutoApproveBelow auto-approves changes with impact below this threshold
	AutoApproveBelow float64 `json:"auto_approve_below"`

	// AutoRejectAbove auto-rejects changes with risk above this threshold
	AutoRejectAbove float64 `json:"auto_reject_above"`

	// TimeoutDuration is how long to wait for approval before expiring
	TimeoutDuration time.Duration `json:"timeout_duration"`

	// MaxPendingPerAgent limits pending proposals per agent
	MaxPendingPerAgent int `json:"max_pending_per_agent"`

	// NotifyOnNewProposal enables notifications for new proposals
	NotifyOnNewProposal bool `json:"notify_on_new_proposal"`

	// NotifyOnExpiry enables notifications when proposals expire
	NotifyOnExpiry bool `json:"notify_on_expiry"`

	// AllowedApprovers lists user IDs allowed to approve (empty = all)
	AllowedApprovers []string `json:"allowed_approvers,omitempty"`
}

// ApprovalHandler handles approval workflow events.
type ApprovalHandler interface {
	// OnProposalCreated is called when a new proposal is created
	OnProposalCreated(ctx context.Context, proposal *ImprovementProposal) error

	// OnProposalApproved is called when a proposal is approved
	OnProposalApproved(ctx context.Context, proposal *ImprovementProposal) error

	// OnProposalRejected is called when a proposal is rejected
	OnProposalRejected(ctx context.Context, proposal *ImprovementProposal) error

	// OnProposalExpired is called when a proposal expires
	OnProposalExpired(ctx context.Context, proposal *ImprovementProposal) error
}

// ApprovalNotification represents a notification about an approval event.
type ApprovalNotification struct {
	Type       NotificationType      `json:"type"`
	Proposal   *ImprovementProposal  `json:"proposal"`
	Message    string                `json:"message"`
	Timestamp  time.Time             `json:"timestamp"`
	ActionURL  string                `json:"action_url,omitempty"`
}

// NotificationType represents the type of notification.
type NotificationType string

const (
	NotificationTypeNewProposal      NotificationType = "new_proposal"
	NotificationTypeApproved         NotificationType = "approved"
	NotificationTypeRejected         NotificationType = "rejected"
	NotificationTypeExpired          NotificationType = "expired"
	NotificationTypeAutoApproved     NotificationType = "auto_approved"
	NotificationTypeAutoRejected     NotificationType = "auto_rejected"
)

// ApprovalRequest represents a request for approval with context.
type ApprovalRequest struct {
	ProposalID     string                 `json:"proposal_id"`
	AgentID        string                 `json:"agent_id"`
	ProposalType   ImprovementType        `json:"proposal_type"`
	Strategy       LearningStrategy       `json:"strategy"`
	Description    string                 `json:"description"`
	CurrentValue   string                 `json:"current_value"`
	ProposedValue  string                 `json:"proposed_value"`
	Confidence     float64                `json:"confidence"`
	ExpectedImpact float64                `json:"expected_impact"`
	RiskScore      float64                `json:"risk_score"`
	Evidence       []string               `json:"evidence"`
	CreatedAt      time.Time              `json:"created_at"`
	ExpiresAt      time.Time              `json:"expires_at"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ApprovalResponse represents a response to an approval request.
type ApprovalResponse struct {
	ProposalID  string    `json:"proposal_id"`
	Approved    bool      `json:"approved"`
	ApproverID  string    `json:"approver_id"`
	Comment     string    `json:"comment,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// NewApprovalManager creates a new approval manager.
func NewApprovalManager(store ProposalStore, config *ApprovalConfig) *ApprovalManager {
	if config == nil {
		config = DefaultApprovalConfig()
	}

	return &ApprovalManager{
		proposalStore:  store,
		pendingByAgent: make(map[string][]*ImprovementProposal),
		handlers:       make([]ApprovalHandler, 0),
		config:         config,
		notificationCh: make(chan *ApprovalNotification, 100),
	}
}

// DefaultApprovalConfig returns default approval configuration.
func DefaultApprovalConfig() *ApprovalConfig {
	return &ApprovalConfig{
		RequireApproval:     true,
		AutoApproveBelow:    0.1,  // Auto-approve minor changes
		AutoRejectAbove:     0.9,  // Auto-reject high-risk changes
		TimeoutDuration:     24 * time.Hour,
		MaxPendingPerAgent:  5,
		NotifyOnNewProposal: true,
		NotifyOnExpiry:      true,
	}
}

// RegisterHandler registers an approval handler.
func (m *ApprovalManager) RegisterHandler(handler ApprovalHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

// SubmitForApproval submits a proposal for approval.
func (m *ApprovalManager) SubmitForApproval(ctx context.Context, proposal *ImprovementProposal) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check pending limit
	pending := m.pendingByAgent[proposal.AgentID]
	if len(pending) >= m.config.MaxPendingPerAgent {
		return fmt.Errorf("agent %s has too many pending proposals (%d)",
			proposal.AgentID, len(pending))
	}

	// Calculate risk score if not set
	if proposal.Confidence == 0 {
		proposal.Confidence = m.calculateConfidence(proposal)
	}

	// Check for auto-approval/rejection
	if !m.config.RequireApproval {
		proposal.Approve("system-auto")
		m.notify(NotificationTypeAutoApproved, proposal, "Auto-approved (approval not required)")
		return m.applyProposal(ctx, proposal)
	}

	riskScore := m.calculateRiskScore(proposal)

	// Auto-reject high risk
	if riskScore > m.config.AutoRejectAbove {
		proposal.Reject(fmt.Sprintf("Auto-rejected: risk score %.2f exceeds threshold %.2f",
			riskScore, m.config.AutoRejectAbove))
		m.notify(NotificationTypeAutoRejected, proposal, proposal.RejectionReason)
		return m.proposalStore.Save(proposal)
	}

	// Auto-approve low impact
	expectedImpact := m.calculateExpectedImpact(proposal)
	if expectedImpact < m.config.AutoApproveBelow && proposal.Confidence > 0.8 {
		proposal.Approve("system-auto")
		m.notify(NotificationTypeAutoApproved, proposal,
			fmt.Sprintf("Auto-approved: low impact (%.2f) with high confidence (%.2f)",
				expectedImpact, proposal.Confidence))
		return m.applyProposal(ctx, proposal)
	}

	// Requires human approval
	proposal.Status = ProposalStatusPending
	m.pendingByAgent[proposal.AgentID] = append(pending, proposal)

	if err := m.proposalStore.Save(proposal); err != nil {
		return err
	}

	// Notify handlers
	for _, handler := range m.handlers {
		if err := handler.OnProposalCreated(ctx, proposal); err != nil {
			// Log but don't fail
			continue
		}
	}

	m.notify(NotificationTypeNewProposal, proposal, "New improvement proposal requires approval")

	return nil
}

// Approve approves a proposal.
func (m *ApprovalManager) Approve(ctx context.Context, response *ApprovalResponse) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proposal, err := m.proposalStore.Get(response.ProposalID)
	if err != nil {
		return err
	}
	if proposal == nil {
		return fmt.Errorf("proposal not found: %s", response.ProposalID)
	}

	if proposal.Status != ProposalStatusPending {
		return fmt.Errorf("proposal is not pending: %s", proposal.Status)
	}

	// Check if approver is allowed
	if len(m.config.AllowedApprovers) > 0 {
		allowed := false
		for _, approver := range m.config.AllowedApprovers {
			if approver == response.ApproverID {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("user %s is not allowed to approve proposals", response.ApproverID)
		}
	}

	proposal.Approve(response.ApproverID)

	// Remove from pending
	m.removePending(proposal)

	if err := m.proposalStore.Save(proposal); err != nil {
		return err
	}

	// Notify handlers
	for _, handler := range m.handlers {
		if err := handler.OnProposalApproved(ctx, proposal); err != nil {
			continue
		}
	}

	m.notify(NotificationTypeApproved, proposal,
		fmt.Sprintf("Proposal approved by %s", response.ApproverID))

	return m.applyProposal(ctx, proposal)
}

// Reject rejects a proposal.
func (m *ApprovalManager) Reject(ctx context.Context, response *ApprovalResponse) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proposal, err := m.proposalStore.Get(response.ProposalID)
	if err != nil {
		return err
	}
	if proposal == nil {
		return fmt.Errorf("proposal not found: %s", response.ProposalID)
	}

	if proposal.Status != ProposalStatusPending {
		return fmt.Errorf("proposal is not pending: %s", proposal.Status)
	}

	proposal.Reject(response.Comment)

	// Remove from pending
	m.removePending(proposal)

	if err := m.proposalStore.Save(proposal); err != nil {
		return err
	}

	// Notify handlers
	for _, handler := range m.handlers {
		if err := handler.OnProposalRejected(ctx, proposal); err != nil {
			continue
		}
	}

	m.notify(NotificationTypeRejected, proposal,
		fmt.Sprintf("Proposal rejected: %s", response.Comment))

	return nil
}

// GetPendingApprovals returns pending approvals for review.
func (m *ApprovalManager) GetPendingApprovals(agentID string, limit int) ([]*ApprovalRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var proposals []*ImprovementProposal

	if agentID != "" {
		proposals = m.pendingByAgent[agentID]
	} else {
		// Get all pending
		for _, agentProposals := range m.pendingByAgent {
			proposals = append(proposals, agentProposals...)
		}
	}

	if limit > 0 && len(proposals) > limit {
		proposals = proposals[:limit]
	}

	requests := make([]*ApprovalRequest, len(proposals))
	for i, p := range proposals {
		requests[i] = m.toApprovalRequest(p)
	}

	return requests, nil
}

// GetApprovalRequest returns a specific approval request.
func (m *ApprovalManager) GetApprovalRequest(proposalID string) (*ApprovalRequest, error) {
	proposal, err := m.proposalStore.Get(proposalID)
	if err != nil {
		return nil, err
	}
	if proposal == nil {
		return nil, nil
	}
	return m.toApprovalRequest(proposal), nil
}

// ExpireOldProposals expires proposals that have exceeded timeout.
func (m *ApprovalManager) ExpireOldProposals(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	expired := 0
	cutoff := time.Now().Add(-m.config.TimeoutDuration)

	for agentID, proposals := range m.pendingByAgent {
		remaining := make([]*ImprovementProposal, 0)

		for _, p := range proposals {
			if p.CreatedAt.Before(cutoff) {
				p.Status = ProposalStatusExpired
				if err := m.proposalStore.Save(p); err != nil {
					continue
				}

				// Notify handlers
				for _, handler := range m.handlers {
					handler.OnProposalExpired(ctx, p)
				}

				if m.config.NotifyOnExpiry {
					m.notify(NotificationTypeExpired, p, "Proposal expired without approval")
				}

				expired++
			} else {
				remaining = append(remaining, p)
			}
		}

		m.pendingByAgent[agentID] = remaining
	}

	return expired, nil
}

// Notifications returns the notification channel.
func (m *ApprovalManager) Notifications() <-chan *ApprovalNotification {
	return m.notificationCh
}

// GetConfig returns the approval configuration.
func (m *ApprovalManager) GetConfig() *ApprovalConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig updates the approval configuration.
func (m *ApprovalManager) UpdateConfig(config *ApprovalConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// Stats returns approval statistics.
func (m *ApprovalManager) Stats() *ApprovalStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalPending := 0
	for _, proposals := range m.pendingByAgent {
		totalPending += len(proposals)
	}

	return &ApprovalStats{
		TotalPending:   totalPending,
		AgentsWithPending: len(m.pendingByAgent),
	}
}

// ApprovalStats contains approval statistics.
type ApprovalStats struct {
	TotalPending      int `json:"total_pending"`
	AgentsWithPending int `json:"agents_with_pending"`
}

// removePending removes a proposal from pending lists.
func (m *ApprovalManager) removePending(proposal *ImprovementProposal) {
	pending := m.pendingByAgent[proposal.AgentID]
	for i, p := range pending {
		if p.ID == proposal.ID {
			m.pendingByAgent[proposal.AgentID] = append(pending[:i], pending[i+1:]...)
			break
		}
	}
}

// applyProposal applies an approved proposal.
func (m *ApprovalManager) applyProposal(ctx context.Context, proposal *ImprovementProposal) error {
	proposal.MarkApplied()
	return m.proposalStore.Save(proposal)
}

// notify sends a notification.
func (m *ApprovalManager) notify(notifType NotificationType, proposal *ImprovementProposal, message string) {
	select {
	case m.notificationCh <- &ApprovalNotification{
		Type:      notifType,
		Proposal:  proposal,
		Message:   message,
		Timestamp: time.Now(),
	}:
	default:
		// Channel full, skip notification
	}
}

// toApprovalRequest converts a proposal to an approval request.
func (m *ApprovalManager) toApprovalRequest(p *ImprovementProposal) *ApprovalRequest {
	return &ApprovalRequest{
		ProposalID:     p.ID,
		AgentID:        p.AgentID,
		ProposalType:   p.ImprovementType,
		Strategy:       p.Strategy,
		Description:    p.Description,
		CurrentValue:   p.CurrentValue,
		ProposedValue:  p.ProposedValue,
		Confidence:     p.Confidence,
		ExpectedImpact: m.calculateExpectedImpact(p),
		RiskScore:      m.calculateRiskScore(p),
		Evidence:       p.Evidence,
		CreatedAt:      p.CreatedAt,
		ExpiresAt:      p.CreatedAt.Add(m.config.TimeoutDuration),
	}
}

// calculateConfidence calculates confidence score for a proposal.
func (m *ApprovalManager) calculateConfidence(p *ImprovementProposal) float64 {
	confidence := 0.5 // Base confidence

	// More evidence increases confidence
	if len(p.Evidence) > 3 {
		confidence += 0.2
	} else if len(p.Evidence) > 0 {
		confidence += 0.1
	}

	// Higher sample size increases confidence
	if p.SampleSize > 100 {
		confidence += 0.2
	} else if p.SampleSize > 50 {
		confidence += 0.1
	}

	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// calculateRiskScore calculates risk score for a proposal.
func (m *ApprovalManager) calculateRiskScore(p *ImprovementProposal) float64 {
	risk := 0.3 // Base risk

	// System prompt changes are higher risk
	if p.ImprovementType == ImprovementTypeSystemPrompt {
		risk += 0.3
	}

	// Tool configuration changes are higher risk
	if p.ImprovementType == ImprovementTypeToolConfig {
		risk += 0.2
	}

	// Low confidence increases risk
	if p.Confidence < 0.5 {
		risk += 0.2
	}

	// Large changes are higher risk
	if len(p.ProposedValue) > len(p.CurrentValue)*2 {
		risk += 0.1
	}

	if risk > 1.0 {
		risk = 1.0
	}

	return risk
}

// calculateExpectedImpact calculates expected impact of a proposal.
func (m *ApprovalManager) calculateExpectedImpact(p *ImprovementProposal) float64 {
	impact := 0.5 // Base impact

	// System prompt changes have high impact
	if p.ImprovementType == ImprovementTypeSystemPrompt {
		impact = 0.8
	}

	// Few-shot examples have moderate impact
	if p.ImprovementType == ImprovementTypeFewShotExamples {
		impact = 0.4
	}

	// Weight by confidence
	return impact * p.Confidence
}

// LoggingApprovalHandler is a simple handler that logs approval events.
type LoggingApprovalHandler struct {
	LogFunc func(format string, args ...interface{})
}

// OnProposalCreated logs proposal creation.
func (h *LoggingApprovalHandler) OnProposalCreated(ctx context.Context, p *ImprovementProposal) error {
	if h.LogFunc != nil {
		h.LogFunc("New proposal created: %s for agent %s", p.ID, p.AgentID)
	}
	return nil
}

// OnProposalApproved logs proposal approval.
func (h *LoggingApprovalHandler) OnProposalApproved(ctx context.Context, p *ImprovementProposal) error {
	if h.LogFunc != nil {
		h.LogFunc("Proposal approved: %s by %s", p.ID, *p.ApprovedBy)
	}
	return nil
}

// OnProposalRejected logs proposal rejection.
func (h *LoggingApprovalHandler) OnProposalRejected(ctx context.Context, p *ImprovementProposal) error {
	if h.LogFunc != nil {
		h.LogFunc("Proposal rejected: %s - %s", p.ID, p.RejectionReason)
	}
	return nil
}

// OnProposalExpired logs proposal expiry.
func (h *LoggingApprovalHandler) OnProposalExpired(ctx context.Context, p *ImprovementProposal) error {
	if h.LogFunc != nil {
		h.LogFunc("Proposal expired: %s", p.ID)
	}
	return nil
}

// WebhookApprovalHandler sends webhook notifications for approval events.
type WebhookApprovalHandler struct {
	WebhookURL string
	Secret     string
}

// OnProposalCreated sends webhook for new proposal.
func (h *WebhookApprovalHandler) OnProposalCreated(ctx context.Context, p *ImprovementProposal) error {
	// Implementation would send HTTP POST to webhook URL
	return nil
}

// OnProposalApproved sends webhook for approved proposal.
func (h *WebhookApprovalHandler) OnProposalApproved(ctx context.Context, p *ImprovementProposal) error {
	return nil
}

// OnProposalRejected sends webhook for rejected proposal.
func (h *WebhookApprovalHandler) OnProposalRejected(ctx context.Context, p *ImprovementProposal) error {
	return nil
}

// OnProposalExpired sends webhook for expired proposal.
func (h *WebhookApprovalHandler) OnProposalExpired(ctx context.Context, p *ImprovementProposal) error {
	return nil
}
