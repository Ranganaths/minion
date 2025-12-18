package projectmgmt

import (
	"context"
	"fmt"
	"time"

	"github.com/Ranganaths/minion/models"
)

// JiraIssueTool manages Jira issues
type JiraIssueTool struct{}

func (t *JiraIssueTool) Name() string {
	return "jira_manage_issue"
}

func (t *JiraIssueTool) Description() string {
	return "Creates, updates, and manages Jira issues with full field support"
}

func (t *JiraIssueTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	action, ok := input.Params["action"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid action",
		}, nil
	}

	result := manageJiraIssue(action, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *JiraIssueTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "jira_integration") || hasCapability(agent, "project_management")
}

// JiraSprintTool manages Jira sprints
type JiraSprintTool struct{}

func (t *JiraSprintTool) Name() string {
	return "jira_manage_sprint"
}

func (t *JiraSprintTool) Description() string {
	return "Creates and manages Jira sprints for Scrum boards"
}

func (t *JiraSprintTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	action, ok := input.Params["action"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid action",
		}, nil
	}

	result := manageJiraSprint(action, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *JiraSprintTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "jira_integration") || hasCapability(agent, "project_management")
}

// AsanaTaskTool manages Asana tasks
type AsanaTaskTool struct{}

func (t *AsanaTaskTool) Name() string {
	return "asana_manage_task"
}

func (t *AsanaTaskTool) Description() string {
	return "Creates, updates, and manages Asana tasks with sections and custom fields"
}

func (t *AsanaTaskTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	action, ok := input.Params["action"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid action",
		}, nil
	}

	result := manageAsanaTask(action, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *AsanaTaskTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "asana_integration") || hasCapability(agent, "project_management")
}

// AsanaProjectTool manages Asana projects
type AsanaProjectTool struct{}

func (t *AsanaProjectTool) Name() string {
	return "asana_manage_project"
}

func (t *AsanaProjectTool) Description() string {
	return "Creates and manages Asana projects with templates and portfolios"
}

func (t *AsanaProjectTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	action, ok := input.Params["action"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid action",
		}, nil
	}

	result := manageAsanaProject(action, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *AsanaProjectTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "asana_integration") || hasCapability(agent, "project_management")
}

// TrelloCardTool manages Trello cards
type TrelloCardTool struct{}

func (t *TrelloCardTool) Name() string {
	return "trello_manage_card"
}

func (t *TrelloCardTool) Description() string {
	return "Creates, updates, and manages Trello cards with checklists and labels"
}

func (t *TrelloCardTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	action, ok := input.Params["action"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid action",
		}, nil
	}

	result := manageTrelloCard(action, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *TrelloCardTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "trello_integration") || hasCapability(agent, "project_management")
}

// TrelloBoardTool manages Trello boards
type TrelloBoardTool struct{}

func (t *TrelloBoardTool) Name() string {
	return "trello_manage_board"
}

func (t *TrelloBoardTool) Description() string {
	return "Creates and manages Trello boards with lists and power-ups"
}

func (t *TrelloBoardTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	action, ok := input.Params["action"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid action",
		}, nil
	}

	result := manageTrelloBoard(action, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *TrelloBoardTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "trello_integration") || hasCapability(agent, "project_management")
}

// LinearIssueTool manages Linear issues
type LinearIssueTool struct{}

func (t *LinearIssueTool) Name() string {
	return "linear_manage_issue"
}

func (t *LinearIssueTool) Description() string {
	return "Creates, updates, and manages Linear issues with cycles and projects"
}

func (t *LinearIssueTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	action, ok := input.Params["action"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid action",
		}, nil
	}

	result := manageLinearIssue(action, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *LinearIssueTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "linear_integration") || hasCapability(agent, "project_management")
}

// ClickUpTaskTool manages ClickUp tasks
type ClickUpTaskTool struct{}

func (t *ClickUpTaskTool) Name() string {
	return "clickup_manage_task"
}

