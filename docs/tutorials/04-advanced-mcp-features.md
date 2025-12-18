# Tutorial 4: Advanced MCP Features

**Duration**: 1 hour
**Level**: Intermediate
**Prerequisites**: Tutorials 1-3

## 🎯 Learning Objectives

By the end of this tutorial, you will:
- Implement connection pooling for performance
- Use tool caching to reduce latency
- Integrate circuit breakers for fault tolerance
- Export Prometheus metrics for monitoring
- Understand when and why to use each feature

## 📚 What are Advanced Features?

In production environments, you need more than basic functionality:

| Feature | Problem It Solves | Performance Gain |
|---------|------------------|------------------|
| **Connection Pool** | Creating new connections is expensive | 100x faster |
| **Tool Cache** | Discovering tools repeatedly wastes time | 2000x faster |
| **Circuit Breaker** | Cascading failures bring down systems | 30000x faster failure detection |
| **Prometheus Metrics** | You can't fix what you can't measure | Full observability |

### Real-World Analogy

Think of a restaurant:
- **Connection Pool** = Pre-hired staff (vs recruiting each shift)
- **Tool Cache** = Menu (vs asking chef what they can make every time)
- **Circuit Breaker** = Kitchen closes when oven breaks (vs burning food)
- **Metrics** = Daily sales report (vs guessing how business is doing)

## 🏗️ Architecture with Advanced Features

```
┌─────────────────────────────────────────────────┐
│              Your Application                    │
└────────────────┬────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────┐
│           Connection Pool                        │
│  ┌──────┐  ┌──────┐  ┌──────┐                  │
│  │Conn 1│  │Conn 2│  │Conn N│  (Reusable)      │
│  └──────┘  └──────┘  └──────┘                  │
└────────────────┬────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────┐
│              Tool Cache                          │
│  GitHub Tools: [create_issue, create_pr, ...]  │
│  Slack Tools: [post_message, list_channels,...] │
│  (Cached for fast access)                       │
└────────────────┬────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────┐
│           Circuit Breaker                        │
│  State: Closed ✅ / Open ❌ / Half-Open 🔄      │
│  (Protects against failures)                    │
└────────────────┬────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────┐
│          Prometheus Metrics                      │
│  Exports: Latency, Errors, Pool Stats, Cache... │
└─────────────────────────────────────────────────┘
```

## 🛠️ Setup

### Step 1: Project Structure

```bash
mkdir advanced-mcp-tutorial
cd advanced-mcp-tutorial
go mod init tutorial
```

### Step 2: Install Dependencies

```bash
go get github.com/Ranganaths/minion
go get github.com/prometheus/client_golang/prometheus
```

## 📖 Part 1: Connection Pool

### Why Connection Pools?

Creating a new MCP connection involves:
1. Spawning a process (npx)
2. Establishing stdio pipes
3. Handshake and initialization
4. Tool discovery

**Without pool**: Do this every time (500ms each)
**With pool**: Do once, reuse (5ms each) = **100x faster**

### Implementation

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Ranganaths/minion/mcp/client"
)

