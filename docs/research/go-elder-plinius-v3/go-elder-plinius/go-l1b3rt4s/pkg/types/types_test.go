package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJailbreakPromptValidateValid(t *testing.T) {
	opts := JailbreakPrompt{
		DateAdded: "test",
		Category: "test",
		ID: "test-id-123",
		TargetModels: "gpt-4",
		Description: "test description",
		PromptTemplate: "test prompttemplate",
		Source: "test",
		Tags: "test",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestJailbreakPromptValidateEmpty(t *testing.T) {
	opts := JailbreakPrompt{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestPromptTemplateValidateValid(t *testing.T) {
	opts := PromptTemplate{
		Category: "test",
		Template: "test",
		ID: "test-id-123",
		Variables: "test",
		TargetModel: "gpt-4",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestPromptTemplateValidateEmpty(t *testing.T) {
	opts := PromptTemplate{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestSearchOptionsValidateValid(t *testing.T) {
	opts := SearchOptions{
		Limit: 10,
		Query: "test query",
		Categories: "test",
		Tags: "test",
		TargetModels: "gpt-4",
	}
	assert.NoError(t, opts.Validate())
}

func TestSearchOptionsValidateEmpty(t *testing.T) {
	opts := SearchOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestSearchOptionsDefaults(t *testing.T) {
	opts := SearchOptions{}
	opts.Query = "test"
	opts.Defaults()
	assert.Equal(t, 50, opts.Limit)
}

func TestTestResultValidateValid(t *testing.T) {
	opts := TestResult{
		Model: "gpt-4",
		Response: "test",
		TestedAt: "test",
		PromptID: "test-promptid-123",
	}
	assert.NoError(t, opts.Validate())
}

func TestTestResultValidateEmpty(t *testing.T) {
	opts := TestResult{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestSafetyCheckOptionsValidateValid(t *testing.T) {
	opts := SafetyCheckOptions{
		Model: "gpt-4",
		Prompt: "test prompt",
		CheckTypes: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestSafetyCheckOptionsValidateEmpty(t *testing.T) {
	opts := SafetyCheckOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestSearchOptionsValidateLimitNegative(t *testing.T) {
	opts := SearchOptions{Query: "test", Limit: -1}
	assert.Error(t, opts.Validate())
}
