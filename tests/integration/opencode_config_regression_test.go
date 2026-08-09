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

	// §11.4.120 RECONCILED 2026-08-09 (was "ConfigHasOnlyHelixAgentDebateModel").
	//
	// This subtest previously asserted `assert.Len(provider.Models, 1)` plus the
	// sole key `helixagent-debate`. That COUNT encoded the pre-d6f4ea0a
	// single-model provider. Commit d6f4ea0a "feat: Unified Helix Agent provider
	// with dual model routing" deliberately made it TWO models under ONE
	// provider — debate for coder/summarizer, the provider chain for task/title
	// — and /v1/models lists both canonical ids (openai_compatible.go:2336-2337).
	//
	// The count was never the contract. This file's own header states the
	// contract: "only HelixAgent models, NOT models from other providers", and
	// the regression being prevented is named in CRITICAL_NoOpenAIProviderKey —
	// an `openai` provider key making OpenCode enumerate every OpenAI model.
	// Two HelixAgent-OWNED models is not that regression; a foreign model is.
	//
	// Reconciled to assert the NEW mechanism: the declared model set is EXACTLY
	// the canonical HelixAgent set.
	//
	// HONEST COMPARISON (§11.4.6) — this is NOT "strictly stronger". As sets:
	// the old form (`Len==1` AND has `helixagent-debate`) is equivalent to
	// `models == {helixagent-debate}`; the new form is
	// `models == {helixagent-debate, helixagent-llm}`. Neither implies the
	// other — the old singleton is the tighter set, and the old form's key
	// assertion DID catch alias regression (it is not a bare count). The
	// justification is not strength but CORRECTNESS: the old count encoded the
	// pre-d6f4ea0a single-model shape, had been failing since d6f4ea0a made the
	// provider dual-model by design, and asserted an implementation detail
	// rather than this file's stated contract ("only HelixAgent models, not
	// models from other providers"). The replacement pins the exact canonical
	// id-set, which IS that contract.
	//
	// PAIRED §1.1 MUTATION (updated with this reconciliation): add any
	// non-HelixAgent key (e.g. "gpt-4o") to the generator's Models map at
	// cmd/helixagent/main.go → this subtest FAILs on the unexpected-model
	// assertion. Captured 2026-08-09: `Provider declares model "gpt-4o" which
	// is NOT a canonical HelixAgent model id`. Mutating a canonical id to its
	// legacy alias also FAILs it (on the model-present assertion).
	t.Run("ConfigDeclaresOnlyCanonicalHelixAgentModels", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		provider := openCodeConfig.Provider["helixagent"]

		canonical := map[string]bool{
			openCodeCanonicalDebateID: true,
			openCodeCanonicalLLMID:    true,
		}

		// No model outside the canonical HelixAgent set may be declared — this
		// is the actual "no model pollution" contract.
		for id := range provider.Models {
			assert.True(t, canonical[id],
				"Provider declares model %q which is NOT a canonical HelixAgent "+
					"model id; only %v may be declared", id, []string{
					openCodeCanonicalDebateID, openCodeCanonicalLLMID,
				})
		}

		// Both canonical models must be present — the dual-model routing
		// d6f4ea0a introduced (coder/summarizer → debate, task/title → chain).
		model, exists := provider.Models[openCodeCanonicalDebateID]
		assert.True(t, exists, "Model %q MUST be defined", openCodeCanonicalDebateID)
		assert.Equal(t, openCodeCanonicalDebateName, model.Name)

		_, hasLLM := provider.Models[openCodeCanonicalLLMID]
		assert.True(t, hasLLM, "Model %q MUST be defined", openCodeCanonicalLLMID)
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

	// §11.4.120 RECONCILED 2026-08-09 (was "CRITICAL_OnlyHelixAgentDebateModel").
	// Same reconciliation + same paired §1.1 mutation as
	// ConfigDeclaresOnlyCanonicalHelixAgentModels above: the superseded
	// `require.Len(provider.Models, 1)` asserted the pre-d6f4ea0a single-model
	// shape, not the "no models from other providers" contract this file exists
	// to defend. Reconciled to the canonical-set assertion. Per the honest
	// comparison recorded above, that is not "strictly stronger" than the count
	// — it is the assertion that actually matches the stated contract.
	t.Run("CRITICAL_OnlyCanonicalHelixAgentModels", func(t *testing.T) {
		// Ensures ONLY canonical HelixAgent-owned models are declared

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfigFull
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		provider := openCodeConfig.Provider["helixagent"]

		// Must declare at least one model, and NOTHING outside the canonical set
		require.NotEmpty(t, provider.Models,
			"CRITICAL: Provider must declare at least one model")

		for id := range provider.Models {
			require.True(t,
				id == openCodeCanonicalDebateID || id == openCodeCanonicalLLMID,
				"CRITICAL REGRESSION: model %q is not a canonical HelixAgent "+
					"model id — OpenCode would surface a model this provider "+
					"does not own", id)
		}

		// The debate model is the one every quality-path agent binds to
		_, hasDebate := provider.Models[openCodeCanonicalDebateID]
		require.True(t, hasDebate,
			"CRITICAL: Model %q must be defined", openCodeCanonicalDebateID)

		// ...and the provider-chain model the fast-path agents bind to. Without
		// this, a regression dropping the LLM model would pass the subset check
		// above (a subset assertion cannot detect a MISSING member).
		_, hasLLM := provider.Models[openCodeCanonicalLLMID]
		require.True(t, hasLLM,
			"CRITICAL: Model %q must be defined", openCodeCanonicalLLMID)
	})
}

