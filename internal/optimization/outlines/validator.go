package outlines

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// ValidationError represents a validation error.
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s", e.Path, e.Message)
	}
	return e.Message
}

// ValidationResult contains the result of schema validation.
type ValidationResult struct {
	Valid  bool               `json:"valid"`
	Errors []*ValidationError `json:"errors,omitempty"`
	Data   interface{}        `json:"data,omitempty"`
}

// AddError adds a validation error.
func (r *ValidationResult) AddError(path, message string) {
	r.Valid = false
	r.Errors = append(r.Errors, &ValidationError{
		Path:    path,
		Message: message,
	})
}

// ErrorMessages returns all error messages as strings.
func (r *ValidationResult) ErrorMessages() []string {
	messages := make([]string, len(r.Errors))
	for i, err := range r.Errors {
		messages[i] = err.Error()
	}
	return messages
}

// SchemaValidator validates JSON data against a schema.
type SchemaValidator struct {
	schema          *JSONSchema
	compiledPattern *regexp.Regexp
}

// NewSchemaValidator creates a new schema validator.
func NewSchemaValidator(schema *JSONSchema) (*SchemaValidator, error) {
	v := &SchemaValidator{schema: schema}

	// Pre-compile pattern if present
	if schema.Pattern != "" {
		re, err := regexp.Compile(schema.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", schema.Pattern, err)
		}
		v.compiledPattern = re
	}

	return v, nil
}

// Validate validates a JSON string against the schema.
func (v *SchemaValidator) Validate(jsonStr string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		result.AddError("", fmt.Sprintf("invalid JSON: %v", err))
		return result
	}

	v.validateValue(data, v.schema, "", result)
	result.Data = data

	return result
}

// ValidateData validates parsed data against the schema.
func (v *SchemaValidator) ValidateData(data interface{}) *ValidationResult {
	result := &ValidationResult{Valid: true, Data: data}
	v.validateValue(data, v.schema, "", result)
	return result
}

func (v *SchemaValidator) validateValue(data interface{}, schema *JSONSchema, path string, result *ValidationResult) {
	if schema == nil {
		return
	}

	// Handle enum first (can be any type)
	if len(schema.Enum) > 0 {
		v.validateEnum(data, schema.Enum, path, result)
		return
	}

	// Handle const
	if schema.Const != nil {
		if !reflect.DeepEqual(data, schema.Const) {
			result.AddError(path, fmt.Sprintf("value must be %v", schema.Const))
		}
		return
	}

	// Handle oneOf/anyOf/allOf
	if len(schema.OneOf) > 0 {
		v.validateOneOf(data, schema.OneOf, path, result)
		return
	}
	if len(schema.AnyOf) > 0 {
		v.validateAnyOf(data, schema.AnyOf, path, result)
		return
	}
	if len(schema.AllOf) > 0 {
		v.validateAllOf(data, schema.AllOf, path, result)
		return
	}

	// Validate based on type
	switch schema.Type {
	case "object":
		v.validateObject(data, schema, path, result)
	case "array":
		v.validateArray(data, schema, path, result)
	case "string":
		v.validateString(data, schema, path, result)
	case "number":
		v.validateNumber(data, schema, path, result)
	case "integer":
		v.validateInteger(data, schema, path, result)
	case "boolean":
		v.validateBoolean(data, path, result)
	case "null":
		v.validateNull(data, path, result)
	case "":
		// No type specified, any type is valid
	default:
		result.AddError(path, fmt.Sprintf("unknown type: %s", schema.Type))
	}
}

func (v *SchemaValidator) validateObject(data interface{}, schema *JSONSchema, path string, result *ValidationResult) {
	obj, ok := data.(map[string]interface{})
	if !ok {
		result.AddError(path, "expected object")
		return
	}

	// Check required properties
	for _, req := range schema.Required {
		if _, exists := obj[req]; !exists {
			result.AddError(joinPath(path, req), "required property missing")
		}
	}

	// Validate each property
	for propName, propSchema := range schema.Properties {
		if propValue, exists := obj[propName]; exists {
			v.validateValue(propValue, propSchema, joinPath(path, propName), result)
		}
	}

	// Check additional properties
	if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
		for propName := range obj {
			if _, defined := schema.Properties[propName]; !defined {
				result.AddError(joinPath(path, propName), "additional property not allowed")
			}
		}
	}
}

