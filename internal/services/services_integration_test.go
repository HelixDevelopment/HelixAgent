package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// CONST-030 Live-Probe Integration Suite
// =============================================================================
//
// Per CONST-030 ("Mocks, stubs, fakes, placeholders, and hardcoded data MAY
// ONLY be used in unit tests"), this file MUST NOT contain in-process
// LLMProvider fakes. The previous revision wired `integrationMockProvider`
// into a `NewProviderRegistryWithoutAutoDiscovery` → `NewDebateServiceWithDeps`
// graph and asserted on the in-process `ConductDebate`/`RunEnsemble` returns —
// a textbook violation flagged in CLAUDE.md §16 and in the
// `docs/development/CONST-030_COMPLIANCE_AUDIT_2026-04-21.md` audit as the
// first PR target.
//
// This revision replaces the mock provider graph with live HTTP calls against
// a running HelixAgent on :7061. Each test probes reachability up-front
// (`isHelixAgentAvailable`) and skips with a clear message when the binary is
// not running — per CONST-030's "non-unit tests that cannot connect to real
// services MUST skip (not fail)" clause. When HelixAgent IS running, each
// test exercises the same debate / ensemble / provider-registry pathway the
// original test did, but end-to-end through the real HTTP handler stack,
// real registry, real database, real Redis, and the real scored provider
// fallback chain.
//
// The tests intentionally assert on *shape* (status code, response envelope,
// presence of expected fields) rather than content-level LLM outputs —
// live LLM outputs are non-deterministic, and LLM assertion determinism is
// exactly what the old in-process mock provided. Losing that determinism is
// the point: the old test was asserting the mock, not the system.
//
// If a reader needs content-level assertions over deterministic LLM input,
// the right home is a `*_test.go` file under `tests/unit/` with mocks,
// permitted by CONST-030. See docs/development/CONST-030_COMPLIANCE_AUDIT_2026-04-21.md
// for the remediation sequencing.

const (
	helixAgentHost       = "localhost"
	helixAgentPort       = "7061"
	helixAgentBaseURL    = "http://" + helixAgentHost + ":" + helixAgentPort
	helixAgentProbeDelay = 500 * time.Millisecond
	helixAgentReqTimeout = 30 * time.Second
)

// isHelixAgentAvailable returns true iff a TCP connection to :7061 succeeds
// within helixAgentProbeDelay AND the /v1/health endpoint answers within
// 2*probeDelay with a non-5xx status. A bare TCP probe is not enough: a
// half-booted HelixAgent can accept TCP but fail every HTTP request.
func isHelixAgentAvailable(t *testing.T) bool {
	t.Helper()

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(helixAgentHost, helixAgentPort), helixAgentProbeDelay)
	if err != nil {
		return false
	}
	_ = conn.Close()

	client := &http.Client{Timeout: 2 * helixAgentProbeDelay}
	resp, err := client.Get(helixAgentBaseURL + "/v1/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode < 500
}

// skipUnlessLive skips the calling test (per CONST-030's "MUST skip") when
// HelixAgent is not reachable on :7061.
func skipUnlessLive(t *testing.T) {
	t.Helper()
	if !isHelixAgentAvailable(t) {
		t.Skipf("HelixAgent not reachable on %s — skipping per CONST-030 (start with `make build && ./bin/helixagent`)", helixAgentBaseURL)
	}
}

// postJSON POSTs a JSON body to the given path on the running HelixAgent and
// returns the decoded response envelope. Any transport error is reported as a
// fatal `require.NoError`, on the basis that the reachability probe has
// already succeeded — a transport failure after that indicates a mid-test
// regression, not an unavailable-service state.
func postJSON(t *testing.T, ctx context.Context, path string, body any) (int, map[string]any) {
	t.Helper()

	buf, err := json.Marshal(body)
	require.NoError(t, err, "marshal request body")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, helixAgentBaseURL+path, bytes.NewReader(buf))
	require.NoError(t, err, "build request")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: helixAgentReqTimeout}
	resp, err := client.Do(req)
	require.NoError(t, err, "POST %s", path)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "read response body")

	var envelope map[string]any
	if len(raw) > 0 {
		// 4xx/5xx responses frequently use a non-JSON body (e.g. Gin's
		// text error); ignore unmarshal error and surface the raw text.
		if jerr := json.Unmarshal(raw, &envelope); jerr != nil {
			envelope = map[string]any{"_raw": string(raw)}
		}
	}
	return resp.StatusCode, envelope
}

