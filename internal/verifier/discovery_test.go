package verifier

import (
	"testing"
	"time"
)

func TestDefaultDiscoveryConfig(t *testing.T) {
	cfg := DefaultDiscoveryConfig()
	if cfg == nil {
		t.Fatal("DefaultDiscoveryConfig returned nil")
	}
}

func TestDiscoveryConfig_Fields(t *testing.T) {
	cfg := &DiscoveryConfig{
		Enabled:               true,
		DiscoveryInterval:     time.Hour,
		MaxModelsForEnsemble:  5,
		MinScore:              80.0,
		RequireVerification:   true,
		RequireCodeVisibility: true,
		RequireDiversity:      true,
		ProviderPriority:      []string{"openai", "anthropic"},
	}

	if !cfg.Enabled {
		t.Error("Enabled mismatch")
	}
	if cfg.DiscoveryInterval != time.Hour {
		t.Error("DiscoveryInterval mismatch")
	}
	if cfg.MaxModelsForEnsemble != 5 {
		t.Error("MaxModelsForEnsemble mismatch")
	}
	if cfg.MinScore != 80.0 {
		t.Error("MinScore mismatch")
	}
	if !cfg.RequireVerification {
		t.Error("RequireVerification mismatch")
	}
	if !cfg.RequireCodeVisibility {
		t.Error("RequireCodeVisibility mismatch")
	}
	if len(cfg.ProviderPriority) != 2 {
		t.Error("ProviderPriority length mismatch")
	}
}

func TestNewModelDiscoveryService(t *testing.T) {
	svc := NewModelDiscoveryService(nil, nil, nil, nil)
	if svc == nil {
		t.Fatal("NewModelDiscoveryService returned nil")
	}
}

func TestNewModelDiscoveryService_CustomConfig(t *testing.T) {
	customCfg := &DiscoveryConfig{
		Enabled:              true,
		MaxModelsForEnsemble: 3,
		MinScore:             80.0,
	}

	svc := NewModelDiscoveryService(nil, nil, nil, customCfg)
	if svc == nil {
		t.Fatal("service is nil")
	}
}

func TestModelDiscoveryService_GetDiscoveredModels_Empty(t *testing.T) {
	svc := NewModelDiscoveryService(nil, nil, nil, nil)

	models := svc.GetDiscoveredModels()
	if models == nil {
		t.Error("expected non-nil slice")
	}
	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}
}

func TestModelDiscoveryService_GetSelectedModels_Empty(t *testing.T) {
	svc := NewModelDiscoveryService(nil, nil, nil, nil)

	models := svc.GetSelectedModels()
	if models == nil {
		t.Error("expected non-nil slice")
	}
	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}
}

func TestModelDiscoveryService_GetDiscoveryStats(t *testing.T) {
	svc := NewModelDiscoveryService(nil, nil, nil, nil)

	stats := svc.GetDiscoveryStats()
	if stats == nil {
		t.Fatal("stats is nil")
	}
}

func TestModelDiscoveryService_GetModelForDebate(t *testing.T) {
	svc := NewModelDiscoveryService(nil, nil, nil, nil)

	_, found := svc.GetModelForDebate("non-existent")
	if found {
		t.Error("expected model not to be found")
	}
}

func TestModelDiscoveryService_Stop(t *testing.T) {
	svc := NewModelDiscoveryService(nil, nil, nil, nil)

	// Should not panic even if not started
	svc.Stop()
}

func TestDiscoveredModel_Fields(t *testing.T) {
	now := time.Now()
	model := &DiscoveredModel{
		ModelID:       "test-model",
		ModelName:     "Test Model",
		Provider:      "test-provider",
		ProviderID:    "test-id",
		DiscoveredAt:  now,
		Verified:      true,
		VerifiedAt:    now,
		CodeVisible:   true,
		OverallScore:  95.5,
		ScoreSuffix:   "(SC:9.6)",
		Capabilities:  []string{"chat", "code"},
		ContextWindow: 8192,
	}

	if model.ModelID != "test-model" {
		t.Error("ModelID mismatch")
	}
	if model.ModelName != "Test Model" {
		t.Error("ModelName mismatch")
	}
	if model.Provider != "test-provider" {
		t.Error("Provider mismatch")
	}
	if !model.Verified {
		t.Error("Verified mismatch")
	}
	if !model.CodeVisible {
		t.Error("CodeVisible mismatch")
	}
	if model.OverallScore != 95.5 {
		t.Error("OverallScore mismatch")
	}
	if model.ScoreSuffix != "(SC:9.6)" {
		t.Error("ScoreSuffix mismatch")
	}
	if len(model.Capabilities) != 2 {
		t.Error("Capabilities length mismatch")
	}
	if model.ContextWindow != 8192 {
		t.Error("ContextWindow mismatch")
	}
}