// =============================================================================
// CANONICAL MODEL-ID CONTRACT — §11.4.115 RED-baseline + polarity switch
// =============================================================================

// THE CONTRACT. `helixagent-debate` / `helixagent-llm` are the CANONICAL model
// identifiers for CLI-agent configs; `helix-debate` / `helix-llm` are LEGACY
// ALIASES retained only so already-deployed clients keep working. Six
// independent sources establish this — none of them a test under repair:
//
//  1. internal/handlers/openai_compatible.go:532 — "helixagent-debate
//     (canonical for CLI configs)"; :535 — "helix-debate ... (legacy aliases)";
//     :2292 "canonical name"; :2295 "helix-debate: legacy alias". That comment
//     names docs/api/API_REFERENCE.md + /v1/models as the authority.
//  2. docs/api/API_REFERENCE.md — the cited authority: 15 model references, ALL
//     `helixagent-debate`; `helix-debate` appears ZERO times. HONEST LIMIT: it
//     says NOTHING about the provider-chain model — `helixagent-llm` and
//     `helix-llm` each occur ZERO times there, so the LLM id rests on source 1
//     (openai_compatible.go:2298 "canonical name for the provider chain") plus
//     the /v1/models listing, NOT on API_REFERENCE.md.
//  3. cmd/helixagent/main.go:4252 — handleGenerateCrush, the SIBLING CLI-agent
//     config generator (reached from -generate-crush-config), emits model ID
//     `helixagent-debate` + provider name "HelixAgent AI Debate Ensemble".
//     Same file, same purpose, canonical naming.
//  4. configs/cli-agents/opencode.json (git-TRACKED) — the shipped reference
//     OpenCode config: provider name "HelixAgent", model key
//     `helixagent-debate`, model name "HelixAgent AI Debate Ensemble", agent
//     binding "helixagent/helixagent-debate". AUTHORITATIVE FOR NAMING ONLY —
//     it predates d6f4ea0a and still encodes the single-model shape, so it is
//     explicitly NOT authority for how many models the provider declares.
//     (Same for scripts/cli-agents/configs/opencode.template.json:12,19-25 and
//     scripts/cli-agents/generate-all-configs.sh:586,697, which already emit
//     `helixagent-debate` — this change REDUCES generator divergence.)
//  5. docs/guides/API_KEY_AND_OPENCODE_GUIDE.md:79-96 — the operator-facing
//     guide documents the generated config as `helixagent/helixagent-debate`.
//  6. Branding: README H1 is "HelixAgent: AI-Powered Ensemble LLM Service";
//     `HelixAgent` occurs 4274 times across docs/ + README.md and `Helix Agent`
//     (spaced) ZERO times; `HelixLLM` occurs 460 times and `Helix LLM` (spaced)
//     ZERO times. cmd/helixagent/main.go:2893/2900/2907 held the ONLY spaced
//     occurrences in the entire repository.
//
// USER-VISIBLE CONSEQUENCE of shipping the legacy alias — why this is a product
// defect and not cosmetics. openai_compatible.go:829 reads:
//
//	requestedDebateExplicitly := req.Model == "helixagent-debate" ||
//	                             req.Model == "helixagent-ensemble"
//
// which matches NEITHER `helix-debate` NOR `helixagent/helix-debate`. An
// OpenCode user driven by the generated config therefore never satisfies the
// explicit-debate override introduced by commit 495a43c8, so the intent
// classifier may silently downgrade their coder-agent debate request to a
// single provider. openai_compatible.go:821 states the violated principle:
// "The model name IS the contract — a request asking for the debate model must
// get the debate, otherwise the model name lies."
//
// WIRE EVIDENCE that the bare id is what actually reaches the server:
// challenges/scripts/opencode_helixllm_hello_challenge.sh:40,49 records the
// shape "Captured from the actual OpenCode CLI's HTTP traffic" as a BARE
// `"model": "helix-llm"` — not the provider-qualified form. So the config's
// model KEY is the string compared at :829.
//
// HONEST SCOPE (§11.4.6): `requestedDebateExplicitly` exists only in
// handleStreamingChatCompletions (openai_compatible.go:747+); there is no
// non-streaming equivalent. The override is therefore lost for STREAMING
// requests — which is OpenCode's normal mode (stream=true in the captured
// shape above) — not for every request shape.
//
// The server keeps accepting the legacy aliases (openai_compatible.go:543-550)
// and /v1/models keeps listing all five IDs (:2336-2343) — this change is
// GENERATOR-ONLY, so already-deployed configs continue to work.
//
// KNOWN DOC CONFLICT (not fixed here — separate work item):
// docs/guides/API_KEY_AND_OPENCODE_GUIDE.md:79-96 documents the provider
// `name` as "HelixAgent AI Debate Ensemble" and a single-model shape. It was
// already stale before this change (it predates the 4-agent map); this change
// makes the provider-name line newly contradicted.
const (
	openCodeCanonicalProviderName = "HelixAgent"
	openCodeCanonicalDebateID     = "helixagent-debate"
	openCodeCanonicalLLMID        = "helixagent-llm"
	openCodeCanonicalDebateName   = "HelixAgent AI Debate Ensemble"
	// Corrected spelling of the pre-existing name, not a new name: `HelixLLM`
	// is the product's own one-word spelling (460 doc occurrences vs 0 for the
	// spaced form). No "HelixAgent LLM" is invented here — that string has zero
	// documentary support (§11.4.6).
	openCodeCanonicalLLMName = "HelixLLM"
	openCodeLegacyDebateID   = "helix-debate"
	openCodeLegacyLLMID      = "helix-llm"
)

