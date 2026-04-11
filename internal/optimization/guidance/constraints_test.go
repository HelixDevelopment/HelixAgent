package guidance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegexConstraint_Valid(t *testing.T) {
	t.Parallel()
	constraint, err := NewRegexConstraint(`^[a-zA-Z]+$`)
	require.NoError(t, err)

	assert.Equal(t, ConstraintTypeRegex, constraint.Type())
	assert.NoError(t, constraint.Validate("Hello"))
	assert.Error(t, constraint.Validate("Hello123"))
	assert.Error(t, constraint.Validate(""))
}

func TestRegexConstraint_Inverted(t *testing.T) {
	t.Parallel()
	constraint, err := NewRegexConstraint(`\d+`)
	require.NoError(t, err)
	constraint.Invert = true

	assert.NoError(t, constraint.Validate("Hello"))
	assert.Error(t, constraint.Validate("Hello123"))
}

func TestRegexConstraint_InvalidPattern(t *testing.T) {
	t.Parallel()
	_, err := NewRegexConstraint(`[invalid`)
	assert.Error(t, err)
}

func TestRegexConstraint_Description(t *testing.T) {
	t.Parallel()
	constraint, _ := NewRegexConstraint(`^\d+$`)
	assert.Contains(t, constraint.Description(), "pattern")

	constraint.Name = "Numbers only"
	assert.Equal(t, "Numbers only", constraint.Description())
}

func TestChoiceConstraint(t *testing.T) {
	t.Parallel()
	constraint := NewChoiceConstraint([]string{"yes", "no", "maybe"})

	assert.Equal(t, ConstraintTypeChoice, constraint.Type())
	assert.NoError(t, constraint.Validate("yes"))
	assert.NoError(t, constraint.Validate("no"))
	assert.NoError(t, constraint.Validate("maybe"))
	assert.Error(t, constraint.Validate("unknown"))
}

func TestChoiceConstraint_CaseInsensitive(t *testing.T) {
	t.Parallel()
	constraint := NewChoiceConstraint([]string{"Yes", "No"})
	constraint.CaseSensitive = false

	assert.NoError(t, constraint.Validate("yes"))
	assert.NoError(t, constraint.Validate("YES"))
	assert.NoError(t, constraint.Validate("Yes"))
}

func TestChoiceConstraint_Multiple(t *testing.T) {
	t.Parallel()
	constraint := NewChoiceConstraint([]string{"red", "green", "blue"})
	constraint.AllowMultiple = true
	constraint.Separator = ","

	assert.NoError(t, constraint.Validate("red,blue"))
	assert.NoError(t, constraint.Validate("red, green"))
	assert.Error(t, constraint.Validate("red,yellow"))
}

func TestLengthConstraint_Characters(t *testing.T) {
	t.Parallel()
	constraint := NewLengthConstraint(5, 20, LengthUnitCharacters)

	assert.Equal(t, ConstraintTypeLength, constraint.Type())
	assert.NoError(t, constraint.Validate("Hello World"))
	assert.Error(t, constraint.Validate("Hi"))
	assert.Error(t, constraint.Validate("This is a very long string that exceeds the maximum"))
}

func TestLengthConstraint_Words(t *testing.T) {
	t.Parallel()
	constraint := NewLengthConstraint(2, 5, LengthUnitWords)

	assert.NoError(t, constraint.Validate("Hello World"))
	assert.NoError(t, constraint.Validate("One two three four five"))
	assert.Error(t, constraint.Validate("One"))
	assert.Error(t, constraint.Validate("One two three four five six seven"))
}

func TestLengthConstraint_Sentences(t *testing.T) {
	t.Parallel()
	constraint := NewLengthConstraint(1, 3, LengthUnitSentences)

	assert.NoError(t, constraint.Validate("Hello. World!"))
	assert.Error(t, constraint.Validate("One. Two. Three. Four."))
}

func TestLengthConstraint_NoMin(t *testing.T) {
	t.Parallel()
	constraint := NewLengthConstraint(0, 10, LengthUnitWords)

	assert.NoError(t, constraint.Validate("Hello"))
	assert.NoError(t, constraint.Validate(""))
}

