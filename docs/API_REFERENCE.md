# API Reference

Complete API documentation for the Minion framework.

## Core Package (`core/`)

### Framework

The main entry point for the Minion framework.

#### NewFramework

Creates a new framework instance with functional options.

```go
func NewFramework(opts ...Option) *FrameworkImpl
```

**Options:**
- `WithStorage(store storage.Store)` - Set storage backend
- `WithLLMProvider(provider llm.Provider)` - Set LLM provider
- `WithBehaviorRegistry(registry BehaviorRegistry)` - Set behavior registry
- `WithToolRegistry(registry tools.Registry)` - Set tool registry
- `WithSkillRegistry(registry *skills.InMemorySkillRegistry)` - Set skill registry
- `WithSkillsDirectory(path string)` - Load skills from directory

**Example:**
```go
framework := core.NewFramework(
    core.WithStorage(storage.NewInMemory()),
    core.WithLLMProvider(llm.NewOpenAI(apiKey)),
)
defer framework.Close()
```

#### Agent Operations

```go
// Create agent
func (f *FrameworkImpl) CreateAgent(ctx context.Context, req *models.CreateAgentRequest) (*models.Agent, error)

// Get agent by ID
func (f *FrameworkImpl) GetAgent(ctx context.Context, id string) (*models.Agent, error)

// Update agent
func (f *FrameworkImpl) UpdateAgent(ctx context.Context, id string, req *models.UpdateAgentRequest) (*models.Agent, error)

// Delete agent
func (f *FrameworkImpl) DeleteAgent(ctx context.Context, id string) error

// List agents with pagination
func (f *FrameworkImpl) ListAgents(ctx context.Context, req *models.ListAgentsRequest) (*models.ListAgentsResponse, error)
```

#### Execution

```go
// Execute agent with input
func (f *FrameworkImpl) Execute(ctx context.Context, agentID string, input *models.Input) (*models.Output, error)
```

#### Tool Operations

```go
// Register a tool
func (f *FrameworkImpl) RegisterTool(tool interface{}) error

// Get tools for agent (filtered by capabilities)
func (f *FrameworkImpl) GetToolsForAgent(agent *models.Agent) []interface{}

// Execute tool by name
func (f *FrameworkImpl) ExecuteTool(ctx context.Context, toolName string, params map[string]interface{}) (*models.ToolOutput, error)

// Get tool by name
func (f *FrameworkImpl) GetTool(name string) (tools.Tool, error)

// List all tool names
func (f *FrameworkImpl) ListTools() []string
```

#### Skill Operations

```go
// Register a skill
func (f *FrameworkImpl) RegisterSkill(skill skills.Skill) error

// Load skills from directory
func (f *FrameworkImpl) LoadSkillsFromDirectory(path string) (int, error)

// Get skills for agent
func (f *FrameworkImpl) GetSkillsForAgent(agent *models.Agent) []skills.Skill

// Execute skill by name
func (f *FrameworkImpl) ExecuteSkill(ctx context.Context, skillName string, input *skills.SkillInput) (*skills.SkillOutput, error)

// List all skills
func (f *FrameworkImpl) ListSkills() []skills.SkillInfo

// Watch skills directory for hot-reload
func (f *FrameworkImpl) WatchSkillsDirectory(path string) error

// Stop watching
func (f *FrameworkImpl) StopWatchingSkills() error
```

#### MCP Operations

```go
// Connect to MCP server
func (f *FrameworkImpl) ConnectMCPServer(ctx context.Context, config interface{}) error

// Disconnect from MCP server
func (f *FrameworkImpl) DisconnectMCPServer(serverName string) error

// List connected MCP servers
func (f *FrameworkImpl) ListMCPServers() []string

// Get MCP server status
func (f *FrameworkImpl) GetMCPServerStatus() map[string]interface{}

// Refresh tools from MCP server
func (f *FrameworkImpl) RefreshMCPTools(ctx context.Context, serverName string) error
```

#### Metrics

```go
// Get agent metrics
func (f *FrameworkImpl) GetMetrics(ctx context.Context, agentID string) (*models.Metrics, error)

// Get agent activities
func (f *FrameworkImpl) GetActivities(ctx context.Context, agentID string, limit int) ([]*models.Activity, error)
```

