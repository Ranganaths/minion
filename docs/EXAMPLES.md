# Examples

This page provides an index of all example applications included with Minion.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/Ranganaths/minion.git
cd minion

# Set API key
export OPENAI_API_KEY="sk-..."

# Run an example
go run examples/basic/main.go
```

## Basic Examples

### Basic Agent

Create and execute a simple agent with LLM.

```bash
go run examples/basic/main.go
```

**Demonstrates:**
- Framework initialization
- Agent creation and activation
- Basic execution
- Metrics retrieval

**Location:** [`examples/basic/`](../examples/basic/)

---

### Agent with Tools

Agent that can use built-in tools.

```bash
go run examples/with_tools/main.go
```

**Demonstrates:**
- Tool registration
- Capability-based tool filtering
- Tool execution
- Tool output handling

**Location:** [`examples/with_tools/`](../examples/with_tools/)

---

### Custom Behavior

Create a custom behavior for specialized agent processing.

```bash
go run examples/custom_behavior/main.go
```

**Demonstrates:**
- Behavior interface implementation
- Custom system prompts
- Input/output processing
- Behavior registration

**Location:** [`examples/custom_behavior/`](../examples/custom_behavior/)

---

### Domain Tools

Use domain-specific tools for various tasks.

```bash
go run examples/domain_tools/main.go
```

**Demonstrates:**
- Data tools (CSV, JSON, XML)
- HTTP tools
- File operations
- Security tools

**Location:** [`examples/domain_tools/`](../examples/domain_tools/)

## Multi-Agent Examples

### Multi-Agent Basic

Simple multi-agent system with coordinator and workers.

```bash
go run examples/multiagent-basic/main.go
```

**Demonstrates:**
- Orchestrator creation
- Worker specialization
- Task delegation
- Result aggregation

**Location:** [`examples/multiagent-basic/`](../examples/multiagent-basic/)

---

### Multi-Agent Custom

Advanced multi-agent patterns with custom workflows.

```bash
go run examples/multiagent-custom/main.go
```

**Demonstrates:**
- Custom worker implementations
- Complex task decomposition
- Inter-agent communication
- Error handling

**Location:** [`examples/multiagent-custom/`](../examples/multiagent-custom/)

---

### Multi-Agent Tracing

Multi-agent system with distributed tracing.

```bash
go run examples/multiagent-tracing/main.go
```

**Demonstrates:**
- Distributed traces across agents
- Span propagation
- Performance monitoring
- Trace visualization

**Location:** [`examples/multiagent-tracing/`](../examples/multiagent-tracing/)

## Protocol Examples

### A2A Server

Expose agent via A2A protocol for inter-agent communication.

```bash
go run examples/a2a-server/main.go
```

**Demonstrates:**
- A2A server setup
- Agent Card generation
- JSON-RPC endpoints
- SSE streaming

**Endpoints:**
- `GET /.well-known/agent.json` - Agent Card
- `POST /` - JSON-RPC endpoint

**Location:** [`examples/a2a-server/`](../examples/a2a-server/)

---

### AG-UI Server

Agent with real-time streaming for frontend applications.

```bash
go run examples/agui-server/main.go
```

**Demonstrates:**
- AG-UI server setup
- SSE event streaming
- Token-by-token output
- State synchronization

**Endpoints:**
- `POST /` - Run request with SSE

**Location:** [`examples/agui-server/`](../examples/agui-server/)

## Industry Examples

### Sales Agent

AI sales assistant with CRM integration.

```bash
go run examples/sales_agent/main.go
```

**Demonstrates:**
- Lead qualification
- CRM tool integration
- Email composition
- Sales pipeline management

**Location:** [`examples/sales_agent/`](../examples/sales_agent/)

---

### Sales Automation

Full sales workflow automation.

```bash
go run examples/sales-automation/main.go
```

**Demonstrates:**
- Lead scoring
- Automated outreach
- Follow-up scheduling
- Analytics

**Location:** [`examples/sales-automation/`](../examples/sales-automation/)

---

### Customer Support

AI-powered customer support agent.

```bash
go run examples/customer-support/main.go
```

**Demonstrates:**
- Ticket classification
- Knowledge base search
- Response generation
- Escalation handling

**Location:** [`examples/customer-support/`](../examples/customer-support/)

---

### Business Automation

General business process automation.

```bash
go run examples/business_automation/main.go
```

**Demonstrates:**
- Document processing
- Approval workflows
- Report generation
- Integration patterns

**Location:** [`examples/business_automation/`](../examples/business_automation/)

---

### DevOps Automation

Infrastructure and deployment automation.

```bash
go run examples/devops-automation/main.go
```

**Demonstrates:**
- CI/CD integration
- Infrastructure management
- Monitoring automation
- Incident response

**Location:** [`examples/devops-automation/`](../examples/devops-automation/)

## Advanced Examples

### Chain Features

LLM chain patterns including RAG, sequential, and router chains.

```bash
go run examples/chain-features/main.go
```

**Demonstrates:**
- LLM chains
- RAG chains
- Sequential chains
- Router chains
- Transform chains

**Location:** [`examples/chain-features/`](../examples/chain-features/)

---

### LLM Worker

Dedicated LLM worker pattern.

```bash
go run examples/llm_worker/main.go
```

**Demonstrates:**
- Worker pool pattern
- Request queuing
- Batch processing
- Rate limiting

**Location:** [`examples/llm_worker/`](../examples/llm_worker/)

---

### TupleLeap Integration

Using TupleLeap as LLM provider.

```bash
go run examples/tupleleap_example/main.go
```

**Demonstrates:**
- TupleLeap provider setup
- Custom model configuration
- API integration

**Location:** [`examples/tupleleap_example/`](../examples/tupleleap_example/)

## Observability Examples

### Tracing

OpenTelemetry distributed tracing.

```bash
go run examples/tracing/main.go
```

**Demonstrates:**
- Tracer initialization
- Span creation
- Attribute recording
- Error tracking

**Location:** [`examples/tracing/`](../examples/tracing/)

---

### Full Observability

Complete observability setup with tracing and metrics.

```bash
go run examples/observability/main.go
```

**Demonstrates:**
- OpenTelemetry tracing
- Prometheus metrics
- Custom instrumentation
- Dashboard integration

**Location:** [`examples/observability/`](../examples/observability/)

---

### Debug Time-Travel

Time-travel debugging for agent execution.

```bash
go run examples/debug-timetravel/main.go
```

**Demonstrates:**
- Execution recording
- State snapshots
- Timeline navigation
- Replay debugging

**Location:** [`examples/debug-timetravel/`](../examples/debug-timetravel/)

## Evaluation Examples

### Agent Evaluation

Benchmark and evaluate agent performance.

```bash
go run examples/evaluation/main.go
```

**Demonstrates:**
- Test case creation
- Evaluation metrics
- Performance benchmarks
- Quality assessment

**Location:** [`examples/evaluation/`](../examples/evaluation/)

---

### Self-Improving Agent

Agent that improves based on feedback.

```bash
go run examples/self-improving/main.go
```

**Demonstrates:**
- Feedback collection
- Performance analysis
- Prompt optimization
- Iterative improvement

**Location:** [`examples/self-improving/`](../examples/self-improving/)

## Running Examples

### Prerequisites

1. **Go 1.24+** installed
2. **API key** set as environment variable

```bash
# OpenAI
export OPENAI_API_KEY="sk-..."

