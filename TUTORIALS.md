# Minion Multi-Agent Framework - Tutorials

**Quick, practical tutorials for building production-ready multi-agent systems**

---

## Table of Contents

### Quick Start (5 minutes)
- [Tutorial 0: Your First Agent](#tutorial-0-your-first-agent)

### Basic Tutorials (15-30 minutes)
- [Tutorial 1: Orchestrator and Workers](#tutorial-1-orchestrator-and-workers)
- [Tutorial 2: Task Workflows](#tutorial-2-task-workflows)
- [Tutorial 3: Message Communication](#tutorial-3-message-communication)
- [Tutorial 4: Adding Persistence](#tutorial-4-adding-persistence)

### Intermediate Tutorials (30-45 minutes)
- [Tutorial 5: Distributed Deployment](#tutorial-5-distributed-deployment)
- [Tutorial 6: Auto-Scaling Workers](#tutorial-6-auto-scaling-workers)
- [Tutorial 7: Load Balancing Strategies](#tutorial-7-load-balancing-strategies)
- [Tutorial 8: Adding Resilience](#tutorial-8-adding-resilience)

### Advanced Tutorials (45-60 minutes)
- [Tutorial 9: Custom Protocol Backend](#tutorial-9-custom-protocol-backend)
- [Tutorial 10: Production Deployment](#tutorial-10-production-deployment)
- [Tutorial 11: Monitoring and Observability](#tutorial-11-monitoring-and-observability)
- [Tutorial 12: Performance Optimization](#tutorial-12-performance-optimization)

### Real-World Examples
- [Example 1: Data Processing Pipeline](#example-1-data-processing-pipeline)
- [Example 2: Web Scraping Swarm](#example-2-web-scraping-swarm)
- [Example 3: Distributed Testing Framework](#example-3-distributed-testing-framework)

---

## Tutorial 0: Your First Agent

**Time: 5 minutes**
**Goal: Create and run a simple agent**

### Step 1: Create a Basic Agent

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/Ranganaths/minion/core/multiagent"
)

func main() {
    ctx := context.Background()

    // Create a simple worker agent
    agent := multiagent.NewWorkerAgent(
        "worker-1",
        []string{"general"}, // Capabilities
        nil,                 // Use default protocol
        nil,                 // Use default ledger
    )

    fmt.Printf("Created agent: %s\n", agent.GetMetadata().AgentID)
    fmt.Printf("Capabilities: %v\n", agent.GetMetadata().Capabilities)

    // Start the agent
    if err := agent.Start(ctx); err != nil {
        log.Fatal(err)
    }

    fmt.Println("Agent is running!")

    // Keep running for 10 seconds
    time.Sleep(10 * time.Second)

    // Stop the agent
    if err := agent.Stop(ctx); err != nil {
        log.Fatal(err)
    }

    fmt.Println("Agent stopped")
}
```

### Step 2: Run It

```bash
cd /path/to/minion
go run examples/tutorial0/main.go
```

**Expected Output:**
```
Created agent: worker-1
Capabilities: [general]
Agent is running!
Agent stopped
```

### What You Learned
- ✅ How to create a worker agent
- ✅ How to start and stop agents
- ✅ Basic agent metadata

**Next:** [Tutorial 1: Orchestrator and Workers](#tutorial-1-orchestrator-and-workers)

---

## Tutorial 1: Orchestrator and Workers

**Time: 15 minutes**
**Goal: Create an orchestrator that manages worker agents**

### Step 1: Create the System

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/Ranganaths/minion/core/multiagent"
)

func main() {
    ctx := context.Background()

    // Create in-memory protocol for communication
    protocol := multiagent.NewInMemoryProtocol(nil)

    // Create in-memory ledger for task tracking
    ledger := multiagent.NewInMemoryLedger()

    // Create orchestrator
    orchestrator := multiagent.NewOrchestratorAgent(
        "orchestrator-1",
        protocol,
        ledger,
    )

    // Create 3 worker agents
    workers := make([]*multiagent.WorkerAgent, 3)
    for i := 0; i < 3; i++ {
        workerID := fmt.Sprintf("worker-%d", i+1)
        workers[i] = multiagent.NewWorkerAgent(
            workerID,
            []string{"general"},
            protocol,
            ledger,
        )
    }

    // Start orchestrator
    if err := orchestrator.Start(ctx); err != nil {
        log.Fatal(err)
    }
    fmt.Println("Orchestrator started")

    // Start workers
    for _, worker := range workers {
        if err := worker.Start(ctx); err != nil {
            log.Fatal(err)
        }
        fmt.Printf("Started %s\n", worker.GetMetadata().AgentID)
    }

    // Register workers with orchestrator
    for _, worker := range workers {
        orchestrator.RegisterWorker(worker)
    }
    fmt.Printf("Registered %d workers\n", len(workers))

    // Create a simple task
    task := &multiagent.Task{
        ID:          "task-1",
        Name:        "Hello Task",
        Description: "A simple test task",
        Type:        "general",
        Priority:    multiagent.PriorityMedium,
        Input: map[string]interface{}{
            "message": "Hello from orchestrator!",
        },
    }

    // Execute task through orchestrator
    fmt.Printf("Executing task: %s\n", task.Name)
    result, err := orchestrator.ExecuteTask(ctx, task)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Task completed! Result: %v\n", result)

    // Cleanup
    for _, worker := range workers {
        worker.Stop(ctx)
    }
    orchestrator.Stop(ctx)
}
```

### Step 2: Run It

```bash
go run examples/tutorial1/main.go
```

**Expected Output:**
```
Orchestrator started
Started worker-1
Started worker-2
Started worker-3
Registered 3 workers
Executing task: Hello Task
Task completed! Result: map[status:success worker:worker-1]
```

### What You Learned
- ✅ How to create an orchestrator
- ✅ How to register workers with an orchestrator
- ✅ How to execute tasks through the orchestrator
- ✅ Basic task structure

**Next:** [Tutorial 2: Task Workflows](#tutorial-2-task-workflows)

---

## Tutorial 2: Task Workflows

**Time: 20 minutes**
**Goal: Create multi-step workflows with dependencies**

### Step 1: Define a Workflow

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/Ranganaths/minion/core/multiagent"
)

func main() {
    ctx := context.Background()

    // Setup (same as Tutorial 1)
    protocol := multiagent.NewInMemoryProtocol(nil)
    ledger := multiagent.NewInMemoryLedger()
    orchestrator := multiagent.NewOrchestratorAgent("orchestrator-1", protocol, ledger)

    // Create workers with different capabilities
    dataWorker := multiagent.NewWorkerAgent("data-worker", []string{"data"}, protocol, ledger)
    processWorker := multiagent.NewWorkerAgent("process-worker", []string{"processing"}, protocol, ledger)
    reportWorker := multiagent.NewWorkerAgent("report-worker", []string{"reporting"}, protocol, ledger)

    // Start all agents
    orchestrator.Start(ctx)
    dataWorker.Start(ctx)
    processWorker.Start(ctx)
    reportWorker.Start(ctx)

    orchestrator.RegisterWorker(dataWorker)
    orchestrator.RegisterWorker(processWorker)
    orchestrator.RegisterWorker(reportWorker)

    // Create a workflow with dependencies
    workflow := &multiagent.Workflow{
        ID:   "data-pipeline-1",
        Name: "Data Processing Pipeline",
        Tasks: []*multiagent.Task{
            {
                ID:          "task-1-fetch",
                Name:        "Fetch Data",
                Type:        "data",
                Priority:    multiagent.PriorityHigh,
                Input:       map[string]interface{}{"source": "database"},
            },
            {
                ID:          "task-2-process",
                Name:        "Process Data",
                Type:        "processing",
                Priority:    multiagent.PriorityMedium,
                Dependencies: []string{"task-1-fetch"}, // Depends on task 1
                Input:       map[string]interface{}{"operation": "transform"},
            },
            {
                ID:          "task-3-report",
                Name:        "Generate Report",
                Type:        "reporting",
                Priority:    multiagent.PriorityMedium,
                Dependencies: []string{"task-2-process"}, // Depends on task 2
                Input:       map[string]interface{}{"format": "pdf"},
            },
        },
    }

    // Execute workflow
    fmt.Printf("Starting workflow: %s\n", workflow.Name)
    fmt.Println("Tasks will execute in order based on dependencies...")

    startTime := time.Now()
    if err := orchestrator.ExecuteWorkflow(ctx, workflow); err != nil {
        log.Fatal(err)
    }
    duration := time.Since(startTime)

    fmt.Printf("Workflow completed in %v\n", duration)

    // Get task details from ledger
    for _, task := range workflow.Tasks {
        taskDetails, _ := ledger.GetTask(ctx, task.ID)
        fmt.Printf("  %s: %s (assigned to: %s)\n",
            taskDetails.ID,
            taskDetails.Status,
            taskDetails.AssignedTo)
    }

    // Cleanup
    dataWorker.Stop(ctx)
    processWorker.Stop(ctx)
    reportWorker.Stop(ctx)
    orchestrator.Stop(ctx)
}
```

### Step 2: Visualize Dependencies

```
Task Flow:
┌─────────────┐
│ Fetch Data  │
│  (task-1)   │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│Process Data │
│  (task-2)   │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│Generate Rpt │
│  (task-3)   │
└─────────────┘
```

### Step 3: Run It

```bash
go run examples/tutorial2/main.go
```

**Expected Output:**
```
Starting workflow: Data Processing Pipeline
Tasks will execute in order based on dependencies...
Workflow completed in 342ms
  task-1-fetch: completed (assigned to: data-worker)
  task-2-process: completed (assigned to: process-worker)
  task-3-report: completed (assigned to: report-worker)
```

### What You Learned
- ✅ How to create workflows with multiple tasks
- ✅ How to define task dependencies
- ✅ How workers are matched by capability
- ✅ How to track task execution through the ledger

**Next:** [Tutorial 3: Message Communication](#tutorial-3-message-communication)

---

## Tutorial 3: Message Communication

**Time: 20 minutes**
**Goal: Learn different messaging patterns**

### Pattern 1: Direct Messaging

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/Ranganaths/minion/core/multiagent"
)

func main() {
    ctx := context.Background()
    protocol := multiagent.NewInMemoryProtocol(nil)

    // Create two agents
    agent1 := multiagent.NewWorkerAgent("agent-1", []string{"general"}, protocol, nil)
    agent2 := multiagent.NewWorkerAgent("agent-2", []string{"general"}, protocol, nil)

    agent1.Start(ctx)
    agent2.Start(ctx)

    // Agent 1 sends direct message to Agent 2
    msg := &multiagent.Message{
        Type:    multiagent.MessageTypeCustom,
        From:    "agent-1",
        To:      "agent-2",
        Payload: map[string]interface{}{
            "text": "Hello Agent 2!",
        },
    }

    fmt.Println("Agent 1 sending message...")
    if err := protocol.Send(ctx, msg); err != nil {
        panic(err)
    }

    // Agent 2 receives message
    time.Sleep(100 * time.Millisecond)
    messages, err := protocol.Receive(ctx, "agent-2")
    if err != nil {
        panic(err)
    }

    for _, msg := range messages {
        fmt.Printf("Agent 2 received: %v\n", msg.Payload)
    }

    agent1.Stop(ctx)
    agent2.Stop(ctx)
}
```

### Pattern 2: Publish-Subscribe

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/Ranganaths/minion/core/multiagent"
)

func main() {
    ctx := context.Background()
    protocol := multiagent.NewInMemoryProtocol(nil)

    // Create publisher and multiple subscribers
    publisher := multiagent.NewWorkerAgent("publisher", []string{"general"}, protocol, nil)
    sub1 := multiagent.NewWorkerAgent("subscriber-1", []string{"general"}, protocol, nil)
    sub2 := multiagent.NewWorkerAgent("subscriber-2", []string{"general"}, protocol, nil)
    sub3 := multiagent.NewWorkerAgent("subscriber-3", []string{"general"}, protocol, nil)

    publisher.Start(ctx)
    sub1.Start(ctx)
    sub2.Start(ctx)
    sub3.Start(ctx)

    // Subscribers subscribe to event type
    eventType := multiagent.MessageType("data.updated")
    protocol.Subscribe(ctx, "subscriber-1", eventType)
    protocol.Subscribe(ctx, "subscriber-2", eventType)
    protocol.Subscribe(ctx, "subscriber-3", eventType)

    fmt.Println("3 subscribers registered for 'data.updated' events")

    // Publisher broadcasts event
    event := &multiagent.Message{
        Type: eventType,
        From: "publisher",
        Payload: map[string]interface{}{
            "event": "New data available",
            "count": 42,
        },
    }

    fmt.Println("Publisher broadcasting event...")
    protocol.Broadcast(ctx, event)

    // All subscribers receive the event
    time.Sleep(100 * time.Millisecond)

    for _, subID := range []string{"subscriber-1", "subscriber-2", "subscriber-3"} {
        messages, _ := protocol.Receive(ctx, subID)
        for _, msg := range messages {
            fmt.Printf("%s received: %v\n", subID, msg.Payload)
        }
    }

    // Cleanup
    publisher.Stop(ctx)
    sub1.Stop(ctx)
    sub2.Stop(ctx)
    sub3.Stop(ctx)
}
```

### Pattern 3: Request-Reply

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/Ranganaths/minion/core/multiagent"
)

func main() {
    ctx := context.Background()
    protocol := multiagent.NewInMemoryProtocol(nil)

    requester := multiagent.NewWorkerAgent("requester", []string{"general"}, protocol, nil)
    responder := multiagent.NewWorkerAgent("responder", []string{"general"}, protocol, nil)

    requester.Start(ctx)
    responder.Start(ctx)

    // Responder listens for requests
    go func() {
        for {
            messages, _ := protocol.Receive(ctx, "responder")
            for _, msg := range messages {
                if msg.Type == multiagent.MessageTypeRequest {
                    fmt.Printf("Responder received request: %v\n", msg.Payload)

                    // Send reply
                    reply := &multiagent.Message{
                        Type: multiagent.MessageTypeReply,
                        From: "responder",
                        To:   msg.From,
                        Payload: map[string]interface{}{
                            "result": "Request processed successfully",
                            "data":   []int{1, 2, 3, 4, 5},
                        },
                        CorrelationID: msg.ID,
                    }
                    protocol.Send(ctx, reply)
                }
            }
            time.Sleep(50 * time.Millisecond)
        }
    }()

    // Requester sends request
    request := &multiagent.Message{
        Type: multiagent.MessageTypeRequest,
        From: "requester",
        To:   "responder",
        Payload: map[string]interface{}{
            "action": "get_data",
        },
    }

    fmt.Println("Requester sending request...")
    protocol.Send(ctx, request)

    // Wait for reply
    time.Sleep(200 * time.Millisecond)
    messages, _ := protocol.Receive(ctx, "requester")
    for _, msg := range messages {
        if msg.Type == multiagent.MessageTypeReply {
            fmt.Printf("Requester received reply: %v\n", msg.Payload)
        }
    }

    requester.Stop(ctx)
    responder.Stop(ctx)
}
```

### What You Learned
- ✅ Direct messaging between agents
- ✅ Publish-subscribe for broadcasting events
- ✅ Request-reply for synchronous-style communication
- ✅ Message types and correlation IDs

**Next:** [Tutorial 4: Adding Persistence](#tutorial-4-adding-persistence)

---

## Tutorial 4: Adding Persistence

**Time: 25 minutes**
**Goal: Store tasks and state in PostgreSQL**

### Step 1: Setup PostgreSQL

```bash
# Using Docker
docker run --name minion-postgres \
  -e POSTGRES_PASSWORD=minion123 \
  -e POSTGRES_USER=minion \
  -e POSTGRES_DB=minion \
  -p 5432:5432 \
  -d postgres:15
```

### Step 2: Run Migrations

```bash
# Install migrate tool
brew install golang-migrate

# Run migrations
migrate -path migrations -database "postgresql://minion:minion123@localhost:5432/minion?sslmode=disable" up
```

### Step 3: Use PostgreSQL Ledger

```go
package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"

    _ "github.com/lib/pq"
    "github.com/Ranganaths/minion/core/multiagent"
)

func main() {
    ctx := context.Background()

    // Connect to PostgreSQL
    connStr := "postgresql://minion:minion123@localhost:5432/minion?sslmode=disable"
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Test connection
    if err := db.Ping(); err != nil {
        log.Fatal("Cannot connect to database:", err)
    }
    fmt.Println("Connected to PostgreSQL")

    // Create PostgreSQL ledger
    ledgerConfig := &multiagent.PostgresLedgerConfig{
        MaxConnections:     10,
        MaxIdleConnections: 5,
        ConnMaxLifetime:    3600,
    }
    ledger := multiagent.NewPostgresLedger(db, ledgerConfig)

    // Initialize ledger
    if err := ledger.Initialize(ctx); err != nil {
        log.Fatal("Failed to initialize ledger:", err)
    }
    fmt.Println("Ledger initialized")

    // Create task
    task := &multiagent.Task{
        ID:          "persistent-task-1",
        Name:        "Persistent Task",
        Description: "This task is stored in PostgreSQL",
        Type:        "general",
        Priority:    multiagent.PriorityHigh,
        CreatedBy:   "tutorial",
        Status:      multiagent.TaskStatusPending,
        Input: map[string]interface{}{
            "data": "important information",
        },
        Metadata: map[string]interface{}{
            "customer": "acme-corp",
            "region":   "us-west",
        },
    }

    // Save task to database
    if err := ledger.CreateTask(ctx, task); err != nil {
        log.Fatal("Failed to create task:", err)
    }
    fmt.Printf("Task created: %s\n", task.ID)

    // Update task status
    task.Status = multiagent.TaskStatusRunning
    if err := ledger.UpdateTask(ctx, task); err != nil {
        log.Fatal("Failed to update task:", err)
    }
    fmt.Println("Task status updated to: running")

    // Record progress
    progress := &multiagent.ProgressUpdate{
        TaskID:    task.ID,
        Progress:  0.5,
        Message:   "Processing 50% complete",
        Timestamp: time.Now(),
    }
    if err := ledger.RecordProgress(ctx, progress); err != nil {
        log.Fatal("Failed to record progress:", err)
    }
    fmt.Println("Progress recorded: 50%")

    // Complete task
    result := map[string]interface{}{
        "status":  "success",
        "records": 1000,
    }
    if err := ledger.CompleteTask(ctx, task.ID, result); err != nil {
        log.Fatal("Failed to complete task:", err)
    }
    fmt.Println("Task completed")

    // Retrieve task from database
    retrieved, err := ledger.GetTask(ctx, task.ID)
    if err != nil {
        log.Fatal("Failed to get task:", err)
    }

    fmt.Printf("\nRetrieved task from database:\n")
    fmt.Printf("  ID: %s\n", retrieved.ID)
    fmt.Printf("  Name: %s\n", retrieved.Name)
    fmt.Printf("  Status: %s\n", retrieved.Status)
    fmt.Printf("  Result: %v\n", retrieved.Result)

    // List all tasks
    filter := &multiagent.TaskFilter{
        Status: multiagent.TaskStatusCompleted,
    }
    tasks, err := ledger.ListTasks(ctx, filter)
    if err != nil {
        log.Fatal("Failed to list tasks:", err)
    }
    fmt.Printf("\nCompleted tasks in database: %d\n", len(tasks))

    // Get statistics
    stats, err := ledger.Stats(ctx)
    if err != nil {
        log.Fatal("Failed to get stats:", err)
    }
    fmt.Printf("\nLedger Statistics:\n")
    fmt.Printf("  Total tasks: %d\n", stats.TotalTasks)
    fmt.Printf("  Completed: %d\n", stats.CompletedTasks)
    fmt.Printf("  Failed: %d\n", stats.FailedTasks)
}
```

### Step 4: Run It

```bash
go run examples/tutorial4/main.go
```

**Expected Output:**
```
Connected to PostgreSQL
Ledger initialized
Task created: persistent-task-1
Task status updated to: running
Progress recorded: 50%
Task completed

Retrieved task from database:
  ID: persistent-task-1
  Name: Persistent Task
  Status: completed
  Result: map[records:1000 status:success]

Completed tasks in database: 1

Ledger Statistics:
  Total tasks: 1
  Completed: 1
  Failed: 0
```

### What You Learned
- ✅ How to connect to PostgreSQL
- ✅ How to use PostgresLedger for persistence
- ✅ How to track task lifecycle in database
- ✅ How to query tasks and statistics

**Next:** [Tutorial 5: Distributed Deployment](#tutorial-5-distributed-deployment)

---

## Tutorial 5: Distributed Deployment

**Time: 30 minutes**
**Goal: Deploy multi-server system with Redis**

### Step 1: Setup Redis

```bash
docker run --name minion-redis \
  -p 6379:6379 \
  -d redis:7-alpine
```

### Step 2: Create Orchestrator Server

**File: `cmd/orchestrator/main.go`**

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/redis/go-redis/v9"
    "github.com/Ranganaths/minion/core/multiagent"
)

func main() {
    ctx := context.Background()

    // Connect to Redis
    redisClient := redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "",
        DB:       0,
    })

    if err := redisClient.Ping(ctx).Err(); err != nil {
        log.Fatal("Cannot connect to Redis:", err)
    }
    log.Println("Connected to Redis")

    // Create Redis protocol
    protocolConfig := &multiagent.RedisProtocolConfig{
        ConsumerGroup: "minion-group",
        StreamMaxLen:  10000,
        BatchSize:     10,
        BlockTimeout:  5000,
    }
    protocol := multiagent.NewRedisProtocol(redisClient, protocolConfig)

    // Create in-memory ledger (or PostgreSQL)
    ledger := multiagent.NewInMemoryLedger()

    // Create orchestrator
    orchestrator := multiagent.NewOrchestratorAgent(
        "orchestrator-main",
        protocol,
        ledger,
    )

    // Start orchestrator
    if err := orchestrator.Start(ctx); err != nil {
        log.Fatal(err)
    }
    log.Println("Orchestrator started and listening for workers...")

    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    log.Println("Shutting down orchestrator...")
    orchestrator.Stop(ctx)
    redisClient.Close()
}
```

### Step 3: Create Worker Server

**File: `cmd/worker/main.go`**

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/redis/go-redis/v9"
    "github.com/Ranganaths/minion/core/multiagent"
)

func main() {
    ctx := context.Background()

    // Get worker ID from environment or generate
    workerID := os.Getenv("WORKER_ID")
    if workerID == "" {
        workerID = fmt.Sprintf("worker-%d", os.Getpid())
    }

    // Get capabilities from environment
    capabilities := []string{"general"}
    if cap := os.Getenv("WORKER_CAPABILITIES"); cap != "" {
        capabilities = []string{cap}
    }

    // Connect to Redis
    redisClient := redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "",
        DB:       0,
    })

    if err := redisClient.Ping(ctx).Err(); err != nil {
        log.Fatal("Cannot connect to Redis:", err)
    }
    log.Printf("Worker %s connected to Redis\n", workerID)

    // Create Redis protocol
    protocolConfig := &multiagent.RedisProtocolConfig{
        ConsumerGroup: "minion-group",
        StreamMaxLen:  10000,
        BatchSize:     10,
        BlockTimeout:  5000,
    }
    protocol := multiagent.NewRedisProtocol(redisClient, protocolConfig)

    // Create worker
    worker := multiagent.NewWorkerAgent(
        workerID,
        capabilities,
        protocol,
        nil,
    )

    // Start worker
    if err := worker.Start(ctx); err != nil {
        log.Fatal(err)
    }
    log.Printf("Worker %s started with capabilities: %v\n", workerID, capabilities)

    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    log.Printf("Shutting down worker %s...\n", workerID)
    worker.Stop(ctx)
    redisClient.Close()
}
```

### Step 4: Run Distributed System

**Terminal 1 - Orchestrator:**
```bash
go run cmd/orchestrator/main.go
```

**Terminal 2 - Worker 1:**
```bash
WORKER_ID=worker-1 WORKER_CAPABILITIES=data go run cmd/worker/main.go
```

**Terminal 3 - Worker 2:**
```bash
WORKER_ID=worker-2 WORKER_CAPABILITIES=processing go run cmd/worker/main.go
```

**Terminal 4 - Worker 3:**
```bash
WORKER_ID=worker-3 WORKER_CAPABILITIES=reporting go run cmd/worker/main.go
```

### Step 5: Submit Tasks

**File: `cmd/client/main.go`**

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/redis/go-redis/v9"
    "github.com/Ranganaths/minion/core/multiagent"
)

func main() {
    ctx := context.Background()

    // Connect to Redis
    redisClient := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    protocol := multiagent.NewRedisProtocol(redisClient, &multiagent.RedisProtocolConfig{
        ConsumerGroup: "minion-group",
    })

    // Create task
    task := &multiagent.Task{
        ID:       "distributed-task-1",
        Name:     "Process Data",
        Type:     "processing",
        Priority: multiagent.PriorityHigh,
        Input: map[string]interface{}{
            "records": 1000,
        },
    }

    // Send task to orchestrator
    msg := &multiagent.Message{
        Type:    multiagent.MessageTypeTaskAssignment,
        To:      "orchestrator-main",
        Payload: task,
    }

    if err := protocol.Send(ctx, msg); err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Task submitted: %s\n", task.ID)
}
```

**Terminal 5 - Client:**
```bash
go run cmd/client/main.go
```

### What You Learned
- ✅ How to use Redis for distributed messaging
- ✅ How to run orchestrator and workers on different machines
- ✅ How to scale workers independently
- ✅ How to submit tasks from clients

**Next:** [Tutorial 6: Auto-Scaling Workers](#tutorial-6-auto-scaling-workers)

---

## Tutorial 6: Auto-Scaling Workers

**Time: 30 minutes**
**Goal: Automatically scale workers based on load**

### Step 1: Create Auto-Scaling System

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/Ranganaths/minion/core/multiagent"
)

func main() {
    ctx := context.Background()

    // Setup
    protocol := multiagent.NewInMemoryProtocol(nil)
    ledger := multiagent.NewInMemoryLedger()

    // Create worker pool
    workerPool := multiagent.NewWorkerPool(protocol, ledger)

    // Create auto-scaler with policy
    policy := &multiagent.ScalingPolicy{
        // Scale up triggers
        MaxQueueDepth:      50,
        MaxUtilization:     0.80,
        MinIdleWorkers:     2,
        ScaleUpThreshold:   3, // Need 3 consecutive high-load checks

        // Scale down triggers
        MinQueueDepth:      10,
        MinUtilization:     0.30,
        ScaleDownThreshold: 5, // Need 5 consecutive low-load checks

        // Limits
        MinWorkers:         2,
        MaxWorkers:         10,

        // Cooldowns
        ScaleUpCooldown:    2 * time.Minute,
        ScaleDownCooldown:  5 * time.Minute,

        // Increments
        ScaleUpIncrement:   2,
        ScaleDownIncrement: 1,
    }

    autoscaler := multiagent.NewAutoscaler(workerPool, policy)

    // Start with minimum workers
    fmt.Printf("Starting with %d workers\n", policy.MinWorkers)
    for i := 0; i < policy.MinWorkers; i++ {
        workerPool.AddWorker(ctx, "general")
    }

    // Start auto-scaler
    go autoscaler.Start(ctx, 30*time.Second)

    // Simulate varying load
    fmt.Println("\nSimulating load patterns...")

    scenarios := []struct {
        name       string
        queueDepth int
        duration   time.Duration
    }{
        {"Low load", 5, 2 * time.Minute},
        {"High load spike", 60, 3 * time.Minute},
        {"Medium load", 25, 2 * time.Minute},
        {"Very high load", 100, 3 * time.Minute},
        {"Back to low", 5, 2 * time.Minute},
    }

    for _, scenario := range scenarios {
        fmt.Printf("\n=== %s (queue: %d) ===\n", scenario.name, scenario.queueDepth)

        // Simulate load
        simulateLoad(ctx, workerPool, scenario.queueDepth)

        // Monitor scaling
        monitorScaling(ctx, workerPool, autoscaler, scenario.duration)
    }
}

func simulateLoad(ctx context.Context, pool *multiagent.WorkerPool, queueDepth int) {
    // This would be actual task queue in real system
    // For simulation, we just set the metric
    stats := pool.GetStats()
    stats.QueueDepth = queueDepth
}

func monitorScaling(ctx context.Context, pool *multiagent.WorkerPool, scaler *multiagent.Autoscaler, duration time.Duration) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    timeout := time.After(duration)

    for {
        select {
        case <-ticker.C:
            stats := pool.GetStats()
            decision := scaler.GetLastDecision()

            fmt.Printf("[%s] Workers: %d, Queue: %d, Util: %.2f%%, Action: %s\n",
                time.Now().Format("15:04:05"),
                stats.TotalWorkers,
                stats.QueueDepth,
                stats.Utilization*100,
                decision.Action,
            )

        case <-timeout:
            return
        }
    }
}
```

### Step 2: Run and Observe

```bash
go run examples/tutorial6/main.go
```

**Expected Output:**
```
Starting with 2 workers

Simulating load patterns...

=== Low load (queue: 5) ===
[10:00:00] Workers: 2, Queue: 5, Util: 25.00%, Action: none

=== High load spike (queue: 60) ===
[10:00:30] Workers: 2, Queue: 60, Util: 95.00%, Action: none
[10:01:00] Workers: 2, Queue: 60, Util: 95.00%, Action: none
[10:01:30] Workers: 4, Queue: 60, Util: 75.00%, Action: scale_up
[10:02:00] Workers: 4, Queue: 60, Util: 75.00%, Action: none

=== Medium load (queue: 25) ===
[10:02:30] Workers: 4, Queue: 25, Util: 50.00%, Action: none

=== Very high load (queue: 100) ===
[10:03:00] Workers: 4, Queue: 100, Util: 98.00%, Action: none
[10:03:30] Workers: 6, Queue: 100, Util: 82.00%, Action: scale_up
[10:04:00] Workers: 8, Queue: 100, Util: 65.00%, Action: scale_up

=== Back to low (queue: 5) ===
[10:05:00] Workers: 8, Queue: 5, Util: 12.00%, Action: none
[10:07:30] Workers: 7, Queue: 5, Util: 14.00%, Action: scale_down
```

### What You Learned
- ✅ How to configure scaling policies
- ✅ How threshold-based scaling prevents flapping
- ✅ How cooldown periods work
- ✅ How to monitor scaling decisions

**Next:** [Tutorial 7: Load Balancing Strategies](#tutorial-7-load-balancing-strategies)

---

## Tutorial 7: Load Balancing Strategies

**Time: 30 minutes**
**Goal: Compare different load balancing strategies**

### Step 1: Benchmark Different Strategies

```go
package main

import (
    "context"
    "fmt"
    "math/rand"
    "time"

    "github.com/Ranganaths/minion/core/multiagent"
)

func main() {
    ctx := context.Background()

    // Create workers with varying performance
    workers := createHeterogeneousWorkers()

    // Test each strategy
    strategies := []multiagent.LoadBalancerStrategy{
        multiagent.StrategyRoundRobin,
        multiagent.StrategyLeastLoaded,
        multiagent.StrategyRandom,
        multiagent.StrategyCapabilityBest,
        multiagent.StrategyLatencyBased,
    }

    fmt.Println("Load Balancing Strategy Comparison\n")
    fmt.Println("Workers:")
    for _, w := range workers {
        fmt.Printf("  %s: capabilities=%v, performance=%.1fx\n",
            w.GetMetadata().AgentID,
            w.GetMetadata().Capabilities,
            w.performanceMultiplier,
        )
    }
    fmt.Println()

    results := make(map[multiagent.LoadBalancerStrategy]*BenchmarkResult)

    for _, strategy := range strategies {
        result := benchmarkStrategy(ctx, strategy, workers, 100)
        results[strategy] = result

        fmt.Printf("Strategy: %s\n", strategy)
        fmt.Printf("  Avg Latency: %v\n", result.AvgLatency)
        fmt.Printf("  Total Time: %v\n", result.TotalTime)
        fmt.Printf("  Worker Utilization:\n")
        for workerID, count := range result.TaskDistribution {
            util := float64(count) / float64(result.TotalTasks) * 100
            fmt.Printf("    %s: %d tasks (%.1f%%)\n", workerID, count, util)
        }
        fmt.Println()
    }

    // Find best strategy
    var bestStrategy multiagent.LoadBalancerStrategy
    bestLatency := time.Duration(1<<63 - 1)

    for strategy, result := range results {
        if result.AvgLatency < bestLatency {
            bestLatency = result.AvgLatency
            bestStrategy = strategy
        }
    }

    fmt.Printf("Best strategy: %s (avg latency: %v)\n", bestStrategy, bestLatency)
}

type BenchmarkResult struct {
    Strategy         multiagent.LoadBalancerStrategy
    TotalTasks       int
    TotalTime        time.Duration
    AvgLatency       time.Duration
    TaskDistribution map[string]int
}

func benchmarkStrategy(ctx context.Context, strategy multiagent.LoadBalancerStrategy, workers []*WorkerAgent, taskCount int) *BenchmarkResult {
    config := &multiagent.LoadBalancerConfig{
        Strategy:                  strategy,
        EnablePerformanceTracking: true,
        TrackingWindowSize:        100,
    }
    factory := multiagent.NewLoadBalancerFactory(config)
    balancer := factory.CreateLoadBalancer()

    result := &BenchmarkResult{
        Strategy:         strategy,
        TotalTasks:       taskCount,
        TaskDistribution: make(map[string]int),
    }

    start := time.Now()
    var totalLatency time.Duration

    for i := 0; i < taskCount; i++ {
        task := &multiagent.Task{
            ID:   fmt.Sprintf("task-%d", i),
            Type: "general",
        }

        // Select worker
        worker, err := balancer.SelectWorker(ctx, task, workers)
        if err != nil {
            continue
        }

        // Simulate task execution
        taskStart := time.Now()
        executionTime := simulateTaskExecution(worker)
        latency := time.Since(taskStart)

        totalLatency += latency
        result.TaskDistribution[worker.GetMetadata().AgentID]++

        // Record result for learning
        balancer.RecordResult(worker.GetMetadata().AgentID, task, executionTime, nil)
    }

    result.TotalTime = time.Since(start)
    result.AvgLatency = totalLatency / time.Duration(taskCount)

    return result
}

func createHeterogeneousWorkers() []*WorkerAgent {
    protocol := multiagent.NewInMemoryProtocol(nil)

    return []*WorkerAgent{
        {
            metadata: &multiagent.AgentMetadata{
                AgentID:      "worker-fast",
                Capabilities: []string{"general"},
                Status:       multiagent.StatusIdle,
            },
            protocol:              protocol,
            performanceMultiplier: 2.0, // Fast worker
        },
        {
            metadata: &multiagent.AgentMetadata{
                AgentID:      "worker-medium-1",
                Capabilities: []string{"general"},
                Status:       multiagent.StatusIdle,
            },
            protocol:              protocol,
            performanceMultiplier: 1.0, // Normal speed
        },
        {
            metadata: &multiagent.AgentMetadata{
                AgentID:      "worker-medium-2",
                Capabilities: []string{"general"},
                Status:       multiagent.StatusIdle,
            },
            protocol:              protocol,
            performanceMultiplier: 1.0,
        },
        {
            metadata: &multiagent.AgentMetadata{
                AgentID:      "worker-slow",
                Capabilities: []string{"general"},
                Status:       multiagent.StatusIdle,
            },
            protocol:              protocol,
            performanceMultiplier: 0.5, // Slow worker
        },
    }
}

func simulateTaskExecution(worker *WorkerAgent) time.Duration {
    baseTime := 100 * time.Millisecond
    executionTime := time.Duration(float64(baseTime) / worker.performanceMultiplier)

    // Add some randomness
    jitter := time.Duration(rand.Intn(20)-10) * time.Millisecond
    executionTime += jitter

    time.Sleep(executionTime)
    return executionTime
}
```

### Step 2: Run Benchmark

```bash
go run examples/tutorial7/main.go
```

**Expected Output:**
```
Load Balancing Strategy Comparison

Workers:
  worker-fast: capabilities=[general], performance=2.0x
  worker-medium-1: capabilities=[general], performance=1.0x
  worker-medium-2: capabilities=[general], performance=1.0x
  worker-slow: capabilities=[general], performance=0.5x

Strategy: round_robin
  Avg Latency: 125ms
  Total Time: 12.5s
  Worker Utilization:
    worker-fast: 25 tasks (25.0%)
    worker-medium-1: 25 tasks (25.0%)
    worker-medium-2: 25 tasks (25.0%)
    worker-slow: 25 tasks (25.0%)

Strategy: least_loaded
  Avg Latency: 118ms
  Total Time: 11.8s
  Worker Utilization:
    worker-fast: 28 tasks (28.0%)
    worker-medium-1: 24 tasks (24.0%)
    worker-medium-2: 24 tasks (24.0%)
    worker-slow: 24 tasks (24.0%)

Strategy: capability_best
  Avg Latency: 95ms
  Total Time: 9.5s
  Worker Utilization:
    worker-fast: 45 tasks (45.0%)
    worker-medium-1: 22 tasks (22.0%)
    worker-medium-2: 23 tasks (23.0%)
    worker-slow: 10 tasks (10.0%)

Strategy: latency_based
  Avg Latency: 88ms
  Total Time: 8.8s
  Worker Utilization:
    worker-fast: 52 tasks (52.0%)
    worker-medium-1: 20 tasks (20.0%)
    worker-medium-2: 20 tasks (20.0%)
    worker-slow: 8 tasks (8.0%)

Best strategy: latency_based (avg latency: 88ms)
```

### What You Learned
- ✅ How different strategies distribute work
- ✅ When to use each strategy
- ✅ How performance tracking improves routing
- ✅ How to benchmark and compare strategies

**Next:** [Tutorial 8: Adding Resilience](#tutorial-8-adding-resilience)

---

## Tutorial 8: Adding Resilience

**Time: 30 minutes**
**Goal: Add retry, timeout, and circuit breaker patterns**

### Step 1: Retry with Backoff

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/Ranganaths/minion/core/multiagent/resilience"
)

func main() {
    ctx := context.Background()

    // Configure retry policy
    retryConfig := &resilience.RetryConfig{
        MaxAttempts:   5,
        InitialDelay:  100 * time.Millisecond,
        MaxDelay:      2 * time.Second,
        BackoffFactor: 2.0,
        Jitter:        true,
    }

    // Simulate unreliable operation
    attemptCount := 0
    unreliableOp := func() (string, error) {
        attemptCount++
        fmt.Printf("Attempt %d...\n", attemptCount)

        if attemptCount < 4 {
            return "", errors.New("temporary failure")
        }

        return "Success!", nil
    }

    // Execute with retry
    fmt.Println("Executing unreliable operation with retry...")
    result, err := resilience.RetryWithBackoff(ctx, retryConfig, unreliableOp)

    if err != nil {
        fmt.Printf("Failed after %d attempts: %v\n", attemptCount, err)
    } else {
        fmt.Printf("Succeeded on attempt %d: %s\n", attemptCount, result)
    }
}
```

### Step 2: Timeout Pattern

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/Ranganaths/minion/core/multiagent/resilience"
)

func main() {
    ctx := context.Background()

    // Fast operation (completes in time)
    fmt.Println("Testing fast operation...")
    fastOp := func(ctx context.Context) (string, error) {
        time.Sleep(100 * time.Millisecond)
        return "Fast result", nil
    }

    result, err := resilience.WithTimeout(ctx, 500*time.Millisecond, fastOp)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    } else {
        fmt.Printf("Success: %s\n", result)
    }

    // Slow operation (times out)
    fmt.Println("\nTesting slow operation...")
    slowOp := func(ctx context.Context) (string, error) {
        time.Sleep(2 * time.Second)
        return "Slow result", nil
    }

    result, err = resilience.WithTimeout(ctx, 500*time.Millisecond, slowOp)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    } else {
        fmt.Printf("Success: %s\n", result)
    }
}
```

### Step 3: Circuit Breaker

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/Ranganaths/minion/core/multiagent/resilience"
)

func main() {
    ctx := context.Background()

    // Create circuit breaker
    cb := resilience.NewCircuitBreaker(
        "external-api",
        5,                 // Max failures before opening
        30*time.Second,    // Timeout before trying again
        60*time.Second,    // Reset timeout
    )

    // Simulate failing service
    callCount := 0
    externalAPI := func() error {
        callCount++

        // Fail first 10 calls
        if callCount <= 10 {
            return errors.New("service unavailable")
        }

        return nil
    }

    // Make calls through circuit breaker
    for i := 1; i <= 15; i++ {
        fmt.Printf("\nCall %d: ", i)

        err := cb.Execute(externalAPI)

        if err != nil {
            fmt.Printf("Failed - %v\n", err)
        } else {
            fmt.Printf("Success\n")
        }

        fmt.Printf("Circuit state: %s, Failures: %d\n",
            cb.State(),
            cb.FailureCount())

        time.Sleep(100 * time.Millisecond)
    }
}
```

### Step 4: Combine All Resilience Patterns

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "math/rand"
    "time"

    "github.com/Ranganaths/minion/core/multiagent/resilience"
)

func main() {
    ctx := context.Background()

    // Create circuit breaker
    cb := resilience.NewCircuitBreaker("db", 3, 5*time.Second, 10*time.Second)

    // Retry configuration
    retryConfig := &resilience.RetryConfig{
        MaxAttempts:   3,
        InitialDelay:  100 * time.Millisecond,
        MaxDelay:      1 * time.Second,
        BackoffFactor: 2.0,
        Jitter:        true,
    }

    // Simulate database operation
    dbQuery := func(ctx context.Context) ([]string, error) {
        // Random failure (30% chance)
        if rand.Float64() < 0.3 {
            return nil, errors.New("db connection timeout")
        }

        // Simulate query time
        time.Sleep(50 * time.Millisecond)

        return []string{"record1", "record2", "record3"}, nil
    }

    // Resilient execution combining all patterns
    fmt.Println("Executing resilient database queries...")

    for i := 1; i <= 10; i++ {
        fmt.Printf("\nQuery %d: ", i)

        // Execute with all resilience patterns
        err := cb.Execute(func() error {
            _, err := resilience.RetryWithBackoff(ctx, retryConfig, func() ([]string, error) {
                return resilience.WithTimeout(ctx, 200*time.Millisecond, dbQuery)
            })
            return err
        })

        if err != nil {
            fmt.Printf("Failed - %v\n", err)
        } else {
            fmt.Printf("Success\n")
        }

        time.Sleep(200 * time.Millisecond)
    }
}
```

### What You Learned
- ✅ How to implement retry with exponential backoff
- ✅ How to add timeouts to operations
- ✅ How circuit breakers prevent cascading failures
- ✅ How to combine resilience patterns

**Next:** [Tutorial 9: Custom Protocol Backend](#tutorial-9-custom-protocol-backend)

---

## Tutorial 9: Custom Protocol Backend

**Time: 45 minutes**
**Goal: Implement a custom protocol backend**

### Step 1: Implement Protocol Interface

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"

    "github.com/nats-io/nats.go"
    "github.com/Ranganaths/minion/core/multiagent"
)

// NATSProtocol implements Protocol interface using NATS messaging
type NATSProtocol struct {
    conn          *nats.Conn
    subscriptions map[string]*nats.Subscription
    mu            sync.RWMutex
}

func NewNATSProtocol(url string) (*NATSProtocol, error) {
    conn, err := nats.Connect(url)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to NATS: %w", err)
    }

    return &NATSProtocol{
        conn:          conn,
        subscriptions: make(map[string]*nats.Subscription),
    }, nil
}

func (np *NATSProtocol) Send(ctx context.Context, msg *multiagent.Message) error {
    // Serialize message
    data, err := json.Marshal(msg)
    if err != nil {
        return fmt.Errorf("failed to marshal message: %w", err)
    }

    // Publish to agent-specific subject
    subject := fmt.Sprintf("agent.%s", msg.To)
    if err := np.conn.Publish(subject, data); err != nil {
        return fmt.Errorf("failed to publish message: %w", err)
    }

    return nil
}

func (np *NATSProtocol) Receive(ctx context.Context, agentID string) ([]*multiagent.Message, error) {
    // Subscribe to agent's subject if not already subscribed
    subject := fmt.Sprintf("agent.%s", agentID)

    np.mu.Lock()
    if _, exists := np.subscriptions[agentID]; !exists {
        msgChan := make(chan *nats.Msg, 100)

        sub, err := np.conn.ChanSubscribe(subject, msgChan)
        if err != nil {
            np.mu.Unlock()
            return nil, fmt.Errorf("failed to subscribe: %w", err)
        }

        np.subscriptions[agentID] = sub
    }
    np.mu.Unlock()

    // Receive messages (non-blocking)
    var messages []*multiagent.Message

    np.mu.RLock()
    sub := np.subscriptions[agentID]
    np.mu.RUnlock()

    // Get subscription channel
    msgChan := make(chan *nats.Msg, 10)
    sub.SetPendingLimits(-1, -1)

    // Drain available messages
    for {
        select {
        case natsMsg := <-msgChan:
            var msg multiagent.Message
            if err := json.Unmarshal(natsMsg.Data, &msg); err != nil {
                continue
            }
            messages = append(messages, &msg)

        case <-ctx.Done():
            return messages, ctx.Err()

        default:
            return messages, nil
        }
    }
}

func (np *NATSProtocol) Subscribe(ctx context.Context, agentID string, msgType multiagent.MessageType) error {
    subject := fmt.Sprintf("events.%s", msgType)

    np.mu.Lock()
    defer np.mu.Unlock()

    if _, exists := np.subscriptions[subject]; exists {
        return nil // Already subscribed
    }

    msgChan := make(chan *nats.Msg, 100)
    sub, err := np.conn.ChanSubscribe(subject, msgChan)
    if err != nil {
        return fmt.Errorf("failed to subscribe to events: %w", err)
    }

    np.subscriptions[subject] = sub
    return nil
}

func (np *NATSProtocol) Broadcast(ctx context.Context, msg *multiagent.Message) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    subject := fmt.Sprintf("events.%s", msg.Type)
    return np.conn.Publish(subject, data)
}

func (np *NATSProtocol) Close() error {
    np.mu.Lock()
    defer np.mu.Unlock()

    // Close all subscriptions
    for _, sub := range np.subscriptions {
        sub.Unsubscribe()
    }

    np.conn.Close()
    return nil
}
```

### Step 2: Use Custom Protocol

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/Ranganaths/minion/core/multiagent"
)

func main() {
    ctx := context.Background()

    // Create NATS protocol
    natsURL := "nats://localhost:4222"
    protocol, err := NewNATSProtocol(natsURL)
    if err != nil {
        log.Fatal(err)
    }
    defer protocol.Close()

    fmt.Println("Connected to NATS")

    // Create agents using NATS protocol
    orchestrator := multiagent.NewOrchestratorAgent(
        "orchestrator-1",
        protocol,
        multiagent.NewInMemoryLedger(),
    )

    worker := multiagent.NewWorkerAgent(
        "worker-1",
        []string{"general"},
        protocol,
        nil,
    )

    // Start agents
    orchestrator.Start(ctx)
    worker.Start(ctx)

    orchestrator.RegisterWorker(worker)

    // Execute task
    task := &multiagent.Task{
        ID:   "nats-task-1",
        Name: "NATS Test Task",
        Type: "general",
    }

    fmt.Println("Executing task via NATS...")
    result, err := orchestrator.ExecuteTask(ctx, task)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Task completed: %v\n", result)

    // Cleanup
    worker.Stop(ctx)
    orchestrator.Stop(ctx)
}
```

### Step 3: Start NATS Server

```bash
# Using Docker
docker run -p 4222:4222 -p 8222:8222 nats:latest

# Run example
go run examples/tutorial9/main.go
```

### What You Learned
- ✅ How to implement Protocol interface
- ✅ How to integrate third-party messaging systems
- ✅ How to handle serialization/deserialization
- ✅ How to make protocols pluggable

**Next:** [Tutorial 10: Production Deployment](#tutorial-10-production-deployment)

---

## Tutorial 10: Production Deployment

**Time: 60 minutes**
**Goal: Deploy production-ready system with Docker Compose**

### Step 1: Create Docker Compose Configuration

**File: `docker-compose.yml`**

```yaml
version: '3.8'

services:
  # PostgreSQL for persistence
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: minion
      POSTGRES_USER: minion
      POSTGRES_PASSWORD: ${DB_PASSWORD:-minion123}
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U minion"]
      interval: 10s
      timeout: 5s
      retries: 5

  # Redis for distributed messaging
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  # Prometheus for metrics
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'

  # Grafana for visualization
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_PASSWORD:-admin}
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana/dashboards:/etc/grafana/provisioning/dashboards
    depends_on:
      - prometheus

  # Orchestrator
  orchestrator:
    build:
      context: .
      dockerfile: Dockerfile.orchestrator
    environment:
      PROTOCOL_TYPE: redis
      REDIS_ADDR: redis:6379
      LEDGER_TYPE: postgres
      DB_CONNECTION: postgresql://minion:${DB_PASSWORD:-minion123}@postgres:5432/minion?sslmode=disable
      LOAD_BALANCER_STRATEGY: capability_best
      METRICS_ENABLED: "true"
      METRICS_PORT: 9091
    ports:
      - "9091:9091"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  # Worker pool (scalable)
  worker:
    build:
      context: .
      dockerfile: Dockerfile.worker
    environment:
      PROTOCOL_TYPE: redis
      REDIS_ADDR: redis:6379
      WORKER_CAPABILITIES: general
      METRICS_ENABLED: "true"
    depends_on:
      - orchestrator
    deploy:
      replicas: 3
      restart_policy:
        condition: on-failure
        delay: 5s
        max_attempts: 3

volumes:
  postgres_data:
  redis_data:
  prometheus_data:
  grafana_data:
```

### Step 2: Create Dockerfiles

**File: `Dockerfile.orchestrator`**

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o orchestrator ./cmd/orchestrator

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/orchestrator .
COPY --from=builder /app/migrations ./migrations

EXPOSE 9091

CMD ["./orchestrator"]
```

**File: `Dockerfile.worker`**

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o worker ./cmd/worker

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/worker .

CMD ["./worker"]
```

### Step 3: Create Prometheus Configuration

**File: `prometheus.yml`**

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'orchestrator'
    static_configs:
      - targets: ['orchestrator:9091']

  - job_name: 'workers'
    static_configs:
      - targets: ['worker:9092']
```

### Step 4: Create Environment File

**File: `.env`**

```bash
# Database
DB_PASSWORD=secure_password_here

# Grafana
GRAFANA_PASSWORD=admin_password_here

# Worker scaling
WORKER_MIN_REPLICAS=2
WORKER_MAX_REPLICAS=20

# Load balancer
LOAD_BALANCER_STRATEGY=capability_best

# Auto-scaling
AUTOSCALING_ENABLED=true
AUTOSCALING_MAX_QUEUE_DEPTH=50
AUTOSCALING_MAX_UTILIZATION=0.80
```

### Step 5: Deploy

```bash
# Build and start all services
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f orchestrator
docker-compose logs -f worker

# Scale workers
docker-compose up -d --scale worker=5

# Stop all services
docker-compose down

# Stop and remove volumes (fresh start)
docker-compose down -v
```

### Step 6: Health Checks

```bash
# Check PostgreSQL
docker-compose exec postgres psql -U minion -c "SELECT 1"

# Check Redis
docker-compose exec redis redis-cli ping

# Check orchestrator health
curl http://localhost:9091/health

# Check metrics
curl http://localhost:9091/metrics
```

### Step 7: Access Monitoring

- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin/admin)
- **Orchestrator Metrics**: http://localhost:9091/metrics

### What You Learned
- ✅ How to containerize multi-agent system
- ✅ How to configure service dependencies
- ✅ How to set up monitoring and metrics
- ✅ How to scale workers dynamically
- ✅ Production deployment best practices

---

## Example 1: Data Processing Pipeline

**Goal: Build a data ETL pipeline with Minion**

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/Ranganaths/minion/core/multiagent"
)

func main() {
    ctx := context.Background()

    // Setup system
    system := setupSystem(ctx)
    defer system.Cleanup()

    // Define ETL workflow
    workflow := &multiagent.Workflow{
        ID:   "etl-pipeline-1",
        Name: "Customer Data ETL",
        Tasks: []*multiagent.Task{
            {
                ID:   "extract",
                Name: "Extract from Source DB",
                Type: "extraction",
                Input: map[string]interface{}{
                    "source": "mysql://prod-db",
                    "table":  "customers",
                    "filter": "updated_at > '2024-01-01'",
                },
            },
            {
                ID:           "transform",
                Name:         "Transform Data",
                Type:         "transformation",
                Dependencies: []string{"extract"},
                Input: map[string]interface{}{
                    "operations": []string{
                        "normalize_phone_numbers",
                        "validate_emails",
                        "enrich_geo_data",
                    },
                },
            },
            {
                ID:           "validate",
                Name:         "Validate Data Quality",
                Type:         "validation",
                Dependencies: []string{"transform"},
                Input: map[string]interface{}{
                    "rules": []string{
                        "no_nulls_in_required_fields",
                        "valid_email_format",
                        "phone_number_format",
                    },
                },
            },
            {
                ID:           "load",
                Name:         "Load to Data Warehouse",
                Type:         "loading",
                Dependencies: []string{"validate"},
                Input: map[string]interface{}{
                    "destination": "snowflake://warehouse",
                    "table":       "customers_enriched",
                    "mode":        "upsert",
                },
            },
            {
                ID:           "notify",
                Name:         "Send Completion Notification",
                Type:         "notification",
                Dependencies: []string{"load"},
                Input: map[string]interface{}{
                    "recipients": []string{"data-team@company.com"},
                    "template":   "etl_complete",
                },
            },
        },
    }

    // Execute workflow
    fmt.Printf("Starting ETL pipeline: %s\n", workflow.Name)
    if err := system.orchestrator.ExecuteWorkflow(ctx, workflow); err != nil {
        log.Fatal(err)
    }

    fmt.Println("ETL pipeline completed successfully!")

    // Get execution metrics
    metrics := system.GetWorkflowMetrics(workflow.ID)
    fmt.Printf("Records processed: %d\n", metrics.RecordsProcessed)
    fmt.Printf("Execution time: %v\n", metrics.Duration)
    fmt.Printf("Data quality score: %.2f%%\n", metrics.QualityScore)
}
```

---

## Example 2: Web Scraping Swarm

**Goal: Coordinate multiple scrapers for parallel web scraping**

```go
package main

import (
    "context"
    "fmt"
    "sync"

    "github.com/Ranganaths/minion/core/multiagent"
)

type ScrapingCoordinator struct {
    orchestrator *multiagent.OrchestratorAgent
    scrapers     []*ScraperAgent
    results      sync.Map
}

type ScraperAgent struct {
    *multiagent.WorkerAgent
    userAgent string
    rateLimit int
}

func main() {
    ctx := context.Background()

    // Create scraping swarm
    coordinator := NewScrapingCoordinator(10) // 10 scrapers

    // Define URLs to scrape
    urls := []string{
        "https://example.com/page1",
        "https://example.com/page2",
        // ... 1000s of URLs
    }

    // Distribute scraping tasks
    tasks := make([]*multiagent.Task, len(urls))
    for i, url := range urls {
        tasks[i] = &multiagent.Task{
            ID:   fmt.Sprintf("scrape-%d", i),
            Type: "web_scraping",
            Input: map[string]interface{}{
                "url":         url,
                "selectors":   []string{".title", ".price", ".description"},
                "max_retries": 3,
            },
        }
    }

    // Execute in parallel
    fmt.Printf("Scraping %d URLs with %d workers...\n", len(urls), len(coordinator.scrapers))

    results := coordinator.ExecuteParallel(ctx, tasks)

    fmt.Printf("Scraped %d pages successfully\n", len(results))

    // Aggregate results
    aggregated := coordinator.AggregateResults(results)
    fmt.Printf("Total products found: %d\n", aggregated.TotalProducts)
    fmt.Printf("Average price: $%.2f\n", aggregated.AvgPrice)
}
```

---

## Example 3: Distributed Testing Framework

**Goal: Run tests in parallel across multiple agents**

```go
package main

import (
    "context"
    "fmt"

    "github.com/Ranganaths/minion/core/multiagent"
)

type TestRunner struct {
    orchestrator *multiagent.OrchestratorAgent
    testAgents   []*TestAgent
}

type TestAgent struct {
    *multiagent.WorkerAgent
    environment string
}

func main() {
    ctx := context.Background()

    // Create test runner
    runner := NewTestRunner(&TestConfig{
        Environments: []string{"dev", "staging", "prod"},
        Browsers:     []string{"chrome", "firefox", "safari"},
        Parallelism:  20,
    })

    // Define test suite
    testSuite := &TestSuite{
        Name: "E2E Test Suite",
        Tests: []*Test{
            {Name: "User Login", Type: "e2e"},
            {Name: "Checkout Flow", Type: "e2e"},
            {Name: "API Performance", Type: "load"},
            {Name: "Security Scan", Type: "security"},
        },
    }

    // Execute test matrix
    fmt.Println("Running distributed tests...")

    results := runner.RunTestMatrix(ctx, testSuite)

    // Generate report
    report := generateTestReport(results)
    fmt.Printf("\nTest Results:\n")
    fmt.Printf("  Total: %d\n", report.Total)
    fmt.Printf("  Passed: %d\n", report.Passed)
    fmt.Printf("  Failed: %d\n", report.Failed)
    fmt.Printf("  Duration: %v\n", report.Duration)

    if report.Failed > 0 {
        fmt.Println("\nFailed tests:")
        for _, failure := range report.Failures {
            fmt.Printf("  - %s: %s\n", failure.Test, failure.Error)
        }
    }
}
```

---

## Troubleshooting Guide

### Common Issues

#### 1. Workers Not Connecting

**Symptom**: Workers start but don't receive tasks

**Solutions**:
```bash
# Check Redis connection
redis-cli -h localhost -p 6379 ping

# Check consumer group exists
redis-cli XINFO GROUPS agent:orchestrator-main

# Verify worker subscriptions
redis-cli XINFO CONSUMERS agent:orchestrator-main minion-group
```

#### 2. Tasks Stuck in Pending

**Symptom**: Tasks created but never executed

**Solutions**:
```go
// Check worker capabilities match task type
task.Type = "general" // Must match worker capabilities

// Verify workers are registered
orchestrator.ListWorkers()

// Check worker status
for _, worker := range workers {
    fmt.Printf("%s: %s\n", worker.ID, worker.Status)
}
```

#### 3. Database Connection Errors

**Symptom**: "connection refused" or "too many connections"

**Solutions**:
```go
// Increase connection pool
config := &multiagent.PostgresLedgerConfig{
    MaxConnections:     25,
    MaxIdleConnections: 10,
    ConnMaxLifetime:    3600,
}

// Check database connectivity
db.Ping()

// Review connection leaks
db.Stats()
```

#### 4. High Latency

**Symptom**: Tasks taking too long to execute

**Solutions**:
```go
// Enable performance tracking
lbConfig := &multiagent.LoadBalancerConfig{
    Strategy:                  multiagent.StrategyLatencyBased,
    EnablePerformanceTracking: true,
}

// Check worker distribution
stats := loadBalancer.GetStats()

// Add more workers
for i := 0; i < 5; i++ {
    workerPool.AddWorker(ctx, "general")
}
```

---

## Best Practices

### 1. Start Simple
- Begin with in-memory protocol
- Add persistence when needed
- Scale horizontally when required

### 2. Monitor Everything
- Enable metrics from day one
- Use distributed tracing
- Set up alerting

### 3. Design for Failure
- Always use timeouts
- Implement retry logic
- Add circuit breakers

### 4. Test at Scale
- Load test before production
- Test failure scenarios
- Validate auto-scaling

### 5. Document Architecture
- Keep diagrams updated
- Document design decisions
- Maintain runbooks

---

## Next Steps

- **Read**: [AGENTIC_DESIGN_PATTERNS.md](AGENTIC_DESIGN_PATTERNS.md) for advanced patterns
- **Review**: [PHASE3_COMPLETE.md](PHASE3_COMPLETE.md) for full system capabilities
- **Explore**: Example applications in `/examples` directory
- **Join**: Community discussions and support

---

**Happy Building with Minion!** 🎉