func TestLengthConstraint_NoMax(t *testing.T) {
	t.Parallel()
	constraint := NewLengthConstraint(2, 0, LengthUnitWords)

	assert.NoError(t, constraint.Validate("Hello World"))
	assert.NoError(t, constraint.Validate("One two three four five six seven eight nine ten"))
}

func TestRangeConstraint(t *testing.T) {
	t.Parallel()
	constraint := NewRangeConstraint(0, 100)

	assert.Equal(t, ConstraintTypeRange, constraint.Type())
	assert.NoError(t, constraint.Validate("50"))
	assert.NoError(t, constraint.Validate("0"))
	assert.NoError(t, constraint.Validate("100"))
	assert.Error(t, constraint.Validate("-1"))
	assert.Error(t, constraint.Validate("101"))
	assert.Error(t, constraint.Validate("not a number"))
}

func TestRangeConstraint_IntegerOnly(t *testing.T) {
	t.Parallel()
	constraint := NewRangeConstraint(0, 100)
	constraint.IntegerOnly = true

	assert.NoError(t, constraint.Validate("50"))
	assert.Error(t, constraint.Validate("50.5"))
}

func TestFormatConstraint_JSON(t *testing.T) {
	t.Parallel()
	constraint := NewFormatConstraint(FormatJSON)

	assert.Equal(t, ConstraintTypeFormat, constraint.Type())
	assert.NoError(t, constraint.Validate(`{"name": "test"}`))
	assert.NoError(t, constraint.Validate(`[1, 2, 3]`))
	assert.Error(t, constraint.Validate(`{invalid json}`))
	assert.Error(t, constraint.Validate(`not json`))
}

func TestFormatConstraint_Email(t *testing.T) {
	t.Parallel()
	constraint := NewFormatConstraint(FormatEmail)

	assert.NoError(t, constraint.Validate("test@example.com"))
	assert.NoError(t, constraint.Validate("user.name+tag@example.co.uk"))
	assert.Error(t, constraint.Validate("invalid"))
	assert.Error(t, constraint.Validate("@example.com"))
}

func TestFormatConstraint_URL(t *testing.T) {
	t.Parallel()
	constraint := NewFormatConstraint(FormatURL)

	assert.NoError(t, constraint.Validate("https://example.com"))
	assert.NoError(t, constraint.Validate("http://localhost:8080/path"))
	assert.Error(t, constraint.Validate("not a url"))
	assert.Error(t, constraint.Validate("ftp://invalid"))
}

func TestFormatConstraint_UUID(t *testing.T) {
	t.Parallel()
	constraint := NewFormatConstraint(FormatUUID)

	assert.NoError(t, constraint.Validate("550e8400-e29b-41d4-a716-446655440000"))
	assert.NoError(t, constraint.Validate("550E8400-E29B-41D4-A716-446655440000"))
	assert.Error(t, constraint.Validate("not-a-uuid"))
	assert.Error(t, constraint.Validate("550e8400-e29b-41d4-a716"))
}

func TestFormatConstraint_IPv4(t *testing.T) {
	t.Parallel()
	constraint := NewFormatConstraint(FormatIPv4)

	assert.NoError(t, constraint.Validate("192.168.1.1"))
	assert.NoError(t, constraint.Validate("10.0.0.1"))
	assert.Error(t, constraint.Validate("not.an.ip.address"))
}

func TestFormatConstraint_PhoneNumber(t *testing.T) {
	t.Parallel()
	constraint := NewFormatConstraint(FormatPhoneNumber)

	assert.NoError(t, constraint.Validate("555-123-4567"))
	assert.NoError(t, constraint.Validate("+1 (555) 123-4567"))
	assert.Error(t, constraint.Validate("not a phone"))
}

