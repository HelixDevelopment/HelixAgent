package router

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/database"
	"github.com/superagent/superagent/internal/handlers"
	"github.com/superagent/superagent/internal/middleware"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/services"
	"github.com/superagent/superagent/pkg/metrics"
)

// SetupRouter creates and configures the main HTTP router.
func SetupRouter(cfg *config.Config) *gin.Engine {
	r := gin.New()

	// Middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Initialize database
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize user service
	userService := services.NewUserService(db, cfg.Server.JWTSecret, 24*time.Hour)

	// Initialize memory service
	memoryService := services.NewMemoryService(cfg)

	// Initialize services
	registryConfig := services.LoadRegistryConfigFromAppConfig(cfg)
	providerRegistry := services.NewProviderRegistry(registryConfig, memoryService)

	// Initialize completion handler
	completionHandler := handlers.NewCompletionHandler(providerRegistry.GetRequestService())

	// Initialize unified OpenAI-compatible handler
	unifiedHandler := handlers.NewUnifiedHandler(providerRegistry, cfg)

	// Initialize auth middleware
	authConfig := middleware.AuthConfig{
		SecretKey:   cfg.Server.JWTSecret,
		TokenExpiry: 24 * time.Hour,
		Issuer:      "superagent",
		SkipPaths:   []string{"/health", "/v1/health", "/metrics", "/v1/auth/login", "/v1/auth/register"},
		Required:    true,
	}
	auth := middleware.NewAuthMiddleware(authConfig, userService)

	// Health endpoints
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	r.GET("/v1/health", func(c *gin.Context) {
		// Enhanced health check with provider status
		health := providerRegistry.HealthCheck()
		healthyCount := 0
		for _, err := range health {
			if err == nil {
				healthyCount++
			}
		}

		c.JSON(200, gin.H{
			"status": "healthy",
			"providers": map[string]any{
				"total":     len(health),
				"healthy":   healthyCount,
				"unhealthy": len(health) - healthyCount,
			},
			"timestamp": time.Now().Unix(),
		})
	})

	// Metrics endpoint
	r.GET("/metrics", gin.WrapH(metrics.Handler()))

	// Authentication endpoints
	authGroup := r.Group("/v1/auth")
	{
		authGroup.POST("/register", auth.Register)
		authGroup.POST("/login", auth.Login)
		authGroup.POST("/refresh", auth.Refresh)
		authGroup.POST("/logout", auth.Logout)
		authGroup.GET("/me", func(c *gin.Context) {
			c.JSON(200, auth.GetAuthInfo(c))
		})
	}

	// Public API endpoints (no auth required)
	public := r.Group("/v1")
	{
		// Model listing
		public.GET("/models", completionHandler.Models)

		// Provider info (public endpoints)
		public.GET("/providers", func(c *gin.Context) {
			providers := providerRegistry.ListProviders()
			c.JSON(200, gin.H{
				"providers": providers,
				"count":     len(providers),
			})
		})
	}

	// Protected API endpoints (auth required)
	protected := r.Group("/v1", auth.Middleware([]string{"/health", "/v1/health", "/metrics"}))
	{
		// Register OpenAI-compatible routes for seamless integration
		unifiedHandler.RegisterOpenAIRoutes(protected, func(c *gin.Context) {
			c.Next() // Already authenticated by the group middleware
		})

		// Legacy endpoints (keep for backward compatibility)
		protected.POST("/completions", completionHandler.Complete)
		protected.POST("/completions/stream", completionHandler.CompleteStream)

		// Chat endpoints
		protected.POST("/chat/completions", completionHandler.Chat)
		protected.POST("/chat/completions/stream", completionHandler.ChatStream)

		// Ensemble endpoints
		protected.POST("/ensemble/completions", func(c *gin.Context) {
			var req handlers.CompletionRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// Force ensemble mode
			if req.EnsembleConfig == nil {
				req.EnsembleConfig = &models.EnsembleConfig{
					Strategy:            "confidence_weighted",
					MinProviders:        2,
					ConfidenceThreshold: 0.8,
					FallbackToBest:      true,
					Timeout:             30,
					PreferredProviders:  []string{},
				}
			}

			// Create a basic internal request
			internalReq := &models.LLMRequest{
				ID:        "ensemble-" + time.Now().Format("20060102150405"),
				SessionID: "ensemble-session",
				UserID:    "anonymous",
				Prompt:    req.Prompt,
				ModelParams: models.ModelParameters{
					Model:            req.Model,
					Temperature:      req.Temperature,
					MaxTokens:        req.MaxTokens,
					TopP:             req.TopP,
					StopSequences:    req.Stop,
					ProviderSpecific: map[string]any{},
				},
				EnsembleConfig: req.EnsembleConfig,
				MemoryEnhanced: req.MemoryEnhanced,
				Memory:         map[string]string{},
				Status:         "pending",
				CreatedAt:      time.Now(),
				RequestType:    "ensemble",
			}

			// Convert messages
			messages := make([]models.Message, 0, len(req.Messages))
			for _, msg := range req.Messages {
				messages = append(messages, models.Message{
					Role:      msg.Role,
					Content:   msg.Content,
					Name:      msg.Name,
					ToolCalls: msg.ToolCalls,
				})
			}
			internalReq.Messages = messages

			// Process with ensemble
			ensembleService := providerRegistry.GetEnsembleService()
			result, err := ensembleService.RunEnsemble(c.Request.Context(), internalReq)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Return ensemble result with metadata
			c.JSON(200, gin.H{
				"id":      result.Selected.ID,
				"object":  "ensemble.completion",
				"created": result.Selected.CreatedAt.Unix(),
				"model":   result.Selected.ProviderName,
				"choices": []gin.H{
					{
						"index": 0,
						"message": gin.H{
							"role":    "assistant",
							"content": result.Selected.Content,
						},
						"finish_reason": result.Selected.FinishReason,
					},
				},
				"usage": gin.H{
					"prompt_tokens":     result.Selected.TokensUsed / 2,
					"completion_tokens": result.Selected.TokensUsed / 2,
					"total_tokens":      result.Selected.TokensUsed,
				},
				"ensemble": gin.H{
					"voting_method":     result.VotingMethod,
					"responses_count":   len(result.Responses),
					"scores":            result.Scores,
					"metadata":          result.Metadata,
					"selected_provider": result.Selected.ProviderName,
					"selection_score":   result.Selected.SelectionScore,
				},
			})
		})

		// Provider management endpoints
		providerGroup := protected.Group("/providers")
		{
			providerGroup.GET("", func(c *gin.Context) {
				providers := providerRegistry.ListProviders()
				result := make([]gin.H, 0, len(providers))

				for _, name := range providers {
					provider, err := providerRegistry.GetProvider(name)
					if err == nil {
						capabilities := provider.GetCapabilities()
						result = append(result, gin.H{
							"name":                      name,
							"supported_models":          capabilities.SupportedModels,
							"supported_features":        capabilities.SupportedFeatures,
							"supports_streaming":        capabilities.SupportsStreaming,
							"supports_function_calling": capabilities.SupportsFunctionCalling,
							"supports_vision":           capabilities.SupportsVision,
							"metadata":                  capabilities.Metadata,
						})
					}
				}

				c.JSON(200, gin.H{
					"providers": result,
					"count":     len(result),
				})
			})

			providerGroup.GET("/:name/health", func(c *gin.Context) {
				name := c.Param("name")
				health := providerRegistry.HealthCheck()

				if err, exists := health[name]; exists {
					if err != nil {
						c.JSON(503, gin.H{
							"provider": name,
							"healthy":  false,
							"error":    err.Error(),
						})
					} else {
						c.JSON(200, gin.H{
							"provider": name,
							"healthy":  true,
						})
					}
				} else {
					c.JSON(404, gin.H{"error": "provider not found"})
				}
			})
		}

		// Admin endpoints
		admin := protected.Group("/admin")
		admin.Use(auth.RequireAdmin())
		{
			admin.GET("/health/all", func(c *gin.Context) {
				health := providerRegistry.HealthCheck()
				c.JSON(200, gin.H{
					"provider_health": health,
					"timestamp":       time.Now().Unix(),
				})
			})
		}
	}

	return r
}
