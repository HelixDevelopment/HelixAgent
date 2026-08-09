package testutil

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// §11.4.115 RED-baseline-on-the-broken-artifact + polarity switch.
//
// These tests reproduce the Redis port-coupling defect on the CURRENT,
// pre-fix artifact and, with the polarity flipped, become the standing
// GREEN regression guard (§11.4.135) that the defect stays absent.
//
//	RED_MODE=1 — assert the DEFECT IS PRESENT (reproduce on a pre-fix tree).
//	RED_MODE=0 — assert the defect is ABSENT. THIS IS NOW THE DEFAULT.
//
// The default was RED while the defect was live and flipped to GREEN in the
// same commit as the fix, per §11.4.115: one source, two roles — the
// bug-catcher IS the regression guard. To re-run the RED role against the
// pre-fix artifact: `git stash && RED_MODE=1 go test ./internal/testutil/`.
//
// CAPTURED RED BASELINE (2026-08-09, on the pre-fix tree):
//
//	RED_MODE=1 -> PASS  (defect reproduced)
//	RED_MODE=0 -> FAIL:
//	  TestRedisGate_ResolvesCanonicalRedisService
//	      expected: "8102"   actual: "8110"
//	  TestRedisGate_RejectsNonRedisListener
//	      Should be false  (bare-TCP gate accepted a non-Redis listener)
//
// Both assertions FAILED before the fix and PASS after it, so neither is a
// blind test that merely agrees with whatever the code does.
//
// Both polarities are infra-free and deterministic (§11.4.50): they bind
// only ephemeral loopback ports they allocate themselves, and never depend
// on a live Redis being up or down.
//
// THE DEFECT (measured 2026-08-08, suite log
// qa-results/helixagent_suite_20260808T171748Z.log):
//
//	--- FAIL: TestCheckRedisHealth (2.31s)
//	    main_test.go:1146: failed to connect to Redis:
//	                       dial tcp 127.0.0.1:6379: connect: connection refused
//
// That test DOES call testutil.RequireRedis(t) first — the guard is present
// and it did not skip. It did not skip because the guard and the code under
// test resolve DIFFERENT endpoints from the SAME env var:
//
//	guard   (DefaultInfraConfig) REDIS_PORT ?: 8110  = ports.RedisMCP
//	subject (checkRedisHealth)   REDIS_PORT ?: 6379  = pre-CONST-027 legacy
//
// With REDIS_PORT unset — the state of this host, which has no .env — the
// guard probed the password-protected MCP-backend Redis on 8110 (up), so it
// reported "available", and the subject then dialled 6379 (down) and failed.
// A guard that does not assert the condition its test depends on is a
// §11.4.201(1) false-negative gate: it let the test proceed into a
// guaranteed failure, which was then reported as a product defect.
//
// The canonical value is neither of those two. internal/ports (CONST-027)
// registers RedisDefault at offset 102 -> 8102, .env.example:276 documents
// REDIS_PORT=8102, and helix_agent's own docker-compose.yml:53 publishes
// "${REDIS_PORT:-8102}:6379". 8110 is RedisMCP — a DIFFERENT service.

// redModeEnabled reports whether the RED polarity is active. Opt-in, because
// the fix has landed and the GREEN guard is the role that ships.
func redModeEnabled() bool {
	return os.Getenv("RED_MODE") == "1"
}

