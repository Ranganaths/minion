package agents

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Ranganaths/minion/llm"
)

// ReActAgent implements the ReAct (Reasoning and Acting) framework.
// It alternates between thinking (reasoning) and acting (using tools).
type ReActAgent struct {
	llm         llm.Provider
	tools       []Tool
	toolsByName map[string]Tool
	maxIter     int
	verbose     bool
}

// ReActAgentConfig configures the ReAct agent
type ReActAgentConfig struct {
	// LLM is the language model provider (required)
	LLM llm.Provider

	// Tools are the tools available to the agent (required)
	Tools []Tool

	// MaxIterations is the maximum number of reasoning steps (default: 10)
	MaxIterations int

	// Verbose enables verbose logging
	Verbose bool
}

// NewReActAgent creates a new ReAct agent
func NewReActAgent(cfg ReActAgentConfig) (*ReActAgent, error) {
	if cfg.LLM == nil {
		return nil, fmt.Errorf("LLM is required")
	}
	if len(cfg.Tools) == 0 {
		return nil, fmt.Errorf("at least one tool is required")
	}

	maxIter := cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = 10
	}

	toolsByName := make(map[string]Tool)
	for _, tool := range cfg.Tools {
		toolsByName[tool.Name()] = tool
	}

	return &ReActAgent{
		llm:         cfg.LLM,
		tools:       cfg.Tools,
		toolsByName: toolsByName,
		maxIter:     maxIter,
		verbose:     cfg.Verbose,
	}, nil
}

// Plan decides what action to take
func (a *ReActAgent) Plan(ctx context.Context, input AgentInput) (AgentAction, error) {
	prompt := a.buildPrompt(input)

	resp, err := a.llm.GenerateCompletion(ctx, &llm.CompletionRequest{
		UserPrompt:  prompt,
		Temperature: 0.0, // Use low temperature for reasoning
		MaxTokens:   1000,
	})
	if err != nil {
		return AgentAction{}, fmt.Errorf("llm error: %w", err)
	}

	return a.parseOutput(resp.Text)
}

// InputKeys returns the expected input keys
func (a *ReActAgent) InputKeys() []string {
	return []string{"input"}
}

// OutputKeys returns the output keys
func (a *ReActAgent) OutputKeys() []string {
	return []string{"output"}
}

// buildPrompt constructs the ReAct prompt
func (a *ReActAgent) buildPrompt(input AgentInput) string {
	toolDescriptions := a.buildToolDescriptions()
	toolNames := a.buildToolNames()

	var scratchpad string
	for _, step := range input.IntermediateSteps {
		scratchpad += step.Action.Log
		scratchpad += fmt.Sprintf("\nObservation: %s\n", step.Observation)
	}

	prompt := fmt.Sprintf(`Answer the following questions as best you can. You have access to the following tools:

%s

Use the following format:

Question: the input question you must answer
Thought: you should always think about what to do
Action: the action to take, should be one of [%s]
Action Input: the input to the action
Observation: the result of the action
... (this Thought/Action/Action Input/Observation can repeat N times)
Thought: I now know the final answer
Final Answer: the final answer to the original input question

Begin!

Question: %s
%s`, toolDescriptions, toolNames, input.Input, scratchpad)

	if scratchpad == "" {
		prompt += "Thought: "
	}

	return prompt
}

// buildToolDescriptions builds the tool descriptions section
func (a *ReActAgent) buildToolDescriptions() string {
	var parts []string
	for _, tool := range a.tools {
		parts = append(parts, fmt.Sprintf("%s: %s", tool.Name(), tool.Description()))
	}
	return strings.Join(parts, "\n")
}

// buildToolNames builds the comma-separated tool names
func (a *ReActAgent) buildToolNames() string {
	var names []string
	for _, tool := range a.tools {
		names = append(names, tool.Name())
	}
	return strings.Join(names, ", ")
}

