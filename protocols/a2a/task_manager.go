package a2a

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskHandler processes tasks and returns results
type TaskHandler interface {
	// HandleTask processes a task and updates it with results
	HandleTask(ctx context.Context, task *Task) error

	// HandleTaskStream processes a task with streaming updates
	HandleTaskStream(ctx context.Context, task *Task, updates chan<- TaskUpdate) error

	// SupportsStreaming indicates if the handler supports streaming
	SupportsStreaming() bool
}

// TaskUpdate represents an update during task processing
type TaskUpdate struct {
	// Type of update
	Type TaskUpdateType

	// Status update (for status changes)
	Status *TaskStatus

	// Artifact update (for artifact production)
	Artifact *Artifact

	// Message update (for intermediate messages)
	Message *Message

	// Error if something went wrong
	Error error
}

// TaskUpdateType indicates the type of task update
type TaskUpdateType string

const (
	TaskUpdateTypeStatus   TaskUpdateType = "status"
	TaskUpdateTypeArtifact TaskUpdateType = "artifact"
	TaskUpdateTypeMessage  TaskUpdateType = "message"
	TaskUpdateTypeError    TaskUpdateType = "error"
)

// NewStatusUpdate creates a status update
func NewStatusUpdate(state TaskState, message *Message) TaskUpdate {
	return TaskUpdate{
		Type: TaskUpdateTypeStatus,
		Status: &TaskStatus{
			State:     state,
			Message:   message,
			Timestamp: time.Now(),
		},
	}
}

// NewArtifactUpdate creates an artifact update
func NewArtifactUpdate(artifact *Artifact) TaskUpdate {
	return TaskUpdate{
		Type:     TaskUpdateTypeArtifact,
		Artifact: artifact,
	}
}

// NewMessageUpdate creates a message update
func NewMessageUpdate(message *Message) TaskUpdate {
	return TaskUpdate{
		Type:    TaskUpdateTypeMessage,
		Message: message,
	}
}

// NewErrorUpdate creates an error update
func NewErrorUpdate(err error) TaskUpdate {
	return TaskUpdate{
		Type:  TaskUpdateTypeError,
		Error: err,
	}
}

// TaskManager manages A2A tasks
type TaskManager struct {
	mu       sync.RWMutex
	tasks    map[string]*Task
	sessions map[string][]string // sessionID -> taskIDs
	handler  TaskHandler
	config   TaskManagerConfig
}

// TaskManagerConfig configures the task manager
type TaskManagerConfig struct {
	// MaxTasksPerSession limits tasks per session (0 = unlimited)
	MaxTasksPerSession int

	// TaskTimeout is the default timeout for tasks
	TaskTimeout time.Duration

	// RetainCompletedTasks determines if completed tasks are kept
	RetainCompletedTasks bool

	// MaxRetainedTasks limits retained tasks (0 = unlimited)
	MaxRetainedTasks int

	// CleanupInterval is how often to clean up old tasks
	CleanupInterval time.Duration
}

// DefaultTaskManagerConfig returns default configuration
func DefaultTaskManagerConfig() TaskManagerConfig {
	return TaskManagerConfig{
		MaxTasksPerSession:   100,
		TaskTimeout:          5 * time.Minute,
		RetainCompletedTasks: true,
		MaxRetainedTasks:     1000,
		CleanupInterval:      10 * time.Minute,
	}
}

// NewTaskManager creates a new task manager
func NewTaskManager(handler TaskHandler, config TaskManagerConfig) *TaskManager {
	tm := &TaskManager{
		tasks:    make(map[string]*Task),
		sessions: make(map[string][]string),
		handler:  handler,
		config:   config,
	}

	// Start cleanup goroutine
	if config.CleanupInterval > 0 {
		go tm.cleanupLoop()
	}

	return tm
}

// cleanupLoop periodically cleans up old tasks
func (tm *TaskManager) cleanupLoop() {
	ticker := time.NewTicker(tm.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		tm.cleanup()
	}
}