#### Lifecycle

```go
// Close framework and release resources
func (f *FrameworkImpl) Close() error
```

### Behavior Interface

```go
type Behavior interface {
    // GetSystemPrompt generates the system prompt for the agent
    GetSystemPrompt(agent *models.Agent) string

    // ProcessInput prepares input before LLM execution
    ProcessInput(ctx context.Context, agent *models.Agent, input *models.Input) (*models.ProcessedInput, error)

    // ProcessOutput enhances output after LLM execution
    ProcessOutput(ctx context.Context, agent *models.Agent, output *models.Output) (*models.ProcessedOutput, error)
}
```

## Models Package (`models/`)

### Agent

```go
type Agent struct {
    ID           string                 `json:"id"`
    Name         string                 `json:"name"`
    Description  string                 `json:"description"`
    BehaviorType string                 `json:"behavior_type"`
    Status       AgentStatus            `json:"status"`
    Config       AgentConfig            `json:"config"`
    Capabilities []string               `json:"capabilities"`
    Metadata     map[string]interface{} `json:"metadata"`
    CreatedAt    time.Time              `json:"created_at"`
    UpdatedAt    time.Time              `json:"updated_at"`
}

type AgentStatus string

const (
    StatusDraft    AgentStatus = "draft"
    StatusActive   AgentStatus = "active"
    StatusInactive AgentStatus = "inactive"
    StatusArchived AgentStatus = "archived"
)

type AgentConfig struct {
    LLMProvider string                 `json:"llm_provider"`
    LLMModel    string                 `json:"llm_model"`
    Temperature float64                `json:"temperature"`
    MaxTokens   int                    `json:"max_tokens"`
    Personality string                 `json:"personality"`
    Language    string                 `json:"language"`
    Custom      map[string]interface{} `json:"custom"`
}
```

### CreateAgentRequest

```go
type CreateAgentRequest struct {
    Name         string                 `json:"name"`
    Description  string                 `json:"description"`
    BehaviorType string                 `json:"behavior_type"`
    Config       AgentConfig            `json:"config"`
    Capabilities []string               `json:"capabilities"`
    Metadata     map[string]interface{} `json:"metadata"`
}
```

### UpdateAgentRequest

```go
type UpdateAgentRequest struct {
    Name         *string                 `json:"name,omitempty"`
    Description  *string                 `json:"description,omitempty"`
    Status       *AgentStatus            `json:"status,omitempty"`
    Config       *AgentConfig            `json:"config,omitempty"`
    Capabilities *[]string               `json:"capabilities,omitempty"`
    Metadata     *map[string]interface{} `json:"metadata,omitempty"`
}
```

### Input/Output

```go
type Input struct {
    Raw     interface{}            `json:"raw"`
    Type    string                 `json:"type"`
    Context map[string]interface{} `json:"context,omitempty"`
}

type Output struct {
    Result   interface{}            `json:"result"`
    Type     string                 `json:"type"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}
```

### ToolInput/ToolOutput

```go
type ToolInput struct {
    ToolName string                 `json:"tool_name"`
    Params   map[string]interface{} `json:"params"`
}

type ToolOutput struct {
    Success bool        `json:"success"`
    Result  interface{} `json:"result,omitempty"`
    Error   string      `json:"error,omitempty"`
}
```

### Metrics

```go
type Metrics struct {
    AgentID              string    `json:"agent_id"`
    TotalExecutions      int64     `json:"total_executions"`
    SuccessfulExecutions int64     `json:"successful_executions"`
    FailedExecutions     int64     `json:"failed_executions"`
    AvgExecutionTime     float64   `json:"avg_execution_time"`
    CreatedAt            time.Time `json:"created_at"`
    UpdatedAt            time.Time `json:"updated_at"`
}
```

### Activity

```go
type Activity struct {
    ID        string     `json:"id"`
    AgentID   string     `json:"agent_id"`
    Action    string     `json:"action"`
    Input     *Input     `json:"input,omitempty"`
    Output    *Output    `json:"output,omitempty"`
    Status    string     `json:"status"`
    Duration  int64      `json:"duration"`
    Error     string     `json:"error,omitempty"`
    CreatedAt time.Time  `json:"created_at"`
}
```

## LLM Package (`llm/`)

### Provider Interface

```go
type Provider interface {
    GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
    GenerateChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    Name() string
}
```

### CompletionRequest/Response

```go
type CompletionRequest struct {
    SystemPrompt string   `json:"system_prompt"`
    UserPrompt   string   `json:"user_prompt"`
    Temperature  float64  `json:"temperature"`
    MaxTokens    int      `json:"max_tokens"`
    Model        string   `json:"model"`
    Stop         []string `json:"stop,omitempty"`
}

