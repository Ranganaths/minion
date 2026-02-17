package agui

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

// Handler processes AG-UI run requests
type Handler interface {
	// HandleRun processes a run request and streams events
	HandleRun(ctx context.Context, req RunRequest, emitter *EventEmitter) error
}

// Server implements the AG-UI protocol HTTP server
type Server struct {
	mu      sync.RWMutex
	handler Handler
	config  ServerConfig
	runs    map[string]*runState // runID -> state
}

// runState tracks the state of a running request
type runState struct {
	request   RunRequest
	state     *StateManager
	startTime time.Time
	done      chan struct{}
}

// ServerConfig configures the AG-UI server
type ServerConfig struct {
	// EnableCORS enables CORS headers
	EnableCORS bool

	// AllowedOrigins for CORS (empty = allow all)
	AllowedOrigins []string

	// HeartbeatInterval for SSE keepalive
	HeartbeatInterval time.Duration

	// RequestTimeout for processing requests
	RequestTimeout time.Duration

	// MaxConcurrentRuns limits concurrent runs
	MaxConcurrentRuns int

	// EnableAuthentication enables auth checking
	EnableAuthentication bool

	// AuthValidator validates authentication
	AuthValidator func(token string) bool
}

// DefaultServerConfig returns default configuration
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		EnableCORS:        true,
		HeartbeatInterval: 30 * time.Second,
		RequestTimeout:    10 * time.Minute,
		MaxConcurrentRuns: 100,
	}
}

// NewServer creates a new AG-UI server
func NewServer(handler Handler, config ServerConfig) *Server {
	return &Server{
		handler: handler,
		config:  config,
		runs:    make(map[string]*runState),
	}
}

// Handler returns the HTTP handler
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Main run endpoint
	mux.HandleFunc("/", s.handleRun)

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
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		if s.config.AuthValidator == nil {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if !s.config.AuthValidator(token) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleRun handles run requests with SSE streaming
func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Generate run ID if not provided
	if req.RunID == "" {
		req.RunID = uuid.New().String()
	}

	// Check concurrent run limit
	s.mu.Lock()
	if len(s.runs) >= s.config.MaxConcurrentRuns {
		s.mu.Unlock()
		s.writeError(w, "Too many concurrent runs", http.StatusTooManyRequests)
		return
	}

	// Create run state
	state := &runState{
		request:   req,
		startTime: time.Now(),
		done:      make(chan struct{}),
	}
	s.runs[req.RunID] = state
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.runs, req.RunID)
		s.mu.Unlock()
		close(state.done)
	}()

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), s.config.RequestTimeout)
	defer cancel()

	// Create event emitter
	emitter := NewEventEmitter(req.ThreadID, req.RunID, 100)
	defer emitter.Close()

	// Create state manager
	stateManager := NewStateManagerWithInitial(emitter, req.State)
	state.state = stateManager

	// Start processing in goroutine
	go func() {
		if err := s.handler.HandleRun(ctx, req, emitter); err != nil {
			emitter.EmitRunError(err.Error(), "HANDLER_ERROR")
		}
	}()

	// Stream events
	s.streamEvents(ctx, w, flusher, emitter)
}

// streamEvents streams events via SSE
func (s *Server) streamEvents(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, emitter *EventEmitter) {
	heartbeat := time.NewTicker(s.config.HeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-heartbeat.C:
			// Send heartbeat comment
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()

		case event, ok := <-emitter.Events():
			if !ok {
				return
			}

			// Serialize event
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}

			// Write SSE event
			fmt.Fprintf(w, "event: %s\n", event.Type)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			// Check for terminal events
			if event.Type == EventRunFinished || event.Type == EventRunError {
				return
			}
		}
	}
}

// writeError writes an error response
func (s *Server) writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// ListenAndServe starts the server
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.Handler())
}

// GetRunState returns the state for a run
func (s *Server) GetRunState(runID string) (*StateManager, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if state, ok := s.runs[runID]; ok {
		return state.state, true
	}
	return nil, false
}

// Stats returns server statistics
func (s *Server) Stats() ServerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ServerStats{
		ActiveRuns: len(s.runs),
	}
}

// ServerStats contains server statistics
type ServerStats struct {
	ActiveRuns int
}

// Client is an AG-UI client for connecting to AG-UI servers
type Client struct {
	httpClient *http.Client
	baseURL    string
	config     ClientConfig
}

// ClientConfig configures the client
type ClientConfig struct {
	// Timeout for requests
	Timeout time.Duration

	// BearerToken for authentication
	BearerToken string

	// Headers to include in requests
	Headers map[string]string
}

// DefaultClientConfig returns default client configuration
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Timeout: 10 * time.Minute,
		Headers: make(map[string]string),
	}
}

// NewClient creates a new AG-UI client
func NewClient(baseURL string, config ClientConfig) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		baseURL: baseURL,
		config:  config,
	}
}

// Run sends a run request and returns an event channel
func (c *Client) Run(ctx context.Context, req RunRequest) (<-chan Event, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	if c.config.BearerToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.config.BearerToken)
	}

	for k, v := range c.config.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return c.readEvents(ctx, resp.Body), nil
}

// readEvents reads SSE events from a response body
func (c *Client) readEvents(ctx context.Context, body io.ReadCloser) <-chan Event {
	events := make(chan Event, 100)

	go func() {
		defer close(events)
		defer body.Close()

		reader := bufio.NewReader(body)
		var eventType, data string

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					events <- Event{
						BaseEvent: BaseEvent{Type: EventRunError},
						RunError: &RunErrorEvent{
							BaseEvent: BaseEvent{Type: EventRunError},
							Message:   err.Error(),
						},
					}
				}
				return
			}

			line = strings.TrimSpace(line)

			// Empty line signals end of event
			if line == "" {
				if eventType != "" && data != "" {
					var event Event
					if err := json.Unmarshal([]byte(data), &event); err == nil {
						events <- event

						// Check for terminal events
						if event.Type == EventRunFinished || event.Type == EventRunError {
							return
						}
					}
				}
				eventType = ""
				data = ""
				continue
			}

			// Skip comments
			if line[0] == ':' {
				continue
			}

			// Parse field
			if strings.HasPrefix(line, "event:") {
				eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") {
				data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			}
		}
	}()

	return events
}

// RunSync sends a run request and collects all events
func (c *Client) RunSync(ctx context.Context, req RunRequest) ([]Event, error) {
	eventChan, err := c.Run(ctx, req)
	if err != nil {
		return nil, err
	}

	var events []Event
	for event := range eventChan {
		events = append(events, event)
	}

	return events, nil
}

// CollectMessages collects all text messages from events
func CollectMessages(events []Event) []Message {
	messages := make(map[string]*Message)

	for _, event := range events {
		switch event.Type {
		case EventTextMessageStart:
			if event.TextMessageStart != nil {
				messages[event.TextMessageStart.MessageID] = &Message{
					ID:   event.TextMessageStart.MessageID,
					Role: event.TextMessageStart.Role,
				}
			}
		case EventTextMessageContent:
			if event.TextMessageContent != nil {
				if msg, ok := messages[event.TextMessageContent.MessageID]; ok {
					msg.Content += event.TextMessageContent.Delta
				}
			}
		}
	}

	result := make([]Message, 0, len(messages))
	for _, msg := range messages {
		result = append(result, *msg)
	}

	return result
}
