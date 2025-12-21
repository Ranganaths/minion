package multiagent

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// GroupChat manages a conversation between multiple agents
type GroupChat struct {
	mu sync.RWMutex

	// Agents in the group
	agents []*ConversableAgent

	// Conversation state
	messages []ConversationMessage

	// Configuration
	maxRounds       int
	speakerSelection SpeakerSelectionMode
	allowRepeatSpeaker bool

	// LLM for auto speaker selection
	llmProvider LLMProvider

	// Admin agent (optional)
	adminName string

	// Current speaker
	currentSpeaker *ConversableAgent
	lastSpeaker    *ConversableAgent

	// Round robin state
	roundRobinIndex int
}

// SpeakerSelectionMode defines how the next speaker is selected
type SpeakerSelectionMode string

const (
	// SpeakerSelectionRoundRobin rotates through agents in order
	SpeakerSelectionRoundRobin SpeakerSelectionMode = "round_robin"

	// SpeakerSelectionRandom selects a random agent
	SpeakerSelectionRandom SpeakerSelectionMode = "random"

	// SpeakerSelectionManual requires human selection
	SpeakerSelectionManual SpeakerSelectionMode = "manual"

	// SpeakerSelectionAuto uses LLM to decide the next speaker
	SpeakerSelectionAuto SpeakerSelectionMode = "auto"
)

// GroupChatConfig configures a group chat
type GroupChatConfig struct {
	// Agents are the agents in the group (required, min 2)
	Agents []*ConversableAgent

	// MaxRounds is the maximum conversation rounds (default: 10)
	MaxRounds int

	// SpeakerSelectionMode defines how speakers are selected
	SpeakerSelectionMode SpeakerSelectionMode

	// AllowRepeatSpeaker allows the same agent to speak twice in a row
	AllowRepeatSpeaker bool

	// LLMProvider is required for auto speaker selection
	LLMProvider LLMProvider

	// AdminName is the name of the admin agent (optional)
	AdminName string
}

// NewGroupChat creates a new group chat
func NewGroupChat(cfg GroupChatConfig) (*GroupChat, error) {
	if len(cfg.Agents) < 2 {
		return nil, fmt.Errorf("at least 2 agents are required for group chat")
	}

	maxRounds := cfg.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 10
	}

	selectionMode := cfg.SpeakerSelectionMode
	if selectionMode == "" {
		selectionMode = SpeakerSelectionRoundRobin
	}

	if selectionMode == SpeakerSelectionAuto && cfg.LLMProvider == nil {
		return nil, fmt.Errorf("LLMProvider is required for auto speaker selection")
	}

	return &GroupChat{
		agents:             cfg.Agents,
		messages:           make([]ConversationMessage, 0),
		maxRounds:          maxRounds,
		speakerSelection:   selectionMode,
		allowRepeatSpeaker: cfg.AllowRepeatSpeaker,
		llmProvider:        cfg.LLMProvider,
		adminName:          cfg.AdminName,
	}, nil
}

// Run starts the group chat conversation
func (gc *GroupChat) Run(ctx context.Context, initialMessage string, initiator *ConversableAgent) ([]ConversationMessage, error) {
	gc.mu.Lock()
	gc.messages = make([]ConversationMessage, 0)
	gc.mu.Unlock()

	// Add initial message
	gc.addMessage(ConversationMessage{
		Role:    "user",
		Content: initialMessage,
		Name:    initiator.Name(),
	})

	// Broadcast initial message to all agents
	for _, agent := range gc.agents {
		if agent != initiator {
			agent.Receive(ctx, initialMessage, initiator)
		}
	}

	gc.lastSpeaker = initiator

	// Run conversation rounds
	for round := 0; round < gc.maxRounds; round++ {
		select {
		case <-ctx.Done():
			return gc.GetMessages(), ctx.Err()
		default:
		}

		// Select next speaker
		nextSpeaker, err := gc.selectNextSpeaker(ctx)
		if err != nil {
			return gc.GetMessages(), fmt.Errorf("speaker selection error: %w", err)
		}

		if nextSpeaker == nil {
			break // No more speakers
		}

		gc.currentSpeaker = nextSpeaker

		// Generate reply
		reply, err := nextSpeaker.GenerateReply(ctx, gc.lastSpeaker)
		if err != nil {
			if nextSpeaker.IsTerminated() {
				break
			}
			return gc.GetMessages(), fmt.Errorf("reply generation error: %w", err)
		}

		// Add reply to messages
		gc.addMessage(ConversationMessage{
			Role:    "assistant",
			Content: reply,
			Name:    nextSpeaker.Name(),
		})

		// Broadcast reply to all other agents
		for _, agent := range gc.agents {
			if agent != nextSpeaker {
				agent.Receive(ctx, reply, nextSpeaker)
			}
		}

		// Check for termination
		if gc.isTerminationMessage(reply) {
			break
		}

		gc.lastSpeaker = nextSpeaker
	}

	return gc.GetMessages(), nil
}

// selectNextSpeaker selects the next speaker based on the selection mode
func (gc *GroupChat) selectNextSpeaker(ctx context.Context) (*ConversableAgent, error) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	eligibleAgents := gc.getEligibleAgents()
	if len(eligibleAgents) == 0 {
		return nil, nil
	}

	switch gc.speakerSelection {
	case SpeakerSelectionRoundRobin:
		return gc.selectRoundRobin(eligibleAgents), nil

	case SpeakerSelectionRandom:
		return gc.selectRandom(eligibleAgents), nil

	case SpeakerSelectionAuto:
		return gc.selectAuto(ctx, eligibleAgents)

	case SpeakerSelectionManual:
		// For manual mode, return the first eligible agent
		// In a real implementation, this would prompt for input
		return eligibleAgents[0], nil

	default:
		return gc.selectRoundRobin(eligibleAgents), nil
	}
}

