package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/minion/mcp/client"
	"github.com/yourusername/minion/mcp/observability"
)

// VirtualSDR represents an AI-powered Sales Development Representative
type VirtualSDR struct {
	name           string
	mcpManager     *client.MCPClientManager
	pool           *client.ConnectionPool
	cache          *client.ToolCache
	circuitBreaker *client.CircuitBreaker
	prometheus     *observability.PrometheusMetrics
	collector      *observability.MetricsCollector
}

// NewVirtualSDR creates a new virtual SDR agent
func NewVirtualSDR(name string) *VirtualSDR {
	// Create connection pool for efficient resource usage
	poolConfig := client.DefaultPoolConfig()
	poolConfig.MaxOpenConns = 20
	poolConfig.MaxIdleConns = 10
	pool := client.NewConnectionPool(poolConfig)

	// Create tool cache with LRU eviction
	cacheConfig := client.DefaultCacheConfig()
	cacheConfig.EvictionPolicy = client.CachePolicyLRU
	cacheConfig.TTL = 10 * time.Minute // Cache tools for 10 minutes
	cache := client.NewToolCache(cacheConfig)

	// Create circuit breaker for fault tolerance
	cbConfig := client.DefaultCircuitBreakerConfig()
	cbConfig.MaxFailures = 3
	cbConfig.Timeout = 30 * time.Second
	circuitBreaker := client.NewCircuitBreaker(cbConfig)

	// Create MCP client manager
	manager := client.NewMCPClientManager()

	// Setup Prometheus metrics
	prometheus := observability.NewPrometheusMetrics(manager).
		WithCache(cache).
		WithPool(pool)

	collector := observability.NewMetricsCollector(prometheus, 30*time.Second)

	return &VirtualSDR{
		name:           name,
		mcpManager:     manager,
		pool:           pool,
		cache:          cache,
		circuitBreaker: circuitBreaker,
		prometheus:     prometheus,
		collector:      collector,
	}
}

// Start initializes the SDR agent and connects to MCP servers
func (sdr *VirtualSDR) Start(ctx context.Context) error {
	log.Printf("🤖 Starting Virtual SDR: %s", sdr.name)

	// Start metrics collection
	sdr.collector.Start()

	// Expose metrics endpoint for Prometheus
	http.HandleFunc("/metrics", sdr.handleMetrics)
	http.HandleFunc("/health", sdr.handleHealth)
	http.HandleFunc("/status", sdr.handleStatus)

	go func() {
		log.Println("📊 Metrics server starting on :9090")
		if err := http.ListenAndServe(":9090", nil); err != nil {
			log.Printf("Failed to start metrics server: %v", err)
		}
	}()

	// Connect to Salesforce MCP Server
	if err := sdr.connectSalesforce(ctx); err != nil {
		log.Printf("⚠️  Failed to connect to Salesforce: %v", err)
	}

	// Connect to Gmail MCP Server
	if err := sdr.connectGmail(ctx); err != nil {
		log.Printf("⚠️  Failed to connect to Gmail: %v", err)
	}

	// Connect to Calendar MCP Server
	if err := sdr.connectCalendar(ctx); err != nil {
		log.Printf("⚠️  Failed to connect to Calendar: %v", err)
	}

	log.Println("✅ Virtual SDR started successfully")
	return nil
}

// connectSalesforce connects to Salesforce MCP server
func (sdr *VirtualSDR) connectSalesforce(ctx context.Context) error {
	log.Println("🔌 Connecting to Salesforce MCP server...")

	config := &client.ClientConfig{
		ServerName: "salesforce",
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-salesforce"},
		Env: map[string]string{
			"SALESFORCE_INSTANCE_URL": os.Getenv("SALESFORCE_INSTANCE_URL"),
			"SALESFORCE_CLIENT_ID":    os.Getenv("SALESFORCE_CLIENT_ID"),
			"SALESFORCE_CLIENT_SECRET": os.Getenv("SALESFORCE_CLIENT_SECRET"),
			"SALESFORCE_USERNAME":     os.Getenv("SALESFORCE_USERNAME"),
			"SALESFORCE_PASSWORD":     os.Getenv("SALESFORCE_PASSWORD"),
		},
	}

	// Use circuit breaker for connection
	err := sdr.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		return sdr.mcpManager.AddClient(ctx, config)
	})

	if err != nil {
		return fmt.Errorf("failed to connect to Salesforce: %w", err)
	}

	// Discover and cache tools
	tools, err := sdr.discoverTools(ctx, "salesforce")
	if err != nil {
		return fmt.Errorf("failed to discover Salesforce tools: %w", err)
	}

	log.Printf("✅ Connected to Salesforce: %d tools available", len(tools))
	for _, tool := range tools {
		log.Printf("   - %s: %s", tool.Name, tool.Description)
	}

	return nil
}

