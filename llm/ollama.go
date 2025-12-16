package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OllamaProvider implements the Provider interface for Ollama (local models)
type OllamaProvider struct {
	httpClient *http.Client
	baseURL    string
}

// NewOllama creates a new Ollama provider
// If baseURL is empty, defaults to http://localhost:11434
func NewOllama(baseURL string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	return &OllamaProvider{
		httpClient: &http.Client{},
		baseURL:    baseURL,
	}
}

// Name returns the provider name
func (p *OllamaProvider) Name() string {
	return "ollama"
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string                 `json:"model"`
	Prompt   string                 `json:"prompt,omitempty"`
	Messages []ollamaMessage        `json:"messages,omitempty"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type ollamaResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response,omitempty"`
	Message   struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message,omitempty"`
	Done bool `json:"done"`
}

// GenerateCompletion generates a text completion using Ollama
func (p *OllamaProvider) GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// Combine prompts
	prompt := req.SystemPrompt
	if prompt != "" {
		prompt += "\n\n"
	}
	prompt += req.UserPrompt

	ollamaReq := ollamaRequest{
		Model:  req.Model,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": req.Temperature,
		},
	}

	if req.MaxTokens > 0 {
		ollamaReq.Options["num_predict"] = req.MaxTokens
	}

	data, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/generate", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("ollama API error (status %d): %s", httpResp.StatusCode, string(body))
	}

	var resp ollamaResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &CompletionResponse{
		Text:         resp.Response,
		TokensUsed:   0, // Ollama doesn't return token count
		FinishReason: "stop",
		Model:        resp.Model,
	}, nil
}

// GenerateChat generates a chat response using Ollama
func (p *OllamaProvider) GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	ollamaReq := ollamaRequest{
		Model:  req.Model,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": req.Temperature,
		},
	}

	if req.MaxTokens > 0 {
		ollamaReq.Options["num_predict"] = req.MaxTokens
	}

	// Convert messages
	ollamaReq.Messages = make([]ollamaMessage, len(req.Messages))
	for i, msg := range req.Messages {
		ollamaReq.Messages[i] = ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	data, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("ollama API error (status %d): %s", httpResp.StatusCode, string(body))
	}

	var resp ollamaResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &ChatResponse{
		Message: Message{
			Role:    resp.Message.Role,
			Content: resp.Message.Content,
		},
		TokensUsed:   0, // Ollama doesn't return token count
		FinishReason: "stop",
		Model:        resp.Model,
	}, nil
}
