package multiagent

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// ConversableAgent is the base class for agents that can converse with each other.
// Inspired by AutoGen's ConversableAgent, it provides a foundation for multi-agent conversations.
type ConversableAgent struct {
	mu sync.RWMutex

	// Agent identity
	name        string
	systemMessage string
	description string

	// LLM configuration
	llmProvider LLMProvider

	// Capabilities
	capabilities []string

	// Conversation state
	conversationHistory []ConversationMessage
	maxHistoryLength    int

	// Human-in-the-loop
	humanInputMode HumanInputMode
	humanInputFunc HumanInputFunc

	// Reply handlers
	replyFuncs []ReplyFunc

	// Termination
	maxConsecutiveAutoReply int
	autoReplyCounter        int
	isTerminated            bool
}

// ConversationMessage represents a message in a conversation
type ConversationMessage struct {
	Role    string                 `json:"role"`    // "user", "assistant", "system", "function"
	Content string                 `json:"content"` // Message content
	Name    string                 `json:"name,omitempty"`    // Sender name
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// HumanInputMode defines when human input is requested
type HumanInputMode string

const (
	// HumanInputNever - human input is never requested
	HumanInputNever HumanInputMode = "NEVER"

	// HumanInputTerminate - human input requested only when termination condition is met
	HumanInputTerminate HumanInputMode = "TERMINATE"

	// HumanInputAlways - human input is always requested
	HumanInputAlways HumanInputMode = "ALWAYS"
)

// HumanInputFunc is a function that gets human input
type HumanInputFunc func(ctx context.Context, prompt string) (string, error)

// ReplyFunc is a function that generates a reply
type ReplyFunc func(ctx context.Context, messages []ConversationMessage, sender *ConversableAgent) (string, bool, error)

// ConversableAgentConfig configures a conversable agent
type ConversableAgentConfig struct {
	// Name is the agent name (required)
	Name string

	// SystemMessage is the system prompt for the agent
	SystemMessage string

	// Description describes the agent's role
	Description string

	// LLMProvider is the LLM to use (optional for some agent types)
	LLMProvider LLMProvider

	// Capabilities are the agent's capabilities
	Capabilities []string

	// HumanInputMode controls when human input is requested
	HumanInputMode HumanInputMode

	// HumanInputFunc is the function to get human input
	HumanInputFunc HumanInputFunc

	// MaxConsecutiveAutoReply is the max auto-replies before termination (default: 100)
	MaxConsecutiveAutoReply int

	// MaxHistoryLength limits conversation history (0 = unlimited)
	MaxHistoryLength int
}

// NewConversableAgent creates a new conversable agent
func NewConversableAgent(cfg ConversableAgentConfig) (*ConversableAgent, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	maxAutoReply := cfg.MaxConsecutiveAutoReply
	if maxAutoReply <= 0 {
		maxAutoReply = 100
	}

	return &ConversableAgent{
		name:                    cfg.Name,
		systemMessage:           cfg.SystemMessage,
		description:             cfg.Description,
		llmProvider:             cfg.LLMProvider,
		capabilities:            cfg.Capabilities,
		conversationHistory:     make([]ConversationMessage, 0),
		maxHistoryLength:        cfg.MaxHistoryLength,
		humanInputMode:          cfg.HumanInputMode,
		humanInputFunc:          cfg.HumanInputFunc,
		maxConsecutiveAutoReply: maxAutoReply,
	}, nil
}

// Name returns the agent name
func (a *ConversableAgent) Name() string {
	return a.name
}

// Description returns the agent description
func (a *ConversableAgent) Description() string {
	return a.description
}

// SystemMessage returns the system message
func (a *ConversableAgent) SystemMessage() string {
	return a.systemMessage
}

// Send sends a message to another agent and gets a reply
func (a *ConversableAgent) Send(ctx context.Context, message string, recipient *ConversableAgent, requestReply bool) (string, error) {
	// Add message to our history
	a.addMessage(ConversationMessage{
		Role:    "assistant",
		Content: message,
		Name:    a.name,
	})

	// Recipient receives the message
	recipient.Receive(ctx, message, a)

	if requestReply {
		return recipient.GenerateReply(ctx, a)
	}

	return "", nil
}

// Receive receives a message from another agent
func (a *ConversableAgent) Receive(ctx context.Context, message string, sender *ConversableAgent) {
	senderName := ""
	if sender != nil {
		senderName = sender.Name()
	}

	a.addMessage(ConversationMessage{
		Role:    "user",
		Content: message,
		Name:    senderName,
	})
}

// GenerateReply generates a reply based on conversation history
func (a *ConversableAgent) GenerateReply(ctx context.Context, sender *ConversableAgent) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check termination
	if a.isTerminated {
		return "", fmt.Errorf("agent is terminated")
	}

	// Check human input mode
	if a.humanInputMode == HumanInputAlways {
		if a.humanInputFunc != nil {
			return a.humanInputFunc(ctx, "Please provide input:")
		}
		return "", fmt.Errorf("human input required but no input function provided")
	}

	// Try custom reply functions first
	for _, replyFunc := range a.replyFuncs {
		reply, handled, err := replyFunc(ctx, a.conversationHistory, sender)
		if err != nil {
			return "", err
		}
		if handled {
			return reply, nil
		}
	}

	// Use LLM if available
	if a.llmProvider != nil {
		return a.generateLLMReply(ctx)
	}

	// No way to generate reply
	return "", fmt.Errorf("no LLM provider or reply function available")
}

