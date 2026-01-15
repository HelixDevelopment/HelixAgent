// Package verifier provides integration with LLMsVerifier for model verification,
// scoring, and health monitoring capabilities.
package verifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"dev.helix.agent/internal/auth/oauth_credentials"
	"dev.helix.agent/internal/llm"
)

// StartupVerifier orchestrates the complete startup verification pipeline
// It is the single source of truth for all LLM provider information
type StartupVerifier struct {
	config          *StartupConfig
	verifierSvc     *VerificationService
	scoringSvc      *ScoringService
	healthSvc       *HealthService

	// Provider creation functions (dependency injection)
	providerFactory ProviderFactory

	// OAuth credential reader
	oauthReader     *oauth_credentials.OAuthCredentialReader

	// Results
	providers       map[string]*UnifiedProvider
	rankedProviders []*UnifiedProvider
	debateTeam      *DebateTeamResult

	// State
	initialized     bool
	lastVerifyAt    time.Time
	mu              sync.RWMutex

	log             *logrus.Logger
}

// ProviderFactory is a function that creates LLM providers
type ProviderFactory func(providerType string, config ProviderCreateConfig) (llm.LLMProvider, error)

// ProviderCreateConfig contains configuration for creating a provider
type ProviderCreateConfig struct {
	APIKey     string
	BaseURL    string
	Model      string
	OAuthToken string
	Anonymous  bool
}

// NewStartupVerifier creates a new startup verifier
func NewStartupVerifier(cfg *StartupConfig, log *logrus.Logger) *StartupVerifier {
	if cfg == nil {
		cfg = DefaultStartupConfig()
	}
	if log == nil {
		log = logrus.New()
	}

	// Initialize services
	verifierCfg := DefaultConfig()
	verifierSvc := NewVerificationService(verifierCfg)
	scoringSvc, err := NewScoringService(verifierCfg)
	if err != nil {
		log.WithError(err).Warn("Failed to create scoring service, using default")
		scoringSvc = nil
	}

	return &StartupVerifier{
		config:      cfg,
		verifierSvc: verifierSvc,
		scoringSvc:  scoringSvc,
		providers:   make(map[string]*UnifiedProvider),
		oauthReader: oauth_credentials.NewOAuthCredentialReader(),
		log:         log,
	}
}

// SetProviderFactory sets the provider factory function
func (sv *StartupVerifier) SetProviderFactory(factory ProviderFactory) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	sv.providerFactory = factory
}

// SetProviderFunc sets the function used by verification service to call LLMs
func (sv *StartupVerifier) SetProviderFunc(fn func(ctx context.Context, modelID, provider, prompt string) (string, error)) {
	sv.verifierSvc.SetProviderFunc(fn)
}

