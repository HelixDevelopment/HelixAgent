package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChatRequestValidateValid(t *testing.T) {
	opts := ChatRequest{
		Models: "gpt-4",
		Mode: "test",
		Prompt: "test prompt",
		SystemPrompt: "test systemprompt",
	}
	assert.NoError(t, opts.Validate())
}

func TestChatRequestValidateEmpty(t *testing.T) {
	opts := ChatRequest{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestChatRequestDefaults(t *testing.T) {
	opts := ChatRequest{}
	opts.Prompt = "test"
	opts.Defaults()
	assert.Equal(t, 2048, opts.MaxTokens)
	assert.Equal(t, 0.7, opts.Temperature)
}

func TestModelResponseValidateValid(t *testing.T) {
	opts := ModelResponse{
		Model: "gpt-4",
		Response: "test",
		FinishReason: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestModelResponseValidateEmpty(t *testing.T) {
	opts := ModelResponse{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestParseltongueOptionsValidateValid(t *testing.T) {
	opts := ParseltongueOptions{
		Techniques: "test",
		Text: "test text",
	}
	assert.NoError(t, opts.Validate())
}

func TestParseltongueOptionsValidateEmpty(t *testing.T) {
	opts := ParseltongueOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestAutoTuneOptionsValidateValid(t *testing.T) {
	opts := AutoTuneOptions{
		Model: "gpt-4",
		Context: "test",
		Prompt: "test prompt",
	}
	assert.NoError(t, opts.Validate())
}

func TestAutoTuneOptionsValidateEmpty(t *testing.T) {
	opts := AutoTuneOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}
