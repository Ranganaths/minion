package agents

import (
	"context"
	"testing"

	"github.com/Ranganaths/minion/llm"
)

// mockLLMProvider implements llm.Provider for testing
type mockLLMProvider struct {
	responses []string
	callCount int
}

func (m *mockLLMProvider) GenerateCompletion(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	response := "Final Answer: Test response"
	if m.callCount < len(m.responses) {
		response = m.responses[m.callCount]
	}
	m.callCount++
	return &llm.CompletionResponse{
		Text:       response,
		TokensUsed: 10,
	}, nil
}

func (m *mockLLMProvider) GenerateChat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{
		Message:    llm.Message{Role: "assistant", Content: "Chat response"},
		TokensUsed: 10,
	}, nil
}

func (m *mockLLMProvider) Name() string {
	return "mock"
}

func TestCalculatorTool(t *testing.T) {
	ctx := context.Background()
	calc := NewCalculatorTool()

	t.Run("basic operations", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"2 + 2", "4"},
			{"10 - 3", "7"},
			{"4 * 5", "20"},
			{"15 / 3", "5"},
			{"(2 + 3) * 4", "20"},
		}

		for _, tt := range tests {
			result, err := calc.Call(ctx, tt.input)
			if err != nil {
				t.Errorf("unexpected error for %s: %v", tt.input, err)
				continue
			}
			if result != tt.expected {
				t.Errorf("for %s: expected %s, got %s", tt.input, tt.expected, result)
			}
		}
	})

	t.Run("tool metadata", func(t *testing.T) {
		if calc.Name() != "Calculator" {
			t.Errorf("unexpected name: %s", calc.Name())
		}
		if calc.Description() == "" {
			t.Error("expected non-empty description")
		}
	})
}

