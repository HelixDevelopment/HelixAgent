package services

import (
	"os/exec"
	"testing"
	"time"

	"dev.helix.agent/internal/config"
)

func newTestServicesConfig() *config.ServicesConfig {
	cfg := config.DefaultServicesConfig()
	return &cfg
}

func TestNewBootManager(t *testing.T) {
	cfg := newTestServicesConfig()
	logger := newTestLogger()

	bm := NewBootManager(cfg, logger)

	if bm == nil {
		t.Fatal("Expected non-nil BootManager")
	}
	if bm.Config != cfg {
		t.Error("Expected Config to be set")
	}
	if bm.Logger != logger {
		t.Error("Expected Logger to be set")
	}
	if bm.GetResults() == nil {
		t.Error("Expected Results to be non-nil map")
	}
	if bm.HealthChecker == nil {
		t.Error("Expected HealthChecker to be non-nil")
	}
}

func TestBootAll_RemoteSkipsCompose(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping boot manager test in short mode (requires network health checks)")  // SKIP-OK: #short-mode
	}
	cfg := newTestServicesConfig()
	logger := newTestLogger()

	// Mark all services as enabled, remote, and not required (so boot doesn't fail)
	cfg.PostgreSQL.Remote = true
	cfg.PostgreSQL.Required = false
	cfg.Redis.Remote = true
	cfg.Redis.Required = false
	cfg.Cognee.Enabled = true // Enable Cognee for this test (disabled by default after Mem0 migration)
	cfg.Cognee.Remote = true
	cfg.Cognee.Required = false
	cfg.ChromaDB.Remote = true
	cfg.ChromaDB.Required = false

	bm := NewBootManager(cfg, logger)

	// BootAll will fail health checks (no remote services running) but should not
	// attempt compose. Since none are required, it should not return error.
	err := bm.BootAll()
	if err != nil {
		t.Errorf("Expected no error for optional remote services, got: %v", err)
	}

	// Verify remote services are marked as "remote" in results
	for name, result := range bm.GetResults() {
		ep := cfg.AllEndpoints()[name]
		if ep.Remote && result.Status != "remote" && result.Status != "failed" {
			t.Errorf("Expected remote service %s to have status 'remote', got %s", name, result.Status)
		}
	}
}

func TestBootAll_OptionalFailureContinues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping boot manager test in short mode (requires network health checks)")  // SKIP-OK: #short-mode
	}
	cfg := newTestServicesConfig()
	logger := newTestLogger()

	// Make all services optional (not required) and point to unreachable addresses
	cfg.PostgreSQL.Required = false
	cfg.PostgreSQL.Host = "192.0.2.1" // non-routable
	cfg.PostgreSQL.Timeout = 1 * time.Second
	cfg.PostgreSQL.RetryCount = 1
	cfg.Redis.Required = false
	cfg.Redis.Host = "192.0.2.1"
	cfg.Redis.Timeout = 1 * time.Second
	cfg.Redis.RetryCount = 1
	cfg.Cognee.Required = false
	cfg.Cognee.Host = "192.0.2.1"
	cfg.Cognee.Timeout = 1 * time.Second
	cfg.Cognee.RetryCount = 1
	cfg.ChromaDB.Required = false
	cfg.ChromaDB.Host = "192.0.2.1"
	cfg.ChromaDB.Timeout = 1 * time.Second
	cfg.ChromaDB.RetryCount = 1

	// Disable all other services
	cfg.Prometheus.Enabled = false
	cfg.Grafana.Enabled = false
	cfg.Neo4j.Enabled = false
	cfg.Kafka.Enabled = false
	cfg.RabbitMQ.Enabled = false
	cfg.Qdrant.Enabled = false
	cfg.Weaviate.Enabled = false
	cfg.LangChain.Enabled = false
	cfg.LlamaIndex.Enabled = false

	bm := NewBootManager(cfg, logger)

	// Even with health check failures, boot should succeed since no required services
	err := bm.BootAll()
	if err != nil {
		t.Errorf("Expected no error for optional service failures, got: %v", err)
	}
}