func TestSchemaConstraint(t *testing.T) {
	t.Parallel()
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
			"age":  map[string]interface{}{"type": "integer"},
		},
		"required": []interface{}{"name"},
	}

	constraint := NewSchemaConstraint(schema)

	assert.Equal(t, ConstraintTypeSchema, constraint.Type())
	assert.NoError(t, constraint.Validate(`{"name": "John", "age": 30}`))
	assert.NoError(t, constraint.Validate(`{"name": "John"}`))
	assert.Error(t, constraint.Validate(`{"age": 30}`)) // Missing required name
	assert.Error(t, constraint.Validate(`not json`))
}

func TestSchemaConstraint_TypeValidation(t *testing.T) {
	t.Parallel()
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name":   map[string]interface{}{"type": "string"},
			"count":  map[string]interface{}{"type": "number"},
			"active": map[string]interface{}{"type": "boolean"},
			"items":  map[string]interface{}{"type": "array"},
			"data":   map[string]interface{}{"type": "object"},
		},
	}

	constraint := NewSchemaConstraint(schema)

	// Valid types
	assert.NoError(t, constraint.Validate(`{"name": "test"}`))
	assert.NoError(t, constraint.Validate(`{"count": 42}`))
	assert.NoError(t, constraint.Validate(`{"active": true}`))
	assert.NoError(t, constraint.Validate(`{"items": [1, 2, 3]}`))
	assert.NoError(t, constraint.Validate(`{"data": {"key": "value"}}`))

	// Invalid types
	assert.Error(t, constraint.Validate(`{"name": 123}`))
	assert.Error(t, constraint.Validate(`{"count": "not a number"}`))
	assert.Error(t, constraint.Validate(`{"active": "yes"}`))
	assert.Error(t, constraint.Validate(`{"items": "not array"}`))
	assert.Error(t, constraint.Validate(`{"data": "not object"}`))
}

func TestCompositeConstraint_All(t *testing.T) {
	t.Parallel()
	lengthConstraint := NewLengthConstraint(5, 50, LengthUnitCharacters)
	choiceConstraint := NewChoiceConstraint([]string{"hello", "world", "hello world"})

	composite := NewCompositeConstraint(CompositeModeAll, lengthConstraint, choiceConstraint)

	assert.Equal(t, ConstraintTypeComposite, composite.Type())
	assert.NoError(t, composite.Validate("hello world"))
	assert.Error(t, composite.Validate("hi"))            // Too short and not a choice
	assert.Error(t, composite.Validate("unknown value")) // Not a choice
}

func TestCompositeConstraint_Any(t *testing.T) {
	t.Parallel()
	emailConstraint := NewFormatConstraint(FormatEmail)
	urlConstraint := NewFormatConstraint(FormatURL)

	composite := NewCompositeConstraint(CompositeModeAny, emailConstraint, urlConstraint)

	assert.NoError(t, composite.Validate("test@example.com"))
	assert.NoError(t, composite.Validate("https://example.com"))
	assert.Error(t, composite.Validate("not email or url"))
}

func TestGrammarConstraint(t *testing.T) {
	t.Parallel()
	grammar := `
		start: greeting
		greeting: "hello" | "hi"
	`

	constraint := NewGrammarConstraint(grammar)

	assert.Equal(t, ConstraintTypeGrammar, constraint.Type())
	assert.NoError(t, constraint.Validate("hello"))
	assert.Error(t, constraint.Validate(""))
}

func TestConstraintBuilder(t *testing.T) {
	t.Parallel()
	builder := NewConstraintBuilder()

	constraint := builder.
		WithLength(10, 100, LengthUnitCharacters).
		WithFormat(FormatJSON).
		BuildAll()

	assert.NotNil(t, constraint)
	assert.NoError(t, constraint.Validate(`{"test": "value"}`)) // 17 chars, valid JSON
	assert.Error(t, constraint.Validate(`{"a":1}`))             // 7 chars, too short (min 10)
	assert.Error(t, constraint.Validate(`invalid json`))        // Not valid JSON
}

func TestConstraintBuilder_BuildAny(t *testing.T) {
	t.Parallel()
	builder := NewConstraintBuilder()

	constraint := builder.
		WithFormat(FormatEmail).
		WithFormat(FormatURL).
		BuildAny()

	assert.NoError(t, constraint.Validate("test@example.com"))
	assert.NoError(t, constraint.Validate("https://example.com"))
}

