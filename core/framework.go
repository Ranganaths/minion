package core

import (
	"context"
	"fmt"
	"time"

	"github.com/Ranganaths/minion/llm"
	"github.com/Ranganaths/minion/mcp/bridge"
	"github.com/Ranganaths/minion/mcp/client"
	"github.com/Ranganaths/minion/models"
	"github.com/Ranganaths/minion/storage"
	"github.com/Ranganaths/minion/tools"
	"github.com/google/uuid"
)

// FrameworkImpl is the main implementation of the agent framework
type FrameworkImpl struct {
	store            storage.Store
	llmProvider      llm.Provider
	behaviorRegistry BehaviorRegistry
	toolRegistry     tools.Registry

	// MCP (Model Context Protocol) components
	mcpClientManager *client.MCPClientManager
	mcpBridge        *bridge.BridgeRegistry
}

// Option is a functional option for configuring the framework
type Option func(*FrameworkImpl)

// WithStorage sets the storage implementation
func WithStorage(store storage.Store) Option {
	return func(f *FrameworkImpl) {
		f.store = store
	}
}

// WithLLMProvider sets the LLM provider
func WithLLMProvider(provider llm.Provider) Option {
	return func(f *FrameworkImpl) {
		f.llmProvider = provider
	}
}

// WithBehaviorRegistry sets the behavior registry
func WithBehaviorRegistry(registry BehaviorRegistry) Option {
	return func(f *FrameworkImpl) {
		f.behaviorRegistry = registry
	}
}

// WithToolRegistry sets the tool registry
func WithToolRegistry(registry tools.Registry) Option {
	return func(f *FrameworkImpl) {
		f.toolRegistry = registry
	}
}

// NewFramework creates a new agent framework with the given options
func NewFramework(opts ...Option) *FrameworkImpl {
	// Initialize MCP client manager
	mcpManager := client.NewMCPClientManager(nil) // Use default config

	f := &FrameworkImpl{
		behaviorRegistry: NewBehaviorRegistry(),
		toolRegistry:     tools.NewRegistry(),
		mcpClientManager: mcpManager,
	}

	// Initialize MCP bridge (requires framework reference for tool registration)
	f.mcpBridge = bridge.NewBridgeRegistry(mcpManager, f)

	for _, opt := range opts {
		opt(f)
	}

	return f
}

