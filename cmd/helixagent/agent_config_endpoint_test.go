package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
// The default is GREEN in the same commit as the fix, matching the
// convention already established by opencode_baseurl_test.go and
// redis_endpoint_test.go. redModeEnabled() is shared with those files.
//
// WHY THIS FILE EXISTS. Commit 604a2f49 resolved the endpoint for TWO
// generators (-generate-opencode-config, -generate-crush-config) and was
// returned NO-GO because the same defect still shipped on every OTHER agent.
// Measured on this host, on a build of the pre-fix tree:
//
//	$ helixagent -generate-agent-config=aider | grep -o 'localhost:[0-9]*'
//	localhost:8100                    <- server binds 7061
//	$ PORT=9999 helixagent -generate-agent-config=aider
//	localhost:8100                    <- unmoved: a literal, not a resolution
//	$ helixagent -generate-all-agents --all-agents-output-dir=DIR
//	48 files written, 47 containing localhost:8100
//
// (48, not 47: `ls` hides the dotfile `.aider.conf.yml`. The census below
// uses a directory walk for exactly that reason — an instrument that cannot
// see a file cannot testify about it.)
//
// TWO INDEPENDENT HARDCODE SOURCES, which is why a one-line fix was not
// enough. Routing GeneratorConfig through the resolver moved 659 of 1082
// endpoint occurrences; 423 remained on :8100 across 47 files because
// cliagents.DefaultMCPServers() is literally
// DefaultMCPServersForHost("localhost", 8100) — a second literal, reached by
// a different call path, emitting the nine /v1/{mcp,acp,lsp,embeddings,
// vision,cognee,rag,formatters,monitoring} URLs that are the CLI agent's
// actual working surface. Both are now resolved.
//
// WHAT THIS SUITE DOES AND DOES NOT PROVE (§11.4.6). It asserts PROPERTIES
// of every emitted file rather than the absence of one known number, and it
// is deliberately built from THREE checks because the obvious single check
// is defeatable: a movement-only oracle (does the port follow PORT?) is
// satisfied by the provider URL alone, so a PARTIAL hardcode — provider
// resolved, endpoints literal, exactly the 659-moved/423-stayed state
// measured above — passes it. The by-name and invariance checks close that.
// It does NOT prove no fourth hardcode source exists anywhere in the
// generator stack; it proves that whatever a generator emits into these
// files names the right port.

// helixAgentEndpointProbePortA and ...PortB are two arbitrary, unrelated
// ports used to drive the metamorphic check below. They are deliberately
// NOT any port this project uses (7061 HelixAgent, 8100 llm-verifier, 8443
// HelixLLM, 15432 postgres-MCP), so no static literal anywhere in the tree
// can coincide with them and pass by accident.
const (
	helixAgentEndpointProbePortA = "19071"
	helixAgentEndpointProbePortB = "19073"
)

// deterministicAPIKey pins HELIXAGENT_API_KEY so two generator runs differ
// ONLY by the port under test. Without it each run mints a fresh random key
// and the two outputs are never comparable.
const deterministicAPIKey = "test-only-not-a-real-key"

// unsetEnvForTest clears key for the duration of the test and restores the
// original value afterwards.
//
// t.Setenv has no "unset" form, and a bare os.Unsetenv would clear the
// variable for the REST OF THE PACKAGE RUN — seeding exactly the
// order-dependence §11.4.50 forbids, where a test's verdict depends on which
// test ran before it.
func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()

	previous, existed := os.LookupEnv(key)
	require.NoError(t, os.Unsetenv(key), "must be able to clear %s", key)

	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, previous)
			return
		}
		_ = os.Unsetenv(key)
	})
}

// quietLogger keeps generator chatter out of the test log.
func quietLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	return logger
}

