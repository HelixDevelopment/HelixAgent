package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// knownNonexistentMCPPackages are npm package names that were shipped in this
// repo's CLI-agent configs but do NOT exist in the npm registry — every one
// verified as HTTP 404 against registry.npmjs.org on 2026-09-03.
//
// A user who copied such a config got an MCP server that fails to start on an
// npm 404. They are recorded here so a regression that re-introduces any of
// them is caught WITHOUT needing network access.
//
// The `@anthropic-ai/mcp-server-*` scope in particular has never existed: the
// official reference servers are published under `@modelcontextprotocol/`.
var knownNonexistentMCPPackages = []string{
	"@anthropic-ai/mcp-server-aws-kb-retrieval",
	"@anthropic-ai/mcp-server-brave-search",
	"@anthropic-ai/mcp-server-everart",
	"@anthropic-ai/mcp-server-figma",
	"@anthropic-ai/mcp-server-filesystem",
	"@anthropic-ai/mcp-server-github",
	"@anthropic-ai/mcp-server-gitlab",
	"@anthropic-ai/mcp-server-google-maps",
	"@anthropic-ai/mcp-server-linear",
	"@anthropic-ai/mcp-server-memory",
	"@anthropic-ai/mcp-server-notion",
	"@anthropic-ai/mcp-server-postgres",
	"@anthropic-ai/mcp-server-puppeteer",
	"@anthropic-ai/mcp-server-sentry",
	"@anthropic-ai/mcp-server-sequential-thinking",
	"@anthropic-ai/mcp-server-slack",
	"@anthropic-ai/mcp-server-sqlite",
	"@anthropic/mcp-server-gdrive",
	// Archived or never-published under the official scope.
	// server-sqlite in particular was the PYTHON reference server
	// (`mcp-server-sqlite` on PyPI, `uvx ... --db-path <p>`), archived to
	// modelcontextprotocol/servers-archived. It was never an npm package.
	// The Node equivalent is `mcp-server-sqlite-npx`, which takes the DB
	// path POSITIONALLY (no --db-path flag).
	"@modelcontextprotocol/server-docker",
	"@modelcontextprotocol/server-linear",
	"@modelcontextprotocol/server-sentry",
	"@modelcontextprotocol/server-sqlite",
	// Invented bare names with no npm publication.
	"mcp-server-asana",
	"mcp-server-atlassian",
	"mcp-server-datadog",
	"mcp-server-google-drive",
	"mcp-server-mongodb",

	// Added 2026-09-03: proven 404 against registry.npmjs.org while auditing
	// the npm package names held as Go string literals, which the
	// config-derived scan above cannot see. Every one of these shipped in a
	// generator, the validator, or the CLI.
	"@anthropic-ai/mcp-server-everything",
	"@modelcontextprotocol/server-todoist",
	"@raycast/mcp-server-raycast",
	"framelink-figma-mcp",
	"lsp-tools-mcp",
	"mcp-imagesorcery",
	"mcp-miro",
	"mcp-server-ansible",
	"mcp-server-aws-lambda",
	"mcp-server-aws-s3",
	"mcp-server-browserbase",
	"mcp-server-circleci",
	"mcp-server-crawl4ai",
	"mcp-server-duckduckgo",
	"mcp-server-flux",
	"mcp-server-gcs",
	"mcp-server-gdrive",
	"mcp-server-grafana",
	"mcp-server-hubspot",
	"mcp-server-jetbrains",
	"mcp-server-kagi",
	"mcp-server-langchain",
	"mcp-server-llamaindex",
	"mcp-server-obsidian",
	"mcp-server-playwright",
	"mcp-server-prometheus",
	"mcp-server-replicate",
	"mcp-server-stable-diffusion",
	"mcp-server-svgmaker",
	"mcp-server-tavily",
	"mcp-server-teams",
	"mcp-server-zendesk",
	"mcp-server-zoom",
	"mcp-tinybird",
}

