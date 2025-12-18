package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Ranganaths/minion/observability"
	"github.com/google/uuid"
)

// WorkerAgent represents a specialized worker agent
type WorkerAgent struct {
	metadata    *AgentMetadata
	protocol    Protocol
	taskHandler TaskHandler
	running     bool
	stopCh      chan struct{}
	metrics     *observability.MetricsCollector
	tracer      *observability.Tracer
}

// TaskHandler defines how a worker processes tasks
type TaskHandler interface {
	// HandleTask processes a task and returns the result
	HandleTask(ctx context.Context, task *Task) (interface{}, error)

	// GetCapabilities returns the capabilities this handler supports
	GetCapabilities() []string

	// GetName returns the handler name
	GetName() string
}

// NewWorkerAgent creates a new worker agent
func NewWorkerAgent(metadata *AgentMetadata, protocol Protocol, handler TaskHandler) *WorkerAgent {
	if metadata.AgentID == "" {
		metadata.AgentID = uuid.New().String()
	}

	return &WorkerAgent{
		metadata:    metadata,
		protocol:    protocol,
		taskHandler: handler,
		stopCh:      make(chan struct{}),
		metrics:     observability.GetMetrics(),
		tracer:      observability.GetTracer(),
	}
}

// Start starts the worker agent
func (w *WorkerAgent) Start(ctx context.Context) error {
	w.running = true

	// Subscribe to task messages
	err := w.protocol.Subscribe(ctx, w.metadata.AgentID, []MessageType{
		MessageTypeTask,
		MessageTypeDelegate,
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Start message processing loop
	go w.processMessages(ctx)

	return nil
}

// Stop stops the worker agent
func (w *WorkerAgent) Stop(ctx context.Context) error {
	w.running = false
	close(w.stopCh)

	return w.protocol.Unsubscribe(ctx, w.metadata.AgentID)
}

// processMessages processes incoming messages
func (w *WorkerAgent) processMessages(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for w.running {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			messages, err := w.protocol.Receive(ctx, w.metadata.AgentID)
			if err != nil {
				continue
			}

			for _, msg := range messages {
				w.handleMessage(ctx, msg)
			}
		case <-ctx.Done():
			return
		}
	}
}

// handleMessage handles a single message
func (w *WorkerAgent) handleMessage(ctx context.Context, msg *Message) {
	switch msg.Type {
	case MessageTypeTask:
		w.handleTaskMessage(ctx, msg)
	case MessageTypeDelegate:
		w.handleDelegateMessage(ctx, msg)
	}
}

// handleTaskMessage handles a task message
func (w *WorkerAgent) handleTaskMessage(ctx context.Context, msg *Message) {
	// Extract task from message
	var task *Task

	// Handle both *Task and Task types
	switch v := msg.Content.(type) {
	case *Task:
		task = v
	case Task:
		task = &v
	case map[string]interface{}:
		// Try to convert from map (in case of JSON serialization)
		taskBytes, _ := json.Marshal(v)
		task = &Task{}
		if err := json.Unmarshal(taskBytes, task); err != nil {
			w.sendErrorResponse(ctx, msg, fmt.Errorf("invalid task format: %w", err), "")
			return
		}
	default:
		w.sendErrorResponse(ctx, msg, fmt.Errorf("invalid task format: unexpected type %T", msg.Content), "")
		return
	}

	// Start tracing span
	capability := "unknown"
	if len(w.metadata.Capabilities) > 0 {
		capability = w.metadata.Capabilities[0]
	}
	ctx, span := w.tracer.StartWorkerSpan(ctx, w.metadata.AgentID, capability, task.ID)
	defer w.tracer.EndSpan(span, nil)

	// Update agent status
	w.metadata.Status = StatusBusy
	w.metrics.RecordMultiagentWorkerBusy()

	// Track processing duration
	start := time.Now()

	// Process task
	result, err := w.taskHandler.HandleTask(ctx, task)

	// Record processing duration
	duration := time.Since(start)
	// Use the LLM request recording since we don't have a specific worker processing duration metric yet
	// This records the work that was done
	if err == nil {
		w.metrics.RecordLLMRequest("worker", capability, duration, 0, 0, 0, nil)
	}

	// Update agent status
	w.metadata.Status = StatusIdle
	w.metrics.RecordMultiagentWorkerIdle()

	// Send response - use task.ID for InReplyTo so orchestrator can match it
	if err != nil {
		w.metrics.RecordMultiagentError("worker", "task_processing_failed")
		w.sendErrorResponse(ctx, msg, err, task.ID)
		return
	}

	w.sendSuccessResponse(ctx, msg, result, task.ID)
}

// handleDelegateMessage handles a delegate message
func (w *WorkerAgent) handleDelegateMessage(ctx context.Context, msg *Message) {
	// Similar to handleTaskMessage, but with delegation semantics
	w.handleTaskMessage(ctx, msg)
}

// sendSuccessResponse sends a success response
func (w *WorkerAgent) sendSuccessResponse(ctx context.Context, originalMsg *Message, result interface{}, taskID string) {
	response := &Message{
		ID:        uuid.New().String(),
		Type:      MessageTypeResult,
		From:      w.metadata.AgentID,
		To:        originalMsg.From,
		InReplyTo: taskID, // Use task ID for matching
		Content:   result,
		Metadata: map[string]interface{}{
			"original_message_id": originalMsg.ID,
		},
		CreatedAt: time.Now(),
	}

	w.protocol.Send(ctx, response)
}

// sendErrorResponse sends an error response
func (w *WorkerAgent) sendErrorResponse(ctx context.Context, originalMsg *Message, err error, taskID string) {
	response := &Message{
		ID:        uuid.New().String(),
		Type:      MessageTypeError,
		From:      w.metadata.AgentID,
		To:        originalMsg.From,
		InReplyTo: taskID, // Use task ID for matching
		Content:   err.Error(),
		Metadata: map[string]interface{}{
			"original_message_id": originalMsg.ID,
		},
		CreatedAt: time.Now(),
	}

	w.protocol.Send(ctx, response)
}

// GetMetadata returns the agent metadata
func (w *WorkerAgent) GetMetadata() *AgentMetadata {
	return w.metadata
}

// --- Specialized Worker Implementations ---

// CoderWorker handles code generation and execution tasks
type CoderWorker struct {
	llmProvider LLMProvider
}

// NewCoderWorker creates a new coder worker
func NewCoderWorker(llmProvider LLMProvider) *CoderWorker {
	return &CoderWorker{
		llmProvider: llmProvider,
	}
}

// HandleTask handles code generation tasks
func (c *CoderWorker) HandleTask(ctx context.Context, task *Task) (interface{}, error) {
	// Generate code using LLM
	systemPrompt := `You are an expert programmer. Generate clean, efficient, and well-documented code.
Output only the code without explanations unless specifically requested.`

	userPrompt := fmt.Sprintf("Task: %s\n\nDescription: %s\n\nInput: %v\n\nGenerate the required code.",
		task.Name, task.Description, task.Input)

	resp, err := c.llmProvider.GenerateCompletion(ctx, &CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.2,
		MaxTokens:    2000,
		Model:        "gpt-4",
	})
	if err != nil {
		return nil, fmt.Errorf("code generation failed: %w", err)
	}

	return map[string]interface{}{
		"code":        resp.Text,
		"language":    "auto-detected",
		"tokens_used": resp.TokensUsed,
	}, nil
}