// VerifyAllProviders runs the complete startup verification pipeline
func (sv *StartupVerifier) VerifyAllProviders(ctx context.Context) (*StartupResult, error) {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	result := &StartupResult{
		StartedAt: time.Now(),
		Providers: make([]*UnifiedProvider, 0),
		Errors:    make([]StartupError, 0),
	}

	sv.log.Info("Starting LLMsVerifier unified startup verification pipeline")

	// Phase 1: Discover all providers
	sv.log.Info("Phase 1: Discovering providers from environment")
	discovered, err := sv.discoverProviders(ctx)
	if err != nil {
		result.Errors = append(result.Errors, StartupError{
			Phase:       "discovery",
			Error:       err.Error(),
			Recoverable: false,
		})
		return result, fmt.Errorf("provider discovery failed: %w", err)
	}
	result.TotalProviders = len(discovered)
	sv.log.WithField("count", len(discovered)).Info("Discovered providers")

	// Phase 2: Verify all providers in parallel
	sv.log.Info("Phase 2: Verifying providers")
	verified := sv.verifyProviders(ctx, discovered, result)
	sv.log.WithField("verified", len(verified)).Info("Verified providers")

	// Phase 3: Score all verified providers
	sv.log.Info("Phase 3: Scoring providers")
	scored := sv.scoreProviders(ctx, verified)
	sv.log.WithField("scored", len(scored)).Info("Scored providers")

	// Phase 4: Rank providers by score
	sv.log.Info("Phase 4: Ranking providers")
	sv.rankProviders(scored)

	// Phase 5: Select debate team
	sv.log.Info("Phase 5: Selecting AI Debate Team")
	debateTeam, err := sv.selectDebateTeam()
	if err != nil {
		result.Errors = append(result.Errors, StartupError{
			Phase:       "debate_team_selection",
			Error:       err.Error(),
			Recoverable: true,
		})
		sv.log.WithError(err).Warn("Failed to select complete debate team")
	}
	result.DebateTeam = debateTeam
	sv.debateTeam = debateTeam

	// Compile results
	result.CompletedAt = time.Now()
	result.DurationMs = result.CompletedAt.Sub(result.StartedAt).Milliseconds()
	result.RankedProviders = sv.rankedProviders
	result.Providers = make([]*UnifiedProvider, 0, len(sv.providers))
	for _, p := range sv.providers {
		result.Providers = append(result.Providers, p)
		if p.AuthType == AuthTypeAPIKey {
			result.APIKeyProviders++
		} else if p.AuthType == AuthTypeOAuth {
			result.OAuthProviders++
		} else if p.AuthType == AuthTypeFree || p.AuthType == AuthTypeAnonymous {
			result.FreeProviders++
		}
		if p.Verified {
			result.VerifiedCount++
		} else {
			result.FailedCount++
		}
	}

	sv.initialized = true
	sv.lastVerifyAt = time.Now()

	sv.log.WithFields(logrus.Fields{
		"total":     result.TotalProviders,
		"verified":  result.VerifiedCount,
		"failed":    result.FailedCount,
		"oauth":     result.OAuthProviders,
		"api_key":   result.APIKeyProviders,
		"free":      result.FreeProviders,
		"duration":  fmt.Sprintf("%dms", result.DurationMs),
	}).Info("Startup verification pipeline completed")

	return result, nil
}

// discoverProviders discovers all available providers
func (sv *StartupVerifier) discoverProviders(ctx context.Context) ([]*ProviderDiscoveryResult, error) {
	discovered := make([]*ProviderDiscoveryResult, 0)
	seen := make(map[string]bool)

	// 1. Discover OAuth providers first (highest priority)
	oauthProviders := sv.discoverOAuthProviders(ctx)
	for _, p := range oauthProviders {
		if !seen[p.Type] {
			discovered = append(discovered, p)
			seen[p.Type] = true
			sv.log.WithFields(logrus.Fields{
				"provider": p.Type,
				"source":   p.Source,
			}).Debug("Discovered OAuth provider")
		}
	}

	// 2. Discover API key providers from environment
	for providerType, info := range SupportedProviders {
		if seen[providerType] {
			continue // Already discovered via OAuth
		}

		for _, envVar := range info.EnvVars {
			apiKey := os.Getenv(envVar)
			if apiKey != "" && !isPlaceholder(apiKey) {
				discovered = append(discovered, &ProviderDiscoveryResult{
					ID:          providerType,
					Type:        providerType,
					AuthType:    info.AuthType,
					Discovered:  true,
					Source:      "env",
					Credentials: maskAPIKey(apiKey),
					BaseURL:     info.BaseURL,
					Models:      info.Models,
				})
				seen[providerType] = true
				sv.log.WithFields(logrus.Fields{
					"provider": providerType,
					"env_var":  envVar,
				}).Debug("Discovered API key provider")
				break
			}
		}
	}

	// 3. Discover free providers (always available)
	freeProviders := sv.discoverFreeProviders(ctx)
	for _, p := range freeProviders {
		if !seen[p.Type] {
			discovered = append(discovered, p)
			seen[p.Type] = true
			sv.log.WithFields(logrus.Fields{
				"provider": p.Type,
				"source":   p.Source,
			}).Debug("Discovered free provider")
		}
	}

	return discovered, nil
}

