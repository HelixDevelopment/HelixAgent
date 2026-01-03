package verifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// ModelDiscoveryService automatically discovers, verifies, and selects the best models
type ModelDiscoveryService struct {
	verificationService *VerificationService
	scoringService      *ScoringService
	healthService       *HealthService
	config              *DiscoveryConfig
	discoveredModels    map[string]*DiscoveredModel
	selectedModels      []*SelectedModel
	httpClient          *http.Client
	mu                  sync.RWMutex
	stopCh              chan struct{}
	wg                  sync.WaitGroup
}

// DiscoveryConfig represents discovery configuration
type DiscoveryConfig struct {
	Enabled               bool          `yaml:"enabled"`
	DiscoveryInterval     time.Duration `yaml:"discovery_interval"`
	MaxModelsForEnsemble  int           `yaml:"max_models_for_ensemble"`
	MinScore              float64       `yaml:"min_score"`
	RequireVerification   bool          `yaml:"require_verification"`
	RequireCodeVisibility bool          `yaml:"require_code_visibility"`
	RequireDiversity      bool          `yaml:"require_diversity"`
	ProviderPriority      []string      `yaml:"provider_priority"`
}

// DiscoveredModel represents a discovered model
type DiscoveredModel struct {
	ModelID       string    `json:"model_id"`
	ModelName     string    `json:"model_name"`
	Provider      string    `json:"provider"`
	ProviderID    string    `json:"provider_id"`
	DiscoveredAt  time.Time `json:"discovered_at"`
	Verified      bool      `json:"verified"`
	VerifiedAt    time.Time `json:"verified_at,omitempty"`
	CodeVisible   bool      `json:"code_visible"`
	OverallScore  float64   `json:"overall_score"`
	ScoreSuffix   string    `json:"score_suffix"`
	Capabilities  []string  `json:"capabilities,omitempty"`
	ContextWindow int       `json:"context_window,omitempty"`
}

// SelectedModel represents a model selected for AI debate ensemble
type SelectedModel struct {
	*DiscoveredModel
	Rank       int       `json:"rank"`
	VoteWeight float64   `json:"vote_weight"`
	Selected   bool      `json:"selected"`
	SelectedAt time.Time `json:"selected_at"`
}

// ProviderCredentials represents provider API credentials
type ProviderCredentials struct {
	ProviderName string `json:"provider_name"`
	APIKey       string `json:"api_key"`
	BaseURL      string `json:"base_url,omitempty"`
}

// NewModelDiscoveryService creates a new model discovery service
func NewModelDiscoveryService(
	vs *VerificationService,
	ss *ScoringService,
	hs *HealthService,
	cfg *DiscoveryConfig,
) *ModelDiscoveryService {
	if cfg == nil {
		cfg = DefaultDiscoveryConfig()
	}

	return &ModelDiscoveryService{
		verificationService: vs,
		scoringService:      ss,
		healthService:       hs,
		config:              cfg,
		discoveredModels:    make(map[string]*DiscoveredModel),
		selectedModels:      make([]*SelectedModel, 0),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopCh: make(chan struct{}),
	}
}

// Start starts the discovery service
func (s *ModelDiscoveryService) Start(credentials []ProviderCredentials) error {
	s.wg.Add(1)
	go s.discoveryLoop(credentials)
	return nil
}

// Stop stops the discovery service
func (s *ModelDiscoveryService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// discoveryLoop runs periodic discovery
func (s *ModelDiscoveryService) discoveryLoop(credentials []ProviderCredentials) {
	defer s.wg.Done()

	// Initial discovery
	s.runDiscoveryPipeline(credentials)

	ticker := time.NewTicker(s.config.DiscoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.runDiscoveryPipeline(credentials)
		}
	}
}

// runDiscoveryPipeline runs the complete discovery, verification, scoring, and selection pipeline
func (s *ModelDiscoveryService) runDiscoveryPipeline(credentials []ProviderCredentials) {
	ctx := context.Background()

	// Step 1: Discover all models from all providers
	discovered := s.discoverAllModels(ctx, credentials)

	// Step 2: Verify all discovered models
	verified := s.verifyDiscoveredModels(ctx, discovered)

	// Step 3: Score all verified models
	scored := s.scoreVerifiedModels(ctx, verified)

	// Step 4: Select top models for ensemble
	selected := s.selectTopModels(scored)

	// Update state
	s.mu.Lock()
	s.discoveredModels = make(map[string]*DiscoveredModel)
	for _, m := range scored {
		s.discoveredModels[m.ModelID] = m
	}
	s.selectedModels = selected
	s.mu.Unlock()
}