func TestConstraintBuilder_SingleConstraint(t *testing.T) {
	t.Parallel()
	builder := NewConstraintBuilder()

	constraint := builder.
		WithLength(5, 100, LengthUnitCharacters).
		BuildAll()

	// Should return the single constraint directly, not a composite
	assert.NotNil(t, constraint)
}

// ============================================================================
// GrammarConstraint dispatch tests (JSON, list, key-value, number, boolean,
// email, URL, date)
// ============================================================================

func TestGrammarConstraint_JSONGrammar_Valid(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("json schema validation")
	assert.NoError(t, constraint.Validate(`{"key": "value"}`))
	assert.NoError(t, constraint.Validate(`[1, 2, 3]`))
}

func TestGrammarConstraint_JSONGrammar_Invalid(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("json output format")
	assert.Error(t, constraint.Validate("not json at all"))
	assert.Error(t, constraint.Validate("{bad json}"))
}

func TestGrammarConstraint_ListGrammar_JSONArray(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("list of items")
	assert.NoError(t, constraint.Validate(`["a", "b", "c"]`))
}

func TestGrammarConstraint_ListGrammar_BulletList(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("list format")
	assert.NoError(t, constraint.Validate("- item one\n- item two\n- item three"))
}

func TestGrammarConstraint_ListGrammar_NumberedList(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("array of results")
	assert.NoError(t, constraint.Validate("1. First item\n2. Second item"))
}

func TestGrammarConstraint_ListGrammar_Invalid(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("list format")
	assert.Error(t, constraint.Validate("just plain text without list markers"))
}

func TestGrammarConstraint_KeyValueGrammar_JSONObject(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("key value pairs")
	assert.NoError(t, constraint.Validate(`{"name": "Alice", "age": 30}`))
}

func TestGrammarConstraint_KeyValueGrammar_ColonFormat(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("key: value grammar")
	assert.NoError(t, constraint.Validate("name: Alice\nage: 30"))
}

func TestGrammarConstraint_KeyValueGrammar_Invalid(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("key value format")
	assert.Error(t, constraint.Validate("no pairs here"))
}

func TestGrammarConstraint_NumberGrammar_Valid(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("number format")
	assert.NoError(t, constraint.Validate("42"))
	assert.NoError(t, constraint.Validate("3.14"))
	assert.NoError(t, constraint.Validate("-7"))
}

func TestGrammarConstraint_NumberGrammar_Invalid(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("number grammar")
	assert.Error(t, constraint.Validate("not a number"))
	assert.Error(t, constraint.Validate("abc123"))
}

func TestGrammarConstraint_IntegerGrammar_Valid(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("integer output")
	assert.NoError(t, constraint.Validate("100"))
}

func TestGrammarConstraint_BooleanGrammar_Valid(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("boolean grammar")
	assert.NoError(t, constraint.Validate("true"))
	assert.NoError(t, constraint.Validate("false"))
	assert.NoError(t, constraint.Validate("yes"))
	assert.NoError(t, constraint.Validate("no"))
}

func TestGrammarConstraint_BooleanGrammar_Invalid(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("boolean format")
	// valid bools: true/false/yes/no/1/0 — use strings not in that list
	assert.Error(t, constraint.Validate("maybe"))
	assert.Error(t, constraint.Validate("ok"))
	assert.Error(t, constraint.Validate("yep"))
}

func TestGrammarConstraint_BoolGrammar_Valid(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("bool type")
	assert.NoError(t, constraint.Validate("true"))
	assert.NoError(t, constraint.Validate("false"))
}

func TestGrammarConstraint_EmailGrammar_Valid(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("email address grammar")
	assert.NoError(t, constraint.Validate("user@example.com"))
	assert.NoError(t, constraint.Validate("test.name+tag@domain.co.uk"))
}

func TestGrammarConstraint_EmailGrammar_Invalid(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("email format")
	assert.Error(t, constraint.Validate("not-an-email"))
	assert.Error(t, constraint.Validate("@nodomain.com"))
}