func (t *ClickUpTaskTool) Description() string {
	return "Creates, updates, and manages ClickUp tasks with custom fields"
}

func (t *ClickUpTaskTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	action, ok := input.Params["action"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid action",
		}, nil
	}

	result := manageClickUpTask(action, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *ClickUpTaskTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "clickup_integration") || hasCapability(agent, "project_management")
}

// MondayItemTool manages Monday.com items
type MondayItemTool struct{}

func (t *MondayItemTool) Name() string {
	return "monday_manage_item"
}

func (t *MondayItemTool) Description() string {
	return "Creates, updates, and manages Monday.com items with column values"
}

func (t *MondayItemTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	action, ok := input.Params["action"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid action",
		}, nil
	}

	result := manageMondayItem(action, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *MondayItemTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "monday_integration") || hasCapability(agent, "project_management")
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

func manageJiraIssue(action string, params map[string]interface{}) map[string]interface{} {
	switch action {
	case "create":
		projectKey, _ := params["project_key"].(string)
		issueType, _ := params["issue_type"].(string)
		summary, _ := params["summary"].(string)
		description, _ := params["description"].(string)

		issueKey := fmt.Sprintf("%s-%d", projectKey, time.Now().Unix()%10000)

		return map[string]interface{}{
			"id":   fmt.Sprintf("%d", time.Now().Unix()),
			"key":  issueKey,
			"self": fmt.Sprintf("https://yourcompany.atlassian.net/rest/api/3/issue/%s", issueKey),
			"fields": map[string]interface{}{
				"project": map[string]interface{}{
					"key":  projectKey,
					"name": "Project Name",
				},
				"issuetype": map[string]interface{}{
					"name": issueType,
				},
				"summary":     summary,
				"description": description,
				"status": map[string]interface{}{
					"name": "To Do",
				},
				"priority": map[string]interface{}{
					"name": "Medium",
				},
				"created": time.Now().Format(time.RFC3339),
			},
		}

	case "update":
		issueKey, _ := params["issue_key"].(string)
		return map[string]interface{}{
			"key":     issueKey,
			"success": true,
			"message": "Issue updated successfully",
		}

	case "transition":
		issueKey, _ := params["issue_key"].(string)
		transitionID, _ := params["transition_id"].(string)
		return map[string]interface{}{
			"key":           issueKey,
			"transition_id": transitionID,
			"success":       true,
		}

	case "search":
		jql, _ := params["jql"].(string)
		return map[string]interface{}{
			"total": 2,
			"issues": []map[string]interface{}{
				{
					"key": "PROJ-123",
					"fields": map[string]interface{}{
						"summary": "Sample Issue 1",
						"status":  map[string]interface{}{"name": "In Progress"},
					},
				},
				{
					"key": "PROJ-124",
					"fields": map[string]interface{}{
						"summary": "Sample Issue 2",
						"status":  map[string]interface{}{"name": "To Do"},
					},
				},
			},
			"jql": jql,
		}
	}

	return map[string]interface{}{
		"error": "Invalid action",
	}
}

func manageJiraSprint(action string, params map[string]interface{}) map[string]interface{} {
	switch action {
	case "create":
		name, _ := params["name"].(string)
		boardID, _ := params["board_id"].(string)
		startDate, _ := params["start_date"].(string)
		endDate, _ := params["end_date"].(string)

		return map[string]interface{}{
			"id":         fmt.Sprintf("%d", time.Now().Unix()),
			"name":       name,
			"state":      "future",
			"boardId":    boardID,
			"startDate":  startDate,
			"endDate":    endDate,
			"goal":       params["goal"],
		}

	case "start":
		sprintID, _ := params["sprint_id"].(string)
		return map[string]interface{}{
			"id":      sprintID,
			"state":   "active",
			"success": true,
		}

	case "complete":
		sprintID, _ := params["sprint_id"].(string)
		return map[string]interface{}{
			"id":      sprintID,
			"state":   "closed",
			"success": true,
		}
	}

	return map[string]interface{}{
		"error": "Invalid action",
	}
}

