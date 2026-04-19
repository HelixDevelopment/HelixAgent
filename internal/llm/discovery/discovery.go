// Package discovery provides 3-tier dynamic model discovery for LLM providers.
//
// Discovery tiers:
//   - Tier 1: Query provider's own API (e.g., /v1/models) for available models
//   - Tier 2: Query models.dev API for provider's model catalog
//   - Tier 3: Fall back to hardcoded known models
//
// Results are cached with configurable TTL to avoid excessive API calls.
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"dev.helix.agent/internal/modelsdev"
	"github.com/sirupsen/logrus"
)

// ProviderConfig configures model discovery for a specific LLM provider.
type ProviderConfig struct {
	// ProviderName is the provider identifier (e.g., "openai", "groq", "chutes")
	ProviderName string

	// ModelsEndpoint is the provider's API endpoint for listing models (Tier 1).
	// Empty string skips Tier 1 discovery.
	ModelsEndpoint string

	// ModelsDevID is the provider's ID on models.dev (Tier 2).
	// Empty string skips Tier 2 discovery.
	ModelsDevID string

	// APIKey for authenticating with the provider's models endpoint.
	APIKey string

	// AuthHeader is the HTTP header name for authentication (default: "Authorization").
	AuthHeader string

	// AuthPrefix is prepended to APIKey in the auth header (default: "Bearer ").
	AuthPrefix string

	// ExtraHeaders are additional HTTP headers for the models request.
	ExtraHeaders map[string]string

	// ModelFilter filters model IDs from API/models.dev responses.
	// Return true to include the model. If nil, the default chat model filter is used.
	ModelFilter func(modelID string) bool

	// ResponseParser is a custom parser for non-OpenAI-compatible model list responses.
	// If nil, the standard OpenAI /v1/models response format is assumed.
	ResponseParser func(resp *http.Response) ([]string, error)

	// FallbackModels is the hardcoded list of known models (Tier 3).
	FallbackModels []string

	// CacheTTL controls how long discovered models are cached (default: 1 hour).
	CacheTTL time.Duration
}

// Discoverer handles 3-tier model discovery for LLM providers.
//
// Concurrent-safe by construction (CONST-029): the (models, discoveredAt,
// tier) triple lives in an atomic.Pointer[discoveryState]. cacheModels
// constructs a fresh state and atomically Stores it; readers Load and
// inspect the snapshot. Tiered discovery is read-mostly so the lock-free
// path is the common one.
type Discoverer struct {
	config     ProviderConfig
	state      atomic.Pointer[discoveryState]
	log        *logrus.Logger
	httpClient *http.Client
}

// discoveryState is the joint cache state.
type discoveryState struct {
	models       []string
	discoveredAt time.Time
	tier         int
}

