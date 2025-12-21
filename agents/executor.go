package agents

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// DefaultAgentExecutor executes agents with tools
type DefaultAgentExecutor struct {
	agent         Agent
	tools         map[string]Tool
	maxIterations int
	callbacks     []AgentCallback
	verbose       bool
	returnIntermediateSteps bool
}

// AgentExecutorConfig configures the agent executor
type AgentExecutorConfig struct {
	// Agent is the agent to execute (required)
	Agent Agent

	// Tools are the available tools (required)
	Tools []Tool

	// MaxIterations is the maximum iterations (default: 15)
	MaxIterations int

	// Callbacks are optional callbacks for events
	Callbacks []AgentCallback

	// Verbose enables verbose output
	Verbose bool

	// ReturnIntermediateSteps includes steps in output
	ReturnIntermediateSteps bool
}

// NewAgentExecutor creates a new agent executor
func NewAgentExecutor(cfg AgentExecutorConfig) (*DefaultAgentExecutor, error) {
	if cfg.Agent == nil {
		return nil, fmt.Errorf("agent is required")
	}

	maxIter := cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = 15
	}

	toolMap := make(map[string]Tool)
	for _, tool := range cfg.Tools {
		toolMap[strings.ToLower(tool.Name())] = tool
	}

	return &DefaultAgentExecutor{
		agent:                   cfg.Agent,
		tools:                   toolMap,
		maxIterations:           maxIter,
		callbacks:               cfg.Callbacks,
		verbose:                 cfg.Verbose,
		returnIntermediateSteps: cfg.ReturnIntermediateSteps,
	}, nil
}

// Run executes the agent until completion
func (e *DefaultAgentExecutor) Run(ctx context.Context, input string) (string, error) {
	return e.RunWithHistory(ctx, input, "")
}

// RunWithHistory executes with conversation history
func (e *DefaultAgentExecutor) RunWithHistory(ctx context.Context, input string, history string) (string, error) {
	var steps []AgentStep

	for i := 0; i < e.maxIterations; i++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// Plan next action
		agentInput := AgentInput{
			Input:             input,
			IntermediateSteps: steps,
			ChatHistory:       history,
		}

		action, err := e.agent.Plan(ctx, agentInput)
		if err != nil {
			return "", fmt.Errorf("agent planning error: %w", err)
		}

		// Notify callbacks
		e.notifyAction(ctx, action)

		// Check if agent is finished
		if action.Finish {
			e.notifyFinish(ctx, action.FinalAnswer)
			return action.FinalAnswer, nil
		}

		// Execute tool
		observation, err := e.executeTool(ctx, action.Tool, action.ToolInput)
		if err != nil {
			observation = fmt.Sprintf("Error: %s", err.Error())
		}

		// Record step
		steps = append(steps, AgentStep{
			Action:      action,
			Observation: observation,
		})

		if e.verbose {
			fmt.Printf("Thought: %s\n", action.Log)
			fmt.Printf("Action: %s\n", action.Tool)
			fmt.Printf("Action Input: %s\n", action.ToolInput)
			fmt.Printf("Observation: %s\n\n", observation)
		}
	}

	return "", fmt.Errorf("agent exceeded maximum iterations (%d)", e.maxIterations)
}

// Stream executes and streams intermediate steps
func (e *DefaultAgentExecutor) Stream(ctx context.Context, input string) (<-chan AgentStreamEvent, error) {
	ch := make(chan AgentStreamEvent)

	go func() {
		defer close(ch)

		var steps []AgentStep

		for i := 0; i < e.maxIterations; i++ {
			select {
			case <-ctx.Done():
				ch <- AgentStreamEvent{
					Type:  AgentEventError,
					Error: ctx.Err(),
				}
				return
			default:
			}

			// Plan next action
			agentInput := AgentInput{
				Input:             input,
				IntermediateSteps: steps,
			}

			action, err := e.agent.Plan(ctx, agentInput)
			if err != nil {
				ch <- AgentStreamEvent{
					Type:  AgentEventError,
					Error: err,
				}
				return
			}

			// Send thought event
			ch <- AgentStreamEvent{
				Type: AgentEventThought,
				Step: &AgentStep{Action: action},
			}

			// Check if finished
			if action.Finish {
				ch <- AgentStreamEvent{
					Type:        AgentEventFinish,
					FinalAnswer: action.FinalAnswer,
				}
				return
			}

			// Send action event
			ch <- AgentStreamEvent{
				Type: AgentEventAction,
				Step: &AgentStep{Action: action},
			}

			// Execute tool
			observation, err := e.executeTool(ctx, action.Tool, action.ToolInput)
			if err != nil {
				observation = fmt.Sprintf("Error: %s", err.Error())
			}

			step := AgentStep{
				Action:      action,
				Observation: observation,
			}
			steps = append(steps, step)

			// Send observation event
			ch <- AgentStreamEvent{
				Type: AgentEventObservation,
				Step: &step,
			}
		}

		ch <- AgentStreamEvent{
			Type:  AgentEventError,
			Error: fmt.Errorf("exceeded maximum iterations"),
		}
	}()

	return ch, nil
}

// executeTool runs a tool and returns the observation
func (e *DefaultAgentExecutor) executeTool(ctx context.Context, toolName, toolInput string) (string, error) {
	// Find tool (case-insensitive)
	tool, ok := e.tools[strings.ToLower(toolName)]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	// Notify callbacks
	e.notifyToolStart(ctx, toolName, toolInput)

	// Execute tool
	startTime := time.Now()
	result, err := tool.Call(ctx, toolInput)
	duration := time.Since(startTime)

	if err != nil {
		e.notifyToolError(ctx, toolName, err)
		return "", err
	}

	e.notifyToolEnd(ctx, toolName, result)

	if e.verbose {
		fmt.Printf("[Tool %s took %v]\n", toolName, duration)
	}

	return result, nil
}

// Callback notification helpers

func (e *DefaultAgentExecutor) notifyAction(ctx context.Context, action AgentAction) {
	for _, cb := range e.callbacks {
		cb.OnAgentAction(ctx, action)
	}
}

func (e *DefaultAgentExecutor) notifyFinish(ctx context.Context, output string) {
	for _, cb := range e.callbacks {
		cb.OnAgentFinish(ctx, output)
	}
}

func (e *DefaultAgentExecutor) notifyToolStart(ctx context.Context, tool, input string) {
	for _, cb := range e.callbacks {
		cb.OnToolStart(ctx, tool, input)
	}
}

func (e *DefaultAgentExecutor) notifyToolEnd(ctx context.Context, tool, output string) {
	for _, cb := range e.callbacks {
		cb.OnToolEnd(ctx, tool, output)
	}
}

func (e *DefaultAgentExecutor) notifyToolError(ctx context.Context, tool string, err error) {
	for _, cb := range e.callbacks {
		cb.OnToolError(ctx, tool, err)
	}
}
