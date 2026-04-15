package challenge

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelixSpecifierChallenge(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping specifier challenge in short mode (requires live LLM providers)")
	}

	baseURL := getBaseURL()
	if !serverHealthy(baseURL) {
		t.Skip("HelixAgent server not running at " + baseURL)
	}

	providers := getAvailableProviders(t, baseURL)
	if len(providers) == 0 {
		t.Skip("No providers available for specifier challenge")
	}
	t.Logf("Running specifier challenge with %d providers", len(providers))

	t.Run("SimpleRequestDirectResponse", func(t *testing.T) {
		client := &http.Client{Timeout: 60 * time.Second}
		reqBody := ChatCompletionRequest{
			Model: "helixagent/helixagent-debate",
			Messages: []Message{
				{Role: "user", Content: "What is 2+2? Answer with just the number."},
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Simple request should return 200. Body: %s", string(respBody))

		var chatResp ChatCompletionResponse
		if err := json.Unmarshal(respBody, &chatResp); err == nil {
			assert.NotEmpty(t, chatResp.ID, "Response should have an ID")
			assert.NotEmpty(t, chatResp.Choices, "Response should have choices")
		}
	})

	t.Run("MediumRequestDebateConsensus", func(t *testing.T) {
		client := &http.Client{Timeout: 120 * time.Second}
		reqBody := ChatCompletionRequest{
			Model: "helixagent/helixagent-debate",
			Messages: []Message{
				{Role: "user", Content: "Compare microservices vs monolith architecture. Give exactly 3 pros and 3 cons for each approach."},
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Medium request should return 200. Body: %s", truncate(string(respBody), 500))

		var chatResp ChatCompletionResponse
		if err := json.Unmarshal(respBody, &chatResp); err == nil {
			assert.NotEmpty(t, chatResp.Choices, "Should have response choices")
			if len(chatResp.Choices) > 0 {
				content := chatResp.Choices[0].Message.Content
				assert.NotEmpty(t, content, "Response should have content")
				t.Logf("Medium request response length: %d chars", len(content))
			}
		}
	})

	t.Run("ComplexRequestSpecFlow", func(t *testing.T) {
		client := &http.Client{Timeout: 180 * time.Second}
		reqBody := ChatCompletionRequest{
			Model: "helixagent/helixagent-debate",
			Messages: []Message{
				{Role: "user", Content: "Design a complete distributed task scheduler system with fault tolerance, horizontal scaling, priority queues, retry mechanisms, dead letter queues, and a monitoring dashboard. Provide the architecture specification including all components, their interactions, data flows, failure modes, and recovery strategies."},
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Complex request should return 200. Body: %s", truncate(string(respBody), 500))

		var chatResp ChatCompletionResponse
		if err := json.Unmarshal(respBody, &chatResp); err == nil {
			assert.NotEmpty(t, chatResp.Choices, "Should have response choices")
			if len(chatResp.Choices) > 0 {
				content := chatResp.Choices[0].Message.Content
				assert.NotEmpty(t, content, "Spec flow should produce content")
				t.Logf("Complex spec flow response length: %d chars", len(content))
			}
		}
	})

	t.Run("SpecifierEffortClassification", func(t *testing.T) {
		client := &http.Client{Timeout: 180 * time.Second}
		testCases := []struct {
			name      string
			prompt    string
			minLength int
		}{
			{
				name:      "simple_fact",
				prompt:    "What is the capital of France?",
				minLength: 1,
			},
			{
				name:      "medium_analysis",
				prompt:    "Analyze the trade-offs between SQL and NoSQL databases for a social media application with millions of users.",
				minLength: 100,
			},
			{
				name:      "complex_architecture",
				prompt:    "Create a full specification for a real-time collaborative document editing system that supports offline editing, conflict resolution using CRDTs, version history, commenting, and real-time presence indicators.",
				minLength: 200,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				reqBody := ChatCompletionRequest{
					Model: "helixagent/helixagent-debate",
					Messages: []Message{
						{Role: "user", Content: tc.prompt},
					},
				}
				body, _ := json.Marshal(reqBody)

				resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
				require.NoError(t, err)
				defer resp.Body.Close()

				respBody, _ := io.ReadAll(resp.Body)
				assert.Equal(t, http.StatusOK, resp.StatusCode, "Effort classification request should return 200")

				var chatResp ChatCompletionResponse
				if err := json.Unmarshal(respBody, &chatResp); err == nil && len(chatResp.Choices) > 0 {
					content := chatResp.Choices[0].Message.Content
					assert.GreaterOrEqual(t, len(content), tc.minLength,
						"Response for %s should be at least %d chars, got %d", tc.name, tc.minLength, len(content))
					t.Logf("Effort classification [%s]: %d chars", tc.name, len(content))
				}
			})
		}
	})

	t.Run("SpecifierWithDebateIntegration", func(t *testing.T) {
		client := &http.Client{Timeout: 180 * time.Second}
		reqBody := ChatCompletionRequest{
			Model: "helixagent/helixagent-debate",
			Messages: []Message{
				{Role: "system", Content: "You are an expert software architect. Provide detailed technical specifications."},
				{Role: "user", Content: "Design an event-driven microservices architecture for a ride-sharing platform with real-time GPS tracking, dynamic pricing, driver-rider matching algorithm, payment processing, and rating system. Include API specifications, event schemas, and deployment strategy."},
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Specifier+debate request should return 200")

		var chatResp ChatCompletionResponse
		if err := json.Unmarshal(respBody, &chatResp); err == nil && len(chatResp.Choices) > 0 {
			content := chatResp.Choices[0].Message.Content
			assert.NotEmpty(t, content, "Specifier+debate should produce content")
			t.Logf("Specifier+debate response length: %d chars", len(content))
		}
	})

	t.Run("SpecifierResilience", func(t *testing.T) {
		client := &http.Client{Timeout: 60 * time.Second}
		reqBody := ChatCompletionRequest{
			Model: "helixagent/helixagent-debate",
			Messages: []Message{
				{Role: "user", Content: ""},
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Contains(t, []int{http.StatusOK, http.StatusBadRequest, http.StatusUnprocessableEntity},
			resp.StatusCode, "System should handle empty prompt gracefully")
	})
}