type CompletionResponse struct {
    Text         string `json:"text"`
    TokensUsed   int    `json:"tokens_used"`
    Model        string `json:"model"`
    FinishReason string `json:"finish_reason"`
}
```

### ChatRequest/Response

```go
type ChatRequest struct {
    Messages    []Message `json:"messages"`
    Temperature float64   `json:"temperature"`
    MaxTokens   int       `json:"max_tokens"`
    Model       string    `json:"model"`
}

type ChatResponse struct {
    Message    Message `json:"message"`
    TokensUsed int     `json:"tokens_used"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}
```

### Provider Constructors

```go
// OpenAI
func NewOpenAI(apiKey string) Provider
func NewOpenAIWithTools(apiKey string) Provider

// Anthropic
func NewAnthropic(apiKey string) Provider

// Ollama
func NewOllama(endpoint string) Provider

// TupleLeap
func NewTupleLeap(apiKey string) Provider
```

## Tools Package (`tools/`)

### Tool Interface

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]interface{}  // JSON Schema
    Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error)
    RequiredCapabilities() []string
}
```

### Registry

```go
// Create new registry
func NewRegistry() Registry

// Register a tool
func (r *Registry) Register(tool Tool) error

// Get tool by name
func (r *Registry) Get(name string) (Tool, error)

// List all tool names
func (r *Registry) List() []string

// Execute tool
func (r *Registry) Execute(ctx context.Context, name string, input *models.ToolInput) (*models.ToolOutput, error)

// Get tools for agent (filtered by capabilities)
func (r *Registry) GetToolsForAgent(agent *models.Agent) []Tool
```

## Storage Package (`storage/`)

### Store Interface

```go
type Store interface {
    // Agent CRUD
    Create(ctx context.Context, agent *models.Agent) error
    Get(ctx context.Context, id string) (*models.Agent, error)
    Update(ctx context.Context, agent *models.Agent) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, req *models.ListAgentsRequest) ([]*models.Agent, int, error)

    // Metrics
    CreateMetrics(ctx context.Context, metrics *models.Metrics) error
    GetMetrics(ctx context.Context, agentID string) (*models.Metrics, error)
    UpdateMetrics(ctx context.Context, metrics *models.Metrics) error

    // Activities
    RecordActivity(ctx context.Context, activity *models.Activity) error
    GetActivities(ctx context.Context, agentID string, limit int) ([]*models.Activity, error)

    // Lifecycle
    Close() error
}
```

### In-Memory Storage

```go
func NewInMemory() Store
```

### PostgreSQL Storage

```go
type PostgreSQLConfig struct {
    Host     string
    Port     int
    User     string
    Password string
    Database string
    SSLMode  string
}

func NewPostgreSQL(config PostgreSQLConfig) (Store, error)
```

## A2A Protocol (`protocols/a2a/`)

### Server

```go
type ServerConfig struct {
    EnableCORS         bool
    AllowedOrigins     []string
    MaxConcurrentTasks int
    TaskTimeout        time.Duration
    EnableAuth         bool
    AuthTokens         []string
}

func DefaultServerConfig() ServerConfig
func NewA2AServer(framework core.Framework, agentID, baseURL string, config ServerConfig) (*Server, error)
func (s *Server) ListenAndServe(addr string) error
func (s *Server) Handler() http.Handler
```

### Client

```go
type ClientConfig struct {
    Timeout        time.Duration
    Headers        map[string]string
    BearerToken    string
    RetryAttempts  int
    RetryDelay     time.Duration
    FetchAgentCard bool
}

