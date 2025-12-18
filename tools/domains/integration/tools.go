package integration

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Ranganaths/minion/models"
)

// APICallerTool calls external APIs and parses responses
type APICallerTool struct{}

func (t *APICallerTool) Name() string {
	return "api_caller"
}

func (t *APICallerTool) Description() string {
	return "Calls external APIs with authentication and response parsing capabilities"
}

func (t *APICallerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	url, ok := input.Params["url"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid URL",
		}, nil
	}

	method := "GET"
	if m, ok := input.Params["method"].(string); ok {
		method = m
	}

	headers, _ := input.Params["headers"].(map[string]string)
	body, _ := input.Params["body"].(string)

	result := callAPI(url, method, headers, body)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *APICallerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "api_integration") || hasCapability(agent, "integration")
}

// FileParserTool parses CSV, JSON, and Excel files
type FileParserTool struct{}

func (t *FileParserTool) Name() string {
	return "file_parser"
}

func (t *FileParserTool) Description() string {
	return "Parses CSV, JSON, and Excel files into structured data"
}

func (t *FileParserTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	content, ok := input.Params["content"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid file content",
		}, nil
	}

	fileType := "auto"
	if ft, ok := input.Params["file_type"].(string); ok {
		fileType = ft
	}

	parsed := parseFile(content, fileType)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   parsed,
	}, nil
}

func (t *FileParserTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "file_parsing") || hasCapability(agent, "integration")
}

// WebScraperTool extracts data from websites
type WebScraperTool struct{}

func (t *WebScraperTool) Name() string {
	return "web_scraper"
}

func (t *WebScraperTool) Description() string {
	return "Extracts structured data from websites using selectors and patterns"
}

func (t *WebScraperTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	url, ok := input.Params["url"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid URL",
		}, nil
	}

	selectors, _ := input.Params["selectors"].(map[string]string)

	scraped := scrapeWebsite(url, selectors)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   scraped,
	}, nil
}

func (t *WebScraperTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "web_scraping") || hasCapability(agent, "integration")
}

// DatabaseConnectorTool queries external databases
type DatabaseConnectorTool struct{}

func (t *DatabaseConnectorTool) Name() string {
	return "database_connector"
}

func (t *DatabaseConnectorTool) Description() string {
	return "Connects to and queries external databases (PostgreSQL, MySQL, MongoDB)"
}

func (t *DatabaseConnectorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	connectionString, ok := input.Params["connection_string"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid connection string",
		}, nil
	}

	query, ok := input.Params["query"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid query",
		}, nil
	}

	dbType := "postgres"
	if dt, ok := input.Params["db_type"].(string); ok {
		dbType = dt
	}

	result := queryDatabase(connectionString, query, dbType)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *DatabaseConnectorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "database_access") || hasCapability(agent, "integration")
}

// DataSyncTool syncs data between systems
type DataSyncTool struct{}

func (t *DataSyncTool) Name() string {
	return "data_sync"
}

func (t *DataSyncTool) Description() string {
	return "Synchronizes data between different systems with conflict resolution"
}

func (t *DataSyncTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	sourceData, ok := input.Params["source_data"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid source data",
		}, nil
	}

	targetData, ok := input.Params["target_data"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid target data",
		}, nil
	}

	syncStrategy := "merge"
	if ss, ok := input.Params["sync_strategy"].(string); ok {
		syncStrategy = ss
	}

	syncResult := syncData(sourceData, targetData, syncStrategy)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   syncResult,
	}, nil
}

func (t *DataSyncTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "data_sync") || hasCapability(agent, "integration")
}

// WebhookHandlerTool processes incoming webhooks
type WebhookHandlerTool struct{}

func (t *WebhookHandlerTool) Name() string {
	return "webhook_handler"
}

func (t *WebhookHandlerTool) Description() string {
	return "Processes and validates incoming webhook payloads"
}

func (t *WebhookHandlerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	payload, ok := input.Params["payload"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid webhook payload",
		}, nil
	}

	signature, _ := input.Params["signature"].(string)
	secret, _ := input.Params["secret"].(string)

	processed := processWebhook(payload, signature, secret)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   processed,
	}, nil
}

func (t *WebhookHandlerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "webhook_processing") || hasCapability(agent, "integration")
}

// EmailSenderTool sends emails via SMTP
type EmailSenderTool struct{}

func (t *EmailSenderTool) Name() string {
	return "email_sender"
}

func (t *EmailSenderTool) Description() string {
	return "Sends emails via SMTP with template support and attachments"
}

