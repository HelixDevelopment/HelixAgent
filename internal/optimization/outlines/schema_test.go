package outlines

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// ParseSchema Tests
// =============================================================================

func TestParseSchema_ValidJSON(t *testing.T) {
	t.Parallel()
	data := []byte(`{"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"]}`)
	schema, err := ParseSchema(data)
	require.NoError(t, err)
	assert.Equal(t, "object", schema.Type)
	assert.Len(t, schema.Properties, 1)
	assert.Contains(t, schema.Required, "name")
}

func TestParseSchema_InvalidJSON_Dedicated(t *testing.T) {
	t.Parallel()
	data := []byte(`{invalid json}`)
	schema, err := ParseSchema(data)
	assert.Error(t, err)
	assert.Nil(t, schema)
	assert.Contains(t, err.Error(), "failed to parse schema")
}

func TestParseSchema_EmptyObject(t *testing.T) {
	t.Parallel()
	data := []byte(`{}`)
	schema, err := ParseSchema(data)
	require.NoError(t, err)
	assert.Equal(t, "", schema.Type)
	assert.Nil(t, schema.Properties)
}

func TestParseSchema_AllFields(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"type": "object",
		"description": "A test schema",
		"format": "custom",
		"pattern": "^[a-z]+$",
		"minLength": 1,
		"maxLength": 100,
		"minimum": 0,
		"maximum": 999,
		"exclusiveMinimum": -1,
		"exclusiveMaximum": 1000,
		"minItems": 0,
		"maxItems": 50,
		"uniqueItems": true,
		"additionalProperties": false,
		"enum": ["a", "b", "c"],
		"default": "a"
	}`)

	schema, err := ParseSchema(data)
	require.NoError(t, err)
	assert.Equal(t, "object", schema.Type)
	assert.Equal(t, "A test schema", schema.Description)
	assert.Equal(t, "custom", schema.Format)
	assert.Equal(t, "^[a-z]+$", schema.Pattern)
	assert.NotNil(t, schema.MinLength)
	assert.Equal(t, 1, *schema.MinLength)
	assert.NotNil(t, schema.MaxLength)
	assert.Equal(t, 100, *schema.MaxLength)
	assert.NotNil(t, schema.Minimum)
	assert.Equal(t, float64(0), *schema.Minimum)
	assert.NotNil(t, schema.Maximum)
	assert.Equal(t, float64(999), *schema.Maximum)
	assert.True(t, schema.UniqueItems)
	assert.NotNil(t, schema.AdditionalProperties)
	assert.False(t, *schema.AdditionalProperties)
	assert.Len(t, schema.Enum, 3)
}

// =============================================================================
// ParseSchemaFromMap Tests
// =============================================================================

func TestParseSchemaFromMap_Valid(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"type": "string",
		"enum": []interface{}{"red", "green", "blue"},
	}

	schema, err := ParseSchemaFromMap(data)
	require.NoError(t, err)
	assert.Equal(t, "string", schema.Type)
	assert.Len(t, schema.Enum, 3)
}

func TestParseSchemaFromMap_WithProperties(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"age": map[string]interface{}{
				"type": "integer",
			},
		},
		"required": []interface{}{"name"},
	}

	schema, err := ParseSchemaFromMap(data)
	require.NoError(t, err)
	assert.Equal(t, "object", schema.Type)
	assert.Len(t, schema.Properties, 2)
}

// =============================================================================
// JSONSchema Methods Tests
// =============================================================================

func TestJSONSchema_String_Dedicated(t *testing.T) {
	t.Parallel()
	schema := &JSONSchema{
		Type: "string",
	}
	str := schema.String()
	assert.Contains(t, str, `"type": "string"`)
}

func TestJSONSchema_IsRequired_True(t *testing.T) {
	t.Parallel()
	schema := &JSONSchema{
		Required: []string{"name", "email"},
	}
	assert.True(t, schema.IsRequired("name"))
	assert.True(t, schema.IsRequired("email"))
}

func TestJSONSchema_IsRequired_False(t *testing.T) {
	t.Parallel()
	schema := &JSONSchema{
		Required: []string{"name"},
	}
	assert.False(t, schema.IsRequired("age"))
}

func TestJSONSchema_IsRequired_Empty(t *testing.T) {
	t.Parallel()
	schema := &JSONSchema{}
	assert.False(t, schema.IsRequired("anything"))
}

func TestJSONSchema_GetPropertySchema_Found(t *testing.T) {
	t.Parallel()
	nameSchema := &JSONSchema{Type: "string"}
	schema := &JSONSchema{
		Properties: map[string]*JSONSchema{
			"name": nameSchema,
		},
	}

	result := schema.GetPropertySchema("name")
	assert.Equal(t, nameSchema, result)
}

func TestJSONSchema_GetPropertySchema_NotFound(t *testing.T) {
	t.Parallel()
	schema := &JSONSchema{
		Properties: map[string]*JSONSchema{
			"name": {Type: "string"},
		},
	}

	result := schema.GetPropertySchema("age")
	assert.Nil(t, result)
}

func TestJSONSchema_GetPropertySchema_NilProperties(t *testing.T) {
	t.Parallel()
	schema := &JSONSchema{}
	result := schema.GetPropertySchema("name")
	assert.Nil(t, result)
}

// =============================================================================
// SchemaBuilder Tests
// =============================================================================

func TestNewSchemaBuilder(t *testing.T) {
	t.Parallel()
	b := NewSchemaBuilder()
	assert.NotNil(t, b)
	assert.NotNil(t, b.schema)
}

func TestSchemaBuilder_Type_Dedicated(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().Type("custom").Build()
	assert.Equal(t, "custom", schema.Type)
}

func TestSchemaBuilder_Object(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().Object().Build()
	assert.Equal(t, "object", schema.Type)
	assert.NotNil(t, schema.Properties)
}

func TestSchemaBuilder_Array_Dedicated(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().Array().Build()
	assert.Equal(t, "array", schema.Type)
}

func TestSchemaBuilder_String_Type(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().String().Build()
	assert.Equal(t, "string", schema.Type)
}

func TestSchemaBuilder_Number_Dedicated(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().Number().Build()
	assert.Equal(t, "number", schema.Type)
}

func TestSchemaBuilder_Integer_Dedicated(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().Integer().Build()
	assert.Equal(t, "integer", schema.Type)
}

func TestSchemaBuilder_Boolean_Dedicated(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().Boolean().Build()
	assert.Equal(t, "boolean", schema.Type)
}

func TestSchemaBuilder_Property(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().
		Object().
		Property("name", StringSchema()).
		Property("age", IntegerSchema()).
		Build()

	assert.Len(t, schema.Properties, 2)
	assert.Equal(t, "string", schema.Properties["name"].Type)
	assert.Equal(t, "integer", schema.Properties["age"].Type)
}

func TestSchemaBuilder_Property_NilProperties(t *testing.T) {
	t.Parallel()
	// Property should initialize the map if nil
	schema := NewSchemaBuilder().
		Property("name", StringSchema()).
		Build()

	assert.NotNil(t, schema.Properties)
	assert.Equal(t, "string", schema.Properties["name"].Type)
}

func TestSchemaBuilder_Required(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().
		Object().
		Required("name", "email").
		Build()

	assert.Contains(t, schema.Required, "name")
	assert.Contains(t, schema.Required, "email")
}

func TestSchemaBuilder_Items(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().
		Array().
		Items(StringSchema()).
		Build()

	assert.NotNil(t, schema.Items)
	assert.Equal(t, "string", schema.Items.Type)
}

func TestSchemaBuilder_Enum_Dedicated(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().
		Enum("red", "green", "blue").
		Build()

	assert.Len(t, schema.Enum, 3)
}

func TestSchemaBuilder_MinMaxLength(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().
		String().
		MinLength(1).
		MaxLength(255).
		Build()

	assert.NotNil(t, schema.MinLength)
	assert.Equal(t, 1, *schema.MinLength)
	assert.NotNil(t, schema.MaxLength)
	assert.Equal(t, 255, *schema.MaxLength)
}

func TestSchemaBuilder_MinMaximum(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().
		Number().
		Minimum(0).
		Maximum(100).
		Build()

	assert.NotNil(t, schema.Minimum)
	assert.Equal(t, float64(0), *schema.Minimum)
	assert.NotNil(t, schema.Maximum)
	assert.Equal(t, float64(100), *schema.Maximum)
}

func TestSchemaBuilder_Pattern(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().
		String().
		Pattern(`^\d{3}-\d{2}-\d{4}$`).
		Build()

	assert.Equal(t, `^\d{3}-\d{2}-\d{4}$`, schema.Pattern)
}

func TestSchemaBuilder_MinMaxItems(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().
		Array().
		MinItems(1).
		MaxItems(10).
		Build()

	assert.NotNil(t, schema.MinItems)
	assert.Equal(t, 1, *schema.MinItems)
	assert.NotNil(t, schema.MaxItems)
	assert.Equal(t, 10, *schema.MaxItems)
}

func TestSchemaBuilder_Description_Dedicated(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().
		Description("A test schema").
		Build()

	assert.Equal(t, "A test schema", schema.Description)
}

func TestSchemaBuilder_Default_Dedicated(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().
		String().
		Default("hello").
		Build()

	assert.Equal(t, "hello", schema.Default)
}

func TestSchemaBuilder_Format_Dedicated(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().
		String().
		Format("date-time").
		Build()

	assert.Equal(t, "date-time", schema.Format)
}

func TestSchemaBuilder_Chaining(t *testing.T) {
	t.Parallel()
	schema := NewSchemaBuilder().
		Object().
		Property("email", NewSchemaBuilder().
			String().
			Format("email").
			Pattern(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`).
			MinLength(5).
			MaxLength(254).
			Description("User email address").
			Build()).
		Required("email").
		Description("User object").
		Build()

	assert.Equal(t, "object", schema.Type)
	assert.Equal(t, "User object", schema.Description)
	emailProp := schema.Properties["email"]
	assert.NotNil(t, emailProp)
	assert.Equal(t, "email", emailProp.Format)
	assert.Equal(t, "string", emailProp.Type)
}

