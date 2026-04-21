// Package service_wiring_test provides in-process service-wiring tests for
// HelixAgent. These exercise in-memory construction of the provider registry,
// debate team config, MCP/LSP/ACP managers, and other services — no live
// :7061 HTTP. CONST-030 compliant via Pattern-4 demote-to-unit.
package service_wiring_test

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	security "dev.helix.agent/internal/adapters/security"
	"dev.helix.agent/internal/background"
	"dev.helix.agent/internal/cache"
	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/database"
	"dev.helix.agent/internal/middleware"
	"dev.helix.agent/internal/notifications"
	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/testutil"
	"dev.helix.agent/internal/verifier"
	"digital.vasic.debate"
	"digital.vasic.debate/orchestrator"
	"digital.vasic.toolschema"
)

// TestServiceWiring_ProviderServices tests that provider services are properly wired
func TestServiceWiring_ProviderServices(t *testing.T) {
	// Integration test — no external deps required

	t.Run("ProviderRegistry initialization", func(t *testing.T) {
		// Create config
		registryConfig := services.LoadRegistryConfigFromAppConfig(nil)
		require.NotNil(t, registryConfig, "Registry config should be created")

		// Create provider registry without auto-discovery to avoid real API calls
		registry := services.NewProviderRegistryWithoutAutoDiscovery(registryConfig, nil)
		require.NotNil(t, registry, "ProviderRegistry should be initialized")

		// Verify ensemble service is wired
		ensembleService := registry.GetEnsembleService()
		require.NotNil(t, ensembleService, "EnsembleService should be wired to registry")

		// Verify request service is wired
		requestService := registry.GetRequestService()
		require.NotNil(t, requestService, "RequestService should be wired to registry")

		// Verify discovery service can be retrieved (may be nil without auto-discovery)
		discovery := registry.GetDiscovery()
		// Discovery is nil when auto-discovery is disabled
		assert.Nil(t, discovery, "Discovery should be nil when auto-discovery is disabled")
	})

	t.Run("ProviderRegistry with auto-discovery", func(t *testing.T) {
		registryConfig := services.LoadRegistryConfigFromAppConfig(nil)
		registry := services.NewProviderRegistry(registryConfig, nil)
		require.NotNil(t, registry, "ProviderRegistry with auto-discovery should be initialized")

		// Verify discovery service is wired with auto-discovery enabled
		discovery := registry.GetDiscovery()
		assert.NotNil(t, discovery, "Discovery should be wired when auto-discovery is enabled")
	})

	t.Run("ProviderRegistry known provider types", func(t *testing.T) {
		registryConfig := services.LoadRegistryConfigFromAppConfig(nil)
		registry := services.NewProviderRegistryWithoutAutoDiscovery(registryConfig, nil)

		providerTypes := registry.GetKnownProviderTypes()
		require.NotEmpty(t, providerTypes, "Should have known provider types")

		// Verify expected core providers are in the list
		// Note: Available provider types come from provider_discovery.go ProviderMappings
		expectedProviders := []string{"claude", "deepseek", "gemini", "mistral", "openrouter", "qwen", "zen", "cerebras", "ollama"}
		for _, expected := range expectedProviders {
			found := false
			for _, pt := range providerTypes {
				if pt == expected {
					found = true
					break
				}
			}
			assert.True(t, found, "Provider type %s should be known", expected)
		}
	})

	t.Run("ProviderRegistry health check", func(t *testing.T) {
		registryConfig := services.LoadRegistryConfigFromAppConfig(nil)
		registry := services.NewProviderRegistryWithoutAutoDiscovery(registryConfig, nil)

		// Health check should work even with no providers
		health := registry.HealthCheck()
		assert.NotNil(t, health, "HealthCheck should return a map")
	})

	t.Run("ProviderRegistry circuit breaker", func(t *testing.T) {
		registryConfig := services.LoadRegistryConfigFromAppConfig(nil)
		registryConfig.CircuitBreaker.Enabled = true
		registry := services.NewProviderRegistryWithoutAutoDiscovery(registryConfig, nil)
		require.NotNil(t, registry, "Registry with circuit breaker should be initialized")
	})
}

