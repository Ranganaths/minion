package observability

import (
	"encoding/json"
	"testing"
)

func TestDashboardBuilderCreate(t *testing.T) {
	builder := NewDashboardBuilder("Test Dashboard", "test-dashboard")
	dashboard := builder.Build()

	if dashboard.Title != "Test Dashboard" {
		t.Errorf("Expected title 'Test Dashboard', got '%s'", dashboard.Title)
	}

	if dashboard.UID != "test-dashboard" {
		t.Errorf("Expected UID 'test-dashboard', got '%s'", dashboard.UID)
	}
}

func TestDashboardBuilderSetDescription(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")
	builder.SetDescription("Test description")
	dashboard := builder.Build()

	if dashboard.Description != "Test description" {
		t.Errorf("Expected description 'Test description', got '%s'", dashboard.Description)
	}
}

func TestDashboardBuilderAddTags(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")
	builder.AddTags("tag1", "tag2")
	dashboard := builder.Build()

	// Default tags plus added tags
	found := 0
	for _, tag := range dashboard.Tags {
		if tag == "tag1" || tag == "tag2" {
			found++
		}
	}
	if found != 2 {
		t.Errorf("Expected 2 added tags, found %d", found)
	}
}

func TestDashboardBuilderSetTimeRange(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")
	builder.SetTimeRange("now-24h", "now")
	dashboard := builder.Build()

	if dashboard.Time.From != "now-24h" {
		t.Errorf("Expected from 'now-24h', got '%s'", dashboard.Time.From)
	}

	if dashboard.Time.To != "now" {
		t.Errorf("Expected to 'now', got '%s'", dashboard.Time.To)
	}
}

func TestDashboardBuilderSetRefresh(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")
	builder.SetRefresh("10s")
	dashboard := builder.Build()

	if dashboard.Refresh != "10s" {
		t.Errorf("Expected refresh '10s', got '%s'", dashboard.Refresh)
	}
}

func TestDashboardBuilderAddRow(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")
	builder.AddRow("Test Row")
	dashboard := builder.Build()

	if len(dashboard.Panels) != 1 {
		t.Errorf("Expected 1 panel, got %d", len(dashboard.Panels))
	}

	if dashboard.Panels[0].Type != "row" {
		t.Errorf("Expected type 'row', got '%s'", dashboard.Panels[0].Type)
	}

	if dashboard.Panels[0].Title != "Test Row" {
		t.Errorf("Expected title 'Test Row', got '%s'", dashboard.Panels[0].Title)
	}
}

func TestDashboardBuilderAddStatPanel(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")
	builder.AddStatPanel("Test Stat", "test_metric", "{{label}}", "percent", 0, 6)
	dashboard := builder.Build()

	if len(dashboard.Panels) != 1 {
		t.Errorf("Expected 1 panel, got %d", len(dashboard.Panels))
	}

	panel := dashboard.Panels[0]
	if panel.Type != "stat" {
		t.Errorf("Expected type 'stat', got '%s'", panel.Type)
	}

	if panel.GridPos.W != 6 {
		t.Errorf("Expected width 6, got %d", panel.GridPos.W)
	}

	if len(panel.Targets) != 1 {
		t.Errorf("Expected 1 target, got %d", len(panel.Targets))
	}

	if panel.Targets[0].Expr != "test_metric" {
		t.Errorf("Expected expr 'test_metric', got '%s'", panel.Targets[0].Expr)
	}
}

func TestDashboardBuilderAddGaugePanel(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")
	builder.AddGaugePanel("Test Gauge", "test_metric", "percent", 0, 100, 0, 8)
	dashboard := builder.Build()

	if len(dashboard.Panels) != 1 {
		t.Errorf("Expected 1 panel, got %d", len(dashboard.Panels))
	}

	panel := dashboard.Panels[0]
	if panel.Type != "gauge" {
		t.Errorf("Expected type 'gauge', got '%s'", panel.Type)
	}
}

func TestDashboardBuilderAddGraphPanel(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")
	queries := []Target{
		{Expr: "metric1", RefID: "A", LegendFormat: "{{label}}"},
		{Expr: "metric2", RefID: "B", LegendFormat: "{{other}}"},
	}
	builder.AddGraphPanel("Test Graph", queries, "short", 8)
	dashboard := builder.Build()

	if len(dashboard.Panels) != 1 {
		t.Errorf("Expected 1 panel, got %d", len(dashboard.Panels))
	}

	panel := dashboard.Panels[0]
	if panel.Type != "timeseries" {
		t.Errorf("Expected type 'timeseries', got '%s'", panel.Type)
	}

	if len(panel.Targets) != 2 {
		t.Errorf("Expected 2 targets, got %d", len(panel.Targets))
	}
}

func TestDashboardBuilderAddTablePanel(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")
	builder.AddTablePanel("Test Table", "test_metric", 6)
	dashboard := builder.Build()

	if len(dashboard.Panels) != 1 {
		t.Errorf("Expected 1 panel, got %d", len(dashboard.Panels))
	}

	panel := dashboard.Panels[0]
	if panel.Type != "table" {
		t.Errorf("Expected type 'table', got '%s'", panel.Type)
	}
}

func TestDashboardBuilderNextRow(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")
	builder.AddStatPanel("Stat 1", "metric1", "", "none", 0, 6)
	initialY := builder.currentY
	builder.NextRow(4)
	builder.AddStatPanel("Stat 2", "metric2", "", "none", 0, 6)
	dashboard := builder.Build()

	if len(dashboard.Panels) != 2 {
		t.Errorf("Expected 2 panels, got %d", len(dashboard.Panels))
	}

	// Second panel should be at a higher Y position
	if dashboard.Panels[1].GridPos.Y <= initialY {
		t.Errorf("Expected second panel at Y > %d, got %d", initialY, dashboard.Panels[1].GridPos.Y)
	}
}