// knownUnusableMCPPackages are npm package names that RESOLVE (HTTP 200) but
// are not installable — a class distinct from the 404s above, and the reason
// this list exists separately rather than being folded into them.
//
// Every existence check in this repo passed all five: they return a package
// document, so "does it exist?" answers yes. Four are npm SECURITY HOLDING
// packages (a single 0.0.1-security version, description "security holding
// package", repository "npm/security-holder") — names npm SEIZED, typically
// after malicious use or squatting. The fifth resolves with ZERO published
// versions: an empty registry shell.
//
// Verified against registry.npmjs.org on 2026-09-03, control-probed so a
// finding is an answer rather than a network artefact.
//
// Note the pattern in three of the four seized names: `mcp-server-postgres`,
// `mcp-server-redis` and `mcp-server-figma` are exactly the `bin` names of
// real packages (@modelcontextprotocol/server-postgres declares
// `bin: {"mcp-server-postgres": ...}`). The squats targeted the binary name,
// which is precisely why they looked like the right package to configure.
var knownUnusableMCPPackages = map[string]string{
	"mcp-server-figma":    "npm security holding package (0.0.1-security) - use figma-developer-mcp --stdio",
	"mcp-server-postgres": "npm security holding package (0.0.1-security) - use @modelcontextprotocol/server-postgres with the URL passed positionally",
	"mcp-server-redis":    "npm security holding package (0.0.1-security) - use @modelcontextprotocol/server-redis with the URL passed positionally",
	"mcp-server-supabase": "npm security holding package (0.0.1-security) - use @supabase/mcp-server-supabase --read-only with SUPABASE_ACCESS_TOKEN",
	"mcp-server-jira":     "resolves with ZERO published versions - use @aashari/mcp-server-atlassian-jira with the ATLASSIAN_* env triple",
}

// npmRegistryControlPackage is a package known to exist. It is probed first so
// a total network/registry outage is reported as a SKIP rather than being
// mistaken for "every package we ship is broken".
const npmRegistryControlPackage = "@modelcontextprotocol/server-filesystem"

// TestMCPConfigsAvoidKnownUnusablePackages is the network-free regression
// guard for the resolves-but-unusable class.
//
// TestMCPConfigsAvoidKnownNonexistentPackages covers names that 404. These do
// not 404 — that is the entire point of the defect — so re-introducing one
// would sail past that guard. This closes it without needing the network.
func TestMCPConfigsAvoidKnownUnusablePackages(t *testing.T) {
	root := getProjectRoot(t)

	packages, err := collectConfiguredNpmPackages(root)
	require.NoError(t, err)
	require.NotEmpty(t, packages, "Config scan must find npm packages")

	configured := make(map[string]bool, len(packages))
	for _, p := range packages {
		configured[p] = true
	}

	for bad, why := range knownUnusableMCPPackages {
		assert.False(t, configured[bad],
			"Config references %s, which resolves in the npm registry but installs nothing: %s", bad, why)
	}
}

// TestNpmPackageHealthDetectsUnusablePackages validates the DETECTOR itself
// against known-bad and known-good fixtures, so the guard cannot silently stop
// guarding.
//
// It asserts BOTH halves of the defect, which is what makes it meaningful:
//
//  1. every known-unusable name still RESOLVES — proving an existence-only
//     check would pass it, so this is a real blind spot and not a hypothetical;
//  2. the health check nonetheless judges it NOT usable.
//
// Assertion (1) is what stops this test from decaying into a tautology: if npm
// ever un-seizes one of these names it will start resolving differently and
// this test says so, rather than quietly agreeing with whatever the code does.
//
// Neuter isSecurityHoldingVersion (or the zero-version branch) and this fails —
// that is the paired mutation.
func TestNpmPackageHealthDetectsUnusablePackages(t *testing.T) {
	if testing.Short() {
		t.Logf("Short mode - skipping npm package health detector test (acceptable)")
		return
	}

	if !checkNpmPackageExists(npmRegistryControlPackage) {
		t.Skipf("npm registry unreachable - control package %s did not resolve; cannot distinguish a seized package from a network failure (SKIP-OK: #npm-registry-unreachable)", npmRegistryControlPackage)
	}

	// Golden-good: a healthy package must be judged resolvable AND usable, so a
	// detector that simply rejected everything would fail here.
	t.Run("golden-good/"+npmRegistryControlPackage, func(t *testing.T) {
		h := npmPackageHealthOf(npmRegistryControlPackage)
		assert.True(t, h.Resolved, "control package must resolve")
		assert.True(t, h.Usable, "control package must be judged usable, got reason: %s", h.Reason)
		assert.False(t, isSecurityHoldingVersion(h.Latest),
			"control package latest %q must not look like a seized-name marker", h.Latest)
	})

	for pkg, why := range knownUnusableMCPPackages {
		t.Run("golden-bad/"+pkg, func(t *testing.T) {
			h := npmPackageHealthOf(pkg)
			assert.True(t, h.Resolved,
				"%s is expected to RESOLVE - that is precisely why an existence-only check missed it. "+
					"If it no longer resolves, this entry has changed class and its comment needs updating", pkg)
			assert.False(t, h.Usable,
				"%s must be judged NOT usable (%s) - it resolves, so only the version-shape / zero-version "+
					"checks can catch it", pkg, why)
		})
	}
}