// TestServiceWiring_DebateServices tests that debate services are properly wired
func TestServiceWiring_DebateServices(t *testing.T) {
	// Integration test — no external deps required

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	t.Run("DebateService initialization", func(t *testing.T) {
		debateService := services.NewDebateService(logger)
		require.NotNil(t, debateService, "DebateService should be initialized")

		// Verify comm logger is wired
		commLogger := debateService.GetCommLogger()
		assert.NotNil(t, commLogger, "CommLogger should be wired")
	})

	t.Run("DebateService with dependencies", func(t *testing.T) {
		registryConfig := services.LoadRegistryConfigFromAppConfig(nil)
		registry := services.NewProviderRegistryWithoutAutoDiscovery(registryConfig, nil)

		debateService := services.NewDebateServiceWithDeps(logger, registry, nil)
		require.NotNil(t, debateService, "DebateService with deps should be initialized")
	})

	t.Run("DebateTeamConfig initialization", func(t *testing.T) {
		registryConfig := services.LoadRegistryConfigFromAppConfig(nil)
		registry := services.NewProviderRegistryWithoutAutoDiscovery(registryConfig, nil)

		teamConfig := services.NewDebateTeamConfig(registry, nil, logger)
		require.NotNil(t, teamConfig, "DebateTeamConfig should be initialized")

		// Verify team summary can be retrieved
		summary := teamConfig.GetTeamSummary()
		assert.NotNil(t, summary, "Team summary should be retrievable")
	})

	t.Run("DebateService with team config", func(t *testing.T) {
		registryConfig := services.LoadRegistryConfigFromAppConfig(nil)
		registry := services.NewProviderRegistryWithoutAutoDiscovery(registryConfig, nil)

		teamConfig := services.NewDebateTeamConfig(registry, nil, logger)
		debateService := services.NewDebateServiceWithDeps(logger, registry, nil)
		debateService.SetTeamConfig(teamConfig)

		retrievedConfig := debateService.GetTeamConfig()
		assert.Equal(t, teamConfig, retrievedConfig, "Team config should be properly set")
	})

	t.Run("MultiPassValidation config", func(t *testing.T) {
		config := services.DefaultValidationConfig()
		require.NotNil(t, config, "DefaultValidationConfig should be created")

		assert.True(t, config.EnableValidation, "Validation should be enabled by default")
		assert.True(t, config.EnablePolish, "Polish should be enabled by default")
		assert.Greater(t, config.ValidationTimeout, time.Duration(0), "ValidationTimeout should be positive")
		assert.Greater(t, config.PolishTimeout, time.Duration(0), "PolishTimeout should be positive")
	})

	t.Run("ValidationPhases structure", func(t *testing.T) {
		phases := services.ValidationPhases()
		require.Len(t, phases, 4, "Should have 4 validation phases")

		// Verify phase order
		assert.Equal(t, services.PhaseInitialResponse, phases[0].Phase)
		assert.Equal(t, services.PhaseValidation, phases[1].Phase)
		assert.Equal(t, services.PhasePolishImprove, phases[2].Phase)
		assert.Equal(t, services.PhaseFinalConclusion, phases[3].Phase)
	})

	t.Run("Orchestrator integration", func(t *testing.T) {
		// Test default orchestrator config
		config := orchestrator.DefaultOrchestratorConfig()
		require.NotNil(t, config, "DefaultOrchestratorConfig should be created")

		// Verify default config values
		assert.Greater(t, config.DefaultMaxRounds, 0, "DefaultMaxRounds should be positive")
		assert.Greater(t, config.DefaultTimeout, time.Duration(0), "DefaultTimeout should be positive")
		assert.Greater(t, config.MinAgentsPerDebate, 0, "MinAgentsPerDebate should be positive")
		assert.True(t, config.EnableLearning, "EnableLearning should be true by default")
		assert.True(t, config.EnableCrossDebateLearning, "EnableCrossDebateLearning should be true by default")
	})

	t.Run("LessonBank config", func(t *testing.T) {
		config := debate.DefaultLessonBankConfig()
		require.NotNil(t, config, "DefaultLessonBankConfig should be created")

		assert.Greater(t, config.MaxLessons, 0, "MaxLessons should be positive")
		assert.Greater(t, config.MinConfidence, 0.0, "MinConfidence should be positive")
		assert.True(t, config.EnableSemanticSearch, "EnableSemanticSearch should be true by default")
	})
}

// mockOrchestratorRegistry adapts ProviderRegistry to orchestrator.ProviderRegistry interface
type mockOrchestratorRegistry struct {
	registry *services.ProviderRegistry
}

func (m *mockOrchestratorRegistry) GetProvider(name string) (llmInterface, error) {
	return m.registry.GetProvider(name)
}

func (m *mockOrchestratorRegistry) GetAvailableProviders() []string {
	return m.registry.ListProviders()
}

