# Tutorial 1: Framework Basics

**Duration**: 30 minutes
**Level**: Beginner
**Prerequisites**: Basic Go knowledge

## 🎯 Learning Objectives

By the end of this tutorial, you will:
- Understand the Minion framework architecture
- Create and configure your first AI agent
- Use built-in tools for task automation
- Manage agent capabilities and permissions
- Handle tool execution and responses

## 📚 What is Minion?

Minion is a Go-based framework for building AI-powered automation agents. Think of it as a **tool execution engine** where:

- **Agents** = AI assistants that can use tools
- **Tools** = Functions that perform specific tasks
- **Capabilities** = Permissions defining what agents can do

### Real-World Analogy

Imagine hiring an assistant:
- **Agent** = The assistant
- **Tools** = Skills they can use (email, calendar, spreadsheets)
- **Capabilities** = Their job permissions (can send emails, can't delete files)

## 🏗️ Architecture Overview

```
┌─────────────────────────────────────────────────┐
│              Minion Framework                    │
│                                                  │
│  ┌────────────────────────────────────────────┐ │
│  │           Agent Registry                    │ │
│  │  ┌───────┐  ┌───────┐  ┌───────┐          │ │
│  │  │Agent 1│  │Agent 2│  │Agent 3│          │ │
│  │  └───────┘  └───────┘  └───────┘          │ │
│  └────────────────────────────────────────────┘ │
│                      │                           │
│  ┌───────────────────▼──────────────────────┐  │
│  │            Tool Registry                  │  │
│  │  ┌──────┐  ┌──────┐  ┌──────┐           │  │
│  │  │Tool 1│  │Tool 2│  │Tool N│           │  │
│  │  └──────┘  └──────┘  └──────┘           │  │
│  └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

## 🛠️ Setup

### Step 1: Create Project Directory

```bash
mkdir minion-tutorial
cd minion-tutorial

# Initialize Go module
go mod init tutorial
```

### Step 2: Install Minion

```bash
go get github.com/yourusername/minion
```

### Step 3: Create Main File

Create `main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"
)

func main() {
	fmt.Println("Hello, Minion!")
}
```

### Step 4: Verify Setup

```bash
go run main.go
# Output: Hello, Minion!
```

## 📖 Core Concepts

### 1. Framework

The **Framework** is the central coordinator. It manages:
- Agent lifecycle
- Tool registry
- Capability checking

```go
import "github.com/yourusername/minion/core"

// Create framework
framework := core.NewFramework()
defer framework.Close()
```

### 2. Agents

**Agents** are autonomous entities that can execute tools. Each agent has:
- **ID**: Unique identifier
- **Name**: Human-readable name
- **Capabilities**: List of allowed actions

```go
import "github.com/yourusername/minion/models"

agent := &models.Agent{
    ID:   "agent-001",
    Name: "TaskBot",
    Capabilities: []string{
        "file_operations",
        "email",
    },
}
```

### 3. Tools

**Tools** are functions that agents can execute. Each tool has:
- **Name**: Unique identifier (e.g., "send_email")
- **Description**: What it does
- **Parameters**: Input schema
- **Execute**: The function to run

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]interface{}
    Execute(ctx context.Context, input *ToolInput) (*ToolOutput, error)
}
```

### 4. Capabilities

**Capabilities** are permissions. Before executing a tool, the framework checks:

```
Does agent have capability for this tool?
  ✅ Yes → Execute tool
  ❌ No  → Deny access
```

## 🚀 Hands-On: Your First Agent

Let's build a simple task automation agent step by step.

### Step 1: Import Required Packages

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/yourusername/minion/core"
	"github.com/yourusername/minion/models"
)

func main() {
	// We'll add code here
}
```

### Step 2: Initialize Framework

```go
func main() {
	// Create context
	ctx := context.Background()

	// Initialize framework
	framework := core.NewFramework()
	defer framework.Close()

	fmt.Println("✅ Framework initialized")
}
```

### Step 3: Create Your First Agent

```go
func main() {
	ctx := context.Background()
	framework := core.NewFramework()
	defer framework.Close()

	// Create agent configuration
	agentReq := &models.CreateAgentRequest{
		Name: "TaskBot",
		Description: "A simple task automation agent",
		Capabilities: []string{
			"basic_operations",  // Allow basic tools
		},
	}

	// Create agent
	agent, err := framework.CreateAgent(ctx, agentReq)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Printf("✅ Created agent: %s (ID: %s)\n", agent.Name, agent.ID)
}
```

### Step 4: List Available Tools

```go
// After creating agent
tools := framework.GetToolsForAgent(agent)