// NewDiscoverer creates a new model discoverer with the given configuration.
func NewDiscoverer(config ProviderConfig) *Discoverer {
	if config.AuthHeader == "" {
		config.AuthHeader = "Authorization"
		// Only default to "Bearer " when using standard Authorization header
		if config.AuthPrefix == "" {
			config.AuthPrefix = "Bearer "
		}
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 1 * time.Hour
	}

	return &Discoverer{
		config: config,
		log:    logrus.StandardLogger(),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// DiscoverModels returns available models using 3-tier discovery:
//   - Tier 1: Provider API (e.g., /v1/models)
//   - Tier 2: models.dev API
//   - Tier 3: Hardcoded fallback
//
// Results are cached for CacheTTL duration.
func (d *Discoverer) DiscoverModels() []string {
	if cur := d.state.Load(); cur != nil && len(cur.models) > 0 && time.Since(cur.discoveredAt) < d.config.CacheTTL {
		result := make([]string, len(cur.models))
		copy(result, cur.models)
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Tier 1: Provider API
	if d.config.ModelsEndpoint != "" && d.config.APIKey != "" {
		models := d.discoverFromProviderAPI(ctx)
		if len(models) > 0 {
			d.cacheModels(models, 1)
			d.log.WithFields(logrus.Fields{
				"provider": d.config.ProviderName,
				"count":    len(models),
				"tier":     1,
				"source":   "provider_api",
			}).Info("Discovered models from provider API")
			return models
		}
	}

	// Tier 2: models.dev API
	if d.config.ModelsDevID != "" {
		models := d.discoverFromModelsDev(ctx)
		if len(models) > 0 {
			d.cacheModels(models, 2)
			d.log.WithFields(logrus.Fields{
				"provider": d.config.ProviderName,
				"count":    len(models),
				"tier":     2,
				"source":   "models_dev",
			}).Info("Discovered models from models.dev")
			return models
		}
	}

	// Tier 3: Hardcoded fallback
	if len(d.config.FallbackModels) > 0 {
		d.cacheModels(d.config.FallbackModels, 3)
		d.log.WithFields(logrus.Fields{
			"provider": d.config.ProviderName,
			"count":    len(d.config.FallbackModels),
			"tier":     3,
			"source":   "fallback",
		}).Info("Using fallback model list")
	}

	return d.config.FallbackModels
}

// GetCachedModels returns the currently cached models without triggering discovery.
// Falls back to FallbackModels if cache is empty.
func (d *Discoverer) GetCachedModels() []string {
	cur := d.state.Load()
	if cur == nil || len(cur.models) == 0 {
		return d.config.FallbackModels
	}
	result := make([]string, len(cur.models))
	copy(result, cur.models)
	return result
}

// GetDiscoveryTier returns which tier provided the current models (0 = not discovered).
func (d *Discoverer) GetDiscoveryTier() int {
	cur := d.state.Load()
	if cur == nil {
		return 0
	}
	return cur.tier
}

// InvalidateCache forces the next DiscoverModels() call to re-fetch models.
func (d *Discoverer) InvalidateCache() {
	d.state.Store(nil)
}

func (d *Discoverer) cacheModels(models []string, tier int) {
	d.state.Store(&discoveryState{
		models:       models,
		discoveredAt: time.Now(),
		tier:         tier,
	})
}

// openAIModelsResponse is the standard OpenAI-compatible /v1/models response.
type openAIModelsResponse struct {
	Data []openAIModel `json:"data"`
}

type openAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// discoverFromProviderAPI queries the provider's models endpoint (Tier 1).
func (d *Discoverer) discoverFromProviderAPI(ctx context.Context) []string {
	req, err := http.NewRequestWithContext(ctx, "GET", d.config.ModelsEndpoint, nil)
	if err != nil {
		d.log.WithError(err).WithField("provider", d.config.ProviderName).
			Debug("Failed to create models API request")
		return nil
	}

	// Set authentication header
	if d.config.APIKey != "" {
		req.Header.Set(d.config.AuthHeader, d.config.AuthPrefix+d.config.APIKey)
	}

	// Set extra headers
	for k, v := range d.config.ExtraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.log.WithError(err).WithField("provider", d.config.ProviderName).
			Debug("Provider models API request failed")
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		d.log.WithFields(logrus.Fields{
			"provider": d.config.ProviderName,
			"status":   resp.StatusCode,
		}).Debug("Provider models API returned non-200")
		return nil
	}

	// Use custom parser if provided
	if d.config.ResponseParser != nil {
		models, err := d.config.ResponseParser(resp)
		if err != nil {
			d.log.WithError(err).WithField("provider", d.config.ProviderName).
				Debug("Custom response parser failed")
			return nil
		}
		return d.filterModels(models)
	}

	// Default: parse OpenAI-compatible response
	var apiResp openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		d.log.WithError(err).WithField("provider", d.config.ProviderName).
			Debug("Failed to parse models response")
		return nil
	}

	var models []string
	for _, m := range apiResp.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}

	return d.filterModels(models)
}

// discoverFromModelsDev queries models.dev API (Tier 2).
func (d *Discoverer) discoverFromModelsDev(ctx context.Context) []string {
	client := modelsdev.NewClient(nil)

	opts := &modelsdev.ListModelsOptions{
		Limit: 100,
	}

	resp, err := client.ListProviderModels(ctx, d.config.ModelsDevID, opts)
	if err != nil {
		d.log.WithError(err).WithField("provider", d.config.ProviderName).
			Debug("models.dev API request failed")
		return nil
	}

	if resp == nil || len(resp.Models) == 0 {
		return nil
	}

	var models []string
	for _, m := range resp.Models {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}

	return d.filterModels(models)
}

