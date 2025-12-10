package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/database"
	llm "github.com/superagent/superagent/internal/llm"
	"github.com/superagent/superagent/internal/middleware"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/plugins"
	"github.com/superagent/superagent/internal/services"
)

func main() {
	cfg := config.Load()

	// Initialize database
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize services
	memoryService := services.NewMemoryService(cfg)
	registryConfig := services.LoadRegistryConfigFromAppConfig(cfg)
	providerRegistry := services.NewProviderRegistry(registryConfig, memoryService)

	// Initialize plugin system with hot-reload if enabled
	var hotReloadManager *plugins.HotReloadManager
	if cfg.Plugins.HotReload {
		pluginRegistry := plugins.NewRegistry()
		hr, err := plugins.NewHotReloadManager(cfg, pluginRegistry)
		if err != nil {
			log.Printf("Warning: Failed to initialize hot-reload manager: %v", err)
		} else {
			hotReloadManager = hr
			if err := hotReloadManager.Start(context.Background()); err != nil {
				log.Printf("Warning: Failed to start hot-reload manager: %v", err)
			} else {
				defer hotReloadManager.Stop()
				log.Printf("Hot-reload manager started")
			}
		}
	}

	// Initialize user service
	userService := services.NewUserService(db, cfg.Server.JWTSecret, 24*time.Hour)

	// Create auth middleware
	authConfig := middleware.AuthConfig{
		SecretKey:   cfg.Server.JWTSecret,
		TokenExpiry: 24 * time.Hour,
		Issuer:      "superagent",
		SkipPaths:   []string{"/health", "/v1/health", "/v1/auth/login"},
		Required:    true,
	}
	auth := middleware.NewAuthMiddleware(authConfig, userService)

	r := gin.Default()

	// Health endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Authentication endpoints
	r.POST("/v1/auth/login", auth.Login)

	// Attach auth middleware to protected routes
	protected := r.Group("", auth.Middleware([]string{"/health", "/v1/health", "/v1/auth/login"}))

	// Providers endpoints (stub MVP) - protected for mutation
	protected.GET("/v1/providers", func(c *gin.Context) {
		c.JSON(http.StatusOK, []gin.H{
			{"id": "prov-default", "name": "DefaultProvider", "type": "builtin", "enabled": true},
		})
	})
	protected.POST("/v1/providers", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"success": true, "message": "provider added (stub)"})
	})

	// Completions endpoint (ensemble-based)
	r.POST("/v1/completions", func(c *gin.Context) {
		var req models.LLMRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		responses, selected, err := llm.RunEnsemble(&req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"responses": responses, "selected": selected})
	})

	// gRPC-like bridge endpoint (paralleled path to show gRPC integration readiness)
	r.POST("/grpc/llm/complete", func(c *gin.Context) {
		var req models.LLMRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		responses, selected, err := llm.RunEnsemble(&req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"responses": responses, "selected": selected})
	})

	// Plugin system endpoints
	pluginGroup := r.Group("/v1/plugins")
	{
		pluginGroup.GET("", func(c *gin.Context) {
			plugins := providerRegistry.ListProviders()
			pluginInfo := make([]gin.H, 0, len(plugins))
			for _, name := range plugins {
				if config, err := providerRegistry.GetProviderConfig(name); err == nil {
					pluginInfo = append(pluginInfo, gin.H{
						"name":    name,
						"enabled": config.Enabled,
						"type":    config.Type,
						"models":  config.Models,
					})
				}
			}
			c.JSON(http.StatusOK, gin.H{
				"plugins": pluginInfo,
				"count":   len(plugins),
			})
		})

		pluginGroup.GET("/:name", func(c *gin.Context) {
			name := c.Param("name")
			if config, err := providerRegistry.GetProviderConfig(name); err == nil {
				c.JSON(http.StatusOK, config)
			} else {
				c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
			}
		})

		pluginGroup.GET("/:name/health", func(c *gin.Context) {
			name := c.Param("name")
			if provider, err := providerRegistry.GetProvider(name); err == nil {
				if err := provider.HealthCheck(); err != nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{
						"healthy": false,
						"error":   err.Error(),
					})
				} else {
					c.JSON(http.StatusOK, gin.H{
						"healthy": true,
					})
				}
			} else {
				c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
			}
		})

		// Hot-reload management endpoints
		pluginGroup.GET("/hot-reload/status", func(c *gin.Context) {
			if hotReloadManager != nil {
				c.JSON(http.StatusOK, hotReloadManager.GetStats())
			} else {
				c.JSON(http.StatusOK, gin.H{
					"enabled": false,
					"message": "Hot-reload not configured",
				})
			}
		})

		pluginGroup.POST("/hot-reload/enable", func(c *gin.Context) {
			if hotReloadManager != nil {
				hotReloadManager.Enable()
				c.JSON(http.StatusOK, gin.H{"status": "enabled"})
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Hot-reload not configured"})
			}
		})

		pluginGroup.POST("/hot-reload/disable", func(c *gin.Context) {
			if hotReloadManager != nil {
				hotReloadManager.Disable()
				c.JSON(http.StatusOK, gin.H{"status": "disabled"})
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Hot-reload not configured"})
			}
		})

		pluginGroup.POST("/hot-reload/reload/:name", func(c *gin.Context) {
			name := c.Param("name")
			if hotReloadManager != nil {
				if err := hotReloadManager.ReloadPlugin(name); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				} else {
					c.JSON(http.StatusOK, gin.H{"status": "reloaded", "plugin": name})
				}
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Hot-reload not configured"})
			}
		})
	}

	// Configuration management endpoints
	configGroup := r.Group("/v1/config")
	{
		configGroup.GET("", func(c *gin.Context) {
			// Return safe configuration (no secrets)
			c.JSON(http.StatusOK, gin.H{
				"server": gin.H{
					"port": cfg.Server.Port,
					"mode": cfg.Server.Mode,
					"cors": cfg.Server.EnableCORS,
				},
				"llm": gin.H{
					"default_timeout": cfg.LLM.DefaultTimeout,
					"max_retries":     cfg.LLM.MaxRetries,
				},
				"monitoring": gin.H{
					"enabled": cfg.Monitoring.Enabled,
				},
			})
		})

		configGroup.GET("/providers", func(c *gin.Context) {
			healthResults := providerRegistry.HealthCheck()
			results := make([]gin.H, 0)

			for name, err := range healthResults {
				status := "healthy"
				errorMsg := ""
				if err != nil {
					status = "unhealthy"
					errorMsg = err.Error()
				}

				results = append(results, gin.H{
					"name":   name,
					"status": status,
					"error":  errorMsg,
				})
			}

			c.JSON(http.StatusOK, gin.H{
				"providers": results,
				"total":     len(results),
				"healthy":   len(healthResults) - countErrors(healthResults),
			})
		})
	}

	// Advanced monitoring endpoints
	r.GET("/v1/health", func(c *gin.Context) {
		healthResults := providerRegistry.HealthCheck()
		healthyCount := len(healthResults) - countErrors(healthResults)

		status := "healthy"
		if healthyCount == 0 {
			status = "unhealthy"
		} else if healthyCount < len(healthResults) {
			status = "degraded"
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    status,
			"timestamp": time.Now().UTC(),
			"providers": gin.H{
				"total":   len(healthResults),
				"healthy": healthyCount,
			},
			"components": []string{"database", "providers", "plugins"},
		})
	})

	// Metrics endpoint
	r.GET("/metrics", func(c *gin.Context) {
		// Return Prometheus metrics format
		c.Header("Content-Type", "text/plain")
		c.String(http.StatusOK, "# HELP superagent_requests_total Total number of requests\n# TYPE superagent_requests_total counter\nsuperagent_requests_total 42\n")
	})

	// Models endpoint for OpenAI compatibility
	r.GET("/v1/models", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data": []gin.H{
				{"id": "gpt-3.5-turbo", "object": "model", "owned_by": "superagent"},
				{"id": "claude-3-sonnet", "object": "model", "owned_by": "anthropic"},
				{"id": "llama2", "object": "model", "owned_by": "ollama"},
			},
		})
	})

	// Ensemble completion endpoint
	r.POST("/v1/ensemble/completions", func(c *gin.Context) {
		var req models.LLMRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Use ensemble service
		ensemble := providerRegistry.GetEnsembleService()
		if ensemble == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Ensemble service not available"})
			return
		}

		response, err := ensemble.RunEnsemble(context.Background(), &req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, response)
	})

	// Run server
	log.Printf("Starting SuperAgent server on port %s", cfg.Server.Port)
	r.Run(":" + cfg.Server.Port)
}

func countErrors(healthResults map[string]error) int {
	count := 0
	for _, err := range healthResults {
		if err != nil {
			count++
		}
	}
	return count
}
