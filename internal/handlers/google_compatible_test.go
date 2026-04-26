package handlers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSplitGoogleModelAction covers the param parser that recovers
// model + method from a Google-style `<MODEL>:<METHOD>` path segment.
func TestSplitGoogleModelAction(t *testing.T) {
	cases := []struct {
		name           string
		param          string
		fullPath       string
		wantModel      string
		wantMethod     string
	}{
		{
			name:       "model:generateContent in single segment",
			param:      "gemini-1.5-pro:generateContent",
			fullPath:   "/v1beta/models/gemini-1.5-pro:generateContent",
			wantModel:  "gemini-1.5-pro",
			wantMethod: "generateContent",
		},
		{
			name:       "model:streamGenerateContent",
			param:      "gemini-2.0-flash:streamGenerateContent",
			fullPath:   "/v1beta/models/gemini-2.0-flash:streamGenerateContent",
			wantModel:  "gemini-2.0-flash",
			wantMethod: "streamGenerateContent",
		},
		{
			name:       "no colon — fall back to URL suffix",
			param:      "gemini-1.0-pro",
			fullPath:   "/v1/models/gemini-1.0-pro/generateContent",
			wantModel:  "gemini-1.0-pro",
			wantMethod: "generateContent",
		},
		{
			name:       "model with version dot survives",
			param:      "gemini-1.5-flash-002:generateContent",
			fullPath:   "/v1beta/models/gemini-1.5-flash-002:generateContent",
			wantModel:  "gemini-1.5-flash-002",
			wantMethod: "generateContent",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			model, method := splitGoogleModelAction(c.param, c.fullPath)
			assert.Equal(t, c.wantModel, model)
			assert.Equal(t, c.wantMethod, method)
		})
	}
}

// TestTranslateGoogleToOpenAI_BasicShape covers the happy path: a single
// user turn with a system instruction, generationConfig knobs.
func TestTranslateGoogleToOpenAI_BasicShape(t *testing.T) {
	in := &GoogleGenerateContentRequest{
		SystemInstruction: &GoogleContent{
			Parts: []GooglePart{{Text: "You are helpful."}},
		},
		Contents: []GoogleContent{
			{
				Role:  "user",
				Parts: []GooglePart{{Text: "Reply with PONG."}},
			},
		},
		GenerationConfig: &GoogleGenerationConfig{
			Temperature:     0.5,
			TopP:            0.9,
			MaxOutputTokens: 100,
			StopSequences:   []string{"###"},
		},
	}

	out, err := translateGoogleToOpenAI(in, "gemini-1.5-pro")
	require.NoError(t, err)
	assert.Equal(t, "gemini-1.5-pro", out.Model)
	assert.Equal(t, 100, out.MaxTokens)
	assert.Equal(t, 0.5, out.Temperature)
	assert.Equal(t, 0.9, out.TopP)
	assert.Equal(t, []string{"###"}, out.Stop)
	require.Len(t, out.Messages, 2)
	assert.Equal(t, "system", out.Messages[0].Role)
	assert.Equal(t, "You are helpful.", out.Messages[0].Content)
	assert.Equal(t, "user", out.Messages[1].Role)
	assert.Equal(t, "Reply with PONG.", out.Messages[1].Content)
}

// TestTranslateGoogleToOpenAI_RoleMapping verifies model→assistant
// mapping and that empty role defaults to user.
func TestTranslateGoogleToOpenAI_RoleMapping(t *testing.T) {
	in := &GoogleGenerateContentRequest{
		Contents: []GoogleContent{
			{Role: "user", Parts: []GooglePart{{Text: "Q"}}},
			{Role: "model", Parts: []GooglePart{{Text: "A"}}},
			{Role: "", Parts: []GooglePart{{Text: "Q2 (no role)"}}},
		},
	}
	out, err := translateGoogleToOpenAI(in, "gemini-pro")
	require.NoError(t, err)
	require.Len(t, out.Messages, 3)
	assert.Equal(t, "user", out.Messages[0].Role)
	assert.Equal(t, "assistant", out.Messages[1].Role)
	assert.Equal(t, "user", out.Messages[2].Role)
}