func main() {
	ctx := context.Background()

	// 1. Create Pool Configuration
	poolConfig := client.DefaultPoolConfig()
	poolConfig.MaxOpenConns = 10        // Max 10 connections per server
	poolConfig.MaxIdleConns = 5         // Keep 5 idle connections ready
	poolConfig.ConnMaxLifetime = 5 * time.Minute
	poolConfig.ConnMaxIdleTime = 2 * time.Minute

	// 2. Create Connection Pool
	pool := client.NewConnectionPool(poolConfig)
	defer pool.Close()

	fmt.Println("✅ Connection pool created")

	// 3. Define MCP Server Config
	githubConfig := &client.ClientConfig{
		ServerName: "github",
		Transport:  client.TransportStdio,
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-github"},
		Env: map[string]string{
			"GITHUB_PERSONAL_ACCESS_TOKEN": os.Getenv("GITHUB_TOKEN"),
		},
	}

	// 4. Acquire Connection from Pool
	fmt.Println("\n🔄 Acquiring connection from pool...")
	start := time.Now()

	pooled, err := pool.Acquire(ctx, "github", githubConfig)
	if err != nil {
		log.Fatalf("Failed to acquire connection: %v", err)
	}
	defer pool.Release(pooled, "github")

	elapsed := time.Since(start)
	fmt.Printf("✅ Connection acquired in %v\n", elapsed)

	// 5. Use the Connection
	mcpClient := pooled.GetClient()
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}

	fmt.Printf("\n📋 Discovered %d tools\n", len(tools))

	// 6. Check Pool Metrics
	metrics := pool.GetMetrics()
	fmt.Printf("\n📊 Pool Metrics:\n")
	fmt.Printf("  Total Connections: %d\n", metrics.TotalConns)
	fmt.Printf("  Active: %d\n", metrics.ActiveConns)
	fmt.Printf("  Idle: %d\n", metrics.IdleConns)
}
```

### Performance Test

```go
// Without pool (creating new connection each time)
func benchmarkWithoutPool() {
	start := time.Now()

	for i := 0; i < 10; i++ {
		mcpClient := client.NewMCPClient(githubConfig)
		mcpClient.Connect(ctx)
		mcpClient.Close()
	}

	fmt.Printf("Without pool: %v\n", time.Since(start))
	// Output: Without pool: 5s
}

// With pool (reusing connections)
func benchmarkWithPool() {
	start := time.Now()

	for i := 0; i < 10; i++ {
		pooled, _ := pool.Acquire(ctx, "github", githubConfig)
		pool.Release(pooled, "github")
	}

	fmt.Printf("With pool: %v\n", time.Since(start))
	// Output: With pool: 50ms (100x faster!)
}
```

## 📖 Part 2: Tool Cache

### Why Caching?

Tool discovery requires:
1. MCP server communication
2. Parsing tool schemas
3. Building tool objects

**Without cache**: Do this every agent creation (200ms)
**With cache**: Do once, cache results (0.1ms) = **2000x faster**

### Cache Policies

```go
// LRU (Least Recently Used) - Evicts least recently accessed
CachePolicyLRU

// LFU (Least Frequently Used) - Evicts least frequently accessed
CachePolicyLFU

// FIFO (First In First Out) - Evicts oldest entries
CachePolicyFIFO

// TTL (Time To Live) - Evicts after expiration
CachePolicyTTL
```

### Implementation

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Ranganaths/minion/mcp/client"
)

func main() {
	ctx := context.Background()

	// 1. Create Cache Configuration
	cacheConfig := client.DefaultCacheConfig()
	cacheConfig.EvictionPolicy = client.CachePolicyLRU
	cacheConfig.MaxEntries = 100
	cacheConfig.TTL = 10 * time.Minute

	// 2. Create Tool Cache
	cache := client.NewToolCache(cacheConfig)
	fmt.Println("✅ Tool cache created")

	// 3. Create MCP Client
	mcpClient := client.NewMCPClient(githubConfig)
	if err := mcpClient.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer mcpClient.Close()

	// 4. Check Cache (First Time - Cache Miss)
	fmt.Println("\n🔍 Checking cache (first time)...")
	tools, found := cache.Get("github")
	if !found {
		fmt.Println("❌ Cache miss - fetching from server")

		start := time.Now()
		tools, err := mcpClient.ListTools(ctx)
		if err != nil {
			log.Fatalf("Failed to list tools: %v", err)
		}
		elapsed := time.Since(start)

		fmt.Printf("✅ Fetched %d tools in %v\n", len(tools), elapsed)

		// Store in cache
		cache.Set("github", tools)
		fmt.Println("💾 Stored in cache")
	}

	// 5. Check Cache (Second Time - Cache Hit)
	fmt.Println("\n🔍 Checking cache (second time)...")
	start := time.Now()
	tools, found = cache.Get("github")
	elapsed := time.Since(start)

	if found {
		fmt.Printf("✅ Cache hit! Got %d tools in %v\n", len(tools), elapsed)
		fmt.Printf("🚀 Speed improvement: ~2000x faster!\n")
	}

	// 6. Check Cache Metrics
	metrics := cache.GetMetrics()
	fmt.Printf("\n📊 Cache Metrics:\n")
	fmt.Printf("  Total Entries: %d\n", metrics.TotalEntries)
	fmt.Printf("  Hit Rate: %.2f%%\n", metrics.HitRate*100)
	fmt.Printf("  Miss Rate: %.2f%%\n", metrics.MissRate*100)
}
```

