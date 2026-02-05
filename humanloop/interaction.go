package humanloop

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InteractionType represents the type of human interaction
type InteractionType string

const (
	InteractionTypeQuery       InteractionType = "query"       // Ask human for information
	InteractionTypeConfirm     InteractionType = "confirm"     // Yes/No confirmation
	InteractionTypeChoice      InteractionType = "choice"      // Select from options
	InteractionTypeFreeform    InteractionType = "freeform"    // Free text input
	InteractionTypeCorrection  InteractionType = "correction"  // Correct agent output
	InteractionTypeGuidance    InteractionType = "guidance"    // Provide guidance/steering
)

// InteractionStatus represents the status of an interaction
type InteractionStatus string

const (
	InteractionStatusPending   InteractionStatus = "pending"
	InteractionStatusResponded InteractionStatus = "responded"
	InteractionStatusTimeout   InteractionStatus = "timeout"
	InteractionStatusCancelled InteractionStatus = "cancelled"
)

// InteractionRequest represents a request for human interaction
type InteractionRequest struct {
	ID          string                 `json:"id"`
	Type        InteractionType        `json:"type"`
	Title       string                 `json:"title"`
	Message     string                 `json:"message"`
	Options     []InteractionOption    `json:"options,omitempty"`    // For choice type
	Default     interface{}            `json:"default,omitempty"`    // Default value
	Validation  *ValidationRule        `json:"validation,omitempty"` // For freeform type
	Context     map[string]interface{} `json:"context,omitempty"`
	AgentID     string                 `json:"agent_id,omitempty"`
	TaskID      string                 `json:"task_id,omitempty"`
	Status      InteractionStatus      `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	RespondedAt *time.Time             `json:"responded_at,omitempty"`
}

// InteractionOption represents a choice option
type InteractionOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// ValidationRule defines validation for freeform input
type ValidationRule struct {
	Type      string `json:"type"`      // text, number, email, url, regex
	Pattern   string `json:"pattern,omitempty"` // For regex type
	MinLength int    `json:"min_length,omitempty"`
	MaxLength int    `json:"max_length,omitempty"`
	Min       *float64 `json:"min,omitempty"` // For number type
	Max       *float64 `json:"max,omitempty"`
}

// InteractionResponse represents a human's response
type InteractionResponse struct {
	RequestID  string                 `json:"request_id"`
	Value      interface{}            `json:"value"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	RespondedBy string                `json:"responded_by,omitempty"`
	RespondedAt time.Time             `json:"responded_at"`
}

// InteractionHandler handles human interactions
type InteractionHandler struct {
	requests    map[string]*InteractionRequest
	responses   map[string]*InteractionResponse
	listeners   map[string][]chan *InteractionResponse
	notifier    *NotificationManager
	timeout     time.Duration
	mu          sync.RWMutex
}

// InteractionHandlerConfig configures the interaction handler
type InteractionHandlerConfig struct {
	Notifier       *NotificationManager
	DefaultTimeout time.Duration
}

// NewInteractionHandler creates a new interaction handler
func NewInteractionHandler(config InteractionHandlerConfig) *InteractionHandler {
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 30 * time.Minute
	}

	return &InteractionHandler{
		requests:  make(map[string]*InteractionRequest),
		responses: make(map[string]*InteractionResponse),
		listeners: make(map[string][]chan *InteractionResponse),
		notifier:  config.Notifier,
		timeout:   config.DefaultTimeout,
	}
}

// Query asks a human for information
func (h *InteractionHandler) Query(ctx context.Context, title, message string, context map[string]interface{}) (*InteractionResponse, error) {
	return h.request(ctx, &InteractionRequest{
		Type:    InteractionTypeQuery,
		Title:   title,
		Message: message,
		Context: context,
	})
}

// Confirm asks for yes/no confirmation
func (h *InteractionHandler) Confirm(ctx context.Context, title, message string, context map[string]interface{}) (bool, error) {
	resp, err := h.request(ctx, &InteractionRequest{
		Type:    InteractionTypeConfirm,
		Title:   title,
		Message: message,
		Options: []InteractionOption{
			{Value: "yes", Label: "Yes"},
			{Value: "no", Label: "No"},
		},
		Context: context,
	})
	if err != nil {
		return false, err
	}

	if v, ok := resp.Value.(string); ok {
		return v == "yes", nil
	}
	if v, ok := resp.Value.(bool); ok {
		return v, nil
	}
	return false, errors.New("invalid response type")
}

// Choose presents options to the human
func (h *InteractionHandler) Choose(ctx context.Context, title, message string, options []InteractionOption, context map[string]interface{}) (*InteractionResponse, error) {
	if len(options) == 0 {
		return nil, errors.New("at least one option is required")
	}

	return h.request(ctx, &InteractionRequest{
		Type:    InteractionTypeChoice,
		Title:   title,
		Message: message,
		Options: options,
		Context: context,
	})
}