// discoverAllModels discovers models from all providers
func (s *ModelDiscoveryService) discoverAllModels(ctx context.Context, credentials []ProviderCredentials) []*DiscoveredModel {
	var wg sync.WaitGroup
	resultChan := make(chan []*DiscoveredModel, len(credentials))

	for _, cred := range credentials {
		wg.Add(1)
		go func(c ProviderCredentials) {
			defer wg.Done()
			models := s.discoverModelsFromProvider(ctx, c)
			resultChan <- models
		}(cred)
	}

	wg.Wait()
	close(resultChan)

	var allModels []*DiscoveredModel
	for models := range resultChan {
		allModels = append(allModels, models...)
	}

	return allModels
}

// discoverModelsFromProvider discovers models from a specific provider
func (s *ModelDiscoveryService) discoverModelsFromProvider(ctx context.Context, cred ProviderCredentials) []*DiscoveredModel {
	endpoint := s.getDiscoveryEndpoint(cred)
	if endpoint == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil
	}

	// Set authentication header based on provider
	switch cred.ProviderName {
	case "anthropic":
		req.Header.Set("x-api-key", cred.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "google":
		// Google uses query parameter
	default:
		req.Header.Set("Authorization", "Bearer "+cred.APIKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	return s.parseModelsResponse(cred, resp)
}

// getDiscoveryEndpoint returns the discovery endpoint for a provider
func (s *ModelDiscoveryService) getDiscoveryEndpoint(cred ProviderCredentials) string {
	baseURL := cred.BaseURL

	endpoints := map[string]string{
		"openai":     "https://api.openai.com/v1/models",
		"anthropic":  "https://api.anthropic.com/v1/models",
		"google":     "https://generativelanguage.googleapis.com/v1/models",
		"groq":       "https://api.groq.com/openai/v1/models",
		"together":   "https://api.together.xyz/v1/models",
		"mistral":    "https://api.mistral.ai/v1/models",
		"deepseek":   "https://api.deepseek.com/v1/models",
		"ollama":     "http://localhost:11434/api/tags",
		"openrouter": "https://openrouter.ai/api/v1/models",
		"xai":        "https://api.x.ai/v1/models",
		"cerebras":   "https://api.cerebras.ai/v1/models",
	}

	if baseURL != "" {
		return baseURL + "/models"
	}

	return endpoints[cred.ProviderName]
}

// parseModelsResponse parses the models response from a provider
func (s *ModelDiscoveryService) parseModelsResponse(cred ProviderCredentials, resp *http.Response) []*DiscoveredModel {
	var result struct {
		Data   []map[string]interface{} `json:"data"`
		Models []map[string]interface{} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	models := result.Data
	if len(models) == 0 {
		models = result.Models
	}

	var discovered []*DiscoveredModel
	for _, m := range models {
		modelID, _ := m["id"].(string)
		if modelID == "" {
			modelID, _ = m["name"].(string)
		}
		if modelID == "" {
			continue
		}

		// Filter out non-chat models
		if !s.isChatModel(modelID, cred.ProviderName) {
			continue
		}

		discovered = append(discovered, &DiscoveredModel{
			ModelID:      modelID,
			ModelName:    modelID,
			Provider:     cred.ProviderName,
			ProviderID:   fmt.Sprintf("%s-%s", cred.ProviderName, modelID),
			DiscoveredAt: time.Now(),
		})
	}

	return discovered
}

// isChatModel checks if a model is a chat/completion model
func (s *ModelDiscoveryService) isChatModel(modelID, provider string) bool {
	// Filter out embedding, moderation, and other non-chat models
	excludePatterns := []string{
		"embedding", "embed", "moderation", "tts", "whisper",
		"dall-e", "davinci", "babbage", "curie", "ada",
	}

	for _, pattern := range excludePatterns {
		if containsIgnoreCase(modelID, pattern) {
			return false
		}
	}

	return true
}

// verifyDiscoveredModels verifies all discovered models
func (s *ModelDiscoveryService) verifyDiscoveredModels(ctx context.Context, models []*DiscoveredModel) []*DiscoveredModel {
	var wg sync.WaitGroup
	var mu sync.Mutex
	verified := make([]*DiscoveredModel, 0)

	for _, model := range models {
		wg.Add(1)
		go func(m *DiscoveredModel) {
			defer wg.Done()

			result, err := s.verificationService.VerifyModel(ctx, m.ModelID, m.Provider)
			if err != nil {
				return
			}

			m.Verified = result.Verified
			m.VerifiedAt = time.Now()
			m.CodeVisible = result.CodeVisible

			if m.Verified && (!s.config.RequireCodeVisibility || m.CodeVisible) {
				mu.Lock()
				verified = append(verified, m)
				mu.Unlock()
			}
		}(model)
	}

	wg.Wait()
	return verified
}

// scoreVerifiedModels scores all verified models
func (s *ModelDiscoveryService) scoreVerifiedModels(ctx context.Context, models []*DiscoveredModel) []*DiscoveredModel {
	var wg sync.WaitGroup

	for _, model := range models {
		wg.Add(1)
		go func(m *DiscoveredModel) {
			defer wg.Done()

			result, err := s.scoringService.CalculateScore(ctx, m.ModelID)
			if err != nil {
				m.OverallScore = 5.0 // Default score
				m.ScoreSuffix = "(SC:5.0)"
				return
			}

			m.OverallScore = result.OverallScore
			m.ScoreSuffix = result.ScoreSuffix
		}(model)
	}

	wg.Wait()
	return models
}

// selectTopModels selects the top models for AI debate ensemble
func (s *ModelDiscoveryService) selectTopModels(models []*DiscoveredModel) []*SelectedModel {
	// Filter by minimum score
	filtered := make([]*DiscoveredModel, 0)
	for _, m := range models {
		if m.OverallScore >= s.config.MinScore {
			filtered = append(filtered, m)
		}
	}

	// Sort by score descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].OverallScore > filtered[j].OverallScore
	})

	// Select top N with diversity consideration
	selected := make([]*SelectedModel, 0, s.config.MaxModelsForEnsemble)
	providersUsed := make(map[string]bool)

	for _, m := range filtered {
		if len(selected) >= s.config.MaxModelsForEnsemble {
			break
		}

		// Apply diversity if required
		if s.config.RequireDiversity && providersUsed[m.Provider] {
			continue
		}

		selected = append(selected, &SelectedModel{
			DiscoveredModel: m,
			Rank:            len(selected) + 1,
			VoteWeight:      s.calculateVoteWeight(m),
			Selected:        true,
			SelectedAt:      time.Now(),
		})

		providersUsed[m.Provider] = true
	}

	return selected
}

// calculateVoteWeight calculates the vote weight for AI debate
func (s *ModelDiscoveryService) calculateVoteWeight(m *DiscoveredModel) float64 {
	// Normalize score to 0-1 range
	weight := m.OverallScore / 10.0

	// Apply code visibility bonus
	if m.CodeVisible {
		weight *= 1.1
	}

	// Cap at 1.0
	if weight > 1.0 {
		weight = 1.0
	}

	return weight
}

// GetSelectedModels returns the currently selected models for AI debate
func (s *ModelDiscoveryService) GetSelectedModels() []*SelectedModel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.selectedModels
}

