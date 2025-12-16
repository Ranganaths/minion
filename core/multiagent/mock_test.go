package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// MockLLMProvider is a mock LLM provider for testing
type MockLLMProvider struct {
	// Responses to return for different prompts (keyed by user prompt substring)
	ResponseMap map[string]string

	// Track calls
	CallCount        int
	LastSystemPrompt string
	LastUserPrompt   string
}

// NewMockLLMProvider creates a new mock LLM provider
func NewMockLLMProvider() *MockLLMProvider {
	return &MockLLMProvider{
		ResponseMap: make(map[string]string),
	}
}

// GenerateCompletion generates a mock completion
func (m *MockLLMProvider) GenerateCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	m.CallCount++
	m.LastSystemPrompt = req.SystemPrompt
	m.LastUserPrompt = req.UserPrompt

	// Check if we have a specific response for this prompt
	for key, response := range m.ResponseMap {
		if strings.Contains(req.UserPrompt, key) {
			return &CompletionResponse{
				Text:         response,
				TokensUsed:   len(response) / 4, // Rough estimate
				Model:        "mock-model",
				FinishReason: "stop",
			}, nil
		}
	}

	// Default: generate a simple task decomposition
	return m.generateDefaultResponse(req)
}

// generateDefaultResponse generates a default task decomposition response
func (m *MockLLMProvider) generateDefaultResponse(req *CompletionRequest) (*CompletionResponse, error) {
	// Parse task name/description from user prompt
	lines := strings.Split(req.UserPrompt, "\n")
	taskName := "Unknown Task"
	for _, line := range lines {
		if strings.HasPrefix(line, "Task:") {
			taskName = strings.TrimSpace(strings.TrimPrefix(line, "Task:"))
			break
		}
	}

	// Generate a simple decomposition
	response := SubtaskResponse{
		Subtasks: []SubtaskSpec{
			{
				Name:         fmt.Sprintf("Analyze %s", taskName),
				Description:  "Analyze the requirements and gather necessary information",
				AssignedTo:   "data_analysis",
				Dependencies: []string{},
				Priority:     7,
				Input:        "Analyze requirements",
			},
			{
				Name:         fmt.Sprintf("Execute %s", taskName),
				Description:  "Execute the main task based on analysis",
				AssignedTo:   "code_generation",
				Dependencies: []string{fmt.Sprintf("Analyze %s", taskName)},
				Priority:     8,
				Input:        "Execute based on analysis",
			},
			{
				Name:         fmt.Sprintf("Review %s", taskName),
				Description:  "Review the results and ensure quality",
				AssignedTo:   "code_review",
				Dependencies: []string{fmt.Sprintf("Execute %s", taskName)},
				Priority:     6,
				Input:        "Review results",
			},
		},
	}

	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, err
	}

	return &CompletionResponse{
		Text:         string(jsonBytes),
		TokensUsed:   len(jsonBytes) / 4,
		Model:        "mock-model",
		FinishReason: "stop",
	}, nil
}

// SetResponse sets a custom response for a specific prompt substring
func (m *MockLLMProvider) SetResponse(promptSubstring, response string) {
	m.ResponseMap[promptSubstring] = response
}

// SetSimpleTask sets a simple task response
func (m *MockLLMProvider) SetSimpleTask(taskName, capability string) {
	response := SubtaskResponse{
		Subtasks: []SubtaskSpec{
			{
				Name:         taskName,
				Description:  fmt.Sprintf("Execute %s", taskName),
				AssignedTo:   capability,
				Dependencies: []string{},
				Priority:     5,
				Input:        taskName,
			},
		},
	}

	jsonBytes, _ := json.Marshal(response)
	m.ResponseMap[taskName] = string(jsonBytes)
}

// SetCodeGenerationTask sets a code generation task response
func (m *MockLLMProvider) SetCodeGenerationTask() {
	response := `{
  "subtasks": [
    {
      "name": "Design API structure",
      "description": "Design the REST API endpoints and data models",
      "assigned_to": "code_generation",
      "dependencies": [],
      "priority": 8,
      "input": "Design REST API structure"
    },
    {
      "name": "Implement handlers",
      "description": "Implement HTTP handlers for each endpoint",
      "assigned_to": "code_generation",
      "dependencies": ["Design API structure"],
      "priority": 9,
      "input": "Implement HTTP handlers"
    },
    {
      "name": "Write tests",
      "description": "Write unit tests for the handlers",
      "assigned_to": "code_generation",
      "dependencies": ["Implement handlers"],
      "priority": 7,
      "input": "Write unit tests"
    }
  ]
}`
	m.ResponseMap["REST API"] = response
	m.ResponseMap["Generate REST API"] = response
}

// SetDataAnalysisTask sets a data analysis task response
func (m *MockLLMProvider) SetDataAnalysisTask() {
	response := `{
  "subtasks": [
    {
      "name": "Load and clean data",
      "description": "Load the dataset and perform data cleaning",
      "assigned_to": "data_analysis",
      "dependencies": [],
      "priority": 8,
      "input": "Load and clean data"
    },
    {
      "name": "Analyze trends",
      "description": "Perform statistical analysis to identify trends",
      "assigned_to": "statistical_analysis",
      "dependencies": ["Load and clean data"],
      "priority": 9,
      "input": "Analyze statistical trends"
    },
    {
      "name": "Create visualizations",
      "description": "Generate charts and graphs to visualize findings",
      "assigned_to": "visualization",
      "dependencies": ["Analyze trends"],
      "priority": 7,
      "input": "Create visualizations"
    }
  ]
}`
	m.ResponseMap["Analyze Sales Data"] = response
	m.ResponseMap["data analysis"] = response
}

// MockWorkerHandler is a mock task handler for testing workers
type MockWorkerHandler struct {
	HandlerFunc  func(ctx context.Context, task *Task) (interface{}, error)
	capabilities []string
	name         string
	callCount    int
}

// NewMockWorkerHandler creates a new mock worker handler
func NewMockWorkerHandler(name string, capabilities []string) *MockWorkerHandler {
	return &MockWorkerHandler{
		name:         name,
		capabilities: capabilities,
		HandlerFunc: func(ctx context.Context, task *Task) (interface{}, error) {
			// Default: return success with task info
			return map[string]interface{}{
				"status":    "completed",
				"task_name": task.Name,
				"result":    fmt.Sprintf("Mock result for %s", task.Name),
			}, nil
		},
	}
}

// HandleTask processes a task
func (m *MockWorkerHandler) HandleTask(ctx context.Context, task *Task) (interface{}, error) {
	m.callCount++
	if m.HandlerFunc != nil {
		return m.HandlerFunc(ctx, task)
	}
	return nil, fmt.Errorf("no handler function set")
}

// GetCapabilities returns the capabilities
func (m *MockWorkerHandler) GetCapabilities() []string {
	return m.capabilities
}

// GetName returns the handler name
func (m *MockWorkerHandler) GetName() string {
	return m.name
}

// GetCallCount returns the number of times HandleTask was called
func (m *MockWorkerHandler) GetCallCount() int {
	return m.callCount
}

// SetHandlerFunc sets a custom handler function
func (m *MockWorkerHandler) SetHandlerFunc(fn func(ctx context.Context, task *Task) (interface{}, error)) {
	m.HandlerFunc = fn
}