// llmInterface is used for type compatibility in tests
type llmInterface = interface {
	// Intentionally empty - used for type compatibility
}

// TestServiceWiring_MCPLSPACPServices tests MCP/LSP/ACP services wiring
func TestServiceWiring_MCPLSPACPServices(t *testing.T) {
	// Integration test — no external deps required

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	t.Run("MCPClient initialization", func(t *testing.T) {
		mcpClient := services.NewMCPClient(logger)
		require.NotNil(t, mcpClient, "MCPClient should be initialized")
	})

	t.Run("LSPManager initialization", func(t *testing.T) {
		lspManager := services.NewLSPManager(nil, nil, logger)
		require.NotNil(t, lspManager, "LSPManager should be initialized")
	})

	t.Run("LSPManager with config", func(t *testing.T) {
		config := services.DefaultLSPConfig()
		require.NotNil(t, config, "DefaultLSPConfig should be created")

		// Verify default server configs
		assert.Contains(t, config.ServerConfigs, "gopls", "Should have gopls config")
		assert.Contains(t, config.ServerConfigs, "rust-analyzer", "Should have rust-analyzer config")
		assert.Contains(t, config.ServerConfigs, "pylsp", "Should have pylsp config")
		assert.Contains(t, config.ServerConfigs, "ts-language-server", "Should have ts-language-server config")
	})

	t.Run("UnifiedProtocolManager initialization", func(t *testing.T) {
		protocolManager := services.NewUnifiedProtocolManager(nil, nil, logger)
		require.NotNil(t, protocolManager, "UnifiedProtocolManager should be initialized")
	})
}

// TestServiceWiring_ToolServices tests tool services wiring
func TestServiceWiring_ToolServices(t *testing.T) {
	// Integration test — no external deps required

	t.Run("ToolSchemaRegistry exists and has tools", func(t *testing.T) {
		registry := tools.ToolSchemaRegistry
		require.NotNil(t, registry, "ToolSchemaRegistry should exist")
		assert.NotEmpty(t, registry, "ToolSchemaRegistry should have tools")
	})

	t.Run("Core tools are registered", func(t *testing.T) {
		coreTools := []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"}
		for _, toolName := range coreTools {
			schema, found := tools.GetToolSchema(toolName)
			assert.True(t, found, "Tool %s should be registered", toolName)
			assert.NotNil(t, schema, "Tool %s schema should not be nil", toolName)
		}
	})

	t.Run("Web tools are registered", func(t *testing.T) {
		webTools := []string{"WebFetch", "WebSearch"}
		for _, toolName := range webTools {
			schema, found := tools.GetToolSchema(toolName)
			assert.True(t, found, "Tool %s should be registered", toolName)
			assert.Equal(t, tools.CategoryWeb, schema.Category, "Tool %s should be in web category", toolName)
		}
	})

	t.Run("Version control tools are registered", func(t *testing.T) {
		vcTools := []string{"Git", "Diff"}
		for _, toolName := range vcTools {
			schema, found := tools.GetToolSchema(toolName)
			assert.True(t, found, "Tool %s should be registered", toolName)
			assert.Equal(t, tools.CategoryVersionControl, schema.Category, "Tool %s should be in version_control category", toolName)
		}
	})

	t.Run("Workflow tools are registered", func(t *testing.T) {
		workflowTools := []string{"PR", "Issue", "Workflow"}
		for _, toolName := range workflowTools {
			schema, found := tools.GetToolSchema(toolName)
			assert.True(t, found, "Tool %s should be registered", toolName)
			assert.Equal(t, tools.CategoryWorkflow, schema.Category, "Tool %s should be in workflow category", toolName)
		}
	})

	t.Run("Tool schema validation", func(t *testing.T) {
		for name, schema := range tools.ToolSchemaRegistry {
			assert.NotEmpty(t, schema.Name, "Tool %s should have a name", name)
			assert.NotEmpty(t, schema.Description, "Tool %s should have a description", name)
			assert.NotEmpty(t, schema.RequiredFields, "Tool %s should have required fields", name)
			assert.NotEmpty(t, schema.Category, "Tool %s should have a category", name)
		}
	})

	t.Run("Tool alias resolution", func(t *testing.T) {
		// Test lowercase alias
		schema, found := tools.GetToolSchema("bash")
		assert.True(t, found, "Should find tool by lowercase alias")
		assert.Equal(t, "Bash", schema.Name, "Should resolve to correct tool")

		// Test shell alias
		schema, found = tools.GetToolSchema("shell")
		assert.True(t, found, "Should find tool by shell alias")
		assert.Equal(t, "Bash", schema.Name, "Should resolve to Bash tool")
	})
}

