package challenges

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	pb "dev.helix.agent/pkg/api"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// STANDING REGRESSION GUARDS for challenges/scripts/protocol_grpc_challenge.sh
// (§11.4.135 — every fixed defect gets a permanent, falsifiable guard).
//
// WHAT WAS UNGUARDED, AND WHY IT MATTERED.
//
// The gRPC challenge used to decide "gRPC works" from `nc -z` — a bare TCP
// connect — at four independent call sites, every one of which recorded
// SUCCESS when nothing answered. That is two distinct defects stacked:
//
//	fail-open   absence of a server was reported as a PASS
//	no identity a TCP accept was treated as proof of OUR gRPC server
//
// so the challenge could and did certify "Protocol gRPC PASSED" against a
// port nothing had ever bound, and would equally certify it against any
// unrelated process that happened to hold the port (this repo has been bitten
// twice: a Weaviate container on the old :50051, and a teardown guard probing
// :8100 that reached a different process entirely).
//
// The fix replaced `nc -z` with an identity oracle that puts a REAL gRPC
// request on the wire and lets the peer's own answer decide. But until this
// file existed, NOTHING committed pinned that. The only guards touching the
// script — TestK8sManifests_GRPCPortMatchesRegistry and
// TestChallengeScript_GRPCPortMatchesRegistry in internal/ports — pin the PORT
// LITERAL only. Re-introducing `nc -z`-as-PASS would have left every committed
// test green: the port would still be 8112, and the oracle would be gone.
//
// These guards close that gap by RUNNING the script against peers whose
// behaviour is known by construction, and asserting the verdict it reaches.
// They are deliberately runtime-grade, not grep-grade: a grep for "nc -z"
// would be satisfied by any rewrite that reintroduced fail-open behaviour
// under a different spelling (§11.4.226 — the evidence class at closure is
// what decides whether a fix holds).

const (
	// grpcChallengeScriptRel is the artifact under guard.
	grpcChallengeScriptRel = "challenges/scripts/protocol_grpc_challenge.sh"

	// grpcServerSourceRel owns the literals the script's error vector targets.
	grpcServerSourceRel = "cmd/grpc-server/main.go"
)

// ansiEscape strips terminal colouring from the script's log lines.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// outputDirLine captures the run directory the framework announces.
var outputDirLine = regexp.MustCompile(`Output directory: (\S+)`)

// challengeRepoRoot returns the repository root relative to this package.
func challengeRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return root
}

// challengeRun is one execution of the gRPC challenge.
type challengeRun struct {
	exitCode   int
	stdout     string
	outputDir  string
	results    string
	assertions string
}

// test4Verdict returns the recorded status of the error-handling assertion.
func (r challengeRun) test4Verdict(t *testing.T) string {
	t.Helper()
	for _, line := range strings.Split(r.assertions, "\n") {
		if strings.HasPrefix(line, "grpc_errors|") {
			parts := strings.Split(line, "|")
			require.GreaterOrEqual(t, len(parts), 3, "malformed assertion line: %q", line)
			return parts[2]
		}
	}
	t.Fatalf("no grpc_errors assertion recorded; assertions were:\n%s", r.assertions)
	return ""
}

// healthStub serves the HTTP /health the challenge's main() gates on, so the
// run never calls start_helixagent and never contacts a real service.
func healthStub(t *testing.T) int {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	_, portStr, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	return port
}

// pathWithoutGrpcurl returns PATH with every directory that provides grpcurl
// removed, so the RPC-semantics tier deterministically takes its SKIP branch
// (§11.4.50 — the guard's verdict must not depend on what the host happens to
// have installed). Reports false when removing those directories would also
// remove a tool the script genuinely needs, so the caller can SKIP honestly
// (§11.4.3) rather than assert against a script that cannot run.
func pathWithoutGrpcurl(t *testing.T) (string, bool) {
	t.Helper()

	var kept []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, "grpcurl")); err == nil && !info.IsDir() {
			continue
		}
		kept = append(kept, dir)
	}
	sanitized := strings.Join(kept, string(filepath.ListSeparator))

	// The script cannot run at all without these.
	for _, tool := range []string{"bash", "curl", "nc", "grep", "sed", "date"} {
		if !lookPathIn(sanitized, tool) {
			return "", false
		}
	}
	return sanitized, true
}

// lookPathIn reports whether tool is an executable file on the given PATH.
func lookPathIn(path, tool string) bool {
	for _, dir := range filepath.SplitList(path) {
		info, err := os.Stat(filepath.Join(dir, tool))
		if err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return true
		}
	}
	return false
}

// runGRPCChallenge executes the challenge script against grpcPort.
//
// The probe budget is compressed (2 attempts, 1 s apart, 2 s timeout) so the
// not-grpc path — which retries deliberately to survive the server's
// Listen→Serve warmup — completes in seconds rather than the production ~39 s.
// The retry LOOP is still exercised; only its budget shrinks.
func runGRPCChallenge(t *testing.T, grpcPort int, sanitizedPath string) challengeRun {
	t.Helper()

	root := challengeRepoRoot(t)
	script := filepath.Join(root, grpcChallengeScriptRel)
	require.FileExists(t, script)

	waitForFreshRunDir(t, root)

	cmd := exec.Command("bash", script)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PATH="+sanitizedPath,
		fmt.Sprintf("HELIXAGENT_PORT=%d", healthStub(t)),
		fmt.Sprintf("HELIXAGENT_PORT_GRPC=%d", grpcPort),
		"GRPC_PROBE_ATTEMPTS=2",
		"GRPC_PROBE_RETRY_DELAY=1",
		"GRPC_PROBE_TIMEOUT=2",
	)

	out, err := cmd.CombinedOutput()
	run := challengeRun{stdout: ansiEscape.ReplaceAllString(string(out), "")}
	if err != nil {
		var exitErr *exec.ExitError
		if ok := asExitError(err, &exitErr); ok {
			run.exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("running %s: %v\n%s", grpcChallengeScriptRel, err, run.stdout)
		}
	}

	// Resolve the run directory from the script's OWN announcement.
	//
	// NEVER from `challenges/results/<id>/latest`, and never from `ls -t`:
	// finalize_challenge creates that pointer with `ln -sf` and no `-n`, so
	// once `latest` exists as a symlink to a directory every later run
	// dereferences it and drops a nested link INSIDE the first run's
	// directory instead of repointing it. `latest` is therefore pinned to the
	// first run forever, and those nested links keep bumping that directory's
	// mtime so it also stays newest by `ls -t`. Both shortcuts silently read a
	// stale run — this guard would then assert against the wrong artifact.
	m := outputDirLine.FindStringSubmatch(run.stdout)
	require.Lenf(t, m, 2, "could not find the run directory in script output:\n%s", run.stdout)
	run.outputDir = m[1]

	run.results = readIfPresent(filepath.Join(run.outputDir, "results", "protocol-grpc_results.json"))
	run.assertions = readIfPresent(filepath.Join(run.outputDir, "logs", "assertions.log"))
	return run
}

