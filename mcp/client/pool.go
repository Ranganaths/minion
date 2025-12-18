package client

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ConnectionPool manages a pool of MCP client connections
type ConnectionPool struct {
	clients     map[string]*PooledClient
	mu          sync.RWMutex
	config      *PoolConfig
	acquireWait map[string][]chan *PooledClient
	metrics     *poolMetrics
}

// PoolConfig configures the connection pool
type PoolConfig struct {
	MaxIdleConns     int           // Maximum idle connections
	MaxOpenConns     int           // Maximum open connections (0 = unlimited)
	ConnMaxLifetime  time.Duration // Maximum lifetime of a connection
	ConnMaxIdleTime  time.Duration // Maximum idle time before closing
	HealthCheckPeriod time.Duration // Period for health checks
}

// DefaultPoolConfig returns default pool configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxIdleConns:      5,
		MaxOpenConns:      10,
		ConnMaxLifetime:   30 * time.Minute,
		ConnMaxIdleTime:   5 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
	}
}

// PooledClient wraps a client with pool metadata
type PooledClient struct {
	client      *MCPClient
	pool        *ConnectionPool
	createdAt   time.Time
	lastUsedAt  time.Time
	inUse       bool
	healthCheck *HealthChecker
	mu          sync.Mutex
}

// poolMetrics tracks pool performance
type poolMetrics struct {
	totalConns    int
	idleConns     int
	activeConns   int
	waitCount     int64
	waitDuration  time.Duration
	acquiredTotal int64
	releasedTotal int64
	mu            sync.RWMutex
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config *PoolConfig) *ConnectionPool {
	if config == nil {
		config = DefaultPoolConfig()
	}

	pool := &ConnectionPool{
		clients:     make(map[string]*PooledClient),
		config:      config,
		acquireWait: make(map[string][]chan *PooledClient),
		metrics: &poolMetrics{
			totalConns:  0,
			idleConns:   0,
			activeConns: 0,
		},
	}

	// Start background cleanup
	go pool.cleanupLoop()

	return pool
}

// Acquire gets a client from the pool or creates a new one
func (p *ConnectionPool) Acquire(ctx context.Context, serverName string, config *ClientConfig) (*PooledClient, error) {
	startTime := time.Now()

	// Try to get an idle client first
	p.mu.Lock()
	if pooled, exists := p.clients[serverName]; exists {
		pooled.mu.Lock()
		if !pooled.inUse {
			// Reuse idle connection
			pooled.inUse = true
			pooled.lastUsedAt = time.Now()
			pooled.mu.Unlock()
			p.mu.Unlock()

			p.updateMetrics(true, time.Since(startTime))
			return pooled, nil
		}
		pooled.mu.Unlock()
	}

	// Check if we can create a new connection
	canCreate := p.config.MaxOpenConns == 0 || p.metrics.totalConns < p.config.MaxOpenConns
	p.mu.Unlock()

	if canCreate {
		// Create new connection
		client, err := newMCPClient(config)
		if err != nil {
			return nil, err
		}

		if err := client.Connect(ctx); err != nil {
			return nil, err
		}

		pooled := &PooledClient{
			client:     client,
			pool:       p,
			createdAt:  time.Now(),
			lastUsedAt: time.Now(),
			inUse:      true,
		}

		p.mu.Lock()
		p.clients[serverName] = pooled
		p.metrics.totalConns++
		p.metrics.activeConns++
		p.mu.Unlock()

		p.updateMetrics(true, time.Since(startTime))
		return pooled, nil
	}

	// Wait for a connection to become available
	waitChan := make(chan *PooledClient, 1)
	p.mu.Lock()
	p.acquireWait[serverName] = append(p.acquireWait[serverName], waitChan)
	p.metrics.waitCount++
	p.mu.Unlock()

	select {
	case <-ctx.Done():
		// Remove from wait list
		p.mu.Lock()
		p.removeFromWaitList(serverName, waitChan)
		p.mu.Unlock()
		return nil, ctx.Err()

	case pooled := <-waitChan:
		p.updateMetrics(true, time.Since(startTime))
		return pooled, nil

	case <-time.After(30 * time.Second):
		// Timeout waiting for connection
		p.mu.Lock()
		p.removeFromWaitList(serverName, waitChan)
		p.mu.Unlock()
		return nil, fmt.Errorf("timeout waiting for connection")
	}
}

