package a2a

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Server implements the A2A protocol HTTP server with JSON-RPC and SSE support
type Server struct {
	mu          sync.RWMutex
	card        *AgentCard
	taskManager *TaskManager
	config      ServerConfig
	subscribers map[string][]chan TaskUpdate // taskID -> subscribers
}

// ServerConfig configures the A2A server
type ServerConfig struct {
	// EnableCORS enables CORS headers
	EnableCORS bool

	// AllowedOrigins for CORS (empty = allow all)
	AllowedOrigins []string

	// EnableSSE enables Server-Sent Events
	EnableSSE bool

	// SSEHeartbeatInterval is how often to send heartbeats
	SSEHeartbeatInterval time.Duration

	// MaxSubscribersPerTask limits SSE subscribers per task
	MaxSubscribersPerTask int

	// RequestTimeout for processing requests
	RequestTimeout time.Duration

	// EnableAuthentication enables auth checking
	EnableAuthentication bool

	// AuthValidator validates authentication
	AuthValidator AuthValidator
}

// AuthValidator validates authentication credentials
type AuthValidator interface {
	// ValidateToken validates a bearer token
	ValidateToken(token string) (bool, error)

	// ValidateBasic validates basic auth credentials
	ValidateBasic(username, password string) (bool, error)
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		EnableCORS:            true,
		EnableSSE:             true,
		SSEHeartbeatInterval:  30 * time.Second,
		MaxSubscribersPerTask: 10,
		RequestTimeout:        5 * time.Minute,
	}
}

// NewServer creates a new A2A server
func NewServer(card *AgentCard, taskManager *TaskManager, config ServerConfig) *Server {
	return &Server{
		card:        card,
		taskManager: taskManager,
		config:      config,
		subscribers: make(map[string][]chan TaskUpdate),
	}
}

// Handler returns the HTTP handler for the A2A server
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Agent card discovery
	mux.HandleFunc("/.well-known/agent.json", s.handleAgentCard)

	// JSON-RPC endpoint
	mux.HandleFunc("/", s.handleJSONRPC)

	// Wrap with middleware
	var handler http.Handler = mux
	if s.config.EnableCORS {
		handler = s.corsMiddleware(handler)
	}
	if s.config.EnableAuthentication {
		handler = s.authMiddleware(handler)
	}

	return handler
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			allowed := len(s.config.AllowedOrigins) == 0
			for _, o := range s.config.AllowedOrigins {
				if o == origin || o == "*" {
					allowed = true
					break
				}
			}
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// authMiddleware validates authentication
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for agent card and OPTIONS
		if r.URL.Path == "/.well-known/agent.json" || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		if s.config.AuthValidator == nil {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			s.writeError(w, nil, ErrorCodeInvalidRequest, "Authentication required", nil)
			return
		}

		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 {
			s.writeError(w, nil, ErrorCodeInvalidRequest, "Invalid authorization header", nil)
			return
		}

		var valid bool
		var err error

		switch strings.ToLower(parts[0]) {
		case "bearer":
			valid, err = s.config.AuthValidator.ValidateToken(parts[1])
		case "basic":
			// Decode basic auth
			// For simplicity, we'll just pass the encoded value
			valid, err = s.config.AuthValidator.ValidateBasic(parts[1], "")
		default:
			s.writeError(w, nil, ErrorCodeInvalidRequest, "Unsupported authentication scheme", nil)
			return
		}

		if err != nil || !valid {
			s.writeError(w, nil, ErrorCodeInvalidRequest, "Authentication failed", nil)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleAgentCard serves the agent card
func (s *Server) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	data, err := s.card.ToJSON()
	if err != nil {
		http.Error(w, "Failed to serialize agent card", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// handleJSONRPC processes JSON-RPC requests
func (s *Server) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for SSE accept header
	accept := r.Header.Get("Accept")
	isSSE := strings.Contains(accept, "text/event-stream")

	// Parse request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, nil, ErrorCodeParseError, "Failed to read request body", nil)
		return
	}
	defer r.Body.Close()

	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeError(w, nil, ErrorCodeParseError, "Failed to parse JSON-RPC request", nil)
		return
	}

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		s.writeError(w, req.ID, ErrorCodeInvalidRequest, "Invalid JSON-RPC version", nil)
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), s.config.RequestTimeout)
	defer cancel()

	// Route to appropriate handler
	switch req.Method {
	case MethodTaskSend:
		s.handleTaskSend(ctx, w, req)
	case MethodTaskSendSubscribe:
		if isSSE && s.config.EnableSSE {
			s.handleTaskSendSubscribeSSE(ctx, w, r, req)
		} else {
			s.handleTaskSendSubscribe(ctx, w, req)
		}
	case MethodTaskGet:
		s.handleTaskGet(ctx, w, req)
	case MethodTaskCancel:
		s.handleTaskCancel(ctx, w, req)
	case MethodTaskResubscribe:
		if isSSE && s.config.EnableSSE {
			s.handleTaskResubscribeSSE(ctx, w, r, req)
		} else {
			s.writeError(w, req.ID, ErrorCodeUnsupportedMethod, "Resubscribe requires SSE", nil)
		}
	default:
		s.writeError(w, req.ID, ErrorCodeMethodNotFound, fmt.Sprintf("Method %s not found", req.Method), nil)
	}
}

