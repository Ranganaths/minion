package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/Ranganaths/minion/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTool struct {
	name        string
	description string
	canExec     bool
	execErr     error
	execResult  interface{}
}

func (m *mockTool) Name() string              { return m.name }
func (m *mockTool) Description() string      { return m.description }
func (m *mockTool) CanExecute(agent *models.Agent) bool { return m.canExec }
func (m *mockTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	if m.execErr != nil {
		return nil, m.execErr
	}
	return &models.ToolOutput{
		ToolName: m.name,
		Success:  true,
		Result:   m.execResult,
	}, nil
}

func TestInMemoryRegistry_Register(t *testing.T) {
	registry := NewRegistry()
	tool := &mockTool{name: "test-tool", description: "A test tool"}

	t.Run("Register_Success", func(t *testing.T) {
		err := registry.Register(tool)
		require.NoError(t, err)
		assert.Equal(t, 1, registry.Count())
	})

	t.Run("Register_Duplicate", func(t *testing.T) {
		err := registry.Register(tool)
		assert.ErrorIs(t, err, ErrToolAlreadyExists)
	})
}

func TestInMemoryRegistry_Get(t *testing.T) {
	registry := NewRegistry()
	tool := &mockTool{name: "get-tool", description: "Get test tool"}
	_ = registry.Register(tool)

	t.Run("Get_Exists", func(t *testing.T) {
		result, err := registry.Get("get-tool")
		require.NoError(t, err)
		assert.Equal(t, "get-tool", result.Name())
	})

	t.Run("Get_NotFound", func(t *testing.T) {
		_, err := registry.Get("non-existent")
		assert.ErrorIs(t, err, ErrToolNotFound)
	})
}

func TestInMemoryRegistry_Execute(t *testing.T) {
	registry := NewRegistry()
	tool := &mockTool{
		name:        "exec-tool",
		description: "Execute test tool",
		execResult:  "test result",
	}
	_ = registry.Register(tool)

	ctx := context.Background()
	input := &models.ToolInput{
		Data:   "test data",
		Params: map[string]interface{}{"key": "value"},
	}

	t.Run("Execute_Success", func(t *testing.T) {
		result, err := registry.Execute(ctx, "exec-tool", input)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, "test result", result.Result)
	})

	t.Run("Execute_NotFound", func(t *testing.T) {
		_, err := registry.Execute(ctx, "non-existent", input)
		assert.ErrorIs(t, err, ErrToolNotFound)
	})

	t.Run("Execute_ToolError", func(t *testing.T) {
		errTool := &mockTool{
			name:    "error-tool",
			execErr: errors.New("tool execution failed"),
		}
		_ = registry.Register(errTool)

		_, err := registry.Execute(ctx, "error-tool", input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool execution failed")
	})
}

func TestInMemoryRegistry_GetToolsForAgent(t *testing.T) {
	registry := NewRegistry()

	tool1 := &mockTool{name: "tool1", description: "Tool 1", canExec: true}
	tool2 := &mockTool{name: "tool2", description: "Tool 2", canExec: false}
	tool3 := &mockTool{name: "tool3", description: "Tool 3", canExec: true}

	_ = registry.Register(tool1)
	_ = registry.Register(tool2)
	_ = registry.Register(tool3)

	agent := &models.Agent{ID: "test-agent"}

	t.Run("GetToolsForAgent", func(t *testing.T) {
		tools := registry.GetToolsForAgent(agent)
		assert.Len(t, tools, 2)
		names := make([]string, len(tools))
		for i, t := range tools {
			names[i] = t.Name()
		}
		assert.Contains(t, names, "tool1")
		assert.Contains(t, names, "tool3")
		assert.NotContains(t, names, "tool2")
	})
}

func TestInMemoryRegistry_List(t *testing.T) {
	registry := NewRegistry()

	_ = registry.Register(&mockTool{name: "list-tool-1", description: "List tool 1"})
	_ = registry.Register(&mockTool{name: "list-tool-2", description: "List tool 2"})
	_ = registry.Register(&mockTool{name: "list-tool-3", description: "List tool 3"})

	names := registry.List()
	assert.Len(t, names, 3)
	assert.Contains(t, names, "list-tool-1")
	assert.Contains(t, names, "list-tool-2")
	assert.Contains(t, names, "list-tool-3")
}

func TestInMemoryRegistry_Count(t *testing.T) {
	registry := NewRegistry()
	assert.Equal(t, 0, registry.Count())

	_ = registry.Register(&mockTool{name: "count-tool-1", description: "Count tool 1"})
	assert.Equal(t, 1, registry.Count())

	_ = registry.Register(&mockTool{name: "count-tool-2", description: "Count tool 2"})
	assert.Equal(t, 2, registry.Count())
}

func TestToolExecution_Success(t *testing.T) {
	registry := NewRegistry()

	slowTool := &mockTool{
		name:        "exec-tool",
		description: "An execution tool",
		execResult:  "done",
	}
	_ = registry.Register(slowTool)

	ctx := context.Background()
	input := &models.ToolInput{Data: "test"}

	result, err := registry.Execute(ctx, "exec-tool", input)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "done", result.Result)
	assert.Equal(t, "exec-tool", result.ToolName)
}

func TestInMemoryRegistry_Concurrent(t *testing.T) {
	registry := NewRegistry()
	tool := &mockTool{name: "concurrent-tool", description: "Concurrent test tool"}
	_ = registry.Register(tool)

	ctx := context.Background()
	input := &models.ToolInput{Data: "test"}

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := registry.Execute(ctx, "concurrent-tool", input)
			done <- (err == nil)
		}()
	}

	for i := 0; i < 10; i++ {
		assert.True(t, <-done)
	}
}

func TestToolOutput_Structure(t *testing.T) {
	registry := NewRegistry()

	customTool := &mockTool{
		name:        "structured-tool",
		description: "Structured output tool",
		execResult: map[string]interface{}{
			"status":  "ok",
			"data":    []int{1, 2, 3},
			"count":   3,
		},
	}
	_ = registry.Register(customTool)

	ctx := context.Background()
	input := &models.ToolInput{Data: "test"}

	result, err := registry.Execute(ctx, "structured-tool", input)
	require.NoError(t, err)

	assert.Equal(t, "structured-tool", result.ToolName)
	assert.True(t, result.Success)
	assert.Empty(t, result.Error)
	assert.NotNil(t, result.Result)

	resultMap, ok := result.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "ok", resultMap["status"])
}
