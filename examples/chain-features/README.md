# Chain Features Example

This example demonstrates the production-ready features in the Minion chain package, including safe type assertions, context-aware streaming, LLM request validation, and non-panicking configuration.

## Features Demonstrated

### 1. Safe Type Assertions

The chain package provides safe type assertion helpers that prevent runtime panics:

```go
baseChain := chain.NewBaseChain("my_chain")

// Safe extraction with proper error handling
count, err := baseChain.GetInt(inputs, "count")
rate, err := baseChain.GetFloat(inputs, "rate")
enabled, err := baseChain.GetBool(inputs, "enabled")
tags, err := baseChain.GetStringSlice(inputs, "tags")
metadata, err := baseChain.GetMap(inputs, "metadata")

// With defaults (never fails)
count := baseChain.GetIntOr(inputs, "count", 10)
rate := baseChain.GetFloatOr(inputs, "rate", 1.0)
enabled := baseChain.GetBoolOr(inputs, "enabled", false)

// Utility functions
str := chain.AsString(anyValue)        // Safe conversion to string
slice := chain.AsStringSlice(anyValue) // Safe conversion to []string
```

**Benefits:**
- Handles JSON number types (float64 → int)
- Clear error messages with field names
- No runtime panics from failed type assertions

### 2. LLM Request Validation

Validate LLM requests before sending to prevent API errors:

```go
req := &llm.CompletionRequest{
    Model:       "gpt-4",
    UserPrompt:  "Hello!",
    Temperature: 0.7,
    MaxTokens:   100,
}

// Validate the request
if err := req.Validate(); err != nil {
    log.Fatalf("Invalid request: %v", err)
}

// Apply defaults
reqWithDefaults := req.WithDefaults("gpt-4", 1000)

// Validate and apply defaults in one step
validReq, err := llm.ValidateCompletionRequest(req, "gpt-4", 1000)
```

**Validation Rules:**
| Field | Rule |
|-------|------|
| Model | Required |
| Temperature | 0 to 2.0 |
| MaxTokens | Non-negative |
| UserPrompt/SystemPrompt | At least one required |
| Messages[].Role | Must be "system", "user", or "assistant" |

### 3. Context-Aware Streaming

All chain `Stream()` methods now properly handle context cancellation:

```go
// Stream with timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

streamCh, err := chain.Stream(ctx, inputs)
if err != nil {
    log.Fatal(err)
}

for event := range streamCh {
    switch event.Type {
    case chain.StreamEventStart:
        fmt.Println("Started")
    case chain.StreamEventChunk:
        fmt.Printf("Chunk: %s\n", event.Content)
    case chain.StreamEventComplete:
        fmt.Println("Done:", event.Data)
    case chain.StreamEventError:
        fmt.Println("Error:", event.Error)
    }
}
// Channel automatically closes, goroutine cleaned up
```

**Benefits:**
- No goroutine leaks on early cancellation
- Proper resource cleanup
- Context deadline respected

### 4. Non-Panicking Configuration

Use `Require*` methods instead of `Must*` for production:

```go
env := config.NewEnv("MYAPP")

// Safe retrieval with defaults
apiKey := env.GetString("API_KEY", "default")
timeout := env.GetDuration("TIMEOUT", 30*time.Second)

// Non-panicking required values
apiKey, err := env.RequireString("API_KEY")
if err != nil {
    log.Fatalf("Configuration error: %v", err)
}

maxTokens, err := env.RequireInt("MAX_TOKENS")
debug, err := env.RequireBool("DEBUG")
timeout, err := env.RequireDuration("TIMEOUT")
temperature, err := env.RequireFloat64("TEMPERATURE")
```

## Running the Example

```bash
cd examples/chain-features
go run main.go
```

**Expected Output:**
```
=== Minion Chain Features Example ===

--- Config Package: Safe Environment Variables ---
API Key: default-key (from env or default)
Max Retries: 3
Timeout: 30s
Debug: false
RequireString returned error (expected): MYAPP_REQUIRED_KEY: required environment variable is not set
...

--- LLM Package: Request Validation ---
Valid request passed validation
Invalid request 1: validation error on field 'Model': model name is required
Invalid request 2: validation error on field 'Temperature': temperature must be between 0 and 2.0
...

--- Chain Package: Safe Type Assertions ---
GetInt('count'): 42
GetInt('float_int'): 100 (from float64)
...

--- Chain Package: Context-Aware Streaming ---
1. Normal streaming (consume all events):
   Event: type=start, content=starting
   Event: type=chunk, content=chunk-0
   ...

=== All examples completed successfully! ===
```

## Use Cases

1. **API Gateway**: Validate incoming LLM requests before processing
2. **Background Workers**: Handle context cancellation gracefully
3. **Data Pipelines**: Safely extract typed values from dynamic inputs
4. **Microservices**: Fail fast on missing configuration

## Related Documentation

- [Production Readiness Guide](../../docs/PRODUCTION_READINESS.md)
- [LLM Providers Guide](../../LLM_PROVIDERS.md)
- [Chain Package](../../chain/)
