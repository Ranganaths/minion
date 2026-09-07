package storage

import (
	"context"
	"testing"
	"time"

	"github.com/Ranganaths/minion/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryStore_Agent_CRUD(t *testing.T) {
	store := NewInMemory()
	ctx := context.Background()

	agent := &models.Agent{
		ID:           "test-agent-1",
		Name:         "Test Agent",
		Description:  "A test agent",
		BehaviorType: "analytical",
		Status:       models.StatusActive,
		Config: models.AgentConfig{
			LLMProvider: "openai",
			LLMModel:    "gpt-4",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	t.Run("Create", func(t *testing.T) {
		err := store.Create(ctx, agent)
		require.NoError(t, err)
	})

	t.Run("Get", func(t *testing.T) {
		result, err := store.Get(ctx, "test-agent-1")
		require.NoError(t, err)
		assert.Equal(t, agent.ID, result.ID)
		assert.Equal(t, agent.Name, result.Name)
	})

	t.Run("Get_NotFound", func(t *testing.T) {
		_, err := store.Get(ctx, "non-existent")
		assert.ErrorIs(t, err, ErrAgentNotFound)
	})

	t.Run("Update", func(t *testing.T) {
		agent.Name = "Updated Agent"
		err := store.Update(ctx, agent)
		require.NoError(t, err)

		result, err := store.Get(ctx, "test-agent-1")
		require.NoError(t, err)
		assert.Equal(t, "Updated Agent", result.Name)
	})

	t.Run("Update_NotFound", func(t *testing.T) {
		err := store.Update(ctx, &models.Agent{ID: "non-existent"})
		assert.ErrorIs(t, err, ErrAgentNotFound)
	})

	t.Run("Delete", func(t *testing.T) {
		err := store.Delete(ctx, "test-agent-1")
		require.NoError(t, err)

		_, err = store.Get(ctx, "test-agent-1")
		assert.ErrorIs(t, err, ErrAgentNotFound)
	})

	t.Run("Delete_NotFound", func(t *testing.T) {
		err := store.Delete(ctx, "non-existent")
		assert.ErrorIs(t, err, ErrAgentNotFound)
	})
}

func TestInMemoryStore_Create_Duplicate(t *testing.T) {
	store := NewInMemory()
	ctx := context.Background()

	agent := &models.Agent{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	err := store.Create(ctx, agent)
	require.NoError(t, err)

	err = store.Create(ctx, agent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestInMemoryStore_List(t *testing.T) {
	store := NewInMemory()
	ctx := context.Background()

	statusActive := models.StatusActive
	statusInactive := models.StatusInactive

	agents := []*models.Agent{
		{ID: "agent-1", Name: "Alpha", BehaviorType: "analytical", Status: statusActive},
		{ID: "agent-2", Name: "Beta", BehaviorType: "conversational", Status: statusActive},
		{ID: "agent-3", Name: "Gamma", BehaviorType: "analytical", Status: statusInactive},
	}

	for _, a := range agents {
		_ = store.Create(ctx, a)
	}

	t.Run("List_All", func(t *testing.T) {
		results, total, err := store.List(ctx, &models.ListAgentsRequest{})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, results, 3)
	})

	t.Run("List_ByBehaviorType", func(t *testing.T) {
		results, total, err := store.List(ctx, &models.ListAgentsRequest{
			BehaviorType: strPtr("analytical"),
		})
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, results, 2)
	})

	t.Run("List_ByStatus", func(t *testing.T) {
		results, total, err := store.List(ctx, &models.ListAgentsRequest{
			Status: &statusActive,
		})
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, results, 2)
	})

	t.Run("List_BySearch", func(t *testing.T) {
		results, total, err := store.List(ctx, &models.ListAgentsRequest{
			Search: "alpha",
		})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, results, 1)
		assert.Equal(t, "Alpha", results[0].Name)
	})

	t.Run("List_Pagination", func(t *testing.T) {
		results, total, err := store.List(ctx, &models.ListAgentsRequest{
			Page:     1,
			PageSize: 2,
		})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, results, 2)
	})

	t.Run("List_Pagination_Page2", func(t *testing.T) {
		results, total, err := store.List(ctx, &models.ListAgentsRequest{
			Page:     2,
			PageSize: 2,
		})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, results, 1)
	})
}