// discoverOAuthProviders discovers OAuth-based providers
func (sv *StartupVerifier) discoverOAuthProviders(ctx context.Context) []*ProviderDiscoveryResult {
	var providers []*ProviderDiscoveryResult

	// Check Claude OAuth
	if sv.isClaudeOAuthEnabled() {
		creds, err := sv.oauthReader.ReadClaudeCredentials()
		if err == nil && creds != nil && creds.ClaudeAiOauth != nil {
			providers = append(providers, &ProviderDiscoveryResult{
				ID:          "claude",
				Type:        "claude",
				AuthType:    AuthTypeOAuth,
				Discovered:  true,
				Source:      "oauth",
				Credentials: "OAuth (Claude Code)",
				BaseURL:     "https://api.anthropic.com/v1/messages",
				Models:      []string{"claude-opus-4-5-20251101", "claude-sonnet-4-5-20250929", "claude-haiku-4-5-20251001"},
			})
		}
	}

	// Check Qwen OAuth
	if sv.isQwenOAuthEnabled() {
		creds, err := sv.oauthReader.ReadQwenCredentials()
		if err == nil && creds != nil {
			providers = append(providers, &ProviderDiscoveryResult{
				ID:          "qwen",
				Type:        "qwen",
				AuthType:    AuthTypeOAuth,
				Discovered:  true,
				Source:      "oauth",
				Credentials: "OAuth (Qwen Code)",
				BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
				Models:      []string{"qwen-max", "qwen-plus", "qwen-turbo"},
			})
		}
	}

	return providers
}

// discoverFreeProviders discovers free/anonymous providers
func (sv *StartupVerifier) discoverFreeProviders(ctx context.Context) []*ProviderDiscoveryResult {
	var providers []*ProviderDiscoveryResult

	if !sv.config.EnableFreeProviders {
		return providers
	}

	// Zen is always available (anonymous mode)
	providers = append(providers, &ProviderDiscoveryResult{
		ID:          "zen",
		Type:        "zen",
		AuthType:    AuthTypeFree,
		Discovered:  true,
		Source:      "auto",
		Credentials: "Anonymous",
		BaseURL:     "https://opencode.ai/zen/v1/chat/completions",
		Models:      []string{"opencode/grok-code", "opencode/big-pickle", "opencode/glm-4.7-free"},
	})

	// Check if Ollama is running locally
	ollamaURL := os.Getenv("OLLAMA_BASE_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	// Health check Ollama - only add if it's actually running
	if ollamaURL != "" {
		ollamaModels := sv.checkOllamaHealth(ollamaURL)
		if len(ollamaModels) > 0 {
			providers = append(providers, &ProviderDiscoveryResult{
				ID:          "ollama",
				Type:        "ollama",
				AuthType:    AuthTypeLocal,
				Discovered:  true,
				Source:      "auto",
				Credentials: "Local",
				BaseURL:     ollamaURL,
				Models:      ollamaModels,
			})
			sv.log.WithFields(logrus.Fields{
				"url":    ollamaURL,
				"models": ollamaModels,
			}).Info("Ollama discovered with available models")
		} else {
			sv.log.WithField("url", ollamaURL).Debug("Ollama not running or no models available")
		}
	}

	return providers
}

// checkOllamaHealth checks if Ollama is running and returns available models
func (sv *StartupVerifier) checkOllamaHealth(baseURL string) []string {
	// Ollama API endpoint for listing models
	tagsURL := strings.TrimSuffix(baseURL, "/") + "/api/tags"

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(tagsURL)
	if err != nil {
		sv.log.WithError(err).Debug("Failed to connect to Ollama")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		sv.log.WithField("status", resp.StatusCode).Debug("Ollama returned non-OK status")
		return nil
	}

	// Parse the response to get model names
	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		sv.log.WithError(err).Debug("Failed to decode Ollama response")
		// If we can't parse but Ollama is running, return default model
		return []string{"llama3.2"}
	}

	if len(tagsResp.Models) == 0 {
		sv.log.Debug("Ollama running but no models available")
		return nil
	}

	// Extract model names
	models := make([]string, 0, len(tagsResp.Models))
	for _, m := range tagsResp.Models {
		if m.Name != "" {
			models = append(models, m.Name)
		}
	}

	return models
}

