package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AnthropicProvider implements the Provider interface for Anthropic Claude
type AnthropicProvider struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// NewAnthropic creates a new Anthropic provider
func NewAnthropic(apiKey string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey:     apiKey,
		httpClient: &http.Client{},
		baseURL:    "https://api.anthropic.com/v1",
	}
}

// Name returns the provider name
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	System      string             `json:"system,omitempty"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Model string `json:"model"`
}

// GenerateCompletion generates a text completion using Anthropic Claude
func (p *AnthropicProvider) GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// Build request
	anthropicReq := anthropicRequest{
		Model: req.Model,
		Messages: []anthropicMessage{
			{Role: "user", Content: req.UserPrompt},
		},
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		System:      req.SystemPrompt,
	}

	// Make API call
	resp, err := p.callAPI(ctx, anthropicReq)
	if err != nil {
		return nil, err
	}

	// Extract text
	var text string
	if len(resp.Content) > 0 {
		text = resp.Content[0].Text
	}

	return &CompletionResponse{
		Text:         text,
		TokensUsed:   resp.Usage.InputTokens + resp.Usage.OutputTokens,
		FinishReason: resp.StopReason,
		Model:        resp.Model,
	}, nil
}

// GenerateChat generates a chat response using Anthropic Claude
func (p *AnthropicProvider) GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Convert messages
	messages := make([]anthropicMessage, 0)
	var systemPrompt string

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
		} else {
			messages = append(messages, anthropicMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	anthropicReq := anthropicRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		System:      systemPrompt,
	}

	resp, err := p.callAPI(ctx, anthropicReq)
	if err != nil {
		return nil, err
	}

	var text string
	if len(resp.Content) > 0 {
		text = resp.Content[0].Text
	}

	return &ChatResponse{
		Message: Message{
			Role:    resp.Role,
			Content: text,
		},
		TokensUsed:   resp.Usage.InputTokens + resp.Usage.OutputTokens,
		FinishReason: resp.StopReason,
		Model:        resp.Model,
	}, nil
}

func (p *AnthropicProvider) callAPI(ctx context.Context, req anthropicRequest) (*anthropicResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("anthropic API error (status %d): %s", httpResp.StatusCode, string(body))
	}

	var resp anthropicResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