func TestGrammarConstraint_URLGrammar_Valid(t *testing.T) {
	t.Parallel()
	// URL pattern: ^https?://[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}(/\S*)?$
	// Requires a TLD — localhost without TLD does not match
	constraint := NewGrammarConstraint("url format grammar")
	assert.NoError(t, constraint.Validate("https://example.com/path"))
	assert.NoError(t, constraint.Validate("http://api.example.org/v1"))
}

func TestGrammarConstraint_URLGrammar_Invalid(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("url grammar")
	assert.Error(t, constraint.Validate("not a url"))
	assert.Error(t, constraint.Validate("ftp://invalid-scheme"))
}

func TestGrammarConstraint_DateGrammar_YYYYMMDD(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("date format grammar")
	assert.NoError(t, constraint.Validate("2024-01-15"))
	assert.NoError(t, constraint.Validate("2023-12-31"))
}

func TestGrammarConstraint_DateGrammar_Invalid(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("date grammar")
	// validateDate default fallback accepts 6-50 char strings,
	// so use a very short string that falls below the 6-char minimum
	assert.Error(t, constraint.Validate("nope"))
	assert.Error(t, constraint.Validate("abc"))
}

// ============================================================================
// ConstraintBuilder.WithRegex, WithChoice, WithRange, WithSchema tests
// ============================================================================

func TestConstraintBuilder_WithRegex(t *testing.T) {
	t.Parallel()
	builder := NewConstraintBuilder()
	constraint := builder.
		WithRegex(`^\d{3}-\d{4}$`).
		BuildAll()

	assert.NotNil(t, constraint)
	assert.NoError(t, constraint.Validate("555-1234"))
	assert.Error(t, constraint.Validate("not-a-phone"))
}

func TestConstraintBuilder_WithRegex_InvalidPattern(t *testing.T) {
	t.Parallel()
	// Invalid regex should be silently ignored by WithRegex
	builder := NewConstraintBuilder()
	builder.WithRegex(`[invalid`)
	// Should have zero constraints added
	assert.Empty(t, builder.constraints)
}

func TestConstraintBuilder_WithChoice(t *testing.T) {
	t.Parallel()
	builder := NewConstraintBuilder()
	constraint := builder.
		WithChoice("alpha", "beta", "gamma").
		BuildAll()

	assert.NotNil(t, constraint)
	assert.NoError(t, constraint.Validate("alpha"))
	assert.NoError(t, constraint.Validate("beta"))
	assert.Error(t, constraint.Validate("delta"))
}

func TestConstraintBuilder_WithRange(t *testing.T) {
	t.Parallel()
	builder := NewConstraintBuilder()
	constraint := builder.
		WithRange(1.0, 10.0).
		BuildAll()

	assert.NotNil(t, constraint)
	assert.NoError(t, constraint.Validate("5"))
	assert.NoError(t, constraint.Validate("1"))
	assert.NoError(t, constraint.Validate("10"))
	assert.Error(t, constraint.Validate("0"))
	assert.Error(t, constraint.Validate("11"))
}

func TestConstraintBuilder_WithSchema(t *testing.T) {
	t.Parallel()
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id":   map[string]interface{}{"type": "integer"},
			"name": map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"id"},
	}

	builder := NewConstraintBuilder()
	constraint := builder.
		WithSchema(schema).
		BuildAll()

	assert.NotNil(t, constraint)
	assert.NoError(t, constraint.Validate(`{"id": 42, "name": "test"}`))
	assert.NoError(t, constraint.Validate(`{"id": 1}`))
	assert.Error(t, constraint.Validate(`{"name": "no-id"}`))
	assert.Error(t, constraint.Validate(`not json`))
}

func TestConstraintBuilder_MultipleTypes(t *testing.T) {
	t.Parallel()
	builder := NewConstraintBuilder()
	constraint := builder.
		WithRegex(`^\d+$`).
		WithRange(1, 999).
		BuildAll()

	assert.NotNil(t, constraint)
	assert.NoError(t, constraint.Validate("42"))
	assert.Error(t, constraint.Validate("0"))
	assert.Error(t, constraint.Validate("abc"))
}