// verifyProviders verifies all discovered providers
func (sv *StartupVerifier) verifyProviders(ctx context.Context, discovered []*ProviderDiscoveryResult, result *StartupResult) []*UnifiedProvider {
	verified := make([]*UnifiedProvider, 0)

	if sv.config.ParallelVerification {
		// Parallel verification
		var wg sync.WaitGroup
		var mu sync.Mutex
		sem := make(chan struct{}, sv.config.MaxConcurrency)

		for _, disc := range discovered {
			wg.Add(1)
			go func(d *ProviderDiscoveryResult) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				provider, err := sv.verifyProvider(ctx, d)
				mu.Lock()
				defer mu.Unlock()

				if err != nil {
					result.Errors = append(result.Errors, StartupError{
						Provider:    d.Type,
						Phase:       "verification",
						Error:       err.Error(),
						Recoverable: true,
					})
					sv.log.WithError(err).WithField("provider", d.Type).Warn("Provider verification failed")
				}
				if provider != nil {
					verified = append(verified, provider)
					sv.providers[provider.ID] = provider
				}
			}(disc)
		}
		wg.Wait()
	} else {
		// Sequential verification
		for _, disc := range discovered {
			provider, err := sv.verifyProvider(ctx, disc)
			if err != nil {
				result.Errors = append(result.Errors, StartupError{
					Provider:    disc.Type,
					Phase:       "verification",
					Error:       err.Error(),
					Recoverable: true,
				})
				sv.log.WithError(err).WithField("provider", disc.Type).Warn("Provider verification failed")
			}
			if provider != nil {
				verified = append(verified, provider)
				sv.providers[provider.ID] = provider
			}
		}
	}

	return verified
}

// verifyProvider verifies a single provider
func (sv *StartupVerifier) verifyProvider(ctx context.Context, disc *ProviderDiscoveryResult) (*UnifiedProvider, error) {
	// Create timeout context
	verifyCtx, cancel := context.WithTimeout(ctx, sv.config.VerificationTimeout)
	defer cancel()

	provider := &UnifiedProvider{
		ID:          disc.ID,
		Name:        disc.Type,
		Type:        disc.Type,
		AuthType:    disc.AuthType,
		BaseURL:     disc.BaseURL,
		Status:      StatusUnknown,
		Models:      make([]UnifiedModel, 0),
	}

	// Set default model
	if len(disc.Models) > 0 {
		provider.DefaultModel = disc.Models[0]
	}

	// Special handling by auth type
	switch disc.AuthType {
	case AuthTypeOAuth:
		return sv.verifyOAuthProvider(verifyCtx, provider, disc)
	case AuthTypeFree, AuthTypeAnonymous:
		return sv.verifyFreeProvider(verifyCtx, provider, disc)
	case AuthTypeLocal:
		return sv.verifyLocalProvider(verifyCtx, provider, disc)
	default:
		return sv.verifyAPIKeyProvider(verifyCtx, provider, disc)
	}
}

