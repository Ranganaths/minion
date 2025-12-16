# TupleLeap AI Provider Example

This example demonstrates how to use TupleLeap AI with the Minion framework.

## Overview

TupleLeap AI is integrated as a first-class provider in Minion, supporting both direct usage and multi-agent orchestration.

## Prerequisites

### TupleLeap AI API Key

You need a TupleLeap AI API key to run this example.

```bash
export TUPLELEAP_API_KEY="your-api-key-here"
```

### Optional: Custom Base URL

If you're using a custom or self-hosted TupleLeap instance:

```bash
export TUPLELEAP_BASE_URL="https://custom.api.tupleleap.ai/v1"
```

## Running the Example

### Basic Usage

```bash
cd examples/tupleleap_example
go run main.go
```

### Expected Output

```
=== Example 1: Direct TupleLeap Usage ===

1. Simple Completion:
Response: Go offers several benefits for backend development including strong concurrency support...
Tokens: 145
Model: tupleleap-default

2. Chat Conversation:
Response: The main advantages of microservices include independent deployment...
Tokens: 234

=== Example 2: TupleLeap with Minion Workers ===

Started TupleLeap worker

Executing task: Code Review
Status: success
Response: This is a clean and simple function implementation...
Tokens: 87

Executing task: Documentation
Status: success
Response: # REST API Project

A Go-based REST API service...
Tokens: 156
```

## Features Demonstrated

### 1. Direct TupleLeap Usage

```go
provider := llm.NewTupleLeap(apiKey)

resp, err := provider.GenerateCompletion(ctx, &llm.CompletionRequest{
    SystemPrompt: "You are a helpful AI assistant.",
    UserPrompt:   "What are the benefits of using Go?",
    Temperature:  0.7,
    MaxTokens:    200,
    Model:        "tupleleap-default",
})
```

### 2. Chat Conversations

```go
chatResp, err := provider.GenerateChat(ctx, &llm.ChatRequest{
    Messages: []llm.Message{
        {Role: "system", Content: "You are an expert."},
        {Role: "user", Content: "What is microservices?"},
        {Role: "assistant", Content: "Microservices is..."},
        {Role: "user", Content: "What are the advantages?"},
    },
    Temperature: 0.7,
    MaxTokens:   300,
    Model:       "tupleleap-default",
})
```

### 3. Minion Worker Integration

```go
// Create TupleLeap-powered worker
worker := multiagent.NewWorkerAgent(
    "tupleleap-worker",
    []string{"tupleleap"},
    protocol,
    ledger,
)

// Register handler
worker.RegisterHandler("tupleleap", func(task *multiagent.Task) (*multiagent.Result, error) {
    provider := llm.NewTupleLeap(apiKey)
    resp, err := provider.GenerateCompletion(ctx, req)
    // ...
})
```

### 4. Task Orchestration

```go
task := &multiagent.Task{
    Type: "tupleleap",
    Input: map[string]interface{}{
        "system_prompt": "You are an expert code reviewer.",
        "user_prompt":   "Review this code...",
        "model":         "tupleleap-default",
        "temperature":   0.7,
        "max_tokens":    300,
    },
}

result, err := orchestrator.ExecuteTask(ctx, task)
```

## Configuration Options

### Provider Configuration

```go
// Default endpoint
provider := llm.NewTupleLeap(apiKey)

// Custom endpoint
provider := llm.NewTupleLeapWithBaseURL(apiKey, "https://custom.api.tupleleap.ai/v1")

// Modify after creation
provider.SetBaseURL("https://another.endpoint/v1")
provider.SetHTTPClient(customHTTPClient)
```

### Request Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `SystemPrompt` | string | System instruction for the model | "" |
| `UserPrompt` | string | User's input text | Required |
| `Temperature` | float64 | Randomness (0.0-1.0) | 0.7 |
| `MaxTokens` | int | Maximum tokens to generate | 100 |
| `Model` | string | Model identifier | Required |

## Use Cases

### Code Review

```go
task := &multiagent.Task{
    Type: "tupleleap",
    Input: map[string]interface{}{
        "system_prompt": "You are an expert code reviewer.",
        "user_prompt": `Review this code:
func ProcessData(data []byte) error {
    // implementation
}`,
        "model": "tupleleap-default",
    },
}
```

### Documentation Generation

```go
task := &multiagent.Task{
    Type: "tupleleap",
    Input: map[string]interface{}{
        "system_prompt": "You are a technical writer.",
        "user_prompt":   "Generate API documentation for this endpoint...",
        "model":         "tupleleap-default",
    },
}
```

### Natural Language Processing

