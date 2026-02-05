package tracing

import (
	"context"

	"github.com/Ranganaths/minion/llm"
)

// TracedLLMProvider wraps an LLM provider with tracing
type TracedLLMProvider struct {
	provider  llm.Provider
	collector *TraceCollector
}

// NewTracedLLMProvider creates a new traced LLM provider
func NewTracedLLMProvider(provider llm.Provider, collector *TraceCollector) *TracedLLMProvider {
	return &TracedLLMProvider{
		provider:  provider,
		collector: collector,
	}
}

// GenerateCompletion generates a completion with tracing
func (p *TracedLLMProvider) GenerateCompletion(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	// Start LLM span
	ctx, spanID := p.collector.StartLLMSpan(
		ctx,
		p.provider.Name(),
		req.Model,
		req.SystemPrompt,
		req.UserPrompt,
		req.Temperature,
		req.MaxTokens,
	)

	// Call underlying provider
	resp, err := p.provider.GenerateCompletion(ctx, req)

	// End LLM span
	var response string
	var promptTokens, completionTokens int
	var cost float64
	var finishReason string

	if resp != nil {
		response = resp.Text
		promptTokens = resp.TokensUsed / 2  // Estimate if not provided separately
		completionTokens = resp.TokensUsed / 2
		finishReason = resp.FinishReason
		// Estimate cost based on model (could be improved with a pricing table)
		cost = estimateCost(req.Model, resp.TokensUsed)
	}

	p.collector.EndLLMSpan(ctx, spanID, response, promptTokens, completionTokens, cost, finishReason, err)

	return resp, err
}

// GenerateChat generates a chat completion with tracing
func (p *TracedLLMProvider) GenerateChat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	// Build a combined prompt for tracing purposes
	var systemPrompt, userPrompt string
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			systemPrompt += msg.Content + "\n"
		case "user":
			userPrompt += msg.Content + "\n"
		}
	}

	// Start LLM span
	ctx, spanID := p.collector.StartLLMSpan(
		ctx,
		p.provider.Name(),
		req.Model,
		systemPrompt,
		userPrompt,
		req.Temperature,
		req.MaxTokens,
	)

	// Call underlying provider
	resp, err := p.provider.GenerateChat(ctx, req)

	// End LLM span
	var response string
	var promptTokens, completionTokens int
	var cost float64
	var finishReason string

	if resp != nil {
		response = resp.Message.Content
		// TokensUsed is total, estimate split
		promptTokens = resp.TokensUsed / 2
		completionTokens = resp.TokensUsed - promptTokens
		finishReason = resp.FinishReason
		cost = estimateCost(req.Model, resp.TokensUsed)
	}

	p.collector.EndLLMSpan(ctx, spanID, response, promptTokens, completionTokens, cost, finishReason, err)

	return resp, err
}

// Name returns the provider name
func (p *TracedLLMProvider) Name() string {
	return p.provider.Name()
}

// estimateCost estimates the cost based on model and tokens
// This is a simplified estimation - in production, use actual pricing
func estimateCost(model string, totalTokens int) float64 {
	// Rough pricing per 1K tokens (as of 2024)
	var pricePerKToken float64
	switch {
	case contains(model, "gpt-4"):
		pricePerKToken = 0.03 // $0.03 per 1K tokens average
	case contains(model, "gpt-3.5"):
		pricePerKToken = 0.002
	case contains(model, "claude-3-opus"):
		pricePerKToken = 0.015
	case contains(model, "claude-3-sonnet"):
		pricePerKToken = 0.003
	case contains(model, "claude-3-haiku"):
		pricePerKToken = 0.00025
	default:
		pricePerKToken = 0.001 // Default fallback
	}
	return float64(totalTokens) / 1000.0 * pricePerKToken
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Ensure TracedLLMProvider implements llm.Provider
var _ llm.Provider = (*TracedLLMProvider)(nil)
