package kimicode

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
)

func TestKimiCodeCLIProvider_DefaultConfig(t *testing.T) {
	config := DefaultKimiCodeCLIConfig()
	if config.Model != KimiCodeDefaultModel {
		t.Errorf("Expected default model %s, got %s", KimiCodeDefaultModel, config.Model)
	}
	if config.Timeout != 180*time.Second {
		t.Errorf("Expected timeout 180s, got %v", config.Timeout)
	}
	if config.MaxOutputTokens != KimiCodeMaxOutput {
		t.Errorf("Expected max output tokens %d, got %d", KimiCodeMaxOutput, config.MaxOutputTokens)
	}
}

func TestKimiCodeCLIProvider_NewProvider(t *testing.T) {
	provider := NewKimiCodeCLIProvider(DefaultKimiCodeCLIConfig())
	if provider == nil {
		t.Fatal("Expected provider to be created")
	}
	if provider.model != KimiCodeDefaultModel {
		t.Errorf("Expected model %s, got %s", KimiCodeDefaultModel, provider.model)
	}
}

func TestKimiCodeCLIProvider_NewProviderWithModel(t *testing.T) {
	provider := NewKimiCodeCLIProviderWithModel("custom-model")
	if provider == nil {
		t.Fatal("Expected provider to be created")
	}
	if provider.model != "custom-model" {
		t.Errorf("Expected model custom-model, got %s", provider.model)
	}
}

func TestKimiCodeCLIProvider_GetName(t *testing.T) {
	provider := NewKimiCodeCLIProvider(DefaultKimiCodeCLIConfig())
	if provider.GetName() != "kimi-code-cli" {
		t.Errorf("Expected name kimi-code-cli, got %s", provider.GetName())
	}
}

func TestKimiCodeCLIProvider_GetProviderType(t *testing.T) {
	provider := NewKimiCodeCLIProvider(DefaultKimiCodeCLIConfig())
	if provider.GetProviderType() != "kimi-code" {
		t.Errorf("Expected type kimi-code, got %s", provider.GetProviderType())
	}
}

func TestKimiCodeCLIProvider_GetCurrentModel(t *testing.T) {
	provider := NewKimiCodeCLIProvider(DefaultKimiCodeCLIConfig())
	if provider.GetCurrentModel() != KimiCodeDefaultModel {
		t.Errorf("Expected model %s, got %s", KimiCodeDefaultModel, provider.GetCurrentModel())
	}
}

func TestKimiCodeCLIProvider_SetModel(t *testing.T) {
	provider := NewKimiCodeCLIProvider(DefaultKimiCodeCLIConfig())
	provider.SetModel("new-model")
	if provider.GetCurrentModel() != "new-model" {
		t.Errorf("Expected model new-model, got %s", provider.GetCurrentModel())
	}
}

func TestKimiCodeCLIProvider_GetCapabilities(t *testing.T) {
	provider := NewKimiCodeCLIProvider(DefaultKimiCodeCLIConfig())
	caps := provider.GetCapabilities()
	if caps == nil {
		t.Fatal("Expected capabilities to be returned")
	}
	if !caps.SupportsStreaming {
		t.Error("Expected streaming support")
	}
	if caps.Limits.MaxTokens != KimiCodeMaxOutput {
		t.Errorf("Expected max tokens %d, got %d", KimiCodeMaxOutput, caps.Limits.MaxTokens)
	}
}

func TestKimiCodeCLIProvider_GetAvailableModels(t *testing.T) {
	provider := NewKimiCodeCLIProvider(DefaultKimiCodeCLIConfig())
	models := provider.GetAvailableModels()
	if len(models) == 0 {
		t.Error("Expected at least one model")
	}
	found := false
	for _, m := range models {
		if m == KimiCodeDefaultModel {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected default model %s in available models", KimiCodeDefaultModel)
	}
}

func TestKimiCodeCLIProvider_IsModelAvailable(t *testing.T) {
	provider := NewKimiCodeCLIProvider(DefaultKimiCodeCLIConfig())
	if !provider.IsModelAvailable(KimiCodeDefaultModel) {
		t.Errorf("Expected model %s to be available", KimiCodeDefaultModel)
	}
	if provider.IsModelAvailable("non-existent-model") {
		t.Error("Expected non-existent model to be unavailable")
	}
}

func TestKimiCodeCLIProvider_GetBestAvailableModel(t *testing.T) {
	provider := NewKimiCodeCLIProvider(DefaultKimiCodeCLIConfig())
	model := provider.GetBestAvailableModel()
	if model == "" {
		t.Error("Expected a best available model")
	}
}

func TestKimiCodeCLIProvider_ParseJSONResponse(t *testing.T) {
	provider := NewKimiCodeCLIProvider(DefaultKimiCodeCLIConfig())

	tests := []struct {
		name          string
		input         string
		expectedText  string
		expectedThink string
	}{
		{
			name:          "simple text response",
			input:         `{"role":"assistant","content":[{"type":"text","text":"Hello World"}]}`,
			expectedText:  "Hello World",
			expectedThink: "",
		},
		{
			name:          "text with thinking",
			input:         `{"role":"assistant","content":[{"type":"think","think":"Processing..."},{"type":"text","text":"Done"}]}`,
			expectedText:  "Done",
			expectedThink: "Processing...\n",
		},
		{
			name: "multiline response",
			input: `{"role":"assistant","content":[{"type":"text","text":"Line 1"}]}
{"role":"assistant","content":[{"type":"text","text":"Line 2"}]}`,
			expectedText:  "Line 1Line 2",
			expectedThink: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, think := provider.parseJSONResponse(tt.input)
			if text != tt.expectedText {
				t.Errorf("Expected text %q, got %q", tt.expectedText, text)
			}
			if think != tt.expectedThink {
				t.Errorf("Expected think %q, got %q", tt.expectedThink, think)
			}
		})
	}
}

