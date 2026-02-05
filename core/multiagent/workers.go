package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Ranganaths/minion/observability"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

// WorkerAgent represents a specialized worker agent
type WorkerAgent struct {
	metadata    *AgentMetadata
	protocol    Protocol
	taskHandler TaskHandler
	running     atomic.Bool
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
	w.running.Store(true)

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
	w.running.Store(false)
	close(w.stopCh)

	return w.protocol.Unsubscribe(ctx, w.metadata.AgentID)
}

// processMessages processes incoming messages
func (w *WorkerAgent) processMessages(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for w.running.Load() {
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
			w.sendErrorResponse(ctx, msg, fmt.Errorf("invalid task format: %w", err), "", msg.TraceContext)
			return
		}
	default:
		w.sendErrorResponse(ctx, msg, fmt.Errorf("invalid task format: unexpected type %T", msg.Content), "", msg.TraceContext)
		return
	}

	// Extract trace context from message for distributed tracing
	var traceContext *TraceContext
	if msg.TraceContext != nil {
		traceContext = msg.TraceContext
	} else if task.TraceContext != nil {
		traceContext = task.TraceContext
	}

	// Start tracing span with parent context if available
	capability := "unknown"
	if len(w.metadata.Capabilities) > 0 {
		capability = w.metadata.Capabilities[0]
	}
	ctx, span := w.tracer.StartWorkerSpan(ctx, w.metadata.AgentID, capability, task.ID)
	defer w.tracer.EndSpan(span, nil)

	// Add trace correlation attributes for agent traceability
	if traceContext != nil {
		span.SetAttributes(
			attribute.String(observability.AttrRootTraceID, traceContext.RootTraceID),
			attribute.String(observability.AttrParentSpanID, traceContext.ParentSpanID),
			attribute.String(observability.AttrOrchestratorID, traceContext.OrchestratorID),
			attribute.String(observability.AttrExecutionID, traceContext.ExecutionID),
		)

		// Add baggage items as attributes
		if traceContext.Baggage != nil {
			for key, value := range traceContext.Baggage {
				span.SetAttributes(attribute.String("baggage."+key, value))
			}
		}
	}

	// Update agent status
	w.metadata.Status = StatusBusy
	w.metrics.RecordMultiagentWorkerBusy()

	// Record worker started event
	RecordExecutionEvent(ctx, &ExecutionEvent{
		ID:           uuid.New().String(),
		Type:         EventTypeWorkerStarted,
		TaskID:       task.ID,
		AgentID:      w.metadata.AgentID,
		AgentRole:    w.metadata.Role,
		Action:       "start_task",
		Input:        task.Input,
		Timestamp:    time.Now(),
		TraceContext: traceContext,
		Metadata: map[string]interface{}{
			"capability": capability,
			"task_name":  task.Name,
		},
	})

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

		// Record worker failed event
		RecordExecutionEvent(ctx, &ExecutionEvent{
			ID:           uuid.New().String(),
			Type:         EventTypeTaskFailed,
			TaskID:       task.ID,
			AgentID:      w.metadata.AgentID,
			AgentRole:    w.metadata.Role,
			Action:       "task_failed",
			Error:        err.Error(),
			Duration:     duration,
			Timestamp:    time.Now(),
			TraceContext: traceContext,
			Metadata: map[string]interface{}{
				"capability": capability,
				"task_name":  task.Name,
			},
		})

		w.sendErrorResponse(ctx, msg, err, task.ID, traceContext)
		return
	}

	// Record worker completed event
	RecordExecutionEvent(ctx, &ExecutionEvent{
		ID:           uuid.New().String(),
		Type:         EventTypeWorkerCompleted,
		TaskID:       task.ID,
		AgentID:      w.metadata.AgentID,
		AgentRole:    w.metadata.Role,
		Action:       "task_completed",
		Output:       result,
		Duration:     duration,
		Timestamp:    time.Now(),
		TraceContext: traceContext,
		Metadata: map[string]interface{}{
			"capability": capability,
			"task_name":  task.Name,
		},
	})

	w.sendSuccessResponse(ctx, msg, result, task.ID, traceContext)
}

