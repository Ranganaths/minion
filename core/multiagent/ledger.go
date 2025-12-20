package multiagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskLedger manages high-level task planning and decomposition
// Similar to AutoGen's Magentic-One task ledger
type TaskLedger struct {
	mu          sync.RWMutex
	tasks       map[string]*Task
	taskHistory []string // Task IDs in order
	metadata    map[string]interface{}
}

// NewTaskLedger creates a new task ledger
func NewTaskLedger() *TaskLedger {
	return &TaskLedger{
		tasks:       make(map[string]*Task),
		taskHistory: make([]string, 0),
		metadata:    make(map[string]interface{}),
	}
}

// CreateTask creates a new task
func (tl *TaskLedger) CreateTask(ctx context.Context, task *Task) error {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	if task.ID == "" {
		task.ID = uuid.New().String()
	}

	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	task.UpdatedAt = time.Now()

	if task.Status == "" {
		task.Status = TaskStatusPending
	}

	tl.tasks[task.ID] = task
	tl.taskHistory = append(tl.taskHistory, task.ID)

	return nil
}

// GetTask retrieves a task by ID
func (tl *TaskLedger) GetTask(ctx context.Context, taskID string) (*Task, error) {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	task, exists := tl.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	return task, nil
}

// UpdateTask updates a task
func (tl *TaskLedger) UpdateTask(ctx context.Context, task *Task) error {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	if _, exists := tl.tasks[task.ID]; !exists {
		return fmt.Errorf("task %s not found", task.ID)
	}

	task.UpdatedAt = time.Now()
	tl.tasks[task.ID] = task

	return nil
}

// CompleteTask marks a task as completed
func (tl *TaskLedger) CompleteTask(ctx context.Context, taskID string, output interface{}) error {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	task, exists := tl.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Status = TaskStatusCompleted
	task.Output = output
	now := time.Now()
	task.CompletedAt = &now
	task.UpdatedAt = now

	return nil
}

// FailTask marks a task as failed
func (tl *TaskLedger) FailTask(ctx context.Context, taskID string, err error) error {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	task, exists := tl.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Status = TaskStatusFailed
	task.Error = err.Error()
	task.UpdatedAt = time.Now()

	return nil
}

// GetPendingTasks returns all pending tasks
func (tl *TaskLedger) GetPendingTasks(ctx context.Context) ([]*Task, error) {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	var pending []*Task
	for _, task := range tl.tasks {
		if task.Status == TaskStatusPending {
			pending = append(pending, task)
		}
	}

	return pending, nil
}

// GetTasksByStatus returns tasks with a specific status
func (tl *TaskLedger) GetTasksByStatus(ctx context.Context, status TaskStatus) ([]*Task, error) {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	var filtered []*Task
	for _, task := range tl.tasks {
		if task.Status == status {
			filtered = append(filtered, task)
		}
	}

	return filtered, nil
}

// GetHistory returns task history
func (tl *TaskLedger) GetHistory() []string {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	history := make([]string, len(tl.taskHistory))
	copy(history, tl.taskHistory)

	return history
}

// DeleteTask deletes a task
func (tl *TaskLedger) DeleteTask(ctx context.Context, taskID string) error {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	if _, exists := tl.tasks[taskID]; !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	delete(tl.tasks, taskID)

	// Remove from history
	newHistory := make([]string, 0, len(tl.taskHistory))
	for _, id := range tl.taskHistory {
		if id != taskID {
			newHistory = append(newHistory, id)
		}
	}
	tl.taskHistory = newHistory

	return nil
}

// ListTasks returns all tasks
func (tl *TaskLedger) ListTasks(ctx context.Context) []*Task {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	tasks := make([]*Task, 0, len(tl.tasks))
	for _, task := range tl.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// ProgressEntry represents a single step in execution progress
type ProgressEntry struct {
	ID          string                 `json:"id"`
	TaskID      string                 `json:"task_id"`
	AgentID     string                 `json:"agent_id"`
	Step        int                    `json:"step"`
	Action      string                 `json:"action"`
	Description string                 `json:"description"`
	Input       interface{}            `json:"input,omitempty"`
	Output      interface{}            `json:"output,omitempty"`
	Status      string                 `json:"status"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// ProgressLedger tracks step-by-step execution progress
// Similar to AutoGen's Magentic-One progress ledger
type ProgressLedger struct {
	mu      sync.RWMutex
	entries map[string][]*ProgressEntry // taskID -> entries
	current map[string]int               // taskID -> current step
}

// NewProgressLedger creates a new progress ledger
func NewProgressLedger() *ProgressLedger {
	return &ProgressLedger{
		entries: make(map[string][]*ProgressEntry),
		current: make(map[string]int),
	}
}

// AddEntry adds a progress entry
func (pl *ProgressLedger) AddEntry(ctx context.Context, entry *ProgressEntry) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	// Auto-increment step if not provided
	if entry.Step == 0 {
		entry.Step = pl.current[entry.TaskID] + 1
	}

	pl.entries[entry.TaskID] = append(pl.entries[entry.TaskID], entry)
	pl.current[entry.TaskID] = entry.Step

	return nil
}

// GetProgress returns all progress entries for a task
func (pl *ProgressLedger) GetProgress(ctx context.Context, taskID string) ([]*ProgressEntry, error) {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	entries, exists := pl.entries[taskID]
	if !exists {
		return []*ProgressEntry{}, nil
	}

	// Return a copy
	result := make([]*ProgressEntry, len(entries))
	copy(result, entries)

	return result, nil
}

// GetCurrentStep returns the current step for a task
func (pl *ProgressLedger) GetCurrentStep(ctx context.Context, taskID string) int {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	return pl.current[taskID]
}

// GetLatestEntry returns the latest progress entry for a task
func (pl *ProgressLedger) GetLatestEntry(ctx context.Context, taskID string) (*ProgressEntry, error) {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	entries, exists := pl.entries[taskID]
	if !exists || len(entries) == 0 {
		return nil, fmt.Errorf("no progress entries found for task %s", taskID)
	}

	return entries[len(entries)-1], nil
}

// RecordProgress records a progress update (convenience method matching LedgerBackend interface)
func (pl *ProgressLedger) RecordProgress(ctx context.Context, taskID, agentID string, status TaskStatus, message string) error {
	entry := &ProgressEntry{
		TaskID:      taskID,
		AgentID:     agentID,
		Status:      string(status),
		Description: message,
	}
	return pl.AddEntry(ctx, entry)
}

// Clear clears all progress for a task
func (pl *ProgressLedger) Clear(ctx context.Context, taskID string) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	delete(pl.entries, taskID)
	delete(pl.current, taskID)

	return nil
}
