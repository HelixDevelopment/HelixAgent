package guidance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLLMBackend is a mock LLM backend for testing.
type MockLLMBackend struct {
	responses []string
	callCount int
	shouldErr bool
}

func (m *MockLLMBackend) Complete(ctx context.Context, prompt string) (string, error) {
	if m.shouldErr {
		return "", errors.New("mock error")
	}
	if m.callCount < len(m.responses) {
		response := m.responses[m.callCount]
		m.callCount++
		return response, nil
	}
	return "default response", nil
}

func (m *MockLLMBackend) CompleteWithHint(ctx context.Context, prompt string, hint string) (string, error) {
	return m.Complete(ctx, prompt)
}

func TestConstrainedGenerator_Generate(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"test@example.com"},
	}

	generator := NewConstrainedGenerator(backend, nil)
	constraint := NewFormatConstraint(FormatEmail)

	result, err := generator.Generate(context.Background(), "Generate an email", constraint)

	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Equal(t, "test@example.com", result.Output)
	assert.Equal(t, 1, result.Attempts)
}

func TestConstrainedGenerator_GenerateWithRetry(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"invalid", "still invalid", "test@example.com"},
	}

	generator := NewConstrainedGenerator(backend, &GeneratorConfig{
		DefaultMaxRetries:      5,
		RetryDelay:             1 * time.Millisecond,
		IncludeConstraintHints: true,
	})

	constraint := NewFormatConstraint(FormatEmail)

	result, err := generator.GenerateWithRetry(context.Background(), "Generate an email", constraint, 5)

	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Equal(t, "test@example.com", result.Output)
	assert.Equal(t, 3, result.Attempts)
}

func TestConstrainedGenerator_GenerateRetryExceeded(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"invalid1", "invalid2", "invalid3"},
	}

	config := DefaultGeneratorConfig()
	config.RetryDelay = 1 * time.Millisecond

	generator := NewConstrainedGenerator(backend, config)
	constraint := NewFormatConstraint(FormatEmail)

	result, err := generator.GenerateWithRetry(context.Background(), "Generate an email", constraint, 3)

	assert.Error(t, err)
	assert.False(t, result.Valid)
	assert.Equal(t, 3, result.Attempts)
	assert.NotEmpty(t, result.ValidationErrors)
}

func TestConstrainedGenerator_GenerateBackendError(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		shouldErr: true,
	}

	generator := NewConstrainedGenerator(backend, &GeneratorConfig{
		DefaultMaxRetries: 2,
		RetryDelay:        1 * time.Millisecond,
	})
	constraint := NewFormatConstraint(FormatEmail)

	result, err := generator.GenerateWithRetry(context.Background(), "Generate an email", constraint, 2)

	assert.Error(t, err)
	assert.False(t, result.Valid)
}

func TestConstrainedGenerator_GenerateContextCancelled(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"invalid", "invalid", "invalid"},
	}

	generator := NewConstrainedGenerator(backend, &GeneratorConfig{
		DefaultMaxRetries: 5,
		RetryDelay:        100 * time.Millisecond,
	})
	constraint := NewFormatConstraint(FormatEmail)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := generator.GenerateWithRetry(ctx, "Generate an email", constraint, 5)

	assert.Error(t, err)
}

func TestDefaultGeneratorConfig(t *testing.T) {
	t.Parallel()
	config := DefaultGeneratorConfig()

	assert.Equal(t, 3, config.DefaultMaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.RetryDelay)
	assert.True(t, config.IncludeConstraintHints)
}

func TestGenerationResult(t *testing.T) {
	t.Parallel()
	result := &GenerationResult{
		Output:   "test output",
		Valid:    true,
		Attempts: 2,
		ValidationErrors: []string{
			"first attempt failed",
		},
		Metadata: &GenerationMetadata{
			Model:          "gpt-4",
			Provider:       "openai",
			TokensUsed:     100,
			LatencyMs:      150,
			ConstraintType: ConstraintTypeRegex,
			Timestamp:      time.Now(),
		},
	}

	assert.Equal(t, "test output", result.Output)
	assert.True(t, result.Valid)
	assert.Equal(t, 2, result.Attempts)
	assert.Len(t, result.ValidationErrors, 1)
	assert.NotNil(t, result.Metadata)
	assert.Equal(t, "gpt-4", result.Metadata.Model)
}