func (v *SchemaValidator) validateArray(data interface{}, schema *JSONSchema, path string, result *ValidationResult) {
	arr, ok := data.([]interface{})
	if !ok {
		result.AddError(path, "expected array")
		return
	}

	// Check min/max items
	if schema.MinItems != nil && len(arr) < *schema.MinItems {
		result.AddError(path, fmt.Sprintf("array must have at least %d items", *schema.MinItems))
	}
	if schema.MaxItems != nil && len(arr) > *schema.MaxItems {
		result.AddError(path, fmt.Sprintf("array must have at most %d items", *schema.MaxItems))
	}

	// Check unique items
	if schema.UniqueItems {
		seen := make(map[string]bool)
		for i, item := range arr {
			key := fmt.Sprintf("%v", item)
			if seen[key] {
				result.AddError(fmt.Sprintf("%s[%d]", path, i), "duplicate item in array")
			}
			seen[key] = true
		}
	}

	// Validate each item
	if schema.Items != nil {
		for i, item := range arr {
			v.validateValue(item, schema.Items, fmt.Sprintf("%s[%d]", path, i), result)
		}
	}
}

func (v *SchemaValidator) validateString(data interface{}, schema *JSONSchema, path string, result *ValidationResult) {
	str, ok := data.(string)
	if !ok {
		result.AddError(path, "expected string")
		return
	}

	// Check length constraints
	if schema.MinLength != nil && len(str) < *schema.MinLength {
		result.AddError(path, fmt.Sprintf("string must be at least %d characters", *schema.MinLength))
	}
	if schema.MaxLength != nil && len(str) > *schema.MaxLength {
		result.AddError(path, fmt.Sprintf("string must be at most %d characters", *schema.MaxLength))
	}

	// Check pattern
	if schema.Pattern != "" {
		re := v.compiledPattern
		if re == nil {
			// Compile on the fly if this is a nested schema
			re, _ = regexp.Compile(schema.Pattern)
		}
		if re != nil && !re.MatchString(str) {
			result.AddError(path, fmt.Sprintf("string must match pattern %q", schema.Pattern))
		}
	}

	// Check format
	if schema.Format != "" {
		v.validateFormat(str, schema.Format, path, result)
	}
}

func (v *SchemaValidator) validateFormat(str, format, path string, result *ValidationResult) {
	var valid bool
	switch format {
	case "email":
		valid = isValidEmail(str)
	case "uri", "url":
		valid = isValidURI(str)
	case "date-time":
		valid = isValidDateTime(str)
	case "date":
		valid = isValidDate(str)
	case "time":
		valid = isValidTime(str)
	case "uuid":
		valid = isValidUUID(str)
	case "ipv4":
		valid = isValidIPv4(str)
	case "ipv6":
		valid = isValidIPv6(str)
	default:
		// Unknown format, skip validation
		return
	}

	if !valid {
		result.AddError(path, fmt.Sprintf("invalid %s format", format))
	}
}

func (v *SchemaValidator) validateNumber(data interface{}, schema *JSONSchema, path string, result *ValidationResult) {
	var num float64
	switch n := data.(type) {
	case float64:
		num = n
	case int:
		num = float64(n)
	case int64:
		num = float64(n)
	default:
		result.AddError(path, "expected number")
		return
	}

	v.validateNumericConstraints(num, schema, path, result)
}

func (v *SchemaValidator) validateInteger(data interface{}, schema *JSONSchema, path string, result *ValidationResult) {
	var num float64
	switch n := data.(type) {
	case float64:
		if n != float64(int64(n)) {
			result.AddError(path, "expected integer")
			return
		}
		num = n
	case int:
		num = float64(n)
	case int64:
		num = float64(n)
	default:
		result.AddError(path, "expected integer")
		return
	}

	v.validateNumericConstraints(num, schema, path, result)
}

