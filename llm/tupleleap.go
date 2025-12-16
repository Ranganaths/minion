package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// TupleLeapProvider implements the Provider interface for TupleLeap AI
type TupleLeapProvider struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// NewTupleLeap creates a new TupleLeap AI provider
func NewTupleLeap(apiKey string) *TupleLeapProvider {
	return &TupleLeapProvider{
		apiKey:     apiKey,
		httpClient: &http.Client{},
		baseURL:    "https://api.tupleleap.ai/v1",
	}
}

// NewTupleLeapWithBaseURL creates a TupleLeap provider with custom base URL
func NewTupleLeapWithBaseURL(apiKey, baseURL string) *TupleLeapProvider {
	return &TupleLeapProvider{
		apiKey:     apiKey,
		httpClient: &http.Client{},
		baseURL:    baseURL,
	}
}

// Name returns the provider name
func (p *TupleLeapProvider) Name() string {
	return "tupleleap"
}

// TupleLeap request/response structures
type tupleLeapMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type tupleLeapCompletionRequest struct {
	Model       string             `json:"model"`
	Prompt      string             `json:"prompt,omitempty"`
	Messages    []tupleLeapMessage `json:"messages,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Stream      bool               `json:"stream"`
}

type tupleLeapCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message,omitempty"`
		Text         string `json:"text,omitempty"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// GenerateCompletion generates a text completion using TupleLeap AI
func (p *TupleLeapProvider) GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// Build prompt
	prompt := req.SystemPrompt
	if prompt != "" {
		prompt += "\n\n"
	}
	prompt += req.UserPrompt

	tupleLeapReq := tupleLeapCompletionRequest{
		Model:       req.Model,
		Prompt:      prompt,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	data, err := json.Marshal(tupleLeapReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/completions", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("tupleleap API error (status %d): %s", httpResp.StatusCode, string(body))
	}

	var resp tupleLeapCompletionResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no completion choices returned")
	}

	// Extract text from response
	text := resp.Choices[0].Text
	if text == "" && resp.Choices[0].Message.Content != "" {
		text = resp.Choices[0].Message.Content
	}

	return &CompletionResponse{
		Text:         text,
		TokensUsed:   resp.Usage.TotalTokens,
		FinishReason: resp.Choices[0].FinishReason,
		Model:        resp.Model,
	}, nil
}

// GenerateChat generates a chat response using TupleLeap AI
func (p *TupleLeapProvider) GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Convert messages
	messages := make([]tupleLeapMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = tupleLeapMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	tupleLeapReq := tupleLeapCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	data, err := json.Marshal(tupleLeapReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("tupleleap API error (status %d): %s", httpResp.StatusCode, string(body))
	}

	var resp tupleLeapCompletionResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no chat choices returned")
	}

	return &ChatResponse{
		Message: Message{
			Role:    resp.Choices[0].Message.Role,
			Content: resp.Choices[0].Message.Content,
		},
		TokensUsed:   resp.Usage.TotalTokens,
		FinishReason: resp.Choices[0].FinishReason,
		Model:        resp.Model,
	}, nil
}

// SetHTTPClient allows setting a custom HTTP client
func (p *TupleLeapProvider) SetHTTPClient(client *http.Client) {
	p.httpClient = client
}

// SetBaseURL allows setting a custom base URL
func (p *TupleLeapProvider) SetBaseURL(baseURL string) {
	p.baseURL = baseURL
}