// verifyOAuthProvider verifies an OAuth-based provider
func (sv *StartupVerifier) verifyOAuthProvider(ctx context.Context, provider *UnifiedProvider, disc *ProviderDiscoveryResult) (*UnifiedProvider, error) {
	sv.log.WithField("provider", provider.Type).Debug("Verifying OAuth provider")

	// Run verification through LLMsVerifier
	modelID := provider.DefaultModel
	if modelID == "" {
		modelID = disc.Models[0]
	}

	result, err := sv.verifierSvc.VerifyModel(ctx, modelID, provider.Type)
	if err != nil {
		// OAuth providers are trusted even if verification fails
		// (CLI tokens may have different API scopes)
		if sv.config.TrustOAuthOnFailure {
			sv.log.WithError(err).WithField("provider", provider.Type).Warn("OAuth verification failed, trusting CLI credentials")
			provider.Verified = true
			provider.Status = StatusHealthy
			provider.Score = 8.0 + sv.config.OAuthPriorityBoost // High default for OAuth
			provider.TestResults = map[string]bool{"oauth_trusted": true}
			return provider, nil
		}
		return nil, fmt.Errorf("OAuth provider verification failed: %w", err)
	}

	provider.Verified = result.Verified
	provider.VerifiedAt = time.Now()
	provider.Score = result.OverallScore/10.0 + sv.config.OAuthPriorityBoost // OAuth gets priority boost
	provider.ScoreSuffix = result.ScoreSuffix
	provider.CodeVisible = result.CodeVisible
	provider.TestResults = result.TestsMap

	if provider.Verified {
		provider.Status = StatusHealthy
	} else {
		provider.Status = StatusUnhealthy
		provider.ErrorMessage = result.ErrorMessage
	}

	// Build models list
	for _, modelID := range disc.Models {
		provider.Models = append(provider.Models, UnifiedModel{
			ID:       modelID,
			Name:     modelID,
			Provider: provider.Type,
			Verified: provider.Verified,
			Score:    provider.Score,
		})
	}

	return provider, nil
}

// verifyFreeProvider verifies a free/anonymous provider
func (sv *StartupVerifier) verifyFreeProvider(ctx context.Context, provider *UnifiedProvider, disc *ProviderDiscoveryResult) (*UnifiedProvider, error) {
	sv.log.WithField("provider", provider.Type).Debug("Verifying free provider")

	// Free providers get a reduced verification (health check only)
	// They're always available but with lower scores
	provider.Verified = true
	provider.Status = StatusHealthy
	provider.Score = sv.config.FreeProviderBaseScore
	provider.VerifiedAt = time.Now()
	provider.TestResults = map[string]bool{"free_provider": true}

	// Try full verification if possible
	if len(disc.Models) > 0 {
		result, err := sv.verifierSvc.VerifyModel(ctx, disc.Models[0], provider.Type)
		if err == nil && result.Verified {
			provider.Score = result.OverallScore / 10.0
			provider.CodeVisible = result.CodeVisible
			provider.TestResults = result.TestsMap
		}
	}

	// Build models list
	for _, modelID := range disc.Models {
		provider.Models = append(provider.Models, UnifiedModel{
			ID:       modelID,
			Name:     modelID,
			Provider: provider.Type,
			Verified: true,
			Score:    provider.Score,
		})
	}

	return provider, nil
}

// verifyLocalProvider verifies a local provider (Ollama)
func (sv *StartupVerifier) verifyLocalProvider(ctx context.Context, provider *UnifiedProvider, disc *ProviderDiscoveryResult) (*UnifiedProvider, error) {
	sv.log.WithField("provider", provider.Type).Debug("Verifying local provider")

	// Local providers have lowest priority
	provider.Verified = true
	provider.Status = StatusHealthy
	provider.Score = 5.0 // Low score, only fallback
	provider.VerifiedAt = time.Now()
	provider.TestResults = map[string]bool{"local_provider": true}
	provider.Priority = 20 // Lowest priority

	// Build models list
	for _, modelID := range disc.Models {
		provider.Models = append(provider.Models, UnifiedModel{
			ID:       modelID,
			Name:     modelID,
			Provider: provider.Type,
			Verified: true,
			Score:    provider.Score,
		})
	}

	return provider, nil
}

// verifyAPIKeyProvider verifies an API key-based provider
func (sv *StartupVerifier) verifyAPIKeyProvider(ctx context.Context, provider *UnifiedProvider, disc *ProviderDiscoveryResult) (*UnifiedProvider, error) {
	sv.log.WithField("provider", provider.Type).Debug("Verifying API key provider")

	modelID := provider.DefaultModel
	if modelID == "" && len(disc.Models) > 0 {
		modelID = disc.Models[0]
	}

	result, err := sv.verifierSvc.VerifyModel(ctx, modelID, provider.Type)
	if err != nil {
		provider.Status = StatusUnhealthy
		provider.ErrorMessage = err.Error()
		return provider, err
	}

	provider.Verified = result.Verified
	provider.VerifiedAt = time.Now()
	provider.Score = result.OverallScore / 10.0 // Convert 0-100 to 0-10
	provider.ScoreSuffix = result.ScoreSuffix
	provider.CodeVisible = result.CodeVisible
	provider.TestResults = result.TestsMap

	if provider.Verified {
		provider.Status = StatusHealthy
	} else {
		provider.Status = StatusUnhealthy
		provider.ErrorMessage = result.ErrorMessage
	}

	// Build models list
	for _, modelID := range disc.Models {
		provider.Models = append(provider.Models, UnifiedModel{
			ID:       modelID,
			Name:     modelID,
			Provider: provider.Type,
			Verified: provider.Verified,
			Score:    provider.Score,
		})
	}

	return provider, nil
}