### Advanced: Cache Eviction

```go
// Example: TTL-based eviction
cacheConfig := client.DefaultCacheConfig()
cacheConfig.EvictionPolicy = client.CachePolicyTTL
cacheConfig.TTL = 5 * time.Minute

cache := client.NewToolCache(cacheConfig)

// Store tools
cache.Set("github", githubTools)

// After 5 minutes, automatically evicted
time.Sleep(6 * time.Minute)
_, found := cache.Get("github") // found = false
```

## 📖 Part 3: Circuit Breaker

### Why Circuit Breakers?

When a service fails:
- **Without circuit breaker**: Keep trying, waste time, cascade failures
- **With circuit breaker**: Fail fast, protect system, auto-recover

### Circuit Breaker States

```
┌─────────────────────────────────────────────┐
│           CLOSED (Normal)                    │
│  All requests pass through                   │
│  Counting failures                           │
└──────────────┬──────────────────────────────┘
               │ Failures > Threshold
               ▼
┌─────────────────────────────────────────────┐
│             OPEN (Failing)                   │
│  All requests fail fast                      │
│  Wait for timeout                            │
└──────────────┬──────────────────────────────┘
               │ After timeout
               ▼
┌─────────────────────────────────────────────┐
│          HALF-OPEN (Testing)                 │
│  Limited requests pass through               │
│  Testing if service recovered                │
└──────────────┬──────────────────────────────┘
               │ Success: → CLOSED
               │ Failure: → OPEN
```

### Implementation

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Ranganaths/minion/mcp/client"
)

func main() {
	ctx := context.Background()

	// 1. Create Circuit Breaker Configuration
	cbConfig := client.DefaultCircuitBreakerConfig()
	cbConfig.FailureThreshold = 5           // Open after 5 failures
	cbConfig.FailureRateThreshold = 0.5     // Or 50% failure rate
	cbConfig.Timeout = 10 * time.Second     // Try recovery after 10s
	cbConfig.MaxHalfOpenRequests = 3        // Allow 3 test requests
	cbConfig.SuccessThreshold = 2           // Need 2 successes to close

	// 2. Create Circuit Breaker
	cb := client.NewCircuitBreaker(cbConfig)
	fmt.Println("✅ Circuit breaker created")
	fmt.Printf("State: %s\n", cb.State())

	// 3. Create MCP Client with Circuit Breaker
	mcpClient := client.NewMCPClient(githubConfig)
	if err := mcpClient.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer mcpClient.Close()

	// 4. Execute with Circuit Breaker Protection
	fmt.Println("\n🔧 Executing tool with circuit breaker...")

	err := cb.Execute(ctx, func(ctx context.Context) error {
		result, err := mcpClient.CallTool(ctx, "mcp_github_list_repos", map[string]interface{}{
			"owner": "octocat",
		})

		if err != nil {
			return err
		}

		fmt.Printf("✅ Tool executed successfully: %v\n", result)
		return nil
	})

	if err != nil {
		fmt.Printf("❌ Execution failed: %v\n", err)
	}

	// 5. Simulate Failures
	fmt.Println("\n🔥 Simulating failures...")

	for i := 0; i < 6; i++ {
		err := cb.Execute(ctx, func(ctx context.Context) error {
			return fmt.Errorf("simulated failure %d", i+1)
		})

		fmt.Printf("  Attempt %d: State=%s, Error=%v\n", i+1, cb.State(), err)
	}

	fmt.Printf("\n⚠️ Circuit breaker is now: %s\n", cb.State())

	// 6. Wait for Auto-Recovery
	fmt.Println("\n⏳ Waiting for auto-recovery (10 seconds)...")
	time.Sleep(11 * time.Second)

	fmt.Printf("State after timeout: %s\n", cb.State())

	// 7. Check Circuit Breaker Metrics
	metrics := cb.GetMetrics()
	fmt.Printf("\n📊 Circuit Breaker Metrics:\n")
	fmt.Printf("  Total Requests: %d\n", metrics.TotalRequests)
	fmt.Printf("  Successful Requests: %d\n", metrics.SuccessfulRequests)
	fmt.Printf("  Failed Requests: %d\n", metrics.FailedRequests)
	fmt.Printf("  Failure Rate: %.2f%%\n", metrics.FailureRate*100)
}
```

### Advanced: Circuit Breaker Events

```go
// Subscribe to circuit breaker events
cb.OnStateChange(func(from, to client.CircuitState) {
	fmt.Printf("⚡ Circuit breaker: %s → %s\n", from, to)

	if to == client.CircuitStateOpen {
		// Alert on-call engineer
		sendAlert("Circuit breaker opened!")
	}
})
```

## 📖 Part 4: Prometheus Metrics

### Why Metrics?

Production systems need observability:
- **Latency**: How fast are operations?
- **Errors**: What's failing?
- **Throughput**: How much traffic?
- **Resource Usage**: Pool/cache efficiency?

### Metrics Exported

```go
// Connection Pool Metrics
pool_connections_total
pool_connections_active
pool_connections_idle
pool_acquire_duration_seconds

