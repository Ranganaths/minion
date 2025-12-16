# Tool Integration Example

This example demonstrates how to create custom tools and integrate them with agents in the framework.

## What This Example Shows

1. **Custom tool creation** - Implementing the Tool interface
2. **Tool registration** - Adding tools to the framework
3. **Capability-based tool filtering** - Tools available based on agent capabilities
4. **Tool execution** - Both through agents and directly
5. **Multiple tool types** - Calculator, Text Analysis, and Weather tools

## Prerequisites

- Go 1.25 or higher
- OpenAI API key

## Setup

```bash
export OPENAI_API_KEY="your-api-key-here"
```

## Running the Example

```bash
cd pkg/agentframework/examples/with_tools
go run main.go
```

## Custom Tools Demonstrated

### 1. Calculator Tool
- **Capabilities required**: `math`, `calculation`
- **Operations**: add, subtract, multiply, divide, power
- **Usage**: Performs mathematical calculations

### 2. Text Analysis Tool
- **Capabilities required**: `text_analysis`
- **Features**: Word count, character count, sentence count, average word length
- **Usage**: Analyzes text and provides statistics

### 3. Weather Tool
- **Capabilities required**: None (available to all agents)
- **Features**: Returns simulated weather data for a location
- **Usage**: Demonstrates API integration pattern

## Creating a Custom Tool

To create a custom tool, implement the `tools.Tool` interface:

```go
type MyCustomTool struct{}

func (t *MyCustomTool) Name() string {
    return "my_tool"
}

func (t *MyCustomTool) Description() string {
    return "What this tool does"
}

func (t *MyCustomTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
    // Your tool logic here
    return &models.ToolOutput{
        ToolName: t.Name(),
        Success:  true,
        Result:   "tool result",
    }, nil
}

func (t *MyCustomTool) CanExecute(agent *models.Agent) bool {
    // Check if agent has required capabilities
    for _, cap := range agent.Capabilities {
        if cap == "required_capability" {
            return true
        }
    }
    return false
}
```

## Registering Tools

```go
tool := &MyCustomTool{}
if err := framework.RegisterTool(tool); err != nil {
    log.Fatalf("Failed to register tool: %v", err)
}
```

## Capability-Based Filtering

Tools are automatically filtered based on agent capabilities:

```go
// Agent with "math" capability
mathAgent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
    Name: "Math Agent",
    Capabilities: []string{"math"},
})

// Only tools with CanExecute returning true for this agent will be available
availableTools := framework.GetToolsForAgent(mathAgent)
```

## Direct Tool Execution

You can also execute tools directly without going through an agent:

```go
result, err := tool.Execute(ctx, &models.ToolInput{
    Params: map[string]interface{}{
        "param1": "value1",
        "param2": 42,
    },
})
```

## Expected Output

```
🛠️  Agent Framework - Tool Integration Example
================================================

1. Initializing framework...
   ✓ Framework initialized

2. Registering tools...
   ✓ Registered: calculator
   ✓ Registered: text_analysis
   ✓ Registered: weather

3. Creating math-capable agent...
   ✓ Agent created: Math Assistant

4. Checking available tools for math agent...
   ✓ Found 2 available tool(s):
   - calculator: Performs basic mathematical calculations
   - weather: Gets weather information for a location

5. Creating text analysis agent...
   ✓ Agent created: Text Analyzer

   Checking available tools for text agent...
   ✓ Found 2 available tool(s):
   - text_analysis: Analyzes text and provides statistics
   - weather: Gets weather information for a location

6. Executing agents...

   Math Agent:
   Response: 15 multiplied by 7 equals 105.

   Text Analysis Agent:
   Response: The text contains 9 words and 44 characters...

7. Direct tool execution demo...

   Calculator Tool:
   Result: 15 × 7 = 105

   Text Analysis Tool:
   Result: map[avg_word_length:4.888889 character_count:44 sentence_count:1 word_count:9]

✅ Tool integration example completed successfully!
```

## Key Concepts

### Tool Interface

All tools must implement:
- `Name()` - Unique tool identifier
- `Description()` - What the tool does
- `Execute()` - Tool logic
- `CanExecute()` - Capability-based filtering

### Tool Input

Tools receive structured input:
```go
type ToolInput struct {
    Data    interface{}              // Main data payload
    Params  map[string]interface{}   // Parameters
    Context map[string]interface{}   // Additional context
}
```

### Tool Output

Tools return structured output:
```go
type ToolOutput struct {
    ToolName      string
    Success       bool
    Result        interface{}
    ExecutionTime int64
    Error         string
}
```

## Next Steps

- Try the **custom_behavior** example to create custom agent behaviors
- Implement your own tools for domain-specific functionality
- Explore combining multiple tools in a single agent