// handleDelegateMessage handles a delegate message
func (w *WorkerAgent) handleDelegateMessage(ctx context.Context, msg *Message) {
	// Similar to handleTaskMessage, but with delegation semantics
	w.handleTaskMessage(ctx, msg)
}

// sendSuccessResponse sends a success response with trace context propagation
func (w *WorkerAgent) sendSuccessResponse(ctx context.Context, originalMsg *Message, result interface{}, taskID string, traceContext *TraceContext) {
	// Create response trace context preserving the chain
	var responseTraceContext *TraceContext
	if traceContext != nil {
		responseTraceContext = &TraceContext{
			TraceID:        w.tracer.GetTraceID(ctx),
			SpanID:         w.tracer.GetSpanID(ctx),
			RootTraceID:    traceContext.RootTraceID,
			ParentSpanID:   traceContext.SpanID, // Link back to parent
			OrchestratorID: traceContext.OrchestratorID,
			ExecutionID:    traceContext.ExecutionID,
			Baggage:        traceContext.Baggage,
		}
	}

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
		CreatedAt:    time.Now(),
		TraceContext: responseTraceContext,
	}

	if err := w.protocol.Send(ctx, response); err != nil {
		// Log the error but don't fail - the orchestrator will timeout if needed
		w.metrics.RecordMultiagentError("worker", "send_response_failed")
	}
}

// sendErrorResponse sends an error response with trace context propagation
func (w *WorkerAgent) sendErrorResponse(ctx context.Context, originalMsg *Message, err error, taskID string, traceContext *TraceContext) {
	// Create response trace context preserving the chain
	var responseTraceContext *TraceContext
	if traceContext != nil {
		responseTraceContext = &TraceContext{
			TraceID:        w.tracer.GetTraceID(ctx),
			SpanID:         w.tracer.GetSpanID(ctx),
			RootTraceID:    traceContext.RootTraceID,
			ParentSpanID:   traceContext.SpanID, // Link back to parent
			OrchestratorID: traceContext.OrchestratorID,
			ExecutionID:    traceContext.ExecutionID,
			Baggage:        traceContext.Baggage,
		}
	}

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
		CreatedAt:    time.Now(),
		TraceContext: responseTraceContext,
	}

	if err := w.protocol.Send(ctx, response); err != nil {
		// Log the error but don't fail - the orchestrator will timeout if needed
		w.metrics.RecordMultiagentError("worker", "send_error_response_failed")
	}
}

// GetMetadata returns the agent metadata
func (w *WorkerAgent) GetMetadata() *AgentMetadata {
	return w.metadata
}

// --- Specialized Worker Implementations ---

// WorkerConfig holds configuration for worker implementations
type WorkerConfig struct {
	Model       string  `json:"model"`       // LLM model to use (empty = provider default)
	Temperature float64 `json:"temperature"` // Temperature for generation
	MaxTokens   int     `json:"max_tokens"`  // Maximum tokens for response
}

// DefaultWorkerConfig returns default worker configuration
func DefaultWorkerConfig() *WorkerConfig {
	return &WorkerConfig{
		Model:       "", // Use provider's default
		Temperature: 0.3,
		MaxTokens:   2000,
	}
}

// CoderWorker handles code generation and execution tasks
type CoderWorker struct {
	llmProvider LLMProvider
	config      *WorkerConfig
}

// NewCoderWorker creates a new coder worker
func NewCoderWorker(llmProvider LLMProvider) *CoderWorker {
	return &CoderWorker{
		llmProvider: llmProvider,
		config:      DefaultWorkerConfig(),
	}
}

// NewCoderWorkerWithConfig creates a new coder worker with custom config
func NewCoderWorkerWithConfig(llmProvider LLMProvider, config *WorkerConfig) *CoderWorker {
	if config == nil {
		config = DefaultWorkerConfig()
	}
	return &CoderWorker{
		llmProvider: llmProvider,
		config:      config,
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
		MaxTokens:    c.config.MaxTokens,
		Model:        c.config.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("code generation failed: %w", err)
	}

	// Detect programming language from code content
	language := detectProgrammingLanguage(resp.Text)

	return map[string]interface{}{
		"code":        resp.Text,
		"language":    language,
		"tokens_used": resp.TokensUsed,
		"model":       resp.Model,
	}, nil
}

