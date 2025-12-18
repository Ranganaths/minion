package client

import (
	"testing"
)

func TestSchemaValidator_ValidateInput_RequiredFields(t *testing.T) {
	validator := NewSchemaValidator(false)

	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
			"age":  map[string]interface{}{"type": "number"},
		},
		"required": []interface{}{"name"},
	}

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "Valid with all required fields",
			params:  map[string]interface{}{"name": "John", "age": 30.0},
			wantErr: false,
		},
		{
			name:    "Valid with only required fields",
			params:  map[string]interface{}{"name": "John"},
			wantErr: false,
		},
		{
			name:    "Missing required field",
			params:  map[string]interface{}{"age": 30.0},
			wantErr: true,
		},
		{
			name:    "Empty params",
			params:  map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateInput(tt.params, schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateInput() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSchemaValidator_ValidateString(t *testing.T) {
	validator := NewSchemaValidator(false)

	tests := []struct {
		name    string
		value   interface{}
		schema  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "Valid string",
			value:   "hello",
			schema:  map[string]interface{}{"type": "string"},
			wantErr: false,
		},
		{
			name:    "String with min length - valid",
			value:   "hello",
			schema:  map[string]interface{}{"type": "string", "minLength": 3.0},
			wantErr: false,
		},
		{
			name:    "String with min length - invalid",
			value:   "hi",
			schema:  map[string]interface{}{"type": "string", "minLength": 5.0},
			wantErr: true,
		},
		{
			name:    "String with max length - valid",
			value:   "hello",
			schema:  map[string]interface{}{"type": "string", "maxLength": 10.0},
			wantErr: false,
		},
		{
			name:    "String with max length - invalid",
			value:   "hello world",
			schema:  map[string]interface{}{"type": "string", "maxLength": 5.0},
			wantErr: true,
		},
		{
			name:    "Wrong type",
			value:   123,
			schema:  map[string]interface{}{"type": "string"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateValue("testField", tt.value, tt.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateString() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSchemaValidator_ValidateNumber(t *testing.T) {
	validator := NewSchemaValidator(false)

	tests := []struct {
		name    string
		value   interface{}
		schema  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "Valid number",
			value:   42.0,
			schema:  map[string]interface{}{"type": "number"},
			wantErr: false,
		},
		{
			name:    "Valid integer as float",
			value:   42.0,
			schema:  map[string]interface{}{"type": "integer"},
			wantErr: false,
		},
		{
			name:    "Number with minimum - valid",
			value:   10.0,
			schema:  map[string]interface{}{"type": "number", "minimum": 5.0},
			wantErr: false,
		},
		{
			name:    "Number with minimum - invalid",
			value:   3.0,
			schema:  map[string]interface{}{"type": "number", "minimum": 5.0},
			wantErr: true,
		},
		{
			name:    "Number with maximum - valid",
			value:   10.0,
			schema:  map[string]interface{}{"type": "number", "maximum": 20.0},
			wantErr: false,
		},
		{
			name:    "Number with maximum - invalid",
			value:   25.0,
			schema:  map[string]interface{}{"type": "number", "maximum": 20.0},
			wantErr: true,
		},
		{
			name:    "Int type",
			value:   42,
			schema:  map[string]interface{}{"type": "number"},
			wantErr: false,
		},
		{
			name:    "Wrong type",
			value:   "not a number",
			schema:  map[string]interface{}{"type": "number"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateValue("testField", tt.value, tt.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateNumber() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSchemaValidator_ValidateArray(t *testing.T) {
	validator := NewSchemaValidator(false)

	tests := []struct {
		name    string
		value   interface{}
		schema  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "Valid array",
			value:   []interface{}{"a", "b", "c"},
			schema:  map[string]interface{}{"type": "array"},
			wantErr: false,
		},
		{
			name:    "Array with min items - valid",
			value:   []interface{}{"a", "b", "c"},
			schema:  map[string]interface{}{"type": "array", "minItems": 2.0},
			wantErr: false,
		},
		{
			name:    "Array with min items - invalid",
			value:   []interface{}{"a"},
			schema:  map[string]interface{}{"type": "array", "minItems": 2.0},
			wantErr: true,
		},
		{
			name:    "Array with max items - valid",
			value:   []interface{}{"a", "b"},
			schema:  map[string]interface{}{"type": "array", "maxItems": 3.0},
			wantErr: false,
		},
		{
			name:    "Array with max items - invalid",
			value:   []interface{}{"a", "b", "c", "d"},
			schema:  map[string]interface{}{"type": "array", "maxItems": 3.0},
			wantErr: true,
		},
		{
			name:  "Array with item schema - valid",
			value: []interface{}{"hello", "world"},
			schema: map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			wantErr: false,
		},
		{
			name:  "Array with item schema - invalid",
			value: []interface{}{"hello", 123},
			schema: map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			wantErr: true,
		},
		{
			name:    "Wrong type",
			value:   "not an array",
			schema:  map[string]interface{}{"type": "array"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateValue("testField", tt.value, tt.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateArray() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSchemaValidator_StrictMode(t *testing.T) {
	strictValidator := NewSchemaValidator(true)
	relaxedValidator := NewSchemaValidator(false)

	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
	}

	params := map[string]interface{}{
		"name":    "John",
		"unknown": "field", // Unknown parameter
	}

	// Strict mode should reject unknown parameters
	err := strictValidator.ValidateInput(params, schema)
	if err == nil {
		t.Error("Expected error in strict mode for unknown parameter")
	}

	// Relaxed mode should allow unknown parameters
	err = relaxedValidator.ValidateInput(params, schema)
	if err != nil {
		t.Errorf("Expected no error in relaxed mode, got: %v", err)
	}
}

func TestGetJSONType(t *testing.T) {
	tests := []struct {
		value    interface{}
		expected string
	}{
		{nil, "null"},
		{true, "boolean"},
		{"hello", "string"},
		{42.0, "number"},
		{42, "number"},
		{[]interface{}{}, "array"},
		{map[string]interface{}{}, "object"},
	}

	for _, tt := range tests {
		result := getJSONType(tt.value)
		if result != tt.expected {
			t.Errorf("getJSONType(%v) = %s, want %s", tt.value, result, tt.expected)
		}
	}
}

func TestGetRequiredFields(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
			"age":  map[string]interface{}{"type": "number"},
		},
		"required": []interface{}{"name", "age"},
	}

	required := GetRequiredFields(schema)
	if len(required) != 2 {
		t.Errorf("Expected 2 required fields, got %d", len(required))
	}

	// Verify specific fields
	hasName := false
	hasAge := false
	for _, field := range required {
		if field == "name" {
			hasName = true
		}
		if field == "age" {
			hasAge = true
		}
	}

	if !hasName || !hasAge {
		t.Error("Expected required fields to include 'name' and 'age'")
	}
}

func TestGetSchemaDescription(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
			"age":  map[string]interface{}{"type": "number"},
		},
		"required": []interface{}{"name"},
	}

	desc := GetSchemaDescription(schema)
	if desc == "" {
		t.Error("Expected non-empty description")
	}

	// Should mention both total and required parameters
	if desc != "Parameters: 2 defined, 1 required" {
		t.Errorf("Unexpected description: %s", desc)
	}
}

func TestValidateToolCall(t *testing.T) {
	validator := NewSchemaValidator(false)

	tool := &MCPTool{
		Name:        "create_user",
		Description: "Creates a new user",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"username": map[string]interface{}{"type": "string"},
				"email":    map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"username", "email"},
		},
	}

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "Valid params",
			params: map[string]interface{}{
				"username": "john_doe",
				"email":    "john@example.com",
			},
			wantErr: false,
		},
		{
			name: "Missing required field",
			params: map[string]interface{}{
				"username": "john_doe",
			},
			wantErr: true,
		},
		{
			name: "Wrong type",
			params: map[string]interface{}{
				"username": "john_doe",
				"email":    123, // Should be string
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateToolCall(tool, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateToolCall() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateToolCall_NoSchema(t *testing.T) {
	validator := NewSchemaValidator(false)

	tool := &MCPTool{
		Name:        "simple_tool",
		Description: "A tool without schema",
		InputSchema: nil,
	}

	params := map[string]interface{}{
		"any": "value",
	}

	// Should not error when no schema is defined
	err := validator.ValidateToolCall(tool, params)
	if err != nil {
		t.Errorf("Expected no error for tool without schema, got: %v", err)
	}
}
