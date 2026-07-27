package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"dev.helix.agent/internal/testutil"
)

// ---------------------------------------------------------------------------
// Endpoint resolution
// ---------------------------------------------------------------------------
//
// These chaos suites previously hardcoded `http://localhost:8100`. That is the
// port the canonical registry (internal/ports, docs/development/port-registry.md)
// assigns to HELIXAGENT_PORT_HTTP, but the shipped deployment config
// (configs/development.yaml — the file the helixagent unit is launched with)
// binds the server to a different port, and another service can legitimately own
// 8100. Because testutil.ServerAvailable() only checks "some HTTP response with
// status < 500", a foreign service answering 404 on /health was enough to make
// the whole chaos suite run against the WRONG process and then publish verdicts
// about HelixAgent. The resolution below is deterministic and configuration
// driven (no port scanning, no guessing), and every suite additionally verifies
// the endpoint really is HelixAgent before asserting anything about it.

// helixAgentBaseURL resolves the HelixAgent HTTP endpoint, in order:
//
//  1. HELIXAGENT_URL                      (explicit operator override)
//  2. HELIXAGENT_HOST / HELIXAGENT_PORT   (via testutil.ServerURL)
//  3. server.host/server.port from <repo-root>/configs/development.yaml
//     — the same file the deployed helixagent process is started with
//  4. testutil.ServerURL()                (canonical registry default)
func helixAgentBaseURL(t *testing.T) string {
	t.Helper()

	if u := strings.TrimSpace(os.Getenv("HELIXAGENT_URL")); u != "" {
		return strings.TrimSuffix(u, "/")
	}
	if os.Getenv("HELIXAGENT_HOST") != "" || os.Getenv("HELIXAGENT_PORT") != "" {
		return strings.TrimSuffix(testutil.ServerURL(), "/")
	}
	if host, port, ok := deployedServerAddr(); ok {
		return fmt.Sprintf("http://%s", net.JoinHostPort(host, port))
	}
	return strings.TrimSuffix(testutil.ServerURL(), "/")
}

// deployedServerAddr reads server.host / server.port out of the tracked
// development config. Returns ok=false when the file cannot be read or does not
// declare a port — callers then fall back to the registry default.
func deployedServerAddr() (host, port string, ok bool) {
	root, err := repoRoot()
	if err != nil {
		return "", "", false
	}
	raw, err := os.ReadFile(filepath.Join(root, "configs", "development.yaml"))
	if err != nil {
		return "", "", false
	}
	var cfg struct {
		Server struct {
			Host any `yaml:"host"`
			Port any `yaml:"port"`
		} `yaml:"server"`
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return "", "", false
	}
	if cfg.Server.Port == nil {
		return "", "", false
	}
	port = strings.TrimSpace(fmt.Sprintf("%v", cfg.Server.Port))
	if port == "" || port == "0" {
		return "", "", false
	}
	host = strings.TrimSpace(fmt.Sprintf("%v", cfg.Server.Host))
	// A server bound to 0.0.0.0 / :: is reached over the loopback address.
	if host == "" || host == "0.0.0.0" || host == "::" || host == "<nil>" {
		host = "localhost"
	}
	return host, port, true
}

// repoRoot walks up from the working directory to the directory holding the
// `module dev.helix.agent` go.mod.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if isModuleRoot(filepath.Join(dir, "go.mod"), "dev.helix.agent") {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("module root for dev.helix.agent not found above working directory")
		}
		dir = parent
	}
}

