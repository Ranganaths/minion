package selfimprove

import (
	"context"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Enabled {
		t.Error("Expected Enabled to be false by default")
	}

	if config.LearningConfig == nil {
		t.Error("Expected LearningConfig to be set")
	}

	if config.LearnAfterEveryN != 100 {
		t.Errorf("Expected LearnAfterEveryN to be 100, got %d", config.LearnAfterEveryN)
	}

	if config.RegressionThreshold != 0.1 {
		t.Errorf("Expected RegressionThreshold to be 0.1, got %f", config.RegressionThreshold)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *SelfImprovementConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid regression threshold",
			config: &SelfImprovementConfig{
				RegressionThreshold: 1.5,
				LearningConfig:      DefaultLearningConfig(),
			},
			wantErr: true,
		},
		{
			name: "invalid approval threshold",
			config: &SelfImprovementConfig{
				RequireApprovalAbove: -0.5,
				LearningConfig:       DefaultLearningConfig(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGlobalKillSwitch(t *testing.T) {
	// Start enabled
	EnableGlobally()
	if IsGloballyDisabled() {
		t.Error("Expected globally enabled")
	}

	// Disable
	DisableGlobally()
	if !IsGloballyDisabled() {
		t.Error("Expected globally disabled")
	}

	// Re-enable
	EnableGlobally()
	if IsGloballyDisabled() {
		t.Error("Expected globally re-enabled")
	}
}

func TestExperienceIsSuccessful(t *testing.T) {
	exp := &Experience{
		Success: true,
		Score:   0.8,
	}

	if !exp.IsSuccessful(0.7) {
		t.Error("Expected experience to be successful")
	}

	if exp.IsSuccessful(0.9) {
		t.Error("Expected experience to not meet higher threshold")
	}

	exp.Success = false
	if exp.IsSuccessful(0.5) {
		t.Error("Expected unsuccessful experience to return false")
	}
}

func TestExperienceIsFailed(t *testing.T) {
	exp := &Experience{
		Success: false,
		Score:   0.3,
	}

	if !exp.IsFailed(0.4) {
		t.Error("Expected experience to be failed")
	}

	exp.Success = true
	exp.Score = 0.2
	if !exp.IsFailed(0.4) {
		t.Error("Expected low-scoring experience to be failed")
	}
}

func TestExperienceHasHumanFeedback(t *testing.T) {
	exp := &Experience{}

	if exp.HasHumanFeedback() {
		t.Error("Expected no human feedback")
	}

	rating := 0.8
	exp.HumanRating = &rating
	if !exp.HasHumanFeedback() {
		t.Error("Expected to detect human rating")
	}

	exp.HumanRating = nil
	feedback := "Good job"
	exp.HumanFeedback = &feedback
	if !exp.HasHumanFeedback() {
		t.Error("Expected to detect human feedback")
	}

	exp.HumanFeedback = nil
	correction := "Better answer"
	exp.Correction = &correction
	if !exp.HasHumanFeedback() {
		t.Error("Expected to detect correction")
	}
}

func TestDefaultExperienceFormatter(t *testing.T) {
	formatter := NewDefaultExperienceFormatter()

	experiences := []*Experience{
		{
			ID:       "exp-1",
			Input:    "What is 2+2?",
			Output:   "4",
			Score:    0.9,
			TaskType: "math",
		},
		{
			ID:       "exp-2",
			Input:    "What is 3+3?",
			Output:   "6",
			Score:    0.85,
			TaskType: "math",
		},
	}

	// Test FormatAsFewShot
	fewShot := formatter.FormatAsFewShot(experiences)
	if fewShot == "" {
		t.Error("Expected non-empty few-shot format")
	}
	if len(fewShot) < 50 {
		t.Error("Few-shot format seems too short")
	}

	// Test empty list
	emptyFewShot := formatter.FormatAsFewShot([]*Experience{})
	if emptyFewShot != "" {
		t.Error("Expected empty string for empty list")
	}

	// Test FormatAsContext
	context := formatter.FormatAsContext(experiences)
	if context == "" {
		t.Error("Expected non-empty context format")
	}
}

func TestProposalLifecycle(t *testing.T) {
	proposal := &ImprovementProposal{
		ID:              "prop-1",
		Strategy:        StrategyFewShot,
		AgentID:         "agent-1",
		ImprovementType: ImprovementTypeFewShotExamples,
		Status:          ProposalStatusPending,
	}

	// Test approval
	proposal.Approve("user-1")
	if proposal.Status != ProposalStatusApproved {
		t.Error("Expected approved status")
	}
	if proposal.ApprovedAt == nil {
		t.Error("Expected ApprovedAt to be set")
	}
	if proposal.ApprovedBy == nil || *proposal.ApprovedBy != "user-1" {
		t.Error("Expected ApprovedBy to be 'user-1'")
	}

	// Test applied
	proposal.MarkApplied()
	if proposal.Status != ProposalStatusApplied {
		t.Error("Expected applied status")
	}
	if proposal.AppliedAt == nil {
		t.Error("Expected AppliedAt to be set")
	}

	// Test rollback
	proposal.Rollback("Performance regression")
	if proposal.Status != ProposalStatusRolledBack {
		t.Error("Expected rolled back status")
	}
	if proposal.RollbackReason != "Performance regression" {
		t.Error("Expected rollback reason to be set")
	}
}

func TestProposalReject(t *testing.T) {
	proposal := &ImprovementProposal{
		ID:     "prop-1",
		Status: ProposalStatusPending,
	}

	proposal.Reject("Not useful")
	if proposal.Status != ProposalStatusRejected {
		t.Error("Expected rejected status")
	}
	if proposal.RejectionReason != "Not useful" {
		t.Error("Expected rejection reason to be set")
	}
}

func TestProposalIsActionable(t *testing.T) {
	tests := []struct {
		status     ProposalStatus
		actionable bool
	}{
		{ProposalStatusPending, true},
		{ProposalStatusApproved, true},
		{ProposalStatusRejected, false},
		{ProposalStatusApplied, false},
		{ProposalStatusRolledBack, false},
	}

	for _, tt := range tests {
		proposal := &ImprovementProposal{Status: tt.status}
		if proposal.IsActionable() != tt.actionable {
			t.Errorf("IsActionable() for status %s = %v, want %v",
				tt.status, proposal.IsActionable(), tt.actionable)
		}
	}
}

func TestInMemoryProposalStore(t *testing.T) {
	store := NewInMemoryProposalStore()

	proposal := &ImprovementProposal{
		ID:      "prop-1",
		AgentID: "agent-1",
		Status:  ProposalStatusPending,
	}

	// Save
	err := store.Save(proposal)
	if err != nil {
		t.Errorf("Save() error = %v", err)
	}

	// Get
	retrieved, err := store.Get("prop-1")
	if err != nil {
		t.Errorf("Get() error = %v", err)
	}
	if retrieved == nil || retrieved.ID != "prop-1" {
		t.Error("Expected to retrieve proposal")
	}

	// Get non-existent
	notFound, _ := store.Get("non-existent")
	if notFound != nil {
		t.Error("Expected nil for non-existent proposal")
	}

	// GetPending
	pending, err := store.GetPending(10)
	if err != nil {
		t.Errorf("GetPending() error = %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending, got %d", len(pending))
	}

	// Delete
	err = store.Delete("prop-1")
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	deleted, _ := store.Get("prop-1")
	if deleted != nil {
		t.Error("Expected proposal to be deleted")
	}
}

func TestLearningEngine(t *testing.T) {
	ctx := context.Background()
	config := DefaultLearningConfig()
	config.MinExperiencesForLearn = 2

	// Create mock experience store
	store := &mockExperienceStore{
		experiences: make(map[string]*Experience),
	}

	engine := NewLearningEngine(config, store)

	// Add some experiences
	for i := 0; i < 5; i++ {
		exp := &Experience{
			ID:        "exp-" + string(rune('0'+i)),
			AgentID:   "agent-1",
			TaskType:  "math",
			Success:   true,
			Score:     0.8,
			Timestamp: time.Now(),
		}
		store.experiences[exp.ID] = exp
	}

	// Test TriggerLearning (no strategies registered, should return no proposals)
	err := engine.TriggerLearning(ctx, "agent-1")
	if err != nil {
		t.Errorf("TriggerLearning() error = %v", err)
	}

	// Test with global disable
	DisableGlobally()
	err = engine.TriggerLearning(ctx, "agent-1")
	if err == nil {
		t.Error("Expected error when globally disabled")
	}
	EnableGlobally()
}

func TestSelfImprovementMetrics(t *testing.T) {
	metrics := NewSelfImprovementMetrics("agent-1")

	// Record executions
	metrics.RecordExecution(true, 0.8, 100*time.Millisecond)
	metrics.RecordExecution(true, 0.9, 150*time.Millisecond)
	metrics.RecordExecution(false, 0.3, 200*time.Millisecond)

	stats := metrics.GetStats()

	if stats["total_executions"].(int64) != 3 {
		t.Errorf("Expected 3 total executions, got %v", stats["total_executions"])
	}

	if stats["successful_executions"].(int64) != 2 {
		t.Errorf("Expected 2 successful executions, got %v", stats["successful_executions"])
	}

	// Record learning cycle
	metrics.RecordLearningCycle()
	stats = metrics.GetStats()
	if stats["learning_cycles"].(int64) != 1 {
		t.Errorf("Expected 1 learning cycle, got %v", stats["learning_cycles"])
	}

	// Record improvement
	metrics.RecordImprovement(true)
	stats = metrics.GetStats()
	if stats["improvements_applied"].(int64) != 1 {
		t.Errorf("Expected 1 improvement, got %v", stats["improvements_applied"])
	}

	// Record rollback
	metrics.RecordRollback()
	stats = metrics.GetStats()
	if stats["rollbacks"].(int64) != 1 {
		t.Errorf("Expected 1 rollback, got %v", stats["rollbacks"])
	}
}

// Mock experience store for testing
type mockExperienceStore struct {
	experiences map[string]*Experience
}

func (s *mockExperienceStore) Store(ctx context.Context, exp *Experience) error {
	s.experiences[exp.ID] = exp
	return nil
}

func (s *mockExperienceStore) Get(ctx context.Context, id string) (*Experience, error) {
	return s.experiences[id], nil
}

func (s *mockExperienceStore) GetByTraceID(ctx context.Context, traceID string) (*Experience, error) {
	for _, exp := range s.experiences {
		if exp.TraceID == traceID {
			return exp, nil
		}
	}
	return nil, nil
}

func (s *mockExperienceStore) Query(ctx context.Context, query *ExperienceQuery) ([]*Experience, error) {
	var result []*Experience
	for _, exp := range s.experiences {
		result = append(result, exp)
	}
	return result, nil
}

func (s *mockExperienceStore) GetByAgent(ctx context.Context, agentID string, limit int) ([]*Experience, error) {
	var result []*Experience
	for _, exp := range s.experiences {
		if exp.AgentID == agentID {
			result = append(result, exp)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *mockExperienceStore) GetByTaskType(ctx context.Context, taskType string, limit int) ([]*Experience, error) {
	var result []*Experience
	for _, exp := range s.experiences {
		if exp.TaskType == taskType {
			result = append(result, exp)
		}
	}
	return result, nil
}

func (s *mockExperienceStore) GetSuccessful(ctx context.Context, minScore float64, limit int) ([]*Experience, error) {
	var result []*Experience
	for _, exp := range s.experiences {
		if exp.Success && exp.Score >= minScore {
			result = append(result, exp)
		}
	}
	return result, nil
}

func (s *mockExperienceStore) GetFailed(ctx context.Context, maxScore float64, limit int) ([]*Experience, error) {
	var result []*Experience
	for _, exp := range s.experiences {
		if !exp.Success || exp.Score <= maxScore {
			result = append(result, exp)
		}
	}
	return result, nil
}

func (s *mockExperienceStore) FindSimilar(ctx context.Context, embedding []float32, limit int) ([]*Experience, error) {
	return nil, nil
}

func (s *mockExperienceStore) GetStats(ctx context.Context, agentID string) (*ExperienceStats, error) {
	return &ExperienceStats{}, nil
}

func (s *mockExperienceStore) GetGlobalStats(ctx context.Context) (*ExperienceStats, error) {
	return &ExperienceStats{}, nil
}

func (s *mockExperienceStore) Update(ctx context.Context, exp *Experience) error {
	s.experiences[exp.ID] = exp
	return nil
}

func (s *mockExperienceStore) Delete(ctx context.Context, id string) error {
	delete(s.experiences, id)
	return nil
}

func (s *mockExperienceStore) Prune(ctx context.Context, olderThan time.Time, keepTopN int) (int, error) {
	return 0, nil
}

func (s *mockExperienceStore) Count(ctx context.Context) (int, error) {
	return len(s.experiences), nil
}
