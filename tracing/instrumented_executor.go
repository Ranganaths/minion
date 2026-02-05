package tracing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Ranganaths/minion/agents"
	"github.com/Ranganaths/minion/llm"
)

// InstrumentedAgentExecutor is a fully instrumented agent executor that captures
// all execution details including LLM calls, tool invocations, and agent decisions.
// Unlike TracedAgentExecutor which wraps an existing executor, this executor
// re-implements the execution loop with full instrumentation.
type InstrumentedAgentExecutor struct {
	agent         agents.Agent
	tools         map[string]agents.Tool
	llmProvider   llm.Provider
	maxIterations int
	collector     *TraceCollector
	agentID       string
	agentName     string
	verbose       bool
}

// InstrumentedExecutorConfig configures the instrumented executor
type InstrumentedExecutorConfig struct {
	// Agent is the agent to execute (required)
	Agent agents.Agent

	// Tools are the available tools (required)
	Tools []agents.Tool

	// LLMProvider is the LLM provider to use (optional - uses agent's provider if not set)
	LLMProvider llm.Provider

	// MaxIterations is the maximum iterations (default: 15)
	MaxIterations int

	// Store is the trace storage backend (required)
	Store TraceStore

	// CollectorConfig configures the trace collector
	CollectorConfig CollectorConfig

	// AgentID identifies the agent
	AgentID string

	// AgentName is the human-readable agent name
	AgentName string

	// Hooks are real-time notification hooks
	Hooks []TraceHook

	// Verbose enables verbose output
	Verbose bool
}

// NewInstrumentedAgentExecutor creates a new fully instrumented agent executor
func NewInstrumentedAgentExecutor(cfg InstrumentedExecutorConfig) (*InstrumentedAgentExecutor, error) {
	if cfg.Agent == nil {
		return nil, fmt.Errorf("agent is required")
	}
	if cfg.Store == nil {
		return nil, fmt.Errorf("store is required")
	}

	maxIter := cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = 15
	}

	toolMap := make(map[string]agents.Tool)
	for _, tool := range cfg.Tools {
		toolMap[strings.ToLower(tool.Name())] = tool
	}

	collector := NewTraceCollector(cfg.Store, cfg.CollectorConfig)
	for _, hook := range cfg.Hooks {
		collector.AddHook(hook)
	}

	return &InstrumentedAgentExecutor{
		agent:         cfg.Agent,
		tools:         toolMap,
		llmProvider:   cfg.LLMProvider,
		maxIterations: maxIter,
		collector:     collector,
		agentID:       cfg.AgentID,
		agentName:     cfg.AgentName,
		verbose:       cfg.Verbose,
	}, nil
}

// Run executes the agent until completion with full tracing
func (e *InstrumentedAgentExecutor) Run(ctx context.Context, input string) (string, error) {
	return e.RunWithHistory(ctx, input, "")
}

// RunWithHistory executes with conversation history and full tracing
func (e *InstrumentedAgentExecutor) RunWithHistory(ctx context.Context, input string, history string) (string, error) {
	// Start trace
	ctx, _ = e.collector.StartTrace(ctx, e.agentID, e.agentName, input)

	var steps []agents.AgentStep
	var finalOutput string
	var execErr error

	for i := 0; i < e.maxIterations; i++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			execErr = ctx.Err()
			break
		default:
		}

		if execErr != nil {
			break
		}

		// Start iteration span
		ctx, iterSpanID := e.collector.StartIteration(ctx)

		// Plan next action
		agentInput := agents.AgentInput{
			Input:             input,
			IntermediateSteps: steps,
			ChatHistory:       history,
		}

		action, err := e.agent.Plan(ctx, agentInput)
		if err != nil {
			e.collector.EndIteration(ctx, iterSpanID, err)
			execErr = fmt.Errorf("agent planning error: %w", err)
			break
		}

		// Record the decision
		if action.Finish {
			e.collector.RecordDecision(ctx, "", action.FinalAnswer, action.Log, i+1, true)
			e.collector.EndIteration(ctx, iterSpanID, nil)
			finalOutput = action.FinalAnswer
			break
		} else {
			e.collector.RecordDecision(ctx, action.Tool, action.ToolInput, action.Log, i+1, false)
		}

		// Execute tool with tracing
		observation, err := e.executeTool(ctx, action.Tool, action.ToolInput)
		if err != nil {
			observation = fmt.Sprintf("Error: %s", err.Error())
		}

		// Record step
		steps = append(steps, agents.AgentStep{
			Action:      action,
			Observation: observation,
		})

		e.collector.EndIteration(ctx, iterSpanID, nil)

		if e.verbose {
			fmt.Printf("Thought: %s\n", action.Log)
			fmt.Printf("Action: %s\n", action.Tool)
			fmt.Printf("Action Input: %s\n", action.ToolInput)
			fmt.Printf("Observation: %s\n\n", observation)
		}
	}

	if execErr == nil && finalOutput == "" {
		execErr = fmt.Errorf("agent exceeded maximum iterations (%d)", e.maxIterations)
	}

	// End trace
	if traceErr := e.collector.EndTrace(ctx, finalOutput, execErr); traceErr != nil {
		fmt.Printf("Warning: failed to save trace: %v\n", traceErr)
	}

	return finalOutput, execErr
}

