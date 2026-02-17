// Package a2a implements the Agent-to-Agent (A2A) protocol for inter-agent communication.
// A2A is an open protocol developed by Google and managed by the Linux Foundation,
// enabling seamless communication between AI agents built on different frameworks.
//
// Specification: https://a2a-protocol.org/latest/
package a2a

import (
	"encoding/json"
	"time"
)

// Protocol version
const (
	ProtocolVersion = "1.0"
	ProtocolName    = "a2a"
)

// TaskState represents the lifecycle state of an A2A task
type TaskState string

const (
	// TaskStateSubmitted indicates the task has been submitted but not yet started
	TaskStateSubmitted TaskState = "submitted"

	// TaskStateWorking indicates the task is currently being processed
	TaskStateWorking TaskState = "working"

	// TaskStateCompleted indicates the task has completed successfully
	TaskStateCompleted TaskState = "completed"

	// TaskStateFailed indicates the task has failed
	TaskStateFailed TaskState = "failed"

	// TaskStateInputRequired indicates the task needs additional input from the user
	TaskStateInputRequired TaskState = "input-required"

	// TaskStateCanceled indicates the task was canceled
	TaskStateCanceled TaskState = "canceled"
)

// MessageRole represents the role of a message sender
type MessageRole string

const (
	MessageRoleUser  MessageRole = "user"
	MessageRoleAgent MessageRole = "agent"
)

// PartType represents the type of content in a message part
type PartType string

const (
	PartTypeText  PartType = "text"
	PartTypeFile  PartType = "file"
	PartTypeData  PartType = "data"
	PartTypeImage PartType = "image"
)