func (v *SchemaValidator) validateNumericConstraints(num float64, schema *JSONSchema, path string, result *ValidationResult) {
	if schema.Minimum != nil && num < *schema.Minimum {
		result.AddError(path, fmt.Sprintf("value must be >= %v", *schema.Minimum))
	}
	if schema.Maximum != nil && num > *schema.Maximum {
		result.AddError(path, fmt.Sprintf("value must be <= %v", *schema.Maximum))
	}
	if schema.ExclusiveMinimum != nil && num <= *schema.ExclusiveMinimum {
		result.AddError(path, fmt.Sprintf("value must be > %v", *schema.ExclusiveMinimum))
	}
	if schema.ExclusiveMaximum != nil && num >= *schema.ExclusiveMaximum {
		result.AddError(path, fmt.Sprintf("value must be < %v", *schema.ExclusiveMaximum))
	}
}

func (v *SchemaValidator) validateBoolean(data interface{}, path string, result *ValidationResult) {
	if _, ok := data.(bool); !ok {
		result.AddError(path, "expected boolean")
	}
}

func (v *SchemaValidator) validateNull(data interface{}, path string, result *ValidationResult) {
	if data != nil {
		result.AddError(path, "expected null")
	}
}

func (v *SchemaValidator) validateEnum(data interface{}, enum []interface{}, path string, result *ValidationResult) {
	for _, allowed := range enum {
		if reflect.DeepEqual(data, allowed) {
			return
		}
	}
	result.AddError(path, fmt.Sprintf("value must be one of %v", enum))
}

func (v *SchemaValidator) validateOneOf(data interface{}, schemas []*JSONSchema, path string, result *ValidationResult) {
	validCount := 0
	for _, schema := range schemas {
		subResult := &ValidationResult{Valid: true}
		v.validateValue(data, schema, path, subResult)
		if subResult.Valid {
			validCount++
		}
	}
	if validCount != 1 {
		result.AddError(path, fmt.Sprintf("must match exactly one schema (matched %d)", validCount))
	}
}

func (v *SchemaValidator) validateAnyOf(data interface{}, schemas []*JSONSchema, path string, result *ValidationResult) {
	for _, schema := range schemas {
		subResult := &ValidationResult{Valid: true}
		v.validateValue(data, schema, path, subResult)
		if subResult.Valid {
			return
		}
	}
	result.AddError(path, "must match at least one schema")
}

func (v *SchemaValidator) validateAllOf(data interface{}, schemas []*JSONSchema, path string, result *ValidationResult) {
	for _, schema := range schemas {
		v.validateValue(data, schema, path, result)
	}
}

func joinPath(base, property string) string {
	if base == "" {
		return property
	}
	return base + "." + property
}

// Format validation helpers

func isValidEmail(s string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(s)
}

func isValidURI(s string) bool {
	re := regexp.MustCompile(`^https?://[^\s]+$`)
	return re.MatchString(s)
}

func isValidDateTime(s string) bool {
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(.\d+)?(Z|[+-]\d{2}:\d{2})?$`)
	return re.MatchString(s)
}

func isValidDate(s string) bool {
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	return re.MatchString(s)
}

func isValidTime(s string) bool {
	re := regexp.MustCompile(`^\d{2}:\d{2}:\d{2}(.\d+)?$`)
	return re.MatchString(s)
}

func isValidUUID(s string) bool {
	re := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	return re.MatchString(s)
}

func isValidIPv4(s string) bool {
	re := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	if !re.MatchString(s) {
		return false
	}
	parts := strings.Split(s, ".")
	for _, p := range parts {
		var n int
		fmt.Sscanf(p, "%d", &n)
		if n < 0 || n > 255 {
			return false
		}
	}
	return true
}

func isValidIPv6(s string) bool {
	re := regexp.MustCompile(`^([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$|^::$|^([0-9a-fA-F]{1,4}:)*::([0-9a-fA-F]{1,4}:)*[0-9a-fA-F]{1,4}$`)
	return re.MatchString(s)
}

// Validate is a convenience function to validate JSON against a schema.
func Validate(jsonStr string, schema *JSONSchema) *ValidationResult {
	validator, err := NewSchemaValidator(schema)
	if err != nil {
		return &ValidationResult{
			Valid:  false,
			Errors: []*ValidationError{{Message: err.Error()}},
		}
	}
	return validator.Validate(jsonStr)
}
