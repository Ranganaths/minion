// Package store provides implementations of the ExperienceStore interface.
package store

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/Ranganaths/minion/core/selfimprove"
)

// InMemoryExperienceStore is an in-memory implementation of ExperienceStore.
// Suitable for development, testing, and single-instance deployments.
type InMemoryExperienceStore struct {
	mu          sync.RWMutex
	experiences map[string]*selfimprove.Experience
	byAgent     map[string][]string // agentID -> experienceIDs
	byTaskType  map[string][]string // taskType -> experienceIDs
	byTraceID   map[string]string   // traceID -> experienceID
}

// NewInMemoryExperienceStore creates a new in-memory experience store.
func NewInMemoryExperienceStore() *InMemoryExperienceStore {
	return &InMemoryExperienceStore{
		experiences: make(map[string]*selfimprove.Experience),
		byAgent:     make(map[string][]string),
		byTaskType:  make(map[string][]string),
		byTraceID:   make(map[string]string),
	}
}

// Store saves an experience to the store.
func (s *InMemoryExperienceStore) Store(ctx context.Context, exp *selfimprove.Experience) error {
	if exp == nil {
		return errors.New("experience cannot be nil")
	}
	if exp.ID == "" {
		return errors.New("experience ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Store the experience
	s.experiences[exp.ID] = exp

	// Index by agent
	if exp.AgentID != "" {
		s.byAgent[exp.AgentID] = append(s.byAgent[exp.AgentID], exp.ID)
	}

	// Index by task type
	if exp.TaskType != "" {
		s.byTaskType[exp.TaskType] = append(s.byTaskType[exp.TaskType], exp.ID)
	}

	// Index by trace ID
	if exp.TraceID != "" {
		s.byTraceID[exp.TraceID] = exp.ID
	}

	return nil
}

// Get retrieves an experience by ID.
func (s *InMemoryExperienceStore) Get(ctx context.Context, id string) (*selfimprove.Experience, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exp, ok := s.experiences[id]
	if !ok {
		return nil, errors.New("experience not found")
	}
	return exp, nil
}

// GetByTraceID retrieves an experience by its trace ID.
func (s *InMemoryExperienceStore) GetByTraceID(ctx context.Context, traceID string) (*selfimprove.Experience, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	expID, ok := s.byTraceID[traceID]
	if !ok {
		return nil, errors.New("experience not found for trace ID")
	}

	exp, ok := s.experiences[expID]
	if !ok {
		return nil, errors.New("experience not found")
	}
	return exp, nil
}

// Query retrieves experiences matching the query criteria.
func (s *InMemoryExperienceStore) Query(ctx context.Context, query *selfimprove.ExperienceQuery) ([]*selfimprove.Experience, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*selfimprove.Experience

	for _, exp := range s.experiences {
		if matchesQuery(exp, query) {
			results = append(results, exp)
		}
	}

	// Sort results
	sortExperiences(results, query.OrderBy, query.OrderDesc)

	// Apply offset and limit
	if query.Offset > 0 {
		if query.Offset >= len(results) {
			return []*selfimprove.Experience{}, nil
		}
		results = results[query.Offset:]
	}

	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	return results, nil
}

// GetByAgent retrieves experiences for a specific agent.
func (s *InMemoryExperienceStore) GetByAgent(ctx context.Context, agentID string, limit int) ([]*selfimprove.Experience, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byAgent[agentID]
	if !ok {
		return []*selfimprove.Experience{}, nil
	}

	results := make([]*selfimprove.Experience, 0, len(ids))
	for _, id := range ids {
		if exp, ok := s.experiences[id]; ok {
			results = append(results, exp)
		}
	}

	// Sort by timestamp descending (most recent first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// GetByTaskType retrieves experiences for a specific task type.
func (s *InMemoryExperienceStore) GetByTaskType(ctx context.Context, taskType string, limit int) ([]*selfimprove.Experience, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTaskType[taskType]
	if !ok {
		return []*selfimprove.Experience{}, nil
	}

	results := make([]*selfimprove.Experience, 0, len(ids))
	for _, id := range ids {
		if exp, ok := s.experiences[id]; ok {
			results = append(results, exp)
		}
	}

	// Sort by timestamp descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// GetSuccessful retrieves successful experiences above a score threshold.
func (s *InMemoryExperienceStore) GetSuccessful(ctx context.Context, minScore float64, limit int) ([]*selfimprove.Experience, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*selfimprove.Experience
	for _, exp := range s.experiences {
		if exp.Success && exp.Score >= minScore {
			results = append(results, exp)
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// GetFailed retrieves failed experiences below a score threshold.
func (s *InMemoryExperienceStore) GetFailed(ctx context.Context, maxScore float64, limit int) ([]*selfimprove.Experience, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*selfimprove.Experience
	for _, exp := range s.experiences {
		if !exp.Success || exp.Score <= maxScore {
			results = append(results, exp)
		}
	}

	// Sort by timestamp descending (most recent first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// FindSimilar finds experiences with similar embeddings using cosine similarity.
func (s *InMemoryExperienceStore) FindSimilar(ctx context.Context, embedding []float32, limit int) ([]*selfimprove.Experience, error) {
	if len(embedding) == 0 {
		return []*selfimprove.Experience{}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	type scored struct {
		exp   *selfimprove.Experience
		score float64
	}

	var scored_results []scored
	for _, exp := range s.experiences {
		if len(exp.Embedding) == len(embedding) {
			similarity := cosineSimilarity(embedding, exp.Embedding)
			scored_results = append(scored_results, scored{exp: exp, score: similarity})
		}
	}

	// Sort by similarity descending
	sort.Slice(scored_results, func(i, j int) bool {
		return scored_results[i].score > scored_results[j].score
	})

	results := make([]*selfimprove.Experience, 0, limit)
	for i := 0; i < len(scored_results) && (limit <= 0 || i < limit); i++ {
		results = append(results, scored_results[i].exp)
	}

	return results, nil
}

// GetStats returns aggregated statistics for an agent.
func (s *InMemoryExperienceStore) GetStats(ctx context.Context, agentID string) (*selfimprove.ExperienceStats, error) {
	experiences, err := s.GetByAgent(ctx, agentID, 0) // 0 = no limit
	if err != nil {
		return nil, err
	}

	return calculateStats(experiences), nil
}

// GetGlobalStats returns aggregated statistics across all agents.
func (s *InMemoryExperienceStore) GetGlobalStats(ctx context.Context) (*selfimprove.ExperienceStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	experiences := make([]*selfimprove.Experience, 0, len(s.experiences))
	for _, exp := range s.experiences {
		experiences = append(experiences, exp)
	}

	return calculateStats(experiences), nil
}

// Update updates an existing experience.
func (s *InMemoryExperienceStore) Update(ctx context.Context, exp *selfimprove.Experience) error {
	if exp == nil {
		return errors.New("experience cannot be nil")
	}
	if exp.ID == "" {
		return errors.New("experience ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.experiences[exp.ID]; !ok {
		return errors.New("experience not found")
	}

	s.experiences[exp.ID] = exp
	return nil
}

// Delete removes an experience from the store.
func (s *InMemoryExperienceStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.experiences[id]
	if !ok {
		return errors.New("experience not found")
	}

	// Remove from indexes
	if exp.AgentID != "" {
		s.byAgent[exp.AgentID] = removeFromSlice(s.byAgent[exp.AgentID], id)
	}
	if exp.TaskType != "" {
		s.byTaskType[exp.TaskType] = removeFromSlice(s.byTaskType[exp.TaskType], id)
	}
	if exp.TraceID != "" {
		delete(s.byTraceID, exp.TraceID)
	}

	delete(s.experiences, id)
	return nil
}

// Prune removes old experiences while keeping top performers.
func (s *InMemoryExperienceStore) Prune(ctx context.Context, olderThan time.Time, keepTopN int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// First, identify top performers to keep
	var allExps []*selfimprove.Experience
	for _, exp := range s.experiences {
		allExps = append(allExps, exp)
	}

	// Sort by score descending
	sort.Slice(allExps, func(i, j int) bool {
		return allExps[i].Score > allExps[j].Score
	})

	// Mark top N for keeping
	keepIDs := make(map[string]bool)
	for i := 0; i < len(allExps) && i < keepTopN; i++ {
		keepIDs[allExps[i].ID] = true
	}

	// Remove old experiences that aren't in top N
	pruned := 0
	for id, exp := range s.experiences {
		if exp.Timestamp.Before(olderThan) && !keepIDs[id] {
			// Remove from indexes
			if exp.AgentID != "" {
				s.byAgent[exp.AgentID] = removeFromSlice(s.byAgent[exp.AgentID], id)
			}
			if exp.TaskType != "" {
				s.byTaskType[exp.TaskType] = removeFromSlice(s.byTaskType[exp.TaskType], id)
			}
			if exp.TraceID != "" {
				delete(s.byTraceID, exp.TraceID)
			}

			delete(s.experiences, id)
			pruned++
		}
	}

	return pruned, nil
}

// Count returns the total number of experiences.
func (s *InMemoryExperienceStore) Count(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.experiences), nil
}

// Helper functions

func matchesQuery(exp *selfimprove.Experience, query *selfimprove.ExperienceQuery) bool {
	if query == nil {
		return true
	}

	if query.AgentID != "" && exp.AgentID != query.AgentID {
		return false
	}

	if query.TaskType != "" && exp.TaskType != query.TaskType {
		return false
	}

	if query.MinScore != nil && exp.Score < *query.MinScore {
		return false
	}

	if query.MaxScore != nil && exp.Score > *query.MaxScore {
		return false
	}

	if query.Success != nil && exp.Success != *query.Success {
		return false
	}

	if query.HasHumanFeedback != nil {
		hasFeedback := exp.HasHumanFeedback()
		if *query.HasHumanFeedback != hasFeedback {
			return false
		}
	}

	if query.Since != nil && exp.Timestamp.Before(*query.Since) {
		return false
	}

	if query.Until != nil && exp.Timestamp.After(*query.Until) {
		return false
	}

	if query.PromptVersion != "" && exp.PromptVersion != query.PromptVersion {
		return false
	}

	return true
}

func sortExperiences(exps []*selfimprove.Experience, orderBy string, desc bool) {
	sort.Slice(exps, func(i, j int) bool {
		var less bool
		switch orderBy {
		case "score":
			less = exps[i].Score < exps[j].Score
		case "timestamp":
			less = exps[i].Timestamp.Before(exps[j].Timestamp)
		case "latency":
			less = exps[i].LatencyMs < exps[j].LatencyMs
		case "tokens":
			less = exps[i].TokensUsed < exps[j].TokensUsed
		default:
			less = exps[i].Timestamp.Before(exps[j].Timestamp)
		}
		if desc {
			return !less
		}
		return less
	})
}

func calculateStats(experiences []*selfimprove.Experience) *selfimprove.ExperienceStats {
	stats := &selfimprove.ExperienceStats{
		ScoresByTaskType: make(map[string]float64),
		CountByTaskType:  make(map[string]int),
		RecentTrend:      selfimprove.TrendUnknown,
	}

	if len(experiences) == 0 {
		return stats
	}

	var totalScore, totalLatency, totalTokens float64
	taskTypeScores := make(map[string][]float64)

	for _, exp := range experiences {
		stats.TotalCount++
		if exp.Success {
			stats.SuccessCount++
		} else {
			stats.FailureCount++
		}

		totalScore += exp.Score
		totalLatency += float64(exp.LatencyMs)
		totalTokens += float64(exp.TokensUsed)

		if exp.TaskType != "" {
			taskTypeScores[exp.TaskType] = append(taskTypeScores[exp.TaskType], exp.Score)
			stats.CountByTaskType[exp.TaskType]++
		}

		if stats.FirstExperience == nil || exp.Timestamp.Before(*stats.FirstExperience) {
			t := exp.Timestamp
			stats.FirstExperience = &t
		}
		if stats.LastExperience == nil || exp.Timestamp.After(*stats.LastExperience) {
			t := exp.Timestamp
			stats.LastExperience = &t
		}
	}

	stats.AvgScore = totalScore / float64(stats.TotalCount)
	stats.AvgLatencyMs = totalLatency / float64(stats.TotalCount)
	stats.AvgTokensUsed = totalTokens / float64(stats.TotalCount)
	stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalCount)

	// Calculate average scores by task type
	for taskType, scores := range taskTypeScores {
		var sum float64
		for _, score := range scores {
			sum += score
		}
		stats.ScoresByTaskType[taskType] = sum / float64(len(scores))
	}

	// Calculate trend (compare recent 20% vs older 80%)
	if len(experiences) >= 10 {
		sorted := make([]*selfimprove.Experience, len(experiences))
		copy(sorted, experiences)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Timestamp.Before(sorted[j].Timestamp)
		})

		recentStart := len(sorted) * 4 / 5
		var oldAvg, recentAvg float64

		for i := 0; i < recentStart; i++ {
			oldAvg += sorted[i].Score
		}
		oldAvg /= float64(recentStart)

		for i := recentStart; i < len(sorted); i++ {
			recentAvg += sorted[i].Score
		}
		recentAvg /= float64(len(sorted) - recentStart)

		diff := recentAvg - oldAvg
		if diff > 0.05 {
			stats.RecentTrend = selfimprove.TrendImproving
		} else if diff < -0.05 {
			stats.RecentTrend = selfimprove.TrendDeclining
		} else {
			stats.RecentTrend = selfimprove.TrendStable
		}
	}

	return stats
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}
