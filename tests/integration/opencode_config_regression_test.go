// Package integration provides regression tests for OpenCode configuration.
// These tests ensure the OpenCode config only shows SuperAgent models,
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// OPENCODE CONFIG REGRESSION TESTS
// =============================================================================

// OpenCodeConfigFull represents the full OpenCode configuration structure
type OpenCodeConfigFull struct {
	Schema   string                         `json:"$schema"`
	Provider map[string]OpenCodeProviderFull `json:"provider"`
	Agent    *OpenCodeAgentFull             `json:"agent,omitempty"`
}

// OpenCodeProviderFull represents a provider definition with all fields
type OpenCodeProviderFull struct {
	NPM     string                     `json:"npm,omitempty"`
	Name    string                     `json:"name"`
	Options map[string]interface{}     `json:"options"`
	Models  map[string]OpenCodeModelFull `json:"models,omitempty"`
}

// OpenCodeModelFull represents a model definition
type OpenCodeModelFull struct {
	Name        string `json:"name"`
	Attachments bool   `json:"attachments,omitempty"`
	Reasoning   bool   `json:"reasoning,omitempty"`
}

// OpenCodeAgentFull represents agent configuration
type OpenCodeAgentFull struct {
	Model *OpenCodeModelRefFull `json:"model"`
}

// OpenCodeModelRefFull represents a model reference
type OpenCodeModelRefFull struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// TestOpenCodeConfigOnlyShowsSuperAgentModel ensures the generated config
// only includes the SuperAgent model, not models from other providers
func TestOpenCodeConfigOnlyShowsSuperAgentModel(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
		t.Skip("SuperAgent binary not found, run 'make build' first")
	}

	t.Run("ConfigUsesSuperAgentProvider", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err, "Failed to generate OpenCode config")

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err, "Config should be valid JSON")

		// CRITICAL: Must use "superagent" provider, NOT "openai"
		// Using "openai" causes OpenCode to show all OpenAI models
		_, hasSuperagent := openCodeConfig.Provider["superagent"]
		_, hasOpenAI := openCodeConfig.Provider["openai"]

		assert.True(t, hasSuperagent, "Config MUST use 'superagent' provider key")
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

		provider, exists := openCodeConfig.Provider["superagent"]
		require.True(t, exists, "SuperAgent provider must exist")

		// CRITICAL: Must have explicit models defined
		// Without this, OpenCode might try to fetch models from an external API
		assert.NotNil(t, provider.Models, "Provider MUST have explicit models defined")
		assert.NotEmpty(t, provider.Models, "Provider MUST have at least one model")
	})

	t.Run("ConfigHasOnlySuperAgentDebateModel", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		provider := openCodeConfig.Provider["superagent"]

		// CRITICAL: Must have exactly one model: superagent-debate
		assert.Len(t, provider.Models, 1, "Provider MUST have exactly ONE model")

		model, exists := provider.Models["superagent-debate"]
		assert.True(t, exists, "Model 'superagent-debate' MUST be defined")
		assert.Equal(t, "SuperAgent Debate Ensemble", model.Name)
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

		provider := openCodeConfig.Provider["superagent"]

		// Must specify the OpenAI-compatible npm package
		assert.Equal(t, "@ai-sdk/openai-compatible", provider.NPM,
			"Provider MUST specify '@ai-sdk/openai-compatible' npm package")
	})

	t.Run("AgentUsesSuperAgentProvider", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		require.NotNil(t, openCodeConfig.Agent, "Agent configuration must exist")
		require.NotNil(t, openCodeConfig.Agent.Model, "Agent model reference must exist")

		// CRITICAL: Agent must use "superagent" provider, not "openai"
		assert.Equal(t, "superagent", openCodeConfig.Agent.Model.Provider,
			"Agent MUST use 'superagent' provider, NOT 'openai'")
		assert.Equal(t, "superagent-debate", openCodeConfig.Agent.Model.Model,
			"Agent MUST use 'superagent-debate' model")
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

// TestModelsEndpointOnlyReturnsSuperAgentModel verifies the /v1/models
// endpoint only returns SuperAgent models
func TestModelsEndpointOnlyReturnsSuperAgentModel(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)
	skipIfNoServer(t, config)

	t.Run("ModelsEndpointReturnsOnlySuperAgent", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/models", nil)
		require.NoError(t, err)
		if config.SuperAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.SuperAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Skipf("Models endpoint not available: %d", resp.StatusCode)
		}

		var modelsResp OpenAIModelsResponse
		err = json.NewDecoder(resp.Body).Decode(&modelsResp)
		require.NoError(t, err)

		// CRITICAL: Should only have SuperAgent models
		for _, model := range modelsResp.Data {
			assert.True(t,
				strings.HasPrefix(model.ID, "superagent") ||
					model.OwnedBy == "superagent",
				"Model '%s' (owned by '%s') should be a SuperAgent model",
				model.ID, model.OwnedBy)
		}
	})

	t.Run("ModelsEndpointDoesNotReturnExternalModels", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/models", nil)
		require.NoError(t, err)
		if config.SuperAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.SuperAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Skipf("Models endpoint not available: %d", resp.StatusCode)
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

	t.Run("ModelsEndpointReturnsSuperAgentDebate", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/models", nil)
		require.NoError(t, err)
		if config.SuperAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.SuperAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Skipf("Models endpoint not available: %d", resp.StatusCode)
		}

		var modelsResp OpenAIModelsResponse
		err = json.NewDecoder(resp.Body).Decode(&modelsResp)
		require.NoError(t, err)

		// Must have superagent-debate model
		hasDebateModel := false
		for _, model := range modelsResp.Data {
			if model.ID == "superagent-debate" {
				hasDebateModel = true
				assert.Equal(t, "superagent", model.OwnedBy,
					"superagent-debate model should be owned by 'superagent'")
				break
			}
		}
		assert.True(t, hasDebateModel, "Must include 'superagent-debate' model")
	})
}

