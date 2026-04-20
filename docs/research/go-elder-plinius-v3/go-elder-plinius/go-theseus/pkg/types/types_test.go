package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentConfigValidateValid(t *testing.T) {
	opts := AgentConfig{
		Model: "gpt-4",
		MemoryType: "test",
		Tools: "test",
		Goals: "test",
		Description: "test description",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestAgentConfigValidateEmpty(t *testing.T) {
	opts := AgentConfig{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestAgentValidateValid(t *testing.T) {
	opts := Agent{
		Status: "test",
		CurrentTask: "test",
		ID: "test-id-123",
	}
	assert.NoError(t, opts.Validate())
}

func TestAgentValidateEmpty(t *testing.T) {
	opts := Agent{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestTaskEntryValidateValid(t *testing.T) {
	opts := TaskEntry{
		Result: "test",
		Status: "test",
		Timestamp: "test",
		Task: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestTaskEntryValidateEmpty(t *testing.T) {
	opts := TaskEntry{}
	err := opts.Validate()
	assert.Error(t, err)
}
