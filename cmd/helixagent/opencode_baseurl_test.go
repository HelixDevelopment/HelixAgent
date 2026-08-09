package main

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// §11.4.115 RED-baseline-on-the-broken-artifact + polarity switch.
//
//	RED_MODE=1 — assert the DEFECT IS PRESENT (reproduce on a pre-fix tree).
//	RED_MODE=0 — assert the defect is ABSENT. THIS IS THE DEFAULT.
//
// The default flipped to GREEN in the same commit as the fix, per §11.4.115,
// matching the convention already established by redis_endpoint_test.go.
//
// CAPTURED RED BASELINE (2026-08-09, pre-fix tree, this host):
//
//	RED_MODE=1 -> PASS  (defect reproduced, both tests)
//	  emitted baseURL "http://localhost:8100/v1"
//	  port 8100 != config.Load().Server.Port 7061
//	  http://localhost:8100/health -> 404, body "404 page not found",
//	  i.e. NOT a service identifying as "helixagent"
//	RED_MODE=0 -> FAIL:
//	  "8100" should be equal to "7061"
//	  the generated OpenCode baseURL must reach a service identifying as
//	  "helixagent"; got a non-HelixAgent responder
//
// The RED_MODE=0 assertions failed before the fix and pass after it, so this
// is not a blind test written to agree with the new code.
//
// THE PRODUCT DEFECT. The generator resolved its port with its OWN literal
// default, disagreeing with the one the server binds from:
//
//	generator (main.go handleGenerateOpenCode):
//	    port := os.Getenv("PORT"); if port == "" { port = "8100" }
//	server  (internal/config/config.go:316, via config.Load()):
//	    Port: getEnv("PORT", "7061")
//
// Two defaults for the SAME env var. With PORT unset — the documented
// out-of-the-box path — the server binds 7061 and the generator wrote 8100
// into the user's opencode.json. This is not a cosmetic mismatch: on this
// host :8100 is legitimately owned by llm-verifier (the gateway<->verifier
// contract recorded in scripts/systemd/), which answers Go's default-mux
// `404 page not found`. So the documented generator aimed real end users at
// a DIFFERENT SERVICE. It is the same registry-drift defect class that
// scripts/lib/helixagent_readiness.sh documents at the script layer, but
// here it escapes into a user-facing artifact instead of a test.
//
// WHY THE FIX IS NOT "SWAP 8100 -> 7061" (§11.4.6, §11.4.111). A second
// hardcoded literal is the same defect with a different number: it drifts
// again the moment config.Load's default changes, and it silently ignores an
// operator's PORT= in the gitignored .env that main() applies via
// godotenv.Overload BEFORE anything reads it. The port is only knowable by
// asking the authority, so the generator now resolves through config.Load()
// — the very call the server binds from (main.go:1813 -> :2219) — and the
// two agree BY CONSTRUCTION rather than by coincidence. That is the same
// reasoning helixagent_readiness.sh applies when it asks the kernel which
// port the PID it started actually bound; the authority differs (a generator
// has no PID to inspect) but the discipline is identical: resolve, never
// assume.
//
// TWO TESTS, DELIBERATELY. The port-agreement test is deterministic and
// needs no running infrastructure, so it guards the drift on every CI run.
// The identity test is the one that asserts what the operator actually cares
// about — that the emitted URL reaches HelixAgent and not some other process
// holding that port — and it can only speak when something is listening.
// Neither substitutes for the other: agreement without identity would still
// pass if config.Load itself named a squatted port, and identity without
// agreement would go silent in a bare CI container.

// redModeEnabled reports whether the RED polarity is selected. Absent or "0"
// means GREEN (assert the defect is absent) — the standing default.
func redModeEnabled(t *testing.T) bool {
	t.Helper()
	return strings.TrimSpace(os.Getenv("RED_MODE")) == "1"
}

// legacyOpenCodeBaseURLPort is the literal the generator used to emit. On
// this host it belongs to llm-verifier, not to HelixAgent.
const legacyOpenCodeBaseURLPort = "8100"

// helixAgentHealthServiceName is the identity HelixAgent reports on /health:
//
//	{"service":"helixagent","status":"healthy"}
//
// A port number is not an identity (§11.4.111). This mirrors
// tests/integration/helixagent_identity.go — keep the two in step.
const helixAgentHealthServiceName = "helixagent"

// generateOpenCodeBaseURL runs the real generator into a temp file and
// returns the baseURL it emitted for the "helixagent" provider. It drives
// the SHIPPED code path (the same function the -generate-opencode-config
// flag and the boot-time auto-regenerate goroutine both call), not a
// reimplementation of it.
func generateOpenCodeBaseURL(t *testing.T) string {
	t.Helper()

	out := filepath.Join(t.TempDir(), "opencode.json")

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	err := handleGenerateOpenCode(&AppConfig{
		Logger:         logger,
		OpenCodeOutput: out,
	})
	require.NoError(t, err, "the OpenCode config generator must succeed")

	raw, err := os.ReadFile(out)
	require.NoError(t, err, "the generator must write the config file")

	var cfg OpenCodeConfig
	require.NoError(t, json.Unmarshal(raw, &cfg),
		"the generated OpenCode config must be valid JSON")

	provider, ok := cfg.Provider["helixagent"]
	require.True(t, ok,
		"the generated config must declare the 'helixagent' provider")
	require.NotNil(t, provider.Options,
		"the 'helixagent' provider must carry options")
	require.NotEmpty(t, provider.Options.BaseURL,
		"the 'helixagent' provider must carry a baseURL")

	return provider.Options.BaseURL
}