// getEligibleAgents returns agents that can speak next
func (gc *GroupChat) getEligibleAgents() []*ConversableAgent {
	eligible := make([]*ConversableAgent, 0)
	for _, agent := range gc.agents {
		if agent.IsTerminated() {
			continue
		}
		if !gc.allowRepeatSpeaker && gc.lastSpeaker != nil && agent == gc.lastSpeaker {
			continue
		}
		eligible = append(eligible, agent)
	}
	return eligible
}

// selectRoundRobin selects the next agent in round-robin order
func (gc *GroupChat) selectRoundRobin(eligible []*ConversableAgent) *ConversableAgent {
	if len(eligible) == 0 {
		return nil
	}
	gc.roundRobinIndex = (gc.roundRobinIndex + 1) % len(eligible)
	return eligible[gc.roundRobinIndex]
}

// selectRandom selects a random agent
func (gc *GroupChat) selectRandom(eligible []*ConversableAgent) *ConversableAgent {
	if len(eligible) == 0 {
		return nil
	}
	rand.Seed(time.Now().UnixNano())
	return eligible[rand.Intn(len(eligible))]
}

// selectAuto uses LLM to select the next speaker
func (gc *GroupChat) selectAuto(ctx context.Context, eligible []*ConversableAgent) (*ConversableAgent, error) {
	if gc.llmProvider == nil || len(eligible) == 0 {
		return gc.selectRoundRobin(eligible), nil
	}

	// Build agent descriptions
	var agentDescriptions strings.Builder
	for _, agent := range eligible {
		desc := agent.Description()
		if desc == "" {
			desc = "General purpose agent"
		}
		agentDescriptions.WriteString(fmt.Sprintf("- %s: %s\n", agent.Name(), desc))
	}

	// Build conversation summary
	var conversationSummary strings.Builder
	for _, msg := range gc.messages {
		conversationSummary.WriteString(fmt.Sprintf("%s: %s\n", msg.Name, msg.Content))
	}

	prompt := fmt.Sprintf(`You are a group chat manager. Based on the conversation, select the next speaker.

Available speakers:
%s

Recent conversation:
%s

Who should speak next? Reply with ONLY the name of the agent.`, agentDescriptions.String(), conversationSummary.String())

	resp, err := gc.llmProvider.GenerateCompletion(ctx, &CompletionRequest{
		UserPrompt:  prompt,
		Temperature: 0.0,
		MaxTokens:   50,
	})
	if err != nil {
		// Fallback to round robin on error
		return gc.selectRoundRobin(eligible), nil
	}

	// Find agent by name
	selectedName := strings.TrimSpace(resp.Text)
	for _, agent := range eligible {
		if strings.EqualFold(agent.Name(), selectedName) {
			return agent, nil
		}
	}

	// Fallback if name not found
	return gc.selectRoundRobin(eligible), nil
}

// isTerminationMessage checks if a message contains termination signal
func (gc *GroupChat) isTerminationMessage(message string) bool {
	terminationKeywords := []string{"TERMINATE", "TASK COMPLETE", "FINISHED", "DONE"}
	upperMessage := strings.ToUpper(message)
	for _, keyword := range terminationKeywords {
		if strings.Contains(upperMessage, keyword) {
			return true
		}
	}
	return false
}

// addMessage adds a message to the group chat
func (gc *GroupChat) addMessage(msg ConversationMessage) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.messages = append(gc.messages, msg)
}

// GetMessages returns all messages
func (gc *GroupChat) GetMessages() []ConversationMessage {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	result := make([]ConversationMessage, len(gc.messages))
	copy(result, gc.messages)
	return result
}

// GetAgents returns all agents
func (gc *GroupChat) GetAgents() []*ConversableAgent {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	result := make([]*ConversableAgent, len(gc.agents))
	copy(result, gc.agents)
	return result
}

// AddAgent adds an agent to the group
func (gc *GroupChat) AddAgent(agent *ConversableAgent) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.agents = append(gc.agents, agent)
}

// RemoveAgent removes an agent from the group
func (gc *GroupChat) RemoveAgent(name string) bool {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	for i, agent := range gc.agents {
		if agent.Name() == name {
			gc.agents = append(gc.agents[:i], gc.agents[i+1:]...)
			return true
		}
	}
	return false
}

// Reset clears the group chat state
func (gc *GroupChat) Reset() {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	gc.messages = make([]ConversationMessage, 0)
	gc.currentSpeaker = nil
	gc.lastSpeaker = nil
	gc.roundRobinIndex = 0

	for _, agent := range gc.agents {
		agent.ClearHistory()
	}
}

// GroupChatManager manages multiple group chats
type GroupChatManager struct {
	groupChat *GroupChat
	llmProvider LLMProvider
}

// NewGroupChatManager creates a new group chat manager
func NewGroupChatManager(groupChat *GroupChat, llmProvider LLMProvider) *GroupChatManager {
	return &GroupChatManager{
		groupChat:   groupChat,
		llmProvider: llmProvider,
	}
}

// Initiate starts a conversation in the group chat
func (m *GroupChatManager) Initiate(ctx context.Context, message string, initiator *ConversableAgent) ([]ConversationMessage, error) {
	return m.groupChat.Run(ctx, message, initiator)
}