// TestRedisGate_ResolvesCanonicalRedisService asserts that the availability
// gate points at the Redis the product actually uses (ports.RedisDefault),
// rather than at the password-protected MCP-backend Redis (ports.RedisMCP).
//
// This is a §11.4.111 resolve-by-stable-name defect: the gate was migrated
// off the legacy 6379 literal onto an 81xx registry port, but onto the entry
// for the WRONG service. infra_test.go:25 records the mis-binding in its own
// comment — `assert.Equal(t, "8110", cfg.RedisPort) // HELIXAGENT_PORT_REDIS_MCP`.
func TestRedisGate_ResolvesCanonicalRedisService(t *testing.T) {
	t.Setenv("REDIS_HOST", "")
	t.Setenv("REDIS_PORT", "")

	canonical := strconv.Itoa(ports.Get(ports.RedisDefault))
	mcpBackend := strconv.Itoa(ports.Get(ports.RedisMCP))
	require.NotEqual(t, canonical, mcpBackend,
		"registry sanity: RedisDefault and RedisMCP must be distinct services")

	got := DefaultInfraConfig().RedisPort

	if redModeEnabled() {
		assert.Equal(t, mcpBackend, got,
			"RED baseline: the gate is expected to still be mis-bound to "+
				"ports.RedisMCP (the password-protected MCP-backend Redis). "+
				"If this assertion fails, the defect is already fixed — "+
				"re-run with RED_MODE=0.")
		assert.NotEqual(t, canonical, got,
			"RED baseline: the gate must NOT yet resolve the canonical Redis")
		return
	}

	assert.Equal(t, canonical, got,
		"the availability gate MUST resolve the same Redis service the "+
			"product dials (ports.RedisDefault, CONST-027), so a passing "+
			"guard actually implies the subject can connect")
}

// TestRedisGate_RejectsNonRedisListener is the golden-bad fixture that
// self-validates the gate itself (§11.4.107(10)).
//
// RedisAvailable() claims "Redis is reachable". Pre-fix it performed a bare
// TCP dial, so ANY process holding the port satisfied it. That is not a
// theoretical hole on this host: ports.RedisMCP (8110) is served by
// helixagent-mcp-redis-backend, which is password-protected and answers a
// bare PING with "-NOAUTH Authentication required." — measured 2026-08-08.
// A TCP-only probe reports that endpoint as available even though every
// command against it fails, so the gate cannot distinguish "Redis I can
// use" from "a socket that happens to be open".
//
// A guard must assert the condition it CLAIMS (§11.4.201). The fixture below
// is deliberately NOT Redis — it accepts the connection and says nothing —
// so a correct gate must reject it.
func TestRedisGate_RejectsNonRedisListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "golden-bad fixture: could not bind loopback listener")
	t.Cleanup(func() { _ = ln.Close() })

	// Accept and hold connections without ever speaking the Redis protocol.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			// Hold briefly, answer nothing, then drop. This is what a
			// non-Redis occupant of the port looks like on the wire.
			go func(c net.Conn) {
				time.Sleep(200 * time.Millisecond)
				_ = c.Close()
			}(conn)
		}
	}()

	host, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	t.Setenv("REDIS_HOST", host)
	t.Setenv("REDIS_PORT", port)
	t.Setenv("REDIS_PASSWORD", "")
	resetInfraCacheForTest()

	got := RedisAvailable()

	if redModeEnabled() {
		assert.True(t, got,
			"RED baseline: the bare-TCP gate is expected to accept a "+
				"non-Redis listener. If this fails, the gate already "+
				"verifies the Redis protocol — re-run with RED_MODE=0.")
		return
	}

	assert.False(t, got,
		"RedisAvailable() must reject an endpoint that is reachable but "+
			"does not answer the Redis protocol; a TCP-open check cannot "+
			"tell a usable Redis from a NOAUTH-rejecting or foreign occupant")
}

// =============================================================================
// RESP-speaking fixtures for the AUTH+PING handshake
//
// TestRedisGate_RejectsNonRedisListener above covers exactly ONE failure
// shape: a listener that accepts the connection and then says NOTHING. That
// is the cheap half of the space, and it leaves the gate's headline mechanism
// — the handshake itself — unguarded. Proof (§1.1): weakening checkRedisPing
// to accept ANY reply
//
//	resp, ok := reply("PING")   ->   _, ok := reply("PING")
//	if !ok { return false }          if !ok { return false }
//	return strings.HasPrefix(resp, "+PONG")   ->   return true
//
// keeps that test GREEN, because a silent listener still fails the read. The
// whole point of the handshake is to reject a listener that DOES reply but is
// unusable, and nothing exercised that.
//
// It is not a theoretical shape either. Measured on this host 2026-08-08:
// helixagent-mcp-redis-backend on HELIXAGENT_PORT_REDIS_MCP is started with
// --requirepass and answers a bare PING with
//
//	-NOAUTH Authentication required.
//
// and helix_agent's own docker-compose.yml starts the canonical Redis with
// --requirepass too. "Replied, therefore up" calls both of those available
// while every command against them fails.
//
// The fixtures below close that: a golden-BAD that replies -NOAUTH (and a
// -WRONGPASS sibling), and the golden-GOOD the gate also lacked — because a
// one-sided suite is defeated by the opposite mutation. With only golden-bad
// fixtures, `func checkRedisPing(...) bool { return false }` passes
// everything, and RequireRedis would silently skip every Redis test forever.
// =============================================================================

