package main

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// HXC-286 — GROUP A: HelixAgent's OWN endpoint composition
// (helixAgentClientBaseURL + the Crush baseURL/MCP-URL sites at
// cmd/helixagent/main.go:1948,4460,4508).
//
// LINE-NUMBER BASELINE (F4 of the round-5 review, declared 2026-08-13).
// Every main.go line number in THIS FILE is a PRE-FIX HEAD line — the tree
// this RED baseline reproduces the defect against, which is the correct
// baseline for a §11.4.115 RED test and the reason the prose is present-tense
// about them. They do NOT resolve in the worktree, where the same sites have
// moved. The sibling doc in internal/netaddr/netaddr.go cites main.go by
// WORKTREE line (:305 / :802 / :2442) because it describes the post-fix tree;
// one changeset, two baselines, each now stated where it is used rather than
// left for a reader to infer. The stable identifiers are the FUNCTION NAMES
// each citation already carries (helixAgentClientBaseURL, getCrushMCPServers,
// the Crush generator) — prefer those over any line number.
//
// §11.4.115 RED-baseline-on-the-broken-artifact + polarity switch.
//
//	RED_MODE=1 — assert the DEFECT IS PRESENT (reproduce on the pre-fix tree).
//	RED_MODE=0 — assert the defect is ABSENT. THIS IS THE DEFAULT.
//
// THE DEFECT (empirically verified, not assumed — §11.4.6). The pre-fix code
// composed "http://%s:%d" via fmt.Sprintf using whatever HELIXAGENT_HOST
// resolved to verbatim. helixAgentHostIsDialable already unbrackets-then-
// parses to VALIDATE the host (so it happily accepts an operator writing
// HELIXAGENT_HOST as a bare, unbracketed IPv6 literal — the natural way to
// type a bare IP address, since brackets are URL-authority syntax, not a
// property of the address itself) but helixAgentClientHost then returned
// that value UNBRACKETED, and fmt.Sprintf appended ":<port>" onto it
// literally. For an IPv6 host this produces a string with three or more
// colons that NO reader can parse into (host, port) — measured directly
// against net/url.Parse and net.SplitHostPort on this host, 2026-08-12:
//
//	fmt.Sprintf("http://%s:%d", "2001:db8::1", 7061)
//	  -> "http://2001:db8::1:7061"
//	  net/url.Parse:        parse "http://2001:db8::1:7061": invalid port "db8::1:7061" after host
//	  net.SplitHostPort:    address 2001:db8::1:7061: too many colons in address
//
// So a HelixAgent operator who deploys behind a routable IPv6 address and
// sets HELIXAGENT_HOST to it (unbracketed, as one naturally would) gets a
// baseURL that is emitted into generated CLI-agent configs but that NO HTTP
// client can dial: the "reader" (any net/http.Client, or any tool that calls
// net.SplitHostPort on the authority) rejects the address outright before a
// single byte reaches the wire. Reachability is lost completely — the exact
// HXC-286 family defect, verified concretely for this call path rather than
// assumed from the family's general description.
const redModeGroupAHost = "2001:db8::1"

func redModeGroupAEnabled(t *testing.T) bool {
	t.Helper()
	return os.Getenv("RED_MODE") == "1"
}

// withHelixAgentHost sets HELIXAGENT_HOST for the duration of the test and
// restores whatever was there before, so this test never leaks state into
// its siblings (they run in the same package/process).
func withHelixAgentHost(t *testing.T, host string) {
	t.Helper()
	prev, had := os.LookupEnv("HELIXAGENT_HOST")
	require.NoError(t, os.Setenv("HELIXAGENT_HOST", host))
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("HELIXAGENT_HOST", prev)
		} else {
			_ = os.Unsetenv("HELIXAGENT_HOST")
		}
	})
}

// TestHelixAgentClientBaseURL_UnbracketedIPv6 drives
// helixAgentClientBaseURL — the function backing main.go:1948 — directly.
func TestHelixAgentClientBaseURL_UnbracketedIPv6(t *testing.T) {
	withHelixAgentHost(t, redModeGroupAHost)

	got, err := helixAgentClientBaseURL(&AppConfig{})
	require.NoError(t, err, "helixAgentClientBaseURL itself must not error — the defect is in the STRING it produces, not in an early failure")

	_, parseErr := url.Parse(got)

	if redModeGroupAEnabled(t) {
		require.Error(t, parseErr,
			"RED_MODE=1: the pre-fix baseURL %q must be UNPARSEABLE by a reader — that is the defect", got)
		return
	}

	require.NoError(t, parseErr,
		"the baseURL %q must be parseable by any HTTP client (net/url); an IPv6 HELIXAGENT_HOST must be bracketed", got)

	u, err := url.Parse(got)
	require.NoError(t, err)
	require.Equal(t, redModeGroupAHost, u.Hostname(),
		"the parsed host must be the exact HELIXAGENT_HOST value, not a truncated or merged string")
	require.NotEmpty(t, u.Port(), "the port must survive composition — a bracket omission silently swallows it into the host")
}

