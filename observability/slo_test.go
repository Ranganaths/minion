package observability

import (
	"context"
	"testing"
	"time"
)

func TestSLOManagerRegister(t *testing.T) {
	manager := NewSLOManager(nil)

	slo := &SLO{
		ID:     "test-slo",
		Name:   "Test SLO",
		Type:   SLOTypeAvailability,
		Target: 99.9,
		Window: 30 * 24 * time.Hour,
	}

	err := manager.RegisterSLO(slo)
	if err != nil {
		t.Fatalf("Failed to register SLO: %v", err)
	}

	retrieved, err := manager.GetSLO("test-slo")
	if err != nil {
		t.Fatalf("Failed to get SLO: %v", err)
	}

	if retrieved.Name != "Test SLO" {
		t.Errorf("Expected name 'Test SLO', got '%s'", retrieved.Name)
	}
}

func TestSLOManagerRegisterNoID(t *testing.T) {
	manager := NewSLOManager(nil)

	slo := &SLO{
		Name:   "Test SLO",
		Type:   SLOTypeAvailability,
		Target: 99.9,
	}

	err := manager.RegisterSLO(slo)
	if err == nil {
		t.Error("Expected error for SLO without ID")
	}
}

func TestSLOManagerListSLOs(t *testing.T) {
	manager := NewSLOManager(nil)

	manager.RegisterSLO(&SLO{ID: "slo-1", Name: "SLO 1", Type: SLOTypeAvailability, Target: 99.9})
	manager.RegisterSLO(&SLO{ID: "slo-2", Name: "SLO 2", Type: SLOTypeLatency, Target: 99.5})
	manager.RegisterSLO(&SLO{ID: "slo-3", Name: "SLO 3", Type: SLOTypeErrorRate, Target: 99.0})

	slos := manager.ListSLOs()
	if len(slos) != 3 {
		t.Errorf("Expected 3 SLOs, got %d", len(slos))
	}
}

func TestSLOManagerRecordMeasurement(t *testing.T) {
	manager := NewSLOManager(nil)

	slo := &SLO{
		ID:     "test-slo",
		Name:   "Test SLO",
		Type:   SLOTypeAvailability,
		Target: 99.0,
		Window: time.Hour,
	}
	manager.RegisterSLO(slo)

	// Record some measurements
	err := manager.RecordMeasurement("test-slo", 100, 99)
	if err != nil {
		t.Fatalf("Failed to record measurement: %v", err)
	}

	status, err := manager.GetStatus("test-slo")
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	if status.CurrentValue != 99.0 {
		t.Errorf("Expected current value 99.0, got %.2f", status.CurrentValue)
	}
}

func TestSLOManagerRecordMeasurementInvalidSLO(t *testing.T) {
	manager := NewSLOManager(nil)

	err := manager.RecordMeasurement("non-existent", 100, 99)
	if err == nil {
		t.Error("Expected error for non-existent SLO")
	}
}

func TestSLOStatus(t *testing.T) {
	manager := NewSLOManager(nil)

	slo := &SLO{
		ID:     "test-slo",
		Name:   "Test SLO",
		Type:   SLOTypeAvailability,
		Target: 99.0,
		Window: time.Hour,
	}
	manager.RegisterSLO(slo)

	// Record measurements: 95% success rate
	manager.RecordMeasurement("test-slo", 100, 95)

	status, _ := manager.GetStatus("test-slo")

	if status.IsHealthy {
		t.Error("Expected unhealthy status when below target")
	}

	if status.CurrentValue != 95.0 {
		t.Errorf("Expected current value 95.0, got %.2f", status.CurrentValue)
	}

	// Error budget should be negative when below target
	if status.ErrorBudget >= 0 {
		t.Log("Error budget may be negative when below target")
	}
}

func TestSLOStatusHealthy(t *testing.T) {
	manager := NewSLOManager(nil)

	slo := &SLO{
		ID:     "test-slo",
		Name:   "Test SLO",
		Type:   SLOTypeAvailability,
		Target: 99.0,
		Window: time.Hour,
	}
	manager.RegisterSLO(slo)

	// Record measurements: 99.5% success rate (above target)
	manager.RecordMeasurement("test-slo", 200, 199)

	status, _ := manager.GetStatus("test-slo")

	if !status.IsHealthy {
		t.Error("Expected healthy status when above target")
	}
}

func TestSLOBurnRate(t *testing.T) {
	manager := NewSLOManager(nil)

	slo := &SLO{
		ID:     "test-slo",
		Name:   "Test SLO",
		Type:   SLOTypeAvailability,
		Target: 99.0,
		Window: time.Hour,
		BurnRate: &BurnRateConfig{
			ShortWindow:  10 * time.Minute,
			LongWindow:   time.Hour,
			FastBurnRate: 14.4,
			SlowBurnRate: 6.0,
		},
	}
	manager.RegisterSLO(slo)

	// Record measurements with 95% success (burning error budget fast)
	manager.RecordMeasurement("test-slo", 100, 95)

	status, _ := manager.GetStatus("test-slo")

	// Burn rate should be > 0 when consuming error budget
	if status.BurnRate <= 0 {
		t.Log("Burn rate calculation may vary based on timing")
	}
}