func DefaultClientConfig() ClientConfig
func NewClient(baseURL string, config ClientConfig) *Client
func (c *Client) Connect(ctx context.Context) error
func (c *Client) AgentCard() *AgentCard
func (c *Client) SendTask(ctx context.Context, params TaskSendParams) (*Task, error)
func (c *Client) SendTaskStream(ctx context.Context, params TaskSendParams) (<-chan TaskUpdate, error)
func (c *Client) GetTask(ctx context.Context, taskID string, historyLength int) (*Task, error)
func (c *Client) CancelTask(ctx context.Context, taskID string) (*Task, error)
```

### Agent Card

```go
type AgentCard struct {
    Name         string           `json:"name"`
    Description  string           `json:"description"`
    URL          string           `json:"url"`
    Version      string           `json:"version,omitempty"`
    Capabilities AgentCapabilities `json:"capabilities"`
    Skills       []AgentSkill     `json:"skills,omitempty"`
    Provider     *ProviderInfo    `json:"provider,omitempty"`
}

func NewAgentCardBuilder(name, description, url string) *AgentCardBuilder
func (b *AgentCardBuilder) WithVersion(version string) *AgentCardBuilder
func (b *AgentCardBuilder) WithStreaming(enabled bool) *AgentCardBuilder
func (b *AgentCardBuilder) AddSkill(skill AgentSkill) *AgentCardBuilder
func (b *AgentCardBuilder) Build() AgentCard
```

### Task

```go
type TaskState string

const (
    TaskStateSubmitted TaskState = "submitted"
    TaskStateWorking   TaskState = "working"
    TaskStateCompleted TaskState = "completed"
    TaskStateFailed    TaskState = "failed"
    TaskStateCanceled  TaskState = "canceled"
)

type Task struct {
    ID       string     `json:"id"`
    Status   TaskStatus `json:"status"`
    History  []Message  `json:"history,omitempty"`
    Metadata any        `json:"metadata,omitempty"`
}

func NewTask(id string) *Task
func (t *Task) UpdateState(state TaskState, msg *Message)
func (t *Task) AddMessage(msg Message)
```

## AG-UI Protocol (`protocols/agui/`)

### Server

```go
type ServerConfig struct {
    EnableCORS     bool
    AllowedOrigins []string
    EventTimeout   time.Duration
}

func DefaultServerConfig() ServerConfig
func NewAGUIServer(framework core.Framework, agentID string, config ServerConfig) (*Server, error)
func (s *Server) ListenAndServe(addr string) error
func (s *Server) Handler() http.Handler
```

### Event Emitter

```go
func NewEventEmitter(threadID, runID string, bufferSize int) *EventEmitter
func (e *EventEmitter) Events() <-chan Event
func (e *EventEmitter) Close()

// Event methods
func (e *EventEmitter) EmitRunStarted()
func (e *EventEmitter) EmitRunFinished()
func (e *EventEmitter) EmitRunError(err error)
func (e *EventEmitter) EmitTextMessageStart(msgID string, role Role) string
func (e *EventEmitter) EmitTextMessageContent(msgID, content string)
func (e *EventEmitter) EmitTextMessageEnd(msgID string)
func (e *EventEmitter) EmitToolCallStart(toolName, msgID string) string
func (e *EventEmitter) EmitToolCallArgs(toolCallID, args string)
func (e *EventEmitter) EmitToolCallEnd(toolCallID, result string)
func (e *EventEmitter) EmitStateDelta(delta []JSONPatchOperation)
```

### Streaming Helpers

```go
// Streaming message
func (e *EventEmitter) NewStreamingMessage(role Role) *StreamingMessage
func (m *StreamingMessage) WriteString(s string)
func (m *StreamingMessage) End()
func (m *StreamingMessage) Content() string
func (m *StreamingMessage) MessageID() string