func TestDashboardBuilderAddVariable(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")
	variable := TemplateVariable{
		Name:  "datasource",
		Label: "Data Source",
		Type:  "datasource",
	}
	builder.AddVariable(variable)
	dashboard := builder.Build()

	if len(dashboard.Templating.List) != 1 {
		t.Errorf("Expected 1 variable, got %d", len(dashboard.Templating.List))
	}

	if dashboard.Templating.List[0].Name != "datasource" {
		t.Errorf("Expected name 'datasource', got '%s'", dashboard.Templating.List[0].Name)
	}
}

func TestDashboardBuilderToJSON(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")
	builder.AddStatPanel("Test Stat", "test_metric", "", "none", 0, 6)

	jsonData, err := builder.ToJSON()
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if parsed["title"] != "Test" {
		t.Errorf("Expected title 'Test' in JSON")
	}
}

func TestCreateMinionDashboard(t *testing.T) {
	dashboard := CreateMinionDashboard()

	if dashboard.Title == "" {
		t.Error("Dashboard should have a title")
	}

	if dashboard.UID == "" {
		t.Error("Dashboard should have a UID")
	}

	if len(dashboard.Panels) == 0 {
		t.Error("Dashboard should have panels")
	}

	// Check for expected panel types
	hasRow := false
	hasStat := false
	hasTimeseries := false
	for _, panel := range dashboard.Panels {
		switch panel.Type {
		case "row":
			hasRow = true
		case "stat":
			hasStat = true
		case "timeseries":
			hasTimeseries = true
		}
	}

	if !hasRow {
		t.Error("Dashboard should have row panels")
	}
	if !hasStat {
		t.Error("Dashboard should have stat panels")
	}
	if !hasTimeseries {
		t.Error("Dashboard should have timeseries panels")
	}
}

func TestCreateSLODashboard(t *testing.T) {
	dashboard := CreateSLODashboard()

	if dashboard.Title == "" {
		t.Error("Dashboard should have a title")
	}

	if dashboard.UID == "" {
		t.Error("Dashboard should have a UID")
	}

	if len(dashboard.Panels) == 0 {
		t.Error("Dashboard should have panels")
	}

	// SLO dashboard should have gauge panels
	hasGauge := false
	for _, panel := range dashboard.Panels {
		if panel.Type == "gauge" {
			hasGauge = true
			break
		}
	}

	if !hasGauge {
		t.Error("SLO Dashboard should have gauge panels")
	}
}

func TestGetDefaultDashboards(t *testing.T) {
	dashboards := GetDefaultDashboards()

	if len(dashboards) < 2 {
		t.Errorf("Expected at least 2 default dashboards, got %d", len(dashboards))
	}

	// Check that all dashboards have required fields
	for _, dashboard := range dashboards {
		if dashboard.UID == "" {
			t.Error("Dashboard should have a UID")
		}
		if dashboard.Title == "" {
			t.Error("Dashboard should have a title")
		}
	}
}

func TestDashboardExporter(t *testing.T) {
	exporter := NewDashboardExporter()

	dashboard1 := CreateMinionDashboard()
	dashboard2 := CreateSLODashboard()

	exporter.Add(dashboard1)
	exporter.Add(dashboard2)

	exported, err := exporter.ExportAll()
	if err != nil {
		t.Fatalf("Failed to export dashboards: %v", err)
	}

	if len(exported) != 2 {
		t.Errorf("Expected 2 exported dashboards, got %d", len(exported))
	}

	// Verify each exported dashboard is valid JSON
	for uid, data := range exported {
		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Errorf("Invalid JSON for dashboard %s: %v", uid, err)
		}
	}
}

func TestDashboardThresholds(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")
	builder.AddGaugePanel("Test Gauge", "test_metric", "percent", 0, 100, 0, 8)
	dashboard := builder.Build()

	panel := dashboard.Panels[0]
	if panel.FieldConfig == nil {
		t.Fatal("Expected field config")
	}

	if panel.FieldConfig.Defaults.Thresholds == nil {
		t.Fatal("Expected thresholds")
	}

	if len(panel.FieldConfig.Defaults.Thresholds.Steps) != 3 {
		t.Errorf("Expected 3 threshold steps, got %d", len(panel.FieldConfig.Defaults.Thresholds.Steps))
	}
}

func TestGridPositioning(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")

	// Add panels at specific positions
	builder.AddStatPanel("Stat 1", "metric1", "", "none", 0, 6)
	builder.AddStatPanel("Stat 2", "metric2", "", "none", 6, 6)
	builder.AddStatPanel("Stat 3", "metric3", "", "none", 12, 6)
	builder.AddStatPanel("Stat 4", "metric4", "", "none", 18, 6)

	dashboard := builder.Build()

	// Verify positions
	for i, panel := range dashboard.Panels {
		expectedX := (i * 6) % 24
		if panel.GridPos.X != expectedX {
			t.Errorf("Panel %d: expected X=%d, got %d", i, expectedX, panel.GridPos.X)
		}
	}
}

func TestPanelIDs(t *testing.T) {
	builder := NewDashboardBuilder("Test", "test")

	builder.AddRow("Row 1")
	builder.AddStatPanel("Stat 1", "metric1", "", "none", 0, 6)
	builder.AddGraphPanel("Graph 1", []Target{{Expr: "metric2", RefID: "A"}}, "short", 8)

	dashboard := builder.Build()

	// All panel IDs should be unique
	ids := make(map[int]bool)
	for _, panel := range dashboard.Panels {
		if ids[panel.ID] {
			t.Errorf("Duplicate panel ID: %d", panel.ID)
		}
		ids[panel.ID] = true
	}
}