// generateAllAgentConfigs drives the REAL -generate-all-agents code path
// into dir and returns every file it wrote, keyed by base name.
//
// It walks the directory rather than listing it, so dotfile configs
// (.aider.conf.yml) are included. A census that silently omits a file is the
// §11.4.201 false-null at the instrument layer.
func generateAllAgentConfigs(t *testing.T, dir string) map[string]string {
	t.Helper()

	err := handleGenerateAllAgents(&AppConfig{
		Logger:             quietLogger(),
		AllAgentsOutputDir: dir,
	})
	require.NoError(t, err, "the all-agents generator must succeed")

	written := make(map[string]string)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err, "the generator's output directory must be readable")

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		raw, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, readErr, "generated config %s must be readable", entry.Name())

		written[entry.Name()] = string(raw)
	}

	require.NotEmpty(t, written, "the all-agents generator must write at least one config")

	return written
}

// helixAgentAPIPaths is HelixAgent's HTTP API surface, MEASURED from the
// generated corpus rather than assumed (§11.4.6). Every path below appeared
// on a resolved endpoint in a full 48-agent generation:
//
//	/v1  /v1/acp  /v1/cognee  /v1/debate  /v1/embeddings  /v1/format
//	/v1/formatters  /v1/lsp  /v1/mcp  /v1/monitoring  /v1/rag  /v1/vision
//
// Binding the assertion to these paths is the by-NAME check (§11.4.111): a
// URL that offers a HelixAgent API must reach HelixAgent's port, whatever
// host it is spelled with and whichever code path emitted it. A new endpoint
// added without being listed here is not silently excused — it still has to
// satisfy the movement and invariant checks below.
var helixAgentAPIPaths = map[string]bool{
	"/v1":            true,
	"/v1/acp":        true,
	"/v1/cognee":     true,
	"/v1/debate":     true,
	"/v1/embeddings": true,
	"/v1/format":     true,
	"/v1/formatters": true,
	"/v1/lsp":        true,
	"/v1/mcp":        true,
	"/v1/monitoring": true,
	"/v1/rag":        true,
	"/v1/vision":     true,
}

// allowedInvariantHTTPPorts are ports permitted to appear in BOTH generation
// runs — i.e. genuinely foreign services that correctly do NOT move with
// HelixAgent's PORT.
//
// It is EMPTY, and that is a measured fact, not an oversight: across a full
// 48-agent generation every http(s) URL carrying a port resolved to the
// HelixAgent endpoint (1082 of 1082). The one foreign service in the corpus,
// the postgres MCP on 15432, is a `postgresql://` URL and so is not an
// http(s) endpoint at all. An empty allowlist therefore means "any http port
// that survives a PORT change is a hardcode", which is exactly the property
// this suite must enforce. If a legitimate foreign HTTP service is ever
// added, this test fails loudly and the port is added here WITH its
// justification — failing closed, never silently widening.
var allowedInvariantHTTPPorts = map[string]bool{}

// httpEndpoint is one "scheme://host:port/path" occurrence in a config.
type httpEndpoint struct {
	host string
	port string
	path string
}

func (e httpEndpoint) String() string {
	return e.host + ":" + e.port + e.path
}

// httpEndpointPattern matches http(s) URLs that carry an explicit port, in
// any of the corpus's formats (JSON, YAML, TOML — an endpoint is a URL in
// all of them).
//
// Deliberately NOT anchored to a known host: a hardcode spelled
// "127.0.0.1:8100" is exactly as broken as one spelled "localhost:8100", and
// an oracle keyed to the resolved host would be blind to the former.
var httpEndpointPattern = regexp.MustCompile(`https?://([^/:"'\s,\)\]]+):(\d+)((?:/[^"'\s,\)\]]*)?)`)

// httpEndpointsIn returns every ported http(s) endpoint in content.
func httpEndpointsIn(content string) []httpEndpoint {
	matches := httpEndpointPattern.FindAllStringSubmatch(content, -1)

	endpoints := make([]httpEndpoint, 0, len(matches))
	for _, match := range matches {
		endpoints = append(endpoints, httpEndpoint{
			host: match[1],
			port: match[2],
			path: strings.TrimSuffix(match[3], "/"),
		})
	}

	return endpoints
}

