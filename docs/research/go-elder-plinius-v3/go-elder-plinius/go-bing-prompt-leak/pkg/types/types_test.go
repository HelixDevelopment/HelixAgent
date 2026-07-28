package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLeakEntryValidateValid(t *testing.T) {
	opts := LeakEntry{
		ID:               "test-id-123",
		Confidence:       0.95,
		Date:             "test",
		Version:          "test",
		LeakedContent:    "test",
		ExtractionMethod: "test",
		Source:           "test",
		Tags:             "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestLeakEntryValidateEmpty(t *testing.T) {
	opts := LeakEntry{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestTechniqueEntryValidateValid(t *testing.T) {
	opts := TechniqueEntry{
		ModelTarget: "gpt-4",
		Steps:       "test",
		Description: "test description",
		Category:    "test",
		Name:        "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestTechniqueEntryValidateEmpty(t *testing.T) {
	opts := TechniqueEntry{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestSearchOptionsValidateValid(t *testing.T) {
	opts := SearchOptions{
		Versions: "test",
		Limit:    10,
		Query:    "test query",
		Methods:  "test",
		Tags:     "test",
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

func TestLeakEntryValidateConfidenceRange(t *testing.T) {
	opts := LeakEntry{ID: "test", Confidence: 1.5}
	assert.Error(t, opts.Validate())
	opts.Confidence = -0.1
	assert.Error(t, opts.Validate())
}

func TestSearchOptionsValidateLimitNegative(t *testing.T) {
	opts := SearchOptions{Query: "test", Limit: -1}
	assert.Error(t, opts.Validate())
}