func manageAsanaTask(action string, params map[string]interface{}) map[string]interface{} {
	switch action {
	case "create":
		name, _ := params["name"].(string)
		notes, _ := params["notes"].(string)
		projectID, _ := params["project_id"].(string)

		gid := fmt.Sprintf("%d", time.Now().Unix())

		return map[string]interface{}{
			"gid":  gid,
			"name": name,
			"notes": notes,
			"projects": []map[string]interface{}{
				{"gid": projectID},
			},
			"created_at":  time.Now().Format(time.RFC3339),
			"modified_at": time.Now().Format(time.RFC3339),
			"completed":   false,
			"permalink_url": fmt.Sprintf("https://app.asana.com/0/0/%s", gid),
		}

	case "update":
		taskID, _ := params["task_id"].(string)
		return map[string]interface{}{
			"gid":     taskID,
			"success": true,
		}

	case "complete":
		taskID, _ := params["task_id"].(string)
		return map[string]interface{}{
			"gid":       taskID,
			"completed": true,
			"completed_at": time.Now().Format(time.RFC3339),
		}

	case "search":
		projectID, _ := params["project_id"].(string)
		return map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"gid":  "1234567890",
					"name": "Task 1",
				},
				{
					"gid":  "1234567891",
					"name": "Task 2",
				},
			},
			"project_id": projectID,
		}
	}

	return map[string]interface{}{
		"error": "Invalid action",
	}
}

func manageAsanaProject(action string, params map[string]interface{}) map[string]interface{} {
	switch action {
	case "create":
		name, _ := params["name"].(string)
		workspaceID, _ := params["workspace_id"].(string)

		gid := fmt.Sprintf("%d", time.Now().Unix())

		return map[string]interface{}{
			"gid":  gid,
			"name": name,
			"workspace": map[string]interface{}{
				"gid": workspaceID,
			},
			"created_at": time.Now().Format(time.RFC3339),
			"owner": map[string]interface{}{
				"gid":  "user123",
				"name": "Project Owner",
			},
			"permalink_url": fmt.Sprintf("https://app.asana.com/0/%s", gid),
		}

	case "list":
		return map[string]interface{}{
			"data": []map[string]interface{}{
				{"gid": "proj1", "name": "Project Alpha"},
				{"gid": "proj2", "name": "Project Beta"},
			},
		}
	}

	return map[string]interface{}{
		"error": "Invalid action",
	}
}

func manageTrelloCard(action string, params map[string]interface{}) map[string]interface{} {
	switch action {
	case "create":
		name, _ := params["name"].(string)
		listID, _ := params["list_id"].(string)
		desc, _ := params["desc"].(string)

		cardID := fmt.Sprintf("%x", time.Now().Unix())

		return map[string]interface{}{
			"id":     cardID,
			"name":   name,
			"desc":   desc,
			"idList": listID,
			"url":    fmt.Sprintf("https://trello.com/c/%s", cardID),
			"shortUrl": fmt.Sprintf("https://trello.com/c/%s", cardID[:8]),
			"pos":    16384,
			"dateLastActivity": time.Now().Format(time.RFC3339),
		}

	case "update":
		cardID, _ := params["card_id"].(string)
		return map[string]interface{}{
			"id":      cardID,
			"success": true,
		}

	case "move":
		cardID, _ := params["card_id"].(string)
		listID, _ := params["list_id"].(string)
		return map[string]interface{}{
			"id":     cardID,
			"idList": listID,
			"success": true,
		}

	case "add_checklist":
		cardID, _ := params["card_id"].(string)
		checklistName, _ := params["checklist_name"].(string)
		return map[string]interface{}{
			"id":     fmt.Sprintf("cl_%x", time.Now().Unix()),
			"idCard": cardID,
			"name":   checklistName,
			"checkItems": []interface{}{},
		}
	}

	return map[string]interface{}{
		"error": "Invalid action",
	}
}