// TestServiceWiring_CacheServices tests cache services wiring
func TestServiceWiring_CacheServices(t *testing.T) {
	// Integration test — no external deps required

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	t.Run("CacheFactory initialization", func(t *testing.T) {
		factory := services.NewCacheFactory(nil, logger)
		require.NotNil(t, factory, "CacheFactory should be initialized")
	})

	t.Run("CacheFactory creates in-memory cache without Redis", func(t *testing.T) {
		factory := services.NewCacheFactory(nil, logger)
		cache := factory.CreateDefaultCache(30 * time.Minute)
		require.NotNil(t, cache, "Should create cache without Redis")
	})

	t.Run("CacheFactory creates different cache types", func(t *testing.T) {
		factory := services.NewCacheFactory(nil, logger)

		memoryCache := factory.CreateCache("memory", 30*time.Minute)
		require.NotNil(t, memoryCache, "Should create memory cache")

		// Redis cache falls back to memory when Redis unavailable
		redisCache := factory.CreateCache("redis", 30*time.Minute)
		require.NotNil(t, redisCache, "Should create cache (fallback to memory)")
	})

	t.Run("InMemoryCache initialization", func(t *testing.T) {
		inMemoryCache := services.NewInMemoryCache(30 * time.Minute)
		require.NotNil(t, inMemoryCache, "InMemoryCache should be initialized")
	})

	t.Run("RedisClient initialization without config", func(t *testing.T) {
		// Creating with nil config should still work but won't connect
		redisClient := cache.NewRedisClient(nil)
		require.NotNil(t, redisClient, "RedisClient should be initialized even without config")
	})
}

// TestServiceWiring_BackgroundServices tests background task services wiring
func TestServiceWiring_BackgroundServices(t *testing.T) {
	// Integration test — no external deps required

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	t.Run("WorkerPoolConfig defaults", func(t *testing.T) {
		config := background.DefaultWorkerPoolConfig()
		require.NotNil(t, config, "DefaultWorkerPoolConfig should be created")

		assert.Greater(t, config.MinWorkers, 0, "MinWorkers should be positive")
		assert.Greater(t, config.MaxWorkers, 0, "MaxWorkers should be positive")
		assert.GreaterOrEqual(t, config.MaxWorkers, config.MinWorkers, "MaxWorkers should be >= MinWorkers")
		assert.Greater(t, config.ScaleInterval, time.Duration(0), "ScaleInterval should be positive")
		assert.Greater(t, config.QueuePollInterval, time.Duration(0), "QueuePollInterval should be positive")
	})

	t.Run("PostgresTaskQueue initialization with nil repository", func(t *testing.T) {
		// This tests that the queue can be created even without a real repository
		// In production, a real repository would be provided
		defer func() {
			// Expect this to panic or handle gracefully
			if r := recover(); r != nil {
				t.Log("Expected: PostgresTaskQueue requires a repository")
			}
		}()

		// Note: This may panic if the implementation requires a non-nil repository
		queue := background.NewPostgresTaskQueue(nil, logger)
		if queue != nil {
			t.Log("PostgresTaskQueue created with nil repository (allowed)")
		}
	})
}

// TestServiceWiring_NotificationServices tests notification services wiring
func TestServiceWiring_NotificationServices(t *testing.T) {
	// Integration test — no external deps required

	t.Run("HubConfig defaults", func(t *testing.T) {
		config := notifications.DefaultHubConfig()
		require.NotNil(t, config, "DefaultHubConfig should be created")

		assert.Greater(t, config.EventBufferSize, 0, "EventBufferSize should be positive")
		assert.Greater(t, config.WorkerCount, 0, "WorkerCount should be positive")
		assert.Greater(t, config.NotificationTimeout, time.Duration(0), "NotificationTimeout should be positive")
	})

	t.Run("SSEManager initialization", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.WarnLevel)

		sseConfig := notifications.DefaultSSEConfig()
		require.NotNil(t, sseConfig, "DefaultSSEConfig should be created")

		sseManager := notifications.NewSSEManager(sseConfig, logger)
		require.NotNil(t, sseManager, "SSEManager should be initialized")
	})

	t.Run("WebSocketServer initialization", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.WarnLevel)

		config := &notifications.WebSocketConfig{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			PingInterval:    30 * time.Second,
		}

		wsServer := notifications.NewWebSocketServer(config, logger)
		require.NotNil(t, wsServer, "WebSocketServer should be initialized")
	})

	t.Run("PollingStore initialization", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.WarnLevel)

		config := notifications.DefaultPollingConfig()
		require.NotNil(t, config, "DefaultPollingConfig should be created")

		pollingStore := notifications.NewPollingStore(config, logger)
		require.NotNil(t, pollingStore, "PollingStore should be initialized")
	})
}

