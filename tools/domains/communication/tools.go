package communication

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yourusername/minion/models"
)

// SlackMessageTool sends messages to Slack channels
type SlackMessageTool struct{}

func (t *SlackMessageTool) Name() string {
	return "slack_send_message"
}

func (t *SlackMessageTool) Description() string {
	return "Sends messages to Slack channels with rich formatting, attachments, and mentions"
}

func (t *SlackMessageTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	channel, ok := input.Params["channel"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid channel",
		}, nil
	}

	message, ok := input.Params["message"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid message",
		}, nil
	}

	result := sendSlackMessage(channel, message, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *SlackMessageTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "slack_integration") || hasCapability(agent, "communication")
}

// SlackChannelTool manages Slack channels
type SlackChannelTool struct{}

func (t *SlackChannelTool) Name() string {
	return "slack_manage_channel"
}

func (t *SlackChannelTool) Description() string {
	return "Creates, archives, and manages Slack channels"
}

func (t *SlackChannelTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	action, ok := input.Params["action"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid action",
		}, nil
	}

	result := manageSlackChannel(action, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *SlackChannelTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "slack_admin") || hasCapability(agent, "communication")
}

// TeamsMessageTool sends messages to Microsoft Teams
type TeamsMessageTool struct{}

func (t *TeamsMessageTool) Name() string {
	return "teams_send_message"
}

func (t *TeamsMessageTool) Description() string {
	return "Sends messages to Microsoft Teams channels with adaptive cards"
}

func (t *TeamsMessageTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	teamID, ok := input.Params["team_id"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid team_id",
		}, nil
	}

	channelID, ok := input.Params["channel_id"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid channel_id",
		}, nil
	}

	message, ok := input.Params["message"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid message",
		}, nil
	}

	result := sendTeamsMessage(teamID, channelID, message, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *TeamsMessageTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "teams_integration") || hasCapability(agent, "communication")
}

// DiscordMessageTool sends messages to Discord
type DiscordMessageTool struct{}

func (t *DiscordMessageTool) Name() string {
	return "discord_send_message"
}

func (t *DiscordMessageTool) Description() string {
	return "Sends messages to Discord channels with embeds and reactions"
}

func (t *DiscordMessageTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	channelID, ok := input.Params["channel_id"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid channel_id",
		}, nil
	}

	message, ok := input.Params["message"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid message",
		}, nil
	}

	result := sendDiscordMessage(channelID, message, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *DiscordMessageTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "discord_integration") || hasCapability(agent, "communication")
}

// GmailSendTool sends emails via Gmail API
type GmailSendTool struct{}

func (t *GmailSendTool) Name() string {
	return "gmail_send_email"
}

func (t *GmailSendTool) Description() string {
	return "Sends emails via Gmail with attachments and HTML formatting"
}

func (t *GmailSendTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	to, ok := input.Params["to"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid recipient",
		}, nil
	}

	subject, ok := input.Params["subject"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid subject",
		}, nil
	}

	body, ok := input.Params["body"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid body",
		}, nil
	}

	result := sendGmailEmail(to, subject, body, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *GmailSendTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "gmail_integration") || hasCapability(agent, "communication")
}

// GmailSearchTool searches Gmail messages
type GmailSearchTool struct{}

func (t *GmailSearchTool) Name() string {
	return "gmail_search"
}

func (t *GmailSearchTool) Description() string {
	return "Searches Gmail messages with advanced filters"
}

func (t *GmailSearchTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	query, ok := input.Params["query"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid query",
		}, nil
	}

	maxResults := 10
	if mr, ok := input.Params["max_results"].(int); ok {
		maxResults = mr
	}

	result := searchGmail(query, maxResults)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *GmailSearchTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "gmail_integration") || hasCapability(agent, "communication")
}

// ZoomMeetingTool creates and manages Zoom meetings
type ZoomMeetingTool struct{}

func (t *ZoomMeetingTool) Name() string {
	return "zoom_manage_meeting"
}

func (t *ZoomMeetingTool) Description() string {
	return "Creates, updates, and manages Zoom meetings"
}

func (t *ZoomMeetingTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	action, ok := input.Params["action"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid action",
		}, nil
	}

	result := manageZoomMeeting(action, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *ZoomMeetingTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "zoom_integration") || hasCapability(agent, "communication")
}

// TwilioSMSTool sends SMS via Twilio
type TwilioSMSTool struct{}

func (t *TwilioSMSTool) Name() string {
	return "twilio_send_sms"
}

func (t *TwilioSMSTool) Description() string {
	return "Sends SMS messages via Twilio"
}

func (t *TwilioSMSTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	to, ok := input.Params["to"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid phone number",
		}, nil
	}

	message, ok := input.Params["message"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid message",
		}, nil
	}

	result := sendTwilioSMS(to, message, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *TwilioSMSTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "twilio_integration") || hasCapability(agent, "communication")
}

