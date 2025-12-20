# Minion LLM Provider Support

**Guide to using and extending LLM providers in Minion**

---

## Table of Contents

1. [Currently Supported Providers](#currently-supported-providers)
2. [Provider Interface](#provider-interface)
3. [Using OpenAI](#using-openai)
4. [Adding New Providers](#adding-new-providers)
5. [Provider Implementations](#provider-implementations)
6. [Best Practices](#best-practices)
7. [Roadmap](#roadmap)

---

## Currently Supported Providers

| Provider | Status | Models | Location |
|----------|--------|--------|----------|
| **OpenAI** | ✅ **Supported** | GPT-4, GPT-3.5-turbo, etc. | `llm/openai.go` |
| **TupleLeap AI** | ✅ **Supported** | Custom models | `llm/tupleleap.go` |
| Anthropic (Claude) | ✅ **Ready** | Claude 3, Claude 2 | `llm/anthropic.go` |
| Ollama | ✅ **Ready** | Llama 2, Mistral, etc. | `llm/ollama.go` |
| Google (Gemini) | 🔨 Implementation below | Gemini Pro, Gemini Ultra | See guide |
| Azure OpenAI | 🔨 Implementation below | GPT-4, GPT-3.5-turbo | See guide |
| Cohere | 📋 Planned | Command, Command R+ | TBD |
| Hugging Face | 📋 Planned | Various models | TBD |

---

## Provider Interface

Minion uses a clean, standardized interface for all LLM providers:

```go
package llm

type Provider interface {
    // GenerateCompletion generates a text completion
    GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)

    // GenerateChat generates a chat response
    GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

    // Name returns the provider name
    Name() string
}
```

### Request/Response Types

```go
// CompletionRequest represents a completion request
type CompletionRequest struct {
    SystemPrompt string
    UserPrompt   string
    Temperature  float64
    MaxTokens    int
    Model        string
}

// CompletionResponse represents a completion response
type CompletionResponse struct {
    Text          string
    TokensUsed    int
    FinishReason  string
    Model         string
}

// ChatRequest represents a chat request
type ChatRequest struct {
    Messages    []Message
    Temperature float64
    MaxTokens   int
    Model       string
}

// Message represents a chat message
type Message struct {
    Role    string // "system", "user", "assistant"
    Content string
}

// ChatResponse represents a chat response
type ChatResponse struct {
    Message       Message
    TokensUsed    int
    FinishReason  string
    Model         string
}
```

---

## Using OpenAI

### Installation

```bash
go get github.com/sashabaranov/go-openai
```

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/Ranganaths/minion/llm"
)

func main() {
    ctx := context.Background()

    // Create OpenAI provider
    apiKey := os.Getenv("OPENAI_API_KEY")
    provider := llm.NewOpenAI(apiKey)

    // Simple completion
    resp, err := provider.GenerateCompletion(ctx, &llm.CompletionRequest{
        SystemPrompt: "You are a helpful assistant.",
        UserPrompt:   "What is the capital of France?",
        Temperature:  0.7,
        MaxTokens:    100,
        Model:        "gpt-4",
    })

    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Response: %s\n", resp.Text)
    fmt.Printf("Tokens used: %d\n", resp.TokensUsed)
}
```

### Chat Conversation

```go
// Multi-turn conversation
chatResp, err := provider.GenerateChat(ctx, &llm.ChatRequest{
    Messages: []llm.Message{
        {Role: "system", Content: "You are a helpful assistant."},
        {Role: "user", Content: "Tell me about Go programming."},
        {Role: "assistant", Content: "Go is a statically typed..."},
        {Role: "user", Content: "What are its main features?"},
    },
    Temperature: 0.7,
    MaxTokens:   500,
    Model:       "gpt-4",
})
```

### Integration with Minion Workers

```go
package main

import (
    "context"

    "github.com/Ranganaths/minion/core/multiagent"
    "github.com/Ranganaths/minion/llm"
)

func main() {
    ctx := context.Background()

    // Create LLM provider
    provider := llm.NewOpenAI(os.Getenv("OPENAI_API_KEY"))

    // Create worker with LLM capability
    worker := multiagent.NewWorkerAgent(
        "llm-worker-1",
        []string{"text_analysis", "summarization"},
        protocol,
        ledger,
    )

    // Register LLM-powered task handler
    worker.RegisterHandler("text_analysis", func(task *multiagent.Task) (*multiagent.Result, error) {
        // Extract input text
        text := task.Input["text"].(string)

        // Call LLM
        resp, err := provider.GenerateCompletion(ctx, &llm.CompletionRequest{
            SystemPrompt: "Analyze the following text and provide insights.",
            UserPrompt:   text,
            Temperature:  0.7,
            MaxTokens:    500,
            Model:        "gpt-4",
        })

        if err != nil {
            return nil, err
        }

        return &multiagent.Result{
            Status: "success",
            Data: map[string]interface{}{
                "analysis":    resp.Text,
                "tokens_used": resp.TokensUsed,
            },
        }, nil
    })

    worker.Start(ctx)
}
```

---

## Adding New Providers

### Step 1: Implement the Provider Interface

Create a new file `llm/your_provider.go`:

```go
package llm

import (
    "context"
    // Import your provider's SDK
)

type YourProvider struct {
    client *YourSDKClient
    name   string
}

func NewYourProvider(apiKey string) *YourProvider {
    return &YourProvider{
        client: NewYourSDKClient(apiKey),
        name:   "your_provider",
    }
}

func (p *YourProvider) Name() string {
    return p.name
}

func (p *YourProvider) GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    // Implement completion logic
}

func (p *YourProvider) GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // Implement chat logic
}
```

### Step 2: Test Your Provider

```go
package llm_test

import (
    "context"
    "testing"

    "github.com/Ranganaths/minion/llm"
)

func TestYourProvider(t *testing.T) {
    provider := llm.NewYourProvider("test-key")

    resp, err := provider.GenerateCompletion(context.Background(), &llm.CompletionRequest{
        UserPrompt: "Hello",
        Model:      "model-name",
    })

    if err != nil {
        t.Fatal(err)
    }

    if resp.Text == "" {
        t.Error("Expected non-empty response")
    }
}
```

---

## Provider Implementations

### TupleLeap AI

**File: `llm/tupleleap.go`**

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/Ranganaths/minion/llm"
)

func main() {
    ctx := context.Background()

    // Create TupleLeap provider
    apiKey := os.Getenv("TUPLELEAP_API_KEY")
    provider := llm.NewTupleLeap(apiKey)

    // Or with custom base URL
    // provider := llm.NewTupleLeapWithBaseURL(apiKey, "https://custom.api.tupleleap.ai/v1")

    // Generate completion
    resp, err := provider.GenerateCompletion(ctx, &llm.CompletionRequest{
        SystemPrompt: "You are a helpful assistant.",
        UserPrompt:   "What is the capital of France?",
        Temperature:  0.7,
        MaxTokens:    100,
        Model:        "tupleleap-default", // Use your model name
    })

    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Response: %s\n", resp.Text)
    fmt.Printf("Tokens used: %d\n", resp.TokensUsed)
}
```

**Usage with Chat:**

```go
provider := llm.NewTupleLeap(os.Getenv("TUPLELEAP_API_KEY"))

resp, err := provider.GenerateChat(ctx, &llm.ChatRequest{
    Messages: []llm.Message{
        {Role: "system", Content: "You are an expert programmer."},
        {Role: "user", Content: "Explain what is a closure in JavaScript."},
    },
    Temperature: 0.7,
    MaxTokens:   500,
    Model:       "tupleleap-default",
})
```

**Environment Variables:**

```bash
# Required
export TUPLELEAP_API_KEY="your-api-key-here"

# Optional: Custom base URL (if using self-hosted or different endpoint)
export TUPLELEAP_BASE_URL="https://custom.api.tupleleap.ai/v1"
```

**Integration with Minion Workers:**

```go
// Create TupleLeap worker
worker := multiagent.NewWorkerAgent(
    "tupleleap-worker",
    []string{"tupleleap"},
    protocol,
    ledger,
)

// Register handler
worker.RegisterHandler("tupleleap", func(task *multiagent.Task) (*multiagent.Result, error) {
    provider := llm.NewTupleLeap(os.Getenv("TUPLELEAP_API_KEY"))

    resp, err := provider.GenerateCompletion(ctx, &llm.CompletionRequest{
        SystemPrompt: task.Input["system_prompt"].(string),
        UserPrompt:   task.Input["user_prompt"].(string),
        Temperature:  0.7,
        MaxTokens:    500,
        Model:        task.Input["model"].(string),
    })

    if err != nil {
        return nil, err
    }

    return &multiagent.Result{
        Status: "success",
        Data: map[string]interface{}{
            "text":        resp.Text,
            "tokens_used": resp.TokensUsed,
            "model":       resp.Model,
        },
    }, nil
})
```

**Supported Features:**
- ✅ Text completion
- ✅ Chat completion
- ✅ Custom base URL support
- ✅ Temperature control
- ✅ Max tokens configuration
- ✅ Token usage tracking

**API Compatibility:**
TupleLeap AI provider follows the OpenAI-compatible API format, making it easy to integrate with existing systems.

---

### Anthropic (Claude)

**File: `llm/anthropic.go`**

```go
package llm

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type AnthropicProvider struct {
    apiKey     string
    httpClient *http.Client
    baseURL    string
}

func NewAnthropic(apiKey string) *AnthropicProvider {
    return &AnthropicProvider{
        apiKey:     apiKey,
        httpClient: &http.Client{},
        baseURL:    "https://api.anthropic.com/v1",
    }
}

func (p *AnthropicProvider) Name() string {
    return "anthropic"
}

type anthropicMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type anthropicRequest struct {
    Model       string              `json:"model"`
    Messages    []anthropicMessage  `json:"messages"`
    MaxTokens   int                 `json:"max_tokens"`
    Temperature float64             `json:"temperature,omitempty"`
    System      string              `json:"system,omitempty"`
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
        return nil, fmt.Errorf("anthropic API error: %s", string(body))
    }

    var resp anthropicResponse
    if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
        return nil, err
    }

    return &resp, nil
}
```

**Usage:**

```go
provider := llm.NewAnthropic(os.Getenv("ANTHROPIC_API_KEY"))

resp, err := provider.GenerateCompletion(ctx, &llm.CompletionRequest{
    SystemPrompt: "You are a helpful assistant.",
    UserPrompt:   "Explain quantum computing.",
    Temperature:  0.7,
    MaxTokens:    1000,
    Model:        "claude-3-opus-20240229",
})
```

**Supported Models:**
- `claude-3-opus-20240229` (most capable)
- `claude-3-sonnet-20240229` (balanced)
- `claude-3-haiku-20240307` (fastest)
- `claude-2.1`
- `claude-2.0`

---

### Google Gemini

**File: `llm/gemini.go`**

```go
package llm

import (
    "context"
    "fmt"

    "google.golang.org/api/option"
    "google.golang.org/genai"
)

type GeminiProvider struct {
    client *genai.Client
    name   string
}

func NewGemini(apiKey string) (*GeminiProvider, error) {
    ctx := context.Background()
    client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
    if err != nil {
        return nil, err
    }

    return &GeminiProvider{
        client: client,
        name:   "gemini",
    }, nil
}

func (p *GeminiProvider) Name() string {
    return p.name
}

func (p *GeminiProvider) GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    model := p.client.GenerativeModel(req.Model)

    // Configure generation
    model.SetTemperature(float32(req.Temperature))
    model.SetMaxOutputTokens(int32(req.MaxTokens))

    // Combine system and user prompts
    prompt := req.SystemPrompt + "\n\n" + req.UserPrompt

    resp, err := model.GenerateContent(ctx, genai.Text(prompt))
    if err != nil {
        return nil, fmt.Errorf("gemini completion error: %w", err)
    }

    if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
        return nil, fmt.Errorf("no completion generated")
    }

    text := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])

    return &CompletionResponse{
        Text:         text,
        TokensUsed:   int(resp.UsageMetadata.TotalTokenCount),
        FinishReason: string(resp.Candidates[0].FinishReason),
        Model:        req.Model,
    }, nil
}

func (p *GeminiProvider) GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    model := p.client.GenerativeModel(req.Model)

    model.SetTemperature(float32(req.Temperature))
    model.SetMaxOutputTokens(int32(req.MaxTokens))

    // Start chat session
    chat := model.StartChat()

    // Add message history
    for i, msg := range req.Messages {
        if i < len(req.Messages)-1 {
            // Add historical messages
            if msg.Role == "user" {
                chat.History = append(chat.History, &genai.Content{
                    Parts: []genai.Part{genai.Text(msg.Content)},
                    Role:  "user",
                })
            } else if msg.Role == "assistant" {
                chat.History = append(chat.History, &genai.Content{
                    Parts: []genai.Part{genai.Text(msg.Content)},
                    Role:  "model",
                })
            }
        }
    }

    // Send last message
    lastMsg := req.Messages[len(req.Messages)-1]
    resp, err := chat.SendMessage(ctx, genai.Text(lastMsg.Content))
    if err != nil {
        return nil, err
    }

    if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
        return nil, fmt.Errorf("no response generated")
    }

    text := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])

    return &ChatResponse{
        Message: Message{
            Role:    "assistant",
            Content: text,
        },
        TokensUsed:   int(resp.UsageMetadata.TotalTokenCount),
        FinishReason: string(resp.Candidates[0].FinishReason),
        Model:        req.Model,
    }, nil
}

func (p *GeminiProvider) Close() error {
    return p.client.Close()
}
```

**Installation:**

```bash
go get google.golang.org/genai
```

**Usage:**

```go
provider, err := llm.NewGemini(os.Getenv("GEMINI_API_KEY"))
if err != nil {
    log.Fatal(err)
}
defer provider.Close()

resp, err := provider.GenerateCompletion(ctx, &llm.CompletionRequest{
    SystemPrompt: "You are an expert programmer.",
    UserPrompt:   "Explain dependency injection.",
    Temperature:  0.7,
    MaxTokens:    1000,
    Model:        "gemini-pro",
})
```

**Supported Models:**
- `gemini-pro` (general purpose)
- `gemini-pro-vision` (multimodal)
- `gemini-ultra` (most capable, limited access)

---

### Ollama (Local Models)

**File: `llm/ollama.go`**

```go
package llm

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type OllamaProvider struct {
    httpClient *http.Client
    baseURL    string
    name       string
}

func NewOllama(baseURL string) *OllamaProvider {
    if baseURL == "" {
        baseURL = "http://localhost:11434"
    }

    return &OllamaProvider{
        httpClient: &http.Client{},
        baseURL:    baseURL,
        name:       "ollama",
    }
}

func (p *OllamaProvider) Name() string {
    return p.name
}

type ollamaRequest struct {
    Model    string `json:"model"`
    Prompt   string `json:"prompt,omitempty"`
    Messages []struct {
        Role    string `json:"role"`
        Content string `json:"content"`
    } `json:"messages,omitempty"`
    Stream  bool                   `json:"stream"`
    Options map[string]interface{} `json:"options,omitempty"`
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

func (p *OllamaProvider) GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    // Combine prompts
    prompt := req.SystemPrompt + "\n\n" + req.UserPrompt

    ollamaReq := ollamaRequest{
        Model:  req.Model,
        Prompt: prompt,
        Stream: false,
        Options: map[string]interface{}{
            "temperature": req.Temperature,
            "num_predict": req.MaxTokens,
        },
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
        return nil, fmt.Errorf("ollama API error: %s", string(body))
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

func (p *OllamaProvider) GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    ollamaReq := ollamaRequest{
        Model:  req.Model,
        Stream: false,
        Options: map[string]interface{}{
            "temperature": req.Temperature,
            "num_predict": req.MaxTokens,
        },
    }

    // Convert messages
    ollamaReq.Messages = make([]struct {
        Role    string `json:"role"`
        Content string `json:"content"`
    }, len(req.Messages))

    for i, msg := range req.Messages {
        ollamaReq.Messages[i].Role = msg.Role
        ollamaReq.Messages[i].Content = msg.Content
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
        return nil, fmt.Errorf("ollama API error: %s", string(body))
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
        TokensUsed:   0,
        FinishReason: "stop",
        Model:        resp.Model,
    }, nil
}
```

**Installation:**

```bash
# Install Ollama
curl https://ollama.ai/install.sh | sh

# Pull a model
ollama pull llama2
ollama pull mistral
ollama pull codellama
```

**Usage:**

```go
// Connect to local Ollama instance
provider := llm.NewOllama("http://localhost:11434")

resp, err := provider.GenerateCompletion(ctx, &llm.CompletionRequest{
    SystemPrompt: "You are a coding assistant.",
    UserPrompt:   "Write a function to reverse a string.",
    Temperature:  0.7,
    MaxTokens:    500,
    Model:        "codellama",
})
```

**Popular Models:**
- `llama2` - General purpose
- `llama2:13b` - Larger Llama 2
- `mistral` - Fast and capable
- `codellama` - Code generation
- `phi` - Efficient small model
- `neural-chat` - Conversational

---

### Azure OpenAI

**File: `llm/azure_openai.go`**

```go
package llm

import (
    "context"
    "fmt"

    "github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
    "github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

type AzureOpenAIProvider struct {
    client   *azopenai.Client
    endpoint string
    name     string
}

func NewAzureOpenAI(endpoint, apiKey string) (*AzureOpenAIProvider, error) {
    keyCredential := azcore.NewKeyCredential(apiKey)

    client, err := azopenai.NewClientWithKeyCredential(endpoint, keyCredential, nil)
    if err != nil {
        return nil, err
    }

    return &AzureOpenAIProvider{
        client:   client,
        endpoint: endpoint,
        name:     "azure_openai",
    }, nil
}

func (p *AzureOpenAIProvider) Name() string {
    return p.name
}

func (p *AzureOpenAIProvider) GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    messages := []azopenai.ChatRequestMessageClassification{
        &azopenai.ChatRequestSystemMessage{
            Content: azopenai.NewChatRequestSystemMessageContent(req.SystemPrompt),
        },
        &azopenai.ChatRequestUserMessage{
            Content: azopenai.NewChatRequestUserMessageContent(req.UserPrompt),
        },
    }

    temperature := float32(req.Temperature)
    maxTokens := int32(req.MaxTokens)

    resp, err := p.client.GetChatCompletions(ctx, azopenai.ChatCompletionsOptions{
        Messages:       messages,
        DeploymentName: &req.Model,
        Temperature:    &temperature,
        MaxTokens:      &maxTokens,
    }, nil)

    if err != nil {
        return nil, fmt.Errorf("azure openai error: %w", err)
    }

    if len(resp.Choices) == 0 {
        return nil, fmt.Errorf("no completions returned")
    }

    choice := resp.Choices[0]
    content := ""
    if choice.Message.Content != nil {
        content = *choice.Message.Content
    }

    return &CompletionResponse{
        Text:         content,
        TokensUsed:   int(*resp.Usage.TotalTokens),
        FinishReason: string(*choice.FinishReason),
        Model:        req.Model,
    }, nil
}

func (p *AzureOpenAIProvider) GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    messages := make([]azopenai.ChatRequestMessageClassification, len(req.Messages))

    for i, msg := range req.Messages {
        switch msg.Role {
        case "system":
            messages[i] = &azopenai.ChatRequestSystemMessage{
                Content: azopenai.NewChatRequestSystemMessageContent(msg.Content),
            }
        case "user":
            messages[i] = &azopenai.ChatRequestUserMessage{
                Content: azopenai.NewChatRequestUserMessageContent(msg.Content),
            }
        case "assistant":
            messages[i] = &azopenai.ChatRequestAssistantMessage{
                Content: azopenai.NewChatRequestAssistantMessageContent(msg.Content),
            }
        }
    }

    temperature := float32(req.Temperature)
    maxTokens := int32(req.MaxTokens)

    resp, err := p.client.GetChatCompletions(ctx, azopenai.ChatCompletionsOptions{
        Messages:       messages,
        DeploymentName: &req.Model,
        Temperature:    &temperature,
        MaxTokens:      &maxTokens,
    }, nil)

    if err != nil {
        return nil, err
    }

    if len(resp.Choices) == 0 {
        return nil, fmt.Errorf("no chat completions returned")
    }

    choice := resp.Choices[0]
    content := ""
    if choice.Message.Content != nil {
        content = *choice.Message.Content
    }

    return &ChatResponse{
        Message: Message{
            Role:    string(*choice.Message.Role),
            Content: content,
        },
        TokensUsed:   int(*resp.Usage.TotalTokens),
        FinishReason: string(*choice.FinishReason),
        Model:        req.Model,
    }, nil
}
```

**Installation:**

```bash
go get github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai
```

**Usage:**

```go
endpoint := "https://your-resource.openai.azure.com/"
apiKey := os.Getenv("AZURE_OPENAI_KEY")

provider, err := llm.NewAzureOpenAI(endpoint, apiKey)
if err != nil {
    log.Fatal(err)
}

// Use your deployment name as the model
resp, err := provider.GenerateCompletion(ctx, &llm.CompletionRequest{
    SystemPrompt: "You are a helpful assistant.",
    UserPrompt:   "Hello!",
    Temperature:  0.7,
    MaxTokens:    100,
    Model:        "your-gpt4-deployment", // Your deployment name
})
```

---

## Best Practices

### 1. Provider Selection Strategy

```go
type MultiProviderClient struct {
    providers map[string]llm.Provider
}

func (m *MultiProviderClient) SelectProvider(task *Task) llm.Provider {
    switch task.Requirements {
    case "fast":
        return m.providers["ollama"] // Local, instant
    case "smart":
        return m.providers["openai"] // GPT-4
    case "cost":
        return m.providers["anthropic"] // Claude Haiku
    default:
        return m.providers["openai"]
    }
}
```

### 2. Fallback Chain

```go
func GenerateWithFallback(ctx context.Context, providers []llm.Provider, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
    var lastErr error

    for _, provider := range providers {
        resp, err := provider.GenerateCompletion(ctx, req)
        if err == nil {
            return resp, nil
        }
        lastErr = err
        log.Printf("Provider %s failed: %v, trying next...", provider.Name(), err)
    }

    return nil, fmt.Errorf("all providers failed: %w", lastErr)
}

// Usage
providers := []llm.Provider{
    llm.NewOpenAI(openaiKey),
    llm.NewAnthropic(anthropicKey),
    llm.NewOllama(""),
}

resp, err := GenerateWithFallback(ctx, providers, req)
```

### 3. Caching

```go
type CachedProvider struct {
    provider llm.Provider
    cache    map[string]*llm.CompletionResponse
    mu       sync.RWMutex
}

func (c *CachedProvider) GenerateCompletion(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
    // Create cache key
    key := fmt.Sprintf("%s:%s:%s", req.Model, req.SystemPrompt, req.UserPrompt)

    // Check cache
    c.mu.RLock()
    if cached, ok := c.cache[key]; ok {
        c.mu.RUnlock()
        return cached, nil
    }
    c.mu.RUnlock()

    // Call provider
    resp, err := c.provider.GenerateCompletion(ctx, req)
    if err != nil {
        return nil, err
    }

    // Cache result
    c.mu.Lock()
    c.cache[key] = resp
    c.mu.Unlock()

    return resp, nil
}
```

### 4. Rate Limiting

```go
import "golang.org/x/time/rate"

type RateLimitedProvider struct {
    provider llm.Provider
    limiter  *rate.Limiter
}

func NewRateLimitedProvider(provider llm.Provider, requestsPerMinute int) *RateLimitedProvider {
    return &RateLimitedProvider{
        provider: provider,
        limiter:  rate.NewLimiter(rate.Limit(requestsPerMinute)/60.0, 1),
    }
}

func (r *RateLimitedProvider) GenerateCompletion(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
    // Wait for rate limit
    if err := r.limiter.Wait(ctx); err != nil {
        return nil, err
    }

    return r.provider.GenerateCompletion(ctx, req)
}
```

### 5. Cost Tracking

```go
type CostTrackingProvider struct {
    provider   llm.Provider
    totalCost  float64
    totalTokens int
    mu         sync.Mutex
}

func (c *CostTrackingProvider) GenerateCompletion(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
    resp, err := c.provider.GenerateCompletion(ctx, req)
    if err != nil {
        return nil, err
    }

    // Calculate cost (example: GPT-4 pricing)
    var costPerToken float64
    switch req.Model {
    case "gpt-4":
        costPerToken = 0.03 / 1000 // $0.03 per 1K tokens
    case "gpt-3.5-turbo":
        costPerToken = 0.002 / 1000 // $0.002 per 1K tokens
    default:
        costPerToken = 0.01 / 1000
    }

    cost := float64(resp.TokensUsed) * costPerToken

    c.mu.Lock()
    c.totalCost += cost
    c.totalTokens += resp.TokensUsed
    c.mu.Unlock()

    return resp, nil
}

func (c *CostTrackingProvider) GetStats() (totalCost float64, totalTokens int) {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.totalCost, c.totalTokens
}
```

---

## Roadmap

### Completed ✅
- [x] OpenAI support (GPT-4, GPT-3.5-turbo)
- [x] TupleLeap AI support
- [x] Anthropic (Claude) support
- [x] Ollama (local models) support

### In Progress
- [ ] Google Gemini - Implementation ready
- [ ] Azure OpenAI - Implementation ready
- [ ] Streaming responses
- [ ] Function calling support

### Planned
- [ ] Cohere support
- [ ] Hugging Face Inference API
- [ ] Embeddings support
- [ ] AWS Bedrock
- [ ] Together AI
- [ ] Replicate
- [ ] Custom model hosting support
- [ ] Multi-modal support (vision, audio)

---

## Contributing

### Adding a New Provider

1. Create `llm/your_provider.go`
2. Implement the `Provider` interface
3. Add tests in `llm/your_provider_test.go`
4. Update this documentation
5. Submit PR

### Testing

```bash
# Run all provider tests
go test ./llm/...

# Test specific provider
go test ./llm -run TestOpenAI

# With coverage
go test ./llm -cover
```

---

## Examples

### Multi-Provider Worker

```go
package main

import (
    "context"
    "os"

    "github.com/Ranganaths/minion/core/multiagent"
    "github.com/Ranganaths/minion/llm"
)

func main() {
    ctx := context.Background()

    // Create multiple providers
    providers := map[string]llm.Provider{
        "openai":    llm.NewOpenAI(os.Getenv("OPENAI_API_KEY")),
        "anthropic": llm.NewAnthropic(os.Getenv("ANTHROPIC_API_KEY")),
        "ollama":    llm.NewOllama(""),
    }

    // Create worker
    worker := multiagent.NewWorkerAgent("llm-worker", []string{"ai"}, protocol, ledger)

    // Register handler that uses multiple providers
    worker.RegisterHandler("ai", func(task *multiagent.Task) (*multiagent.Result, error) {
        providerName := task.Input["provider"].(string)
        provider := providers[providerName]

        resp, err := provider.GenerateCompletion(ctx, &llm.CompletionRequest{
            SystemPrompt: task.Input["system"].(string),
            UserPrompt:   task.Input["prompt"].(string),
            Temperature:  0.7,
            MaxTokens:    1000,
            Model:        task.Input["model"].(string),
        })

        if err != nil {
            return nil, err
        }

        return &multiagent.Result{
            Status: "success",
            Data: map[string]interface{}{
                "text":   resp.Text,
                "tokens": resp.TokensUsed,
                "model":  resp.Model,
            },
        }, nil
    })

    worker.Start(ctx)
}
```

---

## Summary

**Current Status:**
- ✅ **4 providers fully supported**: OpenAI, TupleLeap, Anthropic, Ollama
- 🔨 **2 providers ready to implement**: Google Gemini, Azure OpenAI
- 📋 **6+ more providers planned**

**Philosophy:**
- **Clean interface** - Easy to add providers
- **Flexible** - Use any provider or mix multiple
- **Production-ready** - Rate limiting, fallbacks, caching

**Quick Start:**
```go
// Choose your provider
provider := llm.NewOpenAI(os.Getenv("OPENAI_API_KEY"))
// or
provider := llm.NewAnthropic(os.Getenv("ANTHROPIC_API_KEY"))
// or
provider := llm.NewTupleLeap(os.Getenv("TUPLELEAP_API_KEY"))
// or
provider := llm.NewOllama("http://localhost:11434")

// Use with Minion framework
framework := core.NewFramework(
    core.WithLLMProvider(provider),
    core.WithStorage(storage.NewInMemory()),
)
```

For questions or contributions, see [CONTRIBUTING.md](CONTRIBUTING.md)
