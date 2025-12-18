package client

import (
	"testing"
	"time"
)

// Note: These tests focus on pool mechanics. Full integration tests with real
// MCP clients are in the integration test suite.

func TestConnectionPool_Creation(t *testing.T) {
	pool := NewConnectionPool(DefaultPoolConfig())
	defer pool.Close()

	if pool == nil {
		t.Fatal("Expected pool to be created")
	}

	metrics := pool.GetMetrics()
	if metrics.TotalConns != 0 {
		t.Errorf("Expected 0 initial connections, got %d", metrics.TotalConns)
	}
}

func TestConnectionPool_Config(t *testing.T) {
	config := &PoolConfig{
		MaxIdleConns:      3,
		MaxOpenConns:      10,
		ConnMaxLifetime:   20 * time.Minute,
		ConnMaxIdleTime:   3 * time.Minute,
		HealthCheckPeriod: 20 * time.Second,
	}
	pool := NewConnectionPool(config)
	defer pool.Close()

	if pool.config.MaxIdleConns != 3 {
		t.Errorf("Expected MaxIdleConns 3, got %d", pool.config.MaxIdleConns)
	}

	if pool.config.MaxOpenConns != 10 {
		t.Errorf("Expected MaxOpenConns 10, got %d", pool.config.MaxOpenConns)
	}
}

func TestConnectionPool_DefaultConfig(t *testing.T) {
	pool := NewConnectionPool(nil)
	defer pool.Close()

	// Should use defaults
	if pool.config.MaxIdleConns != 5 {
		t.Errorf("Expected default MaxIdleConns 5, got %d", pool.config.MaxIdleConns)
	}

	if pool.config.MaxOpenConns != 10 {
		t.Errorf("Expected default MaxOpenConns 10, got %d", pool.config.MaxOpenConns)
	}
}

func TestConnectionPool_Metrics(t *testing.T) {
	pool := NewConnectionPool(DefaultPoolConfig())
	defer pool.Close()

	metrics := pool.GetMetrics()

	// Initial metrics should be zero
	if metrics.TotalConns != 0 {
		t.Errorf("Expected 0 total connections, got %d", metrics.TotalConns)
	}

	if metrics.ActiveConns != 0 {
		t.Errorf("Expected 0 active connections, got %d", metrics.ActiveConns)
	}

	if metrics.IdleConns != 0 {
		t.Errorf("Expected 0 idle connections, got %d", metrics.IdleConns)
	}

	if metrics.WaitCount != 0 {
		t.Errorf("Expected 0 wait count, got %d", metrics.WaitCount)
	}
}

func TestConnectionPool_Close(t *testing.T) {
	pool := NewConnectionPool(DefaultPoolConfig())

	// Close empty pool
	err := pool.Close()
	if err != nil {
		// Errors may occur, but shouldn't panic
	}

	metrics := pool.GetMetrics()
	if metrics.TotalConns != 0 {
		t.Errorf("Expected 0 connections after close, got %d", metrics.TotalConns)
	}
}

func TestPooledClient_Age(t *testing.T) {
	// Create a mock pooled client
	pooled := &PooledClient{
		createdAt: time.Now().Add(-5 * time.Minute),
	}

	age := pooled.Age()
	if age < 4*time.Minute {
		t.Errorf("Expected age >= 4 minutes, got %v", age)
	}
}

func TestPooledClient_IdleTime(t *testing.T) {
	// Create a mock pooled client
	pooled := &PooledClient{
		lastUsedAt: time.Now().Add(-2 * time.Minute),
	}

	idleTime := pooled.IdleTime()
	if idleTime < 1*time.Minute {
		t.Errorf("Expected idle time >= 1 minute, got %v", idleTime)
	}
}