// cleanup removes old completed tasks
func (tm *TaskManager) cleanup() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.config.MaxRetainedTasks <= 0 {
		return
	}

	// Count completed tasks
	var completedTasks []*Task
	for _, task := range tm.tasks {
		if task.Status.State == TaskStateCompleted || task.Status.State == TaskStateFailed || task.Status.State == TaskStateCanceled {
			completedTasks = append(completedTasks, task)
		}
	}

	// Remove excess completed tasks (oldest first)
	if len(completedTasks) > tm.config.MaxRetainedTasks {
		// Sort by timestamp (oldest first)
		for i := 0; i < len(completedTasks)-1; i++ {
			for j := i + 1; j < len(completedTasks); j++ {
				if completedTasks[i].Status.Timestamp.After(completedTasks[j].Status.Timestamp) {
					completedTasks[i], completedTasks[j] = completedTasks[j], completedTasks[i]
				}
			}
		}

		// Remove oldest
		toRemove := len(completedTasks) - tm.config.MaxRetainedTasks
		for i := 0; i < toRemove; i++ {
			delete(tm.tasks, completedTasks[i].ID)
			tm.removeFromSession(completedTasks[i].SessionID, completedTasks[i].ID)
		}
	}
}

// removeFromSession removes a task ID from a session
func (tm *TaskManager) removeFromSession(sessionID, taskID string) {
	if sessionID == "" {
		return
	}
	tasks := tm.sessions[sessionID]
	for i, id := range tasks {
		if id == taskID {
			tm.sessions[sessionID] = append(tasks[:i], tasks[i+1:]...)
			break
		}
	}
}

// CreateTask creates a new task
func (tm *TaskManager) CreateTask(params TaskSendParams) (*Task, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Generate ID if not provided
	taskID := params.ID
	if taskID == "" {
		taskID = uuid.New().String()
	}

	// Check if task already exists
	if _, exists := tm.tasks[taskID]; exists {
		return nil, fmt.Errorf("task %s already exists", taskID)
	}

	// Check session limits
	if params.SessionID != "" && tm.config.MaxTasksPerSession > 0 {
		if len(tm.sessions[params.SessionID]) >= tm.config.MaxTasksPerSession {
			return nil, fmt.Errorf("session %s has reached max tasks limit", params.SessionID)
		}
	}

	// Create task
	task := NewTask(taskID)
	task.SessionID = params.SessionID
	task.Metadata = params.Metadata
	task.AddMessage(params.Message)

	// Store task
	tm.tasks[taskID] = task

	// Track session
	if params.SessionID != "" {
		tm.sessions[params.SessionID] = append(tm.sessions[params.SessionID], taskID)
	}

	return task, nil
}

// GetTask retrieves a task by ID
func (tm *TaskManager) GetTask(taskID string) (*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	return task, nil
}

// GetTaskWithHistory retrieves a task with limited history
func (tm *TaskManager) GetTaskWithHistory(taskID string, historyLength int) (*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	// If history length is specified, limit the history
	if historyLength > 0 && len(task.History) > historyLength {
		limitedTask := *task
		start := len(task.History) - historyLength
		limitedTask.History = task.History[start:]
		return &limitedTask, nil
	}

	return task, nil
}

// UpdateTaskStatus updates a task's status
func (tm *TaskManager) UpdateTaskStatus(taskID string, state TaskState, message *Message) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.UpdateState(state, message)
	return nil
}

// AddTaskArtifact adds an artifact to a task
func (tm *TaskManager) AddTaskArtifact(taskID string, artifact Artifact) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.AddArtifact(artifact)
	return nil
}

// AddTaskMessage adds a message to a task's history
func (tm *TaskManager) AddTaskMessage(taskID string, message Message) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.AddMessage(message)
	return nil
}

// CancelTask cancels a task
func (tm *TaskManager) CancelTask(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	// Can only cancel tasks that aren't already completed
	if task.Status.State == TaskStateCompleted || task.Status.State == TaskStateFailed || task.Status.State == TaskStateCanceled {
		return fmt.Errorf("task %s is already in terminal state: %s", taskID, task.Status.State)
	}

	task.UpdateState(TaskStateCanceled, nil)
	return nil
}