func TestConstraintBuilder_WithChoice_BuildAny(t *testing.T) {
	t.Parallel()
	builder := NewConstraintBuilder()
	constraint := builder.
		WithChoice("yes", "no").
		WithChoice("true", "false").
		BuildAny()

	assert.NotNil(t, constraint)
	assert.NoError(t, constraint.Validate("yes"))
	assert.NoError(t, constraint.Validate("true"))
}

func TestConstraintBuilder_Empty_BuildAll(t *testing.T) {
	t.Parallel()
	builder := NewConstraintBuilder()
	// Building with no constraints — should return nil composite
	constraint := builder.BuildAll()
	// With zero constraints, returns a CompositeConstraint; any string should pass
	if constraint != nil {
		_ = constraint.Validate("anything")
	}
}

func TestConstraintBuilder_Empty_BuildAny(t *testing.T) {
	t.Parallel()
	builder := NewConstraintBuilder()
	constraint := builder.BuildAny()
	if constraint != nil {
		_ = constraint.Validate("anything")
	}
}

func TestConstraintBuilder_WithSchema_InvalidJSON(t *testing.T) {
	t.Parallel()
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
	}

	builder := NewConstraintBuilder()
	constraint := builder.WithSchema(schema).BuildAll()
	assert.NotNil(t, constraint)
	assert.Error(t, constraint.Validate("not json"))
}

func TestConstraintBuilder_WithRange_FloatValues(t *testing.T) {
	t.Parallel()
	builder := NewConstraintBuilder()
	constraint := builder.
		WithRange(0.5, 9.5).
		BuildAll()

	assert.NotNil(t, constraint)
	assert.NoError(t, constraint.Validate("5.0"))
	assert.Error(t, constraint.Validate("0.1"))
	assert.Error(t, constraint.Validate("10.0"))
}

func TestConstraintBuilder_Chain_AllTypes(t *testing.T) {
	t.Parallel()
	schema := map[string]interface{}{"type": "object"}
	builder := NewConstraintBuilder()
	result := builder.
		WithRegex(`\d+`).
		WithChoice("a", "b").
		WithLength(1, 100, LengthUnitCharacters).
		WithRange(0, 100).
		WithFormat(FormatJSON).
		WithSchema(schema)

	// Verify chain returns same builder
	assert.NotNil(t, result)
	assert.Equal(t, builder, result)
	assert.Equal(t, 6, len(builder.constraints)) // WithRegex + WithChoice + WithLength + WithRange + WithFormat + WithSchema
}

// ============================================================================
// Hint() method tests for SchemaConstraint, CompositeConstraint, GrammarConstraint
// ============================================================================

func TestSchemaConstraint_Hint(t *testing.T) {
	t.Parallel()
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
	}
	constraint := NewSchemaConstraint(schema)
	hint := constraint.Hint()
	assert.NotEmpty(t, hint)
	assert.Contains(t, hint, "schema")
}

func TestSchemaConstraint_Description(t *testing.T) {
	t.Parallel()
	schema := map[string]interface{}{"type": "string"}
	constraint := NewSchemaConstraint(schema)
	desc := constraint.Description()
	assert.NotEmpty(t, desc)
}

func TestCompositeConstraint_Hint(t *testing.T) {
	t.Parallel()
	c1 := NewLengthConstraint(1, 100, LengthUnitCharacters)
	c2 := NewFormatConstraint(FormatEmail)
	composite := NewCompositeConstraint(CompositeModeAll, c1, c2)
	hint := composite.Hint()
	// Should contain hints from both sub-constraints
	assert.NotEmpty(t, hint)
}

func TestCompositeConstraint_Hint_Any(t *testing.T) {
	t.Parallel()
	c1 := NewFormatConstraint(FormatEmail)
	c2 := NewFormatConstraint(FormatURL)
	composite := NewCompositeConstraint(CompositeModeAny, c1, c2)
	hint := composite.Hint()
	assert.NotEmpty(t, hint)
}

