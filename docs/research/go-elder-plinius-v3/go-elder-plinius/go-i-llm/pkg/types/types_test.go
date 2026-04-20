package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConversationPatternValidateValid(t *testing.T) {
	opts := ConversationPattern{
		Description: "test description",
		Category: "test",
		Template: "test",
		ID: "test-id-123",
		Variables: "test",
		Example: "test",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestConversationPatternValidateEmpty(t *testing.T) {
	opts := ConversationPattern{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestAgentConfigValidateValid(t *testing.T) {
	opts := AgentConfig{
		Model: "gpt-4",
		Name: "Test Name",
		SystemPrompt: "test systemprompt",
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
	opts.Name = "test"
	opts.Defaults()
	assert.Equal(t, 0.7, opts.Temperature)
}

func TestToolValidateValid(t *testing.T) {
	opts := Tool{
		Parameters: "test",
		Description: "test description",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestToolValidateEmpty(t *testing.T) {
	opts := Tool{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestPromptChainValidateValid(t *testing.T) {
	opts := PromptChain{
		Description: "test description",
		Category: "test",
		ID: "test-id-123",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestPromptChainValidateEmpty(t *testing.T) {
	opts := PromptChain{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestChainStepValidateValid(t *testing.T) {
	opts := ChainStep{
		PromptTemplate: "test prompttemplate",
		Condition: "test",
		OutputKey: "test",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestChainStepValidateEmpty(t *testing.T) {
	opts := ChainStep{}
	err := opts.Validate()
	assert.Error(t, err)
}