// portOf extracts the port from a "http://host:port/v1" style base URL.
func portOf(t *testing.T, baseURL string) string {
	t.Helper()

	trimmed := strings.TrimSuffix(strings.TrimSuffix(baseURL, "/"), "/v1")
	hostPort := strings.TrimPrefix(strings.TrimPrefix(trimmed, "http://"), "https://")
	_, port, err := net.SplitHostPort(hostPort)
	require.NoError(t, err, "the emitted baseURL must carry an explicit port: %q", baseURL)
	return port
}

// TestOpenCodeConfig_BaseURLPortMatchesServerBindPort asserts the generator
// and the server resolve the SAME port for the SAME env var.
//
// This is the drift guard: it needs no running server, so it runs on every
// invocation and fails the moment the two resolutions diverge again.
func TestOpenCodeConfig_BaseURLPortMatchesServerBindPort(t *testing.T) {
	baseURL := generateOpenCodeBaseURL(t)
	emitted := portOf(t, baseURL)

	// config.Load() is what run() calls at main.go:1813 and what the server
	// binds from at main.go:2219 — the authority, not a second guess.
	bind := config.Load().Server.Port

	if redModeEnabled(t) {
		require.Equal(t, legacyOpenCodeBaseURLPort, emitted,
			"RED_MODE=1: the pre-fix generator emits its own hardcoded %s",
			legacyOpenCodeBaseURLPort)
		require.NotEqual(t, bind, emitted,
			"RED_MODE=1: the defect IS the disagreement with the bind port")
		return
	}

	require.Equal(t, bind, emitted,
		"the generated OpenCode baseURL must name the port the server binds; "+
			"a second hardcoded default re-creates the drift this guards")
}

// TestOpenCodeConfig_BaseURLIdentifiesAsHelixAgent asserts the emitted
// baseURL reaches a service that identifies itself as HelixAgent.
//
// This is the assertion that matters to the end user: not "a string
// changed", but "the URL we hand you answers as HelixAgent". Binding the
// assertion to the reported identity rather than to a port number is the
// by-name resolution §11.4.111 mandates.
//
// HONEST INSTRUMENT (§11.4.6, §11.4.3). Identity is only knowable when
// something answers. Three outcomes, never collapsed:
//   - nothing listening      -> SKIP with reason (no instrument, no verdict)
//   - answers as helixagent  -> the GREEN verdict
//   - answers as anything else (including a bare 404 from a foreign Go
//     default mux) -> FAIL; that is exactly the shipped defect
func TestOpenCodeConfig_BaseURLIdentifiesAsHelixAgent(t *testing.T) {
	baseURL := generateOpenCodeBaseURL(t)

	healthURL := strings.TrimSuffix(strings.TrimSuffix(baseURL, "/"), "/v1") + "/health"

	service, reachable, detail := probeServiceIdentity(healthURL, 3*time.Second)

	if !reachable {
		t.Skipf("nothing is listening at %s, so the responder's identity is "+
			"not knowable from here: %s (SKIP-OK: #infra-unavailable)",
			healthURL, detail) // SKIP-OK: #infra-unavailable
	}

	identifiesAsHelixAgent := strings.EqualFold(service, helixAgentHealthServiceName)

	if redModeEnabled(t) {
		require.False(t, identifiesAsHelixAgent,
			"RED_MODE=1: the pre-fix baseURL %s must NOT reach HelixAgent "+
				"(observed responder identity: %s)", healthURL, detail)
		return
	}

	require.True(t, identifiesAsHelixAgent,
		"the generated OpenCode baseURL %s must reach a service identifying "+
			"as %q; a user handed this config would otherwise be pointed at a "+
			"different service entirely (observed: %s)",
		healthURL, helixAgentHealthServiceName, detail)
}

// probeServiceIdentity GETs url and reports the value of the response's
// "service" field.
//
// It returns (service, reachable, detail). reachable is false ONLY when
// nothing answered at all — a connection refusal. Any HTTP answer, including
// a 404 or a non-JSON body, is reachable-but-not-HelixAgent and yields an
// empty service with a descriptive detail: a foreign occupant answering must
// never be reported as "unreachable", because that would let the caller skip
// past the very defect it is looking for (the §11.4.201 false-null).
//
// The "service" field must be PRESENT and equal to helixagent. There is
// deliberately no fallback to {"status":"healthy"}: this repo's own mock LLM
// server answers /health with exactly that and no service field
// (challenges/codebase/mock_server/main.go), and internal/testutil/infra.go
// records how such a fallback previously let tests pass against the mock.
func probeServiceIdentity(url string, timeout time.Duration) (service string, reachable bool, detail string) {
	client := &http.Client{Timeout: timeout}

	resp, err := client.Get(url)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			// A timeout is not a refusal: something may well be there and
			// simply slow or black-holing. Treat it as reachable-unknown so
			// it cannot be silently skipped past.
			return "", true, "request timed out: " + err.Error()
		}
		return "", false, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()

	body := make([]byte, 4096)
	n, _ := resp.Body.Read(body)
	snippet := strings.TrimSpace(string(body[:n]))

	var payload map[string]any
	if jsonErr := json.Unmarshal([]byte(snippet), &payload); jsonErr != nil {
		return "", true, "HTTP " + strconv.Itoa(resp.StatusCode) +
			", non-JSON body: " + truncate(snippet, 120)
	}

	svc, _ := payload["service"].(string)
	if svc == "" {
		return "", true, "HTTP " + strconv.Itoa(resp.StatusCode) +
			", JSON body with no \"service\" field: " + truncate(snippet, 120)
	}

	return svc, true, "HTTP " + strconv.Itoa(resp.StatusCode) +
		", service=" + svc
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