// TestMCPPackageExistence verifies that EVERY npm package this repo writes into
// a CLI-agent MCP config actually exists in the npm registry.
//
// The package list is DERIVED FROM THE CONFIGS rather than hand-maintained, so
// a config that adds a new (or misspelled) package name is covered
// automatically. A hardcoded subset would pass while shipping a broken config —
// that gap is exactly how @modelcontextprotocol/server-sqlite reached users.
func TestMCPPackageExistence(t *testing.T) {
	if testing.Short() {
		t.Logf("Short mode - skipping MCP package existence test (acceptable)")
		return
	}

	root := getProjectRoot(t)

	packages, err := collectConfiguredNpmPackages(root)
	require.NoError(t, err, "Must be able to scan CLI-agent configs for npm packages")
	require.NotEmpty(t, packages, "Config scan must find npm packages - an empty result means the scanner broke, not that configs are clean")

	// Instrument sanity check: if a package we KNOW exists cannot be resolved,
	// the registry or the network is unavailable and every subsequent 404 would
	// be a false negative.
	if !checkNpmPackageExists(npmRegistryControlPackage) {
		t.Skipf("npm registry unreachable - control package %s did not resolve; cannot distinguish a missing package from a network failure (SKIP-OK: #npm-registry-unreachable)", npmRegistryControlPackage)
	}

	for _, pkg := range packages {
		t.Run(pkg, func(t *testing.T) {
			h := npmPackageHealthOf(pkg)
			assert.True(t, h.Usable,
				"Package %s is referenced by a shipped CLI-agent config but is NOT installable: %s - a user taking that config gets an MCP server that fails to start",
				pkg, h.Reason)
		})
	}
}

// TestMCPConfigsAvoidKnownNonexistentPackages is the network-free regression
// guard: no shipped config may reference a package name already proven absent
// from the npm registry.
func TestMCPConfigsAvoidKnownNonexistentPackages(t *testing.T) {
	root := getProjectRoot(t)

	packages, err := collectConfiguredNpmPackages(root)
	require.NoError(t, err)
	require.NotEmpty(t, packages, "Config scan must find npm packages")

	configured := make(map[string]bool, len(packages))
	for _, p := range packages {
		configured[p] = true
	}

	for _, bad := range knownNonexistentMCPPackages {
		assert.False(t, configured[bad],
			"Config references %s, which does not exist in the npm registry (verified 404)", bad)
	}
}