func (t *EmailSenderTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	to, ok := input.Params["to"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid recipient email",
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

	result := sendEmail(to, subject, body)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *EmailSenderTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "email_sending") || hasCapability(agent, "integration")
}

// SlackNotifierTool sends notifications to Slack
type SlackNotifierTool struct{}

func (t *SlackNotifierTool) Name() string {
	return "slack_notifier"
}

func (t *SlackNotifierTool) Description() string {
	return "Sends formatted messages and notifications to Slack channels"
}

func (t *SlackNotifierTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
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

	result := sendSlackMessage(channel, message)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *SlackNotifierTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "slack_integration") || hasCapability(agent, "integration")
}

// CloudStorageTool interacts with cloud storage (S3, GCS, Azure)
type CloudStorageTool struct{}

func (t *CloudStorageTool) Name() string {
	return "cloud_storage"
}

func (t *CloudStorageTool) Description() string {
	return "Uploads, downloads, and manages files in cloud storage (S3, GCS, Azure Blob)"
}

func (t *CloudStorageTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	operation, ok := input.Params["operation"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid operation",
		}, nil
	}

	provider := "s3"
	if p, ok := input.Params["provider"].(string); ok {
		provider = p
	}

	result := performCloudStorageOperation(operation, provider, input.Params)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *CloudStorageTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "cloud_storage") || hasCapability(agent, "integration")
}

// DataExporterTool exports data to various formats
type DataExporterTool struct{}

func (t *DataExporterTool) Name() string {
	return "data_exporter"
}

func (t *DataExporterTool) Description() string {
	return "Exports data to CSV, JSON, Excel, PDF, and other formats"
}

func (t *DataExporterTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	data, ok := input.Params["data"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid data",
		}, nil
	}

	format := "csv"
	if f, ok := input.Params["format"].(string); ok {
		format = f
	}

	exported := exportData(data, format)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   exported,
	}, nil
}

func (t *DataExporterTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "data_export") || hasCapability(agent, "integration")
}

// EventStreamProcessorTool processes event streams (Kafka, RabbitMQ)
type EventStreamProcessorTool struct{}

func (t *EventStreamProcessorTool) Name() string {
	return "event_stream_processor"
}

func (t *EventStreamProcessorTool) Description() string {
	return "Processes and transforms event streams from Kafka, RabbitMQ, or other message queues"
}

func (t *EventStreamProcessorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	events, ok := input.Params["events"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid events data",
		}, nil
	}

	processor := "filter"
	if p, ok := input.Params["processor"].(string); ok {
		processor = p
	}

	processed := processEventStream(events, processor)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   processed,
	}, nil
}

func (t *EventStreamProcessorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "event_processing") || hasCapability(agent, "integration")
}

// OAuthAuthenticatorTool handles OAuth authentication flows
type OAuthAuthenticatorTool struct{}

func (t *OAuthAuthenticatorTool) Name() string {
	return "oauth_authenticator"
}

func (t *OAuthAuthenticatorTool) Description() string {
	return "Handles OAuth 2.0 authentication flows for third-party integrations"
}

func (t *OAuthAuthenticatorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	provider, ok := input.Params["provider"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid provider",
		}, nil
	}

	clientID, _ := input.Params["client_id"].(string)
	clientSecret, _ := input.Params["client_secret"].(string)

	result := performOAuthFlow(provider, clientID, clientSecret)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *OAuthAuthenticatorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "oauth_authentication") || hasCapability(agent, "integration")
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

func callAPI(url, method string, headers map[string]string, body string) map[string]interface{} {
	// Simulated API call
	return map[string]interface{}{
		"status_code": 200,
		"headers": map[string]string{
			"Content-Type": "application/json",
		},
		"body": map[string]interface{}{
			"success": true,
			"data":    "API response data",
			"message": "Request successful",
		},
		"request_info": map[string]interface{}{
			"url":    url,
			"method": method,
			"timestamp": time.Now().Format(time.RFC3339),
		},
	}
}

func parseFile(content, fileType string) map[string]interface{} {
	if fileType == "auto" {
		fileType = detectFileType(content)
	}

	var parsed interface{}
	var err error

	switch fileType {
	case "json":
		parsed, err = parseJSON(content)
	case "csv":
		parsed, err = parseCSV(content)
	default:
		return map[string]interface{}{
			"error": fmt.Sprintf("Unsupported file type: %s", fileType),
		}
	}

	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return map[string]interface{}{
		"file_type":    fileType,
		"parsed_data":  parsed,
		"record_count": getRecordCount(parsed),
	}
}

func detectFileType(content string) string {
	content = strings.TrimSpace(content)

	if strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[") {
		return "json"
	}

	// Simple CSV detection
	if strings.Contains(content, ",") && strings.Contains(content, "\n") {
		return "csv"
	}

	return "unknown"
}