// waitForFreshRunDir blocks until the current wall-clock second has no
// existing run directory, so this run gets one of its own.
//
// WHY THIS IS NECESSARY. init_challenge derives OUTPUT_DIR from
// `date +%Y%m%d_%H%M%S` — one-second granularity, no uniquifier. Two runs
// starting inside the same second therefore SHARE a directory and APPEND to
// the same assertions.log, and since main() computes its verdict by counting
// |FAILED| lines in that log, the second run inherits the first run's
// failures. Observed directly: this guard's positive arm passed alone and
// failed in-suite, because the preceding foreign-peer run (deliberately
// FAILED, 0.34 s earlier) had written into the very log the next run counted.
//
// That is a real framework defect and it is reported as such, not papered
// over: any two challenges finishing within one second cross-contaminate each
// other's verdict. This wait only guarantees THIS guard is deterministic
// (§11.4.50); it does not fix the framework.
func waitForFreshRunDir(t *testing.T, root string) {
	t.Helper()
	base := filepath.Join(root, "challenges", "results", "protocol-grpc")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(base, time.Now().Format("20060102_150405"))); os.IsNotExist(err) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("no free run-directory second under %s within 5s", base)
}

func asExitError(err error, target **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok {
		*target = e
		return true
	}
	return false
}

func readIfPresent(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

// silentTCPPeer accepts TCP connections and speaks nothing — exactly what a
// bare `nc -z` probe sees, and exactly what the pre-fix script called a PASS.
func silentTCPPeer(t *testing.T) int {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = lis.Close() })

	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			// Hold it open without writing a byte. Closing immediately would
			// let curl report a reset, which is a different signal from the
			// "accepted, then silent" case this peer exists to represent.
			t.Cleanup(func() { _ = conn.Close() })
		}
	}()
	return lis.Addr().(*net.TCPAddr).Port
}

// closedPort returns a TCP port on the loopback that NOTHING is listening on.
//
// The number is obtained by binding an ephemeral listener and closing it, so
// it is a real port the kernel handed out rather than an invented constant.
// Freedom is then VERIFIED by dialling, never assumed (§11.4.6) — and on BOTH
// loopback stacks, because the oracle probes `nc -z localhost` and `localhost`
// may resolve to ::1 ahead of 127.0.0.1. A port some other process claims in
// the gap is retried, never silently accepted: a guard that quietly asserted
// against an occupied port would be testing a different peer than it claims.
func closedPort(t *testing.T) int {
	t.Helper()

	for attempt := 0; attempt < 10; attempt++ {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		port := lis.Addr().(*net.TCPAddr).Port
		require.NoError(t, lis.Close())

		if portIsClosed(t, fmt.Sprintf("127.0.0.1:%d", port)) &&
			portIsClosed(t, fmt.Sprintf("[::1]:%d", port)) {
			return port
		}
	}
	t.Fatal("could not obtain a port that nothing listens on after 10 attempts")
	return 0
}

// portIsClosed reports whether a TCP dial to addr finds no listener.
func portIsClosed(t *testing.T, addr string) bool {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return true
	}
	_ = conn.Close()
	return false
}

type identityFacade struct {
	pb.UnimplementedLLMFacadeServer
}

type identityProvider struct {
	pb.UnimplementedLLMProviderServer
}

// grpcPeer starts an in-process gRPC server built from the repo's OWN
// generated stubs and returns its port. When facade is true it serves
// llm.LLMFacade (the service the challenge identifies by); otherwise it serves
// only llm.LLMProvider, which speaks gRPC fluently but is the wrong server.
func grpcPeer(t *testing.T, facade bool) int {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	if facade {
		pb.RegisterLLMFacadeServer(srv, &identityFacade{})
	} else {
		pb.RegisterLLMProviderServer(srv, &identityProvider{})
	}

	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	// Let the listener reach Serve before the script probes it.
	time.Sleep(200 * time.Millisecond)
	return lis.Addr().(*net.TCPAddr).Port
}

// TestProtocolGRPCChallenge_ClosedPortIsSkippedNotPassed pins the ORIGINAL
// defect: a port NOTHING listens on must not produce a passing run.
//
// This is the arm the rest of this file was missing, and PASS-on-absence is
// precisely the shape the historical false-green took. The script's own header
// records it: the challenge dialled :7062 — a port no HelixAgent process has
// ever bound — "found nothing, and ... certified the gRPC protocol healthy
// without once contacting the gRPC server". Every other arm here puts
// SOMETHING on the port, so all three stay green against a script that treats
// absence as success: mutating the oracle's `absent` branch to
// `record_assertion ... "true"` leaves them passing while the mutated script
// exits 0 with `"status": "PASSED"` against a closed port.
//
// Absence is a SKIP, not a FAIL: main() neither starts nor builds
// cmd/grpc-server, so a missing listener is an un-staged precondition rather
// than a proven product defect (§11.4.3). The arm therefore pins BOTH
// directions — nothing recorded PASSED (the fail-open bluff this file exists
// for) and nothing recorded FAILED (a false-positive refusal is its own bluff,
// §11.4.201).
//
// Unlike the other three arms this one needs no PATH surgery and can never
// SKIP itself: the absent verdict is reached before any `command -v grpcurl`
// check, so the outcome cannot depend on what the host happens to have
// installed (§11.4.50).
func TestProtocolGRPCChallenge_ClosedPortIsSkippedNotPassed(t *testing.T) {
	run := runGRPCChallenge(t, closedPort(t), os.Getenv("PATH"))

	require.NotEqualf(t, 0, run.exitCode,
		"a run that verified nothing must not exit 0; output:\n%s", run.stdout)
	require.Containsf(t, run.results, `"status": "SKIPPED"`,
		"a closed port must be reported SKIPPED — never PASSED (fail-open on absence) and "+
			"never FAILED (an un-staged precondition is not a product defect); results:\n%s\nassertions:\n%s",
		run.results, run.assertions)
	require.Containsf(t, run.results, `"assertions_passed": 0`,
		"nothing may be recorded as PASSED against a port nothing listens on; assertions:\n%s",
		run.assertions)
	require.Containsf(t, run.results, `"assertions_failed": 0`,
		"an absent listener must not be recorded as a failure either; assertions:\n%s",
		run.assertions)
	require.Equalf(t, "SKIPPED", run.test4Verdict(t),
		"the error-handling probe must SKIP against a closed port; assertions:\n%s",
		run.assertions)
	require.Containsf(t, run.assertions, "not listening on",
		"the skip must name the absent listener, not some incidental reason; assertions:\n%s",
		run.assertions)

	// The oracle decides `absent` from nc alone and returns BEFORE issuing any
	// gRPC probe, so the probe-evidence file is never created. Asserting its
	// absence pins that short-circuit structurally, with no timing assertion:
	// were the absent path to fall into the retry loop it would burn the whole
	// attempt budget against a port that cannot answer, and blur the
	// absent/not-grpc distinction the entire oracle rests on.
	require.NoFileExistsf(t, filepath.Join(run.outputDir, "logs", "grpc_identity_probe.txt"),
		"an absent listener must short-circuit before the gRPC probe runs; output:\n%s", run.stdout)
}