// scoreProviders calculates scores for all providers using LLMsVerifier
func (sv *StartupVerifier) scoreProviders(ctx context.Context, providers []*UnifiedProvider) []*UnifiedProvider {
	for _, p := range providers {
		// Score is already set during verification
		// Apply additional scoring logic here if needed
		if p.Verified && p.Status == StatusHealthy {
			// Bonus for verified healthy providers
			p.Score += 0.5
		}

		// Cap score at 10.0
		if p.Score > 10.0 {
			p.Score = 10.0
		}
	}
	return providers
}

// rankProviders ranks providers by score (highest first)
func (sv *StartupVerifier) rankProviders(providers []*UnifiedProvider) {
	// Sort by: OAuth first (when enabled), then by score descending
	sort.Slice(providers, func(i, j int) bool {
		// OAuth providers come first
		iIsOAuth := providers[i].AuthType == AuthTypeOAuth
		jIsOAuth := providers[j].AuthType == AuthTypeOAuth

		if iIsOAuth && !jIsOAuth {
			return true
		}
		if !iIsOAuth && jIsOAuth {
			return false
		}

		// Then sort by score (descending)
		return providers[i].Score > providers[j].Score
	})

	sv.rankedProviders = providers
}

// selectDebateTeam selects the 15 LLMs for the AI debate team
func (sv *StartupVerifier) selectDebateTeam() (*DebateTeamResult, error) {
	if len(sv.rankedProviders) == 0 {
		return nil, fmt.Errorf("no verified providers available for debate team")
	}

	team := &DebateTeamResult{
		Positions:  make([]*DebatePosition, sv.config.PositionCount),
		TotalLLMs:  sv.config.DebateTeamSize,
		MinScore:   sv.config.MinScore,
		OAuthFirst: true,
		SelectedAt: time.Now(),
	}

	roles := []string{"analyst", "proposer", "critic", "synthesis", "mediator"}

	// Collect all LLMs from ranked providers
	var allLLMs []*DebateLLM
	for _, provider := range sv.rankedProviders {
		if !provider.Verified || provider.Score < sv.config.MinScore {
			continue
		}

		for _, model := range provider.Models {
			allLLMs = append(allLLMs, &DebateLLM{
				Provider:     provider.Type,
				ProviderType: provider.Type,
				ModelID:      model.ID,
				ModelName:    model.Name,
				AuthType:     provider.AuthType,
				Score:        provider.Score,
				Verified:     provider.Verified,
				IsOAuth:      provider.AuthType == AuthTypeOAuth,
			})
		}
	}

	if len(allLLMs) < sv.config.PositionCount {
		return nil, fmt.Errorf("insufficient verified LLMs: have %d, need at least %d", len(allLLMs), sv.config.PositionCount)
	}

	// Assign primaries (top 5)
	used := make(map[string]bool)
	llmIndex := 0

	for i := 0; i < sv.config.PositionCount; i++ {
		position := &DebatePosition{
			Position: i + 1,
			Role:     roles[i],
		}

		// Find primary (first unused LLM)
		for llmIndex < len(allLLMs) {
			llm := allLLMs[llmIndex]
			key := fmt.Sprintf("%s:%s", llm.Provider, llm.ModelID)
			if !used[key] {
				position.Primary = llm
				used[key] = true
				llmIndex++
				break
			}
			llmIndex++
		}

		team.Positions[i] = position
	}

	// Assign fallbacks
	for i := 0; i < sv.config.PositionCount; i++ {
		position := team.Positions[i]
		if position.Primary == nil {
			continue
		}

		primaryIsOAuth := position.Primary.IsOAuth
		fallbacksAssigned := 0

		// For OAuth primaries, prefer non-OAuth fallbacks
		for _, llm := range allLLMs {
			if fallbacksAssigned >= sv.config.FallbacksPerPosition {
				break
			}

			key := fmt.Sprintf("%s:%s", llm.Provider, llm.ModelID)
			if used[key] {
				continue
			}

			// Strategy: OAuth primary -> non-OAuth fallback
			if sv.config.OAuthPrimaryNonOAuthFallback && primaryIsOAuth && llm.IsOAuth {
				continue // Skip OAuth for OAuth primary's fallback
			}

			if fallbacksAssigned == 0 {
				position.Fallback1 = llm
			} else {
				position.Fallback2 = llm
			}
			used[key] = true
			fallbacksAssigned++
		}

		// If not enough non-OAuth fallbacks, use any
		if fallbacksAssigned < sv.config.FallbacksPerPosition {
			for _, llm := range allLLMs {
				if fallbacksAssigned >= sv.config.FallbacksPerPosition {
					break
				}

				key := fmt.Sprintf("%s:%s", llm.Provider, llm.ModelID)
				if used[key] {
					continue
				}

				if fallbacksAssigned == 0 || position.Fallback1 == nil {
					position.Fallback1 = llm
				} else {
					position.Fallback2 = llm
				}
				used[key] = true
				fallbacksAssigned++
			}
		}
	}

	// Count total LLMs selected
	totalSelected := 0
	for _, pos := range team.Positions {
		if pos.Primary != nil {
			totalSelected++
		}
		if pos.Fallback1 != nil {
			totalSelected++
		}
		if pos.Fallback2 != nil {
			totalSelected++
		}
	}
	team.TotalLLMs = totalSelected

	sv.log.WithField("total_llms", totalSelected).Info("AI Debate Team selected")
	return team, nil
}

