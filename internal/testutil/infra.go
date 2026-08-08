package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/ports"
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
//
// HELIXAGENT_URL precedence (M5, 2026-07-28): several test suites
// (tests/unit/cli_agent_legacy/cli_agent_test.go's cliAgentTestBaseURL,
// tests/integration/protocol_comprehensive_integration_test.go's 5+
// call sites) read HELIXAGENT_URL directly as an operator override and use
// it AS-IS as the server base URL — while ServerHost/ServerPort (and
// therefore ServerAvailable / RequireServer's availability GATE, and every
// suite that resolves its base URL via ServerURL()) only ever looked at
// HELIXAGENT_HOST/HELIXAGENT_PORT. An operator who exported ONLY
// HELIXAGENT_URL got silently ignored by the gate and by ServerURL()-based
// suites while the other suites honoured it — a split env-variable
// contract. HELIXAGENT_URL, when set, is now parsed here into ServerHost/
// ServerPort too, so the availability gate probes the SAME host/port every
// suite's base URL resolves to, regardless of which of the two env vars the
// operator set.
func DefaultInfraConfig() InfraConfig {
	serverHost := envOr("HELIXAGENT_HOST", "localhost")
	serverPort := envOr("HELIXAGENT_PORT", "8100") // HELIXAGENT_PORT_HTTP
	if h, p, ok := splitHelixAgentURL(os.Getenv("HELIXAGENT_URL")); ok {
		serverHost, serverPort = h, p
	}

	return InfraConfig{
		PostgresHost: envOr("DB_HOST", "localhost"),
		PostgresPort: envOr("DB_PORT", "8109"), // HELIXAGENT_PORT_POSTGRES_TEST
		// Redis resolves through ports.Redis* so this GATE and the product
		// code it gates agree by construction (§11.4.111 / §11.4.201).
		//
		// A previous revision defaulted to "8110" — HELIXAGENT_PORT_REDIS_MCP.
		// That is a DIFFERENT service: the password-protected MCP-backend
		// Redis, not the canonical application Redis the product dials. The
		// mis-binding was recorded, unremarked, in this package's own test:
		// `assert.Equal(t, "8110", cfg.RedisPort) // HELIXAGENT_PORT_REDIS_MCP`.
		// Because 8110 is up on the dev host while the product's endpoint was
		// not, RequireRedis reported "available" and every test it guarded
		// then failed inside the subject with
		// "dial tcp 127.0.0.1:6379: connect: connection refused" — an absent
		// dependency reported as a product defect.
		RedisHost:   ports.RedisHost(),
		RedisPort:   ports.RedisPortString(),
		MockLLMHost: envOr("MOCK_LLM_HOST", "localhost"),
		MockLLMPort: envOr("MOCK_LLM_PORT", "8106"), // HELIXAGENT_PORT_MOCK_LLM
		ServerHost:  serverHost,
		ServerPort:  serverPort,
	}
}

// splitHelixAgentURL parses rawURL (as given via HELIXAGENT_URL) into a
// host + port pair. rawURL may omit its scheme (e.g. "localhost:9999"); a
// missing "://" is treated as "http://<rawURL>" before parsing. A URL with
// no explicit port defaults to 443 for https and 80 otherwise, matching
// standard HTTP-client behaviour. Returns ok=false for an empty or
// unparsable input, in which case the caller's existing
// HELIXAGENT_HOST/HELIXAGENT_PORT-derived defaults apply unchanged.
func splitHelixAgentURL(rawURL string) (host, port string, ok bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", "", false
	}

	parseable := rawURL
	if !strings.Contains(parseable, "://") {
		parseable = "http://" + parseable
	}

	u, err := url.Parse(parseable)
	if err != nil || u.Hostname() == "" {
		return "", "", false
	}

	host = u.Hostname()
	port = u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return host, port, true
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

// RedisAvailable returns true if Redis is reachable AND usable — that is, if
// a real client with this process's credentials can actually issue commands.
//
// WHY A PROTOCOL HANDSHAKE, NOT A TCP DIAL (§11.4.201)
// This used to call checkTCP, so ANY process holding the port satisfied it.
// That is not a theoretical hole here: the MCP-backend Redis on
// HELIXAGENT_PORT_REDIS_MCP is started with --requirepass and answers a bare
// PING with "-NOAUTH Authentication required." (measured 2026-08-08), and
// helix_agent's own docker-compose.yml starts the canonical Redis with
// --requirepass too. A TCP-open probe calls both of those "available" while
// every command against them fails — so tests did not skip, and then failed
// inside the subject as though the product were broken.
//
// A guard must assert the condition it CLAIMS. This one completes the same
// AUTH+PING exchange a real client would, using the same credentials the
// product would use, and reports available only on a "+PONG".
func RedisAvailable() bool {
	cfg := DefaultInfraConfig()
	return cachedCheck("redis", func() bool {
		return checkRedisPing(cfg.RedisHost, cfg.RedisPort,
			ports.RedisPassword(), 2*time.Second)
	})
}