// crushBaseURLsFromGeneratedConfig runs the SHIPPED handleGenerateCrush code
// path (the same function the -generate-crush-config flag calls) into a temp
// file and returns every baseURL-shaped string it wrote: the provider
// base_url (main.go:4460's fmt.Sprintf) and the MCP server URL
// (main.go:4508's fmt.Sprintf, via getCrushMCPServers).
func crushBaseURLsFromGeneratedConfig(t *testing.T) (providerBaseURL string, mcpURLs []string) {
	t.Helper()

	out := filepath.Join(t.TempDir(), "crush.json")
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	err := handleGenerateCrush(&AppConfig{
		Logger:      logger,
		CrushOutput: out,
	})
	require.NoError(t, err, "the Crush config generator must succeed")

	raw, err := os.ReadFile(out)
	require.NoError(t, err)

	var cfg CrushConfig
	require.NoError(t, json.Unmarshal(raw, &cfg), "the generated Crush config must be valid JSON")

	provider, ok := cfg.Providers["helixagent"]
	require.True(t, ok, "the generated config must declare the 'helixagent' provider")
	providerBaseURL = provider.BaseURL
	require.NotEmpty(t, providerBaseURL)

	for name, mcp := range cfg.Mcp {
		if mcp.URL != "" {
			mcpURLs = append(mcpURLs, mcp.URL)
			_ = name
		}
	}
	require.NotEmpty(t, mcpURLs, "the generated config must declare at least one MCP server URL")

	return providerBaseURL, mcpURLs
}

// TestHandleGenerateCrush_UnbracketedIPv6ProviderBaseURL drives the actual
// shipped generator (main.go:4460) and asserts on the provider base_url it
// writes into the user's crush.json.
func TestHandleGenerateCrush_UnbracketedIPv6ProviderBaseURL(t *testing.T) {
	withHelixAgentHost(t, redModeGroupAHost)

	providerBaseURL, _ := crushBaseURLsFromGeneratedConfig(t)
	_, parseErr := url.Parse(providerBaseURL)

	if redModeGroupAEnabled(t) {
		require.Error(t, parseErr,
			"RED_MODE=1: the shipped generator's provider base_url %q must be unparseable pre-fix", providerBaseURL)
		return
	}

	require.NoError(t, parseErr,
		"the shipped generator's provider base_url %q must be parseable", providerBaseURL)
	u, _ := url.Parse(providerBaseURL)
	require.Equal(t, redModeGroupAHost, u.Hostname())
	require.NotEmpty(t, u.Port())
}

// TestHandleGenerateCrush_UnbracketedIPv6MCPURL drives the same generator and
// asserts on the MCP server URL (main.go:4508) — the second independent
// composition site in this function, sharing the same host/port pair as the
// provider baseURL. Both MUST agree: if only one were fixed, the crush.json
// would name a working provider but broken MCP endpoints (or vice versa) —
// exactly the "absorption class" the task warns about, checked explicitly
// here rather than assumed from the provider check alone.
func TestHandleGenerateCrush_UnbracketedIPv6MCPURL(t *testing.T) {
	withHelixAgentHost(t, redModeGroupAHost)

	_, mcpURLs := crushBaseURLsFromGeneratedConfig(t)

	for _, raw := range mcpURLs {
		_, parseErr := url.Parse(raw)
		if redModeGroupAEnabled(t) {
			require.Error(t, parseErr,
				"RED_MODE=1: MCP URL %q must be unparseable pre-fix", raw)
			continue
		}
		require.NoError(t, parseErr, "MCP URL %q must be parseable", raw)
		u, _ := url.Parse(raw)
		require.Equal(t, redModeGroupAHost, u.Hostname(),
			"MCP URL %q must name the exact configured host", raw)
		require.NotEmpty(t, u.Port(), "MCP URL %q must carry an explicit port", raw)
	}
}
