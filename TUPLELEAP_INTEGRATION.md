# TupleLeap AI Integration - Complete

**Status**: ✅ Fully Integrated
**Date**: December 16, 2025

---

## Summary

TupleLeap AI has been successfully integrated as a first-class LLM provider in the Minion framework, joining OpenAI, Anthropic, and Ollama.

---

## What Was Added

### 1. Core Provider Implementation ✅

**File**: `llm/tupleleap.go` (250 lines)

```go
// Full implementation with:
- Text completion support
- Chat completion support
- Custom base URL support
- OpenAI-compatible API format
- Token usage tracking
- Error handling
```

**Key Features**:
- ✅ `GenerateCompletion()` - Text generation
- ✅ `GenerateChat()` - Multi-turn conversations
- ✅ `NewTupleLeap()` - Default endpoint
- ✅ `NewTupleLeapWithBaseURL()` - Custom endpoint
- ✅ `SetHTTPClient()` - Custom HTTP client
- ✅ `SetBaseURL()` - Runtime URL changes

### 2. Factory Integration ✅

**File**: `llm/factory.go` (Updated)

```go
// Added TupleLeap to factory system:
const (
    ProviderTypeTupleLeap ProviderType = "tupleleap"
)

// Auto-detection from environment:
CreateDefaultProviders() {
    // Includes TupleLeap if TUPLELEAP_API_KEY is set
}
```

**Environment Variables**:
- `TUPLELEAP_API_KEY` - API authentication
- `TUPLELEAP_BASE_URL` - Optional custom endpoint
- `LLM_PROVIDER=tupleleap` - Set as default provider

### 3. Documentation ✅

**File**: `LLM_PROVIDERS.md` (Updated)

Added comprehensive TupleLeap section with:
- ✅ Usage examples
- ✅ Environment variables
- ✅ Minion worker integration
- ✅ API compatibility notes
- ✅ Supported features list

### 4. Complete Example ✅

**Files**:
- `examples/tupleleap_example/main.go` (200 lines)
- `examples/tupleleap_example/README.md` (400 lines)

**Example Includes**:
- Direct TupleLeap usage
- Chat conversations
- Minion worker integration
- Multi-task orchestration
- Error handling
- Best practices

---

## Usage

### Quick Start

```bash
# Set API key
export TUPLELEAP_API_KEY="your-api-key-here"

# Run example
cd examples/tupleleap_example
go run main.go
```

### Code Example

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/yourusername/minion/llm"
)

func main() {
    ctx := context.Background()

    // Create provider
    provider := llm.NewTupleLeap(os.Getenv("TUPLELEAP_API_KEY"))

    // Generate completion
    resp, err := provider.GenerateCompletion(ctx, &llm.CompletionRequest{
        SystemPrompt: "You are a helpful assistant.",
        UserPrompt:   "What is Go programming?",
        Temperature:  0.7,
        MaxTokens:    200,
        Model:        "tupleleap-default",
    })

    if err != nil {
        panic(err)
    }

    fmt.Printf("Response: %s\n", resp.Text)
    fmt.Printf("Tokens: %d\n", resp.TokensUsed)
}
```

### With Minion Workers

```go
// Create worker
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
        Model:        task.Input["model"].(string),
    })

    return &multiagent.Result{
        Status: "success",
        Data: map[string]interface{}{
            "text":        resp.Text,
            "tokens_used": resp.TokensUsed,
        },
    }, nil
})
```

### Multi-Provider Support

```go
// Auto-detect all available providers
factory := llm.CreateDefaultProviders()

// Lists: openai, anthropic, tupleleap, ollama (based on env vars)
providers := factory.ListProviders()

// Get specific provider
tupleLeap, _ := factory.GetProvider("tupleleap")
openai, _ := factory.GetProvider("openai")

