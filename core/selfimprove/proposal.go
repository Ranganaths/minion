package selfimprove

import (
	"time"
)

// ImprovementType categorizes the type of improvement being proposed.
type ImprovementType string

const (
	// ImprovementTypeSystemPrompt is an improvement to the system prompt
	ImprovementTypeSystemPrompt ImprovementType = "system_prompt"

	// ImprovementTypeUserPrompt is an improvement to the user prompt template
	ImprovementTypeUserPrompt ImprovementType = "user_prompt"

	// ImprovementTypeFewShotExamples is an improvement to few-shot examples
	ImprovementTypeFewShotExamples ImprovementType = "few_shot_examples"

	// ImprovementTypeTaskDecomposition is an improvement to task decomposition
	ImprovementTypeTaskDecomposition ImprovementType = "task_decomposition"

	// ImprovementTypeWorkerSelection is an improvement to worker selection
	ImprovementTypeWorkerSelection ImprovementType = "worker_selection"

	// ImprovementTypeRetryStrategy is an improvement to retry strategies
	ImprovementTypeRetryStrategy ImprovementType = "retry_strategy"

	// ImprovementTypeToolUsage is an improvement to tool usage patterns
	ImprovementTypeToolUsage ImprovementType = "tool_usage"

	// ImprovementTypeToolConfig is an improvement to tool configuration
	ImprovementTypeToolConfig ImprovementType = "tool_config"
)

// ProposalStatus tracks the lifecycle of an improvement proposal.
type ProposalStatus string

const (
	// ProposalStatusPending is a proposal awaiting review
	ProposalStatusPending ProposalStatus = "pending"

	// ProposalStatusApproved is a proposal that has been approved
	ProposalStatusApproved ProposalStatus = "approved"

	// ProposalStatusRejected is a proposal that was rejected
	ProposalStatusRejected ProposalStatus = "rejected"

	// ProposalStatusApplied is a proposal that has been applied
	ProposalStatusApplied ProposalStatus = "applied"

	// ProposalStatusRolledBack is a proposal that was rolled back
	ProposalStatusRolledBack ProposalStatus = "rolled_back"

	// ProposalStatusTesting is a proposal being A/B tested
	ProposalStatusTesting ProposalStatus = "testing"

	// ProposalStatusExpired is a proposal that expired without approval
	ProposalStatusExpired ProposalStatus = "expired"
)