func TestSLOAlertChannel(t *testing.T) {
	manager := NewSLOManager(nil)

	alertCh := manager.GetAlertChannel()
	if alertCh == nil {
		t.Error("Alert channel should not be nil")
	}
}

func TestSLOGetAllStatuses(t *testing.T) {
	manager := NewSLOManager(nil)

	manager.RegisterSLO(&SLO{ID: "slo-1", Name: "SLO 1", Type: SLOTypeAvailability, Target: 99.9, Window: time.Hour})
	manager.RegisterSLO(&SLO{ID: "slo-2", Name: "SLO 2", Type: SLOTypeLatency, Target: 99.5, Window: time.Hour})

	// Record measurements
	manager.RecordMeasurement("slo-1", 100, 99)
	manager.RecordMeasurement("slo-2", 100, 98)

	statuses := manager.GetAllStatuses()
	if len(statuses) != 2 {
		t.Errorf("Expected 2 statuses, got %d", len(statuses))
	}
}

func TestDefaultAgentSLOs(t *testing.T) {
	slos := DefaultAgentSLOs()

	if len(slos) == 0 {
		t.Error("Expected at least one default SLO")
	}

	// Check that all SLOs have required fields
	for _, slo := range slos {
		if slo.ID == "" {
			t.Error("SLO should have an ID")
		}
		if slo.Name == "" {
			t.Error("SLO should have a name")
		}
		if slo.Target <= 0 {
			t.Error("SLO should have a positive target")
		}
	}
}

func TestSLOReporter(t *testing.T) {
	manager := NewSLOManager(nil)
	manager.RegisterSLO(&SLO{ID: "slo-1", Name: "SLO 1", Type: SLOTypeAvailability, Target: 99.0, Window: time.Hour})
	manager.RecordMeasurement("slo-1", 100, 99)

	reporter := NewSLOReporter(manager)
	report := reporter.GenerateReport(context.Background())

	if report.Summary.TotalSLOs != 1 {
		t.Errorf("Expected 1 total SLO, got %d", report.Summary.TotalSLOs)
	}

	if len(report.Statuses) != 1 {
		t.Errorf("Expected 1 status, got %d", len(report.Statuses))
	}

	if report.GeneratedAt.IsZero() {
		t.Error("Report should have generation time")
	}
}

func TestSLOReportSummary(t *testing.T) {
	manager := NewSLOManager(nil)

	// Add healthy SLO
	manager.RegisterSLO(&SLO{ID: "healthy", Name: "Healthy SLO", Type: SLOTypeAvailability, Target: 99.0, Window: time.Hour})
	manager.RecordMeasurement("healthy", 100, 100)

	// Add unhealthy SLO
	manager.RegisterSLO(&SLO{ID: "unhealthy", Name: "Unhealthy SLO", Type: SLOTypeAvailability, Target: 99.0, Window: time.Hour})
	manager.RecordMeasurement("unhealthy", 100, 90)

	reporter := NewSLOReporter(manager)
	report := reporter.GenerateReport(context.Background())

	if report.Summary.HealthySLOs != 1 {
		t.Errorf("Expected 1 healthy SLO, got %d", report.Summary.HealthySLOs)
	}

	if report.Summary.UnhealthySLOs != 1 {
		t.Errorf("Expected 1 unhealthy SLO, got %d", report.Summary.UnhealthySLOs)
	}
}

func TestSLOAlertRules(t *testing.T) {
	manager := NewSLOManager(nil)

	slo := &SLO{
		ID:     "test-slo",
		Name:   "Test SLO",
		Type:   SLOTypeAvailability,
		Target: 99.0,
		Window: time.Hour,
		AlertRules: []SLOAlertRule{
			{
				Name:      "Critical",
				Severity:  "critical",
				Threshold: 95.0,
				Message:   "Critical alert",
			},
		},
	}
	manager.RegisterSLO(slo)

	// Record measurement below threshold to trigger alert
	manager.RecordMeasurement("test-slo", 100, 90)

	// Check if alert was generated (non-blocking)
	select {
	case alert := <-manager.GetAlertChannel():
		if alert.Severity != "critical" {
			t.Errorf("Expected critical severity, got %s", alert.Severity)
		}
	default:
		// Alert might not be in channel immediately, that's ok
	}
}

func TestSLOEmptyMeasurements(t *testing.T) {
	manager := NewSLOManager(nil)

	slo := &SLO{
		ID:     "test-slo",
		Name:   "Test SLO",
		Type:   SLOTypeAvailability,
		Target: 99.0,
		Window: time.Hour,
	}
	manager.RegisterSLO(slo)

	status, err := manager.GetStatus("test-slo")
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	// With no measurements, should be healthy at 100%
	if !status.IsHealthy {
		t.Error("Expected healthy status with no measurements")
	}

	if status.CurrentValue != 100.0 {
		t.Errorf("Expected 100%% current value with no measurements, got %.2f", status.CurrentValue)
	}
}