// GetCapabilities returns coder capabilities
func (c *CoderWorker) GetCapabilities() []string {
	return []string{"code_generation", "code_review", "debugging", "refactoring"}
}

// GetName returns the handler name
func (c *CoderWorker) GetName() string {
	return "coder"
}

// AnalystWorker handles data analysis tasks
type AnalystWorker struct {
	llmProvider LLMProvider
}

// NewAnalystWorker creates a new analyst worker
func NewAnalystWorker(llmProvider LLMProvider) *AnalystWorker {
	return &AnalystWorker{
		llmProvider: llmProvider,
	}
}

// HandleTask handles data analysis tasks
func (a *AnalystWorker) HandleTask(ctx context.Context, task *Task) (interface{}, error) {
	systemPrompt := `You are an expert data analyst. Analyze data and provide insights.
Provide clear explanations and actionable recommendations.`

	userPrompt := fmt.Sprintf("Task: %s\n\nDescription: %s\n\nData: %v\n\nProvide your analysis.",
		task.Name, task.Description, task.Input)

	resp, err := a.llmProvider.GenerateCompletion(ctx, &CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.3,
		MaxTokens:    1500,
		Model:        "gpt-4",
	})
	if err != nil {
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	return map[string]interface{}{
		"analysis":    resp.Text,
		"confidence":  "high",
		"tokens_used": resp.TokensUsed,
	}, nil
}

// GetCapabilities returns analyst capabilities
func (a *AnalystWorker) GetCapabilities() []string {
	return []string{"data_analysis", "statistical_analysis", "forecasting", "visualization"}
}

// GetName returns the handler name
func (a *AnalystWorker) GetName() string {
	return "analyst"
}