func parseJSON(content string) (interface{}, error) {
	var data interface{}
	err := json.Unmarshal([]byte(content), &data)
	return data, err
}

func parseCSV(content string) ([]map[string]interface{}, error) {
	reader := csv.NewReader(strings.NewReader(content))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return []map[string]interface{}{}, nil
	}

	// First row as headers
	headers := records[0]
	result := []map[string]interface{}{}

	for i := 1; i < len(records); i++ {
		row := make(map[string]interface{})
		for j, value := range records[i] {
			if j < len(headers) {
				row[headers[j]] = value
			}
		}
		result = append(result, row)
	}

	return result, nil
}

func getRecordCount(data interface{}) int {
	switch v := data.(type) {
	case []interface{}:
		return len(v)
	case []map[string]interface{}:
		return len(v)
	case map[string]interface{}:
		return 1
	default:
		return 0
	}
}

func scrapeWebsite(url string, selectors map[string]string) map[string]interface{} {
	// Simulated web scraping
	scraped := map[string]interface{}{}

	for key := range selectors {
		scraped[key] = fmt.Sprintf("Scraped content for %s", key)
	}

	return map[string]interface{}{
		"url":           url,
		"scraped_data":  scraped,
		"scrape_time":   time.Now().Format(time.RFC3339),
		"success":       true,
	}
}

func queryDatabase(connectionString, query, dbType string) map[string]interface{} {
	// Simulated database query
	mockResults := []map[string]interface{}{
		{
			"id":   1,
			"name": "Record 1",
			"value": 100,
		},
		{
			"id":   2,
			"name": "Record 2",
			"value": 200,
		},
	}

	return map[string]interface{}{
		"success":      true,
		"rows_returned": len(mockResults),
		"results":      mockResults,
		"query":        query,
		"db_type":      dbType,
		"execution_time_ms": 45,
	}
}

func syncData(sourceData, targetData []map[string]interface{}, strategy string) map[string]interface{} {
	added := 0
	updated := 0
	deleted := 0
	conflicts := []map[string]interface{}{}

	// Create lookup map for target data
	targetMap := make(map[string]map[string]interface{})
	for _, record := range targetData {
		if id, ok := record["id"].(string); ok {
			targetMap[id] = record
		}
	}

	// Process source data
	for _, sourceRecord := range sourceData {
		id, ok := sourceRecord["id"].(string)
		if !ok {
			continue
		}

		if targetRecord, exists := targetMap[id]; exists {
			// Check for conflicts
			if hasConflict(sourceRecord, targetRecord) {
				conflicts = append(conflicts, map[string]interface{}{
					"id":     id,
					"source": sourceRecord,
					"target": targetRecord,
				})
			} else {
				updated++
			}
		} else {
			added++
		}
	}

	return map[string]interface{}{
		"strategy":       strategy,
		"records_added":  added,
		"records_updated": updated,
		"records_deleted": deleted,
		"conflicts":      conflicts,
		"conflict_count": len(conflicts),
		"sync_timestamp": time.Now().Format(time.RFC3339),
		"status":         "completed",
	}
}

func hasConflict(source, target map[string]interface{}) bool {
	// Simple conflict detection
	sourceUpdated, sourceOk := source["updated_at"].(string)
	targetUpdated, targetOk := target["updated_at"].(string)

	if sourceOk && targetOk {
		return sourceUpdated != targetUpdated
	}

	return false
}

func processWebhook(payload map[string]interface{}, signature, secret string) map[string]interface{} {
	// Simulated webhook processing
	valid := validateWebhookSignature(signature, secret)

	eventType, _ := payload["event_type"].(string)

	return map[string]interface{}{
		"valid":      valid,
		"event_type": eventType,
		"payload":    payload,
		"processed_at": time.Now().Format(time.RFC3339),
		"actions_taken": []string{
			"Validated signature",
			"Parsed payload",
			"Triggered event handler",
		},
	}
}

func validateWebhookSignature(signature, secret string) bool {
	// Simulated signature validation
	return signature != "" && secret != ""
}

func sendEmail(to, subject, body string) map[string]interface{} {
	// Simulated email sending
	return map[string]interface{}{
		"success":    true,
		"message_id": fmt.Sprintf("msg_%d", time.Now().Unix()),
		"to":         to,
		"subject":    subject,
		"sent_at":    time.Now().Format(time.RFC3339),
		"status":     "delivered",
	}
}

