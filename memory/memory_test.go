package memory

import (
	"context"
	"testing"
)

func TestInMemoryChatMessageHistory(t *testing.T) {
	ctx := context.Background()

	t.Run("add and retrieve messages", func(t *testing.T) {
		history := NewInMemoryChatMessageHistory()

		err := history.AddUserMessage(ctx, "Hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = history.AddAIMessage(ctx, "Hi there!")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		messages, err := history.Messages(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(messages))
		}

		if messages[0].Role != RoleHuman {
			t.Errorf("expected human role, got %s", messages[0].Role)
		}
		if messages[0].Content != "Hello" {
			t.Errorf("expected 'Hello', got '%s'", messages[0].Content)
		}

		if messages[1].Role != RoleAI {
			t.Errorf("expected AI role, got %s", messages[1].Role)
		}
	})

	t.Run("clear messages", func(t *testing.T) {
		history := NewInMemoryChatMessageHistory()
		history.AddUserMessage(ctx, "Test")
		history.Clear(ctx)

		if history.Len() != 0 {
			t.Errorf("expected 0 messages after clear, got %d", history.Len())
		}
	})

	t.Run("last message", func(t *testing.T) {
		history := NewInMemoryChatMessageHistory()

		_, ok := history.LastMessage()
		if ok {
			t.Error("expected no last message on empty history")
		}

		history.AddUserMessage(ctx, "First")
		history.AddAIMessage(ctx, "Second")

		msg, ok := history.LastMessage()
		if !ok {
			t.Error("expected last message")
		}
		if msg.Content != "Second" {
			t.Errorf("expected 'Second', got '%s'", msg.Content)
		}
	})
}

func TestConversationBufferMemory(t *testing.T) {
	ctx := context.Background()

	t.Run("save and load context", func(t *testing.T) {
		memory := NewConversationBufferMemory()

		// Save a conversation turn
		err := memory.SaveContext(ctx,
			map[string]any{"input": "What is AI?"},
			map[string]any{"output": "AI stands for Artificial Intelligence."})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Load memory variables
		vars, err := memory.LoadMemoryVariables(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		history, ok := vars["history"].(string)
		if !ok {
			t.Fatal("expected string history")
		}

		if history == "" {
			t.Error("expected non-empty history")
		}

		if !contains(history, "What is AI?") || !contains(history, "AI stands for") {
			t.Errorf("history missing content: %s", history)
		}
	})

	t.Run("custom config", func(t *testing.T) {
		memory := NewConversationBufferMemory(ConversationBufferMemoryConfig{
			MemoryConfig: MemoryConfig{
				MemoryKey:   "chat_history",
				InputKey:    "question",
				OutputKey:   "answer",
				HumanPrefix: "User",
				AIPrefix:    "Bot",
			},
		})

		memory.SaveContext(ctx,
			map[string]any{"question": "Hi"},
			map[string]any{"answer": "Hello!"})

		vars, _ := memory.LoadMemoryVariables(ctx)
		if _, ok := vars["chat_history"]; !ok {
			t.Error("expected chat_history key")
		}
	})

	t.Run("return messages mode", func(t *testing.T) {
		memory := NewConversationBufferMemory(ConversationBufferMemoryConfig{
			MemoryConfig: MemoryConfig{
				ReturnMessages: true,
			},
		})

		memory.SaveContext(ctx,
			map[string]any{"input": "Test"},
			map[string]any{"output": "Response"})

		vars, _ := memory.LoadMemoryVariables(ctx)
		messages, ok := vars["history"].([]ChatMessage)
		if !ok {
			t.Fatal("expected []ChatMessage")
		}

		if len(messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(messages))
		}
	})

	t.Run("memory variables", func(t *testing.T) {
		memory := NewConversationBufferMemory()
		vars := memory.MemoryVariables()

		if len(vars) != 1 || vars[0] != "history" {
			t.Errorf("unexpected memory variables: %v", vars)
		}
	})
}

func TestConversationBufferWindowMemory(t *testing.T) {
	ctx := context.Background()

	t.Run("window limits messages", func(t *testing.T) {
		memory := NewConversationBufferWindowMemory(2) // Keep 2 turns

		// Add 3 turns
		for i := 0; i < 3; i++ {
			memory.SaveContext(ctx,
				map[string]any{"input": "Question " + string(rune('A'+i))},
				map[string]any{"output": "Answer " + string(rune('A'+i))})
		}

		vars, _ := memory.LoadMemoryVariables(ctx)
		history := vars["history"].(string)

		// Should not contain first turn
		if contains(history, "Question A") {
			t.Error("expected Question A to be excluded from window")
		}

		// Should contain last two turns
		if !contains(history, "Question B") || !contains(history, "Question C") {
			t.Error("expected Question B and C to be in window")
		}
	})
}

func TestConversationSummaryMemory(t *testing.T) {
	ctx := context.Background()

	t.Run("summarize conversation", func(t *testing.T) {
		summaries := []string{}
		mockSummarize := func(ctx context.Context, existing string, messages []ChatMessage) (string, error) {
			summary := existing
			for _, msg := range messages {
				if summary != "" {
					summary += " "
				}
				summary += "[" + string(msg.Role) + ": " + msg.Content + "]"
			}
			summaries = append(summaries, summary)
			return summary, nil
		}

		memory := NewConversationSummaryMemory(ConversationSummaryMemoryConfig{
			SummarizeFunc: mockSummarize,
		})

		memory.SaveContext(ctx,
			map[string]any{"input": "Hello"},
			map[string]any{"output": "Hi"})

		if memory.GetSummary() == "" {
			t.Error("expected non-empty summary")
		}

		vars, _ := memory.LoadMemoryVariables(ctx)
		if vars["history"] != memory.GetSummary() {
			t.Error("loaded history should match summary")
		}
	})
}

func TestChatMessageHelpers(t *testing.T) {
	t.Run("NewHumanMessage", func(t *testing.T) {
		msg := NewHumanMessage("test")
		if msg.Role != RoleHuman || msg.Content != "test" {
			t.Error("unexpected message")
		}
	})

	t.Run("NewAIMessage", func(t *testing.T) {
		msg := NewAIMessage("response")
		if msg.Role != RoleAI || msg.Content != "response" {
			t.Error("unexpected message")
		}
	})

	t.Run("NewSystemMessage", func(t *testing.T) {
		msg := NewSystemMessage("system")
		if msg.Role != RoleSystem {
			t.Error("unexpected role")
		}
	})

	t.Run("NewFunctionMessage", func(t *testing.T) {
		msg := NewFunctionMessage("calc", "42")
		if msg.Role != RoleFunction || msg.Name != "calc" || msg.Content != "42" {
			t.Error("unexpected message")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