// httpPortsIn returns the set of ports used by ported http(s) endpoints.
func httpPortsIn(content string) map[string]bool {
	ports := make(map[string]bool)
	for _, endpoint := range httpEndpointsIn(content) {
		ports[endpoint.port] = true
	}

	return ports
}

// sortedKeys renders a port set for a failure message.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys
}

// TestAllAgentConfigs_HelixEndpointsTrackServerBindPort asserts that EVERY
// agent config the fan-out writes names the port the server actually binds.
//
// THE ORACLE IS METAMORPHIC (§11.4.107(8)), not a comparison against a known
// number. The generator runs twice under two different PORT values, and each
// file is classified per port:
//
//	invariant = ports present in BOTH runs   -> a foreign service; the
//	            postgres MCP on 15432 is the only one in this tree, and it is
//	            CORRECT for it not to move with HelixAgent's PORT
//	variant   = ports present in only ONE run -> a HelixAgent endpoint, which
//	            MUST equal that run's PORT
//
// The assertion is `variant == {PORT}` exactly. That catches both failure
// directions without naming any port:
//
//   - a hardcoded endpoint does not move between runs, so it lands in
//     `invariant` and leaves `variant` EMPTY -> FAIL (this is the shipped
//     defect: every file except opencode.json had an empty variant set)
//   - an endpoint resolved to the WRONG value moves but does not equal PORT
//     -> FAIL
//
// Because the probe ports are arbitrary and unrelated to anything in the
// tree, no literal can satisfy this by coincidence — which is the property
// that makes this a guard and not a restatement of the fix.
func TestAllAgentConfigs_HelixEndpointsTrackServerBindPort(t *testing.T) {
	t.Setenv("HELIXAGENT_API_KEY", deterministicAPIKey)

	t.Setenv("PORT", helixAgentEndpointProbePortA)
	runA := generateAllAgentConfigs(t, t.TempDir())

	t.Setenv("PORT", helixAgentEndpointProbePortB)
	runB := generateAllAgentConfigs(t, t.TempDir())

	require.Equal(t, len(runA), len(runB),
		"both runs must write the same number of configs; a run that writes "+
			"fewer files is dropping an agent, not passing this test")

	// Measured floor: the generator writes 48 files on this tree. Asserting a
	// floor (not an exact count) lets agents be ADDED without touching this
	// test, while a silent LOSS of agents — which would otherwise shrink the
	// audited surface and make this suite quietly weaker — still fails.
	require.GreaterOrEqual(t, len(runA), 48,
		"expected at least the 48 agent configs this generator is documented "+
			"to write; got %d — an agent was lost, and a lost agent is a "+
			"capability removal (§11.4.122), not a test-scope reduction",
		len(runA))

	// §11.4.146 STEP 3: the case-space here is EVERY agent, enumerated, with
	// a per-agent verdict — not a sample. defective[] accumulates the full
	// list so a failure names every affected agent in one run instead of
	// stopping at the first.
	var defective []string

	for name, contentA := range runA {
		contentB, ok := runB[name]
		require.True(t, ok, "config %s was written by one run but not the other", name)

		var faults []string

		// CHECK 1 — BY NAME. Every URL offering a HelixAgent API must carry
		// that run's port. This is the check that catches a PARTIAL hardcode:
		// when the provider URL resolves but the nine MCP endpoints do not,
		// the movement checks below are satisfied by the provider URL alone
		// while the endpoints — the CLI agent's actual working surface — still
		// point at the wrong service.
		for _, run := range []struct {
			label     string
			content   string
			probePort string
		}{
			{"PORT=" + helixAgentEndpointProbePortA, contentA, helixAgentEndpointProbePortA},
			{"PORT=" + helixAgentEndpointProbePortB, contentB, helixAgentEndpointProbePortB},
		} {
			stray := make(map[string]bool)
			for _, endpoint := range httpEndpointsIn(run.content) {
				if helixAgentAPIPaths[endpoint.path] && endpoint.port != run.probePort {
					stray[endpoint.String()] = true
				}
			}

			if len(stray) > 0 {
				faults = append(faults, fmt.Sprintf(
					"%s: HelixAgent API endpoints on the wrong port: %v",
					run.label, sortedKeys(stray)))
			}
		}

		portsA := httpPortsIn(contentA)
		portsB := httpPortsIn(contentB)

		// CHECK 2 — INVARIANCE. Any http port surviving a PORT change is a
		// hardcode unless it is an explicitly justified foreign service.
		invariant := make(map[string]bool)
		for port := range portsA {
			if portsB[port] && !allowedInvariantHTTPPorts[port] {
				invariant[port] = true
			}
		}

		if len(invariant) > 0 {
			faults = append(faults, fmt.Sprintf(
				"ports unchanged across a PORT change (hardcoded): %v",
				sortedKeys(invariant)))
		}

		// CHECK 3 — MOVEMENT. What does vary must be exactly the run's port,
		// which catches a total hardcode (nothing varies) and a resolution to
		// the wrong value (something varies, but not to PORT).
		variantA := make(map[string]bool)
		for port := range portsA {
			if !portsB[port] {
				variantA[port] = true
			}
		}

		variantB := make(map[string]bool)
		for port := range portsB {
			if !portsA[port] {
				variantB[port] = true
			}
		}

		if len(variantA) != 1 || !variantA[helixAgentEndpointProbePortA] ||
			len(variantB) != 1 || !variantB[helixAgentEndpointProbePortB] {
			faults = append(faults, fmt.Sprintf(
				"ports tracking PORT=%s were %v (want exactly [%s]); "+
					"tracking PORT=%s were %v (want exactly [%s])",
				helixAgentEndpointProbePortA, sortedKeys(variantA), helixAgentEndpointProbePortA,
				helixAgentEndpointProbePortB, sortedKeys(variantB), helixAgentEndpointProbePortB))
		}

		if len(faults) > 0 {
			defective = append(defective,
				fmt.Sprintf("%s — %s", name, strings.Join(faults, "; ")))
		}
	}

	sort.Strings(defective)

	if redModeEnabled(t) {
		require.NotEmpty(t, defective,
			"RED_MODE=1: the pre-fix generators emit a hardcoded endpoint, so "+
				"at least one config must FAIL to track PORT")
		t.Logf("RED_MODE=1: %d/%d configs do not track the server bind port:\n  %s",
			len(defective), len(runA), strings.Join(defective, "\n  "))

		return
	}

	require.Empty(t, defective,
		"%d of %d agent configs do not name the port the server binds. Each "+
			"listed config points its user at an endpoint this binary does not "+
			"serve:\n  %s",
		len(defective), len(runA), strings.Join(defective, "\n  "))

	t.Logf("all %d agent configs track the server bind port", len(runA))
}