func TestSelectedModel_Fields(t *testing.T) {
	now := time.Now()
	discovered := &DiscoveredModel{
		ModelID:      "test-model",
		ModelName:    "Test Model",
		Provider:     "test-provider",
		OverallScore: 95.5,
		CodeVisible:  true,
	}

	model := &SelectedModel{
		DiscoveredModel: discovered,
		Rank:            1,
		VoteWeight:      0.85,
		Selected:        true,
		SelectedAt:      now,
	}

	if model.Rank != 1 {
		t.Error("Rank mismatch")
	}
	if model.VoteWeight != 0.85 {
		t.Error("VoteWeight mismatch")
	}
	if !model.Selected {
		t.Error("Selected mismatch")
	}
	if model.DiscoveredModel.ModelID != "test-model" {
		t.Error("embedded ModelID mismatch")
	}
}

func TestProviderCredentials_Fields(t *testing.T) {
	creds := ProviderCredentials{
		ProviderName: "openai",
		APIKey:       "sk-test-key",
		BaseURL:      "https://api.openai.com",
	}

	if creds.ProviderName != "openai" {
		t.Error("ProviderName mismatch")
	}
	if creds.APIKey != "sk-test-key" {
		t.Error("APIKey mismatch")
	}
	if creds.BaseURL != "https://api.openai.com" {
		t.Error("BaseURL mismatch")
	}
}

func TestDiscoveryStats_Fields(t *testing.T) {
	stats := &DiscoveryStats{
		TotalDiscovered:  100,
		TotalVerified:    80,
		TotalSelected:    10,
		CodeVisibleCount: 75,
		AverageScore:     82.5,
		ByProvider:       map[string]int{"openai": 50, "anthropic": 50},
	}

	if stats.TotalDiscovered != 100 {
		t.Error("TotalDiscovered mismatch")
	}
	if stats.TotalVerified != 80 {
		t.Error("TotalVerified mismatch")
	}
	if stats.TotalSelected != 10 {
		t.Error("TotalSelected mismatch")
	}
	if stats.CodeVisibleCount != 75 {
		t.Error("CodeVisibleCount mismatch")
	}
	if stats.AverageScore != 82.5 {
		t.Error("AverageScore mismatch")
	}
	if len(stats.ByProvider) != 2 {
		t.Error("ByProvider length mismatch")
	}
}

func TestDiscoveryConfig_ZeroValue(t *testing.T) {
	var cfg DiscoveryConfig

	if cfg.Enabled {
		t.Error("zero Enabled should be false")
	}
	if cfg.DiscoveryInterval != 0 {
		t.Error("zero DiscoveryInterval should be 0")
	}
	if cfg.MaxModelsForEnsemble != 0 {
		t.Error("zero MaxModelsForEnsemble should be 0")
	}
}

func TestDiscoveredModel_ZeroValue(t *testing.T) {
	var model DiscoveredModel

	if model.ModelID != "" {
		t.Error("zero ModelID should be empty")
	}
	if model.Verified {
		t.Error("zero Verified should be false")
	}
	if model.OverallScore != 0 {
		t.Error("zero OverallScore should be 0")
	}
}

func TestSelectedModel_ZeroValue(t *testing.T) {
	var model SelectedModel

	if model.Rank != 0 {
		t.Error("zero Rank should be 0")
	}
	if model.VoteWeight != 0 {
		t.Error("zero VoteWeight should be 0")
	}
	if model.Selected {
		t.Error("zero Selected should be false")
	}
}