// Route based on requirements
if task.Priority == PriorityHigh {
    provider = openai // Use GPT-4
} else {
    provider = tupleLeap // Use TupleLeap
}
```

---

## API Compatibility

TupleLeap provider follows the **OpenAI-compatible API format**:

### Endpoints

```
POST /v1/completions       - Text completion
POST /v1/chat/completions  - Chat completion
```

### Request Format

```json
{
  "model": "tupleleap-default",
  "prompt": "Hello world",
  "temperature": 0.7,
  "max_tokens": 100
}
```

### Response Format

```json
{
  "id": "cmpl-123",
  "object": "text_completion",
  "model": "tupleleap-default",
  "choices": [{
    "text": "Hello! How can I help you?",
    "finish_reason": "stop"
  }],
  "usage": {
    "total_tokens": 45
  }
}
```

---

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TUPLELEAP_API_KEY` | ✅ Yes | - | API authentication key |
| `TUPLELEAP_BASE_URL` | ❌ No | `https://api.tupleleap.ai/v1` | Custom endpoint |
| `LLM_PROVIDER` | ❌ No | `openai` | Set to `tupleleap` for default |

### Provider Configuration

```go
// Default configuration
provider := llm.NewTupleLeap(apiKey)

// Custom endpoint
provider := llm.NewTupleLeapWithBaseURL(apiKey, "https://custom.endpoint/v1")

// Runtime changes
provider.SetBaseURL("https://new.endpoint/v1")
provider.SetHTTPClient(customClient)
```

---

## Features

### Supported ✅

- ✅ Text completion (`GenerateCompletion`)
- ✅ Chat completion (`GenerateChat`)
- ✅ System prompts
- ✅ Temperature control
- ✅ Max tokens configuration
- ✅ Token usage tracking
- ✅ Custom base URL
- ✅ Custom HTTP client
- ✅ Error handling
- ✅ Context support (timeouts, cancellation)

### Not Yet Supported ⏳

- ⏳ Streaming responses
- ⏳ Function calling
- ⏳ Embeddings
- ⏳ Fine-tuning

---

## File Structure

```
minion/
├── llm/
│   ├── interface.go          # Provider interface
│   ├── openai.go              # OpenAI provider
│   ├── anthropic.go           # Anthropic provider
│   ├── ollama.go              # Ollama provider
│   ├── tupleleap.go           # ✅ TupleLeap provider (NEW)
│   └── factory.go             # ✅ Updated with TupleLeap
├── examples/
│   ├── llm_worker/
│   │   └── main.go
│   └── tupleleap_example/     # ✅ NEW
│       ├── main.go            # ✅ Complete example
│       └── README.md          # ✅ Documentation
├── LLM_PROVIDERS.md           # ✅ Updated with TupleLeap
└── TUPLELEAP_INTEGRATION.md   # ✅ This document
```

---

## Testing

### Unit Testing

```bash
# Test TupleLeap provider
go test ./llm -run TestTupleLeap

# Test all providers
go test ./llm/...
```

### Integration Testing

```bash
# Set API key
export TUPLELEAP_API_KEY="test-key"

# Run integration test
cd examples/tupleleap_example
go run main.go
```

### Expected Output

```
=== Example 1: Direct TupleLeap Usage ===

1. Simple Completion:
Response: Go offers excellent concurrency support...
Tokens: 145
Model: tupleleap-default

2. Chat Conversation:
Response: Microservices architecture provides...
Tokens: 234

=== Example 2: TupleLeap with Minion Workers ===

Started TupleLeap worker

Executing task: Code Review
Status: success
Response: This is a clean implementation...
Tokens: 87
```

---

## Provider Comparison

| Feature | OpenAI | Anthropic | TupleLeap | Ollama |
|---------|--------|-----------|-----------|--------|
| **Status** | ✅ Built-in | ✅ Ready | ✅ **Ready** | ✅ Ready |
| **Location** | `openai.go` | `anthropic.go` | `tupleleap.go` | `ollama.go` |
| **Cost** | $$$ | $$ | $$ | Free (local) |
| **Speed** | Fast | Fast | Fast | Very Fast |
| **Quality** | Excellent | Excellent | Good | Good |
| **Custom URL** | ❌ | ❌ | ✅ | ✅ |
| **Local** | ❌ | ❌ | ❌ | ✅ |