// TestAgentConfig_SingleAgent_ResolvesRatherThanHardcodes covers the
// specific entry point the review named: -generate-agent-config=<agent>.
//
// Two assertions, because they fail for different reasons and a fix that
// satisfies one can still miss the other:
//
//   - with PORT unset, the emitted port equals config.Load().Server.Port
//     (the value run() binds) — proves agreement with the server
//   - with PORT set to an arbitrary probe, the emitted port FOLLOWS it —
//     proves the value is RESOLVED and not a second literal that happens to
//     equal the first
func TestAgentConfig_SingleAgent_ResolvesRatherThanHardcodes(t *testing.T) {
	t.Setenv("HELIXAGENT_API_KEY", deterministicAPIKey)

	generate := func(t *testing.T) map[string]bool {
		t.Helper()

		out := filepath.Join(t.TempDir(), "aider.yml")
		err := handleGenerateAgentConfig(&AppConfig{
			Logger:              quietLogger(),
			GenerateAgentConfig: "aider",
			AgentConfigOutput:   out,
		})
		require.NoError(t, err, "the single-agent generator must succeed")

		raw, err := os.ReadFile(out)
		require.NoError(t, err, "the generator must write the config file")

		ports := httpPortsIn(string(raw))
		require.NotEmpty(t, ports,
			"the generated config must name an http endpoint at all")

		return ports
	}

	t.Run("agrees_with_server_bind_port", func(t *testing.T) {
		unsetEnvForTest(t, "PORT")

		bind := config.Load().Server.Port
		ports := generate(t)

		require.True(t, ports[bind],
			"the generated agent config must name the port the server binds "+
				"(%s); got %v", bind, sortedKeys(ports))
	})

	t.Run("follows_PORT_so_it_is_resolved_not_literal", func(t *testing.T) {
		t.Setenv("PORT", helixAgentEndpointProbePortA)

		ports := generate(t)

		require.True(t, ports[helixAgentEndpointProbePortA],
			"with PORT=%s the generated config must name that port; got %v — "+
				"an unmoved port is a hardcode, not a resolution",
			helixAgentEndpointProbePortA, sortedKeys(ports))

		require.Len(t, ports, 1,
			"every http endpoint in the config must resolve to the same port; "+
				"got %v — a partial hardcode (provider URL resolved, MCP "+
				"endpoints literal) lands here", sortedKeys(ports))
	})
}

