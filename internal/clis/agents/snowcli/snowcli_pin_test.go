package snowcli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 BLUFF-001 PIN GUARD — snowcli real-exec wiring (§11.4.115 / §11.4.135)
//
// HISTORY: SnowCLI.query USED to return a hardcoded {"result":"Query result"}
// WITHOUT ever exec-ing the real `snow` CLI (zero os/exec in snowcli.go) — a
// BLUFF-001 / CONST-035 false-success (the agent claimed to run SQL while
// running nothing).
//
// FIX: query now exec-s the real `snow sql -q "<sql>"` command via exec.LookPath
// + exec.CommandContext (resolveSnowBinary). When the binary is absent it
// returns an HONEST error, never a fabricated success.
// ---------------------------------------------------------------------------

func writeFakeSnow(t *testing.T, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: D-17 — POSIX shell-script fake binary; real-exec path is cross-platform.")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-snow")
	// Echoes the marker + all args so we can prove the SQL was forwarded.
	script := "#!/bin/sh\nprintf '" + marker + ":%s' \"$*\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake snow: %v", err)
	}
	return bin
}

func TestD17_SnowCLI_QueryExecsRealBinary(t *testing.T) {
	const marker = "FAKE_SNOW_RAN_5b3e"
	bin := writeFakeSnow(t, marker)
	t.Setenv("SNOW_BIN", bin)

	s := New()
	ctx := context.Background()

	res, err := s.query(ctx, map[string]interface{}{"sql": "SELECT 42"})
	if err != nil {
		t.Fatalf("query returned error with fake binary injected: %v", err)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}
	result, _ := m["result"].(string)
	if !strings.Contains(result, marker) {
		t.Fatalf("D-17 REGRESSION: SnowCLI.query did NOT exec the snow binary — marker %q absent from %q (BLUFF-001?).", marker, result)
	}
	if result == "Query result" {
		t.Fatalf("D-17 REGRESSION: SnowCLI.query returned the hardcoded placeholder %q instead of real CLI output (BLUFF-001).", result)
	}
	if !strings.Contains(result, "SELECT 42") {
		t.Fatalf("D-17 REGRESSION: SQL was not forwarded to the snow binary (got %q).", result)
	}
}

func TestD17_SnowCLI_AbsentBinaryIsHonestError(t *testing.T) {
	t.Setenv("SNOW_BIN", filepath.Join(t.TempDir(), "does-not-exist-snow"))

	s := New()
	ctx := context.Background()

	if res, err := s.query(ctx, map[string]interface{}{"sql": "SELECT 1"}); err == nil {
		t.Fatalf("D-17 BLUFF: query returned success %v with NO snow binary — must be an honest error.", res)
	}
	if s.IsAvailable() {
		t.Fatal("D-17 BLUFF: IsAvailable() = true with NO snow binary — must reflect real PATH state.")
	}
}

func TestD17_SnowCLI_IsStubBluff(t *testing.T) {
	if os.Getenv("PIN_STUB_BLUFF") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with PIN_STUB_BLUFF=1.")
	}
	t.Setenv("SNOW_BIN", filepath.Join(t.TempDir(), "does-not-exist-snow"))
	s := New()
	ctx := context.Background()
	res, err := s.query(ctx, map[string]interface{}{"sql": "SELECT 1"})
	if err != nil {
		return // fixed artifact: honest error.
	}
	m, _ := res.(map[string]interface{})
	if r, _ := m["result"].(string); r == "Query result" {
		t.Fatalf("D-17 BLUFF PINNED: query returned hardcoded %q without exec (BLUFF-001).", r)
	}
}
