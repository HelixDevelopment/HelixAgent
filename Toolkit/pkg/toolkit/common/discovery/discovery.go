package discovery

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit"
	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit/common/http"
)

// CapabilityInferrer infers model capabilities
type CapabilityInferrer interface {
	InferCapabilities(modelID, modelType string) toolkit.ModelCapabilities
}

// CategoryInferrer infers model categories
type CategoryInferrer interface {
	InferCategory(modelID, modelType string) string
}

// ModelFormatter formats model information
type ModelFormatter interface {
	FormatModelName(modelID string) string
	GetModelDescription(modelID string) string
}

// BaseDiscovery provides common discovery functionality
type BaseDiscovery struct {
	providerName       string
	capabilityInferrer CapabilityInferrer
	categoryInferrer   CategoryInferrer
	modelFormatter     ModelFormatter
}

// NewBaseDiscovery creates a new base discovery instance
func NewBaseDiscovery(providerName string, capabilityInferrer CapabilityInferrer, categoryInferrer CategoryInferrer, modelFormatter ModelFormatter) *BaseDiscovery {
	return &BaseDiscovery{
		providerName:       providerName,
		capabilityInferrer: capabilityInferrer,
		categoryInferrer:   categoryInferrer,
		modelFormatter:     modelFormatter,
	}
}

// ConvertToModelInfo converts model data to ModelInfo
func (b *BaseDiscovery) ConvertToModelInfo(modelID, modelType string) toolkit.ModelInfo {
	return toolkit.ModelInfo{
		ID:      modelID,
		Object:  modelType,
		Created: time.Now().Unix(),
		OwnedBy: b.providerName,
	}
}

// DefaultCategoryInferrer provides default category inference
type DefaultCategoryInferrer struct{}

// InferCategory infers the category of a model
func (d *DefaultCategoryInferrer) InferCategory(modelID, modelType string) string {
	modelLower := strings.ToLower(modelID)
	typeLower := strings.ToLower(modelType)

	if strings.Contains(typeLower, "embedding") || strings.Contains(modelLower, "embedding") {
		return "embedding"
	}
	if strings.Contains(typeLower, "rerank") || strings.Contains(modelLower, "rerank") {
		return "rerank"
	}
	if strings.Contains(typeLower, "audio") || strings.Contains(modelLower, "tts") || strings.Contains(modelLower, "speech") {
		return "audio"
	}
	if strings.Contains(typeLower, "video") || strings.Contains(modelLower, "t2v") || strings.Contains(modelLower, "i2v") {
		return "video"
	}
	if strings.Contains(typeLower, "vision") || strings.Contains(modelLower, "vl") || strings.Contains(modelLower, "multimodal") {
		return "vision"
	}

	return "chat" // Default category
}

