package observability

import (
	"encoding/json"
)

// DashboardConfig represents a Grafana dashboard configuration
type DashboardConfig struct {
	ID            int              `json:"id,omitempty"`
	UID           string           `json:"uid"`
	Title         string           `json:"title"`
	Description   string           `json:"description,omitempty"`
	Tags          []string         `json:"tags,omitempty"`
	Timezone      string           `json:"timezone"`
	SchemaVersion int              `json:"schemaVersion"`
	Version       int              `json:"version"`
	Refresh       string           `json:"refresh,omitempty"`
	Time          DashboardTime    `json:"time"`
	Panels        []Panel          `json:"panels"`
	Templating    TemplatingConfig `json:"templating,omitempty"`
	Annotations   AnnotationConfig `json:"annotations,omitempty"`
}

// DashboardTime represents dashboard time range
type DashboardTime struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Panel represents a Grafana panel
type Panel struct {
	ID           int              `json:"id"`
	Title        string           `json:"title"`
	Description  string           `json:"description,omitempty"`
	Type         string           `json:"type"` // graph, stat, gauge, table, etc.
	GridPos      GridPos          `json:"gridPos"`
	Targets      []Target         `json:"targets,omitempty"`
	Options      map[string]interface{} `json:"options,omitempty"`
	FieldConfig  *FieldConfig     `json:"fieldConfig,omitempty"`
	Datasource   *Datasource      `json:"datasource,omitempty"`
}

// GridPos represents panel position and size
type GridPos struct {
	H int `json:"h"` // Height
	W int `json:"w"` // Width
	X int `json:"x"` // X position
	Y int `json:"y"` // Y position
}

// Target represents a panel data source query
type Target struct {
	Expr         string `json:"expr"`
	RefID        string `json:"refId"`
	LegendFormat string `json:"legendFormat,omitempty"`
	Interval     string `json:"interval,omitempty"`
	Step         int    `json:"step,omitempty"`
}

// FieldConfig represents field configuration
type FieldConfig struct {
	Defaults  FieldDefaults   `json:"defaults"`
	Overrides []FieldOverride `json:"overrides,omitempty"`
}

// FieldDefaults represents default field settings
type FieldDefaults struct {
	Color     map[string]interface{} `json:"color,omitempty"`
	Custom    map[string]interface{} `json:"custom,omitempty"`
	Mappings  []interface{}          `json:"mappings,omitempty"`
	Thresholds *ThresholdConfig      `json:"thresholds,omitempty"`
	Unit      string                 `json:"unit,omitempty"`
	Min       *float64               `json:"min,omitempty"`
	Max       *float64               `json:"max,omitempty"`
}

// ThresholdConfig represents threshold configuration
type ThresholdConfig struct {
	Mode  string            `json:"mode"`
	Steps []ThresholdStep   `json:"steps"`
}

// ThresholdStep represents a threshold step
type ThresholdStep struct {
	Color string   `json:"color"`
	Value *float64 `json:"value"`
}

// FieldOverride represents a field override
type FieldOverride struct {
	Matcher    map[string]interface{} `json:"matcher"`
	Properties []interface{}          `json:"properties"`
}

// Datasource represents a data source reference
type Datasource struct {
	Type string `json:"type"`
	UID  string `json:"uid,omitempty"`
}

// TemplatingConfig represents dashboard templating
type TemplatingConfig struct {
	List []TemplateVariable `json:"list,omitempty"`
}

// TemplateVariable represents a template variable
type TemplateVariable struct {
	Name        string      `json:"name"`
	Label       string      `json:"label,omitempty"`
	Type        string      `json:"type"` // query, custom, constant, datasource
	Query       interface{} `json:"query,omitempty"`
	Current     interface{} `json:"current,omitempty"`
	Hide        int         `json:"hide"`
	IncludeAll  bool        `json:"includeAll"`
	Multi       bool        `json:"multi"`
	Options     []interface{} `json:"options,omitempty"`
	Refresh     int         `json:"refresh"`
	Datasource  *Datasource `json:"datasource,omitempty"`
}

