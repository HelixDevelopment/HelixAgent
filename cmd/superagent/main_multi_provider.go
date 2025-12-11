package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/superagent/superagent/internal/cache"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/database"
	"github.com/superagent/superagent/internal/handlers"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/services"
)

func main() {
	// Load multi-provider configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/multi-provider.yaml"
	}

	multiConfig, err := config.LoadMultiProviderConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load multi-provider config: %v", err)
	}

	// Initialize database
	dbConfig := &config.Config{
		Database: config.DatabaseConfig{
			Host:     multiConfig.Database.Host,
			Port:     multiConfig.Database.Port,
			User:     multiConfig.Database.User,
			Password: multiConfig.Database.Password,
			Name:     multiConfig.Database.Name,
			SSLMode:  "disable",
		},
		Redis: multiConfig.Redis,
		LLM: config.LLMConfig{
			DefaultTimeout: multiConfig.Server.Timeout,
			MaxRetries:     3,
		},
		Server: config.ServerConfig{
			Host: multiConfig.Server.Host,
			Port: fmt.Sprintf("%d", multiConfig.Server.Port),
		},
	}

	db, err := database.NewPostgresDB(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize Redis
	rdb := cache.NewRedisClient(&config.Config{
		Redis: multiConfig.Redis,
	})
	defer rdb.Close()

	// Convert multi-provider config to registry config
	registryConfig := convertToRegistryConfig(multiConfig)

	// Initialize memory service
	memoryService := services.NewMemoryService(dbConfig)

	// Initialize provider registry
	providerRegistry := services.NewProviderRegistry(registryConfig, memoryService)

	// Initialize handlers
	unifiedHandler := handlers.NewUnifiedHandler(providerRegistry, dbConfig)

	// Initialize MCP/LSP handlers
	var mcpHandler *handlers.MCPHandler
	var lspHandler *handlers.LSPHandler
	var mcpManager *services.MCPManager

	if multiConfig.MCP != nil && multiConfig.MCP.Enabled {
		mcpHandler = handlers.NewMCPHandler(providerRegistry, multiConfig.MCP)
		mcpManager = mcpHandler.GetMCPManager()
	}

	if multiConfig.LSP != nil && multiConfig.LSP.Enabled {
		lspHandler = handlers.NewLSPHandler(providerRegistry, multiConfig.LSP)
		// LSP client will be initialized per request
	}

	// Initialize tool registry
	toolRegistry := services.NewToolRegistry(mcpManager, nil) // LSP client added later if needed
	if err := toolRegistry.RefreshTools(context.Background()); err != nil {
		log.Printf("Failed to refresh tools: %v", err)
	}

	// Initialize Gin
	if !multiConfig.Server.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now().Unix(),
			"version":   "1.0.0",
		})
	})

	// Register OpenAI-compatible routes
	api := router.Group("/v1")
	auth := func(c *gin.Context) { /* Simple auth for now */ }
	unifiedHandler.RegisterOpenAIRoutes(api, auth)

	// Admin routes for debugging
	admin := router.Group("/admin")
	{
		admin.GET("/providers", func(c *gin.Context) {
			providers := providerRegistry.ListProviders()
			c.JSON(http.StatusOK, gin.H{
				"providers": providers,
				"count":     len(providers),
			})
		})

		admin.GET("/ensemble/status", func(c *gin.Context) {
			ensemble := providerRegistry.GetEnsembleService()
			if ensemble == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ensemble service not available"})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"status":        "active",
				"strategy":      multiConfig.Ensemble.Strategy,
				"min_providers": multiConfig.Ensemble.MinProviders,
				"max_providers": multiConfig.Ensemble.MaxProviders,
			})
		})
	}

	// MCP (Model Context Protocol) endpoints
	if mcpHandler != nil {
		mcp := router.Group("/mcp")
		{
			mcp.GET("/capabilities", mcpHandler.MCPCapabilities)
			mcp.GET("/tools", mcpHandler.MCPTools)
			mcp.POST("/tools/call", mcpHandler.MCPToolsCall)
			mcp.GET("/prompts", mcpHandler.MCPPrompts)
			mcp.GET("/resources", mcpHandler.MCPResources)
		}
	}

	// LSP (Language Server Protocol) endpoints
	if lspHandler != nil {
		lsp := router.Group("/lsp")
		{
			lsp.GET("/capabilities", lspHandler.LSPCapabilities)
			lsp.POST("/completion", lspHandler.LSPCompletion)
			lsp.POST("/hover", lspHandler.LSPHover)
			lsp.POST("/codeActions", lspHandler.LSPCodeActions)
		}
	}

	// Start server
	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", multiConfig.Server.Host, multiConfig.Server.Port),
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Starting SuperAgent server on %s:%d", multiConfig.Server.Host, multiConfig.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}

// convertToRegistryConfig converts MultiProviderConfig to RegistryConfig
func convertToRegistryConfig(multiConfig *config.MultiProviderConfig) *services.RegistryConfig {
	registryConfig := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxRetries:     3,
		HealthCheck: services.HealthCheckConfig{
			Interval:         30 * time.Second,
			Timeout:          10 * time.Second,
			FailureThreshold: 3,
		},
		Providers: make(map[string]*services.ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy:            multiConfig.Ensemble.Strategy,
			MinProviders:        multiConfig.Ensemble.MinProviders,
			ConfidenceThreshold: multiConfig.Ensemble.ConfidenceThreshold,
			FallbackToBest:      multiConfig.Ensemble.FallbackToBest,
			Timeout:             int(multiConfig.Ensemble.Timeout.Seconds()),
			PreferredProviders:  multiConfig.Ensemble.PreferredProviders,
		},
		Routing: &services.RoutingConfig{
			Strategy: "weighted",
			Weights:  multiConfig.Ensemble.ProviderWeights,
		},
	}

	// Convert providers
	for name, provider := range multiConfig.Providers {
		registryConfig.Providers[name] = &services.ProviderConfig{
			Name:           provider.Name,
			Type:           provider.Type,
			Enabled:        provider.Enabled,
			APIKey:         provider.APIKey,
			BaseURL:        provider.BaseURL,
			Timeout:        provider.Timeout,
			MaxRetries:     provider.MaxRetries,
			Weight:         provider.Weight,
			Tags:           provider.Tags,
			Capabilities:   provider.Capabilities,
			CustomSettings: provider.CustomSettings,
			Models:         convertModelConfigs(provider.Models),
		}
	}

	return registryConfig
}

// convertModelConfigs converts []config.ModelConfig to []services.ModelConfig
func convertModelConfigs(modelConfigs []config.ModelConfig) []services.ModelConfig {
	var result []services.ModelConfig
	for _, mc := range modelConfigs {
		result = append(result, services.ModelConfig{
			ID:           mc.ID,
			Name:         mc.Name,
			Enabled:      mc.Enabled,
			Weight:       mc.Weight,
			Capabilities: mc.Capabilities,
			CustomParams: mc.CustomParams,
		})
	}
	return result
}
