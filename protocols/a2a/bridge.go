package a2a

import (
	"context"
	"fmt"

	"github.com/Ranganaths/minion/core"
	"github.com/Ranganaths/minion/models"
	"github.com/Ranganaths/minion/skills"
	"github.com/google/uuid"
)

// MinionBridge bridges the Minion framework with the A2A protocol
type MinionBridge struct {
	framework core.Framework
	agentID   string
}

// NewMinionBridge creates a new bridge between Minion and A2A
func NewMinionBridge(framework core.Framework, agentID string) *MinionBridge {
	return &MinionBridge{
		framework: framework,
		agentID:   agentID,
	}
}

// HandleTask implements TaskHandler for synchronous task handling
func (b *MinionBridge) HandleTask(ctx context.Context, task *Task) error {
	// Convert A2A message to Minion input
	input := b.messageToInput(task)

	// Execute via Minion framework
	output, err := b.framework.Execute(ctx, b.agentID, input)
	if err != nil {
		task.UpdateState(TaskStateFailed, &Message{
			Role:  MessageRoleAgent,
			Parts: []Part{NewTextPart(fmt.Sprintf("Execution failed: %v", err))},
		})
		return err
	}

	// Convert output to A2A message
	responseMsg := b.outputToMessage(output)
	task.AddMessage(responseMsg)

	// Add any artifacts
	if output.Metadata != nil {
		if artifacts, ok := output.Metadata["artifacts"].([]interface{}); ok {
			for i, a := range artifacts {
				if artifactMap, ok := a.(map[string]interface{}); ok {
					artifact := Artifact{
						ID:    uuid.New().String(),
						Index: i,
					}
					if name, ok := artifactMap["name"].(string); ok {
						artifact.Name = name
					}
					if desc, ok := artifactMap["description"].(string); ok {
						artifact.Description = desc
					}
					if content, ok := artifactMap["content"].(string); ok {
						artifact.Parts = []Part{NewTextPart(content)}
					}
					task.AddArtifact(artifact)
				}
			}
		}
	}

	task.UpdateState(TaskStateCompleted, &responseMsg)
	return nil
}

// HandleTaskStream implements TaskHandler for streaming task handling
func (b *MinionBridge) HandleTaskStream(ctx context.Context, task *Task, updates chan<- TaskUpdate) error {
	// Convert A2A message to Minion input
	input := b.messageToInput(task)

	// For now, we execute synchronously and send updates
	// TODO: Integrate with Minion's streaming capabilities when available

	// Send working status
	updates <- NewStatusUpdate(TaskStateWorking, nil)

	// Execute via Minion framework
	output, err := b.framework.Execute(ctx, b.agentID, input)
	if err != nil {
		errMsg := Message{
			Role:  MessageRoleAgent,
			Parts: []Part{NewTextPart(fmt.Sprintf("Execution failed: %v", err))},
		}
		updates <- NewStatusUpdate(TaskStateFailed, &errMsg)
		return err
	}

	// Convert output to A2A message
	responseMsg := b.outputToMessage(output)
	task.AddMessage(responseMsg)
	updates <- NewMessageUpdate(&responseMsg)

	// Add any artifacts
	if output.Metadata != nil {
		if artifacts, ok := output.Metadata["artifacts"].([]interface{}); ok {
			for i, a := range artifacts {
				if artifactMap, ok := a.(map[string]interface{}); ok {
					artifact := Artifact{
						ID:    uuid.New().String(),
						Index: i,
					}
					if name, ok := artifactMap["name"].(string); ok {
						artifact.Name = name
					}
					if desc, ok := artifactMap["description"].(string); ok {
						artifact.Description = desc
					}
					if content, ok := artifactMap["content"].(string); ok {
						artifact.Parts = []Part{NewTextPart(content)}
					}
					task.AddArtifact(artifact)
					updates <- NewArtifactUpdate(&artifact)
				}
			}
		}
	}

	updates <- NewStatusUpdate(TaskStateCompleted, &responseMsg)
	return nil
}

// SupportsStreaming returns true as we support streaming
func (b *MinionBridge) SupportsStreaming() bool {
	return true
}