// DiscoveryService handles model discovery for providers
type DiscoveryService struct {
	client    *http.Client
	baseURL   string
	cache     map[string][]toolkit.ModelInfo
	cacheTime map[string]time.Time
	cacheTTL  time.Duration
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService(client *http.Client, baseURL string) *DiscoveryService {
	return &DiscoveryService{
		client:    client,
		baseURL:   baseURL,
		cache:     make(map[string][]toolkit.ModelInfo),
		cacheTime: make(map[string]time.Time),
		cacheTTL:  5 * time.Minute, // Cache for 5 minutes
	}
}

// DiscoverModels discovers available models from the provider
func (d *DiscoveryService) DiscoverModels(ctx context.Context, providerName string) ([]toolkit.ModelInfo, error) {
	// Check cache first
	if models, ok := d.checkCache(providerName); ok {
		return models, nil
	}

	// Fetch from API
	models, err := d.fetchModelsFromAPI(ctx, providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to discover models for %s: %w", providerName, err)
	}

	// Cache the results
	d.cache[providerName] = models
	d.cacheTime[providerName] = time.Now()

	return models, nil
}

// checkCache checks if we have cached results that are still valid
func (d *DiscoveryService) checkCache(providerName string) ([]toolkit.ModelInfo, bool) {
	cacheTime, exists := d.cacheTime[providerName]
	if !exists {
		return nil, false
	}

	if time.Since(cacheTime) > d.cacheTTL {
		// Cache expired
		delete(d.cache, providerName)
		delete(d.cacheTime, providerName)
		return nil, false
	}

	models, exists := d.cache[providerName]
	return models, exists
}

// fetchModelsFromAPI fetches models from the provider's API
func (d *DiscoveryService) fetchModelsFromAPI(ctx context.Context, providerName string) ([]toolkit.ModelInfo, error) {
	// This is a generic implementation - specific providers may override this
	endpoint := "/models"

	resp, err := d.client.Get(ctx, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Note: In a real implementation, you'd decode the response body here
	// For now, we'll return mock data based on provider
	return d.getMockModels(providerName), nil
}

// getMockModels returns mock model data for testing
// In production, this would be replaced with actual API calls
func (d *DiscoveryService) getMockModels(providerName string) []toolkit.ModelInfo {
	switch strings.ToLower(providerName) {
	case "openai":
		return []toolkit.ModelInfo{
			{ID: "gpt-4", Object: "model", Created: time.Now().Unix(), OwnedBy: "openai"},
			{ID: "gpt-3.5-turbo", Object: "model", Created: time.Now().Unix(), OwnedBy: "openai"},
		}
	case "anthropic":
		return []toolkit.ModelInfo{
			{ID: "claude-3-opus", Object: "model", Created: time.Now().Unix(), OwnedBy: "anthropic"},
			{ID: "claude-3-sonnet", Object: "model", Created: time.Now().Unix(), OwnedBy: "anthropic"},
		}
	case "siliconflow":
		return []toolkit.ModelInfo{
			{ID: "deepseek-chat", Object: "model", Created: time.Now().Unix(), OwnedBy: "siliconflow"},
			{ID: "deepseek-coder", Object: "model", Created: time.Now().Unix(), OwnedBy: "siliconflow"},
		}
	case "chutes":
		return []toolkit.ModelInfo{
			{ID: "llama-3-70b", Object: "model", Created: time.Now().Unix(), OwnedBy: "chutes"},
			{ID: "mixtral-8x7b", Object: "model", Created: time.Now().Unix(), OwnedBy: "chutes"},
		}
	default:
		return []toolkit.ModelInfo{
			{ID: "unknown-model", Object: "model", Created: time.Now().Unix(), OwnedBy: providerName},
		}
	}
}

// FilterModels filters models based on criteria
func (d *DiscoveryService) FilterModels(models []toolkit.ModelInfo, criteria map[string]interface{}) []toolkit.ModelInfo {
	var filtered []toolkit.ModelInfo

	for _, model := range models {
		if d.matchesCriteria(model, criteria) {
			filtered = append(filtered, model)
		}
	}

	return filtered
}

// matchesCriteria checks if a model matches the given criteria
func (d *DiscoveryService) matchesCriteria(model toolkit.ModelInfo, criteria map[string]interface{}) bool {
	for key, value := range criteria {
		switch key {
		case "owned_by":
			if ownedBy, ok := value.(string); ok && model.OwnedBy != ownedBy {
				return false
			}
		case "id_contains":
			if substring, ok := value.(string); ok && !strings.Contains(model.ID, substring) {
				return false
			}
		case "created_after":
			if timestamp, ok := value.(int64); ok && model.Created <= timestamp {
				return false
			}
		}
	}
	return true
}

// SortModels sorts models by various criteria
func (d *DiscoveryService) SortModels(models []toolkit.ModelInfo, sortBy string, ascending bool) []toolkit.ModelInfo {
	sorted := make([]toolkit.ModelInfo, len(models))
	copy(sorted, models)

	sort.Slice(sorted, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "id":
			less = sorted[i].ID < sorted[j].ID
		case "created":
			less = sorted[i].Created < sorted[j].Created
		case "owned_by":
			less = sorted[i].OwnedBy < sorted[j].OwnedBy
		default:
			less = sorted[i].ID < sorted[j].ID
		}

		if ascending {
			return less
		}
		return !less
	})

	return sorted
}

// ClearCache clears the discovery cache
func (d *DiscoveryService) ClearCache() {
	d.cache = make(map[string][]toolkit.ModelInfo)
	d.cacheTime = make(map[string]time.Time)
}

// SetCacheTTL sets the cache TTL
func (d *DiscoveryService) SetCacheTTL(ttl time.Duration) {
	d.cacheTTL = ttl
}