// fakeRedis is a loopback listener speaking just enough RESP to answer the
// inline AUTH and PING commands checkRedisPing sends. It records what it
// received so a test can prove the gate really authenticated rather than
// merely getting lucky.
type fakeRedis struct {
	addr string

	mu       sync.Mutex
	received []string
}

// startFakeRedis binds an ephemeral loopback port and answers each inline
// command with reply(cmd). Returns once the listener is accepting.
func startFakeRedis(t *testing.T, reply func(cmd string) string) *fakeRedis {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "fixture: could not bind loopback listener")
	t.Cleanup(func() { _ = ln.Close() })

	f := &fakeRedis{addr: ln.Addr().String()}

	go func() {
		for {
			conn, aerr := ln.Accept()
			if aerr != nil {
				return // listener closed by t.Cleanup
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				// Bounded so a fixture can never outlive its test.
				_ = c.SetDeadline(time.Now().Add(5 * time.Second))
				r := bufio.NewReader(c)
				for {
					line, rerr := r.ReadString('\n')
					if rerr != nil {
						return
					}
					cmd := strings.TrimRight(line, "\r\n")
					f.record(cmd)
					if _, werr := c.Write([]byte(reply(cmd))); werr != nil {
						return
					}
				}
			}(conn)
		}
	}()

	return f
}

func (f *fakeRedis) record(cmd string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.received = append(f.received, cmd)
}

func (f *fakeRedis) commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.received...)
}

// pointGateAt aims the availability gate at the fixture and clears the
// once-per-run verdict cache so this test observes its own endpoint.
func pointGateAt(t *testing.T, f *fakeRedis, password string) {
	t.Helper()
	host, port, err := net.SplitHostPort(f.addr)
	require.NoError(t, err)
	t.Setenv("REDIS_HOST", host)
	t.Setenv("REDIS_PORT", port)
	t.Setenv("REDIS_PASSWORD", password)
	resetInfraCacheForTest()
	t.Cleanup(resetInfraCacheForTest)
}

// --- golden-bad: replies, but is unusable ---------------------------------

// TestRedisGate_RejectsNoAuthListener is the golden-bad fixture for the
// measured failure mode: a password-protected Redis probed without
// credentials answers every command with "-NOAUTH Authentication required."
//
// This endpoint is REACHABLE and it REPLIES. Only a gate that inspects the
// reply can reject it — which is precisely the assertion the commit's
// headline mechanism exists to make, and precisely what no test covered.
func TestRedisGate_RejectsNoAuthListener(t *testing.T) {
	f := startFakeRedis(t, func(string) string {
		return "-NOAUTH Authentication required.\r\n"
	})
	// No password configured — exactly the state of a host with no .env,
	// which is how this was hit for real.
	pointGateAt(t, f, "")

	got := RedisAvailable()

	if redModeEnabled() {
		assert.True(t, got,
			"RED baseline: a reply-means-up gate is expected to accept a "+
				"-NOAUTH endpoint. If this fails, the gate already inspects "+
				"the reply — re-run with RED_MODE=0.")
		return
	}

	assert.False(t, got,
		"RedisAvailable() must reject an endpoint that answers -NOAUTH: it is "+
			"reachable and it replies, but every command against it fails. "+
			"Reporting it available is what let guarded tests proceed into a "+
			"guaranteed failure and be read as product defects")

	assert.Contains(t, f.commands(), "PING",
		"the gate must actually have spoken to the endpoint; a verdict "+
			"reached without sending anything would be a different bug that "+
			"happens to produce the right answer here")
}