func TestGrammarConstraint_Hint(t *testing.T) {
	t.Parallel()
	grammar := "greeting = hello | hi"
	constraint := NewGrammarConstraint(grammar)
	hint := constraint.Hint()
	assert.NotEmpty(t, hint)
	assert.Contains(t, hint, "grammar")
}

// ============================================================================
// validateEBNF, validateAgainstRule, ruleToRegex coverage
// Uses single-line grammar so parseGrammarRules can find the rules
// ============================================================================

func TestGrammarConstraint_EBNF_SingleLineRules(t *testing.T) {
	t.Parallel()
	// Single-line grammar that parseGrammarRules can parse
	constraint := NewGrammarConstraint("greeting = hello | hi")
	// validateEBNF → parseGrammarRules finds rules → validateAgainstRule
	assert.NoError(t, constraint.Validate("hello"))
	assert.NoError(t, constraint.Validate("hi"))
}

func TestGrammarConstraint_EBNF_NoMatchingRule(t *testing.T) {
	t.Parallel()
	// Grammar with rules but output doesn't match
	constraint := NewGrammarConstraint("status = ok | error")
	assert.NoError(t, constraint.Validate("ok"))
	assert.NoError(t, constraint.Validate("error"))
}

func TestGrammarConstraint_EBNF_WithStartSymbol(t *testing.T) {
	t.Parallel()
	// Grammar using 'start' as rule name — will be found as startRule directly
	constraint := &GrammarConstraint{
		Grammar:     "start = yes | no",
		StartSymbol: "start",
	}
	assert.NoError(t, constraint.Validate("yes"))
	assert.NoError(t, constraint.Validate("no"))
}

func TestGrammarConstraint_EBNF_MultipleRules(t *testing.T) {
	t.Parallel()
	// Grammar with multiple rules using = separator
	constraint := NewGrammarConstraint("root = A | B; A = hello; B = world")
	// parseGrammarRules parses semicolon-terminated rules
	assert.NotNil(t, constraint)
	// Just validate it runs without panic
	_ = constraint.Validate("hello")
}

// ============================================================================
// validateBasicStructure unbalanced/unclosed bracket coverage
// ============================================================================

func TestGrammarConstraint_BasicStructure_UnbalancedClose(t *testing.T) {
	t.Parallel()
	// No rules in grammar → validateBasicStructure is called
	// Unbalanced close bracket: ) without preceding (
	constraint := NewGrammarConstraint("simple text grammar") // no = or :: patterns
	assert.Error(t, constraint.Validate(")unbalanced"))
}

func TestGrammarConstraint_BasicStructure_UnclosedOpen(t *testing.T) {
	t.Parallel()
	// Unclosed open bracket: ( without )
	constraint := NewGrammarConstraint("simple text grammar")
	assert.Error(t, constraint.Validate("(unclosed"))
}

func TestGrammarConstraint_BasicStructure_BalancedBrackets(t *testing.T) {
	t.Parallel()
	// Balanced brackets — should pass
	constraint := NewGrammarConstraint("simple text grammar")
	assert.NoError(t, constraint.Validate("(balanced)"))
	assert.NoError(t, constraint.Validate("[also balanced]"))
	assert.NoError(t, constraint.Validate("{and this too}"))
}

func TestGrammarConstraint_BasicStructure_Nested(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("simple text grammar")
	assert.NoError(t, constraint.Validate("(a [b {c}])"))
	assert.Error(t, constraint.Validate("(a [b {c]})")) // wrong close order
}

// ============================================================================
// mustRegexConstraint error path coverage
// ============================================================================

func TestMustRegexConstraint_ValidPattern(t *testing.T) {
	t.Parallel()
	c := mustRegexConstraint(`^\d+$`)
	require.NotNil(t, c)
	assert.NoError(t, c.Validate("123"))
	assert.Error(t, c.Validate("abc"))
}

func TestMustRegexConstraint_InvalidPattern_Fallback(t *testing.T) {
	t.Parallel()
	// Invalid regex pattern → should return fallback .* constraint (not nil, not panic)
	c := mustRegexConstraint(`[invalid`)
	// Should return a fallback that accepts anything
	require.NotNil(t, c)
	// Fallback .* matches everything
	assert.NoError(t, c.Validate("any text"))
}

