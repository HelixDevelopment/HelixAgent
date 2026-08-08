package config

import (
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/ports"
)

func TestDefaultServicesConfig(t *testing.T) {
	// Save and restore CONTAINERS_REMOTE_ENABLED
	originalEnv := os.Getenv("CONTAINERS_REMOTE_ENABLED")
	defer os.Setenv("CONTAINERS_REMOTE_ENABLED", originalEnv)

	// For "default" tests, explicitly set to false to override any .env file
	os.Setenv("CONTAINERS_REMOTE_ENABLED", "false")

	cfg := DefaultServicesConfig()

	// All core services should be present with correct defaults
	t.Run("PostgreSQL defaults", func(t *testing.T) {
		if cfg.PostgreSQL.Host != "localhost" {
			t.Errorf("Expected PostgreSQL host 'localhost', got %s", cfg.PostgreSQL.Host)
		}
		if cfg.PostgreSQL.Port != "5432" {
			t.Errorf("Expected PostgreSQL port '5432', got %s", cfg.PostgreSQL.Port)
		}
		if !cfg.PostgreSQL.Enabled {
			t.Error("Expected PostgreSQL to be enabled")
		}
		if !cfg.PostgreSQL.Required {
			t.Error("Expected PostgreSQL to be required")
		}
		if cfg.PostgreSQL.Remote {
			t.Error("Expected PostgreSQL to not be remote by default")
		}
		if cfg.PostgreSQL.HealthType != "pgx" {
			t.Errorf("Expected PostgreSQL health type 'pgx', got %s", cfg.PostgreSQL.HealthType)
		}
		if cfg.PostgreSQL.RetryCount != 6 {
			t.Errorf("Expected PostgreSQL retry count 6, got %d", cfg.PostgreSQL.RetryCount)
		}
		if cfg.PostgreSQL.ServiceName != "postgres" {
			t.Errorf("Expected PostgreSQL service name 'postgres', got %s", cfg.PostgreSQL.ServiceName)
		}
	})

	t.Run("Redis defaults", func(t *testing.T) {
		if cfg.Redis.Host != "localhost" {
			t.Errorf("Expected Redis host 'localhost', got %s", cfg.Redis.Host)
		}
		// Reconciled 2026-08-09 (§11.4.120) — see config_test.go: the
		// endpoint descriptor now resolves through ports.RedisDefault
		// instead of the superseded 6379 literal.
		if wantRedisPort := ports.RedisPortString(); cfg.Redis.Port != wantRedisPort {
			t.Errorf("Expected Redis port %q (ports.RedisDefault), got %s",
				wantRedisPort, cfg.Redis.Port)
		}
		if !cfg.Redis.Enabled {
			t.Error("Expected Redis to be enabled")
		}
		if !cfg.Redis.Required {
			t.Error("Expected Redis to be required")
		}
		if cfg.Redis.HealthType != "redis" {
			t.Errorf("Expected Redis health type 'redis', got %s", cfg.Redis.HealthType)
		}
	})

	t.Run("Cognee defaults", func(t *testing.T) {
		if cfg.Cognee.Port != "8000" {
			t.Errorf("Expected Cognee port '8000', got %s", cfg.Cognee.Port)
		}
		// Cognee is no longer required - disabled by default for Mem0 migration
		if cfg.Cognee.Required {
			t.Error("Expected Cognee to NOT be required (disabled for Mem0)")
		}
		if cfg.Cognee.Enabled {
			t.Error("Expected Cognee to be disabled by default (replaced by Mem0)")
		}
		if cfg.Cognee.HealthType != "http" {
			t.Errorf("Expected Cognee health type 'http', got %s", cfg.Cognee.HealthType)
		}
		if cfg.Cognee.HealthPath != "/" {
			t.Errorf("Expected Cognee health path '/', got %s", cfg.Cognee.HealthPath)
		}
	})

	t.Run("ChromaDB defaults", func(t *testing.T) {
		if cfg.ChromaDB.Port != "8001" {
			t.Errorf("Expected ChromaDB port '8001', got %s", cfg.ChromaDB.Port)
		}
		if !cfg.ChromaDB.Required {
			t.Error("Expected ChromaDB to be required")
		}
		if cfg.ChromaDB.HealthPath != "/api/v2/heartbeat" {
			t.Errorf("Expected ChromaDB health path '/api/v2/heartbeat', got %s", cfg.ChromaDB.HealthPath)
		}
	})

	t.Run("Optional services disabled by default", func(t *testing.T) {
		optionalServices := []struct {
			name string
			ep   ServiceEndpoint
		}{
			{"Prometheus", cfg.Prometheus},
			{"Grafana", cfg.Grafana},
			{"Neo4j", cfg.Neo4j},
			{"Kafka", cfg.Kafka},
			{"RabbitMQ", cfg.RabbitMQ},
			{"Qdrant", cfg.Qdrant},
			{"Weaviate", cfg.Weaviate},
			{"LangChain", cfg.LangChain},
			{"LlamaIndex", cfg.LlamaIndex},
		}
		for _, svc := range optionalServices {
			if svc.ep.Enabled {
				t.Errorf("Expected %s to be disabled by default", svc.name)
			}
			if svc.ep.Required {
				t.Errorf("Expected %s to not be required", svc.name)
			}
		}
	})

	t.Run("MCPServers initialized as empty map", func(t *testing.T) {
		if cfg.MCPServers == nil {
			t.Error("Expected MCPServers to be non-nil map")
		}
		if len(cfg.MCPServers) != 0 {
			t.Errorf("Expected MCPServers to be empty, got %d entries", len(cfg.MCPServers))
		}
	})
}

