package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromptEntryValidateValid(t *testing.T) {
	opts := PromptEntry{
		Model:      "gpt-4",
		ID:         "test-id-123",
		Confidence: 0.95,
		PromptText: "test prompttext",
		Date:       "test",
		Version:    "test",
		Source:     "test",
		Tags:       "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestPromptEntryValidateEmpty(t *testing.T) {
	opts := PromptEntry{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestSearchOptionsValidateValid(t *testing.T) {
	opts := SearchOptions{
		MinConfidence: 0.95,
		Versions:      "test",
		Models:        "gpt-4",
		Limit:         10,
		Query:         "test query",
		Tags:          "test",
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

func TestPromptEntryValidateConfidenceRange(t *testing.T) {
	opts := PromptEntry{ID: "test", Confidence: 1.5}
	assert.Error(t, opts.Validate())
	opts.Confidence = -0.1
	assert.Error(t, opts.Validate())
}

func TestSearchOptionsValidateLimitNegative(t *testing.T) {
	opts := SearchOptions{Query: "test", Limit: -1}
	assert.Error(t, opts.Validate())
}
