package ports

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setPrefix sets HELIXAGENT_PORT_PREFIX, resets the cached
// prefix, and registers a cleanup to restore prior state.
func setPrefix(t *testing.T, value string) {
	t.Helper()
	prev, had := os.LookupEnv(EnvPrefix)
	require.NoError(t, os.Setenv(EnvPrefix, value))
	PrefixResetForTest()
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(EnvPrefix, prev)
		} else {
			_ = os.Unsetenv(EnvPrefix)
		}
		PrefixResetForTest()
	})
}

func TestPrefix_DefaultIsEight(t *testing.T) {
	require.NoError(t, os.Unsetenv(EnvPrefix))
	PrefixResetForTest()
	t.Cleanup(PrefixResetForTest)

	assert.Equal(t, 8, Prefix(),
		"with no env var set, Prefix() must be DefaultPrefix (8)")
}

func TestPrefix_EnvNineShiftsBand(t *testing.T) {
	setPrefix(t, "9")
	assert.Equal(t, 9, Prefix(),
		"HELIXAGENT_PORT_PREFIX=9 must select the 9xxx band")
	assert.Equal(t, 9100, Get(HelixAgentHTTP),
		"HelixAgentHTTP in prefix-9 band should be 9100")
	assert.Equal(t, 9200, Get(MCPFetch),
		"MCPFetch in prefix-9 band should be 9200")
	assert.Equal(t, 9300, Get(ACPManager),
		"ACPManager in prefix-9 band should be 9300")
}

func TestPrefix_InvalidValuesFallBackToDefault(t *testing.T) {
	cases := []string{"7", "10", "0", "-1", "abc", "", "  "}
	for _, c := range cases {
		c := c
		t.Run(fmt.Sprintf("value=%q", c), func(t *testing.T) {
			setPrefix(t, c)
			assert.Equal(t, DefaultPrefix, Prefix(),
				"invalid prefix %q must fall back to DefaultPrefix",
				c)
		})
	}
}

func TestGet_DefaultsMatchCanonicalMap(t *testing.T) {
	setPrefix(t, "8")

	cases := map[Service]int{
		// Core band 81xx.
		HelixAgentHTTP:  8100,
		PostgresPrimary: 8101,
		RedisDefault:    8102,
		MCPBridge:       8103,
		MCPRouterAlt:    8104,
		HelixLLM:        8105,
		MockLLM:         8106,
		PostgresReplica: 8107,
		PostgresExtra:   8108,
		PostgresTest:    8109,
		RedisMCP:        8110,
		HelixAgentGRPC:  8112,
		Cognee:          8120,
		ChromaDB:        8121,
		Qdrant:          8122,
		Neo4jHTTP:       8123,
		Neo4jBolt:       8124,
		// MCP tiers (spot checks).
		MCPFetch:              8200,
		MCPPostgres:           8209,
		MCPMongoDB:            8210,
		MCPSupabase:           8214,
		MCPQdrant:             8215,
		MCPWeaviate:           8218,
		MCPGitHub:             8220,
		MCPK8sAlt:             8233,
		MCPPlaywright:         8234,
		MCPCrawl4AI:           8237,
		MCPSlack:              8238,
		MCPTelegram:           8240,
		MCPNotion:             8250,
		MCPAtlassian:          8259,
		MCPBraveSearch:        8260,
		MCPOpenAI:             8269,
		MCPGDrive:             8270,
		MCPGmail:              8274,
		MCPDatadog:            8275,
		MCPPrometheus:         8277,
		MCPStripe:             8278,
		MCPZendesk:            8280,
		MCPFigma:              8281,
		MCPSequentialThinking: 8206,
		// Observability band 83xx.
		ACPManager:       8300,
		ElasticsearchAlt: 8301,
		OpenSearch:       8302,
		SignozOTel:       8303,
		Prometheus:       8310,
		Grafana:          8311,
		Jaeger:           8312,
	}

	for svc, want := range cases {
		// Ensure env var is NOT set so Get() returns the default.
		_ = os.Unsetenv(string(svc))
		got := Get(svc)
		assert.Equal(t, want, got,
			"Get(%s) at prefix=8 should be %d, got %d",
			svc, want, got)
	}
}

