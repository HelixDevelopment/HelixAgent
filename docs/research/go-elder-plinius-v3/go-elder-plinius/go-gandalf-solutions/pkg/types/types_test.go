package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevelSolutionValidateValid(t *testing.T) {
	opts := LevelSolution{
		Techniques: "test",
		Category: "test",
		Description: "test description",
		Difficulty: "test",
		SystemPromptLeak: "test systempromptleak",
		Solutions: "test",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestLevelSolutionValidateEmpty(t *testing.T) {
	opts := LevelSolution{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestAdventureSolutionValidateValid(t *testing.T) {
	opts := AdventureSolution{
		Description: "test description",
		Adventure: "test",
		Difficulty: "test",
		Solutions: "test",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestAdventureSolutionValidateEmpty(t *testing.T) {
	opts := AdventureSolution{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestPromptLeakValidateValid(t *testing.T) {
	opts := PromptLeak{
		Model: "gpt-4",
		Date: "test",
		LeakedContent: "test",
		ID: "test-id-123",
		ExtractionMethod: "test",
		Source: "test",
		Confidence: 0.95,
	}
	assert.NoError(t, opts.Validate())
}

func TestPromptLeakValidateEmpty(t *testing.T) {
	opts := PromptLeak{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestSearchOptionsValidateValid(t *testing.T) {
	opts := SearchOptions{
		Query: "test query",
		Techniques: "test",
		Difficulty: "test",
		Limit: 10,
		Categories: "test",
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

func TestPromptLeakValidateConfidenceRange(t *testing.T) {
	opts := PromptLeak{ID: "test", Confidence: 1.5}
	assert.Error(t, opts.Validate())
	opts.Confidence = -0.1
	assert.Error(t, opts.Validate())
}

func TestSearchOptionsValidateLimitNegative(t *testing.T) {
	opts := SearchOptions{Query: "test", Limit: -1}
	assert.Error(t, opts.Validate())
}