// ProcessTask processes a task synchronously
func (tm *TaskManager) ProcessTask(ctx context.Context, taskID string) error {
	task, err := tm.GetTask(taskID)
	if err != nil {
		return err
	}

	// Update status to working
	tm.UpdateTaskStatus(taskID, TaskStateWorking, nil)

	// Process via handler
	if err := tm.handler.HandleTask(ctx, task); err != nil {
		tm.UpdateTaskStatus(taskID, TaskStateFailed, &Message{
			Role:  MessageRoleAgent,
			Parts: []Part{NewTextPart(err.Error())},
		})
		return err
	}

	// Update the stored task
	tm.mu.Lock()
	tm.tasks[taskID] = task
	tm.mu.Unlock()

	return nil
}

// ProcessTaskStream processes a task with streaming updates
func (tm *TaskManager) ProcessTaskStream(ctx context.Context, taskID string) (<-chan TaskUpdate, error) {
	task, err := tm.GetTask(taskID)
	if err != nil {
		return nil, err
	}

	if !tm.handler.SupportsStreaming() {
		return nil, fmt.Errorf("handler does not support streaming")
	}

	updates := make(chan TaskUpdate, 100)

	go func() {
		defer close(updates)

		// Update status to working
		tm.UpdateTaskStatus(taskID, TaskStateWorking, nil)
		updates <- NewStatusUpdate(TaskStateWorking, nil)

		// Create internal update channel
		internalUpdates := make(chan TaskUpdate, 100)

		// Process via handler
		go func() {
			err := tm.handler.HandleTaskStream(ctx, task, internalUpdates)
			if err != nil {
				internalUpdates <- NewErrorUpdate(err)
			}
			close(internalUpdates)
		}()

		// Forward updates and apply to task
		for update := range internalUpdates {
			switch update.Type {
			case TaskUpdateTypeStatus:
				tm.UpdateTaskStatus(taskID, update.Status.State, update.Status.Message)
			case TaskUpdateTypeArtifact:
				tm.AddTaskArtifact(taskID, *update.Artifact)
			case TaskUpdateTypeMessage:
				tm.AddTaskMessage(taskID, *update.Message)
			case TaskUpdateTypeError:
				tm.UpdateTaskStatus(taskID, TaskStateFailed, &Message{
					Role:  MessageRoleAgent,
					Parts: []Part{NewTextPart(update.Error.Error())},
				})
			}
			updates <- update
		}

		// Update stored task
		tm.mu.Lock()
		tm.tasks[taskID] = task
		tm.mu.Unlock()
	}()

	return updates, nil
}

// GetSessionTasks retrieves all tasks for a session
func (tm *TaskManager) GetSessionTasks(sessionID string) ([]*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	taskIDs, exists := tm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	tasks := make([]*Task, 0, len(taskIDs))
	for _, id := range taskIDs {
		if task, ok := tm.tasks[id]; ok {
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

// ListTasks returns all tasks (optionally filtered by state)
func (tm *TaskManager) ListTasks(state *TaskState) []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tasks := make([]*Task, 0, len(tm.tasks))
	for _, task := range tm.tasks {
		if state == nil || task.Status.State == *state {
			tasks = append(tasks, task)
		}
	}

	return tasks
}

// Stats returns task manager statistics
func (tm *TaskManager) Stats() TaskManagerStats {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := TaskManagerStats{
		TotalTasks:    len(tm.tasks),
		TotalSessions: len(tm.sessions),
		TasksByState:  make(map[TaskState]int),
	}

	for _, task := range tm.tasks {
		stats.TasksByState[task.Status.State]++
	}

	return stats
}

// TaskManagerStats contains task manager statistics
type TaskManagerStats struct {
	TotalTasks    int
	TotalSessions int
	TasksByState  map[TaskState]int
}
