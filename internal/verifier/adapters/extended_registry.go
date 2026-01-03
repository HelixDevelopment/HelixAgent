package adapters

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"llm-verifier/database"
	"llm-verifier/verification"
)

// ExtendedProviderRegistry extends SuperAgent's provider registry with LLMsVerifier capabilities
type ExtendedProviderRegistry struct {
	adapters         *ProviderAdapterRegistry
	verifier         *verification.Verifier
	db               *database.Database
	verifiedModels   map[string]*VerifiedModel
	providerHealth   map[string]*ProviderHealthStatus
	mu               sync.RWMutex
	config           *ExtendedRegistryConfig
	eventChan        chan *ProviderEvent
	stopCh           chan struct{}
	wg               sync.WaitGroup
}

// ExtendedRegistryConfig represents extended registry configuration
type ExtendedRegistryConfig struct {
	AutoVerifyNewProviders  bool          `yaml:"auto_verify_new_providers"`
	VerificationInterval    time.Duration `yaml:"verification_interval"`
	HealthCheckInterval     time.Duration `yaml:"health_check_interval"`
	ScoreUpdateInterval     time.Duration `yaml:"score_update_interval"`
	FailoverEnabled         bool          `yaml:"failover_enabled"`
	MaxFailoverAttempts     int           `yaml:"max_failover_attempts"`
	CodeVisibilityTest      bool          `yaml:"code_visibility_test"`
	MandatoryVerification   bool          `yaml:"mandatory_verification"`
}

// VerifiedModel represents a verified model with its verification status
type VerifiedModel struct {
	ModelID           string    `json:"model_id"`
	ModelName         string    `json:"model_name"`
	ProviderID        string    `json:"provider_id"`
	ProviderName      string    `json:"provider_name"`
	Verified          bool      `json:"verified"`
	VerificationScore float64   `json:"verification_score"`
	LastVerifiedAt    time.Time `json:"last_verified_at"`
	VerificationTests map[string]bool `json:"verification_tests"`
	CodeVisible       bool      `json:"code_visible"`
	OverallScore      float64   `json:"overall_score"`
	ScoreSuffix       string    `json:"score_suffix"`
}

// ProviderHealthStatus represents provider health status
type ProviderHealthStatus struct {
	ProviderID       string    `json:"provider_id"`
	ProviderName     string    `json:"provider_name"`
	Healthy          bool      `json:"healthy"`
	AvgResponseMs    int64     `json:"avg_response_ms"`
	SuccessRate      float64   `json:"success_rate"`
	LastCheckAt      time.Time `json:"last_check_at"`
	ConsecutiveFails int       `json:"consecutive_fails"`
	CircuitOpen      bool      `json:"circuit_open"`
}