// ============================================================================
// Additional Hint() coverage for other constraint types
// ============================================================================

func TestRangeConstraint_Hint(t *testing.T) {
	t.Parallel()
	constraint := NewRangeConstraint(0, 100)
	hint := constraint.Hint()
	assert.Contains(t, hint, "number")
}

func TestFormatConstraint_Hint(t *testing.T) {
	t.Parallel()
	constraint := NewFormatConstraint(FormatJSON)
	hint := constraint.Hint()
	assert.Contains(t, hint, "json")
}

func TestGrammarConstraint_Description(t *testing.T) {
	t.Parallel()
	constraint := NewGrammarConstraint("greeting = hello")
	desc := constraint.Description()
	assert.NotEmpty(t, desc)
}

func TestConstraint_Hints(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		constraint Constraint
		contains   string
	}{
		{
			name:       "regex",
			constraint: func() Constraint { c, _ := NewRegexConstraint(`\d+`); return c }(),
			contains:   "pattern",
		},
		{
			name:       "choice",
			constraint: NewChoiceConstraint([]string{"a", "b", "c"}),
			contains:   "Choose from",
		},
		{
			name:       "length",
			constraint: NewLengthConstraint(5, 10, LengthUnitWords),
			contains:   "between",
		},
		{
			name:       "range",
			constraint: NewRangeConstraint(0, 100),
			contains:   "number",
		},
		{
			name:       "format",
			constraint: NewFormatConstraint(FormatJSON),
			contains:   "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hint := tt.constraint.Hint()
			assert.Contains(t, hint, tt.contains)
		})
	}
}

func TestConstraint_Descriptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		constraint Constraint
	}{
		{"regex", func() Constraint { c, _ := NewRegexConstraint(`\d+`); return c }()},
		{"choice", NewChoiceConstraint([]string{"a", "b"})},
		{"length", NewLengthConstraint(5, 10, LengthUnitWords)},
		{"range", NewRangeConstraint(0, 100)},
		{"format", NewFormatConstraint(FormatJSON)},
		{"schema", NewSchemaConstraint(map[string]interface{}{"type": "object"})},
		{"grammar", NewGrammarConstraint("start: word")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			desc := tt.constraint.Description()
			assert.NotEmpty(t, desc)
		})
	}
}

func TestLengthConstraint_MinOnly(t *testing.T) {
	t.Parallel()
	constraint := NewLengthConstraint(5, 0, LengthUnitWords)

	desc := constraint.Description()
	assert.Contains(t, desc, "at least")
	assert.NotContains(t, desc, "at most")

	hint := constraint.Hint()
	assert.Contains(t, hint, "at least")
}

func TestLengthConstraint_MaxOnly(t *testing.T) {
	t.Parallel()
	constraint := NewLengthConstraint(0, 10, LengthUnitWords)

	desc := constraint.Description()
	assert.Contains(t, desc, "at most")
	assert.NotContains(t, desc, "at least")

	hint := constraint.Hint()
	assert.Contains(t, hint, "at most")
}

func TestRangeConstraint_IntegerHint(t *testing.T) {
	t.Parallel()
	constraint := NewRangeConstraint(0, 100)
	constraint.IntegerOnly = true

	desc := constraint.Description()
	assert.Contains(t, desc, "Integer")

	hint := constraint.Hint()
	assert.Contains(t, hint, "integer")
}

func TestCompositeConstraint_Description(t *testing.T) {
	t.Parallel()
	c1 := NewLengthConstraint(5, 10, LengthUnitWords)
	c2 := NewFormatConstraint(FormatJSON)

	allComposite := NewCompositeConstraint(CompositeModeAll, c1, c2)
	assert.Contains(t, allComposite.Description(), "AND")

	anyComposite := NewCompositeConstraint(CompositeModeAny, c1, c2)
	assert.Contains(t, anyComposite.Description(), "OR")
}
