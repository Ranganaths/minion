# Basic Agent Framework Example

This example demonstrates the fundamental usage of the Agent Framework.

## What This Example Shows

1. **Framework initialization** - Setting up storage and LLM provider
2. **Agent creation** - Creating an agent with configuration
3. **Agent activation** - Updating agent status
4. **Agent execution** - Running queries through the agent
5. **Metrics retrieval** - Getting execution statistics
6. **Activity tracking** - Viewing execution history
7. **Agent listing** - Querying available agents

## Prerequisites

- Go 1.25 or higher
- OpenAI API key

## Setup

Set your OpenAI API key:

```bash
export OPENAI_API_KEY="your-api-key-here"
```

## Running the Example

```bash
cd pkg/agentframework/examples/basic
go run main.go
```

## Expected Output

```
🤖 Agent Framework - Basic Example
===================================

1. Initializing framework...
   ✓ Framework initialized

2. Creating agent...
   ✓ Agent created: My First Agent (ID: xxx-xxx-xxx)
   - Behavior: default
   - Status: draft

3. Activating agent...
   ✓ Agent activated: active

4. Executing agent with questions...

   Question 1: What is 2 + 2?
   Answer: 2 + 2 equals 4.
   Tokens used: 15

   Question 2: Explain quantum computing in one sentence.
   Answer: Quantum computing uses quantum bits...
   Tokens used: 42

   ...

5. Retrieving agent metrics...
   ✓ Metrics retrieved:
   - Total executions: 3
   - Successful: 3
   - Failed: 0
   - Avg execution time: 1234.56ms

6. Retrieving recent activities...
   ✓ Found 3 activities:
   1. Action: execute | Status: success | Duration: 1200ms
   2. Action: execute | Status: success | Duration: 1300ms
   3. Action: execute | Status: success | Duration: 1100ms

7. Listing all agents...
   ✓ Found 1 agent(s):
   1. My First Agent (xxx-xxx-xxx) - Status: active

✅ Example completed successfully!
```

## Key Concepts

### Framework Configuration

The framework is configured using the options pattern:

```go
framework := core.NewFramework(
    core.WithStorage(storage.NewInMemory()),
    core.WithLLMProvider(llm.NewOpenAI(apiKey)),
)
```

### Agent Configuration

Agents are configured with:
- **Name & Description** - Identification
- **BehaviorType** - How the agent processes input/output (default: "default")
- **Config** - LLM settings (model, temperature, max tokens, personality)
- **Capabilities** - What the agent can do
- **Status** - Lifecycle state (draft, active, inactive, archived)

### Execution Flow

1. Input is processed by the agent's behavior
2. System prompt is generated based on agent configuration
3. LLM processes the request
4. Output is processed by the agent's behavior
5. Activity is recorded and metrics are updated

## Next Steps

- Try the **with_tools** example to see tool integration
- Try the **custom_behavior** example to create custom agent behaviors
- Explore the framework documentation in `pkg/agentframework/README.md`
