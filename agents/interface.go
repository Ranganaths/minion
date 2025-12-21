// Package agents provides autonomous agents that can use tools to accomplish tasks.
// Agents use LLMs to decide which actions to take and which tools to use.
package agents

import (
	"context"
)

// Agent is the interface for autonomous agents
type Agent interface {
	// Plan decides what action to take given the input and previous steps
	Plan(ctx context.Context, input AgentInput) (AgentAction, error)

	// InputKeys returns the expected input keys
	InputKeys() []string

	// OutputKeys returns the output keys
	OutputKeys() []string
}

// AgentInput contains the input for agent planning
type AgentInput struct {
	// Input is the user's original input/question
	Input string

	// IntermediateSteps contains previous action/observation pairs
	IntermediateSteps []AgentStep

	// ChatHistory is optional conversation history
	ChatHistory string
}

// AgentStep represents a single step in agent execution
type AgentStep struct {
	// Action is the action that was taken
	Action AgentAction

	// Observation is the result of the action
	Observation string
}

// AgentAction represents an action decided by the agent
type AgentAction struct {
	// Tool is the name of the tool to use (empty if finishing)
	Tool string

	// ToolInput is the input to pass to the tool
	ToolInput string

	// Log is the agent's reasoning/thought process
	Log string

	// Finish indicates this is the final answer
	Finish bool

	// FinalAnswer is the final answer (when Finish is true)
	FinalAnswer string
}

// Tool is the interface for tools that agents can use
type Tool interface {
	// Name returns the tool name (used by agent to invoke it)
	Name() string

	// Description returns a description of what the tool does
	Description() string

	// Call executes the tool with the given input
	Call(ctx context.Context, input string) (string, error)
}

// ToolConfig provides additional tool configuration
type ToolConfig struct {
	// ReturnDirect if true, returns tool output directly as final answer
	ReturnDirect bool

	// Verbose enables verbose logging
	Verbose bool
}

// AgentExecutor runs an agent with tools until completion
type AgentExecutor interface {
	// Run executes the agent loop until a final answer is reached
	Run(ctx context.Context, input string) (string, error)

	// RunWithHistory executes with conversation history
	RunWithHistory(ctx context.Context, input string, history string) (string, error)

	// Stream executes and streams intermediate steps
	Stream(ctx context.Context, input string) (<-chan AgentStreamEvent, error)
}

// AgentStreamEvent represents a streaming event during agent execution
type AgentStreamEvent struct {
	// Type indicates the event type
	Type AgentStreamEventType

	// Step contains the current step (for step events)
	Step *AgentStep

	// FinalAnswer contains the final answer (for finish events)
	FinalAnswer string

	// Error contains any error (for error events)
	Error error
}

// AgentStreamEventType indicates the type of stream event
type AgentStreamEventType string

const (
	// AgentEventThought indicates agent is thinking
	AgentEventThought AgentStreamEventType = "thought"

	// AgentEventAction indicates agent decided on an action
	AgentEventAction AgentStreamEventType = "action"

	// AgentEventObservation indicates tool returned observation
	AgentEventObservation AgentStreamEventType = "observation"

	// AgentEventFinish indicates agent finished with final answer
	AgentEventFinish AgentStreamEventType = "finish"

	// AgentEventError indicates an error occurred
	AgentEventError AgentStreamEventType = "error"
)

// AgentCallback provides hooks into agent execution
type AgentCallback interface {
	// OnAgentAction is called when agent decides on an action
	OnAgentAction(ctx context.Context, action AgentAction)

	// OnAgentFinish is called when agent finishes
	OnAgentFinish(ctx context.Context, output string)

	// OnToolStart is called before tool execution
	OnToolStart(ctx context.Context, tool string, input string)

	// OnToolEnd is called after tool execution
	OnToolEnd(ctx context.Context, tool string, output string)

	// OnToolError is called when tool errors
	OnToolError(ctx context.Context, tool string, err error)
}

// NoopAgentCallback is a no-op implementation of AgentCallback
type NoopAgentCallback struct{}

func (NoopAgentCallback) OnAgentAction(ctx context.Context, action AgentAction)   {}
func (NoopAgentCallback) OnAgentFinish(ctx context.Context, output string)        {}
func (NoopAgentCallback) OnToolStart(ctx context.Context, tool string, input string) {}
func (NoopAgentCallback) OnToolEnd(ctx context.Context, tool string, output string)  {}
func (NoopAgentCallback) OnToolError(ctx context.Context, tool string, err error)    {}