// ResearcherWorker handles research and information gathering tasks
type ResearcherWorker struct {
	llmProvider LLMProvider
}

// NewResearcherWorker creates a new researcher worker
func NewResearcherWorker(llmProvider LLMProvider) *ResearcherWorker {
	return &ResearcherWorker{
		llmProvider: llmProvider,
	}
}

// HandleTask handles research tasks
func (r *ResearcherWorker) HandleTask(ctx context.Context, task *Task) (interface{}, error) {
	systemPrompt := `You are an expert researcher. Gather information, synthesize findings, and provide comprehensive summaries.
Include sources and evidence for your claims.`

	userPrompt := fmt.Sprintf("Task: %s\n\nDescription: %s\n\nQuery: %v\n\nProvide your research findings.",
		task.Name, task.Description, task.Input)

	resp, err := r.llmProvider.GenerateCompletion(ctx, &CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.4,
		MaxTokens:    2000,
		Model:        "gpt-4",
	})
	if err != nil {
		return nil, fmt.Errorf("research failed: %w", err)
	}

	return map[string]interface{}{
		"findings":    resp.Text,
		"sources":     []string{}, // TODO: Extract sources from response
		"tokens_used": resp.TokensUsed,
	}, nil
}

// GetCapabilities returns researcher capabilities
func (r *ResearcherWorker) GetCapabilities() []string {
	return []string{"research", "information_gathering", "synthesis", "summarization"}
}

// GetName returns the handler name
func (r *ResearcherWorker) GetName() string {
	return "researcher"
}

// WriterWorker handles content creation and writing tasks
type WriterWorker struct {
	llmProvider LLMProvider
}

// NewWriterWorker creates a new writer worker
func NewWriterWorker(llmProvider LLMProvider) *WriterWorker {
	return &WriterWorker{
		llmProvider: llmProvider,
	}
}

// HandleTask handles writing tasks
func (w *WriterWorker) HandleTask(ctx context.Context, task *Task) (interface{}, error) {
	systemPrompt := `You are an expert content writer. Create engaging, clear, and well-structured content.
Adapt your style to the target audience and purpose.`

	userPrompt := fmt.Sprintf("Task: %s\n\nDescription: %s\n\nRequirements: %v\n\nCreate the content.",
		task.Name, task.Description, task.Input)

	resp, err := w.llmProvider.GenerateCompletion(ctx, &CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.7,
		MaxTokens:    2000,
		Model:        "gpt-4",
	})
	if err != nil {
		return nil, fmt.Errorf("writing failed: %w", err)
	}

	return map[string]interface{}{
		"content":     resp.Text,
		"word_count":  len(resp.Text) / 5, // Rough estimate
		"tokens_used": resp.TokensUsed,
	}, nil
}

// GetCapabilities returns writer capabilities
func (w *WriterWorker) GetCapabilities() []string {
	return []string{"content_creation", "editing", "copywriting", "technical_writing"}
}

// GetName returns the handler name
func (w *WriterWorker) GetName() string {
	return "writer"
}

// ReviewerWorker handles review and quality assurance tasks
type ReviewerWorker struct {
	llmProvider LLMProvider
}

// NewReviewerWorker creates a new reviewer worker
func NewReviewerWorker(llmProvider LLMProvider) *ReviewerWorker {
	return &ReviewerWorker{
		llmProvider: llmProvider,
	}
}

// HandleTask handles review tasks
func (r *ReviewerWorker) HandleTask(ctx context.Context, task *Task) (interface{}, error) {
	systemPrompt := `You are an expert reviewer. Critically evaluate content, code, or work products.
Provide constructive feedback and specific recommendations for improvement.`

	userPrompt := fmt.Sprintf("Task: %s\n\nDescription: %s\n\nContent to Review: %v\n\nProvide your review.",
		task.Name, task.Description, task.Input)

	resp, err := r.llmProvider.GenerateCompletion(ctx, &CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.3,
		MaxTokens:    1500,
		Model:        "gpt-4",
	})
	if err != nil {
		return nil, fmt.Errorf("review failed: %w", err)
	}

	return map[string]interface{}{
		"review":      resp.Text,
		"rating":      "pending", // TODO: Parse rating from response
		"tokens_used": resp.TokensUsed,
	}, nil
}

// GetCapabilities returns reviewer capabilities
func (r *ReviewerWorker) GetCapabilities() []string {
	return []string{"code_review", "content_review", "quality_assurance", "testing"}
}

// GetName returns the handler name
func (r *ReviewerWorker) GetName() string {
	return "reviewer"
}