// TestServiceWiring_MonitoringServices tests monitoring services wiring
func TestServiceWiring_MonitoringServices(t *testing.T) {
	// Integration test — no external deps required

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	t.Run("OAuthTokenMonitor initialization", func(t *testing.T) {
		config := services.DefaultOAuthTokenMonitorConfig()
		require.NotNil(t, config, "DefaultOAuthTokenMonitorConfig should be created")

		monitor := services.NewOAuthTokenMonitor(logger, config)
		require.NotNil(t, monitor, "OAuthTokenMonitor should be initialized")
	})

	t.Run("OAuthTokenMonitor start and stop", func(t *testing.T) {
		config := services.DefaultOAuthTokenMonitorConfig()
		config.CheckInterval = 100 * time.Millisecond // Short interval for testing

		monitor := services.NewOAuthTokenMonitor(logger, config)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start in background
		go monitor.Start(ctx)

		// Give it time to start
		time.Sleep(50 * time.Millisecond)

		// Stop should be graceful
		cancel()
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("ProviderHealthMonitor initialization", func(t *testing.T) {
		registryConfig := services.LoadRegistryConfigFromAppConfig(nil)
		registry := services.NewProviderRegistryWithoutAutoDiscovery(registryConfig, nil)

		config := services.DefaultProviderHealthMonitorConfig()
		require.NotNil(t, config, "DefaultProviderHealthMonitorConfig should be created")

		monitor := services.NewProviderHealthMonitor(registry, logger, config)
		require.NotNil(t, monitor, "ProviderHealthMonitor should be initialized")
	})

	t.Run("FallbackChainValidator initialization", func(t *testing.T) {
		registryConfig := services.LoadRegistryConfigFromAppConfig(nil)
		registry := services.NewProviderRegistryWithoutAutoDiscovery(registryConfig, nil)

		teamConfig := services.NewDebateTeamConfig(registry, nil, logger)
		validator := services.NewFallbackChainValidator(logger, teamConfig)
		require.NotNil(t, validator, "FallbackChainValidator should be initialized")
	})

	t.Run("FallbackChainValidator validation", func(t *testing.T) {
		registryConfig := services.LoadRegistryConfigFromAppConfig(nil)
		registry := services.NewProviderRegistryWithoutAutoDiscovery(registryConfig, nil)

		teamConfig := services.NewDebateTeamConfig(registry, nil, logger)
		validator := services.NewFallbackChainValidator(logger, teamConfig)

		result := validator.Validate()
		require.NotNil(t, result, "Validation result should not be nil")
		// Result may or may not be valid depending on team config state
		assert.NotNil(t, result.Positions, "Positions should be in result")
	})
}

// TestServiceWiring_SecurityServices tests security services wiring
func TestServiceWiring_SecurityServices(t *testing.T) {
	// Integration test — no external deps required

	t.Run("AuthMiddleware initialization", func(t *testing.T) {
		config := middleware.AuthConfig{
			SecretKey:   "test-secret-key-for-testing",
			TokenExpiry: 24 * time.Hour,
			Issuer:      "helixagent-test",
		}

		auth, err := middleware.NewAuthMiddleware(config, nil)
		require.NoError(t, err, "AuthMiddleware should be created")
		require.NotNil(t, auth, "AuthMiddleware should not be nil")
	})

	t.Run("AuthMiddleware requires secret key", func(t *testing.T) {
		config := middleware.AuthConfig{
			SecretKey: "", // Empty secret
		}

		_, err := middleware.NewAuthMiddleware(config, nil)
		assert.Error(t, err, "Should fail without secret key")
	})

	t.Run("RateLimiter initialization", func(t *testing.T) {
		rateLimiter := middleware.NewRateLimiter(nil)
		require.NotNil(t, rateLimiter, "RateLimiter should be initialized")
	})

	t.Run("RateLimiter with custom config", func(t *testing.T) {
		config := &middleware.RateLimitConfig{
			Requests: 100,
			Window:   time.Minute,
		}

		rateLimiter := middleware.NewRateLimiterWithConfig(nil, config)
		require.NotNil(t, rateLimiter, "RateLimiter with config should be initialized")
	})

	t.Run("AuditLogger initialization", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.WarnLevel)

		auditLogger := security.NewInMemoryAuditLogger(1000, logger)
		require.NotNil(t, auditLogger, "AuditLogger should be initialized")
	})

	t.Run("AuditLogger logging", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.WarnLevel)

		auditLogger := security.NewInMemoryAuditLogger(1000, logger)

		event := &security.AuditEvent{
			EventType: "test_event",
			Action:    "test_action",
			Result:    "success",
			UserID:    "test-user",
		}

		err := auditLogger.Log(context.Background(), event)
		assert.NoError(t, err, "Should log audit event")
	})
}