func isModuleRoot(goModPath, module string) bool {
	raw, err := os.ReadFile(goModPath)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "module "+module {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Identity gate
// ---------------------------------------------------------------------------

var (
	identityOnce sync.Once
	identityBase string
	identityErr  error
)

// requireHelixAgent returns the base URL of a verified-live HelixAgent server.
//
// It composes the pre-existing testutil.RequireServer gate (which governs the
// "no server running at all" case) with a positive identity probe: /health must
// answer 200 with a healthy payload AND /v1/models must return HelixAgent's own
// catalogue. If some *other* service answers at the resolved address the test
// fails loudly instead of silently producing a HelixAgent verdict from a
// stranger's responses.
func requireHelixAgent(t *testing.T) string {
	t.Helper()
	testutil.RequireServer(t)

	identityOnce.Do(func() {
		identityBase = helixAgentBaseURL(t)
		identityErr = verifyHelixAgentIdentity(identityBase)
	})
	if identityErr != nil {
		t.Fatalf("resolved endpoint %s is not a live HelixAgent server: %v\n"+
			"resolution order: HELIXAGENT_URL, HELIXAGENT_HOST/HELIXAGENT_PORT, "+
			"configs/development.yaml server.port, canonical registry default",
			identityBase, identityErr)
	}
	return identityBase
}

// verifyHelixAgentIdentity performs the positive identity probe.
func verifyHelixAgentIdentity(base string) error {
	client := newChaosClient(2, 10*time.Second)
	defer client.CloseIdleConnections()

	status, body, err := getWithBody(client, base+"/health")
	if err != nil {
		return fmt.Errorf("GET /health failed: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("GET /health returned %d (want 200); body=%s", status, truncate(body, 200))
	}
	var health struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &health); err != nil {
		return fmt.Errorf("GET /health body is not JSON: %s", truncate(body, 200))
	}
	if !strings.EqualFold(health.Status, "healthy") {
		return fmt.Errorf("GET /health reports status=%q (want \"healthy\")", health.Status)
	}

	status, body, err = getWithBody(client, base+"/v1/models")
	if err != nil {
		return fmt.Errorf("GET /v1/models failed: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("GET /v1/models returned %d (want 200); body=%s", status, truncate(body, 200))
	}
	models, err := parseModelList(body)
	if err != nil {
		return fmt.Errorf("GET /v1/models: %w", err)
	}
	for _, m := range models.Data {
		if m.OwnedBy == "helixagent" {
			return nil
		}
	}
	return fmt.Errorf("GET /v1/models returned no model owned_by \"helixagent\" (%d models); "+
		"the service on this address is not HelixAgent", len(models.Data))
}

type modelList struct {
	Object string `json:"object"`
	Data   []struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

func parseModelList(body []byte) (modelList, error) {
	var ml modelList
	if err := json.Unmarshal(body, &ml); err != nil {
		return ml, fmt.Errorf("body is not a JSON model list: %s", truncate(body, 200))
	}
	if ml.Object != "list" {
		return ml, fmt.Errorf("object=%q (want \"list\")", ml.Object)
	}
	if len(ml.Data) == 0 {
		return ml, fmt.Errorf("model list is empty")
	}
	return ml, nil
}

// ---------------------------------------------------------------------------
// HTTP client
// ---------------------------------------------------------------------------

// newChaosClient builds an HTTP client whose socket usage is hard-capped.
//
// The previous suites used http.DefaultTransport semantics and closed response
// bodies without draining them, so Go could never reuse a keep-alive connection:
// every request opened a fresh TCP connection. Twenty goroutines looping for ten
// seconds produced ~28 000 sockets in TIME_WAIT, which exhausted the host's
// ephemeral port range (32768-60999) and made every subsequent connect() return
// EADDRNOTAVAIL. The suites then reported "Server should remain responsive" —
// blaming the server for the *client's* resource exhaustion. Capping
// MaxConnsPerHost bounds the socket count regardless of load, and draining
// bodies (see drainClose) lets connections be reused.
func newChaosClient(maxConns int, timeout time.Duration) *http.Client {
	if maxConns < 1 {
		maxConns = 1
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: nil, // never route loopback chaos traffic through a proxy
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxConnsPerHost:     maxConns,
			MaxIdleConns:        maxConns,
			MaxIdleConnsPerHost: maxConns,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

const maxCapturedBody = 1 << 20 // 1 MiB

// readClose reads the (bounded) body and closes it.
//
// Reading the body to completion before closing is what lets Go return the
// connection to the idle pool. The previous suites called resp.Body.Close()
// on an undrained body, which forces the transport to tear the connection
// down — the root of the socket churn described above.
func readClose(resp *http.Response) []byte {
	if resp == nil || resp.Body == nil {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxCapturedBody))
	_ = resp.Body.Close()
	return body
}

func getWithBody(client *http.Client, url string) (int, []byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, readClose(resp), nil
}

func truncate(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ---------------------------------------------------------------------------
// Health oracle
// ---------------------------------------------------------------------------

// serverHealth probes /health and returns a descriptive error when the server is
// not healthy. Unlike the previous checkAvailable helper — which returned true
// for *any* HTTP response, including a 404 from an unrelated service — this
// requires 200 plus a healthy status payload.
func serverHealth(base string) error {
	client := newChaosClient(2, 10*time.Second)
	defer client.CloseIdleConnections()

	status, body, err := getWithBody(client, base+"/health")
	if err != nil {
		return fmt.Errorf("health probe transport error: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("health probe returned %d; body=%s", status, truncate(body, 200))
	}
	var health struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &health); err != nil {
		return fmt.Errorf("health body is not JSON: %s", truncate(body, 200))
	}
	if !strings.EqualFold(health.Status, "healthy") {
		return fmt.Errorf("health status=%q (want \"healthy\")", health.Status)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Response bookkeeping
// ---------------------------------------------------------------------------

// statusTally records the outcome of a storm: how many responses arrived per
// status code, how many transport errors occurred (with the first message kept
// for diagnostics), and how many response bodies were not valid JSON.
type statusTally struct {
	mu           sync.Mutex
	byStatus     map[int]int64
	transportErr int64
	firstErr     string
	nonJSONBody  int64
	firstNonJSON string
	total        int64
}

func newTally() *statusTally {
	return &statusTally{byStatus: make(map[int]int64)}
}

func (s *statusTally) recordError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transportErr++
	if s.firstErr == "" {
		s.firstErr = err.Error()
	}
}

// recordResponse tallies a status code. When requireJSON is set the body must
// parse as JSON — a truncated / empty / HTML body under load means the server
// dropped the response mid-flight rather than refusing cleanly.
func (s *statusTally) recordResponse(status int, body []byte, requireJSON bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.total++
	s.byStatus[status]++
	if requireJSON && !json.Valid(bytes.TrimSpace(body)) {
		s.nonJSONBody++
		if s.firstNonJSON == "" {
			s.firstNonJSON = fmt.Sprintf("status=%d body=%q", status, truncate(body, 200))
		}
	}
}

// firstError records the first message seen across concurrent goroutines.
type firstError struct {
	mu  sync.Mutex
	msg string
	n   int64
}

func (f *firstError) record(msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	if f.msg == "" {
		f.msg = msg
	}
}

func (f *firstError) snapshot() (int64, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.n, f.msg
}

func (s *statusTally) snapshot() (total, transportErr, nonJSON int64, byStatus map[int]int64, firstErr, firstNonJSON string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	byStatus = make(map[int]int64, len(s.byStatus))
	for k, v := range s.byStatus {
		byStatus[k] = v
	}
	return s.total, s.transportErr, s.nonJSONBody, byStatus, s.firstErr, s.firstNonJSON
}

// serverErrors counts crash-shaped 5xx responses. 503 is excluded: it is the
// documented graceful-shed status of the concurrency limiter, i.e. a clean
// refusal rather than a failure.
func serverErrors(byStatus map[int]int64) int64 {
	var n int64
	for status, count := range byStatus {
		if status >= 500 && status != http.StatusServiceUnavailable {
			n += count
		}
	}
	return n
}

// successes counts 2xx responses.
func successes(byStatus map[int]int64) int64 {
	var n int64
	for status, count := range byStatus {
		if status >= 200 && status < 300 {
			n += count
		}
	}
	return n
}

func formatStatuses(byStatus map[int]int64) string {
	if len(byStatus) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(byStatus))
	for status, count := range byStatus {
		parts = append(parts, fmt.Sprintf("%d=%d", status, count))
	}
	// Deterministic ordering for readable failure output.
	sortStrings(parts)
	return strings.Join(parts, " ")
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