func TestTemplatedGenerator_GenerateFromTemplate(t *testing.T) {
	t.Parallel()
	// Note: Placeholders are sorted alphabetically, so "age" comes before "name"
	backend := &MockLLMBackend{
		responses: []string{"30", "John"},
	}

	generator := NewConstrainedGenerator(backend, &GeneratorConfig{
		RetryDelay: 1 * time.Millisecond,
	})
	templatedGen := NewTemplatedGenerator(generator)

	template := &Template{
		Text: "Name: {{name}}, Age: {{age}}",
		Placeholders: map[string]Constraint{
			"name": NewLengthConstraint(1, 50, LengthUnitCharacters),
			"age":  NewLengthConstraint(1, 3, LengthUnitCharacters),
		},
	}

	result, err := templatedGen.GenerateFromTemplate(context.Background(), template)

	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Contains(t, result.FilledTemplate, "John")
	assert.Contains(t, result.Values, "name")
}

func TestSelectionGenerator_Select(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"Python"},
	}

	generator := NewSelectionGenerator(backend)
	options := []string{"Python", "JavaScript", "Go", "Rust"}

	result, err := generator.Select(context.Background(), "Best language for scripting:", options)

	require.NoError(t, err)
	assert.Equal(t, "Python", result.Selected)
	assert.Equal(t, 0, result.Index)
}

func TestSelectionGenerator_SelectContains(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"I would choose Python for this task"},
	}

	generator := NewSelectionGenerator(backend)
	options := []string{"Python", "JavaScript", "Go"}

	result, err := generator.Select(context.Background(), "Best language:", options)

	require.NoError(t, err)
	assert.Equal(t, "Python", result.Selected)
}

func TestSelectionGenerator_SelectMultiple(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"Python, Go"},
	}

	generator := NewSelectionGenerator(backend)
	options := []string{"Python", "JavaScript", "Go", "Rust"}

	result, err := generator.SelectMultiple(context.Background(), "Top 2 languages:", options, 2)

	require.NoError(t, err)
	assert.Len(t, result.Selected, 2)
	assert.Contains(t, result.Selected, "Python")
	assert.Contains(t, result.Selected, "Go")
}

func TestSelectionGenerator_SelectError(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		shouldErr: true,
	}

	generator := NewSelectionGenerator(backend)

	_, err := generator.Select(context.Background(), "Select:", []string{"a", "b"})

	assert.Error(t, err)
}

func TestStructuredGenerator_GenerateJSON(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{`{"name": "Test", "value": 42}`},
	}

	generator := NewConstrainedGenerator(backend, &GeneratorConfig{
		RetryDelay: 1 * time.Millisecond,
	})
	structuredGen := NewStructuredGenerator(generator)

	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name":  map[string]interface{}{"type": "string"},
			"value": map[string]interface{}{"type": "number"},
		},
	}

	result, err := structuredGen.GenerateJSON(context.Background(), "Generate a test object", schema)

	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Contains(t, result.Output, "name")
}

func TestStructuredGenerator_GenerateList(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"Item 1", "Item 2", "Item 3"},
	}

	generator := NewConstrainedGenerator(backend, &GeneratorConfig{
		RetryDelay: 1 * time.Millisecond,
	})
	structuredGen := NewStructuredGenerator(generator)

	constraint := NewLengthConstraint(1, 100, LengthUnitCharacters)

	result, err := structuredGen.GenerateList(context.Background(), "Generate items", constraint, 3)

	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Len(t, result.Items, 3)
}

func TestGuidedCompletion_CompleteUntil(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"Hello, this is a test output."},
	}

	config := &GuidedCompletionConfig{
		MaxTokens:   100,
		Temperature: 0.7,
	}

	completion := NewGuidedCompletion(backend, config)

	result, err := completion.CompleteUntil(context.Background(), "Generate text", func(s string) bool {
		return len(s) > 10
	})

	require.NoError(t, err)
	assert.True(t, result.Complete)
	assert.NotEmpty(t, result.Output)
}

func TestGuidedCompletion_CompleteWithPrefix(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"Answer: The solution is 42."},
	}

	completion := NewGuidedCompletion(backend, nil)

	result, err := completion.CompleteWithPrefix(context.Background(), "What is the answer?", "Answer:")

	require.NoError(t, err)
	assert.True(t, result.Complete)
	assert.True(t, len(result.Output) > 0)
}

func TestGuidedCompletion_CompleteWithSuffix(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"The end result is positive"},
	}

	completion := NewGuidedCompletion(backend, nil)

	result, err := completion.CompleteWithSuffix(context.Background(), "Generate conclusion", "THE END")

	require.NoError(t, err)
	assert.True(t, result.Complete)
}

func TestGuidedCompletion_Error(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		shouldErr: true,
	}

	completion := NewGuidedCompletion(backend, nil)

	_, err := completion.CompleteUntil(context.Background(), "Generate", nil)

	assert.Error(t, err)
}
