package selfimprove

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PromptVersionManager manages versions of prompts across agents.
type PromptVersionManager struct {
	mu sync.RWMutex

	// Versions by agent ID
	versions map[string][]*PromptVersionRecord

	// Active versions by agent ID
	activeVersions map[string]string // agentID -> versionID

	// Rollback history
	rollbackHistory map[string][]*RollbackRecord

	// Configuration
	maxVersions int
}

// PromptVersionRecord represents a version of a prompt.
type PromptVersionRecord struct {
	ID           string    `json:"id"`
	AgentID      string    `json:"agent_id"`
	Version      int       `json:"version"`
	Prompt       string    `json:"prompt"`
	ProposalID   string    `json:"proposal_id,omitempty"` // If created from a proposal
	Strategy     string    `json:"strategy,omitempty"`    // Strategy that created this
	CreatedAt    time.Time `json:"created_at"`
	ActivatedAt  *time.Time `json:"activated_at,omitempty"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
	IsActive     bool      `json:"is_active"`

	// Performance metrics
	Executions   int     `json:"executions"`
	TotalScore   float64 `json:"total_score"`
	SuccessCount int     `json:"success_count"`
	FailureCount int     `json:"failure_count"`
}

// RollbackRecord represents a rollback event.
type RollbackRecord struct {
	ID             string    `json:"id"`
	AgentID        string    `json:"agent_id"`
	FromVersionID  string    `json:"from_version_id"`
	ToVersionID    string    `json:"to_version_id"`
	Reason         string    `json:"reason"`
	RolledBackAt   time.Time `json:"rolled_back_at"`
	ScoreBefore    float64   `json:"score_before"`
	ScoreAfter     float64   `json:"score_after,omitempty"`
}

// NewPromptVersionManager creates a new prompt version manager.
func NewPromptVersionManager(maxVersions int) *PromptVersionManager {
	if maxVersions <= 0 {
		maxVersions = 10
	}

	return &PromptVersionManager{
		versions:        make(map[string][]*PromptVersionRecord),
		activeVersions:  make(map[string]string),
		rollbackHistory: make(map[string][]*RollbackRecord),
		maxVersions:     maxVersions,
	}
}

// CreateVersion creates a new prompt version.
func (m *PromptVersionManager) CreateVersion(
	agentID string,
	prompt string,
	proposalID string,
	strategy string,
) *PromptVersionRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions := m.versions[agentID]
	versionNum := len(versions) + 1

	record := &PromptVersionRecord{
		ID:        fmt.Sprintf("%s-v%d", agentID, versionNum),
		AgentID:   agentID,
		Version:   versionNum,
		Prompt:    prompt,
		ProposalID: proposalID,
		Strategy:  strategy,
		CreatedAt: time.Now(),
		IsActive:  false,
	}

	m.versions[agentID] = append(m.versions[agentID], record)

	// Prune old versions if needed
	m.pruneVersionsLocked(agentID)

	return record
}

// ActivateVersion activates a specific version.
func (m *PromptVersionManager) ActivateVersion(agentID, versionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions := m.versions[agentID]
	var targetVersion *PromptVersionRecord

	// Deactivate current version
	for _, v := range versions {
		if v.IsActive {
			v.IsActive = false
			now := time.Now()
			v.DeactivatedAt = &now
		}
		if v.ID == versionID {
			targetVersion = v
		}
	}

	if targetVersion == nil {
		return fmt.Errorf("version not found: %s", versionID)
	}

	// Activate target version
	targetVersion.IsActive = true
	now := time.Now()
	targetVersion.ActivatedAt = &now
	targetVersion.DeactivatedAt = nil

	m.activeVersions[agentID] = versionID

	return nil
}

// GetActiveVersion returns the active prompt version for an agent.
func (m *PromptVersionManager) GetActiveVersion(agentID string) *PromptVersionRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versionID, ok := m.activeVersions[agentID]
	if !ok {
		return nil
	}

	for _, v := range m.versions[agentID] {
		if v.ID == versionID {
			return v
		}
	}

	return nil
}

// GetActivePrompt returns the active prompt for an agent.
func (m *PromptVersionManager) GetActivePrompt(agentID string) string {
	version := m.GetActiveVersion(agentID)
	if version == nil {
		return ""
	}
	return version.Prompt
}

// RecordExecution records an execution against the active version.
func (m *PromptVersionManager) RecordExecution(agentID string, score float64, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	versionID, ok := m.activeVersions[agentID]
	if !ok {
		return
	}

	for _, v := range m.versions[agentID] {
		if v.ID == versionID {
			v.Executions++
			v.TotalScore += score
			if success {
				v.SuccessCount++
			} else {
				v.FailureCount++
			}
			break
		}
	}
}

// RollbackToVersion rolls back to a previous version.
func (m *PromptVersionManager) RollbackToVersion(agentID, targetVersionID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentVersionID := m.activeVersions[agentID]
	if currentVersionID == "" {
		return fmt.Errorf("no active version to rollback from")
	}

	var currentVersion, targetVersion *PromptVersionRecord
	for _, v := range m.versions[agentID] {
		if v.ID == currentVersionID {
			currentVersion = v
		}
		if v.ID == targetVersionID {
			targetVersion = v
		}
	}

	if targetVersion == nil {
		return fmt.Errorf("target version not found: %s", targetVersionID)
	}

	// Deactivate current version
	if currentVersion != nil {
		currentVersion.IsActive = false
		now := time.Now()
		currentVersion.DeactivatedAt = &now
	}

	// Activate target version
	targetVersion.IsActive = true
	now := time.Now()
	targetVersion.ActivatedAt = &now
	targetVersion.DeactivatedAt = nil

	m.activeVersions[agentID] = targetVersionID

	// Record rollback
	scoreBefore := 0.0
	if currentVersion != nil && currentVersion.Executions > 0 {
		scoreBefore = currentVersion.TotalScore / float64(currentVersion.Executions)
	}

	rollback := &RollbackRecord{
		ID:            fmt.Sprintf("rollback-%s-%d", agentID, time.Now().UnixNano()),
		AgentID:       agentID,
		FromVersionID: currentVersionID,
		ToVersionID:   targetVersionID,
		Reason:        reason,
		RolledBackAt:  time.Now(),
		ScoreBefore:   scoreBefore,
	}

	m.rollbackHistory[agentID] = append(m.rollbackHistory[agentID], rollback)

	return nil
}

// RollbackToPrevious rolls back to the previous version.
func (m *PromptVersionManager) RollbackToPrevious(agentID, reason string) error {
	m.mu.RLock()
	versions := m.versions[agentID]
	currentVersionID := m.activeVersions[agentID]
	m.mu.RUnlock()

	if len(versions) < 2 {
		return fmt.Errorf("no previous version to rollback to")
	}

	// Find previous version
	var prevVersionID string
	for i := len(versions) - 1; i >= 0; i-- {
		if versions[i].ID != currentVersionID {
			prevVersionID = versions[i].ID
			break
		}
	}

	if prevVersionID == "" {
		return fmt.Errorf("could not find previous version")
	}

	return m.RollbackToVersion(agentID, prevVersionID, reason)
}

// GetVersionHistory returns all versions for an agent.
func (m *PromptVersionManager) GetVersionHistory(agentID string) []*PromptVersionRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := m.versions[agentID]
	result := make([]*PromptVersionRecord, len(versions))
	copy(result, versions)
	return result
}

// GetRollbackHistory returns rollback history for an agent.
func (m *PromptVersionManager) GetRollbackHistory(agentID string) []*RollbackRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := m.rollbackHistory[agentID]
	result := make([]*RollbackRecord, len(history))
	copy(result, history)
	return result
}

// GetVersionMetrics returns performance metrics for a version.
func (m *PromptVersionManager) GetVersionMetrics(agentID, versionID string) *VersionMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, v := range m.versions[agentID] {
		if v.ID == versionID {
			avgScore := 0.0
			successRate := 0.0
			if v.Executions > 0 {
				avgScore = v.TotalScore / float64(v.Executions)
				successRate = float64(v.SuccessCount) / float64(v.Executions)
			}

			return &VersionMetrics{
				VersionID:   versionID,
				Executions:  v.Executions,
				AvgScore:    avgScore,
				SuccessRate: successRate,
				SuccessCount: v.SuccessCount,
				FailureCount: v.FailureCount,
			}
		}
	}

	return nil
}

// VersionMetrics contains performance metrics for a version.
type VersionMetrics struct {
	VersionID    string  `json:"version_id"`
	Executions   int     `json:"executions"`
	AvgScore     float64 `json:"avg_score"`
	SuccessRate  float64 `json:"success_rate"`
	SuccessCount int     `json:"success_count"`
	FailureCount int     `json:"failure_count"`
}

// CompareVersions compares two versions.
func (m *PromptVersionManager) CompareVersions(agentID, versionA, versionB string) *VersionComparison {
	metricsA := m.GetVersionMetrics(agentID, versionA)
	metricsB := m.GetVersionMetrics(agentID, versionB)

	if metricsA == nil || metricsB == nil {
		return nil
	}

	return &VersionComparison{
		VersionA:         versionA,
		VersionB:         versionB,
		ScoreDiff:        metricsB.AvgScore - metricsA.AvgScore,
		SuccessRateDiff:  metricsB.SuccessRate - metricsA.SuccessRate,
		ExecutionsDiff:   metricsB.Executions - metricsA.Executions,
		BetterVersion:    determineBetterVersion(metricsA, metricsB),
	}
}

// VersionComparison represents a comparison between two versions.
type VersionComparison struct {
	VersionA        string  `json:"version_a"`
	VersionB        string  `json:"version_b"`
	ScoreDiff       float64 `json:"score_diff"`
	SuccessRateDiff float64 `json:"success_rate_diff"`
	ExecutionsDiff  int     `json:"executions_diff"`
	BetterVersion   string  `json:"better_version"`
}

// ShouldRollback determines if rollback is recommended based on performance.
func (m *PromptVersionManager) ShouldRollback(agentID string, threshold float64) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := m.versions[agentID]
	if len(versions) < 2 {
		return false, ""
	}

	currentVersionID := m.activeVersions[agentID]
	var currentVersion, prevVersion *PromptVersionRecord

	for i, v := range versions {
		if v.ID == currentVersionID {
			currentVersion = v
			if i > 0 {
				prevVersion = versions[i-1]
			}
			break
		}
	}

	if currentVersion == nil || prevVersion == nil {
		return false, ""
	}

	// Need minimum executions to compare
	if currentVersion.Executions < 10 || prevVersion.Executions < 10 {
		return false, ""
	}

	currentAvg := currentVersion.TotalScore / float64(currentVersion.Executions)
	prevAvg := prevVersion.TotalScore / float64(prevVersion.Executions)

	// Rollback if current is worse by more than threshold
	if prevAvg - currentAvg > threshold {
		reason := fmt.Sprintf("Performance regression: current avg %.2f vs previous %.2f (diff: %.2f)",
			currentAvg, prevAvg, prevAvg-currentAvg)
		return true, reason
	}

	return false, ""
}

// AutoRollbackCheck checks if automatic rollback should be triggered.
func (m *PromptVersionManager) AutoRollbackCheck(ctx context.Context, agentID string, threshold float64) error {
	shouldRollback, reason := m.ShouldRollback(agentID, threshold)
	if shouldRollback {
		return m.RollbackToPrevious(agentID, "Auto-rollback: "+reason)
	}
	return nil
}

// pruneVersionsLocked removes old versions beyond maxVersions (must hold lock).
func (m *PromptVersionManager) pruneVersionsLocked(agentID string) {
	versions := m.versions[agentID]
	if len(versions) <= m.maxVersions {
		return
	}

	// Keep most recent versions, but never remove active version
	activeVersionID := m.activeVersions[agentID]
	newVersions := make([]*PromptVersionRecord, 0, m.maxVersions)

	// Add active version first if exists
	for _, v := range versions {
		if v.ID == activeVersionID {
			newVersions = append(newVersions, v)
			break
		}
	}

	// Add most recent versions
	for i := len(versions) - 1; i >= 0 && len(newVersions) < m.maxVersions; i-- {
		if versions[i].ID != activeVersionID {
			newVersions = append([]*PromptVersionRecord{versions[i]}, newVersions...)
		}
	}

	m.versions[agentID] = newVersions
}

func determineBetterVersion(a, b *VersionMetrics) string {
	if a == nil || b == nil {
		return "unknown"
	}

	// Weight score more heavily if both have sufficient executions
	if a.Executions >= 10 && b.Executions >= 10 {
		if b.AvgScore > a.AvgScore+0.05 {
			return b.VersionID
		}
		if a.AvgScore > b.AvgScore+0.05 {
			return a.VersionID
		}
	}

	// Consider success rate if scores are close
	if b.SuccessRate > a.SuccessRate+0.1 {
		return b.VersionID
	}
	if a.SuccessRate > b.SuccessRate+0.1 {
		return a.VersionID
	}

	return "inconclusive"
}