// AnnotationConfig represents annotation configuration
type AnnotationConfig struct {
	List []AnnotationQuery `json:"list,omitempty"`
}

// AnnotationQuery represents an annotation query
type AnnotationQuery struct {
	Name       string      `json:"name"`
	Enable     bool        `json:"enable"`
	Hide       bool        `json:"hide"`
	Datasource *Datasource `json:"datasource,omitempty"`
	Expr       string      `json:"expr,omitempty"`
}

// DashboardBuilder builds Grafana dashboards programmatically
type DashboardBuilder struct {
	dashboard *DashboardConfig
	panelID   int
	currentY  int
}

// NewDashboardBuilder creates a new dashboard builder
func NewDashboardBuilder(title, uid string) *DashboardBuilder {
	return &DashboardBuilder{
		dashboard: &DashboardConfig{
			UID:           uid,
			Title:         title,
			Timezone:      "browser",
			SchemaVersion: 36,
			Version:       1,
			Refresh:       "5s",
			Time: DashboardTime{
				From: "now-6h",
				To:   "now",
			},
			Panels: make([]Panel, 0),
			Tags:   []string{"minion", "agents"},
		},
		panelID:  1,
		currentY: 0,
	}
}

// SetDescription sets the dashboard description
func (b *DashboardBuilder) SetDescription(desc string) *DashboardBuilder {
	b.dashboard.Description = desc
	return b
}

// AddTags adds tags to the dashboard
func (b *DashboardBuilder) AddTags(tags ...string) *DashboardBuilder {
	b.dashboard.Tags = append(b.dashboard.Tags, tags...)
	return b
}

// SetTimeRange sets the default time range
func (b *DashboardBuilder) SetTimeRange(from, to string) *DashboardBuilder {
	b.dashboard.Time.From = from
	b.dashboard.Time.To = to
	return b
}

// SetRefresh sets the refresh interval
func (b *DashboardBuilder) SetRefresh(interval string) *DashboardBuilder {
	b.dashboard.Refresh = interval
	return b
}

// AddRow adds a row header
func (b *DashboardBuilder) AddRow(title string) *DashboardBuilder {
	panel := Panel{
		ID:    b.panelID,
		Title: title,
		Type:  "row",
		GridPos: GridPos{
			H: 1,
			W: 24,
			X: 0,
			Y: b.currentY,
		},
	}
	b.dashboard.Panels = append(b.dashboard.Panels, panel)
	b.panelID++
	b.currentY++
	return b
}

// AddStatPanel adds a stat panel
func (b *DashboardBuilder) AddStatPanel(title, query, legendFormat, unit string, x, w int) *DashboardBuilder {
	panel := Panel{
		ID:    b.panelID,
		Title: title,
		Type:  "stat",
		GridPos: GridPos{
			H: 4,
			W: w,
			X: x,
			Y: b.currentY,
		},
		Targets: []Target{
			{
				Expr:         query,
				RefID:        "A",
				LegendFormat: legendFormat,
			},
		},
		Datasource: &Datasource{Type: "prometheus"},
		FieldConfig: &FieldConfig{
			Defaults: FieldDefaults{
				Unit: unit,
				Color: map[string]interface{}{
					"mode": "palette-classic",
				},
			},
		},
		Options: map[string]interface{}{
			"colorMode":   "value",
			"graphMode":   "area",
			"justifyMode": "auto",
			"orientation": "auto",
			"reduceOptions": map[string]interface{}{
				"calcs":  []string{"lastNotNull"},
				"fields": "",
				"values": false,
			},
			"textMode": "auto",
		},
	}
	b.dashboard.Panels = append(b.dashboard.Panels, panel)
	b.panelID++
	return b
}