func sendSlackMessage(channel, message string) map[string]interface{} {
	// Simulated Slack message sending
	return map[string]interface{}{
		"success":   true,
		"channel":   channel,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
		"message_id": fmt.Sprintf("slack_%d", time.Now().Unix()),
	}
}

func performCloudStorageOperation(operation, provider string, params map[string]interface{}) map[string]interface{} {
	switch operation {
	case "upload":
		fileName, _ := params["file_name"].(string)
		return map[string]interface{}{
			"success":   true,
			"operation": "upload",
			"provider":  provider,
			"file_name": fileName,
			"url":       fmt.Sprintf("https://%s.example.com/%s", provider, fileName),
			"size":      1024,
		}

	case "download":
		fileName, _ := params["file_name"].(string)
		return map[string]interface{}{
			"success":   true,
			"operation": "download",
			"provider":  provider,
			"file_name": fileName,
			"content":   "File content here",
		}

	case "list":
		return map[string]interface{}{
			"success":   true,
			"operation": "list",
			"provider":  provider,
			"files": []string{
				"file1.txt",
				"file2.csv",
				"document.pdf",
			},
		}

	case "delete":
		fileName, _ := params["file_name"].(string)
		return map[string]interface{}{
			"success":   true,
			"operation": "delete",
			"provider":  provider,
			"file_name": fileName,
			"deleted_at": time.Now().Format(time.RFC3339),
		}

	default:
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Unknown operation: %s", operation),
		}
	}
}

func exportData(data []map[string]interface{}, format string) map[string]interface{} {
	var exported string

	switch format {
	case "csv":
		exported = exportToCSV(data)
	case "json":
		exported = exportToJSON(data)
	case "xml":
		exported = exportToXML(data)
	default:
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Unsupported format: %s", format),
		}
	}

	return map[string]interface{}{
		"success":      true,
		"format":       format,
		"content":      exported,
		"record_count": len(data),
		"exported_at":  time.Now().Format(time.RFC3339),
	}
}

func exportToCSV(data []map[string]interface{}) string {
	if len(data) == 0 {
		return ""
	}

	// Get headers from first record
	var headers []string
	for key := range data[0] {
		headers = append(headers, key)
	}

	var builder strings.Builder

	// Write headers
	builder.WriteString(strings.Join(headers, ","))
	builder.WriteString("\n")

	// Write data
	for _, record := range data {
		var values []string
		for _, header := range headers {
			value := fmt.Sprintf("%v", record[header])
			values = append(values, value)
		}
		builder.WriteString(strings.Join(values, ","))
		builder.WriteString("\n")
	}

	return builder.String()
}

func exportToJSON(data []map[string]interface{}) string {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func exportToXML(data []map[string]interface{}) string {
	var builder strings.Builder
	builder.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	builder.WriteString("<records>\n")

	for _, record := range data {
		builder.WriteString("  <record>\n")
		for key, value := range record {
			builder.WriteString(fmt.Sprintf("    <%s>%v</%s>\n", key, value, key))
		}
		builder.WriteString("  </record>\n")
	}

	builder.WriteString("</records>")
	return builder.String()
}

func processEventStream(events []map[string]interface{}, processor string) map[string]interface{} {
	processedEvents := []map[string]interface{}{}

	for _, event := range events {
		processed := processEvent(event, processor)
		if processed != nil {
			processedEvents = append(processedEvents, processed)
		}
	}

	return map[string]interface{}{
		"processor":        processor,
		"events_received":  len(events),
		"events_processed": len(processedEvents),
		"events_filtered":  len(events) - len(processedEvents),
		"processed_events": processedEvents,
		"processing_time_ms": 123,
	}
}

func processEvent(event map[string]interface{}, processor string) map[string]interface{} {
	switch processor {
	case "filter":
		// Filter out events without required fields
		if _, hasType := event["event_type"]; !hasType {
			return nil
		}
		return event

	case "transform":
		// Add processing metadata
		event["processed_at"] = time.Now().Format(time.RFC3339)
		event["processor"] = processor
		return event

	case "enrich":
		// Enrich with additional data
		event["enriched"] = true
		event["enrichment_timestamp"] = time.Now().Format(time.RFC3339)
		return event

	default:
		return event
	}
}

func performOAuthFlow(provider, clientID, clientSecret string) map[string]interface{} {
	// Simulated OAuth flow
	return map[string]interface{}{
		"success":       true,
		"provider":      provider,
		"access_token":  "mock_access_token_" + provider,
		"refresh_token": "mock_refresh_token_" + provider,
		"expires_in":    3600,
		"token_type":    "Bearer",
		"scope":         "read write",
		"obtained_at":   time.Now().Format(time.RFC3339),
	}
}