fmt.Printf("\n📋 Available tools for %s:\n", agent.Name)
for _, tool := range tools {
	fmt.Printf("  • %s - %s\n", tool.Name(), tool.Description())
}
```

### Step 5: Execute a Tool

```go
// Execute a simple tool (example: echo tool)
input := &models.ToolInput{
	ToolName: "echo",
	Params: map[string]interface{}{
		"message": "Hello from TaskBot!",
	},
}

output, err := framework.ExecuteTool(ctx, agent.ID, input)
if err != nil {
	log.Fatalf("Tool execution failed: %v", err)
}

if output.Success {
	fmt.Printf("\n✅ Tool executed successfully!\n")
	fmt.Printf("Result: %v\n", output.Result)
} else {
	fmt.Printf("\n❌ Tool failed: %s\n", output.Error)
}
```

### Complete Example

Here's the complete `main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/yourusername/minion/core"
	"github.com/yourusername/minion/models"
)

func main() {
	// 1. Initialize
	ctx := context.Background()
	framework := core.NewFramework()
	defer framework.Close()
	fmt.Println("✅ Framework initialized")

	// 2. Create Agent
	agentReq := &models.CreateAgentRequest{
		Name:        "TaskBot",
		Description: "A simple task automation agent",
		Capabilities: []string{
			"basic_operations",
		},
	}

	agent, err := framework.CreateAgent(ctx, agentReq)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	fmt.Printf("✅ Created agent: %s (ID: %s)\n", agent.Name, agent.ID)

	// 3. List Tools
	tools := framework.GetToolsForAgent(agent)
	fmt.Printf("\n📋 Available tools (%d total):\n", len(tools))
	for i, tool := range tools {
		if i < 5 { // Show first 5
			fmt.Printf("  • %s - %s\n", tool.Name(), tool.Description())
		}
	}

	// 4. Execute Tool
	fmt.Println("\n🔧 Executing echo tool...")
	input := &models.ToolInput{
		ToolName: "echo",
		Params: map[string]interface{}{
			"message": "Hello from TaskBot!",
		},
	}

	output, err := framework.ExecuteTool(ctx, agent.ID, input)
	if err != nil {
		log.Fatalf("Tool execution failed: %v", err)
	}

	if output.Success {
		fmt.Printf("✅ Success!\n")
		fmt.Printf("Result: %v\n", output.Result)
	} else {
		fmt.Printf("❌ Failed: %s\n", output.Error)
	}
}
```

### Run It!

```bash
go run main.go
```

**Expected Output**:
```
✅ Framework initialized
✅ Created agent: TaskBot (ID: agent-xyz123)

📋 Available tools (84 total):
  • echo - Echo back a message
  • file_read - Read file contents
  • file_write - Write to file
  • http_get - Make HTTP GET request
  • time_now - Get current time

🔧 Executing echo tool...
✅ Success!
Result: Hello from TaskBot!
```

## 🎓 Key Concepts Explained

### Capability System

The capability system provides security through permissions:

```go
// Level 1: Category-level access
Capabilities: []string{"file_operations"}
// Agent can use: file_read, file_write, file_delete, etc.

// Level 2: Tool-specific access
Capabilities: []string{"file_read", "file_write"}
// Agent can ONLY use: file_read and file_write

// Level 3: Global access (use with caution!)
Capabilities: []string{"*"}
// Agent can use ALL tools
```

### Tool Input/Output

Every tool execution follows this pattern:

```go
// Input
input := &models.ToolInput{
	ToolName: "tool_name",
	Params: map[string]interface{}{
		"param1": "value1",
		"param2": 123,
	},
}

// Output
type ToolOutput struct {
	Success bool
	Result  interface{}
	Error   string
}
```

### Error Handling

Always check for errors at two levels:

```go
// Level 1: Execution error
output, err := framework.ExecuteTool(ctx, agentID, input)
if err != nil {
	// Framework-level error (agent not found, etc.)
	log.Fatal(err)
}

// Level 2: Tool error
if !output.Success {
	// Tool-level error (invalid params, operation failed, etc.)
	fmt.Println(output.Error)
}
```

## 🏋️ Practice Exercises

### Exercise 1: Create Multiple Agents

Create three agents with different capabilities:

```go
// 1. FileAgent - Only file operations
// 2. WebAgent - Only HTTP operations
// 3. AdminAgent - All operations
```

<details>
<summary>Click to see solution</summary>

```go
// FileAgent
fileAgent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
	Name: "FileAgent",
	Capabilities: []string{"file_operations"},
})

