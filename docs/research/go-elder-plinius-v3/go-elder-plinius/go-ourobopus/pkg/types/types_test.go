package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetaPromptValidateValid(t *testing.T) {
	opts := MetaPrompt{
		ReflectionQuestions: "test",
		Description: "test description",
		Category: "test",
		ID: "test-id-123",
		BasePrompt: "test baseprompt",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestMetaPromptValidateEmpty(t *testing.T) {
	opts := MetaPrompt{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestIterationResultValidateValid(t *testing.T) {
	opts := IterationResult{
		Output: "test",
		Improvements: "test",
		Prompt: "test prompt",
	}
	assert.NoError(t, opts.Validate())
}

func TestIterationResultValidateEmpty(t *testing.T) {
	opts := IterationResult{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestRefinementConfigValidateValid(t *testing.T) {
	opts := RefinementConfig{
		Model: "gpt-4",
		EvaluationCriteria: "test",
		InitialPrompt: "test initialprompt",
	}
	assert.NoError(t, opts.Validate())
}

func TestRefinementConfigValidateEmpty(t *testing.T) {
	opts := RefinementConfig{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestSelfReflectionValidateConfidenceRange(t *testing.T) {
	opts := SelfReflection{ID: "test", Confidence: 1.5}
	assert.Error(t, opts.Validate())
	opts.Confidence = -0.1
	assert.Error(t, opts.Validate())
}
