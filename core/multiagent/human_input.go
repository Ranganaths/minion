package multiagent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
)

// HumanInputHandler manages human-in-the-loop interactions
type HumanInputHandler struct {
	mu sync.Mutex

	// Input mode
	mode HumanInputMode

	// Input function
	inputFunc HumanInputFunc

	// Approval required actions
	requireApprovalFor []string

	// Timeout for human input
	timeoutSeconds int

	// Default action when timeout
	defaultOnTimeout string
}

// HumanInputHandlerConfig configures the human input handler
type HumanInputHandlerConfig struct {
	// Mode is the human input mode
	Mode HumanInputMode

	// InputFunc is the function to get human input (defaults to console)
	InputFunc HumanInputFunc

	// RequireApprovalFor lists actions that require human approval
	RequireApprovalFor []string

	// TimeoutSeconds is the timeout for input (0 = no timeout)
	TimeoutSeconds int

	// DefaultOnTimeout is the default response on timeout
	DefaultOnTimeout string
}

// NewHumanInputHandler creates a new human input handler
func NewHumanInputHandler(cfg HumanInputHandlerConfig) *HumanInputHandler {
	inputFunc := cfg.InputFunc
	if inputFunc == nil {
		inputFunc = ConsoleInputFunc
	}

	return &HumanInputHandler{
		mode:               cfg.Mode,
		inputFunc:          inputFunc,
		requireApprovalFor: cfg.RequireApprovalFor,
		timeoutSeconds:     cfg.TimeoutSeconds,
		defaultOnTimeout:   cfg.DefaultOnTimeout,
	}
}