// Streaming tool call
func (e *EventEmitter) NewStreamingToolCall(toolName, msgID string) *StreamingToolCall
func (t *StreamingToolCall) WriteArgs(args string)
func (t *StreamingToolCall) End(result string)
func (t *StreamingToolCall) Args() string
func (t *StreamingToolCall) ToolCallID() string
```

### State Manager

```go
func NewStateManager(emitter *EventEmitter) *StateManager
func NewStateManagerWithInitial(emitter *EventEmitter, initial map[string]any) *StateManager
func (s *StateManager) Set(path string, value any) error
func (s *StateManager) Get(path string) (any, bool)
func (s *StateManager) GetAll() map[string]any
func (s *StateManager) Remove(path string) error
func (s *StateManager) Patch(ops []JSONPatchOperation) error
```

### Event Types

```go
type EventType string

const (
    EventRunStarted         EventType = "RUN_STARTED"
    EventRunFinished        EventType = "RUN_FINISHED"
    EventRunError           EventType = "RUN_ERROR"
    EventTextMessageStart   EventType = "TEXT_MESSAGE_START"
    EventTextMessageContent EventType = "TEXT_MESSAGE_CONTENT"
    EventTextMessageEnd     EventType = "TEXT_MESSAGE_END"
    EventToolCallStart      EventType = "TOOL_CALL_START"
    EventToolCallArgs       EventType = "TOOL_CALL_ARGS"
    EventToolCallEnd        EventType = "TOOL_CALL_END"
    EventStateDelta         EventType = "STATE_DELTA"
)
```

## MCP Package (`mcp/`)

### Client Configuration

```go
type ClientConfig struct {
    ServerName string
    Command    string
    Args       []string
    Env        map[string]string
    WorkDir    string
}
```

### Client Manager

```go
func NewMCPClientManager(config *ManagerConfig) *MCPClientManager
func (m *MCPClientManager) ConnectServer(ctx context.Context, config *ClientConfig) error
func (m *MCPClientManager) DisconnectServer(name string) error
func (m *MCPClientManager) ListServers() []string
func (m *MCPClientManager) GetStatus() map[string]ServerStatus
func (m *MCPClientManager) Close() error
```

## Skills Package (`skills/`)

### Skill Interface

```go
type Skill interface {
    Name() string
    Description() string
    Category() string
    Tags() []string
    Version() string
    Dependencies() []string
    RequiredCapabilities() []string
    CanExecute(agent *models.Agent) bool
    Execute(ctx context.Context, input *SkillInput) (*SkillOutput, error)
}
```

### SkillInput/SkillOutput

```go
type SkillInput struct {
    Query             string
    Parameters        map[string]any
    Context           map[string]any
    Agent             *models.Agent
    DependencyResults map[string]*SkillOutput
}

