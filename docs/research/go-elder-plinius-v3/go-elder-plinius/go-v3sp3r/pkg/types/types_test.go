package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandRequestValidateValid(t *testing.T) {
	opts := CommandRequest{
		Context:         "test",
		SafetyLevel:     "test",
		NaturalLanguage: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestCommandRequestValidateEmpty(t *testing.T) {
	opts := CommandRequest{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestCommandResultValidateValid(t *testing.T) {
	opts := CommandResult{
		SafetyWarning: "test",
		Description:   "test description",
		Command:       "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestCommandResultValidateEmpty(t *testing.T) {
	opts := CommandResult{}
	err := opts.Validate()
	assert.Error(t, err)
}