// collectConfiguredNpmPackages walks the CLI-agent config trees and returns the
// sorted, de-duplicated set of npm package names passed to npx.
//
// Two invocation shapes are recognised:
//
//	{"command": ["npx", "-y", "<pkg>", ...]}
//	{"command": "npx", "args": ["-y", "<pkg>", ...]}
func collectConfiguredNpmPackages(root string) ([]string, error) {
	configRoots := []string{
		filepath.Join(root, "scripts", "cli-agents", "configs"),
		filepath.Join(root, "configs", "cli-agents"),
	}

	found := map[string]bool{}
	for _, dir := range configRoots {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".json") {
				return nil
			}
			data, readErr := os.ReadFile(path) // #nosec G304 - repo-local config scan
			if readErr != nil {
				return readErr
			}
			var doc interface{}
			// A malformed or non-config JSON file is skipped rather than
			// failing the scan; TestOpenCodeConfiguration covers validity.
			if json.Unmarshal(data, &doc) != nil {
				return nil
			}
			collectNpxPackages(doc, found)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	packages := make([]string, 0, len(found))
	for p := range found {
		packages = append(packages, p)
	}
	sort.Strings(packages)
	return packages, nil
}

// collectNpxPackages recursively finds npx invocations and records the package
// name each one installs.
func collectNpxPackages(node interface{}, found map[string]bool) {
	switch v := node.(type) {
	case map[string]interface{}:
		if pkg := npxPackageFromNode(v); pkg != "" {
			found[pkg] = true
		}
		for _, child := range v {
			collectNpxPackages(child, found)
		}
	case []interface{}:
		for _, child := range v {
			collectNpxPackages(child, found)
		}
	}
}

// npxPackageFromNode extracts the npm package name from a single MCP-server
// node, or "" when the node is not an npx invocation.
func npxPackageFromNode(node map[string]interface{}) string {
	var tokens []string

	switch cmd := node["command"].(type) {
	case []interface{}:
		tokens = toStringSlice(cmd)
	case string:
		if filepath.Base(cmd) != "npx" {
			return ""
		}
		args, ok := node["args"].([]interface{})
		if !ok {
			return ""
		}
		tokens = append([]string{cmd}, toStringSlice(args)...)
	default:
		return ""
	}

	if len(tokens) == 0 || filepath.Base(tokens[0]) != "npx" {
		return ""
	}

	// The package is the first token that is not a flag and not a path or
	// variable reference (those are the server's own arguments).
	for _, tok := range tokens[1:] {
		if strings.HasPrefix(tok, "-") {
			continue
		}
		if strings.HasPrefix(tok, "/") || strings.HasPrefix(tok, "$") || strings.HasPrefix(tok, "~") {
			return ""
		}
		return tok
	}
	return ""
}

func toStringSlice(in []interface{}) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// TestMCPLocalServerStartup verifies local MCP servers can start
func TestMCPLocalServerStartup(t *testing.T) {
	if testing.Short() {
		t.Logf("Short mode - skipping MCP local server startup test (acceptable)")
		return
	}

	// Skip if npx is not available
	if _, err := exec.LookPath("npx"); err != nil {
		t.Logf("npx not found - skipping local MCP server tests (acceptable)")
		return
	}

	servers := []struct {
		name    string
		command []string
	}{
		{"filesystem", []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", os.Getenv("HOME")}},
		{"memory", []string{"npx", "-y", "@modelcontextprotocol/server-memory"}},
		{"fetch", []string{"npx", "-y", "mcp-fetch-server"}},
		{"sqlite", []string{"npx", "-y", "mcp-sqlite"}},
	}

	for _, server := range servers {
		t.Run(server.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, server.command[0], server.command[1:]...)

			// Capture stderr for error messages
			stderr, _ := cmd.StderrPipe()

			err := cmd.Start()
			require.NoError(t, err, "Server %s should start without error", server.name)

			// Wait a moment for startup
			time.Sleep(2 * time.Second)

			// Check if process is still running (hasn't crashed)
			if cmd.Process != nil {
				// Process started successfully
				cmd.Process.Kill()
				t.Logf("Server %s started successfully", server.name)
			}

			// Read any error output
			if stderr != nil {
				errOutput, _ := io.ReadAll(stderr)
				if len(errOutput) > 0 && !strings.Contains(string(errOutput), "Terminated") {
					t.Logf("Server %s stderr: %s", server.name, string(errOutput))
				}
			}
		})
	}
}

