package agentql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/agentql/agentql/pkg/minion/models"
)

// SQLExecutorTool executes SQL queries against the semantic layer
type SQLExecutorTool struct {
	db *sql.DB
}

// NewSQLExecutorTool creates a new SQL executor tool
func NewSQLExecutorTool(db *sql.DB) *SQLExecutorTool {
	return &SQLExecutorTool{
		db: db,
	}
}

func (t *SQLExecutorTool) Name() string {
	return "sql_executor"
}

func (t *SQLExecutorTool) Description() string {
	return "Executes SQL queries against the AgentQL semantic layer and returns structured results"
}

func (t *SQLExecutorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	startTime := time.Now()

	// Extract SQL query
	sqlQuery, ok := input.Params["sql"].(string)
	if !ok || sqlQuery == "" {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Error:    "sql parameter is required",
		}, nil
	}

	// Get optional parameters
	maxRows := 1000
	if mr, ok := input.Params["max_rows"].(float64); ok {
		maxRows = int(mr)
	}

	timeout := 30 * time.Second
	if to, ok := input.Params["timeout"].(float64); ok {
		timeout = time.Duration(to) * time.Millisecond
	}

	// Create context with timeout
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute query
	rows, err := t.db.QueryContext(queryCtx, sqlQuery)
	if err != nil {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Error:    fmt.Sprintf("query execution failed: %v", err),
			Metadata: map[string]interface{}{
				"sql":            sqlQuery,
				"execution_time": time.Since(startTime).Milliseconds(),
			},
		}, nil
	}
	defer rows.Close()

	// Get column information
	columns, err := rows.Columns()
	if err != nil {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Error:    fmt.Sprintf("failed to get columns: %v", err),
		}, nil
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Error:    fmt.Sprintf("failed to get column types: %v", err),
		}, nil
	}

	// Build column info
	columnInfo := make([]map[string]interface{}, len(columns))
	for i, col := range columns {
		colType := columnTypes[i]
		columnInfo[i] = map[string]interface{}{
			"name":     col,
			"type":     colType.DatabaseTypeName(),
			"nullable": func() bool { n, ok := colType.Nullable(); return ok && n }(),
		}
	}

	// Scan results
	results := make([]map[string]interface{}, 0)
	rowCount := 0

	for rows.Next() && rowCount < maxRows {
		// Create a slice of interface{} to hold each column value
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return &models.ToolOutput{
				ToolName: t.Name(),
				Success:  false,
				Error:    fmt.Sprintf("failed to scan row: %v", err),
			}, nil
		}

		// Create a map for this row
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]

			// Convert []byte to string
			if b, ok := val.([]byte); ok {
				val = string(b)
			}

			rowMap[col] = val
		}

		results = append(results, rowMap)
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Error:    fmt.Sprintf("error iterating rows: %v", err),
		}, nil
	}

	executionTime := time.Since(startTime)

	// Build result
	result := map[string]interface{}{
		"columns":     columnInfo,
		"rows":        results,
		"row_count":   rowCount,
		"truncated":   rowCount >= maxRows,
		"sql":         sqlQuery,
		"executed_at": startTime.Format(time.RFC3339),
	}

	return &models.ToolOutput{
		ToolName:      t.Name(),
		Success:       true,
		Result:        result,
		ExecutionTime: executionTime.Milliseconds(),
		Metadata: map[string]interface{}{
			"row_count":      rowCount,
			"column_count":   len(columns),
			"execution_time": executionTime.Milliseconds(),
			"truncated":      rowCount >= maxRows,
		},
	}, nil
}

func (t *SQLExecutorTool) CanExecute(agent *models.Agent) bool {
	// Check if agent has SQL execution capability
	for _, cap := range agent.Capabilities {
		if cap == "sql_execution" || cap == "query_execution" {
			return true
		}
	}
	return false
}

// SQLGeneratorTool generates SQL queries from natural language using AgentQL behaviors
type SQLGeneratorTool struct {
	// This tool works with the AgentQL adapter to generate SQL
}

func (t *SQLGeneratorTool) Name() string {
	return "sql_generator"
}

func (t *SQLGeneratorTool) Description() string {
	return "Generates SQL queries from natural language requests using domain-specific knowledge"
}

func (t *SQLGeneratorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	// Extract natural language query
	nlQuery, ok := input.Params["query"].(string)
	if !ok || nlQuery == "" {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Error:    "query parameter is required",
		}, nil
	}

	// Extract schema context if provided
	schema, _ := input.Params["schema"].(map[string]interface{})
	domain, _ := input.Params["domain"].(string)

	// For now, this returns a placeholder
	// In real implementation, this would use the AgentQL SQL generation behaviors
	result := map[string]interface{}{
		"natural_language": nlQuery,
		"sql":              "", // Would be generated by behavior
		"explanation":      fmt.Sprintf("SQL query for: %s", nlQuery),
		"domain":           domain,
		"schema_context":   schema,
		"confidence":       0.85,
	}

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
		Metadata: map[string]interface{}{
			"domain": domain,
		},
	}, nil
}

func (t *SQLGeneratorTool) CanExecute(agent *models.Agent) bool {
	// Check if agent has SQL generation capability
	for _, cap := range agent.Capabilities {
		if cap == "sql_generation" || cap == "query_generation" {
			return true
		}
	}
	return false
}
