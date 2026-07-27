package integration

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
)

// providersProbePath is HelixAgent's LLM provider-registry route. A 200
// carrying a `providers` array is the identity signal that the host behind a
// candidate base URL is a HelixAgent with its provider registry wired — not
// merely "some HTTP server answered".
const providersProbePath = "/v1/providers"

var (
	helixBaseOnce sync.Once
	helixBase     string
	helixProbed   []string
)

// helixAgentBaseURL returns the base URL of a live HelixAgent that actually
// serves the LLM provider API.
//
// Why this exists instead of a hardcoded host:port: these tests previously
// pinned `http://localhost:8080`, which on a live platform host is the gateway
// — it answers every route with a 302 to an HTTPS port whose certificate is
// not valid for localhost, so the tests failed with an opaque x509 error
// rather than an honest "HelixAgent is not here". The companion guard
// (testutil.RequireServer) probes the canonical 81xx registry slot :8100, but
// that slot is served by LLMsVerifier, which answers every HelixAgent route
// with 404; because testutil's reachability probe treats any status < 500 as
// "available", that guard passed against the wrong service instead of
// skipping. HelixAgent itself binds the port named in its shipped config
// (configs/development.yaml -> server.port), which the 81xx registry migration
// never updated. Probing for a 200 whose body carries the provider registry
// makes both mistakes impossible.
func helixAgentBaseURL(t *testing.T) string {
	t.Helper()
	helixBaseOnce.Do(func() {
		helixBase, helixProbed = resolveHelixAgent()
	})
	if helixBase == "" {
		t.Skipf("HelixAgent provider API not reachable; probed %v — start with: make run (SKIP-OK: #infra-unavailable)", helixProbed)
	}
	return helixBase
}

// resolveHelixAgent returns the first candidate base URL whose provider route
// responds 200 with a provider registry, plus every URL probed.
func resolveHelixAgent() (string, []string) {
	client := newHelixProbeClient()
	var probed []string
	for _, base := range helixAgentCandidates() {
		url := base + providersProbePath
		probed = append(probed, url)
		if body, ok := probeOK(client, url); ok && hasJSONArrayField(body, "providers") {
			return base, probed
		}
	}
	return "", probed
}

// newHelixProbeClient builds the probe client used to identify HelixAgent.
//
// Redirects are deliberately NOT followed: the platform gateway answers :8080
// with a 302 to an HTTPS port whose certificate is not valid for localhost.
// Following it turns a clean "this is not HelixAgent" verdict into an opaque
// TLS error.
func newHelixProbeClient() *http.Client {
	return &http.Client{
		Timeout: 3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// helixAgentCandidates lists, in precedence order, the base URLs that may host
// a live HelixAgent.
func helixAgentCandidates() []string {
	seen := make(map[string]bool)
	var out []string
	add := func(u string) {
		u = strings.TrimRight(u, "/")
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}

	// 1. Explicit operator override always wins.
	add(os.Getenv("HELIXAGENT_BASE_URL"))
	// 2. The shared default (honours HELIXAGENT_HOST / HELIXAGENT_PORT, else
	//    the canonical 81xx registry slot).
	add(testutil.ServerURL())
	// 3. The port HelixAgent's shipped default config actually binds.
	host := os.Getenv("HELIXAGENT_HOST")
	if host == "" {
		host = "localhost"
	}
	add("http://" + net.JoinHostPort(host, "7061"))

	return out
}

// probeOK issues a GET and reports whether it returned 200, along with the body.
func probeOK(client *http.Client, url string) ([]byte, bool) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, false
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, false
	}
	return body, resp.StatusCode == http.StatusOK
}

// hasJSONArrayField reports whether body is a JSON object carrying `field` as
// an array.
func hasJSONArrayField(body []byte, field string) bool {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	_, ok := payload[field].([]interface{})
	return ok
}