// connectGmail connects to Gmail MCP server
func (sdr *VirtualSDR) connectGmail(ctx context.Context) error {
	log.Println("🔌 Connecting to Gmail MCP server...")

	config := &client.ClientConfig{
		ServerName: "gmail",
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-gmail"},
		Env: map[string]string{
			"GMAIL_CREDENTIALS": os.Getenv("GMAIL_CREDENTIALS"),
		},
	}

	err := sdr.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		return sdr.mcpManager.AddClient(ctx, config)
	})

	if err != nil {
		return fmt.Errorf("failed to connect to Gmail: %w", err)
	}

	tools, err := sdr.discoverTools(ctx, "gmail")
	if err != nil {
		return fmt.Errorf("failed to discover Gmail tools: %w", err)
	}

	log.Printf("✅ Connected to Gmail: %d tools available", len(tools))
	return nil
}

// connectCalendar connects to Calendar MCP server
func (sdr *VirtualSDR) connectCalendar(ctx context.Context) error {
	log.Println("🔌 Connecting to Calendar MCP server...")

	config := &client.ClientConfig{
		ServerName: "calendar",
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-google-calendar"},
		Env: map[string]string{
			"GOOGLE_CALENDAR_CREDENTIALS": os.Getenv("GOOGLE_CALENDAR_CREDENTIALS"),
		},
	}

	err := sdr.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		return sdr.mcpManager.AddClient(ctx, config)
	})

	if err != nil {
		return fmt.Errorf("failed to connect to Calendar: %w", err)
	}

	tools, err := sdr.discoverTools(ctx, "calendar")
	if err != nil {
		return fmt.Errorf("failed to discover Calendar tools: %w", err)
	}

	log.Printf("✅ Connected to Calendar: %d tools available", len(tools))
	return nil
}

// discoverTools discovers tools with caching
func (sdr *VirtualSDR) discoverTools(ctx context.Context, serverName string) ([]client.MCPTool, error) {
	// Check cache first
	if tools, found := sdr.cache.Get(serverName); found {
		log.Printf("📦 Cache hit for %s tools", serverName)
		return tools, nil
	}

	log.Printf("🔍 Cache miss - discovering %s tools...", serverName)

	// Acquire connection from pool
	pooled, err := sdr.pool.Acquire(ctx, serverName, nil)
	if err != nil {
		// Pool may not have this connection yet, get from manager
		mcpClient := sdr.mcpManager.GetClient(serverName)
		if mcpClient == nil {
			return nil, fmt.Errorf("client not found: %s", serverName)
		}

		// Discover tools
		tools, err := mcpClient.DiscoverTools(ctx)
		if err != nil {
			return nil, err
		}

		// Cache the results
		sdr.cache.Set(serverName, tools)
		return tools, nil
	}

	defer sdr.pool.Release(pooled, serverName)

	// Get client and discover tools
	mcpClient := pooled.GetClient()
	tools, err := mcpClient.DiscoverTools(ctx)
	if err != nil {
		return nil, err
	}

	// Cache the results
	sdr.cache.Set(serverName, tools)
	return tools, nil
}

// QualifyLead qualifies a lead using Salesforce data
func (sdr *VirtualSDR) QualifyLead(ctx context.Context, leadID string) error {
	log.Printf("🎯 Qualifying lead: %s", leadID)

	// Use circuit breaker to protect against failures
	err := sdr.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		// Get lead data from Salesforce
		params := map[string]interface{}{
			"leadId": leadID,
		}

		result, err := sdr.callTool(ctx, "salesforce", "get_lead", params)
		if err != nil {
			return fmt.Errorf("failed to get lead: %w", err)
		}

		log.Printf("📋 Lead data retrieved: %v", result)

		// Analyze lead score
		qualificationScore := sdr.calculateLeadScore(result)
		log.Printf("📊 Lead score: %d/100", qualificationScore)

		// Update lead status in Salesforce
		if qualificationScore >= 70 {
			return sdr.updateLeadStatus(ctx, leadID, "Qualified")
		}

		return sdr.updateLeadStatus(ctx, leadID, "Nurture")
	})

	if err != nil {
		if sdr.circuitBreaker.IsOpen() {
			log.Println("⚠️  Circuit breaker is open - Salesforce may be unavailable")
		}
		return err
	}

	log.Printf("✅ Lead %s qualified successfully", leadID)
	return nil
}