// TestProtocolGRPCChallenge_SilentTCPPeerIsNotAPass pins the FAIL-OPEN
// POLARITY: a peer that accepts TCP and speaks nothing must NOT yield a pass.
//
// This is the guard that re-introducing `nc -z`-as-PASS breaks. Under the
// pre-fix script this exact peer produced "Port <n> listening" → PASSED →
// exit 0. Under the fixed script the identity oracle gets no gRPC response,
// verdicts not-grpc, and every assertion FAILs.
func TestProtocolGRPCChallenge_SilentTCPPeerIsNotAPass(t *testing.T) {
	sanitized, ok := pathWithoutGrpcurl(t)
	if !ok {
		t.Skip("cannot remove grpcurl from PATH without also removing bash/curl/nc/grep/sed/date " +
			"(SKIP-OK: #grpcurl-shares-dir-with-core-tools)")
	}

	run := runGRPCChallenge(t, silentTCPPeer(t), sanitized)

	require.NotEqualf(t, 0, run.exitCode,
		"a silent TCP peer must not produce a successful challenge run; output:\n%s", run.stdout)
	require.Containsf(t, run.results, `"status": "FAILED"`,
		"a silent TCP peer must be reported FAILED, not PASSED/SKIPPED; results were:\n%s\nassertions:\n%s",
		run.results, run.assertions)
	require.Containsf(t, run.results, `"assertions_passed": 0`,
		"nothing may be recorded as PASSED against a peer that never spoke gRPC; assertions:\n%s",
		run.assertions)
	require.Equalf(t, "FAILED", run.test4Verdict(t),
		"the error-handling probe must FAIL against a peer that never spoke gRPC; assertions:\n%s",
		run.assertions)
}

// TestProtocolGRPCChallenge_ForeignGRPCPeerIsNotAPass pins the second half of
// the identity oracle: speaking gRPC is not enough, it must be OUR service.
//
// The peer here is a real gRPC server built from this repo's own stubs, so it
// completes the HTTP/2 handshake perfectly — it simply does not serve
// llm.LLMFacade. That is the Weaviate-on-:50051 shape, and it must FAIL.
func TestProtocolGRPCChallenge_ForeignGRPCPeerIsNotAPass(t *testing.T) {
	sanitized, ok := pathWithoutGrpcurl(t)
	if !ok {
		t.Skip("cannot remove grpcurl from PATH without also removing bash/curl/nc/grep/sed/date " +
			"(SKIP-OK: #grpcurl-shares-dir-with-core-tools)")
	}

	run := runGRPCChallenge(t, grpcPeer(t, false), sanitized)

	require.NotEqualf(t, 0, run.exitCode,
		"a gRPC peer that does not serve %s must not pass; output:\n%s", "llm.LLMFacade", run.stdout)
	require.Containsf(t, run.results, `"status": "FAILED"`,
		"a foreign gRPC peer must be reported FAILED; results:\n%s\nassertions:\n%s",
		run.results, run.assertions)
	require.Containsf(t, run.assertions, "does not serve llm.LLMFacade",
		"the failure must name the identity mismatch, not some incidental error; assertions:\n%s",
		run.assertions)
	require.Equalf(t, "FAILED", run.test4Verdict(t),
		"the error-handling probe must FAIL against a foreign gRPC peer; assertions:\n%s",
		run.assertions)
}

// TestProtocolGRPCChallenge_RealFacadePeerPasses pins the POSITIVE arm: a real
// llm.LLMFacade server IS recognised.
//
// Without this, the two negative guards above could be satisfied by a script
// that fails against everything — a gate that never passes is as useless as
// one that never fails (§11.4.201: a false-positive refusal is its own bluff).
// grpcurl is removed from PATH so the RPC-semantics tier SKIPs and the verdict
// turns purely on the identity oracle.
func TestProtocolGRPCChallenge_RealFacadePeerPasses(t *testing.T) {
	sanitized, ok := pathWithoutGrpcurl(t)
	if !ok {
		t.Skip("cannot remove grpcurl from PATH without also removing bash/curl/nc/grep/sed/date " +
			"(SKIP-OK: #grpcurl-shares-dir-with-core-tools)")
	}

	run := runGRPCChallenge(t, grpcPeer(t, true), sanitized)

	require.Equalf(t, 0, run.exitCode,
		"a real llm.LLMFacade peer must produce a passing run; output:\n%s", run.stdout)
	require.Containsf(t, run.results, `"status": "PASSED"`,
		"a real llm.LLMFacade peer must be reported PASSED; results:\n%s\nassertions:\n%s",
		run.results, run.assertions)
	require.Containsf(t, run.assertions, "grpc_server|available|PASSED",
		"the identity assertion must be the one that passed; assertions:\n%s", run.assertions)
	require.Containsf(t, run.assertions, "answered a real gRPC request",
		"the PASS must cite the positive gRPC evidence, never a TCP accept; assertions:\n%s",
		run.assertions)

	// An absent grpcurl must be an honest SKIP, never a PASS: that tier is
	// the second place this script could fail open (§11.4.69 no-fail-open).
	require.Equalf(t, "SKIPPED", run.test4Verdict(t),
		"a missing grpcurl must SKIP the error-handling probe, not pass it; assertions:\n%s",
		run.assertions)
}

// --- Error-vector drift guard (test 4) -------------------------------------

var (
	scriptErrMethod  = regexp.MustCompile(`GRPC_INVALID_METHOD="([^"]+)"`)
	scriptErrMessage = regexp.MustCompile(`GRPC_INVALID_EXPECT_MSG="([^"]+)"`)
	scriptErrCode    = regexp.MustCompile(`GRPC_INVALID_EXPECT_CODE="([^"]+)"`)
	scriptErrPayload = regexp.MustCompile(`GRPC_INVALID_PAYLOAD='([^']+)'`)
)

func redMode() bool { return strings.TrimSpace(os.Getenv("RED_MODE")) == "1" }