// WebAgent
webAgent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
	Name: "WebAgent",
	Capabilities: []string{"http_operations"},
})

// AdminAgent
adminAgent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
	Name: "AdminAgent",
	Capabilities: []string{"*"},
})

// Verify different tool access
fmt.Printf("FileAgent tools: %d\n", len(framework.GetToolsForAgent(fileAgent)))
fmt.Printf("WebAgent tools: %d\n", len(framework.GetToolsForAgent(webAgent)))
fmt.Printf("AdminAgent tools: %d\n", len(framework.GetToolsForAgent(adminAgent)))
```
</details>

### Exercise 2: Execute Multiple Tools

Execute 3 different tools in sequence:

```go
// 1. Get current time
// 2. Write time to file
// 3. Read file back
```

<details>
<summary>Click to see solution</summary>

```go
// 1. Get current time
output1, _ := framework.ExecuteTool(ctx, agent.ID, &models.ToolInput{
	ToolName: "time_now",
	Params:   map[string]interface{}{},
})
currentTime := output1.Result

// 2. Write to file
output2, _ := framework.ExecuteTool(ctx, agent.ID, &models.ToolInput{
	ToolName: "file_write",
	Params: map[string]interface{}{
		"path":    "/tmp/time.txt",
		"content": fmt.Sprintf("Current time: %v", currentTime),
	},
})

// 3. Read file back
output3, _ := framework.ExecuteTool(ctx, agent.ID, &models.ToolInput{
	ToolName: "file_read",
	Params: map[string]interface{}{
		"path": "/tmp/time.txt",
	},
})
fmt.Printf("File contents: %v\n", output3.Result)
```
</details>

### Exercise 3: Handle Tool Failures

Try to execute a tool the agent doesn't have permission for:

```go
// Create limited agent
// Try to use unauthorized tool
// Handle the error appropriately
```

<details>
<summary>Click to see solution</summary>

```go
// Create agent with limited capabilities
limitedAgent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
	Name:         "LimitedAgent",
	Capabilities: []string{"file_read"}, // Only reading
})

// Try to write (should fail)
output, err := framework.ExecuteTool(ctx, limitedAgent.ID, &models.ToolInput{
	ToolName: "file_write",
	Params: map[string]interface{}{
		"path":    "/tmp/test.txt",
		"content": "test",
	},
})

if err != nil {
	fmt.Printf("❌ Error: %v\n", err)
} else if !output.Success {
	fmt.Printf("❌ Tool failed: %s\n", output.Error)
} else {
	fmt.Println("✅ Unexpected success!")
}
```
</details>

## 🐛 Troubleshooting

### Issue 1: "Agent not found"

**Problem**: Trying to execute tool with invalid agent ID

**Solution**:
```go
// Store agent reference
agent, _ := framework.CreateAgent(ctx, req)

// Use agent.ID consistently
output, _ := framework.ExecuteTool(ctx, agent.ID, input)
```

### Issue 2: "Tool not available"

**Problem**: Agent doesn't have capability for tool

**Solution**:
```go
// Add capability when creating agent
Capabilities: []string{"file_operations"}

// Or check available tools first
tools := framework.GetToolsForAgent(agent)
```

### Issue 3: "Invalid parameters"

**Problem**: Tool parameters don't match schema

**Solution**:
```go
// Check tool schema
tool := framework.GetTool("tool_name")
fmt.Println(tool.Parameters())

// Match parameter names and types
Params: map[string]interface{}{
	"name_from_schema": correctType,
}
```

## 📝 Summary

Congratulations! You've learned:

✅ How to initialize the Minion framework
✅ How to create and configure agents
✅ How to list and execute tools
✅ How the capability system works
✅ How to handle tool execution results

### Key Takeaways

1. **Framework** = Central coordinator
2. **Agents** = Entities that execute tools
3. **Tools** = Functions agents can call
4. **Capabilities** = Permission system
5. **Always check both** execution errors AND tool success

## 🎯 Next Steps

You're now ready for:

**[Tutorial 2: MCP Integration Basics →](02-mcp-basics.md)**

Learn how to connect external services via Model Context Protocol!

### Additional Resources

- [Framework API Reference](../api/framework.md)
- [Built-in Tools List](../api/tools.md)
- [Capability System Guide](../guides/capabilities.md)

## 💬 Questions?

- [Open an Issue](https://github.com/yourusername/minion/issues)
- [Join Discussions](https://github.com/yourusername/minion/discussions)
- [View Examples](../../mcp/examples/)

---

**Great job! 🎉 Continue to [Tutorial 2](02-mcp-basics.md) when ready.**