// =============================================================================
// Helper Schema Functions Tests
// =============================================================================

func TestStringSchema(t *testing.T) {
	t.Parallel()
	s := StringSchema()
	assert.Equal(t, "string", s.Type)
}

func TestIntegerSchema(t *testing.T) {
	t.Parallel()
	s := IntegerSchema()
	assert.Equal(t, "integer", s.Type)
}

func TestNumberSchema_Dedicated(t *testing.T) {
	t.Parallel()
	s := NumberSchema()
	assert.Equal(t, "number", s.Type)
}

func TestBooleanSchema(t *testing.T) {
	t.Parallel()
	s := BooleanSchema()
	assert.Equal(t, "boolean", s.Type)
}

func TestArraySchema_Dedicated(t *testing.T) {
	t.Parallel()
	s := ArraySchema(StringSchema())
	assert.Equal(t, "array", s.Type)
	assert.NotNil(t, s.Items)
	assert.Equal(t, "string", s.Items.Type)
}

func TestEnumSchema(t *testing.T) {
	t.Parallel()
	s := EnumSchema("a", "b", "c")
	assert.Len(t, s.Enum, 3)
}

func TestObjectSchema(t *testing.T) {
	t.Parallel()
	props := map[string]*JSONSchema{
		"name": StringSchema(),
		"age":  IntegerSchema(),
	}
	s := ObjectSchema(props, "name")
	assert.Equal(t, "object", s.Type)
	assert.Len(t, s.Properties, 2)
	assert.Contains(t, s.Required, "name")
}

