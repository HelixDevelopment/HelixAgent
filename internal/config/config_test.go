package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	for _, key := range []string{
		"PORT", "SUPERAGENT_API_KEY", "JWT_SECRET",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"REDIS_HOST", "REDIS_PORT",
		"COGNEE_BASE_URL", "COGNEE_API_KEY",
		"LLM_TIMEOUT", "LLM_MAX_RETRIES",
		"ENSEMBLE_STRATEGY", "ENSEMBLE_MIN_PROVIDERS",
		"METRICS_ENABLED", "LOG_LEVEL",
		"RATE_LIMITING_ENABLED", "RATE_LIMIT_REQUESTS",
		"PLUGIN_AUTO_RELOAD", "PLUGIN_HOT_RELOAD",
		"MAX_CONCURRENT_REQUESTS", "REQUEST_TIMEOUT",
	} {
		originalEnv[key] = os.Getenv(key)
		os.Unsetenv(key)
	}
	defer func() {
		// Restore original environment
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	t.Run("DefaultConfig", func(t *testing.T) {
		cfg := Load()

		// Test ServerConfig defaults
		if cfg.Server.Port != "8080" {
			t.Errorf("Expected Server.Port '8080', got %s", cfg.Server.Port)
		}
		if cfg.Server.APIKey != "development-key" {
			t.Errorf("Expected Server.APIKey 'development-key', got %s", cfg.Server.APIKey)
		}
		if cfg.Server.JWTSecret != "development-jwt-secret-key-change-in-production" {
			t.Errorf("Expected Server.JWTSecret 'development-jwt-secret-key-change-in-production', got %s", cfg.Server.JWTSecret)
		}
		if cfg.Server.ReadTimeout != 30*time.Second {
			t.Errorf("Expected Server.ReadTimeout 30s, got %v", cfg.Server.ReadTimeout)
		}
		if cfg.Server.WriteTimeout != 30*time.Second {
			t.Errorf("Expected Server.WriteTimeout 30s, got %v", cfg.Server.WriteTimeout)
		}
		if cfg.Server.TokenExpiry != 24*time.Hour {
			t.Errorf("Expected Server.TokenExpiry 24h, got %v", cfg.Server.TokenExpiry)
		}
		if cfg.Server.Host != "0.0.0.0" {
			t.Errorf("Expected Server.Host '0.0.0.0', got %s", cfg.Server.Host)
		}
		if cfg.Server.Mode != "release" {
			t.Errorf("Expected Server.Mode 'release', got %s", cfg.Server.Mode)
		}
		if !cfg.Server.EnableCORS {
			t.Error("Expected Server.EnableCORS true")
		}
		if len(cfg.Server.CORSOrigins) != 1 || cfg.Server.CORSOrigins[0] != "*" {
			t.Errorf("Expected Server.CORSOrigins ['*'], got %v", cfg.Server.CORSOrigins)
		}
		if !cfg.Server.RequestLogging {
			t.Error("Expected Server.RequestLogging true")
		}
		if cfg.Server.DebugEnabled {
			t.Error("Expected Server.DebugEnabled false")
		}

		// Test DatabaseConfig defaults
		if cfg.Database.Host != "localhost" {
			t.Errorf("Expected Database.Host 'localhost', got %s", cfg.Database.Host)
		}
		if cfg.Database.Port != "5432" {
			t.Errorf("Expected Database.Port '5432', got %s", cfg.Database.Port)
		}
		if cfg.Database.User != "superagent" {
			t.Errorf("Expected Database.User 'superagent', got %s", cfg.Database.User)
		}
		if cfg.Database.Password != "secret" {
			t.Errorf("Expected Database.Password 'secret', got %s", cfg.Database.Password)
		}
		if cfg.Database.Name != "superagent_db" {
			t.Errorf("Expected Database.Name 'superagent_db', got %s", cfg.Database.Name)
		}
		if cfg.Database.SSLMode != "disable" {
			t.Errorf("Expected Database.SSLMode 'disable', got %s", cfg.Database.SSLMode)
		}
		if cfg.Database.MaxConnections != 20 {
			t.Errorf("Expected Database.MaxConnections 20, got %d", cfg.Database.MaxConnections)
		}
		if cfg.Database.ConnTimeout != 10*time.Second {
			t.Errorf("Expected Database.ConnTimeout 10s, got %v", cfg.Database.ConnTimeout)
		}
		if cfg.Database.PoolSize != 10 {
			t.Errorf("Expected Database.PoolSize 10, got %d", cfg.Database.PoolSize)
		}

		// Test RedisConfig defaults
		if cfg.Redis.Host != "localhost" {
			t.Errorf("Expected Redis.Host 'localhost', got %s", cfg.Redis.Host)
		}
		if cfg.Redis.Port != "6379" {
			t.Errorf("Expected Redis.Port '6379', got %s", cfg.Redis.Port)
		}
		if cfg.Redis.Password != "" {
			t.Errorf("Expected Redis.Password '', got %s", cfg.Redis.Password)
		}
		if cfg.Redis.DB != 0 {
			t.Errorf("Expected Redis.DB 0, got %d", cfg.Redis.DB)
		}
		if cfg.Redis.PoolSize != 10 {
			t.Errorf("Expected Redis.PoolSize 10, got %d", cfg.Redis.PoolSize)
		}
		if cfg.Redis.Timeout != 5*time.Second {
			t.Errorf("Expected Redis.Timeout 5s, got %v", cfg.Redis.Timeout)
		}

		// Test LLMConfig defaults
		if cfg.LLM.DefaultTimeout != 60*time.Second {
			t.Errorf("Expected LLM.DefaultTimeout 60s, got %v", cfg.LLM.DefaultTimeout)
		}
		if cfg.LLM.MaxRetries != 3 {
			t.Errorf("Expected LLM.MaxRetries 3, got %d", cfg.LLM.MaxRetries)
		}
		if cfg.LLM.Ensemble.Strategy != "confidence_weighted" {
			t.Errorf("Expected LLM.Ensemble.Strategy 'confidence_weighted', got %s", cfg.LLM.Ensemble.Strategy)
		}
		if cfg.LLM.Ensemble.MinProviders != 2 {
			t.Errorf("Expected LLM.Ensemble.MinProviders 2, got %d", cfg.LLM.Ensemble.MinProviders)
		}
		if cfg.LLM.Ensemble.MaxProviders != 5 {
			t.Errorf("Expected LLM.Ensemble.MaxProviders 5, got %d", cfg.LLM.Ensemble.MaxProviders)
		}
		if cfg.LLM.Ensemble.ConfidenceThreshold != 0.8 {
			t.Errorf("Expected LLM.Ensemble.ConfidenceThreshold 0.8, got %f", cfg.LLM.Ensemble.ConfidenceThreshold)
		}
		if !cfg.LLM.Ensemble.FallbackToBest {
			t.Error("Expected LLM.Ensemble.FallbackToBest true")
		}
		if cfg.LLM.Ensemble.Timeout != 30*time.Second {
			t.Errorf("Expected LLM.Ensemble.Timeout 30s, got %v", cfg.LLM.Ensemble.Timeout)
		}
		if len(cfg.LLM.Ensemble.PreferredProviders) != 0 {
			t.Errorf("Expected LLM.Ensemble.PreferredProviders empty, got %v", cfg.LLM.Ensemble.PreferredProviders)
		}
	})

	t.Run("EnvironmentOverrides", func(t *testing.T) {
		// Set environment variables
		os.Setenv("PORT", "9090")
		os.Setenv("SUPERAGENT_API_KEY", "test-api-key")
		os.Setenv("JWT_SECRET", "test-jwt-secret")
		os.Setenv("DB_HOST", "test-db-host")
		os.Setenv("DB_PORT", "5433")
		os.Setenv("DB_USER", "test-user")
		os.Setenv("DB_PASSWORD", "test-password")
		os.Setenv("DB_NAME", "test-db")
		os.Setenv("REDIS_HOST", "test-redis-host")
		os.Setenv("REDIS_PORT", "6380")
		os.Setenv("COGNEE_BASE_URL", "http://test-cognee:8000")
		os.Setenv("COGNEE_API_KEY", "test-cognee-key")
		os.Setenv("LLM_TIMEOUT", "90s")
		os.Setenv("LLM_MAX_RETRIES", "5")
		os.Setenv("ENSEMBLE_STRATEGY", "majority_vote")
		os.Setenv("ENSEMBLE_MIN_PROVIDERS", "3")
		os.Setenv("METRICS_ENABLED", "false")
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("RATE_LIMITING_ENABLED", "false")
		os.Setenv("RATE_LIMIT_REQUESTS", "200")
		os.Setenv("PLUGIN_AUTO_RELOAD", "true")
		os.Setenv("PLUGIN_HOT_RELOAD", "true")
		os.Setenv("MAX_CONCURRENT_REQUESTS", "20")
		os.Setenv("REQUEST_TIMEOUT", "120s")

		cfg := Load()

		// Verify overrides
		if cfg.Server.Port != "9090" {
			t.Errorf("Expected Server.Port '9090', got %s", cfg.Server.Port)
		}
		if cfg.Server.APIKey != "test-api-key" {
			t.Errorf("Expected Server.APIKey 'test-api-key', got %s", cfg.Server.APIKey)
		}
		if cfg.Server.JWTSecret != "test-jwt-secret" {
			t.Errorf("Expected Server.JWTSecret 'test-jwt-secret', got %s", cfg.Server.JWTSecret)
		}
		if cfg.Database.Host != "test-db-host" {
			t.Errorf("Expected Database.Host 'test-db-host', got %s", cfg.Database.Host)
		}
		if cfg.Database.Port != "5433" {
			t.Errorf("Expected Database.Port '5433', got %s", cfg.Database.Port)
		}
		if cfg.Database.User != "test-user" {
			t.Errorf("Expected Database.User 'test-user', got %s", cfg.Database.User)
		}
		if cfg.Database.Password != "test-password" {
			t.Errorf("Expected Database.Password 'test-password', got %s", cfg.Database.Password)
		}
		if cfg.Database.Name != "test-db" {
			t.Errorf("Expected Database.Name 'test-db', got %s", cfg.Database.Name)
		}
		if cfg.Redis.Host != "test-redis-host" {
			t.Errorf("Expected Redis.Host 'test-redis-host', got %s", cfg.Redis.Host)
		}
		if cfg.Redis.Port != "6380" {
			t.Errorf("Expected Redis.Port '6380', got %s", cfg.Redis.Port)
		}
		if cfg.Cognee.BaseURL != "http://test-cognee:8000" {
			t.Errorf("Expected Cognee.BaseURL 'http://test-cognee:8000', got %s", cfg.Cognee.BaseURL)
		}
		if cfg.Cognee.APIKey != "test-cognee-key" {
			t.Errorf("Expected Cognee.APIKey 'test-cognee-key', got %s", cfg.Cognee.APIKey)
		}
		if cfg.LLM.DefaultTimeout != 90*time.Second {
			t.Errorf("Expected LLM.DefaultTimeout 90s, got %v", cfg.LLM.DefaultTimeout)
		}
		if cfg.LLM.MaxRetries != 5 {
			t.Errorf("Expected LLM.MaxRetries 5, got %d", cfg.LLM.MaxRetries)
		}
		if cfg.LLM.Ensemble.Strategy != "majority_vote" {
			t.Errorf("Expected LLM.Ensemble.Strategy 'majority_vote', got %s", cfg.LLM.Ensemble.Strategy)
		}
		if cfg.LLM.Ensemble.MinProviders != 3 {
			t.Errorf("Expected LLM.Ensemble.MinProviders 3, got %d", cfg.LLM.Ensemble.MinProviders)
		}
		if cfg.Monitoring.Enabled {
			t.Error("Expected Monitoring.Enabled false")
		}
		if cfg.Monitoring.LogLevel != "debug" {
			t.Errorf("Expected Monitoring.LogLevel 'debug', got %s", cfg.Monitoring.LogLevel)
		}
		if cfg.Security.RateLimiting.Enabled {
			t.Error("Expected Security.RateLimiting.Enabled false")
		}
		if cfg.Security.RateLimiting.Requests != 200 {
			t.Errorf("Expected Security.RateLimiting.Requests 200, got %d", cfg.Security.RateLimiting.Requests)
		}
		if !cfg.Plugins.AutoReload {
			t.Error("Expected Plugins.AutoReload true")
		}
		if !cfg.Plugins.HotReload {
			t.Error("Expected Plugins.HotReload true")
		}
		if cfg.Performance.MaxConcurrentRequests != 20 {
			t.Errorf("Expected Performance.MaxConcurrentRequests 20, got %d", cfg.Performance.MaxConcurrentRequests)
		}
		if cfg.Performance.RequestTimeout != 120*time.Second {
			t.Errorf("Expected Performance.RequestTimeout 120s, got %v", cfg.Performance.RequestTimeout)
		}

		// Clean up
		for key := range originalEnv {
			os.Unsetenv(key)
		}
	})

	t.Run("GetEnvHelpers", func(t *testing.T) {
		// Test getIntEnv
		os.Setenv("TEST_INT", "42")
		if getIntEnv("TEST_INT", 0) != 42 {
			t.Errorf("Expected getIntEnv to return 42, got %d", getIntEnv("TEST_INT", 0))
		}
		if getIntEnv("TEST_INT_MISSING", 99) != 99 {
			t.Errorf("Expected getIntEnv to return default 99, got %d", getIntEnv("TEST_INT_MISSING", 99))
		}
		os.Setenv("TEST_INT_INVALID", "not-a-number")
		if getIntEnv("TEST_INT_INVALID", 100) != 100 {
			t.Errorf("Expected getIntEnv to return default 100 for invalid, got %d", getIntEnv("TEST_INT_INVALID", 100))
		}

		// Test getBoolEnv
		os.Setenv("TEST_BOOL_TRUE", "true")
		if !getBoolEnv("TEST_BOOL_TRUE", false) {
			t.Error("Expected getBoolEnv to return true")
		}
		os.Setenv("TEST_BOOL_FALSE", "false")
		if getBoolEnv("TEST_BOOL_FALSE", true) {
			t.Error("Expected getBoolEnv to return false")
		}
		if !getBoolEnv("TEST_BOOL_MISSING", true) {
			t.Error("Expected getBoolEnv to return default true")
		}
		os.Setenv("TEST_BOOL_INVALID", "not-a-bool")
		if getBoolEnv("TEST_BOOL_INVALID", false) {
			t.Error("Expected getBoolEnv to return default false for invalid")
		}

		// Test getDurationEnv
		os.Setenv("TEST_DURATION", "5m")
		if getDurationEnv("TEST_DURATION", time.Second) != 5*time.Minute {
			t.Errorf("Expected getDurationEnv to return 5m, got %v", getDurationEnv("TEST_DURATION", time.Second))
		}
		if getDurationEnv("TEST_DURATION_MISSING", 10*time.Second) != 10*time.Second {
			t.Errorf("Expected getDurationEnv to return default 10s, got %v", getDurationEnv("TEST_DURATION_MISSING", 10*time.Second))
		}
		os.Setenv("TEST_DURATION_INVALID", "not-a-duration")
		if getDurationEnv("TEST_DURATION_INVALID", time.Hour) != time.Hour {
			t.Errorf("Expected getDurationEnv to return default 1h for invalid, got %v", getDurationEnv("TEST_DURATION_INVALID", time.Hour))
		}

		// Test getFloatEnv
		os.Setenv("TEST_FLOAT", "3.14")
		if getFloatEnv("TEST_FLOAT", 0) != 3.14 {
			t.Errorf("Expected getFloatEnv to return 3.14, got %f", getFloatEnv("TEST_FLOAT", 0))
		}
		if getFloatEnv("TEST_FLOAT_MISSING", 2.71) != 2.71 {
			t.Errorf("Expected getFloatEnv to return default 2.71, got %f", getFloatEnv("TEST_FLOAT_MISSING", 2.71))
		}
		os.Setenv("TEST_FLOAT_INVALID", "not-a-float")
		if getFloatEnv("TEST_FLOAT_INVALID", 1.0) != 1.0 {
			t.Errorf("Expected getFloatEnv to return default 1.0 for invalid, got %f", getFloatEnv("TEST_FLOAT_INVALID", 1.0))
		}

		// Test getEnvSlice
		os.Setenv("TEST_SLICE", "a,b,c,d")
		slice := getEnvSlice("TEST_SLICE", []string{})
		if len(slice) != 4 || slice[0] != "a" || slice[1] != "b" || slice[2] != "c" || slice[3] != "d" {
			t.Errorf("Expected getEnvSlice to return [a b c d], got %v", slice)
		}
		defaultSlice := []string{"default", "values"}
		result := getEnvSlice("TEST_SLICE_MISSING", defaultSlice)
		if len(result) != 2 || result[0] != "default" || result[1] != "values" {
			t.Errorf("Expected getEnvSlice to return default [default values], got %v", result)
		}

		// Clean up
		os.Unsetenv("TEST_INT")
		os.Unsetenv("TEST_INT_INVALID")
		os.Unsetenv("TEST_BOOL_TRUE")
		os.Unsetenv("TEST_BOOL_FALSE")
		os.Unsetenv("TEST_BOOL_INVALID")
		os.Unsetenv("TEST_DURATION")
		os.Unsetenv("TEST_DURATION_INVALID")
		os.Unsetenv("TEST_FLOAT")
		os.Unsetenv("TEST_FLOAT_INVALID")
		os.Unsetenv("TEST_SLICE")
	})
}

func TestConfigValidation(t *testing.T) {
	t.Run("EmptyConfigNotNil", func(t *testing.T) {
		cfg := Load()
		if cfg == nil {
			t.Fatal("Config should not be nil")
		}
	})

	t.Run("ProvidersMapInitialized", func(t *testing.T) {
		cfg := Load()
		if cfg.LLM.Providers == nil {
			t.Fatal("LLM.Providers map should be initialized")
		}
	})

	t.Run("ConfigImmutableAfterLoad", func(t *testing.T) {
		cfg1 := Load()
		cfg2 := Load()

		// Modify cfg1
		cfg1.Server.Port = "9999"

		// cfg2 should not be affected
		if cfg2.Server.Port == "9999" {
			t.Error("Config instances should be independent")
		}
	})
}
