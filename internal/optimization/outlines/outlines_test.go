package outlines

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaBuilder(t *testing.T) {
	schema := NewSchemaBuilder().
		Object().
		Property("name", StringSchema()).
		Property("age", IntegerSchema()).
		Property("email", PatternString(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)).
		Required("name", "age").
		Build()

	assert.Equal(t, "object", schema.Type)
	assert.Len(t, schema.Properties, 3)
	assert.Contains(t, schema.Required, "name")
	assert.Contains(t, schema.Required, "age")
}

func TestSchemaBuilder_Array(t *testing.T) {
	schema := NewSchemaBuilder().
		Array().
		Items(StringSchema()).
		MinItems(1).
		MaxItems(10).
		Build()

	assert.Equal(t, "array", schema.Type)
	assert.NotNil(t, schema.Items)
	assert.Equal(t, 1, *schema.MinItems)
	assert.Equal(t, 10, *schema.MaxItems)
}

func TestSchemaBuilder_String(t *testing.T) {
	schema := NewSchemaBuilder().
		String().
		MinLength(5).
		MaxLength(100).
		Pattern(`^[A-Z]`).
		Build()

	assert.Equal(t, "string", schema.Type)
	assert.Equal(t, 5, *schema.MinLength)
	assert.Equal(t, 100, *schema.MaxLength)
	assert.Equal(t, `^[A-Z]`, schema.Pattern)
}

func TestSchemaBuilder_Number(t *testing.T) {
	schema := NewSchemaBuilder().
		Number().
		Minimum(0).
		Maximum(100).
		Build()

	assert.Equal(t, "number", schema.Type)
	assert.Equal(t, float64(0), *schema.Minimum)
	assert.Equal(t, float64(100), *schema.Maximum)
}