// TestTranslateGoogleToOpenAI_InlineDataRejected covers refusal of image
// / audio inlineData parts, which can't be expressed in the OpenAI
// text-only request the translator builds.
func TestTranslateGoogleToOpenAI_InlineDataRejected(t *testing.T) {
	in := &GoogleGenerateContentRequest{
		Contents: []GoogleContent{
			{
				Role: "user",
				Parts: []GooglePart{
					{Text: "Describe this image:"},
					{InlineData: map[string]interface{}{"mimeType": "image/png", "data": "..."}},
				},
			},
		},
	}
	_, err := translateGoogleToOpenAI(in, "gemini-pro-vision")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inlineData")
}

// TestTranslateOpenAIToGoogle_HappyPath covers the response-side
// translation: an OpenAI completion JSON becomes a Google
// GenerateContentResponse with one candidate, role="model", finish
// reason mapped to UPPER_SNAKE_CASE, and usageMetadata populated.
func TestTranslateOpenAIToGoogle_HappyPath(t *testing.T) {
	openaiJSON := []byte(`{
		"id": "chatcmpl-zzz",
		"model": "helixagent-ensemble",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "PONG"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 8, "completion_tokens": 1, "total_tokens": 9}
	}`)

	got := translateOpenAIToGoogle(openaiJSON, "gemini-1.5-pro")
	assert.Equal(t, "helixagent-ensemble", got.ModelVersion)
	require.Len(t, got.Candidates, 1)
	assert.Equal(t, "model", got.Candidates[0].Content.Role)
	require.Len(t, got.Candidates[0].Content.Parts, 1)
	assert.Equal(t, "PONG", got.Candidates[0].Content.Parts[0].Text)
	assert.Equal(t, "STOP", got.Candidates[0].FinishReason)
	require.NotNil(t, got.UsageMetadata)
	assert.Equal(t, 8, got.UsageMetadata.PromptTokenCount)
	assert.Equal(t, 1, got.UsageMetadata.CandidatesTokenCount)
	assert.Equal(t, 9, got.UsageMetadata.TotalTokenCount)
}

// TestMapOpenAIFinishReasonToGoogle covers the finish-reason vocabulary
// translation from OpenAI's lowercase to Google's UPPER_SNAKE_CASE.
func TestMapOpenAIFinishReasonToGoogle(t *testing.T) {
	cases := []struct{ in, want string }{
		{"stop", "STOP"},
		{"", "STOP"},
		{"length", "MAX_TOKENS"},
		{"content_filter", "SAFETY"},
		{"tool_calls", "STOP"},
		{"function_call", "STOP"},
		{"unknown_reason", "STOP"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, mapOpenAIFinishReasonToGoogle(c.in),
			"mapOpenAIFinishReasonToGoogle(%q)", c.in)
	}
}

// TestTranslateOpenAIToGoogle_DegenerateUpstreamYieldsEmptyCandidate
// verifies that a degenerate OpenAI response (zero choices) still yields
// a valid Google envelope with at least one (empty) candidate so SDKs
// that index into candidates[0] don't crash.
func TestTranslateOpenAIToGoogle_DegenerateUpstreamYieldsEmptyCandidate(t *testing.T) {
	got := translateOpenAIToGoogle([]byte(`{}`), "gemini-1.5-pro")
	require.Len(t, got.Candidates, 1)
	assert.Equal(t, "model", got.Candidates[0].Content.Role)
	assert.Equal(t, "", got.Candidates[0].Content.Parts[0].Text)
	assert.Equal(t, "STOP", got.Candidates[0].FinishReason)
	assert.Equal(t, "gemini-1.5-pro", got.ModelVersion)
}

// TestGoogleErrorEnvelopeShape verifies the Google error JSON shape
// matches what google-genai SDKs expect — the `error` wrapper, with
// numeric `code`, string `message`, and string `status`.
func TestGoogleErrorEnvelopeShape(t *testing.T) {
	errResp := GoogleErrorResponse{
		Error: GoogleError{
			Code:    400,
			Message: "test message",
			Status:  "INVALID_ARGUMENT",
		},
	}
	b, err := json.Marshal(errResp)
	require.NoError(t, err)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &got))
	inner, ok := got["error"].(map[string]interface{})
	require.True(t, ok, "expected error wrapper, got %v", got)
	assert.EqualValues(t, 400, inner["code"])
	assert.Equal(t, "test message", inner["message"])
	assert.Equal(t, "INVALID_ARGUMENT", inner["status"])
}
