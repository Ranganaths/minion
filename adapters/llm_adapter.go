// Package adapters provides adapters for integrating chain and RAG components
// with the multi-agent system. These adapters bridge the llm.Provider interface
// with the multiagent.LLMProvider interface, enabling seamless interoperability.
package adapters

import (
	"context"
	"strings"

	"github.com/Ranganaths/minion/core/multiagent"
	"github.com/Ranganaths/minion/llm"
)

// MultiAgentLLMAdapter adapts llm.Provider to multiagent.LLMProvider.
// Use this adapter when you have an llm.Provider (e.g., OpenAI, Anthropic)
// and need to use it within the multi-agent system.
type MultiAgentLLMAdapter struct {
	provider llm.Provider
}

// NewMultiAgentLLMAdapter creates an adapter that wraps an llm.Provider
// for use with the multi-agent system.
func NewMultiAgentLLMAdapter(provider llm.Provider) *MultiAgentLLMAdapter {
	return &MultiAgentLLMAdapter{provider: provider}
}

// GenerateCompletion implements multiagent.LLMProvider.
// It converts the multiagent request format to llm.Provider format.
func (a *MultiAgentLLMAdapter) GenerateCompletion(
	ctx context.Context,
	req *multiagent.CompletionRequest,
) (*multiagent.CompletionResponse, error) {
	llmReq := &llm.CompletionRequest{
		SystemPrompt: req.SystemPrompt,
		UserPrompt:   req.UserPrompt,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		Model:        req.Model,
	}

	resp, err := a.provider.GenerateCompletion(ctx, llmReq)
	if err != nil {
		return nil, err
	}

	return &multiagent.CompletionResponse{
		Text:       resp.Text,
		TokensUsed: resp.TokensUsed,
	}, nil
}

// ChainLLMAdapter adapts multiagent.LLMProvider to llm.Provider.
// Use this adapter when you have a multiagent.LLMProvider and need
// to use it within chains (LLMChain, RAGChain, etc.).
type ChainLLMAdapter struct {
	provider multiagent.LLMProvider
}

// NewChainLLMAdapter creates an adapter that wraps a multiagent.LLMProvider
// for use with chain components.
func NewChainLLMAdapter(provider multiagent.LLMProvider) *ChainLLMAdapter {
	return &ChainLLMAdapter{provider: provider}
}

// GenerateCompletion implements llm.Provider.
// It converts the llm request format to multiagent format.
func (a *ChainLLMAdapter) GenerateCompletion(
	ctx context.Context,
	req *llm.CompletionRequest,
) (*llm.CompletionResponse, error) {
	maReq := &multiagent.CompletionRequest{
		SystemPrompt: req.SystemPrompt,
		UserPrompt:   req.UserPrompt,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		Model:        req.Model,
	}

	resp, err := a.provider.GenerateCompletion(ctx, maReq)
	if err != nil {
		return nil, err
	}

	return &llm.CompletionResponse{
		Text:       resp.Text,
		TokensUsed: resp.TokensUsed,
	}, nil
}

// GenerateChat implements llm.Provider.
// It converts chat messages to a completion request.
func (a *ChainLLMAdapter) GenerateChat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	// Build prompt from messages
	var promptBuilder strings.Builder
	var systemPrompt string

	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			systemPrompt = msg.Content
		case "user":
			promptBuilder.WriteString("User: ")
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n")
		case "assistant":
			promptBuilder.WriteString("Assistant: ")
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n")
		}
	}
	promptBuilder.WriteString("Assistant: ")

	resp, err := a.provider.GenerateCompletion(ctx, &multiagent.CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   promptBuilder.String(),
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		Model:        req.Model,
	})
	if err != nil {
		return nil, err
	}

	return &llm.ChatResponse{
		Message: llm.Message{
			Role:    "assistant",
			Content: resp.Text,
		},
		TokensUsed: resp.TokensUsed,
	}, nil
}

// Name implements llm.Provider.
func (a *ChainLLMAdapter) Name() string {
	return "multiagent_adapter"
}
