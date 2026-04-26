package handlers

import (
	"testing"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertSingleResponseToOpenAI_PreservesToolCalls covers the
// CONST-032 reproduction guard for the OpenCode tool-call hang.
//
// Pre-fix: convertSingleResponseToOpenAI built the OpenAIMessage with
// only Role + Content, dropping resp.ToolCalls on the floor. Responses
// with finish_reason="tool_calls" reached clients with an empty/missing
// tool_calls array — the client knew a tool was wanted but couldn't
// see which one, and hung waiting for follow-up that never came.
func TestConvertSingleResponseToOpenAI_PreservesToolCalls(t *testing.T) {
	t.Parallel()
	h := &UnifiedHandler{}

	resp := &models.LLMResponse{
		ID:           "test-id-1",
		Content:      "",
		FinishReason: "tool_calls",
		TokensUsed:   100,
		ToolCalls: []models.ToolCall{
			{
				ID:   "call_abc123",
				Type: "function",
				Function: models.ToolCallFunction{
					Name:      "read_file",
					Arguments: `{"path":"/tmp/example.txt"}`,
				},
			},
		},
	}

	out := h.convertSingleResponseToOpenAI(resp, "helix-llm")

	require.Len(t, out.Choices, 1)
	assert.Equal(t, "tool_calls", out.Choices[0].FinishReason,
		"finish_reason must remain tool_calls when ToolCalls are present")

	msg := out.Choices[0].Message
	require.Len(t, msg.ToolCalls, 1, "tool_calls must be propagated to the OpenAI message")
	assert.Equal(t, "call_abc123", msg.ToolCalls[0].ID)
	assert.Equal(t, "function", msg.ToolCalls[0].Type)
	assert.Equal(t, "read_file", msg.ToolCalls[0].Function.Name)
	assert.Equal(t, `{"path":"/tmp/example.txt"}`, msg.ToolCalls[0].Function.Arguments)
}

// TestConvertSingleResponseToOpenAI_ReclassifiesEmptyToolCalls covers
// the defensive case: an upstream provider returned finish_reason
// "tool_calls" but no actual ToolCalls. Emitting that to the client
// reproduces the OpenCode hang. The conversion now reclassifies
// finish_reason to "stop" (if there's content) or "error" (if nothing
// at all) so the client never sees the hang shape.
func TestConvertSingleResponseToOpenAI_ReclassifiesEmptyToolCalls(t *testing.T) {
	t.Parallel()
	h := &UnifiedHandler{}

	t.Run("with content → stop", func(t *testing.T) {
		out := h.convertSingleResponseToOpenAI(&models.LLMResponse{
			ID:           "x",
			Content:      "I'll just answer in text.",
			FinishReason: "tool_calls",
		}, "helix-llm")
		require.Len(t, out.Choices, 1)
		assert.Equal(t, "stop", out.Choices[0].FinishReason,
			"empty tool_calls + content present should reclassify to stop")
		assert.Empty(t, out.Choices[0].Message.ToolCalls)
	})

	t.Run("no content → error", func(t *testing.T) {
		out := h.convertSingleResponseToOpenAI(&models.LLMResponse{
			ID:           "x",
			Content:      "",
			FinishReason: "tool_calls",
		}, "helix-llm")
		require.Len(t, out.Choices, 1)
		assert.Equal(t, "error", out.Choices[0].FinishReason,
			"no content + no tool_calls should reclassify to error so the client fails loudly")
		assert.Empty(t, out.Choices[0].Message.ToolCalls)
	})
}

// TestConvertSingleResponseToOpenAI_NoToolCallsIsClean verifies the
// happy path: a normal text response with no tool calls produces a
// message without a tool_calls field at all (omitempty), preserving
// backward compatibility.
func TestConvertSingleResponseToOpenAI_NoToolCallsIsClean(t *testing.T) {
	t.Parallel()
	h := &UnifiedHandler{}

	out := h.convertSingleResponseToOpenAI(&models.LLMResponse{
		ID:           "x",
		Content:      "Hello!",
		FinishReason: "stop",
	}, "helix-llm")

	require.Len(t, out.Choices, 1)
	assert.Equal(t, "stop", out.Choices[0].FinishReason)
	assert.Equal(t, "Hello!", out.Choices[0].Message.Content)
	assert.Empty(t, out.Choices[0].Message.ToolCalls,
		"tool_calls must be empty/omitted when none present")
}
