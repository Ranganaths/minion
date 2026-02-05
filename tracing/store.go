package tracing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// TraceStore defines the interface for trace persistence
type TraceStore interface {
	// SaveTrace persists a complete trace
	SaveTrace(ctx context.Context, trace *Trace) error

	// GetTrace retrieves a trace by ID
	GetTrace(ctx context.Context, traceID TraceID) (*Trace, error)

	// GetTraceByAgent retrieves traces for a specific agent
	GetTracesByAgent(ctx context.Context, agentID string, limit, offset int) ([]*Trace, error)

	// GetTracesBySession retrieves traces for a specific session
	GetTracesBySession(ctx context.Context, sessionID string, limit, offset int) ([]*Trace, error)

	// QueryTraces queries traces with filters
	QueryTraces(ctx context.Context, query *TraceQuery) (*TraceQueryResult, error)

	// DeleteTrace deletes a trace by ID
	DeleteTrace(ctx context.Context, traceID TraceID) error

	// DeleteTracesBefore deletes traces older than a given time
	DeleteTracesBefore(ctx context.Context, before time.Time) (int64, error)

	// GetTraceSummaries retrieves trace summaries for listing
	GetTraceSummaries(ctx context.Context, limit, offset int) ([]*TraceSummary, error)

	// GetSpan retrieves a specific span
	GetSpan(ctx context.Context, traceID TraceID, spanID SpanID) (*Span, error)

	// Stats returns storage statistics
	Stats(ctx context.Context) (*TraceStoreStats, error)

	// Close closes the store
	Close() error
}

// TraceStoreStats contains storage statistics
type TraceStoreStats struct {
	TotalTraces      int64            `json:"total_traces"`
	TotalSpans       int64            `json:"total_spans"`
	TotalTokens      int64            `json:"total_tokens"`
	TotalCost        float64          `json:"total_cost"`
	OldestTrace      *time.Time       `json:"oldest_trace,omitempty"`
	NewestTrace      *time.Time       `json:"newest_trace,omitempty"`
	TracesByAgent    map[string]int64 `json:"traces_by_agent"`
	TracesByStatus   map[string]int64 `json:"traces_by_status"`
	AvgDuration      float64          `json:"avg_duration_ms"`
	AvgTokensPerTrace float64         `json:"avg_tokens_per_trace"`
}

// InMemoryTraceStore is an in-memory implementation of TraceStore
type InMemoryTraceStore struct {
	mu     sync.RWMutex
	traces map[TraceID]*Trace

	// Indexes for faster queries
	byAgent   map[string][]TraceID
	bySession map[string][]TraceID
}

// NewInMemoryTraceStore creates a new in-memory trace store
func NewInMemoryTraceStore() *InMemoryTraceStore {
	return &InMemoryTraceStore{
		traces:    make(map[TraceID]*Trace),
		byAgent:   make(map[string][]TraceID),
		bySession: make(map[string][]TraceID),
	}
}

// SaveTrace persists a trace to memory
func (s *InMemoryTraceStore) SaveTrace(ctx context.Context, trace *Trace) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Make a deep copy to avoid external modifications
	data, err := json.Marshal(trace)
	if err != nil {
		return fmt.Errorf("failed to marshal trace: %w", err)
	}

	var traceCopy Trace
	if err := json.Unmarshal(data, &traceCopy); err != nil {
		return fmt.Errorf("failed to unmarshal trace: %w", err)
	}

	s.traces[trace.ID] = &traceCopy

	// Update indexes
	s.byAgent[trace.AgentID] = append(s.byAgent[trace.AgentID], trace.ID)
	if trace.SessionID != "" {
		s.bySession[trace.SessionID] = append(s.bySession[trace.SessionID], trace.ID)
	}

	return nil
}

