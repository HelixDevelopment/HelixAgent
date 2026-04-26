package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertChunkToSSE_EscapesUnsafeContent covers the CONST-032
// reproduction guard for the SSE Chunk JSON Validity bug.
//
// The previous fmt.Sprintf-based encoder broke whenever chunk.Content
// contained a `"`, `\`, control char, or newline — clients saw
// "JSON parse error: Unterminated string". The fix uses json.Marshal
// so the standard library escapes those characters. The test asserts
// the emitted SSE line is parseable for content shapes that broke the
// old encoder.
func TestConvertChunkToSSE_EscapesUnsafeContent(t *testing.T) {
	t.Parallel()
	h := &UnifiedHandler{}

	cases := []struct {
		name    string
		content string
	}{
		{"plain ascii", "Hello world"},
		{"double quotes", `He said "hello".`},
		{"backslash", `path\to\file`},
		{"newline", "line one\nline two"},
		{"carriage return", "windows\r\nlines"},
		{"tab", "col1\tcol2"},
		{"control char", "before\x00after"},
		{"json snippet", `{"a": "b"}`},
		{"chinese + emoji", "你好！👋"},
		{"empty", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			line := h.convertChunkToSSE(
				&models.LLMResponse{Content: c.content}, "stream-id", "helix-llm")

			require.True(t, strings.HasPrefix(line, "data: "),
				"line must start with `data: ` (got %q)", line[:min(20, len(line))])
			require.True(t, strings.HasSuffix(line, "\n\n"),
				"line must end with double newline")

			payload := strings.TrimSuffix(strings.TrimPrefix(line, "data: "), "\n\n")
			var got map[string]interface{}
			err := json.Unmarshal([]byte(payload), &got)
			require.NoError(t, err,
				"JSON.parse must succeed for content %q (payload %q)", c.content, payload)

			assert.Equal(t, "stream-id", got["id"])
			assert.Equal(t, "chat.completion.chunk", got["object"])
			assert.Equal(t, "helix-llm", got["model"])

			choices, ok := got["choices"].([]interface{})
			require.True(t, ok, "choices must be a JSON array")
			require.Len(t, choices, 1)
			delta := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
			assert.Equal(t, c.content, delta["content"],
				"round-tripped content must equal input verbatim")
		})
	}
}

// TestConvertChunkToSSE_RealUserCrash reproduces the literal payload
// shape from the user's screenshot:
//
//	JSON parsing failed: Text: {"id":"chatcmpl-...","model":"helix-llm",
//	"choices":[{"index":0,"delta":{"content":"..
//	Error message: JSON Parse error: Unterminated string
//
// The reproducing content is an ellipsis followed by a quote — exactly
// what triggered the truncation. Asserts the emitted JSON parses.
func TestConvertChunkToSSE_RealUserCrash(t *testing.T) {
	t.Parallel()
	h := &UnifiedHandler{}
	content := `It's nice to meet you. Is there something I can help you with or would you like to chat about "the codebase"?`
	line := h.convertChunkToSSE(
		&models.LLMResponse{Content: content}, "chatcmpl-1777204088869570613", "helix-llm")
	payload := strings.TrimSuffix(strings.TrimPrefix(line, "data: "), "\n\n")
	var got map[string]interface{}
	err := json.Unmarshal([]byte(payload), &got)
	require.NoError(t, err, "the exact crash scenario must produce parseable JSON")
}

// min is a Go 1.21 builtin substitute for older versions; kept inline
// so the test file is self-contained.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