// Prompt asks for freeform input
func (h *InteractionHandler) Prompt(ctx context.Context, title, message string, validation *ValidationRule, context map[string]interface{}) (*InteractionResponse, error) {
	return h.request(ctx, &InteractionRequest{
		Type:       InteractionTypeFreeform,
		Title:      title,
		Message:    message,
		Validation: validation,
		Context:    context,
	})
}

// RequestCorrection asks human to correct agent output
func (h *InteractionHandler) RequestCorrection(ctx context.Context, title, message string, originalOutput interface{}, context map[string]interface{}) (*InteractionResponse, error) {
	if context == nil {
		context = make(map[string]interface{})
	}
	context["original_output"] = originalOutput

	return h.request(ctx, &InteractionRequest{
		Type:    InteractionTypeCorrection,
		Title:   title,
		Message: message,
		Default: originalOutput,
		Context: context,
	})
}

// RequestGuidance asks human for guidance
func (h *InteractionHandler) RequestGuidance(ctx context.Context, title, message string, suggestedActions []string, context map[string]interface{}) (*InteractionResponse, error) {
	options := make([]InteractionOption, len(suggestedActions))
	for i, action := range suggestedActions {
		options[i] = InteractionOption{
			Value: action,
			Label: action,
		}
	}

	return h.request(ctx, &InteractionRequest{
		Type:    InteractionTypeGuidance,
		Title:   title,
		Message: message,
		Options: options,
		Context: context,
	})
}

// request creates and processes an interaction request
func (h *InteractionHandler) request(ctx context.Context, req *InteractionRequest) (*InteractionResponse, error) {
	// Generate ID and set timestamps
	req.ID = uuid.New().String()
	req.Status = InteractionStatusPending
	req.CreatedAt = time.Now()
	expiresAt := req.CreatedAt.Add(h.timeout)
	req.ExpiresAt = &expiresAt

	// Store request
	h.mu.Lock()
	h.requests[req.ID] = req
	h.mu.Unlock()

	// Create listener channel
	resultChan := make(chan *InteractionResponse, 1)
	h.mu.Lock()
	h.listeners[req.ID] = append(h.listeners[req.ID], resultChan)
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.listeners, req.ID)
		h.mu.Unlock()
	}()

	// Send notification
	h.notifyInteraction(ctx, req)

	// Wait for response
	select {
	case resp := <-resultChan:
		return resp, nil
	case <-time.After(time.Until(*req.ExpiresAt)):
		h.mu.Lock()
		req.Status = InteractionStatusTimeout
		h.mu.Unlock()
		return nil, errors.New("interaction request timed out")
	case <-ctx.Done():
		h.mu.Lock()
		req.Status = InteractionStatusCancelled
		h.mu.Unlock()
		return nil, ctx.Err()
	}
}

// Respond provides a response to an interaction request
func (h *InteractionHandler) Respond(ctx context.Context, requestID string, value interface{}, userID string) error {
	h.mu.Lock()
	req, ok := h.requests[requestID]
	if !ok {
		h.mu.Unlock()
		return fmt.Errorf("interaction request %s not found", requestID)
	}

	if req.Status != InteractionStatusPending {
		h.mu.Unlock()
		return fmt.Errorf("interaction request is already %s", req.Status)
	}

	// Validate response
	if err := h.validateResponse(req, value); err != nil {
		h.mu.Unlock()
		return err
	}

	now := time.Now()
	req.Status = InteractionStatusResponded
	req.RespondedAt = &now

	response := &InteractionResponse{
		RequestID:   requestID,
		Value:       value,
		RespondedBy: userID,
		RespondedAt: now,
	}
	h.responses[requestID] = response

	// Get listeners
	listeners := h.listeners[requestID]
	h.mu.Unlock()

	// Notify listeners
	for _, ch := range listeners {
		select {
		case ch <- response:
		default:
		}
	}

	return nil
}