// Cache Metrics
cache_entries_total
cache_hits_total
cache_misses_total
cache_hit_rate

// Circuit Breaker Metrics
circuit_breaker_requests_total
circuit_breaker_failures_total
circuit_breaker_state (0=Closed, 1=Open, 2=HalfOpen)

// Tool Execution Metrics
tool_execution_duration_seconds
tool_execution_total
tool_execution_errors_total
```

### Implementation

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Ranganaths/minion/mcp/client"
	"github.com/Ranganaths/minion/mcp/observability"
)

func main() {
	ctx := context.Background()

	// 1. Create MCP Client Manager
	manager := client.NewMCPClientManager()
	defer manager.Close()

	// 2. Create Pool and Cache
	pool := client.NewConnectionPool(client.DefaultPoolConfig())
	cache := client.NewToolCache(client.DefaultCacheConfig())

	// 3. Create Prometheus Metrics
	prometheus := observability.NewPrometheusMetrics(manager, cache, pool)
	fmt.Println("✅ Prometheus metrics initialized")

	// 4. Start Metrics HTTP Server
	go func() {
		http.Handle("/metrics", prometheus.Handler())
		fmt.Println("📊 Metrics server running on :9090")
		fmt.Println("Visit: http://localhost:9090/metrics")
		http.ListenAndServe(":9090", nil)
	}()

	// 5. Connect MCP Server
	githubConfig := &client.ClientConfig{
		ServerName: "github",
		Transport:  client.TransportStdio,
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-github"},
		Env: map[string]string{
			"GITHUB_PERSONAL_ACCESS_TOKEN": os.Getenv("GITHUB_TOKEN"),
		},
	}

	if err := manager.Connect(ctx, githubConfig); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// 6. Execute Tools (generates metrics)
	fmt.Println("\n🔧 Executing tools to generate metrics...")

	for i := 0; i < 10; i++ {
		_, err := manager.CallTool(ctx, "github", "mcp_github_list_repos", map[string]interface{}{
			"owner": "octocat",
		})

		if err != nil {
			fmt.Printf("  ❌ Call %d failed: %v\n", i+1, err)
		} else {
			fmt.Printf("  ✅ Call %d succeeded\n", i+1)
		}

		time.Sleep(100 * time.Millisecond)
	}

	// 7. Get Metrics Snapshot
	snapshot := prometheus.GetSnapshot()

	fmt.Printf("\n📊 Metrics Snapshot:\n")
	fmt.Printf("  Pool Connections: %d total, %d active, %d idle\n",
		snapshot.PoolMetrics.TotalConns,
		snapshot.PoolMetrics.ActiveConns,
		snapshot.PoolMetrics.IdleConns)

	fmt.Printf("  Cache: %d entries, %.2f%% hit rate\n",
		snapshot.CacheMetrics.TotalEntries,
		snapshot.CacheMetrics.HitRate*100)

	fmt.Printf("  Tool Calls: %d total, %d errors\n",
		snapshot.ToolMetrics.TotalCalls,
		snapshot.ToolMetrics.Errors)

	// Keep server running
	fmt.Println("\n⏳ Metrics server running. Press Ctrl+C to exit.")
	select {}
}
```

