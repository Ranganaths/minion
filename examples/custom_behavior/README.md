# Custom Behavior Example

This example demonstrates how to create and use custom behaviors to specialize agent processing logic.

## What This Example Shows

1. **Custom behavior creation** - Implementing the Behavior interface
2. **Behavior registration** - Adding behaviors to the framework
3. **Specialized processing** - Custom input/output processing
4. **Domain-specific prompts** - Tailored system prompts for different tasks
5. **Multiple behavior types** - Sentiment Analysis, Translation, and Code Review

## Prerequisites

- Go 1.25 or higher
- OpenAI API key

## Setup

```bash
export OPENAI_API_KEY="your-api-key-here"
```

## Running the Example

```bash
cd examples/custom_behavior
go run main.go
```

## Custom Behaviors Demonstrated

### 1. Sentiment Analysis Behavior
- **Purpose**: Analyzes emotional tone and sentiment
- **Processing**: Cleans input, adds structured instructions
- **Output**: Sentiment classification with confidence scores

### 2. Translation Behavior
- **Purpose**: Translates text between languages
- **Processing**: Uses agent metadata for source/target languages
- **Output**: Translated text with metadata

### 3. Code Review Behavior
- **Purpose**: Reviews code for quality and issues
- **Processing**: Formats code with language context
- **Output**: Structured review with recommendations

## Creating a Custom Behavior

Implement the `core.Behavior` interface:

```go
type MyCustomBehavior struct{}

// GetSystemPrompt generates the system prompt for the agent
func (b *MyCustomBehavior) GetSystemPrompt(agent *models.Agent) string {
    return fmt.Sprintf(`You are %s.

%s

Your specific instructions here...
`, agent.Name, agent.Description)
}

// ProcessInput prepares input before LLM execution
func (b *MyCustomBehavior) ProcessInput(
    ctx context.Context,
    agent *models.Agent,
    input *models.Input,
) (*models.ProcessedInput, error) {
    // Transform input
    processedText := transformInput(input.Raw)

    return &models.ProcessedInput{
        Original:     input,
        Processed:    processedText,
        Instructions: "Additional instructions for the LLM",
        ExtraContext: map[string]interface{}{
            "metadata_key": "metadata_value",
        },
    }, nil
}

// ProcessOutput enhances output after LLM execution
func (b *MyCustomBehavior) ProcessOutput(
    ctx context.Context,
    agent *models.Agent,
    output *models.Output,
) (*models.ProcessedOutput, error) {
    // Add metadata or transform output
    enhanced := map[string]interface{}{
        "processed_at": time.Now(),
        "custom_data":  "value",
    }

    return &models.ProcessedOutput{
        Original:  output,
        Processed: output.Result,
        Enhanced:  enhanced,
    }, nil
}
```

## Registering Custom Behaviors

```go
behavior := &MyCustomBehavior{}
if err := framework.RegisterBehavior("my_behavior", behavior); err != nil {
    log.Fatalf("Failed to register behavior: %v", err)
}

// Create agent with custom behavior
agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name:         "My Agent",
    BehaviorType: "my_behavior",  // Use your registered behavior type
    // ... other config
})
```

## Behavior Interface Methods

### GetSystemPrompt
- Called once per execution
- Generates the system prompt for the LLM
- Can use agent configuration and metadata

### ProcessInput
- Called before LLM execution
- Transforms and validates input
- Adds context and instructions
- Can return errors to abort execution

### ProcessOutput
- Called after LLM execution
- Post-processes LLM response
- Adds metadata and enhancements
- Can transform the output format

## Expected Output

```
🎨 Agent Framework - Custom Behavior Example
============================================

1. Initializing framework...
   ✓ Framework initialized

2. Registering custom behaviors...
   ✓ Registered: sentiment_analysis
   ✓ Registered: translation
   ✓ Registered: code_review

3. Creating sentiment analysis agent...
   ✓ Agent created: Sentiment Analyzer (Behavior: sentiment_analysis)

4. Creating translation agent...
   ✓ Agent created: Spanish Translator (Behavior: translation)

5. Creating code review agent...
   ✓ Agent created: Code Reviewer (Behavior: code_review)

6. Executing agents with custom behaviors...

   === Sentiment Analysis Agent ===
   Input: I absolutely love this product! It has exceeded all my expectations...

   Analysis:
   Sentiment: Positive
   Confidence: 95%
   Key Indicators: "love", "exceeded expectations", "much easier"
   Explanation: The text expresses strong positive emotions...

   === Translation Agent ===
   Input (English): Hello, how are you today? I hope you're having a wonderful day!

   Translation (Spanish):
   ¡Hola! ¿Cómo estás hoy? ¡Espero que estés teniendo un día maravilloso!

   === Code Review Agent ===
   Input:
   func calculateTotal(items []Item) float64 { ... }

   Review:
   Quality Score: 7/10
   Issues:
   - Consider using range instead of index loop
   - Variable naming could be more descriptive
   Security: No concerns
   Performance: Good for small datasets
   Recommendations: ...

7. Agent metrics summary...

   Sentiment Analyzer:
   - Executions: 1
   - Success rate: 100.0%
   - Avg time: 1234.56ms

   Spanish Translator:
   - Executions: 1
   - Success rate: 100.0%
   - Avg time: 1456.78ms

   Code Reviewer:
   - Executions: 1
   - Success rate: 100.0%
   - Avg time: 1678.90ms

✅ Custom behavior example completed successfully!
```

## Use Cases for Custom Behaviors

1. **Domain-Specific Processing**
   - SQL query generation
   - Legal document analysis
   - Medical diagnosis assistance

2. **Input Validation**
   - Format checking
   - Security validation
   - Data sanitization

3. **Output Formatting**
   - Structured responses
   - Format conversion
   - Data enrichment

4. **Context Enhancement**
   - Adding domain knowledge
   - Including relevant metadata
   - Retrieving additional data

## Key Concepts

### Behavior vs. Tool

- **Behavior**: Shapes how an agent processes all inputs/outputs
- **Tool**: Provides specific functionality that can be called

### When to Use Custom Behaviors

Use custom behaviors when you need to:
- Change how agents interpret and process input
- Customize system prompts based on agent type
- Add domain-specific processing logic
- Enforce specific output formats
- Add pre/post-processing steps

### Behavior Configuration

Behaviors can access:
- Agent configuration (`agent.Config`)
- Agent metadata (`agent.Metadata`)
- Agent capabilities (`agent.Capabilities`)
- Input context (`input.Context`)

## Next Steps

- Combine custom behaviors with custom tools
- Create behavior hierarchies (behaviors that wrap other behaviors)
- Explore specialized domain-specific behaviors
- Review the main README.md for more details