func TestFunctionTool(t *testing.T) {
	ctx := context.Background()

	t.Run("create and call", func(t *testing.T) {
		tool, err := NewFunctionTool(FunctionToolConfig{
			Name:        "echo",
			Description: "Echoes input",
			Func: func(ctx context.Context, input string) (string, error) {
				return "Echo: " + input, nil
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err := tool.Call(ctx, "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != "Echo: hello" {
			t.Errorf("unexpected result: %s", result)
		}
	})

	t.Run("missing name returns error", func(t *testing.T) {
		_, err := NewFunctionTool(FunctionToolConfig{
			Description: "test",
			Func:        func(ctx context.Context, input string) (string, error) { return "", nil },
		})
		if err == nil {
			t.Error("expected error for missing name")
		}
	})
}

func TestToolRegistry(t *testing.T) {
	registry := NewToolRegistry()
	calc := NewCalculatorTool()

	t.Run("register and get", func(t *testing.T) {
		registry.Register(calc)

		tool, ok := registry.Get("calculator")
		if !ok {
			t.Error("expected to find tool")
		}
		if tool.Name() != "Calculator" {
			t.Errorf("unexpected tool name: %s", tool.Name())
		}
	})

	t.Run("list tools", func(t *testing.T) {
		tools := registry.List()
		if len(tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(tools))
		}
	})

	t.Run("remove tool", func(t *testing.T) {
		registry.Remove("Calculator")
		_, ok := registry.Get("Calculator")
		if ok {
			t.Error("expected tool to be removed")
		}
	})
}

func TestReActAgent(t *testing.T) {
	ctx := context.Background()

	t.Run("create agent", func(t *testing.T) {
		llm := &mockLLMProvider{}
		calc := NewCalculatorTool()

		agent, err := NewReActAgent(ReActAgentConfig{
			LLM:   llm,
			Tools: []Tool{calc},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(agent.InputKeys()) != 1 || agent.InputKeys()[0] != "input" {
			t.Error("unexpected input keys")
		}

		if len(agent.OutputKeys()) != 1 || agent.OutputKeys()[0] != "output" {
			t.Error("unexpected output keys")
		}
	})

	t.Run("plan with final answer", func(t *testing.T) {
		llm := &mockLLMProvider{
			responses: []string{"Thought: I know this.\nFinal Answer: 42"},
		}

		agent, _ := NewReActAgent(ReActAgentConfig{
			LLM:   llm,
			Tools: []Tool{NewCalculatorTool()},
		})

		action, err := agent.Plan(ctx, AgentInput{Input: "What is 6 * 7?"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !action.Finish {
			t.Error("expected finish action")
		}
		if action.FinalAnswer != "42" {
			t.Errorf("expected '42', got '%s'", action.FinalAnswer)
		}
	})

	t.Run("plan with tool action", func(t *testing.T) {
		llm := &mockLLMProvider{
			responses: []string{"Thought: I need to calculate.\nAction: Calculator\nAction Input: 2 + 2"},
		}

		agent, _ := NewReActAgent(ReActAgentConfig{
			LLM:   llm,
			Tools: []Tool{NewCalculatorTool()},
		})

		action, err := agent.Plan(ctx, AgentInput{Input: "Calculate 2 + 2"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if action.Finish {
			t.Error("expected non-finish action")
		}
		if action.Tool != "Calculator" {
			t.Errorf("expected 'Calculator', got '%s'", action.Tool)
		}
		if action.ToolInput != "2 + 2" {
			t.Errorf("expected '2 + 2', got '%s'", action.ToolInput)
		}
	})

	t.Run("get tool", func(t *testing.T) {
		agent, _ := NewReActAgent(ReActAgentConfig{
			LLM:   &mockLLMProvider{},
			Tools: []Tool{NewCalculatorTool()},
		})

		tool, ok := agent.GetTool("Calculator")
		if !ok {
			t.Error("expected to find Calculator tool")
		}
		if tool.Name() != "Calculator" {
			t.Errorf("unexpected tool: %s", tool.Name())
		}

		_, ok = agent.GetTool("NonExistent")
		if ok {
			t.Error("expected not to find NonExistent tool")
		}
	})

	t.Run("missing LLM returns error", func(t *testing.T) {
		_, err := NewReActAgent(ReActAgentConfig{
			Tools: []Tool{NewCalculatorTool()},
		})
		if err == nil {
			t.Error("expected error for missing LLM")
		}
	})

	t.Run("missing tools returns error", func(t *testing.T) {
		_, err := NewReActAgent(ReActAgentConfig{
			LLM: &mockLLMProvider{},
		})
		if err == nil {
			t.Error("expected error for missing tools")
		}
	})
}

func TestAgentExecutor(t *testing.T) {
	ctx := context.Background()

	t.Run("run to completion", func(t *testing.T) {
		llm := &mockLLMProvider{
			responses: []string{"Final Answer: Done!"},
		}

		agent, _ := NewReActAgent(ReActAgentConfig{
			LLM:   llm,
			Tools: []Tool{NewCalculatorTool()},
		})

		executor, err := NewAgentExecutor(AgentExecutorConfig{
			Agent: agent,
			Tools: []Tool{NewCalculatorTool()},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err := executor.Run(ctx, "Complete this task")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != "Done!" {
			t.Errorf("unexpected result: %s", result)
		}
	})

	t.Run("run with tool use", func(t *testing.T) {
		llm := &mockLLMProvider{
			responses: []string{
				"Thought: Need to calculate.\nAction: Calculator\nAction Input: 5 + 5",
				"Final Answer: The result is 10",
			},
		}

		agent, _ := NewReActAgent(ReActAgentConfig{
			LLM:   llm,
			Tools: []Tool{NewCalculatorTool()},
		})

		executor, _ := NewAgentExecutor(AgentExecutorConfig{
			Agent:         agent,
			Tools:         []Tool{NewCalculatorTool()},
			MaxIterations: 5,
		})

		result, err := executor.Run(ctx, "What is 5 + 5?")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != "The result is 10" {
			t.Errorf("unexpected result: %s", result)
		}
	})

	t.Run("stream events", func(t *testing.T) {
		llm := &mockLLMProvider{
			responses: []string{"Final Answer: Streamed result"},
		}

		agent, _ := NewReActAgent(ReActAgentConfig{
			LLM:   llm,
			Tools: []Tool{NewCalculatorTool()},
		})

		executor, _ := NewAgentExecutor(AgentExecutorConfig{
			Agent: agent,
			Tools: []Tool{NewCalculatorTool()},
		})

		ch, err := executor.Stream(ctx, "Stream this")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var finishEvent *AgentStreamEvent
		for event := range ch {
			if event.Type == AgentEventFinish {
				finishEvent = &event
			}
		}

		if finishEvent == nil {
			t.Error("expected finish event")
		} else if finishEvent.FinalAnswer != "Streamed result" {
			t.Errorf("unexpected final answer: %s", finishEvent.FinalAnswer)
		}
	})
}

func TestConversationalReActAgent(t *testing.T) {
	t.Run("includes chat history in input keys", func(t *testing.T) {
		llm := &mockLLMProvider{}

		agent, err := NewConversationalReActAgent(ConversationalReActAgentConfig{
			ReActAgentConfig: ReActAgentConfig{
				LLM:   llm,
				Tools: []Tool{NewCalculatorTool()},
			},
			MemoryKey: "history",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		keys := agent.InputKeys()
		if len(keys) != 2 {
			t.Errorf("expected 2 input keys, got %d", len(keys))
		}
	})
}