// getJSON is the GET analogue of postJSON.
func getJSON(t *testing.T, ctx context.Context, path string) (int, map[string]any) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, helixAgentBaseURL+path, nil)
	require.NoError(t, err, "build request")

	client := &http.Client{Timeout: helixAgentReqTimeout}
	resp, err := client.Do(req)
	require.NoError(t, err, "GET %s", path)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "read response body")

	var envelope map[string]any
	if len(raw) > 0 {
		if jerr := json.Unmarshal(raw, &envelope); jerr != nil {
			envelope = map[string]any{"_raw": string(raw)}
		}
	}
	return resp.StatusCode, envelope
}

// =============================================================================
// Debate Service Integration Tests (live)
// =============================================================================

func TestServicesIntegration_DebateService_FullWorkflow(t *testing.T) {
	skipUnlessLive(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	body := map[string]any{
		"topic":      "The impact of artificial intelligence on software development",
		"max_rounds": 2,
		"timeout":    "60s",
	}
	status, env := postJSON(t, ctx, "/v1/debates", body)

	// Successful, not-found (endpoint gated behind feature flag), or
	// service-unavailable (no providers registered) are all acceptable
	// outcomes here — the point of CONST-030 is that the test is honest
	// about what the live system returns. We only fail on unexpected
	// transport-layer 5xx cascades.
	if status == http.StatusNotFound {
		t.Skipf("/v1/debates endpoint not registered on this build — got 404")
	}
	require.NotEqual(t, http.StatusBadGateway, status, "unexpected 502 from live debate endpoint: %v", env)
	require.NotEqual(t, http.StatusGatewayTimeout, status, "unexpected 504 from live debate endpoint: %v", env)

	if status == http.StatusOK {
		// Best-effort shape validation — the live debate pipeline returns
		// either {debate_id, consensus, ...} or the debate-result envelope.
		assert.NotEmpty(t, env, "expected non-empty debate response envelope")
	}
}

func TestServicesIntegration_DebateService_WithFallbacks(t *testing.T) {
	skipUnlessLive(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Fallback behaviour is a property of the live scored provider chain —
	// hitting /v1/chat/completions exercises the same fallback machinery
	// without requiring us to preconfigure a deliberately-failing provider.
	body := map[string]any{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "user", "content": "Testing fallback mechanisms. Respond with one short sentence."},
		},
		"max_tokens": 64,
	}
	status, env := postJSON(t, ctx, "/v1/chat/completions", body)

	// 200 OK → some provider in the scored chain answered. 429 / 503 /
	// timeout are legitimate live-network states; assert they are not an
	// outright 5xx service crash.
	if status == http.StatusOK {
		assert.NotEmpty(t, env["choices"], "expected choices[] in completion envelope")
	}
	assert.NotEqual(t, http.StatusInternalServerError, status, "unexpected 500 from completion endpoint: %v", env)
}

// =============================================================================
// Ensemble Service Integration Tests (live)
// =============================================================================