func TestParseSchema(t *testing.T) {
	jsonSchema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"}
		},
		"required": ["name"]
	}`

	schema, err := ParseSchema([]byte(jsonSchema))
	require.NoError(t, err)

	assert.Equal(t, "object", schema.Type)
	assert.Len(t, schema.Properties, 2)
	assert.True(t, schema.IsRequired("name"))
	assert.False(t, schema.IsRequired("age"))
}

func TestSchemaValidator_ValidObject(t *testing.T) {
	schema := ObjectSchema(map[string]*JSONSchema{
		"name": StringSchema(),
		"age":  IntegerSchema(),
	}, "name", "age")

	validator, err := NewSchemaValidator(schema)
	require.NoError(t, err)

	result := validator.Validate(`{"name": "John", "age": 30}`)

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestSchemaValidator_MissingRequired(t *testing.T) {
	schema := ObjectSchema(map[string]*JSONSchema{
		"name": StringSchema(),
		"age":  IntegerSchema(),
	}, "name", "age")

	validator, err := NewSchemaValidator(schema)
	require.NoError(t, err)

	result := validator.Validate(`{"name": "John"}`)

	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "required property missing")
}

func TestSchemaValidator_InvalidType(t *testing.T) {
	schema := ObjectSchema(map[string]*JSONSchema{
		"name": StringSchema(),
		"age":  IntegerSchema(),
	})

	validator, err := NewSchemaValidator(schema)
	require.NoError(t, err)

	result := validator.Validate(`{"name": "John", "age": "thirty"}`)

	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "expected integer")
}

func TestSchemaValidator_StringConstraints(t *testing.T) {
	minLen := 5
	maxLen := 10
	schema := &JSONSchema{
		Type:      "string",
		MinLength: &minLen,
		MaxLength: &maxLen,
	}

	validator, err := NewSchemaValidator(schema)
	require.NoError(t, err)

	// Too short
	result := validator.Validate(`"abc"`)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0].Message, "at least 5 characters")

	// Too long
	result = validator.Validate(`"abcdefghijk"`)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0].Message, "at most 10 characters")

	// Valid
	result = validator.Validate(`"abcdef"`)
	assert.True(t, result.Valid)
}

func TestSchemaValidator_NumberConstraints(t *testing.T) {
	min := float64(0)
	max := float64(100)
	schema := &JSONSchema{
		Type:    "number",
		Minimum: &min,
		Maximum: &max,
	}

	validator, err := NewSchemaValidator(schema)
	require.NoError(t, err)

	// Too low
	result := validator.Validate(`-1`)
	assert.False(t, result.Valid)

	// Too high
	result = validator.Validate(`101`)
	assert.False(t, result.Valid)

	// Valid
	result = validator.Validate(`50`)
	assert.True(t, result.Valid)
}

func TestSchemaValidator_Pattern(t *testing.T) {
	schema := PatternString(`^[A-Z][a-z]+$`)

	validator, err := NewSchemaValidator(schema)
	require.NoError(t, err)

	// Doesn't match
	result := validator.Validate(`"john"`)
	assert.False(t, result.Valid)

	// Matches
	result = validator.Validate(`"John"`)
	assert.True(t, result.Valid)
}

func TestSchemaValidator_Enum(t *testing.T) {
	schema := EnumSchema("red", "green", "blue")

	validator, err := NewSchemaValidator(schema)
	require.NoError(t, err)

	// Invalid value
	result := validator.Validate(`"yellow"`)
	assert.False(t, result.Valid)

	// Valid value
	result = validator.Validate(`"red"`)
	assert.True(t, result.Valid)
}

func TestSchemaValidator_Array(t *testing.T) {
	minItems := 1
	maxItems := 3
	schema := &JSONSchema{
		Type:     "array",
		Items:    IntegerSchema(),
		MinItems: &minItems,
		MaxItems: &maxItems,
	}

	validator, err := NewSchemaValidator(schema)
	require.NoError(t, err)

	// Empty array (too few)
	result := validator.Validate(`[]`)
	assert.False(t, result.Valid)

	// Too many items
	result = validator.Validate(`[1, 2, 3, 4]`)
	assert.False(t, result.Valid)

	// Invalid item type
	result = validator.Validate(`[1, "two", 3]`)
	assert.False(t, result.Valid)

	// Valid
	result = validator.Validate(`[1, 2, 3]`)
	assert.True(t, result.Valid)
}

func TestSchemaValidator_NestedObject(t *testing.T) {
	schema := ObjectSchema(map[string]*JSONSchema{
		"user": ObjectSchema(map[string]*JSONSchema{
			"name":  StringSchema(),
			"email": StringSchema(),
		}, "name"),
		"active": BooleanSchema(),
	}, "user")

	validator, err := NewSchemaValidator(schema)
	require.NoError(t, err)

	// Valid nested
	result := validator.Validate(`{"user": {"name": "John", "email": "john@example.com"}, "active": true}`)
	assert.True(t, result.Valid)

	// Missing nested required
	result = validator.Validate(`{"user": {"email": "john@example.com"}}`)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0].Path, "user.name")
}

func TestSchemaValidator_Formats(t *testing.T) {
	tests := []struct {
		format string
		valid  string
		invalid string
	}{
		{"email", `"test@example.com"`, `"not-an-email"`},
		{"uri", `"https://example.com"`, `"not-a-uri"`},
		{"date", `"2024-01-15"`, `"15-01-2024"`},
		{"uuid", `"550e8400-e29b-41d4-a716-446655440000"`, `"not-a-uuid"`},
		{"ipv4", `"192.168.1.1"`, `"999.999.999.999"`},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			schema := &JSONSchema{Type: "string", Format: tt.format}
			validator, err := NewSchemaValidator(schema)
			require.NoError(t, err)

			result := validator.Validate(tt.valid)
			assert.True(t, result.Valid, "Expected %s to be valid for format %s", tt.valid, tt.format)

			result = validator.Validate(tt.invalid)
			assert.False(t, result.Valid, "Expected %s to be invalid for format %s", tt.invalid, tt.format)
		})
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain json",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "json with text",
			input:    `Here is the result: {"key": "value"} and some more text`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "json in markdown",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "json array",
			input:    `[1, 2, 3]`,
			expected: `[1, 2, 3]`,
		},
		{
			name:     "nested json",
			input:    `{"outer": {"inner": "value"}}`,
			expected: `{"outer": {"inner": "value"}}`,
		},
		{
			name:     "no json",
			input:    `Just plain text`,
			expected: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindMatchingBrace(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{`{}`, 1},
		{`{"key": "value"}`, 15},
		{`{"nested": {"key": "value"}}`, 27},
		{`{"array": [1, 2, 3]}`, 19},
		{`{`, -1},
		{`{"key": "value with { brace"}`, 28},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := findMatchingBrace(tt.input, '{', '}')
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock LLM provider for testing
type mockProvider struct {
	responses []string
	callCount int
}

func (m *mockProvider) Complete(ctx context.Context, prompt string) (string, error) {
	if m.callCount < len(m.responses) {
		response := m.responses[m.callCount]
		m.callCount++
		return response, nil
	}
	return "", nil
}

func TestStructuredGenerator_ValidResponse(t *testing.T) {
	schema := ObjectSchema(map[string]*JSONSchema{
		"name": StringSchema(),
		"age":  IntegerSchema(),
	}, "name", "age")

	provider := &mockProvider{
		responses: []string{`{"name": "John", "age": 30}`},
	}

	generator, err := NewStructuredGenerator(provider, schema, nil)
	require.NoError(t, err)

	result, err := generator.Generate(context.Background(), "Generate a person")
	require.NoError(t, err)

	assert.True(t, result.Valid)
	assert.Equal(t, 0, result.Retries)

	data := result.ParsedData.(map[string]interface{})
	assert.Equal(t, "John", data["name"])
}

func TestStructuredGenerator_RetryOnInvalid(t *testing.T) {
	schema := ObjectSchema(map[string]*JSONSchema{
		"name": StringSchema(),
		"age":  IntegerSchema(),
	}, "name", "age")

	provider := &mockProvider{
		responses: []string{
			`{"name": "John"}`,                    // Missing age
			`{"name": "John", "age": "thirty"}`,   // Wrong type
			`{"name": "John", "age": 30}`,         // Valid
		},
	}

	generator, err := NewStructuredGenerator(provider, schema, nil)
	require.NoError(t, err)

	result, err := generator.Generate(context.Background(), "Generate a person")
	require.NoError(t, err)

	assert.True(t, result.Valid)
	assert.Equal(t, 2, result.Retries)
}

func TestStructuredGenerator_StrictMode(t *testing.T) {
	schema := ObjectSchema(map[string]*JSONSchema{
		"name": StringSchema(),
	}, "name")

	config := DefaultGeneratorConfig()
	config.StrictMode = true

	provider := &mockProvider{
		responses: []string{`not valid json`},
	}

	generator, err := NewStructuredGenerator(provider, schema, config)
	require.NoError(t, err)

	result, err := generator.Generate(context.Background(), "Generate")
	require.NoError(t, err)

	assert.False(t, result.Valid)
	assert.Equal(t, 0, result.Retries)
}

func TestRegexGenerator(t *testing.T) {
	provider := &mockProvider{
		responses: []string{`ABC123`},
	}

	generator, err := NewRegexGenerator(provider, `^[A-Z]{3}[0-9]{3}$`, nil)
	require.NoError(t, err)

	result, err := generator.Generate(context.Background(), "Generate a code")
	require.NoError(t, err)

	assert.True(t, result.Valid)
	assert.Equal(t, "ABC123", result.Content)
}

func TestChoiceGenerator(t *testing.T) {
	provider := &mockProvider{
		responses: []string{`red`},
	}

	generator := NewChoiceGenerator(provider, []string{"red", "green", "blue"}, nil)

	result, err := generator.Generate(context.Background(), "Pick a color")
	require.NoError(t, err)

	assert.True(t, result.Valid)
	assert.Equal(t, "red", result.Content)
}

func TestChoiceGenerator_CaseInsensitive(t *testing.T) {
	provider := &mockProvider{
		responses: []string{`RED`},
	}

	generator := NewChoiceGenerator(provider, []string{"red", "green", "blue"}, nil)

	result, err := generator.Generate(context.Background(), "Pick a color")
	require.NoError(t, err)

	assert.True(t, result.Valid)
	assert.Equal(t, "red", result.Content) // Returns canonical form
}

func TestCompiledPattern(t *testing.T) {
	pattern, err := CompilePattern(`^[A-Z][a-z]+$`)
	require.NoError(t, err)

	assert.True(t, pattern.Match("John"))
	assert.False(t, pattern.Match("john"))
	assert.False(t, pattern.Match("JOHN"))
}

func TestValidate_Convenience(t *testing.T) {
	schema := StringSchema()

	result := Validate(`"hello"`, schema)
	assert.True(t, result.Valid)

	result = Validate(`123`, schema)
	assert.False(t, result.Valid)
}

func TestSchemaValidator_OneOf(t *testing.T) {
	schema := &JSONSchema{
		OneOf: []*JSONSchema{
			StringSchema(),
			IntegerSchema(),
		},
	}

	validator, err := NewSchemaValidator(schema)
	require.NoError(t, err)

	// String is valid
	result := validator.Validate(`"hello"`)
	assert.True(t, result.Valid)

	// Integer is valid
	result = validator.Validate(`42`)
	assert.True(t, result.Valid)

	// Array is not valid
	result = validator.Validate(`[1, 2, 3]`)
	assert.False(t, result.Valid)
}

func TestSchemaValidator_AnyOf(t *testing.T) {
	schema := &JSONSchema{
		AnyOf: []*JSONSchema{
			{Type: "string", MinLength: intPtr(5)},
			{Type: "integer"},
		},
	}

	validator, err := NewSchemaValidator(schema)
	require.NoError(t, err)

	// Long string is valid
	result := validator.Validate(`"hello world"`)
	assert.True(t, result.Valid)

	// Short string is invalid
	result = validator.Validate(`"hi"`)
	assert.False(t, result.Valid)

	// Integer is valid
	result = validator.Validate(`42`)
	assert.True(t, result.Valid)
}

func intPtr(i int) *int {
	return &i
}

func BenchmarkSchemaValidator(b *testing.B) {
	schema := ObjectSchema(map[string]*JSONSchema{
		"name":  StringSchema(),
		"age":   IntegerSchema(),
		"email": PatternString(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
		"tags":  ArraySchema(StringSchema()),
	}, "name", "age")

	validator, _ := NewSchemaValidator(schema)
	jsonStr := `{"name": "John Doe", "age": 30, "email": "john@example.com", "tags": ["developer", "go"]}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.Validate(jsonStr)
	}
}

func BenchmarkExtractJSON(b *testing.B) {
	input := `Here is your result: {"name": "John", "age": 30, "items": [1, 2, 3]} and some more text after`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractJSON(input)
	}
}
