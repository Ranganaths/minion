package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// OrchestratorConfig configures the orchestrator behavior
type OrchestratorConfig struct {
	MaxRetries         int           `json:"max_retries"`
	RetryDelay         time.Duration `json:"retry_delay"`
	MaxConcurrentTasks int           `json:"max_concurrent_tasks"`
	TaskTimeout        time.Duration `json:"task_timeout"`
	EnableReplanning   bool          `json:"enable_replanning"` // Re-plan on errors
}

// DefaultOrchestratorConfig returns default configuration
func DefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		MaxRetries:         3,
		RetryDelay:         time.Second * 2,
		MaxConcurrentTasks: 5,
		TaskTimeout:        time.Minute * 5,
		EnableReplanning:   true,
	}
}

// Orchestrator coordinates multiple agents to accomplish complex tasks
// Based on AutoGen's Magentic-One orchestrator pattern
type Orchestrator struct {
	mu             sync.RWMutex
	id             string
	protocol       Protocol
	taskLedger     *TaskLedger
	progressLedger *ProgressLedger
	workers        map[string]*AgentMetadata // agentID -> metadata
	config         *OrchestratorConfig
	llmProvider    LLMProvider // For planning and decision making
}

// LLMProvider defines interface for LLM operations
type LLMProvider interface {
	GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
}

// CompletionRequest represents an LLM completion request
type CompletionRequest struct {
	SystemPrompt string
	UserPrompt   string
	Temperature  float64
	MaxTokens    int
	Model        string
}

// CompletionResponse represents an LLM completion response
type CompletionResponse struct {
	Text         string
	TokensUsed   int
	Model        string
	FinishReason string
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(
	protocol Protocol,
	llmProvider LLMProvider,
	config *OrchestratorConfig,
) *Orchestrator {
	if config == nil {
		config = DefaultOrchestratorConfig()
	}

	orchestratorID := uuid.New().String()

	// Subscribe to receive result and error messages from workers
	protocol.Subscribe(context.Background(), orchestratorID, []MessageType{
		MessageTypeResult,
		MessageTypeError,
		MessageTypeInform,
	})

	return &Orchestrator{
		id:             orchestratorID,
		protocol:       protocol,
		taskLedger:     NewTaskLedger(),
		progressLedger: NewProgressLedger(),
		workers:        make(map[string]*AgentMetadata),
		config:         config,
		llmProvider:    llmProvider,
	}
}

// RegisterWorker registers a worker agent
func (o *Orchestrator) RegisterWorker(metadata *AgentMetadata) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if metadata.AgentID == "" {
		return fmt.Errorf("agent ID is required")
	}

	o.workers[metadata.AgentID] = metadata

	// Subscribe worker to relevant message types
	return o.protocol.Subscribe(context.Background(), metadata.AgentID, []MessageType{
		MessageTypeTask,
		MessageTypeDelegate,
	})
}

// UnregisterWorker removes a worker agent
func (o *Orchestrator) UnregisterWorker(agentID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	delete(o.workers, agentID)

	return o.protocol.Unsubscribe(context.Background(), agentID)
}

// GetWorkers returns all registered workers
func (o *Orchestrator) GetWorkers() []*AgentMetadata {
	o.mu.RLock()
	defer o.mu.RUnlock()

	workers := make([]*AgentMetadata, 0, len(o.workers))
	for _, w := range o.workers {
		workers = append(workers, w)
	}

	return workers
}