// detectProgrammingLanguage attempts to detect the programming language from code
func detectProgrammingLanguage(code string) string {
	code = strings.TrimSpace(code)

	// Check for common language indicators
	switch {
	case strings.HasPrefix(code, "package ") && strings.Contains(code, "func "):
		return "go"
	case strings.HasPrefix(code, "#!/usr/bin/python") || strings.HasPrefix(code, "#!/usr/bin/env python"):
		return "python"
	case strings.Contains(code, "def ") && strings.Contains(code, ":"):
		return "python"
	case strings.HasPrefix(code, "#!/bin/bash") || strings.HasPrefix(code, "#!/bin/sh"):
		return "bash"
	case strings.Contains(code, "function ") && strings.Contains(code, "const "):
		return "javascript"
	case strings.Contains(code, "interface ") && strings.Contains(code, ": "):
		return "typescript"
	case strings.Contains(code, "public class ") || strings.Contains(code, "private class "):
		return "java"
	case strings.Contains(code, "#include") && strings.Contains(code, "int main"):
		return "c"
	case strings.Contains(code, "#include") && strings.Contains(code, "std::"):
		return "cpp"
	case strings.HasPrefix(code, "<?php"):
		return "php"
	case strings.Contains(code, "fn ") && strings.Contains(code, "let "):
		return "rust"
	default:
		return "unknown"
	}
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
	config      *WorkerConfig
}

// NewAnalystWorker creates a new analyst worker
func NewAnalystWorker(llmProvider LLMProvider) *AnalystWorker {
	return &AnalystWorker{
		llmProvider: llmProvider,
		config:      DefaultWorkerConfig(),
	}
}

// NewAnalystWorkerWithConfig creates a new analyst worker with custom config
func NewAnalystWorkerWithConfig(llmProvider LLMProvider, config *WorkerConfig) *AnalystWorker {
	if config == nil {
		config = DefaultWorkerConfig()
	}
	return &AnalystWorker{
		llmProvider: llmProvider,
		config:      config,
	}
}