// GetDiscoveredModels returns all discovered models
func (s *ModelDiscoveryService) GetDiscoveredModels() []*DiscoveredModel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	models := make([]*DiscoveredModel, 0, len(s.discoveredModels))
	for _, m := range s.discoveredModels {
		models = append(models, m)
	}
	return models
}

// GetModelForDebate returns a model by ID if it's selected for debate
func (s *ModelDiscoveryService) GetModelForDebate(modelID string) (*SelectedModel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, m := range s.selectedModels {
		if m.ModelID == modelID {
			return m, true
		}
	}
	return nil, false
}

// GetDiscoveryStats returns discovery statistics
func (s *ModelDiscoveryService) GetDiscoveryStats() *DiscoveryStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &DiscoveryStats{
		TotalDiscovered: len(s.discoveredModels),
		TotalSelected:   len(s.selectedModels),
		ByProvider:      make(map[string]int),
	}

	for _, m := range s.discoveredModels {
		stats.ByProvider[m.Provider]++
		if m.Verified {
			stats.TotalVerified++
		}
		if m.CodeVisible {
			stats.CodeVisibleCount++
		}
	}

	if len(s.selectedModels) > 0 {
		var totalScore float64
		for _, m := range s.selectedModels {
			totalScore += m.OverallScore
		}
		stats.AverageScore = totalScore / float64(len(s.selectedModels))
	}

	return stats
}

// DiscoveryStats represents discovery statistics
type DiscoveryStats struct {
	TotalDiscovered  int            `json:"total_discovered"`
	TotalVerified    int            `json:"total_verified"`
	TotalSelected    int            `json:"total_selected"`
	CodeVisibleCount int            `json:"code_visible_count"`
	AverageScore     float64        `json:"average_score"`
	ByProvider       map[string]int `json:"by_provider"`
}

// DefaultDiscoveryConfig returns default discovery configuration
func DefaultDiscoveryConfig() *DiscoveryConfig {
	return &DiscoveryConfig{
		Enabled:               true,
		DiscoveryInterval:     24 * time.Hour,
		MaxModelsForEnsemble:  5,
		MinScore:              7.0,
		RequireVerification:   true,
		RequireCodeVisibility: true,
		RequireDiversity:      true,
		ProviderPriority: []string{
			"openai", "anthropic", "google", "groq", "together",
			"mistral", "deepseek", "ollama", "openrouter",
		},
	}
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
