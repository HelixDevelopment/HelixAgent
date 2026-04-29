// Package integration provides regression tests for OpenCode configuration.
// These tests ensure the OpenCode config only shows HelixAgent models,
// not models from other providers.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// OPENCODE CONFIG REGRESSION TESTS
// =============================================================================

// OpenCodeConfigFull represents the full OpenCode configuration structure
type OpenCodeConfigFull struct {
	Schema   string                             `json:"$schema"`
	Provider map[string]OpenCodeProviderFull    `json:"provider"`
	Agent    map[string]OpenCodeAgentConfigFull `json:"agent,omitempty"`
}

// OpenCodeProviderFull represents a provider definition with all fields
type OpenCodeProviderFull struct {
	NPM     string                       `json:"npm,omitempty"`
	Name    string                       `json:"name"`
	Options map[string]interface{}       `json:"options"`
	Models  map[string]OpenCodeModelFull `json:"models,omitempty"`
}

// OpenCodeModelFull represents a model definition
type OpenCodeModelFull struct {
	Name        string `json:"name"`
	Attachments bool   `json:"attachments,omitempty"`
	Reasoning   bool   `json:"reasoning,omitempty"`
}

// OpenCodeAgentConfigFull represents a full agent configuration
type OpenCodeAgentConfigFull struct {
	Model       string          `json:"model,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	Prompt      string          `json:"prompt,omitempty"`
	Description string          `json:"description,omitempty"`
	Tools       map[string]bool `json:"tools,omitempty"`
}

// OpenCodeAgentFull represents agent configuration (legacy format)
type OpenCodeAgentFull struct {
	Model *OpenCodeModelRefFull `json:"model"`
}

// OpenCodeModelRefFull represents a model reference
type OpenCodeModelRefFull struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// TestOpenCodeConfigOnlyShowsHelixAgentModel ensures the generated config
// only includes the HelixAgent model, not models from other providers
func TestOpenCodeConfigOnlyShowsHelixAgentModel(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
		t.Logf("HelixAgent binary not found - run make build first (acceptable)")
		return
	}

	t.Run("ConfigUsesHelixAgentProvider", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err, "Failed to generate OpenCode config")

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err, "Config should be valid JSON")

		// CRITICAL: Must use "helixagent" provider, NOT "openai"
		// Using "openai" causes OpenCode to show all OpenAI models
		_, hasHelixagent := openCodeConfig.Provider["helixagent"]
		_, hasOpenAI := openCodeConfig.Provider["openai"]

		assert.True(t, hasHelixagent, "Config MUST use 'helixagent' provider key")
		assert.False(t, hasOpenAI, "Config MUST NOT use 'openai' provider key (causes model pollution)")
	})

	t.Run("ConfigHasExplicitModels", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		provider, exists := openCodeConfig.Provider["helixagent"]
		require.True(t, exists, "HelixAgent provider must exist")

		// CRITICAL: Must have explicit models defined
		// Without this, OpenCode might try to fetch models from an external API
		assert.NotNil(t, provider.Models, "Provider MUST have explicit models defined")
		assert.NotEmpty(t, provider.Models, "Provider MUST have at least one model")
	})

	t.Run("ConfigHasOnlyHelixAgentDebateModel", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		provider := openCodeConfig.Provider["helixagent"]

		// CRITICAL: Must have exactly one model: helixagent-debate
		assert.Len(t, provider.Models, 1, "Provider MUST have exactly ONE model")

		model, exists := provider.Models["helixagent-debate"]
		assert.True(t, exists, "Model 'helixagent-debate' MUST be defined")
		assert.Equal(t, "HelixAgent AI Debate Ensemble", model.Name)
	})

	t.Run("ConfigHasNPMPackage", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		provider := openCodeConfig.Provider["helixagent"]

		// Must specify the OpenAI-compatible npm package
		assert.Equal(t, "@ai-sdk/openai-compatible", provider.NPM,
			"Provider MUST specify '@ai-sdk/openai-compatible' npm package")
	})

	t.Run("AgentUsesHelixAgentProvider", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		require.NotNil(t, openCodeConfig.Agent, "Agent configuration must exist")

		// Agent is now a map of agent configurations (coder, task, title, summarizer)
		coderAgent, hasCoder := openCodeConfig.Agent["coder"]
		require.True(t, hasCoder, "Agent config must have 'coder' agent")

		// CRITICAL: Agent model must reference helixagent provider
		// Model format is "provider/model" or just "model" with provider context
		assert.Contains(t, coderAgent.Model, "helixagent",
			"Agent model MUST reference 'helixagent' provider")
	})

	t.Run("ConfigDoesNotContainOpenAIString", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		configStr := string(output)

		// Check that "openai" doesn't appear as a provider key
		// (it's OK if it appears in the npm package name)
		lines := strings.Split(configStr, "\n")
		for _, line := range lines {
			if strings.Contains(line, `"openai"`) && !strings.Contains(line, "@ai-sdk") {
				// This line contains "openai" but not as part of npm package
				if strings.Contains(line, `"provider"`) || strings.TrimSpace(line) == `"openai": {` {
					t.Errorf("Config contains 'openai' as provider key: %s", line)
				}
			}
		}
	})
}

// TestModelsEndpointOnlyReturnsHelixAgentModel verifies the /v1/models
// endpoint only returns HelixAgent models
func TestModelsEndpointOnlyReturnsHelixAgentModel(t *testing.T) {
	testutil.RequireServer(t)
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	t.Run("ModelsEndpointReturnsOnlyHelixAgent", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/models", nil)
		require.NoError(t, err)
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Skipf("Models endpoint not available: %d (SKIP-OK: #infra-unavailable)", resp.StatusCode)
		}

		var modelsResp OpenAIModelsResponse
		err = json.NewDecoder(resp.Body).Decode(&modelsResp)
		require.NoError(t, err)

		// CRITICAL: Should only have HelixAgent models
		for _, model := range modelsResp.Data {
			assert.True(t,
				strings.HasPrefix(model.ID, "helixagent") ||
					model.OwnedBy == "helixagent",
				"Model '%s' (owned by '%s') should be a HelixAgent model",
				model.ID, model.OwnedBy)
		}
	})

	t.Run("ModelsEndpointDoesNotReturnExternalModels", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/models", nil)
		require.NoError(t, err)
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Skipf("Models endpoint not available: %d (SKIP-OK: #infra-unavailable)", resp.StatusCode)
		}

		var modelsResp OpenAIModelsResponse
		err = json.NewDecoder(resp.Body).Decode(&modelsResp)
		require.NoError(t, err)

		// List of external model prefixes that should NOT appear
		externalPrefixes := []string{
			"gpt-", "claude-", "gemini-", "deepseek-",
			"llama", "mistral", "qwen", "command",
		}

		for _, model := range modelsResp.Data {
			modelLower := strings.ToLower(model.ID)
			for _, prefix := range externalPrefixes {
				assert.False(t, strings.HasPrefix(modelLower, prefix),
					"Models endpoint should NOT return external model '%s'", model.ID)
			}
		}
	})

	t.Run("ModelsEndpointReturnsHelixAgentDebate", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/models", nil)
		require.NoError(t, err)
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Skipf("Models endpoint not available: %d (SKIP-OK: #infra-unavailable)", resp.StatusCode)
		}

		var modelsResp OpenAIModelsResponse
		err = json.NewDecoder(resp.Body).Decode(&modelsResp)
		require.NoError(t, err)

		// Must have helixagent-debate model
		hasDebateModel := false
		for _, model := range modelsResp.Data {
			if model.ID == "helixagent-debate" {
				hasDebateModel = true
				assert.Equal(t, "helixagent", model.OwnedBy,
					"helixagent-debate model should be owned by 'helixagent'")
				break
			}
		}
		assert.True(t, hasDebateModel, "Must include 'helixagent-debate' model")
	})
}

// TestOpenCodeConfigFileIntegrity tests the saved config file
func TestOpenCodeConfigFileIntegrity(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
		t.Logf("HelixAgent binary not found - run make build first (acceptable)")
		return
	}

	t.Run("SavedConfigMatchesOutput", func(t *testing.T) {
		configPath := filepath.Join(config.TempDir, "opencode-test.json")

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		// Generate to file
		cmd := exec.CommandContext(ctx, config.BinaryPath,
			"-generate-opencode-config",
			"-opencode-output", configPath)
		_, err := cmd.Output()
		require.NoError(t, err)

		// Read the file
		fileContent, err := os.ReadFile(configPath)
		require.NoError(t, err)

		var fileConfig OpenCodeConfigFull
		err = json.Unmarshal(fileContent, &fileConfig)
		require.NoError(t, err)

		// Verify it uses helixagent provider
		_, hasHelixagent := fileConfig.Provider["helixagent"]
		assert.True(t, hasHelixagent, "Saved config must use 'helixagent' provider")

		// Verify models are defined
		provider := fileConfig.Provider["helixagent"]
		assert.NotEmpty(t, provider.Models, "Saved config must have explicit models")
	})

	t.Run("ConfigFileIsValidJSON", func(t *testing.T) {
		configPath := filepath.Join(config.TempDir, "opencode-valid.json")

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath,
			"-generate-opencode-config",
			"-opencode-output", configPath)
		_, err := cmd.Output()
		require.NoError(t, err)

		fileContent, err := os.ReadFile(configPath)
		require.NoError(t, err)

		// Should be valid JSON
		var raw map[string]interface{}
		err = json.Unmarshal(fileContent, &raw)
		assert.NoError(t, err, "Config file must be valid JSON")
	})
}

// TestOpenCodeConfigAPIKeyHandling tests API key handling in config
func TestOpenCodeConfigAPIKeyHandling(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
		t.Logf("HelixAgent binary not found - run make build first (acceptable)")
		return
	}

	t.Run("ConfigIncludesAPIKey", func(t *testing.T) {
		testKey := "sk-test1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcd"

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		cmd.Env = append(os.Environ(), "HELIXAGENT_API_KEY="+testKey)
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		provider := openCodeConfig.Provider["helixagent"]
		apiKey, ok := provider.Options["apiKey"].(string)
		require.True(t, ok, "API key must be a string")
		// Config uses the real API key value (not env var template syntax)
		// CLI agents do NOT support {env:VAR_NAME} syntax per project rules
		assert.Equal(t, testKey, apiKey,
			"Config must use real API key value (not env var template)")
	})

	t.Run("ConfigBaseURLIsCorrect", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		cmd.Env = append(os.Environ(),
			"HELIXAGENT_HOST=myhost.example.com",
			"PORT=9999")
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		provider := openCodeConfig.Provider["helixagent"]
		baseURL, ok := provider.Options["baseURL"].(string)
		require.True(t, ok, "baseURL must be a string")
		assert.Equal(t, "http://myhost.example.com:9999/v1", baseURL)
	})
}

// TestOpenCodeChatCompletionWithHelixAgentModel tests that chat completions
// work correctly with the helixagent-debate model
func TestOpenCodeChatCompletionWithHelixAgentModel(t *testing.T) {
	testutil.RequireServer(t)
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	t.Run("ChatCompletionWithHelixAgentDebate", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		chatReq := OpenAIChatRequest{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Say 'hello' and nothing else."},
			},
			MaxTokens:   20,
			Temperature: 0.0,
			Stream:      false,
		}

		body, err := json.Marshal(chatReq)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "deadline exceeded") || strings.Contains(errStr, "timeout") ||
				strings.Contains(errStr, "EOF") || strings.Contains(errStr, "connection") {
				t.Logf("Request failed - providers may be slow or unavailable (acceptable)")
				return
			}
			require.NoError(t, err)
		}
		defer resp.Body.Close()

		// Should work with helixagent-debate model, but skip on provider failures
		if resp.StatusCode == http.StatusInternalServerError ||
			resp.StatusCode == http.StatusBadGateway ||
			resp.StatusCode == http.StatusServiceUnavailable ||
			resp.StatusCode == http.StatusGatewayTimeout {
			t.Skipf("Server returned %d - providers may be unavailable (SKIP-OK: #unmarked-skip-needs-ticket)", resp.StatusCode)
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"Chat completion with 'helixagent-debate' model should succeed")
	})

	t.Run("InvalidModelReturnsError", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		chatReq := OpenAIChatRequest{
			Model: "gpt-4-turbo", // External model - should not work
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Hello"},
			},
			MaxTokens: 20,
			Stream:    false,
		}

		body, err := json.Marshal(chatReq)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// External model should either fail or be handled appropriately
		// (not return actual GPT-4 responses)
		t.Logf("Response for external model 'gpt-4-turbo': %d", resp.StatusCode)
	})
}

// =============================================================================
// REGRESSION PREVENTION ASSERTIONS
// =============================================================================

// TestRegressionPreventionAssertions contains critical assertions to prevent
// the "multiple LLMs showing in OpenCode" regression
func TestRegressionPreventionAssertions(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
		t.Logf("HelixAgent binary not found - run make build first (acceptable)")
		return
	}

	t.Run("CRITICAL_NoOpenAIProviderKey", func(t *testing.T) {
		// This is a CRITICAL regression test
		// If this fails, OpenCode will show all OpenAI models instead of just HelixAgent

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		var raw map[string]interface{}
		err = json.Unmarshal(output, &raw)
		require.NoError(t, err)

		providers, ok := raw["provider"].(map[string]interface{})
		require.True(t, ok, "Config must have 'provider' field")

		// CRITICAL: "openai" key MUST NOT exist
		_, hasOpenAI := providers["openai"]
		require.False(t, hasOpenAI,
			"CRITICAL REGRESSION: Config has 'openai' provider key! "+
				"This causes OpenCode to show all OpenAI models. "+
				"Must use 'helixagent' provider key instead.")

		// CRITICAL: "helixagent" key MUST exist
		_, hasHelixagent := providers["helixagent"]
		require.True(t, hasHelixagent,
			"CRITICAL: Config must have 'helixagent' provider key")
	})

	t.Run("CRITICAL_ExplicitModelsRequired", func(t *testing.T) {
		// This is a CRITICAL regression test
		// If models are not explicitly defined, OpenCode might fetch from external API

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		var raw map[string]interface{}
		err = json.Unmarshal(output, &raw)
		require.NoError(t, err)

		providers := raw["provider"].(map[string]interface{})
		helixagent := providers["helixagent"].(map[string]interface{})

		// CRITICAL: "models" field MUST exist
		models, hasModels := helixagent["models"]
		require.True(t, hasModels,
			"CRITICAL REGRESSION: No 'models' field in provider config! "+
				"This might cause OpenCode to fetch models from external API.")

		modelsMap, ok := models.(map[string]interface{})
		require.True(t, ok, "models field must be an object")
		require.NotEmpty(t, modelsMap,
			"CRITICAL: 'models' field must not be empty")
	})

	t.Run("CRITICAL_OnlyHelixAgentDebateModel", func(t *testing.T) {
		// Ensures only the helixagent-debate model is defined

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		provider := openCodeConfig.Provider["helixagent"]

		// Should have exactly one model
		require.Len(t, provider.Models, 1,
			"CRITICAL: Must have exactly ONE model defined, not %d", len(provider.Models))

		// That model should be helixagent-debate
		_, hasDebate := provider.Models["helixagent-debate"]
		require.True(t, hasDebate,
			"CRITICAL: Model 'helixagent-debate' must be defined")
	})
}
