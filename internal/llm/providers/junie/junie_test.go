package junie_test

import (
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/llm/providers/junie"
)

func TestJunieCLIConfig_Default(t *testing.T) {
	t.Parallel()
	config := junie.DefaultJunieCLIConfig()
	if config.Timeout != 180*time.Second {
		t.Errorf("Default timeout should be 180s")
	}
	if config.MaxOutputTokens != 8192 {
		t.Errorf("Default MaxOutputTokens should be 8192")
	}
	if config.Model != "" {
		t.Errorf("Default Model should be empty")
	}
}

func TestDefaultJunieConfig(t *testing.T) {
	t.Parallel()
	config := junie.DefaultJunieConfig()
	if config.Timeout != 180*time.Second {
		t.Errorf("Default timeout should be 180s")
	}
	if config.MaxTokens != 8192 {
		t.Errorf("Default MaxTokens should be 8192")
	}
	if config.Model != "sonnet" {
		t.Errorf("Default Model should be sonnet")
	}
}

func TestDefaultJunieACPConfig(t *testing.T) {
	t.Parallel()
	config := junie.DefaultJunieACPConfig()
	if config.Timeout != 180*time.Second {
		t.Errorf("Default timeout should be 180s")
	}
	if config.MaxTokens != 8192 {
		t.Errorf("Default MaxTokens should be 8192")
	}
	if config.CWD != "." {
		t.Errorf("Default CWD should be .")
	}
}

func TestNewJunieCLIProvider(t *testing.T) {
	t.Parallel()
	config := junie.DefaultJunieCLIConfig()
	p := junie.NewJunieCLIProvider(config)
	if p.GetCurrentModel() == "" {
		t.Errorf("Default model should be set")
	}
}

func TestNewJunieACPProvider(t *testing.T) {
	t.Parallel()
	config := junie.DefaultJunieACPConfig()
	p := junie.NewJunieACPProvider(config)
	if p.GetCurrentModel() == "" {
		t.Errorf("Default model should be set")
	}
}

func TestNewJunieProvider(t *testing.T) {
	t.Parallel()
	config := junie.DefaultJunieConfig()
	p := junie.NewJunieProvider(config)
	if p.GetCurrentModel() != "sonnet" {
		t.Errorf("Default model should be sonnet")
	}
}

func TestJunieCLIProvider_GetName(t *testing.T) {
	t.Parallel()
	p := junie.NewJunieCLIProvider(junie.DefaultJunieCLIConfig())
	name := p.GetName()
	if name != "junie-cli" {
		t.Errorf("Expected name junie-cli, got %s", name)
	}
}

func TestJunieCLIProvider_GetProviderType(t *testing.T) {
	t.Parallel()
	p := junie.NewJunieCLIProvider(junie.DefaultJunieCLIConfig())
	providerType := p.GetProviderType()
	if providerType != "junie" {
		t.Errorf("Expected provider type junie, got %s", providerType)
	}
}

func TestJunieCLIProvider_GetCurrentModel(t *testing.T) {
	t.Parallel()
	config := junie.DefaultJunieCLIConfig()
	config.Model = "opus"
	p := junie.NewJunieCLIProvider(config)
	model := p.GetCurrentModel()
	if model != "opus" {
		t.Errorf("Expected model opus, got %s", model)
	}
}

func TestJunieCLIProvider_SetModel(t *testing.T) {
	t.Parallel()
	p := junie.NewJunieCLIProvider(junie.DefaultJunieCLIConfig())
	p.SetModel("gemini-pro")
	if p.GetCurrentModel() != "gemini-pro" {
		t.Errorf("Expected model gemini-pro, got %s", p.GetCurrentModel())
	}
}

func TestJunieACPProvider_GetName(t *testing.T) {
	t.Parallel()
	p := junie.NewJunieACPProvider(junie.DefaultJunieACPConfig())
	name := p.GetName()
	if name != "junie-acp" {
		t.Errorf("Expected name junie-acp, got %s", name)
	}
}

func TestJunieACPProvider_GetProviderType(t *testing.T) {
	t.Parallel()
	p := junie.NewJunieACPProvider(junie.DefaultJunieACPConfig())
	providerType := p.GetProviderType()
	if providerType != "junie" {
		t.Errorf("Expected provider type junie, got %s", providerType)
	}
}

func TestJunieProvider_GetName(t *testing.T) {
	t.Parallel()
	p := junie.NewJunieProvider(junie.DefaultJunieConfig())
	name := p.GetName()
	if name != "junie" {
		t.Errorf("Expected name junie, got %s", name)
	}
}

func TestJunieProvider_GetProviderType(t *testing.T) {
	t.Parallel()
	p := junie.NewJunieProvider(junie.DefaultJunieConfig())
	providerType := p.GetProviderType()
	if providerType != "junie" {
		t.Errorf("Expected provider type junie, got %s", providerType)
	}
}

func TestGetKnownJunieModels(t *testing.T) {
	t.Parallel()
	models := junie.GetKnownJunieModels()
	if len(models) == 0 {
		t.Errorf("Expected at least one model, got %d", len(models))
	}
	for _, model := range models {
		if model == "" {
			t.Errorf("Model should not be empty")
		}
	}
}

func TestGetBYOKModels(t *testing.T) {
	t.Parallel()
	byok := junie.GetBYOKModels()
	if len(byok) == 0 {
		t.Errorf("Expected at least one provider, got %d", len(byok))
	}
	for provider := range byok {
		if provider != "anthropic" && provider != "openai" && provider != "google" && provider != "grok" {
			t.Errorf("Expected provider in byok map, got %s", provider)
		}
	}
}

func TestIsJunieInstalled(t *testing.T) {
	t.Parallel()
	installed := junie.IsJunieInstalled()
	if installed {
		t.Logf("Junie should be installed: %v", installed)
	}
}

func TestIsJunieAuthenticated(t *testing.T) {
	t.Parallel()
	authenticated := junie.IsJunieAuthenticated()
	if authenticated {
		t.Logf("Junie should be authenticated: %v", authenticated)
	}
}

func TestJunieProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	p := junie.NewJunieProvider(junie.DefaultJunieConfig())
	caps := p.GetCapabilities()
	if len(caps.SupportedModels) == 0 {
		t.Errorf("Expected at least one model, got %d", len(caps.SupportedModels))
	}
	if !caps.SupportsStreaming {
		t.Errorf("Expected streaming support")
	}
	if !caps.SupportsTools {
		t.Errorf("Expected tools support")
	}
}

func TestJunieProvider_ValidateConfig(t *testing.T) {
	t.Parallel()
	p := junie.NewJunieProvider(junie.DefaultJunieConfig())
	valid, issues := p.ValidateConfig(nil)
	if junie.IsJunieInstalled() && (os.Getenv("JUNIE_API_KEY") != "" || junie.IsJunieAuthenticated()) {
		if !valid {
			t.Logf("ValidateConfig returned valid=false, issues: %v", issues)
		}
	} else {
		if valid {
			t.Errorf("Expected invalid config when Junie not available")
		}
		if len(issues) == 0 {
			t.Errorf("Expected issues when Junie not available")
		}
	}
}