// challengeScriptBody returns the script under guard. GRPC_CHALLENGE_SCRIPT
// overrides the path so RED_MODE can be run against the pre-fix revision
// (§11.4.115 — the RED baseline must be taken on the actually-broken artifact,
// never on a synthetic stand-in).
//
// Note for whoever re-runs the RED baseline: the artifact that carried the
// unknown-field vector was the UNCOMMITTED working tree this fix started from,
// not `git show HEAD:` — HEAD (1f14ca48) carries an older, differently-broken
// vector that addressed a schema-valid payload to a service the server does
// not implement. Pointing RED_MODE at HEAD therefore fails for an unrelated
// reason; point it at the pre-fix working copy.
func challengeScriptBody(t *testing.T) string {
	t.Helper()
	path := os.Getenv("GRPC_CHALLENGE_SCRIPT")
	if path == "" {
		path = filepath.Join(challengeRepoRoot(t), grpcChallengeScriptRel)
	}
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

// TestChallengeScript_ErrorVectorMatchesServer asserts test 4 targets an input
// the SERVER rejects, and that the literals it expects still exist in Go.
//
// WHY THE OLD VECTOR WAS NOT A TEST OF HELIXAGENT AT ALL. Test 4 used to send
// a field the schema does not define and accept "unknown field" as proof of
// server-side validation. With a -proto-sourced schema grpcurl performs the
// JSON→protobuf conversion LOCALLY, so that request fails client-side and is
// never transmitted: a server instrumented with a logging interceptor recorded
// nothing for it. Matching grpcurl's wording would therefore have certified
// grpcurl's parser rather than the product. proto3 also tolerates unknown
// fields on the wire by design, so the premise was never a valid conformance
// claim either.
//
// A bash script cannot call into Go, so the expected code and message are
// necessarily duplicated. This test is what makes that duplication safe.
func TestChallengeScript_ErrorVectorMatchesServer(t *testing.T) {
	body := challengeScriptBody(t)

	// Assert on CODE, not prose. The script documents the removed vector at
	// length so the next reader understands why it must not come back, and a
	// whole-file match would be tripped by that very explanation — the guard
	// would forbid describing the defect it exists to prevent.
	code := stripShellComments(body)

	if redMode() {
		require.Containsf(t, code, "this_field_does_not_exist_in_the_schema",
			"RED_MODE=1: the pre-fix script sends an unknown field, a vector that "+
				"fails inside grpcurl and never reaches the server")
		require.Containsf(t, code, `"unknown field"`,
			"RED_MODE=1: the pre-fix script accepts grpcurl's own parser error as a PASS")
		return
	}

	require.NotContainsf(t, code, "this_field_does_not_exist_in_the_schema",
		"test 4 must not send an unknown field: grpcurl rejects it locally, so the "+
			"request never reaches the server and the assertion tests grpcurl, not HelixAgent")
	require.NotContainsf(t, code, `"unknown field"`,
		"test 4 must not accept grpcurl's client-side parser wording as evidence of "+
			"server-side validation")

	method := firstSubmatch(t, scriptErrMethod, body, "GRPC_INVALID_METHOD")
	message := firstSubmatch(t, scriptErrMessage, body, "GRPC_INVALID_EXPECT_MSG")
	expectCode := firstSubmatch(t, scriptErrCode, body, "GRPC_INVALID_EXPECT_CODE")
	payload := firstSubmatch(t, scriptErrPayload, body, "GRPC_INVALID_PAYLOAD")

	src := readIfPresent(filepath.Join(challengeRepoRoot(t), grpcServerSourceRel))
	require.NotEmptyf(t, src, "cannot read %s; if it moved, update this guard rather than "+
		"deleting it — the drift it catches is still real", grpcServerSourceRel)

	require.Containsf(t, src, "func (s *LLMFacadeServer) "+method+"(",
		"%s targets %s, but %s defines no such LLMFacade handler",
		grpcChallengeScriptRel, method, grpcServerSourceRel)

	require.Containsf(t, src, fmt.Sprintf("status.Error(codes.%s, %q)", expectCode, message),
		"%s expects %s %q from %s, but no such rejection exists in %s — the vector has "+
			"drifted away from the handler it targets",
		grpcChallengeScriptRel, expectCode, message, method, grpcServerSourceRel)

	// The payload must actually omit the field whose absence triggers the
	// rejection, or the request would succeed and the assertion would be
	// asserting nothing.
	require.NotContainsf(t, payload, `"name"`,
		"the invalid payload must OMIT the required field that triggers %q; got %s",
		message, payload)
}

// TestChallengeScript_ErrorVectorProbeIsSideEffectFree pins the ORDER that
// makes test 4's probe safe to fire at a live server.
//
// The script asserts, as a load-bearing fact, that AddProvider "answers
// InvalidArgument 'provider name is required' BEFORE mutating any state — so
// this probe is side-effect-free and leaves no provider registered". Nothing
// pinned that. TestChallengeScript_ErrorVectorMatchesServer pins the LITERALS
// — the handler exists, the rejection exists, the payload omits `name` — and
// every one of them survives moving the validation BELOW s.providers.Put.
// After that refactor the whole guard set stays green while the challenge
// silently registers a phantom empty-named provider into the server under test
// on every run: a probe that mutates the system it is measuring, and a
// challenge whose own comment has quietly become false (§11.4.6).
//
// The assertion is POSITIONAL WITHIN the handler body, not a textual layout
// match, so it is indifferent to comments, whitespace and added fields. It can
// fire on exactly two events: the validation genuinely moving after the
// mutation (the defect), or the validation being extracted out of the body (a
// refactor). The second is a loud, deliberate prompt to re-establish the
// invariant wherever it now lives — per this file's standing posture, update
// the guard, never delete it (§11.4.120: reconcile a gate, do not fake-pass
// it).
//
// No RED_MODE branch: this pins an invariant that has never been broken, so
// there is no broken artifact to take a RED baseline on. A synthetic RED would
// prove only that the assertion can be made to fail on demand (§11.4.115).
func TestChallengeScript_ErrorVectorProbeIsSideEffectFree(t *testing.T) {
	if redMode() {
		t.Skip("RED_MODE=1 replays the pre-fix error vector; the validate-before-mutate " +
			"order it depends on was never broken, so there is no RED baseline to take " +
			"(SKIP-OK: #no-red-baseline-for-unbroken-invariant)")
	}

	script := challengeScriptBody(t)
	method := firstSubmatch(t, scriptErrMethod, script, "GRPC_INVALID_METHOD")
	message := firstSubmatch(t, scriptErrMessage, script, "GRPC_INVALID_EXPECT_MSG")

	src := readIfPresent(filepath.Join(challengeRepoRoot(t), grpcServerSourceRel))
	require.NotEmptyf(t, src, "cannot read %s; if it moved, update this guard rather than "+
		"deleting it — the drift it catches is still real", grpcServerSourceRel)

	body := goFuncBody(t, src, "func (s *LLMFacadeServer) "+method+"(")

	reject := strings.Index(body, strconv.Quote(message))
	require.GreaterOrEqualf(t, reject, 0,
		"%s rejects %q somewhere in %s, but not inside %s itself — if that validation was "+
			"extracted into a helper, re-point this guard at the helper and assert the helper "+
			"is called before s.providers.Put; do not drop the invariant",
		grpcServerSourceRel, message, grpcServerSourceRel, method)

	mutate := strings.Index(body, "s.providers.Put(")
	require.GreaterOrEqualf(t, mutate, 0,
		"%s no longer registers the provider via s.providers.Put; re-point this guard at the "+
			"new mutation site rather than deleting it", method)

	require.Lessf(t, reject, mutate,
		"%s must reject %q BEFORE s.providers.Put, or test 4's probe stops being "+
			"side-effect-free and starts registering a phantom provider on every challenge run",
		method, message)
}

// TestChallengeScript_ErrorVectorDemandsBothCodeAndMessage pins the CONJUNCTION
// that makes test 4's verdict a server-reach proof.
//
// The script states the invariant as a load-bearing fact — "BOTH must hold. The
// code alone is not enough (a foreign peer answers InvalidArgument for its own
// unrelated reasons); the message alone is the server-reach proof" — and encodes
// it in the single `&&` joining the two `grep -qF` probes. Nothing pinned that
// `&&`. Flipping it to `||` leaves ALL SIX guards above green, with grpcurl
// absent and present: the four runtime arms never reach test 4's comparison
// (three verdict before it; the fourth SKIPs on absent grpcurl), and the two
// static guards assert the LITERALS, not how they are combined. The mutant is
// therefore silent, and under it the challenge accepts a bare InvalidArgument
// from ANY peer as proof our handler answered — the exact identity bluff the
// test-4 redesign exists to close. §11.4.226: prose does not bind, seams do.
//
// The assertion is POSITIONAL — it reads what JOINS the two probes rather than
// matching their layout — so it is indifferent to line breaks, indentation and
// probe order. Commentary cannot satisfy it: the body is comment-stripped first,
// so the paragraph above the condition (which itself says "BOTH must hold")
// can never stand in for the code.
//
// No committed runtime arm ever evaluates this condition — test 4 opens with
// `if ! command -v grpcurl; then record_skip; return`, so the three
// grpcurl-absent arms skip it and the closed-port arm returns before reaching
// it. This static guard is the only thing holding the condition's shape.
//
// THE INVARIANT: EXACTLY ONE GATING LIST ELEMENT, AND IT IS THE CONJUNCTION.
//
// Between the `if`/`elif` that OPENS test 4's condition and the `then` that
// CLOSES it there must be EXACTLY ONE list element, and that element must be
// the two probes joined by `&&`.
//
// Stating it that way is the round-7 correction, and the distance from the
// earlier "no disjunction anywhere in the condition" is not academic. Absence
// of disjunction is a CONSEQUENCE of the invariant, never the invariant itself,
// and a mutant can satisfy the consequence while destroying the property. Bash
// gates an `if` on the LAST element of its condition list, so
//
//	if <codeProbe> && <msgProbe>; [ 1 = 1 ]; then
//
// evaluates the conjunction, DISCARDS it, and gates on `[ 1 = 1 ]`. Measured
// live, that records `grpc_errors … "true"` unconditionally — for an empty
// response, for a foreign peer's error, for a response matching neither
// expectation. Both probes are still textually present, still pipeline-headed,
// still conjoined by `&&`, and no disjunction has been added anywhere. Six
// successive guard designs reasoned about where a `||` could hide; every one of
// them passed this.
//
// A list element is closed by `;`, by `&`, or by a NEWLINE that no operator is
// waiting on. All three spell the same collapse, and all three are forbidden
// except as the single separator that ends the condition:
//
//	Z1  `; [ -n "$resp" ]; then` — gates on any non-empty grpcurl output, which
//	    is verbatim the foreign-peer identity bluff the conjunction closes.
//	Z2  `; true; then` — the same collapse onto a bare tautology.
//	Z3  `; [ 1 = 1 ]; then` — likewise, spelled as a test so it looks load-bearing.
//	Z4  the NEWLINE spelling of Z3: the conjunction, a newline, `[ 1 = 1 ]`, a
//	    newline, `then`. `if false`⏎`true`⏎`then` takes the true branch, so this
//	    is Z3 exactly, and it is the spelling that also walked through every
//	    window whose right edge stopped at the first non-continuation newline.
//	Z5  the separator BETWEEN the probes (`CODE && [ 1 = 1 ]; MSG; then`), which
//	    discards the code probe's element and gates on the message alone.
//	Z6  a separator BEFORE the probes (`[ 1 = 1 ]; CODE && MSG; then`). This one
//	    still gates on the conjunction and is harmless on its face; it is
//	    rejected anyway, because "exactly one element" is the cheap invariant to
//	    hold and a second element is a standing invitation to the Z1–Z5 edits.
//	Z7  `&` in place of `;`, which backgrounds the conjunction and gates on the
//	    launch status — always 0.
//	Z8  a brace group whose last member is not the conjunction, `{ A && B; false; }`.
//	    This one collapses toward FALSE — measured, it verdicts false even on a
//	    response matching both expectations — so it breaks the challenge rather
//	    than bluffing it. Rejected all the same.
//
// A second element is not the only way to unseat the conjunction. `|` binds
// TIGHTER than `&&`, so extending the last probe into a pipeline —
//
//	if <codeProbe> && <msgProbe> | cat; then
//
// — parses as `codeProbe && (msgProbe | cat)`, and a pipeline exits with its
// LAST stage's status. `cat` always succeeds, so the message probe stops
// deciding anything and the verdict collapses to code-only: measured on a
// response carrying InvalidArgument from a foreign peer but NOT our message,
// pristine verdicts false and this verdicts true. It adds no separator and no
// disjunction, and it keeps both probes pipeline-headed and `&&`-joined. The
// walk rejects it because `|` is not one of the constructs it can name — the
// fail-closed default earning its keep on an axis nobody enumerated.
//
// The disjunction mutants remain what they were, and remain caught:
//
//	M7  the joiner `&&` flipped to `||`. Caught by the joiner assertion, which
//	    is the original and still-correct reading of the middle.
//	F1  `|| true` appended AFTER the last probe. Bash AND-OR lists are
//	    left-associative, so `A && B || true` is `(A && B) || true` and the
//	    condition ALWAYS succeeds — strictly worse than flipping `&&` to `||`,
//	    and the canonical real-world silence-a-failing-check edit. The `||`
//	    lives to the RIGHT of the last probe, so the joiner reads clean.
//	F2  the second probe's stdin flipped from `"$resp"` to the expectation
//	    itself, so the probe greps the expectation against itself and is always
//	    true — collapsing the verdict to code-only, i.e. verbatim the foreign-
//	    peer bluff this conjunction exists to close. The `&&` still reads clean.
//	L1  `if true || …` prefixed BEFORE the first probe, and
//	L2  `if [ -n "$resp" ] || …`, its plausible-looking twin. By the same
//	    left-associativity, `(true || CODE) && MSG` discards the code probe's
//	    result and collapses the verdict to message-only: measured on a `resp`
//	    carrying the server's message but NOT the status code, pristine verdicts
//	    false and both mutants verdict true. The `||` lives to the LEFT of the
//	    first probe, so the joiner AND any first-probe-anchored window read
//	    clean. Less severe than F2 — the surviving half is the server-reach-
//	    bearing one, so the foreign-peer identity bluff stays closed — but it
//	    still defeats "BOTH must hold".
//	DL  a `[ -n "$resp" ] ||` parked on the line ABOVE the probes, and
//	DR  a `true || false` parked on the line BELOW them, each reachable only
//	    because bash continues a statement after a trailing `&&`, `||` or `|`.
//
// HOW THE WINDOW IS CUT.
//
// Each probe is located by its literal text WITH its pipeline head, so a probe
// fed anything but `$resp` fails loudly as probe-not-found (the correct
// §11.4.120 re-point signal, not a silent pass) — that is what catches F2. Each
// literal must also occur exactly ONCE, so the guard cannot be pointed at a
// well-formed decoy `if` while the verdict is recorded by a collapsed one
// further down.
//
// The left edge is the FIRST `if`/`elif` in the probes' own statement — first,
// not nearest, so an `if` standing inside the condition cannot drag the edge
// rightward past a disjunction. The statement bound honours BOTH of bash's
// continuation spellings (a `\`-escaped newline and a newline after a trailing
// `&&`, `||` or `|`), which is what keeps DL inside the window. A missing
// opener FAILs loudly and asks to be re-pointed.
//
// The right edge is found by WALKING the text after the last probe and
// accounting for every byte of it, rather than by searching for a `then`. The
// walk distinguishes a newline that CONTINUES the element (one an operator or a
// `\` is waiting on) from a newline that CLOSES it, and it accepts `then` only
// in the closed state — which is bash's own rule, since `then` is a reserved
// word only in command position. That one change closes the Z-class and the
// `[ x = then ]` edge-move together: a `then` standing as an ordinary argument
// is never in command position, so it can no longer terminate the walk.
//
// A missing `then` now FAILs. The revision before this one made it deliberately
// benign — "`then` on its own line is valid bash and the condition simply ends
// where the statement does" — and that sentence carried the false premise Z4
// walks through: inside an `if` condition a bare newline does NOT end the
// statement, it separates list elements, so "the statement ends here" silently
// placed every later element OUTSIDE the window. `then` on its own line still
// passes; it is now REACHED by the walk instead of assumed away. (A comment
// carrying a false premise is worse than no comment, which is the second time
// this chain has been bitten by exactly that.)
//
// Honest boundary. This guard pins the SHAPE OF THE CONDITION — that both
// probes are conjoined and that the conjunction is the one thing gating the
// branch. It does NOT pin the mapping from that condition to the recorded
// verdict: negating the whole conjunction while leaving the branches in place,
// or swapping the two `record_assertion` bodies, inverts test 4 without
// touching the shape, and nothing in this file currently catches either (the
// four runtime arms cannot — three verdict before test 4 and the fourth removes
// grpcurl from PATH, so none reaches the comparison). That is a branch-polarity
// invariant and wants its own guard; widening this one to cover it would only
// blur what it certifies.
//
// It also reads the script as TEXT, with no notion of code position versus data
// position. A probe literal sitting inside a heredoc or a quoted string is
// indistinguishable to it from the same literal in the condition. The
// occurs-exactly-once rule closes the reachable spelling of that — a decoy
// alongside the real gate makes the count 2 and FAILs — but a script whose ONLY
// occurrence were in data, with the real verdict gated by differently-spelled
// code, would be read as though the data were the condition. Closing that needs
// a shell parser rather than a walk, or the branch-polarity guard tying the
// condition to the `record_assertion` calls it decides; it is named here rather
// than left for a later round to discover.
//
// And it reads the CONDITION, not the environment the condition runs in. A
// `grep() { return 0; }` defined anywhere earlier in the script makes both
// probes unconditionally true while leaving this condition byte-identical to
// pristine — measured, it verdicts true on an empty response. Nothing textual
// about the condition can catch that; it wants a runtime arm that drives test 4
// against a known-bad response and asserts the recorded verdict is false, which
// is the same missing arm the branch-polarity gap needs. Both are gaps in
// COVERAGE of test 4, not in this guard's certificate, and both stay open.
//
// The strictness deliberately trades a little false-positive risk for the
// silent passes it removes. A De Morgan rewrite (`if ! A || ! B; then`) is
// semantically equivalent yet FAILs here, and so does the harmless Z6; both are
// acceptable loud failures, because every message below names what to do
// instead of merely rejecting. Formatting is unaffected: line continuations,
// blank lines inside the condition, indentation, probe order, further `&&`
// bracket conjuncts, `then` on its own line and a comment carrying `||` are all
// indifferent.
//
// No RED_MODE branch, for the same reason as the guard above: this pins an
// invariant that has never been broken in a shipped artifact, so there is no
// broken artifact to take a RED baseline on (§11.4.115). Falsifiability comes
// from the paired §1.1 mutations instead — every mutant named above FAILs this
// test, and each was applied to the script, `bash -n` checked, run, and
// restored.
//
// The window's edges are only as trustworthy as the lexer that finds them, and
// three mutants moved an edge rather than crossing it: `&& : "#" || true`
// (a quoted `#` that stripShellComments cuts anyway), `&& test -z "$(`⏎`)" ||
// true` (a newline carried inside a command substitution), `&& [ x = then ] ||
// true` (a `then` standing as an ordinary argument). All three left an earlier
// revision GREEN while the live shell recorded a pass on a token-free response.
// They are one defect — a lexer that passes by default on syntax it cannot read
// — and the walk below fixes the default rather than the three instances. See
// scanCondition.
func TestChallengeScript_ErrorVectorDemandsBothCodeAndMessage(t *testing.T) {
	if redMode() {
		t.Skip("RED_MODE=1 replays the pre-fix error vector, whose verdict predates this " +
			"code-AND-message conjunction entirely, so there is no RED baseline to take " +
			"(SKIP-OK: #no-red-baseline-for-unbroken-invariant)")
	}

	// Each probe is pinned WITH its pipeline head, so redirecting a probe's
	// stdin away from the response (F2) cannot pass as the same probe.
	const (
		codeProbe = `printf '%s' "$resp" | grep -qF "$GRPC_INVALID_EXPECT_CODE"`
		msgProbe  = `printf '%s' "$resp" | grep -qF "$GRPC_INVALID_EXPECT_MSG"`
	)
	code := stripShellComments(challengeScriptBody(t))

	reject := strings.Index(code, codeProbe)
	require.GreaterOrEqualf(t, reject, 0,
		"test 4 no longer tests THE RESPONSE with %s. Either the probe moved or its input "+
			"is no longer $resp — a probe fed anything else (the expectation itself, a "+
			"literal) is trivially true and stops proving our handler answered. If you "+
			"restructured this condition, re-point this assertion at its new form — do not "+
			"delete it", codeProbe)

	identify := strings.Index(code, msgProbe)
	require.GreaterOrEqualf(t, identify, 0,
		"test 4 no longer tests THE RESPONSE with %s. Either the probe moved or its input "+
			"is no longer $resp — a probe fed anything else (the expectation itself, a "+
			"literal) is trivially true and collapses the verdict to code-only, which any "+
			"foreign peer answering InvalidArgument satisfies. If you restructured this "+
			"condition, re-point this assertion at its new form — do not delete it", msgProbe)

	// Each probe must be THE probe. This guard reads the FIRST occurrence, so a
	// second copy would let a well-formed decoy absorb the certificate while the
	// verdict is recorded by a collapsed condition somewhere else in the script.
	for _, probe := range []string{codeProbe, msgProbe} {
		require.Equalf(t, 1, strings.Count(code, probe),
			"test 4's probe %s occurs %d times in %s, and this guard reads the first — so a "+
				"second copy lets it certify one `if` while the verdict is gated by another. "+
				"Keep exactly one occurrence, or re-point this guard at the statement that "+
				"actually records the verdict; do not delete it",
			probe, strings.Count(code, probe), grpcChallengeScriptRel)
	}

	start, joinLo, joinHi, lastEnd := reject, reject+len(codeProbe), identify, identify+len(msgProbe)
	if identify < reject {
		start, joinLo, joinHi, lastEnd = identify, identify+len(msgProbe), reject, reject+len(codeProbe)
	}
	joiner := code[joinLo:joinHi]

	// Cut the window over the WHOLE condition — from the keyword that OPENS it
	// through the `then` that CLOSES it — so an edit on either flank is inside
	// the window, not merely between the probes. The left edge is bounded by the
	// probes' own statement so it cannot run away into a neighbouring one; the
	// right edge is walked byte by byte (scanCondition) so it cannot stop early
	// at a newline that merely separates list elements.
	opener, openerEnd := firstShellKeyword(code, statementStart(code, start), start, "if", "elif")
	require.GreaterOrEqualf(t, opener, 0,
		"test 4's probes are no longer opened by an `if`/`elif` in their own statement, so this "+
			"guard cannot bound the condition it must read. If the verdict moved out of an `if` "+
			"— into a helper, a variable, a `[[ ]]` test — re-point this assertion at its new "+
			"form and keep it proving BOTH probes must hold; do not delete it")

	tail := scanCondition(code, lastEnd, len(code), false, true)
	condition := code[opener:tail.end]

	require.Containsf(t, joiner, "&&",
		"test 4 must require the status code AND the server's own message — they are joined "+
			"by %q, not `&&`. Either probe alone is satisfiable without reaching our handler: "+
			"any peer can answer InvalidArgument, and only the message exists solely in our Go "+
			"source. If you restructured this condition, re-point this assertion — do not delete it",
		joiner)
	require.NotContainsf(t, condition, "||",
		"test 4's condition must contain no disjunction anywhere from the `if` that opens it "+
			"through its `then`, but it does: %q. Bash AND-OR lists are left-associative, so a "+
			"`true ||` prefixed before the first probe collapses the verdict to the last probe "+
			"alone, and a `|| true` appended after the last probe makes the whole condition "+
			"always succeed — on either flank the verdict stops requiring BOTH probes. If you "+
			"genuinely need a disjunction here — a De Morgan rewrite reads equivalently — "+
			"re-point this assertion at the new form and keep it proving BOTH probes must "+
			"hold; do not delete it", condition)

	// EXACTLY ONE GATING ELEMENT. Bash gates an `if` on the LAST element of its
	// condition list, so a second element — however it is spelled — throws the
	// conjunction away and decides the verdict on its own. The walk below
	// therefore permits a separator only as the one that ends the condition,
	// with nothing but whitespace between it and `then`.
	require.Truef(t, tail.readable,
		"test 4's condition does not end at its `then` with the probes' conjunction as the "+
			"one thing gating it; this stands after the last probe: %q. Bash gates an `if` on "+
			"the LAST element of its condition list, and `;`, `&`, and a newline no operator "+
			"is waiting on all close an element — so `A && B; [ 1 = 1 ]; then` evaluates the "+
			"conjunction, DISCARDS it, and records a pass unconditionally, with both probes "+
			"still present and no disjunction anywhere. Only whitespace, a `\\`-continuation, "+
			"a further `&&`-joined single-line bracket test, and the one separator before "+
			"`then` may stand here. If you genuinely need another construct, teach "+
			"scanCondition to read it AND re-point these assertions at the new form — do not "+
			"delete them, and do not widen this to a deny-list", tail.rest)
	require.Truef(t, tail.sawThen,
		"test 4's condition never reaches a `then` that closes it, so this guard cannot tell "+
			"where the condition ends — and everything past the end it cannot see is a place a "+
			"second list element can gate the verdict from. `then` on its own line is fine; "+
			"what is not is a condition whose `then` this walk cannot reach. If the verdict "+
			"moved into some other construct, re-point this assertion at it — do not delete it")

	// FAIL CLOSED on the rest. Every byte of the window that is not the opener,
	// a probe, or the closing `then` must be a construct this guard can name.
	// Searching for known-bad forms is what let three separate mutants through;
	// requiring known-good is what closes the class. See scanCondition.
	for _, seg := range []struct {
		where   string
		lo, hi  int
		pending bool
	}{
		// After `if` a command is still awaited, so newlines there continue the
		// element; after the first probe a complete command has just ended, so a
		// newline there would CLOSE it — and closing it discards that probe.
		{"between the `if` and the first probe", openerEnd, start, true},
		{"joining the two probes", joinLo, joinHi, false},
	} {
		s := scanCondition(code, seg.lo, seg.hi, seg.pending, false)
		require.Truef(t, s.readable,
			"test 4's condition contains a shell construct this guard cannot read, %s: %q. "+
				"The window that proves BOTH probes must hold is cut by an `if`/`elif` on one "+
				"side and a `then` on the other, and a construct it does not understand can "+
				"move either edge — a quoted `#` (stripped as if it began a comment), an open "+
				"`$(`, a stray quote — parking a `||` outside the window while the verdict "+
				"still collapses. A `;`, `&` or bare newline here is worse still: it closes "+
				"the gating element, so one of the two probes stops deciding anything. Only "+
				"whitespace, a `\\`-continuation, the `&&` joiner and a complete single-line "+
				"bracket test may stand here. If you genuinely need this construct, teach "+
				"scanCondition to read it AND re-point these assertions at the new form — do "+
				"not delete them, and do not widen this to a deny-list", seg.where, s.rest)
	}
}

// conditionScan is what one walk over a stretch of test 4's condition found.
type conditionScan struct {
	end      int    // one past the last byte the walk accounted for
	rest     string // the first fragment it could not account for, when !readable
	readable bool   // every byte was a construct the walk can name, in a position it may hold
	sawThen  bool   // the walk reached the `then` that closes the condition
}

// scanCondition walks s over [lo, hi), accounting for every byte as part of the
// ONE list element that must gate test 4's `if`.
//
// pending says whether an operator is already awaiting its right-hand operand
// at lo, which is what decides whether a newline CONTINUES the element or
// CLOSES it. closes says this stretch runs to the condition's `then`, which is
// the only position where a separator — and then only whitespace after it — may
// legitimately stand.
//
// This is the guard's FAIL-CLOSED default, and it is the whole point of the
// window machinery. Rounds 2-5 each widened a hand-rolled lexer to cover the
// construct the last bug used, and each time the next construct walked through
// the gap the lexer still had. Three found together made the shape plain:
//
//   - `&& : "#" || true; then` — bash never starts a comment inside quotes, but
//     stripShellComments cuts at the first `#` regardless, so the guard's view of
//     the line ends at `: "` and the `||` is simply not in the window.
//   - `&& test -z "$(`⏎`)" || true; then` — bash carries the statement across the
//     newline inside the command substitution, so a window that ends at the
//     first non-continuation newline stops a line early.
//   - `&& [ x = then ] || true; then` — reserved words are POSITIONAL, so that
//     `then` is an ordinary argument to `[`; a guard that searched for the word
//     matched it anyway and pulled the right edge in before the `||`.
//
// Those are not three bugs. They are one: the lexer PASSED on syntax it did not
// understand — §11.4.201's fail-open shape at the meta level, where the guard
// itself is the thing that fails open. A fourth flank patch would have left the
// generator of flanks intact.
//
// Round 6 then found the same fail-open shape one level up, in what the walk
// was asked to prove. `A && B; [ 1 = 1 ]; then` is readable to the byte — a
// separator and a bracket test, both allowlisted — and adds no disjunction, yet
// it discards the conjunction and gates the verdict on a tautology. The bytes
// were never the invariant; their POSITION is. So the walk now tracks whether
// the gating element is still open, and refuses anything that stands after it
// has been closed. `then` is accepted ONLY in that closed state, which is also
// bash's own rule for when `then` is a reserved word rather than an argument —
// so the round-5 `[ x = then ]` edge-move dies of the same clause, at its root
// rather than by a patch describing it.
//
// Whitespace, a `\`-continuation, the `&&` joiner, a complete single-line
// bracket-test conjunct, and the one separator before `then` are the whole of
// what may stand. Anything else — a stray quote, a `$(`, a backtick, a bare
// word, an `&`, a brace group, a bracket left unclosed because the window was
// cut inside it — is unaccounted for and FAILS loudly.
func scanCondition(s string, lo, hi int, pending, closes bool) conditionScan {
	sepAt := -1 // where the gating element was closed, or -1 while it is open

	// Report from the separator when there is one: the useful fragment is the
	// whole trailing element, not the byte the walk happened to stop on. Bound
	// it to the end of that line so a broken condition cannot quote the rest of
	// the file back at whoever wrote it.
	fail := func(at int) conditionScan {
		from := at
		if sepAt >= 0 {
			from = sepAt
		}
		to := hi
		if nl := strings.IndexByte(s[at:hi], '\n'); nl >= 0 {
			to = at + nl + 1
		}
		return conditionScan{end: to, rest: s[from:to]}
	}

	for i := lo; i < hi; {
		switch c := s[i]; {
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '\\' && i+1 < hi && s[i+1] == '\n':
			i += 2 // an escaped newline joins the lines: still one command
		case c == '\n':
			if pending {
				i++ // the line ended on an operator, so the element continues
				continue
			}
			if sepAt < 0 {
				sepAt = i // a bare newline closes the element, exactly like `;`
			}
			i++
		case c == ';':
			if sepAt < 0 {
				sepAt = i
			}
			i++
		case sepAt >= 0:
			// The gating element is closed. Only the `then` that ends the
			// condition may follow — anything else is a second element, and
			// bash would gate the verdict on IT rather than the conjunction.
			if closes && i+len("then") <= hi && shellKeywordAt(s, "then", i) {
				return conditionScan{end: i + len("then"), readable: true, sawThen: true}
			}
			return fail(i)
		case strings.HasPrefix(s[i:hi], "&&"):
			pending = true
			i += 2
		case c == '[':
			n, ok := bracketConjunct(s[i:hi])
			if !ok {
				return fail(i)
			}
			pending = false
			i += n
		default:
			return fail(i)
		}
	}

	if sepAt >= 0 && !closes {
		return fail(hi) // an element was closed where no `then` can follow
	}
	return conditionScan{end: hi, readable: true}
}

// bracketConjunct consumes a complete `[ … ]` / `[[ … ]]` test at the head of s
// and reports its length. A further `&&`-joined conjunct is benign on its face
// — an AND can only make the verdict STRICTER, never weaker — but only while
// the guard can see the whole of it, so the test must close on the same line
// with its quotes and `${…}` balanced and no command substitution inside.
//
// That is not fastidiousness: an unbalanced quote, an unbalanced brace and an
// open `$(` are precisely the constructs that carry a bash statement past a
// newline the walk would otherwise read as closing the gating element, which is
// how a `||` gets parked outside the window (NB's mutant is exactly this, one
// nesting level out). Refusing to read past any of them keeps the edge honest —
// an unclosed bracket is reported as unreadable rather than skipped over.
func bracketConjunct(s string) (int, bool) {
	open, closer := "[", "]"
	if strings.HasPrefix(s, "[[") {
		open, closer = "[[", "]]"
	}
	var quote byte // 0 when outside quotes, else the opening ' or "
	braces := 0    // depth of ${…}, which must be closed before the test is
	for i := len(open); i < len(s); {
		c := s[i]
		if c == '\n' {
			return 0, false // the test must not span the line the window ends on
		}
		if quote != 0 {
			switch {
			case c == quote:
				quote = 0
			case quote == '\'':
				// single quotes are literal: nothing expands, nothing escapes
			case c == '`' || c == '\\':
				return 0, false
			case c == '$' && i+1 < len(s) && s[i+1] == '(':
				return 0, false
			case c == '$' && i+1 < len(s) && s[i+1] == '{':
				braces++
				i++
			case c == '}' && braces > 0:
				braces--
			}
			i++
			continue
		}
		switch {
		case braces == 0 && strings.HasPrefix(s[i:], closer):
			return i + len(closer), true
		case c == '\'' || c == '"':
			quote = c
			i++
		case c == '$' && i+1 < len(s) && s[i+1] == '(':
			return 0, false
		case c == '$' && i+1 < len(s) && s[i+1] == '{':
			braces += 1
			i += 2
		case c == '}':
			if braces == 0 {
				return 0, false
			}
			braces--
			i++
		case c == '`' || c == '(' || c == ')' || c == '<' || c == '>' ||
			c == '&' || c == '|' || c == ';' || c == '#' || c == '\\' || c == '[':
			return 0, false
		default:
			i++
		}
	}
	return 0, false // never closed: the window was cut inside the test
}

// danglesIntoNextLine reports whether the line ended by the newline at index nl
// leaves its statement open, so the following line continues it. Bash accepts
// both spellings: a `\`-escaped newline, and a newline after a trailing `&&`,
// `||` or `|`. Treating only the first as a continuation is what would let a
// `[ -n "$resp" ] ||`-newline-`CODE && MSG` hide its `||` above the window's
// left edge.
//
// It answers only for the LEFT edge now (see statementStart). The right edge is
// walked by scanCondition, which tracks the pending operator directly instead of
// re-reading the previous line — the two differ on a blank line inside a
// condition, where the operator two lines up is still unsatisfied and the walk
// must keep going.
func danglesIntoNextLine(s string, nl int) bool {
	if nl > 0 && s[nl-1] == '\\' {
		return true
	}
	line := strings.TrimRight(s[:nl], " \t")
	return strings.HasSuffix(line, "&&") || strings.HasSuffix(line, "||") ||
		strings.HasSuffix(line, "|")
}

// statementStart returns the index at which the shell statement containing idx
// begins, walking back over continued lines. It bounds the backward search for
// the `if`/`elif` that opens the condition.
//
// It under-reaches rather than over-reaches, and deliberately so: a condition
// whose opener sits behind a bare newline (`if`⏎`CODE && MSG; then`) is a
// statement this bound stops short of, so the opener is not found and the guard
// FAILs asking to be re-pointed. That is the fail-closed direction. The
// symmetric forward bound used to exist and is GONE — it read a bare newline as
// the end of the statement, which is false inside an `if` condition (there a
// newline separates list elements) and silently placed every later element
// outside the window. scanCondition walks the right edge instead.
func statementStart(s string, idx int) int {
	for i := idx; i > 0; i-- {
		if s[i-1] == '\n' && !danglesIntoNextLine(s, i-1) {
			return i
		}
	}
	return 0
}

// shellWordBoundary reports whether index i of s delimits a shell word. Either
// end of s counts, so a keyword abutting the text's edge still stands alone.
func shellWordBoundary(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return true
	}
	switch s[i] {
	case ' ', '\t', '\n', ';', '&', '|', '(', ')', '\\':
		return true
	}
	return false
}