// GetTrace retrieves a trace by ID
func (s *InMemoryTraceStore) GetTrace(ctx context.Context, traceID TraceID) (*Trace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trace, ok := s.traces[traceID]
	if !ok {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	return trace, nil
}

// GetTracesByAgent retrieves traces for a specific agent
func (s *InMemoryTraceStore) GetTracesByAgent(ctx context.Context, agentID string, limit, offset int) ([]*Trace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.byAgent[agentID]
	return s.getTracesFromIDs(ids, limit, offset)
}

// GetTracesBySession retrieves traces for a specific session
func (s *InMemoryTraceStore) GetTracesBySession(ctx context.Context, sessionID string, limit, offset int) ([]*Trace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.bySession[sessionID]
	return s.getTracesFromIDs(ids, limit, offset)
}

// QueryTraces queries traces with filters
func (s *InMemoryTraceStore) QueryTraces(ctx context.Context, query *TraceQuery) (*TraceQueryResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matching []*Trace

	for _, trace := range s.traces {
		if s.matchesFilter(trace, &query.Filter) {
			matching = append(matching, trace)
		}
	}

	// Sort by start time descending (newest first)
	for i := 0; i < len(matching)-1; i++ {
		for j := i + 1; j < len(matching); j++ {
			if matching[i].StartTime.Before(matching[j].StartTime) {
				matching[i], matching[j] = matching[j], matching[i]
			}
		}
	}

	totalCount := int64(len(matching))

	// Apply pagination
	start := query.Offset
	if start >= len(matching) {
		return &TraceQueryResult{
			Traces:     []*Trace{},
			TotalCount: totalCount,
			HasMore:    false,
		}, nil
	}

	end := start + query.Limit
	if end > len(matching) {
		end = len(matching)
	}

	return &TraceQueryResult{
		Traces:     matching[start:end],
		TotalCount: totalCount,
		HasMore:    end < len(matching),
	}, nil
}

// DeleteTrace deletes a trace by ID
func (s *InMemoryTraceStore) DeleteTrace(ctx context.Context, traceID TraceID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	trace, ok := s.traces[traceID]
	if !ok {
		return fmt.Errorf("trace not found: %s", traceID)
	}

	// Remove from indexes
	s.removeFromIndex(s.byAgent, trace.AgentID, traceID)
	if trace.SessionID != "" {
		s.removeFromIndex(s.bySession, trace.SessionID, traceID)
	}

	delete(s.traces, traceID)
	return nil
}

// DeleteTracesBefore deletes traces older than a given time
func (s *InMemoryTraceStore) DeleteTracesBefore(ctx context.Context, before time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var deleted int64
	var toDelete []TraceID

	for id, trace := range s.traces {
		if trace.StartTime.Before(before) {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		trace := s.traces[id]
		s.removeFromIndex(s.byAgent, trace.AgentID, id)
		if trace.SessionID != "" {
			s.removeFromIndex(s.bySession, trace.SessionID, id)
		}
		delete(s.traces, id)
		deleted++
	}

	return deleted, nil
}

// GetTraceSummaries retrieves trace summaries for listing
func (s *InMemoryTraceStore) GetTraceSummaries(ctx context.Context, limit, offset int) ([]*TraceSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect all traces
	var traces []*Trace
	for _, trace := range s.traces {
		traces = append(traces, trace)
	}

	// Sort by start time descending
	for i := 0; i < len(traces)-1; i++ {
		for j := i + 1; j < len(traces); j++ {
			if traces[i].StartTime.Before(traces[j].StartTime) {
				traces[i], traces[j] = traces[j], traces[i]
			}
		}
	}

	// Apply pagination
	start := offset
	if start >= len(traces) {
		return []*TraceSummary{}, nil
	}

	end := start + limit
	if end > len(traces) {
		end = len(traces)
	}

	// Convert to summaries
	summaries := make([]*TraceSummary, 0, end-start)
	for _, trace := range traces[start:end] {
		summaries = append(summaries, trace.ToSummary())
	}

	return summaries, nil
}

// GetSpan retrieves a specific span
func (s *InMemoryTraceStore) GetSpan(ctx context.Context, traceID TraceID, spanID SpanID) (*Span, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trace, ok := s.traces[traceID]
	if !ok {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	for _, span := range trace.Spans {
		if span.ID == spanID {
			return span, nil
		}
	}

	return nil, fmt.Errorf("span not found: %s", spanID)
}

// Stats returns storage statistics
func (s *InMemoryTraceStore) Stats(ctx context.Context) (*TraceStoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &TraceStoreStats{
		TracesByAgent:  make(map[string]int64),
		TracesByStatus: make(map[string]int64),
	}

	var totalDuration int64
	var totalTokens int64

	for _, trace := range s.traces {
		stats.TotalTraces++
		stats.TotalSpans += int64(len(trace.Spans))
		stats.TotalTokens += int64(trace.TotalTokens.TotalTokens)
		stats.TotalCost += trace.TotalCost
		totalDuration += trace.Duration
		totalTokens += int64(trace.TotalTokens.TotalTokens)

		stats.TracesByAgent[trace.AgentID]++
		stats.TracesByStatus[string(trace.Status)]++

		if stats.OldestTrace == nil || trace.StartTime.Before(*stats.OldestTrace) {
			t := trace.StartTime
			stats.OldestTrace = &t
		}
		if stats.NewestTrace == nil || trace.StartTime.After(*stats.NewestTrace) {
			t := trace.StartTime
			stats.NewestTrace = &t
		}
	}

	if stats.TotalTraces > 0 {
		stats.AvgDuration = float64(totalDuration) / float64(stats.TotalTraces)
		stats.AvgTokensPerTrace = float64(totalTokens) / float64(stats.TotalTraces)
	}

	return stats, nil
}

// Close closes the store (no-op for in-memory)
func (s *InMemoryTraceStore) Close() error {
	return nil
}

// Helper methods

func (s *InMemoryTraceStore) getTracesFromIDs(ids []TraceID, limit, offset int) ([]*Trace, error) {
	if offset >= len(ids) {
		return []*Trace{}, nil
	}

	end := offset + limit
	if end > len(ids) {
		end = len(ids)
	}

	// Reverse order (newest first)
	traces := make([]*Trace, 0, end-offset)
	for i := len(ids) - 1 - offset; i >= 0 && len(traces) < limit; i-- {
		if trace, ok := s.traces[ids[i]]; ok {
			traces = append(traces, trace)
		}
	}

	return traces, nil
}

func (s *InMemoryTraceStore) matchesFilter(trace *Trace, filter *TraceFilter) bool {
	if filter.AgentID != "" && trace.AgentID != filter.AgentID {
		return false
	}
	if filter.SessionID != "" && trace.SessionID != filter.SessionID {
		return false
	}
	if filter.Status != "" && trace.Status != filter.Status {
		return false
	}
	if filter.MinDuration > 0 && trace.Duration < filter.MinDuration {
		return false
	}
	if filter.MaxDuration > 0 && trace.Duration > filter.MaxDuration {
		return false
	}
	if filter.StartTimeFrom != nil && trace.StartTime.Before(*filter.StartTimeFrom) {
		return false
	}
	if filter.StartTimeTo != nil && trace.StartTime.After(*filter.StartTimeTo) {
		return false
	}
	if filter.HasError != nil {
		hasError := trace.Status == SpanStatusError
		if *filter.HasError != hasError {
			return false
		}
	}
	if filter.MinTokens > 0 && trace.TotalTokens.TotalTokens < filter.MinTokens {
		return false
	}
	if filter.ToolName != "" {
		found := false
		for _, span := range trace.Spans {
			if span.Type == SpanTypeToolCall && span.ToolDetails != nil && span.ToolDetails.ToolName == filter.ToolName {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (s *InMemoryTraceStore) removeFromIndex(index map[string][]TraceID, key string, id TraceID) {
	slice := index[key]
	for i, v := range slice {
		if v == id {
			index[key] = append(slice[:i], slice[i+1:]...)
			return
		}
	}
}

// Ensure InMemoryTraceStore implements TraceStore
var _ TraceStore = (*InMemoryTraceStore)(nil)
