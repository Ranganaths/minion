package visualization

import (
	"context"
	"fmt"

	"github.com/Ranganaths/minion/models"
)

// ChartType represents different types of charts
type ChartType string

const (
	ChartTypeBar    ChartType = "bar"
	ChartTypeLine   ChartType = "line"
	ChartTypePie    ChartType = "pie"
	ChartTypeTable  ChartType = "table"
	ChartTypeScatter ChartType = "scatter"
)

// ChartData represents the structure for chart visualization
type ChartData struct {
	Type        ChartType              `json:"type"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	XAxis       *AxisConfig            `json:"xAxis,omitempty"`
	YAxis       *AxisConfig            `json:"yAxis,omitempty"`
	Series      []SeriesData           `json:"series"`
	Legend      bool                   `json:"legend"`
	Colors      []string               `json:"colors,omitempty"`
	Options     map[string]interface{} `json:"options,omitempty"`
}

// AxisConfig configures chart axes
type AxisConfig struct {
	Label    string   `json:"label"`
	Type     string   `json:"type"` // "category", "value", "time"
	Data     []string `json:"data,omitempty"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	Format   string   `json:"format,omitempty"` // e.g., "currency", "percent"
}

// SeriesData represents a data series in a chart
type SeriesData struct {
	Name   string        `json:"name"`
	Type   string        `json:"type,omitempty"` // Can override chart type per series
	Data   []interface{} `json:"data"`
	Color  string        `json:"color,omitempty"`
	Stack  string        `json:"stack,omitempty"` // For stacked charts
}

// BarChartTool creates bar charts from data
type BarChartTool struct{}

func (t *BarChartTool) Name() string {
	return "bar_chart_visualizer"
}

func (t *BarChartTool) Description() string {
	return "Creates bar charts for comparing values across categories. Ideal for sales by region, product comparisons, monthly revenue, etc."
}

func (t *BarChartTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	// Extract parameters
	title, _ := input.Params["title"].(string)
	if title == "" {
		title = "Bar Chart"
	}

	description, _ := input.Params["description"].(string)
	categories, _ := input.Params["categories"].([]interface{})
	series, _ := input.Params["series"].([]interface{})

	xAxisLabel, _ := input.Params["x_axis_label"].(string)
	yAxisLabel, _ := input.Params["y_axis_label"].(string)
	stacked, _ := input.Params["stacked"].(bool)

	if len(categories) == 0 {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Error:    "categories parameter is required",
		}, nil
	}

	// Convert categories to strings
	categoryStrings := make([]string, len(categories))
	for i, cat := range categories {
		categoryStrings[i] = fmt.Sprintf("%v", cat)
	}

	// Build series data
	seriesData := make([]SeriesData, 0)
	if len(series) > 0 {
		for _, s := range series {
			if seriesMap, ok := s.(map[string]interface{}); ok {
				name, _ := seriesMap["name"].(string)
				data, _ := seriesMap["data"].([]interface{})
				color, _ := seriesMap["color"].(string)

				sd := SeriesData{
					Name:  name,
					Type:  string(ChartTypeBar),
					Data:  data,
					Color: color,
				}
				if stacked {
					sd.Stack = "total"
				}
				seriesData = append(seriesData, sd)
			}
		}
	}

	chartData := &ChartData{
		Type:        ChartTypeBar,
		Title:       title,
		Description: description,
		XAxis: &AxisConfig{
			Label: xAxisLabel,
			Type:  "category",
			Data:  categoryStrings,
		},
		YAxis: &AxisConfig{
			Label: yAxisLabel,
			Type:  "value",
		},
		Series: seriesData,
		Legend: len(seriesData) > 1,
		Options: map[string]interface{}{
			"stacked": stacked,
		},
	}

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   chartData,
		Metadata: map[string]interface{}{
			"chart_type":     "bar",
			"category_count": len(categories),
			"series_count":   len(seriesData),
		},
	}, nil
}

func (t *BarChartTool) CanExecute(agent *models.Agent) bool {
	return containsCapability(agent.Capabilities, "visualization")
}

// LineChartTool creates line charts for trends over time
type LineChartTool struct{}

func (t *LineChartTool) Name() string {
	return "line_chart_visualizer"
}

func (t *LineChartTool) Description() string {
	return "Creates line charts for showing trends over time. Perfect for revenue trends, sales growth, KPI tracking, etc."
}