// SendFollowUpEmail sends a personalized follow-up email
func (sdr *VirtualSDR) SendFollowUpEmail(ctx context.Context, leadEmail, leadName string) error {
	log.Printf("📧 Sending follow-up email to: %s (%s)", leadName, leadEmail)

	err := sdr.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		// Compose personalized email
		subject := fmt.Sprintf("Following up on our conversation, %s", leadName)
		body := sdr.composeFollowUpEmail(leadName)

		params := map[string]interface{}{
			"to":      leadEmail,
			"subject": subject,
			"body":    body,
		}

		_, err := sdr.callTool(ctx, "gmail", "send_email", params)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("✅ Follow-up email sent to %s", leadName)
	return nil
}

// ScheduleMeeting schedules a meeting with a qualified lead
func (sdr *VirtualSDR) ScheduleMeeting(ctx context.Context, leadEmail, leadName string, proposedTime time.Time) error {
	log.Printf("📅 Scheduling meeting with: %s", leadName)

	err := sdr.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		params := map[string]interface{}{
			"attendees":   []string{leadEmail},
			"summary":     fmt.Sprintf("Discovery Call with %s", leadName),
			"description": "Let's discuss how we can help your business grow",
			"startTime":   proposedTime.Format(time.RFC3339),
			"endTime":     proposedTime.Add(30 * time.Minute).Format(time.RFC3339),
		}

		_, err := sdr.callTool(ctx, "calendar", "create_event", params)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to schedule meeting: %w", err)
	}

	log.Printf("✅ Meeting scheduled with %s for %s", leadName, proposedTime.Format(time.RFC1123))
	return nil
}

// RunDailyWorkflow executes the daily SDR workflow
func (sdr *VirtualSDR) RunDailyWorkflow(ctx context.Context) error {
	log.Println("🔄 Starting daily SDR workflow...")

	// 1. Get new leads from Salesforce
	leads, err := sdr.getNewLeads(ctx)
	if err != nil {
		return fmt.Errorf("failed to get new leads: %w", err)
	}

	log.Printf("📊 Found %d new leads to process", len(leads))

	// 2. Qualify each lead
	for _, leadID := range leads {
		if err := sdr.QualifyLead(ctx, leadID); err != nil {
			log.Printf("⚠️  Failed to qualify lead %s: %v", leadID, err)
			continue
		}

		// Small delay to avoid rate limiting
		time.Sleep(1 * time.Second)
	}

	// 3. Send follow-up emails to qualified leads
	qualifiedLeads, err := sdr.getQualifiedLeads(ctx)
	if err != nil {
		return fmt.Errorf("failed to get qualified leads: %w", err)
	}

	log.Printf("📧 Sending follow-ups to %d qualified leads", len(qualifiedLeads))

	for _, lead := range qualifiedLeads {
		if err := sdr.SendFollowUpEmail(ctx, lead.Email, lead.Name); err != nil {
			log.Printf("⚠️  Failed to send email to %s: %v", lead.Name, err)
			continue
		}

		time.Sleep(500 * time.Millisecond)
	}

	log.Println("✅ Daily workflow completed")
	return nil
}

// Helper methods

func (sdr *VirtualSDR) callTool(ctx context.Context, serverName, toolName string, params map[string]interface{}) (interface{}, error) {
	mcpClient := sdr.mcpManager.GetClient(serverName)
	if mcpClient == nil {
		return nil, fmt.Errorf("client not found: %s", serverName)
	}

	return mcpClient.CallTool(ctx, toolName, params)
}

func (sdr *VirtualSDR) calculateLeadScore(leadData interface{}) int {
	// Simplified lead scoring logic
	// In production, this would analyze company size, industry, engagement, etc.
	return 75 // Mock score
}

func (sdr *VirtualSDR) updateLeadStatus(ctx context.Context, leadID, status string) error {
	params := map[string]interface{}{
		"leadId": leadID,
		"status": status,
	}

	_, err := sdr.callTool(ctx, "salesforce", "update_lead", params)
	return err
}

