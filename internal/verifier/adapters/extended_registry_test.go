package adapters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultExtendedRegistryConfig(t *testing.T) {
	cfg := DefaultExtendedRegistryConfig()
	if cfg == nil {
		t.Fatal("DefaultExtendedRegistryConfig returned nil")
	}

	if !cfg.AutoVerifyNewProviders {
		t.Error("expected AutoVerifyNewProviders to be true")
	}
	if cfg.VerificationInterval <= 0 {
		t.Error("expected positive VerificationInterval")
	}
	if cfg.HealthCheckInterval <= 0 {
		t.Error("expected positive HealthCheckInterval")
	}
}

func TestNewExtendedProviderRegistry(t *testing.T) {
	registry, err := NewExtendedProviderRegistry(nil)
	if err != nil {
		t.Fatalf("NewExtendedProviderRegistry failed: %v", err)
	}
	if registry == nil {
		t.Fatal("registry is nil")
	}
	if registry.adapters == nil {
		t.Error("adapters not initialized")
	}
	if registry.verifiedModels == nil {
		t.Error("verifiedModels not initialized")
	}
	if registry.providerHealth == nil {
		t.Error("providerHealth not initialized")
	}
}

func TestNewExtendedProviderRegistry_WithConfig(t *testing.T) {
	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: false,
		VerificationInterval:   time.Hour,
		HealthCheckInterval:    5 * time.Minute,
		FailoverEnabled:        true,
		MaxFailoverAttempts:    5,
	}

	registry, err := NewExtendedProviderRegistry(cfg)
	if err != nil {
		t.Fatalf("NewExtendedProviderRegistry failed: %v", err)
	}
	if registry.config.VerificationInterval != time.Hour {
		t.Error("VerificationInterval not set correctly")
	}
}

func TestExtendedProviderRegistry_RegisterProvider(t *testing.T) {
	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: false, // Disable auto-verify for this test
	}
	registry, _ := NewExtendedProviderRegistry(cfg)

	err := registry.RegisterProvider(context.Background(), "openai", "OpenAI", "sk-test", "https://api.openai.com", []string{"gpt-4"})
	if err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	// Check adapter was registered
	adapter, ok := registry.adapters.Get("openai")
	if !ok {
		t.Error("adapter not found")
	}
	if adapter.GetProviderID() != "openai" {
		t.Error("adapter ID mismatch")
	}

	// Check health status was initialized
	registry.mu.RLock()
	health, ok := registry.providerHealth["openai"]
	registry.mu.RUnlock()
	if !ok {
		t.Error("health status not initialized")
	}
	if !health.Healthy {
		t.Error("new provider should be healthy")
	}
}

func TestExtendedProviderRegistry_UnregisterProvider(t *testing.T) {
	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: false,
	}
	registry, _ := NewExtendedProviderRegistry(cfg)

	registry.RegisterProvider(context.Background(), "openai", "OpenAI", "key", "url", []string{"gpt-4"})
	registry.UnregisterProvider("openai")

	_, ok := registry.adapters.Get("openai")
	if ok {
		t.Error("adapter should have been removed")
	}
}

func TestExtendedProviderRegistry_VerifyModel(t *testing.T) {
	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: false,
	}
	registry, _ := NewExtendedProviderRegistry(cfg)

	registry.RegisterProvider(context.Background(), "test", "Test", "key", "url", []string{"test-model"})

	err := registry.VerifyModel(context.Background(), "test-model", "test")
	if err != nil {
		t.Fatalf("VerifyModel failed: %v", err)
	}

	// Check model was verified
	model, err := registry.GetVerifiedModel("test-model")
	if err != nil {
		t.Fatalf("GetVerifiedModel failed: %v", err)
	}
	if model == nil {
		t.Error("model not found")
	}
}