func TestInMemoryStore_FindByBehaviorType(t *testing.T) {
	store := NewInMemory()
	ctx := context.Background()

	_ = store.Create(ctx, &models.Agent{ID: "a1", BehaviorType: "analytical"})
	_ = store.Create(ctx, &models.Agent{ID: "a2", BehaviorType: "conversational"})
	_ = store.Create(ctx, &models.Agent{ID: "a3", BehaviorType: "analytical"})

	results, err := store.FindByBehaviorType(ctx, "analytical")
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestInMemoryStore_FindByStatus(t *testing.T) {
	store := NewInMemory()
	ctx := context.Background()

	_ = store.Create(ctx, &models.Agent{ID: "a1", Status: models.StatusActive})
	_ = store.Create(ctx, &models.Agent{ID: "a2", Status: models.StatusInactive})
	_ = store.Create(ctx, &models.Agent{ID: "a3", Status: models.StatusActive})

	results, err := store.FindByStatus(ctx, models.StatusActive)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestInMemoryStore_Metrics(t *testing.T) {
	store := NewInMemory()
	ctx := context.Background()

	now := time.Now()
	metrics := &models.Metrics{
		AgentID:             "agent-1",
		TotalExecutions:     10,
		SuccessfulExecutions: 8,
		FailedExecutions:    2,
		AvgExecutionTime:    150.5,
		LastExecutionAt:     &now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	t.Run("CreateMetrics", func(t *testing.T) {
		err := store.CreateMetrics(ctx, metrics)
		require.NoError(t, err)
	})

	t.Run("GetMetrics", func(t *testing.T) {
		result, err := store.GetMetrics(ctx, "agent-1")
		require.NoError(t, err)
		assert.Equal(t, metrics.TotalExecutions, result.TotalExecutions)
	})

	t.Run("GetMetrics_NotFound", func(t *testing.T) {
		_, err := store.GetMetrics(ctx, "non-existent")
		assert.ErrorIs(t, err, ErrMetricsNotFound)
	})

	t.Run("UpdateMetrics", func(t *testing.T) {
		metrics.TotalExecutions = 20
		err := store.UpdateMetrics(ctx, metrics)
		require.NoError(t, err)

		result, err := store.GetMetrics(ctx, "agent-1")
		require.NoError(t, err)
		assert.Equal(t, 20, result.TotalExecutions)
	})

	t.Run("CreateMetrics_Duplicate", func(t *testing.T) {
		err := store.CreateMetrics(ctx, metrics)
		assert.Error(t, err)
	})
}

func TestInMemoryStore_Activity(t *testing.T) {
	store := NewInMemory()
	ctx := context.Background()

	activity := &models.Activity{
		ID:       "act-1",
		AgentID:  "agent-1",
		Action:   "execute",
		Status:   "success",
		Duration: 100,
	}

	t.Run("RecordActivity", func(t *testing.T) {
		err := store.RecordActivity(ctx, activity)
		require.NoError(t, err)
	})

	t.Run("GetActivities", func(t *testing.T) {
		results, err := store.GetActivities(ctx, "agent-1", 10)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, activity.ID, results[0].ID)
	})

	t.Run("GetActivities_WithLimit", func(t *testing.T) {
		_ = store.RecordActivity(ctx, &models.Activity{ID: "act-2", AgentID: "agent-1", Action: "execute"})
		_ = store.RecordActivity(ctx, &models.Activity{ID: "act-3", AgentID: "agent-1", Action: "execute"})

		results, err := store.GetActivities(ctx, "agent-1", 2)
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("GetActivityByID", func(t *testing.T) {
		result, err := store.GetActivityByID(ctx, "act-1")
		require.NoError(t, err)
		assert.Equal(t, activity.ID, result.ID)
	})

	t.Run("GetActivityByID_NotFound", func(t *testing.T) {
		_, err := store.GetActivityByID(ctx, "non-existent")
		assert.ErrorIs(t, err, ErrActivityNotFound)
	})

	t.Run("GetActivities_NotFound", func(t *testing.T) {
		results, err := store.GetActivities(ctx, "non-existent", 10)
		require.NoError(t, err)
		assert.Len(t, results, 0)
	})
}

func TestInMemoryStore_Transaction(t *testing.T) {
	store := NewInMemory()
	ctx := context.Background()

	agent := &models.Agent{
		ID:   "tx-agent",
		Name: "Transaction Test",
	}

	t.Run("BeginAndCommit", func(t *testing.T) {
		tx, err := store.Begin(ctx)
		require.NoError(t, err)

		err = tx.Create(ctx, agent)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		// Verify the agent was created
		result, err := store.Get(ctx, "tx-agent")
		require.NoError(t, err)
		assert.Equal(t, agent.Name, result.Name)
	})

	t.Run("Rollback", func(t *testing.T) {
		// In-memory store doesn't support true transactions
		// The rollback is a no-op - the test verifies the method is called without error
		tx, err := store.Begin(ctx)
		require.NoError(t, err)

		err = tx.Create(ctx, &models.Agent{ID: "rollback-agent", Name: "Transaction Test"})
		require.NoError(t, err)

		err = tx.Rollback()
		require.NoError(t, err)
	})
}

func TestInMemoryStore_Close(t *testing.T) {
	store := NewInMemory()
	err := store.Close()
	assert.NoError(t, err)
}

func TestInMemoryStore_Delete_Cascades(t *testing.T) {
	store := NewInMemory()
	ctx := context.Background()

	agent := &models.Agent{ID: "cascade-agent", Name: "Cascade Test"}
	_ = store.Create(ctx, agent)

	_ = store.CreateMetrics(ctx, &models.Metrics{AgentID: "cascade-agent"})
	_ = store.RecordActivity(ctx, &models.Activity{ID: "cascade-act", AgentID: "cascade-agent"})

	err := store.Delete(ctx, "cascade-agent")
	require.NoError(t, err)

	_, err = store.GetMetrics(ctx, "cascade-agent")
	assert.ErrorIs(t, err, ErrMetricsNotFound)

	activities, _ := store.GetActivities(ctx, "cascade-agent", 10)
	assert.Len(t, activities, 0)
}

func strPtr(s string) *string {
	return &s
}