// TestServiceWiring_CompleteServiceGraph tests that the complete service graph is wired
func TestServiceWiring_CompleteServiceGraph(t *testing.T) {
	// Integration test — no external deps required

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	t.Run("Complete service initialization", func(t *testing.T) {
		// 1. Provider Services
		registryConfig := services.LoadRegistryConfigFromAppConfig(nil)
		providerRegistry := services.NewProviderRegistryWithoutAutoDiscovery(registryConfig, nil)
		require.NotNil(t, providerRegistry, "ProviderRegistry should be initialized")

		// 2. Cache Services
		cacheFactory := services.NewCacheFactory(nil, logger)
		require.NotNil(t, cacheFactory, "CacheFactory should be initialized")

		sharedCache := cacheFactory.CreateDefaultCache(30 * time.Minute)
		require.NotNil(t, sharedCache, "Shared cache should be created")

		// 3. Debate Services
		debateTeamConfig := services.NewDebateTeamConfig(providerRegistry, nil, logger)
		require.NotNil(t, debateTeamConfig, "DebateTeamConfig should be initialized")

		debateService := services.NewDebateServiceWithDeps(logger, providerRegistry, nil)
		require.NotNil(t, debateService, "DebateService should be initialized")

		debateService.SetTeamConfig(debateTeamConfig)

		// 4. MCP/LSP Services
		mcpClient := services.NewMCPClient(logger)
		require.NotNil(t, mcpClient, "MCPClient should be initialized")

		lspManager := services.NewLSPManager(nil, sharedCache, logger)
		require.NotNil(t, lspManager, "LSPManager should be initialized")

		protocolManager := services.NewUnifiedProtocolManager(nil, sharedCache, logger)
		require.NotNil(t, protocolManager, "ProtocolManager should be initialized")

		// 5. Monitoring Services
		oauthMonitorConfig := services.DefaultOAuthTokenMonitorConfig()
		oauthTokenMonitor := services.NewOAuthTokenMonitor(logger, oauthMonitorConfig)
		require.NotNil(t, oauthTokenMonitor, "OAuthTokenMonitor should be initialized")

		healthMonitorConfig := services.DefaultProviderHealthMonitorConfig()
		providerHealthMonitor := services.NewProviderHealthMonitor(providerRegistry, logger, healthMonitorConfig)
		require.NotNil(t, providerHealthMonitor, "ProviderHealthMonitor should be initialized")

		fallbackValidator := services.NewFallbackChainValidator(logger, debateTeamConfig)
		require.NotNil(t, fallbackValidator, "FallbackChainValidator should be initialized")

		// 6. Security Services
		authConfig := middleware.AuthConfig{
			SecretKey:   "test-secret-for-integration",
			TokenExpiry: 24 * time.Hour,
			Issuer:      "helixagent-test",
		}
		authMiddleware, err := middleware.NewAuthMiddleware(authConfig, nil)
		require.NoError(t, err, "AuthMiddleware should be created")
		require.NotNil(t, authMiddleware, "AuthMiddleware should not be nil")

		rateLimiter := middleware.NewRateLimiter(nil)
		require.NotNil(t, rateLimiter, "RateLimiter should be initialized")

		auditLogger := security.NewInMemoryAuditLogger(10000, logger)
		require.NotNil(t, auditLogger, "AuditLogger should be initialized")

		// 7. Notification Services
		sseConfig := notifications.DefaultSSEConfig()
		sseManager := notifications.NewSSEManager(sseConfig, logger)
		require.NotNil(t, sseManager, "SSEManager should be initialized")

		wsConfig := &notifications.WebSocketConfig{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			PingInterval:    30 * time.Second,
		}
		wsServer := notifications.NewWebSocketServer(wsConfig, logger)
		require.NotNil(t, wsServer, "WebSocketServer should be initialized")

		t.Log("All services successfully wired and initialized")
	})
}