// TestHelixAgentClientHost_IsDialable is the guard for MAJOR-B: the dial host
// had no enforcement at all.
//
// THE MUTATION THIS MUST CATCH (written by the reviewer, not the author):
// change helixAgentClientHost to return config.Load().Server.Host. That
// value is getEnv("SERVER_HOST", "0.0.0.0") — the BIND WILDCARD: a valid
// address to listen on, and a meaningless one to dial. Before this test the
// whole package still passed under that mutation.
//
// The wildcard is not hypothetical: docker-compose.multi-provider.yaml:81
// sets HELIXAGENT_HOST: 0.0.0.0, and the pre-fix resolver passed the env var
// through verbatim, so the unusable host shipped through a committed compose
// file with no mutation at all:
//
//	$ HELIXAGENT_HOST=0.0.0.0 helixagent -generate-opencode-config
//	        "baseURL": "http://0.0.0.0:7061/v1"
func TestHelixAgentClientHost_IsDialable(t *testing.T) {
	t.Run("default_is_dialable", func(t *testing.T) {
		unsetEnvForTest(t, "HELIXAGENT_HOST")

		host := helixAgentClientHost()

		require.True(t, helixAgentHostIsDialable(host),
			"the default dial host %q is not an address a client can connect "+
				"to; the bind address (SERVER_HOST, default 0.0.0.0) is not an "+
				"answer to the dial-side question", host)
	})

	t.Run("wildcard_override_is_normalised", func(t *testing.T) {
		for _, wildcard := range []string{"0.0.0.0", "::", "[::]", "0:0:0:0:0:0:0:0", "  0.0.0.0  "} {
			t.Run(strings.TrimSpace(wildcard), func(t *testing.T) {
				t.Setenv("HELIXAGENT_HOST", wildcard)

				host := helixAgentClientHost()

				require.True(t, helixAgentHostIsDialable(host),
					"HELIXAGENT_HOST=%q is a listen wildcard; the generated "+
						"config must not hand it to a user as a dial target "+
						"(got %q)", wildcard, host)
			})
		}
	})

	t.Run("routable_override_passes_through", func(t *testing.T) {
		// A real remote host is the documented reason HELIXAGENT_HOST exists.
		// Normalising the wildcard must not break it (§11.4.122: repair the
		// unusable value, do not remove the capability).
		for _, routable := range []string{"helix.example.internal", "10.1.2.3", "::1"} {
			t.Setenv("HELIXAGENT_HOST", routable)

			require.Equal(t, routable, helixAgentClientHost(),
				"a routable HELIXAGENT_HOST must be passed through untouched")
		}
	})

	t.Run("emitted_config_never_carries_the_wildcard", func(t *testing.T) {
		// §11.4.108: the helper being right is the SOURCE layer. This asserts
		// the ARTIFACT layer — what a user actually opens.
		t.Setenv("HELIXAGENT_API_KEY", deterministicAPIKey)
		t.Setenv("HELIXAGENT_HOST", "0.0.0.0")

		out := filepath.Join(t.TempDir(), "opencode.json")
		require.NoError(t, handleGenerateOpenCode(&AppConfig{
			Logger:         quietLogger(),
			OpenCodeOutput: out,
		}), "the OpenCode generator must succeed")

		raw, err := os.ReadFile(out)
		require.NoError(t, err)

		require.NotContains(t, string(raw), "0.0.0.0:",
			"the emitted config names the bind wildcard as an endpoint host; "+
				"a user handed this config cannot connect to it")
	})
}

