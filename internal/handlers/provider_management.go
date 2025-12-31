package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/services"
)

// ProviderManagementHandler handles provider CRUD operations
type ProviderManagementHandler struct {
	providerRegistry *services.ProviderRegistry
	log              *logrus.Logger
}

// NewProviderManagementHandler creates a new provider management handler
func NewProviderManagementHandler(providerRegistry *services.ProviderRegistry, log *logrus.Logger) *ProviderManagementHandler {
	return &ProviderManagementHandler{
		providerRegistry: providerRegistry,
		log:              log,
	}
}

// AddProviderRequest represents a request to add a new provider
type AddProviderRequest struct {
	Name    string                 `json:"name" binding:"required"`
	Type    string                 `json:"type" binding:"required"`
	APIKey  string                 `json:"api_key" binding:"required"`
	BaseURL string                 `json:"base_url" binding:"required"`
	Model   string                 `json:"model" binding:"required"`
	Weight  float64                `json:"weight"`
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}

// UpdateProviderRequest represents a request to update a provider
type UpdateProviderRequest struct {
	Name    string                 `json:"name"`
	APIKey  string                 `json:"api_key"`
	BaseURL string                 `json:"base_url"`
	Model   string                 `json:"model"`
	Weight  float64                `json:"weight"`
	Enabled *bool                  `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}

// ProviderResponse represents a provider response
type ProviderResponse struct {
	Success  bool                `json:"success"`
	Message  string              `json:"message"`
	Provider *models.LLMProvider `json:"provider,omitempty"`
}

// AddProvider handles POST /v1/providers
func (h *ProviderManagementHandler) AddProvider(c *gin.Context) {
	var req AddProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.WithError(err).Error("Failed to bind add provider request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate provider type
	validTypes := []string{"deepseek", "claude", "gemini", "qwen", "zai", "ollama", "openrouter"}
	isValidType := false
	for _, t := range validTypes {
		if req.Type == t {
			isValidType = true
			break
		}
	}
	if !isValidType {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "invalid provider type",
			"valid_types": validTypes,
		})
		return
	}

	// Check if provider already exists
	existingProviders := h.providerRegistry.ListProviders()
	for _, name := range existingProviders {
		if name == req.Name {
			c.JSON(http.StatusConflict, gin.H{
				"error": "provider already exists",
				"name":  req.Name,
			})
			return
		}
	}

	// Set default weight if not provided
	if req.Weight == 0 {
		req.Weight = 1.0
	}

	// Create provider configuration
	providerConfig := services.ProviderConfig{
		Name:    req.Name,
		Type:    req.Type,
		APIKey:  req.APIKey,
		BaseURL: req.BaseURL,
		Weight:  req.Weight,
		Enabled: req.Enabled,
		Timeout: 30 * time.Second,
		Models: []services.ModelConfig{{
			ID:      req.Model,
			Name:    req.Model,
			Enabled: true,
			Weight:  1.0,
		}},
	}

	// Register the provider
	if err := h.providerRegistry.RegisterProviderFromConfig(providerConfig); err != nil {
		h.log.WithError(err).Error("Failed to register provider")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	provider := &models.LLMProvider{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Type:         req.Type,
		BaseURL:      req.BaseURL,
		Model:        req.Model,
		Weight:       req.Weight,
		Enabled:      req.Enabled,
		Config:       req.Config,
		HealthStatus: "unknown",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	h.log.WithFields(logrus.Fields{
		"name": req.Name,
		"type": req.Type,
	}).Info("Provider added successfully")

	c.JSON(http.StatusCreated, ProviderResponse{
		Success:  true,
		Message:  "Provider added successfully",
		Provider: provider,
	})
}

// GetProvider handles GET /v1/providers/:id
func (h *ProviderManagementHandler) GetProvider(c *gin.Context) {
	providerID := c.Param("id")

	provider, err := h.providerRegistry.GetProvider(providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":       "provider not found",
			"provider_id": providerID,
		})
		return
	}

	capabilities := provider.GetCapabilities()

	c.JSON(http.StatusOK, gin.H{
		"id":                        providerID,
		"name":                      providerID,
		"supported_models":          capabilities.SupportedModels,
		"supported_features":        capabilities.SupportedFeatures,
		"supports_streaming":        capabilities.SupportsStreaming,
		"supports_function_calling": capabilities.SupportsFunctionCalling,
		"supports_vision":           capabilities.SupportsVision,
		"limits":                    capabilities.Limits,
		"metadata":                  capabilities.Metadata,
	})
}

// UpdateProvider handles PUT /v1/providers/:id
func (h *ProviderManagementHandler) UpdateProvider(c *gin.Context) {
	providerID := c.Param("id")

	var req UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.WithError(err).Error("Failed to bind update provider request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if provider exists
	_, err := h.providerRegistry.GetProvider(providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":       "provider not found",
			"provider_id": providerID,
		})
		return
	}

	// Build update config
	updateConfig := services.ProviderConfig{
		Name:    providerID,
		APIKey:  req.APIKey,
		BaseURL: req.BaseURL,
		Weight:  req.Weight,
	}

	if req.Model != "" {
		updateConfig.Models = []services.ModelConfig{{
			ID:      req.Model,
			Name:    req.Model,
			Enabled: true,
			Weight:  1.0,
		}}
	}

	if req.Enabled != nil {
		updateConfig.Enabled = *req.Enabled
	}

	// Update the provider
	if err := h.providerRegistry.UpdateProvider(providerID, updateConfig); err != nil {
		h.log.WithError(err).Error("Failed to update provider")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.log.WithField("provider_id", providerID).Info("Provider updated successfully")

	c.JSON(http.StatusOK, ProviderResponse{
		Success: true,
		Message: "Provider updated successfully",
	})
}

// DeleteProvider handles DELETE /v1/providers/:id
func (h *ProviderManagementHandler) DeleteProvider(c *gin.Context) {
	providerID := c.Param("id")

	// Check if provider exists
	_, err := h.providerRegistry.GetProvider(providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":       "provider not found",
			"provider_id": providerID,
		})
		return
	}

	// Check force parameter
	force := c.Query("force") == "true"

	// Remove the provider
	if err := h.providerRegistry.RemoveProvider(providerID, force); err != nil {
		h.log.WithError(err).Error("Failed to remove provider")
		c.JSON(http.StatusConflict, gin.H{
			"error":       err.Error(),
			"provider_id": providerID,
			"hint":        "Use ?force=true to force removal",
		})
		return
	}

	h.log.WithField("provider_id", providerID).Info("Provider removed successfully")

	c.JSON(http.StatusOK, ProviderResponse{
		Success: true,
		Message: "Provider removed successfully",
	})
}