func TestGet_EnvOverrideWins(t *testing.T) {
	setPrefix(t, "8")

	const custom = 9876
	require.NoError(t, os.Setenv(string(HelixAgentHTTP),
		strconv.Itoa(custom)))
	t.Cleanup(func() {
		_ = os.Unsetenv(string(HelixAgentHTTP))
	})

	assert.Equal(t, custom, Get(HelixAgentHTTP),
		"env override must win over default")
	assert.Equal(t, 8100, Default(HelixAgentHTTP),
		"Default() must ignore env override")
}

func TestGet_InvalidEnvFallsBackToDefault(t *testing.T) {
	setPrefix(t, "8")

	cases := map[string]int{
		"not-a-number": 8100,
		"0":            8100, // reject 0
		"65536":        8100, // out of 16-bit range
		"-1":           8100,
	}
	for raw, want := range cases {
		raw, want := raw, want
		t.Run(fmt.Sprintf("raw=%q", raw), func(t *testing.T) {
			require.NoError(t, os.Setenv(
				string(HelixAgentHTTP), raw))
			t.Cleanup(func() {
				_ = os.Unsetenv(string(HelixAgentHTTP))
			})
			assert.Equal(t, want, Get(HelixAgentHTTP),
				"invalid override %q must fall back", raw)
		})
	}
}

func TestGet_UnregisteredServicePanics(t *testing.T) {
	assert.Panics(t, func() {
		Get(Service("NOT_REGISTERED"))
	}, "Get() on unregistered Service must panic")
	assert.Panics(t, func() {
		Default(Service("ALSO_NOT_REGISTERED"))
	}, "Default() on unregistered Service must panic")
}

func TestHelpers_FormatsAreCorrect(t *testing.T) {
	setPrefix(t, "8")
	_ = os.Unsetenv(string(HelixAgentHTTP))

	assert.Equal(t, ":8100", Addr(HelixAgentHTTP))
	assert.Equal(t, "localhost:8100",
		HostPort(HelixAgentHTTP, "localhost"))
	assert.Equal(t, "http://localhost:8100",
		HTTPURL(HelixAgentHTTP, "localhost"))
	assert.Equal(t, "https://localhost:8100",
		HTTPSURL(HelixAgentHTTP, "localhost"))
}

func TestAll_ReturnsEveryRegisteredService(t *testing.T) {
	services := All()
	assert.Equal(t, len(offsets), len(services),
		"All() must return exactly len(offsets) services")

	seen := make(map[Service]struct{})
	for _, s := range services {
		seen[s] = struct{}{}
	}
	for s := range offsets {
		_, ok := seen[s]
		assert.True(t, ok,
			"All() must include registered service %s", s)
	}
}

func TestExport_IncludesPrefixAndAllServices(t *testing.T) {
	setPrefix(t, "8")
	m := Export()

	assert.Equal(t, "8", m[EnvPrefix],
		"Export() must include the active prefix")
	assert.Len(t, m, len(offsets)+1,
		"Export() should have one entry per service plus prefix")
	assert.Equal(t, "8100", m[string(HelixAgentHTTP)])
	assert.Equal(t, "8209", m[string(MCPPostgres)])
	assert.Equal(t, "8300", m[string(ACPManager)])
}

func TestExport_ReflectsEnvOverride(t *testing.T) {
	setPrefix(t, "8")
	require.NoError(t, os.Setenv(string(RedisDefault), "9999"))
	t.Cleanup(func() { _ = os.Unsetenv(string(RedisDefault)) })

	m := Export()
	assert.Equal(t, "9999", m[string(RedisDefault)],
		"Export() must honor env overrides")
	assert.Equal(t, "8100", m[string(HelixAgentHTTP)],
		"unrelated services must retain their defaults")
}