// TestServiceWiring_DatabaseIntegration tests database-related service wiring
func TestServiceWiring_DatabaseIntegration(t *testing.T) {
	// Integration test — no external deps required

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	t.Run("ModelMetadataRepository without pool", func(t *testing.T) {
		// Repository should work in standalone mode with nil pool
		repo := database.NewModelMetadataRepository(nil, logger)
		require.NotNil(t, repo, "ModelMetadataRepository should be initialized with nil pool")
	})
}

// TestServiceWiring_ConfigLoading tests configuration loading and service wiring
func TestServiceWiring_ConfigLoading(t *testing.T) {
	// Integration test — no external deps required

	t.Run("LoadRegistryConfigFromAppConfig with nil", func(t *testing.T) {
		cfg := services.LoadRegistryConfigFromAppConfig(nil)
		require.NotNil(t, cfg, "Should create config from nil app config")

		assert.Greater(t, cfg.DefaultTimeout, time.Duration(0), "DefaultTimeout should be positive")
		assert.Greater(t, cfg.MaxRetries, 0, "MaxRetries should be positive")
		assert.NotNil(t, cfg.HealthCheck, "HealthCheck config should exist")
		assert.NotNil(t, cfg.CircuitBreaker, "CircuitBreaker config should exist")
		assert.NotNil(t, cfg.Ensemble, "Ensemble config should exist")
		assert.NotNil(t, cfg.Routing, "Routing config should exist")
	})

	t.Run("LoadRegistryConfigFromAppConfig with app config", func(t *testing.T) {
		appConfig := &config.Config{
			LLM: config.LLMConfig{
				DefaultTimeout: 60 * time.Second,
				MaxRetries:     5,
			},
		}

		cfg := services.LoadRegistryConfigFromAppConfig(appConfig)
		require.NotNil(t, cfg, "Should create config from app config")

		assert.Equal(t, 60*time.Second, cfg.DefaultTimeout, "DefaultTimeout should match app config")
		assert.Equal(t, 5, cfg.MaxRetries, "MaxRetries should match app config")
	})
}

// TestServiceWiring_EnsembleService tests ensemble service wiring
func TestServiceWiring_EnsembleService(t *testing.T) {
	// Integration test — no external deps required

	t.Run("EnsembleService initialization", func(t *testing.T) {
		ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)
		require.NotNil(t, ensemble, "EnsembleService should be initialized")
	})

	t.Run("EnsembleService with different strategies", func(t *testing.T) {
		strategies := []string{"confidence_weighted", "majority_vote", "fastest_response", "weighted_average"}

		for _, strategy := range strategies {
			ensemble := services.NewEnsembleService(strategy, 30*time.Second)
			require.NotNil(t, ensemble, "EnsembleService should be initialized with %s strategy", strategy)
		}
	})
}

// TestServiceWiring_MemoryService tests memory service wiring
func TestServiceWiring_MemoryService(t *testing.T) {
	// Integration test — no external deps required

	t.Run("MemoryService initialization", func(t *testing.T) {
		memoryService := services.NewMemoryService(nil)
		require.NotNil(t, memoryService, "MemoryService should be initialized")
	})

	t.Run("ContextManager initialization", func(t *testing.T) {
		contextManager := services.NewContextManager(100)
		require.NotNil(t, contextManager, "ContextManager should be initialized")
	})
}

// TestServiceWiring_EmbeddingService tests embedding service wiring
func TestServiceWiring_EmbeddingService(t *testing.T) {
	// Integration test — no external deps required

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	t.Run("EmbeddingManager initialization", func(t *testing.T) {
		config := services.EmbeddingConfig{
			VectorProvider: "pgvector",
			Timeout:        30 * time.Second,
			CacheEnabled:   true,
		}

		embeddingManager := services.NewEmbeddingManagerWithConfig(nil, nil, logger, config)
		require.NotNil(t, embeddingManager, "EmbeddingManager should be initialized")
	})
}