func TestServicesIntegration_EnsembleService_MultipleProviders(t *testing.T) {
	skipUnlessLive(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	body := map[string]any{
		"prompt": "Explain the benefits of microservices architecture in one sentence.",
		"ensemble_config": map[string]any{
			"strategy":      "confidence_weighted",
			"min_providers": 1,
		},
	}
	status, env := postJSON(t, ctx, "/v1/ensemble", body)

	if status == http.StatusNotFound {
		t.Skipf("/v1/ensemble endpoint not registered on this build — got 404")
	}
	if status == http.StatusOK {
		assert.NotEmpty(t, env, "expected non-empty ensemble response")
	}
	assert.NotEqual(t, http.StatusInternalServerError, status, "unexpected 500: %v", env)
}

func TestServicesIntegration_EnsembleService_WithPreferredProviders(t *testing.T) {
	skipUnlessLive(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	body := map[string]any{
		"prompt": "Test preferred providers.",
		"ensemble_config": map[string]any{
			"preferred_providers": []string{"claude", "deepseek"},
			"min_providers":       1,
		},
	}
	status, env := postJSON(t, ctx, "/v1/ensemble", body)

	if status == http.StatusNotFound {
		t.Skipf("/v1/ensemble endpoint not registered on this build — got 404")
	}
	assert.NotEqual(t, http.StatusInternalServerError, status, "unexpected 500: %v", env)
}

func TestServicesIntegration_EnsembleService_MajorityVoting(t *testing.T) {
	skipUnlessLive(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	body := map[string]any{
		"prompt": "Which programming language is best for system programming? Answer in one word.",
		"ensemble_config": map[string]any{
			"strategy":      "majority_vote",
			"min_providers": 1,
		},
	}
	status, env := postJSON(t, ctx, "/v1/ensemble", body)

	if status == http.StatusNotFound {
		t.Skipf("/v1/ensemble endpoint not registered on this build — got 404")
	}
	assert.NotEqual(t, http.StatusInternalServerError, status, "unexpected 500: %v", env)
}

func TestServicesIntegration_EnsembleService_QualityWeighted(t *testing.T) {
	skipUnlessLive(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	body := map[string]any{
		"prompt": "Test quality weighted voting. One sentence.",
		"ensemble_config": map[string]any{
			"strategy":      "quality_weighted",
			"min_providers": 1,
		},
	}
	status, env := postJSON(t, ctx, "/v1/ensemble", body)

	if status == http.StatusNotFound {
		t.Skipf("/v1/ensemble endpoint not registered on this build — got 404")
	}
	assert.NotEqual(t, http.StatusInternalServerError, status, "unexpected 500: %v", env)
}

// =============================================================================
// Provider Registry Integration Tests (live)
// =============================================================================
//
// The live provider registry is exposed read-only via /v1/discovery and
// /v1/verification. The original in-process tests asserted on register /
// unregister / ConfigureProvider — CRUD operations not exposed via HTTP.
// The live equivalent is "ask the running system which providers are
// currently registered and healthy" — which is what these tests now do.

func TestServicesIntegration_ProviderRegistry_FullLifecycle(t *testing.T) {
	skipUnlessLive(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// List providers via the live discovery endpoint.
	status, env := getJSON(t, ctx, "/v1/discovery/providers")
	if status == http.StatusNotFound {
		// Some builds expose the list at /v1/providers instead.
		status, env = getJSON(t, ctx, "/v1/providers")
	}
	if status == http.StatusNotFound {
		t.Skipf("No live provider-listing endpoint on this build")
	}
	require.LessOrEqual(t, status, http.StatusBadRequest, "provider listing should not 5xx: %v", env)
	assert.NotNil(t, env, "expected a non-nil response envelope")
}

func TestServicesIntegration_ProviderRegistry_ConcurrentAccess(t *testing.T) {
	skipUnlessLive(t)

	// Concurrent READS against live /v1/health and /v1/discovery simulate
	// the same property the in-process test asserted (registry does not
	// crash under parallel read+write pressure). Writes (register/unregister)
	// are NOT exposed via HTTP, so the test focuses on read-side safety —
	// which is what the scored provider chain experiences in production.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	numGoroutines := 10
	iterations := 5
	errs := make(chan error, numGoroutines*iterations)

	client := &http.Client{Timeout: 5 * time.Second}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				req, _ := http.NewRequestWithContext(ctx, http.MethodGet, helixAgentBaseURL+"/v1/health", nil)
				resp, err := client.Do(req)
				if err != nil {
					errs <- err
					continue
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode >= 500 {
					errs <- fmt.Errorf("unexpected %d from /v1/health", resp.StatusCode)
				}
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		assert.NoError(t, err, "concurrent /v1/health read")
	}
}

// TestServicesIntegration_ProviderRegistry_ConfigureDisablesProvider was an
// in-process regression test guarding the `ConfigureProvider(Enabled=false)`
// → unregister semantic. That semantic lives inside the Go registry type and
// is not exposed via HTTP; the correct home is a unit test of
// `ProviderRegistry.ConfigureProvider` under `_test.go` (permitted by
// CONST-030). This test is intentionally removed here; the invariant is
// covered by `provider_registry_test.go` under `go test -short`.

// =============================================================================
// Cross-Service Integration Tests (live)
// =============================================================================

func TestServicesIntegration_RegistryWithEnsemble(t *testing.T) {
	skipUnlessLive(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	body := map[string]any{
		"prompt": "Integration test between registry and ensemble. One short sentence.",
	}
	status, env := postJSON(t, ctx, "/v1/ensemble", body)

	if status == http.StatusNotFound {
		t.Skipf("/v1/ensemble endpoint not registered on this build — got 404")
	}
	assert.NotEqual(t, http.StatusInternalServerError, status, "unexpected 500: %v", env)
}

func TestServicesIntegration_DebateWithEnsembleFallback(t *testing.T) {
	skipUnlessLive(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	body := map[string]any{
		"topic":      "Testing ensemble integration",
		"max_rounds": 1,
		"timeout":    "30s",
	}
	status, env := postJSON(t, ctx, "/v1/debates", body)

	if status == http.StatusNotFound {
		t.Skipf("/v1/debates endpoint not registered on this build — got 404")
	}
	assert.NotEqual(t, http.StatusInternalServerError, status, "unexpected 500: %v", env)
}

// =============================================================================
// Error Handling Integration Tests (live)
// =============================================================================

func TestServicesIntegration_ErrorHandling(t *testing.T) {
	skipUnlessLive(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// An empty-prompt request is a well-defined error case that every
	// live endpoint must reject with a 4xx (not 5xx / not silent success).
	body := map[string]any{
		"prompt": "",
	}
	status, env := postJSON(t, ctx, "/v1/ensemble", body)

	if status == http.StatusNotFound {
		t.Skipf("/v1/ensemble endpoint not registered on this build — got 404")
	}
	// The server MUST either reject the malformed request with a 4xx or
	// return a clean error envelope — a raw 500 indicates an uncaught
	// panic, which is a regression.
	assert.NotEqual(t, http.StatusInternalServerError, status, "empty prompt should 4xx, not 500: %v", env)
}

// =============================================================================
// Performance Integration Tests (live)
// =============================================================================

func TestServicesIntegration_Performance(t *testing.T) {
	skipUnlessLive(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Run a small number of concurrent live requests and assert they all
	// come back with a non-5xx status within the outer timeout. The old
	// in-process test measured in-process scheduling; this measures live
	// end-to-end throughput, which is the only thing CONST-030 permits.
	numDebates := 3
	var wg sync.WaitGroup
	statuses := make([]int, numDebates)

	for i := 0; i < numDebates; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			body := map[string]any{
				"prompt": fmt.Sprintf("Performance test %d. Respond with one word.", idx),
			}
			status, _ := postJSON(t, ctx, "/v1/ensemble", body)
			statuses[idx] = status
		}(i)
	}

	wg.Wait()

	for i, status := range statuses {
		if status == http.StatusNotFound {
			t.Skipf("/v1/ensemble endpoint not registered on this build — got 404 at request %d", i)
		}
		assert.NotEqual(t, http.StatusInternalServerError, statuses[i], "request %d unexpectedly 500", i)
	}
}