// shellKeywordAt reports whether kw begins at index i of s as a word in its own
// right, so `if` is never matched inside `elif`, `notify` or a quoted literal.
// Boundaries are read from the whole of s, not from any window into it.
func shellKeywordAt(s, kw string, i int) bool {
	return strings.HasPrefix(s[i:], kw) &&
		shellWordBoundary(s, i-1) && shellWordBoundary(s, i+len(kw))
}

// firstShellKeyword returns the bounds of the first occurrence of any of kws in
// [lo, hi) that stands alone as a shell word, or (-1, -1) when there is none.
// FIRST, not last: a compound `if` command begins at its first `if`, so an `if`
// nested INSIDE the condition can never pull the window's left edge rightward
// past a disjunction standing before it.
func firstShellKeyword(s string, lo, hi int, kws ...string) (int, int) {
	for i := lo; i < hi; i++ {
		for _, kw := range kws {
			if i+len(kw) <= hi && shellKeywordAt(s, kw, i) {
				return i, i + len(kw)
			}
		}
	}
	return -1, -1
}

// goFuncBody returns the source of the top-level function whose signature
// starts with sig, up to the next top-level declaration.
func goFuncBody(t *testing.T, src, sig string) string {
	t.Helper()
	start := strings.Index(src, sig)
	require.GreaterOrEqualf(t, start, 0, "%s defines no %s", grpcServerSourceRel, sig)

	rest := src[start+len(sig):]
	if end := strings.Index(rest, "\nfunc "); end >= 0 {
		return rest[:end]
	}
	return rest
}

// stripShellComments removes `#`-to-end-of-line from each line so assertions
// read the script's code rather than its commentary.
func stripShellComments(body string) string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func firstSubmatch(t *testing.T, re *regexp.Regexp, body, name string) string {
	t.Helper()
	m := re.FindStringSubmatch(body)
	require.Lenf(t, m, 2, "%s must define %s", grpcChallengeScriptRel, name)
	return m[1]
}
