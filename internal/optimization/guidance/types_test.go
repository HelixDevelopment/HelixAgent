package guidance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPromptTemplate(t *testing.T) {
	t.Parallel()
	template := &PromptTemplate{
		Name:        "test_template",
		Description: "A test template",
		Template:    "Hello {{name}}, your age is {{age}}",
		Variables: []TemplateVariable{
			{Name: "name", Required: true, Type: VariableTypeString},
			{Name: "age", Required: false, Type: VariableTypeNumber, Default: 0},
		},
	}

	assert.Equal(t, "test_template", template.Name)
	assert.Equal(t, "A test template", template.Description)
	assert.Len(t, template.Variables, 2)
	assert.True(t, template.Variables[0].Required)
}

func TestTemplateVariable(t *testing.T) {
	t.Parallel()
	variable := &TemplateVariable{
		Name:        "username",
		Description: "The user's name",
		Required:    true,
		Default:     "Guest",
		Type:        VariableTypeString,
	}

	assert.Equal(t, "username", variable.Name)
	assert.Equal(t, "The user's name", variable.Description)
	assert.True(t, variable.Required)
	assert.Equal(t, "Guest", variable.Default)
	assert.Equal(t, VariableTypeString, variable.Type)
}

func TestVariableTypes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, VariableType("string"), VariableTypeString)
	assert.Equal(t, VariableType("number"), VariableTypeNumber)
	assert.Equal(t, VariableType("boolean"), VariableTypeBoolean)
	assert.Equal(t, VariableType("list"), VariableTypeList)
	assert.Equal(t, VariableType("object"), VariableTypeObject)
}

func TestDefaultGenerationConfig(t *testing.T) {
	t.Parallel()
	config := DefaultGenerationConfig()

	assert.Equal(t, 0.7, config.Temperature)
	assert.Equal(t, 500, config.MaxTokens)
	assert.Equal(t, 1.0, config.TopP)
}

func TestGenerationConfig(t *testing.T) {
	t.Parallel()
	config := &GenerationConfig{
		Model:            "gpt-4",
		Provider:         "openai",
		Temperature:      0.8,
		MaxTokens:        1000,
		TopP:             0.9,
		TopK:             50,
		StopSequences:    []string{"\n", "###"},
		FrequencyPenalty: 0.5,
		PresencePenalty:  0.3,
		Seed:             12345,
	}

	assert.Equal(t, "gpt-4", config.Model)
	assert.Equal(t, "openai", config.Provider)
	assert.Equal(t, 0.8, config.Temperature)
	assert.Equal(t, 1000, config.MaxTokens)
	assert.Contains(t, config.StopSequences, "\n")
}

func TestValidationResult(t *testing.T) {
	t.Parallel()
	result := &ValidationResult{
		Valid:    false,
		Errors:   []string{"Invalid format", "Missing field"},
		Warnings: []string{"Deprecated format"},
		Details:  map[string]interface{}{"field": "name"},
	}

	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 2)
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Details, "field")
}

func TestGuidanceSession(t *testing.T) {
	t.Parallel()
	session := &GuidanceSession{
		ID:        "session-123",
		StartedAt: time.Now(),
		Config:    DefaultGenerationConfig(),
		History:   []GenerationRecord{},
		Context:   map[string]interface{}{"user": "test"},
	}

	assert.Equal(t, "session-123", session.ID)
	assert.NotNil(t, session.Config)
	assert.Empty(t, session.History)
	assert.Contains(t, session.Context, "user")
}

func TestGenerationRecord(t *testing.T) {
	t.Parallel()
	record := &GenerationRecord{
		Timestamp:   time.Now(),
		Prompt:      "Generate an email",
		Output:      "test@example.com",
		Constraints: []string{"email_format"},
		Valid:       true,
		Attempts:    2,
		LatencyMs:   150,
	}

	assert.NotZero(t, record.Timestamp)
	assert.Equal(t, "Generate an email", record.Prompt)
	assert.Equal(t, "test@example.com", record.Output)
	assert.True(t, record.Valid)
	assert.Equal(t, 2, record.Attempts)
}