// CreateAgent creates a new agent
func (f *FrameworkImpl) CreateAgent(ctx context.Context, req *models.CreateAgentRequest) (*models.Agent, error) {
	if f.store == nil {
		return nil, fmt.Errorf("storage not configured")
	}

	// Validate required fields
	if req.Name == "" {
		return nil, fmt.Errorf("agent name is required")
	}
	if req.BehaviorType == "" {
		req.BehaviorType = "default"
	}

	// Check if behavior exists
	if _, err := f.behaviorRegistry.Get(req.BehaviorType); err != nil {
		return nil, fmt.Errorf("invalid behavior type: %w", err)
	}

	// Create agent
	agent := &models.Agent{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Description:  req.Description,
		BehaviorType: req.BehaviorType,
		Status:       models.StatusDraft,
		Config:       req.Config,
		Capabilities: req.Capabilities,
		Metadata:     req.Metadata,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Initialize metadata if nil
	if agent.Metadata == nil {
		agent.Metadata = make(map[string]interface{})
	}

	// Save to storage
	if err := f.store.Create(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Initialize metrics
	metrics := &models.Metrics{
		AgentID:   agent.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := f.store.CreateMetrics(ctx, metrics); err != nil {
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}

	return agent, nil
}

// GetAgent retrieves an agent by ID
func (f *FrameworkImpl) GetAgent(ctx context.Context, id string) (*models.Agent, error) {
	if f.store == nil {
		return nil, fmt.Errorf("storage not configured")
	}

	return f.store.Get(ctx, id)
}

// UpdateAgent updates an existing agent
func (f *FrameworkImpl) UpdateAgent(ctx context.Context, id string, req *models.UpdateAgentRequest) (*models.Agent, error) {
	if f.store == nil {
		return nil, fmt.Errorf("storage not configured")
	}

	// Get existing agent
	agent, err := f.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Apply updates
	if req.Name != nil {
		agent.Name = *req.Name
	}
	if req.Description != nil {
		agent.Description = *req.Description
	}
	if req.Status != nil {
		agent.Status = *req.Status
	}
	if req.Config != nil {
		agent.Config = *req.Config
	}
	if req.Capabilities != nil {
		agent.Capabilities = *req.Capabilities
	}
	if req.Metadata != nil {
		agent.Metadata = *req.Metadata
	}

	agent.UpdatedAt = time.Now()

	// Save updates
	if err := f.store.Update(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	return agent, nil
}

// DeleteAgent deletes an agent
func (f *FrameworkImpl) DeleteAgent(ctx context.Context, id string) error {
	if f.store == nil {
		return fmt.Errorf("storage not configured")
	}

	return f.store.Delete(ctx, id)
}

// ListAgents returns a paginated list of agents
func (f *FrameworkImpl) ListAgents(ctx context.Context, req *models.ListAgentsRequest) (*models.ListAgentsResponse, error) {
	if f.store == nil {
		return nil, fmt.Errorf("storage not configured")
	}

	agents, total, err := f.store.List(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	// Calculate total pages
	totalPages := 0
	if req.PageSize > 0 {
		totalPages = (total + req.PageSize - 1) / req.PageSize
	}

	// Convert []*Agent to []Agent
	agentList := make([]models.Agent, len(agents))
	for i, a := range agents {
		agentList[i] = *a
	}

	return &models.ListAgentsResponse{
		Agents:     agentList,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	}, nil
}

// RegisterBehavior registers a new behavior type
func (f *FrameworkImpl) RegisterBehavior(behaviorType string, behavior Behavior) error {
	return f.behaviorRegistry.Register(behaviorType, behavior)
}

// GetBehavior retrieves a behavior by type
func (f *FrameworkImpl) GetBehavior(behaviorType string) (Behavior, error) {
	return f.behaviorRegistry.Get(behaviorType)
}

// RegisterTool registers a new tool
func (f *FrameworkImpl) RegisterTool(tool interface{}) error {
	t, ok := tool.(tools.Tool)
	if !ok {
		return fmt.Errorf("tool must implement tools.Tool interface")
	}

	return f.toolRegistry.Register(t)
}

// GetToolsForAgent returns tools available for an agent
func (f *FrameworkImpl) GetToolsForAgent(agent *models.Agent) []interface{} {
	availableTools := f.toolRegistry.GetToolsForAgent(agent)

	// Convert to []interface{}
	result := make([]interface{}, len(availableTools))
	for i, t := range availableTools {
		result[i] = t
	}

	return result
}

// ExecuteTool executes a tool by name with the given parameters
func (f *FrameworkImpl) ExecuteTool(ctx context.Context, toolName string, params map[string]interface{}) (*models.ToolOutput, error) {
	input := &models.ToolInput{
		Params: params,
	}

	return f.toolRegistry.Execute(ctx, toolName, input)
}

// GetTool retrieves a tool by name
func (f *FrameworkImpl) GetTool(name string) (tools.Tool, error) {
	return f.toolRegistry.Get(name)
}

// ListTools returns all registered tool names
func (f *FrameworkImpl) ListTools() []string {
	return f.toolRegistry.List()
}

// Execute executes an agent with the given input
func (f *FrameworkImpl) Execute(ctx context.Context, agentID string, input *models.Input) (*models.Output, error) {
	if f.store == nil {
		return nil, fmt.Errorf("storage not configured")
	}
	if f.llmProvider == nil {
		return nil, fmt.Errorf("LLM provider not configured")
	}

	startTime := time.Now()

	// 1. Get agent
	agent, err := f.store.Get(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Check if agent is active
	if agent.Status != models.StatusActive && agent.Status != models.StatusDraft {
		return nil, fmt.Errorf("agent is not active (status: %s)", agent.Status)
	}

	// 2. Get behavior
	behavior, err := f.behaviorRegistry.Get(agent.BehaviorType)
	if err != nil {
		return nil, fmt.Errorf("failed to get behavior: %w", err)
	}

	// 3. Process input
	processedInput, err := behavior.ProcessInput(ctx, agent, input)
	if err != nil {
		return nil, fmt.Errorf("failed to process input: %w", err)
	}

	// 4. Generate system prompt
	systemPrompt := behavior.GetSystemPrompt(agent)

	// 5. Prepare user prompt
	userPrompt := fmt.Sprintf("%v", processedInput.Processed)
	if processedInput.Instructions != "" {
		userPrompt = fmt.Sprintf("%s\n\nInstructions: %s", userPrompt, processedInput.Instructions)
	}

	// 6. Call LLM
	llmModel := agent.Config.LLMModel
	if llmModel == "" {
		llmModel = "gpt-4" // Default model
	}

	temperature := agent.Config.Temperature
	if temperature == 0 {
		temperature = 0.7 // Default temperature
	}

	maxTokens := agent.Config.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1000 // Default max tokens
	}

	completion, err := f.llmProvider.GenerateCompletion(ctx, &llm.CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  temperature,
		MaxTokens:    maxTokens,
		Model:        llmModel,
	})
	if err != nil {
		// Record failed execution
		f.recordActivity(ctx, agent.ID, input, nil, "failed", time.Since(startTime), err.Error())
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// 7. Create output
	output := &models.Output{
		Result: completion.Text,
		Type:   "text",
		Metadata: map[string]interface{}{
			"tokens_used":   completion.TokensUsed,
			"model":         completion.Model,
			"finish_reason": completion.FinishReason,
		},
	}

	// 8. Process output
	processedOutput, err := behavior.ProcessOutput(ctx, agent, output)
	if err != nil {
		return nil, fmt.Errorf("failed to process output: %w", err)
	}

	// 9. Record successful execution
	duration := time.Since(startTime)
	f.recordActivity(ctx, agent.ID, input, processedOutput.Original, "success", duration, "")

	// 10. Update metrics
	f.updateMetrics(ctx, agent.ID, true, duration)

	return processedOutput.Original, nil
}

// GetMetrics retrieves metrics for an agent
func (f *FrameworkImpl) GetMetrics(ctx context.Context, agentID string) (*models.Metrics, error) {
	if f.store == nil {
		return nil, fmt.Errorf("storage not configured")
	}

	return f.store.GetMetrics(ctx, agentID)
}

// GetActivities retrieves recent activities for an agent
func (f *FrameworkImpl) GetActivities(ctx context.Context, agentID string, limit int) ([]*models.Activity, error) {
	if f.store == nil {
		return nil, fmt.Errorf("storage not configured")
	}

	return f.store.GetActivities(ctx, agentID, limit)
}

// recordActivity records an activity in storage
func (f *FrameworkImpl) recordActivity(ctx context.Context, agentID string, input *models.Input, output *models.Output, status string, duration time.Duration, errorMsg string) {
	activity := &models.Activity{
		ID:        uuid.New().String(),
		AgentID:   agentID,
		Action:    "execute",
		Input:     input,
		Output:    output,
		Status:    status,
		Duration:  duration.Milliseconds(),
		Error:     errorMsg,
		CreatedAt: time.Now(),
	}

	// Best effort - don't fail execution if activity recording fails
	_ = f.store.RecordActivity(ctx, activity)
}

// updateMetrics updates agent metrics
func (f *FrameworkImpl) updateMetrics(ctx context.Context, agentID string, success bool, duration time.Duration) {
	metrics, err := f.store.GetMetrics(ctx, agentID)
	if err != nil {
		return // Best effort
	}

	metrics.TotalExecutions++
	if success {
		metrics.SuccessfulExecutions++
	} else {
		metrics.FailedExecutions++
	}

	// Update average execution time
	if metrics.TotalExecutions > 0 {
		totalTime := metrics.AvgExecutionTime * float64(metrics.TotalExecutions-1)
		totalTime += float64(duration.Milliseconds())
		metrics.AvgExecutionTime = totalTime / float64(metrics.TotalExecutions)
	}

	metrics.UpdatedAt = time.Now()

	// Best effort - don't fail execution if metrics update fails
	_ = f.store.UpdateMetrics(ctx, metrics)
}

// ConnectMCPServer connects to an external MCP server and registers its tools
func (f *FrameworkImpl) ConnectMCPServer(ctx context.Context, config interface{}) error {
	// Convert config to ClientConfig
	clientConfig, ok := config.(*client.ClientConfig)
	if !ok {
		return fmt.Errorf("invalid config type: expected *client.ClientConfig")
	}

	// Connect to server
	if err := f.mcpClientManager.ConnectServer(ctx, clientConfig); err != nil {
		return fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	// Register tools from server
	if err := f.mcpBridge.RegisterServerTools(ctx, clientConfig.ServerName); err != nil {
		// Disconnect on registration failure
		_ = f.mcpClientManager.DisconnectServer(clientConfig.ServerName)
		return fmt.Errorf("failed to register MCP tools: %w", err)
	}

	return nil
}

// DisconnectMCPServer disconnects from an MCP server and unregisters its tools
func (f *FrameworkImpl) DisconnectMCPServer(serverName string) error {
	// Unregister tools first
	if err := f.mcpBridge.UnregisterServerTools(serverName); err != nil {
		// Continue with disconnect even if unregister fails
		_ = err
	}

	// Disconnect from server
	if err := f.mcpClientManager.DisconnectServer(serverName); err != nil {
		return fmt.Errorf("failed to disconnect from MCP server: %w", err)
	}

	return nil
}

// ListMCPServers returns names of all connected MCP servers
func (f *FrameworkImpl) ListMCPServers() []string {
	return f.mcpClientManager.ListServers()
}

// GetMCPServerStatus returns status of all connected MCP servers
func (f *FrameworkImpl) GetMCPServerStatus() map[string]interface{} {
	status := f.mcpClientManager.GetStatus()

	// Convert to map[string]interface{} for API compatibility
	result := make(map[string]interface{})
	for name, s := range status {
		result[name] = s
	}

	return result
}

// RefreshMCPTools refreshes tools from an MCP server
func (f *FrameworkImpl) RefreshMCPTools(ctx context.Context, serverName string) error {
	// Unregister existing tools
	if err := f.mcpBridge.UnregisterServerTools(serverName); err != nil {
		// If no tools found, that's fine - continue with re-registration
		_ = err
	}

	// Re-register tools
	if err := f.mcpBridge.RegisterServerTools(ctx, serverName); err != nil {
		return fmt.Errorf("failed to refresh MCP tools: %w", err)
	}

	return nil
}

// Close closes the framework and releases resources
func (f *FrameworkImpl) Close() error {
	// Close MCP connections
	if f.mcpClientManager != nil {
		if err := f.mcpClientManager.Close(); err != nil {
			return fmt.Errorf("failed to close MCP client manager: %w", err)
		}
	}

	// Close storage
	if f.store != nil {
		return f.store.Close()
	}

	return nil
}