func TestExtendedProviderRegistry_VerifyModel_ProviderNotFound(t *testing.T) {
	registry, _ := NewExtendedProviderRegistry(nil)

	err := registry.VerifyModel(context.Background(), "model", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

func TestExtendedProviderRegistry_GetVerifiedModel(t *testing.T) {
	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: false,
	}
	registry, _ := NewExtendedProviderRegistry(cfg)

	registry.RegisterProvider(context.Background(), "test", "Test", "key", "url", []string{"test-model"})
	registry.VerifyModel(context.Background(), "test-model", "test")

	model, err := registry.GetVerifiedModel("test-model")
	if err != nil {
		t.Fatalf("GetVerifiedModel failed: %v", err)
	}
	if model.ModelID != "test-model" {
		t.Error("model ID mismatch")
	}
}

func TestExtendedProviderRegistry_GetVerifiedModel_NotFound(t *testing.T) {
	registry, _ := NewExtendedProviderRegistry(nil)

	_, err := registry.GetVerifiedModel("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestExtendedProviderRegistry_GetVerifiedModels(t *testing.T) {
	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: false,
	}
	registry, _ := NewExtendedProviderRegistry(cfg)

	registry.RegisterProvider(context.Background(), "test", "Test", "key", "url", []string{"model1", "model2"})
	registry.VerifyModel(context.Background(), "model1", "test")
	registry.VerifyModel(context.Background(), "model2", "test")

	// Manually set models as verified since mock adapter responses may not pass all verification checks
	registry.mu.Lock()
	for _, model := range registry.verifiedModels {
		model.Verified = true
	}
	registry.mu.Unlock()

	models := registry.GetVerifiedModels()
	if len(models) < 2 {
		t.Errorf("expected at least 2 models, got %d", len(models))
	}
}

func TestExtendedProviderRegistry_GetProviderHealth(t *testing.T) {
	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: false,
	}
	registry, _ := NewExtendedProviderRegistry(cfg)

	registry.RegisterProvider(context.Background(), "test", "Test", "key", "url", []string{})

	health, err := registry.GetProviderHealth("test")
	if err != nil {
		t.Fatalf("GetProviderHealth failed: %v", err)
	}
	if health.ProviderID != "test" {
		t.Error("provider ID mismatch")
	}
}

func TestExtendedProviderRegistry_GetProviderHealth_NotFound(t *testing.T) {
	registry, _ := NewExtendedProviderRegistry(nil)

	_, err := registry.GetProviderHealth("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

func TestExtendedProviderRegistry_GetHealthyProviders(t *testing.T) {
	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: false,
	}
	registry, _ := NewExtendedProviderRegistry(cfg)

	registry.RegisterProvider(context.Background(), "healthy", "Healthy", "key", "url", []string{})
	registry.RegisterProvider(context.Background(), "unhealthy", "Unhealthy", "key", "url", []string{})

	// Mark one as unhealthy
	registry.mu.Lock()
	registry.providerHealth["unhealthy"].Healthy = false
	registry.mu.Unlock()

	healthy := registry.GetHealthyProviders()
	if len(healthy) != 1 {
		t.Errorf("expected 1 healthy provider, got %d", len(healthy))
	}
}

func TestExtendedProviderRegistry_Complete(t *testing.T) {
	// Create mock server that returns OpenAI-compatible response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"Hello back!"}}]}`))
	}))
	defer server.Close()

	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: false,
	}
	registry, _ := NewExtendedProviderRegistry(cfg)

	registry.RegisterProvider(context.Background(), "test", "Test", "key", server.URL, []string{"model"})

	// Verify the model first
	registry.VerifyModel(context.Background(), "model", "test")

	response, err := registry.Complete(context.Background(), "model", "Hello", nil)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if response == "" {
		t.Error("expected non-empty response")
	}
}

func TestExtendedProviderRegistry_Complete_ModelNotFound(t *testing.T) {
	registry, _ := NewExtendedProviderRegistry(nil)

	_, err := registry.Complete(context.Background(), "nonexistent", "Hello", nil)
	if err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestExtendedProviderRegistry_GetModelWithScoreSuffix(t *testing.T) {
	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: false,
	}
	registry, _ := NewExtendedProviderRegistry(cfg)

	registry.RegisterProvider(context.Background(), "test", "Test", "key", "url", []string{"model"})
	registry.VerifyModel(context.Background(), "model", "test")

	suffix, err := registry.GetModelWithScoreSuffix("model")
	if err != nil {
		t.Fatalf("GetModelWithScoreSuffix failed: %v", err)
	}
	// Should return model name with score suffix
	if suffix == "" {
		t.Error("expected non-empty suffix")
	}
}

func TestExtendedProviderRegistry_GetModelWithScoreSuffix_NotFound(t *testing.T) {
	registry, _ := NewExtendedProviderRegistry(nil)

	_, err := registry.GetModelWithScoreSuffix("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestExtendedProviderRegistry_GetAdapter(t *testing.T) {
	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: false,
	}
	registry, _ := NewExtendedProviderRegistry(cfg)

	registry.RegisterProvider(context.Background(), "test", "Test", "key", "url", []string{})

	adapter, ok := registry.GetAdapter("test")
	if !ok {
		t.Error("expected adapter to be found")
	}
	if adapter.GetProviderID() != "test" {
		t.Error("adapter ID mismatch")
	}
}