// checkRedisPing reports whether a usable Redis answers at host:port.
//
// Implemented against the wire protocol with net rather than a client library
// so that this test-infrastructure package stays dependency-free and so the
// probe's failure modes are inspectable here rather than inside a pool
// abstraction. RESP inline commands are used because they are exactly what
// redis-cli sends and every Redis version accepts them.
func checkRedisPing(host, port, password string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	_ = conn.SetDeadline(deadline)

	reply := func(cmd string) (string, bool) {
		if _, werr := conn.Write([]byte(cmd + "\r\n")); werr != nil {
			return "", false
		}
		buf := make([]byte, 256)
		n, rerr := conn.Read(buf)
		if rerr != nil || n == 0 {
			return "", false
		}
		return string(buf[:n]), true
	}

	// AUTH first when a password is configured; a server that does not
	// require one answers "-ERR Client sent AUTH, but no password is set",
	// which is a perfectly usable server, so only an explicit failure to
	// exchange bytes is fatal here — the PING below is the real verdict.
	if password != "" {
		if _, ok := reply("AUTH " + password); !ok {
			return false
		}
	}

	resp, ok := reply("PING")
	if !ok {
		return false
	}
	// "+PONG" is the only accepted answer. "-NOAUTH ...", "-WRONGPASS ...",
	// an HTTP response, or silence from a foreign occupant all fall through.
	return strings.HasPrefix(resp, "+PONG")
}

// MockLLMAvailable returns true if the Mock LLM server is reachable.
func MockLLMAvailable() bool {
	cfg := DefaultInfraConfig()
	return cachedCheck("mockllm", func() bool {
		return checkTCP(cfg.MockLLMHost, cfg.MockLLMPort, 2*time.Second)
	})
}

// ServerAvailable returns true if the HelixAgent server — specifically, NOT
// merely "something listening on that port" — is reachable.
//
// WHY THE IDENTITY CHECK EXISTS (§11.4.111 / §11.4.201)
// This used to call checkHTTP, which accepts ANY status < 500. A DIFFERENT
// service occupying the configured port therefore satisfied it: on the
// development host, llmsverifier legitimately owns :8100 and answers
// `404 page not found`. 404 is < 500, so ServerAvailable returned true,
// RequireServer did NOT skip, and roughly a dozen packages then fired real
// requests at the wrong service and failed on the wrong-shaped replies —
// reported as product failures when nothing was wrong with the product.
//
// A guard must assert the condition it CLAIMS ("the HelixAgent server is
// reachable"), not a proxy that another process can satisfy. So: require a
// 200 AND a HelixAgent-shaped health payload, binding to the service's
// reported identity rather than to a port number alone.
//
// CORRECTION (2026-07-28, §11.4.194 code review): an earlier revision probed
// bare /health and, absent a "service" field, fell back to accepting any
// {"status":"healthy"} body. That fallback was claimed to be closed by
// having router.go emit "service":"helixagent" on /health — it was not: the
// fallback still ACCEPTED a response with no "service" field at all, and
// this repo's OWN mock LLM server (challenges/codebase/mock_server/main.go
// healthHandler) answers /health with EXACTLY {"status":"healthy"} and no
// service field, so pointing HELIXAGENT_HOST/PORT at the mock still
// satisfied this gate. See checkHelixAgentHealth's doc comment for the
// actual closure: probe /v1/health and REQUIRE its "providers" key, which
// the mock cannot produce (it has no /v1/health handler at all).
func ServerAvailable() bool {
	cfg := DefaultInfraConfig()
	return cachedCheck("server", func() bool {
		url := fmt.Sprintf("http://%s:%s/v1/health", cfg.ServerHost, cfg.ServerPort)
		return checkHelixAgentHealth(url, 3*time.Second)
	})
}

