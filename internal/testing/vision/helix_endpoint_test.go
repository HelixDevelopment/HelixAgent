package vision

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

// visionProbePath is the vision subsystem's own health route. A 200 here is the
// identity signal that the host behind a candidate base URL is a HelixAgent
// with vision registered — not merely "some HTTP server answered".
const visionProbePath = "/v1/vision/health"

var (
	visionBaseOnce sync.Once
	visionBase     string
	visionProbed   []string
)

// helixAgentBaseURL returns the base URL of a live HelixAgent that actually
// serves the vision API.
//
// Why this exists instead of using testutil.ServerURL() directly: on a host
// running the full Helix platform, neighbouring services occupy the ports the
// tests used to assume. testutil.ServerURL() defaults to the canonical 81xx
// registry slot :8100, but on a live platform host that slot is served by
// LLMsVerifier, which answers every HelixAgent route with 404. HelixAgent
// itself binds the port named in its shipped config (configs/development.yaml
// -> server.port), which the 81xx registry migration never updated.
//
// testutil's reachability probe treats any status < 500 as "available", so a
// plain reachability check happily binds these tests to that foreign service;
// the mismatch then surfaces much later as a confusing 404 assertion failure
// rather than an honest "HelixAgent is not here". Probing for a 200 plus
// HelixAgent's own vision service banner makes that class of mistake
// impossible.
func helixAgentBaseURL(t *testing.T) string {
	t.Helper()
	visionBaseOnce.Do(func() {
		visionBase, visionProbed = resolveHelixAgentVision()
	})
	if visionBase == "" {
		t.Skipf("HelixAgent vision API not reachable; probed %v — start with: make run (SKIP-OK: #infra-unavailable)", visionProbed)
	}
	return visionBase
}

// resolveHelixAgentVision returns the first candidate base URL whose vision
// health route responds 200 with a vision service banner, plus every URL probed.
func resolveHelixAgentVision() (string, []string) {
	client := newHelixProbeClient()
	var probed []string
	for _, base := range helixAgentCandidates() {
		url := base + visionProbePath
		probed = append(probed, url)
		if body, ok := probeOK(client, url); ok && jsonFieldEquals(body, "service", "vision") {
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

// jsonFieldEquals reports whether body is a JSON object whose `field` is the
// string `want`.
func jsonFieldEquals(body []byte, field, want string) bool {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	got, _ := payload[field].(string)
	return got == want
}
