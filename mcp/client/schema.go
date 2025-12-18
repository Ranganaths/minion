package client

import (
	"fmt"
	"reflect"
)

// SchemaValidator validates tool inputs against JSON schemas
type SchemaValidator struct {
	strictMode bool
}

// NewSchemaValidator creates a new schema validator
func NewSchemaValidator(strictMode bool) *SchemaValidator {
	return &SchemaValidator{
		strictMode: strictMode,
	}
}

// ValidateInput validates input parameters against a tool's input schema
func (v *SchemaValidator) ValidateInput(params map[string]interface{}, schema map[string]interface{}) error {
	if schema == nil {
		return nil // No schema means no validation
	}

	// Get schema type
	schemaType, ok := schema["type"].(string)
	if !ok {
		return nil // No type specified
	}

	if schemaType != "object" {
		return fmt.Errorf("unsupported schema type: %s (expected 'object')", schemaType)
	}

	// Get properties
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		properties = make(map[string]interface{})
	}

	// Get required fields
	required := []string{}
	if requiredArray, ok := schema["required"].([]interface{}); ok {
		for _, r := range requiredArray {
			if reqStr, ok := r.(string); ok {
				required = append(required, reqStr)
			}
		}
	}

	// Check required fields
	for _, fieldName := range required {
		if _, exists := params[fieldName]; !exists {
			return fmt.Errorf("required field missing: %s", fieldName)
		}
	}

	// Validate each parameter
	for paramName, paramValue := range params {
		propSchema, exists := properties[paramName]
		if !exists {
			if v.strictMode {
				return fmt.Errorf("unknown parameter: %s", paramName)
			}
			continue // Skip unknown parameters in non-strict mode
		}

		propSchemaMap, ok := propSchema.(map[string]interface{})
		if !ok {
			continue // Skip if property schema is not a map
		}

		if err := v.validateValue(paramName, paramValue, propSchemaMap); err != nil {
			return err
		}
	}

	return nil
}

// validateValue validates a single value against its schema
func (v *SchemaValidator) validateValue(fieldName string, value interface{}, schema map[string]interface{}) error {
	expectedType, ok := schema["type"].(string)
	if !ok {
		return nil // No type constraint
	}

	actualType := getJSONType(value)

	// Type checking
	if !v.isTypeCompatible(actualType, expectedType) {
		return fmt.Errorf("field '%s': expected type '%s', got '%s'", fieldName, expectedType, actualType)
	}

	// Additional validations based on type
	switch expectedType {
	case "string":
		return v.validateString(fieldName, value, schema)
	case "number", "integer":
		return v.validateNumber(fieldName, value, schema)
	case "array":
		return v.validateArray(fieldName, value, schema)
	case "object":
		return v.validateObject(fieldName, value, schema)
	}

	return nil
}

// validateString validates string values
func (v *SchemaValidator) validateString(fieldName string, value interface{}, schema map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("field '%s': expected string", fieldName)
	}

	// Min length
	if minLength, ok := schema["minLength"].(float64); ok {
		if float64(len(str)) < minLength {
			return fmt.Errorf("field '%s': string too short (min: %.0f, got: %d)", fieldName, minLength, len(str))
		}
	}

	// Max length
	if maxLength, ok := schema["maxLength"].(float64); ok {
		if float64(len(str)) > maxLength {
			return fmt.Errorf("field '%s': string too long (max: %.0f, got: %d)", fieldName, maxLength, len(str))
		}
	}

	// Pattern (regex) - simplified check
	if pattern, ok := schema["pattern"].(string); ok && pattern != "" {
		// Basic validation - in production, use regexp package
		_ = pattern // Avoid unused variable
		// TODO: Implement regex validation
	}

	// Enum
	if enum, ok := schema["enum"].([]interface{}); ok {
		found := false
		for _, allowed := range enum {
			if allowedStr, ok := allowed.(string); ok && allowedStr == str {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("field '%s': value not in enum", fieldName)
		}
	}

	return nil
}