// TestRedisGate_RejectsWrongPasswordListener is the sibling shape: credentials
// were supplied but rejected. Same class — replies, unusable — and it also
// proves the gate does not treat "AUTH got an answer" as success.
func TestRedisGate_RejectsWrongPasswordListener(t *testing.T) {
	f := startFakeRedis(t, func(cmd string) string {
		if strings.HasPrefix(cmd, "AUTH ") {
			return "-WRONGPASS invalid username-password pair\r\n"
		}
		return "-NOAUTH Authentication required.\r\n"
	})
	pointGateAt(t, f, "definitely-not-the-password")

	got := RedisAvailable()

	if redModeEnabled() {
		assert.True(t, got, "RED baseline: reply-means-up accepts -WRONGPASS")
		return
	}

	assert.False(t, got,
		"a Redis that rejects our credentials is not a Redis we can use; "+
			"an available verdict here sends the caller into a failure whose "+
			"cause is invisible at the call site")
}

// --- golden-good: a real handshake must still be accepted -----------------

// TestRedisGate_AcceptsAuthenticatedRedis is the golden-good fixture.
//
// Without it the golden-bad tests above are satisfied by a gate that rejects
// EVERYTHING — `return false` — which would make RequireRedis skip every
// Redis test in the repo forever while every one of these tests stayed green.
// That is the same PASS-bluff as the defect, just inverted.
//
// It also asserts the gate sent the CONFIGURED password, so "authenticates"
// is verified rather than assumed.
func TestRedisGate_AcceptsAuthenticatedRedis(t *testing.T) {
	const password = "helixagent-test-secret"

	f := startFakeRedis(t, func(cmd string) string {
		switch {
		case cmd == "AUTH "+password:
			return "+OK\r\n"
		case cmd == "PING":
			return "+PONG\r\n"
		default:
			return "-ERR unexpected command\r\n"
		}
	})
	pointGateAt(t, f, password)

	assert.True(t, RedisAvailable(),
		"a Redis that completes AUTH+PING MUST be reported available; a gate "+
			"that rejects everything skips the entire Redis suite forever and "+
			"passes every golden-bad test while doing it")

	assert.Contains(t, f.commands(), "AUTH "+password,
		"the gate must AUTHENTICATE with the configured credentials, not just "+
			"open a socket and hope; without this assertion a gate that never "+
			"sends AUTH still passes whenever the server does not require one")
	assert.Contains(t, f.commands(), "PING")
}

// TestRedisGate_AcceptsUnauthenticatedRedis covers the other legitimate
// deployment: a Redis started without --requirepass. AUTH must be skipped
// (sending it would draw "-ERR Client sent AUTH, but no password is set")
// and the bare PING must carry the verdict.
func TestRedisGate_AcceptsUnauthenticatedRedis(t *testing.T) {
	f := startFakeRedis(t, func(cmd string) string {
		if cmd == "PING" {
			return "+PONG\r\n"
		}
		return "-ERR Client sent AUTH, but no password is set\r\n"
	})
	pointGateAt(t, f, "")

	assert.True(t, RedisAvailable(),
		"a password-less Redis answering +PONG is usable and must be accepted")

	for _, cmd := range f.commands() {
		assert.False(t, strings.HasPrefix(cmd, "AUTH"),
			"no password is configured, so the gate must not send AUTH")
	}
}

