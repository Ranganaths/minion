package adapters

import (
	"context"
	"testing"

	"github.com/Ranganaths/minion/core/multiagent"
	"github.com/Ranganaths/minion/llm"
)

// mockLLMProvider implements llm.Provider for testing
type mockLLMProvider struct {
	completionResponse *llm.CompletionResponse
	chatResponse       *llm.ChatResponse
	err                error
}

func (m *mockLLMProvider) GenerateCompletion(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.completionResponse, nil
}

func (m *mockLLMProvider) GenerateChat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.chatResponse, nil
}

func (m *mockLLMProvider) Name() string {
	return "mock"
}

// mockMultiAgentLLMProvider implements multiagent.LLMProvider for testing
type mockMultiAgentLLMProvider struct {
	response *multiagent.CompletionResponse
	err      error
}

func (m *mockMultiAgentLLMProvider) GenerateCompletion(ctx context.Context, req *multiagent.CompletionRequest) (*multiagent.CompletionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func TestMultiAgentLLMAdapter(t *testing.T) {
	ctx := context.Background()

	t.Run("successful completion", func(t *testing.T) {
		provider := &mockLLMProvider{
			completionResponse: &llm.CompletionResponse{
				Text:       "Hello, world!",
				TokensUsed: 10,
			},
		}

		adapter := NewMultiAgentLLMAdapter(provider)

		resp, err := adapter.GenerateCompletion(ctx, &multiagent.CompletionRequest{
			SystemPrompt: "You are a helpful assistant",
			UserPrompt:   "Say hello",
			Temperature:  0.7,
			MaxTokens:    100,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Text != "Hello, world!" {
			t.Errorf("expected 'Hello, world!', got '%s'", resp.Text)
		}

		if resp.TokensUsed != 10 {
			t.Errorf("expected 10 tokens, got %d", resp.TokensUsed)
		}
	})

	t.Run("handles error", func(t *testing.T) {
		provider := &mockLLMProvider{
			err: context.DeadlineExceeded,
		}

		adapter := NewMultiAgentLLMAdapter(provider)

		_, err := adapter.GenerateCompletion(ctx, &multiagent.CompletionRequest{
			UserPrompt: "Say hello",
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestChainLLMAdapter(t *testing.T) {
	ctx := context.Background()

	t.Run("successful completion", func(t *testing.T) {
		provider := &mockMultiAgentLLMProvider{
			response: &multiagent.CompletionResponse{
				Text:       "Generated response",
				TokensUsed: 15,
			},
		}

		adapter := NewChainLLMAdapter(provider)

		resp, err := adapter.GenerateCompletion(ctx, &llm.CompletionRequest{
			SystemPrompt: "You are a helpful assistant",
			UserPrompt:   "Generate something",
			Temperature:  0.5,
			MaxTokens:    200,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Text != "Generated response" {
			t.Errorf("expected 'Generated response', got '%s'", resp.Text)
		}

		if resp.TokensUsed != 15 {
			t.Errorf("expected 15 tokens, got %d", resp.TokensUsed)
		}
	})

	t.Run("successful chat", func(t *testing.T) {
		provider := &mockMultiAgentLLMProvider{
			response: &multiagent.CompletionResponse{
				Text:       "Chat response",
				TokensUsed: 20,
			},
		}

		adapter := NewChainLLMAdapter(provider)

		resp, err := adapter.GenerateChat(ctx, &llm.ChatRequest{
			Messages: []llm.Message{
				{Role: "system", Content: "You are a helpful assistant"},
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
				{Role: "user", Content: "How are you?"},
			},
			Temperature: 0.5,
			MaxTokens:   200,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Message.Role != "assistant" {
			t.Errorf("expected role 'assistant', got '%s'", resp.Message.Role)
		}

		if resp.Message.Content != "Chat response" {
			t.Errorf("expected 'Chat response', got '%s'", resp.Message.Content)
		}

		if resp.TokensUsed != 20 {
			t.Errorf("expected 20 tokens, got %d", resp.TokensUsed)
		}
	})

	t.Run("name returns multiagent_adapter", func(t *testing.T) {
		provider := &mockMultiAgentLLMProvider{}
		adapter := NewChainLLMAdapter(provider)

		if adapter.Name() != "multiagent_adapter" {
			t.Errorf("expected name 'multiagent_adapter', got '%s'", adapter.Name())
		}
	})
}