// GetRankedProviders returns providers ranked by score
func (sv *StartupVerifier) GetRankedProviders() []*UnifiedProvider {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.rankedProviders
}

// GetDebateTeam returns the selected debate team
func (sv *StartupVerifier) GetDebateTeam() *DebateTeamResult {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.debateTeam
}

// GetProvider returns a specific provider by ID
func (sv *StartupVerifier) GetProvider(id string) (*UnifiedProvider, bool) {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	p, ok := sv.providers[id]
	return p, ok
}

// GetVerifiedProviders returns all verified providers
func (sv *StartupVerifier) GetVerifiedProviders() []*UnifiedProvider {
	sv.mu.RLock()
	defer sv.mu.RUnlock()

	var verified []*UnifiedProvider
	for _, p := range sv.providers {
		if p.Verified {
			verified = append(verified, p)
		}
	}
	return verified
}

// IsInitialized returns whether the startup verification has completed
func (sv *StartupVerifier) IsInitialized() bool {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.initialized
}

// Helper functions

func (sv *StartupVerifier) isClaudeOAuthEnabled() bool {
	env := os.Getenv("CLAUDE_CODE_USE_OAUTH_CREDENTIALS")
	return env == "true" || env == "1" || env == "yes"
}

func (sv *StartupVerifier) isQwenOAuthEnabled() bool {
	env := os.Getenv("QWEN_CODE_USE_OAUTH_CREDENTIALS")
	if env == "true" || env == "1" || env == "yes" {
		return true
	}
	// Auto-detect if credentials exist
	if env == "" || env == "auto" {
		_, err := sv.oauthReader.ReadQwenCredentials()
		return err == nil
	}
	return false
}

func isPlaceholder(value string) bool {
	placeholders := []string{
		"your-api-key", "sk-xxx", "xxx", "placeholder",
		"your_api_key", "api_key_here", "INSERT_KEY",
	}
	lower := strings.ToLower(value)
	for _, p := range placeholders {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
