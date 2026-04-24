package testutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// Infrastructure availability cache — checked once per test run.
// The map is lazily initialized via sync.Once to avoid eager work
// at package import time.
var (
	infraResults   map[string]bool
	infraResultsMu sync.Once
	infraMu        sync.RWMutex
)

// getInfraResults returns the lazily initialized infrastructure results map.
func getInfraResults() map[string]bool {
	infraResultsMu.Do(func() {
		infraResults = make(map[string]bool)
	})
	return infraResults
}

// InfraConfig holds connection details for test infrastructure.
type InfraConfig struct {
	PostgresHost string
	PostgresPort string
	RedisHost    string
	RedisPort    string
	MockLLMHost  string
	MockLLMPort  string
	ServerHost   string
	ServerPort   string
}

// DefaultInfraConfig returns the default infrastructure configuration
// using environment variables with sensible defaults for the test stack.
//
// Defaults track the canonical 81xx port registry (CONST-027; see
// docs/development/port-registry.md). Prior defaults targeted the
// legacy 7061 / 15432 / 6379 / 18081 ports, which caused every
// server-dependent e2e test to skip against a running binary.
func DefaultInfraConfig() InfraConfig {
	return InfraConfig{
		PostgresHost: envOr("DB_HOST", "localhost"),
		PostgresPort: envOr("DB_PORT", "8109"),        // HELIXAGENT_PORT_POSTGRES_TEST
		RedisHost:    envOr("REDIS_HOST", "localhost"),
		RedisPort:    envOr("REDIS_PORT", "8110"),     // HELIXAGENT_PORT_REDIS_MCP
		MockLLMHost:  envOr("MOCK_LLM_HOST", "localhost"),
		MockLLMPort:  envOr("MOCK_LLM_PORT", "8106"),  // HELIXAGENT_PORT_MOCK_LLM
		ServerHost:   envOr("HELIXAGENT_HOST", "localhost"),
		ServerPort:   envOr("HELIXAGENT_PORT", "8100"), // HELIXAGENT_PORT_HTTP
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// checkTCP checks if a TCP endpoint is reachable within the timeout.
func checkTCP(host, port string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// checkHTTP checks if an HTTP endpoint responds with a non-5xx status.
func checkHTTP(url string, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

// cachedCheck performs a check once and caches the result.
func cachedCheck(key string, checker func() bool) bool {
	results := getInfraResults()

	infraMu.RLock()
	if result, ok := results[key]; ok {
		infraMu.RUnlock()
		return result
	}
	infraMu.RUnlock()

	result := checker()

	infraMu.Lock()
	results[key] = result
	infraMu.Unlock()

	return result
}

// PostgresAvailable returns true if PostgreSQL is reachable.
func PostgresAvailable() bool {
	cfg := DefaultInfraConfig()
	return cachedCheck("postgres", func() bool {
		return checkTCP(cfg.PostgresHost, cfg.PostgresPort, 2*time.Second)
	})
}

// RedisAvailable returns true if Redis is reachable.
func RedisAvailable() bool {
	cfg := DefaultInfraConfig()
	return cachedCheck("redis", func() bool {
		return checkTCP(cfg.RedisHost, cfg.RedisPort, 2*time.Second)
	})
}

// MockLLMAvailable returns true if the Mock LLM server is reachable.
func MockLLMAvailable() bool {
	cfg := DefaultInfraConfig()
	return cachedCheck("mockllm", func() bool {
		return checkTCP(cfg.MockLLMHost, cfg.MockLLMPort, 2*time.Second)
	})
}

// ServerAvailable returns true if the HelixAgent server is reachable.
func ServerAvailable() bool {
	cfg := DefaultInfraConfig()
	return cachedCheck("server", func() bool {
		url := fmt.Sprintf("http://%s:%s/health", cfg.ServerHost, cfg.ServerPort)
		return checkHTTP(url, 3*time.Second)
	})
}

// RequirePostgres skips the test if PostgreSQL is not available.
func RequirePostgres(t *testing.T) {
	t.Helper()
	if !PostgresAvailable() {
		t.Skip("PostgreSQL not available — start with: make test-infra-start")  // SKIP-OK: #legacy-untriaged
	}
}

// RequireRedis skips the test if Redis is not available.
func RequireRedis(t *testing.T) {
	t.Helper()
	if !RedisAvailable() {
		t.Skip("Redis not available — start with: make test-infra-start")  // SKIP-OK: #legacy-untriaged
	}
}

// RequireMockLLM skips the test if the Mock LLM server is not available.
func RequireMockLLM(t *testing.T) {
	t.Helper()
	if !MockLLMAvailable() {
		t.Skip("Mock LLM not available — start with: make test-infra-start")  // SKIP-OK: #legacy-untriaged
	}
}

// RequireServer skips the test if the HelixAgent server is not available.
// Also skips in short mode since server-dependent tests make live API calls
// that may take minutes to complete.
func RequireServer(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping server-dependent test in short mode (requires live HelixAgent server with LLM providers)")  // SKIP-OK: #short-mode
	}
	if !ServerAvailable() {
		t.Skip("HelixAgent server not available — start with: make run")  // SKIP-OK: #legacy-untriaged
	}
}

// RequireInfra skips if any core infrastructure (Postgres + Redis) is unavailable.
func RequireInfra(t *testing.T) {
	t.Helper()
	RequirePostgres(t)
	RequireRedis(t)
}

// RequireFullInfra skips if any infrastructure component is unavailable.
func RequireFullInfra(t *testing.T) {
	t.Helper()
	RequirePostgres(t)
	RequireRedis(t)
	RequireMockLLM(t)
}

// RequireEnv skips if the given environment variable is unset, empty,
// or contains an obvious unsubstituted placeholder like "$ApiKey_…" or
// "<YOUR_KEY>". Treating placeholders as "missing" is important for
// .env files that rely on shell-style interpolation that godotenv does
// not perform — without this guard, every downstream test would hit
// the API with a literal "$VarName" token and fail with a 401 that
// looks like a real regression.
func RequireEnv(t *testing.T, envVar string) {
	t.Helper()
	v := os.Getenv(envVar)
	if v == "" {
		t.Skipf("Environment variable %s not set", envVar)
	}
	if strings.HasPrefix(v, "$") || strings.HasPrefix(v, "<") {
		t.Skipf("Environment variable %s looks like an unsubstituted placeholder (%q)", envVar, v)
	}
}

// RequireAPIKey skips if the API key env var for a provider is not set.
func RequireAPIKey(t *testing.T, provider string) {
	t.Helper()
	envVars := map[string]string{
		"openai":      "OPENAI_API_KEY",
		"anthropic":   "ANTHROPIC_API_KEY",
		"deepseek":    "DEEPSEEK_API_KEY",
		"gemini":      "GEMINI_API_KEY",
		"mistral":     "MISTRAL_API_KEY",
		"groq":        "GROQ_API_KEY",
		"cohere":      "COHERE_API_KEY",
		"xai":         "XAI_API_KEY",
		"together":    "TOGETHER_API_KEY",
		"replicate":   "REPLICATE_API_KEY",
		"fireworks":   "FIREWORKS_API_KEY",
		"cerebras":    "CEREBRAS_API_KEY",
		"ai21":        "AI21_API_KEY",
		"perplexity":  "PERPLEXITY_API_KEY",
		"huggingface": "HUGGINGFACE_API_KEY",
		"chutes":      "CHUTES_API_KEY",
		"openrouter":  "OPENROUTER_API_KEY",
		"zai":         "ZAI_API_KEY",
		"qwen":        "QWEN_API_KEY",
		"nvidia":      "NVIDIA_API_KEY",
		"sambanova":   "SAMBANOVA_API_KEY",
	}
	envVar, ok := envVars[provider]
	if !ok {
		envVar = fmt.Sprintf("%s_API_KEY", provider)
	}
	RequireEnv(t, envVar)
}

// RequireExternalService skips if an external TCP service is not reachable.
func RequireExternalService(t *testing.T, name, host, port string) {
	t.Helper()
	if !cachedCheck("ext:"+name, func() bool {
		return checkTCP(host, port, 3*time.Second)
	}) {
		t.Skipf("External service %s not available at %s:%s", name, host, port)
	}
}

// RequireHTTPEndpoint skips if an HTTP endpoint does not respond.
func RequireHTTPEndpoint(t *testing.T, name, url string) {
	t.Helper()
	if !cachedCheck("http:"+name, func() bool {
		return checkHTTP(url, 3*time.Second)
	}) {
		t.Skipf("HTTP endpoint %s not available at %s", name, url)
	}
}

// TestTimeout returns a context with a standardized test timeout.
func TestTimeout(t *testing.T, d time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx, cancel
}

// ShortTimeout returns a context with a short test timeout (5s).
func ShortTimeout(t *testing.T) (context.Context, context.CancelFunc) {
	return TestTimeout(t, 5*time.Second)
}

// MediumTimeout returns a context with a medium test timeout (30s).
func MediumTimeout(t *testing.T) (context.Context, context.CancelFunc) {
	return TestTimeout(t, 30*time.Second)
}

// LongTimeout returns a context with a long test timeout (2m).
func LongTimeout(t *testing.T) (context.Context, context.CancelFunc) {
	return TestTimeout(t, 2*time.Minute)
}

// ServerURL returns the base URL of the HelixAgent test server.
func ServerURL() string {
	cfg := DefaultInfraConfig()
	return fmt.Sprintf("http://%s:%s", cfg.ServerHost, cfg.ServerPort)
}

// PostgresDSN returns the PostgreSQL connection string for tests.
func PostgresDSN() string {
	cfg := DefaultInfraConfig()
	user := envOr("DB_USER", "helixagent")
	pass := envOr("DB_PASSWORD", "helixagent123")
	name := envOr("DB_NAME", "helixagent_db")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, pass, cfg.PostgresHost, cfg.PostgresPort, name)
}

// RedisAddr returns the Redis address for tests.
func RedisAddr() string {
	cfg := DefaultInfraConfig()
	return net.JoinHostPort(cfg.RedisHost, cfg.RedisPort)
}

// RedisPassword returns the Redis password for tests.
func RedisPassword() string {
	return envOr("REDIS_PASSWORD", "")
}