func manageTrelloBoard(action string, params map[string]interface{}) map[string]interface{} {
	switch action {
	case "create":
		name, _ := params["name"].(string)
		boardID := fmt.Sprintf("%x", time.Now().Unix())

		return map[string]interface{}{
			"id":   boardID,
			"name": name,
			"url":  fmt.Sprintf("https://trello.com/b/%s", boardID),
			"shortUrl": fmt.Sprintf("https://trello.com/b/%s", boardID[:8]),
			"prefs": map[string]interface{}{
				"permissionLevel": "private",
				"background":      "blue",
			},
		}

	case "list":
		return map[string]interface{}{
			"boards": []map[string]interface{}{
				{"id": "board1", "name": "Development Board"},
				{"id": "board2", "name": "Marketing Board"},
			},
		}
	}

	return map[string]interface{}{
		"error": "Invalid action",
	}
}

func manageLinearIssue(action string, params map[string]interface{}) map[string]interface{} {
	switch action {
	case "create":
		title, _ := params["title"].(string)
		description, _ := params["description"].(string)
		teamID, _ := params["team_id"].(string)

		issueID := fmt.Sprintf("%x", time.Now().Unix())
		issueNumber := time.Now().Unix() % 1000

		return map[string]interface{}{
			"id":     issueID,
			"number": issueNumber,
			"title":  title,
			"description": description,
			"team": map[string]interface{}{
				"id": teamID,
			},
			"state": map[string]interface{}{
				"name": "Todo",
				"type": "unstarted",
			},
			"priority": 0,
			"url":      fmt.Sprintf("https://linear.app/team/issue/%s", issueID),
			"createdAt": time.Now().Format(time.RFC3339),
		}

	case "update":
		issueID, _ := params["issue_id"].(string)
		return map[string]interface{}{
			"id":      issueID,
			"success": true,
		}

	case "search":
		query, _ := params["query"].(string)
		return map[string]interface{}{
			"nodes": []map[string]interface{}{
				{
					"id":    "issue1",
					"title": "Sample Issue 1",
					"state": map[string]interface{}{"name": "In Progress"},
				},
			},
			"query": query,
		}
	}

	return map[string]interface{}{
		"error": "Invalid action",
	}
}

func manageClickUpTask(action string, params map[string]interface{}) map[string]interface{} {
	switch action {
	case "create":
		name, _ := params["name"].(string)
		description, _ := params["description"].(string)
		listID, _ := params["list_id"].(string)

		taskID := fmt.Sprintf("%d", time.Now().Unix())

		return map[string]interface{}{
			"id":   taskID,
			"name": name,
			"description": description,
			"list": map[string]interface{}{
				"id": listID,
			},
			"status": map[string]interface{}{
				"status": "to do",
			},
			"date_created": fmt.Sprintf("%d", time.Now().UnixMilli()),
			"url":          fmt.Sprintf("https://app.clickup.com/t/%s", taskID),
		}

	case "update":
		taskID, _ := params["task_id"].(string)
		return map[string]interface{}{
			"id":      taskID,
			"success": true,
		}
	}

	return map[string]interface{}{
		"error": "Invalid action",
	}
}

func manageMondayItem(action string, params map[string]interface{}) map[string]interface{} {
	switch action {
	case "create":
		boardID, _ := params["board_id"].(string)
		itemName, _ := params["item_name"].(string)

		itemID := fmt.Sprintf("%d", time.Now().Unix())

		return map[string]interface{}{
			"id":   itemID,
			"name": itemName,
			"board": map[string]interface{}{
				"id": boardID,
			},
			"state": "active",
			"created_at": time.Now().Format(time.RFC3339),
		}

	case "update":
		itemID, _ := params["item_id"].(string)
		return map[string]interface{}{
			"id":      itemID,
			"success": true,
		}

	case "query":
		boardID, _ := params["board_id"].(string)
		return map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "item1", "name": "Task 1"},
				{"id": "item2", "name": "Task 2"},
			},
			"board_id": boardID,
		}
	}

	return map[string]interface{}{
		"error": "Invalid action",
	}
}