// AddGaugePanel adds a gauge panel
func (b *DashboardBuilder) AddGaugePanel(title, query, unit string, min, max float64, x, w int) *DashboardBuilder {
	panel := Panel{
		ID:    b.panelID,
		Title: title,
		Type:  "gauge",
		GridPos: GridPos{
			H: 5,
			W: w,
			X: x,
			Y: b.currentY,
		},
		Targets: []Target{
			{
				Expr:  query,
				RefID: "A",
			},
		},
		Datasource: &Datasource{Type: "prometheus"},
		FieldConfig: &FieldConfig{
			Defaults: FieldDefaults{
				Unit: unit,
				Min:  &min,
				Max:  &max,
				Thresholds: &ThresholdConfig{
					Mode: "percentage",
					Steps: []ThresholdStep{
						{Color: "green", Value: nil},
						{Color: "yellow", Value: float64Ptr(70)},
						{Color: "red", Value: float64Ptr(90)},
					},
				},
			},
		},
	}
	b.dashboard.Panels = append(b.dashboard.Panels, panel)
	b.panelID++
	return b
}

// AddGraphPanel adds a time series graph panel
func (b *DashboardBuilder) AddGraphPanel(title string, queries []Target, unit string, height int) *DashboardBuilder {
	panel := Panel{
		ID:    b.panelID,
		Title: title,
		Type:  "timeseries",
		GridPos: GridPos{
			H: height,
			W: 24,
			X: 0,
			Y: b.currentY,
		},
		Targets:    queries,
		Datasource: &Datasource{Type: "prometheus"},
		FieldConfig: &FieldConfig{
			Defaults: FieldDefaults{
				Unit: unit,
				Custom: map[string]interface{}{
					"drawStyle":         "line",
					"lineInterpolation": "linear",
					"barAlignment":      0,
					"lineWidth":         1,
					"fillOpacity":       10,
					"gradientMode":      "none",
					"spanNulls":         false,
					"showPoints":        "auto",
					"pointSize":         5,
					"stacking": map[string]interface{}{
						"mode":  "none",
						"group": "A",
					},
					"axisPlacement":  "auto",
					"axisLabel":      "",
					"scaleDistribution": map[string]interface{}{
						"type": "linear",
					},
				},
			},
		},
		Options: map[string]interface{}{
			"tooltip": map[string]interface{}{
				"mode": "single",
				"sort": "none",
			},
			"legend": map[string]interface{}{
				"showLegend":  true,
				"displayMode": "list",
				"placement":   "bottom",
				"calcs":       []string{},
			},
		},
	}
	b.dashboard.Panels = append(b.dashboard.Panels, panel)
	b.panelID++
	b.currentY += height
	return b
}

// AddTablePanel adds a table panel
func (b *DashboardBuilder) AddTablePanel(title, query string, height int) *DashboardBuilder {
	panel := Panel{
		ID:    b.panelID,
		Title: title,
		Type:  "table",
		GridPos: GridPos{
			H: height,
			W: 24,
			X: 0,
			Y: b.currentY,
		},
		Targets: []Target{
			{
				Expr:  query,
				RefID: "A",
			},
		},
		Datasource: &Datasource{Type: "prometheus"},
	}
	b.dashboard.Panels = append(b.dashboard.Panels, panel)
	b.panelID++
	b.currentY += height
	return b
}

// NextRow moves to the next row
func (b *DashboardBuilder) NextRow(height int) *DashboardBuilder {
	b.currentY += height
	return b
}

// AddVariable adds a template variable
func (b *DashboardBuilder) AddVariable(v TemplateVariable) *DashboardBuilder {
	b.dashboard.Templating.List = append(b.dashboard.Templating.List, v)
	return b
}

// Build returns the built dashboard configuration
func (b *DashboardBuilder) Build() *DashboardConfig {
	return b.dashboard
}

// ToJSON returns the dashboard as JSON
func (b *DashboardBuilder) ToJSON() ([]byte, error) {
	return json.MarshalIndent(b.dashboard, "", "  ")
}