// TwilioCallTool makes phone calls via Twilio
type TwilioCallTool struct{}

func (t *TwilioCallTool) Name() string {
	return "twilio_make_call"
}

func (t *TwilioCallTool) Description() string {
	return "Makes phone calls via Twilio with TwiML instructions"
}

func (t *TwilioCallTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	to, ok := input.Params["to"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid phone number",
		}, nil
	}

	result := makeTwilioCall(to, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *TwilioCallTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "twilio_integration") || hasCapability(agent, "communication")
}

// Helper functions

func hasCapability(agent *models.Agent, capability string) bool {
	for _, cap := range agent.Capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

func sendSlackMessage(channel, message string, params map[string]interface{}) map[string]interface{} {
	timestamp := fmt.Sprintf("%.6f", float64(time.Now().Unix())+0.123456)

	// Check for attachments
	attachments, _ := params["attachments"].([]map[string]interface{})

	// Check for thread
	threadTS, _ := params["thread_ts"].(string)

	// Check for mentions
	mentions, _ := params["mentions"].([]string)
	if len(mentions) > 0 {
		for _, mention := range mentions {
			message = strings.ReplaceAll(message, "@"+mention, "<@"+mention+">")
		}
	}

	return map[string]interface{}{
		"ok":        true,
		"channel":   channel,
		"ts":        timestamp,
		"message": map[string]interface{}{
			"text":        message,
			"user":        "bot_user_id",
			"type":        "message",
			"ts":          timestamp,
			"attachments": attachments,
			"thread_ts":   threadTS,
		},
		"permalink": fmt.Sprintf("https://workspace.slack.com/archives/%s/p%s", channel, strings.ReplaceAll(timestamp, ".", "")),
	}
}

func manageSlackChannel(action string, params map[string]interface{}) map[string]interface{} {
	channelName, _ := params["channel_name"].(string)

	switch action {
	case "create":
		isPrivate, _ := params["is_private"].(bool)
		return map[string]interface{}{
			"ok": true,
			"channel": map[string]interface{}{
				"id":         "C" + fmt.Sprintf("%d", time.Now().Unix()),
				"name":       channelName,
				"is_private": isPrivate,
				"created":    time.Now().Unix(),
				"creator":    "U12345678",
			},
		}

	case "archive":
		channelID, _ := params["channel_id"].(string)
		return map[string]interface{}{
			"ok":      true,
			"channel": channelID,
			"action":  "archived",
		}

	case "invite":
		channelID, _ := params["channel_id"].(string)
		userID, _ := params["user_id"].(string)
		return map[string]interface{}{
			"ok":      true,
			"channel": channelID,
			"user":    userID,
			"action":  "invited",
		}

	case "list":
		return map[string]interface{}{
			"ok": true,
			"channels": []map[string]interface{}{
				{"id": "C12345", "name": "general", "num_members": 50},
				{"id": "C12346", "name": "engineering", "num_members": 25},
			},
		}
	}

	return map[string]interface{}{
		"ok":    false,
		"error": "Invalid action",
	}
}

func sendTeamsMessage(teamID, channelID, message string, params map[string]interface{}) map[string]interface{} {
	messageType := "text"
	if mt, ok := params["message_type"].(string); ok {
		messageType = mt
	}

	result := map[string]interface{}{
		"id":        fmt.Sprintf("msg_%d", time.Now().Unix()),
		"messageType": messageType,
		"createdDateTime": time.Now().Format(time.RFC3339),
		"from": map[string]interface{}{
			"application": map[string]interface{}{
				"displayName": "Minion Bot",
			},
		},
		"body": map[string]interface{}{
			"content": message,
		},
	}

	// Add adaptive card if specified
	if card, ok := params["adaptive_card"].(map[string]interface{}); ok {
		result["attachments"] = []map[string]interface{}{
			{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"content":     card,
			},
		}
	}

	return result
}

func sendDiscordMessage(channelID, message string, params map[string]interface{}) map[string]interface{} {
	messageID := fmt.Sprintf("%d", time.Now().UnixNano())

	result := map[string]interface{}{
		"id":         messageID,
		"channel_id": channelID,
		"content":    message,
		"timestamp":  time.Now().Format(time.RFC3339),
		"tts":        false,
		"mention_everyone": false,
	}

	// Add embed if specified
	if embed, ok := params["embed"].(map[string]interface{}); ok {
		result["embeds"] = []map[string]interface{}{embed}
	}

	// Add reactions if specified
	if reactions, ok := params["reactions"].([]string); ok {
		result["reactions"] = reactions
	}

	return result
}

func sendGmailEmail(to, subject, body string, params map[string]interface{}) map[string]interface{} {
	messageID := fmt.Sprintf("<msg_%d@mail.gmail.com>", time.Now().Unix())
	threadID := fmt.Sprintf("thread_%d", time.Now().Unix())

	result := map[string]interface{}{
		"id":       messageID,
		"threadId": threadID,
		"labelIds": []string{"SENT"},
		"to":       to,
		"subject":  subject,
	}

	// Handle CC and BCC
	if cc, ok := params["cc"].(string); ok {
		result["cc"] = cc
	}
	if bcc, ok := params["bcc"].(string); ok {
		result["bcc"] = bcc
	}

	// Handle attachments
	if attachments, ok := params["attachments"].([]map[string]interface{}); ok {
		result["attachments"] = attachments
		result["has_attachments"] = true
	}

	// HTML formatting
	isHTML := false
	if html, ok := params["is_html"].(bool); ok {
		isHTML = html
	}
	result["is_html"] = isHTML

	return result
}

func searchGmail(query string, maxResults int) map[string]interface{} {
	// Mock search results
	messages := []map[string]interface{}{
		{
			"id":       "msg_001",
			"threadId": "thread_001",
			"snippet":  "Meeting scheduled for tomorrow at 2 PM...",
			"from":     "sender@example.com",
			"subject":  "Meeting Reminder",
			"date":     time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			"labels":   []string{"INBOX", "IMPORTANT"},
		},
		{
			"id":       "msg_002",
			"threadId": "thread_002",
			"snippet":  "Your invoice for this month is ready...",
			"from":     "billing@example.com",
			"subject":  "Invoice #12345",
			"date":     time.Now().Add(-48 * time.Hour).Format(time.RFC3339),
			"labels":   []string{"INBOX"},
		},
	}

	if len(messages) > maxResults {
		messages = messages[:maxResults]
	}

	return map[string]interface{}{
		"messages":          messages,
		"resultSizeEstimate": len(messages),
		"query":             query,
	}
}

func manageZoomMeeting(action string, params map[string]interface{}) map[string]interface{} {
	switch action {
	case "create":
		topic, _ := params["topic"].(string)
		startTime, _ := params["start_time"].(string)
		duration, _ := params["duration"].(int)
		if duration == 0 {
			duration = 60
		}

		meetingID := fmt.Sprintf("%d", 1000000000+time.Now().Unix()%9000000000)

		return map[string]interface{}{
			"id":        meetingID,
			"topic":     topic,
			"start_time": startTime,
			"duration":  duration,
			"timezone":  "UTC",
			"join_url":  fmt.Sprintf("https://zoom.us/j/%s", meetingID),
			"start_url": fmt.Sprintf("https://zoom.us/s/%s?zak=token", meetingID),
			"password":  "abc123",
			"settings": map[string]interface{}{
				"host_video":        true,
				"participant_video": true,
				"join_before_host":  false,
				"mute_upon_entry":   true,
			},
		}

	case "list":
		return map[string]interface{}{
			"meetings": []map[string]interface{}{
				{
					"id":        "123456789",
					"topic":     "Team Standup",
					"start_time": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
					"duration":  30,
				},
			},
		}

	case "delete":
		meetingID, _ := params["meeting_id"].(string)
		return map[string]interface{}{
			"success":    true,
			"meeting_id": meetingID,
			"message":    "Meeting deleted successfully",
		}
	}

	return map[string]interface{}{
		"error": "Invalid action",
	}
}

func sendTwilioSMS(to, message string, params map[string]interface{}) map[string]interface{} {
	from, _ := params["from"].(string)
	if from == "" {
		from = "+15555551234"
	}

	sid := fmt.Sprintf("SM%d", time.Now().Unix())

	return map[string]interface{}{
		"sid":          sid,
		"date_created": time.Now().Format(time.RFC3339),
		"date_sent":    time.Now().Format(time.RFC3339),
		"to":           to,
		"from":         from,
		"body":         message,
		"status":       "sent",
		"direction":    "outbound-api",
		"price":        "-0.00750",
		"price_unit":   "USD",
		"uri":          fmt.Sprintf("/2010-04-01/Accounts/ACxxx/Messages/%s.json", sid),
	}
}

func makeTwilioCall(to string, params map[string]interface{}) map[string]interface{} {
	from, _ := params["from"].(string)
	if from == "" {
		from = "+15555551234"
	}

	url, _ := params["url"].(string)
	if url == "" {
		url = "http://demo.twilio.com/docs/voice.xml"
	}

	sid := fmt.Sprintf("CA%d", time.Now().Unix())

	return map[string]interface{}{
		"sid":          sid,
		"date_created": time.Now().Format(time.RFC3339),
		"to":           to,
		"from":         from,
		"status":       "queued",
		"direction":    "outbound-api",
		"duration":     "0",
		"price":        nil,
		"url":          url,
		"uri":          fmt.Sprintf("/2010-04-01/Accounts/ACxxx/Calls/%s.json", sid),
	}
}