// ExecuteTask executes a complex task using multiple agents
func (o *Orchestrator) ExecuteTask(ctx context.Context, taskReq *TaskRequest) (*TaskResult, error) {
	// 1. Create main task
	task := &Task{
		ID:          uuid.New().String(),
		Name:        taskReq.Name,
		Description: taskReq.Description,
		Type:        taskReq.Type,
		Priority:    taskReq.Priority,
		CreatedBy:   o.id,
		Input:       taskReq.Input,
		Status:      TaskStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := o.taskLedger.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// 2. Plan task decomposition using LLM
	subtasks, err := o.planTask(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("task planning failed: %w", err)
	}

	// 3. Execute subtasks
	result, err := o.executeSubtasks(ctx, task, subtasks)
	if err != nil {
		// If replanning is enabled, try to recover
		if o.config.EnableReplanning {
			return o.replanAndExecute(ctx, task, err)
		}
		return nil, err
	}

	return result, nil
}

// planTask uses LLM to decompose a task into subtasks
func (o *Orchestrator) planTask(ctx context.Context, task *Task) ([]*Task, error) {
	// Log planning step
	o.progressLedger.AddEntry(ctx, &ProgressEntry{
		TaskID:      task.ID,
		AgentID:     o.id,
		Action:      "planning",
		Description: "Decomposing task into subtasks",
		Status:      "in_progress",
	})

	// Get available workers
	workers := o.GetWorkers()
	workerCapabilities := o.buildWorkerCapabilitiesPrompt(workers)

	// Create planning prompt with strict JSON formatting requirements
	systemPrompt := `You are an orchestrator agent that decomposes complex tasks into smaller subtasks.
Your role is to analyze the given task and break it down into executable steps.

Available workers and their capabilities:
` + workerCapabilities + `

You MUST respond with ONLY valid JSON in this exact format (no additional text):

{
  "subtasks": [
    {
      "name": "Short task name",
      "description": "Detailed description of what needs to be done",
      "assigned_to": "worker_capability_name",
      "dependencies": ["name_of_task_1", "name_of_task_2"],
      "priority": 5,
      "input": "Specific input for this subtask"
    }
  ]
}

Rules:
1. assigned_to must match a worker capability exactly (e.g., "code_generation", "data_analysis", "research")
2. dependencies should reference other subtask names (use empty array [] if no dependencies)
3. priority is 1-10 (1=lowest, 10=highest)
4. Ensure dependencies form a valid DAG (no cycles)
5. Order subtasks logically

Output ONLY the JSON object, nothing else.`

	userPrompt := fmt.Sprintf(`Task: %s
Description: %s
Input: %v

Please decompose this task into subtasks.`, task.Name, task.Description, task.Input)

	// Call LLM for planning
	resp, err := o.llmProvider.GenerateCompletion(ctx, &CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.3, // Lower temperature for more deterministic planning
		MaxTokens:    2000,
		Model:        "gpt-4",
	})
	if err != nil {
		return nil, fmt.Errorf("LLM planning failed: %w", err)
	}

	// Parse LLM response into subtasks
	subtasks, err := o.parseSubtasks(resp.Text, task.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subtasks: %w", err)
	}

	// Log successful planning
	o.progressLedger.AddEntry(ctx, &ProgressEntry{
		TaskID:      task.ID,
		AgentID:     o.id,
		Action:      "planning",
		Description: fmt.Sprintf("Created %d subtasks", len(subtasks)),
		Output:      subtasks,
		Status:      "completed",
	})

	return subtasks, nil
}

// executeSubtasks executes a list of subtasks
func (o *Orchestrator) executeSubtasks(ctx context.Context, mainTask *Task, subtasks []*Task) (*TaskResult, error) {
	// Track completion
	completed := make(map[string]bool)
	results := make(map[string]interface{})
	var lastError error

	for _, subtask := range subtasks {
		// Check dependencies
		if !o.checkDependencies(subtask, completed) {
			continue // Skip for now, will retry
		}

		// Assign to worker
		if err := o.assignTaskToWorker(ctx, subtask); err != nil {
			lastError = err
			continue
		}

		// Wait for completion (with timeout)
		result, err := o.waitForTaskCompletion(ctx, subtask)
		if err != nil {
			lastError = err
			if o.config.MaxRetries > 0 {
				// Retry logic
				result, err = o.retryTask(ctx, subtask)
				if err != nil {
					continue
				}
			} else {
				continue
			}
		}

		completed[subtask.ID] = true
		results[subtask.ID] = result
	}

	// Check if all completed
	if len(completed) != len(subtasks) {
		return nil, fmt.Errorf("failed to complete all subtasks: %w", lastError)
	}

	// Aggregate results
	return &TaskResult{
		TaskID:      mainTask.ID,
		Status:      "completed",
		Output:      results,
		CompletedAt: time.Now(),
	}, nil
}

// assignTaskToWorker assigns a task to the most suitable worker
func (o *Orchestrator) assignTaskToWorker(ctx context.Context, task *Task) error {
	// Find suitable worker
	worker, err := o.findWorkerForTask(task)
	if err != nil {
		return err
	}

	// Update task status
	task.Status = TaskStatusAssigned
	task.AssignedTo = worker.AgentID
	o.taskLedger.UpdateTask(ctx, task)

	// Send task message to worker
	msg := &Message{
		ID:        uuid.New().String(),
		Type:      MessageTypeTask,
		From:      o.id,
		To:        worker.AgentID,
		Content:   task,
		CreatedAt: time.Now(),
	}

	return o.protocol.Send(ctx, msg)
}