// ConsoleInputFunc reads input from console stdin
func ConsoleInputFunc(ctx context.Context, prompt string) (string, error) {
	fmt.Print(prompt + " ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(input), nil
}

// GetInput gets human input based on the mode and context
func (h *HumanInputHandler) GetInput(ctx context.Context, prompt string, action string) (HumanInputResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if we should request input based on mode
	switch h.mode {
	case HumanInputNever:
		return HumanInputResult{
			Action:  HumanActionContinue,
			Skipped: true,
		}, nil

	case HumanInputTerminate:
		// Only request input for termination decisions
		if !strings.Contains(strings.ToUpper(action), "TERMINATE") {
			return HumanInputResult{
				Action:  HumanActionContinue,
				Skipped: true,
			}, nil
		}

	case HumanInputAlways:
		// Always request input
	}

	// Check if this action requires approval
	if len(h.requireApprovalFor) > 0 {
		requiresApproval := false
		for _, approvalAction := range h.requireApprovalFor {
			if strings.Contains(strings.ToLower(action), strings.ToLower(approvalAction)) {
				requiresApproval = true
				break
			}
		}
		if !requiresApproval && h.mode != HumanInputAlways {
			return HumanInputResult{
				Action:  HumanActionContinue,
				Skipped: true,
			}, nil
		}
	}

	// Get input from human
	input, err := h.inputFunc(ctx, prompt)
	if err != nil {
		return HumanInputResult{}, err
	}

	return h.parseInput(input), nil
}

// RequestApproval requests approval for an action
func (h *HumanInputHandler) RequestApproval(ctx context.Context, action string, details string) (bool, error) {
	prompt := fmt.Sprintf("Approve action '%s'?\nDetails: %s\n[y/n]:", action, details)

	input, err := h.inputFunc(ctx, prompt)
	if err != nil {
		return false, err
	}

	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes", nil
}

// RequestFeedback requests feedback from the human
func (h *HumanInputHandler) RequestFeedback(ctx context.Context, message string) (string, error) {
	prompt := fmt.Sprintf("%s\nYour feedback:", message)
	return h.inputFunc(ctx, prompt)
}

// parseInput parses human input into a result
func (h *HumanInputHandler) parseInput(input string) HumanInputResult {
	input = strings.TrimSpace(input)
	lowerInput := strings.ToLower(input)

	// Check for special commands
	switch {
	case lowerInput == "" || lowerInput == "skip":
		return HumanInputResult{
			Action:  HumanActionContinue,
			Skipped: true,
		}
	case lowerInput == "exit" || lowerInput == "quit" || lowerInput == "terminate":
		return HumanInputResult{
			Action: HumanActionTerminate,
		}
	case lowerInput == "approve" || lowerInput == "y" || lowerInput == "yes":
		return HumanInputResult{
			Action: HumanActionApprove,
		}
	case lowerInput == "reject" || lowerInput == "n" || lowerInput == "no":
		return HumanInputResult{
			Action: HumanActionReject,
		}
	default:
		return HumanInputResult{
			Action:   HumanActionIntercept,
			Response: input,
		}
	}
}

// HumanInputResult represents the result of human input
type HumanInputResult struct {
	// Action is the action to take
	Action HumanAction

	// Response is the human's response (for intercept action)
	Response string

	// Skipped indicates the input was skipped
	Skipped bool
}

// HumanAction represents actions from human input
type HumanAction string

const (
	// HumanActionContinue - continue with auto-reply
	HumanActionContinue HumanAction = "continue"

	// HumanActionIntercept - use human's response instead
	HumanActionIntercept HumanAction = "intercept"

	// HumanActionTerminate - terminate the conversation
	HumanActionTerminate HumanAction = "terminate"

	// HumanActionApprove - approve the action
	HumanActionApprove HumanAction = "approve"

	// HumanActionReject - reject the action
	HumanActionReject HumanAction = "reject"
)

// HumanApprovalRequired is a decorator that adds human approval to task handlers
type HumanApprovalRequired struct {
	handler      TaskHandler
	inputHandler *HumanInputHandler
	actions      []string // Actions requiring approval
}

// NewHumanApprovalRequired wraps a task handler with human approval
func NewHumanApprovalRequired(handler TaskHandler, inputHandler *HumanInputHandler, actions []string) *HumanApprovalRequired {
	return &HumanApprovalRequired{
		handler:      handler,
		inputHandler: inputHandler,
		actions:      actions,
	}
}

// HandleTask handles a task with human approval
func (h *HumanApprovalRequired) HandleTask(ctx context.Context, task *Task) (interface{}, error) {
	// Check if this task type requires approval
	requiresApproval := false
	for _, action := range h.actions {
		if strings.EqualFold(task.Type, action) || strings.Contains(task.Name, action) {
			requiresApproval = true
			break
		}
	}

	if requiresApproval {
		details := fmt.Sprintf("Task: %s\nDescription: %s\nInput: %v", task.Name, task.Description, task.Input)
		approved, err := h.inputHandler.RequestApproval(ctx, task.Type, details)
		if err != nil {
			return nil, fmt.Errorf("approval request failed: %w", err)
		}
		if !approved {
			return map[string]interface{}{
				"status":  "rejected",
				"message": "Task was rejected by human",
			}, nil
		}
	}

	return h.handler.HandleTask(ctx, task)
}

// GetCapabilities returns the wrapped handler's capabilities
func (h *HumanApprovalRequired) GetCapabilities() []string {
	return h.handler.GetCapabilities()
}

// GetName returns the wrapped handler's name
func (h *HumanApprovalRequired) GetName() string {
	return h.handler.GetName() + "_with_approval"
}

// InteractiveSession manages an interactive human-agent session
type InteractiveSession struct {
	agent        *ConversableAgent
	inputHandler *HumanInputHandler
	maxTurns     int
}

// NewInteractiveSession creates a new interactive session
func NewInteractiveSession(agent *ConversableAgent, inputHandler *HumanInputHandler, maxTurns int) *InteractiveSession {
	if maxTurns <= 0 {
		maxTurns = 100
	}
	return &InteractiveSession{
		agent:        agent,
		inputHandler: inputHandler,
		maxTurns:     maxTurns,
	}
}

// Start starts the interactive session
func (s *InteractiveSession) Start(ctx context.Context) error {
	fmt.Printf("Starting interactive session with %s\n", s.agent.Name())
	fmt.Println("Type 'exit' or 'quit' to end the session.")
	fmt.Println()

	for turn := 0; turn < s.maxTurns; turn++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Get human input
		result, err := s.inputHandler.GetInput(ctx, "You:", "user_input")
		if err != nil {
			return err
		}

		if result.Action == HumanActionTerminate {
			fmt.Println("Session ended.")
			return nil
		}

		if result.Skipped || result.Response == "" {
			continue
		}

		// Send to agent and get reply
		s.agent.Receive(ctx, result.Response, nil)
		reply, err := s.agent.GenerateReply(ctx, nil)
		if err != nil {
			return err
		}

		fmt.Printf("%s: %s\n\n", s.agent.Name(), reply)
	}

	return nil
}