// ImprovementProposal represents a proposed improvement to agent behavior.
type ImprovementProposal struct {
	// ID is the unique identifier for this proposal
	ID string `json:"id"`

	// Strategy that generated this proposal
	Strategy LearningStrategy `json:"strategy"`

	// AgentID identifies which agent this proposal is for
	AgentID string `json:"agent_id"`

	// TaskType identifies which task type this applies to (optional)
	TaskType string `json:"task_type,omitempty"`

	// ImprovementType categorizes the type of change
	ImprovementType ImprovementType `json:"improvement_type"`

	// CurrentValue is the current prompt/behavior being improved
	CurrentValue string `json:"current_value"`

	// ProposedValue is the proposed new prompt/behavior
	ProposedValue string `json:"proposed_value"`

	// Rationale explains why this improvement is being proposed
	Rationale string `json:"rationale"`

	// SupportingExperiences lists the experience IDs used as evidence
	SupportingExperiences []string `json:"supporting_experiences"`

	// ExpectedImprovement is the estimated score gain (0.0-1.0)
	ExpectedImprovement float64 `json:"expected_improvement"`

	// Confidence in this proposal (0.0-1.0)
	Confidence float64 `json:"confidence"`

	// Description provides a human-readable summary
	Description string `json:"description,omitempty"`

	// Evidence provides supporting evidence for the proposal
	Evidence []string `json:"evidence,omitempty"`

	// SampleSize is the number of samples used to generate this proposal
	SampleSize int `json:"sample_size,omitempty"`

	// Status tracks the proposal lifecycle
	Status ProposalStatus `json:"status"`

	// Priority for processing (higher = more important)
	Priority int `json:"priority"`

	// CreatedAt is when the proposal was created
	CreatedAt time.Time `json:"created_at"`

	// ApprovedAt is when the proposal was approved
	ApprovedAt *time.Time `json:"approved_at,omitempty"`

	// ApprovedBy identifies who approved (e.g., "auto", "user-123")
	ApprovedBy *string `json:"approved_by,omitempty"`

	// AppliedAt is when the proposal was applied
	AppliedAt *time.Time `json:"applied_at,omitempty"`

	// RejectedAt is when the proposal was rejected
	RejectedAt *time.Time `json:"rejected_at,omitempty"`

	// RejectionReason explains why the proposal was rejected
	RejectionReason string `json:"rejection_reason,omitempty"`

	// RolledBackAt is when the proposal was rolled back
	RolledBackAt *time.Time `json:"rolled_back_at,omitempty"`

	// RollbackReason explains why the proposal was rolled back
	RollbackReason string `json:"rollback_reason,omitempty"`

	// ABTestResults contains A/B test metrics if tested
	ABTestResults *ABTestResults `json:"ab_test_results,omitempty"`

	// Metadata contains additional context
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ABTestResults contains the results of A/B testing a proposal.
type ABTestResults struct {
	// ControlGroup metrics (current behavior)
	ControlSamples int     `json:"control_samples"`
	ControlAvgScore float64 `json:"control_avg_score"`

	// TreatmentGroup metrics (proposed behavior)
	TreatmentSamples int     `json:"treatment_samples"`
	TreatmentAvgScore float64 `json:"treatment_avg_score"`

	// Statistical results
	Improvement     float64 `json:"improvement"`       // treatment - control
	PValue          float64 `json:"p_value"`           // statistical significance
	Significant     bool    `json:"significant"`       // p < threshold
	ConfidenceLevel float64 `json:"confidence_level"`  // typically 0.95

	// Decision
	Winner          string  `json:"winner"` // "control", "treatment", or "inconclusive"
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

// Approve marks the proposal as approved.
func (p *ImprovementProposal) Approve(approver string) {
	now := time.Now()
	p.Status = ProposalStatusApproved
	p.ApprovedAt = &now
	p.ApprovedBy = &approver
}

// Reject marks the proposal as rejected.
func (p *ImprovementProposal) Reject(reason string) {
	now := time.Now()
	p.Status = ProposalStatusRejected
	p.RejectedAt = &now
	p.RejectionReason = reason
}

// MarkApplied marks the proposal as applied.
func (p *ImprovementProposal) MarkApplied() {
	now := time.Now()
	p.Status = ProposalStatusApplied
	p.AppliedAt = &now
}

// Rollback marks the proposal as rolled back.
func (p *ImprovementProposal) Rollback(reason string) {
	now := time.Now()
	p.Status = ProposalStatusRolledBack
	p.RolledBackAt = &now
	p.RollbackReason = reason
}

// IsActionable returns true if the proposal can be acted upon.
func (p *ImprovementProposal) IsActionable() bool {
	return p.Status == ProposalStatusPending || p.Status == ProposalStatusApproved
}

// RequiresHumanApproval determines if human approval is needed based on impact.
func (p *ImprovementProposal) RequiresHumanApproval(config *SelfImprovementConfig) bool {
	if config.LearningConfig != nil && config.LearningConfig.RequireHumanApproval {
		return true
	}

	// High-impact changes require approval
	if p.ExpectedImprovement > config.RequireApprovalAbove {
		return true
	}

	// System prompt changes typically require approval
	if p.ImprovementType == ImprovementTypeSystemPrompt {
		return true
	}

	return false
}

// ProposalStore defines the interface for storing improvement proposals.
type ProposalStore interface {
	// Save stores a proposal
	Save(proposal *ImprovementProposal) error

	// Get retrieves a proposal by ID
	Get(id string) (*ImprovementProposal, error)

	// GetByAgent retrieves proposals for an agent
	GetByAgent(agentID string, status *ProposalStatus, limit int) ([]*ImprovementProposal, error)

	// GetPending retrieves pending proposals
	GetPending(limit int) ([]*ImprovementProposal, error)

	// Update updates a proposal
	Update(proposal *ImprovementProposal) error

	// Delete removes a proposal
	Delete(id string) error
}

// InMemoryProposalStore is an in-memory implementation of ProposalStore.
type InMemoryProposalStore struct {
	proposals map[string]*ImprovementProposal
}

// NewInMemoryProposalStore creates a new in-memory proposal store.
func NewInMemoryProposalStore() *InMemoryProposalStore {
	return &InMemoryProposalStore{
		proposals: make(map[string]*ImprovementProposal),
	}
}

// Save stores a proposal.
func (s *InMemoryProposalStore) Save(proposal *ImprovementProposal) error {
	s.proposals[proposal.ID] = proposal
	return nil
}

// Get retrieves a proposal by ID.
func (s *InMemoryProposalStore) Get(id string) (*ImprovementProposal, error) {
	if p, ok := s.proposals[id]; ok {
		return p, nil
	}
	return nil, nil
}

// GetByAgent retrieves proposals for an agent.
func (s *InMemoryProposalStore) GetByAgent(agentID string, status *ProposalStatus, limit int) ([]*ImprovementProposal, error) {
	var results []*ImprovementProposal
	for _, p := range s.proposals {
		if p.AgentID == agentID {
			if status == nil || p.Status == *status {
				results = append(results, p)
			}
		}
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	return results, nil
}

// GetPending retrieves pending proposals.
func (s *InMemoryProposalStore) GetPending(limit int) ([]*ImprovementProposal, error) {
	var results []*ImprovementProposal
	for _, p := range s.proposals {
		if p.Status == ProposalStatusPending {
			results = append(results, p)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// Update updates a proposal.
func (s *InMemoryProposalStore) Update(proposal *ImprovementProposal) error {
	s.proposals[proposal.ID] = proposal
	return nil
}

// Delete removes a proposal.
func (s *InMemoryProposalStore) Delete(id string) error {
	delete(s.proposals, id)
	return nil
}