// ProviderEvent represents a provider event
type ProviderEvent struct {
	Type       string    `json:"type"`
	ProviderID string    `json:"provider_id"`
	ModelID    string    `json:"model_id,omitempty"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
}

// NewExtendedProviderRegistry creates a new extended provider registry
func NewExtendedProviderRegistry(db *database.Database, cfg *ExtendedRegistryConfig) (*ExtendedProviderRegistry, error) {
	if cfg == nil {
		cfg = DefaultExtendedRegistryConfig()
	}

	verifier := verification.NewVerifier(db, nil)

	return &ExtendedProviderRegistry{
		adapters:       NewProviderAdapterRegistry(),
		verifier:       verifier,
		db:             db,
		verifiedModels: make(map[string]*VerifiedModel),
		providerHealth: make(map[string]*ProviderHealthStatus),
		config:         cfg,
		eventChan:      make(chan *ProviderEvent, 100),
		stopCh:         make(chan struct{}),
	}, nil
}

// Start starts the registry background services
func (r *ExtendedProviderRegistry) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Start health check loop
	r.wg.Add(1)
	go r.healthCheckLoop()

	// Start verification loop if auto-verify is enabled
	if r.config.AutoVerifyNewProviders {
		r.wg.Add(1)
		go r.verificationLoop()
	}

	return nil
}

// Stop stops the registry background services
func (r *ExtendedProviderRegistry) Stop() {
	close(r.stopCh)
	r.wg.Wait()
	close(r.eventChan)
}

// RegisterProvider registers a new provider with verification
func (r *ExtendedProviderRegistry) RegisterProvider(ctx context.Context, providerID, providerName, apiKey, baseURL string, models []string) error {
	// Create adapter
	adapter, err := NewProviderAdapter(providerID, providerName, apiKey, baseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create provider adapter: %w", err)
	}

	// Register adapter
	r.adapters.Register(adapter)

	// Initialize health status
	r.mu.Lock()
	r.providerHealth[providerID] = &ProviderHealthStatus{
		ProviderID:   providerID,
		ProviderName: providerName,
		Healthy:      true,
		LastCheckAt:  time.Now(),
	}
	r.mu.Unlock()

	// Register models
	for _, modelID := range models {
		r.mu.Lock()
		r.verifiedModels[modelID] = &VerifiedModel{
			ModelID:      modelID,
			ModelName:    modelID,
			ProviderID:   providerID,
			ProviderName: providerName,
			Verified:     false,
		}
		r.mu.Unlock()
	}

	// Auto-verify if enabled
	if r.config.AutoVerifyNewProviders {
		go func() {
			for _, modelID := range models {
				if err := r.VerifyModel(ctx, modelID, providerID); err != nil {
					r.emitEvent("verification_failed", providerID, modelID, err.Error())
				}
			}
		}()
	}

	r.emitEvent("provider_registered", providerID, "", fmt.Sprintf("Provider %s registered with %d models", providerName, len(models)))
	return nil
}

// UnregisterProvider removes a provider from the registry
func (r *ExtendedProviderRegistry) UnregisterProvider(providerID string) {
	r.adapters.Remove(providerID)

	r.mu.Lock()
	delete(r.providerHealth, providerID)
	// Remove associated models
	for modelID, model := range r.verifiedModels {
		if model.ProviderID == providerID {
			delete(r.verifiedModels, modelID)
		}
	}
	r.mu.Unlock()

	r.emitEvent("provider_unregistered", providerID, "", "Provider unregistered")
}

// VerifyModel verifies a specific model using LLMsVerifier
func (r *ExtendedProviderRegistry) VerifyModel(ctx context.Context, modelID, providerID string) error {
	adapter, ok := r.adapters.Get(providerID)
	if !ok {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	// Create provider function for verifier
	providerFunc := func(ctx context.Context, model, provider, prompt string) (string, error) {
		return adapter.Complete(ctx, model, prompt, nil)
	}

	// Run verification
	result, err := r.verifier.VerifyModel(ctx, modelID, adapter.GetProviderName(), providerFunc)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Update verified model
	r.mu.Lock()
	if model, ok := r.verifiedModels[modelID]; ok {
		model.Verified = result.Verified
		model.VerificationScore = result.Score
		model.LastVerifiedAt = time.Now()
		model.VerificationTests = result.Tests
		model.CodeVisible = result.CodeVisible
		model.OverallScore = result.OverallScore
		model.ScoreSuffix = fmt.Sprintf("(SC:%.1f)", result.OverallScore)
	}
	r.mu.Unlock()

	eventType := "verification_passed"
	if !result.Verified {
		eventType = "verification_failed"
	}
	r.emitEvent(eventType, providerID, modelID, fmt.Sprintf("Score: %.2f, Code visible: %v", result.Score, result.CodeVisible))

	return nil
}

// GetVerifiedModel returns a verified model by ID
func (r *ExtendedProviderRegistry) GetVerifiedModel(modelID string) (*VerifiedModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, ok := r.verifiedModels[modelID]
	if !ok {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	return model, nil
}

// GetVerifiedModels returns all verified models
func (r *ExtendedProviderRegistry) GetVerifiedModels() []*VerifiedModel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]*VerifiedModel, 0, len(r.verifiedModels))
	for _, model := range r.verifiedModels {
		if model.Verified {
			models = append(models, model)
		}
	}

	// Sort by score descending
	sort.Slice(models, func(i, j int) bool {
		return models[i].OverallScore > models[j].OverallScore
	})

	return models
}

// GetModelWithScoreSuffix returns model name with score suffix
func (r *ExtendedProviderRegistry) GetModelWithScoreSuffix(modelID string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, ok := r.verifiedModels[modelID]
	if !ok {
		return modelID, fmt.Errorf("model not found: %s", modelID)
	}

	if model.ScoreSuffix != "" {
		return fmt.Sprintf("%s %s", model.ModelName, model.ScoreSuffix), nil
	}

	return model.ModelName, nil
}

// GetHealthyProviders returns all healthy providers
func (r *ExtendedProviderRegistry) GetHealthyProviders() []*ProviderHealthStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	healthy := make([]*ProviderHealthStatus, 0)
	for _, health := range r.providerHealth {
		if health.Healthy && !health.CircuitOpen {
			healthy = append(healthy, health)
		}
	}

	// Sort by success rate descending
	sort.Slice(healthy, func(i, j int) bool {
		return healthy[i].SuccessRate > healthy[j].SuccessRate
	})

	return healthy
}

// GetProviderHealth returns health status for a specific provider
func (r *ExtendedProviderRegistry) GetProviderHealth(providerID string) (*ProviderHealthStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	health, ok := r.providerHealth[providerID]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	return health, nil
}

// Complete sends a completion request with verification and failover support
func (r *ExtendedProviderRegistry) Complete(ctx context.Context, modelID, prompt string, options map[string]interface{}) (string, error) {
	r.mu.RLock()
	model, ok := r.verifiedModels[modelID]
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("model not found: %s", modelID)
	}

	// Check mandatory verification
	if r.config.MandatoryVerification && !model.Verified {
		return "", fmt.Errorf("model not verified: %s", modelID)
	}

	adapter, ok := r.adapters.Get(model.ProviderID)
	if !ok {
		return "", fmt.Errorf("provider not found: %s", model.ProviderID)
	}

	// Check health
	r.mu.RLock()
	health := r.providerHealth[model.ProviderID]
	r.mu.RUnlock()

	if health != nil && health.CircuitOpen {
		// Try failover if enabled
		if r.config.FailoverEnabled {
			return r.completeWithFailover(ctx, modelID, prompt, options)
		}
		return "", fmt.Errorf("provider circuit open: %s", model.ProviderID)
	}

	// Execute request
	response, err := adapter.Complete(ctx, modelID, prompt, options)
	if err != nil {
		r.recordProviderFailure(model.ProviderID)
		if r.config.FailoverEnabled {
			return r.completeWithFailover(ctx, modelID, prompt, options)
		}
		return "", err
	}

	r.recordProviderSuccess(model.ProviderID, adapter.GetMetrics().AvgLatencyMs)
	return response, nil
}

// completeWithFailover attempts to complete with failover providers
func (r *ExtendedProviderRegistry) completeWithFailover(ctx context.Context, modelID, prompt string, options map[string]interface{}) (string, error) {
	healthyProviders := r.GetHealthyProviders()

	for i := 0; i < r.config.MaxFailoverAttempts && i < len(healthyProviders); i++ {
		health := healthyProviders[i]
		adapter, ok := r.adapters.Get(health.ProviderID)
		if !ok {
			continue
		}

		response, err := adapter.Complete(ctx, modelID, prompt, options)
		if err == nil {
			r.emitEvent("failover_success", health.ProviderID, modelID, "Failover successful")
			return response, nil
		}
	}

	return "", fmt.Errorf("all failover attempts failed")
}

// recordProviderSuccess records a successful provider request
func (r *ExtendedProviderRegistry) recordProviderSuccess(providerID string, latencyMs float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if health, ok := r.providerHealth[providerID]; ok {
		health.Healthy = true
		health.AvgResponseMs = int64(latencyMs)
		health.ConsecutiveFails = 0
		health.CircuitOpen = false
		health.LastCheckAt = time.Now()

		// Update success rate
		totalRequests := float64(health.ConsecutiveFails) + 1
		health.SuccessRate = 1.0 / totalRequests * 100
	}
}

// recordProviderFailure records a failed provider request
func (r *ExtendedProviderRegistry) recordProviderFailure(providerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if health, ok := r.providerHealth[providerID]; ok {
		health.ConsecutiveFails++
		health.LastCheckAt = time.Now()

		// Open circuit breaker after threshold
		if health.ConsecutiveFails >= 5 {
			health.CircuitOpen = true
			health.Healthy = false
			r.emitEvent("circuit_opened", providerID, "", "Circuit breaker opened after 5 consecutive failures")
		}
	}
}

// healthCheckLoop runs periodic health checks
func (r *ExtendedProviderRegistry) healthCheckLoop() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.runHealthChecks()
		}
	}
}

// runHealthChecks performs health checks on all providers
func (r *ExtendedProviderRegistry) runHealthChecks() {
	ctx := context.Background()
	adapters := r.adapters.GetAll()

	for _, adapter := range adapters {
		err := adapter.HealthCheck(ctx)
		if err != nil {
			r.recordProviderFailure(adapter.GetProviderID())
		} else {
			r.recordProviderSuccess(adapter.GetProviderID(), 0)
		}
	}
}

// verificationLoop runs periodic verification
func (r *ExtendedProviderRegistry) verificationLoop() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.config.VerificationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.runVerifications()
		}
	}
}

// runVerifications re-verifies all models
func (r *ExtendedProviderRegistry) runVerifications() {
	ctx := context.Background()

	r.mu.RLock()
	models := make([]*VerifiedModel, 0, len(r.verifiedModels))
	for _, model := range r.verifiedModels {
		models = append(models, model)
	}
	r.mu.RUnlock()

	for _, model := range models {
		if err := r.VerifyModel(ctx, model.ModelID, model.ProviderID); err != nil {
			r.emitEvent("re_verification_failed", model.ProviderID, model.ModelID, err.Error())
		}
	}
}

// emitEvent emits a provider event
func (r *ExtendedProviderRegistry) emitEvent(eventType, providerID, modelID, message string) {
	select {
	case r.eventChan <- &ProviderEvent{
		Type:       eventType,
		ProviderID: providerID,
		ModelID:    modelID,
		Message:    message,
		Timestamp:  time.Now(),
	}:
	default:
		// Event channel full, drop event
	}
}

// Events returns the event channel
func (r *ExtendedProviderRegistry) Events() <-chan *ProviderEvent {
	return r.eventChan
}

// GetAdapter returns a provider adapter by ID
func (r *ExtendedProviderRegistry) GetAdapter(providerID string) (*ProviderAdapter, bool) {
	return r.adapters.Get(providerID)
}

// DefaultExtendedRegistryConfig returns default configuration
func DefaultExtendedRegistryConfig() *ExtendedRegistryConfig {
	return &ExtendedRegistryConfig{
		AutoVerifyNewProviders: true,
		VerificationInterval:   24 * time.Hour,
		HealthCheckInterval:    30 * time.Second,
		ScoreUpdateInterval:    12 * time.Hour,
		FailoverEnabled:        true,
		MaxFailoverAttempts:    3,
		CodeVisibilityTest:     true,
		MandatoryVerification:  false,
	}
}

// TopModelsRequest represents a request for top models
type TopModelsRequest struct {
	Limit          int      `json:"limit"`
	ProviderFilter []string `json:"provider_filter,omitempty"`
	MinScore       float64  `json:"min_score,omitempty"`
	RequireCode    bool     `json:"require_code_visibility"`
}

// GetTopModels returns top scoring verified models
func (r *ExtendedProviderRegistry) GetTopModels(req *TopModelsRequest) []*VerifiedModel {
	models := r.GetVerifiedModels()

	// Apply filters
	filtered := make([]*VerifiedModel, 0)
	for _, model := range models {
		// Provider filter
		if len(req.ProviderFilter) > 0 {
			found := false
			for _, p := range req.ProviderFilter {
				if model.ProviderName == p {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Min score filter
		if req.MinScore > 0 && model.OverallScore < req.MinScore {
			continue
		}

		// Code visibility filter
		if req.RequireCode && !model.CodeVisible {
			continue
		}

		filtered = append(filtered, model)
	}

	// Apply limit
	if req.Limit > 0 && len(filtered) > req.Limit {
		filtered = filtered[:req.Limit]
	}

	return filtered
}