// generateOpenCodeConfigFull runs the built binary's -generate-opencode-config
// and decodes stdout. Shared by the canonical-ID guard below.
func generateOpenCodeConfigFull(t *testing.T, config *TestConfig) OpenCodeConfigFull {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
	output, err := cmd.Output()
	require.NoError(t, err, "Failed to generate OpenCode config")

	var openCodeConfig OpenCodeConfigFull
	require.NoError(t, json.Unmarshal(output, &openCodeConfig),
		"Config should be valid JSON")
	return openCodeConfig
}

// TestOpenCodeConfigUsesCanonicalModelIDs is the §11.4.115 polarity-switched
// regression guard for the canonical-model-ID contract above, and the standing
// §11.4.135 guard against a re-drift to the legacy aliases.
//
//	RED_MODE=1 — assert the DEFECT IS PRESENT (reproduce on a pre-fix binary).
//	RED_MODE=0 — assert the defect is ABSENT. THIS IS NOW THE DEFAULT.
//
// The default flipped to GREEN in the same commit as the generator fix.
//
// CAPTURED BASELINES (2026-08-09, bin/helixagent built from the pre-fix tree):
//
//	RED_MODE=1 -> PASS (defect reproduced: provider name "Helix Agent";
//	                    models map[helix-debate helix-llm]; agent coder bound to
//	                    "helixagent/helix-debate")
//	RED_MODE=0 -> FAIL:
//	  "Not equal: expected: \"HelixAgent\" actual: \"Helix Agent\""
//	  "generated OpenCode config MUST declare the canonical debate model id
//	   \"helixagent-debate\" ... got keys [helix-debate helix-llm]"
//
// The RED_MODE=0 assertions failed on the pre-fix binary and pass on the fixed
// one, so this is not a blind test written to agree with the new code.
func TestOpenCodeConfigUsesCanonicalModelIDs(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	// A standing §11.4.135 guard MUST NOT report green when the artifact under
	// test is absent — that is a fail-open false negative (§11.4.3 / §11.4.201).
	// SKIP loudly instead of returning a silent PASS, matching the
	// t.Skipf(... SKIP-OK: ...) convention already used at :277/:310/:348.
	if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
		t.Skipf("HelixAgent binary not found at %s — run `make build` first; "+
			"this guard cannot certify a canonical config without the artifact "+
			"(SKIP-OK: #infra-unavailable)", config.BinaryPath)
	}

	openCodeConfig := generateOpenCodeConfigFull(t, config)
	provider, exists := openCodeConfig.Provider["helixagent"]
	require.True(t, exists, "HelixAgent provider must exist")

	modelIDs := make([]string, 0, len(provider.Models))
	for id := range provider.Models {
		modelIDs = append(modelIDs, id)
	}

	agentModels := make([]string, 0, len(openCodeConfig.Agent))
	for _, agent := range openCodeConfig.Agent {
		agentModels = append(agentModels, agent.Model)
	}

	if os.Getenv("RED_MODE") == "1" {
		// Reproduce the defect on the pre-fix artifact.
		_, hasLegacyDebate := provider.Models[openCodeLegacyDebateID]
		assert.True(t, hasLegacyDebate,
			"RED baseline: the generator is expected to still emit the legacy "+
				"model id %q. If this fails the defect is already fixed — "+
				"re-run with RED_MODE=0.", openCodeLegacyDebateID)

		_, hasCanonicalDebate := provider.Models[openCodeCanonicalDebateID]
		assert.False(t, hasCanonicalDebate,
			"RED baseline: the canonical id %q is expected to be ABSENT pre-fix",
			openCodeCanonicalDebateID)

		assert.NotEqual(t, openCodeCanonicalProviderName, provider.Name,
			"RED baseline: provider name is expected to still be the "+
				"non-canonical spaced form pre-fix; got %q", provider.Name)
		return
	}

	// --- GREEN guard: the canonical contract holds. ---

	assert.Equal(t, openCodeCanonicalProviderName, provider.Name,
		"generated OpenCode config MUST use the canonical product name; "+
			"`Helix Agent` (spaced) has zero occurrences in docs/ + README.md")

	debate, hasDebate := provider.Models[openCodeCanonicalDebateID]
	assert.True(t, hasDebate,
		"generated OpenCode config MUST declare the canonical debate model id "+
			"%q (openai_compatible.go:532 — \"canonical for CLI configs\"); "+
			"got keys %v", openCodeCanonicalDebateID, modelIDs)
	if hasDebate {
		assert.Equal(t, openCodeCanonicalDebateName, debate.Name,
			"debate model display name MUST match the shipped reference config "+
				"configs/cli-agents/opencode.json")
	}

	llm, hasLLM := provider.Models[openCodeCanonicalLLMID]
	assert.True(t, hasLLM,
		"generated OpenCode config MUST declare the canonical provider-chain "+
			"model id %q; got keys %v", openCodeCanonicalLLMID, modelIDs)
	if hasLLM {
		assert.Equal(t, openCodeCanonicalLLMName, llm.Name,
			"provider-chain model display name MUST use the product's own "+
				"one-word spelling `HelixLLM`")
	}

	// The legacy aliases MUST NOT be re-introduced into a freshly generated
	// config. They remain valid on the wire (openai_compatible.go:544-549) for
	// already-deployed configs; they are simply not what we generate.
	assert.NotContains(t, modelIDs, openCodeLegacyDebateID,
		"generated config MUST NOT re-introduce the legacy alias %q — it "+
			"bypasses the openai_compatible.go:829 explicit-debate override",
		openCodeLegacyDebateID)
	assert.NotContains(t, modelIDs, openCodeLegacyLLMID,
		"generated config MUST NOT re-introduce the legacy alias %q",
		openCodeLegacyLLMID)

	// Every agent binding must reference a model this config actually declares,
	// in provider-qualified canonical form. A binding to an undeclared model is
	// a config OpenCode cannot resolve.
	//
	// The non-empty requirement is load-bearing: on an empty agent map both the
	// loop below and the NotContains that follows it would pass vacuously.
	require.NotEmpty(t, openCodeConfig.Agent,
		"generated config MUST declare agents — an empty agent map makes every "+
			"binding assertion below vacuous")

	for name, agent := range openCodeConfig.Agent {
		assert.True(t,
			strings.HasPrefix(agent.Model, "helixagent/"),
			"agent %q model %q MUST be provider-qualified", name, agent.Model)
		declared := strings.TrimPrefix(agent.Model, "helixagent/")
		_, ok := provider.Models[declared]
		assert.True(t, ok,
			"agent %q binds model %q which the provider does not declare "+
				"(declared: %v)", name, agent.Model, modelIDs)
	}

	assert.NotContains(t, agentModels, "helixagent/"+openCodeLegacyDebateID,
		"agent bindings MUST NOT use the legacy debate alias — this is the "+
			"exact path that loses the openai_compatible.go:829 "+
			"explicit-debate override")
}