// messageToInput converts an A2A message to Minion input
func (b *MinionBridge) messageToInput(task *Task) *models.Input {
	// Get the last user message
	var lastMessage *Message
	for i := len(task.History) - 1; i >= 0; i-- {
		if task.History[i].Role == MessageRoleUser {
			lastMessage = &task.History[i]
			break
		}
	}

	if lastMessage == nil {
		return &models.Input{
			Raw:  "",
			Type: "text",
		}
	}

	// Extract text content
	var textContent string
	for _, part := range lastMessage.Parts {
		if part.Type == PartTypeText {
			textContent += part.Text
		}
	}

	// Build context from conversation history
	contextData := make(map[string]interface{})
	var history []map[string]string
	for _, msg := range task.History {
		if &msg == lastMessage {
			continue // Skip the current message
		}
		role := "user"
		if msg.Role == MessageRoleAgent {
			role = "assistant"
		}
		history = append(history, map[string]string{
			"role":    role,
			"content": msg.GetText(),
		})
	}
	contextData["history"] = history
	contextData["a2a_task_id"] = task.ID
	contextData["a2a_session_id"] = task.SessionID

	return &models.Input{
		Raw:     textContent,
		Type:    "text",
		Context: contextData,
	}
}

// outputToMessage converts Minion output to an A2A message
func (b *MinionBridge) outputToMessage(output *models.Output) Message {
	// Extract response from Result
	response := ""
	if output.Result != nil {
		response = fmt.Sprintf("%v", output.Result)
	}
	parts := []Part{NewTextPart(response)}

	// Add any additional data as parts
	if output.Metadata != nil {
		if data, ok := output.Metadata["structured_data"]; ok {
			dataStr := fmt.Sprintf("%v", data)
			parts = append(parts, Part{
				Type:     PartTypeData,
				MimeType: "application/json",
				Data:     dataStr,
			})
		}
	}

	return Message{
		Role:  MessageRoleAgent,
		Parts: parts,
	}
}

// AgentToAgentCard converts a Minion agent to an A2A Agent Card
func AgentToAgentCard(agent *models.Agent, baseURL string) AgentCard {
	builder := NewAgentCardBuilder(
		agent.Name,
		agent.Description,
		baseURL,
	).
		WithVersion("1.0").
		WithStreaming(true).
		WithStateHistory(true)

	// Convert capabilities to skills
	for _, cap := range agent.Capabilities {
		builder.AddSkill(AgentSkill{
			ID:          cap,
			Name:        cap,
			Description: fmt.Sprintf("Agent capability: %s", cap),
			Tags:        []string{"capability"},
		})
	}

	// Add metadata
	if agent.Metadata != nil {
		for k, v := range agent.Metadata {
			builder.WithMetadata(k, v)
		}
	}

	return builder.Build()
}

// SkillsToAgentSkills converts Minion skills to A2A Agent Skills
func SkillsToAgentSkills(minionSkills []skills.SkillInfo) []AgentSkill {
	agentSkills := make([]AgentSkill, 0, len(minionSkills))

	for _, skill := range minionSkills {
		agentSkill := AgentSkill{
			ID:          skill.Name,
			Name:        skill.Name,
			Description: skill.Description,
			Tags:        skill.Tags,
		}

		agentSkills = append(agentSkills, agentSkill)
	}

	return agentSkills
}

// A2AServer wraps the A2A server with Minion integration
type A2AServer struct {
	*Server
	framework core.Framework
	agentID   string
}

// NewA2AServer creates an A2A server integrated with Minion
func NewA2AServer(framework core.Framework, agentID string, baseURL string, config ServerConfig) (*A2AServer, error) {
	// Get agent
	ctx := context.Background()
	agent, err := framework.GetAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Create agent card
	card := AgentToAgentCard(agent, baseURL)

	// Add skills from framework
	skillInfos := framework.ListSkills()
	card.Skills = append(card.Skills, SkillsToAgentSkills(skillInfos)...)

	// Create bridge
	bridge := NewMinionBridge(framework, agentID)

	// Create task manager
	taskManager := NewTaskManager(bridge, DefaultTaskManagerConfig())

	// Create server
	server := NewServer(&card, taskManager, config)

	return &A2AServer{
		Server:    server,
		framework: framework,
		agentID:   agentID,
	}, nil
}

// UpdateAgentCard refreshes the agent card from the current agent state
func (s *A2AServer) UpdateAgentCard(ctx context.Context) error {
	agent, err := s.framework.GetAgent(ctx, s.agentID)
	if err != nil {
		return err
	}

	card := AgentToAgentCard(agent, s.card.URL)
	skillInfos := s.framework.ListSkills()
	card.Skills = append(card.Skills, SkillsToAgentSkills(skillInfos)...)

	s.mu.Lock()
	s.card = &card
	s.mu.Unlock()

	return nil
}
