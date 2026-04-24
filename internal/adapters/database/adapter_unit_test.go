package database

import (
	"os"
	"testing"

	"dev.helix.agent/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPostgresConfig_EmptyConfigUsesDefaults(t *testing.T) {
	cfg := &config.Config{}
	result := buildPostgresConfig(cfg)
	require.NotNil(t, result)
	// Defaults from postgres.DefaultConfig()
	assert.Equal(t, "localhost", result.Host)
	assert.Equal(t, 5432, result.Port)
	assert.Equal(t, "helixagent", result.ApplicationName)
}

func TestBuildPostgresConfig_ConfigValuesOverrideDefaults(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "testhost",
			Port:     "5433",
			User:     "testuser",
			Password: "testpass",
			Name:     "testdb",
			SSLMode:  "require",
		},
	}
	result := buildPostgresConfig(cfg)
	require.NotNil(t, result)
	assert.Equal(t, "testhost", result.Host)
	assert.Equal(t, 5433, result.Port)
	assert.Equal(t, "testuser", result.User)
	assert.Equal(t, "testpass", result.Password)
	assert.Equal(t, "testdb", result.DBName)
	assert.Equal(t, "require", result.SSLMode)
}

func TestBuildPostgresConfig_EnvVarsOverrideEmptyConfig(t *testing.T) {
	// Set environment variables
	os.Setenv("DB_HOST", "envhost")
	os.Setenv("DB_PORT", "6432")
	os.Setenv("DB_USER", "envuser")
	os.Setenv("DB_PASSWORD", "envpass")
	os.Setenv("DB_NAME", "envdb")
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
	}()

	cfg := &config.Config{} // no values set
	result := buildPostgresConfig(cfg)
	require.NotNil(t, result)
	assert.Equal(t, "envhost", result.Host)
	assert.Equal(t, 6432, result.Port)
	assert.Equal(t, "envuser", result.User)
	assert.Equal(t, "envpass", result.Password)
	assert.Equal(t, "envdb", result.DBName)
}

func TestBuildPostgresConfig_ConfigOverridesEnvVars(t *testing.T) {
	os.Setenv("DB_HOST", "envhost")
	os.Setenv("DB_PORT", "6432")
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
	}()

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host: "confighost",
			Port: "7432",
		},
	}
	result := buildPostgresConfig(cfg)
	require.NotNil(t, result)
	assert.Equal(t, "confighost", result.Host)
	assert.Equal(t, 7432, result.Port)
}

func TestBuildPostgresConfig_InvalidPortUsesDefault(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Port: "not-a-number",
		},
	}
	result := buildPostgresConfig(cfg)
	require.NotNil(t, result)
	// Should fall back to default port 5432
	assert.Equal(t, 5432, result.Port)
}

func TestBuildPostgresConfig_InvalidPortEnvVarUsesDefault(t *testing.T) {
	os.Setenv("DB_PORT", "not-a-number")
	defer os.Unsetenv("DB_PORT")

	cfg := &config.Config{}
	result := buildPostgresConfig(cfg)
	require.NotNil(t, result)
	assert.Equal(t, 5432, result.Port)
}

func TestBuildPostgresConfig_ApplicationNameAlwaysSet(t *testing.T) {
	cfg := &config.Config{}
	result := buildPostgresConfig(cfg)
	require.NotNil(t, result)
	assert.Equal(t, "helixagent", result.ApplicationName)
}

func TestNewClientWithFallback_ConnectionFailureReturnsError(t *testing.T) {
	// Use invalid host/port to cause connection failure
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host-that-does-not-exist",
			Port:     "1",
			User:     "none",
			Password: "none",
			Name:     "none",
		},
	}
	client, err := NewClientWithFallback(cfg)
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestNewClientWithFallback_ConnectionSuccessReturnsClient(t *testing.T) {
	// This test requires a real PostgreSQL instance, skip if not available
	if os.Getenv("DB_HOST") == "" {
		t.Skip("DB_HOST not set, skipping integration test")  // SKIP-OK: #integration-mode-only
	}
	cfg := &config.Config{}
	client, err := NewClientWithFallback(cfg)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	defer client.Close()
	assert.NotNil(t, client)
	assert.NoError(t, client.Ping())
}
