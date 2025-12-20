package config

import (
	"os"
	"testing"
	"time"
)

func TestEnv_GetString(t *testing.T) {
	env := NewEnv("TEST")

	// Test default value
	result := env.GetString("NONEXISTENT", "default")
	if result != "default" {
		t.Errorf("expected 'default', got '%s'", result)
	}

	// Test with value set
	os.Setenv("TEST_STRING_VAR", "value")
	defer os.Unsetenv("TEST_STRING_VAR")

	result = env.GetString("STRING_VAR", "default")
	if result != "value" {
		t.Errorf("expected 'value', got '%s'", result)
	}
}

func TestEnv_GetInt(t *testing.T) {
	env := NewEnv("TEST")

	// Test default value
	result := env.GetInt("NONEXISTENT", 42)
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}

	// Test with value set
	os.Setenv("TEST_INT_VAR", "100")
	defer os.Unsetenv("TEST_INT_VAR")

	result = env.GetInt("INT_VAR", 42)
	if result != 100 {
		t.Errorf("expected 100, got %d", result)
	}

	// Test with invalid value
	os.Setenv("TEST_INT_INVALID", "not_a_number")
	defer os.Unsetenv("TEST_INT_INVALID")

	result = env.GetInt("INT_INVALID", 42)
	if result != 42 {
		t.Errorf("expected 42 for invalid int, got %d", result)
	}
}

func TestEnv_GetFloat64(t *testing.T) {
	env := NewEnv("TEST")

	// Test default value
	result := env.GetFloat64("NONEXISTENT", 3.14)
	if result != 3.14 {
		t.Errorf("expected 3.14, got %f", result)
	}

	// Test with value set
	os.Setenv("TEST_FLOAT_VAR", "2.718")
	defer os.Unsetenv("TEST_FLOAT_VAR")

	result = env.GetFloat64("FLOAT_VAR", 3.14)
	if result != 2.718 {
		t.Errorf("expected 2.718, got %f", result)
	}
}

func TestEnv_GetBool(t *testing.T) {
	env := NewEnv("TEST")

	tests := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"1", true},
		{"yes", true},
		{"YES", true},
		{"on", true},
		{"ON", true},
		{"false", false},
		{"FALSE", false},
		{"0", false},
		{"no", false},
		{"off", false},
	}

	for _, tc := range tests {
		os.Setenv("TEST_BOOL_VAR", tc.value)
		result := env.GetBool("BOOL_VAR", !tc.expected)
		if result != tc.expected {
			t.Errorf("for value '%s', expected %v, got %v", tc.value, tc.expected, result)
		}
	}
	os.Unsetenv("TEST_BOOL_VAR")

	// Test default value
	result := env.GetBool("NONEXISTENT", true)
	if result != true {
		t.Error("expected default true")
	}
}

func TestEnv_GetDuration(t *testing.T) {
	env := NewEnv("TEST")

	// Test default value
	result := env.GetDuration("NONEXISTENT", 5*time.Second)
	if result != 5*time.Second {
		t.Errorf("expected 5s, got %v", result)
	}

	// Test with value set
	os.Setenv("TEST_DURATION_VAR", "10m")
	defer os.Unsetenv("TEST_DURATION_VAR")

	result = env.GetDuration("DURATION_VAR", 5*time.Second)
	if result != 10*time.Minute {
		t.Errorf("expected 10m, got %v", result)
	}

	// Test with invalid value
	os.Setenv("TEST_DURATION_INVALID", "not_a_duration")
	defer os.Unsetenv("TEST_DURATION_INVALID")

	result = env.GetDuration("DURATION_INVALID", 5*time.Second)
	if result != 5*time.Second {
		t.Errorf("expected 5s for invalid duration, got %v", result)
	}
}

func TestEnv_GetStringSlice(t *testing.T) {
	env := NewEnv("TEST")

	// Test default value
	result := env.GetStringSlice("NONEXISTENT", []string{"a", "b"})
	if len(result) != 2 || result[0] != "a" || result[1] != "b" {
		t.Errorf("expected [a, b], got %v", result)
	}

	// Test with value set
	os.Setenv("TEST_SLICE_VAR", "x, y, z")
	defer os.Unsetenv("TEST_SLICE_VAR")

	result = env.GetStringSlice("SLICE_VAR", []string{"a", "b"})
	if len(result) != 3 || result[0] != "x" || result[1] != "y" || result[2] != "z" {
		t.Errorf("expected [x, y, z], got %v", result)
	}

	// Test with empty segments
	os.Setenv("TEST_SLICE_EMPTY", "a, , b")
	defer os.Unsetenv("TEST_SLICE_EMPTY")

	result = env.GetStringSlice("SLICE_EMPTY", []string{})
	if len(result) != 2 || result[0] != "a" || result[1] != "b" {
		t.Errorf("expected [a, b] (empty segments removed), got %v", result)
	}
}

func TestEnv_MustGetString(t *testing.T) {
	env := NewEnv("TEST")

	// Test with value set
	os.Setenv("TEST_REQUIRED", "value")
	defer os.Unsetenv("TEST_REQUIRED")

	result := env.MustGetString("REQUIRED")
	if result != "value" {
		t.Errorf("expected 'value', got '%s'", result)
	}

	// Test panic on missing
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing required variable")
		}
	}()

	env.MustGetString("NONEXISTENT")
}

func TestEnv_IsSet(t *testing.T) {
	env := NewEnv("TEST")

	// Test not set
	if env.IsSet("NONEXISTENT") {
		t.Error("expected IsSet to return false for unset variable")
	}

	// Test set with value
	os.Setenv("TEST_SET_VAR", "value")
	defer os.Unsetenv("TEST_SET_VAR")

	if !env.IsSet("SET_VAR") {
		t.Error("expected IsSet to return true for set variable")
	}

	// Test set with empty value
	os.Setenv("TEST_EMPTY_VAR", "")
	defer os.Unsetenv("TEST_EMPTY_VAR")

	if !env.IsSet("EMPTY_VAR") {
		t.Error("expected IsSet to return true for variable set to empty string")
	}
}

func TestEnv_NoPrefix(t *testing.T) {
	env := NewEnv("")

	os.Setenv("BARE_VAR", "bare_value")
	defer os.Unsetenv("BARE_VAR")

	result := env.GetString("BARE_VAR", "default")
	if result != "bare_value" {
		t.Errorf("expected 'bare_value', got '%s'", result)
	}
}

func TestDefaultEnv(t *testing.T) {
	os.Setenv("MINION_TEST_VAR", "minion_value")
	defer os.Unsetenv("MINION_TEST_VAR")

	result := GetString("TEST_VAR", "default")
	if result != "minion_value" {
		t.Errorf("expected 'minion_value', got '%s'", result)
	}
}
