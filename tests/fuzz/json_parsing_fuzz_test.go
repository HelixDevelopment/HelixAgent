//go:build fuzz

// Package fuzz provides Go native fuzzing tests for critical parsing paths
// in the HelixAgent system. These tests ensure that malformed or adversarial
// input never causes panics or undefined behavior.
package fuzz

import (
	"encoding/json"
	"strings"
	"testing"

	"dev.helix.agent/internal/models"
)

// FuzzLLMRequestParsing tests that parsing LLMRequest from arbitrary JSON bytes
// never panics. This ensures robustness against malformed API payloads.
func FuzzLLMRequestParsing(f *testing.F) {
	// Seed corpus: valid LLMRequest payloads
	f.Add([]byte(`{"prompt":"hello","messages":[{"role":"user","content":"hi"}]}`))
	f.Add([]byte(`{"model_params":{"temperature":0.7,"max_tokens":100}}`))
	f.Add([]byte(`{"memory_enhanced":true,"memory":{"key":"value"}}`))
	f.Add([]byte(`{"tools":[{"type":"function","function":{"name":"bash","description":"run command"}}]}`))
	f.Add([]byte(`{"tool_choice":"auto"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))
	f.Add([]byte("\x00\x01\x02\xff\xfe\xfd"))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzLLMRequestParsing panicked with input %q: %v", data, r)
			}
		}()

		// Attempt to unmarshal into LLMRequest — must not panic
		var req models.LLMRequest
		_ = json.Unmarshal(data, &req)

		// If parsing succeeded, attempt to re-marshal for round-trip safety
		if req.Prompt != "" || len(req.Messages) > 0 || len(req.Memory) > 0 {
			_, _ = json.Marshal(&req)
		}

		// Also try parsing as LLMResponse
		var resp models.LLMResponse
		_ = json.Unmarshal(data, &resp)

		// Also try parsing as a generic OpenAI-compatible chat request
		var chatReq struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Temperature *float64 `json:"temperature"`
			MaxTokens   *int     `json:"max_tokens"`
			Stream      bool     `json:"stream"`
			Tools       []struct {
				Type string `json:"type"`
			} `json:"tools"`
		}
		_ = json.Unmarshal(data, &chatReq)

		// Safe field access — must not panic
		for _, msg := range chatReq.Messages {
			_ = len(msg.Role)
			_ = len(msg.Content)
		}
	})
}

// FuzzMessageParsing tests that parsing Message structs from arbitrary JSON
// never panics. Messages are the core unit of LLM communication.
func FuzzMessageParsing(f *testing.F) {
	// Seed corpus: valid Message payloads
	f.Add(`{"role":"user","content":"hello world"}`)
	f.Add(`{"role":"assistant","content":"I can help with that"}`)
	f.Add(`{"role":"system","content":"You are a helpful AI assistant"}`)
	f.Add(`{"role":"","content":""}`)
	f.Add(`{"role":"tool","content":null}`)
	f.Add(`{}`)
	f.Add(`invalid json`)
	f.Add(`{"role":"` + string(make([]byte, 10000)) + `"}`)

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzMessageParsing panicked with input %q: %v", input, r)
			}
		}()

		var msg models.Message
		if err := json.Unmarshal([]byte(input), &msg); err != nil {
			return // invalid JSON is acceptable; panics are not
		}

		// Safe field access
		_ = len(msg.Role)
		_ = len(msg.Content)

		// Re-marshal for round-trip safety
		_, _ = json.Marshal(&msg)

		// Also parse as slice of messages
		var msgs []models.Message
		_ = json.Unmarshal([]byte(input), &msgs)
		for _, m := range msgs {
			_ = len(m.Role)
		}
	})
}

// FuzzProviderResponseParsing tests that parsing LLM provider responses from
// arbitrary JSON bytes never panics. Providers return responses in various formats
// (OpenAI-compatible, Cohere, Gemini, etc.) and all must be handled safely.
func FuzzProviderResponseParsing(f *testing.F) {
	// Seed corpus: valid and malformed provider response payloads
	f.Add([]byte(`{"id":"chatcmpl-123","content":"Hello","confidence":0.95,"tokens_used":50,"response_time":1200}`))
	f.Add([]byte(`{"id":"","provider_id":"openai","provider_name":"OpenAI","content":"response text","finish_reason":"stop","tool_calls":[]}`))
	f.Add([]byte(`{"id":"resp-1","tool_calls":[{"id":"call-1","type":"function","function":{"name":"bash","arguments":"{\"command\":\"ls\"}"}}]}`))
	f.Add([]byte(`{"content":null,"confidence":-1,"tokens_used":-100,"response_time":-1}`))
	f.Add([]byte(`{"metadata":{"model":"gpt-4","usage":{"prompt_tokens":10,"completion_tokens":20}}}`))
	// Non-OpenAI provider formats (Cohere, Gemini-style)
	f.Add([]byte(`{"text":"response","generation_id":"gen-123","finish_reason":"COMPLETE","meta":{"tokens":{"input_tokens":5,"output_tokens":15}}}`))
	f.Add([]byte(`{"candidates":[{"content":{"parts":[{"text":"hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":10}}`))
	// Boundary cases
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))
	f.Add([]byte("\x00\xff\xfe"))
	// Very long content
	f.Add([]byte(`{"content":"` + strings.Repeat("x", 8192) + `"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzProviderResponseParsing panicked with input %q: %v", data, r)
			}
		}()

		// Parse as LLMResponse — the primary response type
		var resp models.LLMResponse
		_ = json.Unmarshal(data, &resp)

		// Safe field access on the parsed response
		_ = len(resp.ID)
		_ = len(resp.Content)
		_ = len(resp.ProviderID)
		_ = len(resp.ProviderName)
		_ = len(resp.FinishReason)
		_ = resp.Confidence
		_ = resp.TokensUsed
		_ = resp.ResponseTime
		_ = resp.Selected
		_ = resp.SelectionScore

		// Walk tool calls without panicking
		for _, tc := range resp.ToolCalls {
			_ = len(tc.ID)
			_ = len(tc.Type)
			_ = len(tc.Function.Name)
			_ = len(tc.Function.Arguments)
			// Try parsing the arguments JSON string
			if tc.Function.Arguments != "" {
				var args map[string]interface{}
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
			}
		}

		// Walk metadata
		for k, v := range resp.Metadata {
			_ = len(k)
			_ = v
		}

		// Re-marshal for round-trip safety
		_, _ = json.Marshal(&resp)

		// Also parse as a non-OpenAI format (Gemini candidates style)
		var geminiResp struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			} `json:"candidates"`
			UsageMetadata struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
		}
		if err := json.Unmarshal(data, &geminiResp); err == nil {
			for _, c := range geminiResp.Candidates {
				for _, p := range c.Content.Parts {
					_ = len(p.Text)
				}
				_ = len(c.FinishReason)
			}
		}

		// Also parse as Cohere-style response
		var cohereResp struct {
			Text         string `json:"text"`
			GenerationID string `json:"generation_id"`
			FinishReason string `json:"finish_reason"`
			Meta         struct {
				Tokens struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"tokens"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(data, &cohereResp); err == nil {
			_ = len(cohereResp.Text)
			_ = len(cohereResp.GenerationID)
		}
	})
}