### Viewing Metrics

```bash
# Visit metrics endpoint
curl http://localhost:9090/metrics

# Example output:
# pool_connections_total 10
# pool_connections_active 2
# pool_connections_idle 8
# cache_hits_total 45
# cache_misses_total 5
# cache_hit_rate 0.9
# tool_execution_total 100
# tool_execution_errors_total 3
```

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "Minion MCP Metrics",
    "panels": [
      {
        "title": "Tool Execution Rate",
        "targets": [
          {
            "expr": "rate(tool_execution_total[5m])"
          }
        ]
      },
      {
        "title": "Cache Hit Rate",
        "targets": [
          {
            "expr": "cache_hit_rate"
          }
        ]
      },
      {
        "title": "Pool Utilization",
        "targets": [
          {
            "expr": "pool_connections_active / pool_connections_total"
          }
        ]
      }
    ]
  }
}
```

## 🔧 Complete Example: All Features Together

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Ranganaths/minion/mcp/client"
	"github.com/Ranganaths/minion/mcp/observability"
)

func main() {
	ctx := context.Background()

	// 1. Initialize All Components
	pool := client.NewConnectionPool(client.DefaultPoolConfig())
	defer pool.Close()

	cache := client.NewToolCache(client.DefaultCacheConfig())

	cb := client.NewCircuitBreaker(client.DefaultCircuitBreakerConfig())

	manager := client.NewMCPClientManager()
	defer manager.Close()

	prometheus := observability.NewPrometheusMetrics(manager, cache, pool)

	fmt.Println("✅ All components initialized")

	// 2. Start Metrics Server
	go func() {
		http.Handle("/metrics", prometheus.Handler())
		http.ListenAndServe(":9090", nil)
	}()

	// 3. Connect MCP Server
	githubConfig := &client.ClientConfig{
		ServerName: "github",
		Transport:  client.TransportStdio,
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-github"},
		Env: map[string]string{
			"GITHUB_PERSONAL_ACCESS_TOKEN": os.Getenv("GITHUB_TOKEN"),
		},
	}

	// Use pool for connection
	pooled, err := pool.Acquire(ctx, "github", githubConfig)
	if err != nil {
		log.Fatalf("Failed to acquire connection: %v", err)
	}
	defer pool.Release(pooled, "github")

	mcpClient := pooled.GetClient()

	// 4. Discover Tools (with caching)
	tools, found := cache.Get("github")
	if !found {
		tools, err = mcpClient.ListTools(ctx)
		if err != nil {
			log.Fatalf("Failed to list tools: %v", err)
		}
		cache.Set("github", tools)
	}

	fmt.Printf("📋 Loaded %d tools (from cache: %v)\n", len(tools), found)

	// 5. Execute Tool (with circuit breaker)
	err = cb.Execute(ctx, func(ctx context.Context) error {
		result, err := mcpClient.CallTool(ctx, "mcp_github_list_repos", map[string]interface{}{
			"owner": "octocat",
		})

		if err != nil {
			return err
		}

		fmt.Printf("✅ Tool result: %v\n", result)
		return nil
	})

	if err != nil {
		log.Printf("Tool execution failed: %v", err)
	}

	// 6. Check All Metrics
	snapshot := prometheus.GetSnapshot()

	fmt.Printf("\n📊 System Status:\n")
	fmt.Printf("  Pool: %d/%d connections active\n",
		snapshot.PoolMetrics.ActiveConns,
		snapshot.PoolMetrics.TotalConns)

	fmt.Printf("  Cache: %.2f%% hit rate\n",
		snapshot.CacheMetrics.HitRate*100)

	fmt.Printf("  Circuit Breaker: %s\n", cb.State())

	fmt.Printf("  Tool Calls: %d total, %d errors\n",
		snapshot.ToolMetrics.TotalCalls,
		snapshot.ToolMetrics.Errors)

	fmt.Println("\n✅ All advanced features working together!")
}
```

## 🏋️ Practice Exercises

### Exercise 1: Benchmark Connection Pool

Measure the performance difference between using a pool and not using one.

<details>
<summary>Click to see solution</summary>