func TestIsKimiCodeInstalled(t *testing.T) {
	installed := IsKimiCodeInstalled()
	_ = installed
}

func TestIsInsideKimiCodeSession(t *testing.T) {
	original := os.Getenv("KIMI_CODE_SESSION")
	defer os.Setenv("KIMI_CODE_SESSION", original)

	os.Setenv("KIMI_CODE_SESSION", "")
	if IsInsideKimiCodeSession() {
		t.Error("Expected not to be inside session")
	}

	os.Setenv("KIMI_CODE_SESSION", "test-session")
	if !IsInsideKimiCodeSession() {
		t.Error("Expected to be inside session")
	}
}

func TestGetKnownKimiCodeModels(t *testing.T) {
	models := GetKnownKimiCodeModels()
	if len(models) == 0 {
		t.Error("Expected at least one known model")
	}
}

func TestKimiCodeCLIProvider_Complete_Integration(t *testing.T) {
	if os.Getenv("KIMI_CODE_USE_OAUTH_CREDENTIALS") != "true" {
		t.Skip("Skipping integration test: KIMI_CODE_USE_OAUTH_CREDENTIALS not set")  // SKIP-OK: #integration-mode-only
	}

	if !IsKimiCodeInstalled() {
		t.Skip("Skipping integration test: kimi CLI not installed")  // SKIP-OK: #integration-mode-only
	}

	if !IsKimiCodeAuthenticated() {
		t.Skip("Skipping integration test: kimi CLI not authenticated")  // SKIP-OK: #integration-mode-only
	}

	provider := NewKimiCodeCLIProvider(DefaultKimiCodeCLIConfig())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := provider.Complete(ctx, &models.LLMRequest{
		Prompt: "Reply with just the word 'OK'",
		ModelParams: models.ModelParameters{
			MaxTokens: 10,
		},
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if resp == nil {
		t.Fatal("Expected response")
	}
	if resp.Content == "" {
		t.Error("Expected non-empty content")
	}
	if !strings.Contains(strings.ToUpper(resp.Content), "OK") {
		t.Errorf("Expected response to contain 'OK', got %s", resp.Content)
	}
}

func TestKimiCodeCLIProvider_HealthCheck_Integration(t *testing.T) {
	if os.Getenv("KIMI_CODE_USE_OAUTH_CREDENTIALS") != "true" {
		t.Skip("Skipping integration test: KIMI_CODE_USE_OAUTH_CREDENTIALS not set")  // SKIP-OK: #integration-mode-only
	}

	if !IsKimiCodeInstalled() {
		t.Skip("Skipping integration test: kimi CLI not installed")  // SKIP-OK: #integration-mode-only
	}

	if !IsKimiCodeAuthenticated() {
		t.Skip("Skipping integration test: kimi CLI not authenticated")  // SKIP-OK: #integration-mode-only
	}

	provider := NewKimiCodeCLIProvider(DefaultKimiCodeCLIConfig())
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	err := provider.HealthCheck()
	if err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}

	_ = ctx
}

func TestKimiCodeCLIProvider_ValidateConfig(t *testing.T) {
	provider := NewKimiCodeCLIProvider(DefaultKimiCodeCLIConfig())
	valid, errors := provider.ValidateConfig(nil)
	_ = valid
	_ = errors
}

func TestCanUseKimiCodeCLI(t *testing.T) {
	original := os.Getenv("KIMI_CODE_USE_OAUTH_CREDENTIALS")
	defer os.Setenv("KIMI_CODE_USE_OAUTH_CREDENTIALS", original)

	os.Setenv("KIMI_CODE_USE_OAUTH_CREDENTIALS", "false")
	if CanUseKimiCodeCLI() {
		t.Error("Expected CanUseKimiCodeCLI to be false when env var is false")
	}
}

func TestKimiCodeCLIProvider_CompleteStream_Integration(t *testing.T) {
	if os.Getenv("KIMI_CODE_USE_OAUTH_CREDENTIALS") != "true" {
		t.Skip("Skipping integration test: KIMI_CODE_USE_OAUTH_CREDENTIALS not set")  // SKIP-OK: #integration-mode-only
	}

	if !IsKimiCodeInstalled() {
		t.Skip("Skipping integration test: kimi CLI not installed")  // SKIP-OK: #integration-mode-only
	}

	if !IsKimiCodeAuthenticated() {
		t.Skip("Skipping integration test: kimi CLI not authenticated")  // SKIP-OK: #integration-mode-only
	}

	provider := NewKimiCodeCLIProvider(DefaultKimiCodeCLIConfig())
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	respChan, err := provider.CompleteStream(ctx, &models.LLMRequest{
		Prompt: "Count from 1 to 5",
		ModelParams: models.ModelParameters{
			MaxTokens: 50,
		},
	})
	if err != nil {
		t.Fatalf("CompleteStream failed: %v", err)
	}

	var chunks []string
	for resp := range respChan {
		if resp.Content != "" {
			chunks = append(chunks, resp.Content)
		}
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}
}
