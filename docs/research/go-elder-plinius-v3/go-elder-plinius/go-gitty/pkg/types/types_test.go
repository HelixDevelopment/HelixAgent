package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitOptionsValidateValid(t *testing.T) {
	opts := CommitOptions{
		Files:    "test",
		Language: "test",
		Context:  "test",
		Style:    "test",
		Diff:     "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestCommitOptionsValidateEmpty(t *testing.T) {
	opts := CommitOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestReviewOptionsValidateValid(t *testing.T) {
	opts := ReviewOptions{
		Files:      "test",
		Language:   "test",
		FocusAreas: "test",
		Severity:   "test",
		Diff:       "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestReviewOptionsValidateEmpty(t *testing.T) {
	opts := ReviewOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestSuggestionValidateValid(t *testing.T) {
	opts := Suggestion{
		CodeExample: "test",
		Confidence:  0.95,
		Description: "test description",
		Category:    "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestSuggestionValidateEmpty(t *testing.T) {
	opts := Suggestion{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestSuggestionValidateConfidenceRange(t *testing.T) {
	opts := Suggestion{ID: "test", Confidence: 1.5}
	assert.Error(t, opts.Validate())
	opts.Confidence = -0.1
	assert.Error(t, opts.Validate())
}