func (sdr *VirtualSDR) composeFollowUpEmail(leadName string) string {
	return fmt.Sprintf(`Hi %s,

I wanted to follow up on our recent conversation about how our solution can help your business grow.

Based on what you shared, I believe we can help you:
- Increase sales efficiency by 40%%
- Reduce customer acquisition costs
- Scale your operations effectively

Would you be available for a 30-minute discovery call this week? I'd love to show you a personalized demo.

Best regards,
%s
Virtual Sales Development Representative

P.S. This email was intelligently composed and sent by an AI agent using MCP integration!
`, leadName, sdr.name)
}

type Lead struct {
	ID    string
	Name  string
	Email string
}

func (sdr *VirtualSDR) getNewLeads(ctx context.Context) ([]string, error) {
	// In production, this would query Salesforce for new leads
	// For demo purposes, return mock data
	return []string{"lead-001", "lead-002", "lead-003"}, nil
}

func (sdr *VirtualSDR) getQualifiedLeads(ctx context.Context) ([]Lead, error) {
	// In production, this would query Salesforce for qualified leads
	// For demo purposes, return mock data
	return []Lead{
		{ID: "lead-001", Name: "John Smith", Email: "john@example.com"},
		{ID: "lead-002", Name: "Jane Doe", Email: "jane@example.com"},
	}, nil
}

// HTTP Handlers

func (sdr *VirtualSDR) handleMetrics(w http.ResponseWriter, r *http.Request) {
	snapshot := sdr.collector.GetLatest()
	if snapshot == nil {
		http.Error(w, "No metrics available", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.Write([]byte(snapshot.ToPrometheusFormat()))
}

func (sdr *VirtualSDR) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := sdr.mcpManager.GetStatus()
	allHealthy := true

	for _, s := range status {
		if !s.Connected {
			allHealthy = false
			break
		}
	}

	if allHealthy {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Some services unavailable"))
	}
}

func (sdr *VirtualSDR) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status := sdr.mcpManager.GetStatus()
	cacheMetrics := sdr.cache.GetMetrics()
	poolMetrics := sdr.pool.GetMetrics()
	cbMetrics := sdr.circuitBreaker.GetMetrics()

	fmt.Fprintf(w, `{
  "sdr_name": "%s",
  "mcp_servers": %d,
  "cache": {
    "hit_rate": %.2f,
    "hits": %d,
    "misses": %d,
    "size": %d
  },
  "pool": {
    "total": %d,
    "active": %d,
    "idle": %d
  },
  "circuit_breaker": {
    "state": "%s",
    "failure_rate": %.2f
  },
  "servers": %v
}`,
		sdr.name,
		len(status),
		cacheMetrics.HitRate,
		cacheMetrics.Hits,
		cacheMetrics.Misses,
		cacheMetrics.CurrentSize,
		poolMetrics.TotalConns,
		poolMetrics.ActiveConns,
		poolMetrics.IdleConns,
		cbMetrics.State,
		cbMetrics.FailureRate,
		status,
	)
}

// Stop gracefully shuts down the SDR agent
func (sdr *VirtualSDR) Stop() {
	log.Println("🛑 Stopping Virtual SDR...")

	sdr.collector.Stop()
	sdr.pool.Close()
	sdr.mcpManager.DisconnectAll()

	log.Println("✅ Virtual SDR stopped")
}

func main() {
	// Create virtual SDR agent
	sdr := NewVirtualSDR("Alex the AI SDR")

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the agent
	if err := sdr.Start(ctx); err != nil {
		log.Fatalf("Failed to start SDR: %v", err)
	}

	// Run daily workflow
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		// Run immediately on start
		if err := sdr.RunDailyWorkflow(ctx); err != nil {
			log.Printf("Workflow error: %v", err)
		}

		for {
			select {
			case <-ticker.C:
				if err := sdr.RunDailyWorkflow(ctx); err != nil {
					log.Printf("Workflow error: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Print status
	log.Println("\n" + `
╔═══════════════════════════════════════════════════════════╗
║         🤖 Virtual SDR Agent Running                      ║
║                                                           ║
║  Endpoints:                                               ║
║    • Metrics:  http://localhost:9090/metrics             ║
║    • Health:   http://localhost:9090/health              ║
║    • Status:   http://localhost:9090/status              ║
║                                                           ║
║  Features:                                                ║
║    ✅ Connection pooling for performance                  ║
║    ✅ Tool caching with LRU eviction                      ║
║    ✅ Circuit breaker for fault tolerance                 ║
║    ✅ Prometheus metrics for monitoring                   ║
║    ✅ Multi-server MCP integration                        ║
║                                                           ║
║  Press Ctrl+C to stop                                     ║
╚═══════════════════════════════════════════════════════════╝
`)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown
	sdr.Stop()
}