// =============================================================================
// DELIBERATELY-UNMIGRATED RedisAvailable CALLERS
//
// 6fbf6282 claimed guard and subject "agree by construction". That holds for
// the guard's own resolution — DefaultInfraConfig goes through ports.Redis*,
// the same helpers the product uses — but it is NOT true of every CALLER. A
// caller whose subject re-derives its own endpoint disagrees with the guard
// no matter how correct the guard is, and three do.
//
// Leaving them undocumented would make the commit's claim read as universal
// when it is not, so each is pinned below with its rationale. Documentation
// alone rots, so TestUnmigratedRedisCallers_LedgerIsCurrent asserts each
// entry still describes the file it names: migrate a caller and the ledger
// FAILS until it is updated (§11.4.120 — reconcile the gate, never
// fake-pass it).
//
// WHY THESE ARE DOCUMENTED RATHER THAN RECONCILED HERE:
//
// Entries 1 and 2 are one-line changes in files outside this change's scope;
// they are handed on rather than reached into, so a concurrent edit is not
// clobbered. Entry 3 is different and worth reading before "fixing" it: the
// obvious reconciliation — teach this package's gate to honour REDIS_URL —
// would RE-CREATE the very defect 6fbf6282 closed. internal/ports, which the
// PRODUCT resolves through, does not read REDIS_URL. Teaching only the guard
// to read it makes guard and product resolve different endpoints again. The
// correct fix is in internal/ports (product behaviour) or in mock_checker.go
// (its precondition), never here.
// =============================================================================

// unmigratedRedisCaller pins one call site that guards with RedisAvailable
// but whose subject resolves a DIFFERENT endpoint.
type unmigratedRedisCaller struct {
	// Path is repo-root-relative.
	Path string
	// Marker is a literal that is present while the divergence is present.
	// When it disappears the caller was (probably) migrated and this entry
	// must be re-checked and removed.
	Marker string
	// Why records the divergence and its consequence.
	Why string
}

var unmigratedRedisCallers = []unmigratedRedisCaller{
	{
		Path:   "tests/integration/infrastructure_integration_test.go",
		Marker: `infraGetEnvOrDefault("REDIS_PORT", "6379")`,
		Why: "Four TestIntegration_Redis_* tests guard with testutil.RequireRedis " +
			"(ports.RedisDefault + REDIS_PASSWORD) while their subjects build a " +
			"config.RedisConfig from REDIS_PORT defaulted to 6379 with the " +
			"password baked in as \"helixagent123\". Both directions leak: with " +
			"REDIS_PORT unset the guard probes the canonical port and the " +
			"subject dials 6379, and with compose Redis up on the canonical " +
			"port but REDIS_PASSWORD unset the guard draws -NOAUTH and skips " +
			"four tests whose subjects carry the password and would have " +
			"connected. Fix: drop the local defaults and resolve through " +
			"testutil.RedisAddr / testutil.RedisPassword.",
	},
	{
		Path:   "internal/mcp/servers/redis_adapter_test.go",
		Marker: "Port:     6379,",
		Why: "TestRedisAdapter_Integration guards with testutil.RequireRedis but " +
			"hardcodes Host \"localhost\", Port 6379 and an empty password in " +
			"its RedisAdapterConfig, so the guard vouches for a different " +
			"service than the subject dials. Its own adapter.Initialize error " +
			"path then skips, so the mismatch degrades to a silent skip rather " +
			"than a failure — coverage vanishing quietly. Fix: resolve the " +
			"adapter config through ports.RedisHost / ports.RedisPort / " +
			"ports.RedisPassword.",
	},
	{
		Path:   "tests/testutils/mock_checker.go",
		Marker: `os.Getenv("REDIS_URL")`,
		Why: "IsRedisAvailable accepts EITHER REDIS_HOST or REDIS_URL as its " +
			"precondition, then delegates to testutil.RedisAvailable, which " +
			"only ever dials RedisHost():RedisPort() and ignores the URL " +
			"entirely. An operator who exports only REDIS_URL therefore gets " +
			"\"Redis not available. Set REDIS_HOST or REDIS_URL\" while " +
			"REDIS_URL IS set — the guard blaming the operator for the guard's " +
			"own blind spot. GetMockConfig's RedisURL default also still " +
			"carries the pre-CONST-027 port 16379. Fix belongs in " +
			"internal/ports (so product and guard both honour REDIS_URL) or in " +
			"this file's precondition — NOT in this package alone, which would " +
			"re-split guard and subject.",
	},
}

