package verifier

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultDiscoveryConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultDiscoveryConfig()
	if cfg == nil {
		t.Fatal("DefaultDiscoveryConfig returned nil")
	}
}

func TestDiscoveryConfig_Fields(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	svc := NewModelDiscoveryService(nil, nil, nil, nil)
	if svc == nil {
		t.Fatal("NewModelDiscoveryService returned nil")
	}
}

func TestNewModelDiscoveryService_CustomConfig(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	svc := NewModelDiscoveryService(nil, nil, nil, nil)

	stats := svc.GetDiscoveryStats()
	if stats == nil {
		t.Fatal("stats is nil")
	}
}

func TestModelDiscoveryService_GetModelForDebate(t *testing.T) {
	t.Parallel()
	svc := NewModelDiscoveryService(nil, nil, nil, nil)

	_, found := svc.GetModelForDebate("non-existent")
	if found {
		t.Error("expected model not to be found")
	}
}

func TestModelDiscoveryService_Stop(t *testing.T) {
	t.Parallel()
	svc := NewModelDiscoveryService(nil, nil, nil, nil)

	// Should not panic even if not started
	svc.Stop()
}

func TestDiscoveredModel_Fields(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// =====================================================
// ADDITIONAL DISCOVERY TESTS FOR COMPREHENSIVE COVERAGE
// =====================================================

func TestModelDiscoveryService_getDiscoveryEndpoint(t *testing.T) {
	t.Parallel()
	svc := NewModelDiscoveryService(nil, nil, nil, nil)

	tests := []struct {
		name           string
		cred           ProviderCredentials
		expectedPrefix string
	}{
		{
			name:           "openai",
			cred:           ProviderCredentials{ProviderName: "openai"},
			expectedPrefix: "https://api.openai.com",
		},
		{
			name:           "anthropic",
			cred:           ProviderCredentials{ProviderName: "anthropic"},
			expectedPrefix: "https://api.anthropic.com",
		},
		{
			name:           "gemini",
			cred:           ProviderCredentials{ProviderName: "gemini"},
			expectedPrefix: "https://generativelanguage.googleapis.com",
		},
		{
			name:           "groq",
			cred:           ProviderCredentials{ProviderName: "groq"},
			expectedPrefix: "https://api.groq.com",
		},
		{
			name:           "together",
			cred:           ProviderCredentials{ProviderName: "together"},
			expectedPrefix: "https://api.together.xyz",
		},
		{
			name:           "mistral",
			cred:           ProviderCredentials{ProviderName: "mistral"},
			expectedPrefix: "https://api.mistral.ai",
		},
		{
			name:           "deepseek",
			cred:           ProviderCredentials{ProviderName: "deepseek"},
			expectedPrefix: "https://api.deepseek.com",
		},
		{
			name:           "ollama",
			cred:           ProviderCredentials{ProviderName: "ollama"},
			expectedPrefix: "http://localhost:11434",
		},
		{
			name:           "openrouter",
			cred:           ProviderCredentials{ProviderName: "openrouter"},
			expectedPrefix: "https://openrouter.ai",
		},
		{
			name:           "xai",
			cred:           ProviderCredentials{ProviderName: "xai"},
			expectedPrefix: "https://api.x.ai",
		},
		{
			name:           "cerebras",
			cred:           ProviderCredentials{ProviderName: "cerebras"},
			expectedPrefix: "https://api.cerebras.ai",
		},
		{
			name:           "custom base URL",
			cred:           ProviderCredentials{ProviderName: "custom", BaseURL: "https://custom.api.com"},
			expectedPrefix: "https://custom.api.com/models",
		},
		{
			name:           "unknown provider",
			cred:           ProviderCredentials{ProviderName: "unknown"},
			expectedPrefix: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			endpoint := svc.getDiscoveryEndpoint(tt.cred)
			if tt.expectedPrefix == "" && endpoint != "" {
				t.Errorf("expected empty endpoint for unknown provider, got %s", endpoint)
			}
			if tt.expectedPrefix != "" && !strings.HasPrefix(endpoint, tt.expectedPrefix) {
				t.Errorf("expected endpoint to start with %s, got %s", tt.expectedPrefix, endpoint)
			}
		})
	}
}

