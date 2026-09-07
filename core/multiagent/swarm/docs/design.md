# Agent Swarms - Design Document

## Overview

Agent Swarms enable **parallel task execution** where multiple specialized agents work on the same or related tasks simultaneously, with results aggregated using configurable strategies. Unlike orchestrators (sequential delegation) or group chats (sequential conversation), swarms emphasize **collective intelligence** through parallel execution.

## Core Concepts

### Swarm
A swarm is a dynamic group of agents sharing a common purpose, tagged for discovery and coordinated through parallel task execution.

### Key Differences from Existing Patterns

| Pattern | Execution | Communication | Use Case |
|---------|-----------|---------------|----------|
| **Orchestrator** | Sequential delegation | Direct messaging | Task decomposition |
| **GroupChat** | Sequential conversation | Round-robin | Discussion/debate |
| **WorkerPool** | Parallel, homogeneous | Work queue | Load distribution |
| **Swarms** | Parallel, heterogeneous | Result aggregation | Collective problem-solving |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Swarm                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │  SwarmAgent │  │  SwarmAgent │  │  SwarmAgent │        │
│  │   (tag:A)   │  │   (tag:B)  │  │  (tag:A,B) │        │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘        │
│         │                 │                 │               │
│         └─────────────────┼─────────────────┘               │
│                           │                                 │
│                    ┌──────▼──────┐                          │
│                    │   Task      │                          │
│                    │  Executor   │                          │
│                    └──────┬──────┘                          │
│                           │                                 │
│                    ┌──────▼──────┐                          │
│                    │  Aggregator │                          │
│                    └─────────────┘                          │
└─────────────────────────────────────────────────────────────┘
```

## Components

### 1. Swarm
- **ID**: Unique identifier
- **Name**: Human-readable name
- **Tags**: Labels for discovery and routing
- **Agents**: Members of the swarm
- **Config**: Swarm-specific settings

### 2. SwarmAgent
- Wraps an existing agent with swarm membership
- Can belong to multiple swarms
- Reports results back to swarm executor

### 3. TaskExecutor
- Dispatches tasks to swarm agents in parallel
- Handles timeouts and retries per agent
- Collects results as they arrive

### 4. ResultAggregator
- Configurable aggregation strategies
- Handles partial failures
- Provides consensus/ranking mechanisms

## Aggregation Strategies

| Strategy | Description | Use Case |
|----------|-------------|----------|
| `FirstComplete` | Return first result | Speed-critical, any-answer |
| `AllComplete` | Wait for all, return all | Complete analysis |
| `Majority` | Return most common answer | Consensus decisions |
| `BestRanked` | Score and return best | Quality-critical |
| `Average` | Numeric average | Measurements |

## Tag-Based Discovery

Agents can be discovered by tags:
```go
// Find all agents with "security" tag
agents := swarm.FindByTag("security")

// Find agents with ALL specified tags
agents := swarm.FindByTags("critical", "billing")
```

## Task Execution Flow

1. Submit task to swarm
2. Filter agents by task requirements (tags)
3. Dispatch task to all eligible agents in parallel
4. Collect results using aggregation strategy
5. Return aggregated result

## Error Handling

- **Partial Failure**: Continue with successful agents if minThreshold met
- **Total Failure**: Return error if no agents succeed
- **Timeout**: Configurable per-agent and overall timeout

## Integration

Swarms integrate with:
- `Orchestrator`: Can be used as a worker
- `Protocol`: Uses existing message passing
- `Ledger`: Task tracking available
- `Metrics`: Swarm-specific metrics collected