func TestObjectSchema_NoRequired(t *testing.T) {
	t.Parallel()
	props := map[string]*JSONSchema{
		"name": StringSchema(),
	}
	s := ObjectSchema(props)
	assert.Equal(t, "object", s.Type)
	assert.Nil(t, s.Required)
}

func TestPatternString(t *testing.T) {
	t.Parallel()
	s := PatternString(`^\d+$`)
	assert.Equal(t, "string", s.Type)
	assert.Equal(t, `^\d+$`, s.Pattern)
}

// =============================================================================
// CompiledPattern Tests
// =============================================================================

func TestCompilePattern_Valid(t *testing.T) {
	t.Parallel()
	cp, err := CompilePattern(`^[a-z]+$`)
	require.NoError(t, err)
	assert.NotNil(t, cp)
	assert.Equal(t, `^[a-z]+$`, cp.Pattern)
	assert.NotNil(t, cp.Regex)
}

func TestCompilePattern_Invalid(t *testing.T) {
	t.Parallel()
	cp, err := CompilePattern(`[invalid`)
	assert.Error(t, err)
	assert.Nil(t, cp)
	assert.Contains(t, err.Error(), "invalid pattern")
}

func TestCompiledPattern_Match_True(t *testing.T) {
	t.Parallel()
	cp, err := CompilePattern(`^\d{3}-\d{4}$`)
	require.NoError(t, err)

	assert.True(t, cp.Match("123-4567"))
}