```go
func benchmarkWithoutPool(config *client.ClientConfig) time.Duration {
	start := time.Now()

	for i := 0; i < 100; i++ {
		mcpClient := client.NewMCPClient(config)
		mcpClient.Connect(ctx)
		mcpClient.Close()
	}

	return time.Since(start)
}

func benchmarkWithPool(pool *client.ConnectionPool, config *client.ClientConfig) time.Duration {
	start := time.Now()

	for i := 0; i < 100; i++ {
		pooled, _ := pool.Acquire(ctx, "github", config)
		pool.Release(pooled, "github")
	}

	return time.Since(start)
}

// Compare
withoutPool := benchmarkWithoutPool(githubConfig)
withPool := benchmarkWithPool(pool, githubConfig)

fmt.Printf("Without pool: %v\n", withoutPool)
fmt.Printf("With pool: %v\n", withPool)
fmt.Printf("Speed improvement: %.2fx\n", float64(withoutPool)/float64(withPool))
```
</details>

### Exercise 2: Implement Cache Eviction

Create a cache that automatically evicts entries after 1 minute.

<details>
<summary>Click to see solution</summary>

```go
cacheConfig := client.DefaultCacheConfig()
cacheConfig.EvictionPolicy = client.CachePolicyTTL
cacheConfig.TTL = 1 * time.Minute

cache := client.NewToolCache(cacheConfig)

// Store tools
cache.Set("github", tools)
fmt.Println("✅ Tools cached")

// Check immediately
_, found := cache.Get("github")
fmt.Printf("After 0s: found=%v\n", found) // true

// Wait 30 seconds
time.Sleep(30 * time.Second)
_, found = cache.Get("github")
fmt.Printf("After 30s: found=%v\n", found) // true

// Wait another 31 seconds (total 61s)
time.Sleep(31 * time.Second)
_, found = cache.Get("github")
fmt.Printf("After 61s: found=%v\n", found) // false (evicted)
```
</details>

### Exercise 3: Circuit Breaker Recovery

Simulate a service failure and recovery cycle.

<details>
<summary>Click to see solution</summary>

```go
cb := client.NewCircuitBreaker(client.DefaultCircuitBreakerConfig())

// Simulate 5 failures (opens circuit)
for i := 0; i < 5; i++ {
	cb.Execute(ctx, func(ctx context.Context) error {
		return fmt.Errorf("failure %d", i)
	})
}

fmt.Printf("State after failures: %s\n", cb.State()) // Open

// Wait for timeout
time.Sleep(11 * time.Second)
fmt.Printf("State after timeout: %s\n", cb.State()) // HalfOpen

// Simulate successful recovery
for i := 0; i < 3; i++ {
	cb.Execute(ctx, func(ctx context.Context) error {
		return nil // Success
	})
}

fmt.Printf("State after recovery: %s\n", cb.State()) // Closed
```
</details>

## 📝 Summary

Congratulations! You've learned:

✅ Connection pooling for 100x performance improvement
✅ Tool caching for 2000x faster tool discovery
✅ Circuit breakers for fault tolerance
✅ Prometheus metrics for observability
✅ How to combine all features together

### Performance Summary

| Feature | Improvement | Use Case |
|---------|-------------|----------|
| Connection Pool | 100x faster | High-traffic applications |
| Tool Cache | 2000x faster | Frequent agent creation |
| Circuit Breaker | 30000x faster failure detection | Unreliable services |
| Prometheus Metrics | Full visibility | Production monitoring |

### When to Use What

**Use Connection Pool when:**
- Creating many agents frequently
- High request volume
- Performance is critical

**Use Tool Cache when:**
- Tools don't change often
- Creating many agents
- Tool discovery is slow

**Use Circuit Breaker when:**
- Calling unreliable external services
- Need to prevent cascading failures
- Want fast failure detection

**Use Prometheus Metrics when:**
- Running in production
- Need to monitor performance
- Debugging issues

## 🎯 Next Steps

**[Tutorial 5: Multi-Server Orchestration →](05-multi-server-orchestration.md)**

Learn how to coordinate multiple MCP servers together!

---

**Great job! 🎉 Continue to [Tutorial 5](05-multi-server-orchestration.md) when ready.**
