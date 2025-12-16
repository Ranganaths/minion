# LLM Worker Example

This example demonstrates how to use Minion with multiple LLM providers (OpenAI, Anthropic, Ollama).

## Features

- Multiple LLM provider support
- Provider-agnostic worker design
- Parallel execution across providers
- Result comparison

## Prerequisites

### 1. OpenAI (Optional)

```bash
export OPENAI_API_KEY="sk-..."
```

Get API key from: https://platform.openai.com/api-keys

### 2. Anthropic (Optional)

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
```

Get API key from: https://console.anthropic.com/

### 3. Ollama (Optional)

```bash
# Install Ollama
curl https://ollama.ai/install.sh | sh

# Pull a model
ollama pull llama2
```

**Note:** At least one provider must be configured to run the example.

## Running the Example

### Basic Usage

```bash
# With OpenAI only
export OPENAI_API_KEY="sk-..."
go run main.go
```

### With All Providers

```bash
# Set all API keys
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."

# Start Ollama (in separate terminal)
ollama serve

# Run example
go run main.go
```

## Expected Output

```
Available LLM providers: [openai anthropic ollama]
Started openai-worker with provider openai
Started anthropic-worker with provider anthropic
Started ollama-worker with provider ollama

Executing workflow: Compare LLM Providers

=== Results ===

OpenAI Analysis (openai):
  Status: completed
  Response: The sentiment of this text is positive. The phrase "exceeded my expectations" indicates satisfaction and delight...
  Tokens: 45
  Model: gpt-4

Anthropic Analysis (anthropic):
  Status: completed
  Response: This text expresses a strongly positive sentiment. The reviewer indicates that the product surpassed their initial expectations...
  Tokens: 52
  Model: claude-3-sonnet-20240229

Ollama Analysis (ollama):
  Status: completed
  Response: The sentiment is positive. The use of "exceeded my expectations" shows satisfaction...
  Tokens: 0
  Model: llama2
```

## How It Works

### 1. Provider Factory

```go
// Create factory with all available providers
providerFactory := llm.CreateDefaultProviders()

// Lists providers based on environment variables
// Includes: openai, anthropic, ollama (if available)
```

### 2. Provider-Specific Workers

```go
// Each worker specializes in one provider
worker := createLLMWorker("openai-worker", "openai", providerFactory, protocol, ledger)

// Worker capability matches provider name
capabilities: []string{"openai"}
```

### 3. Task Routing

```go
// Tasks are routed by type to matching worker
task := &multiagent.Task{
    Type: "openai",  // Routes to openai-worker
    Input: map[string]interface{}{
        "user_prompt": "Analyze sentiment...",
        "model": "gpt-4",
    },
}
```

### 4. Parallel Execution

The orchestrator executes all tasks in parallel, sending each to the appropriate provider-specific worker.

## Customization

### Add More Tasks

```go
workflow.Tasks = append(workflow.Tasks, &multiagent.Task{
    ID:   "translation-task",
    Type: "openai",
    Input: map[string]interface{}{
        "system_prompt": "You are a translator.",
        "user_prompt":   "Translate to French: Hello world",
        "model":         "gpt-3.5-turbo",
    },
})
```

### Use Chat API

```go
// In worker handler
resp, err := provider.GenerateChat(ctx, &llm.ChatRequest{
    Messages: []llm.Message{
        {Role: "system", Content: "You are helpful."},
        {Role: "user", Content: "Hello!"},
    },
    Model:       "gpt-4",
    Temperature: 0.7,
    MaxTokens:   100,
})
```

### Change Models

```go
// OpenAI models
"gpt-4"
"gpt-4-turbo-preview"
"gpt-3.5-turbo"

// Anthropic models
"claude-3-opus-20240229"
"claude-3-sonnet-20240229"
"claude-3-haiku-20240307"

// Ollama models
"llama2"
"mistral"
"codellama"
"phi"
```

## Cost Comparison

Running this example with all 3 tasks:

| Provider | Model | Tokens | Cost |
|----------|-------|--------|------|
| OpenAI | GPT-4 | ~50 | $0.0015 |
| Anthropic | Claude 3 Sonnet | ~50 | $0.0002 |
| Ollama | Llama 2 | ~50 | $0.0000 (local) |

**Total: ~$0.0017 per run**

## Production Patterns

### 1. Provider Fallback

```go
providers := []llm.Provider{
    openaiProvider,
    anthropicProvider,
    ollamaProvider, // Fallback to local
}

for _, provider := range providers {
    resp, err := provider.GenerateCompletion(ctx, req)
    if err == nil {
        return resp, nil
    }
}
```

### 2. Cost-Based Routing

```go
func selectProvider(task *Task) string {
    if task.Priority == PriorityHigh {
        return "openai" // Best quality
    } else if task.Priority == PriorityMedium {
        return "anthropic" // Good balance
    } else {
        return "ollama" // Free local
    }
}
```

### 3. Rate Limiting

```go
type RateLimitedWorker struct {
    worker  *WorkerAgent
    limiter *rate.Limiter
}

// Limit: 60 requests per minute
limiter := rate.NewLimiter(1, 1)
limiter.Wait(ctx) // Wait if needed
```

## Troubleshooting

### OpenAI Errors

```bash
# API key not set
Error: OpenAI API key required
→ export OPENAI_API_KEY="sk-..."

# Rate limit
Error: Rate limit exceeded
→ Reduce concurrent tasks or wait
```

### Anthropic Errors

```bash
# API key not set
Error: Anthropic API key required
→ export ANTHROPIC_API_KEY="sk-ant-..."

# Invalid model
Error: model not found
→ Use: claude-3-sonnet-20240229
```

### Ollama Errors

```bash
# Ollama not running
Error: connection refused
→ ollama serve

# Model not found
Error: model 'llama2' not found
→ ollama pull llama2
```

## Next Steps

- Add more providers (Cohere, Google Gemini)
- Implement streaming responses
- Add result caching
- Create A/B testing framework
- Build cost tracking dashboard

## Related Examples

- [Tutorial 1: Orchestrator and Workers](../../TUTORIALS.md#tutorial-1)
- [Tutorial 3: Message Communication](../../TUTORIALS.md#tutorial-3)
- [Example 1: Data Processing Pipeline](../../TUTORIALS.md#example-1)