// Stream executes and streams intermediate steps with full tracing
func (e *InstrumentedAgentExecutor) Stream(ctx context.Context, input string) (<-chan agents.AgentStreamEvent, error) {
	ch := make(chan agents.AgentStreamEvent)

	go func() {
		defer close(ch)

		// Start trace
		ctx, _ = e.collector.StartTrace(ctx, e.agentID, e.agentName, input)

		var steps []agents.AgentStep
		var finalOutput string
		var execErr error

		for i := 0; i < e.maxIterations; i++ {
			select {
			case <-ctx.Done():
				ch <- agents.AgentStreamEvent{
					Type:  agents.AgentEventError,
					Error: ctx.Err(),
				}
				e.collector.EndTrace(ctx, "", ctx.Err())
				return
			default:
			}

			// Start iteration span
			ctx, iterSpanID := e.collector.StartIteration(ctx)

			// Plan next action
			agentInput := agents.AgentInput{
				Input:             input,
				IntermediateSteps: steps,
			}

			action, err := e.agent.Plan(ctx, agentInput)
			if err != nil {
				e.collector.EndIteration(ctx, iterSpanID, err)
				ch <- agents.AgentStreamEvent{
					Type:  agents.AgentEventError,
					Error: err,
				}
				e.collector.EndTrace(ctx, "", err)
				return
			}

			// Send thought event
			ch <- agents.AgentStreamEvent{
				Type: agents.AgentEventThought,
				Step: &agents.AgentStep{Action: action},
			}

			// Check if finished
			if action.Finish {
				e.collector.RecordDecision(ctx, "", action.FinalAnswer, action.Log, i+1, true)
				e.collector.EndIteration(ctx, iterSpanID, nil)
				finalOutput = action.FinalAnswer
				ch <- agents.AgentStreamEvent{
					Type:        agents.AgentEventFinish,
					FinalAnswer: action.FinalAnswer,
				}
				break
			}

			// Record decision
			e.collector.RecordDecision(ctx, action.Tool, action.ToolInput, action.Log, i+1, false)

			// Send action event
			ch <- agents.AgentStreamEvent{
				Type: agents.AgentEventAction,
				Step: &agents.AgentStep{Action: action},
			}

			// Execute tool with tracing
			observation, err := e.executeTool(ctx, action.Tool, action.ToolInput)
			if err != nil {
				observation = fmt.Sprintf("Error: %s", err.Error())
			}

			step := agents.AgentStep{
				Action:      action,
				Observation: observation,
			}
			steps = append(steps, step)

			// Send observation event
			ch <- agents.AgentStreamEvent{
				Type: agents.AgentEventObservation,
				Step: &step,
			}

			e.collector.EndIteration(ctx, iterSpanID, nil)
		}

		if finalOutput == "" && execErr == nil {
			execErr = fmt.Errorf("exceeded maximum iterations")
			ch <- agents.AgentStreamEvent{
				Type:  agents.AgentEventError,
				Error: execErr,
			}
		}

		// End trace
		if traceErr := e.collector.EndTrace(ctx, finalOutput, execErr); traceErr != nil {
			fmt.Printf("Warning: failed to save trace: %v\n", traceErr)
		}
	}()

	return ch, nil
}

// executeTool runs a tool with tracing
func (e *InstrumentedAgentExecutor) executeTool(ctx context.Context, toolName, toolInput string) (string, error) {
	// Find tool (case-insensitive)
	tool, ok := e.tools[strings.ToLower(toolName)]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	// Start tool span
	ctx, spanID := e.collector.StartToolSpan(ctx, toolName, toolInput)

	// Execute tool
	startTime := time.Now()
	result, err := tool.Call(ctx, toolInput)
	duration := time.Since(startTime)

	// End tool span
	e.collector.EndToolSpan(ctx, spanID, result, err)

	if e.verbose {
		fmt.Printf("[Tool %s took %v]\n", toolName, duration)
	}

	return result, err
}

// GetCollector returns the trace collector
func (e *InstrumentedAgentExecutor) GetCollector() *TraceCollector {
	return e.collector
}

// GetLastTraceID returns the ID of the last completed trace
func (e *InstrumentedAgentExecutor) GetLastTraceID() TraceID {
	return e.collector.GetTraceID()
}

// SetSessionID sets the session ID for subsequent traces
func (e *InstrumentedAgentExecutor) SetSessionID(sessionID string) {
	e.collector.SetSessionID(sessionID)
}

// AddHook adds a real-time trace hook
func (e *InstrumentedAgentExecutor) AddHook(hook TraceHook) {
	e.collector.AddHook(hook)
}

// Ensure InstrumentedAgentExecutor implements AgentExecutor
var _ agents.AgentExecutor = (*InstrumentedAgentExecutor)(nil)

// TracedTool wraps a tool with tracing
type TracedTool struct {
	tool      agents.Tool
	collector *TraceCollector
}

// NewTracedTool creates a new traced tool wrapper
func NewTracedTool(tool agents.Tool, collector *TraceCollector) *TracedTool {
	return &TracedTool{
		tool:      tool,
		collector: collector,
	}
}

// Name returns the tool name
func (t *TracedTool) Name() string {
	return t.tool.Name()
}

// Description returns the tool description
func (t *TracedTool) Description() string {
	return t.tool.Description()
}

// Call executes the tool with tracing
func (t *TracedTool) Call(ctx context.Context, input string) (string, error) {
	// Start tool span
	ctx, spanID := t.collector.StartToolSpan(ctx, t.tool.Name(), input)

	// Execute tool
	result, err := t.tool.Call(ctx, input)

	// End tool span
	t.collector.EndToolSpan(ctx, spanID, result, err)

	return result, err
}

// Ensure TracedTool implements Tool
var _ agents.Tool = (*TracedTool)(nil)

// WrapToolsWithTracing wraps a slice of tools with tracing
func WrapToolsWithTracing(tools []agents.Tool, collector *TraceCollector) []agents.Tool {
	wrapped := make([]agents.Tool, len(tools))
	for i, tool := range tools {
		wrapped[i] = NewTracedTool(tool, collector)
	}
	return wrapped
}