// generateLLMReply generates a reply using the LLM
func (a *ConversableAgent) generateLLMReply(ctx context.Context) (string, error) {
	// Build prompt from history
	var promptBuilder strings.Builder

	for _, msg := range a.conversationHistory {
		if msg.Name != "" {
			promptBuilder.WriteString(fmt.Sprintf("%s (%s): %s\n", msg.Name, msg.Role, msg.Content))
		} else {
			promptBuilder.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
		}
	}
	promptBuilder.WriteString(fmt.Sprintf("%s: ", a.name))

	resp, err := a.llmProvider.GenerateCompletion(ctx, &CompletionRequest{
		SystemPrompt: a.systemMessage,
		UserPrompt:   promptBuilder.String(),
		Temperature:  0.7,
		MaxTokens:    1000,
	})
	if err != nil {
		return "", err
	}

	// Increment auto-reply counter and check termination
	a.autoReplyCounter++
	if a.autoReplyCounter >= a.maxConsecutiveAutoReply {
		a.isTerminated = true
	}

	reply := resp.Text

	// Add reply to history
	a.conversationHistory = append(a.conversationHistory, ConversationMessage{
		Role:    "assistant",
		Content: reply,
		Name:    a.name,
	})

	return reply, nil
}

// RegisterReplyFunc registers a custom reply function
func (a *ConversableAgent) RegisterReplyFunc(fn ReplyFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.replyFuncs = append(a.replyFuncs, fn)
}

// addMessage adds a message to history with length limiting
func (a *ConversableAgent) addMessage(msg ConversationMessage) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.conversationHistory = append(a.conversationHistory, msg)

	// Limit history length if configured
	if a.maxHistoryLength > 0 && len(a.conversationHistory) > a.maxHistoryLength {
		a.conversationHistory = a.conversationHistory[len(a.conversationHistory)-a.maxHistoryLength:]
	}
}

// GetHistory returns the conversation history
func (a *ConversableAgent) GetHistory() []ConversationMessage {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]ConversationMessage, len(a.conversationHistory))
	copy(result, a.conversationHistory)
	return result
}

// ClearHistory clears conversation history
func (a *ConversableAgent) ClearHistory() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.conversationHistory = make([]ConversationMessage, 0)
	a.autoReplyCounter = 0
	a.isTerminated = false
}

// ResetAutoReplyCounter resets the auto-reply counter
func (a *ConversableAgent) ResetAutoReplyCounter() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.autoReplyCounter = 0
	a.isTerminated = false
}

// IsTerminated returns whether the agent is terminated
func (a *ConversableAgent) IsTerminated() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isTerminated
}

// AssistantAgent is a conversable agent that acts as an AI assistant
type AssistantAgent struct {
	*ConversableAgent
}

// NewAssistantAgent creates a new assistant agent
func NewAssistantAgent(name string, llmProvider LLMProvider, systemMessage string) (*AssistantAgent, error) {
	if systemMessage == "" {
		systemMessage = "You are a helpful AI assistant. Solve tasks using your reasoning and language skills. " +
			"When you need to perform an action, explain what you're doing. " +
			"Reply TERMINATE when the task is done."
	}

	base, err := NewConversableAgent(ConversableAgentConfig{
		Name:           name,
		SystemMessage:  systemMessage,
		LLMProvider:    llmProvider,
		HumanInputMode: HumanInputNever,
		Capabilities:   []string{"assistant", "reasoning", "coding"},
	})
	if err != nil {
		return nil, err
	}

	return &AssistantAgent{ConversableAgent: base}, nil
}

// UserProxyAgent represents a human user in the conversation
type UserProxyAgent struct {
	*ConversableAgent
	codeExecutionEnabled bool
}

// NewUserProxyAgent creates a new user proxy agent
func NewUserProxyAgent(name string, humanInputFunc HumanInputFunc) (*UserProxyAgent, error) {
	base, err := NewConversableAgent(ConversableAgentConfig{
		Name:           name,
		HumanInputMode: HumanInputAlways,
		HumanInputFunc: humanInputFunc,
		Capabilities:   []string{"human", "feedback"},
	})
	if err != nil {
		return nil, err
	}

	return &UserProxyAgent{
		ConversableAgent:    base,
		codeExecutionEnabled: false,
	}, nil
}

// EnableCodeExecution enables code execution for this agent
func (a *UserProxyAgent) EnableCodeExecution() {
	a.codeExecutionEnabled = true
}

// IsCodeExecutionEnabled returns whether code execution is enabled
func (a *UserProxyAgent) IsCodeExecutionEnabled() bool {
	return a.codeExecutionEnabled
}
