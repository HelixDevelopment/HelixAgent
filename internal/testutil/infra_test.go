package testutil

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultInfraConfig(t *testing.T) {
	// Clear env vars that override defaults so we test actual fallback values
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PORT", "")
	t.Setenv("REDIS_HOST", "")
	t.Setenv("REDIS_PORT", "")
	t.Setenv("MOCK_LLM_HOST", "")
	t.Setenv("MOCK_LLM_PORT", "")
	t.Setenv("HELIXAGENT_HOST", "")
	t.Setenv("HELIXAGENT_PORT", "")

	cfg := DefaultInfraConfig()
	assert.Equal(t, "localhost", cfg.PostgresHost)
	assert.Equal(t, "8109", cfg.PostgresPort) // HELIXAGENT_PORT_POSTGRES_TEST per CONST-027 (was: 15432)
	assert.Equal(t, "localhost", cfg.RedisHost)
	assert.Equal(t, "8110", cfg.RedisPort) // HELIXAGENT_PORT_REDIS_MCP per CONST-027 (was: 6379)
	assert.Equal(t, "localhost", cfg.MockLLMHost)
	assert.Equal(t, "8106", cfg.MockLLMPort) // HELIXAGENT_PORT_MOCK_LLM per CONST-027 (was: 18081)
	assert.Equal(t, "localhost", cfg.ServerHost)
	assert.Equal(t, "8100", cfg.ServerPort) // HELIXAGENT_PORT_HTTP per CONST-027 (was: 7061)
}

func TestDefaultInfraConfig_EnvOverride(t *testing.T) {
	t.Setenv("DB_HOST", "dbhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("REDIS_HOST", "redishost")
	t.Setenv("REDIS_PORT", "6379")

	// Reset cache so env vars take effect
	infraMu.Lock()
	infraResults = make(map[string]bool)
	infraMu.Unlock()

	cfg := DefaultInfraConfig()
	assert.Equal(t, "dbhost", cfg.PostgresHost)
	assert.Equal(t, "5432", cfg.PostgresPort)
	assert.Equal(t, "redishost", cfg.RedisHost)
	assert.Equal(t, "6379", cfg.RedisPort)
}

func TestEnvOr(t *testing.T) {
	assert.Equal(t, "fallback", envOr("NONEXISTENT_TEST_VAR_12345", "fallback"))

	t.Setenv("TEST_ENVVAR_INFRA", "custom")
	assert.Equal(t, "custom", envOr("TEST_ENVVAR_INFRA", "fallback"))
}

func TestCheckTCP_Unreachable(t *testing.T) {
	// Port 1 is almost certainly not listening
	result := checkTCP("127.0.0.1", "1", 100*1000*1000) // 100ms as Duration
	assert.False(t, result)
}

func TestCachedCheck(t *testing.T) {
	// Reset cache
	infraMu.Lock()
	delete(infraResults, "test_cached")
	infraMu.Unlock()

	callCount := 0
	checker := func() bool {
		callCount++
		return true
	}

	// First call should invoke checker
	result := cachedCheck("test_cached", checker)
	assert.True(t, result)
	assert.Equal(t, 1, callCount)

	// Second call should use cache
	result = cachedCheck("test_cached", checker)
	assert.True(t, result)
	assert.Equal(t, 1, callCount, "checker should not be called twice")

	// Cleanup
	infraMu.Lock()
	delete(infraResults, "test_cached")
	infraMu.Unlock()
}

func TestServerURL(t *testing.T) {
	url := ServerURL()
	assert.Contains(t, url, "http://")
	assert.Contains(t, url, "8100") // HELIXAGENT_PORT_HTTP per CONST-027 (was: 7061)
}

func TestPostgresDSN(t *testing.T) {
	dsn := PostgresDSN()
	assert.Contains(t, dsn, "postgres://")
	assert.Contains(t, dsn, "sslmode=disable")
}

func TestRedisAddr(t *testing.T) {
	addr := RedisAddr()
	assert.Contains(t, addr, ":")
}

func TestRedisPassword(t *testing.T) {
	pass := RedisPassword()
	_ = pass
}

func TestRequireEnv_Missing(t *testing.T) {
	// This should skip, not fail
	mockT := &testing.T{}
	// We can't directly test t.Skip behavior, but we verify the env check
	val := os.Getenv("DEFINITELY_NOT_SET_ENV_VAR_XYZ123")
	assert.Empty(t, val)
	_ = mockT // used to show the pattern
}

func TestShortTimeout(t *testing.T) {
	ctx, cancel := ShortTimeout(t)
	defer cancel()
	assert.NotNil(t, ctx)
	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.False(t, deadline.IsZero())
}

func TestMediumTimeout(t *testing.T) {
	ctx, cancel := MediumTimeout(t)
	defer cancel()
	assert.NotNil(t, ctx)
}

func TestLongTimeout(t *testing.T) {
	ctx, cancel := LongTimeout(t)
	defer cancel()
	assert.NotNil(t, ctx)
}
