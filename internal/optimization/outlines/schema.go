// Package outlines provides structured output generation with JSON schema constraints.
package outlines

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// JSONSchema represents a JSON Schema definition.
type JSONSchema struct {
	Type                 string                 `json:"type,omitempty"`
	Properties           map[string]*JSONSchema `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	Items                *JSONSchema            `json:"items,omitempty"`
	Enum                 []interface{}          `json:"enum,omitempty"`
	Const                interface{}            `json:"const,omitempty"`
	MinLength            *int                   `json:"minLength,omitempty"`
	MaxLength            *int                   `json:"maxLength,omitempty"`
	Minimum              *float64               `json:"minimum,omitempty"`
	Maximum              *float64               `json:"maximum,omitempty"`
	ExclusiveMinimum     *float64               `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum     *float64               `json:"exclusiveMaximum,omitempty"`
	Pattern              string                 `json:"pattern,omitempty"`
	MinItems             *int                   `json:"minItems,omitempty"`
	MaxItems             *int                   `json:"maxItems,omitempty"`
	UniqueItems          bool                   `json:"uniqueItems,omitempty"`
	AdditionalProperties *bool                  `json:"additionalProperties,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Default              interface{}            `json:"default,omitempty"`
	Format               string                 `json:"format,omitempty"`
	OneOf                []*JSONSchema          `json:"oneOf,omitempty"`
	AnyOf                []*JSONSchema          `json:"anyOf,omitempty"`
	AllOf                []*JSONSchema          `json:"allOf,omitempty"`
	Not                  *JSONSchema            `json:"not,omitempty"`
	Definitions          map[string]*JSONSchema `json:"definitions,omitempty"`
	Ref                  string                 `json:"$ref,omitempty"`
}

// ParseSchema parses a JSON schema from bytes.
func ParseSchema(data []byte) (*JSONSchema, error) {
	var schema JSONSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}
	return &schema, nil
}

// ParseSchemaFromMap parses a JSON schema from a map.
func ParseSchemaFromMap(data map[string]interface{}) (*JSONSchema, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal map: %w", err)
	}
	return ParseSchema(bytes)
}

// String returns the JSON representation of the schema.
func (s *JSONSchema) String() string {
	bytes, _ := json.MarshalIndent(s, "", "  ")
	return string(bytes)
}

// IsRequired checks if a property is required.
func (s *JSONSchema) IsRequired(property string) bool {
	for _, req := range s.Required {
		if req == property {
			return true
		}
	}
	return false
}

// GetPropertySchema returns the schema for a property.
func (s *JSONSchema) GetPropertySchema(property string) *JSONSchema {
	if s.Properties == nil {
		return nil
	}
	return s.Properties[property]
}

// SchemaBuilder provides a fluent API for building JSON schemas.
type SchemaBuilder struct {
	schema *JSONSchema
}

// NewSchemaBuilder creates a new schema builder.
func NewSchemaBuilder() *SchemaBuilder {
	return &SchemaBuilder{
		schema: &JSONSchema{},
	}
}

// Type sets the schema type.
func (b *SchemaBuilder) Type(t string) *SchemaBuilder {
	b.schema.Type = t
	return b
}

// Object sets the type to object.
func (b *SchemaBuilder) Object() *SchemaBuilder {
	b.schema.Type = "object"
	if b.schema.Properties == nil {
		b.schema.Properties = make(map[string]*JSONSchema)
	}
	return b
}

// Array sets the type to array.
func (b *SchemaBuilder) Array() *SchemaBuilder {
	b.schema.Type = "array"
	return b
}

// String sets the type to string.
func (b *SchemaBuilder) String() *SchemaBuilder {
	b.schema.Type = "string"
	return b
}

// Number sets the type to number.
func (b *SchemaBuilder) Number() *SchemaBuilder {
	b.schema.Type = "number"
	return b
}

// Integer sets the type to integer.
func (b *SchemaBuilder) Integer() *SchemaBuilder {
	b.schema.Type = "integer"
	return b
}

// Boolean sets the type to boolean.
func (b *SchemaBuilder) Boolean() *SchemaBuilder {
	b.schema.Type = "boolean"
	return b
}

// Property adds a property to an object schema.
func (b *SchemaBuilder) Property(name string, schema *JSONSchema) *SchemaBuilder {
	if b.schema.Properties == nil {
		b.schema.Properties = make(map[string]*JSONSchema)
	}
	b.schema.Properties[name] = schema
	return b
}

// Required marks properties as required.
func (b *SchemaBuilder) Required(properties ...string) *SchemaBuilder {
	b.schema.Required = append(b.schema.Required, properties...)
	return b
}

// Items sets the items schema for an array.
func (b *SchemaBuilder) Items(schema *JSONSchema) *SchemaBuilder {
	b.schema.Items = schema
	return b
}

// Enum sets allowed values.
func (b *SchemaBuilder) Enum(values ...interface{}) *SchemaBuilder {
	b.schema.Enum = values
	return b
}

// MinLength sets minimum string length.
func (b *SchemaBuilder) MinLength(n int) *SchemaBuilder {
	b.schema.MinLength = &n
	return b
}

// MaxLength sets maximum string length.
func (b *SchemaBuilder) MaxLength(n int) *SchemaBuilder {
	b.schema.MaxLength = &n
	return b
}

// Minimum sets minimum number value.
func (b *SchemaBuilder) Minimum(n float64) *SchemaBuilder {
	b.schema.Minimum = &n
	return b
}

// Maximum sets maximum number value.
func (b *SchemaBuilder) Maximum(n float64) *SchemaBuilder {
	b.schema.Maximum = &n
	return b
}

// Pattern sets a regex pattern for strings.
func (b *SchemaBuilder) Pattern(pattern string) *SchemaBuilder {
	b.schema.Pattern = pattern
	return b
}

// MinItems sets minimum array items.
func (b *SchemaBuilder) MinItems(n int) *SchemaBuilder {
	b.schema.MinItems = &n
	return b
}

// MaxItems sets maximum array items.
func (b *SchemaBuilder) MaxItems(n int) *SchemaBuilder {
	b.schema.MaxItems = &n
	return b
}

// Description sets the schema description.
func (b *SchemaBuilder) Description(desc string) *SchemaBuilder {
	b.schema.Description = desc
	return b
}

// Default sets the default value.
func (b *SchemaBuilder) Default(value interface{}) *SchemaBuilder {
	b.schema.Default = value
	return b
}

// Format sets the format (e.g., "date-time", "email", "uri").
func (b *SchemaBuilder) Format(format string) *SchemaBuilder {
	b.schema.Format = format
	return b
}

// Build returns the constructed schema.
func (b *SchemaBuilder) Build() *JSONSchema {
	return b.schema
}

// Common schema helpers

// StringSchema creates a string schema.
func StringSchema() *JSONSchema {
	return &JSONSchema{Type: "string"}
}

// IntegerSchema creates an integer schema.
func IntegerSchema() *JSONSchema {
	return &JSONSchema{Type: "integer"}
}

// NumberSchema creates a number schema.
func NumberSchema() *JSONSchema {
	return &JSONSchema{Type: "number"}
}

// BooleanSchema creates a boolean schema.
func BooleanSchema() *JSONSchema {
	return &JSONSchema{Type: "boolean"}
}

// ArraySchema creates an array schema with item type.
func ArraySchema(items *JSONSchema) *JSONSchema {
	return &JSONSchema{Type: "array", Items: items}
}

// EnumSchema creates an enum schema.
func EnumSchema(values ...interface{}) *JSONSchema {
	return &JSONSchema{Enum: values}
}

// ObjectSchema creates an object schema with properties.
func ObjectSchema(properties map[string]*JSONSchema, required ...string) *JSONSchema {
	return &JSONSchema{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
}

// PatternString creates a string schema with a regex pattern.
func PatternString(pattern string) *JSONSchema {
	return &JSONSchema{Type: "string", Pattern: pattern}
}

// CompiledPattern holds a compiled regex pattern for validation.
type CompiledPattern struct {
	Pattern string
	Regex   *regexp.Regexp
}

// CompilePattern compiles a regex pattern.
func CompilePattern(pattern string) (*CompiledPattern, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}
	return &CompiledPattern{
		Pattern: pattern,
		Regex:   re,
	}, nil
}

// Match checks if a string matches the pattern.
func (p *CompiledPattern) Match(s string) bool {
	return p.Regex.MatchString(s)
}