// TestProbeServiceIdentity_ClassifiesByDialStage is the guard for MAJOR-C:
// probeServiceIdentity was fail-open against its own doc comment.
//
// Its documentation promised reachable==false ONLY when nothing answered.
// The code special-cased net.Error timeouts and sent EVERY other transport
// error to the refusal bucket — so a squatter that accepts and closes,
// resets, or speaks a non-HTTP protocol was reported "unreachable" and the
// caller SKIPped. Those are precisely the cases where something IS on the
// port and is NOT HelixAgent: the shipped defect, converted into a silent
// pass.
//
// Each case below is a REAL listener on a real socket, not a fake.
func TestProbeServiceIdentity_ClassifiesByDialStage(t *testing.T) {
	cases := []struct {
		name string
		// serve handles one accepted connection; nil means "close listener,
		// leaving the port closed".
		serve func(net.Conn)
	}{
		{
			name: "tcp_accept_then_close",
			serve: func(conn net.Conn) {
				_ = conn.Close()
			},
		},
		{
			name: "connection_reset",
			serve: func(conn net.Conn) {
				// SetLinger(0) makes Close send RST instead of FIN.
				if tcp, ok := conn.(*net.TCPConn); ok {
					_ = tcp.SetLinger(0)
				}
				_ = conn.Close()
			},
		},
		{
			name: "non_http_line_protocol",
			serve: func(conn net.Conn) {
				_, _ = conn.Write([]byte("220 smtp.example.com ESMTP ready\r\n"))
				_ = conn.Close()
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err, "the test needs a real listening socket")
			defer func() { _ = listener.Close() }()

			go func() {
				for {
					conn, acceptErr := listener.Accept()
					if acceptErr != nil {
						return
					}
					testCase.serve(conn)
				}
			}()

			url := "http://" + listener.Addr().String() + "/health"
			service, reachable, detail := probeServiceIdentity(url, 3*time.Second)

			require.True(t, reachable,
				"a responder that %s is SOMETHING listening on the port, and "+
					"it is not HelixAgent — reporting it unreachable lets the "+
					"caller SKIP past the exact defect it is looking for "+
					"(detail: %s)", testCase.name, detail)
			require.Empty(t, service,
				"a non-HelixAgent responder must not yield a service identity")
		})
	}

	t.Run("closed_port_is_honestly_unreachable", func(t *testing.T) {
		// Bind then immediately release, so the address is one nothing is
		// listening on: the ONE case that legitimately SKIPs.
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := listener.Addr().String()
		require.NoError(t, listener.Close())

		_, reachable, detail := probeServiceIdentity("http://"+addr+"/health", 3*time.Second)

		require.False(t, reachable,
			"a refused connection is the one honest 'nothing is there' case; "+
				"reporting it reachable would turn an unavailable-infra SKIP "+
				"into a false FAIL (detail: %s)", detail)
	})
}