func TestModelDiscoveryService_isChatModel(t *testing.T) {
	t.Parallel()
	svc := NewModelDiscoveryService(nil, nil, nil, nil)

	tests := []struct {
		name     string
		modelID  string
		provider string
		expected bool
	}{
		{"gpt-4", "gpt-4", "openai", true},
		{"gpt-4o", "gpt-4o", "openai", true},
		{"claude-3", "claude-3-opus", "anthropic", true},
		{"embedding model", "text-embedding-ada-002", "openai", false},
		{"embed model", "embed-v1", "openai", false},
		{"moderation model", "text-moderation-001", "openai", false},
		{"tts model", "tts-1", "openai", false},
		{"whisper model", "whisper-1", "openai", false},
		{"dall-e model", "dall-e-3", "openai", false},
		{"davinci model", "text-davinci-003", "openai", false},
		{"babbage model", "babbage-002", "openai", false},
		{"curie model", "curie-001", "openai", false},
		{"ada model", "ada-001", "openai", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := svc.isChatModel(tt.modelID, tt.provider)
			if result != tt.expected {
				t.Errorf("isChatModel(%s, %s) = %v, want %v", tt.modelID, tt.provider, result, tt.expected)
			}
		})
	}
}

func TestModelDiscoveryService_calculateVoteWeight(t *testing.T) {
	t.Parallel()
	svc := NewModelDiscoveryService(nil, nil, nil, nil)

	tests := []struct {
		name      string
		model     *DiscoveredModel
		minWeight float64
		maxWeight float64
	}{
		{
			name: "high score with code visibility",
			model: &DiscoveredModel{
				OverallScore: 9.5,
				CodeVisible:  true,
			},
			minWeight: 0.95,
			maxWeight: 1.0,
		},
		{
			name: "high score without code visibility",
			model: &DiscoveredModel{
				OverallScore: 9.5,
				CodeVisible:  false,
			},
			minWeight: 0.9,
			maxWeight: 1.0,
		},
		{
			name: "low score",
			model: &DiscoveredModel{
				OverallScore: 5.0,
				CodeVisible:  false,
			},
			minWeight: 0.4,
			maxWeight: 0.6,
		},
		{
			name: "very high score caps at 1.0",
			model: &DiscoveredModel{
				OverallScore: 10.0,
				CodeVisible:  true,
			},
			minWeight: 1.0,
			maxWeight: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			weight := svc.calculateVoteWeight(tt.model)
			if weight < tt.minWeight || weight > tt.maxWeight {
				t.Errorf("expected weight in range [%f, %f], got %f", tt.minWeight, tt.maxWeight, weight)
			}
		})
	}
}

func TestModelDiscoveryService_selectTopModels(t *testing.T) {
	t.Parallel()
	cfg := &DiscoveryConfig{
		MaxModelsForEnsemble: 3,
		MinScore:             7.0,
		RequireDiversity:     true,
	}
	svc := NewModelDiscoveryService(nil, nil, nil, cfg)

	models := []*DiscoveredModel{
		{ModelID: "model1", Provider: "openai", OverallScore: 9.5},
		{ModelID: "model2", Provider: "anthropic", OverallScore: 9.0},
		{ModelID: "model3", Provider: "gemini", OverallScore: 8.5},
		{ModelID: "model4", Provider: "openai", OverallScore: 8.0}, // Same provider as model1
		{ModelID: "model5", Provider: "groq", OverallScore: 6.0},   // Below min score
	}

	selected := svc.selectTopModels(models)

	if len(selected) != 3 {
		t.Errorf("expected 3 selected models, got %d", len(selected))
	}

	// Check ranking
	for i, s := range selected {
		if s.Rank != i+1 {
			t.Errorf("expected rank %d, got %d", i+1, s.Rank)
		}
		if !s.Selected {
			t.Error("Selected should be true")
		}
	}
}

func TestModelDiscoveryService_selectTopModels_NoDiversity(t *testing.T) {
	t.Parallel()
	cfg := &DiscoveryConfig{
		MaxModelsForEnsemble: 3,
		MinScore:             7.0,
		RequireDiversity:     false,
	}
	svc := NewModelDiscoveryService(nil, nil, nil, cfg)

	models := []*DiscoveredModel{
		{ModelID: "model1", Provider: "openai", OverallScore: 9.5},
		{ModelID: "model2", Provider: "openai", OverallScore: 9.0},
		{ModelID: "model3", Provider: "openai", OverallScore: 8.5},
	}

	selected := svc.selectTopModels(models)

	if len(selected) != 3 {
		t.Errorf("expected 3 selected models (no diversity required), got %d", len(selected))
	}
}

func TestModelDiscoveryService_selectTopModels_AllBelowMinScore(t *testing.T) {
	t.Parallel()
	cfg := &DiscoveryConfig{
		MaxModelsForEnsemble: 3,
		MinScore:             9.0,
		RequireDiversity:     false,
	}
	svc := NewModelDiscoveryService(nil, nil, nil, cfg)

	models := []*DiscoveredModel{
		{ModelID: "model1", Provider: "openai", OverallScore: 6.0},
		{ModelID: "model2", Provider: "anthropic", OverallScore: 5.0},
	}

	selected := svc.selectTopModels(models)

	if len(selected) != 0 {
		t.Errorf("expected 0 selected models (all below min score), got %d", len(selected))
	}
}