// CreateMinionDashboard creates the default Minion agent dashboard
func CreateMinionDashboard() *DashboardConfig {
	builder := NewDashboardBuilder("Minion Agent Dashboard", "minion-overview")
	builder.SetDescription("Comprehensive dashboard for monitoring Minion AI agents")
	builder.AddTags("ai", "llm", "monitoring")
	builder.SetTimeRange("now-6h", "now")

	// Overview Row
	builder.AddRow("Overview")
	builder.AddStatPanel("Active Agents", "minion_active_agents", "", "none", 0, 6)
	builder.AddStatPanel("Total Executions", "sum(minion_agent_executions_total)", "", "short", 6, 6)
	builder.AddStatPanel("Success Rate", `sum(rate(minion_agent_executions_total{status="success"}[5m])) / sum(rate(minion_agent_executions_total[5m])) * 100`, "", "percent", 12, 6)
	builder.AddStatPanel("Avg Duration", "avg(rate(minion_agent_duration_seconds_sum[5m]) / rate(minion_agent_duration_seconds_count[5m]))", "", "s", 18, 6)
	builder.NextRow(4)

	// Agent Performance Row
	builder.AddRow("Agent Performance")
	builder.AddGraphPanel("Agent Executions Rate", []Target{
		{Expr: `sum(rate(minion_agent_executions_total[5m])) by (status)`, RefID: "A", LegendFormat: "{{status}}"},
	}, "ops", 8)

	builder.AddGraphPanel("Agent Duration (p99)", []Target{
		{Expr: `histogram_quantile(0.99, sum(rate(minion_agent_duration_seconds_bucket[5m])) by (le))`, RefID: "A", LegendFormat: "p99"},
		{Expr: `histogram_quantile(0.95, sum(rate(minion_agent_duration_seconds_bucket[5m])) by (le))`, RefID: "B", LegendFormat: "p95"},
		{Expr: `histogram_quantile(0.50, sum(rate(minion_agent_duration_seconds_bucket[5m])) by (le))`, RefID: "C", LegendFormat: "p50"},
	}, "s", 8)

	// LLM Metrics Row
	builder.AddRow("LLM Metrics")
	builder.AddGraphPanel("LLM Request Rate", []Target{
		{Expr: `sum(rate(minion_llm_requests_total[5m])) by (provider, model)`, RefID: "A", LegendFormat: "{{provider}}/{{model}}"},
	}, "ops", 8)

	builder.AddGraphPanel("LLM Latency", []Target{
		{Expr: `histogram_quantile(0.99, sum(rate(minion_llm_latency_seconds_bucket[5m])) by (le, provider))`, RefID: "A", LegendFormat: "{{provider}} p99"},
		{Expr: `histogram_quantile(0.50, sum(rate(minion_llm_latency_seconds_bucket[5m])) by (le, provider))`, RefID: "B", LegendFormat: "{{provider}} p50"},
	}, "s", 8)

	builder.AddGraphPanel("Token Usage", []Target{
		{Expr: `sum(rate(minion_llm_tokens_total[5m])) by (type)`, RefID: "A", LegendFormat: "{{type}}"},
	}, "short", 8)

	// Tool Metrics Row
	builder.AddRow("Tool Metrics")
	builder.AddGraphPanel("Tool Call Rate", []Target{
		{Expr: `sum(rate(minion_tool_calls_total[5m])) by (tool_name)`, RefID: "A", LegendFormat: "{{tool_name}}"},
	}, "ops", 8)

	builder.AddGraphPanel("Tool Error Rate", []Target{
		{Expr: `sum(rate(minion_tool_errors_total[5m])) by (tool_name)`, RefID: "A", LegendFormat: "{{tool_name}}"},
	}, "ops", 8)

	// Cost Row
	builder.AddRow("Cost")
	builder.AddStatPanel("Daily Cost", `sum(increase(minion_llm_cost_total[24h]))`, "", "currencyUSD", 0, 8)
	builder.AddStatPanel("Hourly Cost", `sum(increase(minion_llm_cost_total[1h]))`, "", "currencyUSD", 8, 8)
	builder.NextRow(4)

	builder.AddGraphPanel("Cost Over Time", []Target{
		{Expr: `sum(rate(minion_llm_cost_total[1h])) by (provider) * 3600`, RefID: "A", LegendFormat: "{{provider}}"},
	}, "currencyUSD", 8)

	return builder.Build()
}