func TestOutputModes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, OutputMode("text"), OutputModeText)
	assert.Equal(t, OutputMode("json"), OutputModeJSON)
	assert.Equal(t, OutputMode("xml"), OutputModeXML)
	assert.Equal(t, OutputMode("yaml"), OutputModeYAML)
	assert.Equal(t, OutputMode("markdown"), OutputModeMarkdown)
	assert.Equal(t, OutputMode("code"), OutputModeCode)
}

func TestOutputSpec(t *testing.T) {
	t.Parallel()
	spec := &OutputSpec{
		Mode:     OutputModeJSON,
		Schema:   map[string]interface{}{"type": "object"},
		Language: "go",
		Examples: []string{`{"key": "value"}`},
	}

	assert.Equal(t, OutputModeJSON, spec.Mode)
	assert.NotNil(t, spec.Schema)
	assert.Equal(t, "go", spec.Language)
	assert.Len(t, spec.Examples, 1)
}

func TestGuidanceError(t *testing.T) {
	t.Parallel()
	err := &GuidanceError{
		Code:        ErrorCodeConstraintViolation,
		Message:     "Output does not match pattern",
		Details:     map[string]interface{}{"pattern": "\\d+"},
		Recoverable: true,
	}

	assert.Equal(t, ErrorCodeConstraintViolation, err.Code)
	assert.Equal(t, "Output does not match pattern", err.Error())
	assert.True(t, err.Recoverable)
}

func TestErrorCodes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ErrorCode("constraint_violation"), ErrorCodeConstraintViolation)
	assert.Equal(t, ErrorCode("invalid_input"), ErrorCodeInvalidInput)
	assert.Equal(t, ErrorCode("generation_failed"), ErrorCodeGenerationFailed)
	assert.Equal(t, ErrorCode("timeout"), ErrorCodeTimeout)
	assert.Equal(t, ErrorCode("retry_exhausted"), ErrorCodeRetryExhausted)
	assert.Equal(t, ErrorCode("backend_error"), ErrorCodeBackendError)
}

func TestGuidanceMetrics(t *testing.T) {
	t.Parallel()
	metrics := &GuidanceMetrics{
		TotalGenerations:        100,
		SuccessfulGenerations:   85,
		FailedGenerations:       15,
		TotalRetries:            25,
		AverageAttempts:         1.3,
		AverageLatencyMs:        150.5,
		ConstraintViolationRate: 0.15,
		LastUpdated:             time.Now(),
	}

	assert.Equal(t, int64(100), metrics.TotalGenerations)
	assert.Equal(t, int64(85), metrics.SuccessfulGenerations)
	assert.Equal(t, 1.3, metrics.AverageAttempts)
	assert.Equal(t, 0.15, metrics.ConstraintViolationRate)
}

func TestConstraintSet(t *testing.T) {
	t.Parallel()
	emailConstraint := NewFormatConstraint(FormatEmail)
	lengthConstraint := NewLengthConstraint(5, 100, LengthUnitCharacters)

	set := NewConstraintSet("email_validation", CompositeModeAll, emailConstraint, lengthConstraint)

	assert.Equal(t, "email_validation", set.Name)
	assert.Equal(t, CompositeModeAll, set.Mode)
	assert.Len(t, set.Constraints, 2)

	composite := set.ToConstraint()
	assert.NotNil(t, composite)
	assert.Equal(t, ConstraintTypeComposite, composite.Type())
}

func TestDefaultValidationContext(t *testing.T) {
	t.Parallel()
	ctx := DefaultValidationContext()

	assert.True(t, ctx.Strict)
	assert.False(t, ctx.AllowPartial)
	assert.True(t, ctx.TrimWhitespace)
	assert.False(t, ctx.CaseInsensitive)
}