---

## Best Practices

### 1. Environment Variables

```bash
# Always use environment variables
export TUPLELEAP_API_KEY="your-key"

# Never hardcode in source
```

### 2. Error Handling

```go
resp, err := provider.GenerateCompletion(ctx, req)
if err != nil {
    if strings.Contains(err.Error(), "401") {
        log.Println("Invalid API key")
    } else if strings.Contains(err.Error(), "429") {
        log.Println("Rate limit exceeded - implement backoff")
    }
}
```

### 3. Timeouts

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

resp, err := provider.GenerateCompletion(ctx, req)
```

### 4. Fallback Chain

```go
providers := []llm.Provider{
    llm.NewTupleLeap(key1),
    llm.NewOpenAI(key2),
    llm.NewOllama(""), // Local fallback
}

for _, provider := range providers {
    resp, err := provider.GenerateCompletion(ctx, req)
    if err == nil {
        return resp, nil
    }
}
```

---

## Production Deployment

### Docker

```dockerfile
FROM golang:1.21-alpine
WORKDIR /app
COPY . .
RUN go build -o worker main.go

ENV TUPLELEAP_API_KEY=""
CMD ["./worker"]
```

### Docker Compose

```yaml
services:
  tupleleap-worker:
    build: .
    environment:
      - TUPLELEAP_API_KEY=${TUPLELEAP_API_KEY}
      - TUPLELEAP_BASE_URL=${TUPLELEAP_BASE_URL}
    deploy:
      replicas: 3
```

### Kubernetes

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: tupleleap-secret
type: Opaque
stringData:
  api-key: your-api-key
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tupleleap-worker
spec:
  replicas: 5
  template:
    spec:
      containers:
      - name: worker
        env:
        - name: TUPLELEAP_API_KEY
          valueFrom:
            secretKeyRef:
              name: tupleleap-secret
              key: api-key
```

---

## Troubleshooting

### Issue 1: API Key Not Set

```
Error: TUPLELEAP_API_KEY not set
```

**Solution**:
```bash
export TUPLELEAP_API_KEY="your-api-key"
```

### Issue 2: Connection Refused

```
Error: dial tcp: connection refused
```

**Solution**: Check `TUPLELEAP_BASE_URL` or network connectivity.

### Issue 3: Rate Limit

```
Error: status 429: Rate limit exceeded
```

**Solution**: Implement rate limiting:
```go
limiter := rate.NewLimiter(10, 1) // 10 req/sec
limiter.Wait(ctx)
```

---

## Next Steps

1. **Try the Example**
   ```bash
   cd examples/tupleleap_example
   export TUPLELEAP_API_KEY="your-key"
   go run main.go
   ```

2. **Integrate with Your Workers**
   ```go
   provider := llm.NewTupleLeap(os.Getenv("TUPLELEAP_API_KEY"))
   // Use in your handlers
   ```

3. **Combine with Other Providers**
   ```go
   factory := llm.CreateDefaultProviders()
   // Automatic multi-provider support
   ```

4. **Deploy to Production**
   - See Docker/Kubernetes examples above
   - Configure environment variables
   - Monitor usage and costs

---

## Support

**Minion Framework**:
- Documentation: [LLM_PROVIDERS.md](LLM_PROVIDERS.md)
- Examples: [/examples](examples/)
- Tutorials: [TUTORIALS.md](TUTORIALS.md)

**TupleLeap AI**:
- Website: https://tupleleap.ai
- Documentation: https://docs.tupleleap.ai
- Support: support@tupleleap.ai

---

## Changelog

**v1.0.0** - December 16, 2025
- ✅ Initial TupleLeap integration
- ✅ Complete provider implementation
- ✅ Factory integration
- ✅ Documentation
- ✅ Examples and tutorials
- ✅ Production-ready

---

**Integration Status**: ✅ **COMPLETE**

TupleLeap AI is now a fully supported, production-ready provider in the Minion framework!