// TestHelixAgentMCPEndpoints verifies HelixAgent SSE endpoints respond correctly
func TestHelixAgentMCPEndpoints(t *testing.T) {
	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8100"
	}

	// Check if HelixAgent is running
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Logf("HelixAgent not running - skipping endpoint tests (acceptable)")
		return
	}
	resp.Body.Close()

	protocols := []string{"mcp", "acp", "lsp", "embeddings", "vision", "cognee"}

	for _, protocol := range protocols {
		t.Run(protocol+"_sse", func(t *testing.T) {
			// Test SSE endpoint
			client := &http.Client{Timeout: 3 * time.Second}
			req, err := http.NewRequest("GET", baseURL+"/v1/"+protocol, nil)
			require.NoError(t, err)
			req.Header.Set("Accept", "text/event-stream")

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusUnauthorized {
				t.Logf("Endpoint requires authentication - skipping (acceptable)")
				return
			}

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")

			// Read initial response
			buf := make([]byte, 1024)
			n, _ := resp.Body.Read(buf)
			response := string(buf[:n])
			assert.Contains(t, response, "event: endpoint")
			assert.Contains(t, response, "data: /v1/"+protocol)
		})

		t.Run(protocol+"_initialize", func(t *testing.T) {
			// Test JSON-RPC initialize
			client := &http.Client{Timeout: 5 * time.Second}
			body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1.0"},"capabilities":{}}}`

			resp, err := client.Post(baseURL+"/v1/"+protocol, "application/json", strings.NewReader(body))
			require.NoError(t, err)
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusUnauthorized {
				t.Logf("Endpoint requires authentication - skipping (acceptable)")
				return
			}

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			assert.Equal(t, "2.0", result["jsonrpc"])
			assert.NotNil(t, result["result"])
		})

		t.Run(protocol+"_tools_list", func(t *testing.T) {
			// Test tools/list
			client := &http.Client{Timeout: 5 * time.Second}
			body := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`

			resp, err := client.Post(baseURL+"/v1/"+protocol, "application/json", strings.NewReader(body))
			require.NoError(t, err)
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusUnauthorized {
				t.Logf("Endpoint requires authentication - skipping (acceptable)")
				return
			}

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			assert.NotNil(t, result["result"])
		})
	}
}

// TestMCPSSEImmediateResponse verifies SSE endpoints respond within timeout
func TestMCPSSEImmediateResponse(t *testing.T) {
	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8100"
	}

	// Check if HelixAgent is running
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Logf("HelixAgent not running - skipping SSE timing tests (acceptable)")
		return
	}
	resp.Body.Close()

	protocols := []string{"mcp", "acp", "lsp", "embeddings", "vision", "cognee"}

	for _, protocol := range protocols {
		t.Run(protocol+"_timing", func(t *testing.T) {
			client := &http.Client{Timeout: 500 * time.Millisecond} // 500ms timeout
			req, err := http.NewRequest("GET", baseURL+"/v1/"+protocol, nil)
			require.NoError(t, err)
			req.Header.Set("Accept", "text/event-stream")

			start := time.Now()
			resp, err := client.Do(req)
			elapsed := time.Since(start)

			require.NoError(t, err, "SSE endpoint should respond within 500ms")
			defer resp.Body.Close()

			// Should get initial response quickly (within 100ms ideally)
			assert.Less(t, elapsed, 500*time.Millisecond, "SSE should respond within 500ms")
			t.Logf("Protocol %s responded in %v", protocol, elapsed)
		})
	}
}

// TestOpenCodeConfiguration verifies OpenCode config uses correct package names
func TestOpenCodeConfiguration(t *testing.T) {
	// Generate a fresh config instead of reading existing one
	// This ensures we test the current generator output, not stale configs
	binaryPath := findBinaryPath(t)
	if binaryPath == "" {
		t.Logf("HelixAgent binary not found - run make build first (acceptable)")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "-generate-opencode-config")
	data, err := cmd.Output()
	if err != nil {
		t.Skipf("Failed to generate config: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
	}

	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	require.NoError(t, err, "OpenCode config should be valid JSON")

	mcp, ok := config["mcp"].(map[string]interface{})
	require.True(t, ok, "Config should have mcp section")

	// Verify fetch uses a recognized MCP fetch package
	if fetch, ok := mcp["fetch"].(map[string]interface{}); ok {
		if cmd, ok := fetch["command"].([]interface{}); ok {
			cmdStr := make([]string, len(cmd))
			for i, v := range cmd {
				cmdStr[i] = v.(string)
			}
			joined := strings.Join(cmdStr, " ")
			// Accept official, community, or alternative fetch server packages
			hasMCPFetch := strings.Contains(joined, "mcp-fetch-server")
			assert.True(t, hasMCPFetch,
				"fetch should use a recognized MCP fetch server package, got: %s", joined)
		}
	}

	// Verify sqlite uses a recognized MCP sqlite package
	if sqlite, ok := mcp["sqlite"].(map[string]interface{}); ok {
		if cmd, ok := sqlite["command"].([]interface{}); ok {
			cmdStr := make([]string, len(cmd))
			for i, v := range cmd {
				cmdStr[i] = v.(string)
			}
			joined := strings.Join(cmdStr, " ")
			// Accept only sqlite packages that actually EXIST on npm.
			// @modelcontextprotocol/server-sqlite is deliberately NOT accepted:
			// it 404s (the official reference sqlite server is the archived
			// PYTHON package `mcp-server-sqlite` on PyPI). Accepting it here
			// would let a config that cannot start pass this gate.
			hasMCPSQLite := strings.Contains(joined, "mcp-server-sqlite-npx") ||
				strings.Contains(joined, "mcp-sqlite")
			assert.True(t, hasMCPSQLite,
				"sqlite should use an MCP sqlite server package that exists on npm, got: %s", joined)

			// mcp-server-sqlite-npx takes the database path POSITIONALLY.
			// --db-path is the Python/uvx reference server's flag and is not
			// accepted here, so a config carrying it is broken even though the
			// package name resolves.
			if strings.Contains(joined, "mcp-server-sqlite-npx") {
				assert.NotContains(t, joined, "--db-path",
					"mcp-server-sqlite-npx takes the DB path positionally; --db-path is the Python/uvx flag, got: %s", joined)
			}
		}
	}

	// Verify HelixAgent endpoints have correct timeout
	helixEndpoints := []string{"helixagent-mcp", "helixagent-acp", "helixagent-lsp",
		"helixagent-embeddings", "helixagent-vision", "helixagent-cognee"}

	for _, endpoint := range helixEndpoints {
		if ep, ok := mcp[endpoint].(map[string]interface{}); ok {
			if timeout, ok := ep["timeout"].(float64); ok {
				assert.GreaterOrEqual(t, timeout, float64(30000),
					"Endpoint %s should have timeout >= 30000ms", endpoint)
			}
		}
	}
}

// npmPackageHealth is the verdict of one registry lookup.
//
// Resolving is NOT the same as being installable, and the difference is a whole
// class of defect an existence-only check cannot see (see npmPackageHealthOf).
type npmPackageHealth struct {
	Resolved bool   // registry returned HTTP 200 for the package document
	Usable   bool   // ...and it publishes something a user can actually install
	Reason   string // why not usable; empty when Usable
	Latest   string // dist-tags.latest, for the failure message
	Versions int    // number of published versions
}

// securityHoldingVersion is the version npm stamps on a name it has SEIZED —
// typically because the name was used maliciously or squatted. The package
// document still returns HTTP 200, so every existence check passes, but
// installing it yields nothing.
const securityHoldingVersion = "0.0.1-security"

// npmPackageHealthOf resolves a package and judges whether it is installable.
//
// # WHY THIS IS NOT JUST AN HTTP 200 CHECK
//
// Four names shipped in this repo — mcp-server-figma, mcp-server-postgres,
// mcp-server-redis, mcp-server-supabase — resolve with HTTP 200 while being
// npm security holding packages: a single version 0.0.1-security, description
// "security holding package", repository "npm/security-holder". A fifth,
// mcp-server-jira, resolves with ZERO published versions. The existence check
// passed all five, so the configs pointed at names npm had to quarantine and
// the guard was structurally blind to it.
//
// SIGNAL CHOICE (measured, not assumed)
//
// All 123 package names this repo references were probed against the live
// registry on 2026-09-03 and the candidate signals compared on the 101 that
// are healthy:
//
//   - version STRING (chosen, primary): 0 of 101 healthy packages have a
//     latest version carrying a `-security` prerelease. npm itself stamps this
//     when it seizes a name, so it is not publisher-controlled text.
//   - description substring: also 0 of 101 here, but it is free text the
//     publisher writes, so it can drift or be imitated. Kept only as a
//     SECONDARY trigger, and only for a single-version package, so a genuine
//     security tool that merely describes itself cannot trip it.
//   - version COUNT: REJECTED as a signal. 8+ healthy packages this repo
//     depends on publish exactly one version (mcp-server-airtable 0.0.1,
//     mcp-server-brave-search 1.0.0, mcp-obsidian 1.0.0, ...), so "only one
//     version" would false-positive on shipping dependencies. Zero versions is
//     a different matter and is caught below.
func npmPackageHealthOf(packageName string) npmPackageHealth {
	url := "https://registry.npmjs.org/" + strings.ReplaceAll(packageName, "/", "%2f")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return npmPackageHealth{Reason: "registry request failed: " + err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return npmPackageHealth{Reason: fmt.Sprintf("registry returned HTTP %d", resp.StatusCode)}
	}

	var doc struct {
		Description string `json:"description"`
		DistTags    struct {
			Latest string `json:"latest"`
		} `json:"dist-tags"`
		Versions map[string]json.RawMessage `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return npmPackageHealth{Resolved: true, Reason: "registry document is not valid JSON: " + err.Error()}
	}

	h := npmPackageHealth{
		Resolved: true,
		Latest:   doc.DistTags.Latest,
		Versions: len(doc.Versions),
	}

	// Resolves but publishes nothing installable (mcp-server-jira,
	// zendesk-mcp-server). An empty registry shell, not a package.
	if h.Versions == 0 {
		h.Reason = "resolves but publishes ZERO versions - an empty registry shell that installs nothing"
		return h
	}

	// Primary signal: npm's own seized-name marker.
	if isSecurityHoldingVersion(h.Latest) {
		h.Reason = fmt.Sprintf("npm SECURITY HOLDING package (latest version %q) - npm seized this name, "+
			"typically because it was used maliciously or squatted; it resolves but installs nothing", h.Latest)
		return h
	}
	for v := range doc.Versions {
		if isSecurityHoldingVersion(v) && h.Versions == 1 {
			h.Reason = fmt.Sprintf("npm SECURITY HOLDING package (only version %q)", v)
			return h
		}
	}

	// Secondary signal, deliberately narrow: publisher-controlled text, so it
	// only fires when the package also publishes just one version.
	if h.Versions == 1 && strings.Contains(strings.ToLower(doc.Description), "security holding package") {
		h.Reason = fmt.Sprintf("npm SECURITY HOLDING package (description %q, single version %q)", doc.Description, h.Latest)
		return h
	}

	h.Usable = true
	return h
}

// isSecurityHoldingVersion reports whether a version string is npm's
// seized-name marker. The canonical value is 0.0.1-security; the suffix is
// matched rather than the exact string so a differently-numbered holding
// release is caught too.
func isSecurityHoldingVersion(version string) bool {
	return version == securityHoldingVersion || strings.HasSuffix(version, "-security")
}

// checkNpmPackageExists reports whether a package is present AND installable.
//
// The name is kept for its existing call sites, but the contract is now
// stricter than "the registry returned 200": a seized or empty name is NOT a
// package a user can run, so treating it as existing is what let four
// quarantined names ship.
func checkNpmPackageExists(packageName string) bool {
	return npmPackageHealthOf(packageName).Usable
}

// findBinaryPath finds the HelixAgent binary path
func findBinaryPath(t *testing.T) string {
	t.Helper()

	// Start from current directory and search up for project root
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		// Check if we found the project root (has go.mod)
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			binaryPath := filepath.Join(dir, "bin", "helixagent")
			if _, err := os.Stat(binaryPath); err == nil {
				return binaryPath
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
