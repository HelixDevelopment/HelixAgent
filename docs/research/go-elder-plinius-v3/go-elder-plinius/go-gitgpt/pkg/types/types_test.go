package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitOptionsValidateValid(t *testing.T) {
	opts := CommitOptions{
		Files: "test",
		Language: "test",
		Context: "test",
		Style: "test",
		Diff: "test",
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
		FocusAreas: "test",
		Language: "test",
		Diff: "test",
		Files: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestReviewOptionsValidateEmpty(t *testing.T) {
	opts := ReviewOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}