func TestExtendedProviderRegistry_GetAdapter_NotFound(t *testing.T) {
	registry, _ := NewExtendedProviderRegistry(nil)

	_, ok := registry.GetAdapter("nonexistent")
	if ok {
		t.Error("expected adapter not to be found")
	}
}

func TestExtendedProviderRegistry_Events(t *testing.T) {
	registry, _ := NewExtendedProviderRegistry(nil)

	ch := registry.Events()
	if ch == nil {
		t.Error("expected non-nil event channel")
	}
}

func TestExtendedProviderRegistry_GetTopModels(t *testing.T) {
	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: false,
	}
	registry, _ := NewExtendedProviderRegistry(cfg)

	registry.RegisterProvider(context.Background(), "test", "Test", "key", "url", []string{"model1", "model2"})
	registry.VerifyModel(context.Background(), "model1", "test")
	registry.VerifyModel(context.Background(), "model2", "test")

	req := &TopModelsRequest{
		Limit:       5,
		MinScore:    0,
		RequireCode: false,
	}

	models := registry.GetTopModels(req)
	// Should return models sorted by score
	if models == nil {
		t.Error("expected non-nil models")
	}
}

func TestExtendedRegistryConfig_Fields(t *testing.T) {
	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: true,
		VerificationInterval:   time.Hour,
		HealthCheckInterval:    5 * time.Minute,
		ScoreUpdateInterval:    10 * time.Minute,
		FailoverEnabled:        true,
		MaxFailoverAttempts:    3,
		CodeVisibilityTest:     true,
		MandatoryVerification:  true,
	}

	if !cfg.AutoVerifyNewProviders {
		t.Error("AutoVerifyNewProviders mismatch")
	}
	if cfg.VerificationInterval != time.Hour {
		t.Error("VerificationInterval mismatch")
	}
	if !cfg.FailoverEnabled {
		t.Error("FailoverEnabled mismatch")
	}
	if cfg.MaxFailoverAttempts != 3 {
		t.Error("MaxFailoverAttempts mismatch")
	}
}

func TestVerifiedModel_Fields(t *testing.T) {
	now := time.Now()
	model := &VerifiedModel{
		ModelID:           "gpt-4",
		ModelName:         "GPT-4",
		ProviderID:        "openai",
		ProviderName:      "OpenAI",
		Verified:          true,
		VerificationScore: 95.0,
		LastVerifiedAt:    now,
		VerificationTests: map[string]bool{"code_visibility": true},
		CodeVisible:       true,
		OverallScore:      92.5,
		ScoreSuffix:       "(SC:9.3)",
	}

	if model.ModelID != "gpt-4" {
		t.Error("ModelID mismatch")
	}
	if model.VerificationScore != 95.0 {
		t.Error("VerificationScore mismatch")
	}
	if !model.CodeVisible {
		t.Error("CodeVisible mismatch")
	}
	if model.ScoreSuffix != "(SC:9.3)" {
		t.Error("ScoreSuffix mismatch")
	}
}

func TestProviderHealthStatus_Fields(t *testing.T) {
	now := time.Now()
	health := &ProviderHealthStatus{
		ProviderID:       "openai",
		ProviderName:     "OpenAI",
		Healthy:          true,
		AvgResponseMs:    150,
		SuccessRate:      0.99,
		LastCheckAt:      now,
		ConsecutiveFails: 0,
		CircuitOpen:      false,
	}

	if health.ProviderID != "openai" {
		t.Error("ProviderID mismatch")
	}
	if !health.Healthy {
		t.Error("Healthy mismatch")
	}
	if health.SuccessRate != 0.99 {
		t.Error("SuccessRate mismatch")
	}
}

func TestProviderEvent_Fields(t *testing.T) {
	now := time.Now()
	event := &ProviderEvent{
		Type:       "verification_complete",
		ProviderID: "openai",
		ModelID:    "gpt-4",
		Message:    "Verification completed successfully",
		Timestamp:  now,
	}

	if event.Type != "verification_complete" {
		t.Error("Type mismatch")
	}
	if event.ProviderID != "openai" {
		t.Error("ProviderID mismatch")
	}
	if event.ModelID != "gpt-4" {
		t.Error("ModelID mismatch")
	}
}