// TestOffsets_NoCollisions is the critical invariant: every
// service MUST have a unique offset. Two services with the same
// offset would map to the same default port — a runtime conflict.
func TestOffsets_NoCollisions(t *testing.T) {
	inverse := make(map[int][]Service)
	for svc, off := range offsets {
		inverse[off] = append(inverse[off], svc)
	}

	var collisions []string
	for off, svcs := range inverse {
		if len(svcs) > 1 {
			names := make([]string, len(svcs))
			for i, s := range svcs {
				names[i] = string(s)
			}
			sort.Strings(names)
			collisions = append(collisions,
				fmt.Sprintf("offset %d → %v", off, names))
		}
	}
	sort.Strings(collisions)
	assert.Empty(t, collisions,
		"offsets must be unique per service; collisions found: %v",
		collisions)
}

// TestOffsets_FitIn16BitAtBothPrefixes guards against drift —
// any offset large enough that Prefix()*1000+offset exceeds
// 65535 would be invalid regardless of which prefix we pick.
func TestOffsets_FitIn16BitAtBothPrefixes(t *testing.T) {
	for _, p := range []int{8, 9} {
		for svc, off := range offsets {
			port := p*1000 + off
			assert.Less(t, port, 65536,
				"port for %s at prefix %d (%d) exceeds 16 bits",
				svc, p, port)
			assert.Greater(t, port, 0,
				"port for %s at prefix %d (%d) must be >0",
				svc, p, port)
		}
	}
}

// TestOffsets_WithinExpectedBands guards band discipline:
// core ≤ 199, MCP = 200–281, observability = 300–312.
func TestOffsets_WithinExpectedBands(t *testing.T) {
	coreServices := map[Service]struct{}{
		HelixAgentHTTP: {}, PostgresPrimary: {}, RedisDefault: {},
		MCPBridge: {}, MCPRouterAlt: {}, HelixLLM: {},
		MockLLM: {}, PostgresReplica: {}, PostgresExtra: {},
		PostgresTest: {}, RedisMCP: {},
		// HelixAgentLiveness — early-bind liveness probe (offset 111),
		// added 2026-04-25 alongside the verification-window /health
		// fix. Belongs in the core band, not MCP.
		HelixAgentLiveness: {},
		// HelixAgentGRPC — the gRPC API served by cmd/grpc-server
		// (offset 112), registered 2026-08-09 when it was found
		// hardcoded to :50051, a port this project's own Weaviate
		// container publishes. It is core HelixAgent surface, not an
		// MCP server, so it belongs in the 100-199 band.
		HelixAgentGRPC: {},
		Cognee:         {},
		ChromaDB:       {}, Qdrant: {}, Neo4jHTTP: {}, Neo4jBolt: {},
	}
	observability := map[Service]struct{}{
		ACPManager: {}, ElasticsearchAlt: {}, OpenSearch: {},
		SignozOTel: {}, Prometheus: {}, Grafana: {}, Jaeger: {},
	}

	for svc, off := range offsets {
		switch {
		case func() bool { _, ok := coreServices[svc]; return ok }():
			assert.InDelta(t, 150, off, 50,
				"core %s offset %d must be in 100-199",
				svc, off)
		case func() bool { _, ok := observability[svc]; return ok }():
			assert.InDelta(t, 306, off, 6,
				"observability %s offset %d must be in 300-312",
				svc, off)
		default:
			// everything else is MCP band: 200-281.
			assert.GreaterOrEqual(t, off, 200,
				"MCP %s offset %d must be >= 200", svc, off)
			assert.LessOrEqual(t, off, 281,
				"MCP %s offset %d must be <= 281", svc, off)
		}
	}
}

// TestEnvVarNames_AllStartWithHelixAgentPrefix enforces the
// canonical env-var naming rule.
func TestEnvVarNames_AllStartWithHelixAgentPrefix(t *testing.T) {
	for svc := range offsets {
		assert.True(t,
			len(string(svc)) > len("HELIXAGENT_PORT_") &&
				string(svc)[:len("HELIXAGENT_PORT_")] ==
					"HELIXAGENT_PORT_",
			"Service %q must start with HELIXAGENT_PORT_", svc)
	}
}