// HandleTask handles data analysis tasks
func (a *AnalystWorker) HandleTask(ctx context.Context, task *Task) (interface{}, error) {
	systemPrompt := `You are an expert data analyst. Analyze data and provide insights.
Provide clear explanations and actionable recommendations.
Include a confidence assessment (high/medium/low) based on data quality and completeness.`

	userPrompt := fmt.Sprintf("Task: %s\n\nDescription: %s\n\nData: %v\n\nProvide your analysis with confidence assessment.",
		task.Name, task.Description, task.Input)

	resp, err := a.llmProvider.GenerateCompletion(ctx, &CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  a.config.Temperature,
		MaxTokens:    a.config.MaxTokens,
		Model:        a.config.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	// Extract confidence from the response
	confidence := extractConfidenceFromText(resp.Text)

	return map[string]interface{}{
		"analysis":    resp.Text,
		"confidence":  confidence,
		"tokens_used": resp.TokensUsed,
		"model":       resp.Model,
	}, nil
}

// extractConfidenceFromText extracts confidence level from analysis text
func extractConfidenceFromText(text string) string {
	lowerText := strings.ToLower(text)

	// Look for explicit confidence mentions
	confidencePatterns := []struct {
		pattern string
		level   string
	}{
		{"confidence: high", "high"},
		{"confidence: medium", "medium"},
		{"confidence: low", "low"},
		{"high confidence", "high"},
		{"medium confidence", "medium"},
		{"low confidence", "low"},
		{"confident", "high"},
		{"uncertain", "low"},
		{"limited data", "low"},
		{"insufficient", "low"},
		{"strong evidence", "high"},
		{"weak evidence", "low"},
	}

	for _, p := range confidencePatterns {
		if strings.Contains(lowerText, p.pattern) {
			return p.level
		}
	}

	// Default to medium if no explicit confidence found
	return "medium"
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
	config      *WorkerConfig
}

// NewResearcherWorker creates a new researcher worker
func NewResearcherWorker(llmProvider LLMProvider) *ResearcherWorker {
	return &ResearcherWorker{
		llmProvider: llmProvider,
		config:      DefaultWorkerConfig(),
	}
}

// NewResearcherWorkerWithConfig creates a new researcher worker with custom config
func NewResearcherWorkerWithConfig(llmProvider LLMProvider, config *WorkerConfig) *ResearcherWorker {
	if config == nil {
		config = DefaultWorkerConfig()
	}
	return &ResearcherWorker{
		llmProvider: llmProvider,
		config:      config,
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
		Temperature:  r.config.Temperature,
		MaxTokens:    r.config.MaxTokens,
		Model:        r.config.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("research failed: %w", err)
	}

	// Extract sources from the response text
	sources := extractSourcesFromText(resp.Text)

	return map[string]interface{}{
		"findings":    resp.Text,
		"sources":     sources,
		"tokens_used": resp.TokensUsed,
		"model":       resp.Model,
	}, nil
}

// extractSourcesFromText extracts URLs and citations from text
func extractSourcesFromText(text string) []string {
	sources := make([]string, 0)
	seen := make(map[string]bool)

	// Pattern for URLs
	urlPattern := regexp.MustCompile(`https?://[^\s\)\]\>\"\']+`)
	urls := urlPattern.FindAllString(text, -1)
	for _, url := range urls {
		// Clean up trailing punctuation
		url = strings.TrimRight(url, ".,;:!?")
		if !seen[url] {
			sources = append(sources, url)
			seen[url] = true
		}
	}

	// Pattern for academic-style citations [Author, Year] or (Author, Year)
	citationPattern := regexp.MustCompile(`[\[\(]([A-Z][a-zA-Z]+(?:\s+(?:et\s+al\.?|&|and)\s+[A-Z][a-zA-Z]+)*,?\s*\d{4}[a-z]?)[\]\)]`)
	citations := citationPattern.FindAllStringSubmatch(text, -1)
	for _, match := range citations {
		if len(match) > 1 && !seen[match[1]] {
			sources = append(sources, match[1])
			seen[match[1]] = true
		}
	}

	// Pattern for numbered references like [1], [2]
	refPattern := regexp.MustCompile(`\[(\d+)\]`)
	refs := refPattern.FindAllStringSubmatch(text, -1)
	for _, match := range refs {
		if len(match) > 1 {
			ref := "Reference " + match[1]
			if !seen[ref] {
				sources = append(sources, ref)
				seen[ref] = true
			}
		}
	}

	return sources
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
	config      *WorkerConfig
}

// NewWriterWorker creates a new writer worker
func NewWriterWorker(llmProvider LLMProvider) *WriterWorker {
	config := DefaultWorkerConfig()
	config.Temperature = 0.7 // Higher creativity for writing
	return &WriterWorker{
		llmProvider: llmProvider,
		config:      config,
	}
}

// NewWriterWorkerWithConfig creates a new writer worker with custom config
func NewWriterWorkerWithConfig(llmProvider LLMProvider, config *WorkerConfig) *WriterWorker {
	if config == nil {
		config = DefaultWorkerConfig()
		config.Temperature = 0.7
	}
	return &WriterWorker{
		llmProvider: llmProvider,
		config:      config,
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
		Temperature:  w.config.Temperature,
		MaxTokens:    w.config.MaxTokens,
		Model:        w.config.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("writing failed: %w", err)
	}

	// Count words properly by splitting on whitespace
	wordCount := countWords(resp.Text)

	return map[string]interface{}{
		"content":     resp.Text,
		"word_count":  wordCount,
		"tokens_used": resp.TokensUsed,
		"model":       resp.Model,
	}, nil
}

// countWords counts the number of words in a text
func countWords(text string) int {
	// Split on whitespace and count non-empty parts
	words := strings.Fields(text)
	return len(words)
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
	config      *WorkerConfig
}

// NewReviewerWorker creates a new reviewer worker
func NewReviewerWorker(llmProvider LLMProvider) *ReviewerWorker {
	return &ReviewerWorker{
		llmProvider: llmProvider,
		config:      DefaultWorkerConfig(),
	}
}

// NewReviewerWorkerWithConfig creates a new reviewer worker with custom config
func NewReviewerWorkerWithConfig(llmProvider LLMProvider, config *WorkerConfig) *ReviewerWorker {
	if config == nil {
		config = DefaultWorkerConfig()
	}
	return &ReviewerWorker{
		llmProvider: llmProvider,
		config:      config,
	}
}

// HandleTask handles review tasks
func (r *ReviewerWorker) HandleTask(ctx context.Context, task *Task) (interface{}, error) {
	systemPrompt := `You are an expert reviewer. Critically evaluate content, code, or work products.
Provide constructive feedback and specific recommendations for improvement.
Include a rating in your review (e.g., "Rating: 8/10" or "Grade: A").`

	userPrompt := fmt.Sprintf("Task: %s\n\nDescription: %s\n\nContent to Review: %v\n\nProvide your review with rating.",
		task.Name, task.Description, task.Input)

	resp, err := r.llmProvider.GenerateCompletion(ctx, &CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  r.config.Temperature,
		MaxTokens:    r.config.MaxTokens,
		Model:        r.config.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("review failed: %w", err)
	}

	// Extract rating from the review text
	rating := extractRatingFromText(resp.Text)

	return map[string]interface{}{
		"review":      resp.Text,
		"rating":      rating,
		"tokens_used": resp.TokensUsed,
		"model":       resp.Model,
	}, nil
}

// extractRatingFromText extracts a rating from review text
func extractRatingFromText(text string) string {
	lowerText := strings.ToLower(text)

	// Look for explicit numeric ratings (e.g., "8/10", "4 out of 5")
	numericPattern := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:\/|out\s+of)\s*(\d+)`)
	if matches := numericPattern.FindStringSubmatch(lowerText); len(matches) > 2 {
		return matches[1] + "/" + matches[2]
	}

	// Look for star ratings (e.g., "★★★★☆" or "4 stars")
	starPattern := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*stars?`)
	if matches := starPattern.FindStringSubmatch(lowerText); len(matches) > 1 {
		return matches[1] + " stars"
	}

	// Look for grade ratings (e.g., "Grade: A", "Rating: B+")
	gradePattern := regexp.MustCompile(`(?:grade|rating)[\s:]+([A-F][+-]?)`)
	if matches := gradePattern.FindStringSubmatch(lowerText); len(matches) > 1 {
		return strings.ToUpper(matches[1])
	}

	// Analyze sentiment for qualitative rating
	positiveIndicators := []string{
		"excellent", "outstanding", "exceptional", "perfect", "superb",
		"great", "very good", "high quality", "well done", "impressive",
	}
	negativeIndicators := []string{
		"poor", "bad", "terrible", "awful", "unacceptable",
		"needs improvement", "inadequate", "below standard", "unsatisfactory",
	}
	neutralIndicators := []string{
		"acceptable", "adequate", "satisfactory", "average", "okay", "ok",
		"decent", "fair", "moderate",
	}

	for _, indicator := range positiveIndicators {
		if strings.Contains(lowerText, indicator) {
			return "positive"
		}
	}
	for _, indicator := range negativeIndicators {
		if strings.Contains(lowerText, indicator) {
			return "needs_improvement"
		}
	}
	for _, indicator := range neutralIndicators {
		if strings.Contains(lowerText, indicator) {
			return "acceptable"
		}
	}

	return "reviewed"
}

// GetCapabilities returns reviewer capabilities
func (r *ReviewerWorker) GetCapabilities() []string {
	return []string{"code_review", "content_review", "quality_assurance", "testing"}
}

// GetName returns the handler name
func (r *ReviewerWorker) GetName() string {
	return "reviewer"
}
