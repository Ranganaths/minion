package llm

import (
	"testing"
)

func TestCompletionRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *CompletionRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request",
			req: &CompletionRequest{
				Model:       "gpt-4",
				UserPrompt:  "Hello",
				Temperature: 0.7,
				MaxTokens:   100,
			},
			wantErr: false,
		},
		{
			name: "valid with system prompt only",
			req: &CompletionRequest{
				Model:        "gpt-4",
				SystemPrompt: "You are helpful",
				Temperature:  0.5,
			},
			wantErr: false,
		},
		{
			name: "missing model",
			req: &CompletionRequest{
				UserPrompt:  "Hello",
				Temperature: 0.7,
			},
			wantErr: true,
			errMsg:  "Model",
		},
		{
			name: "missing prompts",
			req: &CompletionRequest{
				Model:       "gpt-4",
				Temperature: 0.7,
			},
			wantErr: true,
			errMsg:  "UserPrompt",
		},
		{
			name: "temperature too low",
			req: &CompletionRequest{
				Model:       "gpt-4",
				UserPrompt:  "Hello",
				Temperature: -0.1,
			},
			wantErr: true,
			errMsg:  "Temperature",
		},
		{
			name: "temperature too high",
			req: &CompletionRequest{
				Model:       "gpt-4",
				UserPrompt:  "Hello",
				Temperature: 2.5,
			},
			wantErr: true,
			errMsg:  "Temperature",
		},
		{
			name: "negative max tokens",
			req: &CompletionRequest{
				Model:      "gpt-4",
				UserPrompt: "Hello",
				MaxTokens:  -10,
			},
			wantErr: true,
			errMsg:  "MaxTokens",
		},
		{
			name: "zero values are valid",
			req: &CompletionRequest{
				Model:       "gpt-4",
				UserPrompt:  "Hello",
				Temperature: 0,
				MaxTokens:   0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				ve, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("expected ValidationError, got %T", err)
					return
				}
				if ve.Field != tt.errMsg {
					t.Errorf("expected error field %s, got %s", tt.errMsg, ve.Field)
				}
			}
		})
	}
}

func TestChatRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *ChatRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request",
			req: &ChatRequest{
				Model: "gpt-4",
				Messages: []Message{
					{Role: "user", Content: "Hello"},
				},
				Temperature: 0.7,
			},
			wantErr: false,
		},
		{
			name: "valid with multiple messages",
			req: &ChatRequest{
				Model: "gpt-4",
				Messages: []Message{
					{Role: "system", Content: "Be helpful"},
					{Role: "user", Content: "Hello"},
					{Role: "assistant", Content: "Hi there!"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing model",
			req: &ChatRequest{
				Messages: []Message{
					{Role: "user", Content: "Hello"},
				},
			},
			wantErr: true,
			errMsg:  "Model",
		},
		{
			name: "empty messages",
			req: &ChatRequest{
				Model:    "gpt-4",
				Messages: []Message{},
			},
			wantErr: true,
			errMsg:  "Messages",
		},
		{
			name: "invalid role",
			req: &ChatRequest{
				Model: "gpt-4",
				Messages: []Message{
					{Role: "invalid", Content: "Hello"},
				},
			},
			wantErr: true,
			errMsg:  "Messages[0].Role",
		},
		{
			name: "empty role",
			req: &ChatRequest{
				Model: "gpt-4",
				Messages: []Message{
					{Role: "", Content: "Hello"},
				},
			},
			wantErr: true,
			errMsg:  "Messages[0].Role",
		},
		{
			name: "temperature too high",
			req: &ChatRequest{
				Model: "gpt-4",
				Messages: []Message{
					{Role: "user", Content: "Hello"},
				},
				Temperature: 3.0,
			},
			wantErr: true,
			errMsg:  "Temperature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				ve, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("expected ValidationError, got %T", err)
					return
				}
				if ve.Field != tt.errMsg {
					t.Errorf("expected error field %s, got %s", tt.errMsg, ve.Field)
				}
			}
		})
	}
}

func TestCompletionRequest_WithDefaults(t *testing.T) {
	req := &CompletionRequest{
		UserPrompt:  "Hello",
		Temperature: 0.7,
	}

	result := req.WithDefaults("gpt-4", 1000)

	if result.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", result.Model)
	}
	if result.MaxTokens != 1000 {
		t.Errorf("expected max tokens 1000, got %d", result.MaxTokens)
	}
	// Original should be unchanged
	if req.Model != "" {
		t.Errorf("original should be unchanged, got model %s", req.Model)
	}
}

func TestChatRequest_WithDefaults(t *testing.T) {
	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	result := req.WithDefaults("gpt-4", 500)

	if result.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", result.Model)
	}
	if result.MaxTokens != 500 {
		t.Errorf("expected max tokens 500, got %d", result.MaxTokens)
	}
}

func TestValidateCompletionRequest(t *testing.T) {
	req := &CompletionRequest{
		UserPrompt: "Hello",
	}

	result, err := ValidateCompletionRequest(req, "gpt-4", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", result.Model)
	}
}

func TestValidateChatRequest(t *testing.T) {
	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	result, err := ValidateChatRequest(req, "gpt-4", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", result.Model)
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{Field: "Model", Message: "is required"}
	expected := "validation error on field 'Model': is required"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}