// filterModels applies the configured model filter or the default chat model filter.
func (d *Discoverer) filterModels(models []string) []string {
	if len(models) == 0 {
		return nil
	}

	var filtered []string
	for _, m := range models {
		if d.config.ModelFilter != nil {
			if d.config.ModelFilter(m) {
				filtered = append(filtered, m)
			}
		} else if IsChatModel(m) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// IsChatModel returns true if the model ID appears to be a chat/completion model
// (filters out embedding, moderation, TTS, whisper, and legacy models).
func IsChatModel(modelID string) bool {
	excludePatterns := []string{
		"embedding", "embed-", "moderation", "tts", "whisper",
		"dall-e", "davinci", "babbage", "curie", "ada-",
		"text-search", "text-similarity", "code-search",
		"text-embedding", "text-davinci", "canary",
	}

	lower := strings.ToLower(modelID)
	for _, pattern := range excludePatterns {
		if strings.Contains(lower, pattern) {
			return false
		}
	}

	return true
}

// ParseGeminiModelsResponse parses Google's Gemini models API response format.
// Google returns: { "models": [{ "name": "models/gemini-2.0-flash", ... }] }
func ParseGeminiModelsResponse(resp *http.Response) ([]string, error) {
	var result struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini models response: %w", err)
	}

	var models []string
	for _, m := range result.Models {
		name := m.Name
		// Strip "models/" prefix
		name = strings.TrimPrefix(name, "models/")
		if name == "" {
			continue
		}

		// Only include models that support generateContent
		supportsChat := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" || method == "streamGenerateContent" {
				supportsChat = true
				break
			}
		}
		if supportsChat {
			models = append(models, name)
		}
	}

	return models, nil
}

// ParseOllamaModelsResponse parses Ollama's /api/tags response format.
// Ollama returns: { "models": [{ "name": "llama2:latest", ... }] }
func ParseOllamaModelsResponse(resp *http.Response) ([]string, error) {
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse Ollama models response: %w", err)
	}

	var models []string
	for _, m := range result.Models {
		if m.Name != "" {
			models = append(models, m.Name)
		}
	}

	return models, nil
}

// ParseCohereModelsResponse parses Cohere's /v1/models response format.
// Cohere returns: { "models": [{ "name": "command-r-plus", ... }] }
func ParseCohereModelsResponse(resp *http.Response) ([]string, error) {
	var result struct {
		Models []struct {
			Name      string   `json:"name"`
			Endpoints []string `json:"endpoints"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse Cohere models response: %w", err)
	}

	var models []string
	for _, m := range result.Models {
		if m.Name == "" {
			continue
		}
		// Only include models that support chat
		supportsChat := false
		for _, ep := range m.Endpoints {
			if ep == "chat" || ep == "generate" {
				supportsChat = true
				break
			}
		}
		if supportsChat || len(m.Endpoints) == 0 {
			models = append(models, m.Name)
		}
	}

	return models, nil
}

// ParseReplicateModelsResponse parses Replicate's /v1/models response format.
func ParseReplicateModelsResponse(resp *http.Response) ([]string, error) {
	var result struct {
		Results []struct {
			URL   string `json:"url"`
			Owner string `json:"owner"`
			Name  string `json:"name"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse Replicate models response: %w", err)
	}

	var models []string
	for _, m := range result.Results {
		if m.Owner != "" && m.Name != "" {
			models = append(models, m.Owner+"/"+m.Name)
		}
	}

	return models, nil
}

// ParseZAIModelsResponse parses ZAI/Zhipu's models API response format.
// ZAI returns: { "data": [{ "id": "glm-4.7", ... }] } (OpenAI-compatible).
func ParseZAIModelsResponse(resp *http.Response) ([]string, error) {
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse ZAI models response: %w", err)
	}

	var models []string
	for _, m := range result.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}

	return models, nil
}
