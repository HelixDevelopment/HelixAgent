package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDebateConfigValidateValid(t *testing.T) {
	opts := DebateConfig{
		Format: "test",
		SystemPromptB: "test systempromptb",
		Topic: "test",
		ModelB: "gpt-4",
		JudgeModel: "gpt-4",
		SystemPromptA: "test systemprompta",
		ModelA: "gpt-4",
	}
	assert.NoError(t, opts.Validate())
}

func TestDebateConfigValidateEmpty(t *testing.T) {
	opts := DebateConfig{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestCollaborationConfigValidateValid(t *testing.T) {
	opts := CollaborationConfig{
		Mode: "test",
		ModelA: "gpt-4",
		ModelB: "gpt-4",
		Task: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestCollaborationConfigValidateEmpty(t *testing.T) {
	opts := CollaborationConfig{}
	err := opts.Validate()
	assert.Error(t, err)
}