func TestModelDiscoveryService_Start_Stop(t *testing.T) {
	t.Parallel()
	cfg := &DiscoveryConfig{
		Enabled:           true,
		DiscoveryInterval: 100 * time.Millisecond,
	}
	svc := NewModelDiscoveryService(nil, nil, nil, cfg)

	// Start with empty credentials
	err := svc.Start([]ProviderCredentials{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Allow some time for the goroutine to start
	time.Sleep(50 * time.Millisecond)

	// Stop
	svc.Stop()

	// Stopping again should be safe
	svc.Stop()
}

func TestContainsIgnoreCase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"Hello World", "world", true},
		{"Hello World", "HELLO", true},
		{"Hello World", "xyz", false},
		{"", "test", false},
		{"test", "", true},
		{"UPPERCASE", "uppercase", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			t.Parallel()
			result := containsIgnoreCase(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("containsIgnoreCase(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestDiscoveryStats_Calculation(t *testing.T) {
	t.Parallel()
	vs := NewVerificationService(nil)
	ss, _ := NewScoringService(nil)
	hs := NewHealthService(nil)
	cfg := DefaultDiscoveryConfig()

	svc := NewModelDiscoveryService(vs, ss, hs, cfg)

	// Manually add some discovered models
	model1 := &DiscoveredModel{
		ModelID:      "model1",
		Provider:     "openai",
		Verified:     true,
		CodeVisible:  true,
		OverallScore: 9.0,
	}
	svc.discoveredModels.Put("model1", model1)
	svc.discoveredModels.Put("model2", &DiscoveredModel{
		ModelID:      "model2",
		Provider:     "anthropic",
		Verified:     true,
		CodeVisible:  false,
		OverallScore: 8.0,
	})
	svc.discoveredModels.Put("model3", &DiscoveredModel{
		ModelID:      "model3",
		Provider:     "openai",
		Verified:     false,
		CodeVisible:  false,
		OverallScore: 5.0,
	})
	svc.selectedModels.Replace([]*SelectedModel{
		{
			DiscoveredModel: model1,
			Rank:            1,
		},
	})

	stats := svc.GetDiscoveryStats()

	if stats.TotalDiscovered != 3 {
		t.Errorf("expected 3 discovered, got %d", stats.TotalDiscovered)
	}
	if stats.TotalVerified != 2 {
		t.Errorf("expected 2 verified, got %d", stats.TotalVerified)
	}
	if stats.CodeVisibleCount != 1 {
		t.Errorf("expected 1 code visible, got %d", stats.CodeVisibleCount)
	}
	if stats.TotalSelected != 1 {
		t.Errorf("expected 1 selected, got %d", stats.TotalSelected)
	}
	if stats.ByProvider["openai"] != 2 {
		t.Errorf("expected 2 openai models, got %d", stats.ByProvider["openai"])
	}
	if stats.ByProvider["anthropic"] != 1 {
		t.Errorf("expected 1 anthropic model, got %d", stats.ByProvider["anthropic"])
	}
}

func TestModelDiscoveryService_GetModelForDebate_Found(t *testing.T) {
	t.Parallel()
	svc := NewModelDiscoveryService(nil, nil, nil, nil)

	// Add a selected model
	svc.selectedModels.Replace([]*SelectedModel{
		{
			DiscoveredModel: &DiscoveredModel{
				ModelID:  "test-model",
				Provider: "openai",
			},
			Selected: true,
		},
	})

	model, found := svc.GetModelForDebate("test-model")
	if !found {
		t.Error("expected to find model")
	}
	if model.ModelID != "test-model" {
		t.Errorf("expected model ID 'test-model', got '%s'", model.ModelID)
	}
}

func TestDiscoveryConfig_DefaultValues(t *testing.T) {
	t.Parallel()
	cfg := DefaultDiscoveryConfig()

	if !cfg.Enabled {
		t.Error("expected Enabled to be true by default")
	}
	if cfg.DiscoveryInterval != 24*time.Hour {
		t.Errorf("expected DiscoveryInterval 24h, got %v", cfg.DiscoveryInterval)
	}
	if cfg.MaxModelsForEnsemble != 5 {
		t.Errorf("expected MaxModelsForEnsemble 5, got %d", cfg.MaxModelsForEnsemble)
	}
	if cfg.MinScore != 7.0 {
		t.Errorf("expected MinScore 7.0, got %f", cfg.MinScore)
	}
	if !cfg.RequireVerification {
		t.Error("expected RequireVerification to be true")
	}
	if !cfg.RequireCodeVisibility {
		t.Error("expected RequireCodeVisibility to be true")
	}
	if !cfg.RequireDiversity {
		t.Error("expected RequireDiversity to be true")
	}
	if len(cfg.ProviderPriority) == 0 {
		t.Error("expected ProviderPriority to be non-empty")
	}
}