type SkillOutput struct {
    Result      any
    Explanation string
    Metadata    map[string]any
    Error       string
}
```

### Registry

```go
func NewSkillRegistry() *InMemorySkillRegistry
func (r *InMemorySkillRegistry) Register(skill Skill) error
func (r *InMemorySkillRegistry) Get(name string) (Skill, error)
func (r *InMemorySkillRegistry) List() []SkillInfo
func (r *InMemorySkillRegistry) GetForAgent(agent *models.Agent) []Skill
func (r *InMemorySkillRegistry) LoadDirectory(path string) (int, error)
func (r *InMemorySkillRegistry) WatchDirectory(path string) error
func (r *InMemorySkillRegistry) StopWatching() error
func (r *InMemorySkillRegistry) GetDependencies(skillName string) ([]Skill, error)
```

## Memory Package (`memory/`)

### Buffer Memory

```go
func NewBufferMemory(maxMessages int) *BufferMemory
func (m *BufferMemory) Add(ctx context.Context, message Message) error
func (m *BufferMemory) GetHistory(ctx context.Context) ([]Message, error)
func (m *BufferMemory) Clear(ctx context.Context) error
```

### Semantic Memory

```go
func NewSemanticMemory(embedder Embedder, store VectorStore) *SemanticMemory
func (m *SemanticMemory) Store(ctx context.Context, content string, metadata map[string]any) error
func (m *SemanticMemory) Search(ctx context.Context, query string, topK int) ([]SearchResult, error)
func (m *SemanticMemory) Delete(ctx context.Context, id string) error
```

### Knowledge Graph Memory

```go
func NewKnowledgeGraphMemory(driver neo4j.Driver) *KnowledgeGraphMemory
func (m *KnowledgeGraphMemory) AddEntity(ctx context.Context, entity Entity) error
func (m *KnowledgeGraphMemory) AddRelation(ctx context.Context, from, relation, to string) error
func (m *KnowledgeGraphMemory) FindPath(ctx context.Context, start, end string) ([]string, error)
func (m *KnowledgeGraphMemory) Query(ctx context.Context, cypher string) ([]map[string]any, error)
```

## Chain Package (`chain/`)

### Chain Interface

```go
type Chain interface {
    Run(ctx context.Context, input any) (any, error)
    Name() string
}
```

### LLM Chain

```go
func NewLLMChain(provider llm.Provider, promptTemplate string) *LLMChain
func (c *LLMChain) Run(ctx context.Context, input any) (any, error)
func (c *LLMChain) WithParser(parser OutputParser) *LLMChain
```

### RAG Chain

```go
func NewRAGChain(retriever Retriever, llmChain *LLMChain) *RAGChain
func (c *RAGChain) Run(ctx context.Context, query any) (any, error)
```

### Sequential Chain

```go
func NewSequentialChain(chains ...Chain) *SequentialChain
func (c *SequentialChain) Run(ctx context.Context, input any) (any, error)
```

### Router Chain

```go
type Route struct {
    Condition string
    Chain     Chain
}

func NewRouterChain(routes ...Route) *RouterChain
func (c *RouterChain) Run(ctx context.Context, input any) (any, error)
```

## Observability Package (`observability/`)

### Tracing

```go
type Config struct {
    ServiceName string
    Endpoint    string
    Sampler     float64
}

func InitTracer(config Config) error
func GetTracer() *Tracer
func StartAgentSpan(ctx context.Context, agentID, agentName, operation string) (context.Context, trace.Span)
func StartToolSpan(ctx context.Context, toolName string, params map[string]any) (context.Context, trace.Span)
func StartLLMSpan(ctx context.Context, provider, model string) (context.Context, trace.Span)
func StartSpan(ctx context.Context, name string, kind SpanKind) (context.Context, trace.Span)
func EndSpan(span trace.Span, err error)
func RecordError(span trace.Span, err error, description string)
```

## Metrics Package (`metrics/`)

### Initialization

```go
type Config struct {
    Endpoint string
}

func InitMetrics(config Config) error
```

### Metric Types

```go
func NewCounter(name string, labels Labels) *Counter
func (c *Counter) Inc()
func (c *Counter) Add(value float64)

func NewGauge(name string, labels Labels) *Gauge
func (g *Gauge) Set(value float64)
func (g *Gauge) Inc()
func (g *Gauge) Dec()

func NewHistogram(name string, labels Labels) *Histogram
func (h *Histogram) Observe(value float64)
```

### Built-in Metrics

```go
const (
    MetricAgentExecutionsTotal = "agent_executions_total"
    MetricLLMCallsTotal        = "llm_calls_total"
    MetricLLMCallErrors        = "llm_call_errors_total"
    MetricLLMTokensUsed        = "llm_tokens_used_total"
    MetricLLMCallDuration      = "llm_call_duration_seconds"
)
```

## Debug Package (`debug/`)

### Recorder

```go
func NewRecorder(store SnapshotStore) *Recorder
func (r *Recorder) Snapshot(ctx context.Context, state ExecutionState) error
func (r *Recorder) GetSnapshots(ctx context.Context, executionID string) ([]Snapshot, error)
```

### Timeline

```go
func NewTimeline(recorder *Recorder) *Timeline
func (t *Timeline) Load(ctx context.Context, executionID string) error
func (t *Timeline) StepForward() error
func (t *Timeline) StepBackward() error
func (t *Timeline) JumpTo(index int) error
func (t *Timeline) GetCurrentState() ExecutionState
func (t *Timeline) GetCurrentIndex() int
func (t *Timeline) Length() int
```