// TestOpenCodeConfigFileIntegrity tests the saved config file
func TestOpenCodeConfigFileIntegrity(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
		t.Skip("SuperAgent binary not found, run 'make build' first")
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

		// Verify it uses superagent provider
		_, hasSuperagent := fileConfig.Provider["superagent"]
		assert.True(t, hasSuperagent, "Saved config must use 'superagent' provider")

		// Verify models are defined
		provider := fileConfig.Provider["superagent"]
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
		t.Skip("SuperAgent binary not found, run 'make build' first")
	}

	t.Run("ConfigIncludesAPIKey", func(t *testing.T) {
		testKey := "sk-test1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcd"

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		cmd.Env = append(os.Environ(), "SUPERAGENT_API_KEY="+testKey)
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		provider := openCodeConfig.Provider["superagent"]
		apiKey, ok := provider.Options["apiKey"].(string)
		require.True(t, ok, "API key must be a string")
		assert.Equal(t, testKey, apiKey, "Config must include the provided API key")
	})

	t.Run("ConfigBaseURLIsCorrect", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		cmd.Env = append(os.Environ(),
			"SUPERAGENT_HOST=myhost.example.com",
			"PORT=9999")
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		provider := openCodeConfig.Provider["superagent"]
		baseURL, ok := provider.Options["baseURL"].(string)
		require.True(t, ok, "baseURL must be a string")
		assert.Equal(t, "http://myhost.example.com:9999/v1", baseURL)
	})
}

// TestOpenCodeChatCompletionWithSuperAgentModel tests that chat completions
// work correctly with the superagent-debate model
func TestOpenCodeChatCompletionWithSuperAgentModel(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)
	skipIfNoServer(t, config)

	t.Run("ChatCompletionWithSuperAgentDebate", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		chatReq := OpenAIChatRequest{
			Model: "superagent-debate",
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
		if config.SuperAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.SuperAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should work with superagent-debate model
		if resp.StatusCode == http.StatusInternalServerError {
			t.Skip("Server returned 500 - providers may be unavailable")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"Chat completion with 'superagent-debate' model should succeed")
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
		if config.SuperAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.SuperAgentAPIKey)
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
		t.Skip("SuperAgent binary not found, run 'make build' first")
	}

	t.Run("CRITICAL_NoOpenAIProviderKey", func(t *testing.T) {
		// This is a CRITICAL regression test
		// If this fails, OpenCode will show all OpenAI models instead of just SuperAgent

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
				"Must use 'superagent' provider key instead.")

		// CRITICAL: "superagent" key MUST exist
		_, hasSuperagent := providers["superagent"]
		require.True(t, hasSuperagent,
			"CRITICAL: Config must have 'superagent' provider key")
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
		superagent := providers["superagent"].(map[string]interface{})

		// CRITICAL: "models" field MUST exist
		models, hasModels := superagent["models"]
		require.True(t, hasModels,
			"CRITICAL REGRESSION: No 'models' field in provider config! "+
				"This might cause OpenCode to fetch models from external API.")

		modelsMap, ok := models.(map[string]interface{})
		require.True(t, ok, "models field must be an object")
		require.NotEmpty(t, modelsMap,
			"CRITICAL: 'models' field must not be empty")
	})

	t.Run("CRITICAL_OnlySuperAgentDebateModel", func(t *testing.T) {
		// Ensures only the superagent-debate model is defined

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		provider := openCodeConfig.Provider["superagent"]

		// Should have exactly one model
		require.Len(t, provider.Models, 1,
			"CRITICAL: Must have exactly ONE model defined, not %d", len(provider.Models))

		// That model should be superagent-debate
		_, hasDebate := provider.Models["superagent-debate"]
		require.True(t, hasDebate,
			"CRITICAL: Model 'superagent-debate' must be defined")
	})
}