func TestEnvironmentOverrides(t *testing.T) {
	// Save and restore env
	envVars := []string{
		"SVC_POSTGRESQL_HOST", "SVC_POSTGRESQL_PORT", "SVC_POSTGRESQL_REMOTE",
		"SVC_POSTGRESQL_ENABLED", "SVC_POSTGRESQL_REQUIRED",
		"SVC_REDIS_HOST", "SVC_REDIS_PORT", "SVC_REDIS_TIMEOUT",
		"SVC_COGNEE_URL", "SVC_COGNEE_HEALTH_PATH", "SVC_COGNEE_HEALTH_TYPE",
		"SVC_CHROMADB_RETRY_COUNT",
	}
	origValues := make(map[string]string)
	for _, k := range envVars {
		origValues[k] = os.Getenv(k)
		_ = os.Unsetenv(k)
	}
	defer func() {
		for k, v := range origValues {
			if v != "" {
				_ = os.Setenv(k, v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}()

	t.Run("Host and port override", func(t *testing.T) {
		_ = os.Setenv("SVC_POSTGRESQL_HOST", "db.remote.example.com")
		_ = os.Setenv("SVC_POSTGRESQL_PORT", "15432")
		defer func() { _ = os.Unsetenv("SVC_POSTGRESQL_HOST") }()
		defer func() { _ = os.Unsetenv("SVC_POSTGRESQL_PORT") }()

		cfg := DefaultServicesConfig()
		LoadServicesFromEnv(&cfg)

		if cfg.PostgreSQL.Host != "db.remote.example.com" {
			t.Errorf("Expected PostgreSQL host override, got %s", cfg.PostgreSQL.Host)
		}
		if cfg.PostgreSQL.Port != "15432" {
			t.Errorf("Expected PostgreSQL port override, got %s", cfg.PostgreSQL.Port)
		}
	})

	t.Run("Remote flag override", func(t *testing.T) {
		_ = os.Setenv("SVC_POSTGRESQL_REMOTE", "true")
		defer func() { _ = os.Unsetenv("SVC_POSTGRESQL_REMOTE") }()

		cfg := DefaultServicesConfig()
		LoadServicesFromEnv(&cfg)

		if !cfg.PostgreSQL.Remote {
			t.Error("Expected PostgreSQL remote flag to be true")
		}
	})

	t.Run("Enabled/Required override", func(t *testing.T) {
		_ = os.Setenv("SVC_POSTGRESQL_ENABLED", "false")
		_ = os.Setenv("SVC_POSTGRESQL_REQUIRED", "false")
		defer func() { _ = os.Unsetenv("SVC_POSTGRESQL_ENABLED") }()
		defer func() { _ = os.Unsetenv("SVC_POSTGRESQL_REQUIRED") }()

		cfg := DefaultServicesConfig()
		LoadServicesFromEnv(&cfg)

		if cfg.PostgreSQL.Enabled {
			t.Error("Expected PostgreSQL enabled to be false")
		}
		if cfg.PostgreSQL.Required {
			t.Error("Expected PostgreSQL required to be false")
		}
	})

	t.Run("URL override", func(t *testing.T) {
		_ = os.Setenv("SVC_COGNEE_URL", "https://cognee.remote.example.com")
		defer func() { _ = os.Unsetenv("SVC_COGNEE_URL") }()

		cfg := DefaultServicesConfig()
		LoadServicesFromEnv(&cfg)

		if cfg.Cognee.URL != "https://cognee.remote.example.com" {
			t.Errorf("Expected Cognee URL override, got %s", cfg.Cognee.URL)
		}
	})

	t.Run("Timeout override", func(t *testing.T) {
		_ = os.Setenv("SVC_REDIS_TIMEOUT", "30s")
		defer func() { _ = os.Unsetenv("SVC_REDIS_TIMEOUT") }()

		cfg := DefaultServicesConfig()
		LoadServicesFromEnv(&cfg)

		if cfg.Redis.Timeout != 30*time.Second {
			t.Errorf("Expected Redis timeout 30s, got %v", cfg.Redis.Timeout)
		}
	})

	t.Run("RetryCount override", func(t *testing.T) {
		_ = os.Setenv("SVC_CHROMADB_RETRY_COUNT", "10")
		defer func() { _ = os.Unsetenv("SVC_CHROMADB_RETRY_COUNT") }()

		cfg := DefaultServicesConfig()
		LoadServicesFromEnv(&cfg)

		if cfg.ChromaDB.RetryCount != 10 {
			t.Errorf("Expected ChromaDB retry count 10, got %d", cfg.ChromaDB.RetryCount)
		}
	})

	t.Run("HealthPath and HealthType override", func(t *testing.T) {
		_ = os.Setenv("SVC_COGNEE_HEALTH_PATH", "/health")
		_ = os.Setenv("SVC_COGNEE_HEALTH_TYPE", "tcp")
		defer func() { _ = os.Unsetenv("SVC_COGNEE_HEALTH_PATH") }()
		defer func() { _ = os.Unsetenv("SVC_COGNEE_HEALTH_TYPE") }()

		cfg := DefaultServicesConfig()
		LoadServicesFromEnv(&cfg)

		if cfg.Cognee.HealthPath != "/health" {
			t.Errorf("Expected Cognee health path '/health', got %s", cfg.Cognee.HealthPath)
		}
		if cfg.Cognee.HealthType != "tcp" {
			t.Errorf("Expected Cognee health type 'tcp', got %s", cfg.Cognee.HealthType)
		}
	})
}

func TestRequiredEndpoints(t *testing.T) {
	cfg := DefaultServicesConfig()
	required := cfg.RequiredEndpoints()

	// Should contain the 3 required services (Cognee removed - replaced by Mem0)
	expectedRequired := []string{"postgresql", "redis", "chromadb"}
	for _, name := range expectedRequired {
		if _, ok := required[name]; !ok {
			t.Errorf("Expected %s in required endpoints", name)
		}
	}

	// Cognee should NOT be in required endpoints (disabled for Mem0 migration)
	if _, ok := required["cognee"]; ok {
		t.Error("Cognee should NOT be in required endpoints (disabled for Mem0)")
	}

	// Should NOT contain optional services
	optionalNames := []string{"prometheus", "grafana", "neo4j", "kafka", "rabbitmq", "qdrant", "weaviate", "langchain", "llamaindex"}
	for _, name := range optionalNames {
		if _, ok := required[name]; ok {
			t.Errorf("Did not expect %s in required endpoints", name)
		}
	}
}

func TestAllEndpoints(t *testing.T) {
	cfg := DefaultServicesConfig()
	all := cfg.AllEndpoints()

	// Should have 18 core services (includes zookeeper, clickhouse, minio, spark_master, spark_worker)
	expectedCount := 18
	if len(all) != expectedCount {
		t.Errorf("Expected %d endpoints, got %d", expectedCount, len(all))
	}

	// Verify all names present
	expectedNames := []string{
		"postgresql", "redis", "cognee", "chromadb",
		"prometheus", "grafana", "neo4j", "kafka", "zookeeper", "rabbitmq",
		"qdrant", "weaviate", "langchain", "llamaindex",
		"clickhouse", "minio", "spark_master", "spark_worker",
	}
	for _, name := range expectedNames {
		if _, ok := all[name]; !ok {
			t.Errorf("Expected %s in all endpoints", name)
		}
	}
}

func TestResolvedURL(t *testing.T) {
	t.Run("URL field takes precedence", func(t *testing.T) {
		ep := ServiceEndpoint{
			Host: "localhost",
			Port: "5432",
			URL:  "postgres://custom:5432/db",
		}
		if ep.ResolvedURL() != "postgres://custom:5432/db" {
			t.Errorf("Expected URL field to take precedence, got %s", ep.ResolvedURL())
		}
	})

	t.Run("Host:Port construction", func(t *testing.T) {
		ep := ServiceEndpoint{
			Host: "localhost",
			Port: "5432",
		}
		if ep.ResolvedURL() != "localhost:5432" {
			t.Errorf("Expected 'localhost:5432', got %s", ep.ResolvedURL())
		}
	})

	t.Run("Host only (no port)", func(t *testing.T) {
		ep := ServiceEndpoint{
			Host: "localhost",
		}
		if ep.ResolvedURL() != "localhost" {
			t.Errorf("Expected 'localhost', got %s", ep.ResolvedURL())
		}
	})

	t.Run("Empty returns empty", func(t *testing.T) {
		ep := ServiceEndpoint{}
		if ep.ResolvedURL() != "" {
			t.Errorf("Expected empty string, got %s", ep.ResolvedURL())
		}
	})
}

func TestRemoteFlag(t *testing.T) {
	// Save and restore CONTAINERS_REMOTE_ENABLED
	originalEnv := os.Getenv("CONTAINERS_REMOTE_ENABLED")
	defer os.Setenv("CONTAINERS_REMOTE_ENABLED", originalEnv)

	t.Run("Default is not remote", func(t *testing.T) {
		// Explicitly set to false to override any .env file
		os.Setenv("CONTAINERS_REMOTE_ENABLED", "false")
		cfg := DefaultServicesConfig()
		if cfg.PostgreSQL.Remote {
			t.Error("Expected PostgreSQL default to not be remote")
		}
		if cfg.Redis.Remote {
			t.Error("Expected Redis default to not be remote")
		}
	})

	t.Run("Remote disables auto-start expectation", func(t *testing.T) {
		cfg := DefaultServicesConfig()
		cfg.PostgreSQL.Remote = true
		cfg.PostgreSQL.Host = "db.remote.com"

		// Remote services should still be in AllEndpoints
		all := cfg.AllEndpoints()
		pgEp := all["postgresql"]
		if !pgEp.Remote {
			t.Error("Expected remote flag to be preserved in AllEndpoints")
		}

		// Remote required services should still be in RequiredEndpoints
		required := cfg.RequiredEndpoints()
		if _, ok := required["postgresql"]; !ok {
			t.Error("Expected remote PostgreSQL to still be in required endpoints")
		}
	})
}

func TestServicesConfigInLoad(t *testing.T) {
	// Save and restore env
	envKeys := []string{
		"SVC_POSTGRESQL_HOST", "SVC_POSTGRESQL_ENABLED", "SVC_POSTGRESQL_PORT",
		"SVC_REDIS_REMOTE", "SVC_REDIS_ENABLED",
		"PORT", "DB_HOST", "REDIS_HOST", "GIN_MODE",
	}
	orig := make(map[string]string)
	for _, k := range envKeys {
		orig[k] = os.Getenv(k)
		_ = os.Unsetenv(k)
	}
	defer func() {
		for k, v := range orig {
			if v != "" {
				_ = os.Setenv(k, v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}()

	cfg := Load()

	// Verify Services field is populated
	if cfg.Services.PostgreSQL.Host != "localhost" {
		t.Errorf("Expected Services.PostgreSQL.Host 'localhost', got %s", cfg.Services.PostgreSQL.Host)
	}
	if !cfg.Services.PostgreSQL.Enabled {
		t.Error("Expected Services.PostgreSQL.Enabled to be true")
	}
	if cfg.Services.MCPServers == nil {
		t.Error("Expected Services.MCPServers to be non-nil")
	}
}