// validateResponse validates the response against the request
func (h *InteractionHandler) validateResponse(req *InteractionRequest, value interface{}) error {
	switch req.Type {
	case InteractionTypeConfirm:
		// Accept string "yes"/"no" or bool
		if v, ok := value.(string); ok {
			if v != "yes" && v != "no" {
				return errors.New("confirm response must be 'yes' or 'no'")
			}
		} else if _, ok := value.(bool); !ok {
			return errors.New("confirm response must be boolean or 'yes'/'no'")
		}

	case InteractionTypeChoice:
		v, ok := value.(string)
		if !ok {
			return errors.New("choice response must be a string")
		}
		// Check if value is a valid option
		valid := false
		for _, opt := range req.Options {
			if opt.Value == v {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid choice: %s", v)
		}

	case InteractionTypeFreeform:
		if req.Validation != nil {
			return h.validateFreeform(value, req.Validation)
		}
	}

	return nil
}

// validateFreeform validates freeform input
func (h *InteractionHandler) validateFreeform(value interface{}, rule *ValidationRule) error {
	switch rule.Type {
	case "text":
		v, ok := value.(string)
		if !ok {
			return errors.New("expected text input")
		}
		if rule.MinLength > 0 && len(v) < rule.MinLength {
			return fmt.Errorf("input must be at least %d characters", rule.MinLength)
		}
		if rule.MaxLength > 0 && len(v) > rule.MaxLength {
			return fmt.Errorf("input must be at most %d characters", rule.MaxLength)
		}

	case "number":
		var num float64
		switch v := value.(type) {
		case float64:
			num = v
		case int:
			num = float64(v)
		case int64:
			num = float64(v)
		default:
			return errors.New("expected numeric input")
		}
		if rule.Min != nil && num < *rule.Min {
			return fmt.Errorf("value must be at least %f", *rule.Min)
		}
		if rule.Max != nil && num > *rule.Max {
			return fmt.Errorf("value must be at most %f", *rule.Max)
		}
	}

	return nil
}

// GetRequest returns an interaction request by ID
func (h *InteractionHandler) GetRequest(id string) (*InteractionRequest, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	req, ok := h.requests[id]
	return req, ok
}

// GetResponse returns a response by request ID
func (h *InteractionHandler) GetResponse(requestID string) (*InteractionResponse, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	resp, ok := h.responses[requestID]
	return resp, ok
}

// GetPendingRequests returns all pending interaction requests
func (h *InteractionHandler) GetPendingRequests() []*InteractionRequest {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*InteractionRequest, 0)
	for _, req := range h.requests {
		if req.Status == InteractionStatusPending {
			result = append(result, req)
		}
	}
	return result
}

// Cancel cancels an interaction request
func (h *InteractionHandler) Cancel(ctx context.Context, requestID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	req, ok := h.requests[requestID]
	if !ok {
		return fmt.Errorf("interaction request %s not found", requestID)
	}

	if req.Status != InteractionStatusPending {
		return fmt.Errorf("interaction request is already %s", req.Status)
	}

	req.Status = InteractionStatusCancelled

	// Notify listeners with nil to signal cancellation
	listeners := h.listeners[requestID]
	for _, ch := range listeners {
		close(ch)
	}

	return nil
}

// notifyInteraction sends a notification for an interaction request
func (h *InteractionHandler) notifyInteraction(ctx context.Context, req *InteractionRequest) {
	if h.notifier == nil {
		return
	}

	notification := &Notification{
		ID:       uuid.New().String(),
		Type:     NotificationTypeFeedbackRequest,
		Title:    req.Title,
		Message:  req.Message,
		Priority: NotificationPriorityHigh,
		Metadata: map[string]interface{}{
			"request_id":       req.ID,
			"interaction_type": req.Type,
			"agent_id":         req.AgentID,
			"task_id":          req.TaskID,
		},
		CreatedAt: time.Now(),
		ExpiresAt: req.ExpiresAt,
	}

	h.notifier.Notify(ctx, notification)
}

// AgentSteerer provides steering capabilities for agents
type AgentSteerer struct {
	handler *InteractionHandler
	agentID string
}

// NewAgentSteerer creates a new agent steerer
func NewAgentSteerer(handler *InteractionHandler, agentID string) *AgentSteerer {
	return &AgentSteerer{
		handler: handler,
		agentID: agentID,
	}
}

// AskHuman asks the human a question
func (s *AgentSteerer) AskHuman(ctx context.Context, question string, context map[string]interface{}) (string, error) {
	resp, err := s.handler.Query(ctx, "Agent Question", question, context)
	if err != nil {
		return "", err
	}
	if v, ok := resp.Value.(string); ok {
		return v, nil
	}
	return fmt.Sprintf("%v", resp.Value), nil
}

// GetApproval gets human approval
func (s *AgentSteerer) GetApproval(ctx context.Context, action, reason string, context map[string]interface{}) (bool, error) {
	message := fmt.Sprintf("Action: %s\nReason: %s", action, reason)
	return s.handler.Confirm(ctx, "Approval Required", message, context)
}

// GetGuidance gets guidance from human
func (s *AgentSteerer) GetGuidance(ctx context.Context, situation string, options []string, context map[string]interface{}) (string, error) {
	resp, err := s.handler.RequestGuidance(ctx, "Guidance Needed", situation, options, context)
	if err != nil {
		return "", err
	}
	if v, ok := resp.Value.(string); ok {
		return v, nil
	}
	return "", errors.New("invalid guidance response")
}

// RequestCorrection requests correction of output
func (s *AgentSteerer) RequestCorrection(ctx context.Context, description string, output interface{}, context map[string]interface{}) (interface{}, error) {
	resp, err := s.handler.RequestCorrection(ctx, "Correction Needed", description, output, context)
	if err != nil {
		return nil, err
	}
	return resp.Value, nil
}