// findWorkerForTask finds the best worker for a task
func (o *Orchestrator) findWorkerForTask(task *Task) (*AgentMetadata, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// If already assigned, return that worker
	if task.AssignedTo != "" {
		worker, exists := o.workers[task.AssignedTo]
		if exists {
			return worker, nil
		}
	}

	// Find worker with matching capabilities
	var bestWorker *AgentMetadata
	highestScore := 0

	for _, worker := range o.workers {
		if worker.Status == StatusOffline || worker.Status == StatusFailed {
			continue
		}

		// Simple scoring based on capability match
		score := o.scoreWorker(worker, task)
		if score > highestScore {
			highestScore = score
			bestWorker = worker
		}
	}

	if bestWorker == nil {
		return nil, fmt.Errorf("no suitable worker found for task %s", task.ID)
	}

	return bestWorker, nil
}

// scoreWorker scores how well a worker matches a task
func (o *Orchestrator) scoreWorker(worker *AgentMetadata, task *Task) int {
	score := 0

	// Check if worker has required capabilities
	// This is a simple implementation; can be enhanced
	if worker.Role == RoleSpecialist {
		score += 10
	}

	if worker.Status == StatusIdle {
		score += 5
	}

	// Priority bonus
	score += worker.Priority

	return score
}

// buildWorkerCapabilitiesPrompt builds a prompt describing worker capabilities
func (o *Orchestrator) buildWorkerCapabilitiesPrompt(workers []*AgentMetadata) string {
	prompt := ""
	for _, w := range workers {
		prompt += fmt.Sprintf("- Agent %s (Role: %s): Capabilities: %v\n",
			w.AgentID, w.Role, w.Capabilities)
	}
	return prompt
}

// checkDependencies checks if all dependencies are completed
func (o *Orchestrator) checkDependencies(task *Task, completed map[string]bool) bool {
	for _, depID := range task.Dependencies {
		if !completed[depID] {
			return false
		}
	}
	return true
}