func TestCompiledPattern_Match_False(t *testing.T) {
	t.Parallel()
	cp, err := CompilePattern(`^\d{3}-\d{4}$`)
	require.NoError(t, err)

	assert.False(t, cp.Match("abc-defg"))
	assert.False(t, cp.Match("12-345"))
}

func TestCompiledPattern_Match_TableDriven(t *testing.T) {
	t.Parallel()
	cp, err := CompilePattern(`^[a-zA-Z][a-zA-Z0-9_]*$`)
	require.NoError(t, err)

	tests := []struct {
		input    string
		expected bool
	}{
		{"validName", true},
		{"Valid_Name_123", true},
		{"_invalid", false},
		{"123invalid", false},
		{"", false},
		{"a", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, cp.Match(tt.input))
		})
	}
}

// =============================================================================
// JSON Serialization Round-Trip Tests
// =============================================================================

func TestJSONSchema_Serialization_RoundTrip(t *testing.T) {
	t.Parallel()
	original := NewSchemaBuilder().
		Object().
		Property("name", NewSchemaBuilder().String().MinLength(1).MaxLength(50).Build()).
		Property("age", NewSchemaBuilder().Integer().Minimum(0).Maximum(150).Build()).
		Property("tags", NewSchemaBuilder().Array().Items(StringSchema()).MinItems(0).MaxItems(10).Build()).
		Required("name", "age").
		Description("Person schema").
		Build()

	// Serialize
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Deserialize
	restored, err := ParseSchema(data)
	require.NoError(t, err)

	assert.Equal(t, original.Type, restored.Type)
	assert.Equal(t, original.Description, restored.Description)
	assert.Len(t, restored.Properties, 3)
	assert.Equal(t, original.Required, restored.Required)
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkParseSchema(b *testing.B) {
	data := []byte(`{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name"]}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseSchema(data)
	}
}

func BenchmarkSchemaBuilder_Build(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewSchemaBuilder().
			Object().
			Property("name", StringSchema()).
			Property("age", IntegerSchema()).
			Required("name").
			Build()
	}
}

func BenchmarkCompiledPattern_Match(b *testing.B) {
	cp, _ := CompilePattern(`^[a-zA-Z][a-zA-Z0-9_]*$`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cp.Match("validIdentifier123")
	}
}
