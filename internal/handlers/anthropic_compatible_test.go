package handlers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTranslateAnthropicToOpenAI_BasicShape covers the happy path for
// Finding #20: a string-content user message with a string system
// prompt should produce a valid OpenAIChatRequest with the same model,
// max_tokens, and message ordering.
func TestTranslateAnthropicToOpenAI_BasicShape(t *testing.T) {
	in := &AnthropicMessageRequest{
		Model:     "claude-3-5-sonnet-20240620",
		System:    "You are a helpful assistant.",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: "Reply with just PONG."},
		},
	}

	out, err := translateAnthropicToOpenAI(in)
	require.NoError(t, err)
	assert.Equal(t, "claude-3-5-sonnet-20240620", out.Model)
	assert.Equal(t, 1024, out.MaxTokens)
	assert.False(t, out.Stream)
	require.Len(t, out.Messages, 2)
	assert.Equal(t, "system", out.Messages[0].Role)
	assert.Equal(t, "You are a helpful assistant.", out.Messages[0].Content)
	assert.Equal(t, "user", out.Messages[1].Role)
	assert.Equal(t, "Reply with just PONG.", out.Messages[1].Content)
}

// TestTranslateAnthropicToOpenAI_ContentBlocks covers the case where
// `content` is an array of typed blocks (Anthropic's other supported
// shape). Text blocks are concatenated with newlines; tool_use returns
// an explicit error so the client knows to use OpenAI tools instead.
func TestTranslateAnthropicToOpenAI_ContentBlocks(t *testing.T) {
	t.Run("multi-text concatenated", func(t *testing.T) {
		// JSON-decoded content arrives as []interface{} of map[string]interface{}.
		var content any
		require.NoError(t, json.Unmarshal([]byte(`[
			{"type":"text","text":"First line."},
			{"type":"text","text":"Second line."}
		]`), &content))
		in := &AnthropicMessageRequest{
			Model:     "claude-3",
			MaxTokens: 256,
			Messages:  []AnthropicMessage{{Role: "user", Content: content}},
		}
		out, err := translateAnthropicToOpenAI(in)
		require.NoError(t, err)
		require.Len(t, out.Messages, 1)
		assert.Equal(t, "First line.\nSecond line.", out.Messages[0].Content)
	})

	t.Run("tool_use block rejected", func(t *testing.T) {
		var content any
		require.NoError(t, json.Unmarshal([]byte(`[
			{"type":"tool_use","id":"x","name":"y","input":{}}
		]`), &content))
		in := &AnthropicMessageRequest{
			Model:     "claude-3",
			MaxTokens: 256,
			Messages:  []AnthropicMessage{{Role: "assistant", Content: content}},
		}
		_, err := translateAnthropicToOpenAI(in)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tool_use")
	})
}

// TestTranslateAnthropicToOpenAI_SystemAsBlocks covers the case where
// the `system` field arrives as an array of text blocks rather than a
// flat string.
func TestTranslateAnthropicToOpenAI_SystemAsBlocks(t *testing.T) {
	var sys any
	require.NoError(t, json.Unmarshal([]byte(`[
		{"type":"text","text":"You are helpful."},
		{"type":"text","text":"Be concise."}
	]`), &sys))
	in := &AnthropicMessageRequest{
		Model:     "claude-3",
		MaxTokens: 256,
		System:    sys,
		Messages:  []AnthropicMessage{{Role: "user", Content: "Hi"}},
	}
	out, err := translateAnthropicToOpenAI(in)
	require.NoError(t, err)
	require.Len(t, out.Messages, 2)
	assert.Equal(t, "system", out.Messages[0].Role)
	assert.Equal(t, "You are helpful.\nBe concise.", out.Messages[0].Content)
}

// TestTranslateOpenAIToAnthropic_HappyPath covers the response-side
// translation: the OpenAI completion JSON produced by ChatCompletions
// is re-shaped into the Anthropic message envelope with content blocks,
// stop_reason, and usage tokens populated.
func TestTranslateOpenAIToAnthropic_HappyPath(t *testing.T) {
	openaiJSON := []byte(`{
		"id": "chatcmpl-abc",
		"model": "helixagent-ensemble",
		"choices": [{
			"message": {"role": "assistant", "content": "PONG"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 12, "completion_tokens": 1}
	}`)

	got := translateOpenAIToAnthropic(openaiJSON, "claude-3-5-sonnet-20240620")
	assert.Equal(t, "chatcmpl-abc", got.ID)
	assert.Equal(t, "message", got.Type)
	assert.Equal(t, "assistant", got.Role)
	assert.Equal(t, "helixagent-ensemble", got.Model)
	assert.Equal(t, "end_turn", got.StopReason)
	require.Len(t, got.Content, 1)
	assert.Equal(t, "text", got.Content[0].Type)
	assert.Equal(t, "PONG", got.Content[0].Text)
	assert.Equal(t, 12, got.Usage.InputTokens)
	assert.Equal(t, 1, got.Usage.OutputTokens)
}

// TestMapFinishReason covers the OpenAI → Anthropic stop-reason
// vocabulary mapping.
func TestMapFinishReason(t *testing.T) {
	cases := []struct{ in, want string }{
		{"stop", "end_turn"},
		{"", "end_turn"},
		{"length", "max_tokens"},
		{"tool_calls", "tool_use"},
		{"function_call", "tool_use"},
		{"weird_unknown", "end_turn"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, mapFinishReason(c.in), "mapFinishReason(%q)", c.in)
	}
}

// TestTranslateOpenAIToAnthropic_EmptyChoicesFallsBackToRequestedModel
// verifies that even a degenerate upstream response yields a valid
// Anthropic envelope (so Claude Code doesn't crash on a parse error).
func TestTranslateOpenAIToAnthropic_EmptyChoicesFallsBackToRequestedModel(t *testing.T) {
	got := translateOpenAIToAnthropic([]byte(`{}`), "claude-3-5-sonnet")
	assert.Equal(t, "claude-3-5-sonnet", got.Model)
	assert.Equal(t, "message", got.Type)
	assert.Equal(t, "assistant", got.Role)
	require.Len(t, got.Content, 1)
	assert.Equal(t, "", got.Content[0].Text)
	assert.NotEmpty(t, got.ID, "ID should fall back to a generated msg_<ts> value")
	assert.True(t, len(got.ID) > 4 && got.ID[:4] == "msg_",
		"generated ID should be msg_-prefixed, got %q", got.ID)
}