// TestDebateTeamConfig_StartupVerifierIntegration tests that DebateTeamConfig
// properly uses StartupVerifier when available (CRITICAL: OAuth providers like
// Claude and Qwen will NOT be included in the debate team without this!)
func TestDebateTeamConfig_StartupVerifierIntegration(t *testing.T) {
	testutil.RequireServer(t)
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	t.Run("DebateTeamConfig uses StartupVerifier when set", func(t *testing.T) {
		// Create provider registry
		providerRegistry := services.NewProviderRegistry(nil, nil)
		require.NotNil(t, providerRegistry)

		// Create StartupVerifier with mocked OAuth provider
		cfg := verifier.DefaultStartupConfig()
		startupVerifier := verifier.NewStartupVerifier(cfg, logger)
		require.NotNil(t, startupVerifier)

		// Set the startup verifier on the registry
		providerRegistry.SetStartupVerifier(startupVerifier)

		// Create debate team config
		debateTeamConfig := services.NewDebateTeamConfig(providerRegistry, nil, logger)
		require.NotNil(t, debateTeamConfig)

		// CRITICAL: Set the StartupVerifier on the DebateTeamConfig
		// This is the fix that ensures OAuth providers are included
		sv := providerRegistry.GetStartupVerifier()
		if sv != nil {
			debateTeamConfig.SetStartupVerifier(sv)
		}

		// Verify StartupVerifier is set (no direct getter, but we can check behavior)
		// The DebateTeamConfig should use StartupVerifier path when collecting LLMs
		assert.NotNil(t, sv, "StartupVerifier should be retrievable from registry")
	})

	t.Run("DebateTeamConfig without StartupVerifier falls back to legacy", func(t *testing.T) {
		// Create provider registry WITHOUT StartupVerifier
		providerRegistry := services.NewProviderRegistry(nil, nil)
		require.NotNil(t, providerRegistry)

		// Create debate team config without setting StartupVerifier
		debateTeamConfig := services.NewDebateTeamConfig(providerRegistry, nil, logger)
		require.NotNil(t, debateTeamConfig)

		// Verify no StartupVerifier is set
		sv := providerRegistry.GetStartupVerifier()
		assert.Nil(t, sv, "StartupVerifier should be nil when not set")
	})

	t.Run("Router pattern: set StartupVerifier on DebateTeamConfig", func(t *testing.T) {
		// This test mimics the exact pattern used in router.go
		providerRegistry := services.NewProviderRegistry(nil, nil)

		// Simulate startup verification setting the verifier
		cfg := verifier.DefaultStartupConfig()
		startupVerifier := verifier.NewStartupVerifier(cfg, logger)
		providerRegistry.SetStartupVerifier(startupVerifier)

		// Create debate team config (as done in router.go)
		debateTeamConfig := services.NewDebateTeamConfig(
			providerRegistry,
			providerRegistry.GetDiscovery(),
			logger,
		)

		// CRITICAL FIX: Set the StartupVerifier so OAuth providers are included
		if sv := providerRegistry.GetStartupVerifier(); sv != nil {
			debateTeamConfig.SetStartupVerifier(sv)
		}

		// Verify the pattern works
		require.NotNil(t, debateTeamConfig)
		sv := providerRegistry.GetStartupVerifier()
		assert.NotNil(t, sv, "StartupVerifier should be set")
	})
}

// TestVerificationTimeout tests that the verification timeout is sufficient
// for slow providers like Zen (free) and ZAI (GLM)
func TestVerificationTimeout(t *testing.T) {
	t.Run("Default timeout is 120 seconds for slow providers", func(t *testing.T) {
		cfg := verifier.DefaultStartupConfig()

		// The timeout should be at least 2 minutes to handle slow providers
		// Zen takes ~10s per model, ZAI can also be slow
		assert.GreaterOrEqual(t, cfg.VerificationTimeout, 120*time.Second,
			"Verification timeout should be at least 2 minutes for slow providers")
	})

	t.Run("OAuth trust is enabled by default", func(t *testing.T) {
		cfg := verifier.DefaultStartupConfig()

		// OAuth trust should be enabled so Claude and Qwen are included
		// even if their product-restricted tokens fail API verification
		assert.True(t, cfg.TrustOAuthOnFailure,
			"TrustOAuthOnFailure should be true to include OAuth providers")
	})

	t.Run("OAuth priority boost defaults to zero for pure score-based ranking", func(t *testing.T) {
		cfg := verifier.DefaultStartupConfig()

		// OAuthPriorityBoost is 0.0 by default - all providers are sorted purely by score
		// OAuth providers are still included via TrustOAuthOnFailure
		assert.Equal(t, 0.0, cfg.OAuthPriorityBoost,
			"OAuthPriorityBoost should be 0.0 for pure score-based ranking")
	})
}