// validateNumber validates number values
func (v *SchemaValidator) validateNumber(fieldName string, value interface{}, schema map[string]interface{}) error {
	var num float64
	switch val := value.(type) {
	case float64:
		num = val
	case float32:
		num = float64(val)
	case int:
		num = float64(val)
	case int64:
		num = float64(val)
	default:
		return fmt.Errorf("field '%s': expected number", fieldName)
	}

	// Minimum
	if minimum, ok := schema["minimum"].(float64); ok {
		if num < minimum {
			return fmt.Errorf("field '%s': value too small (min: %.2f, got: %.2f)", fieldName, minimum, num)
		}
	}

	// Maximum
	if maximum, ok := schema["maximum"].(float64); ok {
		if num > maximum {
			return fmt.Errorf("field '%s': value too large (max: %.2f, got: %.2f)", fieldName, maximum, num)
		}
	}

	// Exclusive minimum
	if exclusiveMin, ok := schema["exclusiveMinimum"].(float64); ok {
		if num <= exclusiveMin {
			return fmt.Errorf("field '%s': value must be greater than %.2f", fieldName, exclusiveMin)
		}
	}

	// Exclusive maximum
	if exclusiveMax, ok := schema["exclusiveMaximum"].(float64); ok {
		if num >= exclusiveMax {
			return fmt.Errorf("field '%s': value must be less than %.2f", fieldName, exclusiveMax)
		}
	}

	return nil
}

// validateArray validates array values
func (v *SchemaValidator) validateArray(fieldName string, value interface{}, schema map[string]interface{}) error {
	arr, ok := value.([]interface{})
	if !ok {
		return fmt.Errorf("field '%s': expected array", fieldName)
	}

	// Min items
	if minItems, ok := schema["minItems"].(float64); ok {
		if float64(len(arr)) < minItems {
			return fmt.Errorf("field '%s': array too short (min: %.0f, got: %d)", fieldName, minItems, len(arr))
		}
	}

	// Max items
	if maxItems, ok := schema["maxItems"].(float64); ok {
		if float64(len(arr)) > maxItems {
			return fmt.Errorf("field '%s': array too long (max: %.0f, got: %d)", fieldName, maxItems, len(arr))
		}
	}

	// Items schema
	if itemsSchema, ok := schema["items"].(map[string]interface{}); ok {
		for i, item := range arr {
			itemName := fmt.Sprintf("%s[%d]", fieldName, i)
			if err := v.validateValue(itemName, item, itemsSchema); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateObject validates object values
func (v *SchemaValidator) validateObject(fieldName string, value interface{}, schema map[string]interface{}) error {
	obj, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("field '%s': expected object", fieldName)
	}

	// Nested properties
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for propName, propValue := range obj {
			if propSchema, exists := properties[propName]; exists {
				if propSchemaMap, ok := propSchema.(map[string]interface{}); ok {
					nestedName := fmt.Sprintf("%s.%s", fieldName, propName)
					if err := v.validateValue(nestedName, propValue, propSchemaMap); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// getJSONType returns the JSON type name for a Go value
func getJSONType(value interface{}) string {
	if value == nil {
		return "null"
	}

	switch value.(type) {
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64, float32, int, int32, int64, uint, uint32, uint64:
		return "number"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return reflect.TypeOf(value).Kind().String()
	}
}

// isTypeCompatible checks if actual type is compatible with expected type
func (v *SchemaValidator) isTypeCompatible(actual, expected string) bool {
	if actual == expected {
		return true
	}

	// Special cases
	if expected == "integer" && actual == "number" {
		return true // Numbers can be integers
	}

	return false
}

// ValidateToolCall validates a complete tool call
func (v *SchemaValidator) ValidateToolCall(tool *MCPTool, params map[string]interface{}) error {
	if tool == nil {
		return fmt.Errorf("tool is nil")
	}

	if tool.InputSchema == nil {
		return nil // No schema to validate against
	}

	return v.ValidateInput(params, tool.InputSchema)
}

// GetSchemaDescription returns a human-readable description of the schema
func GetSchemaDescription(schema map[string]interface{}) string {
	if schema == nil {
		return "No schema defined"
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return "Schema format not supported"
	}

	required := []string{}
	if requiredArray, ok := schema["required"].([]interface{}); ok {
		for _, r := range requiredArray {
			if reqStr, ok := r.(string); ok {
				required = append(required, reqStr)
			}
		}
	}

	desc := fmt.Sprintf("Parameters: %d defined", len(properties))
	if len(required) > 0 {
		desc += fmt.Sprintf(", %d required", len(required))
	}

	return desc
}

// GetRequiredFields returns a list of required field names
func GetRequiredFields(schema map[string]interface{}) []string {
	if schema == nil {
		return []string{}
	}

	required := []string{}
	if requiredArray, ok := schema["required"].([]interface{}); ok {
		for _, r := range requiredArray {
			if reqStr, ok := r.(string); ok {
				required = append(required, reqStr)
			}
		}
	}

	return required
}
