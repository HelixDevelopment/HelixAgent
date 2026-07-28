package stress

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// HXC-165 — latency stress runs must never write over a tracked file.
//
// The committed reference baseline lives at
// reports/latency/p99-baseline-2026-03-16.txt and is READ-ONLY for every test
// in this package. Before this fix, TestExtremeP99LatencyBaseline wrote its own
// measurements straight over that path on every run, so the yardstick silently
// became whatever the machine last measured, the tree was permanently dirty,
// and the corrupted artefact was repeatedly swept into commits (two of the five
// historical modifications landed via blanket "Auto-commit" commits).
//
// Nothing in the tree reads that file for comparison; it is retained as a
// historical artefact. Per-run output now goes to reports/latency/runs/, which
// is gitignored.
//
// Two independent layers protect it:
//
//   - TestMain below hashes the baseline before and after the WHOLE package
//     run, so ANY test in this package that writes to it fails the package —
//     including a re-inlined os.WriteFile at the original defect site.
//   - TestP99LatencyBaselineArtifactIsolation additionally pins the writer's
//     behaviour and self-validates its own hash oracle.

// latencyBaselineRelPath is the tracked reference baseline filename.
const latencyBaselineRelPath = "p99-baseline-2026-03-16.txt"

// runUTCMarker prefixes the per-run timestamp in a run report.
const runUTCMarker = "Run-UTC: "

// frozenPreFixDateHeader is the header the pre-fix writer emitted on EVERY run
// regardless of when it ran. Its reappearance in run output is a regression.
const frozenPreFixDateHeader = "Date: 2026-03-16"

// runUTCStamp extracts the Run-UTC timestamp from a run report.
func runUTCStamp(report string) (time.Time, error) {
	idx := strings.Index(report, runUTCMarker)
	if idx < 0 {
		return time.Time{}, fmt.Errorf("no %q header", runUTCMarker)
	}
	value := report[idx+len(runUTCMarker):]
	if nl := strings.IndexByte(value, '\n'); nl >= 0 {
		value = value[:nl]
	}
	stamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("unparseable %s%q: %w", runUTCMarker, value, err)
	}
	return stamp, nil
}

// latencyStats is the percentile summary produced by one latency stress run.
type latencyStats struct {
	Samples int
	Min     time.Duration
	Mean    time.Duration
	P50     time.Duration
	P90     time.Duration
	P99     time.Duration
	Max     time.Duration
}

// latencyReportDir returns the tracked directory holding the reference baseline.
func latencyReportDir() string {
	return filepath.Join("..", "..", "reports", "latency")
}

// latencyRunsDir returns the gitignored directory that receives per-run
// reports. It is deliberately a subdirectory of latencyReportDir so the
// baseline and its runs stay discoverable together, while only the runs
// directory is excluded from version control.
//
// Regeneration (§11.4.77): this directory holds pure test output. Nothing
// builds, runs or tests against it; it is recreated by re-running
//
//	go test -run '^TestExtremeP99LatencyBaseline$' ./tests/stress/
func latencyRunsDir() string {
	return filepath.Join(latencyReportDir(), "runs")
}

// latencyBaselinePath returns the path of the tracked reference baseline.
func latencyBaselinePath() string {
	return filepath.Join(latencyReportDir(), latencyBaselineRelPath)
}

// fileSHA256 returns the hex SHA-256 of a file. A read failure is reported as
// an error and never conflated with "the content changed" — collapsing the two
// would let an unreadable file masquerade as a detected modification.
// A file that does not exist hashes to "" with no error, so an absent baseline
// that stays absent compares equal.
func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // test-controlled path
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return sha256Hex(data), nil
}