func (t *LineChartTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	// Extract parameters
	title, _ := input.Params["title"].(string)
	if title == "" {
		title = "Line Chart"
	}

	description, _ := input.Params["description"].(string)
	xData, _ := input.Params["x_data"].([]interface{})
	series, _ := input.Params["series"].([]interface{})

	xAxisLabel, _ := input.Params["x_axis_label"].(string)
	yAxisLabel, _ := input.Params["y_axis_label"].(string)
	smooth, _ := input.Params["smooth"].(bool)

	if len(xData) == 0 {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Error:    "x_data parameter is required",
		}, nil
	}

	// Convert x_data to strings
	xDataStrings := make([]string, len(xData))
	for i, x := range xData {
		xDataStrings[i] = fmt.Sprintf("%v", x)
	}

	// Build series data
	seriesData := make([]SeriesData, 0)
	if len(series) > 0 {
		for _, s := range series {
			if seriesMap, ok := s.(map[string]interface{}); ok {
				name, _ := seriesMap["name"].(string)
				data, _ := seriesMap["data"].([]interface{})
				color, _ := seriesMap["color"].(string)

				seriesData = append(seriesData, SeriesData{
					Name:  name,
					Type:  string(ChartTypeLine),
					Data:  data,
					Color: color,
				})
			}
		}
	}

	chartData := &ChartData{
		Type:        ChartTypeLine,
		Title:       title,
		Description: description,
		XAxis: &AxisConfig{
			Label: xAxisLabel,
			Type:  "category",
			Data:  xDataStrings,
		},
		YAxis: &AxisConfig{
			Label: yAxisLabel,
			Type:  "value",
		},
		Series: seriesData,
		Legend: len(seriesData) > 1,
		Options: map[string]interface{}{
			"smooth": smooth,
		},
	}

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   chartData,
		Metadata: map[string]interface{}{
			"chart_type":   "line",
			"point_count":  len(xData),
			"series_count": len(seriesData),
		},
	}, nil
}

func (t *LineChartTool) CanExecute(agent *models.Agent) bool {
	return containsCapability(agent.Capabilities, "visualization")
}

// PieChartTool creates pie charts for showing proportions
type PieChartTool struct{}

func (t *PieChartTool) Name() string {
	return "pie_chart_visualizer"
}

func (t *PieChartTool) Description() string {
	return "Creates pie charts for showing proportions and distributions. Great for market share, revenue by product, customer segments, etc."
}

func (t *PieChartTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	// Extract parameters
	title, _ := input.Params["title"].(string)
	if title == "" {
		title = "Pie Chart"
	}

	description, _ := input.Params["description"].(string)
	data, _ := input.Params["data"].([]interface{})
	showPercentage, _ := input.Params["show_percentage"].(bool)

	if len(data) == 0 {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Error:    "data parameter is required (array of {name, value} objects)",
		}, nil
	}

	// Convert data to proper format
	pieData := make([]interface{}, 0)
	total := 0.0
	for _, d := range data {
		if dataMap, ok := d.(map[string]interface{}); ok {
			name, _ := dataMap["name"].(string)
			value, _ := dataMap["value"].(float64)
			total += value

			pieData = append(pieData, map[string]interface{}{
				"name":  name,
				"value": value,
			})
		}
	}

	// Calculate percentages if requested
	if showPercentage && total > 0 {
		for i, d := range pieData {
			if dataMap, ok := d.(map[string]interface{}); ok {
				value, _ := dataMap["value"].(float64)
				percentage := (value / total) * 100
				dataMap["percentage"] = fmt.Sprintf("%.1f%%", percentage)
				pieData[i] = dataMap
			}
		}
	}

	chartData := &ChartData{
		Type:        ChartTypePie,
		Title:       title,
		Description: description,
		Series: []SeriesData{
			{
				Name: title,
				Type: string(ChartTypePie),
				Data: pieData,
			},
		},
		Legend: true,
		Options: map[string]interface{}{
			"show_percentage": showPercentage,
		},
	}

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   chartData,
		Metadata: map[string]interface{}{
			"chart_type":  "pie",
			"slice_count": len(pieData),
			"total_value": total,
		},
	}, nil
}

func (t *PieChartTool) CanExecute(agent *models.Agent) bool {
	return containsCapability(agent.Capabilities, "visualization")
}

// TableVisualizerTool creates formatted tables from data
type TableVisualizerTool struct{}

func (t *TableVisualizerTool) Name() string {
	return "table_visualizer"
}

func (t *TableVisualizerTool) Description() string {
	return "Creates formatted tables for displaying detailed data. Supports sorting, filtering, and highlighting."
}

func (t *TableVisualizerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	// Extract parameters
	title, _ := input.Params["title"].(string)
	if title == "" {
		title = "Data Table"
	}

	description, _ := input.Params["description"].(string)
	columns, _ := input.Params["columns"].([]interface{})
	rows, _ := input.Params["rows"].([]interface{})
	sortable, _ := input.Params["sortable"].(bool)
	filterable, _ := input.Params["filterable"].(bool)

	if len(columns) == 0 || len(rows) == 0 {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Error:    "columns and rows parameters are required",
		}, nil
	}

	tableData := map[string]interface{}{
		"title":       title,
		"description": description,
		"columns":     columns,
		"rows":        rows,
		"sortable":    sortable,
		"filterable":  filterable,
	}

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   tableData,
		Metadata: map[string]interface{}{
			"chart_type":   "table",
			"column_count": len(columns),
			"row_count":    len(rows),
		},
	}, nil
}

func (t *TableVisualizerTool) CanExecute(agent *models.Agent) bool {
	return containsCapability(agent.Capabilities, "visualization")
}

// Helper function
func containsCapability(capabilities []string, capability string) bool {
	for _, c := range capabilities {
		if c == capability {
			return true
		}
	}
	return false
}
