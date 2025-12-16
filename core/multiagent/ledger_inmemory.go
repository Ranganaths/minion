package multiagent

import (
	"context"
	"time"
)

// InMemoryLedger implements LedgerBackend using in-memory storage
// This is a wrapper around the existing TaskLedger and ProgressLedger
type InMemoryLedger struct {
	taskLedger     *TaskLedger
	progressLedger *ProgressLedger
}

// NewInMemoryLedger creates a new in-memory ledger backend
func NewInMemoryLedger() *InMemoryLedger {
	return &InMemoryLedger{
		taskLedger:     NewTaskLedger(),
		progressLedger: NewProgressLedger(),
	}
}

// CreateTask creates a new task
func (iml *InMemoryLedger) CreateTask(ctx context.Context, task *Task) error {
	return iml.taskLedger.CreateTask(ctx, task)
}

// GetTask retrieves a task by ID
func (iml *InMemoryLedger) GetTask(ctx context.Context, taskID string) (*Task, error) {
	return iml.taskLedger.GetTask(ctx, taskID)
}

// UpdateTask updates a task
func (iml *InMemoryLedger) UpdateTask(ctx context.Context, task *Task) error {
	return iml.taskLedger.UpdateTask(ctx, task)
}

// CompleteTask marks a task as completed
func (iml *InMemoryLedger) CompleteTask(ctx context.Context, taskID string, result interface{}) error {
	return iml.taskLedger.CompleteTask(ctx, taskID, result)
}

// FailTask marks a task as failed
func (iml *InMemoryLedger) FailTask(ctx context.Context, taskID string, errMsg string) error {
	return iml.taskLedger.FailTask(ctx, taskID, errMsg)
}

// DeleteTask deletes a task
func (iml *InMemoryLedger) DeleteTask(ctx context.Context, taskID string) error {
	return iml.taskLedger.DeleteTask(ctx, taskID)
}

// ListTasks lists tasks matching the filter
func (iml *InMemoryLedger) ListTasks(ctx context.Context, filter *TaskFilter) ([]*Task, error) {
	tasks := iml.taskLedger.ListTasks(ctx)

	// Apply filters
	if filter == nil {
		return tasks, nil
	}

	var filtered []*Task
	for _, task := range tasks {
		// Status filter
		if len(filter.Statuses) > 0 {
			statusMatch := false
			for _, status := range filter.Statuses {
				if task.Status == status {
					statusMatch = true
					break
				}
			}
			if !statusMatch {
				continue
			}
		}

		// Assignment filter
		if filter.AssignedTo != "" && task.AssignedTo != filter.AssignedTo {
			continue
		}

		// Creator filter
		if filter.CreatedBy != "" && task.CreatedBy != filter.CreatedBy {
			continue
		}

		// Time filters
		if filter.CreatedAfter != nil && task.CreatedAt.Before(*filter.CreatedAfter) {
			continue
		}

		if filter.CreatedBefore != nil && task.CreatedAt.After(*filter.CreatedBefore) {
			continue
		}

		if filter.UpdatedAfter != nil && task.UpdatedAt.Before(*filter.UpdatedAfter) {
			continue
		}

		if filter.UpdatedBefore != nil && task.UpdatedAt.After(*filter.UpdatedBefore) {
			continue
		}

		filtered = append(filtered, task)
	}

	// Apply pagination
	if filter.Limit > 0 {
		offset := filter.Offset
		if offset < 0 {
			offset = 0
		}

		if offset >= len(filtered) {
			return []*Task{}, nil
		}

		end := offset + filter.Limit
		if end > len(filtered) {
			end = len(filtered)
		}

		filtered = filtered[offset:end]
	}

	return filtered, nil
}

// RecordProgress records a progress update
func (iml *InMemoryLedger) RecordProgress(ctx context.Context, progress *ProgressUpdate) error {
	return iml.progressLedger.RecordProgress(ctx, progress.TaskID, progress.AgentID, progress.Status, progress.Message)
}

// GetProgress retrieves all progress updates for a task
func (iml *InMemoryLedger) GetProgress(ctx context.Context, taskID string) ([]*ProgressUpdate, error) {
	updates := iml.progressLedger.GetProgress(ctx, taskID)

	// Convert to ProgressUpdate format
	var result []*ProgressUpdate
	for _, update := range updates {
		result = append(result, &ProgressUpdate{
			TaskID:     update.TaskID,
			AgentID:    update.AgentID,
			Status:     update.Status,
			Message:    update.Message,
			RecordedAt: update.Timestamp,
		})
	}

	return result, nil
}

// GetLatestProgress retrieves the latest progress update for a task
func (iml *InMemoryLedger) GetLatestProgress(ctx context.Context, taskID string) (*ProgressUpdate, error) {
	updates := iml.progressLedger.GetProgress(ctx, taskID)

	if len(updates) == 0 {
		return nil, nil
	}

	// Get the latest update
	latest := updates[len(updates)-1]

	return &ProgressUpdate{
		TaskID:     latest.TaskID,
		AgentID:    latest.AgentID,
		Status:     latest.Status,
		Message:    latest.Message,
		RecordedAt: latest.Timestamp,
	}, nil
}

// PurgeCompletedTasks deletes completed tasks older than the specified duration
func (iml *InMemoryLedger) PurgeCompletedTasks(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	count := int64(0)

	tasks := iml.taskLedger.ListTasks(ctx)
	for _, task := range tasks {
		// Check if task is completed and older than cutoff
		if (task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed || task.Status == TaskStatusCancelled) &&
			task.CompletedAt != nil && task.CompletedAt.Before(cutoff) {

			if err := iml.taskLedger.DeleteTask(ctx, task.ID); err == nil {
				count++
			}
		}
	}

	return count, nil
}

// PurgeProgress deletes progress updates older than the specified duration
func (iml *InMemoryLedger) PurgeProgress(ctx context.Context, olderThan time.Duration) (int64, error) {
	// In-memory progress ledger doesn't support purging yet
	// This would require extending the ProgressLedger implementation
	return 0, nil
}

// Health checks ledger health (always healthy for in-memory)
func (iml *InMemoryLedger) Health(ctx context.Context) error {
	return nil
}

// Stats returns ledger statistics
func (iml *InMemoryLedger) Stats(ctx context.Context) (*LedgerStats, error) {
	stats := &LedgerStats{}

	tasks := iml.taskLedger.ListTasks(ctx)
	stats.TotalTasks = int64(len(tasks))

	var oldestTime, newestTime time.Time

	for _, task := range tasks {
		// Count by status
		switch task.Status {
		case TaskStatusPending:
			stats.PendingTasks++
		case TaskStatusInProgress:
			stats.InProgressTasks++
		case TaskStatusCompleted:
			stats.CompletedTasks++
		case TaskStatusFailed:
			stats.FailedTasks++
		}

		// Track oldest/newest
		if oldestTime.IsZero() || task.CreatedAt.Before(oldestTime) {
			oldestTime = task.CreatedAt
		}
		if newestTime.IsZero() || task.CreatedAt.After(newestTime) {
			newestTime = task.CreatedAt
		}
	}

	stats.OldestTask = oldestTime
	stats.NewestTask = newestTime

	// Count progress updates
	for _, task := range tasks {
		updates := iml.progressLedger.GetProgress(ctx, task.ID)
		stats.TotalProgress += int64(len(updates))
	}

	return stats, nil
}

// Close closes the ledger (no-op for in-memory)
func (iml *InMemoryLedger) Close() error {
	return nil
}