func TestDefaultRetryStrategy(t *testing.T) {
	t.Parallel()
	strategy := DefaultRetryStrategy()

	assert.Equal(t, 3, strategy.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, strategy.InitialDelay)
	assert.Equal(t, 5*time.Second, strategy.MaxDelay)
	assert.Equal(t, 2.0, strategy.BackoffMultiplier)
	assert.Equal(t, 0.1, strategy.JitterFactor)
}

func TestRetryStrategy_GetDelay(t *testing.T) {
	t.Parallel()
	strategy := &RetryStrategy{
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          1 * time.Second,
		BackoffMultiplier: 2.0,
	}

	delay1 := strategy.GetDelay(1)
	delay2 := strategy.GetDelay(2)
	delay3 := strategy.GetDelay(3)
	delay10 := strategy.GetDelay(10) // Should be capped at MaxDelay

	assert.Equal(t, 100*time.Millisecond, delay1)
	assert.Equal(t, 200*time.Millisecond, delay2)
	assert.Equal(t, 400*time.Millisecond, delay3)
	assert.Equal(t, 1*time.Second, delay10) // Capped
}

func TestDefaultCapabilities(t *testing.T) {
	t.Parallel()
	caps := DefaultCapabilities()

	assert.True(t, caps.SupportsRegex)
	assert.False(t, caps.SupportsGrammar)
	assert.True(t, caps.SupportsSchema)
	assert.True(t, caps.SupportsChoice)
	assert.Equal(t, 10, caps.MaxRetries)
	assert.Contains(t, caps.SupportedFormats, FormatJSON)
}

func TestPredefinedConstraints(t *testing.T) {
	t.Parallel()
	// Email
	assert.NoError(t, PredefinedConstraints.Email.Validate("test@example.com"))
	assert.Error(t, PredefinedConstraints.Email.Validate("invalid"))

	// URL
	assert.NoError(t, PredefinedConstraints.URL.Validate("https://example.com"))
	assert.Error(t, PredefinedConstraints.URL.Validate("invalid"))

	// UUID
	assert.NoError(t, PredefinedConstraints.UUID.Validate("550e8400-e29b-41d4-a716-446655440000"))
	assert.Error(t, PredefinedConstraints.UUID.Validate("invalid"))

	// YesNo
	assert.NoError(t, PredefinedConstraints.YesNo.Validate("yes"))
	assert.NoError(t, PredefinedConstraints.YesNo.Validate("no"))
	assert.Error(t, PredefinedConstraints.YesNo.Validate("maybe"))

	// TrueFalse
	assert.NoError(t, PredefinedConstraints.TrueFalse.Validate("true"))
	assert.NoError(t, PredefinedConstraints.TrueFalse.Validate("false"))
	assert.Error(t, PredefinedConstraints.TrueFalse.Validate("yes"))

	// Numeric
	assert.NoError(t, PredefinedConstraints.Numeric.Validate("123"))
	assert.NoError(t, PredefinedConstraints.Numeric.Validate("-45.67"))
	assert.Error(t, PredefinedConstraints.Numeric.Validate("abc"))

	// AlphaNumeric
	assert.NoError(t, PredefinedConstraints.AlphaNumeric.Validate("abc123"))
	assert.Error(t, PredefinedConstraints.AlphaNumeric.Validate("abc 123"))
	assert.Error(t, PredefinedConstraints.AlphaNumeric.Validate("abc-123"))

	// SingleWord
	assert.NoError(t, PredefinedConstraints.SingleWord.Validate("hello"))
	assert.Error(t, PredefinedConstraints.SingleWord.Validate("hello world"))

	// SingleSentence
	assert.NoError(t, PredefinedConstraints.SingleSentence.Validate("This is a sentence."))
	assert.Error(t, PredefinedConstraints.SingleSentence.Validate("First. Second."))
}