// CreateSLODashboard creates an SLO dashboard
func CreateSLODashboard() *DashboardConfig {
	builder := NewDashboardBuilder("Minion SLO Dashboard", "minion-slo")
	builder.SetDescription("Service Level Objectives monitoring for Minion agents")
	builder.AddTags("slo", "sli", "reliability")

	// SLO Overview Row
	builder.AddRow("SLO Overview")
	builder.AddGaugePanel("Agent Availability SLO",
		`sum(rate(minion_agent_executions_total{status="success"}[30d])) / sum(rate(minion_agent_executions_total[30d])) * 100`,
		"percent", 0, 100, 0, 8)
	builder.AddGaugePanel("LLM Success Rate SLO",
		`sum(rate(minion_llm_requests_total{status="success"}[30d])) / sum(rate(minion_llm_requests_total[30d])) * 100`,
		"percent", 0, 100, 8, 8)
	builder.AddGaugePanel("Tool Success Rate SLO",
		`sum(rate(minion_tool_calls_total{status="success"}[30d])) / sum(rate(minion_tool_calls_total[30d])) * 100`,
		"percent", 0, 100, 16, 8)
	builder.NextRow(5)

	// Error Budget Row
	builder.AddRow("Error Budget")
	builder.AddGraphPanel("Error Budget Remaining", []Target{
		{Expr: `(0.995 - (1 - sum(rate(minion_agent_executions_total{status="success"}[30d])) / sum(rate(minion_agent_executions_total[30d])))) / 0.005 * 100`, RefID: "A", LegendFormat: "Agent Availability"},
		{Expr: `(0.99 - (1 - sum(rate(minion_llm_requests_total{status="success"}[30d])) / sum(rate(minion_llm_requests_total[30d])))) / 0.01 * 100`, RefID: "B", LegendFormat: "LLM Success Rate"},
	}, "percent", 8)

	// Burn Rate Row
	builder.AddRow("Burn Rate")
	builder.AddGraphPanel("SLO Burn Rate (1h window)", []Target{
		{Expr: `(1 - sum(rate(minion_agent_executions_total{status="success"}[1h])) / sum(rate(minion_agent_executions_total[1h]))) / 0.005`, RefID: "A", LegendFormat: "Agent Availability"},
		{Expr: `(1 - sum(rate(minion_llm_requests_total{status="success"}[1h])) / sum(rate(minion_llm_requests_total[1h]))) / 0.01`, RefID: "B", LegendFormat: "LLM Success Rate"},
	}, "short", 8)

	return builder.Build()
}

// Helper function
func float64Ptr(v float64) *float64 {
	return &v
}

// DashboardExporter exports dashboard configurations
type DashboardExporter struct {
	dashboards map[string]*DashboardConfig
}

// NewDashboardExporter creates a new dashboard exporter
func NewDashboardExporter() *DashboardExporter {
	return &DashboardExporter{
		dashboards: make(map[string]*DashboardConfig),
	}
}

// Add adds a dashboard
func (e *DashboardExporter) Add(dashboard *DashboardConfig) {
	e.dashboards[dashboard.UID] = dashboard
}

// ExportAll exports all dashboards as JSON
func (e *DashboardExporter) ExportAll() (map[string][]byte, error) {
	result := make(map[string][]byte)
	for uid, dashboard := range e.dashboards {
		data, err := json.MarshalIndent(dashboard, "", "  ")
		if err != nil {
			return nil, err
		}
		result[uid] = data
	}
	return result, nil
}

// GetDefaultDashboards returns all default dashboards
func GetDefaultDashboards() []*DashboardConfig {
	return []*DashboardConfig{
		CreateMinionDashboard(),
		CreateSLODashboard(),
	}
}