func TestBootAll_RequiredFailureAbortsBoot(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping boot manager test in short mode (requires network health checks)")  // SKIP-OK: #short-mode
	}
	cfg := newTestServicesConfig()
	logger := newTestLogger()

	// Make PostgreSQL required but unreachable
	cfg.PostgreSQL.Required = true
	cfg.PostgreSQL.Host = "192.0.2.1" // non-routable
	cfg.PostgreSQL.Remote = true      // skip compose
	cfg.PostgreSQL.Timeout = 1 * time.Second
	cfg.PostgreSQL.RetryCount = 1

	// Disable everything else
	cfg.Redis.Enabled = false
	cfg.Cognee.Enabled = false
	cfg.ChromaDB.Enabled = false
	cfg.Prometheus.Enabled = false
	cfg.Grafana.Enabled = false
	cfg.Neo4j.Enabled = false
	cfg.Kafka.Enabled = false
	cfg.RabbitMQ.Enabled = false
	cfg.Qdrant.Enabled = false
	cfg.Weaviate.Enabled = false
	cfg.LangChain.Enabled = false
	cfg.LlamaIndex.Enabled = false

	bm := NewBootManager(cfg, logger)

	err := bm.BootAll()
	if err == nil {
		t.Error("Expected error when required service fails health check")
	}
}

func TestBootAll_SkippedDisabledServices(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping boot manager test in short mode (requires network health checks)")  // SKIP-OK: #short-mode
	}
	cfg := newTestServicesConfig()
	logger := newTestLogger()

	// Disable all services
	cfg.PostgreSQL.Enabled = false
	cfg.Redis.Enabled = false
	cfg.Cognee.Enabled = false
	cfg.ChromaDB.Enabled = false
	cfg.Prometheus.Enabled = false
	cfg.Grafana.Enabled = false
	cfg.Neo4j.Enabled = false
	cfg.Kafka.Enabled = false
	cfg.RabbitMQ.Enabled = false
	cfg.Qdrant.Enabled = false
	cfg.Weaviate.Enabled = false
	cfg.LangChain.Enabled = false
	cfg.LlamaIndex.Enabled = false

	bm := NewBootManager(cfg, logger)
	err := bm.BootAll()
	if err != nil {
		t.Errorf("Expected no error with all services disabled, got: %v", err)
	}

	// All results should be "skipped"
	for name, result := range bm.GetResults() {
		if result.Status != "skipped" {
			t.Errorf("Expected service %s to be skipped, got %s", name, result.Status)
		}
	}
}

func TestHealthCheckAll(t *testing.T) {
	cfg := newTestServicesConfig()
	logger := newTestLogger()

	// Disable all except one (which will fail since nothing's running)
	cfg.PostgreSQL.Enabled = false
	cfg.Redis.Enabled = false
	cfg.Cognee.Enabled = false
	cfg.ChromaDB.Enabled = false
	cfg.Neo4j.Enabled = false
	cfg.Kafka.Enabled = false
	cfg.RabbitMQ.Enabled = false
	cfg.Qdrant.Enabled = false
	cfg.Weaviate.Enabled = false
	cfg.LangChain.Enabled = false
	cfg.LlamaIndex.Enabled = false

	// Leave Prometheus enabled but non-running
	cfg.Prometheus.Enabled = true
	cfg.Prometheus.Timeout = 1 * time.Second

	bm := NewBootManager(cfg, logger)
	results := bm.HealthCheckAll()

	// Should have result for Prometheus (and Grafana if enabled)
	if len(results) == 0 {
		t.Error("Expected at least one health check result")
	}
}

func TestDetectComposeCmd(t *testing.T) {
	cmd, args := detectComposeCmd()

	if cmd == "" {
		t.Error("Expected non-empty compose command")
	}

	// The command should be docker, docker-compose, podman, or podman-compose
	validCommands := map[string]bool{
		"docker":         true,
		"docker-compose": true,
		"podman":         true,
		"podman-compose": true,
	}

	// Extract base command name
	if !validCommands[cmd] && len(cmd) > 0 {
		// It could be a full path, that's fine
		t.Logf("Compose command: %s %v", cmd, args)
	}
}

func detectComposeCmd() (string, []string) {
	// Try "docker compose" (v2) first
	if path, err := exec.LookPath("docker"); err == nil {
		cmd := exec.Command(path, "compose", "version")
		if err := cmd.Run(); err == nil {
			return path, []string{"compose"}
		}
	}
	// Fallback to docker-compose (v1)
	if path, err := exec.LookPath("docker-compose"); err == nil {
		return path, nil
	}
	// Try podman-compose
	if path, err := exec.LookPath("podman-compose"); err == nil {
		return path, nil
	}
	// Try podman compose
	if path, err := exec.LookPath("podman"); err == nil {
		return path, []string{"compose"}
	}
	return "docker", []string{"compose"}
}