// Release returns a client to the pool
func (p *ConnectionPool) Release(pooled *PooledClient, serverName string) {
	pooled.mu.Lock()
	pooled.inUse = false
	pooled.lastUsedAt = time.Now()
	pooled.mu.Unlock()

	p.updateMetrics(false, 0)

	// Try to satisfy a waiting acquire
	p.mu.Lock()
	if waiters, exists := p.acquireWait[serverName]; exists && len(waiters) > 0 {
		// Give to first waiter
		waiter := waiters[0]
		p.acquireWait[serverName] = waiters[1:]

		pooled.mu.Lock()
		pooled.inUse = true
		pooled.lastUsedAt = time.Now()
		pooled.mu.Unlock()

		p.mu.Unlock()

		select {
		case waiter <- pooled:
		default:
			// Waiter went away, release again
			p.Release(pooled, serverName)
		}
		return
	}
	p.mu.Unlock()
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var lastErr error
	for serverName, pooled := range p.clients {
		if err := pooled.client.Disconnect(); err != nil {
			lastErr = fmt.Errorf("failed to close connection to %s: %w", serverName, err)
		}
	}

	p.clients = make(map[string]*PooledClient)
	p.metrics.totalConns = 0
	p.metrics.idleConns = 0
	p.metrics.activeConns = 0

	return lastErr
}

// GetMetrics returns pool metrics
func (p *ConnectionPool) GetMetrics() PoolMetrics {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()

	return PoolMetrics{
		TotalConns:    p.metrics.totalConns,
		IdleConns:     p.metrics.idleConns,
		ActiveConns:   p.metrics.activeConns,
		WaitCount:     p.metrics.waitCount,
		WaitDuration:  p.metrics.waitDuration,
		AcquiredTotal: p.metrics.acquiredTotal,
		ReleasedTotal: p.metrics.releasedTotal,
	}
}

// PoolMetrics contains pool statistics
type PoolMetrics struct {
	TotalConns    int
	IdleConns     int
	ActiveConns   int
	WaitCount     int64
	WaitDuration  time.Duration
	AcquiredTotal int64
	ReleasedTotal int64
}

// cleanupLoop periodically cleans up stale connections
func (p *ConnectionPool) cleanupLoop() {
	ticker := time.NewTicker(p.config.HealthCheckPeriod)
	defer ticker.Stop()

	for range ticker.C {
		p.cleanup()
	}
}

// cleanup removes stale and idle connections
func (p *ConnectionPool) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	toRemove := []string{}

	for serverName, pooled := range p.clients {
		pooled.mu.Lock()

		// Check if connection is in use
		if pooled.inUse {
			pooled.mu.Unlock()
			continue
		}

		// Check max lifetime
		if p.config.ConnMaxLifetime > 0 && now.Sub(pooled.createdAt) > p.config.ConnMaxLifetime {
			toRemove = append(toRemove, serverName)
			pooled.mu.Unlock()
			continue
		}

		// Check max idle time
		if p.config.ConnMaxIdleTime > 0 && now.Sub(pooled.lastUsedAt) > p.config.ConnMaxIdleTime {
			// Keep minimum idle connections
			if p.metrics.idleConns > p.config.MaxIdleConns {
				toRemove = append(toRemove, serverName)
			}
		}

		pooled.mu.Unlock()
	}

	// Remove stale connections
	for _, serverName := range toRemove {
		if pooled, exists := p.clients[serverName]; exists {
			pooled.client.Disconnect()
			delete(p.clients, serverName)
			p.metrics.totalConns--
			if !pooled.inUse {
				p.metrics.idleConns--
			}
		}
	}
}

// updateMetrics updates pool metrics
func (p *ConnectionPool) updateMetrics(acquired bool, waitDuration time.Duration) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	if acquired {
		p.metrics.acquiredTotal++
		p.metrics.activeConns++
		if p.metrics.idleConns > 0 {
			p.metrics.idleConns--
		}
		if waitDuration > 0 {
			p.metrics.waitDuration += waitDuration
		}
	} else {
		p.metrics.releasedTotal++
		if p.metrics.activeConns > 0 {
			p.metrics.activeConns--
		}
		p.metrics.idleConns++
	}
}

// removeFromWaitList removes a channel from the wait list
func (p *ConnectionPool) removeFromWaitList(serverName string, ch chan *PooledClient) {
	if waiters, exists := p.acquireWait[serverName]; exists {
		newWaiters := make([]chan *PooledClient, 0, len(waiters))
		for _, waiter := range waiters {
			if waiter != ch {
				newWaiters = append(newWaiters, waiter)
			}
		}
		p.acquireWait[serverName] = newWaiters
	}
}

// GetClient returns the underlying client (use with caution)
func (pc *PooledClient) GetClient() *MCPClient {
	return pc.client
}

// IsHealthy checks if the connection is healthy
func (pc *PooledClient) IsHealthy() bool {
	return pc.client.IsConnected()
}

// Age returns how long the connection has existed
func (pc *PooledClient) Age() time.Duration {
	return time.Since(pc.createdAt)
}

// IdleTime returns how long the connection has been idle
func (pc *PooledClient) IdleTime() time.Duration {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return time.Since(pc.lastUsedAt)
}