// handleTaskSend handles synchronous task sending
func (s *Server) handleTaskSend(ctx context.Context, w http.ResponseWriter, req JSONRPCRequest) {
	var params TaskSendParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeError(w, req.ID, ErrorCodeInvalidParams, "Invalid params", err.Error())
		return
	}

	// Create task
	task, err := s.taskManager.CreateTask(params)
	if err != nil {
		s.writeError(w, req.ID, ErrorCodeInternalError, "Failed to create task", err.Error())
		return
	}

	// Process task synchronously
	if err := s.taskManager.ProcessTask(ctx, task.ID); err != nil {
		// Task already marked as failed, just return the task
	}

	// Get updated task
	task, _ = s.taskManager.GetTask(task.ID)
	s.writeResponse(w, req.ID, task)
}

// handleTaskSendSubscribe handles task sending with subscription (non-SSE)
func (s *Server) handleTaskSendSubscribe(ctx context.Context, w http.ResponseWriter, req JSONRPCRequest) {
	var params TaskSendParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeError(w, req.ID, ErrorCodeInvalidParams, "Invalid params", err.Error())
		return
	}

	// Create task
	task, err := s.taskManager.CreateTask(params)
	if err != nil {
		s.writeError(w, req.ID, ErrorCodeInternalError, "Failed to create task", err.Error())
		return
	}

	// Process task with streaming (but we'll collect all updates)
	updates, err := s.taskManager.ProcessTaskStream(ctx, task.ID)
	if err != nil {
		// Fall back to sync processing
		if err := s.taskManager.ProcessTask(ctx, task.ID); err != nil {
			// Task already marked as failed
		}
		task, _ = s.taskManager.GetTask(task.ID)
		s.writeResponse(w, req.ID, task)
		return
	}

	// Wait for all updates
	for range updates {
		// Just consume updates
	}

	// Get final task state
	task, _ = s.taskManager.GetTask(task.ID)
	s.writeResponse(w, req.ID, task)
}

// handleTaskSendSubscribeSSE handles task sending with SSE streaming
func (s *Server) handleTaskSendSubscribeSSE(ctx context.Context, w http.ResponseWriter, r *http.Request, req JSONRPCRequest) {
	var params TaskSendParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeError(w, req.ID, ErrorCodeInvalidParams, "Invalid params", err.Error())
		return
	}

	// Create task
	task, err := s.taskManager.CreateTask(params)
	if err != nil {
		s.writeError(w, req.ID, ErrorCodeInternalError, "Failed to create task", err.Error())
		return
	}

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial task state
	s.writeSSEEvent(w, flusher, SSEEventTaskStatus, TaskStatusUpdate{
		TaskID: task.ID,
		Status: task.Status,
	})

	// Process task with streaming
	updates, err := s.taskManager.ProcessTaskStream(ctx, task.ID)
	if err != nil {
		s.writeSSEEvent(w, flusher, SSEEventError, map[string]string{"error": err.Error()})
		s.writeSSEEvent(w, flusher, SSEEventDone, nil)
		return
	}

	// Stream updates via SSE
	s.streamUpdates(ctx, w, flusher, task.ID, updates)
}

// handleTaskGet handles task status queries
func (s *Server) handleTaskGet(ctx context.Context, w http.ResponseWriter, req JSONRPCRequest) {
	var params TaskQueryParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeError(w, req.ID, ErrorCodeInvalidParams, "Invalid params", err.Error())
		return
	}

	task, err := s.taskManager.GetTaskWithHistory(params.ID, params.HistoryLength)
	if err != nil {
		s.writeError(w, req.ID, ErrorCodeTaskNotFound, "Task not found", err.Error())
		return
	}

	s.writeResponse(w, req.ID, task)
}

// handleTaskCancel handles task cancellation
func (s *Server) handleTaskCancel(ctx context.Context, w http.ResponseWriter, req JSONRPCRequest) {
	var params TaskCancelParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeError(w, req.ID, ErrorCodeInvalidParams, "Invalid params", err.Error())
		return
	}

	if err := s.taskManager.CancelTask(params.ID); err != nil {
		s.writeError(w, req.ID, ErrorCodeTaskNotFound, "Failed to cancel task", err.Error())
		return
	}

	task, _ := s.taskManager.GetTask(params.ID)
	s.writeResponse(w, req.ID, task)
}