```go
task := &multiagent.Task{
    Type: "tupleleap",
    Input: map[string]interface{}{
        "system_prompt": "You are a sentiment analyzer.",
        "user_prompt":   "Analyze sentiment: This product is amazing!",
        "model":         "tupleleap-default",
    },
}
```

## Multi-Provider Setup

Combine TupleLeap with other providers:

```go
factory := llm.CreateDefaultProviders()
// Auto-includes TupleLeap if TUPLELEAP_API_KEY is set

// Get specific provider
tupleLeapProvider, _ := factory.GetProvider("tupleleap")
openaiProvider, _ := factory.GetProvider("openai")

// Use based on requirements
if task.RequiresFastResponse {
    provider = tupleLeapProvider
} else if task.RequiresHighQuality {
    provider = openaiProvider
}
```

## Error Handling

```go
resp, err := provider.GenerateCompletion(ctx, req)
if err != nil {
    // Handle different error types
    switch {
    case strings.Contains(err.Error(), "status 401"):
        log.Println("Invalid API key")
    case strings.Contains(err.Error(), "status 429"):
        log.Println("Rate limit exceeded")
    case strings.Contains(err.Error(), "status 500"):
        log.Println("Server error")
    default:
        log.Printf("Unknown error: %v", err)
    }
}
```

## Best Practices

### 1. Environment Variables

```bash
# Always use environment variables for API keys
export TUPLELEAP_API_KEY="your-key"

# Never hardcode in source
# ❌ provider := llm.NewTupleLeap("hardcoded-key")
# ✅ provider := llm.NewTupleLeap(os.Getenv("TUPLELEAP_API_KEY"))
```

### 2. Rate Limiting

```go
import "golang.org/x/time/rate"

limiter := rate.NewLimiter(10, 1) // 10 requests per second

func makeRequest(ctx context.Context) {
    if err := limiter.Wait(ctx); err != nil {
        return
    }
    provider.GenerateCompletion(ctx, req)
}
```

### 3. Caching

```go
type CachedProvider struct {
    provider llm.Provider
    cache    map[string]*llm.CompletionResponse
}

func (cp *CachedProvider) GenerateCompletion(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
    key := fmt.Sprintf("%s:%s", req.SystemPrompt, req.UserPrompt)

    if cached, ok := cp.cache[key]; ok {
        return cached, nil
    }

    resp, err := cp.provider.GenerateCompletion(ctx, req)
    if err == nil {
        cp.cache[key] = resp
    }

    return resp, err
}
```

### 4. Timeout Handling

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

resp, err := provider.GenerateCompletion(ctx, req)
if err == context.DeadlineExceeded {
    log.Println("Request timed out after 30 seconds")
}
```

## Troubleshooting

### API Key Not Set

```
Error: TUPLELEAP_API_KEY not set
```

**Solution:**
```bash
export TUPLELEAP_API_KEY="your-api-key-here"
```

### Invalid API Key

```
Error: tupleleap API error (status 401): Unauthorized
```

**Solution:** Check that your API key is correct and active.

### Rate Limit Exceeded

```
Error: tupleleap API error (status 429): Rate limit exceeded
```

**Solution:** Implement rate limiting or reduce request frequency.

### Connection Refused

```
Error: request failed: dial tcp: connection refused
```

**Solution:** Check if custom base URL is correct or if TupleLeap service is accessible.

## Production Deployment

### Docker

```dockerfile
FROM golang:1.21-alpine

WORKDIR /app

COPY . .
RUN go build -o tupleleap-worker main.go

ENV TUPLELEAP_API_KEY=""

CMD ["./tupleleap-worker"]
```

### Docker Compose

```yaml
services:
  tupleleap-worker:
    build: .
    environment:
      - TUPLELEAP_API_KEY=${TUPLELEAP_API_KEY}
      - TUPLELEAP_BASE_URL=${TUPLELEAP_BASE_URL:-https://api.tupleleap.ai/v1}
```

### Kubernetes

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: tupleleap-secret
type: Opaque
stringData:
  api-key: your-api-key-here
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tupleleap-worker
spec:
  template:
    spec:
      containers:
      - name: worker
        image: tupleleap-worker:latest
        env:
        - name: TUPLELEAP_API_KEY
          valueFrom:
            secretKeyRef:
              name: tupleleap-secret
              key: api-key
```

## Next Steps

- Explore other providers: [LLM_PROVIDERS.md](../../LLM_PROVIDERS.md)
- Learn about multi-provider setups
- Implement custom task handlers
- Build production workflows

## Support

For TupleLeap AI specific questions:
- Visit: https://tupleleap.ai
- Documentation: https://docs.tupleleap.ai

For Minion framework questions:
- See: [TUTORIALS.md](../../TUTORIALS.md)
- Examples: [/examples](../)