// sha256Hex hashes bytes already in hand, so a caller that has read a file once
// never needs to re-read it merely to hash it again — a re-read would give a
// concurrent external writer a window to turn a consistent check into a
// confusing failure.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestMain is the package-wide HXC-165 regression guard (§11.4.135). It hashes
// the tracked reference baseline before and after every test in this package
// and fails the run if the bytes changed.
//
// This guards the DEFECT SITE rather than only the helper: if anyone re-inlines
// a direct write to the baseline into any test here, this fails even though the
// helper itself is still well-behaved.
//
// Blast radius, stated plainly: an unreadable baseline aborts the whole ~50-file
// stress package before any test runs. That is deliberate fail-closed behaviour
// — the alternative is running the suite with the guard silently disarmed — but
// it does mean a permissions or I/O fault here blocks far more than latency.
// A baseline that is merely ABSENT is not an error: it hashes to "" and, as long
// as it stays absent, compares equal.
func TestMain(m *testing.M) {
	baseline := latencyBaselinePath()

	before, beforeErr := fileSHA256(baseline)
	if beforeErr != nil {
		fmt.Fprintf(os.Stderr, "HXC-165 guard: could not hash %s before the run: %v\n", baseline, beforeErr)
		os.Exit(1)
	}

	code := m.Run()

	after, afterErr := fileSHA256(baseline)
	switch {
	case afterErr != nil:
		fmt.Fprintf(os.Stderr, "HXC-165 guard: could not hash %s after the run: %v\n", baseline, afterErr)
		if code == 0 {
			code = 1
		}
	case after != before:
		// Attribution boundary (§11.4.6): a before/after pair proves the file
		// CHANGED DURING THE WINDOW. It cannot distinguish a write by a test in
		// this package from a concurrent external writer (other agents and
		// checkouts share this tree). Failing on the change is correct; naming
		// the culprit would assert more than the evidence supports.
		fmt.Fprintf(os.Stderr,
			"HXC-165: the tracked reference baseline %s changed during this package run "+
				"(sha256 %q -> %q) — a test in this package, or a concurrent external writer. "+
				"Latency output belongs in %s, which is gitignored.\n",
			baseline, before, after, latencyRunsDir())
		if code == 0 {
			code = 1
		}
	}

	os.Exit(code)
}

// formatLatencyReport renders one run's measurements. The header carries the
// run's own UTC timestamp — the pre-fix report hardcoded "Date: 2026-03-16"
// regardless of when it ran, which is what let months-old-looking output
// masquerade as the original baseline.
func formatLatencyReport(stats latencyStats, at time.Time) string {
	return fmt.Sprintf(
		"P99 Latency Run Report\n"+
			"======================\n"+
			"Run-UTC: %s\n"+
			"Samples: %d\n"+
			"Min: %v\n"+
			"Mean: %v\n"+
			"P50: %v\n"+
			"P90: %v\n"+
			"P99: %v\n"+
			"Max: %v\n",
		at.Format(time.RFC3339Nano),
		stats.Samples, stats.Min, stats.Mean, stats.P50, stats.P90, stats.P99, stats.Max,
	)
}

// writeLatencyRunReport writes one timestamped per-run report into the
// gitignored runs directory and returns the path written (empty on failure —
// report writing is best-effort and never fails the measurement it describes;
// TestP99LatencyBaselineArtifactIsolation is what proves the write lands).
//
// The filename carries a nanosecond UTC stamp plus the PID so concurrent runs
// on a shared host cannot collide or overwrite one another.
func writeLatencyRunReport(t *testing.T, stats latencyStats) string {
	t.Helper()

	dir := latencyRunsDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Logf("Warning: could not create latency runs dir %s: %v", dir, err)
		return ""
	}

	now := time.Now().UTC()
	name := fmt.Sprintf("p99-run-%s-pid%d.txt", now.Format("20060102T150405.000000000"), os.Getpid())
	path := filepath.Join(dir, name)

	if err := os.WriteFile(path, []byte(formatLatencyReport(stats, now)), 0o600); err != nil {
		t.Logf("Warning: could not write latency run report %s: %v", path, err)
		return ""
	}

	t.Logf("Latency run report written to %s", path)
	return path
}