// waitForTaskCompletion waits for a task to complete
func (o *Orchestrator) waitForTaskCompletion(ctx context.Context, task *Task) (interface{}, error) {
	timeout := time.After(o.config.TaskTimeout)
	ticker := time.NewTicker(100 * time.Millisecond) // Check more frequently
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("task %s timed out after %v", task.ID, o.config.TaskTimeout)
		case <-ticker.C:
			// Check for result messages from workers
			messages, err := o.protocol.Receive(ctx, o.id)
			if err == nil {
				for _, msg := range messages {
					if msg.Type == MessageTypeResult && msg.InReplyTo == task.ID {
						// Worker completed the task
						o.taskLedger.CompleteTask(ctx, task.ID, msg.Content)
						return msg.Content, nil
					}
					if msg.Type == MessageTypeError && msg.InReplyTo == task.ID {
						// Worker failed the task
						errMsg := fmt.Sprintf("%v", msg.Content)
						o.taskLedger.FailTask(ctx, task.ID, fmt.Errorf("%s", errMsg))
						return nil, fmt.Errorf("task failed: %s", errMsg)
					}
				}
			}

			// Also check task ledger status (in case updated directly)
			currentTask, err := o.taskLedger.GetTask(ctx, task.ID)
			if err != nil {
				return nil, err
			}

			if currentTask.Status == TaskStatusCompleted {
				return currentTask.Output, nil
			}

			if currentTask.Status == TaskStatusFailed {
				return nil, fmt.Errorf("task failed: %s", currentTask.Error)
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// retryTask retries a failed task
func (o *Orchestrator) retryTask(ctx context.Context, task *Task) (interface{}, error) {
	for i := 0; i < o.config.MaxRetries; i++ {
		time.Sleep(o.config.RetryDelay)

		task.Status = TaskStatusPending
		o.taskLedger.UpdateTask(ctx, task)

		if err := o.assignTaskToWorker(ctx, task); err != nil {
			continue
		}

		result, err := o.waitForTaskCompletion(ctx, task)
		if err == nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("task failed after %d retries", o.config.MaxRetries)
}

// replanAndExecute replans and re-executes a task after failure
func (o *Orchestrator) replanAndExecute(ctx context.Context, task *Task, prevError error) (*TaskResult, error) {
	// Log replanning
	o.progressLedger.AddEntry(ctx, &ProgressEntry{
		TaskID:      task.ID,
		AgentID:     o.id,
		Action:      "replanning",
		Description: fmt.Sprintf("Replanning due to error: %v", prevError),
		Status:      "in_progress",
	})

	// Re-plan with error context
	subtasks, err := o.planTask(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("replanning failed: %w", err)
	}

	// Execute new plan
	return o.executeSubtasks(ctx, task, subtasks)
}

// SubtaskResponse represents the expected JSON structure from LLM
type SubtaskResponse struct {
	Subtasks []SubtaskSpec `json:"subtasks"`
}

// SubtaskSpec represents a single subtask specification from LLM
type SubtaskSpec struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	AssignedTo   string   `json:"assigned_to"`   // Worker capability/role
	Dependencies []string `json:"dependencies"`  // Names of dependent subtasks
	Priority     int      `json:"priority"`      // 1-10
	Input        interface{} `json:"input"`
}

// parseSubtasks parses LLM output into subtasks
func (o *Orchestrator) parseSubtasks(llmOutput string, parentTaskID string) ([]*Task, error) {
	// Try to extract JSON from the LLM output
	// LLM might include explanatory text, so we need to extract the JSON block
	jsonStr := o.extractJSON(llmOutput)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in LLM output")
	}

	// Parse JSON response
	var response SubtaskResponse
	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		return nil, fmt.Errorf("failed to parse LLM JSON response: %w", err)
	}

	if len(response.Subtasks) == 0 {
		return nil, fmt.Errorf("no subtasks found in LLM response")
	}

	// Convert SubtaskSpecs to Tasks
	subtasks := make([]*Task, len(response.Subtasks))
	subtaskIDMap := make(map[string]string) // name -> ID mapping for dependencies

	for i, spec := range response.Subtasks {
		taskID := uuid.New().String()

		// Map priority (1-10 to our TaskPriority)
		var priority TaskPriority
		switch {
		case spec.Priority >= 9:
			priority = PriorityCritical
		case spec.Priority >= 7:
			priority = PriorityHigh
		case spec.Priority >= 4:
			priority = PriorityNormal
		default:
			priority = PriorityLow
		}

		subtasks[i] = &Task{
			ID:          taskID,
			Name:        spec.Name,
			Description: spec.Description,
			Type:        "subtask",
			Priority:    priority,
			CreatedBy:   o.id,
			Input:       spec.Input,
			Status:      TaskStatusPending,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Metadata: map[string]interface{}{
				"parent_task_id":     parentTaskID,
				"assigned_to_spec":   spec.AssignedTo,
				"dependency_names":   spec.Dependencies,
			},
		}

		subtaskIDMap[spec.Name] = taskID
	}

	// Resolve dependencies (convert names to IDs)
	for _, subtask := range subtasks {
		if depNames, ok := subtask.Metadata["dependency_names"].([]string); ok && len(depNames) > 0 {
			depIDs := make([]string, 0, len(depNames))
			for _, depName := range depNames {
				if depID, exists := subtaskIDMap[depName]; exists {
					depIDs = append(depIDs, depID)
				}
			}
			subtask.Dependencies = depIDs
		}
	}

	return subtasks, nil
}

// extractJSON extracts JSON object or array from text that might contain other content
func (o *Orchestrator) extractJSON(text string) string {
	// Try to find JSON object
	startObj := strings.Index(text, "{")
	if startObj != -1 {
		// Find matching closing brace
		depth := 0
		for i := startObj; i < len(text); i++ {
			switch text[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return text[startObj : i+1]
				}
			}
		}
	}

	// Try to find JSON array
	startArr := strings.Index(text, "[")
	if startArr != -1 {
		depth := 0
		for i := startArr; i < len(text); i++ {
			switch text[i] {
			case '[':
				depth++
			case ']':
				depth--
				if depth == 0 {
					return text[startArr : i+1]
				}
			}
		}
	}

	return ""
}

// TaskRequest represents a request to execute a task
type TaskRequest struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Type        string       `json:"type"`
	Priority    TaskPriority `json:"priority"`
	Input       interface{}  `json:"input"`
}

// TaskResult represents the result of a task execution
type TaskResult struct {
	TaskID      string      `json:"task_id"`
	Status      string      `json:"status"`
	Output      interface{} `json:"output"`
	Error       string      `json:"error,omitempty"`
	CompletedAt time.Time   `json:"completed_at"`
}
