package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentConfigValidateValid(t *testing.T) {
	opts := AgentConfig{
		Role:         "test",
		Model:        "gpt-4",
		Dependencies: "test",
		SystemPrompt: "test systemprompt",
		Outputs:      "test",
		Description:  "test description",
		Inputs:       "test",
		Name:         "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestAgentConfigValidateEmpty(t *testing.T) {
	opts := AgentConfig{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestAgentConfigDefaults(t *testing.T) {
	opts := AgentConfig{}
	opts.Description = "test"
	opts.Name = "test"
	opts.Defaults()
	assert.Equal(t, 0.7, opts.Temperature)
}

func TestTeamConfigValidateValid(t *testing.T) {
	opts := TeamConfig{
		Model:         "gpt-4",
		TeamName:      "Test TeamName",
		ExecutionMode: "test",
		Idea:          "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestTeamConfigValidateEmpty(t *testing.T) {
	opts := TeamConfig{}
	err := opts.Validate()
	assert.Error(t, err)
}