// handleTaskResubscribeSSE handles resubscription to task updates
func (s *Server) handleTaskResubscribeSSE(ctx context.Context, w http.ResponseWriter, r *http.Request, req JSONRPCRequest) {
	var params TaskQueryParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeError(w, req.ID, ErrorCodeInvalidParams, "Invalid params", err.Error())
		return
	}

	task, err := s.taskManager.GetTask(params.ID)
	if err != nil {
		s.writeError(w, req.ID, ErrorCodeTaskNotFound, "Task not found", err.Error())
		return
	}

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send current task state
	s.writeSSEEvent(w, flusher, SSEEventTaskStatus, TaskStatusUpdate{
		TaskID: task.ID,
		Status: task.Status,
	})

	// If task is in terminal state, we're done
	if task.Status.State == TaskStateCompleted || task.Status.State == TaskStateFailed || task.Status.State == TaskStateCanceled {
		s.writeSSEEvent(w, flusher, SSEEventDone, nil)
		return
	}

	// Subscribe to updates
	updateChan := make(chan TaskUpdate, 100)
	s.subscribe(task.ID, updateChan)
	defer s.unsubscribe(task.ID, updateChan)

	// Stream updates
	s.streamUpdates(ctx, w, flusher, task.ID, updateChan)
}

// subscribe adds a subscriber for task updates
func (s *Server) subscribe(taskID string, ch chan TaskUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subscribers[taskID] = append(s.subscribers[taskID], ch)
}

// unsubscribe removes a subscriber
func (s *Server) unsubscribe(taskID string, ch chan TaskUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subs := s.subscribers[taskID]
	for i, sub := range subs {
		if sub == ch {
			s.subscribers[taskID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
}

// notifySubscribers notifies all subscribers of a task update
func (s *Server) notifySubscribers(taskID string, update TaskUpdate) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ch := range s.subscribers[taskID] {
		select {
		case ch <- update:
		default:
			// Subscriber is slow, skip
		}
	}
}

// streamUpdates streams task updates via SSE
func (s *Server) streamUpdates(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, taskID string, updates <-chan TaskUpdate) {
	heartbeat := time.NewTicker(s.config.SSEHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			s.writeSSEEvent(w, flusher, SSEEventDone, nil)
			return

		case <-heartbeat.C:
			// Send heartbeat comment
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()

		case update, ok := <-updates:
			if !ok {
				s.writeSSEEvent(w, flusher, SSEEventDone, nil)
				return
			}

			switch update.Type {
			case TaskUpdateTypeStatus:
				s.writeSSEEvent(w, flusher, SSEEventTaskStatus, TaskStatusUpdate{
					TaskID: taskID,
					Status: *update.Status,
					Final:  update.Status.State == TaskStateCompleted || update.Status.State == TaskStateFailed || update.Status.State == TaskStateCanceled,
				})
				if update.Status.State == TaskStateCompleted || update.Status.State == TaskStateFailed || update.Status.State == TaskStateCanceled {
					s.writeSSEEvent(w, flusher, SSEEventDone, nil)
					return
				}

			case TaskUpdateTypeArtifact:
				s.writeSSEEvent(w, flusher, SSEEventTaskArtifact, TaskArtifactUpdate{
					TaskID:   taskID,
					Artifact: *update.Artifact,
				})

			case TaskUpdateTypeMessage:
				s.writeSSEEvent(w, flusher, SSEEventTaskMessage, update.Message)

			case TaskUpdateTypeError:
				s.writeSSEEvent(w, flusher, SSEEventError, map[string]string{
					"error": update.Error.Error(),
				})
			}

			// Also notify other subscribers
			s.notifySubscribers(taskID, update)
		}
	}
}

// writeSSEEvent writes an SSE event
func (s *Server) writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, event SSEEventType, data any) {
	eventData, _ := json.Marshal(data)
	fmt.Fprintf(w, "id: %s\n", uuid.New().String())
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", eventData)
	flusher.Flush()
}

// writeResponse writes a successful JSON-RPC response
func (s *Server) writeResponse(w http.ResponseWriter, id any, result any) {
	w.Header().Set("Content-Type", "application/json")
	resp := NewJSONRPCResponse(id, result)
	json.NewEncoder(w).Encode(resp)
}

// writeError writes a JSON-RPC error response
func (s *Server) writeError(w http.ResponseWriter, id any, code int, message string, data any) {
	w.Header().Set("Content-Type", "application/json")
	resp := NewJSONRPCError(id, code, message, data)
	json.NewEncoder(w).Encode(resp)
}

// ListenAndServe starts the A2A server
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.Handler())
}

// SSEClient reads SSE events from a response body
type SSEClient struct {
	reader *bufio.Reader
}

// NewSSEClient creates a new SSE client from a response body
func NewSSEClient(body io.Reader) *SSEClient {
	return &SSEClient{
		reader: bufio.NewReader(body),
	}
}

// ReadEvent reads the next SSE event
func (c *SSEClient) ReadEvent() (*SSEEvent, error) {
	var event SSEEvent
	var eventType, data string

	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)

		// Empty line signals end of event
		if line == "" {
			if eventType != "" && data != "" {
				event.Event = SSEEventType(eventType)
				if err := json.Unmarshal([]byte(data), &event.Data); err != nil {
					event.Data = data // Use raw string if not JSON
				}
				return &event, nil
			}
			continue
		}

		// Skip comments
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Parse field
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		field := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch field {
		case "event":
			eventType = value
		case "data":
			data = value
		case "id":
			// Event ID, can be used for replay
		case "retry":
			// Retry interval, can be used for reconnection
		}
	}
}