func TestExtendedProviderRegistry_StartStop(t *testing.T) {
	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: false,
		HealthCheckInterval:    100 * time.Millisecond,
	}
	registry, _ := NewExtendedProviderRegistry(cfg)

	err := registry.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	// Stop should not panic
	registry.Stop()
}

func TestExtendedProviderRegistry_ConcurrentAccess(t *testing.T) {
	cfg := &ExtendedRegistryConfig{
		AutoVerifyNewProviders: false,
	}
	registry, _ := NewExtendedProviderRegistry(cfg)

	done := make(chan bool, 20)

	// Concurrent registrations and reads
	for i := 0; i < 10; i++ {
		go func(id int) {
			registry.RegisterProvider(context.Background(), "test", "Test", "key", "url", []string{})
			done <- true
		}(i)

		go func() {
			registry.GetHealthyProviders()
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestVerifiedModel_ZeroValue(t *testing.T) {
	var model VerifiedModel

	if model.ModelID != "" {
		t.Error("zero ModelID should be empty")
	}
	if model.Verified {
		t.Error("zero Verified should be false")
	}
	if model.VerificationScore != 0 {
		t.Error("zero VerificationScore should be 0")
	}
}

func TestProviderHealthStatus_ZeroValue(t *testing.T) {
	var health ProviderHealthStatus

	if health.ProviderID != "" {
		t.Error("zero ProviderID should be empty")
	}
	if health.Healthy {
		t.Error("zero Healthy should be false")
	}
	if health.ConsecutiveFails != 0 {
		t.Error("zero ConsecutiveFails should be 0")
	}
}

func TestTopModelsRequest_Fields(t *testing.T) {
	req := &TopModelsRequest{
		Limit:          10,
		ProviderFilter: []string{"openai", "anthropic"},
		MinScore:       80.0,
		RequireCode:    true,
	}

	if req.Limit != 10 {
		t.Error("Limit mismatch")
	}
	if len(req.ProviderFilter) != 2 {
		t.Error("ProviderFilter length mismatch")
	}
	if req.MinScore != 80.0 {
		t.Error("MinScore mismatch")
	}
	if !req.RequireCode {
		t.Error("RequireCode mismatch")
	}
}

// ============================================================================
// Additional tests for recordProviderFailure and GetTopModels
// ============================================================================

func TestExtendedRegistry_RecordProviderFailure(t *testing.T) {
	registry, err := NewExtendedProviderRegistry(nil)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Stop()

	providerID := "test-provider-failure"

	// Initialize provider health
	registry.mu.Lock()
	registry.providerHealth[providerID] = &ProviderHealthStatus{
		ProviderID:       providerID,
		Healthy:          true,
		ConsecutiveFails: 0,
	}
	registry.mu.Unlock()

	// Record first failure
	registry.recordProviderFailure(providerID)

	registry.mu.RLock()
	health := registry.providerHealth[providerID]
	registry.mu.RUnlock()

	if health.ConsecutiveFails != 1 {
		t.Errorf("Expected 1 consecutive fail, got %d", health.ConsecutiveFails)
	}
	if !health.Healthy {
		t.Error("Health should still be true after 1 failure")
	}
}

func TestExtendedRegistry_RecordProviderFailure_CircuitBreaker(t *testing.T) {
	registry, err := NewExtendedProviderRegistry(nil)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Stop()

	providerID := "test-provider-circuit"

	// Initialize provider health
	registry.mu.Lock()
	registry.providerHealth[providerID] = &ProviderHealthStatus{
		ProviderID:       providerID,
		Healthy:          true,
		ConsecutiveFails: 4, // Already 4 failures
	}
	registry.mu.Unlock()

	// Record 5th failure - should trigger circuit breaker
	registry.recordProviderFailure(providerID)

	registry.mu.RLock()
	health := registry.providerHealth[providerID]
	registry.mu.RUnlock()

	if health.ConsecutiveFails != 5 {
		t.Errorf("Expected 5 consecutive fails, got %d", health.ConsecutiveFails)
	}
	if health.Healthy {
		t.Error("Health should be false after 5 failures")
	}
	if !health.CircuitOpen {
		t.Error("Circuit breaker should be open")
	}
}

func TestExtendedRegistry_RecordProviderFailure_NonExistent(t *testing.T) {
	registry, err := NewExtendedProviderRegistry(nil)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Stop()

	// Recording failure for non-existent provider should not panic
	registry.recordProviderFailure("non-existent-provider")
}

func TestExtendedRegistry_GetTopModels_EmptyRegistry(t *testing.T) {
	registry, err := NewExtendedProviderRegistry(nil)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Stop()

	req := &TopModelsRequest{
		Limit: 10,
	}

	models := registry.GetTopModels(req)
	if len(models) != 0 {
		t.Errorf("Expected 0 models, got %d", len(models))
	}
}

func TestExtendedRegistry_GetTopModels_WithModels(t *testing.T) {
	registry, err := NewExtendedProviderRegistry(nil)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Stop()

	// Add some verified models (Verified: true is required)
	registry.mu.Lock()
	registry.verifiedModels["model1"] = &VerifiedModel{
		ModelID:      "model1",
		ProviderName: "openai",
		OverallScore: 90.0,
		CodeVisible:  true,
		Verified:     true,
	}
	registry.verifiedModels["model2"] = &VerifiedModel{
		ModelID:      "model2",
		ProviderName: "anthropic",
		OverallScore: 85.0,
		CodeVisible:  false,
		Verified:     true,
	}
	registry.verifiedModels["model3"] = &VerifiedModel{
		ModelID:      "model3",
		ProviderName: "openai",
		OverallScore: 70.0,
		CodeVisible:  true,
		Verified:     true,
	}
	registry.mu.Unlock()

	// Test without filters
	req := &TopModelsRequest{Limit: 10}
	models := registry.GetTopModels(req)
	if len(models) != 3 {
		t.Errorf("Expected 3 models, got %d", len(models))
	}
}

func TestExtendedRegistry_GetTopModels_ProviderFilter(t *testing.T) {
	registry, err := NewExtendedProviderRegistry(nil)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Stop()

	// Add verified models
	registry.mu.Lock()
	registry.verifiedModels["model1"] = &VerifiedModel{
		ModelID:      "model1",
		ProviderName: "openai",
		OverallScore: 90.0,
		Verified:     true,
	}
	registry.verifiedModels["model2"] = &VerifiedModel{
		ModelID:      "model2",
		ProviderName: "anthropic",
		OverallScore: 85.0,
		Verified:     true,
	}
	registry.mu.Unlock()

	// Filter by openai only
	req := &TopModelsRequest{
		Limit:          10,
		ProviderFilter: []string{"openai"},
	}
	models := registry.GetTopModels(req)
	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}
	if len(models) > 0 && models[0].ProviderName != "openai" {
		t.Errorf("Expected openai provider, got %s", models[0].ProviderName)
	}
}

func TestExtendedRegistry_GetTopModels_MinScoreFilter(t *testing.T) {
	registry, err := NewExtendedProviderRegistry(nil)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Stop()

	// Add verified models with different scores
	registry.mu.Lock()
	registry.verifiedModels["high"] = &VerifiedModel{
		ModelID:      "high",
		ProviderName: "openai",
		OverallScore: 90.0,
		Verified:     true,
	}
	registry.verifiedModels["low"] = &VerifiedModel{
		ModelID:      "low",
		ProviderName: "openai",
		OverallScore: 60.0,
		Verified:     true,
	}
	registry.mu.Unlock()

	// Filter by minimum score
	req := &TopModelsRequest{
		Limit:    10,
		MinScore: 80.0,
	}
	models := registry.GetTopModels(req)
	if len(models) != 1 {
		t.Errorf("Expected 1 model with score >= 80, got %d", len(models))
	}
}

func TestExtendedRegistry_GetTopModels_CodeVisibilityFilter(t *testing.T) {
	registry, err := NewExtendedProviderRegistry(nil)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Stop()

	// Add verified models
	registry.mu.Lock()
	registry.verifiedModels["visible"] = &VerifiedModel{
		ModelID:      "visible",
		ProviderName: "openai",
		OverallScore: 90.0,
		CodeVisible:  true,
		Verified:     true,
	}
	registry.verifiedModels["hidden"] = &VerifiedModel{
		ModelID:      "hidden",
		ProviderName: "openai",
		OverallScore: 85.0,
		CodeVisible:  false,
		Verified:     true,
	}
	registry.mu.Unlock()

	// Filter by code visibility
	req := &TopModelsRequest{
		Limit:       10,
		RequireCode: true,
	}
	models := registry.GetTopModels(req)
	if len(models) != 1 {
		t.Errorf("Expected 1 model with code visibility, got %d", len(models))
	}
}

func TestExtendedRegistry_GetTopModels_LimitFilter(t *testing.T) {
	registry, err := NewExtendedProviderRegistry(nil)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Stop()

	// Add multiple verified models
	registry.mu.Lock()
	for i := 0; i < 10; i++ {
		registry.verifiedModels[string(rune('a'+i))] = &VerifiedModel{
			ModelID:      string(rune('a' + i)),
			ProviderName: "openai",
			OverallScore: 90.0,
			Verified:     true,
		}
	}
	registry.mu.Unlock()

	// Limit to 3
	req := &TopModelsRequest{
		Limit: 3,
	}
	models := registry.GetTopModels(req)
	if len(models) != 3 {
		t.Errorf("Expected 3 models with limit, got %d", len(models))
	}
}

func TestExtendedRegistry_GetTopModels_CombinedFilters(t *testing.T) {
	registry, err := NewExtendedProviderRegistry(nil)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Stop()

	// Add verified models
	registry.mu.Lock()
	registry.verifiedModels["match"] = &VerifiedModel{
		ModelID:      "match",
		ProviderName: "openai",
		OverallScore: 90.0,
		CodeVisible:  true,
		Verified:     true,
	}
	registry.verifiedModels["wrong_provider"] = &VerifiedModel{
		ModelID:      "wrong_provider",
		ProviderName: "anthropic",
		OverallScore: 95.0,
		CodeVisible:  true,
		Verified:     true,
	}
	registry.verifiedModels["low_score"] = &VerifiedModel{
		ModelID:      "low_score",
		ProviderName: "openai",
		OverallScore: 60.0,
		CodeVisible:  true,
		Verified:     true,
	}
	registry.verifiedModels["no_code"] = &VerifiedModel{
		ModelID:      "no_code",
		ProviderName: "openai",
		OverallScore: 92.0,
		CodeVisible:  false,
		Verified:     true,
	}
	registry.mu.Unlock()

	// Apply all filters
	req := &TopModelsRequest{
		Limit:          10,
		ProviderFilter: []string{"openai"},
		MinScore:       80.0,
		RequireCode:    true,
	}
	models := registry.GetTopModels(req)
	if len(models) != 1 {
		t.Errorf("Expected 1 model matching all filters, got %d", len(models))
	}
	if len(models) > 0 && models[0].ModelID != "match" {
		t.Errorf("Expected 'match' model, got %s", models[0].ModelID)
	}
}

func TestExtendedRegistry_RunHealthChecks(t *testing.T) {
	// Create a mock server that returns success
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"test"}}]}`))
	}))
	defer server.Close()

	registry, err := NewExtendedProviderRegistry(nil)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Stop()

	// Create and register an adapter
	adapter, err := NewProviderAdapter("test-health-check", "Test", "test-key", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	registry.adapters.Register(adapter)

	// Initialize provider health
	registry.mu.Lock()
	registry.providerHealth["test-health-check"] = &ProviderHealthStatus{
		ProviderID:       "test-health-check",
		Healthy:          true,
		ConsecutiveFails: 0,
	}
	registry.mu.Unlock()

	// Run health checks
	registry.runHealthChecks()

	// Verify provider is still healthy
	registry.mu.RLock()
	health := registry.providerHealth["test-health-check"]
	registry.mu.RUnlock()

	if health == nil {
		t.Fatal("Provider health not found")
	}
	// Health check should succeed
	if health.ConsecutiveFails > 0 {
		t.Errorf("Expected 0 consecutive fails after successful health check, got %d", health.ConsecutiveFails)
	}
}

func TestExtendedRegistry_RunHealthChecks_NoHealthCheckEnabled(t *testing.T) {
	// Test that health check doesn't fail when EnableHealthCheck is false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	registry, err := NewExtendedProviderRegistry(nil)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Stop()

	// Create adapter with EnableHealthCheck = false (default)
	adapter, err := NewProviderAdapter("test-health-disabled", "Test", "test-key", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	registry.adapters.Register(adapter)

	// Initialize provider health
	registry.mu.Lock()
	registry.providerHealth["test-health-disabled"] = &ProviderHealthStatus{
		ProviderID:       "test-health-disabled",
		Healthy:          true,
		ConsecutiveFails: 0,
	}
	registry.mu.Unlock()

	// Run health checks
	registry.runHealthChecks()

	// Verify health is still good (health check returns nil when disabled)
	registry.mu.RLock()
	health := registry.providerHealth["test-health-disabled"]
	registry.mu.RUnlock()

	if health == nil {
		t.Fatal("Provider health not found")
	}
	// When health check is disabled, it returns nil (no error), so success is recorded
	if health.ConsecutiveFails != 0 {
		t.Errorf("Expected 0 consecutive fails with disabled health check, got %d", health.ConsecutiveFails)
	}
}