// TestP99LatencyBaselineArtifactIsolation pins the HXC-165 fix at the writer
// level and self-validates its own oracle.
//
// It asserts that writing a run report (a) lands inside the gitignored runs
// directory and (b) leaves the tracked reference baseline byte-identical, then
// (c) proves the sha256 comparison used for (b) actually detects a modification
// — a golden-good / golden-bad pair per §11.4.107(10), run against a temporary
// copy so no tracked file is ever written by this test.
//
// The §11.4.115 RED reproduction against the real pre-fix artifact was captured
// live during the fix (baseline sha256 73f8ef85… -> 88c8b1a9… after one run of
// TestExtremeP99LatencyBaseline) and is recorded in the fix commit; it is
// deliberately NOT re-enacted here, because deliberately corrupting a tracked
// file on every RED invocation is not crash-safe and races other checkouts.
func TestP99LatencyBaselineArtifactIsolation(t *testing.T) {
	baseline := latencyBaselinePath()

	// Read the baseline ONCE. Every later use hashes these same bytes, so a
	// concurrent external writer cannot make the golden-good half of the oracle
	// check disagree with the snapshot it is validating.
	originalBytes, readErr := os.ReadFile(baseline) //nolint:gosec // test-controlled path
	if readErr != nil {
		if os.IsNotExist(readErr) {
			t.Skipf("reference baseline %s is not present in this checkout", baseline) // SKIP-OK: #requires-reference-baseline
		}
		t.Fatalf("could not read reference baseline: %v", readErr)
	}
	before := sha256Hex(originalBytes)

	stats := latencyStats{
		Samples: 3,
		Min:     1 * time.Microsecond,
		Mean:    2 * time.Microsecond,
		P50:     2 * time.Microsecond,
		P90:     3 * time.Microsecond,
		P99:     4 * time.Microsecond,
		Max:     5 * time.Microsecond,
	}

	// (a) the report must land in the gitignored runs directory.
	path := writeLatencyRunReport(t, stats)
	if path == "" {
		t.Fatal("writeLatencyRunReport did not write a report")
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	if path == baseline {
		t.Fatalf("run report was written to the tracked baseline path %s", baseline)
	}
	absRuns, err := filepath.Abs(latencyRunsDir())
	if err != nil {
		t.Fatalf("could not resolve runs dir: %v", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("could not resolve report path: %v", err)
	}
	if filepath.Dir(absPath) != absRuns {
		t.Fatalf("run report landed outside the gitignored runs dir: got %s, want a file directly in %s", absPath, absRuns)
	}
	// The report must have real content, not merely exist: os.Stat succeeds on a
	// zero-byte file, and "it landed in the right directory" says nothing about
	// what is in it. These assertions pin the two behaviours the fix claims —
	// the report is populated, and it carries its OWN run timestamp rather than
	// the frozen header that let pre-fix output masquerade as the baseline.
	reportBytes, readReportErr := os.ReadFile(path) //nolint:gosec // path returned by writeLatencyRunReport
	if readReportErr != nil {
		t.Fatalf("run report %s was not created or is unreadable: %v", path, readReportErr)
	}
	report := string(reportBytes)
	if len(reportBytes) == 0 {
		t.Fatalf("run report %s is empty", path)
	}
	if strings.Contains(report, frozenPreFixDateHeader) {
		t.Fatalf("run report %s carries the frozen pre-fix header %q — run output must carry its own timestamp:\n%s",
			path, frozenPreFixDateHeader, report)
	}
	stamp, stampErr := runUTCStamp(report)
	if stampErr != nil {
		t.Fatalf("run report %s: %v\ncontent:\n%s", path, stampErr, report)
	}
	if age := time.Since(stamp); age > time.Hour || age < -time.Minute {
		t.Fatalf("run report %s has Run-UTC %s, which is not from this run (age %s)", path, stamp, age)
	}

	// (b) the tracked baseline must be untouched.
	after, err := fileSHA256(baseline)
	if err != nil {
		t.Fatalf("could not re-hash reference baseline: %v", err)
	}
	if after != before {
		// Same attribution boundary as TestMain (§11.4.6): the before/after pair
		// proves the file CHANGED, not who changed it.
		t.Fatalf("tracked reference baseline %s changed during this test (sha256 %s -> %s) — a write by this test, or a concurrent external writer",
			baseline, before, after)
	}

	// (c) prove the comparison in (b) is not blind: the same oracle must report
	// "unchanged" for an untouched copy and "changed" for a modified one.
	assertHashOracleDetectsChange(t, originalBytes, before)

	t.Logf("Tracked baseline unchanged (sha256 %s); run report isolated at %s", after, path)
}

// assertHashOracleDetectsChange is the golden-good / golden-bad self-validation
// for the sha256 comparison used above. It operates entirely on a temporary
// copy staged from bytes the caller already read, so it neither writes to a
// tracked path nor re-reads one that a concurrent writer could change underneath
// it.
func assertHashOracleDetectsChange(t *testing.T, original []byte, baselineHash string) {
	t.Helper()

	probe := filepath.Join(t.TempDir(), latencyBaselineRelPath)
	if writeErr := os.WriteFile(probe, original, 0o600); writeErr != nil {
		t.Fatalf("could not stage oracle probe: %v", writeErr)
	}

	// golden-good: an identical copy must hash identically.
	good, err := fileSHA256(probe)
	if err != nil {
		t.Fatalf("oracle self-validation could not hash the unmodified probe: %v", err)
	}
	if good != baselineHash {
		t.Fatalf("oracle self-validation: unmodified copy hashed %s, want %s", good, baselineHash)
	}

	// golden-bad: the pre-fix write pattern applied to the probe must be detected.
	defectBytes := []byte(formatLatencyReport(latencyStats{Samples: 1}, time.Now().UTC()))
	if writeErr := os.WriteFile(probe, defectBytes, 0o600); writeErr != nil {
		t.Fatalf("could not apply oracle golden-bad mutation: %v", writeErr)
	}
	bad, err := fileSHA256(probe)
	if err != nil {
		t.Fatalf("oracle self-validation could not hash the mutated probe: %v", err)
	}
	if bad == baselineHash {
		t.Fatalf("oracle self-validation FAILED: an overwritten copy still hashed %s — this guard cannot detect the HXC-165 defect", bad)
	}
}