// checkHelixAgentHealth reports whether the URL is served by HelixAgent, as
// distinct from any other process that happens to hold the port.
//
// WHY /v1/health + REQUIRED "providers" KEY (§11.4.111 / §11.4.194 / §11.4.201,
// corrected 2026-07-28). A prior revision of this function probed bare
// /health and, when no "service" field was present, fell back to accepting
// any {"status":"healthy"} body. router.go's /health handler DOES emit
// "service":"helixagent" — but this repo's OWN mock LLM server
// (challenges/codebase/mock_server/main.go healthHandler) answers /health
// with EXACTLY {"status":"healthy"} and no service field, and ALSO
// implements /v1/chat/completions with fabricated replies. So a
// HELIXAGENT_HOST/PORT pointed at the mock still satisfied the old fallback
// and let tests run — and PASS — against the mock instead of the real
// server. A prior commit message claimed emitting "service":"helixagent" on
// /health "closes that false-GREEN vector at the source" — that claim was
// WRONG; this function is what actually closes it.
//
// The closure requires a field the mock cannot produce: router.go's
// /v1/health has emitted a "providers" object (total/healthy/unhealthy)
// since the endpoint's original introduction (commit 4c968f22, 2025-12-10 —
// confirmed via `git log -S'"providers": map[string]any{' -- internal/router/
// router.go`, which returns only that one commit, and `git blame
// internal/router/router.go:500`, which still attributes the "providers"
// line to it), long before the "service" field existed (added in commit
// 3d96b4cf, 2026-07-28). Requiring "providers" therefore introduces no
// false-negative against any real HelixAgent server binary in this repo's
// history — every one of them has always emitted it on /v1/health. The mock
// server registers ONLY "/health" and "/v1/chat/completions"
// (challenges/codebase/mock_server/main.go's main(), via net/http's default
// ServeMux) — it has no "/v1/health" handler, so a request there 404s with a
// plain-text "404 page not found" body and is rejected below (not JSON, and
// even if it were, it would still lack "providers").
func checkHelixAgentHealth(url string, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// A healthy HelixAgent answers 200. A foreign occupant typically answers
	// 404/401/403 — all of which the old `< 500` test wrongly accepted.
	if resp.StatusCode != http.StatusOK {
		return false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return false
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		// Not JSON at all (e.g. the plain-text "404 page not found" body) —
		// definitively not HelixAgent's health response.
		return false
	}

	// Mandatory identity gate: only a real HelixAgent /v1/health response
	// carries "providers" (see the doc comment above for why this, unlike
	// bare "status"=="healthy", cannot be satisfied by this repo's own mock
	// LLM server). Reject outright, regardless of status/service, if absent.
	if _, ok := payload["providers"]; !ok {
		return false
	}

	// Prefer an explicit service identifier when the server provides one.
	if svc, ok := payload["service"].(string); ok {
		return strings.EqualFold(svc, "helixagent") || strings.EqualFold(svc, "helix_agent")
	}

	status, _ := payload["status"].(string)
	return strings.EqualFold(status, "healthy")
}

// RequirePostgres skips the test if PostgreSQL is not available.
func RequirePostgres(t *testing.T) {
	t.Helper()
	if !PostgresAvailable() {
		t.Skip("PostgreSQL not available — start with: make test-infra-start") // SKIP-OK: #legacy-untriaged
	}
}

// RequireRedis skips the test if Redis is not available.
func RequireRedis(t *testing.T) {
	t.Helper()
	if !RedisAvailable() {
		t.Skip("Redis not available — start with: make test-infra-start") // SKIP-OK: #legacy-untriaged
	}
}

// RequireMockLLM skips the test if the Mock LLM server is not available.
func RequireMockLLM(t *testing.T) {
	t.Helper()
	if !MockLLMAvailable() {
		t.Skip("Mock LLM not available — start with: make test-infra-start") // SKIP-OK: #legacy-untriaged
	}
}

// RequireServer skips the test if the HelixAgent server is not available.
// Also skips in short mode since server-dependent tests make live API calls
// that may take minutes to complete.
func RequireServer(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping server-dependent test in short mode (requires live HelixAgent server with LLM providers)") // SKIP-OK: #short-mode
	}
	if !ServerAvailable() {
		t.Skip("HelixAgent server not available — start with: make run") // SKIP-OK: #legacy-untriaged
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
//
// HELIXAGENT_URL, when set, is returned AS-IS (trimmed of a trailing slash)
// — the same convention already used by cliAgentTestBaseURL() in
// tests/unit/cli_agent_legacy/cli_agent_test.go and the 5+ call sites in
// tests/integration/protocol_comprehensive_integration_test.go, so every
// suite resolving its base URL through this function agrees with those
// suites byte-for-byte (M5, 2026-07-28). DefaultInfraConfig() independently
// parses HELIXAGENT_URL into ServerHost/ServerPort so the ServerAvailable
// availability gate keeps probing the SAME host/port this URL resolves to.
func ServerURL() string {
	if rawURL := strings.TrimSpace(os.Getenv("HELIXAGENT_URL")); rawURL != "" {
		return strings.TrimRight(rawURL, "/")
	}
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

// RedisPassword returns the Redis password for tests, resolved through the
// same helper the product uses so a test client and the code under test can
// never disagree about credentials.
func RedisPassword() string {
	return ports.RedisPassword()
}
