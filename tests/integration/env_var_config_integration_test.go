//go:build integration
// +build integration

// Package integration provides environment variable config integration tests.
// These tests verify that config.Load() reads expected environment variables
// and applies correct defaults.
package integration

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/config"
)

// TestConfig_LoadDefaults verifies that config.Load() returns sensible
// defaults when no environment variables are set.
func TestConfig_LoadDefaults(t *testing.T) {
	cfg := config.Load()
	require.NotNil(t, cfg)

	assert.Equal(t, "8100", cfg.Server.Port, "default port should be 8100")
	assert.Equal(t, "localhost", cfg.Database.Host, "default DB host should be localhost")
	assert.Equal(t, "5432", cfg.Database.Port, "default DB port should be 5432")
	assert.Equal(t, "helixagent", cfg.Database.User, "default DB user")
	assert.Equal(t, "helixagent_db", cfg.Database.Name, "default DB name")
}

// TestConfig_LoadReadsPortEnv verifies that PORT env var overrides
// the default server port.
func TestConfig_LoadReadsPortEnv(t *testing.T) {
	prev := os.Getenv("PORT")
	os.Setenv("PORT", "9999")
	defer os.Setenv("PORT", prev)

	cfg := config.Load()
	assert.Equal(t, "9999", cfg.Server.Port)
}

// TestConfig_LoadReadsRedisEnv verifies that REDIS_HOST and REDIS_PORT
// env vars are picked up by config loading.
func TestConfig_LoadReadsRedisEnv(t *testing.T) {
	prevHost := os.Getenv("REDIS_HOST")
	prevPort := os.Getenv("REDIS_PORT")
	os.Setenv("REDIS_HOST", "redis.example.com")
	os.Setenv("REDIS_PORT", "16379")
	defer func() {
		os.Setenv("REDIS_HOST", prevHost)
		os.Setenv("REDIS_PORT", prevPort)
	}()

	cfg := config.Load()
	assert.Equal(t, "redis.example.com", cfg.Redis.Host)
	assert.Equal(t, "16379", cfg.Redis.Port)
}

// TestConfig_LoadDatabaseSSLMode verifies that DB_SSLMODE env var
// controls the SSL mode for PostgreSQL connections.
func TestConfig_LoadDatabaseSSLMode(t *testing.T) {
	prev := os.Getenv("DB_SSLMODE")
	os.Setenv("DB_SSLMODE", "require")
	defer os.Setenv("DB_SSLMODE", prev)

	cfg := config.Load()
	assert.Equal(t, "require", cfg.Database.SSLMode)
}

// TestConfig_LoadCogneeDisabledByDefault verifies that Cognee is
// disabled by default since Mem0 replaced it as the primary memory.
func TestConfig_LoadCogneeDisabledByDefault(t *testing.T) {
	// Clear env to test default
	prev := os.Getenv("COGNEE_ENABLED")
	os.Unsetenv("COGNEE_ENABLED")
	defer os.Setenv("COGNEE_ENABLED", prev)

	cfg := config.Load()
	assert.False(t, cfg.Cognee.Enabled, "Cognee should be disabled by default")
}