# Or Anthropic
export ANTHROPIC_API_KEY="sk-ant-..."

# Or TupleLeap
export TUPLELEAP_API_KEY="..."
```

### Running

```bash
# From project root
go run examples/<example-name>/main.go

# Or build first
go build -o example examples/<example-name>/main.go
./example
```

### Common Issues

**"API key not set"**
```bash
export OPENAI_API_KEY="your-key-here"
```

**"Module not found"**
```bash
go mod tidy
```

**"Port already in use"** (for server examples)
```bash
# Use different port
PORT=9090 go run examples/a2a-server/main.go
```

## Creating Your Own Examples

Use the basic example as a template:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/Ranganaths/minion/core"
    "github.com/Ranganaths/minion/llm"
    "github.com/Ranganaths/minion/models"
    "github.com/Ranganaths/minion/storage"
)

func main() {
    // 1. Create framework
    framework := core.NewFramework(
        core.WithStorage(storage.NewInMemory()),
        core.WithLLMProvider(llm.NewOpenAI(os.Getenv("OPENAI_API_KEY"))),
    )
    defer framework.Close()

    // 2. Create agent
    ctx := context.Background()
    agent, _ := framework.CreateAgent(ctx, &models.CreateAgentRequest{
        Name:        "My Example Agent",
        Description: "Description of what this agent does",
    })

    // 3. Activate
    activeStatus := models.StatusActive
    framework.UpdateAgent(ctx, agent.ID, &models.UpdateAgentRequest{
        Status: &activeStatus,
    })

    // 4. Execute
    output, _ := framework.Execute(ctx, agent.ID, &models.Input{
        Raw: "Your prompt here",
    })

    fmt.Println(output.Result)
}
```

## Next Steps

- [Getting Started](GETTING_STARTED.md) - Installation guide
- [Architecture](ARCHITECTURE.md) - System design
- [API Reference](API_REFERENCE.md) - Complete API docs
- [Protocols](PROTOCOLS.md) - A2A, AG-UI, MCP