// TestGenerators_RejectUnparseablePort is the guard for MAJOR-D: bad-input
// handling was inconsistent across the generators.
//
// With PORT=abc, Crush hard-errored (correct) while OpenCode exited 0 and
// wrote "baseURL": "http://localhost:abc/v1" into the user's config — both
// guards green. All four entry points now apply the one parse rule in
// helixAgentServePortNumber, so the table below is the whole generator
// surface, not a sample.
func TestGenerators_RejectUnparseablePort(t *testing.T) {
	generators := []struct {
		name   string
		invoke func(t *testing.T, out string) error
	}{
		{
			name: "opencode",
			invoke: func(t *testing.T, out string) error {
				return handleGenerateOpenCode(&AppConfig{
					Logger:         quietLogger(),
					OpenCodeOutput: out,
				})
			},
		},
		{
			name: "crush",
			invoke: func(t *testing.T, out string) error {
				return handleGenerateCrush(&AppConfig{
					Logger:      quietLogger(),
					CrushOutput: out,
				})
			},
		},
		{
			name: "agent-config",
			invoke: func(t *testing.T, out string) error {
				return handleGenerateAgentConfig(&AppConfig{
					Logger:              quietLogger(),
					GenerateAgentConfig: "aider",
					AgentConfigOutput:   out,
				})
			},
		},
		{
			name: "all-agents",
			invoke: func(t *testing.T, out string) error {
				return handleGenerateAllAgents(&AppConfig{
					Logger:             quietLogger(),
					AllAgentsOutputDir: filepath.Dir(out),
				})
			},
		},
	}

	// §11.4.146 STEP 3: the case-space is every generator CROSSED WITH every
	// class of unusable PORT, not the one value that was reported.
	//
	// The numeric cases are here because fixing only "abc" left the same
	// defect wearing a numeric costume: strconv.Atoi answers "is this an
	// integer", which is a weaker question than "is this a port". Measured on
	// the build that had just fixed "abc", every one of these shipped:
	//
	//	PORT=0     -> "http://localhost:0/v1"
	//	PORT=-1    -> "http://localhost:-1/v1"
	//	PORT=65536 -> "http://localhost:65536/v1"
	badPorts := []struct {
		value  string
		reason string
	}{
		{value: "abc", reason: "not a number"},
		{value: "70 61", reason: "not a number (embedded space)"},
		{value: "0", reason: "port 0 is not a dialable destination"},
		{value: "-1", reason: "negative"},
		{value: "65536", reason: "one past the top of the range"},
		{value: "99999", reason: "far out of range"},
	}

	for _, generator := range generators {
		for _, bad := range badPorts {
			t.Run(generator.name+"/PORT="+bad.value, func(t *testing.T) {
				t.Setenv("HELIXAGENT_API_KEY", deterministicAPIKey)
				t.Setenv("PORT", bad.value)

				out := filepath.Join(t.TempDir(), "config.json")
				err := generator.invoke(t, out)

				require.Error(t, err,
					"PORT=%q (%s) must be refused, not written into a user's "+
						"config as an endpoint nothing can reach",
					bad.value, bad.reason)
				require.Contains(t, err.Error(), bad.value,
					"the error must name the offending value so the operator "+
						"can find it; got: %v", err)

				raw, readErr := os.ReadFile(out)
				if readErr == nil {
					require.NotContains(t, string(raw), ":"+bad.value,
						"the refused run still wrote a config naming the "+
							"invalid port")
				}
			})
		}
	}
}

// TestHelixAgentServePortNumber_MatchesServeResolution asserts the int-typed
// resolver and the string-typed one cannot disagree — they are two views of
// one answer, and a config struct typing the port as an int must not become
// a reason to re-derive it.
func TestHelixAgentServePortNumber_MatchesServeResolution(t *testing.T) {
	for _, port := range []string{"", helixAgentEndpointProbePortA, helixAgentEndpointProbePortB} {
		name := port
		if name == "" {
			name = "PORT_unset"
			unsetEnvForTest(t, "PORT")
		} else {
			t.Setenv("PORT", port)
		}

		t.Run(name, func(t *testing.T) {
			asString := helixAgentServePort(nil)

			asNumber, err := helixAgentServePortNumber(nil)
			require.NoError(t, err, "a numeric PORT must resolve")

			require.Equal(t, asString, strconv.Itoa(asNumber),
				"the int and string resolvers must return the same port")
		})
	}
}