// Part represents a component of a message (text, file, data, etc.)
type Part struct {
	// Type of the part
	Type PartType `json:"type"`

	// Text content (for text parts)
	Text string `json:"text,omitempty"`

	// MimeType for file/data parts
	MimeType string `json:"mimeType,omitempty"`

	// Data contains base64-encoded binary data
	Data string `json:"data,omitempty"`

	// URI for file references
	URI string `json:"uri,omitempty"`

	// Name of the file or data
	Name string `json:"name,omitempty"`

	// Metadata for additional context
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewTextPart creates a new text part
func NewTextPart(text string) Part {
	return Part{
		Type: PartTypeText,
		Text: text,
	}
}

// NewFilePart creates a new file part
func NewFilePart(name, mimeType, uri string) Part {
	return Part{
		Type:     PartTypeFile,
		Name:     name,
		MimeType: mimeType,
		URI:      uri,
	}
}

// NewDataPart creates a new data part with base64-encoded content
func NewDataPart(name, mimeType, data string) Part {
	return Part{
		Type:     PartTypeData,
		Name:     name,
		MimeType: mimeType,
		Data:     data,
	}
}

// Message represents a message in an A2A conversation
type Message struct {
	// Role indicates who sent the message (user or agent)
	Role MessageRole `json:"role"`

	// Parts contains the message content
	Parts []Part `json:"parts"`

	// Metadata for additional context
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewTextMessage creates a simple text message
func NewTextMessage(role MessageRole, text string) Message {
	return Message{
		Role:  role,
		Parts: []Part{NewTextPart(text)},
	}
}

// GetText returns concatenated text from all text parts
func (m *Message) GetText() string {
	var text string
	for _, part := range m.Parts {
		if part.Type == PartTypeText {
			text += part.Text
		}
	}
	return text
}

// Artifact represents an output artifact from task execution
type Artifact struct {
	// ID uniquely identifies the artifact
	ID string `json:"id"`

	// Name of the artifact
	Name string `json:"name"`

	// Description of what the artifact contains
	Description string `json:"description,omitempty"`

	// Parts contain the artifact content
	Parts []Part `json:"parts"`

	// Index for ordering multiple artifacts
	Index int `json:"index,omitempty"`

	// Append indicates if this artifact appends to a previous one
	Append bool `json:"append,omitempty"`

	// LastChunk indicates this is the final chunk
	LastChunk bool `json:"lastChunk,omitempty"`

	// Metadata for additional context
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TaskStatus represents the current status of a task
type TaskStatus struct {
	// State is the current task state
	State TaskState `json:"state"`

	// Message provides additional status information
	Message *Message `json:"message,omitempty"`

	// Timestamp when the status was updated
	Timestamp time.Time `json:"timestamp"`
}

// Task represents an A2A task
type Task struct {
	// ID uniquely identifies the task
	ID string `json:"id"`

	// SessionID groups related tasks
	SessionID string `json:"sessionId,omitempty"`

	// Status is the current task status
	Status TaskStatus `json:"status"`

	// History contains the conversation history
	History []Message `json:"history,omitempty"`

	// Artifacts produced by the task
	Artifacts []Artifact `json:"artifacts,omitempty"`

	// Metadata for additional context
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewTask creates a new task with the given ID
func NewTask(id string) *Task {
	return &Task{
		ID: id,
		Status: TaskStatus{
			State:     TaskStateSubmitted,
			Timestamp: time.Now(),
		},
		History:   make([]Message, 0),
		Artifacts: make([]Artifact, 0),
		Metadata:  make(map[string]any),
	}
}

// UpdateState updates the task state
func (t *Task) UpdateState(state TaskState, message *Message) {
	t.Status = TaskStatus{
		State:     state,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// AddMessage adds a message to the task history
func (t *Task) AddMessage(msg Message) {
	t.History = append(t.History, msg)
}

// AddArtifact adds an artifact to the task
func (t *Task) AddArtifact(artifact Artifact) {
	t.Artifacts = append(t.Artifacts, artifact)
}

// PushNotificationConfig configures push notifications for async tasks
type PushNotificationConfig struct {
	// URL to send notifications to
	URL string `json:"url"`

	// Token for authentication
	Token string `json:"token,omitempty"`

	// Authentication configuration
	Authentication *AuthenticationInfo `json:"authentication,omitempty"`
}

// AuthenticationInfo contains authentication details
type AuthenticationInfo struct {
	// Schemes supported (e.g., "Bearer", "Basic")
	Schemes []string `json:"schemes,omitempty"`

	// Credentials for authentication
	Credentials string `json:"credentials,omitempty"`
}

// TaskSendParams are parameters for sending a task
type TaskSendParams struct {
	// ID of the task (optional, will be generated if not provided)
	ID string `json:"id,omitempty"`

	// SessionID to group related tasks
	SessionID string `json:"sessionId,omitempty"`

	// Message to send
	Message Message `json:"message"`

	// HistoryLength limits conversation history (0 = unlimited)
	HistoryLength int `json:"historyLength,omitempty"`

	// PushNotification configures async notifications
	PushNotification *PushNotificationConfig `json:"pushNotification,omitempty"`

	// Metadata for additional context
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TaskQueryParams are parameters for querying task status
type TaskQueryParams struct {
	// ID of the task to query
	ID string `json:"id"`

	// HistoryLength limits returned history (0 = unlimited)
	HistoryLength int `json:"historyLength,omitempty"`
}

// TaskCancelParams are parameters for canceling a task
type TaskCancelParams struct {
	// ID of the task to cancel
	ID string `json:"id"`
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Standard JSON-RPC error codes
const (
	ErrorCodeParseError     = -32700
	ErrorCodeInvalidRequest = -32600
	ErrorCodeMethodNotFound = -32601
	ErrorCodeInvalidParams  = -32602
	ErrorCodeInternalError  = -32603

	// A2A-specific error codes
	ErrorCodeTaskNotFound      = -32001
	ErrorCodeTaskCanceled      = -32002
	ErrorCodePushNotSupported  = -32003
	ErrorCodeUnsupportedMethod = -32004
)

// NewJSONRPCRequest creates a new JSON-RPC request
func NewJSONRPCRequest(id any, method string, params any) (*JSONRPCRequest, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsJSON,
	}, nil
}

// NewJSONRPCResponse creates a successful JSON-RPC response
func NewJSONRPCResponse(id any, result any) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// NewJSONRPCError creates an error JSON-RPC response
func NewJSONRPCError(id any, code int, message string, data any) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

// A2A Methods
const (
	MethodTaskSend         = "tasks/send"
	MethodTaskSendSubscribe = "tasks/sendSubscribe"
	MethodTaskGet          = "tasks/get"
	MethodTaskCancel       = "tasks/cancel"
	MethodTaskResubscribe  = "tasks/resubscribe"
)

// SSE Event types for streaming
type SSEEventType string

const (
	SSEEventTaskStatus   SSEEventType = "task-status"
	SSEEventTaskArtifact SSEEventType = "task-artifact"
	SSEEventTaskMessage  SSEEventType = "task-message"
	SSEEventError        SSEEventType = "error"
	SSEEventDone         SSEEventType = "done"
)

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	// Event type
	Event SSEEventType `json:"event"`

	// Data payload
	Data any `json:"data"`
}

// TaskStatusUpdate is sent via SSE when task status changes
type TaskStatusUpdate struct {
	// TaskID being updated
	TaskID string `json:"taskId"`

	// Status update
	Status TaskStatus `json:"status"`

	// Final indicates this is the last update
	Final bool `json:"final,omitempty"`
}

// TaskArtifactUpdate is sent via SSE when artifacts are produced
type TaskArtifactUpdate struct {
	// TaskID being updated
	TaskID string `json:"taskId"`

	// Artifact produced
	Artifact Artifact `json:"artifact"`
}
