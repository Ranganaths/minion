package humanloop

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FeedbackType represents the type of feedback
type FeedbackType string

const (
	FeedbackTypeRating     FeedbackType = "rating"     // Numeric rating (1-5 stars, etc.)
	FeedbackTypeBinary     FeedbackType = "binary"     // Thumbs up/down
	FeedbackTypeText       FeedbackType = "text"       // Free text feedback
	FeedbackTypeCorrection FeedbackType = "correction" // Corrected output
	FeedbackTypePreference FeedbackType = "preference" // Preference between options
	FeedbackTypeLabel      FeedbackType = "label"      // Category label
)

// Feedback represents user feedback on agent output
type Feedback struct {
	ID          string                 `json:"id"`
	Type        FeedbackType           `json:"type"`
	AgentID     string                 `json:"agent_id"`
	TaskID      string                 `json:"task_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	InputData   interface{}            `json:"input_data,omitempty"`
	OutputData  interface{}            `json:"output_data,omitempty"`
	Value       interface{}            `json:"value"`                // Rating value, text, etc.
	Label       string                 `json:"label,omitempty"`      // Category label
	Correction  interface{}            `json:"correction,omitempty"` // Corrected output
	Preference  *PreferenceData        `json:"preference,omitempty"` // Preference data
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// PreferenceData represents preference feedback between options
type PreferenceData struct {
	Options    []interface{} `json:"options"`
	Chosen     int           `json:"chosen"`      // Index of chosen option
	Reasoning  string        `json:"reasoning,omitempty"`
}

// FeedbackStore stores feedback data
type FeedbackStore interface {
	Store(ctx context.Context, feedback *Feedback) error
	Get(ctx context.Context, id string) (*Feedback, error)
	List(ctx context.Context, filter *FeedbackFilter) ([]*Feedback, error)
	GetStats(ctx context.Context, filter *FeedbackFilter) (*FeedbackStats, error)
	Delete(ctx context.Context, id string) error
}

// FeedbackFilter filters feedback queries
type FeedbackFilter struct {
	AgentID     string
	TaskID      string
	SessionID   string
	Type        FeedbackType
	UserID      string
	StartTime   *time.Time
	EndTime     *time.Time
	MinRating   *float64
	MaxRating   *float64
	Tags        []string
	Labels      []string
	Limit       int
	Offset      int
}

// FeedbackStats represents aggregated feedback statistics
type FeedbackStats struct {
	TotalCount     int64              `json:"total_count"`
	AverageRating  float64            `json:"average_rating,omitempty"`
	RatingDist     map[int]int64      `json:"rating_distribution,omitempty"`
	PositiveCount  int64              `json:"positive_count,omitempty"`
	NegativeCount  int64              `json:"negative_count,omitempty"`
	LabelCounts    map[string]int64   `json:"label_counts,omitempty"`
	TagCounts      map[string]int64   `json:"tag_counts,omitempty"`
}

// FeedbackCollector collects and processes feedback
type FeedbackCollector struct {
	store       FeedbackStore
	notifier    *NotificationManager
	handlers    []FeedbackHandler
	minRating   float64 // Minimum acceptable rating (triggers escalation below)
	mu          sync.RWMutex
}

// FeedbackHandler processes feedback
type FeedbackHandler interface {
	HandleFeedback(ctx context.Context, feedback *Feedback) error
}

// FeedbackCollectorConfig configures the feedback collector
type FeedbackCollectorConfig struct {
	Store           FeedbackStore
	Notifier        *NotificationManager
	MinRating       float64 // Minimum acceptable rating
}

// NewFeedbackCollector creates a new feedback collector
func NewFeedbackCollector(config FeedbackCollectorConfig) *FeedbackCollector {
	if config.MinRating == 0 {
		config.MinRating = 3.0 // Default minimum rating
	}

	return &FeedbackCollector{
		store:     config.Store,
		notifier:  config.Notifier,
		handlers:  make([]FeedbackHandler, 0),
		minRating: config.MinRating,
	}
}

// RegisterHandler registers a feedback handler
func (c *FeedbackCollector) RegisterHandler(handler FeedbackHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers = append(c.handlers, handler)
}

// CollectRating collects a numeric rating
func (c *FeedbackCollector) CollectRating(ctx context.Context, agentID string, output interface{}, rating float64, userID string) (*Feedback, error) {
	feedback := &Feedback{
		ID:         uuid.New().String(),
		Type:       FeedbackTypeRating,
		AgentID:    agentID,
		OutputData: output,
		Value:      rating,
		UserID:     userID,
		CreatedAt:  time.Now(),
	}

	return c.processFeedback(ctx, feedback)
}

// CollectBinary collects thumbs up/down feedback
func (c *FeedbackCollector) CollectBinary(ctx context.Context, agentID string, output interface{}, positive bool, userID string) (*Feedback, error) {
	feedback := &Feedback{
		ID:         uuid.New().String(),
		Type:       FeedbackTypeBinary,
		AgentID:    agentID,
		OutputData: output,
		Value:      positive,
		UserID:     userID,
		CreatedAt:  time.Now(),
	}

	return c.processFeedback(ctx, feedback)
}

// CollectText collects text feedback
func (c *FeedbackCollector) CollectText(ctx context.Context, agentID string, output interface{}, text string, userID string) (*Feedback, error) {
	feedback := &Feedback{
		ID:         uuid.New().String(),
		Type:       FeedbackTypeText,
		AgentID:    agentID,
		OutputData: output,
		Value:      text,
		UserID:     userID,
		CreatedAt:  time.Now(),
	}

	return c.processFeedback(ctx, feedback)
}

// CollectCorrection collects a corrected output
func (c *FeedbackCollector) CollectCorrection(ctx context.Context, agentID string, input, originalOutput, correctedOutput interface{}, userID string) (*Feedback, error) {
	feedback := &Feedback{
		ID:         uuid.New().String(),
		Type:       FeedbackTypeCorrection,
		AgentID:    agentID,
		InputData:  input,
		OutputData: originalOutput,
		Correction: correctedOutput,
		UserID:     userID,
		CreatedAt:  time.Now(),
	}

	return c.processFeedback(ctx, feedback)
}

// CollectPreference collects preference between options
func (c *FeedbackCollector) CollectPreference(ctx context.Context, agentID string, input interface{}, options []interface{}, chosenIndex int, reasoning, userID string) (*Feedback, error) {
	if chosenIndex < 0 || chosenIndex >= len(options) {
		return nil, errors.New("chosen index out of range")
	}

	feedback := &Feedback{
		ID:        uuid.New().String(),
		Type:      FeedbackTypePreference,
		AgentID:   agentID,
		InputData: input,
		Preference: &PreferenceData{
			Options:   options,
			Chosen:    chosenIndex,
			Reasoning: reasoning,
		},
		UserID:    userID,
		CreatedAt: time.Now(),
	}

	return c.processFeedback(ctx, feedback)
}

// CollectLabel collects a category label
func (c *FeedbackCollector) CollectLabel(ctx context.Context, agentID string, output interface{}, label string, userID string) (*Feedback, error) {
	feedback := &Feedback{
		ID:         uuid.New().String(),
		Type:       FeedbackTypeLabel,
		AgentID:    agentID,
		OutputData: output,
		Label:      label,
		UserID:     userID,
		CreatedAt:  time.Now(),
	}

	return c.processFeedback(ctx, feedback)
}

// processFeedback processes and stores feedback
func (c *FeedbackCollector) processFeedback(ctx context.Context, feedback *Feedback) (*Feedback, error) {
	// Store feedback
	if c.store != nil {
		if err := c.store.Store(ctx, feedback); err != nil {
			return nil, fmt.Errorf("failed to store feedback: %w", err)
		}
	}

	// Check for low rating
	if feedback.Type == FeedbackTypeRating {
		if rating, ok := feedback.Value.(float64); ok && rating < c.minRating {
			c.notifyLowRating(ctx, feedback)
		}
	}

	// Check for negative binary feedback
	if feedback.Type == FeedbackTypeBinary {
		if positive, ok := feedback.Value.(bool); ok && !positive {
			c.notifyNegativeFeedback(ctx, feedback)
		}
	}

	// Call handlers
	c.mu.RLock()
	handlers := make([]FeedbackHandler, len(c.handlers))
	copy(handlers, c.handlers)
	c.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler.HandleFeedback(ctx, feedback); err != nil {
			// Log but don't fail
			continue
		}
	}

	return feedback, nil
}

// notifyLowRating sends notification for low rating
func (c *FeedbackCollector) notifyLowRating(ctx context.Context, feedback *Feedback) {
	if c.notifier == nil {
		return
	}

	rating, _ := feedback.Value.(float64)
	notification := &Notification{
		ID:       uuid.New().String(),
		Type:     NotificationTypeWarning,
		Title:    "Low Rating Received",
		Message:  fmt.Sprintf("Agent %s received rating %.1f from user %s", feedback.AgentID, rating, feedback.UserID),
		Priority: NotificationPriorityHigh,
		Metadata: map[string]interface{}{
			"feedback_id": feedback.ID,
			"agent_id":    feedback.AgentID,
			"rating":      rating,
		},
		CreatedAt: time.Now(),
	}

	c.notifier.Notify(ctx, notification)
}

// notifyNegativeFeedback sends notification for negative feedback
func (c *FeedbackCollector) notifyNegativeFeedback(ctx context.Context, feedback *Feedback) {
	if c.notifier == nil {
		return
	}

	notification := &Notification{
		ID:       uuid.New().String(),
		Type:     NotificationTypeWarning,
		Title:    "Negative Feedback Received",
		Message:  fmt.Sprintf("Agent %s received negative feedback from user %s", feedback.AgentID, feedback.UserID),
		Priority: NotificationPriorityMedium,
		Metadata: map[string]interface{}{
			"feedback_id": feedback.ID,
			"agent_id":    feedback.AgentID,
		},
		CreatedAt: time.Now(),
	}

	c.notifier.Notify(ctx, notification)
}

// GetStats returns feedback statistics
func (c *FeedbackCollector) GetStats(ctx context.Context, filter *FeedbackFilter) (*FeedbackStats, error) {
	if c.store == nil {
		return nil, errors.New("no feedback store configured")
	}
	return c.store.GetStats(ctx, filter)
}

// List returns feedback matching the filter
func (c *FeedbackCollector) List(ctx context.Context, filter *FeedbackFilter) ([]*Feedback, error) {
	if c.store == nil {
		return nil, errors.New("no feedback store configured")
	}
	return c.store.List(ctx, filter)
}

// MemoryFeedbackStore is an in-memory feedback store
type MemoryFeedbackStore struct {
	feedback map[string]*Feedback
	mu       sync.RWMutex
}

// NewMemoryFeedbackStore creates a new in-memory feedback store
func NewMemoryFeedbackStore() *MemoryFeedbackStore {
	return &MemoryFeedbackStore{
		feedback: make(map[string]*Feedback),
	}
}

func (s *MemoryFeedbackStore) Store(ctx context.Context, feedback *Feedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.feedback[feedback.ID] = feedback
	return nil
}

func (s *MemoryFeedbackStore) Get(ctx context.Context, id string) (*Feedback, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.feedback[id]
	if !ok {
		return nil, nil
	}
	return f, nil
}

func (s *MemoryFeedbackStore) List(ctx context.Context, filter *FeedbackFilter) ([]*Feedback, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]*Feedback, 0)
	for _, f := range s.feedback {
		if s.matchesFilter(f, filter) {
			results = append(results, f)
		}
	}

	// Apply pagination
	if filter != nil {
		if filter.Offset > 0 && filter.Offset < len(results) {
			results = results[filter.Offset:]
		}
		if filter.Limit > 0 && filter.Limit < len(results) {
			results = results[:filter.Limit]
		}
	}

	return results, nil
}

func (s *MemoryFeedbackStore) matchesFilter(f *Feedback, filter *FeedbackFilter) bool {
	if filter == nil {
		return true
	}
	if filter.AgentID != "" && f.AgentID != filter.AgentID {
		return false
	}
	if filter.TaskID != "" && f.TaskID != filter.TaskID {
		return false
	}
	if filter.SessionID != "" && f.SessionID != filter.SessionID {
		return false
	}
	if filter.Type != "" && f.Type != filter.Type {
		return false
	}
	if filter.UserID != "" && f.UserID != filter.UserID {
		return false
	}
	if filter.StartTime != nil && f.CreatedAt.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && f.CreatedAt.After(*filter.EndTime) {
		return false
	}
	return true
}

func (s *MemoryFeedbackStore) GetStats(ctx context.Context, filter *FeedbackFilter) (*FeedbackStats, error) {
	feedbackList, err := s.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	stats := &FeedbackStats{
		RatingDist:  make(map[int]int64),
		LabelCounts: make(map[string]int64),
		TagCounts:   make(map[string]int64),
	}

	var ratingSum float64
	var ratingCount int64

	for _, f := range feedbackList {
		stats.TotalCount++

		switch f.Type {
		case FeedbackTypeRating:
			if rating, ok := f.Value.(float64); ok {
				ratingSum += rating
				ratingCount++
				stats.RatingDist[int(rating)]++
			}
		case FeedbackTypeBinary:
			if positive, ok := f.Value.(bool); ok {
				if positive {
					stats.PositiveCount++
				} else {
					stats.NegativeCount++
				}
			}
		case FeedbackTypeLabel:
			if f.Label != "" {
				stats.LabelCounts[f.Label]++
			}
		}

		for _, tag := range f.Tags {
			stats.TagCounts[tag]++
		}
	}

	if ratingCount > 0 {
		stats.AverageRating = ratingSum / float64(ratingCount)
	}

	return stats, nil
}

func (s *MemoryFeedbackStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.feedback, id)
	return nil
}

// Ensure MemoryFeedbackStore implements FeedbackStore
var _ FeedbackStore = (*MemoryFeedbackStore)(nil)

// RLHFDataExporter exports feedback data for RLHF training
type RLHFDataExporter struct {
	store FeedbackStore
}

// NewRLHFDataExporter creates a new RLHF data exporter
func NewRLHFDataExporter(store FeedbackStore) *RLHFDataExporter {
	return &RLHFDataExporter{store: store}
}

// RLHFExample represents a training example for RLHF
type RLHFExample struct {
	Input      interface{}   `json:"input"`
	Chosen     interface{}   `json:"chosen"`
	Rejected   interface{}   `json:"rejected,omitempty"`
	Preference float64       `json:"preference,omitempty"` // 0-1 preference score
}

// ExportPreferences exports preference data for RLHF training
func (e *RLHFDataExporter) ExportPreferences(ctx context.Context, filter *FeedbackFilter) ([]RLHFExample, error) {
	if filter == nil {
		filter = &FeedbackFilter{}
	}
	filter.Type = FeedbackTypePreference

	feedbackList, err := e.store.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	examples := make([]RLHFExample, 0, len(feedbackList))
	for _, f := range feedbackList {
		if f.Preference != nil && len(f.Preference.Options) >= 2 {
			example := RLHFExample{
				Input:  f.InputData,
				Chosen: f.Preference.Options[f.Preference.Chosen],
			}
			// Find a rejected option (first non-chosen)
			for i, opt := range f.Preference.Options {
				if i != f.Preference.Chosen {
					example.Rejected = opt
					break
				}
			}
			examples = append(examples, example)
		}
	}

	return examples, nil
}

// ExportCorrections exports correction data for fine-tuning
func (e *RLHFDataExporter) ExportCorrections(ctx context.Context, filter *FeedbackFilter) ([]RLHFExample, error) {
	if filter == nil {
		filter = &FeedbackFilter{}
	}
	filter.Type = FeedbackTypeCorrection

	feedbackList, err := e.store.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	examples := make([]RLHFExample, 0, len(feedbackList))
	for _, f := range feedbackList {
		if f.Correction != nil {
			example := RLHFExample{
				Input:    f.InputData,
				Chosen:   f.Correction, // Human-corrected output
				Rejected: f.OutputData, // Original agent output
			}
			examples = append(examples, example)
		}
	}

	return examples, nil
}