// repoFileContains reports whether the repo-root-relative path contains
// marker. Split out so the ledger check's own detector can be validated
// (§11.4.107(10)) rather than trusted.
func repoFileContains(relPath, marker string) (bool, error) {
	// This package lives at <repo>/internal/testutil.
	data, err := os.ReadFile(filepath.Join("..", "..", relPath))
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), marker), nil
}

// TestUnmigratedRedisCallers_LedgerIsCurrent gives the ledger teeth.
//
// Without it the list is a comment: a caller could be migrated (or renamed,
// or deleted) and the ledger would keep asserting a divergence that no
// longer exists, which is its own small bluff. A FAILURE here is not
// necessarily a defect — it most likely means someone did the right thing
// and the entry should now be removed.
func TestUnmigratedRedisCallers_LedgerIsCurrent(t *testing.T) {
	require.NotEmpty(t, unmigratedRedisCallers,
		"an empty ledger must mean every caller was migrated, not that the "+
			"ledger was quietly emptied to make this test pass")

	for _, entry := range unmigratedRedisCallers {
		t.Run(entry.Path, func(t *testing.T) {
			found, err := repoFileContains(entry.Path, entry.Marker)
			require.NoError(t, err,
				"ledger entry names a file that cannot be read; if the caller "+
					"was moved or deleted, update or remove this entry")

			assert.True(t, found,
				"marker %q is gone from %s.\n\nThis entry documents a KNOWN "+
					"guard/subject endpoint divergence:\n\n  %s\n\nIf you "+
					"migrated this caller: delete the entry. If the marker "+
					"merely drifted: update it. Do NOT weaken this assertion — "+
					"a ledger that cannot detect its own staleness is worse "+
					"than no ledger.",
				entry.Marker, entry.Path, entry.Why)

			assert.NotEmpty(t, entry.Why,
				"every entry must record WHY it is unmigrated; an unexplained "+
					"exemption is indistinguishable from an oversight")
		})
	}
}

// TestRepoFileContains_DetectsBothPolarities self-validates the ledger's
// detector. A detector hard-wired to `return true` would make the ledger
// pass forever regardless of what the tree actually contains; one hard-wired
// to `return false` would make it fail forever. Golden-good and golden-bad
// both, on a fixture this test writes itself.
func TestRepoFileContains_DetectsBothPolarities(t *testing.T) {
	// repoFileContains resolves its argument against the repo root, so the
	// fixture path has to be expressed relative to that root. Both sides of
	// filepath.Rel must be absolute for it to work.
	dir := t.TempDir()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	rel, err := filepath.Rel(root, dir)
	require.NoError(t, err, "fixture dir must be addressable from the repo root")

	present := filepath.Join(dir, "present.txt")
	require.NoError(t, os.WriteFile(present, []byte("alpha MARKER omega\n"), 0o600))
	absent := filepath.Join(dir, "absent.txt")
	require.NoError(t, os.WriteFile(absent, []byte("alpha omega\n"), 0o600))

	t.Run("golden-good: marker present is detected", func(t *testing.T) {
		found, err := repoFileContains(filepath.Join(rel, "present.txt"), "MARKER")
		require.NoError(t, err)
		assert.True(t, found)
	})

	t.Run("golden-bad: marker absent is detected", func(t *testing.T) {
		found, err := repoFileContains(filepath.Join(rel, "absent.txt"), "MARKER")
		require.NoError(t, err)
		assert.False(t, found,
			"a detector that cannot report absence would make the ledger "+
				"green forever")
	})

	t.Run("missing file surfaces as an error, not a false negative", func(t *testing.T) {
		_, err := repoFileContains(filepath.Join(rel, "nope.txt"), "MARKER")
		assert.Error(t, err,
			"a deleted caller must fail loudly; swallowing the error would "+
				"silently retire the ledger entry")
	})
}

// resetInfraCacheForTest clears the once-per-run availability cache so a
// test that rewrites REDIS_HOST/REDIS_PORT observes its own endpoint rather
// than a verdict cached from an earlier probe. Mirrors the inline reset
// already used by TestDefaultInfraConfig_EnvOverride.
func resetInfraCacheForTest() {
	infraMu.Lock()
	infraResults = make(map[string]bool)
	infraMu.Unlock()
}