// parseOutput parses the LLM output to extract the action
func (a *ReActAgent) parseOutput(text string) (AgentAction, error) {
	// Check for final answer
	finalAnswerRegex := regexp.MustCompile(`(?i)Final\s*Answer\s*:\s*(.*)`)
	if matches := finalAnswerRegex.FindStringSubmatch(text); len(matches) > 1 {
		return AgentAction{
			Finish:      true,
			FinalAnswer: strings.TrimSpace(matches[1]),
			Log:         text,
		}, nil
	}

	// Parse action and action input
	actionRegex := regexp.MustCompile(`(?i)Action\s*:\s*(.+?)(?:\n|$)`)
	actionInputRegex := regexp.MustCompile(`(?i)Action\s*Input\s*:\s*(.+?)(?:\n|$)`)

	actionMatch := actionRegex.FindStringSubmatch(text)
	if len(actionMatch) < 2 {
		// If no action found, treat as final answer
		return AgentAction{
			Finish:      true,
			FinalAnswer: strings.TrimSpace(text),
			Log:         text,
		}, nil
	}

	action := strings.TrimSpace(actionMatch[1])
	var actionInput string

	inputMatch := actionInputRegex.FindStringSubmatch(text)
	if len(inputMatch) > 1 {
		actionInput = strings.TrimSpace(inputMatch[1])
	}

	return AgentAction{
		Tool:      action,
		ToolInput: actionInput,
		Log:       text,
		Finish:    false,
	}, nil
}

// GetTool returns a tool by name
func (a *ReActAgent) GetTool(name string) (Tool, bool) {
	// Case-insensitive lookup
	for toolName, tool := range a.toolsByName {
		if strings.EqualFold(toolName, name) {
			return tool, true
		}
	}
	return nil, false
}

// ConversationalReActAgent extends ReActAgent with conversation history support
type ConversationalReActAgent struct {
	*ReActAgent
	memoryKey string
}

// ConversationalReActAgentConfig configures the conversational ReAct agent
type ConversationalReActAgentConfig struct {
	ReActAgentConfig
	MemoryKey string
}

// NewConversationalReActAgent creates a new conversational ReAct agent
func NewConversationalReActAgent(cfg ConversationalReActAgentConfig) (*ConversationalReActAgent, error) {
	base, err := NewReActAgent(cfg.ReActAgentConfig)
	if err != nil {
		return nil, err
	}

	memoryKey := cfg.MemoryKey
	if memoryKey == "" {
		memoryKey = "chat_history"
	}

	return &ConversationalReActAgent{
		ReActAgent: base,
		memoryKey:  memoryKey,
	}, nil
}

// Plan decides what action to take with conversation history
func (a *ConversationalReActAgent) Plan(ctx context.Context, input AgentInput) (AgentAction, error) {
	prompt := a.buildConversationalPrompt(input)

	resp, err := a.llm.GenerateCompletion(ctx, &llm.CompletionRequest{
		UserPrompt:  prompt,
		Temperature: 0.0,
		MaxTokens:   1000,
	})
	if err != nil {
		return AgentAction{}, fmt.Errorf("llm error: %w", err)
	}

	return a.parseOutput(resp.Text)
}

// buildConversationalPrompt builds prompt with conversation history
func (a *ConversationalReActAgent) buildConversationalPrompt(input AgentInput) string {
	toolDescriptions := a.buildToolDescriptions()
	toolNames := a.buildToolNames()

	var scratchpad string
	for _, step := range input.IntermediateSteps {
		scratchpad += step.Action.Log
		scratchpad += fmt.Sprintf("\nObservation: %s\n", step.Observation)
	}

	historySection := ""
	if input.ChatHistory != "" {
		historySection = fmt.Sprintf("\nPrevious conversation:\n%s\n", input.ChatHistory)
	}

	prompt := fmt.Sprintf(`Assistant is a large language model trained to be helpful.

Assistant has access to the following tools:

%s

To use a tool, please use the following format:

Thought: Do I need to use a tool? Yes
Action: the action to take, should be one of [%s]
Action Input: the input to the action
Observation: the result of the action

When you have a response to say to the Human, or if you do not need to use a tool, you MUST use the format:

Thought: Do I need to use a tool? No
Final Answer: [your response here]

Begin!
%s
New input: %s
%s`, toolDescriptions, toolNames, historySection, input.Input, scratchpad)

	if scratchpad == "" {
		prompt += "Thought: "
	}

	return prompt
}

// InputKeys returns the expected input keys
func (a *ConversationalReActAgent) InputKeys() []string {
	return []string{"input", a.memoryKey}
}
